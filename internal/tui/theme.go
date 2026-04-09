package tui

import (
	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Name            string
	Primary         lipgloss.Color
	Secondary       lipgloss.Color
	Accent          lipgloss.Color
	Background      lipgloss.Color
	Foreground      lipgloss.Color
	Success         lipgloss.Color
	Warning         lipgloss.Color
	Error           lipgloss.Color
	Info            lipgloss.Color
	Border          lipgloss.Color
	BorderActive    lipgloss.Color
	Title           lipgloss.Color
	Text            lipgloss.Color
	TextMuted       lipgloss.Color
	InputPrompt     lipgloss.Color
	OutputUser      lipgloss.Color
	OutputAssistant lipgloss.Color
}

var Themes = map[string]Theme{
	"midnight": {
		Name:            "midnight",
		Primary:         lipgloss.Color("#7DD3FC"),
		Secondary:       lipgloss.Color("#A5B4FC"),
		Accent:          lipgloss.Color("#C4B5FD"),
		Background:      lipgloss.Color("#0F172A"),
		Foreground:      lipgloss.Color("#F1F5F9"),
		Success:         lipgloss.Color("#34D399"),
		Warning:         lipgloss.Color("#FBBF24"),
		Error:           lipgloss.Color("#F87171"),
		Info:            lipgloss.Color("#60A5FA"),
		Border:          lipgloss.Color("#334155"),
		BorderActive:    lipgloss.Color("#7DD3FC"),
		Title:           lipgloss.Color("#7DD3FC"),
		Text:            lipgloss.Color("#E2E8F0"),
		TextMuted:       lipgloss.Color("#94A3B8"),
		InputPrompt:     lipgloss.Color("#34D399"),
		OutputUser:      lipgloss.Color("#60A5FA"),
		OutputAssistant: lipgloss.Color("#34D399"),
	},
	"aurora": {
		Name:            "aurora",
		Primary:         lipgloss.Color("#F472B6"),
		Secondary:       lipgloss.Color("#A78BFA"),
		Accent:          lipgloss.Color("#67E8F9"),
		Background:      lipgloss.Color("#0C0A1D"),
		Foreground:      lipgloss.Color("#FAFAFA"),
		Success:         lipgloss.Color("#4ADE80"),
		Warning:         lipgloss.Color("#FACC15"),
		Error:           lipgloss.Color("#FB7185"),
		Info:            lipgloss.Color("#38BDF8"),
		Border:          lipgloss.Color("#6B21A8"),
		BorderActive:    lipgloss.Color("#F472B6"),
		Title:           lipgloss.Color("#F472B6"),
		Text:            lipgloss.Color("#FAFAFA"),
		TextMuted:       lipgloss.Color("#A1A1AA"),
		InputPrompt:     lipgloss.Color("#4ADE80"),
		OutputUser:      lipgloss.Color("#38BDF8"),
		OutputAssistant: lipgloss.Color("#4ADE80"),
	},
	"forest": {
		Name:            "forest",
		Primary:         lipgloss.Color("#86EFAC"),
		Secondary:       lipgloss.Color("#A7F3D0"),
		Accent:          lipgloss.Color("#D9F99D"),
		Background:      lipgloss.Color("#052E16"),
		Foreground:      lipgloss.Color("#ECFDF5"),
		Success:         lipgloss.Color("#4ADE80"),
		Warning:         lipgloss.Color("#FDE047"),
		Error:           lipgloss.Color("#FCA5A5"),
		Info:            lipgloss.Color("#67E8F9"),
		Border:          lipgloss.Color("#166534"),
		BorderActive:    lipgloss.Color("#86EFAC"),
		Title:           lipgloss.Color("#86EFAC"),
		Text:            lipgloss.Color("#ECFDF5"),
		TextMuted:       lipgloss.Color("#86EFAC"),
		InputPrompt:     lipgloss.Color("#BEF264"),
		OutputUser:      lipgloss.Color("#67E8F9"),
		OutputAssistant: lipgloss.Color("#BEF264"),
	},
	"ocean": {
		Name:            "ocean",
		Primary:         lipgloss.Color("#22D3EE"),
		Secondary:       lipgloss.Color("#38BDF8"),
		Accent:          lipgloss.Color("#818CF8"),
		Background:      lipgloss.Color("#0C4A6E"),
		Foreground:      lipgloss.Color("#F0F9FF"),
		Success:         lipgloss.Color("#4ADE80"),
		Warning:         lipgloss.Color("#FBBF24"),
		Error:           lipgloss.Color("#FB923C"),
		Info:            lipgloss.Color("#22D3EE"),
		Border:          lipgloss.Color("#0369A1"),
		BorderActive:    lipgloss.Color("#22D3EE"),
		Title:           lipgloss.Color("#22D3EE"),
		Text:            lipgloss.Color("#F0F9FF"),
		TextMuted:       lipgloss.Color("#7DD3FC"),
		InputPrompt:     lipgloss.Color("#4ADE80"),
		OutputUser:      lipgloss.Color("#818CF8"),
		OutputAssistant: lipgloss.Color("#4ADE80"),
	},
	"purple": {
		Name:            "purple",
		Primary:         lipgloss.Color("#7C3AED"),
		Secondary:       lipgloss.Color("#A78BFA"),
		Accent:          lipgloss.Color("#C4B5FD"),
		Background:      lipgloss.Color("#1F2937"),
		Foreground:      lipgloss.Color("#E5E7EB"),
		Success:         lipgloss.Color("#10B981"),
		Warning:         lipgloss.Color("#F59E0B"),
		Error:           lipgloss.Color("#EF4444"),
		Info:            lipgloss.Color("#3B82F6"),
		Border:          lipgloss.Color("#7C3AED"),
		BorderActive:    lipgloss.Color("#A78BFA"),
		Title:           lipgloss.Color("#7C3AED"),
		Text:            lipgloss.Color("#E5E7EB"),
		TextMuted:       lipgloss.Color("#9CA3AF"),
		InputPrompt:     lipgloss.Color("#10B981"),
		OutputUser:      lipgloss.Color("#3B82F6"),
		OutputAssistant: lipgloss.Color("#10B981"),
	},
	"dracula": {
		Name:            "dracula",
		Primary:         lipgloss.Color("#BD93F9"),
		Secondary:       lipgloss.Color("#FF79C6"),
		Accent:          lipgloss.Color("#8BE9FD"),
		Background:      lipgloss.Color("#282A36"),
		Foreground:      lipgloss.Color("#F8F8F2"),
		Success:         lipgloss.Color("#50FA7B"),
		Warning:         lipgloss.Color("#FFB86C"),
		Error:           lipgloss.Color("#FF5555"),
		Info:            lipgloss.Color("#8BE9FD"),
		Border:          lipgloss.Color("#BD93F9"),
		BorderActive:    lipgloss.Color("#FF79C6"),
		Title:           lipgloss.Color("#BD93F9"),
		Text:            lipgloss.Color("#F8F8F2"),
		TextMuted:       lipgloss.Color("#6272A4"),
		InputPrompt:     lipgloss.Color("#50FA7B"),
		OutputUser:      lipgloss.Color("#8BE9FD"),
		OutputAssistant: lipgloss.Color("#50FA7B"),
	},
	"monokai": {
		Name:            "monokai",
		Primary:         lipgloss.Color("#F92672"),
		Secondary:       lipgloss.Color("#AE81FF"),
		Accent:          lipgloss.Color("#66D9EF"),
		Background:      lipgloss.Color("#272822"),
		Foreground:      lipgloss.Color("#F8F8F2"),
		Success:         lipgloss.Color("#A6E22E"),
		Warning:         lipgloss.Color("#FD971F"),
		Error:           lipgloss.Color("#F92672"),
		Info:            lipgloss.Color("#66D9EF"),
		Border:          lipgloss.Color("#F92672"),
		BorderActive:    lipgloss.Color("#AE81FF"),
		Title:           lipgloss.Color("#F92672"),
		Text:            lipgloss.Color("#F8F8F2"),
		TextMuted:       lipgloss.Color("#75715E"),
		InputPrompt:     lipgloss.Color("#A6E22E"),
		OutputUser:      lipgloss.Color("#66D9EF"),
		OutputAssistant: lipgloss.Color("#A6E22E"),
	},
	"nord": {
		Name:            "nord",
		Primary:         lipgloss.Color("#88C0D0"),
		Secondary:       lipgloss.Color("#81A1C1"),
		Accent:          lipgloss.Color("#5E81AC"),
		Background:      lipgloss.Color("#2E3440"),
		Foreground:      lipgloss.Color("#ECEFF4"),
		Success:         lipgloss.Color("#A3BE8C"),
		Warning:         lipgloss.Color("#EBCB8B"),
		Error:           lipgloss.Color("#BF616A"),
		Info:            lipgloss.Color("#88C0D0"),
		Border:          lipgloss.Color("#88C0D0"),
		BorderActive:    lipgloss.Color("#81A1C1"),
		Title:           lipgloss.Color("#88C0D0"),
		Text:            lipgloss.Color("#ECEFF4"),
		TextMuted:       lipgloss.Color("#4C566A"),
		InputPrompt:     lipgloss.Color("#A3BE8C"),
		OutputUser:      lipgloss.Color("#88C0D0"),
		OutputAssistant: lipgloss.Color("#A3BE8C"),
	},
	"solarized": {
		Name:            "solarized",
		Primary:         lipgloss.Color("#268BD2"),
		Secondary:       lipgloss.Color("#2AA198"),
		Accent:          lipgloss.Color("#859900"),
		Background:      lipgloss.Color("#002B36"),
		Foreground:      lipgloss.Color("#839496"),
		Success:         lipgloss.Color("#859900"),
		Warning:         lipgloss.Color("#B58900"),
		Error:           lipgloss.Color("#DC322F"),
		Info:            lipgloss.Color("#268BD2"),
		Border:          lipgloss.Color("#268BD2"),
		BorderActive:    lipgloss.Color("#2AA198"),
		Title:           lipgloss.Color("#268BD2"),
		Text:            lipgloss.Color("#839496"),
		TextMuted:       lipgloss.Color("#586E75"),
		InputPrompt:     lipgloss.Color("#859900"),
		OutputUser:      lipgloss.Color("#268BD2"),
		OutputAssistant: lipgloss.Color("#859900"),
	},
	"dark": {
		Name:            "dark",
		Primary:         lipgloss.Color("#FFFFFF"),
		Secondary:       lipgloss.Color("#9CA3AF"),
		Accent:          lipgloss.Color("#6B7280"),
		Background:      lipgloss.Color("#000000"),
		Foreground:      lipgloss.Color("#FFFFFF"),
		Success:         lipgloss.Color("#22C55E"),
		Warning:         lipgloss.Color("#EAB308"),
		Error:           lipgloss.Color("#EF4444"),
		Info:            lipgloss.Color("#3B82F6"),
		Border:          lipgloss.Color("#374151"),
		BorderActive:    lipgloss.Color("#FFFFFF"),
		Title:           lipgloss.Color("#FFFFFF"),
		Text:            lipgloss.Color("#FFFFFF"),
		TextMuted:       lipgloss.Color("#6B7280"),
		InputPrompt:     lipgloss.Color("#22C55E"),
		OutputUser:      lipgloss.Color("#3B82F6"),
		OutputAssistant: lipgloss.Color("#22C55E"),
	},
	"light": {
		Name:            "light",
		Primary:         lipgloss.Color("#1F2937"),
		Secondary:       lipgloss.Color("#4B5563"),
		Accent:          lipgloss.Color("#6B7280"),
		Background:      lipgloss.Color("#FFFFFF"),
		Foreground:      lipgloss.Color("#1F2937"),
		Success:         lipgloss.Color("#059669"),
		Warning:         lipgloss.Color("#D97706"),
		Error:           lipgloss.Color("#DC2626"),
		Info:            lipgloss.Color("#2563EB"),
		Border:          lipgloss.Color("#D1D5DB"),
		BorderActive:    lipgloss.Color("#1F2937"),
		Title:           lipgloss.Color("#1F2937"),
		Text:            lipgloss.Color("#1F2937"),
		TextMuted:       lipgloss.Color("#6B7280"),
		InputPrompt:     lipgloss.Color("#059669"),
		OutputUser:      lipgloss.Color("#2563EB"),
		OutputAssistant: lipgloss.Color("#059669"),
	},
}

