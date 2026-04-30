package web

import (
	"encoding/json"
	"fmt"
	"time"
)

func (h *Handler) handleMemoryLayersWS(client *Client) {
	if h.memMgr == nil {
		h.sendError(client, "Memory manager not available")
		return
	}
	pm := h.memMgr.GetPromptMemory()
	layers := map[string]any{
		"memory": pm.GetMemoryContent(),
		"user":   pm.GetUserContent(),
	}
	h.sendToClient(client, WSResponse{Type: "memory_layers", Data: layers})
}

func (h *Handler) handleMemorySearchWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid memory search request")
		return
	}
	query, _ := data["query"].(string)
	limit := 5
	if l, ok := data["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	if h.memMgr == nil {
		h.sendError(client, "Memory manager not available")
		return
	}
	ctx := h.shutdownCtx
	results, err := h.memMgr.Search(ctx, query, limit)
	if err != nil {
		h.sendError(client, "Search failed: "+err.Error())
		return
	}
	h.sendToClient(client, WSResponse{Type: "memory_search", Data: results})
}

func (h *Handler) handleMemoryRecallWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid memory recall request")
		return
	}
	query, _ := data["query"].(string)
	if h.memMgr == nil {
		h.sendError(client, "Memory manager not available")
		return
	}
	ctx := h.shutdownCtx
	contextStr := h.memMgr.BuildSystemContext(ctx, query)
	h.sendToClient(client, WSResponse{Type: "memory_recall", Data: map[string]any{
		"query":   query,
		"context": contextStr,
	}})
}

func (h *Handler) handleMemoryStoreWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid memory store request")
		return
	}
	content, _ := data["content"].(string)
	if content == "" {
		h.sendError(client, "Content is required for store")
		return
	}
	if h.memMgr == nil {
		h.sendError(client, "Memory manager not available")
		return
	}
	pm := h.memMgr.GetPromptMemory()
	key, _ := data["key"].(string)
	if key == "" {
		key = "fact"
	}
	entry := fmt.Sprintf("- **%s**: %s (stored %s)", key, content, time.Now().Format("2006-01-02"))
	if err := pm.AppendToSection("## Stored Facts", entry); err != nil {
		h.sendError(client, "Store failed: "+err.Error())
		return
	}
	h.sendToClient(client, WSResponse{Type: "memory_store", Data: map[string]any{
		"stored": true,
		"key":    key,
	}})
}

func (h *Handler) handleMemoryUpdateWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid memory update request")
		return
	}
	file, _ := data["file"].(string)
	content, _ := data["content"].(string)
	if file != "memory" && file != "user" {
		h.sendError(client, "File must be 'memory' or 'user'")
		return
	}
	if content == "" {
		h.sendError(client, "Content must not be empty")
		return
	}
	if h.memMgr == nil {
		h.sendError(client, "Memory manager not available")
		return
	}
	pm := h.memMgr.GetPromptMemory()
	var updateErr error
	if file == "memory" {
		updateErr = pm.UpdateMemory(content)
	} else {
		updateErr = pm.UpdateUserProfile(content)
	}
	if updateErr != nil {
		h.sendError(client, "Update failed: "+updateErr.Error())
		return
	}
	pm.EnforceLimit()
	h.sendToClient(client, WSResponse{Type: "memory_update", Data: map[string]any{
		"success": true,
		"file":    file,
	}})
	layers := map[string]any{
		"memory": pm.GetMemoryContent(),
		"user":   pm.GetUserContent(),
	}
	h.sendToClient(client, WSResponse{Type: "memory_layers", Data: layers})
	stats := map[string]any{
		"memory_chars": len(pm.GetMemoryContent()),
		"user_chars":   len(pm.GetUserContent()),
	}
	h.sendToClient(client, WSResponse{Type: "memory_stats", Data: stats})
}

func (h *Handler) handleMemoryStatsWS(client *Client) {
	if h.memMgr == nil {
		h.sendError(client, "Memory manager not available")
		return
	}
	pm := h.memMgr.GetPromptMemory()
	stats := map[string]any{
		"memory_chars": len(pm.GetMemoryContent()),
		"user_chars":   len(pm.GetUserContent()),
	}
	h.sendToClient(client, WSResponse{Type: "memory_stats", Data: stats})
}

func (h *Handler) handleMemoryObservationsWS(client *Client) {
	if h.memMgr == nil || h.dataStore == nil {
		h.sendToClient(client, WSResponse{Type: "memory_observations", Data: []any{}})
		return
	}

	rows, err := h.dataStore.DB().Query(
		`SELECT id, category, key, value, confidence, observed_at, session_id FROM user_observations ORDER BY observed_at DESC LIMIT 100`,
	)
	if err != nil {
		h.sendError(client, "Failed to query observations: "+err.Error())
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
	h.sendToClient(client, WSResponse{Type: "memory_observations", Data: observations})
}

func (h *Handler) handleMemoryObservationDeleteWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid observation delete request")
		return
	}

	id, ok := data["id"].(float64)
	if !ok {
		h.sendError(client, "Observation id is required")
		return
	}

	if h.dataStore == nil {
		h.sendError(client, "Store not available")
		return
	}

	result, err := h.dataStore.DB().Exec(`DELETE FROM user_observations WHERE id = ?`, int(id))
	if err != nil {
		h.sendError(client, "Failed to delete observation: "+err.Error())
		return
	}
	affected, _ := result.RowsAffected()
	h.sendToClient(client, WSResponse{Type: "memory_observation_delete", Data: map[string]any{
		"success":  true,
		"affected": affected,
	}})

	h.handleMemoryObservationsWS(client)
}
