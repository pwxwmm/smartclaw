package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/plans"
	"github.com/instructkr/smartclaw/internal/services"
	"github.com/instructkr/smartclaw/internal/tools"
)

type slashCommandHandler func(m *Model, args []string) tea.Cmd

var slashCommands = map[string]slashCommandHandler{}

var slashCommandAliases = map[string]string{}

func registerSlashCommand(name string, handler slashCommandHandler, aliases ...string) {
	slashCommands[name] = handler
	for _, alias := range aliases {
		slashCommandAliases[alias] = name
	}
}

func initColorProfile() {
	// 自动检测终端颜色能力，如果检测失败则强制启用 TrueColor
	profile := termenv.ColorProfile()

	// 如果检测到的是 Ascii（无颜色），尝试设置环境变量后重新检测
	if profile == termenv.Ascii {
		// 尝试设置最小化的环境变量
		if os.Getenv("TERM") == "" {
			os.Setenv("TERM", "xterm-256color")
		}
		if os.Getenv("COLORTERM") == "" {
			os.Setenv("COLORTERM", "truecolor")
		}

		// 重新检测
		profile = termenv.ColorProfile()
	}

	// 如果仍然是 Ascii，强制使用 TrueColor（假设现代终端都支持）
	if profile == termenv.Ascii {
		profile = termenv.TrueColor
	}

	// 设置 lipgloss 的颜色 profile
	lipgloss.SetColorProfile(profile)
}

func StartTUI() error {
	initColorProfile()

	p := tea.NewProgram(
		InitialModel(),
		tea.WithAltScreen(),
	)

	m, err := p.Run()
	if err != nil {
		return err
	}

	if model, ok := m.(Model); ok {
		_ = model
	}

	return nil
}

func StartTUIWithClient(client *api.Client) error {
	initColorProfile()

	p := tea.NewProgram(
		InitialModelWithLocalClient(client),
		tea.WithAltScreen(),
	)

	m, err := p.Run()
	if err != nil {
		return err
	}

	if model, ok := m.(Model); ok {
		_ = model
	}

	return nil
}

func StartTUIWithClientAndLearningLoop(client *api.Client, loop *learning.LearningLoop) error {
	initColorProfile()

	m := InitialModelWithLocalClient(client)
	m.learningLoop = loop

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)

	result, err := p.Run()
	if err != nil {
		return err
	}

	if model, ok := result.(Model); ok {
		_ = model
	}

	return nil
}

func StartTUIWithRemoteClient(baseURL, token, model string) error {
	initColorProfile()

	remoteClient := NewRemoteClient(baseURL, token)
	remoteClient.SetModel(model)

	p := tea.NewProgram(
		InitialModelWithClient(remoteClient),
		tea.WithAltScreen(),
	)

	m, err := p.Run()
	if err != nil {
		return err
	}

	if model, ok := m.(Model); ok {
		_ = model
	}

	return nil
}

func cmdHelp(m *Model, args []string) tea.Cmd {
	m.showHelp = !m.showHelp
	AddOutput(m, m.formatAssistantOutput("Help toggled. Press Ctrl+H to toggle again."))
	return nil
}

func cmdStatus(m *Model, args []string) tea.Cmd {
	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
		"Model: %s\nTokens: %d\nCost: $%.4f\nMode: %s\nTheme: %s",
		m.model, m.tokens, m.cost, m.mode, m.theme.Name)))
	return nil
}

func cmdClear(m *Model, args []string) tea.Cmd {
	ClearOutput(m)
	return nil
}

func cmdQuit(m *Model, args []string) tea.Cmd {
	return tea.Quit
}

func cmdModel(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		currentModel := m.modelManager.GetCurrentModel()
		if currentModel != nil {
			AddOutput(m, m.formatAssistantOutput(m.modelManager.FormatModelInfo(currentModel)))
		} else {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Current model: %s", m.model)))
		}
		return nil
	}

	switch args[0] {
	case "list", "ls":
		AddOutput(m, m.formatAssistantOutput(m.modelManager.FormatModelList()))

	case "switch", "use":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /model switch <model-id>"))
			return nil
		}
		modelID := args[1]
		if err := m.modelManager.SetCurrentModel(modelID); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to switch model: %v", err)))
			return nil
		}
		m.model = modelID
		model := m.modelManager.GetCurrentModel()
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Switched to %s\n%s", model.Name, model.Description)))

	case "info":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /model info <model-id>"))
			return nil
		}
		model := m.modelManager.GetModel(args[1])
		if model == nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Model %s not found", args[1])))
			return nil
		}
		AddOutput(m, m.formatAssistantOutput(m.modelManager.FormatModelInfo(model)))

	case "compare":
		if len(args) < 2 {
			AddOutput(m, m.formatAssistantOutput(
				"Model Comparison:\n"+
					"  /model compare <id1> <id2>  - Compare two models\n\n"+
					"Example:\n"+
					"  /model compare claude-sonnet-4-5 gpt-4o",
			))
			return nil
		}
		modelIDs := args[1:]
		AddOutput(m, m.formatAssistantOutput(m.modelManager.CompareModels(modelIDs)))

	default:
		AddOutput(m, m.formatAssistantOutput(
			"Model Management Commands:\n"+
				"  /model              - Show current model details\n"+
				"  /model list         - List all available models\n"+
				"  /model switch <id>  - Switch to a different model\n"+
				"  /model info <id>    - Show detailed model info\n"+
				"  /model compare <ids> - Compare multiple models\n\n"+
				"Examples:\n"+
				"  /model switch claude-sonnet-4-5\n"+
				"  /model compare claude-opus-4-6 gpt-4o",
		))
	}
	return nil
}

func cmdCost(m *Model, args []string) tea.Cmd {
	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Tokens used: %d\nEstimated cost: $%.4f", m.tokens, m.cost)))
	return nil
}

func cmdVoice(m *Model, args []string) tea.Cmd {
	ProcessVoiceCommand(m, args)
	return nil
}

func cmdSession(m *Model, args []string) tea.Cmd {
	if m.sessionManager == nil {
		AddOutput(m, m.formatError("Session manager not initialized. Please restart SmartClaw."))
		return nil
	}

	if m.currentSession == nil {
		AddOutput(m, m.formatError("No active session. Creating new session..."))
		m.currentSession = m.sessionManager.NewSession(m.model, "default")
		return nil
	}

	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Session Management Commands:\n"+
				"  /session new          - Create new session\n"+
				"  /session list         - List all sessions\n"+
				"  /session load <id>    - Load a session\n"+
				"  /session save         - Save current session\n"+
				"  /session delete <id>  - Delete a session\n"+
				"  /session export <id>  - Export session (markdown/json)\n"+
				"  /session record start - Start recording session\n"+
				"  /session record stop  - Stop recording session\n"+
				"  /session record status- Show recording status\n"+
				"  /session replay <file>- Replay a recording\n\n"+
				"Current session: %s\n"+
				"Messages: %d",
			m.currentSession.ID, len(m.currentSession.Messages))))
		return nil
	}

	switch args[0] {
	case "new":
		m.currentSession = m.sessionManager.NewSession(m.model, "default")
		ClearOutput(m)
		m.tokens = 0
		m.cost = 0
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("New session created: %s", m.currentSession.ID)))

	case "list", "ls":
		sessions, err := m.sessionManager.List()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to list sessions: %v", err)))
			return nil
		}

		if len(sessions) == 0 {
			AddOutput(m, m.formatAssistantOutput("No sessions found."))
			return nil
		}

		var output strings.Builder
		output.WriteString("Saved Sessions:\n\n")
		for i, s := range sessions {
			if i >= 10 {
				break
			}
			title := s.Title
			if title == "" {
				title = "Untitled"
			}
			if len(title) > 40 {
				title = title[:40] + "..."
			}
			output.WriteString(fmt.Sprintf("  %s  %s  [%d msgs]  %s\n",
				s.ID[:16],
				s.UpdatedAt.Format("2006-01-02 15:04"),
				s.MessageCount,
				title))
		}
		if len(sessions) > 10 {
			output.WriteString(fmt.Sprintf("\n  ... and %d more sessions", len(sessions)-10))
		}
		AddOutput(m, m.formatAssistantOutput(output.String()))
		return nil

	case "load":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /session load <session-id>"))
			return nil
		}

		sessionID := args[1]
		sess, err := m.sessionManager.Load(sessionID)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to load session: %v", err)))
			return nil
		}

		m.currentSession = sess
		ClearOutput(m)
		m.tokens = sess.Tokens
		m.cost = sess.Cost
		m.model = sess.Model

		for _, msg := range sess.Messages {
			if msg.Role == "user" {
				m.output = append(m.output, m.formatUserInput(msg.Content))
				m.rawOutput = append(m.rawOutput, "\nYou: "+msg.Content)
			} else {
				m.output = append(m.output, m.formatAssistantOutput(msg.Content))
				m.rawOutput = append(m.rawOutput, "\nSmartClaw: "+msg.Content)
			}
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Session loaded: %s\nMessages: %d\nTokens: %d\nCost: $%.4f",
			sess.ID, len(sess.Messages), sess.Tokens, sess.Cost)))

	case "save":
		if m.currentSession == nil {
			AddOutput(m, m.formatError("No active session to save"))
			return nil
		}

		if err := m.sessionManager.Save(m.currentSession); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to save session: %v", err)))
			return nil
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Session saved: %s\nLocation: %s/%s.json",
			m.currentSession.ID,
			m.sessionManager.GetSessionsDir(),
			m.currentSession.ID)))

	case "delete", "rm":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /session delete <session-id>"))
			return nil
		}

		sessionID := args[1]
		if err := m.sessionManager.Delete(sessionID); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to delete session: %v", err)))
			return nil
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Session deleted: %s", sessionID)))

	case "export":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /session export <session-id> [markdown|json]"))
			return nil
		}

		sessionID := args[1]
		format := "markdown"
		if len(args) >= 3 {
			format = args[2]
		}

		content, err := m.sessionManager.Export(sessionID, format)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to export session: %v", err)))
			return nil
		}

		homeDir, _ := os.UserHomeDir()
		exportPath := filepath.Join(homeDir, ".smartclaw", "exports")
		os.MkdirAll(exportPath, 0755)

		ext := "md"
		if format == "json" {
			ext = "json"
		}
		filename := fmt.Sprintf("%s_export.%s", sessionID, ext)
		fullPath := filepath.Join(exportPath, filename)

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to write export file: %v", err)))
			return nil
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Session exported to: %s\nFormat: %s",
			fullPath, format)))

	case "record":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /session record <start|stop|status>"))
			return nil
		}

		switch args[1] {
		case "start":
			if m.sessionRecorder != nil && m.sessionRecorder.IsRecording() {
				AddOutput(m, m.formatError("Already recording. Use /session record stop first."))
				return nil
			}
			recorder, err := services.NewSessionRecorder("")
			if err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Failed to create recorder: %v", err)))
				return nil
			}
			if err := recorder.Start(); err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Failed to start recording: %v", err)))
				return nil
			}
			m.sessionRecorder = recorder
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Recording started: %s", recorder.GetPath())))

		case "stop":
			if m.sessionRecorder == nil || !m.sessionRecorder.IsRecording() {
				AddOutput(m, m.formatError("Not currently recording."))
				return nil
			}
			path := m.sessionRecorder.GetPath()
			if err := m.sessionRecorder.Stop(); err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Failed to stop recording: %v", err)))
				return nil
			}
			m.sessionRecorder = nil
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Recording stopped. Saved to: %s", path)))

		case "status":
			if m.sessionRecorder != nil && m.sessionRecorder.IsRecording() {
				AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
					"Recording: active\nFile: %s\nEntries: %d",
					m.sessionRecorder.GetPath(),
					len(m.sessionRecorder.GetEntries()))))
			} else {
				AddOutput(m, m.formatAssistantOutput("Recording: inactive"))
			}

		default:
			AddOutput(m, m.formatError("Usage: /session record <start|stop|status>"))
		}

	case "replay":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /session replay <recording-file>"))
			return nil
		}
		playback, err := services.NewPlayback(args[1])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to open recording: %v", err)))
			return nil
		}
		defer playback.Close()

		if err := playback.Load(); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to load recording: %v", err)))
			return nil
		}

		entries := playback.GetAll()
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Replay: %s (%d entries)\n\n", args[1], len(entries)))
		for _, entry := range entries {
			role, _ := entry.Data["role"].(string)
			content, _ := entry.Data["content"].(string)
			if role != "" && content != "" {
				sb.WriteString(fmt.Sprintf("[%s] %s\n", role, content))
			} else {
				sb.WriteString(fmt.Sprintf("[%s] %v\n", entry.Type, entry.Data))
			}
		}
		AddOutput(m, m.formatAssistantOutput(sb.String()))

	default:
		AddOutput(m, m.formatError(fmt.Sprintf("Unknown session command: %s", args[0])))
	}
	return nil
}

