package keyring

import (
	"encoding/json"
	"testing"
	"time"

	gokeyring "github.com/zalando/go-keyring"
)

func init() {
	// Use in-memory mock keyring for tests.
	gokeyring.MockInit()
}

func TestCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt string
		want      bool
	}{
		{"empty expiry", "", true},
		{"invalid format", "not-a-date", true},
		{"past time", time.Now().Add(-1 * time.Hour).Format(time.RFC3339), true},
		{"future time", time.Now().Add(1 * time.Hour).Format(time.RFC3339), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Credentials{ExpiresAt: tt.expiresAt}
			if got := c.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredentials_ExpiresIn(t *testing.T) {
	t.Run("empty expiry returns zero", func(t *testing.T) {
		c := &Credentials{}
		if d := c.ExpiresIn(); d != 0 {
			t.Errorf("ExpiresIn() = %v, want 0", d)
		}
	})

	t.Run("past expiry returns zero", func(t *testing.T) {
		c := &Credentials{ExpiresAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339)}
		if d := c.ExpiresIn(); d != 0 {
			t.Errorf("ExpiresIn() = %v, want 0", d)
		}
	})

	t.Run("future expiry returns positive duration", func(t *testing.T) {
		c := &Credentials{ExpiresAt: time.Now().Add(30 * time.Minute).Format(time.RFC3339)}
		d := c.ExpiresIn()
		if d <= 29*time.Minute || d > 30*time.Minute {
			t.Errorf("ExpiresIn() = %v, want ~30m", d)
		}
	})
}

func TestCredentials_HasBearerToken(t *testing.T) {
	c := &Credentials{}
	if c.HasBearerToken() {
		t.Error("empty credentials should not have bearer token")
	}
	c.AccessToken = "tok"
	if !c.HasBearerToken() {
		t.Error("should have bearer token when access_token is set")
	}
}

func TestCredentials_HasAPIKey(t *testing.T) {
	c := &Credentials{}
	if c.HasAPIKey() {
		t.Error("empty credentials should not have API key")
	}
	c.APIKey = "z_key"
	if !c.HasAPIKey() {
		t.Error("should have API key when api_key is set")
	}
}

func TestSetAndGetCredentials(t *testing.T) {
	profile := "test-json-profile"

	creds := &Credentials{
		APIKey:       "z_test123",
		AccessToken:  "eyJhbGci.test",
		RefreshToken: "refresh_abc",
		ExpiresAt:    "2026-04-20T15:30:00Z",
		UserEmail:    "fred@frobozz.infocom",
	}

	if err := SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	got, err := GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}

	if got.APIKey != creds.APIKey {
		t.Errorf("APIKey = %q, want %q", got.APIKey, creds.APIKey)
	}
	if got.AccessToken != creds.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, creds.AccessToken)
	}
	if got.RefreshToken != creds.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, creds.RefreshToken)
	}
	if got.ExpiresAt != creds.ExpiresAt {
		t.Errorf("ExpiresAt = %q, want %q", got.ExpiresAt, creds.ExpiresAt)
	}
	if got.UserEmail != creds.UserEmail {
		t.Errorf("UserEmail = %q, want %q", got.UserEmail, creds.UserEmail)
	}
}

func TestGetCredentials_OnlyAPIKey(t *testing.T) {
	profile := "test-apikey-only"

	creds := &Credentials{APIKey: "z_onlykey"}
	if err := SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	got, err := GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}

	if got.APIKey != "z_onlykey" {
		t.Errorf("APIKey = %q, want %q", got.APIKey, "z_onlykey")
	}
	if got.HasBearerToken() {
		t.Error("should not have bearer token")
	}
}

func TestGetCredentials_OnlyBearerToken(t *testing.T) {
	profile := "test-bearer-only"

	creds := &Credentials{
		AccessToken:  "bearer_tok",
		RefreshToken: "refresh_tok",
		ExpiresAt:    "2026-04-20T15:30:00Z",
		UserEmail:    "user@example.com",
	}
	if err := SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	got, err := GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}

	if got.HasAPIKey() {
		t.Error("should not have API key")
	}
	if !got.HasBearerToken() {
		t.Error("should have bearer token")
	}
	if got.UserEmail != "user@example.com" {
		t.Errorf("UserEmail = %q, want %q", got.UserEmail, "user@example.com")
	}
}

func TestGetCredentials_LegacyMigration(t *testing.T) {
	profile := "test-legacy"

	// Store a raw API key string (legacy format)
	if err := gokeyring.Set(serviceName, profile, "z_legacy_key_123"); err != nil {
		t.Fatalf("setting raw keyring value: %v", err)
	}

	// GetCredentials should parse it as legacy and migrate
	got, err := GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}

	if got.APIKey != "z_legacy_key_123" {
		t.Errorf("APIKey = %q, want %q", got.APIKey, "z_legacy_key_123")
	}

	// Verify the migration happened: re-read raw value, should be JSON now
	raw, err := gokeyring.Get(serviceName, profile)
	if err != nil {
		t.Fatalf("reading raw keyring after migration: %v", err)
	}

	var migrated Credentials
	if err := json.Unmarshal([]byte(raw), &migrated); err != nil {
		t.Fatalf("migrated value is not valid JSON: %v (raw: %q)", err, raw)
	}
	if migrated.APIKey != "z_legacy_key_123" {
		t.Errorf("migrated APIKey = %q, want %q", migrated.APIKey, "z_legacy_key_123")
	}
}

func TestGetCredentials_NotFound(t *testing.T) {
	got, err := GetCredentials("nonexistent-profile")
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if got.HasAPIKey() || got.HasBearerToken() {
		t.Error("expected empty credentials for nonexistent profile")
	}
}

func TestSet_PreservesBearerToken(t *testing.T) {
	profile := "test-set-preserve"

	// Start with bearer token only
	creds := &Credentials{
		AccessToken:  "bearer_tok",
		RefreshToken: "refresh_tok",
		ExpiresAt:    "2026-04-20T15:30:00Z",
		UserEmail:    "user@example.com",
	}
	if err := SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	// Use Set() to add an API key -- should preserve bearer token
	if err := Set(profile, "z_new_key"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}

	if got.APIKey != "z_new_key" {
		t.Errorf("APIKey = %q, want %q", got.APIKey, "z_new_key")
	}
	if got.AccessToken != "bearer_tok" {
		t.Errorf("AccessToken = %q, want %q (should be preserved)", got.AccessToken, "bearer_tok")
	}
}

func TestGet_BackwardsCompatible(t *testing.T) {
	profile := "test-get-compat"

	creds := &Credentials{APIKey: "z_compat_key"}
	if err := SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	key, err := Get(profile)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if key != "z_compat_key" {
		t.Errorf("Get() = %q, want %q", key, "z_compat_key")
	}
}

func TestDelete(t *testing.T) {
	profile := "test-delete"

	creds := &Credentials{
		APIKey:      "z_del_key",
		AccessToken: "del_tok",
	}
	if err := SetCredentials(profile, creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	if err := Delete(profile); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err := GetCredentials(profile)
	if err != nil {
		t.Fatalf("GetCredentials after delete: %v", err)
	}
	if got.HasAPIKey() || got.HasBearerToken() {
		t.Error("expected empty credentials after delete")
	}
}

func TestDelete_NotFound(t *testing.T) {
	if err := Delete("nonexistent-for-delete"); err != nil {
		t.Errorf("Delete of nonexistent profile should not error: %v", err)
	}
}
