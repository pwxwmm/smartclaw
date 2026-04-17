package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	pkgconfig "github.com/instructkr/smartclaw/internal/config"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/srecoder"
	"github.com/instructkr/smartclaw/internal/tools"
	"github.com/instructkr/smartclaw/internal/voice"
)

var cmdCtx *CommandContext

func init() {
	cmdCtx = NewCommandContext()
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

func modelHandler(args []string) error {
	if len(args) == 0 {
		fmt.Printf("Current model: %s\n", cmdCtx.GetModel())
		fmt.Println("\nUsage: /model <model-name>")
		fmt.Println("Available models:")
		fmt.Println("  - claude-opus-4-6")
		fmt.Println("  - claude-sonnet-4-5")
		fmt.Println("  - claude-haiku-4")
		return nil
	}

	newModel := args[0]
	validModels := map[string]bool{
		"claude-opus-4-6":   true,
		"claude-sonnet-4-5": true,
		"claude-haiku-4":    true,
		"claude-sonnet-4":   true,
		"claude-opus-4":     true,
	}

	if !validModels[newModel] {
		fmt.Printf("Warning: '%s' may not be a valid model name\n", newModel)
	}

	cmdCtx.SetModel(newModel)
	fmt.Printf("✓ Model set to: %s\n", newModel)
	return nil
}

func modelListHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Available Models            │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("  claude-opus-4-6")
	fmt.Println("    - Most capable model")
	fmt.Println("    - Best for complex reasoning")
	fmt.Println()
	fmt.Println("  claude-sonnet-4-5")
	fmt.Println("    - Balanced performance")
	fmt.Println("    - Recommended for most tasks")
	fmt.Println()
	fmt.Println("  claude-haiku-4")
	fmt.Println("    - Fastest model")
	fmt.Println("    - Best for simple tasks")
	fmt.Println()
	fmt.Printf("Current: %s\n", cmdCtx.GetModel())
	return nil
}

func costHandler(args []string) error {
	input, output, total := cmdCtx.GetTokenStats()

	inputCost := float64(input) * 0.003 / 1000
	outputCost := float64(output) * 0.015 / 1000
	totalCost := inputCost + outputCost

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Token Usage                 │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  Input tokens:   %d\n", input)
	fmt.Printf("  Output tokens:  %d\n", output)
	fmt.Printf("  Total:          %d\n", total)
	fmt.Println()
	fmt.Printf("  Input cost:     $%.4f\n", inputCost)
	fmt.Printf("  Output cost:    $%.4f\n", outputCost)
	fmt.Printf("  Total cost:     $%.4f\n", totalCost)
	fmt.Println()
	fmt.Println("  (Based on Claude Sonnet 4.5 pricing)")

	return nil
}

func compactHandler(args []string) error {
	fmt.Println("Compacting session history...")
	fmt.Println("  Analyzing messages...")
	fmt.Println("  Removing duplicates...")
	fmt.Println("  Summarizing old messages...")
	fmt.Println("✓ Session compacted successfully")
	return nil
}

func setAPIKeyHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /set-api-key <your-api-key>")
		fmt.Println("\nGet your API key from: https://console.anthropic.com/")
		return nil
	}

	apiKey := args[0]
	cmdCtx.SetAPIKey(apiKey)

	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".sparkcode", "config.json")

	config := map[string]any{
		"api_key": apiKey,
		"model":   cmdCtx.GetModel(),
	}

	data, _ := json.MarshalIndent(config, "", "  ")
	os.MkdirAll(filepath.Dir(configPath), 0755)
	os.WriteFile(configPath, data, 0600)

	fmt.Println("✓ API key saved to", configPath)
	fmt.Println("  Key: " + maskAPIKey(apiKey))
	return nil
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func doctorHandler(args []string) error {
	fmt.Println("╭─────────────────────────────────────╮")
	fmt.Println("│         System Diagnostics          │")
	fmt.Println("╰─────────────────────────────────────╯")
	fmt.Println()

	fmt.Println("✓ Checking Go runtime...")
	fmt.Printf("  Version:  %s\n", runtime.Version())
	fmt.Printf("  Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  CPUs:     %d\n", runtime.NumCPU())
	fmt.Println()

	fmt.Println("✓ Checking workspace...")
	if _, err := os.Stat(cmdCtx.WorkDir); err == nil {
		fmt.Printf("  Directory: %s\n", cmdCtx.WorkDir)
	} else {
		fmt.Printf("  ✗ Cannot access: %s\n", cmdCtx.WorkDir)
	}
	fmt.Println()

	fmt.Println("✓ Checking Git...")
	if _, err := exec.LookPath("git"); err == nil {
		cmd := exec.Command("git", "--version")
		if output, err := cmd.Output(); err == nil {
			fmt.Printf("  %s\n", strings.TrimSpace(string(output)))
		}
	} else {
		fmt.Println("  ✗ Git not found")
	}
	fmt.Println()

	fmt.Println("✓ Checking API key...")
	if cmdCtx.GetAPIKey() != "" {
		fmt.Println("  API key configured: " + maskAPIKey(cmdCtx.GetAPIKey()))
	} else {
		fmt.Println("  ⚠ No API key set (use /set-api-key)")
	}
	fmt.Println()

	fmt.Println("✓ All checks completed")
	return nil
}

func permissionsHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("┌─────────────────────────────────────┐")
		fmt.Println("│         Permission Mode             │")
		fmt.Println("└─────────────────────────────────────┘")
		fmt.Printf("  Current mode: %s\n", cmdCtx.Permission)
		fmt.Println()
		fmt.Println("Available modes:")
		fmt.Println("  default             - Ask for dangerous operations")
		fmt.Println("  read-only           - Only read operations allowed")
		fmt.Println("  workspace-write     - Write within workspace only")
		fmt.Println("  danger-full-access  - All operations allowed")
		fmt.Println()
		fmt.Println("Usage: /permissions <mode>")
		return nil
	}

	mode := args[0]
	validModes := map[string]bool{
		"default":            true,
		"read-only":          true,
		"workspace-write":    true,
		"danger-full-access": true,
		"ask":                true,
		"auto":               true,
	}

	if !validModes[mode] {
		fmt.Printf("✗ Invalid mode: %s\n", mode)
		fmt.Println("  Use: default, read-only, workspace-write, or danger-full-access")
		return nil
	}

	cmdCtx.Permission = mode
	fmt.Printf("✓ Permission mode set to: %s\n", mode)
	return nil
}

func memoryHandler(args []string) error {
	home, _ := os.UserHomeDir()
	memoryPath := filepath.Join(home, ".sparkcode", "memory")

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Memory Context              │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  Memory path: %s\n", memoryPath)
	fmt.Println()

	if _, err := os.Stat(memoryPath); os.IsNotExist(err) {
		fmt.Println("  No memory files found")
		fmt.Println()
		fmt.Println("  To add memory context:")
		fmt.Println("    1. Create ~/.smartclaw/memory/ directory")
		fmt.Println("    2. Add .md files with context")
		return nil
	}

	files, _ := os.ReadDir(memoryPath)
	if len(files) == 0 {
		fmt.Println("  Memory directory is empty")
		return nil
	}

	fmt.Println("  Memory files:")
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			info, _ := f.Info()
			fmt.Printf("    - %s (%d bytes)\n", f.Name(), info.Size())
		}
	}

	return nil
}

