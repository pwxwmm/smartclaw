package web

import (
	"context"
	"encoding/json"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/warroom"
)

func (h *Handler) handleWarRoomStartWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid warroom start request")
		return
	}

	title, _ := data["title"].(string)
	if title == "" {
		h.sendError(client, "title is required")
		return
	}
	description, _ := data["description"].(string)
	if description == "" {
		h.sendError(client, "description is required")
		return
	}
	incidentID, _ := data["incident_id"].(string)

	var agentTypes []warroom.DomainAgentType
	if at, ok := data["agent_types"]; ok {
		if arr, ok := at.([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					agentTypes = append(agentTypes, warroom.DomainAgentType(s))
				}
			}
		}
	}

	var contextData map[string]any
	if ctxVal, ok := data["context"]; ok {
		if m, ok := ctxVal.(map[string]any); ok {
			contextData = m
		}
	}

	req := warroom.WarRoomRequest{
		IncidentID:  incidentID,
		Title:       title,
		Description: description,
		AgentTypes:  agentTypes,
		Context:     contextData,
	}

	coordinator := warroom.DefaultWarRoomCoordinator()
	if coordinator == nil {
		h.sendError(client, "War Room coordinator not initialized")
		return
	}

	session, err := coordinator.StartWarRoom(context.Background(), req)
	if err != nil {
		h.sendError(client, "War Room start failed: "+err.Error())
		return
	}

	h.sendToClient(client, WSResponse{
		Type: "warroom_started",
		Data: sessionToMap(session),
	})

	h.hub.Broadcast(mustMarshalWSResponse(WSResponse{
		Type: "warroom_update",
		Data: map[string]any{
			"event":      "session_created",
			"session_id": session.ID,
			"title":      session.Title,
			"status":     session.Status,
		},
	}))

	go h.pollWarRoomUpdates(session.ID)
}

func (h *Handler) handleWarRoomStatusWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid warroom status request")
		return
	}

	sessionID, _ := data["session_id"].(string)
	if sessionID == "" {
		h.sendError(client, "session_id is required")
		return
	}

	coordinator := warroom.DefaultWarRoomCoordinator()
	if coordinator == nil {
		h.sendError(client, "War Room coordinator not initialized")
		return
	}

	session := coordinator.GetSession(sessionID)
	if session == nil {
		h.sendError(client, "session not found: "+sessionID)
		return
	}

	h.sendToClient(client, WSResponse{
		Type: "warroom_status",
		Data: sessionToMap(session),
	})
}

func (h *Handler) handleWarRoomStopWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid warroom stop request")
		return
	}

	sessionID, _ := data["session_id"].(string)
	if sessionID == "" {
		h.sendError(client, "session_id is required")
		return
	}

	coordinator := warroom.DefaultWarRoomCoordinator()
	if coordinator == nil {
		h.sendError(client, "War Room coordinator not initialized")
		return
	}

	result, err := coordinator.CloseSession(sessionID)
	if err != nil {
		h.sendError(client, "War Room stop failed: "+err.Error())
		return
	}

	h.sendToClient(client, WSResponse{
		Type: "warroom_stopped",
		Data: investigationResultToMap(result),
	})

	h.hub.Broadcast(mustMarshalWSResponse(WSResponse{
		Type: "warroom_update",
		Data: map[string]any{
			"event":      "session_closed",
			"session_id": sessionID,
			"status":     "closed",
		},
	}))
}

func (h *Handler) handleWarRoomListWS(client *Client) {
	coordinator := warroom.DefaultWarRoomCoordinator()
	if coordinator == nil {
		h.sendToClient(client, WSResponse{
			Type: "warroom_list",
			Data: map[string]any{"sessions": []any{}},
		})
		return
	}

	sessions := coordinator.ListSessions()
	list := make([]any, 0, len(sessions))
	for _, s := range sessions {
		list = append(list, sessionToMap(s))
	}

	h.sendToClient(client, WSResponse{
		Type: "warroom_list",
		Data: map[string]any{"sessions": list},
	})
}

func (h *Handler) handleWarRoomAssignTaskWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid warroom assign task request")
		return
	}

	sessionID, _ := data["session_id"].(string)
	agentType, _ := data["agent_type"].(string)
	task, _ := data["task"].(string)

	if sessionID == "" || agentType == "" || task == "" {
		h.sendError(client, "session_id, agent_type, and task are required")
		return
	}

	coordinator := warroom.DefaultWarRoomCoordinator()
	if coordinator == nil {
		h.sendError(client, "War Room coordinator not initialized")
		return
	}

	err := coordinator.AssignTask(context.Background(), sessionID, warroom.DomainAgentType(agentType), task)
	if err != nil {
		h.sendError(client, "Assign task failed: "+err.Error())
		return
	}

	h.sendToClient(client, WSResponse{
		Type: "warroom_task_assigned",
		Data: map[string]any{
			"session_id": sessionID,
			"agent_type": agentType,
			"task":       task,
		},
	})
}

