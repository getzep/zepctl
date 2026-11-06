package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/getzep/zepctl/internal/keyring"
)

// ErrSSOLoginRequired directs an SSO-bound Member to the enterprise login flow.
var ErrSSOLoginRequired = errors.New("enterprise SSO login is required; run \"zepctl auth login --sso\"")

// SSOLoginOptions configures the dashboard-to-CLI enterprise SSO handoff.
type SSOLoginOptions struct {
	AppURL      string
	NoBrowser   bool
	ReadCode    func(context.Context) (string, error)
	Output      io.Writer
	OpenBrowser func(string) error
	HTTPClient  *http.Client
}

// ResolveSSOAppURL selects the production dashboard or an explicit deployment.
func ResolveSSOAppURL(apiURL, explicit string) (string, error) {
	if explicit == "" {
		if apiURL == "" {
			apiURL = DefaultAPIURL
		}
		if !isProductionSSOAPI(apiURL) {
			return "", fmt.Errorf("set --sso-url to the dashboard URL for this API deployment")
		}
		explicit = "https://app.getzep.com"
	}
	app, err := url.Parse(explicit)
	if err != nil {
		return "", fmt.Errorf("parsing the SSO dashboard URL: %w", err)
	}
	validScheme := validSSOAppScheme(app)
	if app.Hostname() == "" || app.User != nil || app.RawQuery != "" || app.Fragment != "" ||
		app.ForceQuery || (app.Path != "" && app.Path != "/") || !validScheme {
		return "", fmt.Errorf("the SSO dashboard URL must be an HTTPS origin; HTTP is allowed only on loopback")
	}
	return strings.TrimRight(app.String(), "/"), nil
}

func validSSOAppScheme(app *url.URL) bool {
	loopback := app.Hostname() == "127.0.0.1" || app.Hostname() == "localhost" || app.Hostname() == "::1"
	return app.Scheme == "https" || (app.Scheme == "http" && loopback)
}

func isProductionSSOAPI(raw string) bool {
	api, err := url.Parse(raw)
	return err == nil && api.Scheme == "https" && api.Hostname() == "api.getzep.com" &&
		(api.Port() == "" || api.Port() == "443")
}

type ssoFlow struct {
	state, verifier, redirectURI string
}

func newSSOFlow() (*ssoFlow, error) {
	state := make([]byte, 32)
	verifier := make([]byte, 32)
	if _, err := rand.Read(state); err != nil {
		return nil, fmt.Errorf("generating SSO state: %w", err)
	}
	if _, err := rand.Read(verifier); err != nil {
		return nil, fmt.Errorf("generating SSO verifier: %w", err)
	}
	return &ssoFlow{
		state: base64.RawURLEncoding.EncodeToString(state), verifier: base64.RawURLEncoding.EncodeToString(verifier),
		redirectURI: callbackURL(),
	}, nil
}

func (f *ssoFlow) startURL(appURL string, manual bool) string {
	challenge := sha256.Sum256([]byte(f.verifier))
	params := url.Values{
		"redirect_uri": {f.redirectURI}, "state": {f.state},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])},
	}
	if manual {
		params.Set("manual", "true")
	}
	return appURL + "/api/auth/cli/start?" + params.Encode()
}

// LoginSSO obtains the current enterprise credential through a PKCE-bound code.
// It does not store the credential or use the provider's refresh endpoint.
func LoginSSO(ctx context.Context, opts SSOLoginOptions) (*TokenResult, error) {
	appURL, err := ResolveSSOAppURL("", opts.AppURL)
	if err != nil {
		return nil, err
	}
	flow, err := newSSOFlow()
	if err != nil {
		return nil, err
	}
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	if opts.OpenBrowser == nil {
		opts.OpenBrowser = func(target string) error { return openBrowser(ctx, target) }
	}
	ctx, cancel := context.WithTimeout(ctx, callbackTimeout)
	defer cancel()
	var code string
	if opts.NoBrowser {
		code, err = flow.manualCode(ctx, appURL, opts)
	} else {
		code, err = flow.browserCode(ctx, appURL, opts)
	}
	if err != nil {
		return nil, err
	}
	return flow.exchange(ctx, appURL, code, opts.HTTPClient)
}

