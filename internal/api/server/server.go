package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/gateway"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/serverauth"
	"github.com/instructkr/smartclaw/internal/session"
	"github.com/instructkr/smartclaw/internal/skills"
	"github.com/instructkr/smartclaw/internal/store"

	"github.com/instructkr/smartclaw/internal/onboarding"
)

// APIServer is a unified REST + WebSocket server that uses Gateway as the
// primary message handler and delegates to subsystems for non-chat operations.
type APIServer struct {
	gw            *gateway.Gateway
	store         *store.Store
	memMgr        *memory.MemoryManager
	sessMgr       *session.Manager
	mcpRegistry   *mcp.MCPServerRegistry
	skillTracker  *learning.SkillTracker
	skillImprover *learning.SkillImprover
	skillWriter   *learning.SkillWriter
	skillRegistry *skills.Registry
	costGuard     *costguard.CostGuard
	onboardingMgr *onboarding.Manager
	workflowSvc   *WorkflowService
	userModelEngine *memory.UserModelingEngine
	hub           *Hub
	roomMgr       *RoomManager
	noAuth       bool
	addr         string

	httpServer *http.Server
	rl         *serverauth.RateLimiter
	auth       *serverauth.AuthManager
}

// NewAPIServer creates a new APIServer with the given dependencies.
func NewAPIServer(
	gw *gateway.Gateway,
	st *store.Store,
	memMgr *memory.MemoryManager,
	sessMgr *session.Manager,
	mcpRegistry *mcp.MCPServerRegistry,
	noAuth bool,
	addr string,
) (*APIServer, error) {
	auth, err := serverauth.NewAuthManager()
	if err != nil {
		return nil, err
	}

	return &APIServer{
		gw:            gw,
		store:         st,
		memMgr:        memMgr,
		sessMgr:       sessMgr,
		mcpRegistry:   mcpRegistry,
		onboardingMgr: onboarding.NewManager(st),
		hub:           NewHub(),
		roomMgr:       NewRoomManager(),
		noAuth:        noAuth,
		addr:          addr,
		rl:            serverauth.NewRateLimiter(),
		auth:          auth,
	}, nil
}

// Start creates the HTTP mux, registers routes, and starts listening.
// It blocks until the server exits or an error occurs.
func (s *APIServer) Start() error {
	mux := s.registerRoutes()

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second, // long for SSE streaming
		IdleTimeout:  60 * time.Second,
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("api server: listen %s: %w", s.addr, err)
	}

	go s.hub.Run()

	slog.Info("api server: listening", "addr", ln.Addr().String())

	if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *APIServer) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Close implements lifecycle.Closable for graceful shutdown registration.
func (s *APIServer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.Shutdown(ctx)
}

// SetAPIKey configures the API key used for authentication.
func (s *APIServer) SetAPIKey(key string) {
	s.auth.SetAPIKey(key)
}

// Hub returns the WebSocket hub for external access (e.g., broadcasting).
func (s *APIServer) Hub() *Hub {
	return s.hub
}

// WithCostGuard sets the CostGuard instance for cost analytics endpoints.
func (s *APIServer) WithCostGuard(cg *costguard.CostGuard) *APIServer {
	s.costGuard = cg
	return s
}

// WithSkillTracker sets the SkillTracker for health/trending/score endpoints.
func (s *APIServer) WithSkillTracker(st *learning.SkillTracker) *APIServer {
	s.skillTracker = st
	return s
}

func (s *APIServer) WithSkillRegistry(r *skills.Registry) *APIServer {
	s.skillRegistry = r
	return s
}

func (s *APIServer) WithSkillImprover(improver *learning.SkillImprover) *APIServer {
	s.skillImprover = improver
	return s
}

func (s *APIServer) WithSkillWriter(writer *learning.SkillWriter) *APIServer {
	s.skillWriter = writer
	return s
}

// WithWorkflowService sets the WorkflowService for workflow endpoints.
func (s *APIServer) WithWorkflowService(ws *WorkflowService) *APIServer {
	s.workflowSvc = ws
	return s
}

func (s *APIServer) WithUserModelEngine(ume *memory.UserModelingEngine) *APIServer {
	s.userModelEngine = ume
	return s
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
	for m := range snapshot.ModelQueryCounts {
		model = m
		break
	}
	cg := costguard.NewCostGuard(costguard.DefaultBudgetConfig())
	cost, _ := cg.CalculateCost(model, int(snapshot.TotalInputTokens), int(snapshot.TotalOutputTokens))
	return cost
}
