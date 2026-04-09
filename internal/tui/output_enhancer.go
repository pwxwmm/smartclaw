package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type OutputMessage struct {
	Content    string
	Role       string
	Timestamp  time.Time
	Bookmarked bool
	Folded     bool
	HasCode    bool
	ID         string
}

type OutputMode int

const (
	OutputModeAll OutputMode = iota
	OutputModeCode
	OutputModeText
)

type OutputEnhancer struct {
	messages      []OutputMessage
	mode          OutputMode
	bookmarksOnly bool
	showTimestamp bool
}

func NewOutputEnhancer() *OutputEnhancer {
	return &OutputEnhancer{
		messages:      make([]OutputMessage, 0),
		mode:          OutputModeAll,
		bookmarksOnly: false,
		showTimestamp: true,
	}
}

func (oe *OutputEnhancer) AddMessage(content, role string) string {
	msg := OutputMessage{
		Content:    content,
		Role:       role,
		Timestamp:  time.Now(),
		Bookmarked: false,
		Folded:     false,
		HasCode:    oe.detectCodeBlocks(content),
		ID:         generateMessageID(),
	}
	oe.messages = append(oe.messages, msg)
	return msg.ID
}

func (oe *OutputEnhancer) detectCodeBlocks(content string) bool {
	return strings.Contains(content, "```")
}

func (oe *OutputEnhancer) ToggleBookmark(id string) bool {
	for i := range oe.messages {
		if oe.messages[i].ID == id {
			oe.messages[i].Bookmarked = !oe.messages[i].Bookmarked
			return true
		}
	}
	return false
}

func (oe *OutputEnhancer) ToggleFold(id string) bool {
	for i := range oe.messages {
		if oe.messages[i].ID == id && oe.messages[i].HasCode {
			oe.messages[i].Folded = !oe.messages[i].Folded
			return true
		}
	}
	return false
}

func (oe *OutputEnhancer) SetMode(mode OutputMode) {
	oe.mode = mode
}

func (oe *OutputEnhancer) ToggleBookmarksOnly() {
	oe.bookmarksOnly = !oe.bookmarksOnly
}

func (oe *OutputEnhancer) ToggleTimestamps() {
	oe.showTimestamp = !oe.showTimestamp
}

func (oe *OutputEnhancer) GetFilteredMessages() []OutputMessage {
	var filtered []OutputMessage

	for _, msg := range oe.messages {
		if oe.bookmarksOnly && !msg.Bookmarked {
			continue
		}

		switch oe.mode {
		case OutputModeCode:
			if !msg.HasCode {
				continue
			}
		case OutputModeText:
			if msg.HasCode {
				continue
			}
		}

		filtered = append(filtered, msg)
	}

	return filtered
}

func (oe *OutputEnhancer) RenderMessage(msg OutputMessage, theme Theme, width int) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true)

	if msg.Role == "user" {
		headerStyle = headerStyle.Foreground(theme.Info)
	} else {
		headerStyle = headerStyle.Foreground(theme.Primary)
	}

	var header string
	if oe.showTimestamp {
		timestamp := msg.Timestamp.Format("15:04")
		header = fmt.Sprintf("▶ %s [%s]", msg.Role, timestamp)
	} else {
		header = fmt.Sprintf("▶ %s", msg.Role)
	}

	if msg.Bookmarked {
		bookmarkStyle := lipgloss.NewStyle().Foreground(theme.Warning)
		header = bookmarkStyle.Render("★ ") + header
	}

	sb.WriteString(headerStyle.Render(header))
	sb.WriteString("\n")

	if msg.Folded && msg.HasCode {
		content := oe.renderFoldedContent(msg, theme, width)
		sb.WriteString(content)
	} else {
		sb.WriteString(msg.Content)
	}

	sb.WriteString("\n")

	return sb.String()
}

func (oe *OutputEnhancer) renderFoldedContent(msg OutputMessage, theme Theme, width int) string {
	codeBlockCount := strings.Count(msg.Content, "```") / 2
	lines := strings.Count(msg.Content, "\n") + 1

	foldStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1).
		Width(width - 4)

	foldText := fmt.Sprintf("📁 %d code blocks (%d lines) - Press 'f' to expand", codeBlockCount, lines)

	mutedStyle := lipgloss.NewStyle().Foreground(theme.TextMuted)
	return foldStyle.Render(mutedStyle.Render(foldText))
}

func (oe *OutputEnhancer) GetStats() (total, bookmarked, withCode int) {
	for _, msg := range oe.messages {
		total++
		if msg.Bookmarked {
			bookmarked++
		}
		if msg.HasCode {
			withCode++
		}
	}
	return
}
