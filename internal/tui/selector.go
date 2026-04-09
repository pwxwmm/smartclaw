package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MenuItem struct {
	Label       string
	Description string
	Value       string
	Disabled    bool
}

type Menu struct {
	items       []MenuItem
	selected    int
	offset      int
	height      int
	width       int
	title       string
	showDesc    bool
	showSearch  bool
	searchQuery string
}

var menuItemStyle = lipgloss.NewStyle().
	Padding(0, 2)

var menuItemSelectedStyle = lipgloss.NewStyle().
	Padding(0, 2).
	Background(lipgloss.Color("#7C3AED")).
	Foreground(lipgloss.Color("#FFFFFF"))

var menuItemDisabledStyle = lipgloss.NewStyle().
	Padding(0, 2).
	Foreground(lipgloss.Color("#6B7280"))

var menuDescStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#9CA3AF")).
	Padding(0, 2, 0, 6)

func NewMenu(title string, items []MenuItem) *Menu {
	return &Menu{
		items:       items,
		selected:    0,
		offset:      0,
		height:      10,
		width:       60,
		title:       title,
		showDesc:    true,
		showSearch:  false,
		searchQuery: "",
	}
}

func (m *Menu) SetHeight(height int) {
	m.height = height
}

func (m *Menu) SetWidth(width int) {
	m.width = width
}

func (m *Menu) ShowDescription(show bool) {
	m.showDesc = show
}

func (m *Menu) ShowSearch(show bool) {
	m.showSearch = show
}

func (m *Menu) Init() tea.Cmd {
	return nil
}

func (m *Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.moveUp()
		case "down", "j":
			m.moveDown()
		case "enter":
			return m, tea.Quit
		case "escape":
			m.selected = -1
			return m, tea.Quit
		default:
			if m.showSearch {
				m.searchQuery += msg.String()
				m.filterItems()
			}
		}
	}

	return m, nil
}

func (m *Menu) View() string {
	theme := GetTheme()

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Padding(0, 1)

	sb.WriteString(titleStyle.Render(m.title))
	sb.WriteString("\n")

	if m.showSearch {
		sb.WriteString(m.renderSearch(theme))
		sb.WriteString("\n")
	}

	sb.WriteString(m.renderItems(theme))
	sb.WriteString("\n")

	sb.WriteString(m.renderHelp(theme))

	return sb.String()
}

func (m *Menu) renderSearch(theme Theme) string {
	searchStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted)

	return searchStyle.Render("Search: " + m.searchQuery + "_")
}

func (m *Menu) renderItems(theme Theme) string {
	visibleItems := m.getVisibleItems()
	var lines []string

	for i, item := range visibleItems {
		lines = append(lines, m.renderItem(item, i, theme))
	}

	return strings.Join(lines, "\n")
}

func (m *Menu) renderItem(item MenuItem, index int, theme Theme) string {
	var style lipgloss.Style

	if item.Disabled {
		style = menuItemDisabledStyle
	} else if index == m.selected-m.offset {
		style = menuItemSelectedStyle
	} else {
		style = menuItemStyle
	}

	label := item.Label
	if index == m.selected-m.offset && !item.Disabled {
		label = "▶ " + label
	} else {
		label = "  " + label
	}

	result := style.Render(label)

	if m.showDesc && item.Description != "" {
		desc := menuDescStyle.Render(item.Description)
		result += "\n" + desc
	}

	return result
}

func (m *Menu) renderHelp(theme Theme) string {
	helpStyle := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Italic(true)

	return helpStyle.Render("↑↓: Navigate | Enter: Select | Esc: Cancel")
}

func (m *Menu) moveUp() {
	for i := m.selected - 1; i >= 0; i-- {
		if !m.items[i].Disabled {
			m.selected = i
			break
		}
	}

	if m.selected < m.offset {
		m.offset = m.selected
	}
}

func (m *Menu) moveDown() {
	for i := m.selected + 1; i < len(m.items); i++ {
		if !m.items[i].Disabled {
			m.selected = i
			break
		}
	}

	if m.selected >= m.offset+m.height {
		m.offset = m.selected - m.height + 1
	}
}

