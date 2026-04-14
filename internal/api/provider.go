package api

import (
	"context"
)

type ProviderClient interface {
	CreateMessage(ctx context.Context, messages []MessageParam, system any) (*MessageResponse, error)
	StreamMessageCtx(ctx context.Context, req *MessageRequest, handler func(event string, data []byte) error) error
	SetModel(model string)
	GetModel() string
}

func (c *Client) CreateMessageCtx(ctx context.Context, messages []MessageParam, system any) (*MessageResponse, error) {
	return c.CreateMessageWithSystem(ctx, messages, system)
}

func (c *Client) GetModel() string {
	return c.Model
}

func (c *Client) StreamMessageCtx(ctx context.Context, req *MessageRequest, handler func(event string, data []byte) error) error {
	if c.IsOpenAI {
		return c.StreamMessageOpenAI(ctx, req, handler)
	}
	return c.StreamMessageSSE(ctx, req, handler)
}
