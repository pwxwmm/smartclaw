package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/compact"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/services"
	"github.com/instructkr/smartclaw/internal/session"
	"github.com/instructkr/smartclaw/internal/voice"
)

type Model struct {
	textArea           textarea.Model
	output             []string
	rawOutput          []string
	outputEnhancer     *OutputEnhancer
	codeExecutor       *CodeExecutor
	modelManager       *ModelManager
	gitManager         *GitManager
	editorManager      *EditorManager
	agentManager       *AgentManager
	templateManager    *TemplateManager
	commandPalette     *CommandPalette
	subagentManager    *SubagentManager
	memoryStore        *memory.MemoryStore
	voiceManager       *voice.VoiceManager
	history            []string
	historyIndex       int
	width              int
	height             int
	ready              bool
	showHelp           bool
	model              string
	tokens             int
	cost               float64
	theme              Theme
	mode               string
	statusBar          *StatusBar
	contextViz         *ContextVisualization
	contextManager     *ContextManager
	historySearch      *HistorySearchMode
	agentStatus        *AgentStatusLine
	spinner            *Spinner
	spinnerFrame       int
	loading            bool
	dialog             *Dialog
	showDialog         bool
	tabs               *Tab
	mouse              *MouseSupport
	updater            *AutoUpdater
	autocomplete       *AutoComplete
	streaming          *StreamingOutput
	imageViewer        *ImageViewer
	apiClient          *api.Client
	apiMu              *sync.Mutex
	messages           []api.Message
	showThinking       bool
	currentResponse    strings.Builder
	markdown           *MarkdownRenderer
	viewportOffset     int
	streamingIdx       int
	selection          *Selection
	selectMode         bool
	copyFeedback       string
	sessionManager     *session.Manager
	currentSession     *session.Session
	autoSave           bool
	editMode           bool
	lastError          *SmartError
	lastInput          string
	workDir            string
	fileCompletions    []string
	completionIndex    int
	showCompletions    bool
	completionPage     int
	completionPageSize int
	compactService     *compact.CompactService
	autoCompactEnabled bool
	sessionRecorder    *services.SessionRecorder
}

func InitialModel() Model {
	ta := textarea.New()
	ta.Placeholder = "Ask me anything... (Tab to toggle edit mode)"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(3)

	theme := GetTheme()
	statusBar := NewStatusBar(80)
	contextViz := NewContextVisualization(200000, 30)
	contextManager := NewContextManager(200000)
	outputEnhancer := NewOutputEnhancer()
	historySearch := NewHistorySearchMode()
	agentStatus := NewAgentStatusLine(80)
	spinner := NewSpinner("Thinking...")
	tabs := NewTabs([]string{"Chat", "Context", "Tools", "Settings"})
	mouse := NewMouseSupport()
	updater := NewAutoUpdater("1.0.0")
	autocomplete := NewAutoComplete(8, 60)
	streaming := NewStreamingOutput()
	imageViewer := NewImageViewer(80, 20)
	markdown := NewMarkdownRenderer(theme)

	sessionManager, err := session.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize session manager: %v\n", err)
	}

	var currentSession *session.Session
	if sessionManager != nil {
		currentSession = sessionManager.NewSession("claude-sonnet-4-5")
	}

	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}

	codeExecutor := NewCodeExecutor(workDir)
	modelManager := NewModelManager("claude-sonnet-4-5")
	gitManager := NewGitManager(workDir)
	editorManager := NewEditorManager(workDir)
	agentManager := NewAgentManager(workDir)
	templateManager := NewTemplateManager()
	commandPalette := NewCommandPalette()
	subagentManager := NewSubagentManager(workDir, agentManager)

	memStore, err := memory.NewMemoryStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize memory store: %v\n", err)
	}

	voiceMgr := voice.NewVoiceManager(voice.VoiceConfig{
		Mode:             voice.VoiceModeDisabled,
		Language:         "en",
		Model:            "whisper-1",
		SampleRate:       16000,
		RecordingTimeout: 30,
		SilenceThreshold: 3,
		VadEnabled:       true,
		VadThreshold:     0.5,
	})

	return Model{
		textArea:           ta,
		output:             make([]string, 0),
		rawOutput:          make([]string, 0),
		outputEnhancer:     outputEnhancer,
		codeExecutor:       codeExecutor,
		modelManager:       modelManager,
		gitManager:         gitManager,
		editorManager:      editorManager,
		agentManager:       agentManager,
		templateManager:    templateManager,
		commandPalette:     commandPalette,
		subagentManager:    subagentManager,
		memoryStore:        memStore,
		voiceManager:       voiceMgr,
		history:            make([]string, 0),
		historyIndex:       -1,
		model:              "claude-sonnet-4-5",
		theme:              theme,
		mode:               "ask",
		statusBar:          statusBar,
		contextViz:         contextViz,
		contextManager:     contextManager,
		historySearch:      historySearch,
		agentStatus:        agentStatus,
		spinner:            spinner,
		spinnerFrame:       0,
		loading:            false,
		tabs:               tabs,
		mouse:              mouse,
		updater:            updater,
		autocomplete:       autocomplete,
		streaming:          streaming,
		imageViewer:        imageViewer,
		messages:           make([]api.Message, 0),
		apiMu:              &sync.Mutex{},
		showThinking:       viper.GetBool("show_thinking"),
		markdown:           markdown,
		viewportOffset:     0,
		streamingIdx:       -1,
		selection:          &Selection{},
		selectMode:         false,
		copyFeedback:       "",
		sessionManager:     sessionManager,
		currentSession:     currentSession,
		autoSave:           viper.GetBool("auto_save"),
		editMode:           false,
		lastError:          nil,
		lastInput:          "",
		workDir:            workDir,
		fileCompletions:    make([]string, 0),
		completionIndex:    0,
		showCompletions:    false,
		completionPage:     0,
		completionPageSize: 10,
		compactService:     compact.NewCompactService(compact.DefaultCompactConfig("claude-sonnet-4-5", 200000)),
		autoCompactEnabled: true,
	}
}

