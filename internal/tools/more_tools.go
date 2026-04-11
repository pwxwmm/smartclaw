package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/instructkr/smartclaw/internal/logger"
)

// Global todo manager instance
var (
	todoManager     *TodoManager
	todoManagerOnce sync.Once
	todoManagerErr  error
)

func getTodoManager() *TodoManager {
	todoManagerOnce.Do(func() {
		todoManager, todoManagerErr = NewTodoManager()
	})
	if todoManagerErr != nil {
		// Return a simple in-memory manager if persistence fails
		return &TodoManager{
			todos: make(map[string]*TodoList),
		}
	}
	return todoManager
}

// TodoWriteTool manages todo lists for task tracking
type TodoWriteTool struct {
	sessionID string
}

// NewTodoWriteTool creates a new TodoWriteTool with session context
func NewTodoWriteTool(sessionID string) *TodoWriteTool {
	return &TodoWriteTool{sessionID: sessionID}
}

func (t *TodoWriteTool) Name() string { return "todowrite" }
func (t *TodoWriteTool) Description() string {
	return "Manage the session task checklist. Use this to track progress and plan work."
}

func (t *TodoWriteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todos": map[string]any{
				"type":        "array",
				"description": "The updated todo list",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content": map[string]any{
							"type":        "string",
							"description": "Brief description of the task",
						},
						"status": map[string]any{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed", "cancelled"},
							"description": "Current status of the todo",
						},
						"priority": map[string]any{
							"type":        "string",
							"enum":        []string{"high", "medium", "low"},
							"description": "Priority level",
						},
					},
					"required": []string{"content", "status"},
				},
			},
		},
		"required": []string{"todos"},
	}
}

func (t *TodoWriteTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	todosRaw, ok := input["todos"].([]any)
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "todos must be an array"}
	}

	// Parse todos
	todos := make([]TodoItem, 0, len(todosRaw))
	for _, item := range todosRaw {
		if itemMap, ok := item.(map[string]any); ok {
			todo := TodoItem{}
			if content, ok := itemMap["content"].(string); ok {
				todo.Content = content
			}
			if status, ok := itemMap["status"].(string); ok {
				todo.Status = status
			} else {
				todo.Status = "pending"
			}
			if priority, ok := itemMap["priority"].(string); ok {
				todo.Priority = priority
			} else {
				todo.Priority = "medium"
			}
			todos = append(todos, todo)
		}
	}

	// Get session ID (use default if not set)
	sessionID := t.sessionID
	if sessionID == "" {
		sessionID = "default"
	}

	// Get todo manager
	manager := getTodoManager()

	// Get old todos for comparison
	oldTodos := manager.GetOldTodos(sessionID)

	// Update todos
	if err := manager.Set(sessionID, todos); err != nil {
		return nil, &Error{Code: "PERSIST_ERROR", Message: err.Error()}
	}

	// Check for verification nudge
	verificationNudge := manager.CheckVerificationNudge(sessionID, todos)

	// Prepare response
	response := map[string]any{
		"success":            true,
		"old_todos":          oldTodos,
		"new_todos":          todos,
		"count":              len(todos),
		"verification_nudge": verificationNudge,
	}

	// Add nudge message if needed
	if verificationNudge {
		response["message"] = "Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current task if applicable.\n\nNOTE: You just closed out 3+ tasks and none of them was a verification step. Before writing your final summary, spawn the verification agent (subagent_type=\"verification\"). You cannot self-assign PARTIAL by listing caveats in your summary — only the verifier issues a verdict."
	} else {
		response["message"] = "Todos have been modified successfully. Ensure that you continue to use the todo list to track your progress. Please proceed with the current task if applicable."
	}

	return response, nil
}

// AskUserQuestionTool asks the user questions during execution
type AskUserQuestionTool struct{}

func (t *AskUserQuestionTool) Name() string { return "ask_user" }
func (t *AskUserQuestionTool) Description() string {
	return "Ask the user questions to gather information, clarify ambiguity, or get decisions"
}

