package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type AgentStatus struct {
	Name      string
	Status    string
	Progress  int
	Message   string
	StartTime int64
}

type AgentStatusLine struct {
	agents   []AgentStatus
	maxName  int
	maxWidth int
}

func NewAgentStatusLine(width int) *AgentStatusLine {
	return &AgentStatusLine{
		agents:   make([]AgentStatus, 0),
		maxName:  15,
		maxWidth: width,
	}
}

func (a *AgentStatusLine) AddAgent(name, status string) {
	a.agents = append(a.agents, AgentStatus{
		Name:   name,
		Status: status,
	})
	if len(name) > a.maxName {
		a.maxName = len(name)
	}
}

func (a *AgentStatusLine) UpdateProgress(name string, progress int, message string) {
	for i, agent := range a.agents {
		if agent.Name == name {
			a.agents[i].Progress = progress
			a.agents[i].Message = message
			break
		}
	}
}

func (a *AgentStatusLine) RemoveAgent(name string) {
	for i, agent := range a.agents {
		if agent.Name == name {
			a.agents = append(a.agents[:i], a.agents[i+1:]...)
			break
		}
	}
}

func (a *AgentStatusLine) Render() string {
	if len(a.agents) == 0 {
		return ""
	}

	var lines []string
	theme := GetTheme()

	for _, agent := range a.agents {
		line := a.renderAgent(agent, theme)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (a *AgentStatusLine) renderAgent(agent AgentStatus, theme Theme) string {
	nameStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Width(a.maxName)

	statusStyle := lipgloss.NewStyle().
		Foreground(theme.Success)

	runningStyle := lipgloss.NewStyle().
		Foreground(theme.Warning)

	idleStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted)

	var statusText string
	switch agent.Status {
	case "running":
		statusText = runningStyle.Render("●")
	case "completed":
		statusText = statusStyle.Render("✓")
	case "error":
		statusText = lipgloss.NewStyle().Foreground(theme.Error).Render("✗")
	default:
		statusText = idleStyle.Render("○")
	}

	progress := ""
	if agent.Progress > 0 {
		bar := NewProgressBar(20, 100)
		bar.SetProgress(float64(agent.Progress))
		progress = bar.Render()
	}

	message := ""
	if agent.Message != "" {
		messageStyle := lipgloss.NewStyle().
			Foreground(theme.TextMuted)
		message = " " + messageStyle.Render(agent.Message)
	}

	return fmt.Sprintf("%s %s %s%s",
		nameStyle.Render(agent.Name),
		statusText,
		progress,
		message,
	)
}

type ContextVisualization struct {
	tokens    int
	maxTokens int
	messages  int
	context   []string
	showGraph bool
	width     int
}

func NewContextVisualization(maxTokens int, width int) *ContextVisualization {
	return &ContextVisualization{
		tokens:    0,
		maxTokens: maxTokens,
		messages:  0,
		context:   make([]string, 0),
		showGraph: true,
		width:     width,
	}
}

func (c *ContextVisualization) SetTokens(tokens int) {
	c.tokens = tokens
}

func (c *ContextVisualization) SetMessages(count int) {
	c.messages = count
}

func (c *ContextVisualization) AddContext(name string) {
	c.context = append(c.context, name)
}

func (c *ContextVisualization) ClearContext() {
	c.context = make([]string, 0)
}

func (c *ContextVisualization) Render() string {
	theme := GetTheme()

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	sb.WriteString(titleStyle.Render("📊 Context"))
	sb.WriteString("\n")

	percentage := float64(c.tokens) / float64(c.maxTokens) * 100

	tokenStyle := lipgloss.NewStyle().
		Foreground(theme.Text)

	sb.WriteString(tokenStyle.Render(fmt.Sprintf("Tokens: %d / %d (%.1f%%)", c.tokens, c.maxTokens, percentage)))
	sb.WriteString("\n")

	sb.WriteString(tokenStyle.Render(fmt.Sprintf("Messages: %d", c.messages)))
	sb.WriteString("\n")

	if c.showGraph {
		sb.WriteString("\n")
		sb.WriteString(c.renderUsageGraph(theme))
	}

	if len(c.context) > 0 {
		sb.WriteString("\n")
		sb.WriteString(c.renderContextList(theme))
	}

	return sb.String()
}

func (c *ContextVisualization) renderUsageGraph(theme Theme) string {
	barWidth := 30
	filled := int(float64(barWidth) * float64(c.tokens) / float64(c.maxTokens))

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
	if percentage := float64(c.tokens) / float64(c.maxTokens); percentage > 0.9 {
		color = theme.Error
	} else if percentage > 0.7 {
		color = theme.Warning
	} else {
		color = theme.Success
	}

	style := lipgloss.NewStyle().Foreground(color)
	return style.Render(bar.String())
}

func (c *ContextVisualization) renderContextList(theme Theme) string {
	labelStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	var items []string
	for _, ctx := range c.context {
		items = append(items, "  • "+ctx)
	}

	return labelStyle.Render("Context:") + "\n" + strings.Join(items, "\n")
}

type StatusBar struct {
	model      string
	tokens     int
	cost       float64
	mode       string
	width      int
	showCost   bool
	showTokens bool
}

func NewStatusBar(width int) *StatusBar {
	return &StatusBar{
		model:      "claude-sonnet-4-5",
		tokens:     0,
		cost:       0.0,
		mode:       "ask",
		width:      width,
		showCost:   true,
		showTokens: true,
	}
}

func (s *StatusBar) SetModel(model string) {
	s.model = model
}

func (s *StatusBar) SetTokens(tokens int) {
	s.tokens = tokens
}

func (s *StatusBar) SetCost(cost float64) {
	s.cost = cost
}

func (s *StatusBar) SetMode(mode string) {
	s.mode = mode
}

func (s *StatusBar) Render() string {
	theme := GetTheme()

	leftStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Padding(0, 1)

	rightStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Padding(0, 1)

	owl := lipgloss.NewStyle().
		Foreground(theme.Success).
		Render("(o,o)")

	left := leftStyle.Render(owl + " SmartClaw")

	var rightParts []string
	if s.showTokens {
		tokensStyle := lipgloss.NewStyle().
			Foreground(theme.Info)
		rightParts = append(rightParts, tokensStyle.Render(fmt.Sprintf("● %d tokens", s.tokens)))
	}
	if s.showCost {
		costStyle := lipgloss.NewStyle().
			Foreground(theme.Warning)
		rightParts = append(rightParts, costStyle.Render(fmt.Sprintf("● $%.4f", s.cost)))
	}

	modeStyle := lipgloss.NewStyle().
		Foreground(theme.Success)
	rightParts = append(rightParts, modeStyle.Render(fmt.Sprintf("● %s", s.mode)))

	right := rightStyle.Render(strings.Join(rightParts, " "))

	spacer := strings.Repeat(" ", max(0, s.width-lipgloss.Width(left)-lipgloss.Width(right)-2))

	return left + spacer + right
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
