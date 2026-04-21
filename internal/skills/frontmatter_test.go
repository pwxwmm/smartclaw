package skills

import (
	"strings"
	"testing"
)

func TestParseSKILLFrontmatter_WithFrontmatter(t *testing.T) {
	input := `---
name: code-review
version: "1.0"
description: "Review code changes with best practices"
author: smartclaw
platforms: [cli, web, telegram]
tags: [code, review, quality]
tools: [read_file, grep, ast_grep]
triggers: [/review, /code-review]
requires: []
config_vars:
  - name: strictness
    type: enum
    default: normal
    description: "Review strictness level"
    required: false
    enum_values: [relaxed, normal, strict]
  - name: max_suggestions
    type: int
    default: 5
    description: "Maximum number of suggestions"
    required: false
slash_commands:
  - name: /review
    description: "Review staged changes"
    template: "Review the current code changes with {{strictness}} strictness, max {{max_suggestions}} suggestions"
---
# Code Review

Some body content here.
`

	schema, body, err := ParseSKILLFrontmatter(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}

	if schema.Name != "code-review" {
		t.Errorf("name: got %q, want %q", schema.Name, "code-review")
	}
	if schema.Version != "1.0" {
		t.Errorf("version: got %q, want %q", schema.Version, "1.0")
	}
	if schema.Description != "Review code changes with best practices" {
		t.Errorf("description: got %q", schema.Description)
	}
	if schema.Author != "smartclaw" {
		t.Errorf("author: got %q", schema.Author)
	}

	wantPlatforms := []string{"cli", "web", "telegram"}
	if len(schema.Platforms) != len(wantPlatforms) {
		t.Fatalf("platforms: got %v, want %v", schema.Platforms, wantPlatforms)
	}
	for i, p := range wantPlatforms {
		if schema.Platforms[i] != p {
			t.Errorf("platforms[%d]: got %q, want %q", i, schema.Platforms[i], p)
		}
	}

	if len(schema.ConfigVars) != 2 {
		t.Fatalf("config_vars: got %d, want 2", len(schema.ConfigVars))
	}
	if schema.ConfigVars[0].Name != "strictness" {
		t.Errorf("config_vars[0].name: got %q", schema.ConfigVars[0].Name)
	}
	if schema.ConfigVars[0].Type != "enum" {
		t.Errorf("config_vars[0].type: got %q", schema.ConfigVars[0].Type)
	}
	if schema.ConfigVars[0].Default != "normal" {
		t.Errorf("config_vars[0].default: got %v", schema.ConfigVars[0].Default)
	}
	if len(schema.ConfigVars[0].EnumValues) != 3 {
		t.Errorf("config_vars[0].enum_values: got %v", schema.ConfigVars[0].EnumValues)
	}

	if schema.ConfigVars[1].Name != "max_suggestions" {
		t.Errorf("config_vars[1].name: got %q", schema.ConfigVars[1].Name)
	}
	if schema.ConfigVars[1].Type != "int" {
		t.Errorf("config_vars[1].type: got %q", schema.ConfigVars[1].Type)
	}

	if len(schema.SlashCommands) != 1 {
		t.Fatalf("slash_commands: got %d, want 1", len(schema.SlashCommands))
	}
	if schema.SlashCommands[0].Name != "/review" {
		t.Errorf("slash_commands[0].name: got %q", schema.SlashCommands[0].Name)
	}
	if !strings.Contains(schema.SlashCommands[0].Template, "{{strictness}}") {
		t.Errorf("slash_commands[0].template: expected {{strictness}} placeholder, got %q", schema.SlashCommands[0].Template)
	}

	if !strings.Contains(body, "# Code Review") {
		t.Errorf("body: expected markdown body, got %q", body)
	}
}

