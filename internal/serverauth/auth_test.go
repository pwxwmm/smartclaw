package serverauth

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func newTestAuthManager(apiKey string) *AuthManager {
	secretKey := make([]byte, 32)
	copy(secretKey, []byte("test-secret-key-32-bytes-long!!"))
	return NewAuthManagerWithKey(secretKey, apiKey, "")
}

func newTestAuthManagerWithLegacy(apiKey, legacyToken string) *AuthManager {
	secretKey := make([]byte, 32)
	copy(secretKey, []byte("test-secret-key-32-bytes-long!!"))
	return NewAuthManagerWithKey(secretKey, apiKey, legacyToken)
}

func TestNewAuthManager(t *testing.T) {
	am, err := NewAuthManager()
	if err != nil {
		t.Fatalf("NewAuthManager returned error: %v", err)
	}
	if am == nil {
		t.Fatal("NewAuthManager returned nil")
	}
	if am.IsAuthRequired() {
		t.Error("new AuthManager should not require auth by default (no env set)")
	}
}

func TestLoginValidAPIKey(t *testing.T) {
	am := newTestAuthManager("correct-key")

	token, err := am.Login("correct-key")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}

	session, err := am.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if session.UserID != "default" {
		t.Errorf("UserID = %q, want %q", session.UserID, "default")
	}
}

func TestLoginInvalidAPIKey(t *testing.T) {
	am := newTestAuthManager("correct-key")

	_, err := am.Login("wrong-key")
	if err == nil {
		t.Error("expected error for wrong API key")
	}
}

func TestLoginNoKeyRequired(t *testing.T) {
	am := newTestAuthManager("")

	token, err := am.Login("any-key")
	if err != nil {
		t.Fatalf("Login should succeed with no API key configured: %v", err)
	}
	if token == "" {
		t.Error("token should not be empty")
	}
}

func TestValidateToken(t *testing.T) {
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

func TestValidateExpiredToken(t *testing.T) {
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

func TestValidateLegacyToken(t *testing.T) {
	am := newTestAuthManagerWithLegacy("apikey", "legacy-token-abc")

	if !am.ValidateLegacyToken("legacy-token-abc") {
		t.Error("should validate correct legacy token")
	}

	if am.ValidateLegacyToken("wrong-legacy") {
		t.Error("should not validate wrong legacy token")
	}

	am2 := newTestAuthManager("apikey")
	if am2.ValidateLegacyToken("any-token") {
		t.Error("should not validate when no legacy token configured")
	}
}

func TestIsAuthRequired(t *testing.T) {
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

func TestAuthMiddleware_Protection(t *testing.T) {
	am := newTestAuthManager("secret123")
	handler := AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, am, false)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing token: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	token, _ := am.GenerateToken("testuser")
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("valid token: status = %d, want %d", w2.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_SkipsWebSocket(t *testing.T) {
	am := newTestAuthManager("secret123")
	handler := AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, am, false)

	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("websocket upgrade bypasses auth: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_NoAuthFlag(t *testing.T) {
	am := newTestAuthManager("secret123")
	handler := AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, am, true)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("noAuth flag: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_NoAuthRequired(t *testing.T) {
	am := newTestAuthManager("")
	handler := AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, am, false)

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("no auth required: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAuthMiddleware_LegacyToken(t *testing.T) {
	am := newTestAuthManagerWithLegacy("apikey123", "legacy-token-xyz")
	handler := AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, am, false)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer legacy-token-xyz")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid legacy token: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := CORSMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("OPTIONS: status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS origin header not set")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("CORS methods header not set")
	}

	req2 := httptest.NewRequest("GET", "/api/test", nil)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("GET with CORS: status = %d, want %d", w2.Code, http.StatusOK)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	rl := NewRateLimiter()
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d under limit: status = %d", i, w.Code)
			break
		}
	}

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over limit: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter()
	handler := rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 50; i++ {
		req1 := httptest.NewRequest("GET", "/api/test", nil)
		req1.RemoteAddr = "10.0.0.1:1234"
		w1 := httptest.NewRecorder()
		handler(w1, req1)
		if w1.Code != http.StatusOK {
			t.Errorf("ip1 request %d: status = %d", i, w1.Code)
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

func TestRateLimitMiddleware_Concurrent(t *testing.T) {
	rl := NewRateLimiter()
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

func TestExtractToken_CookiePreferred(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test?token=query-token", nil)
	req.Header.Set("Authorization", "Bearer header-token")
	req.AddCookie(&http.Cookie{Name: "smartclaw-token", Value: "cookie-token"})

	token := ExtractToken(req)
	if token != "cookie-token" {
		t.Errorf("cookie token should be preferred, got %q", token)
	}
}

func TestExtractToken_HeaderFallback(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer header-token")

	token := ExtractToken(req)
	if token != "header-token" {
		t.Errorf("header token fallback failed, got %q", token)
	}
}

func TestExtractToken_QueryFallback(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test?token=query-token", nil)

	token := ExtractToken(req)
	if token != "query-token" {
		t.Errorf("query token fallback failed, got %q", token)
	}
}

func TestExtractToken_NoneProvided(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/test", nil)

	token := ExtractToken(req)
	if token != "" {
		t.Errorf("no token should return empty string, got %q", token)
	}
}

func TestValidateAccessToken_EmptyToken(t *testing.T) {
	am := newTestAuthManager("secret")
	if ValidateAccessToken("", am) {
		t.Error("empty token should not be valid")
	}
}

func TestValidateAccessToken_NilManager(t *testing.T) {
	if ValidateAccessToken("some-token", nil) {
		t.Error("nil manager should not validate token")
	}
}

func TestExpireSession(t *testing.T) {
	am := newTestAuthManager("testkey")

	token, _ := am.GenerateToken("user1")
	if !am.SessionExists(token) {
		t.Error("session should exist after generation")
	}

	am.ExpireSession(token)

	_, err := am.ValidateToken(token)
	if err == nil {
		t.Error("expected error for expired token after ExpireSession")
	}
}

func TestCleanupExpiredNow(t *testing.T) {
	am := newTestAuthManager("testkey")

	token1, _ := am.GenerateToken("user1")
	am.GenerateToken("user2")

	am.ExpireSession(token1)

	cleaned := am.CleanupExpiredNow()
	if cleaned != 1 {
		t.Errorf("expected 1 expired session cleaned, got %d", cleaned)
	}

	if am.SessionExists(token1) {
		t.Error("expired session should be removed")
	}
}

func TestSessionDuration(t *testing.T) {
	if SessionDuration != 24*time.Hour {
		t.Errorf("SessionDuration = %v, want %v", SessionDuration, 24*time.Hour)
	}
}
