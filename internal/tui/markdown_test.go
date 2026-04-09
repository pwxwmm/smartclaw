package tui

import (
	"fmt"
	"strings"
	"testing"
)

func TestRemoveHeadingMarkers(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple h3 heading",
			input:    "\x1b[1;36m### Heading\x1b[0m",
			expected: "\x1b[1;36mHeading\x1b[0m",
		},
		{
			name:     "h3 with styled markers",
			input:    "\x1b[1;36m### \x1b[0m\x1b[1;36mHeading\x1b[0m",
			expected: "\x1b[1;36mHeading\x1b[0m",
		},
		{
			name:     "h2 heading",
			input:    "\x1b[1;35m## Heading\x1b[0m",
			expected: "\x1b[1;35mHeading\x1b[0m",
		},
		{
			name:     "h1 heading",
			input:    "\x1b[1;34m# Heading\x1b[0m",
			expected: "\x1b[1;34mHeading\x1b[0m",
		},
		{
			name:     "normal text unchanged",
			input:    "Normal text without heading",
			expected: "Normal text without heading",
		},
		{
			name:     "heading in middle of text",
			input:    "Before\n\x1b[1;36m### Heading\x1b[0m\nAfter",
			expected: "Before\n\x1b[1;36mHeading\x1b[0m\nAfter",
		},
		{
			name:     "plain heading without ANSI",
			input:    "### Plain Heading",
			expected: "Plain Heading",
		},
		{
			name:     "plain h2 without ANSI",
			input:    "## Plain H2",
			expected: "Plain H2",
		},
		{
			name:     "plain h1 without ANSI",
			input:    "# Plain H1",
			expected: "Plain H1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeHeadingMarkers(tt.input)
			if result != tt.expected {
				t.Errorf("removeHeadingMarkers() = %q, want %q", result, tt.expected)
				fmt.Printf("Input (escaped): %q\n", tt.input)
				fmt.Printf("Got (escaped): %q\n", result)
				fmt.Printf("Want (escaped): %q\n", tt.expected)
			}
		})
	}
}

func TestMarkdownRenderer_RenderWithStyle(t *testing.T) {
	theme := Theme{Name: "dark"}
	renderer := NewMarkdownRenderer(theme)

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "h3 heading",
			input:    "### Test Heading",
			contains: "Test Heading",
		},
		{
			name:     "h2 heading",
			input:    "## Another Heading",
			contains: "Another Heading",
		},
		{
			name:     "h1 heading",
			input:    "# Main Heading",
			contains: "Main Heading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderer.RenderWithStyle(tt.input, 80)

			if !strings.Contains(result, tt.contains) {
				t.Errorf("RenderWithStyle() result doesn't contain %q. Got: %q", tt.contains, result)
			}

			if strings.Contains(result, "###") {
				t.Errorf("RenderWithStyle() should not contain ###. Got: %q", result)
			}
			if strings.Contains(result, "##") && !strings.Contains(result, "#!") {
				// Allow ## in code blocks but not as heading markers
				if strings.HasPrefix(result, "##") || strings.Contains(result, "\n##") {
					t.Errorf("RenderWithStyle() should not contain ## heading markers. Got: %q", result)
				}
			}
		})
	}
}
