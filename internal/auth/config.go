package auth

// Default OAuth configuration values. These are the production tenant's
// settings, so a bare `auth login` with no --env authenticates against
// production out of the box.
//
// They can be overridden per-profile via the `oauth-issuer`,
// `oauth-client-id`, and `oauth-audience` config fields, so a single binary
// can authenticate against multiple OAuth tenants. See OAuthConfigFor.
var (
	defaultIssuer   = "https://auth.getzep.com"
	defaultClientID = "8b0f41c63c6141c282c3b4dfd740708d"
	defaultAudience = "urn:zep:zepctl"
)

// DefaultOAuthConfig returns the OAuth configuration for zepctl using
// build-time defaults. Prefer OAuthConfigFor when a profile is available.
//
// The client ID is NOT a secret. It is a public OAuth 2.0 client identifier
// for a "Front-end" application, which has no client secret. It is safe to
// commit to source control and compile into the binary.
//
// The audience is a public token-audience identifier (it rides in every
// issued token), not a secret. It must be requested at login because the
// production API enforces the aud claim; without it every bearer request 401s.
func DefaultOAuthConfig() *OAuthConfig {
	return &OAuthConfig{
		Issuer:   defaultIssuer,
		ClientID: defaultClientID,
		Audience: defaultAudience,
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
