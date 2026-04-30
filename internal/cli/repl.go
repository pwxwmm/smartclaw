package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/instructkr/smartclaw/internal/adapters"
	"github.com/instructkr/smartclaw/internal/agents"
	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/auth"
	"github.com/instructkr/smartclaw/internal/commands"
	"github.com/instructkr/smartclaw/internal/contextmgr"
	"github.com/instructkr/smartclaw/internal/gateway"
	"github.com/instructkr/smartclaw/internal/lifecycle"
	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/permissions"
	"github.com/instructkr/smartclaw/internal/plugins"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/store"
	"github.com/instructkr/smartclaw/internal/tools"
	"github.com/instructkr/smartclaw/internal/tui"
)

var replCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start interactive REPL mode",
	Long:  `Start an interactive REPL session with Claude AI.`,
	Run:   runREPL,
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Start TUI mode (Terminal User Interface)",
	Long:  `Start SmartClaw with a modern terminal user interface.`,
	Run:   runTUI,
}

func init() {
	rootCmd.AddCommand(replCmd)
	rootCmd.AddCommand(tuiCmd)
	replCmd.Flags().Bool("simple", false, "Use simple CLI instead of TUI")
	tuiCmd.Flags().String("remote", "", "Remote server URL (e.g., http://localhost:8080)")
	tuiCmd.Flags().String("remote-token", "", "Authentication token for remote server")
}

type REPLSession struct {
	model        string
	maxTokens    int
	showThinking bool
	session      *runtime.Session
	sessionMgr   *runtime.SessionManager
	apiClient    *api.Client
	toolReg      *tools.ToolRegistry
	cmdReg       *commands.CommandRegistry
	ctxMgr       *runtime.ContextManager
	totalTokens  int
	totalCost    float64
	learningLoop *learning.LearningLoop
}

