package autoremediation

import (
	"context"
	"fmt"
	"sync"

	"github.com/instructkr/smartclaw/internal/tools"
)

var (
	defaultEngine   *RemediationEngine
	defaultEngineMu sync.RWMutex
)

func SetRemediationEngine(e *RemediationEngine) {
	defaultEngineMu.Lock()
	defer defaultEngineMu.Unlock()
	defaultEngine = e
}

func DefaultRemediationEngine() *RemediationEngine {
	defaultEngineMu.RLock()
	defer defaultEngineMu.RUnlock()
	return defaultEngine
}

func InitRemediationEngine(runbookDir string, sloProvider SLOProvider, commander Commander) *RemediationEngine {
	e := NewRemediationEngine(runbookDir)
	if sloProvider != nil {
		e.SetSLOProvider(sloProvider)
	}
	if commander != nil {
		e.SetCommander(commander)
	}
	if err := e.LoadRunbooks(); err != nil {
		// non-blocking — runbook loading failures are logged but don't prevent initialization
	}
	SetRemediationEngine(e)
	return e
}

func RegisterAllTools() {
	tools.Register(&RemediationSuggestTool{})
	tools.Register(&RemediationApproveTool{})
	tools.Register(&RemediationExecuteTool{})
}

type RemediationSuggestTool struct{ BaseRemediationTool }

type BaseRemediationTool struct{}

func (t *BaseRemediationTool) Name() string                { return "" }
func (t *BaseRemediationTool) Description() string         { return "" }
func (t *BaseRemediationTool) InputSchema() map[string]any { return nil }
func (t *BaseRemediationTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	return nil, nil
}

func (t *RemediationSuggestTool) Name() string { return "remediation_suggest" }

func (t *RemediationSuggestTool) Description() string {
	return "Suggest remediation runbooks for a service based on SLO status and trigger type."
}

func (t *RemediationSuggestTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"service":           map[string]any{"type": "string", "description": "Target service name"},
			"trigger":           map[string]any{"type": "string", "description": "Trigger type filter: slo_burn, alert_severity, metric_threshold, manual"},
			"autonomy_override": map[string]any{"type": "string", "description": "Force a specific autonomy level (suggest, pre_approved, auto, speculative)"},
		},
		"required": []string{"service"},
	}
}

func (t *RemediationSuggestTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	engine := DefaultRemediationEngine()
	if engine == nil {
		return nil, fmt.Errorf("remediation engine not initialized")
	}

	service, _ := input["service"].(string)
	if service == "" {
		return nil, fmt.Errorf("service is required")
	}

	trigger, _ := input["trigger"].(string)

	autonomyOverride, _ := input["autonomy_override"].(string)
	if autonomyOverride != "" {
		_ = AutonomyLevel(autonomyOverride)
	}

	assessment, assessErr := engine.AssessSLO(service)

	runbooks, err := engine.SuggestRemediation(service, trigger)
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"service":  service,
		"runbooks": runbooks,
	}

	if assessErr == nil && assessment != nil {
		result["assessment"] = assessment
		if autonomyOverride != "" {
			assessment.AutonomyLevel = AutonomyLevel(autonomyOverride)
		}
	}

	return result, nil
}

type RemediationApproveTool struct{ BaseRemediationTool }

func (t *RemediationApproveTool) Name() string { return "remediation_approve" }

func (t *RemediationApproveTool) Description() string {
	return "Approve a pending remediation action so it can be executed."
}

func (t *RemediationApproveTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action_id": map[string]any{"type": "string", "description": "The action ID to approve"},
			"approver":  map[string]any{"type": "string", "description": "Who approved this action"},
		},
		"required": []string{"action_id"},
	}
}

func (t *RemediationApproveTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	engine := DefaultRemediationEngine()
	if engine == nil {
		return nil, fmt.Errorf("remediation engine not initialized")
	}

	actionID, _ := input["action_id"].(string)
	if actionID == "" {
		return nil, fmt.Errorf("action_id is required")
	}

	approver, _ := input["approver"].(string)

	if err := engine.ApproveAction(actionID, approver); err != nil {
		return nil, err
	}

	action := engine.GetAction(actionID)
	return action, nil
}

type RemediationExecuteTool struct{ BaseRemediationTool }

func (t *RemediationExecuteTool) Name() string { return "remediation_execute" }

func (t *RemediationExecuteTool) Description() string {
	return "Execute a remediation runbook. If autonomy is sufficient, executes immediately; otherwise creates a pending action requiring approval."
}

func (t *RemediationExecuteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"runbook_id": map[string]any{"type": "string", "description": "Which runbook to execute"},
			"service":    map[string]any{"type": "string", "description": "Target service"},
			"trigger":    map[string]any{"type": "string", "description": "What triggered this remediation"},
			"parameters": map[string]any{"type": "object", "description": "Step parameters/overrides"},
		},
		"required": []string{"runbook_id", "service"},
	}
}

func (t *RemediationExecuteTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	engine := DefaultRemediationEngine()
	if engine == nil {
		return nil, fmt.Errorf("remediation engine not initialized")
	}

	runbookID, _ := input["runbook_id"].(string)
	if runbookID == "" {
		return nil, fmt.Errorf("runbook_id is required")
	}

	service, _ := input["service"].(string)
	if service == "" {
		return nil, fmt.Errorf("service is required")
	}

	trigger, _ := input["trigger"].(string)

	var autonomy AutonomyLevel = AutonomySuggest
	if assessment, err := engine.AssessSLO(service); err == nil && assessment != nil {
		autonomy = assessment.AutonomyLevel
	}

	action, err := engine.CreateAction(runbookID, service, trigger, autonomy)
	if err != nil {
		return nil, err
	}

	if autonomySufficient(autonomy, getRunbookAutonomy(engine, runbookID)) {
		result, err := engine.ExecuteAction(ctx, action.ID)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	return action, nil
}

func getRunbookAutonomy(engine *RemediationEngine, runbookID string) AutonomyLevel {
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	if rb, ok := engine.runbooks[runbookID]; ok {
		return rb.Autonomy
	}
	return AutonomySpeculative
}
