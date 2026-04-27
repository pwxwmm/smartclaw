package web

import (
	"context"
	"encoding/json"

	"github.com/instructkr/smartclaw/internal/tools"
)

func (h *Handler) handleAgentListWS(client *Client) {
	result, err := tools.Execute(context.Background(), "agent", map[string]any{"operation": "list"})
	if err != nil {
		h.sendToClient(client, WSResponse{Type: "agent_list", Data: map[string]any{
			"agents": []any{},
			"count":  0,
		}})
		return
	}
	h.sendToClient(client, WSResponse{Type: "agent_list", Data: result})

	if agentList, ok := result.(map[string]any); ok {
		if agents, ok := agentList["agents"].([]any); ok {
			for _, a := range agents {
				if agent, ok := a.(map[string]any); ok {
					agentID, _ := agent["agent_id"].(string)
					status, _ := agent["status"].(string)
					if agentID != "" {
						h.sendToClient(client, WSResponse{
							Type: "agent_status",
							Data: map[string]any{
								"status":  status,
								"agentId": agentID,
							},
						})
					}
				}
			}
		}
	}
}

func (h *Handler) broadcastAgentStatus(agentID, status string) {
	resp := WSResponse{
		Type: "agent_status",
		Data: map[string]any{
			"status":  status,
			"agentId": agentID,
		},
	}
	h.hub.Broadcast(mustMarshalWSResponse(resp))
}

func (h *Handler) handleAgentStopWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid agent stop request")
		return
	}
	agentID, _ := data["agent_id"].(string)
	if agentID == "" {
		h.sendError(client, "agent_id is required")
		return
	}

	result, err := tools.Execute(context.Background(), "agent", map[string]any{
		"operation": "stop",
		"agent_id":  agentID,
	})
	if err != nil {
		h.sendToClient(client, WSResponse{Type: "agent_stop", Data: map[string]any{
			"success":  false,
			"agent_id": agentID,
			"error":    err.Error(),
		}})
		h.broadcastAgentStatus(agentID, "error")
		return
	}
	h.sendToClient(client, WSResponse{Type: "agent_stop", Data: result})
	h.broadcastAgentStatus(agentID, "done")
}

func (h *Handler) handleAgentOutputWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid agent output request")
		return
	}
	agentID, _ := data["agent_id"].(string)
	if agentID == "" {
		h.sendError(client, "agent_id is required")
		return
	}

	result, err := tools.Execute(context.Background(), "agent", map[string]any{
		"operation": "output",
		"agent_id":  agentID,
	})
	if err != nil {
		h.sendToClient(client, WSResponse{Type: "agent_output", Data: map[string]any{
			"agent_id": agentID,
			"error":    err.Error(),
		}})
		return
	}
	h.sendToClient(client, WSResponse{Type: "agent_output", Data: result})
}
