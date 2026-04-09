package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	LogoSmall = `
   ,~~.
  ( o o)
   \ = /
  //"""\\
 ((/o o\))
`

	LogoMedium = `
    ███████╗ ██████╗ ███╗   ██╗ █████╗ ███████╗
    ██╔════╝██╔════╝ ████╗  ██║██╔══██╗██╔════╝
    ███████╗██║  ███╗██╔██╗ ██║███████║███████╗
    ╚════██║██║   ██║██║╚██╗██║██╔══██║╚════██║
    ███████║╚██████╔╝██║ ╚████║██║  ██║███████║
    ╚══════╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝
`

	LogoASCII = `
                    ███████╗ ██████╗ ███╗   ██╗ █████╗ ███████╗
                    ██╔════╝██╔════╝ ████╗  ██║██╔══██╗██╔════╝
                    ███████╗██║  ███╗██╔██╗ ██║███████║███████╗
                    ╚════██║██║   ██║██║╚██╗██║██╔══██║╚════██║
                    ███████║╚██████╔╝██║ ╚████║██║  ██║███████║
                    ╚══════╝ ╚═════╝ ╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝
`

	OwlLogo = `
          ,___          .-;'
          "'-.;\_...._/'.(
   _,      \       _   /'._
  / \      /      '-' /._  \
 /  /      /       _  / '\  \
 \  \      /      '-' /   \  \
  \  \    /          /     \  \
   \  \  /          /       \  \
    \  \/          /         \  \
     \  \         /           \  \
      \  \       /             \  \
       \  \     /               \  \
        \  \   /                 \  \
         \  \ /                   \  \
          \  v                    \  \
           \/                      \ /
            '______________________'
             |_ _|  |_ _|
             (o o)  (o o)
             |o o|  |o o|
            //'''\\//'''\\
`

	OwlLogoCompact = `
    ,___   .-;'
    "'-.;\_...._/'.(
   \       _   /'._
    \     '-' /._  \
   _____________  /
  /  OWL      \  \
 /   SMART    /  /
 \   CLAW    /  /
  \_________/. /
    |o o|  |o o|
    //''\\//''\\
`

	MinimalOwl = `
    (o,o)
    /)__)
   -"-"-
`
)

type LogoRenderer struct {
	theme  Theme
	frames []string
	frame  int
}

func NewLogoRenderer(theme Theme) *LogoRenderer {
	return &LogoRenderer{
		theme: theme,
		frames: []string{
			MinimalOwl,
		},
	}
}

func (l *LogoRenderer) Render(size string) string {
	var logo string
	switch size {
	case "small":
		logo = LogoSmall
	case "medium":
		logo = LogoMedium
	case "compact":
		logo = OwlLogoCompact
	case "owl":
		logo = OwlLogo
	default:
		logo = MinimalOwl
	}

	style := lipgloss.NewStyle().
		Foreground(l.theme.Primary).
		Bold(true)

	return style.Render(logo)
}

func (l *LogoRenderer) RenderWithTitle(title, subtitle string) string {
	logo := l.Render("compact")

	titleStyle := lipgloss.NewStyle().
		Foreground(l.theme.Primary).
		Bold(true).
		Padding(0, 2)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(l.theme.Secondary).
		Italic(true).
		Padding(0, 2)

	content := lipgloss.JoinVertical(lipgloss.Left,
		logo,
		"",
		titleStyle.Render(title),
		subtitleStyle.Render(subtitle),
	)

	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(content)
}

func (l *LogoRenderer) RenderInline() string {
	owl := lipgloss.NewStyle().
		Foreground(l.theme.Primary).
		Bold(true).
		Render("(o,o)")

	text := lipgloss.NewStyle().
		Foreground(l.theme.Title).
		Bold(true).
		Render(" SmartClaw")

	return owl + text
}

func (l *LogoRenderer) RenderAnimated() string {
	frames := []string{
		"  (o,o)  ",
		"  (o,o)  ",
		"  (O,O)  ",
		"  (o,o)  ",
	}

	frame := frames[l.frame%len(frames)]
	l.frame++

	style := lipgloss.NewStyle().
		Foreground(l.theme.Primary).
		Bold(true)

	return style.Render(frame)
}

func RenderWelcomeScreen(theme Theme, width int) string {
	logo := NewLogoRenderer(theme)

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Align(lipgloss.Center)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(theme.Secondary).
		Align(lipgloss.Center)

	taglineStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Italic(true).
		Align(lipgloss.Center)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(theme.Primary).
		Padding(1, 3).
		Width(width - 4)

	content := lipgloss.JoinVertical(lipgloss.Center,
		logo.Render("compact"),
		"",
		titleStyle.Render("Welcome to SmartClaw"),
		subtitleStyle.Render("Your AI-Powered Coding Companion"),
		"",
		taglineStyle.Render("\"The wise owl watches, the smart claw builds\""),
		"",
		renderQuickCommands(theme),
	)

	return boxStyle.Render(content)
}

func renderQuickCommands(theme Theme) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	cmdStyle := lipgloss.NewStyle().
		Foreground(theme.Text)

	descStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted)

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help", "Show available commands"},
		{"/model", "Change AI model"},
		{"/theme", "Switch color theme"},
		{"/clear", "Clear conversation"},
	}

	var lines []string
	lines = append(lines, headerStyle.Render("Quick Commands:"))

	for _, c := range commands {
		line := lipgloss.JoinHorizontal(lipgloss.Left,
			cmdStyle.Render("  "+c.cmd),
			strings.Repeat(" ", 12-len(c.cmd)),
			descStyle.Render(c.desc),
		)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}
