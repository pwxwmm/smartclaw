package warroom

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.Exec("PRAGMA foreign_keys=ON")
	if _, err := db.Exec(schemaForTest()); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return db
}

func schemaForTest() string {
	return `
CREATE TABLE IF NOT EXISTS warroom_sessions (
    id TEXT PRIMARY KEY,
    incident_id TEXT DEFAULT '',
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    context TEXT DEFAULT '{}',
    created_at TEXT NOT NULL,
    closed_at TEXT
);

CREATE TABLE IF NOT EXISTS warroom_agents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES warroom_sessions(id) ON DELETE CASCADE,
    agent_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'spawning',
    assigned_at TEXT NOT NULL,
    last_active TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS warroom_findings (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES warroom_sessions(id) ON DELETE CASCADE,
    agent_type TEXT NOT NULL,
    category TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    confidence REAL NOT NULL DEFAULT 0.5,
    evidence TEXT DEFAULT '[]',
    cross_references TEXT DEFAULT '[]',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS warroom_timeline (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES warroom_sessions(id) ON DELETE CASCADE,
    agent_type TEXT DEFAULT '',
    event TEXT NOT NULL,
    details TEXT DEFAULT '',
    timestamp TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS warroom_blackboard_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES warroom_sessions(id) ON DELETE CASCADE,
    agent_type TEXT NOT NULL,
    content TEXT NOT NULL,
    entry_type TEXT DEFAULT 'observation',
    timestamp TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS warroom_hypotheses (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES warroom_sessions(id) ON DELETE CASCADE,
    agent_type TEXT NOT NULL,
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'proposed',
    confidence REAL NOT NULL DEFAULT 0.5,
    evidence TEXT DEFAULT '[]',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS warroom_shared_facts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES warroom_sessions(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    source_agent TEXT NOT NULL,
    confirmation_count INTEGER DEFAULT 1,
    confirming_agents TEXT DEFAULT '[]',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_warroom_agents_session ON warroom_agents(session_id);
CREATE INDEX IF NOT EXISTS idx_warroom_findings_session ON warroom_findings(session_id);
CREATE INDEX IF NOT EXISTS idx_warroom_timeline_session ON warroom_timeline(session_id);
CREATE INDEX IF NOT EXISTS idx_warroom_blackboard_session ON warroom_blackboard_entries(session_id);
CREATE INDEX IF NOT EXISTS idx_warroom_hypotheses_session ON warroom_hypotheses(session_id);
CREATE INDEX IF NOT EXISTS idx_warroom_facts_session ON warroom_shared_facts(session_id);
`
}

