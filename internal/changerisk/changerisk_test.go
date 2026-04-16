package changerisk

import (
	"context"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/tools"
)

type mockTopologyProvider struct {
	neighbors map[string][]string
	err       error
}

func (m *mockTopologyProvider) GetNeighbors(serviceID string, depth int) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.neighbors[serviceID], nil
}

type mockIncidentProvider struct {
	incidents map[string][]IncidentInfo
	sloStatus map[string]*SLOInfo
	incErr    error
	sloErr    error
}

func (m *mockIncidentProvider) GetRecentIncidents(service string, since time.Time) ([]IncidentInfo, error) {
	if m.incErr != nil {
		return nil, m.incErr
	}
	return m.incidents[service], nil
}

func (m *mockIncidentProvider) GetSLOStatus(service string) (*SLOInfo, error) {
	if m.sloErr != nil {
		return nil, m.sloErr
	}
	return m.sloStatus[service], nil
}

func sampleRequest() ChangeRequest {
	return ChangeRequest{
		ID:       "cr-001",
		Type:     ChangeDeployment,
		Service:  "api-gateway",
		Services: []string{"api-gateway", "auth-service"},
		Labels:   map[string]string{},
	}
}

func TestAssessNoProviders(t *testing.T) {
	c := NewChangeRiskChecker()
	req := sampleRequest()

	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if assessment.RequestID != "cr-001" {
		t.Errorf("expected request_id cr-001, got %s", assessment.RequestID)
	}

	if assessment.OverallScore <= 0 {
		t.Error("expected positive overall score")
	}

	if assessment.RiskLevel == "" {
		t.Error("expected non-empty risk level")
	}

	foundCategories := map[string]bool{}
	for _, f := range assessment.Factors {
		foundCategories[f.Category] = true
	}
	for _, cat := range []string{"blast_radius", "recent_incidents", "slo_burn", "change_failure", "time_risk"} {
		if !foundCategories[cat] {
			t.Errorf("missing factor category: %s", cat)
		}
	}

	for _, f := range assessment.Factors {
		if f.Category == "blast_radius" && f.Score != 0.5 {
			t.Errorf("expected blast_radius score 0.5 with no provider, got %.2f", f.Score)
		}
		if f.Category == "recent_incidents" && f.Score != 0.3 {
			t.Errorf("expected recent_incidents score 0.3 with no provider, got %.2f", f.Score)
		}
		if f.Category == "slo_burn" && f.Score != 0.2 {
			t.Errorf("expected slo_burn score 0.2 with no provider, got %.2f", f.Score)
		}
	}
}

func TestAssessWithTopology(t *testing.T) {
	c := NewChangeRiskChecker()
	c.SetTopologyProvider(&mockTopologyProvider{
		neighbors: map[string][]string{
			"api-gateway":  {"auth-service", "user-service", "payment-service", "db-primary"},
			"auth-service": {"user-service", "redis-cache"},
		},
	})

	req := sampleRequest()
	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var blastFactor *RiskFactor
	for i := range assessment.Factors {
		if assessment.Factors[i].Category == "blast_radius" {
			blastFactor = &assessment.Factors[i]
		}
	}
	if blastFactor == nil {
		t.Fatal("missing blast_radius factor")
	}

	if blastFactor.Score == 0.5 {
		t.Error("blast_radius score should not be default 0.5 with topology provider")
	}

	if assessment.BlastRadius == nil {
		t.Fatal("expected BlastInfo with topology provider")
	}

	if assessment.BlastRadius.TotalAffected < 2 {
		t.Errorf("expected at least 2 affected services, got %d", assessment.BlastRadius.TotalAffected)
	}
}

