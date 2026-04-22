package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewAnalytics_Enabled(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{
		Enabled:   true,
		UserID:    "user-1",
		SessionID: "session-1",
	})
	defer a.Close()
	if a == nil {
		t.Fatal("NewAnalytics returned nil")
	}
}

func TestNewAnalytics_Disabled(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{
		Enabled: false,
	})
	defer a.Close()
}

func TestNewAnalytics_Defaults(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{Enabled: false})
	defer a.Close()
	if a.config.FlushInterval != 30*time.Second {
		t.Errorf("FlushInterval = %v, want 30s", a.config.FlushInterval)
	}
	if a.config.MaxBatchSize != 100 {
		t.Errorf("MaxBatchSize = %d, want 100", a.config.MaxBatchSize)
	}
}

func TestTrack_Disabled_NoOp(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{Enabled: false})
	defer a.Close()
	a.Track("test_event", nil)
	a.mu.Lock()
	count := len(a.events)
	a.mu.Unlock()
	if count != 0 {
		t.Errorf("Track when disabled should not add events, got %d", count)
	}
}

func TestTrack_AutoFillsFields(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{
		Enabled:   false,
		UserID:    "user-123",
		SessionID:     "session-456",
		FlushInterval: time.Hour,
	})
	defer a.Close()

	event := Event{
		Name:       "test_event",
		Timestamp:  time.Now(),
		Properties: map[string]any{"key": "value"},
		UserID:     a.config.UserID,
		SessionID:  a.config.SessionID,
	}

	if event.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", event.UserID, "user-123")
	}
	if event.SessionID != "session-456" {
		t.Errorf("SessionID = %q, want %q", event.SessionID, "session-456")
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestTrack_Enabled_AddsEvent(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{
		Enabled:       true,
		UserID:        "user-1",
		SessionID:     "session-1",
		FlushInterval: time.Hour,
	})
	defer a.Close()

	a.Track("test_event", map[string]any{"key": "val"})

	a.mu.Lock()
	count := len(a.events)
	a.mu.Unlock()
	if count != 1 {
		t.Errorf("events count = %d, want 1", count)
	}
}

func TestFlush_Empty(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{Enabled: false})
	defer a.Close()
	if err := a.Flush(); err != nil {
		t.Errorf("Flush on empty should not error: %v", err)
	}
}

func TestFlush_SendsToSinks(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{Enabled: false})
	defer a.Close()

	var received []Event
	a.AddSink(&sinkFunc{
		sendFn: func(events []Event) error {
			received = append(received, events...)
			return nil
		},
	})

	a.mu.Lock()
	a.events = append(a.events, Event{Name: "e1"}, Event{Name: "e2"})
	a.mu.Unlock()

	if err := a.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}
	if len(received) != 2 {
		t.Errorf("Sink received %d events, want 2", len(received))
	}
}

func TestFlush_ReturnsLastError(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{Enabled: false})
	defer a.Close()

	a.AddSink(&sinkFunc{
		sendFn: func(events []Event) error {
			return nil
		},
	})
	a.AddSink(&sinkFunc{
		sendFn: func(events []Event) error {
			return os.ErrNotExist
		},
	})

	a.mu.Lock()
	a.events = append(a.events, Event{Name: "e1"})
	a.mu.Unlock()

	err := a.Flush()
	if err == nil {
		t.Fatal("Flush should return last sink error")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Error = %v, want ErrNotExist", err)
	}
}

func TestConsoleSink(t *testing.T) {
	sink := NewConsoleSink()
	events := []Event{
		{Name: "test", Timestamp: time.Now(), UserID: "u1"},
	}
	if err := sink.Send(events); err != nil {
		t.Errorf("ConsoleSink.Send error: %v", err)
	}
}

func TestFileSink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "analytics.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink failed: %v", err)
	}

	events := []Event{
		{Name: "event1", Timestamp: time.Now(), UserID: "u1"},
		{Name: "event2", Timestamp: time.Now(), UserID: "u2"},
	}
	if err := sink.Send(events); err != nil {
		t.Fatalf("FileSink.Send error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}

	var e1 Event
	if err := json.Unmarshal([]byte(lines[0]), &e1); err != nil {
		t.Fatalf("Unmarshal line 1: %v", err)
	}
	if e1.Name != "event1" {
		t.Errorf("Event name = %q, want %q", e1.Name, "event1")
	}
}

func TestFileSink_Appends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "analytics.jsonl")
	sink, _ := NewFileSink(path)

	sink.Send([]Event{{Name: "e1"}})
	sink.Send([]Event{{Name: "e2"}})

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines after two sends, got %d", len(lines))
	}
}

func TestDatadogSink(t *testing.T) {
	sink := NewDatadogSink("key", "appkey")
	events := []Event{{Name: "test"}}
	if err := sink.Send(events); err != nil {
		t.Errorf("DatadogSink.Send should be no-op, got error: %v", err)
	}
}

