package memory

import (
	"context"
	"strings"
	"testing"
)

// --- NewPreferenceTracker ---

func TestNewPreferenceTracker(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")

	tracker := NewPreferenceTracker(engine)
	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}
	if tracker.engine != engine {
		t.Fatal("expected engine to be set")
	}
	if tracker.WorkflowThreshold != 3 {
		t.Fatalf("expected default WorkflowThreshold 3, got %d", tracker.WorkflowThreshold)
	}
	if len(tracker.sessionPatterns) != 0 {
		t.Fatalf("expected empty sessionPatterns, got %d entries", len(tracker.sessionPatterns))
	}
}

func TestNewPreferenceTrackerNilEngine(t *testing.T) {
	tracker := NewPreferenceTracker(nil)
	if tracker == nil {
		t.Fatal("expected non-nil tracker even with nil engine")
	}
}

// --- ObserveCodeStyle ---

func TestObserveCodeStyleNilEngine(t *testing.T) {
	tracker := NewPreferenceTracker(nil)
	err := tracker.ObserveCodeStyle(context.Background(), "sess-1", "func main() {}")
	if err != nil {
		t.Fatalf("expected nil error with nil engine, got %v", err)
	}
}

func TestObserveCodeStyleEmptySnippet(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)

	err := tracker.ObserveCodeStyle(context.Background(), "sess-1", "")
	if err != nil {
		t.Fatalf("expected nil error with empty snippet, got %v", err)
	}
}

func TestObserveCodeStyleGoCode(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	code := `func main() {
	msg := "hello"
	fmt.Println(msg)
}`

	err := tracker.ObserveCodeStyle(ctx, "sess-1", code)
	if err != nil {
		t.Fatalf("ObserveCodeStyle error: %v", err)
	}

	// Verify observations were recorded
	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = 'code_style'").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least 1 code_style observation")
	}
}

func TestObserveCodeStylePythonCode(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	code := `def main():
    print("hello")

if __name__ == "__main__":
    main()
`

	err := tracker.ObserveCodeStyle(ctx, "sess-1", code)
	if err != nil {
		t.Fatalf("ObserveCodeStyle error: %v", err)
	}

	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = 'code_style'").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least 1 code_style observation for Python code")
	}
}

func TestObserveCodeStyleTabsVsSpaces(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	tabCode := `func main() {
	msg := "hello"
	fmt.Println(msg)
}`

	err := tracker.ObserveCodeStyle(ctx, "sess-1", tabCode)
	if err != nil {
		t.Fatalf("ObserveCodeStyle error: %v", err)
	}

	var indentValue string
	err = s.DB().QueryRow("SELECT value FROM user_observations WHERE category = 'code_style' AND key = 'indentation'").Scan(&indentValue)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if indentValue != "tabs" {
		t.Fatalf("expected indentation value %q, got %q", "tabs", indentValue)
	}
}

// --- ObserveCommunication ---

func TestObserveCommunicationNilEngine(t *testing.T) {
	tracker := NewPreferenceTracker(nil)
	err := tracker.ObserveCommunication(context.Background(), "sess-1", "Hello")
	if err != nil {
		t.Fatalf("expected nil error with nil engine, got %v", err)
	}
}

func TestObserveCommunicationEmptyMessage(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)

	err := tracker.ObserveCommunication(context.Background(), "sess-1", "")
	if err != nil {
		t.Fatalf("expected nil error with empty message, got %v", err)
	}
}

func TestObserveCommunicationBriefMessage(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	err := tracker.ObserveCommunication(ctx, "sess-1", "Fix it")
	if err != nil {
		t.Fatalf("ObserveCommunication error: %v", err)
	}

	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = 'communication_style'").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least 1 communication_style observation")
	}

	var verbosity string
	err = s.DB().QueryRow("SELECT value FROM user_observations WHERE category = 'communication_style' AND key = 'verbosity'").Scan(&verbosity)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if verbosity != "brief" {
		t.Fatalf("expected verbosity %q, got %q", "brief", verbosity)
	}
}

func TestObserveCommunicationVerboseMessage(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	longMsg := strings.Repeat("word ", 60)
	err := tracker.ObserveCommunication(ctx, "sess-1", longMsg)
	if err != nil {
		t.Fatalf("ObserveCommunication error: %v", err)
	}

	var verbosity string
	err = s.DB().QueryRow("SELECT value FROM user_observations WHERE category = 'communication_style' AND key = 'verbosity'").Scan(&verbosity)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if verbosity != "verbose" {
		t.Fatalf("expected verbosity %q, got %q", "verbose", verbosity)
	}
}