func TestAssessBlastRadiusLargeTopology(t *testing.T) {
	c := NewChangeRiskChecker()
	neighbors := make([]string, 25)
	for i := range neighbors {
		neighbors[i] = "svc-" + string(rune('a'+i))
	}
	c.SetTopologyProvider(&mockTopologyProvider{
		neighbors: map[string][]string{
			"api-gateway": neighbors,
		},
	})

	req := ChangeRequest{
		ID:      "cr-002",
		Type:    ChangeDeployment,
		Service: "api-gateway",
		Labels:  map[string]string{},
	}

	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var blastFactor *RiskFactor
	for i := range assessment.Factors {
		if assessment.Factors[i].Category == "blast_radius" {
			blastFactor = &assessment.Factors[i]
		}
	}
	if blastFactor == nil {
		t.Fatal("missing blast_radius factor")
	}

	if blastFactor.Score != 1.0 {
		t.Errorf("expected max blast_radius score 1.0 for 25+ services, got %.2f", blastFactor.Score)
	}
}

func TestAssessWithIncidents(t *testing.T) {
	c := NewChangeRiskChecker()
	c.SetIncidentProvider(&mockIncidentProvider{
		incidents: map[string][]IncidentInfo{
			"api-gateway": {
				{ID: "inc-1", Title: "High latency", Severity: "critical", Service: "api-gateway", Status: "active", StartedAt: time.Now().Add(-2 * time.Hour)},
				{ID: "inc-2", Title: "5xx errors", Severity: "high", Service: "api-gateway", Status: "active", StartedAt: time.Now().Add(-1 * time.Hour)},
				{ID: "inc-3", Title: "Slow DB", Severity: "medium", Service: "api-gateway", Status: "active", StartedAt: time.Now().Add(-3 * time.Hour)},
			},
			"auth-service": {
				{ID: "inc-4", Title: "Token failures", Severity: "high", Service: "auth-service", Status: "active", StartedAt: time.Now().Add(-30 * time.Minute)},
			},
		},
		sloStatus: map[string]*SLOInfo{
			"api-gateway": {Service: "api-gateway", SLOName: "availability", Target: 99.9, Current: 99.5, ErrorBudgetRemaining: 0.05, BurnRate: 5.0},
		},
	})

	req := sampleRequest()
	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range assessment.Factors {
		if f.Category == "recent_incidents" && f.Score <= 0.3 {
			t.Errorf("expected elevated recent_incidents score with active incidents, got %.2f", f.Score)
		}
		if f.Category == "slo_burn" && f.Score <= 0.2 {
			t.Errorf("expected elevated slo_burn score with high burn rate, got %.2f", f.Score)
		}
	}
}

func TestAssessSLOBurnLevels(t *testing.T) {
	tests := []struct {
		name     string
		burnRate float64
		budget   float64
		wantMin  float64
	}{
		{"low burn", 0.5, 0.5, 0.0},
		{"moderate burn", 1.5, 0.5, 0.3},
		{"high burn", 4.0, 0.5, 0.6},
		{"critical burn", 12.0, 0.5, 0.9},
		{"low burn low budget", 0.5, 0.05, 0.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewChangeRiskChecker()
			c.SetIncidentProvider(&mockIncidentProvider{
				incidents: map[string][]IncidentInfo{},
				sloStatus: map[string]*SLOInfo{
					"svc": {Service: "svc", SLOName: "availability", BurnRate: tt.burnRate, ErrorBudgetRemaining: tt.budget},
				},
			})

			req := ChangeRequest{ID: "cr-slo", Type: ChangeDeployment, Service: "svc", Labels: map[string]string{}}
			assessment, err := c.Assess(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, f := range assessment.Factors {
				if f.Category == "slo_burn" {
					if f.Score < tt.wantMin {
						t.Errorf("expected slo_burn score >= %.2f, got %.2f", tt.wantMin, f.Score)
					}
				}
			}
		})
	}
}

