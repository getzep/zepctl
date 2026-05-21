package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// IDTokenClaims holds the claims extracted from a Kinde ID token.
// Only the fields needed for display are included.
type IDTokenClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Sub   string `json:"sub"`
}

// ParseUnverifiedIDToken extracts claims from an ID token without validating the
// signature. The ID token is used only for display (user email/name) and
// is not stored long-term.
func ParseUnverifiedIDToken(idToken string) (*IDTokenClaims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid ID token format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decoding ID token payload: %w", err)
	}

	var claims IDTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parsing ID token claims: %w", err)
	}

	return &claims, nil
}
