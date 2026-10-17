package keyring

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	serviceName = "zepctl"
)

// Credentials holds all authentication credentials for a profile.
// All fields are optional -- a profile may have just an API key,
// just a bearer token, or both.
type Credentials struct {
	APIKey       string `json:"api_key,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	UserEmail    string `json:"user_email,omitempty"`
}

// IsExpired returns true if the access token has expired.
func (c *Credentials) IsExpired() bool {
	if c.ExpiresAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().After(t)
}

// ExpiresIn returns the duration until the access token expires.
// Returns 0 if already expired or unparseable.
func (c *Credentials) ExpiresIn() time.Duration {
	if c.ExpiresAt == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return 0
	}
	d := time.Until(t)
	if d < 0 {
		return 0
	}
	return d
}

// HasBearerToken returns true if bearer token fields are populated.
func (c *Credentials) HasBearerToken() bool {
	return c.AccessToken != ""
}

// HasAPIKey returns true if an API key is configured.
func (c *Credentials) HasAPIKey() bool {
	return c.APIKey != ""
}

// GetCredentials retrieves structured credentials for a profile.
// Transparently migrates legacy raw API key strings to JSON format.
func GetCredentials(profile string) (*Credentials, error) {
	raw, err := keyring.Get(serviceName, profile)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return &Credentials{}, nil
		}
		return nil, fmt.Errorf("retrieving credentials from keychain: %w", err)
	}

	if raw == "" {
		return &Credentials{}, nil
	}

	// Try parsing as JSON (new format)
	var creds Credentials
	if err := json.Unmarshal([]byte(raw), &creds); err == nil {
		return &creds, nil
	}

	// JSON parse failed -- treat as legacy raw API key string.
	// Migrate in-place to JSON format.
	creds = Credentials{APIKey: raw}
	if err := SetCredentials(profile, &creds); err != nil {
		// Migration failed, but we still have the credentials in memory.
		// Return them without persisting the upgrade.
		return &creds, nil
	}
	return &creds, nil
}

// SetCredentials stores structured credentials for a profile.
func SetCredentials(profile string, creds *Credentials) error {
	data, err := json.Marshal(creds) //nolint:gosec // G117: intentionally marshaling credentials for keychain storage
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	if err := keyring.Set(serviceName, profile, string(data)); err != nil {
		return fmt.Errorf("storing credentials in keychain: %w", err)
	}
	return nil
}

// Set stores an API key for a profile in the system keychain.
// Uses the new JSON format.
func Set(profile, apiKey string) error {
	creds, err := GetCredentials(profile)
	if err != nil {
		// If we can't read existing creds, start fresh
		creds = &Credentials{}
	}
	creds.APIKey = apiKey
	return SetCredentials(profile, creds)
}

// Get retrieves an API key for a profile from the system keychain.
// Handles both legacy raw strings and JSON format.
func Get(profile string) (string, error) {
	creds, err := GetCredentials(profile)
	if err != nil {
		return "", err
	}
	return creds.APIKey, nil
}

// Delete removes all credentials for a profile from the system keychain.
func Delete(profile string) error {
	if err := keyring.Delete(serviceName, profile); err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("deleting credentials from keychain: %w", err)
	}
	return nil
}
