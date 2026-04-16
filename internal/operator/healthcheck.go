package operator

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/httpclient"
	"github.com/instructkr/smartclaw/internal/tools"
)

type TopologyProvider interface {
	GetNodeHealth(serviceID string) (string, error)
}

type AlertProvider interface {
	GetActiveAlertCount(service string) (int, error)
}

type HealthChecker struct {
	topology TopologyProvider
	alerts   AlertProvider
	httpDo   func(req *http.Request) (*http.Response, error)
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{}
}

func (h *HealthChecker) SetTopologyProvider(tp TopologyProvider) {
	h.topology = tp
}

func (h *HealthChecker) SetAlertProvider(ap AlertProvider) {
	h.alerts = ap
}

func (h *HealthChecker) SetHTTPDo(fn func(req *http.Request) (*http.Response, error)) {
	h.httpDo = fn
}

func (h *HealthChecker) ExecuteCheck(ctx context.Context, check HealthCheckDef) HealthCheckResult {
	result := HealthCheckResult{
		CheckID:   check.ID,
		CheckName: check.Name,
		Threshold: check.Threshold,
		Timestamp: time.Now(),
	}

	if check.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, check.Timeout)
		defer cancel()
	}

	start := time.Now()
	defer func() {
		result.Duration = time.Since(start)
	}()

	switch check.Type {
	case CheckHTTP:
		h.executeHTTPCheck(ctx, check, &result)
	case CheckTCP:
		h.executeTCPCheck(ctx, check, &result)
	case CheckSLO:
		h.executeSLOCheck(ctx, check, &result)
	case CheckAlert:
		h.executeAlertCheck(ctx, check, &result)
	case CheckTopology:
		h.executeTopologyCheck(ctx, check, &result)
	case CheckCustom:
		h.executeCustomCheck(ctx, check, &result)
	default:
		result.Status = CheckError
		result.Message = fmt.Sprintf("unknown check type: %s", check.Type)
	}

	return result
}

func (h *HealthChecker) executeHTTPCheck(ctx context.Context, check HealthCheckDef, result *HealthCheckResult) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, check.Target, nil)
	if err != nil {
		result.Status = CheckError
		result.Message = fmt.Sprintf("failed to create request: %v", err)
		return
	}

	do := h.httpDo
	if do == nil {
		timeout := 10 * time.Second
		if check.Timeout > 0 {
			timeout = check.Timeout
		}
		do = httpclient.NewClient(timeout).Do
	}

	resp, err := do(req)
	if err != nil {
		result.Status = CheckFail
		result.Message = fmt.Sprintf("request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	result.Value = float64(result.Duration.Milliseconds())

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		result.Status = CheckPass
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	} else {
		result.Status = CheckFail
		result.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
}

func (h *HealthChecker) executeTCPCheck(ctx context.Context, check HealthCheckDef, result *HealthCheckResult) {
	startDial := time.Now()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", check.Target)
	result.Value = float64(time.Since(startDial).Milliseconds())

	if err != nil {
		result.Status = CheckFail
		result.Message = fmt.Sprintf("TCP connection failed: %v", err)
		return
	}
	conn.Close()
	result.Status = CheckPass
	result.Message = "TCP connection succeeded"
}

func (h *HealthChecker) executeSLOCheck(_ context.Context, check HealthCheckDef, result *HealthCheckResult) {
	if h.alerts == nil {
		result.Status = CheckError
		result.Message = "alert provider not configured"
		return
	}

	burnRate, err := h.getSLOBurnRate(check.Target)
	if err != nil {
		result.Status = CheckError
		result.Message = fmt.Sprintf("failed to get SLO burn rate: %v", err)
		return
	}

	result.Value = burnRate

	if burnRate >= 3.0 {
		result.Status = CheckFail
		result.Message = fmt.Sprintf("SLO burn rate %.2f >= 3.0", burnRate)
	} else if burnRate >= 1.0 {
		result.Status = CheckWarn
		result.Message = fmt.Sprintf("SLO burn rate %.2f >= 1.0", burnRate)
	} else {
		result.Status = CheckPass
		result.Message = fmt.Sprintf("SLO burn rate %.2f < 1.0", burnRate)
	}
}

func (h *HealthChecker) executeAlertCheck(_ context.Context, check HealthCheckDef, result *HealthCheckResult) {
	if h.alerts == nil {
		result.Status = CheckError
		result.Message = "alert provider not configured"
		return
	}

	count, err := h.alerts.GetActiveAlertCount(check.Target)
	if err != nil {
		result.Status = CheckError
		result.Message = fmt.Sprintf("failed to get alert count: %v", err)
		return
	}

	result.Value = float64(count)

	if check.Threshold > 0 && float64(count) >= check.Threshold {
		result.Status = CheckFail
		result.Message = fmt.Sprintf("alert count %d >= threshold %.0f", count, check.Threshold)
	} else {
		result.Status = CheckPass
		result.Message = fmt.Sprintf("alert count %d < threshold %.0f", count, check.Threshold)
	}
}

func (h *HealthChecker) executeTopologyCheck(_ context.Context, check HealthCheckDef, result *HealthCheckResult) {
	if h.topology == nil {
		result.Status = CheckError
		result.Message = "topology provider not configured"
		return
	}

	health, err := h.topology.GetNodeHealth(check.Target)
	if err != nil {
		result.Status = CheckError
		result.Message = fmt.Sprintf("failed to get node health: %v", err)
		return
	}

	switch strings.ToLower(health) {
	case "healthy":
		result.Status = CheckPass
		result.Value = 1.0
		result.Message = "node healthy"
	case "degraded":
		result.Status = CheckWarn
		result.Value = 0.5
		result.Message = "node degraded"
	case "down":
		result.Status = CheckFail
		result.Value = 0.0
		result.Message = "node down"
	default:
		result.Status = CheckUnknown
		result.Value = -1.0
		result.Message = fmt.Sprintf("unknown health status: %s", health)
	}
}

func (h *HealthChecker) executeCustomCheck(ctx context.Context, check HealthCheckDef, result *HealthCheckResult) {
	if validationResult := tools.ValidateCommandSecurity(check.Target); !validationResult.Allowed {
		slog.Warn("healthcheck command rejected by security policy", "command", check.Target, "reason", validationResult.Reason)
		result.Status = CheckError
		result.Message = fmt.Sprintf("command rejected by security policy: %s", validationResult.Reason)
		return
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", check.Target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = CheckFail
			result.Message = "command timed out"
		} else {
			result.Status = CheckFail
			result.Message = fmt.Sprintf("command failed: %v: %s", err, string(output))
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Value = float64(exitErr.ExitCode())
		}
		return
	}

	result.Status = CheckPass
	result.Value = 0
	result.Message = strings.TrimSpace(string(output))
}

func (h *HealthChecker) getSLOBurnRate(service string) (float64, error) {
	if h.alerts == nil {
		return 0, fmt.Errorf("alert provider not configured")
	}

	count, err := h.alerts.GetActiveAlertCount(service)
	if err != nil {
		return 0, err
	}

	if count == 0 {
		return 0, nil
	}

	return float64(count) / 10.0, nil
}

func parseFloatSuffix(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty string")
	}
	return strconv.ParseFloat(s, 64)
}
