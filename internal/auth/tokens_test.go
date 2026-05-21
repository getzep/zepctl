package auth

import (
	"testing"
	"time"

	"github.com/getzep/zepctl/internal/keyring"
	gokeyring "github.com/zalando/go-keyring"
)

func init() {
	gokeyring.MockInit()
}

func TestStoreBearerToken_PreservesAPIKey(t *testing.T) {
	profile := "test-store-preserve"
	creds := &keyring.Credentials{APIKey: "z_existing_key"}
	if err := keyring.SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	result := &TokenResult{
		AccessToken:  "new_bearer",
		RefreshToken: "new_refresh",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}

	if err := StoreBearerToken(profile, result, "user@example.com"); err != nil {
		t.Fatalf("StoreBearerToken: %v", err)
	}

	got, err := keyring.GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}

	if got.APIKey != "z_existing_key" {
		t.Errorf("APIKey = %q, want %q (should be preserved)", got.APIKey, "z_existing_key")
	}
	if got.AccessToken != "new_bearer" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "new_bearer")
	}
	if got.UserEmail != "user@example.com" {
		t.Errorf("UserEmail = %q, want %q", got.UserEmail, "user@example.com")
	}
}

func TestClearBearerToken_PreservesAPIKey(t *testing.T) {
	profile := "test-clear-preserve"
	creds := &keyring.Credentials{
		APIKey:       "z_keep_this",
		AccessToken:  "clear_this",
		RefreshToken: "clear_this_too",
		ExpiresAt:    "2026-04-20T15:30:00Z",
		UserEmail:    "clear@example.com",
	}
	if err := keyring.SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	if err := ClearBearerToken(profile); err != nil {
		t.Fatalf("ClearBearerToken: %v", err)
	}

	got, err := keyring.GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}

	if got.APIKey != "z_keep_this" {
		t.Errorf("APIKey = %q, want %q (should be preserved)", got.APIKey, "z_keep_this")
	}
	if got.HasBearerToken() {
		t.Error("bearer token should be cleared")
	}
	if got.UserEmail != "" {
		t.Errorf("UserEmail = %q, want empty", got.UserEmail)
	}
}

func TestStoreBearerToken_NewProfile(t *testing.T) {
	profile := "test-store-new-profile"

	result := &TokenResult{
		AccessToken:  "new_bearer",
		RefreshToken: "new_refresh",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}

	if err := StoreBearerToken(profile, result, "user@example.com"); err != nil {
		t.Fatalf("StoreBearerToken: %v", err)
	}

	got, err := keyring.GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if got.AccessToken != "new_bearer" {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, "new_bearer")
	}
	if got.RefreshToken != "new_refresh" {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, "new_refresh")
	}
	if got.UserEmail != "user@example.com" {
		t.Errorf("UserEmail = %q, want %q", got.UserEmail, "user@example.com")
	}
	// No API key should be set on a fresh profile.
	if got.HasAPIKey() {
		t.Errorf("new profile should not have API key, got %q", got.APIKey)
	}
}

func TestClearBearerToken_NonexistentProfile(t *testing.T) {
	// Clearing bearer token on a profile that doesn't exist should not error.
	if err := ClearBearerToken("test-clear-nonexistent"); err != nil {
		t.Errorf("ClearBearerToken on nonexistent profile: %v", err)
	}
}
