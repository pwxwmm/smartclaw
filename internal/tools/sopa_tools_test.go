package tools

import (
	"os"
	"sync"
	"testing"
	"time"
)

func TestSopaGetBaseURL(t *testing.T) {
	os.Unsetenv("SOPA_API_URL")
	url := sopaGetBaseURL()
	if url != sopaDefaultBaseURL {
		t.Errorf("Expected default URL %s, got %s", sopaDefaultBaseURL, url)
	}
}

func TestSopaGetBaseURLFromEnv(t *testing.T) {
	os.Setenv("SOPA_API_URL", "http://custom:9090")
	defer os.Unsetenv("SOPA_API_URL")

	url := sopaGetBaseURL()
	if url != "http://custom:9090" {
		t.Errorf("Expected custom URL, got %s", url)
	}
}

func TestSopaGetToken(t *testing.T) {
	os.Setenv("SOPA_API_TOKEN", "test-token")
	defer os.Unsetenv("SOPA_API_TOKEN")

	token := sopaGetToken()
	if token != "test-token" {
		t.Errorf("Expected 'test-token', got %s", token)
	}
}

func TestSopaBuildQueryParams(t *testing.T) {
	tests := []struct {
		params map[string]string
		want   string
	}{
		{nil, ""},
		{map[string]string{}, ""},
		{map[string]string{"key": "val"}, "?key=val"},
		{map[string]string{"a": "1", "b": "2"}, "?a=1&b=2"},
		{map[string]string{"key": ""}, ""},
	}
	for _, tt := range tests {
		result := sopaBuildQueryParams(tt.params)
		if tt.params == nil || len(tt.params) == 0 {
			if result != "" {
				t.Errorf("Expected empty for %v, got %s", tt.params, result)
			}
		}
	}
}

func TestSopaFloatToInt(t *testing.T) {
	if sopaFloatToInt(float64(42)) != 42 {
		t.Error("float64(42) should convert to 42")
	}
	if sopaFloatToInt("not a float") != 0 {
		t.Error("non-float should return 0")
	}
	if sopaFloatToInt(nil) != 0 {
		t.Error("nil should return 0")
	}
}

func TestSopaGetString(t *testing.T) {
	input := map[string]any{"name": "test", "num": 42}
	if sopaGetString(input, "name") != "test" {
		t.Error("Should return string value")
	}
	if sopaGetString(input, "num") != "" {
		t.Error("Non-string should return empty")
	}
	if sopaGetString(input, "missing") != "" {
		t.Error("Missing key should return empty")
	}
}

func TestSopaGetInt(t *testing.T) {
	input := map[string]any{"count": float64(5), "name": "test"}
	if sopaGetInt(input, "count") != 5 {
		t.Error("Should return int from float64")
	}
	if sopaGetInt(input, "name") != 0 {
		t.Error("Non-number should return 0")
	}
}

func TestSopaBreakerAllowWhenClosed(t *testing.T) {
	sopaBreakerMu.Lock()
	sopaBreakerOpen = false
	sopaConsecFails = 0
	sopaBreakerMu.Unlock()

	if err := sopaBreakerAllow(); err != nil {
		t.Errorf("Breaker should allow when closed: %v", err)
	}
}

func TestSopaBreakerOpenAndCoolDown(t *testing.T) {
	sopaBreakerMu.Lock()
	sopaBreakerOpen = true
	sopaBreakerUntil = time.Now().Add(1 * time.Hour)
	sopaBreakerMu.Unlock()

	err := sopaBreakerAllow()
	if err == nil {
		t.Error("Breaker should reject when open and not cooled down")
	}

	sopaBreakerMu.Lock()
	sopaBreakerUntil = time.Now().Add(-1 * time.Second)
	sopaBreakerMu.Unlock()

	if err := sopaBreakerAllow(); err != nil {
		t.Errorf("Breaker should allow after cooldown: %v", err)
	}

	sopaBreakerMu.Lock()
	sopaBreakerOpen = false
	sopaConsecFails = 0
	sopaBreakerMu.Unlock()
}

func TestSopaBreakerRecordSuccess(t *testing.T) {
	sopaBreakerMu.Lock()
	sopaConsecFails = 3
	sopaBreakerMu.Unlock()

	sopaBreakerRecord(true)

	sopaBreakerMu.Lock()
	fails := sopaConsecFails
	open := sopaBreakerOpen
	sopaBreakerMu.Unlock()

	if fails != 0 {
		t.Errorf("Expected 0 consecutive fails, got %d", fails)
	}
	if open {
		t.Error("Breaker should not be open after success")
	}
}

