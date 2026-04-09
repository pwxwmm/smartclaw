package tools

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type LSPTool struct{}

func NewLSPTool() *LSPTool {
	return &LSPTool{}
}

func (t *LSPTool) Name() string        { return "lsp" }
func (t *LSPTool) Description() string { return "LSP operations for code navigation and analysis" }

func (t *LSPTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"goto_definition", "find_references", "symbols", "diagnostics", "rename", "hover"},
			},
			"file_path": map[string]interface{}{"type": "string"},
			"line":      map[string]interface{}{"type": "integer"},
			"character": map[string]interface{}{"type": "integer"},
			"new_name":  map[string]interface{}{"type": "string"},
		},
		"required": []string{"operation", "file_path"},
	}
}

func (t *LSPTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	operation, _ := input["operation"].(string)
	filePath, _ := input["file_path"].(string)
	line, _ := input["line"].(int)
	character, _ := input["character"].(int)

	if filePath == "" {
		return nil, ErrRequiredField("file_path")
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	rootPath := filepath.Dir(absPath)

	client, err := GetOrCreateLSPClient(absPath, rootPath)
	if err != nil {
		return nil, &Error{Code: "LSP_ERROR", Message: fmt.Sprintf("Failed to create LSP client: %v", err)}
	}

	content, err := ioutil.ReadFile(absPath)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	languageID := t.getLanguageID(filepath.Ext(absPath))
	if err := client.DidOpen(ctx, absPath, languageID, string(content)); err != nil {
		return nil, &Error{Code: "LSP_ERROR", Message: fmt.Sprintf("Failed to open document: %v", err)}
	}
	switch operation {
	case "goto_definition":
		return client.GotoDefinition(ctx, absPath, line, character)
	case "find_references":
		return client.FindReferences(ctx, absPath, line, character)
	case "symbols":
		return client.DocumentSymbols(ctx, absPath)
	case "diagnostics":
		return t.getDiagnostics(ctx, client, absPath)
	case "rename":
		newName, _ := input["new_name"].(string)
		if newName == "" {
			return nil, ErrRequiredField("new_name")
		}
		return client.Rename(ctx, absPath, line, character, newName)
	case "hover":
		return client.Hover(ctx, absPath, line, character)
	default:
		return nil, &Error{Code: "INVALID_OPERATION", Message: "Unknown operation: " + operation}
	}
}

func (t *LSPTool) getDiagnostics(ctx context.Context, client *LSPClient, filePath string) (interface{}, error) {
	return map[string]interface{}{
		"diagnostics": []interface{}{},
		"message":     "Diagnostics support requires LSP server with diagnostic support",
	}, nil
}

func (t *LSPTool) getLanguageID(ext string) string {
	languageMap := map[string]string{
		".go":   "go",
		".ts":   "typescript",
		".tsx":  "typescriptreact",
		".js":   "javascript",
		".jsx":  "javascriptreact",
		".py":   "python",
		".rs":   "rust",
		".java": "java",
		".c":    "c",
		".cpp":  "cpp",
		".h":    "c",
		".hpp":  "cpp",
	}

	if lang, ok := languageMap[ext]; ok {
		return lang
	}
	return strings.TrimPrefix(ext, ".")
}

type SessionTool struct{}

func (t *SessionTool) Name() string        { return "session" }
func (t *SessionTool) Description() string { return "Session operations" }

func (t *SessionTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type": "string",
				"enum": []string{"list", "read", "search", "info"},
			},
			"session_id": map[string]interface{}{"type": "string"},
			"query":      map[string]interface{}{"type": "string"},
			"limit":      map[string]interface{}{"type": "integer"},
		},
		"required": []string{"operation"},
	}
}

func (t *SessionTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	operation, _ := input["operation"].(string)

	switch operation {
	case "list":
		return map[string]interface{}{"sessions": []interface{}{}, "message": "Session requires storage"}, nil
	case "read":
		sessionID, _ := input["session_id"].(string)
		return map[string]interface{}{"session_id": sessionID, "messages": []interface{}{}}, nil
	case "search":
		return map[string]interface{}{"results": []interface{}{}}, nil
	case "info":
		sessionID, _ := input["session_id"].(string)
		return map[string]interface{}{"session_id": sessionID}, nil
	default:
		return nil, &Error{Code: "INVALID_OPERATION", Message: "Unknown operation: " + operation}
	}
}
