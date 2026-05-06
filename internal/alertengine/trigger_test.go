package alertengine

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockCoordinator is a mock implementation of WarRoomTriggerCoordinator.
type mockCoordinator struct {
	mu            sync.Mutex
	sessionCount  int
	createCount   int
	lastTitle     string
	lastService   string
	lastSeverity  string
	failOnCreate  bool
}

func (m *mockCoordinator) StartWarRoomFromAlert(ctx context.Context, source string, title string, description string, severity string, service string, labels map[string]string, annotations map[string]string) (*WarRoomTriggerResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failOnCreate {
		return nil, context.DeadlineExceeded
	}
	m.createCount++
	m.lastTitle = title
	m.lastService = service
	m.lastSeverity = severity
	return &WarRoomTriggerResult{
		SessionID: "wr-auto-001",
		Title:     title,
		Triggered: true,
	}, nil
}

func (m *mockCoordinator) ActiveSessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessionCount
}

func (m *mockCoordinator) setSessionCount(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionCount = n
}

func (m *mockCoordinator) getCreateCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createCount
}

func TestWarRoomTrigger_ShouldTrigger(t *testing.T) {
	coord := &mockCoordinator{sessionCount: 0}
	config := DefaultWarRoomTriggerConfig()
	trigger := NewWarRoomTrigger(config, coord)

	alert := Alert{
		Source:   "sopa",
		Name:     "GPU OOM",
		Severity: "critical",
		Service:  "gpu-cluster-01",
		Labels:   map[string]string{"env": "prod"},
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert)

	if count := coord.getCreateCount(); count != 1 {
		t.Errorf("expected 1 War Room creation, got %d", count)
	}
	if coord.lastSeverity != "critical" {
		t.Errorf("expected severity 'critical', got %q", coord.lastSeverity)
	}
	if coord.lastService != "gpu-cluster-01" {
		t.Errorf("expected service 'gpu-cluster-01', got %q", coord.lastService)
	}
}

func TestWarRoomTrigger_LowSeverity(t *testing.T) {
	coord := &mockCoordinator{}
	config := DefaultWarRoomTriggerConfig()
	trigger := NewWarRoomTrigger(config, coord)

	alert := Alert{
		Source:   "sopa",
		Name:     "Info Alert",
		Severity: "info",
		Service:  "web",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert)

	if count := coord.getCreateCount(); count != 0 {
		t.Errorf("expected 0 War Room creations for low severity, got %d", count)
	}
}

func TestWarRoomTrigger_Disabled(t *testing.T) {
	coord := &mockCoordinator{}
	config := DefaultWarRoomTriggerConfig()
	config.Enabled = false
	trigger := NewWarRoomTrigger(config, coord)

	alert := Alert{
		Source:   "sopa",
		Name:     "Critical Alert",
		Severity: "critical",
		Service:  "db",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert)

	if count := coord.getCreateCount(); count != 0 {
		t.Errorf("expected 0 War Room creations when disabled, got %d", count)
	}
}

func TestWarRoomTrigger_MaxConcurrent(t *testing.T) {
	coord := &mockCoordinator{sessionCount: 5}
	config := DefaultWarRoomTriggerConfig()
	config.MaxConcurrent = 5
	trigger := NewWarRoomTrigger(config, coord)

	alert := Alert{
		Source:   "sopa",
		Name:     "Critical Alert",
		Severity: "critical",
		Service:  "api",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert)

	if count := coord.getCreateCount(); count != 0 {
		t.Errorf("expected 0 War Room creations when max concurrent reached, got %d", count)
	}
}

func TestWarRoomTrigger_CooldownByService(t *testing.T) {
	coord := &mockCoordinator{sessionCount: 0}
	config := DefaultWarRoomTriggerConfig()
	config.Cooldown = 10 * time.Minute
	trigger := NewWarRoomTrigger(config, coord)

	alert1 := Alert{
		Source:   "sopa",
		Name:     "GPU OOM",
		Severity: "critical",
		Service:  "gpu-cluster-01",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert1)
	if count := coord.getCreateCount(); count != 1 {
		t.Fatalf("expected 1 War Room creation after first alert, got %d", count)
	}

	alert2 := Alert{
		Source:   "sopa",
		Name:     "GPU OOM Again",
		Severity: "critical",
		Service:  "gpu-cluster-01",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert2)
	if count := coord.getCreateCount(); count != 1 {
		t.Errorf("expected 1 War Room creation (cooldown), got %d", count)
	}
}

