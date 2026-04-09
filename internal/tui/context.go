package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
)

type ContextMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Tokens    int       `json:"tokens"`
	Keep      bool      `json:"keep"`
}

type ContextManager struct {
	messages  []ContextMessage
	maxTokens int
}

func NewContextManager(maxTokens int) *ContextManager {
	return &ContextManager{
		messages:  make([]ContextMessage, 0),
		maxTokens: maxTokens,
	}
}

func (cm *ContextManager) AddMessage(role, content string, tokens int) string {
	msg := ContextMessage{
		ID:        generateMessageID(),
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		Tokens:    tokens,
		Keep:      false,
	}
	cm.messages = append(cm.messages, msg)
	return msg.ID
}

func (cm *ContextManager) RemoveMessage(id string) bool {
	for i, msg := range cm.messages {
		if msg.ID == id {
			cm.messages = append(cm.messages[:i], cm.messages[i+1:]...)
			return true
		}
	}
	return false
}

func (cm *ContextManager) KeepMessage(id string) bool {
	for i := range cm.messages {
		if cm.messages[i].ID == id {
			cm.messages[i].Keep = true
			return true
		}
	}
	return false
}

func (cm *ContextManager) UnkeepMessage(id string) bool {
	for i := range cm.messages {
		if cm.messages[i].ID == id {
			cm.messages[i].Keep = false
			return true
		}
	}
	return false
}

func (cm *ContextManager) GetMessages() []ContextMessage {
	return cm.messages
}

func (cm *ContextManager) GetMessage(id string) *ContextMessage {
	for i := range cm.messages {
		if cm.messages[i].ID == id {
			return &cm.messages[i]
		}
	}
	return nil
}

func (cm *ContextManager) Clear() {
	cm.messages = make([]ContextMessage, 0)
}

func (cm *ContextManager) ClearNonKept() {
	var kept []ContextMessage
	for _, msg := range cm.messages {
		if msg.Keep {
			kept = append(kept, msg)
		}
	}
	cm.messages = kept
}

func (cm *ContextManager) GetTokenCount() int {
	total := 0
	for _, msg := range cm.messages {
		total += msg.Tokens
	}
	return total
}

func (cm *ContextManager) GetMessageCount() int {
	return len(cm.messages)
}

func (cm *ContextManager) CompressOldMessages(keepCount int) int {
	if len(cm.messages) <= keepCount {
		return 0
	}

	var kept []ContextMessage
	var toCompress []ContextMessage

	for _, msg := range cm.messages {
		if msg.Keep {
			kept = append(kept, msg)
		} else {
			toCompress = append(toCompress, msg)
		}
	}

	if len(toCompress) <= keepCount {
		return 0
	}

	keepFromEnd := keepCount
	if keepFromEnd > len(toCompress) {
		keepFromEnd = len(toCompress)
	}

	compressed := toCompress[len(toCompress)-keepFromEnd:]
	cm.messages = append(kept, compressed...)

	return len(toCompress) - keepFromEnd
}

func (cm *ContextManager) RenderStats() string {
	theme := GetTheme()

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Underline(true)

	sb.WriteString(titleStyle.Render("📊 Context Usage"))
	sb.WriteString("\n\n")

	tokens := cm.GetTokenCount()
	percentage := float64(tokens) / float64(cm.maxTokens) * 100

	tokenStyle := lipgloss.NewStyle().Foreground(theme.Text)
	sb.WriteString(tokenStyle.Render(fmt.Sprintf("Tokens: %d / %d (%.1f%%)", tokens, cm.maxTokens, percentage)))
	sb.WriteString("\n")

	sb.WriteString(tokenStyle.Render(fmt.Sprintf("Messages: %d", cm.GetMessageCount())))
	sb.WriteString("\n")

	kept := 0
	for _, msg := range cm.messages {
		if msg.Keep {
			kept++
		}
	}
	if kept > 0 {
		keepStyle := lipgloss.NewStyle().Foreground(theme.Success)
		sb.WriteString(keepStyle.Render(fmt.Sprintf("Kept: %d messages", kept)))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(cm.renderUsageBar(theme))

	return sb.String()
}

func (cm *ContextManager) renderUsageBar(theme Theme) string {
	barWidth := 30
	tokens := cm.GetTokenCount()
	filled := int(float64(barWidth) * float64(tokens) / float64(cm.maxTokens))

	if filled > barWidth {
		filled = barWidth
	}

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < barWidth; i++ {
		if i < filled {
			if i < barWidth*70/100 {
				bar.WriteString("█")
			} else if i < barWidth*90/100 {
				bar.WriteString("▓")
			} else {
				bar.WriteString("░")
			}
		} else {
			bar.WriteString("░")
		}
	}
	bar.WriteString("]")

	var color lipgloss.Color
	if percentage := float64(tokens) / float64(cm.maxTokens); percentage > 0.9 {
		color = theme.Error
	} else if percentage > 0.7 {
		color = theme.Warning
	} else {
		color = theme.Success
	}

	style := lipgloss.NewStyle().Foreground(color)
	return style.Render(bar.String())
}

func (cm *ContextManager) RenderMessageList() string {
	theme := GetTheme()

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Underline(true)

	sb.WriteString(titleStyle.Render("📝 Message List"))
	sb.WriteString("\n\n")

	if len(cm.messages) == 0 {
		mutedStyle := lipgloss.NewStyle().Foreground(theme.TextMuted)
		sb.WriteString(mutedStyle.Render("No messages in context"))
		return sb.String()
	}

	for i, msg := range cm.messages {
		sb.WriteString(cm.renderMessageItem(msg, i+1, theme))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (cm *ContextManager) renderMessageItem(msg ContextMessage, index int, theme Theme) string {
	var sb strings.Builder

	idStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Width(8)

	roleStyle := lipgloss.NewStyle().
		Bold(true)

	if msg.Role == "user" {
		roleStyle = roleStyle.Foreground(theme.Info)
	} else {
		roleStyle = roleStyle.Foreground(theme.Primary)
	}

	keepIndicator := " "
	if msg.Keep {
		keepIndicator = "★"
	}

	keepStyle := lipgloss.NewStyle().Foreground(theme.Warning)

	content := msg.Content
	if len(content) > 60 {
		content = content[:57] + "..."
	}
	content = strings.ReplaceAll(content, "\n", " ")

	sb.WriteString(fmt.Sprintf("%s %s %-8s %s %s\n",
		idStyle.Render(msg.ID[:8]),
		keepStyle.Render(keepIndicator),
		roleStyle.Render(msg.Role),
		content,
		lipgloss.NewStyle().Foreground(theme.TextMuted).Render(fmt.Sprintf("(%d tokens)", msg.Tokens))))

	return sb.String()
}

func generateMessageID() string {
	return uuid.New().String()[:8]
}
