package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

// --- NewProfileUpdater ---

func TestNewProfileUpdater(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)

	updater := NewProfileUpdater(engine, nil, tracker)
	if updater == nil {
		t.Fatal("expected non-nil updater")
	}
	if updater.engine != engine {
		t.Fatal("expected engine to be set")
	}
	if updater.tracker != tracker {
		t.Fatal("expected tracker to be set")
	}
	if updater.UpdateThreshold != 5 {
		t.Fatalf("expected default UpdateThreshold 5, got %d", updater.UpdateThreshold)
	}
}

func TestNewProfileUpdaterAllNil(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	if updater == nil {
		t.Fatal("expected non-nil updater even with nil components")
	}
}

// --- UpdateProfile ---

func TestUpdateProfileNilEngine(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	err := updater.UpdateProfile(context.Background())
	if err == nil {
		t.Fatal("expected error with nil engine")
	}
	if !strings.Contains(err.Error(), "engine not available") {
		t.Fatalf("expected engine-not-available error, got %v", err)
	}
}

func TestUpdateProfileNilStore(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")
	updater := NewProfileUpdater(engine, nil, nil)
	err := updater.UpdateProfile(context.Background())
	if err == nil {
		t.Fatal("expected error with nil store in engine")
	}
	if !strings.Contains(err.Error(), "store not available") {
		t.Fatalf("expected store-not-available error, got %v", err)
	}
}

func TestUpdateProfileEmptyObservations(t *testing.T) {
	s := newTestStoreForModeling(t)
	pm := newTestPromptMemory(t)
	engine := NewUserModelingEngine(s, pm, "user-1")
	tracker := NewPreferenceTracker(engine)
	updater := NewProfileUpdater(engine, pm, tracker)

	err := updater.UpdateProfile(context.Background())
	if err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}

	userContent := pm.GetUserContent()
	if userContent == "" {
		t.Fatal("expected non-empty user profile after update")
	}
}

func TestUpdateProfileWithObservations(t *testing.T) {
	s := newTestStoreForModeling(t)
	pm := newTestPromptMemory(t)
	engine := NewUserModelingEngine(s, pm, "user-1")
	tracker := NewPreferenceTracker(engine)
	updater := NewProfileUpdater(engine, pm, tracker)
	ctx := context.Background()

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

	err := updater.UpdateProfile(ctx)
	if err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}

	userContent := pm.GetUserContent()
	if userContent == "" {
		t.Fatal("expected non-empty user profile after update")
	}
	if !strings.Contains(userContent, "Code Style") {
		t.Fatal("expected Code Style section in profile")
	}
	if !strings.Contains(userContent, "tabs") {
		t.Fatal("expected tabs preference in profile")
	}
}

func TestUpdateProfileNilPromptMem(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	updater := NewProfileUpdater(engine, nil, tracker)

	err := updater.UpdateProfile(context.Background())
	if err != nil {
		t.Fatalf("UpdateProfile with nil promptMem should not error, got %v", err)
	}
}

func TestUpdateProfileTrimsToBudget(t *testing.T) {
	s := newTestStoreForModeling(t)
	pm := newTestPromptMemory(t)
	engine := NewUserModelingEngine(s, pm, "user-1")
	tracker := NewPreferenceTracker(engine)
	updater := NewProfileUpdater(engine, pm, tracker)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		if err := engine.RecordObservation(ctx, "knowledge", "lang", "Skill-"+string(rune('A'+i%26))+"-detailed-description-here", 0.8, "sess-1"); err != nil {
			t.Fatalf("RecordObservation error: %v", err)
		}
	}

	err := updater.UpdateProfile(ctx)
	if err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}

	userContent := pm.GetUserContent()
	if len(userContent) > layers.MaxPromptMemoryChars {
		t.Fatalf("expected user content within budget (%d chars), got %d chars", layers.MaxPromptMemoryChars, len(userContent))
	}
}

// --- ShouldUpdate ---

func TestShouldUpdateBelowThreshold(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	updater.UpdateThreshold = 5

	if updater.ShouldUpdate(4) {
		t.Fatal("expected ShouldUpdate(4) to be false with threshold 5")
	}
	if updater.ShouldUpdate(0) {
		t.Fatal("expected ShouldUpdate(0) to be false")
	}
}

func TestShouldUpdateAtThreshold(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	updater.UpdateThreshold = 5

	if !updater.ShouldUpdate(5) {
		t.Fatal("expected ShouldUpdate(5) to be true with threshold 5")
	}
}

func TestShouldUpdateAboveThreshold(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	updater.UpdateThreshold = 5

	if !updater.ShouldUpdate(10) {
		t.Fatal("expected ShouldUpdate(10) to be true with threshold 5")
	}
}

