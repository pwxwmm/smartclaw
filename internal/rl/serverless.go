package rl

import (
	"context"
	"time"
)

// ServerlessBackend defines the interface for serverless execution backends
// that can create and manage remote workspaces for RL episode execution.
// Implementations include Daytona, Modal, and similar platforms.
type ServerlessBackend interface {
	// Name returns the backend identifier (e.g., "daytona", "modal").
	Name() string

	// CreateWorkspace creates a new execution workspace.
	// Returns the workspace ID on success.
	CreateWorkspace(ctx context.Context, config WorkspaceConfig) (string, error)

	// ExecuteTask runs a task in the specified workspace.
	// Returns the task result.
	ExecuteTask(ctx context.Context, workspaceID string, task TaskSpec) (*TaskResult, error)

	// DestroyWorkspace tears down a workspace, releasing its resources.
	DestroyWorkspace(ctx context.Context, workspaceID string) error

	// WorkspaceStatus returns the current status of a workspace.
	WorkspaceStatus(ctx context.Context, workspaceID string) (*WorkspaceStatus, error)

	// ListWorkspaces returns all active workspaces.
	ListWorkspaces(ctx context.Context) ([]WorkspaceInfo, error)
}

// WorkspaceConfig specifies parameters for creating a new workspace.
type WorkspaceConfig struct {
	Name     string            `json:"name"`
	Image    string            `json:"image,omitempty"`       // container image
	Env      map[string]string `json:"env,omitempty"`         // environment variables
	CPUCores float64           `json:"cpu_cores,omitempty"`   // CPU allocation
	MemoryMB int               `json:"memory_mb,omitempty"`   // memory in MB
	Timeout  time.Duration     `json:"timeout,omitempty"`     // max workspace lifetime
	Labels   map[string]string `json:"labels,omitempty"`      // for filtering
}

// TaskSpec specifies a task to execute in a workspace.
type TaskSpec struct {
	Command   string            `json:"command"`               // command to execute
	Args      []string          `json:"args,omitempty"`        // command arguments
	WorkDir   string            `json:"work_dir,omitempty"`    // working directory
	Env       map[string]string `json:"env,omitempty"`         // task-specific env vars
	Timeout   time.Duration     `json:"timeout,omitempty"`     // per-task timeout
	InputData []byte            `json:"input_data,omitempty"`  // stdin data
}

// TaskResult holds the result of a task executed in a workspace.
type TaskResult struct {
	ExitCode    int           `json:"exit_code"`
	Stdout      string        `json:"stdout"`
	Stderr      string        `json:"stderr"`
	Duration    time.Duration `json:"duration"`
	WorkspaceID string        `json:"workspace_id"`
}

// WorkspaceStatus represents the current state of a workspace.
type WorkspaceStatus struct {
	ID        string    `json:"id"`
	State     string    `json:"state"` // "creating", "running", "stopped", "error"
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// WorkspaceInfo provides summary information about a workspace.
type WorkspaceInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}
