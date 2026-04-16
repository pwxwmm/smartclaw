package alertengine

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestFingerprintAlert_SameAlert(t *testing.T) {
	now := time.Now()
	a1 := Alert{Source: "prometheus", Name: "HighCPU", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now}
	a2 := Alert{Source: "prometheus", Name: "HighCPU", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(5 * time.Minute)}

	fp1 := FingerprintAlert(&a1)
	fp2 := FingerprintAlert(&a2)

	if fp1 != fp2 {
		t.Errorf("same alert fields should produce same fingerprint: %s != %s", fp1, fp2)
	}
}

func TestFingerprintAlert_DifferentInstanceLabel(t *testing.T) {
	a1 := Alert{Source: "prometheus", Name: "HighCPU", Service: "api", Labels: map[string]string{"env": "prod", "instance": "10.0.0.1"}}
	a2 := Alert{Source: "prometheus", Name: "HighCPU", Service: "api", Labels: map[string]string{"env": "prod", "instance": "10.0.0.2"}}

	fp1 := FingerprintAlert(&a1)
	fp2 := FingerprintAlert(&a2)

	if fp1 != fp2 {
		t.Errorf("alerts differing only in excluded labels should have same fingerprint: %s != %s", fp1, fp2)
	}
}

func TestFingerprintAlert_DifferentName(t *testing.T) {
	a1 := Alert{Source: "prometheus", Name: "HighCPU", Service: "api", Labels: map[string]string{"env": "prod"}}
	a2 := Alert{Source: "prometheus", Name: "HighMem", Service: "api", Labels: map[string]string{"env": "prod"}}

	fp1 := FingerprintAlert(&a1)
	fp2 := FingerprintAlert(&a2)

	if fp1 == fp2 {
		t.Error("alerts with different names should have different fingerprints")
	}
}

func TestNormalizeLabels(t *testing.T) {
	labels := map[string]string{
		"env":          "prod",
		"instance":     "10.0.0.1",
		"pod":          "api-xyz",
		"team":         "backend",
		"container":    "main",
		"hostname":     "node1",
		"pod_name":     "api-abc",
		"container_id": "docker123",
		"job":          "api-job",
		"endpoint":     "/health",
		"region":       "us-east",
	}

	normalized := NormalizeLabels(labels)

	expectedCount := 3 // env, team, region
	if len(normalized) != expectedCount {
		t.Errorf("expected %d normalized labels, got %d: %v", expectedCount, len(normalized), normalized)
	}

	for _, k := range []string{"env", "team", "region"} {
		if _, ok := normalized[k]; !ok {
			t.Errorf("expected key %q in normalized labels", k)
		}
	}

	for _, k := range []string{"instance", "pod", "container", "hostname", "pod_name", "container_id", "job", "endpoint"} {
		if _, ok := normalized[k]; ok {
			t.Errorf("excluded key %q should not be in normalized labels", k)
		}
	}
}

func TestIngest_Dedup(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	for i := 0; i < 5; i++ {
		a := Alert{
			Source:   "prometheus",
			Name:     "HighCPU",
			Severity: "high",
			Service:  "api",
			Labels:   map[string]string{"env": "prod"},
			FiredAt:  now.Add(time.Duration(i) * time.Minute),
			Status:   "firing",
		}
		e.Ingest(a)
	}

	da := e.deduped[FingerprintAlert(&Alert{Source: "prometheus", Name: "HighCPU", Service: "api", Labels: map[string]string{"env": "prod"}})]
	if da == nil {
		t.Fatal("expected deduped alert to exist")
	}
	if da.Count != 5 {
		t.Errorf("expected count 5, got %d", da.Count)
	}
	if da.Severity != "high" {
		t.Errorf("expected severity high, got %s", da.Severity)
	}
}

