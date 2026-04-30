package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func createSessionSafe(t *testing.T, m *DirectConnectManager, ctx context.Context, projectID string) *DirectConnectSession {
	t.Helper()
	s, err := m.CreateSession(ctx, projectID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	time.Sleep(time.Microsecond)
	return s
}

func TestNewDirectConnectManager(t *testing.T) {
	m, err := NewDirectConnectManager(8080, "", "token123", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.port != 8080 {
		t.Errorf("expected port 8080, got %d", m.port)
	}
	if m.authToken != "token123" {
		t.Errorf("expected authToken token123, got %q", m.authToken)
	}
	if len(m.sessions) != 0 {
		t.Errorf("expected empty sessions map, got %d entries", len(m.sessions))
	}
}

func TestNewDirectConnectManager_EmptyAuthToken(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.authToken != "" {
		t.Errorf("expected empty auth token, got %q", m.authToken)
	}
}

func TestCreateSession(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, err := m.CreateSession(ctx, "project-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.ProjectID != "project-1" {
		t.Errorf("expected projectID project-1, got %q", session.ProjectID)
	}
	if session.Metadata == nil {
		t.Error("expected initialized metadata map")
	}
	if session.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
	if session.LastActivity.IsZero() {
		t.Error("expected non-zero LastActivity")
	}
}

func TestCreateSession_Multiple(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s1, _ := m.CreateSession(ctx, "p1")
	time.Sleep(time.Microsecond)
	s2, _ := m.CreateSession(ctx, "p2")
	if s1.ID == s2.ID {
		t.Error("expected unique session IDs")
	}
}

func TestGetSession_Exists(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, _ := m.CreateSession(ctx, "project-x")
	found, ok := m.GetSession(created.ID)
	if !ok {
		t.Error("expected session to be found")
	}
	if found.ID != created.ID {
		t.Errorf("expected session ID %q, got %q", created.ID, found.ID)
	}
}

func TestGetSession_NotExists(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	_, ok := m.GetSession("nonexistent")
	if ok {
		t.Error("expected session not to be found")
	}
}

func TestUpdateActivity(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, _ := m.CreateSession(ctx, "project")
	original := session.LastActivity

	time.Sleep(time.Millisecond * 10)
	m.UpdateActivity(session.ID)

	found, _ := m.GetSession(session.ID)
	if !found.LastActivity.After(original) {
		t.Error("expected LastActivity to be updated")
	}
}

func TestUpdateActivity_NonexistentSession(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	m.UpdateActivity("nonexistent")
}

func TestRemoveSession(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, _ := m.CreateSession(ctx, "project")
	m.RemoveSession(session.ID)
	_, ok := m.GetSession(session.ID)
	if ok {
		t.Error("expected session to be removed")
	}
}

func TestRemoveSession_Nonexistent(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	m.RemoveSession("nonexistent")
}

func TestListSessions_Empty(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	sessions := m.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_Multiple(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	s1, _ := m.CreateSession(ctx, "p1")
	time.Sleep(time.Microsecond)
	s2, _ := m.CreateSession(ctx, "p2")
	time.Sleep(time.Microsecond)
	s3, _ := m.CreateSession(ctx, "p3")
	sessions := m.ListSessions()
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
	for _, want := range []*DirectConnectSession{s1, s2, s3} {
		got, ok := m.GetSession(want.ID)
		if !ok || got.ProjectID != want.ProjectID {
			t.Errorf("session %s not found or project mismatch", want.ID)
		}
	}
}

func TestCleanupStaleSessions_RemovesOld(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, _ := m.CreateSession(ctx, "project")

	m.mu.Lock()
	session.LastActivity = time.Now().Add(-2 * time.Hour)
	m.mu.Unlock()

	m.CleanupStaleSessions(time.Hour)
	_, ok := m.GetSession(session.ID)
	if ok {
		t.Error("expected stale session to be cleaned up")
	}
}

func TestCleanupStaleSessions_KeepsFresh(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	session, _ := m.CreateSession(ctx, "project")

	m.CleanupStaleSessions(time.Hour)
	_, ok := m.GetSession(session.ID)
	if !ok {
		t.Error("expected fresh session to remain")
	}
}

func TestCleanupStaleSessions_Mixed(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	oldSession := createSessionSafe(t, m, ctx, "old")
	freshSession := createSessionSafe(t, m, ctx, "fresh")

	m.mu.Lock()
	oldSession.LastActivity = time.Now().Add(-2 * time.Hour)
	m.mu.Unlock()

	m.CleanupStaleSessions(time.Hour)

	_, oldOk := m.GetSession(oldSession.ID)
	_, freshOk := m.GetSession(freshSession.ID)
	if oldOk {
		t.Error("expected old session to be cleaned up")
	}
	if !freshOk {
		t.Error("expected fresh session to remain")
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if !called {
		t.Error("expected handler to be called when no auth token configured")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidBearer(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "secret123", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if !called {
		t.Error("expected handler to be called with valid token")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "secret123", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	handler := m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid token")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "secret123", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	handler := m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without auth header")
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_WrongScheme(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "secret123", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	handler := m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with wrong scheme")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic secret123")
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestGetPort(t *testing.T) {
	m, err := NewDirectConnectManager(9999, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.GetPort() != 9999 {
		t.Errorf("expected port 9999, got %d", m.GetPort())
	}
}

func TestHTTPHandlers_ListSessions(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "tok", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	createSessionSafe(t, m, ctx, "proj1")
	createSessionSafe(t, m, ctx, "proj2")

	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			sessions := m.ListSessions()
			json.NewEncoder(w).Encode(sessions)
		}
	}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/sessions", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var sessions []*DirectConnectSession
	json.NewDecoder(resp.Body).Decode(&sessions)
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestHTTPHandlers_CreateSession(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "tok", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req struct {
				ProjectID string `json:"project_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			session, err := m.CreateSession(r.Context(), req.ProjectID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(session)
		}
	}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := `{"project_id":"test-project"}`
	req, _ := http.NewRequest("POST", srv.URL+"/sessions", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var session DirectConnectSession
	json.NewDecoder(resp.Body).Decode(&session)
	if session.ProjectID != "test-project" {
		t.Errorf("expected project_id test-project, got %q", session.ProjectID)
	}
}

func TestHTTPHandlers_GetSession(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "tok", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	created, _ := m.CreateSession(ctx, "proj")

	mux := http.NewServeMux()
	mux.HandleFunc("/sessions/", m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/sessions/"):]
		session, exists := m.GetSession(id)
		if !exists {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(session)
	}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/sessions/"+created.ID, nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTPHandlers_GetSession_NotFound(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "tok", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sessions/", m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/sessions/"):]
		session, exists := m.GetSession(id)
		if !exists {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(session)
	}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/sessions/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHTTPHandlers_AuthRequired(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "mytoken", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/sessions", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", resp.StatusCode)
	}
}

func TestStartStop(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "test-token", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	stopErr := m.Stop(ctx)
	if stopErr != nil {
		t.Errorf("expected nil from Stop on unstarted server, got %v", stopErr)
	}
}

func TestConcurrentAccess(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, _ := m.CreateSession(ctx, "proj")
			m.GetSession(s.ID)
			m.UpdateActivity(s.ID)
			m.ListSessions()
		}(i)
	}
	wg.Wait()

	sessions := m.ListSessions()
	if len(sessions) != 100 {
		t.Errorf("expected 100 sessions, got %d", len(sessions))
	}
}

func TestHTTPHandlers_CreateSession_InvalidBody(t *testing.T) {
	m, err := NewDirectConnectManager(0, "", "tok", false, nil, "")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/sessions", m.legacyAuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var req struct {
				ProjectID string `json:"project_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			session, err := m.CreateSession(r.Context(), req.ProjectID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(session)
		}
	}))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/sessions", strings.NewReader("not-json"))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}
