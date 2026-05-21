package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/keyring"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	gokeyring "github.com/zalando/go-keyring"
)

func init() {
	gokeyring.MockInit()
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"z_abc123def456", "z_...f456"},
		{"short", "short"},
		{"z_7x2f", "z_7x2f"},
		{"z_abcdefghijklmnop", "z_...mnop"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maskKey(tt.input)
			if got != tt.want {
				t.Errorf("maskKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{30 * time.Second, "30 seconds"},
		{47 * time.Minute, "47 minutes"},
		{90 * time.Minute, "1h 30m"},
		{1 * time.Second, "1 seconds"},
		{2 * time.Hour, "2 hours"},
		{time.Hour + 15*time.Minute, "1h 15m"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.input)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// writeTestConfig writes a minimal config file to tmpDir so config.Load works.
func writeTestConfig(t *testing.T, tmpDir string) {
	t.Helper()
	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := "current-profile: test\nprofiles:\n  - name: test\n    api-url: https://api.getzep.com\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}

// resetCmdFlags clears named flag values to their registered defaults and
// resets the Changed bit. Cobra commands are package-level globals;
// without this, flag.Set state survives across tests.
func resetCmdFlags(t *testing.T, cmd *cobra.Command, names ...string) {
	t.Helper()
	for _, n := range names {
		if f := cmd.Flags().Lookup(n); f != nil {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		}
	}
}

// captureStdout runs fn and returns everything written to os.Stdout.
// Uses defer to restore os.Stdout even if fn panics.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() {
		w.Close()
		os.Stdout = old
	}()

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// setupAutoSelectTest sets up viper, HOME, and a mock server for
// autoSelectProject tests. The mock /api/web/v1/authenticate endpoint
// returns accountUUID alongside the projects produced by projectsFn.
func setupAutoSelectTest(t *testing.T, accountUUID string, projectsFn func() ([]projectInfo, int)) (*config.Config, *config.Profile) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	mux := http.NewServeMux()
	mux.HandleFunc("/api/web/v1/authenticate", func(w http.ResponseWriter, _ *http.Request) {
		projects, status := projectsFn()
		if status != 0 && status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account_uuid": accountUUID,
			"projects":     projects,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	viper.Set("api-url", srv.URL)

	cfg := &config.Config{
		CurrentProfile: "test",
		Profiles:       []config.Profile{{Name: "test"}},
	}
	profile := cfg.GetProfile("test")
	return cfg, profile
}

// runAuthStatus sets up a config with a "test" profile, seeds the keychain
// with creds, runs authStatusCmd, and returns the captured stdout.
func runAuthStatus(t *testing.T, creds *keyring.Credentials) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeTestConfig(t, tmpDir)
	_, _ = config.Reload()

	if err := keyring.SetCredentials("test", creds); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	cmd := &cobra.Command{Use: "test"}
	var runErr error
	got := captureStdout(t, func() {
		runErr = authStatusCmd.RunE(cmd, nil)
	})
	if runErr != nil {
		t.Fatalf("auth status: %v", runErr)
	}
	return got
}

func TestAutoSelectProject_SingleProject(t *testing.T) {
	cfg, profile := setupAutoSelectTest(t, "acc-123", func() ([]projectInfo, int) {
		return []projectInfo{{UUID: "proj-1", Name: "My Project"}}, http.StatusOK
	})

	err := autoSelectProject(context.Background(), cfg, profile, "test-token")
	if err != nil {
		t.Fatalf("autoSelectProject: %v", err)
	}
	if profile.ProjectUUID != "proj-1" {
		t.Errorf("ProjectUUID = %q, want %q", profile.ProjectUUID, "proj-1")
	}
	if profile.AccountUUID != "acc-123" {
		t.Errorf("AccountUUID = %q, want %q", profile.AccountUUID, "acc-123")
	}
}

// TestAutoSelectProject_NoProjects verifies that when authenticate returns
// zero projects, autoSelectProject errors out -- but the account UUID is
// still persisted to the profile for follow-up commands.
func TestAutoSelectProject_NoProjects(t *testing.T) {
	cfg, profile := setupAutoSelectTest(t, "acc-456", func() ([]projectInfo, int) {
		return []projectInfo{}, http.StatusOK
	})

	err := autoSelectProject(context.Background(), cfg, profile, "test-token")
	if err == nil {
		t.Fatal("expected error for no projects")
	}
	if !strings.Contains(err.Error(), "no projects found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "no projects found")
	}
	if profile.AccountUUID != "acc-456" {
		t.Errorf("AccountUUID = %q, want %q (should be persisted even when no projects)",
			profile.AccountUUID, "acc-456")
	}
}

func TestAutoSelectProject_AuthenticateFails(t *testing.T) {
	cfg, profile := setupAutoSelectTest(t, "acc-789", func() ([]projectInfo, int) {
		return nil, http.StatusInternalServerError
	})

	err := autoSelectProject(context.Background(), cfg, profile, "test-token")
	if err == nil {
		t.Fatal("expected error from failed authenticate")
	}
	if profile.AccountUUID != "" {
		t.Errorf("AccountUUID = %q, want empty (no save when authenticate fails)", profile.AccountUUID)
	}
}

func TestAuthStatusOutput_BearerOnly(t *testing.T) {
	got := runAuthStatus(t, &keyring.Credentials{
		AccessToken:  "eyJhbGci.valid_tok",
		RefreshToken: "refresh_tok",
		ExpiresAt:    time.Now().Add(47 * time.Minute).Format(time.RFC3339),
		UserEmail:    "test@example.com",
	})

	checks := []string{
		"API key:       not configured",
		"valid (expires in",
		"test@example.com",
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestAuthStatusOutput_BothCredentials(t *testing.T) {
	got := runAuthStatus(t, &keyring.Credentials{
		APIKey:       "z_test_key_12345",
		AccessToken:  "eyJhbGci.test",
		RefreshToken: "refresh_tok",
		ExpiresAt:    time.Now().Add(47 * time.Minute).Format(time.RFC3339),
		UserEmail:    "fred@frobozz.infocom",
	})

	checks := []string{
		"z_...2345",            // masked API key
		"valid (expires in",    // bearer status
		"fred@frobozz.infocom", // user email
	}
	for _, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}

func TestAuthStatusOutput_APIKeyOnly(t *testing.T) {
	got := runAuthStatus(t, &keyring.Credentials{
		APIKey: "z_only_key_9999",
	})

	if !strings.Contains(got, "z_...9999") {
		t.Errorf("output missing masked API key:\n%s", got)
	}
	if !strings.Contains(got, "Bearer token:  not configured") {
		t.Errorf("output should show bearer as not configured:\n%s", got)
	}
}

// TestLogoutRevocationFailure_StillClearsToken verifies that auth logout
// clears bearer token fields even when the revocation request fails.
func TestLogoutRevocationFailure_StillClearsToken(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeTestConfig(t, tmpDir)
	_, _ = config.Reload()

	// Seed keychain with both credentials and a refresh token that
	// will trigger the revocation path.
	if err := keyring.SetCredentials("test", &keyring.Credentials{
		APIKey:       "z_preserved_key",
		AccessToken:  "old_access",
		RefreshToken: "old_refresh",
		ExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
		UserEmail:    "user@example.com",
	}); err != nil {
		t.Fatalf("SetCredentials: %v", err)
	}

	// Use a canceled context so RevokeToken (which hits the real Kinde
	// issuer URL) fails immediately. ClearBearerToken does not use the
	// context, so the rest of logout proceeds.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(ctx)

	var runErr error
	captureStdout(t, func() {
		runErr = authLogoutCmd.RunE(cmd, nil)
	})
	if runErr != nil {
		t.Fatalf("auth logout should succeed even when revocation fails: %v", runErr)
	}

	// Verify bearer token was cleared but API key preserved.
	creds, err := keyring.GetCredentials("test")
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if creds.HasBearerToken() {
		t.Error("bearer token should be cleared after logout")
	}
	if creds.APIKey != "z_preserved_key" {
		t.Errorf("API key should be preserved, got %q", creds.APIKey)
	}
	if creds.UserEmail != "" {
		t.Errorf("UserEmail should be cleared, got %q", creds.UserEmail)
	}
}

func resetAuthLoginFlags(t *testing.T) {
	t.Helper()
	resetCmdFlags(t, authLoginCmd, "no-browser", "env")
}

// TestAuthLogin_BootstrapWithEnv_AppliesEnvToNewProfile verifies that
// `zepctl --profile foo auth login --env development` creates the new
// profile with api-url/oauth-issuer/oauth-client-id taken from the
// environment preset. The OAuth flow itself is short-circuited via a
// canceled context -- by that point the profile has already been written
// to disk, which is what we're asserting.
func TestAuthLogin_BootstrapWithEnv_AppliesEnvToNewProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yaml := `current-profile: ""
profiles: []
environments:
  - name: development
    api-url: https://api.dev.example.com
    oauth-issuer: https://issuer.example.com
    oauth-client-id: dev-client-id
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()
	resetAuthLoginFlags(t)
	t.Cleanup(func() { resetAuthLoginFlags(t) })

	if err := authLoginCmd.Flags().Set("env", "development"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	viper.Set("profile", "dev")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(ctx)
	// Mirror authLoginCmd's flagset onto our test cmd so RunE can read
	// --env and --no-browser via cmd.Flags().
	cmd.Flags().AddFlagSet(authLoginCmd.Flags())

	// Login will fail (canceled ctx + no callback server), but the profile
	// is created and saved before the OAuth flow begins.
	captureStdout(t, func() {
		_ = authLoginCmd.RunE(cmd, nil)
	})

	cfg, _ := config.Reload()
	p := cfg.GetProfile("dev")
	if p == nil {
		t.Fatal("profile 'dev' not created from env preset")
	}
	if p.APIURL != "https://api.dev.example.com" {
		t.Errorf("APIURL = %q, want from env", p.APIURL)
	}
	if p.OAuthIssuer != "https://issuer.example.com" {
		t.Errorf("OAuthIssuer = %q, want from env", p.OAuthIssuer)
	}
	if p.OAuthClientID != "dev-client-id" {
		t.Errorf("OAuthClientID = %q, want from env", p.OAuthClientID)
	}
}

func TestAuthLogin_BootstrapWithUnknownEnv_ErrorsBeforeAuth(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Empty config, no environments defined.
	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("current-profile: \"\"\nprofiles: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()
	resetAuthLoginFlags(t)
	t.Cleanup(func() { resetAuthLoginFlags(t) })

	if err := authLoginCmd.Flags().Set("env", "missing-env"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	viper.Set("profile", "x")

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	cmd.Flags().AddFlagSet(authLoginCmd.Flags())

	err := authLoginCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error for unknown environment")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to mention 'not found'", err.Error())
	}

	// And the profile must NOT have been created -- we want the failure to
	// happen before any persistent change to config.
	cfg, _ := config.Reload()
	if cfg.GetProfile("x") != nil {
		t.Error("profile 'x' should not be created on unknown env")
	}
}

func TestAuthLogin_ExistingProfileWithEnv_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	yaml := `current-profile: existing
profiles:
  - name: existing
    api-url: https://existing.example.com
environments:
  - name: development
    api-url: https://api.dev.example.com
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()
	resetAuthLoginFlags(t)
	t.Cleanup(func() { resetAuthLoginFlags(t) })

	if err := authLoginCmd.Flags().Set("env", "development"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	cmd.Flags().AddFlagSet(authLoginCmd.Flags())

	err := authLoginCmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("expected error: --env on existing profile is ambiguous")
	}
	if !strings.Contains(err.Error(), "update-profile") {
		t.Errorf("error = %q, want to redirect to 'update-profile'", err.Error())
	}

	// Profile must be unchanged.
	cfg, _ := config.Reload()
	p := cfg.GetProfile("existing")
	if p == nil || p.APIURL != "https://existing.example.com" {
		t.Errorf("profile mutated: %+v", p)
	}
}

func TestAuthStatusOutput_BearerExpired(t *testing.T) {
	got := runAuthStatus(t, &keyring.Credentials{
		AccessToken:  "expired_tok",
		RefreshToken: "refresh_tok",
		ExpiresAt:    time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
	})

	if !strings.Contains(got, "Bearer token:  expired") {
		t.Errorf("output should show bearer as expired:\n%s", got)
	}
	if !strings.Contains(got, "API key:       not configured") {
		t.Errorf("output should show API key as not configured:\n%s", got)
	}
}
