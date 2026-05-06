package warroom

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockAgentRunner struct {
	mu      sync.Mutex
	results map[DomainAgentType]string
	calls   []callRecord
}

type callRecord struct {
	AgentType DomainAgentType
	Task      string
}

func newMockAgentRunner() *mockAgentRunner {
	return &mockAgentRunner{
		results: make(map[DomainAgentType]string),
	}
}

func (m *mockAgentRunner) RunAgent(ctx context.Context, agentType DomainAgentType, task string, tools []string, opts ...RunAgentOptions) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, callRecord{AgentType: agentType, Task: task})
	if result, ok := m.results[agentType]; ok {
		return result, nil
	}
	return fmt.Sprintf("Mock result from %s agent", agentType), nil
}

func (m *mockAgentRunner) setResult(agentType DomainAgentType, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[agentType] = result
}

func (m *mockAgentRunner) getCalls() []callRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]callRecord, len(m.calls))
	copy(out, m.calls)
	return out
}

func TestStartWarRoom(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, err := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "DB Latency Spike",
		Description: "Investigating elevated database latency",
	})
	if err != nil {
		t.Fatalf("StartWarRoom failed: %v", err)
	}
	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.Title != "DB Latency Spike" {
		t.Errorf("expected title 'DB Latency Spike', got %q", session.Title)
	}
	if session.Status != WarRoomActive {
		t.Errorf("expected status active, got %q", session.Status)
	}
	if len(session.Agents) != 8 {
		t.Errorf("expected 8 agents by default, got %d", len(session.Agents))
	}
	if len(session.Timeline) == 0 {
		t.Error("expected timeline entries")
	}
	if session.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestStartWarRoomWithSubset(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, err := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Network Outage",
		Description: "Investigating connectivity issues",
		AgentTypes:  []DomainAgentType{AgentNetwork, AgentInfra},
	})
	if err != nil {
		t.Fatalf("StartWarRoom failed: %v", err)
	}
	if len(session.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(session.Agents))
	}

	found := map[DomainAgentType]bool{}
	for _, a := range session.Agents {
		found[a.AgentType] = true
	}
	if !found[AgentNetwork] || !found[AgentInfra] {
		t.Error("expected network and infra agents")
	}
	if found[AgentDatabase] || found[AgentApp] || found[AgentSecurity] {
		t.Error("did not expect database, app, or security agents")
	}
}

func TestStartWarRoomValidation(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	_, err := c.StartWarRoom(ctx, WarRoomRequest{Description: "no title"})
	if err == nil {
		t.Error("expected error for missing title")
	}

	_, err = c.StartWarRoom(ctx, WarRoomRequest{Title: "no desc"})
	if err == nil {
		t.Error("expected error for missing description")
	}

	_, err = c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "test",
		Description: "test",
		AgentTypes:  []DomainAgentType{"invalid"},
	})
	if err == nil {
		t.Error("expected error for invalid agent type")
	}
}

func TestStartWarRoomWithIncidentID(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, err := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Incident Test",
		Description: "test",
		IncidentID:  "INC-123",
	})
	if err != nil {
		t.Fatalf("StartWarRoom failed: %v", err)
	}
	if session.IncidentID != "INC-123" {
		t.Errorf("expected incident_id 'INC-123', got %q", session.IncidentID)
	}
}

func TestStartWarRoomWithContext(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, err := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Context Test",
		Description: "test",
		Context:     map[string]any{"region": "us-east-1", "service": "api"},
	})
	if err != nil {
		t.Fatalf("StartWarRoom failed: %v", err)
	}
	if session.Context["region"] != "us-east-1" {
		t.Error("expected context to contain region")
	}
}

func TestAssignTask(t *testing.T) {
	c := NewWarRoomCoordinator()
	runner := newMockAgentRunner()
	c.SetAgentRunner(runner)
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Task Test",
		Description: "test",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	err := c.AssignTask(ctx, session.ID, AgentNetwork, "Check DNS resolution")
	if err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	s := c.GetSession(session.ID)
	hasTaskAssigned := false
	for _, e := range s.Timeline {
		if e.Event == "task_assigned" && e.AgentType == AgentNetwork {
			hasTaskAssigned = true
		}
	}
	if !hasTaskAssigned {
		t.Error("expected task_assigned timeline entry")
	}
}

func TestAssignTaskInvalidSession(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	err := c.AssignTask(ctx, "nonexistent", AgentNetwork, "task")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestAssignTaskInvalidAgent(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "test",
		Description: "test",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	err := c.AssignTask(ctx, session.ID, AgentDatabase, "task")
	if err == nil {
		t.Error("expected error for unassigned agent type")
	}
}

func TestBroadcast(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Broadcast Test",
		Description: "test",
		AgentTypes:  []DomainAgentType{AgentNetwork, AgentDatabase},
	})

	err := c.Broadcast(ctx, session.ID, "New finding: DNS resolution failing")
	if err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	s := c.GetSession(session.ID)
	hasBroadcast := false
	for _, e := range s.Timeline {
		if e.Event == "broadcast" {
			hasBroadcast = true
		}
	}
	if !hasBroadcast {
		t.Error("expected broadcast timeline entry")
	}
}

