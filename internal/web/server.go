package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"nhooyr.io/websocket"

	"github.com/instructkr/smartclaw/internal/adapters"
	"github.com/instructkr/smartclaw/internal/agents"
	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/commands"
	"github.com/instructkr/smartclaw/internal/contextmgr"
	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/gateway"
	"github.com/instructkr/smartclaw/internal/lifecycle"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/permissions"
	"github.com/instructkr/smartclaw/internal/plugins"
	"github.com/instructkr/smartclaw/internal/playbook"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/serverauth"
	"github.com/instructkr/smartclaw/internal/skills"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/tools"
	"github.com/instructkr/smartclaw/internal/watchdog"
	"github.com/instructkr/smartclaw/internal/wiki"
)

//go:embed static/*
var staticFS embed.FS

const maxFileServeSize = 50 * 1024 * 1024 // 50MB

type WebServer struct {
	port         int
	hub          *Hub
	handler      *Handler
	workDir      string
	apiClient    *api.Client
	server       *http.Server
	otlpShutdown func(context.Context) error
	authManager  *AuthManager
	noAuth       bool
}

func NewWebServer(port int, workDir string, apiClient *api.Client, noAuth bool) (*WebServer, error) {
	hub := NewHub()
	handler := NewHandler(hub, workDir, apiClient)

	authManager, err := NewAuthManager()
	if err != nil {
		return nil, err
	}

	return &WebServer{
		port:        port,
		hub:         hub,
		handler:     handler,
		workDir:     workDir,
		apiClient:   apiClient,
		authManager: authManager,
		noAuth:      noAuth,
	}, nil
}

func (s *WebServer) initSubsystems() {
	if s.handler.dataStore != nil {
		home, _ := os.UserHomeDir()
		sessionsDir := filepath.Join(home, ".smartclaw", "sessions")
		if count, err := s.handler.dataStore.MigrateJSONSessions(sessionsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: session migration failed: %v\n", err)
		} else if count > 0 {
			fmt.Fprintf(os.Stderr, "Migrated %d sessions from JSON to SQLite\n", count)
		}
	}

	mm, err := memory.NewMemoryManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: memory manager init failed: %v\n", err)
	} else {
		tools.SetMemoryManagerForTools(mm)
		tools.SetIncidentMemory(mm.GetIncidentMemory())
		s.handler.memMgr = mm

		homeDir, _ := os.UserHomeDir()
		smartclawDir := filepath.Join(homeDir, ".smartclaw")
		agentsMDHierarchy := contextmgr.NewAgentsMDHierarchy(s.workDir, smartclawDir)
		if loadErr := agentsMDHierarchy.Load(context.Background()); loadErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: AGENTS.md hierarchy load failed: %v\n", loadErr)
		}
		mm.SetAgentsMDHierarchy(agentsMDHierarchy)

		contextAdapter := commands.NewAgentsMDContextAdapter(agentsMDHierarchy)
		commands.SetGlobalContextManager(contextAdapter)
	}

	tools.SetAllowedDirs([]string{s.workDir})

	adapters.InitInnovationPackages(mm, s.apiClient)
	if mm != nil && s.apiClient != nil {
		mm.SetLLMClient(s.apiClient)
		llmAdapter := learning.NewAPIClientAdapter(s.apiClient, "")
		tools.SetLLMClientForConversationRecall(llmAdapter)
	}
	if mm != nil {
		tools.SetStoreForConversationRecall(mm.GetStore())
	}
	lifecycle.Register(adapters.NewInnovationShutdown())

	otlpShutdown, err := observability.InitOTLP()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: OTLP init failed: %v\n", err)
	} else {
		s.otlpShutdown = otlpShutdown
	}

	mcpRegistry := mcp.NewMCPServerRegistry()
	s.handler.mcpRegistry = mcpRegistry
	autoStartMCPServers(mcpRegistry)

	profileRegistry := agents.NewProfileRegistry()
	agentPermMgr := permissions.NewAgentPermissionManager()

	for _, profile := range profileRegistry.List() {
		permSet := permissions.NewAgentPermissionSet(
			profile.AgentType,
			profile.Tools,
			profile.DisallowedTools,
			permissions.AgentPermissionMode(profile.PermissionMode),
		)
		agentPermMgr.Register(permSet)
	}

	tools.SetGlobalProfileRegistry(&webProfileRegistryAdapter{reg: profileRegistry})

	var gw *gateway.Gateway
	if s.handler.memMgr != nil && s.apiClient != nil {
		var gwSt *store.Store
		if s.handler.memMgr != nil {
			gwSt = s.handler.memMgr.GetStore()
		}
		if gwSt == nil {
			gwSt = s.handler.dataStore
		}

		llmAdapter := learning.NewAPIClientAdapter(s.apiClient, "")
		home, _ := os.UserHomeDir()
		skillsDir := filepath.Join(home, ".smartclaw", "skills")
		var promptMem learning.PromptMemoryWriter
		if s.handler.memMgr != nil {
			promptMem = s.handler.memMgr.GetPromptMemory()
		}
		gwLearningLoop := learning.NewLearningLoop(llmAdapter, promptMem, skillsDir)
		if gwSt != nil {
			gwTracker := learning.NewSkillTracker(gwSt)
			gwLearningLoop.SetSkillTracker(gwTracker)
		}

		gw = gateway.NewGateway(
			func() *runtime.QueryEngine {
				engine := runtime.NewQueryEngine(s.apiClient, runtime.QueryConfig{})
				engine.SetAgentPermissionManager(agentPermMgr)
				return engine
			},
			s.handler.memMgr,
			gwLearningLoop,
		)
		s.handler.gw = gw

		if s.handler.cronTrigger != nil {
			s.handler.cronTrigger.SetGateway(gw)
		}
	}

	tools.SetGlobalAgentSwitchFunc(func(cfg *tools.AgentSwitchConfig) error {
		slog.Info("web: agent switch requested", "agent", cfg.AgentType)

		rtlCfg := &runtime.AgentConfig{
			AgentType:       cfg.AgentType,
			SystemPrompt:    cfg.SystemPrompt,
			Model:           cfg.Model,
			AllowedTools:    cfg.AllowedTools,
			DisallowedTools: cfg.DisallowedTools,
			PermissionMode:  cfg.PermissionMode,
			MaxTurns:        cfg.MaxTurns,
		}

		if gw != nil {
			gw.SetCurrentAgentConfig(rtlCfg)
		}

		if cfg.SystemPrompt != "" && s.handler.prompt != nil {
			s.handler.prompt.SetPersona(cfg.SystemPrompt)
		}

		if cfg.Model != "" && s.handler.apiClient != nil {
			s.handler.apiClient.SetModel(cfg.Model)
		}

		return nil
	})

	if s.handler.cronTrigger != nil {
		commands.SetGlobalCronTrigger(&webCronTriggerAdapter{ct: s.handler.cronTrigger})
		commands.SetGlobalScheduleParser(&webScheduleParserAdapter{})
	}

	pluginRegistry := plugins.NewPluginRegistry("")
	if err := pluginRegistry.Initialize(context.Background()); err != nil {
		slog.Warn("Plugin registry init failed", "error", err)
	}
	pluginRegistry.RegisterToolsInRegistry(tools.GetRegistry())
	commands.SetGlobalPluginRegistry(pluginRegistry)

	if s.handler.wikiClient != nil && s.handler.wikiClient.IsEnabled() && s.handler.memMgr != nil {
		wikiProvider := wiki.NewWikiMemoryProvider(s.handler.wikiClient)
		s.handler.memMgr.RegisterProvider("wiki", wikiProvider)
	}

	if s.handler.dataStore != nil {
		s.handler.skillTracker = learning.NewSkillTracker(s.handler.dataStore)
		tools.GetTeamRegistry().SetStore(s.handler.dataStore)
		tools.GetTeamRegistry().LoadFromStore(context.Background())
	}
}

