package watchdog

import (
	"context"
	"fmt"
	"sync"

	"github.com/instructkr/smartclaw/internal/alertengine"
	"github.com/instructkr/smartclaw/internal/tools"
)

var (
	defaultWatchdogMu sync.RWMutex
	defaultWatchdog   *Watchdog
)

func SetDefaultWatchdog(w *Watchdog) {
	defaultWatchdogMu.Lock()
	defer defaultWatchdogMu.Unlock()
	defaultWatchdog = w
}

func DefaultWatchdog() *Watchdog {
	defaultWatchdogMu.RLock()
	defer defaultWatchdogMu.RUnlock()
	return defaultWatchdog
}

func InitDefaultWatchdog() *Watchdog {
	ae := alertengine.DefaultAlertEngine()
	w := NewWatchdog(ae)
	SetDefaultWatchdog(w)
	return w
}

type WatchdogStartTool struct{}

func (t *WatchdogStartTool) Name() string { return "watchdog_start" }

func (t *WatchdogStartTool) Description() string {
	return "Enable the error-driven watchdog. Starts monitoring process output for error patterns and auto-triggering the alert→remediation→verify pipeline."
}

func (t *WatchdogStartTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{},
	}
}

func (t *WatchdogStartTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	w := DefaultWatchdog()
	if w == nil {
		return nil, fmt.Errorf("watchdog not initialized; call InitDefaultWatchdog first")
	}
	w.Start()
	return map[string]any{
		"status":  "enabled",
		"message": "Watchdog started — monitoring for errors",
	}, nil
}

type WatchdogStopTool struct{}

func (t *WatchdogStopTool) Name() string { return "watchdog_stop" }

func (t *WatchdogStopTool) Description() string {
	return "Disable the error-driven watchdog. Stops monitoring process output."
}

func (t *WatchdogStopTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{},
	}
}

func (t *WatchdogStopTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	w := DefaultWatchdog()
	if w == nil {
		return nil, fmt.Errorf("watchdog not initialized")
	}
	w.Stop()
	return map[string]any{
		"status":  "disabled",
		"message": "Watchdog stopped",
	}, nil
}

type WatchdogStatusTool struct{}

func (t *WatchdogStatusTool) Name() string { return "watchdog_status" }

func (t *WatchdogStatusTool) Description() string {
	return "Get the current status of the error-driven watchdog: enabled state, active watches, and recent errors."
}

func (t *WatchdogStatusTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{},
	}
}

func (t *WatchdogStatusTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	w := DefaultWatchdog()
	if w == nil {
		return WatchdogStatus{
			Enabled:       false,
			ActiveWatches: []ProcessWatch{},
			RecentErrors:  []DetectedError{},
		}, nil
	}
	return w.GetStatus(), nil
}

func RegisterWatchdogTools() {
	tools.Register(&WatchdogStartTool{})
	tools.Register(&WatchdogStopTool{})
	tools.Register(&WatchdogStatusTool{})
}