func TestBroadcastInvalidSession(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	err := c.Broadcast(ctx, "nonexistent", "msg")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestSubmitFinding(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Finding Test",
		Description: "test",
		AgentTypes:  []DomainAgentType{AgentNetwork, AgentDatabase},
	})

	finding := Finding{
		ID:          "f1",
		AgentType:   AgentNetwork,
		Category:    "root_cause",
		Title:       "DNS Misconfiguration",
		Description: "DNS pointing to wrong IP",
		Confidence:  0.9,
		Evidence:    []string{"nslookup returned 10.0.0.1 instead of 10.0.0.2"},
		CreatedAt:   time.Now(),
	}

	err := c.SubmitFinding(session.ID, AgentNetwork, finding)
	if err != nil {
		t.Fatalf("SubmitFinding failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	s := c.GetSession(session.ID)
	if len(s.Findings) == 0 {
		t.Error("expected at least one finding in session")
	}
}

func TestSubmitFindingInvalidSession(t *testing.T) {
	c := NewWarRoomCoordinator()

	err := c.SubmitFinding("nonexistent", AgentNetwork, Finding{ID: "f1"})
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestGetSession(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "GetSession Test",
		Description: "test",
	})

	found := c.GetSession(session.ID)
	if found == nil {
		t.Error("expected to find session")
	}
	if found.ID != session.ID {
		t.Error("session ID mismatch")
	}

	missing := c.GetSession("nonexistent")
	if missing != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestListSessions(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	s1, _ := c.StartWarRoom(ctx, WarRoomRequest{Title: "Session 1", Description: "test"})
	s2, _ := c.StartWarRoom(ctx, WarRoomRequest{Title: "Session 2", Description: "test"})

	sessions := c.ListSessions()
	if len(sessions) < 2 {
		t.Errorf("expected at least 2 sessions, got %d", len(sessions))
	}

	ids := map[string]bool{}
	for _, s := range sessions {
		ids[s.ID] = true
	}
	if !ids[s1.ID] || !ids[s2.ID] {
		t.Error("expected both session IDs in list")
	}
}

func TestCloseSession(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Close Test",
		Description: "test",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	result, err := c.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	if result.SessionID != session.ID {
		t.Error("result session ID mismatch")
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
	if len(result.Recommendations) == 0 {
		t.Error("expected recommendations")
	}

	s := c.GetSession(session.ID)
	if s.Status != WarRoomClosed {
		t.Errorf("expected status closed, got %q", s.Status)
	}
	if s.ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}
}

func TestCloseSessionWithRootCause(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Root Cause Test",
		Description: "test",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	c.mu.Lock()
	session.Findings = append(session.Findings,
		Finding{
			ID:         "f1",
			AgentType:  AgentNetwork,
			Category:   "root_cause",
			Title:      "DNS Failure",
			Confidence: 0.85,
			CreatedAt:  time.Now(),
		},
		Finding{
			ID:         "f2",
			AgentType:  AgentNetwork,
			Category:   "root_cause",
			Title:      "Network Partition",
			Confidence: 0.95,
			CreatedAt:  time.Now(),
		},
		Finding{
			ID:         "f3",
			AgentType:  AgentNetwork,
			Category:   "symptom",
			Title:      "High Latency",
			Confidence: 0.7,
			CreatedAt:  time.Now(),
		},
	)
	c.mu.Unlock()

	result, err := c.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	if result.RootCause == nil {
		t.Fatal("expected root cause to be identified")
	}
	if result.RootCause.Title != "Network Partition" {
		t.Errorf("expected highest confidence root cause 'Network Partition', got %q", result.RootCause.Title)
	}
	if result.RootCause.Confidence != 0.95 {
		t.Errorf("expected confidence 0.95, got %f", result.RootCause.Confidence)
	}
}

func TestCloseSessionInvalid(t *testing.T) {
	c := NewWarRoomCoordinator()

	_, err := c.CloseSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestDomainAgentDefinitions(t *testing.T) {
	expectedTypes := []DomainAgentType{AgentNetwork, AgentDatabase, AgentInfra, AgentApp, AgentSecurity}
	for _, at := range expectedTypes {
		agent, ok := BuiltInAgents[at]
		if !ok {
			t.Errorf("expected built-in agent for type %q", at)
			continue
		}
		if agent.Type != at {
			t.Errorf("agent type mismatch: expected %q, got %q", at, agent.Type)
		}
		if agent.Name == "" {
			t.Errorf("agent %q missing name", at)
		}
		if agent.Description == "" {
			t.Errorf("agent %q missing description", at)
		}
		if len(agent.InvestigationSteps) == 0 {
			t.Errorf("agent %q missing investigation steps", at)
		}
		if len(agent.Tools) == 0 {
			t.Errorf("agent %q missing tools", at)
		}
		if len(agent.FocusAreas) == 0 {
			t.Errorf("agent %q missing focus areas", at)
		}
	}
}

func TestAllAgentTypes(t *testing.T) {
	types := AllAgentTypes()
	if len(types) != 8 {
		t.Errorf("expected 8 agent types, got %d", len(types))
	}
}

func TestGetAgent(t *testing.T) {
	agent, ok := GetAgent(AgentNetwork)
	if !ok {
		t.Error("expected to find network agent")
	}
	if agent.Name != "Network Investigator" {
		t.Errorf("expected 'Network Investigator', got %q", agent.Name)
	}

	_, ok = GetAgent("nonexistent")
	if ok {
		t.Error("did not expect to find nonexistent agent")
	}
}

func TestChannelMessagePassing(t *testing.T) {
	c := NewWarRoomCoordinator()
	runner := newMockAgentRunner()
	runner.setResult(AgentNetwork, "DNS resolution failing for api.example.com")
	c.SetAgentRunner(runner)
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Channel Test",
		Description: "test channel message passing",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	err := c.AssignTask(ctx, session.ID, AgentNetwork, "Investigate DNS")
	if err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	s := c.GetSession(session.ID)

	findingsFromRunner := false
	for _, f := range s.Findings {
		if strings.Contains(f.Description, "DNS") || strings.Contains(f.Title, "Network") {
			findingsFromRunner = true
		}
	}
	if !findingsFromRunner {
		t.Error("expected findings from agent runner to be recorded")
	}

	calls := runner.getCalls()
	if len(calls) == 0 {
		t.Error("expected runner to be called")
	}
}

func TestContextCancellation(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx, cancel := context.WithCancel(context.Background())

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Cancel Test",
		Description: "test context cancellation",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	cancel()
	time.Sleep(100 * time.Millisecond)

	s := c.GetSession(session.ID)

	for _, a := range s.Agents {
		if a.AgentType == AgentNetwork && a.Status == AgentStatusRunning {
			t.Error("expected agent to stop after context cancellation")
		}
	}
}

func TestAgentRunnerIntegration(t *testing.T) {
	c := NewWarRoomCoordinator()
	runner := newMockAgentRunner()
	runner.setResult(AgentDatabase, "Replication lag detected: 45s behind primary")
	runner.setResult(AgentInfra, "Node memory pressure detected: 92% utilization")
	c.SetAgentRunner(runner)
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Integration Test",
		Description: "Full agent runner integration",
		AgentTypes:  []DomainAgentType{AgentDatabase, AgentInfra},
	})

	c.AssignTask(ctx, session.ID, AgentDatabase, "Check replication status")
	c.AssignTask(ctx, session.ID, AgentInfra, "Check node health")

	time.Sleep(300 * time.Millisecond)

	calls := runner.getCalls()
	if len(calls) < 2 {
		t.Errorf("expected at least 2 runner calls, got %d", len(calls))
	}

	s := c.GetSession(session.ID)

	dbFinding := false
	infraFinding := false
	for _, f := range s.Findings {
		if f.AgentType == AgentDatabase {
			dbFinding = true
		}
		if f.AgentType == AgentInfra {
			infraFinding = true
		}
	}
	if !dbFinding {
		t.Error("expected finding from database agent")
	}
	if !infraFinding {
		t.Error("expected finding from infrastructure agent")
	}
}

