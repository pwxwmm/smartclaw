package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
)

type BootstrapConfig struct {
	AutoInstall bool
	SkipUpdate  bool
	Force       bool
}

func RunBootstrap(config BootstrapConfig) error {
	fmt.Println("Running bootstrap...")

	home, _ := os.UserHomeDir()
	smartDir := filepath.Join(home, ".smartclaw")

	if config.AutoInstall {
		os.MkdirAll(smartDir, 0755)
		fmt.Printf("Created config directory: %s\n", smartDir)
	}

	if !config.SkipUpdate {
		fmt.Println("Checking for updates...")
	}

	fmt.Println("Bootstrap complete")
	return nil
}

func InitConfig() error {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".smartclaw", "config.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := `{
  "model": "claude-sonnet-4-5",
  "permission": "ask",
  "log_level": "info"
}`
		os.MkdirAll(filepath.Dir(configPath), 0755)
		return os.WriteFile(configPath, []byte(defaultConfig), 0644)
	}
	return nil
}

func CheckPrerequisites() error {
	fmt.Println("Checking prerequisites...")
	return nil
}