func cmdAgent(m *Model, args []string) tea.Cmd {
	ProcessAgentCommand(m, args)
	return nil
}

func cmdTemplate(m *Model, args []string) tea.Cmd {
	ProcessTemplateCommand(m, args)
	return nil
}

func cmdSubagent(m *Model, args []string) tea.Cmd {
	ProcessSubagentCommand(m, args)
	return nil
}

func cmdMemory(m *Model, args []string) tea.Cmd {
	ProcessMemoryCommand(m, args)
	return nil
}

func cmdCompact(m *Model, args []string) tea.Cmd {
	ProcessCompactCommand(m, args)
	return nil
}

func cmdGit(m *Model, args []string) tea.Cmd {
	ProcessGitCommand(m, args)
	return nil
}

func cmdLSP(m *Model, args []string) tea.Cmd {
	ProcessLSPCommand(m, args)
	return nil
}

func cmdTeam(m *Model, args []string) tea.Cmd {
	ProcessTeamCommand(m, args)
	return nil
}

func cmdProvider(m *Model, args []string) tea.Cmd {
	ProcessProviderCommand(m, args)
	return nil
}

func cmdMCP(m *Model, args []string) tea.Cmd {
	return ProcessMCPCommand(m, args)
}

func cmdMCPStart(m *Model, args []string) tea.Cmd {
	return ProcessMCPStartCommand(m, args)
}

func cmdMCPStop(m *Model, args []string) tea.Cmd {
	ProcessMCPStopCommand(m, args)
	return nil
}

func cmdTest(m *Model, args []string) tea.Cmd {
	AddOutput(m, m.formatAssistantOutput("✓ Command processing is working! Session ID: "+m.currentSession.ID))
	return nil
}

func cmdEdit(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Opening editor for new content..."))

		content, err := m.editorManager.EditMultiline()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to open editor: %v", err)))
			return nil
		}

		if content == "" {
			AddOutput(m, m.formatAssistantOutput("No content provided"))
			return nil
		}

		m.textArea.SetValue(content)
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Loaded %d bytes from editor", len(content))))
	} else {
		filePath := args[0]
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Opening %s in editor...", filePath)))

		content, err := m.editorManager.EditFile(filePath)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to edit file: %v", err)))
			return nil
		}

		m.textArea.SetValue(content)
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Loaded %s (%d bytes)", filePath, len(content))))
	}

	return nil
}

func cmdEditor(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput(m.editorManager.FormatEditorInfo(m.editorManager.GetEditor())))
		return nil
	}

	switch args[0] {
	case "list", "ls":
		AddOutput(m, m.formatAssistantOutput(m.editorManager.ListAvailableEditors()))

	case "vim":
		m.editorManager.SetEditor(EditorVim)
		AddOutput(m, m.formatAssistantOutput("✓ Editor set to: vim"))

	case "nvim", "neovim":
		m.editorManager.SetEditor(EditorNeovim)
		AddOutput(m, m.formatAssistantOutput("✓ Editor set to: neovim"))

	case "nano":
		m.editorManager.SetEditor(EditorNano)
		AddOutput(m, m.formatAssistantOutput("✓ Editor set to: nano"))

	case "code", "vscode":
		m.editorManager.SetEditor(EditorCode)
		AddOutput(m, m.formatAssistantOutput("✓ Editor set to: VS Code"))

	case "emacs":
		m.editorManager.SetEditor(EditorEmacs)
		AddOutput(m, m.formatAssistantOutput("✓ Editor set to: emacs"))

	case "subl", "sublime":
		m.editorManager.SetEditor(EditorSublime)
		AddOutput(m, m.formatAssistantOutput("✓ Editor set to: Sublime Text"))

	case "atom":
		m.editorManager.SetEditor(EditorAtom)
		AddOutput(m, m.formatAssistantOutput("✓ Editor set to: Atom"))

	default:
		AddOutput(m, m.formatError(fmt.Sprintf("Unknown editor: %s", args[0])))
	}

	return nil
}

func cmdMultilines(m *Model, args []string) tea.Cmd {
	AddOutput(m, m.formatAssistantOutput("Opening editor for multiline input..."))

	currentContent := m.textArea.Value()
	content, err := m.editorManager.EditContent(currentContent, ".txt")
	if err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Failed to open editor: %v", err)))
		return nil
	}

	m.textArea.SetValue(content)
	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Updated input (%d bytes)", len(content))))

	return nil
}

func cmdRetry(m *Model, args []string) tea.Cmd {
	if m.lastError == nil {
		AddOutput(m, m.formatError("No previous error to retry"))
		return nil
	}

	if !m.lastError.Retryable {
		AddOutput(m, m.formatError("This error is not retryable: "+m.lastError.Message))
		return nil
	}

	if m.lastInput == "" {
		AddOutput(m, m.formatError("No previous input to retry"))
		return nil
	}

	AddOutput(m, m.formatAssistantOutput("Retrying last request..."))
	return m.processInput(m.lastInput)
}

func cmdTheme(m *Model, args []string) tea.Cmd {
	if len(args) > 0 {
		themeName := args[0]
		if SetTheme(themeName) {
			m.theme = GetTheme()
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Theme changed to: %s", themeName)))
		} else {
			AddOutput(m, m.formatError(fmt.Sprintf("Unknown theme: %s. Available: %s", themeName, strings.Join(ListThemes(), ", "))))
		}
	} else {
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Current theme: %s\nAvailable themes: %s", m.theme.Name, strings.Join(ListThemes(), ", "))))
	}
	return nil
}

func cmdMode(m *Model, args []string) tea.Cmd {
	if len(args) > 0 {
		m.mode = args[0]
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Permission mode changed to: %s", args[0])))
	} else {
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Current mode: %s\nAvailable modes: ask, read-only, workspace-write, danger-full-access", m.mode)))
	}
	return nil
}

func cmdContext(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput(m.contextManager.RenderStats()))
		return nil
	}

	switch args[0] {
	case "list", "ls":
		AddOutput(m, m.formatAssistantOutput(m.contextManager.RenderMessageList()))

	case "remove", "rm":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /context remove <message-id>"))
			return nil
		}
		msgID := args[1]
		if m.contextManager.RemoveMessage(msgID) {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Message %s removed from context", msgID)))
		} else {
			AddOutput(m, m.formatError(fmt.Sprintf("Message %s not found", msgID)))
		}

	case "keep":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /context keep <message-id>"))
			return nil
		}
		msgID := args[1]
		if m.contextManager.KeepMessage(msgID) {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Message %s marked as important ★", msgID)))
		} else {
			AddOutput(m, m.formatError(fmt.Sprintf("Message %s not found", msgID)))
		}

	case "unkeep":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /context unkeep <message-id>"))
			return nil
		}
		msgID := args[1]
		if m.contextManager.UnkeepMessage(msgID) {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Message %s unmarked", msgID)))
		} else {
			AddOutput(m, m.formatError(fmt.Sprintf("Message %s not found", msgID)))
		}

	case "compress":
		keepCount := 5
		if len(args) >= 2 {
			fmt.Sscanf(args[1], "%d", &keepCount)
		}
		removed := m.contextManager.CompressOldMessages(keepCount)
		if removed > 0 {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Compressed %d old messages (kept %d recent messages)", removed, keepCount)))
		} else {
			AddOutput(m, m.formatAssistantOutput("No messages to compress"))
		}

	case "clear":
		if len(args) >= 2 && args[1] == "--all" {
			m.contextManager.Clear()
			AddOutput(m, m.formatAssistantOutput("All messages cleared from context"))
		} else {
			m.contextManager.ClearNonKept()
			AddOutput(m, m.formatAssistantOutput("Non-kept messages cleared from context"))
		}

	case "stats":
		AddOutput(m, m.formatAssistantOutput(m.contextManager.RenderStats()))

	default:
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Context Management Commands:\n"+
				"  /context          - Show context statistics\n"+
				"  /context list     - List all messages\n"+
				"  /context remove <id>  - Remove message\n"+
				"  /context keep <id>    - Mark as important\n"+
				"  /context unkeep <id>  - Unmark message\n"+
				"  /context compress [n] - Compress old messages (keep n recent)\n"+
				"  /context clear [--all] - Clear context (all or non-kept)",
		)))
	}
	return nil
}

func cmdTabs(m *Model, args []string) tea.Cmd {
	if len(args) > 0 {
		switch args[0] {
		case "next":
			m.tabs.Next()
		case "prev":
			m.tabs.Prev()
		default:
			AddOutput(m, m.formatAssistantOutput("Usage: /tabs [next|prev]"))
		}
	} else {
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Current tab: %s", m.tabs.GetLabel())))
	}
	return nil
}

