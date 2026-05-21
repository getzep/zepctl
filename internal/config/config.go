package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/getzep/zepctl/internal/keyring"
	"github.com/getzep/zepctl/internal/output"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	configMu     sync.Mutex
	cachedConfig *Config
	configErr    error
)

// Profile represents a named configuration profile.
// Credentials (API key and/or bearer token) are stored in the system keychain,
// not in this config file.
//
// OAuthIssuer and OAuthClientID are optional per-profile overrides for the
// OIDC issuer and OAuth client ID used by `auth login`. When empty, the
// binary's build-time defaults are used (see internal/auth/config.go).
// Setting these per profile lets a single binary authenticate against
// multiple OAuth tenants -- e.g. one profile for production and another
// for development.
type Profile struct {
	Name          string `yaml:"name"`
	APIURL        string `yaml:"api-url,omitempty"`
	AccountUUID   string `yaml:"account-uuid,omitempty"`
	ProjectUUID   string `yaml:"project-uuid,omitempty"`
	OAuthIssuer   string `yaml:"oauth-issuer,omitempty"`
	OAuthClientID string `yaml:"oauth-client-id,omitempty"`
	OAuthAudience string `yaml:"oauth-audience,omitempty"`
}

// Environment is a named preset of auth fields (api-url plus the OAuth
// issuer/client-id/audience triple) stored in the user's config file.
// See `zepctl config add-environment`.
type Environment struct {
	Name          string `yaml:"name"`
	APIURL        string `yaml:"api-url,omitempty"`
	OAuthIssuer   string `yaml:"oauth-issuer,omitempty"`
	OAuthClientID string `yaml:"oauth-client-id,omitempty"`
	OAuthAudience string `yaml:"oauth-audience,omitempty"`
}

// Config represents the zepctl configuration.
type Config struct {
	CurrentProfile string        `yaml:"current-profile"`
	Profiles       []Profile     `yaml:"profiles"`
	Environments   []Environment `yaml:"environments,omitempty"`
	Defaults       Defaults      `yaml:"defaults"`
}

// Defaults represents default settings.
type Defaults struct {
	Output   string `yaml:"output"`
	PageSize int    `yaml:"page-size"`
}

// GetConfigPath returns the path to the config file.
func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".zepctl", "config.yaml"), nil
}

// Load loads the configuration from disk.
// The config is cached after the first load for efficiency.
func Load() (*Config, error) {
	configMu.Lock()
	defer configMu.Unlock()
	if cachedConfig != nil || configErr != nil {
		return cachedConfig, configErr
	}
	cachedConfig, configErr = loadFromDisk()
	return cachedConfig, configErr
}

// loadFromDisk reads and parses the config file.
func loadFromDisk() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{
				Defaults: Defaults{
					Output:   "table",
					PageSize: 50,
				},
			}, nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if dropped := dedupeProfiles(&cfg); len(dropped) > 0 {
		output.Warn("config contained duplicate profile names: %s. Keeping the last entry (by file position) for each duplicate name; earlier duplicates will be removed on next save.", strings.Join(dropped, ", "))
	}

	if dropped := dedupeEnvironments(&cfg); len(dropped) > 0 {
		output.Warn("config contained duplicate environment names: %s. Keeping the last entry (by file position) for each duplicate name; earlier duplicates will be removed on next save.", strings.Join(dropped, ", "))
	}

	return &cfg, nil
}

// dedupeByName collapses entries that share a name. When duplicates are
// found, the LAST occurrence by position wins (later writes typically
// reflect the most recent intent). Returns the unique set of duplicate
// names that were collapsed; order of remaining entries is preserved.
func dedupeByName[T any](items *[]T, name func(T) string) []string {
	if len(*items) < 2 {
		return nil
	}

	lastIndex := make(map[string]int, len(*items))
	for i, v := range *items {
		lastIndex[name(v)] = i
	}

	kept := make([]T, 0, len(*items))
	var dupes []string
	reported := make(map[string]bool, len(*items))
	for i, v := range *items {
		n := name(v)
		if lastIndex[n] != i {
			if !reported[n] {
				dupes = append(dupes, n)
				reported[n] = true
			}
			continue
		}
		kept = append(kept, v)
	}

	if len(dupes) == 0 {
		return nil
	}
	*items = kept
	return dupes
}