func (t *AskUserQuestionTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"questions": map[string]any{
				"type":        "array",
				"description": "Questions to ask the user",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"question": map[string]any{
							"type":        "string",
							"description": "The question to ask",
						},
						"header": map[string]any{
							"type":        "string",
							"description": "Short header for the question (max 30 chars)",
						},
						"options": map[string]any{
							"type":        "array",
							"description": "Available choices",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"label": map[string]any{
										"type":        "string",
										"description": "Display text (1-5 words)",
									},
									"description": map[string]any{
										"type":        "string",
										"description": "Explanation of choice",
									},
								},
								"required": []string{"label"},
							},
						},
						"multiple": map[string]any{
							"type":        "boolean",
							"description": "Allow selecting multiple choices",
						},
					},
					"required": []string{"question"},
				},
			},
		},
		"required": []string{"questions"},
	}
}

func (t *AskUserQuestionTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	questionsRaw, ok := input["questions"].([]any)
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "questions must be an array"}
	}

	// In non-interactive mode, return pending status
	// In interactive mode, this would prompt the user via CLI
	// For now, we return a structured response that the runtime can handle

	questions := make([]map[string]any, 0, len(questionsRaw))
	for _, q := range questionsRaw {
		if qMap, ok := q.(map[string]any); ok {
			questions = append(questions, qMap)
		}
	}

	return map[string]any{
		"status":         "pending_user_response",
		"questions":      questions,
		"question_count": len(questions),
		"message":        "User interaction required. Questions prepared for display.",
		"instructions":   "Present these questions to the user and collect responses before continuing.",
	}, nil
}

// ConfigTool manages configuration settings
type ConfigTool struct {
	configDir string
}

// NewConfigTool creates a new ConfigTool
func NewConfigTool() *ConfigTool {
	home, _ := os.UserHomeDir()
	return &ConfigTool{
		configDir: filepath.Join(home, ".smartclaw"),
	}
}

func (t *ConfigTool) Name() string { return "config" }
func (t *ConfigTool) Description() string {
	return "Get or set Claude Code settings"
}

func (t *ConfigTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"get", "set", "list"},
				"description": "Operation to perform",
			},
			"key": map[string]any{
				"type":        "string",
				"description": "Configuration key (e.g., 'model', 'theme')",
			},
			"value": map[string]any{
				"description": "New value for set operation",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *ConfigTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	operation, _ := input["operation"].(string)

	switch operation {
	case "get":
		return t.get(input)
	case "set":
		return t.set(input)
	case "list":
		return t.list()
	default:
		return nil, &Error{Code: "INVALID_OPERATION", Message: "unknown operation: " + operation}
	}
}

func (t *ConfigTool) get(input map[string]any) (any, error) {
	key, _ := input["key"].(string)
	if key == "" {
		return nil, &Error{Code: "MISSING_KEY", Message: "key is required for get operation"}
	}

	config := t.loadConfig()
	value, exists := config[key]
	if !exists {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Unknown setting: %s", key),
		}, nil
	}

	return map[string]any{
		"success": true,
		"key":     key,
		"value":   value,
	}, nil
}

