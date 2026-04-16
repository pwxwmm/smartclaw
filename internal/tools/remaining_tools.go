package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PowerShellTool struct{ BaseTool }

func newPowerShellTool() *PowerShellTool {
	return &PowerShellTool{BaseTool: NewBaseTool("powershell", "Execute PowerShell command", map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{"type": "string"},
			"timeout": map[string]any{"type": "integer"},
		},
		"required": []string{"command"},
	})}
}

func (t *PowerShellTool) Execute(ctx context.Context, input map[string]any) (any, error) {
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

	if validationResult := ValidateCommandSecurity(cmdStr); !validationResult.Allowed {
		return nil, fmt.Errorf("command rejected by security policy: %s", validationResult.Reason)
	}

	cmd := exec.CommandContext(ctx, "powershell", "-Command", cmdStr)
	output, err := cmd.CombinedOutput()

	return map[string]any{
		"output":   string(output),
		"exitCode": cmd.ProcessState.ExitCode(),
	}, err
}

type ScheduleCronTool struct{ BaseTool }

func (t *ScheduleCronTool) Name() string { return "schedule_cron" }
func (t *ScheduleCronTool) Description() string {
	return "Schedule, list, or delete cron jobs. Actions: schedule, list, delete"
}

func (t *ScheduleCronTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action":   map[string]any{"type": "string", "description": "Action: schedule, list, delete", "default": "schedule"},
			"schedule": map[string]any{"type": "string", "description": "Cron schedule expression (e.g. '*/5 * * * *')"},
			"command":  map[string]any{"type": "string", "description": "Command or instruction to run"},
			"task_id":  map[string]any{"type": "string", "description": "Task ID for delete action"},
		},
		"required": []string{"action"},
	}
}

func (t *ScheduleCronTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	action, _ := input["action"].(string)
	if action == "" {
		action = "schedule"
	}

	cronDir, err := getCronDir()
	if err != nil {
		return nil, fmt.Errorf("failed to access cron directory: %w", err)
	}

	switch action {
	case "schedule":
		return t.scheduleCron(input, cronDir)
	case "list":
		return t.listCrons(cronDir)
	case "delete":
		return t.deleteCron(input, cronDir)
	default:
		return nil, fmt.Errorf("unknown action: %s (valid: schedule, list, delete)", action)
	}
}

func (t *ScheduleCronTool) scheduleCron(input map[string]any, cronDir string) (any, error) {
	schedule, _ := input["schedule"].(string)
	command, _ := input["command"].(string)
	if schedule == "" {
		return nil, ErrRequiredField("schedule")
	}
	if command == "" {
		return nil, ErrRequiredField("command")
	}

	if err := os.MkdirAll(cronDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cron directory: %w", err)
	}

	taskID := fmt.Sprintf("cron_%d", time.Now().UnixNano())
	task := map[string]any{
		"id":          taskID,
		"schedule":    schedule,
		"instruction": command,
		"enabled":     true,
		"created_at":  time.Now().Format(time.RFC3339),
	}

	taskJSON, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cron task: %w", err)
	}

	path := filepath.Join(cronDir, taskID+".json")
	if err := os.WriteFile(path, taskJSON, 0644); err != nil {
		return nil, fmt.Errorf("failed to write cron task: %w", err)
	}

	return map[string]any{
		"id":       taskID,
		"schedule": schedule,
		"command":  command,
		"status":   "scheduled",
		"path":     path,
	}, nil
}

func (t *ScheduleCronTool) listCrons(cronDir string) (any, error) {
	entries, err := os.ReadDir(cronDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{"tasks": []any{}, "count": 0}, nil
		}
		return nil, fmt.Errorf("failed to read cron directory: %w", err)
	}

	var tasks []map[string]any
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cronDir, entry.Name()))
		if err != nil {
			continue
		}
		var task map[string]any
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return map[string]any{
		"tasks": tasks,
		"count": len(tasks),
	}, nil
}

