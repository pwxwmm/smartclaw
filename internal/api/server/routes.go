package server

import (
	"net/http"
	"strings"

	"github.com/instructkr/smartclaw/internal/observability"
)

func (s *APIServer) registerRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	w := s.wrapHandler
	rl := s.rl.Middleware

	mux.HandleFunc("/health", corsMiddleware(s.handleHealth))
	mux.HandleFunc("/ws", s.handleWebSocket)

	mux.HandleFunc("/api/auth/login", corsMiddleware(s.handleAuthLogin))
	mux.HandleFunc("/api/auth/status", corsMiddleware(s.handleAuthStatus))

	mux.HandleFunc("/api/chat", w(rl(s.handleChat)))
	mux.HandleFunc("/api/chat/stream", w(rl(s.handleChatStream)))

	mux.HandleFunc("/api/sessions", w(rl(s.handleSessions)))
	mux.HandleFunc("/api/sessions/search", w(rl(s.handleSessionSearch)))
	mux.HandleFunc("/api/sessions/", w(rl(s.handleSessionRoutes)))

	mux.HandleFunc("/api/config", w(rl(s.handleConfig)))

	mux.HandleFunc("/api/files", w(rl(s.handleFileTree)))
	mux.HandleFunc("/api/file", w(rl(s.handleFileContent)))
	mux.HandleFunc("/api/upload", w(rl(s.handleFileUpload)))

	mux.HandleFunc("/api/stats", w(rl(s.handleStats)))
	mux.HandleFunc("/api/telemetry", w(rl(s.handleTelemetry)))
	mux.HandleFunc("/api/telemetry/frontend", w(rl(s.handleFrontendTelemetry)))

	mux.HandleFunc("/api/skills", w(rl(s.handleSkills)))
	mux.HandleFunc("/api/skills/health", w(rl(s.handleSkillHealth)))
	mux.HandleFunc("/api/skills/trending", w(rl(s.handleSkillTrending)))
	mux.HandleFunc("/api/skills/marketplace/search", w(rl(s.handleMarketplaceSearch)))
	mux.HandleFunc("/api/skills/marketplace/install", w(rl(s.handleMarketplaceInstall)))
	mux.HandleFunc("/api/skills/marketplace/publish", w(rl(s.handleMarketplacePublish)))
	mux.HandleFunc("/api/skills/marketplace/featured", w(rl(s.handleMarketplaceFeatured)))
	mux.HandleFunc("/api/skills/marketplace/categories", w(rl(s.handleMarketplaceCategories)))
	mux.HandleFunc("/api/skills/", w(rl(s.handleSkillRoutes)))

	mux.HandleFunc("/api/memory", w(rl(s.handleMemory)))
	mux.HandleFunc("/api/memory/search", w(rl(s.handleMemorySearch)))
	mux.HandleFunc("/api/memory/update", w(rl(s.handleMemoryUpdate)))
	mux.HandleFunc("/api/memory/observations", w(rl(s.handleMemoryObservations)))

	mux.HandleFunc("/api/agents", w(rl(s.handleAgents)))
	mux.HandleFunc("/api/agents/switch", w(rl(s.handleAgentSwitch)))

	mux.HandleFunc("/api/mcp", w(rl(s.handleMCPServers)))
	mux.HandleFunc("/api/mcp/catalog", w(rl(s.handleMCPCatalog)))

	mux.HandleFunc("/api/cron/tasks", w(rl(s.handleCronTasks)))
	mux.HandleFunc("/api/cron/tasks/", w(rl(s.handleCronTaskRoutes)))

	mux.HandleFunc("/api/git-status", w(rl(s.handleGitStatus)))
	mux.HandleFunc("/api/search", w(rl(s.handleSearch)))
	mux.HandleFunc("/api/search/semantic", w(rl(s.handleSemanticSearch)))
	mux.HandleFunc("/api/search/suggest", w(rl(s.handleKnowledgeSuggest)))

	mux.HandleFunc("/api/cost/dashboard", w(rl(s.handleCostDashboard)))
	mux.HandleFunc("/api/cost/history", w(rl(s.handleCostHistory)))
	mux.HandleFunc("/api/cost/forecast", w(rl(s.handleCostForecast)))
	mux.HandleFunc("/api/cost/estimate", w(rl(s.handleCostEstimate)))
	mux.HandleFunc("/api/cost/projects", w(rl(s.handleCostProjects)))
	mux.HandleFunc("/api/cost/report", w(rl(s.handleCostReport)))

	mux.HandleFunc("/api/workflows", w(rl(s.handleWorkflows)))
	mux.HandleFunc("/api/workflows/", w(rl(s.handleWorkflowRoutes)))
	mux.HandleFunc("/api/workflow-tools", w(rl(s.handleWorkflowTools)))

	mux.HandleFunc("/api/onboarding/status", w(rl(s.handleOnboardingStatus)))
	mux.HandleFunc("/api/onboarding/start", w(rl(s.handleOnboardingStart)))
	mux.HandleFunc("/api/onboarding/step", w(rl(s.handleOnboardingStep)))

	mux.HandleFunc("/api/profile", w(rl(s.handleProfile)))
	mux.HandleFunc("/api/profile/style", w(rl(s.handleProfileStyle)))
	mux.HandleFunc("/api/profile/observations", w(rl(s.handleProfileObservations)))
	mux.HandleFunc("/api/profile/observations/delete-all", w(rl(s.handleProfileObservationsDeleteAll)))
	mux.HandleFunc("/api/profile/", w(rl(s.handleProfileRoutes)))

	mux.HandleFunc("/api/rooms", w(rl(s.handleRooms)))
	mux.HandleFunc("/api/rooms/", w(rl(s.handleRoomRoutes)))

	mux.Handle("/metrics", observability.PrometheusHandler())

	mux.HandleFunc("/", s.serveIndex)

	return mux
}

func (s *APIServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"name":    "smartclaw-api",
		"version": "1.0.0",
		"status":  "running",
	})
}

func (s *APIServer) handleSkillRoutes(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/skills/")
	if strings.HasSuffix(name, "/improve") {
		skillName := strings.TrimSuffix(name, "/improve")
		s.handleSkillImprove(w, r, skillName)
		return
	}
	if strings.HasSuffix(name, "/score") {
		skillName := strings.TrimSuffix(name, "/score")
		s.handleSkillScore(w, r, skillName)
		return
	}
	s.handleSkillDetail(w, r, name)
}

func (s *APIServer) handleSessionRoutes(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session id required"})
		return
	}
	s.handleSessionGet(w, r, id)
}

func (s *APIServer) handleCronTaskRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/toggle"):
		s.handleCronTaskToggle(w, r)
	case strings.HasSuffix(path, "/run"):
		s.handleCronTaskRun(w, r)
	default:
		s.handleCronTaskDetail(w, r)
	}
}