func cmdLoading(m *Model, args []string) tea.Cmd {
	m.loading = !m.loading
	if m.loading {
		m.spinner.Start()
		AddOutput(m, m.formatAssistantOutput("Loading indicator started"))
	} else {
		m.spinner.Stop()
		AddOutput(m, m.formatAssistantOutput("Loading indicator stopped"))
	}
	return nil
}

func cmdDialog(m *Model, args []string) tea.Cmd {
	if len(args) > 0 {
		switch args[0] {
		case "info":
			m.dialog = NewDialog("Info", "This is an info dialog", DialogInfo)
			m.showDialog = true
		case "confirm":
			m.dialog = NewConfirmDialog("Confirm", "Are you sure?", func(b bool) {
				if b {
					AddOutput(m, m.formatAssistantOutput("Confirmed!"))
				} else {
					AddOutput(m, m.formatAssistantOutput("Cancelled."))
				}
			})
			m.showDialog = true
		case "select":
			items := []string{"Option 1", "Option 2", "Option 3"}
			m.dialog = NewSelectDialog("Select", "Choose an option:", items, func(i int) {
				AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Selected: %s", items[i])))
			})
			m.showDialog = true
		default:
			AddOutput(m, m.formatAssistantOutput("Usage: /dialog [info|confirm|select]"))
		}
	} else {
		AddOutput(m, m.formatAssistantOutput("Usage: /dialog [info|confirm|select]"))
	}
	return nil
}

func cmdAutonomous(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /autonomous <task_description>\nOptions: --max-steps N, --no-verify, --create-pr"))
		return nil
	}
	maxSteps := 20
	verify := true
	createPR := false

	filteredArgs := args[:0]
	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--max-steps="):
			if n, err := strconv.Atoi(strings.TrimPrefix(arg, "--max-steps=")); err == nil {
				maxSteps = n
			}
		case arg == "--no-verify":
			verify = false
		case arg == "--create-pr":
			createPR = true
		default:
			filteredArgs = append(filteredArgs, arg)
		}
	}
	taskDesc := strings.Join(filteredArgs, " ")
	_ = maxSteps
	_ = verify
	_ = createPR

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Autonomous task started: %s\nNote: Full autonomous execution requires the runtime engine. Use the autonomous_execute tool for integrated execution.", taskDesc)))
	return nil
}

func cmdPlaybook(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /playbook <list|execute|create> [name] [params...]\n  /playbook list - List available playbooks\n  /playbook execute <name> [key=value...] - Execute a playbook\n  /playbook create <name> - Create a new playbook"))
		return nil
	}
	subCmd := args[0]
	switch subCmd {
	case "list":
		AddOutput(m, m.formatAssistantOutput("Playbooks: Use the playbook_list tool to see available playbooks."))
	case "execute":
		if len(args) < 2 {
			AddOutput(m, m.formatAssistantOutput("Usage: /playbook execute <name> [key=value...]"))
			return nil
		}
		name := args[1]
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Executing playbook: %s\nNote: Use the playbook_execute tool for integrated execution.", name)))
	case "create":
		if len(args) < 2 {
			AddOutput(m, m.formatAssistantOutput("Usage: /playbook create <name>"))
			return nil
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Creating playbook: %s\nNote: Use the playbook_create tool with YAML content.", args[1])))
	default:
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Unknown playbook subcommand: %s. Use list, execute, or create.", subCmd)))
	}
	return nil
}

func cmdPlanPersistent(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		ps := m.GetPlanStore()
		if ps == nil {
			AddOutput(m, m.formatError("Plan store not available"))
			return nil
		}
		planList, err := ps.List()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error listing plans: %v", err)))
			return nil
		}
		if len(planList) == 0 {
			AddOutput(m, m.formatAssistantOutput("No plans found. Use /plan create <title> to create one."))
			return nil
		}
		var lines []string
		for _, p := range planList {
			lines = append(lines, fmt.Sprintf("  %s [%s] %s - %s", p.ID[:16], p.Status, p.Title, p.UpdatedAt.Format("2006-01-02 15:04")))
		}
		AddOutput(m, m.formatAssistantOutput("Plans:\n"+strings.Join(lines, "\n")))
		return nil
	}

	subCmd := args[0]
	ps := m.GetPlanStore()
	if ps == nil {
		AddOutput(m, m.formatError("Plan store not available"))
		return nil
	}

	switch subCmd {
	case "list":
		planList, err := ps.List()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error listing plans: %v", err)))
			return nil
		}
		if len(planList) == 0 {
			AddOutput(m, m.formatAssistantOutput("No plans found."))
			return nil
		}
		var lines []string
		for _, p := range planList {
			lines = append(lines, fmt.Sprintf("  %s [%s] %s - %s", p.ID[:16], p.Status, p.Title, p.UpdatedAt.Format("2006-01-02 15:04")))
		}
		AddOutput(m, m.formatAssistantOutput("Plans:\n"+strings.Join(lines, "\n")))

	case "create":
		if len(args) < 2 {
			AddOutput(m, m.formatAssistantOutput("Usage: /plan create <title>"))
			return nil
		}
		title := strings.Join(args[1:], " ")
		cwd := m.workDir
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		plan, err := ps.Create(title, "", cwd)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error creating plan: %v", err)))
			return nil
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Created plan: %s (status: %s)", plan.ID, plan.Status)))

	case "show", "get":
		if len(args) < 2 {
			AddOutput(m, m.formatAssistantOutput("Usage: /plan show <id>"))
			return nil
		}
		plan, err := ps.Get(args[1])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return nil
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Plan: %s\nTitle: %s\nStatus: %s\nCreated: %s\nUpdated: %s\n\n%s",
			plan.ID, plan.Title, plan.Status,
			plan.CreatedAt.Format("2006-01-02 15:04"),
			plan.UpdatedAt.Format("2006-01-02 15:04"),
			plan.Content)))

	case "status":
		if len(args) < 3 {
			AddOutput(m, m.formatAssistantOutput("Usage: /plan status <id> <draft|active|completed|abandoned>"))
			return nil
		}
		validStatuses := map[string]bool{plans.StatusDraft: true, plans.StatusActive: true, plans.StatusCompleted: true, plans.StatusAbandoned: true}
		if !validStatuses[args[2]] {
			AddOutput(m, m.formatError(fmt.Sprintf("Invalid status: %s. Valid: draft, active, completed, abandoned", args[2])))
			return nil
		}
		if err := ps.SetStatus(args[1], args[2]); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return nil
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Plan %s status set to %s", args[1], args[2])))

	case "delete":
		if len(args) < 2 {
			AddOutput(m, m.formatAssistantOutput("Usage: /plan delete <id>"))
			return nil
		}
		if err := ps.Delete(args[1]); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return nil
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Deleted plan: %s", args[1])))

	case "search":
		if len(args) < 2 {
			AddOutput(m, m.formatAssistantOutput("Usage: /plan search <query>"))
			return nil
		}
		results, err := ps.Search(strings.Join(args[1:], " "))
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return nil
		}
		if len(results) == 0 {
			AddOutput(m, m.formatAssistantOutput("No matching plans found."))
			return nil
		}
		var lines []string
		for _, p := range results {
			lines = append(lines, fmt.Sprintf("  %s [%s] %s", p.ID[:16], p.Status, p.Title))
		}
		AddOutput(m, m.formatAssistantOutput("Search results:\n"+strings.Join(lines, "\n")))

	default:
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Unknown plan subcommand: %s\nUse: list, create, show, status, delete, search", subCmd)))
	}
	return nil
}

func init() {
	registerSlashCommand("help", cmdHelp, "h")
	registerSlashCommand("status", cmdStatus)
	registerSlashCommand("clear", cmdClear)
	registerSlashCommand("exit", cmdQuit, "quit", "q")
	registerSlashCommand("model", cmdModel, "m")
	registerSlashCommand("cost", cmdCost)
	registerSlashCommand("voice", cmdVoice)
	registerSlashCommand("session", cmdSession, "s")
	registerSlashCommand("agent", cmdAgent)
	registerSlashCommand("template", cmdTemplate)
	registerSlashCommand("subagent", cmdSubagent)
	registerSlashCommand("memory", cmdMemory)
	registerSlashCommand("compact", cmdCompact)
	registerSlashCommand("git", cmdGit)
	registerSlashCommand("lsp", cmdLSP)
	registerSlashCommand("team", cmdTeam)
	registerSlashCommand("provider", cmdProvider)
	registerSlashCommand("mcp", cmdMCP)
	registerSlashCommand("mcp-start", cmdMCPStart)
	registerSlashCommand("mcp-stop", cmdMCPStop)
	registerSlashCommand("test", cmdTest)
	registerSlashCommand("edit", cmdEdit)
	registerSlashCommand("editor", cmdEditor)
	registerSlashCommand("multilines", cmdMultilines, "multiline")
	registerSlashCommand("retry", cmdRetry)
	registerSlashCommand("theme", cmdTheme)
	registerSlashCommand("mode", cmdMode)
	registerSlashCommand("context", cmdContext)
	registerSlashCommand("tabs", cmdTabs)
	registerSlashCommand("loading", cmdLoading)
	registerSlashCommand("dialog", cmdDialog)
	registerSlashCommand("autonomous", cmdAutonomous, "auto")
	registerSlashCommand("playbook", cmdPlaybook, "pb")
	registerSlashCommand("plan", cmdPlanPersistent)
}

func ProcessSlashCommand(cmd string, m *Model) tea.Cmd {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	command := parts[0]
	args := parts[1:]

	cmdName := strings.TrimPrefix(strings.ToLower(command), "/")
	if resolved, ok := slashCommandAliases[cmdName]; ok {
		cmdName = resolved
	}

	if handler, ok := slashCommands[cmdName]; ok {
		return handler(m, args)
	}

	AddOutput(m, m.formatError(fmt.Sprintf("Unknown command: %s. Type /help for available commands.", command)))
	return nil
}

func ProcessAgentCommand(m *Model, args []string) {
	if len(args) == 0 {
		ProcessAgentList(m)
		return
	}

	switch args[0] {
	case "list", "ls":
		ProcessAgentList(m)
	case "switch", "use":
		ProcessAgentSwitch(m, args[1:])
	case "create":
		ProcessAgentCreate(m, args[1:])
	case "delete", "rm":
		ProcessAgentDelete(m, args[1:])
	case "info":
		ProcessAgentInfo(m, args[1:])
	case "export":
		ProcessAgentExport(m, args[1:])
	case "import":
		ProcessAgentImport(m, args[1:])
	default:
		AddOutput(m, m.formatAssistantOutput(
			"Agent Management Commands:\n"+
				"  /agent                 - List all agents\n"+
				"  /agent switch <name>   - Switch to an agent\n"+
				"  /agent info <name>     - Show agent details\n"+
				"  /agent create <name> <desc> <prompt> - Create custom agent\n"+
				"  /agent delete <name>   - Delete custom agent\n"+
				"  /agent export <name> [json|md] - Export agent\n"+
				"  /agent import <file>   - Import agent from file"))
	}
}

