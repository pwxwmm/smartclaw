// Package moreright is a stub completion service.
// Deprecated: This package is a stub and will be removed in a future version.
package moreright

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type MoreRightConfig struct {
	Enabled   bool
	APIKey    string
	Endpoint  string
	RateLimit int
	Timeout   time.Duration
}

type MoreRightClient struct {
	config *MoreRightConfig
	client *http.Client
}

func NewMoreRightClient(config MoreRightConfig) *MoreRightClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &MoreRightClient{
		config: &config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

func (m *MoreRightClient) IsEnabled() bool {
	return m.config.Enabled
}

func (m *MoreRightClient) GetCapabilities() []string {
	return []string{
		"code_completion",
		"code_generation",
		"code_review",
		"refactoring",
	}
}

type CompletionRequest struct {
	Code      string `json:"code"`
	Language  string `json:"language"`
	MaxTokens int    `json:"max_tokens"`
}

type CompletionResponse struct {
	Completion string  `json:"completion"`
	Confidence float64 `json:"confidence"`
}

func (m *MoreRightClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("more-right is disabled")
	}
	return &CompletionResponse{
		Completion: "Completion not available",
		Confidence: 0.0,
	}, nil
}

type GenerationRequest struct {
	Prompt   string `json:"prompt"`
	Language string `json:"language"`
	Style    string `json:"style"`
}

type GenerationResponse struct {
	Code     string `json:"code"`
	Language string `json:"language"`
	Tokens   int    `json:"tokens"`
}

func (m *MoreRightClient) Generate(ctx context.Context, req GenerationRequest) (*GenerationResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("more-right is disabled")
	}
	return &GenerationResponse{
		Code:     "// Generated code placeholder",
		Language: req.Language,
		Tokens:   0,
	}, nil
}

func (m *MoreRightClient) Analyze(code string) (map[string]any, error) {
	lines := strings.Count(code, "\n")
	chars := len(code)
	return map[string]any{
		"lines":      lines,
		"characters": chars,
		"words":      len(strings.Fields(code)),
	}, nil
}
