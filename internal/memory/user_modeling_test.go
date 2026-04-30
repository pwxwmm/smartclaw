package memory

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

// --- Test helpers ---

func newTestStoreForModeling(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newTestPromptMemory(t *testing.T) *layers.PromptMemory {
	t.Helper()
	dir := t.TempDir()
	pm, err := layers.NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}
	return pm
}

// --- NewUserModelingEngine ---

func TestNewUserModelingEngine(t *testing.T) {
	s := newTestStoreForModeling(t)
	pm := newTestPromptMemory(t)

	engine := NewUserModelingEngine(s, pm, "user-1")
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}
	if engine.userID != "user-1" {
		t.Fatalf("expected userID %q, got %q", "user-1", engine.userID)
	}
	if engine.store != s {
		t.Fatal("expected store to be set")
	}
	if engine.promptMem != pm {
		t.Fatal("expected promptMem to be set")
	}
}

func TestNewUserModelingEngineEmptyUserID(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "")
	if engine.userID != "default" {
		t.Fatalf("expected userID %q for empty string, got %q", "default", engine.userID)
	}
}

func TestNewUserModelingEngineNilStore(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")
	if engine == nil {
		t.Fatal("expected non-nil engine even with nil store")
	}
}

// --- SynthesizeModel ---

func TestSynthesizeModelNilStore(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")
	_, err := engine.SynthesizeModel(context.Background())
	if err == nil {
		t.Fatal("expected error with nil store")
	}
	if !strings.Contains(err.Error(), "store not available") {
		t.Fatalf("expected store-not-available error, got %v", err)
	}
}

func TestSynthesizeModelEmptyObservations(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")

	snapshot, err := engine.SynthesizeModel(context.Background())
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snapshot.Preferences) != 0 {
		t.Fatalf("expected 0 preferences, got %d", len(snapshot.Preferences))
	}
	if snapshot.CommunicationStyle != "" {
		t.Fatalf("expected empty communication style, got %q", snapshot.CommunicationStyle)
	}
	if len(snapshot.KnowledgeBackground) != 0 {
		t.Fatalf("expected 0 knowledge entries, got %d", len(snapshot.KnowledgeBackground))
	}
	if len(snapshot.TopPatterns) != 0 {
		t.Fatalf("expected 0 patterns, got %d", len(snapshot.TopPatterns))
	}
	if len(snapshot.Conflicts) != 0 {
		t.Fatalf("expected 0 conflicts, got %d", len(snapshot.Conflicts))
	}
	if snapshot.LastUpdated.IsZero() {
		t.Fatal("expected non-zero LastUpdated")
	}
}

func TestSynthesizeModelWithObservations(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	// Record various observation types
	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "code_style", "naming", "camelCase", 0.7, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "communication_style", "style", "concise", 0.6, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "knowledge", "language", "Go", 0.9, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "knowledge", "framework", "React", 0.7, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "pattern", "workflow", "edit-test-commit", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	if len(snapshot.Preferences) != 2 {
		t.Fatalf("expected 2 preferences (code_style entries), got %d", len(snapshot.Preferences))
	}
	if snapshot.Preferences["indentation"] != "tabs" {
		t.Fatalf("expected indentation=tabs, got %q", snapshot.Preferences["indentation"])
	}
	if snapshot.Preferences["naming"] != "camelCase" {
		t.Fatalf("expected naming=camelCase, got %q", snapshot.Preferences["naming"])
	}
	if snapshot.CommunicationStyle != "concise" {
		t.Fatalf("expected communication style %q, got %q", "concise", snapshot.CommunicationStyle)
	}
	if len(snapshot.KnowledgeBackground) != 2 {
		t.Fatalf("expected 2 knowledge entries, got %d", len(snapshot.KnowledgeBackground))
	}
	if len(snapshot.TopPatterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(snapshot.TopPatterns))
	}
	if snapshot.TopPatterns[0].Pattern != "edit-test-commit" {
		t.Fatalf("expected pattern %q, got %q", "edit-test-commit", snapshot.TopPatterns[0].Pattern)
	}
}

