package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// helper: create an AgentManager with temp dirs and built-in agents loaded
func newTestAgentManager(t *testing.T) *AgentManager {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "agents")
	projectDir := filepath.Join(tmpDir, "project")

	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: configPath,
		projectDir: projectDir,
	}
	am.loadBuiltInAgents()

	// Set default current agent (same logic as NewAgentManager)
	if general, exists := am.agents["general-purpose"]; exists {
		am.currentAgent = general
	}

	return am
}

// ──────────────────────────── Constants ────────────────────────────

func TestAgentSourceConstants(t *testing.T) {
	tests := []struct {
		name  string
		value AgentSource
		want  string
	}{
		{"BuiltIn", AgentSourceBuiltIn, "built-in"},
		{"UserSettings", AgentSourceUserSettings, "userSettings"},
		{"ProjectSettings", AgentSourceProjectSettings, "projectSettings"},
		{"Plugin", AgentSourcePlugin, "plugin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("AgentSource %s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestPermissionModeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value PermissionMode
		want  string
	}{
		{"Ask", PermissionModeAsk, "ask"},
		{"ReadOnly", PermissionModeReadOnly, "read-only"},
		{"WorkspaceWrite", PermissionModeWorkspaceWrite, "workspace-write"},
		{"DangerFullAccess", PermissionModeDangerFullAccess, "danger-full-access"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("PermissionMode %s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestAgentMemoryScopeConstants(t *testing.T) {
	tests := []struct {
		name  string
		value AgentMemoryScope
		want  string
	}{
		{"User", AgentMemoryUser, "user"},
		{"Project", AgentMemoryProject, "project"},
		{"Local", AgentMemoryLocal, "local"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.want {
				t.Errorf("AgentMemoryScope %s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestAgentIsolationModeConstants(t *testing.T) {
	if string(AgentIsolationWorktree) != "worktree" {
		t.Errorf("AgentIsolationWorktree = %q, want %q", AgentIsolationWorktree, "worktree")
	}
	if string(AgentIsolationRemote) != "remote" {
		t.Errorf("AgentIsolationRemote = %q, want %q", AgentIsolationRemote, "remote")
	}
}

// ──────────────────────────── NewAgentManager ────────────────────────────

func TestNewAgentManager_NonNilAgentsMap(t *testing.T) {
	am := newTestAgentManager(t)
	if am.agents == nil {
		t.Error("agents map is nil")
	}
}

func TestNewAgentManager_LoadsBuiltInAgents(t *testing.T) {
	am := newTestAgentManager(t)

	expectedAgents := []string{
		"general-purpose", "explore", "plan", "code-review",
		"test-engineer", "devops", "security", "architect",
		"refactor", "docs",
	}

	for _, name := range expectedAgents {
		agent, err := am.GetAgent(name)
		if err != nil {
			t.Errorf("built-in agent %q not found: %v", name, err)
			continue
		}
		if agent.Source != AgentSourceBuiltIn {
			t.Errorf("agent %q Source = %q, want %q", name, agent.Source, AgentSourceBuiltIn)
		}
		if agent.SystemPrompt == "" {
			t.Errorf("agent %q has empty SystemPrompt", name)
		}
		if agent.WhenToUse == "" {
			t.Errorf("agent %q has empty WhenToUse", name)
		}
	}
}

func TestNewAgentManager_DefaultCurrentAgentIsGeneralPurpose(t *testing.T) {
	am := newTestAgentManager(t)
	current := am.GetCurrentAgent()
	if current == nil {
		t.Fatal("currentAgent is nil")
	}
	if current.AgentType != "general-purpose" {
		t.Errorf("default currentAgent = %q, want %q", current.AgentType, "general-purpose")
	}
}

// ──────────────────────────── GetCurrentAgent / GetAgent ────────────────────────────

func TestGetCurrentAgent_Initial(t *testing.T) {
	am := newTestAgentManager(t)
	agent := am.GetCurrentAgent()
	if agent == nil {
		t.Fatal("GetCurrentAgent() returned nil")
	}
	if agent.AgentType != "general-purpose" {
		t.Errorf("GetCurrentAgent().AgentType = %q, want %q", agent.AgentType, "general-purpose")
	}
}

func TestGetAgent_ValidType(t *testing.T) {
	am := newTestAgentManager(t)
	agent, err := am.GetAgent("explore")
	if err != nil {
		t.Fatalf("GetAgent(%q) returned error: %v", "explore", err)
	}
	if agent.AgentType != "explore" {
		t.Errorf("GetAgent(%q).AgentType = %q, want %q", "explore", agent.AgentType, "explore")
	}
	if agent.Color != "green" {
		t.Errorf("GetAgent(%q).Color = %q, want %q", "explore", agent.Color, "green")
	}
}

func TestGetAgent_InvalidType(t *testing.T) {
	am := newTestAgentManager(t)
	_, err := am.GetAgent("nonexistent")
	if err == nil {
		t.Error("GetAgent(nonexistent) should return error")
	}
	if !strings.Contains(err.Error(), "agent not found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "agent not found")
	}
}

// ──────────────────────────── SetCurrentAgent ────────────────────────────

func TestSetCurrentAgent_ValidType(t *testing.T) {
	am := newTestAgentManager(t)
	err := am.SetCurrentAgent("plan")
	if err != nil {
		t.Fatalf("SetCurrentAgent(%q) returned error: %v", "plan", err)
	}
	current := am.GetCurrentAgent()
	if current.AgentType != "plan" {
		t.Errorf("after SetCurrentAgent(%q), currentAgent = %q, want %q", "plan", current.AgentType, "plan")
	}
}

func TestSetCurrentAgent_InvalidType(t *testing.T) {
	am := newTestAgentManager(t)
	err := am.SetCurrentAgent("nonexistent")
	if err == nil {
		t.Error("SetCurrentAgent(nonexistent) should return error")
	}
	// Current agent should remain unchanged
	current := am.GetCurrentAgent()
	if current.AgentType != "general-purpose" {
		t.Errorf("after failed switch, currentAgent = %q, want %q", current.AgentType, "general-purpose")
	}
}

func TestSetCurrentAgent_CallbackInvoked(t *testing.T) {
	am := newTestAgentManager(t)

	var callbackCalled bool
	var callbackAgent string
	am.SetOnAgentSwitch(func(agentType string) error {
		callbackCalled = true
		callbackAgent = agentType
		return nil
	})

	err := am.SetCurrentAgent("explore")
	if err != nil {
		t.Fatalf("SetCurrentAgent returned error: %v", err)
	}
	if !callbackCalled {
		t.Error("onAgentSwitch callback was not called")
	}
	if callbackAgent != "explore" {
		t.Errorf("callback agentType = %q, want %q", callbackAgent, "explore")
	}
}

func TestSetCurrentAgent_CallbackError(t *testing.T) {
	am := newTestAgentManager(t)

	am.SetOnAgentSwitch(func(agentType string) error {
		return os.ErrPermission
	})

	err := am.SetCurrentAgent("explore")
	if err == nil {
		t.Error("expected error when callback fails")
	}
	if !strings.Contains(err.Error(), "callback failed") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "callback failed")
	}
}

func TestSetOnAgentSwitch(t *testing.T) {
	am := newTestAgentManager(t)
	called := false
	am.SetOnAgentSwitch(func(agentType string) error {
		called = true
		return nil
	})
	am.SetCurrentAgent("docs")
	if !called {
		t.Error("SetOnAgentSwitch callback not invoked")
	}
}

// ──────────────────────────── ListAgents ────────────────────────────

func TestListAgents_ReturnsBuiltInAgents(t *testing.T) {
	am := newTestAgentManager(t)
	agents := am.ListAgents()

	if len(agents) < 10 {
		t.Errorf("ListAgents() returned %d agents, want at least 10", len(agents))
	}

	// Verify all built-in agents are present
	found := make(map[string]bool)
	for _, a := range agents {
		found[a.AgentType] = true
	}
	expectedBuiltIn := []string{
		"general-purpose", "explore", "plan", "code-review",
		"test-engineer", "devops", "security", "architect",
		"refactor", "docs",
	}
	for _, name := range expectedBuiltIn {
		if !found[name] {
			t.Errorf("built-in agent %q not in ListAgents()", name)
		}
	}
}

func TestListAgents_SortedBySourceThenName(t *testing.T) {
	am := newTestAgentManager(t)
	agents := am.ListAgents()

	for i := 1; i < len(agents); i++ {
		prev, curr := agents[i-1], agents[i]
		if prev.Source > curr.Source {
			t.Errorf("agents not sorted by source: %q (%s) before %q (%s)", prev.AgentType, prev.Source, curr.AgentType, curr.Source)
		}
		if prev.Source == curr.Source && prev.AgentType > curr.AgentType {
			t.Errorf("agents with same source not sorted by name: %q before %q", prev.AgentType, curr.AgentType)
		}
	}
}

// ──────────────────────────── ListAgentsBySource ────────────────────────────

func TestListAgentsBySource_GroupsBySource(t *testing.T) {
	am := newTestAgentManager(t)
	bySource := am.ListAgentsBySource()

	builtIn, ok := bySource[AgentSourceBuiltIn]
	if !ok {
		t.Fatal("no built-in agents in ListAgentsBySource()")
	}
	if len(builtIn) < 10 {
		t.Errorf("built-in agents count = %d, want at least 10", len(builtIn))
	}

	// Verify no non-built-in agents in built-in group
	for _, a := range builtIn {
		if a.Source != AgentSourceBuiltIn {
			t.Errorf("agent %q in built-in group has source %q", a.AgentType, a.Source)
		}
	}
}

// ──────────────────────────── GetSystemPrompt ────────────────────────────

func TestGetSystemPrompt(t *testing.T) {
	am := newTestAgentManager(t)
	prompt := am.GetSystemPrompt()
	if prompt == "" {
		t.Error("GetSystemPrompt() returned empty string")
	}
	if !strings.Contains(prompt, "helpful AI assistant") {
		t.Errorf("GetSystemPrompt() = %q, want to contain 'helpful AI assistant'", prompt)
	}
}

func TestGetSystemPrompt_AfterSwitch(t *testing.T) {
	am := newTestAgentManager(t)
	am.SetCurrentAgent("explore")
	prompt := am.GetSystemPrompt()
	if !strings.Contains(prompt, "exploration agent") {
		t.Errorf("after switching to explore, prompt = %q, want to contain 'exploration agent'", prompt)
	}
}

// ──────────────────────────── AgentDefinition struct ────────────────────────────

func TestAgentDefinition_FieldAssignments(t *testing.T) {
	agent := &AgentDefinition{
		AgentType:          "custom-agent",
		WhenToUse:          "Custom agent for testing",
		Tools:              []string{"read", "write", "edit"},
		DisallowedTools:    []string{"bash"},
		Skills:             []string{"testing"},
		McpServers:         []McpServerSpec{{Name: "test-mcp", Command: "test-cmd", Args: []string{"arg1"}}},
		Hooks:              HooksSettings{"pre": {{Type: "shell", Command: "echo hi"}}},
		Color:              "pink",
		Model:              "claude-opus-4-6",
		Effort:             "high",
		PermissionMode:     PermissionModeWorkspaceWrite,
		MaxTurns:           42,
		Filename:           "custom-agent",
		BaseDir:            "/tmp/agents",
		InitialPrompt:      "Hello!",
		Memory:             AgentMemoryProject,
		Isolation:          AgentIsolationWorktree,
		Background:         true,
		Source:             AgentSourceUserSettings,
		SystemPrompt:       "You are a custom agent.",
		RequiredMcpServers: []string{"test-mcp"},
		OmitClaudeMd:       true,
	}

	if agent.AgentType != "custom-agent" {
		t.Errorf("AgentType = %q, want %q", agent.AgentType, "custom-agent")
	}
	if agent.WhenToUse != "Custom agent for testing" {
		t.Errorf("WhenToUse = %q, want %q", agent.WhenToUse, "Custom agent for testing")
	}
	if len(agent.Tools) != 3 {
		t.Errorf("len(Tools) = %d, want 3", len(agent.Tools))
	}
	if len(agent.DisallowedTools) != 1 || agent.DisallowedTools[0] != "bash" {
		t.Errorf("DisallowedTools = %v, want [bash]", agent.DisallowedTools)
	}
	if len(agent.Skills) != 1 || agent.Skills[0] != "testing" {
		t.Errorf("Skills = %v, want [testing]", agent.Skills)
	}
	if len(agent.McpServers) != 1 || agent.McpServers[0].Name != "test-mcp" {
		t.Errorf("McpServers = %v, unexpected", agent.McpServers)
	}
	if len(agent.Hooks["pre"]) != 1 || agent.Hooks["pre"][0].Command != "echo hi" {
		t.Errorf("Hooks = %v, unexpected", agent.Hooks)
	}
	if agent.Color != "pink" {
		t.Errorf("Color = %q, want %q", agent.Color, "pink")
	}
	if agent.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", agent.Model, "claude-opus-4-6")
	}
	if agent.Effort != "high" {
		t.Errorf("Effort = %q, want %q", agent.Effort, "high")
	}
	if agent.PermissionMode != PermissionModeWorkspaceWrite {
		t.Errorf("PermissionMode = %q, want %q", agent.PermissionMode, PermissionModeWorkspaceWrite)
	}
	if agent.MaxTurns != 42 {
		t.Errorf("MaxTurns = %d, want 42", agent.MaxTurns)
	}
	if agent.Memory != AgentMemoryProject {
		t.Errorf("Memory = %q, want %q", agent.Memory, AgentMemoryProject)
	}
	if agent.Isolation != AgentIsolationWorktree {
		t.Errorf("Isolation = %q, want %q", agent.Isolation, AgentIsolationWorktree)
	}
	if !agent.Background {
		t.Error("Background = false, want true")
	}
	if agent.Source != AgentSourceUserSettings {
		t.Errorf("Source = %q, want %q", agent.Source, AgentSourceUserSettings)
	}
	if agent.SystemPrompt != "You are a custom agent." {
		t.Errorf("SystemPrompt = %q, want %q", agent.SystemPrompt, "You are a custom agent.")
	}
	if len(agent.RequiredMcpServers) != 1 || agent.RequiredMcpServers[0] != "test-mcp" {
		t.Errorf("RequiredMcpServers = %v, want [test-mcp]", agent.RequiredMcpServers)
	}
	if !agent.OmitClaudeMd {
		t.Error("OmitClaudeMd = false, want true")
	}
}

func TestAgentDefinition_JSONRoundTrip(t *testing.T) {
	agent := &AgentDefinition{
		AgentType:      "json-test",
		WhenToUse:      "JSON round-trip test",
		Source:         AgentSourceBuiltIn,
		SystemPrompt:   "Test prompt",
		Tools:          []string{"read", "write"},
		PermissionMode: PermissionModeAsk,
		MaxTurns:       10,
		Memory:         AgentMemoryUser,
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded AgentDefinition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.AgentType != agent.AgentType {
		t.Errorf("AgentType after round-trip = %q, want %q", decoded.AgentType, agent.AgentType)
	}
	if decoded.Source != agent.Source {
		t.Errorf("Source after round-trip = %q, want %q", decoded.Source, agent.Source)
	}
	if decoded.MaxTurns != agent.MaxTurns {
		t.Errorf("MaxTurns after round-trip = %d, want %d", decoded.MaxTurns, agent.MaxTurns)
	}
}

// ──────────────────────────── McpServerSpec / HookConfig ────────────────────────────

func TestMcpServerSpec(t *testing.T) {
	spec := McpServerSpec{
		Name:    "my-server",
		Command: "npx",
		Args:    []string{"-y", "my-mcp-server"},
		Env:     map[string]string{"API_KEY": "secret"},
	}
	if spec.Name != "my-server" {
		t.Errorf("Name = %q, want %q", spec.Name, "my-server")
	}
	if len(spec.Args) != 2 {
		t.Errorf("len(Args) = %d, want 2", len(spec.Args))
	}
	if spec.Env["API_KEY"] != "secret" {
		t.Errorf("Env[API_KEY] = %q, want %q", spec.Env["API_KEY"], "secret")
	}
}

func TestHookConfig(t *testing.T) {
	hc := HookConfig{
		Type:    "shell",
		Command: "echo hello",
		Tools:   []string{"write", "edit"},
	}
	if hc.Type != "shell" {
		t.Errorf("Type = %q, want %q", hc.Type, "shell")
	}
	if len(hc.Tools) != 2 {
		t.Errorf("len(Tools) = %d, want 2", len(hc.Tools))
	}
}

func TestHooksSettings(t *testing.T) {
	hs := HooksSettings{
		"pre":  {{Type: "shell", Command: "pre-cmd"}},
		"post": {{Type: "shell", Command: "post-cmd"}},
	}
	if len(hs) != 2 {
		t.Errorf("len(HooksSettings) = %d, want 2", len(hs))
	}
}

// ──────────────────────────── FormatAgentInfo ────────────────────────────

func TestFormatAgentInfo_NonEmpty(t *testing.T) {
	am := newTestAgentManager(t)
	agent, _ := am.GetAgent("general-purpose")
	output := am.FormatAgentInfo(agent)
	if output == "" {
		t.Error("FormatAgentInfo() returned empty string")
	}
}

func TestFormatAgentInfo_ContainsAgentDetails(t *testing.T) {
	am := newTestAgentManager(t)
	agent := &AgentDefinition{
		AgentType:      "test-agent",
		WhenToUse:      "Test description",
		Source:         AgentSourceBuiltIn,
		Color:          "blue",
		Model:          "claude-sonnet-4-5",
		PermissionMode: PermissionModeAsk,
		Tools:          []string{"read", "write"},
		DisallowedTools: []string{"bash"},
		Skills:         []string{"testing"},
		MaxTurns:       5,
		Memory:         AgentMemoryUser,
		SystemPrompt:   "You are a test agent.",
	}
	output := am.FormatAgentInfo(agent)

	checks := []string{
		"test-agent",
		"Test description",
		"built-in",
		"claude-sonnet-4-5",
		"ask",
		"blue",
		"read, write",
		"bash",
		"testing",
		"5",
		"user",
		"You are a test agent.",
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Errorf("FormatAgentInfo() output missing %q", want)
		}
	}
}

func TestFormatAgentInfo_OmitsEmptyFields(t *testing.T) {
	am := newTestAgentManager(t)
	agent := &AgentDefinition{
		AgentType:    "minimal",
		WhenToUse:    "Minimal agent",
		Source:       AgentSourceBuiltIn,
		SystemPrompt: "Minimal prompt",
	}
	output := am.FormatAgentInfo(agent)

	// Should not contain model/permission/tools sections for empty fields
	if strings.Contains(output, "模型:") {
		t.Error("FormatAgentInfo should not show model when empty")
	}
}

// ──────────────────────────── FormatAgentList ────────────────────────────

func TestFormatAgentList_NonEmpty(t *testing.T) {
	am := newTestAgentManager(t)
	output := am.FormatAgentList()
	if output == "" {
		t.Error("FormatAgentList() returned empty string")
	}
}

func TestFormatAgentList_ContainsBuiltInAgents(t *testing.T) {
	am := newTestAgentManager(t)
	output := am.FormatAgentList()

	if !strings.Contains(output, "内置 Agents") {
		t.Error("FormatAgentList() missing built-in section header")
	}
	if !strings.Contains(output, "general-purpose") {
		t.Error("FormatAgentList() missing general-purpose agent")
	}
	if !strings.Contains(output, "✓") {
		t.Error("FormatAgentList() missing current agent marker")
	}
}

func TestFormatAgentList_ShowsUsageInstructions(t *testing.T) {
	am := newTestAgentManager(t)
	output := am.FormatAgentList()

	if !strings.Contains(output, "使用方法") {
		t.Error("FormatAgentList() missing usage instructions")
	}
	if !strings.Contains(output, "/agent switch") {
		t.Error("FormatAgentList() missing /agent switch instruction")
	}
}

// ──────────────────────────── CreateCustomAgent ────────────────────────────

func TestCreateCustomAgent(t *testing.T) {
	am := newTestAgentManager(t)

	agent := &AgentDefinition{
		AgentType:    "my-custom",
		WhenToUse:    "My custom agent",
		SystemPrompt: "You are custom.",
		Color:        "pink",
	}

	err := am.CreateCustomAgent(agent)
	if err != nil {
		t.Fatalf("CreateCustomAgent() returned error: %v", err)
	}

	retrieved, err := am.GetAgent("my-custom")
	if err != nil {
		t.Fatalf("GetAgent after CreateCustomAgent returned error: %v", err)
	}
	if retrieved.AgentType != "my-custom" {
		t.Errorf("retrieved AgentType = %q, want %q", retrieved.AgentType, "my-custom")
	}
	if retrieved.Source != AgentSourceUserSettings {
		t.Errorf("retrieved Source = %q, want %q", retrieved.Source, AgentSourceUserSettings)
	}
}

func TestCreateCustomAgent_SetsSourceToUserSettings(t *testing.T) {
	am := newTestAgentManager(t)

	agent := &AgentDefinition{
		AgentType:    "source-test",
		WhenToUse:    "Source test",
		SystemPrompt: "Test",
		Source:       AgentSourcePlugin, // should be overwritten
	}

	err := am.CreateCustomAgent(agent)
	if err != nil {
		t.Fatalf("CreateCustomAgent() returned error: %v", err)
	}

	retrieved, _ := am.GetAgent("source-test")
	if retrieved.Source != AgentSourceUserSettings {
		t.Errorf("Source = %q, want %q (should be overwritten to userSettings)", retrieved.Source, AgentSourceUserSettings)
	}
}

func TestCreateCustomAgent_DuplicateType(t *testing.T) {
	am := newTestAgentManager(t)

	agent := &AgentDefinition{
		AgentType:    "general-purpose", // already exists as built-in
		WhenToUse:    "Duplicate",
		SystemPrompt: "Dupe",
	}

	err := am.CreateCustomAgent(agent)
	if err == nil {
		t.Error("CreateCustomAgent with duplicate type should return error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "already exists")
	}
}

func TestCreateCustomAgent_EmptyType(t *testing.T) {
	am := newTestAgentManager(t)

	agent := &AgentDefinition{
		AgentType:    "",
		WhenToUse:    "No type",
		SystemPrompt: "Empty type",
	}

	err := am.CreateCustomAgent(agent)
	if err == nil {
		t.Error("CreateCustomAgent with empty type should return error")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "cannot be empty")
	}
}

func TestCreateCustomAgent_SavesToFile(t *testing.T) {
	am := newTestAgentManager(t)

	agent := &AgentDefinition{
		AgentType:    "file-test",
		WhenToUse:    "File save test",
		SystemPrompt: "Saved to disk.",
		Color:        "red",
	}

	err := am.CreateCustomAgent(agent)
	if err != nil {
		t.Fatalf("CreateCustomAgent() returned error: %v", err)
	}

	// Verify file was created
	filePath := filepath.Join(am.configPath, "file-test.md")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("agent file %q was not created", filePath)
	}

	// Verify file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read agent file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: file-test") {
		t.Errorf("agent file missing 'name: file-test', content:\n%s", content)
	}
	if !strings.Contains(content, "description: File save test") {
		t.Errorf("agent file missing description, content:\n%s", content)
	}
	if !strings.Contains(content, "Saved to disk.") {
		t.Errorf("agent file missing system prompt, content:\n%s", content)
	}
}

func TestCreateCustomAgent_NoConfigPath(t *testing.T) {
	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: "",
	}
	am.loadBuiltInAgents()

	agent := &AgentDefinition{
		AgentType:    "no-config",
		WhenToUse:    "No config path",
		SystemPrompt: "Test",
	}

	err := am.CreateCustomAgent(agent)
	if err == nil {
		t.Error("CreateCustomAgent with empty configPath should return error")
	}
	if !strings.Contains(err.Error(), "config path not set") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "config path not set")
	}
}

