package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/gateway"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/onboarding"
	"github.com/instructkr/smartclaw/internal/playbook"
	"github.com/instructkr/smartclaw/internal/skills"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/tools"
)

const maxFileServeSize = 50 * 1024 * 1024

func (s *APIServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getConfig(w, r)
	case http.MethodPost:
		s.setConfig(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) getConfig(w http.ResponseWriter, r *http.Request) {
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
}

func (s *APIServer) setConfig(w http.ResponseWriter, r *http.Request) {
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
}

func (s *APIServer) handleFileTree(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}

	homeDir, _ := os.UserHomeDir()
	workDir := filepath.Join(homeDir, "projects")
	fullPath := filepath.Join(workDir, path)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(workDir)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	tree, err := buildFileTree(fullPath, 3)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, tree)
}

func (s *APIServer) handleFileContent(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "path required"})
		return
	}

	homeDir, _ := os.UserHomeDir()
	workDir := filepath.Join(homeDir, "projects")
	fullPath := filepath.Join(workDir, path)
	fullPath = filepath.Clean(fullPath)

	if !strings.HasPrefix(fullPath, filepath.Clean(workDir)) {
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

func (s *APIServer) handleFileUpload(w http.ResponseWriter, r *http.Request) {
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

	homeDir, _ := os.UserHomeDir()
	workDir := filepath.Join(homeDir, "projects")
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

func (s *APIServer) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	result, err := tools.Execute(r.Context(), "git_status", map[string]any{})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *APIServer) handleFrontendTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var payload map[string]any
	json.NewDecoder(r.Body).Decode(&payload)
	slog.Debug("frontend telemetry received", "metrics", payload)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *APIServer) handleSkills(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sm := skills.GetSkillManager()
		if sm == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, sm.List())
	case http.MethodPost:
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

		writeJSON(w, http.StatusCreated, map[string]any{"success": true, "name": schema.Name})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleSkillDetail(w http.ResponseWriter, r *http.Request, name string) {
	sm := skills.GetSkillManager()
	if sm == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill manager not available"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		skill := sm.Get(name)
		if skill == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "skill not found"})
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
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
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
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid action"})
		}
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleSkillHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.skillTracker == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"skills":       []any{},
			"generated_at": time.Now().Format(time.RFC3339),
			"healthy":      0,
			"degraded":     0,
			"failing":      0,
			"unused":       0,
		})
		return
	}

	report, err := s.skillTracker.GetHealthReport()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type healthEntry struct {
		SkillID          string  `json:"skill_id"`
		SuccessRate      float64 `json:"success_rate"`
		TotalInvocations int     `json:"total_invocations"`
		Trend            string  `json:"trend"`
		LastUsed         string  `json:"last_used,omitempty"`
		Health           string  `json:"health"`
		Recommendation   string  `json:"recommendation"`
	}

	entries := make([]healthEntry, 0, len(report.Skills))
	for _, sk := range report.Skills {
		entry := healthEntry{
			SkillID:          sk.SkillID,
			SuccessRate:      sk.SuccessRate,
			TotalInvocations: sk.TotalInvocations,
			Trend:            sk.Trend,
			Health:           string(sk.Health),
			Recommendation:   sk.Recommendation,
		}
		if sk.LastUsed != nil {
			entry.LastUsed = sk.LastUsed.Format(time.RFC3339)
		}
		entries = append(entries, entry)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"skills":       entries,
		"generated_at": report.GeneratedAt.Format(time.RFC3339),
		"healthy":      report.Healthy,
		"degraded":     report.Degraded,
		"failing":      report.Failing,
		"unused":       report.Unused,
	})
}

