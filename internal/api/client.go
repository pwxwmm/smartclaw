package api

import (
	"context"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/instructkr/smartclaw/internal/constants"
	apperrors "github.com/instructkr/smartclaw/internal/errors"
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
	if c.IsGoogle {
		var systemStr string
		if sb, ok := system.([]SystemBlock); ok && len(sb) > 0 {
			systemStr = sb[0].Text
		} else if s, ok := system.(string); ok {
			systemStr = s
		}
		return c.CreateMessageGoogle(messages, systemStr)
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
		if err != nil {
			return nil, apperrors.Wrap(err, "OPENAI_API_ERROR", "OpenAI API error",
				apperrors.WithCategory(apperrors.CategoryNetwork))
		}

		return sdkCompletionToResponse(comp), nil
	}

	c.ensureSDKClient()

	params := buildSDKMessages(messages, system, c.Model, constants.APIRequestMaxTokens, c.Thinking, nil)

	msg, err := c.sdkClient.Messages.New(ctx, params)
	if err != nil {
		return nil, apperrors.Wrap(err, "ANTHROPIC_API_ERROR", "Anthropic API error",
			apperrors.WithCategory(apperrors.CategoryNetwork))
	}

	return sdkMessageToResponse(msg), nil
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
