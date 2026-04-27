package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderHeader() string {
	logo := NewLogoRenderer(m.theme)

	titleStyle := lipgloss.NewStyle().
		Foreground(m.theme.Primary).
		Bold(true).
		Padding(0, 1)

	statsStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Padding(0, 1)

	left := logo.RenderInline()
	right := statsStyle.Render(fmt.Sprintf(" %s | %d tokens | $%.4f", m.model, m.tokens, m.cost))

	tabs := m.renderTabs()

	header := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top,
			titleStyle.Render(left),
			strings.Repeat(" ", max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)-4)),
			right,
		),
		tabs,
	)

	borderStyle := lipgloss.NewStyle().
		Foreground(m.theme.Border)

	return borderStyle.Render("╭"+strings.Repeat("─", m.width-2)+"╮\n") +
		header +
		borderStyle.Render("├"+strings.Repeat("─", m.width-2)+"┤")
}

func (m Model) renderTabs() string {
	tabNames := []string{"Chat", "Context", "Tools", "Settings"}
	var tabs []string

	for i, name := range tabNames {
		style := m.theme.TabStyle(i == 0)
		tabs = append(tabs, style.Render(name))
	}

	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(lipgloss.JoinHorizontal(lipgloss.Left, tabs...))
}

func (m Model) renderOutput(height int) string {
	if len(m.output) == 0 {
		welcomeStyle := lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Italic(true).
			Padding(2, 1)

		logo := NewLogoRenderer(m.theme)
		content := lipgloss.JoinVertical(lipgloss.Center,
			logo.Render("compact"),
			"",
			welcomeStyle.Render("Start a conversation by typing your message below."),
			welcomeStyle.Render("Type /help for available commands."),
		)

		boxStyle := lipgloss.NewStyle().
			Padding(1, 2).
			Width(m.width - 4)

		return boxStyle.Render(content)
	}

	totalLines := 0
	for _, msg := range m.output {
		totalLines += len(strings.Split(msg, "\n"))
	}

	maxOffset := max(0, totalLines-height)
	if m.viewportOffset > maxOffset {
		m.viewportOffset = maxOffset
	}

	startLine := m.viewportOffset
	endLine := startLine + height
	if endLine > totalLines {
		endLine = totalLines
	}

	var visibleLines []string
	currentLine := 0
	for _, msg := range m.output {
		msgLines := strings.Split(msg, "\n")
		for _, line := range msgLines {
			if currentLine >= startLine && currentLine < endLine {
				visibleLines = append(visibleLines, line)
			}
			currentLine++
		}
	}

	content := strings.Join(visibleLines, "\n")

	scrollIndicator := ""
	if totalLines > height {
		scrollIndicator = lipgloss.NewStyle().
			Foreground(m.theme.TextMuted).
			Render(fmt.Sprintf("[%d/%d]", m.viewportOffset+1, totalLines))
	}

	if scrollIndicator != "" {
		content = scrollIndicator + "\n" + content
	}

	outputStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Width(m.width - 2)

	return outputStyle.Render(content)
}

func (m Model) renderInput() string {
	promptStyle := lipgloss.NewStyle().
		Foreground(m.theme.Primary).
		Bold(true).
		Padding(0, 1)

	owl := lipgloss.NewStyle().
		Foreground(m.theme.Success).
		Render("(o,o)")

	prompt := promptStyle.Render(owl + " → ")

	line := m.textArea.Line() + 1
	position := fmt.Sprintf("Ln %d", line)

	positionStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Padding(0, 1)

	mode := "发送模式"
	modeStyle := lipgloss.NewStyle().
		Foreground(m.theme.Success).
		Bold(true)
	if m.editMode {
		mode = "编辑模式"
		modeStyle = lipgloss.NewStyle().
			Foreground(m.theme.Warning).
			Bold(true)
	}

	modeText := modeStyle.Render(fmt.Sprintf("【%s】", mode))
	help := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Render("Tab: 切换模式 | ↑↓: history")

	textareaView := m.textArea.View()

	bgPattern := strings.NewReplacer(
		"\x1b[40m", "",
		"\x1b[41m", "",
		"\x1b[42m", "",
		"\x1b[43m", "",
		"\x1b[44m", "",
		"\x1b[45m", "",
		"\x1b[46m", "",
		"\x1b[47m", "",
		"\x1b[48;2;", "",
		"\x1b[48;5;", "",
	)
	textareaView = bgPattern.Replace(textareaView)

	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Border).
		Padding(0, 1).
		Width(m.width - 4)

	return inputBox.Render(prompt+textareaView) + "\n" +
		positionStyle.Render(position) + "  " + modeText + "  " + help
}