func (s *APIServer) handleSkillTrending(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.skillTracker == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	limit := parseIntDefault(r.URL.Query().Get("limit"), 10)
	trending, err := s.skillTracker.GetTrending(limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

func (s *APIServer) handleSkillScore(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.skillTracker == nil {
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

	score, err := s.skillTracker.GetEffectivenessScore(name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

func (s *APIServer) handleSkillImprove(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.skillTracker == nil || s.skillImprover == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"success": false,
			"name":    name,
			"message": "skill improvement not available (tracker or improver not configured)",
		})
		return
	}

	if !s.skillImprover.ShouldImprove(s.skillTracker, name) {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"name":    name,
			"message": "skill does not need improvement",
		})
		return
	}

	ctx := r.Context()

	var originalSkill *learning.ExtractedSkill
	if s.store != nil {
		record, err := s.store.GetSkill(ctx, name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read skill: " + err.Error()})
			return
		}
		if record != nil && record.Content != "" {
			originalSkill = learning.ParseExistingSkill(name, record.Content)
		}
	}
	if originalSkill == nil && s.skillWriter != nil {
		skillPath := s.skillWriter.GetSkillsDir() + "/" + name + "/SKILL.md"
		if content, err := readSkillContent(skillPath); err == nil {
			originalSkill = learning.ParseExistingSkill(name, content)
		}
	}
	if originalSkill == nil {
		originalSkill = &learning.ExtractedSkill{
			Name:        name,
			Description: "Skill (content unavailable)",
			Steps:       []string{"Execute the skill"},
			Tools:       []string{"bash"},
			Tags:        []string{"learned"},
		}
	}

	failures, err := s.skillTracker.GetRecentFailures(ctx, name, 10)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get failure history: " + err.Error()})
		return
	}

	improved, err := s.skillImprover.Improve(ctx, name, failures, originalSkill)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "improvement failed: " + err.Error()})
		return
	}

	if s.skillWriter != nil {
		if err := s.skillImprover.ApplyImprovement(s.skillWriter, improved); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to apply improvement: " + err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":        true,
		"name":           name,
		"version":        improved.Version,
		"change_summary": improved.ChangeSummary,
		"steps":          improved.Steps,
	})
}

func (s *APIServer) handleMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.memMgr == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "memory manager not available"})
		return
	}

	pm := s.memMgr.GetPromptMemory()
	layers := map[string]any{
		"memory_content": pm.GetMemoryContent(),
		"user_content":   pm.GetUserContent(),
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"layers": layers,
		"budget": s.memMgr.GetBudget(),
	})
}

