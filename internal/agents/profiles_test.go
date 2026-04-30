package agents

import (
	"strings"
	"testing"

	"github.com/instructkr/smartclaw/internal/tui"
)

// --- Profile constructor tests ---

func TestBuildProfile(t *testing.T) {
	p := BuildProfile()

	if p.AgentType != "build" {
		t.Errorf("AgentType = %q, want %q", p.AgentType, "build")
	}
	if p.Tools != nil {
		t.Errorf("Tools = %v, want nil (all tools permitted)", p.Tools)
	}
	if p.DisallowedTools != nil {
		t.Errorf("DisallowedTools = %v, want nil", p.DisallowedTools)
	}
	if p.PermissionMode != tui.PermissionModeAsk {
		t.Errorf("PermissionMode = %q, want %q", p.PermissionMode, tui.PermissionModeAsk)
	}
	if p.Color != "blue" {
		t.Errorf("Color = %q, want %q", p.Color, "blue")
	}
	if p.Memory != tui.AgentMemoryProject {
		t.Errorf("Memory = %q, want %q", p.Memory, tui.AgentMemoryProject)
	}
	if p.MaxTurns != 0 {
		t.Errorf("MaxTurns = %d, want 0 (unlimited)", p.MaxTurns)
	}
	if p.Model != "" {
		t.Errorf("Model = %q, want empty string (use default)", p.Model)
	}
	if p.SystemPrompt == "" {
		t.Error("SystemPrompt is empty, want non-empty")
	}
	if p.WhenToUse == "" {
		t.Error("WhenToUse is empty, want non-empty")
	}
}

func TestPlanProfile(t *testing.T) {
	p := PlanProfile()

	if p.AgentType != "plan" {
		t.Errorf("AgentType = %q, want %q", p.AgentType, "plan")
	}

	expectedTools := []string{
		"read", "glob", "grep",
		"lsp_symbols", "lsp_find_references", "lsp_goto_definition",
		"ast_grep_search",
		"web_fetch", "web_search",
	}
	if len(p.Tools) != len(expectedTools) {
		t.Errorf("Tools length = %d, want %d", len(p.Tools), len(expectedTools))
	}
	toolSet := make(map[string]bool, len(p.Tools))
	for _, t := range p.Tools {
		toolSet[t] = true
	}
	for _, et := range expectedTools {
		if !toolSet[et] {
			t.Errorf("missing expected tool %q in Tools", et)
		}
	}

	expectedDisallowed := []string{"write", "edit", "bash", "docker_exec", "execute_code"}
	if len(p.DisallowedTools) != len(expectedDisallowed) {
		t.Errorf("DisallowedTools length = %d, want %d", len(p.DisallowedTools), len(expectedDisallowed))
	}
	for _, d := range expectedDisallowed {
		found := false
		for _, ad := range p.DisallowedTools {
			if ad == d {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected disallowed tool %q", d)
		}
	}

	if p.PermissionMode != tui.PermissionModeReadOnly {
		t.Errorf("PermissionMode = %q, want %q", p.PermissionMode, tui.PermissionModeReadOnly)
	}
	if p.MaxTurns != 15 {
		t.Errorf("MaxTurns = %d, want 15", p.MaxTurns)
	}
	if p.Color != "yellow" {
		t.Errorf("Color = %q, want %q", p.Color, "yellow")
	}
	if p.Memory != tui.AgentMemoryProject {
		t.Errorf("Memory = %q, want %q", p.Memory, tui.AgentMemoryProject)
	}
	if p.Model != "" {
		t.Errorf("Model = %q, want empty string", p.Model)
	}
	if p.SystemPrompt == "" {
		t.Error("SystemPrompt is empty, want non-empty")
	}
	if p.WhenToUse == "" {
		t.Error("WhenToUse is empty, want non-empty")
	}
}

func TestOpsProfile(t *testing.T) {
	p := OpsProfile()

	if p.AgentType != "ops" {
		t.Errorf("AgentType = %q, want %q", p.AgentType, "ops")
	}

	expectedTools := []string{
		"bash", "read", "write", "edit", "glob", "grep", "docker_exec",
	}
	if len(p.Tools) != len(expectedTools) {
		t.Errorf("Tools length = %d, want %d", len(p.Tools), len(expectedTools))
	}
	toolSet := make(map[string]bool, len(p.Tools))
	for _, t := range p.Tools {
		toolSet[t] = true
	}
	for _, et := range expectedTools {
		if !toolSet[et] {
			t.Errorf("missing expected tool %q in Tools", et)
		}
	}

	expectedDisallowed := []string{"execute_code"}
	if len(p.DisallowedTools) != len(expectedDisallowed) {
		t.Errorf("DisallowedTools length = %d, want %d", len(p.DisallowedTools), len(expectedDisallowed))
	}
	for _, d := range expectedDisallowed {
		found := false
		for _, ad := range p.DisallowedTools {
			if ad == d {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected disallowed tool %q", d)
		}
	}

	if p.PermissionMode != tui.PermissionModeAsk {
		t.Errorf("PermissionMode = %q, want %q", p.PermissionMode, tui.PermissionModeAsk)
	}
	if p.MaxTurns != 0 {
		t.Errorf("MaxTurns = %d, want 0 (unlimited)", p.MaxTurns)
	}
	if p.Color != "orange" {
		t.Errorf("Color = %q, want %q", p.Color, "orange")
	}
	if p.Memory != tui.AgentMemoryProject {
		t.Errorf("Memory = %q, want %q", p.Memory, tui.AgentMemoryProject)
	}
	if p.Model != "" {
		t.Errorf("Model = %q, want empty string", p.Model)
	}
	if p.SystemPrompt == "" {
		t.Error("SystemPrompt is empty, want non-empty")
	}
	if p.WhenToUse == "" {
		t.Error("WhenToUse is empty, want non-empty")
	}
}

// --- ProfileRegistry tests ---

func TestNewProfileRegistry(t *testing.T) {
	r := NewProfileRegistry()
	if r == nil {
		t.Fatal("NewProfileRegistry() returned nil")
	}
	if len(r.profiles) != 3 {
		t.Errorf("registry has %d profiles, want 3", len(r.profiles))
	}
}

func TestProfileRegistry_Get_Valid(t *testing.T) {
	r := NewProfileRegistry()

	tests := []struct {
		name      string
		agentType string
	}{
		{"build profile", "build"},
		{"plan profile", "plan"},
		{"ops profile", "ops"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := r.Get(tt.agentType)
			if err != nil {
				t.Fatalf("Get(%q) returned error: %v", tt.agentType, err)
			}
			if p.AgentType != tt.agentType {
				t.Errorf("Get(%q).AgentType = %q, want %q", tt.agentType, p.AgentType, tt.agentType)
			}
		})
	}
}

func TestProfileRegistry_Get_Invalid(t *testing.T) {
	r := NewProfileRegistry()

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("Get(\"nonexistent\") should return an error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want error containing \"not found\"", err.Error())
	}
}

