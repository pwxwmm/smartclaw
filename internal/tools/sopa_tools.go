package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"
)

const sopaDefaultBaseURL = "http://localhost:8080"

func sopaGetBaseURL() string {
	if v := os.Getenv("SOPA_API_URL"); v != "" {
		return v
	}
	return sopaDefaultBaseURL
}

func sopaGetToken() string {
	return os.Getenv("SOPA_API_TOKEN")
}

var (
	sopaClientOnce sync.Once
	sopaHTTPClient *http.Client

	sopaBreakerMu    sync.Mutex
	sopaConsecFails  int
	sopaBreakerOpen  bool
	sopaBreakerUntil time.Time
)

const (
	sopaBreakerThreshold = 5                // open after 5 consecutive failures
	sopaBreakerCooldown  = 30 * time.Second // wait 30s before retrying
)

func sopaSharedClient() *http.Client {
	sopaClientOnce.Do(func() {
		sopaHTTPClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	})
	return sopaHTTPClient
}

func sopaBreakerAllow() error {
	sopaBreakerMu.Lock()
	defer sopaBreakerMu.Unlock()
	if sopaBreakerOpen {
		if time.Now().After(sopaBreakerUntil) {
			sopaBreakerOpen = false
			sopaConsecFails = 0
			return nil
		}
		return fmt.Errorf("SOPA: circuit breaker open, retry after %s", sopaBreakerUntil.Format(time.RFC3339))
	}
	return nil
}

func sopaBreakerRecord(success bool) {
	sopaBreakerMu.Lock()
	defer sopaBreakerMu.Unlock()
	if success {
		sopaConsecFails = 0
		sopaBreakerOpen = false
	} else {
		sopaConsecFails++
		if sopaConsecFails >= sopaBreakerThreshold {
			sopaBreakerOpen = true
			sopaBreakerUntil = time.Now().Add(sopaBreakerCooldown)
		}
	}
}

// sopaAPICall is a shared helper for SOPA REST API calls.
// method: HTTP method (GET, POST, etc.)
// path: API path (e.g. "/api/inventory/nodes")
// body: request body object (nil for no body)
// result: pointer to decode JSON response into
func sopaAPICall(method, path string, body, result any) error {
	if err := sopaBreakerAllow(); err != nil {
		return err
	}

	baseURL := sopaGetBaseURL()
	fullURL := baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("SOPA: failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("SOPA: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if token := sopaGetToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := sopaSharedClient().Do(req)
	if err != nil {
		sopaBreakerRecord(false)
		return fmt.Errorf("SOPA: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		sopaBreakerRecord(false)
		return fmt.Errorf("SOPA: failed to read response: %w", err)
	}

	if resp.StatusCode >= 500 {
		sopaBreakerRecord(false)
		return fmt.Errorf("SOPA: server error %d: %s", resp.StatusCode, string(respBody))
	}

	sopaBreakerRecord(true)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("SOPA: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("SOPA: failed to decode response: %w (body: %s)", err, string(respBody))
		}
	}

	return nil
}

func sopaBuildQueryParams(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	v := url.Values{}
	for key, val := range params {
		if val != "" {
			v.Set(key, val)
		}
	}
	encoded := v.Encode()
	if encoded == "" {
		return ""
	}
	return "?" + encoded
}

func sopaFloatToInt(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

func sopaGetString(input map[string]any, key string) string {
	v, _ := input[key].(string)
	return v
}

func sopaGetInt(input map[string]any, key string) int {
	return sopaFloatToInt(input[key])
}

// --- Infrastructure Tools ---

// SopaListNodesTool lists inventory nodes with filters.
type SopaListNodesTool struct{ BaseTool }

func (t *SopaListNodesTool) Name() string { return "sopa_list_nodes" }

func (t *SopaListNodesTool) Description() string {
	return "List inventory nodes from SOPA with optional filters. Use for SRE infrastructure monitoring and host discovery."
}

func (t *SopaListNodesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"page":      map[string]any{"type": "integer", "description": "Page number for pagination"},
			"pageSize":  map[string]any{"type": "integer", "description": "Number of results per page"},
			"status":    map[string]any{"type": "string", "description": "Filter by node status"},
			"online":    map[string]any{"type": "string", "description": "Filter by online status (true/false)"},
			"hostname":  map[string]any{"type": "string", "description": "Filter by hostname"},
			"ip":        map[string]any{"type": "string", "description": "Filter by IP address"},
			"keyword":   map[string]any{"type": "string", "description": "General keyword search"},
			"sortBy":    map[string]any{"type": "string", "description": "Field to sort by"},
			"sortOrder": map[string]any{"type": "string", "description": "Sort order (asc/desc)"},
		},
	}
}

