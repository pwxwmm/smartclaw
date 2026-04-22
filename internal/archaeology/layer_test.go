package archaeology

import (
	"context"
	"strings"
	"testing"

	"github.com/instructkr/smartclaw/internal/git"
)

func TestExtractFilePaths(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "single file path",
			query:    "look at internal/tools/registry.go",
			expected: []string{"internal/tools/registry.go"},
		},
		{
			name:     "multiple file paths",
			query:    "check internal/foo/bar.go and pkg/baz/qux.go",
			expected: []string{"internal/foo/bar.go", "pkg/baz/qux.go"},
		},
		{
			name:     "no file paths",
			query:    "how do I write a for loop",
			expected: nil,
		},
		{
			name:     "file path in quotes",
			query:    `the file "internal/memory/manager.go" has an issue`,
			expected: []string{"internal/memory/manager.go"},
		},
		{
			name:     "file path with dash",
			query:    "edit internal/git/context-file.go",
			expected: []string{"internal/git/context-file.go"},
		},
		{
			name:     "deduplicates paths",
			query:    "internal/foo/bar.go and internal/foo/bar.go",
			expected: []string{"internal/foo/bar.go"},
		},
		{
			name:     "empty query",
			query:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFilePaths(tt.query)
			if len(result) != len(tt.expected) {
				t.Errorf("extractFilePaths(%q) = %v, want %v", tt.query, result, tt.expected)
				return
			}
			for i, p := range result {
				if p != tt.expected[i] {
					t.Errorf("extractFilePaths(%q)[%d] = %q, want %q", tt.query, i, p, tt.expected[i])
				}
			}
		})
	}
}

func TestParseBlamePorcelain(t *testing.T) {
	input := `abc1234 1 1 1
author Alice
author-mail <alice@example.com>
author-time 1705312800
summary feat: add feature
	line one content
def5678 2 2 1
author Bob
author-mail <bob@example.com>
author-time 1705399200
summary fix: bug fix
	line two content`

	results := git.ParseBlamePorcelain(input, 0)

	if len(results) != 2 {
		t.Fatalf("ParseBlamePorcelain returned %d entries, want 2", len(results))
	}
	if results[0].Commit != "abc1234" {
		t.Errorf("results[0].Commit = %q, want %q", results[0].Commit, "abc1234")
	}
	if results[0].Author != "Alice" {
		t.Errorf("results[0].Author = %q, want %q", results[0].Author, "Alice")
	}
	if results[0].Line != 1 {
		t.Errorf("results[0].Line = %d, want 1", results[0].Line)
	}
	if results[0].Content != "line one content" {
		t.Errorf("results[0].Content = %q, want %q", results[0].Content, "line one content")
	}
	if results[1].Author != "Bob" {
		t.Errorf("results[1].Author = %q, want %q", results[1].Author, "Bob")
	}
}

func TestParseBlamePorcelainMaxLines(t *testing.T) {
	input := `abc1234 1 1 1
author Alice
author-time 1705312800
	line one
def5678 2 2 1
author Bob
author-time 1705399200
	line two`

	results := git.ParseBlamePorcelain(input, 1)
	if len(results) != 1 {
		t.Fatalf("ParseBlamePorcelain with maxLines=1 returned %d entries, want 1", len(results))
	}
	if results[0].Author != "Alice" {
		t.Errorf("results[0].Author = %q, want %q", results[0].Author, "Alice")
	}
}

func TestParseFileLog(t *testing.T) {
	input := `abc1234|Alice|2024-01-15 10:30:00 +0100|feat: add feature
def5678|Bob|2024-01-14 09:00:00 +0100|fix: bug fix`

	results := git.ParseFileLog(input)

	if len(results) != 2 {
		t.Fatalf("ParseFileLog returned %d entries, want 2", len(results))
	}
	if results[0].Hash != "abc1234" {
		t.Errorf("results[0].Hash = %q, want %q", results[0].Hash, "abc1234")
	}
	if results[0].Author != "Alice" {
		t.Errorf("results[0].Author = %q, want %q", results[0].Author, "Alice")
	}
	if results[0].Subject != "feat: add feature" {
		t.Errorf("results[0].Subject = %q, want %q", results[0].Subject, "feat: add feature")
	}
	if results[1].Author != "Bob" {
		t.Errorf("results[1].Author = %q, want %q", results[1].Author, "Bob")
	}
}

func TestParseFileLogEmpty(t *testing.T) {
	results := git.ParseFileLog("")
	if len(results) != 0 {
		t.Errorf("ParseFileLog empty input = %d entries, want 0", len(results))
	}
}

func TestParseFileLogMalformed(t *testing.T) {
	input := "only-two|parts\nabc|def|ghi|jkl|extra"
	results := git.ParseFileLog(input)
	if len(results) != 1 {
		t.Fatalf("ParseFileLog malformed input = %d entries, want 1", len(results))
	}
	if results[0].Hash != "abc" {
		t.Errorf("results[0].Hash = %q, want %q", results[0].Hash, "abc")
	}
}

func TestBuildArchaeologyPromptEmptyQuery(t *testing.T) {
	al := NewArchaeologyLayer("/tmp/nonexistent")
	result := al.BuildArchaeologyPrompt(context.Background(), "")
	if result != "" {
		t.Errorf("BuildArchaeologyPrompt empty query = %q, want empty", result)
	}
}

func TestBuildArchaeologyPromptNoFilePaths(t *testing.T) {
	al := NewArchaeologyLayer("/tmp/nonexistent")
	result := al.BuildArchaeologyPrompt(context.Background(), "how do I write a for loop")
	if result != "" {
		t.Errorf("BuildArchaeologyPrompt no file paths = %q, want empty", result)
	}
}

func TestBuildArchaeologyPromptWithNonexistentDir(t *testing.T) {
	al := NewArchaeologyLayer("/tmp/nonexistent-dir-for-test")
	result := al.BuildArchaeologyPrompt(context.Background(), "check internal/foo/bar.go")
	if result != "" {
		t.Errorf("BuildArchaeologyPrompt with nonexistent dir = %q, want empty (git commands fail)", result)
	}
}

func TestFormatAuthorCounts(t *testing.T) {
	counts := map[string]int{"alice": 3, "bob": 2}
	result := formatAuthorCounts(counts)
	if !strings.Contains(result, "alice(3)") {
		t.Errorf("formatAuthorCounts missing alice(3): %q", result)
	}
	if !strings.Contains(result, "bob(2)") {
		t.Errorf("formatAuthorCounts missing bob(2): %q", result)
	}
}

func TestFormatOwnership(t *testing.T) {
	authorLines := map[string]int{"alice": 65, "bob": 35}
	result := formatOwnership(authorLines, 100)
	if !strings.Contains(result, "65% alice") {
		t.Errorf("formatOwnership missing 65%% alice: %q", result)
	}
	if !strings.Contains(result, "35% bob") {
		t.Errorf("formatOwnership missing 35%% bob: %q", result)
	}
}

func TestTruncateDate(t *testing.T) {
	result := truncateDate("2024-01-15 10:30:00 +0100")
	if result != "2024-01-15" {
		t.Errorf("truncateDate = %q, want %q", result, "2024-01-15")
	}
	result = truncateDate("short")
	if result != "short" {
		t.Errorf("truncateDate short = %q, want %q", result, "short")
	}
}
