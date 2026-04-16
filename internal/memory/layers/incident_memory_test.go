package layers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewIncidentMemory(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)
	if im == nil {
		t.Fatal("NewIncidentMemory returned nil")
	}
	if im.store == nil {
		t.Error("store should not be nil")
	}
}

func TestNewIncidentMemory_NilStore(t *testing.T) {
	im := NewIncidentMemory(nil)
	if im == nil {
		t.Fatal("should return instance even with nil store")
	}
}

func TestIncidentMemory_CreateAndGetIncident(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	incident := &Incident{
		ID:               "INC-001",
		Title:            "Database connection timeout",
		Severity:         "high",
		Status:           "active",
		Service:          "api-gateway",
		Description:      "Connection pool exhausted",
		AlertSource:      "prometheus",
		AffectedServices: []string{"api-gateway", "auth-service"},
		StartedAt:        time.Now().UTC().Truncate(time.Second),
	}

	err := im.CreateIncident(context.Background(), incident)
	if err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	got, err := im.GetIncident("INC-001")
	if err != nil {
		t.Fatalf("GetIncident failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetIncident returned nil")
	}
	if got.Title != "Database connection timeout" {
		t.Errorf("expected Title=Database connection timeout, got %s", got.Title)
	}
	if got.Severity != "high" {
		t.Errorf("expected Severity=high, got %s", got.Severity)
	}
	if got.Status != "active" {
		t.Errorf("expected Status=active, got %s", got.Status)
	}
	if got.Service != "api-gateway" {
		t.Errorf("expected Service=api-gateway, got %s", got.Service)
	}
	if len(got.AffectedServices) != 2 {
		t.Errorf("expected 2 affected services, got %d", len(got.AffectedServices))
	}
}

func TestIncidentMemory_GetIncident_NotFound(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	got, err := im.GetIncident("NONEXISTENT")
	if err != nil {
		t.Fatalf("GetIncident should not error for missing: %v", err)
	}
	if got != nil {
		t.Error("should return nil for missing incident")
	}
}

func TestIncidentMemory_CreateIncident_DefaultStartedAt(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	incident := &Incident{
		ID:       "INC-002",
		Title:    "Test",
		Severity: "low",
		Status:   "active",
	}

	err := im.CreateIncident(context.Background(), incident)
	if err != nil {
		t.Fatalf("CreateIncident failed: %v", err)
	}

	got, _ := im.GetIncident("INC-002")
	if got.StartedAt.IsZero() {
		t.Error("StartedAt should default to current time when zero")
	}
}

func TestIncidentMemory_UpdateIncident(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	incident := &Incident{
		ID:        "INC-010",
		Title:     "Original",
		Severity:  "medium",
		Status:    "active",
		Service:   "web",
		StartedAt: time.Now().UTC(),
	}
	im.CreateIncident(context.Background(), incident)

	err := im.UpdateIncident(context.Background(), "INC-010", map[string]any{
		"severity": "critical",
		"status":   "investigating",
	})
	if err != nil {
		t.Fatalf("UpdateIncident failed: %v", err)
	}

	got, _ := im.GetIncident("INC-010")
	if got.Severity != "critical" {
		t.Errorf("expected Severity=critical, got %s", got.Severity)
	}
	if got.Status != "investigating" {
		t.Errorf("expected Status=investigating, got %s", got.Status)
	}
	if got.Title != "Original" {
		t.Errorf("Title should not change, got %s", got.Title)
	}
}

func TestIncidentMemory_UpdateIncident_IgnoresDisallowedFields(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	incident := &Incident{
		ID:        "INC-011",
		Title:     "Original",
		Severity:  "low",
		Status:    "active",
		Service:   "web",
		StartedAt: time.Now().UTC(),
	}
	im.CreateIncident(context.Background(), incident)

	err := im.UpdateIncident(context.Background(), "INC-011", map[string]any{
		"id": "should-not-change",
	})
	if err != nil {
		t.Fatalf("UpdateIncident failed: %v", err)
	}

	got, _ := im.GetIncident("INC-011")
	if got.ID != "INC-011" {
		t.Errorf("ID should not change, got %s", got.ID)
	}
}

func TestIncidentMemory_UpdateIncident_EmptyUpdates(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	incident := &Incident{
		ID:        "INC-012",
		Title:     "Test",
		Severity:  "low",
		Status:    "active",
		Service:   "web",
		StartedAt: time.Now().UTC(),
	}
	im.CreateIncident(context.Background(), incident)

	err := im.UpdateIncident(context.Background(), "INC-012", map[string]any{})
	if err != nil {
		t.Fatalf("UpdateIncident with empty updates should not fail: %v", err)
	}
}