func autoStartMCPServers(registry *mcp.MCPServerRegistry) {
	servers := registry.ListServers()
	for _, config := range servers {
		if config.AutoStart {
			slog.Info("Auto-starting MCP server", "name", config.Name)
		}
	}
}

func (s *WebServer) Start() error {
	s.initSubsystems()
	s.initWorkflowService()

	s.handler.StartSessionCleanup(0)
	s.handler.initWarRoomIfNeeded()
	s.handler.InitAlertAutoTrigger()

	mux := http.NewServeMux()

	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("failed to setup static files: %w", err)
	}

	mux.HandleFunc("/", s.serveIndex)
	mux.HandleFunc("/static/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Service-Worker-Allowed", "/")
		data, err := staticFS.ReadFile("static/sw.js")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Write(data)
	})
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/auth/status", s.handleAuthStatus)
	rl := serverauth.NewRateLimiter()
	mux.HandleFunc("/api/files", rl.Middleware(s.authMiddleware(s.handleFileTree)))
	mux.HandleFunc("/api/file", rl.Middleware(s.authMiddleware(s.handleFileContent)))
	mux.HandleFunc("/api/git-status", rl.Middleware(s.authMiddleware(s.handleGitStatusAPI)))
	mux.HandleFunc("/api/upload", rl.Middleware(s.authMiddleware(s.handleFileUpload)))
	mux.HandleFunc("/api/sessions", rl.Middleware(s.authMiddleware(s.handleSessions)))
	mux.HandleFunc("/api/config", rl.Middleware(s.authMiddleware(s.handleConfig)))
	mux.HandleFunc("/api/stats", rl.Middleware(s.authMiddleware(s.handleStats)))
	mux.HandleFunc("/api/telemetry", rl.Middleware(s.authMiddleware(s.handleTelemetry)))
	mux.HandleFunc("/api/privacy/audit", rl.Middleware(s.authMiddleware(s.handlePrivacyAudit)))
	mux.HandleFunc("/api/telemetry/frontend", rl.Middleware(s.handleFrontendTelemetry))
	mux.HandleFunc("/api/skills", rl.Middleware(s.authMiddleware(s.handleSkills)))
	mux.HandleFunc("/api/skills/health", rl.Middleware(s.authMiddleware(s.handleSkillHealthAPI)))
	mux.HandleFunc("/api/skills/trending", rl.Middleware(s.authMiddleware(s.handleSkillTrendingAPI)))
	mux.HandleFunc("/api/skills/", rl.Middleware(s.authMiddleware(s.handleSkillDetail)))
	mux.HandleFunc("/api/memory", rl.Middleware(s.authMiddleware(s.handleMemoryAPI)))
	mux.HandleFunc("/api/memory/search", rl.Middleware(s.authMiddleware(s.handleMemorySearch)))
	mux.HandleFunc("/api/memory/update", rl.Middleware(s.authMiddleware(s.handleMemoryUpdateAPI)))
	mux.HandleFunc("/api/memory/observations", rl.Middleware(s.authMiddleware(s.handleMemoryObservationsAPI)))
	mux.HandleFunc("/api/sessions/search", rl.Middleware(s.authMiddleware(s.handleSessionSearchAPI)))
	mux.HandleFunc("/api/chat/search", rl.Middleware(s.authMiddleware(s.handleChatSearchAPI)))
	mux.HandleFunc("/api/wiki", rl.Middleware(s.authMiddleware(s.handleWikiAPI)))
	mux.HandleFunc("/api/wiki/page", rl.Middleware(s.authMiddleware(s.handleWikiPageAPI)))
	mux.HandleFunc("/api/agents", rl.Middleware(s.authMiddleware(s.handleAgentsAPI)))
	mux.HandleFunc("/api/mcp", rl.Middleware(s.authMiddleware(s.handleMCPServersAPI)))
	mux.HandleFunc("/api/mcp/catalog", rl.Middleware(s.authMiddleware(s.handleMCPCatalogAPI)))
	mux.HandleFunc("/api/mcp/status", rl.Middleware(s.authMiddleware(s.handleMCPStatusAPI)))
	mux.HandleFunc("/api/mcp/tools", rl.Middleware(s.authMiddleware(s.handleMCPToolsAPI)))
	mux.HandleFunc("/api/mcp/resources", rl.Middleware(s.authMiddleware(s.handleMCPResourcesAPI)))
	mux.HandleFunc("/api/templates", rl.Middleware(s.authMiddleware(s.handleTemplatesAPI)))
	mux.HandleFunc("/api/cron/tasks", rl.Middleware(s.authMiddleware(s.handleCronTasksAPI)))
	mux.HandleFunc("/api/cron/tasks/", rl.Middleware(s.authMiddleware(s.handleCronTaskRoutes)))
	mux.HandleFunc("/api/chat/share", rl.Middleware(s.authMiddleware(s.handleShareCreate)))
	mux.HandleFunc("/api/chat/export", rl.Middleware(s.authMiddleware(s.handleChatExport)))
	mux.HandleFunc("/api/chat/export-pdf", rl.Middleware(s.authMiddleware(s.handleExportPDF)))
	mux.HandleFunc("/api/workflows", rl.Middleware(s.authMiddleware(s.handleWorkflowsAPI)))
	mux.HandleFunc("/api/workflows/", rl.Middleware(s.authMiddleware(s.handleWorkflowRoutesAPI)))
	mux.HandleFunc("/api/workflow-tools", rl.Middleware(s.authMiddleware(s.handleWorkflowToolsAPI)))
	mux.HandleFunc("/api/cost/dashboard", rl.Middleware(s.authMiddleware(s.handleCostDashboard)))
	mux.HandleFunc("/api/cost/history", rl.Middleware(s.authMiddleware(s.handleCostHistory)))
	mux.HandleFunc("/api/cost/forecast", rl.Middleware(s.authMiddleware(s.handleCostForecast)))
	mux.HandleFunc("/api/cost/projects", rl.Middleware(s.authMiddleware(s.handleCostProjects)))
	mux.HandleFunc("/api/cost/report", rl.Middleware(s.authMiddleware(s.handleCostReport)))
	mux.HandleFunc("/api/onboarding/start", rl.Middleware(s.authMiddleware(s.handleOnboardingStart)))
	mux.HandleFunc("/api/onboarding/step", rl.Middleware(s.authMiddleware(s.handleOnboardingStep)))
	mux.HandleFunc("/api/onboarding/status", rl.Middleware(s.authMiddleware(s.handleOnboardingStatus)))
	mux.HandleFunc("/api/profile", rl.Middleware(s.authMiddleware(s.handleProfile)))
	mux.HandleFunc("/api/profile/observations", rl.Middleware(s.authMiddleware(s.handleProfileObservations)))
	mux.HandleFunc("/api/profile/style", rl.Middleware(s.authMiddleware(s.handleProfileStyle)))
	mux.HandleFunc("/api/profile/observations/", rl.Middleware(s.authMiddleware(s.handleProfileObservationRoutes)))
	mux.HandleFunc("/api/skills/marketplace/categories", rl.Middleware(s.authMiddleware(s.handleMarketplaceCategories)))
	mux.HandleFunc("/api/skills/marketplace/featured", rl.Middleware(s.authMiddleware(s.handleMarketplaceFeatured)))
	mux.HandleFunc("/api/skills/marketplace/search", rl.Middleware(s.authMiddleware(s.handleMarketplaceSearch)))
	mux.HandleFunc("/api/skills/marketplace/install", rl.Middleware(s.authMiddleware(s.handleMarketplaceInstall)))
	mux.HandleFunc("/api/skills/marketplace/publish", rl.Middleware(s.authMiddleware(s.handleMarketplacePublish)))
	mux.HandleFunc("/api/watchdog/status", rl.Middleware(s.authMiddleware(s.handleWatchdogStatus)))
	mux.HandleFunc("/api/warroom/auto-trigger-config", rl.Middleware(s.authMiddleware(s.handleWarRoomAutoTriggerConfig)))
	mux.HandleFunc("/share/", s.handleShareView)
	mux.Handle("/metrics", observability.PrometheusHandler())

	s.server = &http.Server{
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	addr := fmt.Sprintf(":%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.port = ln.Addr().(*net.TCPAddr).Port

	go s.hub.Run()

	fmt.Printf("SmartClaw WebUI running at http://localhost:%d\nAuthor: weimengmeng 天气晴 <1300042631@qq.com>\n", s.port)

	if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func (s *WebServer) Port() int {
	return s.port
}

func (s *WebServer) Stop() error {
	adapters.ShutdownInnovationPackages()

	if s.otlpShutdown != nil {
		s.otlpShutdown(context.Background())
	}
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func (s *WebServer) Close() error {
	return s.Stop()
}

func (s *WebServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Failed to load page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (s *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.noAuth && s.authManager.IsAuthRequired() {
		token := s.extractToken(r)
		if !s.validateAccessToken(token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	userID := r.URL.Query().Get("user")
	if userID == "" {
		userID = "default"
	}

	client := NewClient(s.hub, userID)
	s.hub.Register(client)

	go addToRecentProjects(s.workDir)

	projectName := filepath.Base(s.workDir)
	s.handler.sendToClient(client, WSResponse{
		Type:    "project_changed",
		Path:    s.workDir,
		Message: projectName,
	})

	go s.writePump(client, conn)
	go s.readPump(client, conn)
}

func (s *WebServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return serverauth.AuthMiddleware(next, s.authManager, s.noAuth)
}

func (s *WebServer) extractToken(r *http.Request) string {
	return serverauth.ExtractToken(r)
}

func (s *WebServer) validateAccessToken(token string) bool {
	return serverauth.ValidateAccessToken(token, s.authManager)
}

func (s *WebServer) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	token, err := s.authManager.Login(req.APIKey)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "smartclaw-token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *WebServer) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := s.noAuth || !s.authManager.IsAuthRequired()

	if !authenticated {
		token := s.extractToken(r)
		authenticated = s.validateAccessToken(token)
	}

	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": authenticated})
}

func (s *WebServer) readPump(client *Client, conn *websocket.Conn) {
	defer func() {
		s.hub.Unregister(client)
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	conn.SetReadLimit(65536)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		_, message, err := conn.Read(ctx)
		cancel()
		if err != nil {
			closeCode := websocket.CloseStatus(err)
			if closeCode != websocket.StatusGoingAway && closeCode != -1 {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		s.handler.HandleMessage(client, message)
	}
}

func (s *WebServer) writePump(client *Client, conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				conn.Close(websocket.StatusNormalClosure, "")
				return
			}

			buf := make([]byte, 0, len(message))
			buf = append(buf, message...)

			n := len(client.send)
			for i := 0; i < n; i++ {
				buf = append(buf, '\n')
				buf = append(buf, <-client.send...)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := conn.Write(ctx, websocket.MessageText, buf)
			cancel()
			if err != nil {
				return
			}

		case message, ok := <-client.sendImmediate:
			if !ok {
				conn.Close(websocket.StatusNormalClosure, "")
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := conn.Write(ctx, websocket.MessageText, message)
			cancel()
			if err != nil {
				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := conn.Ping(ctx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *WebServer) handleFileTree(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}

	workDir := s.handler.getWorkDir()

	fullPath := filepath.Join(workDir, path)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(workDir)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	tree, err := s.handler.buildFileTree(fullPath, 3)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, tree)
}

func (s *WebServer) handleFileContent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
		return
	}

	workDir := s.handler.getWorkDir()

	fullPath := filepath.Join(workDir, path)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(workDir)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}
	if info.Size() > maxFileServeSize {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxFileServeSize)})
		return
	}

	f, err := os.Open(fullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxFileServeSize))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"path":    path,
		"content": string(data),
	})
}

