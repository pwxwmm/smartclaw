package server

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/session"
	"github.com/instructkr/smartclaw/internal/skills"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/tools"
	"github.com/instructkr/smartclaw/internal/wiki"
)

const maxFileServeSize = 50 * 1024 * 1024

type APIHandler struct {
	WorkDir     string
	APIClient   *api.Client
	SessMgr     *session.Manager
	DataStore   *store.Store
	MemMgr      *memory.MemoryManager
	WikiClient  *wiki.WikiClient
	MCPRegistry *mcp.MCPServerRegistry
	CostGuard   interface {
		CalculateCost(model string, inputTokens, outputTokens int) (float64, costguard.CostBreakdown)
	}
	ShowThinking bool
}

func (h *APIHandler) handleAuthLogin(w http.ResponseWriter, r *http.Request, authMgr *AuthManager) {
	if r.Method != http.MethodPost {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	token, err := authMgr.Login(req.APIKey)
	if err != nil {
		WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "smartclaw-token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (h *APIHandler) handleAuthStatus(w http.ResponseWriter, r *http.Request, authMgr *AuthManager, noAuth bool) {
	authenticated := noAuth || authMgr == nil || !authMgr.IsAuthRequired()

	if !authenticated {
		token := extractToken(r)
		authenticated = validateAccessToken(token, authMgr)
	}

	WriteJSON(w, http.StatusOK, map[string]bool{"authenticated": authenticated})
}

func (h *APIHandler) handleFileTree(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}

	fullPath := filepath.Join(h.WorkDir, path)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(h.WorkDir)) {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	tree, err := BuildFileTree(fullPath, 3)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, tree)
}

func (h *APIHandler) handleFileContent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
		return
	}

	fullPath := filepath.Join(h.WorkDir, path)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(h.WorkDir)) {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if info.Size() > maxFileServeSize {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxFileServeSize)})
		return
	}

	f, err := os.Open(fullPath)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxFileServeSize))
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"path":    path,
		"content": string(data),
	})
}

func (h *APIHandler) handleGitStatusAPI(w http.ResponseWriter, r *http.Request) {
	statusMap, err := GetGitStatus(h.WorkDir)
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]string{})
		return
	}
	WriteJSON(w, http.StatusOK, statusMap)
}

func (h *APIHandler) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large (max 50MB)"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "file field required"})
		return
	}
	defer file.Close()

	relPath := r.FormValue("path")
	if relPath == "" {
		relPath = header.Filename
	}

	fullPath := filepath.Join(h.WorkDir, relPath)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(h.WorkDir)) {
		WriteJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create directory"})
		return
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create file"})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to write file"})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"path":    relPath,
		"size":    written,
	})
}

func (h *APIHandler) handleSessions(w http.ResponseWriter, r *http.Request) {
	if h.DataStore != nil {
		sessions, err := h.DataStore.ListAllSessions(50)
		if err != nil {
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		WriteJSON(w, http.StatusOK, sessions)
		return
	}

	if h.SessMgr == nil {
		WriteJSON(w, http.StatusOK, []any{})
		return
	}

	sessions, err := h.SessMgr.List()
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	WriteJSON(w, http.StatusOK, sessions)
}

func (h *APIHandler) handleSessionSearchAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		WriteJSON(w, http.StatusOK, []any{})
		return
	}
	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	limit = clampLimit(limit, 10)

	if h.MemMgr == nil {
		WriteJSON(w, http.StatusOK, []any{})
		return
	}

	fragments, err := h.MemMgr.Search(r.Context(), query, limit)
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
	WriteJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".smartclaw", "config.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			WriteJSON(w, http.StatusOK, map[string]any{})
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
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		homeDir, _ := os.UserHomeDir()
		configDir := filepath.Join(homeDir, ".smartclaw")
		os.MkdirAll(configDir, 0755)
		configPath := filepath.Join(configDir, "config.json")
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to marshal config"})
			return
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
			return
		}

		h.reloadAPIClient(config)

		WriteJSON(w, http.StatusOK, map[string]string{"status": "saved"})
		return
	}

	WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func (h *APIHandler) reloadAPIClient(config map[string]any) {
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
	h.APIClient = newClient
	log.Printf("Provider config reloaded: model=%s base_url=%s openai=%v", model, baseURL, openai)
}

func (h *APIHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	model := "sre-model"
	if h.APIClient != nil {
		model = h.APIClient.Model
	}
	WriteJSON(w, http.StatusOK, StatsResponse{
		TokensLimit: 200000,
		Model:       model,
	})
}

func (h *APIHandler) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	snapshot := observability.DefaultMetrics.Snapshot()

	response := map[string]any{
		"query_count":         snapshot.QueryCount,
		"query_total_time_ms": snapshot.QueryTotalTime.Milliseconds(),
		"cache_hits":          snapshot.CacheHits,
		"cache_misses":        snapshot.CacheMisses,
		"cache_hit_rate":      CacheHitRate(snapshot.CacheHits, snapshot.CacheMisses),
		"total_input_tokens":  snapshot.TotalInputTokens,
		"total_output_tokens": snapshot.TotalOutputTokens,
		"total_cache_read":    snapshot.TotalCacheRead,
		"total_cache_create":  snapshot.TotalCacheCreate,
		"estimated_cost_usd":  EstimateCost(snapshot),
		"tool_executions":     snapshot.ToolExecutions,
		"memory_layer_sizes":  snapshot.MemoryLayerSizes,
		"model_query_counts":  snapshot.ModelQueryCounts,
		"timestamp":           time.Now().Format(time.RFC3339),
	}

	WriteJSON(w, http.StatusOK, response)
}

