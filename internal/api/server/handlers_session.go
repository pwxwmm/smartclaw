package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/instructkr/smartclaw/internal/store"
)

func (s *APIServer) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSessions(w, r)
	case http.MethodPost:
		s.createSession(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) listSessions(w http.ResponseWriter, r *http.Request) {
	if s.store != nil {
		sessions, err := s.store.ListAllSessions(50)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, sessions)
		return
	}

	if s.sessMgr == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	sessions, err := s.sessMgr.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, sessions)
}

func (s *APIServer) createSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model  string `json:"model"`
		UserID string `json:"user_id"`
		Title  string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Model == "" {
		req.Model = "sre-model"
	}
	if req.UserID == "" {
		req.UserID = getUserID(r)
	}

	sessionID := uuid.New().String()[:8] + "_" + time.Now().Format("20060102_150405")

	if s.store != nil {
		now := time.Now()
		err := s.store.UpsertSession(r.Context(), &store.Session{
			ID:        sessionID,
			UserID:    req.UserID,
			Model:     req.Model,
			Title:     req.Title,
			CreatedAt: now,
			UpdatedAt: now,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        sessionID,
		"user_id":   req.UserID,
		"model":     req.Model,
		"title":     req.Title,
		"created_at": time.Now().Format(time.RFC3339),
	})
}

func (s *APIServer) handleSessionGet(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		s.getSession(w, r, id)
	case http.MethodDelete:
		s.deleteSession(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *APIServer) getSession(w http.ResponseWriter, r *http.Request, id string) {
	if s.store != nil {
		session, err := s.store.GetSession(id)
		if err != nil || session == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		writeJSON(w, http.StatusOK, session)
		return
	}

	if s.sessMgr != nil {
		sess, err := s.sessMgr.Load(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		writeJSON(w, http.StatusOK, sess)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
}

func (s *APIServer) deleteSession(w http.ResponseWriter, r *http.Request, id string) {
	if s.store != nil {
		if err := s.store.DeleteSession(r.Context(), id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "id": id})
		return
	}

	if s.sessMgr != nil {
		if err := s.sessMgr.Delete(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "id": id})
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
}

func (s *APIServer) handleSessionSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
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
	limit = clampLimit(limit, 10)

	if s.memMgr == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}

	fragments, err := s.memMgr.Search(r.Context(), query, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

func extractSessionID(path string) string {
	prefix := "/api/sessions/"
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimSuffix(s, "/")
	return s
}
