package compact

import (
	"context"
	"testing"
	"time"
)

func TestDefaultCompactConfig(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if config.Model != "claude-sonnet-4-5" {
		t.Errorf("Expected model 'claude-sonnet-4-5', got '%s'", config.Model)
	}

	if config.ContextWindow != 200000 {
		t.Errorf("Expected context window 200000, got %d", config.ContextWindow)
	}

	if !config.AutoCompactEnabled {
		t.Error("Expected auto compact to be enabled")
	}
}

func TestNewCompactService(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	service := NewCompactService(config)

	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	if service.config != config {
		t.Error("Expected config to be set")
	}
}

func TestShouldCompact(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	service := NewCompactService(config)

	should, warning := service.ShouldCompact(1000)
	if should {
		t.Error("Expected no compact for low token usage")
	}

	should, warning = service.ShouldCompact(180000)
	if !should {
		t.Error("Expected compact for high token usage")
	}

	if warning.Level == "" {
		t.Error("Expected warning level to be set")
	}
}

func TestShouldCompactCritical(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	service := NewCompactService(config)

	should, warning := service.ShouldCompact(195000)
	if !should {
		t.Error("Expected compact for critical token usage")
	}

	if warning.Level != "critical" {
		t.Errorf("Expected warning level 'critical', got '%s'", warning.Level)
	}
}

func TestCompactNotEnoughMessages(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	service := NewCompactService(config)

	messages := []Message{
		&BaseMessage{Type: "user", Role: "user", Content: "hello"},
	}

	result, err := service.Compact(context.Background(), messages, CompactReasonManual)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Success {
		t.Error("Expected failure for insufficient messages")
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	service := NewCompactService(config)

	stats := service.GetStats()
	if stats.TokensBefore != 0 {
		t.Error("Expected initial tokens before to be 0")
	}
}

func TestGetState(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	service := NewCompactService(config)

	state := service.GetState()
	if state.ConsecutiveFailures != 0 {
		t.Error("Expected initial consecutive failures to be 0")
	}
}

func TestResetConsecutiveFailures(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	service := NewCompactService(config)

	service.ResetConsecutiveFailures()
}

func TestUpdateConfig(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	service := NewCompactService(config)

	newConfig := DefaultCompactConfig("claude-opus-4-6", 300000)
	service.UpdateConfig(newConfig)

	if service.config.Model != "claude-opus-4-6" {
		t.Error("Expected config to be updated")
	}
}

func TestEstimateTokens(t *testing.T) {
	messages := []Message{
		&BaseMessage{Type: "user", Role: "user", Content: "hello world"},
	}

	tokens := estimateTokens(messages)
	if tokens <= 0 {
		t.Error("Expected positive token count")
	}
}

func TestEstimateTokensWithContentBlocks(t *testing.T) {
	messages := []Message{
		&AssistantMessage{
			BaseMessage: BaseMessage{Type: "assistant", Role: "assistant"},
			Content: []ContentBlock{
				{Type: "text", Text: "This is a response"},
			},
		},
	}

	tokens := estimateTokens(messages)
	if tokens <= 0 {
		t.Error("Expected positive token count for content blocks")
	}
}

func TestStripImagesFromMessages(t *testing.T) {
	messages := []Message{
		&BaseMessage{Type: "user", Role: "user", Content: "hello"},
	}

	result := stripImagesFromMessages(messages)
	if len(result) != 1 {
		t.Errorf("Expected 1 message, got %d", len(result))
	}
}

func TestStripReinjectedAttachments(t *testing.T) {
	messages := []Message{
		&BaseMessage{Type: "user", Role: "user", Content: "hello"},
		&BaseMessage{Type: "attachment", Role: "user", Content: "file"},
	}

	result := stripReinjectedAttachments(messages)
	if len(result) != 1 {
		t.Errorf("Expected 1 message after stripping attachments, got %d", len(result))
	}
}

func TestGroupMessagesByApiRound(t *testing.T) {
	messages := []Message{
		&BaseMessage{Type: "user", Role: "user", Content: "question"},
		&BaseMessage{Type: "assistant", Role: "assistant", Content: "answer"},
		&BaseMessage{Type: "user", Role: "user", Content: "follow-up"},
		&BaseMessage{Type: "assistant", Role: "assistant", Content: "response"},
	}

	groups := groupMessagesByApiRound(messages)
	if len(groups) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(groups))
	}
}

