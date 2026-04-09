package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MouseAction int

const (
	MouseClick MouseAction = iota
	MouseDoubleClick
	MouseRightClick
	MouseMiddleClick
	MouseWheelUp
	MouseWheelDown
)

type MouseMsg struct {
	X      int
	Y      int
	Action MouseAction
	Button tea.MouseButton
}

type MouseRegion struct {
	ID      string
	X       int
	Y       int
	Width   int
	Height  int
	OnClick func(MouseMsg)
	OnHover func(MouseMsg)
	Hovered bool
	Enabled bool
}

type MouseSupport struct {
	regions   []MouseRegion
	mouseX    int
	mouseY    int
	enabled   bool
	lastClick time.Time
	theme     Theme
}

func NewMouseSupport() *MouseSupport {
	return &MouseSupport{
		regions: make([]MouseRegion, 0),
		enabled: true,
		theme:   GetTheme(),
	}
}

func (m *MouseSupport) AddRegion(region MouseRegion) {
	m.regions = append(m.regions, region)
}

func (m *MouseSupport) RemoveRegion(id string) {
	for i, r := range m.regions {
		if r.ID == id {
			m.regions = append(m.regions[:i], m.regions[i+1:]...)
			break
		}
	}
}

func (m *MouseSupport) ClearRegions() {
	m.regions = make([]MouseRegion, 0)
}

func (m *MouseSupport) Update(msg tea.Msg) tea.Cmd {
	if !m.enabled {
		return nil
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		m.mouseX = msg.X
		m.mouseY = msg.Y

		for i, region := range m.regions {
			if !region.Enabled {
				continue
			}

			if m.isInRegion(msg.X, msg.Y, region) {
				switch msg.Type {
				case tea.MouseLeft:
					m.regions[i].Hovered = true
					if region.OnHover != nil {
						region.OnHover(MouseMsg{X: msg.X, Y: msg.Y, Action: MouseClick})
					}
					if msg.Action == tea.MouseActionPress && region.OnClick != nil {
						region.OnClick(MouseMsg{X: msg.X, Y: msg.Y, Action: MouseClick})
					}
				case tea.MouseRight:
					if region.OnClick != nil {
						region.OnClick(MouseMsg{X: msg.X, Y: msg.Y, Action: MouseRightClick})
					}
				}
			} else {
				m.regions[i].Hovered = false
			}

			switch msg.Type {
			case tea.MouseWheelUp:
				for _, r := range m.regions {
					if m.isInRegion(msg.X, msg.Y, r) && r.OnHover != nil {
						r.OnHover(MouseMsg{X: msg.X, Y: msg.Y, Action: MouseWheelUp})
					}
				}
			case tea.MouseWheelDown:
				for _, r := range m.regions {
					if m.isInRegion(msg.X, msg.Y, r) && r.OnHover != nil {
						r.OnHover(MouseMsg{X: msg.X, Y: msg.Y, Action: MouseWheelDown})
					}
				}
			}
		}
	}

	return nil
}

func (m *MouseSupport) isInRegion(x, y int, region MouseRegion) bool {
	return x >= region.X && x < region.X+region.Width &&
		y >= region.Y && y < region.Y+region.Height
}

func (m *MouseSupport) GetMousePosition() (int, int) {
	return m.mouseX, m.mouseY
}

func (m *MouseSupport) Enable(enabled bool) {
	m.enabled = enabled
}

func (m *MouseSupport) IsEnabled() bool {
	return m.enabled
}

type ClickableStyle struct {
	normal   lipgloss.Style
	hover    lipgloss.Style
	active   lipgloss.Style
	disabled lipgloss.Style
}

func NewClickableStyle(theme Theme) ClickableStyle {
	return ClickableStyle{
		normal: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.Border),
		hover: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.Primary).
			Foreground(theme.Primary),
		active: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(theme.Success).
			Background(theme.Primary),
		disabled: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(theme.TextMuted).
			Foreground(theme.TextMuted),
	}
}

type Button struct {
	text    string
	x       int
	y       int
	width   int
	height  int
	onClick func()
	hovered bool
	enabled bool
	style   ClickableStyle
	region  MouseRegion
}

func NewButton(text string, onClick func()) *Button {
	return &Button{
		text:    text,
		width:   len(text) + 4,
		height:  1,
		onClick: onClick,
		hovered: false,
		enabled: true,
		style:   NewClickableStyle(GetTheme()),
	}
}

func (b *Button) SetPosition(x, y int) {
	b.x = x
	b.y = y
	b.region = MouseRegion{
		ID:      "button_" + b.text,
		X:       x,
		Y:       y,
		Width:   b.width,
		Height:  b.height,
		OnClick: func(_ MouseMsg) { b.onClick() },
	}
}

func (b *Button) SetEnabled(enabled bool) {
	b.enabled = enabled
	b.region.Enabled = enabled
}

func (b *Button) Render() string {
	var style lipgloss.Style

	if !b.enabled {
		style = b.style.disabled
	} else if b.hovered {
		style = b.style.hover
	} else {
		style = b.style.normal
	}

	return style.Render(b.text)
}

func (b *Button) GetRegion() MouseRegion {
	return b.region
}
