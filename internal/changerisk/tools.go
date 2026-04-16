package changerisk

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/tools"
)

var (
	defaultCheckerMu sync.RWMutex
	defaultChecker   *ChangeRiskChecker
)

func SetChangeRiskChecker(c *ChangeRiskChecker) {
	defaultCheckerMu.Lock()
	defer defaultCheckerMu.Unlock()
	defaultChecker = c
}

func DefaultChangeRiskChecker() *ChangeRiskChecker {
	defaultCheckerMu.RLock()
	defer defaultCheckerMu.RUnlock()
	return defaultChecker
}

func getChecker() *ChangeRiskChecker {
	defaultCheckerMu.RLock()
	c := defaultChecker
	defaultCheckerMu.RUnlock()
	if c != nil {
		return c
	}
	c = NewChangeRiskChecker()
	SetChangeRiskChecker(c)
	return c
}

type RiskPreflightTool struct{ tools.BaseTool }

func (t *RiskPreflightTool) Name() string { return "risk_preflight" }

func (t *RiskPreflightTool) Description() string {
	return "Pre-flight risk assessment for a proposed change. Evaluates blast radius, recent incidents, SLO burn rates, and change failure history before deployment or configuration changes."
}

func (t *RiskPreflightTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{
				"type":        "string",
				"description": "Type of change: deployment, config_change, scaling, rollback, hotfix, migration",
				"enum":        []string{"deployment", "config_change", "scaling", "rollback", "hotfix", "migration"},
			},
			"service": map[string]any{
				"type":        "string",
				"description": "Primary service being changed",
			},
			"services": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "All affected services (including primary)",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Description of the change",
			},
			"requester": map[string]any{
				"type":        "string",
				"description": "Who is making the change",
			},
			"priority": map[string]any{
				"type":        "string",
				"description": "Priority level: emergency, high, normal, low",
				"enum":        []string{"emergency", "high", "normal", "low"},
			},
			"scheduled_at": map[string]any{
				"type":        "string",
				"description": "Scheduled time in RFC3339 format",
			},
			"labels": map[string]any{
				"type":        "object",
				"description": "Additional labels (e.g. freeze=true for change freeze)",
			},
		},
		"required": []string{"type", "service"},
	}
}

func (t *RiskPreflightTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	changeType, _ := input["type"].(string)
	if changeType == "" {
		return nil, fmt.Errorf("'type' is required")
	}

	service, _ := input["service"].(string)
	if service == "" {
		return nil, fmt.Errorf("'service' is required")
	}

	req := ChangeRequest{
		ID:          fmt.Sprintf("cr-%d", time.Now().UnixNano()),
		Type:        ChangeType(changeType),
		Service:     service,
		Description: getString(input, "description"),
		Requester:   getString(input, "requester"),
		Priority:    getString(input, "priority"),
		Labels:      make(map[string]string),
	}

	if services, ok := input["services"]; ok {
		if arr, ok := services.([]any); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok && s != "" {
					req.Services = append(req.Services, s)
				}
			}
		}
	}

	if scheduledAt, ok := input["scheduled_at"].(string); ok && scheduledAt != "" {
		t, err := time.Parse(time.RFC3339, scheduledAt)
		if err == nil {
			req.ScheduledAt = &t
		}
	}

	if labels, ok := input["labels"]; ok {
		if m, ok := labels.(map[string]any); ok {
			for k, v := range m {
				if s, ok := v.(string); ok {
					req.Labels[k] = s
				}
			}
		}
	}

	checker := getChecker()
	assessment, err := checker.Assess(req)
	if err != nil {
		return nil, err
	}
	return assessment, nil
}

type RiskHistoryTool struct{ tools.BaseTool }

func (t *RiskHistoryTool) Name() string { return "risk_history" }

func (t *RiskHistoryTool) Description() string {
	return "Query change risk history. Returns past change records with risk assessments and outcomes, optionally filtered by service."
}

func (t *RiskHistoryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"service": map[string]any{
				"type":        "string",
				"description": "Filter by service name",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum records to return (default 20)",
				"default":     20,
			},
		},
	}
}

func (t *RiskHistoryTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	service, _ := input["service"].(string)
	limit := 20
	if v, ok := input["limit"]; ok {
		switch val := v.(type) {
		case float64:
			limit = int(val)
		case int:
			limit = val
		case string:
			if n, err := strconv.Atoi(val); err == nil {
				limit = n
			}
		}
	}

	checker := getChecker()
	records := checker.GetHistory(service, limit)
	return records, nil
}

func getString(input map[string]any, key string) string {
	v, _ := input[key].(string)
	return v
}

func RegisterTools(registry *tools.ToolRegistry) {
	registry.Register(&RiskPreflightTool{})
	registry.Register(&RiskHistoryTool{})
}
