package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type ColorTestModel3 struct {
	width   int
	height  int
	ready   bool
	output  string
	printed bool
}

func InitialColorTestModel3() ColorTestModel3 {
	pythonCode := `这是一个 Python 示例：

` + "```python" + `
import requests

response = requests.get('https://api.example.com')
print(response.status_code)
` + "```" + `

这就是如何发送 HTTP 请求。`

	renderer := NewMarkdownRenderer(GetTheme())
	rendered := renderer.RenderWithStyle(pythonCode, 80)

	return ColorTestModel3{
		output:  rendered,
		printed: false,
	}
}

func (m ColorTestModel3) Init() tea.Cmd {
	return nil
}

func (m ColorTestModel3) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyCtrlD {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		if !m.printed {
			m.printed = true
			return m, tea.Println(m.output)
		}
	}

	return m, nil
}

func (m ColorTestModel3) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var sb strings.Builder

	sb.WriteString("=== tea.Println TEST ===")
	sb.WriteString("\n\n")
	sb.WriteString("Content was sent via tea.Println (bypasses View() filtering).\n")
	sb.WriteString("If you see colors above, this is the solution!\n\n")
	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Ctrl+C to exit"))

	return sb.String()
}

func RunColorTestTUI3() error {
	lipgloss.SetColorProfile(termenv.TrueColor)

	p := tea.NewProgram(
		InitialColorTestModel3(),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	return err
}
