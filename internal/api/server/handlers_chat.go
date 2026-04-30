package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/instructkr/smartclaw/internal/gateway"
	"github.com/instructkr/smartclaw/internal/serverauth"
)

func (s *APIServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.gw == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gateway not available"})
		return
	}

	content, sessionID, err := s.parseChatRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	userID := getUserID(r)

	var resp *gateway.GatewayResponse
	if sessionID != "" {
		resp, err = s.gw.HandleMessageWithSession(r.Context(), userID, "api", content, sessionID)
	} else {
		resp, err = s.gw.HandleMessage(r.Context(), userID, "api", content)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"content":    resp.Content,
		"session_id": resp.SessionID,
		"platform":   resp.Platform,
		"usage": map[string]any{
			"input_tokens":  resp.Usage.InputTokens,
			"output_tokens": resp.Usage.OutputTokens,
			"cost":          resp.Usage.Cost,
		},
		"duration_ms": resp.Duration.Milliseconds(),
	})
}

func (s *APIServer) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if s.gw == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "gateway not available"})
		return
	}

	content, sessionID, err := s.parseChatRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	userID := getUserID(r)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, canFlush := w.(http.Flusher)

	var resp *gateway.GatewayResponse
	if sessionID != "" {
		resp, err = s.gw.HandleMessageWithSession(r.Context(), userID, "api", content, sessionID)
	} else {
		resp, err = s.gw.HandleMessage(r.Context(), userID, "api", content)
	}

	if err != nil {
		sseEvent(w, "error", map[string]string{"error": err.Error()})
		if canFlush {
			flusher.Flush()
		}
		return
	}

	sseEvent(w, "message_start", map[string]any{
		"session_id": resp.SessionID,
		"platform":   resp.Platform,
	})

	sseEvent(w, "content", map[string]string{"text": resp.Content})

	sseEvent(w, "usage", map[string]any{
		"input_tokens":  resp.Usage.InputTokens,
		"output_tokens": resp.Usage.OutputTokens,
		"cost":          resp.Usage.Cost,
		"duration_ms":   resp.Duration.Milliseconds(),
	})

	sseEvent(w, "done", map[string]string{"status": "complete"})

	if canFlush {
		flusher.Flush()
	}
}

func (s *APIServer) parseChatRequest(r *http.Request) (content, sessionID string, err error) {
	contentType := r.Header.Get("Content-Type")

	if contentType == "application/json" {
		var req struct {
			Content   string `json:"content"`
			SessionID string `json:"session_id"`
		}
		if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
			return "", "", fmt.Errorf("invalid JSON: %w", decodeErr)
		}
		return req.Content, req.SessionID, nil
	}

	content = r.FormValue("content")
	sessionID = r.FormValue("session_id")
	if content == "" {
		return "", "", fmt.Errorf("content is required")
	}
	return content, sessionID, nil
}

func sseEvent(w http.ResponseWriter, eventType string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
}

func (s *APIServer) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
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

	token, err := s.auth.Login(req.APIKey)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "smartclaw-token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(serverauth.SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *APIServer) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := s.noAuth || !s.auth.IsAuthRequired()
	if !authenticated {
		token := extractToken(r)
		authenticated = validateAccessToken(token, s.auth)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": authenticated})
}

func sseTimestamp() string {
	return time.Now().Format(time.RFC3339)
}