func dedupeProfiles(cfg *Config) []string {
	return dedupeByName(&cfg.Profiles, func(p Profile) string { return p.Name })
}

func dedupeEnvironments(cfg *Config) []string {
	return dedupeByName(&cfg.Environments, func(e Environment) string { return e.Name })
}

// validateUniqueNames errors if any two items share a name. Defense-in-depth
// for Save: Load already dedupes on parse, but in-memory mutations could in
// theory introduce a duplicate before Save is called.
func validateUniqueNames[T any](items []T, name func(T) string, kind string) error {
	seen := make(map[string]bool, len(items))
	for _, v := range items {
		n := name(v)
		if seen[n] {
			return fmt.Errorf("duplicate %s name %q in config", kind, n)
		}
		seen[n] = true
	}
	return nil
}

// findByName returns a pointer into items whose name matches target, or nil.
// The pointer is into the underlying slice so callers can mutate in place.
func findByName[T any](items []T, target string, name func(T) string) *T {
	for i := range items {
		if name(items[i]) == target {
			return &items[i]
		}
	}
	return nil
}

// Reload forces a reload of the configuration from disk.
// This is useful after modifying the config file (e.g., adding a profile).
func Reload() (*Config, error) {
	configMu.Lock()
	cachedConfig = nil
	configErr = nil
	configMu.Unlock()
	return Load()
}

// Save writes the configuration to disk and updates the cache.
func (c *Config) Save() error {
	if err := validateUniqueNames(c.Profiles, func(p Profile) string { return p.Name }, "profile"); err != nil {
		return err
	}
	if err := validateUniqueNames(c.Environments, func(e Environment) string { return e.Name }, "environment"); err != nil {
		return err
	}

	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	// Update cache to reflect saved changes.
	configMu.Lock()
	cachedConfig = c
	configErr = nil
	configMu.Unlock()
	return nil
}

// GetProfile returns the profile with the given name.
func (c *Config) GetProfile(name string) *Profile {
	return findByName(c.Profiles, name, func(p Profile) string { return p.Name })
}

// GetEnvironment returns the environment preset with the given name, or nil
// if not configured.
func (c *Config) GetEnvironment(name string) *Environment {
	return findByName(c.Environments, name, func(e Environment) string { return e.Name })
}

// GetCurrentProfile returns the current active profile.
func (c *Config) GetCurrentProfile() *Profile {
	// Check for override from flag or env var
	if profile := viper.GetString("profile"); profile != "" {
		return c.GetProfile(profile)
	}
	return c.GetProfile(c.CurrentProfile)
}

// GetAPIKeyOverride returns the API key if explicitly set via the --api-key
// flag or ZEP_API_KEY environment variable. Returns empty string if neither
// is set (does not check the profile keychain). Used to detect explicit
// overrides.
func GetAPIKeyOverride() string {
	return viper.GetString("api-key")
}

// GetAPIKey returns the API key to use, checking flags, env, and profile keychain.
func GetAPIKey() string {
	// Flag/env takes precedence
	if key := viper.GetString("api-key"); key != "" {
		return key
	}

	// Then check current profile's keychain entry
	cfg, err := Load()
	if err != nil {
		return ""
	}

	if profile := cfg.GetCurrentProfile(); profile != nil {
		creds, err := keyring.GetCredentials(profile.Name)
		if err != nil {
			return ""
		}
		return creds.APIKey
	}

	return ""
}

// GetProjectUUID returns the project UUID to use, checking flags, env, and profile.
func GetProjectUUID() string {
	// Flag/env takes precedence
	if p := viper.GetString("project"); p != "" {
		return p
	}

	cfg, err := Load()
	if err != nil {
		return ""
	}

	if profile := cfg.GetCurrentProfile(); profile != nil {
		return profile.ProjectUUID
	}

	return ""
}

// GetAPIURL returns the API URL to use, checking flags, env, and profile.
// Returns empty string if no explicit URL is configured, allowing the SDK to use its default.
func GetAPIURL() string {
	// Flag/env takes precedence
	if url := viper.GetString("api-url"); url != "" {
		return url
	}

	// Then check current profile
	cfg, err := Load()
	if err != nil {
		return ""
	}

	if profile := cfg.GetCurrentProfile(); profile != nil && profile.APIURL != "" {
		return profile.APIURL
	}

	return ""
}