func (t *SopaListNodesTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	params := map[string]string{
		"page":      strconv.Itoa(sopaGetInt(input, "page")),
		"pageSize":  strconv.Itoa(sopaGetInt(input, "pageSize")),
		"status":    sopaGetString(input, "status"),
		"online":    sopaGetString(input, "online"),
		"hostname":  sopaGetString(input, "hostname"),
		"ip":        sopaGetString(input, "ip"),
		"keyword":   sopaGetString(input, "keyword"),
		"sortBy":    sopaGetString(input, "sortBy"),
		"sortOrder": sopaGetString(input, "sortOrder"),
	}

	path := "/api/inventory/nodes" + sopaBuildQueryParams(params)

	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaGetNodeTool gets detailed node info.
type SopaGetNodeTool struct{ BaseTool }

func (t *SopaGetNodeTool) Name() string { return "sopa_get_node" }

func (t *SopaGetNodeTool) Description() string {
	return "Get detailed information about a specific infrastructure node. Use for SRE diagnostics and host investigation."
}

func (t *SopaGetNodeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "Node ID"},
		},
		"required": []string{"id"},
	}
}

func (t *SopaGetNodeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id := sopaGetString(input, "id")
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	path := fmt.Sprintf("/api/inventory/nodes/%s", url.PathEscape(id))
	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaNodeLogsTool gets node operation logs.
type SopaNodeLogsTool struct{ BaseTool }

func (t *SopaNodeLogsTool) Name() string { return "sopa_node_logs" }

func (t *SopaNodeLogsTool) Description() string {
	return "Get operation logs for a specific node. Use for SRE audit trails and troubleshooting node operations."
}

func (t *SopaNodeLogsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":            map[string]any{"type": "string", "description": "Node ID"},
			"page":          map[string]any{"type": "integer", "description": "Page number"},
			"pageSize":      map[string]any{"type": "integer", "description": "Results per page"},
			"operationType": map[string]any{"type": "string", "description": "Filter by operation type"},
			"status":        map[string]any{"type": "string", "description": "Filter by operation status"},
		},
		"required": []string{"id"},
	}
}

func (t *SopaNodeLogsTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id := sopaGetString(input, "id")
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	params := map[string]string{
		"page":          strconv.Itoa(sopaGetInt(input, "page")),
		"pageSize":      strconv.Itoa(sopaGetInt(input, "pageSize")),
		"operationType": sopaGetString(input, "operationType"),
		"status":        sopaGetString(input, "status"),
	}

	path := fmt.Sprintf("/api/inventory/nodes/%s/logs", url.PathEscape(id))
	path += sopaBuildQueryParams(params)

	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaNodeTasksTool gets node task history.
type SopaNodeTasksTool struct{ BaseTool }

func (t *SopaNodeTasksTool) Name() string { return "sopa_node_tasks" }

func (t *SopaNodeTasksTool) Description() string {
	return "Get task execution history for a specific node. Use for SRE to review past operations and task outcomes on a host."
}

func (t *SopaNodeTasksTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":        map[string]any{"type": "string", "description": "Node ID"},
			"job_type":  map[string]any{"type": "string", "description": "Filter by job type"},
			"user_name": map[string]any{"type": "string", "description": "Filter by user who initiated the task"},
		},
		"required": []string{"id"},
	}
}

func (t *SopaNodeTasksTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id := sopaGetString(input, "id")
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	params := map[string]string{
		"job_type":  sopaGetString(input, "job_type"),
		"user_name": sopaGetString(input, "user_name"),
	}

	path := fmt.Sprintf("/api/inventory/nodes/%s/tasks", url.PathEscape(id))
	path += sopaBuildQueryParams(params)

	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaClusterStatsTool gets cluster statistics.
type SopaClusterStatsTool struct{ BaseTool }

func (t *SopaClusterStatsTool) Name() string { return "sopa_cluster_stats" }

func (t *SopaClusterStatsTool) Description() string {
	return "Get statistics for an infrastructure cluster. Use for SRE capacity planning and cluster health monitoring."
}

func (t *SopaClusterStatsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "Cluster ID"},
		},
		"required": []string{"id"},
	}
}

func (t *SopaClusterStatsTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id := sopaGetString(input, "id")
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	path := fmt.Sprintf("/api/inventory/clusters/%s/stats", url.PathEscape(id))
	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Task Execution Tools ---

// SopaExecuteTaskTool executes a script on target agents.
type SopaExecuteTaskTool struct{ BaseTool }

func (t *SopaExecuteTaskTool) Name() string { return "sopa_execute_task" }

func (t *SopaExecuteTaskTool) Description() string {
	return "Execute a script on target agents via SOPA. Use for SRE deployment, patching, and remediation operations."
}

func (t *SopaExecuteTaskTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"scriptId":    map[string]any{"type": "string", "description": "Script ID to execute"},
			"agentIds":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Target agent IDs"},
			"params":      map[string]any{"type": "object", "description": "Script parameters as key-value pairs"},
			"timeout":     map[string]any{"type": "integer", "description": "Execution timeout in seconds"},
			"taskType":    map[string]any{"type": "string", "description": "Type of task"},
			"executeMode": map[string]any{"type": "string", "description": "Execution mode (e.g. serial, parallel)"},
			"async":       map[string]any{"type": "boolean", "description": "Whether to execute asynchronously"},
		},
		"required": []string{"scriptId"},
	}
}