func TestProfileRegistry_List(t *testing.T) {
	r := NewProfileRegistry()
	list := r.List()

	if len(list) != 3 {
		t.Fatalf("List() returned %d profiles, want 3", len(list))
	}

	found := make(map[string]bool)
	for _, p := range list {
		found[p.AgentType] = true
	}
	for _, name := range []string{"build", "plan", "ops"} {
		if !found[name] {
			t.Errorf("List() missing profile %q", name)
		}
	}
}

// --- ToAgentDefinition tests ---

func TestToAgentDefinition(t *testing.T) {
	p := &AgentProfile{
		AgentType:       "test",
		WhenToUse:       "test when to use",
		SystemPrompt:    "test system prompt",
		Tools:           []string{"read", "grep"},
		DisallowedTools: []string{"write"},
		Model:           "test-model",
		PermissionMode:  tui.PermissionModeAsk,
		Color:           "green",
		MaxTurns:        5,
		Memory:          tui.AgentMemoryLocal,
	}

	def := ToAgentDefinition(p)

	if def.AgentType != p.AgentType {
		t.Errorf("AgentType = %q, want %q", def.AgentType, p.AgentType)
	}
	if def.WhenToUse != p.WhenToUse {
		t.Errorf("WhenToUse = %q, want %q", def.WhenToUse, p.WhenToUse)
	}
	if def.SystemPrompt != p.SystemPrompt {
		t.Errorf("SystemPrompt = %q, want %q", def.SystemPrompt, p.SystemPrompt)
	}
	if len(def.Tools) != len(p.Tools) {
		t.Errorf("Tools length = %d, want %d", len(def.Tools), len(p.Tools))
	}
	if len(def.DisallowedTools) != len(p.DisallowedTools) {
		t.Errorf("DisallowedTools length = %d, want %d", len(def.DisallowedTools), len(p.DisallowedTools))
	}
	if def.Model != p.Model {
		t.Errorf("Model = %q, want %q", def.Model, p.Model)
	}
	if def.PermissionMode != p.PermissionMode {
		t.Errorf("PermissionMode = %q, want %q", def.PermissionMode, p.PermissionMode)
	}
	if def.Color != p.Color {
		t.Errorf("Color = %q, want %q", def.Color, p.Color)
	}
	if def.MaxTurns != p.MaxTurns {
		t.Errorf("MaxTurns = %d, want %d", def.MaxTurns, p.MaxTurns)
	}
	if def.Memory != p.Memory {
		t.Errorf("Memory = %q, want %q", def.Memory, p.Memory)
	}
	if def.Source != tui.AgentSourceBuiltIn {
		t.Errorf("Source = %q, want %q", def.Source, tui.AgentSourceBuiltIn)
	}
}

