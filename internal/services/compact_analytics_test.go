package services

import (
	"context"
	"testing"
)

func TestCompactService_DefaultThreshold(t *testing.T) {
	s := NewCompactService(0)
	if s.GetThreshold() != 100000 {
		t.Errorf("default threshold = %d, want 100000", s.GetThreshold())
	}
}

func TestCompactService_NegativeThreshold(t *testing.T) {
	s := NewCompactService(-5)
	if s.GetThreshold() != 100000 {
		t.Errorf("negative threshold should default to 100000, got %d", s.GetThreshold())
	}
}

func TestCompactService_CustomThreshold(t *testing.T) {
	s := NewCompactService(50000)
	if s.GetThreshold() != 50000 {
		t.Errorf("threshold = %d, want 50000", s.GetThreshold())
	}
}

func TestCompactService_SetThreshold(t *testing.T) {
	s := NewCompactService(100000)
	s.SetThreshold(200000)
	if s.GetThreshold() != 200000 {
		t.Errorf("after SetThreshold(200000), got %d", s.GetThreshold())
	}
}

func TestCompactService_ShouldCompact_Below(t *testing.T) {
	s := NewCompactService(1000)
	if s.ShouldCompact(999) {
		t.Error("ShouldCompact(999) with threshold 1000 should be false")
	}
}

func TestCompactService_ShouldCompact_At(t *testing.T) {
	s := NewCompactService(1000)
	if !s.ShouldCompact(1000) {
		t.Error("ShouldCompact(1000) with threshold 1000 should be true")
	}
}

func TestCompactService_ShouldCompact_Above(t *testing.T) {
	s := NewCompactService(1000)
	if !s.ShouldCompact(1500) {
		t.Error("ShouldCompact(1500) with threshold 1000 should be true")
	}
}

func TestCompactService_Compact_ShortConversation(t *testing.T) {
	s := NewCompactService(1000)
	msgs := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	result, err := s.Compact(context.Background(), msgs)
	if err != nil {
		t.Fatalf("Compact returned error: %v", err)
	}
	if len(result) != len(msgs) {
		t.Errorf("short conversation should not be compacted, got %d messages", len(result))
	}
}

func TestCompactService_Compact_LongConversation(t *testing.T) {
	s := NewCompactService(1000)
	msgs := []Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
		{Role: "user", Content: "q3"},
		{Role: "assistant", Content: "a3"},
	}
	result, err := s.Compact(context.Background(), msgs)
	if err != nil {
		t.Fatalf("Compact returned error: %v", err)
	}
	if len(result) >= len(msgs) {
		t.Errorf("long conversation should be compacted, got %d messages (was %d)", len(result), len(msgs))
	}
	if result[0].Role != "user" {
		t.Errorf("first message should be preserved as user, got %q", result[0].Role)
	}
}

func TestAutoCompact_DefaultMaxTokens(t *testing.T) {
	s := NewCompactService(1000)
	a := NewAutoCompact(s, 0)
	if a.maxTokens != 200000 {
		t.Errorf("default maxTokens = %d, want 200000", a.maxTokens)
	}
}

func TestAutoCompact_NoCompactNeeded(t *testing.T) {
	s := NewCompactService(10000)
	a := NewAutoCompact(s, 200000)
	msgs := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	result, compacted, err := a.CheckAndCompact(context.Background(), msgs, 5000)
	if err != nil {
		t.Fatalf("CheckAndCompact returned error: %v", err)
	}
	if compacted {
		t.Error("should not compact when below threshold")
	}
	if len(result) != len(msgs) {
		t.Error("messages should be unchanged when not compacting")
	}
}

func TestAutoCompact_CompactNeeded(t *testing.T) {
	s := NewCompactService(100)
	a := NewAutoCompact(s, 200000)
	msgs := []Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
		{Role: "user", Content: "q3"},
	}
	_, compacted, err := a.CheckAndCompact(context.Background(), msgs, 500)
	if err != nil {
		t.Fatalf("CheckAndCompact returned error: %v", err)
	}
	if !compacted {
		t.Error("should compact when above threshold")
	}
}

func TestMicroCompact_KeepAll(t *testing.T) {
	s := NewCompactService(1000)
	m := NewMicroCompact(s)
	msgs := []Message{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "hello"},
	}
	result, err := m.CompactRecent(context.Background(), msgs, 5)
	if err != nil {
		t.Fatalf("CompactRecent returned error: %v", err)
	}
	if len(result) != len(msgs) {
		t.Error("should keep all messages when keepLast >= len(msgs)")
	}
}

