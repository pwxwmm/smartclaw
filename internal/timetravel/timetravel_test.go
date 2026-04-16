package timetravel

import (
	"context"
	"strings"
	"testing"
	"time"
)

type mockRecordingStore struct {
	entries []RecordingEntry
	paths   []string
	loadErr error
	listErr error
}

func (m *mockRecordingStore) LoadRecording(path string) ([]RecordingEntry, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	return m.entries, nil
}

func (m *mockRecordingStore) ListRecordings() ([]string, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.paths, nil
}

type mockIncidentStore struct {
	incident *IncidentInfo
	timeline []TimelineEvent
	incErr   error
	tlErr    error
}

func (m *mockIncidentStore) GetIncidentTimeline(incidentID string) ([]TimelineEvent, error) {
	if m.tlErr != nil {
		return nil, m.tlErr
	}
	return m.timeline, nil
}

func (m *mockIncidentStore) GetIncident(incidentID string) (*IncidentInfo, error) {
	if m.incErr != nil {
		return nil, m.incErr
	}
	return m.incident, nil
}

func baseTime() time.Time {
	return time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
}

func TestReplayIncident(t *testing.T) {
	bt := baseTime()
	incident := &IncidentInfo{
		ID:        "INC-001",
		Title:     "Database connection pool exhausted",
		Severity:  "high",
		Status:    "resolved",
		Service:   "payment-service",
		StartedAt: bt,
	}
	resolvedAt := bt.Add(2 * time.Hour)
	incident.ResolvedAt = &resolvedAt

	timeline := []TimelineEvent{
		{Timestamp: bt, Type: "alert", Content: "High error rate detected on payment-service", Source: "prometheus"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "hypothesis", Content: "DB connection pool may be exhausted", Source: "agent"},
		{Timestamp: bt.Add(10 * time.Minute), Type: "evidence", Content: "sopa_node_logs shows connection refused errors", Source: "agent"},
		{Timestamp: bt.Add(30 * time.Minute), Type: "mitigation", Content: "Restarted database connection pool", Source: "agent"},
		{Timestamp: bt.Add(60 * time.Minute), Type: "resolution", Content: "Root cause: max_connections set too low after config change", Source: "agent"},
	}

	store := &mockIncidentStore{incident: incident, timeline: timeline}
	engine := NewTimeTravelEngine()
	engine.SetIncidentStore(store)

	session, err := engine.ReplayIncident(context.Background(), "INC-001")
	if err != nil {
		t.Fatalf("ReplayIncident failed: %v", err)
	}
	if session.IncidentID != "INC-001" {
		t.Errorf("expected incident_id INC-001, got %s", session.IncidentID)
	}
	if session.SourceType != ReplayIncidentMemory {
		t.Errorf("expected source_type incident_memory, got %s", session.SourceType)
	}
	if session.Status != ReplayComplete {
		t.Errorf("expected status complete, got %s", session.Status)
	}
	if len(session.Events) != 5 {
		t.Errorf("expected 5 events, got %d", len(session.Events))
	}
	if session.Summary == nil {
		t.Fatal("expected summary to be populated")
	}
	if session.Summary.TotalEvents != 5 {
		t.Errorf("expected 5 total events, got %d", session.Summary.TotalEvents)
	}
}

