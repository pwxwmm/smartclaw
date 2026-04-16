package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-api-key")
	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got '%s'", client.APIKey)
	}

	if client.BaseURL != DefaultBaseURL {
		t.Errorf("Expected base URL '%s', got '%s'", DefaultBaseURL, client.BaseURL)
	}

	if client.HTTPClient == nil {
		t.Error("Expected non-nil HTTP client")
	}
}

func TestClientCreateMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if r.URL.Path != "/v1/messages" {
			t.Errorf("Expected /v1/messages path, got %s", r.URL.Path)
		}

		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("Expected x-api-key header 'test-key', got '%s'", r.Header.Get("x-api-key"))
		}

		resp := MessageResponse{
			ID:    "msg_test",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-sonnet-4-5",
			Content: []ContentBlock{
				{Type: "text", Text: "Hello!"},
			},
			StopReason: "end_turn",
			Usage: Usage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	resp, err := client.CreateMessage(context.Background(), messages, "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.ID != "msg_test" {
		t.Errorf("Expected ID 'msg_test', got '%s'", resp.ID)
	}

	if len(resp.Content) != 1 {
		t.Errorf("Expected 1 content block, got %d", len(resp.Content))
	}

	if resp.Content[0].Text != "Hello!" {
		t.Errorf("Expected text 'Hello!', got '%s'", resp.Content[0].Text)
	}
}

func TestClientCreateMessageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL

	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	_, err := client.CreateMessage(context.Background(), messages, "")
	if err == nil {
		t.Error("Expected error for 500 response, got nil")
	}
}

func TestMessageRequest(t *testing.T) {
	req := MessageRequest{
		Model:     "claude-sonnet-4-5",
		MaxTokens: 4096,
		Messages: []Message{
			{Role: "user", Content: "Hello"},
		},
		System: "You are helpful",
		Stream: false,
	}

	if req.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected model 'claude-sonnet-4-5', got '%s'", req.Model)
	}

	if req.MaxTokens != 4096 {
		t.Errorf("Expected max tokens 4096, got %d", req.MaxTokens)
	}

	if len(req.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(req.Messages))
	}
}

func TestMessageResponse(t *testing.T) {
	resp := MessageResponse{
		ID:    "msg_123",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-sonnet-4-5",
		Content: []ContentBlock{
			{Type: "text", Text: "Response text"},
		},
		StopReason: "end_turn",
		Usage: Usage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	if resp.ID != "msg_123" {
		t.Errorf("Expected ID 'msg_123', got '%s'", resp.ID)
	}

	if resp.StopReason != "end_turn" {
		t.Errorf("Expected stop reason 'end_turn', got '%s'", resp.StopReason)
	}

	if resp.Usage.InputTokens != 100 {
		t.Errorf("Expected input tokens 100, got %d", resp.Usage.InputTokens)
	}
}

func TestContentBlock(t *testing.T) {
	textBlock := ContentBlock{
		Type: "text",
		Text: "Hello world",
	}

	if textBlock.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", textBlock.Type)
	}

	toolUseBlock := ContentBlock{
		Type:  "tool_use",
		ID:    "tool_123",
		Name:  "read_file",
		Input: map[string]any{"path": "/test"},
	}

	if toolUseBlock.Type != "tool_use" {
		t.Errorf("Expected type 'tool_use', got '%s'", toolUseBlock.Type)
	}

	if toolUseBlock.Name != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", toolUseBlock.Name)
	}
}

func TestUsage(t *testing.T) {
	usage := Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	}

	if usage.InputTokens != 1000 {
		t.Errorf("Expected input tokens 1000, got %d", usage.InputTokens)
	}

	if usage.OutputTokens != 500 {
		t.Errorf("Expected output tokens 500, got %d", usage.OutputTokens)
	}
}

func TestClientSetModel(t *testing.T) {
	client := NewClient("test-key")
	client.Model = "claude-opus-4-6"

	if client.Model != "claude-opus-4-6" {
		t.Errorf("Expected model 'claude-opus-4-6', got '%s'", client.Model)
	}
}

func TestClientWithCustomHTTPClient(t *testing.T) {
	client := NewClient("test-key")

	if client.HTTPClient.Timeout <= 0 {
		t.Error("Expected positive timeout for HTTP client")
	}
}
