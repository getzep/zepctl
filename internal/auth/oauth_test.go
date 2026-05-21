package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	gokeyring "github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func init() {
	gokeyring.MockInit()
}

func TestRevokeToken(t *testing.T) {
	var receivedClientID, receivedToken string
	revokeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}
		receivedClientID = r.Form.Get("client_id")
		receivedToken = r.Form.Get("token")
		w.WriteHeader(http.StatusOK)
	}))
	defer revokeServer.Close()

	cfg := &OAuthConfig{
		Issuer:   revokeServer.URL,
		ClientID: "test-client-id",
	}

	err := RevokeToken(context.Background(), cfg, "refresh_to_revoke")
	if err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if receivedClientID != "test-client-id" {
		t.Errorf("client_id = %q, want %q", receivedClientID, "test-client-id")
	}
	if receivedToken != "refresh_to_revoke" {
		t.Errorf("token = %q, want %q", receivedToken, "refresh_to_revoke")
	}
}

func TestRevokeToken_ServerError(t *testing.T) {
	revokeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer revokeServer.Close()

	cfg := &OAuthConfig{
		Issuer:   revokeServer.URL,
		ClientID: "test-client-id",
	}

	err := RevokeToken(context.Background(), cfg, "refresh_tok")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestRevokeToken_Accepts2xxCodes(t *testing.T) {
	revokeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer revokeServer.Close()

	cfg := &OAuthConfig{
		Issuer:   revokeServer.URL,
		ClientID: "test-client-id",
	}

	err := RevokeToken(context.Background(), cfg, "refresh_tok")
	if err != nil {
		t.Errorf("RevokeToken should accept 204, got error: %v", err)
	}
}

func TestGenerateAuthURL(t *testing.T) {
	session := NewKeychainSession("test-auth-url")
	cfg := &OAuthConfig{
		Issuer:   "https://test.kinde.com",
		ClientID: "test-client-id",
	}

	authURL := generateAuthURL(cfg, session)
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parsing auth URL: %v", err)
	}

	if parsed.Host != "test.kinde.com" {
		t.Errorf("host = %q, want %q", parsed.Host, "test.kinde.com")
	}

	q := parsed.Query()
	checks := map[string]string{
		"client_id":                "test-client-id",
		"response_type":            "code",
		"code_challenge_method":    "S256",
		"is_use_auth_success_page": "true",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query param %q = %q, want %q", key, got, want)
		}
	}

	if q.Get("code_challenge") == "" {
		t.Error("code_challenge should be present (PKCE)")
	}
	if q.Get("state") == "" {
		t.Error("state should be present")
	}

	scope := q.Get("scope")
	for _, s := range []string{"openid", "profile", "email", "offline"} {
		if !strings.Contains(scope, s) {
			t.Errorf("scope missing %q in %q", s, scope)
		}
	}

	// State and code verifier should be stored in session.
	state, _ := session.GetState()
	if state == "" {
		t.Error("session state should be set after generateAuthURL")
	}
	verifier, _ := session.GetCodeVerifier()
	if verifier == "" {
		t.Error("session code verifier should be set after generateAuthURL")
	}

	if got, ok := q["audience"]; ok {
		t.Errorf("audience param should be absent when cfg.Audience empty, got %v", got)
	}
}

func TestGenerateAuthURL_WithAudience(t *testing.T) {
	session := NewKeychainSession("test-auth-url-audience")
	cfg := &OAuthConfig{
		Issuer:   "https://test.kinde.com",
		ClientID: "test-client-id",
		Audience: "https://api.example.com/api",
	}

	authURL := generateAuthURL(cfg, session)
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parsing auth URL: %v", err)
	}
	if got := parsed.Query().Get("audience"); got != "https://api.example.com/api" {
		t.Errorf("audience param = %q, want %q", got, "https://api.example.com/api")
	}
}