func TestFindingConfidenceRanking(t *testing.T) {
	findings := []Finding{
		{ID: "1", Category: "root_cause", Confidence: 0.5, Title: "Low confidence"},
		{ID: "2", Category: "symptom", Confidence: 0.99, Title: "High confidence symptom"},
		{ID: "3", Category: "root_cause", Confidence: 0.85, Title: "Medium confidence"},
		{ID: "4", Category: "root_cause", Confidence: 0.95, Title: "High confidence root"},
	}

	var rootCause *Finding
	for i := range findings {
		f := &findings[i]
		if f.Category == "root_cause" {
			if rootCause == nil || f.Confidence > rootCause.Confidence {
				rootCause = f
			}
		}
	}

	if rootCause == nil {
		t.Fatal("expected to find a root cause")
	}
	if rootCause.ID != "4" {
		t.Errorf("expected highest confidence root cause (ID=4, conf=0.95), got ID=%s conf=%f", rootCause.ID, rootCause.Confidence)
	}
}

func TestFindingCrossPollination(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Cross-Pollination Test",
		Description: "test that findings from one agent are broadcast to others",
		AgentTypes:  []DomainAgentType{AgentNetwork, AgentDatabase, AgentInfra},
	})

	finding := Finding{
		ID:          "f-cross",
		AgentType:   AgentNetwork,
		Category:    "root_cause",
		Title:       "DNS Failure",
		Description: "DNS resolution failing",
		Confidence:  0.9,
		CreatedAt:   time.Now(),
	}

	err := c.SubmitFinding(session.ID, AgentNetwork, finding)
	if err != nil {
		t.Fatalf("SubmitFinding failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	s := c.GetSession(session.ID)
	if len(s.Findings) == 0 {
		t.Error("expected at least one finding in session")
	}

	hasNetworkFinding := false
	for _, f := range s.Findings {
		if f.AgentType == AgentNetwork && f.ID == "f-cross" {
			hasNetworkFinding = true
		}
	}
	if !hasNetworkFinding {
		t.Error("expected network agent finding to be recorded in session")
	}

	hasTimelineEntry := false
	for _, e := range s.Timeline {
		if e.Event == "finding_submitted" && e.AgentType == AgentNetwork {
			hasTimelineEntry = true
		}
	}
	if !hasTimelineEntry {
		t.Error("expected finding_submitted timeline entry")
	}
}

func TestGenerateRecommendations(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		expect   []string
	}{
		{
			name:     "empty findings",
			findings: []Finding{},
			expect:   []string{"Continue monitoring"},
		},
		{
			name:     "root_cause category",
			findings: []Finding{{Category: "root_cause"}},
			expect:   []string{"root cause"},
		},
		{
			name:     "symptom category",
			findings: []Finding{{Category: "symptom"}},
			expect:   []string{"Monitor related symptoms"},
		},
		{
			name:     "dependency category",
			findings: []Finding{{Category: "dependency"}},
			expect:   []string{"circuit breakers"},
		},
		{
			name:     "config category",
			findings: []Finding{{Category: "config"}},
			expect:   []string{"Audit configuration"},
		},
		{
			name:     "metric category",
			findings: []Finding{{Category: "metric"}},
			expect:   []string{"alerting"},
		},
		{
			name:     "log category",
			findings: []Finding{{Category: "log"}},
			expect:   []string{"log coverage"},
		},
		{
			name:     "multiple categories",
			findings: []Finding{{Category: "root_cause"}, {Category: "symptom"}, {Category: "dependency"}},
			expect:   []string{"root cause", "symptoms", "circuit breakers"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs := generateRecommendations(tt.findings)
			for _, exp := range tt.expect {
				found := false
				for _, rec := range recs {
					if strings.Contains(rec, exp) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected recommendation containing %q, got %v", exp, recs)
				}
			}
		})
	}
}