func TestWarRoomTrigger_DifferentServicesNoCooldown(t *testing.T) {
	coord := &mockCoordinator{sessionCount: 0}
	config := DefaultWarRoomTriggerConfig()
	config.Cooldown = 10 * time.Minute
	trigger := NewWarRoomTrigger(config, coord)

	alert1 := Alert{
		Source:   "sopa",
		Name:     "GPU OOM",
		Severity: "critical",
		Service:  "gpu-cluster-01",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert1)

	alert2 := Alert{
		Source:   "sopa",
		Name:     "DB Slow",
		Severity: "high",
		Service:  "mysql-primary",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert2)

	if count := coord.getCreateCount(); count != 2 {
		t.Errorf("expected 2 War Room creations (different services), got %d", count)
	}
}

func TestWarRoomTrigger_CooldownExpires(t *testing.T) {
	coord := &mockCoordinator{sessionCount: 0}
	config := DefaultWarRoomTriggerConfig()
	config.Cooldown = 1 * time.Millisecond
	trigger := NewWarRoomTrigger(config, coord)

	alert := Alert{
		Source:   "sopa",
		Name:     "GPU OOM",
		Severity: "critical",
		Service:  "gpu-cluster-01",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert)
	if count := coord.getCreateCount(); count != 1 {
		t.Fatalf("expected 1 War Room creation, got %d", count)
	}

	time.Sleep(5 * time.Millisecond)

	trigger.OnAlert(context.Background(), alert)
	if count := coord.getCreateCount(); count != 2 {
		t.Errorf("expected 2 War Room creations after cooldown expires, got %d", count)
	}
}

func TestWarRoomTrigger_UpdateConfig(t *testing.T) {
	coord := &mockCoordinator{sessionCount: 0}
	config := DefaultWarRoomTriggerConfig()
	trigger := NewWarRoomTrigger(config, coord)

	newConfig := WarRoomTriggerConfig{
		Enabled:       false,
		MinSeverity:   "critical",
		Cooldown:      5 * time.Minute,
		MaxConcurrent: 3,
	}
	trigger.UpdateConfig(newConfig)

	got := trigger.GetConfig()
	if got.Enabled != false {
		t.Error("expected enabled to be false")
	}
	if got.MinSeverity != "critical" {
		t.Errorf("expected min_severity 'critical', got %q", got.MinSeverity)
	}
	if got.Cooldown != 5*time.Minute {
		t.Errorf("expected cooldown 5m, got %v", got.Cooldown)
	}
	if got.MaxConcurrent != 3 {
		t.Errorf("expected max_concurrent 3, got %d", got.MaxConcurrent)
	}
}

func TestWarRoomTrigger_NilCoordinator(t *testing.T) {
	config := DefaultWarRoomTriggerConfig()
	trigger := NewWarRoomTrigger(config, nil)

	alert := Alert{
		Source:   "sopa",
		Name:     "Critical Alert",
		Severity: "critical",
		Service:  "api",
		FiredAt:  time.Now(),
	}

	trigger.OnAlert(context.Background(), alert)
}

func TestBuildAlertDescription(t *testing.T) {
	alert := Alert{
		Source:      "sopa",
		Name:        "GPU OOM",
		Severity:    "critical",
		Service:     "gpu-cluster-01",
		Annotations: map[string]string{"description": "Memory exceeded 95%"},
	}

	desc := buildAlertDescription(alert)

	if desc == "" {
		t.Error("expected non-empty description")
	}
	if !contains(desc, "critical") {
		t.Error("expected description to contain severity")
	}
	if !contains(desc, "gpu-cluster-01") {
		t.Error("expected description to contain service")
	}
	if !contains(desc, "Memory exceeded 95%") {
		t.Error("expected description to contain annotation")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
