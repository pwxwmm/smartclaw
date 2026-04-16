package tools

import (
	"context"
	"os"

	"github.com/instructkr/smartclaw/internal/worktree"
)

type WorktreeCreateTool struct{ BaseTool }

func (t *WorktreeCreateTool) Name() string { return "worktree_create" }

func (t *WorktreeCreateTool) Description() string {
	return "Create a git worktree for parallel development using the worktree manager"
}

func (t *WorktreeCreateTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Name for the worktree (used as branch name with smartclaw/ prefix)"},
			"ref":  map[string]any{"type": "string", "description": "Git ref to base the worktree on (branch, tag, commit)"},
			"repo": map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
		"required": []string{"name"},
	}
}

func (t *WorktreeCreateTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return nil, ErrRequiredField("name")
	}

	ref, _ := input["ref"].(string)
	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	mgr := worktree.NewManager(repoDir)
	path, err := mgr.Create(ctx, name, ref)
	if err != nil {
		return nil, &Error{Code: "WORKTREE_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"name":    name,
		"path":    path,
		"branch":  "smartclaw/" + name,
		"created": true,
	}, nil
}

type WorktreeRemoveTool struct{ BaseTool }

func (t *WorktreeRemoveTool) Name() string { return "worktree_remove" }

func (t *WorktreeRemoveTool) Description() string {
	return "Remove a git worktree and its associated branch"
}

func (t *WorktreeRemoveTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Name of the worktree to remove"},
			"repo": map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
		"required": []string{"name"},
	}
}

func (t *WorktreeRemoveTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return nil, ErrRequiredField("name")
	}

	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	mgr := worktree.NewManager(repoDir)
	if err := mgr.Remove(ctx, name); err != nil {
		return nil, &Error{Code: "WORKTREE_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"name":    name,
		"removed": true,
	}, nil
}

type WorktreeListTool struct{ BaseTool }

func (t *WorktreeListTool) Name() string { return "worktree_list" }

func (t *WorktreeListTool) Description() string {
	return "List all smartclaw-managed git worktrees"
}

func (t *WorktreeListTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"repo": map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
	}
}

func (t *WorktreeListTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	mgr := worktree.NewManager(repoDir)
	worktrees, err := mgr.List(ctx)
	if err != nil {
		return nil, &Error{Code: "WORKTREE_ERROR", Message: err.Error()}
	}

	result := make([]map[string]any, len(worktrees))
	for i, wt := range worktrees {
		result[i] = map[string]any{
			"name":    wt.Name,
			"path":    wt.Path,
			"branch":  wt.Branch,
			"baseRef": wt.BaseRef,
		}
	}

	return map[string]any{
		"worktrees": result,
		"count":     len(result),
	}, nil
}

type WorktreeDiffTool struct{ BaseTool }

func (t *WorktreeDiffTool) Name() string { return "worktree_diff" }

func (t *WorktreeDiffTool) Description() string {
	return "Show the diff between a worktree branch and its merge base"
}

func (t *WorktreeDiffTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string", "description": "Name of the worktree"},
			"repo": map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
		"required": []string{"name"},
	}
}

func (t *WorktreeDiffTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return nil, ErrRequiredField("name")
	}

	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	mgr := worktree.NewManager(repoDir)
	diff, err := mgr.Diff(ctx, name)
	if err != nil {
		return nil, &Error{Code: "WORKTREE_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"name": name,
		"diff": string(diff),
	}, nil
}

type WorktreeMergeTool struct{ BaseTool }

func (t *WorktreeMergeTool) Name() string { return "worktree_merge" }

func (t *WorktreeMergeTool) Description() string {
	return "Merge a worktree branch back into the main branch"
}

func (t *WorktreeMergeTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":     map[string]any{"type": "string", "description": "Name of the worktree to merge"},
			"strategy": map[string]any{"type": "string", "description": "Merge strategy (merge, squash, rebase)", "enum": []string{"merge", "squash", "rebase"}},
			"repo":     map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
		"required": []string{"name"},
	}
}

func (t *WorktreeMergeTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return nil, ErrRequiredField("name")
	}

	strategy, _ := input["strategy"].(string)
	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	mgr := worktree.NewManager(repoDir)
	err := mgr.Merge(ctx, name, strategy)
	if err != nil {
		return nil, &Error{Code: "WORKTREE_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"name":     name,
		"strategy": strategy,
		"merged":   true,
	}, nil
}
