package commands

import (
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}
}

func TestRegistryRegister(t *testing.T) {
	registry := NewRegistry()

	cmd := Command{
		Name:        "test",
		Description: "Test command",
	}

	handler := func(args []string) error { return nil }

	registry.Register(cmd, handler)

	if !registry.Has("test") {
		t.Error("Expected command to be registered")
	}
}

func TestRegistryGet(t *testing.T) {
	registry := NewRegistry()

	cmd := Command{
		Name:        "test",
		Description: "Test command",
	}

	registry.Register(cmd, func(args []string) error { return nil })

	retrieved := registry.Get("test")
	if retrieved.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", retrieved.Name)
	}
}

func TestRegistryExecute(t *testing.T) {
	registry := NewRegistry()

	executed := false
	cmd := Command{
		Name:        "test",
		Description: "Test command",
	}

	registry.Register(cmd, func(args []string) error {
		executed = true
		return nil
	})

	err := registry.Execute("test", []string{"arg1"})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !executed {
		t.Error("Expected handler to be executed")
	}
}

func TestRegistryExecuteNonexistent(t *testing.T) {
	registry := NewRegistry()

	err := registry.Execute("nonexistent", []string{})
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}
}

func TestRegistryHas(t *testing.T) {
	registry := NewRegistry()

	if registry.Has("test") {
		t.Error("Expected Has to return false for nonexistent command")
	}

	cmd := Command{Name: "test"}
	registry.Register(cmd, func(args []string) error { return nil })

	if !registry.Has("test") {
		t.Error("Expected Has to return true for registered command")
	}
}

func TestRegistryAll(t *testing.T) {
	registry := NewRegistry()

	cmd1 := Command{Name: "cmd1", Description: "Command 1"}
	cmd2 := Command{Name: "cmd2", Description: "Command 2"}

	registry.Register(cmd1, func(args []string) error { return nil })
	registry.Register(cmd2, func(args []string) error { return nil })

	all := registry.All()
	if len(all) < 2 {
		t.Errorf("Expected at least 2 commands, got %d", len(all))
	}
}

func TestRegistryHelp(t *testing.T) {
	registry := NewRegistry()

	help := registry.Help()
	if help == "" {
		t.Error("Expected non-empty help string")
	}
}

func TestCommand(t *testing.T) {
	cmd := Command{
		Name:        "help",
		Description: "Show help",
		Aliases:     []string{"h", "?"},
	}

	if cmd.Name != "help" {
		t.Errorf("Expected name 'help', got '%s'", cmd.Name)
	}

	if len(cmd.Aliases) != 2 {
		t.Errorf("Expected 2 aliases, got %d", len(cmd.Aliases))
	}
}

func TestGetRegistry(t *testing.T) {
	registry := GetRegistry()
	if registry == nil {
		t.Error("Expected non-nil registry")
	}
}

func TestExecute(t *testing.T) {
	err := Execute("nonexistent", []string{})
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}
}

func TestRegister(t *testing.T) {
	cmd := Command{
		Name:        "custom-test",
		Description: "Custom test command",
	}

	Register(cmd, func(args []string) error { return nil })

	if !GetRegistry().Has("custom-test") {
		t.Error("Expected custom command to be registered")
	}
}