// ──────────────────────────── UpdateCustomAgent ────────────────────────────

func TestUpdateCustomAgent(t *testing.T) {
	am := newTestAgentManager(t)

	// First create a custom agent
	agent := &AgentDefinition{
		AgentType:    "updatable",
		WhenToUse:    "Original description",
		SystemPrompt: "Original prompt",
		Color:        "blue",
	}
	am.CreateCustomAgent(agent)

	// Now update it
	updates := &AgentDefinition{
		WhenToUse:      "Updated description",
		SystemPrompt:   "Updated prompt",
		Color:          "red",
		Tools:          []string{"read"},
		PermissionMode: PermissionModeReadOnly,
	}

	err := am.UpdateCustomAgent("updatable", updates)
	if err != nil {
		t.Fatalf("UpdateCustomAgent() returned error: %v", err)
	}

	updated, _ := am.GetAgent("updatable")
	if updated.WhenToUse != "Updated description" {
		t.Errorf("WhenToUse = %q, want %q", updated.WhenToUse, "Updated description")
	}
	if updated.SystemPrompt != "Updated prompt" {
		t.Errorf("SystemPrompt = %q, want %q", updated.SystemPrompt, "Updated prompt")
	}
	if updated.Color != "red" {
		t.Errorf("Color = %q, want %q", updated.Color, "red")
	}
	if updated.PermissionMode != PermissionModeReadOnly {
		t.Errorf("PermissionMode = %q, want %q", updated.PermissionMode, PermissionModeReadOnly)
	}
}