func ProcessAgentList(m *Model) {
	if m.agentManager == nil {
		AddOutput(m, m.formatError("Agent manager not initialized"))
		return
	}
	AddOutput(m, m.formatAssistantOutput(m.agentManager.FormatAgentList()))
}

func ProcessAgentSwitch(m *Model, args []string) {
	if m.agentManager == nil {
		AddOutput(m, m.formatError("Agent manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatError("Usage: /agent switch <agent-name>"))
		AddOutput(m, m.formatAssistantOutput(m.agentManager.FormatAgentList()))
		return
	}
	agentName := args[0]
	if err := m.agentManager.SetCurrentAgent(agentName); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}
	agent, _ := m.agentManager.GetAgent(agentName)
	if agent != nil {
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Switched to agent: %s\n%s", agent.AgentType, agent.WhenToUse)))
	} else {
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Switched to agent: %s", agentName)))
	}
}

func ProcessAgentCreate(m *Model, args []string) {
	if m.agentManager == nil {
		AddOutput(m, m.formatError("Agent manager not initialized"))
		return
	}
	if len(args) < 3 {
		AddOutput(m, m.formatAssistantOutput(
			"Usage: /agent create <name> <description> <system-prompt> [options]\n\n"+
				"Options:\n"+
				"  --model <model>           Set model\n"+
				"  --tools <tool1,tool2>     Allowed tools\n"+
				"  --disallow <tool1,tool2>  Disallowed tools\n"+
				"  --permission <mode>       Permission mode (ask/read-only/workspace-write/danger-full-access)\n\n"+
				"Example:\n"+
				"  /agent create myagent \"My custom agent\" \"You are a helpful assistant...\" --model claude-sonnet-4-5"))
		return
	}

	agent := &AgentDefinition{
		AgentType:    args[0],
		WhenToUse:    args[1],
		SystemPrompt: args[2],
	}

	for i := 3; i < len(args); i++ {
		switch args[i] {
		case "--model":
			if i+1 < len(args) {
				agent.Model = args[i+1]
				i++
			}
		case "--tools":
			if i+1 < len(args) {
				agent.Tools = strings.Split(args[i+1], ",")
				i++
			}
		case "--disallow":
			if i+1 < len(args) {
				agent.DisallowedTools = strings.Split(args[i+1], ",")
				i++
			}
		case "--permission":
			if i+1 < len(args) {
				agent.PermissionMode = PermissionMode(args[i+1])
				i++
			}
		}
	}

	if err := m.agentManager.CreateCustomAgent(agent); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}
	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Created custom agent: %s", agent.AgentType)))
}

func ProcessAgentDelete(m *Model, args []string) {
	if m.agentManager == nil {
		AddOutput(m, m.formatError("Agent manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatError("Usage: /agent delete <agent-name>"))
		return
	}
	if err := m.agentManager.DeleteCustomAgent(args[0]); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}
	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Deleted agent: %s", args[0])))
}

func ProcessAgentInfo(m *Model, args []string) {
	if m.agentManager == nil {
		AddOutput(m, m.formatError("Agent manager not initialized"))
		return
	}
	if len(args) == 0 {
		agent := m.agentManager.GetCurrentAgent()
		if agent != nil {
			AddOutput(m, m.formatAssistantOutput(m.agentManager.FormatAgentInfo(agent)))
			return
		}
		AddOutput(m, m.formatAssistantOutput("Usage: /agent info <agent-name>"))
		return
	}
	agent, err := m.agentManager.GetAgent(args[0])
	if err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}
	AddOutput(m, m.formatAssistantOutput(m.agentManager.FormatAgentInfo(agent)))
}

func ProcessAgentExport(m *Model, args []string) {
	if m.agentManager == nil {
		AddOutput(m, m.formatError("Agent manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /agent export <agent-name> [json|md]"))
		return
	}

	agentName := args[0]
	format := "md"
	if len(args) > 1 {
		format = args[1]
	}

	content, err := m.agentManager.ExportAgent(agentName, format)
	if err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}

	homeDir, _ := os.UserHomeDir()
	exportPath := filepath.Join(homeDir, ".smartclaw", "exports")
	os.MkdirAll(exportPath, 0755)

	ext := "md"
	if format == "json" {
		ext = "json"
	}
	filename := fmt.Sprintf("%s_agent.%s", agentName, ext)
	fullPath := filepath.Join(exportPath, filename)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Failed to write export file: %v", err)))
		return
	}

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Exported agent to: %s", fullPath)))
}

func ProcessAgentImport(m *Model, args []string) {
	if m.agentManager == nil {
		AddOutput(m, m.formatError("Agent manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /agent import <file-path>"))
		return
	}

	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Failed to read file: %v", err)))
		return
	}

	format := "md"
	if strings.HasSuffix(filePath, ".json") {
		format = "json"
	}

	if err := m.agentManager.ImportAgent(string(data), format); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Imported agent from: %s", filePath)))
}

func ProcessTemplateCommand(m *Model, args []string) {
	if len(args) == 0 {
		ProcessTemplateList(m)
		return
	}

	switch args[0] {
	case "list", "ls":
		ProcessTemplateList(m)
	case "use":
		ProcessTemplateUse(m, args[1:])
	case "create":
		ProcessTemplateCreate(m, args[1:])
	case "delete", "rm":
		ProcessTemplateDelete(m, args[1:])
	case "info":
		ProcessTemplateInfo(m, args[1:])
	case "export":
		ProcessTemplateExport(m, args[1:])
	case "import":
		ProcessTemplateImport(m, args[1:])
	default:
		AddOutput(m, m.formatAssistantOutput(
			"Template Management Commands:\n"+
				"  /template              - List all templates\n"+
				"  /template use <id>     - Use a template\n"+
				"  /template info <id>    - Show template details\n"+
				"  /template create <id>  - Create custom template\n"+
				"  /template delete <id>  - Delete custom template\n"+
				"  /template export <id>  - Export template\n"+
				"  /template import <file> - Import template"))
	}
}

func ProcessTemplateList(m *Model) {
	if m.templateManager == nil {
		AddOutput(m, m.formatError("Template manager not initialized"))
		return
	}
	AddOutput(m, m.formatAssistantOutput(m.templateManager.FormatTemplateList()))
}

func ProcessTemplateUse(m *Model, args []string) {
	if m.templateManager == nil {
		AddOutput(m, m.formatError("Template manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatError("Usage: /template use <template-id> [var1=value1] [var2=value2] ..."))
		return
	}
	templateID := args[0]
	variables := make(map[string]string)
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			variables[parts[0]] = parts[1]
		}
	}
	content, err := m.templateManager.RenderTemplate(templateID, variables)
	if err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}
	m.textArea.SetValue(content)
	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Template '%s' loaded. Variables replaced:\n%s", templateID, content)))
}

func ProcessTemplateCreate(m *Model, args []string) {
	if m.templateManager == nil {
		AddOutput(m, m.formatError("Template manager not initialized"))
		return
	}
	if len(args) < 3 {
		AddOutput(m, m.formatAssistantOutput(
			"Usage: /template create <id> <name> <description> <content>\n"+
				"Example: /template create mytemplate \"My Template\" \"A custom template\" \"You are a...\""))
		return
	}
	template := &PromptTemplate{
		ID:          args[0],
		Name:        args[1],
		Description: args[2],
		Content:     strings.Join(args[3:], " "),
	}
	if err := m.templateManager.CreateTemplate(template); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}
	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Created template: %s", args[0])))
}

func ProcessTemplateDelete(m *Model, args []string) {
	if m.templateManager == nil {
		AddOutput(m, m.formatError("Template manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatError("Usage: /template delete <template-id>"))
		return
	}
	if err := m.templateManager.DeleteTemplate(args[0]); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}
	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Deleted template: %s", args[0])))
}

func ProcessTemplateInfo(m *Model, args []string) {
	if m.templateManager == nil {
		AddOutput(m, m.formatError("Template manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /template info <template-id>"))
		return
	}
	template, err := m.templateManager.GetTemplate(args[0])
	if err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}
	AddOutput(m, m.formatAssistantOutput(m.templateManager.FormatTemplateInfo(template)))
}

func ProcessTemplateExport(m *Model, args []string) {
	if m.templateManager == nil {
		AddOutput(m, m.formatError("Template manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /template export <template-id> [json|md]"))
		return
	}

	templateID := args[0]
	format := "json"
	if len(args) > 1 {
		format = args[1]
	}

	content, err := m.templateManager.ExportTemplate(templateID, format)
	if err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}

	homeDir, _ := os.UserHomeDir()
	exportPath := filepath.Join(homeDir, ".smartclaw", "exports")
	os.MkdirAll(exportPath, 0755)

	ext := "json"
	if format == "md" || format == "markdown" {
		ext = "md"
	}
	filename := fmt.Sprintf("%s_template.%s", templateID, ext)
	fullPath := filepath.Join(exportPath, filename)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Failed to write export file: %v", err)))
		return
	}

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Exported template to: %s", fullPath)))
}

func ProcessTemplateImport(m *Model, args []string) {
	if m.templateManager == nil {
		AddOutput(m, m.formatError("Template manager not initialized"))
		return
	}
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /template import <file-path>"))
		return
	}

	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Failed to read file: %v", err)))
		return
	}

	format := "json"
	if strings.HasSuffix(filePath, ".md") {
		format = "md"
	}

	if err := m.templateManager.ImportTemplate(string(data), format); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
		return
	}

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Imported template from: %s", filePath)))
}

