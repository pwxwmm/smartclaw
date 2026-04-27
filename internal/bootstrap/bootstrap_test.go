package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunBootstrap(t *testing.T) {
	err := RunBootstrap(BootstrapConfig{})
	if err != nil {
		t.Errorf("RunBootstrap() returned error: %v", err)
	}
}

func TestRunBootstrap_AutoInstall(t *testing.T) {
	home, _ := os.UserHomeDir()
	smartDir := filepath.Join(home, ".smartclaw")

	err := RunBootstrap(BootstrapConfig{AutoInstall: true})
	if err != nil {
		t.Errorf("RunBootstrap(AutoInstall=true) returned error: %v", err)
	}

	if _, err := os.Stat(smartDir); os.IsNotExist(err) {
		t.Errorf("SmartClaw directory %q should exist after AutoInstall", smartDir)
	}
}

func TestRunBootstrap_SkipUpdate(t *testing.T) {
	err := RunBootstrap(BootstrapConfig{SkipUpdate: true})
	if err != nil {
		t.Errorf("RunBootstrap(SkipUpdate=true) returned error: %v", err)
	}
}

func TestRunBootstrap_AllFlags(t *testing.T) {
	err := RunBootstrap(BootstrapConfig{
		AutoInstall: true,
		SkipUpdate:  true,
		Force:       true,
	})
	if err != nil {
		t.Errorf("RunBootstrap(all flags) returned error: %v", err)
	}
}

func TestInitConfig_CreatesFile(t *testing.T) {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".smartclaw", "config.json")
	os.Remove(configPath)
	defer os.Remove(configPath)

	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() returned error: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file %q should exist after InitConfig()", configPath)
	}
}

func TestInitConfig_Idempotent(t *testing.T) {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".smartclaw", "config.json")
	os.Remove(configPath)
	defer os.Remove(configPath)

	err := InitConfig()
	if err != nil {
		t.Fatalf("First InitConfig() returned error: %v", err)
	}

	err = InitConfig()
	if err != nil {
		t.Fatalf("Second InitConfig() returned error: %v", err)
	}
}

func TestInitConfig_DoesNotOverwrite(t *testing.T) {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".smartclaw")
	configPath := filepath.Join(configDir, "config.json")
	os.MkdirAll(configDir, 0755)
	customContent := `{"model": "custom-model"}`
	os.WriteFile(configPath, []byte(customContent), 0644)
	defer os.Remove(configPath)

	err := InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() returned error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if string(data) != customContent {
		t.Errorf("Config was overwritten; got %q, want %q", string(data), customContent)
	}
}

func TestCheckPrerequisites(t *testing.T) {
	err := CheckPrerequisites()
	if err != nil {
		t.Errorf("CheckPrerequisites() returned error: %v", err)
	}
}

func TestBootstrapConfig_Defaults(t *testing.T) {
	config := BootstrapConfig{}
	if config.AutoInstall {
		t.Error("Default AutoInstall should be false")
	}
	if config.SkipUpdate {
		t.Error("Default SkipUpdate should be false")
	}
	if config.Force {
		t.Error("Default Force should be false")
	}
}