func TestTruncateText(t *testing.T) {
	text := "This is a long text that should be truncated"
	result := truncateText(text, 10)

	if len(result) > 13 {
		t.Errorf("Expected truncated text to be <= 13 chars, got %d", len(result))
	}

	if result[:10] != text[:10] {
		t.Error("Expected first 10 chars to match")
	}
}

func TestTruncateTextShort(t *testing.T) {
	text := "short"
	result := truncateText(text, 100)

	if result != text {
		t.Error("Expected short text to remain unchanged")
	}
}

func TestCreateSummaryMessage(t *testing.T) {
	summary := "Test summary content"
	msg := createSummaryMessage(summary)

	if msg == nil {
		t.Fatal("Expected non-nil message")
	}

	if msg.Summary != summary {
		t.Errorf("Expected summary '%s', got '%s'", summary, msg.Summary)
	}

	if !msg.IsCompacted {
		t.Error("Expected IsCompacted to be true")
	}

	if msg.Type != "compact_summary" {
		t.Errorf("Expected type 'compact_summary', got '%s'", msg.Type)
	}
}

func TestCompactReason(t *testing.T) {
	reasons := []CompactReason{
		CompactReasonAuto,
		CompactReasonManual,
		CompactReasonPartial,
	}

	for _, reason := range reasons {
		if reason == "" {
			t.Error("Compact reason should not be empty")
		}
	}
}

func TestCompactStats(t *testing.T) {
	stats := CompactStats{
		TokensBefore:     10000,
		TokensAfter:      3000,
		TokensSaved:      7000,
		MessagesBefore:   20,
		MessagesAfter:    5,
		CompactDuration:  time.Second,
		SummaryTokens:    500,
		SummaryGenerated: true,
		Reason:           CompactReasonAuto,
		Timestamp:        time.Now(),
	}

	if stats.TokensSaved != 7000 {
		t.Errorf("Expected tokens saved 7000, got %d", stats.TokensSaved)
	}
}

func TestCompactWarning(t *testing.T) {
	warning := CompactWarning{
		Level:       "warning",
		TokensUsed:  150000,
		TokensLimit: 180000,
		PercentLeft: 16.67,
		Message:     "Context window running low",
	}

	if warning.Level != "warning" {
		t.Errorf("Expected level 'warning', got '%s'", warning.Level)
	}
}

func TestCompactResult(t *testing.T) {
	result := CompactResult{
		Success:      true,
		Summary:      "Test summary",
		MessagesTrim: 15,
	}

	if !result.Success {
		t.Error("Expected success to be true")
	}
}

func TestMicroCompact(t *testing.T) {
	mc := NewMicroCompact()

	if mc == nil {
		t.Fatal("Expected non-nil MicroCompact")
	}
}

func TestMicroCompactIsCompactable(t *testing.T) {
	mc := NewMicroCompact()

	if !mc.IsCompactable("read_file") {
		t.Error("Expected read_file to be compactable")
	}

	if mc.IsCompactable("unknown_tool") {
		t.Error("Expected unknown_tool not to be compactable")
	}
}

func TestMicroCompactCompactToolResult(t *testing.T) {
	mc := NewMicroCompact()

	content := "This is a very long content that should be truncated if it exceeds the token limit"
	result := mc.CompactToolResult("read_file", content, 10)

	if result == content {
		t.Error("Expected content to be truncated")
	}
}

func TestMicroCompactAddRemoveTool(t *testing.T) {
	mc := NewMicroCompact()

	mc.AddCompactableTool("custom_tool")
	if !mc.IsCompactable("custom_tool") {
		t.Error("Expected custom_tool to be compactable after adding")
	}

	mc.RemoveCompactableTool("custom_tool")
	if mc.IsCompactable("custom_tool") {
		t.Error("Expected custom_tool not to be compactable after removing")
	}
}

