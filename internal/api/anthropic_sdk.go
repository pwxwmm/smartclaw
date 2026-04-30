package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

func newAnthropicSDKClient(apiKey, baseURL string, providerHeaders map[string]string) anthropic.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithHeader("User-Agent", "SmartClaw/1.0"),
		option.WithHeader("anthropic-beta", "prompt-caching-2024-07-31"),
		option.WithMaxRetries(2),
	}

	if baseURL != "" && baseURL != DefaultBaseURL {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	for k, v := range providerHeaders {
		opts = append(opts, option.WithHeader(k, v))
	}

	return anthropic.NewClient(opts...)
}

func sdkMessageToResponse(msg *anthropic.Message) *MessageResponse {
	if msg == nil {
		return nil
	}

	resp := &MessageResponse{
		ID:         msg.ID,
		Type:       string(msg.Type),
		Role:       string(msg.Role),
		Model:      string(msg.Model),
		StopReason: string(msg.StopReason),
		Usage: Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
			CacheCreation: int(msg.Usage.CacheCreation.Ephemeral5mInputTokens +
				msg.Usage.CacheCreation.Ephemeral1hInputTokens),
			CacheRead: int(msg.Usage.CacheReadInputTokens),
		},
	}

	for _, block := range msg.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.TextBlock:
			resp.Content = append(resp.Content, ContentBlock{
				Type: "text",
				Text: variant.Text,
			})
		case anthropic.ThinkingBlock:
			resp.Content = append(resp.Content, ContentBlock{
				Type:     "thinking",
				Thinking: variant.Thinking,
			})
		case anthropic.RedactedThinkingBlock:
			resp.Content = append(resp.Content, ContentBlock{
				Type:     "redacted_thinking",
				Thinking: "...(redacted)...",
			})
		case anthropic.ToolUseBlock:
			var input map[string]any
			if err := json.Unmarshal(variant.Input, &input); err != nil {
				input = nil
			}
			resp.Content = append(resp.Content, ContentBlock{
				Type:  "tool_use",
				ID:    block.ID,
				Name:  block.Name,
				Input: input,
			})
		case anthropic.ServerToolUseBlock:
			var input map[string]any
			if raw, err := json.Marshal(variant.Input); err == nil {
				json.Unmarshal(raw, &input)
			}
			resp.Content = append(resp.Content, ContentBlock{
				Type:  "server_tool_use",
				ID:    block.ID,
				Name:  string(variant.Name),
				Input: input,
			})
		default:
			if block.Text != "" {
				resp.Content = append(resp.Content, ContentBlock{
					Type: block.Type,
					Text: block.Text,
				})
			}
		}
	}

	return resp
}

func buildSDKMessages(messages []MessageParam, system any, model string, maxTokens int, thinking *ThinkingConfig, tools []ToolDefinition) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: int64(maxTokens),
	}

	params.Messages = make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		sdkBlocks := convertContentToSDKBlocks(msg.Content)
		switch msg.Role {
		case "user":
			params.Messages = append(params.Messages, anthropic.NewUserMessage(sdkBlocks...))
		case "assistant":
			params.Messages = append(params.Messages, anthropic.NewAssistantMessage(sdkBlocks...))
		}
	}

	params.System = convertSystemToSDK(system)

	if thinking != nil && thinking.Type == "enabled" && thinking.BudgetTokens > 0 {
		params.Thinking = anthropic.ThinkingConfigParamOfEnabled(int64(thinking.BudgetTokens))
	}

	if len(tools) > 0 {
		params.Tools = make([]anthropic.ToolUnionParam, 0, len(tools))
		for _, td := range tools {
			toolParam := anthropic.ToolParam{
				Name:        td.Name,
				Description: anthropic.String(td.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: td.InputSchema,
				},
			}
			params.Tools = append(params.Tools, anthropic.ToolUnionParam{OfTool: &toolParam})
		}
	}

	return params
}