func (s *WebServer) handleGitStatusAPI(w http.ResponseWriter, r *http.Request) {
	statusMap, err := s.handler.getGitStatus()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, statusMap)
}

func (s *WebServer) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large (max 50MB)"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field required"})
		return
	}
	defer file.Close()

	relPath := r.FormValue("path")
	if relPath == "" {
		relPath = header.Filename
	}

	workDir := s.handler.getWorkDir()

	fullPath := filepath.Join(workDir, relPath)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(workDir)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create directory"})
		return
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create file"})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to write file"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"path":    relPath,
		"size":    written,
	})
}

func (s *WebServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	if s.handler.dataStore != nil {
		sessions, err := s.handler.dataStore.ListAllSessions(50)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, sessions)
		return
	}

	if s.handler.sessMgr == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	sessions, err := s.handler.sessMgr.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, sessions)
}

func (s *WebServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
		var config map[string]any
		if err := json.Unmarshal(data, &config); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}
		if key, ok := config["api_key"].(string); ok && key != "" {
			if len(key) > 7 {
				config["api_key"] = key[:3] + "***" + key[len(key)-4:]
			} else {
				config["api_key"] = "***"
			}
		}
		masked, err := json.Marshal(config)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(masked)
		return
	}

	if r.Method == http.MethodPost {
		var config map[string]any
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		homeDir, _ := os.UserHomeDir()
		configDir := filepath.Join(homeDir, ".smartclaw")
		os.MkdirAll(configDir, 0755)
		configPath := filepath.Join(configDir, "config.json")
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to marshal config"})
			return
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
			return
		}

		s.reloadAPIClient(config)

		writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
		return
	}

	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (s *WebServer) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.handler.GetStats())
}

