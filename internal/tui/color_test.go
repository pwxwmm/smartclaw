package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func RunColorTest() error {
	testMarkdown := `Here's a Python HTTP example:

` + "```python" + `
import requests

response = requests.get('https://api.example.com')
print(response.status_code)
` + "```" + `

That's how you make an HTTP request.`

	model := InitialModel()
	model.ready = true
	model.width = 80
	model.height = 24

	output := model.formatAssistantOutput(testMarkdown)
	model.output = append(model.output, output)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	fmt.Println("=== TUI Color Test ===")
	fmt.Println("If you see colors in the code block above, syntax highlighting is working!")
	fmt.Println()
	fmt.Println("Press Ctrl+C to exit")
	fmt.Println()

	m, err := p.Run()
	if err != nil {
		return err
	}

	_ = m
	return nil
}
