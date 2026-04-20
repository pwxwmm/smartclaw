package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ProviderConfig holds generic configuration for a memory provider.
type ProviderConfig map[string]any

// GetString retrieves a string value from the config, returning the default if missing or wrong type.
func (c ProviderConfig) GetString(key string, defaultVal string) string {
	if v, ok := c[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

// GetInt retrieves an int value from the config, returning the default if missing or wrong type.
func (c ProviderConfig) GetInt(key string, defaultVal int) int {
	if v, ok := c[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return defaultVal
}

// GetDuration retrieves a duration value from the config, returning the default if missing or wrong type.
func (c ProviderConfig) GetDuration(key string, defaultVal time.Duration) time.Duration {
	if v, ok := c[key]; ok {
		if d, ok := v.(time.Duration); ok {
			return d
		}
	}
	return defaultVal
}

// SearchResult represents a single result from a provider search operation.
type SearchResult struct {
	Key       string
	Value     any
	Relevance float64
	Tags      []string
	CreatedAt time.Time
}

// MemoryProvider is the interface that all memory backends must implement.
// Providers are pluggable storage backends for SmartClaw's memory system,
// following the pattern established by the Hermes Agent architecture.
type MemoryProvider interface {
	// Name returns the unique identifier for this provider.
	Name() string

	// Initialize sets up the provider with the given configuration.
	Initialize(ctx context.Context, config ProviderConfig) error

	// Store persists a value under the given key with optional TTL and tags.
	Store(ctx context.Context, key string, value any, ttl time.Duration, tags []string) error

	// Retrieve fetches the value associated with the given key.
	Retrieve(ctx context.Context, key string) (any, error)

	// Search queries the provider for entries matching the query, limited to limit results.
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)

	// Delete removes the entry associated with the given key.
	Delete(ctx context.Context, key string) error

	// List returns all keys matching the given prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// Close releases any resources held by the provider.
	Close() error
}

// Lifecycle hook function types. Hooks are called after the corresponding
// operation completes successfully. They are informational only; errors
// from hooks are logged but do not affect the operation result.

// OnStoreHook is called after a successful Store operation.
type OnStoreHook func(ctx context.Context, key string, value any, ttl time.Duration, tags []string)

// OnRetrieveHook is called after a successful Retrieve operation.
type OnRetrieveHook func(ctx context.Context, key string, value any)

// OnDeleteHook is called after a successful Delete operation.
type OnDeleteHook func(ctx context.Context, key string)

// OnSearchHook is called after a successful Search operation.
type OnSearchHook func(ctx context.Context, query string, results []SearchResult)

// ProviderRegistry manages the set of available MemoryProvider implementations.
// It supports registering, retrieving, and listing providers, and maintains
// a global default provider.
type ProviderRegistry struct {
	mu       sync.RWMutex
	providers map[string]MemoryProvider
	defaultName string
}

// globalRegistry is the package-level default registry.
var globalRegistry = NewProviderRegistry()

// NewProviderRegistry creates a new empty ProviderRegistry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]MemoryProvider),
	}
}

// Register adds a provider to the registry under the given name.
// If a provider with the same name already exists, it returns an error.
// The first provider registered becomes the default unless SetDefault is called.
func (r *ProviderRegistry) Register(name string, provider MemoryProvider) error {
	if name == "" {
		return fmt.Errorf("provider registry: name must not be empty")
	}
	if provider == nil {
		return fmt.Errorf("provider registry: provider must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider registry: provider %q already registered", name)
	}

	r.providers[name] = provider

	// First provider becomes the default.
	if r.defaultName == "" {
		r.defaultName = name
	}

	return nil
}

// Get retrieves a provider by name. Returns an error if not found.
func (r *ProviderRegistry) Get(name string) (MemoryProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider registry: provider %q not found", name)
	}
	return p, nil
}

// List returns the names of all registered providers, sorted alphabetically.
func (r *ProviderRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SetDefault sets the default provider by name. Returns an error if the
// provider is not registered.
func (r *ProviderRegistry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("provider registry: cannot set default: provider %q not found", name)
	}
	r.defaultName = name
	return nil
}

// GetDefault returns the default provider. Returns an error if no default is set.
func (r *ProviderRegistry) GetDefault() (MemoryProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultName == "" {
		return nil, fmt.Errorf("provider registry: no default provider set")
	}
	p, ok := r.providers[r.defaultName]
	if !ok {
		return nil, fmt.Errorf("provider registry: default provider %q not found", r.defaultName)
	}
	return p, nil
}

// Unregister removes a provider from the registry. If the removed provider
// was the default, the default is cleared (must be explicitly re-set).
func (r *ProviderRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("provider registry: provider %q not found", name)
	}

	delete(r.providers, name)

	if r.defaultName == name {
		r.defaultName = ""
	}

	return nil
}

// Global registry convenience functions.

// RegisterProvider registers a provider in the global registry.
func RegisterProvider(name string, provider MemoryProvider) error {
	return globalRegistry.Register(name, provider)
}

// GetProvider retrieves a provider from the global registry.
func GetProvider(name string) (MemoryProvider, error) {
	return globalRegistry.Get(name)
}

// ListProviders returns all provider names from the global registry.
func ListProviders() []string {
	return globalRegistry.List()
}

// SetDefaultProvider sets the default provider in the global registry.
func SetDefaultProvider(name string) error {
	return globalRegistry.SetDefault(name)
}

// GetDefaultProvider returns the default provider from the global registry.
func GetDefaultProvider() (MemoryProvider, error) {
	return globalRegistry.GetDefault()
}
