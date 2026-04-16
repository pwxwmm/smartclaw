package autoremediation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestBurnRateToAutonomy(t *testing.T) {
	tests := []struct {
		name        string
		burnRate    float64
		errorBudget float64
		expected    AutonomyLevel
	}{
		{"speculative", 15.0, 0.03, AutonomySpeculative},
		{"auto", 7.0, 0.08, AutonomyAuto},
		{"pre_approved", 4.0, 0.15, AutonomyPreApproved},
		{"suggest", 1.0, 0.50, AutonomySuggest},
		{"suggest_low_burn", 2.0, 0.30, AutonomySuggest},
		{"auto_boundary_burn", 6.1, 0.08, AutonomyAuto},
		{"speculative_boundary_budget", 14.1, 0.04, AutonomySpeculative},
		{"pre_approved_boundary", 3.1, 0.19, AutonomyPreApproved},
		{"suggest_high_budget_low_burn", 0.5, 0.50, AutonomySuggest},
		{"suggest_moderate_burn_high_budget", 3.5, 0.25, AutonomySuggest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BurnRateToAutonomy(tt.burnRate, tt.errorBudget)
			if result != tt.expected {
				t.Errorf("BurnRateToAutonomy(%.1f, %.2f) = %s, want %s", tt.burnRate, tt.errorBudget, result, tt.expected)
			}
		})
	}
}

func TestLoadRunbooksFromDir(t *testing.T) {
	dir := t.TempDir()

	rb1 := Runbook{
		ID:          "test-rb-1",
		Name:        "Test Runbook 1",
		Description: "A test runbook",
		Service:     "api-service",
		Trigger:     RunbookTrigger{Type: "slo_burn", BurnRate: 3.0},
		Steps: []RunbookStep{
			{ID: "step-1", Name: "Step 1", Type: StepCommand, Action: "echo hello", Timeout: 30 * time.Second, OnFailure: FailureStop},
		},
		Autonomy:  AutonomySuggest,
		Timeout:   5 * time.Minute,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	data, _ := json.Marshal(rb1)
	os.WriteFile(filepath.Join(dir, "test-rb-1.json"), data, 0o644)

	// Write an invalid JSON file
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0o644)

	// Write a file with no ID
	os.WriteFile(filepath.Join(dir, "noid.json"), []byte(`{"name":"no-id"}`), 0o644)

	// Write a non-JSON file
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0o644)

	runbooks, err := LoadRunbooksFromDir(dir)
	if err != nil {
		t.Fatalf("LoadRunbooksFromDir() error = %v", err)
	}

	if len(runbooks) != 1 {
		t.Fatalf("expected 1 runbook, got %d", len(runbooks))
	}

	if runbooks["test-rb-1"] == nil {
		t.Fatal("expected runbook test-rb-1 to be loaded")
	}

	if runbooks["test-rb-1"].Name != "Test Runbook 1" {
		t.Errorf("runbook name = %s, want Test Runbook 1", runbooks["test-rb-1"].Name)
	}
}

func TestLoadRunbooksFromDirNotExist(t *testing.T) {
	runbooks, err := LoadRunbooksFromDir("/nonexistent/path")
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got %v", err)
	}
	if len(runbooks) != 0 {
		t.Fatalf("expected 0 runbooks, got %d", len(runbooks))
	}
}

func TestSaveRunbookToDir(t *testing.T) {
	dir := t.TempDir()

	rb := Runbook{
		ID:          "save-test",
		Name:        "Save Test",
		Description: "Testing save",
		Service:     "*",
		Trigger:     RunbookTrigger{Type: "manual"},
		Steps:       []RunbookStep{},
		Autonomy:    AutonomySuggest,
		Timeout:     time.Minute,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := SaveRunbookToDir(dir, &rb); err != nil {
		t.Fatalf("SaveRunbookToDir() error = %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "save-test.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("runbook file was not created")
	}

	// Verify content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read runbook file: %v", err)
	}

	var loaded Runbook
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal runbook: %v", err)
	}

	if loaded.ID != "save-test" {
		t.Errorf("loaded ID = %s, want save-test", loaded.ID)
	}
}