func TestSopaBreakerRecordFailureOpensBreaker(t *testing.T) {
	sopaBreakerMu.Lock()
	sopaConsecFails = sopaBreakerThreshold - 1
	sopaBreakerMu.Unlock()

	sopaBreakerRecord(false)

	sopaBreakerMu.Lock()
	open := sopaBreakerOpen
	sopaBreakerMu.Unlock()

	if !open {
		t.Error("Breaker should open after threshold failures")
	}

	sopaBreakerMu.Lock()
	sopaBreakerOpen = false
	sopaConsecFails = 0
	sopaBreakerMu.Unlock()
}

func TestSopaBreakerRecordFailureBelowThreshold(t *testing.T) {
	sopaBreakerMu.Lock()
	sopaConsecFails = 0
	sopaBreakerOpen = false
	sopaBreakerMu.Unlock()

	sopaBreakerRecord(false)

	sopaBreakerMu.Lock()
	open := sopaBreakerOpen
	fails := sopaConsecFails
	sopaBreakerMu.Unlock()

	if open {
		t.Error("Breaker should not open below threshold")
	}
	if fails != 1 {
		t.Errorf("Expected 1 consecutive fail, got %d", fails)
	}

	sopaBreakerMu.Lock()
	sopaConsecFails = 0
	sopaBreakerMu.Unlock()
}

func TestSopaSharedClient(t *testing.T) {
	client := sopaSharedClient()
	if client == nil {
		t.Error("Shared client should not be nil")
	}
	client2 := sopaSharedClient()
	if client != client2 {
		t.Error("Should return same singleton client")
	}

	sopaClientOnce = sync.Once{}
	sopaHTTPClient = nil
}

func TestSopaToolNameToIncidentNameMapping(t *testing.T) {
	if sopaToolNameToIncidentName("sopa_list_faults") != "sopa_fault_tracking_list" {
		t.Error("Should map sopa_list_faults")
	}
	if sopaToolNameToIncidentName("sopa_other") != "sopa_other" {
		t.Error("Other names should pass through")
	}
}

func TestSopaListNodesToolSchema(t *testing.T) {
	tool := &SopaListNodesTool{}
	if tool.Name() != "sopa_list_nodes" {
		t.Errorf("Expected name 'sopa_list_nodes', got '%s'", tool.Name())
	}
	if tool.InputSchema() == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestSopaGetNodeToolMissingID(t *testing.T) {
	tool := &SopaGetNodeTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestSopaNodeLogsToolMissingID(t *testing.T) {
	tool := &SopaNodeLogsTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestSopaNodeTasksToolMissingID(t *testing.T) {
	tool := &SopaNodeTasksTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestSopaClusterStatsToolMissingID(t *testing.T) {
	tool := &SopaClusterStatsTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestSopaExecuteTaskToolMissingScriptID(t *testing.T) {
	tool := &SopaExecuteTaskTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing scriptId")
	}
}

func TestSopaExecuteOrchestrationToolMissingID(t *testing.T) {
	tool := &SopaExecuteOrchestrationTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing orchestrationId")
	}
}

func TestSopaGetFaultToolMissingID(t *testing.T) {
	tool := &SopaGetFaultTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestSopaFaultWarrantyToolMissingID(t *testing.T) {
	tool := &SopaFaultWarrantyTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestSopaApproveAuditToolMissingID(t *testing.T) {
	tool := &SopaApproveAuditTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestSopaRejectAuditToolMissingID(t *testing.T) {
	tool := &SopaRejectAuditTool{}
	_, err := tool.Execute(nil, map[string]any{})
	if err == nil {
		t.Error("Expected error for missing id")
	}
}

func TestSopaAllToolSchemas(t *testing.T) {
	tools := []Tool{
		&SopaListNodesTool{},
		&SopaGetNodeTool{},
		&SopaNodeLogsTool{},
		&SopaNodeTasksTool{},
		&SopaClusterStatsTool{},
		&SopaExecuteTaskTool{},
		&SopaExecuteOrchestrationTool{},
		&SopaTaskStatusTool{},
		&SopaListFaultsTool{},
		&SopaGetFaultTool{},
		&SopaListFaultTypesTool{},
		&SopaFaultWarrantyTool{},
		&SopaListAuditsTool{},
		&SopaApproveAuditTool{},
		&SopaRejectAuditTool{},
	}
	for _, tool := range tools {
		if tool.Name() == "" {
			t.Error("Tool name should not be empty")
		}
		if tool.InputSchema() == nil {
			t.Errorf("Tool %s: InputSchema should not be nil", tool.Name())
		}
		if tool.Description() == "" {
			t.Errorf("Tool %s: Description should not be empty", tool.Name())
		}
	}
}
