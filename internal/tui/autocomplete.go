package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Suggestion struct {
	Text        string
	Description string
	Type        string
}

type SuggestionType int

const (
	SuggestionCommand SuggestionType = iota
	SuggestionFile
	SuggestionCode
	SuggestionHistory
	SuggestionVariable
)

type SuggestionList struct {
	suggestions []Suggestion
	selected    int
	visible     bool
	x           int
	y           int
	maxHeight   int
	maxWidth    int
	theme       Theme
}

func NewSuggestionList(maxHeight, maxWidth int) *SuggestionList {
	return &SuggestionList{
		suggestions: make([]Suggestion, 0),
		selected:    0,
		visible:     false,
		maxHeight:   maxHeight,
		maxWidth:    maxWidth,
		theme:       GetTheme(),
	}
}

func (s *SuggestionList) SetSuggestions(suggestions []Suggestion) {
	s.suggestions = suggestions
	s.selected = 0
	s.visible = len(suggestions) > 0
}

func (s *SuggestionList) SetPosition(x, y int) {
	s.x = x
	s.y = y
}

func (s *SuggestionList) Clear() {
	s.suggestions = make([]Suggestion, 0)
	s.visible = false
	s.selected = 0
}

func (s *SuggestionList) Next() {
	if len(s.suggestions) > 0 {
		s.selected = (s.selected + 1) % len(s.suggestions)
	}
}

func (s *SuggestionList) Prev() {
	if len(s.suggestions) > 0 {
		s.selected = (s.selected - 1 + len(s.suggestions)) % len(s.suggestions)
	}
}

func (s *SuggestionList) GetSelected() Suggestion {
	if s.selected >= 0 && s.selected < len(s.suggestions) {
		return s.suggestions[s.selected]
	}
	return Suggestion{}
}

func (s *SuggestionList) IsVisible() bool {
	return s.visible
}

func (s *SuggestionList) Render() string {
	if !s.visible || len(s.suggestions) == 0 {
		return ""
	}

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.maxWidth)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(s.maxWidth).
		Background(s.theme.Primary).
		Foreground(lipgloss.Color("#FFFFFF"))

	descStyle := lipgloss.NewStyle().
		Foreground(s.theme.TextMuted).
		Italic(true)

	var lines []string
	start := 0
	if len(s.suggestions) > s.maxHeight {
		if s.selected >= s.maxHeight {
			start = s.selected - s.maxHeight + 1
		}
	}

	end := start + s.maxHeight
	if end > len(s.suggestions) {
		end = len(s.suggestions)
	}

	for i := start; i < end; i++ {
		suggestion := s.suggestions[i]
		var line string

		icon := s.getTypeIcon(suggestion.Type)
		text := icon + " " + suggestion.Text

		if i == s.selected {
			line = selectedStyle.Render(text)
		} else {
			line = itemStyle.Render(text)
		}

		if suggestion.Description != "" {
			desc := descStyle.Render(" - " + suggestion.Description)
			line = line + desc
		}

		lines = append(lines, line)
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(s.theme.Border).
		Padding(0, 0)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (s *SuggestionList) getTypeIcon(typeStr string) string {
	switch typeStr {
	case "command":
		return "⚡"
	case "file":
		return "📄"
	case "code":
		return "💻"
	case "history":
		return "📜"
	case "variable":
		return "🔤"
	default:
		return "•"
	}
}

type AutoComplete struct {
	suggestions  *SuggestionList
	commands     []Suggestion
	history      []string
	fileCache    []string
	triggerChars string
	enabled      bool
}

func NewAutoComplete(maxHeight, maxWidth int) *AutoComplete {
	return &AutoComplete{
		suggestions: NewSuggestionList(maxHeight, maxWidth),
		commands: []Suggestion{
			{Text: "/help", Description: "Show help", Type: "command"},
			{Text: "/status", Description: "Show status", Type: "command"},
			{Text: "/model", Description: "Change model", Type: "command"},
			{Text: "/theme", Description: "Change theme", Type: "command"},
			{Text: "/clear", Description: "Clear output", Type: "command"},
			{Text: "/exit", Description: "Exit SmartClaw", Type: "command"},
			{Text: "/voice", Description: "Voice mode", Type: "command"},
			{Text: "/cost", Description: "Show cost", Type: "command"},
			{Text: "/context", Description: "Show context", Type: "command"},
			{Text: "/dialog", Description: "Test dialog", Type: "command"},
			{Text: "/autonomous", Description: "Start autonomous task execution", Type: "command"},
			{Text: "/playbook", Description: "Manage playbooks (list/execute/create)", Type: "command"},
			{Text: "/plan", Description: "Manage persistent plans", Type: "command"},
		},
		history:      make([]string, 0),
		triggerChars: "/",
		enabled:      true,
	}
}

func (a *AutoComplete) Update(input string) {
	if !a.enabled {
		a.suggestions.Clear()
		return
	}

	if len(input) == 0 {
		a.suggestions.Clear()
		return
	}

	var matches []Suggestion

	if strings.HasPrefix(input, "/") {
		for _, cmd := range a.commands {
			if strings.HasPrefix(cmd.Text, input) {
				matches = append(matches, cmd)
			}
		}
	}

	for _, h := range a.history {
		if strings.Contains(h, input) {
			matches = append(matches, Suggestion{
				Text:        h,
				Description: "History",
				Type:        "history",
			})
		}
	}

	a.suggestions.SetSuggestions(matches)
}

func (a *AutoComplete) AddHistory(item string) {
	a.history = append(a.history, item)
	if len(a.history) > 100 {
		a.history = a.history[1:]
	}
}

func (a *AutoComplete) Next() {
	a.suggestions.Next()
}

func (a *AutoComplete) Prev() {
	a.suggestions.Prev()
}

func (a *AutoComplete) GetSelected() Suggestion {
	return a.suggestions.GetSelected()
}

func (a *AutoComplete) IsVisible() bool {
	return a.suggestions.IsVisible()
}

func (a *AutoComplete) Render() string {
	return a.suggestions.Render()
}

func (a *AutoComplete) SetEnabled(enabled bool) {
	a.enabled = enabled
}

func (a *AutoComplete) Clear() {
	a.suggestions.Clear()
}