func TestReplayIncidentNotFound(t *testing.T) {
	store := &mockIncidentStore{incident: nil}
	engine := NewTimeTravelEngine()
	engine.SetIncidentStore(store)

	_, err := engine.ReplayIncident(context.Background(), "INC-999")
	if err == nil {
		t.Fatal("expected error for missing incident")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestReplayIncidentNoStore(t *testing.T) {
	engine := NewTimeTravelEngine()
	_, err := engine.ReplayIncident(context.Background(), "INC-001")
	if err == nil {
		t.Fatal("expected error when incident store not configured")
	}
}

func TestReplayRecording(t *testing.T) {
	bt := baseTime()
	entries := []RecordingEntry{
		{Timestamp: bt, Type: "message", Data: map[string]any{"role": "user", "content": "Investigate high latency on api-gateway"}},
		{Timestamp: bt.Add(1 * time.Minute), Type: "tool_call", Data: map[string]any{"tool": "sopa_node_logs", "input": map[string]any{"id": "api-gateway-01"}}},
		{Timestamp: bt.Add(2 * time.Minute), Type: "tool_result", Data: map[string]any{"tool": "sopa_node_logs", "result": "connection pool exhausted"}},
		{Timestamp: bt.Add(5 * time.Minute), Type: "tool_call", Data: map[string]any{"tool": "sopa_execute_task", "input": map[string]any{"scriptId": "restart-service"}}},
		{Timestamp: bt.Add(8 * time.Minute), Type: "tool_result", Data: map[string]any{"tool": "sopa_execute_task", "result": "service restarted successfully"}},
	}

	store := &mockRecordingStore{entries: entries, paths: []string{"/tmp/session_1.jsonl"}}
	engine := NewTimeTravelEngine()
	engine.SetRecordingStore(store)

	session, err := engine.ReplayRecording(context.Background(), "/tmp/session_1.jsonl")
	if err != nil {
		t.Fatalf("ReplayRecording failed: %v", err)
	}
	if session.SourceType != ReplayRecording {
		t.Errorf("expected source_type recording, got %s", session.SourceType)
	}
	if session.SourceID != "/tmp/session_1.jsonl" {
		t.Errorf("expected source_id /tmp/session_1.jsonl, got %s", session.SourceID)
	}
	if len(session.Events) != 5 {
		t.Errorf("expected 5 events, got %d", len(session.Events))
	}
	if session.Summary == nil {
		t.Fatal("expected summary to be populated")
	}
}

func TestReplayRecordingNoStore(t *testing.T) {
	engine := NewTimeTravelEngine()
	_, err := engine.ReplayRecording(context.Background(), "/tmp/test.jsonl")
	if err == nil {
		t.Fatal("expected error when recording store not configured")
	}
}

func TestExtractEventsFromRecording(t *testing.T) {
	bt := baseTime()
	entries := []RecordingEntry{
		{Timestamp: bt, Type: "tool_call", Data: map[string]any{"tool": "bash", "input": map[string]any{"command": "kubectl get pods"}}},
		{Timestamp: bt.Add(1 * time.Minute), Type: "tool_result", Data: map[string]any{"tool": "bash", "result": "3 pods running"}},
		{Timestamp: bt.Add(2 * time.Minute), Type: "message", Data: map[string]any{"role": "assistant", "content": "All pods are healthy"}},
	}

	engine := NewTimeTravelEngine()
	events := engine.ExtractEventsFromRecording(entries)

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	if events[0].Type != "tool_call" {
		t.Errorf("event 0: expected type tool_call, got %s", events[0].Type)
	}
	if events[0].Actor != "agent" {
		t.Errorf("event 0: expected actor agent, got %s", events[0].Actor)
	}
	if !strings.Contains(events[0].Action, "bash") {
		t.Errorf("event 0: expected action to mention bash, got %s", events[0].Action)
	}

	if events[1].Type != "tool_result" {
		t.Errorf("event 1: expected type tool_result, got %s", events[1].Type)
	}
	if events[1].Actor != "system" {
		t.Errorf("event 1: expected actor system, got %s", events[1].Actor)
	}

	if events[2].Type != "message" {
		t.Errorf("event 2: expected type message, got %s", events[2].Type)
	}
	if events[2].Actor != "assistant" {
		t.Errorf("event 2: expected actor assistant, got %s", events[2].Actor)
	}
}

func TestExtractEventsFromTimeline(t *testing.T) {
	bt := baseTime()
	incident := &IncidentInfo{
		ID:        "INC-002",
		Title:     "API timeout",
		Severity:  "critical",
		Service:   "api-gateway",
		StartedAt: bt,
	}

	timeline := []TimelineEvent{
		{Timestamp: bt, Type: "alert", Content: "API latency spike detected", Source: "grafana"},
		{Timestamp: bt.Add(2 * time.Minute), Type: "mitigation", Content: "Scaled up replicas", Source: "agent"},
	}

	engine := NewTimeTravelEngine()
	events := engine.ExtractEventsFromTimeline(timeline, incident)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != "alert" {
		t.Errorf("event 0: expected type alert, got %s", events[0].Type)
	}
	if events[0].Service != "api-gateway" {
		t.Errorf("event 0: expected service api-gateway, got %s", events[0].Service)
	}
	if events[0].Severity != "critical" {
		t.Errorf("event 0: expected severity critical, got %s", events[0].Severity)
	}
	if events[0].Actor != "grafana" {
		t.Errorf("event 0: expected actor grafana, got %s", events[0].Actor)
	}
}

func TestAnalyzeTimelinePhaseDetection(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "High error rate detected"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "tool_call", Actor: "agent", Action: "search logs for errors"},
		{Timestamp: bt.Add(10 * time.Minute), Type: "tool_call", Actor: "agent", Action: "grep for connection refused"},
		{Timestamp: bt.Add(15 * time.Minute), Type: "tool_call", Actor: "agent", Action: "read config file"},
		{Timestamp: bt.Add(30 * time.Minute), Type: "tool_call", Actor: "agent", Action: "kubectl restart deployment"},
		{Timestamp: bt.Add(60 * time.Minute), Type: "resolution", Actor: "agent", Action: "Incident resolved"},
	}

	engine := NewTimeTravelEngine()
	summary := engine.AnalyzeTimeline(events)

	if len(summary.Timeline) == 0 {
		t.Fatal("expected phases to be detected")
	}

	phaseNames := make([]string, len(summary.Timeline))
	for i, p := range summary.Timeline {
		phaseNames[i] = p.Name
	}

	hasDetection := false
	hasResolution := false
	for _, name := range phaseNames {
		if name == "detection" {
			hasDetection = true
		}
		if name == "resolution" {
			hasResolution = true
		}
	}
	if !hasDetection {
		t.Errorf("expected detection phase, got phases: %v", phaseNames)
	}
	if !hasResolution {
		t.Errorf("expected resolution phase, got phases: %v", phaseNames)
	}
}

