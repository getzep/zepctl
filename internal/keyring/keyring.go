package keyring

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	serviceName = "zepctl"
)

// Set stores an API key for a profile in the system keychain.
func Set(profile, apiKey string) error {
	if err := keyring.Set(serviceName, profile, apiKey); err != nil {
		return fmt.Errorf("storing API key in keychain: %w", err)
	}
	return nil
}

// Get retrieves an API key for a profile from the system keychain.
func Get(profile string) (string, error) {
	apiKey, err := keyring.Get(serviceName, profile)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", nil
		}
		return "", fmt.Errorf("retrieving API key from keychain: %w", err)
	}
	return apiKey, nil
}

// Delete removes an API key for a profile from the system keychain.
func Delete(profile string) error {
	if err := keyring.Delete(serviceName, profile); err != nil {
		if err == keyring.ErrNotFound {
			return nil
		}
		return fmt.Errorf("deleting API key from keychain: %w", err)
	}
	return nil
}