func (t *ConfigTool) set(input map[string]any) (any, error) {
	key, _ := input["key"].(string)
	if key == "" {
		return nil, &Error{Code: "MISSING_KEY", Message: "key is required for set operation"}
	}

	value := input["value"]

	config := t.loadConfig()
	previousValue := config[key]
	config[key] = value

	if err := t.saveConfig(config); err != nil {
		return nil, &Error{Code: "SAVE_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"success":        true,
		"key":            key,
		"previous_value": previousValue,
		"new_value":      value,
	}, nil
}

func (t *ConfigTool) list() (any, error) {
	config := t.loadConfig()

	settings := make([]map[string]any, 0, len(config))
	for k, v := range config {
		settings = append(settings, map[string]any{
			"key":   k,
			"value": v,
		})
	}

	return map[string]any{
		"success":  true,
		"settings": settings,
		"count":    len(settings),
	}, nil
}

func (t *ConfigTool) loadConfig() map[string]any {
	configPath := filepath.Join(t.configDir, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return map[string]any{
			"model":      "claude-sonnet-4-5",
			"permission": "ask",
			"log_level":  "info",
		}
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return map[string]any{}
	}

	return config
}

func (t *ConfigTool) saveConfig(config map[string]any) error {
	if err := os.MkdirAll(t.configDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(t.configDir, "config.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// SkillTool loads and executes skills
type SkillTool struct {
	skillsDir string
}

// NewSkillTool creates a new SkillTool
func NewSkillTool() *SkillTool {
	home, _ := os.UserHomeDir()
	return &SkillTool{
		skillsDir: filepath.Join(home, ".smartclaw", "skills"),
	}
}

func (t *SkillTool) Name() string { return "skill" }
func (t *SkillTool) Description() string {
	return "Load a skill or slash command to get specialized instructions"
}

func (t *SkillTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "The skill name (e.g., 'commit', 'review-pr', 'git-master')",
			},
			"user_message": map[string]any{
				"type":        "string",
				"description": "Optional context or arguments for the skill",
			},
		},
		"required": []string{"name"},
	}
}

func (t *SkillTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return nil, ErrRequiredField("name")
	}

	userMessage, _ := input["user_message"].(string)

	// Strip leading slash if present
	name = strings.TrimPrefix(name, "/")

	// Try to load skill from various locations
	skillContent, skillPath, err := t.loadSkill(name)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Skill not found: %s", name),
		}, nil
	}

	return map[string]any{
		"success":      true,
		"skill_name":   name,
		"skill_path":   skillPath,
		"content":      skillContent,
		"user_message": userMessage,
		"message":      fmt.Sprintf("Skill '%s' loaded successfully", name),
	}, nil
}

func (t *SkillTool) loadSkill(name string) (string, string, error) {
	// Search locations for skills
	searchPaths := []string{
		filepath.Join(t.skillsDir, name, "SKILL.md"),
		filepath.Join(t.skillsDir, name+".md"),
		filepath.Join(".", ".claude", "skills", name, "SKILL.md"),
		filepath.Join(".", ".claude", "skills", name+".md"),
		filepath.Join(".", name+".skill.md"),
	}

	for _, path := range searchPaths {
		if content, err := os.ReadFile(path); err == nil {
			return string(content), path, nil
		}
	}

	// Check for bundled skills
	bundledSkill := t.getBundledSkill(name)
	if bundledSkill != "" {
		return bundledSkill, "bundled:" + name, nil
	}

	return "", "", fmt.Errorf("skill not found: %s", name)
}