func (m Model) renderFileCompletions() string {
	if len(m.fileCompletions) == 0 {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(m.theme.Primary).
		Bold(true).
		Padding(0, 1)

	itemStyle := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(m.theme.Primary).
		Background(lipgloss.Color("236")).
		Bold(true).
		Padding(0, 1)

	var lines []string

	totalPages := (len(m.fileCompletions) + m.completionPageSize - 1) / m.completionPageSize
	title := fmt.Sprintf("📁 File Completions (Page %d/%d, Total: %d)", m.completionPage+1, totalPages, len(m.fileCompletions))
	lines = append(lines, titleStyle.Render(title))

	pageStart := m.completionPage * m.completionPageSize
	pageEnd := pageStart + m.completionPageSize
	if pageEnd > len(m.fileCompletions) {
		pageEnd = len(m.fileCompletions)
	}

	for i := pageStart; i < pageEnd; i++ {
		item := m.fileCompletions[i]
		if i-pageStart == m.completionIndex {
			lines = append(lines, selectedStyle.Render("  → "+item))
		} else {
			lines = append(lines, itemStyle.Render("    "+item))
		}
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted).
		Padding(0, 1)

	helpText := "↑↓: navigate | Enter: select | Esc: cancel"
	if totalPages > 1 {
		helpText = "↑↓: navigate & page | Enter: select | Esc: cancel"
	}
	lines = append(lines, helpStyle.Render("  "+helpText))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Border).
		Padding(0, 1).
		Width(m.width - 4)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderStatus() string {
	m.statusBar.SetModel(m.model)
	m.statusBar.SetTokens(m.tokens)
	m.statusBar.SetCost(m.cost)
	m.statusBar.SetMode(m.mode)

	borderStyle := lipgloss.NewStyle().
		Foreground(m.theme.Border)

	hintStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted)

	mouseState := "🖱scroll"
	if !m.mouseEnabled {
		mouseState = "✋select"
	}
	hints := hintStyle.Render(fmt.Sprintf(" PgUp/PgDn:scroll │ Ctrl+G:%s │ c:copy │ C:copy-msg │ b:copy-code ", mouseState))

	return borderStyle.Render("╰"+strings.Repeat("─", m.width-2)+"╯\n") +
		m.statusBar.Render() + "\n" + hints
}