func (t *SopaExecuteTaskTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	scriptID := sopaGetString(input, "scriptId")
	if scriptID == "" {
		return nil, ErrRequiredField("scriptId")
	}

	body := map[string]any{
		"scriptId": scriptID,
	}

	if v, ok := input["agentIds"]; ok {
		body["agentIds"] = v
	}
	if v, ok := input["params"]; ok {
		body["params"] = v
	}
	if v := sopaGetInt(input, "timeout"); v > 0 {
		body["timeout"] = v
	}
	if v := sopaGetString(input, "taskType"); v != "" {
		body["taskType"] = v
	}
	if v := sopaGetString(input, "executeMode"); v != "" {
		body["executeMode"] = v
	}
	if v, ok := input["async"]; ok {
		body["async"] = v
	}

	var result map[string]any
	if err := sopaAPICall("POST", "/api/task/execute", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaExecuteOrchestrationTool executes an orchestration workflow.
type SopaExecuteOrchestrationTool struct{ BaseTool }

func (t *SopaExecuteOrchestrationTool) Name() string { return "sopa_execute_orchestration" }

func (t *SopaExecuteOrchestrationTool) Description() string {
	return "Execute an orchestration workflow via SOPA. Use for SRE multi-step incident response and coordinated deployment operations."
}

func (t *SopaExecuteOrchestrationTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"orchestrationId": map[string]any{"type": "string", "description": "Orchestration workflow ID"},
			"agentIds":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Target agent IDs"},
			"executeMode":     map[string]any{"type": "string", "description": "Execution mode"},
			"timeout":         map[string]any{"type": "integer", "description": "Execution timeout in seconds"},
			"async":           map[string]any{"type": "boolean", "description": "Whether to execute asynchronously"},
		},
		"required": []string{"orchestrationId"},
	}
}

func (t *SopaExecuteOrchestrationTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	orchestrationID := sopaGetString(input, "orchestrationId")
	if orchestrationID == "" {
		return nil, ErrRequiredField("orchestrationId")
	}

	body := map[string]any{
		"orchestrationId": orchestrationID,
	}

	if v, ok := input["agentIds"]; ok {
		body["agentIds"] = v
	}
	if v := sopaGetString(input, "executeMode"); v != "" {
		body["executeMode"] = v
	}
	if v := sopaGetInt(input, "timeout"); v > 0 {
		body["timeout"] = v
	}
	if v, ok := input["async"]; ok {
		body["async"] = v
	}

	var result map[string]any
	if err := sopaAPICall("POST", "/api/orchestration/execute", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaTaskStatusTool checks task execution status.
type SopaTaskStatusTool struct{ BaseTool }

func (t *SopaTaskStatusTool) Name() string { return "sopa_task_status" }

func (t *SopaTaskStatusTool) Description() string {
	return "Check task execution status and list task executions. Use for SRE to monitor running and completed operations."
}

func (t *SopaTaskStatusTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"page":     map[string]any{"type": "integer", "description": "Page number"},
			"pageSize": map[string]any{"type": "integer", "description": "Results per page"},
			"status":   map[string]any{"type": "string", "description": "Filter by execution status"},
		},
	}
}

func (t *SopaTaskStatusTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	params := map[string]string{
		"page":     strconv.Itoa(sopaGetInt(input, "page")),
		"pageSize": strconv.Itoa(sopaGetInt(input, "pageSize")),
		"status":   sopaGetString(input, "status"),
	}

	path := "/api/task-execution/list" + sopaBuildQueryParams(params)

	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Fault & Incident Tools ---

// SopaListFaultsTool lists fault tracking records.
type SopaListFaultsTool struct{ BaseTool }

func (t *SopaListFaultsTool) Name() string { return "sopa_list_faults" }

func (t *SopaListFaultsTool) Description() string {
	return "List fault tracking records from SOPA. Use for SRE incident management and fault analysis."
}

func (t *SopaListFaultsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"page":     map[string]any{"type": "integer", "description": "Page number"},
			"pageSize": map[string]any{"type": "integer", "description": "Results per page"},
			"status":   map[string]any{"type": "string", "description": "Filter by fault status"},
			"severity": map[string]any{"type": "string", "description": "Filter by fault severity"},
		},
	}
}

