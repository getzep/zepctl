package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/keyring"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	gokeyring "github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func init() {
	// Use in-memory mock keyring for tests.
	gokeyring.MockInit()
}

func TestBearerTransport_SetsHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	transport := &BearerTransport{
		Token: "test_bearer_token",
		Base:  http.DefaultTransport,
	}

	httpClient := &http.Client{Transport: transport}
	resp, err := httpClient.Get(srv.URL) //nolint:noctx // test-only, no context needed
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if gotAuth != "Bearer test_bearer_token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test_bearer_token")
	}
}

func TestCredentialType_Constants(t *testing.T) {
	if CredentialAPIKey == CredentialBearer {
		t.Error("CredentialAPIKey and CredentialBearer should be different")
	}
}

func TestSetCredentialType_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		set  CredentialType
		want CredentialType
	}{
		{"api key", CredentialAPIKey, CredentialAPIKey},
		{"bearer", CredentialBearer, CredentialBearer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			SetCredentialType(cmd, tt.set)
			got := credentialTypeFromCommand(cmd)
			if got != tt.want {
				t.Errorf("credentialTypeFromCommand() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCredentialTypeFromCommand_DefaultsToAPIKey(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	if got := credentialTypeFromCommand(cmd); got != CredentialAPIKey {
		t.Errorf("credentialTypeFromCommand() = %d, want CredentialAPIKey", got)
	}
}

func TestRefreshFailureTransport_PassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &refreshFailureTransport{
		base:    http.DefaultTransport,
		profile: "test-refresh-passthrough",
	}

	httpClient := &http.Client{Transport: transport}
	resp, err := httpClient.Get(srv.URL) //nolint:noctx // test-only
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRefreshFailureTransport_DetectsRetrieveError(t *testing.T) {
	// Simulate the base transport returning an oauth2.RetrieveError,
	// which happens when the Kinde SDK tries to refresh an expired token.
	retrieveErr := &oauth2.RetrieveError{
		Response: &http.Response{StatusCode: http.StatusUnauthorized},
	}
	base := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, retrieveErr
	})

	transport := &refreshFailureTransport{
		base:    base,
		profile: "test-refresh-detect",
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", http.NoBody)
	resp, err := transport.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := `session expired; run "zepctl auth login" to re-authenticate`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestRefreshFailureTransport_NonRetrieveErrorPassesThrough(t *testing.T) {
	// Non-oauth2 errors should pass through unmodified.
	origErr := fmt.Errorf("network timeout")
	base := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, origErr
	})

	transport := &refreshFailureTransport{
		base:    base,
		profile: "test-refresh-other",
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", http.NoBody)
	resp, err := transport.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, origErr) {
		t.Errorf("expected original error %v, got %v", origErr, err)
	}
}

func TestRefreshFailureTransport_InvalidGrantRetry(t *testing.T) {
	// Simulate the concurrent refresh race: the base transport returns
	// invalid_grant (another process already rotated the refresh token),
	// but fresh credentials are available in the keychain.
	profile := "test-invalid-grant-retry"

	// Pre-load fresh credentials into the mock keychain.
	freshCreds := &keyring.Credentials{
		AccessToken:  "fresh_access_token",
		RefreshToken: "fresh_refresh_token",
		ExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if err := keyring.SetCredentials(profile, freshCreds); err != nil {
		t.Fatalf("seeding keychain: %v", err)
	}

	// Backend server that expects the fresh token.
	var gotAuth string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	invalidGrant := &oauth2.RetrieveError{
		Response:  &http.Response{StatusCode: 400},
		ErrorCode: "invalid_grant",
	}
	var calls atomic.Int32
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			// First call: simulate the SDK's failed refresh.
			return nil, invalidGrant
		}
		// Should not reach here -- the retry bypasses this transport.
		return nil, fmt.Errorf("unexpected second call to base transport")
	})

	transport := &refreshFailureTransport{
		base:    base,
		profile: profile,
	}

	// The request must target the real backend so the retry succeeds.
	req, _ := http.NewRequest(http.MethodGet, backend.URL+"/test", http.NoBody) //nolint:noctx // test-only
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if gotAuth != "Bearer fresh_access_token" {
		t.Errorf("retry Authorization = %q, want %q", gotAuth, "Bearer fresh_access_token")
	}
}