func TestAnalyzeTimelineKeyMoments(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "Error rate spike"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "tool_result", Actor: "system", Action: "logs returned", Result: "root cause: connection pool exhausted", Severity: "medium"},
		{Timestamp: bt.Add(10 * time.Minute), Type: "tool_result", Actor: "system", Action: "logs returned", Result: "found deadlock in database", Severity: "critical"},
		{Timestamp: bt.Add(30 * time.Minute), Type: "mitigation", Actor: "agent", Action: "Restarted service"},
		{Timestamp: bt.Add(60 * time.Minute), Type: "resolution", Actor: "agent", Action: "Incident resolved"},
	}

	engine := NewTimeTravelEngine()
	summary := engine.AnalyzeTimeline(events)

	if len(summary.KeyMoments) == 0 {
		t.Fatal("expected key moments to be identified")
	}

	hasRootCause := false
	for _, km := range summary.KeyMoments {
		if km.Type == "root_cause" {
			hasRootCause = true
			if km.Importance < 0.9 {
				t.Errorf("root cause key moment should have high importance, got %.2f", km.Importance)
			}
		}
	}
	if !hasRootCause {
		t.Error("expected a root_cause key moment from 'root cause' mention")
	}
}

func TestAnalyzeTimelineSeverityEscalation(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "Error rate spike", Severity: "medium"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "alert", Actor: "prometheus", Action: "Error rate worsening", Severity: "high"},
		{Timestamp: bt.Add(10 * time.Minute), Type: "alert", Actor: "prometheus", Action: "Service down", Severity: "critical"},
	}

	engine := NewTimeTravelEngine()
	summary := engine.AnalyzeTimeline(events)

	hasEscalation := false
	for _, km := range summary.KeyMoments {
		if km.Type == "escalation" && strings.Contains(km.Description, "escalated") {
			hasEscalation = true
		}
	}
	if !hasEscalation {
		t.Error("expected severity escalation key moments")
	}
}