func TestShouldUpdateCustomThreshold(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	updater.UpdateThreshold = 3

	if updater.ShouldUpdate(2) {
		t.Fatal("expected ShouldUpdate(2) to be false with threshold 3")
	}
	if !updater.ShouldUpdate(3) {
		t.Fatal("expected ShouldUpdate(3) to be true with threshold 3")
	}
}

// --- FormatProfile ---

func TestFormatProfileNil(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	result := updater.FormatProfile(nil)
	if result != "" {
		t.Fatalf("expected empty string for nil snapshot, got %q", result)
	}
}

func TestFormatProfileEmptySnapshot(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: make(map[string]string),
	}
	result := updater.FormatProfile(snapshot)
	if result == "" {
		t.Fatal("expected non-empty result for empty snapshot (should have header)")
	}
	if !strings.Contains(result, "# User Profile") {
		t.Fatal("expected User Profile header")
	}
}

func TestFormatProfileCodeStylePreferences(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{
			"indentation": "tabs",
			"naming":      "camelCase",
			"language":    "go",
		},
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "## Code Style") {
		t.Fatal("expected Code Style section")
	}
	if !strings.Contains(result, "indentation: tabs") {
		t.Fatal("expected indentation: tabs in output")
	}
	if !strings.Contains(result, "naming: camelCase") {
		t.Fatal("expected naming: camelCase in output")
	}
	if !strings.Contains(result, "language: go") {
		t.Fatal("expected language: go in output")
	}
}

func TestFormatProfileCommunicationStyle(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		CommunicationStyle: "concise",
		Preferences:        map[string]string{},
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "## Communication Preferences") {
		t.Fatal("expected Communication Preferences section")
	}
	if !strings.Contains(result, "style: concise") {
		t.Fatal("expected style: concise in output")
	}
}

func TestFormatProfileOtherPreferences(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{
			"editor":    "vim",
			"framework": "react",
		},
		CommunicationStyle: "brief",
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "editor: vim") {
		t.Fatal("expected editor: vim in output")
	}
	if !strings.Contains(result, "framework: react") {
		t.Fatal("expected framework: react in output")
	}
}

func TestFormatProfileWorkPatterns(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{},
		TopPatterns: []layers.WorkPattern{
			{Pattern: "edit-test-commit", Frequency: 5, LastSeen: time.Now()},
			{Pattern: "search-then-fix", Frequency: 3, LastSeen: time.Now()},
		},
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "## Work Patterns") {
		t.Fatal("expected Work Patterns section")
	}
	if !strings.Contains(result, "edit-test-commit (freq: 5)") {
		t.Fatal("expected pattern with frequency in output")
	}
	if !strings.Contains(result, "search-then-fix (freq: 3)") {
		t.Fatal("expected second pattern with frequency in output")
	}
}

func TestFormatProfileKnowledge(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences:         map[string]string{},
		KnowledgeBackground: []string{"Go", "Docker", "Kubernetes"},
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "## Knowledge") {
		t.Fatal("expected Knowledge section")
	}
	if !strings.Contains(result, "- Go\n") {
		t.Fatal("expected - Go in knowledge section")
	}
	if !strings.Contains(result, "- Docker\n") {
		t.Fatal("expected - Docker in knowledge section")
	}
}

func TestFormatProfileUnresolvedConflicts(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{},
		Conflicts: []ObservationConflict{
			{
				Category:   "code_style",
				Key:        "indentation",
				Thesis:     "tabs",
				Antithesis: "spaces",
				Resolved:   false,
			},
		},
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "## Unresolved Conflicts") {
		t.Fatal("expected Unresolved Conflicts section")
	}
	if !strings.Contains(result, "code_style/indentation") {
		t.Fatal("expected conflict category/key in output")
	}
}

func TestFormatProfileResolvedConflictsHidden(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{},
		Conflicts: []ObservationConflict{
			{
				Category:   "code_style",
				Key:        "indentation",
				Thesis:     "tabs",
				Antithesis: "spaces",
				Resolved:   true,
			},
		},
	}

	result := updater.FormatProfile(snapshot)
	if strings.Contains(result, "Unresolved Conflicts") {
		t.Fatal("expected no Unresolved Conflicts section for resolved conflicts")
	}
}

func TestFormatProfileCodeStyleKeysSeparated(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{
			"indentation": "tabs",
			"editor":      "vim",
		},
		CommunicationStyle: "brief",
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "## Code Style") {
		t.Fatal("expected Code Style section")
	}
	if !strings.Contains(result, "## Communication Preferences") {
		t.Fatal("expected Communication Preferences section")
	}
	if !strings.Contains(result, "editor: vim") {
		t.Fatal("expected editor: vim in Communication Preferences (non-code-style key)")
	}
}

