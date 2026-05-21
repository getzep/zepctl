package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestParseUnverifiedIDToken(t *testing.T) {
	// Build a valid JWT-shaped ID token (header.payload.signature).
	claims := map[string]string{
		"email": "fred@frobozz.infocom",
		"name":  "Fred",
		"sub":   "kp_user123",
	}
	payload, _ := json.Marshal(claims)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	body := base64.RawURLEncoding.EncodeToString(payload)
	sig := base64.RawURLEncoding.EncodeToString([]byte("signature"))

	token := header + "." + body + "." + sig

	got, err := ParseUnverifiedIDToken(token)
	if err != nil {
		t.Fatalf("ParseUnverifiedIDToken: %v", err)
	}
	if got.Email != "fred@frobozz.infocom" {
		t.Errorf("Email = %q, want %q", got.Email, "fred@frobozz.infocom")
	}
	if got.Name != "Fred" {
		t.Errorf("Name = %q, want %q", got.Name, "Fred")
	}
	if got.Sub != "kp_user123" {
		t.Errorf("Sub = %q, want %q", got.Sub, "kp_user123")
	}
}

func TestParseUnverifiedIDToken_InvalidFormat(t *testing.T) {
	_, err := ParseUnverifiedIDToken("not-a-jwt")
	if err == nil {
		t.Error("expected error for invalid token format")
	}
}

func TestParseUnverifiedIDToken_InvalidPayload(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	sig := base64.RawURLEncoding.EncodeToString([]byte("sig"))
	token := header + ".!!!invalid!!!" + "." + sig

	_, err := ParseUnverifiedIDToken(token)
	if err == nil {
		t.Error("expected error for invalid base64 payload")
	}
}

func TestParseUnverifiedIDToken_InvalidJSON(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(`not json`))
	sig := base64.RawURLEncoding.EncodeToString([]byte("sig"))
	token := header + "." + body + "." + sig

	_, err := ParseUnverifiedIDToken(token)
	if err == nil {
		t.Error("expected error for invalid JSON payload")
	}
}
