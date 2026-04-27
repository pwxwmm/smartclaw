package operator

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockTopologyProvider struct {
	health string
	err    error
}

func (m *mockTopologyProvider) GetNodeHealth(_ string) (string, error) {
	return m.health, m.err
}

type mockAlertProvider struct {
	count int
	err   error
}

func (m *mockAlertProvider) GetActiveAlertCount(_ string) (int, error) {
	return m.count, m.err
}

type mockCronScheduler struct {
	mu          sync.Mutex
	scheduled   map[string]string
	unscheduled []string
	callbacks   map[string]func()
}

func newMockCronScheduler() *mockCronScheduler {
	return &mockCronScheduler{
		scheduled: make(map[string]string),
		callbacks: make(map[string]func()),
	}
}

func (m *mockCronScheduler) ScheduleCron(id string, schedule string, fn func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scheduled[id] = schedule
	m.callbacks[id] = fn
	return nil
}

func (m *mockCronScheduler) UnscheduleCron(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.scheduled, id)
	delete(m.callbacks, id)
	m.unscheduled = append(m.unscheduled, id)
	return nil
}

func (m *mockCronScheduler) getScheduled(id string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.scheduled[id]
	return s, ok
}

func (m *mockCronScheduler) runCallback(id string) {
	m.mu.Lock()
	fn, ok := m.callbacks[id]
	m.mu.Unlock()
	if ok && fn != nil {
		fn()
	}
}

func newTestManager() *OperatorManager {
	m := NewOperatorManager()
	m.SetHealthChecker(NewHealthChecker())
	return m
}

func TestOperatorEnable(t *testing.T) {
	m := newTestManager()

	config := OperatorConfig{
		Name:          "test-operator",
		Schedule:      "*/5 * * * *",
		AutonomyLevel: AutonomySuggest,
		HealthChecks: []HealthCheckDef{
			{ID: "hc1", Name: "HTTP Check", Type: CheckHTTP, Target: "http://localhost:8080/health"},
		},
	}

	result, err := m.Enable(context.Background(), config)
	if err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	if result.ID == "" {
		t.Fatal("expected non-empty config ID")
	}
	if !result.Enabled {
		t.Fatal("expected config to be enabled")
	}
	if result.Name != "test-operator" {
		t.Fatalf("expected name 'test-operator', got %q", result.Name)
	}
	if result.AutonomyLevel != AutonomySuggest {
		t.Fatalf("expected autonomy 'suggest', got %q", result.AutonomyLevel)
	}
	if result.MaxAutoActions != 3 {
		t.Fatalf("expected max_auto_actions 3, got %d", result.MaxAutoActions)
	}

	status := m.GetStatus(result.ID)
	if status == nil {
		t.Fatal("expected status to exist")
	}
	if !status.Enabled {
		t.Fatal("expected status to be enabled")
	}

	operators := m.ListOperators()
	if len(operators) != 1 {
		t.Fatalf("expected 1 operator, got %d", len(operators))
	}
}

func TestOperatorEnableDefaults(t *testing.T) {
	m := newTestManager()

	config := OperatorConfig{
		Name: "minimal-operator",
	}

	result, err := m.Enable(context.Background(), config)
	if err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	if result.Schedule != "*/5 * * * *" {
		t.Fatalf("expected default schedule, got %q", result.Schedule)
	}
	if result.AutonomyLevel != AutonomySuggest {
		t.Fatalf("expected default autonomy 'suggest', got %q", result.AutonomyLevel)
	}
	if result.MaxAutoActions != 3 {
		t.Fatalf("expected default max_auto_actions 3, got %d", result.MaxAutoActions)
	}
	if len(result.EscalationPolicy.Levels) != 2 {
		t.Fatalf("expected 2 default escalation levels, got %d", len(result.EscalationPolicy.Levels))
	}
}

func TestOperatorDisable(t *testing.T) {
	m := newTestManager()

	config := OperatorConfig{
		Name: "test-operator",
	}
	result, err := m.Enable(context.Background(), config)
	if err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	err = m.Disable(result.ID)
	if err != nil {
		t.Fatalf("Disable failed: %v", err)
	}

	status := m.GetStatus(result.ID)
	if status.Enabled {
		t.Fatal("expected status to be disabled")
	}

	operators := m.ListOperators()
	if operators[0].Enabled {
		t.Fatal("expected config to be disabled")
	}
}

