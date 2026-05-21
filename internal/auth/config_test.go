package auth

import "testing"

func TestOAuthConfigFor_FallsBackToDefaults(t *testing.T) {
	cfg := OAuthConfigFor("", "", "")
	if cfg.Issuer != defaultIssuer {
		t.Errorf("Issuer = %q, want default %q", cfg.Issuer, defaultIssuer)
	}
	if cfg.ClientID != defaultClientID {
		t.Errorf("ClientID = %q, want default %q", cfg.ClientID, defaultClientID)
	}
	if cfg.Audience != "" {
		t.Errorf("Audience = %q, want empty default", cfg.Audience)
	}
}

func TestOAuthConfigFor_OverridesIssuerOnly(t *testing.T) {
	cfg := OAuthConfigFor("https://custom.kinde.com", "", "")
	if cfg.Issuer != "https://custom.kinde.com" {
		t.Errorf("Issuer = %q, want override", cfg.Issuer)
	}
	if cfg.ClientID != defaultClientID {
		t.Errorf("ClientID = %q, want default %q", cfg.ClientID, defaultClientID)
	}
}

func TestOAuthConfigFor_OverridesClientIDOnly(t *testing.T) {
	cfg := OAuthConfigFor("", "custom-client", "")
	if cfg.Issuer != defaultIssuer {
		t.Errorf("Issuer = %q, want default %q", cfg.Issuer, defaultIssuer)
	}
	if cfg.ClientID != "custom-client" {
		t.Errorf("ClientID = %q, want override", cfg.ClientID)
	}
}

func TestOAuthConfigFor_OverridesAudienceOnly(t *testing.T) {
	cfg := OAuthConfigFor("", "", "https://api.example.com")
	if cfg.Issuer != defaultIssuer {
		t.Errorf("Issuer = %q, want default", cfg.Issuer)
	}
	if cfg.Audience != "https://api.example.com" {
		t.Errorf("Audience = %q, want override", cfg.Audience)
	}
}

func TestOAuthConfigFor_OverridesAll(t *testing.T) {
	cfg := OAuthConfigFor("https://custom.kinde.com", "custom-client", "https://api.example.com")
	if cfg.Issuer != "https://custom.kinde.com" {
		t.Errorf("Issuer = %q, want override", cfg.Issuer)
	}
	if cfg.ClientID != "custom-client" {
		t.Errorf("ClientID = %q, want override", cfg.ClientID)
	}
	if cfg.Audience != "https://api.example.com" {
		t.Errorf("Audience = %q, want override", cfg.Audience)
	}
}
