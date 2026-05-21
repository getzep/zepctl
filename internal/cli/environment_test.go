package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getzep/zepctl/internal/config"
	"github.com/spf13/viper"
)

// emptyConfigDir writes a config.yaml with no profiles or environments to
// tmpDir/.zepctl/, so config.Load() returns a clean Config.
func emptyConfigDir(t *testing.T, tmpDir string) {
	t.Helper()
	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgContent := "current-profile: \"\"\nprofiles: []\n"
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(cfgContent), 0o600); err != nil {
		t.Fatal(err)
	}
}

func resetEnvFlags(t *testing.T) {
	t.Helper()
	envFlags := []string{"api-url", "oauth-issuer", "oauth-client-id", "oauth-audience"}
	resetCmdFlags(t, configAddEnvironmentCmd, envFlags...)
	resetCmdFlags(t, configUpdateEnvironmentCmd, envFlags...)
	resetCmdFlags(t, configDeleteEnvironmentCmd, "force")
}

func TestAddEnvironment_New(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	emptyConfigDir(t, tmpDir)
	_, _ = config.Reload()
	resetEnvFlags(t)
	t.Cleanup(func() { resetEnvFlags(t) })

	if err := configAddEnvironmentCmd.Flags().Set("api-url", "https://api.dev.example.com"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configAddEnvironmentCmd.Flags().Set("oauth-issuer", "https://issuer.example.com"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}
	if err := configAddEnvironmentCmd.Flags().Set("oauth-client-id", "client-xyz"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := configAddEnvironmentCmd.RunE(configAddEnvironmentCmd, []string{"development"}); err != nil {
		t.Fatalf("add-environment: %v", err)
	}

	cfg, err := config.Reload()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	env := cfg.GetEnvironment("development")
	if env == nil {
		t.Fatal("environment 'development' missing after add-environment")
	}
	if env.APIURL != "https://api.dev.example.com" {
		t.Errorf("APIURL = %q, want %q", env.APIURL, "https://api.dev.example.com")
	}
	if env.OAuthIssuer != "https://issuer.example.com" {
		t.Errorf("OAuthIssuer = %q", env.OAuthIssuer)
	}
	if env.OAuthClientID != "client-xyz" {
		t.Errorf("OAuthClientID = %q", env.OAuthClientID)
	}
}

func TestAddEnvironment_Duplicate_Errors(t *testing.T) {
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
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()
	resetEnvFlags(t)
	t.Cleanup(func() { resetEnvFlags(t) })

	err := configAddEnvironmentCmd.RunE(configAddEnvironmentCmd, []string{"development"})
	if err == nil {
		t.Fatal("expected error for duplicate environment")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to mention 'already exists'", err.Error())
	}
}

func TestUpdateEnvironment_PartialUpdate(t *testing.T) {
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
    api-url: https://old.example.com
    oauth-issuer: https://issuer.example.com
    oauth-client-id: old-client
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()
	resetEnvFlags(t)
	t.Cleanup(func() { resetEnvFlags(t) })

	if err := configUpdateEnvironmentCmd.Flags().Set("api-url", "https://new.example.com"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := configUpdateEnvironmentCmd.RunE(configUpdateEnvironmentCmd, []string{"development"}); err != nil {
		t.Fatalf("update-environment: %v", err)
	}

	cfg, _ := config.Reload()
	env := cfg.GetEnvironment("development")
	if env == nil {
		t.Fatal("environment missing")
	}
	if env.APIURL != "https://new.example.com" {
		t.Errorf("APIURL = %q, want updated", env.APIURL)
	}
	// Untouched fields preserved.
	if env.OAuthIssuer != "https://issuer.example.com" {
		t.Errorf("OAuthIssuer = %q, want preserved", env.OAuthIssuer)
	}
	if env.OAuthClientID != "old-client" {
		t.Errorf("OAuthClientID = %q, want preserved", env.OAuthClientID)
	}
}

func TestUpdateEnvironment_ClearField(t *testing.T) {
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
    oauth-client-id: to-be-cleared
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()
	resetEnvFlags(t)
	t.Cleanup(func() { resetEnvFlags(t) })

	// Explicit empty string clears the field (mirrors update-profile semantics).
	if err := configUpdateEnvironmentCmd.Flags().Set("oauth-client-id", ""); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := configUpdateEnvironmentCmd.RunE(configUpdateEnvironmentCmd, []string{"development"}); err != nil {
		t.Fatalf("update-environment: %v", err)
	}

	cfg, _ := config.Reload()
	env := cfg.GetEnvironment("development")
	if env.OAuthClientID != "" {
		t.Errorf("OAuthClientID = %q, want cleared", env.OAuthClientID)
	}
}

func TestUpdateEnvironment_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	emptyConfigDir(t, tmpDir)
	_, _ = config.Reload()
	resetEnvFlags(t)
	t.Cleanup(func() { resetEnvFlags(t) })

	if err := configUpdateEnvironmentCmd.Flags().Set("api-url", "x"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	err := configUpdateEnvironmentCmd.RunE(configUpdateEnvironmentCmd, []string{"missing"})
	if err == nil {
		t.Fatal("expected error for unknown environment")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to mention 'not found'", err.Error())
	}
}

func TestUpdateEnvironment_NoFlagsProvided(t *testing.T) {
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
    api-url: https://x.example.com
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()
	resetEnvFlags(t)
	t.Cleanup(func() { resetEnvFlags(t) })

	err := configUpdateEnvironmentCmd.RunE(configUpdateEnvironmentCmd, []string{"development"})
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
	if !strings.Contains(err.Error(), "no flags provided") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestDeleteEnvironment_Force(t *testing.T) {
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
    api-url: https://x.example.com
  - name: local
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Reload()
	resetEnvFlags(t)
	t.Cleanup(func() { resetEnvFlags(t) })

	if err := configDeleteEnvironmentCmd.Flags().Set("force", "true"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	if err := configDeleteEnvironmentCmd.RunE(configDeleteEnvironmentCmd, []string{"development"}); err != nil {
		t.Fatalf("delete-environment: %v", err)
	}

	cfg, _ := config.Reload()
	if cfg.GetEnvironment("development") != nil {
		t.Error("development environment should be deleted")
	}
	if cfg.GetEnvironment("local") == nil {
		t.Error("local environment should be preserved")
	}
}

func TestDeleteEnvironment_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	emptyConfigDir(t, tmpDir)
	_, _ = config.Reload()
	resetEnvFlags(t)
	t.Cleanup(func() { resetEnvFlags(t) })

	if err := configDeleteEnvironmentCmd.Flags().Set("force", "true"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	err := configDeleteEnvironmentCmd.RunE(configDeleteEnvironmentCmd, []string{"missing"})
	if err == nil {
		t.Fatal("expected error for unknown environment")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q", err.Error())
	}
}
