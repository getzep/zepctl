package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/keyring"
	"github.com/spf13/viper"
)

// TestAddProfile_ExistingWithAPIKey_ReturnsError verifies that
// "config add-profile <name>" returns an error when the profile already
// has an API key.
func TestAddProfile_ExistingWithAPIKey_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeTestConfig(t, tmpDir) // creates "test" profile
	_, _ = config.Reload()

	// Seed keychain with an API key for the existing profile.
	if err := keyring.Set("test", "z_existing_key"); err != nil {
		t.Fatalf("keyring.Set: %v", err)
	}

	err := configAddProfileCmd.RunE(configAddProfileCmd, []string{"test"})
	if err == nil {
		t.Fatal("expected error when profile already has API key")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "already exists")
	}
}

// TestAddProfile_NoAPIKey verifies that --no-api-key creates a
// bearer-only profile without prompting for or storing an API key.
func TestAddProfile_NoAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("current-profile: \"\"\nprofiles: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()

	if err := configAddProfileCmd.Flags().Set("no-api-key", "true"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configAddProfileCmd.Flags().Set("api-url", "https://api.dev.example.com"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configAddProfileCmd.Flags().Set("oauth-issuer", "https://dev.kinde.com"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	t.Cleanup(func() {
		_ = configAddProfileCmd.Flags().Set("no-api-key", "false")
		_ = configAddProfileCmd.Flags().Set("api-url", "")
		_ = configAddProfileCmd.Flags().Set("oauth-issuer", "")
	})

	if err := configAddProfileCmd.RunE(configAddProfileCmd, []string{"bearer-only"}); err != nil {
		t.Fatalf("add-profile: %v", err)
	}

	cfg, err := config.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	p := cfg.GetProfile("bearer-only")
	if p == nil {
		t.Fatal("profile 'bearer-only' missing")
	}
	if p.OAuthIssuer != "https://dev.kinde.com" {
		t.Errorf("OAuthIssuer = %q, want %q", p.OAuthIssuer, "https://dev.kinde.com")
	}

	creds, _ := keyring.GetCredentials("bearer-only")
	if creds.HasAPIKey() {
		t.Errorf("expected no API key in keychain, got %q", creds.APIKey)
	}
}

// TestUpdateProfile_OAuthFields verifies that --oauth-issuer and
// --oauth-client-id flags are persisted to the profile and that
// passing an empty string clears the override.
func TestUpdateProfile_OAuthFields(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeTestConfig(t, tmpDir) // creates "test" profile
	_, _ = config.Reload()

	if err := configUpdateProfileCmd.Flags().Set("oauth-issuer", "https://dev.kinde.com"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configUpdateProfileCmd.Flags().Set("oauth-client-id", "dev-client"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	t.Cleanup(func() {
		_ = configUpdateProfileCmd.Flags().Set("oauth-issuer", "")
		_ = configUpdateProfileCmd.Flags().Set("oauth-client-id", "")
	})

	if err := configUpdateProfileCmd.RunE(configUpdateProfileCmd, []string{"test"}); err != nil {
		t.Fatalf("update-profile: %v", err)
	}

	cfg, err := config.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	p := cfg.GetProfile("test")
	if p == nil {
		t.Fatal("profile 'test' missing after update")
	}
	if p.OAuthIssuer != "https://dev.kinde.com" {
		t.Errorf("OAuthIssuer = %q, want %q", p.OAuthIssuer, "https://dev.kinde.com")
	}
	if p.OAuthClientID != "dev-client" {
		t.Errorf("OAuthClientID = %q, want %q", p.OAuthClientID, "dev-client")
	}

	// Now clear both with explicit empty strings.
	if err := configUpdateProfileCmd.Flags().Set("oauth-issuer", ""); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configUpdateProfileCmd.Flags().Set("oauth-client-id", ""); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configUpdateProfileCmd.RunE(configUpdateProfileCmd, []string{"test"}); err != nil {
		t.Fatalf("update-profile (clear): %v", err)
	}
	cfg, _ = config.Reload()
	p = cfg.GetProfile("test")
	if p.OAuthIssuer != "" {
		t.Errorf("OAuthIssuer = %q, want cleared", p.OAuthIssuer)
	}
	if p.OAuthClientID != "" {
		t.Errorf("OAuthClientID = %q, want cleared", p.OAuthClientID)
	}
}

func resetAddProfileFlags(t *testing.T) {
	t.Helper()
	resetCmdFlags(t, configAddProfileCmd,
		"api-key", "api-url", "oauth-issuer", "oauth-client-id", "oauth-audience", "env", "no-api-key")
}

func resetUpdateProfileFlags(t *testing.T) {
	t.Helper()
	resetCmdFlags(t, configUpdateProfileCmd,
		"api-key", "api-url", "project", "account", "oauth-issuer", "oauth-client-id", "oauth-audience", "env")
}

// writeConfigYAML writes a verbatim YAML config to tmpDir/.zepctl/config.yaml.
func writeConfigYAML(t *testing.T, tmpDir, yaml string) {
	t.Helper()
	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAddProfile_WithEnv_AppliesPreset(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeConfigYAML(t, tmpDir, `current-profile: ""
profiles: []
environments:
  - name: development
    api-url: https://api.dev.example.com
    oauth-issuer: https://issuer.example.com
    oauth-client-id: dev-client-id
    oauth-audience: https://api.dev.example.com/api
`)
	_, _ = config.Reload()
	resetAddProfileFlags(t)
	t.Cleanup(func() { resetAddProfileFlags(t) })

	if err := configAddProfileCmd.Flags().Set("no-api-key", "true"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configAddProfileCmd.Flags().Set("env", "development"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := configAddProfileCmd.RunE(configAddProfileCmd, []string{"dev"}); err != nil {
		t.Fatalf("add-profile: %v", err)
	}

	cfg, _ := config.Reload()
	p := cfg.GetProfile("dev")
	if p == nil {
		t.Fatal("profile 'dev' missing")
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
	if p.OAuthAudience != "https://api.dev.example.com/api" {
		t.Errorf("OAuthAudience = %q, want from env", p.OAuthAudience)
	}
}

func TestAddProfile_WithEnv_ExplicitFlagOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeConfigYAML(t, tmpDir, `current-profile: ""
profiles: []
environments:
  - name: development
    api-url: https://api.dev.example.com
    oauth-issuer: https://issuer.example.com
    oauth-client-id: dev-client-id
`)
	_, _ = config.Reload()
	resetAddProfileFlags(t)
	t.Cleanup(func() { resetAddProfileFlags(t) })

	if err := configAddProfileCmd.Flags().Set("no-api-key", "true"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configAddProfileCmd.Flags().Set("env", "development"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	// Explicit --api-url should win over the env preset.
	if err := configAddProfileCmd.Flags().Set("api-url", "http://localhost:8001"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := configAddProfileCmd.RunE(configAddProfileCmd, []string{"local"}); err != nil {
		t.Fatalf("add-profile: %v", err)
	}

	cfg, _ := config.Reload()
	p := cfg.GetProfile("local")
	if p.APIURL != "http://localhost:8001" {
		t.Errorf("APIURL = %q, want explicit override", p.APIURL)
	}
	// OAuth fields untouched -- still come from the env.
	if p.OAuthIssuer != "https://issuer.example.com" {
		t.Errorf("OAuthIssuer = %q, want from env", p.OAuthIssuer)
	}
	if p.OAuthClientID != "dev-client-id" {
		t.Errorf("OAuthClientID = %q, want from env", p.OAuthClientID)
	}
}

func TestAddProfile_WithEnv_UnknownErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	emptyConfigDir(t, tmpDir)
	_, _ = config.Reload()
	resetAddProfileFlags(t)
	t.Cleanup(func() { resetAddProfileFlags(t) })

	if err := configAddProfileCmd.Flags().Set("no-api-key", "true"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configAddProfileCmd.Flags().Set("env", "missing-env"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	err := configAddProfileCmd.RunE(configAddProfileCmd, []string{"x"})
	if err == nil {
		t.Fatal("expected error for unknown environment")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to mention 'not found'", err.Error())
	}
}

func TestUpdateProfile_WithEnv_ReplacesAllThreeFields(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeConfigYAML(t, tmpDir, `current-profile: dev
profiles:
  - name: dev
    api-url: https://stale.example.com
    oauth-issuer: https://stale-issuer.example.com
    oauth-client-id: stale-client
environments:
  - name: development
    api-url: https://api.dev.example.com
    oauth-issuer: https://issuer.example.com
    oauth-client-id: fresh-client-id
`)
	_, _ = config.Reload()
	resetUpdateProfileFlags(t)
	t.Cleanup(func() { resetUpdateProfileFlags(t) })

	if err := configUpdateProfileCmd.Flags().Set("env", "development"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := configUpdateProfileCmd.RunE(configUpdateProfileCmd, []string{"dev"}); err != nil {
		t.Fatalf("update-profile: %v", err)
	}

	cfg, _ := config.Reload()
	p := cfg.GetProfile("dev")
	if p.APIURL != "https://api.dev.example.com" {
		t.Errorf("APIURL = %q, want replaced", p.APIURL)
	}
	if p.OAuthIssuer != "https://issuer.example.com" {
		t.Errorf("OAuthIssuer = %q, want replaced", p.OAuthIssuer)
	}
	if p.OAuthClientID != "fresh-client-id" {
		t.Errorf("OAuthClientID = %q, want replaced", p.OAuthClientID)
	}
}

func TestUpdateProfile_WithEnv_ExplicitFlagOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeConfigYAML(t, tmpDir, `current-profile: dev
profiles:
  - name: dev
    api-url: https://stale.example.com
environments:
  - name: development
    api-url: https://api.dev.example.com
    oauth-issuer: https://issuer.example.com
    oauth-client-id: dev-client-id
`)
	_, _ = config.Reload()
	resetUpdateProfileFlags(t)
	t.Cleanup(func() { resetUpdateProfileFlags(t) })

	if err := configUpdateProfileCmd.Flags().Set("env", "development"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configUpdateProfileCmd.Flags().Set("oauth-client-id", "override-client"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := configUpdateProfileCmd.RunE(configUpdateProfileCmd, []string{"dev"}); err != nil {
		t.Fatalf("update-profile: %v", err)
	}

	cfg, _ := config.Reload()
	p := cfg.GetProfile("dev")
	if p.APIURL != "https://api.dev.example.com" {
		t.Errorf("APIURL = %q, want from env", p.APIURL)
	}
	if p.OAuthClientID != "override-client" {
		t.Errorf("OAuthClientID = %q, want explicit override", p.OAuthClientID)
	}
}

func TestUpdateProfile_WithEnv_UnknownErrors(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	writeConfigYAML(t, tmpDir, `current-profile: dev
profiles:
  - name: dev
`)
	_, _ = config.Reload()
	resetUpdateProfileFlags(t)
	t.Cleanup(func() { resetUpdateProfileFlags(t) })

	if err := configUpdateProfileCmd.Flags().Set("env", "missing-env"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	err := configUpdateProfileCmd.RunE(configUpdateProfileCmd, []string{"dev"})
	if err == nil {
		t.Fatal("expected error for unknown environment")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to mention 'not found'", err.Error())
	}
}

// TestAddProfile_NewProfile verifies that "config add-profile <name>"
// creates a new profile with an API key stored in the keychain.
func TestAddProfile_NewProfile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	// Start with a config that has no profiles.
	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgContent := "current-profile: \"\"\nprofiles: []\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(cfgContent), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()

	// Set the api-key flag on the command so it doesn't prompt on stdin.
	if err := configAddProfileCmd.Flags().Set("api-key", "z_brand_new_key"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	t.Cleanup(func() {
		_ = configAddProfileCmd.Flags().Set("api-key", "")
		_ = configAddProfileCmd.Flags().Set("api-url", "")
	})

	err := configAddProfileCmd.RunE(configAddProfileCmd, []string{"brandnew"})
	if err != nil {
		t.Fatalf("add-profile: %v", err)
	}

	// Verify profile was created in config.
	cfg, err := config.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	p := cfg.GetProfile("brandnew")
	if p == nil {
		t.Fatal("profile 'brandnew' not found after add-profile")
	}

	// Verify the new profile was set as current (since no profile was active).
	if cfg.CurrentProfile != "brandnew" {
		t.Errorf("CurrentProfile = %q, want %q", cfg.CurrentProfile, "brandnew")
	}

	// Verify API key in keychain.
	creds, err := keyring.GetCredentials("brandnew")
	if err != nil {
		t.Fatalf("GetCredentials: %v", err)
	}
	if creds.APIKey != "z_brand_new_key" {
		t.Errorf("APIKey = %q, want %q", creds.APIKey, "z_brand_new_key")
	}
}
