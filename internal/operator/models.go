package operator

import "time"

// AutonomyLevel defines how much autonomy the operator has to take actions.
type AutonomyLevel string

const (
	AutonomyObserve AutonomyLevel = "observe" // watch only, no actions
	AutonomySuggest AutonomyLevel = "suggest" // suggest actions, require approval
	AutonomyAuto    AutonomyLevel = "auto"    // auto-execute pre-approved actions
	AutonomyFull    AutonomyLevel = "full"    // auto-execute all actions including speculative
)

// CheckType defines the type of health check to perform.
type CheckType string

const (
	CheckHTTP     CheckType = "http"     // HTTP health check
	CheckTCP      CheckType = "tcp"      // TCP connectivity check
	CheckSLO      CheckType = "slo"      // SLO burn rate check
	CheckAlert    CheckType = "alert"    // alert threshold check
	CheckCustom   CheckType = "custom"   // custom command/script
	CheckTopology CheckType = "topology" // topology health check
)

// CheckStatus defines the result status of a health check.
type CheckStatus string

const (
	CheckPass    CheckStatus = "pass"
	CheckWarn    CheckStatus = "warn"
	CheckFail    CheckStatus = "fail"
	CheckError   CheckStatus = "error"
	CheckUnknown CheckStatus = "unknown"
)

// OperatorConfig holds the configuration for an operator instance.
type OperatorConfig struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Enabled          bool              `json:"enabled"`
	HealthChecks     []HealthCheckDef  `json:"health_checks"`
	EscalationPolicy EscalationPolicy  `json:"escalation_policy"`
	Schedule         string            `json:"schedule"`         // cron expression for main check cycle
	MaxAutoActions   int               `json:"max_auto_actions"` // per cycle, default 3
	AutonomyLevel    AutonomyLevel     `json:"autonomy_level"`
	NotifyChannels   []string          `json:"notify_channels"` // where to send alerts
	Labels           map[string]string `json:"labels"`
}

// HealthCheckDef defines a single health check within an operator config.
type HealthCheckDef struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Type      CheckType     `json:"type"`
	Target    string        `json:"target"`   // service name, URL, or query
	Schedule  string        `json:"schedule"` // cron expression (overrides main schedule)
	Timeout   time.Duration `json:"timeout"`
	Threshold float64       `json:"threshold"` // failure threshold (0.0-1.0)
	Severity  string        `json:"severity"`  // if check fails, what severity alert
}

// HealthCheckResult holds the result of executing a health check.
type HealthCheckResult struct {
	CheckID   string        `json:"check_id"`
	CheckName string        `json:"check_name"`
	Status    CheckStatus   `json:"status"`
	Value     float64       `json:"value"` // measured value
	Threshold float64       `json:"threshold"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// EscalationPolicy defines the escalation levels for an operator.
type EscalationPolicy struct {
	Levels []EscalationLevel `json:"levels"`
}

// EscalationLevel defines a single escalation level.
type EscalationLevel struct {
	Level    int           `json:"level"`     // 1, 2, 3...
	Trigger  string        `json:"trigger"`   // condition: "check_fail", "slo_burn>3", "alert_count>5"
	Actions  []string      `json:"actions"`   // what to do: "notify", "create_incident", "auto_remediate", "warroom"
	WaitTime time.Duration `json:"wait_time"` // wait before escalating to next level
	Notify   []string      `json:"notify"`    // who to notify at this level
}

// OperatorStatus holds the current status of an operator.
type OperatorStatus struct {
	ConfigID          string              `json:"config_id"`
	Enabled           bool                `json:"enabled"`
	AutonomyLevel     AutonomyLevel       `json:"autonomy_level"`
	LastCheckCycle    *time.Time          `json:"last_check_cycle,omitempty"`
	NextCheckCycle    *time.Time          `json:"next_check_cycle,omitempty"`
	TotalChecks       int                 `json:"total_checks"`
	PassingChecks     int                 `json:"passing_checks"`
	FailingChecks     int                 `json:"failing_checks"`
	ActiveEscalations int                 `json:"active_escalations"`
	AutoActionsToday  int                 `json:"auto_actions_today"`
	RecentResults     []HealthCheckResult `json:"recent_results"`
	Uptime            float64             `json:"uptime"` // % of checks passing over last 24h
}

// OperatorEvent records an event in the operator's lifecycle.
type OperatorEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"` // check_cycle, check_result, escalation, action, alert
	Details   string    `json:"details"`
	Severity  string    `json:"severity"`
}
