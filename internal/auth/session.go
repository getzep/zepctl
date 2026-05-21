package auth

import (
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"github.com/getzep/zepctl/internal/keyring"
)

const bearerTokenType = "Bearer"

// KeychainSession stores OAuth tokens in the OS keychain and keeps
// ephemeral OAuth state (CSRF state, PKCE code verifier) in memory.
type KeychainSession struct {
	profile      string
	state        string
	codeVerifier string
	// lastIDToken captures the ID token from the most recent SetRawToken
	// call. The standard oauth2.Token struct has no dedicated field for
	// it, but the token response includes it in Extra("id_token").
	lastIDToken string
}

// NewKeychainSession creates a session for the given profile. Tokens are
// persisted to the OS keychain; OAuth flow state is held in memory.
func NewKeychainSession(profile string) *KeychainSession {
	return &KeychainSession{profile: profile}
}

// SetRawToken persists an oauth2.Token to the keychain, preserving any
// existing API key in the profile's credential entry. Also captures the
// ID token (if present) for later extraction by the caller.
func (s *KeychainSession) SetRawToken(token *oauth2.Token) error {
	creds, err := keyring.GetCredentials(s.profile)
	if err != nil {
		creds = &keyring.Credentials{}
	}

	if token == nil {
		// Clear bearer fields but preserve any existing API key.
		creds.AccessToken = ""
		creds.RefreshToken = ""
		creds.ExpiresAt = ""
		return keyring.SetCredentials(s.profile, creds)
	}

	creds.AccessToken = token.AccessToken
	creds.RefreshToken = token.RefreshToken
	if !token.Expiry.IsZero() {
		creds.ExpiresAt = token.Expiry.Format(time.RFC3339)
	}

	// Capture the ID token from the Extra fields.
	if idToken, ok := token.Extra("id_token").(string); ok {
		s.lastIDToken = idToken
	}

	return keyring.SetCredentials(s.profile, creds)
}

// LastIDToken returns the ID token captured from the most recent
// SetRawToken call. Returns empty string if no ID token was present.
func (s *KeychainSession) LastIDToken() string {
	return s.lastIDToken
}

// GetRawToken reads the stored oauth2.Token from the keychain.
// Returns nil (not an error) if no bearer token is stored.
func (s *KeychainSession) GetRawToken() (*oauth2.Token, error) {
	creds, err := keyring.GetCredentials(s.profile)
	if err != nil {
		return nil, fmt.Errorf("reading credentials: %w", err)
	}

	if !creds.HasBearerToken() {
		return nil, nil
	}

	tok := &oauth2.Token{
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		TokenType:    bearerTokenType,
	}

	if creds.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, creds.ExpiresAt)
		if err == nil {
			tok.Expiry = t
		}
	}

	return tok, nil
}

// SetState stores the CSRF state parameter for the current OAuth flow.
func (s *KeychainSession) SetState(state string) error {
	s.state = state
	return nil
}

// GetState returns the stored CSRF state parameter.
func (s *KeychainSession) GetState() (string, error) {
	return s.state, nil
}

// SetCodeVerifier stores the PKCE code verifier for the current OAuth flow.
func (s *KeychainSession) SetCodeVerifier(codeVerifier string) error {
	s.codeVerifier = codeVerifier
	return nil
}

// GetCodeVerifier returns the stored PKCE code verifier.
func (s *KeychainSession) GetCodeVerifier() (string, error) {
	return s.codeVerifier, nil
}