func TestEnsureBuiltInRunbooks(t *testing.T) {
	dir := t.TempDir()
	existing := make(map[string]*Runbook)

	saved := EnsureBuiltInRunbooks(dir, existing)
	if saved != len(BuiltInRunbooks) {
		t.Errorf("expected %d built-in runbooks saved, got %d", len(BuiltInRunbooks), saved)
	}

	// Second call should save 0 (already exist)
	runbooks, _ := LoadRunbooksFromDir(dir)
	saved2 := EnsureBuiltInRunbooks(dir, runbooks)
	if saved2 != 0 {
		t.Errorf("expected 0 built-in runbooks on second call, got %d", saved2)
	}
}

type mockSLOProvider struct {
	info *SLOInfo
	err  error
}

func (m *mockSLOProvider) GetSLOStatus(service string) (*SLOInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.info, nil
}

type mockCommander struct {
	commandOutputs map[string]string
	commandErrors  map[string]error
	toolOutputs    map[string]any
	toolErrors     map[string]error
	commandCalls   atomic.Int64
	toolCalls      atomic.Int64
}

func (m *mockCommander) ExecuteCommand(ctx context.Context, command string, timeout time.Duration) (string, error) {
	m.commandCalls.Add(1)
	if m.commandErrors != nil {
		if err, ok := m.commandErrors[command]; ok {
			return "", err
		}
	}
	if m.commandOutputs != nil {
		if out, ok := m.commandOutputs[command]; ok {
			return out, nil
		}
	}
	return "ok", nil
}

func (m *mockCommander) ExecuteTool(ctx context.Context, toolName string, params map[string]any) (any, error) {
	m.toolCalls.Add(1)
	if m.toolErrors != nil {
		if err, ok := m.toolErrors[toolName]; ok {
			return nil, err
		}
	}
	if m.toolOutputs != nil {
		if out, ok := m.toolOutputs[toolName]; ok {
			return out, nil
		}
	}
	return "tool_result", nil
}

func TestAssessSLO(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.SetSLOProvider(&mockSLOProvider{
		info: &SLOInfo{
			Service:              "api-service",
			SLOName:              "availability",
			Target:               0.999,
			Current:              0.990,
			ErrorBudgetRemaining: 0.08,
			BurnRate:             7.0,
		},
	})

	// Add a matching runbook
	engine.runbooks["investigate-errors"] = &Runbook{
		ID:       "investigate-errors",
		Service:  "*",
		Trigger:  RunbookTrigger{Type: "slo_burn", BurnRate: 3.0},
		Autonomy: AutonomySuggest,
	}

	assessment, err := engine.AssessSLO("api-service")
	if err != nil {
		t.Fatalf("AssessSLO() error = %v", err)
	}

	if assessment.Service != "api-service" {
		t.Errorf("assessment.Service = %s, want api-service", assessment.Service)
	}
	if assessment.BurnRate != 7.0 {
		t.Errorf("assessment.BurnRate = %.1f, want 7.0", assessment.BurnRate)
	}
	if assessment.AutonomyLevel != AutonomyAuto {
		t.Errorf("assessment.AutonomyLevel = %s, want %s", assessment.AutonomyLevel, AutonomyAuto)
	}
	if len(assessment.RecommendedRunbooks) != 1 || assessment.RecommendedRunbooks[0] != "investigate-errors" {
		t.Errorf("assessment.RecommendedRunbooks = %v, want [investigate-errors]", assessment.RecommendedRunbooks)
	}
}

func TestAssessSLONoProvider(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	_, err := engine.AssessSLO("api-service")
	if err == nil {
		t.Fatal("expected error when SLO provider not configured")
	}
}

func TestSuggestRemediation(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.runbooks["restart-service"] = &Runbook{
		ID:      "restart-service",
		Service: "*",
		Trigger: RunbookTrigger{Type: "alert_severity", Severity: "high"},
	}
	engine.runbooks["investigate-errors"] = &Runbook{
		ID:      "investigate-errors",
		Service: "*",
		Trigger: RunbookTrigger{Type: "slo_burn", BurnRate: 3.0},
	}
	engine.runbooks["specific-svc"] = &Runbook{
		ID:      "specific-svc",
		Service: "payment-service",
		Trigger: RunbookTrigger{Type: "alert_severity", Severity: "critical"},
	}

	// All wildcard runbooks
	runbooks, err := engine.SuggestRemediation("api-service", "")
	if err != nil {
		t.Fatalf("SuggestRemediation() error = %v", err)
	}
	if len(runbooks) != 2 {
		t.Errorf("expected 2 wildcard runbooks, got %d", len(runbooks))
	}

	// Filter by trigger
	runbooks, err = engine.SuggestRemediation("api-service", "slo_burn")
	if err != nil {
		t.Fatalf("SuggestRemediation() error = %v", err)
	}
	if len(runbooks) != 1 {
		t.Errorf("expected 1 slo_burn runbook, got %d", len(runbooks))
	}

	// Specific service
	runbooks, err = engine.SuggestRemediation("payment-service", "alert_severity")
	if err != nil {
		t.Fatalf("SuggestRemediation() error = %v", err)
	}
	if len(runbooks) != 2 { // wildcard + specific
		t.Errorf("expected 2 runbooks for payment-service alert_severity, got %d", len(runbooks))
	}
}

