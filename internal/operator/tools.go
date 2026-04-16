package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/tools"
)

var (
	defaultManagerMu sync.RWMutex
	defaultManager   *OperatorManager
)

func SetOperatorManager(m *OperatorManager) {
	defaultManagerMu.Lock()
	defer defaultManagerMu.Unlock()
	defaultManager = m
}

func DefaultOperatorManager() *OperatorManager {
	defaultManagerMu.RLock()
	defer defaultManagerMu.RUnlock()
	return defaultManager
}

type OperatorEnableTool struct{ tools.BaseTool }

func (t *OperatorEnableTool) Name() string { return "operator_enable" }

func (t *OperatorEnableTool) Description() string {
	return "Enable operator mode for autonomous 24/7 SRE monitoring with health checks, SLO monitoring, and auto-escalation."
}

func (t *OperatorEnableTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Operator name",
			},
			"schedule": map[string]any{
				"type":        "string",
				"description": "Main check cycle cron expression",
				"default":     "*/5 * * * *",
			},
			"autonomy_level": map[string]any{
				"type":        "string",
				"description": "Autonomy level: observe, suggest, auto, full",
				"default":     "suggest",
				"enum":        []string{"observe", "suggest", "auto", "full"},
			},
			"health_checks": map[string]any{
				"type":        "array",
				"description": "List of health check definitions",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":        map[string]any{"type": "string", "description": "Check ID"},
						"name":      map[string]any{"type": "string", "description": "Check name"},
						"type":      map[string]any{"type": "string", "description": "Check type: http, tcp, slo, alert, custom, topology"},
						"target":    map[string]any{"type": "string", "description": "Target: URL, host:port, service name, or command"},
						"schedule":  map[string]any{"type": "string", "description": "Per-check cron schedule (overrides main)"},
						"timeout":   map[string]any{"type": "integer", "description": "Timeout in seconds"},
						"threshold": map[string]any{"type": "number", "description": "Failure threshold"},
						"severity":  map[string]any{"type": "string", "description": "Alert severity if check fails"},
					},
				},
			},
			"escalation_policy": map[string]any{
				"type":        "object",
				"description": "Escalation policy with trigger conditions and actions",
			},
			"notify_channels": map[string]any{
				"type":        "array",
				"description": "Notification targets",
				"items":       map[string]any{"type": "string"},
			},
			"labels": map[string]any{
				"type":        "object",
				"description": "Additional labels",
			},
		},
		"required": []string{"name"},
	}
}

func (t *OperatorEnableTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	mgr := DefaultOperatorManager()
	if mgr == nil {
		return nil, fmt.Errorf("operator manager not initialized")
	}

	name, _ := input["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	schedule, _ := input["schedule"].(string)
	if schedule == "" {
		schedule = "*/5 * * * *"
	}

	autonomyStr, _ := input["autonomy_level"].(string)
	autonomy := AutonomyLevel(autonomyStr)
	if autonomy == "" {
		autonomy = AutonomySuggest
	}

	config := OperatorConfig{
		Name:           name,
		Enabled:        true,
		Schedule:       schedule,
		AutonomyLevel:  autonomy,
		MaxAutoActions: 3,
		Labels:         make(map[string]string),
	}

	if checksRaw, ok := input["health_checks"]; ok {
		checks, err := parseHealthChecks(checksRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid health_checks: %w", err)
		}
		config.HealthChecks = checks
	}

	if polRaw, ok := input["escalation_policy"]; ok {
		pol, err := parseEscalationPolicy(polRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid escalation_policy: %w", err)
		}
		config.EscalationPolicy = pol
	}

	if chRaw, ok := input["notify_channels"]; ok {
		if chArr, ok := chRaw.([]any); ok {
			for _, ch := range chArr {
				if s, ok := ch.(string); ok {
					config.NotifyChannels = append(config.NotifyChannels, s)
				}
			}
		}
	}

	if lblRaw, ok := input["labels"]; ok {
		if lblMap, ok := lblRaw.(map[string]any); ok {
			for k, v := range lblMap {
				if s, ok := v.(string); ok {
					config.Labels[k] = s
				}
			}
		}
	}

	result, err := mgr.Enable(ctx, config)
	if err != nil {
		return nil, err
	}

	status := mgr.GetStatus(result.ID)

	return map[string]any{
		"config": result,
		"status": status,
	}, nil
}

type OperatorDisableTool struct{ tools.BaseTool }

func (t *OperatorDisableTool) Name() string { return "operator_disable" }

func (t *OperatorDisableTool) Description() string {
	return "Disable operator mode. Stops health check scheduling and cleans up resources."
}

func (t *OperatorDisableTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"config_id": map[string]any{
				"type":        "string",
				"description": "Specific operator config to disable. If empty, disables all operators.",
			},
		},
	}
}

func (t *OperatorDisableTool) Execute(_ context.Context, input map[string]any) (any, error) {
	mgr := DefaultOperatorManager()
	if mgr == nil {
		return nil, fmt.Errorf("operator manager not initialized")
	}

	configID, _ := input["config_id"].(string)

	if configID != "" {
		if err := mgr.Disable(configID); err != nil {
			return nil, err
		}
		return map[string]any{
			"message":  fmt.Sprintf("operator %s disabled", configID),
			"disabled": []string{configID},
		}, nil
	}

	disabled := mgr.DisableAll()
	return map[string]any{
		"message":  fmt.Sprintf("disabled %d operator(s)", len(disabled)),
		"disabled": disabled,
	}, nil
}

func parseHealthChecks(raw any) ([]HealthCheckDef, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	var checks []struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Type      string  `json:"type"`
		Target    string  `json:"target"`
		Schedule  string  `json:"schedule"`
		Timeout   float64 `json:"timeout"`
		Threshold float64 `json:"threshold"`
		Severity  string  `json:"severity"`
	}

	if err := json.Unmarshal(data, &checks); err != nil {
		return nil, err
	}

	result := make([]HealthCheckDef, len(checks))
	for i, c := range checks {
		result[i] = HealthCheckDef{
			ID:        c.ID,
			Name:      c.Name,
			Type:      CheckType(c.Type),
			Target:    c.Target,
			Schedule:  c.Schedule,
			Timeout:   time.Duration(c.Timeout * float64(time.Second)),
			Threshold: c.Threshold,
			Severity:  c.Severity,
		}
	}
	return result, nil
}

func parseEscalationPolicy(raw any) (EscalationPolicy, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return EscalationPolicy{}, err
	}

	var policy EscalationPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return EscalationPolicy{}, err
	}
	return policy, nil
}

func RegisterOperatorTools(registry *tools.ToolRegistry) {
	registry.Register(&OperatorEnableTool{})
	registry.Register(&OperatorDisableTool{})
}
