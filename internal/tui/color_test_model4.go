package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type ColorTestModel4 struct {
	width       int
	height      int
	ready       bool
	glamourOut  string
	testResults []string
}

func InitialColorTestModel4() ColorTestModel4 {
	pythonCode := "```python\nimport requests\nprint('hello')\n```"

	renderer := NewMarkdownRenderer(GetTheme())
	glamourOut := renderer.RenderWithStyle(pythonCode, 80)

	testResults := []string{}

	testResults = append(testResults, "=== Test 1: Hardcoded ANSI (known to work) ===")
	testResults = append(testResults, "\x1b[38;5;204mimport requests\x1b[0m")
	testResults = append(testResults, "\x1b[38;5;212mprint('hello')\x1b[0m")
	testResults = append(testResults, "")

	testResults = append(testResults, "=== Test 2: Glamour output (should have ANSI) ===")
	testResults = append(testResults, glamourOut)
	testResults = append(testResults, "")

	testResults = append(testResults, "=== Test 3: Byte comparison ===")
	testResults = append(testResults, fmt.Sprintf("Hardcoded first byte: %d (should be 27)", "\x1b"[0]))
	testResults = append(testResults, fmt.Sprintf("Glamour first byte: %d", glamourOut[0]))
	testResults = append(testResults, fmt.Sprintf("Glamour has ESC char: %v", strings.Contains(glamourOut, "\x1b")))
	testResults = append(testResults, "")

	testResults = append(testResults, "=== Test 4: Raw glamour bytes (first 100) ===")
	if len(glamourOut) > 100 {
		testResults = append(testResults, fmt.Sprintf("%q", glamourOut[:100]))
	}

	return ColorTestModel4{
		glamourOut:  glamourOut,
		testResults: testResults,
	}
}

func (m ColorTestModel4) Init() tea.Cmd {
	return nil
}

func (m ColorTestModel4) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ColorTestModel4) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sb strings.Builder

	for _, result := range m.testResults {
		sb.WriteString(result)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Ctrl+C to exit"))

	return sb.String()
}

func RunColorTestTUI4() error {
	lipgloss.SetColorProfile(termenv.TrueColor)

	p := tea.NewProgram(
		InitialColorTestModel4(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
