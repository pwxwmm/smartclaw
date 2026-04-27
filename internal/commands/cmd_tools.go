package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/instructkr/smartclaw/internal/tools"
)

func init() {
	Register(Command{
		Name:    "tools",
		Summary: "List available tools",
	}, toolsHandler)

	Register(Command{
		Name:    "tasks",
		Summary: "List or manage tasks",
	}, tasksHandler)

	Register(Command{
		Name:    "lsp",
		Summary: "LSP operations",
	}, lspHandler)

	Register(Command{
		Name:    "read",
		Summary: "Read file",
	}, readHandler)

	Register(Command{
		Name:    "write",
		Summary: "Write file",
	}, writeHandler)

	Register(Command{
		Name:    "exec",
		Summary: "Execute command",
	}, execHandler)

	Register(Command{
		Name:    "browse",
		Summary: "Open browser",
	}, browseHandler)

	Register(Command{
		Name:    "web",
		Summary: "Web operations",
	}, webHandler)

	Register(Command{
		Name:    "ide",
		Summary: "IDE integration",
	}, ideHandler)

	Register(Command{
		Name:    "install",
		Summary: "Install package",
	}, installHandler)
}

func toolsHandler(args []string) error {
	fmt.Println("┌─────────────────────────────────────┐")
	fmt.Println("│         Available Tools             │")
	fmt.Println("└─────────────────────────────────────┘")
	fmt.Println()

	toolList := []struct {
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

	for _, t := range toolList {
		fmt.Printf("  %-15s %s\n", t.name, t.desc)
	}

	fmt.Println("\nTotal: 43 tools available")
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

func webHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /web [fetch|search] <args>")
		return nil
	}
	fmt.Printf("Web operation: %s\n", args[0])
	return nil
}

func ideHandler(args []string) error {
	fmt.Println("IDE Integration")
	fmt.Println("  Supported: VS Code, Vim, JetBrains")
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
