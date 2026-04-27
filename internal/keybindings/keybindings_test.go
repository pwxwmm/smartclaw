package keybindings

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestNewKeyBindings(t *testing.T) {
	t.Parallel()

	kb := NewKeyBindings()
	if kb == nil {
		t.Fatal("NewKeyBindings() returned nil")
	}
	if len(kb.bindings) != 0 {
		t.Errorf("new KeyBindings should have empty bindings, got %d", len(kb.bindings))
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()

	kb := NewKeyBindings()
	kb.Register(KeyBinding{Key: "k", Command: "test-cmd", Modifier: "ctrl"})

	b, ok := kb.Get("ctrl+k")
	if !ok {
		t.Error("Get(\"ctrl+k\") should find registered binding")
	}
	if b.Command != "test-cmd" {
		t.Errorf("Command = %q, want %q", b.Command, "test-cmd")
	}
	if b.Key != "k" {
		t.Errorf("Key = %q, want %q", b.Key, "k")
	}
	if b.Modifier != "ctrl" {
		t.Errorf("Modifier = %q, want %q", b.Modifier, "ctrl")
	}
}

func TestRegister_Overwrite(t *testing.T) {
	t.Parallel()

	kb := NewKeyBindings()
	kb.Register(KeyBinding{Key: "k", Command: "first", Modifier: "ctrl"})
	kb.Register(KeyBinding{Key: "k", Command: "second", Modifier: "ctrl"})

	b, ok := kb.Get("ctrl+k")
	if !ok {
		t.Fatal("Get(\"ctrl+k\") should find binding")
	}
	if b.Command != "second" {
		t.Errorf("Command = %q, want %q (should overwrite)", b.Command, "second")
	}
}

func TestRegister_NoModifier(t *testing.T) {
	t.Parallel()

	kb := NewKeyBindings()
	kb.Register(KeyBinding{Key: "enter", Command: "submit", Modifier: ""})

	b, ok := kb.Get("+enter")
	if !ok {
		t.Error("Get(\"+enter\") should find binding with empty modifier")
	}
	if b.Command != "submit" {
		t.Errorf("Command = %q, want %q", b.Command, "submit")
	}
}

func TestGet_DefaultBindings(t *testing.T) {
	t.Parallel()

	kb := NewKeyBindings()

	tests := []struct {
		key     string
		command string
	}{
		{"ctrl+c", "copy"},
		{"ctrl+v", "paste"},
		{"ctrl+z", "undo"},
		{"ctrl+s", "save"},
		{"ctrl+p", "search"},
		{"ctrl+r", "refresh"},
		{"ctrl+/", "comment"},
		{"escape", "cancel"},
		{"enter", "submit"},
		{"tab", "complete"},
	}

	for _, tt := range tests {
		b, ok := kb.Get(tt.key)
		if !ok {
			t.Errorf("Get(%q) should find default binding", tt.key)
			continue
		}
		if b.Command != tt.command {
			t.Errorf("Get(%q).Command = %q, want %q", tt.key, b.Command, tt.command)
		}
	}
}

func TestGet_CustomOverridesDefault(t *testing.T) {
	t.Parallel()

	kb := NewKeyBindings()
	kb.Register(KeyBinding{Key: "c", Command: "custom-copy", Modifier: "ctrl"})

	b, ok := kb.Get("ctrl+c")
	if !ok {
		t.Fatal("Get(\"ctrl+c\") should find binding")
	}
	if b.Command != "custom-copy" {
		t.Errorf("Command = %q, want %q (custom should override default)", b.Command, "custom-copy")
	}
}

func TestGet_NotFound(t *testing.T) {
	t.Parallel()

	kb := NewKeyBindings()
	_, ok := kb.Get("ctrl+nonexistent")
	if ok {
		t.Error("Get(\"ctrl+nonexistent\") should not find a binding")
	}
}

func TestList(t *testing.T) {
	kb := NewKeyBindings()
	kb.Register(KeyBinding{Key: "a", Command: "cmd-a", Modifier: "ctrl"})
	kb.Register(KeyBinding{Key: "b", Command: "cmd-b", Modifier: "alt"})

	list := kb.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d items, want 2", len(list))
	}

	commands := make([]string, 0, len(list))
	for _, b := range list {
		commands = append(commands, b.Command)
	}
	sort.Strings(commands)

	if commands[0] != "cmd-a" || commands[1] != "cmd-b" {
		t.Errorf("List() commands = %v, want [cmd-a, cmd-b]", commands)
	}
}

func TestList_Empty(t *testing.T) {
	t.Parallel()

	kb := NewKeyBindings()
	list := kb.List()
	if len(list) != 0 {
		t.Errorf("List() on empty KeyBindings returned %d items, want 0", len(list))
	}
}

func TestLoadFromFile_Nonexistent(t *testing.T) {
	t.Parallel()

	kb, err := LoadFromFile("/nonexistent/path/keybindings.json")
	if err != nil {
		t.Errorf("LoadFromFile() on nonexistent path returned error: %v", err)
	}
	if kb == nil {
		t.Fatal("LoadFromFile() returned nil")
	}
}

func TestLoadFromFile_Existing(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "keybindings.json")
	os.WriteFile(path, []byte("{}"), 0644)

	kb, err := LoadFromFile(path)
	if err != nil {
		t.Errorf("LoadFromFile() returned error: %v", err)
	}
	if kb == nil {
		t.Fatal("LoadFromFile() returned nil")
	}
}

func TestDefaultBindings_AllPresent(t *testing.T) {
	t.Parallel()

	if len(defaultBindings) != 10 {
		t.Errorf("expected 10 default bindings, got %d", len(defaultBindings))
	}
}

func TestKeyBinding_Fields(t *testing.T) {
	t.Parallel()

	kb := KeyBinding{Key: "x", Command: "execute", Modifier: "alt"}
	if kb.Key != "x" {
		t.Errorf("Key = %q, want %q", kb.Key, "x")
	}
	if kb.Command != "execute" {
		t.Errorf("Command = %q, want %q", kb.Command, "execute")
	}
	if kb.Modifier != "alt" {
		t.Errorf("Modifier = %q, want %q", kb.Modifier, "alt")
	}
}
