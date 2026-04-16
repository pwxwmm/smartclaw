package transports

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestNewTransport_SSE(t *testing.T) {
	cfg := &Config{Type: TransportSSE, Endpoint: "http://localhost/events"}
	transport := NewTransport(cfg)
	if _, ok := transport.(*SSETransport); !ok {
		t.Error("expected SSETransport")
	}
}

func TestNewTransport_WebSocket(t *testing.T) {
	cfg := &Config{Type: TransportWebSocket, Endpoint: "ws://localhost/ws"}
	transport := NewTransport(cfg)
	if _, ok := transport.(*WebSocketTransport); !ok {
		t.Error("expected WebSocketTransport")
	}
}

func TestNewTransport_Hybrid(t *testing.T) {
	cfg := &Config{Type: TransportHybrid, Endpoint: "http://localhost/events"}
	transport := NewTransport(cfg)
	if _, ok := transport.(*HybridTransport); !ok {
		t.Error("expected HybridTransport")
	}
}

func TestNewTransport_Default(t *testing.T) {
	cfg := &Config{Type: "unknown", Endpoint: "http://localhost/events"}
	transport := NewTransport(cfg)
	if _, ok := transport.(*SSETransport); !ok {
		t.Error("unknown type should default to SSETransport")
	}
}

func TestSSETransport_ConnectAndReceive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "data: hello world\n\n")
		flusher.Flush()
		<-r.Context().Done()
	}))
	defer server.Close()

	cfg := &Config{Type: TransportSSE, Endpoint: server.URL}
	transport := NewSSETransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	if !transport.IsConnected() {
		t.Error("should be connected after Connect")
	}

	msg, err := transport.Receive()
	if err != nil {
		t.Fatalf("Receive error: %v", err)
	}
	if string(msg) != "hello world" {
		t.Errorf("message = %q, want %q", string(msg), "hello world")
	}

	if err := transport.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if transport.IsConnected() {
		t.Error("should not be connected after Close")
	}
}

func TestSSETransport_Send(t *testing.T) {
	transport := NewSSETransport(&Config{Endpoint: "http://localhost"})
	err := transport.Send([]byte("test"))
	if err == nil {
		t.Error("SSE Send should return error (read-only transport)")
	}
}

func TestSSETransport_Connect_InvalidURL(t *testing.T) {
	cfg := &Config{Type: TransportSSE, Endpoint: "http://nonexistent-host-invalid:99999/events"}
	transport := NewSSETransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err == nil {
		t.Error("expected error connecting to invalid URL")
	}
}

func TestSSETransport_Connect_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &Config{Type: TransportSSE, Endpoint: server.URL}
	transport := NewSSETransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err == nil {
		t.Error("expected error for non-200 status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500: %v", err)
	}
}

func TestSSETransport_Receive_NotConnected(t *testing.T) {
	transport := NewSSETransport(&Config{Endpoint: "http://localhost"})
	_, err := transport.Receive()
	if err == nil {
		t.Error("expected error when receiving while not connected")
	}
}

func TestSSETransport_CustomHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: ok\n\n")
	}))
	defer server.Close()

	cfg := &Config{
		Type:     TransportSSE,
		Endpoint: server.URL,
		Headers:  map[string]string{"Authorization": "Bearer test-token"},
	}
	transport := NewSSETransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer transport.Close()

	if receivedAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, "Bearer test-token")
	}
}

func TestSSETransport_SkipEmptyLines(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "\n\n\n")
		fmt.Fprintf(w, "data: found\n\n")
	}))
	defer server.Close()

	cfg := &Config{Type: TransportSSE, Endpoint: server.URL}
	transport := NewSSETransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer transport.Close()

	msg, err := transport.Receive()
	if err != nil {
		t.Fatalf("Receive error: %v", err)
	}
	if string(msg) != "found" {
		t.Errorf("message = %q, want %q", string(msg), "found")
	}
}

func TestWebSocketTransport_Connect(t *testing.T) {
	cfg := &Config{Type: TransportWebSocket, Endpoint: "ws://localhost/ws"}
	transport := NewWebSocketTransport(cfg)

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	if !transport.IsConnected() {
		t.Error("should be connected")
	}
}

