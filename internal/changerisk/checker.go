package changerisk

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

type ChangeRiskChecker struct {
	mu             sync.RWMutex
	history        []ChangeRecord
	topology       TopologyProvider
	incidents      IncidentProvider
	riskThresholds RiskThresholds
}

func NewChangeRiskChecker() *ChangeRiskChecker {
	return &ChangeRiskChecker{
		history:        make([]ChangeRecord, 0),
		riskThresholds: DefaultRiskThresholds(),
	}
}

func Shutdown() {
}

func (c *ChangeRiskChecker) SetTopologyProvider(tp TopologyProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.topology = tp
}

func (c *ChangeRiskChecker) SetIncidentProvider(ip IncidentProvider) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.incidents = ip
}

func (c *ChangeRiskChecker) SetRiskThresholds(thresholds RiskThresholds) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.riskThresholds = thresholds
}

func (c *ChangeRiskChecker) Assess(req ChangeRequest) (*RiskAssessment, error) {
	if req.Service == "" {
		return nil, fmt.Errorf("change request must specify a service")
	}

	c.mu.RLock()
	topo := c.topology
	inc := c.incidents
	thresholds := c.riskThresholds
	hist := c.history
	c.mu.RUnlock()

	allServices := c.allServices(req)

	var factors []RiskFactor

	blastFactor, blastInfo := c.assessBlastRadius(req, topo, allServices)
	factors = append(factors, blastFactor)

	incidentFactor := c.assessRecentIncidents(req, inc, allServices)
	factors = append(factors, incidentFactor)

	sloFactor := c.assessSLOBurn(req, inc, allServices)
	factors = append(factors, sloFactor)

	failureFactor := c.assessChangeFailure(req, hist, allServices)
	factors = append(factors, failureFactor)

	timeFactor := c.assessTimeRisk(req)
	factors = append(factors, timeFactor)

	overallScore := c.computeOverallScore(factors)
	riskLevel := c.scoreToLevel(overallScore, thresholds)
	recommendations := c.generateRecommendations(factors, riskLevel)
	approved := overallScore < thresholds.AutoApproveMax

	assessment := &RiskAssessment{
		RequestID:       req.ID,
		OverallScore:    overallScore,
		RiskLevel:       riskLevel,
		Factors:         factors,
		Recommendations: recommendations,
		BlastRadius:     blastInfo,
		Approved:        approved,
		AssessedAt:      time.Now(),
	}

	metricRiskAssessments.WithLabelValues(string(riskLevel)).Inc()

	return assessment, nil
}

func (c *ChangeRiskChecker) RecordChange(rec ChangeRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history, rec)
	if len(c.history) > config.MaxHistorySize {
		c.history = c.history[len(c.history)-config.MaxHistorySize:]
	}
	metricRiskHistorySize.Set(float64(len(c.history)))
}

func (c *ChangeRiskChecker) GetHistory(service string, limit int) []ChangeRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	var filtered []ChangeRecord
	for i := len(c.history) - 1; i >= 0 && len(filtered) < limit; i-- {
		if service == "" || c.history[i].Service == service {
			filtered = append(filtered, c.history[i])
		}
	}

	return filtered
}

func (c *ChangeRiskChecker) FailureRate(service string, changeType ChangeType) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.failureRateLocked(service, changeType)
}

func (c *ChangeRiskChecker) failureRateLocked(service string, changeType ChangeType) float64 {
	var total, failures int
	for _, rec := range c.history {
		if rec.Service != service {
			continue
		}
		if changeType != "" && rec.Type != changeType {
			continue
		}
		total++
		if !rec.Success {
			failures++
		}
	}
	if total == 0 {
		return 0
	}
	return float64(failures) / float64(total)
}

func (c *ChangeRiskChecker) allServices(req ChangeRequest) []string {
	seen := map[string]bool{req.Service: true}
	services := []string{req.Service}
	for _, s := range req.Services {
		if !seen[s] && s != "" {
			seen[s] = true
			services = append(services, s)
		}
	}
	return services
}

