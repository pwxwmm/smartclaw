package chain

import (
	"math"
	"testing"
)

func TestAnalyzeChain_ReadFileEditFile_SameFile(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "read_file", Input: map[string]any{"path": "/tmp/foo.go"}},
			{ToolName: "edit_file", Input: map[string]any{"path": "/tmp/foo.go"}},
		},
		Query: "fix the bug",
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	s := suggestions[0]
	if s.ToolName != "edit_file" {
		t.Errorf("expected merged tool 'edit_file', got '%s'", s.ToolName)
	}
	if s.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9 for same-file read+edit, got %.1f", s.Confidence)
	}
	if s.Savings != 1 {
		t.Errorf("expected savings 1, got %d", s.Savings)
	}
	if len(s.Steps) != 2 || s.Steps[0] != 0 || s.Steps[1] != 1 {
		t.Errorf("expected steps [0,1], got %v", s.Steps)
	}
}

func TestAnalyzeChain_ReadFileEditFile_DifferentFile(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "read_file", Input: map[string]any{"path": "/tmp/a.go"}},
			{ToolName: "edit_file", Input: map[string]any{"path": "/tmp/b.go"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Confidence != 0.5 {
		t.Errorf("expected confidence 0.5 for different-file read+edit, got %.1f", suggestions[0].Confidence)
	}
}

func TestAnalyzeChain_BashBash(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "bash", Input: map[string]any{"command": "go build"}},
			{ToolName: "bash", Input: map[string]any{"command": "go test"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	s := suggestions[0]
	if s.ToolName != "bash" {
		t.Errorf("expected merged tool 'bash', got '%s'", s.ToolName)
	}
	if s.Confidence != 0.7 {
		t.Errorf("expected confidence 0.7 for bash+bash, got %.1f", s.Confidence)
	}
}

func TestAnalyzeChain_NoMergeablePattern(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "think", Input: map[string]any{}},
			{ToolName: "web_search", Input: map[string]any{"query": "golang generics"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 0 {
		t.Fatalf("expected 0 suggestions for non-mergeable pattern, got %d", len(suggestions))
	}
}

func TestAnalyzeChain_OverlappingPatterns_Greedy(t *testing.T) {
	co := NewChainOptimizer()
	// read_file, edit_file, bash → read_file+edit_file should match first (greedy),
	// leaving bash alone.
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "read_file", Input: map[string]any{"path": "x.go"}},
			{ToolName: "edit_file", Input: map[string]any{"path": "x.go"}},
			{ToolName: "bash", Input: map[string]any{"command": "go test"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion (greedy), got %d", len(suggestions))
	}
	if suggestions[0].Steps[0] != 0 || suggestions[0].Steps[1] != 1 {
		t.Errorf("expected greedy match on [0,1], got %v", suggestions[0].Steps)
	}
}

func TestAnalyzeChain_MultipleNonOverlappingMatches(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "bash", Input: map[string]any{"command": "ls"}},
			{ToolName: "bash", Input: map[string]any{"command": "pwd"}},
			{ToolName: "glob", Input: map[string]any{"pattern": "*.go"}},
			{ToolName: "read_file", Input: map[string]any{"path": "main.go"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 2 {
		t.Fatalf("expected 2 non-overlapping suggestions, got %d", len(suggestions))
	}
}

func TestShouldMerge_ShortHistory(t *testing.T) {
	co := NewChainOptimizer()
	if co.ShouldMerge("query", []ChainStep{{ToolName: "bash"}}) {
		t.Error("should not merge with < 2 steps")
	}
	if co.ShouldMerge("query", nil) {
		t.Error("should not merge with nil history")
	}
}

func TestShouldMerge_MergeablePattern(t *testing.T) {
	co := NewChainOptimizer()
	history := []ChainStep{
		{ToolName: "read_file", Input: map[string]any{"path": "a.go"}},
		{ToolName: "edit_file", Input: map[string]any{"path": "a.go"}},
	}
	if !co.ShouldMerge("fix bug", history) {
		t.Error("should merge read+edit on same file (confidence 0.9 >= 0.7)")
	}
}

func TestShouldMerge_LowConfidence(t *testing.T) {
	co := NewChainOptimizer()
	history := []ChainStep{
		{ToolName: "read_file", Input: map[string]any{"path": "a.go"}},
		{ToolName: "edit_file", Input: map[string]any{"path": "b.go"}},
	}
	if co.ShouldMerge("fix bug", history) {
		t.Error("should not merge with low confidence (0.5 < 0.7)")
	}
}

func TestShouldMerge_Disabled(t *testing.T) {
	co := NewChainOptimizer()
	co.SetEnabled(false)
	history := []ChainStep{
		{ToolName: "read_file", Input: map[string]any{"path": "a.go"}},
		{ToolName: "edit_file", Input: map[string]any{"path": "a.go"}},
	}
	if co.ShouldMerge("fix bug", history) {
		t.Error("should not merge when disabled")
	}
}

func TestIsEnabled(t *testing.T) {
	co := NewChainOptimizer()
	if !co.IsEnabled() {
		t.Error("should be enabled by default")
	}
	co.SetEnabled(false)
	if co.IsEnabled() {
		t.Error("should be disabled after SetEnabled(false)")
	}
	co.SetEnabled(true)
	if !co.IsEnabled() {
		t.Error("should be enabled after SetEnabled(true)")
	}
}

func TestEstimateSavings(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "bash", Input: map[string]any{"command": "ls"}},
			{ToolName: "bash", Input: map[string]any{"command": "pwd"}},
			{ToolName: "glob", Input: map[string]any{"pattern": "*.go"}},
			{ToolName: "read_file", Input: map[string]any{"path": "main.go"}},
		},
	}

	callsSaved, confidence := co.EstimateSavings(chain)
	if callsSaved != 2 {
		t.Errorf("expected 2 calls saved, got %d", callsSaved)
	}
	if confidence <= 0 || confidence > 1 {
		t.Errorf("expected confidence in (0,1], got %.2f", confidence)
	}
}

func TestEstimateSavings_EmptyChain(t *testing.T) {
	co := NewChainOptimizer()
	callsSaved, confidence := co.EstimateSavings(&Chain{})
	if callsSaved != 0 || confidence != 0 {
		t.Errorf("expected (0,0) for empty chain, got (%d, %.2f)", callsSaved, confidence)
	}
}

func TestEstimateSavings_SingleStep(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{Steps: []ChainStep{{ToolName: "bash"}}}
	callsSaved, confidence := co.EstimateSavings(chain)
	if callsSaved != 0 || confidence != 0 {
		t.Errorf("expected (0,0) for single-step chain, got (%d, %.2f)", callsSaved, confidence)
	}
}

func TestEstimateSavings_NoMergeable(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "think", Input: map[string]any{}},
			{ToolName: "web_search", Input: map[string]any{}},
		},
	}
	callsSaved, confidence := co.EstimateSavings(chain)
	if callsSaved != 0 || confidence != 0 {
		t.Errorf("expected (0,0) for non-mergeable chain, got (%d, %.2f)", callsSaved, confidence)
	}
}

