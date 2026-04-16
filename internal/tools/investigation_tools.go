package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/memory/layers"
)

var (
	incidentMemoryMu   sync.RWMutex
	incidentMemoryInst *layers.IncidentMemory
)

func SetIncidentMemory(im *layers.IncidentMemory) {
	incidentMemoryMu.Lock()
	defer incidentMemoryMu.Unlock()
	incidentMemoryInst = im
}

func getIncidentMemory() *layers.IncidentMemory {
	incidentMemoryMu.RLock()
	defer incidentMemoryMu.RUnlock()
	return incidentMemoryInst
}

type InvestigateIncidentTool struct{ BaseTool }

func (t *InvestigateIncidentTool) Name() string { return "investigate_incident" }
func (t *InvestigateIncidentTool) Description() string {
	return "Hypothesis-driven incident investigation. Form a hypothesis about root cause, then validate it by executing evidence queries (SOPA tool calls) and analyzing results. Use this to structure your investigation before gathering evidence."
}

func (t *InvestigateIncidentTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"incident_id": map[string]any{
				"type":        "string",
				"description": "ID of the incident to investigate",
			},
			"hypothesis": map[string]any{
				"type":        "string",
				"description": "Your hypothesis about the root cause of this incident",
			},
			"confidence": map[string]any{
				"type":        "number",
				"description": "Initial confidence level (0.0 to 1.0)",
				"default":     0.5,
			},
			"evidence_queries": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of SOPA tool queries to validate this hypothesis (e.g. 'sopa_node_logs for db-primary', 'sopa_get_node for api-gateway')",
			},
		},
		"required": []string{"incident_id", "hypothesis"},
	}
}

func (t *InvestigateIncidentTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	incidentID, _ := input["incident_id"].(string)
	hypothesis, _ := input["hypothesis"].(string)
	confidence, _ := input["confidence"].(float64)
	if confidence == 0 {
		confidence = 0.5
	}

	var evidenceQueries []string
	if eq, ok := input["evidence_queries"]; ok {
		if arr, ok := eq.([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					evidenceQueries = append(evidenceQueries, s)
				}
			}
		}
	}

	if incidentID == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	if hypothesis == "" {
		return nil, fmt.Errorf("hypothesis is required")
	}

	im := getIncidentMemory()
	if im != nil {
		hypothesisData := map[string]any{
			"hypothesis":       hypothesis,
			"confidence":       confidence,
			"evidence_queries": evidenceQueries,
		}
		content, _ := json.Marshal(hypothesisData)
		_ = im.AddTimelineEvent(ctx, incidentID, layers.TimelineEvent{
			Timestamp: time.Now().UTC(),
			Type:      "hypothesis",
			Content:   string(content),
			Source:    "agent",
		})
	}

	evidence := make([]map[string]any, 0, len(evidenceQueries))
	for _, q := range evidenceQueries {
		evidence = append(evidence, map[string]any{
			"query":    q,
			"finding":  "",
			"supports": nil,
		})
	}

	result := map[string]any{
		"incident_id":    incidentID,
		"hypothesis":     hypothesis,
		"confidence":     confidence,
		"evidence":       evidence,
		"recommendation": "",
		"next_steps":     "Execute the evidence_queries using SOPA tools (sopa_node_logs, sopa_get_node, sopa_list_faults, etc.) to gather findings, then call investigate_incident again with updated confidence and findings.",
	}

	return result, nil
}

// IncidentTimelineTool records timeline events for active incident investigations.
type IncidentTimelineTool struct{ BaseTool }

func (t *IncidentTimelineTool) Name() string { return "incident_timeline" }
func (t *IncidentTimelineTool) Description() string {
	return "Record a timeline event for an active incident investigation. Use this to build a chronological record of what happened during incident response. Actions: alert, hypothesis, evidence, action, escalation, mitigation, resolution."
}

func (t *IncidentTimelineTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"incident_id": map[string]any{
				"type":        "string",
				"description": "ID of the incident",
			},
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"alert", "hypothesis", "evidence", "action", "escalation", "mitigation", "resolution"},
				"description": "Type of timeline event",
			},
			"details": map[string]any{
				"type":        "string",
				"description": "Description of what happened",
			},
			"source": map[string]any{
				"type":        "string",
				"description": "Source of this event (e.g. agent, user, alert-system)",
				"default":     "agent",
			},
		},
		"required": []string{"incident_id", "action", "details"},
	}
}

func (t *IncidentTimelineTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	incidentID, _ := input["incident_id"].(string)
	action, _ := input["action"].(string)
	details, _ := input["details"].(string)
	source, _ := input["source"].(string)
	if source == "" {
		source = "agent"
	}

	if incidentID == "" {
		return nil, fmt.Errorf("incident_id is required")
	}
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}
	if details == "" {
		return nil, fmt.Errorf("details is required")
	}

	validActions := map[string]bool{
		"alert": true, "hypothesis": true, "evidence": true,
		"action": true, "escalation": true, "mitigation": true, "resolution": true,
	}
	if !validActions[action] {
		return nil, fmt.Errorf("invalid action '%s'; must be one of: alert, hypothesis, evidence, action, escalation, mitigation, resolution", action)
	}

	im := getIncidentMemory()
	if im != nil {
		err := im.AddTimelineEvent(ctx, incidentID, layers.TimelineEvent{
			Timestamp: time.Now().UTC(),
			Type:      action,
			Content:   details,
			Source:    source,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to add timeline event: %w", err)
		}

		inc, _ := im.GetIncident(incidentID)
		eventCount := 0
		if inc != nil {
			eventCount = len(inc.TimelineEvents)
		}

		return map[string]any{
			"status":      "recorded",
			"incident_id": incidentID,
			"action":      action,
			"event_count": eventCount,
		}, nil
	}

	return map[string]any{
		"status":      "recorded_no_store",
		"incident_id": incidentID,
		"action":      action,
		"note":        "Incident memory not available; event was not persisted",
	}, nil
}