func TestRefreshFailureTransport_InvalidGrantExpiredFallback(t *testing.T) {
	// When invalid_grant fires but keychain creds are also expired,
	// it should fall through to the clear-and-error path.
	profile := "test-invalid-grant-expired"

	expiredCreds := &keyring.Credentials{
		AccessToken:  "stale_token",
		RefreshToken: "stale_refresh",
		ExpiresAt:    time.Now().Add(-time.Hour).Format(time.RFC3339),
	}
	if err := keyring.SetCredentials(profile, expiredCreds); err != nil {
		t.Fatalf("seeding keychain: %v", err)
	}

	invalidGrant := &oauth2.RetrieveError{
		Response:  &http.Response{StatusCode: 400},
		ErrorCode: "invalid_grant",
	}
	base := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, invalidGrant
	})

	transport := &refreshFailureTransport{
		base:    base,
		profile: profile,
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", http.NoBody)
	resp, err := transport.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := `session expired; run "zepctl auth login" to re-authenticate`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestNewWithCredential_APIKey_MissingKey(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", t.TempDir())
	_, _ = config.Reload()

	_, err := NewWithCredential(context.Background(), CredentialAPIKey)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "no API key configured") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "no API key configured")
	}
}

func TestNewForCommand_APIKeyOverrideForcesAPIKey(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", t.TempDir())
	_, _ = config.Reload()

	viper.Set("api-key", "z_override_key")

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	SetCredentialType(cmd, CredentialBearer) // command declares bearer

	// Should succeed using API key despite bearer declaration.
	c, err := NewForCommand(cmd)
	if err != nil {
		t.Fatalf("NewForCommand with API key override: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestRefreshFailureTransport_ClearsKeychainOnFailure(t *testing.T) {
	profile := "test-refresh-clear-keychain"

	creds := &keyring.Credentials{
		APIKey:       "z_preserved",
		AccessToken:  "old_access",
		RefreshToken: "old_refresh",
		ExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if err := keyring.SetCredentials(profile, creds); err != nil {
		t.Fatalf("seeding keychain: %v", err)
	}

	// Non-invalid_grant RetrieveError triggers clear-and-error path.
	retrieveErr := &oauth2.RetrieveError{
		Response: &http.Response{StatusCode: http.StatusUnauthorized},
	}
	base := roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return nil, retrieveErr
	})

	transport := &refreshFailureTransport{
		base:    base,
		profile: profile,
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", http.NoBody)
	resp, _ := transport.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Verify bearer token was cleared but API key preserved.
	got, err := keyring.GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if got.HasBearerToken() {
		t.Error("bearer token should be cleared after refresh failure")
	}
	if got.APIKey != "z_preserved" {
		t.Errorf("API key should be preserved, got %q", got.APIKey)
	}
}

func TestNewWithCredential_Bearer_NoProfile(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	t.Setenv("HOME", t.TempDir())
	_, _ = config.Reload()

	_, err := NewWithCredential(context.Background(), CredentialBearer)
	if err == nil {
		t.Fatal("expected error for missing profile with bearer credential")
	}
	if !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error = %q, want actionable message mentioning %q", err.Error(), "auth login")
	}
}

func TestNormalizeSDKBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty stays empty", "", ""},
		{"bare host gets /api/v2", "https://api.development.example.com", "https://api.development.example.com/api/v2"},
		{"trailing slash gets /api/v2", "https://api.development.example.com/", "https://api.development.example.com/api/v2"},
		{"already versioned is unchanged", "https://api.development.example.com/api/v2", "https://api.development.example.com/api/v2"},
		{"versioned with trailing slash is trimmed", "https://api.development.example.com/api/v2/", "https://api.development.example.com/api/v2"},
		{"different version is preserved", "https://api.development.example.com/api/v3", "https://api.development.example.com/api/v3"},
		{"two-digit version is preserved", "https://api.development.example.com/api/v10", "https://api.development.example.com/api/v10"},
		{"localhost without version", "http://localhost:8000", "http://localhost:8000/api/v2"},
		{"localhost with version", "http://localhost:8000/api/v2", "http://localhost:8000/api/v2"},
		{"proxied path keeps prefix and gets /api/v2", "https://proxy.example.com/zep", "https://proxy.example.com/zep/api/v2"},
		{"proxied path that already includes /api/v2 is unchanged", "https://proxy.example.com/zep/api/v2", "https://proxy.example.com/zep/api/v2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSDKBaseURL(tt.in)
			if got != tt.want {
				t.Errorf("normalizeSDKBaseURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
