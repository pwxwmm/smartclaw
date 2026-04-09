package migrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Migration struct {
	ID          string
	Name        string
	Description string
	Execute     func(config map[string]interface{}) error
}

func GetMigrations() []Migration {
	return []Migration{
		{
			ID:          "migrate_sonnet_45_to_46",
			Name:        "Migrate Sonnet 4.5 to 4.6",
			Description: "Update default model from sonnet-4-5 to sonnet-4-6",
			Execute:     migrateSonnet45To46,
		},
		{
			ID:          "migrate_fennec_to_opus",
			Name:        "Migrate Fennec to Opus",
			Description: "Update model from fennec to opus",
			Execute:     migrateFennecToOpus,
		},
		{
			ID:          "migrate_opus_to_opus_1m",
			Name:        "Migrate Opus to Opus 1M",
			Description: "Update model from opus to opus-1m",
			Execute:     migrateOpusToOpus1M,
		},
		{
			ID:          "migrate_legacy_opus",
			Name:        "Migrate Legacy Opus",
			Description: "Update legacy opus model configuration",
			Execute:     migrateLegacyOpus,
		},
		{
			ID:          "migrate_bypass_permissions",
			Name:        "Migrate Bypass Permissions",
			Description: "Migrate bypass permissions accepted to settings",
			Execute:     migrateBypassPermissions,
		},
		{
			ID:          "migrate_auto_updates",
			Name:        "Migrate Auto Updates",
			Description: "Migrate auto updates to settings",
			Execute:     migrateAutoUpdates,
		},
		{
			ID:          "migrate_enable_all_mcp",
			Name:        "Migrate Enable All MCP",
			Description: "Migrate enable all project MCP servers to settings",
			Execute:     migrateEnableAllMCP,
		},
		{
			ID:          "migrate_repl_bridge",
			Name:        "Migrate REPL Bridge",
			Description: "Migrate REPL bridge enabled to remote control at startup",
			Execute:     migrateReplBridge,
		},
		{
			ID:          "reset_auto_mode_opt_in",
			Name:        "Reset Auto Mode Opt-In",
			Description: "Reset auto mode opt-in for default offer",
			Execute:     resetAutoModeOptIn,
		},
		{
			ID:          "reset_pro_to_opus",
			Name:        "Reset Pro to Opus",
			Description: "Reset pro users to opus default",
			Execute:     resetProToOpus,
		},
	}
}

func migrateSonnet45To46(config map[string]interface{}) error {
	if model, ok := config["model"].(string); ok {
		if strings.Contains(model, "sonnet-4-5") && !strings.Contains(model, "sonnet-4-6") {
			config["model"] = strings.ReplaceAll(model, "sonnet-4-5", "sonnet-4-6")
		}
	}
	return nil
}

func migrateFennecToOpus(config map[string]interface{}) error {
	if model, ok := config["model"].(string); ok {
		if strings.Contains(model, "fennec") {
			config["model"] = strings.ReplaceAll(model, "fennec", "opus")
		}
	}
	return nil
}

func migrateOpusToOpus1M(config map[string]interface{}) error {
	if model, ok := config["model"].(string); ok {
		if model == "opus" {
			config["model"] = "opus-1m"
		}
	}
	return nil
}

func migrateLegacyOpus(config map[string]interface{}) error {
	if model, ok := config["model"].(string); ok {
		if strings.HasPrefix(model, "opus-") && !strings.HasSuffix(model, "-1m") {
			config["model"] = "opus-1m"
		}
	}
	return nil
}

func migrateBypassPermissions(config map[string]interface{}) error {
	if bypass, ok := config["bypass_permissions_accepted"].(bool); ok && bypass {
		if perms, ok := config["permissions"].(map[string]interface{}); ok {
			perms["bypass_accepted"] = true
		} else {
			config["permissions"] = map[string]interface{}{"bypass_accepted": true}
		}
		delete(config, "bypass_permissions_accepted")
	}
	return nil
}

func migrateAutoUpdates(config map[string]interface{}) error {
	if autoUpdate, ok := config["auto_updates_enabled"].(bool); ok {
		if settings, ok := config["settings"].(map[string]interface{}); ok {
			settings["auto_updates"] = autoUpdate
		} else {
			config["settings"] = map[string]interface{}{"auto_updates": autoUpdate}
		}
		delete(config, "auto_updates_enabled")
	}
	return nil
}

func migrateEnableAllMCP(config map[string]interface{}) error {
	if enableAll, ok := config["enable_all_project_mcp_servers"].(bool); ok {
		if mcp, ok := config["mcp"].(map[string]interface{}); ok {
			mcp["enable_all_project_servers"] = enableAll
		} else {
			config["mcp"] = map[string]interface{}{"enable_all_project_servers": enableAll}
		}
		delete(config, "enable_all_project_mcp_servers")
	}
	return nil
}

func migrateReplBridge(config map[string]interface{}) error {
	if enabled, ok := config["repl_bridge_enabled"].(bool); ok {
		if remote, ok := config["remote"].(map[string]interface{}); ok {
			remote["control_at_startup"] = enabled
		} else {
			config["remote"] = map[string]interface{}{"control_at_startup": enabled}
		}
		delete(config, "repl_bridge_enabled")
	}
	return nil
}

func resetAutoModeOptIn(config map[string]interface{}) error {
	delete(config, "auto_mode_opt_in")
	return nil
}

func resetProToOpus(config map[string]interface{}) error {
	if accountType, ok := config["account_type"].(string); ok && accountType == "pro" {
		config["model"] = "opus-1m"
	}
	return nil
}

type MigrationRunner struct {
	configPath string
	applied    map[string]bool
}

func NewMigrationRunner(configPath string) *MigrationRunner {
	return &MigrationRunner{
		configPath: configPath,
		applied:    make(map[string]bool),
	}
}

func (r *MigrationRunner) LoadApplied() error {
	data, err := os.ReadFile(r.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var applied []string
	if err := json.Unmarshal(data, &applied); err != nil {
		return err
	}

	for _, id := range applied {
		r.applied[id] = true
	}

	return nil
}

func (r *MigrationRunner) SaveApplied() error {
	applied := make([]string, 0, len(r.applied))
	for id := range r.applied {
		applied = append(applied, id)
	}

	data, err := json.MarshalIndent(applied, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(r.configPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(r.configPath, data, 0644)
}

func (r *MigrationRunner) Run(config map[string]interface{}) error {
	migrations := GetMigrations()

	for _, migration := range migrations {
		if r.applied[migration.ID] {
			continue
		}

		if err := migration.Execute(config); err != nil {
			return fmt.Errorf("migration %s failed: %w", migration.ID, err)
		}

		r.applied[migration.ID] = true
	}

	return r.SaveApplied()
}

func (r *MigrationRunner) IsApplied(id string) bool {
	return r.applied[id]
}

func (r *MigrationRunner) GetPending() []string {
	var pending []string
	for _, migration := range GetMigrations() {
		if !r.applied[migration.ID] {
			pending = append(pending, migration.ID)
		}
	}
	return pending
}
