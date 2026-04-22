package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/instructkr/smartclaw/internal/gateway"
)

func TestPlatformAdapterInterface(t *testing.T) {
	var _ PlatformAdapter = (*DiscordAdapter)(nil)
	var _ PlatformAdapter = (*WhatsAppAdapter)(nil)
	var _ PlatformAdapter = (*SignalAdapter)(nil)
	var _ PlatformAdapter = (*EmailAdapter)(nil)
	var _ PlatformAdapter = (*SlackAdapter)(nil)
}

func TestDiscordAdapterName(t *testing.T) {
	da := NewDiscordAdapter("test-token", nil)
	if got := da.Name(); got != "discord" {
		t.Errorf("DiscordAdapter.Name() = %q, want %q", got, "discord")
	}
}

func TestWhatsAppAdapterName(t *testing.T) {
	wa := NewWhatsAppAdapter("phone-id", "token", nil)
	if got := wa.Name(); got != "whatsapp" {
		t.Errorf("WhatsAppAdapter.Name() = %q, want %q", got, "whatsapp")
	}
}

func TestSignalAdapterName(t *testing.T) {
	sa := NewSignalAdapter("http://localhost:8080", "+1234567890", nil)
	if got := sa.Name(); got != "signal" {
		t.Errorf("SignalAdapter.Name() = %q, want %q", got, "signal")
	}
}

func TestEmailAdapterName(t *testing.T) {
	ea := NewEmailAdapter("smtp.example.com", "587", "user@example.com", "pass", "imap.example.com", "993", nil)
	if got := ea.Name(); got != "email" {
		t.Errorf("EmailAdapter.Name() = %q, want %q", got, "email")
	}
}

func TestDiscordTruncateHelper(t *testing.T) {
	text := strings.Repeat("x", 3000)
	if len(text) <= discordMaxMessageLen {
		t.Error("test setup: text should be longer than max")
	}
	truncated := text[:discordMaxMessageLen-len("[...truncated]")] + "[...truncated]"
	if len(truncated) != discordMaxMessageLen {
		t.Errorf("truncated length = %d, want %d", len(truncated), discordMaxMessageLen)
	}
}

func TestWhatsAppTruncateHelper(t *testing.T) {
	text := strings.Repeat("y", 5000)
	if len(text) <= whatsappMaxMessageLen {
		t.Error("test setup: text should be longer than max")
	}
	truncated := text[:whatsappMaxMessageLen-len("[...truncated]")] + "[...truncated]"
	if len(truncated) != whatsappMaxMessageLen {
		t.Errorf("truncated length = %d, want %d", len(truncated), whatsappMaxMessageLen)
	}
}

func TestSignalTruncateHelper(t *testing.T) {
	text := strings.Repeat("z", 5000)
	if len(text) <= signalMaxMessageLen {
		t.Error("test setup: text should be longer than max")
	}
	truncated := text[:signalMaxMessageLen-len("[...truncated]")] + "[...truncated]"
	if len(truncated) != signalMaxMessageLen {
		t.Errorf("truncated length = %d, want %d", len(truncated), signalMaxMessageLen)
	}
}

func TestDiscordAdapterCreation(t *testing.T) {
	da := NewDiscordAdapter("bot-token-123", nil)
	if da.token != "bot-token-123" {
		t.Errorf("token = %q, want %q", da.token, "bot-token-123")
	}
	if da.gateway != nil {
		t.Error("gateway should be nil")
	}
}

func TestWhatsAppAdapterCreation(t *testing.T) {
	wa := NewWhatsAppAdapter("phone-123", "access-token", nil)
	if wa.phoneNumberID != "phone-123" {
		t.Errorf("phoneNumberID = %q, want %q", wa.phoneNumberID, "phone-123")
	}
	if wa.accessToken != "access-token" {
		t.Errorf("accessToken = %q, want %q", wa.accessToken, "access-token")
	}
}

func TestSignalAdapterCreation(t *testing.T) {
	sa := NewSignalAdapter("http://signal-cli:8080", "+1234567890", nil)
	if sa.signalCLIURL != "http://signal-cli:8080" {
		t.Errorf("signalCLIURL = %q, want %q", sa.signalCLIURL, "http://signal-cli:8080")
	}
	if sa.phoneNumber != "+1234567890" {
		t.Errorf("phoneNumber = %q, want %q", sa.phoneNumber, "+1234567890")
	}
}

func TestEmailAdapterCreation(t *testing.T) {
	ea := NewEmailAdapter("smtp.test.com", "587", "user@test.com", "pass", "imap.test.com", "993", nil)
	if ea.smtpHost != "smtp.test.com" {
		t.Errorf("smtpHost = %q, want %q", ea.smtpHost, "smtp.test.com")
	}
	if ea.smtpPort != "587" {
		t.Errorf("smtpPort = %q, want %q", ea.smtpPort, "587")
	}
}

