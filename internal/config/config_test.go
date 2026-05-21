package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/getzep/zepctl/internal/keyring"
	"github.com/spf13/viper"
	gokeyring "github.com/zalando/go-keyring"
)

func init() {
	gokeyring.MockInit()
}

// resetConfig clears the cached config and viper state for test isolation.
func resetConfig(t *testing.T) {
	t.Helper()
	configMu.Lock()
	cachedConfig = nil
	configErr = nil
	configMu.Unlock()
	viper.Reset()
}

// setTestConfig injects a config into the cache so Load() returns it
// without reading from disk.
func setTestConfig(t *testing.T, cfg *Config) {
	t.Helper()
	configMu.Lock()
	cachedConfig = cfg
	configErr = nil
	configMu.Unlock()
}

func TestGetProjectUUID_FlagTakesPrecedence(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	setTestConfig(t, &Config{
		CurrentProfile: "default",
		Profiles: []Profile{
			{Name: "default", ProjectUUID: "profile-uuid"},
		},
	})

	viper.Set("project", "flag-uuid")
	t.Setenv("ZEP_PROJECT", "env-uuid")

	got := GetProjectUUID()
	if got != "flag-uuid" {
		t.Errorf("GetProjectUUID() = %q, want %q (flag should take precedence)", got, "flag-uuid")
	}
}

func TestGetProjectUUID_EnvOverridesProfile(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	setTestConfig(t, &Config{
		CurrentProfile: "default",
		Profiles: []Profile{
			{Name: "default", ProjectUUID: "profile-uuid"},
		},
	})

	// Viper reads ZEP_PROJECT env var when bound. Since we're testing
	// without flag binding, simulate env by setting the viper key to empty
	// (no flag) and using the env var via AutomaticEnv.
	viper.SetEnvPrefix("ZEP")
	viper.AutomaticEnv()
	t.Setenv("ZEP_PROJECT", "env-uuid")

	got := GetProjectUUID()
	if got != "env-uuid" {
		t.Errorf("GetProjectUUID() = %q, want %q (env should override profile)", got, "env-uuid")
	}
}

func TestGetProjectUUID_FallsBackToProfile(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	setTestConfig(t, &Config{
		CurrentProfile: "default",
		Profiles: []Profile{
			{Name: "default", ProjectUUID: "profile-uuid"},
		},
	})

	got := GetProjectUUID()
	if got != "profile-uuid" {
		t.Errorf("GetProjectUUID() = %q, want %q", got, "profile-uuid")
	}
}

func TestGetProjectUUID_EmptyWhenNothingSet(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	setTestConfig(t, &Config{
		CurrentProfile: "default",
		Profiles: []Profile{
			{Name: "default"},
		},
	})

	got := GetProjectUUID()
	if got != "" {
		t.Errorf("GetProjectUUID() = %q, want empty", got)
	}
}

func TestLoad_ReturnsDefaultsWhenNoFile(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	// Point HOME to a temp dir with no config file.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Defaults.Output != "table" {
		t.Errorf("default output = %q, want %q", cfg.Defaults.Output, "table")
	}
	if cfg.Defaults.PageSize != 50 {
		t.Errorf("default page size = %d, want 50", cfg.Defaults.PageSize)
	}
}