func TestWarRoomPersistence_SaveLoadSession(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	now := time.Now().Truncate(time.Millisecond)
	session := &WarRoomSession{
		ID:          "test-session-1",
		IncidentID:  "INC-42",
		Title:       "DB Latency Spike",
		Description: "Elevated database latency in production",
		Status:      WarRoomActive,
		Agents: []AgentAssignment{
			{
				AgentType:  AgentNetwork,
				Status:     AgentStatusRunning,
				AssignedAt: now,
				LastActive: now,
				Findings:   []Finding{},
			},
			{
				AgentType:  AgentDatabase,
				Status:     AgentStatusSpawning,
				AssignedAt: now,
				LastActive: now,
				Findings:   []Finding{},
			},
		},
		Findings: []Finding{
			{
				ID:          "f1",
				AgentType:   AgentNetwork,
				Category:    "symptom",
				Title:       "DNS Timeout",
				Description: "DNS resolution timeout on primary endpoint",
				Confidence:  0.85,
				Evidence:    []string{"nslookup timeout", "dig +timeout"},
				CrossReferences: []CrossReference{
					{
						FindingID:   "f2",
						ReferencedBy: AgentDatabase,
						Agrees:      true,
						Notes:       "DB also sees timeouts",
					},
				},
				CreatedAt: now,
			},
		},
		Timeline: []TimelineEntry{
			{
				Timestamp: now,
				Event:     "war_room_started",
				Details:   "Started with 2 agents",
			},
			{
				Timestamp: now,
				AgentType: AgentNetwork,
				Event:     "finding_submitted",
				Details:   "DNS Timeout",
			},
		},
		CreatedAt: now,
		Context:   map[string]any{"region": "us-east-1", "severity": "high"},
	}

	err := store.SaveSession(session)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	loaded, err := store.LoadSession("test-session-1")
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("ID: got %q, want %q", loaded.ID, session.ID)
	}
	if loaded.IncidentID != session.IncidentID {
		t.Errorf("IncidentID: got %q, want %q", loaded.IncidentID, session.IncidentID)
	}
	if loaded.Title != session.Title {
		t.Errorf("Title: got %q, want %q", loaded.Title, session.Title)
	}
	if loaded.Description != session.Description {
		t.Errorf("Description: got %q, want %q", loaded.Description, session.Description)
	}
	if loaded.Status != session.Status {
		t.Errorf("Status: got %q, want %q", loaded.Status, session.Status)
	}
	if len(loaded.Agents) != 2 {
		t.Fatalf("Agents: got %d, want 2", len(loaded.Agents))
	}
	if loaded.Agents[0].AgentType != AgentNetwork {
		t.Errorf("Agent[0].AgentType: got %q, want %q", loaded.Agents[0].AgentType, AgentNetwork)
	}
	if loaded.Agents[0].Status != AgentStatusRunning {
		t.Errorf("Agent[0].Status: got %q, want %q", loaded.Agents[0].Status, AgentStatusRunning)
	}
	if len(loaded.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1", len(loaded.Findings))
	}
	if loaded.Findings[0].ID != "f1" {
		t.Errorf("Finding[0].ID: got %q, want %q", loaded.Findings[0].ID, "f1")
	}
	if loaded.Findings[0].Confidence != 0.85 {
		t.Errorf("Finding[0].Confidence: got %f, want 0.85", loaded.Findings[0].Confidence)
	}
	if len(loaded.Findings[0].Evidence) != 2 {
		t.Errorf("Finding[0].Evidence: got %d, want 2", len(loaded.Findings[0].Evidence))
	}
	if len(loaded.Findings[0].CrossReferences) != 1 {
		t.Errorf("Finding[0].CrossReferences: got %d, want 1", len(loaded.Findings[0].CrossReferences))
	}
	if !loaded.Findings[0].CrossReferences[0].Agrees {
		t.Error("Finding[0].CrossReferences[0].Agrees should be true")
	}
	if len(loaded.Timeline) != 2 {
		t.Fatalf("Timeline: got %d, want 2", len(loaded.Timeline))
	}
	if loaded.Timeline[0].Event != "war_room_started" {
		t.Errorf("Timeline[0].Event: got %q, want %q", loaded.Timeline[0].Event, "war_room_started")
	}
	if loaded.Context["region"] != "us-east-1" {
		t.Errorf("Context[region]: got %v, want us-east-1", loaded.Context["region"])
	}
	if loaded.CreatedAt.Sub(now) > time.Second {
		t.Errorf("CreatedAt: got %v, want ~%v", loaded.CreatedAt, now)
	}
}

func TestWarRoomPersistence_SaveLoadClosedSession(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	now := time.Now().Truncate(time.Millisecond)
	closedAt := now.Add(5 * time.Minute)
	session := &WarRoomSession{
		ID:          "closed-session",
		Title:       "Resolved Incident",
		Description: "This was resolved",
		Status:      WarRoomClosed,
		CreatedAt:   now,
		ClosedAt:    &closedAt,
		Context:     map[string]any{},
	}

	store.SaveSession(session)

	loaded, err := store.LoadSession("closed-session")
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	if loaded.Status != WarRoomClosed {
		t.Errorf("Status: got %q, want %q", loaded.Status, WarRoomClosed)
	}
	if loaded.ClosedAt == nil {
		t.Fatal("ClosedAt should not be nil")
	}
	if loaded.ClosedAt.Sub(closedAt) > time.Second {
		t.Errorf("ClosedAt: got %v, want ~%v", loaded.ClosedAt, closedAt)
	}
}

