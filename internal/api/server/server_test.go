package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/serverauth"
	"github.com/instructkr/smartclaw/internal/session"
	"github.com/instructkr/smartclaw/internal/store"
)

func newTestAPIServer(t *testing.T) *APIServer {
	t.Helper()
	tmpDir := t.TempDir()
	st, err := store.NewStoreWithDir(tmpDir)
	if err != nil {
		t.Fatalf("NewStoreWithDir: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	sessMgr, err := session.NewManager()
	if err != nil {
		t.Fatalf("session.NewManager: %v", err)
	}

	mcpReg := mcp.NewMCPServerRegistry()

	srv, err := NewAPIServer(nil, st, nil, sessMgr, mcpReg, false, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewAPIServer: %v", err)
	}
	return srv
}

func newTestAPIServerNoAuth(t *testing.T) *APIServer {
	t.Helper()
	srv := newTestAPIServer(t)
	srv.noAuth = true
	return srv
}

func newTestAPIServerWithAuth(t *testing.T, apiKey string) *APIServer {
	t.Helper()
	srv := newTestAPIServer(t)
	srv.auth = serverauth.NewAuthManagerWithKey(
		[]byte("test-secret-key-32-bytes-long!!"),
		apiKey,
		"",
	)
	return srv
}

func testServer(t *testing.T, srv *APIServer) *httptest.Server {
	t.Helper()
	mux := srv.registerRoutes()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func getValidToken(t *testing.T, srv *APIServer) string {
	t.Helper()
	token, err := srv.auth.Login("test-api-key")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return token
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	ts := testServer(t, srv)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

func TestAuthLogin(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)

	payload, _ := json.Marshal(map[string]string{"api_key": "test-api-key"})
	resp, err := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["token"] == "" {
		t.Error("token should not be empty")
	}

	cookie := resp.Cookies()
	found := false
	for _, c := range cookie {
		if c.Name == "smartclaw-token" {
			found = true
			if c.Value == "" {
				t.Error("cookie value should not be empty")
			}
			if !c.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
		}
	}
	if !found {
		t.Error("smartclaw-token cookie not set")
	}
}

func TestAuthLoginInvalidKey(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "correct-key")
	ts := testServer(t, srv)

	payload, _ := json.Marshal(map[string]string{"api_key": "wrong-key"})
	resp, err := http.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST /api/auth/login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestAuthStatus(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)

	resp, err := http.Get(ts.URL + "/api/auth/status")
	if err != nil {
		t.Fatalf("GET /api/auth/status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["authenticated"] {
		t.Error("should not be authenticated without token")
	}
}

func TestAuthStatusAuthenticated(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)
	token := getValidToken(t, srv)

	req, _ := http.NewRequest("GET", ts.URL+"/api/auth/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/auth/status: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]bool
	json.NewDecoder(resp.Body).Decode(&body)
	if !body["authenticated"] {
		t.Error("should be authenticated with valid token")
	}
}

func TestProtectedEndpointWithoutAuth(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)

	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestProtectedEndpointWithAuth(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)
	token := getValidToken(t, srv)

	req, _ := http.NewRequest("GET", ts.URL+"/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
	}
}

func TestProtectedEndpointNoAuthMode(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	ts := testServer(t, srv)

	resp, err := http.Get(ts.URL + "/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d (noAuth bypasses auth)", resp.StatusCode, http.StatusOK)
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	ts := testServer(t, srv)

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/sessions", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS /api/sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("OPTIONS status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", origin, "*")
	}

	methods := resp.Header.Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("Access-Control-Allow-Methods not set")
	}

	headers := resp.Header.Get("Access-Control-Allow-Headers")
	if headers == "" {
		t.Error("Access-Control-Allow-Headers not set")
	}

	maxAge := resp.Header.Get("Access-Control-Max-Age")
	if maxAge == "" {
		t.Error("Access-Control-Max-Age not set")
	}
}

func TestCORSHeadersOnGET(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	ts := testServer(t, srv)

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("CORS origin on GET = %q, want %q", origin, "*")
	}
}

func TestChatEndpointNoGateway(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)
	token := getValidToken(t, srv)

	payload, _ := json.Marshal(map[string]string{"content": "hello"})
	req, _ := http.NewRequest("POST", ts.URL+"/api/chat", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/chat: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d (gateway nil)", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestChatEndpointMethodNotAllowed(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	ts := testServer(t, srv)

	req, _ := http.NewRequest("GET", ts.URL+"/api/chat", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/chat: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestSessionListEndpoint(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)
	token := getValidToken(t, srv)

	req, _ := http.NewRequest("GET", ts.URL+"/api/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}

func TestConfigEndpoint(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)
	token := getValidToken(t, srv)

	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".smartclaw")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")

	originalData, readErr := os.ReadFile(configPath)

	testConfig := map[string]any{
		"api_key": "sk-ant-test1234567890abcd",
		"model":   "claude-sonnet-4-5",
	}
	configData, _ := json.Marshal(testConfig)
	os.WriteFile(configPath, configData, 0644)
	t.Cleanup(func() {
		if readErr == nil {
			os.WriteFile(configPath, originalData, 0644)
		} else {
			os.Remove(configPath)
		}
	})

	req, _ := http.NewRequest("GET", ts.URL+"/api/config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d; body=%s", resp.StatusCode, body)
	}

	var config map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if apiKey, ok := config["api_key"].(string); ok {
		if apiKey == "sk-ant-test1234567890abcd" {
			t.Error("api_key should be masked")
		}
		if len(apiKey) < 3 {
			t.Error("masked api_key too short")
		}
	}
}

func TestIndexEndpoint(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	ts := testServer(t, srv)

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["name"] != "smartclaw-api" {
		t.Errorf("name = %q, want %q", body["name"], "smartclaw-api")
	}
}

func TestNotFoundEndpoint(t *testing.T) {
	srv := newTestAPIServerNoAuth(t)
	ts := testServer(t, srv)

	resp, err := http.Get(ts.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET /nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv := newTestAPIServerWithAuth(t, "test-api-key")
	ts := testServer(t, srv)
	token := getValidToken(t, srv)

	req, _ := http.NewRequest("GET", ts.URL+"/api/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAPIServerClose(t *testing.T) {
	srv := newTestAPIServer(t)
	if err := srv.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestAPIServerShutdown(t *testing.T) {
	srv := newTestAPIServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error: %v", err)
	}
}

func TestSetAPIKey(t *testing.T) {
	srv := newTestAPIServer(t)
	srv.SetAPIKey("new-key")
	if !srv.auth.IsAuthRequired() {
		t.Error("auth should be required after SetAPIKey")
	}
}