func (t *ScheduleCronTool) deleteCron(input map[string]any, cronDir string) (any, error) {
	taskID, _ := input["task_id"].(string)
	if taskID == "" {
		return nil, ErrRequiredField("task_id")
	}

	path := filepath.Join(cronDir, taskID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to delete cron task: %w", err)
	}

	return map[string]any{
		"id":     taskID,
		"status": "deleted",
	}, nil
}

func getCronDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".smartclaw", "cron"), nil
}

type RemoteTriggerTool struct{ BaseTool }

func (t *RemoteTriggerTool) Name() string { return "remote_trigger" }
func (t *RemoteTriggerTool) Description() string {
	return "Execute a command on a remote host via SSH. Requires SSH key or ssh-agent setup."
}

func (t *RemoteTriggerTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host":    map[string]any{"type": "string", "description": "Remote host (user@hostname or hostname)"},
			"command": map[string]any{"type": "string", "description": "Command to execute on the remote host"},
			"timeout": map[string]any{"type": "integer", "default": 30, "description": "SSH connection timeout in seconds"},
			"port":    map[string]any{"type": "integer", "default": 22, "description": "SSH port"},
		},
		"required": []string{"host", "command"},
	}
}

func (t *RemoteTriggerTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	host, _ := input["host"].(string)
	command, _ := input["command"].(string)
	if host == "" {
		return nil, ErrRequiredField("host")
	}
	if command == "" {
		return nil, ErrRequiredField("command")
	}

	timeout := 30
	if ts, ok := input["timeout"].(int); ok && ts > 0 {
		timeout = ts
	}

	port := "22"
	if p, ok := input["port"].(int); ok && p > 0 {
		port = fmt.Sprintf("%d", p)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	args := []string{"-o", "StrictHostKeyChecking=accept-new",
		"-o", fmt.Sprintf("ConnectTimeout=%d", timeout),
		"-p", port, host, command}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	timedOut := ctx.Err() == context.DeadlineExceeded

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return map[string]any{
		"host":      host,
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
		"timed_out": timedOut,
	}, nil
}

type SendMessageTool struct{ BaseTool }

func (t *SendMessageTool) Name() string { return "send_message" }
func (t *SendMessageTool) Description() string {
	return "Send a message to a channel or user. Supports gateway delivery platforms (telegram, web, terminal)."
}

func (t *SendMessageTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"channel":  map[string]any{"type": "string", "description": "Target channel or user ID"},
			"message":  map[string]any{"type": "string", "description": "Message content to send"},
			"platform": map[string]any{"type": "string", "default": "terminal", "description": "Delivery platform: terminal, web, telegram"},
		},
		"required": []string{"channel", "message"},
	}
}

func (t *SendMessageTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	channel, _ := input["channel"].(string)
	message, _ := input["message"].(string)
	platform, _ := input["platform"].(string)

	if channel == "" {
		return nil, ErrRequiredField("channel")
	}
	if message == "" {
		return nil, ErrRequiredField("message")
	}
	if platform == "" {
		platform = "terminal"
	}

	home, _ := os.UserHomeDir()
	outboxDir := filepath.Join(home, ".smartclaw", "outbox")
	os.MkdirAll(outboxDir, 0755)

	outboxEntry := map[string]any{
		"channel":   channel,
		"message":   message,
		"platform":  platform,
		"timestamp": time.Now().Format(time.RFC3339),
		"status":    "queued",
	}

	data, _ := json.MarshalIndent(outboxEntry, "", "  ")
	filename := fmt.Sprintf("msg_%d.json", time.Now().UnixNano())
	outPath := filepath.Join(outboxDir, filename)

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return map[string]any{
			"channel":  channel,
			"platform": platform,
			"status":   "failed",
			"error":    err.Error(),
		}, nil
	}

	return map[string]any{
		"channel":  channel,
		"platform": platform,
		"status":   "queued",
		"path":     outPath,
	}, nil
}

type EnterWorktreeTool struct{ BaseTool }

func (t *EnterWorktreeTool) Name() string { return "enter_worktree" }
func (t *EnterWorktreeTool) Description() string {
	return "Create and enter a git worktree. Creates a new working directory linked to a branch."
}