func TestWarRoomPersistence_SaveLoadBlackboard(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	sessionID := "bb-test-session"
	session := &WarRoomSession{
		ID:          sessionID,
		Title:       "BB Test",
		Description: "Blackboard persistence test",
		Status:      WarRoomActive,
		CreatedAt:   time.Now(),
		Context:     map[string]any{},
	}
	store.SaveSession(session)

	bb := NewBlackboard(sessionID)
	bb.WriteEntry(BlackboardEntry{
		Key:       "dns_status",
		Value:     "DNS resolution failing for api.example.com",
		Author:    AgentNetwork,
		Category:  "observation",
		Timestamp: time.Now().Truncate(time.Millisecond),
	})
	bb.WriteEntry(BlackboardEntry{
		Key:       "cpu_usage",
		Value:     "92% utilization",
		Author:    AgentInfra,
		Category:  "metric",
		Timestamp: time.Now().Truncate(time.Millisecond),
	})

	bb.AddHypothesis(Hypothesis{
		ID:                    "h1",
		Description:           "DNS misconfiguration causing service failures",
		ProposedBy:            AgentNetwork,
		Confidence:            0.7,
		SupportingEvidence:    []string{"nslookup timeout", "dig failure"},
		ContradictingEvidence: []string{"internal DNS works"},
		Status:                "proposed",
	})

	bb.AddSharedFact(SharedFact{
		Content:     "Service A is down",
		Source:      AgentApp,
		ConfirmedBy: []DomainAgentType{AgentApp, AgentInfra},
		Confidence:  0.8,
	})

	err := store.SaveBlackboard(sessionID, bb)
	if err != nil {
		t.Fatalf("SaveBlackboard failed: %v", err)
	}

	loaded, err := store.LoadBlackboard(sessionID)
	if err != nil {
		t.Fatalf("LoadBlackboard failed: %v", err)
	}

	entries := loaded.ReadEntries("")
	if len(entries) != 2 {
		t.Fatalf("entries: got %d, want 2", len(entries))
	}

	obs := loaded.ReadEntries("observation")
	if len(obs) != 1 {
		t.Fatalf("observation entries: got %d, want 1", len(obs))
	}
	if obs[0].Key != "dns_status" {
		t.Errorf("observation key: got %q, want %q", obs[0].Key, "dns_status")
	}
	if obs[0].Author != AgentNetwork {
		t.Errorf("observation author: got %q, want %q", obs[0].Author, AgentNetwork)
	}

	hypotheses := loaded.GetHypotheses()
	if len(hypotheses) != 1 {
		t.Fatalf("hypotheses: got %d, want 1", len(hypotheses))
	}
	if hypotheses[0].Description != "DNS misconfiguration causing service failures" {
		t.Errorf("hypothesis description: got %q", hypotheses[0].Description)
	}
	if hypotheses[0].ProposedBy != AgentNetwork {
		t.Errorf("hypothesis proposed_by: got %q, want %q", hypotheses[0].ProposedBy, AgentNetwork)
	}
	if hypotheses[0].Confidence != 0.7 {
		t.Errorf("hypothesis confidence: got %f, want 0.7", hypotheses[0].Confidence)
	}
	if len(hypotheses[0].SupportingEvidence) != 2 {
		t.Errorf("hypothesis supporting evidence: got %d, want 2", len(hypotheses[0].SupportingEvidence))
	}
	if len(hypotheses[0].ContradictingEvidence) != 1 {
		t.Errorf("hypothesis contradicting evidence: got %d, want 1", len(hypotheses[0].ContradictingEvidence))
	}

	facts := loaded.GetSharedFacts()
	if len(facts) != 1 {
		t.Fatalf("shared facts: got %d, want 1", len(facts))
	}
	if facts[0].Content != "Service A is down" {
		t.Errorf("fact content: got %q, want %q", facts[0].Content, "Service A is down")
	}
	if facts[0].Source != AgentApp {
		t.Errorf("fact source: got %q, want %q", facts[0].Source, AgentApp)
	}
	if len(facts[0].ConfirmedBy) != 2 {
		t.Errorf("fact confirmed_by: got %d, want 2", len(facts[0].ConfirmedBy))
	}
}

func TestWarRoomPersistence_ListSessions(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	now := time.Now()
	for i := 0; i < 3; i++ {
		session := &WarRoomSession{
			ID:          fmtSessionID(i),
			Title:       fmtSessionTitle(i),
			Description: "test",
			Status:      WarRoomActive,
			CreatedAt:   now,
			Context:     map[string]any{},
		}
		store.SaveSession(session)
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("sessions: got %d, want 3", len(sessions))
	}
}

func TestWarRoomPersistence_DeleteSession(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	session := &WarRoomSession{
		ID:          "delete-me",
		Title:       "Delete Test",
		Description: "test",
		Status:      WarRoomActive,
		CreatedAt:   time.Now(),
		Context:     map[string]any{},
	}
	store.SaveSession(session)

	err := store.DeleteSession("delete-me")
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	_, err = store.LoadSession("delete-me")
	if err == nil {
		t.Error("expected error loading deleted session")
	}
}

