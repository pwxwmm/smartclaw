package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGlobToolExecute(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "glob-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	files := []string{"test1.txt", "test2.txt", "other.log"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"pattern": "*.txt",
		"path":    tmpDir,
	})

	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Errorf("Result should be a map, got %T", result)
	}

	files_result, ok := resultMap["files"].([]string)
	if !ok {
		t.Error("Expected files in result")
	}

	if len(files_result) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files_result))
	}
}

func TestGrepToolExecute(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grep-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1 hello\nline2 world\nline3 hello world\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"pattern": "hello",
		"path":    testFile,
	})

	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Errorf("Result should be a map, got %T", result)
	}

	matches, ok := resultMap["matches"].([]interface{})
	if !ok {
		t.Error("Expected matches in result")
	}

	if len(matches) == 0 {
		t.Error("Expected at least one match")
	}
}

func TestWebFetchTool(t *testing.T) {
	tool := &WebFetchTool{}

	if tool.Name() != "web_fetch" {
		t.Errorf("Expected name 'web_fetch', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestWebSearchTool(t *testing.T) {
	tool := &WebSearchTool{}

	if tool.Name() != "web_search" {
		t.Errorf("Expected name 'web_search', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestLSPTool(t *testing.T) {
	tool := &LSPTool{}

	if tool.Name() != "lsp" {
		t.Errorf("Expected name 'lsp', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestAgentTool(t *testing.T) {
	tool := &AgentTool{}

	if tool.Name() != "agent" {
		t.Errorf("Expected name 'agent', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestTodoWriteTool(t *testing.T) {
	tool := &TodoWriteTool{}

	if tool.Name() != "todowrite" {
		t.Errorf("Expected name 'todowrite', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestTodoWriteToolExecute(t *testing.T) {
	tool := NewTodoWriteTool("test-session")

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"todos": []interface{}{
			map[string]interface{}{
				"content": "Test task",
				"status":  "pending",
			},
		},
	})

	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	if result == nil {
		t.Error("Result should not be nil")
	}
}

func TestSkillTool(t *testing.T) {
	tool := &SkillTool{}

	if tool.Name() != "skill" {
		t.Errorf("Expected name 'skill', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestASTGrepTool(t *testing.T) {
	tool := &ASTGrepTool{}

	if tool.Name() != "ast_grep" {
		t.Errorf("Expected name 'ast_grep', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestCodeSearchTool(t *testing.T) {
	tool := &CodeSearchTool{}

	if tool.Name() != "code_search" {
		t.Errorf("Expected name 'code_search', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestBatchTool(t *testing.T) {
	tool := &BatchTool{}

	if tool.Name() != "batch" {
		t.Errorf("Expected name 'batch', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestParallelTool(t *testing.T) {
	tool := &ParallelTool{}

	if tool.Name() != "parallel" {
		t.Errorf("Expected name 'parallel', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestPipelineTool(t *testing.T) {
	tool := &PipelineTool{}

	if tool.Name() != "pipeline" {
		t.Errorf("Expected name 'pipeline', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestMcpTool(t *testing.T) {
	tool := &McpTool{}

	if tool.Name() != "mcp" {
		t.Errorf("Expected name 'mcp', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestImageTool(t *testing.T) {
	tool := &ImageTool{}

	if tool.Name() != "image" {
		t.Errorf("Expected name 'image', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestPDFTool(t *testing.T) {
	tool := &PDFTool{}

	if tool.Name() != "pdf" {
		t.Errorf("Expected name 'pdf', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestAudioTool(t *testing.T) {
	tool := &AudioTool{}

	if tool.Name() != "audio" {
		t.Errorf("Expected name 'audio', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestCacheTool(t *testing.T) {
	tool := &CacheTool{}

	if tool.Name() != "cache" {
		t.Errorf("Expected name 'cache', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestThinkTool(t *testing.T) {
	tool := &ThinkTool{}

	if tool.Name() != "think" {
		t.Errorf("Expected name 'think', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestDeepThinkTool(t *testing.T) {
	tool := &DeepThinkTool{}

	if tool.Name() != "deepthink" {
		t.Errorf("Expected name 'deepthink', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestForkTool(t *testing.T) {
	tool := &ForkTool{}

	if tool.Name() != "fork" {
		t.Errorf("Expected name 'fork', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestEnvTool(t *testing.T) {
	tool := &EnvTool{}

	if tool.Name() != "env" {
		t.Errorf("Expected name 'env', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}
}

func TestToolExecutionError(t *testing.T) {
	tool := &BashTool{}

	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing command")
	}
}

func TestRegistryGetNonExistent(t *testing.T) {
	registry := NewRegistry()

	if registry.Get("nonexistent") != nil {
		t.Error("Expected nil for non-existent tool")
	}
}

func TestRegistryExecuteNonExistent(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Execute(context.Background(), "nonexistent", nil)
	if err == nil {
		t.Error("Expected error for non-existent tool")
	}
}

func TestRegistryAllEmpty(t *testing.T) {
	registry := NewRegistry()

	if len(registry.All()) != 0 {
		t.Error("Expected empty registry")
	}

	if len(registry.Names()) != 0 {
		t.Error("Expected empty names")
	}
}
