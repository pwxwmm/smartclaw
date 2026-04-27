package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/compact"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/plans"
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
	mouseEnabled       bool
	updater            *AutoUpdater
	autocomplete       *AutoComplete
	streaming          *StreamingOutput
	imageViewer        *ImageViewer
	apiClient          *api.Client
	apiMu              *sync.Mutex
	messages           []api.Message
	showThinking       bool
	streamState        *streamingState
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
	planStore          *plans.PlanStore
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
		currentSession = sessionManager.NewSession("claude-sonnet-4-5", "default")
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
		mouseEnabled:       false,
		updater:            updater,
		autocomplete:       autocomplete,
		streaming:          streaming,
		imageViewer:        imageViewer,
		messages:           make([]api.Message, 0),
		apiMu:              &sync.Mutex{},
		streamState:        &streamingState{},
		showThinking:       false,
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

func (m *Model) GetPlanStore() *plans.PlanStore {
	if m.planStore == nil {
		cwd := m.workDir
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		ps, err := plans.NewPlanStore(cwd)
		if err != nil {
			return nil
		}
		m.planStore = ps
	}
	return m.planStore
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
			return m, nil
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
			return m, nil
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
		case tea.KeyCtrlG:
			m.mouseEnabled = !m.mouseEnabled
			if m.mouseEnabled {
				m.copyFeedback = "Mouse: scroll enabled (Ctrl+G to select/copy)"
				return m, tea.EnableMouseCellMotion
			}
			m.copyFeedback = "Mouse: select/copy enabled (Ctrl+G to scroll)"
			return m, tea.DisableMouse
		case tea.KeyCtrlW:
			m.showThinking = !m.showThinking
			if m.showThinking {
				m.copyFeedback = "Thinking block: expanded"
			} else {
				m.copyFeedback = "Thinking block: collapsed"
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
			return m, nil
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
			return m, nil
		}
		if !m.mouseEnabled {
			return m, nil
		}
		return m, nil
	case TickMsg:
		m.spinnerFrame = (m.spinnerFrame + 1) % 4
		if m.loading && m.streamingIdx >= 0 && m.streamingIdx < len(m.output) && m.streamingIdx < len(m.rawOutput) {
			m.streamState.mu.Lock()
			currentContent := m.streamState.text.String()
			m.streamState.mu.Unlock()
			m.output[m.streamingIdx] = m.formatAssistantOutput(currentContent)
			m.scrollToBottom()
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
		m.scrollToBottom()
	case APICallMsg:
		m.output = append(m.output, m.formatUserInput(msg.text))
		m.output = append(m.output, m.formatAssistantOutput(""))
		m.rawOutput = append(m.rawOutput, "\nYou: "+msg.text)
		m.rawOutput = append(m.rawOutput, "")
		m.scrollToBottom()

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
		m.streamState.mu.Lock()
		m.streamState.text.Reset()
		m.streamState.thinking.Reset()
		m.streamState.mu.Unlock()
		m.loading = true
		return m, tea.Batch(m.callAPI(msg.text), tickCmd())
	case APIResponseMsg:
		m.loading = false
		if m.streamingIdx >= 0 && m.streamingIdx < len(m.output) && m.streamingIdx < len(m.rawOutput) {
			m.output[m.streamingIdx] = m.formatAssistantOutput(msg.text)
			m.rawOutput[m.streamingIdx] = "\nSmartClaw: " + msg.text
			m.scrollToBottom()

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
		m.scrollToBottom()
	case ErrorMsg:
		smartErr := ClassifyError(fmt.Errorf("%s", msg.err))
		m.lastError = smartErr
		m.output = append(m.output, m.formatSmartError(smartErr))
		m.rawOutput = append(m.rawOutput, smartErr.Message)
		m.scrollToBottom()
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

func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), tea.EnableMouseCellMotion)
}

func AddOutput(m *Model, text string) {
	m.output = append(m.output, text)
	cleanText := RemoveANSIColors(text)
	m.rawOutput = append(m.rawOutput, cleanText)
}

func AddError(m *Model, err string) {
	m.output = append(m.output, m.formatError(err))
	m.rawOutput = append(m.rawOutput, "Error: "+err)
}

func (m *Model) scrollToBottom() {
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
	m.viewportOffset = max(0, totalLines-estimatedHeight)
}

func ClearOutput(m *Model) {
	m.output = make([]string, 0)
	m.rawOutput = make([]string, 0)
	m.streamingIdx = -1
	m.streamState.mu.Lock()
	m.streamState.text.Reset()
	m.streamState.thinking.Reset()
	m.streamState.mu.Unlock()
	m.viewportOffset = 0
	m.tokens = 0
	m.cost = 0
	m.loading = false
	m.messages = make([]api.Message, 0)
}