func InitialModelWithClient(client *api.Client) Model {
	m := InitialModel()
	m.apiClient = client
	if client != nil {
		m.model = client.Model
	}
	return m
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.commandPalette != nil && m.commandPalette.IsVisible() {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.Type == tea.KeyEsc {
				m.commandPalette.Hide()
				return m, nil
			}
		}
		cp, cmd := m.commandPalette.Update(msg)
		m.commandPalette = &cp
		if cmd != nil {
			selectedCmd := m.commandPalette.GetSelectedCommand()
			if selectedCmd != "" {
				return m, ProcessSlashCommand(selectedCmd, &m)
			}
		}
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.copyFeedback = ""
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			return m, tea.Quit
		case tea.KeyEsc:
			if m.showCompletions {
				m.showCompletions = false
				m.fileCompletions = nil
				return m, nil
			}
		case tea.KeyTab:
			m.editMode = !m.editMode
			return m, nil
		case tea.KeyEnter:
			if m.showCompletions && len(m.fileCompletions) > 0 {
				selected := m.fileCompletions[m.completionIndex]

				currentInput := m.textArea.Value()
				lastAtIndex := strings.LastIndex(currentInput, "@")
				if lastAtIndex != -1 {
					beforeAt := currentInput[:lastAtIndex+1]

					dir, _ := ExtractFilePrefix(currentInput)
					if dir != "" {
						beforeAt += dir + "/"
					}

					newInput := beforeAt + selected + " "
					m.textArea.SetValue(newInput)
					m.showCompletions = false
					m.fileCompletions = nil
				}
				return m, nil
			}

			if m.editMode {
				var cmd tea.Cmd
				m.textArea, cmd = m.textArea.Update(tea.KeyMsg{Type: tea.KeyEnter})
				return m, cmd
			}

			input := m.textArea.Value()
			if input == "" {
				return m, nil
			}
			m.history = append(m.history, input)
			m.historyIndex = len(m.history)
			m.textArea.SetValue("")

			if strings.HasPrefix(input, "/") {
				return m, ProcessSlashCommand(input, &m)
			}
			return m, m.processInput(input)
		case tea.KeyCtrlJ:
			var cmd tea.Cmd
			m.textArea, cmd = m.textArea.Update(tea.KeyMsg{Type: tea.KeyEnter})
			return m, cmd
		case tea.KeyRight:
			if m.showCompletions && len(m.fileCompletions) > 0 {
				selected := m.fileCompletions[m.completionIndex]

				if strings.HasSuffix(selected, "/") {
					currentInput := m.textArea.Value()
					lastAtIndex := strings.LastIndex(currentInput, "@")
					if lastAtIndex != -1 {
						beforeAt := currentInput[:lastAtIndex+1]

						dir, _ := ExtractFilePrefix(currentInput)
						if dir != "" {
							beforeAt += dir + "/"
						}

						newInput := beforeAt + selected
						m.textArea.SetValue(newInput)
						m.completionPage = 0
						m.completionIndex = 0

						dir, prefix := ExtractFilePrefix(newInput)
						searchDir := m.workDir
						if dir != "" {
							searchDir = filepath.Join(m.workDir, dir)
						}
						files, err := GetFilesInDirectory(searchDir)
						if err == nil {
							m.fileCompletions = FilterCompletions(files, prefix)
							m.showCompletions = len(m.fileCompletions) > 0
						} else {
							m.showCompletions = false
							m.fileCompletions = nil
						}

						return m, nil
					}
				}
			}
		case tea.KeyUp:
			if m.showCompletions && len(m.fileCompletions) > 0 {
				if m.completionIndex > 0 {
					m.completionIndex--
				} else if m.completionPage > 0 {
					m.completionPage--
					m.completionIndex = m.completionPageSize - 1
				}
				return m, nil
			}
			if m.textArea.Line() == 0 && m.historyIndex > 0 {
				m.historyIndex--
				m.textArea.SetValue(m.history[m.historyIndex])
				return m, nil
			}
		case tea.KeyDown:
			if m.showCompletions && len(m.fileCompletions) > 0 {
				pageStart := m.completionPage * m.completionPageSize
				pageEnd := pageStart + m.completionPageSize
				if pageEnd > len(m.fileCompletions) {
					pageEnd = len(m.fileCompletions)
				}
				itemsInPage := pageEnd - pageStart

				if m.completionIndex < itemsInPage-1 {
					m.completionIndex++
				} else if pageEnd < len(m.fileCompletions) {
					m.completionPage++
					m.completionIndex = 0
				}
				return m, nil
			}
			if m.textArea.Line() == 0 && m.historyIndex >= 0 && m.historyIndex < len(m.history)-1 {
				m.historyIndex++
				m.textArea.SetValue(m.history[m.historyIndex])
				return m, nil
			} else if m.textArea.Line() == 0 && m.historyIndex == len(m.history)-1 {
				m.historyIndex = len(m.history)
				m.textArea.SetValue("")
				return m, nil
			}
		case tea.KeyPgUp:
			m.viewportOffset = max(0, m.viewportOffset-10)
		case tea.KeyPgDown:
			totalLines := 0
			for _, msg := range m.output {
				totalLines += len(strings.Split(msg, "\n"))
			}
			estimatedHeight := m.height - 10
			if m.showHelp {
				estimatedHeight -= 12
			}
			if estimatedHeight <= 0 {
				estimatedHeight = 20
			}
			maxOffset := max(0, totalLines-estimatedHeight)
			m.viewportOffset = min(m.viewportOffset+10, maxOffset)
		case tea.KeyCtrlH:
			m.showHelp = !m.showHelp
		case tea.KeyCtrlS:
			if m.sessionManager == nil || m.currentSession == nil {
				m.copyFeedback = "✗ No active session to save"
				return m, nil
			}
			if err := m.sessionManager.Save(m.currentSession); err != nil {
				m.copyFeedback = "✗ Failed to save session"
			} else {
				m.copyFeedback = "✓ Session saved"
			}
			return m, nil
		case tea.KeyCtrlL:
			ClearOutput(&m)
			return m, nil
		case tea.KeyCtrlT:
			if m.outputEnhancer != nil {
				m.outputEnhancer.ToggleTimestamps()
				if m.outputEnhancer.showTimestamp {
					m.copyFeedback = "✓ Timestamps enabled"
				} else {
					m.copyFeedback = "✓ Timestamps disabled"
				}
			}
			return m, nil
		case tea.KeyCtrlF:
			if m.outputEnhancer != nil {
				m.outputEnhancer.mode = (m.outputEnhancer.mode + 1) % 3
				modes := []string{"all", "code only", "text only"}
				m.copyFeedback = fmt.Sprintf("✓ Filter: %s", modes[m.outputEnhancer.mode])
			}
			return m, nil
		case tea.KeyCtrlR:
			if m.lastError == nil {
				m.copyFeedback = "✗ No previous error to retry"
				return m, nil
			}
			if !m.lastError.Retryable {
				m.copyFeedback = "✗ Error is not retryable"
				return m, nil
			}
			if m.lastInput == "" {
				m.copyFeedback = "✗ No previous input to retry"
				return m, nil
			}
			m.copyFeedback = "✓ Retrying..."
			return m, m.processInput(m.lastInput)
		case tea.KeyCtrlK:
			if m.commandPalette != nil {
				m.commandPalette.Show()
			}
			return m, nil
		default:
			switch msg.String() {
			case "c":
				estimatedHeight := m.height - 10
				if m.showHelp {
					estimatedHeight -= 12
				}
				if estimatedHeight <= 0 {
					estimatedHeight = 20
				}
				text := m.GetVisibleText(estimatedHeight)
				if text != "" {
					if err := CopyToClipboard(text); err != nil {
						m.copyFeedback = "✗ Failed to copy"
					} else {
						m.copyFeedback = "✓ Copied visible text"
					}
				}
			case "C":
				text := m.GetLastMessage()
				if text != "" {
					if err := CopyToClipboard(text); err != nil {
						m.copyFeedback = "✗ Failed to copy"
					} else {
						m.copyFeedback = "✓ Copied last message"
					}
				}
			case "b":
				text := m.GetLastCodeBlock()
				if text != "" {
					if err := CopyToClipboard(text); err != nil {
						m.copyFeedback = "✗ Failed to copy"
					} else {
						m.copyFeedback = "✓ Copied last code block"
					}
				}
			case "a":
				text := m.GetAllMessages()
				if text != "" {
					if err := CopyToClipboard(text); err != nil {
						m.copyFeedback = "✗ Failed to copy"
					} else {
						m.copyFeedback = "✓ Copied all messages"
					}
				}
			}
		}
	case tea.MouseMsg:
		switch msg.Type {
		case tea.MouseWheelUp:
			m.viewportOffset = max(0, m.viewportOffset-3)
		case tea.MouseWheelDown:
			totalLines := 0
			for _, outputMsg := range m.output {
				totalLines += len(strings.Split(outputMsg, "\n"))
			}
			estimatedHeight := m.height - 10
			if m.showHelp {
				estimatedHeight -= 12
			}
			if estimatedHeight <= 0 {
				estimatedHeight = 20
			}
			maxOffset := max(0, totalLines-estimatedHeight)
			m.viewportOffset = min(m.viewportOffset+3, maxOffset)
		}
		return m, nil
	case TickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % 4
		if m.loading && m.streamingIdx >= 0 && m.streamingIdx < len(m.output) && m.streamingIdx < len(m.rawOutput) {
			currentContent := m.currentResponse.String()
			m.output[m.streamingIdx] = m.formatAssistantOutput(currentContent)
			return m, tickCmd()
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textArea.SetWidth(msg.Width - 10)
		m.ready = true
	case UserInputMsg:
		m.output = append(m.output, m.formatAssistantOutput("Processing: "+msg.text))
		m.rawOutput = append(m.rawOutput, "SmartClaw: Processing: "+msg.text)
	case APICallMsg:
		m.output = append(m.output, m.formatUserInput(msg.text))
		m.output = append(m.output, m.formatAssistantOutput(""))
		m.rawOutput = append(m.rawOutput, "\nYou: "+msg.text)
		m.rawOutput = append(m.rawOutput, "")

		m.lastInput = msg.text
		m.lastError = nil

		if m.currentSession != nil {
			m.currentSession.AddMessage("user", msg.text)
		}

		m.contextManager.AddMessage("user", msg.text, 0)

		if m.sessionRecorder != nil && m.sessionRecorder.IsRecording() {
			m.sessionRecorder.RecordMessage("user", msg.text)
		}

		m.streamingIdx = len(m.output) - 1
		m.currentResponse.Reset()
		m.loading = true
		return m, tea.Batch(m.callAPI(msg.text), tickCmd())
	case APIResponseMsg:
		m.loading = false
		if m.streamingIdx >= 0 && m.streamingIdx < len(m.output) && m.streamingIdx < len(m.rawOutput) {
			m.output[m.streamingIdx] = m.formatAssistantOutput(msg.text)
			m.rawOutput[m.streamingIdx] = "\nSmartClaw: " + msg.text

			if m.currentSession != nil {
				m.currentSession.AddMessage("assistant", msg.text)
				m.currentSession.Tokens = m.tokens
				m.currentSession.Cost = m.cost

				if m.autoSave && m.sessionManager != nil {
					m.sessionManager.Save(m.currentSession)
				}
			}

			m.contextManager.AddMessage("assistant", msg.text, msg.tokens)

			if m.sessionRecorder != nil && m.sessionRecorder.IsRecording() {
				m.sessionRecorder.RecordMessage("assistant", msg.text)
			}
		}
		m.tokens += msg.tokens
		m.streamingIdx = -1

		if m.autoCompactEnabled && m.compactService != nil {
			if shouldCompact, warning := m.compactService.ShouldCompact(m.tokens); shouldCompact {
				removed := m.contextManager.CompressOldMessages(5)
				if removed > 0 {
					AddOutput(&m, m.formatAssistantOutput(fmt.Sprintf(
						"🔄 Auto-compacted: removed %d messages (%s)", removed, warning.Message)))
				}
			}
		}
	case CommandMsg:
		return m, ProcessSlashCommand(msg.cmd, &m)
	case OutputMsg:
		m.output = append(m.output, msg.text)
		m.rawOutput = append(m.rawOutput, msg.text)
	case ErrorMsg:
		smartErr := ClassifyError(fmt.Errorf("%s", msg.err))
		m.lastError = smartErr
		m.output = append(m.output, m.formatSmartError(smartErr))
		m.rawOutput = append(m.rawOutput, smartErr.Message)
	}

	var cmd tea.Cmd
	m.textArea, cmd = m.textArea.Update(msg)
	cmds = append(cmds, cmd)

	currentInput := m.textArea.Value()
	if strings.Contains(currentInput, "@") {
		dir, prefix := ExtractFilePrefix(currentInput)
		searchDir := m.workDir
		if dir != "" {
			searchDir = filepath.Join(m.workDir, dir)
		}

		files, err := GetFilesInDirectory(searchDir)
		if err == nil {
			completions := FilterCompletions(files, prefix)
			if len(completions) > 0 {
				if !m.showCompletions || len(completions) != len(m.fileCompletions) {
					m.completionPage = 0
					m.completionIndex = 0
				}
				m.fileCompletions = completions
				m.showCompletions = true
			} else {
				m.showCompletions = false
			}
		} else {
			m.showCompletions = false
		}
	} else {
		m.showCompletions = false
		m.fileCompletions = nil
		m.completionPage = 0
		m.completionIndex = 0
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return RenderWelcomeScreen(m.theme, 80)
	}

	if m.commandPalette != nil && m.commandPalette.IsVisible() {
		return m.commandPalette.View()
	}

	var sb strings.Builder

	sb.WriteString(m.renderHeader())
	sb.WriteString("\n")

	outputHeight := m.height - 10
	if m.showHelp {
		outputHeight -= 12
	}
	sb.WriteString(m.renderOutput(outputHeight))
	sb.WriteString("\n")

	if m.loading {
		sb.WriteString(m.renderLoading())
		sb.WriteString("\n")
	}

	sb.WriteString(m.renderInput())
	sb.WriteString("\n")

	if m.showCompletions && len(m.fileCompletions) > 0 {
		sb.WriteString(m.renderFileCompletions())
		sb.WriteString("\n")
	}

	if m.copyFeedback != "" {
		feedbackStyle := lipgloss.NewStyle().
			Foreground(m.theme.Success).
			Padding(0, 1)
		sb.WriteString(feedbackStyle.Render(m.copyFeedback))
		sb.WriteString("\n")
	}

	sb.WriteString(m.renderStatus())

	if m.showHelp {
		sb.WriteString("\n")
		sb.WriteString(m.renderHelp())
	}

	return sb.String()
}

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

	return borderStyle.Render("╰"+strings.Repeat("─", m.width-2)+"╯\n") +
		m.statusBar.Render()
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