func ProcessSubagentCommand(m *Model, args []string) {
	if m.subagentManager == nil {
		AddOutput(m, m.formatError("Subagent manager not initialized"))
		return
	}

	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput(m.subagentManager.FormatTaskList()))
		return
	}

	switch args[0] {
	case "list", "ls":
		AddOutput(m, m.formatAssistantOutput(m.subagentManager.FormatTaskList()))

	case "spawn", "start":
		if len(args) < 3 {
			AddOutput(m, m.formatAssistantOutput(
				"Usage: /subagent spawn <agent-type> <prompt> [options]\n\n"+
					"Options:\n"+
					"  --background    - Run in background\n"+
					"  --worktree      - Create isolated worktree\n\n"+
					"Available agent types: explore, librarian, oracle, metis, momus, build, quick\n\n"+
					"Example:\n"+
					"  /subagent spawn explore \"Find authentication patterns\""))
			return
		}

		agentType := args[1]
		prompt := strings.Join(args[2:], " ")
		isBackground := false
		withWorktree := false

		if strings.Contains(prompt, "--background") {
			isBackground = true
			prompt = strings.ReplaceAll(prompt, "--background", "")
		}
		if strings.Contains(prompt, "--worktree") {
			withWorktree = true
			prompt = strings.ReplaceAll(prompt, "--worktree", "")
		}
		prompt = strings.TrimSpace(prompt)

		var opts []SpawnOption
		if isBackground {
			opts = append(opts, WithBackground(true))
		}

		task, err := m.subagentManager.SpawnSubagent(nil, agentType, prompt, opts...)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to spawn subagent: %v", err)))
			return
		}

		if withWorktree {
			worktreePath, err := m.subagentManager.CreateWorktree(task.ID)
			if err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Failed to create worktree: %v", err)))
			} else {
				AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Created worktree: %s", worktreePath)))
			}
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"✓ Spawned subagent: %s\nTask ID: %s\nAgent: %s\nBackground: %v",
			task.ID[:8], task.ID, agentType, isBackground)))

	case "status":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /subagent status <task-id>"))
			return
		}
		task, err := m.subagentManager.GetTask(args[1])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(m.subagentManager.FormatTaskInfo(task)))

	case "cancel":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /subagent cancel <task-id>"))
			return
		}
		if err := m.subagentManager.CancelTask(args[1]); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Cancelled task: %s", args[1])))

	case "export":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /subagent export <task-id>"))
			return
		}
		content, err := m.subagentManager.ExportTask(args[1])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return
		}

		homeDir, _ := os.UserHomeDir()
		exportPath := filepath.Join(homeDir, ".smartclaw", "exports")
		os.MkdirAll(exportPath, 0755)

		filename := fmt.Sprintf("%s_subagent.json", args[1])
		fullPath := filepath.Join(exportPath, filename)

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to write export file: %v", err)))
			return
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Exported task to: %s", fullPath)))

	case "worktree":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /subagent worktree <task-id> [create|remove]"))
			return
		}
		taskID := args[1]
		action := "create"
		if len(args) >= 3 {
			action = args[2]
		}

		if action == "create" {
			path, err := m.subagentManager.CreateWorktree(taskID)
			if err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
				return
			}
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Created worktree: %s", path)))
		} else if action == "remove" {
			if err := m.subagentManager.RemoveWorktree(taskID); err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
				return
			}
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Removed worktree for task: %s", taskID)))
		}

	default:
		AddOutput(m, m.formatAssistantOutput(
			"Subagent Management Commands:\n"+
				"  /subagent                     - List all subagent tasks\n"+
				"  /subagent spawn <type> <prompt> [opts] - Spawn a new subagent\n"+
				"  /subagent status <task-id>    - Show task status\n"+
				"  /subagent cancel <task-id>    - Cancel running task\n"+
				"  /subagent export <task-id>    - Export task to JSON\n"+
				"  /subagent worktree <id> [create|remove] - Manage worktrees"))
	}
}

func ProcessMemoryCommand(m *Model, args []string) {
	if m.memoryStore == nil {
		AddOutput(m, m.formatError("Memory store not initialized"))
		return
	}

	if len(args) == 0 {
		keys := m.memoryStore.List()
		if len(keys) == 0 {
			AddOutput(m, m.formatAssistantOutput("Memory store is empty. Use /memory set to add items."))
			return
		}

		var sb strings.Builder
		sb.WriteString("Memory Store Contents:\n\n")
		for _, key := range keys {
			value, err := m.memoryStore.Get(key)
			if err == nil {
				sb.WriteString(fmt.Sprintf("  • %s: %v\n", key, value))
			}
		}
		AddOutput(m, m.formatAssistantOutput(sb.String()))
		return
	}

	switch args[0] {
	case "set", "save":
		if len(args) < 3 {
			AddOutput(m, m.formatAssistantOutput(
				"Usage: /memory set <key> <value> [ttl]\n\n"+
					"Example:\n"+
					"  /memory set user_name \"John\"\n"+
					"  /memory set temp_data \"value\" 1h"))
			return
		}

		key := args[1]
		value := args[2]
		ttl := time.Duration(0)

		if len(args) >= 4 {
			parsed, err := time.ParseDuration(args[3])
			if err == nil {
				ttl = parsed
			}
		}

		if err := m.memoryStore.Set(key, value, ttl); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return
		}

		ttlStr := ""
		if ttl > 0 {
			ttlStr = fmt.Sprintf(" (expires in %s)", ttl)
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Saved to memory: %s%s", key, ttlStr)))

	case "get":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /memory get <key>"))
			return
		}

		value, err := m.memoryStore.Get(args[1])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("%s: %v", args[1], value)))

	case "delete", "rm", "remove":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /memory delete <key>"))
			return
		}

		if err := m.memoryStore.Delete(args[1]); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Deleted from memory: %s", args[1])))

	case "list", "ls":
		keys := m.memoryStore.List()
		if len(keys) == 0 {
			AddOutput(m, m.formatAssistantOutput("Memory store is empty"))
			return
		}

		var sb strings.Builder
		sb.WriteString("Memory Keys:\n")
		for _, key := range keys {
			sb.WriteString(fmt.Sprintf("  • %s\n", key))
		}
		AddOutput(m, m.formatAssistantOutput(sb.String()))

	case "clear":
		if err := m.memoryStore.Clear(); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput("✓ Memory store cleared"))

	case "search":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /memory search <query>"))
			return
		}

		results := m.memoryStore.Search(args[1])
		if len(results) == 0 {
			AddOutput(m, m.formatAssistantOutput("No results found"))
			return
		}

		var sb strings.Builder
		sb.WriteString("Search Results:\n")
		for _, mem := range results {
			sb.WriteString(fmt.Sprintf("  • %s: %v\n", mem.Key, mem.Value))
		}
		AddOutput(m, m.formatAssistantOutput(sb.String()))

	case "tag":
		if len(args) < 3 {
			AddOutput(m, m.formatAssistantOutput(
				"Usage: /memory tag <key> <tag> [add|remove]\n\n"+
					"Example:\n"+
					"  /memory tag user_name project1 add\n"+
					"  /memory tag user_name project1 remove"))
			return
		}

		key := args[1]
		tag := args[2]
		action := "add"
		if len(args) >= 4 {
			action = args[3]
		}

		if action == "add" {
			if err := m.memoryStore.AddTag(key, tag); err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
				return
			}
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Added tag '%s' to '%s'", tag, key)))
		} else if action == "remove" {
			if err := m.memoryStore.RemoveTag(key, tag); err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Error: %v", err)))
				return
			}
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Removed tag '%s' from '%s'", tag, key)))
		}

	case "bytag":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /memory bytag <tag>"))
			return
		}

		results := m.memoryStore.GetByTag(args[1])
		if len(results) == 0 {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("No items with tag: %s", args[1])))
			return
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Items with tag '%s':\n", args[1]))
		for _, mem := range results {
			sb.WriteString(fmt.Sprintf("  • %s: %v\n", mem.Key, mem.Value))
		}
		AddOutput(m, m.formatAssistantOutput(sb.String()))

	default:
		AddOutput(m, m.formatAssistantOutput(
			"Memory Management Commands:\n"+
				"  /memory                    - List all stored items\n"+
				"  /memory set <key> <value> [ttl] - Store a value\n"+
				"  /memory get <key>          - Retrieve a value\n"+
				"  /memory delete <key>       - Delete a value\n"+
				"  /memory list               - List all keys\n"+
				"  /memory clear              - Clear all memory\n"+
				"  /memory search <query>     - Search memory\n"+
				"  /memory tag <key> <tag> [add|remove] - Manage tags\n"+
				"  /memory bytag <tag>        - Get items by tag"))
	}
}

func ProcessVoiceCommand(m *Model, args []string) {
	if m.voiceManager == nil {
		AddOutput(m, m.formatError("Voice manager not initialized"))
		return
	}

	if len(args) == 0 {
		mode := m.voiceManager.GetMode()
		modeStr := "disabled"
		switch mode {
		case 1:
			modeStr = "push-to-talk"
		case 2:
			modeStr = "always-on"
		}

		config := m.voiceManager.GetConfig()
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Voice Mode Status:\n"+
				"  Mode: %s\n"+
				"  Language: %s\n"+
				"  Model: %s\n"+
				"  Sample Rate: %d Hz\n\n"+
				"Commands:\n"+
				"  /voice on       - Enable always-on mode\n"+
				"  /voice off      - Disable voice mode\n"+
				"  /voice ptt      - Enable push-to-talk mode\n"+
				"  /voice record   - Start recording (push-to-talk)\n"+
				"  /voice stop     - Stop recording and transcribe\n"+
				"  /voice config   - Show current configuration",
			modeStr, config.Language, config.Model, config.SampleRate)))
		return
	}

	switch args[0] {
	case "on", "enable":
		m.voiceManager.SetMode(2)
		AddOutput(m, m.formatAssistantOutput("✓ Voice mode enabled (always-on)"))

	case "off", "disable":
		m.voiceManager.SetMode(0)
		AddOutput(m, m.formatAssistantOutput("✓ Voice mode disabled"))

	case "ptt", "push-to-talk":
		m.voiceManager.SetMode(1)
		AddOutput(m, m.formatAssistantOutput("✓ Voice mode enabled (push-to-talk)\nPress /voice record to start recording"))

	case "record":
		if m.voiceManager.GetMode() != 1 {
			AddOutput(m, m.formatError("Push-to-talk mode required. Use /voice ptt first."))
			return
		}

		AddOutput(m, m.formatAssistantOutput("🎤 Recording... Press /voice stop when done."))
		if err := m.voiceManager.StartPushToTalk(nil); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to start recording: %v", err)))
		}

	case "stop":
		if m.voiceManager.GetMode() != 1 {
			AddOutput(m, m.formatError("No recording in progress"))
			return
		}

		AddOutput(m, m.formatAssistantOutput("⏹ Stopping recording and transcribing..."))
		result, err := m.voiceManager.StopPushToTalk(nil)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Transcription failed: %v", err)))
			return
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"📝 Transcription:\n%s\n\nDuration: %.1fs | Language: %s",
			result.Text, result.Duration, result.Language)))
		m.textArea.SetValue(result.Text)

	case "config":
		config := m.voiceManager.GetConfig()
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Voice Configuration:\n"+
				"  Mode: %d\n"+
				"  Language: %s\n"+
				"  Model: %s\n"+
				"  Sample Rate: %d\n"+
				"  Recording Timeout: %ds\n"+
				"  Silence Threshold: %d\n"+
				"  VAD Enabled: %v\n"+
				"  VAD Threshold: %.2f",
			config.Mode, config.Language, config.Model, config.SampleRate,
			config.RecordingTimeout, config.SilenceThreshold, config.VadEnabled, config.VadThreshold)))

	case "lang", "language":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /voice language <code> (e.g., en, zh, ja)"))
			return
		}
		config := m.voiceManager.GetConfig()
		config.Language = args[1]
		m.voiceManager.UpdateConfig(config)
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Language set to: %s", args[1])))

	case "keyterm":
		if len(args) < 2 {
			keyterms := m.voiceManager.GetKeyterms()
			if len(keyterms) == 0 {
				AddOutput(m, m.formatAssistantOutput("No keyterms set. Use /voice keyterm add <term>"))
			} else {
				AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Keyterms: %s", strings.Join(keyterms, ", "))))
			}
			return
		}

		if args[1] == "add" && len(args) >= 3 {
			m.voiceManager.AddKeyterm(args[2])
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Added keyterm: %s", args[2])))
		} else if args[1] == "clear" {
			m.voiceManager.SetKeyterms([]string{})
			AddOutput(m, m.formatAssistantOutput("✓ Cleared all keyterms"))
		}

	case "transcribe":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /voice transcribe <audio-file>"))
			return
		}

		AddOutput(m, m.formatAssistantOutput("🔄 Transcribing audio file..."))
		result, err := m.voiceManager.TranscribeFile(nil, args[1])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Transcription failed: %v", err)))
			return
		}

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"📝 Transcription:\n%s\n\nDuration: %.1fs | Language: %s",
			result.Text, result.Duration, result.Language)))

	default:
		AddOutput(m, m.formatAssistantOutput(
			"Voice Commands:\n"+
				"  /voice           - Show voice status\n"+
				"  /voice on        - Enable always-on mode\n"+
				"  /voice off       - Disable voice mode\n"+
				"  /voice ptt       - Enable push-to-talk mode\n"+
				"  /voice record    - Start recording (PTT mode)\n"+
				"  /voice stop      - Stop and transcribe\n"+
				"  /voice config    - Show configuration\n"+
				"  /voice lang <c>  - Set language code\n"+
				"  /voice keyterm   - Manage keyterms\n"+
				"  /voice transcribe <file> - Transcribe audio file"))
	}
}