func TestIngest_HighestSeverity(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	a1 := Alert{Source: "prometheus", Name: "HighCPU", Severity: "medium", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"}
	a2 := Alert{Source: "prometheus", Name: "HighCPU", Severity: "critical", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(time.Minute), Status: "firing"}

	e.Ingest(a1)
	da := e.Ingest(a2)

	if da.Severity != "critical" {
		t.Errorf("expected severity to be critical (highest), got %s", da.Severity)
	}
}

func TestAutoEscalation(t *testing.T) {
	e := NewAlertEngine()
	e.autoEscalateThreshold = 5
	now := time.Now()

	fp := FingerprintAlert(&Alert{Source: "prometheus", Name: "HighCPU", Service: "api", Labels: map[string]string{"env": "prod"}})

	for i := 0; i < 6; i++ {
		a := Alert{
			Source:   "prometheus",
			Name:     "HighCPU",
			Severity: "medium",
			Service:  "api",
			Labels:   map[string]string{"env": "prod"},
			FiredAt:  now.Add(time.Duration(i) * time.Minute),
			Status:   "firing",
		}
		e.Ingest(a)
	}

	result := e.Correlate()
	found := false
	for _, g := range result.Groups {
		for _, da := range g.Alerts {
			if da.Fingerprint == fp && da.Severity == "high" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected severity to be auto-escalated from medium to high with 6 alerts (> threshold 5)")
	}
}

func TestTimeWindowGrouping(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	a1 := Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"}
	a2 := Alert{Source: "datadog", Name: "HighLatency", Severity: "medium", Service: "web", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(2 * time.Minute), Status: "firing"}
	a3 := Alert{Source: "prometheus", Name: "DiskFull", Severity: "critical", Service: "db", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(10 * time.Minute), Status: "firing"}

	e.Ingest(a1)
	e.Ingest(a2)
	e.Ingest(a3)

	result := e.Correlate()

	timeWindowGroups := 0
	for _, g := range result.Groups {
		if g.Correlation == "time_window" {
			timeWindowGroups++
		}
	}

	if timeWindowGroups < 1 {
		t.Error("expected at least one time_window group for alerts within 5 minutes")
	}

	for _, g := range result.Groups {
		if g.Correlation == "time_window" && g.RootAlert != nil {
			if g.RootAlert.Severity != "critical" && g.RootAlert.Severity != "high" {
				t.Errorf("root alert should be most severe, got %s", g.RootAlert.Severity)
			}
		}
	}
}

type mockTopologyProvider struct {
	neighbors map[string][]string
}

func (m *mockTopologyProvider) GetNeighbors(serviceID string, depth int) ([]string, error) {
	if n, ok := m.neighbors[serviceID]; ok {
		return n, nil
	}
	return nil, nil
}

func TestTopologyCorrelation(t *testing.T) {
	e := NewAlertEngine()
	tp := &mockTopologyProvider{
		neighbors: map[string][]string{
			"api":    {"web", "cache", "db"},
			"web":    {"api", "cdn"},
			"db":     {"api", "cache"},
			"cache":  {"api", "db"},
			"cdn":    {"web"},
			"auth":   {"userdb"},
			"userdb": {"auth"},
		},
	}
	e.SetTopologyProvider(tp)

	now := time.Now()
	a1 := Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"}
	a2 := Alert{Source: "datadog", Name: "HighLatency", Severity: "medium", Service: "db", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(2 * time.Minute), Status: "firing"}
	a3 := Alert{Source: "cloudwatch", Name: "AuthFailure", Severity: "low", Service: "auth", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(15 * time.Minute), Status: "firing"}

	e.Ingest(a1)
	e.Ingest(a2)
	e.Ingest(a3)

	result := e.Correlate()

	topoGroupFound := false
	for _, g := range result.Groups {
		if g.Correlation == "topology" {
			topoGroupFound = true
			if g.Score <= 0 {
				t.Errorf("topology group should have positive score, got %f", g.Score)
			}
			services := make(map[string]bool)
			for _, svc := range g.Services {
				services[svc] = true
			}
			if services["api"] && services["db"] {
				// api and db are 1-hop neighbors, should be correlated via topology
			}
		}
	}

	if !topoGroupFound {
		t.Errorf("expected at least one topology-correlated group; got groups: %+v", result.Groups)
	}
}

func TestQueryFilters(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})
	e.Ingest(Alert{Source: "datadog", Name: "HighLatency", Severity: "medium", Service: "web", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})
	e.Ingest(Alert{Source: "prometheus", Name: "DiskFull", Severity: "critical", Service: "api", Labels: map[string]string{"env": "staging"}, FiredAt: now, Status: "firing"})

	results := e.Query("api", "", time.Time{})
	if len(results) != 2 {
		t.Errorf("expected 2 results for service=api, got %d", len(results))
	}

	results = e.Query("", "critical", time.Time{})
	if len(results) != 1 {
		t.Errorf("expected 1 result for severity=critical, got %d", len(results))
	}

	results = e.Query("", "", now.Add(-time.Hour))
	if len(results) != 3 {
		t.Errorf("expected 3 results for since=1h ago, got %d", len(results))
	}

	results = e.Query("", "", now.Add(time.Hour))
	if len(results) != 0 {
		t.Errorf("expected 0 results for since=1h future, got %d", len(results))
	}
}