func (t *EnterWorktreeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Path for the new worktree"},
			"branch":  map[string]any{"type": "string", "description": "Branch name (creates new branch if -b prefix used)"},
			"workdir": map[string]any{"type": "string", "description": "Working directory of the git repo"},
		},
		"required": []string{"path"},
	}
}

func (t *EnterWorktreeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return nil, ErrRequiredField("path")
	}

	workdir, _ := input["workdir"].(string)
	if workdir == "" {
		workdir = "."
	}

	args := []string{"worktree", "add"}
	if branch, ok := input["branch"].(string); ok && branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, path)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree add failed: %s", strings.TrimSpace(string(output)))
	}

	absPath, _ := filepath.Abs(path)
	return map[string]any{
		"path":    absPath,
		"status":  "created",
		"message": strings.TrimSpace(string(output)),
	}, nil
}

type ExitWorktreeTool struct{ BaseTool }

func (t *ExitWorktreeTool) Name() string { return "exit_worktree" }
func (t *ExitWorktreeTool) Description() string {
	return "Remove a git worktree and clean up"
}

func (t *ExitWorktreeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Path of the worktree to remove"},
			"workdir": map[string]any{"type": "string", "description": "Working directory of the main git repo"},
			"force":   map[string]any{"type": "boolean", "description": "Force removal even with uncommitted changes"},
		},
	}
}

func (t *ExitWorktreeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	workdir, _ := input["workdir"].(string)
	if workdir == "" {
		workdir = "."
	}

	args := []string{"worktree", "remove"}
	if force, ok := input["force"].(bool); ok && force {
		args = append(args, "--force")
	}
	if path != "" {
		args = append(args, path)
	} else {
		return nil, ErrRequiredField("path")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree remove failed: %s", strings.TrimSpace(string(output)))
	}

	return map[string]any{
		"path":    path,
		"status":  "removed",
		"message": strings.TrimSpace(string(output)),
	}, nil
}

type BriefTool struct{ BaseTool }

func (t *BriefTool) Name() string { return "brief" }
func (t *BriefTool) Description() string {
	return "Summarize a topic or text concisely. Uses the LLM to generate a brief."
}

func (t *BriefTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"topic":  map[string]any{"type": "string", "description": "Topic or text to summarize"},
			"length": map[string]any{"type": "string", "default": "short", "description": "Summary length: short (1-2 sentences), medium (1 paragraph), long (multi-paragraph)"},
		},
		"required": []string{"topic"},
	}
}

func (t *BriefTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	topic, _ := input["topic"].(string)
	if topic == "" {
		return nil, ErrRequiredField("topic")
	}

	length, _ := input["length"].(string)
	if length == "" {
		length = "short"
	}

	lengthGuide := "1-2 sentences"
	switch length {
	case "medium":
		lengthGuide = "one paragraph"
	case "long":
		lengthGuide = "2-3 paragraphs"
	}

	prompt := fmt.Sprintf("Provide a brief, factual summary of the following topic in %s. Be concise and informative.\n\nTopic: %s", lengthGuide, topic)

	return map[string]any{
		"topic":  topic,
		"prompt": prompt,
		"length": length,
		"note":   "Returns a summarization prompt that should be sent to the LLM. The runtime will auto-route this.",
	}, nil
}

type SleepTool struct{ BaseTool }

func (t *SleepTool) Name() string        { return "sleep" }
func (t *SleepTool) Description() string { return "Sleep for specified duration" }

func (t *SleepTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"seconds": map[string]any{"type": "number"},
		},
		"required": []string{"seconds"},
	}
}

func (t *SleepTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	seconds, _ := input["seconds"].(float64)
	if seconds > 0 {
		time.Sleep(time.Duration(seconds) * time.Second)
	}
	return map[string]any{
		"slept": seconds,
	}, nil
}

type ToolSearchTool struct{ BaseTool }

func (t *ToolSearchTool) Name() string        { return "tool_search" }
func (t *ToolSearchTool) Description() string { return "Search for deferred tools by keyword" }

