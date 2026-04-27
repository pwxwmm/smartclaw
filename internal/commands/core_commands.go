package commands

import (
	"fmt"
	"runtime"
	"time"
)

var cmdCtx *CommandContext

func init() {
	cmdCtx = NewCommandContext()

	// Register core commands
	Register(Command{
		Name:    "help",
		Summary: "Show available commands",
		Aliases: []string{"h", "?"},
	}, helpHandler)

	Register(Command{
		Name:    "status",
		Summary: "Show session status",
	}, statusHandler)

	Register(Command{
		Name:    "exit",
		Summary: "Exit REPL",
		Aliases: []string{"quit", "q"},
	}, exitHandler)

	Register(Command{
		Name:    "clear",
		Summary: "Clear session",
	}, clearHandler)
}

func GetCurrentModel() string {
	return cmdCtx.GetModel()
}

func helpHandler(args []string) error {
	fmt.Println("╭───────────────────────────────────────╮")
	fmt.Println("│    SMARTCODE - Available Commands    │")
	fmt.Println("╰───────────────────────────────────────╯")
	fmt.Println()

	categories := map[string][]struct {
		name string
		desc string
	}{
		"Core": {
			{"help", "Show this help message"},
			{"status", "Show session status"},
			{"exit", "Exit REPL"},
			{"clear", "Clear session"},
		},
		"Model & Config": {
			{"model [name]", "Show or set model"},
			{"model-list", "List available models"},
			{"config", "Show configuration"},
			{"set-api-key <key>", "Set API key"},
		},
		"Session": {
			{"session", "List sessions"},
			{"resume <id>", "Resume session"},
			{"save", "Save session"},
			{"export", "Export session"},
		},
		"Git": {
			{"git-status", "Show git status"},
			{"git-diff", "Show diff"},
			{"git-commit <msg>", "Commit changes"},
			{"git-branch", "List branches"},
		},
		"MCP": {
			{"mcp", "List MCP servers"},
			{"mcp-add <name> <cmd>", "Add MCP server"},
			{"mcp-remove <name>", "Remove MCP server"},
		},
		"Tools": {
			{"tools", "List available tools"},
			{"skills", "List skills"},
			{"agents", "List agents"},
		},
		"Diagnostics": {
			{"doctor", "Run diagnostics"},
			{"cost", "Show token usage"},
			{"compact", "Compact history"},
		},
	}

	for cat, cmds := range categories {
		fmt.Printf("◆ %s\n", cat)
		for _, cmd := range cmds {
			fmt.Printf("  /%-20s %s\n", cmd.name, cmd.desc)
		}
		fmt.Println()
	}

	fmt.Println("Usage: /<command> [arguments]")
	fmt.Println("Example: /model claude-opus-4, /git-status, /cost")
	return nil
}

func statusHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Session Status              │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  Model:     %s\n", cmdCtx.GetModel())
	fmt.Printf("  Runtime:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Work Dir:  %s\n", cmdCtx.WorkDir)
	fmt.Printf("  Time:      %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("  Uptime:    %v\n", time.Since(cmdCtx.StartTime).Round(time.Second))

	if s := cmdCtx.GetSession(); s != nil {
		fmt.Printf("  Session:   %s\n", s.ID)
		fmt.Printf("  Messages:  %d\n", s.MessageCount)
		fmt.Println("  Status:    active")
	} else {
		fmt.Println("  Session:   none")
	}

	input, output, total := cmdCtx.GetTokenStats()
	fmt.Printf("  Tokens:    %d in / %d out / %d total\n", input, output, total)

	return nil
}

func exitHandler(args []string) error {
	fmt.Println("╭─────────────────────╮")
	fmt.Println("│  Goodbye! 👋        │")
	fmt.Println("╰─────────────────────╯")
	return fmt.Errorf("exit")
}

func clearHandler(args []string) error {
	cmdCtx.Session = nil
	cmdCtx.InputTokens = 0
	cmdCtx.OutputTokens = 0
	cmdCtx.TokenCount = 0
	fmt.Println("✓ Session cleared")
	return nil
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}
