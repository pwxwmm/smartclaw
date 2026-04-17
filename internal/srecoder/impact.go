package srecoder

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/changerisk"
	"github.com/instructkr/smartclaw/internal/topology"
)

type RelatedAlert struct {
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Service  string `json:"service"`
	Count    int    `json:"count"`
	Status   string `json:"status"`
}

type RelatedRunbook struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Service     string `json:"service"`
	TriggerType string `json:"trigger_type"`
}

type ImpactAnalysis struct {
	ChangedFile      string           `json:"changed_file"`
	ChangeType       string           `json:"change_type"`
	AffectedServices []string         `json:"affected_services"`
	BlastRadius      []string         `json:"blast_radius"`
	RiskLevel        string           `json:"risk_level"`
	RiskScore        float64          `json:"risk_score"`
	RelatedAlerts    []RelatedAlert   `json:"related_alerts"`
	Runbooks         []RelatedRunbook `json:"runbooks"`
	Suggestions      []string         `json:"suggestions"`
}

var diffFileRegex = regexp.MustCompile(`^---\s+a/(.+)$`)

func (m *SRECodingMode) AnalyzeImpact(ctx context.Context, filePath string, changeType string) (*ImpactAnalysis, error) {
	analysis := &ImpactAnalysis{
		ChangedFile: filePath,
		ChangeType:  changeType,
	}

	serviceName := fileToService(filePath)
	if serviceName == "" {
		serviceName = filepath.Base(filepath.Dir(filePath))
	}

	m.analyzeTopologyImpact(analysis, serviceName)
	m.analyzeChangeRisk(analysis, serviceName, changeType)
	m.analyzeAlerts(analysis, serviceName)
	m.analyzeRunbooks(analysis, serviceName)
	m.generateSuggestions(analysis)

	return analysis, nil
}

func (m *SRECodingMode) AnalyzeDiff(ctx context.Context, diffContent string) ([]*ImpactAnalysis, error) {
	var results []*ImpactAnalysis

	lines := strings.Split(diffContent, "\n")
	seen := make(map[string]bool)

	for _, line := range lines {
		matches := diffFileRegex.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		filePath := matches[1]
		if seen[filePath] {
			continue
		}
		seen[filePath] = true

		changeType := detectChangeType(filePath, diffContent)
		analysis, err := m.AnalyzeImpact(ctx, filePath, changeType)
		if err != nil {
			continue
		}
		results = append(results, analysis)
	}

	return results, nil
}

func (m *SRECodingMode) analyzeTopologyImpact(analysis *ImpactAnalysis, serviceName string) {
	g := m.GetTopology()
	if g == nil {
		analysis.BlastRadius = []string{"topology unavailable"}
		analysis.AffectedServices = []string{serviceName}
		return
	}

	nodeID := findServiceNode(g, serviceName)
	if nodeID == "" {
		analysis.AffectedServices = []string{serviceName}
		analysis.BlastRadius = []string{serviceName + " (not found in topology)"}
		return
	}

	result := topology.BlastRadius(g, nodeID, 3)
	if result == nil {
		analysis.AffectedServices = []string{serviceName}
		return
	}

	analysis.AffectedServices = []string{serviceName}

	seen := map[string]bool{serviceName: true}
	for _, node := range result.Affected {
		name := node.Name
		if name == "" {
			name = node.ID
		}
		if !seen[name] {
			seen[name] = true
			analysis.AffectedServices = append(analysis.AffectedServices, name)
		}
	}

	blastSet := map[string]bool{}
	for _, svc := range analysis.AffectedServices {
		blastSet[svc] = true
	}
	for svc := range blastSet {
		analysis.BlastRadius = append(analysis.BlastRadius, svc)
	}
}

func (m *SRECodingMode) analyzeChangeRisk(analysis *ImpactAnalysis, serviceName string, changeType string) {
	checker := m.GetChangeRisk()
	if checker == nil {
		analysis.RiskLevel = "medium"
		analysis.RiskScore = 0.4
		return
	}

	req := changerisk.ChangeRequest{
		ID:          fmt.Sprintf("sre-code-%d", time.Now().UnixNano()),
		Type:        mapChangeType(changeType),
		Service:     serviceName,
		Services:    analysis.AffectedServices,
		Description: fmt.Sprintf("Code change to %s", analysis.ChangedFile),
	}

	assessment, err := checker.Assess(req)
	if err != nil {
		analysis.RiskLevel = "medium"
		analysis.RiskScore = 0.4
		return
	}

	analysis.RiskLevel = string(assessment.RiskLevel)
	analysis.RiskScore = assessment.OverallScore

	if assessment.BlastRadius != nil {
		for _, svc := range assessment.BlastRadius.AffectedServices {
			found := false
			for _, existing := range analysis.AffectedServices {
				if existing == svc {
					found = true
					break
				}
			}
			if !found {
				analysis.AffectedServices = append(analysis.AffectedServices, svc)
			}
		}
	}
}

