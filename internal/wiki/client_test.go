package wiki

import (
	"context"
	"testing"
	"time"
)

func TestNewWikiClient(t *testing.T) {
	config := WikiConfig{
		BaseURL:   "http://localhost:8080/api",
		APIToken:  "test-token",
		SpaceName: "smartclaw",
		Enabled:   true,
	}
	client := NewWikiClient(config)
	if !client.IsEnabled() {
		t.Error("expected client to be enabled")
	}
	if client.GetConfig().BaseURL != config.BaseURL {
		t.Errorf("expected BaseURL %s, got %s", config.BaseURL, client.GetConfig().BaseURL)
	}
}

func TestWikiClient_Disabled(t *testing.T) {
	client := NewWikiClient(WikiConfig{Enabled: false})
	if client.IsEnabled() {
		t.Error("expected client to be disabled")
	}
}

func TestWikiClient_EmptyBaseURL(t *testing.T) {
	client := NewWikiClient(WikiConfig{Enabled: true, BaseURL: ""})
	if client.IsEnabled() {
		t.Error("expected client to be disabled with empty base URL")
	}
}

func TestWikiClient_DefaultTimeout(t *testing.T) {
	client := NewWikiClient(WikiConfig{Enabled: true, BaseURL: "http://localhost:8080/api"})
	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", client.httpClient.Timeout)
	}
}

func TestWikiSearchResult_Empty(t *testing.T) {
	result := &WikiSearchResult{Query: "test", Results: nil, Total: 0}
	if result.Total != 0 {
		t.Errorf("expected 0 results, got %d", result.Total)
	}
}

func TestWikiProvider_Name(t *testing.T) {
	client := NewWikiClient(WikiConfig{})
	provider := NewWikiMemoryProvider(client)
	if provider.Name() != "llmwiki" {
		t.Errorf("expected provider name 'llmwiki', got %s", provider.Name())
	}
}

func TestWikiProvider_ReadOnly(t *testing.T) {
	client := NewWikiClient(WikiConfig{})
	provider := NewWikiMemoryProvider(client)

	err := provider.Store(context.Background(), "key", "value", 0, nil)
	if err == nil {
		t.Error("expected Store to return error for read-only provider")
	}

	err = provider.Delete(context.Background(), "key")
	if err == nil {
		t.Error("expected Delete to return error for read-only provider")
	}
}

func TestWikiProvider_Search_Disabled(t *testing.T) {
	client := NewWikiClient(WikiConfig{Enabled: false})
	provider := NewWikiMemoryProvider(client)

	results, err := provider.Search(context.Background(), "test", 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for disabled provider, got %d", len(results))
	}
}

func TestWikiProvider_List_Unsupported(t *testing.T) {
	client := NewWikiClient(WikiConfig{})
	provider := NewWikiMemoryProvider(client)

	_, err := provider.List(context.Background(), "")
	if err == nil {
		t.Error("expected List to return error for unsupported operation")
	}
}

func TestWikiProvider_Close(t *testing.T) {
	client := NewWikiClient(WikiConfig{})
	provider := NewWikiMemoryProvider(client)

	if err := provider.Close(); err != nil {
		t.Errorf("expected Close to return nil, got %v", err)
	}
}

func TestWikiProvider_Initialize(t *testing.T) {
	client := NewWikiClient(WikiConfig{})
	provider := NewWikiMemoryProvider(client)

	if err := provider.Initialize(context.Background(), nil); err != nil {
		t.Errorf("expected Initialize to return nil, got %v", err)
	}
}

func TestWikiClient_Search_Disabled(t *testing.T) {
	client := NewWikiClient(WikiConfig{Enabled: false})
	result, err := client.Search(context.Background(), "test", 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("expected 0 total, got %d", result.Total)
	}
}

func TestWikiClient_ListPages_Disabled(t *testing.T) {
	client := NewWikiClient(WikiConfig{Enabled: false})
	pages, err := client.ListPages(context.Background(), 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if pages != nil {
		t.Errorf("expected nil pages for disabled client, got %v", pages)
	}
}

func TestWikiClient_GetPage_Disabled(t *testing.T) {
	client := NewWikiClient(WikiConfig{Enabled: false})
	_, err := client.GetPage(context.Background(), "page1")
	if err == nil {
		t.Error("expected error for GetPage on disabled client")
	}
}