func (t *SkillTool) getBundledSkill(name string) string {
	// Bundled skills that are always available
	bundledSkills := map[string]string{
		"help": "# Help Skill\n\nThis skill provides guidance on using Claude Code.\n\nAvailable features:\n- File operations (read, write, edit, glob, grep)\n- Code analysis (LSP, AST grep)\n- Web tools (fetch, search)\n- Agent spawning for parallel tasks\n- Session management\n- MCP protocol integration\n\nUse /help to see all available commands.\n",

		"commit": "# Commit Skill\n\nThis skill helps create well-structured git commits.\n\n## Guidelines\n\n1. **Atomic commits**: One logical change per commit\n2. **Clear messages**: Describe what and why, not how\n3. **Conventional commits**: Use prefixes (feat, fix, docs, style, refactor, test, chore)\n\n## Commit Message Format\n\n<type>(<scope>): <subject>\n\n<body>\n\n<footer>\n\n## Types\n- feat: New feature\n- fix: Bug fix\n- docs: Documentation\n- style: Formatting\n- refactor: Code restructuring\n- test: Adding tests\n- chore: Maintenance\n\n## Example\n\nfeat(auth): add OAuth2 support\n\nImplement OAuth2 authentication flow with PKCE challenge.\nSupports Google, GitHub, and custom providers.\n\nCloses #123\n",

		"git-master": "# Git Master Skill\n\nExpert-level git operations and workflows.\n\n## Best Practices\n\n1. **Atomic commits**: One logical change per commit\n2. **Meaningful messages**: Clear, descriptive commit messages\n3. **Branch hygiene**: Delete merged branches\n4. **Rebase over merge**: Keep history linear when possible\n5. **Sign commits**: Use GPG signing for security\n\n## Common Workflows\n\n### Feature Development\n\ngit checkout -b feature/my-feature\n# Make changes\ngit add -p\ngit commit -m \"feat: add new feature\"\ngit push -u origin feature/my-feature\n# Create PR\n\n### Bug Fix\n\ngit checkout -b fix/bug-description\n# Fix bug\ngit add .\ngit commit -m \"fix: correct issue with X\"\ngit push -u origin fix/bug-description\n# Create PR\n\n### Hotfix\n\ngit checkout main\ngit pull\ngit checkout -b hotfix/critical-fix\n# Apply fix\ngit commit -m \"fix: critical security issue\"\ngit push -u origin hotfix/critical-fix\n# Create PR and merge immediately\n\n## Useful Commands\n\n- Interactive rebase: git rebase -i HEAD~3\n- Cherry pick: git cherry-pick <commit>\n- Stash: git stash push -m \"message\"\n- Clean branches: git branch --merged | grep -v \"*\" | xargs -n 1 git branch -d\n",
	}

	return bundledSkills[name]
}

// NotebookEditTool edits Jupyter notebook cells
type NotebookEditTool struct{}

func (t *NotebookEditTool) Name() string { return "notebook_edit" }
func (t *NotebookEditTool) Description() string {
	return "Edit Jupyter notebook cells"
}

func (t *NotebookEditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the notebook file",
			},
			"cell_number": map[string]any{
				"type":        "integer",
				"description": "Cell number to edit (0-indexed)",
			},
			"source": map[string]any{
				"type":        "string",
				"description": "New cell source code",
			},
			"cell_type": map[string]any{
				"type":        "string",
				"enum":        []string{"code", "markdown"},
				"description": "Type of cell",
			},
		},
		"required": []string{"path", "cell_number"},
	}
}

func (t *NotebookEditTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return nil, ErrRequiredField("path")
	}

	cellNumber, ok := input["cell_number"].(int)
	if !ok {
		// Try float64 (JSON number)
		if f, ok := input["cell_number"].(float64); ok {
			cellNumber = int(f)
		} else {
			return nil, ErrRequiredField("cell_number")
		}
	}

	source, _ := input["source"].(string)
	cellType, _ := input["cell_type"].(string)
	if cellType == "" {
		cellType = "code"
	}

	// Read notebook
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	var notebook map[string]any
	if err := json.Unmarshal(data, &notebook); err != nil {
		return nil, &Error{Code: "PARSE_ERROR", Message: "invalid notebook format"}
	}

	cells, ok := notebook["cells"].([]any)
	if !ok {
		return nil, &Error{Code: "INVALID_NOTEBOOK", Message: "notebook has no cells"}
	}

	if cellNumber < 0 || cellNumber >= len(cells) {
		return nil, &Error{Code: "INVALID_CELL", Message: fmt.Sprintf("cell %d does not exist", cellNumber)}
	}

	// Update cell
	cell, ok := cells[cellNumber].(map[string]any)
	if !ok {
		return nil, &Error{Code: "INVALID_CELL", Message: "invalid cell format"}
	}

	if source != "" {
		cell["source"] = strings.Split(source, "\n")
	}
	if cellType != "" {
		cell["cell_type"] = cellType
	}

	// Write notebook back
	data, err = json.MarshalIndent(notebook, "", "  ")
	if err != nil {
		return nil, &Error{Code: "ENCODE_ERROR", Message: err.Error()}
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, &Error{Code: "WRITE_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"success":     true,
		"path":        path,
		"cell_number": cellNumber,
		"message":     fmt.Sprintf("Cell %d updated successfully", cellNumber),
	}, nil
}