func TestTokenResultFromSession(t *testing.T) {
	session := NewKeychainSession("test-token-result")
	expiry := time.Now().Add(1 * time.Hour)

	if err := session.SetRawToken(&oauth2.Token{
		AccessToken:  "access_xyz",
		RefreshToken: "refresh_xyz",
		Expiry:       expiry,
	}); err != nil {
		t.Fatalf("SetRawToken: %v", err)
	}

	result, err := tokenResultFromSession(session)
	if err != nil {
		t.Fatalf("tokenResultFromSession: %v", err)
	}
	if result.AccessToken != "access_xyz" {
		t.Errorf("AccessToken = %q, want %q", result.AccessToken, "access_xyz")
	}
	if result.RefreshToken != "refresh_xyz" {
		t.Errorf("RefreshToken = %q, want %q", result.RefreshToken, "refresh_xyz")
	}
}

func TestExchangeManualCode(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "manual_access_token",
			"refresh_token": "manual_refresh_token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	cfg := &OAuthConfig{
		Issuer:   tokenServer.URL,
		ClientID: "test-client-id",
	}
	session := NewKeychainSession("test-manual-exchange")

	// Populate state and code verifier as generateAuthURL would.
	_ = session.SetState("test-state")
	_ = session.SetCodeVerifier("test-verifier")

	result, err := exchangeManualCode(context.Background(), cfg, session, "test-auth-code")
	if err != nil {
		t.Fatalf("exchangeManualCode: %v", err)
	}
	if result.AccessToken != "manual_access_token" {
		t.Errorf("AccessToken = %q, want %q", result.AccessToken, "manual_access_token")
	}
}

func TestExchangeManualCode_ServerError(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "authorization code expired",
		})
	}))
	defer tokenServer.Close()

	cfg := &OAuthConfig{
		Issuer:   tokenServer.URL,
		ClientID: "test-client-id",
	}
	session := NewKeychainSession("test-exchange-error")

	_ = session.SetState("test-state")
	_ = session.SetCodeVerifier("test-verifier")

	_, err := exchangeManualCode(context.Background(), cfg, session, "expired-code")
	if err == nil {
		t.Fatal("expected error for expired authorization code")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "token exchange failed") {
		t.Errorf("error = %q, want to contain 'token exchange failed'", errStr)
	}
}

func TestExchangeManualCode_CapturesIDToken(t *testing.T) {
	// Create a minimal JWT-shaped ID token for testing.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		`{"email":"test@example.com","name":"Test User","sub":"user-123"}`,
	))
	testIDToken := header + "." + payload + ".test-signature"

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access_tok",
			"refresh_token": "refresh_tok",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"id_token":      testIDToken,
		})
	}))
	defer tokenServer.Close()

	cfg := &OAuthConfig{
		Issuer:   tokenServer.URL,
		ClientID: "test-client-id",
	}
	session := NewKeychainSession("test-idtoken-capture")

	_ = session.SetState("test-state")
	_ = session.SetCodeVerifier("test-verifier")

	result, err := exchangeManualCode(context.Background(), cfg, session, "test-code")
	if err != nil {
		t.Fatalf("exchangeManualCode: %v", err)
	}

	if result.IDToken != testIDToken {
		t.Errorf("IDToken not captured; got %q", result.IDToken)
	}
	if session.LastIDToken() != testIDToken {
		t.Errorf("session.LastIDToken() = %q, want test ID token", session.LastIDToken())
	}
}

func TestExchangeCode_StateMismatch(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The server should never be reached -- state validation happens
		// client-side before the token exchange request is sent.
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "should_not_get_this",
			"refresh_token": "should_not_get_this",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	cfg := &OAuthConfig{
		Issuer:   tokenServer.URL,
		ClientID: "test-client-id",
	}
	session := NewKeychainSession("test-state-mismatch")

	_ = session.SetState("correct-state")
	_ = session.SetCodeVerifier("test-verifier")

	// Exchange with a wrong state value -- should be rejected.
	err := exchangeCode(context.Background(), cfg, session, "test-auth-code", "wrong-state-value")
	if err == nil {
		t.Fatal("expected error when state does not match")
	}
	if !strings.Contains(err.Error(), "state mismatch") {
		t.Errorf("error = %q, want to contain 'state mismatch'", err.Error())
	}

	// The token should NOT have been persisted.
	tok, _ := session.GetRawToken()
	if tok != nil && tok.AccessToken == "should_not_get_this" {
		t.Error("token should not be stored after state mismatch")
	}
}

