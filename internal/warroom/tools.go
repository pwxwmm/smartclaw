package warroom

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	tools.Register(&WarRoomHandoffTool{})
	tools.Register(&WarRoomEvaluateTool{})
	tools.Register(&WarRoomBlackboardWriteTool{})
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

type WarRoomHandoffTool struct{}

func (t *WarRoomHandoffTool) Name() string { return "warroom_handoff" }
func (t *WarRoomHandoffTool) Description() string {
	return "Request another agent in the War Room to investigate a specific question. Use when you need information from a different domain expert."
}
func (t *WarRoomHandoffTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "War room session ID",
			},
			"target_agent": map[string]any{
				"type":        "string",
				"description": "The agent to ask (network, database, infra, app, security, training, inference)",
			},
			"question": map[string]any{
				"type":        "string",
				"description": "The specific question to ask the other agent",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Additional context for the question",
			},
			"priority": map[string]any{
				"type":        "string",
				"description": "Priority level: low, medium, or high",
			},
		},
		"required": []string{"session_id", "target_agent", "question"},
	}
}

func (t *WarRoomHandoffTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	targetAgent, _ := input["target_agent"].(string)
	if targetAgent == "" {
		return nil, fmt.Errorf("target_agent is required")
	}
	question, _ := input["question"].(string)
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}
	contextStr, _ := input["context"].(string)
	priority, _ := input["priority"].(string)
	if priority == "" {
		priority = "medium"
	}

	coordinator := getCoordinator()
	req := HandoffRequest{
		FromAgent: AgentReasoning,
		ToAgent:   DomainAgentType(targetAgent),
		Question:  question,
		Context:   contextStr,
		Priority:  priority,
	}
	resp, err := coordinator.RequestHandoff(ctx, sessionID, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type WarRoomEvaluateTool struct{}

func (t *WarRoomEvaluateTool) Name() string { return "warroom_evaluate" }
func (t *WarRoomEvaluateTool) Description() string {
	return "Evaluate a finding from another agent. Agree or disagree with findings and adjust their confidence."
}
func (t *WarRoomEvaluateTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "War room session ID",
			},
			"finding_id": map[string]any{
				"type":        "string",
				"description": "The ID of the finding to evaluate",
			},
			"agrees": map[string]any{
				"type":        "boolean",
				"description": "Whether you agree with this finding",
			},
			"confidence_adjustment": map[string]any{
				"type":        "number",
				"description": "Confidence adjustment (-0.3 to +0.3)",
			},
			"notes": map[string]any{
				"type":        "string",
				"description": "Explanation for your evaluation",
			},
		},
		"required": []string{"session_id", "finding_id", "agrees"},
	}
}

func (t *WarRoomEvaluateTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	findingID, _ := input["finding_id"].(string)
	if findingID == "" {
		return nil, fmt.Errorf("finding_id is required")
	}
	agrees, _ := input["agrees"].(bool)
	adjustment, _ := input["confidence_adjustment"].(float64)
	notes, _ := input["notes"].(string)

	if adjustment == 0 {
		if agrees {
			adjustment = 0.05
		} else {
			adjustment = -0.05
		}
	}

	coordinator := getCoordinator()
	coordinator.EvolveConfidence(sessionID, findingID, adjustment, notes)

	session := coordinator.GetSession(sessionID)
	if session != nil {
		for i := range session.Findings {
			if session.Findings[i].ID == findingID {
				xref := CrossReference{
					FindingID:    findingID,
					ReferencedBy: AgentReasoning,
					Agrees:       agrees,
					Notes:        notes,
				}
				session.Findings[i].CrossReferences = append(session.Findings[i].CrossReferences, xref)
				break
			}
		}
	}

	return map[string]any{
		"finding_id":  findingID,
		"agrees":      agrees,
		"adjustment":  adjustment,
		"status":      "evaluated",
	}, nil
}

type WarRoomBlackboardWriteTool struct{}

func (t *WarRoomBlackboardWriteTool) Name() string { return "warroom_blackboard_write" }
func (t *WarRoomBlackboardWriteTool) Description() string {
	return "Write an observation or finding to the shared blackboard so all agents in the War Room can see it."
}
func (t *WarRoomBlackboardWriteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "War room session ID",
			},
			"key": map[string]any{
				"type":        "string",
				"description": "A short key for this observation (e.g. 'gpu_memory_status', 'error_pattern')",
			},
			"value": map[string]any{
				"type":        "string",
				"description": "The observation content",
			},
			"category": map[string]any{
				"type":        "string",
				"description": "Category: observation, metric, log_excerpt, hypothesis",
			},
		},
		"required": []string{"session_id", "key", "value"},
	}
}

func (t *WarRoomBlackboardWriteTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	sessionID, _ := input["session_id"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	key, _ := input["key"].(string)
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}
	value, _ := input["value"].(string)
	if value == "" {
		return nil, fmt.Errorf("value is required")
	}
	category, _ := input["category"].(string)
	if category == "" {
		category = "observation"
	}

	coordinator := getCoordinator()
	bb, ok := coordinator.GetBlackboard(sessionID)
	if !ok || bb == nil {
		return nil, fmt.Errorf("blackboard not found for session %s", sessionID)
	}

	bb.WriteEntry(BlackboardEntry{
		Key:       key,
		Value:     value,
		Author:    AgentReasoning,
		Category:  category,
		Timestamp: time.Now(),
	})

	return map[string]any{
		"session_id": sessionID,
		"key":        key,
		"category":   category,
		"status":     "written",
	}, nil
}
