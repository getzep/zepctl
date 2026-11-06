package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/getzep/zepctl/internal/keyring"
	"github.com/stretchr/testify/require"
)

func TestResolveSSOAppURL(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://example.com/path", "https://user@example.com", "https://example.com?x=y"} {
		_, err := ResolveSSOAppURL("", raw)
		require.Error(t, err, raw)
	}
	got, err := ResolveSSOAppURL(DefaultAPIURL, "")
	require.NoError(t, err)
	require.Equal(t, "https://app.getzep.com", got)
	_, err = ResolveSSOAppURL("https://custom.example.com", "")
	require.Error(t, err)
}

func TestSSOFlowPKCEAndCallback(t *testing.T) {
	flow, err := newSSOFlow()
	require.NoError(t, err)
	start, err := url.Parse(flow.startURL("https://app.example.com", false))
	require.NoError(t, err)
	digest := sha256.Sum256([]byte(flow.verifier))
	require.Equal(t, base64.RawURLEncoding.EncodeToString(digest[:]), start.Query().Get("code_challenge"))
	require.NotContains(t, start.String(), flow.verifier)
	codes := make(chan string, 1)
	code := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	handler := flow.callbackHandler(codes, "127.0.0.1:18923")
	for _, state := range []string{"wrong", flow.state} {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:18923/callback?code="+code+"&state="+state, http.NoBody)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if state == "wrong" {
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Empty(t, codes)
		} else {
			require.Equal(t, http.StatusOK, response.Code)
			require.Equal(t, code, <-codes)
		}
	}
}

func TestSSOManualLoginAndStorage(t *testing.T) {
	code := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/auth/cli/exchange", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, code, body["code"])
		require.Len(t, body["code_verifier"], 43)
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-sso-credential", "token_type": "Bearer", "expires_at": time.Now().Add(time.Hour),
		}))
	}))
	defer server.Close()
	result, err := LoginSSO(context.Background(), SSOLoginOptions{
		AppURL: server.URL, NoBrowser: true, Output: io.Discard,
		ReadCode: func(context.Context) (string, error) { return code, nil },
	})
	require.NoError(t, err)
	profile := t.Name()
	require.NoError(t, keyring.SetCredentials(profile, &keyring.Credentials{APIKey: "test-api-key", RefreshToken: "old-refresh"}))
	require.NoError(t, StoreSSOBearerToken(profile, result, "member@example.com"))
	creds, err := keyring.GetCredentials(profile)
	require.NoError(t, err)
	require.True(t, creds.EnterpriseSSO)
	require.Equal(t, "test-api-key", creds.APIKey)
	require.Empty(t, creds.RefreshToken)
	require.NoError(t, ClearBearerToken(profile))
	creds, err = keyring.GetCredentials(profile)
	require.NoError(t, err)
	require.False(t, creds.EnterpriseSSO)
	require.Equal(t, "test-api-key", creds.APIKey)
}

func TestSSOExchangeDoesNotExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "sensitive-response")
	}))
	defer server.Close()
	flow, err := newSSOFlow()
	require.NoError(t, err)
	_, err = flow.exchange(context.Background(), server.URL, base64.RawURLEncoding.EncodeToString(make([]byte, 32)), nil)
	require.Error(t, err)
	require.False(t, strings.Contains(err.Error(), "sensitive-response"))
}

func TestSSOBrowserLogin(t *testing.T) {
	code := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	var params url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		digest := sha256.Sum256([]byte(body["code_verifier"]))
		require.Equal(t, params.Get("code_challenge"), base64.RawURLEncoding.EncodeToString(digest[:]))
		require.Equal(t, params.Get("redirect_uri"), body["redirect_uri"])
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-sso-credential", "token_type": "Bearer", "expires_at": time.Now().Add(time.Hour),
		}))
	}))
	defer server.Close()
	result, err := LoginSSO(context.Background(), SSOLoginOptions{
		AppURL: server.URL, Output: io.Discard,
		OpenBrowser: func(start string) error {
			parsed, err := url.Parse(start)
			require.NoError(t, err)
			params = parsed.Query()
			callback := params.Get("redirect_uri") + "?code=" + code + "&state=" + params.Get("state")
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, callback, http.NoBody)
			require.NoError(t, err)
			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, http.StatusOK, response.StatusCode)
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "test-sso-credential", result.AccessToken)
}

func TestSSOLoginCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := LoginSSO(ctx, SSOLoginOptions{
		NoBrowser: true, Output: io.Discard,
		ReadCode: func(ctx context.Context) (string, error) { return "", ctx.Err() },
	})
	require.ErrorIs(t, err, context.Canceled)
}
