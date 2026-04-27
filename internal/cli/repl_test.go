package cli

import (
	"testing"

	"github.com/instructkr/smartclaw/internal/runtime"
)

func TestHandleSlashCommand_Status(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
		model:   "test-model",
	}
	result := s.handleSlashCommand("/status")
	if result {
		t.Error("/status should return false (not exit)")
	}
}

func TestHandleSlashCommand_Cost(t *testing.T) {
	s := &REPLSession{
		session:    &runtime.Session{Messages: []runtime.Message{}},
		totalCost:  1.23,
		totalTokens: 5000,
	}
	result := s.handleSlashCommand("/cost")
	if result {
		t.Error("/cost should return false (not exit)")
	}
}

func TestHandleSlashCommand_Clear(t *testing.T) {
	sess := &runtime.Session{
		Messages: []runtime.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
	}
	s := &REPLSession{
		session: sess,
		ctxMgr:  runtime.NewContextManager(100000),
	}
	result := s.handleSlashCommand("/clear")
	if result {
		t.Error("/clear should return false (not exit)")
	}
	if sess.MessageCount() != 0 {
		t.Errorf("session should be cleared after /clear, got %d messages", sess.MessageCount())
	}
}

func TestHandleSlashCommand_Model_WithArg(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
		model:   "old-model",
	}
	s.handleSlashCommand("/model claude-opus-4-6")
	if s.model != "claude-opus-4-6" {
		t.Errorf("model = %q, want %q", s.model, "claude-opus-4-6")
	}
}

func TestHandleSlashCommand_Model_WithMultipleArgs(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
		model:   "old",
	}
	s.handleSlashCommand("/model new-model extra")
	if s.model != "new-model" {
		t.Errorf("model = %q, want first arg %q", s.model, "new-model")
	}
}

func TestHandleSlashCommand_Resume_WithoutArg(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
	}
	result := s.handleSlashCommand("/resume")
	if result {
		t.Error("/resume without arg should return false (not exit)")
	}
}

func TestHandleSlashCommand_Compact(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
		ctxMgr:  runtime.NewContextManager(100000),
	}
	result := s.handleSlashCommand("/compact")
	if result {
		t.Error("/compact should return false (not exit)")
	}
}

func TestHandleSlashCommand_Help_NotExit(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
	}
	result := s.handleSlashCommand("/help")
	if result {
		t.Error("/help should return false (not exit)")
	}
}

func TestHandleSlashCommand_ExitVariants(t *testing.T) {
	s := &REPLSession{
		session: &runtime.Session{Messages: []runtime.Message{}},
	}
	if !s.handleSlashCommand("/exit") {
		t.Error("/exit should return true")
	}
	if !s.handleSlashCommand("/quit") {
		t.Error("/quit should return true")
	}
}

func TestBuildAPIMessages_WithNonStringContent(t *testing.T) {
	sess := &runtime.Session{
		Messages: []runtime.Message{
			{Role: "user", Content: 42},
			{Role: "assistant", Content: "response"},
		},
	}
	s := &REPLSession{session: sess}
	messages := s.buildAPIMessages()
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Content != "" {
		t.Errorf("non-string content should result in empty string, got %q", messages[0].Content)
	}
	if messages[1].Content != "response" {
		t.Errorf("string content should be preserved, got %q", messages[1].Content)
	}
}

func TestLastUserInput_NoUserMessages(t *testing.T) {
	sess := &runtime.Session{
		Messages: []runtime.Message{
			{Role: "assistant", Content: "hello"},
		},
	}
	s := &REPLSession{session: sess}
	result := s.lastUserInput()
	if result != "" {
		t.Errorf("expected empty string with no user messages, got %q", result)
	}
}

func TestLastUserInput_MixedContent(t *testing.T) {
	sess := &runtime.Session{
		Messages: []runtime.Message{
			{Role: "user", Content: 42},
			{Role: "user", Content: "real question"},
		},
	}
	s := &REPLSession{session: sess}
	result := s.lastUserInput()
	if result != "real question" {
		t.Errorf("expected 'real question', got %q", result)
	}
}

func TestUpdateCost_Accumulation(t *testing.T) {
	s := &REPLSession{}
	s.updateCost(1000, 0)
	firstCost := s.totalCost
	s.updateCost(0, 500)
	secondCost := s.totalCost
	if secondCost <= firstCost {
		t.Errorf("cost should accumulate: before=%f, after=%f", firstCost, secondCost)
	}
}

func TestUpdateCost_Calculation(t *testing.T) {
	s := &REPLSession{}
	s.updateCost(1000, 500)
	expected := float64(1000)*0.000003 + float64(500)*0.000015
	if s.totalCost != expected {
		t.Errorf("totalCost = %f, want %f", s.totalCost, expected)
	}
}