func (c *ChangeRiskChecker) assessBlastRadius(req ChangeRequest, topo TopologyProvider, allServices []string) (RiskFactor, *BlastInfo) {
	if topo == nil {
		return RiskFactor{
			Category:    "blast_radius",
			Description: "Topology provider unavailable — assuming moderate blast radius",
			Score:       0.5,
			Weight:      0.30,
			Details:     map[string]string{"reason": "no_topology_provider"},
		}, nil
	}

	affectedSet := make(map[string]bool)
	for _, s := range allServices {
		affectedSet[s] = true
	}

	directDeps := 0
	for _, svc := range allServices {
		neighbors, err := topo.GetNeighbors(svc, 2)
		if err != nil {
			continue
		}
		for _, n := range neighbors {
			if !affectedSet[n] {
				affectedSet[n] = true
				directDeps++
			}
		}
	}

	totalAffected := len(affectedSet)
	affectedList := make([]string, 0, totalAffected)
	for s := range affectedSet {
		affectedList = append(affectedList, s)
	}
	sort.Strings(affectedList)

	score := math.Min(1.0, float64(totalAffected)/20.0)

	blastInfo := &BlastInfo{
		DirectDependencies: directDeps,
		TotalAffected:      totalAffected,
		AffectedServices:   affectedList,
		CriticalPath:       score > 0.5,
	}

	return RiskFactor{
		Category:    "blast_radius",
		Description: fmt.Sprintf("Change affects %d services (%d direct dependencies)", totalAffected, directDeps),
		Score:       score,
		Weight:      0.30,
		Details:     blastInfo,
	}, blastInfo
}

func (c *ChangeRiskChecker) assessRecentIncidents(req ChangeRequest, inc IncidentProvider, allServices []string) RiskFactor {
	if inc == nil {
		return RiskFactor{
			Category:    "recent_incidents",
			Description: "Incident provider unavailable — assuming low incident risk",
			Score:       0.3,
			Weight:      0.25,
			Details:     map[string]string{"reason": "no_incident_provider"},
		}
	}

	since := time.Now().AddDate(0, 0, -7)
	severityWeight := map[string]float64{
		"critical": 3.0,
		"high":     2.0,
		"medium":   1.0,
		"low":      0.5,
	}

	var totalWeighted float64
	var totalRaw int
	allIncidents := make([]IncidentInfo, 0)

	for _, svc := range allServices {
		incidents, err := inc.GetRecentIncidents(svc, since)
		if err != nil {
			continue
		}
		for _, incident := range incidents {
			totalRaw++
			w, ok := severityWeight[incident.Severity]
			if !ok {
				w = 1.0
			}
			totalWeighted += w
			allIncidents = append(allIncidents, incident)
		}
	}

	score := math.Min(1.0, float64(totalRaw)/5.0)

	if totalWeighted > 0 {
		score = math.Min(1.0, totalWeighted/10.0)
	}

	return RiskFactor{
		Category:    "recent_incidents",
		Description: fmt.Sprintf("%d recent incidents in affected services (weighted score: %.1f)", totalRaw, totalWeighted),
		Score:       score,
		Weight:      0.25,
		Details:     map[string]any{"count": totalRaw, "weighted_score": totalWeighted, "incidents": allIncidents},
	}
}

func (c *ChangeRiskChecker) assessSLOBurn(req ChangeRequest, inc IncidentProvider, allServices []string) RiskFactor {
	if inc == nil {
		return RiskFactor{
			Category:    "slo_burn",
			Description: "Incident provider unavailable — assuming low SLO burn risk",
			Score:       0.2,
			Weight:      0.25,
			Details:     map[string]string{"reason": "no_incident_provider"},
		}
	}

	var maxScore float64
	sloDetails := make([]map[string]any, 0)

	for _, svc := range allServices {
		slo, err := inc.GetSLOStatus(svc)
		if err != nil || slo == nil {
			continue
		}

		var score float64
		switch {
		case slo.BurnRate > 10:
			score = 1.0
		case slo.BurnRate > 3:
			score = 0.7
		case slo.BurnRate > 1:
			score = 0.4
		default:
			score = 0.1
		}

		if slo.ErrorBudgetRemaining < 0.10 {
			score += 0.3
		}

		if score > 1.0 {
			score = 1.0
		}

		sloDetails = append(sloDetails, map[string]any{
			"service":   svc,
			"burn_rate": slo.BurnRate,
			"score":     score,
			"budget":    slo.ErrorBudgetRemaining,
		})

		if score > maxScore {
			maxScore = score
		}
	}

	if len(sloDetails) == 0 {
		return RiskFactor{
			Category:    "slo_burn",
			Description: "No SLO data available for affected services",
			Score:       0.2,
			Weight:      0.25,
			Details:     map[string]string{"reason": "no_slo_data"},
		}
	}

	return RiskFactor{
		Category:    "slo_burn",
		Description: fmt.Sprintf("Max SLO burn risk %.2f across %d services", maxScore, len(sloDetails)),
		Score:       maxScore,
		Weight:      0.25,
		Details:     sloDetails,
	}
}