func TestOverallScoreCalculation(t *testing.T) {
	c := NewChangeRiskChecker()

	factors := []RiskFactor{
		{Category: "blast_radius", Score: 0.8, Weight: 0.30},
		{Category: "recent_incidents", Score: 0.6, Weight: 0.25},
		{Category: "slo_burn", Score: 0.4, Weight: 0.25},
		{Category: "change_failure", Score: 0.2, Weight: 0.15},
		{Category: "time_risk", Score: 0.2, Weight: 0.05},
	}

	score := c.computeOverallScore(factors)
	expected := (0.8*0.30 + 0.6*0.25 + 0.4*0.25 + 0.2*0.15 + 0.2*0.05) / 1.0
	if delta := score - expected; delta < -0.001 || delta > 0.001 {
		t.Errorf("expected overall score %.4f, got %.4f", expected, score)
	}
}

func TestRiskLevelMapping(t *testing.T) {
	c := NewChangeRiskChecker()
	thresholds := DefaultRiskThresholds()

	tests := []struct {
		score float64
		level RiskLevel
	}{
		{0.10, RiskLow},
		{0.25, RiskMedium},
		{0.50, RiskHigh},
		{0.75, RiskCritical},
		{0.99, RiskCritical},
	}

	for _, tt := range tests {
		got := c.scoreToLevel(tt.score, thresholds)
		if got != tt.level {
			t.Errorf("score %.2f: expected %s, got %s", tt.score, tt.level, got)
		}
	}
}

