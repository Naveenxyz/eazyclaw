package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"
)

// OAuthConfig holds configuration for an OAuth flow.
type OAuthConfig struct {
	ClientID    string
	AuthURL     string
	TokenURL    string
	RedirectURL string
	Scopes      []string
}

// AnthropicOAuthConfig returns the pre-configured OAuth config for Anthropic.
func AnthropicOAuthConfig() OAuthConfig {
	return OAuthConfig{
		ClientID:    "", // Must be set by the user or config.
		AuthURL:     "https://console.anthropic.com/oauth/authorize",
		TokenURL:    "https://console.anthropic.com/oauth/token",
		RedirectURL: "http://localhost:8989/callback",
		Scopes:      []string{"messages:write"},
	}
}

// OpenAIOAuthConfig returns the pre-configured OAuth config for OpenAI.
func OpenAIOAuthConfig() OAuthConfig {
	return OAuthConfig{
		ClientID:    "", // Must be set by the user or config.
		AuthURL:     "https://auth.openai.com/authorize",
		TokenURL:    "https://auth.openai.com/oauth/token",
		RedirectURL: "http://localhost:8989/callback",
		Scopes:      []string{"openid", "profile"},
	}
}

// StartOAuthFlow runs an interactive OAuth flow with PKCE.
// It opens the browser for authorization and starts a local HTTP server
// on localhost:8989 to receive the callback.
func StartOAuthFlow(cfg OAuthConfig) (*oauth2.Token, error) {
	// Generate PKCE code verifier and challenge.
	verifier, err := generateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("oauth: generate code verifier: %w", err)
	}
	challenge := generateCodeChallenge(verifier)

	oauth2Cfg := &oauth2.Config{
		ClientID:    cfg.ClientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		},
		RedirectURL: cfg.RedirectURL,
		Scopes:      cfg.Scopes,
	}

	// Build auth URL with PKCE parameters.
	state, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("oauth: generate state: %w", err)
	}

	authURL := oauth2Cfg.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)

	// Channel to receive the authorization code.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	// Start local HTTP server.
	listener, err := net.Listen("tcp", "localhost:8989")
	if err != nil {
		return nil, fmt.Errorf("oauth: listen on :8989: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("oauth: state mismatch")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("oauth: authorization error: %s", errMsg)
			http.Error(w, errMsg, http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("oauth: no code in callback")
			http.Error(w, "No code", http.StatusBadRequest)
			return
		}

		fmt.Fprint(w, "<html><body><h1>Authorization successful!</h1><p>You can close this window.</p></body></html>")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("oauth: server error: %w", err)
		}
	}()

	// Open browser.
	if err := openBrowser(authURL); err != nil {
		fmt.Printf("Open this URL in your browser:\n%s\n", authURL)
	}

	// Wait for the callback.
	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		_ = srv.Close()
		return nil, err
	case <-time.After(5 * time.Minute):
		_ = srv.Close()
		return nil, fmt.Errorf("oauth: timed out waiting for authorization")
	}

	// Shut down the server.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	// Exchange code for token with PKCE verifier.
	token, err := oauth2Cfg.Exchange(
		context.Background(),
		code,
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("oauth: exchange code: %w", err)
	}

	return token, nil
}

// generateCodeVerifier creates a cryptographically random PKCE code verifier.
func generateCodeVerifier() (string, error) {
	return generateRandomString(64)
}

// generateCodeChallenge creates a S256 PKCE code challenge from a verifier.
func generateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// generateRandomString creates a cryptographically random URL-safe string.
func generateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:length], nil
}

// openBrowser opens the given URL in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}