// InteractivePrompt reads input from user
func InteractivePrompt(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

type BrowseTool struct{}

func (t *BrowseTool) Name() string { return "browse" }
func (t *BrowseTool) Description() string {
	return "Open URL in a headless browser. Delegates to browser_navigate for actual page loading."
}

func (t *BrowseTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "URL to navigate to"},
		},
		"required": []string{"url"},
	}
}

func (t *BrowseTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	url, _ := input["url"].(string)
	if url == "" {
		return nil, ErrRequiredField("url")
	}

	nav := &BrowserNavigateTool{}
	result, err := nav.Execute(ctx, map[string]any{"url": url})
	if err != nil {
		return map[string]any{
			"url":    url,
			"status": "fallback",
			"note":   "Browser not available; URL prepared for manual opening",
		}, nil
	}
	return result, nil
}

type AttachTool struct{}

func (t *AttachTool) Name() string { return "attach" }
func (t *AttachTool) Description() string {
	return "List running processes or inspect a process. Use action=list to see processes, action=inspect for details."
}

func (t *AttachTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pid":    map[string]any{"type": "string", "description": "Process ID to inspect"},
			"action": map[string]any{"type": "string", "default": "list", "description": "Action: list or inspect"},
		},
	}
}

func (t *AttachTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	action, _ := input["action"].(string)
	if action == "" {
		action = "list"
	}

	switch action {
	case "list":
		cmd := exec.CommandContext(ctx, "ps", "aux")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("ps command failed: %w", err)
		}
		lines := strings.Split(string(output), "\n")
		if len(lines) > 50 {
			lines = append(lines[:1], lines[1:50]...)
			lines = append(lines, "... (truncated)")
		}
		return map[string]any{
			"processes": strings.Join(lines, "\n"),
			"action":    "list",
		}, nil

	case "inspect":
		pid, _ := input["pid"].(string)
		if pid == "" {
			return nil, ErrRequiredField("pid")
		}
		cmd := exec.CommandContext(ctx, "ps", "-p", pid, "-o", "pid,ppid,user,%cpu,%mem,etime,command")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("process %s not found: %w", pid, err)
		}
		return map[string]any{
			"pid":     pid,
			"details": strings.TrimSpace(string(output)),
			"action":  "inspect",
		}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s (valid: list, inspect)", action)
	}
}

type DebugTool struct{}

func (t *DebugTool) Name() string { return "debug" }
func (t *DebugTool) Description() string {
	return "Toggle debug logging level. When enabled, sets log level to debug; when disabled, reverts to info."
}

func (t *DebugTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enable": map[string]any{"type": "boolean"},
		},
	}
}

func (t *DebugTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	enable, _ := input["enable"].(bool)

	logLevel := "info"
	if enable {
		logLevel = "debug"
		os.Setenv("SMARTCLAW_LOG_LEVEL", "debug")
		logger.SetLevel(logger.LevelDebug)
	} else {
		os.Setenv("SMARTCLAW_LOG_LEVEL", "info")
		logger.SetLevel(logger.LevelInfo)
	}

	return map[string]any{
		"debug":     enable,
		"log_level": logLevel,
	}, nil
}

type IndexTool struct{}

func (t *IndexTool) Name() string { return "index" }
func (t *IndexTool) Description() string {
	return "Build a code index of symbols (functions, types, variables) in a directory for fast lookup"
}

func (t *IndexTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "Directory to index"},
			"exclude": map[string]any{"type": "array", "description": "Patterns to exclude"},
		},
	}
}