var currentTheme = Themes["midnight"]

func GetTheme() Theme {
	return currentTheme
}

func SetTheme(name string) bool {
	if theme, ok := Themes[name]; ok {
		currentTheme = theme
		return true
	}
	return false
}

func ListThemes() []string {
	names := make([]string, 0, len(Themes))
	for name := range Themes {
		names = append(names, name)
	}
	return names
}

func (t Theme) TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(t.Title).
		Padding(0, 1)
}

func (t Theme) BoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)
}

func (t Theme) InputStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.InputPrompt).
		Bold(true)
}

func (t Theme) OutputStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Text)
}

func (t Theme) StatusStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Padding(0, 1)
}

func (t Theme) HelpStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Italic(true)
}

func (t Theme) ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Error).
		Bold(true)
}

func (t Theme) SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Success).
		Bold(true)
}

func (t Theme) WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Warning).
		Bold(true)
}

func (t Theme) InfoStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Info).
		Bold(true)
}

func (t Theme) MessageStyle(role string) lipgloss.Style {
	base := lipgloss.NewStyle().Padding(0, 1)

	switch role {
	case "user":
		return base.Foreground(t.OutputUser).Bold(true)
	case "assistant":
		return base.Foreground(t.OutputAssistant).Bold(true)
	default:
		return base.Foreground(t.Text)
	}
}

func (t Theme) BorderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border).
		Padding(0, 1)
}

func (t Theme) ActiveBorderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderActive).
		Padding(0, 1)
}

func (t Theme) GradientStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Primary).
		Background(t.Background)
}

func (t Theme) TabStyle(active bool) lipgloss.Style {
	if active {
		return lipgloss.NewStyle().
			Foreground(t.Background).
			Background(t.Primary).
			Bold(true).
			Padding(0, 2).
			MarginRight(1)
	}
	return lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Background).
		Padding(0, 2).
		MarginRight(1)
}
