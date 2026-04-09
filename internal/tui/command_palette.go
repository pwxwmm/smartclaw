package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CommandPaletteItem struct {
	ID          string
	Name        string
	Description string
	Category    string
	Handler     func() tea.Cmd
}

type CommandPalette struct {
	visible  bool
	input    textinput.Model
	items    []CommandPaletteItem
	filtered []CommandPaletteItem
	selected int
	width    int
	height   int
	styles   commandPaletteStyles
}

type commandPaletteStyles struct {
	container lipgloss.Style
	input     lipgloss.Style
	item      lipgloss.Style
	selected  lipgloss.Style
	category  lipgloss.Style
	help      lipgloss.Style
}

func NewCommandPalette() *CommandPalette {
	ti := textinput.New()
	ti.Placeholder = "搜索命令..."
	ti.Focus()

	styles := commandPaletteStyles{
		container: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1),
		input: lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")),
		item: lipgloss.NewStyle().
			Padding(0, 2),
		selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Background(lipgloss.Color("235")).
			Padding(0, 2),
		category: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")),
		help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
	}

	return &CommandPalette{
		input:    ti,
		items:    getDefaultCommands(),
		filtered: getDefaultCommands(),
		selected: 0,
		styles:   styles,
	}
}

func getDefaultCommands() []CommandPaletteItem {
	return []CommandPaletteItem{
		{ID: "help", Name: "/help", Description: "显示帮助信息", Category: "系统"},
		{ID: "clear", Name: "/clear", Description: "清空输出", Category: "系统"},
		{ID: "exit", Name: "/exit", Description: "退出程序", Category: "系统"},
		{ID: "status", Name: "/status", Description: "显示状态", Category: "系统"},
		{ID: "cost", Name: "/cost", Description: "显示成本统计", Category: "系统"},

		{ID: "model", Name: "/model", Description: "模型管理", Category: "AI"},
		{ID: "model-list", Name: "/model list", Description: "列出所有模型", Category: "AI"},
		{ID: "model-switch", Name: "/model switch", Description: "切换模型", Category: "AI"},
		{ID: "agent", Name: "/agent", Description: "Agent 管理", Category: "AI"},
		{ID: "agent-list", Name: "/agent list", Description: "列出所有 Agent", Category: "AI"},
		{ID: "agent-switch", Name: "/agent switch", Description: "切换 Agent", Category: "AI"},
		{ID: "template", Name: "/template", Description: "模板管理", Category: "AI"},
		{ID: "template-list", Name: "/template list", Description: "列出所有模板", Category: "AI"},
		{ID: "template-use", Name: "/template use", Description: "使用模板", Category: "AI"},

		{ID: "config", Name: "/config", Description: "配置管理", Category: "配置"},
		{ID: "config-show", Name: "/config show", Description: "显示配置", Category: "配置"},
		{ID: "config-set", Name: "/config set", Description: "设置配置", Category: "配置"},
		{ID: "config-reset", Name: "/config reset", Description: "重置配置", Category: "配置"},
		{ID: "config-export", Name: "/config export", Description: "导出配置", Category: "配置"},
		{ID: "config-import", Name: "/config import", Description: "导入配置", Category: "配置"},

		{ID: "session", Name: "/session", Description: "会话管理", Category: "会话"},
		{ID: "session-new", Name: "/session new", Description: "新建会话", Category: "会话"},
		{ID: "session-list", Name: "/session list", Description: "列出会话", Category: "会话"},
		{ID: "session-save", Name: "/session save", Description: "保存会话", Category: "会话"},
		{ID: "session-load", Name: "/session load", Description: "加载会话", Category: "会话"},

		{ID: "context", Name: "/context", Description: "上下文管理", Category: "上下文"},
		{ID: "context-list", Name: "/context list", Description: "列出消息", Category: "上下文"},
		{ID: "context-clear", Name: "/context clear", Description: "清空上下文", Category: "上下文"},
		{ID: "context-compress", Name: "/context compress", Description: "压缩上下文", Category: "上下文"},

		{ID: "git", Name: "/git", Description: "Git 集成", Category: "Git"},
		{ID: "git-status", Name: "/git status", Description: "Git 状态", Category: "Git"},
		{ID: "git-commit", Name: "/git commit", Description: "Git 提交", Category: "Git"},
		{ID: "git-push", Name: "/git push", Description: "Git 推送", Category: "Git"},
		{ID: "git-pull", Name: "/git pull", Description: "Git 拉取", Category: "Git"},

		{ID: "edit", Name: "/edit", Description: "打开编辑器", Category: "编辑"},
		{ID: "editor", Name: "/editor", Description: "编辑器设置", Category: "编辑"},
		{ID: "multilines", Name: "/multilines", Description: "多行输入", Category: "编辑"},

		{ID: "run", Name: "/run", Description: "执行代码", Category: "执行"},
		{ID: "python", Name: "/python", Description: "执行 Python", Category: "执行"},
		{ID: "js", Name: "/js", Description: "执行 JavaScript", Category: "执行"},
		{ID: "shell", Name: "/shell", Description: "执行 Shell", Category: "执行"},

		{ID: "theme", Name: "/theme", Description: "主题设置", Category: "显示"},
		{ID: "mode", Name: "/mode", Description: "模式设置", Category: "显示"},
		{ID: "voice", Name: "/voice", Description: "语音设置", Category: "显示"},

		{ID: "mcp", Name: "/mcp", Description: "MCP 服务器", Category: "MCP"},
		{ID: "hooks", Name: "/hooks", Description: "钩子管理", Category: "MCP"},
		{ID: "skills", Name: "/skills", Description: "技能管理", Category: "MCP"},
	}
}