func TestWarRoomStartTool(t *testing.T) {
	c := NewWarRoomCoordinator()
	SetWarRoomCoordinator(c)
	defer func() { SetWarRoomCoordinator(nil) }()

	tool := &WarRoomStartTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"title":       "Tool Test",
		"description": "Testing warroom_start tool",
	})
	if err != nil {
		t.Fatalf("warroom_start failed: %v", err)
	}

	session, ok := result.(*WarRoomSession)
	if !ok {
		t.Fatal("expected *WarRoomSession result")
	}
	if session.Title != "Tool Test" {
		t.Errorf("expected title 'Tool Test', got %q", session.Title)
	}

	_, err = tool.Execute(ctx, map[string]any{})
	if err == nil {
		t.Error("expected error for missing title")
	}
}

func TestWarRoomStartToolWithAgentTypes(t *testing.T) {
	c := NewWarRoomCoordinator()
	SetWarRoomCoordinator(c)
	defer func() { SetWarRoomCoordinator(nil) }()

	tool := &WarRoomStartTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"title":       "Subset Test",
		"description": "Testing with subset of agents",
		"agent_types": []any{"network", "infra"},
	})
	if err != nil {
		t.Fatalf("warroom_start failed: %v", err)
	}

	session := result.(*WarRoomSession)
	if len(session.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(session.Agents))
	}
}