func TestAddPattern_Custom(t *testing.T) {
	co := NewChainOptimizer()
	co.AddPattern(MergePattern{
		Sequence: []string{"web_search", "web_fetch"},
		MergedAs: "execute_code",
		Reason:   "search-then-fetch: web search then fetching result can be collapsed",
		Savings:  1,
	})

	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "web_search", Input: map[string]any{"query": "golang"}},
			{ToolName: "web_fetch", Input: map[string]any{"url": "https://example.com"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion with custom pattern, got %d", len(suggestions))
	}
	if suggestions[0].ToolName != "execute_code" {
		t.Errorf("expected merged tool 'execute_code', got '%s'", suggestions[0].ToolName)
	}
}

func TestAnalyzeChain_EmptyChain(t *testing.T) {
	co := NewChainOptimizer()
	suggestions := co.AnalyzeChain(&Chain{})
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for empty chain, got %d", len(suggestions))
	}
}

func TestAnalyzeChain_SingleStep(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{Steps: []ChainStep{{ToolName: "bash"}}}
	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for single step, got %d", len(suggestions))
	}
}

func TestAnalyzeChain_NilChain(t *testing.T) {
	co := NewChainOptimizer()
	suggestions := co.AnalyzeChain(nil)
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for nil chain, got %d", len(suggestions))
	}
}