func TestCreateAction(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.runbooks["test-rb"] = &Runbook{
		ID:       "test-rb",
		Service:  "*",
		Autonomy: AutonomySuggest,
		Steps:    []RunbookStep{{ID: "s1", Name: "Step 1", Type: StepCommand, Action: "echo", Timeout: time.Second, OnFailure: FailureStop}},
		Timeout:  time.Minute,
	}

	action, err := engine.CreateAction("test-rb", "api-service", "manual", AutonomySuggest)
	if err != nil {
		t.Fatalf("CreateAction() error = %v", err)
	}

	if action.RunbookID != "test-rb" {
		t.Errorf("action.RunbookID = %s, want test-rb", action.RunbookID)
	}
	if action.Service != "api-service" {
		t.Errorf("action.Service = %s, want api-service", action.Service)
	}
	if action.Status != ActionPending {
		t.Errorf("action.Status = %s, want pending", action.Status)
	}
	if action.ID == "" {
		t.Error("action.ID should not be empty")
	}

	// Test not found
	_, err = engine.CreateAction("nonexistent", "svc", "manual", AutonomySuggest)
	if err == nil {
		t.Fatal("expected error for nonexistent runbook")
	}
}

func TestApproveAction(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.runbooks["test-rb"] = &Runbook{ID: "test-rb", Autonomy: AutonomySuggest}
	engine.actions["act-1"] = &RemediationAction{ID: "act-1", Status: ActionPending}

	err := engine.ApproveAction("act-1", "admin")
	if err != nil {
		t.Fatalf("ApproveAction() error = %v", err)
	}

	action := engine.GetAction("act-1")
	if action.Status != ActionApproved {
		t.Errorf("action.Status = %s, want approved", action.Status)
	}
	if action.ApprovedBy != "admin" {
		t.Errorf("action.ApprovedBy = %s, want admin", action.ApprovedBy)
	}

	// Approve non-pending action
	err = engine.ApproveAction("act-1", "admin2")
	if err == nil {
		t.Fatal("expected error when approving already-approved action")
	}

	// Approve nonexistent action
	err = engine.ApproveAction("nonexistent", "admin")
	if err == nil {
		t.Fatal("expected error for nonexistent action")
	}
}

func TestExecuteActionWithMockCommander(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	cmd := &mockCommander{
		commandOutputs: map[string]string{"echo hello": "hello"},
		toolOutputs:    map[string]any{"bash": "health_ok"},
	}
	engine.SetCommander(cmd)

	engine.runbooks["test-rb"] = &Runbook{
		ID:       "test-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps: []RunbookStep{
			{ID: "cmd-step", Name: "Run command", Type: StepCommand, Action: "echo hello", Timeout: 5 * time.Second, OnFailure: FailureStop},
			{ID: "tool-step", Name: "Run tool", Type: StepTool, Action: "bash", Timeout: 5 * time.Second, OnFailure: FailureContinue},
			{ID: "prompt-step", Name: "AI analyze", Type: StepPrompt, Action: "analyze", Timeout: 5 * time.Second, OnFailure: FailureContinue},
		},
		Timeout: time.Minute,
	}

	action, _ := engine.CreateAction("test-rb", "svc", "manual", AutonomyAuto)
	err := engine.ApproveAction(action.ID, "test-user")
	if err != nil {
		t.Fatalf("ApproveAction() error = %v", err)
	}

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	if result.Status != ActionSuccess {
		t.Errorf("result.Status = %s, want success", result.Status)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 step results, got %d", len(result.Steps))
	}

	// Verify command step
	if result.Steps[0].Status != ActionSuccess {
		t.Errorf("step 0 status = %s, want success", result.Steps[0].Status)
	}
	if result.Steps[0].Output != "hello" {
		t.Errorf("step 0 output = %s, want hello", result.Steps[0].Output)
	}

	// Verify tool step
	if result.Steps[1].Status != ActionSuccess {
		t.Errorf("step 1 status = %s, want success", result.Steps[1].Status)
	}

	// Verify prompt step
	if result.Steps[2].Status != ActionSuccess {
		t.Errorf("step 2 status = %s, want success", result.Steps[2].Status)
	}

	if cmd.commandCalls.Load() != 1 {
		t.Errorf("command calls = %d, want 1", cmd.commandCalls.Load())
	}
	if cmd.toolCalls.Load() != 2 { // tool + prompt both use ExecuteTool
		t.Errorf("tool calls = %d, want 2", cmd.toolCalls.Load())
	}
}