func TestWebSocketTransport_SendNotConnected(t *testing.T) {
	cfg := &Config{Type: TransportWebSocket, Endpoint: "ws://localhost/ws"}
	transport := NewWebSocketTransport(cfg)

	err := transport.Send([]byte("test"))
	if err == nil {
		t.Error("expected error when sending while not connected")
	}
}

func TestWebSocketTransport_SendAndClose(t *testing.T) {
	cfg := &Config{Type: TransportWebSocket, Endpoint: "ws://localhost/ws"}
	transport := NewWebSocketTransport(cfg)

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	if err := transport.Send([]byte("hello")); err != nil {
		t.Fatalf("Send error: %v", err)
	}

	if err := transport.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if transport.IsConnected() {
		t.Error("should not be connected after close")
	}
}

func TestWebSocketTransport_Receive(t *testing.T) {
	cfg := &Config{Type: TransportWebSocket, Endpoint: "ws://localhost/ws"}
	transport := NewWebSocketTransport(cfg)

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	transport.recvChan <- []byte("test message")

	msg, err := transport.Receive()
	if err != nil {
		t.Fatalf("Receive error: %v", err)
	}
	if string(msg) != "test message" {
		t.Errorf("message = %q, want %q", string(msg), "test message")
	}

	transport.Close()
}

func TestWebSocketTransport_Receive_Error(t *testing.T) {
	cfg := &Config{Type: TransportWebSocket, Endpoint: "ws://localhost/ws"}
	transport := NewWebSocketTransport(cfg)

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	testErr := fmt.Errorf("test error")
	transport.errChan <- testErr

	_, err := transport.Receive()
	if err == nil {
		t.Error("expected error from errChan")
	}

	transport.Close()
}

func TestWebSocketTransport_Receive_Cancelled(t *testing.T) {
	cfg := &Config{Type: TransportWebSocket, Endpoint: "ws://localhost/ws"}
	transport := NewWebSocketTransport(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	cancel()

	_, err := transport.Receive()
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestWebSocketTransport_Send_Timeout(t *testing.T) {
	cfg := &Config{Type: TransportWebSocket, Endpoint: "ws://localhost/ws"}
	transport := NewWebSocketTransport(cfg)

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}

	transport.Close()

	sendCtx, sendCancel := context.WithCancel(context.Background())
	sendCancel()

	transport2 := NewWebSocketTransport(cfg)
	transport2.Connect(sendCtx)

	_ = transport2
}

func TestWebSocketTransport_Headers(t *testing.T) {
	cfg := &Config{
		Type:     TransportWebSocket,
		Endpoint: "ws://localhost/ws",
		Headers:  map[string]string{"Authorization": "Bearer token123"},
	}
	transport := NewWebSocketTransport(cfg)

	h := transport.Headers()
	if h.Get("Authorization") != "Bearer token123" {
		t.Errorf("Authorization header = %q, want %q", h.Get("Authorization"), "Bearer token123")
	}
}

func TestHybridTransport_ModeSwitch(t *testing.T) {
	cfg := &Config{Type: TransportHybrid, Endpoint: "http://localhost/events"}
	transport := NewHybridTransport(cfg)

	if transport.GetMode() != "sse" {
		t.Errorf("initial mode = %q, want sse", transport.GetMode())
	}

	if err := transport.SwitchMode("websocket"); err != nil {
		t.Fatalf("SwitchMode error: %v", err)
	}
	if transport.GetMode() != "websocket" {
		t.Errorf("mode after switch = %q, want websocket", transport.GetMode())
	}

	if err := transport.SwitchMode("sse"); err != nil {
		t.Fatalf("SwitchMode error: %v", err)
	}
	if transport.GetMode() != "sse" {
		t.Errorf("mode after switch back = %q, want sse", transport.GetMode())
	}
}

func TestHybridTransport_InvalidMode(t *testing.T) {
	cfg := &Config{Type: TransportHybrid, Endpoint: "http://localhost/events"}
	transport := NewHybridTransport(cfg)

	err := transport.SwitchMode("invalid")
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestHybridTransport_NotConnected(t *testing.T) {
	cfg := &Config{Type: TransportHybrid, Endpoint: "http://localhost/events"}
	transport := NewHybridTransport(cfg)

	if transport.IsConnected() {
		t.Error("should not be connected before Connect")
	}

	_, err := transport.Receive()
	if err == nil {
		t.Error("expected error when receiving while not connected")
	}

	err = transport.Send([]byte("test"))
	if err == nil {
		t.Error("expected error when sending while not connected")
	}
}

func TestHybridTransport_ConnectAndClose(t *testing.T) {
	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: hybrid-sse\n\n")
	}))
	defer sseServer.Close()

	cfg := &Config{Type: TransportHybrid, Endpoint: sseServer.URL}
	transport := NewHybridTransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	if !transport.IsConnected() {
		t.Error("should be connected")
	}

	if err := transport.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if transport.IsConnected() {
		t.Error("should not be connected after close")
	}
}