func (m Model) renderHelp() string {
	cmdStyle := lipgloss.NewStyle().
		Foreground(m.theme.Primary).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted)

	commands := []struct {
		cmd  string
		desc string
	}{
		{"/help", "Show available commands"},
		{"/session", "Session management"},
		{"/status", "Show session status"},
		{"/model", "Model management"},
		{"/theme", "Change color theme"},
		{"/clear", "Clear output"},
		{"/exit", "Exit SmartClaw"},
		{"/voice", "Voice input mode"},
		{"/cost", "Show token usage"},
		{"/retry", "Retry last failed request"},
		{"", ""},
		{"Model Management", ""},
		{"/model", "Show current model"},
		{"/model list", "List all models"},
		{"/model switch <id>", "Switch model"},
		{"/model info <id>", "Model details"},
		{"/model compare <ids>", "Compare models"},
		{"", ""},
		{"Context Management", ""},
		{"/context", "Show context statistics"},
		{"/context list", "List all messages"},
		{"/context remove <id>", "Remove message"},
		{"/context keep <id>", "Mark as important"},
		{"/context compress [n]", "Compress old messages"},
		{"/context clear", "Clear non-kept messages"},
		{"", ""},
		{"Code Execution", ""},
		{"/run <lang> <code>", "Execute code (python/js/go/bash)"},
		{"/run auto <code>", "Auto-detect and execute"},
		{"/python <code>", "Execute Python code"},
		{"/js <code>", "Execute JavaScript code"},
		{"/shell <cmd>", "Execute shell command"},
		{"", ""},
		{"Git Integration", ""},
		{"/git status", "Show repository status"},
		{"/git log [n]", "Show commit history"},
		{"/git branches", "List all branches"},
		{"/git diff", "Show changes"},
		{"/git add <files>", "Stage files"},
		{"/git commit <msg>", "Create commit"},
		{"/git push", "Push to remote"},
		{"/git pull", "Pull from remote"},
		{"/git checkout <br>", "Switch branch"},
		{"/git branch <name>", "Create branch"},
		{"", ""},
		{"Editor Integration", ""},
		{"/edit", "Open editor for new content"},
		{"/edit <file>", "Edit existing file"},
		{"/multilines", "Edit multiline input"},
		{"/editor", "Show current editor"},
		{"/editor list", "List available editors"},
		{"/editor <name>", "Set editor (vim/nvim/nano/code)"},
		{"", ""},
		{"Output Enhancement", ""},
		{"Ctrl+T", "Toggle timestamps"},
		{"Ctrl+F", "Filter output (all/code/text)"},
		{"", ""},
		{"File References", ""},
		{"@filename", "Read file content"},
		{"@file:10-20", "Read specific lines"},
		{"@./path/file", "Relative path"},
		{"↑↓", "Navigate & page files"},
		{"Enter", "Select file"},
		{"→", "Enter folder"},
		{"Esc", "Cancel completion"},
		{"", ""},
		{"Shortcuts", ""},
		{"Tab", "Toggle edit/send mode"},
		{"Ctrl+S", "Save session"},
		{"Ctrl+L", "Clear output"},
		{"Ctrl+R", "Retry last request"},
		{"Ctrl+N", "New session"},
		{"Ctrl+H", "Toggle help"},
		{"Ctrl+G", "Toggle mouse (scroll vs select/copy)"},
		{"Ctrl+W", "Toggle thinking block expand/collapse"},
		{"", ""},
		{"Copy Shortcuts", ""},
		{"c", "Copy visible text"},
		{"C", "Copy last message"},
		{"b", "Copy last code block"},
		{"a", "Copy all messages"},
	}

	var lines []string
	titleStyle := lipgloss.NewStyle().
		Foreground(m.theme.Primary).
		Bold(true).
		Underline(true)

	lines = append(lines, titleStyle.Render("  Commands"))
	lines = append(lines, "")

	for _, c := range commands {
		if c.cmd == "" {
			if c.desc == "" {
				lines = append(lines, "")
			} else {
				headerStyle := lipgloss.NewStyle().
					Foreground(m.theme.TextMuted).
					Bold(true).
					Padding(1, 0)
				lines = append(lines, headerStyle.Render("  "+c.desc))
			}
		} else {
			line := "  " + cmdStyle.Render(c.cmd) +
				strings.Repeat(" ", 12-len(c.cmd)) +
				descStyle.Render(c.desc)
			lines = append(lines, line)
		}
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(0, 1).
		Width(50)

	return boxStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) formatUserInput(input string) string {
	userLabelStyle := lipgloss.NewStyle().
		Foreground(m.theme.OutputUser).
		Bold(true)

	textStyle := lipgloss.NewStyle().
		Foreground(m.theme.Text).
		PaddingLeft(2)

	var header string
	if m.outputEnhancer != nil && m.outputEnhancer.showTimestamp {
		timestamp := time.Now().Format("15:04")
		header = userLabelStyle.Render(fmt.Sprintf("▶ You [%s]:", timestamp))
	} else {
		header = userLabelStyle.Render("▶ You:")
	}

	return header + "\n" + textStyle.Render(input)
}

func (m Model) formatAssistantOutput(output string, rawPrefix ...string) string {
	asstLabelStyle := lipgloss.NewStyle().
		Foreground(m.theme.OutputAssistant).
		Bold(true)

	var rendered string
	if m.markdown != nil && m.width > 10 {
		rendered = m.markdown.RenderWithStyle(output, m.width-4)
	} else {
		rendered = output
	}

	contentStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	var header string
	if m.outputEnhancer != nil && m.outputEnhancer.showTimestamp {
		timestamp := time.Now().Format("15:04")
		header = asstLabelStyle.Render(fmt.Sprintf("◆ SmartClaw [%s]:", timestamp))
	} else {
		header = asstLabelStyle.Render("◆ SmartClaw:")
	}

	result := header + "\n"

	if len(rawPrefix) > 0 && rawPrefix[0] != "" {
		prefix := rawPrefix[0]
		indentedLines := make([]string, 0, strings.Count(prefix, "\n")+1)
		for _, line := range strings.Split(prefix, "\n") {
			indentedLines = append(indentedLines, "  "+line)
		}
		result += strings.Join(indentedLines, "\n") + "\n"
	}

	result += contentStyle.Render(rendered)
	return result
}

