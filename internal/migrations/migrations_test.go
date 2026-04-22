package migrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMigrations_ReturnsTen(t *testing.T) {
	migrations := GetMigrations()
	if len(migrations) != 10 {
		t.Errorf("GetMigrations() returned %d migrations, want 10", len(migrations))
	}
}

func TestGetMigrations_AllHaveID(t *testing.T) {
	migrations := GetMigrations()
	for _, m := range migrations {
		if m.ID == "" {
			t.Error("Migration has empty ID")
		}
		if m.Name == "" {
			t.Error("Migration has empty Name")
		}
		if m.Execute == nil {
			t.Errorf("Migration %s has nil Execute", m.ID)
		}
	}
}

func TestMigrateSonnet45To46(t *testing.T) {
	config := map[string]any{"model": "claude-sonnet-4-5"}
	if err := migrateSonnet45To46(config); err != nil {
		t.Fatalf("migrateSonnet45To46 failed: %v", err)
	}
	if config["model"] != "claude-sonnet-4-6" {
		t.Errorf("model = %v, want claude-sonnet-4-6", config["model"])
	}
}

func TestMigrateSonnet45To46_NoMatch(t *testing.T) {
	config := map[string]any{"model": "claude-opus-4"}
	migrateSonnet45To46(config)
	if config["model"] != "claude-opus-4" {
		t.Errorf("model should be unchanged, got %v", config["model"])
	}
}

func TestMigrateSonnet45To46_Already46(t *testing.T) {
	config := map[string]any{"model": "claude-sonnet-4-6"}
	migrateSonnet45To46(config)
	if config["model"] != "claude-sonnet-4-6" {
		t.Errorf("model should be unchanged, got %v", config["model"])
	}
}

func TestMigrateSonnet45To46_NoModel(t *testing.T) {
	config := map[string]any{}
	migrateSonnet45To46(config)
	if _, ok := config["model"]; ok {
		t.Error("model key should not be added when absent")
	}
}

func TestMigrateFennecToOpus(t *testing.T) {
	config := map[string]any{"model": "fennec-1.0"}
	if err := migrateFennecToOpus(config); err != nil {
		t.Fatalf("migration failed: %v", err)
	}
	if config["model"] != "opus-1.0" {
		t.Errorf("model = %v, want opus-1.0", config["model"])
	}
}

func TestMigrateFennecToOpus_NoMatch(t *testing.T) {
	config := map[string]any{"model": "sonnet-4-6"}
	migrateFennecToOpus(config)
	if config["model"] != "sonnet-4-6" {
		t.Errorf("model should be unchanged, got %v", config["model"])
	}
}

func TestMigrateOpusToOpus1M(t *testing.T) {
	config := map[string]any{"model": "opus"}
	migrateOpusToOpus1M(config)
	if config["model"] != "opus-1m" {
		t.Errorf("model = %v, want opus-1m", config["model"])
	}
}

func TestMigrateOpusToOpus1M_NotExactOpus(t *testing.T) {
	config := map[string]any{"model": "opus-4"}
	migrateOpusToOpus1M(config)
	if config["model"] != "opus-4" {
		t.Errorf("model should be unchanged (not exact 'opus'), got %v", config["model"])
	}
}

func TestMigrateLegacyOpus(t *testing.T) {
	config := map[string]any{"model": "opus-4"}
	migrateLegacyOpus(config)
	if config["model"] != "opus-1m" {
		t.Errorf("model = %v, want opus-1m", config["model"])
	}
}

func TestMigrateLegacyOpus_Already1M(t *testing.T) {
	config := map[string]any{"model": "opus-1m"}
	migrateLegacyOpus(config)
	if config["model"] != "opus-1m" {
		t.Errorf("model should be unchanged, got %v", config["model"])
	}
}

func TestMigrateLegacyOpus_NotOpusPrefix(t *testing.T) {
	config := map[string]any{"model": "sonnet-4-6"}
	migrateLegacyOpus(config)
	if config["model"] != "sonnet-4-6" {
		t.Errorf("model should be unchanged, got %v", config["model"])
	}
}