func TestOperatorDisableNotFound(t *testing.T) {
	m := newTestManager()

	err := m.Disable("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestOperatorDisableAll(t *testing.T) {
	m := newTestManager()

	m.Enable(context.Background(), OperatorConfig{Name: "op1"})
	m.Enable(context.Background(), OperatorConfig{Name: "op2"})

	disabled := m.DisableAll()
	if len(disabled) != 2 {
		t.Fatalf("expected 2 disabled, got %d", len(disabled))
	}

	for _, id := range disabled {
		status := m.GetStatus(id)
		if status.Enabled {
			t.Fatal("expected all operators to be disabled")
		}
	}
}

func TestRunCheckCycle(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 0})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test-operator",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "alert1", Name: "Alert Check", Type: CheckAlert, Target: "my-service", Threshold: 5},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)

	status, err := m.RunCheckCycle(context.Background(), result.ID)
	if err != nil {
		t.Fatalf("RunCheckCycle failed: %v", err)
	}

	if status.TotalChecks != 1 {
		t.Fatalf("expected 1 total check, got %d", status.TotalChecks)
	}
	if status.PassingChecks != 1 {
		t.Fatalf("expected 1 passing check, got %d", status.PassingChecks)
	}
	if status.LastCheckCycle == nil {
		t.Fatal("expected last_check_cycle to be set")
	}
}

func TestRunCheckCycleFailingChecks(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 10})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test-operator",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "alert1", Name: "Alert Check", Type: CheckAlert, Target: "my-service", Threshold: 5},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)

	status, err := m.RunCheckCycle(context.Background(), result.ID)
	if err != nil {
		t.Fatalf("RunCheckCycle failed: %v", err)
	}

	if status.FailingChecks != 1 {
		t.Fatalf("expected 1 failing check, got %d", status.FailingChecks)
	}
	if status.Uptime != 0.0 {
		t.Fatalf("expected uptime 0.0, got %.2f", status.Uptime)
	}
}

func TestHealthCheckHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:      "http1",
		Name:    "HTTP Check",
		Type:    CheckHTTP,
		Target:  server.URL,
		Timeout: 5 * time.Second,
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckPass {
		t.Fatalf("expected pass, got %s: %s", result.Status, result.Message)
	}
}

func TestHealthCheckHTTPFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:      "http1",
		Name:    "HTTP Check",
		Type:    CheckHTTP,
		Target:  server.URL,
		Timeout: 5 * time.Second,
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckFail {
		t.Fatalf("expected fail, got %s: %s", result.Status, result.Message)
	}
}

func TestHealthCheckHTTPWithInjectedClient(t *testing.T) {
	hc := NewHealthChecker()
	hc.SetHTTPDo(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	})

	check := HealthCheckDef{
		ID:     "http1",
		Name:   "HTTP Check",
		Type:   CheckHTTP,
		Target: "http://example.com",
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckPass {
		t.Fatalf("expected pass, got %s", result.Status)
	}
}

func TestHealthCheckTCP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:      "tcp1",
		Name:    "TCP Check",
		Type:    CheckTCP,
		Target:  listener.Addr().String(),
		Timeout: 5 * time.Second,
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckPass {
		t.Fatalf("expected pass, got %s: %s", result.Status, result.Message)
	}
}

func TestHealthCheckTCPFail(t *testing.T) {
	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:      "tcp1",
		Name:    "TCP Check",
		Type:    CheckTCP,
		Target:  "127.0.0.1:1",
		Timeout: 1 * time.Second,
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckFail {
		t.Fatalf("expected fail, got %s: %s", result.Status, result.Message)
	}
}

