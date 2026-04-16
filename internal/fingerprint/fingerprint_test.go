package fingerprint

import (
	"database/sql"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func makeTestIncidentData(id string) IncidentData {
	now := time.Date(2024, 6, 10, 14, 30, 0, 0, time.UTC)
	mitigated := now.Add(45 * time.Minute)
	resolved := now.Add(90 * time.Minute)

	return IncidentData{
		ID:               id,
		Title:            "Database connection timeout on api-gateway",
		Severity:         "high",
		Status:           "mitigated",
		Service:          "api-gateway",
		StartedAt:        now,
		MitigatedAt:      &mitigated,
		ResolvedAt:       &resolved,
		AffectedServices: []string{"api-gateway", "auth-service", "user-service"},
		Labels: map[string]string{
			"service_type":   "api",
			"data_loss":      "false",
			"time_to_triage": "15",
		},
		BlastRadius:         0.6,
		IsCriticalPath:      true,
		DependencyDepth:     3,
		SLOBurnRate:         2.5,
		ErrorBudgetUsed:     0.35,
		ToolCallCount:       25,
		InvestigationSteps:  8,
		RemediationAttempts: 2,
		AutoRemediated:      false,
		HumanIntervention:   true,
		Escalated:           true,
		EscalationCount:     3,
		HasRunbook:          true,
		HasPostmortem:       false,
		IsRecurring:         true,
		SimilarPastCount:    4,
		CategoryHints:       []string{"database", "network"},
	}
}

func TestGenerateFingerprint(t *testing.T) {
	data := makeTestIncidentData("inc-001")
	fp := GenerateFingerprint(data)

	if fp.IncidentID != "inc-001" {
		t.Errorf("expected incident_id=inc-001, got %s", fp.IncidentID)
	}
	if fp.Version != fingerprintVersion {
		t.Errorf("expected version=%d, got %d", fingerprintVersion, fp.Version)
	}
	if fp.GeneratedAt.IsZero() {
		t.Error("generated_at should not be zero")
	}

	nonZero := 0
	for i, v := range fp.Vector {
		if v != 0 {
			nonZero++
		}
		if v < 0 || v > 1 {
			t.Errorf("vector[%d]=%f out of [0,1] range", i, v)
		}
	}
	if nonZero == 0 {
		t.Error("expected some non-zero vector elements")
	}

	for i := 39; i < VectorSize; i++ {
		if fp.Vector[i] != 0 {
			t.Errorf("reserved vector[%d] should be 0, got %f", i, fp.Vector[i])
		}
	}
}

func TestTemporalFeatures(t *testing.T) {
	monday9am := time.Date(2024, 6, 10, 9, 0, 0, 0, time.UTC)
	mitigated := monday9am.Add(2 * time.Hour)
	resolved := monday9am.Add(5 * time.Hour)

	data := IncidentData{
		ID:          "inc-t1",
		Title:       "test",
		Severity:    "medium",
		StartedAt:   monday9am,
		MitigatedAt: &mitigated,
		ResolvedAt:  &resolved,
		Labels:      map[string]string{"time_to_triage": "30"},
	}

	fp := GenerateFingerprint(data)
	tf := fp.Features.Temporal

	hourNorm := 9.0 / 23.0
	if math.Abs(tf.HourOfDay-hourNorm) > 0.01 {
		t.Errorf("hour_of_day: expected ~%f, got %f", hourNorm, tf.HourOfDay)
	}

	dayNorm := float64(time.Monday) / 6.0
	if math.Abs(tf.DayOfWeek-dayNorm) > 0.01 {
		t.Errorf("day_of_week: expected ~%f, got %f", dayNorm, tf.DayOfWeek)
	}

	if tf.IsBusinessHours != 1.0 {
		t.Errorf("is_business_hours: expected 1.0, got %f", tf.IsBusinessHours)
	}
	if tf.IsWeekend != 0.0 {
		t.Errorf("is_weekend: expected 0.0, got %f", tf.IsWeekend)
	}

	expectedDuration := 5.0 / 72.0
	if math.Abs(tf.Duration-expectedDuration) > 0.01 {
		t.Errorf("duration: expected ~%f, got %f", expectedDuration, tf.Duration)
	}

	if tf.TimeToTriage < 0 || tf.TimeToTriage > 1 {
		t.Errorf("time_to_triage out of range: %f", tf.TimeToTriage)
	}

	if tf.TimeToMitigate < 0 || tf.TimeToMitigate > 1 {
		t.Errorf("time_to_mitigate out of range: %f", tf.TimeToMitigate)
	}

	saturday := time.Date(2024, 6, 15, 14, 0, 0, 0, time.UTC)
	data2 := IncidentData{ID: "inc-t2", Title: "test", Severity: "low", StartedAt: saturday}
	fp2 := GenerateFingerprint(data2)
	if fp2.Features.Temporal.IsWeekend != 1.0 {
		t.Errorf("weekend: expected 1.0, got %f", fp2.Features.Temporal.IsWeekend)
	}
	if fp2.Features.Temporal.IsBusinessHours != 0.0 {
		t.Errorf("business hours on weekend: expected 0.0, got %f", fp2.Features.Temporal.IsBusinessHours)
	}
}

func TestSeverityFeatures(t *testing.T) {
	tests := []struct {
		severity  string
		wantLevel float64
	}{
		{"info", 0.0},
		{"low", 0.25},
		{"medium", 0.5},
		{"high", 0.75},
		{"critical", 1.0},
		{"unknown", 0.5},
	}

	for _, tt := range tests {
		data := IncidentData{ID: "inc-sev", Title: "test", Severity: tt.severity, StartedAt: time.Now()}
		fp := GenerateFingerprint(data)
		if math.Abs(fp.Features.Severity.SeverityLevel-tt.wantLevel) > 0.01 {
			t.Errorf("severity %s: expected %f, got %f", tt.severity, tt.wantLevel, fp.Features.Severity.SeverityLevel)
		}
	}

	data := IncidentData{
		ID: "inc-sev-esc", Title: "test", Severity: "high",
		StartedAt: time.Now(), Escalated: true, EscalationCount: 7,
	}
	fp := GenerateFingerprint(data)
	if fp.Features.Severity.Escalated != 1.0 {
		t.Errorf("escalated: expected 1.0, got %f", fp.Features.Severity.Escalated)
	}
	if fp.Features.Severity.EscalationCount != 1.0 {
		t.Errorf("escalation_count (7, cap 5): expected 1.0, got %f", fp.Features.Severity.EscalationCount)
	}
}

func TestTopologyFeatures(t *testing.T) {
	services := make([]string, 25)
	for i := range services {
		services[i] = "svc-" + string(rune('A'+i))
	}

	data := IncidentData{
		ID:               "inc-topo",
		Title:            "test",
		Severity:         "medium",
		StartedAt:        time.Now(),
		AffectedServices: services,
		BlastRadius:      0.8,
		IsCriticalPath:   true,
		DependencyDepth:  7,
	}

	fp := GenerateFingerprint(data)
	tf := fp.Features.Topology

	if tf.AffectedServiceCount != 1.0 {
		t.Errorf("affected_service_count (25, cap 20): expected 1.0, got %f", tf.AffectedServiceCount)
	}
	if math.Abs(tf.BlastRadiusScore-0.8) > 0.01 {
		t.Errorf("blast_radius: expected 0.8, got %f", tf.BlastRadiusScore)
	}
	if tf.IsCriticalPath != 1.0 {
		t.Errorf("is_critical_path: expected 1.0, got %f", tf.IsCriticalPath)
	}
	if tf.DependencyDepth != 1.0 {
		t.Errorf("dependency_depth (7, cap 5): expected 1.0, got %f", tf.DependencyDepth)
	}
}

func TestServiceFeatures(t *testing.T) {
	tests := []struct {
		service  string
		wantType float64
	}{
		{"frontend-app", 0.0},
		{"api-gateway", 0.25},
		{"worker-pool", 0.5},
		{"postgres-db", 0.75},
		{"infra-platform", 1.0},
		{"unknown-svc", 0.25},
	}

	for _, tt := range tests {
		data := IncidentData{ID: "inc-svc", Title: "test", Severity: "low", Service: tt.service, StartedAt: time.Now()}
		fp := GenerateFingerprint(data)
		if math.Abs(fp.Features.Service.ServiceType-tt.wantType) > 0.01 {
			t.Errorf("service %s: expected type %f, got %f", tt.service, tt.wantType, fp.Features.Service.ServiceType)
		}
	}

	data := IncidentData{
		ID: "inc-svc-label", Title: "test", Severity: "low",
		Service: "custom", Labels: map[string]string{"service_type": "datastore"},
		AffectedServices: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
		StartedAt:        time.Now(),
	}
	fp := GenerateFingerprint(data)
	if fp.Features.Service.ServiceType != 0.75 {
		t.Errorf("service_type from label: expected 0.75, got %f", fp.Features.Service.ServiceType)
	}
	if fp.Features.Service.ServiceCount != 1.0 {
		t.Errorf("service_count (12, cap 10): expected 1.0, got %f", fp.Features.Service.ServiceCount)
	}

	hash1 := hashServiceName("api-gateway")
	hash2 := hashServiceName("api-gateway")
	if hash1 != hash2 {
		t.Error("hash should be deterministic")
	}
	if hash1 < 0 || hash1 > 1 {
		t.Errorf("hash out of range: %f", hash1)
	}
	if hashServiceName("") != 0.0 {
		t.Error("empty name should hash to 0")
	}
}

func TestImpactFeatures(t *testing.T) {
	data := IncidentData{
		ID:              "inc-impact",
		Title:           "test",
		Severity:        "critical",
		StartedAt:       time.Now(),
		BlastRadius:     0.3,
		SLOBurnRate:     3.0,
		ErrorBudgetUsed: 0.8,
		Labels:          map[string]string{"data_loss": "true"},
	}

	fp := GenerateFingerprint(data)
	imp := fp.Features.Impact

	if imp.UserImpactScore != 1.0 {
		t.Errorf("user_impact for critical: expected 1.0, got %f", imp.UserImpactScore)
	}

	if imp.SLOViolationCount != 0.6 {
		t.Errorf("slo_violation (3.0/5.0): expected 0.6, got %f", imp.SLOViolationCount)
	}

	if math.Abs(imp.ErrorBudgetBurned-0.8) > 0.01 {
		t.Errorf("error_budget: expected 0.8, got %f", imp.ErrorBudgetBurned)
	}

	if imp.DataLoss != 1.0 {
		t.Errorf("data_loss: expected 1.0, got %f", imp.DataLoss)
	}
}

func TestResponseFeatures(t *testing.T) {
	data := IncidentData{
		ID:                  "inc-resp",
		Title:               "test",
		Severity:            "medium",
		StartedAt:           time.Now(),
		ToolCallCount:       60,
		InvestigationSteps:  25,
		RemediationAttempts: 6,
		AutoRemediated:      true,
		HumanIntervention:   true,
	}

	fp := GenerateFingerprint(data)
	rf := fp.Features.Response

	if rf.ToolCallCount != 1.0 {
		t.Errorf("tool_call_count (60, cap 50): expected 1.0, got %f", rf.ToolCallCount)
	}
	if rf.InvestigationSteps != 1.0 {
		t.Errorf("investigation_steps (25, cap 20): expected 1.0, got %f", rf.InvestigationSteps)
	}
	if rf.RemediationAttempts != 1.0 {
		t.Errorf("remediation_attempts (6, cap 5): expected 1.0, got %f", rf.RemediationAttempts)
	}
	if rf.AutoRemediated != 1.0 {
		t.Errorf("auto_remediated: expected 1.0, got %f", rf.AutoRemediated)
	}
	if rf.HumanIntervention != 1.0 {
		t.Errorf("human_intervention: expected 1.0, got %f", rf.HumanIntervention)
	}
}

func TestCategoryFeatures(t *testing.T) {
	tests := []struct {
		title       string
		hints       []string
		wantNetwork bool
		wantDB      bool
		wantDeploy  bool
	}{
		{
			title:       "Network timeout connecting to database",
			hints:       []string{},
			wantNetwork: true,
			wantDB:      true,
		},
		{
			title:      "Deployment rollback due to errors",
			hints:      []string{"deployment"},
			wantDeploy: true,
		},
		{
			title:       "DNS resolution failure",
			hints:       nil,
			wantNetwork: true,
		},
		{
			title: "Application crash with OOM",
			hints: []string{"app"},
		},
		{
			title: "Security certificate expired",
			hints: []string{"security"},
		},
		{
			title: "Misconfig in rate limiting",
			hints: []string{"config"},
		},
		{
			title: "Scaling threshold exceeded",
			hints: []string{"capacity"},
		},
		{
			title: "Server node unreachable",
			hints: []string{"infra"},
		},
	}

	for _, tt := range tests {
		cat := inferCategories(tt.title, nil, tt.hints)
		if tt.wantNetwork && cat.IsNetworkIssue != 1.0 {
			t.Errorf("title=%q: expected network=1.0, got %f", tt.title, cat.IsNetworkIssue)
		}
		if tt.wantDB && cat.IsDatabaseIssue != 1.0 {
			t.Errorf("title=%q: expected database=1.0, got %f", tt.title, cat.IsDatabaseIssue)
		}
		if tt.wantDeploy && cat.IsDeploymentIssue != 1.0 {
			t.Errorf("title=%q: expected deployment=1.0, got %f", tt.title, cat.IsDeploymentIssue)
		}
	}
}

func TestLabelFeatures(t *testing.T) {
	data := IncidentData{
		ID:               "inc-label",
		Title:            "test",
		Severity:         "medium",
		StartedAt:        time.Now(),
		HasRunbook:       true,
		HasPostmortem:    true,
		IsRecurring:      true,
		SimilarPastCount: 12,
	}

	fp := GenerateFingerprint(data)
	lf := fp.Features.Label

	if lf.HasRunbook != 1.0 {
		t.Errorf("has_runbook: expected 1.0, got %f", lf.HasRunbook)
	}
	if lf.HasPostmortem != 1.0 {
		t.Errorf("has_postmortem: expected 1.0, got %f", lf.HasPostmortem)
	}
	if lf.IsRecurring != 1.0 {
		t.Errorf("is_recurring: expected 1.0, got %f", lf.IsRecurring)
	}
	if lf.SimilarPastCount != 1.0 {
		t.Errorf("similar_past_count (12, cap 10): expected 1.0, got %f", lf.SimilarPastCount)
	}
}

func TestCosineSimilarity(t *testing.T) {
	var a [VectorSize]float64
	var b [VectorSize]float64

	for i := 0; i < VectorSize; i++ {
		a[i] = 1.0
		b[i] = 1.0
	}
	sim := CosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 0.0001 {
		t.Errorf("identical vectors: expected 1.0, got %f", sim)
	}

	for i := 0; i < VectorSize; i++ {
		b[i] = 0.0
	}
	sim = CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("a vs zero vector: expected 0, got %f", sim)
	}

	for i := 0; i < VectorSize; i++ {
		a[i] = 1.0
		b[i] = -1.0
	}
	sim = CosineSimilarity(a, b)
	if math.Abs(sim-(-1.0)) > 0.0001 {
		t.Errorf("opposite vectors: expected -1.0, got %f", sim)
	}

	for i := 0; i < VectorSize; i++ {
		a[i] = 0.0
		b[i] = 0.0
	}
	sim = CosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("zero vectors: expected 0, got %f", sim)
	}

	a[0] = 1.0
	a[1] = 0.0
	b[0] = 0.0
	b[1] = 1.0
	sim = CosineSimilarity(a, b)
	if math.Abs(sim) > 0.0001 {
		t.Errorf("orthogonal vectors: expected ~0, got %f", sim)
	}
}