func (cp *CommandPalette) Show() {
	cp.visible = true
	cp.input.SetValue("")
	cp.input.Focus()
	cp.filtered = cp.items
	cp.selected = 0
}

func (cp *CommandPalette) Hide() {
	cp.visible = false
}

func (cp *CommandPalette) IsVisible() bool {
	return cp.visible
}

func (cp *CommandPalette) SetSize(width, height int) {
	cp.width = width
	cp.height = height
}

func (cp *CommandPalette) Update(msg tea.Msg) (CommandPalette, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			cp.Hide()
			return *cp, nil

		case tea.KeyUp:
			if cp.selected > 0 {
				cp.selected--
			}
			return *cp, nil

		case tea.KeyDown:
			if cp.selected < len(cp.filtered)-1 {
				cp.selected++
			}
			return *cp, nil

		case tea.KeyEnter:
			if len(cp.filtered) > 0 && cp.selected < len(cp.filtered) {
				selected := cp.filtered[cp.selected]
				cp.Hide()
				if selected.Handler != nil {
					return *cp, selected.Handler()
				}
			}
			return *cp, nil
		}
	}

	cp.input, cmd = cp.input.Update(msg)
	cp.filterCommands()

	return *cp, cmd
}

func (cp *CommandPalette) filterCommands() {
	query := strings.ToLower(cp.input.Value())
	if query == "" {
		cp.filtered = cp.items
		cp.selected = 0
		return
	}

	var filtered []CommandPaletteItem
	for _, item := range cp.items {
		if strings.Contains(strings.ToLower(item.Name), query) ||
			strings.Contains(strings.ToLower(item.Description), query) ||
			strings.Contains(strings.ToLower(item.Category), query) {
			filtered = append(filtered, item)
		}
	}

	cp.filtered = filtered
	if cp.selected >= len(cp.filtered) {
		cp.selected = max(0, len(cp.filtered)-1)
	}
}

func (cp *CommandPalette) View() string {
	if !cp.visible {
		return ""
	}

	var sb strings.Builder

	inputStyle := cp.styles.container.Width(min(cp.width-4, 60))
	sb.WriteString(inputStyle.Render(cp.input.View()))
	sb.WriteString("\n")

	maxItems := min(cp.height-6, 15)
	startIdx := 0
	if cp.selected >= maxItems {
		startIdx = cp.selected - maxItems + 1
	}

	endIdx := min(startIdx+maxItems, len(cp.filtered))

	var currentCategory string
	for i := startIdx; i < endIdx; i++ {
		item := cp.filtered[i]

		if item.Category != currentCategory {
			currentCategory = item.Category
			sb.WriteString("\n")
			sb.WriteString(cp.styles.category.Render("  " + currentCategory))
			sb.WriteString("\n")
		}

		style := cp.styles.item
		if i == cp.selected {
			style = cp.styles.selected
			sb.WriteString(style.Render("▶ " + item.Name + " - " + item.Description))
		} else {
			sb.WriteString(style.Render("  " + item.Name + " - " + item.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(cp.styles.help.Render("  ↑/↓ 导航  Enter 选择  Esc 关闭"))

	return sb.String()
}

func (cp *CommandPalette) GetSelectedCommand() string {
	if len(cp.filtered) > 0 && cp.selected < len(cp.filtered) {
		return cp.filtered[cp.selected].Name
	}
	return ""
}

func (cp *CommandPalette) AddCommand(item CommandPaletteItem) {
	cp.items = append(cp.items, item)
	sort.Slice(cp.items, func(i, j int) bool {
		if cp.items[i].Category != cp.items[j].Category {
			return cp.items[i].Category < cp.items[j].Category
		}
		return cp.items[i].Name < cp.items[j].Name
	})
}

func (cp *CommandPalette) RemoveCommand(id string) {
	var filtered []CommandPaletteItem
	for _, item := range cp.items {
		if item.ID != id {
			filtered = append(filtered, item)
		}
	}
	cp.items = filtered
}