func TestMigrateBypassPermissions(t *testing.T) {
	config := map[string]any{
		"bypass_permissions_accepted": true,
	}
	migrateBypassPermissions(config)
	perms, ok := config["permissions"].(map[string]any)
	if !ok {
		t.Fatal("permissions map not created")
	}
	if !perms["bypass_accepted"].(bool) {
		t.Error("bypass_accepted should be true")
	}
	if _, exists := config["bypass_permissions_accepted"]; exists {
		t.Error("bypass_permissions_accepted should be deleted")
	}
}

func TestMigrateBypassPermissions_ExistingPerms(t *testing.T) {
	config := map[string]any{
		"bypass_permissions_accepted": true,
		"permissions":                map[string]any{"existing": "value"},
	}
	migrateBypassPermissions(config)
	perms := config["permissions"].(map[string]any)
	if perms["bypass_accepted"] != true {
		t.Error("bypass_accepted should be true in existing permissions")
	}
	if perms["existing"] != "value" {
		t.Error("existing permissions should be preserved")
	}
}

func TestMigrateBypassPermissions_False(t *testing.T) {
	config := map[string]any{
		"bypass_permissions_accepted": false,
	}
	migrateBypassPermissions(config)
	if _, ok := config["permissions"]; ok {
		t.Error("permissions should not be created when bypass is false")
	}
}

func TestMigrateAutoUpdates(t *testing.T) {
	config := map[string]any{
		"auto_updates_enabled": true,
	}
	migrateAutoUpdates(config)
	settings, ok := config["settings"].(map[string]any)
	if !ok {
		t.Fatal("settings map not created")
	}
	if !settings["auto_updates"].(bool) {
		t.Error("auto_updates should be true")
	}
	if _, exists := config["auto_updates_enabled"]; exists {
		t.Error("auto_updates_enabled should be deleted")
	}
}

func TestMigrateAutoUpdates_False(t *testing.T) {
	config := map[string]any{
		"auto_updates_enabled": false,
	}
	migrateAutoUpdates(config)
	settings := config["settings"].(map[string]any)
	if settings["auto_updates"] != false {
		t.Error("auto_updates should be false")
	}
	if _, exists := config["auto_updates_enabled"]; exists {
		t.Error("auto_updates_enabled should be deleted")
	}
}

func TestMigrateEnableAllMCP(t *testing.T) {
	config := map[string]any{
		"enable_all_project_mcp_servers": true,
	}
	migrateEnableAllMCP(config)
	mcp, ok := config["mcp"].(map[string]any)
	if !ok {
		t.Fatal("mcp map not created")
	}
	if !mcp["enable_all_project_servers"].(bool) {
		t.Error("enable_all_project_servers should be true")
	}
	if _, exists := config["enable_all_project_mcp_servers"]; exists {
		t.Error("enable_all_project_mcp_servers should be deleted")
	}
}

func TestMigrateReplBridge(t *testing.T) {
	config := map[string]any{
		"repl_bridge_enabled": true,
	}
	migrateReplBridge(config)
	remote, ok := config["remote"].(map[string]any)
	if !ok {
		t.Fatal("remote map not created")
	}
	if !remote["control_at_startup"].(bool) {
		t.Error("control_at_startup should be true")
	}
	if _, exists := config["repl_bridge_enabled"]; exists {
		t.Error("repl_bridge_enabled should be deleted")
	}
}

func TestResetAutoModeOptIn(t *testing.T) {
	config := map[string]any{
		"auto_mode_opt_in": true,
		"other_key":        "value",
	}
	resetAutoModeOptIn(config)
	if _, exists := config["auto_mode_opt_in"]; exists {
		t.Error("auto_mode_opt_in should be deleted")
	}
	if config["other_key"] != "value" {
		t.Error("other keys should be preserved")
	}
}

func TestResetAutoModeOptIn_NoKey(t *testing.T) {
	config := map[string]any{"other": "value"}
	resetAutoModeOptIn(config)
	if len(config) != 1 {
		t.Error("unrelated keys should not be affected")
	}
}

func TestResetProToOpus(t *testing.T) {
	config := map[string]any{
		"account_type": "pro",
	}
	resetProToOpus(config)
	if config["model"] != "opus-1m" {
		t.Errorf("model = %v, want opus-1m", config["model"])
	}
}

func TestResetProToOpus_NotPro(t *testing.T) {
	config := map[string]any{
		"account_type": "free",
		"model":        "sonnet-4-6",
	}
	resetProToOpus(config)
	if config["model"] != "sonnet-4-6" {
		t.Error("model should be unchanged for non-pro accounts")
	}
}