func TestParseSKILLFrontmatter_NoFrontmatter(t *testing.T) {
	input := `# Code Review

Some plain markdown without frontmatter.
`

	schema, body, err := ParseSKILLFrontmatter(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema != nil {
		t.Error("expected nil schema when no frontmatter present")
	}
	if body != input {
		t.Error("expected original content returned as body")
	}
}

func TestParseSKILLFrontmatter_MalformedYAML(t *testing.T) {
	input := `---
name: [broken yaml
invalid: {structure
---
Some body
`

	_, body, err := ParseSKILLFrontmatter(input)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
	if body != input {
		t.Error("expected original content returned on error")
	}
}

func TestValidateSchema_MissingRequired(t *testing.T) {
	schema := &SkillSchema{}
	errs := ValidateSchema(schema)

	found := map[string]bool{}
	for _, e := range errs {
		found[e.Field] = true
	}

	for _, f := range []string{"name", "version", "description"} {
		if !found[f] {
			t.Errorf("expected validation error for field %q", f)
		}
	}
}

func TestValidateSchema_InvalidPlatform(t *testing.T) {
	schema := &SkillSchema{
		Name:        "test",
		Version:     "1.0",
		Description: "desc",
		Platforms:   []string{"cli", "carrier_pigeon"},
	}
	errs := ValidateSchema(schema)

	found := false
	for _, e := range errs {
		if e.Field == "platforms" && strings.Contains(e.Message, "carrier_pigeon") {
			found = true
		}
	}
	if !found {
		t.Error("expected validation error for invalid platform 'carrier_pigeon'")
	}
}

func TestValidateSchema_InvalidConfigType(t *testing.T) {
	schema := &SkillSchema{
		Name:        "test",
		Version:     "1.0",
		Description: "desc",
		ConfigVars: []ConfigVar{
			{Name: "x", Type: "float"},
		},
	}
	errs := ValidateSchema(schema)

	found := false
	for _, e := range errs {
		if strings.Contains(e.Field, "config_vars") && strings.Contains(e.Message, "float") {
			found = true
		}
	}
	if !found {
		t.Error("expected validation error for invalid config type 'float'")
	}
}

func TestValidateSchema_EnumWithoutValues(t *testing.T) {
	schema := &SkillSchema{
		Name:        "test",
		Version:     "1.0",
		Description: "desc",
		ConfigVars: []ConfigVar{
			{Name: "mode", Type: "enum"},
		},
	}
	errs := ValidateSchema(schema)

	found := false
	for _, e := range errs {
		if strings.Contains(e.Field, "enum_values") {
			found = true
		}
	}
	if !found {
		t.Error("expected validation error for enum without enum_values")
	}
}

func TestValidateSchema_SlashCommandMissingTemplate(t *testing.T) {
	schema := &SkillSchema{
		Name:        "test",
		Version:     "1.0",
		Description: "desc",
		SlashCommands: []SlashCommand{
			{Name: "/foo", Description: "desc"},
		},
	}
	errs := ValidateSchema(schema)

	found := false
	for _, e := range errs {
		if strings.Contains(e.Field, "template") {
			found = true
		}
	}
	if !found {
		t.Error("expected validation error for slash command without template")
	}
}

func TestValidateSchema_Valid(t *testing.T) {
	schema := &SkillSchema{
		Name:        "test",
		Version:     "1.0",
		Description: "A test skill",
		Platforms:   []string{"cli", "web"},
		ConfigVars: []ConfigVar{
			{Name: "mode", Type: "enum", EnumValues: []string{"a", "b"}, Default: "a"},
		},
	}
	errs := ValidateSchema(schema)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestIsPlatformAllowed_Allowed(t *testing.T) {
	schema := &SkillSchema{Platforms: []string{"cli", "web"}}
	if !IsPlatformAllowed(schema, "cli") {
		t.Error("expected cli to be allowed")
	}
	if !IsPlatformAllowed(schema, "web") {
		t.Error("expected web to be allowed")
	}
}

func TestIsPlatformAllowed_Denied(t *testing.T) {
	schema := &SkillSchema{Platforms: []string{"cli"}}
	if IsPlatformAllowed(schema, "telegram") {
		t.Error("expected telegram to be denied")
	}
}

func TestIsPlatformAllowed_EmptyMeansAll(t *testing.T) {
	schema := &SkillSchema{Platforms: nil}
	if !IsPlatformAllowed(schema, "discord") {
		t.Error("expected empty platforms to allow all")
	}
	if !IsPlatformAllowed(schema, "cli") {
		t.Error("expected empty platforms to allow all")
	}
}

func TestResolveConfigVars_Defaults(t *testing.T) {
	schema := &SkillSchema{
		ConfigVars: []ConfigVar{
			{Name: "strictness", Type: "enum", Default: "normal", EnumValues: []string{"relaxed", "normal", "strict"}},
			{Name: "max_suggestions", Type: "int", Default: 5},
		},
	}
	resolved, errs := ResolveConfigVars(schema, map[string]any{})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if resolved["strictness"] != "normal" {
		t.Errorf("strictness: got %v, want normal", resolved["strictness"])
	}
	if resolved["max_suggestions"] != 5 {
		t.Errorf("max_suggestions: got %v, want 5", resolved["max_suggestions"])
	}
}

func TestResolveConfigVars_ProvidedOverrides(t *testing.T) {
	schema := &SkillSchema{
		ConfigVars: []ConfigVar{
			{Name: "strictness", Type: "enum", Default: "normal", EnumValues: []string{"relaxed", "normal", "strict"}},
			{Name: "max_suggestions", Type: "int", Default: 5},
		},
	}
	resolved, errs := ResolveConfigVars(schema, map[string]any{
		"strictness":     "strict",
		"max_suggestions": 10,
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if resolved["strictness"] != "strict" {
		t.Errorf("strictness: got %v, want strict", resolved["strictness"])
	}
	if resolved["max_suggestions"] != 10 {
		t.Errorf("max_suggestions: got %v, want 10", resolved["max_suggestions"])
	}
}

func TestResolveConfigVars_TypeChecking(t *testing.T) {
	schema := &SkillSchema{
		ConfigVars: []ConfigVar{
			{Name: "count", Type: "int"},
			{Name: "flag", Type: "bool"},
			{Name: "name", Type: "string"},
		},
	}

	_, errs := ResolveConfigVars(schema, map[string]any{
		"count": "not_a_number",
	})
	if len(errs) == 0 {
		t.Error("expected type error for non-numeric int")
	}

	_, errs = ResolveConfigVars(schema, map[string]any{
		"name": 123,
	})
	if len(errs) == 0 {
		t.Error("expected type error for non-string name")
	}

	_, errs = ResolveConfigVars(schema, map[string]any{
		"flag": "not_bool",
	})
	if len(errs) == 0 {
		t.Error("expected type error for non-bool flag")
	}
}

func TestResolveConfigVars_RequiredValidation(t *testing.T) {
	schema := &SkillSchema{
		ConfigVars: []ConfigVar{
			{Name: "api_key", Type: "string", Required: true},
		},
	}
	_, errs := ResolveConfigVars(schema, map[string]any{})
	if len(errs) == 0 {
		t.Error("expected error for missing required config var")
	}
}

func TestResolveConfigVars_RequiredWithDefault(t *testing.T) {
	schema := &SkillSchema{
		ConfigVars: []ConfigVar{
			{Name: "level", Type: "string", Required: true, Default: "info"},
		},
	}
	resolved, errs := ResolveConfigVars(schema, map[string]any{})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if resolved["level"] != "info" {
		t.Errorf("level: got %v, want info", resolved["level"])
	}
}

func TestResolveConfigVars_EnumValidation(t *testing.T) {
	schema := &SkillSchema{
		ConfigVars: []ConfigVar{
			{Name: "mode", Type: "enum", EnumValues: []string{"fast", "slow"}},
		},
	}
	_, errs := ResolveConfigVars(schema, map[string]any{
		"mode": "medium",
	})
	if len(errs) == 0 {
		t.Error("expected error for value not in enum_values")
	}
}

func TestGenerateSlashCommands(t *testing.T) {
	schema := &SkillSchema{
		ConfigVars: []ConfigVar{
			{Name: "strictness", Type: "enum", Default: "normal", EnumValues: []string{"relaxed", "normal", "strict"}},
			{Name: "max_suggestions", Type: "int", Default: 5},
		},
		SlashCommands: []SlashCommand{
			{
				Name:        "/review",
				Description: "Review staged changes",
				Template:    "Review the current code changes with {{strictness}} strictness, max {{max_suggestions}} suggestions",
			},
		},
	}

	cmds := GenerateSlashCommands(schema)
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "/review" {
		t.Errorf("name: got %q, want /review", cmds[0].Name)
	}
	if cmds[0].Description != "Review staged changes" {
		t.Errorf("description: got %q", cmds[0].Description)
	}
	if !strings.Contains(cmds[0].PromptTemplate, "normal") {
		t.Errorf("expected defaults resolved in template, got %q", cmds[0].PromptTemplate)
	}
	if !strings.Contains(cmds[0].PromptTemplate, "5") {
		t.Errorf("expected defaults resolved in template, got %q", cmds[0].PromptTemplate)
	}
	if cmds[0].ConfigTemplate != schema.SlashCommands[0].Template {
		t.Errorf("config_template: got %q, want %q", cmds[0].ConfigTemplate, schema.SlashCommands[0].Template)
	}
}

func TestRenderSKILLMarkdown_Roundtrip(t *testing.T) {
	original := `---
name: test-skill
version: "2.0"
description: A test skill
author: tester
platforms:
  - cli
  - web
tags:
  - test
tools:
  - bash
triggers:
  - /test
config_vars:
  - name: level
    type: string
    default: info
    description: Logging level
    required: false
slash_commands:
  - name: /test
    description: Run test
    template: Run test at {{level}} level
---
# Test Skill Body

Some instructions here.
`

	schema, body, err := ParseSKILLFrontmatter(original)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if schema == nil {
		t.Fatal("expected schema")
	}

	rendered, err := RenderSKILLMarkdown(schema, body)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	schema2, body2, err := ParseSKILLFrontmatter(rendered)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if schema2 == nil {
		t.Fatal("expected schema2")
	}

	if schema2.Name != schema.Name {
		t.Errorf("name: got %q, want %q", schema2.Name, schema.Name)
	}
	if schema2.Version != schema.Version {
		t.Errorf("version: got %q, want %q", schema2.Version, schema.Version)
	}
	if schema2.Description != schema.Description {
		t.Errorf("description: got %q, want %q", schema2.Description, schema.Description)
	}
	if schema2.Author != schema.Author {
		t.Errorf("author: got %q, want %q", schema2.Author, schema.Author)
	}
	if len(schema2.Platforms) != len(schema.Platforms) {
		t.Errorf("platforms: got %v, want %v", schema2.Platforms, schema.Platforms)
	}
	if len(schema2.ConfigVars) != len(schema.ConfigVars) {
		t.Errorf("config_vars: got %d, want %d", len(schema2.ConfigVars), len(schema.ConfigVars))
	}
	if len(schema2.SlashCommands) != len(schema.SlashCommands) {
		t.Errorf("slash_commands: got %d, want %d", len(schema2.SlashCommands), len(schema.SlashCommands))
	}

	if !strings.Contains(body2, "# Test Skill Body") {
		t.Errorf("body roundtrip: got %q", body2)
	}
}

func TestRenderSKILLMarkdown_NilSchema(t *testing.T) {
	result, err := RenderSKILLMarkdown(nil, "body content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "body content" {
		t.Errorf("expected passthrough of body, got %q", result)
	}
}

func TestSkill_PlatformAllowed(t *testing.T) {
	skill := &Skill{
		Name: "test",
		Schema: &SkillSchema{
			Platforms: []string{"cli", "web"},
		},
	}
	if !skill.PlatformAllowed("cli") {
		t.Error("expected cli allowed")
	}
	if skill.PlatformAllowed("telegram") {
		t.Error("expected telegram denied")
	}
}

func TestSkill_PlatformAllowed_NoSchema(t *testing.T) {
	skill := &Skill{Name: "test"}
	if !skill.PlatformAllowed("anything") {
		t.Error("expected all platforms allowed when no schema")
	}
}

func TestSkill_ResolveConfig(t *testing.T) {
	skill := &Skill{
		Name: "test",
		Schema: &SkillSchema{
			ConfigVars: []ConfigVar{
				{Name: "level", Type: "string", Default: "info"},
			},
		},
	}
	resolved, errs := skill.ResolveConfig(map[string]any{})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if resolved["level"] != "info" {
		t.Errorf("level: got %v, want info", resolved["level"])
	}
}

func TestSkill_ResolveConfig_NoSchema(t *testing.T) {
	skill := &Skill{Name: "test"}
	provided := map[string]any{"key": "value"}
	resolved, errs := skill.ResolveConfig(provided)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if resolved["key"] != "value" {
		t.Errorf("expected passthrough, got %v", resolved)
	}
}

func TestCoalesce(t *testing.T) {
	if coalesce("", "", "fallback") != "fallback" {
		t.Error("expected first non-empty value")
	}
	if coalesce("first", "second") != "first" {
		t.Error("expected first non-empty value")
	}
	if coalesce("", "", "") != "" {
		t.Error("expected empty string when all empty")
	}
}
