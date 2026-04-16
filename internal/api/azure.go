package api

import (
	"context"
	"fmt"
)

func NewAzureClient(apiKey, resource, deployment string) *Client {
	endpoint := fmt.Sprintf("https://%s.openai.azure.com", resource)
	return &Client{
		APIKey:          apiKey,
		BaseURL:         endpoint,
		Model:           deployment,
		IsOpenAI:        true,
		ProviderHeaders: map[string]string{},
		HTTPClient:      defaultHTTPClient("azure"),
		openaiSDKClient: newAzureSDKClient(apiKey, endpoint, deployment),
	}
}

func (c *Client) CreateMessageAzure(messages []Message, system string) (*MessageResponse, error) {
	c.ensureOpenAISDKClient()

	params := buildSDKOpenAIParams(messages, system, c.Model, 4096)

	comp, err := c.openaiSDKClient.Chat.Completions.New(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("Azure API error: %w", err)
	}

	return sdkCompletionToResponse(comp), nil
}

func (c *Client) StreamMessageAzure(ctx context.Context, messages []Message, system string, handler func(event string, data []byte) error) error {
	c.ensureOpenAISDKClient()

	params := buildSDKOpenAIParams(messages, system, c.Model, 4096)

	return streamWithOpenAISDK(ctx, c.openaiSDKClient, params, handler)
}