func TestAnalyzeTimelineRootCauseHint(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "High error rate"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "tool_result", Actor: "system", Action: "Investigation", Result: "root cause: max_connections too low"},
		{Timestamp: bt.Add(30 * time.Minute), Type: "mitigation", Actor: "agent", Action: "Restarted service"},
		{Timestamp: bt.Add(60 * time.Minute), Type: "resolution", Actor: "agent", Action: "Resolved"},
	}

	engine := NewTimeTravelEngine()
	summary := engine.AnalyzeTimeline(events)

	if summary.RootCauseHint == "" {
		t.Error("expected root cause hint to be identified")
	}
}

func TestAnalyzeTimelineEmpty(t *testing.T) {
	engine := NewTimeTravelEngine()
	summary := engine.AnalyzeTimeline([]ReplayEvent{})

	if summary.TotalEvents != 0 {
		t.Errorf("expected 0 total events, got %d", summary.TotalEvents)
	}
	if len(summary.Timeline) != 0 {
		t.Errorf("expected 0 phases, got %d", len(summary.Timeline))
	}
	if len(summary.KeyMoments) != 0 {
		t.Errorf("expected 0 key moments, got %d", len(summary.KeyMoments))
	}
}

func TestAnalyzeTimelineSingleEvent(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "Error detected"},
	}

	engine := NewTimeTravelEngine()
	summary := engine.AnalyzeTimeline(events)

	if summary.TotalEvents != 1 {
		t.Errorf("expected 1 total event, got %d", summary.TotalEvents)
	}
	if summary.Duration != 0 {
		t.Errorf("expected 0 duration for single event, got %v", summary.Duration)
	}
	if len(summary.Timeline) == 0 {
		t.Error("expected at least one phase for single event")
	}
}

func TestWhatIfScenario(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "Error rate spike"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "tool_call", Actor: "agent", Action: "search logs"},
		{Timestamp: bt.Add(10 * time.Minute), Type: "tool_call", Actor: "agent", Action: "grep for errors"},
		{Timestamp: bt.Add(30 * time.Minute), Type: "mitigation", Actor: "agent", Action: "Restarted service"},
		{Timestamp: bt.Add(60 * time.Minute), Type: "resolution", Actor: "agent", Action: "Resolved"},
	}

	engine := NewTimeTravelEngine()
	session := &ReplaySession{
		ID:     "test-session",
		Events: events,
	}
	engine.mu.Lock()
	engine.sessions["test-session"] = session
	engine.mu.Unlock()

	scenario, err := engine.WhatIf("test-session", 0, "earlier detection through alert tuning")
	if err != nil {
		t.Fatalf("WhatIf failed: %v", err)
	}
	if scenario.ChangePoint.IsZero() {
		t.Error("expected change_point to be set")
	}
	if scenario.Confidence <= 0 || scenario.Confidence > 1 {
		t.Errorf("expected confidence between 0 and 1, got %.2f", scenario.Confidence)
	}
	if scenario.ProjectedOutcome == "" {
		t.Error("expected projected outcome to be set")
	}
}

func TestWhatIfAutoRemediation(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "Error rate spike"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "tool_call", Actor: "agent", Action: "search logs"},
		{Timestamp: bt.Add(30 * time.Minute), Type: "mitigation", Actor: "agent", Action: "Restarted service"},
	}

	engine := NewTimeTravelEngine()
	session := &ReplaySession{ID: "auto-session", Events: events}
	engine.mu.Lock()
	engine.sessions["auto-session"] = session
	engine.mu.Unlock()

	scenario, err := engine.WhatIf("auto-session", 0, "automated remediation via runbook")
	if err != nil {
		t.Fatalf("WhatIf failed: %v", err)
	}
	if scenario.Confidence < 0.7 {
		t.Errorf("auto-remediation should have high confidence, got %.2f", scenario.Confidence)
	}
}

func TestWhatIfDifferentTool(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "Error rate spike"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "tool_call", Actor: "agent", Action: "search logs"},
		{Timestamp: bt.Add(30 * time.Minute), Type: "mitigation", Actor: "agent", Action: "Restarted service"},
	}

	engine := NewTimeTravelEngine()
	session := &ReplaySession{ID: "tool-session", Events: events}
	engine.mu.Lock()
	engine.sessions["tool-session"] = session
	engine.mu.Unlock()

	scenario, err := engine.WhatIf("tool-session", 1, "different tool: sopa_get_node instead of search")
	if err != nil {
		t.Fatalf("WhatIf failed: %v", err)
	}
	if scenario.Confidence > 0.6 {
		t.Errorf("different tool scenario should have lower confidence, got %.2f", scenario.Confidence)
	}
}

