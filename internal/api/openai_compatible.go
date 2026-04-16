package api

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/constants"
	"github.com/instructkr/smartclaw/internal/resilience"
)

var providerBreakers = map[string]*resilience.CircuitBreaker{
	"anthropic": resilience.NewCircuitBreaker("anthropic", 5, 30*time.Second),
	"openai":    resilience.NewCircuitBreaker("openai", 5, 30*time.Second),
	"google":    resilience.NewCircuitBreaker("google", 5, 30*time.Second),
	"azure":     resilience.NewCircuitBreaker("azure", 5, 30*time.Second),
}

func OpenAICompatibleProviders() map[string]string {
	return map[string]string{
		"groq":        "https://api.groq.com/openai",
		"together":    "https://api.together.xyz/v1",
		"deepinfra":   "https://api.deepinfra.com/v1/openai",
		"openrouter":  "https://openrouter.ai/api/v1",
		"xai":         "https://api.x.ai/v1",
		"perplexity":  "https://api.perplexity.ai",
		"mistral":     "https://api.mistral.ai/v1",
		"cerebras":    "https://api.cerebras.ai/v1",
		"siliconflow": "https://api.siliconflow.cn/v1",
		"deepseek":    "https://api.deepseek.com",
	}
}

func OpenAICompatibleProviderHeaders() map[string]map[string]string {
	return map[string]map[string]string{
		"openrouter": {
			"HTTP-Referer": "https://smartclaw.dev",
			"X-Title":      "SmartClaw",
		},
	}
}

func NewOpenAICompatibleClient(apiKey, provider, model string) *Client {
	providers := OpenAICompatibleProviders()
	baseURL, ok := providers[provider]
	if !ok {
		baseURL = provider
	}

	headers := map[string]string{}
	if providerHeaders, exists := OpenAICompatibleProviderHeaders()[provider]; exists {
		headers = providerHeaders
	}

	return &Client{
		APIKey:          apiKey,
		BaseURL:         baseURL,
		Model:           model,
		IsOpenAI:        true,
		ProviderHeaders: headers,
		HTTPClient:      defaultHTTPClient(provider),
		openaiSDKClient: newOpenAISDKClient(apiKey, baseURL, headers),
	}
}

type ProviderConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

func NewClientForProvider(provider string, cfg ProviderConfig) (*Client, error) {
	switch provider {
	case "anthropic":
		c := &Client{
			APIKey:     cfg.APIKey,
			BaseURL:    DefaultBaseURL,
			Model:      cfg.Model,
			IsOpenAI:   false,
			HTTPClient: defaultHTTPClient("anthropic"),
		}
		c.sdkClient = newAnthropicSDKClient(cfg.APIKey, DefaultBaseURL, nil)
		return c, nil
	case "google", "gemini":
		return NewGoogleClient(cfg.APIKey, cfg.Model), nil
	case "azure":
		return NewAzureClient(cfg.APIKey, cfg.BaseURL, cfg.Model), nil
	default:
		knownProviders := OpenAICompatibleProviders()
		if _, isKnown := knownProviders[provider]; isKnown || cfg.BaseURL != "" {
			baseURL := cfg.BaseURL
			if baseURL == "" {
				baseURL = knownProviders[provider]
			}
			headers := map[string]string{}
			if providerHeaders, exists := OpenAICompatibleProviderHeaders()[provider]; exists {
				headers = providerHeaders
			}
			return &Client{
				APIKey:          cfg.APIKey,
				BaseURL:         baseURL,
				Model:           cfg.Model,
				IsOpenAI:        true,
				ProviderHeaders: headers,
				HTTPClient:      defaultHTTPClient(provider),
				openaiSDKClient: newOpenAISDKClient(cfg.APIKey, baseURL, headers),
			}, nil
		}
		return nil, fmt.Errorf("provider %q not recognized and no base_url provided", provider)
	}
}

func defaultHTTPClient(provider string) *http.Client {
	breaker := providerBreakers[provider]
	if breaker == nil {
		breaker = resilience.NewCircuitBreaker(provider, 5, 30*time.Second)
	}
	limiter := resilience.NewRateLimiter(50, time.Second)

	baseTransport := &http.Transport{
		TLSHandshakeTimeout:   time.Duration(constants.APITLSHandshakeTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(constants.APIResponseHeaderTimeout) * time.Second,
		IdleConnTimeout:       time.Duration(constants.APIIdleConnTimeout) * time.Second,
	}

	resilientTransport := resilience.NewResilientTransport(baseTransport, breaker, limiter)

	return &http.Client{
		Timeout: time.Duration(constants.APIClientTimeout) * time.Second,
		Transport: &retryTransport{
			maxRetries: 2,
			backoff:    500 * time.Millisecond,
			inner:      resilientTransport,
		},
	}
}

type retryTransport struct {
	maxRetries int
	backoff    time.Duration
	inner      http.RoundTripper
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	isIdempotent := req.Method == "GET" || req.Method == "HEAD" || req.Method == "PUT" || req.Method == "DELETE"
	isLLMEndpoint := strings.Contains(req.URL.Path, "/messages") || strings.Contains(req.URL.Path, "/completions") || strings.Contains(req.URL.Path, "/chat")
	if isLLMEndpoint {
		isIdempotent = true
	}

	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
	}

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= t.maxRetries; attempt++ {
		if attempt > 0 {
			if !isIdempotent {
				break
			}
			delay := t.backoff * time.Duration(1<<(attempt-1))
			jitter := time.Duration(rand.Int63n(int64(delay / 2)))
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay + jitter):
			}
		}

		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}

		resp, err = t.inner.RoundTrip(req)
		if err != nil {
			if attempt < t.maxRetries && isIdempotent {
				continue
			}
			return nil, err
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			if attempt < t.maxRetries && isIdempotent {
				continue
			}
		}

		return resp, nil
	}

	if resp != nil {
		return resp, nil
	}
	return nil, err
}