func TestHealthCheckSLO(t *testing.T) {
	tests := []struct {
		name       string
		burnRate   float64
		wantStatus CheckStatus
	}{
		{"pass", 0.5, CheckPass},
		{"warn", 1.5, CheckWarn},
		{"fail", 3.5, CheckFail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewHealthChecker()
			hc.SetAlertProvider(&mockAlertProvider{count: int(tt.burnRate * 10)})

			check := HealthCheckDef{
				ID:     "slo1",
				Name:   "SLO Check",
				Type:   CheckSLO,
				Target: "my-service",
			}

			result := hc.ExecuteCheck(context.Background(), check)
			if result.Status != tt.wantStatus {
				t.Fatalf("expected %s, got %s (burn_rate=%.2f)", tt.wantStatus, result.Status, result.Value)
			}
		})
	}
}

func TestHealthCheckSLONoProvider(t *testing.T) {
	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:     "slo1",
		Name:   "SLO Check",
		Type:   CheckSLO,
		Target: "my-service",
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckError {
		t.Fatalf("expected error, got %s", result.Status)
	}
}

func TestHealthCheckAlert(t *testing.T) {
	tests := []struct {
		name      string
		count     int
		threshold float64
		want      CheckStatus
	}{
		{"pass", 2, 5, CheckPass},
		{"fail", 10, 5, CheckFail},
		{"zero_threshold", 1, 0, CheckPass},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewHealthChecker()
			hc.SetAlertProvider(&mockAlertProvider{count: tt.count})

			check := HealthCheckDef{
				ID:        "alert1",
				Name:      "Alert Check",
				Type:      CheckAlert,
				Target:    "my-service",
				Threshold: tt.threshold,
			}

			result := hc.ExecuteCheck(context.Background(), check)
			if result.Status != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, result.Status)
			}
		})
	}
}

func TestHealthCheckAlertNoProvider(t *testing.T) {
	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:     "alert1",
		Name:   "Alert Check",
		Type:   CheckAlert,
		Target: "my-service",
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckError {
		t.Fatalf("expected error, got %s", result.Status)
	}
}

func TestHealthCheckTopology(t *testing.T) {
	tests := []struct {
		name   string
		health string
		want   CheckStatus
		value  float64
	}{
		{"healthy", "healthy", CheckPass, 1.0},
		{"degraded", "degraded", CheckWarn, 0.5},
		{"down", "down", CheckFail, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := NewHealthChecker()
			hc.SetTopologyProvider(&mockTopologyProvider{health: tt.health})

			check := HealthCheckDef{
				ID:     "topo1",
				Name:   "Topology Check",
				Type:   CheckTopology,
				Target: "my-service",
			}

			result := hc.ExecuteCheck(context.Background(), check)
			if result.Status != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, result.Status)
			}
			if result.Value != tt.value {
				t.Fatalf("expected value %.1f, got %.1f", tt.value, result.Value)
			}
		})
	}
}

func TestHealthCheckTopologyNoProvider(t *testing.T) {
	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:     "topo1",
		Name:   "Topology Check",
		Type:   CheckTopology,
		Target: "my-service",
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckError {
		t.Fatalf("expected error, got %s", result.Status)
	}
}

func TestHealthCheckCustom(t *testing.T) {
	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:      "custom1",
		Name:    "Custom Check",
		Type:    CheckCustom,
		Target:  "echo hello",
		Timeout: 5 * time.Second,
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckPass {
		t.Fatalf("expected pass, got %s: %s", result.Status, result.Message)
	}
	if result.Value != 0 {
		t.Fatalf("expected exit code 0, got %.0f", result.Value)
	}
}

func TestHealthCheckCustomFail(t *testing.T) {
	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:      "custom1",
		Name:    "Custom Check",
		Type:    CheckCustom,
		Target:  "exit 1",
		Timeout: 5 * time.Second,
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckFail {
		t.Fatalf("expected fail, got %s: %s", result.Status, result.Message)
	}
}

func TestHealthCheckUnknownType(t *testing.T) {
	hc := NewHealthChecker()
	check := HealthCheckDef{
		ID:     "unknown1",
		Name:   "Unknown Check",
		Type:   CheckType("unknown"),
		Target: "whatever",
	}

	result := hc.ExecuteCheck(context.Background(), check)
	if result.Status != CheckError {
		t.Fatalf("expected error, got %s", result.Status)
	}
}

