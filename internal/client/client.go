package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	zepclient "github.com/getzep/zep-go/v3/client"
	"github.com/getzep/zep-go/v3/option"
	"github.com/getzep/zepctl/internal/auth"
	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/keyring"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

// CredentialType indicates which credential a command requires.
type CredentialType int

const (
	// CredentialAPIKey means the command uses API key authentication.
	CredentialAPIKey CredentialType = iota
	// CredentialBearer means the command uses bearer token authentication.
	CredentialBearer
)

// credentialTypeAnnotation is the Cobra annotation key for declaring a
// command's credential type at registration time.
const credentialTypeAnnotation = "zepctl_credential_type" //nolint:gosec // Annotation key, not a credential

// projectHeader is the HTTP header used to specify the target project
// for bearer-authenticated requests.
const projectHeader = "X-Zep-Project"

// Client is an alias for the Zep client.
type Client = zepclient.Client

// SetCredentialType declares the credential type a command requires. Call
// this at command registration time (typically in an init function).
func SetCredentialType(cmd *cobra.Command, ct CredentialType) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	switch ct {
	case CredentialBearer:
		cmd.Annotations[credentialTypeAnnotation] = "bearer"
	default:
		cmd.Annotations[credentialTypeAnnotation] = "api-key"
	}
}

// credentialTypeFromCommand reads the credential type declared on cmd via
// SetCredentialType. Returns CredentialAPIKey if no annotation is set.
func credentialTypeFromCommand(cmd *cobra.Command) CredentialType {
	if cmd.Annotations != nil {
		if v, ok := cmd.Annotations[credentialTypeAnnotation]; ok && v == "bearer" {
			return CredentialBearer
		}
	}
	return CredentialAPIKey
}

// NewForCommand creates a Zep client using the credential type declared on
// cmd. If the --api-key flag or ZEP_API_KEY env var is set, API key auth
// is used regardless of the command's declaration.
func NewForCommand(cmd *cobra.Command) (*Client, error) {
	// Explicit API key override bypasses the command's declared type.
	if config.GetAPIKeyOverride() != "" {
		return NewWithCredential(cmd.Context(), CredentialAPIKey)
	}
	return NewWithCredential(cmd.Context(), credentialTypeFromCommand(cmd))
}

// NewWithCredential creates a new Zep client using the specified credential type.
func NewWithCredential(ctx context.Context, credType CredentialType) (*Client, error) {
	var opts []option.RequestOption

	switch credType {
	case CredentialBearer:
		httpClient, err := newBearerClient(ctx)
		if err != nil {
			return nil, err
		}
		headers := http.Header{}
		if projectUUID := config.GetProjectUUID(); projectUUID != "" {
			headers.Set(projectHeader, projectUUID)
		}
		opts = append(opts,
			option.WithHTTPClient(httpClient),
			option.WithHTTPHeader(headers),
		)
	default:
		apiKey := config.GetAPIKey()
		if apiKey == "" {
			return nil, fmt.Errorf("no API key configured; set ZEP_API_KEY or run \"zepctl config add-profile\"")
		}
		opts = append(opts, option.WithAPIKey(apiKey))
	}

	if apiURL := config.GetAPIURL(); apiURL != "" {
		opts = append(opts, option.WithBaseURL(apiURL))
	}

	return zepclient.NewClient(opts...), nil
}

// newBearerClient returns an *http.Client that automatically attaches
// bearer tokens and refreshes them when expired via golang.org/x/oauth2.
// The returned client's transport is wrapped to handle refresh failures:
// on a token retrieval error the bearer token fields are cleared from the
// keychain and a user-facing message is returned.
func newBearerClient(ctx context.Context) (*http.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	profile := cfg.GetCurrentProfile()
	if profile == nil {
		return nil, fmt.Errorf("no active profile; run \"zepctl auth login\" to authenticate")
	}

	oauthCfg := auth.OAuthConfigFor(profile.OAuthIssuer, profile.OAuthClientID, profile.OAuthAudience)
	session := auth.NewKeychainSession(profile.Name)

	httpClient, err := auth.NewAutoRefreshClient(ctx, oauthCfg, session)
	if err != nil {
		return nil, err
	}

	// Wrap the transport to detect refresh failures and clear stale tokens.
	httpClient.Transport = &refreshFailureTransport{
		base:    httpClient.Transport,
		profile: profile.Name,
	}

	return httpClient, nil
}

// NewBearerHTTPClient returns a raw *http.Client that attaches bearer token
// auth headers. Refreshes the token if expired. Used for web API calls
// (account resolution, project listing) that go through the web middleware
// rather than the SDK client.
func NewBearerHTTPClient(ctx context.Context) (*http.Client, string, error) {
	httpClient, err := newBearerClient(ctx)
	if err != nil {
		return nil, "", err
	}

	apiURL := config.GetAPIURL()
	if apiURL == "" {
		apiURL = auth.DefaultAPIURL
	}

	return httpClient, apiURL, nil
}

// BearerTransport is an http.RoundTripper that adds a Bearer Authorization header.
// Used by autoSelectProject during login when the SDK client is not yet available.
type BearerTransport struct {
	Token string
	Base  http.RoundTripper
}

func (t *BearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+t.Token)
	return t.Base.RoundTrip(req2)
}

// refreshFailureTransport wraps the oauth2 auto-refresh transport to
// detect token refresh failures. When a refresh fails (expired/revoked
// refresh token), it clears the bearer token fields from the keychain and
// returns a user-facing error message.
type refreshFailureTransport struct {
	base    http.RoundTripper
	profile string
}

func (t *refreshFailureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err == nil {
		return resp, nil
	}

	var retrieveErr *oauth2.RetrieveError
	if !errors.As(err, &retrieveErr) {
		return nil, err
	}

	// If this is an invalid_grant error, another process may have already
	// rotated the refresh token. Re-read from keychain and retry once with
	// the (possibly updated) access token before giving up. The retry
	// goes through http.DefaultTransport directly (not the oauth2 chain)
	// to avoid recursive refresh attempts. If the retry also fails, the
	// error is returned without the friendly "session expired" message --
	// acceptable for a CLI where this is an edge case.
	if retrieveErr.ErrorCode == "invalid_grant" {
		creds, kerr := keyring.GetCredentials(t.profile)
		if kerr == nil && creds.HasBearerToken() && !creds.IsExpired() {
			retry := req.Clone(req.Context())
			retry.Header.Set("Authorization", "Bearer "+creds.AccessToken)
			return http.DefaultTransport.RoundTrip(retry)
		}
	}

	_ = auth.ClearBearerToken(t.profile)
	return nil, fmt.Errorf("session expired; run \"zepctl auth login\" to re-authenticate")
}
