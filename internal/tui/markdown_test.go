package tui

import (
	"regexp"
	"strings"
	"testing"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestRemoveHeadingMarkers(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		contains    string
		notContains string
	}{
		{
			name:     "plain h3 heading",
			input:    "### Heading",
			contains: "Heading",
		},
		{
			name:        "h3 markers removed",
			input:       "### Heading",
			notContains: "###",
		},
		{
			name:     "plain h2 heading",
			input:    "## Heading",
			contains: "Heading",
		},
		{
			name:        "h2 markers removed",
			input:       "## Heading",
			notContains: "##",
		},
		{
			name:     "plain h1 heading",
			input:    "# Heading",
			contains: "Heading",
		},
		{
			name:     "normal text unchanged",
			input:    "Normal text without heading",
			contains: "Normal text without heading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeHeadingMarkers(tt.input)
			clean := stripANSI(result)

			if tt.contains != "" && !strings.Contains(clean, tt.contains) {
				t.Errorf("removeHeadingMarkers() result should contain %q, got %q", tt.contains, clean)
			}
			if tt.notContains != "" && strings.Contains(clean, tt.notContains) {
				t.Errorf("removeHeadingMarkers() result should not contain %q, got %q", tt.notContains, clean)
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
			clean := stripANSI(result)

			if !strings.Contains(clean, tt.contains) {
				t.Errorf("RenderWithStyle() result doesn't contain %q. Got: %q", tt.contains, clean)
			}

			if strings.Contains(clean, "###") {
				t.Errorf("RenderWithStyle() should not contain ###. Got: %q", clean)
			}
		})
	}
}