func (s *APIServer) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if s.memMgr == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	results, err := s.memMgr.Search(r.Context(), req.Query, req.Limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *APIServer) handleMemoryUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		File    string `json:"file"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.File != "memory" && req.File != "user" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file must be 'memory' or 'user'"})
		return
	}
	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content must not be empty"})
		return
	}
	if s.memMgr == nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": "memory manager not available"})
		return
	}

	pm := s.memMgr.GetPromptMemory()
	var updateErr error
	if req.File == "memory" {
		updateErr = pm.UpdateMemory(req.Content)
	} else {
		updateErr = pm.UpdateUserProfile(req.Content)
	}
	if updateErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": updateErr.Error()})
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

func (s *APIServer) handleMemoryObservations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
		if s.store == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		rows, err := s.store.DB().Query(
			`SELECT id, category, key, value, confidence, observed_at, session_id FROM user_observations ORDER BY observed_at DESC LIMIT 100`,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		writeJSON(w, http.StatusOK, observations)

	case http.MethodDelete:
		var req struct {
			ID int `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.ID <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "observation id is required"})
			return
		}
		if s.store == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store not available"})
			return
		}
		result, err := s.store.DB().Exec(`DELETE FROM user_observations WHERE id = ?`, req.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		affected, _ := result.RowsAffected()
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "affected": affected})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	result, err := tools.Execute(r.Context(), "agent", map[string]any{"operation": "list"})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"agents": []any{}, "count": 0})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *APIServer) handleAgentSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		Agent string `json:"agent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Agent == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent is required"})
		return
	}
	result, err := tools.Execute(r.Context(), "agent", map[string]any{
		"operation":  "switch",
		"agent_type": req.Agent,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *APIServer) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	if s.mcpRegistry == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.mcpRegistry.ListServers())
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
		if err := s.mcpRegistry.AddServer(config); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"success": true, "name": req.Name})
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
		if err := s.mcpRegistry.RemoveServer(req.Name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "name": req.Name})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleMCPCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, mcpCatalog)
}

func (s *APIServer) handleCronTasks(w http.ResponseWriter, r *http.Request) {
	ct := s.getCronTrigger()
	switch r.Method {
	case http.MethodGet:
		s.listCronTasks(w, r, ct)
	case http.MethodPost:
		s.createCronTask(w, r, ct)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) getCronTrigger() *gateway.CronTrigger {
	if s.gw == nil {
		return nil
	}
	return s.gw.GetCronTrigger()
}

func (s *APIServer) listCronTasks(w http.ResponseWriter, r *http.Request, ct *gateway.CronTrigger) {
	if ct == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tasks": []any{}})
		return
	}
	tasks, err := ct.ListTasks()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": enrichCronTasks(tasks)})
}

func (s *APIServer) createCronTask(w http.ResponseWriter, r *http.Request, ct *gateway.CronTrigger) {
	var req struct {
		Instruction string `json:"instruction"`
		Schedule    string `json:"schedule"`
		Platform    string `json:"platform"`
		UserID      string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Instruction == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "instruction is required"})
		return
	}
	if req.Schedule == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "schedule is required"})
		return
	}
	if req.Platform == "" {
		req.Platform = "api"
	}
	if req.UserID == "" {
		req.UserID = getUserID(r)
	}
	if ct == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron system not available"})
		return
	}

	expr, err := gateway.ParseNaturalLanguage(req.Schedule)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot parse schedule: " + err.Error()})
		return
	}
	if err := gateway.ValidateCronExpression(expr); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid schedule: " + err.Error()})
		return
	}

	taskID := uuid.New().String()[:8]
	if err := ct.ScheduleCron(taskID, req.UserID, req.Instruction, expr, req.Platform); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	task, _ := ct.GetTask(taskID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"success":   true,
		"task":      enrichCronTask(task),
		"parsed":    expr,
		"humanized": gateway.DescribeCronExpression(expr),
	})
}

func (s *APIServer) handleCronTaskDetail(w http.ResponseWriter, r *http.Request) {
	ct := s.getCronTrigger()
	id := extractCronTaskID(r.URL.Path)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task id is required"})
		return
	}
	if ct == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron system not available"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, err := ct.GetTask(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, enrichCronTask(task))
	case http.MethodDelete:
		if err := ct.DeleteTask(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "id": id})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleCronTaskToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ct := s.getCronTrigger()
	id := extractCronTaskID(r.URL.Path)
	if ct == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron system not available"})
		return
	}

	task, err := ct.GetTask(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	if task.Enabled {
		err = ct.DisableTask(id)
	} else {
		err = ct.EnableTask(id)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	task, _ = ct.GetTask(id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "task": enrichCronTask(task)})
}

func (s *APIServer) handleCronTaskRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ct := s.getCronTrigger()
	id := extractCronTaskID(r.URL.Path)
	if ct == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron system not available"})
		return
	}

	task, err := ct.GetTask(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	task.LastRunAt = time.Now().Format(time.RFC3339)
	ct.ScheduleCron(task.ID, task.UserID, task.Instruction, task.Schedule, task.Platform)

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"id":      id,
		"message": "Task execution triggered",
	})
}

type cronTaskJSON struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Instruction string `json:"instruction"`
	Schedule    string `json:"schedule"`
	Humanized   string `json:"humanized"`
	Platform    string `json:"platform"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	LastRunAt   string `json:"last_run_at,omitempty"`
}

func enrichCronTask(t *gateway.CronTask) cronTaskJSON {
	if t == nil {
		return cronTaskJSON{}
	}
	return cronTaskJSON{
		ID:          t.ID,
		UserID:      t.UserID,
		Instruction: t.Instruction,
		Schedule:    t.Schedule,
		Humanized:   gateway.DescribeCronExpression(t.Schedule),
		Platform:    t.Platform,
		Enabled:     t.Enabled,
		CreatedAt:   t.CreatedAt,
		LastRunAt:   t.LastRunAt,
	}
}

func enrichCronTasks(tasks []*gateway.CronTask) []cronTaskJSON {
	result := make([]cronTaskJSON, 0, len(tasks))
	for _, t := range tasks {
		result = append(result, enrichCronTask(t))
	}
	return result
}

func extractCronTaskID(path string) string {
	prefix := "/api/cron/tasks/"
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, "/toggle")
	s = strings.TrimSuffix(s, "/run")
	return s
}

type mcpCatalogEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Type        string   `json:"type"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Popular     bool     `json:"popular"`
}

var mcpCatalog = []mcpCatalogEntry{
	{Name: "filesystem", Description: "File system operations with access controls", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}, Popular: true},
	{Name: "github", Description: "GitHub API - repos, issues, PRs, search", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-github"}, Popular: true},
	{Name: "postgres", Description: "PostgreSQL database queries and schema", Category: "Data", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-postgres"}, Popular: true},
	{Name: "sqlite", Description: "SQLite database exploration and queries", Category: "Data", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-sqlite"}},
	{Name: "fetch", Description: "Web content fetching and search", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-fetch"}, Popular: true},
	{Name: "memory", Description: "Knowledge graph and persistent memory", Category: "AI", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-memory"}, Popular: true},
}

type fileNode struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Size     int64      `json:"size,omitempty"`
	Children []fileNode `json:"children,omitempty"`
}

func buildFileTree(root string, maxDepth int) ([]fileNode, error) {
	if maxDepth <= 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var nodes []fileNode
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != ".smartclaw" {
			continue
		}

		node := fileNode{Name: name}
		if entry.IsDir() {
			skipDirs := map[string]bool{
				"node_modules": true, "vendor": true, ".git": true,
				"dist": true, "build": true, "bin": true, "__pycache__": true,
			}
			if skipDirs[name] {
				node.Type = "dir"
				nodes = append(nodes, node)
				continue
			}

			node.Type = "dir"
			children, err := buildFileTree(filepath.Join(root, name), maxDepth-1)
			if err == nil {
				node.Children = children
			}
		} else {
			info, err := entry.Info()
			if err == nil {
				node.Size = info.Size()
			}
			node.Type = "file"
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (s *APIServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"messages": []any{},
			"sessions": []any{},
			"query":    "",
		})
		return
	}

	userID := getUserID(r)
	limit := clampLimit(parseIntDefault(r.URL.Query().Get("limit"), 20), 20)
	since := parseTime(r.URL.Query().Get("since"))
	until := parseTime(r.URL.Query().Get("until"))
	role := r.URL.Query().Get("role")
	if role != "" && role != "user" && role != "assistant" && role != "system" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role parameter"})
		return
	}
	sessionID := r.URL.Query().Get("session_id")
	if sessionID != "" && !isValidAlphanumeric(sessionID, 128, "-") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid session_id parameter"})
		return
	}

	opts := store.SearchOptions{
		UserID:    userID,
		Limit:     limit,
		Since:     since,
		Until:     until,
		Role:      role,
		SessionID: sessionID,
	}

	var messageResults []*store.SearchResult
	if s.store != nil {
		messageResults, _ = s.store.SearchMessagesAdvanced(q, opts)
	}
	if messageResults == nil {
		messageResults = []*store.SearchResult{}
	}

	type sessionSearchItem struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Summary   string `json:"summary"`
		Model     string `json:"model"`
		UpdatedAt string `json:"updatedAt"`
	}

	var sessionResults []sessionSearchItem
	if s.store != nil {
		likePattern := "%" + q + "%"
		var rows interface {
			Close() error
			Next() bool
			Scan(dest ...any) error
		}
		var err error
		sessionLimit := 10

		if userID != "" && userID != "default" {
			rows, err = s.store.DB().Query(`
				SELECT id, title, summary, model, updated_at
				FROM sessions
				WHERE (title LIKE ? OR summary LIKE ?) AND user_id = ?
				ORDER BY updated_at DESC LIMIT ?
			`, likePattern, likePattern, userID, sessionLimit)
		} else {
			rows, err = s.store.DB().Query(`
				SELECT id, title, summary, model, updated_at
				FROM sessions
				WHERE title LIKE ? OR summary LIKE ?
				ORDER BY updated_at DESC LIMIT ?
			`, likePattern, likePattern, sessionLimit)
		}
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, title, summary, model, updatedAt string
				if err := rows.Scan(&id, &title, &summary, &model, &updatedAt); err != nil {
					continue
				}
				parsedTime := updatedAt
				if t, err := time.Parse("2006-01-02 15:04:05", updatedAt); err == nil {
					parsedTime = t.Format(time.RFC3339)
				}
				sessionResults = append(sessionResults, sessionSearchItem{
					ID:        id,
					Title:     title,
					Summary:   summary,
					Model:     model,
					UpdatedAt: parsedTime,
				})
			}
		}
	}
	if sessionResults == nil {
		sessionResults = []sessionSearchItem{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"messages": messageResults,
		"sessions": sessionResults,
		"query":    q,
	})
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return time.Time{}
}

func (s *APIServer) handleOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	userID := getUserID(r)
	state, err := s.onboardingMgr.GetState(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *APIServer) handleOnboardingStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	userID := getUserID(r)
	state, err := s.onboardingMgr.GetState(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if state.Step >= 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "onboarding already started"})
		return
	}
	state, err = s.onboardingMgr.StartOnboarding(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	firstStep := onboarding.GetStep(1)
	writeJSON(w, http.StatusOK, map[string]any{
		"state": state,
		"step":  firstStep,
	})
}

func (s *APIServer) handleOnboardingStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		SkillCreated string `json:"skill_created"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	userID := getUserID(r)
	state, nextStep, err := s.onboardingMgr.AdvanceStep(userID, req.SkillCreated)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	result := map[string]any{
		"state": state,
	}
	if nextStep != nil {
		result["step"] = nextStep
	}
	if state.Step == 4 {
		currentStep := onboarding.GetStep(3)
		if currentStep != nil {
			result["insight"] = currentStep.Insight
		}
		result["completed"] = true
		result["message"] = "SmartClaw now knows 3 things about how you work. It will get smarter every session."
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *APIServer) handleCostDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.costGuard == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"current_session":   costguard.CostSnapshot{},
			"today_total":       0,
			"week_total":        0,
			"month_total":       0,
			"budget_remaining":  0,
			"budget_fraction":   0,
			"history":           []costguard.CostHistoryEntry{},
			"model_breakdown":   []costguard.ModelCostBreakdown{},
			"optimization_tips": []costguard.OptimizationTip{},
			"daily_averages":    0,
			"projected_monthly": 0,
		})
		return
	}

	var dbPtr *sql.DB
	if s.store != nil {
		dbPtr = s.store.DB()
	}

	dashboard, err := costguard.GetDashboard(s.costGuard, dbPtr)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, dashboard)
}