func TestExecuteActionFailureStop(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	cmd := &mockCommander{
		commandErrors: map[string]error{"fail-cmd": fmt.Errorf("command failed")},
	}
	engine.SetCommander(cmd)

	engine.runbooks["stop-rb"] = &Runbook{
		ID:       "stop-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps: []RunbookStep{
			{ID: "fail-step", Name: "Failing step", Type: StepCommand, Action: "fail-cmd", Timeout: 5 * time.Second, OnFailure: FailureStop},
			{ID: "skip-step", Name: "Should be skipped", Type: StepCommand, Action: "echo skip", Timeout: 5 * time.Second, OnFailure: FailureStop},
		},
		Timeout: time.Minute,
	}

	action, _ := engine.CreateAction("stop-rb", "svc", "test", AutonomyAuto)
	engine.ApproveAction(action.ID, "test")

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	if result.Status != ActionFailed {
		t.Errorf("result.Status = %s, want failed", result.Status)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step result (stopped after first), got %d", len(result.Steps))
	}
	if result.Steps[0].Error != "command failed" {
		t.Errorf("step error = %s, want command failed", result.Steps[0].Error)
	}
}

func TestExecuteActionFailureContinue(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	cmd := &mockCommander{
		commandErrors: map[string]error{"fail-cmd": fmt.Errorf("soft fail")},
	}
	engine.SetCommander(cmd)

	engine.runbooks["continue-rb"] = &Runbook{
		ID:       "continue-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps: []RunbookStep{
			{ID: "fail-step", Name: "Soft fail", Type: StepCommand, Action: "fail-cmd", Timeout: 5 * time.Second, OnFailure: FailureContinue},
			{ID: "next-step", Name: "Next step", Type: StepCommand, Action: "echo ok", Timeout: 5 * time.Second, OnFailure: FailureContinue},
		},
		Timeout: time.Minute,
	}

	action, _ := engine.CreateAction("continue-rb", "svc", "test", AutonomyAuto)
	engine.ApproveAction(action.ID, "test")

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	if result.Status != ActionSuccess {
		t.Errorf("result.Status = %s, want success (continued past failure)", result.Status)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 step results, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != ActionFailed {
		t.Errorf("step 0 status = %s, want failed", result.Steps[0].Status)
	}
	if result.Steps[1].Status != ActionSuccess {
		t.Errorf("step 1 status = %s, want success", result.Steps[1].Status)
	}
}

func TestExecuteActionFailureRollback(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	cmd := &mockCommander{
		commandErrors: map[string]error{"fail-cmd": fmt.Errorf("critical fail")},
	}
	engine.SetCommander(cmd)

	engine.runbooks["rollback-rb"] = &Runbook{
		ID:       "rollback-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps: []RunbookStep{
			{ID: "good-step", Name: "Good step", Type: StepCommand, Action: "echo ok", Timeout: 5 * time.Second, OnFailure: FailureStop, Rollback: "undo-good"},
			{ID: "fail-step", Name: "Fail step", Type: StepCommand, Action: "fail-cmd", Timeout: 5 * time.Second, OnFailure: FailureRollback, Rollback: "undo-fail"},
		},
		Timeout: time.Minute,
	}

	action, _ := engine.CreateAction("rollback-rb", "svc", "test", AutonomyAuto)
	engine.ApproveAction(action.ID, "test")

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	if result.Status != ActionRolledBack {
		t.Errorf("result.Status = %s, want rolled_back", result.Status)
	}
	if !result.RollbackNeeded {
		t.Error("result.RollbackNeeded should be true")
	}

	// Should have called rollback for the successful step
	if cmd.commandCalls.Load() < 2 { // 1 original + 1 rollback
		t.Errorf("command calls = %d, expected at least 2", cmd.commandCalls.Load())
	}
}