func (t *IndexTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}

	symbolPattern := regexp.MustCompile(`(?m)^(func |type |var |const |def |class |async def |fn |pub fn |let |interface |struct |enum |trait |impl |module )`)

	var symbols []map[string]any
	maxSymbols := 200

	filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil || len(symbols) >= maxSymbols {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "dist" || name == "build" || name == "bin" || name == "__pycache__" {
				return fs.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(filePath)
		supported := map[string]bool{".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".rs": true, ".java": true, ".rb": true}
		if !supported[ext] {
			return nil
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if len(symbols) >= maxSymbols {
				break
			}
			if symbolPattern.MatchString(line) {
				symbols = append(symbols, map[string]any{
					"file": filePath,
					"line": i + 1,
					"text": strings.TrimSpace(line),
				})
			}
		}
		return nil
	})

	return map[string]any{
		"path":    path,
		"symbols": symbols,
		"count":   len(symbols),
	}, nil
}

type CacheTool struct{}

func (t *CacheTool) Name() string { return "cache" }
func (t *CacheTool) Description() string {
	return "Manage tool result cache. Actions: get, set, clear, stats"
}

func (t *CacheTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{"type": "string", "enum": []string{"get", "set", "clear", "stats"}, "description": "Cache operation"},
			"key":    map[string]any{"type": "string", "description": "Cache key"},
			"value":  map[string]any{"type": "string", "description": "Value for set operation"},
		},
	}
}

func (t *CacheTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	action, _ := input["action"].(string)
	key, _ := input["key"].(string)

	switch action {
	case "get":
		if key == "" {
			return nil, ErrRequiredField("key")
		}
		registry := GetRegistry()
		if rc := registry.GetCache(); rc != nil {
			inputMap := map[string]any{"key": key}
			if result, ok := rc.Get("cache_tool", inputMap); ok {
				return map[string]any{"key": key, "found": true, "value": result}, nil
			}
		}
		home, _ := os.UserHomeDir()
		cacheDir := filepath.Join(home, ".smartclaw", "cache")
		data, err := os.ReadFile(filepath.Join(cacheDir, key))
		if err != nil {
			return map[string]any{"key": key, "found": false}, nil
		}
		return map[string]any{"key": key, "found": true, "value": string(data)}, nil

	case "set":
		value, _ := input["value"].(string)
		if key == "" {
			return nil, ErrRequiredField("key")
		}
		registry := GetRegistry()
		if rc := registry.GetCache(); rc != nil {
			rc.Set("cache_tool", map[string]any{"key": key}, value, []string{})
		}
		home, _ := os.UserHomeDir()
		cacheDir := filepath.Join(home, ".smartclaw", "cache")
		os.MkdirAll(cacheDir, 0755)
		if err := os.WriteFile(filepath.Join(cacheDir, key), []byte(value), 0644); err != nil {
			return nil, fmt.Errorf("cache set failed: %w", err)
		}
		return map[string]any{"key": key, "stored": true}, nil

	case "clear":
		registry := GetRegistry()
		if rc := registry.GetCache(); rc != nil {
			rc.Clear()
		}
		home, _ := os.UserHomeDir()
		cacheDir := filepath.Join(home, ".smartclaw", "cache")
		entries, _ := os.ReadDir(cacheDir)
		count := 0
		for _, e := range entries {
			os.Remove(filepath.Join(cacheDir, e.Name()))
			count++
		}
		return map[string]any{"cleared": count, "memory_cache_cleared": true}, nil

	case "stats":
		registry := GetRegistry()
		rc := registry.GetCache()
		home, _ := os.UserHomeDir()
		cacheDir := filepath.Join(home, ".smartclaw", "cache")
		entries, _ := os.ReadDir(cacheDir)
		totalSize := int64(0)
		for _, e := range entries {
			if info, err := e.Info(); err == nil {
				totalSize += info.Size()
			}
		}
		stats := map[string]any{
			"disk_items":     len(entries),
			"disk_size":      totalSize,
			"disk_cache_dir": cacheDir,
		}
		if rc != nil {
			stats["memory_cache_size"] = rc.Size()
		}
		return stats, nil

	default:
		return map[string]any{"key": key, "action": action, "note": "use action: get, set, clear, stats"}, nil
	}
}

type ObserveTool struct{}

