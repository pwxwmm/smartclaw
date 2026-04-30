package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillToolName(t *testing.T) {
	tool := &SkillTool{}
	if tool.Name() != "skill" {
		t.Errorf("Expected name 'skill', got '%s'", tool.Name())
	}
}

func TestSkillToolDescription(t *testing.T) {
	tool := &SkillTool{}
	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestSkillToolInputSchema(t *testing.T) {
	tool := &SkillTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
	if schema["type"] != "object" {
		t.Error("Expected object type in schema")
	}
}

func TestSkillToolMissingName(t *testing.T) {
	tool := &SkillTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing name")
	}
}

func TestSkillToolEmptyName(t *testing.T) {
	tool := &SkillTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"name": "",
	})
	if err == nil {
		t.Error("Expected error for empty name")
	}
}

func TestSkillToolBundledHelp(t *testing.T) {
	tool := &SkillTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "help",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Error("Expected success=true for bundled skill")
	}
	if resultMap["skill_name"] != "help" {
		t.Errorf("Expected skill_name='help', got %v", resultMap["skill_name"])
	}
	content, _ := resultMap["content"].(string)
	if !strings.Contains(content, "Help") {
		t.Error("Bundled help skill should contain Help content")
	}
}

func TestSkillToolBundledCommit(t *testing.T) {
	tool := &SkillTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "commit",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Error("Expected success=true for bundled skill")
	}
	content, _ := resultMap["content"].(string)
	if !strings.Contains(content, "Commit") {
		t.Error("Bundled commit skill should contain Commit content")
	}
}

func TestSkillToolBundledGitMaster(t *testing.T) {
	tool := &SkillTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "git-master",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Error("Expected success=true for bundled git-master skill")
	}
}

func TestSkillToolNotFound(t *testing.T) {
	tool := &SkillTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "nonexistent-skill-xyz",
	})
	if err != nil {
		t.Fatalf("Execute should not return error for not found: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Error("Expected success=false for not found skill")
	}
	if !strings.Contains(resultMap["error"].(string), "not found") {
		t.Errorf("Expected 'not found' in error, got %v", resultMap["error"])
	}
}

func TestSkillToolStripSlashPrefix(t *testing.T) {
	tool := &SkillTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "/help",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Error("Should strip / prefix and find help skill")
	}
}

func TestSkillToolWithUserMessage(t *testing.T) {
	tool := &SkillTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name":         "help",
		"user_message": "some context",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["user_message"] != "some context" {
		t.Errorf("Expected user_message to be passed through, got %v", resultMap["user_message"])
	}
}

func TestSkillToolLoadFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test Skill\n\nCustom skill content"), 0644)

	tool := &SkillTool{skillsDir: tmpDir}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "test-skill",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Error("Expected success=true for disk skill")
	}
	content, _ := resultMap["content"].(string)
	if !strings.Contains(content, "Custom skill content") {
		t.Errorf("Expected custom skill content, got %q", content)
	}
}

func TestSkillToolLoadFromDiskMDFile(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "my-skill.md"), []byte("# My Skill\n\nFrom .md file"), 0644)

	tool := &SkillTool{skillsDir: tmpDir}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "my-skill",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Error("Expected success=true for .md file skill")
	}
}

func TestNewSkillTool(t *testing.T) {
	tool := NewSkillTool()
	if tool == nil {
		t.Fatal("NewSkillTool returned nil")
	}
	if tool.skillsDir == "" {
		t.Error("skillsDir should not be empty")
	}
}
