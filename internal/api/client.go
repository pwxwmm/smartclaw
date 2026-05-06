package api

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/instructkr/smartclaw/internal/constants"
	apperrors "github.com/instructkr/smartclaw/internal/errors"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/openai/openai-go/v3"
)

const (
	DefaultBaseURL = "https://api.anthropic.com"
	DefaultVersion = "2023-06-01"
)

type Client struct {
	APIKey          string
	BaseURL         string
	HTTPClient      *http.Client
	Model           string
	IsOpenAI        bool
	IsGoogle        bool
	Thinking        *ThinkingConfig
	ProviderHeaders map[string]string
	sdkClient       anthropic.Client
	openaiSDKClient openai.Client
}

func NewClient(apiKey string) *Client {
	return NewClientWithModel(apiKey, "", "")
}

func NewClientWithBaseURL(apiKey string, baseURL string) *Client {
	return NewClientWithModel(apiKey, baseURL, "")
}

func NewClientWithModel(apiKey string, baseURL string, model string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if model == "" {
		model = "claude-sonnet-4-5"
	}
	return &Client{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Model:      model,
		HTTPClient: defaultHTTPClient("anthropic"),
		sdkClient:  newAnthropicSDKClient(apiKey, baseURL, nil),
	}
}

func (c *Client) SetModel(model string) {
	c.Model = model
}

func (c *Client) SetOpenAI(isOpenAI bool) {
	c.IsOpenAI = isOpenAI
}

type Message = MessageParam

func (c *Client) CreateMessage(ctx context.Context, messages []Message, system string) (*MessageResponse, error) {
	var systemParam any
	if system != "" {
		systemParam = []SystemBlock{
			{
				Type:         "text",
				Text:         system,
				CacheControl: &CacheControl{Type: "ephemeral"},
			},
		}
	}
	return c.CreateMessageWithSystem(ctx, messages, systemParam)
}

func (c *Client) CreateMessageWithSystem(ctx context.Context, messages []Message, system any) (*MessageResponse, error) {
	provider := c.providerName()
	destHost := c.destinationHost()
	systemPromptLen := c.systemPromptLength(system)
	toolCount := 0
	dataCategories := c.dataCategories(messages, system, toolCount)

	start := time.Now()

	if c.IsGoogle {
		var systemStr string
		if sb, ok := system.([]SystemBlock); ok && len(sb) > 0 {
			systemStr = sb[0].Text
		} else if s, ok := system.(string); ok {
			systemStr = s
		}
		resp, err := c.CreateMessageGoogle(messages, systemStr)
		elapsed := time.Since(start)
		c.recordOutboundAudit(provider, destHost, c.Model, len(messages), systemPromptLen, toolCount, resp, err, elapsed, dataCategories)
		return resp, err
	}

	if c.IsOpenAI {
		c.ensureOpenAISDKClient()

		var systemStr string
		if sb, ok := system.([]SystemBlock); ok && len(sb) > 0 {
			systemStr = sb[0].Text
		} else if s, ok := system.(string); ok {
			systemStr = s
		}

		params := buildSDKOpenAIParams(messages, systemStr, c.Model, constants.APIRequestMaxTokens)

		comp, err := c.openaiSDKClient.Chat.Completions.New(ctx, params)
		elapsed := time.Since(start)
		resp := sdkCompletionToResponseIfNoError(comp, err)
		c.recordOutboundAudit(provider, destHost, c.Model, len(messages), systemPromptLen, toolCount, resp, err, elapsed, dataCategories)
		if err != nil {
			return nil, apperrors.Wrap(err, "OPENAI_API_ERROR", "OpenAI API error",
				apperrors.WithCategory(apperrors.CategoryNetwork))
		}

		return sdkCompletionToResponse(comp), nil
	}

	c.ensureSDKClient()

	params := buildSDKMessages(messages, system, c.Model, constants.APIRequestMaxTokens, c.Thinking, nil)

	msg, err := c.sdkClient.Messages.New(ctx, params)
	elapsed := time.Since(start)
	resp := sdkMessageToResponseIfNoError(msg, err)
	c.recordOutboundAudit(provider, destHost, c.Model, len(messages), systemPromptLen, toolCount, resp, err, elapsed, dataCategories)
	if err != nil {
		return nil, apperrors.Wrap(err, "ANTHROPIC_API_ERROR", "Anthropic API error",
			apperrors.WithCategory(apperrors.CategoryNetwork))
	}

	return sdkMessageToResponse(msg), nil
}