func TestHybridTransport_SSEConnectFails(t *testing.T) {
	cfg := &Config{Type: TransportHybrid, Endpoint: "http://nonexistent-host-invalid:99999/events"}
	transport := NewHybridTransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err == nil {
		t.Error("expected error when SSE connect fails")
	}
}

func TestHybridTransport_ReceiveFromSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: from-sse\n\n")
	}))
	defer server.Close()

	cfg := &Config{Type: TransportHybrid, Endpoint: server.URL}
	transport := NewHybridTransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer transport.Close()

	msg, err := transport.Receive()
	if err != nil {
		t.Fatalf("Receive error: %v", err)
	}
	if string(msg) != "from-sse" {
		t.Errorf("message = %q, want %q", string(msg), "from-sse")
	}
}

func TestHybridTransport_SendViaWS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: ok\n\n")
	}))
	defer server.Close()

	cfg := &Config{Type: TransportHybrid, Endpoint: server.URL}
	transport := NewHybridTransport(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect error: %v", err)
	}
	defer transport.Close()

	if err := transport.Send([]byte("hello")); err != nil {
		t.Fatalf("Send error: %v", err)
	}
}

func TestWebSocketServerIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var msg map[string]any
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return
		}

		if err := wsjson.Write(ctx, conn, map[string]any{"echo": msg}); err != nil {
			return
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Dial error: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := wsjson.Write(ctx, conn, map[string]any{"hello": "world"}); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	var response map[string]any
	if err := wsjson.Read(ctx, conn, &response); err != nil {
		t.Fatalf("Read error: %v", err)
	}

	echo, ok := response["echo"].(map[string]any)
	if !ok {
		t.Fatal("response should have echo field")
	}
	if echo["hello"] != "world" {
		t.Errorf("echo hello = %v, want world", echo["hello"])
	}
}

func TestTrimNewline(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello\n", "hello"},
		{"hello\r\n", "hello"},
		{"hello\r", "hello"},
		{"hello", "hello"},
		{"\n", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := string(trimNewline([]byte(tt.input)))
		if got != tt.want {
			t.Errorf("trimNewline(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsDataLine(t *testing.T) {
	if !isDataLine([]byte("data: hello")) {
		t.Error("data: hello should be a data line")
	}
	if isDataLine([]byte("event: msg")) {
		t.Error("event: msg should not be a data line")
	}
	if isDataLine([]byte("data:")) {
		t.Error("data: (5 chars) should not be a data line")
	}
}

func TestExtractData(t *testing.T) {
	got := string(extractData([]byte("data: hello world")))
	if got != "hello world" {
		t.Errorf("extractData = %q, want %q", got, "hello world")
	}
}

func TestSSETransport_DefaultTimeout(t *testing.T) {
	cfg := &Config{Type: TransportSSE, Endpoint: "http://localhost", Timeout: 0}
	transport := NewSSETransport(cfg)
	if transport.client.Timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", transport.client.Timeout)
	}
}

func TestSSETransport_CustomTimeout(t *testing.T) {
	cfg := &Config{Type: TransportSSE, Endpoint: "http://localhost", Timeout: 30}
	transport := NewSSETransport(cfg)
	if transport.client.Timeout != 30*time.Second {
		t.Errorf("custom timeout = %v, want 30s", transport.client.Timeout)
	}
}
