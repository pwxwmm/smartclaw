package web

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/instructkr/smartclaw/internal/session"
	"github.com/instructkr/smartclaw/internal/store"
)

func (h *Handler) handleSessionList(client *Client) {
	if h.dataStore != nil {
		var sessions []*store.Session
		var err error
		if client.UserID != "" && client.UserID != "default" {
			sessions, err = h.dataStore.ListSessions(client.UserID, 50)
		} else {
			sessions, err = h.dataStore.ListAllSessions(50)
		}
		if err != nil {
			h.sendError(client, fmt.Sprintf("Failed to list sessions: %v", err))
			return
		}

		counts, _ := h.dataStore.GetMessageCountsBatch()
		var infos []SessionInfo
		for _, s := range sessions {
			infos = append(infos, SessionInfo{
				ID:           s.ID,
				UserID:       s.UserID,
				Title:        s.Title,
				Model:        s.Model,
				MessageCount: int(counts[s.ID]),
				CreatedAt:    s.CreatedAt.Format(time.RFC3339),
				UpdatedAt:    s.UpdatedAt.Format(time.RFC3339),
			})
		}
		h.sendToClient(client, WSResponse{Type: "session_list", Sessions: infos})
		return
	}

	if h.sessMgr == nil {
		h.sendToClient(client, WSResponse{Type: "session_list", Sessions: []SessionInfo{}})
		return
	}

	var sessions []session.SessionInfo
	var err error
	if client.UserID != "" && client.UserID != "default" {
		sessions, err = h.sessMgr.ListByUser(client.UserID)
	} else {
		sessions, err = h.sessMgr.List()
	}
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}

	var infos []SessionInfo
	for _, s := range sessions {
		infos = append(infos, SessionInfo{
			ID:           s.ID,
			UserID:       s.UserID,
			Title:        s.Title,
			Model:        s.Model,
			MessageCount: s.MessageCount,
			CreatedAt:    s.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    s.UpdatedAt.Format(time.RFC3339),
		})
	}

	h.sendToClient(client, WSResponse{Type: "session_list", Sessions: infos})
}

func (h *Handler) handleSessionNew(client *Client, msg WSMessage) {
	model := msg.Model
	if model == "" {
		model = h.apiClient.Model
		if m, ok := h.clientModels[client.ID]; ok {
			model = m
		}
	}

	if h.dataStore != nil {
		sess := h.sessMgr.NewSession(model, client.UserID)
		storeSess := &store.Session{
			ID:        sess.ID,
			UserID:    client.UserID,
			Source:    "web",
			Model:     model,
			Title:     sess.Title,
			Tokens:    0,
			Cost:      0,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
		}
		if err := h.dataStore.UpsertSession(h.shutdownCtx, storeSess); err != nil {
			h.sendError(client, fmt.Sprintf("Failed to save session: %v", err))
			return
		}
		h.clientSessMu.Lock()
		h.clientSess[client.ID] = sess
		h.clientSessMu.Unlock()
		h.sendToClient(client, WSResponse{
			Type:    "session_created",
			ID:      sess.ID,
			Message: "New session created",
		})
		return
	}

	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
		return
	}

	sess := h.sessMgr.NewSession(model, client.UserID)
	if err := h.sessMgr.Save(sess); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to save session: %v", err))
		return
	}

	h.clientSessMu.Lock()
	h.clientSess[client.ID] = sess
	h.clientSessMu.Unlock()

	h.sendToClient(client, WSResponse{
		Type:    "session_created",
		ID:      sess.ID,
		Message: "New session created",
	})
}