func TestUpdateCustomAgent_BuiltInAgent(t *testing.T) {
	am := newTestAgentManager(t)

	updates := &AgentDefinition{WhenToUse: "Hacked"}
	err := am.UpdateCustomAgent("general-purpose", updates)
	if err == nil {
		t.Error("UpdateCustomAgent on built-in agent should return error")
	}
	if !strings.Contains(err.Error(), "cannot modify built-in") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "cannot modify built-in")
	}
}

func TestUpdateCustomAgent_NonExistent(t *testing.T) {
	am := newTestAgentManager(t)

	updates := &AgentDefinition{WhenToUse: "Doesn't matter"}
	err := am.UpdateCustomAgent("nonexistent", updates)
	if err == nil {
		t.Error("UpdateCustomAgent on non-existent agent should return error")
	}
}

func TestUpdateCustomAgent_PartialUpdate(t *testing.T) {
	am := newTestAgentManager(t)

	agent := &AgentDefinition{
		AgentType:    "partial-update",
		WhenToUse:    "Original",
		SystemPrompt: "Original prompt",
		Color:        "blue",
	}
	am.CreateCustomAgent(agent)

	// Only update WhenToUse, leave others unchanged
	updates := &AgentDefinition{WhenToUse: "New description"}
	am.UpdateCustomAgent("partial-update", updates)

	updated, _ := am.GetAgent("partial-update")
	if updated.WhenToUse != "New description" {
		t.Errorf("WhenToUse = %q, want %q", updated.WhenToUse, "New description")
	}
	if updated.SystemPrompt != "Original prompt" {
		t.Errorf("SystemPrompt should remain unchanged = %q, want %q", updated.SystemPrompt, "Original prompt")
	}
	if updated.Color != "blue" {
		t.Errorf("Color should remain unchanged = %q, want %q", updated.Color, "blue")
	}
}