func TestSynthesizeModelWithConflicts(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	// Record conflicting observations with different values
	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "code_style", "indentation", "spaces:4", 0.6, "sess-2"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	if len(snapshot.Conflicts) == 0 {
		t.Fatal("expected conflicts from differing high-confidence observations")
	}
	conflict := snapshot.Conflicts[0]
	if conflict.Category != "code_style" {
		t.Fatalf("expected conflict category %q, got %q", "code_style", conflict.Category)
	}
	if conflict.Key != "indentation" {
		t.Fatalf("expected conflict key %q, got %q", "indentation", conflict.Key)
	}
	if !conflict.Resolved {
		t.Fatal("expected conflict to be resolved")
	}
	// Higher confidence thesis should win
	if conflict.Resolution != "tabs" {
		t.Fatalf("expected resolution %q, got %q", "tabs", conflict.Resolution)
	}
}

func TestSynthesizeModelPreferenceCategory(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "preference", "editor", "vim", 0.9, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	if snapshot.Preferences["editor"] != "vim" {
		t.Fatalf("expected editor=vim, got %q", snapshot.Preferences["editor"])
	}
}

func TestSynthesizeModelWorkflowPatternCategory(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "workflow_pattern", "tool", "bash:ls", 0.7, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	if len(snapshot.TopPatterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(snapshot.TopPatterns))
	}
	if snapshot.TopPatterns[0].Pattern != "bash:ls" {
		t.Fatalf("expected pattern %q, got %q", "bash:ls", snapshot.TopPatterns[0].Pattern)
	}
}

func TestSynthesizeModelPatternFrequencyAndSorting(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	// Record "pattern-a" 3 times under key "workflow-a" and "pattern-b" 1 time under key "workflow-b"
	for i := 0; i < 3; i++ {
		if err := engine.RecordObservation(ctx, "pattern", "workflow-a", "pattern-a", 0.7, "sess-1"); err != nil {
			t.Fatalf("RecordObservation error: %v", err)
		}
	}
	if err := engine.RecordObservation(ctx, "pattern", "workflow-b", "pattern-b", 0.7, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	if len(snapshot.TopPatterns) != 2 {
		t.Fatalf("expected 2 patterns, got %d", len(snapshot.TopPatterns))
	}
	if snapshot.TopPatterns[0].Pattern != "pattern-a" {
		t.Fatalf("expected most frequent pattern first, got %q", snapshot.TopPatterns[0].Pattern)
	}
	if snapshot.TopPatterns[0].Frequency != 3 {
		t.Fatalf("expected frequency 3 for pattern-a, got %d", snapshot.TopPatterns[0].Frequency)
	}
	if snapshot.TopPatterns[1].Frequency != 1 {
		t.Fatalf("expected frequency 1 for pattern-b, got %d", snapshot.TopPatterns[1].Frequency)
	}
}

func TestSynthesizeModelTop10PatternsLimit(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	// Record 15 distinct patterns
	for i := 0; i < 15; i++ {
		if err := engine.RecordObservation(ctx, "pattern", "workflow", "pattern-"+string(rune('A'+i)), 0.7, "sess-1"); err != nil {
			t.Fatalf("RecordObservation error: %v", err)
		}
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	if len(snapshot.TopPatterns) > 10 {
		t.Fatalf("expected at most 10 patterns, got %d", len(snapshot.TopPatterns))
	}
}

func TestSynthesizeModelKnowledgeDedup(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "knowledge", "lang", "Go", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "knowledge", "lang", "Go", 0.9, "sess-2"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	// The dedupStrings function should deduplicate after resolving.
	// Both observations resolve to the same value "Go" for the same key,
	// so only one "Go" should appear.
	for i, k := range snapshot.KnowledgeBackground {
		for j, k2 := range snapshot.KnowledgeBackground {
			if i != j && k == k2 {
				t.Fatalf("duplicate knowledge entry: %q at positions %d and %d", k, i, j)
			}
		}
	}
}

func TestSynthesizeModelCachesSnapshot(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	cached := engine.GetSnapshot()
	if cached != snapshot {
		t.Fatal("expected GetSnapshot to return the same snapshot object from last synthesis")
	}
}

// --- GetSnapshot ---

func TestGetSnapshotBeforeSynthesis(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")
	snapshot := engine.GetSnapshot()
	if snapshot != nil {
		t.Fatal("expected nil snapshot before synthesis")
	}
}

// --- RecordObservation ---

func TestRecordObservationNilStore(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")
	err := engine.RecordObservation(context.Background(), "code_style", "indentation", "tabs", 0.8, "sess-1")
	if err != nil {
		t.Fatalf("expected nil error with nil store, got %v", err)
	}
}

func TestRecordObservationPersists(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1")
	if err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	// Verify data was written by querying directly
	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = ? AND key = ?", "code_style", "indentation").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 observation, got %d", count)
	}
}