func (m *Menu) getVisibleItems() []MenuItem {
	start := m.offset
	end := m.offset + m.height

	if end > len(m.items) {
		end = len(m.items)
	}

	return m.items[start:end]
}

func (m *Menu) filterItems() {
}

func (m *Menu) GetSelected() MenuItem {
	if m.selected >= 0 && m.selected < len(m.items) {
		return m.items[m.selected]
	}
	return MenuItem{}
}

func (m *Menu) GetSelectedIndex() int {
	return m.selected
}

type Dropdown struct {
	items       []MenuItem
	selected    int
	expanded    bool
	label       string
	width       int
	placeholder string
}

func NewDropdown(label string, items []MenuItem) *Dropdown {
	return &Dropdown{
		items:       items,
		selected:    0,
		expanded:    false,
		label:       label,
		width:       40,
		placeholder: "Select...",
	}
}

func (d *Dropdown) SetPlaceholder(placeholder string) {
	d.placeholder = placeholder
}

func (d *Dropdown) SetWidth(width int) {
	d.width = width
}

func (d *Dropdown) Init() tea.Cmd {
	return nil
}

func (d *Dropdown) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			d.expanded = !d.expanded
		case "up", "k":
			if d.expanded && d.selected > 0 {
				d.selected--
			}
		case "down", "j":
			if d.expanded && d.selected < len(d.items)-1 {
				d.selected++
			}
		case "escape":
			d.expanded = false
		}
	}

	return d, nil
}

func (d *Dropdown) View() string {
	theme := GetTheme()

	var sb strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(theme.Text).
		Bold(true)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1)

	sb.WriteString(labelStyle.Render(d.label + ": "))
	sb.WriteString("\n")

	currentText := d.placeholder
	if len(d.items) > 0 && d.selected >= 0 {
		currentText = d.items[d.selected].Label
	}

	box := boxStyle.Width(d.width).Render(currentText + " ▼")
	sb.WriteString(box)

	if d.expanded {
		sb.WriteString("\n")
		sb.WriteString(d.renderDropdown(theme))
	}

	return sb.String()
}

func (d *Dropdown) renderDropdown(theme Theme) string {
	dropdownStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.BorderActive).
		Padding(0, 1)

	var lines []string
	for i, item := range d.items {
		if i == d.selected {
			lines = append(lines, "▶ "+item.Label)
		} else {
			lines = append(lines, "  "+item.Label)
		}
	}

	return dropdownStyle.Width(d.width).Render(strings.Join(lines, "\n"))
}

func (d *Dropdown) GetSelected() MenuItem {
	if d.selected >= 0 && d.selected < len(d.items) {
		return d.items[d.selected]
	}
	return MenuItem{}
}

func (d *Dropdown) GetSelectedIndex() int {
	return d.selected
}

type Tab struct {
	labels   []string
	selected int
	width    int
}

func NewTabs(labels []string) *Tab {
	return &Tab{
		labels:   labels,
		selected: 0,
		width:    80,
	}
}

func (t *Tab) SetWidth(width int) {
	t.width = width
}

func (t *Tab) Select(index int) {
	if index >= 0 && index < len(t.labels) {
		t.selected = index
	}
}

func (t *Tab) Next() {
	t.selected = (t.selected + 1) % len(t.labels)
}

func (t *Tab) Prev() {
	t.selected = (t.selected - 1 + len(t.labels)) % len(t.labels)
}

func (t *Tab) Render() string {
	theme := GetTheme()

	tabWidth := t.width / len(t.labels)
	var tabs []string

	for i, label := range t.labels {
		style := lipgloss.NewStyle().
			Padding(0, 2).
			Width(tabWidth).
			Align(lipgloss.Center)

		if i == t.selected {
			style = style.
				Bold(true).
				Foreground(theme.Foreground).
				Background(theme.Primary)
		} else {
			style = style.
				Foreground(theme.TextMuted)
		}

		tabs = append(tabs, style.Render(label))
	}

	return strings.Join(tabs, "")
}

func (t *Tab) GetSelected() int {
	return t.selected
}

func (t *Tab) GetLabel() string {
	if t.selected >= 0 && t.selected < len(t.labels) {
		return t.labels[t.selected]
	}
	return ""
}
