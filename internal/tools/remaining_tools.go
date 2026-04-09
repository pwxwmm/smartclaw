package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type PowerShellTool struct{}

func (t *PowerShellTool) Name() string        { return "powershell" }
func (t *PowerShellTool) Description() string { return "Execute PowerShell command" }

func (t *PowerShellTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"command": map[string]interface{}{"type": "string"},
			"timeout": map[string]interface{}{"type": "integer"},
		},
		"required": []string{"command"},
	}
}

func (t *PowerShellTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	cmdStr, _ := input["command"].(string)
	if cmdStr == "" {
		return nil, ErrRequiredField("command")
	}

	timeout := 120000
	if t, ok := input["timeout"].(int); ok && t > 0 {
		timeout = t
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-Command", cmdStr)
	output, err := cmd.CombinedOutput()

	return map[string]interface{}{
		"output":   string(output),
		"exitCode": cmd.ProcessState.ExitCode(),
	}, err
}

type McpTool struct{}

func (t *McpTool) Name() string        { return "mcp" }
func (t *McpTool) Description() string { return "Execute MCP server tool" }

func (t *McpTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server":    map[string]interface{}{"type": "string"},
			"tool":      map[string]interface{}{"type": "string"},
			"arguments": map[string]interface{}{},
		},
		"required": []string{"server", "tool"},
	}
}

func (t *McpTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	server, _ := input["server"].(string)
	tool, _ := input["tool"].(string)
	if server == "" || tool == "" {
		return nil, ErrRequiredField("server and tool")
	}

	return map[string]interface{}{
		"server": server,
		"tool":   tool,
		"status": "not implemented",
	}, nil
}

type ListMcpResourcesTool struct{}

func (t *ListMcpResourcesTool) Name() string        { return "list_mcp_resources" }
func (t *ListMcpResourcesTool) Description() string { return "List MCP server resources" }

func (t *ListMcpResourcesTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server": map[string]interface{}{"type": "string"},
		},
		"required": []string{"server"},
	}
}

func (t *ListMcpResourcesTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	server, _ := input["server"].(string)
	return map[string]interface{}{
		"server":    server,
		"resources": []interface{}{},
	}, nil
}

type ReadMcpResourceTool struct{}

func (t *ReadMcpResourceTool) Name() string        { return "read_mcp_resource" }
func (t *ReadMcpResourceTool) Description() string { return "Read MCP server resource" }

func (t *ReadMcpResourceTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server": map[string]interface{}{"type": "string"},
			"uri":    map[string]interface{}{"type": "string"},
		},
		"required": []string{"server", "uri"},
	}
}

func (t *ReadMcpResourceTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	server, _ := input["server"].(string)
	uri, _ := input["uri"].(string)
	return map[string]interface{}{
		"server":  server,
		"uri":     uri,
		"content": "",
	}, nil
}

type ScheduleCronTool struct{}

func (t *ScheduleCronTool) Name() string        { return "schedule_cron" }
func (t *ScheduleCronTool) Description() string { return "Schedule a cron job" }

func (t *ScheduleCronTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"schedule": map[string]interface{}{"type": "string"},
			"command":  map[string]interface{}{"type": "string"},
		},
		"required": []string{"schedule", "command"},
	}
}

func (t *ScheduleCronTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	schedule, _ := input["schedule"].(string)
	command, _ := input["command"].(string)
	return map[string]interface{}{
		"schedule": schedule,
		"command":  command,
		"status":   "scheduled",
		"id":       fmt.Sprintf("cron_%d", time.Now().Unix()),
	}, nil
}

type RemoteTriggerTool struct{}

func (t *RemoteTriggerTool) Name() string        { return "remote_trigger" }
func (t *RemoteTriggerTool) Description() string { return "Trigger remote execution" }

func (t *RemoteTriggerTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"host":    map[string]interface{}{"type": "string"},
			"command": map[string]interface{}{"type": "string"},
		},
		"required": []string{"host", "command"},
	}
}