func runREPL(cmd *cobra.Command, args []string) {
	model := viper.GetString("model")
	maxTokens := viper.GetInt("max_tokens")
	if maxTokens == 0 {
		maxTokens = 4096
	}
	showThinking := viper.GetBool("show_thinking")
	skipPerms := viper.GetBool("dangerously_skip_permissions")
	isOpenAI := viper.GetBool("openai")
	apiKeyFlag := viper.GetString("api_key")
	baseURLFlag := viper.GetString("url")

	if baseURLFlag == "" {
		baseURLFlag = viper.GetString("base_url")
	}

	session := &REPLSession{
		model:        model,
		maxTokens:    maxTokens,
		showThinking: showThinking,
		ctxMgr:       runtime.NewContextManager(100000),
	}

	sessionMgr, err := runtime.NewSessionManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create session manager: %v\n", err)
		os.Exit(1)
	}
	session.sessionMgr = sessionMgr
	session.session = sessionMgr.NewSession()

	apiKey := apiKeyFlag
	if apiKey == "" {
		apiKey = auth.GetAPIKey()
	}

	baseURL := baseURLFlag
	if baseURL == "" {
		baseURL = auth.GetBaseURL()
	}

	if apiKey == "" {
		fmt.Println("Warning: No API key set. Set ANTHROPIC_API_KEY or use /set-api-key")
	} else {
		session.apiClient = api.NewClientWithModel(apiKey, baseURL, model)
		session.apiClient.SetOpenAI(isOpenAI)
	}

	session.toolReg = tools.GetRegistry()

	workDir, _ := os.Getwd()
	tools.SetAllowedDirs([]string{workDir})

	mm, err := memory.NewMemoryManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: memory manager init failed: %v\n", err)
	} else {
		tools.SetMemoryManagerForTools(mm)
		tools.SetIncidentMemory(mm.GetIncidentMemory())
	}

	adapters.InitInnovationPackages(mm, session.apiClient)
	if mm != nil && session.apiClient != nil {
		mm.SetLLMClient(session.apiClient)
		llmAdapter := learning.NewAPIClientAdapter(session.apiClient, "")
		tools.SetLLMClientForConversationRecall(llmAdapter)
	}
	if mm != nil {
		tools.SetStoreForConversationRecall(mm.GetStore())
	}
	lifecycle.Register(adapters.NewInnovationShutdown())

	var replStore *store.Store
	if mm != nil {
		replStore = mm.GetStore()
	}
	session.learningLoop = buildLearningLoop(session.apiClient, mm, replStore)

	if shutdown, err := observability.InitOTLP(); err == nil {
		defer shutdown(context.Background())
	}

	profileRegistry := agents.NewProfileRegistry()
	agentPermMgr := permissions.NewAgentPermissionManager()

	for _, profile := range profileRegistry.List() {
		permSet := permissions.NewAgentPermissionSet(
			profile.AgentType,
			profile.Tools,
			profile.DisallowedTools,
			permissions.AgentPermissionMode(profile.PermissionMode),
		)
		agentPermMgr.Register(permSet)
	}

	tools.SetGlobalProfileRegistry(&profileRegistryAdapter{reg: profileRegistry})
	tools.SetGlobalAgentSwitchFunc(func(cfg *tools.AgentSwitchConfig) error {
		slog.Info("repl: agent switch requested", "agent", cfg.AgentType)
		if cfg.Model != "" {
			session.model = cfg.Model
			if session.apiClient != nil {
				session.apiClient.SetModel(cfg.Model)
			}
		}
		return nil
	})

	homeDir, _ := os.UserHomeDir()
	smartclawDir := filepath.Join(homeDir, ".smartclaw")
	agentsMDHierarchy := contextmgr.NewAgentsMDHierarchy(workDir, smartclawDir)
	if err := agentsMDHierarchy.Load(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: AGENTS.md hierarchy load failed: %v\n", err)
	}
	if mm != nil {
		mm.SetAgentsMDHierarchy(agentsMDHierarchy)
	}

	contextAdapter := commands.NewAgentsMDContextAdapter(agentsMDHierarchy)
	commands.SetGlobalContextManager(contextAdapter)

	cronTrigger := gateway.NewCronTrigger(nil, nil)
	commands.SetGlobalCronTrigger(&cronTriggerAdapter{ct: cronTrigger})
	commands.SetGlobalScheduleParser(&scheduleParserAdapter{})

	pluginRegistry := plugins.NewPluginRegistry("")
	if err := pluginRegistry.Initialize(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Plugin registry init failed: %v\n", err)
	}
	pluginRegistry.RegisterToolsInRegistry(tools.GetRegistry())
	commands.SetGlobalPluginRegistry(pluginRegistry)

	fmt.Println("💡 SmartClaw REPL")
	fmt.Printf("Model: %s\n", model)
	if skipPerms {
		fmt.Println("Permissions: DISABLED")
	}
	fmt.Println("Type /help for commands, /exit to quit")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("smart> ")

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if strings.HasPrefix(input, "/") {
			if session.handleSlashCommand(input) {
				break
			}
			continue
		}

		session.handlePrompt(input)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "Error reading input:", err)
	}
}

func (s *REPLSession) handleSlashCommand(input string) bool {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return false
	}

	cmd := strings.TrimPrefix(parts[0], "/")
	args := parts[1:]

	switch cmd {
	case "exit", "quit":
		fmt.Println("Goodbye!")
		return true
	case "help":
		s.printHelp()
	case "status":
		s.printStatus()
	case "model":
		if len(args) > 0 {
			s.model = args[0]
			fmt.Printf("Model set to: %s\n", s.model)
		} else {
			fmt.Printf("Current model: %s\n", s.model)
		}
	case "clear":
		s.session.Clear()
		s.ctxMgr.Clear()
		fmt.Println("Session cleared")
	case "cost":
		s.printCost()
	case "compact":
		s.compact()
	case "session":
		s.listSessions()
	case "resume":
		if len(args) > 0 {
			s.resumeSession(args[0])
		} else {
			fmt.Println("Usage: /resume <session-id>")
		}
	case "save":
		if err := s.sessionMgr.Save(s.session); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save session: %v\n", err)
		} else {
			fmt.Printf("Session saved: %s\n", s.session.ID)
		}
	case "tools":
		s.listTools()
	case "doctor":
		s.runDoctor()
	default:
		fmt.Printf("Unknown command: /%s\n", cmd)
		fmt.Println("Type /help for available commands")
	}

	return false
}