func TestWarRoomStatusTool(t *testing.T) {
	c := NewWarRoomCoordinator()
	SetWarRoomCoordinator(c)
	defer func() { SetWarRoomCoordinator(nil) }()

	startTool := &WarRoomStartTool{}
	statusTool := &WarRoomStatusTool{}
	ctx := context.Background()

	result, _ := startTool.Execute(ctx, map[string]any{
		"title":       "Status Test",
		"description": "test",
	})
	session := result.(*WarRoomSession)

	statusResult, err := statusTool.Execute(ctx, map[string]any{
		"session_id": session.ID,
	})
	if err != nil {
		t.Fatalf("warroom_status failed: %v", err)
	}

	statusSession, ok := statusResult.(*WarRoomSession)
	if !ok {
		t.Fatal("expected *WarRoomSession result")
	}
	if statusSession.ID != session.ID {
		t.Error("session ID mismatch")
	}

	_, err = statusTool.Execute(ctx, map[string]any{
		"session_id": "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestWarRoomStopTool(t *testing.T) {
	c := NewWarRoomCoordinator()
	SetWarRoomCoordinator(c)
	defer func() { SetWarRoomCoordinator(nil) }()

	startTool := &WarRoomStartTool{}
	stopTool := &WarRoomStopTool{}
	ctx := context.Background()

	result, _ := startTool.Execute(ctx, map[string]any{
		"title":       "Stop Test",
		"description": "test",
	})
	session := result.(*WarRoomSession)

	stopResult, err := stopTool.Execute(ctx, map[string]any{
		"session_id": session.ID,
	})
	if err != nil {
		t.Fatalf("warroom_stop failed: %v", err)
	}

	invResult, ok := stopResult.(*InvestigationResult)
	if !ok {
		t.Fatal("expected *InvestigationResult result")
	}
	if invResult.SessionID != session.ID {
		t.Error("session ID mismatch")
	}

	_, err = stopTool.Execute(ctx, map[string]any{
		"session_id": "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestInitWarRoom(t *testing.T) {
	runner := newMockAgentRunner()
	c := InitWarRoom(runner)
	defer func() { SetWarRoomCoordinator(nil) }()

	if c == nil {
		t.Fatal("expected coordinator to be created")
	}

	got := DefaultWarRoomCoordinator()
	if got != c {
		t.Error("expected global coordinator to be set")
	}

	if c.getRunner() == nil {
		t.Error("expected agent runner to be set")
	}
}

func TestInitWarRoomNilRunner(t *testing.T) {
	c := InitWarRoom(nil)
	defer func() { SetWarRoomCoordinator(nil) }()

	if c == nil {
		t.Fatal("expected coordinator to be created")
	}
	if c.getRunner() != nil {
		t.Error("expected nil runner")
	}
}

func TestToolNames(t *testing.T) {
	if (&WarRoomStartTool{}).Name() != "warroom_start" {
		t.Error("unexpected warroom_start tool name")
	}
	if (&WarRoomStatusTool{}).Name() != "warroom_status" {
		t.Error("unexpected warroom_status tool name")
	}
	if (&WarRoomStopTool{}).Name() != "warroom_stop" {
		t.Error("unexpected warroom_stop tool name")
	}
}

func TestToolInputSchemas(t *testing.T) {
	schema := (&WarRoomStartTool{}).InputSchema()
	if schema["type"] != "object" {
		t.Error("expected object type in schema")
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties in schema")
	}
	if _, ok := props["title"]; !ok {
		t.Error("expected title property")
	}
	if _, ok := props["description"]; !ok {
		t.Error("expected description property")
	}
}

func TestCloseSessionAgentReports(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Report Test",
		Description: "test agent reports",
		AgentTypes:  []DomainAgentType{AgentNetwork, AgentDatabase},
	})

	c.mu.Lock()
	for i := range session.Agents {
		if session.Agents[i].AgentType == AgentNetwork {
			session.Agents[i].Findings = append(session.Agents[i].Findings,
				Finding{ID: "f1", Title: "DNS issue", Category: "symptom", Confidence: 0.8, CreatedAt: time.Now()},
			)
		}
	}
	session.Findings = append(session.Findings,
		Finding{ID: "f1", AgentType: AgentNetwork, Title: "DNS issue", Category: "symptom", Confidence: 0.8, CreatedAt: time.Now()},
	)
	c.mu.Unlock()

	result, err := c.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	if result.AgentReports[AgentNetwork] == "" {
		t.Error("expected network agent report")
	}
	if result.AgentReports[AgentDatabase] != "No findings reported" {
		t.Errorf("expected 'No findings reported' for database agent, got %q", result.AgentReports[AgentDatabase])
	}
}

func TestCloseSessionSummary(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Summary Test",
		Description: "test summary",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	c.mu.Lock()
	session.Findings = append(session.Findings,
		Finding{
			ID:         "f1",
			AgentType:  AgentNetwork,
			Category:   "root_cause",
			Title:      "DNS Failure",
			Confidence: 0.9,
			CreatedAt:  time.Now(),
		},
	)
	c.mu.Unlock()

	result, _ := c.CloseSession(session.ID)

	if !strings.Contains(result.Summary, "Summary Test") {
		t.Error("expected summary to contain title")
	}
	if !strings.Contains(result.Summary, "Root cause") {
		t.Error("expected summary to mention Root cause")
	}
	if !strings.Contains(result.Summary, "90.0%") {
		t.Error("expected summary to contain confidence percentage")
	}
}

func TestBlackboardWriteAndRead(t *testing.T) {
	bb := NewBlackboard("test-session")

	bb.WriteEntry(BlackboardEntry{
		Key:      "dns_status",
		Value:    "DNS resolution failing for api.example.com",
		Author:   AgentNetwork,
		Category: "observation",
	})
	bb.WriteEntry(BlackboardEntry{
		Key:      "cpu_usage",
		Value:    "92% utilization",
		Author:   AgentInfra,
		Category: "metric",
	})

	all := bb.ReadEntries("")
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}

	obs := bb.ReadEntries("observation")
	if len(obs) != 1 {
		t.Errorf("expected 1 observation entry, got %d", len(obs))
	}
	if obs[0].Key != "dns_status" {
		t.Errorf("expected key 'dns_status', got %q", obs[0].Key)
	}
}

func TestBlackboardHypotheses(t *testing.T) {
	bb := NewBlackboard("test-session")

	bb.AddHypothesis(Hypothesis{
		ID:          "h1",
		Description: "DNS misconfiguration causing service failures",
		ProposedBy:  AgentNetwork,
		Confidence:  0.7,
		Status:      "proposed",
	})

	hypotheses := bb.GetHypotheses()
	if len(hypotheses) != 1 {
		t.Fatalf("expected 1 hypothesis, got %d", len(hypotheses))
	}
	if hypotheses[0].Description != "DNS misconfiguration causing service failures" {
		t.Errorf("unexpected hypothesis description: %q", hypotheses[0].Description)
	}

	updated := bb.UpdateHypothesis("h1", func(h *Hypothesis) {
		h.Status = "confirmed"
		h.Confidence = 0.9
	})
	if !updated {
		t.Error("expected hypothesis to be updated")
	}

	hypotheses = bb.GetHypotheses()
	if hypotheses[0].Status != "confirmed" {
		t.Errorf("expected status 'confirmed', got %q", hypotheses[0].Status)
	}
}

func TestBlackboardSharedFacts(t *testing.T) {
	bb := NewBlackboard("test-session")

	bb.AddSharedFact(SharedFact{
		Content:     "Service A is down",
		Source:      AgentApp,
		ConfirmedBy: []DomainAgentType{AgentApp},
		Confidence:  0.8,
	})

	facts := bb.GetSharedFacts()
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}

	confirmed := bb.ConfirmFact("Service A is down", AgentInfra)
	if !confirmed {
		t.Error("expected fact to be confirmed")
	}

	facts = bb.GetSharedFacts()
	if len(facts[0].ConfirmedBy) != 2 {
		t.Errorf("expected 2 confirmers, got %d", len(facts[0].ConfirmedBy))
	}
}

func TestBlackboardSnapshot(t *testing.T) {
	bb := NewBlackboard("test-session")

	snapshot := bb.GetSnapshot()
	if !strings.Contains(snapshot, "No shared context yet") {
		t.Error("expected empty blackboard message")
	}

	bb.WriteEntry(BlackboardEntry{
		Key: "test_key", Value: "test_value", Author: AgentNetwork, Category: "observation",
	})
	snapshot = bb.GetSnapshot()
	if !strings.Contains(snapshot, "test_key") || !strings.Contains(snapshot, "test_value") {
		t.Error("expected snapshot to contain entry data")
	}
	if !strings.Contains(snapshot, "Shared Blackboard Context") {
		t.Error("expected snapshot header")
	}
}