func (c *ChangeRiskChecker) assessChangeFailure(req ChangeRequest, hist []ChangeRecord, allServices []string) RiskFactor {
	var totalDataPoints int
	var maxRate float64

	for _, svc := range allServices {
		var svcTotal, svcFailures int
		for _, rec := range hist {
			if rec.Service == svc {
				svcTotal++
				if !rec.Success {
					svcFailures++
				}
			}
		}
		totalDataPoints += svcTotal

		if svcTotal >= 3 {
			rate := float64(svcFailures) / float64(svcTotal)
			if rate > maxRate {
				maxRate = rate
			}
		}
	}

	if totalDataPoints < 3 {
		return RiskFactor{
			Category:    "change_failure",
			Description: fmt.Sprintf("Insufficient change history (%d data points) for reliable failure rate", totalDataPoints),
			Score:       0.3,
			Weight:      0.15,
			Details:     map[string]any{"data_points": totalDataPoints, "min_required": 3},
		}
	}

	return RiskFactor{
		Category:    "change_failure",
		Description: fmt.Sprintf("Historical change failure rate: %.0f%%", maxRate*100),
		Score:       maxRate,
		Weight:      0.15,
		Details:     map[string]any{"failure_rate": maxRate, "data_points": totalDataPoints},
	}
}

func (c *ChangeRiskChecker) assessTimeRisk(req ChangeRequest) RiskFactor {
	t := time.Now()
	if req.ScheduledAt != nil {
		t = *req.ScheduledAt
	}

	var hourScore float64
	hour := t.Hour()
	if hour >= 9 && hour < 17 {
		hourScore = 0.2
	} else {
		hourScore = 0.6
	}

	var dayScore float64
	switch t.Weekday() {
	case time.Friday:
		dayScore = 0.5
	case time.Saturday, time.Sunday:
		dayScore = 0.7
	default:
		dayScore = 0.2
	}

	var freezeScore float64
	if req.Labels != nil {
		if v, ok := req.Labels["freeze"]; ok && v == "true" {
			freezeScore = 0.9
		}
	}

	score := math.Max(math.Max(hourScore, dayScore), freezeScore)

	desc := "Business hours"
	if hourScore > 0.5 {
		desc = "Outside business hours"
	}
	if dayScore > 0.4 {
		desc = fmt.Sprintf("%s (%s)", desc, t.Weekday().String())
	}
	if freezeScore > 0 {
		desc = "Change freeze active"
	}

	return RiskFactor{
		Category:    "time_risk",
		Description: desc,
		Score:       score,
		Weight:      0.05,
		Details: map[string]any{
			"hour_score":   hourScore,
			"day_score":    dayScore,
			"freeze_score": freezeScore,
			"scheduled_at": t,
			"weekday":      t.Weekday().String(),
			"hour":         hour,
		},
	}
}

func (c *ChangeRiskChecker) computeOverallScore(factors []RiskFactor) float64 {
	var weightedSum, totalWeight float64
	for _, f := range factors {
		weightedSum += f.Score * f.Weight
		totalWeight += f.Weight
	}
	if totalWeight == 0 {
		return 0
	}
	return weightedSum / totalWeight
}

func (c *ChangeRiskChecker) scoreToLevel(score float64, thresholds RiskThresholds) RiskLevel {
	switch {
	case score >= thresholds.HighCutoff:
		return RiskCritical
	case score >= thresholds.MediumCutoff:
		return RiskHigh
	case score >= thresholds.LowCutoff:
		return RiskMedium
	default:
		return RiskLow
	}
}

func (c *ChangeRiskChecker) generateRecommendations(factors []RiskFactor, level RiskLevel) []string {
	var recs []string

	for _, f := range factors {
		switch f.Category {
		case "blast_radius":
			if f.Score > 0.7 {
				recs = append(recs, "Consider staged rollout instead of full deployment")
			}
		case "recent_incidents":
			if f.Score > 0.7 {
				recs = append(recs, "Active incidents detected — delay change until resolved")
			}
		case "slo_burn":
			if f.Score > 0.7 {
				recs = append(recs, "SLO burn rate elevated — change may push error budget over")
			}
		case "change_failure":
			if f.Score > 0.5 {
				recs = append(recs, "This service has high change failure rate — add extra validation")
			}
		case "time_risk":
			if f.Score > 0.5 {
				recs = append(recs, "Consider scheduling during business hours")
			}
		}
	}

	if level == RiskCritical {
		recs = append(recs, "⚠️ Critical risk — require explicit approval from SRE lead")
	}

	if len(recs) == 0 {
		recs = append(recs, "Risk is within acceptable bounds — proceed with standard change process")
	}

	return recs
}

func InitChangeRiskChecker(topology TopologyProvider, incidents IncidentProvider) *ChangeRiskChecker {
	c := NewChangeRiskChecker()
	if topology != nil {
		c.SetTopologyProvider(topology)
	}
	if incidents != nil {
		c.SetIncidentProvider(incidents)
	}
	SetChangeRiskChecker(c)
	return c
}
