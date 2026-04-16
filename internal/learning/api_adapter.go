package learning

import (
	"context"
	"fmt"

	"github.com/instructkr/smartclaw/internal/api"
)

type APIClientAdapter struct {
	client *api.Client
	model  string
}

func NewAPIClientAdapter(client *api.Client, model string) *APIClientAdapter {
	if model == "" {
		model = "claude-3-5-haiku-20241022"
	}
	return &APIClientAdapter{client: client, model: model}
}

func (a *APIClientAdapter) CreateMessage(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("learning: API client not configured")
	}

	originalModel := a.client.Model
	a.client.SetModel(a.model)
	defer a.client.SetModel(originalModel)

	messages := []api.MessageParam{
		{Role: "user", Content: userPrompt},
	}

	resp, err := a.client.CreateMessageWithSystem(ctx, messages, systemPrompt)
	if err != nil {
		return "", fmt.Errorf("learning: API call: %w", err)
	}

	var result string
	for _, block := range resp.Content {
		if block.Type == "text" {
			result += block.Text
		}
	}

	return result, nil
}
