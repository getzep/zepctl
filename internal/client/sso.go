package client

import (
	"net/http"
	"time"

	"github.com/getzep/zepctl/internal/auth"
)

type ssoRequiredTransport struct {
	base http.RoundTripper
}

func (t *ssoRequiredTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusConflict && resp.Header.Get("X-Zep-SSO-Login-Required") == "1" {
		_ = resp.Body.Close()
		return nil, auth.ErrSSOLoginRequired
	}
	return resp, nil
}

func withSSORequiredHandling(httpClient *http.Client) *http.Client {
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	httpClient.Transport = &ssoRequiredTransport{base: base}
	return httpClient
}

type enterpriseBearerTransport struct {
	token     string
	expiresAt time.Time
}

func (t *enterpriseBearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.expiresAt.After(time.Now()) {
		return nil, auth.ErrSSOLoginRequired
	}
	return (&BearerTransport{Token: t.token, Base: http.DefaultTransport}).RoundTrip(req)
}
