package provider

import (
	"fmt"
	"sync"
)

// Registry manages available LLM providers and model-to-provider mappings.
type Registry struct {
	mu              sync.RWMutex
	providers       map[string]Provider
	modelToProvider map[string]string // model name → provider name
	defaultModel    string
}

// NewRegistry creates a new provider registry.
func NewRegistry(defaultModel string) *Registry {
	return &Registry{
		providers:       make(map[string]Provider),
		modelToProvider: make(map[string]string),
		defaultModel:    defaultModel,
	}
}

// Register adds a provider and its associated model to the registry.
func (r *Registry) Register(p Provider, model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
	r.modelToProvider[model] = p.Name()
}

// DefaultProvider returns the provider mapped to the default model.
func (r *Registry) DefaultProvider() (Provider, string, error) {
	return r.ForModel(r.defaultModel)
}

// ForModel returns the provider and model name for a given model string.
func (r *Registry) ForModel(model string) (Provider, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	name, ok := r.modelToProvider[model]
	if !ok {
		return nil, "", fmt.Errorf("no provider registered for model %q", model)
	}
	p, ok := r.providers[name]
	if !ok {
		return nil, "", fmt.Errorf("provider %q not found", name)
	}
	return p, model, nil
}