func (s *APIServer) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if s.workflowSvc == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		workflows, err := s.workflowSvc.ListWorkflows()
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
		if s.workflowSvc == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workflow service not available"})
			return
		}
		if err := s.workflowSvc.SaveWorkflow(&pb); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"success": true, "name": pb.Name})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleWorkflowRoutes(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	name = strings.TrimSuffix(name, "/")
	if name == "" {
		s.handleWorkflows(w, r)
		return
	}

	if strings.HasSuffix(name, "/execute") {
		workflowName := strings.TrimSuffix(name, "/execute")
		s.handleWorkflowExecute(w, r, workflowName)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if s.workflowSvc == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workflow service not available"})
			return
		}
		pb, err := s.workflowSvc.GetWorkflow(name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, pb)
	case http.MethodDelete:
		if s.workflowSvc == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workflow service not available"})
			return
		}
		if err := s.workflowSvc.DeleteWorkflow(name); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "name": name})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleWorkflowExecute(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.workflowSvc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workflow service not available"})
		return
	}

	var req struct {
		Params map[string]string `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Params = map[string]string{}
	}

	execCtx, err := s.workflowSvc.ExecuteWorkflow(r.Context(), name, req.Params)
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

func (s *APIServer) handleWorkflowTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if s.workflowSvc == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, s.workflowSvc.GetAvailableTools())
}

func (s *APIServer) handleCostHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	period := r.URL.Query().Get("period")
	days := 7
	switch period {
	case "30d":
		days = 30
	case "90d":
		days = 90
	case "7d":
		days = 7
	default:
		days = 7
	}

	if s.store == nil {
		writeJSON(w, http.StatusOK, []costguard.CostHistoryEntry{})
		return
	}

	history, err := costguard.GetCostHistory(s.store.DB(), days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, history)
}

func (s *APIServer) handleCostForecast(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var dbPtr *sql.DB
	if s.store != nil {
		dbPtr = s.store.DB()
	}

	budget := costguard.DefaultBudgetConfig()
	if s.costGuard != nil {
		budget = s.costGuard.GetConfig()
	}

	forecast, err := costguard.ForecastCost(dbPtr, budget)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, forecast)
}

func (s *APIServer) handleCostEstimate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	taskType := r.URL.Query().Get("task_type")
	if taskType == "" {
		taskType = "general"
	}

	var dbPtr *sql.DB
	if s.store != nil {
		dbPtr = s.store.DB()
	}

	estimate, err := costguard.EstimateTaskCost(dbPtr, taskType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, estimate)
}

func (s *APIServer) handleCostProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	days := parseIntDefault(r.URL.Query().Get("days"), 30)

	if s.store == nil {
		writeJSON(w, http.StatusOK, []costguard.ProjectCost{})
		return
	}

	projects, err := costguard.GetProjectCosts(s.store.DB(), days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, projects)
}

func (s *APIServer) handleCostReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var dbPtr *sql.DB
	if s.store != nil {
		dbPtr = s.store.DB()
	}

	budget := costguard.DefaultBudgetConfig()
	if s.costGuard != nil {
		budget = s.costGuard.GetConfig()
	}

	report, err := costguard.GenerateWeeklyReport(dbPtr, budget)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (s *APIServer) handleMarketplaceSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.skillRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]any{"skills": []any{}, "total": 0, "page": 1, "page_size": 20})
		return
	}

	mp := skills.NewMarketplace(s.skillRegistry)
	q := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("pageSize"), 20)

	result, err := mp.SearchMarketplace(q, category, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *APIServer) handleMarketplaceInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

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

	if s.skillRegistry == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill registry not available"})
		return
	}

	mp := skills.NewMarketplace(s.skillRegistry)
	meta, err := mp.InstallSkill(req.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"skill":   meta,
	})
}

func (s *APIServer) handleMarketplacePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

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

	if s.skillRegistry == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "skill registry not available"})
		return
	}

	mp := skills.NewMarketplace(s.skillRegistry)
	if err := mp.PublishSkill(req.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"name":    req.Name,
	})
}

func (s *APIServer) handleMarketplaceFeatured(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.skillRegistry == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	mp := skills.NewMarketplace(s.skillRegistry)
	featured, err := mp.GetFeatured()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, featured)
}

func (s *APIServer) handleMarketplaceCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	mp := skills.NewMarketplace(s.skillRegistry)
	writeJSON(w, http.StatusOK, mp.GetCategories())
}

var validCommunicationStyles = map[string]bool{
	"concise": true, "verbose": true, "technical": true,
	"plain": true, "step-by-step": true, "direct": true,
}

func (s *APIServer) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.userModelEngine == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"preferences":          map[string]string{},
			"communication_style":  "",
			"knowledge_background": []string{},
			"top_patterns":         []any{},
			"conflicts":            []any{},
			"last_updated":         time.Time{},
		})
		return
	}

	snapshot, err := s.userModelEngine.SynthesizeModel(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type patternJSON struct {
		Pattern   string `json:"pattern"`
		Frequency int    `json:"frequency"`
		LastSeen  string `json:"last_seen"`
	}
	type conflictJSON struct {
		Category             string  `json:"category"`
		Key                  string  `json:"key"`
		Thesis               string  `json:"thesis"`
		ThesisConfidence     float64 `json:"thesis_confidence"`
		ThesisObservedAt     string  `json:"thesis_observed_at"`
		Antithesis           string  `json:"antithesis"`
		AntithesisConfidence float64 `json:"antithesis_confidence"`
		AntithesisObservedAt string  `json:"antithesis_observed_at"`
		Resolved             bool    `json:"resolved"`
		Resolution           string  `json:"resolution"`
	}

	patterns := make([]patternJSON, 0, len(snapshot.TopPatterns))
	for _, p := range snapshot.TopPatterns {
		patterns = append(patterns, patternJSON{
			Pattern:   p.Pattern,
			Frequency: p.Frequency,
			LastSeen:  p.LastSeen.Format(time.RFC3339),
		})
	}

	conflicts := make([]conflictJSON, 0, len(snapshot.Conflicts))
	for _, c := range snapshot.Conflicts {
		conflicts = append(conflicts, conflictJSON{
			Category:             c.Category,
			Key:                  c.Key,
			Thesis:               c.Thesis,
			ThesisConfidence:     c.ThesisConfidence,
			ThesisObservedAt:     c.ThesisObservedAt.Format(time.RFC3339),
			Antithesis:           c.Antithesis,
			AntithesisConfidence: c.AntithesisConfidence,
			AntithesisObservedAt: c.AntithesisObservedAt.Format(time.RFC3339),
			Resolved:             c.Resolved,
			Resolution:           c.Resolution,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"preferences":          snapshot.Preferences,
		"communication_style":  snapshot.CommunicationStyle,
		"knowledge_background": snapshot.KnowledgeBackground,
		"top_patterns":         patterns,
		"conflicts":            conflicts,
		"last_updated":         snapshot.LastUpdated.Format(time.RFC3339),
	})
}

func (s *APIServer) handleProfileStyle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		CommunicationStyle string `json:"communication_style"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if !validCommunicationStyles[req.CommunicationStyle] {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid style; must be one of: concise, verbose, technical, plain, step-by-step, direct",
		})
		return
	}

	userID := getUserID(r)

	if s.userModelEngine != nil {
		if err := s.userModelEngine.RecordObservation(r.Context(), "communication_style", "style", req.CommunicationStyle, 0.95, "profile-ui"); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record style: " + err.Error()})
			return
		}
	}

	if s.memMgr != nil {
		pm := s.memMgr.GetPromptMemory()
		if pm != nil {
			existing := pm.GetUserContent()
			var newContent string
			styleLine := "Communication style: " + req.CommunicationStyle
			if existing == "" {
				newContent = "# User Profile\n\n" + styleLine + "\n"
			} else if strings.Contains(existing, "Communication style:") {
				lines := strings.Split(existing, "\n")
				for i, line := range lines {
					if strings.HasPrefix(line, "Communication style:") {
						lines[i] = styleLine
						break
					}
				}
				newContent = strings.Join(lines, "\n")
			} else {
				newContent = existing + "\n" + styleLine + "\n"
			}
			if err := pm.UpdateUserProfile(newContent); err != nil {
				slog.Warn("failed to update user profile", "error", err)
			}
			pm.EnforceLimit()
		}
	}

	if s.userModelEngine != nil {
		if _, err := s.userModelEngine.SynthesizeModel(r.Context()); err != nil {
			slog.Warn("failed to synthesize user model", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":             true,
		"communication_style": req.CommunicationStyle,
		"user_id":             userID,
	})
}

