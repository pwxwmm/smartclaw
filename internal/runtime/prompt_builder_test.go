package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPromptBuilderFreeze(t *testing.T) {
	pb := NewPromptBuilder()

	first := pb.Build()
	if first == "" {
		t.Fatal("Build() returned empty string")
	}

	pb.MarkDirty()
	if !pb.IsDirty() {
		t.Error("expected dirty after MarkDirty")
	}

	second := pb.Build()
	if second != first {
		t.Error("rebuild should produce same content with no changes")
	}
	if pb.IsDirty() {
		t.Error("expected clean after Build()")
	}
}

func TestPromptBuilderPersona(t *testing.T) {
	pb := NewPromptBuilder()

	pb.SetPersona("You are a test assistant.")
	prompt := pb.Build()

	if prompt != "You are a test assistant." {
		t.Errorf("expected custom persona, got %q", prompt)
	}
}

func TestPromptBuilderSkill(t *testing.T) {
	pb := NewPromptBuilder()

	pb.AddSkill("how to debug Go programs")
	prompt := pb.Build()

	if !contains(prompt, "how to debug Go programs") {
		t.Error("prompt should contain added skill")
	}
	if !contains(prompt, "<skill>") {
		t.Error("prompt should contain skill tags")
	}
}

func TestPromptBuilderContext(t *testing.T) {
	pb := NewPromptBuilder()

	pb.AddContext("This project uses Go 1.25")
	prompt := pb.Build()

	if !contains(prompt, "This project uses Go 1.25") {
		t.Error("prompt should contain added context")
	}
	if !contains(prompt, "<project_context>") {
		t.Error("prompt should contain context tags")
	}
}

func TestPromptBuilderRemoveSkill(t *testing.T) {
	pb := NewPromptBuilder()

	pb.AddSkill("skill A")
	pb.AddSkill("skill B")
	pb.Build()

	pb.RemoveSkill(0)
	if !pb.IsDirty() {
		t.Error("expected dirty after RemoveSkill")
	}

	prompt := pb.Build()
	if contains(prompt, "skill A") {
		t.Error("skill A should be removed")
	}
	if !contains(prompt, "skill B") {
		t.Error("skill B should still be present")
	}
}

func TestPromptBuilderMemoryFile(t *testing.T) {
	tmpDir := t.TempDir()
	smartclawDir := filepath.Join(tmpDir, ".smartclaw")
	os.MkdirAll(smartclawDir, 0755)

	os.WriteFile(filepath.Join(smartclawDir, "MEMORY.md"), []byte("User prefers Go"), 0644)
	os.WriteFile(filepath.Join(smartclawDir, "USER.md"), []byte("Senior developer"), 0644)

	pb := NewPromptBuilder()
	pb.homeDir = tmpDir
	pb.MarkDirty()

	prompt := pb.Build()
	if !contains(prompt, "User prefers Go") {
		t.Error("prompt should contain MEMORY.md content")
	}
	if !contains(prompt, "Senior developer") {
		t.Error("prompt should contain USER.md content")
	}
	if !contains(prompt, "<user_memory>") {
		t.Error("prompt should contain user_memory tags")
	}
	if !contains(prompt, "<user_profile>") {
		t.Error("prompt should contain user_profile tags")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
