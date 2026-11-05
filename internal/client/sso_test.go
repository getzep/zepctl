package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/getzep/zepctl/internal/auth"
	"github.com/getzep/zepctl/internal/config"
	"github.com/getzep/zepctl/internal/keyring"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestSSORequiredMarkerPreservesOtherConflicts(t *testing.T) {
	for _, marker := range []string{"", "1"} {
		t.Run("marker="+marker, func(t *testing.T) {
			transport := &ssoRequiredTransport{base: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusConflict, Header: http.Header{
					"X-Zep-Sso-Login-Required": {marker},
				}, Body: io.NopCloser(strings.NewReader("policy conflict"))}, nil
			})}
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", http.NoBody)
			require.NoError(t, err)
			resp, err := transport.RoundTrip(req)
			if marker == "1" {
				require.ErrorIs(t, err, auth.ErrSSOLoginRequired)
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.Equal(t, http.StatusConflict, resp.StatusCode)
				require.NoError(t, resp.Body.Close())
			}
		})
	}
}

func TestEnterpriseSSOClientDoesNotUseProviderRefresh(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ZEP_BEARER_TOKEN", "")
	_, err := config.Reload()
	require.NoError(t, err)
	profile := t.Name()
	cfg := &config.Config{CurrentProfile: profile, Profiles: []config.Profile{{Name: profile}}}
	require.NoError(t, cfg.Save())
	for _, expired := range []bool{false, true} {
		expiry := time.Now().Add(time.Hour)
		if expired {
			expiry = time.Now().Add(-time.Hour)
		}
		require.NoError(t, keyring.SetCredentials(profile, &keyring.Credentials{
			AccessToken: "test-enterprise-credential", RefreshToken: "must-not-be-used",
			ExpiresAt: expiry.Format(time.RFC3339), EnterpriseSSO: true,
		}))
		client, err := newBearerClient(context.Background())
		if expired {
			require.True(t, errors.Is(err, auth.ErrSSOLoginRequired))
		} else {
			require.NoError(t, err)
			wrapped, ok := client.Transport.(*ssoRequiredTransport)
			require.True(t, ok)
			_, ok = wrapped.base.(*enterpriseBearerTransport)
			require.True(t, ok)
		}
	}
}
