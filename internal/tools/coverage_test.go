package tools

import (
	"context"
	"strings"
	"testing"
)

func TestTodoWriteToolMissingTodos(t *testing.T) {
	tool := NewTodoWriteTool("test-session")
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing todos")
	}
}

func TestTodoWriteToolInvalidTodos(t *testing.T) {
	tool := NewTodoWriteTool("test-session")
	_, err := tool.Execute(context.Background(), map[string]any{
		"todos": "not an array",
	})
	if err == nil {
		t.Error("Expected error for non-array todos")
	}
}

func TestTodoWriteToolWithPriorities(t *testing.T) {
	tool := NewTodoWriteTool("test-session")
	result, err := tool.Execute(context.Background(), map[string]any{
		"todos": []any{
			map[string]any{
				"content":  "High priority task",
				"status":   "pending",
				"priority": "high",
			},
			map[string]any{
				"content":  "Low priority task",
				"status":   "pending",
				"priority": "low",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["count"].(int) != 2 {
		t.Errorf("Expected count=2, got %d", resultMap["count"])
	}
	if resultMap["success"] != true {
		t.Error("Expected success=true")
	}
}

func TestTodoWriteToolDefaultSessionID(t *testing.T) {
	tool := NewTodoWriteTool("")
	result, err := tool.Execute(context.Background(), map[string]any{
		"todos": []any{
			map[string]any{
				"content": "Default session task",
				"status":  "pending",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result == nil {
		t.Error("Result should not be nil")
	}
}

func TestTodoWriteToolDefaultPriority(t *testing.T) {
	tool := NewTodoWriteTool("test")
	result, err := tool.Execute(context.Background(), map[string]any{
		"todos": []any{
			map[string]any{
				"content": "No priority specified",
				"status":  "pending",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resultMap := result.(map[string]any)
	newTodos, ok := resultMap["new_todos"].([]TodoItem)
	if !ok || len(newTodos) == 0 {
		t.Fatal("Expected new_todos in result")
	}
	if newTodos[0].Priority != "medium" {
		t.Errorf("Expected default priority 'medium', got %q", newTodos[0].Priority)
	}
}

func TestTodoWriteToolDefaultStatus(t *testing.T) {
	tool := NewTodoWriteTool("test")
	result, err := tool.Execute(context.Background(), map[string]any{
		"todos": []any{
			map[string]any{
				"content": "No status specified",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resultMap := result.(map[string]any)
	newTodos, ok := resultMap["new_todos"].([]TodoItem)
	if !ok || len(newTodos) == 0 {
		t.Fatal("Expected new_todos in result")
	}
	if newTodos[0].Status != "pending" {
		t.Errorf("Expected default status 'pending', got %q", newTodos[0].Status)
	}
}

func TestAskUserQuestionToolSchema(t *testing.T) {
	tool := &AskUserQuestionTool{}
	if tool.Name() != "ask_user" {
		t.Errorf("Expected name 'ask_user', got '%s'", tool.Name())
	}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestAskUserQuestionToolMissingQuestions(t *testing.T) {
	tool := &AskUserQuestionTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing questions")
	}
}

func TestAskUserQuestionToolExecute(t *testing.T) {
	tool := &AskUserQuestionTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"questions": []any{
			map[string]any{
				"question": "Do you want to continue?",
				"header":   "Confirm",
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
 resultMap := result.(map[string]any)
	if resultMap["status"] != "pending_user_response" {
		t.Errorf("Expected status 'pending_user_response', got %v", resultMap["status"])
	}
}

func TestConfigToolName(t *testing.T) {
	tool := &ConfigTool{}
	if tool.Name() != "config" {
		t.Errorf("Expected name 'config', got '%s'", tool.Name())
	}
}

func TestConfigToolSchema(t *testing.T) {
	tool := &ConfigTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestNotebookEditToolName(t *testing.T) {
	tool := &NotebookEditTool{}
	if tool.Name() != "notebook_edit" {
		t.Errorf("Expected name 'notebook_edit', got '%s'", tool.Name())
	}
}

func TestAssessQueryComplexity(t *testing.T) {
	tests := []struct {
		input   string
		wantMin float64
	}{
		{"hello", 0.0},
		{strings.Repeat("word ", 60), 0.2},
		{strings.Repeat("word ", 110), 0.4},
		{"refactor the codebase", 0.2},
		{"debug this error", 0.1},
		{"deploy to production", 0.15},
		{"use browser to scrape", 0.15},
		{"refactor and deploy to production with browser", 0.5},
	}

	for _, tt := range tests {
		score := AssessQueryComplexity(tt.input)
		if score < tt.wantMin {
			t.Errorf("AssessQueryComplexity(%q) = %.2f, want >= %.2f", tt.input[:min(len(tt.input), 30)], score, tt.wantMin)
		}
		if score > 1.0 {
			t.Errorf("AssessQueryComplexity(%q) = %.2f, want <= 1.0", tt.input[:min(len(tt.input), 30)], score)
		}
	}
}

func TestAssessQueryComplexityCapped(t *testing.T) {
	input := "refactor architect debug fix error deploy production browser scrape " + strings.Repeat("word ", 200)
	score := AssessQueryComplexity(input)
	if score > 1.0 {
		t.Errorf("Score should be capped at 1.0, got %.2f", score)
	}
}

func TestRegistryExecuteWithCache(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&ReadFileTool{})

	cache := NewResultCache(100, 0)
	registry.SetCache(cache)

	if registry.GetCache() != cache {
		t.Error("Cache should be set")
	}
}

func TestRegistryInvalidateCache(t *testing.T) {
	registry := NewRegistry()
	cache := NewResultCache(100, 0)
	registry.SetCache(cache)

	registry.InvalidateCache([]string{"/tmp/test.txt"})
}

func TestRegistrySetChainOptimizer(t *testing.T) {
	registry := NewRegistry()
	optimizer := NewChainOptimizer()
	registry.SetChainOptimizer(optimizer)

	if registry.GetChainOptimizer() != optimizer {
		t.Error("Chain optimizer should be set")
	}
}

func TestRegistrySetBatchExecutor(t *testing.T) {
	registry := NewRegistry()
	be := NewBatchExecutor()
	registry.SetBatchExecutor(be)

	if registry.GetBatchExecutor() != be {
		t.Error("Batch executor should be set")
	}
}

func TestRegistrySetDistributionCov(t *testing.T) {
	registry := NewRegistry()
	d := NewToolsetDistribution(1)
	registry.SetDistribution(d)

	if registry.GetDistribution() != d {
		t.Error("Distribution should be set")
	}
}

func TestRegistryUnregister(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&ReadFileTool{})

	if registry.Get("read_file") == nil {
		t.Error("Tool should be registered")
	}

	registry.Unregister("read_file")

	if registry.Get("read_file") != nil {
		t.Error("Tool should be unregistered")
	}
}

func TestExtractDepFilesCoverage(t *testing.T) {
	tests := []struct {
		toolName string
		input    map[string]any
		wantLen  int
	}{
		{"read_file", map[string]any{"path": "/tmp/test.txt"}, 1},
		{"write_file", map[string]any{"path": "/tmp/out.txt", "content": "hi"}, 1},
		{"glob", map[string]any{"pattern": "*.go", "path": "/src"}, 2},
		{"bash", map[string]any{"command": "echo hi"}, 0},
		{"edit_file", map[string]any{"path": "/tmp/f.go", "old_string": "a", "new_string": "b"}, 1},
	}

	for _, tt := range tests {
		files := extractDepFiles(tt.toolName, tt.input)
		if len(files) != tt.wantLen {
			t.Errorf("extractDepFiles(%q, %v) returned %d files, want %d", tt.toolName, tt.input, len(files), tt.wantLen)
		}
	}
}

func TestAuditResultToString(t *testing.T) {
	if auditResultToString(nil) != "" {
		t.Error("nil should return empty string")
	}
	if auditResultToString("hello") != "hello" {
		t.Error("string should be returned as-is")
	}
	if auditResultToString([]byte("bytes")) != "bytes" {
		t.Error("bytes should be converted to string")
	}
	result := auditResultToString(map[string]any{"key": "value"})
	if !strings.Contains(result, "key") {
		t.Error("map should be marshaled to JSON")
	}
}

func TestErrNotImplemented(t *testing.T) {
	err := ErrNotImplemented("my_tool")
	if err.Code != "NOT_IMPLEMENTED" {
		t.Errorf("Expected code 'NOT_IMPLEMENTED', got %q", err.Code)
	}
	if !strings.Contains(err.Message, "my_tool") {
		t.Error("Error message should contain tool name")
	}
}

func TestErrToolNotFound(t *testing.T) {
	err := ErrToolNotFound("missing_tool")
	if err.Code != "TOOL_NOT_FOUND" {
		t.Errorf("Expected code 'TOOL_NOT_FOUND', got %q", err.Code)
	}
}

func TestErrorToAppError(t *testing.T) {
	err := &Error{Code: "TEST", Message: "test error"}
	appErr := err.ToAppError()
	if appErr == nil {
		t.Error("ToAppError should return non-nil")
	}
}

func TestSopaToolNameToIncidentName(t *testing.T) {
	if sopaToolNameToIncidentName("sopa_list_faults") != "sopa_fault_tracking_list" {
		t.Error("Should map sopa_list_faults")
	}
	if sopaToolNameToIncidentName("sopa_other") != "sopa_other" {
		t.Error("Other names should pass through")
	}
}

func TestGetRegistry(t *testing.T) {
	registry := GetRegistry()
	if registry == nil {
		t.Error("GetRegistry should return non-nil")
	}
}

func TestRegisterAndGet(t *testing.T) {
	originalRegistry := defaultRegistry
	defer func() { defaultRegistry = originalRegistry }()

	defaultRegistry = NewRegistry()
	Register(&ReadFileTool{})

	tool := Get("read_file")
	if tool == nil {
		t.Error("Tool should be registered via package-level function")
	}
}

func TestAllTools(t *testing.T) {
	tools := All()
	if len(tools) == 0 {
		t.Error("All() should return tools")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
