package assistant

import (
	"context"
	"strings"
	"testing"
)

func TestNewAssistant(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "You are helpful")
	if a.Name != "test" {
		t.Errorf("Name = %q, want %q", a.Name, "test")
	}
	if a.Persona != "You are helpful" {
		t.Errorf("Persona = %q, want %q", a.Persona, "You are helpful")
	}
	if a.Variables == nil {
		t.Error("Variables should not be nil")
	}
	if len(a.Variables) != 0 {
		t.Errorf("Variables should be empty, got %d items", len(a.Variables))
	}
	if a.Context == nil {
		t.Error("Context should not be nil")
	}
	if len(a.Context) != 0 {
		t.Errorf("Context should be empty, got %d items", len(a.Context))
	}
}

func TestSetModel(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "persona")
	a.SetModel("claude-opus-4-6")
	if a.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", a.Model, "claude-opus-4-6")
	}
}

func TestSetVariable(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "persona")
	a.SetVariable("key1", "value1")
	if a.Variables["key1"] != "value1" {
		t.Errorf("Variables[\"key1\"] = %q, want %q", a.Variables["key1"], "value1")
	}
}

func TestGetVariable(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "persona")
	a.SetVariable("key1", "value1")
	if got := a.GetVariable("key1"); got != "value1" {
		t.Errorf("GetVariable(\"key1\") = %q, want %q", got, "value1")
	}
}

func TestGetVariable_NotSet(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "persona")
	if got := a.GetVariable("nonexistent"); got != "" {
		t.Errorf("GetVariable(\"nonexistent\") = %q, want empty string", got)
	}
}

func TestSetVariable_Overwrite(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "persona")
	a.SetVariable("key1", "first")
	a.SetVariable("key1", "second")
	if got := a.GetVariable("key1"); got != "second" {
		t.Errorf("GetVariable(\"key1\") = %q, want %q (overwritten)", got, "second")
	}
}

func TestAddContext(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "persona")
	a.AddContext("context line 1")
	a.AddContext("context line 2")
	if len(a.Context) != 2 {
		t.Fatalf("Context length = %d, want 2", len(a.Context))
	}
	if a.Context[0] != "context line 1" {
		t.Errorf("Context[0] = %q, want %q", a.Context[0], "context line 1")
	}
	if a.Context[1] != "context line 2" {
		t.Errorf("Context[1] = %q, want %q", a.Context[1], "context line 2")
	}
}

func TestClearContext(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "persona")
	a.AddContext("some context")
	a.ClearContext()
	if len(a.Context) != 0 {
		t.Errorf("Context length after ClearContext = %d, want 0", len(a.Context))
	}
}

func TestGeneratePrompt(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "You are a coding assistant")
	a.AddContext("Project: Go")
	prompt := a.GeneratePrompt("Write a function")

	if !strings.Contains(prompt, "You are a coding assistant") {
		t.Error("Prompt should contain persona")
	}
	if !strings.Contains(prompt, "Project: Go") {
		t.Error("Prompt should contain context")
	}
	if !strings.Contains(prompt, "User: Write a function") {
		t.Error("Prompt should contain user input")
	}
}

func TestGeneratePrompt_EmptyContext(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test", "Hello")
	prompt := a.GeneratePrompt("test input")

	want := "Hello\n\nUser: test input"
	if prompt != want {
		t.Errorf("GeneratePrompt() = %q, want %q", prompt, want)
	}
}

func TestProcess(t *testing.T) {
	t.Parallel()

	a := NewAssistant("test-bot", "You are helpful")
	result, err := a.Process(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Process() returned error: %v", err)
	}
	if !strings.Contains(result, "[test-bot]") {
		t.Error("Process result should contain assistant name")
	}
	if !strings.Contains(result, "Processed:") {
		t.Error("Process result should contain 'Processed:'")
	}
	if !strings.Contains(result, "hello") {
		t.Error("Process result should contain input text")
	}
}

func TestNewAssistantManager(t *testing.T) {
	t.Parallel()

	m := NewAssistantManager()
	if m == nil {
		t.Fatal("NewAssistantManager() returned nil")
	}
	if len(m.assistants) != 0 {
		t.Errorf("new AssistantManager should have empty assistants, got %d", len(m.assistants))
	}
	if m.active != nil {
		t.Error("new AssistantManager should have nil active")
	}
}

func TestAssistantManager_Register(t *testing.T) {
	t.Parallel()

	m := NewAssistantManager()
	a := NewAssistant("bot1", "persona1")
	m.Register(a)

	got := m.Get("bot1")
	if got == nil {
		t.Fatal("Get(\"bot1\") returned nil")
	}
	if got.Name != "bot1" {
		t.Errorf("Get(\"bot1\").Name = %q, want %q", got.Name, "bot1")
	}
}

func TestAssistantManager_Get_NotFound(t *testing.T) {
	t.Parallel()

	m := NewAssistantManager()
	got := m.Get("nonexistent")
	if got != nil {
		t.Error("Get(\"nonexistent\") should return nil")
	}
}

func TestAssistantManager_SetActive(t *testing.T) {
	t.Parallel()

	m := NewAssistantManager()
	a := NewAssistant("bot1", "persona1")
	m.Register(a)

	err := m.SetActive("bot1")
	if err != nil {
		t.Fatalf("SetActive() returned error: %v", err)
	}
	if m.Active() != a {
		t.Error("Active() should return the set assistant")
	}
}

func TestAssistantManager_SetActive_NotFound(t *testing.T) {
	t.Parallel()

	m := NewAssistantManager()
	err := m.SetActive("nonexistent")
	if err == nil {
		t.Error("SetActive(\"nonexistent\") should return error")
	}
}

func TestAssistantManager_Active_NoneSet(t *testing.T) {
	t.Parallel()

	m := NewAssistantManager()
	if m.Active() != nil {
		t.Error("Active() with no active set should return nil")
	}
}

func TestAssistantManager_List(t *testing.T) {
	m := NewAssistantManager()
	m.Register(NewAssistant("bot1", "p1"))
	m.Register(NewAssistant("bot2", "p2"))
	m.Register(NewAssistant("bot3", "p3"))

	names := m.List()
	if len(names) != 3 {
		t.Fatalf("List() returned %d items, want 3", len(names))
	}
}

func TestAssistantManager_List_Empty(t *testing.T) {
	t.Parallel()

	m := NewAssistantManager()
	names := m.List()
	if len(names) != 0 {
		t.Errorf("List() on empty manager returned %d items, want 0", len(names))
	}
}

func TestAssistant_FullWorkflow(t *testing.T) {
	a := NewAssistant("coder", "You write clean Go code")
	a.SetModel("claude-sonnet-4-5")
	a.SetVariable("lang", "go")
	a.AddContext("Project uses Go 1.25")
	a.AddContext("Follow standard project layout")

	prompt := a.GeneratePrompt("Write a test")
	if !strings.Contains(prompt, "You write clean Go code") {
		t.Error("Prompt missing persona")
	}
	if !strings.Contains(prompt, "Go 1.25") {
		t.Error("Prompt missing first context")
	}
	if !strings.Contains(prompt, "standard project layout") {
		t.Error("Prompt missing second context")
	}
	if !strings.Contains(prompt, "Write a test") {
		t.Error("Prompt missing user input")
	}
}