func TestRecordObservationMultiple(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1")
		if err != nil {
			t.Fatalf("RecordObservation %d error: %v", i, err)
		}
	}

	var count int
	err := s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = ?", "code_style").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5 observations, got %d", count)
	}
}

// --- GetConflicts ---

func TestGetConflictsNoSnapshot(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")
	conflicts := engine.GetConflicts()
	if conflicts != nil {
		t.Fatalf("expected nil conflicts with no snapshot, got %v", conflicts)
	}
}

func TestGetConflictsAllResolved(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "code_style", "indentation", "spaces:2", 0.6, "sess-2"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	if _, err := engine.SynthesizeModel(ctx); err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	// All conflicts in findConflicts are marked as resolved
	conflicts := engine.GetConflicts()
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 unresolved conflicts (all are resolved), got %d", len(conflicts))
	}
}

func TestGetConflictsFiltersResolved(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "code_style", "indentation", "spaces:2", 0.6, "sess-2"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	// Manually add an unresolved conflict to test filtering
	engine.mu.Lock()
	snapshot.Conflicts = append(snapshot.Conflicts, ObservationConflict{
		Category:   "test",
		Key:        "test",
		Thesis:     "a",
		Antithesis: "b",
		Resolved:   false,
	})
	engine.mu.Unlock()

	conflicts := engine.GetConflicts()
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 unresolved conflict, got %d", len(conflicts))
	}
	if conflicts[0].Category != "test" {
		t.Fatalf("expected category %q, got %q", "test", conflicts[0].Category)
	}
}

// --- loadObservations ---

func TestLoadObservationsDefaultUser(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "default")
	ctx := context.Background()

	// Insert observation directly
	_, err := s.DB().ExecContext(ctx,
		"INSERT INTO user_observations (category, key, value, confidence, observed_at, session_id, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"code_style", "indentation", "tabs", 0.8, time.Now().Format(time.RFC3339), "sess-1", "default",
	)
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}

	observations, err := engine.loadObservations(ctx)
	if err != nil {
		t.Fatalf("loadObservations error: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(observations))
	}
	if observations[0].Category != "code_style" {
		t.Fatalf("expected category %q, got %q", "code_style", observations[0].Category)
	}
}

func TestLoadObservationsSpecificUser(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-42")
	ctx := context.Background()

	// Insert observation for a different user
	_, err := s.DB().ExecContext(ctx,
		"INSERT INTO user_observations (category, key, value, confidence, observed_at, session_id, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"code_style", "indentation", "tabs", 0.8, time.Now().Format(time.RFC3339), "sess-1", "user-42",
	)
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}

	// Insert observation for another user
	_, err = s.DB().ExecContext(ctx,
		"INSERT INTO user_observations (category, key, value, confidence, observed_at, session_id, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"code_style", "indentation", "spaces", 0.8, time.Now().Format(time.RFC3339), "sess-2", "other-user",
	)
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}

	observations, err := engine.loadObservations(ctx)
	if err != nil {
		t.Fatalf("loadObservations error: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected 1 observation for user-42, got %d", len(observations))
	}
}

