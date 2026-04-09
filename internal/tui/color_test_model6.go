package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type ColorTestModel6 struct {
	width  int
	height int
	ready  bool
}

func InitialColorTestModel6() ColorTestModel6 {
	return ColorTestModel6{}
}

func (m ColorTestModel6) Init() tea.Cmd {
	return nil
}

func (m ColorTestModel6) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ColorTestModel6) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sb strings.Builder

	sb.WriteString("=== Test 1: Hardcoded glamour format (known to work) ===\n")
	sb.WriteString("  \x1b[38;5;204m\x1b[0m\x1b[38;5;252m\x1b[0m  \x1b[38;5;204mimport\x1b[0m\x1b[38;5;251m \x1b[0m\x1b[38;5;251mrequests\x1b[0m\n\n")

	sb.WriteString("=== Test 2: Glamour called DIRECTLY in View() ===\n")
	pythonCode := "```python\nimport requests\n```"

	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(80),
		glamour.WithColorProfile(termenv.TrueColor),
	)
	if err != nil {
		sb.WriteString("Error creating renderer: " + err.Error() + "\n")
	} else {
		rendered, err := r.Render(pythonCode)
		if err != nil {
			sb.WriteString("Error rendering: " + err.Error() + "\n")
		} else {
			sb.WriteString(rendered)
		}
	}
	sb.WriteString("\n")

	sb.WriteString("=== Test 3: Glamour via helper function ===\n")
	renderer := NewMarkdownRenderer(GetTheme())
	rendered2 := renderer.RenderWithStyle(pythonCode, 80)
	sb.WriteString(rendered2)
	sb.WriteString("\n\n")

	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Ctrl+C to exit"))

	return sb.String()
}

func RunColorTestTUI6() error {
	lipgloss.SetColorProfile(termenv.TrueColor)

	p := tea.NewProgram(
		InitialColorTestModel6(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