func TestLoad_ParsesConfigFile(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	configDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}

	configContent := `current-profile: myprofile
profiles:
  - name: myprofile
    api-url: https://api.example.com
    project-uuid: abc-123
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.CurrentProfile != "myprofile" {
		t.Errorf("CurrentProfile = %q, want %q", cfg.CurrentProfile, "myprofile")
	}
	p := cfg.GetProfile("myprofile")
	if p == nil {
		t.Fatal("profile 'myprofile' not found")
	}
	if p.ProjectUUID != "abc-123" {
		t.Errorf("ProjectUUID = %q, want %q", p.ProjectUUID, "abc-123")
	}
}

func TestGetAPIKey_FlagTakesPrecedence(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	setTestConfig(t, &Config{
		CurrentProfile: "default",
		Profiles:       []Profile{{Name: "default"}},
	})
	// Seed the profile's keychain with an API key.
	if err := keyring.Set("default", "z_profile_key"); err != nil {
		t.Fatalf("keyring.Set: %v", err)
	}

	viper.Set("api-key", "z_flag_key")

	got := GetAPIKey()
	if got != "z_flag_key" {
		t.Errorf("GetAPIKey() = %q, want %q (flag should take precedence)", got, "z_flag_key")
	}
}

func TestGetAPIKey_FallsBackToProfile(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	setTestConfig(t, &Config{
		CurrentProfile: "default",
		Profiles:       []Profile{{Name: "default"}},
	})
	if err := keyring.Set("default", "z_profile_key"); err != nil {
		t.Fatalf("keyring.Set: %v", err)
	}

	got := GetAPIKey()
	if got != "z_profile_key" {
		t.Errorf("GetAPIKey() = %q, want %q", got, "z_profile_key")
	}
}

func TestGetAPIKey_EmptyWhenNothingSet(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	// Use a profile name that has no keychain entry.
	setTestConfig(t, &Config{
		CurrentProfile: "empty-profile",
		Profiles:       []Profile{{Name: "empty-profile"}},
	})

	got := GetAPIKey()
	if got != "" {
		t.Errorf("GetAPIKey() = %q, want empty", got)
	}
}

func TestGetAPIKeyOverride_ReturnsOverride(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	viper.Set("api-key", "z_explicit_override")

	got := GetAPIKeyOverride()
	if got != "z_explicit_override" {
		t.Errorf("GetAPIKeyOverride() = %q, want %q", got, "z_explicit_override")
	}

	// Empty when not set.
	viper.Reset()
	got = GetAPIKeyOverride()
	if got != "" {
		t.Errorf("GetAPIKeyOverride() = %q, want empty", got)
	}
}

func TestDedupeProfiles_KeepsLastOccurrence(t *testing.T) {
	cfg := &Config{
		Profiles: []Profile{
			{Name: "default", APIURL: "https://api.example.com"},
			{Name: "other"},
			{Name: "default", APIURL: "https://api.getzep.com"},
		},
	}
	dupes := dedupeProfiles(cfg)
	if len(dupes) != 1 || dupes[0] != "default" {
		t.Errorf("dupes = %v, want [default]", dupes)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("profiles = %d, want 2", len(cfg.Profiles))
	}
	// Order is preserved: "other" stays at its original index, and the kept
	// "default" copy is the LAST one (the prod URL).
	if cfg.Profiles[0].Name != "other" {
		t.Errorf("Profiles[0].Name = %q, want %q", cfg.Profiles[0].Name, "other")
	}
	if cfg.Profiles[1].Name != "default" || cfg.Profiles[1].APIURL != "https://api.getzep.com" {
		t.Errorf("Profiles[1] = %+v, want last default with prod URL", cfg.Profiles[1])
	}
}

// TestDedupeProfiles_ReportsEachNameOnce verifies the unique-names guarantee
// of the returned slice: a name appearing N>2 times is still reported once
// so the user-facing warning doesn't repeat the same name multiple times.
func TestDedupeProfiles_ReportsEachNameOnce(t *testing.T) {
	cfg := &Config{
		Profiles: []Profile{
			{Name: "default", APIURL: "a"},
			{Name: "default", APIURL: "b"},
			{Name: "default", APIURL: "c"},
		},
	}
	dupes := dedupeProfiles(cfg)
	if len(dupes) != 1 || dupes[0] != "default" {
		t.Errorf("dupes = %v, want exactly one [default] entry", dupes)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].APIURL != "c" {
		t.Errorf("expected single profile with last APIURL, got %+v", cfg.Profiles)
	}
}

func TestDedupeProfiles_NoDuplicates(t *testing.T) {
	cfg := &Config{
		Profiles: []Profile{
			{Name: "a"},
			{Name: "b"},
		},
	}
	if dupes := dedupeProfiles(cfg); dupes != nil {
		t.Errorf("dupes = %v, want nil", dupes)
	}
	if len(cfg.Profiles) != 2 {
		t.Errorf("profiles mutated when no dupes present")
	}
}

// TestLoad_DedupesAndWarnsOnDuplicates writes a YAML config with two
// "default" profiles to disk, calls Load, and verifies (a) the cached
// config has the duplicates collapsed, keeping the last by file position,
// and (b) a warning naming the duplicate was emitted to stderr.
func TestLoad_DedupesAndWarnsOnDuplicates(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	cfgDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	yaml := `current-profile: default
profiles:
  - name: default
    api-url: https://api.example.com
  - name: keep
    api-url: https://other.example.com
  - name: default
    api-url: https://api.getzep.com
defaults:
  output: table
  page-size: 50
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Capture stderr for the duration of Load() so we can assert on the warning.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w

	cfg, loadErr := Load()

	_ = w.Close()
	os.Stderr = origStderr
	var stderr bytes.Buffer
	_, _ = io.Copy(&stderr, r)

	if loadErr != nil {
		t.Fatalf("Load: %v", loadErr)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("Profiles after Load: got %d, want 2 (deduped)", len(cfg.Profiles))
	}
	// "keep" stays in its original position (index 1 -> 0 after dedupe);
	// "default" survives at the position of its last occurrence.
	if cfg.Profiles[0].Name != "keep" {
		t.Errorf("Profiles[0].Name = %q, want %q", cfg.Profiles[0].Name, "keep")
	}
	if cfg.Profiles[1].Name != "default" || cfg.Profiles[1].APIURL != "https://api.getzep.com" {
		t.Errorf("Profiles[1] = %+v, want last default with prod URL", cfg.Profiles[1])
	}
	if !bytes.Contains(stderr.Bytes(), []byte("default")) {
		t.Errorf("stderr did not mention duplicate name 'default'; got: %q", stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("Warning")) {
		t.Errorf("stderr did not include a Warning prefix; got: %q", stderr.String())
	}
}

