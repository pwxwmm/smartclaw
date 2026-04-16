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

const googleBaseURL = "https://generativelanguage.googleapis.com/v1beta"

const maxAPIResponseSize = 1 * 1024 * 1024 // 1MB

type googleRequest struct {
	Contents          []googleContent  `json:"contents"`
	SystemInstruction *googleTextPart  `json:"systemInstruction,omitempty"`
	GenerationConfig  *googleGenConfig `json:"generationConfig,omitempty"`
}

type googleContent struct {
	Role  string       `json:"role"`
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text string `json:"text,omitempty"`
}

type googleTextPart struct {
	Parts []googlePart `json:"parts"`
}

type googleGenConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
}

type googleResponse struct {
	Candidates    []googleCandidate `json:"candidates"`
	UsageMetadata *googleUsage      `json:"usageMetadata,omitempty"`
}

type googleCandidate struct {
	Content googleContent `json:"content"`
}

type googleUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
}

func NewGoogleClient(apiKey, model string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    googleBaseURL,
		Model:      model,
		IsOpenAI:   false,
		IsGoogle:   true,
		HTTPClient: defaultHTTPClient("google"),
	}
}

func (c *Client) CreateMessageGoogle(messages []Message, system string) (*MessageResponse, error) {
	req := c.buildGoogleRequest(messages, system, 4096)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent", c.BaseURL, c.Model)
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	var googleResp googleResponse
	if err := json.Unmarshal(respBody, &googleResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(googleResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	var text string
	for _, part := range googleResp.Candidates[0].Content.Parts {
		text += part.Text
	}

	usage := Usage{}
	if googleResp.UsageMetadata != nil {
		usage.InputTokens = googleResp.UsageMetadata.PromptTokenCount
		usage.OutputTokens = googleResp.UsageMetadata.CandidatesTokenCount
	}

	return &MessageResponse{
		ID:    "google-response",
		Type:  "message",
		Role:  "assistant",
		Model: c.Model,
		Content: []ContentBlock{
			{Type: "text", Text: text},
		},
		Usage:      usage,
		StopReason: "end_turn",
	}, nil
}

func (c *Client) StreamMessageGoogle(ctx context.Context, messages []Message, system string, handler func(event string, data []byte) error) error {
	req := c.buildGoogleRequest(messages, system, 4096)

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse", c.BaseURL, c.Model)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxAPIResponseSize))
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
		if strings.TrimSpace(data) == "" {
			continue
		}

		var googleResp googleResponse
		if err := json.Unmarshal([]byte(data), &googleResp); err != nil {
			continue
		}

		if len(googleResp.Candidates) == 0 {
			continue
		}

		var text string
		for _, part := range googleResp.Candidates[0].Content.Parts {
			text += part.Text
		}

		if text == "" {
			continue
		}

		if !contentBlockStarted {
			startData := map[string]any{
				"type":  "content_block_start",
				"index": 0,
				"content_block": map[string]any{
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

		eventData := map[string]any{
			"type":  "content_block_delta",
			"index": 0,
			"delta": map[string]any{
				"type": "text_delta",
				"text": text,
			},
		}
		msgBytes, _ := json.Marshal(eventData)
		if err := handler("content_block_delta", msgBytes); err != nil {
			return err
		}
	}

	messageStopSent = true
	msgBytes, _ := json.Marshal(&MessageResponse{StopReason: "end_turn"})
	if err := handler("message_stop", msgBytes); err != nil {
		return err
	}

	return scanner.Err()
}

func (c *Client) buildGoogleRequest(messages []Message, system string, maxTokens int) googleRequest {
	contents := make([]googleContent, 0, len(messages))

	for _, msg := range messages {
		var text string
		switch v := msg.Content.(type) {
		case string:
			text = v
		case []ContentBlock:
			for _, block := range v {
				if block.Type == "text" {
					text = block.Text
					break
				}
			}
		}

		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}

		contents = append(contents, googleContent{
			Role:  role,
			Parts: []googlePart{{Text: text}},
		})
	}

	req := googleRequest{
		Contents: contents,
		GenerationConfig: &googleGenConfig{
			MaxOutputTokens: maxTokens,
		},
	}

	if system != "" {
		req.SystemInstruction = &googleTextPart{
			Parts: []googlePart{{Text: system}},
		}
	}

	return req
}