func TestBlackboardCreatedOnStartWarRoom(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, err := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Blackboard Test",
		Description: "test blackboard creation",
	})
	if err != nil {
		t.Fatalf("StartWarRoom failed: %v", err)
	}

	bb, ok := c.GetBlackboard(session.ID)
	if !ok || bb == nil {
		t.Error("expected blackboard to be created for session")
	}
}

func TestBlackboardCleanedUpOnClose(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Cleanup Test",
		Description: "test blackboard cleanup",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	sessionID := session.ID
	c.CloseSession(sessionID)

	_, ok := c.GetBlackboard(sessionID)
	if ok {
		t.Error("expected blackboard to be cleaned up after close")
	}
}

func TestSubmitFindingWritesToBlackboard(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "BB Write Test",
		Description: "test finding writes to blackboard",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	finding := Finding{
		ID:          "f-bb",
		AgentType:   AgentNetwork,
		Category:    "symptom",
		Title:       "DNS Failure",
		Description: "DNS resolution failing",
		Confidence:  0.8,
		Evidence:    []string{"nslookup failed"},
		CreatedAt:   time.Now(),
	}

	c.SubmitFinding(session.ID, AgentNetwork, finding)

	bb, _ := c.GetBlackboard(session.ID)
	entries := bb.ReadEntries("")
	if len(entries) == 0 {
		t.Error("expected blackboard to have entries after finding submission")
	}
}

func TestCrossValidationAgreement(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "CrossVal Test",
		Description: "test cross-validation",
		AgentTypes:  []DomainAgentType{AgentNetwork, AgentInfra},
	})

	finding1 := Finding{
		ID:          "f1",
		AgentType:   AgentNetwork,
		Category:    "symptom",
		Title:       "DNS timeout error",
		Description: "DNS resolution timeout causing service failures",
		Confidence:  0.6,
		Evidence:    []string{"timeout on DNS lookup"},
		CreatedAt:   time.Now(),
	}
	c.SubmitFinding(session.ID, AgentNetwork, finding1)

	finding2 := Finding{
		ID:          "f2",
		AgentType:   AgentInfra,
		Category:    "symptom",
		Title:       "Service timeout",
		Description: "Service experiencing timeout and latency issues",
		Confidence:  0.6,
		Evidence:    []string{"timeout on requests"},
		CreatedAt:   time.Now(),
	}
	c.SubmitFinding(session.ID, AgentInfra, finding2)

	s := c.GetSession(session.ID)
	if len(s.Findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(s.Findings))
	}

	infraFinding := s.Findings[1]
	if len(infraFinding.CrossReferences) == 0 {
		t.Error("expected cross-references on second finding")
	}

	if infraFinding.Confidence <= 0.6 {
		t.Errorf("expected confidence boost from cross-validation, got %.2f", infraFinding.Confidence)
	}
}

func TestCrossValidationDisagreement(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Disagree Test",
		Description: "test cross-validation disagreement",
		AgentTypes:  []DomainAgentType{AgentNetwork, AgentApp},
	})

	finding1 := Finding{
		ID:          "f1",
		AgentType:   AgentNetwork,
		Category:    "root_cause",
		Title:       "Network partition",
		Description: "Network partition detected between data centers",
		Confidence:  0.8,
		Evidence:    []string{"packet loss detected"},
		CreatedAt:   time.Now(),
	}
	c.SubmitFinding(session.ID, AgentNetwork, finding1)

	finding2 := Finding{
		ID:          "f2",
		AgentType:   AgentApp,
		Category:    "metric",
		Title:       "App metrics normal",
		Description: "Application metrics show no anomaly - throughput stable",
		Confidence:  0.7,
		Evidence:    []string{"no error in logs"},
		CreatedAt:   time.Now(),
	}
	c.SubmitFinding(session.ID, AgentApp, finding2)

	s := c.GetSession(session.ID)
	if len(s.Findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(s.Findings))
	}
}

func TestEvolveConfidence(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Evolve Test",
		Description: "test confidence evolution",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	finding := Finding{
		ID:          "f-evolve",
		AgentType:   AgentNetwork,
		Category:    "root_cause",
		Title:       "DNS Failure",
		Description: "DNS resolution failing",
		Confidence:  0.5,
		Evidence:    []string{"nslookup failed"},
		CreatedAt:   time.Now(),
	}
	c.SubmitFinding(session.ID, AgentNetwork, finding)

	c.EvolveConfidence(session.ID, "f-evolve", 0.2, "reasoning agent agrees")

	s := c.GetSession(session.ID)
	for _, f := range s.Findings {
		if f.ID == "f-evolve" {
			if f.Confidence != 0.7 {
				t.Errorf("expected confidence 0.7, got %.2f", f.Confidence)
			}
			break
		}
	}

	hasEvolved := false
	for _, e := range s.Timeline {
		if e.Event == "confidence_evolved" {
			hasEvolved = true
		}
	}
	if !hasEvolved {
		t.Error("expected confidence_evolved timeline entry")
	}
}

func TestEvolveConfidenceBounds(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Bounds Test",
		Description: "test confidence bounds",
		AgentTypes:  []DomainAgentType{AgentNetwork},
	})

	finding := Finding{
		ID:          "f-bounds",
		AgentType:   AgentNetwork,
		Category:    "symptom",
		Title:       "Test",
		Description: "test",
		Confidence:  0.9,
		Evidence:    []string{"test"},
		CreatedAt:   time.Now(),
	}
	c.SubmitFinding(session.ID, AgentNetwork, finding)

	c.EvolveConfidence(session.ID, "f-bounds", 0.2, "large boost")

	s := c.GetSession(session.ID)
	for _, f := range s.Findings {
		if f.ID == "f-bounds" {
			if f.Confidence > 0.95 {
				t.Errorf("expected confidence capped at 0.95, got %.2f", f.Confidence)
			}
			break
		}
	}
}