func ProcessCompactCommand(m *Model, args []string) {
	if len(args) == 0 {
		stats := m.contextManager.GetTokenCount()
		maxTokens := m.contextManager.maxTokens
		percentage := float64(stats) / float64(maxTokens) * 100

		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Context Usage:\n"+
				"  Tokens: %d / %d (%.1f%%)\n"+
				"  Messages: %d\n\n"+
				"Commands:\n"+
				"  /compact now       - Compact now\n"+
				"  /compact auto      - Toggle auto-compact\n"+
				"  /compact status    - Show compact stats\n"+
				"  /compact config    - Show configuration",
			stats, maxTokens, percentage, m.contextManager.GetMessageCount())))
		return
	}

	switch args[0] {
	case "now", "manual":
		AddOutput(m, m.formatAssistantOutput("🔄 Compacting conversation history..."))

		messages := m.contextManager.GetMessages()
		if len(messages) < 3 {
			AddOutput(m, m.formatError("Not enough messages to compact"))
			return
		}

		tokensBefore := m.contextManager.GetTokenCount()
		removed := m.contextManager.CompressOldMessages(5)
		tokensAfter := m.contextManager.GetTokenCount()

		if removed > 0 {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
				"✓ Compacted %d messages\n"+
					"Tokens before: %d\n"+
					"Tokens after: %d\n"+
					"Tokens saved: ~%d",
				removed,
				tokensBefore,
				tokensAfter,
				tokensBefore-tokensAfter)))
		} else {
			AddOutput(m, m.formatAssistantOutput("No messages to compact"))
		}

	case "auto":
		m.autoCompactEnabled = !m.autoCompactEnabled
		status := "disabled"
		if m.autoCompactEnabled {
			status = "enabled"
		}
		threshold := m.contextManager.maxTokens * 70 / 100
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Auto-compact %s\n\n"+
				"When enabled, context will be automatically compacted when token usage\n"+
				"exceeds the threshold (~%d tokens, 70%% of %d).\n\n"+
				"Use /compact now to manually compact at any time.",
			status,
			threshold,
			m.contextManager.maxTokens)))

	case "status":
		state := m.compactService.GetState()
		autoLabel := "disabled"
		if m.autoCompactEnabled {
			autoLabel = "enabled"
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Compact Statistics:\n"+
				"  Total messages: %d\n"+
				"  Kept messages: %d\n"+
				"  Current tokens: %d\n"+
				"  Max tokens: %d\n"+
				"  Usage: %.1f%%\n"+
				"  Auto-compact: %s\n"+
				"  Total tokens saved: %d\n"+
				"  Last compact: %s",
			m.contextManager.GetMessageCount(),
			countKeptMessages(m),
			m.contextManager.GetTokenCount(),
			m.contextManager.maxTokens,
			float64(m.contextManager.GetTokenCount())/float64(m.contextManager.maxTokens)*100,
			autoLabel,
			state.TotalTokensSaved,
			state.LastCompactTime.Format("15:04:05"))))

	case "config":
		autoStatus := "disabled"
		if m.autoCompactEnabled {
			autoStatus = "enabled"
		}
		AddOutput(m, m.formatAssistantOutput(
			"Compact Configuration:\n"+
				"  Auto-compact: "+autoStatus+"\n"+
				"  Warning threshold: 70%\n"+
				"  Error threshold: 90%\n"+
				"  Keep recent: 5 messages\n\n"+
				"Configuration can be adjusted in ~/.smartclaw/config.yaml"))

	default:
		AddOutput(m, m.formatAssistantOutput(
			"Compact Commands:\n"+
				"  /compact         - Show context usage\n"+
				"  /compact now     - Compact now\n"+
				"  /compact auto    - Toggle auto-compact\n"+
				"  /compact status  - Show statistics\n"+
				"  /compact config  - Show configuration"))
	}
}

func countKeptMessages(m *Model) int {
	count := 0
	for _, msg := range m.contextManager.GetMessages() {
		if msg.Keep {
			count++
		}
	}
	return count
}

func ProcessGitCommand(m *Model, args []string) {
	if m.gitManager == nil {
		AddOutput(m, m.formatError("Git manager not initialized"))
		return
	}

	if !m.gitManager.IsGitRepo() {
		AddOutput(m, m.formatError("Not a git repository. Navigate to a git repository first."))
		return
	}

	if len(args) == 0 {
		status, err := m.gitManager.GetStatus()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get git status: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(m.gitManager.FormatStatus(status)))
		return
	}

	switch args[0] {
	case "status", "st":
		status, err := m.gitManager.GetStatus()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get status: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(m.gitManager.FormatStatus(status)))

	case "diff":
		cached := false
		if len(args) >= 2 && (args[1] == "cached" || args[1] == "staged") {
			cached = true
		}
		diff, err := m.gitManager.Diff(cached)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get diff: %v", err)))
			return
		}
		if diff == "" {
			AddOutput(m, m.formatAssistantOutput("No changes to show"))
			return
		}
		AddOutput(m, m.formatAssistantOutput(diff))

	case "log":
		count := 10
		if len(args) >= 2 {
			fmt.Sscanf(args[1], "%d", &count)
		}
		commits, err := m.gitManager.GetLog(count)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get log: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(m.gitManager.FormatLog(commits)))

	case "branches", "br":
		branches, err := m.gitManager.GetBranches()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get branches: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(m.gitManager.FormatBranches(branches)))

	case "add":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /git add <file1> [file2] ...  or /git add ."))
			return
		}
		files := args[1:]
		if err := m.gitManager.Add(files); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to add files: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Staged %d file(s): %s", len(files), strings.Join(files, ", "))))

	case "commit":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /git commit <message>"))
			return
		}
		message := strings.Join(args[1:], " ")
		if err := m.gitManager.Commit(message); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to commit: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Committed: %s", message)))

	case "ai-commit":
		status, err := m.gitManager.GetStatus()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get status: %v", err)))
			return
		}
		if len(status.Staged) == 0 && len(status.Unstaged) == 0 {
			AddOutput(m, m.formatError("No changes to commit. Use /git add <files> first."))
			return
		}
		if len(status.Staged) == 0 {
			AddOutput(m, m.formatAssistantOutput("No staged changes. Staging all modified files..."))
			if err := m.gitManager.Add([]string{"."}); err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Failed to stage files: %v", err)))
				return
			}
		}
		diff, err := m.gitManager.Diff(true)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get staged diff: %v", err)))
			return
		}
		files := status.Staged
		if len(files) == 0 {
			files = status.Unstaged
		}
		commitMsg := generateAICommitMessage(diff, files)
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("📝 Generated commit message:\n  %s\n\nCommitting...", commitMsg)))
		if err := m.gitManager.Commit(commitMsg); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to commit: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Committed: %s", commitMsg)))

	case "ai-review":
		diff, err := m.gitManager.Diff(false)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get diff: %v", err)))
			return
		}
		if diff == "" {
			stagedDiff, _ := m.gitManager.Diff(true)
			if stagedDiff == "" {
				AddOutput(m, m.formatAssistantOutput("No changes to review"))
				return
			}
			diff = stagedDiff
		}
		review := generateAIReview(diff)
		AddOutput(m, m.formatAssistantOutput(review))

	case "push":
		status, _ := m.gitManager.GetStatus()
		setUpstream := !status.HasUpstream
		branch := status.Branch
		if len(args) >= 2 {
			branch = args[1]
		}
		if err := m.gitManager.Push(setUpstream, branch); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to push: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Pushed to %s", branch)))

	case "pull":
		if err := m.gitManager.Pull(); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to pull: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput("✓ Pulled latest changes"))

	case "checkout", "co":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /git checkout <branch>"))
			return
		}
		if err := m.gitManager.Checkout(args[1]); err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to checkout: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Switched to branch: %s", args[1])))

	case "branch":
		if len(args) >= 2 && args[1] != "list" && args[1] != "ls" {
			if err := m.gitManager.CreateBranch(args[1]); err != nil {
				AddOutput(m, m.formatError(fmt.Sprintf("Failed to create branch: %v", err)))
				return
			}
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Created and switched to branch: %s", args[1])))
			return
		}
		branches, err := m.gitManager.GetBranches()
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get branches: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(m.gitManager.FormatBranches(branches)))

	default:
		AddOutput(m, m.formatAssistantOutput(
			"Git Commands:\n"+
				"  /git                     - Show git status\n"+
				"  /git status              - Detailed status\n"+
				"  /git diff [cached]       - Show diff\n"+
				"  /git log [count]         - Show commit log\n"+
				"  /git branches            - List branches\n"+
				"  /git add <files>         - Stage files\n"+
				"  /git commit <message>    - Commit with message\n"+
				"  /git ai-commit           - AI-generated commit message\n"+
				"  /git ai-review           - AI review of changes\n"+
				"  /git push [branch]       - Push changes\n"+
				"  /git pull                - Pull latest\n"+
				"  /git checkout <branch>   - Switch branch\n"+
				"  /git branch <name>       - Create new branch"))
	}
}

