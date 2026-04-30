package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/instructkr/smartclaw/internal/playbook"
	"gopkg.in/yaml.v3"
)

type ToolDescriptor struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Inputs      []string `json:"inputs"`
}

// WorkflowService manages workflow CRUD and execution via the playbook system.
type WorkflowService struct {
	workflowsDir string
	mu           sync.RWMutex
	playbookMgr  *playbook.Manager
}

// NewWorkflowService creates a new WorkflowService backed by ~/.smartclaw/workflows/.
func NewWorkflowService() (*WorkflowService, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	dir := filepath.Join(homeDir, ".smartclaw", "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create workflows dir: %w", err)
	}
	return &WorkflowService{
		workflowsDir: dir,
		playbookMgr:  playbook.NewManager(dir),
	}, nil
}

// ListWorkflows returns all saved workflows (playbooks) in the workflows directory.
func (ws *WorkflowService) ListWorkflows() ([]playbook.Playbook, error) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	pbs, err := ws.playbookMgr.List()
	if err != nil {
		return nil, err
	}
	result := make([]playbook.Playbook, 0, len(pbs))
	for _, pb := range pbs {
		if pb != nil {
			result = append(result, *pb)
		}
	}
	return result, nil
}

// GetWorkflow reads a single workflow by name.
func (ws *WorkflowService) GetWorkflow(name string) (*playbook.Playbook, error) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.playbookMgr.Load(name)
}

// SaveWorkflow writes a workflow to the workflows directory as YAML.
func (ws *WorkflowService) SaveWorkflow(pb *playbook.Playbook) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if pb.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if err := os.MkdirAll(ws.workflowsDir, 0o755); err != nil {
		return fmt.Errorf("create workflows dir: %w", err)
	}
	return ws.playbookMgr.Save(pb)
}

// DeleteWorkflow removes a workflow file by name.
func (ws *WorkflowService) DeleteWorkflow(name string) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.playbookMgr.Delete(name)
}

// ExecuteWorkflow runs a workflow by delegating to the playbook executor.
func (ws *WorkflowService) ExecuteWorkflow(ctx context.Context, name string, params map[string]string) (*playbook.ExecutionContext, error) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.playbookMgr.Execute(ctx, name, params, nil)
}

// GetAvailableTools returns the list of tool descriptors for the node palette.
func (ws *WorkflowService) GetAvailableTools() []ToolDescriptor {
	return builtInTools
}

var builtInTools = []ToolDescriptor{
	{Name: "read_file", Description: "Read file contents from disk", Category: "file", Inputs: []string{"path"}},
	{Name: "write_file", Description: "Write content to a file", Category: "file", Inputs: []string{"path", "content"}},
	{Name: "edit_file", Description: "Edit file with string replacement", Category: "file", Inputs: []string{"path", "find", "replace"}},
	{Name: "glob", Description: "Find files matching a glob pattern", Category: "file", Inputs: []string{"pattern"}},
	{Name: "grep", Description: "Search file contents with regex", Category: "file", Inputs: []string{"pattern", "path"}},

	{Name: "bash", Description: "Execute a shell command", Category: "code", Inputs: []string{"command"}},
	{Name: "lsp", Description: "LSP operations (definition, references, diagnostics)", Category: "code", Inputs: []string{"operation", "file", "line"}},
	{Name: "ast_grep", Description: "AST pattern search and replace", Category: "code", Inputs: []string{"pattern", "language"}},

	{Name: "web_fetch", Description: "Fetch and convert a URL to markdown", Category: "web", Inputs: []string{"url"}},
	{Name: "web_search", Description: "Search the web for information", Category: "web", Inputs: []string{"query"}},
	{Name: "browser_navigate", Description: "Navigate to a URL in headless browser", Category: "web", Inputs: []string{"url"}},

	{Name: "git_status", Description: "Show git working directory status", Category: "git", Inputs: []string{}},
	{Name: "git_diff", Description: "Show git diff of changes", Category: "git", Inputs: []string{"staged"}},
	{Name: "git_log", Description: "Show recent git commit log", Category: "git", Inputs: []string{"count"}},

	{Name: "agent", Description: "Spawn sub-agent for parallel tasks", Category: "agent", Inputs: []string{"task", "type"}},
	{Name: "think", Description: "Structured thinking step before action", Category: "agent", Inputs: []string{"thought"}},
	{Name: "skill", Description: "Load and execute a skill", Category: "agent", Inputs: []string{"name"}},

	{Name: "docker_exec", Description: "Execute command in Docker container", Category: "docker", Inputs: []string{"container", "command"}},
	{Name: "execute_code", Description: "Run code in RPC sandbox with tool access", Category: "docker", Inputs: []string{"code", "language"}},

	{Name: "condition", Description: "Branch based on a condition expression", Category: "flow", Inputs: []string{"expression"}},
	{Name: "delay", Description: "Wait for a specified duration", Category: "flow", Inputs: []string{"duration"}},
	{Name: "parallel", Description: "Execute steps in parallel", Category: "flow", Inputs: []string{"branches"}},
}

// SaveWorkflowYAML is a helper to write a playbook directly as YAML bytes.
func (ws *WorkflowService) SaveWorkflowYAML(name string, data []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	if name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if err := os.MkdirAll(ws.workflowsDir, 0o755); err != nil {
		return fmt.Errorf("create workflows dir: %w", err)
	}

	var pb playbook.Playbook
	if err := yaml.Unmarshal(data, &pb); err != nil {
		return fmt.Errorf("invalid playbook YAML: %w", err)
	}

	path := filepath.Join(ws.workflowsDir, name+".yaml")
	return os.WriteFile(path, data, 0o644)
}