func TestIncidentMemory_ResolveIncident(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	incident := &Incident{
		ID:        "INC-020",
		Title:     "Resolvable incident",
		Severity:  "high",
		Status:    "active",
		Service:   "api",
		StartedAt: time.Now().UTC(),
	}
	im.CreateIncident(context.Background(), incident)

	err := im.ResolveIncident(context.Background(), "INC-020", "config error", "fixed config")
	if err != nil {
		t.Fatalf("ResolveIncident failed: %v", err)
	}

	got, _ := im.GetIncident("INC-020")
	if got.Status != "resolved" {
		t.Errorf("expected Status=resolved, got %s", got.Status)
	}
	if got.RootCause != "config error" {
		t.Errorf("expected RootCause=config error, got %s", got.RootCause)
	}
	if got.Remediation != "fixed config" {
		t.Errorf("expected Remediation=fixed config, got %s", got.Remediation)
	}
	if got.ResolvedAt == nil {
		t.Error("ResolvedAt should be set")
	}
}

func TestIncidentMemory_ListActiveIncidents(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	now := time.Now().UTC()

	im.CreateIncident(context.Background(), &Incident{ID: "INC-A1", Title: "Active 1", Severity: "critical", Status: "active", Service: "svc1", StartedAt: now})
	im.CreateIncident(context.Background(), &Incident{ID: "INC-A2", Title: "Active 2", Severity: "high", Status: "investigating", Service: "svc2", StartedAt: now})
	im.CreateIncident(context.Background(), &Incident{ID: "INC-A3", Title: "Resolved", Severity: "low", Status: "active", Service: "svc3", StartedAt: now})
	im.ResolveIncident(context.Background(), "INC-A3", "done", "fixed")

	active, err := im.ListActiveIncidents()
	if err != nil {
		t.Fatalf("ListActiveIncidents failed: %v", err)
	}

	if len(active) != 2 {
		t.Errorf("expected 2 active incidents, got %d", len(active))
	}

	for _, inc := range active {
		if inc.Status == "resolved" {
			t.Errorf("resolved incident should not appear in active list: %s", inc.ID)
		}
	}
}

func TestIncidentMemory_ListIncidentsByService(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	now := time.Now().UTC()

	im.CreateIncident(context.Background(), &Incident{ID: "INC-S1", Title: "Service A incident", Severity: "high", Status: "active", Service: "service-a", StartedAt: now})
	im.CreateIncident(context.Background(), &Incident{ID: "INC-S2", Title: "Service B incident", Severity: "low", Status: "active", Service: "service-b", StartedAt: now})
	im.CreateIncident(context.Background(), &Incident{ID: "INC-S3", Title: "Another Service A", Severity: "medium", Status: "active", Service: "service-a", StartedAt: now})

	incidents, err := im.ListIncidentsByService("service-a")
	if err != nil {
		t.Fatalf("ListIncidentsByService failed: %v", err)
	}

	if len(incidents) != 2 {
		t.Errorf("expected 2 incidents for service-a, got %d", len(incidents))
	}

	for _, inc := range incidents {
		if inc.Service != "service-a" {
			t.Errorf("expected Service=service-a, got %s", inc.Service)
		}
	}
}

func TestIncidentMemory_AddTimelineEvent(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	im.CreateIncident(context.Background(), &Incident{
		ID:        "INC-TL1",
		Title:     "Timeline test",
		Severity:  "medium",
		Status:    "active",
		Service:   "web",
		StartedAt: time.Now().UTC(),
	})

	event := TimelineEvent{
		Timestamp: time.Now().UTC().Truncate(time.Second),
		Type:      "detection",
		Content:   "Alert triggered by monitoring",
		Source:    "prometheus",
	}

	err := im.AddTimelineEvent(context.Background(), "INC-TL1", event)
	if err != nil {
		t.Fatalf("AddTimelineEvent failed: %v", err)
	}

	got, _ := im.GetIncident("INC-TL1")
	if len(got.TimelineEvents) != 1 {
		t.Fatalf("expected 1 timeline event, got %d", len(got.TimelineEvents))
	}

	ev := got.TimelineEvents[0]
	if ev.Type != "detection" {
		t.Errorf("expected Type=detection, got %s", ev.Type)
	}
	if ev.Content != "Alert triggered by monitoring" {
		t.Errorf("unexpected Content: %s", ev.Content)
	}
	if ev.Source != "prometheus" {
		t.Errorf("expected Source=prometheus, got %s", ev.Source)
	}
}

