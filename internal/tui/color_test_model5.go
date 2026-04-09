package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type ColorTestModel5 struct {
	width  int
	height int
	ready  bool
}

func InitialColorTestModel5() ColorTestModel5 {
	return ColorTestModel5{}
}

func (m ColorTestModel5) Init() tea.Cmd {
	return nil
}

func (m ColorTestModel5) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyCtrlD {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	}

	return m, nil
}

func (m ColorTestModel5) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sb strings.Builder

	sb.WriteString("=== Test 1: Hardcoded ANSI (NO leading spaces) ===\n")
	sb.WriteString("\x1b[38;5;204mimport requests\x1b[0m\n")
	sb.WriteString("\x1b[38;5;212mprint('hello')\x1b[0m\n\n")

	sb.WriteString("=== Test 2: Hardcoded ANSI (WITH leading spaces like glamour) ===\n")
	sb.WriteString("  \x1b[38;5;204mimport requests\x1b[0m\n")
	sb.WriteString("  \x1b[38;5;212mprint('hello')\x1b[0m\n\n")

	sb.WriteString("=== Test 3: Hardcoded ANSI (WITH padding spaces before ANSI) ===\n")
	sb.WriteString("  " + strings.Repeat(" ", 76) + "\x1b[38;5;204mimport requests\x1b[0m\n")
	sb.WriteString("  " + strings.Repeat(" ", 76) + "\x1b[38;5;212mprint('hello')\x1b[0m\n\n")

	sb.WriteString("=== Test 4: Exact glamour format ===\n")
	sb.WriteString("  \x1b[38;5;252m" + strings.Repeat(" ", 76) + "\x1b[0m\n")
	sb.WriteString("  \x1b[38;5;204m\x1b[0m\x1b[38;5;252m\x1b[0m  \x1b[38;5;252m\x1b[0m\x1b[38;5;252m\x1b[0m\x1b[38;5;204mimport\x1b[0m\x1b[38;5;251m \x1b[0m\x1b[38;5;251mrequests\x1b[0m\n")
	sb.WriteString("  \x1b[38;5;212m\x1b[0m\x1b[38;5;252m\x1b[0m  \x1b[38;5;252m\x1b[0m\x1b[38;5;212mprint\x1b[0m\x1b[38;5;187m(\x1b[0m\x1b[38;5;173m'hello'\x1b[0m\x1b[38;5;187m)\x1b[0m\n\n")

	sb.WriteString("=== Test 5: Simplified glamour format ===\n")
	sb.WriteString("  \x1b[38;5;204mimport requests\x1b[0m\n")
	sb.WriteString("  \x1b[38;5;212mprint('hello')\x1b[0m\n\n")

	sb.WriteString("=== Analysis ===\n")
	sb.WriteString("If Test 1 has colors but Tests 2-5 don't, the issue is with\n")
	sb.WriteString("how TUI handles ANSI codes after leading whitespace.\n\n")

	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Ctrl+C to exit"))

	return sb.String()
}

func RunColorTestTUI5() error {
	lipgloss.SetColorProfile(termenv.TrueColor)

	p := tea.NewProgram(
		InitialColorTestModel5(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
