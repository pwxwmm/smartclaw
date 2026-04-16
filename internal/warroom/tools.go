package warroom

import (
	"context"
	"fmt"
	"sync"

	"github.com/instructkr/smartclaw/internal/tools"
)

var (
	defaultCoordinatorMu sync.RWMutex
	defaultCoordinator   *WarRoomCoordinator
)

func SetWarRoomCoordinator(c *WarRoomCoordinator) {
	defaultCoordinatorMu.Lock()
	defer defaultCoordinatorMu.Unlock()
	defaultCoordinator = c
}

func DefaultWarRoomCoordinator() *WarRoomCoordinator {
	defaultCoordinatorMu.RLock()
	defer defaultCoordinatorMu.RUnlock()
	return defaultCoordinator
}

func getCoordinator() *WarRoomCoordinator {
	c := DefaultWarRoomCoordinator()
	if c == nil {
		c = NewWarRoomCoordinator()
		SetWarRoomCoordinator(c)
	}
	return c
}

func RegisterAllTools() {
	tools.Register(&WarRoomStartTool{})
	tools.Register(&WarRoomStatusTool{})
	tools.Register(&WarRoomStopTool{})
}

type WarRoomStartTool struct{}

func (t *WarRoomStartTool) Name() string { return "warroom_start" }
func (t *WarRoomStartTool) Description() string {
	return "Start a war room session with domain-specialized sub-agents (network, database, infra, app, security) to collaboratively investigate an incident."
}

func (t *WarRoomStartTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{
				"type":        "string",
				"description": "War room title",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "What is being investigated",
			},
			"incident_id": map[string]any{
				"type":        "string",
				"description": "Link to incident ID (optional)",
			},
			"agent_types": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Agent types to include (default: all 5: network, database, infra, app, security)",
			},
			"context": map[string]any{
				"type":        "object",
				"description": "Shared context for all agents",
			},
		},
		"required": []string{"title", "description"},
	}
}

func (t *WarRoomStartTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	title, _ := input["title"].(string)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	description, _ := input["description"].(string)
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}
	incidentID, _ := input["incident_id"].(string)

	var agentTypes []DomainAgentType
	if at, ok := input["agent_types"]; ok {
		if arr, ok := at.([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					agentTypes = append(agentTypes, DomainAgentType(s))
				}
			}
		}
	}

	var contextData map[string]any
	if ctxVal, ok := input["context"]; ok {
		if m, ok := ctxVal.(map[string]any); ok {
			contextData = m
		}
	}

	req := WarRoomRequest{
		IncidentID:  incidentID,
		Title:       title,
		Description: description,
		AgentTypes:  agentTypes,
		Context:     contextData,
	}

	coordinator := getCoordinator()
	session, err := coordinator.StartWarRoom(ctx, req)
	if err != nil {
		return nil, err
	}
	return session, nil
}

type WarRoomStatusTool struct{}

func (t *WarRoomStatusTool) Name() string { return "warroom_status" }
func (t *WarRoomStatusTool) Description() string {
	return "Check the status of a war room session including all findings and timeline."
}

func (t *WarRoomStatusTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "War room session ID",
			},
		},
		"required": []string{"session_id"},
	}
}

func (t *WarRoomStatusTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	coordinator := getCoordinator()
	session := coordinator.GetSession(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return session, nil
}

type WarRoomStopTool struct{}

func (t *WarRoomStopTool) Name() string { return "warroom_stop" }
func (t *WarRoomStopTool) Description() string {
	return "Stop a war room session and get the final investigation result with root cause analysis and recommendations."
}

func (t *WarRoomStopTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "War room session ID",
			},
		},
		"required": []string{"session_id"},
	}
}

func (t *WarRoomStopTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	coordinator := getCoordinator()
	result, err := coordinator.CloseSession(sessionID)
	if err != nil {
		return nil, err
	}
	return result, nil
}