func convertContentToSDKBlocks(content any) []anthropic.ContentBlockParamUnion {
	switch c := content.(type) {
	case string:
		return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(c)}
	case []ContentBlock:
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(c))
		for _, block := range c {
			blocks = append(blocks, convertContentBlockToSDK(block)...)
		}
		return blocks
	case []any:
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(c))
		for _, item := range c {
			if m, ok := item.(map[string]any); ok {
				blocks = append(blocks, convertMapToSDKBlock(m)...)
			}
		}
		return blocks
	default:
		data, err := json.Marshal(content)
		if err != nil {
			return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(fmt.Sprintf("%v", content))}
		}
		var cb []ContentBlock
		if err := json.Unmarshal(data, &cb); err != nil {
			return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(string(data))}
		}
		return convertContentToSDKBlocks(cb)
	}
}

func convertContentBlockToSDK(block ContentBlock) []anthropic.ContentBlockParamUnion {
	switch block.Type {
	case "text":
		return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(block.Text)}
	case "thinking":
		return []anthropic.ContentBlockParamUnion{anthropic.NewThinkingBlock("", block.Thinking)}
	case "tool_use":
		return []anthropic.ContentBlockParamUnion{anthropic.NewToolUseBlock(block.ID, block.Input, block.Name)}
	case "tool_result":
		contentStr := ""
		if block.Content != nil {
			switch c := block.Content.(type) {
			case string:
				contentStr = c
			default:
				data, _ := json.Marshal(c)
				contentStr = string(data)
			}
		}
		return []anthropic.ContentBlockParamUnion{anthropic.NewToolResultBlock(block.ToolUseID, contentStr, block.IsError)}
	case "image":
		if block.Source != nil {
			if block.Source.URL != "" {
				return []anthropic.ContentBlockParamUnion{
					anthropic.NewImageBlock(anthropic.URLImageSourceParam{
						URL: block.Source.URL,
					}),
				}
			}
			if block.Source.Data != "" {
				return []anthropic.ContentBlockParamUnion{
					anthropic.NewImageBlockBase64(block.Source.MediaType, block.Source.Data),
				}
			}
		}
		return nil
	default:
		if block.Text != "" {
			return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(block.Text)}
		}
		return nil
	}
}

func convertMapToSDKBlock(m map[string]any) []anthropic.ContentBlockParamUnion {
	blockType, _ := m["type"].(string)
	switch blockType {
	case "text":
		text, _ := m["text"].(string)
		return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(text)}
	case "thinking":
		thinking, _ := m["thinking"].(string)
		return []anthropic.ContentBlockParamUnion{anthropic.NewThinkingBlock("", thinking)}
	case "tool_use":
		id, _ := m["id"].(string)
		name, _ := m["name"].(string)
		input := m["input"]
		return []anthropic.ContentBlockParamUnion{anthropic.NewToolUseBlock(id, input, name)}
	case "tool_result":
		toolUseID, _ := m["tool_use_id"].(string)
		isError, _ := m["is_error"].(bool)
		content := m["content"]
		contentStr := ""
		switch c := content.(type) {
		case string:
			contentStr = c
		default:
			data, _ := json.Marshal(c)
			contentStr = string(data)
		}
		return []anthropic.ContentBlockParamUnion{anthropic.NewToolResultBlock(toolUseID, contentStr, isError)}
	case "image":
		source, _ := m["source"].(map[string]any)
		if source != nil {
			sourceType, _ := source["type"].(string)
			if sourceType == "url" {
				url, _ := source["url"].(string)
				return []anthropic.ContentBlockParamUnion{
					anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: url}),
				}
			}
			mediaType, _ := source["media_type"].(string)
			data, _ := source["data"].(string)
			if data != "" {
				return []anthropic.ContentBlockParamUnion{
					anthropic.NewImageBlockBase64(mediaType, data),
				}
			}
		}
		return nil
	default:
		if text, ok := m["text"].(string); ok {
			return []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock(text)}
		}
		return nil
	}
}

