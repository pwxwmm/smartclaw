package wiki

import (
	"context"
	"fmt"
	"time"

	"github.com/instructkr/smartclaw/internal/memory"
)

type WikiMemoryProvider struct {
	client *WikiClient
}

func NewWikiMemoryProvider(client *WikiClient) *WikiMemoryProvider {
	return &WikiMemoryProvider{client: client}
}

func (wmp *WikiMemoryProvider) Name() string {
	return "llmwiki"
}

func (wmp *WikiMemoryProvider) Initialize(ctx context.Context, config memory.ProviderConfig) error {
	return nil
}

func (wmp *WikiMemoryProvider) Store(ctx context.Context, key string, value any, ttl time.Duration, tags []string) error {
	return fmt.Errorf("wiki provider is read-only")
}

func (wmp *WikiMemoryProvider) Retrieve(ctx context.Context, key string) (any, error) {
	page, err := wmp.client.GetPage(ctx, key)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (wmp *WikiMemoryProvider) Search(ctx context.Context, query string, limit int) ([]memory.SearchResult, error) {
	if !wmp.client.IsEnabled() {
		return nil, nil
	}

	result, err := wmp.client.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	var results []memory.SearchResult
	for _, page := range result.Results {
		results = append(results, memory.SearchResult{
			Key:       page.ID,
			Value:     page.Content,
			Relevance: 1.0,
			Tags:      page.Tags,
		})
	}
	return results, nil
}

func (wmp *WikiMemoryProvider) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("wiki provider is read-only")
}

func (wmp *WikiMemoryProvider) List(ctx context.Context, prefix string) ([]string, error) {
	return nil, fmt.Errorf("wiki provider list not supported")
}

func (wmp *WikiMemoryProvider) Close() error {
	return nil
}
