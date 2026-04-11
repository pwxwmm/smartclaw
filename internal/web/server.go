package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/observability"
)

//go:embed static/*
var staticFS embed.FS

type WebServer struct {
	port      int
	hub       *Hub
	handler   *Handler
	workDir   string
	apiClient *api.Client
	server    *http.Server
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
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
	}
}

func (s *WebServer) Start() error {
	mux := http.NewServeMux()

	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		return fmt.Errorf("failed to setup static files: %w", err)
	}

	mux.HandleFunc("/", s.serveIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticContent))))
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/api/files", s.handleFileTree)
	mux.HandleFunc("/api/file", s.handleFileContent)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/telemetry", s.handleTelemetry)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go s.hub.Run()

	fmt.Printf("SmartClaw WebUI running at http://localhost:%d\nAuthor: weimengmeng 天气晴 <1300042631@qq.com>\n", s.port)

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func (s *WebServer) Stop() error {
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := NewClient(s.hub)
	s.hub.Register(client)

	go s.writePump(client, conn)
	go s.readPump(client, conn)
}

func (s *WebServer) readPump(client *Client, conn *websocket.Conn) {
	defer func() {
		s.hub.Unregister(client)
		conn.Close()
	}()

	conn.SetReadLimit(65536)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
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
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(client.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
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

	data, err := os.ReadFile(fullPath)
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
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
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
	inputPrice := 0.000003
	outputPrice := 0.000015
	return float64(snapshot.TotalInputTokens)*inputPrice + float64(snapshot.TotalOutputTokens)*outputPrice
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