func (c *Client) CreateMessageWithTools(ctx context.Context, messages []Message, system any, tools []ToolDefinition) (*MessageResponse, error) {
	if len(tools) == 0 {
		return c.CreateMessageWithSystem(ctx, messages, system)
	}

	provider := c.providerName()
	destHost := c.destinationHost()
	systemPromptLen := c.systemPromptLength(system)
	toolCount := len(tools)
	dataCategories := c.dataCategories(messages, system, toolCount)

	start := time.Now()

	if c.IsOpenAI {
		c.ensureOpenAISDKClient()

		var systemStr string
		if sb, ok := system.([]SystemBlock); ok && len(sb) > 0 {
			systemStr = sb[0].Text
		} else if s, ok := system.(string); ok {
			systemStr = s
		}

		params := buildSDKOpenAIParamsWithTools(messages, systemStr, c.Model, constants.APIRequestMaxTokens, tools)

		comp, err := c.openaiSDKClient.Chat.Completions.New(ctx, params)
		elapsed := time.Since(start)
		resp := sdkCompletionToResponseIfNoError(comp, err)
		c.recordOutboundAudit(provider, destHost, c.Model, len(messages), systemPromptLen, toolCount, resp, err, elapsed, dataCategories)
		if err != nil {
			return nil, apperrors.Wrap(err, "OPENAI_API_ERROR", "OpenAI API error",
				apperrors.WithCategory(apperrors.CategoryNetwork))
		}

		return sdkCompletionToResponse(comp), nil
	}

	return c.CreateMessageWithSystem(ctx, messages, system)
}

func (c *Client) ensureSDKClient() {
	if c.APIKey != "" && c.sdkClient.Options == nil {
		c.sdkClient = newAnthropicSDKClient(c.APIKey, c.BaseURL, c.ProviderHeaders)
	}
}

func (c *Client) ensureOpenAISDKClient() {
	if c.APIKey != "" && c.openaiSDKClient.Options == nil {
		c.openaiSDKClient = newOpenAISDKClient(c.APIKey, c.BaseURL, c.ProviderHeaders)
	}
}

func (c *Client) providerName() string {
	if c.IsGoogle {
		return "google"
	}
	if c.IsOpenAI {
		return "openai"
	}
	return "anthropic"
}

func (c *Client) destinationHost() string {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if u, err := url.Parse(baseURL); err == nil {
		return u.Host
	}
	return baseURL
}

func (c *Client) systemPromptLength(system any) int {
	if sb, ok := system.([]SystemBlock); ok {
		total := 0
		for _, b := range sb {
			total += len(b.Text)
		}
		return total
	}
	if s, ok := system.(string); ok {
		return len(s)
	}
	return 0
}

func (c *Client) dataCategories(messages []Message, system any, toolCount int) []string {
	cats := []string{"conversation"}
	if system != nil {
		cats = append(cats, "system_prompt")
	}
	if toolCount > 0 {
		cats = append(cats, "tools")
	}
	return cats
}

func (c *Client) recordOutboundAudit(provider, destHost, model string, msgCount, systemPromptLen, toolCount int, resp *MessageResponse, err error, elapsed time.Duration, dataCategories []string) {
	entry := &observability.OutboundAuditEntry{
		Provider:        provider,
		DestinationHost: destHost,
		Model:           model,
		MessageCount:    msgCount,
		SystemPromptLen: systemPromptLen,
		ToolCount:       toolCount,
		Duration:        elapsed.Milliseconds(),
		DataCategories:  dataCategories,
	}
	if resp != nil {
		entry.InputTokens = resp.Usage.InputTokens
		entry.OutputTokens = resp.Usage.OutputTokens
	}
	observability.RecordOutboundAudit(entry)
}

func sdkCompletionToResponseIfNoError(comp *openai.ChatCompletion, err error) *MessageResponse {
	if err != nil || comp == nil {
		return nil
	}
	return sdkCompletionToResponse(comp)
}

func sdkMessageToResponseIfNoError(msg *anthropic.Message, err error) *MessageResponse {
	if err != nil || msg == nil {
		return nil
	}
	return sdkMessageToResponse(msg)
}

func (c *Client) StreamMessage(ctx context.Context, messages []Message, system string, onEvent func(event string, data []byte)) error {
	req := &MessageRequest{
		Model:     c.Model,
		MaxTokens: 4096,
		Messages:  messages,
		System:    system,
		Stream:    true,
	}

	if req.System != nil {
		if sysStr, ok := req.System.(string); ok && sysStr != "" {
			req.System = []SystemBlock{
				{
					Type: "text",
					Text: sysStr,
					CacheControl: &CacheControl{
						Type: "ephemeral",
					},
				},
			}
		}
	}

	return c.StreamMessageSSE(ctx, req, func(event string, data []byte) error {
		if onEvent != nil {
			onEvent(event, data)
		}
		return nil
	})
}
