package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.anthropic.com"
	DefaultVersion = "2023-06-01"
)

// Client is the Anthropic API client
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Model      string
	IsOpenAI   bool
}

// NewClient creates a new API client with default model
func NewClient(apiKey string) *Client {
	return NewClientWithModel(apiKey, "", "")
}

// NewClientWithBaseURL creates a new API client with custom base URL (uses default model)
func NewClientWithBaseURL(apiKey string, baseURL string) *Client {
	return NewClientWithModel(apiKey, baseURL, "")
}

// NewClientWithModel creates a new API client with custom base URL and model
func NewClientWithModel(apiKey string, baseURL string, model string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 300 * time.Second,
			Transport: &http.Transport{
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				IdleConnTimeout:       30 * time.Second,
			},
		},
	}
}

// SetModel sets the model for the client
func (c *Client) SetModel(model string) {
	c.Model = model
}

// SetOpenAI sets whether to use OpenAI-compatible API
func (c *Client) SetOpenAI(isOpenAI bool) {
	c.IsOpenAI = isOpenAI
}

// buildEndpointURL constructs an API endpoint URL from the base URL.
// Handles base URLs that may already include /v1 or the full endpoint path.
// e.g. "http://host:8000" + "/v1/chat/completions" → "http://host:8000/v1/chat/completions"
// e.g. "http://host:8000/v1" + "/v1/chat/completions" → "http://host:8000/v1/chat/completions"
// e.g. "http://host:8000/v1/" + "/v1/chat/completions" → "http://host:8000/v1/chat/completions"
func (c *Client) buildEndpointURL(endpoint string) string {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	cleanEndpoint := strings.TrimLeft(endpoint, "/")

	if strings.HasSuffix(baseURL, "/"+cleanEndpoint) {
		return baseURL
	}

	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/" + strings.TrimPrefix(cleanEndpoint, "v1/")
	}

	return baseURL + "/" + cleanEndpoint
}

// Message is a simple message type for backward compatibility
type Message = MessageParam

// CreateMessage sends a message to the API
func (c *Client) CreateMessage(messages []Message, system string) (*MessageResponse, error) {
	if c.IsOpenAI {
		return c.CreateMessageOpenAI(messages, system)
	}

	req := MessageRequest{
		Model:     c.Model,
		MaxTokens: 4096,
		Messages:  messages,
		System:    system,
		Stream:    false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.buildEndpointURL("/v1/messages"), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", DefaultVersion)
	httpReq.Header.Set("User-Agent", "claude-code/2.1.86")
	httpReq.Header.Set("client-name", "claude-code")
	httpReq.Header.Set("x-client", "Claude Code")
	httpReq.Header.Set("x-client-version", "2.1.86")
	httpReq.Header.Set("accept", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	var result MessageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// StreamMessage sends a message and streams the response
func (c *Client) StreamMessage(messages []Message, system string, onEvent func(event string, data []byte)) error {
	req := MessageRequest{
		Model:     c.Model,
		MaxTokens: 4096,
		Messages:  messages,
		System:    system,
		Stream:    true,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.buildEndpointURL("/v1/messages"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", DefaultVersion)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// TODO: Implement SSE parsing
	// For now, just read the whole response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	onEvent("message", respBody)
	return nil
}