func TestToAgentDefinition_NilTools(t *testing.T) {
	p := BuildProfile()
	def := ToAgentDefinition(p)

	if def.Tools != nil {
		t.Errorf("Tools = %v, want nil", def.Tools)
	}
	if def.DisallowedTools != nil {
		t.Errorf("DisallowedTools = %v, want nil", def.DisallowedTools)
	}
	if def.Source != tui.AgentSourceBuiltIn {
		t.Errorf("Source = %q, want %q", def.Source, tui.AgentSourceBuiltIn)
	}
}

// --- Cross-profile invariant tests ---

func TestAllProfiles_SystemPromptsNonEmpty(t *testing.T) {
	profiles := []*AgentProfile{BuildProfile(), PlanProfile(), OpsProfile()}
	for _, p := range profiles {
		if p.SystemPrompt == "" {
			t.Errorf("profile %q has empty SystemPrompt", p.AgentType)
		}
	}
}

func TestAllProfiles_WhenToUseNonEmpty(t *testing.T) {
	profiles := []*AgentProfile{BuildProfile(), PlanProfile(), OpsProfile()}
	for _, p := range profiles {
		if p.WhenToUse == "" {
			t.Errorf("profile %q has empty WhenToUse", p.AgentType)
		}
	}
}

func TestPlanProfile_ReadOnlyToolsOnly(t *testing.T) {
	p := PlanProfile()

	writeTools := map[string]bool{
		"write":        true,
		"edit":         true,
		"bash":         true,
		"docker_exec":  true,
		"execute_code": true,
	}

	for _, tool := range p.Tools {
		if writeTools[tool] {
			t.Errorf("Plan profile Tools contains write/exec tool %q, should be read-only", tool)
		}
	}

	disallowedSet := make(map[string]bool, len(p.DisallowedTools))
	for _, d := range p.DisallowedTools {
		disallowedSet[d] = true
	}
	for wt := range writeTools {
		if !disallowedSet[wt] {
			t.Errorf("Plan profile DisallowedTools missing %q", wt)
		}
	}
}

func TestOpsProfile_HasBashButNotExecuteCode(t *testing.T) {
	p := OpsProfile()

	hasBash := false
	for _, tool := range p.Tools {
		if tool == "bash" {
			hasBash = true
			break
		}
	}
	if !hasBash {
		t.Error("Ops profile Tools missing \"bash\"")
	}

	for _, tool := range p.Tools {
		if tool == "execute_code" {
			t.Error("Ops profile Tools contains \"execute_code\", should not")
		}
	}

	hasExecuteCodeDisallowed := false
	for _, d := range p.DisallowedTools {
		if d == "execute_code" {
			hasExecuteCodeDisallowed = true
			break
		}
	}
	if !hasExecuteCodeDisallowed {
		t.Error("Ops profile DisallowedTools missing \"execute_code\"")
	}
}

// --- Table-driven profile field tests ---

func TestProfiles_AllFieldsCorrect(t *testing.T) {
	tests := []struct {
		name           string
		profile        *AgentProfile
		wantType       string
		wantPermMode   tui.PermissionMode
		wantColor      string
		wantMemory     tui.AgentMemoryScope
		wantMaxTurns   int
		wantModel      string
	}{
		{
			name:         "build",
			profile:      BuildProfile(),
			wantType:     "build",
			wantPermMode: tui.PermissionModeAsk,
			wantColor:    "blue",
			wantMemory:   tui.AgentMemoryProject,
			wantMaxTurns: 0,
			wantModel:    "",
		},
		{
			name:         "plan",
			profile:      PlanProfile(),
			wantType:     "plan",
			wantPermMode: tui.PermissionModeReadOnly,
			wantColor:    "yellow",
			wantMemory:   tui.AgentMemoryProject,
			wantMaxTurns: 15,
			wantModel:    "",
		},
		{
			name:         "ops",
			profile:      OpsProfile(),
			wantType:     "ops",
			wantPermMode: tui.PermissionModeAsk,
			wantColor:    "orange",
			wantMemory:   tui.AgentMemoryProject,
			wantMaxTurns: 0,
			wantModel:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.profile.AgentType != tt.wantType {
				t.Errorf("AgentType = %q, want %q", tt.profile.AgentType, tt.wantType)
			}
			if tt.profile.PermissionMode != tt.wantPermMode {
				t.Errorf("PermissionMode = %q, want %q", tt.profile.PermissionMode, tt.wantPermMode)
			}
			if tt.profile.Color != tt.wantColor {
				t.Errorf("Color = %q, want %q", tt.profile.Color, tt.wantColor)
			}
			if tt.profile.Memory != tt.wantMemory {
				t.Errorf("Memory = %q, want %q", tt.profile.Memory, tt.wantMemory)
			}
			if tt.profile.MaxTurns != tt.wantMaxTurns {
				t.Errorf("MaxTurns = %d, want %d", tt.profile.MaxTurns, tt.wantMaxTurns)
			}
			if tt.profile.Model != tt.wantModel {
				t.Errorf("Model = %q, want %q", tt.profile.Model, tt.wantModel)
			}
		})
	}
}