func TestObserveCommunicationMarkdownFormat(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	msg := "## Header\n\nHere is some `code` and **bold** text."
	err := tracker.ObserveCommunication(ctx, "sess-1", msg)
	if err != nil {
		t.Fatalf("ObserveCommunication error: %v", err)
	}

	var formatValue string
	err = s.DB().QueryRow("SELECT value FROM user_observations WHERE category = 'communication_style' AND key = 'format'").Scan(&formatValue)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if formatValue != "markdown" {
		t.Fatalf("expected format %q, got %q", "markdown", formatValue)
	}
}

func TestObserveCommunicationDirectiveStyle(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	msg := "Fix this bug and implement the feature now"
	err := tracker.ObserveCommunication(ctx, "sess-1", msg)
	if err != nil {
		t.Fatalf("ObserveCommunication error: %v", err)
	}

	var interactionStyle string
	err = s.DB().QueryRow("SELECT value FROM user_observations WHERE category = 'communication_style' AND key = 'interaction_style'").Scan(&interactionStyle)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if interactionStyle != "directive" {
		t.Fatalf("expected interaction_style %q, got %q", "directive", interactionStyle)
	}
}

func TestObserveCommunicationExploratoryStyle(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	msg := "Maybe we could explore using a different approach? How about trying the cache?"
	err := tracker.ObserveCommunication(ctx, "sess-1", msg)
	if err != nil {
		t.Fatalf("ObserveCommunication error: %v", err)
	}

	var interactionStyle string
	err = s.DB().QueryRow("SELECT value FROM user_observations WHERE category = 'communication_style' AND key = 'interaction_style'").Scan(&interactionStyle)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if interactionStyle != "exploratory" {
		t.Fatalf("expected interaction_style %q, got %q", "exploratory", interactionStyle)
	}
}

func TestObserveCommunicationInquisitive(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	msg := "What about this? Can we do that? How does this work???"
	err := tracker.ObserveCommunication(ctx, "sess-1", msg)
	if err != nil {
		t.Fatalf("ObserveCommunication error: %v", err)
	}

	var questionStyle string
	err = s.DB().QueryRow("SELECT value FROM user_observations WHERE category = 'communication_style' AND key = 'question_style'").Scan(&questionStyle)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if questionStyle != "inquisitive" {
		t.Fatalf("expected question_style %q, got %q", "inquisitive", questionStyle)
	}
}

// --- ObserveWorkflow ---

func TestObserveWorkflowNilEngine(t *testing.T) {
	tracker := NewPreferenceTracker(nil)
	err := tracker.ObserveWorkflow(context.Background(), "sess-1", "bash", nil)
	if err != nil {
		t.Fatalf("expected nil error with nil engine, got %v", err)
	}
}

func TestObserveWorkflowEmptyToolName(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)

	err := tracker.ObserveWorkflow(context.Background(), "sess-1", "", nil)
	if err != nil {
		t.Fatalf("expected nil error with empty tool name, got %v", err)
	}
}

func TestObserveWorkflowBelowThreshold(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	// Below default threshold of 3
	for i := 0; i < 2; i++ {
		err := tracker.ObserveWorkflow(ctx, "sess-1", "bash", map[string]any{"command": "ls"})
		if err != nil {
			t.Fatalf("ObserveWorkflow error: %v", err)
		}
	}

	var count int
	err := s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = 'workflow_pattern'").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 observations below threshold, got %d", count)
	}
}

func TestObserveWorkflowAtThreshold(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	tracker.WorkflowThreshold = 3
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := tracker.ObserveWorkflow(ctx, "sess-1", "bash", map[string]any{"command": "ls"})
		if err != nil {
			t.Fatalf("ObserveWorkflow %d error: %v", i, err)
		}
	}

	var count int
	err := s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = 'workflow_pattern'").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 observation at threshold, got %d", count)
	}
}

func TestObserveWorkflowSentinelPreventsReRecording(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	tracker.WorkflowThreshold = 2
	ctx := context.Background()

	// Reach threshold
	for i := 0; i < 2; i++ {
		err := tracker.ObserveWorkflow(ctx, "sess-1", "bash", map[string]any{"command": "ls"})
		if err != nil {
			t.Fatalf("ObserveWorkflow %d error: %v", i, err)
		}
	}

	// Call more times — sentinel should prevent re-recording
	for i := 0; i < 5; i++ {
		err := tracker.ObserveWorkflow(ctx, "sess-1", "bash", map[string]any{"command": "ls"})
		if err != nil {
			t.Fatalf("ObserveWorkflow post-threshold %d error: %v", i, err)
		}
	}

	var count int
	err := s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = 'workflow_pattern'").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 observation (sentinel prevents re-recording), got %d", count)
	}
}

