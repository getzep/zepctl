package auth

import (
	"testing"
	"time"

	"github.com/getzep/zepctl/internal/keyring"
	gokeyring "github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

func init() {
	gokeyring.MockInit()
}

func TestKeychainSession_SetGetRawToken(t *testing.T) {
	session := NewKeychainSession("test-session-set-get")
	expiry := time.Now().Add(1 * time.Hour).Truncate(time.Second)

	err := session.SetRawToken(&oauth2.Token{
		AccessToken:  "access_123",
		RefreshToken: "refresh_456",
		TokenType:    "Bearer",
		Expiry:       expiry,
	})
	if err != nil {
		t.Fatalf("SetRawToken: %v", err)
	}

	tok, err := session.GetRawToken()
	if err != nil {
		t.Fatalf("GetRawToken: %v", err)
	}
	if tok == nil {
		t.Fatal("GetRawToken returned nil")
	}
	if tok.AccessToken != "access_123" {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "access_123")
	}
	if tok.RefreshToken != "refresh_456" {
		t.Errorf("RefreshToken = %q, want %q", tok.RefreshToken, "refresh_456")
	}
	if tok.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want %q", tok.TokenType, "Bearer")
	}
	if !tok.Expiry.Equal(expiry) {
		t.Errorf("Expiry = %v, want %v", tok.Expiry, expiry)
	}
}

func TestKeychainSession_PreservesAPIKey(t *testing.T) {
	profile := "test-session-preserve-key"
	creds := &keyring.Credentials{APIKey: "z_my_api_key"}
	if err := keyring.SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	session := NewKeychainSession(profile)
	err := session.SetRawToken(&oauth2.Token{
		AccessToken:  "bearer_tok",
		RefreshToken: "refresh_tok",
	})
	if err != nil {
		t.Fatalf("SetRawToken: %v", err)
	}

	got, err := keyring.GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if got.APIKey != "z_my_api_key" {
		t.Errorf("API key should be preserved, got %q", got.APIKey)
	}
	if got.AccessToken != "bearer_tok" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "bearer_tok")
	}
}

func TestKeychainSession_SetRawToken_NilPreservesAPIKey(t *testing.T) {
	profile := "test-session-nil-token"
	creds := &keyring.Credentials{
		APIKey:       "z_my_api_key",
		AccessToken:  "old_access",
		RefreshToken: "old_refresh",
		ExpiresAt:    "2026-04-20T15:30:00Z",
	}
	if err := keyring.SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	session := NewKeychainSession(profile)
	if err := session.SetRawToken(nil); err != nil {
		t.Fatalf("SetRawToken(nil): %v", err)
	}

	got, err := keyring.GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if got.APIKey != "z_my_api_key" {
		t.Errorf("API key should be preserved, got %q", got.APIKey)
	}
	if got.HasBearerToken() {
		t.Errorf("bearer token should be cleared, got AccessToken=%q", got.AccessToken)
	}
	if got.ExpiresAt != "" {
		t.Errorf("ExpiresAt should be cleared, got %q", got.ExpiresAt)
	}
}

func TestKeychainSession_GetRawToken_NoBearerToken(t *testing.T) {
	profile := "test-session-no-bearer"
	creds := &keyring.Credentials{APIKey: "z_key_only"}
	if err := keyring.SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	session := NewKeychainSession(profile)
	tok, err := session.GetRawToken()
	if err != nil {
		t.Fatalf("GetRawToken: %v", err)
	}
	if tok != nil {
		t.Errorf("expected nil token for profile without bearer token, got %+v", tok)
	}
}

func TestKeychainSession_EphemeralState(t *testing.T) {
	session := NewKeychainSession("test-session-state")

	// State starts empty.
	state, err := session.GetState()
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state != "" {
		t.Errorf("initial state = %q, want empty", state)
	}

	if err := session.SetState("csrf_state_123"); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	state, _ = session.GetState()
	if state != "csrf_state_123" {
		t.Errorf("state = %q, want %q", state, "csrf_state_123")
	}
}

func TestKeychainSession_EphemeralCodeVerifier(t *testing.T) {
	session := NewKeychainSession("test-session-verifier")

	if err := session.SetCodeVerifier("verifier_abc"); err != nil {
		t.Fatalf("SetCodeVerifier: %v", err)
	}
	v, _ := session.GetCodeVerifier()
	if v != "verifier_abc" {
		t.Errorf("code verifier = %q, want %q", v, "verifier_abc")
	}
}

// TestKeychainSession_SetRawToken_ReplacesExisting verifies that storing a
// new token fully replaces the previous one (refresh token rotation).
func TestKeychainSession_SetRawToken_ReplacesExisting(t *testing.T) {
	session := NewKeychainSession("test-session-replace")

	// Store first token.
	if err := session.SetRawToken(&oauth2.Token{
		AccessToken:  "first_access",
		RefreshToken: "first_refresh",
	}); err != nil {
		t.Fatalf("SetRawToken (first): %v", err)
	}

	// Store second token (simulates refresh rotation).
	if err := session.SetRawToken(&oauth2.Token{
		AccessToken:  "second_access",
		RefreshToken: "second_refresh",
	}); err != nil {
		t.Fatalf("SetRawToken (second): %v", err)
	}

	tok, err := session.GetRawToken()
	if err != nil {
		t.Fatalf("GetRawToken: %v", err)
	}
	if tok.AccessToken != "second_access" {
		t.Errorf("AccessToken = %q, want %q", tok.AccessToken, "second_access")
	}
	if tok.RefreshToken != "second_refresh" {
		t.Errorf("RefreshToken = %q, want %q", tok.RefreshToken, "second_refresh")
	}

	// Verify the first token is fully gone from the keychain.
	creds, err := keyring.GetCredentials("test-session-replace")
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if creds.AccessToken == "first_access" {
		t.Error("first access token should have been replaced")
	}
	if creds.RefreshToken == "first_refresh" {
		t.Error("first refresh token should have been replaced")
	}
}