func (s *WebServer) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	snapshot := observability.DefaultMetrics.Snapshot()

	response := map[string]any{
		"query_count":         snapshot.QueryCount,
		"query_total_time_ms": snapshot.QueryTotalTime.Milliseconds(),
		"cache_hits":          snapshot.CacheHits,
		"cache_misses":        snapshot.CacheMisses,
		"cache_hit_rate":      cacheHitRate(snapshot.CacheHits, snapshot.CacheMisses),
		"total_input_tokens":  snapshot.TotalInputTokens,
		"total_output_tokens": snapshot.TotalOutputTokens,
		"total_cache_read":    snapshot.TotalCacheRead,
		"total_cache_create":  snapshot.TotalCacheCreate,
		"estimated_cost_usd":  estimateCost(snapshot),
		"tool_executions":     snapshot.ToolExecutions,
		"memory_layer_sizes":  snapshot.MemoryLayerSizes,
		"model_query_counts":  snapshot.ModelQueryCounts,
		"timestamp":           time.Now().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *WebServer) handlePrivacyAudit(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	entries := observability.GetOutboundAuditLog(limit)

	writeJSON(w, http.StatusOK, map[string]any{
		"entries":   entries,
		"count":     len(entries),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (s *WebServer) handleFrontendTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	io.Copy(io.Discard, r.Body)
	r.Body.Close()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *WebServer) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		sm := skills.GetSkillManager()
		if sm == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		skillList := sm.List()
		writeJSON(w, http.StatusOK, skillList)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Version     string   `json:"version"`
			Tags        []string `json:"tags"`
			Tools       []string `json:"tools"`
			Triggers    []string `json:"triggers"`
			Body        string   `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		if req.Name == "" || req.Description == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and description are required"})
			return
		}

		if req.Version == "" {
			req.Version = "1.0"
		}

		schema := &skills.SkillSchema{
			Name:        req.Name,
			Description: req.Description,
			Version:     req.Version,
			Tags:        req.Tags,
			Tools:       req.Tools,
			Triggers:    req.Triggers,
		}

		sm := skills.GetSkillManager()
		if sm == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill manager not available"})
			return
		}

		if err := sm.CreateSkill(schema, req.Body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"success": true,
			"name":    schema.Name,
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *WebServer) handleSkillDetail(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/skills/")

	if strings.HasSuffix(name, "/improve") {
		skillName := strings.TrimSuffix(name, "/improve")
		if skillName == "" {
			http.Error(w, "Skill name required in path", http.StatusBadRequest)
			return
		}
		s.handleSkillImproveByName(w, r, skillName)
		return
	}

	if strings.HasSuffix(name, "/score") {
		skillName := strings.TrimSuffix(name, "/score")
		if skillName == "" {
			http.Error(w, "Skill name required in path", http.StatusBadRequest)
			return
		}
		s.handleSkillScoreAPI(w, r, skillName)
		return
	}

	sm := skills.GetSkillManager()
	if sm == nil {
		http.Error(w, "Skill manager not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		skill := sm.Get(name)
		if skill == nil {
			http.Error(w, "Skill not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, skill)
	case http.MethodDelete:
		sm.Disable(name)
		writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		switch req.Action {
		case "enable":
			sm.Enable(name)
			writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
		case "disable":
			sm.Disable(name)
			writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func cacheHitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func estimateCost(snapshot observability.MetricsSnapshot) float64 {
	model := ""
	if len(snapshot.ModelQueryCounts) > 0 {
		for m := range snapshot.ModelQueryCounts {
			model = m
			break
		}
	}
	cg := costguard.NewCostGuard(costguard.DefaultBudgetConfig())
	cost, _ := cg.CalculateCost(model, int(snapshot.TotalInputTokens), int(snapshot.TotalOutputTokens))
	return cost
}

func (s *WebServer) handleMemoryAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.handler.memMgr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Memory manager not available"})
		return
	}
	mm := s.handler.memMgr
	pm := mm.GetPromptMemory()

	layers := map[string]any{
		"memory_content": pm.GetMemoryContent(),
		"user_content":   pm.GetUserContent(),
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"layers": layers,
		"budget": mm.GetBudget(),
	})
}

func (s *WebServer) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if s.handler.memMgr == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	results, err := s.handler.memMgr.Search(r.Context(), req.Query, req.Limit)
	if err != nil {
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *WebServer) handleMemoryUpdateAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.File != "memory" && req.File != "user" {
		http.Error(w, "File must be 'memory' or 'user'", http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, "Content must not be empty", http.StatusBadRequest)
		return
	}
	if s.handler.memMgr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "Memory manager not available"})
		return
	}
	pm := s.handler.memMgr.GetPromptMemory()
	var updateErr error
	if req.File == "memory" {
		updateErr = pm.UpdateMemory(req.Content)
	} else {
		updateErr = pm.UpdateUserProfile(req.Content)
	}
	if updateErr != nil {
		http.Error(w, "Update failed: "+updateErr.Error(), http.StatusInternalServerError)
		return
	}
	pm.EnforceLimit()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"file":         req.File,
		"memory_chars": len(pm.GetMemoryContent()),
		"user_chars":   len(pm.GetUserContent()),
	})
}

func (s *WebServer) handleMemoryObservationsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
		if s.handler.dataStore == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		rows, err := s.handler.dataStore.DB().Query(
			`SELECT id, category, key, value, confidence, observed_at, session_id FROM user_observations ORDER BY observed_at DESC LIMIT 100`,
		)
		if err != nil {
			http.Error(w, "Failed to query observations", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var observations []map[string]any
		for rows.Next() {
			var id int
			var category, key, value, sessionID string
			var confidence float64
			var observedAt time.Time
			if err := rows.Scan(&id, &category, &key, &value, &confidence, &observedAt, &sessionID); err != nil {
				continue
			}
			observations = append(observations, map[string]any{
				"id":         id,
				"category":   category,
				"key":        key,
				"value":      value,
				"confidence": confidence,
				"observedAt": observedAt.Format(time.RFC3339),
				"sessionId":  sessionID,
			})
		}

		writeJSON(w, http.StatusOK, observations)
	case http.MethodDelete:
		if s.handler.dataStore != nil {
			s.handler.dataStore.DB().Exec("DELETE FROM user_observations")
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type webCronTriggerAdapter struct {
	ct *gateway.CronTrigger
}

func (a *webCronTriggerAdapter) ListTasks() ([]commands.CronTaskInfo, error) {
	tasks, err := a.ct.ListTasks()
	if err != nil {
		return nil, err
	}
	result := make([]commands.CronTaskInfo, 0, len(tasks))
	for _, t := range tasks {
		var lastRun, createdAt time.Time
		if t.LastRunAt != "" {
			lastRun, _ = time.Parse(time.RFC3339, t.LastRunAt)
		}
		if t.CreatedAt != "" {
			createdAt, _ = time.Parse(time.RFC3339, t.CreatedAt)
		}
		result = append(result, commands.CronTaskInfo{
			ID:          t.ID,
			Schedule:    t.Schedule,
			Instruction: t.Instruction,
			Enabled:     t.Enabled,
			LastRunAt:   lastRun,
			CreatedAt:   createdAt,
		})
	}
	return result, nil
}

func (a *webCronTriggerAdapter) CreateTask(schedule, instruction string) (string, error) {
	taskID := fmt.Sprintf("cron_%d", time.Now().UnixNano())
	return taskID, a.ct.ScheduleCron(taskID, "default", instruction, schedule, "web")
}

func (a *webCronTriggerAdapter) DeleteTask(id string) error {
	return a.ct.DeleteTask(id)
}

func (a *webCronTriggerAdapter) ToggleTask(id string) error {
	task, err := a.ct.GetTask(id)
	if err != nil {
		return err
	}
	if task.Enabled {
		return a.ct.DisableTask(id)
	}
	return a.ct.EnableTask(id)
}

func (a *webCronTriggerAdapter) RunTask(id string) error {
	task, err := a.ct.GetTask(id)
	if err != nil {
		return err
	}
	err = a.ct.ScheduleCron(task.ID, task.UserID, task.Instruction, task.Schedule, task.Platform)
	return err
}

type webScheduleParserAdapter struct{}

func (s *webScheduleParserAdapter) ParseNaturalLanguage(input string) (string, error) {
	return gateway.ParseNaturalLanguage(input)
}

type webProfileRegistryAdapter struct {
	reg *agents.ProfileRegistry
}

func (a *webProfileRegistryAdapter) Get(name string) (string, string, string, []string, []string, string, int, error) {
	p, err := a.reg.Get(name)
	if err != nil {
		return "", "", "", nil, nil, "", 0, err
	}
	return p.AgentType, p.SystemPrompt, p.Model, p.Tools, p.DisallowedTools, string(p.PermissionMode), p.MaxTurns, nil
}

func (a *webProfileRegistryAdapter) List() []tools.ProfileEntry {
	profiles := a.reg.List()
	entries := make([]tools.ProfileEntry, 0, len(profiles))
	for _, p := range profiles {
		entries = append(entries, tools.ProfileEntry{
			AgentType:       p.AgentType,
			WhenToUse:       p.WhenToUse,
			SystemPrompt:    p.SystemPrompt,
			Tools:           p.Tools,
			DisallowedTools: p.DisallowedTools,
			Model:           p.Model,
			PermissionMode:  string(p.PermissionMode),
			MaxTurns:        p.MaxTurns,
		})
	}
	return entries
}

func (s *WebServer) handleSessionSearchAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	summaryRequested := r.URL.Query().Get("summary") == "true"

	if s.handler.memMgr == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	if summaryRequested {
		llmClient := getSessionSearchLLMClient(s)
		result, err := s.handler.memMgr.SearchWithSummary(r.Context(), query, limit, llmClient)
		if err != nil {
			http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	fragments, err := s.handler.memMgr.Search(r.Context(), query, limit)
	if err != nil {
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type fragmentJSON struct {
		SessionID string  `json:"sessionId"`
		Timestamp string  `json:"timestamp"`
		Role      string  `json:"role"`
		Content   string  `json:"content"`
		Relevance float64 `json:"relevance"`
	}

	var result []fragmentJSON
	for _, f := range fragments {
		result = append(result, fragmentJSON{
			SessionID: f.SessionID,
			Timestamp: f.Timestamp.Format(time.RFC3339),
			Role:      f.Role,
			Content:   f.Content,
			Relevance: f.Relevance,
		})
	}
	if result == nil {
		result = []fragmentJSON{}
	}
	writeJSON(w, http.StatusOK, result)
}

func getSessionSearchLLMClient(s *WebServer) *learningLLMAdapter {
	if s.apiClient == nil {
		return nil
	}
	return &learningLLMAdapter{client: s.apiClient}
}

type learningLLMAdapter struct {
	client *api.Client
}

func (a *learningLLMAdapter) CreateMessage(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if a.client == nil {
		return "", fmt.Errorf("api client not configured")
	}
	adapter := learning.NewAPIClientAdapter(a.client, "")
	return adapter.CreateMessage(ctx, systemPrompt, userPrompt)
}

func (s *WebServer) handleWikiAPI(w http.ResponseWriter, r *http.Request) {
	if s.handler.wikiClient == nil || !s.handler.wikiClient.IsEnabled() {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"message": "Wiki not configured. Set wiki.base_url in config.",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		pages, err := s.handler.wikiClient.ListPages(r.Context(), 50)
		if err != nil {
			http.Error(w, "Wiki list failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": true,
			"pages":   pages,
		})
	case http.MethodPost:
		var req struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.Limit <= 0 {
			req.Limit = 5
		}
		result, err := s.handler.wikiClient.Search(r.Context(), req.Query, req.Limit)
		if err != nil {
			http.Error(w, "Wiki search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *WebServer) handleWikiPageAPI(w http.ResponseWriter, r *http.Request) {
	if s.handler.wikiClient == nil || !s.handler.wikiClient.IsEnabled() {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"message": "Wiki not configured. Set wiki.base_url in config.",
		})
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pageID := r.URL.Query().Get("id")
	if pageID == "" {
		http.Error(w, "id query parameter is required", http.StatusBadRequest)
		return
	}
	page, err := s.handler.wikiClient.GetPage(r.Context(), pageID)
	if err != nil {
		http.Error(w, "Wiki get page failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": true,
		"page":    page,
	})
}

func (s *WebServer) handleAgentsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result, err := tools.Execute(r.Context(), "agent", map[string]any{"operation": "list"})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"agents": []any{}, "count": 0})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *WebServer) reloadAPIClient(config map[string]any) {
	apiKey, _ := config["api_key"].(string)
	baseURL, _ := config["base_url"].(string)
	model, _ := config["model"].(string)

	if apiKey == "" || baseURL == "" || model == "" {
		return
	}

	openai := false
	if v, ok := config["openai"]; ok {
		openai, _ = v.(bool)
	}

	baseURL = sanitizeBaseURL(baseURL)
	if !openai && strings.Contains(config["base_url"].(string), "/chat/completions") {
		openai = true
	}

	if openai {
		newClient := api.NewOpenAICompatibleClient(apiKey, baseURL, model)
		s.apiClient = newClient
		s.handler.apiClient = newClient
	} else {
		newClient := api.NewClientWithModel(apiKey, baseURL, model)
		newClient.IsOpenAI = false
		s.apiClient = newClient
		s.handler.apiClient = newClient
	}
	log.Printf("Provider config reloaded: model=%s base_url=%s openai=%v", model, baseURL, openai)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *WebServer) handleCronTaskRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/toggle"):
		s.handleCronTaskToggleAPI(w, r)
	case strings.HasSuffix(path, "/run"):
		s.handleCronTaskRunAPI(w, r)
	default:
		s.handleCronTaskDetailAPI(w, r)
	}
}

func (s *WebServer) handleMCPServersAPI(w http.ResponseWriter, r *http.Request) {
	if s.handler.mcpRegistry == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	switch r.Method {
	case http.MethodGet:
		servers := s.handler.mcpRegistry.ListServers()
		writeJSON(w, http.StatusOK, servers)
	case http.MethodPost:
		var req struct {
			Name        string            `json:"name"`
			Type        string            `json:"type"`
			Command     string            `json:"command"`
			Args        []string          `json:"args"`
			URL         string            `json:"url"`
			Env         map[string]string `json:"env"`
			AutoStart   bool              `json:"auto_start"`
			Description string            `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		config := &mcp.ServerConfig{
			Name:        req.Name,
			Type:        req.Type,
			Command:     req.Command,
			Args:        req.Args,
			URL:         req.URL,
			Env:         req.Env,
			AutoStart:   req.AutoStart,
			Description: req.Description,
		}
		if err := s.handler.mcpRegistry.AddServer(config); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"success": true,
			"name":    req.Name,
		})
	case http.MethodDelete:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		if err := s.handler.mcpRegistry.RemoveServer(req.Name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"name":    req.Name,
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *WebServer) handleSkillHealthAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.handler.dataStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"skills":       []any{},
			"generated_at": time.Now().Format(time.RFC3339),
		})
		return
	}

	report := s.handler.getSkillHealthReport()
	writeJSON(w, http.StatusOK, report)
}