func TestHandleCheckResult(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{Name: "test"}
	result, _ := m.Enable(context.Background(), config)

	checkResult := HealthCheckResult{
		CheckID:   "hc1",
		CheckName: "Test Check",
		Status:    CheckFail,
		Message:   "something broke",
		Timestamp: time.Now(),
	}

	err := m.HandleCheckResult(result.ID, checkResult)
	if err != nil {
		t.Fatalf("HandleCheckResult failed: %v", err)
	}

	results := m.GetCheckResults(result.ID)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != CheckFail {
		t.Fatalf("expected fail status, got %s", results[0].Status)
	}

	events := m.GetEvents(result.ID)
	hasCheckResultEvent := false
	for _, e := range events {
		if e.Type == "check_result" {
			hasCheckResultEvent = true
		}
	}
	if !hasCheckResultEvent {
		t.Fatal("expected check_result event")
	}
}

func TestHandleCheckResultMaxResults(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{Name: "test"}
	result, _ := m.Enable(context.Background(), config)

	for i := 0; i < 150; i++ {
		m.HandleCheckResult(result.ID, HealthCheckResult{
			CheckID:   fmt.Sprintf("hc%d", i),
			Status:    CheckPass,
			Timestamp: time.Now(),
		})
	}

	results := m.GetCheckResults(result.ID)
	if len(results) > GetConfig().MaxRecentResults {
		t.Fatalf("expected at most %d results, got %d", GetConfig().MaxRecentResults, len(results))
	}
}

func TestEscalationCheckFail(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 10})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "alert1", Name: "Alert Check", Type: CheckAlert, Target: "svc", Threshold: 5},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}},
				{Level: 2, Trigger: "slo_burn>3", Actions: []string{"notify", "create_incident"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)
	m.RunCheckCycle(context.Background(), result.ID)

	status := m.GetStatus(result.ID)
	if status.ActiveEscalations < 1 {
		t.Fatalf("expected at least 1 active escalation, got %d", status.ActiveEscalations)
	}
}

func TestEscalationSLOBurn(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 50})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "slo1", Name: "SLO Check", Type: CheckSLO, Target: "svc"},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "slo_burn>3", Actions: []string{"notify"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)
	m.RunCheckCycle(context.Background(), result.ID)

	status := m.GetStatus(result.ID)
	if status.ActiveEscalations < 1 {
		t.Fatalf("expected at least 1 active escalation, got %d", status.ActiveEscalations)
	}
}

func TestEscalationAlertCount(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 10})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "alert1", Name: "Alert Check", Type: CheckAlert, Target: "svc", Threshold: 20},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "alert_count>5", Actions: []string{"notify"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)
	m.RunCheckCycle(context.Background(), result.ID)

	status := m.GetStatus(result.ID)
	if status.ActiveEscalations < 1 {
		t.Fatalf("expected at least 1 active escalation, got %d", status.ActiveEscalations)
	}
}

func TestEscalationNoDoubleTrigger(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 10})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "alert1", Name: "Alert Check", Type: CheckAlert, Target: "svc", Threshold: 5},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)

	m.RunCheckCycle(context.Background(), result.ID)
	esc1, _ := m.EvaluateEscalation(result.ID)
	if esc1 != nil {
		t.Fatal("expected no re-escalation at same level")
	}
}

func TestActionObserve(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyObserve,
	}
	result, _ := m.Enable(context.Background(), config)

	out, err := m.ExecuteAction(context.Background(), result.ID, "notify")
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if !strings.Contains(out, "would execute") {
		t.Fatalf("expected 'would execute' message, got %q", out)
	}
}

func TestActionSuggest(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomySuggest,
	}
	result, _ := m.Enable(context.Background(), config)

	out, err := m.ExecuteAction(context.Background(), result.ID, "auto_remediate")
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if !strings.Contains(out, "suggested action") {
		t.Fatalf("expected 'suggested action' message, got %q", out)
	}
}

func TestActionAutoPreApproved(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyAuto,
	}
	result, _ := m.Enable(context.Background(), config)

	out, err := m.ExecuteAction(context.Background(), result.ID, "notify")
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if !strings.Contains(out, "notification sent") {
		t.Fatalf("expected 'notification sent' for pre-approved action, got %q", out)
	}
}

