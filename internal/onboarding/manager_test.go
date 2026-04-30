package onboarding

import (
	"encoding/json"
	"testing"

	"github.com/instructkr/smartclaw/internal/store"
)

// helper: create a real SQLite store in a temp directory for integration tests
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

// --- GetSteps / GetStep tests ---

func TestGetSteps_ReturnsThreeSteps(t *testing.T) {
	steps := GetSteps()
	if len(steps) != 3 {
		t.Fatalf("GetSteps() returned %d steps, want 3", len(steps))
	}

	wantStepNums := []int{1, 2, 3}
	for i, s := range steps {
		if s.Step != wantStepNums[i] {
			t.Errorf("steps[%d].Step = %d, want %d", i, s.Step, wantStepNums[i])
		}
		if s.Title == "" {
			t.Errorf("steps[%d].Title is empty", i)
		}
		if s.Description == "" {
			t.Errorf("steps[%d].Description is empty", i)
		}
		if s.Prompt == "" {
			t.Errorf("steps[%d].Prompt is empty", i)
		}
		if s.SkillName == "" {
			t.Errorf("steps[%d].SkillName is empty", i)
		}
		if s.Insight == "" {
			t.Errorf("steps[%d].Insight is empty", i)
		}
	}
}

func TestGetStep_ValidNumbers(t *testing.T) {
	for n := 1; n <= 3; n++ {
		s := GetStep(n)
		if s == nil {
			t.Fatalf("GetStep(%d) returned nil, want non-nil", n)
		}
		if s.Step != n {
			t.Errorf("GetStep(%d).Step = %d, want %d", n, s.Step, n)
		}
	}
}

func TestGetStep_InvalidNumbers(t *testing.T) {
	invalidSteps := []int{0, 4, -1, 99}
	for _, n := range invalidSteps {
		s := GetStep(n)
		if s != nil {
			t.Errorf("GetStep(%d) returned non-nil (%+v), want nil", n, s)
		}
	}
}

func TestGetStep_StepContent(t *testing.T) {
	steps := GetSteps()
	step1 := GetStep(1)
	if step1.Title != steps[0].Title {
		t.Errorf("GetStep(1).Title = %q, want %q", step1.Title, steps[0].Title)
	}
	step2 := GetStep(2)
	if step2.SkillName != steps[1].SkillName {
		t.Errorf("GetStep(2).SkillName = %q, want %q", step2.SkillName, steps[1].SkillName)
	}
	step3 := GetStep(3)
	if step3.Insight != steps[2].Insight {
		t.Errorf("GetStep(3).Insight = %q, want %q", step3.Insight, steps[2].Insight)
	}
}

// --- JSON serialization tests ---

func TestOnboardingState_JSONSerialization(t *testing.T) {
	state := OnboardingState{
		UserID:    "user-1",
		Step:      2,
		StartedAt: 1700000000,
		DoneAt:    0,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("json.Marshal(OnboardingState): %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal into map: %v", err)
	}

	if result["user_id"] != "user-1" {
		t.Errorf("json user_id = %v, want user-1", result["user_id"])
	}
	if result["step"] != float64(2) {
		t.Errorf("json step = %v, want 2", result["step"])
	}
	if result["started_at"] != float64(1700000000) {
		t.Errorf("json started_at = %v, want 1700000000", result["started_at"])
	}
	// done_at with omitempty and zero value should be absent
	if _, ok := result["done_at"]; ok {
		t.Errorf("json done_at should be omitted when zero, got %v", result["done_at"])
	}
}