func TestRevokeToken_NetworkError(t *testing.T) {
	cfg := &OAuthConfig{
		Issuer:   "http://192.0.2.1", // TEST-NET-1, non-routable
		ClientID: "test-client-id",
	}
	// Use a canceled context to guarantee fast failure.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RevokeToken(ctx, cfg, "refresh_tok")
	if err == nil {
		t.Error("expected error for network failure")
	}
}

func TestGenerateState(t *testing.T) {
	s1 := generateState()
	s2 := generateState()

	if s1 == "" {
		t.Error("state should not be empty")
	}
	if s1 == s2 {
		t.Error("consecutive states should differ")
	}
	if len(s1) < 20 {
		t.Errorf("state too short: %d chars", len(s1))
	}
}

func TestPersistingTokenSource_SkipsRedundantWrites(t *testing.T) {
	session := NewKeychainSession("test-persisting-ts")

	validTok := &oauth2.Token{
		AccessToken:  "access_valid",
		RefreshToken: "refresh_valid",
		Expiry:       time.Now().Add(1 * time.Hour),
		TokenType:    "Bearer",
	}

	// fakeSource always returns the same pointer (simulating ReuseTokenSource
	// returning its cached token).
	calls := 0
	fake := tokenSourceFunc(func() (*oauth2.Token, error) {
		calls++
		return validTok, nil
	})

	pts := &persistingTokenSource{base: fake, session: session}

	// First call: should persist (new pointer).
	tok1, err := pts.Token()
	if err != nil {
		t.Fatalf("Token() #1: %v", err)
	}
	if tok1 != validTok {
		t.Error("expected same pointer from first call")
	}

	// Second call: same pointer, should skip keychain write.
	// We verify by checking that the session still works, and that the
	// optimization doesn't break anything.
	tok2, err := pts.Token()
	if err != nil {
		t.Fatalf("Token() #2: %v", err)
	}
	if tok2 != tok1 {
		t.Error("expected same pointer from second call")
	}
	if calls != 2 {
		t.Errorf("underlying source called %d times, want 2", calls)
	}

	// Simulate a refresh: underlying source returns a new pointer.
	refreshedTok := &oauth2.Token{
		AccessToken:  "access_refreshed",
		RefreshToken: "refresh_refreshed",
		Expiry:       time.Now().Add(1 * time.Hour),
		TokenType:    "Bearer",
	}
	fake = tokenSourceFunc(func() (*oauth2.Token, error) {
		calls++
		return refreshedTok, nil
	})
	pts.base = fake

	tok3, err := pts.Token()
	if err != nil {
		t.Fatalf("Token() #3: %v", err)
	}
	if tok3 != refreshedTok {
		t.Error("expected refreshed pointer")
	}

	// Verify the refreshed token was persisted.
	stored, err := session.GetRawToken()
	if err != nil {
		t.Fatalf("GetRawToken: %v", err)
	}
	if stored.AccessToken != "access_refreshed" {
		t.Errorf("persisted AccessToken = %q, want %q", stored.AccessToken, "access_refreshed")
	}
}

// tokenSourceFunc is a helper that adapts a function to oauth2.TokenSource.
type tokenSourceFunc func() (*oauth2.Token, error)

func (f tokenSourceFunc) Token() (*oauth2.Token, error) { return f() }

func TestNewOAuth2Config(t *testing.T) {
	cfg := &OAuthConfig{
		Issuer:   "https://test.kinde.com",
		ClientID: "test-client-id",
	}

	oauthCfg := newOAuth2Config(cfg)

	if oauthCfg.ClientID != "test-client-id" {
		t.Errorf("ClientID = %q, want %q", oauthCfg.ClientID, "test-client-id")
	}
	if oauthCfg.Endpoint.AuthURL != "https://test.kinde.com/oauth2/auth" {
		t.Errorf("AuthURL = %q", oauthCfg.Endpoint.AuthURL)
	}
	if oauthCfg.Endpoint.TokenURL != "https://test.kinde.com/oauth2/token" {
		t.Errorf("TokenURL = %q", oauthCfg.Endpoint.TokenURL)
	}
}
