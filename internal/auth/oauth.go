package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	callbackTimeout = 5 * time.Minute
	callbackPath    = "/callback"

	// DefaultAPIURL is the default Zep API base URL.
	DefaultAPIURL = "https://api.getzep.com"

	// callbackPort is the fixed port for the OAuth callback server.
	// This port is registered as an allowed callback URL in Kinde
	// (http://127.0.0.1:18923/callback). Using a fixed port avoids
	// the need for wildcard port registration in Kinde.
	callbackPort = 18923
)

// OAuthConfig holds the OAuth configuration for zepctl.
type OAuthConfig struct {
	Issuer   string // OIDC issuer, e.g. "https://myapp.kinde.com".
	ClientID string // OAuth client ID (public, not a secret).
	Audience string // Optional API audience.
}

// TokenResult holds the tokens returned from a successful OAuth exchange.
type TokenResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	IDToken      string
}

// Login performs the Authorization Code + PKCE flow.
// If noBrowser is true, the manual code-paste flow is used.
func Login(ctx context.Context, cfg *OAuthConfig, session *KeychainSession, noBrowser bool) (*TokenResult, error) {
	if noBrowser {
		return loginManual(ctx, cfg, session)
	}
	return loginBrowser(ctx, cfg, session)
}

// newOAuth2Config returns a standard oauth2.Config for the given OAuthConfig.
func newOAuth2Config(cfg *OAuthConfig) *oauth2.Config {
	return &oauth2.Config{
		ClientID:    cfg.ClientID,
		RedirectURL: callbackURL(),
		// Kinde uses "offline" not "offline_access".
		Scopes: []string{"openid", "profile", "email", "offline"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   cfg.Issuer + "/oauth2/auth",
			TokenURL:  cfg.Issuer + "/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
}

// generateAuthURL builds the authorization URL with PKCE parameters and
// stores the state and code verifier in the session.
func generateAuthURL(cfg *OAuthConfig, session *KeychainSession) string {
	oauthCfg := newOAuth2Config(cfg)

	state := generateState()
	_ = session.SetState(state)

	verifier := oauth2.GenerateVerifier()
	_ = session.SetCodeVerifier(verifier)

	opts := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("is_use_auth_success_page", "true"),
	}
	// Kinde populates the access token's aud claim only when an audience is requested.
	if cfg.Audience != "" {
		opts = append(opts, oauth2.SetAuthURLParam("audience", cfg.Audience))
	}

	return oauthCfg.AuthCodeURL(state, opts...)
}

// generateState returns a cryptographically random URL-safe string for CSRF protection.
func generateState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// callbackURL returns the fixed localhost callback URL registered in Kinde.
func callbackURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", callbackPort, callbackPath)
}

// callbackHandler returns an HTTP handler that receives the OAuth redirect,
// exchanges the authorization code for tokens, and signals completion on doneCh.
func callbackHandler(cfg *OAuthConfig, session *KeychainSession, doneCh chan<- error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			desc := r.URL.Query().Get("error_description")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Authentication failed: %s -- %s", html.EscapeString(errParam), html.EscapeString(desc))
			doneCh <- fmt.Errorf("OAuth error: %s: %s", errParam, desc)
			return
		}
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		if err := exchangeCode(r.Context(), cfg, session, code, state); err != nil {
			http.Error(w, "Authentication failed", http.StatusInternalServerError)
			doneCh <- fmt.Errorf("token exchange failed: %w", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		doneCh <- nil
	}
}

