package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/instructkr/smartclaw/internal/api"
)

func TestLocalClientImplementsInterface(t *testing.T) {
	var _ APIClient = (*LocalClient)(nil)
}

func TestRemoteClientImplementsInterface(t *testing.T) {
	var _ APIClient = (*RemoteClient)(nil)
}

func TestNewLocalClient(t *testing.T) {
	client := api.NewClientWithModel("test-key", "", "claude-sonnet-4-5")
	lc := NewLocalClient(client)

	if lc.client != client {
		t.Error("LocalClient.client not set correctly")
	}
	if lc.GetModel() != "claude-sonnet-4-5" {
		t.Errorf("GetModel() = %q, want %q", lc.GetModel(), "claude-sonnet-4-5")
	}
	if lc.IsOpenAI() {
		t.Error("IsOpenAI() = true, want false")
	}
}

func TestLocalClientSetOpenAI(t *testing.T) {
	client := api.NewClient("test-key")
	lc := NewLocalClient(client)

	lc.SetOpenAI(true)
	if !lc.IsOpenAI() {
		t.Error("IsOpenAI() = false after SetOpenAI(true)")
	}
	if !client.IsOpenAI {
		t.Error("underlying client.IsOpenAI not set")
	}

	lc.SetOpenAI(false)
	if lc.IsOpenAI() {
		t.Error("IsOpenAI() = true after SetOpenAI(false)")
	}
}

func TestLocalClientSetModel(t *testing.T) {
	client := api.NewClient("test-key")
	lc := NewLocalClient(client)

	lc.SetModel("gpt-4o")
	if lc.GetModel() != "gpt-4o" {
		t.Errorf("GetModel() = %q, want %q", lc.GetModel(), "gpt-4o")
	}
	if client.Model != "gpt-4o" {
		t.Errorf("client.Model = %q, want %q", client.Model, "gpt-4o")
	}
}

func TestLocalClientClose(t *testing.T) {
	lc := NewLocalClient(api.NewClient("test-key"))
	if err := lc.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

func TestNewRemoteClient(t *testing.T) {
	rc := NewRemoteClient("http://localhost:8080", "mytoken")

	if rc.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL = %q, want %q", rc.baseURL, "http://localhost:8080")
	}
	if rc.token != "mytoken" {
		t.Errorf("token = %q, want %q", rc.token, "mytoken")
	}
	if rc.wsURL != "ws://localhost:8080" {
		t.Errorf("wsURL = %q, want %q", rc.wsURL, "ws://localhost:8080")
	}
}

func TestNewRemoteClientHTTPS(t *testing.T) {
	rc := NewRemoteClient("https://remote.example.com", "token")
	if rc.wsURL != "wss://remote.example.com" {
		t.Errorf("wsURL = %q, want %q", rc.wsURL, "wss://remote.example.com")
	}
}

func TestRemoteClientSetModel(t *testing.T) {
	rc := NewRemoteClient("http://localhost:8080", "")

	rc.SetModel("claude-opus-4-6")
	if rc.GetModel() != "claude-opus-4-6" {
		t.Errorf("GetModel() = %q, want %q", rc.GetModel(), "claude-opus-4-6")
	}
}

func TestRemoteClientIsOpenAI(t *testing.T) {
	rc := NewRemoteClient("http://localhost:8080", "")
	if rc.IsOpenAI() {
		t.Error("IsOpenAI() = true, want false by default")
	}
	rc.SetOpenAI(true)
	if !rc.IsOpenAI() {
		t.Error("IsOpenAI() = false after SetOpenAI(true)")
	}
}

func TestRemoteClientClose(t *testing.T) {
	rc := NewRemoteClient("http://localhost:8080", "")
	if err := rc.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

func TestRemoteClientSendMessage(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("path = %q, want /api/chat", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-token")
		}

		var req api.MessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("model = %q, want %q", req.Model, "test-model")
		}

		resp := api.MessageResponse{
			ID:   "msg-123",
			Type: "message",
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: "text", Text: "Hello from server"},
			},
			Model:      "test-model",
			StopReason: "end_turn",
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	rc := NewRemoteClient(server.URL, "test-token")
	rc.SetModel("test-model")

	req := &api.MessageRequest{
		Model:     "test-model",
		MaxTokens: 1024,
		Messages: []api.MessageParam{
			{Role: "user", Content: "Hi"},
		},
	}

	resp, err := rc.SendMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("SendMessage() error: %v", err)
	}
	if resp.ID != "msg-123" {
		t.Errorf("resp.ID = %q, want %q", resp.ID, "msg-123")
	}
	if len(resp.Content) == 0 || resp.Content[0].Text != "Hello from server" {
		t.Errorf("resp.Content[0].Text = %q, want %q", resp.Content[0].Text, "Hello from server")
	}
}

func TestRemoteClientSendMessageError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	rc := NewRemoteClient(server.URL, "token")
	req := &api.MessageRequest{Model: "test", MaxTokens: 100}

	_, err := rc.SendMessage(context.Background(), req)
	if err == nil {
		t.Fatal("SendMessage() = nil, want error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want to contain 500", err.Error())
	}
}

func TestInitialModelWithLocalClient(t *testing.T) {
	client := api.NewClientWithModel("test-key", "", "gpt-4o")
	client.SetOpenAI(true)

	m := InitialModelWithLocalClient(client)
	if m.model != "gpt-4o" {
		t.Errorf("model = %q, want %q", m.model, "gpt-4o")
	}
	if m.apiClient == nil {
		t.Fatal("apiClient is nil")
	}
	lc, ok := m.apiClient.(*LocalClient)
	if !ok {
		t.Fatal("apiClient is not *LocalClient")
	}
	if !lc.IsOpenAI() {
		t.Error("IsOpenAI() = false, want true")
	}
}

func TestInitialModelWithClientNil(t *testing.T) {
	m := InitialModelWithClient(nil)
	if m.apiClient != nil {
		t.Error("apiClient should be nil")
	}
}
