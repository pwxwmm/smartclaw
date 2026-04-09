package tui

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "Python import",
			code:     "import requests\nprint('hello')",
			expected: "python",
		},
		{
			name:     "Python def",
			code:     "def main():\n    pass",
			expected: "python",
		},
		{
			name:     "Go package",
			code:     "package main\n\nfunc main() {}",
			expected: "go",
		},
		{
			name:     "JavaScript const",
			code:     "const x = 5;\nconsole.log(x);",
			expected: "javascript",
		},
		{
			name:     "TypeScript interface",
			code:     "interface User {\n  name: string\n}",
			expected: "typescript",
		},
		{
			name:     "Bash script",
			code:     "#!/bin/bash\necho 'hello'",
			expected: "bash",
		},
		{
			name:     "SQL query",
			code:     "SELECT * FROM users WHERE id = 1",
			expected: "sql",
		},
		{
			name:     "JSON object",
			code:     "{\"name\": \"John\", \"age\": 30}",
			expected: "json",
		},
		{
			name:     "Empty code",
			code:     "",
			expected: "",
		},
		{
			name:     "Unknown language",
			code:     "some random text without clear patterns",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLanguage(tt.code)
			if result != tt.expected {
				t.Errorf("detectLanguage() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAddLanguageSpecifiers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name: "Add Python specifier",
			input: "```" + `
import requests
print('hello')
` + "```",
			contains: "```python",
		},
		{
			name: "Add Go specifier",
			input: "```" + `
package main
func main() {}
` + "```",
			contains: "```go",
		},
		{
			name: "Preserve existing specifier",
			input: "```javascript" + `
const x = 5;
` + "```",
			contains: "```javascript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddLanguageSpecifiers(tt.input)
			if !contains(result, tt.contains) {
				t.Errorf("AddLanguageSpecifiers() = %q, should contain %q", result, tt.contains)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