func (h *Handler) handleWarRoomBroadcastWS(client *Client, msg WSMessage) {
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid warroom broadcast request")
		return
	}

	sessionID, _ := data["session_id"].(string)
	message, _ := data["message"].(string)

	if sessionID == "" || message == "" {
		h.sendError(client, "session_id and message are required")
		return
	}

	coordinator := warroom.DefaultWarRoomCoordinator()
	if coordinator == nil {
		h.sendError(client, "War Room coordinator not initialized")
		return
	}

	err := coordinator.Broadcast(context.Background(), sessionID, message)
	if err != nil {
		h.sendError(client, "Broadcast failed: "+err.Error())
		return
	}

	h.sendToClient(client, WSResponse{
		Type: "warroom_broadcast_sent",
		Data: map[string]any{
			"session_id": sessionID,
			"message":    message,
		},
	})
}

func (h *Handler) pollWarRoomUpdates(sessionID string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		coordinator := warroom.DefaultWarRoomCoordinator()
		if coordinator == nil {
			return
		}

		session := coordinator.GetSession(sessionID)
		if session == nil || session.Status == warroom.WarRoomClosed {
			return
		}

		for _, a := range session.Agents {
			h.hub.Broadcast(mustMarshalWSResponse(WSResponse{
				Type: "warroom_agent_status",
				Data: map[string]any{
					"session_id": sessionID,
					"agent_type": string(a.AgentType),
					"status":     string(a.Status),
					"last_active": a.LastActive,
					"findings":   len(a.Findings),
				},
			}))
		}

		if len(session.Findings) > 0 {
			latestFindings := session.Findings
			if len(latestFindings) > 5 {
				latestFindings = latestFindings[len(latestFindings)-5:]
			}
			findings := make([]any, 0, len(latestFindings))
			for _, f := range latestFindings {
				findings = append(findings, findingToMap(&f))
			}
			h.hub.Broadcast(mustMarshalWSResponse(WSResponse{
				Type: "warroom_findings",
				Data: map[string]any{
					"session_id": sessionID,
					"findings":   findings,
				},
			}))
		}

		if len(session.Timeline) > 0 {
			latestEntries := session.Timeline
			if len(latestEntries) > 5 {
				latestEntries = latestEntries[len(latestEntries)-5:]
			}
			entries := make([]any, 0, len(latestEntries))
			for _, e := range latestEntries {
				entries = append(entries, map[string]any{
					"timestamp":  e.Timestamp,
					"agent_type": string(e.AgentType),
					"event":      e.Event,
					"details":    e.Details,
				})
			}
			h.hub.Broadcast(mustMarshalWSResponse(WSResponse{
				Type: "warroom_timeline",
				Data: map[string]any{
					"session_id": sessionID,
					"entries":    entries,
				},
			}))
		}
	}
}

func sessionToMap(s *warroom.WarRoomSession) map[string]any {
	if s == nil {
		return nil
	}
	agents := make([]any, 0, len(s.Agents))
	for _, a := range s.Agents {
		findings := make([]any, 0, len(a.Findings))
		for _, f := range a.Findings {
			findings = append(findings, findingToMap(&f))
		}
		agents = append(agents, map[string]any{
			"agent_type":  string(a.AgentType),
			"status":      string(a.Status),
			"assigned_at": a.AssignedAt,
			"last_active": a.LastActive,
			"findings":    findings,
		})
	}
	findings := make([]any, 0, len(s.Findings))
	for _, f := range s.Findings {
		findings = append(findings, findingToMap(&f))
	}
	timeline := make([]any, 0, len(s.Timeline))
	for _, e := range s.Timeline {
		timeline = append(timeline, map[string]any{
			"timestamp":  e.Timestamp,
			"agent_type": string(e.AgentType),
			"event":      e.Event,
			"details":    e.Details,
		})
	}
	return map[string]any{
		"id":          s.ID,
		"incident_id": s.IncidentID,
		"title":       s.Title,
		"description": s.Description,
		"status":      string(s.Status),
		"agents":      agents,
		"findings":    findings,
		"timeline":    timeline,
		"created_at":  s.CreatedAt,
		"closed_at":   s.ClosedAt,
		"context":     s.Context,
	}
}

func findingToMap(f *warroom.Finding) map[string]any {
	if f == nil {
		return nil
	}
	return map[string]any{
		"id":          f.ID,
		"agent_type":  string(f.AgentType),
		"category":    f.Category,
		"title":       f.Title,
		"description": f.Description,
		"confidence":  f.Confidence,
		"evidence":    f.Evidence,
		"created_at":  f.CreatedAt,
	}
}

func investigationResultToMap(r *warroom.InvestigationResult) map[string]any {
	if r == nil {
		return nil
	}
	agentReports := make(map[string]string, len(r.AgentReports))
	for k, v := range r.AgentReports {
		agentReports[string(k)] = v
	}
	findings := make([]any, 0, len(r.AllFindings))
	for _, f := range r.AllFindings {
		findings = append(findings, findingToMap(&f))
	}
	return map[string]any{
		"session_id":      r.SessionID,
		"summary":         r.Summary,
		"root_cause":      findingToMap(r.RootCause),
		"all_findings":    findings,
		"agent_reports":   agentReports,
		"recommendations": r.Recommendations,
		"duration":        r.Duration.String(),
	}
}

func initWarRoom() {
	coordinator := warroom.DefaultWarRoomCoordinator()
	if coordinator == nil {
		warroom.InitWarRoom(nil)
	}
}

func initWarRoomWithClient(client *api.Client) {
	if client == nil {
		initWarRoom()
		return
	}
	warroom.InitWarRoomWithLLM(client)
}

func (h *Handler) initWarRoomIfNeeded() {
	initWarRoomWithClient(h.apiClient)
}