func TestObserveWorkflowDifferentPatternsTrackedSeparately(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	tracker.WorkflowThreshold = 2
	ctx := context.Background()

	// Pattern A reaches threshold
	for i := 0; i < 2; i++ {
		err := tracker.ObserveWorkflow(ctx, "sess-1", "bash", map[string]any{"command": "ls"})
		if err != nil {
			t.Fatalf("ObserveWorkflow A %d error: %v", i, err)
		}
	}

	// Pattern B below threshold
	err := tracker.ObserveWorkflow(ctx, "sess-1", "read_file", map[string]any{"path": "/tmp/test"})
	if err != nil {
		t.Fatalf("ObserveWorkflow B error: %v", err)
	}

	var count int
	err = s.DB().QueryRow("SELECT COUNT(*) FROM user_observations WHERE category = 'workflow_pattern'").Scan(&count)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 observation (only pattern A reached threshold), got %d", count)
	}
}

// --- GetCurrentPreferences ---

func TestGetCurrentPreferencesNoSnapshot(t *testing.T) {
	engine := NewUserModelingEngine(nil, nil, "user-1")
	tracker := NewPreferenceTracker(engine)

	prefs := tracker.GetCurrentPreferences()
	if prefs == nil {
		t.Fatal("expected non-nil PreferenceSnapshot")
	}
	if len(prefs.CodeStyle) != 0 {
		t.Fatalf("expected empty CodeStyle, got %d entries", len(prefs.CodeStyle))
	}
	if len(prefs.CommunicationStyle) != 0 {
		t.Fatalf("expected empty CommunicationStyle, got %d entries", len(prefs.CommunicationStyle))
	}
	if len(prefs.WorkflowPatterns) != 0 {
		t.Fatalf("expected empty WorkflowPatterns, got %d entries", len(prefs.WorkflowPatterns))
	}
}

func TestGetCurrentPreferencesWithSnapshot(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if err := engine.RecordObservation(ctx, "communication_style", "style", "concise", 0.7, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}

	if _, err := engine.SynthesizeModel(ctx); err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	prefs := tracker.GetCurrentPreferences()
	if len(prefs.CodeStyle) == 0 {
		t.Fatal("expected non-empty CodeStyle")
	}
	if prefs.CodeStyle["indentation"] != "tabs" {
		t.Fatalf("expected indentation=tabs, got %q", prefs.CodeStyle["indentation"])
	}
	if len(prefs.CommunicationStyle) == 0 {
		t.Fatal("expected non-empty CommunicationStyle")
	}
	if prefs.CommunicationStyle["style"] != "concise" {
		t.Fatalf("expected style=concise, got %q", prefs.CommunicationStyle["style"])
	}
}

func TestGetCurrentPreferencesIncludesSessionPatterns(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if _, err := engine.SynthesizeModel(ctx); err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	tracker := NewPreferenceTracker(engine)

	tracker.mu.Lock()
	tracker.sessionPatterns["bash:command=ls"] = 2
	tracker.mu.Unlock()

	prefs := tracker.GetCurrentPreferences()
	found := false
	for _, p := range prefs.WorkflowPatterns {
		if p == "bash:command=ls" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected session pattern to appear in WorkflowPatterns")
	}
}

func TestGetCurrentPreferencesExcludesSentinelPatterns(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	ctx := context.Background()

	if err := engine.RecordObservation(ctx, "code_style", "indentation", "tabs", 0.8, "sess-1"); err != nil {
		t.Fatalf("RecordObservation error: %v", err)
	}
	if _, err := engine.SynthesizeModel(ctx); err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	tracker := NewPreferenceTracker(engine)

	tracker.mu.Lock()
	tracker.sessionPatterns["bash:command=ls"] = -999
	tracker.sessionPatterns["read_file:path=/test"] = 1
	tracker.mu.Unlock()

	prefs := tracker.GetCurrentPreferences()
	for _, p := range prefs.WorkflowPatterns {
		if p == "bash:command=ls" {
			t.Fatal("sentinel pattern should not appear in WorkflowPatterns")
		}
	}
}

// --- detectCodeStyle (pure function tests) ---

