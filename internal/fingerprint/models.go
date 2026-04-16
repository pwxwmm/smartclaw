package fingerprint

import "time"

// VectorSize is the dimensionality of the feature vector.
const VectorSize = 64

// IncidentFingerprint represents the fingerprint of an incident as a 64-dim feature vector.
type IncidentFingerprint struct {
	IncidentID  string              `json:"incident_id"`
	Vector      [VectorSize]float64 `json:"vector"`
	Features    FeatureMap          `json:"features"`
	GeneratedAt time.Time           `json:"generated_at"`
	Version     int                 `json:"version"`
}

// FeatureMap holds the human-readable breakdown of all feature categories.
type FeatureMap struct {
	Temporal TemporalFeatures `json:"temporal"`
	Severity SeverityFeatures `json:"severity"`
	Topology TopologyFeatures `json:"topology"`
	Service  ServiceFeatures  `json:"service"`
	Impact   ImpactFeatures   `json:"impact"`
	Response ResponseFeatures `json:"response"`
	Category CategoryFeatures `json:"category"`
	Label    LabelFeatures    `json:"label"`
}

// TemporalFeatures captures time-based characteristics of an incident.
// Vector layout: [0-6]
type TemporalFeatures struct {
	HourOfDay       float64 `json:"hour_of_day"`       // 0.0-23.0 normalized to 0-1
	DayOfWeek       float64 `json:"day_of_week"`       // 0-6 normalized to 0-1
	IsBusinessHours float64 `json:"is_business_hours"` // 1.0 if 9-17 weekday, 0.0 otherwise
	IsWeekend       float64 `json:"is_weekend"`        // 1.0 if Sat/Sun, 0.0
	Duration        float64 `json:"duration"`          // hours, capped at 72, normalized 0-1
	TimeToTriage    float64 `json:"time_to_triage"`    // minutes to first response, normalized 0-1 (60min = 1.0)
	TimeToMitigate  float64 `json:"time_to_mitigate"`  // minutes to mitigation, normalized 0-1 (240min = 1.0)
}

// SeverityFeatures captures severity and escalation characteristics.
// Vector layout: [7-9]
type SeverityFeatures struct {
	SeverityLevel   float64 `json:"severity_level"`   // info=0.0, low=0.25, medium=0.5, high=0.75, critical=1.0
	Escalated       float64 `json:"escalated"`        // 1.0 if escalated, 0.0
	EscalationCount float64 `json:"escalation_count"` // normalized 0-1 (5+ = 1.0)
}

// TopologyFeatures captures infrastructure topology characteristics.
// Vector layout: [10-13]
type TopologyFeatures struct {
	AffectedServiceCount float64 `json:"affected_service_count"` // normalized 0-1 (20+ = 1.0)
	BlastRadiusScore     float64 `json:"blast_radius_score"`     // 0-1
	IsCriticalPath       float64 `json:"is_critical_path"`       // 1.0 if on critical path
	DependencyDepth      float64 `json:"dependency_depth"`       // max depth, normalized 0-1 (5+ = 1.0)
}

// ServiceFeatures captures service-related characteristics.
// Vector layout: [14-16]
type ServiceFeatures struct {
	ServiceType        float64 `json:"service_type"`         // 0=frontend, 0.25=api, 0.5=worker, 0.75=datastore, 1.0=infra
	ServiceCount       float64 `json:"service_count"`        // normalized 0-1 (10+ = 1.0)
	PrimaryServiceHash float64 `json:"primary_service_hash"` // FNV hash of service name, normalized 0-1
}

// ImpactFeatures captures the impact and SLO characteristics.
// Vector layout: [17-20]
type ImpactFeatures struct {
	UserImpactScore   float64 `json:"user_impact_score"`   // 0-1
	SLOViolationCount float64 `json:"slo_violation_count"` // normalized 0-1 (5+ = 1.0)
	ErrorBudgetBurned float64 `json:"error_budget_burned"` // 0-1
	DataLoss          float64 `json:"data_loss"`           // 1.0 if data loss, 0.0
}

// ResponseFeatures captures the response and remediation characteristics.
// Vector layout: [21-26]
type ResponseFeatures struct {
	ToolCallCount       float64 `json:"tool_call_count"`      // normalized 0-1 (50+ = 1.0)
	InvestigationSteps  float64 `json:"investigation_steps"`  // normalized 0-1 (20+ = 1.0)
	RemediationAttempts float64 `json:"remediation_attempts"` // normalized 0-1 (5+ = 1.0)
	AutoRemediated      float64 `json:"auto_remediated"`      // 1.0 if auto-remediated
	HumanIntervention   float64 `json:"human_intervention"`   // 1.0 if human intervened
}

// CategoryFeatures captures the incident category classification.
// Vector layout: [27-34]
type CategoryFeatures struct {
	IsNetworkIssue    float64 `json:"is_network_issue"`
	IsDatabaseIssue   float64 `json:"is_database_issue"`
	IsInfraIssue      float64 `json:"is_infra_issue"`
	IsAppIssue        float64 `json:"is_app_issue"`
	IsSecurityIssue   float64 `json:"is_security_issue"`
	IsConfigIssue     float64 `json:"is_config_issue"`
	IsDeploymentIssue float64 `json:"is_deployment_issue"`
	IsCapacityIssue   float64 `json:"is_capacity_issue"`
}

// LabelFeatures captures label-based metadata characteristics.
// Vector layout: [35-38]
type LabelFeatures struct {
	HasRunbook       float64 `json:"has_runbook"`
	HasPostmortem    float64 `json:"has_postmortem"`
	IsRecurring      float64 `json:"is_recurring"`
	SimilarPastCount float64 `json:"similar_past_count"` // normalized 0-1 (10+ = 1.0)
}

// SimilarityResult represents a single similar incident found by the search.
type SimilarityResult struct {
	IncidentID       string  `json:"incident_id"`
	Similarity       float64 `json:"similarity"`
	IncidentTitle    string  `json:"incident_title,omitempty"`
	IncidentSeverity string  `json:"incident_severity,omitempty"`
	IncidentService  string  `json:"incident_service,omitempty"`
	FeatureMatch     string  `json:"feature_match"`
}

// IncidentBrief is a minimal incident summary used for enriching search results.
type IncidentBrief struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Severity string `json:"severity"`
	Service  string `json:"service,omitempty"`
}

// IncidentStore is the interface for fetching incident metadata.
// This is defined in this package to avoid importing internal/memory.
type IncidentStore interface {
	GetIncident(id string) (*IncidentBrief, error)
	ListIncidents(limit int) ([]IncidentBrief, error)
}

// Feature category ranges within the 64-dim vector.
var (
	// CategoryRanges maps category names to their [start, end) indices in the vector.
	CategoryRanges = map[string][2]int{
		"temporal": {0, 7},
		"severity": {7, 10},
		"topology": {10, 14},
		"service":  {14, 17},
		"impact":   {17, 21},
		"response": {21, 27},
		"category": {27, 35},
		"label":    {35, 39},
	}
)