func (s *APIServer) handleProfileObservations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.store == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	rows, err := s.store.DB().Query(
		`SELECT id, category, key, value, confidence, observed_at, session_id, user_id FROM user_observations ORDER BY observed_at DESC`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type obsJSON struct {
		ID         int64   `json:"id"`
		Category   string  `json:"category"`
		Key        string  `json:"key"`
		Value      string  `json:"value"`
		Confidence float64 `json:"confidence"`
		ObservedAt string  `json:"observed_at"`
		SessionID  string  `json:"session_id"`
		UserID     string  `json:"user_id"`
	}

	var observations []obsJSON
	for rows.Next() {
		var obs obsJSON
		var observedAtStr string
		if err := rows.Scan(&obs.ID, &obs.Category, &obs.Key, &obs.Value, &obs.Confidence, &observedAtStr, &obs.SessionID, &obs.UserID); err != nil {
			continue
		}
		if t, err := time.Parse(time.RFC3339, observedAtStr); err == nil {
			obs.ObservedAt = t.Format(time.RFC3339)
		} else if t, err := time.Parse("2006-01-02 15:04:05", observedAtStr); err == nil {
			obs.ObservedAt = t.Format(time.RFC3339)
		} else {
			obs.ObservedAt = observedAtStr
		}
		observations = append(observations, obs)
	}
	if observations == nil {
		observations = []obsJSON{}
	}

	writeJSON(w, http.StatusOK, observations)
}