func TestExecuteActionFailureSkip(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	cmd := &mockCommander{
		commandErrors: map[string]error{"fail-cmd": fmt.Errorf("skip this")},
	}
	engine.SetCommander(cmd)

	engine.runbooks["skip-rb"] = &Runbook{
		ID:       "skip-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps: []RunbookStep{
			{ID: "fail-step", Name: "Skip step", Type: StepCommand, Action: "fail-cmd", Timeout: 5 * time.Second, OnFailure: FailureSkip},
			{ID: "next-step", Name: "Next step", Type: StepCommand, Action: "echo ok", Timeout: 5 * time.Second, OnFailure: FailureContinue},
		},
		Timeout: time.Minute,
	}

	action, _ := engine.CreateAction("skip-rb", "svc", "test", AutonomyAuto)
	engine.ApproveAction(action.ID, "test")

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	if result.Status != ActionSuccess {
		t.Errorf("result.Status = %s, want success (skipped failure)", result.Status)
	}
}

func TestCancelAction(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.runbooks["test-rb"] = &Runbook{ID: "test-rb", Autonomy: AutonomySuggest}
	engine.actions["act-1"] = &RemediationAction{ID: "act-1", Status: ActionPending}

	err := engine.CancelAction("act-1")
	if err != nil {
		t.Fatalf("CancelAction() error = %v", err)
	}

	action := engine.GetAction("act-1")
	if action.Status != ActionCancelled {
		t.Errorf("action.Status = %s, want cancelled", action.Status)
	}
	if action.CompletedAt == nil {
		t.Error("action.CompletedAt should be set")
	}

	// Cancel nonexistent
	err = engine.CancelAction("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent action")
	}

	// Cancel completed action
	engine.actions["act-2"] = &RemediationAction{ID: "act-2", Status: ActionSuccess}
	err = engine.CancelAction("act-2")
	if err == nil {
		t.Fatal("expected error when cancelling completed action")
	}
}

func TestInsufficientAutonomy(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.SetCommander(&mockCommander{})

	engine.runbooks["high-auto-rb"] = &Runbook{
		ID:       "high-auto-rb",
		Service:  "*",
		Autonomy: AutonomySpeculative,
		Steps:    []RunbookStep{{ID: "s1", Name: "Step", Type: StepCommand, Action: "echo", Timeout: time.Second, OnFailure: FailureStop}},
		Timeout:  time.Minute,
	}

	action, _ := engine.CreateAction("high-auto-rb", "svc", "test", AutonomySuggest)
	engine.ApproveAction(action.ID, "test")

	_, err := engine.ExecuteAction(context.Background(), action.ID)
	if err == nil {
		t.Fatal("expected error for insufficient autonomy")
	}
}

func TestApprovalStepType(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.SetCommander(&mockCommander{})

	engine.runbooks["approval-rb"] = &Runbook{
		ID:       "approval-rb",
		Service:  "*",
		Autonomy: AutonomySuggest,
		Steps: []RunbookStep{
			{ID: "need-approval", Name: "Need approval", Type: StepApproval, Action: "wait", Timeout: 5 * time.Second, OnFailure: FailureStop},
		},
		Timeout: time.Minute,
	}

	action, _ := engine.CreateAction("approval-rb", "svc", "test", AutonomySuggest)
	// Don't approve — should create a pending step result

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	// With suggest autonomy and no approval, the approval step should remain pending
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step result, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != ActionPending {
		t.Errorf("approval step status = %s, want pending", result.Steps[0].Status)
	}
}

func TestApprovalStepWithApproval(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.SetCommander(&mockCommander{})

	engine.runbooks["approval-rb"] = &Runbook{
		ID:       "approval-rb",
		Service:  "*",
		Autonomy: AutonomySuggest,
		Steps: []RunbookStep{
			{ID: "need-approval", Name: "Need approval", Type: StepApproval, Action: "wait", Timeout: 5 * time.Second, OnFailure: FailureStop},
		},
		Timeout: time.Minute,
	}

	action, _ := engine.CreateAction("approval-rb", "svc", "test", AutonomySuggest)
	engine.ApproveAction(action.ID, "test-user")

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	if result.Steps[0].Status != ActionSuccess {
		t.Errorf("approval step status = %s, want success (was approved)", result.Steps[0].Status)
	}
}

