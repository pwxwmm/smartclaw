package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func startHub(srv *APIServer) {
	go srv.hub.Run()
}

func TestHubRegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := newWSClient(hub, "user1")
	hub.Register(client)

	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Errorf("ClientCount = %d, want 1", hub.ClientCount())
	}

	hub.Unregister(client)
	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount after unregister = %d, want 0", hub.ClientCount())
	}
}

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client1 := newWSClient(hub, "user1")
	client2 := newWSClient(hub, "user2")
	hub.Register(client1)
	hub.Register(client2)

	time.Sleep(50 * time.Millisecond)

	msg := []byte(`{"type":"test","content":"hello"}`)
	hub.Broadcast(msg)

	timeout := time.After(2 * time.Second)
	select {
	case received := <-client1.send:
		if string(received) != string(msg) {
			t.Errorf("client1 received %q, want %q", string(received), string(msg))
		}
	case <-timeout:
		t.Error("client1 did not receive broadcast message")
	}

	select {
	case received := <-client2.send:
		if string(received) != string(msg) {
			t.Errorf("client2 received %q, want %q", string(received), string(msg))
		}
	case <-timeout:
		t.Error("client2 did not receive broadcast message")
	}
}

func TestHubBroadcast_DroppedClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := newWSClient(hub, "user1")
	hub.Register(client)
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 256; i++ {
		select {
		case client.send <- []byte("fill"):
		default:
		}
	}

	hub.Broadcast([]byte("overflow"))
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount = %d, want 0 (client should be dropped)", hub.ClientCount())
	}
}

func TestWebSocketUpgradeNoAuth(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	startHub(srv)
	mux := srv.registerRoutes()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	_, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err == nil {
		t.Error("expected error for WebSocket without auth token")
	}
}

func TestWebSocketUpgradeWithToken(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	startHub(srv)
	mux := srv.registerRoutes()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	token := getValidToken(t, srv)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?token=" + token
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial with token: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	time.Sleep(100 * time.Millisecond)
	if srv.hub.ClientCount() < 1 {
		t.Errorf("ClientCount = %d, want >= 1", srv.hub.ClientCount())
	}
}

func TestWebSocketNoAuthMode(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	startHub(srv)
	mux := srv.registerRoutes()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial noAuth: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
}

func TestWebSocketSessionListMessage(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	startHub(srv)
	mux := srv.registerRoutes()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	time.Sleep(100 * time.Millisecond)

	msg := wsMessage{Type: "session_list"}
	msgData, _ := json.Marshal(msg)
	err = conn.Write(ctx, websocket.MessageText, msgData)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	_, received, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	var resp wsResponse
	if err := json.Unmarshal(received, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.Type != "session_list" {
		t.Errorf("response type = %q, want %q", resp.Type, "session_list")
	}
}

func TestWebSocketInvalidMessage(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	startHub(srv)
	mux := srv.registerRoutes()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	time.Sleep(100 * time.Millisecond)

	err = conn.Write(ctx, websocket.MessageText, []byte("not-json"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	_, received, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	var resp wsResponse
	if err := json.Unmarshal(received, &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.Type != "error" {
		t.Errorf("response type = %q, want %q", resp.Type, "error")
	}
}

func TestWebSocketUnknownMessageType(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	startHub(srv)
	mux := srv.registerRoutes()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	time.Sleep(100 * time.Millisecond)

	msg := wsMessage{Type: "unknown_type"}
	msgData, _ := json.Marshal(msg)
	conn.Write(ctx, websocket.MessageText, msgData)

	_, received, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	var resp wsResponse
	json.Unmarshal(received, &resp)
	if resp.Type != "error" {
		t.Errorf("response type = %q, want %q", resp.Type, "error")
	}
}

func TestWebSocketChatNoGateway(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	startHub(srv)
	mux := srv.registerRoutes()
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	time.Sleep(100 * time.Millisecond)

	msg := wsMessage{Type: "chat", Content: "hello"}
	msgData, _ := json.Marshal(msg)
	conn.Write(ctx, websocket.MessageText, msgData)

	_, received, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	var resp wsResponse
	json.Unmarshal(received, &resp)
	if resp.Type != "error" {
		t.Errorf("response type = %q, want %q (gateway nil)", resp.Type, "error")
	}
	if !strings.Contains(resp.Message, "Gateway not available") {
		t.Errorf("error message = %q, should mention gateway", resp.Message)
	}
}

func TestNewWSClientDefaultUserID(t *testing.T) {
	hub := NewHub()
	client := newWSClient(hub, "")
	if client.UserID != "default" {
		t.Errorf("UserID = %q, want %q", client.UserID, "default")
	}

	client2 := newWSClient(hub, "custom-user")
	if client2.UserID != "custom-user" {
		t.Errorf("UserID = %q, want %q", client2.UserID, "custom-user")
	}
}

func TestWebSocketEndpointSkipsAuthForUpgrade(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")

	wrapped := srv.wrapHandler(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	recorder := httptest.NewRecorder()

	wrapped(recorder, req)

	if recorder.Code == http.StatusUnauthorized {
		t.Error("WebSocket upgrade should bypass auth middleware")
	}
}
