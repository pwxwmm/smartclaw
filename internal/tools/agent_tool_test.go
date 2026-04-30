package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewAgentTool(t *testing.T) {
	tool := NewAgentTool("")
	if tool == nil {
		t.Fatal("NewAgentTool should return non-nil")
	}
}

func TestAgentToolMissingOperation(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing operation")
	}
}

func TestAgentToolInvalidOperation(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "invalid",
	})
	if err == nil {
		t.Error("Expected error for invalid operation")
	}
}

func TestAgentToolListEmpty(t *testing.T) {
	tool := NewAgentTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"operation": "list",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	m := result.(map[string]any)
	if m["count"].(int) != 0 {
		t.Errorf("Expected count=0, got %d", m["count"])
	}
}

func TestAgentToolResumeMissingID(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "resume",
	})
	if err == nil {
		t.Error("Expected error for missing agent_id")
	}
}

func TestAgentToolResumeNotFound(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "resume",
		"agent_id":  "nonexistent",
	})
	if err == nil {
		t.Error("Expected error for nonexistent agent")
	}
}

func TestAgentToolStopMissingID(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "stop",
	})
	if err == nil {
		t.Error("Expected error for missing agent_id")
	}
}

func TestAgentToolStopNotFound(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "stop",
		"agent_id":  "nonexistent",
	})
	if err == nil {
		t.Error("Expected error for nonexistent agent")
	}
}

func TestAgentToolOutputMissingID(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "output",
	})
	if err == nil {
		t.Error("Expected error for missing agent_id")
	}
}

func TestAgentToolOutputNotFound(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "output",
		"agent_id":  "nonexistent",
	})
	if err == nil {
		t.Error("Expected error for nonexistent agent")
	}
}

func TestAgentToolSwitchMissingAgentType(t *testing.T) {
	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "switch",
	})
	if err == nil {
		t.Error("Expected error for missing agent_type")
	}
}

func TestAgentToolSwitchNoRegistry(t *testing.T) {
	orig := globalProfileRegistry
	globalProfileRegistry = nil
	defer func() { globalProfileRegistry = orig }()

	tool := NewAgentTool("")
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation":  "switch",
		"agent_type": "explore",
	})
	if err == nil {
		t.Error("Expected error when profile registry not configured")
	}
}

func TestAgentToolGetAgentDefinitionBuiltin(t *testing.T) {
	tool := NewAgentTool("")

	explore := tool.getAgentDefinition("explore")
	if explore == nil || explore.Name != "explore" {
		t.Error("Should find builtin explore agent")
	}

	verification := tool.getAgentDefinition("verification")
	if verification == nil || verification.Name != "verification" {
		t.Error("Should find builtin verification agent")
	}

	deepResearch := tool.getAgentDefinition("deep-research")
	if deepResearch == nil || deepResearch.Name != "deep-research" {
		t.Error("Should find builtin deep-research agent")
	}
}

func TestAgentToolGetAgentDefinitionCustom(t *testing.T) {
	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "custom-agent")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte("# Custom Agent\ndescription: My custom agent\nmodel: claude-sonnet-4-5\n"), 0644)

	tool := NewAgentTool(tmpDir)
	def := tool.getAgentDefinition("custom-agent")
	if def == nil {
		t.Fatal("Should find custom agent")
	}
	if def.Name != "Custom Agent" {
		t.Errorf("Expected name 'Custom Agent', got %s", def.Name)
	}
	if def.Description != "My custom agent" {
		t.Errorf("Expected description 'My custom agent', got %s", def.Description)
	}
}

func TestAgentToolGetAgentDefinitionUnknownNoDir(t *testing.T) {
	tool := NewAgentTool("")
	def := tool.getAgentDefinition("unknown-type")
	if def != nil {
		t.Error("Should return nil for unknown type with no agentsDir")
	}
}