func (t *SopaListFaultsTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	params := map[string]string{
		"page":     strconv.Itoa(sopaGetInt(input, "page")),
		"pageSize": strconv.Itoa(sopaGetInt(input, "pageSize")),
		"status":   sopaGetString(input, "status"),
		"severity": sopaGetString(input, "severity"),
	}

	path := "/api/fault-tracking/list" + sopaBuildQueryParams(params)

	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaGetFaultTool gets fault details.
type SopaGetFaultTool struct{ BaseTool }

func (t *SopaGetFaultTool) Name() string { return "sopa_get_fault" }

func (t *SopaGetFaultTool) Description() string {
	return "Get detailed information about a specific fault record. Use for SRE root cause analysis and incident investigation."
}

func (t *SopaGetFaultTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "Fault tracking record ID"},
		},
		"required": []string{"id"},
	}
}

func (t *SopaGetFaultTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id := sopaGetString(input, "id")
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	path := fmt.Sprintf("/api/fault-tracking/%s", url.PathEscape(id))
	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaListFaultTypesTool lists fault types.
type SopaListFaultTypesTool struct{ BaseTool }

func (t *SopaListFaultTypesTool) Name() string { return "sopa_list_fault_types" }

func (t *SopaListFaultTypesTool) Description() string {
	return "List available fault types from SOPA. Use for SRE incident categorization and pattern analysis."
}

func (t *SopaListFaultTypesTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *SopaListFaultTypesTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	var result map[string]any
	if err := sopaAPICall("GET", "/api/fault-types", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaFaultWarrantyTool gets warranty info for a node.
type SopaFaultWarrantyTool struct{ BaseTool }

func (t *SopaFaultWarrantyTool) Name() string { return "sopa_fault_warranty" }

func (t *SopaFaultWarrantyTool) Description() string {
	return "Get warranty information for a node. Use for SRE hardware lifecycle management and replacement planning."
}

func (t *SopaFaultWarrantyTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "Node ID to check warranty for"},
		},
		"required": []string{"id"},
	}
}

func (t *SopaFaultWarrantyTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id := sopaGetString(input, "id")
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	path := fmt.Sprintf("/api/fault-warranty/node/%s", url.PathEscape(id))
	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- Audit Tools ---

// SopaListAuditsTool lists task audit records.
type SopaListAuditsTool struct{ BaseTool }

func (t *SopaListAuditsTool) Name() string { return "sopa_list_audits" }

func (t *SopaListAuditsTool) Description() string {
	return "List task audit records from SOPA. Use for SRE compliance verification and approval workflow tracking."
}

func (t *SopaListAuditsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"page":     map[string]any{"type": "integer", "description": "Page number"},
			"pageSize": map[string]any{"type": "integer", "description": "Results per page"},
			"status":   map[string]any{"type": "string", "description": "Filter by audit status (pending, approved, rejected)"},
		},
	}
}

func (t *SopaListAuditsTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	params := map[string]string{
		"page":     strconv.Itoa(sopaGetInt(input, "page")),
		"pageSize": strconv.Itoa(sopaGetInt(input, "pageSize")),
		"status":   sopaGetString(input, "status"),
	}

	path := "/api/task-audit/list" + sopaBuildQueryParams(params)

	var result map[string]any
	if err := sopaAPICall("GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaApproveAuditTool approves a task audit.
type SopaApproveAuditTool struct{ BaseTool }

func (t *SopaApproveAuditTool) Name() string { return "sopa_approve_audit" }

func (t *SopaApproveAuditTool) Description() string {
	return "Approve a pending task audit in SOPA. Use for SRE change management and release approval workflows."
}

func (t *SopaApproveAuditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "Audit record ID to approve"},
		},
		"required": []string{"id"},
	}
}

func (t *SopaApproveAuditTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id := sopaGetString(input, "id")
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	path := fmt.Sprintf("/api/task-audit/%s/approve", url.PathEscape(id))
	var result map[string]any
	if err := sopaAPICall("POST", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SopaRejectAuditTool rejects a task audit.
type SopaRejectAuditTool struct{ BaseTool }

func (t *SopaRejectAuditTool) Name() string { return "sopa_reject_audit" }

func (t *SopaRejectAuditTool) Description() string {
	return "Reject a pending task audit in SOPA. Use for SRE change management and release gate enforcement."
}

func (t *SopaRejectAuditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "Audit record ID to reject"},
		},
		"required": []string{"id"},
	}
}

func (t *SopaRejectAuditTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	id := sopaGetString(input, "id")
	if id == "" {
		return nil, ErrRequiredField("id")
	}

	path := fmt.Sprintf("/api/task-audit/%s/reject", url.PathEscape(id))
	var result map[string]any
	if err := sopaAPICall("POST", path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
