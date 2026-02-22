package provider

import (
	"os"
	"sort"
	"strings"

	"github.com/eazyclaw/eazyclaw/internal/config"
)

// modelPrefixToProvider maps model name prefixes to provider names.
var modelPrefixToProvider = map[string]string{
	"claude-": "anthropic",
	"gpt-":    "openai",
	"gemini-": "gemini",
	"kimi-":   "moonshot",
	"k2":      "kimi-coding",
	"glm-":    "zhipu",
}

// ListProviders returns the names of all registered providers.
func (r *Registry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Get retrieves a provider by name.
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// RegisterModel maps a model name to a provider name.
func (r *Registry) RegisterModel(model, providerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modelToProvider[model] = providerName
}

// ForModelByPrefix resolves a provider by matching the model name prefix.
// It first checks exact model registrations, then falls back to prefix matching.
func (r *Registry) ForModelByPrefix(model string) (Provider, bool) {
	// Try exact match first via existing ForModel.
	p, _, err := r.ForModel(model)
	if err == nil {
		return p, true
	}

	// Fall back to prefix matching.
	r.mu.RLock()
	defer r.mu.RUnlock()
	for prefix, provName := range modelPrefixToProvider {
		if strings.HasPrefix(model, prefix) {
			if p, ok := r.providers[provName]; ok {
				return p, true
			}
		}
	}
	return nil, false
}

// AutoRegister checks environment variables and the provided config to
// register all available providers. It also sets up model-to-provider
// mappings based on model prefixes.
func (r *Registry) AutoRegister(cfg config.ProvidersConfig) {
	// Anthropic
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		model := cfg.Anthropic.Model
		if model == "" {
			model = "claude-sonnet-4-6"
		}
		p := NewAnthropicProvider(key, model)
		r.Register(p, model)
	}

	// OpenAI
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		model := cfg.OpenAI.Model
		if model == "" {
			model = "gpt-4o"
		}
		baseURL := cfg.OpenAI.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		p := NewOpenAICompatProvider("openai", key, model, baseURL)
		r.Register(p, model)
	}

	// Gemini
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		model := cfg.Gemini.Model
		if model == "" {
			model = "gemini-2.0-flash"
		}
		p := NewGeminiProvider(key, model)
		r.Register(p, model)
	}

	// Kimi Coding (Anthropic Messages API compatible)
	if key := os.Getenv("KIMI_API_KEY"); key != "" {
		model := cfg.KimiCoding.Model
		if model == "" {
			model = "k2p5"
		}
		baseURL := cfg.KimiCoding.BaseURL
		if baseURL == "" {
			baseURL = "https://api.kimi.com/coding/"
		}
		p := NewAnthropicCompatProvider("kimi-coding", key, model, baseURL)
		r.Register(p, model)
	}

	// Moonshot
	if key := os.Getenv("MOONSHOT_API_KEY"); key != "" {
		model := cfg.Moonshot.Model
		if model == "" {
			model = "kimi-latest"
		}
		baseURL := cfg.Moonshot.BaseURL
		if baseURL == "" {
			baseURL = "https://api.moonshot.ai/v1"
		}
		p := NewOpenAICompatProvider("moonshot", key, model, baseURL)
		r.Register(p, model)
	}

	// Zhipu
	if key := os.Getenv("ZHIPU_API_KEY"); key != "" {
		model := cfg.Zhipu.Model
		if model == "" {
			model = "glm-4"
		}
		baseURL := cfg.Zhipu.BaseURL
		if baseURL == "" {
			baseURL = "https://open.bigmodel.cn/api/paas/v4"
		}
		p := NewOpenAICompatProvider("zhipu", key, model, baseURL)
		r.Register(p, model)
	}
}