func TestLoadObservationsSQLiteDatetimeFormat(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "default")
	ctx := context.Background()

	// Insert with SQLite default datetime format (not RFC3339)
	_, err := s.DB().ExecContext(ctx,
		"INSERT INTO user_observations (category, key, value, confidence, observed_at, session_id, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"code_style", "indentation", "tabs", 0.8, "2025-01-15 10:30:00", "sess-1", "default",
	)
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}

	observations, err := engine.loadObservations(ctx)
	if err != nil {
		t.Fatalf("loadObservations error: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(observations))
	}
	if observations[0].ObservedAt.IsZero() {
		t.Fatal("expected non-zero ObservedAt after parsing SQLite datetime")
	}
}

func TestLoadObservationsEmpty(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")

	observations, err := engine.loadObservations(context.Background())
	if err != nil {
		t.Fatalf("loadObservations error: %v", err)
	}
	if len(observations) != 0 {
		t.Fatalf("expected 0 observations, got %d", len(observations))
	}
}

// --- findConflicts ---

func TestFindConflictsNoConflicts(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	obsGroup := []observation{
		{ID: 1, Category: "code_style", Key: "indentation", Value: "tabs", Confidence: 0.8, ObservedAt: time.Now()},
		{ID: 2, Category: "code_style", Key: "indentation", Value: "tabs", Confidence: 0.7, ObservedAt: time.Now()},
	}

	conflicts := engine.findConflicts("code_style", "indentation", obsGroup)
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts for same-value observations, got %d", len(conflicts))
	}
}

func TestFindConflictsWithConflicts(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	obsGroup := []observation{
		{ID: 1, Category: "code_style", Key: "indentation", Value: "tabs", Confidence: 0.8, ObservedAt: time.Now()},
		{ID: 2, Category: "code_style", Key: "indentation", Value: "spaces:4", Confidence: 0.7, ObservedAt: time.Now()},
	}

	conflicts := engine.findConflicts("code_style", "indentation", obsGroup)
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	c := conflicts[0]
	if c.Thesis != "tabs" {
		t.Fatalf("expected thesis %q, got %q", "tabs", c.Thesis)
	}
	if c.Antithesis != "spaces:4" {
		t.Fatalf("expected antithesis %q, got %q", "spaces:4", c.Antithesis)
	}
	if c.ThesisConfidence != 0.8 {
		t.Fatalf("expected thesis confidence 0.8, got %f", c.ThesisConfidence)
	}
	if c.AntithesisConfidence != 0.7 {
		t.Fatalf("expected antithesis confidence 0.7, got %f", c.AntithesisConfidence)
	}
	if !c.Resolved {
		t.Fatal("expected conflict to be resolved")
	}
}

func TestFindConflictsLowConfidenceFiltered(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	// One observation with confidence <= 0.5, another above
	obsGroup := []observation{
		{ID: 1, Category: "code_style", Key: "indentation", Value: "tabs", Confidence: 0.8, ObservedAt: time.Now()},
		{ID: 2, Category: "code_style", Key: "indentation", Value: "spaces:4", Confidence: 0.3, ObservedAt: time.Now()},
	}

	conflicts := engine.findConflicts("code_style", "indentation", obsGroup)
	// The low-confidence observation is filtered out, leaving only one distinct value
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts when low-confidence values filtered, got %d", len(conflicts))
	}
}

func TestFindConflictsMultipleDistinctValues(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	obsGroup := []observation{
		{ID: 1, Category: "code_style", Key: "indentation", Value: "tabs", Confidence: 0.9, ObservedAt: time.Now()},
		{ID: 2, Category: "code_style", Key: "indentation", Value: "spaces:2", Confidence: 0.7, ObservedAt: time.Now()},
		{ID: 3, Category: "code_style", Key: "indentation", Value: "spaces:4", Confidence: 0.6, ObservedAt: time.Now()},
	}

	conflicts := engine.findConflicts("code_style", "indentation", obsGroup)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts (thesis vs each antithesis), got %d", len(conflicts))
	}
	// All should have "tabs" as thesis
	for i, c := range conflicts {
		if c.Thesis != "tabs" {
			t.Fatalf("conflict[%d]: expected thesis %q, got %q", i, "tabs", c.Thesis)
		}
	}
}

