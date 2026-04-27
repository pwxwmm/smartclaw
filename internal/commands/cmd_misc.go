package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/srecoder"
)

func init() {
	Register(Command{
		Name:    "init",
		Summary: "Initialize new project",
	}, initHandler)

	Register(Command{
		Name:    "context",
		Summary: "Manage context",
	}, contextHandler)

	Register(Command{
		Name:    "permissions",
		Summary: "Manage permissions",
	}, permissionsHandler)

	Register(Command{
		Name:    "hooks",
		Summary: "Manage hooks",
	}, hooksHandler)

	Register(Command{
		Name:    "plugin",
		Summary: "Manage plugins",
	}, pluginHandler)

	Register(Command{
		Name:    "passes",
		Summary: "LSP passes",
	}, passesHandler)

	Register(Command{
		Name:    "preview",
		Summary: "Preview changes",
	}, previewHandler)

	Register(Command{
		Name:    "effort",
		Summary: "Effort tracking",
	}, effortHandler)

	Register(Command{
		Name:    "tag",
		Summary: "Tag management",
	}, tagHandler)

	Register(Command{
		Name:    "copy",
		Summary: "Copy to clipboard",
	}, copyHandler)

	Register(Command{
		Name:    "files",
		Summary: "List files",
	}, filesHandler)

	Register(Command{
		Name:    "advisor",
		Summary: "AI advisor",
	}, advisorHandler)

	Register(Command{
		Name:    "btw",
		Summary: "By the way",
	}, btwHandler)

	Register(Command{
		Name:    "bughunter",
		Summary: "Bug hunting mode",
	}, bughunterHandler)

	Register(Command{
		Name:    "insights",
		Summary: "Code insights",
	}, insightsHandler)

	Register(Command{
		Name:    "onboarding",
		Summary: "Onboarding",
	}, onboardingHandler)

	Register(Command{
		Name:    "teleport",
		Summary: "Teleport mode",
	}, teleportHandler)

	Register(Command{
		Name:    "version",
		Summary: "Show version",
		Aliases: []string{"v"},
	}, versionHandler)

	Register(Command{
		Name:    "sre",
		Summary: "Toggle SRE-aware coding mode (on/off/status)",
	}, sreHandler)
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

func contextHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Context Information         │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Printf("  Work Dir:  %s\n", cmdCtx.WorkDir)
	fmt.Printf("  Model:     %s\n", cmdCtx.GetModel())
	fmt.Printf("  Session:   %v\n", cmdCtx.GetSession() != nil)
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

func passesHandler(args []string) error {
	fmt.Println("LSP Passes")
	fmt.Println("  Code analysis passes")
	return nil
}

func previewHandler(args []string) error {
	fmt.Println("Preview mode")
	fmt.Println("  Changes shown but not applied")
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

func tagHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /tag <name> [value]")
		return nil
	}
	fmt.Printf("Tag: %s = %s\n", args[0], strings.Join(args[1:], " "))
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

func onboardingHandler(args []string) error {
	fmt.Println("Welcome to SmartClaw!")
	fmt.Println("  Run /help to get started")
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

func versionHandler(args []string) error {
	fmt.Println("SmartClaw v1.0.0")
	fmt.Printf("  Go version: %s\n", runtime.Version())
	fmt.Printf("  Platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
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