func (t *ObserveTool) Name() string { return "observe" }
func (t *ObserveTool) Description() string {
	return "Watch for file changes in a directory. Returns recent file modification events."
}

func (t *ObserveTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":        map[string]any{"type": "string", "description": "Directory or file to watch"},
			"pattern":     map[string]any{"type": "string", "description": "Glob pattern filter (e.g. '*.go')"},
			"debounce_ms": map[string]any{"type": "integer", "default": 500, "description": "Debounce interval in milliseconds"},
		},
	}
}

func (t *ObserveTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}
	pattern, _ := input["pattern"].(string)

	debounceMs := 500
	if d, ok := input["debounce_ms"].(int); ok && d > 0 {
		debounceMs = d
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("file watcher unavailable: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(path); err != nil {
		return nil, fmt.Errorf("cannot watch %s: %w", path, err)
	}

	var events []map[string]any
	timeout := time.After(time.Duration(debounceMs) * time.Millisecond)

WatchLoop:
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				break WatchLoop
			}
			if pattern != "" {
				matched, _ := filepath.Match(pattern, filepath.Base(event.Name))
				if !matched {
					continue
				}
			}
			events = append(events, map[string]any{
				"file": event.Name,
				"op":   event.Op.String(),
				"time": time.Now().Format(time.RFC3339),
			})
		case <-timeout:
			break WatchLoop
		case <-ctx.Done():
			break WatchLoop
		}
	}

	return map[string]any{
		"path":        path,
		"pattern":     pattern,
		"events":      events,
		"event_count": len(events),
	}, nil
}

type LazyTool struct{}

func (t *LazyTool) Name() string { return "lazy" }
func (t *LazyTool) Description() string {
	return "Toggle lazy/batch execution mode. When enabled, tool calls are queued and executed in batch rather than immediately."
}

func (t *LazyTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"enable": map[string]any{"type": "boolean"},
		},
	}
}

func (t *LazyTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	enable, _ := input["enable"].(bool)

	registry := GetRegistry()
	be := registry.GetBatchExecutor()
	if be != nil {
		be.SetLazyMode(enable)
	}

	if enable {
		os.Setenv("SMARTCLAW_LAZY_MODE", "true")
	} else {
		os.Setenv("SMARTCLAW_LAZY_MODE", "false")
		if be != nil && be.QueueSize() > 0 {
			results := be.Flush(ctx, registry)
			return map[string]any{
				"lazy":       false,
				"batch_mode": false,
				"flushed":    len(results),
				"results":    results,
			}, nil
		}
	}

	return map[string]any{
		"lazy":       enable,
		"batch_mode": enable,
	}, nil
}

type ThinkTool struct{}

func (t *ThinkTool) Name() string { return "think" }
func (t *ThinkTool) Description() string {
	return "Enable extended thinking mode for the current session. Sets a flag that the runtime uses to request thinking tokens from the LLM."
}

func (t *ThinkTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{"type": "string", "description": "Optional prompt to focus thinking on"},
			"budget": map[string]any{"type": "integer", "default": 10000, "description": "Token budget for thinking"},
		},
		"required": []string{"prompt"},
	}
}

func (t *ThinkTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	prompt, _ := input["prompt"].(string)
	budget := 10000
	if b, ok := input["budget"].(int); ok && b > 0 {
		budget = b
	}

	os.Setenv("SMARTCLAW_THINKING_ENABLED", "true")
	os.Setenv("SMARTCLAW_THINKING_BUDGET", fmt.Sprintf("%d", budget))

	return map[string]any{
		"thinking_enabled": true,
		"budget":           budget,
		"prompt":           prompt,
	}, nil
}

type DeepThinkTool struct{}

func (t *DeepThinkTool) Name() string { return "deepthink" }
func (t *DeepThinkTool) Description() string {
	return "Enable deep thinking with higher token budget for complex problems. Equivalent to think with budget=50000."
}