func TestDetectCodeStyleGoSnippet(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := `func main() {
	msg := "hello"
	fmt.Println(msg)
}`

	observations := tracker.detectCodeStyle(code)
	if len(observations) == 0 {
		t.Fatal("expected at least 1 code style observation for Go code")
	}

	foundIndent := false
	foundNaming := false
	foundLang := false
	for _, obs := range observations {
		if obs.key == "indentation" {
			foundIndent = true
			if obs.value != "tabs" {
				t.Fatalf("expected indentation=tabs, got %q", obs.value)
			}
		}
		if obs.key == "naming" {
			foundNaming = true
		}
		if obs.key == "language" {
			foundLang = true
			if obs.value != "go" {
				t.Fatalf("expected language=go, got %q", obs.value)
			}
		}
	}
	if !foundIndent {
		t.Fatal("expected indentation observation")
	}
	if !foundLang {
		t.Fatal("expected language observation")
	}
	_ = foundNaming
}

func TestDetectCodeStylePythonSnippet(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := `def main():
    self.process()
    print("hello")
`

	observations := tracker.detectCodeStyle(code)
	foundLang := false
	for _, obs := range observations {
		if obs.key == "language" && obs.value == "python" {
			foundLang = true
		}
	}
	if !foundLang {
		t.Fatal("expected Python language detection")
	}
}

func TestDetectCodeStyleRustSnippet(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := `fn main() {
    let mut x = 5;
    impl MyTrait for Foo {}
    pub fn hello() {}
}`

	observations := tracker.detectCodeStyle(code)
	foundLang := false
	for _, obs := range observations {
		if obs.key == "language" && obs.value == "rust" {
			foundLang = true
		}
	}
	if !foundLang {
		t.Fatal("expected Rust language detection")
	}
}

func TestDetectCodeStyleJavaSnippet(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := `public class Main {
    private int count;
    System.out.println("hello");
}`

	observations := tracker.detectCodeStyle(code)
	foundLang := false
	for _, obs := range observations {
		if obs.key == "language" && obs.value == "java" {
			foundLang = true
		}
	}
	if !foundLang {
		t.Fatal("expected Java language detection")
	}
}

func TestDetectCodeStyleTypeScriptSnippet(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := `const x: string = "hello";
interface User {
    name: string;
    age: number;
}`

	observations := tracker.detectCodeStyle(code)
	foundLang := false
	for _, obs := range observations {
		if obs.key == "language" && obs.value == "typescript" {
			foundLang = true
		}
	}
	if !foundLang {
		t.Fatal("expected TypeScript language detection")
	}
}

func TestDetectCodeStyleNoIndicators(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "just plain text without code"
	observations := tracker.detectCodeStyle(code)
	if len(observations) != 0 {
		t.Fatalf("expected 0 observations for plain text, got %d", len(observations))
	}
}

// --- detectIndentation ---

func TestDetectIndentationTabs(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "line1\n\tindented\n\tmore"

	obs := tracker.detectIndentation(code)
	if obs == nil {
		t.Fatal("expected non-nil indentation observation")
	}
	if obs.value != "tabs" {
		t.Fatalf("expected tabs, got %q", obs.value)
	}
	if obs.confidence != 0.8 {
		t.Fatalf("expected confidence 0.8, got %f", obs.confidence)
	}
}

func TestDetectIndentationSpaces2(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "line1\n  indented\n  more"

	obs := tracker.detectIndentation(code)
	if obs == nil {
		t.Fatal("expected non-nil indentation observation")
	}
	if obs.value != "spaces:2" {
		t.Fatalf("expected spaces:2, got %q", obs.value)
	}
}

func TestDetectIndentationSpaces4(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "line1\n    indented\n    more"

	obs := tracker.detectIndentation(code)
	if obs == nil {
		t.Fatal("expected non-nil indentation observation")
	}
	if obs.value != "spaces:4" {
		t.Fatalf("expected spaces:4, got %q", obs.value)
	}
}

func TestDetectIndentationNoIndentation(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "line1\nline2\nline3"
	obs := tracker.detectIndentation(code)
	if obs != nil {
		t.Fatalf("expected nil for no indentation, got %v", obs)
	}
}

func TestDetectIndentationEmptyLines(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "\n\n\n"
	obs := tracker.detectIndentation(code)
	if obs != nil {
		t.Fatalf("expected nil for empty-only lines, got %v", obs)
	}
}

