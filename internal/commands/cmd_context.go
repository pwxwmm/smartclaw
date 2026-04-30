package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/instructkr/smartclaw/internal/contextmgr"
)

func init() {
	Register(Command{
		Name:    "context",
		Summary: "View and manage project context (AGENTS.md, SOUL.md, etc.)",
	}, cmdContextHandler)
}

type ContextFileInfo struct {
	Path     string
	Scope    string
	Loaded   bool
	Size     int
	Priority int
}

var globalContextManager interface {
	ListContextFiles() []ContextFileInfo
	GetContextByScope(scope string) (string, error)
	ReloadContext() error
	GetMergedAgentsMD() string
	GetActiveContextSummary() string
}

func SetGlobalContextManager(cm interface {
	ListContextFiles() []ContextFileInfo
	GetContextByScope(scope string) (string, error)
	ReloadContext() error
	GetMergedAgentsMD() string
	GetActiveContextSummary() string
}) {
	globalContextManager = cm
}

type AgentsMDContextAdapter struct {
	hierarchy *contextmgr.AgentsMDHierarchy
}

func NewAgentsMDContextAdapter(h *contextmgr.AgentsMDHierarchy) *AgentsMDContextAdapter {
	return &AgentsMDContextAdapter{hierarchy: h}
}

func (a *AgentsMDContextAdapter) ListContextFiles() []ContextFileInfo {
	if a.hierarchy == nil {
		return nil
	}
	files := a.hierarchy.List()
	result := make([]ContextFileInfo, 0, len(files))
	scopePriority := map[contextmgr.AgentsMDScope]int{
		contextmgr.ScopeProject:   1,
		contextmgr.ScopeDirectory: 2,
		contextmgr.ScopeUser:      3,
		contextmgr.ScopeWorkspace: 4,
	}
	for _, f := range files {
		pri := scopePriority[f.Scope]
		if pri == 0 {
			pri = 99
		}
		result = append(result, ContextFileInfo{
			Path:     f.Path,
			Scope:    string(f.Scope),
			Loaded:   f.Loaded,
			Size:     f.Size,
			Priority: pri,
		})
	}
	return result
}

func (a *AgentsMDContextAdapter) GetContextByScope(scope string) (string, error) {
	if a.hierarchy == nil {
		return "", nil
	}
	for _, f := range a.hierarchy.List() {
		if string(f.Scope) == scope && f.Loaded {
			return f.Content, nil
		}
	}
	return "", nil
}

func (a *AgentsMDContextAdapter) ReloadContext() error {
	if a.hierarchy == nil {
		return nil
	}
	return a.hierarchy.Reload(context.Background())
}

func (a *AgentsMDContextAdapter) GetMergedAgentsMD() string {
	if a.hierarchy == nil {
		return ""
	}
	return a.hierarchy.Resolve()
}

func (a *AgentsMDContextAdapter) GetActiveContextSummary() string {
	if a.hierarchy == nil {
		return ""
	}
	files := a.hierarchy.List()
	var loaded int
	var totalSize int
	for _, f := range files {
		if f.Loaded {
			loaded++
			totalSize += f.Size
		}
	}
	return fmt.Sprintf("%d/%d files loaded, %d chars total", loaded, len(files), totalSize)
}

func cmdContextHandler(args []string) error {
	if globalContextManager == nil {
		return contextNoManager()
	}

	if len(args) == 0 {
		return contextSummaryHandler(args)
	}

	switch args[0] {
	case "list":
		return contextListHandler(args[1:])
	case "show":
		return contextShowHandler(args[1:])
	case "reload":
		return contextReloadHandler(args[1:])
	case "agents":
		return contextAgentsHandler(args[1:])
	default:
		fmt.Printf("Unknown context subcommand: %s\n", args[0])
		fmt.Println("Usage: /context [list|show <scope>|reload|agents]")
		return nil
	}
}

func contextNoManager() error {
	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Active Context                    │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println("  Context manager not initialized")
	fmt.Println()
	fmt.Println("  The context manager loads AGENTS.md, SOUL.md,")
	fmt.Println("  and other context files from your project.")
	fmt.Println()
	fmt.Println("  Subcommands:")
	fmt.Println("    /context list            List all context files")
	fmt.Println("    /context show <scope>    Show context by scope")
	fmt.Println("    /context reload          Reload context from disk")
	fmt.Println("    /context agents          Show merged AGENTS.md")
	return nil
}