func TestDiscordAdapterStop(t *testing.T) {
	da := NewDiscordAdapter("token", nil)
	if err := da.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

func TestSignalAdapterStop(t *testing.T) {
	sa := NewSignalAdapter("http://localhost:8080", "+1234", nil)
	if err := sa.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

func TestEmailAdapterStop(t *testing.T) {
	ea := NewEmailAdapter("smtp.test.com", "587", "u@t.com", "p", "imap.test.com", "993", nil)
	if err := ea.Stop(); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}
}

func TestDiscordAdapterStartAlreadyRunning(t *testing.T) {
	da := NewDiscordAdapter("token", nil)
	da.mu.Lock()
	da.running = true
	da.mu.Unlock()

	err := da.Start(context.Background())
	if err == nil {
		t.Error("expected error when adapter already running")
	}

	da.mu.Lock()
	da.running = false
	da.mu.Unlock()
}

func TestSignalAdapterStartAlreadyRunning(t *testing.T) {
	sa := NewSignalAdapter("http://localhost:8080", "+1234", nil)
	sa.mu.Lock()
	sa.running = true
	sa.mu.Unlock()

	err := sa.Start(context.Background())
	if err == nil {
		t.Error("expected error when adapter already running")
	}

	sa.mu.Lock()
	sa.running = false
	sa.mu.Unlock()
}

func TestEmailAdapterStartAlreadyRunning(t *testing.T) {
	ea := NewEmailAdapter("smtp.test.com", "587", "u@t.com", "p", "", "", nil)
	ea.mu.Lock()
	ea.running = true
	ea.mu.Unlock()

	err := ea.Start(context.Background())
	if err == nil {
		t.Error("expected error when adapter already running")
	}

	ea.mu.Lock()
	ea.running = false
	ea.mu.Unlock()
}

func TestEmailAdapterSendInvalidAddress(t *testing.T) {
	ea := NewEmailAdapter("smtp.test.com", "587", "u@t.com", "p", "", "", nil)
	resp := &gateway.GatewayResponse{Content: "hello", SessionID: "s1"}
	err := ea.Send("not-an-email", resp)
	if err == nil {
		t.Error("expected error for invalid email address")
	}
}

func TestDiscordSendWithMockServer(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	da := NewDiscordAdapter("test-token", nil)
	da.apiURL = server.URL
	da.client = server.Client()

	resp := &gateway.GatewayResponse{Content: "Hello Discord!", SessionID: "s1"}
	err := da.Send("channel-123", resp)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if receivedBody["content"] != "Hello Discord!" {
		t.Errorf("content = %v, want %q", receivedBody["content"], "Hello Discord!")
	}
}

func TestDiscordSendTruncationWithMockServer(t *testing.T) {
	longText := strings.Repeat("a", 3000)
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	da := NewDiscordAdapter("test-token", nil)
	da.apiURL = server.URL
	da.client = server.Client()

	resp := &gateway.GatewayResponse{Content: longText, SessionID: "s1"}
	err := da.Send("channel-123", resp)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	sent, ok := receivedBody["content"].(string)
	if !ok {
		t.Fatal("content is not a string")
	}
	if len(sent) != discordMaxMessageLen {
		t.Errorf("sent message length = %d, want %d", len(sent), discordMaxMessageLen)
	}
	if !strings.HasSuffix(sent, "[...truncated]") {
		t.Error("truncated message should end with [...truncated]")
	}
}

func TestWhatsAppSendWithMockServer(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"messages":[{"id":"wamid123"}]}`))
	}))
	defer server.Close()

	wa := NewWhatsAppAdapter("phone-id", "test-token", nil)
	wa.apiURL = server.URL
	wa.client = server.Client()

	resp := &gateway.GatewayResponse{Content: "Hello WhatsApp!", SessionID: "s1"}
	err := wa.Send("+1234567890", resp)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if receivedBody["to"] != "+1234567890" {
		t.Errorf("to = %v, want %q", receivedBody["to"], "+1234567890")
	}
}

func TestWhatsAppSendTruncationWithMockServer(t *testing.T) {
	longText := strings.Repeat("b", 5000)
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"messages":[{"id":"wamid123"}]}`))
	}))
	defer server.Close()

	wa := NewWhatsAppAdapter("phone-id", "test-token", nil)
	wa.apiURL = server.URL
	wa.client = server.Client()

	resp := &gateway.GatewayResponse{Content: longText, SessionID: "s1"}
	err := wa.Send("+1234567890", resp)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	textMap, ok := receivedBody["text"].(map[string]any)
	if !ok {
		t.Fatal("text is not a map")
	}
	sent, ok := textMap["body"].(string)
	if !ok {
		t.Fatal("body is not a string")
	}
	if len(sent) != whatsappMaxMessageLen {
		t.Errorf("sent message length = %d, want %d", len(sent), whatsappMaxMessageLen)
	}
	if !strings.HasSuffix(sent, "[...truncated]") {
		t.Error("truncated message should end with [...truncated]")
	}
}

