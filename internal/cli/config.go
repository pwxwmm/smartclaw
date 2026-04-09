package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `View and manage SmartClaw configuration.

Configuration is stored in ~/.smartclaw/config.yaml by default.`,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration values",
	Run:   runConfigList,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	Run:   runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run:   runConfigSet,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Run:   runConfigPath,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
}

func runConfigList(cmd *cobra.Command, args []string) {
	settings := viper.AllSettings()

	fmt.Println("Configuration:")
	fmt.Println("--------------")
	for key, value := range settings {
		fmt.Printf("  %s: %v\n", key, value)
	}
}

func runConfigGet(cmd *cobra.Command, args []string) {
	key := args[0]
	value := viper.Get(key)

	if value == nil {
		fmt.Printf("%s: (not set)\n", key)
		return
	}

	fmt.Printf("%s: %v\n", key, value)
}

func runConfigSet(cmd *cobra.Command, args []string) {
	key := args[0]
	value := args[1]

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error getting home directory:", err)
		os.Exit(1)
	}
	configDir := filepath.Join(home, ".smartclaw")
	configPath := filepath.Join(configDir, "config.json")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "Error creating config directory:", err)
		os.Exit(1)
	}

	var configData map[string]any
	if _, err := os.Stat(configPath); err == nil {
		if data, err := os.ReadFile(configPath); err == nil {
			json.Unmarshal(data, &configData)
		}
	}
	if configData == nil {
		configData = make(map[string]any)
	}

	var parsedValue any = value
	if value == "true" {
		parsedValue = true
	} else if value == "false" {
		parsedValue = false
	}

	configData[key] = parsedValue

	data, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error marshaling config:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		fmt.Fprintln(os.Stderr, "Error writing config:", err)
		os.Exit(1)
	}

	fmt.Printf("Set %s = %v\n", key, parsedValue)
}

func runConfigPath(cmd *cobra.Command, args []string) {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = home + "/.smartclaw/config.yaml (default, not created)"
	}
	fmt.Println(configPath)
}
