package tui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HistorySearchMode struct {
	active       bool
	query        string
	results      []SearchResult
	selectedIdx  int
	resultOffset int
	pageSize     int
}

type SearchResult struct {
	Text       string
	MatchStart int
	MatchEnd   int
	Index      int
}

func NewHistorySearchMode() *HistorySearchMode {
	return &HistorySearchMode{
		active:       false,
		query:        "",
		results:      make([]SearchResult, 0),
		selectedIdx:  0,
		resultOffset: 0,
		pageSize:     8,
	}
}

func (h *HistorySearchMode) Activate() {
	h.active = true
	h.query = ""
	h.results = make([]SearchResult, 0)
	h.selectedIdx = 0
	h.resultOffset = 0
}

func (h *HistorySearchMode) Deactivate() {
	h.active = false
	h.query = ""
	h.results = make([]SearchResult, 0)
	h.selectedIdx = 0
	h.resultOffset = 0
}

func (h *HistorySearchMode) IsActive() bool {
	return h.active
}

func (h *HistorySearchMode) AppendQuery(char string) {
	h.query += char
}

func (h *HistorySearchMode) Backspace() {
	if len(h.query) > 0 {
		h.query = h.query[:len(h.query)-1]
	}
}

func (h *HistorySearchMode) GetQuery() string {
	return h.query
}

func (h *HistorySearchMode) Search(history []string) {
	if h.query == "" {
		h.results = make([]SearchResult, 0)
		return
	}

	var results []SearchResult

	isRegex := false
	var re *regexp.Regexp
	var err error

	if strings.HasPrefix(h.query, "/") && len(h.query) > 1 {
		regexPattern := h.query[1:]
		re, err = regexp.Compile("(?i)" + regexPattern)
		if err == nil {
			isRegex = true
		}
	}

	queryLower := strings.ToLower(h.query)

	for i := len(history) - 1; i >= 0; i-- {
		entry := history[i]
		var matchStart, matchEnd int
		var matched bool

		if isRegex {
			loc := re.FindStringIndex(entry)
			if loc != nil {
				matched = true
				matchStart = loc[0]
				matchEnd = loc[1]
			}
		} else {
			idx := strings.Index(strings.ToLower(entry), queryLower)
			if idx >= 0 {
				matched = true
				matchStart = idx
				matchEnd = idx + len(h.query)
			}
		}

		if matched {
			results = append(results, SearchResult{
				Text:       entry,
				MatchStart: matchStart,
				MatchEnd:   matchEnd,
				Index:      i,
			})
		}
	}

	h.results = results
	h.selectedIdx = 0
	h.resultOffset = 0
}

func (h *HistorySearchMode) NextResult() {
	if len(h.results) == 0 {
		return
	}

	h.selectedIdx++
	if h.selectedIdx >= len(h.results) {
		h.selectedIdx = 0
		h.resultOffset = 0
	}

	if h.selectedIdx >= h.resultOffset+h.pageSize {
		h.resultOffset = h.selectedIdx - h.pageSize + 1
	}
}

func (h *HistorySearchMode) PrevResult() {
	if len(h.results) == 0 {
		return
	}

	h.selectedIdx--
	if h.selectedIdx < 0 {
		h.selectedIdx = len(h.results) - 1
		h.resultOffset = max(0, len(h.results)-h.pageSize)
	}

	if h.selectedIdx < h.resultOffset {
		h.resultOffset = h.selectedIdx
	}
}

func (h *HistorySearchMode) GetSelectedResult() *SearchResult {
	if len(h.results) == 0 || h.selectedIdx < 0 || h.selectedIdx >= len(h.results) {
		return nil
	}
	return &h.results[h.selectedIdx]
}

func (h *HistorySearchMode) HasResults() bool {
	return len(h.results) > 0
}

