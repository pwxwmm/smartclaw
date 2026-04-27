package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	pkgconfig "github.com/instructkr/smartclaw/internal/config"
)

func init() {
	Register(Command{
		Name:    "model",
		Summary: "Show or set model",
	}, modelHandler)

	Register(Command{
		Name:    "model-list",
		Summary: "List available models",
	}, modelListHandler)

	Register(Command{
		Name:    "config",
		Summary: "Show configuration",
	}, configHandler)

	Register(Command{
		Name:    "config-show",
		Summary: "Show configuration",
	}, configShowHandler)

	Register(Command{
		Name:    "config-set",
		Summary: "Set config value",
	}, configSetHandler)

	Register(Command{
		Name:    "config-get",
		Summary: "Get config value",
	}, configGetHandler)

	Register(Command{
		Name:    "config-reset",
		Summary: "Reset configuration",
	}, configResetHandler)

	Register(Command{
		Name:    "config-export",
		Summary: "Export configuration",
	}, configExportHandler)

	Register(Command{
		Name:    "config-import",
		Summary: "Import configuration",
	}, configImportHandler)

	Register(Command{
		Name:    "set-api-key",
		Summary: "Set API key",
	}, setAPIKeyHandler)

	Register(Command{
		Name:    "env",
		Summary: "Show environment",
	}, envHandler)
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
