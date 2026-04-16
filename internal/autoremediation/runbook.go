package autoremediation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var BuiltInRunbooks = []Runbook{
	{
		ID:          "restart-service",
		Name:        "Restart Unhealthy Service",
		Description: "Restarts a service that is failing health checks",
		Service:     "*",
		Trigger:     RunbookTrigger{Type: "alert_severity", Severity: "high"},
		Steps: []RunbookStep{
			{ID: "check-health", Name: "Check service health", Type: StepTool, Action: "bash", Timeout: 30 * time.Second, OnFailure: FailureStop},
			{ID: "restart", Name: "Restart service", Type: StepCommand, Action: "kubectl rollout restart deployment/{{service}} -n {{namespace}}", Timeout: 60 * time.Second, Rollback: "kubectl rollout undo deployment/{{service}} -n {{namespace}}", OnFailure: FailureRollback},
			{ID: "verify", Name: "Verify service healthy", Type: StepTool, Action: "bash", Timeout: 120 * time.Second, OnFailure: FailureContinue},
		},
		Autonomy: AutonomyPreApproved,
		Timeout:  5 * time.Minute,
	},
	{
		ID:          "scale-up",
		Name:        "Scale Up Service",
		Description: "Scales up a service to handle increased load",
		Service:     "*",
		Trigger:     RunbookTrigger{Type: "metric_threshold", Metric: "cpu_utilization", Operator: "gt", Value: 0.85},
		Steps: []RunbookStep{
			{ID: "check-current", Name: "Check current replicas", Type: StepCommand, Action: "kubectl get deployment {{service}} -o jsonpath='{.spec.replicas}'", Timeout: 30 * time.Second, OnFailure: FailureStop},
			{ID: "scale", Name: "Scale up replicas", Type: StepCommand, Action: "kubectl scale deployment {{service}} --replicas=$((current+1))", Timeout: 60 * time.Second, Rollback: "kubectl scale deployment {{service}} --replicas={{original_replicas}}", OnFailure: FailureRollback},
			{ID: "verify", Name: "Verify load balanced", Type: StepCommand, Action: "kubectl get pods -l app={{service}} --field-selector=status.phase=Running", Timeout: 120 * time.Second, OnFailure: FailureContinue},
		},
		Autonomy: AutonomyAuto,
		Timeout:  10 * time.Minute,
	},
	{
		ID:          "investigate-errors",
		Name:        "Investigate Error Spike",
		Description: "AI-driven investigation of error rate spike",
		Service:     "*",
		Trigger:     RunbookTrigger{Type: "slo_burn", BurnRate: 3.0},
		Steps: []RunbookStep{
			{ID: "check-logs", Name: "Check recent error logs", Type: StepTool, Action: "bash", Timeout: 30 * time.Second, OnFailure: FailureContinue},
			{ID: "check-metrics", Name: "Check service metrics", Type: StepTool, Action: "bash", Timeout: 30 * time.Second, OnFailure: FailureContinue},
			{ID: "analyze", Name: "AI analysis", Type: StepPrompt, Action: "Analyze the error patterns and identify root cause. Logs: {{logs_output}} Metrics: {{metrics_output}}", Timeout: 120 * time.Second, OnFailure: FailureContinue},
		},
		Autonomy: AutonomySuggest,
		Timeout:  5 * time.Minute,
	},
}

func LoadRunbooksFromDir(dir string) (map[string]*Runbook, error) {
	runbooks := make(map[string]*Runbook)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return runbooks, nil
		}
		return nil, fmt.Errorf("autoremediation: read runbook dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var rb Runbook
		if err := json.Unmarshal(data, &rb); err != nil {
			continue
		}

		if rb.ID == "" {
			continue
		}

		runbooks[rb.ID] = &rb
	}

	return runbooks, nil
}

func SaveRunbookToDir(dir string, runbook *Runbook) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("autoremediation: create runbook dir: %w", err)
	}

	data, err := json.MarshalIndent(runbook, "", "  ")
	if err != nil {
		return fmt.Errorf("autoremediation: marshal runbook: %w", err)
	}

	path := filepath.Join(dir, runbook.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("autoremediation: write runbook file: %w", err)
	}

	return nil
}

func EnsureBuiltInRunbooks(dir string, existing map[string]*Runbook) int {
	saved := 0
	for i := range BuiltInRunbooks {
		builtin := BuiltInRunbooks[i]
		if _, exists := existing[builtin.ID]; exists {
			continue
		}
		now := time.Now().UTC()
		builtin.CreatedAt = now
		builtin.UpdatedAt = now
		if err := SaveRunbookToDir(dir, &builtin); err != nil {
			continue
		}
		saved++
	}
	return saved
}