// ──────────────────────────── DeleteCustomAgent ────────────────────────────

func TestDeleteCustomAgent(t *testing.T) {
	am := newTestAgentManager(t)

	agent := &AgentDefinition{
		AgentType:    "deletable",
		WhenToUse:    "Delete me",
		SystemPrompt: "Bye",
	}
	am.CreateCustomAgent(agent)

	err := am.DeleteCustomAgent("deletable")
	if err != nil {
		t.Fatalf("DeleteCustomAgent() returned error: %v", err)
	}

	_, err = am.GetAgent("deletable")
	if err == nil {
		t.Error("deleted agent should not be found")
	}
}

func TestDeleteCustomAgent_BuiltInAgent(t *testing.T) {
	am := newTestAgentManager(t)

	err := am.DeleteCustomAgent("general-purpose")
	if err == nil {
		t.Error("DeleteCustomAgent on built-in agent should return error")
	}
	if !strings.Contains(err.Error(), "cannot delete built-in") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "cannot delete built-in")
	}
}

func TestDeleteCustomAgent_NonExistent(t *testing.T) {
	am := newTestAgentManager(t)

	err := am.DeleteCustomAgent("nonexistent")
	if err == nil {
		t.Error("DeleteCustomAgent on non-existent agent should return error")
	}
}