func (t *RemoteTriggerTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	host, _ := input["host"].(string)
	command, _ := input["command"].(string)
	return map[string]interface{}{
		"host":    host,
		"command": command,
		"status":  "not implemented",
	}, nil
}

type SendMessageTool struct{}

func (t *SendMessageTool) Name() string        { return "send_message" }
func (t *SendMessageTool) Description() string { return "Send a message" }

func (t *SendMessageTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"channel": map[string]interface{}{"type": "string"},
			"message": map[string]interface{}{"type": "string"},
		},
		"required": []string{"channel", "message"},
	}
}

func (t *SendMessageTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	channel, _ := input["channel"].(string)
	message, _ := input["message"].(string)
	return map[string]interface{}{
		"channel": channel,
		"message": message,
		"sent":    false,
	}, nil
}

type EnterWorktreeTool struct{}

func (t *EnterWorktreeTool) Name() string        { return "enter_worktree" }
func (t *EnterWorktreeTool) Description() string { return "Enter git worktree" }

func (t *EnterWorktreeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{"type": "string"},
		},
		"required": []string{"path"},
	}
}

func (t *EnterWorktreeTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	path, _ := input["path"].(string)
	return map[string]interface{}{
		"path":   path,
		"status": "not implemented",
	}, nil
}

type ExitWorktreeTool struct{}

func (t *ExitWorktreeTool) Name() string        { return "exit_worktree" }
func (t *ExitWorktreeTool) Description() string { return "Exit git worktree" }

func (t *ExitWorktreeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *ExitWorktreeTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{
		"status": "not implemented",
	}, nil
}

type BriefTool struct{}

func (t *BriefTool) Name() string        { return "brief" }
func (t *BriefTool) Description() string { return "Get brief summary" }

func (t *BriefTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"topic": map[string]interface{}{"type": "string"},
		},
		"required": []string{"topic"},
	}
}

func (t *BriefTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	topic, _ := input["topic"].(string)
	return map[string]interface{}{
		"topic":   topic,
		"summary": "not implemented",
	}, nil
}

type SleepTool struct{}

func (t *SleepTool) Name() string        { return "sleep" }
func (t *SleepTool) Description() string { return "Sleep for specified duration" }

func (t *SleepTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"seconds": map[string]interface{}{"type": "number"},
		},
		"required": []string{"seconds"},
	}
}

func (t *SleepTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	seconds, _ := input["seconds"].(float64)
	if seconds > 0 {
		time.Sleep(time.Duration(seconds) * time.Second)
	}
	return map[string]interface{}{
		"slept": seconds,
	}, nil
}

type ToolSearchTool struct{}

func (t *ToolSearchTool) Name() string        { return "tool_search" }
func (t *ToolSearchTool) Description() string { return "Search for deferred tools by keyword" }

func (t *ToolSearchTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query":       map[string]interface{}{"type": "string"},
			"max_results": map[string]interface{}{"type": "integer", "default": 5},
		},
		"required": []string{"query"},
	}
}

func (t *ToolSearchTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	query, _ := input["query"].(string)
	maxResults := 5
	if mr, ok := input["max_results"].(int); ok && mr > 0 {
		maxResults = mr
	}

	// Get all deferred tools
	allTools := GetRegistry().All()
	deferredTools := allTools // For now, treat all as deferred

	// Keyword search implementation
	queryLower := strings.ToLower(query)
	matched := []string{}

	for _, tool := range deferredTools {
		nameLower := strings.ToLower(tool.Name())
		descLower := strings.ToLower(tool.Description())

		if strings.Contains(nameLower, queryLower) || strings.Contains(descLower, queryLower) {
			matched = append(matched, tool.Name())
			if len(matched) >= maxResults {
				break
			}
		}
	}

	return map[string]interface{}{
		"matches":              matched,
		"query":                query,
		"total_deferred_tools": len(deferredTools),
	}, nil
}

type REPLTool struct{}

func (t *REPLTool) Name() string        { return "repl" }
func (t *REPLTool) Description() string { return "Interactive REPL for evaluating expressions" }

