package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type ColorTestModel struct {
	width  int
	height int
	ready  bool
	output string
}

func InitialColorTestModel() ColorTestModel {
	testOutput := `┌─ Color Test
│
│  This should show colors:
│  [38;5;204mMagenta keyword[0m
│  [38;5;212mPurple function[0m
│  [38;5;173mOrange string[0m
│  [38;5;251mGray variable[0m
│
│  Raw ANSI test:
│  ` + "\x1b[38;5;204m" + `import requests` + "\x1b[0m" + `
│  ` + "\x1b[38;5;212m" + `print("hello")` + "\x1b[0m" + `
│
└─`

	return ColorTestModel{
		output: testOutput,
	}
}

func (m ColorTestModel) Init() tea.Cmd {
	return nil
}

func (m ColorTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ColorTestModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sb strings.Builder

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Render("=== ANSI Color Test in TUI ===")

	sb.WriteString(header)
	sb.WriteString("\n\n")
	sb.WriteString("If you see colors below, TUI rendering works:\n\n")
	sb.WriteString(m.output)
	sb.WriteString("\n\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Ctrl+C to exit"))

	return sb.String()
}

func RunColorTestTUI() error {
	lipgloss.SetColorProfile(termenv.TrueColor)

	p := tea.NewProgram(
		InitialColorTestModel(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
