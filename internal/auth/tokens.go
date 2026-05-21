package auth

import (
	"time"

	"github.com/getzep/zepctl/internal/keyring"
)

// StoreBearerToken saves a new bearer token to the profile's keychain entry.
// Preserves any existing API key.
func StoreBearerToken(profile string, result *TokenResult, email string) error {
	creds, err := keyring.GetCredentials(profile)
	if err != nil {
		creds = &keyring.Credentials{}
	}

	creds.AccessToken = result.AccessToken
	creds.RefreshToken = result.RefreshToken
	creds.ExpiresAt = result.ExpiresAt.Format(time.RFC3339)
	creds.UserEmail = email

	return keyring.SetCredentials(profile, creds)
}

// ClearBearerToken removes bearer token fields from the profile's keychain
// entry, preserving any existing API key.
func ClearBearerToken(profile string) error {
	creds, err := keyring.GetCredentials(profile)
	if err != nil {
		return nil
	}

	creds.AccessToken = ""
	creds.RefreshToken = ""
	creds.ExpiresAt = ""
	creds.UserEmail = ""

	return keyring.SetCredentials(profile, creds)
}