func (m *SRECodingMode) analyzeAlerts(analysis *ImpactAnalysis, serviceName string) {
	ae := m.GetAlerts()
	if ae == nil {
		return
	}

	since := time.Now().Add(-1 * time.Hour)
	for _, svc := range analysis.AffectedServices {
		alerts := ae.Query(svc, "", since)
		for _, da := range alerts {
			if da.Status == "resolved" {
				continue
			}
			analysis.RelatedAlerts = append(analysis.RelatedAlerts, RelatedAlert{
				Name:     da.Name,
				Severity: da.Severity,
				Service:  da.Service,
				Count:    da.Count,
				Status:   da.Status,
			})
		}
	}
}

func (m *SRECodingMode) analyzeRunbooks(analysis *ImpactAnalysis, serviceName string) {
	re := m.GetRunbooks()
	if re == nil {
		return
	}

	for _, svc := range analysis.AffectedServices {
		runbooks, err := re.SuggestRemediation(svc, "")
		if err != nil {
			continue
		}
		for _, rb := range runbooks {
			analysis.Runbooks = append(analysis.Runbooks, RelatedRunbook{
				ID:          rb.ID,
				Name:        rb.Name,
				Description: rb.Description,
				Service:     rb.Service,
				TriggerType: rb.Trigger.Type,
			})
		}
	}
}

func (m *SRECodingMode) generateSuggestions(analysis *ImpactAnalysis) {
	var suggestions []string

	switch {
	case analysis.RiskScore >= 0.75:
		suggestions = append(suggestions, "CRITICAL RISK: Consider staging this change with a canary deployment")
		suggestions = append(suggestions, "Add circuit breakers to all downstream dependencies")
	case analysis.RiskScore >= 0.5:
		suggestions = append(suggestions, "HIGH RISK: Add rollback capability before deploying")
		suggestions = append(suggestions, "Consider adding retry logic with exponential backoff")
	case analysis.RiskScore >= 0.25:
		suggestions = append(suggestions, "MEDIUM RISK: Add health check endpoints for monitoring")
		suggestions = append(suggestions, "Consider adding Prometheus metrics for observability")
	default:
		suggestions = append(suggestions, "LOW RISK: Standard change process applies")
	}

	if len(analysis.RelatedAlerts) > 0 {
		criticalCount := 0
		for _, a := range analysis.RelatedAlerts {
			if a.Severity == "critical" || a.Severity == "high" {
				criticalCount++
			}
		}
		if criticalCount > 0 {
			suggestions = append(suggestions,
				fmt.Sprintf("WARNING: %d high/critical alerts active on affected services — delay change if possible", criticalCount))
		}
	}

	if len(analysis.BlastRadius) > 5 {
		suggestions = append(suggestions,
			fmt.Sprintf("Large blast radius (%d services) — consider feature flags for gradual rollout", len(analysis.BlastRadius)))
	}

	if len(analysis.Runbooks) > 0 {
		suggestions = append(suggestions,
			fmt.Sprintf("%d runbooks available for affected services — reference them in error handling", len(analysis.Runbooks)))
	}

	if analysis.ChangeType == "config_change" {
		suggestions = append(suggestions, "Config changes should include validation and safe defaults")
	}

	if analysis.ChangeType == "deployment" {
		suggestions = append(suggestions, "Add readiness probes and graceful shutdown for zero-downtime deployment")
	}

	analysis.Suggestions = suggestions
}

func fileToService(filePath string) string {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	for i, p := range parts {
		if p == "internal" && i+1 < len(parts) {
			return parts[i+1]
		}
		if p == "pkg" && i+1 < len(parts) {
			return parts[i+1]
		}
		if p == "cmd" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return filepath.Base(filepath.Dir(filePath))
}

func findServiceNode(g *topology.TopologyGraph, serviceName string) string {
	nodes := g.AllNodes()
	for _, n := range nodes {
		if n.Name == serviceName || n.ID == serviceName {
			return n.ID
		}
		if n.Labels != nil {
			if v, ok := n.Labels["app"]; ok && v == serviceName {
				return n.ID
			}
			if v, ok := n.Labels["service"]; ok && v == serviceName {
				return n.ID
			}
		}
		if strings.Contains(n.ID, serviceName) {
			return n.ID
		}
	}
	return ""
}

func detectChangeType(filePath string, diffContent string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".yaml", ".yml", ".json", ".toml", ".env":
		return "config_change"
	case ".sql":
		return "migration"
	}

	base := filepath.Base(filePath)
	if strings.Contains(base, "deploy") || strings.Contains(base, "Dockerfile") {
		return "deployment"
	}

	return "code_change"
}

func mapChangeType(changeType string) changerisk.ChangeType {
	switch changeType {
	case "deployment":
		return changerisk.ChangeDeployment
	case "config_change":
		return changerisk.ChangeConfig
	case "scaling":
		return changerisk.ChangeScaling
	case "rollback":
		return changerisk.ChangeRollback
	case "hotfix":
		return changerisk.ChangeHotfix
	case "migration":
		return changerisk.ChangeMigration
	default:
		return changerisk.ChangeDeployment
	}
}