func (h *Handler) handleSessionLoad(client *Client, msg WSMessage) {
	if h.dataStore != nil {
		storeSess, err := h.dataStore.GetSession(msg.ID)
		if err != nil {
			h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
			return
		}
		if storeSess == nil {
			h.sendError(client, "Session not found")
			return
		}
		if storeSess.UserID != "" && storeSess.UserID != client.UserID && client.UserID != "default" {
			h.sendError(client, "Access denied: session belongs to another user")
			return
		}

		storeMsgs, _ := h.dataStore.GetSessionMessages(msg.ID)

		sess := &session.Session{
			ID:        storeSess.ID,
			UserID:    storeSess.UserID,
			CreatedAt: storeSess.CreatedAt,
			UpdatedAt: storeSess.UpdatedAt,
			Model:     storeSess.Model,
			Tokens:    storeSess.Tokens,
			Cost:      storeSess.Cost,
			Title:     storeSess.Title,
			Messages:  make([]session.Message, 0, len(storeMsgs)),
		}
		for _, m := range storeMsgs {
			sess.Messages = append(sess.Messages, session.Message{
				Role:      m.Role,
				Content:   m.Content,
				Timestamp: m.Timestamp,
				Tokens:    m.Tokens,
			})
		}

		h.clientSessMu.Lock()
		h.clientSess[client.ID] = sess
		h.clientSessMu.Unlock()

		var msgs []MsgItem
		for _, m := range sess.Messages {
			msgs = append(msgs, MsgItem{
				Role:      m.Role,
				Content:   m.Content,
				Timestamp: m.Timestamp.Format(time.RFC3339),
			})
		}

		h.sendToClient(client, WSResponse{
			Type:     "session_loaded",
			ID:       sess.ID,
			Title:    sess.Title,
			Messages: msgs,
		})
		return
	}

	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
		return
	}

	sess, err := h.sessMgr.Load(msg.ID)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
		return
	}

	if sess.UserID != "" && sess.UserID != client.UserID && client.UserID != "default" {
		h.sendError(client, "Access denied: session belongs to another user")
		return
	}

	h.clientSessMu.Lock()
	h.clientSess[client.ID] = sess
	h.clientSessMu.Unlock()

	var msgs []MsgItem
	for _, m := range sess.Messages {
		msgs = append(msgs, MsgItem{
			Role:      m.Role,
			Content:   m.Content,
			Timestamp: m.Timestamp.Format(time.RFC3339),
		})
	}

	h.sendToClient(client, WSResponse{
		Type:     "session_loaded",
		ID:       sess.ID,
		Title:    sess.Title,
		Messages: msgs,
	})
}

func (h *Handler) handleSessionDelete(client *Client, msg WSMessage) {
	if h.dataStore != nil {
		storeSess, err := h.dataStore.GetSession(msg.ID)
		if err != nil {
			h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
			return
		}
		if storeSess == nil {
			h.sendError(client, "Session not found")
			return
		}
		if storeSess.UserID != "" && storeSess.UserID != client.UserID && client.UserID != "default" {
			h.sendError(client, "Access denied: session belongs to another user")
			return
		}
		if err := h.dataStore.DeleteSession(h.shutdownCtx, msg.ID); err != nil {
			h.sendError(client, fmt.Sprintf("Failed to delete session: %v", err))
			return
		}
		h.sendToClient(client, WSResponse{
			Type:    "session_deleted",
			ID:      msg.ID,
			Message: "Session deleted",
		})
		return
	}

	if h.sessMgr == nil {
		h.sendError(client, "Session manager not available")
		return
	}

	sess, err := h.sessMgr.Load(msg.ID)
	if err != nil {
		h.sendError(client, fmt.Sprintf("Failed to load session: %v", err))
		return
	}
	if sess.UserID != "" && sess.UserID != client.UserID && client.UserID != "default" {
		h.sendError(client, "Access denied: session belongs to another user")
		return
	}

	if err := h.sessMgr.Delete(msg.ID); err != nil {
		h.sendError(client, fmt.Sprintf("Failed to delete session: %v", err))
		return
	}

	h.sendToClient(client, WSResponse{
		Type:    "session_deleted",
		ID:      msg.ID,
		Message: "Session deleted",
	})
}

func (h *Handler) handleSessionRename(client *Client, msg WSMessage) {
	if msg.ID == "" {
		h.sendError(client, "Session ID is required")
		return
	}

	if h.dataStore != nil {
		if err := h.dataStore.UpdateSessionTitle(h.shutdownCtx, msg.ID, msg.Title); err != nil {
			h.sendError(client, fmt.Sprintf("Failed to rename session: %v", err))
			return
		}
	} else if h.sessMgr != nil {
		if err := h.sessMgr.Rename(msg.ID, msg.Title); err != nil {
			h.sendError(client, fmt.Sprintf("Failed to rename session: %v", err))
			return
		}
	}

	h.clientSessMu.RLock()
	sess, ok := h.clientSess[client.ID]
	h.clientSessMu.RUnlock()
	if ok && sess.ID == msg.ID {
		sess.Title = msg.Title
	}

	h.handleSessionList(client)
}

func (h *Handler) handleSessionFragmentsWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid session fragments request")
		return
	}

	query, _ := data["query"].(string)
	if query == "" {
		h.sendToClient(client, WSResponse{Type: "session_fragments", Data: []any{}})
		return
	}

	limit := 10
	if l, ok := data["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	if h.memMgr == nil {
		h.sendError(client, "Memory manager not available")
		return
	}

	ctx := h.shutdownCtx
	fragments, err := h.memMgr.Search(ctx, query, limit)
	if err != nil {
		h.sendError(client, "Search failed: "+err.Error())
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
	h.sendToClient(client, WSResponse{Type: "session_fragments", Data: result})
}
