package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

type OpenAIChoice struct {
	Index        int          `json:"index"`
	Message      OpenAIMsg    `json:"message"`
	FinishReason string       `json:"finish_reason"`
	Delta        *OpenAIDelta `json:"delta,omitempty"`
}

type OpenAIMsg struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type OpenAIDelta struct {
	Role             string `json:"role,omitempty"`
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (c *Client) CreateMessageOpenAI(messages []Message, system string) (*MessageResponse, error) {
	openaiMsgs := make([]OpenAIMessage, 0, len(messages)+1)

	if system != "" {
		openaiMsgs = append(openaiMsgs, OpenAIMessage{
			Role:    "system",
			Content: system,
		})
	}

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

		openaiMsgs = append(openaiMsgs, OpenAIMessage{
			Role:    msg.Role,
			Content: content,
		})
	}

	req := OpenAIRequest{
		Model:     c.Model,
		Messages:  openaiMsgs,
		MaxTokens: 4096,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpointURL := c.buildEndpointURL("/v1/chat/completions")
	httpReq, err := http.NewRequest("POST", endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := openaiResp.Choices[0].Message.Content
	if content == "" && openaiResp.Choices[0].Message.ReasoningContent != "" {
		content = openaiResp.Choices[0].Message.ReasoningContent
	}

	result := &MessageResponse{
		ID:    openaiResp.ID,
		Type:  "message",
		Role:  "assistant",
		Model: openaiResp.Model,
		Content: []ContentBlock{
			{
				Type: "text",
				Text: content,
			},
		},
		Usage: Usage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
		StopReason: openaiResp.Choices[0].FinishReason,
	}

	return result, nil
}

func (c *Client) StreamMessageOpenAI(ctx context.Context, req *MessageRequest, handler func(event string, data []byte) error) error {
	openaiMsgs := make([]OpenAIMessage, 0, len(req.Messages)+1)

	if req.System != nil {
		systemStr := ""
		switch v := req.System.(type) {
		case string:
			systemStr = v
		}
		if systemStr != "" {
			openaiMsgs = append(openaiMsgs, OpenAIMessage{
				Role:    "system",
				Content: systemStr,
			})
		}
	}

	for _, msg := range req.Messages {
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

		openaiMsgs = append(openaiMsgs, OpenAIMessage{
			Role:    msg.Role,
			Content: content,
		})
	}

	openaiReq := OpenAIRequest{
		Model:     req.Model,
		Messages:  openaiMsgs,
		MaxTokens: req.MaxTokens,
		Stream:    true,
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.buildEndpointURL("/v1/chat/completions"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	contentBlockStarted := false
	messageStopSent := false
	defer func() {
		if !messageStopSent {
			msgBytes, _ := json.Marshal(&MessageResponse{StopReason: "end_turn"})
			_ = handler("message_stop", msgBytes)
		}
	}()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			break
		}

		var streamResp struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Choices) > 0 {
			delta := streamResp.Choices[0].Delta

			if !contentBlockStarted {
				startData := map[string]interface{}{
					"type":  "content_block_start",
					"index": 0,
					"content_block": map[string]interface{}{
						"type": "text",
						"text": "",
					},
				}
				startBytes, _ := json.Marshal(startData)
				if err := handler("content_block_start", startBytes); err != nil {
					return err
				}
				contentBlockStarted = true
			}

			if delta.ReasoningContent != "" {
				thinkingData := map[string]interface{}{
					"type":  "content_block_delta",
					"index": 0,
					"delta": map[string]interface{}{
						"type":     "thinking_delta",
						"thinking": delta.ReasoningContent,
					},
				}
				thinkingBytes, _ := json.Marshal(thinkingData)
				if err := handler("content_block_delta", thinkingBytes); err != nil {
					return err
				}
			}

			if delta.Content != "" {
				eventData := map[string]interface{}{
					"type":  "content_block_delta",
					"index": 0,
					"delta": map[string]interface{}{
						"type": "text_delta",
						"text": delta.Content,
					},
				}
				msgBytes, _ := json.Marshal(eventData)
				if err := handler("content_block_delta", msgBytes); err != nil {
					return err
				}
			}

			if streamResp.Choices[0].FinishReason != "" {
				messageStopSent = true
				msgResp := &MessageResponse{
					StopReason: streamResp.Choices[0].FinishReason,
				}
				msgBytes, _ := json.Marshal(msgResp)
				if err := handler("message_stop", msgBytes); err != nil {
					return err
				}
			}
		}
	}

	return scanner.Err()
}
