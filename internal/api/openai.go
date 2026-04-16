package api

import (
	"context"
	"fmt"
)

func (c *Client) CreateMessageOpenAI(messages []Message, system string) (*MessageResponse, error) {
	c.ensureOpenAISDKClient()

	params := buildSDKOpenAIParams(messages, system, c.Model, 4096)

	comp, err := c.openaiSDKClient.Chat.Completions.New(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	return sdkCompletionToResponse(comp), nil
}

func (c *Client) StreamMessageOpenAI(ctx context.Context, req *MessageRequest, handler func(event string, data []byte) error) error {
	c.ensureOpenAISDKClient()

	var systemStr string
	if req.System != nil {
		if s, ok := req.System.(string); ok {
			systemStr = s
		}
	}

	params := buildSDKOpenAIParams(req.Messages, systemStr, req.Model, req.MaxTokens)

	return streamWithOpenAISDK(ctx, c.openaiSDKClient, params, handler)
}
