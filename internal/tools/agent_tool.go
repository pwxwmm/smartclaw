package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type AgentDefinition struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	WorkDir     string            `json:"workDir,omitempty"`
	Prompt      string            `json:"prompt,omitempty"`
	Model       string            `json:"model,omitempty"`
	Tools       []string          `json:"tools,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	IsBuiltin   bool              `json:"isBuiltin,omitempty"`
}

type AgentInstance struct {
	ID          string
	Definition  *AgentDefinition
	StartTime   time.Time
	Status      string
	Output      string
	ExitCode    int
	WorktreeDir string
	cmd         *exec.Cmd
	mu          sync.Mutex
}

type AgentTool struct {
	instances map[string]*AgentInstance
	mu        sync.RWMutex
	agentsDir string
}

func NewAgentTool(agentsDir string) *AgentTool {
	return &AgentTool{
		instances: make(map[string]*AgentInstance),
		agentsDir: agentsDir,
	}
}

func (t *AgentTool) Name() string        { return "agent" }
func (t *AgentTool) Description() string { return "Spawn or manage sub-agents for parallel tasks" }

func (t *AgentTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"enum":        []string{"spawn", "resume", "list", "stop", "output"},
				"description": "Operation to perform",
			},
			"agent_type": map[string]any{
				"type":        "string",
				"description": "Type of agent to spawn (explore, verification, custom)",
			},
			"agent_id": map[string]any{
				"type":        "string",
				"description": "Agent ID for resume/stop/output operations",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "Prompt or task for the agent",
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Working directory for agent",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Model to use for agent",
			},
			"fork": map[string]any{
				"type":        "boolean",
				"description": "Whether to fork into worktree",
			},
			"background": map[string]any{
				"type":        "boolean",
				"description": "Run agent in background",
			},
		},
		"required": []string{"operation"},
	}
}

func (t *AgentTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	operation, _ := input["operation"].(string)
	if operation == "" {
		return nil, ErrRequiredField("operation")
	}

	switch operation {
	case "spawn":
		return t.spawnAgent(ctx, input)
	case "resume":
		return t.resumeAgent(ctx, input)
	case "list":
		return t.listAgents()
	case "stop":
		return t.stopAgent(input)
	case "output":
		return t.getOutput(input)
	default:
		return nil, &Error{Code: "INVALID_OPERATION", Message: "unknown operation: " + operation}
	}
}

func (t *AgentTool) spawnAgent(ctx context.Context, input map[string]any) (any, error) {
	agentType, _ := input["agent_type"].(string)
	if agentType == "" {
		agentType = "explore"
	}

	prompt, _ := input["prompt"].(string)
	workdir, _ := input["workdir"].(string)
	model, _ := input["model"].(string)
	fork, _ := input["fork"].(bool)
	background, _ := input["background"].(bool)

	if workdir == "" {
		if wd, err := os.Getwd(); err == nil {
			workdir = wd
		}
	}

	def := t.getAgentDefinition(agentType)
	if def == nil {
		def = &AgentDefinition{
			Name:        agentType,
			Description: "Custom agent",
			Type:        agentType,
		}
	}

	if model != "" {
		def.Model = model
	}
	if def.Model == "" {
		def.Model = "claude-sonnet-4-5"
	}

	instance := &AgentInstance{
		ID:         generateAgentID(),
		Definition: def,
		StartTime:  time.Now(),
		Status:     "starting",
	}

	if fork {
		worktreeDir, err := t.createWorktree(workdir, instance.ID)
		if err != nil {
			return nil, &Error{Code: "WORKTREE_ERROR", Message: err.Error()}
		}
		instance.WorktreeDir = worktreeDir
		workdir = worktreeDir
	}

	t.mu.Lock()
	t.instances[instance.ID] = instance
	t.mu.Unlock()

	if background {
		go t.runAgent(instance, prompt, workdir)
		return map[string]any{
			"agent_id": instance.ID,
			"status":   "running",
			"message":  "Agent started in background",
		}, nil
	}

	t.runAgent(instance, prompt, workdir)

	return map[string]any{
		"agent_id":  instance.ID,
		"status":    instance.Status,
		"output":    instance.Output,
		"exit_code": instance.ExitCode,
	}, nil
}

func (t *AgentTool) runAgent(instance *AgentInstance, prompt, workdir string) {
	instance.mu.Lock()
	instance.Status = "running"
	instance.mu.Unlock()

	args := []string{"run", "agent"}
	if instance.Definition.Model != "" {
		args = append(args, "--model", instance.Definition.Model)
	}
	if prompt != "" {
		args = append(args, "--prompt", prompt)
	}

	cmd := exec.Command("smart", args...)
	cmd.Dir = workdir

	output, err := cmd.CombinedOutput()

	instance.mu.Lock()
	defer instance.mu.Unlock()

	instance.Output = string(output)
	if err != nil {
		instance.Status = "failed"
		if exitErr, ok := err.(*exec.ExitError); ok {
			instance.ExitCode = exitErr.ExitCode()
		} else {
			instance.ExitCode = 1
		}
	} else {
		instance.Status = "completed"
		instance.ExitCode = 0
	}
}

func (t *AgentTool) resumeAgent(ctx context.Context, input map[string]any) (any, error) {
	agentID, _ := input["agent_id"].(string)
	if agentID == "" {
		return nil, ErrRequiredField("agent_id")
	}

	t.mu.RLock()
	instance, exists := t.instances[agentID]
	t.mu.RUnlock()

	if !exists {
		return nil, &Error{Code: "NOT_FOUND", Message: "agent not found: " + agentID}
	}

	return map[string]any{
		"agent_id": instance.ID,
		"status":   instance.Status,
		"output":   instance.Output,
	}, nil
}

func (t *AgentTool) listAgents() (any, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	agents := make([]map[string]any, 0, len(t.instances))
	for id, instance := range t.instances {
		agents = append(agents, map[string]any{
			"agent_id":   id,
			"type":       instance.Definition.Type,
			"status":     instance.Status,
			"started_at": instance.StartTime,
			"worktree":   instance.WorktreeDir,
		})
	}

	return map[string]any{
		"agents": agents,
		"count":  len(agents),
	}, nil
}

func (t *AgentTool) stopAgent(input map[string]any) (any, error) {
	agentID, _ := input["agent_id"].(string)
	if agentID == "" {
		return nil, ErrRequiredField("agent_id")
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	instance, exists := t.instances[agentID]
	if !exists {
		return nil, &Error{Code: "NOT_FOUND", Message: "agent not found: " + agentID}
	}

	if instance.cmd != nil && instance.cmd.Process != nil {
		if err := instance.cmd.Process.Kill(); err != nil {
			return nil, &Error{Code: "KILL_ERROR", Message: err.Error()}
		}
	}

	instance.Status = "stopped"

	return map[string]any{
		"agent_id": agentID,
		"status":   "stopped",
	}, nil
}

func (t *AgentTool) getOutput(input map[string]any) (any, error) {
	agentID, _ := input["agent_id"].(string)
	if agentID == "" {
		return nil, ErrRequiredField("agent_id")
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	instance, exists := t.instances[agentID]
	if !exists {
		return nil, &Error{Code: "NOT_FOUND", Message: "agent not found: " + agentID}
	}

	return map[string]any{
		"agent_id":  agentID,
		"status":    instance.Status,
		"output":    instance.Output,
		"exit_code": instance.ExitCode,
	}, nil
}

func (t *AgentTool) getAgentDefinition(agentType string) *AgentDefinition {
	builtins := map[string]*AgentDefinition{
		"explore": {
			Name:        "explore",
			Description: "Explores codebase to find patterns, implementations, and answers",
			Type:        "explore",
			IsBuiltin:   true,
			Prompt:      "You are an exploration agent. Find and analyze code patterns.",
			Tools:       []string{"read_file", "glob", "grep", "bash"},
		},
		"verification": {
			Name:        "verification",
			Description: "Verifies implementations and catches edge cases",
			Type:        "verification",
			IsBuiltin:   true,
			Prompt:      "You are a verification agent. Test and verify implementations.",
			Tools:       []string{"bash", "read_file", "write_file", "edit_file"},
		},
		"deep-research": {
			Name:        "deep-research",
			Description: "Deep research agent for complex analysis",
			Type:        "deep-research",
			IsBuiltin:   true,
			Tools:       []string{"read_file", "glob", "grep", "web_fetch", "web_search"},
		},
	}

	if def, ok := builtins[agentType]; ok {
		return def
	}

	customDef := t.loadAgentDefinition(agentType)
	return customDef
}

func (t *AgentTool) loadAgentDefinition(agentType string) *AgentDefinition {
	if t.agentsDir == "" {
		return nil
	}

	path := filepath.Join(t.agentsDir, agentType, "AGENT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var def AgentDefinition
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			def.Name = strings.TrimPrefix(line, "# ")
		} else if strings.HasPrefix(line, "description: ") {
			def.Description = strings.TrimPrefix(line, "description: ")
		} else if strings.HasPrefix(line, "model: ") {
			def.Model = strings.TrimPrefix(line, "model: ")
		}
	}

	def.Type = agentType
	def.IsBuiltin = false
	return &def
}

func (t *AgentTool) createWorktree(baseDir, agentID string) (string, error) {
	worktreeDir := filepath.Join(baseDir, ".smartclaw-worktrees", agentID)

	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		return "", err
	}

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD")
	if err := cmd.Run(); err != nil {
		os.RemoveAll(worktreeDir)
		return "", err
	}

	return worktreeDir, nil
}

func (t *AgentTool) CleanupWorktree(agentID string) error {
	t.mu.RLock()
	instance, exists := t.instances[agentID]
	t.mu.RUnlock()

	if !exists || instance.WorktreeDir == "" {
		return nil
	}

	cmd := exec.Command("git", "worktree", "remove", instance.WorktreeDir, "--force")
	if err := cmd.Run(); err != nil {
		return err
	}

	return os.RemoveAll(instance.WorktreeDir)
}

func generateAgentID() string {
	return fmt.Sprintf("agent_%d", time.Now().UnixNano())
}

type AgentRegistry struct {
	agents map[string]*AgentDefinition
}

func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]*AgentDefinition),
	}
}

func (r *AgentRegistry) Register(def *AgentDefinition) {
	r.agents[def.Type] = def
}

func (r *AgentRegistry) Get(agentType string) *AgentDefinition {
	return r.agents[agentType]
}

func (r *AgentRegistry) List() []*AgentDefinition {
	result := make([]*AgentDefinition, 0, len(r.agents))
	for _, def := range r.agents {
		result = append(result, def)
	}
	return result
}

func (r *AgentRegistry) LoadFromDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		agentFile := filepath.Join(dir, entry.Name(), "AGENT.md")
		if data, err := os.ReadFile(agentFile); err == nil {
			var def AgentDefinition
			def.Type = entry.Name()
			def.IsBuiltin = false

			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "# ") {
					def.Name = strings.TrimPrefix(line, "# ")
				} else if strings.HasPrefix(line, "description: ") {
					def.Description = strings.TrimPrefix(line, "description: ")
				}
			}

			r.Register(&def)
		}
	}

	return nil
}

func (r *AgentRegistry) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r.agents, "", "  ")
}
