package provider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Resolver manages provider configurations and resolves model names to providers.
type Resolver struct {
	mu        sync.RWMutex
	providers map[string]*Provider // keyed by "name/model" or just "name"
	primary   string               // key of the primary provider
}

// NewResolver creates an empty resolver.
func NewResolver() *Resolver {
	return &Resolver{
		providers: make(map[string]*Provider),
	}
}

// LoadFromConfig reads provider configuration from ~/.smartclaw/config.json.
func LoadFromConfig() (*Resolver, error) {
	r := NewResolver()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return r, nil // no home dir, return empty
	}

	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config file — try env var fallback
		r.loadFromEnv()
		return r, nil
	}

	var cfg struct {
		Providers []Provider `json:"providers"`
		Routing   struct {
			Strategy        string `json:"strategy"`
			FallbackOnError bool   `json:"fallback_on_error"`
		} `json:"routing"`
		// Legacy single-provider fields
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
		Model   string `json:"model"`
		OpenAI  bool   `json:"openai"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Register explicit providers
	for i := range cfg.Providers {
		p := cfg.Providers[i]
		key := p.Name
		if key == "" {
			key = fmt.Sprintf("provider-%d", i)
		}
		r.Register(key, &p)
	}

	// If no providers registered, try legacy single-provider config
	if len(r.providers) == 0 && cfg.APIKey != "" {
		mode := ModeChatCompletions
		if !cfg.OpenAI {
			mode = ModeAnthropicMessages
		}
		p := &Provider{
			Name:    "default",
			APIKey:  cfg.APIKey,
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
			Mode:    mode,
		}
		if p.BaseURL == "" {
			if p.Mode == ModeAnthropicMessages {
				p.BaseURL = "https://api.anthropic.com"
			} else {
				p.BaseURL = "https://api.openai.com"
			}
		}
		if p.Model == "" {
			if p.Mode == ModeAnthropicMessages {
				p.Model = "claude-sonnet-4-5"
			} else {
				p.Model = "gpt-4o"
			}
		}
		r.Register("default", p)
	}

	return r, nil
}

// loadFromEnv falls back to environment variables when no config file exists.
func (r *Resolver) loadFromEnv() {
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		r.Register("anthropic", &Provider{
			Name:    "anthropic",
			APIKey:  apiKey,
			BaseURL: "https://api.anthropic.com",
			Model:   "claude-sonnet-4-5",
			Mode:    ModeAnthropicMessages,
		})
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		r.Register("openai", &Provider{
			Name:    "openai",
			APIKey:  apiKey,
			BaseURL: "https://api.openai.com",
			Model:   "gpt-4o",
			Mode:    ModeChatCompletions,
		})
	}
	if apiKey := os.Getenv("ZHIPU_API_KEY"); apiKey != "" {
		r.Register("zhipu", &Provider{
			Name:    "zhipu",
			APIKey:  apiKey,
			BaseURL: "https://open.bigmodel.cn/api/paas/v4",
			Model:   "glm-5",
			Mode:    ModeChatCompletions,
		})
	}
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		r.Register("openrouter", &Provider{
			Name:     "openrouter",
			APIKey:   apiKey,
			BaseURL:  "https://openrouter.ai/api/v1",
			Model:    "anthropic/claude-sonnet-4-5",
			Mode:     ModeChatCompletions,
			Fallback: true,
		})
	}
}

// Register adds a provider to the resolver. The first non-fallback provider becomes primary.
func (r *Resolver) Register(key string, p *Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p.Name == "" {
		p.Name = key
	}

	r.providers[key] = p

	// First non-fallback provider is primary
	if r.primary == "" && !p.Fallback {
		r.primary = key
	}
}

// Resolve returns the provider for the given key, or the primary if key is empty.
func (r *Resolver) Resolve(key string) (*Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if key == "" {
		key = r.primary
	}

	if key == "" {
		// Try any provider
		for k, p := range r.providers {
			if !p.Fallback {
				return p, nil
			}
			_ = k
		}
		// Try fallbacks
		for _, p := range r.providers {
			return p, nil
		}
		return nil, fmt.Errorf("no providers configured")
	}

	p, ok := r.providers[key]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", key)
	}
	return p, nil
}

// ResolveModel finds a provider that serves the given model name.
// Model names are matched case-insensitively against provider.Model.
func (r *Resolver) ResolveModel(model string) (*Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// First pass: exact match on non-fallback
	for _, p := range r.providers {
		if !p.Fallback && equalModel(p.Model, model) {
			return p, nil
		}
	}

	// Second pass: prefix match (e.g., "claude" matches "claude-sonnet-4-5")
	for _, p := range r.providers {
		if !p.Fallback && modelPrefixMatch(p.Model, model) {
			return p, nil
		}
	}

	// Third pass: fallback providers
	for _, p := range r.providers {
		if p.Fallback && equalModel(p.Model, model) {
			return p, nil
		}
	}

	// Return primary if no model match
	if r.primary != "" {
		if p, ok := r.providers[r.primary]; ok {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no provider found for model %q", model)
}

// Primary returns the primary (default) provider.
func (r *Resolver) Primary() (*Provider, error) {
	return r.Resolve("")
}

// Fallbacks returns all fallback providers in registration order.
func (r *Resolver) Fallbacks() []*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var fallbacks []*Provider
	for _, p := range r.providers {
		if p.Fallback {
			fallbacks = append(fallbacks, p)
		}
	}
	return fallbacks
}

// List returns all registered providers.
func (r *Resolver) List() []*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Provider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

// SetPrimary changes the primary provider by key.
func (r *Resolver) SetPrimary(key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[key]; !ok {
		return fmt.Errorf("provider %q not found", key)
	}
	r.primary = key
	return nil
}

func equalModel(a, b string) bool {
	if a == b {
		return true
	}
	// Normalize common aliases
	return normalizeModel(a) == normalizeModel(b)
}

func normalizeModel(m string) string {
	// Strip common prefixes for comparison
	s := m
	for _, prefix := range []string{"anthropic/", "openai/", "google/", "meta-llama/"} {
		if len(s) > len(prefix) && s[:len(prefix)] == prefix {
			s = s[len(prefix):]
		}
	}
	return s
}

func modelPrefixMatch(providerModel, query string) bool {
	pm := normalizeModel(providerModel)
	q := normalizeModel(query)
	if len(q) > len(pm) {
		return false
	}
	return pm[:len(q)] == q
}
