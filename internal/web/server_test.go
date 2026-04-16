package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/observability"
)

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       any
		wantStatus int
	}{
		{"ok", http.StatusOK, map[string]string{"hello": "world"}, http.StatusOK},
		{"created", http.StatusCreated, map[string]string{"id": "123"}, http.StatusCreated},
		{"bad_request", http.StatusBadRequest, map[string]string{"error": "bad"}, http.StatusBadRequest},
		{"not_found", http.StatusNotFound, map[string]string{"error": "missing"}, http.StatusNotFound},
		{"internal_error", http.StatusInternalServerError, map[string]string{"error": "fail"}, http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeJSON(w, tt.status, tt.data)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			var result map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
		})
	}
}

func TestWriteJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, nil)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type not application/json")
	}
}

func TestCacheHitRate(t *testing.T) {
	tests := []struct {
		hits   int64
		misses int64
		want   float64
	}{
		{0, 0, 0},
		{10, 0, 1.0},
		{0, 10, 0},
		{5, 5, 0.5},
		{75, 25, 0.75},
	}
	for _, tt := range tests {
		got := cacheHitRate(tt.hits, tt.misses)
		if got != tt.want {
			t.Errorf("cacheHitRate(%d, %d) = %f, want %f", tt.hits, tt.misses, got, tt.want)
		}
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	s := &WebServer{authToken: ""}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("no token configured: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_ValidBearerToken(t *testing.T) {
	s := &WebServer{authToken: "secret123"}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid bearer token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_ValidQueryToken(t *testing.T) {
	s := &WebServer{authToken: "secret123"}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test?token=secret123", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid query token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	s := &WebServer{authToken: "secret123"}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer wrongtoken")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("invalid token: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	s := &WebServer{authToken: "secret123"}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing token: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_WebSocketUpgrade(t *testing.T) {
	s := &WebServer{authToken: "secret123"}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("websocket upgrade bypasses auth: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_QueryTokenPreferred(t *testing.T) {
	s := &WebServer{authToken: "secret123"}
	called := false
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test?token=secret123", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("handler should have been called")
	}
}

func TestRateLimiter_UnderLimit(t *testing.T) {
	rl := &rateLimiter{
		visitors: make(map[string]*visitorInfo),
	}
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want %d", i, w.Code, http.StatusOK)
		}
	}
}

func TestRateLimiter_OverLimit(t *testing.T) {
	rl := &rateLimiter{
		visitors: make(map[string]*visitorInfo),
	}
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	blocked := false
	for i := 0; i < 110; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "5.6.7.8:5678"
		w := httptest.NewRecorder()
		handler(w, req)

		if w.Code == http.StatusTooManyRequests {
			blocked = true
			break
		}
	}

	if !blocked {
		t.Error("expected rate limiter to block after 100 requests")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := &rateLimiter{
		visitors: make(map[string]*visitorInfo),
	}
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("ip1 request %d: status = %d", i, w.Code)
		}

		req2 := httptest.NewRequest("GET", "/api/test", nil)
		req2.RemoteAddr = "10.0.0.2:5678"
		w2 := httptest.NewRecorder()
		handler(w2, req2)
		if w2.Code != http.StatusOK {
			t.Errorf("ip2 request %d: status = %d", i, w2.Code)
		}
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := &rateLimiter{
		visitors: make(map[string]*visitorInfo),
	}
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/api/test", nil)
			req.RemoteAddr = "9.8.7.6:9999"
			w := httptest.NewRecorder()
			handler(w, req)
		}()
	}
	wg.Wait()

	rl.mu.Lock()
	count := rl.visitors["9.8.7.6"].count
	rl.mu.Unlock()

	if count != 20 {
		t.Errorf("expected 20 requests counted, got %d", count)
	}
}

func TestNewRateLimiter(t *testing.T) {
	rl := newRateLimiter()
	if rl == nil {
		t.Fatal("newRateLimiter returned nil")
	}
	if rl.visitors == nil {
		t.Error("visitors map should be initialized")
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := NewClient(hub, "test-user")

	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.Unregister(client)
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after unregister, got %d", hub.ClientCount())
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := NewClient(hub, "test-user")
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	msg := []byte("test broadcast")
	hub.Broadcast(msg)

	select {
	case received := <-client.send:
		if string(received) != string(msg) {
			t.Errorf("received %q, want %q", received, msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timed out waiting for broadcast message")
	}
}

func TestNewClient(t *testing.T) {
	hub := NewHub()
	client := NewClient(hub, "")

	if client.UserID != "default" {
		t.Errorf("empty userID should default to 'default', got %q", client.UserID)
	}
	if client.ID == "" {
		t.Error("client ID should not be empty")
	}
	if client.send == nil {
		t.Error("send channel should be initialized")
	}
}

func TestNewClient_WithUserID(t *testing.T) {
	hub := NewHub()
	client := NewClient(hub, "alice")

	if client.UserID != "alice" {
		t.Errorf("userID = %q, want %q", client.UserID, "alice")
	}
}

func TestWriteJSON_ComplexData(t *testing.T) {
	tests := []struct {
		name   string
		status int
		data   any
	}{
		{"nested_map", http.StatusOK, map[string]any{"nested": map[string]string{"key": "val"}}},
		{"array", http.StatusOK, []string{"a", "b", "c"}},
		{"number", http.StatusOK, map[string]int{"count": 42}},
		{"bool", http.StatusOK, map[string]bool{"active": true}},
		{"empty_map", http.StatusOK, map[string]string{}},
		{"error_with_details", http.StatusBadRequest, map[string]any{"error": "bad request", "code": 400}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			writeJSON(w, tt.status, tt.data)

			if w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}

			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}

			if w.Body.Len() == 0 {
				t.Error("response body should not be empty")
			}
		})
	}
}

func TestWriteJSON_ResponseBodyContent(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %q, want ok", result["status"])
	}
}

func TestAuthMiddleware_BearerPrefixStripped(t *testing.T) {
	s := &WebServer{authToken: "mytoken"}
	called := false
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer mytoken")
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("handler should have been called with valid Bearer token")
	}
}

func TestAuthMiddleware_InvalidBearerFormat(t *testing.T) {
	s := &WebServer{authToken: "mytoken"}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Basic mytoken")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Basic auth with wrong scheme: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_EmptyStringToken_AllowsAll(t *testing.T) {
	s := &WebServer{authToken: ""}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("no auth token configured: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCacheHitRate_EdgeCases(t *testing.T) {
	if got := cacheHitRate(0, 0); got != 0 {
		t.Errorf("cacheHitRate(0,0) = %f, want 0", got)
	}
	if got := cacheHitRate(1, 0); got != 1.0 {
		t.Errorf("cacheHitRate(1,0) = %f, want 1.0", got)
	}
	if got := cacheHitRate(0, 1); got != 0 {
		t.Errorf("cacheHitRate(0,1) = %f, want 0", got)
	}
	if got := cacheHitRate(3, 1); got != 0.75 {
		t.Errorf("cacheHitRate(3,1) = %f, want 0.75", got)
	}
}

func TestEstimateCost_NoModel(t *testing.T) {
	snapshot := observability.MetricsSnapshot{}
	cost := estimateCost(snapshot)
	if cost < 0 {
		t.Errorf("cost should not be negative, got %f", cost)
	}
}