func TestDetectIndentationMixed(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	// More space-indented lines than tab lines
	code := "line1\n    spaces1\n    spaces2\n\ttab1"

	obs := tracker.detectIndentation(code)
	if obs == nil {
		t.Fatal("expected non-nil indentation observation")
	}
	if obs.value != "spaces:4" {
		t.Fatalf("expected spaces:4 for mixed (more spaces), got %q", obs.value)
	}
}

func TestDetectIndentationEqualTabsSpaces(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	// Equal number of tab and space lines returns nil
	code := "\ttab1\n    space1"

	obs := tracker.detectIndentation(code)
	if obs != nil {
		t.Fatalf("expected nil for equal tabs/spaces, got %v", obs)
	}
}

// --- detectNamingConvention ---

func TestDetectNamingConventionSnakeCase(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "my_variable = 1\nanother_name = 2\nthird_item = 3"

	obs := tracker.detectNamingConvention(code)
	if obs == nil {
		t.Fatal("expected non-nil naming observation")
	}
	if obs.value != "snake_case" {
		t.Fatalf("expected snake_case, got %q", obs.value)
	}
}

func TestDetectNamingConventionCamelCase(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "myVariable = 1\nanotherName = 2\nthirdItem = 3"

	obs := tracker.detectNamingConvention(code)
	if obs == nil {
		t.Fatal("expected non-nil naming observation")
	}
	if obs.value != "camelCase" {
		t.Fatalf("expected camelCase, got %q", obs.value)
	}
}

func TestDetectNamingConventionNoIdentifiers(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	code := "1 2 3"
	obs := tracker.detectNamingConvention(code)
	if obs != nil {
		t.Fatalf("expected nil for no identifiers, got %v", obs)
	}
}

func TestDetectNamingConventionEqualCount(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	// Equal count of snake_case and camelCase returns nil
	code := "my_variable myVariable"

	obs := tracker.detectNamingConvention(code)
	if obs != nil {
		t.Fatalf("expected nil for equal naming counts, got %v", obs)
	}
}

// --- detectLanguage ---

func TestDetectLanguageGo(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	tests := []struct {
		snippet  string
		expected string
	}{
		{"func main() {}", "go"},
		{"package main", "go"},
		{"x := 5", "go"},
		{"import (\n\t\"fmt\"\n)", "go"},
	}

	for _, tt := range tests {
		obs := tracker.detectLanguage(tt.snippet)
		if obs == nil {
			t.Fatalf("expected language detection for %q", tt.snippet)
		}
		if obs.value != tt.expected {
			t.Fatalf("expected language %q for %q, got %q", tt.expected, tt.snippet, obs.value)
		}
	}
}

func TestDetectLanguagePython(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	tests := []struct {
		snippet  string
		expected string
	}{
		{"def foo():", "python"},
		{"self.process()", "python"},
		{"if __name__", "python"},
	}

	for _, tt := range tests {
		obs := tracker.detectLanguage(tt.snippet)
		if obs == nil {
			t.Fatalf("expected language detection for %q", tt.snippet)
		}
		if obs.value != tt.expected {
			t.Fatalf("expected language %q for %q, got %q", tt.expected, tt.snippet, obs.value)
		}
	}
}

func TestDetectLanguageRust(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	tests := []struct {
		snippet  string
		expected string
	}{
		{"let mut x = 5;", "rust"},
		{"pub fn hello() {}", "rust"},
	}

	for _, tt := range tests {
		obs := tracker.detectLanguage(tt.snippet)
		if obs == nil {
			t.Fatalf("expected language detection for %q", tt.snippet)
		}
		if obs.value != tt.expected {
			t.Fatalf("expected language %q for %q, got %q", tt.expected, tt.snippet, obs.value)
		}
	}
}

func TestDetectLanguageNoMatch(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	obs := tracker.detectLanguage("just some random text")
	if obs != nil {
		t.Fatalf("expected nil for no language match, got %v", obs)
	}
}

func TestDetectLanguageConfidenceCapped(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	// Multiple strong Go indicators can push score above 0.9
	snippet := "func main() { x := 5 } import ("
	obs := tracker.detectLanguage(snippet)
	if obs == nil {
		t.Fatal("expected language detection")
	}
	if obs.confidence > 0.9 {
		t.Fatalf("expected confidence capped at 0.9, got %f", obs.confidence)
	}
}

// --- detectCommunicationStyle ---

func TestDetectCommunicationStyleBrief(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	observations := tracker.detectCommunicationStyle("Fix bug")
	if len(observations) == 0 {
		t.Fatal("expected observations for brief message")
	}
	found := false
	for _, obs := range observations {
		if obs.key == "verbosity" && obs.value == "brief" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected brief verbosity")
	}
}

