package autoremediation

import (
	"time"
)

// StepType defines the type of a runbook step.
type StepType string

const (
	StepCommand  StepType = "command"  // shell command
	StepAPI      StepType = "api"      // HTTP API call
	StepTool     StepType = "tool"     // SmartClaw tool invocation
	StepPrompt   StepType = "prompt"   // AI prompt for investigation
	StepApproval StepType = "approval" // wait for human approval
)

// FailurePolicy defines what to do when a step fails.
type FailurePolicy string

const (
	FailureStop     FailurePolicy = "stop"
	FailureContinue FailurePolicy = "continue"
	FailureRollback FailurePolicy = "rollback"
	FailureSkip     FailurePolicy = "skip"
)

// AutonomyLevel defines how autonomous the remediation engine can act.
type AutonomyLevel string

const (
	AutonomySuggest     AutonomyLevel = "suggest"      // suggest actions, require approval
	AutonomyPreApproved AutonomyLevel = "pre_approved" // execute pre-approved runbooks
	AutonomyAuto        AutonomyLevel = "auto"         // auto-execute with rollback capability
	AutonomySpeculative AutonomyLevel = "speculative"  // execute even uncertain remediations
)

// ActionStatus defines the status of a remediation action.
type ActionStatus string

const (
	ActionPending    ActionStatus = "pending"
	ActionApproved   ActionStatus = "approved"
	ActionRunning    ActionStatus = "running"
	ActionSuccess    ActionStatus = "success"
	ActionFailed     ActionStatus = "failed"
	ActionRolledBack ActionStatus = "rolled_back"
	ActionCancelled  ActionStatus = "cancelled"
)

// RunbookTrigger defines when a runbook should be triggered.
type RunbookTrigger struct {
	Type     string  `json:"type"`                // slo_burn, alert_severity, metric_threshold, manual
	Metric   string  `json:"metric"`              // e.g., "error_rate", "latency_p99", "availability"
	Operator string  `json:"operator"`            // gt, lt, gte, lte, eq
	Value    float64 `json:"value"`               // threshold value
	BurnRate float64 `json:"burn_rate,omitempty"` // for slo_burn triggers
	Severity string  `json:"severity,omitempty"`  // for alert_severity triggers
}

// RunbookStep defines a single step in a runbook.
type RunbookStep struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Type        StepType       `json:"type"`
	Action      string         `json:"action"` // the actual command/API call/prompt
	Timeout     time.Duration  `json:"timeout"`
	Rollback    string         `json:"rollback,omitempty"` // rollback action if this step fails
	OnFailure   FailurePolicy  `json:"on_failure"`         // stop, continue, rollback, skip
	Parameters  map[string]any `json:"parameters"`
}

// Runbook defines a complete remediation runbook.
type Runbook struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Service     string            `json:"service"`
	Trigger     RunbookTrigger    `json:"trigger"`
	Steps       []RunbookStep     `json:"steps"`
	Autonomy    AutonomyLevel     `json:"autonomy"` // minimum autonomy level required
	Timeout     time.Duration     `json:"timeout"`  // total timeout for all steps
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// StepResult records the outcome of a single runbook step execution.
type StepResult struct {
	StepID      string        `json:"step_id"`
	StepName    string        `json:"step_name"`
	Status      ActionStatus  `json:"status"`
	Output      string        `json:"output"`
	Error       string        `json:"error,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration"`
}

// RemediationAction represents an instantiated remediation in progress or completed.
type RemediationAction struct {
	ID          string        `json:"id"`
	RunbookID   string        `json:"runbook_id"`
	Service     string        `json:"service"`
	Trigger     string        `json:"trigger"`
	Autonomy    AutonomyLevel `json:"autonomy"`
	Status      ActionStatus  `json:"status"`
	Steps       []StepResult  `json:"steps"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	ApprovedBy  string        `json:"approved_by,omitempty"`
}

// RemediationResult is the final result of executing a remediation action.
type RemediationResult struct {
	ActionID       string        `json:"action_id"`
	Status         ActionStatus  `json:"status"`
	Steps          []StepResult  `json:"steps"`
	Summary        string        `json:"summary"`
	RollbackNeeded bool          `json:"rollback_needed"`
	Duration       time.Duration `json:"duration"`
}

// SLOAssessment evaluates the current SLO status for a service and recommends actions.
type SLOAssessment struct {
	Service             string        `json:"service"`
	SLOName             string        `json:"slo_name"`
	BurnRate            float64       `json:"burn_rate"`
	ErrorBudgetLeft     float64       `json:"error_budget_left"`
	AutonomyLevel       AutonomyLevel `json:"autonomy_level"`
	RecommendedRunbooks []string      `json:"recommended_runbooks"`
}

// SLOInfo provides SLO status information for a service.
type SLOInfo struct {
	Service              string  `json:"service"`
	SLOName              string  `json:"slo_name"`
	Target               float64 `json:"target"`
	Current              float64 `json:"current"`
	ErrorBudgetRemaining float64 `json:"error_budget_remaining"`
	BurnRate             float64 `json:"burn_rate"`
}
