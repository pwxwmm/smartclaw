package layers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewPromptMemory(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	// Should create default files
	if _, err := os.Stat(filepath.Join(dir, "MEMORY.md")); os.IsNotExist(err) {
		t.Error("MEMORY.md should exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "USER.md")); os.IsNotExist(err) {
		t.Error("USER.md should exist")
	}

	_ = pm
}

func TestAutoLoadReturnsCombinedContent(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	content := pm.AutoLoad()
	if content == "" {
		t.Error("AutoLoad should return non-empty content")
	}
}

func TestAutoLoadTruncatesToLimit(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	// Write very long content
	longContent := ""
	for i := 0; i < 5000; i++ {
		longContent += "x"
	}
	if err := pm.UpdateMemory(longContent); err != nil {
		t.Fatalf("UpdateMemory: %v", err)
	}

	content := pm.AutoLoad()
	if len(content) > MaxPromptMemoryChars {
		t.Errorf("AutoLoad should truncate to %d chars, got %d", MaxPromptMemoryChars, len(content))
	}
}

func TestUpdateMemory(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	newContent := "# Updated Memory\n\nTest content"
	if err := pm.UpdateMemory(newContent); err != nil {
		t.Fatalf("UpdateMemory: %v", err)
	}

	if got := pm.GetMemoryContent(); got != newContent {
		t.Errorf("GetMemoryContent = %q, want %q", got, newContent)
	}
}

func TestUpdateUserProfile(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	newProfile := "# Updated Profile\n\nTest user"
	if err := pm.UpdateUserProfile(newProfile); err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}

	if got := pm.GetUserContent(); got != newProfile {
		t.Errorf("GetUserContent = %q, want %q", got, newProfile)
	}
}

func TestReload(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	// External modification
	memPath := filepath.Join(dir, "MEMORY.md")
	extContent := "# External Edit\n\nModified outside"
	if err := os.WriteFile(memPath, []byte(extContent), 0644); err != nil {
		t.Fatalf("external write: %v", err)
	}

	if err := pm.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	if got := pm.GetMemoryContent(); got != extContent {
		t.Errorf("after Reload, GetMemoryContent = %q, want %q", got, extContent)
	}
}

func TestAppendToSection(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	// Append to existing section
	if err := pm.AppendToSection("Learned Patterns", "- always run tests after changes"); err != nil {
		t.Fatalf("AppendToSection: %v", err)
	}

	content := pm.GetMemoryContent()
	if content == "" {
		t.Error("content should not be empty after append")
	}
}

func TestAppendToNewSection(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	if err := pm.AppendToSection("New Section", "- new item"); err != nil {
		t.Fatalf("AppendToSection new: %v", err)
	}

	content := pm.GetMemoryContent()
	if content == "" {
		t.Error("content should not be empty after append to new section")
	}
}

func TestGetPaths(t *testing.T) {
	dir := t.TempDir()
	pm, err := NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	if pm.GetMemoryPath() != filepath.Join(dir, "MEMORY.md") {
		t.Errorf("GetMemoryPath = %q, want %q", pm.GetMemoryPath(), filepath.Join(dir, "MEMORY.md"))
	}
	if pm.GetUserPath() != filepath.Join(dir, "USER.md") {
		t.Errorf("GetUserPath = %q, want %q", pm.GetUserPath(), filepath.Join(dir, "USER.md"))
	}
}