func TestDetectCommunicationStyleModerate(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	msg := "I think we should refactor the authentication module to use JWT tokens instead of sessions"
	observations := tracker.detectCommunicationStyle(msg)
	found := false
	for _, obs := range observations {
		if obs.key == "verbosity" && obs.value == "moderate" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected moderate verbosity")
	}
}

func TestDetectCommunicationStyleVerbose(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	msg := strings.Repeat("word ", 60)
	observations := tracker.detectCommunicationStyle(msg)
	found := false
	for _, obs := range observations {
		if obs.key == "verbosity" && obs.value == "verbose" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected verbose verbosity")
	}
}

func TestDetectCommunicationStyleNoDirectiveOrExploratory(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	msg := "Hello there"
	observations := tracker.detectCommunicationStyle(msg)
	for _, obs := range observations {
		if obs.key == "interaction_style" {
			t.Fatalf("expected no interaction_style for neutral message, got %q", obs.value)
		}
	}
}

// --- buildWorkflowPattern ---

func TestBuildWorkflowPatternNoArgs(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	pattern := tracker.buildWorkflowPattern("bash", nil)
	if pattern != "bash" {
		t.Fatalf("expected pattern %q, got %q", "bash", pattern)
	}
}

func TestBuildWorkflowPatternWithRelevantArgs(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	args := map[string]any{
		"command": "ls -la",
		"action":  "list",
		"extra":   "ignored",
	}

	pattern := tracker.buildWorkflowPattern("bash", args)
	if !strings.Contains(pattern, "bash") {
		t.Fatalf("expected pattern to contain tool name %q", "bash")
	}
	if !strings.Contains(pattern, "command=ls -la") {
		t.Fatalf("expected pattern to contain command arg, got %q", pattern)
	}
	if !strings.Contains(pattern, "action=list") {
		t.Fatalf("expected pattern to contain action arg, got %q", pattern)
	}
	if strings.Contains(pattern, "extra") {
		t.Fatalf("expected pattern NOT to contain irrelevant arg 'extra', got %q", pattern)
	}
}

func TestBuildWorkflowPatternRelevantKeys(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	relevantKeys := []string{"action", "type", "mode", "command", "path", "query"}
	for _, key := range relevantKeys {
		args := map[string]any{key: "value"}
		pattern := tracker.buildWorkflowPattern("tool", args)
		if !strings.Contains(pattern, key+"=value") {
			t.Fatalf("expected pattern to contain %s=value, got %q", key, pattern)
		}
	}
}

func TestBuildWorkflowPatternEmptyArgs(t *testing.T) {
	tracker := NewPreferenceTracker(nil)

	pattern := tracker.buildWorkflowPattern("tool", map[string]any{})
	if pattern != "tool" {
		t.Fatalf("expected pattern %q for empty args, got %q", "tool", pattern)
	}
}

// --- Integration: full flow ---

func TestPreferenceTrackerFullFlow(t *testing.T) {
	s := newTestStoreForModeling(t)
	engine := NewUserModelingEngine(s, nil, "user-1")
	tracker := NewPreferenceTracker(engine)
	ctx := context.Background()

	if err := tracker.ObserveCodeStyle(ctx, "sess-1", "func main() {\n\tfmt.Println()\n}"); err != nil {
		t.Fatalf("ObserveCodeStyle error: %v", err)
	}
	if err := tracker.ObserveCommunication(ctx, "sess-1", "Fix the bug now"); err != nil {
		t.Fatalf("ObserveCommunication error: %v", err)
	}

	tracker.WorkflowThreshold = 2
	for i := 0; i < 2; i++ {
		if err := tracker.ObserveWorkflow(ctx, "sess-1", "bash", map[string]any{"command": "go test"}); err != nil {
			t.Fatalf("ObserveWorkflow error: %v", err)
		}
	}

	snapshot, err := engine.SynthesizeModel(ctx)
	if err != nil {
		t.Fatalf("SynthesizeModel error: %v", err)
	}

	if len(snapshot.Preferences) == 0 {
		t.Fatal("expected non-empty preferences after full flow")
	}
	if snapshot.CommunicationStyle == "" {
		t.Fatal("expected non-empty communication style after full flow")
	}

	prefs := tracker.GetCurrentPreferences()
	if len(prefs.CodeStyle) == 0 {
		t.Fatal("expected non-empty code style preferences")
	}
}