func TestFormatProfileFullSnapshot(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{
			"indentation": "tabs",
			"naming":      "camelCase",
			"editor":      "vim",
		},
		CommunicationStyle:  "concise",
		KnowledgeBackground: []string{"Go", "Docker"},
		TopPatterns: []layers.WorkPattern{
			{Pattern: "edit-test-commit", Frequency: 5, LastSeen: time.Now()},
		},
		Conflicts: []ObservationConflict{
			{Category: "code_style", Key: "indentation", Thesis: "tabs", Antithesis: "spaces", Resolved: false},
		},
		LastUpdated: time.Now(),
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "# User Profile") {
		t.Fatal("expected User Profile header")
	}
	if !strings.Contains(result, "## Code Style") {
		t.Fatal("expected Code Style section")
	}
	if !strings.Contains(result, "## Communication Preferences") {
		t.Fatal("expected Communication Preferences section")
	}
	if !strings.Contains(result, "## Work Patterns") {
		t.Fatal("expected Work Patterns section")
	}
	if !strings.Contains(result, "## Knowledge") {
		t.Fatal("expected Knowledge section")
	}
	if !strings.Contains(result, "## Unresolved Conflicts") {
		t.Fatal("expected Unresolved Conflicts section")
	}
}

// --- TrimToBudget ---

func TestTrimToBudgetWithinBudget(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := "short content"
	result := updater.TrimToBudget(content, 100)
	if result != content {
		t.Fatalf("expected unchanged content, got %q", result)
	}
}

func TestTrimToBudgetExactBudget(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := strings.Repeat("x", 50)
	result := updater.TrimToBudget(content, 50)
	if len(result) != 50 {
		t.Fatalf("expected exactly 50 chars, got %d", len(result))
	}
}

func TestTrimToBudgetOverBudget(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := "line1\nline2\nline3\nline4\nline5\n"
	result := updater.TrimToBudget(content, 20)
	if len(result) > 20 {
		t.Fatalf("expected at most 20 chars, got %d", len(result))
	}
}

func TestTrimToBudgetZeroBudget(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := "some content"
	result := updater.TrimToBudget(content, 0)
	if len(result) > 0 {
		t.Fatalf("expected empty result for zero budget, got %d chars", len(result))
	}
}

func TestTrimToBudgetRemovesTrailingEmptyLines(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := "line1\n\n\n"
	result := updater.TrimToBudget(content, 6)
	if strings.HasSuffix(result, "\n") {
		t.Fatalf("expected no trailing newlines, got %q", result)
	}
}

func TestTrimToBudgetRemovesFromEnd(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := "## Header\n- item1\n- item2\n- item3\n"
	result := updater.TrimToBudget(content, 20)
	if !strings.Contains(result, "## Header") {
		t.Fatal("expected header preserved when trimming from end")
	}
}

func TestTrimToBudgetSingleLongLine(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := strings.Repeat("x", 200)
	result := updater.TrimToBudget(content, 50)
	if len(result) > 50 {
		t.Fatalf("expected at most 50 chars, got %d", len(result))
	}
}

func TestTrimToBudgetWithMaxPromptMemoryChars(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := strings.Repeat("x\n", 2000)
	result := updater.TrimToBudget(content, layers.MaxPromptMemoryChars)
	if len(result) > layers.MaxPromptMemoryChars {
		t.Fatalf("expected at most %d chars, got %d", layers.MaxPromptMemoryChars, len(result))
	}
}

// --- formatKV ---