func (s *WebServer) handleSkillTrendingAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.handler.skillTracker == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	trending, err := s.handler.skillTracker.GetTrending(limit)
	if err != nil {
		http.Error(w, "Failed to get trending skills: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type trendingEntry struct {
		SkillID    string `json:"skill_id"`
		UsageCount int    `json:"usage_count"`
		LastUsed   string `json:"last_used,omitempty"`
	}

	entries := make([]trendingEntry, 0, len(trending))
	for _, t := range trending {
		entry := trendingEntry{
			SkillID:    t.SkillID,
			UsageCount: t.UsageCount,
		}
		if t.LastUsed != nil {
			entry.LastUsed = t.LastUsed.Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	writeJSON(w, http.StatusOK, entries)
}

func (s *WebServer) handleSkillScoreAPI(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.handler.skillTracker == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"skill_id":          name,
			"total_invocations": 0,
			"successes":         0,
			"failures":          0,
			"user_overrides":    0,
			"score":             0.5,
		})
		return
	}

	score, err := s.handler.skillTracker.GetEffectivenessScore(name)
	if err != nil {
		http.Error(w, "Failed to get skill score: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"skill_id":          score.SkillID,
		"total_invocations": score.TotalInvocations,
		"successes":         score.Successes,
		"failures":          score.Failures,
		"user_overrides":    score.UserOverrides,
		"score":             score.Score,
	})
}