func TestActionAutoNotPreApproved(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyAuto,
	}
	result, _ := m.Enable(context.Background(), config)

	out, err := m.ExecuteAction(context.Background(), result.ID, "auto_remediate")
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if !strings.Contains(out, "requires approval") {
		t.Fatalf("expected 'requires approval' message, got %q", out)
	}
}

func TestActionFull(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyFull,
	}
	result, _ := m.Enable(context.Background(), config)

	out, err := m.ExecuteAction(context.Background(), result.ID, "restart_service")
	if err != nil {
		t.Fatalf("ExecuteAction failed: %v", err)
	}
	if !strings.Contains(out, "executed") {
		t.Fatalf("expected 'executed' message, got %q", out)
	}
}

func TestOperatorStatusTracking(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 0})
	hc.SetTopologyProvider(&mockTopologyProvider{health: "healthy"})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "alert1", Name: "Alert Check", Type: CheckAlert, Target: "svc", Threshold: 5},
			{ID: "topo1", Name: "Topology Check", Type: CheckTopology, Target: "svc"},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)
	status, _ := m.RunCheckCycle(context.Background(), result.ID)

	if status.TotalChecks != 2 {
		t.Fatalf("expected 2 total checks, got %d", status.TotalChecks)
	}
	if status.PassingChecks != 2 {
		t.Fatalf("expected 2 passing checks, got %d", status.PassingChecks)
	}
	if status.FailingChecks != 0 {
		t.Fatalf("expected 0 failing checks, got %d", status.FailingChecks)
	}
	if status.Uptime != 100.0 {
		t.Fatalf("expected 100%% uptime, got %.2f%%", status.Uptime)
	}
}

func TestOperatorStatusUptime(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 10})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "alert1", Name: "Alert Check", Type: CheckAlert, Target: "svc", Threshold: 5},
			{ID: "alert2", Name: "Alert Check 2", Type: CheckAlert, Target: "svc2", Threshold: 100},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)
	status, _ := m.RunCheckCycle(context.Background(), result.ID)

	if status.PassingChecks != 1 {
		t.Fatalf("expected 1 passing, got %d", status.PassingChecks)
	}
	if status.FailingChecks != 1 {
		t.Fatalf("expected 1 failing, got %d", status.FailingChecks)
	}
	if status.Uptime != 50.0 {
		t.Fatalf("expected 50%% uptime, got %.2f%%", status.Uptime)
	}
}

func TestEventRecording(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{Name: "test"}
	result, _ := m.Enable(context.Background(), config)

	m.AddEvent(result.ID, "test_event", "test details", "info")

	events := m.GetEvents(result.ID)
	found := false
	for _, e := range events {
		if e.Type == "test_event" && e.Details == "test details" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected test_event to be recorded")
	}
}

func TestEventRecordingMax(t *testing.T) {
	m := newTestManager()
	config := OperatorConfig{Name: "test"}
	result, _ := m.Enable(context.Background(), config)

	for i := 0; i < 600; i++ {
		m.AddEvent(result.ID, "bulk_event", fmt.Sprintf("event %d", i), "info")
	}

	events := m.GetEvents(result.ID)
	if len(events) > GetConfig().MaxEvents {
		t.Fatalf("expected at most %d events, got %d", GetConfig().MaxEvents, len(events))
	}
}

func TestMultipleOperatorsSimultaneously(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 0})
	m.SetHealthChecker(hc)

	op1, _ := m.Enable(context.Background(), OperatorConfig{
		Name: "op1",
		HealthChecks: []HealthCheckDef{
			{ID: "a1", Name: "Alert 1", Type: CheckAlert, Target: "svc1", Threshold: 5},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}}},
		},
	})

	op2, _ := m.Enable(context.Background(), OperatorConfig{
		Name: "op2",
		HealthChecks: []HealthCheckDef{
			{ID: "a2", Name: "Alert 2", Type: CheckAlert, Target: "svc2", Threshold: 5},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}}},
		},
	})

	status1, err := m.RunCheckCycle(context.Background(), op1.ID)
	if err != nil {
		t.Fatalf("op1 RunCheckCycle failed: %v", err)
	}

	status2, err := m.RunCheckCycle(context.Background(), op2.ID)
	if err != nil {
		t.Fatalf("op2 RunCheckCycle failed: %v", err)
	}

	if status1.ConfigID != op1.ID {
		t.Fatal("op1 status has wrong config ID")
	}
	if status2.ConfigID != op2.ID {
		t.Fatal("op2 status has wrong config ID")
	}

	operators := m.ListOperators()
	if len(operators) != 2 {
		t.Fatalf("expected 2 operators, got %d", len(operators))
	}
}