func (s *APIServer) handleProfileObservationDelete(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid observation id"})
		return
	}

	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store not available"})
		return
	}

	result, err := s.store.DB().Exec(`DELETE FROM user_observations WHERE id = ?`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()

	if s.userModelEngine != nil {
		if _, err := s.userModelEngine.SynthesizeModel(r.Context()); err != nil {
			slog.Warn("failed to synthesize user model", "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"affected": affected,
		"id":       id,
	})
}

func (s *APIServer) handleProfileObservationsDeleteAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store not available"})
		return
	}

	result, err := s.store.DB().Exec(`DELETE FROM user_observations`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()

	if s.userModelEngine != nil {
		if _, err := s.userModelEngine.SynthesizeModel(r.Context()); err != nil {
			slog.Warn("failed to synthesize user model", "error", err)
		}
	}

	if s.memMgr != nil {
		pm := s.memMgr.GetPromptMemory()
		if pm != nil {
			if err := pm.UpdateUserProfile(""); err != nil {
				slog.Warn("failed to update user profile", "error", err)
			}
			pm.EnforceLimit()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"affected": affected,
	})
}

func (s *APIServer) handleProfileRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/profile/observations/") {
		idStr := strings.TrimPrefix(path, "/api/profile/observations/")
		idStr = strings.TrimSuffix(idStr, "/")
		if idStr != "" && idStr != "all" {
			s.handleProfileObservationDelete(w, r, idStr)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (s *APIServer) handleRooms(w http.ResponseWriter, r *http.Request) {
	if s.roomMgr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "room system not available"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		rooms := s.roomMgr.ListRooms()
		summaries := make([]map[string]any, 0, len(rooms))
		for _, room := range rooms {
			summaries = append(summaries, roomSummary(room))
		}
		writeJSON(w, http.StatusOK, summaries)
	case http.MethodPost:
		var req struct {
			Name      string `json:"name"`
			SessionID string `json:"session_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.Name == "" {
			req.Name = "Collaboration Session"
		}
		userID := getUserID(r)
		room := s.roomMgr.CreateRoom(req.Name, userID, req.SessionID)
		writeJSON(w, http.StatusCreated, map[string]any{
			"room_id": room.ID,
			"name":    room.Name,
			"ws_url":  "/ws?room=" + room.ID,
		})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleRoomRoutes(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/rooms/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		s.handleRooms(w, r)
		return
	}

	if s.roomMgr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "room system not available"})
		return
	}

	room, ok := s.roomMgr.GetRoom(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "room not found"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"id":                room.ID,
			"name":              room.Name,
			"session_id":        room.SessionID,
			"owner_id":          room.OwnerID,
			"created_at":        room.CreatedAt.Format(time.RFC3339),
			"participants":      room.participantList(),
			"active_editor":     room.ActiveEditor,
		})
	case http.MethodDelete:
		userID := getUserID(r)
		room.mu.RLock()
		ownerID := room.OwnerID
		room.mu.RUnlock()
		if userID != ownerID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the room owner can close the room"})
			return
		}
		s.roomMgr.mu.Lock()
		delete(s.roomMgr.rooms, id)
		s.roomMgr.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "room_id": id})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) handleSemanticSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		Query    string  `json:"query"`
		Limit    int     `json:"limit"`
		MinScore float64 `json:"min_score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query is required"})
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.MinScore <= 0 {
		req.MinScore = 0.5
	}

	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store not available"})
		return
	}

	emb := store.GetGlobalEmbedder()
	if emb == nil {
		emb = store.NewDefaultEmbedder()
	}

	queryVec, err := emb.Embed(r.Context(), req.Query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "embedding failed: " + err.Error()})
		return
	}

	results, err := s.store.SearchEmbeddings(r.Context(), queryVec, req.Limit, req.MinScore)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type resultJSON struct {
		SourceType string  `json:"source_type"`
		SourceID   string  `json:"source_id"`
		Content    string  `json:"content"`
		Snippet    string  `json:"snippet"`
		Score      float64 `json:"score"`
	}

	out := make([]resultJSON, len(results))
	for i, r := range results {
		snippet := r.Content
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		out[i] = resultJSON{
			SourceType: r.SourceType,
			SourceID:   r.SourceID,
			Content:    r.Content,
			Snippet:    snippet,
			Score:      r.Score,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": out,
		"query":   req.Query,
		"count":   len(out),
	})
}

