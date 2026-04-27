package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	Register(Command{
		Name:    "memory",
		Summary: "Show memory context",
	}, memoryHandler)

	Register(Command{
		Name:    "skills",
		Summary: "List available skills",
	}, skillsHandler)

	Register(Command{
		Name:    "observe",
		Summary: "Observe mode",
	}, observeHandler)
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

func observeHandler(args []string) error {
	fmt.Println("Observe mode enabled")
	fmt.Println("  Watching for file changes...")
	return nil
}