func TestDeleteCustomAgent_CurrentAgentFallsBackToGeneral(t *testing.T) {
	am := newTestAgentManager(t)

	// Create and switch to a custom agent
	agent := &AgentDefinition{
		AgentType:    "temp-current",
		WhenToUse:    "Temporary",
		SystemPrompt: "Temp",
	}
	am.CreateCustomAgent(agent)
	am.SetCurrentAgent("temp-current")

	// Delete the current agent
	am.DeleteCustomAgent("temp-current")

	// Current agent should fall back to general-purpose
	current := am.GetCurrentAgent()
	if current == nil || current.AgentType != "general-purpose" {
		t.Errorf("after deleting current agent, currentAgent = %v, want general-purpose", current)
	}
}

func TestDeleteCustomAgent_RemovesFile(t *testing.T) {
	am := newTestAgentManager(t)

	agent := &AgentDefinition{
		AgentType:    "file-delete",
		WhenToUse:    "Delete file",
		SystemPrompt: "Bye",
		Filename:     "file-delete",
	}
	am.CreateCustomAgent(agent)

	filePath := filepath.Join(am.configPath, "file-delete.md")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("agent file not created before delete")
	}

	am.DeleteCustomAgent("file-delete")

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("agent file should be deleted")
	}
}

// ──────────────────────────── ExportAgent ────────────────────────────

func TestExportAgent_JSON(t *testing.T) {
	am := newTestAgentManager(t)

	output, err := am.ExportAgent("explore", "json")
	if err != nil {
		t.Fatalf("ExportAgent(json) returned error: %v", err)
	}

	var decoded AgentDefinition
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("exported JSON is invalid: %v", err)
	}
	if decoded.AgentType != "explore" {
		t.Errorf("exported AgentType = %q, want %q", decoded.AgentType, "explore")
	}
	if decoded.Source != AgentSourceBuiltIn {
		t.Errorf("exported Source = %q, want %q", decoded.Source, AgentSourceBuiltIn)
	}
}

func TestExportAgent_Markdown(t *testing.T) {
	am := newTestAgentManager(t)

	output, err := am.ExportAgent("explore", "markdown")
	if err != nil {
		t.Fatalf("ExportAgent(markdown) returned error: %v", err)
	}

	if !strings.HasPrefix(output, "---") {
		t.Error("markdown export should start with frontmatter delimiter")
	}
	if !strings.Contains(output, "name: explore") {
		t.Error("markdown export should contain agent name")
	}
}

func TestExportAgent_MdAlias(t *testing.T) {
	am := newTestAgentManager(t)

	output, err := am.ExportAgent("explore", "md")
	if err != nil {
		t.Fatalf("ExportAgent(md) returned error: %v", err)
	}
	if !strings.HasPrefix(output, "---") {
		t.Error("md export should start with frontmatter delimiter")
	}
}

func TestExportAgent_InvalidType(t *testing.T) {
	am := newTestAgentManager(t)

	_, err := am.ExportAgent("nonexistent", "json")
	if err == nil {
		t.Error("ExportAgent with non-existent agent should return error")
	}
}

func TestExportAgent_UnsupportedFormat(t *testing.T) {
	am := newTestAgentManager(t)

	_, err := am.ExportAgent("explore", "xml")
	if err == nil {
		t.Error("ExportAgent with unsupported format should return error")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "unsupported format")
	}
}

// ──────────────────────────── ImportAgent ────────────────────────────

func TestImportAgent_JSON(t *testing.T) {
	am := newTestAgentManager(t)

	agentJSON := `{
		"agentType": "imported-json",
		"whenToUse": "Imported from JSON",
		"systemPrompt": "You were imported.",
		"color": "cyan"
	}`

	err := am.ImportAgent(agentJSON, "json")
	if err != nil {
		t.Fatalf("ImportAgent(json) returned error: %v", err)
	}

	retrieved, err := am.GetAgent("imported-json")
	if err != nil {
		t.Fatalf("GetAgent after import returned error: %v", err)
	}
	if retrieved.AgentType != "imported-json" {
		t.Errorf("imported AgentType = %q, want %q", retrieved.AgentType, "imported-json")
	}
	if retrieved.Color != "cyan" {
		t.Errorf("imported Color = %q, want %q", retrieved.Color, "cyan")
	}
}

func TestImportAgent_Markdown(t *testing.T) {
	am := newTestAgentManager(t)

	mdContent := `---
name: imported-md
description: Imported from Markdown
color: orange
model: claude-opus-4-6
permissionMode: read-only
---

You were imported from markdown.`

	err := am.ImportAgent(mdContent, "markdown")
	if err != nil {
		t.Fatalf("ImportAgent(markdown) returned error: %v", err)
	}

	retrieved, err := am.GetAgent("imported-md")
	if err != nil {
		t.Fatalf("GetAgent after import returned error: %v", err)
	}
	if retrieved.AgentType != "imported-md" {
		t.Errorf("imported AgentType = %q, want %q", retrieved.AgentType, "imported-md")
	}
	if retrieved.Color != "orange" {
		t.Errorf("imported Color = %q, want %q", retrieved.Color, "orange")
	}
	if retrieved.Model != "claude-opus-4-6" {
		t.Errorf("imported Model = %q, want %q", retrieved.Model, "claude-opus-4-6")
	}
	if retrieved.PermissionMode != PermissionModeReadOnly {
		t.Errorf("imported PermissionMode = %q, want %q", retrieved.PermissionMode, PermissionModeReadOnly)
	}
}

func TestImportAgent_MdAlias(t *testing.T) {
	am := newTestAgentManager(t)

	mdContent := `---
name: md-alias
description: MD alias test
---

Test prompt.`

	err := am.ImportAgent(mdContent, "md")
	if err != nil {
		t.Fatalf("ImportAgent(md) returned error: %v", err)
	}

	retrieved, _ := am.GetAgent("md-alias")
	if retrieved == nil {
		t.Fatal("imported agent not found")
	}
}