func TestRecommendations(t *testing.T) {
	tests := []struct {
		name         string
		factors      []RiskFactor
		level        RiskLevel
		wantContains string
	}{
		{
			"blast radius high",
			[]RiskFactor{{Category: "blast_radius", Score: 0.8, Weight: 0.30}},
			RiskHigh,
			"staged rollout",
		},
		{
			"incidents high",
			[]RiskFactor{{Category: "recent_incidents", Score: 0.8, Weight: 0.25}},
			RiskHigh,
			"delay change",
		},
		{
			"slo burn high",
			[]RiskFactor{{Category: "slo_burn", Score: 0.8, Weight: 0.25}},
			RiskHigh,
			"error budget",
		},
		{
			"failure rate high",
			[]RiskFactor{{Category: "change_failure", Score: 0.6, Weight: 0.15}},
			RiskMedium,
			"extra validation",
		},
		{
			"time risk high",
			[]RiskFactor{{Category: "time_risk", Score: 0.7, Weight: 0.05}},
			RiskLow,
			"business hours",
		},
		{
			"critical risk",
			[]RiskFactor{{Category: "blast_radius", Score: 0.9, Weight: 0.30}},
			RiskCritical,
			"SRE lead",
		},
		{
			"all low",
			[]RiskFactor{{Category: "blast_radius", Score: 0.1, Weight: 0.30}},
			RiskLow,
			"acceptable bounds",
		},
	}

	c := NewChangeRiskChecker()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs := c.generateRecommendations(tt.factors, tt.level)
			found := false
			for _, r := range recs {
				if contains(r, tt.wantContains) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected recommendation containing %q, got %v", tt.wantContains, recs)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRecordChangeAndFailureRate(t *testing.T) {
	c := NewChangeRiskChecker()
	now := time.Now()

	records := []ChangeRecord{
		{ID: "ch-1", Type: ChangeDeployment, Service: "api-gateway", Success: true, AssessedAt: now, ActualRisk: RiskLow, Outcome: "success", CompletedAt: now},
		{ID: "ch-2", Type: ChangeDeployment, Service: "api-gateway", Success: false, AssessedAt: now, ActualRisk: RiskMedium, Outcome: "incident", CompletedAt: now},
		{ID: "ch-3", Type: ChangeDeployment, Service: "api-gateway", Success: false, AssessedAt: now, ActualRisk: RiskHigh, Outcome: "rollback", CompletedAt: now},
		{ID: "ch-4", Type: ChangeConfig, Service: "api-gateway", Success: true, AssessedAt: now, ActualRisk: RiskLow, Outcome: "success", CompletedAt: now},
	}

	for _, r := range records {
		c.RecordChange(r)
	}

	rate := c.FailureRate("api-gateway", ChangeDeployment)
	expectedRate := 2.0 / 3.0
	if delta := rate - expectedRate; delta < -0.001 || delta > 0.001 {
		t.Errorf("expected failure rate %.4f, got %.4f", expectedRate, rate)
	}

	allRate := c.FailureRate("api-gateway", "")
	expectedAll := 2.0 / 4.0
	if delta := allRate - expectedAll; delta < -0.001 || delta > 0.001 {
		t.Errorf("expected overall failure rate %.4f, got %.4f", expectedAll, allRate)
	}

	noSvcRate := c.FailureRate("nonexistent", ChangeDeployment)
	if noSvcRate != 0 {
		t.Errorf("expected 0 failure rate for unknown service, got %.4f", noSvcRate)
	}
}

func TestHistoryQuery(t *testing.T) {
	c := NewChangeRiskChecker()
	now := time.Now()

	c.RecordChange(ChangeRecord{ID: "ch-1", Service: "api-gateway", CompletedAt: now.Add(-3 * time.Hour)})
	c.RecordChange(ChangeRecord{ID: "ch-2", Service: "auth-service", CompletedAt: now.Add(-2 * time.Hour)})
	c.RecordChange(ChangeRecord{ID: "ch-3", Service: "api-gateway", CompletedAt: now.Add(-1 * time.Hour)})

	all := c.GetHistory("", 10)
	if len(all) != 3 {
		t.Errorf("expected 3 records, got %d", len(all))
	}

	filtered := c.GetHistory("api-gateway", 10)
	if len(filtered) != 2 {
		t.Errorf("expected 2 api-gateway records, got %d", len(filtered))
	}

	limited := c.GetHistory("", 1)
	if len(limited) != 1 {
		t.Errorf("expected 1 limited record, got %d", len(limited))
	}
}

func TestTimeRiskBusinessHours(t *testing.T) {
	c := NewChangeRiskChecker()

	businessHour := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	req := ChangeRequest{
		ID:          "cr-time",
		Type:        ChangeDeployment,
		Service:     "svc",
		ScheduledAt: &businessHour,
		Labels:      map[string]string{},
	}

	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range assessment.Factors {
		if f.Category == "time_risk" {
			details, ok := f.Details.(map[string]any)
			if !ok {
				t.Fatal("time_risk details should be a map")
			}
			if hourScore, ok := details["hour_score"].(float64); ok {
				if hourScore != 0.2 {
					t.Errorf("expected hour_score 0.2 for business hours, got %.2f", hourScore)
				}
			}
		}
	}
}

func TestTimeRiskFriday(t *testing.T) {
	c := NewChangeRiskChecker()

	friday := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	req := ChangeRequest{
		ID:          "cr-fri",
		Type:        ChangeDeployment,
		Service:     "svc",
		ScheduledAt: &friday,
		Labels:      map[string]string{},
	}

	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range assessment.Factors {
		if f.Category == "time_risk" {
			details, ok := f.Details.(map[string]any)
			if !ok {
				t.Fatal("time_risk details should be a map")
			}
			if dayScore, ok := details["day_score"].(float64); ok {
				if dayScore != 0.5 {
					t.Errorf("expected day_score 0.5 for Friday, got %.2f", dayScore)
				}
			}
		}
	}
}

func TestTimeRiskWeekend(t *testing.T) {
	c := NewChangeRiskChecker()

	saturday := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	req := ChangeRequest{
		ID:          "cr-wknd",
		Type:        ChangeDeployment,
		Service:     "svc",
		ScheduledAt: &saturday,
		Labels:      map[string]string{},
	}

	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range assessment.Factors {
		if f.Category == "time_risk" && f.Score < 0.5 {
			t.Errorf("expected elevated time_risk score for weekend, got %.2f", f.Score)
		}
	}
}

func TestTimeRiskChangeFreeze(t *testing.T) {
	c := NewChangeRiskChecker()

	req := ChangeRequest{
		ID:      "cr-freeze",
		Type:    ChangeDeployment,
		Service: "svc",
		Labels:  map[string]string{"freeze": "true"},
	}

	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range assessment.Factors {
		if f.Category == "time_risk" && f.Score != 0.9 {
			t.Errorf("expected time_risk score 0.9 during change freeze, got %.2f", f.Score)
		}
	}
}

func TestAutoApproval(t *testing.T) {
	c := NewChangeRiskChecker()

	req := ChangeRequest{
		ID:      "cr-low",
		Type:    ChangeDeployment,
		Service: "svc",
		Labels:  map[string]string{},
	}

	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if assessment.OverallScore < DefaultRiskThresholds().AutoApproveMax && !assessment.Approved {
		t.Error("low-risk change should be auto-approved")
	}

	c2 := NewChangeRiskChecker()
	c2.SetTopologyProvider(&mockTopologyProvider{
		neighbors: map[string][]string{
			"svc": makeLargeServiceList(30),
		},
	})
	c2.SetIncidentProvider(&mockIncidentProvider{
		incidents: map[string][]IncidentInfo{
			"svc": makeHighSeverityIncidents(6),
		},
		sloStatus: map[string]*SLOInfo{
			"svc": {Service: "svc", BurnRate: 15.0, ErrorBudgetRemaining: 0.02},
		},
	})

	highReq := ChangeRequest{
		ID:      "cr-high",
		Type:    ChangeHotfix,
		Service: "svc",
		Labels:  map[string]string{"freeze": "true"},
	}
	highAssessment, err := c2.Assess(highReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if highAssessment.Approved {
		t.Error("high-risk change should NOT be auto-approved")
	}
}

func makeLargeServiceList(n int) []string {
	services := make([]string, n)
	for i := range services {
		services[i] = "svc-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
	}
	return services
}

func makeHighSeverityIncidents(n int) []IncidentInfo {
	incidents := make([]IncidentInfo, n)
	for i := range incidents {
		incidents[i] = IncidentInfo{
			ID:        "inc-auto",
			Title:     "Critical issue",
			Severity:  "critical",
			Service:   "svc",
			Status:    "active",
			StartedAt: time.Now().Add(-1 * time.Hour),
		}
	}
	return incidents
}

func TestAssessRequiresService(t *testing.T) {
	c := NewChangeRiskChecker()
	req := ChangeRequest{ID: "cr-empty", Type: ChangeDeployment, Labels: map[string]string{}}
	_, err := c.Assess(req)
	if err == nil {
		t.Error("expected error for empty service")
	}
}

func TestChangeTypeValues(t *testing.T) {
	types := map[ChangeType]bool{
		ChangeDeployment: true,
		ChangeConfig:     true,
		ChangeScaling:    true,
		ChangeRollback:   true,
		ChangeHotfix:     true,
		ChangeMigration:  true,
	}

	for _, ct := range []ChangeType{"deployment", "config_change", "scaling", "rollback", "hotfix", "migration"} {
		if !types[ct] {
			t.Errorf("missing ChangeType constant for %q", ct)
		}
	}
}

func TestRiskPreflightTool(t *testing.T) {
	tool := &RiskPreflightTool{}

	if tool.Name() != "risk_preflight" {
		t.Errorf("expected tool name risk_preflight, got %s", tool.Name())
	}

	schema := tool.InputSchema()
	if schema["type"] != "object" {
		t.Error("expected object type in schema")
	}

	input := map[string]any{
		"type":     "deployment",
		"service":  "api-gateway",
		"services": []any{"api-gateway", "auth-service"},
	}

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assessment, ok := result.(*RiskAssessment)
	if !ok {
		t.Fatal("expected *RiskAssessment result")
	}
	if assessment.RequestID == "" {
		t.Error("expected non-empty request ID")
	}
}

func TestRiskPreflightToolValidation(t *testing.T) {
	tool := &RiskPreflightTool{}

	_, err := tool.Execute(context.Background(), map[string]any{"service": "svc"})
	if err == nil {
		t.Error("expected error for missing type")
	}

	_, err = tool.Execute(context.Background(), map[string]any{"type": "deployment"})
	if err == nil {
		t.Error("expected error for missing service")
	}
}

func TestRiskPreflightToolScheduledAt(t *testing.T) {
	tool := &RiskPreflightTool{}
	scheduledAt := time.Now().Add(2 * time.Hour).Format(time.RFC3339)

	input := map[string]any{
		"type":         "deployment",
		"service":      "svc",
		"scheduled_at": scheduledAt,
	}

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assessment := result.(*RiskAssessment)
	if assessment.RequestID == "" {
		t.Error("expected non-empty request ID")
	}
}

func TestRiskHistoryTool(t *testing.T) {
	tool := &RiskHistoryTool{}

	if tool.Name() != "risk_history" {
		t.Errorf("expected tool name risk_history, got %s", tool.Name())
	}

	c := getChecker()
	now := time.Now()
	c.RecordChange(ChangeRecord{ID: "hist-1", Service: "svc-a", CompletedAt: now})
	c.RecordChange(ChangeRecord{ID: "hist-2", Service: "svc-b", CompletedAt: now})

	result, err := tool.Execute(context.Background(), map[string]any{"service": "svc-a", "limit": float64(10)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records, ok := result.([]ChangeRecord)
	if !ok {
		t.Fatal("expected []ChangeRecord result")
	}
	if len(records) < 1 {
		t.Error("expected at least 1 record")
	}
}

func TestRegisterTools(t *testing.T) {
	registry := tools.NewRegistryWithoutCache()
	RegisterTools(registry)

	if registry.Get("risk_preflight") == nil {
		t.Error("risk_preflight tool not registered")
	}
	if registry.Get("risk_history") == nil {
		t.Error("risk_history tool not registered")
	}
}

func TestInitChangeRiskChecker(t *testing.T) {
	topo := &mockTopologyProvider{neighbors: map[string][]string{"svc": {"dep1"}}}
	inc := &mockIncidentProvider{incidents: map[string][]IncidentInfo{}, sloStatus: map[string]*SLOInfo{}}

	c := InitChangeRiskChecker(topo, inc)
	if c == nil {
		t.Fatal("expected non-nil checker")
	}

	if DefaultChangeRiskChecker() != c {
		t.Error("expected InitChangeRiskChecker to set default checker")
	}
}

func TestDefaultChangeRiskCheckerLazyInit(t *testing.T) {
	defaultCheckerMu.Lock()
	defaultChecker = nil
	defaultCheckerMu.Unlock()

	c := getChecker()
	if c == nil {
		t.Fatal("expected lazy-init checker")
	}
}

func TestAssessChangeFailureInsufficientData(t *testing.T) {
	c := NewChangeRiskChecker()
	now := time.Now()
	c.RecordChange(ChangeRecord{ID: "ch-1", Service: "svc", Success: true, AssessedAt: now, CompletedAt: now})

	req := ChangeRequest{ID: "cr-insuf", Type: ChangeDeployment, Service: "svc", Labels: map[string]string{}}
	assessment, err := c.Assess(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range assessment.Factors {
		if f.Category == "change_failure" && f.Score != 0.3 {
			t.Errorf("expected 0.3 for insufficient data, got %.2f", f.Score)
		}
	}
}

func TestAllServicesDedup(t *testing.T) {
	c := NewChangeRiskChecker()
	req := ChangeRequest{
		ID:       "cr-dup",
		Type:     ChangeDeployment,
		Service:  "svc-a",
		Services: []string{"svc-a", "svc-b", "svc-a", ""},
		Labels:   map[string]string{},
	}

	allSvc := c.allServices(req)
	if len(allSvc) != 2 {
		t.Errorf("expected 2 unique services, got %d: %v", len(allSvc), allSvc)
	}
}
