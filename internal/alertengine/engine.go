package alertengine

import (
	"sync"
	"time"
)

// AlertEngine is the core engine for alert deduplication and correlation.
type AlertEngine struct {
	mu            sync.RWMutex
	rawAlerts     []Alert
	head          int
	rawCount      int
	totalIngested int
	deduped       map[string]*DedupedAlert
	groups        []AlertGroup
	topology      TopologyProvider

	dedupWindow           time.Duration
	corrWindow            time.Duration
	maxRawAlerts          int
	autoEscalateThreshold int
}

// NewAlertEngine creates a new AlertEngine with default configuration.
func NewAlertEngine() *AlertEngine {
	return &AlertEngine{
		rawAlerts:             make([]Alert, config.MaxRawAlerts),
		head:                  0,
		rawCount:              0,
		totalIngested:         0,
		deduped:               make(map[string]*DedupedAlert),
		groups:                nil,
		topology:              nil,
		dedupWindow:           config.DedupWindow,
		corrWindow:            config.CorrelationWindow,
		maxRawAlerts:          config.MaxRawAlerts,
		autoEscalateThreshold: config.AutoEscalateThreshold,
	}
}

func Shutdown() {
	defaultEngineMu.RLock()
	e := defaultEngine
	defaultEngineMu.RUnlock()
	if e != nil {
		e.Clear()
	}
}

// SetTopologyProvider sets the topology provider for topology-aware correlation.
func (e *AlertEngine) SetTopologyProvider(tp TopologyProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.topology = tp
}

// Ingest processes a single alert, computes its fingerprint, and merges
// it into the deduped index. Returns the resulting DedupedAlert.
func (e *AlertEngine) Ingest(alert Alert) *DedupedAlert {
	alert.Fingerprint = FingerprintAlert(&alert)

	e.mu.Lock()
	defer e.mu.Unlock()

	e.pushRaw(alert)
	metricAlertsIngested.Inc()

	existing, ok := e.deduped[alert.Fingerprint]
	if !ok {
		da := &DedupedAlert{
			Fingerprint:  alert.Fingerprint,
			Name:         alert.Name,
			Severity:     alert.Severity,
			Service:      alert.Service,
			Source:       alert.Source,
			Count:        1,
			FirstFiredAt: alert.FiredAt,
			LastFiredAt:  alert.FiredAt,
			Labels:       NormalizeLabels(alert.Labels),
			Status:       alert.Status,
		}
		e.deduped[alert.Fingerprint] = da
		return &DedupedAlert{
			Fingerprint:  da.Fingerprint,
			Name:         da.Name,
			Severity:     da.Severity,
			Service:      da.Service,
			Source:       da.Source,
			Count:        da.Count,
			FirstFiredAt: da.FirstFiredAt,
			LastFiredAt:  da.LastFiredAt,
			Labels:       da.Labels,
			Status:       da.Status,
		}
	}

	metricAlertsDeduped.Inc()

	existing.Count++
	if alert.FiredAt.Before(existing.FirstFiredAt) {
		existing.FirstFiredAt = alert.FiredAt
	}
	if alert.FiredAt.After(existing.LastFiredAt) {
		existing.LastFiredAt = alert.FiredAt
	}
	if CompareSeverity(alert.Severity, existing.Severity) {
		existing.Severity = alert.Severity
	}
	if alert.Status == "firing" && existing.Status != "firing" {
		existing.Status = "firing"
	}

	if len(e.deduped) > e.maxRawAlerts {
		cutoff := time.Now().Add(-e.dedupWindow)
		for fp, da := range e.deduped {
			if da.LastFiredAt.Before(cutoff) {
				delete(e.deduped, fp)
			}
		}
	}

	return &DedupedAlert{
		Fingerprint:  existing.Fingerprint,
		Name:         existing.Name,
		Severity:     existing.Severity,
		Service:      existing.Service,
		Source:       existing.Source,
		Count:        existing.Count,
		FirstFiredAt: existing.FirstFiredAt,
		LastFiredAt:  existing.LastFiredAt,
		Labels:       existing.Labels,
		Status:       existing.Status,
	}
}

// IngestBatch processes multiple alerts and returns the deduped results.
func (e *AlertEngine) IngestBatch(alerts []Alert) []DedupedAlert {
	results := make([]DedupedAlert, 0, len(alerts))
	for i := range alerts {
		da := e.Ingest(alerts[i])
		results = append(results, *da)
	}
	return results
}

// Query returns deduped alerts matching the given filters.
// Empty string filters are ignored.
func (e *AlertEngine) Query(service string, severity string, since time.Time) []DedupedAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []DedupedAlert
	for _, da := range e.deduped {
		if service != "" && da.Service != service {
			continue
		}
		if severity != "" && da.Severity != severity {
			continue
		}
		if !since.IsZero() && da.LastFiredAt.Before(since) {
			continue
		}
		results = append(results, *da)
	}

	sortDedupedBySeverityAndTime(results)
	return results
}

// GetGroup returns an AlertGroup by its ID.
func (e *AlertEngine) GetGroup(groupID string) *AlertGroup {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for i := range e.groups {
		if e.groups[i].ID == groupID {
			return &e.groups[i]
		}
	}
	return nil
}

// Stats returns current correlation statistics.
func (e *AlertEngine) Stats() CorrelationStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	unmatched := 0
	for _, g := range e.groups {
		if g.Correlation == "unmatched" {
			unmatched++
		}
	}

	return CorrelationStats{
		TotalRaw:       e.totalIngested,
		TotalDeduped:   len(e.deduped),
		TotalGroups:    len(e.groups),
		TotalUnmatched: unmatched,
	}
}

// Clear removes all alerts and groups from the engine.
func (e *AlertEngine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.rawAlerts = make([]Alert, e.maxRawAlerts)
	e.head = 0
	e.rawCount = 0
	e.totalIngested = 0
	e.deduped = make(map[string]*DedupedAlert)
	e.groups = nil
}

// Prune removes deduped alerts older than maxAge.
func (e *AlertEngine) Prune(maxAge time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for fp, da := range e.deduped {
		if da.LastFiredAt.Before(cutoff) {
			delete(e.deduped, fp)
		}
	}
}

// pushRaw adds a raw alert to the ring buffer.
func (e *AlertEngine) pushRaw(alert Alert) {
	e.totalIngested++
	if e.rawCount < e.maxRawAlerts {
		e.rawAlerts[e.rawCount] = alert
		e.rawCount++
	} else {
		e.rawAlerts[e.head] = alert
		e.head = (e.head + 1) % e.maxRawAlerts
	}
}

func sortDedupedBySeverityAndTime(alerts []DedupedAlert) {
	sortSlice(alerts, func(a, b DedupedAlert) bool {
		if SeverityLevel(a.Severity) != SeverityLevel(b.Severity) {
			return SeverityLevel(a.Severity) > SeverityLevel(b.Severity)
		}
		return a.FirstFiredAt.Before(b.FirstFiredAt)
	})
}

func sortSlice[T any](s []T, less func(a, b T) bool) {
	n := len(s)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if less(s[j], s[i]) {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