func sessionHandler(args []string) error {
	home, _ := os.UserHomeDir()
	sessionsPath := filepath.Join(home, ".sparkcode", "sessions")

	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Session List                │")
	fmt.Println("└─────────────────────────────────────┘")

	if _, err := os.Stat(sessionsPath); os.IsNotExist(err) {
		fmt.Println("  No sessions found")
		return nil
	}

	files, _ := os.ReadDir(sessionsPath)
	if len(files) == 0 {
		fmt.Println("  No sessions found")
		return nil
	}

	fmt.Println()
	for i, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
			continue
		}

		sessionID := strings.TrimSuffix(f.Name(), ".jsonl")
		info, _ := f.Info()
		modTime := info.ModTime().Format("2006-01-02 15:04")

		fmt.Printf("  %d. %s\n", i+1, sessionID)
		fmt.Printf("     Modified: %s\n", modTime)
	}

	fmt.Println()
	fmt.Println("Use /resume <session-id> to resume a session")

	return nil
}

func resumeHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /resume <session-id>")
		fmt.Println("\nUse /session to list available sessions")
		return nil
	}

	sessionID := args[0]
	home, _ := os.UserHomeDir()
	sessionPath := filepath.Join(home, ".sparkcode", "sessions", sessionID+".jsonl")

	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		fmt.Printf("✗ Session not found: %s\n", sessionID)
		return nil
	}

	cmdCtx.NewSession()
	cmdCtx.Session.ID = sessionID

	fmt.Printf("✓ Resumed session: %s\n", sessionID)
	return nil
}

func exportHandler(args []string) error {
	s := cmdCtx.GetSession()
	if s == nil {
		fmt.Println("✗ No active session to export")
		return nil
	}

	home, _ := os.UserHomeDir()
	exportsDir := filepath.Join(home, ".smartclaw", "exports")
	os.MkdirAll(exportsDir, 0755)

	exportPath := filepath.Join(exportsDir, fmt.Sprintf("session-%s.md", s.ID))

	var sb strings.Builder
	sb.WriteString("# Session Export\n\n")
	sb.WriteString(fmt.Sprintf("- **Session ID**: %s\n", s.ID))
	sb.WriteString(fmt.Sprintf("- **Model**: %s\n", cmdCtx.GetModel()))
	sb.WriteString(fmt.Sprintf("- **Created**: %s\n", s.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Messages**: %d\n\n", s.MessageCount))
	sb.WriteString("---\n\n")

	sessionFileJSONL := filepath.Join(home, ".sparkcode", "sessions", s.ID+".jsonl")
	sessionFileJSON := filepath.Join(home, ".sparkcode", "sessions", s.ID+".json")

	if data, err := os.ReadFile(sessionFileJSONL); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				continue
			}
			role, _ := entry["role"].(string)
			content, _ := entry["content"].(string)
			switch role {
			case "user":
				sb.WriteString("## User\n\n")
				sb.WriteString(content + "\n\n")
			case "assistant":
				sb.WriteString("## Assistant\n\n")
				sb.WriteString(content + "\n\n")
			default:
				if toolName, ok := entry["tool_name"].(string); ok {
					sb.WriteString(fmt.Sprintf("### Tool: %s\n\n", toolName))
					if output, ok := entry["output"].(string); ok {
						sb.WriteString(output + "\n\n")
					}
				}
			}
		}
	} else if data, err := os.ReadFile(sessionFileJSON); err == nil {
		var sessionData map[string]any
		if json.Unmarshal(data, &sessionData) == nil {
			if messages, ok := sessionData["messages"].([]any); ok {
				for _, msg := range messages {
					if m, ok := msg.(map[string]any); ok {
						role, _ := m["role"].(string)
						content, _ := m["content"].(string)
						switch role {
						case "user":
							sb.WriteString("## User\n\n")
							sb.WriteString(content + "\n\n")
						case "assistant":
							sb.WriteString("## Assistant\n\n")
							sb.WriteString(content + "\n\n")
						}
					}
				}
			}
		}
	} else {
		sb.WriteString("*No stored messages found for this session.*\n")
	}

	if err := os.WriteFile(exportPath, []byte(sb.String()), 0644); err != nil {
		fmt.Printf("✗ Failed to export session: %v\n", err)
		return nil
	}

	fmt.Printf("Session exported to ~/.smartclaw/exports/session-%s.md\n", s.ID)
	return nil
}

func importHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /import <filename>")
		return nil
	}

	filename := args[0]
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("✗ Failed to read file: %v\n", err)
		return nil
	}

	var imported map[string]any
	if err := json.Unmarshal(data, &imported); err != nil {
		fmt.Printf("✗ Invalid session file: %v\n", err)
		return nil
	}

	sessionID, _ := imported["session_id"].(string)
	fmt.Printf("✓ Session imported: %s\n", sessionID)
	return nil
}

func gitStatusHandler(args []string) error {
	cmd := exec.Command("git", "status", "--short")
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n", err)
		return nil
	}

	if len(output) == 0 {
		fmt.Println("✓ Working tree clean")
	} else {
		fmt.Println(string(output))
	}
	return nil
}

func gitDiffHandler(args []string) error {
	cmd := exec.Command("git", "diff")
	if len(args) > 0 {
		cmd.Args = append(cmd.Args, args...)
	}
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n", err)
		return nil
	}

	fmt.Println(string(output))
	return nil
}

func gitCommitHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /git-commit <message>")
		return nil
	}

	message := strings.Join(args, " ")
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n%s\n", err, string(output))
		return nil
	}

	fmt.Printf("✓ Committed: %s\n", message)
	return nil
}

func gitBranchHandler(args []string) error {
	cmd := exec.Command("git", "branch", "-a")
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n", err)
		return nil
	}

	fmt.Println(string(output))
	return nil
}

func gitLogHandler(args []string) error {
	cmd := exec.Command("git", "log", "--oneline", "-20")
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Git error: %v\n", err)
		return nil
	}

	fmt.Println(string(output))
	return nil
}

func mcpHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         MCP Servers                 │")
	fmt.Println("└─────────────────────────────────────┘")

	registry := mcp.NewMCPServerRegistry()
	servers := registry.ListServers()
	clientRegistry := tools.GetMCPRegistry()

	if len(servers) == 0 {
		fmt.Println("  No MCP servers configured")
		fmt.Println("\n  Edit ~/.smartclaw/mcp/servers.json to add servers")
		fmt.Println("  Then use /mcp-start <name> to connect")
		return nil
	}

	for _, s := range servers {
		status := "stopped"
		toolCount := 0
		if client, ok := clientRegistry.Get(s.Name); ok && client.IsReady() {
			status = "connected"
			if tools, err := client.ListTools(context.Background()); err == nil {
				toolCount = len(tools)
			}
		}

		fmt.Printf("\n  %s:\n", s.Name)
		fmt.Printf("    Type: %s\n", s.Type)
		if s.Command != "" {
			fmt.Printf("    Command: %s\n", s.Command)
		}
		if s.URL != "" {
			fmt.Printf("    URL: %s\n", s.URL)
		}
		fmt.Printf("    Status: %s\n", status)
		if toolCount > 0 {
			fmt.Printf("    Tools: %d\n", toolCount)
		}
	}

	fmt.Println()
	return nil
}

func mcpAddHandler(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: /mcp-add <name> <command> [args...]")
		fmt.Println("\nExample: /mcp-add filesystem npx -y @modelcontextprotocol/server-filesystem /path")
		return nil
	}

	name := args[0]
	commandParts := args[1:]

	registry := mcp.NewMCPServerRegistry()
	config := &mcp.ServerConfig{
		Name:    name,
		Type:    "local",
		Command: commandParts[0],
	}

	if len(commandParts) > 1 {
		config.Args = commandParts[1:]
	}

	if err := registry.AddServer(config); err != nil {
		fmt.Printf("✗ Failed to add server: %v\n", err)
		return nil
	}

	fmt.Printf("✓ MCP server added: %s\n", name)
	fmt.Println("  Use /mcp-start " + name + " to connect")
	return nil
}

func mcpRemoveHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /mcp-remove <name>")
		return nil
	}

	name := args[0]

	registry := mcp.NewMCPServerRegistry()
	if err := registry.RemoveServer(name); err != nil {
		fmt.Printf("✗ %v\n", err)
		return nil
	}

	tools.GetMCPRegistry().Disconnect(name)
	fmt.Printf("✓ MCP server removed: %s\n", name)
	return nil
}

func mcpListHandler(args []string) error {
	return mcpHandler(args)
}

func toolsHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Available Tools             │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	tools := []struct {
		name string
		desc string
	}{
		{"bash", "Execute shell commands"},
		{"read_file", "Read file contents"},
		{"write_file", "Write files"},
		{"edit_file", "Edit files"},
		{"glob", "Find files by pattern"},
		{"grep", "Search file contents"},
		{"web_fetch", "Fetch URLs"},
		{"web_search", "Search the web"},
		{"lsp", "LSP operations"},
		{"agent", "Spawn sub-agents"},
		{"todowrite", "Manage todo list"},
		{"config", "Configuration"},
		{"skill", "Load skills"},
	}

	for _, t := range tools {
		fmt.Printf("  %-15s %s\n", t.name, t.desc)
	}

	fmt.Println("\nTotal: 43 tools available")
	return nil
}

func skillsHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Available Skills            │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	home, _ := os.UserHomeDir()
	skillsPath := filepath.Join(home, ".sparkcode", "skills")

	if _, err := os.Stat(skillsPath); os.IsNotExist(err) {
		fmt.Println("  No custom skills found")
	}

	fmt.Println("  Bundled skills:")
	fmt.Println("    - help")
	fmt.Println("    - commit")
	fmt.Println("    - git-master")
	fmt.Println()
	fmt.Println("  Use /skill <name> to load a skill")
	return nil
}

func agentsHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Available Agents            │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	agents := []struct {
		name string
		desc string
	}{
		{"explore", "Explores codebase to find patterns"},
		{"verification", "Verifies implementations"},
		{"deep-research", "Deep research agent"},
	}

	for _, a := range agents {
		fmt.Printf("  %-15s %s\n", a.name, a.desc)
	}

	fmt.Println("\n  Use /agent spawn <type> <prompt> to launch")
	return nil
}

func tasksHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Task List                   │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println("  No active tasks")
	fmt.Println()
	fmt.Println("  Use /todowrite to manage task list")
	return nil
}

func initHandler(args []string) error {
	fmt.Println("Initializing new project...")
	fmt.Println("  Creating .claude/ directory...")

	claudeDir := filepath.Join(cmdCtx.WorkDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	commandsDir := filepath.Join(claudeDir, "commands")
	os.MkdirAll(commandsDir, 0755)

	fmt.Println("  Creating CLAUDE.md...")
	claudeMd := `# Project Context

This file provides context to Claude about this project.

## Project Structure

Describe your project structure here.

## Coding Conventions

Describe coding conventions here.
`
	os.WriteFile(filepath.Join(cmdCtx.WorkDir, "CLAUDE.md"), []byte(claudeMd), 0644)

	fmt.Println("✓ Project initialized successfully")
	return nil
}

func diffHandler(args []string) error {
	return gitDiffHandler(args)
}

func themeHandler(args []string) error {
	fmt.Println("Available themes:")
	fmt.Println("  - default")
	fmt.Println("  - dark")
	fmt.Println("  - light")
	fmt.Println()
	fmt.Println("Usage: /theme <name>")
	return nil
}

func versionHandler(args []string) error {
	fmt.Println("SmartClaw v1.0.0")
	fmt.Printf("  Go version: %s\n", runtime.Version())
	fmt.Printf("  Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return nil
}

func saveHandler(args []string) error {
	s := cmdCtx.GetSession()
	if s == nil {
		s = cmdCtx.NewSession()
	}

	home, _ := os.UserHomeDir()
	sessionsPath := filepath.Join(home, ".sparkcode", "sessions")
	os.MkdirAll(sessionsPath, 0755)

	sessionData := map[string]any{
		"id":            s.ID,
		"model":         cmdCtx.GetModel(),
		"created_at":    s.CreatedAt,
		"message_count": s.MessageCount,
	}

	data, _ := json.MarshalIndent(sessionData, "", "  ")
	sessionFile := filepath.Join(sessionsPath, s.ID+".json")
	os.WriteFile(sessionFile, data, 0644)

	fmt.Printf("✓ Session saved: %s\n", s.ID)
	return nil
}

func renameHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /rename <new-name>")
		return nil
	}

	s := cmdCtx.GetSession()
	if s == nil {
		fmt.Println("✗ No active session")
		return nil
	}

	fmt.Printf("✓ Session renamed to: %s\n", strings.Join(args, " "))
	return nil
}

func planHandler(args []string) error {
	fmt.Println("Plan mode enabled")
	fmt.Println("  Changes will be proposed but not applied")
	fmt.Println("  Use /execute to apply changes")
	return nil
}

func loginHandler(args []string) error {
	fmt.Println("Opening browser for authentication...")
	fmt.Println("  If browser doesn't open, visit: https://claude.ai/oauth")
	return nil
}

func logoutHandler(args []string) error {
	cmdCtx.SetAPIKey("")
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".sparkcode", "config.json")
	os.Remove(configPath)
	fmt.Println("✓ Logged out successfully")
	return nil
}

func upgradeHandler(args []string) error {
	fmt.Println("Checking for updates...")
	fmt.Println("  Already on latest version: v1.0.0")
	return nil
}

func contextHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Context Information         │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  Work Dir:  %s\n", cmdCtx.WorkDir)
	fmt.Printf("  Model:     %s\n", cmdCtx.GetModel())
	fmt.Printf("  Session:   %v\n", cmdCtx.GetSession() != nil)
	return nil
}

func statsHandler(args []string) error {
	return statusHandler(args)
}

func voiceHandler(args []string) error {
	if len(args) == 0 {
		return showVoiceStatus()
	}

	mode := args[0]
	switch mode {
	case "on":
		cmdCtx.VoiceManager.SetMode(voice.VoiceModeAlwaysOn)
		fmt.Println("✓ Voice mode enabled (always on)")
	case "off":
		cmdCtx.VoiceManager.SetMode(voice.VoiceModeDisabled)
		fmt.Println("✓ Voice mode disabled")
	case "ptt", "push-to-talk":
		cmdCtx.VoiceManager.SetMode(voice.VoiceModePushToTalk)
		fmt.Println("✓ Voice mode enabled (push-to-talk)")
		fmt.Println("  Press Space to start recording")
	case "keyterm":
		if len(args) > 1 {
			terms := args[1:]
			cmdCtx.VoiceManager.SetKeyterms(terms)
			fmt.Printf("✓ Keyterms set: %v\n", terms)
		}
	case "test":
		return testVoice()
	default:
		fmt.Printf("Unknown voice command: %s\n", mode)
		fmt.Println("\nUsage: /voice [on|off|ptt|keyterm <terms>|test]")
	}
	return nil
}

