package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
	theme    Theme
}

func NewMarkdownRenderer(theme Theme) *MarkdownRenderer {
	stylePath := "dark"
	style := strings.ToLower(theme.Name)

	switch style {
	case "light":
		stylePath = "light"
	case "dark", "midnight", "aurora", "forest", "ocean", "purple", "dracula", "monokai", "nord", "solarized":
		stylePath = "dark"
	default:
		stylePath = "dark"
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(stylePath),
		glamour.WithWordWrap(80),
		glamour.WithColorProfile(termenv.TrueColor),
	)

	if err != nil {
		r, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(80),
			glamour.WithColorProfile(termenv.TrueColor),
		)
	}

	return &MarkdownRenderer{
		renderer: r,
		theme:    theme,
	}
}

func (m *MarkdownRenderer) Render(markdown string) string {
	rendered, err := m.renderer.Render(markdown)
	if err != nil {
		return markdown
	}

	rendered = removeHeadingMarkers(rendered)
	return strings.TrimSpace(rendered)
}

func (m *MarkdownRenderer) RenderWithStyle(markdown string, width int) string {
	style := strings.ToLower(m.theme.Name)

	markdown = AddLanguageSpecifiers(markdown)

	var r *glamour.TermRenderer
	var err error

	switch style {
	case "dark", "midnight", "aurora", "forest", "ocean", "purple", "dracula", "monokai", "nord", "solarized":
		r, err = glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(width),
			glamour.WithColorProfile(termenv.TrueColor),
		)
	case "light":
		r, err = glamour.NewTermRenderer(
			glamour.WithStylePath("light"),
			glamour.WithWordWrap(width),
			glamour.WithColorProfile(termenv.TrueColor),
		)
	default:
		r, err = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
			glamour.WithColorProfile(termenv.TrueColor),
		)
	}

	if err != nil {
		return markdown
	}

	rendered, err := r.Render(markdown)
	if err != nil {
		return markdown
	}

	rendered = removeHeadingMarkers(rendered)

	lines := strings.Split(rendered, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if trimmed != "" || len(cleanedLines) > 0 {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}

	return strings.Join(cleanedLines, "\n")
}

func removeHeadingMarkers(rendered string) string {
	ansiCode := `\x1b\[[0-9;]*m`

	pattern1 := regexp.MustCompile(`(` + ansiCode + `)[#]+ `)
	pattern2 := regexp.MustCompile(`[#]+ (` + ansiCode + `)`)
	plainPattern := regexp.MustCompile(`^\s*[#]+ `)
	resetResetPattern := regexp.MustCompile(`\x1b\[0m\x1b\[0m`)

	lines := strings.Split(rendered, "\n")
	var processedLines []string

	for _, line := range lines {
		processed := pattern1.ReplaceAllString(line, "$1")
		processed = pattern2.ReplaceAllString(processed, "$1")
		processed = plainPattern.ReplaceAllString(processed, "")

		for resetResetPattern.MatchString(processed) {
			processed = resetResetPattern.ReplaceAllString(processed, "\x1b[0m")
		}

		processedLines = append(processedLines, processed)
	}

	return strings.Join(processedLines, "\n")
}

func (m *MarkdownRenderer) RenderInline(text string) string {
	rendered, err := m.renderer.Render(text)
	if err != nil {
		return text
	}

	rendered = removeHeadingMarkers(rendered)
	rendered = strings.TrimSpace(rendered)
	rendered = strings.TrimPrefix(rendered, "\n")
	rendered = strings.TrimSuffix(rendered, "\n")

	return rendered
}

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var lines []string
	var currentLine strings.Builder
	currentWidth := 0

	for _, word := range words {
		wordWidth := lipgloss.Width(word)

		if currentWidth+wordWidth+1 > width && currentWidth > 0 {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentWidth = 0
		}

		if currentWidth > 0 {
			currentLine.WriteString(" ")
			currentWidth++
		}

		currentLine.WriteString(word)
		currentWidth += wordWidth
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}