func TestSignalSendWithMockServer(t *testing.T) {
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sa := NewSignalAdapter(server.URL, "+1234567890", nil)
	sa.client = server.Client()

	resp := &gateway.GatewayResponse{Content: "Hello Signal!", SessionID: "s1"}
	err := sa.Send("+0987654321", resp)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if receivedBody["message"] != "Hello Signal!" {
		t.Errorf("message = %v, want %q", receivedBody["message"], "Hello Signal!")
	}
}

func TestSignalSendTruncationWithMockServer(t *testing.T) {
	longText := strings.Repeat("c", 5000)
	var receivedBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sa := NewSignalAdapter(server.URL, "+1234567890", nil)
	sa.client = server.Client()

	resp := &gateway.GatewayResponse{Content: longText, SessionID: "s1"}
	err := sa.Send("+0987654321", resp)
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}

	sent, ok := receivedBody["message"].(string)
	if !ok {
		t.Fatal("message is not a string")
	}
	if len(sent) != signalMaxMessageLen {
		t.Errorf("sent message length = %d, want %d", len(sent), signalMaxMessageLen)
	}
	if !strings.HasSuffix(sent, "[...truncated]") {
		t.Error("truncated message should end with [...truncated]")
	}
}

func TestExtractSender(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"John Doe <john@example.com>", "john@example.com"},
		{"john@example.com", "john@example.com"},
		{"<john@example.com>", "john@example.com"},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractSender(tt.input)
		if got != tt.want {
			t.Errorf("extractSender(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWhatsAppWithWebhook(t *testing.T) {
	wa := NewWhatsAppAdapter("phone-id", "token", nil)
	wa = wa.WithWebhook(":9090", "my-verify-token")
	if wa.webhookAddr != ":9090" {
		t.Errorf("webhookAddr = %q, want %q", wa.webhookAddr, ":9090")
	}
	if wa.verifyToken != "my-verify-token" {
		t.Errorf("verifyToken = %q, want %q", wa.verifyToken, "my-verify-token")
	}
}

func TestWhatsAppWebhookVerify(t *testing.T) {
	wa := NewWhatsAppAdapter("phone-id", "token", nil)
	wa.verifyToken = "test-verify"

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/whatsapp/verify", wa.handleVerify)

	req := httptest.NewRequest("GET", "/webhook/whatsapp/verify?hub.mode=subscribe&hub.verify_token=test-verify&hub.challenge=challenge123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("verify status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "challenge123" {
		t.Errorf("verify body = %q, want %q", w.Body.String(), "challenge123")
	}
}

func TestWhatsAppWebhookVerifyBadToken(t *testing.T) {
	wa := NewWhatsAppAdapter("phone-id", "token", nil)
	wa.verifyToken = "test-verify"

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/whatsapp/verify", wa.handleVerify)

	req := httptest.NewRequest("GET", "/webhook/whatsapp/verify?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=challenge123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("verify status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestDiscordRateLimitHandling(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"retry_after":0.01}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	da := NewDiscordAdapter("test-token", nil)
	da.apiURL = server.URL
	da.client = server.Client()

	resp := &gateway.GatewayResponse{Content: "test", SessionID: "s1"}
	err := da.Send("channel-123", resp)
	if err == nil {
		t.Error("expected rate limit error on first attempt")
	}
}

func TestDiscordNonOKResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message": "Missing Access"}`))
	}))
	defer server.Close()

	da := NewDiscordAdapter("test-token", nil)
	da.apiURL = server.URL
	da.client = server.Client()

	resp := &gateway.GatewayResponse{Content: "test", SessionID: "s1"}
	err := da.Send("channel-123", resp)
	if err == nil {
		t.Error("expected error for non-OK response")
	}
}

func TestWhatsAppNonOKResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Invalid token"}}`))
	}))
	defer server.Close()

	wa := NewWhatsAppAdapter("phone-id", "bad-token", nil)
	wa.apiURL = server.URL
	wa.client = server.Client()

	resp := &gateway.GatewayResponse{Content: "test", SessionID: "s1"}
	err := wa.Send("+1234567890", resp)
	if err == nil {
		t.Error("expected error for non-OK response")
	}
}

func TestSignalNonOKResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer server.Close()

	sa := NewSignalAdapter(server.URL, "+1234567890", nil)
	sa.client = server.Client()

	resp := &gateway.GatewayResponse{Content: "test", SessionID: "s1"}
	err := sa.Send("+0987654321", resp)
	if err == nil {
		t.Error("expected error for non-OK response")
	}
}