func TestHandoffManager(t *testing.T) {
	hm := NewHandoffManager()
	sessionID := "test-session"

	hm.CreateSession(sessionID)

	req := HandoffRequest{
		FromAgent: AgentNetwork,
		ToAgent:   AgentDatabase,
		Question:  "Can you check replication status?",
		Context:   "We see DNS failures",
		Priority:  "high",
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		gotReq, ok := hm.TryRecvRequest(sessionID)
		if !ok {
			return
		}
		hm.SendResponse(sessionID, HandoffResponse{
			RequestID:  gotReq.ID,
			FromAgent:  gotReq.ToAgent,
			ToAgent:    gotReq.FromAgent,
			Answer:     "Replication is 45s behind primary",
			Confidence: 0.8,
		})
	}()

	ctx := context.Background()
	resp, err := hm.RequestHandoff(ctx, sessionID, req)
	if err != nil {
		t.Fatalf("RequestHandoff failed: %v", err)
	}
	if resp.Answer != "Replication is 45s behind primary" {
		t.Errorf("unexpected answer: %q", resp.Answer)
	}

	hm.CloseSession(sessionID)
}

func TestHandoffManagerTimeout(t *testing.T) {
	hm := NewHandoffManager()
	sessionID := "timeout-session"

	hm.CreateSession(sessionID)
	defer hm.CloseSession(sessionID)

	req := HandoffRequest{
		FromAgent: AgentNetwork,
		ToAgent:   AgentDatabase,
		Question:  "Check replication",
		Priority:  "low",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := hm.RequestHandoff(ctx, sessionID, req)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestHandoffCreatedOnStartWarRoom(t *testing.T) {
	c := NewWarRoomCoordinator()
	ctx := context.Background()

	session, _ := c.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Handoff Session Test",
		Description: "test handoff session setup",
	})

	req, ok := c.TryRecvHandoff(session.ID)
	_ = req
	_ = ok
}

func TestSelectAgentsPhase3Pool(t *testing.T) {
	plan := SelectAgents("GPU training failure with NCCL timeout")
	found := false
	for _, a := range plan.Phase1Agents {
		if a == AgentTraining {
			found = true
		}
	}
	if !found {
		t.Error("expected training agent in Phase 1 for GPU training description")
	}
}

func TestParseRecommendedAgents(t *testing.T) {
	text := "Based on the analysis, I recommend:\n" +
		"```json\n" +
		"{\"recommend_additional_agents\": [\"training\", \"inference\"], \"reason\": \"GPU-related symptoms suggest deeper investigation needed\"}\n" +
		"```\n" +
		"Please activate these agents."

	agents := parseRecommendedAgents(text)
	if len(agents) != 2 {
		t.Fatalf("expected 2 recommended agents, got %d", len(agents))
	}
	if agents[0] != AgentTraining {
		t.Errorf("expected training, got %s", agents[0])
	}
	if agents[1] != AgentInference {
		t.Errorf("expected inference, got %s", agents[1])
	}
}

func TestParseRecommendedAgentsEmpty(t *testing.T) {
	agents := parseRecommendedAgents("")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents for empty text, got %d", len(agents))
	}
}

func TestParseRecommendedAgentsInvalidJSON(t *testing.T) {
	agents := parseRecommendedAgents("No JSON block here")
	if len(agents) != 0 {
		t.Errorf("expected 0 agents for text without JSON, got %d", len(agents))
	}
}

func TestEvidenceOverlap(t *testing.T) {
	a := []string{"timeout on DNS lookup", "packet loss detected"}
	b := []string{"timeout on DNS lookup", "high latency"}
	if !evidenceOverlap(a, b) {
		t.Error("expected evidence overlap for matching evidence")
	}

	c := []string{"completely different evidence"}
	if evidenceOverlap(a, c) {
		t.Error("expected no evidence overlap for different evidence")
	}
}

func TestKeywordOverlap(t *testing.T) {
	if !keywordOverlap("timeout error causing crash", "error timeout in service") {
		t.Error("expected keyword overlap for shared error/timeout keywords")
	}

	if keywordOverlap("hello world", "foo bar baz") {
		t.Error("expected no keyword overlap for unrelated text")
	}
}

func TestWarRoomHandoffTool(t *testing.T) {
	c := NewWarRoomCoordinator()
	SetWarRoomCoordinator(c)
	defer func() { SetWarRoomCoordinator(nil) }()

	tool := &WarRoomHandoffTool{}
	if tool.Name() != "warroom_handoff" {
		t.Errorf("expected tool name 'warroom_handoff', got %q", tool.Name())
	}

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing session_id")
	}
}

func TestWarRoomEvaluateTool(t *testing.T) {
	c := NewWarRoomCoordinator()
	SetWarRoomCoordinator(c)
	defer func() { SetWarRoomCoordinator(nil) }()

	tool := &WarRoomEvaluateTool{}
	if tool.Name() != "warroom_evaluate" {
		t.Errorf("expected tool name 'warroom_evaluate', got %q", tool.Name())
	}

	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing session_id")
	}
}

func TestRunAgentOptions(t *testing.T) {
	opt := RunAgentOptions{
		SessionID: "test-session",
		BlackboardSnapshotFn: func() string {
			return "snapshot data"
		},
	}
	if opt.SessionID != "test-session" {
		t.Errorf("expected session ID 'test-session', got %q", opt.SessionID)
	}
	if opt.BlackboardSnapshotFn() != "snapshot data" {
		t.Error("expected blackboard snapshot function to return data")
	}
}