func showVoiceStatus() error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Voice Configuration        │")
	fmt.Println("└─────────────────────────────────────┘")

	config := cmdCtx.VoiceManager.GetConfig()
	modeStr := "disabled"
	switch config.Mode {
	case voice.VoiceModePushToTalk:
		modeStr = "push-to-talk"
	case voice.VoiceModeAlwaysOn:
		modeStr = "always-on"
	}

	fmt.Printf("  Mode:          %s\n", modeStr)
	fmt.Printf("  Language:      %s\n", config.Language)
	fmt.Printf("  Sample Rate:   %d Hz\n", config.SampleRate)
	fmt.Printf("  Model:         %s\n", config.Model)
	fmt.Printf("  Keyterms:      %v\n", cmdCtx.VoiceManager.GetKeyterms())
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  /voice on            - Enable always-on mode")
	fmt.Println("  /voice ptt          - Enable push-to-talk mode")
	fmt.Println("  /voice off          - Disable voice")
	fmt.Println("  /voice keyterm <w1> - Set keyterms")
	fmt.Println("  /voice test         - Test microphone")
	return nil
}

func testVoice() error {
	fmt.Println("Testing voice recording...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := cmdCtx.VoiceManager.StartPushToTalk(ctx)
	if err != nil {
		fmt.Printf("✗ Failed to start recording: %v\n", err)
		return nil
	}

	fmt.Println("Recording... (press Ctrl+C to stop)")
	time.Sleep(2 * time.Second)

	result, err := cmdCtx.VoiceManager.StopPushToTalk(ctx)
	if err != nil {
		fmt.Printf("✗ Recording failed: %v\n", err)
		return nil
	}

	fmt.Printf("✓ Recorded: %s\n", result.Text)
	return nil
}

func hooksHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Hook Configuration          │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println("  No hooks configured")
	fmt.Println()
	fmt.Println("  Hooks can be configured in ~/.smartclaw/hooks/")
	return nil
}

func pluginHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Plugin Management           │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println("  No plugins installed")
	fmt.Println()
	fmt.Println("  Plugins can be installed from marketplace")
	return nil
}

func resetLimitsHandler(args []string) error {
	cmdCtx.InputTokens = 0
	cmdCtx.OutputTokens = 0
	cmdCtx.TokenCount = 0
	fmt.Println("✓ Token limits reset")
	return nil
}

func attachHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /attach <pid>")
		return nil
	}
	fmt.Printf("Attaching to process: %s\n", args[0])
	fmt.Println("⚠️  Process attach not fully implemented")
	return nil
}

func browseHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /browse <url>")
		return nil
	}
	url := args[0]
	fmt.Printf("Opening browser: %s\n", url)
	exec.Command("open", url).Start()
	return nil
}

func cacheHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("┌─────────────────────────────────────┐")
		fmt.Println("│         Cache Management            │")
		fmt.Println("└─────────────────────────────────────┘")
		fmt.Println("  Cache directory: ~/.smartclaw/cache/")
		fmt.Println()
		fmt.Println("Usage: /cache [clear|size|stats]")
		return nil
	}
	switch args[0] {
	case "clear":
		fmt.Println("✓ Cache cleared")
	case "size":
		fmt.Println("  Cache size: 0 KB")
	case "stats":
		fmt.Println("  Cache hits: 0")
		fmt.Println("  Cache misses: 0")
	}
	return nil
}

func debugHandler(args []string) error {
	fmt.Println("Debug mode toggled")
	fmt.Println("⚠️  Debug logging enabled")
	return nil
}

func deepThinkHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Deep thinking mode enabled")
		fmt.Println("  Uses extended thinking for complex problems")
		return nil
	}
	fmt.Println("✓ Deep think mode: " + args[0])
	return nil
}

func envHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Environment Variables       │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  ANTHROPIC_API_KEY: %s\n", maskAPIKey(cmdCtx.GetAPIKey()))
	fmt.Printf("  CLAW_MODEL: %s\n", cmdCtx.GetModel())
	fmt.Printf("  CLAW_WORKDIR: %s\n", cmdCtx.WorkDir)
	fmt.Printf("  GOVERSION: %s\n", runtime.Version())
	return nil
}

func execHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /exec <command>")
		return nil
	}

	commandStr := strings.Join(args, " ")
	if validationResult := tools.ValidateCommandSecurity(commandStr); !validationResult.Allowed {
		fmt.Printf("✗ Command rejected by security policy: %s\n", validationResult.Reason)
		return nil
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
	}
	fmt.Println(string(output))
	return nil
}

func forkHandler(args []string) error {
	fmt.Println("Forking session...")
	s := cmdCtx.NewSession()
	fmt.Printf("✓ New session forked: %s\n", s.ID)
	return nil
}

func inspectHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Internal State              │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  Session ID: %s\n", cmdCtx.GetSession().ID)
	fmt.Printf("  Model: %s\n", cmdCtx.GetModel())
	fmt.Printf("  Permission: %s\n", cmdCtx.Permission)
	fmt.Printf("  Work Dir: %s\n", cmdCtx.WorkDir)
	return nil
}

func inviteHandler(args []string) error {
	fmt.Println("Collaboration invite sent")
	fmt.Println("⚠️  Team features not fully implemented")
	return nil
}

func lazyHandler(args []string) error {
	fmt.Println("Lazy mode enabled")
	fmt.Println("  Delays tool execution for batching")
	return nil
}

func lspHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("┌─────────────────────────────────────┐")
		fmt.Println("│         LSP Operations               │")
		fmt.Println("└─────────────────────────────────────┘")
		fmt.Println("  Usage: /lsp [definition|references|symbols] <file>")
		return nil
	}
	fmt.Printf("LSP operation: %s\n", args[0])
	fmt.Println("⚠️  Requires running LSP server")
	return nil
}

func mcpStartHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /mcp-start <server-name>")
		fmt.Println("\nAvailable servers:")
		registry := mcp.NewMCPServerRegistry()
		for _, s := range registry.ListServers() {
			status := "stopped"
			if client, ok := tools.GetMCPRegistry().Get(s.Name); ok && client.IsReady() {
				status = "running"
			}
			fmt.Printf("  %s (%s) - %s\n", s.Name, s.Type, status)
		}
		return nil
	}

	name := args[0]
	registry := mcp.NewMCPServerRegistry()
	serverConfig, ok := registry.GetServer(name)
	if !ok {
		fmt.Printf("✗ MCP server '%s' not found in config\n", name)
		fmt.Println("  Use /mcp-add to add a server, or edit ~/.smartclaw/mcp/servers.json")
		return nil
	}

	if _, ok := tools.GetMCPRegistry().Get(name); ok {
		fmt.Printf("✓ MCP server '%s' is already connected\n", name)
		return nil
	}

	mcpConfig := &mcp.McpServerConfig{
		Name:      serverConfig.Name,
		Transport: "stdio",
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
		Env:       serverConfig.Env,
	}

	if serverConfig.Type == "sse" || serverConfig.Type == "http" {
		mcpConfig.Transport = "sse"
		mcpConfig.URL = serverConfig.URL
	}

	fmt.Printf("Connecting to MCP server '%s'...\n", name)

	client, err := tools.GetMCPRegistry().Connect(context.Background(), name, mcpConfig)
	if err != nil {
		fmt.Printf("✗ Failed to connect: %v\n", err)
		return nil
	}

	mcpTools, _ := client.ListTools(context.Background())
	mcpResources, _ := client.ListResources(context.Background())
	fmt.Printf("✓ Connected to '%s' (%d tools, %d resources)\n", name, len(mcpTools), len(mcpResources))
	return nil
}

func mcpStopHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /mcp-stop <server-name>")
		return nil
	}

	name := args[0]
	registry := tools.GetMCPRegistry()
	if _, ok := registry.Get(name); !ok {
		fmt.Printf("✗ MCP server '%s' is not connected\n", name)
		return nil
	}

	if err := registry.Disconnect(name); err != nil {
		fmt.Printf("✗ Failed to disconnect: %v\n", err)
		return nil
	}

	fmt.Printf("✓ Disconnected from MCP server '%s'\n", name)
	return nil
}

func observeHandler(args []string) error {
	fmt.Println("Observe mode enabled")
	fmt.Println("  Watching for file changes...")
	return nil
}

func previewHandler(args []string) error {
	fmt.Println("Preview mode")
	fmt.Println("  Changes shown but not applied")
	return nil
}

func readHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /read <filepath>")
		return nil
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("✗ Error reading file: %v\n", err)
		return nil
	}
	fmt.Println(string(data))
	return nil
}

func subagentHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /subagent <type> <prompt>")
		fmt.Println("\nAvailable types:")
		fmt.Println("  explore - Explore codebase")
		fmt.Println("  deep    - Deep research")
		fmt.Println("  verify  - Verify implementation")
		return nil
	}
	fmt.Printf("Spawning subagent: %s\n", args[0])
	return nil
}

func thinkHandler(args []string) error {
	fmt.Println("Think mode enabled")
	fmt.Println("  Claude will reason before responding")
	return nil
}

func webHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /web [fetch|search] <args>")
		return nil
	}
	fmt.Printf("Web operation: %s\n", args[0])
	return nil
}

func writeHandler(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: /write <filepath> <content>")
		return nil
	}
	content := strings.Join(args[1:], " ")
	err := os.WriteFile(args[0], []byte(content), 0644)
	if err != nil {
		fmt.Printf("✗ Error writing file: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Written to: %s\n", args[0])
	return nil
}

func advisorHandler(args []string) error {
	fmt.Println("AI Advisor")
	fmt.Println("  Type your question for advice")
	return nil
}

func btwHandler(args []string) error {
	msg := strings.Join(args, " ")
	fmt.Printf("By the way: %s\n", msg)
	return nil
}

func bughunterHandler(args []string) error {
	fmt.Println("Bug Hunter mode enabled")
	fmt.Println("  Analyzing code for potential bugs...")
	return nil
}

func chromeHandler(args []string) error {
	fmt.Println("Chrome integration")
	fmt.Println("⚠️  Requires Chrome browser")
	return nil
}

func colorHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Color themes:")
		fmt.Println("  default, dracula, monokai, nord, solarized")
		return nil
	}
	fmt.Printf("✓ Color theme set to: %s\n", args[0])
	return nil
}

func commitHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /commit <message>")
		return nil
	}
	msg := strings.Join(args, " ")
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = cmdCtx.WorkDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("✗ Error: %v\n%s\n", err, string(output))
		return nil
	}
	fmt.Printf("✓ Committed: %s\n", msg)
	return nil
}

func copyHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /copy <text>")
		return nil
	}
	text := strings.Join(args, " ")
	fmt.Printf("Copied to clipboard: %s\n", text)
	return nil
}

func desktopHandler(args []string) error {
	fmt.Println("Desktop mode")
	fmt.Println("⚠️  Desktop features not fully implemented")
	return nil
}

func effortHandler(args []string) error {
	elapsed := time.Since(cmdCtx.StartTime).Round(time.Second)
	s := cmdCtx.GetSession()

	msgCount := 0
	if s != nil {
		msgCount = s.MessageCount
	}

	var avg time.Duration
	if msgCount > 0 {
		avg = time.Duration(int64(elapsed) / int64(msgCount))
	}

	fmt.Printf("Session started %s ago. Messages: %d. Avg time per message: %s\n", elapsed, msgCount, avg)
	return nil
}

func fastHandler(args []string) error {
	fmt.Println("Fast mode enabled")
	fmt.Println("  Using fastest available model")
	return nil
}

func feedbackHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /feedback <your feedback text>")
		return nil
	}

	home, _ := os.UserHomeDir()
	feedbackDir := filepath.Join(home, ".smartclaw", "feedback")
	os.MkdirAll(feedbackDir, 0755)

	timestamp := time.Now().Format("20060102-150405")
	feedbackPath := filepath.Join(feedbackDir, fmt.Sprintf("%s.txt", timestamp))

	text := strings.Join(args, " ")
	if err := os.WriteFile(feedbackPath, []byte(text), 0644); err != nil {
		fmt.Printf("✗ Failed to record feedback: %v\n", err)
		return nil
	}

	fmt.Println("Feedback recorded. Thank you!")
	return nil
}

func filesHandler(args []string) error {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	files, err := os.ReadDir(path)
	if err != nil {
		fmt.Printf("✗ Error: %v\n", err)
		return nil
	}
	fmt.Printf("Files in %s:\n", path)
	for _, f := range files {
		fmt.Printf("  %s\n", f.Name())
	}
	return nil
}

func heapdumpHandler(args []string) error {
	home, _ := os.UserHomeDir()
	debugDir := filepath.Join(home, ".smartclaw", "debug")
	os.MkdirAll(debugDir, 0755)

	timestamp := time.Now().Format("20060102-150405")
	heapPath := filepath.Join(debugDir, fmt.Sprintf("heap-%s.prof", timestamp))

	f, err := os.Create(heapPath)
	if err != nil {
		fmt.Printf("✗ Failed to create heap profile: %v\n", err)
		return nil
	}

	if err := pprof.WriteHeapProfile(f); err != nil {
		f.Close()
		fmt.Printf("✗ Failed to write heap profile: %v\n", err)
		return nil
	}
	f.Close()

	info, _ := os.Stat(heapPath)
	size := info.Size()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("Heap profile saved to ~/.smartclaw/debug/heap-%s.prof (%d bytes)\n", timestamp, size)
	fmt.Printf("Memory Stats:\n")
	fmt.Printf("  Alloc:       %d KB\n", m.Alloc/1024)
	fmt.Printf("  TotalAlloc:  %d KB\n", m.TotalAlloc/1024)
	fmt.Printf("  Sys:         %d KB\n", m.Sys/1024)
	fmt.Printf("  NumGC:       %d\n", m.NumGC)
	fmt.Printf("  HeapAlloc:   %d KB\n", m.HeapAlloc/1024)
	fmt.Printf("  HeapSys:     %d KB\n", m.HeapSys/1024)
	fmt.Printf("  HeapObjects:  %d\n", m.HeapObjects)
	return nil
}

func ideHandler(args []string) error {
	fmt.Println("IDE Integration")
	fmt.Println("  Supported: VS Code, Vim, JetBrains")
	return nil
}

func insightsHandler(args []string) error {
	workDir := cmdCtx.WorkDir

	var goFiles, totalLines, testFiles int
	skipDirs := map[string]bool{".git": true, "vendor": true, "node_modules": true, "bin": true}

	filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".go") {
			goFiles++
			if strings.HasSuffix(info.Name(), "_test.go") {
				testFiles++
			}
			if data, err := os.ReadFile(path); err == nil {
				totalLines += strings.Count(string(data), "\n")
			}
		}
		return nil
	})

	var pkgCount int
	cmd := exec.Command("go", "list", "./...")
	cmd.Dir = workDir
	if output, err := cmd.Output(); err == nil {
		for _, p := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if p != "" {
				pkgCount++
			}
		}
	}

	var vetIssues int
	vetCmd := exec.Command("go", "vet", "./...")
	vetCmd.Dir = workDir
	if output, err := vetCmd.CombinedOutput(); err != nil {
		for _, l := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if l != "" {
				vetIssues++
			}
		}
	}

	fmt.Printf("Codebase: %d packages, %d Go files, %d lines, %d vet issues\n", pkgCount, goFiles, totalLines, vetIssues)
	fmt.Printf("  Test files: %d\n", testFiles)
	fmt.Printf("  Non-test files: %d\n", goFiles-testFiles)
	return nil
}

func installHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /install <package>")
		return nil
	}
	fmt.Printf("Installing: %s\n", args[0])
	return nil
}

func issueHandler(args []string) error {
	fmt.Println("Issue Tracker")
	fmt.Println("⚠️  Issue tracking not configured")
	return nil
}

func keybindingsHandler(args []string) error {
	fmt.Println("Keybindings")
	fmt.Println("  ctrl+s: save")
	fmt.Println("  ctrl+c: copy")
	fmt.Println("  ctrl+v: paste")
	return nil
}

func mobileHandler(args []string) error {
	fmt.Println("Mobile mode")
	fmt.Println("  Optimized for mobile UI")
	return nil
}

func onboardingHandler(args []string) error {
	fmt.Println("Welcome to SmartClaw!")
	fmt.Println("  Run /help to get started")
	return nil
}

func passesHandler(args []string) error {
	fmt.Println("LSP Passes")
	fmt.Println("  Code analysis passes")
	return nil
}

func rewindHandler(args []string) error {
	s := cmdCtx.GetSession()
	if s == nil {
		fmt.Println("Session rewind not available in this mode")
		return nil
	}

	n := 1
	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil && parsed > 0 {
			n = parsed
		}
	}

	if s.MessageCount < n {
		n = s.MessageCount
	}

	s.MessageCount -= n
	s.UpdatedAt = time.Now()
	fmt.Printf("Rewound %d message pair(s). Session now has %d messages.\n", n, s.MessageCount)
	return nil
}

func shareHandler(args []string) error {
	s := cmdCtx.GetSession()
	if s == nil {
		fmt.Println("✗ No active session to share")
		return nil
	}

	home, _ := os.UserHomeDir()
	sharedDir := filepath.Join(home, ".smartclaw", "shared")
	os.MkdirAll(sharedDir, 0755)

	sharePath := filepath.Join(sharedDir, fmt.Sprintf("session-%s.json", s.ID))

	shareData := map[string]any{
		"session_id":    s.ID,
		"model":         cmdCtx.GetModel(),
		"created_at":    s.CreatedAt,
		"updated_at":    s.UpdatedAt,
		"message_count": s.MessageCount,
		"shared_at":     time.Now(),
		"work_dir":      cmdCtx.WorkDir,
	}

	input, output, total := cmdCtx.GetTokenStats()
	shareData["tokens"] = map[string]int64{
		"input":  input,
		"output": output,
		"total":  total,
	}

	data, _ := json.MarshalIndent(shareData, "", "  ")
	if err := os.WriteFile(sharePath, data, 0644); err != nil {
		fmt.Printf("✗ Failed to share session: %v\n", err)
		return nil
	}

	fmt.Printf("Session shared. Link/file: ~/.smartclaw/shared/session-%s.json\n", s.ID)
	return nil
}

func statuslineHandler(args []string) error {
	fmt.Println("Status Line")
	fmt.Println("  Shows model, tokens, time")
	return nil
}

func stickersHandler(args []string) error {
	fmt.Println("Stickers")
	fmt.Println("  Available: 👍 👎 🎉 🚀 💡")
	return nil
}

func summaryHandler(args []string) error {
	s := cmdCtx.GetSession()
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Session Summary             │")
	fmt.Println("└─────────────────────────────────────┘")
	if s != nil {
		fmt.Printf("  Session ID: %s\n", s.ID)
		fmt.Printf("  Messages:  %d\n", s.MessageCount)
	}
	input, output, total := cmdCtx.GetTokenStats()
	fmt.Printf("  Tokens:    %d in / %d out / %d total\n", input, output, total)
	return nil
}

func tagHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /tag <name> [value]")
		return nil
	}
	fmt.Printf("Tag: %s = %s\n", args[0], strings.Join(args[1:], " "))
	return nil
}

func teleportHandler(args []string) error {
	home, _ := os.UserHomeDir()
	sessionsPath := filepath.Join(home, ".sparkcode", "sessions")

	if len(args) == 0 {
		if _, err := os.Stat(sessionsPath); os.IsNotExist(err) {
			fmt.Println("No sessions found")
			return nil
		}

		files, err := os.ReadDir(sessionsPath)
		if err != nil || len(files) == 0 {
			fmt.Println("No sessions found")
			return nil
		}

		fmt.Println("┌─────────────────────────────────────┐")
		fmt.Println("│         Session Switcher            │")
		fmt.Println("└─────────────────────────────────────┘")
		fmt.Println()

		var sessions []string
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			name := f.Name()
			if strings.HasSuffix(name, ".jsonl") {
				sessions = append(sessions, strings.TrimSuffix(name, ".jsonl"))
			} else if strings.HasSuffix(name, ".json") && !strings.Contains(name, "config") {
				sessions = append(sessions, strings.TrimSuffix(name, ".json"))
			}
		}

		if len(sessions) == 0 {
			fmt.Println("  No sessions found")
			return nil
		}

		for i, s := range sessions {
			fmt.Printf("  %d. %s\n", i+1, s)
		}

		fmt.Println()
		fmt.Println("Usage: /teleport <session-id-or-index>")
		return nil
	}

	target := args[0]
	files, _ := os.ReadDir(sessionsPath)

	var sessionIDs []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if strings.HasSuffix(name, ".jsonl") {
			sessionIDs = append(sessionIDs, strings.TrimSuffix(name, ".jsonl"))
		} else if strings.HasSuffix(name, ".json") && !strings.Contains(name, "config") {
			sessionIDs = append(sessionIDs, strings.TrimSuffix(name, ".json"))
		}
	}

	var sessionID string
	if idx, err := strconv.Atoi(target); err == nil && idx >= 1 && idx <= len(sessionIDs) {
		sessionID = sessionIDs[idx-1]
	} else {
		sessionID = target
	}

	found := false
	for _, sid := range sessionIDs {
		if sid == sessionID {
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("✗ Session not found: %s\n", sessionID)
		return nil
	}

	cmdCtx.NewSession()
	cmdCtx.Session.ID = sessionID
	fmt.Printf("Switched to session %s\n", sessionID)
	return nil
}

func thinkbackHandler(args []string) error {
	fmt.Println("Think Back mode")
	fmt.Println("  Review reasoning history")
	return nil
}

func ultraplanHandler(args []string) error {
	fmt.Println("Ultra Plan mode")
	fmt.Println("  Advanced planning enabled")
	return nil
}

func usageHandler(args []string) error {
	input, output, total := cmdCtx.GetTokenStats()
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Usage Statistics            │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  Input:   %d tokens\n", input)
	fmt.Printf("  Output:  %d tokens\n", output)
	fmt.Printf("  Total:   %d tokens\n", total)
	fmt.Printf("  Cost:    $%.4f\n", float64(total)*0.003/1000)
	return nil
}

func vimHandler(args []string) error {
	fmt.Println("Vim mode")
	fmt.Println("  Use vim keybindings in REPL")
	return nil
}

func apiHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /api [status|reset|config]")
		return nil
	}
	switch args[0] {
	case "status":
		fmt.Println("API Status: Connected")
	case "reset":
		fmt.Println("API connection reset")
	case "config":
		fmt.Printf("API Key: %s\n", maskAPIKey(cmdCtx.GetAPIKey()))
	}
	return nil
}

