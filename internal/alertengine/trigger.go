package alertengine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// WarRoomTriggerConfig controls the auto-trigger behavior for War Room sessions.
type WarRoomTriggerConfig struct {
	Enabled       bool          `json:"enabled"`
	MinSeverity   string        `json:"min_severity"`
	Cooldown      time.Duration `json:"cooldown"`
	MaxConcurrent int           `json:"max_concurrent"`
}

// DefaultWarRoomTriggerConfig returns sensible defaults.
func DefaultWarRoomTriggerConfig() WarRoomTriggerConfig {
	return WarRoomTriggerConfig{
		Enabled:       true,
		MinSeverity:   "high",
		Cooldown:      15 * time.Minute,
		MaxConcurrent: 5,
	}
}

// WarRoomTriggerResult is returned when a War Room is auto-triggered.
type WarRoomTriggerResult struct {
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
	Triggered bool   `json:"triggered"`
	Reason    string `json:"reason,omitempty"`
}

// WarRoomTriggerCoordinator is an interface for triggering War Room sessions.
// The warroom.WarRoomCoordinator implements this to avoid circular imports.
type WarRoomTriggerCoordinator interface {
	StartWarRoomFromAlert(ctx context.Context, source string, title string, description string, severity string, service string, labels map[string]string, annotations map[string]string) (*WarRoomTriggerResult, error)
	ActiveSessionCount() int
}

// WarRoomTrigger bridges AlertEngine callbacks to War Room session creation.
type WarRoomTrigger struct {
	config      WarRoomTriggerConfig
	coordinator WarRoomTriggerCoordinator

	lastTriggered map[string]time.Time
	mu            sync.Mutex
}

// NewWarRoomTrigger creates a new auto-trigger with the given config and coordinator.
func NewWarRoomTrigger(config WarRoomTriggerConfig, coordinator WarRoomTriggerCoordinator) *WarRoomTrigger {
	return &WarRoomTrigger{
		config:        config,
		coordinator:   coordinator,
		lastTriggered: make(map[string]time.Time),
	}
}

// OnAlert is the callback registered on AlertEngine via engine.OnAlert().
func (t *WarRoomTrigger) OnAlert(ctx context.Context, alert Alert) {
	if !t.config.Enabled {
		return
	}

	if t.coordinator == nil {
		return
	}

	if !t.shouldTrigger(alert) {
		return
	}

	title := fmt.Sprintf("[Auto] Alert: %s", alert.Name)
	description := buildAlertDescription(alert)

	result, err := t.coordinator.StartWarRoomFromAlert(
		ctx,
		alert.Source,
		title,
		description,
		alert.Severity,
		alert.Service,
		alert.Labels,
		alert.Annotations,
	)
	if err != nil {
		slog.Error("alertengine: failed to auto-trigger War Room", "error", err, "alert", alert.Name)
		return
	}

	if result != nil && result.Triggered {
		t.mu.Lock()
		t.lastTriggered[alert.Service] = time.Now()
		t.mu.Unlock()

		slog.Info("alertengine: auto-triggered War Room",
			"session_id", result.SessionID,
			"title", result.Title,
			"alert", alert.Name,
			"severity", alert.Severity,
			"service", alert.Service,
		)
	}
}

// shouldTrigger checks if an alert should trigger a War Room session.
func (t *WarRoomTrigger) shouldTrigger(alert Alert) bool {
	// Check severity.
	if SeverityLevel(alert.Severity) < SeverityLevel(t.config.MinSeverity) {
		return false
	}

	// Check max concurrent.
	if t.coordinator.ActiveSessionCount() >= t.config.MaxConcurrent {
		return false
	}

	// Check cooldown per service.
	t.mu.Lock()
	lastTime, exists := t.lastTriggered[alert.Service]
	t.mu.Unlock()

	if exists && time.Since(lastTime) < t.config.Cooldown {
		return false
	}

	return true
}

// UpdateConfig updates the trigger configuration.
func (t *WarRoomTrigger) UpdateConfig(config WarRoomTriggerConfig) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.config = config
}

// GetConfig returns the current trigger configuration.
func (t *WarRoomTrigger) GetConfig() WarRoomTriggerConfig {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.config
}

// buildAlertDescription creates a human-readable description from an alert.
func buildAlertDescription(alert Alert) string {
	desc := fmt.Sprintf("Severity: %s | Service: %s | Source: %s",
		alert.Severity, alert.Service, alert.Source)

	if alert.Annotations != nil {
		if d, ok := alert.Annotations["description"]; ok && d != "" {
			desc += "\n\n" + d
		}
	}

	return desc
}