func (s *REPLSession) handlePrompt(input string) {
	if s.apiClient == nil {
		fmt.Println("Error: No API key configured. Use /set-api-key or set ANTHROPIC_API_KEY")
		return
	}

	userMsg := runtime.Message{
		Role:    "user",
		Content: input,
	}
	s.session.AddMessage(userMsg)
	s.ctxMgr.AddMessage(userMsg)

	fmt.Println("\nThinking...")

	messages := s.buildAPIMessages()

	req := &api.MessageRequest{
		Model:     s.model,
		MaxTokens: s.maxTokens,
		Messages:  messages,
		Tools:     s.buildToolSchemas(),
	}

	var response strings.Builder
	parser := api.NewStreamMessageParser()

	var err error
	if s.apiClient.IsOpenAI {
		err = s.apiClient.StreamMessageOpenAI(context.Background(), req, func(event string, data []byte) error {
			result, err := parser.HandleEvent(event, data)
			if err != nil {
				return err
			}

			if result.TextDelta != "" {
				fmt.Print(result.TextDelta)
				response.WriteString(result.TextDelta)
			}

			if s.showThinking && result.ThinkingDelta != "" {
				fmt.Print(result.ThinkingDelta)
			}

			return nil
		})
	} else {
		err = s.apiClient.StreamMessageSSE(context.Background(), req, func(event string, data []byte) error {
			result, err := parser.HandleEvent(event, data)
			if err != nil {
				return err
			}

			if result.TextDelta != "" {
				fmt.Print(result.TextDelta)
				response.WriteString(result.TextDelta)
			}

			if s.showThinking && result.ThinkingDelta != "" {
				fmt.Print(result.ThinkingDelta)
			}

			return nil
		})
	}

	fmt.Println()

	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		return
	}

	fmt.Println()

	assistantMsg := runtime.Message{
		Role:    "assistant",
		Content: response.String(),
	}
	s.session.AddMessage(assistantMsg)
	s.ctxMgr.AddMessage(assistantMsg)

	msg := parser.GetMessage()
	s.totalTokens += msg.Usage.InputTokens + msg.Usage.OutputTokens
	s.updateCost(msg.Usage.InputTokens, msg.Usage.OutputTokens)
}

func (s *REPLSession) handleToolCall(block api.ContentBlock) {
	fmt.Printf("\n[Tool: %s]\n", block.Name)

	result, err := s.toolReg.Execute(context.Background(), block.Name, block.Input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Tool error: %v\n", err)
		return
	}

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("Result: %s\n", string(resultJSON))
}

func (s *REPLSession) buildAPIMessages() []api.Message {
	messages := make([]api.Message, 0)
	for _, msg := range s.session.Messages {
		var content string
		if str, ok := msg.Content.(string); ok {
			content = str
		}
		messages = append(messages, api.Message{
			Role:    msg.Role,
			Content: content,
		})
	}
	return messages
}

