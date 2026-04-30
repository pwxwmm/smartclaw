package tools

import (
	"context"
	"strings"
	"testing"
)

func TestLSPToolMissingFilePath(t *testing.T) {
	tool := &LSPTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "goto_definition",
	})
	if err == nil {
		t.Error("Expected error for missing file_path")
	}
}

func TestLSPToolEmptyFilePath(t *testing.T) {
	tool := &LSPTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"operation": "goto_definition",
		"file_path": "",
	})
	if err == nil {
		t.Error("Expected error for empty file_path")
	}
}

func TestLSPToolSchema(t *testing.T) {
	tool := &LSPTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Error("Schema should have properties")
	}
	if _, hasOp := props["operation"]; !hasOp {
		t.Error("Schema should have operation property")
	}
	if _, hasFile := props["file_path"]; !hasFile {
		t.Error("Schema should have file_path property")
	}
}

func TestASTGrepToolMissingPattern(t *testing.T) {
	tool := &ASTGrepTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"lang": "go",
	})
	if err == nil {
		t.Error("Expected error for missing pattern")
	}
}

func TestASTGrepToolMissingLang(t *testing.T) {
	tool := &ASTGrepTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "func $NAME($$$) { $$$ }",
	})
	if err == nil {
		t.Error("Expected error for missing lang")
	}
}

func TestASTGrepToolEmptyPattern(t *testing.T) {
	tool := &ASTGrepTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "",
		"lang":    "go",
	})
	if err == nil {
		t.Error("Expected error for empty pattern")
	}
}

func TestASTGrepToolEmptyLang(t *testing.T) {
	tool := &ASTGrepTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "func $NAME($$$) { $$$ }",
		"lang":    "",
	})
	if err == nil {
		t.Error("Expected error for empty lang")
	}
}

func TestASTGrepToolSchema(t *testing.T) {
	tool := &ASTGrepTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Error("Schema should have properties")
	}
	if _, hasPattern := props["pattern"]; !hasPattern {
		t.Error("Schema should have pattern property")
	}
	if _, hasLang := props["lang"]; !hasLang {
		t.Error("Schema should have lang property")
	}
}

func TestASTGrepToolRequiresExternalBinary(t *testing.T) {
	tool := &ASTGrepTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "fmt.Println($MSG)",
		"lang":    "go",
	})
	if err != nil && strings.Contains(err.Error(), "ast-grep") {
		t.Skip("ast-grep binary not available")
	}
}

func TestCodeSearchToolName(t *testing.T) {
	tool := &CodeSearchTool{}
	if tool.Name() != "code_search" {
		t.Errorf("Expected name 'code_search', got '%s'", tool.Name())
	}
}

func TestCodeSearchToolSchema(t *testing.T) {
	tool := &CodeSearchTool{}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestIndexToolName(t *testing.T) {
	tool := &IndexTool{}
	if tool.Name() != "index" {
		t.Errorf("Expected name 'index', got '%s'", tool.Name())
	}
}
