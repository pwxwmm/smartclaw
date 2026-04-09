package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DialogType int

const (
	DialogInfo DialogType = iota
	DialogWarning
	DialogError
	DialogSuccess
	DialogConfirm
	DialogSelect
)

type Dialog struct {
	title      string
	message    string
	dialogType DialogType
	choices    []string
	selected   int
	onConfirm  func(bool)
	onSelect   func(int)
	width      int
	height     int
}

var dialogBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.DoubleBorder()).
	BorderForeground(lipgloss.Color("#7C3AED")).
	Padding(1, 2).
	Margin(1, 2)

var dialogTitleStyle = lipgloss.NewStyle().
	Bold(true).
	Padding(0, 1)

var dialogMessageStyle = lipgloss.NewStyle().
	Padding(1, 0)

var dialogButtonStyle = lipgloss.NewStyle().
	Padding(0, 3).
	Margin(0, 1)

var dialogButtonActiveStyle = lipgloss.NewStyle().
	Padding(0, 3).
	Margin(0, 1).
	Background(lipgloss.Color("#7C3AED")).
	Foreground(lipgloss.Color("#FFFFFF"))

func NewDialog(title, message string, dialogType DialogType) *Dialog {
	return &Dialog{
		title:      title,
		message:    message,
		dialogType: dialogType,
		choices:    []string{"OK"},
		selected:   0,
		width:      50,
		height:     10,
	}
}

func NewConfirmDialog(title, message string, onConfirm func(bool)) *Dialog {
	return &Dialog{
		title:      title,
		message:    message,
		dialogType: DialogConfirm,
		choices:    []string{"Yes", "No"},
		selected:   0,
		onConfirm:  onConfirm,
		width:      50,
		height:     10,
	}
}

func NewSelectDialog(title, message string, choices []string, onSelect func(int)) *Dialog {
	return &Dialog{
		title:      title,
		message:    message,
		dialogType: DialogSelect,
		choices:    choices,
		selected:   0,
		onSelect:   onSelect,
		width:      50,
		height:     len(choices) + 8,
	}
}

func (d *Dialog) Init() tea.Cmd {
	return nil
}

func (d *Dialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			if d.selected > 0 {
				d.selected--
			}
		case "right", "l":
			if d.selected < len(d.choices)-1 {
				d.selected++
			}
		case "up", "k":
			if d.dialogType == DialogSelect && d.selected > 0 {
				d.selected--
			}
		case "down", "j":
			if d.dialogType == DialogSelect && d.selected < len(d.choices)-1 {
				d.selected++
			}
		case "enter":
			return d, tea.Quit
		case "escape":
			d.selected = -1
			return d, tea.Quit
		}
	}

	return d, nil
}

func (d *Dialog) View() string {
	var sb strings.Builder

	icon := d.getIcon()
	titleColor := d.getTitleColor()

	sb.WriteString(dialogTitleStyle.
		Foreground(titleColor).
		Render(icon + " " + d.title))
	sb.WriteString("\n")
	sb.WriteString(dialogMessageStyle.Render(d.message))
	sb.WriteString("\n\n")

	if d.dialogType == DialogSelect {
		for i, choice := range d.choices {
			if i == d.selected {
				sb.WriteString("▶ " + dialogButtonActiveStyle.Render(choice) + "\n")
			} else {
				sb.WriteString("  " + dialogButtonStyle.Render(choice) + "\n")
			}
		}
	} else {
		sb.WriteString(d.renderButtons())
	}

	return dialogBoxStyle.
		Width(d.width).
		Render(sb.String())
}

func (d *Dialog) getIcon() string {
	switch d.dialogType {
	case DialogInfo:
		return "ℹ️"
	case DialogWarning:
		return "⚠️"
	case DialogError:
		return "❌"
	case DialogSuccess:
		return "✅"
	case DialogConfirm:
		return "❓"
	case DialogSelect:
		return "📋"
	default:
		return "💬"
	}
}

func (d *Dialog) getTitleColor() lipgloss.Color {
	switch d.dialogType {
	case DialogInfo:
		return lipgloss.Color("#3B82F6")
	case DialogWarning:
		return lipgloss.Color("#F59E0B")
	case DialogError:
		return lipgloss.Color("#EF4444")
	case DialogSuccess:
		return lipgloss.Color("#10B981")
	case DialogConfirm:
		return lipgloss.Color("#8B5CF6")
	default:
		return lipgloss.Color("#7C3AED")
	}
}

func (d *Dialog) renderButtons() string {
	buttons := make([]string, len(d.choices))
	for i, choice := range d.choices {
		if i == d.selected {
			buttons[i] = dialogButtonActiveStyle.Render(choice)
		} else {
			buttons[i] = dialogButtonStyle.Render(choice)
		}
	}
	return strings.Join(buttons, "  ")
}

func (d *Dialog) GetResult() int {
	return d.selected
}

func (d *Dialog) GetConfirmed() bool {
	return d.selected == 0
}
