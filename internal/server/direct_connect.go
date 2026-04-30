package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/adapters"
	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/lifecycle"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/tools"
	"github.com/instructkr/smartclaw/internal/wiki"
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
	sessions    map[string]*DirectConnectSession
	mu          sync.RWMutex
	port        int
	host        string
	server      *http.Server
	authToken   string
	noAuth      bool
	apiClient   *api.Client
	workDir     string
	handler     *APIHandler
	authMgr     *AuthManager
	otlpShutdown func(context.Context) error
}

func NewDirectConnectManager(port int, host string, authToken string, noAuth bool, apiClient *api.Client, workDir string) (*DirectConnectManager, error) {
	if authToken == "" && !noAuth {
		slog.Warn("direct connect server: no auth token provided, binding to localhost only")
	}

	handler := &APIHandler{
		WorkDir:     workDir,
		APIClient:   apiClient,
		ShowThinking: true,
	}

	authMgr, err := NewAuthManager()
	if err != nil {
		return nil, err
	}
	if authToken != "" {
		authMgr.SetAPIKey(authToken)
	}

	return &DirectConnectManager{
		sessions:  make(map[string]*DirectConnectSession),
		port:      port,
		host:      host,
		authToken: authToken,
		noAuth:    noAuth,
		apiClient: apiClient,
		workDir:   workDir,
		handler:   handler,
		authMgr:   authMgr,
	}, nil
}

func (m *DirectConnectManager) initSubsystems() {
	mm, err := memory.NewMemoryManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: memory manager init failed: %v\n", err)
	} else {
		tools.SetMemoryManagerForTools(mm)
		tools.SetIncidentMemory(mm.GetIncidentMemory())
		m.handler.MemMgr = mm
	}

	tools.SetAllowedDirs([]string{m.workDir})

	adapters.InitInnovationPackages(mm, m.apiClient)
	if mm != nil && m.apiClient != nil {
		mm.SetLLMClient(m.apiClient)
	}
	lifecycle.Register(adapters.NewInnovationShutdown())

	otlpShutdown, err := observability.InitOTLP()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: OTLP init failed: %v\n", err)
	} else {
		m.otlpShutdown = otlpShutdown
	}

	mcpRegistry := mcp.NewMCPServerRegistry()
	m.handler.MCPRegistry = mcpRegistry

	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			Wiki struct {
				BaseURL string `json:"base_url"`
			} `json:"wiki"`
		}
		if json.Unmarshal(data, &cfg) == nil && cfg.Wiki.BaseURL != "" {
			wc := wiki.NewWikiClient(wiki.WikiConfig{
				BaseURL: cfg.Wiki.BaseURL,
				Enabled: true,
			})
			m.handler.WikiClient = wc
			if mm != nil {
				wikiProvider := wiki.NewWikiMemoryProvider(wc)
				mm.RegisterProvider("wiki", wikiProvider)
			}
		}
	}
}