func TestCronSchedulerIntegration(t *testing.T) {
	cs := newMockCronScheduler()
	m := NewOperatorManager()
	m.SetHealthChecker(NewHealthChecker())
	m.SetCronScheduler(cs)

	config := OperatorConfig{
		Name:     "test",
		Schedule: "*/1 * * * *",
		HealthChecks: []HealthCheckDef{
			{ID: "hc1", Name: "Check 1", Type: CheckAlert, Target: "svc", Threshold: 5, Schedule: "*/2 * * * *"},
		},
	}

	result, err := m.Enable(context.Background(), config)
	if err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	mainCronID := "operator_main_" + result.ID
	sched, ok := cs.getScheduled(mainCronID)
	if !ok {
		t.Fatal("expected main cron to be scheduled")
	}
	if sched != "*/1 * * * *" {
		t.Fatalf("expected schedule '*/1 * * * *', got %q", sched)
	}

	checkCronID := "operator_check_" + result.ID + "_hc1"
	sched, ok = cs.getScheduled(checkCronID)
	if !ok {
		t.Fatal("expected check cron to be scheduled")
	}
	if sched != "*/2 * * * *" {
		t.Fatalf("expected schedule '*/2 * * * *', got %q", sched)
	}

	m.Disable(result.ID)

	cs.mu.Lock()
	defer cs.mu.Unlock()
	foundUnschedule := false
	for _, id := range cs.unscheduled {
		if id == mainCronID || id == checkCronID {
			foundUnschedule = true
		}
	}
	if !foundUnschedule {
		t.Fatal("expected cron tasks to be unscheduled")
	}
}

func TestHealthCheckerWithMockProviders(t *testing.T) {
	hc := NewHealthChecker()
	hc.SetTopologyProvider(&mockTopologyProvider{health: "degraded"})
	hc.SetAlertProvider(&mockAlertProvider{count: 3})

	topoResult := hc.ExecuteCheck(context.Background(), HealthCheckDef{
		ID:     "t1",
		Name:   "Topology",
		Type:   CheckTopology,
		Target: "svc",
	})
	if topoResult.Status != CheckWarn {
		t.Fatalf("expected warn for degraded, got %s", topoResult.Status)
	}

	alertResult := hc.ExecuteCheck(context.Background(), HealthCheckDef{
		ID:        "a1",
		Name:      "Alert",
		Type:      CheckAlert,
		Target:    "svc",
		Threshold: 5,
	})
	if alertResult.Status != CheckPass {
		t.Fatalf("expected pass for count 3 < threshold 5, got %s", alertResult.Status)
	}
}

func TestOperatorEnableTool(t *testing.T) {
	mgr := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 0})
	mgr.SetHealthChecker(hc)
	SetOperatorManager(mgr)

	tool := &OperatorEnableTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name":           "test-operator",
		"schedule":       "*/10 * * * *",
		"autonomy_level": "auto",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}

	cfg, ok := resultMap["config"].(*OperatorConfig)
	if !ok {
		t.Fatal("expected config in result")
	}
	if cfg.Name != "test-operator" {
		t.Fatalf("expected name 'test-operator', got %q", cfg.Name)
	}
	if cfg.AutonomyLevel != AutonomyAuto {
		t.Fatalf("expected autonomy 'auto', got %q", cfg.AutonomyLevel)
	}
}

func TestOperatorDisableTool(t *testing.T) {
	mgr := NewOperatorManager()
	hc := NewHealthChecker()
	mgr.SetHealthChecker(hc)
	SetOperatorManager(mgr)

	enableResult, _ := mgr.Enable(context.Background(), OperatorConfig{
		Name: "test-operator",
	})

	tool := &OperatorDisableTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"config_id": enableResult.ID,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}

	disabled, ok := resultMap["disabled"].([]string)
	if !ok || len(disabled) != 1 {
		t.Fatalf("expected 1 disabled, got %v", resultMap["disabled"])
	}
}

