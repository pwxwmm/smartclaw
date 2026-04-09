package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type StreamMsg struct {
	Content string
	Done    bool
}

type StreamModel struct {
	content    string
	done       bool
	speed      time.Duration
	typewriter bool
	cursorPos  int
	theme      Theme
}

func NewStreamModel() StreamModel {
	return StreamModel{
		content:    "",
		done:       false,
		speed:      time.Millisecond * 20,
		typewriter: true,
		cursorPos:  0,
		theme:      GetTheme(),
	}
}

func (m StreamModel) Init() tea.Cmd {
	return nil
}

func (m StreamModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case StreamMsg:
		if m.typewriter && !msg.Done {
			return m, tea.Tick(m.speed, func(t time.Time) tea.Msg {
				return StreamMsg{Content: msg.Content, Done: false}
			})
		}
		m.content = msg.Content
		m.done = msg.Done
		return m, nil
	case time.Time:
		if m.cursorPos < len(m.content) {
			m.cursorPos++
			return m, tea.Tick(m.speed, func(t time.Time) tea.Msg {
				return time.Time(t)
			})
		}
	}

	return m, nil
}

func (m StreamModel) View() string {
	if m.done {
		return m.content
	}

	displayContent := m.content
	if m.cursorPos < len(m.content) {
		displayContent = m.content[:m.cursorPos]
	}

	cursor := "▊"
	if !m.done {
		displayContent += cursor
	}

	return m.theme.OutputStyle().Render(displayContent)
}

func (m *StreamModel) Append(content string) {
	m.content += content
}

func (m *StreamModel) SetContent(content string) {
	m.content = content
}

func (m *StreamModel) SetDone(done bool) {
	m.done = done
}

func (m *StreamModel) Clear() {
	m.content = ""
	m.done = false
	m.cursorPos = 0
}

type StreamingOutput struct {
	lines     []string
	current   string
	streaming bool
	spinner   *Spinner
	prefix    string
}

func NewStreamingOutput() *StreamingOutput {
	return &StreamingOutput{
		lines:     make([]string, 0),
		current:   "",
		streaming: false,
		spinner:   NewSpinner(""),
		prefix:    "Smart: ",
	}
}

func (s *StreamingOutput) Start() {
	s.streaming = true
	s.current = ""
}

func (s *StreamingOutput) Append(text string) {
	s.current += text
}

func (s *StreamingOutput) Done() string {
	s.streaming = false
	return s.current
}

func (s *StreamingOutput) Render() string {
	theme := GetTheme()

	if s.streaming {
		prefix := theme.SuccessStyle().Render(s.prefix)
		spinner := s.spinner.Next()
		return prefix + " " + spinner + " " + s.current
	}

	return theme.SuccessStyle().Render(s.prefix) + " " + s.current
}

func (s *StreamingOutput) GetLines() []string {
	return s.lines
}

func (s *StreamingOutput) Clear() {
	s.lines = make([]string, 0)
	s.current = ""
	s.streaming = false
}