func TestImportAgent_UnsupportedFormat(t *testing.T) {
	am := newTestAgentManager(t)

	err := am.ImportAgent("{}", "xml")
	if err == nil {
		t.Error("ImportAgent with unsupported format should return error")
	}
}

func TestImportAgent_MissingName(t *testing.T) {
	am := newTestAgentManager(t)

	mdContent := `---
description: No name here
---

Some prompt.`

	err := am.ImportAgent(mdContent, "markdown")
	if err == nil {
		t.Error("ImportAgent with missing name should return error")
	}
}

func TestImportAgent_InvalidJSON(t *testing.T) {
	am := newTestAgentManager(t)

	err := am.ImportAgent("{invalid json", "json")
	if err == nil {
		t.Error("ImportAgent with invalid JSON should return error")
	}
}

// ──────────────────────────── parseFrontmatter ────────────────────────────

func TestParseFrontmatter_Valid(t *testing.T) {
	content := `---
name: test-agent
description: Test description
color: blue
---

System prompt here.`

	fm, body := parseFrontmatter(content)

	if fm["name"] != "test-agent" {
		t.Errorf("name = %v, want %q", fm["name"], "test-agent")
	}
	if fm["description"] != "Test description" {
		t.Errorf("description = %v, want %q", fm["description"], "Test description")
	}
	if fm["color"] != "blue" {
		t.Errorf("color = %v, want %q", fm["color"], "blue")
	}
	if !strings.Contains(body, "System prompt here.") {
		t.Errorf("body = %q, want to contain system prompt", body)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "Just regular content without frontmatter."

	fm, body := parseFrontmatter(content)

	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter, got %v", fm)
	}
	if body != content {
		t.Errorf("body should equal original content when no frontmatter")
	}
}

func TestParseFrontmatter_ArrayValue(t *testing.T) {
	content := `---
name: array-test
tools: [read, write, edit]
---

Prompt.`

	fm, _ := parseFrontmatter(content)

	tools, ok := fm["tools"].([]any)
	if !ok {
		t.Fatalf("tools not parsed as array, got %T", fm["tools"])
	}
	if len(tools) != 3 {
		t.Errorf("len(tools) = %d, want 3", len(tools))
	}
}

func TestParseFrontmatter_BooleanValues(t *testing.T) {
	content := `---
name: bool-test
background: true
omitClaudeMd: false
---

Prompt.`

	fm, _ := parseFrontmatter(content)

	if fm["background"] != true {
		t.Errorf("background = %v, want true", fm["background"])
	}
	if fm["omitClaudeMd"] != false {
		t.Errorf("omitClaudeMd = %v, want false", fm["omitClaudeMd"])
	}
}

func TestParseFrontmatter_UnterminatedFrontmatter(t *testing.T) {
	content := `---
name: no-end
description: Missing closing delimiter

Body text.`

	fm, body := parseFrontmatter(content)

	if len(fm) != 0 {
		t.Errorf("expected empty frontmatter for unterminated, got %v", fm)
	}
	if body != content {
		t.Errorf("body should equal original content for unterminated frontmatter")
	}
}

// ──────────────────────────── loadCustomAgents / loadProjectAgents ────────────────────────────

func TestLoadCustomAgents_FromDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, "agents")
	os.MkdirAll(agentsDir, 0755)

	// Write a custom agent markdown file
	agentMD := `---
name: custom-from-file
description: Loaded from custom dir
color: gold
model: custom-model
permissionMode: workspace-write
tools: [read, grep]
---

Custom agent system prompt.`

	err := os.WriteFile(filepath.Join(agentsDir, "custom-from-file.md"), []byte(agentMD), 0644)
	if err != nil {
		t.Fatalf("failed to write test agent file: %v", err)
	}

	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: agentsDir,
		projectDir: "",
	}
	am.loadBuiltInAgents()
	am.loadCustomAgents()

	agent, err := am.GetAgent("custom-from-file")
	if err != nil {
		t.Fatalf("custom agent not loaded: %v", err)
	}
	if agent.Source != AgentSourceUserSettings {
		t.Errorf("Source = %q, want %q", agent.Source, AgentSourceUserSettings)
	}
	if agent.Color != "gold" {
		t.Errorf("Color = %q, want %q", agent.Color, "gold")
	}
	if agent.Model != "custom-model" {
		t.Errorf("Model = %q, want %q", agent.Model, "custom-model")
	}
	if agent.PermissionMode != PermissionModeWorkspaceWrite {
		t.Errorf("PermissionMode = %q, want %q", agent.PermissionMode, PermissionModeWorkspaceWrite)
	}
	if !strings.Contains(agent.SystemPrompt, "Custom agent system prompt") {
		t.Errorf("SystemPrompt = %q, want to contain custom prompt", agent.SystemPrompt)
	}
}

func TestLoadCustomAgents_EmptyConfigPath(t *testing.T) {
	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: "",
	}
	// Should not panic
	am.loadCustomAgents()
}

func TestLoadProjectAgents_FromDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	agentsDir := filepath.Join(projectDir, ".smartclaw", "agents")
	os.MkdirAll(agentsDir, 0755)

	agentMD := `---
name: project-agent
description: Loaded from project dir
---

Project agent prompt.`

	err := os.WriteFile(filepath.Join(agentsDir, "project-agent.md"), []byte(agentMD), 0644)
	if err != nil {
		t.Fatalf("failed to write project agent file: %v", err)
	}

	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: filepath.Join(tmpDir, "nonexistent"), // no user agents
		projectDir: projectDir,
	}
	am.loadBuiltInAgents()
	am.loadProjectAgents()

	agent, err := am.GetAgent("project-agent")
	if err != nil {
		t.Fatalf("project agent not loaded: %v", err)
	}
	if agent.Source != AgentSourceProjectSettings {
		t.Errorf("Source = %q, want %q", agent.Source, AgentSourceProjectSettings)
	}
}

func TestLoadProjectAgents_EmptyProjectDir(t *testing.T) {
	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		projectDir: "",
	}
	// Should not panic
	am.loadProjectAgents()
}

func TestLoadProjectAgents_InvalidMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	agentsDir := filepath.Join(projectDir, ".smartclaw", "agents")
	os.MkdirAll(agentsDir, 0755)

	// Write a markdown file without proper frontmatter (missing name)
	badMD := `---
description: No name field
---

Bad agent.`

	err := os.WriteFile(filepath.Join(agentsDir, "bad-agent.md"), []byte(badMD), 0644)
	if err != nil {
		t.Fatalf("failed to write bad agent file: %v", err)
	}

	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: filepath.Join(tmpDir, "nonexistent"),
		projectDir: projectDir,
	}
	am.loadBuiltInAgents()
	am.loadProjectAgents()

	// Bad agent should not be loaded
	_, err = am.GetAgent("bad-agent")
	if err == nil {
		t.Error("invalid agent should not be loaded")
	}
}