func (m *DirectConnectManager) legacyAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.authToken == "" {
			next(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "unauthorized: missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != m.authToken {
			http.Error(w, "unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func (m *DirectConnectManager) wrapHandler(next http.HandlerFunc) http.HandlerFunc {
	return corsMiddleware(authMiddleware(next, m.authMgr, m.noAuth))
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
	m.initSubsystems()

	mux := http.NewServeMux()


	mux.HandleFunc("/sessions", m.wrapHandler(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			sessions := m.ListSessions()
			json.NewEncoder(w).Encode(sessions)
		case http.MethodPost:
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
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/sessions/", m.wrapHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Path[len("/sessions/"):]
		session, exists := m.GetSession(id)
		if !exists {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(session)
	}))


	mux.HandleFunc("/api/auth/login", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		m.handler.handleAuthLogin(w, r, m.authMgr)
	}))
	mux.HandleFunc("/api/auth/status", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		m.handler.handleAuthStatus(w, r, m.authMgr, m.noAuth)
	}))

	rl := newRateLimiter()
	mux.HandleFunc("/api/files", m.wrapHandler(rl.Middleware(m.handler.handleFileTree)))
	mux.HandleFunc("/api/file", m.wrapHandler(rl.Middleware(m.handler.handleFileContent)))
	mux.HandleFunc("/api/git-status", m.wrapHandler(rl.Middleware(m.handler.handleGitStatusAPI)))
	mux.HandleFunc("/api/upload", m.wrapHandler(rl.Middleware(m.handler.handleFileUpload)))
	mux.HandleFunc("/api/sessions", m.wrapHandler(rl.Middleware(m.handler.handleSessions)))
	mux.HandleFunc("/api/sessions/search", m.wrapHandler(rl.Middleware(m.handler.handleSessionSearchAPI)))
	mux.HandleFunc("/api/config", m.wrapHandler(rl.Middleware(m.handler.handleConfig)))
	mux.HandleFunc("/api/stats", m.wrapHandler(rl.Middleware(m.handler.handleStats)))
	mux.HandleFunc("/api/telemetry", m.wrapHandler(rl.Middleware(m.handler.handleTelemetry)))
	mux.HandleFunc("/api/telemetry/frontend", corsMiddleware(rl.Middleware(m.handler.handleFrontendTelemetry)))
	mux.HandleFunc("/api/skills", m.wrapHandler(rl.Middleware(m.handler.handleSkills)))
	mux.HandleFunc("/api/skills/", m.wrapHandler(rl.Middleware(func(w http.ResponseWriter, r *http.Request) {
		skillName := strings.TrimPrefix(r.URL.Path, "/api/skills/")
		m.handler.handleSkillDetail(w, r, skillName)
	})))
	mux.HandleFunc("/api/memory", m.wrapHandler(rl.Middleware(m.handler.handleMemoryAPI)))
	mux.HandleFunc("/api/memory/search", m.wrapHandler(rl.Middleware(m.handler.handleMemorySearch)))
	mux.HandleFunc("/api/memory/update", m.wrapHandler(rl.Middleware(m.handler.handleMemoryUpdateAPI)))
	mux.HandleFunc("/api/memory/observations", m.wrapHandler(rl.Middleware(m.handler.handleMemoryObservationsAPI)))
	mux.HandleFunc("/api/wiki", m.wrapHandler(rl.Middleware(m.handler.handleWikiAPI)))
	mux.HandleFunc("/api/wiki/page", m.wrapHandler(rl.Middleware(m.handler.handleWikiPageAPI)))
	mux.HandleFunc("/api/agents", m.wrapHandler(rl.Middleware(m.handler.handleAgentsAPI)))
	mux.HandleFunc("/api/mcp", m.wrapHandler(rl.Middleware(m.handler.handleMCPServersAPI)))
	mux.HandleFunc("/api/mcp/catalog", m.wrapHandler(rl.Middleware(m.handler.handleMCPCatalogAPI)))
	mux.HandleFunc("/api/templates", m.wrapHandler(rl.Middleware(m.handler.handleTemplatesAPI)))
	mux.HandleFunc("/api/chat/share", m.wrapHandler(rl.Middleware(m.handler.handleShareCreate)))
	mux.HandleFunc("/api/chat/export", m.wrapHandler(rl.Middleware(m.handler.handleChatExport)))
	mux.HandleFunc("/api/chat/search", m.wrapHandler(rl.Middleware(m.handler.handleChatSearchAPI)))
	mux.Handle("/metrics", observability.PrometheusHandler())


	mux.HandleFunc("/health", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	addr := fmt.Sprintf("%s:%d", m.host, m.port)
	if m.authToken == "" && !m.noAuth && m.host == "" {
		addr = fmt.Sprintf("127.0.0.1:%d", m.port)
	}

	m.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}


	m.port = ln.Addr().(*net.TCPAddr).Port

	actualAddr := ln.Addr().String()
	fmt.Printf("SmartClaw API server running at http://%s\n", actualAddr)

	go func() {
		if err := m.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		}
	}()

	return nil
}

func (m *DirectConnectManager) Stop(ctx context.Context) error {
	if m.server != nil {
		return m.server.Shutdown(ctx)
	}
	return nil
}

func (m *DirectConnectManager) Close() error {
	adapters.ShutdownInnovationPackages()

	if m.otlpShutdown != nil {
		m.otlpShutdown(context.Background())
	}
	if m.server != nil {
		return m.server.Close()
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
