package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WikiConfig holds configuration for the wiki HTTP client.
type WikiConfig struct {
	BaseURL   string        `json:"base_url"`  // e.g., "http://localhost:8080/api"
	APIToken  string        `json:"api_token"` // auth token for llmwiki
	SpaceName string        `json:"space_name"` // wiki space/project name
	Timeout   time.Duration `json:"timeout"`    // HTTP timeout (default: 10s)
	Enabled   bool          `json:"enabled"`
}

// WikiPage represents a single page from the wiki.
type WikiPage struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Tags      []string          `json:"tags,omitempty"`
	UpdatedAt string            `json:"updated_at,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// WikiSearchResult holds the response from a wiki search query.
type WikiSearchResult struct {
	Query   string     `json:"query"`
	Results []WikiPage `json:"results"`
	Total   int        `json:"total"`
}

// WikiClient is an HTTP client for the llmwiki API.
type WikiClient struct {
	config     WikiConfig
	httpClient *http.Client
}

// NewWikiClient creates a new WikiClient with the given configuration.
func NewWikiClient(config WikiConfig) *WikiClient {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &WikiClient{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// Search queries the wiki for pages matching the given query.
func (wc *WikiClient) Search(ctx context.Context, query string, limit int) (*WikiSearchResult, error) {
	if !wc.config.Enabled || wc.config.BaseURL == "" {
		return &WikiSearchResult{Query: query, Results: nil, Total: 0}, nil
	}

	u := fmt.Sprintf("%s/search?q=%s&limit=%d&space=%s",
		wc.config.BaseURL,
		url.QueryEscape(query),
		limit,
		url.QueryEscape(wc.config.SpaceName))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("wiki search request: %w", err)
	}
	if wc.config.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+wc.config.APIToken)
	}

	resp, err := wc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wiki search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("wiki search returned %d: %s", resp.StatusCode, string(body))
	}

	var result WikiSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("wiki search decode: %w", err)
	}
	return &result, nil
}

// GetPage retrieves a single wiki page by its ID.
func (wc *WikiClient) GetPage(ctx context.Context, pageID string) (*WikiPage, error) {
	if !wc.config.Enabled || wc.config.BaseURL == "" {
		return nil, fmt.Errorf("wiki not configured")
	}

	u := fmt.Sprintf("%s/pages/%s?space=%s",
		wc.config.BaseURL,
		url.PathEscape(pageID),
		url.QueryEscape(wc.config.SpaceName))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("wiki get page request: %w", err)
	}
	if wc.config.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+wc.config.APIToken)
	}

	resp, err := wc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wiki get page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wiki get page returned %d", resp.StatusCode)
	}

	var page WikiPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("wiki get page decode: %w", err)
	}
	return &page, nil
}

// ListPages retrieves a list of wiki pages.
func (wc *WikiClient) ListPages(ctx context.Context, limit int) ([]WikiPage, error) {
	if !wc.config.Enabled || wc.config.BaseURL == "" {
		return nil, nil
	}

	u := fmt.Sprintf("%s/pages?limit=%d&space=%s",
		wc.config.BaseURL,
		limit,
		url.QueryEscape(wc.config.SpaceName))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("wiki list pages request: %w", err)
	}
	if wc.config.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+wc.config.APIToken)
	}

	resp, err := wc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wiki list pages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wiki list pages returned %d", resp.StatusCode)
	}

	var pages []WikiPage
	if err := json.NewDecoder(resp.Body).Decode(&pages); err != nil {
		return nil, fmt.Errorf("wiki list pages decode: %w", err)
	}
	return pages, nil
}

// IsEnabled returns true if the wiki client is properly configured and enabled.
func (wc *WikiClient) IsEnabled() bool {
	return wc.config.Enabled && wc.config.BaseURL != ""
}

// GetConfig returns a copy of the wiki client configuration.
func (wc *WikiClient) GetConfig() WikiConfig {
	return wc.config
}