func TestAnalyzeChain_EditFileThenBash(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "edit_file", Input: map[string]any{"path": "main.go"}},
			{ToolName: "bash", Input: map[string]any{"command": "go test"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Confidence != 0.8 {
		t.Errorf("expected confidence 0.8 for edit+test, got %.1f", suggestions[0].Confidence)
	}
	if suggestions[0].ToolName != "execute_code" {
		t.Errorf("expected merged tool 'execute_code', got '%s'", suggestions[0].ToolName)
	}
}

func TestAnalyzeChain_GlobThenReadFile_SameDir(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "glob", Input: map[string]any{"pattern": "/src/*.go"}},
			{ToolName: "read_file", Input: map[string]any{"path": "/src/main.go"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Confidence != 0.8 {
		t.Errorf("expected confidence 0.8 for glob+read same dir, got %.1f", suggestions[0].Confidence)
	}
}

func TestAnalyzeChain_GrepThenReadFile_DifferentPath(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "grep", Input: map[string]any{"pattern": "TODO", "path": "/a/"}},
			{ToolName: "read_file", Input: map[string]any{"path": "/b/file.go"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Confidence != 0.5 {
		t.Errorf("expected confidence 0.5 for grep+read different dirs, got %.1f", suggestions[0].Confidence)
	}
}

func TestAnalyzeChain_BashThenEditFile(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "bash", Input: map[string]any{"command": "go mod tidy"}},
			{ToolName: "edit_file", Input: map[string]any{"path": "go.mod"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Confidence != 0.6 {
		t.Errorf("expected confidence 0.6 for bash+edit, got %.1f", suggestions[0].Confidence)
	}
}

func TestAnalyzeChain_ReadFileReadFile(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "read_file", Input: map[string]any{"path": "a.go"}},
			{ToolName: "read_file", Input: map[string]any{"path": "b.go"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Confidence != 0.5 {
		t.Errorf("expected confidence 0.5 for multi-read different files, got %.1f", suggestions[0].Confidence)
	}
}

func TestAnalyzeChain_SortedBySavings(t *testing.T) {
	co := NewChainOptimizer()
	co.AddPattern(MergePattern{
		Sequence: []string{"bash", "bash", "bash"},
		MergedAs: "bash",
		Reason:   "triple bash",
		Savings:  2,
	})

	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "bash", Input: map[string]any{"command": "a"}},
			{ToolName: "bash", Input: map[string]any{"command": "b"}},
			{ToolName: "bash", Input: map[string]any{"command": "c"}},
			{ToolName: "glob", Input: map[string]any{"pattern": "*.go"}},
			{ToolName: "read_file", Input: map[string]any{"path": "x.go"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 2 {
		t.Fatalf("expected 2 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].Savings < suggestions[1].Savings {
		t.Errorf("expected suggestions sorted by savings descending, got %d then %d", suggestions[0].Savings, suggestions[1].Savings)
	}
}

func TestAnalyzeChain_GlobReadSameFile(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "glob", Input: map[string]any{"pattern": "/src/main.go"}},
			{ToolName: "read_file", Input: map[string]any{"path": "/src/main.go"}},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if math.Abs(suggestions[0].Confidence-0.9) > 0.01 {
		t.Errorf("expected confidence 0.9 for glob+read same file, got %.1f", suggestions[0].Confidence)
	}
}

func TestAnalyzeChain_NilInput(t *testing.T) {
	co := NewChainOptimizer()
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "read_file", Input: nil},
			{ToolName: "edit_file", Input: nil},
		},
	}

	suggestions := co.AnalyzeChain(chain)
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Confidence != 0.5 {
		t.Errorf("expected confidence 0.5 for nil-input read+edit, got %.1f", suggestions[0].Confidence)
	}
}

func TestEstimateSavings_WeightedConfidence(t *testing.T) {
	co := NewChainOptimizer()
	// bash+bash (confidence 0.7, savings 1) + edit_file+bash (confidence 0.8, savings 1)
	chain := &Chain{
		Steps: []ChainStep{
			{ToolName: "bash", Input: map[string]any{"command": "ls"}},
			{ToolName: "bash", Input: map[string]any{"command": "pwd"}},
			{ToolName: "edit_file", Input: map[string]any{"path": "a.go"}},
			{ToolName: "bash", Input: map[string]any{"command": "go test"}},
		},
	}

	callsSaved, confidence := co.EstimateSavings(chain)
	if callsSaved != 2 {
		t.Errorf("expected 2 calls saved, got %d", callsSaved)
	}
	// weighted: (0.7*1 + 0.8*1) / 2 = 0.75
	if math.Abs(confidence-0.75) > 0.01 {
		t.Errorf("expected weighted confidence 0.75, got %.2f", confidence)
	}
}

func TestSameDirectory(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool
	}{
		{"/src/a.go", "/src/b.go", true},
		{"/src/a.go", "/pkg/b.go", false},
		{"", "/src/a.go", false},
		{"/src/a.go", "", false},
		{"a.go", "b.go", false},
	}

	for _, tt := range tests {
		result := sameDirectory(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("sameDirectory(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
		}
	}
}