func (m Model) formatError(err string) string {
	return m.theme.ErrorStyle().Render("✗ " + err)
}

func (m Model) formatSmartError(smartErr *SmartError) string {
	if smartErr == nil {
		return ""
	}
	return m.theme.ErrorStyle().Render(smartErr.FormatError())
}

func (m Model) renderLoading() string {
	frames := []string{
		" (o,o)   ",
		" (O,O)   ",
		" (o,o)   ",
		" (¬,¬)   ",
	}

	frame := frames[m.spinnerFrame%len(frames)]

	style := lipgloss.NewStyle().
		Foreground(m.theme.Primary).
		Bold(true).
		Padding(0, 2)

	textStyle := lipgloss.NewStyle().
		Foreground(m.theme.TextMuted)

	return style.Render(frame) + textStyle.Render(" Thinking...")
}

func formatThinkingBlock(content string, expanded bool, termWidth int) string {
	if content == "" {
		return ""
	}

	if !expanded {
		label := lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true).
			Render(fmt.Sprintf("◈ Thought process (%d chars) — Ctrl+T to expand", len(content)))
		return label + "\n\n"
	}

	if termWidth < 40 {
		termWidth = 40
	}
	if termWidth > 120 {
		termWidth = 120
	}

	borderColor := lipgloss.Color("240")
	labelColor := lipgloss.Color("180")
	contentColor := lipgloss.Color("244")

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	labelStyle := lipgloss.NewStyle().Foreground(labelColor).Bold(true)
	contentStyle := lipgloss.NewStyle().Foreground(contentColor).Italic(true)

	innerWidth := termWidth - 4

	labelText := "◈ Thought Process"
	labelRendered := labelStyle.Render(labelText)
	labelWidth := lipgloss.Width(labelRendered)

	leftBorder := borderStyle.Render("╭─ ")
	cornerRight := borderStyle.Render("╮")
	dashCount := innerWidth - 3 - labelWidth - 1
	if dashCount < 1 {
		dashCount = 1
	}
	top := leftBorder + labelRendered + " " + borderStyle.Render(strings.Repeat("─", dashCount)) + cornerRight

	var wrappedLines []string
	for _, line := range strings.Split(content, "\n") {
		if line == "" {
			wrappedLines = append(wrappedLines, "")
			continue
		}
		for _, wl := range wrapLineRunes(line, innerWidth) {
			wrappedLines = append(wrappedLines, contentStyle.Render(wl))
		}
	}

	if len(wrappedLines) > 50 {
		wrappedLines = wrappedLines[:50]
		wrappedLines = append(wrappedLines, contentStyle.Render("... (truncated)"))
	}

	var bodyBuilder strings.Builder
	sideBorder := borderStyle.Render("│")
	for _, line := range wrappedLines {
		visualWidth := lipgloss.Width(line)
		padCount := innerWidth - visualWidth
		if padCount < 0 {
			padCount = 0
		}
		bodyBuilder.WriteString(sideBorder + " " + line + strings.Repeat(" ", padCount) + " " + sideBorder + "\n")
	}

	bottomLeft := borderStyle.Render("╰")
	bottomRight := borderStyle.Render("╯")
	bottomDashes := borderStyle.Render(strings.Repeat("─", innerWidth+2))
	bottom := bottomLeft + bottomDashes + bottomRight

	return top + "\n" + bodyBuilder.String() + bottom + "\n\n"
}

func wrapLineRunes(line string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{line}
	}
	runes := []rune(line)
	if len(runes) <= maxWidth {
		return []string{line}
	}
	var result []string
	for len(runes) > 0 {
		if len(runes) <= maxWidth {
			result = append(result, string(runes))
			break
		}
		breakAt := maxWidth
		for i := maxWidth; i > maxWidth/2; i-- {
			if runes[i] == ' ' {
				breakAt = i
				break
			}
		}
		result = append(result, string(runes[:breakAt]))
		runes = runes[breakAt:]
		if len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return result
}
