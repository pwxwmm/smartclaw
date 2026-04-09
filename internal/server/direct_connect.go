package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type DirectConnectSession struct {
	ID           string            `json:"id"`
	ClientID     string            `json:"client_id"`
	ProjectID    string            `json:"project_id"`
	CreatedAt    time.Time         `json:"created_at"`
	LastActivity time.Time         `json:"last_activity"`
	Metadata     map[string]string `json:"metadata"`
}

type DirectConnectManager struct {
	sessions map[string]*DirectConnectSession
	mu       sync.RWMutex
	port     int
	server   *http.Server
}

func NewDirectConnectManager(port int) *DirectConnectManager {
	return &DirectConnectManager{
		sessions: make(map[string]*DirectConnectSession),
		port:     port,
	}
}

func (m *DirectConnectManager) CreateSession(ctx context.Context, projectID string) (*DirectConnectSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &DirectConnectSession{
		ID:           fmt.Sprintf("session_%d", time.Now().UnixNano()),
		ClientID:     fmt.Sprintf("client_%d", time.Now().UnixNano()),
		ProjectID:    projectID,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Metadata:     make(map[string]string),
	}

	m.sessions[session.ID] = session
	return session, nil
}

func (m *DirectConnectManager) GetSession(id string) (*DirectConnectSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	return session, exists
}

func (m *DirectConnectManager) UpdateActivity(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[id]; exists {
		session.LastActivity = time.Now()
	}
}

func (m *DirectConnectManager) RemoveSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, id)
}

func (m *DirectConnectManager) ListSessions() []*DirectConnectSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DirectConnectSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

func (m *DirectConnectManager) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/sessions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			sessions := m.ListSessions()
			json.NewEncoder(w).Encode(sessions)
		case "POST":
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
	})

	mux.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/sessions/"):]
		session, exists := m.GetSession(id)
		if !exists {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(session)
	})

	m.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", m.port),
		Handler: mux,
	}

	go m.server.ListenAndServe()
	return nil
}

func (m *DirectConnectManager) Stop(ctx context.Context) error {
	if m.server != nil {
		return m.server.Shutdown(ctx)
	}
	return nil
}

func (m *DirectConnectManager) GetPort() int {
	return m.port
}

func (m *DirectConnectManager) CleanupStaleSessions(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, session := range m.sessions {
		if now.Sub(session.LastActivity) > maxAge {
			delete(m.sessions, id)
		}
	}
}