func (s *WebServer) handleSkillImproveByName(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FailureMessages []string `json:"failure_messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	failures := req.FailureMessages
	if len(failures) == 0 {
		failures = []string{"Manual improvement triggered via API"}
	}

	if s.handler.dataStore == nil || s.handler.apiClient == nil {
		http.Error(w, "Store or API client not available", http.StatusServiceUnavailable)
		return
	}

	llmAdapter := learning.NewAPIClientAdapter(s.handler.apiClient, "")
	improver := learning.NewSkillImprover(llmAdapter)
	writer := learning.NewSkillWriter("")

	originalSkill, err := loadSkillForImprovement(name)
	if err != nil {
		http.Error(w, "Failed to load skill: "+err.Error(), http.StatusNotFound)
		return
	}

	improved, err := improver.Improve(r.Context(), name, failures, originalSkill)
	if err != nil {
		http.Error(w, "Skill improvement failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := improver.ApplyImprovement(writer, improved); err != nil {
		http.Error(w, "Failed to apply improvement: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":        true,
		"name":           improved.Name,
		"version":        improved.Version,
		"change_summary": improved.ChangeSummary,
	})
}

func (s *WebServer) initWorkflowService() {
	ws, err := NewWorkflowServiceHelper()
	if err != nil {
		slog.Warn("Workflow service init failed", "error", err)
		return
	}
	s.handler.workflowSvc = ws
}

func (s *WebServer) handleWorkflowsAPI(w http.ResponseWriter, r *http.Request) {
	ws := s.handler.workflowSvc
	switch r.Method {
	case http.MethodGet:
		if ws == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		workflows, err := ws.ListWorkflows()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, workflows)
	case http.MethodPost:
		var pb playbook.Playbook
		if err := json.NewDecoder(r.Body).Decode(&pb); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if pb.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workflow name is required"})
			return
		}
		if ws == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workflow service not available"})
			return
		}
		if err := ws.SaveWorkflow(&pb); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"success": true, "name": pb.Name})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *WebServer) handleWorkflowRoutesAPI(w http.ResponseWriter, r *http.Request) {
	ws := s.handler.workflowSvc
	name := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	name = strings.TrimSuffix(name, "/")
	if name == "" {
		s.handleWorkflowsAPI(w, r)
		return
	}

	if strings.HasSuffix(name, "/execute") {
		workflowName := strings.TrimSuffix(name, "/execute")
		s.handleWorkflowExecuteAPI(w, r, workflowName)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if ws == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workflow service not available"})
			return
		}
		pb, err := ws.GetWorkflow(name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, pb)
	case http.MethodDelete:
		if ws == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workflow service not available"})
			return
		}
		if err := ws.DeleteWorkflow(name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "name": name})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *WebServer) handleWorkflowExecuteAPI(w http.ResponseWriter, r *http.Request, name string) {
	ws := s.handler.workflowSvc
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if ws == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workflow service not available"})
		return
	}
	var req struct {
		Params map[string]string `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Params = map[string]string{}
	}
	execCtx, err := ws.ExecuteWorkflow(r.Context(), name, req.Params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type stepResultJSON struct {
		StepID   string `json:"step_id"`
		Success  bool   `json:"success"`
		Output   string `json:"output"`
		Duration string `json:"duration"`
		Error    string `json:"error,omitempty"`
	}
	results := make([]stepResultJSON, 0)
	for _, sr := range execCtx.StepResults {
		results = append(results, stepResultJSON{
			StepID:   sr.StepID,
			Success:  sr.Success,
			Output:   sr.Output,
			Duration: sr.Duration.String(),
			Error:    sr.Error,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  execCtx.Status,
		"results": results,
	})
}

func (s *WebServer) handleWorkflowToolsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	ws := s.handler.workflowSvc
	if ws == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, ws.GetAvailableTools())
}

func (s *WebServer) handleWatchdogStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	wd := watchdog.DefaultWatchdog()
	if wd == nil {
		writeJSON(w, http.StatusOK, watchdog.WatchdogStatus{
			Enabled:       false,
			ActiveWatches: []watchdog.ProcessWatch{},
			RecentErrors:  []watchdog.DetectedError{},
		})
		return
	}
	writeJSON(w, http.StatusOK, wd.GetStatus())
}