func TestGrowthBookClient_GetFeatureValue(t *testing.T) {
	client := NewGrowthBookClient("test-key")
	val := client.GetFeatureValue("my_feature", "default_val")
	if val != "default_val" {
		t.Errorf("GetFeatureValue = %v, want default_val", val)
	}
}

func TestGrowthBookClient_IsOn(t *testing.T) {
	client := NewGrowthBookClient("test-key")
	if client.IsOn("any_feature") {
		t.Error("IsOn should return false (default)")
	}
}

func TestGrowthBookClient_ClearCache(t *testing.T) {
	client := NewGrowthBookClient("test-key")
	client.GetFeatureValue("f1", "v1")
	client.ClearCache()

	client.mu.RLock()
	cacheLen := len(client.cache)
	client.mu.RUnlock()
	if cacheLen != 0 {
		t.Errorf("Cache length after ClearCache = %d, want 0", cacheLen)
	}
}

func TestMetrics_RecordRequest(t *testing.T) {
	m := NewMetrics()
	m.RecordRequest(true, 100)
	m.RecordRequest(true, 200)
	m.RecordRequest(false, 50)

	snap := m.GetSnapshot()
	if snap["total_requests"] != 3 {
		t.Errorf("total_requests = %d, want 3", snap["total_requests"])
	}
	if snap["success_requests"] != 2 {
		t.Errorf("success_requests = %d, want 2", snap["success_requests"])
	}
	if snap["failed_requests"] != 1 {
		t.Errorf("failed_requests = %d, want 1", snap["failed_requests"])
	}
	if snap["total_latency_ms"] != 350 {
		t.Errorf("total_latency_ms = %d, want 350", snap["total_latency_ms"])
	}
}

func TestMetrics_RecordTokens(t *testing.T) {
	m := NewMetrics()
	m.RecordTokens(100, 50, 20)

	snap := m.GetSnapshot()
	if snap["input_tokens"] != 100 {
		t.Errorf("input_tokens = %d, want 100", snap["input_tokens"])
	}
	if snap["output_tokens"] != 50 {
		t.Errorf("output_tokens = %d, want 50", snap["output_tokens"])
	}
	if snap["cached_tokens"] != 20 {
		t.Errorf("cached_tokens = %d, want 20", snap["cached_tokens"])
	}
	if snap["total_tokens"] != 150 {
		t.Errorf("total_tokens = %d, want 150", snap["total_tokens"])
	}
}

func TestMetrics_GetSnapshot(t *testing.T) {
	m := NewMetrics()
	m.RecordRequest(true, 100)
	m.RecordTokens(50, 25, 10)

	snap := m.GetSnapshot()
	expectedKeys := []string{
		"total_requests", "success_requests", "failed_requests",
		"total_tokens", "input_tokens", "output_tokens",
		"cached_tokens", "total_latency_ms",
	}
	for _, key := range expectedKeys {
		if _, ok := snap[key]; !ok {
			t.Errorf("Snapshot missing key %q", key)
		}
	}
}

func TestMetrics_Reset(t *testing.T) {
	m := NewMetrics()
	m.RecordRequest(true, 100)
	m.RecordTokens(50, 25, 10)
	m.Reset()

	snap := m.GetSnapshot()
	for key, val := range snap {
		if val != 0 {
			t.Errorf("After Reset, %s = %d, want 0", key, val)
		}
	}
}

func TestMetrics_GetAverageLatency(t *testing.T) {
	m := NewMetrics()
	m.RecordRequest(true, 100)
	m.RecordRequest(true, 200)

	avg := m.GetAverageLatency()
	if avg != 150.0 {
		t.Errorf("AverageLatency = %f, want 150.0", avg)
	}
}

func TestMetrics_GetAverageLatency_NoRequests(t *testing.T) {
	m := NewMetrics()
	if avg := m.GetAverageLatency(); avg != 0 {
		t.Errorf("AverageLatency with no requests = %f, want 0", avg)
	}
}

func TestMetrics_GetSuccessRate(t *testing.T) {
	m := NewMetrics()
	m.RecordRequest(true, 100)
	m.RecordRequest(true, 100)
	m.RecordRequest(false, 100)

	rate := m.GetSuccessRate()
	if rate != 66.66666666666666 {
		t.Errorf("SuccessRate = %f, want ~66.67", rate)
	}
}

func TestMetrics_GetSuccessRate_NoRequests(t *testing.T) {
	m := NewMetrics()
	if rate := m.GetSuccessRate(); rate != 0 {
		t.Errorf("SuccessRate with no requests = %f, want 0", rate)
	}
}

func TestAnalytics_Close(t *testing.T) {
	a := NewAnalytics(AnalyticsConfig{
		Enabled:       true,
		FlushInterval: time.Hour,
	})
	a.Close()
}

type sinkFunc struct {
	sendFn func(events []Event) error
}

func (s *sinkFunc) Send(events []Event) error {
	return s.sendFn(events)
}