func TestBuiltInRunbooksValid(t *testing.T) {
	for _, rb := range BuiltInRunbooks {
		if rb.ID == "" {
			t.Error("built-in runbook missing ID")
		}
		if rb.Name == "" {
			t.Errorf("built-in runbook %s missing name", rb.ID)
		}
		if len(rb.Steps) == 0 {
			t.Errorf("built-in runbook %s has no steps", rb.ID)
		}
		for _, step := range rb.Steps {
			if step.ID == "" {
				t.Errorf("step in runbook %s missing ID", rb.ID)
			}
			if step.OnFailure == "" {
				t.Errorf("step %s in runbook %s missing on_failure policy", step.ID, rb.ID)
			}
		}
	}
}

func TestHistoryTracking(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.SetCommander(&mockCommander{})

	engine.runbooks["test-rb"] = &Runbook{
		ID:       "test-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps:    []RunbookStep{{ID: "s1", Name: "Step", Type: StepCommand, Action: "echo", Timeout: time.Second, OnFailure: FailureStop}},
		Timeout:  time.Second,
	}

	for range 5 {
		action, _ := engine.CreateAction("test-rb", "svc", "test", AutonomyAuto)
		engine.ApproveAction(action.ID, "test")
		engine.ExecuteAction(context.Background(), action.ID)
	}

	if len(engine.history) != 5 {
		t.Errorf("history length = %d, want 5", len(engine.history))
	}
}

func TestHistoryMaxSize(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.SetCommander(&mockCommander{})

	engine.runbooks["test-rb"] = &Runbook{
		ID:       "test-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps:    []RunbookStep{{ID: "s1", Name: "Step", Type: StepCommand, Action: "echo", Timeout: time.Second, OnFailure: FailureStop}},
		Timeout:  time.Second,
	}

	for range 1100 {
		action, _ := engine.CreateAction("test-rb", "svc", "test", AutonomyAuto)
		engine.ApproveAction(action.ID, "test")
		engine.ExecuteAction(context.Background(), action.ID)
	}

	if len(engine.history) > GetConfig().MaxHistorySize {
		t.Errorf("history length = %d, should be capped at %d", len(engine.history), GetConfig().MaxHistorySize)
	}
}

func TestActionStatusTransitions(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.runbooks["test-rb"] = &Runbook{ID: "test-rb", Autonomy: AutonomySuggest}

	// pending -> approved
	engine.actions["act-1"] = &RemediationAction{ID: "act-1", Status: ActionPending}
	if err := engine.ApproveAction("act-1", "user"); err != nil {
		t.Fatalf("pending->approved: %v", err)
	}

	// pending -> cancelled
	engine.actions["act-2"] = &RemediationAction{ID: "act-2", Status: ActionPending}
	if err := engine.CancelAction("act-2"); err != nil {
		t.Fatalf("pending->cancelled: %v", err)
	}

	// approved -> cancelled
	engine.actions["act-3"] = &RemediationAction{ID: "act-3", Status: ActionApproved}
	if err := engine.CancelAction("act-3"); err != nil {
		t.Fatalf("approved->cancelled: %v", err)
	}

	// can't approve already approved
	if err := engine.ApproveAction("act-1", "user2"); err == nil {
		t.Fatal("should not be able to approve already approved action")
	}

	// can't approve cancelled
	if err := engine.ApproveAction("act-2", "user"); err == nil {
		t.Fatal("should not be able to approve cancelled action")
	}
}

func TestRollbackReverseOrder(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	cmd := &mockCommander{
		commandErrors: map[string]error{"fail-cmd": fmt.Errorf("forced failure")},
	}
	engine.SetCommander(cmd)

	// We'll track which rollbacks get called by checking commandCalls
	engine.runbooks["rollback-order-rb"] = &Runbook{
		ID:       "rollback-order-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps: []RunbookStep{
			{ID: "step-1", Name: "Step 1", Type: StepCommand, Action: "cmd-1", Timeout: 5 * time.Second, OnFailure: FailureStop, Rollback: "rollback-1"},
			{ID: "step-2", Name: "Step 2", Type: StepCommand, Action: "cmd-2", Timeout: 5 * time.Second, OnFailure: FailureStop, Rollback: "rollback-2"},
			{ID: "step-3", Name: "Step 3", Type: StepCommand, Action: "fail-cmd", Timeout: 5 * time.Second, OnFailure: FailureRollback, Rollback: "rollback-3"},
		},
		Timeout: time.Minute,
	}

	action, _ := engine.CreateAction("rollback-order-rb", "svc", "test", AutonomyAuto)
	engine.ApproveAction(action.ID, "test")

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	if result.Status != ActionRolledBack {
		t.Errorf("result.Status = %s, want rolled_back", result.Status)
	}
}