func (s *REPLSession) buildToolSchemas() []api.ToolDefinition {
	complexity := tools.AssessQueryComplexity(s.lastUserInput())
	toolList := s.toolReg.SelectToolset(context.Background(), complexity)
	schemas := make([]api.ToolDefinition, 0, len(toolList))
	for _, t := range toolList {
		schemas = append(schemas, api.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return schemas
}

func (s *REPLSession) lastUserInput() string {
	for i := len(s.session.Messages) - 1; i >= 0; i-- {
		if s.session.Messages[i].Role == "user" {
			if str, ok := s.session.Messages[i].Content.(string); ok {
				return str
			}
		}
	}
	return ""
}

func (s *REPLSession) printHelp() {
	fmt.Print(`
Available commands:
  /help              Show this help
  /exit              Exit REPL
  /status            Show session status
  /model [name]      Show or set model
  /clear             Clear session
  /cost              Show token usage and cost
  /compact           Compact session history
  /save              Save current session
  /session           List sessions
  /resume <id>       Resume a session
  /tools             List available tools
  /doctor            Run diagnostics
`)
}

func (s *REPLSession) printStatus() {
	fmt.Printf("Model: %s\n", s.model)
	fmt.Printf("Session ID: %s\n", s.session.ID)
	fmt.Printf("Messages: %d\n", s.session.MessageCount())
	fmt.Printf("Tokens: %d\n", s.totalTokens)
	fmt.Printf("Cost: $%.4f\n", s.totalCost)
}

func (s *REPLSession) printCost() {
	fmt.Printf("Total tokens: %d\n", s.totalTokens)
	fmt.Printf("Estimated cost: $%.4f\n", s.totalCost)
}

func (s *REPLSession) updateCost(inputTokens, outputTokens int) {
	inputCost := float64(inputTokens) * 0.000003
	outputCost := float64(outputTokens) * 0.000015
	s.totalCost += inputCost + outputCost
}

func (s *REPLSession) compact() {
	s.ctxMgr.Trim()
	fmt.Println("Session compacted")
}

func (s *REPLSession) listSessions() {
	sessions, err := s.sessionMgr.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n", err)
		return
	}

	if len(sessions) == 0 {
		fmt.Println("No saved sessions")
		return
	}

	fmt.Println("Saved sessions:")
	for _, sess := range sessions {
		fmt.Printf("  %s (%d messages)\n", sess.ID, sess.MessageCount())
	}
}

func (s *REPLSession) resumeSession(id string) {
	sess, err := s.sessionMgr.Load(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load session: %v\n", err)
		return
	}

	s.session = sess
	fmt.Printf("Resumed session: %s\n", id)
}

func (s *REPLSession) listTools() {
	toolList := s.toolReg.All()
	fmt.Println("Available tools:")
	for _, t := range toolList {
		fmt.Printf("  %-15s %s\n", t.Name(), t.Description())
	}
}

func (s *REPLSession) runDoctor() {
	_ = commands.GetRegistry().Execute("doctor", nil)
}

func runTUI(cmd *cobra.Command, args []string) {
	model := viper.GetString("model")
	isOpenAI := viper.GetBool("openai")
	apiKeyFlag := viper.GetString("api_key")
	baseURLFlag := viper.GetString("url")
	remoteURL, _ := cmd.Flags().GetString("remote")
	remoteToken, _ := cmd.Flags().GetString("remote-token")

	if baseURLFlag == "" {
		baseURLFlag = viper.GetString("base_url")
	}

	if remoteURL != "" {
		if err := tui.StartTUIWithRemoteClient(remoteURL, remoteToken, model); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	apiKey := apiKeyFlag
	if apiKey == "" {
		apiKey = auth.GetAPIKey()
	}

	baseURL := baseURLFlag
	if baseURL == "" {
		baseURL = auth.GetBaseURL()
	}

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: No API key set. Set ANTHROPIC_API_KEY environment variable or use --api-key flag")
		os.Exit(1)
	}

	apiClient := api.NewClientWithModel(apiKey, baseURL, model)
	apiClient.SetOpenAI(isOpenAI)

	mm, err := memory.NewMemoryManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: memory manager init failed: %v\n", err)
	} else {
		tools.SetMemoryManagerForTools(mm)
		tools.SetIncidentMemory(mm.GetIncidentMemory())
	}

	workDir, _ := os.Getwd()
	tools.SetAllowedDirs([]string{workDir})

	adapters.InitInnovationPackages(mm, apiClient)
	if mm != nil && apiClient != nil {
		mm.SetLLMClient(apiClient)
		llmAdapter := learning.NewAPIClientAdapter(apiClient, "")
		tools.SetLLMClientForConversationRecall(llmAdapter)
	}
	if mm != nil {
		tools.SetStoreForConversationRecall(mm.GetStore())
	}
	lifecycle.Register(adapters.NewInnovationShutdown())

	if shutdown, err := observability.InitOTLP(); err == nil {
		defer shutdown(context.Background())
	}

	profileRegistry := agents.NewProfileRegistry()
	agentPermMgr := permissions.NewAgentPermissionManager()

	for _, profile := range profileRegistry.List() {
		permSet := permissions.NewAgentPermissionSet(
			profile.AgentType,
			profile.Tools,
			profile.DisallowedTools,
			permissions.AgentPermissionMode(profile.PermissionMode),
		)
		agentPermMgr.Register(permSet)
	}

	tools.SetGlobalProfileRegistry(&profileRegistryAdapter{reg: profileRegistry})
	tools.SetGlobalAgentSwitchFunc(func(cfg *tools.AgentSwitchConfig) error {
		slog.Info("tui: agent switch requested", "agent", cfg.AgentType)
		if cfg.Model != "" {
			apiClient.SetModel(cfg.Model)
		}
		return nil
	})

	homeDir, _ := os.UserHomeDir()
	smartclawDir := filepath.Join(homeDir, ".smartclaw")
	agentsMDHierarchy := contextmgr.NewAgentsMDHierarchy(workDir, smartclawDir)
	if err := agentsMDHierarchy.Load(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: AGENTS.md hierarchy load failed: %v\n", err)
	}
	if mm != nil {
		mm.SetAgentsMDHierarchy(agentsMDHierarchy)
	}

	contextAdapter := commands.NewAgentsMDContextAdapter(agentsMDHierarchy)
	commands.SetGlobalContextManager(contextAdapter)

	cronTrigger := gateway.NewCronTrigger(nil, nil)
	commands.SetGlobalCronTrigger(&cronTriggerAdapter{ct: cronTrigger})
	commands.SetGlobalScheduleParser(&scheduleParserAdapter{})

	pluginRegistry := plugins.NewPluginRegistry("")
	if err := pluginRegistry.Initialize(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Plugin registry init failed: %v\n", err)
	}
	pluginRegistry.RegisterToolsInRegistry(tools.GetRegistry())
	commands.SetGlobalPluginRegistry(pluginRegistry)

	var tuiStore *store.Store
	if mm != nil {
		tuiStore = mm.GetStore()
	}
	tuiLearningLoop := buildLearningLoop(apiClient, mm, tuiStore)

	if err := tui.StartTUIWithClientAndLearningLoop(apiClient, tuiLearningLoop); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type cronTriggerAdapter struct {
	ct *gateway.CronTrigger
}

func (a *cronTriggerAdapter) ListTasks() ([]commands.CronTaskInfo, error) {
	tasks, err := a.ct.ListTasks()
	if err != nil {
		return nil, err
	}
	result := make([]commands.CronTaskInfo, 0, len(tasks))
	for _, t := range tasks {
		var lastRun, createdAt time.Time
		if t.LastRunAt != "" {
			lastRun, _ = time.Parse(time.RFC3339, t.LastRunAt)
		}
		if t.CreatedAt != "" {
			createdAt, _ = time.Parse(time.RFC3339, t.CreatedAt)
		}
		result = append(result, commands.CronTaskInfo{
			ID:          t.ID,
			Schedule:    t.Schedule,
			Instruction: t.Instruction,
			Enabled:     t.Enabled,
			LastRunAt:   lastRun,
			CreatedAt:   createdAt,
		})
	}
	return result, nil
}

func (a *cronTriggerAdapter) CreateTask(schedule, instruction string) (string, error) {
	taskID := fmt.Sprintf("cron_%d", time.Now().UnixNano())
	return taskID, a.ct.ScheduleCron(taskID, "default", instruction, schedule, "terminal")
}

func (a *cronTriggerAdapter) DeleteTask(id string) error {
	return a.ct.DeleteTask(id)
}

func (a *cronTriggerAdapter) ToggleTask(id string) error {
	task, err := a.ct.GetTask(id)
	if err != nil {
		return err
	}
	if task.Enabled {
		return a.ct.DisableTask(id)
	}
	return a.ct.EnableTask(id)
}

func (a *cronTriggerAdapter) RunTask(id string) error {
	task, err := a.ct.GetTask(id)
	if err != nil {
		return err
	}
	err = a.ct.ScheduleCron(task.ID, task.UserID, task.Instruction, task.Schedule, task.Platform)
	return err
}

type scheduleParserAdapter struct{}

func (s *scheduleParserAdapter) ParseNaturalLanguage(input string) (string, error) {
	return gateway.ParseNaturalLanguage(input)
}

type profileRegistryAdapter struct {
	reg *agents.ProfileRegistry
}

func (a *profileRegistryAdapter) Get(name string) (string, string, string, []string, []string, string, int, error) {
	p, err := a.reg.Get(name)
	if err != nil {
		return "", "", "", nil, nil, "", 0, err
	}
	return p.AgentType, p.SystemPrompt, p.Model, p.Tools, p.DisallowedTools, string(p.PermissionMode), p.MaxTurns, nil
}

func (a *profileRegistryAdapter) List() []tools.ProfileEntry {
	profiles := a.reg.List()
	entries := make([]tools.ProfileEntry, 0, len(profiles))
	for _, p := range profiles {
		entries = append(entries, tools.ProfileEntry{
			AgentType:       p.AgentType,
			WhenToUse:       p.WhenToUse,
			SystemPrompt:    p.SystemPrompt,
			Tools:           p.Tools,
			DisallowedTools: p.DisallowedTools,
			Model:           p.Model,
			PermissionMode:  string(p.PermissionMode),
			MaxTurns:        p.MaxTurns,
		})
	}
	return entries
}