func (h *HistorySearchMode) Render(theme Theme, width int) string {
	if !h.active {
		return ""
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Padding(0, 1)

	searchStyle := lipgloss.NewStyle().
		Foreground(theme.Info).
		Bold(true)

	sb.WriteString(titleStyle.Render("🔍 History Search"))
	sb.WriteString("\n")

	queryLine := searchStyle.Render("Query: ") + h.query
	sb.WriteString(queryLine)

	if !h.HasResults() && h.query != "" {
		mutedStyle := lipgloss.NewStyle().Foreground(theme.TextMuted)
		sb.WriteString("\n")
		sb.WriteString(mutedStyle.Render("No matches found"))
	} else if h.HasResults() {
		sb.WriteString(fmt.Sprintf(" (%d results)", len(h.results)))
		sb.WriteString("\n\n")

		end := h.resultOffset + h.pageSize
		if end > len(h.results) {
			end = len(h.results)
		}

		for i := h.resultOffset; i < end; i++ {
			result := h.results[i]
			line := h.renderResult(result, i == h.selectedIdx, theme, width)
			sb.WriteString(line)
			sb.WriteString("\n")
		}

		if len(h.results) > h.pageSize {
			pageInfo := fmt.Sprintf("\nPage %d/%d",
				(h.resultOffset/h.pageSize)+1,
				(len(h.results)+h.pageSize-1)/h.pageSize)
			mutedStyle := lipgloss.NewStyle().Foreground(theme.TextMuted)
			sb.WriteString(mutedStyle.Render(pageInfo))
		}
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Padding(1, 0)

	sb.WriteString("\n")
	sb.WriteString(helpStyle.Render("Ctrl+R: Next | ↑↓: Navigate | Enter: Select | Esc: Cancel"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1).
		Width(width - 4)

	return boxStyle.Render(sb.String())
}

func (h *HistorySearchMode) renderResult(result SearchResult, isSelected bool, theme Theme, width int) string {
	text := result.Text
	if len(text) > width-10 {
		text = text[:width-13] + "..."
	}

	var rendered string
	if result.MatchEnd > 0 {
		before := text[:result.MatchStart]
		match := text[result.MatchStart:result.MatchEnd]
		after := text[result.MatchEnd:]

		highlightStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true).
			Background(lipgloss.Color("#333333"))

		rendered = before + highlightStyle.Render(match) + after
	} else {
		rendered = text
	}

	itemStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 6)

	selectedStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(width - 6).
		Background(theme.Primary).
		Foreground(lipgloss.Color("#FFFFFF"))

	if isSelected {
		indicator := lipgloss.NewStyle().
			Foreground(theme.Success).
			Bold(true).
			Render("▶")
		return indicator + selectedStyle.Render(rendered)
	}

	indicator := lipgloss.NewStyle().Render(" ")
	return indicator + itemStyle.Render(rendered)
}

func (m Model) updateHistorySearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.historySearch.Deactivate()
			return m, nil

		case tea.KeyEnter:
			if m.historySearch.HasResults() {
				selected := m.historySearch.GetSelectedResult()
				if selected != nil {
					m.textArea.SetValue(selected.Text)
					m.historySearch.Deactivate()
				}
			}
			return m, nil

		case tea.KeyCtrlR:
			m.historySearch.NextResult()
			return m, nil

		case tea.KeyUp:
			m.historySearch.PrevResult()
			return m, nil

		case tea.KeyDown:
			m.historySearch.NextResult()
			return m, nil

		case tea.KeyBackspace, tea.KeyDelete:
			m.historySearch.Backspace()
			m.historySearch.Search(m.history)
			return m, nil

		default:
			if msg.Type == tea.KeyRunes {
				m.historySearch.AppendQuery(string(msg.Runes))
				m.historySearch.Search(m.history)
				return m, nil
			}
		}

	case tea.MouseMsg:
		totalLines := 0
		for _, outputMsg := range m.output {
			totalLines += len(strings.Split(outputMsg, "\n"))
		}
		estimatedHeight := m.height - 10
		if estimatedHeight <= 0 {
			estimatedHeight = 20
		}
		maxOffset := max(0, totalLines-estimatedHeight)

		switch msg.Type {
		case tea.MouseWheelUp:
			m.viewportOffset = max(0, m.viewportOffset-3)
		case tea.MouseWheelDown:
			m.viewportOffset = min(m.viewportOffset+3, maxOffset)
		}
		return m, nil
	}

	return m, nil
}