func (t *ToolSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":       map[string]any{"type": "string"},
			"max_results": map[string]any{"type": "integer", "default": 5},
		},
		"required": []string{"query"},
	}
}

func (t *ToolSearchTool) Execute(ctx context.Context, input map[string]any) (any, error) {
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

	return map[string]any{
		"matches":              matched,
		"query":                query,
		"total_deferred_tools": len(deferredTools),
	}, nil
}

type REPLTool struct{ BaseTool }

func (t *REPLTool) Name() string { return "repl" }
func (t *REPLTool) Description() string {
	return "Evaluate an expression in a sandboxed REPL (JavaScript via Node.js or Python)"
}

func (t *REPLTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"expression": map[string]any{"type": "string", "description": "Expression to evaluate"},
			"language":   map[string]any{"type": "string", "default": "javascript", "description": "Language: javascript or python"},
			"timeout":    map[string]any{"type": "integer", "default": 10000, "description": "Timeout in milliseconds"},
		},
		"required": []string{"expression"},
	}
}

func (t *REPLTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	expr, _ := input["expression"].(string)
	if expr == "" {
		return nil, ErrRequiredField("expression")
	}

	lang, _ := input["language"].(string)
	if lang == "" {
		lang = "javascript"
	}

	timeout := 10000
	if t, ok := input["timeout"].(int); ok && t > 0 {
		timeout = t
	}
	if timeout > 30000 {
		timeout = 30000
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	var cmd *exec.Cmd
	switch lang {
	case "javascript", "js", "node":
		cmd = exec.CommandContext(ctx, "node", "-e", expr)
	case "python", "py", "python3":
		cmd = exec.CommandContext(ctx, "python3", "-c", expr)
	default:
		return nil, fmt.Errorf("unsupported language: %s (supported: javascript, python)", lang)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	timedOut := false

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
		if ctx.Err() == context.DeadlineExceeded {
			timedOut = true
		}
	}

	return map[string]any{
		"expression": expr,
		"language":   lang,
		"result":     strings.TrimSpace(stdout.String()),
		"error":      strings.TrimSpace(stderr.String()),
		"exitCode":   exitCode,
		"timedOut":   timedOut,
	}, nil
}

type EnterPlanModeTool struct{ BaseTool }

func (t *EnterPlanModeTool) Name() string { return "enter_plan_mode" }
func (t *EnterPlanModeTool) Description() string {
	return "Enter planning mode for structured task breakdown"
}

func (t *EnterPlanModeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"goal": map[string]any{"type": "string"},
		},
		"required": []string{"goal"},
	}
}

func (t *EnterPlanModeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	goal, _ := input["goal"].(string)

	return map[string]any{
		"goal":      goal,
		"mode":      "planning",
		"activated": true,
	}, nil
}

type SyntheticOutputTool struct{ BaseTool }

func (t *SyntheticOutputTool) Name() string        { return "synthetic_output" }
func (t *SyntheticOutputTool) Description() string { return "Generate synthetic output for testing" }

func (t *SyntheticOutputTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":  map[string]any{"type": "string"},
			"count": map[string]any{"type": "integer"},
		},
		"required": []string{"type"},
	}
}

func (t *SyntheticOutputTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	typ, _ := input["type"].(string)
	count := 1
	if c, ok := input["count"].(int); ok && c > 0 {
		count = c
	}

	return map[string]any{
		"type":  typ,
		"count": count,
		"data":  []any{},
	}, nil
}

type ExitPlanModeTool struct{ BaseTool }

func (t *ExitPlanModeTool) Name() string { return "exit_plan_mode" }
func (t *ExitPlanModeTool) Description() string {
	return "Exit planning mode and return to normal operation"
}

func (t *ExitPlanModeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
		},
	}
}

func (t *ExitPlanModeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	summary, _ := input["summary"].(string)

	return map[string]any{
		"mode":      "normal",
		"summary":   summary,
		"activated": false,
	}, nil
}