func generateAICommitMessage(diff string, files []string) string {
	commitType := "chore"
	scope := ""
	subject := "update files"

	if len(files) > 0 {
		scope = extractGitScope(files[0])
	}

	lines := strings.Split(diff, "\n")
	added, removed := 0, 0
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed++
		}
	}

	for _, f := range files {
		lower := strings.ToLower(f)
		switch {
		case strings.Contains(lower, "test") || strings.HasSuffix(lower, "_test.go"):
			commitType = "test"
			subject = fmt.Sprintf("add/update tests for %s", scope)
		case strings.Contains(lower, ".md") || strings.Contains(lower, "doc"):
			commitType = "docs"
			subject = fmt.Sprintf("update documentation for %s", scope)
		case strings.Contains(lower, "fix") || strings.Contains(lower, "bug"):
			commitType = "fix"
			subject = fmt.Sprintf("fix issue in %s", scope)
		case strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".py"):
			commitType = "feat"
			subject = fmt.Sprintf("update %s", scope)
		}
	}

	if added > 0 && removed == 0 {
		subject = fmt.Sprintf("add %s", scope)
	} else if removed > 0 && added == 0 {
		subject = fmt.Sprintf("remove code from %s", scope)
	}

	if scope != "" {
		return fmt.Sprintf("%s(%s): %s", commitType, scope, subject)
	}
	return fmt.Sprintf("%s: %s", commitType, subject)
}

func generateAIReview(diff string) string {
	var findings []string
	lines := strings.Split(diff, "\n")

	for i, line := range lines {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(line, "+"), " ")
		lower := strings.ToLower(trimmed)

		switch {
		case strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "api_key"):
			findings = append(findings, fmt.Sprintf("🔴 [SECURITY] Line %d: Potential secret or credential exposed", i+1))
		case strings.Contains(lower, "todo") || strings.Contains(lower, "fixme") || strings.Contains(lower, "hack"):
			findings = append(findings, fmt.Sprintf("🟡 [QUALITY] Line %d: TODO/FIXME/HACK comment found", i+1))
		case strings.Contains(lower, "panic("):
			findings = append(findings, fmt.Sprintf("🟡 [ERROR-HANDLING] Line %d: Unhandled panic() call", i+1))
		}
	}

	if len(findings) == 0 {
		return "🌿 AI Review: No obvious issues found in the current changes.\n\nConsider reviewing manually for:\n- Logic correctness\n- Edge cases\n- Performance implications"
	}

	var sb strings.Builder
	sb.WriteString("🌿 AI Review Results:\n\n")
	for _, f := range findings {
		sb.WriteString(f + "\n")
	}
	sb.WriteString(fmt.Sprintf("\n%d finding(s) in %d lines of diff", len(findings), len(lines)))
	return sb.String()
}

func extractGitScope(filePath string) string {
	parts := strings.Split(filePath, "/")
	if len(parts) > 1 {
		return parts[len(parts)-2]
	}
	name := parts[0]
	ext := strings.LastIndex(name, ".")
	if ext > 0 {
		name = name[:ext]
	}
	if name == "" {
		return "general"
	}
	return name
}

func ProcessLSPCommand(m *Model, args []string) {
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput(
			"LSP Status:\n\n"+
				"Available language servers:\n"+
				"  .go   → gopls\n"+
				"  .ts   → typescript-language-server\n"+
				"  .tsx  → typescript-language-server\n"+
				"  .js   → typescript-language-server\n"+
				"  .py   → pylsp\n"+
				"  .rs   → rust-analyzer\n\n"+
				"Commands:\n"+
				"  /lsp servers           - List active LSP servers\n"+
				"  /lsp definition <file> <line> <char>  - Go to definition\n"+
				"  /lsp references <file> <line> <char>  - Find references\n"+
				"  /lsp symbols <file>    - Document symbols\n"+
				"  /lsp hover <file> <line> <char>       - Hover info\n"+
				"  /lsp completion <file> <line> <char>  - Completions\n"+
				"  /lsp stop              - Stop all LSP servers"))
		return
	}

	switch args[0] {
	case "servers", "list", "ls":
		AddOutput(m, m.formatAssistantOutput(
			"Active LSP Servers:\n\n"+
				"Cached clients are managed per file extension.\n"+
				"Use /lsp stop to shut down all servers."))

	case "definition", "def":
		if len(args) < 4 {
			AddOutput(m, m.formatError("Usage: /lsp definition <file> <line> <character>"))
			return
		}
		result, err := executeLSPOperation(args[1], "definition", args[2:])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("LSP definition failed: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Definition:\n%v", result)))

	case "references", "refs":
		if len(args) < 4 {
			AddOutput(m, m.formatError("Usage: /lsp references <file> <line> <character>"))
			return
		}
		result, err := executeLSPOperation(args[1], "references", args[2:])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("LSP references failed: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("References:\n%v", result)))

	case "symbols", "sym":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /lsp symbols <file>"))
			return
		}
		result, err := executeLSPOperation(args[1], "symbols", nil)
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("LSP symbols failed: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Symbols:\n%v", result)))

	case "hover":
		if len(args) < 4 {
			AddOutput(m, m.formatError("Usage: /lsp hover <file> <line> <character>"))
			return
		}
		result, err := executeLSPOperation(args[1], "hover", args[2:])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("LSP hover failed: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Hover:\n%v", result)))

	case "completion", "comp":
		if len(args) < 4 {
			AddOutput(m, m.formatError("Usage: /lsp completion <file> <line> <character>"))
			return
		}
		result, err := executeLSPOperation(args[1], "completion", args[2:])
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("LSP completion failed: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Completions:\n%v", result)))

	case "stop":
		tools.CloseAllLSPPClients()
		AddOutput(m, m.formatAssistantOutput("✓ All LSP servers stopped"))

	default:
		AddOutput(m, m.formatAssistantOutput(
			"LSP Commands:\n"+
				"  /lsp                     - Show LSP status\n"+
				"  /lsp servers             - List active servers\n"+
				"  /lsp definition <f> <l> <c>  - Go to definition\n"+
				"  /lsp references <f> <l> <c>  - Find references\n"+
				"  /lsp symbols <file>      - Document symbols\n"+
				"  /lsp hover <f> <l> <c>  - Hover info\n"+
				"  /lsp completion <f> <l> <c>  - Completions\n"+
				"  /lsp stop                - Stop all servers"))
	}
}

