package web

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/utils"
)

func (h *Handler) needsApproval(clientID, toolName string, input map[string]any) bool {
	if h.isAutoApproved(clientID, toolName) {
		return false
	}
	allowed, _ := h.unifiedPerm.CheckPermission(toolName, input, nil)
	return !allowed
}

func (h *Handler) isAutoApproved(clientID, toolName string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if tools, ok := h.autoApproved[clientID]; ok {
		return tools[toolName]
	}
	return false
}

func (h *Handler) requestApproval(client *Client, blockID, toolName string, input map[string]any) (bool, error) {
	h.sendToClient(client, WSResponse{
		Type:  "tool_approval_request",
		ID:    blockID,
		Tool:  toolName,
		Input: input,
	})

	ch := make(chan bool, 1)
	h.mu.Lock()
	h.pendingApprovals[blockID] = ch
	h.pendingApprovalMeta[blockID] = &ApprovalMeta{
		ToolName:    toolName,
		Input:       input,
		RequestedAt: time.Now(),
	}
	h.mu.Unlock()

	select {
	case approved := <-ch:
		return approved, nil
	case <-time.After(5 * time.Minute):
		h.mu.Lock()
		delete(h.pendingApprovals, blockID)
		delete(h.pendingApprovalMeta, blockID)
		h.mu.Unlock()
		return false, fmt.Errorf("tool approval timed out after 5 minutes")
	}
}

func (h *Handler) handleToolApproval(client *Client, msg WSMessage) {
	h.mu.Lock()
	ch, ok := h.pendingApprovals[msg.ID]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.pendingApprovals, msg.ID)

	meta := h.pendingApprovalMeta[msg.ID]
	decision := "denied"
	switch msg.Content {
	case "approve":
		decision = "approved"
		utils.Go(func() { observability.AuditApproval(msg.Name, msg.ID, true, client.ID) })
		ch <- true
	case "always_approve":
		decision = "always_approved"
		if h.autoApproved[client.ID] == nil {
			h.autoApproved[client.ID] = make(map[string]bool)
		}
		h.autoApproved[client.ID][msg.Name] = true
		utils.Go(func() { observability.AuditApproval(msg.Name, msg.ID, true, client.ID) })
		ch <- true
	default:
		utils.Go(func() { observability.AuditDenial(msg.Name, msg.ID, client.ID, "user denied") })
		ch <- false
	}

	toolName := msg.Name
	if meta != nil {
		toolName = meta.ToolName
		delete(h.pendingApprovalMeta, msg.ID)
	}

	h.approvalHistory = append(h.approvalHistory, ApprovalRecord{
		BlockID:   msg.ID,
		ToolName:  toolName,
		Decision:  decision,
		Timestamp: time.Now(),
	})
	if len(h.approvalHistory) > 200 {
		h.approvalHistory = h.approvalHistory[len(h.approvalHistory)-200:]
	}
	h.mu.Unlock()
}

func (h *Handler) handleApprovalListWS(client *Client) {
	h.mu.Lock()
	var list []map[string]any
	for blockID, meta := range h.pendingApprovalMeta {
		list = append(list, map[string]any{
			"blockId":     blockID,
			"toolName":    meta.ToolName,
			"input":       meta.Input,
			"requestedAt": meta.RequestedAt.Format(time.RFC3339),
		})
	}
	h.mu.Unlock()
	h.sendToClient(client, WSResponse{Type: "approval_list", Data: list})
}

func (h *Handler) handleApprovalHistoryWS(client *Client) {
	h.mu.Lock()
	history := make([]ApprovalRecord, len(h.approvalHistory))
	copy(history, h.approvalHistory)
	h.mu.Unlock()

	if len(history) > 50 {
		history = history[len(history)-50:]
	}
	h.sendToClient(client, WSResponse{Type: "approval_history", Data: history})
}

func (h *Handler) handleApprovalBulkWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid bulk approval request")
		return
	}
	action, _ := data["action"].(string)
	toolFilter, _ := data["tool_name"].(string)

	h.mu.Lock()
	count := 0
	for blockID, ch := range h.pendingApprovals {
		meta, hasMeta := h.pendingApprovalMeta[blockID]
		if toolFilter != "" && hasMeta && meta.ToolName != toolFilter {
			continue
		}
		decision := action == "approve_all"
		ch <- decision
		delete(h.pendingApprovals, blockID)
		delete(h.pendingApprovalMeta, blockID)

		toolName := ""
		if hasMeta {
			toolName = meta.ToolName
		}
		decisionStr := "denied"
		if decision {
			decisionStr = "approved"
		}
		h.approvalHistory = append(h.approvalHistory, ApprovalRecord{
			BlockID:   blockID,
			ToolName:  toolName,
			Decision:  decisionStr,
			Timestamp: time.Now(),
		})
		count++
	}
	h.mu.Unlock()

	h.sendToClient(client, WSResponse{Type: "approval_bulk", Data: map[string]any{
		"count":  count,
		"action": action,
	}})
}