func TestFormatKV(t *testing.T) {
	result := formatKV("key", "value")
	expected := "- key: value\n"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatKVEmptyValue(t *testing.T) {
	result := formatKV("key", "")
	expected := "- key: \n"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatKVEmptyKey(t *testing.T) {
	result := formatKV("", "value")
	expected := "- : value\n"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

func TestFormatKVSpecialChars(t *testing.T) {
	result := formatKV("lang", "go<>&")
	expected := "- lang: go<>&\n"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}

// --- Integration ---

func TestProfileUpdaterIntegrationFlow(t *testing.T) {
	s := newTestStoreForModeling(t)
	pm := newTestPromptMemory(t)
	engine := NewUserModelingEngine(s, pm, "user-1")
	tracker := NewPreferenceTracker(engine)
	updater := NewProfileUpdater(engine, pm, tracker)
	ctx := context.Background()

	if err := tracker.ObserveCodeStyle(ctx, "sess-1", "func main() {\n\tfmt.Println()\n}"); err != nil {
		t.Fatalf("ObserveCodeStyle error: %v", err)
	}
	if err := tracker.ObserveCommunication(ctx, "sess-1", "Fix the bug and implement the feature now"); err != nil {
		t.Fatalf("ObserveCommunication error: %v", err)
	}

	tracker.WorkflowThreshold = 2
	for i := 0; i < 2; i++ {
		if err := tracker.ObserveWorkflow(ctx, "sess-1", "bash", map[string]any{"command": "go test"}); err != nil {
			t.Fatalf("ObserveWorkflow error: %v", err)
		}
	}

	err := updater.UpdateProfile(ctx)
	if err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}

	userContent := pm.GetUserContent()
	if userContent == "" {
		t.Fatal("expected non-empty USER.md after profile update")
	}
	if !strings.Contains(userContent, "# User Profile") {
		t.Fatal("expected User Profile header in USER.md")
	}
}

func TestProfileUpdaterShouldUpdateDecision(t *testing.T) {
	s := newTestStoreForModeling(t)
	pm := newTestPromptMemory(t)
	engine := NewUserModelingEngine(s, pm, "user-1")
	tracker := NewPreferenceTracker(engine)
	updater := NewProfileUpdater(engine, pm, tracker)

	if updater.ShouldUpdate(4) {
		t.Fatal("should not update below threshold")
	}
	if !updater.ShouldUpdate(5) {
		t.Fatal("should update at threshold")
	}
	if !updater.ShouldUpdate(10) {
		t.Fatal("should update above threshold")
	}
}

func TestProfileUpdaterWithSpecificCodeStyleKeys(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{
			"indentation": "spaces:2",
			"naming":      "snake_case",
			"language":    "python",
			"framework":   "django",
			"editor":      "vscode",
		},
		CommunicationStyle: "verbose",
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "## Code Style") {
		t.Fatal("expected Code Style section")
	}
	if !strings.Contains(result, "indentation: spaces:2") {
		t.Fatal("expected indentation in Code Style")
	}
	if !strings.Contains(result, "language: python") {
		t.Fatal("expected language in Code Style")
	}
	if !strings.Contains(result, "framework: django") {
		t.Fatal("expected framework in Code Style")
	}
	if !strings.Contains(result, "editor: vscode") {
		t.Fatal("expected editor in Code Style")
	}
	if !strings.Contains(result, "## Communication Preferences") {
		t.Fatal("expected Communication Preferences section")
	}
	if !strings.Contains(result, "style: verbose") {
		t.Fatal("expected style in Communication Preferences")
	}
}

func TestProfileUpdaterMultiplePatterns(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	snapshot := &UserModelSnapshot{
		Preferences: map[string]string{},
		TopPatterns: []layers.WorkPattern{
			{Pattern: "edit-test-commit", Frequency: 10, LastSeen: time.Now()},
			{Pattern: "search-fix-verify", Frequency: 7, LastSeen: time.Now()},
			{Pattern: "read-understand-write", Frequency: 3, LastSeen: time.Now()},
		},
	}

	result := updater.FormatProfile(snapshot)
	if !strings.Contains(result, "edit-test-commit (freq: 10)") {
		t.Fatal("expected first pattern")
	}
	if !strings.Contains(result, "search-fix-verify (freq: 7)") {
		t.Fatal("expected second pattern")
	}
	if !strings.Contains(result, "read-understand-write (freq: 3)") {
		t.Fatal("expected third pattern")
	}
}

func TestTrimToBudgetPreservesContentWhenSmall(t *testing.T) {
	updater := NewProfileUpdater(nil, nil, nil)
	content := "# User Profile\n\n## Code Style\n- indentation: tabs\n"
	result := updater.TrimToBudget(content, layers.MaxPromptMemoryChars)
	if result != content {
		t.Fatalf("expected content unchanged when within budget")
	}
}

func TestProfileUpdaterWithRealStoreAndPromptMemory(t *testing.T) {
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir: %v", err)
	}
	defer s.Close()

	pm, err := layers.NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	engine := NewUserModelingEngine(s, pm, "user-1")
	tracker := NewPreferenceTracker(engine)
	updater := NewProfileUpdater(engine, pm, tracker)
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "code_style", "language", "go", 0.9, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "knowledge", "language", "Go", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	err = updater.UpdateProfile(ctx)
	if err != nil {
		t.Fatalf("UpdateProfile error: %v", err)
	}

	userContent := pm.GetUserContent()
	if userContent == "" {
		t.Fatal("expected non-empty user content")
	}
}