func (h *APIHandler) handleFrontendTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	io.Copy(io.Discard, r.Body)
	r.Body.Close()
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *APIHandler) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		sm := skills.GetSkillManager()
		if sm == nil {
			WriteJSON(w, http.StatusOK, []any{})
			return
		}
		skillList := sm.List()
		WriteJSON(w, http.StatusOK, skillList)
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
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		if req.Name == "" || req.Description == "" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "name and description are required"})
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
			WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill manager not available"})
			return
		}

		if err := sm.CreateSkill(schema, req.Body); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"success": true,
			"name":    schema.Name,
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *APIHandler) handleSkillDetail(w http.ResponseWriter, r *http.Request, skillName string) {
	sm := skills.GetSkillManager()
	if sm == nil {
		http.Error(w, "Skill manager not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		skill := sm.Get(skillName)
		if skill == nil {
			http.Error(w, "Skill not found", http.StatusNotFound)
			return
		}
		WriteJSON(w, http.StatusOK, skill)
	case http.MethodDelete:
		sm.Disable(skillName)
		WriteJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
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
			sm.Enable(skillName)
			WriteJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
		case "disable":
			sm.Disable(skillName)
			WriteJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
		default:
			http.Error(w, "Invalid action", http.StatusBadRequest)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleMemoryAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.MemMgr == nil {
		WriteJSON(w, http.StatusOK, map[string]any{"error": "Memory manager not available"})
		return
	}
	pm := h.MemMgr.GetPromptMemory()

	layers := map[string]any{
		"memory_content": pm.GetMemoryContent(),
		"user_content":   pm.GetUserContent(),
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"layers": layers,
		"budget": h.MemMgr.GetBudget(),
	})
}

func (h *APIHandler) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
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
	if h.MemMgr == nil {
		WriteJSON(w, http.StatusOK, []any{})
		return
	}
	results, err := h.MemMgr.Search(r.Context(), req.Query, req.Limit)
	if err != nil {
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJSON(w, http.StatusOK, results)
}

func (h *APIHandler) handleMemoryUpdateAPI(w http.ResponseWriter, r *http.Request) {
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
	if h.MemMgr == nil {
		WriteJSON(w, http.StatusOK, map[string]any{"error": "Memory manager not available"})
		return
	}
	pm := h.MemMgr.GetPromptMemory()
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
	WriteJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"file":         req.File,
		"memory_chars": len(pm.GetMemoryContent()),
		"user_chars":   len(pm.GetUserContent()),
	})
}