func TestVectorToBytesRoundTrip(t *testing.T) {
	var original [VectorSize]float64
	for i := 0; i < VectorSize; i++ {
		original[i] = float64(i) / 64.0
	}

	blob := vectorToBytes(original)
	if len(blob) != VectorSize*8 {
		t.Fatalf("blob size: expected %d, got %d", VectorSize*8, len(blob))
	}

	decoded, err := bytesToVector(blob)
	if err != nil {
		t.Fatalf("bytesToVector error: %v", err)
	}

	for i := 0; i < VectorSize; i++ {
		if math.Abs(original[i]-decoded[i]) > 1e-15 {
			t.Errorf("vector[%d]: expected %f, got %f", i, original[i], decoded[i])
		}
	}

	_, err = bytesToVector([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error for invalid blob size")
	}
}

func TestSQLiteSaveLoadRoundTrip(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := initDB(db); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	data := makeTestIncidentData("inc-db-001")
	fp := GenerateFingerprint(data)

	if err := saveFingerprint(db, &fp); err != nil {
		t.Fatalf("saveFingerprint: %v", err)
	}

	loaded, err := loadFingerprint(db, "inc-db-001")
	if err != nil {
		t.Fatalf("loadFingerprint: %v", err)
	}
	if loaded == nil {
		t.Fatal("loaded fingerprint is nil")
	}

	if loaded.IncidentID != fp.IncidentID {
		t.Errorf("incident_id: expected %s, got %s", fp.IncidentID, loaded.IncidentID)
	}
	if loaded.Version != fp.Version {
		t.Errorf("version: expected %d, got %d", fp.Version, loaded.Version)
	}

	for i := 0; i < VectorSize; i++ {
		if math.Abs(fp.Vector[i]-loaded.Vector[i]) > 1e-15 {
			t.Errorf("vector[%d]: expected %f, got %f", i, fp.Vector[i], loaded.Vector[i])
		}
	}

	missing, err := loadFingerprint(db, "nonexistent")
	if err != nil {
		t.Errorf("loadFingerprint nonexistent: unexpected error %v", err)
	}
	if missing != nil {
		t.Error("expected nil for nonexistent fingerprint")
	}

	fp2 := GenerateFingerprint(IncidentData{
		ID: "inc-db-001", Title: "updated", Severity: "critical", StartedAt: time.Now(),
	})
	if err := saveFingerprint(db, &fp2); err != nil {
		t.Fatalf("upsert saveFingerprint: %v", err)
	}
	upserted, _ := loadFingerprint(db, "inc-db-001")
	if upserted.Features.Severity.SeverityLevel != 1.0 {
		t.Errorf("upsert: expected severity 1.0, got %f", upserted.Features.Severity.SeverityLevel)
	}
}

func TestLoadAllFingerprints(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := initDB(db); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	for i := 0; i < 5; i++ {
		data := IncidentData{
			ID:        fmt.Sprintf("inc-all-%03d", i),
			Title:     "test",
			Severity:  "medium",
			StartedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}
		fp := GenerateFingerprint(data)
		if err := saveFingerprint(db, &fp); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}

	all, err := loadAllFingerprints(db)
	if err != nil {
		t.Fatalf("loadAllFingerprints: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("expected 5 fingerprints, got %d", len(all))
	}
}

func TestSearchSimilar(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	engine := NewFingerprintEngine(db)
	if err := initDB(db); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	baseTime := time.Date(2024, 6, 10, 14, 0, 0, 0, time.UTC)
	similarData := IncidentData{
		ID: "inc-sim-1", Title: "Database connection timeout on api-gateway",
		Severity: "high", Service: "api-gateway", StartedAt: baseTime,
		BlastRadius: 0.6, IsCriticalPath: true, DependencyDepth: 3,
		SLOBurnRate: 2.5, ErrorBudgetUsed: 0.35,
		ToolCallCount: 25, InvestigationSteps: 8, RemediationAttempts: 2,
		Escalated: true, EscalationCount: 3,
		CategoryHints: []string{"database", "network"},
	}
	engine.StoreFingerprint(similarData)

	similarData2 := IncidentData{
		ID: "inc-sim-2", Title: "Database query timeout on auth-service",
		Severity: "high", Service: "auth-service", StartedAt: baseTime.Add(1 * time.Hour),
		BlastRadius: 0.5, IsCriticalPath: true, DependencyDepth: 2,
		SLOBurnRate: 2.0, ErrorBudgetUsed: 0.3,
		ToolCallCount: 20, InvestigationSteps: 6, RemediationAttempts: 1,
		Escalated: true, EscalationCount: 2,
		CategoryHints: []string{"database"},
	}
	engine.StoreFingerprint(similarData2)

	differentData := IncidentData{
		ID: "inc-diff-1", Title: "Frontend CSS styling issue",
		Severity: "low", Service: "frontend-app", StartedAt: time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
		BlastRadius: 0.1, IsCriticalPath: false, DependencyDepth: 0,
		SLOBurnRate: 0.1, ErrorBudgetUsed: 0.01,
		ToolCallCount: 3, InvestigationSteps: 1, RemediationAttempts: 0,
		CategoryHints: []string{"app"},
	}
	engine.StoreFingerprint(differentData)

	results, err := engine.SearchSimilar("inc-sim-1", 0.3, 10)
	if err != nil {
		t.Fatalf("SearchSimilar: %v", err)
	}

	foundSimilar := false
	for _, r := range results {
		if r.IncidentID == "inc-sim-2" {
			foundSimilar = true
			if r.Similarity < 0.3 {
				t.Errorf("similar incident similarity too low: %f", r.Similarity)
			}
		}
	}
	if !foundSimilar {
		t.Error("expected to find inc-sim-2 as similar to inc-sim-1")
	}

	results2, err := engine.SearchSimilar("inc-sim-1", 0.99, 10)
	if err != nil {
		t.Fatalf("SearchSimilar high threshold: %v", err)
	}
	for _, r := range results2 {
		if r.IncidentID != "inc-sim-1" {
			t.Errorf("at threshold 0.99, only self should match, got %s (sim=%f)", r.IncidentID, r.Similarity)
		}
	}

	_, err = engine.SearchSimilar("nonexistent", 0.7, 10)
	if err == nil {
		t.Error("expected error for nonexistent incident")
	}
}

func TestSearchByVector(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	engine := NewFingerprintEngine(db)
	if err := initDB(db); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	for i := 0; i < 3; i++ {
		data := IncidentData{
			ID: fmt.Sprintf("inc-vec-%d", i), Title: "test", Severity: "medium",
			StartedAt: time.Now(), BlastRadius: float64(i) * 0.3,
		}
		engine.StoreFingerprint(data)
	}

	var queryVec [VectorSize]float64
	queryVec[11] = 0.3

	results, err := engine.SearchByVector(queryVec, 0.01, 10)
	if err != nil {
		t.Fatalf("SearchByVector: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected some results with low threshold")
	}

	if len(results) > 1 {
		if results[0].Similarity < results[1].Similarity {
			t.Error("results should be sorted by similarity descending")
		}
	}
}

func TestFeatureMatchIdentification(t *testing.T) {
	var a, b [VectorSize]float64

	for i := 0; i < 39; i++ {
		a[i] = 0.5
		b[i] = 0.5
	}

	match := identifyFeatureMatch(a, b, 0.8)
	if match == "" {
		t.Error("expected feature match for identical vectors")
	}

	expectedCats := []string{"temporal", "severity", "topology", "service", "impact", "response", "category", "label"}
	for _, cat := range expectedCats {
		if !containsCategory(match, cat) {
			t.Errorf("expected category %s in match (got: %s)", cat, match)
		}
	}

	for i := 0; i < VectorSize; i++ {
		b[i] = 0.0
	}
	matchEmpty := identifyFeatureMatch(a, b, 0.8)
	if matchEmpty != "" {
		t.Errorf("expected no match for zero vector, got %q", matchEmpty)
	}
}

func TestFingerprintVersionTracking(t *testing.T) {
	data := makeTestIncidentData("inc-ver")
	fp := GenerateFingerprint(data)
	if fp.Version != 1 {
		t.Errorf("expected version 1, got %d", fp.Version)
	}
}

func TestLargeScaleSearch(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	engine := NewFingerprintEngine(db)
	if err := initDB(db); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	for i := 0; i < 120; i++ {
		data := IncidentData{
			ID:          fmt.Sprintf("inc-scale-%03d", i),
			Title:       fmt.Sprintf("Incident %d", i),
			Severity:    "medium",
			StartedAt:   time.Now().Add(time.Duration(i) * time.Minute),
			BlastRadius: float64(i%10) / 10.0,
			Service:     "svc",
		}
		engine.StoreFingerprint(data)
	}

	results, err := engine.SearchSimilar("inc-scale-050", 0.5, 10)
	if err != nil {
		t.Fatalf("large scale search: %v", err)
	}
	if len(results) > 10 {
		t.Errorf("limit 10 exceeded: got %d", len(results))
	}

	engine2 := NewFingerprintEngine(db)
	if err := engine2.LoadCache(); err != nil {
		t.Fatalf("LoadCache: %v", err)
	}

	results2, err := engine2.SearchSimilar("inc-scale-050", 0.5, 10)
	if err != nil {
		t.Fatalf("search after reload: %v", err)
	}
	if len(results2) != len(results) {
		t.Errorf("cache reload changed results: before=%d, after=%d", len(results), len(results2))
	}
}

func TestEmptyZeroVectorEdgeCase(t *testing.T) {
	var zero [VectorSize]float64
	var nonzero [VectorSize]float64
	nonzero[0] = 1.0

	sim := CosineSimilarity(zero, nonzero)
	if sim != 0 {
		t.Errorf("zero vs nonzero: expected 0, got %f", sim)
	}

	sim = CosineSimilarity(zero, zero)
	if sim != 0 {
		t.Errorf("zero vs zero: expected 0, got %f", sim)
	}

	data := IncidentData{ID: "inc-empty", Title: "", Severity: "", StartedAt: time.Now()}
	fp := GenerateFingerprint(data)
	if fp.IncidentID != "inc-empty" {
		t.Errorf("empty data: expected id=inc-empty, got %s", fp.IncidentID)
	}
}

func TestGetFingerprint(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	engine := NewFingerprintEngine(db)
	if err := initDB(db); err != nil {
		t.Fatalf("initDB: %v", err)
	}

	data := makeTestIncidentData("inc-get")
	engine.StoreFingerprint(data)

	fp, err := engine.GetFingerprint("inc-get")
	if err != nil {
		t.Fatalf("GetFingerprint: %v", err)
	}
	if fp == nil {
		t.Fatal("expected non-nil fingerprint")
	}
	if fp.IncidentID != "inc-get" {
		t.Errorf("expected inc-get, got %s", fp.IncidentID)
	}

	missing, err := engine.GetFingerprint("nonexistent")
	if err != nil {
		t.Fatalf("GetFingerprint nonexistent: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for nonexistent fingerprint")
	}
}

func TestInitFingerprintEngine(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	engine, err := InitFingerprintEngine(db, nil)
	if err != nil {
		t.Fatalf("InitFingerprintEngine: %v", err)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}

	current := DefaultFingerprintEngine()
	if current != engine {
		t.Error("default engine not set correctly")
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	return db
}

func splitCategories(s string) []string {
	if s == "" {
		return nil
	}
	parts := sort.StringSlice(splitByComma(s))
	sort.Sort(parts)
	return parts
}

func containsCategory(match, cat string) bool {
	for _, c := range splitByComma(match) {
		if c == cat {
			return true
		}
	}
	return false
}

func splitByComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}