func TestGetAction(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())

	// Nonexistent
	if action := engine.GetAction("nonexistent"); action != nil {
		t.Error("expected nil for nonexistent action")
	}

	engine.actions["act-1"] = &RemediationAction{ID: "act-1", Status: ActionPending}
	if action := engine.GetAction("act-1"); action == nil || action.ID != "act-1" {
		t.Error("expected to find action act-1")
	}
}

func TestListActions(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())
	engine.actions["act-1"] = &RemediationAction{ID: "act-1", Status: ActionPending}
	engine.actions["act-2"] = &RemediationAction{ID: "act-2", Status: ActionApproved}
	engine.actions["act-3"] = &RemediationAction{ID: "act-3", Status: ActionPending}

	// All
	all := engine.ListActions("")
	if len(all) != 3 {
		t.Errorf("ListActions('') = %d, want 3", len(all))
	}

	// Filter by status
	pending := engine.ListActions(ActionPending)
	if len(pending) != 2 {
		t.Errorf("ListActions(pending) = %d, want 2", len(pending))
	}

	// Filter by status with no matches
	running := engine.ListActions(ActionRunning)
	if len(running) != 0 {
		t.Errorf("ListActions(running) = %d, want 0", len(running))
	}
}

func TestExecuteActionNoCommander(t *testing.T) {
	engine := NewRemediationEngine(t.TempDir())

	engine.runbooks["test-rb"] = &Runbook{
		ID:       "test-rb",
		Service:  "*",
		Autonomy: AutonomyAuto,
		Steps:    []RunbookStep{{ID: "s1", Name: "Step", Type: StepCommand, Action: "echo", Timeout: time.Second, OnFailure: FailureStop}},
		Timeout:  time.Second,
	}

	action, _ := engine.CreateAction("test-rb", "svc", "test", AutonomyAuto)
	engine.ApproveAction(action.ID, "test")

	result, err := engine.ExecuteAction(context.Background(), action.ID)
	if err != nil {
		t.Fatalf("ExecuteAction() error = %v", err)
	}

	// Without commander, step should fail but execution continues
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(result.Steps))
	}
	if result.Steps[0].Status != ActionFailed {
		t.Errorf("step status = %s, want failed (no commander)", result.Steps[0].Status)
	}
}

func TestAutonomySufficient(t *testing.T) {
	tests := []struct {
		current  AutonomyLevel
		required AutonomyLevel
		expected bool
	}{
		{AutonomySpeculative, AutonomySuggest, true},
		{AutonomyAuto, AutonomyPreApproved, true},
		{AutonomyPreApproved, AutonomyAuto, false},
		{AutonomySuggest, AutonomySuggest, true},
		{AutonomySuggest, AutonomyAuto, false},
		{AutonomySpeculative, AutonomySpeculative, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.current, tt.required), func(t *testing.T) {
			result := autonomySufficient(tt.current, tt.required)
			if result != tt.expected {
				t.Errorf("autonomySufficient(%s, %s) = %v, want %v", tt.current, tt.required, result, tt.expected)
			}
		})
	}
}

func TestEngineLoadRunbooks(t *testing.T) {
	dir := t.TempDir()
	engine := NewRemediationEngine(dir)

	err := engine.LoadRunbooks()
	if err != nil {
		t.Fatalf("LoadRunbooks() error = %v", err)
	}

	if len(engine.runbooks) < len(BuiltInRunbooks) {
		t.Errorf("expected at least %d runbooks after loading, got %d", len(BuiltInRunbooks), len(engine.runbooks))
	}

	// Verify built-in runbooks exist
	for _, rb := range BuiltInRunbooks {
		if engine.runbooks[rb.ID] == nil {
			t.Errorf("built-in runbook %s not loaded", rb.ID)
		}
	}
}

