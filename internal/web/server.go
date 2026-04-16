package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"

	"github.com/instructkr/smartclaw/internal/adapters"
	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/tools"
)

type rateLimiter struct {
	visitors map[string]*visitorInfo
	mu       sync.Mutex
}

type visitorInfo struct {
	count    int
	lastSeen time.Time
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{
		visitors: make(map[string]*visitorInfo),
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := strings.Split(r.RemoteAddr, ":")[0]
		rl.mu.Lock()
		v, ok := rl.visitors[ip]
		if !ok {
			v = &visitorInfo{}
			rl.visitors[ip] = v
		}
		v.count++
		v.lastSeen = time.Now()
		if v.count > 100 {
			rl.mu.Unlock()
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		rl.mu.Unlock()
		next(w, r)
	}
}

func (rl *rateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > time.Minute {
				delete(rl.visitors, ip)
			} else {
				v.count = 0
			}
		}
		rl.mu.Unlock()
	}
}

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
	authToken    string
}

func NewWebServer(port int, workDir string, apiClient *api.Client) *WebServer {
	hub := NewHub()
	handler := NewHandler(hub, workDir, apiClient)

	return &WebServer{
		port:      port,
		hub:       hub,
		handler:   handler,
		workDir:   workDir,
		apiClient: apiClient,
		authToken: os.Getenv("SMARTCLAW_AUTH_TOKEN"),
	}
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
	}

	tools.SetAllowedDirs([]string{s.workDir})

	adapters.InitInnovationPackages(mm, s.apiClient)

	otlpShutdown, err := observability.InitOTLP()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: OTLP init failed: %v\n", err)
	} else {
		s.otlpShutdown = otlpShutdown
	}
}

func (s *WebServer) Start() error {
	s.initSubsystems()

	s.handler.StartSessionCleanup(0)

	mux := http.NewServeMux()

	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("failed to setup static files: %w", err)
	}

	mux.HandleFunc("/", s.serveIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))
	mux.HandleFunc("/ws", s.handleWebSocket)
	rl := newRateLimiter()
	mux.HandleFunc("/api/files", rl.Middleware(s.authMiddleware(s.handleFileTree)))
	mux.HandleFunc("/api/file", rl.Middleware(s.authMiddleware(s.handleFileContent)))
	mux.HandleFunc("/api/sessions", rl.Middleware(s.authMiddleware(s.handleSessions)))
	mux.HandleFunc("/api/config", rl.Middleware(s.authMiddleware(s.handleConfig)))
	mux.HandleFunc("/api/stats", rl.Middleware(s.authMiddleware(s.handleStats)))
	mux.HandleFunc("/api/telemetry", rl.Middleware(s.authMiddleware(s.handleTelemetry)))
	mux.Handle("/metrics", observability.PrometheusHandler())

	s.server = &http.Server{
		Handler:      mux,
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
	token := r.URL.Query().Get("token")
	if s.authToken != "" && token != s.authToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
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

	go s.writePump(client, conn)
	go s.readPump(client, conn)
}

func (s *WebServer) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") == "websocket" {
			next(w, r)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			token = r.Header.Get("Authorization")
			if strings.HasPrefix(token, "Bearer ") {
				token = strings.TrimPrefix(token, "Bearer ")
			}
		}

		if s.authToken != "" && token != s.authToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
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

	fullPath := filepath.Join(s.workDir, path)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(s.workDir)) {
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

	fullPath := filepath.Join(s.workDir, path)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(s.workDir)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

	newClient := api.NewClientWithModel(apiKey, baseURL, model)
	newClient.IsOpenAI = openai
	s.apiClient = newClient
	s.handler.apiClient = newClient
	log.Printf("Provider config reloaded: model=%s base_url=%s openai=%v", model, baseURL, openai)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