func TestIncidentMemory_AddTimelineEvent_DefaultTimestamp(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	im.CreateIncident(context.Background(), &Incident{
		ID:        "INC-TL2",
		Title:     "Default timestamp test",
		Severity:  "low",
		Status:    "active",
		Service:   "web",
		StartedAt: time.Now().UTC(),
	})

	event := TimelineEvent{
		Type:    "note",
		Content: "Investigation started",
		Source:  "engineer",
	}

	err := im.AddTimelineEvent(context.Background(), "INC-TL2", event)
	if err != nil {
		t.Fatalf("AddTimelineEvent failed: %v", err)
	}

	got, _ := im.GetIncident("INC-TL2")
	if len(got.TimelineEvents) != 1 {
		t.Fatal("expected 1 timeline event")
	}
	if got.TimelineEvents[0].Timestamp.IsZero() {
		t.Error("timestamp should default to current time when zero")
	}
}

func TestIncidentMemory_CreateAndGetPostmortem(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	im.CreateIncident(context.Background(), &Incident{
		ID:        "INC-PM1",
		Title:     "Postmortem test",
		Severity:  "high",
		Status:    "active",
		Service:   "api",
		StartedAt: time.Now().UTC(),
	})

	pm := &Postmortem{
		ID:             "PM-001",
		IncidentID:     "INC-PM1",
		Title:          "API Outage Postmortem",
		Summary:        "The API was down for 2 hours",
		RootCause:      "Memory leak in connection pool",
		Contributing:   []string{"No memory limits", "Missing health checks"},
		ActionItems:    []string{"Add memory limits", "Add health checks"},
		LessonsLearned: []string{"Monitor memory usage"},
		CreatedAt:      time.Now().UTC().Truncate(time.Second),
	}

	err := im.CreatePostmortem(context.Background(), pm)
	if err != nil {
		t.Fatalf("CreatePostmortem failed: %v", err)
	}

	got, err := im.GetPostmortem("INC-PM1")
	if err != nil {
		t.Fatalf("GetPostmortem failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetPostmortem returned nil")
	}
	if got.Title != "API Outage Postmortem" {
		t.Errorf("expected Title=API Outage Postmortem, got %s", got.Title)
	}
	if got.RootCause != "Memory leak in connection pool" {
		t.Errorf("unexpected RootCause: %s", got.RootCause)
	}
	if len(got.Contributing) != 2 {
		t.Errorf("expected 2 contributing factors, got %d", len(got.Contributing))
	}
	if len(got.ActionItems) != 2 {
		t.Errorf("expected 2 action items, got %d", len(got.ActionItems))
	}
	if len(got.LessonsLearned) != 1 {
		t.Errorf("expected 1 lesson, got %d", len(got.LessonsLearned))
	}
}

func TestIncidentMemory_GetPostmortem_NotFound(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	got, err := im.GetPostmortem("NONEXISTENT")
	if err != nil {
		t.Fatalf("GetPostmortem should not error for missing: %v", err)
	}
	if got != nil {
		t.Error("should return nil for missing postmortem")
	}
}

func TestIncidentMemory_SetAndGetSLOStatus(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	slo := &SLOStatus{
		Service:              "api-gateway",
		SLOName:              "availability",
		Target:               0.999,
		Current:              0.995,
		ErrorBudgetRemaining: 0.5,
		BurnRate:             2.0,
		Status:               "at_risk",
	}

	err := im.SetSLOStatus(slo)
	if err != nil {
		t.Fatalf("SetSLOStatus failed: %v", err)
	}

	statuses, err := im.GetSLOStatuses()
	if err != nil {
		t.Fatalf("GetSLOStatuses failed: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("expected 1 SLO status, got %d", len(statuses))
	}

	got := statuses[0]
	if got.Service != "api-gateway" {
		t.Errorf("expected Service=api-gateway, got %s", got.Service)
	}
	if got.SLOName != "availability" {
		t.Errorf("expected SLOName=availability, got %s", got.SLOName)
	}
	if got.Status != "at_risk" {
		t.Errorf("expected Status=at_risk, got %s", got.Status)
	}
}

func TestIncidentMemory_SetSLOStatus_Upsert(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	slo1 := &SLOStatus{
		Service: "api",
		SLOName: "latency",
		Target:  0.99,
		Current: 0.95,
		Status:  "at_risk",
	}
	im.SetSLOStatus(slo1)

	slo2 := &SLOStatus{
		Service: "api",
		SLOName: "latency",
		Target:  0.99,
		Current: 0.98,
		Status:  "healthy",
	}
	im.SetSLOStatus(slo2)

	statuses, _ := im.GetSLOStatuses()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 SLO status after upsert, got %d", len(statuses))
	}
	if statuses[0].Current != 0.98 {
		t.Errorf("expected Current=0.98 after upsert, got %.2f", statuses[0].Current)
	}
	if statuses[0].Status != "healthy" {
		t.Errorf("expected Status=healthy after upsert, got %s", statuses[0].Status)
	}
}

