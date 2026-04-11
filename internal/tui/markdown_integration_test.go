package tui

import (
	"regexp"
	"strings"
	"testing"
)

var integrationAnsiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripIntegrationANSI(s string) string {
	return integrationAnsiPattern.ReplaceAllString(s, "")
}

func TestMarkdownHeadingRemovalIntegration(t *testing.T) {
	theme := Theme{Name: "dark"}
	renderer := NewMarkdownRenderer(theme)

	input := `# Main Title

This is a paragraph.

## Section One

Content under section one.

### Subsection 1.1

Detailed content here.

#### Deep Heading

Even more details.

##### Very Deep Heading

Final level.

###### Deepest Heading

Bottom level.

Normal text without headings should remain unchanged.

### Another Third-Level Heading

With **bold** and *italic* text.

---

## Another Section

More content.`

	output := renderer.RenderWithStyle(input, 80)
	clean := stripIntegrationANSI(output)

	verifications := []struct {
		name     string
		check    func(string) bool
		expected bool
	}{
		{
			name:     "no ### markers",
			check:    func(s string) bool { return !strings.Contains(s, "###") },
			expected: true,
		},
		{
			name:     "no ## markers",
			check:    func(s string) bool { return !strings.Contains(s, "##") },
			expected: true,
		},
		{
			name:     "no # markers (except for comments)",
			check:    func(s string) bool { return !strings.Contains(s, "# ") },
			expected: true,
		},
		{
			name:     "Main Title text preserved",
			check:    func(s string) bool { return strings.Contains(s, "Main Title") },
			expected: true,
		},
		{
			name:     "Section One text preserved",
			check:    func(s string) bool { return strings.Contains(s, "Section One") },
			expected: true,
		},
		{
			name:     "Subsection text preserved",
			check:    func(s string) bool { return strings.Contains(s, "Subsection 1.1") },
			expected: true,
		},
		{
			name:     "Deep Heading text preserved",
			check:    func(s string) bool { return strings.Contains(s, "Deep Heading") },
			expected: true,
		},
		{
			name:     "normal text preserved",
			check:    func(s string) bool { return strings.Contains(s, "Normal text without headings") },
			expected: true,
		},
		{
			name:     "bold text preserved",
			check:    func(s string) bool { return strings.Contains(s, "bold") },
			expected: true,
		},
		{
			name:     "italic text preserved",
			check:    func(s string) bool { return strings.Contains(s, "italic") },
			expected: true,
		},
	}

	for _, v := range verifications {
		result := v.check(clean)
		if result != v.expected {
			t.Errorf("%s: expected %v, got %v", v.name, v.expected, result)
		} else {
			t.Logf("✓ %s", v.name)
		}
	}
}

func TestMarkdownRendererAllMethods(t *testing.T) {
	theme := Theme{Name: "dark"}
	renderer := NewMarkdownRenderer(theme)

	input := "### Test Heading"

	t.Run("Render", func(t *testing.T) {
		output := renderer.Render(input)
		clean := stripIntegrationANSI(output)
		if strings.Contains(clean, "###") {
			t.Errorf("Render() should not contain ###, got: %q", clean)
		}
		if !strings.Contains(clean, "Test Heading") {
			t.Errorf("Render() should contain heading text, got: %q", clean)
		}
	})

	t.Run("RenderWithStyle", func(t *testing.T) {
		output := renderer.RenderWithStyle(input, 80)
		clean := stripIntegrationANSI(output)
		if strings.Contains(clean, "###") {
			t.Errorf("RenderWithStyle() should not contain ###, got: %q", clean)
		}
		if !strings.Contains(clean, "Test Heading") {
			t.Errorf("RenderWithStyle() should contain heading text, got: %q", clean)
		}
	})

	t.Run("RenderInline", func(t *testing.T) {
		output := renderer.RenderInline(input)
		clean := stripIntegrationANSI(output)
		if strings.Contains(clean, "###") {
			t.Errorf("RenderInline() should not contain ###, got: %q", clean)
		}
		if !strings.Contains(clean, "Test Heading") {
			t.Errorf("RenderInline() should contain heading text, got: %q", clean)
		}
	})
}
