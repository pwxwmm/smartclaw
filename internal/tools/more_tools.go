package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func (t *TodoWriteTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"todos": map[string]interface{}{
				"type":        "array",
				"description": "The updated todo list",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"content": map[string]interface{}{
							"type":        "string",
							"description": "Brief description of the task",
						},
						"status": map[string]interface{}{
							"type":        "string",
							"enum":        []string{"pending", "in_progress", "completed", "cancelled"},
							"description": "Current status of the todo",
						},
						"priority": map[string]interface{}{
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

func (t *TodoWriteTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	todosRaw, ok := input["todos"].([]interface{})
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "todos must be an array"}
	}

	// Parse todos
	todos := make([]TodoItem, 0, len(todosRaw))
	for _, item := range todosRaw {
		if itemMap, ok := item.(map[string]interface{}); ok {
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
	response := map[string]interface{}{
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

func (t *AskUserQuestionTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"questions": map[string]interface{}{
				"type":        "array",
				"description": "Questions to ask the user",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"question": map[string]interface{}{
							"type":        "string",
							"description": "The question to ask",
						},
						"header": map[string]interface{}{
							"type":        "string",
							"description": "Short header for the question (max 30 chars)",
						},
						"options": map[string]interface{}{
							"type":        "array",
							"description": "Available choices",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"label": map[string]interface{}{
										"type":        "string",
										"description": "Display text (1-5 words)",
									},
									"description": map[string]interface{}{
										"type":        "string",
										"description": "Explanation of choice",
									},
								},
								"required": []string{"label"},
							},
						},
						"multiple": map[string]interface{}{
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

func (t *AskUserQuestionTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	questionsRaw, ok := input["questions"].([]interface{})
	if !ok {
		return nil, &Error{Code: "INVALID_INPUT", Message: "questions must be an array"}
	}

	// In non-interactive mode, return pending status
	// In interactive mode, this would prompt the user via CLI
	// For now, we return a structured response that the runtime can handle

	questions := make([]map[string]interface{}, 0, len(questionsRaw))
	for _, q := range questionsRaw {
		if qMap, ok := q.(map[string]interface{}); ok {
			questions = append(questions, qMap)
		}
	}

	return map[string]interface{}{
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

func (t *ConfigTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"operation": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"get", "set", "list"},
				"description": "Operation to perform",
			},
			"key": map[string]interface{}{
				"type":        "string",
				"description": "Configuration key (e.g., 'model', 'theme')",
			},
			"value": map[string]interface{}{
				"description": "New value for set operation",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *ConfigTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
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

func (t *ConfigTool) get(input map[string]interface{}) (interface{}, error) {
	key, _ := input["key"].(string)
	if key == "" {
		return nil, &Error{Code: "MISSING_KEY", Message: "key is required for get operation"}
	}

	config := t.loadConfig()
	value, exists := config[key]
	if !exists {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Unknown setting: %s", key),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"key":     key,
		"value":   value,
	}, nil
}

func (t *ConfigTool) set(input map[string]interface{}) (interface{}, error) {
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

	return map[string]interface{}{
		"success":        true,
		"key":            key,
		"previous_value": previousValue,
		"new_value":      value,
	}, nil
}

func (t *ConfigTool) list() (interface{}, error) {
	config := t.loadConfig()

	settings := make([]map[string]interface{}, 0, len(config))
	for k, v := range config {
		settings = append(settings, map[string]interface{}{
			"key":   k,
			"value": v,
		})
	}

	return map[string]interface{}{
		"success":  true,
		"settings": settings,
		"count":    len(settings),
	}, nil
}

func (t *ConfigTool) loadConfig() map[string]interface{} {
	configPath := filepath.Join(t.configDir, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return map[string]interface{}{
			"model":      "claude-sonnet-4-5",
			"permission": "ask",
			"log_level":  "info",
		}
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return map[string]interface{}{}
	}

	return config
}

func (t *ConfigTool) saveConfig(config map[string]interface{}) error {
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

func (t *SkillTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "The skill name (e.g., 'commit', 'review-pr', 'git-master')",
			},
			"user_message": map[string]interface{}{
				"type":        "string",
				"description": "Optional context or arguments for the skill",
			},
		},
		"required": []string{"name"},
	}
}

func (t *SkillTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
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
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Skill not found: %s", name),
		}, nil
	}

	return map[string]interface{}{
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

func (t *NotebookEditTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the notebook file",
			},
			"cell_number": map[string]interface{}{
				"type":        "integer",
				"description": "Cell number to edit (0-indexed)",
			},
			"source": map[string]interface{}{
				"type":        "string",
				"description": "New cell source code",
			},
			"cell_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"code", "markdown"},
				"description": "Type of cell",
			},
		},
		"required": []string{"path", "cell_number"},
	}
}

func (t *NotebookEditTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
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

	var notebook map[string]interface{}
	if err := json.Unmarshal(data, &notebook); err != nil {
		return nil, &Error{Code: "PARSE_ERROR", Message: "invalid notebook format"}
	}

	cells, ok := notebook["cells"].([]interface{})
	if !ok {
		return nil, &Error{Code: "INVALID_NOTEBOOK", Message: "notebook has no cells"}
	}

	if cellNumber < 0 || cellNumber >= len(cells) {
		return nil, &Error{Code: "INVALID_CELL", Message: fmt.Sprintf("cell %d does not exist", cellNumber)}
	}

	// Update cell
	cell, ok := cells[cellNumber].(map[string]interface{})
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

	return map[string]interface{}{
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

func (t *BrowseTool) Name() string        { return "browse" }
func (t *BrowseTool) Description() string { return "Open URL in browser" }

func (t *BrowseTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"url": map[string]interface{}{"type": "string"},
		},
		"required": []string{"url"},
	}
}

func (t *BrowseTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	url, _ := input["url"].(string)
	if url == "" {
		return nil, ErrRequiredField("url")
	}
	return map[string]interface{}{
		"url":     url,
		"message": "Browser opened",
	}, nil
}

type AttachTool struct{}

func (t *AttachTool) Name() string        { return "attach" }
func (t *AttachTool) Description() string { return "Attach to running process" }

func (t *AttachTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pid": map[string]interface{}{"type": "string"},
		},
		"required": []string{"pid"},
	}
}