func contextSummaryHandler(args []string) error {
	files := globalContextManager.ListContextFiles()
	summary := globalContextManager.GetActiveContextSummary()

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Active Context                    │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()

	if summary != "" {
		for _, line := range strings.Split(summary, "\n") {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println()
	}

	fmt.Println("  Context Files:")
	if len(files) == 0 {
		fmt.Println("    (none loaded)")
	} else {
		for _, f := range files {
			if f.Loaded {
				fmt.Printf("    %-5s %-25s %s chars\n", "\u2713", fmt.Sprintf("%s (%s)", shortName(f.Path), f.Scope), formatSize(f.Size))
			} else {
				fmt.Printf("    %-5s %-25s %s\n", "\u2717", shortName(f.Path), "not found")
			}
		}
	}

	return nil
}

func contextListHandler(args []string) error {
	files := globalContextManager.ListContextFiles()

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Context Files                     │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()

	if len(files) == 0 {
		fmt.Println("  No context files found")
		return nil
	}

	fmt.Printf("  %-5s %-25s %-12s %8s %8s\n", "Stat", "File", "Scope", "Size", "Pri")
	fmt.Printf("  %-5s %-25s %-12s %8s %8s\n", "-----", "-------------------------", "------------", "--------", "--------")

	for _, f := range files {
		status := "\u2717"
		size := "n/a"
		if f.Loaded {
			status = "\u2713"
			size = formatSize(f.Size)
		}
		fmt.Printf("  %-5s %-25s %-12s %8s %8d\n", status, shortName(f.Path), f.Scope, size, f.Priority)
	}

	fmt.Println()
	fmt.Printf("  Total: %d files, %d loaded\n", len(files), countLoaded(files))
	return nil
}

func contextShowHandler(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /context show <scope>")
		fmt.Println()
		fmt.Println("Available scopes: project, user, workspace, directory")
		return nil
	}

	scope := args[0]
	content, err := globalContextManager.GetContextByScope(scope)
	if err != nil {
		fmt.Printf("  Error loading context for scope %q: %v\n", scope, err)
		return nil
	}

	if content == "" {
		fmt.Printf("  No context found for scope: %s\n", scope)
		return nil
	}

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Printf("│  Context: %-30s │\n", scope)
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()
	fmt.Println(content)

	return nil
}

func contextReloadHandler(args []string) error {
	fmt.Println("  Reloading context from disk...")

	if err := globalContextManager.ReloadContext(); err != nil {
		fmt.Printf("  \u2717 Reload failed: %v\n", err)
		return nil
	}

	files := globalContextManager.ListContextFiles()
	loaded := countLoaded(files)

	fmt.Printf("  \u2713 Context reloaded: %d/%d files loaded\n", loaded, len(files))
	return nil
}

func contextAgentsHandler(args []string) error {
	merged := globalContextManager.GetMergedAgentsMD()

	fmt.Println("╭──────────────────────────────────────────╮")
	fmt.Println("│         Merged AGENTS.md                  │")
	fmt.Println("╰──────────────────────────────────────────╯")
	fmt.Println()

	if merged == "" {
		fmt.Println("  No AGENTS.md content found")
		fmt.Println()
		fmt.Println("  AGENTS.md files are loaded from:")
		fmt.Println("    - Project root (./AGENTS.md)")
		fmt.Println("    - User home (~/.smartclaw/AGENTS.md)")
		fmt.Println("    - Workspace directory")
		return nil
	}

	fmt.Println(merged)
	return nil
}

func shortName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func formatSize(size int) string {
	if size >= 1000 {
		return fmt.Sprintf("%s chars", formatNumber(size))
	}
	return fmt.Sprintf("%d chars", size)
}

func formatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []string
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		result = append([]string{s[start:i]}, result...)
	}
	return strings.Join(result, ",")
}

func countLoaded(files []ContextFileInfo) int {
	count := 0
	for _, f := range files {
		if f.Loaded {
			count++
		}
	}
	return count
}