func TestGetEnvironment_FoundAndNotFound(t *testing.T) {
	cfg := &Config{
		Environments: []Environment{
			{Name: "development", APIURL: "https://dev.example.com"},
			{Name: "local", APIURL: "http://localhost:8000"},
		},
	}

	if env := cfg.GetEnvironment("development"); env == nil || env.APIURL != "https://dev.example.com" {
		t.Errorf("GetEnvironment(development) = %+v, want development env", env)
	}
	if env := cfg.GetEnvironment("missing"); env != nil {
		t.Errorf("GetEnvironment(missing) = %+v, want nil", env)
	}
}

func TestGetEnvironment_ReturnsPointerToConfigSlice(t *testing.T) {
	// Mutating the returned pointer must update the underlying config so
	// callers can do `env := cfg.GetEnvironment(...); env.APIURL = "..."` and
	// have Save persist the change. This mirrors GetProfile's contract.
	cfg := &Config{
		Environments: []Environment{{Name: "development", APIURL: "old"}},
	}
	env := cfg.GetEnvironment("development")
	env.APIURL = "new"
	if cfg.Environments[0].APIURL != "new" {
		t.Errorf("Environments[0].APIURL = %q, want %q (mutation must be visible)",
			cfg.Environments[0].APIURL, "new")
	}
}

func TestDedupeEnvironments_KeepsLastOccurrence(t *testing.T) {
	cfg := &Config{
		Environments: []Environment{
			{Name: "development", APIURL: "first"},
			{Name: "local"},
			{Name: "development", APIURL: "last"},
		},
	}
	dupes := dedupeEnvironments(cfg)
	if len(dupes) != 1 || dupes[0] != "development" {
		t.Errorf("dupes = %v, want [development]", dupes)
	}
	if len(cfg.Environments) != 2 {
		t.Fatalf("Environments = %d, want 2", len(cfg.Environments))
	}
	if cfg.Environments[0].Name != "local" {
		t.Errorf("Environments[0].Name = %q, want %q", cfg.Environments[0].Name, "local")
	}
	if cfg.Environments[1].Name != "development" || cfg.Environments[1].APIURL != "last" {
		t.Errorf("Environments[1] = %+v, want last development entry", cfg.Environments[1])
	}
}

func TestDedupeEnvironments_NoDuplicates(t *testing.T) {
	cfg := &Config{
		Environments: []Environment{
			{Name: "development"},
			{Name: "local"},
		},
	}
	if dupes := dedupeEnvironments(cfg); dupes != nil {
		t.Errorf("dupes = %v, want nil", dupes)
	}
	if len(cfg.Environments) != 2 {
		t.Errorf("environments mutated when no dupes present")
	}
}

func TestSave_RejectsDuplicateEnvironments(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	cfg := &Config{
		Environments: []Environment{
			{Name: "development"},
			{Name: "development"},
		},
	}
	if err := cfg.Save(); err == nil {
		t.Fatal("expected error from Save with duplicate environment names")
	}
}

func TestLoad_ParsesEnvironmentsAndDedupes(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	cfgDir := filepath.Join(tmpDir, ".zepctl")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	yaml := `current-profile: dev
profiles:
  - name: dev
    api-url: https://api.development.example.com
environments:
  - name: development
    api-url: https://first.example.com
    oauth-issuer: https://issuer.example.com
    oauth-client-id: first-client-id
  - name: local
    api-url: http://localhost:8000
  - name: development
    api-url: https://second.example.com
    oauth-issuer: https://issuer.example.com
    oauth-client-id: second-client-id
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Discard stderr so the dedupe warning doesn't pollute test output.
	origStderr := os.Stderr
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	os.Stderr = devnull
	defer func() {
		os.Stderr = origStderr
		_ = devnull.Close()
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(cfg.Environments) != 2 {
		t.Fatalf("Environments after dedupe = %d, want 2", len(cfg.Environments))
	}
	dev := cfg.GetEnvironment("development")
	if dev == nil {
		t.Fatal("development environment missing after dedupe")
	}
	if dev.APIURL != "https://second.example.com" {
		t.Errorf("development.APIURL = %q, want last entry %q",
			dev.APIURL, "https://second.example.com")
	}
	if dev.OAuthClientID != "second-client-id" {
		t.Errorf("development.OAuthClientID = %q, want %q",
			dev.OAuthClientID, "second-client-id")
	}
	if local := cfg.GetEnvironment("local"); local == nil || local.APIURL != "http://localhost:8000" {
		t.Errorf("local environment = %+v, want APIURL=http://localhost:8000", local)
	}
}

func TestSave_RejectsDuplicateProfiles(t *testing.T) {
	resetConfig(t)
	defer resetConfig(t)

	cfg := &Config{
		Profiles: []Profile{
			{Name: "default"},
			{Name: "default"},
		},
	}
	// validateNoDuplicateProfiles errors before any disk I/O, so HOME setup
	// is unnecessary here.
	if err := cfg.Save(); err == nil {
		t.Fatal("expected error from Save with duplicate profile names")
	}
}
