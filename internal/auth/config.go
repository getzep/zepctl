package auth

// Default OAuth configuration values. These can be overridden at build
// time via -ldflags for development/testing:
//
//	go build -ldflags "-X github.com/getzep/zepctl/internal/auth.defaultIssuer=https://dev.kinde.com \
//	                    -X github.com/getzep/zepctl/internal/auth.defaultClientID=dev-client-id"
//
// They can also be overridden per-profile via the `oauth-issuer` and
// `oauth-client-id` config fields, so a single binary can authenticate
// against multiple OAuth tenants. See OAuthConfigFor.
var (
	defaultIssuer   = "https://getzep.kinde.com"
	defaultClientID = "8b0f41c63c6141c282c3b4dfd740708d"
)

// DefaultOAuthConfig returns the OAuth configuration for zepctl using
// build-time defaults. Prefer OAuthConfigFor when a profile is available.
//
// The client ID is NOT a secret. It is a public OAuth 2.0 client identifier
// for a "Front-end" application, which has no client secret. It is safe to
// commit to source control and compile into the binary. See spec Sections
// 2.2 and 7.5.
//
// Audience is intentionally empty in the build-time defaults. Profiles that
// target a backend which enforces the aud claim must provide one explicitly
// via the OAuth audience profile field (or via an environment preset).
func DefaultOAuthConfig() *OAuthConfig {
	return &OAuthConfig{
		Issuer:   defaultIssuer,
		ClientID: defaultClientID,
	}
}

// OAuthConfigFor returns the OAuth configuration for the given profile,
// using profile-level overrides where present and falling back to the
// build-time defaults. Any subset of overrides may be set.
func OAuthConfigFor(issuer, clientID, audience string) *OAuthConfig {
	cfg := DefaultOAuthConfig()
	if issuer != "" {
		cfg.Issuer = issuer
	}
	if clientID != "" {
		cfg.ClientID = clientID
	}
	if audience != "" {
		cfg.Audience = audience
	}
	return cfg
}