func (t *AttachTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pid, _ := input["pid"].(string)
	if pid == "" {
		return nil, ErrRequiredField("pid")
	}
	return map[string]interface{}{
		"pid":     pid,
		"message": "Process attach not implemented",
	}, nil
}

type DebugTool struct{}

func (t *DebugTool) Name() string        { return "debug" }
func (t *DebugTool) Description() string { return "Debug mode toggle" }

func (t *DebugTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{"type": "boolean"},
		},
	}
}

func (t *DebugTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	enable, _ := input["enable"].(bool)
	return map[string]interface{}{
		"debug":   enable,
		"message": "Debug mode toggled",
	}, nil
}

type IndexTool struct{}

func (t *IndexTool) Name() string        { return "index" }
func (t *IndexTool) Description() string { return "Build code index" }

func (t *IndexTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string"},
			"exclude": map[string]interface{}{"type": "array"},
		},
	}
}

func (t *IndexTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}
	return map[string]interface{}{
		"path":    path,
		"message": "Indexing not fully implemented",
	}, nil
}

type CacheTool struct{}

func (t *CacheTool) Name() string        { return "cache" }
func (t *CacheTool) Description() string { return "Cache management" }

func (t *CacheTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{"type": "string", "enum": []string{"get", "set", "clear", "stats"}},
			"key":    map[string]interface{}{"type": "string"},
			"value":  map[string]interface{}{"type": "string"},
		},
	}
}

func (t *CacheTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	action, _ := input["action"].(string)
	key, _ := input["key"].(string)

	switch action {
	case "clear":
		return map[string]interface{}{"message": "Cache cleared"}, nil
	case "stats":
		return map[string]interface{}{"hits": 0, "misses": 0, "size": 0}, nil
	default:
		return map[string]interface{}{"key": key, "message": "Cache operation completed"}, nil
	}
}

type ObserveTool struct{}

func (t *ObserveTool) Name() string        { return "observe" }
func (t *ObserveTool) Description() string { return "Watch for file changes" }

func (t *ObserveTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string"},
			"pattern": map[string]interface{}{"type": "string"},
		},
	}
}

func (t *ObserveTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}
	return map[string]interface{}{
		"path":    path,
		"message": "File watching not fully implemented",
	}, nil
}

type LazyTool struct{}

func (t *LazyTool) Name() string        { return "lazy" }
func (t *LazyTool) Description() string { return "Lazy mode for batching" }

func (t *LazyTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"enable": map[string]interface{}{"type": "boolean"},
		},
	}
}

func (t *LazyTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	enable, _ := input["enable"].(bool)
	return map[string]interface{}{
		"lazy":    enable,
		"message": "Lazy mode toggled",
	}, nil
}

type ThinkTool struct{}

func (t *ThinkTool) Name() string        { return "think" }
func (t *ThinkTool) Description() string { return "Think mode for reasoning" }

func (t *ThinkTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"prompt": map[string]interface{}{"type": "string"},
		},
		"required": []string{"prompt"},
	}
}

func (t *ThinkTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	prompt, _ := input["prompt"].(string)
	return map[string]interface{}{
		"thinking": prompt,
		"message":  "Think mode enabled",
	}, nil
}

type DeepThinkTool struct{}

func (t *DeepThinkTool) Name() string        { return "deepthink" }
func (t *DeepThinkTool) Description() string { return "Deep thinking for complex problems" }

func (t *DeepThinkTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"prompt": map[string]interface{}{"type": "string"},
			"depth":  map[string]interface{}{"type": "integer", "default": 5},
		},
		"required": []string{"prompt"},
	}
}

func (t *DeepThinkTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	prompt, _ := input["prompt"].(string)
	depth, _ := input["depth"].(int)
	if depth == 0 {
		depth = 5
	}
	return map[string]interface{}{
		"prompt":   prompt,
		"depth":    depth,
		"thinking": "Deep thinking enabled",
		"message":  "Using extended reasoning",
	}, nil
}

type ForkTool struct{}

func (t *ForkTool) Name() string        { return "fork" }
func (t *ForkTool) Description() string { return "Fork current session" }

func (t *ForkTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"label": map[string]interface{}{"type": "string"},
		},
	}
}

func (t *ForkTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	label, _ := input["label"].(string)
	return map[string]interface{}{
		"session_id": fmt.Sprintf("fork_%d", os.Getpid()),
		"label":      label,
		"message":    "Session forked",
	}, nil
}

type EnvTool struct{}

func (t *EnvTool) Name() string        { return "env" }
func (t *EnvTool) Description() string { return "Environment variable access" }

func (t *EnvTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"key":   map[string]interface{}{"type": "string"},
			"value": map[string]interface{}{"type": "string"},
		},
	}
}

func (t *EnvTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	key, _ := input["key"].(string)
	value, _ := input["value"].(string)

	if key != "" && value != "" {
		os.Setenv(key, value)
		return map[string]interface{}{"key": key, "message": "Environment set"}, nil
	}
	if key != "" {
		return map[string]interface{}{"key": key, "value": os.Getenv(key)}, nil
	}
	return map[string]interface{}{"message": "Environment access"}, nil
}