func executeLSPOperation(filePath, operation string, posArgs []string) (any, error) {
	rootPath := "."
	client, err := tools.GetOrCreateLSPClient(filePath, rootPath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var line, character int
	if len(posArgs) >= 2 {
		fmt.Sscanf(posArgs[0], "%d", &line)
		fmt.Sscanf(posArgs[1], "%d", &character)
	}

	switch operation {
	case "definition":
		return client.GotoDefinition(ctx, filePath, line, character)
	case "references":
		return client.FindReferences(ctx, filePath, line, character)
	case "symbols":
		return client.DocumentSymbols(ctx, filePath)
	case "hover":
		return client.Hover(ctx, filePath, line, character)
	case "completion":
		return client.Completion(ctx, filePath, line, character)
	default:
		return nil, fmt.Errorf("unknown LSP operation: %s", operation)
	}
}

func ProcessTeamCommand(m *Model, args []string) {
	registry := tools.GetTeamRegistry()

	if len(args) == 0 {
		teamIDs := registry.List()
		if len(teamIDs) == 0 {
			AddOutput(m, m.formatAssistantOutput(
				"Team Collaboration:\n\n"+
					"No teams created yet.\n\n"+
					"Commands:\n"+
					"  /team create <name> [desc]      - Create a team\n"+
					"  /team list                      - List teams\n"+
					"  /team info <id>                 - Show team details\n"+
					"  /team share <id> <content>      - Share a memory\n"+
					"  /team memories <id>             - List team memories\n"+
					"  /team search <id> <query>       - Search memories\n"+
					"  /team sync <id>                 - Sync with remote\n"+
					"  /team config <id>               - Configure sync\n"+
					"  /team delete <id>               - Delete a team"))
			return
		}

		var sb strings.Builder
		sb.WriteString("Teams:\n\n")
		for _, id := range teamIDs {
			sb.WriteString(fmt.Sprintf("  • %s\n", id))
		}
		sb.WriteString("\nUse /team info <id> for details")
		AddOutput(m, m.formatAssistantOutput(sb.String()))
		return
	}

	switch args[0] {
	case "create", "new":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /team create <name> [description]"))
			return
		}
		name := args[1]
		desc := ""
		if len(args) >= 3 {
			desc = strings.Join(args[2:], " ")
		}

		result, err := executeTeamTool("team_create", map[string]any{
			"name": name, "description": desc,
		})
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to create team: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Created team: %s (ID: %s)", name, result.(map[string]any)["id"])))

	case "list", "ls":
		teamIDs := registry.List()
		if len(teamIDs) == 0 {
			AddOutput(m, m.formatAssistantOutput("No teams. Use /team create <name> to create one."))
			return
		}
		var sb strings.Builder
		sb.WriteString("Teams:\n\n")
		for _, id := range teamIDs {
			sb.WriteString(fmt.Sprintf("  • %s\n", id))
		}
		AddOutput(m, m.formatAssistantOutput(sb.String()))

	case "info":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /team info <team-id>"))
			return
		}
		teamID := args[1]
		tms, ok := registry.Get(teamID)
		if !ok {
			AddOutput(m, m.formatError(fmt.Sprintf("Team not found: %s", teamID)))
			return
		}
		stats := tms.GetStats()
		team := tms.GetTeam()
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Team: %s\n", teamID))
		if team != nil {
			sb.WriteString(fmt.Sprintf("  Name: %s\n", team.Name))
			sb.WriteString(fmt.Sprintf("  Description: %s\n", team.Description))
			sb.WriteString(fmt.Sprintf("  Members: %d\n", len(team.Members)))
		}
		sb.WriteString(fmt.Sprintf("  Total memories: %v\n", stats["total_memories"]))
		sb.WriteString(fmt.Sprintf("  Sync enabled: %v\n", stats["sync_enabled"]))
		sb.WriteString(fmt.Sprintf("  Last sync: %v\n", stats["last_sync"]))
		AddOutput(m, m.formatAssistantOutput(sb.String()))

	case "share":
		if len(args) < 3 {
			AddOutput(m, m.formatError("Usage: /team share <team-id> <content> [title]"))
			return
		}
		teamID := args[1]
		content := strings.Join(args[2:], " ")
		title := fmt.Sprintf("Shared note (%s)", time.Now().Format("15:04"))

		_, err := executeTeamTool("team_share_memory", map[string]any{
			"team_id": teamID, "title": title, "content": content, "type": "code", "visibility": "team",
		})
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to share memory: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Shared memory with team %s", teamID)))

	case "memories":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /team memories <team-id>"))
			return
		}
		teamID := args[1]
		result, err := executeTeamTool("team_get_memories", map[string]any{
			"team_id": teamID,
		})
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to get memories: %v", err)))
			return
		}
		resMap := result.(map[string]any)
		memories := resMap["memories"].([]map[string]any)
		if len(memories) == 0 {
			AddOutput(m, m.formatAssistantOutput("No memories in this team"))
			return
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Team Memories (%d):\n\n", len(memories)))
		for _, mem := range memories {
			sb.WriteString(fmt.Sprintf("  • [%s] %s (%s)\n", mem["type"], mem["title"], mem["id"]))
		}
		AddOutput(m, m.formatAssistantOutput(sb.String()))

	case "search":
		if len(args) < 3 {
			AddOutput(m, m.formatError("Usage: /team search <team-id> <query>"))
			return
		}
		teamID := args[1]
		query := strings.Join(args[2:], " ")
		result, err := executeTeamTool("team_search_memories", map[string]any{
			"team_id": teamID, "query": query,
		})
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Search failed: %v", err)))
			return
		}
		resMap := result.(map[string]any)
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Found %v results for '%s'", resMap["count"], query)))

	case "sync":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /team sync <team-id>"))
			return
		}
		teamID := args[1]
		_, err := executeTeamTool("team_sync", map[string]any{
			"team_id": teamID,
		})
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Sync failed: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Synced team %s", teamID)))

	case "config":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /team config <team-id> [api_endpoint] [api_key]"))
			return
		}
		teamID := args[1]
		if len(args) < 4 {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
				"Usage: /team config %s <api_endpoint> <api_key>\n\nSet the remote sync endpoint and API key for the team.", teamID)))
			return
		}
		apiEndpoint := args[2]
		apiKey := args[3]
		_, err := executeTeamTool("team_sync", map[string]any{
			"team_id": teamID, "api_endpoint": apiEndpoint, "api_key": apiKey,
		})
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Config failed: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Configured team %s with endpoint %s", teamID, apiEndpoint)))

	case "delete", "rm":
		if len(args) < 2 {
			AddOutput(m, m.formatError("Usage: /team delete <team-id>"))
			return
		}
		_, err := executeTeamTool("team_delete", map[string]any{"id": args[1]})
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to delete team: %v", err)))
			return
		}
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Deleted team: %s", args[1])))

	default:
		AddOutput(m, m.formatAssistantOutput(
			"Team Commands:\n"+
				"  /team                     - Show team status\n"+
				"  /team create <name>       - Create a team\n"+
				"  /team list                - List teams\n"+
				"  /team info <id>           - Show team details\n"+
				"  /team share <id> <content> - Share a memory\n"+
				"  /team memories <id>       - List team memories\n"+
				"  /team search <id> <query> - Search memories\n"+
				"  /team sync <id>           - Sync with remote\n"+
				"  /team config <id> <ep> <key> - Configure sync\n"+
				"  /team delete <id>         - Delete a team"))
	}
}

func executeTeamTool(toolName string, input map[string]any) (any, error) {
	registry := tools.GetRegistry()
	tool := registry.Get(toolName)
	if tool == nil {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return tool.Execute(ctx, input)
}

func ProcessProviderCommand(m *Model, args []string) {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".smartclaw", "config.json")

	if len(args) == 0 {
		showProviderConfig(m, configPath)
		return
	}

	switch args[0] {
	case "set":
		if len(args) < 3 {
			AddOutput(m, m.formatError("Usage: /provider set <key> <value>\nKeys: api_key, base_url, model, openai"))
			return
		}
		key, value := args[1], args[2]
		updateProviderConfig(m, configPath, key, value)
	case "show":
		showProviderConfig(m, configPath)
	default:
		AddOutput(m, m.formatError("Usage: /provider [show] | /provider set <key> <value>\nKeys: api_key, base_url, model, openai"))
	}
}

func showProviderConfig(m *Model, configPath string) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		AddOutput(m, m.formatAssistantOutput("No provider config found. Use /provider set to configure."))
		return
	}
	var cfg map[string]any
	if json.Unmarshal(data, &cfg) != nil {
		AddOutput(m, m.formatError("Failed to parse config"))
		return
	}

	apiKey, _ := cfg["api_key"].(string)
	if len(apiKey) > 8 {
		apiKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	}
	baseURL, _ := cfg["base_url"].(string)
	model, _ := cfg["model"].(string)
	openai, _ := cfg["openai"].(bool)

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
		"Provider Configuration:\n  model:    %s\n  base_url: %s\n  api_key:  %s\n  openai:   %v",
		model, baseURL, apiKey, openai)))
}

func updateProviderConfig(m *Model, configPath, key, value string) {
	var cfg map[string]any
	data, err := os.ReadFile(configPath)
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]any)
	}

	switch key {
	case "api_key":
		cfg["api_key"] = value
	case "base_url":
		cfg["base_url"] = value
	case "model":
		cfg["model"] = value
	case "openai":
		cfg["openai"] = value == "true" || value == "1"
	default:
		AddOutput(m, m.formatError(fmt.Sprintf("Unknown key: %s. Valid keys: api_key, base_url, model, openai", key)))
		return
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		AddOutput(m, m.formatError("Failed to marshal config"))
		return
	}
	if err := os.WriteFile(configPath, out, 0644); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Failed to write config: %v", err)))
		return
	}

	if m.apiClient != nil {
		if key == "model" {
			m.apiClient.SetModel(value)
			m.model = value
		}
		if key == "openai" {
			m.apiClient.SetOpenAI(value == "true" || value == "1")
		}
	}

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Updated %s. Restart for full effect.", key)))
}

func ProcessMCPCommand(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		registry := mcp.NewMCPServerRegistry()
		servers := registry.ListServers()
		clientRegistry := tools.GetMCPRegistry()

		if len(servers) == 0 {
			AddOutput(m, m.formatAssistantOutput("No MCP servers configured.\nEdit ~/.smartclaw/mcp/servers.json to add servers.\nUse /mcp-start <name> to connect."))
			return nil
		}

		var sb strings.Builder
		sb.WriteString("MCP Servers:\n")
		for _, s := range servers {
			status := "stopped"
			toolCount := 0
			if client, ok := clientRegistry.Get(s.Name); ok && client.IsReady() {
				status = "connected"
				if t, err := client.ListTools(context.Background()); err == nil {
					toolCount = len(t)
				}
			}
			sb.WriteString(fmt.Sprintf("\n  %s:\n", s.Name))
			sb.WriteString(fmt.Sprintf("    Type: %s\n", s.Type))
			if s.Command != "" {
				sb.WriteString(fmt.Sprintf("    Command: %s\n", s.Command))
			}
			if s.URL != "" {
				sb.WriteString(fmt.Sprintf("    URL: %s\n", s.URL))
			}
			sb.WriteString(fmt.Sprintf("    Status: %s\n", status))
			if toolCount > 0 {
				sb.WriteString(fmt.Sprintf("    Tools: %d\n", toolCount))
			}
		}
		AddOutput(m, m.formatAssistantOutput(sb.String()))
		return nil
	}

	switch args[0] {
	case "list", "ls":
		return ProcessMCPCommand(m, nil)
	case "start":
		return ProcessMCPStartCommand(m, args[1:])
	case "stop":
		ProcessMCPStopCommand(m, args[1:])
	default:
		AddOutput(m, m.formatAssistantOutput("MCP Commands:\n  /mcp           - List servers\n  /mcp list      - List servers\n  /mcp start <n> - Connect server\n  /mcp stop <n>  - Disconnect server"))
	}
	return nil
}

func ProcessMCPStartCommand(m *Model, args []string) tea.Cmd {
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /mcp-start <server-name>\n\nExample: /mcp-start sopa"))
		return nil
	}

	name := args[0]
	registry := mcp.NewMCPServerRegistry()
	serverConfig, ok := registry.GetServer(name)
	if !ok {
		AddOutput(m, m.formatError(fmt.Sprintf("MCP server '%s' not found. Edit ~/.smartclaw/mcp/servers.json", name)))
		return nil
	}

	if _, ok := tools.GetMCPRegistry().Get(name); ok {
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("MCP server '%s' is already connected", name)))
		return nil
	}

	mcpConfig := &mcp.McpServerConfig{
		Name:      serverConfig.Name,
		Transport: "stdio",
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
		Env:       serverConfig.Env,
	}

	if serverConfig.Type == "sse" || serverConfig.Type == "http" || serverConfig.Type == "remote" {
		mcpConfig.Transport = "sse"
		mcpConfig.URL = serverConfig.URL
		mcpConfig.Headers = serverConfig.Headers
	}

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Connecting to '%s'...", name)))

	return func() tea.Msg {
		client, err := tools.GetMCPRegistry().Connect(context.Background(), name, mcpConfig)
		if err != nil {
			return ErrorMsg{err: fmt.Sprintf("Failed to connect to '%s': %v", name, err)}
		}

		mcpTools, _ := client.ListTools(context.Background())
		mcpResources, _ := client.ListResources(context.Background())
		return OutputMsg{text: fmt.Sprintf("✓ Connected to '%s' (%d tools, %d resources)", name, len(mcpTools), len(mcpResources))}
	}
}

func ProcessMCPStopCommand(m *Model, args []string) {
	if len(args) == 0 {
		AddOutput(m, m.formatAssistantOutput("Usage: /mcp-stop <server-name>"))
		return
	}

	name := args[0]
	registry := tools.GetMCPRegistry()
	if _, ok := registry.Get(name); !ok {
		AddOutput(m, m.formatError(fmt.Sprintf("MCP server '%s' is not connected", name)))
		return
	}

	if err := registry.Disconnect(name); err != nil {
		AddOutput(m, m.formatError(fmt.Sprintf("Failed to disconnect: %v", err)))
		return
	}

	AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Disconnected from MCP server '%s'", name)))
}
