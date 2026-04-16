package alertengine

import "time"

// Alert is a raw alert from any monitoring system.
type Alert struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Name        string            `json:"name"`
	Severity    string            `json:"severity"`
	Status      string            `json:"status"`
	Service     string            `json:"service"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	FiredAt     time.Time         `json:"fired_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
	Fingerprint string            `json:"fingerprint"`
}

// DedupedAlert represents a deduplicated alert (multiple raw alerts folded into one).
type DedupedAlert struct {
	Fingerprint  string            `json:"fingerprint"`
	Name         string            `json:"name"`
	Severity     string            `json:"severity"`
	Service      string            `json:"service"`
	Source       string            `json:"source"`
	Count        int               `json:"count"`
	FirstFiredAt time.Time         `json:"first_fired_at"`
	LastFiredAt  time.Time         `json:"last_fired_at"`
	Labels       map[string]string `json:"labels"`
	Status       string            `json:"status"`
}

// AlertGroup is a group of correlated deduped alerts.
type AlertGroup struct {
	ID          string         `json:"id"`
	Alerts      []DedupedAlert `json:"alerts"`
	RootAlert   *DedupedAlert  `json:"root_alert,omitempty"`
	Correlation string         `json:"correlation"`
	Score       float64        `json:"score"`
	CreatedAt   time.Time      `json:"created_at"`
	Services    []string       `json:"services"`
}

// CorrelationResult is the output of the full correlation pipeline.
type CorrelationResult struct {
	Groups    []AlertGroup     `json:"groups"`
	Unmatched []DedupedAlert   `json:"unmatched"`
	Stats     CorrelationStats `json:"stats"`
}

// CorrelationStats holds statistics about the correlation pipeline run.
type CorrelationStats struct {
	TotalRaw       int `json:"total_raw"`
	TotalDeduped   int `json:"total_deduped"`
	TotalGroups    int `json:"total_groups"`
	TotalUnmatched int `json:"total_unmatched"`
}

// TopologyProvider is an interface for querying the topology graph.
// The actual topology package will implement this.
type TopologyProvider interface {
	GetNeighbors(serviceID string, depth int) (services []string, err error)
}

// Severity levels ordered from most to least severe.
var severityOrder = map[string]int{
	"critical": 5,
	"high":     4,
	"medium":   3,
	"low":      2,
	"info":     1,
}

// SeverityLevel returns the numeric level for a severity string.
// Returns 0 for unknown severities.
func SeverityLevel(sev string) int {
	if lvl, ok := severityOrder[sev]; ok {
		return lvl
	}
	return 0
}

// CompareSeverity returns true if sev1 is more severe than sev2.
func CompareSeverity(sev1, sev2 string) bool {
	return SeverityLevel(sev1) > SeverityLevel(sev2)
}

// EscalateSeverity bumps the severity one level up.
// If already at critical, returns critical.
func EscalateSeverity(sev string) string {
	lvl := SeverityLevel(sev)
	if lvl >= 5 {
		return "critical"
	}
	for name, level := range severityOrder {
		if level == lvl+1 {
			return name
		}
	}
	return sev
}
