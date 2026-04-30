package watchdog

import (
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/alertengine"
)

func TestNewWatchdog(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	if w == nil {
		t.Fatal("expected non-nil watchdog")
	}
	if w.IsEnabled() {
		t.Error("new watchdog should not be enabled")
	}
	if len(w.patterns) == 0 {
		t.Error("expected default patterns")
	}
}

func TestStartStop(t *testing.T) {
	w := NewWatchdog(nil)

	w.Start()
	if !w.IsEnabled() {
		t.Error("expected enabled after Start")
	}

	w.Stop()
	if w.IsEnabled() {
		t.Error("expected disabled after Stop")
	}
}

func TestOnOutputLine(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	w.Start()

	w.OnOutputLine("main.go:42: undefined: foo", "go_build")
	w.OnOutputLine("--- FAIL: TestSomething", "go_test")
	w.OnOutputLine("util.go:10:5: unused variable", "go_lint")
	w.OnOutputLine("panic: runtime error: nil pointer", "runtime")
	w.OnOutputLine("this is not an error", "other")

	status := w.GetStatus()
	if status.ErrorCountToday != 4 {
		t.Errorf("expected 4 errors today, got %d", status.ErrorCountToday)
	}
	if len(status.RecentErrors) != 4 {
		t.Errorf("expected 4 recent errors, got %d", len(status.RecentErrors))
	}
}

func TestOnOutputLineDisabled(t *testing.T) {
	w := NewWatchdog(nil)

	w.OnOutputLine("main.go:42: undefined: foo", "go_build")

	status := w.GetStatus()
	if status.ErrorCountToday != 0 {
		t.Errorf("expected 0 errors when disabled, got %d", status.ErrorCountToday)
	}
}

func TestOnOutputLineEmpty(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	w.Start()

	w.OnOutputLine("", "go_build")

	status := w.GetStatus()
	if status.ErrorCountToday != 0 {
		t.Errorf("expected 0 errors for empty line, got %d", status.ErrorCountToday)
	}
}

func TestGetRecentErrors(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	w.Start()

	for i := 0; i < 5; i++ {
		w.OnOutputLine("file.go:1: error"+string(rune('0'+i)), "go_build")
	}

	errors := w.GetRecentErrors(3)
	if len(errors) != 3 {
		t.Errorf("expected 3 errors, got %d", len(errors))
	}
}

func TestGetStatus(t *testing.T) {
	w := NewWatchdog(nil)
	w.Start()

	status := w.GetStatus()
	if !status.Enabled {
		t.Error("expected enabled")
	}
	if status.ActiveWatches == nil {
		t.Error("expected non-nil active watches slice")
	}
	if status.RecentErrors == nil {
		t.Error("expected non-nil recent errors slice")
	}
}

func TestErrorPatternGoBuild(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	w.Start()

	w.OnOutputLine("internal/foo/bar.go:123: syntax error: unexpected semicolon", "go_build")

	errors := w.GetRecentErrors(1)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].File != "internal/foo/bar.go" {
		t.Errorf("expected file 'internal/foo/bar.go', got %q", errors[0].File)
	}
	if errors[0].Message != "syntax error: unexpected semicolon" {
		t.Errorf("unexpected message: %q", errors[0].Message)
	}
	if errors[0].Source != "go_build" {
		t.Errorf("expected source 'go_build', got %q", errors[0].Source)
	}
	if errors[0].Severity != "high" {
		t.Errorf("expected severity 'high', got %q", errors[0].Severity)
	}
}

func TestErrorPatternGoTest(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	w.Start()

	w.OnOutputLine("--- FAIL: TestWatchdog", "go_test")

	errors := w.GetRecentErrors(1)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].Message != "test failed: TestWatchdog" {
		t.Errorf("unexpected message: %q", errors[0].Message)
	}
	if errors[0].Severity != "high" {
		t.Errorf("expected severity 'high', got %q", errors[0].Severity)
	}
}

func TestErrorPatternRuntime(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	w.Start()

	w.OnOutputLine("panic: nil pointer dereference", "runtime")

	errors := w.GetRecentErrors(1)
	if len(errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errors))
	}
	if errors[0].Severity != "critical" {
		t.Errorf("expected severity 'critical', got %q", errors[0].Severity)
	}
}

func TestCreateAlert(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	w.Start()

	w.OnOutputLine("main.go:10: undefined: x", "go_build")

	results := ae.Query("", "", time.Time{})
	if len(results) == 0 {
		t.Error("expected alert to be ingested into the alert engine")
	}
}

func TestRingBuffer(t *testing.T) {
	ae := alertengine.NewAlertEngine()
	w := NewWatchdog(ae)
	w.Start()

	for i := 0; i < 60; i++ {
		w.OnOutputLine("file.go:1: error", "go_build")
	}

	status := w.GetStatus()
	if len(status.RecentErrors) > maxRecentErrors {
		t.Errorf("expected at most %d recent errors, got %d", maxRecentErrors, len(status.RecentErrors))
	}
}

func TestDefaultPatterns(t *testing.T) {
	patterns := defaultPatterns()
	if len(patterns) != 4 {
		t.Errorf("expected 4 default patterns, got %d", len(patterns))
	}
	sources := map[string]bool{}
	for _, p := range patterns {
		sources[p.Source] = true
	}
	for _, expected := range []string{"go_build", "go_test", "go_lint", "runtime"} {
		if !sources[expected] {
			t.Errorf("missing pattern source: %s", expected)
		}
	}
}
