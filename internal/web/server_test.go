package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/serverauth"
)

func newTestAuthManager(apiKey string) *AuthManager {
	secretKey := make([]byte, 32)
	copy(secretKey, []byte("test-secret-key-32-bytes-long!!"))
	return serverauth.NewAuthManagerWithKey(secretKey, apiKey, "")
}

func newTestAuthManagerWithLegacy(apiKey, legacyToken string) *AuthManager {
	secretKey := make([]byte, 32)
	copy(secretKey, []byte("test-secret-key-32-bytes-long!!"))
	return serverauth.NewAuthManagerWithKey(secretKey, apiKey, legacyToken)
}

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

func TestAuthMiddleware_NoAuthRequired(t *testing.T) {
	am := newTestAuthManager("")
	s := &WebServer{authManager: am}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("no auth required: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_NoAuthFlag(t *testing.T) {
	am := newTestAuthManager("secret123")
	s := &WebServer{authManager: am, noAuth: true}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("noAuth flag: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_ValidSessionToken(t *testing.T) {
	am := newTestAuthManager("secret123")
	s := &WebServer{authManager: am}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token, err := am.GenerateToken("testuser")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid session token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_LegacyToken(t *testing.T) {
	am := newTestAuthManagerWithLegacy("apikey123", "legacy-token-xyz")
	s := &WebServer{authManager: am}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer legacy-token-xyz")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid legacy token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	am := newTestAuthManager("secret123")
	s := &WebServer{authManager: am}
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
	am := newTestAuthManager("secret123")
	s := &WebServer{authManager: am}
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
	am := newTestAuthManager("secret123")
	s := &WebServer{authManager: am}
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

func TestAuthMiddleware_CookieToken(t *testing.T) {
	am := newTestAuthManager("secret123")
	s := &WebServer{authManager: am}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token, err := am.GenerateToken("testuser")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.AddCookie(&http.Cookie{Name: "smartclaw-token", Value: token})
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid cookie token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_QueryToken(t *testing.T) {
	am := newTestAuthManager("secret123")
	s := &WebServer{authManager: am}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	token, err := am.GenerateToken("testuser")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/test?token="+token, nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid query token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimiter_UnderLimit(t *testing.T) {
	rl := serverauth.NewRateLimiter()
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
	rl := serverauth.NewRateLimiter()
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
	rl := serverauth.NewRateLimiter()
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
	rl := serverauth.NewRateLimiter()
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

	count := rl.VisitorCount("9.8.7.6")

	if count != 20 {
		t.Errorf("expected 20 requests counted, got %d", count)
	}
}

func TestNewRateLimiter(t *testing.T) {
	rl := serverauth.NewRateLimiter()
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
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
	am := newTestAuthManagerWithLegacy("apikey", "mytoken")
	s := &WebServer{authManager: am}
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
	am := newTestAuthManager("secret123")
	s := &WebServer{authManager: am}
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

func TestAuthMiddleware_NoAPIKey_AllowsAll(t *testing.T) {
	am := newTestAuthManager("")
	s := &WebServer{authManager: am}
	handler := s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("no API key configured: status = %d, want %d", w.Code, http.StatusOK)
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

func TestAuthManager_GenerateAndValidate(t *testing.T) {
	am := newTestAuthManager("testkey")

	token, err := am.GenerateToken("user1")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	session, err := am.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if session.UserID != "user1" {
		t.Errorf("UserID = %q, want %q", session.UserID, "user1")
	}

	if time.Now().After(session.ExpiresAt) {
		t.Error("session should not be expired")
	}
}

func TestAuthManager_InvalidToken(t *testing.T) {
	am := newTestAuthManager("testkey")

	_, err := am.ValidateToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestAuthManager_Login_ValidKey(t *testing.T) {
	am := newTestAuthManager("correct-key")

	token, err := am.Login("correct-key")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	session, err := am.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if session.UserID != "default" {
		t.Errorf("UserID = %q, want %q", session.UserID, "default")
	}
}

func TestAuthManager_Login_InvalidKey(t *testing.T) {
	am := newTestAuthManager("correct-key")

	_, err := am.Login("wrong-key")
	if err == nil {
		t.Error("expected error for wrong API key")
	}
}

func TestAuthManager_Login_NoKeyRequired(t *testing.T) {
	am := newTestAuthManager("")

	token, err := am.Login("any-key")
	if err != nil {
		t.Fatalf("Login should succeed with no API key configured: %v", err)
	}

	if token == "" {
		t.Error("token should not be empty")
	}
}

func TestAuthManager_ValidateLegacyToken(t *testing.T) {
	am := newTestAuthManagerWithLegacy("apikey", "legacy-token-abc")

	if !am.ValidateLegacyToken("legacy-token-abc") {
		t.Error("should validate correct legacy token")
	}

	if am.ValidateLegacyToken("wrong-legacy") {
		t.Error("should not validate wrong legacy token")
	}
}

func TestAuthManager_IsAuthRequired(t *testing.T) {
	am1 := newTestAuthManager("")
	if am1.IsAuthRequired() {
		t.Error("no API key means auth not required")
	}

	am2 := newTestAuthManager("secret")
	if !am2.IsAuthRequired() {
		t.Error("with API key, auth should be required")
	}

	am3 := newTestAuthManagerWithLegacy("", "legacy")
	if !am3.IsAuthRequired() {
		t.Error("with legacy token, auth should be required")
	}
}

func TestAuthManager_TokenExpiry(t *testing.T) {
	am := newTestAuthManager("testkey")

	token, err := am.GenerateToken("user1")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	am.ExpireSession(token)

	_, err = am.ValidateToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestHandleAuthLogin(t *testing.T) {
	am := newTestAuthManager("test-api-key")
	s := &WebServer{authManager: am}

	body := `{"api_key":"test-api-key"}`
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleAuthLogin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["token"] == "" {
		t.Error("token should not be empty")
	}

	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "smartclaw-token" {
			found = true
			if !c.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Error("cookie should be SameSiteStrict")
			}
		}
	}
	if !found {
		t.Error("smartclaw-token cookie should be set")
	}
}

func TestHandleAuthLogin_InvalidKey(t *testing.T) {
	am := newTestAuthManager("correct-key")
	s := &WebServer{authManager: am}

	body := `{"api_key":"wrong-key"}`
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleAuthLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleAuthStatus_Authenticated(t *testing.T) {
	am := newTestAuthManager("")
	s := &WebServer{authManager: am}

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	w := httptest.NewRecorder()

	s.handleAuthStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !result["authenticated"] {
		t.Error("should be authenticated when no API key required")
	}
}

func TestHandleAuthStatus_NoAuthFlag(t *testing.T) {
	am := newTestAuthManager("secret")
	s := &WebServer{authManager: am, noAuth: true}

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	w := httptest.NewRecorder()

	s.handleAuthStatus(w, req)

	var result map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !result["authenticated"] {
		t.Error("should be authenticated with noAuth flag")
	}
}

func TestHandleAuthStatus_Unauthenticated(t *testing.T) {
	am := newTestAuthManager("secret")
	s := &WebServer{authManager: am}

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	w := httptest.NewRecorder()

	s.handleAuthStatus(w, req)

	var result map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["authenticated"] {
		t.Error("should not be authenticated without valid token")
	}
}

func TestHandleAuthStatus_WithToken(t *testing.T) {
	am := newTestAuthManager("secret")
	s := &WebServer{authManager: am}

	token, err := am.GenerateToken("testuser")
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	req.AddCookie(&http.Cookie{Name: "smartclaw-token", Value: token})
	w := httptest.NewRecorder()

	s.handleAuthStatus(w, req)

	var result map[string]bool
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !result["authenticated"] {
		t.Error("should be authenticated with valid session token")
	}
}

func TestExtractToken_CookiePreferred(t *testing.T) {
	am := newTestAuthManager("secret")
	s := &WebServer{authManager: am}

	token, _ := am.GenerateToken("user")

	req := httptest.NewRequest("GET", "/api/test?token=query-token", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: "smartclaw-token", Value: token})

	extracted := s.extractToken(req)
	if extracted != token {
		t.Errorf("cookie token should be preferred, got %q", extracted)
	}
}

func TestExtractToken_HeaderFallback(t *testing.T) {
	am := newTestAuthManager("secret")
	s := &WebServer{authManager: am}

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer header-token")

	extracted := s.extractToken(req)
	if extracted != "header-token" {
		t.Errorf("header token fallback failed, got %q", extracted)
	}
}

func TestExtractToken_QueryFallback(t *testing.T) {
	am := newTestAuthManager("secret")
	s := &WebServer{authManager: am}

	req := httptest.NewRequest("GET", "/api/test?token=query-token", nil)

	extracted := s.extractToken(req)
	if extracted != "query-token" {
		t.Errorf("query token fallback failed, got %q", extracted)
	}
}

func TestAuthManager_CleanupExpired(t *testing.T) {
	am := newTestAuthManager("testkey")

	token, _ := am.GenerateToken("user1")

	am.ExpireSession(token)

	cleaned := am.CleanupExpiredNow()

	if cleaned != 1 {
		t.Errorf("expected 1 expired session cleaned, got %d", cleaned)
	}

	if am.SessionExists(token) {
		t.Error("expired session should be removed from sessions map")
	}
}