func TestWhatIfInvalidIndex(t *testing.T) {
	engine := NewTimeTravelEngine()
	session := &ReplaySession{ID: "idx-session", Events: []ReplayEvent{}}
	engine.mu.Lock()
	engine.sessions["idx-session"] = session
	engine.mu.Unlock()

	_, err := engine.WhatIf("idx-session", 0, "some change")
	if err == nil {
		t.Fatal("expected error for out-of-range event index")
	}
}

func TestWhatIfSessionNotFound(t *testing.T) {
	engine := NewTimeTravelEngine()
	_, err := engine.WhatIf("nonexistent", 0, "some change")
	if err == nil {
		t.Fatal("expected error for missing session")
	}
}

func TestGetSession(t *testing.T) {
	engine := NewTimeTravelEngine()
	session := &ReplaySession{
		ID:        "get-test",
		Status:    ReplayComplete,
		Events:    []ReplayEvent{},
		CreatedAt: time.Now().UTC(),
	}
	engine.mu.Lock()
	engine.sessions["get-test"] = session
	engine.mu.Unlock()

	got := engine.GetSession("get-test")
	if got == nil {
		t.Fatal("expected session to be found")
	}
	if got.ID != "get-test" {
		t.Errorf("expected ID get-test, got %s", got.ID)
	}

	missing := engine.GetSession("nonexistent")
	if missing != nil {
		t.Error("expected nil for missing session")
	}
}

func TestReplayIncidentTool(t *testing.T) {
	bt := baseTime()
	incident := &IncidentInfo{
		ID:        "INC-TOOL",
		Title:     "Test incident",
		Severity:  "medium",
		Service:   "test-service",
		StartedAt: bt,
	}
	timeline := []TimelineEvent{
		{Timestamp: bt, Type: "alert", Content: "Test alert", Source: "monitor"},
	}

	incStore := &mockIncidentStore{incident: incident, timeline: timeline}
	_ = InitTimeTravelEngine(nil, incStore)
	defer func() { SetTimeTravelEngine(nil) }()

	tool := &ReplayIncidentTool{}

	result, err := tool.Execute(context.Background(), map[string]any{
		"source":    "incident",
		"source_id": "INC-TOOL",
		"analyze":   true,
	})
	if err != nil {
		t.Fatalf("tool execute failed: %v", err)
	}

	session, ok := result.(*ReplaySession)
	if !ok {
		t.Fatal("expected *ReplaySession result")
	}
	if session.IncidentID != "INC-TOOL" {
		t.Errorf("expected incident_id INC-TOOL, got %s", session.IncidentID)
	}
	if session.Summary == nil {
		t.Error("expected summary when analyze=true")
	}
}

func TestReplayIncidentToolNoAnalyze(t *testing.T) {
	bt := baseTime()
	incident := &IncidentInfo{
		ID:        "INC-NOANALYZE",
		Title:     "Test incident",
		Severity:  "low",
		Service:   "test-service",
		StartedAt: bt,
	}
	timeline := []TimelineEvent{
		{Timestamp: bt, Type: "alert", Content: "Test alert", Source: "monitor"},
	}

	incStore := &mockIncidentStore{incident: incident, timeline: timeline}
	_ = InitTimeTravelEngine(nil, incStore)
	defer func() { SetTimeTravelEngine(nil) }()

	tool := &ReplayIncidentTool{}

	result, err := tool.Execute(context.Background(), map[string]any{
		"source":    "incident",
		"source_id": "INC-NOANALYZE",
		"analyze":   false,
	})
	if err != nil {
		t.Fatalf("tool execute failed: %v", err)
	}

	session, ok := result.(*ReplaySession)
	if !ok {
		t.Fatal("expected *ReplaySession result")
	}
	if session.Summary != nil {
		t.Error("expected nil summary when analyze=false")
	}
}