func (m Model) formatAssistantOutput(output string) string {
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

	return header + "\n" + contentStyle.Render(rendered)
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

func (m Model) processInput(input string) tea.Cmd {
	return func() tea.Msg {
		if strings.HasPrefix(input, "/") {
			return CommandMsg{cmd: input}
		}

		if m.apiClient == nil {
			return ErrorMsg{err: "No API key configured. Use /set-api-key or set ANTHROPIC_API_KEY"}
		}

		processedInput := input
		if DetectFileReferences(input) {
			_, processedInput = ParseFileReferences(input, m.workDir)
		}

		return APICallMsg{text: processedInput}
	}
}

type APICallMsg struct {
	text string
}

type APIResponseMsg struct {
	text   string
	tokens int
}

type UserInputMsg struct {
	text string
}

type CommandMsg struct {
	cmd string
}

type OutputMsg struct {
	text string
}

type ErrorMsg struct {
	err string
}

type StreamChunkMsg struct {
	chunk string
}

type TickMsg struct{}

type streamingState struct {
	mu   sync.Mutex
	text strings.Builder
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m Model) Init() tea.Cmd {
	return tickCmd()
}

func (m *Model) callAPI(input string) tea.Cmd {
	return func() tea.Msg {
		m.apiMu.Lock()
		defer m.apiMu.Unlock()

		userMsg := api.Message{
			Role:    "user",
			Content: []api.ContentBlock{{Type: "text", Text: input}},
		}
		m.messages = append(m.messages, userMsg)

		req := &api.MessageRequest{
			Model:     m.model,
			MaxTokens: 4096,
			Messages:  m.messages,
		}

		m.currentResponse.Reset()
		parser := api.NewStreamMessageParser()

		var err error
		if m.apiClient.IsOpenAI {
			err = m.apiClient.StreamMessageOpenAI(context.Background(), req, func(event string, data []byte) error {
				result, err := parser.HandleEvent(event, data)
				if err != nil {
					return err
				}

				if result.TextDelta != "" {
					m.currentResponse.WriteString(result.TextDelta)
				}

				if m.showThinking && result.ThinkingDelta != "" {
					m.currentResponse.WriteString(result.ThinkingDelta)
				}

				return nil
			})
		} else {
			err = m.apiClient.StreamMessageSSE(context.Background(), req, func(event string, data []byte) error {
				result, err := parser.HandleEvent(event, data)
				if err != nil {
					return err
				}

				if result.TextDelta != "" {
					m.currentResponse.WriteString(result.TextDelta)
				}

				if m.showThinking && result.ThinkingDelta != "" {
					m.currentResponse.WriteString(result.ThinkingDelta)
				}

				return nil
			})
		}

		if err != nil {
			return ErrorMsg{err: err.Error()}
		}

		finalResponse := m.currentResponse.String()
		m.currentResponse.Reset()

		assistantMsg := api.Message{
			Role:    "assistant",
			Content: []api.ContentBlock{{Type: "text", Text: finalResponse}},
		}
		m.messages = append(m.messages, assistantMsg)

		return APIResponseMsg{text: finalResponse, tokens: 0}
	}
}

func AddOutput(m *Model, text string) {
	m.output = append(m.output, text)
	// Also add plain text to rawOutput for copy functionality
	cleanText := RemoveANSIColors(text)
	m.rawOutput = append(m.rawOutput, cleanText)
}

func AddError(m *Model, err string) {
	m.output = append(m.output, m.formatError(err))
	m.rawOutput = append(m.rawOutput, "Error: "+err)
}

func ClearOutput(m *Model) {
	m.output = make([]string, 0)
	m.rawOutput = make([]string, 0)
	m.streamingIdx = -1
	m.currentResponse.Reset()
	m.viewportOffset = 0
	m.tokens = 0
	m.cost = 0
	m.loading = false
	m.messages = make([]api.Message, 0)
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