func (h *APIHandler) handleMemoryObservationsAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
		if h.DataStore == nil {
			WriteJSON(w, http.StatusOK, []any{})
			return
		}
		rows, err := h.DataStore.DB().Query(
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
		if observations == nil {
			observations = []map[string]any{}
		}
		WriteJSON(w, http.StatusOK, observations)

	case http.MethodDelete:
		var req struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if req.ID <= 0 {
			http.Error(w, "Observation id is required", http.StatusBadRequest)
			return
		}
		if h.DataStore == nil {
			http.Error(w, "Store not available", http.StatusServiceUnavailable)
			return
		}
		result, err := h.DataStore.DB().Exec(`DELETE FROM user_observations WHERE id = ?`, req.ID)
		if err != nil {
			http.Error(w, "Failed to delete observation", http.StatusInternalServerError)
			return
		}
		affected, _ := result.RowsAffected()
		WriteJSON(w, http.StatusOK, map[string]any{"success": true, "affected": affected})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleWikiAPI(w http.ResponseWriter, r *http.Request) {
	if h.WikiClient == nil || !h.WikiClient.IsEnabled() {
		WriteJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"message": "Wiki not configured. Set wiki.base_url in config.",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		pages, err := h.WikiClient.ListPages(r.Context(), 50)
		if err != nil {
			http.Error(w, "Wiki list failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
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
		result, err := h.WikiClient.Search(r.Context(), req.Query, req.Limit)
		if err != nil {
			http.Error(w, "Wiki search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		WriteJSON(w, http.StatusOK, result)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleWikiPageAPI(w http.ResponseWriter, r *http.Request) {
	if h.WikiClient == nil || !h.WikiClient.IsEnabled() {
		WriteJSON(w, http.StatusOK, map[string]any{
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
	page, err := h.WikiClient.GetPage(r.Context(), pageID)
	if err != nil {
		http.Error(w, "Wiki get page failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"enabled": true,
		"page":    page,
	})
}

func (h *APIHandler) handleAgentsAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result, err := tools.Execute(r.Context(), "agent", map[string]any{"operation": "list"})
	if err != nil {
		WriteJSON(w, http.StatusOK, map[string]any{"agents": []any{}, "count": 0})
		return
	}
	WriteJSON(w, http.StatusOK, result)
}

func (h *APIHandler) handleMCPServersAPI(w http.ResponseWriter, r *http.Request) {
	if h.MCPRegistry == nil {
		WriteJSON(w, http.StatusOK, []any{})
		return
	}

	switch r.Method {
	case http.MethodGet:
		servers := h.MCPRegistry.ListServers()
		WriteJSON(w, http.StatusOK, servers)
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
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Name == "" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
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
		if err := h.MCPRegistry.AddServer(config); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		WriteJSON(w, http.StatusCreated, map[string]any{
			"success": true,
			"name":    req.Name,
		})
	case http.MethodDelete:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Name == "" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
			return
		}
		if err := h.MCPRegistry.RemoveServer(req.Name); err != nil {
			WriteJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"name":    req.Name,
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *APIHandler) handleMCPCatalogAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	catalog := GetCatalogWithInstalledStatus(h.MCPRegistry)
	WriteJSON(w, http.StatusOK, catalog)
}

func (h *APIHandler) handleChatSearchAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		WriteJSON(w, http.StatusOK, []any{})
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	limit = clampLimit(limit, 20)

	codeOnly := r.URL.Query().Get("code") == "true"

	userID := "default"
	if h.DataStore != nil {
		userID = ""
	}

	results, err := SearchMessages(h.DataStore, query, userID, limit, codeOnly)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if results == nil {
		results = []ChatSearchResult{}
	}
	WriteJSON(w, http.StatusOK, results)
}

func (h *APIHandler) handleShareCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id required"})
		return
	}
	shareID := generateShareID()
	if h.DataStore != nil {
		db := h.DataStore.DB()
		if db != nil {
			_, err := db.Exec(`CREATE TABLE IF NOT EXISTS shared_sessions (share_id TEXT PRIMARY KEY, session_id TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, view_count INTEGER DEFAULT 0)`)
			if err == nil {
				db.Exec(`INSERT INTO shared_sessions (share_id, session_id) VALUES (?, ?)`, shareID, req.SessionID)
			}
		}
	}
	url := fmt.Sprintf("/share/%s", shareID)
	WriteJSON(w, http.StatusOK, map[string]string{"share_id": shareID, "url": url})
}

func (h *APIHandler) handleChatExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "markdown"
	}

	var messages []map[string]any
	if h.DataStore != nil {
		db := h.DataStore.DB()
		if db != nil {
			rows, err := db.Query(`SELECT role, content, created_at FROM session_messages WHERE session_id = ? ORDER BY created_at`, sessionID)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var role, content, ts string
					if rows.Scan(&role, &content, &ts) == nil {
						messages = append(messages, map[string]any{"role": role, "content": content, "timestamp": ts})
					}
				}
			}
		}
	}

	if format == "markdown" {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=conversation.md")
		for _, m := range messages {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			fmt.Fprintf(w, "## %s\n\n%s\n\n---\n\n", role, content)
		}
		return
	}
	WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported format"})
}

func (h *APIHandler) handleTemplatesAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		result := make([]map[string]any, 0, len(PresetTemplates))
		result = append(result, PresetTemplates...)

		if h.DataStore != nil {
			custom, err := LoadCustomTemplates(h.DataStore)
			if err == nil {
				result = append(result, custom...)
			}
		}

		WriteJSON(w, http.StatusOK, result)

	case http.MethodPost:
		var req struct {
			ID          string   `json:"id"`
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Category    string   `json:"category"`
			Content     string   `json:"content"`
			Variables   []string `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		if req.Name == "" || req.Content == "" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "name and content are required"})
			return
		}

		if req.ID == "" {
			req.ID = "custom-" + fmt.Sprintf("%d", time.Now().UnixNano())
		}
		if req.Category == "" {
			req.Category = "Custom"
		}
		if len(req.Variables) == 0 {
			req.Variables = ExtractVariables(req.Content)
		}

		if h.DataStore == nil {
			WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		variablesJSON, _ := json.Marshal(req.Variables)
		_, err := h.DataStore.DB().Exec(
			`INSERT OR REPLACE INTO prompt_templates (id, user_id, name, description, category, content, variables, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			req.ID, "default", req.Name, req.Description, req.Category, req.Content, string(variablesJSON),
		)
		if err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"success": true,
			"id":      req.ID,
			"name":    req.Name,
		})

	case http.MethodDelete:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.ID == "" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}
		if len(req.ID) >= 7 && req.ID[:7] == "preset-" {
			WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot delete preset templates"})
			return
		}

		if h.DataStore == nil {
			WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		result, err := h.DataStore.DB().Exec(`DELETE FROM prompt_templates WHERE id=? AND user_id=?`, req.ID, "default")
		if err != nil {
			WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		affected, _ := result.RowsAffected()
		WriteJSON(w, http.StatusOK, map[string]any{"success": true, "affected": affected})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func generateShareID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