func (f *ssoFlow) manualCode(ctx context.Context, appURL string, opts SSOLoginOptions) (string, error) {
	// manual=true makes the dashboard display the code instead of redirecting.
	// This flow does not use a loopback callback listener.
	if opts.ReadCode == nil {
		return "", fmt.Errorf("authorization-code input is not available")
	}
	fmt.Fprintf(opts.Output, "Open this URL to sign in through enterprise SSO:\n%s\n\nEnter the authorization code: ", f.startURL(appURL, true))
	code, err := opts.ReadCode(ctx)
	if err != nil {
		return "", fmt.Errorf("reading the authorization code: %w", err)
	}
	return strings.TrimSpace(code), nil
}

func (f *ssoFlow) browserCode(ctx context.Context, appURL string, opts SSOLoginOptions) (string, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return f.manualCode(ctx, appURL, opts)
	}
	f.redirectURI = "http://" + listener.Addr().String() + callbackPath
	codes := make(chan string, 1)
	server := &http.Server{
		Handler:           f.callbackHandler(codes, listener.Addr().String()),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
	}
	defer server.Close()
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	start := f.startURL(appURL, false)
	fmt.Fprintf(opts.Output, "Open this URL to sign in through enterprise SSO:\n%s\n", start)
	if err := opts.OpenBrowser(start); err != nil {
		fmt.Fprintln(opts.Output, "The browser could not be opened. Open the URL above.")
	}
	select {
	case code := <-codes:
		return code, nil
	case err := <-serveErrors:
		return "", fmt.Errorf("serving the SSO callback: %w", err)
	case <-ctx.Done():
		return "", fmt.Errorf("waiting for SSO authorization: %w", ctx.Err())
	}
}

func (f *ssoFlow) callbackHandler(codes chan<- string, host string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		state := r.URL.Query()["state"]
		code := r.URL.Query()["code"]
		if r.Method != http.MethodGet || r.URL.Path != callbackPath || r.Host != host ||
			len(state) != 1 || len(code) != 1 || !validSSOCode(code[0]) ||
			subtle.ConstantTimeCompare([]byte(state[0]), []byte(f.state)) != 1 {
			http.Error(w, "The CLI callback is not valid.", http.StatusBadRequest)
			return
		}
		select {
		case codes <- code[0]:
			fmt.Fprintln(w, "The CLI received the authorization code. Return to your terminal.")
		default:
			http.Error(w, "The authorization code was already received.", http.StatusConflict)
		}
	})
}

func validSSOCode(code string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(code)
	return err == nil && len(decoded) == 32 && base64.RawURLEncoding.EncodeToString(decoded) == code
}

func (f *ssoFlow) exchange(ctx context.Context, appURL, code string, client *http.Client) (*TokenResult, error) {
	if !validSSOCode(code) {
		return nil, fmt.Errorf("the authorization code is not valid; start the SSO login again")
	}
	body, err := json.Marshal(map[string]string{
		"code": code, "code_verifier": f.verifier, "redirect_uri": f.redirectURI,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding the SSO exchange: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appURL+"/api/auth/cli/exchange", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building the SSO exchange: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	noRedirects := *client
	noRedirects.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirects.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the SSO code exchange could not be completed; start the SSO login again")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the SSO code exchange failed (HTTP %d); start the SSO login again", resp.StatusCode)
	}
	var token struct {
		AccessToken string    `json:"access_token"`
		TokenType   string    `json:"token_type"`
		ExpiresAt   time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&token); err != nil {
		return nil, fmt.Errorf("the SSO exchange returned an invalid response")
	}
	if token.AccessToken == "" || token.TokenType != bearerTokenType || !token.ExpiresAt.After(time.Now()) {
		return nil, fmt.Errorf("the SSO credential is missing or expired; start the SSO login again")
	}
	return &TokenResult{AccessToken: token.AccessToken, IDToken: token.AccessToken, ExpiresAt: token.ExpiresAt}, nil
}

// StoreSSOBearerToken preserves the API key and marks the non-refreshable SSO credential.
func StoreSSOBearerToken(profile string, result *TokenResult, email string) error {
	creds, err := keyring.GetCredentials(profile)
	if err != nil {
		return fmt.Errorf("reading existing credentials: %w", err)
	}
	creds.AccessToken = result.AccessToken
	creds.RefreshToken = ""
	creds.ExpiresAt = result.ExpiresAt.Format(time.RFC3339)
	creds.UserEmail = email
	creds.EnterpriseSSO = true
	return keyring.SetCredentials(profile, creds)
}