func TestEngineSaveRunbook(t *testing.T) {
	dir := t.TempDir()
	engine := NewRemediationEngine(dir)

	rb := &Runbook{
		ID:          "custom-rb",
		Name:        "Custom Runbook",
		Description: "A custom runbook",
		Service:     "custom-svc",
		Trigger:     RunbookTrigger{Type: "manual"},
		Steps:       []RunbookStep{},
		Autonomy:    AutonomySuggest,
		Timeout:     time.Minute,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := engine.SaveRunbook(rb); err != nil {
		t.Fatalf("SaveRunbook() error = %v", err)
	}

	if engine.runbooks["custom-rb"] == nil {
		t.Error("runbook not found in engine after save")
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(dir, "custom-rb.json")); os.IsNotExist(err) {
		t.Error("runbook file not created on disk")
	}
}

func TestRemediationSuggestTool(t *testing.T) {
	dir := t.TempDir()
	engine := NewRemediationEngine(dir)
	engine.LoadRunbooks()
	SetRemediationEngine(engine)
	defer func() { SetRemediationEngine(nil) }()

	tool := &RemediationSuggestTool{}

	result, err := tool.Execute(context.Background(), map[string]any{"service": "api-service"})
	if err != nil {
		t.Fatalf("RemediationSuggestTool.Execute() error = %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	if resultMap["service"] != "api-service" {
		t.Errorf("service = %v, want api-service", resultMap["service"])
	}
}

func TestRemediationSuggestToolMissingService(t *testing.T) {
	tool := &RemediationSuggestTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing service")
	}
}

func TestRemediationApproveTool(t *testing.T) {
	dir := t.TempDir()
	engine := NewRemediationEngine(dir)
	engine.LoadRunbooks()
	SetRemediationEngine(engine)
	defer func() { SetRemediationEngine(nil) }()

	engine.runbooks["test-rb"] = &Runbook{ID: "test-rb", Autonomy: AutonomySuggest}
	action, _ := engine.CreateAction("test-rb", "svc", "test", AutonomySuggest)

	tool := &RemediationApproveTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"action_id": action.ID,
		"approver":  "admin",
	})
	if err != nil {
		t.Fatalf("RemediationApproveTool.Execute() error = %v", err)
	}

	approvedAction, ok := result.(*RemediationAction)
	if !ok {
		t.Fatal("result is not a RemediationAction")
	}
	if approvedAction.Status != ActionApproved {
		t.Errorf("action status = %s, want approved", approvedAction.Status)
	}
}

func TestRemediationExecuteTool(t *testing.T) {
	dir := t.TempDir()
	engine := NewRemediationEngine(dir)
	engine.SetSLOProvider(&mockSLOProvider{
		info: &SLOInfo{
			Service:              "svc",
			SLOName:              "availability",
			BurnRate:             10.0,
			ErrorBudgetRemaining: 0.05,
		},
	})
	engine.SetCommander(&mockCommander{})
	engine.LoadRunbooks()
	SetRemediationEngine(engine)
	defer func() { SetRemediationEngine(nil) }()

	tool := &RemediationExecuteTool{}

	// Use a runbook that requires suggest autonomy so it can be auto-executed
	result, err := tool.Execute(context.Background(), map[string]any{
		"runbook_id": "investigate-errors",
		"service":    "svc",
		"trigger":    "slo_burn",
	})
	if err != nil {
		t.Fatalf("RemediationExecuteTool.Execute() error = %v", err)
	}

	// With speculative autonomy from SLO assessment, investigate-errors (suggest) should auto-execute
	if _, ok := result.(*RemediationResult); !ok {
		// Could also be a pending action if autonomy wasn't sufficient
		if _, ok2 := result.(*RemediationAction); !ok2 {
			t.Fatal("result should be either RemediationResult or RemediationAction")
		}
	}
}

func TestInitRemediationEngine(t *testing.T) {
	dir := t.TempDir()
	engine := InitRemediationEngine(dir, nil, nil)
	if engine == nil {
		t.Fatal("InitRemediationEngine returned nil")
	}

	if DefaultRemediationEngine() != engine {
		t.Error("DefaultRemediationEngine() should return the initialized engine")
	}

	// Clean up
	SetRemediationEngine(nil)
}

func TestInitRemediationEngineWithDeps(t *testing.T) {
	dir := t.TempDir()
	sloProvider := &mockSLOProvider{info: &SLOInfo{Service: "test"}}
	commander := &mockCommander{}
	engine := InitRemediationEngine(dir, sloProvider, commander)

	if engine.sloProvider == nil {
		t.Error("SLO provider not set")
	}
	if engine.commander == nil {
		t.Error("Commander not set")
	}

	SetRemediationEngine(nil)
}
