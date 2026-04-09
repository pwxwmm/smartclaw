package screens

import (
	"fmt"
	"strings"
)

type Screen interface {
	Render() string
	Clear() string
}

type BaseScreen struct {
	Title   string
	Content strings.Builder
	Lines   []string
	CursorY int
	CursorX int
}

func NewScreen(title string) *BaseScreen {
	return &BaseScreen{
		Title:   title,
		Lines:   make([]string, 0),
		CursorY: 0,
		CursorX: 0,
	}
}

func (s *BaseScreen) SetTitle(title string) {
	s.Title = title
}

func (s *BaseScreen) AddLine(line string) {
	s.Lines = append(s.Lines, line)
}

func (s *BaseScreen) AddLines(lines []string) {
	s.Lines = append(s.Lines, lines...)
}

func (s *BaseScreen) Render() string {
	var sb strings.Builder
	sb.WriteString(s.header())
	sb.WriteString(s.content())
	sb.WriteString(s.footer())
	return sb.String()
}

func (s *BaseScreen) Clear() string {
	return "\033[2J\033[H"
}

func (s *BaseScreen) header() string {
	return fmt.Sprintf("┌─ %s ─%s┐\n", s.Title, strings.Repeat("─", 60-len(s.Title)-6))
}

func (s *BaseScreen) content() string {
	var sb strings.Builder
	for i, line := range s.Lines {
		prefix := "│"
		if i == s.CursorY {
			prefix = "▶"
		}
		sb.WriteString(fmt.Sprintf("%s %s %s\n", prefix, line, strings.Repeat(" ", 60-len(line)-3)))
	}
	return sb.String()
}

func (s *BaseScreen) footer() string {
	return fmt.Sprintf("└%s┘\n", strings.Repeat("─", 61))
}

type MenuScreen struct {
	*BaseScreen
	Items    []string
	Selected int
	OnSelect func(int)
}

func NewMenuScreen(title string) *MenuScreen {
	return &MenuScreen{
		BaseScreen: NewScreen(title),
		Items:      make([]string, 0),
		Selected:   0,
	}
}

func (m *MenuScreen) AddItem(item string) {
	m.Items = append(m.Items, item)
}

func (m *MenuScreen) MoveUp() {
	if m.Selected > 0 {
		m.Selected--
	}
}

func (m *MenuScreen) MoveDown() {
	if m.Selected < len(m.Items)-1 {
		m.Selected++
	}
}

func (m *MenuScreen) Select() {
	if m.OnSelect != nil {
		m.OnSelect(m.Selected)
	}
}

func (m *MenuScreen) Render() string {
	m.Lines = m.Items
	return m.BaseScreen.Render()
}

type EditorScreen struct {
	*BaseScreen
	Content  string
	Modified bool
}

func NewEditorScreen(title string) *EditorScreen {
	return &EditorScreen{
		BaseScreen: NewScreen(title),
		Content:    "",
		Modified:   false,
	}
}

func (e *EditorScreen) SetContent(content string) {
	e.Content = content
	e.Modified = true
}

func (e *EditorScreen) GetContent() string {
	return e.Content
}

func (e *EditorScreen) MarkSaved() {
	e.Modified = false
}

func (e *EditorScreen) Render() string {
	lines := strings.Split(e.Content, "\n")
	e.Lines = lines
	return e.BaseScreen.Render()
}