// ──────────────────────────── Built-in Agent Specifics ────────────────────────────

func TestBuiltInAgent_Explore(t *testing.T) {
	am := newTestAgentManager(t)
	agent, err := am.GetAgent("explore")
	if err != nil {
		t.Fatalf("explore agent not found: %v", err)
	}
	if agent.Color != "green" {
		t.Errorf("explore Color = %q, want %q", agent.Color, "green")
	}
	if len(agent.Tools) == 0 {
		t.Error("explore agent should have tools")
	}
	if !agent.OmitClaudeMd {
		t.Error("explore agent should have OmitClaudeMd = true")
	}
}

func TestBuiltInAgent_Plan(t *testing.T) {
	am := newTestAgentManager(t)
	agent, err := am.GetAgent("plan")
	if err != nil {
		t.Fatalf("plan agent not found: %v", err)
	}
	if agent.Color != "yellow" {
		t.Errorf("plan Color = %q, want %q", agent.Color, "yellow")
	}
	if !agent.OmitClaudeMd {
		t.Error("plan agent should have OmitClaudeMd = true")
	}
}

func TestBuiltInAgent_GeneralPurpose(t *testing.T) {
	am := newTestAgentManager(t)
	agent, err := am.GetAgent("general-purpose")
	if err != nil {
		t.Fatalf("general-purpose agent not found: %v", err)
	}
	if agent.Color != "blue" {
		t.Errorf("general-purpose Color = %q, want %q", agent.Color, "blue")
	}
}

// ──────────────────────────── Concurrency ────────────────────────────

func TestAgentManager_ConcurrentAccess(t *testing.T) {
	am := newTestAgentManager(t)

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = am.GetCurrentAgent()
			_ = am.ListAgents()
			_, _ = am.GetAgent("explore")
			_ = am.GetSystemPrompt()
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			agent := &AgentDefinition{
				AgentType:    fmt.Sprintf("concurrent-%d", i),
				WhenToUse:    "Concurrent test",
				SystemPrompt: "Concurrent prompt",
			}
			if err := am.CreateCustomAgent(agent); err != nil {
				select {
				case errors <- err:
				default:
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestAgentManager_ConcurrentSwitch(t *testing.T) {
	am := newTestAgentManager(t)

	agents := []string{"explore", "plan", "code-review", "devops", "security"}
	var wg sync.WaitGroup

	for _, agentType := range agents {
		wg.Add(1)
		go func(at string) {
			defer wg.Done()
			_ = am.SetCurrentAgent(at)
		}(agentType)
	}

	wg.Wait()

	// Just verify we can still get the current agent without panic
	current := am.GetCurrentAgent()
	if current == nil {
		t.Error("GetCurrentAgent() returned nil after concurrent switches")
	}
}

// ──────────────────────────── parseAgentFromMarkdown ────────────────────────────

func TestParseAgentFromMarkdown_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	content := `---
name: parsed-agent
description: Parsed from file
color: teal
model: claude-opus-4-6
permissionMode: ask
tools: [read, write]
disallowedTools: [bash]
skills: [coding]
memory: project
background: true
maxTurns: 10
effort: high
initialPrompt: Hello!
---

Parsed agent system prompt.`

	filePath := filepath.Join(tmpDir, "parsed-agent.md")
	os.WriteFile(filePath, []byte(content), 0644)

	am := &AgentManager{agents: make(map[string]*AgentDefinition)}
	agent := am.parseAgentFromMarkdown(filePath, AgentSourceUserSettings)

	if agent == nil {
		t.Fatal("parseAgentFromMarkdown returned nil")
	}
	if agent.AgentType != "parsed-agent" {
		t.Errorf("AgentType = %q, want %q", agent.AgentType, "parsed-agent")
	}
	if agent.WhenToUse != "Parsed from file" {
		t.Errorf("WhenToUse = %q, want %q", agent.WhenToUse, "Parsed from file")
	}
	if agent.Color != "teal" {
		t.Errorf("Color = %q, want %q", agent.Color, "teal")
	}
	if agent.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", agent.Model, "claude-opus-4-6")
	}
	if agent.PermissionMode != PermissionModeAsk {
		t.Errorf("PermissionMode = %q, want %q", agent.PermissionMode, PermissionModeAsk)
	}
	if len(agent.Tools) != 2 {
		t.Errorf("len(Tools) = %d, want 2", len(agent.Tools))
	}
	if len(agent.DisallowedTools) != 1 || agent.DisallowedTools[0] != "bash" {
		t.Errorf("DisallowedTools = %v, want [bash]", agent.DisallowedTools)
	}
	if len(agent.Skills) != 1 || agent.Skills[0] != "coding" {
		t.Errorf("Skills = %v, want [coding]", agent.Skills)
	}
	if agent.Memory != AgentMemoryProject {
		t.Errorf("Memory = %q, want %q", agent.Memory, AgentMemoryProject)
	}
	if !agent.Background {
		t.Error("Background = false, want true")
	}
	if agent.Source != AgentSourceUserSettings {
		t.Errorf("Source = %q, want %q", agent.Source, AgentSourceUserSettings)
	}
	if !strings.Contains(agent.SystemPrompt, "Parsed agent system prompt") {
		t.Errorf("SystemPrompt = %q, want to contain parsed prompt", agent.SystemPrompt)
	}
	if agent.Filename != "parsed-agent" {
		t.Errorf("Filename = %q, want %q", agent.Filename, "parsed-agent")
	}
}

func TestParseAgentFromMarkdown_MissingName(t *testing.T) {
	tmpDir := t.TempDir()

	content := `---
description: No name
---

Prompt.`

	filePath := filepath.Join(tmpDir, "no-name.md")
	os.WriteFile(filePath, []byte(content), 0644)

	am := &AgentManager{agents: make(map[string]*AgentDefinition)}
	agent := am.parseAgentFromMarkdown(filePath, AgentSourceUserSettings)

	if agent != nil {
		t.Error("expected nil for agent without name")
	}
}

func TestParseAgentFromMarkdown_MissingDescription(t *testing.T) {
	tmpDir := t.TempDir()

	content := `---
name: no-desc
---

Prompt.`

	filePath := filepath.Join(tmpDir, "no-desc.md")
	os.WriteFile(filePath, []byte(content), 0644)

	am := &AgentManager{agents: make(map[string]*AgentDefinition)}
	agent := am.parseAgentFromMarkdown(filePath, AgentSourceUserSettings)

	if agent != nil {
		t.Error("expected nil for agent without description")
	}
}

func TestParseAgentFromMarkdown_NonexistentFile(t *testing.T) {
	am := &AgentManager{agents: make(map[string]*AgentDefinition)}
	agent := am.parseAgentFromMarkdown("/nonexistent/path/agent.md", AgentSourceUserSettings)
	if agent != nil {
		t.Error("expected nil for nonexistent file")
	}
}

func TestParseAgentFromMarkdown_ProjectSource(t *testing.T) {
	tmpDir := t.TempDir()

	content := `---
name: project-sourced
description: From project
---

Project prompt.`

	filePath := filepath.Join(tmpDir, "project-sourced.md")
	os.WriteFile(filePath, []byte(content), 0644)

	am := &AgentManager{agents: make(map[string]*AgentDefinition)}
	agent := am.parseAgentFromMarkdown(filePath, AgentSourceProjectSettings)

	if agent == nil {
		t.Fatal("parseAgentFromMarkdown returned nil")
	}
	if agent.Source != AgentSourceProjectSettings {
		t.Errorf("Source = %q, want %q", agent.Source, AgentSourceProjectSettings)
	}
	if agent.BaseDir != tmpDir {
		t.Errorf("BaseDir = %q, want %q", agent.BaseDir, tmpDir)
	}
}

// ──────────────────────────── saveCustomAgent ────────────────────────────

func TestSaveCustomAgent_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "new-agents-dir")

	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: configPath,
	}
	am.loadBuiltInAgents()

	agent := &AgentDefinition{
		AgentType:    "mkdir-test",
		WhenToUse:    "Test directory creation",
		SystemPrompt: "Prompt",
	}
	am.agents[agent.AgentType] = agent

	err := am.saveCustomAgent(agent)
	if err != nil {
		t.Fatalf("saveCustomAgent() returned error: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("saveCustomAgent should create config directory")
	}
}