func TestWarRoomPersistence_RestoreSessions(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	c1 := NewWarRoomCoordinator()
	c1.SetStore(store)
	ctx := context.Background()

	s1, err := c1.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Session 1",
		Description: "First session",
		AgentTypes:  []DomainAgentType{AgentNetwork, AgentDatabase},
	})
	if err != nil {
		t.Fatalf("StartWarRoom 1 failed: %v", err)
	}

	s2, err := c1.StartWarRoom(ctx, WarRoomRequest{
		Title:       "Session 2",
		Description: "Second session",
		AgentTypes:  []DomainAgentType{AgentInfra},
	})
	if err != nil {
		t.Fatalf("StartWarRoom 2 failed: %v", err)
	}

	c1.SubmitFinding(s1.ID, AgentNetwork, Finding{
		ID:          "f-restore-1",
		AgentType:   AgentNetwork,
		Category:    "symptom",
		Title:       "DNS timeout",
		Description: "DNS resolution failing",
		Confidence:  0.8,
		Evidence:    []string{"nslookup timeout"},
		CreatedAt:   time.Now(),
	})

	bb, _ := c1.GetBlackboard(s2.ID)
	bb.WriteEntry(BlackboardEntry{
		Key: "node_health", Value: "degraded", Author: AgentInfra, Category: "metric",
	})
	c1.FlushBlackboard(s2.ID)

	c1.CloseSession(s1.ID)
	_ = s2

	c2 := NewWarRoomCoordinator()
	c2.SetStore(store)

	err = c2.RestoreSessions()
	if err != nil {
		t.Fatalf("RestoreSessions failed: %v", err)
	}

	sessions := c2.ListSessions()

	activeCount := 0
	for _, s := range sessions {
		if s.Status == WarRoomActive {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("active sessions after restore: got %d, want 1", activeCount)
	}

	for _, s := range sessions {
		if s.Status == WarRoomActive {
			if s.Title != "Session 2" {
				t.Errorf("restored active session title: got %q, want %q", s.Title, "Session 2")
			}
		}
	}

	restoredBB, ok := c2.GetBlackboard(s2.ID)
	if !ok || restoredBB == nil {
		t.Fatal("expected blackboard to be restored for active session")
	}
	entries := restoredBB.ReadEntries("")
	if len(entries) == 0 {
		t.Error("expected blackboard entries to be restored")
	}
}

func TestWarRoomPersistence_SaveNilSession(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	err := store.SaveSession(nil)
	if err == nil {
		t.Error("expected error saving nil session")
	}
}

func TestWarRoomPersistence_SaveNilBlackboard(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	err := store.SaveBlackboard("nonexistent", nil)
	if err == nil {
		t.Error("expected error saving nil blackboard")
	}
}

func TestWarRoomPersistence_LoadNonexistentSession(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	_, err := store.LoadSession("nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent session")
	}
}

func TestWarRoomPersistence_LoadNonexistentBlackboard(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	bb, err := store.LoadBlackboard("nonexistent")
	if err != nil {
		t.Fatalf("LoadBlackboard for nonexistent session should not error, got: %v", err)
	}
	if bb == nil {
		t.Fatal("LoadBlackboard should return empty blackboard, not nil")
	}
	entries := bb.ReadEntries("")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestWarRoomPersistence_UpdateSession(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)

	session := &WarRoomSession{
		ID:          "update-test",
		Title:       "Original",
		Description: "test",
		Status:      WarRoomActive,
		CreatedAt:   time.Now(),
		Context:     map[string]any{},
	}
	store.SaveSession(session)

	session.Title = "Updated"
	session.Findings = append(session.Findings, Finding{
		ID:          "f-updated",
		AgentType:   AgentNetwork,
		Category:    "symptom",
		Title:       "New Finding",
		Description: "Added after update",
		Confidence:  0.6,
		Evidence:    []string{"test evidence"},
		CreatedAt:   time.Now(),
	})
	store.SaveSession(session)

	loaded, err := store.LoadSession("update-test")
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}
	if loaded.Title != "Updated" {
		t.Errorf("Title: got %q, want %q", loaded.Title, "Updated")
	}
	if len(loaded.Findings) != 1 {
		t.Errorf("Findings: got %d, want 1", len(loaded.Findings))
	}
}

func TestWarRoomPersistence_SetStoreOnCoordinator(t *testing.T) {
	c := NewWarRoomCoordinator()

	if c.store != nil {
		t.Error("expected nil store initially")
	}

	db := openTestDB(t)
	defer db.Close()

	store := NewSQLiteWarRoomStore(db)
	c.SetStore(store)

	if c.store == nil {
		t.Error("expected store to be set")
	}
}

func TestWarRoomPersistence_FlushBlackboardNoStore(t *testing.T) {
	c := NewWarRoomCoordinator()

	err := c.FlushBlackboard("nonexistent")
	if err != nil {
		t.Errorf("FlushBlackboard with no store should return nil, got: %v", err)
	}
}

func fmtSessionID(i int) string {
	return "session-" + string(rune('0'+i))
}

func fmtSessionTitle(i int) string {
	titles := []string{"Alpha", "Beta", "Gamma"}
	if i < len(titles) {
		return titles[i]
	}
	return "Session"
}