func TestFindConflictsSingleValueNoConflict(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	obsGroup := []observation{
		{ID: 1, Category: "code_style", Key: "indentation", Value: "tabs", Confidence: 0.8, ObservedAt: time.Now()},
	}

	conflicts := engine.findConflicts("code_style", "indentation", obsGroup)
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts for single-value group, got %d", len(conflicts))
	}
}

func TestFindConflictsAllLowConfidence(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	obsGroup := []observation{
		{ID: 1, Category: "code_style", Key: "indentation", Value: "tabs", Confidence: 0.3, ObservedAt: time.Now()},
		{ID: 2, Category: "code_style", Key: "indentation", Value: "spaces:4", Confidence: 0.4, ObservedAt: time.Now()},
	}

	conflicts := engine.findConflicts("code_style", "indentation", obsGroup)
	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts when all low confidence, got %d", len(conflicts))
	}
}

// --- resolveObservations ---

func TestResolveObservationsEmpty(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")
	result := engine.resolveObservations(nil)
	if result != (observation{}) {
		t.Fatal("expected zero observation for empty group")
	}
}

func TestResolveObservationsSingle(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	obs := observation{ID: 1, Value: "tabs", Confidence: 0.8, ObservedAt: time.Now()}
	result := engine.resolveObservations([]observation{obs})
	if result.Value != "tabs" {
		t.Fatalf("expected value %q, got %q", "tabs", result.Value)
	}
}

func TestResolveObservationsHigherConfidenceWins(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	obs1 := observation{ID: 1, Value: "tabs", Confidence: 0.6, ObservedAt: time.Now().Add(-1 * time.Hour)}
	obs2 := observation{ID: 2, Value: "spaces", Confidence: 0.9, ObservedAt: time.Now()}

	result := engine.resolveObservations([]observation{obs1, obs2})
	if result.Value != "spaces" {
		t.Fatalf("expected higher confidence value %q, got %q", "spaces", result.Value)
	}
}

func TestResolveObservationsRecencyTiebreak(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")

	now := time.Now()
	obs1 := observation{ID: 1, Value: "tabs", Confidence: 0.8, ObservedAt: now.Add(-1 * time.Hour)}
	obs2 := observation{ID: 2, Value: "spaces", Confidence: 0.8, ObservedAt: now}

	result := engine.resolveObservations([]observation{obs1, obs2})
	if result.Value != "spaces" {
		t.Fatalf("expected more recent value %q on tie, got %q", "spaces", result.Value)
	}
}

// --- dedupStrings ---

func TestDedupStringsEmpty(t *testing.T) {
	result := dedupStrings(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestDedupStringsNoDuplicates(t *testing.T) {
	input := []string{"a", "b", "c"}
	result := dedupStrings(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
}

func TestDedupStringsRemovesDuplicates(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b"}
	result := dedupStrings(input)
	if len(result) != 3 {
		t.Fatalf("expected 3 unique items, got %d: %v", len(result), result)
	}
	// Order preserved
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Fatalf("expected [a b c], got %v", result)
	}
}

func TestDedupStringsTrimsSpace(t *testing.T) {
	input := []string{"  a  ", "b", " a "}
	result := dedupStrings(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 items after trim, got %d: %v", len(result), result)
	}
	if result[0] != "a" {
		t.Fatalf("expected trimmed %q, got %q", "a", result[0])
	}
}

func TestDedupStringsFiltersEmpty(t *testing.T) {
	input := []string{"a", "", "  ", "b"}
	result := dedupStrings(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 items (empty filtered), got %d: %v", len(result), result)
	}
}

// --- Concurrent access ---

func TestSynthesizeModelConcurrentAccess(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	// Pre-populate
	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	var wg sync.WaitGroup
	const count = 10

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := engine.SynthesizeModel(ctx)
			if err != nil {
				t.Errorf("SynthesizeModel error: %v", err)
			}
			_ = engine.GetSnapshot()
		}()
	}
	wg.Wait()
}