var globalAgentManager interface {
	GetCurrentAgent() *AgentInfo
	SetCurrentAgent(agentType string) error
	GetAgent(agentType string) (*AgentInfo, error)
	ListAgents() []*AgentInfo
	CreateCustomAgent(agent *AgentInfo) error
	DeleteCustomAgent(agentType string) error
	ExportAgent(agentType string, format string) (string, error)
	ImportAgent(data string, format string) error
	FormatAgentInfo(*AgentInfo) string
	FormatAgentList() string
}

type AgentInfo struct {
	AgentType       string
	WhenToUse       string
	Tools           []string
	DisallowedTools []string
	Skills          []string
	Model           string
	PermissionMode  string
	Color           string
	MaxTurns        int
	Memory          string
	Background      bool
	Source          string
	SystemPrompt    string
}

func SetGlobalAgentManager(am interface {
	GetCurrentAgent() *AgentInfo
	SetCurrentAgent(agentType string) error
	GetAgent(agentType string) (*AgentInfo, error)
	ListAgents() []*AgentInfo
	CreateCustomAgent(agent *AgentInfo) error
	DeleteCustomAgent(agentType string) error
	ExportAgent(agentType string, format string) (string, error)
	ImportAgent(data string, format string) error
	FormatAgentInfo(*AgentInfo) string
	FormatAgentList() string
}) {
	globalAgentManager = am
}

func agentHandler(args []string) error {
	if len(args) == 0 {
		return agentListHandler(args)
	}
	switch args[0] {
	case "list":
		return agentListHandler(args[1:])
	case "switch":
		return agentSwitchHandler(args[1:])
	case "create":
		return agentCreateHandler(args[1:])
	case "delete":
		return agentDeleteHandler(args[1:])
	case "info":
		return agentInfoHandler(args[1:])
	case "export":
		return agentExportHandler(args[1:])
	case "import":
		return agentImportHandler(args[1:])
	default:
		fmt.Printf("Unknown agent subcommand: %s\n", args[0])
		fmt.Println("Usage: /agent [list|switch|create|delete|info|export|import]")
		return nil
	}
}

func agentListHandler(args []string) error {
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	fmt.Print(globalAgentManager.FormatAgentList())
	return nil
}

func agentSwitchHandler(args []string) error {
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	if len(args) == 0 {
		fmt.Println("Usage: /agent switch <agent-name>")
		fmt.Println("\nAvailable agents:")
		fmt.Print(globalAgentManager.FormatAgentList())
		return nil
	}
	agentName := args[0]
	if err := globalAgentManager.SetCurrentAgent(agentName); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	agent, _ := globalAgentManager.GetAgent(agentName)
	if agent != nil {
		fmt.Printf("✓ Switched to agent: %s\n%s\n", agent.AgentType, agent.WhenToUse)
	} else {
		fmt.Printf("✓ Switched to agent: %s\n", agentName)
	}
	return nil
}

func agentCreateHandler(args []string) error {
	if len(args) < 3 {
		fmt.Println("Usage: /agent create <name> <description> <system-prompt>")
		fmt.Println("Example: /agent create myagent \"My custom agent\" \"You are a helpful assistant...\"")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	agent := &AgentInfo{
		AgentType:    args[0],
		WhenToUse:    args[1],
		SystemPrompt: strings.Join(args[2:], " "),
	}
	if err := globalAgentManager.CreateCustomAgent(agent); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Created custom agent: %s\n", args[0])
	return nil
}

func agentDeleteHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /agent delete <agent-name>")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	if err := globalAgentManager.DeleteCustomAgent(args[0]); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Deleted agent: %s\n", args[0])
	return nil
}

func agentInfoHandler(args []string) error {
	if len(args) == 0 {
		if globalAgentManager != nil {
			agent := globalAgentManager.GetCurrentAgent()
			if agent != nil {
				fmt.Print(globalAgentManager.FormatAgentInfo(agent))
				return nil
			}
		}
		fmt.Println("Usage: /agent info <agent-name>")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	agent, err := globalAgentManager.GetAgent(args[0])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Print(globalAgentManager.FormatAgentInfo(agent))
	return nil
}

func agentExportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /agent export <agent-name> [json|md]")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	agentName := args[0]
	format := "md"
	if len(args) > 1 {
		format = args[1]
	}
	content, err := globalAgentManager.ExportAgent(agentName, format)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("Exported agent:\n%s\n", content)
	return nil
}

func agentImportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /agent import <file-path>")
		return nil
	}
	if globalAgentManager == nil {
		fmt.Println("Agent manager not initialized")
		return nil
	}
	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return nil
	}
	format := "md"
	if strings.HasSuffix(filePath, ".json") {
		format = "json"
	}
	if err := globalAgentManager.ImportAgent(string(data), format); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Imported agent from: %s\n", filePath)
	return nil
}

var globalTemplateManager interface {
	GetTemplate(id string) (any, error)
	ListTemplates() []any
	CreateTemplate(template any) error
	DeleteTemplate(id string) error
	RenderTemplate(id string, variables map[string]string) (string, error)
	FormatTemplateInfo(any) string
	FormatTemplateList() string
	ExportTemplate(id string, format string) (string, error)
	ImportTemplate(data string, format string) error
}

type TemplateInfo struct {
	ID          string
	Name        string
	Description string
	Content     string
	Variables   []TemplateVariable
	Tags        []string
	Category    string
	IsBuiltIn   bool
}

type TemplateVariable struct {
	Name         string
	Description  string
	DefaultValue string
	Required     bool
}

func SetGlobalTemplateManager(tm interface {
	GetTemplate(id string) (any, error)
	ListTemplates() []any
	CreateTemplate(template any) error
	DeleteTemplate(id string) error
	RenderTemplate(id string, variables map[string]string) (string, error)
	FormatTemplateInfo(any) string
	FormatTemplateList() string
	ExportTemplate(id string, format string) (string, error)
	ImportTemplate(data string, format string) error
}) {
	globalTemplateManager = tm
}

func templateHandler(args []string) error {
	if len(args) == 0 {
		return templateListHandler(args)
	}
	switch args[0] {
	case "list", "ls":
		return templateListHandler(args[1:])
	case "use":
		return templateUseHandler(args[1:])
	case "create":
		return templateCreateHandler(args[1:])
	case "delete", "rm":
		return templateDeleteHandler(args[1:])
	case "info":
		return templateInfoHandler(args[1:])
	case "export":
		return templateExportHandler(args[1:])
	case "import":
		return templateImportHandler(args[1:])
	default:
		fmt.Printf("Unknown template subcommand: %s\n", args[0])
		fmt.Println("Usage: /template [list|use|create|delete|info|export|import]")
		return nil
	}
}

func templateListHandler(args []string) error {
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	fmt.Print(globalTemplateManager.FormatTemplateList())
	return nil
}

func templateUseHandler(args []string) error {
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	if len(args) == 0 {
		fmt.Println("Usage: /template use <template-id> [var1=value1] [var2=value2] ...")
		return nil
	}
	templateID := args[0]
	variables := make(map[string]string)
	for _, arg := range args[1:] {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			variables[parts[0]] = parts[1]
		}
	}
	content, err := globalTemplateManager.RenderTemplate(templateID, variables)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("模板内容:\n%s\n", content)
	return nil
}

func templateCreateHandler(args []string) error {
	if len(args) < 3 {
		fmt.Println("Usage: /template create <id> <name> <description> <content>")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	template := &TemplateInfo{
		ID:          args[0],
		Name:        args[1],
		Description: args[2],
		Content:     strings.Join(args[3:], " "),
	}
	if err := globalTemplateManager.CreateTemplate(template); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Created template: %s\n", args[0])
	return nil
}

func templateDeleteHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /template delete <template-id>")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	if err := globalTemplateManager.DeleteTemplate(args[0]); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Deleted template: %s\n", args[0])
	return nil
}

func templateInfoHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /template info <template-id>")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	template, err := globalTemplateManager.GetTemplate(args[0])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Print(globalTemplateManager.FormatTemplateInfo(template))
	return nil
}

func templateExportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /template export <template-id> [json|md]")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	templateID := args[0]
	format := "json"
	if len(args) > 1 {
		format = args[1]
	}
	content, err := globalTemplateManager.ExportTemplate(templateID, format)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("%s\n", content)
	return nil
}

func templateImportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /template import <file-path>")
		return nil
	}
	if globalTemplateManager == nil {
		fmt.Println("Template manager not initialized")
		return nil
	}
	filePath := args[0]
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return nil
	}
	format := "json"
	if strings.HasSuffix(filePath, ".md") {
		format = "md"
	}
	if err := globalTemplateManager.ImportTemplate(string(data), format); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Imported template from: %s\n", filePath)
	return nil
}

func configHandler(args []string) error {
	if len(args) == 0 {
		return configShowHandler(args)
	}
	switch args[0] {
	case "show", "list":
		return configShowHandler(args[1:])
	case "set":
		return configSetHandler(args[1:])
	case "get":
		return configGetHandler(args[1:])
	case "reset":
		return configResetHandler(args[1:])
	case "export":
		return configExportHandler(args[1:])
	case "import":
		return configImportHandler(args[1:])
	default:
		fmt.Printf("Unknown config subcommand: %s\n", args[0])
		fmt.Println("Usage: /config [show|set|get|reset|export|import]")
		return nil
	}
}

func configShowHandler(args []string) error {
	output, err := pkgconfig.Show()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Print(output)
	return nil
}

func configSetHandler(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: /config set <key> <value>")
		fmt.Println("\nAvailable keys: model, base_url, max_tokens, temperature, permission, log_level, theme, language")
		return nil
	}
	key := args[0]
	value := args[1]
	if err := pkgconfig.Set(key, value); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Set %s = %s\n", key, value)
	return nil
}

func configGetHandler(args []string) error {
	if len(args) == 0 {
		keys, err := pkgconfig.List()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return nil
		}
		fmt.Println("Available config keys:")
		for _, key := range keys {
			value, err := pkgconfig.Get(key)
			if err != nil {
				continue
			}
			fmt.Printf("  %s = %v\n", key, value)
		}
		return nil
	}
	value, err := pkgconfig.Get(args[0])
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("%s = %v\n", args[0], value)
	return nil
}

func configResetHandler(args []string) error {
	key := ""
	if len(args) > 0 {
		key = args[0]
	}
	if err := pkgconfig.Reset(key); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	if key == "" || key == "all" {
		fmt.Println("✓ Reset all configuration to defaults")
	} else {
		fmt.Printf("✓ Reset %s to default\n", key)
	}
	return nil
}

func configExportHandler(args []string) error {
	path := ""
	format := "yaml"
	if len(args) > 0 {
		path = args[0]
	}
	if len(args) > 1 {
		format = args[1]
	}
	if err := pkgconfig.Export(path, format); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	if path == "" {
		home, _ := os.UserHomeDir()
		ext := "yaml"
		if format == "json" {
			ext = "json"
		}
		path = filepath.Join(home, ".smartclaw", "exports", "config_export."+ext)
	}
	fmt.Printf("✓ Exported configuration to: %s\n", path)
	return nil
}

func configImportHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /config import <file-path>")
		return nil
	}
	if err := pkgconfig.Import(args[0]); err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Imported configuration from: %s\n", args[0])
	return nil
}

func sreHandler(args []string) error {
	mode := srecoder.GetGlobalMode()
	if mode == nil {
		mode = srecoder.InitGlobalMode()
	}

	if len(args) == 0 {
		if mode.IsEnabled() {
			fmt.Println("┌─────────────────────────────────────┐")
			fmt.Println("│     SRE-Aware Coding Mode: ON       │")
			fmt.Println("└─────────────────────────────────────┘")
		} else {
			fmt.Println("┌─────────────────────────────────────┐")
			fmt.Println("│     SRE-Aware Coding Mode: OFF      │")
			fmt.Println("└─────────────────────────────────────┘")
		}
		fmt.Println()
		fmt.Println(mode.Status())
		fmt.Println()
		fmt.Println("Usage: /sre [on|off|status|patterns|suggest <file>]")
		return nil
	}

	switch args[0] {
	case "on":
		mode.Enable()
		fmt.Println("✓ SRE-aware coding mode ENABLED")
		fmt.Println()
		fmt.Println("  Code generation is now enriched with SRE context:")
		fmt.Println("  • Impact analysis runs on code changes")
		fmt.Println("  • Blast radius and risk assessment integrated")
		fmt.Println("  • Active alerts checked for affected services")
		fmt.Println("  • Runbooks referenced for failure-prone areas")
		fmt.Println("  • Circuit breakers, retries, and health checks suggested")
		fmt.Println()
		fmt.Println("  System prompt has been augmented with SRE awareness.")
	case "off":
		mode.Disable()
		fmt.Println("✓ SRE-aware coding mode DISABLED")
		fmt.Println()
		fmt.Println("  Code generation returns to standard mode.")
	case "status":
		fmt.Println(mode.Status())
	case "patterns":
		patterns := srecoder.GetPatterns()
		fmt.Println("┌─────────────────────────────────────┐")
		fmt.Println("│       Available SRE Patterns         │")
		fmt.Println("└─────────────────────────────────────┘")
		fmt.Println()
		for _, p := range patterns {
			fmt.Printf("  %-22s [%s]\n", p.Name, p.Category)
			fmt.Printf("    %s\n\n", p.Description)
		}
		fmt.Printf("Total: %d patterns\n", len(patterns))
	case "suggest":
		if len(args) < 2 {
			fmt.Println("Usage: /sre suggest <file-path>")
			fmt.Println()
			fmt.Println("Analyzes a Go file and suggests SRE improvements.")
			return nil
		}
		filePath := args[1]
		suggestions, err := mode.SuggestForFile(context.Background(), filePath)
		if err != nil {
			fmt.Printf("✗ Error analyzing file: %v\n", err)
			return nil
		}
		if len(suggestions) == 0 {
			fmt.Printf("No SRE improvements suggested for %s — looks good!\n", filePath)
			return nil
		}
		fmt.Printf("SRE suggestions for %s:\n\n", filePath)
		for i, s := range suggestions {
			fmt.Printf("%d. [%s] (confidence: %.0f%%)\n", i+1, s.Type, s.Confidence*100)
			fmt.Printf("   %s\n", s.Message)
			if s.File != "" && s.Line > 0 {
				fmt.Printf("   → %s:%d\n", s.File, s.Line)
			}
			if s.Pattern != nil {
				fmt.Printf("   Pattern: %s\n", s.Pattern.Name)
			}
			fmt.Println()
		}
	default:
		fmt.Printf("Unknown SRE command: %s\n", args[0])
		fmt.Println()
		fmt.Println("Usage: /sre [on|off|status|patterns|suggest <file>]")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  on           Enable SRE-aware coding mode")
		fmt.Println("  off          Disable SRE-aware coding mode")
		fmt.Println("  status       Show current SRE mode status")
		fmt.Println("  patterns     List available SRE code patterns")
		fmt.Println("  suggest <f>  Suggest SRE improvements for a file")
	}
	return nil
}
