package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type ColorTestModel2 struct {
	width  int
	height int
	ready  bool
	output string
}

func InitialColorTestModel2() ColorTestModel2 {
	pythonCode := `这是一个 Python 示例：

` + "```python" + `
import requests

response = requests.get('https://api.example.com')
print(response.status_code)
` + "```" + `

这就是如何发送 HTTP 请求。`

	renderer := NewMarkdownRenderer(GetTheme())
	rendered := renderer.RenderWithStyle(pythonCode, 80)

	return ColorTestModel2{
		output: rendered,
	}
}

func (m ColorTestModel2) Init() tea.Cmd {
	return nil
}

func (m ColorTestModel2) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m ColorTestModel2) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sb strings.Builder

	sb.WriteString("=== DIRECT GLAMOUR OUTPUT TEST ===")
	sb.WriteString("\n\n")
	sb.WriteString("This test shows glamour output directly (no lipgloss, no processing):\n\n")

	sb.WriteString(m.output)

	sb.WriteString("\n\n")
	sb.WriteString("If you see colors above, glamour works. If not, there's a rendering issue.\n\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Ctrl+C to exit"))

	return sb.String()
}

func RunColorTestTUI2() error {
	lipgloss.SetColorProfile(termenv.TrueColor)

	p := tea.NewProgram(
		InitialColorTestModel2(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
