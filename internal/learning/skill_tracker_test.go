package learning

import (
	"path/filepath"
	"testing"

	"github.com/instructkr/smartclaw/internal/store"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return s
}

func TestSkillTracker_RecordAndQuery(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	tracker := NewSkillTracker(s)

	err := tracker.RecordInvocation("test-skill", "session-1")
	if err != nil {
		t.Fatalf("record invocation failed: %v", err)
	}

	err = tracker.RecordOutcome("test-skill", OutcomeSuccess, "session-1")
	if err != nil {
		t.Fatalf("record outcome failed: %v", err)
	}

	score, err := tracker.GetEffectivenessScore("test-skill")
	if err != nil {
		t.Fatalf("get effectiveness score failed: %v", err)
	}

	if score.SkillID != "test-skill" {
		t.Errorf("expected skill_id test-skill, got %s", score.SkillID)
	}
	if score.TotalInvocations != 1 {
		t.Errorf("expected 1 invocation, got %d", score.TotalInvocations)
	}
	if score.Successes != 1 {
		t.Errorf("expected 1 success, got %d", score.Successes)
	}
	if score.Score != 1.0 {
		t.Errorf("expected score 1.0, got %f", score.Score)
	}
}

func TestSkillTracker_FailureReducesScore(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	tracker := NewSkillTracker(s)

	tracker.RecordInvocation("bad-skill", "s1")
	tracker.RecordOutcome("bad-skill", OutcomeFailed, "s1")
	tracker.RecordInvocation("bad-skill", "s2")
	tracker.RecordOutcome("bad-skill", OutcomeFailed, "s2")
	tracker.RecordInvocation("bad-skill", "s3")
	tracker.RecordOutcome("bad-skill", OutcomeFailed, "s3")

	score, _ := tracker.GetEffectivenessScore("bad-skill")
	if score.Score >= DecayThreshold {
		t.Errorf("expected score below threshold %f, got %f", DecayThreshold, score.Score)
	}
}

func TestSkillTracker_UserOverridePartialCredit(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	tracker := NewSkillTracker(s)

	tracker.RecordInvocation("partial-skill", "s1")
	tracker.RecordOutcome("partial-skill", OutcomeUserOverride, "s1")

	score, _ := tracker.GetEffectivenessScore("partial-skill")
	if score.Score != 0.3 {
		t.Errorf("expected score 0.3 for user_override, got %f", score.Score)
	}
}

func TestSkillTracker_DecayCandidates(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	tracker := NewSkillTracker(s)

	for i := 0; i < 3; i++ {
		sid := filepath.Join("s", string(rune('0'+i)))
		tracker.RecordInvocation("decaying-skill", sid)
		tracker.RecordOutcome("decaying-skill", OutcomeFailed, sid)
	}

	tracker.RecordInvocation("good-skill", "s-good")
	tracker.RecordOutcome("good-skill", OutcomeSuccess, "s-good")

	candidates, err := tracker.GetDecayCandidates()
	if err != nil {
		t.Fatalf("get decay candidates failed: %v", err)
	}

	found := false
	for _, c := range candidates {
		if c == "decaying-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected decaying-skill in decay candidates")
	}
}

func TestSkillTracker_NilStore(t *testing.T) {
	tracker := NewSkillTracker(nil)

	err := tracker.RecordInvocation("skill", "session")
	if err != nil {
		t.Errorf("expected nil error with nil store, got %v", err)
	}

	err = tracker.RecordOutcome("skill", OutcomeSuccess, "session")
	if err != nil {
		t.Errorf("expected nil error with nil store, got %v", err)
	}

	score, err := tracker.GetEffectivenessScore("skill")
	if err != nil {
		t.Errorf("expected nil error with nil store, got %v", err)
	}
	if score.Score != 0.5 {
		t.Errorf("expected default score 0.5, got %f", score.Score)
	}
}

func TestSkillTracker_GetAllScores(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	tracker := NewSkillTracker(s)

	tracker.RecordInvocation("skill-a", "s1")
	tracker.RecordOutcome("skill-a", OutcomeSuccess, "s1")

	tracker.RecordInvocation("skill-b", "s2")
	tracker.RecordOutcome("skill-b", OutcomeFailed, "s2")

	scores, err := tracker.GetAllScores()
	if err != nil {
		t.Fatalf("get all scores failed: %v", err)
	}

	if len(scores) != 2 {
		t.Errorf("expected 2 skills, got %d", len(scores))
	}

	if scores["skill-a"].Score != 1.0 {
		t.Errorf("expected skill-a score 1.0, got %f", scores["skill-a"].Score)
	}
}