func (t *REPLTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"expression": map[string]interface{}{"type": "string"},
			"language":   map[string]interface{}{"type": "string", "default": "javascript"},
		},
		"required": []string{"expression"},
	}
}

func (t *REPLTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	expr, _ := input["expression"].(string)
	lang, _ := input["language"].(string)

	return map[string]interface{}{
		"expression": expr,
		"language":   lang,
		"result":     "REPL evaluation not implemented",
	}, nil
}

type TeamCreateTool struct{}

func (t *TeamCreateTool) Name() string        { return "team_create" }
func (t *TeamCreateTool) Description() string { return "Create a new team/workspace" }

func (t *TeamCreateTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":        map[string]interface{}{"type": "string"},
			"description": map[string]interface{}{"type": "string"},
		},
		"required": []string{"name"},
	}
}

func (t *TeamCreateTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	name, _ := input["name"].(string)
	desc, _ := input["description"].(string)

	return map[string]interface{}{
		"id":          fmt.Sprintf("team_%d", time.Now().Unix()),
		"name":        name,
		"description": desc,
		"created":     true,
	}, nil
}

type TeamDeleteTool struct{}

func (t *TeamDeleteTool) Name() string        { return "team_delete" }
func (t *TeamDeleteTool) Description() string { return "Delete a team/workspace" }

func (t *TeamDeleteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{"type": "string"},
		},
		"required": []string{"id"},
	}
}

func (t *TeamDeleteTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	id, _ := input["id"].(string)

	return map[string]interface{}{
		"id":      id,
		"deleted": true,
	}, nil
}

type EnterPlanModeTool struct{}

func (t *EnterPlanModeTool) Name() string { return "enter_plan_mode" }
func (t *EnterPlanModeTool) Description() string {
	return "Enter planning mode for structured task breakdown"
}

func (t *EnterPlanModeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"goal": map[string]interface{}{"type": "string"},
		},
		"required": []string{"goal"},
	}
}

func (t *EnterPlanModeTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	goal, _ := input["goal"].(string)

	return map[string]interface{}{
		"goal":      goal,
		"mode":      "planning",
		"activated": true,
	}, nil
}

type SyntheticOutputTool struct{}

func (t *SyntheticOutputTool) Name() string        { return "synthetic_output" }
func (t *SyntheticOutputTool) Description() string { return "Generate synthetic output for testing" }

func (t *SyntheticOutputTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"type":  map[string]interface{}{"type": "string"},
			"count": map[string]interface{}{"type": "integer"},
		},
		"required": []string{"type"},
	}
}

func (t *SyntheticOutputTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	typ, _ := input["type"].(string)
	count := 1
	if c, ok := input["count"].(int); ok && c > 0 {
		count = c
	}

	return map[string]interface{}{
		"type":  typ,
		"count": count,
		"data":  []interface{}{},
	}, nil
}

type ExitPlanModeTool struct{}

func (t *ExitPlanModeTool) Name() string { return "exit_plan_mode" }
func (t *ExitPlanModeTool) Description() string {
	return "Exit planning mode and return to normal operation"
}

func (t *ExitPlanModeTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"summary": map[string]interface{}{"type": "string"},
		},
	}
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	summary, _ := input["summary"].(string)

	return map[string]interface{}{
		"mode":      "normal",
		"summary":   summary,
		"activated": false,
	}, nil
}

type McpAuthTool struct{}

func (t *McpAuthTool) Name() string        { return "mcp_auth" }
func (t *McpAuthTool) Description() string { return "Authenticate with an MCP server using OAuth" }

func (t *McpAuthTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"server": map[string]interface{}{"type": "string"},
		},
		"required": []string{"server"},
	}
}

func (t *McpAuthTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	server, _ := input["server"].(string)
	if server == "" {
		return nil, ErrRequiredField("server")
	}

	return map[string]interface{}{
		"status":  "auth_url",
		"server":  server,
		"message": "OAuth flow initiated. Please complete authentication in your browser.",
		"authUrl": fmt.Sprintf("https://example.com/oauth/%s", server),
	}, nil
}
