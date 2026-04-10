package learning

import (
	"testing"
)

func TestProactiveEngine_EditThenTest(t *testing.T) {
	pe := NewProactiveEngine()

	pe.RecordAction(Action{Type: "edit", Tool: "edit_file", Content: "fix auth handler"})
	suggestion := pe.MaybeSuggest(Action{Type: "edit", Tool: "write_file", Content: "update auth module"})

	if suggestion == nil {
		t.Fatal("expected suggestion after edit sequence")
	}
	if suggestion.SkillID != "test-generator" {
		t.Errorf("expected test-generator skill, got %s", suggestion.SkillID)
	}
}

func TestProactiveEngine_NoSuggestionForUnrelated(t *testing.T) {
	pe := NewProactiveEngine()

	pe.RecordAction(Action{Type: "query", Tool: "read_file", Content: "show config"})
	suggestion := pe.MaybeSuggest(Action{Type: "query", Tool: "bash", Content: "ls"})

	if suggestion != nil {
		t.Error("expected no suggestion for unrelated actions")
	}
}

func TestProactiveEngine_FixThenRegression(t *testing.T) {
	pe := NewProactiveEngine()

	pe.RecordAction(Action{Type: "debug", Tool: "bash", Content: "debug the crash"})
	suggestion := pe.MaybeSuggest(Action{Type: "fix", Tool: "bash", Content: "fix the null pointer"})

	if suggestion == nil {
		t.Fatal("expected suggestion after debug+fix sequence")
	}
	if suggestion.SkillID != "test-generator" {
		t.Errorf("expected test-generator skill for regression test, got %s", suggestion.SkillID)
	}
}

func TestProactiveEngine_LowConfidenceFiltered(t *testing.T) {
	pe := NewProactiveEngine()
	pe.patterns = nil

	pe.AddPattern(WorkflowPattern{
		Name:       "low-conf",
		Sequence:   []string{"edit"},
		Suggest:    "Should not appear",
		Confidence: 0.2,
	})

	suggestion := pe.MaybeSuggest(Action{Type: "edit", Tool: "edit_file", Content: "change"})

	if suggestion != nil {
		t.Error("expected no suggestion below 0.4 threshold")
	}
}

func TestProactiveEngine_HistoryLimit(t *testing.T) {
	pe := NewProactiveEngine()
	pe.maxHistory = 5

	for i := 0; i < 10; i++ {
		pe.RecordAction(Action{Type: "edit", Tool: "edit_file", Content: "change"})
	}

	if len(pe.userHistory) > 5 {
		t.Errorf("expected history capped at 5, got %d", len(pe.userHistory))
	}
}

func TestProactiveEngine_AddPattern(t *testing.T) {
	pe := NewProactiveEngine()

	custom := WorkflowPattern{
		Name:       "custom-flow",
		Sequence:   []string{"custom-a", "custom-b"},
		Suggest:    "Do custom thing?",
		SkillID:    "custom-skill",
		Confidence: 0.8,
	}
	pe.AddPattern(custom)

	patterns := pe.GetPatterns()
	found := false
	for _, p := range patterns {
		if p.Name == "custom-flow" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected custom pattern to be added")
	}
}

func TestClassifyAction(t *testing.T) {
	tests := []struct {
		action Action
		want   string
	}{
		{Action{Tool: "edit_file", Content: "fix bug"}, "edit"},
		{Action{Tool: "write_file", Content: "create new module"}, "write:new"},
		{Action{Tool: "bash", Content: "go test ./..."}, "test:pass"},
		{Action{Tool: "bash", Content: "dlv debug main.go"}, "debug"},
		{Action{Tool: "read_file", Content: "show file"}, "read"},
	}

	for _, tt := range tests {
		got := classifyAction(tt.action)
		if got != tt.want {
			t.Errorf("classifyAction(%v) = %q, want %q", tt.action, got, tt.want)
		}
	}
}

func TestActionMatches(t *testing.T) {
	tests := []struct {
		actual   string
		expected string
		want     bool
	}{
		{"edit", "edit", true},
		{"write:new", "write:new", true},
		{"write", "write:new", true},
		{"write:new", "write", true},
		{"edit", "write", false},
	}

	for _, tt := range tests {
		got := actionMatches(tt.actual, tt.expected)
		if got != tt.want {
			t.Errorf("actionMatches(%q, %q) = %v, want %v", tt.actual, tt.expected, got, tt.want)
		}
	}
}