func TestIncidentMemory_BuildIncidentPrompt(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	prompt := im.BuildIncidentPrompt()
	if prompt != "" {
		t.Errorf("expected empty prompt with no data, got %q", prompt)
	}

	im.CreateIncident(context.Background(), &Incident{
		ID:               "INC-P1",
		Title:            "Test incident",
		Severity:         "critical",
		Status:           "active",
		Service:          "api",
		StartedAt:        time.Now().UTC(),
		AffectedServices: []string{"api", "web"},
	})

	prompt = im.BuildIncidentPrompt()
	if prompt == "" {
		t.Error("prompt should not be empty with active incident")
	}
	if !contains(prompt, "CRITICAL") {
		t.Error("prompt should contain severity CRITICAL")
	}
	if !contains(prompt, "INC-P1") {
		t.Error("prompt should contain incident ID")
	}
}

func TestIncidentMemory_BuildIncidentPrompt_SLO(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	im.SetSLOStatus(&SLOStatus{
		Service: "api",
		SLOName: "availability",
		Target:  0.999,
		Current: 0.95,
		Status:  "critical",
	})

	prompt := im.BuildIncidentPrompt()
	if prompt == "" {
		t.Error("prompt should not be empty with SLO status")
	}
	if !contains(prompt, "api") {
		t.Error("prompt should contain service name")
	}
}

func TestIncidentMemory_BuildIncidentPrompt_CharLimit(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	now := time.Now().UTC()
	for i := 0; i < 100; i++ {
		im.CreateIncident(context.Background(), &Incident{
			ID:        fmt.Sprintf("INC-LIMIT-%d", i),
			Title:     "Very long incident title that repeats to fill space and test the character limit of the prompt builder",
			Severity:  "medium",
			Status:    "active",
			Service:   "svc",
			StartedAt: now,
		})
	}

	prompt := im.BuildIncidentPrompt()
	if len(prompt) > 2000 {
		t.Errorf("prompt should be capped at ~2000 chars, got %d", len(prompt))
	}
}

func TestIncidentMemory_UpdateIncidentFromToolResult(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	result := map[string]any{
		"data": []any{
			map[string]any{
				"id":          "ALERT-001",
				"title":       "High CPU usage",
				"severity":    "high",
				"status":      "firing",
				"service":     "compute",
				"description": "CPU at 95%",
				"source":      "prometheus",
			},
		},
	}

	err := im.UpdateIncidentFromToolResult("sopa_list_faults", result)
	if err != nil {
		t.Fatalf("UpdateIncidentFromToolResult failed: %v", err)
	}

	got, _ := im.GetIncident("ALERT-001")
	if got == nil {
		t.Fatal("incident should be created from tool result")
	}
	if got.Title != "High CPU usage" {
		t.Errorf("expected Title=High CPU usage, got %s", got.Title)
	}
	if got.Status != "active" {
		t.Errorf("firing status should map to active, got %s", got.Status)
	}
}

func TestIncidentMemory_UpdateIncidentFromToolResult_UnknownTool(t *testing.T) {
	s := newTestStore(t)
	im := NewIncidentMemory(s)

	err := im.UpdateIncidentFromToolResult("unknown_tool", map[string]any{})
	if err != nil {
		t.Fatalf("unknown tool should not return error: %v", err)
	}
}

func TestIncidentMemory_NilStore(t *testing.T) {
	im := NewIncidentMemory(nil)

	if err := im.CreateIncident(context.Background(), &Incident{ID: "test"}); err != nil {
		t.Errorf("CreateIncident with nil store should not error: %v", err)
	}

	if err := im.UpdateIncident(context.Background(), "test", map[string]any{"status": "resolved"}); err != nil {
		t.Errorf("UpdateIncident with nil store should not error: %v", err)
	}

	got, err := im.GetIncident("test")
	if err != nil {
		t.Errorf("GetIncident with nil store should not error: %v", err)
	}
	if got != nil {
		t.Error("GetIncident with nil store should return nil")
	}

	if prompt := im.BuildIncidentPrompt(); prompt != "" {
		t.Errorf("BuildIncidentPrompt with nil store should return empty, got %q", prompt)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
