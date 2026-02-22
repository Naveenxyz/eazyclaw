package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"golang.org/x/oauth2"
)

// Profile represents an authentication profile for a provider.
type Profile struct {
	Name       string        `json:"name"`
	Provider   string        `json:"provider"`
	AuthType   string        `json:"auth_type"` // "api_key" | "oauth"
	APIKey     string        `json:"api_key,omitempty"`
	OAuthToken *oauth2.Token `json:"oauth_token,omitempty"`
}

// profilesFile is the on-disk format for the profiles JSON file.
type profilesFile struct {
	Profiles []Profile         `json:"profiles"`
	Active   map[string]string `json:"active"` // provider name → profile name
}

// ProfileManager manages authentication profiles.
type ProfileManager struct {
	mu       sync.RWMutex
	path     string
	profiles map[string]*Profile // profile name → profile
	active   map[string]string   // provider name → active profile name
}

// LoadProfiles loads profiles from a JSON file. If the file does not exist,
// an empty ProfileManager is returned.
func LoadProfiles(path string) (*ProfileManager, error) {
	pm := &ProfileManager{
		path:     path,
		profiles: make(map[string]*Profile),
		active:   make(map[string]string),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pm, nil
		}
		return nil, fmt.Errorf("profiles: read %s: %w", path, err)
	}

	var pf profilesFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("profiles: unmarshal %s: %w", path, err)
	}

	for i := range pf.Profiles {
		p := pf.Profiles[i]
		pm.profiles[p.Name] = &p
	}

	if pf.Active != nil {
		pm.active = pf.Active
	}

	return pm, nil
}

// GetProfile returns a profile by name.
func (pm *ProfileManager) GetProfile(name string) (*Profile, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.profiles[name]
	return p, ok
}

// ActiveProfile returns the active profile for the given provider.
func (pm *ProfileManager) ActiveProfile(provider string) (*Profile, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	name, ok := pm.active[provider]
	if !ok {
		return nil, false
	}

	p, ok := pm.profiles[name]
	return p, ok
}

// SetActive sets the active profile for a provider.
func (pm *ProfileManager) SetActive(provider, profileName string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.profiles[profileName]; !ok {
		return fmt.Errorf("profiles: profile %q not found", profileName)
	}

	pm.active[provider] = profileName
	return pm.saveLocked()
}

// AddProfile adds a new profile. If a profile with the same name exists, it is overwritten.
func (pm *ProfileManager) AddProfile(p *Profile) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.profiles[p.Name] = p
	return pm.saveLocked()
}

// ListProfiles returns all profiles.
func (pm *ProfileManager) ListProfiles() []*Profile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	profiles := make([]*Profile, 0, len(pm.profiles))
	for _, p := range pm.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// saveLocked writes the current state to disk. Must be called with mu held.
func (pm *ProfileManager) saveLocked() error {
	pf := profilesFile{
		Profiles: make([]Profile, 0, len(pm.profiles)),
		Active:   pm.active,
	}
	for _, p := range pm.profiles {
		pf.Profiles = append(pf.Profiles, *p)
	}

	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return fmt.Errorf("profiles: marshal: %w", err)
	}

	if err := os.WriteFile(pm.path, data, 0600); err != nil {
		return fmt.Errorf("profiles: write %s: %w", pm.path, err)
	}

	return nil
}
