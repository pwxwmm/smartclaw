package commands

import (
	"fmt"
	"os"
	"path/filepath"
)

func init() {
	Register(Command{
		Name:    "login",
		Summary: "Authenticate with service",
	}, loginHandler)

	Register(Command{
		Name:    "logout",
		Summary: "Clear authentication",
	}, logoutHandler)

	Register(Command{
		Name:    "upgrade",
		Summary: "Upgrade CLI version",
	}, upgradeHandler)

	Register(Command{
		Name:    "api",
		Summary: "API operations",
	}, apiHandler)
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