func TestReplayIncidentToolRecording(t *testing.T) {
	bt := baseTime()
	entries := []RecordingEntry{
		{Timestamp: bt, Type: "message", Data: map[string]any{"role": "user", "content": "test"}},
	}
	recStore := &mockRecordingStore{entries: entries, paths: []string{"/tmp/test.jsonl"}}
	_ = InitTimeTravelEngine(recStore, nil)
	defer func() { SetTimeTravelEngine(nil) }()

	tool := &ReplayIncidentTool{}

	result, err := tool.Execute(context.Background(), map[string]any{
		"source":    "recording",
		"source_id": "/tmp/test.jsonl",
	})
	if err != nil {
		t.Fatalf("tool execute failed: %v", err)
	}

	session, ok := result.(*ReplaySession)
	if !ok {
		t.Fatal("expected *ReplaySession result")
	}
	if session.SourceType != ReplayRecording {
		t.Errorf("expected source_type recording, got %s", session.SourceType)
	}
}

func TestReplayIncidentToolValidation(t *testing.T) {
	tool := &ReplayIncidentTool{}

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing source")
	}

	_, err = tool.Execute(context.Background(), map[string]any{"source": "incident"})
	if err == nil {
		t.Fatal("expected error for missing source_id")
	}

	_, err = tool.Execute(context.Background(), map[string]any{"source": "invalid", "source_id": "x"})
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestReplayIncidentToolNoEngine(t *testing.T) {
	SetTimeTravelEngine(nil)
	tool := &ReplayIncidentTool{}

	_, err := tool.Execute(context.Background(), map[string]any{"source": "incident", "source_id": "x"})
	if err == nil {
		t.Fatal("expected error when engine not initialized")
	}
}

func TestInitTimeTravelEngine(t *testing.T) {
	recStore := &mockRecordingStore{}
	incStore := &mockIncidentStore{}

	engine := InitTimeTravelEngine(recStore, incStore)
	if engine == nil {
		t.Fatal("expected engine to be created")
	}

	got := DefaultTimeTravelEngine()
	if got != engine {
		t.Error("expected default engine to be the initialized one")
	}

	SetTimeTravelEngine(nil)
}

func TestInitTimeTravelEngineNilStores(t *testing.T) {
	engine := InitTimeTravelEngine(nil, nil)
	if engine == nil {
		t.Fatal("expected engine to be created even with nil stores")
	}
	SetTimeTravelEngine(nil)
}

func TestPhaseDetectionFullLifecycle(t *testing.T) {
	bt := baseTime()
	events := []ReplayEvent{
		{Timestamp: bt, Type: "alert", Actor: "prometheus", Action: "Error rate spike detected"},
		{Timestamp: bt.Add(1 * time.Minute), Type: "tool_call", Actor: "agent", Action: "search logs for payment-service"},
		{Timestamp: bt.Add(3 * time.Minute), Type: "tool_call", Actor: "agent", Action: "grep for connection errors"},
		{Timestamp: bt.Add(5 * time.Minute), Type: "tool_call", Actor: "agent", Action: "read database config"},
		{Timestamp: bt.Add(10 * time.Minute), Type: "tool_call", Actor: "agent", Action: "kubectl rollout restart deployment"},
		{Timestamp: bt.Add(20 * time.Minute), Type: "resolution", Actor: "agent", Action: "Service fully recovered"},
	}

	engine := NewTimeTravelEngine()
	summary := engine.AnalyzeTimeline(events)

	phaseMap := make(map[string]TimelinePhase)
	for _, p := range summary.Timeline {
		phaseMap[p.Name] = p
	}

	if _, ok := phaseMap["detection"]; !ok {
		t.Error("expected detection phase")
	}
	if _, ok := phaseMap["triage"]; !ok {
		t.Error("expected triage phase")
	}
	if _, ok := phaseMap["investigation"]; !ok {
		t.Error("expected investigation phase")
	}
	if _, ok := phaseMap["mitigation"]; !ok {
		t.Error("expected mitigation phase")
	}
	if _, ok := phaseMap["resolution"]; !ok {
		t.Error("expected resolution phase")
	}

	if summary.Duration == 0 {
		t.Error("expected non-zero duration")
	}
}