func TestCrossReferenceStruct(t *testing.T) {
	finding := Finding{
		ID:          "f1",
		AgentType:   AgentNetwork,
		Category:    "root_cause",
		Title:       "Test",
		Description: "test",
		Confidence:  0.8,
		CrossReferences: []CrossReference{
			{
				FindingID:    "f2",
				ReferencedBy: AgentInfra,
				Agrees:       true,
				Notes:        "Supports the network finding",
			},
		},
		CreatedAt: time.Now(),
	}
	if len(finding.CrossReferences) != 1 {
		t.Errorf("expected 1 cross-reference, got %d", len(finding.CrossReferences))
	}
	if !finding.CrossReferences[0].Agrees {
		t.Error("expected cross-reference to agree")
	}
}

func TestWarRoomIntegrationDataFlow(t *testing.T) {
	coordinator := InitWarRoom(nil)
	defer Shutdown()

	session, err := coordinator.StartWarRoom(context.Background(), WarRoomRequest{
		Title:       "node3 故障排查",
		Description: "node3 OOM 导致级联故障",
		AgentTypes:  []DomainAgentType{AgentInfra, AgentApp},
	})
	if err != nil {
		t.Fatalf("StartWarRoom failed: %v", err)
	}

	bb, ok := coordinator.GetBlackboard(session.ID)
	if !ok || bb == nil {
		t.Fatal("blackboard should exist for session")
	}

	bb.WriteEntry(BlackboardEntry{
		Key: "node3_memory", Value: "95% used", Author: AgentInfra, Category: "metric", Timestamp: time.Now(),
	})
	bb.WriteEntry(BlackboardEntry{
		Key: "app_errors", Value: "503 spike", Author: AgentApp, Category: "observation", Timestamp: time.Now(),
	})

	coordinator.SubmitFinding(session.ID, AgentInfra, Finding{
		ID: "f1", AgentType: AgentInfra, Category: "symptom",
		Title: "High memory on node3", Description: "node3 memory at 95%, OOMKilled pods",
		Confidence: 0.7, Evidence: []string{"kubectl top node"}, CreatedAt: time.Now(),
	})

	coordinator.SubmitFinding(session.ID, AgentApp, Finding{
		ID: "f2", AgentType: AgentApp, Category: "symptom",
		Title: "503 errors from node3", Description: "Application 503 errors, connection pool exhausted on node3",
		Confidence: 0.6, Evidence: []string{"node3 memory", "connection pool"}, CreatedAt: time.Now(),
	})

	s := coordinator.GetSession(session.ID)
	if len(s.Findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(s.Findings))
	}

	hasCrossRef := false
	for _, f := range s.Findings {
		if len(f.CrossReferences) > 0 {
			hasCrossRef = true
			break
		}
	}
	if !hasCrossRef {
		t.Error("expected cross-references between findings about node3")
	}

	for _, f := range s.Findings {
		if f.Confidence < 0.1 || f.Confidence > 0.95 {
			t.Errorf("finding %s confidence %.2f out of bounds", f.ID, f.Confidence)
		}
	}

	snapshot := bb.GetSnapshot()
	if !strings.Contains(snapshot, "node3_memory") || !strings.Contains(snapshot, "app_errors") {
		t.Error("blackboard snapshot should contain written entries")
	}

	coordinator.EvolveConfidence(session.ID, "f1", 0.1, "test boost")
	s = coordinator.GetSession(session.ID)
	for _, f := range s.Findings {
		if f.ID == "f1" && f.Confidence < 0.7 {
			t.Errorf("f1 confidence should have increased, got %.2f", f.Confidence)
		}
	}

	t.Logf("Data flow test: %d findings, %d cross-refs, %d timeline entries",
		len(s.Findings), countCrossRefs(s.Findings), len(s.Timeline))
}

func countCrossRefs(findings []Finding) int {
	total := 0
	for _, f := range findings {
		total += len(f.CrossReferences)
	}
	return total
}

func TestBlackboardWriteTool(t *testing.T) {
	runner := newMockAgentRunner()
	coordinator := InitWarRoom(runner)
	defer Shutdown()

	// Create session to get a blackboard
	session, err := coordinator.StartWarRoom(context.Background(), WarRoomRequest{
		Title:       "Test Blackboard Write",
		Description: "test blackboard write tool",
		AgentTypes:  []DomainAgentType{AgentInfra},
	})
	if err != nil {
		t.Fatalf("StartWarRoom failed: %v", err)
	}

	tool := &WarRoomBlackboardWriteTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"session_id": session.ID,
		"key":        "gpu_status",
		"value":      "GPU 0: 95% memory used, temperature 89C",
		"category":   "metric",
	})
	if err != nil {
		t.Fatalf("BlackboardWriteTool failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}
	if resultMap["status"] != "written" {
		t.Errorf("expected status 'written', got %v", resultMap["status"])
	}

	// Verify entry was written
	bb, _ := coordinator.GetBlackboard(session.ID)
	entries := bb.ReadEntries("")
	found := false
	for _, e := range entries {
		if e.Key == "gpu_status" {
			found = true
			if e.Category != "metric" {
				t.Errorf("expected category 'metric', got %s", e.Category)
			}
		}
	}
	if !found {
		t.Error("expected to find gpu_status entry in blackboard")
	}
}
