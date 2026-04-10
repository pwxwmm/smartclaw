package sandbox

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := Config{
		Enabled:            true,
		NamespaceIsolation: false,
		NetworkIsolation:   false,
		FilesystemMode:     "off",
	}
	mgr := NewManager(config)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManagerGetStatus(t *testing.T) {
	config := Config{
		Enabled:            true,
		NamespaceIsolation: false,
		NetworkIsolation:   true,
		FilesystemMode:     "readonly",
		AllowedMounts:      []string{"/tmp"},
	}
	mgr := NewManager(config)

	status := mgr.GetStatus()

	if !status.Enabled {
		t.Error("Expected Enabled=true")
	}

	if status.NamespaceIsolation {
		t.Error("Expected NamespaceIsolation=false")
	}

	if !status.NetworkIsolation {
		t.Error("Expected NetworkIsolation=true")
	}

	if status.FilesystemMode != "readonly" {
		t.Errorf("Expected FilesystemMode 'readonly', got '%s'", status.FilesystemMode)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled {
		t.Error("Default config should have Enabled=false")
	}

	if config.NamespaceIsolation {
		t.Error("Default config should have NamespaceIsolation=false")
	}

	if config.NetworkIsolation {
		t.Error("Default config should have NetworkIsolation=false")
	}

	if config.FilesystemMode != "off" {
		t.Errorf("Expected FilesystemMode 'off', got '%s'", config.FilesystemMode)
	}
}

func TestStatusString(t *testing.T) {
	status := Status{
		Enabled:            true,
		NamespaceIsolation: false,
		NetworkIsolation:   false,
		FilesystemMode:     "off",
		Platform:           "darwin",
		ContainerEnv:       false,
	}

	str := status.String()
	if str == "" {
		t.Error("Status String() should not be empty")
	}
}

func TestIsLinux(t *testing.T) {
	result := IsLinux()
	if result && defaultPlatform() != "linux" {
		t.Error("IsLinux should match actual platform")
	}
}

func TestSupportsNamespaces(t *testing.T) {
	_ = SupportsNamespaces()
}

func defaultPlatform() string {
	return "darwin"
}
