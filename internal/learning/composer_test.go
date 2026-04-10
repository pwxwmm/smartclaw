package learning

import (
	"testing"
)

func TestSkillComposer_Compose(t *testing.T) {
	composer := NewSkillComposer(nil)

	steps := []SkillStep{
		{SkillName: "refactoring", Description: "Refactor the code"},
		{SkillName: "test-generator", Description: "Generate tests"},
		{SkillName: "deployment", Description: "Deploy"},
	}

	workflow := composer.Compose(steps)
	if workflow == nil {
		t.Fatal("expected non-nil workflow")
	}
	if workflow.Name != "refactoring_test-generator_deployment" {
		t.Errorf("unexpected workflow name: %s", workflow.Name)
	}
	if len(workflow.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(workflow.Steps))
	}
	if len(workflow.Dependencies) != 2 {
		t.Errorf("expected 2 dependency edges, got %d", len(workflow.Dependencies))
	}
}

func TestSkillComposer_ComposeEmpty(t *testing.T) {
	composer := NewSkillComposer(nil)
	workflow := composer.Compose(nil)
	if workflow != nil {
		t.Error("expected nil for empty steps")
	}
}

func TestSkillComposer_ComposeDependencies(t *testing.T) {
	composer := NewSkillComposer(nil)

	steps := []SkillStep{
		{SkillName: "step-a"},
		{SkillName: "step-b"},
		{SkillName: "step-c"},
	}

	workflow := composer.Compose(steps)

	if _, ok := workflow.Dependencies["step_2_step-b"]; !ok {
		t.Error("expected step_2 to depend on step_1")
	}
	if _, ok := workflow.Dependencies["step_3_step-c"]; !ok {
		t.Error("expected step_3 to depend on step_2")
	}
	if _, ok := workflow.Dependencies["step_1_step-a"]; ok {
		t.Error("step_1 should have no dependencies")
	}
}

func TestSkillComposer_Decompose_RefactorDeploy(t *testing.T) {
	composer := NewSkillComposer(nil)

	steps := composer.Decompose("refactor the auth module and deploy it")
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	if steps[0].SkillName != "refactoring" {
		t.Errorf("expected first step 'refactoring', got %s", steps[0].SkillName)
	}
	if steps[2].SkillName != "deployment" {
		t.Errorf("expected last step 'deployment', got %s", steps[2].SkillName)
	}
}

func TestSkillComposer_Decompose_FixDeploy(t *testing.T) {
	composer := NewSkillComposer(nil)

	steps := composer.Decompose("fix the bug and deploy")
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	if steps[0].SkillName != "debugger" {
		t.Errorf("expected first step 'debugger', got %s", steps[0].SkillName)
	}
}

func TestSkillComposer_Decompose_Fallback(t *testing.T) {
	composer := NewSkillComposer(nil)

	steps := composer.Decompose("something random without keywords")
	if len(steps) == 0 {
		t.Fatal("expected at least one fallback step")
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"refactor the code", 1},
		{"test and deploy", 2},
		{"random text", 1},
	}

	for _, tt := range tests {
		got := extractKeywords(tt.input)
		if len(got) < tt.want {
			t.Errorf("extractKeywords(%q) = %d keywords, want at least %d", tt.input, len(got), tt.want)
		}
	}
}

func TestMapKeywordToSkill(t *testing.T) {
	tests := []struct {
		keyword string
		want    string
	}{
		{"refactor", "refactoring"},
		{"test", "test-generator"},
		{"deploy", "deployment"},
		{"debug", "debugger"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		got := mapKeywordToSkill(tt.keyword)
		if got != tt.want {
			t.Errorf("mapKeywordToSkill(%q) = %q, want %q", tt.keyword, got, tt.want)
		}
	}
}