func TestPrune(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	old := Alert{Source: "prometheus", Name: "OldAlert", Severity: "low", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(-48 * time.Hour), Status: "resolved"}
	recent := Alert{Source: "prometheus", Name: "RecentAlert", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(-30 * time.Minute), Status: "firing"}

	e.Ingest(old)
	e.Ingest(recent)

	e.Prune(24 * time.Hour)

	stats := e.Stats()
	if stats.TotalDeduped != 1 {
		t.Errorf("expected 1 deduped alert after pruning, got %d", stats.TotalDeduped)
	}
}

func TestRingBufferOverflow(t *testing.T) {
	e := NewAlertEngine()
	e.maxRawAlerts = 10

	now := time.Now()
	for i := 0; i < 15; i++ {
		a := Alert{
			Source:   "prometheus",
			Name:     fmt.Sprintf("Alert%d", i),
			Severity: "medium",
			Service:  "api",
			Labels:   map[string]string{"index": fmt.Sprintf("%d", i)},
			FiredAt:  now.Add(time.Duration(i) * time.Second),
			Status:   "firing",
		}
		e.Ingest(a)
	}

	stats := e.Stats()
	if stats.TotalRaw != 15 {
		t.Errorf("expected total raw count 15, got %d", stats.TotalRaw)
	}

	// Ring buffer should only hold maxRawAlerts
	e.mu.RLock()
	rawCount := e.rawCount
	e.mu.RUnlock()
	if rawCount != 10 {
		t.Errorf("expected ring buffer to hold 10, got %d", rawCount)
	}
}

func TestCorrelateWithoutTopology(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})
	e.Ingest(Alert{Source: "datadog", Name: "HighLatency", Severity: "medium", Service: "web", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(2 * time.Minute), Status: "firing"})

	result := e.Correlate()

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Stats.TotalDeduped != 2 {
		t.Errorf("expected 2 deduped alerts, got %d", result.Stats.TotalDeduped)
	}
	if result.Stats.TotalGroups == 0 {
		t.Error("expected at least one group")
	}
}

func TestIngestBatch(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	alerts := []Alert{
		{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"},
		{Source: "datadog", Name: "HighLatency", Severity: "medium", Service: "web", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"},
	}

	results := e.IngestBatch(alerts)
	if len(results) != 2 {
		t.Errorf("expected 2 deduped results, got %d", len(results))
	}
}

func TestClear(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})
	e.Correlate()

	e.Clear()

	stats := e.Stats()
	if stats.TotalDeduped != 0 {
		t.Errorf("expected 0 deduped after clear, got %d", stats.TotalDeduped)
	}
	if stats.TotalGroups != 0 {
		t.Errorf("expected 0 groups after clear, got %d", stats.TotalGroups)
	}
}

func TestGetGroup(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})
	result := e.Correlate()

	if len(result.Groups) == 0 {
		t.Fatal("expected at least one group")
	}

	groupID := result.Groups[0].ID
	g := e.GetGroup(groupID)
	if g == nil {
		t.Fatal("expected to find group by ID")
	}
	if g.ID != groupID {
		t.Errorf("expected group ID %s, got %s", groupID, g.ID)
	}

	g = e.GetGroup("nonexistent")
	if g != nil {
		t.Error("expected nil for nonexistent group ID")
	}
}

func TestSeverityOrdering(t *testing.T) {
	if !CompareSeverity("critical", "high") {
		t.Error("critical should be more severe than high")
	}
	if !CompareSeverity("high", "medium") {
		t.Error("high should be more severe than medium")
	}
	if CompareSeverity("low", "high") {
		t.Error("low should not be more severe than high")
	}
}

func TestEscalateSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"info", "low"},
		{"low", "medium"},
		{"medium", "high"},
		{"high", "critical"},
		{"critical", "critical"},
	}
	for _, tt := range tests {
		result := EscalateSeverity(tt.input)
		if result != tt.expected {
			t.Errorf("EscalateSeverity(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestAlertIngestTool(t *testing.T) {
	e := NewAlertEngine()
	SetAlertEngine(e)
	defer SetAlertEngine(nil)

	now := time.Now()
	tool := &AlertIngestTool{}

	result, err := tool.Execute(context.Background(), map[string]any{
		"alerts": []any{
			map[string]any{
				"source":   "prometheus",
				"name":     "HighCPU",
				"severity": "high",
				"service":  "api",
				"labels":   map[string]any{"env": "prod"},
				"fired_at": now.Format(time.RFC3339),
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if resultMap["ingested"].(int) != 1 {
		t.Errorf("expected 1 ingested, got %v", resultMap["ingested"])
	}
}

func TestAlertQueryTool(t *testing.T) {
	e := NewAlertEngine()
	SetAlertEngine(e)
	defer SetAlertEngine(nil)

	now := time.Now()
	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})

	tool := &AlertQueryTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"service": "api",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	alerts, ok := resultMap["alerts"].([]DedupedAlert)
	if !ok {
		t.Fatal("expected alerts slice")
	}
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}
}

func TestAlertCorrelateTool(t *testing.T) {
	e := NewAlertEngine()
	SetAlertEngine(e)
	defer SetAlertEngine(nil)

	now := time.Now()
	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})

	tool := &AlertCorrelateTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"include_topology": false,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	corrResult, ok := result.(*CorrelationResult)
	if !ok {
		t.Fatal("expected CorrelationResult")
	}
	if corrResult.Stats.TotalDeduped != 1 {
		t.Errorf("expected 1 deduped, got %d", corrResult.Stats.TotalDeduped)
	}
}

func TestInitAlertEngine(t *testing.T) {
	tp := &mockTopologyProvider{neighbors: map[string][]string{"api": {"db"}}}
	e := InitAlertEngine(tp)
	defer SetAlertEngine(nil)

	if e == nil {
		t.Fatal("expected non-nil engine")
	}
	if DefaultAlertEngine() != e {
		t.Error("expected global engine to be set")
	}
	if e.topology == nil {
		t.Error("expected topology provider to be set")
	}
}

func TestCorrelateMultipleGroups(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	// Two groups separated by > 5 minutes
	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})
	e.Ingest(Alert{Source: "datadog", Name: "HighLatency", Severity: "medium", Service: "web", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(2 * time.Minute), Status: "firing"})
	e.Ingest(Alert{Source: "cloudwatch", Name: "DiskFull", Severity: "critical", Service: "db", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(20 * time.Minute), Status: "firing"})

	result := e.Correlate()

	if result.Stats.TotalDeduped != 3 {
		t.Errorf("expected 3 deduped, got %d", result.Stats.TotalDeduped)
	}
	if result.Stats.TotalGroups < 2 {
		t.Errorf("expected at least 2 groups (separated by >5min), got %d", result.Stats.TotalGroups)
	}
}

func TestDedupedAlertFirstLastFiredAt(t *testing.T) {
	e := NewAlertEngine()
	now := time.Now()

	fp := FingerprintAlert(&Alert{Source: "prometheus", Name: "HighCPU", Service: "api", Labels: map[string]string{"env": "prod"}})

	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(10 * time.Minute), Status: "firing"})
	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now, Status: "firing"})
	e.Ingest(Alert{Source: "prometheus", Name: "HighCPU", Severity: "high", Service: "api", Labels: map[string]string{"env": "prod"}, FiredAt: now.Add(5 * time.Minute), Status: "firing"})

	da := e.deduped[fp]
	if da == nil {
		t.Fatal("expected deduped alert")
	}
	if !da.FirstFiredAt.Equal(now) {
		t.Errorf("expected FirstFiredAt = %v, got %v", now, da.FirstFiredAt)
	}
	if !da.LastFiredAt.Equal(now.Add(10 * time.Minute)) {
		t.Errorf("expected LastFiredAt = %v, got %v", now.Add(10*time.Minute), da.LastFiredAt)
	}
}
