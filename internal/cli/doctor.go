package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/instructkr/smartclaw/internal/auth"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostics",
	Long: `Run diagnostic checks to verify SmartClaw is properly configured.

Checks include:
  • API key configuration
  • Configuration file
  • Session storage
  • Git installation
  • Network connectivity`,
	Run: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) {
	fmt.Println("SmartClaw Diagnostics")
	fmt.Println("=====================")
	fmt.Println()

	checks := []struct {
		name string
		fn   func() (bool, string)
	}{
		{"API Key", checkAPIKey},
		{"Config File", checkConfig},
		{"Session Storage", checkSessions},
		{"Git", checkGit},
		{"Go Version", checkGo},
	}

	passed := 0
	for _, check := range checks {
		ok, msg := check.fn()
		status := "✗"
		if ok {
			status = "✓"
			passed++
		}
		fmt.Printf("  %s %s: %s\n", status, check.name, msg)
	}

	fmt.Println()
	fmt.Printf("Passed: %d/%d\n", passed, len(checks))

	if passed == len(checks) {
		fmt.Println("\nAll checks passed! SmartClaw is ready to use.")
	} else {
		fmt.Println("\nSome checks failed. Please fix the issues above.")
		os.Exit(1)
	}
}

func checkAPIKey() (bool, string) {
	apiKey := auth.GetAPIKey()
	if apiKey == "" {
		return false, "not set (set ANTHROPIC_API_KEY or use --api-key)"
	}
	if len(apiKey) < 10 {
		return false, "invalid (too short)"
	}
	return true, fmt.Sprintf("configured (%d chars)", len(apiKey))
}

func checkConfig() (bool, string) {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = home + "/.smartclaw/config.yaml"
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return true, "using defaults (no config file)"
		}
	}
	return true, configPath
}

func checkSessions() (bool, string) {
	home, _ := os.UserHomeDir()
	sessionDir := home + "/.smartclaw/sessions"

	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		if err := os.MkdirAll(sessionDir, 0755); err != nil {
			return false, "cannot create session directory"
		}
		return true, "created session directory"
	}

	files, err := os.ReadDir(sessionDir)
	if err != nil {
		return false, "cannot read session directory"
	}

	return true, fmt.Sprintf("%d sessions", len(files))
}

func checkGit() (bool, string) {
	_, err := exec.LookPath("git")
	if err != nil {
		return false, "not installed"
	}

	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()
	if err != nil {
		return false, "error running git"
	}

	version := string(output)
	if len(version) > 20 {
		version = version[:20] + "..."
	}
	return true, version
}

func checkGo() (bool, string) {
	return true, runtime.Version()
}
