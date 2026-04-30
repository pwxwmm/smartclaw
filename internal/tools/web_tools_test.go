package tools

import (
	"context"
	"strings"
	"testing"
)

func TestWebFetchToolMissingURL(t *testing.T) {
	tool := &WebFetchTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing url")
	}
}

func TestWebFetchToolEmptyURL(t *testing.T) {
	tool := &WebFetchTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"url": "",
	})
	if err == nil {
		t.Error("Expected error for empty url")
	}
}

func TestWebFetchToolInvalidURL(t *testing.T) {
	tool := NewWebFetchTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"url": "not-a-valid-url",
	})
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestWebSearchToolMissingQuery(t *testing.T) {
	tool := &WebSearchTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing query")
	}
}

func TestWebSearchToolEmptyQuery(t *testing.T) {
	tool := &WebSearchTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"query": "",
	})
	if err == nil {
		t.Error("Expected error for empty query")
	}
}

func TestNewWebFetchTool(t *testing.T) {
	tool := NewWebFetchTool()
	if tool == nil {
		t.Fatal("NewWebFetchTool returned nil")
	}
	if tool.client == nil {
		t.Error("Client should not be nil")
	}
}

func TestNewWebSearchTool(t *testing.T) {
	tool := NewWebSearchTool("test-key", "exa")
	if tool == nil {
		t.Fatal("NewWebSearchTool returned nil")
	}
	if tool.apiKey != "test-key" {
		t.Error("API key not set")
	}
	if tool.engine != "exa" {
		t.Error("Engine not set")
	}
}

func TestHTTPRequestToolName(t *testing.T) {
	tool := &HTTPRequestTool{}
	if tool.Name() != "http_request" {
		t.Errorf("Expected name 'http_request', got '%s'", tool.Name())
	}
}

func TestHTTPRequestToolMissingURL(t *testing.T) {
	tool := NewHTTPRequestTool()
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing url")
	}
}

func TestJSONParserToolName(t *testing.T) {
	tool := &JSONParserTool{}
	if tool.Name() != "json_parse" {
		t.Errorf("Expected name 'json_parse', got '%s'", tool.Name())
	}
}

func TestJSONParserToolMissingInput(t *testing.T) {
	tool := &JSONParserTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing json input")
	}
}

func TestJSONParserToolInvalidJSON(t *testing.T) {
	tool := &JSONParserTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"json": "not valid json{{{",
	})
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestJSONParserToolValidJSON(t *testing.T) {
	tool := &JSONParserTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"json": `{"key": "value"}`,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}
	if resultMap["key"] != "value" {
		t.Errorf("Expected key=value, got %v", resultMap["key"])
	}
}

func TestJSONStringifyToolName(t *testing.T) {
	tool := &JSONStringifyTool{}
	if tool.Name() != "json_stringify" {
		t.Errorf("Expected name 'json_stringify', got '%s'", tool.Name())
	}
}

func TestJSONStringifyToolExecute(t *testing.T) {
	tool := &JSONStringifyTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"object": map[string]any{"a": 1},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	str, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got %T", result)
	}
	if !strings.Contains(str, `"a"`) {
		t.Errorf("Expected JSON with 'a', got %q", str)
	}
}

func TestURLParserToolName(t *testing.T) {
	tool := &URLParserTool{}
	if tool.Name() != "url_parse" {
		t.Errorf("Expected name 'url_parse', got '%s'", tool.Name())
	}
}

func TestURLParserToolMissingURL(t *testing.T) {
	tool := &URLParserTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing url")
	}
}

func TestURLParserToolExecute(t *testing.T) {
	tool := &URLParserTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"url": "https://example.com/path/to/resource",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resultMap := result.(map[string]any)
	if resultMap["scheme"] != "https" {
		t.Errorf("Expected scheme 'https', got %v", resultMap["scheme"])
	}
	if resultMap["host"] != "example.com" {
		t.Errorf("Expected host 'example.com', got %v", resultMap["host"])
	}
	if resultMap["path"] != "/path/to/resource" {
		t.Errorf("Expected path '/path/to/resource', got %v", resultMap["path"])
	}
}

func TestTextTransformToolName(t *testing.T) {
	tool := &TextTransformTool{}
	if tool.Name() != "text_transform" {
		t.Errorf("Expected name 'text_transform', got '%s'", tool.Name())
	}
}

func TestTextTransformToolUpper(t *testing.T) {
	tool := &TextTransformTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"text":      "hello",
		"transform": "upper",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "HELLO" {
		t.Errorf("Expected 'HELLO', got %q", result)
	}
}

func TestTextTransformToolLower(t *testing.T) {
	tool := &TextTransformTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"text":      "HELLO",
		"transform": "lower",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "hello" {
		t.Errorf("Expected 'hello', got %q", result)
	}
}

func TestTextTransformToolReverse(t *testing.T) {
	tool := &TextTransformTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"text":      "hello",
		"transform": "reverse",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "olleh" {
		t.Errorf("Expected 'olleh', got %q", result)
	}
}

func TestTextTransformToolUnknown(t *testing.T) {
	tool := &TextTransformTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"text":      "hello",
		"transform": "unknown",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "hello" {
		t.Errorf("Unknown transform should return original text, got %q", result)
	}
}
