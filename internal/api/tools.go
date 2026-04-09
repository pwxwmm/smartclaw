package api

// ToolDefinition represents a tool that can be used by the AI
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// ToolUse represents a tool use request from the AI
type ToolUse struct {
	ID    string                 `json:"id"`
	Type  string                 `json:"type"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Type      string      `json:"type"`
	ToolUseID string      `json:"tool_use_id"`
	Content   interface{} `json:"content"`
	IsError   bool        `json:"is_error,omitempty"`
}

// Common tool definitions
var BuiltinTools = []ToolDefinition{
	{
		Name:        "bash",
		Description: "Execute a shell command in the current workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The command to execute",
				},
				"timeout": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in milliseconds",
					"minimum":     1,
				},
			},
			"required": []string{"command"},
		},
	},
	{
		Name:        "read_file",
		Description: "Read a text file from the workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The file path to read",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Line offset to start from",
					"minimum":     0,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum lines to read",
					"minimum":     1,
				},
			},
			"required": []string{"path"},
		},
	},
	{
		Name:        "write_file",
		Description: "Write a text file in the workspace",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The file path to write",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The content to write",
				},
			},
			"required": []string{"path", "content"},
		},
	},
}