func TestAgentToolSchema(t *testing.T) {
	tool := NewAgentTool("")
	if tool.Name() != "agent" {
		t.Errorf("Expected name 'agent', got '%s'", tool.Name())
	}
	if tool.InputSchema() == nil {
		t.Error("InputSchema should not be nil")
	}
	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestAgentRegistry(t *testing.T) {
	registry := NewAgentRegistry()

	def := &AgentDefinition{
		Name:        "Test Agent",
		Description: "A test agent",
		Type:        "test",
	}
	registry.Register(def)

	got := registry.Get("test")
	if got == nil || got.Name != "Test Agent" {
		t.Error("Should retrieve registered agent")
	}

	gotNil := registry.Get("nonexistent")
	if gotNil != nil {
		t.Error("Should return nil for nonexistent agent")
	}

	list := registry.List()
	if len(list) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(list))
	}
}

func TestAgentRegistryLoadFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "my-agent")
	os.MkdirAll(agentDir, 0755)
	os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte("# My Agent\ndescription: Loaded from disk\n"), 0644)

	registry := NewAgentRegistry()
	err := registry.LoadFromDirectory(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDirectory failed: %v", err)
	}

	got := registry.Get("my-agent")
	if got == nil {
		t.Fatal("Should find agent loaded from directory")
	}
	if got.Name != "My Agent" {
		t.Errorf("Expected name 'My Agent', got %s", got.Name)
	}
	if got.Description != "Loaded from disk" {
		t.Errorf("Expected description 'Loaded from disk', got %s", got.Description)
	}
}

func TestAgentRegistryLoadFromNonexistentDirectory(t *testing.T) {
	registry := NewAgentRegistry()
	err := registry.LoadFromDirectory("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for nonexistent directory")
	}
}

func TestAgentRegistryToJSON(t *testing.T) {
	registry := NewAgentRegistry()
	registry.Register(&AgentDefinition{
		Name: "JSON Agent",
		Type: "json-agent",
	})

	data, err := registry.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("ToJSON should return non-empty data")
	}
}

func TestSetGlobalProfileRegistry(t *testing.T) {
	orig := globalProfileRegistry
	defer func() { globalProfileRegistry = orig }()

	mockReg := &mockProfileRegistry{}
	SetGlobalProfileRegistry(mockReg)
	if globalProfileRegistry != mockReg {
		t.Error("Should set global profile registry")
	}
}

func TestSetGlobalAgentSwitchFunc(t *testing.T) {
	orig := globalAgentSwitchFunc
	defer func() { globalAgentSwitchFunc = orig }()

	fn := func(cfg *AgentSwitchConfig) error {
		return nil
	}
	SetGlobalAgentSwitchFunc(fn)
	if globalAgentSwitchFunc == nil {
		t.Error("Should set global agent switch func")
	}
}

type mockProfileRegistry struct{}

func (m *mockProfileRegistry) Get(name string) (string, string, string, []string, []string, string, int, error) {
	return name, "test prompt", "test-model", []string{"bash"}, []string{}, "ask", 10, nil
}

func (m *mockProfileRegistry) List() []ProfileEntry {
	return []ProfileEntry{{AgentType: "test"}}
}

func TestAgentToolSwitchWithRegistry(t *testing.T) {
	origReg := globalProfileRegistry
	origFn := globalAgentSwitchFunc
	defer func() {
		globalProfileRegistry = origReg
		globalAgentSwitchFunc = origFn
	}()

	globalProfileRegistry = &mockProfileRegistry{}
	called := false
	globalAgentSwitchFunc = func(cfg *AgentSwitchConfig) error {
		called = true
		if cfg.AgentType != "explore" {
			t.Errorf("Expected agent_type 'explore', got %s", cfg.AgentType)
		}
		return nil
	}

	tool := NewAgentTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"operation":  "switch",
		"agent_type": "explore",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !called {
		t.Error("Agent switch func should have been called")
	}
	m := result.(map[string]any)
	if m["success"] != true {
		t.Error("Expected success=true")
	}
}

func TestGenerateAgentID(t *testing.T) {
	id := generateAgentID()
	if len(id) == 0 {
		t.Error("Agent ID should not be empty")
	}
	if id[:6] != "agent_" {
		t.Errorf("Agent ID should start with 'agent_', got %s", id[:6])
	}
}
