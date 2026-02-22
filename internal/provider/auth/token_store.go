package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
)

// TokenStore handles persistence of OAuth tokens on disk.
type TokenStore struct {
	basePath string
}

// NewTokenStore creates a new token store. basePath is the directory
// where token files are stored (e.g., "/data/eazyclaw/auth/").
func NewTokenStore(basePath string) *TokenStore {
	return &TokenStore{basePath: basePath}
}

// tokenFilePath returns the file path for a provider's token.
func (s *TokenStore) tokenFilePath(provider string) string {
	return filepath.Join(s.basePath, provider+"_oauth.json")
}

// Save persists an OAuth token for the given provider.
func (s *TokenStore) Save(provider string, token *oauth2.Token) error {
	if err := os.MkdirAll(s.basePath, 0700); err != nil {
		return fmt.Errorf("token store: create directory: %w", err)
	}

	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("token store: marshal token: %w", err)
	}

	path := s.tokenFilePath(provider)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("token store: write %s: %w", path, err)
	}

	return nil
}

// Load reads an OAuth token for the given provider from disk.
func (s *TokenStore) Load(provider string) (*oauth2.Token, error) {
	path := s.tokenFilePath(provider)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("token store: read %s: %w", path, err)
	}

	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("token store: unmarshal %s: %w", path, err)
	}

	return &token, nil
}

// Refresh loads the stored token for a provider and refreshes it if expired.
// The refreshed token is saved back to disk.
func (s *TokenStore) Refresh(provider string, cfg OAuthConfig) (*oauth2.Token, error) {
	token, err := s.Load(provider)
	if err != nil {
		return nil, err
	}

	if token.Valid() {
		return token, nil
	}

	// Token is expired; refresh it.
	oauth2Cfg := &oauth2.Config{
		ClientID: cfg.ClientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		},
		RedirectURL: cfg.RedirectURL,
		Scopes:      cfg.Scopes,
	}

	src := oauth2Cfg.TokenSource(context.Background(), token)
	newToken, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("token store: refresh %s: %w", provider, err)
	}

	if err := s.Save(provider, newToken); err != nil {
		return nil, err
	}

	return newToken, nil
}