func TestMicroCompact_KeepLast(t *testing.T) {
	s := NewCompactService(1000)
	m := NewMicroCompact(s)
	msgs := []Message{
		{Role: "user", Content: "q1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "q2"},
		{Role: "assistant", Content: "a2"},
		{Role: "user", Content: "q3"},
	}
	result, err := m.CompactRecent(context.Background(), msgs, 2)
	if err != nil {
		t.Fatalf("CompactRecent returned error: %v", err)
	}
	if len(result) > 4 {
		t.Errorf("result should have at most first + compacted marker + keepLast, got %d", len(result))
	}
	if result[0].Role != "user" {
		t.Errorf("first message should be preserved, got role %q", result[0].Role)
	}
}

func TestAnalyticsService_LogAndFlush(t *testing.T) {
	var flushed []Event
	sink := &mockSink{flushFn: func(events []Event) error {
		flushed = events
		return nil
	}}
	svc := NewAnalyticsService(sink)
	svc.LogEvent("test_event", map[string]any{"key": "value"})
	svc.LogEvent("another_event", nil)

	events := svc.GetEvents()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Name != "test_event" {
		t.Errorf("event name = %q, want %q", events[0].Name, "test_event")
	}
	if events[0].Metadata["key"] != "value" {
		t.Errorf("event metadata key = %v, want 'value'", events[0].Metadata["key"])
	}

	err := svc.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush returned error: %v", err)
	}
	if len(flushed) != 2 {
		t.Errorf("sink should receive 2 events, got %d", len(flushed))
	}
	remaining := svc.GetEvents()
	if len(remaining) != 0 {
		t.Error("events should be cleared after flush")
	}
}

func TestAnalyticsService_Disabled(t *testing.T) {
	sink := &mockSink{flushFn: func(events []Event) error { return nil }}
	svc := NewAnalyticsService(sink)
	svc.SetEnabled(false)
	svc.LogEvent("test", nil)
	events := svc.GetEvents()
	if len(events) != 0 {
		t.Error("disabled service should not log events")
	}
}

func TestAnalyticsService_Flush_NoSink(t *testing.T) {
	svc := NewAnalyticsService(nil)
	svc.LogEvent("test", nil)
	err := svc.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush with nil sink should not error, got: %v", err)
	}
	if len(svc.GetEvents()) != 0 {
		t.Error("events should be cleared even with nil sink")
	}
}

func TestAnalyticsService_Flush_Empty(t *testing.T) {
	sink := &mockSink{flushFn: func(events []Event) error { return nil }}
	svc := NewAnalyticsService(sink)
	err := svc.Flush(context.Background())
	if err != nil {
		t.Fatalf("Flush with no events should not error, got: %v", err)
	}
}

func TestAnalyticsService_ClearEvents(t *testing.T) {
	sink := &mockSink{flushFn: func(events []Event) error { return nil }}
	svc := NewAnalyticsService(sink)
	svc.LogEvent("test", nil)
	svc.ClearEvents()
	if len(svc.GetEvents()) != 0 {
		t.Error("ClearEvents should remove all events")
	}
}

func TestAnalyticsService_IsEnabled(t *testing.T) {
	sink := &mockSink{flushFn: func(events []Event) error { return nil }}
	svc := NewAnalyticsService(sink)
	if !svc.IsEnabled() {
		t.Error("should be enabled by default")
	}
	svc.SetEnabled(false)
	if svc.IsEnabled() {
		t.Error("should be disabled after SetEnabled(false)")
	}
}

func TestGrowthBookClient_DefaultValue(t *testing.T) {
	c := NewGrowthBookClient()
	val := c.GetFeatureValue("nonexistent", "default")
	if val != "default" {
		t.Errorf("expected default value, got %v", val)
	}
}

func TestGrowthBookClient_SetAndGet(t *testing.T) {
	c := NewGrowthBookClient()
	c.SetFeature("flag_x", true)
	val := c.GetFeatureValue("flag_x", false)
	if val != true {
		t.Errorf("expected true, got %v", val)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("DefaultConfig should have Enabled=true")
	}
	if cfg.OptOut {
		t.Error("DefaultConfig should have OptOut=false")
	}
	if cfg.BatchSize != 100 {
		t.Errorf("DefaultConfig BatchSize = %d, want 100", cfg.BatchSize)
	}
}

type mockSink struct {
	flushFn func(events []Event) error
}

func (m *mockSink) Flush(events []Event) error {
	return m.flushFn(events)
}