func TestOnboardingState_JSONSerialization_WithDoneAt(t *testing.T) {
	state := OnboardingState{
		UserID:    "user-2",
		Step:      4,
		StartedAt: 1700000000,
		DoneAt:    1700000100,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if result["done_at"] != float64(1700000100) {
		t.Errorf("json done_at = %v, want 1700000100", result["done_at"])
	}
}

func TestOnboardingState_JSONRoundTrip(t *testing.T) {
	original := OnboardingState{
		UserID:    "user-3",
		Step:      1,
		StartedAt: 1700000000,
		DoneAt:    1700000500,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded OnboardingState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.UserID != original.UserID {
		t.Errorf("UserID = %q, want %q", decoded.UserID, original.UserID)
	}
	if decoded.Step != original.Step {
		t.Errorf("Step = %d, want %d", decoded.Step, original.Step)
	}
	if decoded.StartedAt != original.StartedAt {
		t.Errorf("StartedAt = %d, want %d", decoded.StartedAt, original.StartedAt)
	}
	if decoded.DoneAt != original.DoneAt {
		t.Errorf("DoneAt = %d, want %d", decoded.DoneAt, original.DoneAt)
	}
}

func TestOnboardingStep_JSONSerialization(t *testing.T) {
	step := OnboardingStep{
		Step:        1,
		Title:       "Fix a Bug",
		Description: "See how SmartClaw learns your debugging pattern.",
		Prompt:      "Ask me to fix a simple bug",
		SkillName:   "bug-fix-workflow",
		Insight:     "I noticed your debugging pattern and created a skill.",
	}

	data, err := json.Marshal(step)
	if err != nil {
		t.Fatalf("json.Marshal(OnboardingStep): %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal into map: %v", err)
	}

	if result["step"] != float64(1) {
		t.Errorf("json step = %v, want 1", result["step"])
	}
	if result["title"] != "Fix a Bug" {
		t.Errorf("json title = %v, want 'Fix a Bug'", result["title"])
	}
	if result["description"] != "See how SmartClaw learns your debugging pattern." {
		t.Errorf("json description mismatch")
	}
	if result["prompt"] != "Ask me to fix a simple bug" {
		t.Errorf("json prompt mismatch")
	}
	if result["skill_name"] != "bug-fix-workflow" {
		t.Errorf("json skill_name = %v, want 'bug-fix-workflow'", result["skill_name"])
	}
	if result["insight"] != "I noticed your debugging pattern and created a skill." {
		t.Errorf("json insight mismatch")
	}
}

func TestOnboardingStep_JSONRoundTrip(t *testing.T) {
	original := GetStep(2)
	if original == nil {
		t.Fatal("GetStep(2) returned nil")
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded OnboardingStep
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.Step != original.Step {
		t.Errorf("Step = %d, want %d", decoded.Step, original.Step)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title = %q, want %q", decoded.Title, original.Title)
	}
	if decoded.Description != original.Description {
		t.Errorf("Description = %q, want %q", decoded.Description, original.Description)
	}
	if decoded.Prompt != original.Prompt {
		t.Errorf("Prompt = %q, want %q", decoded.Prompt, original.Prompt)
	}
	if decoded.SkillName != original.SkillName {
		t.Errorf("SkillName = %q, want %q", decoded.SkillName, original.SkillName)
	}
	if decoded.Insight != original.Insight {
		t.Errorf("Insight = %q, want %q", decoded.Insight, original.Insight)
	}
}

// --- Manager with nil store (no database) ---

func TestNewManager_NilStore(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager(nil) returned nil")
	}
}

func TestManager_GetState_NilStore_ReturnsDefault(t *testing.T) {
	m := NewManager(nil)
	state, err := m.GetState("user-1")
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state.UserID != "user-1" {
		t.Errorf("UserID = %q, want 'user-1'", state.UserID)
	}
	if state.Step != 0 {
		t.Errorf("Step = %d, want 0", state.Step)
	}
}

func TestManager_StartOnboarding_NilStore_ReturnsStep1(t *testing.T) {
	m := NewManager(nil)
	state, err := m.StartOnboarding("user-1")
	if err != nil {
		t.Fatalf("StartOnboarding: %v", err)
	}
	if state.UserID != "user-1" {
		t.Errorf("UserID = %q, want 'user-1'", state.UserID)
	}
	if state.Step != 1 {
		t.Errorf("Step = %d, want 1", state.Step)
	}
	if state.StartedAt == 0 {
		t.Error("StartedAt is zero, want non-zero")
	}
}

func TestManager_CompleteOnboarding_NilStore_ReturnsNil(t *testing.T) {
	m := NewManager(nil)
	err := m.CompleteOnboarding("user-1")
	if err != nil {
		t.Errorf("CompleteOnboarding with nil store should return nil, got: %v", err)
	}
}

func TestManager_AdvanceStep_NilStore_Step0_ReturnsNilStep(t *testing.T) {
	m := NewManager(nil)
	state, step, err := m.AdvanceStep("user-1", "skill")
	if err != nil {
		t.Fatalf("AdvanceStep: %v", err)
	}
	if state.Step != 0 {
		t.Errorf("Step = %d, want 0 (not started)", state.Step)
	}
	if step != nil {
		t.Errorf("step should be nil when not started, got %+v", step)
	}
}

func TestManager_AdvanceStep_NilStore_NoPersistence(t *testing.T) {
	m := NewManager(nil)
	// With nil store, GetState always returns Step=0, so AdvanceStep can't
	// progress since it thinks the user hasn't started.
	state, step, err := m.AdvanceStep("user-1", "skill")
	if err != nil {
		t.Fatalf("AdvanceStep: %v", err)
	}
	if state.Step != 0 {
		t.Errorf("Step = %d, want 0 (nil store can't persist)", state.Step)
	}
	if step != nil {
		t.Errorf("step should be nil with nil store, got %+v", step)
	}
}

// --- Manager with real SQLite store ---

func TestManager_GetState_RealStore_NoRows(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	state, err := m.GetState("nonexistent-user")
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state.UserID != "nonexistent-user" {
		t.Errorf("UserID = %q, want 'nonexistent-user'", state.UserID)
	}
	if state.Step != 0 {
		t.Errorf("Step = %d, want 0 for nonexistent user", state.Step)
	}
}

func TestManager_StartOnboarding_RealStore(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	state, err := m.StartOnboarding("user-1")
	if err != nil {
		t.Fatalf("StartOnboarding: %v", err)
	}
	if state.Step != 1 {
		t.Errorf("Step = %d, want 1", state.Step)
	}
	if state.StartedAt == 0 {
		t.Error("StartedAt is zero, want non-zero")
	}

	// Verify persisted
	persisted, err := m.GetState("user-1")
	if err != nil {
		t.Fatalf("GetState after start: %v", err)
	}
	if persisted.Step != 1 {
		t.Errorf("Persisted Step = %d, want 1", persisted.Step)
	}
	if persisted.StartedAt == 0 {
		t.Error("Persisted StartedAt is zero")
	}
}

func TestManager_StartOnboarding_Idempotent(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	// Start once
	state1, err := m.StartOnboarding("user-1")
	if err != nil {
		t.Fatalf("StartOnboarding 1: %v", err)
	}

	// Advance to step 2
	m.AdvanceStep("user-1", "skill1")

	// Start again — should reset to step 1
	state2, err := m.StartOnboarding("user-1")
	if err != nil {
		t.Fatalf("StartOnboarding 2: %v", err)
	}
	if state2.Step != 1 {
		t.Errorf("After re-start, Step = %d, want 1", state2.Step)
	}

	// Verify done_at was reset
	persisted, _ := m.GetState("user-1")
	if persisted.DoneAt != 0 {
		t.Errorf("After re-start, DoneAt = %d, want 0", persisted.DoneAt)
	}

	// Verify started_at was updated
	if state2.StartedAt < state1.StartedAt {
		t.Errorf("After re-start, StartedAt = %d, should be >= first StartedAt = %d",
			state2.StartedAt, state1.StartedAt)
	}
}

func TestManager_AdvanceStep_RealStore_1to4(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	// Start
	state, err := m.StartOnboarding("user-1")
	if err != nil {
		t.Fatalf("StartOnboarding: %v", err)
	}
	if state.Step != 1 {
		t.Fatalf("After start, Step = %d, want 1", state.Step)
	}

	// Advance 1→2
	state, step, err := m.AdvanceStep("user-1", "skill1")
	if err != nil {
		t.Fatalf("AdvanceStep 1→2: %v", err)
	}
	if state.Step != 2 {
		t.Errorf("After advance 1→2, Step = %d, want 2", state.Step)
	}
	if step == nil {
		t.Fatal("AdvanceStep 1→2 returned nil step, want step 2")
	}
	if step.Step != 2 {
		t.Errorf("Returned step.Step = %d, want 2", step.Step)
	}

	// Advance 2→3
	state, step, err = m.AdvanceStep("user-1", "skill2")
	if err != nil {
		t.Fatalf("AdvanceStep 2→3: %v", err)
	}
	if state.Step != 3 {
		t.Errorf("After advance 2→3, Step = %d, want 3", state.Step)
	}
	if step == nil {
		t.Fatal("AdvanceStep 2→3 returned nil step, want step 3")
	}
	if step.Step != 3 {
		t.Errorf("Returned step.Step = %d, want 3", step.Step)
	}

	// Advance 3→4 (completed)
	state, step, err = m.AdvanceStep("user-1", "skill3")
	if err != nil {
		t.Fatalf("AdvanceStep 3→4: %v", err)
	}
	if state.Step != 4 {
		t.Errorf("After advance 3→4, Step = %d, want 4", state.Step)
	}
	if step != nil {
		t.Errorf("AdvanceStep 3→4 should return nil step (completed), got %+v", step)
	}
	if state.DoneAt == 0 {
		t.Error("After completion, DoneAt is zero, want non-zero")
	}
}

func TestManager_AdvanceStep_RealStore_CannotAdvanceAtStep0(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	// User has not started — step 0
	state, step, err := m.AdvanceStep("user-1", "skill")
	if err != nil {
		t.Fatalf("AdvanceStep: %v", err)
	}
	if state.Step != 0 {
		t.Errorf("Step = %d, want 0 (not started)", state.Step)
	}
	if step != nil {
		t.Errorf("step should be nil when not started, got %+v", step)
	}
}

func TestManager_AdvanceStep_RealStore_CannotAdvancePastCompleted(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	// Complete the onboarding
	m.StartOnboarding("user-1")
	m.AdvanceStep("user-1", "skill1")
	m.AdvanceStep("user-1", "skill2")
	m.AdvanceStep("user-1", "skill3") // step 4

	// Try to advance from step 4
	state, step, err := m.AdvanceStep("user-1", "skill4")
	if err != nil {
		t.Fatalf("AdvanceStep from 4: %v", err)
	}
	if state.Step != 4 {
		t.Errorf("Step = %d, want 4 (already completed)", state.Step)
	}
	if step != nil {
		t.Errorf("step should be nil when completed, got %+v", step)
	}
}

func TestManager_CompleteOnboarding_RealStore(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	// Start and advance partially
	m.StartOnboarding("user-1")
	m.AdvanceStep("user-1", "skill1")

	// Complete directly
	err := m.CompleteOnboarding("user-1")
	if err != nil {
		t.Fatalf("CompleteOnboarding: %v", err)
	}

	// Verify state
	state, err := m.GetState("user-1")
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if state.Step != 4 {
		t.Errorf("Step = %d, want 4", state.Step)
	}
	if state.DoneAt == 0 {
		t.Error("DoneAt is zero, want non-zero")
	}
}

func TestManager_CompleteOnboarding_RealStore_StartedAtPreserved(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	startState, err := m.StartOnboarding("user-1")
	if err != nil {
		t.Fatalf("StartOnboarding: %v", err)
	}
	originalStartedAt := startState.StartedAt

	err = m.CompleteOnboarding("user-1")
	if err != nil {
		t.Fatalf("CompleteOnboarding: %v", err)
	}

	state, _ := m.GetState("user-1")
	if state.StartedAt != originalStartedAt {
		t.Errorf("StartedAt changed after complete: got %d, want %d",
			state.StartedAt, originalStartedAt)
	}
}

func TestManager_AdvanceStep_ReturnsCorrectStepObjects(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	m.StartOnboarding("user-1")

	// Advance 1→2 should return step 2 info
	_, step2, err := m.AdvanceStep("user-1", "skill1")
	if err != nil {
		t.Fatalf("AdvanceStep 1→2: %v", err)
	}
	expected2 := GetStep(2)
	if step2.Title != expected2.Title {
		t.Errorf("Step 2 Title = %q, want %q", step2.Title, expected2.Title)
	}
	if step2.SkillName != expected2.SkillName {
		t.Errorf("Step 2 SkillName = %q, want %q", step2.SkillName, expected2.SkillName)
	}

	// Advance 2→3 should return step 3 info
	_, step3, err := m.AdvanceStep("user-1", "skill2")
	if err != nil {
		t.Fatalf("AdvanceStep 2→3: %v", err)
	}
	expected3 := GetStep(3)
	if step3.Title != expected3.Title {
		t.Errorf("Step 3 Title = %q, want %q", step3.Title, expected3.Title)
	}
}

func TestManager_MultipleUsers_Isolated(t *testing.T) {
	s := newTestStore(t)
	m := NewManager(s)

	m.StartOnboarding("user-a")
	m.StartOnboarding("user-b")

	m.AdvanceStep("user-a", "skill1") // user-a at step 2

	stateA, _ := m.GetState("user-a")
	stateB, _ := m.GetState("user-b")

	if stateA.Step != 2 {
		t.Errorf("user-a Step = %d, want 2", stateA.Step)
	}
	if stateB.Step != 1 {
		t.Errorf("user-b Step = %d, want 1", stateB.Step)
	}
}
