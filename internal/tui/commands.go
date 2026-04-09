package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/instructkr/smartclaw/internal/api"
)

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
		InitialModelWithClient(client),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
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

func ProcessSlashCommand(cmd string, m *Model) tea.Cmd {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return nil
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "/help":
		m.showHelp = !m.showHelp
		AddOutput(m, m.formatAssistantOutput("Help toggled. Press Ctrl+H to toggle again."))
	case "/test":
		AddOutput(m, m.formatAssistantOutput("✓ Command processing is working! Session ID: "+m.currentSession.ID))
	case "/status":
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Model: %s\nTokens: %d\nCost: $%.4f\nMode: %s\nTheme: %s",
			m.model, m.tokens, m.cost, m.mode, m.theme.Name)))
	case "/clear":
		ClearOutput(m)
	case "/exit", "/quit":
		return tea.Quit
	case "/edit":
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

	case "/editor":
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

	case "/multilines", "/multiline":
		AddOutput(m, m.formatAssistantOutput("Opening editor for multiline input..."))

		currentContent := m.textArea.Value()
		content, err := m.editorManager.EditContent(currentContent, ".txt")
		if err != nil {
			AddOutput(m, m.formatError(fmt.Sprintf("Failed to open editor: %v", err)))
			return nil
		}

		m.textArea.SetValue(content)
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("✓ Updated input (%d bytes)", len(content))))

	case "/model":
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
	case "/cost":
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Tokens used: %d\nEstimated cost: $%.4f", m.tokens, m.cost)))
	case "/retry":
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
	case "/voice":
		ProcessVoiceCommand(m, args)
	case "/theme":
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
	case "/mode":
		if len(args) > 0 {
			m.mode = args[0]
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Permission mode changed to: %s", args[0])))
		} else {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf("Current mode: %s\nAvailable modes: ask, read-only, workspace-write, danger-full-access", m.mode)))
		}
	case "/context":
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
	case "/tabs":
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
	case "/loading":
		m.loading = !m.loading
		if m.loading {
			m.spinner.Start()
			AddOutput(m, m.formatAssistantOutput("Loading indicator started"))
		} else {
			m.spinner.Stop()
			AddOutput(m, m.formatAssistantOutput("Loading indicator stopped"))
		}
	case "/dialog":
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
	case "/session":
		if m.sessionManager == nil {
			AddOutput(m, m.formatError("Session manager not initialized. Please restart SmartClaw."))
			return nil
		}

		if m.currentSession == nil {
			AddOutput(m, m.formatError("No active session. Creating new session..."))
			m.currentSession = m.sessionManager.NewSession(m.model)
			return nil
		}

		if len(args) == 0 {
			AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
				"Session Management Commands:\n"+
					"  /session new         - Create new session\n"+
					"  /session list        - List all sessions\n"+
					"  /session load <id>   - Load a session\n"+
					"  /session save        - Save current session\n"+
					"  /session delete <id> - Delete a session\n"+
					"  /session export <id> - Export session (markdown/json)\n\n"+
					"Current session: %s\n"+
					"Messages: %d",
				m.currentSession.ID, len(m.currentSession.Messages))))
			return nil
		}

		switch args[0] {
		case "new":
			m.currentSession = m.sessionManager.NewSession(m.model)
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

		default:
			AddOutput(m, m.formatError(fmt.Sprintf("Unknown session command: %s", args[0])))
		}

	case "/agent":
		ProcessAgentCommand(m, args)

	case "/template":
		ProcessTemplateCommand(m, args)

	case "/subagent":
		ProcessSubagentCommand(m, args)

	case "/memory":
		ProcessMemoryCommand(m, args)

	case "/compact":
		ProcessCompactCommand(m, args)

	default:
		AddOutput(m, m.formatError(fmt.Sprintf("Unknown command: %s. Type /help for available commands.", command)))
	}

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
		AddOutput(m, m.formatAssistantOutput("Auto-compact configuration (coming soon)\n\nUse /compact now to manually compact."))

	case "status":
		AddOutput(m, m.formatAssistantOutput(fmt.Sprintf(
			"Compact Statistics:\n"+
				"  Total messages: %d\n"+
				"  Kept messages: %d\n"+
				"  Current tokens: %d\n"+
				"  Max tokens: %d\n"+
				"  Usage: %.1f%%",
			m.contextManager.GetMessageCount(),
			countKeptMessages(m),
			m.contextManager.GetTokenCount(),
			m.contextManager.maxTokens,
			float64(m.contextManager.GetTokenCount())/float64(m.contextManager.maxTokens)*100)))

	case "config":
		AddOutput(m, m.formatAssistantOutput(
			"Compact Configuration:\n"+
				"  Auto-compact: enabled\n"+
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
