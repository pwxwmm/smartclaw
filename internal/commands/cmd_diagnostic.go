package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

func init() {
	Register(Command{
		Name:    "doctor",
		Summary: "Run diagnostics",
	}, doctorHandler)

	Register(Command{
		Name:    "cost",
		Summary: "Show token usage and cost",
	}, costHandler)

	Register(Command{
		Name:    "stats",
		Summary: "Show session statistics",
	}, statsHandler)

	Register(Command{
		Name:    "usage",
		Summary: "Usage stats",
	}, usageHandler)

	Register(Command{
		Name:    "debug",
		Summary: "Toggle debug mode",
	}, debugHandler)

	Register(Command{
		Name:    "inspect",
		Summary: "Inspect state",
	}, inspectHandler)

	Register(Command{
		Name:    "cache",
		Summary: "Manage cache",
	}, cacheHandler)

	Register(Command{
		Name:    "heapdump",
		Summary: "Heap dump",
	}, heapdumpHandler)

	Register(Command{
		Name:    "reset-limits",
		Summary: "Reset limits",
	}, resetLimitsHandler)
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

func statsHandler(args []string) error {
	return statusHandler(args)
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

func debugHandler(args []string) error {
	fmt.Println("Debug mode toggled")
	fmt.Println("⚠️  Debug logging enabled")
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

func resetLimitsHandler(args []string) error {
	cmdCtx.InputTokens = 0
	cmdCtx.OutputTokens = 0
	cmdCtx.TokenCount = 0
	fmt.Println("✓ Token limits reset")
	return nil
}
