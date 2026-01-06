package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/getzep/zepctl/internal/keyring"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var (
	cachedConfig *Config
	configOnce   sync.Once
	configErr    error
)

// Profile represents a named configuration profile.
// API keys are stored in the system keychain, not in this config file.
type Profile struct {
	Name   string `yaml:"name"`
	APIURL string `yaml:"api-url,omitempty"`
}

// Config represents the zepctl configuration.
type Config struct {
	CurrentProfile string    `yaml:"current-profile"`
	Profiles       []Profile `yaml:"profiles"`
	Defaults       Defaults  `yaml:"defaults"`
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
	configOnce.Do(func() {
		cachedConfig, configErr = loadFromDisk()
	})
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

	return &cfg, nil
}

// Reload forces a reload of the configuration from disk.
// This is useful after modifying the config file (e.g., adding a profile).
func Reload() (*Config, error) {
	configOnce = sync.Once{}
	cachedConfig = nil
	configErr = nil
	return Load()
}

// Save writes the configuration to disk and updates the cache.
func (c *Config) Save() error {
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

	// Update cache to reflect saved changes
	cachedConfig = c
	return nil
}

// GetProfile returns the profile with the given name.
func (c *Config) GetProfile(name string) *Profile {
	for i := range c.Profiles {
		if c.Profiles[i].Name == name {
			return &c.Profiles[i]
		}
	}
	return nil
}

// GetCurrentProfile returns the current active profile.
func (c *Config) GetCurrentProfile() *Profile {
	// Check for override from flag or env var
	if profile := viper.GetString("profile"); profile != "" {
		return c.GetProfile(profile)
	}
	return c.GetProfile(c.CurrentProfile)
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
		if key, err := keyring.Get(profile.Name); err == nil && key != "" {
			return key
		}
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
