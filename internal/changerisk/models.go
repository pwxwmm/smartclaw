package changerisk

import (
	"time"
)

// ChangeType enumerates the kinds of changes.
type ChangeType string

const (
	ChangeDeployment ChangeType = "deployment"
	ChangeConfig     ChangeType = "config_change"
	ChangeScaling    ChangeType = "scaling"
	ChangeRollback   ChangeType = "rollback"
	ChangeHotfix     ChangeType = "hotfix"
	ChangeMigration  ChangeType = "migration"
)

// ChangeRequest describes a proposed change.
type ChangeRequest struct {
	ID          string            `json:"id"`
	Type        ChangeType        `json:"type"`
	Service     string            `json:"service"`
	Services    []string          `json:"services"`
	Description string            `json:"description"`
	Requester   string            `json:"requester"`
	Priority    string            `json:"priority"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	Labels      map[string]string `json:"labels"`
}

// RiskLevel represents the overall risk assessment.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// RiskFactor is an individual risk contributor.
type RiskFactor struct {
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	Weight      float64 `json:"weight"`
	Details     any     `json:"details"`
}

// RiskAssessment is the result of evaluating a ChangeRequest.
type RiskAssessment struct {
	RequestID       string       `json:"request_id"`
	OverallScore    float64      `json:"overall_score"`
	RiskLevel       RiskLevel    `json:"risk_level"`
	Factors         []RiskFactor `json:"factors"`
	Recommendations []string     `json:"recommendations"`
	BlastRadius     *BlastInfo   `json:"blast_radius,omitempty"`
	Approved        bool         `json:"approved"`
	AssessedAt      time.Time    `json:"assessed_at"`
}

// BlastInfo summarizes topology blast radius for the change.
type BlastInfo struct {
	DirectDependencies int      `json:"direct_dependencies"`
	TotalAffected      int      `json:"total_affected"`
	AffectedServices   []string `json:"affected_services"`
	CriticalPath       bool     `json:"critical_path"`
}

// ChangeRecord tracks a historical change for failure rate calculation.
type ChangeRecord struct {
	ID          string     `json:"id"`
	Type        ChangeType `json:"type"`
	Service     string     `json:"service"`
	Success     bool       `json:"success"`
	AssessedAt  time.Time  `json:"assessed_at"`
	ActualRisk  RiskLevel  `json:"actual_risk"`
	Outcome     string     `json:"outcome"`
	CompletedAt time.Time  `json:"completed_at"`
}

// IncidentInfo holds simplified incident data (avoids import cycle).
type IncidentInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Severity  string    `json:"severity"`
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

// SLOInfo holds simplified SLO status data (avoids import cycle).
type SLOInfo struct {
	Service              string  `json:"service"`
	SLOName              string  `json:"slo_name"`
	Target               float64 `json:"target"`
	Current              float64 `json:"current"`
	ErrorBudgetRemaining float64 `json:"error_budget_remaining"`
	BurnRate             float64 `json:"burn_rate"`
}

// TopologyProvider is the interface for querying service topology.
type TopologyProvider interface {
	GetNeighbors(serviceID string, depth int) (services []string, err error)
}

// IncidentProvider is the interface for querying incidents and SLO status.
type IncidentProvider interface {
	GetRecentIncidents(service string, since time.Time) ([]IncidentInfo, error)
	GetSLOStatus(service string) (*SLOInfo, error)
}

// RiskThresholds configures the risk level cutoffs.
type RiskThresholds struct {
	LowCutoff      float64
	MediumCutoff   float64
	HighCutoff     float64
	AutoApproveMax float64
}

// DefaultRiskThresholds returns the standard risk thresholds.
func DefaultRiskThresholds() RiskThresholds {
	return RiskThresholds{
		LowCutoff:      0.25,
		MediumCutoff:   0.50,
		HighCutoff:     0.75,
		AutoApproveMax: 0.30,
	}
}
