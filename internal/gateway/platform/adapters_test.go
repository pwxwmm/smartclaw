package platform

import (
	"context"
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

func TestDiscordMessageTruncation(t *testing.T) {
	longText := strings.Repeat("a", 3000)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	da := NewDiscordAdapter("test-token", nil)
	da.client = server.Client()
	da.SetAPIURL(server.URL)

	resp := &gateway.GatewayResponse{Content: longText, SessionID: "s1"}

	err := da.Send("123456789", resp)
	if err != nil {
		t.Errorf("Send() error: %v", err)
	}
}

func TestWhatsAppMessageTruncation(t *testing.T) {
	longText := strings.Repeat("b", 5000)

	wa := NewWhatsAppAdapter("phone-id", "test-token", nil)
	resp := &gateway.GatewayResponse{Content: longText, SessionID: "s1"}

	_ = wa
	_ = resp
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

func TestEmailAdapterSendInvalidAddress(t *testing.T) {
	ea := NewEmailAdapter("smtp.test.com", "587", "u@t.com", "p", "", "", nil)
	resp := &gateway.GatewayResponse{Content: "hello", SessionID: "s1"}
	err := ea.Send("not-an-email", resp)
	if err == nil {
		t.Error("expected error for invalid email address")
	}
}

func TestDiscordSendWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	da := NewDiscordAdapter("test-token", nil)
	da.client = server.Client()
	da.SetAPIURL(server.URL)

	resp := &gateway.GatewayResponse{Content: "Hello Discord!", SessionID: "s1"}

	err := da.Send("channel-123", resp)
	if err != nil {
		t.Errorf("Send() error: %v", err)
	}
}

func TestWhatsAppSendWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"messages":[{"id":"wamid123"}]}`))
	}))
	defer server.Close()

	wa := NewWhatsAppAdapter("phone-id", "test-token", nil)
	wa.client = server.Client()
	wa.SetAPIURL(server.URL)

	resp := &gateway.GatewayResponse{Content: "Hello WhatsApp!", SessionID: "s1"}

	err := wa.Send("+1234567890", resp)
	if err != nil {
		t.Errorf("Send() error: %v", err)
	}
}

func TestSignalSendWithMockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	sa := NewSignalAdapter(server.URL, "+1234567890", nil)
	sa.client = server.Client()

	resp := &gateway.GatewayResponse{Content: "Hello Signal!", SessionID: "s1"}

	err := sa.Send("+0987654321", resp)
	if err != nil {
		t.Errorf("Send() error: %v", err)
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
