package watchdog

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/instructkr/smartclaw/internal/alertengine"
	"github.com/instructkr/smartclaw/internal/autoremediation"
)

// WatchdogOnAlert is a callback for the alert engine's OnAlert system.
// When an error alert from the watchdog fires, it can suggest remediation.
func WatchdogOnAlert(ctx context.Context, alert alertengine.Alert) {
	if alert.Source != "watchdog" {
		return
	}

	severity := alert.Severity
	if severity == "" {
		severity = "medium"
	}

	if severity != "high" && severity != "critical" {
		slog.Info("watchdog: alert observed, below remediation threshold",
			"severity", severity, "name", alert.Name)
		return
	}

	if alert.Labels["file"] != "" {
		file := alert.Labels["file"]
		cmd := fmt.Sprintf(`/tools dap_start {"program":"%s"}`, file)
		alert.Annotations["debug_suggestion_file"] = file
		alert.Annotations["debug_suggestion_command"] = cmd
		slog.Info("watchdog: debug suggestion added",
			"file", file, "command", cmd)
	}

	re := autoremediation.DefaultRemediationEngine()
	if re == nil {
		slog.Warn("watchdog: remediation engine not available for auto-remediation")
		return
	}

	service := alert.Service
	if service == "" {
		service = "unknown"
	}

	runbooks, err := re.SuggestRemediation(service, "slo_burn")
	if err != nil {
		slog.Warn("watchdog: remediation suggestion failed", "error", err)
		return
	}

	if len(runbooks) == 0 {
		slog.Info("watchdog: no remediation runbooks found", "service", service)
		return
	}

	slog.Info("watchdog: remediation suggested",
		"service", service,
		"runbooks", fmt.Sprintf("%d available", len(runbooks)),
		"alert", alert.Name)
}