func TestOperatorDisableAllTool(t *testing.T) {
	mgr := NewOperatorManager()
	hc := NewHealthChecker()
	mgr.SetHealthChecker(hc)
	SetOperatorManager(mgr)

	mgr.Enable(context.Background(), OperatorConfig{Name: "op1"})
	mgr.Enable(context.Background(), OperatorConfig{Name: "op2"})

	tool := &OperatorDisableTool{}
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map result")
	}

	disabled, ok := resultMap["disabled"].([]string)
	if !ok || len(disabled) != 2 {
		t.Fatalf("expected 2 disabled, got %v", resultMap["disabled"])
	}
}

func TestOperatorEnableToolNoName(t *testing.T) {
	mgr := NewOperatorManager()
	SetOperatorManager(mgr)

	tool := &OperatorEnableTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestResetEscalationState(t *testing.T) {
	m := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 10})
	m.SetHealthChecker(hc)

	config := OperatorConfig{
		Name:          "test",
		AutonomyLevel: AutonomyObserve,
		HealthChecks: []HealthCheckDef{
			{ID: "a1", Name: "Alert", Type: CheckAlert, Target: "svc", Threshold: 5},
		},
		EscalationPolicy: EscalationPolicy{
			Levels: []EscalationLevel{
				{Level: 1, Trigger: "check_fail", Actions: []string{"notify"}},
			},
		},
	}

	result, _ := m.Enable(context.Background(), config)
	m.RunCheckCycle(context.Background(), result.ID)

	status := m.GetStatus(result.ID)
	if status.ActiveEscalations == 0 {
		t.Fatal("expected active escalation")
	}

	m.ResetEscalationState(result.ID)
	status = m.GetStatus(result.ID)
	if status.ActiveEscalations != 0 {
		t.Fatalf("expected 0 active escalations after reset, got %d", status.ActiveEscalations)
	}

	esc, _ := m.EvaluateEscalation(result.ID)
	if esc == nil {
		t.Fatal("expected escalation to trigger again after reset")
	}
}

func TestGetStatusNotFound(t *testing.T) {
	m := newTestManager()
	status := m.GetStatus("nonexistent")
	if status != nil {
		t.Fatal("expected nil for nonexistent config")
	}
}

func TestRunCheckCycleNotFound(t *testing.T) {
	m := newTestManager()
	_, err := m.RunCheckCycle(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestRunCheckCycleNotEnabled(t *testing.T) {
	m := newTestManager()
	result, _ := m.Enable(context.Background(), OperatorConfig{Name: "test"})
	m.Disable(result.ID)

	_, err := m.RunCheckCycle(context.Background(), result.ID)
	if err == nil {
		t.Fatal("expected error for disabled config")
	}
}

func TestInitOperatorManager(t *testing.T) {
	hc := NewHealthChecker()
	cs := newMockCronScheduler()

	m := InitOperatorManager(hc, cs)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.healthChecker != hc {
		t.Fatal("expected health checker to be set")
	}
	if m.cronScheduler != cs {
		t.Fatal("expected cron scheduler to be set")
	}

	if DefaultOperatorManager() != m {
		t.Fatal("expected default manager to be set")
	}
}

func TestOperatorEnableToolWithHealthChecks(t *testing.T) {
	mgr := NewOperatorManager()
	hc := NewHealthChecker()
	hc.SetAlertProvider(&mockAlertProvider{count: 0})
	mgr.SetHealthChecker(hc)
	SetOperatorManager(mgr)

	tool := &OperatorEnableTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "test-with-checks",
		"health_checks": []any{
			map[string]any{
				"id":        "hc1",
				"name":      "Alert Check",
				"type":      "alert",
				"target":    "my-service",
				"threshold": float64(5),
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	cfg := resultMap["config"].(*OperatorConfig)
	if len(cfg.HealthChecks) != 1 {
		t.Fatalf("expected 1 health check, got %d", len(cfg.HealthChecks))
	}
	if cfg.HealthChecks[0].Type != CheckAlert {
		t.Fatalf("expected check type 'alert', got %q", cfg.HealthChecks[0].Type)
	}
}
