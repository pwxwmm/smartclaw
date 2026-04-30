package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/azure"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/respjson"
)

func newOpenAISDKClient(apiKey, baseURL string, providerHeaders map[string]string) openai.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithHeader("User-Agent", "SmartClaw/1.0"),
		option.WithMaxRetries(2),
	}

	if baseURL != "" && baseURL != "https://api.openai.com/v1" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	for k, v := range providerHeaders {
		opts = append(opts, option.WithHeader(k, v))
	}

	return openai.NewClient(opts...)
}

func newAzureSDKClient(apiKey, endpoint, deployment string) openai.Client {
	opts := []option.RequestOption{
		azure.WithEndpoint(endpoint, "2024-06-01"),
		azure.WithAPIKey(apiKey),
		option.WithHeader("User-Agent", "SmartClaw/1.0"),
		option.WithMaxRetries(2),
	}

	return openai.NewClient(opts...)
}

// extractStringFromRaw extracts a string value from a respjson.Field's raw JSON.
func extractStringFromRaw(field respjson.Field) string {
	raw := field.Raw()
	if raw == "" || raw == "null" {
		return ""
	}
	// If it's a JSON string, unmarshal it properly
	if strings.HasPrefix(raw, `"`) {
		var s string
		if err := json.Unmarshal([]byte(raw), &s); err == nil {
			return s
		}
		// Fallback: strip quotes
		return strings.Trim(raw, `"`)
	}
	return raw
}

func sdkCompletionToResponse(comp *openai.ChatCompletion) *MessageResponse {
	if comp == nil || len(comp.Choices) == 0 {
		return nil
	}

	choice := comp.Choices[0]
	content := choice.Message.Content

	result := &MessageResponse{
		ID:    comp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: comp.Model,
		Usage: Usage{
			InputTokens:  int(comp.Usage.PromptTokens),
			OutputTokens: int(comp.Usage.CompletionTokens),
		},
		StopReason: choice.FinishReason,
	}

	// Handle reasoning_content from ExtraFields
	if raw, ok := choice.Message.JSON.ExtraFields["reasoning_content"]; ok {
		reasoningContent := extractStringFromRaw(raw)
		if reasoningContent != "" {
			result.Content = append(result.Content, ContentBlock{
				Type:     "thinking",
				Thinking: reasoningContent,
			})
		}
	}

	result.Content = append(result.Content, ContentBlock{
		Type: "text",
		Text: content,
	})

	return result
}

func buildSDKOpenAIMessages(messages []MessageParam, system any) []openai.ChatCompletionMessageParamUnion {
	sdkMsgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages)+1)

	// Convert system prompt
	switch s := system.(type) {
	case string:
		if s != "" {
			sdkMsgs = append(sdkMsgs, openai.SystemMessage(s))
		}
	case []SystemBlock:
		if len(s) > 0 {
			sdkMsgs = append(sdkMsgs, openai.SystemMessage(s[0].Text))
		}
	}

	// Convert messages
	for _, msg := range messages {
		content := ""
		switch v := msg.Content.(type) {
		case string:
			content = v
		case []ContentBlock:
			for _, block := range v {
				if block.Type == "text" {
					content = block.Text
					break
				}
			}
		}

		switch msg.Role {
		case "user":
			sdkMsgs = append(sdkMsgs, openai.UserMessage(content))
		case "assistant":
			sdkMsgs = append(sdkMsgs, openai.AssistantMessage(content))
		}
	}

	return sdkMsgs
}

func buildSDKOpenAIParams(messages []MessageParam, system any, model string, maxTokens int) openai.ChatCompletionNewParams {
	return openai.ChatCompletionNewParams{
		Model:     openai.ChatModel(model),
		MaxTokens: openai.Int(int64(maxTokens)),
		Messages:  buildSDKOpenAIMessages(messages, system),
	}
}

func streamWithOpenAISDK(ctx context.Context, sdkClient openai.Client, params openai.ChatCompletionNewParams, handler func(event string, data []byte) error) error {
	stream := sdkClient.Chat.Completions.NewStreaming(ctx, params)

	contentBlockStarted := false
	messageStopSent := false
	defer func() {
		if !messageStopSent {
			msgBytes, _ := json.Marshal(&MessageResponse{StopReason: "end_turn"})
			if stopErr := handler("message_stop", msgBytes); stopErr != nil {
				slog.Debug("failed to send message_stop event", "error", stopErr)
			}
		}
	}()

	for stream.Next() {
		chunk := stream.Current()

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		// Check for reasoning content in delta
		reasoningDelta := ""
		if raw, ok := delta.JSON.ExtraFields["reasoning_content"]; ok {
			reasoningDelta = extractStringFromRaw(raw)
		}

		hasContent := delta.Content != "" || reasoningDelta != ""

		if !contentBlockStarted && hasContent {
			if err := emitEvent(handler, "content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
					"type": "text",
					"text": "",
				},
			}); err != nil {
				return err
			}
			contentBlockStarted = true
		}

		// Handle reasoning_content delta
		if reasoningDelta != "" {
			if err := emitEvent(handler, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type":     "thinking_delta",
					"thinking": reasoningDelta,
				},
			}); err != nil {
				return err
			}
		}

		if delta.Content != "" {
			if err := emitEvent(handler, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type": "text_delta",
					"text": delta.Content,
				},
			}); err != nil {
				return err
			}
		}

		if choice.FinishReason != "" {
			messageStopSent = true
			msgResp := &MessageResponse{
				StopReason: choice.FinishReason,
			}
			if err := emitEvent(handler, "message_stop", msgResp); err != nil {
				return err
			}
		}
	}

	if err := stream.Err(); err != nil {
		errData := map[string]any{
			"type": "error",
			"error": map[string]any{
				"type":    "api_error",
				"message": err.Error(),
			},
		}
	if emitErr := emitEvent(handler, "error", errData); emitErr != nil {
		slog.Debug("failed to emit error event", "error", emitErr)
	}
	return fmt.Errorf("OpenAI API streaming error: %w", err)
	}

	return nil
}
