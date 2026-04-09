package tui

import (
	"regexp"
	"strings"

	"github.com/atotto/clipboard"
)

// Selection represents a text selection in the output
type Selection struct {
	StartLine int
	EndLine   int
	StartCol  int
	EndCol    int
	Active    bool
}

// RemoveANSIColors removes all ANSI color codes from a string
func RemoveANSIColors(text string) string {
	// ANSI color code pattern: \x1b[ followed by numbers and semicolons, ending with m
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return ansiPattern.ReplaceAllString(text, "")
}

// CopyToClipboard copies text to the system clipboard
func CopyToClipboard(text string) error {
	// Remove ANSI color codes before copying
	cleanText := RemoveANSIColors(text)
	return clipboard.WriteAll(cleanText)
}

// GetVisibleText extracts the currently visible text from the output
func (m *Model) GetVisibleText(height int) string {
	if len(m.rawOutput) == 0 {
		return ""
	}

	// Calculate total lines
	totalLines := 0
	for _, msg := range m.rawOutput {
		totalLines += len(strings.Split(msg, "\n"))
	}

	// Calculate visible range
	maxOffset := max(0, totalLines-height)
	if m.viewportOffset > maxOffset {
		m.viewportOffset = maxOffset
	}

	startLine := m.viewportOffset
	endLine := startLine + height
	if endLine > totalLines {
		endLine = totalLines
	}

	// Extract visible lines
	var visibleLines []string
	currentLine := 0
	for _, msg := range m.rawOutput {
		msgLines := strings.Split(msg, "\n")
		for _, line := range msgLines {
			if currentLine >= startLine && currentLine < endLine {
				visibleLines = append(visibleLines, line)
			}
			currentLine++
		}
	}

	return strings.Join(visibleLines, "\n")
}

// GetLastMessage returns the last assistant message
func (m *Model) GetLastMessage() string {
	if len(m.rawOutput) == 0 {
		return ""
	}

	// Return the last message (which should be the latest AI response)
	return m.rawOutput[len(m.rawOutput)-1]
}

// GetAllMessages returns all messages in the output
func (m *Model) GetAllMessages() string {
	if len(m.rawOutput) == 0 {
		return ""
	}

	return strings.Join(m.rawOutput, "\n\n")
}

// ExtractCodeBlocks extracts all code blocks from text
func ExtractCodeBlocks(text string) []string {
	// Match code blocks with language: ```lang\ncode\n```
	codeBlockPattern := regexp.MustCompile("`{3}[\\w]*\\n([\\s\\S]*?)\\n`{3}")
	matches := codeBlockPattern.FindAllStringSubmatch(text, -1)

	var blocks []string
	for _, match := range matches {
		if len(match) > 1 {
			blocks = append(blocks, match[1])
		}
	}

	return blocks
}

// GetLastCodeBlock returns the last code block from the last message
func (m *Model) GetLastCodeBlock() string {
	lastMsg := m.GetLastMessage()
	blocks := ExtractCodeBlocks(lastMsg)

	if len(blocks) > 0 {
		return blocks[len(blocks)-1]
	}

	return ""
}
