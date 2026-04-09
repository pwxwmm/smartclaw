package runtime

import (
	"testing"
)

func TestNewContextManager(t *testing.T) {
	cm := NewContextManager(1000)

	if cm == nil {
		t.Error("ContextManager should not be nil")
	}

	if cm.maxTokens != 1000 {
		t.Errorf("Expected maxTokens 1000, got %d", cm.maxTokens)
	}
}

func TestContextManager_AddMessage(t *testing.T) {
	cm := NewContextManager(1000)

	msg := Message{
		Role:    "user",
		Content: "test message",
	}

	cm.AddMessage(msg)

	if len(cm.GetMessages()) != 1 {
		t.Errorf("Expected 1 message, got %d", len(cm.GetMessages()))
	}
}

func TestContextManager_Clear(t *testing.T) {
	cm := NewContextManager(1000)

	cm.AddMessage(Message{Role: "user", Content: "test"})
	cm.Clear()

	if len(cm.GetMessages()) != 0 {
		t.Errorf("Expected 0 messages after clear, got %d", len(cm.GetMessages()))
	}
}

func TestCountTokens(t *testing.T) {
	text := "hello world test"
	count := CountTokens(text)

	if count != 3 {
		t.Errorf("Expected 3 tokens, got %d", count)
	}
}

func TestShouldCompact(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "test message one"},
		{Role: "assistant", Content: "test message two"},
	}

	if ShouldCompact(messages, 1000) {
		t.Error("Should not compact with high token limit")
	}

	if !ShouldCompact(messages, 1) {
		t.Error("Should compact with low token limit")
	}
}

func TestCompact(t *testing.T) {
	messages := make([]Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = Message{
			Role:    "user",
			Content: "test message",
		}
	}

	compacted := Compact(messages, 1)

	if len(compacted) >= len(messages) {
		t.Error("Compacted messages should be fewer than original")
	}
}

func TestNewQueryState(t *testing.T) {
	state := NewQueryState()

	if state == nil {
		t.Error("QueryState should not be nil")
	}

	if len(state.GetMessages()) != 0 {
		t.Error("New QueryState should have no messages")
	}
}

func TestQueryState_AddMessage(t *testing.T) {
	state := NewQueryState()

	state.AddMessage(Message{Role: "user", Content: "test"})

	if len(state.GetMessages()) != 1 {
		t.Errorf("Expected 1 message, got %d", len(state.GetMessages()))
	}
}

func TestQueryState_UpdateUsage(t *testing.T) {
	state := NewQueryState()

	state.UpdateUsage(Usage{InputTokens: 100, OutputTokens: 50})

	usage := state.GetUsage()
	if usage.InputTokens != 100 {
		t.Errorf("Expected input tokens 100, got %d", usage.InputTokens)
	}

	if usage.OutputTokens != 50 {
		t.Errorf("Expected output tokens 50, got %d", usage.OutputTokens)
	}
}
