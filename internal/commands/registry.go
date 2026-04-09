package commands

import (
	"fmt"
	"sort"
	"strings"
)

type Command struct {
	Name        string
	Summary     string
	Usage       string
	Aliases     []string
	Description string
}

type CommandHandler func(args []string) error

type CommandRegistry struct {
	commands map[string]Command
	handlers map[string]CommandHandler
}

func NewRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]Command),
		handlers: make(map[string]CommandHandler),
	}
}

func (r *CommandRegistry) Register(cmd Command, handler CommandHandler) {
	r.commands[cmd.Name] = cmd
	r.handlers[cmd.Name] = handler
	for _, alias := range cmd.Aliases {
		r.commands[alias] = cmd
		r.handlers[alias] = handler
	}
}

func (r *CommandRegistry) Get(name string) Command {
	return r.commands[name]
}

func (r *CommandRegistry) Execute(name string, args []string) error {
	handler, exists := r.handlers[name]
	if !exists {
		return fmt.Errorf("unknown command: /%s", name)
	}
	return handler(args)
}

func (r *CommandRegistry) Has(name string) bool {
	_, exists := r.handlers[name]
	return exists
}

func (r *CommandRegistry) All() []Command {
	seen := make(map[string]bool)
	var result []Command
	for _, cmd := range r.commands {
		if !seen[cmd.Name] {
			seen[cmd.Name] = true
			result = append(result, cmd)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (r *CommandRegistry) Help() string {
	var lines []string
	lines = append(lines, "Available commands:")

	commands := r.All()
	for _, cmd := range commands {
		lines = append(lines, fmt.Sprintf("  /%-15s %s", cmd.Name, cmd.Summary))
	}

	return strings.Join(lines, "\n")
}

var defaultRegistry *CommandRegistry

func init() {
	defaultRegistry = NewRegistry()
	registerAllCommands()
}

func GetRegistry() *CommandRegistry {
	return defaultRegistry
}

func Register(cmd Command, handler CommandHandler) {
	defaultRegistry.Register(cmd, handler)
}

func Execute(name string, args []string) error {
	return defaultRegistry.Execute(name, args)
}

func registerAllCommands() {
	defaultRegistry.Register(Command{
		Name:    "help",
		Summary: "Show available commands",
		Aliases: []string{"h", "?"},
	}, helpHandler)

	defaultRegistry.Register(Command{
		Name:    "status",
		Summary: "Show session status",
	}, statusHandler)

	defaultRegistry.Register(Command{
		Name:    "exit",
		Summary: "Exit REPL",
		Aliases: []string{"quit", "q"},
	}, exitHandler)

	defaultRegistry.Register(Command{
		Name:    "clear",
		Summary: "Clear session",
	}, clearHandler)

	defaultRegistry.Register(Command{
		Name:    "model",
		Summary: "Show or set model",
	}, modelHandler)

	defaultRegistry.Register(Command{
		Name:    "model-list",
		Summary: "List available models",
	}, modelListHandler)

	defaultRegistry.Register(Command{
		Name:    "cost",
		Summary: "Show token usage and cost",
	}, costHandler)

	defaultRegistry.Register(Command{
		Name:    "compact",
		Summary: "Compact session history",
	}, compactHandler)

	defaultRegistry.Register(Command{
		Name:    "config",
		Summary: "Show configuration",
	}, configHandler)

	defaultRegistry.Register(Command{
		Name:    "set-api-key",
		Summary: "Set API key",
	}, setAPIKeyHandler)

	defaultRegistry.Register(Command{
		Name:    "doctor",
		Summary: "Run diagnostics",
	}, doctorHandler)

	defaultRegistry.Register(Command{
		Name:    "permissions",
		Summary: "Manage permissions",
	}, permissionsHandler)

	defaultRegistry.Register(Command{
		Name:    "memory",
		Summary: "Show memory context",
	}, memoryHandler)

	defaultRegistry.Register(Command{
		Name:    "session",
		Summary: "List sessions",
	}, sessionHandler)

	defaultRegistry.Register(Command{
		Name:    "resume",
		Summary: "Resume a session",
	}, resumeHandler)

	defaultRegistry.Register(Command{
		Name:    "export",
		Summary: "Export session",
	}, exportHandler)

	defaultRegistry.Register(Command{
		Name:    "import",
		Summary: "Import session",
	}, importHandler)

	defaultRegistry.Register(Command{
		Name:    "git-status",
		Summary: "Show git status",
		Aliases: []string{"gs"},
	}, gitStatusHandler)

	defaultRegistry.Register(Command{
		Name:    "git-diff",
		Summary: "Show git diff",
		Aliases: []string{"gd"},
	}, gitDiffHandler)

	defaultRegistry.Register(Command{
		Name:    "git-commit",
		Summary: "Commit changes",
		Aliases: []string{"gc"},
	}, gitCommitHandler)

	defaultRegistry.Register(Command{
		Name:    "git-branch",
		Summary: "List branches",
		Aliases: []string{"gb"},
	}, gitBranchHandler)

	defaultRegistry.Register(Command{
		Name:    "git-log",
		Summary: "Show git log",
		Aliases: []string{"gl"},
	}, gitLogHandler)

	defaultRegistry.Register(Command{
		Name:    "mcp",
		Summary: "Manage MCP servers",
	}, mcpHandler)

	defaultRegistry.Register(Command{
		Name:    "mcp-add",
		Summary: "Add MCP server",
	}, mcpAddHandler)

	defaultRegistry.Register(Command{
		Name:    "mcp-remove",
		Summary: "Remove MCP server",
	}, mcpRemoveHandler)

	defaultRegistry.Register(Command{
		Name:    "mcp-list",
		Summary: "List MCP servers",
	}, mcpListHandler)

	defaultRegistry.Register(Command{
		Name:    "tools",
		Summary: "List available tools",
	}, toolsHandler)

	defaultRegistry.Register(Command{
		Name:    "skills",
		Summary: "List available skills",
	}, skillsHandler)

	defaultRegistry.Register(Command{
		Name:    "agents",
		Summary: "List available agents",
	}, agentsHandler)

	defaultRegistry.Register(Command{
		Name:    "tasks",
		Summary: "List or manage tasks",
	}, tasksHandler)

	defaultRegistry.Register(Command{
		Name:    "init",
		Summary: "Initialize new project",
	}, initHandler)

	defaultRegistry.Register(Command{
		Name:    "diff",
		Summary: "Show git diff",
	}, diffHandler)

	defaultRegistry.Register(Command{
		Name:    "theme",
		Summary: "Manage themes",
	}, themeHandler)

	defaultRegistry.Register(Command{
		Name:    "version",
		Summary: "Show version",
		Aliases: []string{"v"},
	}, versionHandler)

	defaultRegistry.Register(Command{
		Name:    "save",
		Summary: "Save current session",
	}, saveHandler)

	defaultRegistry.Register(Command{
		Name:    "rename",
		Summary: "Rename session",
	}, renameHandler)

	defaultRegistry.Register(Command{
		Name:    "plan",
		Summary: "Plan mode",
	}, planHandler)

	defaultRegistry.Register(Command{
		Name:    "login",
		Summary: "Authenticate with service",
	}, loginHandler)

	defaultRegistry.Register(Command{
		Name:    "logout",
		Summary: "Clear authentication",
	}, logoutHandler)

	defaultRegistry.Register(Command{
		Name:    "upgrade",
		Summary: "Upgrade CLI version",
	}, upgradeHandler)

	defaultRegistry.Register(Command{
		Name:    "context",
		Summary: "Manage context",
	}, contextHandler)

	defaultRegistry.Register(Command{
		Name:    "stats",
		Summary: "Show session statistics",
	}, statsHandler)

	defaultRegistry.Register(Command{
		Name:    "voice",
		Summary: "Voice mode control",
	}, voiceHandler)

	defaultRegistry.Register(Command{
		Name:    "hooks",
		Summary: "Manage hooks",
	}, hooksHandler)

	defaultRegistry.Register(Command{
		Name:    "plugin",
		Summary: "Manage plugins",
	}, pluginHandler)

	defaultRegistry.Register(Command{
		Name:    "reset-limits",
		Summary: "Reset limits",
	}, resetLimitsHandler)

	defaultRegistry.Register(Command{
		Name:    "attach",
		Summary: "Attach to process",
	}, attachHandler)

	defaultRegistry.Register(Command{
		Name:    "browse",
		Summary: "Open browser",
	}, browseHandler)

	defaultRegistry.Register(Command{
		Name:    "cache",
		Summary: "Manage cache",
	}, cacheHandler)

	defaultRegistry.Register(Command{
		Name:    "debug",
		Summary: "Toggle debug mode",
	}, debugHandler)

	defaultRegistry.Register(Command{
		Name:    "deepthink",
		Summary: "Enable deep thinking",
	}, deepThinkHandler)

	defaultRegistry.Register(Command{
		Name:    "env",
		Summary: "Show environment",
	}, envHandler)

	defaultRegistry.Register(Command{
		Name:    "exec",
		Summary: "Execute command",
	}, execHandler)

	defaultRegistry.Register(Command{
		Name:    "fork",
		Summary: "Fork session",
	}, forkHandler)

	defaultRegistry.Register(Command{
		Name:    "inspect",
		Summary: "Inspect state",
	}, inspectHandler)

	defaultRegistry.Register(Command{
		Name:    "invite",
		Summary: "Invite collaboration",
	}, inviteHandler)

	defaultRegistry.Register(Command{
		Name:    "lazy",
		Summary: "Lazy mode",
	}, lazyHandler)

	defaultRegistry.Register(Command{
		Name:    "lsp",
		Summary: "LSP operations",
	}, lspHandler)

	defaultRegistry.Register(Command{
		Name:    "mcp-start",
		Summary: "Start MCP server",
	}, mcpStartHandler)

	defaultRegistry.Register(Command{
		Name:    "mcp-stop",
		Summary: "Stop MCP server",
	}, mcpStopHandler)

	defaultRegistry.Register(Command{
		Name:    "observe",
		Summary: "Observe mode",
	}, observeHandler)

	defaultRegistry.Register(Command{
		Name:    "preview",
		Summary: "Preview changes",
	}, previewHandler)

	defaultRegistry.Register(Command{
		Name:    "read",
		Summary: "Read file",
	}, readHandler)

	defaultRegistry.Register(Command{
		Name:    "subagent",
		Summary: "Spawn subagent",
	}, subagentHandler)

	defaultRegistry.Register(Command{
		Name:    "think",
		Summary: "Think mode",
	}, thinkHandler)

	defaultRegistry.Register(Command{
		Name:    "web",
		Summary: "Web operations",
	}, webHandler)

	defaultRegistry.Register(Command{
		Name:    "write",
		Summary: "Write file",
	}, writeHandler)

	defaultRegistry.Register(Command{
		Name:    "advisor",
		Summary: "AI advisor",
	}, advisorHandler)

	defaultRegistry.Register(Command{
		Name:    "btw",
		Summary: "By the way",
	}, btwHandler)

	defaultRegistry.Register(Command{
		Name:    "bughunter",
		Summary: "Bug hunting mode",
	}, bughunterHandler)

	defaultRegistry.Register(Command{
		Name:    "chrome",
		Summary: "Chrome integration",
	}, chromeHandler)

	defaultRegistry.Register(Command{
		Name:    "color",
		Summary: "Color theme",
	}, colorHandler)

	defaultRegistry.Register(Command{
		Name:    "commit",
		Summary: "Git commit",
	}, commitHandler)

	defaultRegistry.Register(Command{
		Name:    "copy",
		Summary: "Copy to clipboard",
	}, copyHandler)

	defaultRegistry.Register(Command{
		Name:    "desktop",
		Summary: "Desktop mode",
	}, desktopHandler)

	defaultRegistry.Register(Command{
		Name:    "effort",
		Summary: "Effort tracking",
	}, effortHandler)

	defaultRegistry.Register(Command{
		Name:    "fast",
		Summary: "Fast mode",
	}, fastHandler)

	defaultRegistry.Register(Command{
		Name:    "feedback",
		Summary: "Send feedback",
	}, feedbackHandler)

	defaultRegistry.Register(Command{
		Name:    "files",
		Summary: "List files",
	}, filesHandler)

	defaultRegistry.Register(Command{
		Name:    "heapdump",
		Summary: "Heap dump",
	}, heapdumpHandler)

	defaultRegistry.Register(Command{
		Name:    "ide",
		Summary: "IDE integration",
	}, ideHandler)

	defaultRegistry.Register(Command{
		Name:    "insights",
		Summary: "Code insights",
	}, insightsHandler)

	defaultRegistry.Register(Command{
		Name:    "install",
		Summary: "Install package",
	}, installHandler)

	defaultRegistry.Register(Command{
		Name:    "issue",
		Summary: "Issue tracker",
	}, issueHandler)

	defaultRegistry.Register(Command{
		Name:    "keybindings",
		Summary: "Manage keybindings",
	}, keybindingsHandler)

	defaultRegistry.Register(Command{
		Name:    "mobile",
		Summary: "Mobile mode",
	}, mobileHandler)

	defaultRegistry.Register(Command{
		Name:    "onboarding",
		Summary: "Onboarding",
	}, onboardingHandler)

	defaultRegistry.Register(Command{
		Name:    "passes",
		Summary: "LSP passes",
	}, passesHandler)

	defaultRegistry.Register(Command{
		Name:    "rewind",
		Summary: "Rewind session",
	}, rewindHandler)

	defaultRegistry.Register(Command{
		Name:    "share",
		Summary: "Share session",
	}, shareHandler)

	defaultRegistry.Register(Command{
		Name:    "statusline",
		Summary: "Status line",
	}, statuslineHandler)

	defaultRegistry.Register(Command{
		Name:    "stickers",
		Summary: "Stickers",
	}, stickersHandler)

	defaultRegistry.Register(Command{
		Name:    "summary",
		Summary: "Session summary",
	}, summaryHandler)

	defaultRegistry.Register(Command{
		Name:    "tag",
		Summary: "Tag management",
	}, tagHandler)

	defaultRegistry.Register(Command{
		Name:    "teleport",
		Summary: "Teleport mode",
	}, teleportHandler)

	defaultRegistry.Register(Command{
		Name:    "thinkback",
		Summary: "Think back",
	}, thinkbackHandler)

	defaultRegistry.Register(Command{
		Name:    "ultraplan",
		Summary: "Ultra planning",
	}, ultraplanHandler)

	defaultRegistry.Register(Command{
		Name:    "usage",
		Summary: "Usage stats",
	}, usageHandler)

	defaultRegistry.Register(Command{
		Name:    "vim",
		Summary: "Vim mode",
	}, vimHandler)

	defaultRegistry.Register(Command{
		Name:    "api",
		Summary: "API operations",
	}, apiHandler)

	defaultRegistry.Register(Command{
		Name:    "agent",
		Summary: "Manage AI agents",
	}, agentHandler)

	defaultRegistry.Register(Command{
		Name:    "agent-list",
		Summary: "List available agents",
	}, agentListHandler)

	defaultRegistry.Register(Command{
		Name:    "agent-switch",
		Summary: "Switch to an agent",
	}, agentSwitchHandler)

	defaultRegistry.Register(Command{
		Name:    "agent-create",
		Summary: "Create custom agent",
	}, agentCreateHandler)

	defaultRegistry.Register(Command{
		Name:    "agent-delete",
		Summary: "Delete custom agent",
	}, agentDeleteHandler)

	defaultRegistry.Register(Command{
		Name:    "agent-info",
		Summary: "Show agent info",
	}, agentInfoHandler)

	defaultRegistry.Register(Command{
		Name:    "agent-export",
		Summary: "Export agent",
	}, agentExportHandler)

	defaultRegistry.Register(Command{
		Name:    "agent-import",
		Summary: "Import agent",
	}, agentImportHandler)

	defaultRegistry.Register(Command{
		Name:    "template",
		Summary: "Manage prompt templates",
	}, templateHandler)

	defaultRegistry.Register(Command{
		Name:    "template-list",
		Summary: "List templates",
	}, templateListHandler)

	defaultRegistry.Register(Command{
		Name:    "template-use",
		Summary: "Use a template",
	}, templateUseHandler)

	defaultRegistry.Register(Command{
		Name:    "template-create",
		Summary: "Create template",
	}, templateCreateHandler)

	defaultRegistry.Register(Command{
		Name:    "template-delete",
		Summary: "Delete template",
	}, templateDeleteHandler)

	defaultRegistry.Register(Command{
		Name:    "template-info",
		Summary: "Show template info",
	}, templateInfoHandler)

	defaultRegistry.Register(Command{
		Name:    "template-export",
		Summary: "Export template",
	}, templateExportHandler)

	defaultRegistry.Register(Command{
		Name:    "template-import",
		Summary: "Import template",
	}, templateImportHandler)

	defaultRegistry.Register(Command{
		Name:    "config",
		Summary: "Manage configuration",
	}, configHandler)

	defaultRegistry.Register(Command{
		Name:    "config-show",
		Summary: "Show configuration",
	}, configShowHandler)

	defaultRegistry.Register(Command{
		Name:    "config-set",
		Summary: "Set config value",
	}, configSetHandler)

	defaultRegistry.Register(Command{
		Name:    "config-get",
		Summary: "Get config value",
	}, configGetHandler)

	defaultRegistry.Register(Command{
		Name:    "config-reset",
		Summary: "Reset configuration",
	}, configResetHandler)

	defaultRegistry.Register(Command{
		Name:    "config-export",
		Summary: "Export configuration",
	}, configExportHandler)

	defaultRegistry.Register(Command{
		Name:    "config-import",
		Summary: "Import configuration",
	}, configImportHandler)
}