func TestSaveCustomAgent_WithTools(t *testing.T) {
	tmpDir := t.TempDir()

	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: filepath.Join(tmpDir, "agents"),
	}

	agent := &AgentDefinition{
		AgentType:       "tools-agent",
		WhenToUse:       "Agent with tools",
		SystemPrompt:    "Prompt",
		Tools:           []string{"read", "write"},
		DisallowedTools: []string{"bash"},
		Skills:          []string{"coding"},
	}
	am.agents[agent.AgentType] = agent

	err := am.saveCustomAgent(agent)
	if err != nil {
		t.Fatalf("saveCustomAgent() returned error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(am.configPath, "tools-agent.md"))
	content := string(data)
	if !strings.Contains(content, "tools: [read, write]") {
		t.Errorf("saved file missing tools, content:\n%s", content)
	}
	if !strings.Contains(content, "disallowedTools: [bash]") {
		t.Errorf("saved file missing disallowedTools, content:\n%s", content)
	}
	if !strings.Contains(content, "skills: [coding]") {
		t.Errorf("saved file missing skills, content:\n%s", content)
	}
}

func TestSaveCustomAgent_UsesFilename(t *testing.T) {
	tmpDir := t.TempDir()

	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: filepath.Join(tmpDir, "agents"),
	}

	agent := &AgentDefinition{
		AgentType:    "type-name",
		WhenToUse:    "Agent with custom filename",
		SystemPrompt: "Prompt",
		Filename:     "custom-filename",
	}
	am.agents[agent.AgentType] = agent

	err := am.saveCustomAgent(agent)
	if err != nil {
		t.Fatalf("saveCustomAgent() returned error: %v", err)
	}

	filePath := filepath.Join(am.configPath, "custom-filename.md")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("expected file at %q, not found", filePath)
	}
}

// ──────────────────────────── NewAgentManager (real) ────────────────────────────

func TestNewAgentManager_Real(t *testing.T) {
	// Test the actual NewAgentManager constructor
	// It may not find custom agents, but built-in agents should always load
	am := NewAgentManager("")

	if am.agents == nil {
		t.Error("agents map is nil")
	}
	if len(am.agents) < 10 {
		t.Errorf("expected at least 10 built-in agents, got %d", len(am.agents))
	}
	current := am.GetCurrentAgent()
	if current == nil {
		t.Error("currentAgent should not be nil")
	}
}

func TestNewAgentManager_FirstAgentAsDefault(t *testing.T) {
	// When general-purpose doesn't exist (shouldn't happen normally),
	// the first agent in the map should be used as default.
	am := &AgentManager{
		agents:     make(map[string]*AgentDefinition),
		configPath: "",
		projectDir: "",
	}

	// Only add one non-general agent
	am.agents["only-agent"] = &AgentDefinition{
		AgentType: "only-agent",
		Source:    AgentSourceBuiltIn,
	}

	// Replicate the default selection logic from NewAgentManager
	if general, exists := am.agents["general-purpose"]; exists {
		am.currentAgent = general
	} else if len(am.agents) > 0 {
		for _, agent := range am.agents {
			am.currentAgent = agent
			break
		}
	}

	current := am.GetCurrentAgent()
	if current == nil {
		t.Fatal("currentAgent should not be nil when agents exist")
	}
	if current.AgentType != "only-agent" {
		t.Errorf("currentAgent = %q, want %q", current.AgentType, "only-agent")
	}
}

// ──────────────────────────── FormatAgentList with multiple sources ────────────────────────────

func TestFormatAgentList_MultipleSources(t *testing.T) {
	am := newTestAgentManager(t)

	am.agents["custom-display"] = &AgentDefinition{
		AgentType:    "custom-display",
		WhenToUse:    "Custom for display",
		SystemPrompt: "Custom",
		Source:       AgentSourceUserSettings,
	}

	output := am.FormatAgentList()

	if !strings.Contains(output, "内置 Agents") {
		t.Error("missing built-in section")
	}
	if !strings.Contains(output, "用户自定义") {
		t.Error("missing user custom section")
	}
}

// ──────────────────────────── Import/Export round-trip ────────────────────────────

func TestExportImportRoundTrip(t *testing.T) {
	am := newTestAgentManager(t)

	// Create a custom agent with various fields
	agent := &AgentDefinition{
		AgentType:      "roundtrip",
		WhenToUse:      "Round trip test",
		SystemPrompt:   "Round trip prompt.",
		Color:          "purple",
		Model:          "claude-sonnet-4-5",
		PermissionMode: PermissionModeWorkspaceWrite,
	}
	am.CreateCustomAgent(agent)

	// Export as JSON
	exported, err := am.ExportAgent("roundtrip", "json")
	if err != nil {
		t.Fatalf("ExportAgent failed: %v", err)
	}

	// Delete the agent
	am.DeleteCustomAgent("roundtrip")

	// Verify it's gone
	_, err = am.GetAgent("roundtrip")
	if err == nil {
		t.Error("agent should be deleted")
	}

	// Re-import
	err = am.ImportAgent(exported, "json")
	if err != nil {
		t.Fatalf("ImportAgent failed: %v", err)
	}

	// Verify restored
	restored, err := am.GetAgent("roundtrip")
	if err != nil {
		t.Fatalf("GetAgent after import failed: %v", err)
	}
	if restored.AgentType != "roundtrip" {
		t.Errorf("restored AgentType = %q, want %q", restored.AgentType, "roundtrip")
	}
	if restored.Color != "purple" {
		t.Errorf("restored Color = %q, want %q", restored.Color, "purple")
	}
}
