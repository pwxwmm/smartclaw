package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ProgressMsg struct {
	Value float64
}

type CompleteMsg struct{}

type ProgressBar struct {
	width     int
	progress  float64
	max       float64
	label     string
	showPct   bool
	color     lipgloss.Color
	completed bool
}

var progressStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7C3AED"))

var progressBgStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#374151"))

var progressFilledStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("#7C3AED"))

func NewProgressBar(width int, max float64) *ProgressBar {
	return &ProgressBar{
		width:     width,
		progress:  0,
		max:       max,
		label:     "",
		showPct:   true,
		color:     lipgloss.Color("#7C3AED"),
		completed: false,
	}
}

func (p *ProgressBar) SetProgress(value float64) {
	p.progress = value
	if p.progress >= p.max {
		p.completed = true
	}
}

func (p *ProgressBar) SetLabel(label string) {
	p.label = label
}

func (p *ProgressBar) SetColor(color lipgloss.Color) {
	p.color = color
}

func (p *ProgressBar) Increment() {
	p.progress++
	if p.progress >= p.max {
		p.completed = true
	}
}

func (p *ProgressBar) IsCompleted() bool {
	return p.completed
}

func (p *ProgressBar) Render() string {
	if p.max == 0 {
		return ""
	}

	percentage := p.progress / p.max * 100
	filledWidth := int(float64(p.width) * p.progress / p.max)

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < p.width; i++ {
		if i < filledWidth {
			bar.WriteString("█")
		} else {
			bar.WriteString("░")
		}
	}
	bar.WriteString("]")

	result := progressStyle.Render(bar.String())
	if p.showPct {
		pct := fmt.Sprintf(" %.1f%%", percentage)
		result += pct
	}

	if p.label != "" {
		result = p.label + " " + result
	}

	return result
}

type Spinner struct {
	frames  []string
	current int
	label   string
	running bool
	color   lipgloss.Color
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

var spinnerDots = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

var spinnerSimple = []string{"|", "/", "-", "\\"}

func NewSpinner(label string) *Spinner {
	return &Spinner{
		frames:  spinnerFrames,
		current: 0,
		label:   label,
		running: false,
		color:   lipgloss.Color("#7C3AED"),
	}
}

func (s *Spinner) SetFrames(frames []string) {
	s.frames = frames
}

func (s *Spinner) SetLabel(label string) {
	s.label = label
}

func (s *Spinner) SetColor(color lipgloss.Color) {
	s.color = color
}

func (s *Spinner) Start() {
	s.running = true
}

func (s *Spinner) Stop() {
	s.running = false
}

func (s *Spinner) Next() string {
	frame := s.frames[s.current]
	s.current = (s.current + 1) % len(s.frames)
	return frame
}

func (s *Spinner) Render() string {
	frame := s.Next()
	style := lipgloss.NewStyle().Foreground(s.color)
	result := style.Render(frame)
	if s.label != "" {
		result = result + " " + s.label
	}
	return result
}

type ProgressWithSpinner struct {
	progress *ProgressBar
	spinner  *Spinner
	showSpin bool
}

func NewProgressWithSpinner(width int, max float64, label string) *ProgressWithSpinner {
	return &ProgressWithSpinner{
		progress: NewProgressBar(width, max),
		spinner:  NewSpinner(label),
		showSpin: true,
	}
}

func (p *ProgressWithSpinner) SetProgress(value float64) {
	p.progress.SetProgress(value)
	if p.progress.IsCompleted() {
		p.spinner.Stop()
	}
}

func (p *ProgressWithSpinner) Render() string {
	if p.showSpin && !p.progress.IsCompleted() {
		return p.spinner.Render() + "\n" + p.progress.Render()
	}
	return p.progress.Render()
}

type SpinnerModel struct {
	spinner  *Spinner
	quitting bool
}

func NewSpinnerModel(label string) SpinnerModel {
	return SpinnerModel{
		spinner: NewSpinner(label),
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return time.Time(t)
	})
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case time.Time:
		if m.quitting {
			return m, nil
		}
		return m, tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
			return time.Time(t)
		})
	}
	return m, nil
}

func (m SpinnerModel) View() string {
	if m.quitting {
		return "Done!\n"
	}
	return m.spinner.Render()
}