func convertSystemToSDK(system any) []anthropic.TextBlockParam {
	if system == nil {
		return nil
	}

	switch s := system.(type) {
	case string:
		if s == "" {
			return nil
		}
		return []anthropic.TextBlockParam{
			{
				Text:         s,
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			},
		}
	case []SystemBlock:
		result := make([]anthropic.TextBlockParam, 0, len(s))
		for _, block := range s {
			tb := anthropic.TextBlockParam{
				Text: block.Text,
			}
			if block.CacheControl != nil && block.CacheControl.Type == "ephemeral" {
				tb.CacheControl = anthropic.NewCacheControlEphemeralParam()
				if block.CacheControl.TTL != "" {
					tb.CacheControl.TTL = anthropic.CacheControlEphemeralTTL(block.CacheControl.TTL)
				}
			}
			result = append(result, tb)
		}
		return result
	default:
		return nil
	}
}

func streamWithSDK(ctx context.Context, sdkClient anthropic.Client, params anthropic.MessageNewParams, handler func(event string, data []byte) error) error {
	stream := sdkClient.Messages.NewStreaming(ctx, params)

	for stream.Next() {
		event := stream.Current()

		switch variant := event.AsAny().(type) {
		case anthropic.MessageStartEvent:
			msg := variant.Message
			data := map[string]any{
				"type": "message_start",
				"message": map[string]any{
					"id":    msg.ID,
					"model": string(msg.Model),
					"role":  string(msg.Role),
					"usage": map[string]any{
						"input_tokens":                msg.Usage.InputTokens,
						"output_tokens":               msg.Usage.OutputTokens,
						"cache_creation_input_tokens": msg.Usage.CacheCreation.Ephemeral5mInputTokens + msg.Usage.CacheCreation.Ephemeral1hInputTokens,
						"cache_read_input_tokens":     msg.Usage.CacheReadInputTokens,
					},
				},
			}
			if err := emitEvent(handler, "message_start", data); err != nil {
				return err
			}

		case anthropic.ContentBlockStartEvent:
			cb := variant.ContentBlock
			contentBlock := map[string]any{
				"type": cb.Type,
			}
			switch cb.Type {
			case "text":
				contentBlock["text"] = cb.Text
			case "thinking":
				contentBlock["thinking"] = cb.Thinking
				contentBlock["signature"] = cb.Signature
			case "tool_use", "server_tool_use":
				contentBlock["id"] = cb.ID
				contentBlock["name"] = cb.Name
			}
			data := map[string]any{
				"type":          "content_block_start",
				"index":         variant.Index,
				"content_block": contentBlock,
			}
			if err := emitEvent(handler, "content_block_start", data); err != nil {
				return err
			}

		case anthropic.ContentBlockDeltaEvent:
			delta := variant.Delta
			deltaMap := map[string]any{
				"type": delta.Type,
			}
			switch delta.Type {
			case "text_delta":
				deltaMap["text"] = delta.Text
			case "thinking_delta":
				deltaMap["thinking"] = delta.Thinking
			case "input_json_delta":
				deltaMap["partial_json"] = delta.PartialJSON
			case "signature_delta":
				deltaMap["signature"] = delta.Signature
			}
			data := map[string]any{
				"type":  "content_block_delta",
				"index": variant.Index,
				"delta": deltaMap,
			}
			if err := emitEvent(handler, "content_block_delta", data); err != nil {
				return err
			}

		case anthropic.ContentBlockStopEvent:
			data := map[string]any{
				"type":  "content_block_stop",
				"index": variant.Index,
			}
			if err := emitEvent(handler, "content_block_stop", data); err != nil {
				return err
			}

		case anthropic.MessageDeltaEvent:
			data := map[string]any{
				"type": "message_delta",
				"delta": map[string]any{
					"stop_reason": string(variant.Delta.StopReason),
				},
				"usage": map[string]any{
					"output_tokens": variant.Usage.OutputTokens,
				},
			}
			if err := emitEvent(handler, "message_delta", data); err != nil {
				return err
			}

		case anthropic.MessageStopEvent:
			data := map[string]any{
				"type": "message_stop",
			}
			if err := emitEvent(handler, "message_stop", data); err != nil {
				return err
			}

		default:
			data := map[string]any{
				"type": "ping",
			}
			if err := emitEvent(handler, "ping", data); err != nil {
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
	return fmt.Errorf("Anthropic API error: %w", err)
	}

	return nil
}

func emitEvent(handler func(event string, data []byte) error, event string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}
	return handler(event, jsonData)
}