func (s *APIServer) handleKnowledgeSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	contextText := r.URL.Query().Get("context")
	if contextText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "context parameter is required"})
		return
	}

	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "store not available"})
		return
	}

	emb := store.GetGlobalEmbedder()
	if emb == nil {
		emb = store.NewDefaultEmbedder()
	}

	queryVec, err := emb.Embed(r.Context(), contextText)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "embedding failed: " + err.Error()})
		return
	}

	results, err := s.store.SearchEmbeddings(r.Context(), queryVec, 3, 0.3)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type suggestion struct {
		SourceType string  `json:"source_type"`
		SourceID   string  `json:"source_id"`
		Content    string  `json:"content"`
		Score      float64 `json:"score"`
		Message    string  `json:"message"`
	}

	suggestions := make([]suggestion, len(results))
	for i, r := range results {
		msg := formatSuggestionMessage(r.SourceType, r.SourceID, r.Content, r.Score)
		suggestions[i] = suggestion{
			SourceType: r.SourceType,
			SourceID:   r.SourceID,
			Content:    r.Content,
			Score:      r.Score,
			Message:    msg,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"suggestions": suggestions,
		"context":     contextText,
	})
}

func formatSuggestionMessage(sourceType, sourceID, content string, score float64) string {
	snippet := content
	if len(snippet) > 80 {
		snippet = snippet[:80] + "..."
	}
	switch sourceType {
	case "message":
		return fmt.Sprintf("This looks similar to session %s — %s", sourceID, snippet)
	case "skill":
		return fmt.Sprintf("Relevant skill: %s — %s", sourceID, snippet)
	case "memory":
		return fmt.Sprintf("Related memory: %s — %s", sourceID, snippet)
	default:
		return fmt.Sprintf("Similar to %s/%s — %s", sourceType, sourceID, snippet)
	}
}

func readSkillContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
