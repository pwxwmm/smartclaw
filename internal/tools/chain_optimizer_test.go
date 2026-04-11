package tools

import (
	"testing"
)

func TestChainOptimizer_Analyze(t *testing.T) {
	o := NewChainOptimizer()
	o.Enable()

	// Record a read_file + edit_file sequence
	o.RecordCall("read_file", map[string]any{"path": "/tmp/test.go"}, nil)
	o.RecordCall("edit_file", map[string]any{"path": "/tmp/test.go", "old_string": "foo", "new_string": "bar"}, nil)

	suggestions := o.Analyze()
	if len(suggestions) == 0 {
		t.Fatal("expected at least one optimization suggestion for read_file+edit_file")
	}

	found := false
	for _, s := range suggestions {
		if s.Pattern == "read_edit" {
			found = true
			if s.EstimatedSavings <= 0 {
				t.Error("expected positive estimated savings")
			}
		}
	}
	if !found {
		t.Error("expected 'read_edit' pattern in suggestions")
	}
}

func TestChainOptimizer_Merge(t *testing.T) {
	o := NewChainOptimizer()

	calls := []ToolCall{
		{Name: "read_file", Input: map[string]any{"path": "/tmp/test.go"}},
		{Name: "edit_file", Input: map[string]any{"path": "/tmp/test.go", "old_string": "foo", "new_string": "bar"}},
	}

	suggestions := o.AnalyzeCalls(calls)
	if len(suggestions) == 0 {
		t.Fatal("expected suggestion for read_file+edit_file")
	}

	merged := o.Merge(calls, suggestions[0])
	if merged == nil {
		t.Fatal("expected merged call, got nil")
	}

	if merged.Language != "bash" {
		t.Errorf("expected language 'bash', got '%s'", merged.Language)
	}
	if len(merged.OriginalCalls) != 2 {
		t.Errorf("expected 2 original calls, got %d", len(merged.OriginalCalls))
	}
	if merged.Script == "" {
		t.Error("expected non-empty script")
	}
}

func TestChainOptimizer_NoMatch(t *testing.T) {
	o := NewChainOptimizer()
	o.Enable()

	o.RecordCall("bash", map[string]any{"command": "ls"}, nil)
	o.RecordCall("read_file", map[string]any{"path": "/tmp/test.go"}, nil)

	suggestions := o.Analyze()
	// bash + read_file is not a registered pattern
	for _, s := range suggestions {
		if s.Pattern == "bash_bash" {
			t.Error("should not detect bash_bash pattern for bash+read_file")
		}
	}
}

func TestChainOptimizer_Disabled(t *testing.T) {
	o := NewChainOptimizer()
	// Don't enable

	o.RecordCall("read_file", map[string]any{"path": "/tmp/test.go"}, nil)
	o.RecordCall("edit_file", map[string]any{"path": "/tmp/test.go"}, nil)

	suggestions := o.Analyze()
	if len(suggestions) != 0 {
		t.Error("disabled optimizer should not produce suggestions from log")
	}
}

func TestChainOptimizer_GlobGrep(t *testing.T) {
	o := NewChainOptimizer()

	calls := []ToolCall{
		{Name: "glob", Input: map[string]any{"pattern": "*.go", "path": "."}},
		{Name: "grep", Input: map[string]any{"pattern": "TODO", "path": "."}},
	}

	suggestions := o.AnalyzeCalls(calls)
	if len(suggestions) == 0 {
		t.Fatal("expected suggestion for glob+grep")
	}

	merged := o.Merge(calls, suggestions[0])
	if merged == nil {
		t.Fatal("expected merged call, got nil")
	}
	if merged.Savings <= 0 {
		t.Error("expected positive savings")
	}
}

func TestChainOptimizer_BashBash(t *testing.T) {
	o := NewChainOptimizer()

	calls := []ToolCall{
		{Name: "bash", Input: map[string]any{"command": "echo hello"}},
		{Name: "bash", Input: map[string]any{"command": "echo world"}},
	}

	suggestions := o.AnalyzeCalls(calls)
	if len(suggestions) == 0 {
		t.Fatal("expected suggestion for bash+bash")
	}

	merged := o.Merge(calls, suggestions[0])
	if merged == nil {
		t.Fatal("expected merged call")
	}
	if !containsStr(merged.Script, "&&") {
		t.Errorf("expected combined script with &&, got: %s", merged.Script)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
