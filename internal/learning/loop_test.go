package learning

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) CreateMessage(_ context.Context, _, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

type mockPromptMemory struct {
	sections map[string][]string
	memory   string
	profile  string
}

func newMockPromptMemory() *mockPromptMemory {
	return &mockPromptMemory{sections: make(map[string][]string)}
}

func (m *mockPromptMemory) AppendToSection(section, line string) error {
	m.sections[section] = append(m.sections[section], line)
	return nil
}

func (m *mockPromptMemory) UpdateMemory(content string) error {
	m.memory = content
	return nil
}

func (m *mockPromptMemory) UpdateUserProfile(profile string) error {
	m.profile = profile
	return nil
}

func (m *mockPromptMemory) AutoLoad() string {
	return m.memory + "\n" + m.profile
}

func TestNudgeEngine_MaybeNudge(t *testing.T) {
	ne := NewNudgeEngine(NudgeConfig{Interval: 10, FlushMinTurns: 6})

	if ne.MaybeNudge(5) != nil {
		t.Error("should not nudge before FlushMinTurns")
	}
	if ne.MaybeNudge(7) != nil {
		t.Error("should not nudge when turn % interval != 0")
	}
	if ne.MaybeNudge(10) == nil {
		t.Error("should nudge at turn 10 (>=6 and 10%10==0)")
	}
	if ne.MaybeNudge(20) == nil {
		t.Error("should nudge at turn 20")
	}
}

func TestNudgeEngine_DefaultConfig(t *testing.T) {
	cfg := DefaultNudgeConfig()
	if cfg.Interval != 10 {
		t.Errorf("Interval = %d, want 10", cfg.Interval)
	}
	if cfg.FlushMinTurns != 6 {
		t.Errorf("FlushMinTurns = %d, want 6", cfg.FlushMinTurns)
	}
}

func TestNudgeEngine_ZeroConfig(t *testing.T) {
	ne := NewNudgeEngine(NudgeConfig{})
	cfg := ne.GetConfig()
	if cfg.Interval != 10 {
		t.Errorf("Interval = %d, want 10", cfg.Interval)
	}
	if cfg.FlushMinTurns != 6 {
		t.Errorf("FlushMinTurns = %d, want 6", cfg.FlushMinTurns)
	}
}

func TestEvaluator_EvaluateWorthKeeping(t *testing.T) {
	client := &mockLLMClient{
		response: `{"worth_keeping": true, "reason": "multi-step debugging approach", "key_steps": ["reproduce", "isolate", "fix"], "skill_name": "debug-go-tests", "skill_category": "debugging"}`,
	}
	eval := NewEvaluator(client)

	messages := []Message{
		{Role: "user", Content: "fix the failing test", Timestamp: time.Now()},
		{Role: "assistant", Content: "I'll reproduce the issue first", Timestamp: time.Now()},
		{Role: "user", Content: "here's the error output", Timestamp: time.Now()},
		{Role: "assistant", Content: "found the root cause, applying fix", Timestamp: time.Now()},
	}

	result, err := eval.Evaluate(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.WorthKeeping {
		t.Error("should be worth keeping")
	}
	if result.SkillName != "debug-go-tests" {
		t.Errorf("SkillName = %q, want %q", result.SkillName, "debug-go-tests")
	}
}

func TestEvaluator_EvaluateNotWorthKeeping(t *testing.T) {
	client := &mockLLMClient{
		response: `{"worth_keeping": false, "reason": "trivial one-off task", "key_steps": [], "skill_name": "", "skill_category": ""}`,
	}
	eval := NewEvaluator(client)

	messages := []Message{
		{Role: "user", Content: "what is 2+2?", Timestamp: time.Now()},
		{Role: "assistant", Content: "4", Timestamp: time.Now()},
	}

	result, err := eval.Evaluate(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.WorthKeeping {
		t.Error("should not be worth keeping")
	}
}

func TestEvaluator_EvaluateBadResponse(t *testing.T) {
	client := &mockLLMClient{response: "not json"}
	eval := NewEvaluator(client)

	messages := []Message{
		{Role: "user", Content: "test", Timestamp: time.Now()},
		{Role: "assistant", Content: "response", Timestamp: time.Now()},
	}

	result, err := eval.Evaluate(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("Evaluate should not error on bad response: %v", err)
	}
	if result.WorthKeeping {
		t.Error("should default to not worth keeping on parse failure")
	}
}

func TestExtractor_Extract(t *testing.T) {
	client := &mockLLMClient{
		response: `{"name": "debug-go-tests", "description": "Debug failing Go tests systematically", "triggers": ["/debug-tests", "failing test"], "steps": ["Reproduce the failure", "Isolate the cause", "Apply minimal fix", "Verify with test run"], "tools": ["bash", "read_file", "grep"], "tags": ["debugging", "go", "testing"]}`,
	}
	ext := NewExtractor(client)

	evaluation := &TaskEvaluation{
		WorthKeeping:  true,
		Reason:        "multi-step debugging",
		KeySteps:      []string{"reproduce", "isolate", "fix"},
		SkillName:     "debug-go-tests",
		SkillCategory: "debugging",
	}

	messages := []Message{
		{Role: "user", Content: "fix test", Timestamp: time.Now()},
		{Role: "assistant", Content: "debugging", Timestamp: time.Now()},
	}

	skill, err := ext.Extract(context.Background(), messages, evaluation)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if skill.Name != "debug-go-tests" {
		t.Errorf("Name = %q, want %q", skill.Name, "debug-go-tests")
	}
	if len(skill.Steps) != 4 {
		t.Errorf("Steps count = %d, want 4", len(skill.Steps))
	}
}

func TestSkillWriter_WriteSkill(t *testing.T) {
	dir := t.TempDir()
	sw := NewSkillWriter(dir)

	skill := &ExtractedSkill{
		Name:        "test-skill",
		Description: "A test skill",
		Triggers:    []string{"/test-skill"},
		Steps:       []string{"Step 1", "Step 2"},
		Tools:       []string{"bash"},
		Tags:        []string{"test"},
	}

	if err := sw.WriteSkill(skill); err != nil {
		t.Fatalf("WriteSkill: %v", err)
	}

	skillPath := filepath.Join(dir, "test-skill", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("SKILL.md should exist")
	}

	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}

	content := string(data)
	if content == "" {
		t.Error("SKILL.md should not be empty")
	}
}

func TestSkillWriter_BackupExisting(t *testing.T) {
	dir := t.TempDir()
	sw := NewSkillWriter(dir)

	skill := &ExtractedSkill{
		Name:        "test-skill",
		Description: "v1",
		Triggers:    []string{"/test"},
		Steps:       []string{"step1"},
		Tools:       []string{"bash"},
		Tags:        []string{"test"},
	}

	if err := sw.WriteSkill(skill); err != nil {
		t.Fatalf("WriteSkill v1: %v", err)
	}

	skill.Description = "v2"
	if err := sw.WriteSkill(skill); err != nil {
		t.Fatalf("WriteSkill v2: %v", err)
	}

	backupPath := filepath.Join(dir, "test-skill", "SKILL.md.bak")
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("SKILL.md.bak should exist after overwrite")
	}
}

func TestLearningLoop_OnTaskComplete(t *testing.T) {
	dir := t.TempDir()
	client := &mockLLMClient{
		response: `{"worth_keeping": true, "reason": "reusable pattern", "key_steps": ["step1"], "skill_name": "test-skill", "skill_category": "test"}`,
	}
	pm := newMockPromptMemory()

	loop := NewLearningLoop(client, pm, dir)
	if !loop.IsEnabled() {
		t.Fatal("loop should be enabled")
	}

	messages := []Message{
		{Role: "user", Content: "do something complex", Timestamp: time.Now()},
		{Role: "assistant", Content: "I'll follow a systematic approach", Timestamp: time.Now()},
		{Role: "user", Content: "looks good", Timestamp: time.Now()},
		{Role: "assistant", Content: "done", Timestamp: time.Now()},
	}

	if err := loop.OnTaskComplete(context.Background(), "test-session", messages, nil); err != nil {
		t.Fatalf("OnTaskComplete: %v", err)
	}

	skillPath := filepath.Join(dir, "test-skill", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("skill should be written to disk")
	}

	if len(pm.sections["Learned Patterns"]) == 0 {
		t.Error("MEMORY.md should be updated with learned pattern")
	}
}

func TestLearningLoop_DisabledWhenNoClient(t *testing.T) {
	loop := NewLearningLoop(nil, nil, "")
	if loop.IsEnabled() {
		t.Error("loop should be disabled when no LLM client")
	}
}

func TestLearningLoop_SkipsShortSessions(t *testing.T) {
	dir := t.TempDir()
	client := &mockLLMClient{}
	pm := newMockPromptMemory()
	loop := NewLearningLoop(client, pm, dir)

	messages := []Message{
		{Role: "user", Content: "hi", Timestamp: time.Now()},
		{Role: "assistant", Content: "hello", Timestamp: time.Now()},
	}

	if err := loop.OnTaskComplete(context.Background(), "test", messages, nil); err != nil {
		t.Fatalf("OnTaskComplete: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) > 0 {
		t.Error("no skill should be created for short sessions")
	}
}