func TestDefaultTimeBasedCompactConfig(t *testing.T) {
	config := DefaultTimeBasedCompactConfig()

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if !config.Enabled {
		t.Error("Expected enabled to be true")
	}

	if config.MaxAge != 30*time.Minute {
		t.Errorf("Expected max age 30m, got %v", config.MaxAge)
	}
}

func TestNewTimeBasedCompact(t *testing.T) {
	config := DefaultTimeBasedCompactConfig()
	tbc := NewTimeBasedCompact(config)

	if tbc == nil {
		t.Fatal("Expected non-nil TimeBasedCompact")
	}
}

func TestTimeBasedCompactShouldCompact(t *testing.T) {
	config := DefaultTimeBasedCompactConfig()
	config.CompactInterval = time.Millisecond * 100
	tbc := NewTimeBasedCompact(config)

	if tbc.ShouldCompact() {
		t.Error("Expected should compact to be false initially (just created)")
	}
}

func TestMarshalCompactStats(t *testing.T) {
	stats := CompactStats{
		TokensBefore:   10000,
		TokensAfter:    3000,
		MessagesBefore: 20,
		MessagesAfter:  5,
		Reason:         CompactReasonAuto,
	}

	data, err := MarshalCompactStats(stats)
	if err != nil {
		t.Fatalf("Expected no error marshalling, got %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty data")
	}
}

func TestUnmarshalCompactStats(t *testing.T) {
	stats := CompactStats{
		TokensBefore:   10000,
		TokensAfter:    3000,
		MessagesBefore: 20,
		MessagesAfter:  5,
		Reason:         CompactReasonAuto,
	}

	data, _ := MarshalCompactStats(stats)
	result, err := UnmarshalCompactStats(data)

	if err != nil {
		t.Fatalf("Expected no error unmarshalling, got %v", err)
	}

	if result.TokensBefore != 10000 {
		t.Errorf("Expected tokens before 10000, got %d", result.TokensBefore)
	}
}

func TestBaseMessage(t *testing.T) {
	msg := &BaseMessage{
		Type:    "user",
		Role:    "user",
		Content: "Hello",
	}

	if msg.GetType() != "user" {
		t.Errorf("Expected type 'user', got '%s'", msg.GetType())
	}

	if msg.GetRole() != "user" {
		t.Errorf("Expected role 'user', got '%s'", msg.GetRole())
	}

	if msg.GetContent() != "Hello" {
		t.Errorf("Expected content 'Hello', got '%v'", msg.GetContent())
	}
}

func TestContentBlock(t *testing.T) {
	block := ContentBlock{
		Type:  "text",
		Text:  "Hello world",
		ID:    "block-1",
		Name:  "test",
		Input: map[string]interface{}{"key": "value"},
	}

	if block.Type != "text" {
		t.Errorf("Expected type 'text', got '%s'", block.Type)
	}

	if block.Text != "Hello world" {
		t.Errorf("Expected text 'Hello world', got '%s'", block.Text)
	}
}

func TestAutoCompactDisabled(t *testing.T) {
	config := DefaultCompactConfig("claude-sonnet-4-5", 200000)
	config.AutoCompactEnabled = false
	service := NewCompactService(config)

	should, _ := service.ShouldCompact(180000)
	if should {
		t.Error("Expected no auto compact when disabled")
	}
}

func TestCompactConstants(t *testing.T) {
	if AutoCompactBufferTokens != 13000 {
		t.Errorf("Expected AutoCompactBufferTokens 13000, got %d", AutoCompactBufferTokens)
	}

	if WarningThresholdBufferTokens != 20000 {
		t.Errorf("Expected WarningThresholdBufferTokens 20000, got %d", WarningThresholdBufferTokens)
	}

	if MaxConsecutiveFailures != 3 {
		t.Errorf("Expected MaxConsecutiveFailures 3, got %d", MaxConsecutiveFailures)
	}
}