func (t *DeepThinkTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{"type": "string", "description": "Problem to think deeply about"},
			"depth":  map[string]any{"type": "integer", "default": 5, "description": "Depth level 1-10"},
		},
		"required": []string{"prompt"},
	}
}

func (t *DeepThinkTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	prompt, _ := input["prompt"].(string)
	depth := 5
	if d, ok := input["depth"].(int); ok && d > 0 {
		depth = d
	}

	budget := 10000 + depth*8000

	os.Setenv("SMARTCLAW_THINKING_ENABLED", "true")
	os.Setenv("SMARTCLAW_THINKING_BUDGET", fmt.Sprintf("%d", budget))

	return map[string]any{
		"thinking_enabled": true,
		"budget":           budget,
		"depth":            depth,
		"prompt":           prompt,
	}, nil
}

type ForkTool struct{}

func (t *ForkTool) Name() string { return "fork" }
func (t *ForkTool) Description() string {
	return "Fork current session into a new branch. Creates a copy of the session state with a new ID."
}

func (t *ForkTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"label": map[string]any{"type": "string", "description": "Label for the forked session"},
		},
	}
}

func (t *ForkTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	label, _ := input["label"].(string)

	home, _ := os.UserHomeDir()
	sessionsDir := filepath.Join(home, ".smartclaw", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	sessionID := fmt.Sprintf("fork_%d", time.Now().UnixNano())

	// Clone current session if it exists
	forkMeta := map[string]any{
		"id":         sessionID,
		"label":      label,
		"forked_at":  time.Now().Format(time.RFC3339),
		"parent_pid": os.Getpid(),
	}

	// Find the most recent session file to clone
	entries, err := os.ReadDir(sessionsDir)
	if err == nil {
		var latestEntry os.DirEntry
		var latestTime time.Time
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			if strings.HasPrefix(e.Name(), "fork_") {
				continue
			}
			info, statErr := e.Info()
			if statErr != nil {
				continue
			}
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestEntry = e
			}
		}

		if latestEntry != nil {
			srcPath := filepath.Join(sessionsDir, latestEntry.Name())
			srcData, readErr := os.ReadFile(srcPath)
			if readErr == nil {
				var srcSession map[string]any
				if json.Unmarshal(srcData, &srcSession) == nil {
					srcSession["id"] = sessionID
					srcSession["title"] = fmt.Sprintf("Fork: %s", label)
					if _, ok := srcSession["messages"]; ok {
						forkMeta["cloned_messages"] = len(srcSession["messages"].([]any))
					}
					dstData, marshalErr := json.MarshalIndent(srcSession, "", "  ")
					if marshalErr == nil {
						dstPath := filepath.Join(sessionsDir, sessionID+".json")
						os.WriteFile(dstPath, dstData, 0644)
						forkMeta["source_session"] = latestEntry.Name()
					}
				}
			}
		}
	}

	data, _ := json.MarshalIndent(forkMeta, "", "  ")
	metaPath := filepath.Join(sessionsDir, sessionID+"_fork_meta.json")
	os.WriteFile(metaPath, data, 0644)

	return map[string]any{
		"session_id":      sessionID,
		"label":           label,
		"meta_path":       metaPath,
		"cloned_session":  forkMeta["source_session"] != nil,
		"cloned_messages": forkMeta["cloned_messages"],
	}, nil
}

type EnvTool struct{}

func (t *EnvTool) Name() string        { return "env" }
func (t *EnvTool) Description() string { return "Environment variable access" }

func (t *EnvTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key":   map[string]any{"type": "string"},
			"value": map[string]any{"type": "string"},
		},
	}
}

func (t *EnvTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	key, _ := input["key"].(string)
	value, _ := input["value"].(string)

	if key != "" && value != "" {
		os.Setenv(key, value)
		return map[string]any{"key": key, "message": "Environment set"}, nil
	}
	if key != "" {
		return map[string]any{"key": key, "value": os.Getenv(key)}, nil
	}
	return map[string]any{"message": "Environment access"}, nil
}