func TestResetProToOpus_NoAccountType(t *testing.T) {
	config := map[string]any{}
	resetProToOpus(config)
	if _, ok := config["model"]; ok {
		t.Error("model should not be set when no account_type")
	}
}

func TestMigrationRunner_New(t *testing.T) {
	r := NewMigrationRunner("/tmp/test_migrations.json")
	if r == nil {
		t.Fatal("NewMigrationRunner returned nil")
	}
	if len(r.applied) != 0 {
		t.Error("applied should be empty initially")
	}
}

func TestMigrationRunner_LoadApplied_MissingFile(t *testing.T) {
	r := NewMigrationRunner(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err := r.LoadApplied(); err != nil {
		t.Errorf("LoadApplied on missing file should not error: %v", err)
	}
}

func TestMigrationRunner_LoadApplied_SaveApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "applied.json")
	r := NewMigrationRunner(path)

	r.applied["migration_1"] = true
	r.applied["migration_2"] = true

	if err := r.SaveApplied(); err != nil {
		t.Fatalf("SaveApplied failed: %v", err)
	}

	r2 := NewMigrationRunner(path)
	if err := r2.LoadApplied(); err != nil {
		t.Fatalf("LoadApplied failed: %v", err)
	}
	if !r2.IsApplied("migration_1") || !r2.IsApplied("migration_2") {
		t.Error("Loaded applied migrations don't match")
	}
}

func TestMigrationRunner_IsApplied(t *testing.T) {
	r := NewMigrationRunner(filepath.Join(t.TempDir(), "applied.json"))
	if r.IsApplied("nonexistent") {
		t.Error("IsApplied should be false for unapplied migration")
	}
	r.applied["test_migration"] = true
	if !r.IsApplied("test_migration") {
		t.Error("IsApplied should be true for applied migration")
	}
}

func TestMigrationRunner_GetPending(t *testing.T) {
	r := NewMigrationRunner(filepath.Join(t.TempDir(), "applied.json"))
	pending := r.GetPending()
	if len(pending) != 10 {
		t.Errorf("GetPending = %d, want 10", len(pending))
	}
}

func TestMigrationRunner_GetPending_SomeApplied(t *testing.T) {
	r := NewMigrationRunner(filepath.Join(t.TempDir(), "applied.json"))
	migrations := GetMigrations()
	r.applied[migrations[0].ID] = true
	r.applied[migrations[1].ID] = true

	pending := r.GetPending()
	if len(pending) != 8 {
		t.Errorf("GetPending = %d, want 8", len(pending))
	}
}

func TestMigrationRunner_Run(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "applied.json")
	r := NewMigrationRunner(path)

	config := map[string]any{
		"model":                         "claude-sonnet-4-5",
		"bypass_permissions_accepted":   true,
		"auto_updates_enabled":          true,
		"enable_all_project_mcp_servers": true,
		"repl_bridge_enabled":           true,
		"auto_mode_opt_in":              true,
	}

	if err := r.Run(config); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if config["model"] != "claude-sonnet-4-6" {
		t.Errorf("model = %v, want claude-sonnet-4-6", config["model"])
	}

	if _, exists := config["bypass_permissions_accepted"]; exists {
		t.Error("bypass_permissions_accepted should be removed")
	}

	if _, exists := config["auto_updates_enabled"]; exists {
		t.Error("auto_updates_enabled should be removed")
	}

	if _, exists := config["auto_mode_opt_in"]; exists {
		t.Error("auto_mode_opt_in should be removed")
	}

	pending := r.GetPending()
	if len(pending) != 0 {
		t.Errorf("GetPending after Run = %d, want 0", len(pending))
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Applied file should be created after Run")
	}
}

func TestMigrationRunner_Run_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "applied.json")
	r := NewMigrationRunner(path)

	config := map[string]any{"model": "claude-sonnet-4-5"}
	r.Run(config)

	if config["model"] != "claude-sonnet-4-6" {
		t.Errorf("first run: model = %v, want claude-sonnet-4-6", config["model"])
	}

	config2 := map[string]any{"model": "claude-sonnet-4-5"}
	r.Run(config2)

	if config2["model"] != "claude-sonnet-4-5" {
		t.Errorf("second run: already-applied migrations should be skipped, model = %v, want claude-sonnet-4-5", config2["model"])
	}
}