func loginBrowser(ctx context.Context, cfg *OAuthConfig, session *KeychainSession) (*TokenResult, error) {
	doneCh := make(chan error, 1)

	lc := net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", callbackPort))
	if err != nil {
		// Port in use -- fall back to manual flow.
		return loginManual(ctx, cfg, session)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, callbackHandler(cfg, session, doneCh))
	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { _ = srv.Serve(listener) }()
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	authURL := generateAuthURL(cfg, session)

	if err := openBrowser(ctx, authURL); err != nil {
		// Browser failed -- fall back to manual flow.
		return loginManual(ctx, cfg, session)
	}

	fmt.Println("Opening browser to authenticate...")

	timeout := time.NewTimer(callbackTimeout)
	defer timeout.Stop()

	select {
	case err := <-doneCh:
		if err != nil {
			return nil, err
		}
		return tokenResultFromSession(session)
	case <-timeout.C:
		return nil, fmt.Errorf("timed out waiting for authentication (5 minutes)")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// loginManual performs the manual code-paste flow.
func loginManual(ctx context.Context, cfg *OAuthConfig, session *KeychainSession) (*TokenResult, error) {
	doneCh := make(chan error, 1)

	// Try to bind the callback port. If it fails, the flow still works
	// via manual code paste.
	var srv *http.Server
	lc := net.ListenConfig{}
	listener, lerr := lc.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", callbackPort))
	if lerr == nil {
		mux := http.NewServeMux()
		mux.HandleFunc(callbackPath, callbackHandler(cfg, session, doneCh))
		srv = &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func() { _ = srv.Serve(listener) }()
	}

	authURL := generateAuthURL(cfg, session)
	printManualInstructions(authURL)

	if srv != nil {
		defer func() {
			shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutCtx)
		}()
	}

	// Read code from stdin in a goroutine -- races with the callback server.
	stdinCh := make(chan codeOrError, 1)
	go readCodeFromStdin(stdinCh)

	timeout := time.NewTimer(callbackTimeout)
	defer timeout.Stop()

	select {
	case err := <-doneCh:
		if err != nil {
			return nil, err
		}
		fmt.Println("\nCallback received -- completing authentication.")
		return tokenResultFromSession(session)
	case result := <-stdinCh:
		if result.err != nil {
			return nil, result.err
		}
		return exchangeManualCode(ctx, cfg, session, result.code)
	case <-timeout.C:
		return nil, fmt.Errorf("timed out waiting for authentication (5 minutes)")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func printManualInstructions(authURL string) {
	fmt.Println("Open this URL in your browser to authenticate:")
	fmt.Println()
	fmt.Println("  " + authURL)
	fmt.Println()
	fmt.Print("Paste the authorization code here (or wait for redirect): ")
}

// exchangeManualCode exchanges a manually pasted authorization code for
// tokens using the session's stored state (preserving the PKCE verifier).
func exchangeManualCode(ctx context.Context, cfg *OAuthConfig, session *KeychainSession, code string) (*TokenResult, error) {
	state, err := session.GetState()
	if err != nil {
		return nil, fmt.Errorf("reading state: %w", err)
	}

	if err := exchangeCode(ctx, cfg, session, code, state); err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	return tokenResultFromSession(session)
}

// exchangeCode performs the Authorization Code + PKCE token exchange.
func exchangeCode(ctx context.Context, cfg *OAuthConfig, session *KeychainSession, code, receivedState string) error {
	storedState, err := session.GetState()
	if err != nil {
		return fmt.Errorf("reading state: %w", err)
	}
	if storedState == "" {
		return fmt.Errorf("state not found in session")
	}
	if storedState != receivedState {
		return fmt.Errorf("state mismatch: expected %s, got %s", storedState, receivedState)
	}

	codeVerifier, err := session.GetCodeVerifier()
	if err != nil {
		return fmt.Errorf("reading code verifier: %w", err)
	}
	if codeVerifier == "" {
		return fmt.Errorf("code verifier not found in session")
	}

	oauthCfg := newOAuth2Config(cfg)

	token, err := oauthCfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return fmt.Errorf("exchanging authorization code: %w", err)
	}

	return session.SetRawToken(token)
}

// tokenResultFromSession reads the persisted token and returns it as a TokenResult.
func tokenResultFromSession(session *KeychainSession) (*TokenResult, error) {
	tok, err := session.GetRawToken()
	if err != nil {
		return nil, fmt.Errorf("reading token after exchange: %w", err)
	}
	if tok == nil {
		return nil, fmt.Errorf("no token stored after exchange")
	}
	return &TokenResult{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    tok.Expiry,
		IDToken:      session.LastIDToken(),
	}, nil
}

// NewAutoRefreshClient creates an HTTP client that automatically refreshes
// the bearer token when it expires. Uses golang.org/x/oauth2's TokenSource
// which handles refresh transparently.
func NewAutoRefreshClient(ctx context.Context, cfg *OAuthConfig, session *KeychainSession) (*http.Client, error) {
	tok, err := session.GetRawToken()
	if err != nil {
		return nil, fmt.Errorf("reading stored token: %w", err)
	}
	if tok == nil {
		return nil, fmt.Errorf("no bearer token stored")
	}

	oauthCfg := newOAuth2Config(cfg)

	// oauth2.Config.TokenSource returns a ReuseTokenSource that auto-refreshes.
	ts := oauthCfg.TokenSource(ctx, tok)

	// Wrap to persist refreshed tokens to keychain.
	pts := &persistingTokenSource{base: ts, session: session}
	return oauth2.NewClient(ctx, pts), nil
}

// persistingTokenSource wraps an oauth2.TokenSource and persists refreshed
// tokens to the keychain via the session. It tracks the last-seen token
// pointer so that it only writes to the keychain when the underlying
// ReuseTokenSource actually refreshes (returns a new pointer).
type persistingTokenSource struct {
	base    oauth2.TokenSource
	session *KeychainSession
	lastTok *oauth2.Token
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	// ReuseTokenSource returns the same pointer when the cached token is
	// still valid. Only persist when we get a new pointer (i.e., a refresh
	// actually happened), avoiding redundant keychain reads+writes.
	if tok != p.lastTok {
		if err := p.session.SetRawToken(tok); err != nil {
			return nil, fmt.Errorf("persisting refreshed token: %w", err)
		}
		p.lastTok = tok
	}
	return tok, nil
}

// RevokeToken revokes a refresh token at the OAuth revocation endpoint.
// Best-effort: callers should log and continue on failure.
func RevokeToken(ctx context.Context, cfg *OAuthConfig, refreshToken string) error {
	revokeURL := cfg.Issuer + "/oauth2/revoke"
	data := url.Values{
		"client_id": {cfg.ClientID},
		"token":     {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, revokeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("building revocation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending revocation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("revocation returned status %d", resp.StatusCode)
	}
	return nil
}

type codeOrError struct {
	code string
	err  error
}

func readCodeFromStdin(ch chan<- codeOrError) {
	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		ch <- codeOrError{err: fmt.Errorf("reading authorization code: %w", err)}
		return
	}
	code = strings.TrimSpace(code)
	if code == "" {
		ch <- codeOrError{err: fmt.Errorf("authorization code cannot be empty")}
		return
	}
	ch <- codeOrError{code: code}
}

func openBrowser(ctx context.Context, targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", targetURL)
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", targetURL)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}
