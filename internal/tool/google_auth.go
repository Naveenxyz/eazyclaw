package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/gmail/v1"
)

// GoogleAuth manages Google OAuth2 authentication and token persistence.
type GoogleAuth struct {
	config    *oauth2.Config
	tokenPath string
	token     *oauth2.Token
	mu        sync.RWMutex
}

// NewGoogleAuth creates a new GoogleAuth with the given credentials and data directory.
func NewGoogleAuth(clientID, clientSecret, redirectURL, dataDir string) *GoogleAuth {
	tokenDir := filepath.Join(dataDir, "auth", "google")
	os.MkdirAll(tokenDir, 0700)

	ga := &GoogleAuth{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				gmail.GmailReadonlyScope,
				gmail.GmailSendScope,
				gmail.GmailModifyScope,
				calendar.CalendarScope,
			},
			Endpoint: google.Endpoint,
		},
		tokenPath: filepath.Join(tokenDir, "token.json"),
	}
	ga.loadToken()
	return ga
}

// loadToken reads the persisted token from disk if it exists.
func (ga *GoogleAuth) loadToken() {
	data, err := os.ReadFile(ga.tokenPath)
	if err != nil {
		return
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return
	}
	ga.token = &tok
}

// saveToken persists the current token to disk.
func (ga *GoogleAuth) saveToken() error {
	ga.mu.RLock()
	tok := ga.token
	ga.mu.RUnlock()

	if tok == nil {
		return fmt.Errorf("no token to save")
	}

	data, err := json.MarshalIndent(tok, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}
	if err := os.WriteFile(ga.tokenPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token: %w", err)
	}
	return nil
}

// AuthURL returns the OAuth2 consent URL for the user to visit.
func (ga *GoogleAuth) AuthURL() string {
	return ga.config.AuthCodeURL("state-token",
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
	)
}

// HandleCallback exchanges the authorization code for a token and saves it.
func (ga *GoogleAuth) HandleCallback(code string) error {
	tok, err := ga.config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("failed to exchange code: %w", err)
	}

	ga.mu.Lock()
	ga.token = tok
	ga.mu.Unlock()

	return ga.saveToken()
}

// Client returns an authenticated HTTP client with automatic token refresh.
func (ga *GoogleAuth) Client(ctx context.Context) (*http.Client, error) {
	ga.mu.RLock()
	tok := ga.token
	ga.mu.RUnlock()

	if tok == nil {
		return nil, fmt.Errorf("not authenticated: visit %s to authorize", ga.AuthURL())
	}

	// TokenSource handles automatic refresh.
	ts := ga.config.TokenSource(ctx, tok)

	// Check if the token was refreshed and persist the new one.
	newTok, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	if newTok.AccessToken != tok.AccessToken {
		ga.mu.Lock()
		ga.token = newTok
		ga.mu.Unlock()
		ga.saveToken()
	}

	return oauth2.NewClient(ctx, ts), nil
}

// IsAuthenticated returns true if a valid token exists.
func (ga *GoogleAuth) IsAuthenticated() bool {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	return ga.token != nil
}

// Revoke clears the token from memory and disk.
func (ga *GoogleAuth) Revoke() error {
	ga.mu.Lock()
	ga.token = nil
	ga.mu.Unlock()

	if err := os.Remove(ga.tokenPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove token file: %w", err)
	}
	return nil
}
