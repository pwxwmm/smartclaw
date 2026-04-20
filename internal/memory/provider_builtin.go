package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/memory/layers"
)

// BuiltinProvider wraps the existing MemoryStore (file-based) for
// Store/Retrieve/Delete/List and delegates Search to SessionSearch (FTS5).
// It implements the MemoryProvider interface and calls lifecycle hooks on
// each operation.
type BuiltinProvider struct {
	store         *MemoryStore
	sessionSearch *layers.SessionSearch
	initialized   bool

	onStore    OnStoreHook
	onRetrieve OnRetrieveHook
	onDelete   OnDeleteHook
	onSearch   OnSearchHook
}

// NewBuiltinProvider creates a BuiltinProvider that wraps the given
// MemoryStore and SessionSearch. Either may be nil, in which case the
// corresponding operations will return errors.
func NewBuiltinProvider(store *MemoryStore, sessionSearch *layers.SessionSearch) *BuiltinProvider {
	return &BuiltinProvider{
		store:         store,
		sessionSearch: sessionSearch,
	}
}

// SetOnStoreHook sets the lifecycle hook called after a successful Store.
func (p *BuiltinProvider) SetOnStoreHook(hook OnStoreHook) {
	p.onStore = hook
}

// SetOnRetrieveHook sets the lifecycle hook called after a successful Retrieve.
func (p *BuiltinProvider) SetOnRetrieveHook(hook OnRetrieveHook) {
	p.onRetrieve = hook
}

// SetOnDeleteHook sets the lifecycle hook called after a successful Delete.
func (p *BuiltinProvider) SetOnDeleteHook(hook OnDeleteHook) {
	p.onDelete = hook
}

// SetOnSearchHook sets the lifecycle hook called after a successful Search.
func (p *BuiltinProvider) SetOnSearchHook(hook OnSearchHook) {
	p.onSearch = hook
}

func (p *BuiltinProvider) Name() string {
	return "builtin"
}

func (p *BuiltinProvider) Initialize(_ context.Context, _ ProviderConfig) error {
	if p.store == nil {
		return fmt.Errorf("builtin provider: MemoryStore is nil")
	}
	p.initialized = true
	return nil
}

func (p *BuiltinProvider) Store(_ context.Context, key string, value any, ttl time.Duration, tags []string) error {
	if p.store == nil {
		return fmt.Errorf("builtin provider: store: MemoryStore is nil")
	}

	if err := p.store.Set(key, value, ttl); err != nil {
		return fmt.Errorf("builtin provider: store: %w", err)
	}

	if len(tags) > 0 {
		for _, tag := range tags {
			if err := p.store.AddTag(key, tag); err != nil {
				slog.Warn("builtin provider: failed to add tag", "key", key, "tag", tag, "error", err)
			}
		}
	}

	if p.onStore != nil {
		p.onStore(context.Background(), key, value, ttl, tags)
	}

	return nil
}

func (p *BuiltinProvider) Retrieve(_ context.Context, key string) (any, error) {
	if p.store == nil {
		return nil, fmt.Errorf("builtin provider: retrieve: MemoryStore is nil")
	}

	value, err := p.store.Get(key)
	if err != nil {
		return nil, fmt.Errorf("builtin provider: retrieve: %w", err)
	}

	if p.onRetrieve != nil {
		p.onRetrieve(context.Background(), key, value)
	}

	return value, nil
}

func (p *BuiltinProvider) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	var results []SearchResult

	if p.sessionSearch != nil {
		fragments, err := p.sessionSearch.Search(ctx, query, limit)
		if err != nil {
			return nil, fmt.Errorf("builtin provider: search: %w", err)
		}

		for _, f := range fragments {
			results = append(results, SearchResult{
				Key:       f.SessionID,
				Value:     f.Content,
				Relevance: f.Relevance,
				CreatedAt: f.Timestamp,
			})
		}
	}

	if p.store != nil {
		memories := p.store.Search(query)
		for _, m := range memories {
			results = append(results, SearchResult{
				Key:       m.Key,
				Value:     m.Value,
				Relevance: 1.0,
				Tags:      m.Tags,
				CreatedAt: m.CreatedAt,
			})
		}
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	if p.onSearch != nil {
		p.onSearch(ctx, query, results)
	}

	return results, nil
}

func (p *BuiltinProvider) Delete(_ context.Context, key string) error {
	if p.store == nil {
		return fmt.Errorf("builtin provider: delete: MemoryStore is nil")
	}

	if err := p.store.Delete(key); err != nil {
		return fmt.Errorf("builtin provider: delete: %w", err)
	}

	if p.onDelete != nil {
		p.onDelete(context.Background(), key)
	}

	return nil
}

func (p *BuiltinProvider) List(_ context.Context, prefix string) ([]string, error) {
	if p.store == nil {
		return nil, fmt.Errorf("builtin provider: list: MemoryStore is nil")
	}

	allKeys := p.store.List()

	if prefix == "" {
		return allKeys, nil
	}

	var filtered []string
	for _, key := range allKeys {
		if strings.HasPrefix(key, prefix) {
			filtered = append(filtered, key)
		}
	}

	return filtered, nil
}

func (p *BuiltinProvider) Close() error {
	p.initialized = false
	return nil
}
