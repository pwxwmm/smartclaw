package skills

import (
	"context"
	"testing"
)

func TestBundledSkillDefinitions(t *testing.T) {
	if len(BundledSkillDefinitions) == 0 {
		t.Error("Expected bundled skill definitions to be populated")
	}

	expectedSkills := []string{
		"code-review", "git-expert", "test-generator", "documentation",
		"refactoring", "debugger", "api-designer", "performance",
		"security", "deployment", "batch", "loop", "remember",
		"verify", "skillify", "simplify", "stuck", "claude-api",
		"keybindings", "update-config",
	}

	for _, name := range expectedSkills {
		if _, ok := BundledSkillDefinitions[name]; !ok {
			t.Errorf("Expected bundled skill '%s' not found", name)
		}
	}
}

func TestGetBundledSkillNames(t *testing.T) {
	names := GetBundledSkillNames()
	if len(names) < 15 {
		t.Errorf("Expected at least 15 bundled skills, got %d", len(names))
	}
}

func TestGetBundledSkill(t *testing.T) {
	skill := GetBundledSkill("code-review")
	if skill == nil {
		t.Error("Expected to find code-review skill")
	}

	if skill.Name != "code-review" {
		t.Errorf("Expected skill name 'code-review', got '%s'", skill.Name)
	}

	if len(skill.Triggers) == 0 {
		t.Error("Expected code-review skill to have triggers")
	}

	if len(skill.Tools) == 0 {
		t.Error("Expected code-review skill to have tools")
	}
}

func TestGetBundledSkillNotFound(t *testing.T) {
	skill := GetBundledSkill("nonexistent")
	if skill != nil {
		t.Error("Expected nil for nonexistent skill")
	}
}

func TestGetBundledSkillsByTag(t *testing.T) {
	skills := GetBundledSkillsByTag("code")
	if len(skills) == 0 {
		t.Error("Expected to find skills with 'code' tag")
	}

	skills = GetBundledSkillsByTag("nonexistent-tag")
	if len(skills) != 0 {
		t.Error("Expected no skills for nonexistent tag")
	}
}

func TestSkillManager(t *testing.T) {
	manager := NewSkillManager()
	if manager == nil {
		t.Fatal("Expected non-nil SkillManager")
	}

	skills := manager.List()
	if len(skills) == 0 {
		t.Error("Expected SkillManager to have skills loaded")
	}
}

func TestSkillManagerGetBundled(t *testing.T) {
	manager := NewSkillManager()

	skill := manager.Get("code-review")
	if skill == nil {
		t.Error("Expected to find bundled skill 'code-review'")
	}

	if skill.Source != "bundled" {
		t.Errorf("Expected skill source 'bundled', got '%s'", skill.Source)
	}
}

func TestSkillManagerListBundled(t *testing.T) {
	manager := NewSkillManager()

	bundled := manager.ListBundled()
	if len(bundled) < 15 {
		t.Errorf("Expected at least 15 bundled skills, got %d", len(bundled))
	}
}

func TestSkillManagerEnableDisable(t *testing.T) {
	manager := NewSkillManager()

	err := manager.Disable("code-review")
	if err != nil {
		t.Errorf("Expected no error disabling skill, got %v", err)
	}

	skill := manager.Get("code-review")
	if skill.Enabled {
		t.Error("Expected skill to be disabled")
	}

	err = manager.Enable("code-review")
	if err != nil {
		t.Errorf("Expected no error enabling skill, got %v", err)
	}

	skill = manager.Get("code-review")
	if !skill.Enabled {
		t.Error("Expected skill to be enabled")
	}
}

func TestSkillManagerSearch(t *testing.T) {
	manager := NewSkillManager()

	results := manager.Search("review")
	if len(results) == 0 {
		t.Error("Expected to find skills matching 'review'")
	}

	results = manager.Search("git")
	if len(results) == 0 {
		t.Error("Expected to find skills matching 'git'")
	}
}

func TestSkillManagerListByTag(t *testing.T) {
	manager := NewSkillManager()

	skills := manager.ListByTag("testing")
	if len(skills) == 0 {
		t.Error("Expected to find skills with 'testing' tag")
	}
}

func TestMcpSkillBuilderPipeline(t *testing.T) {
	pipeline := NewMcpSkillBuilderPipeline(nil, nil)
	if pipeline == nil {
		t.Fatal("Expected non-nil McpSkillBuilderPipeline")
	}

	config := McpSkillBuilderConfig{
		ServerName: "test-server",
		Prefix:     "test",
		Template:   TemplateDefault,
		Tags:       []string{"test"},
	}

	pipeline.RegisterBuilder(config)

	builtSkills := pipeline.GetBuiltSkills()
	if len(builtSkills) != 0 {
		t.Error("Expected no built skills before BuildSkills call")
	}
}

func TestMcpSkillBuilderPipelineRegisterUnregister(t *testing.T) {
	pipeline := NewMcpSkillBuilderPipeline(nil, nil)

	config := McpSkillBuilderConfig{
		ServerName: "test-server",
		Template:   TemplateCode,
	}

	pipeline.RegisterBuilder(config)
	pipeline.UnregisterBuilder("test-server")

	pipeline.RefreshSkills(context.Background())
}

func TestSkillParsing(t *testing.T) {
	manager := NewSkillManager()

	skill := manager.Get("code-review")
	if skill == nil {
		t.Fatal("Expected to find code-review skill")
	}

	if skill.Description == "" {
		t.Error("Expected skill to have description")
	}

	if len(skill.Tags) == 0 {
		t.Error("Expected skill to have tags")
	}
}

func TestSkillContent(t *testing.T) {
	manager := NewSkillManager()

	content, err := manager.GetContent("code-review")
	if err != nil {
		t.Errorf("Expected no error getting skill content, got %v", err)
	}

	if content == "" {
		t.Error("Expected non-empty skill content")
	}

	_, err = manager.GetContent("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent skill")
	}
}

func TestDefaultPipeline(t *testing.T) {
	if defaultPipeline != nil {
		t.Error("Expected defaultPipeline to be nil before initialization")
	}

	InitMcpSkillBuilderPipeline(nil, nil)
	if GetMcpSkillBuilderPipeline() == nil {
		t.Error("Expected non-nil defaultPipeline after initialization")
	}
}

func TestRegisterMcpSkillBuilderWithoutInit(t *testing.T) {
	defaultPipeline = nil

	err := RegisterMcpSkillBuilder(McpSkillBuilderConfig{})
	if err == nil {
		t.Error("Expected error when registering without initialization")
	}
}

func TestBuildMcpSkillsWithoutInit(t *testing.T) {
	defaultPipeline = nil

	err := BuildMcpSkills(context.Background())
	if err == nil {
		t.Error("Expected error when building skills without initialization")
	}
}

func TestBundledSkillContent(t *testing.T) {
	for name, skill := range BundledSkillDefinitions {
		if skill.Name != name {
			t.Errorf("Skill name mismatch: expected '%s', got '%s'", name, skill.Name)
		}

		if skill.Content == "" {
			t.Errorf("Skill '%s' has empty content", name)
		}

		if skill.Description == "" {
			t.Errorf("Skill '%s' has empty description", name)
		}
	}
}
