package tools

import (
	"context"
	"os"

	"github.com/instructkr/smartclaw/internal/github"
)

type GitHubCreatePRTool struct{ BaseTool }

func (t *GitHubCreatePRTool) Name() string { return "github_create_pr" }

func (t *GitHubCreatePRTool) Description() string {
	return "Create a GitHub pull request using the gh CLI"
}

func (t *GitHubCreatePRTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":  map[string]any{"type": "string", "description": "PR title"},
			"body":   map[string]any{"type": "string", "description": "PR body/description"},
			"head":   map[string]any{"type": "string", "description": "Head branch"},
			"base":   map[string]any{"type": "string", "description": "Base branch"},
			"draft":  map[string]any{"type": "boolean", "description": "Create as draft PR"},
			"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels to apply"},
			"repo":   map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
		"required": []string{"title", "body"},
	}
}

func (t *GitHubCreatePRTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	title, _ := input["title"].(string)
	body, _ := input["body"].(string)
	if title == "" {
		return nil, ErrRequiredField("title")
	}

	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	client := github.NewClient(repoDir)

	opts := &github.PROptions{
		Title:  title,
		Body:   body,
		Head:   input["head"].(string),
		Base:   input["base"].(string),
		Draft:  input["draft"].(bool),
		Labels: toStringSlice(input["labels"]),
	}

	pr, err := client.CreatePR(ctx, opts)
	if err != nil {
		return nil, &Error{Code: "GITHUB_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"number": pr.Number,
		"title":  pr.Title,
		"state":  pr.State,
		"url":    pr.HTMLURL,
		"head":   pr.Head,
		"base":   pr.Base,
		"author": pr.Author,
	}, nil
}

type GitHubListPRsTool struct{ BaseTool }

func (t *GitHubListPRsTool) Name() string { return "github_list_prs" }

func (t *GitHubListPRsTool) Description() string {
	return "List GitHub pull requests using the gh CLI"
}

func (t *GitHubListPRsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"state": map[string]any{"type": "string", "description": "PR state filter (open, closed, merged, all)", "enum": []string{"open", "closed", "merged", "all"}},
			"repo":  map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
	}
}

func (t *GitHubListPRsTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	state, _ := input["state"].(string)
	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	client := github.NewClient(repoDir)
	prs, err := client.ListPRs(ctx, state)
	if err != nil {
		return nil, &Error{Code: "GITHUB_ERROR", Message: err.Error()}
	}

	result := make([]map[string]any, len(prs))
	for i, pr := range prs {
		result[i] = map[string]any{
			"number": pr.Number,
			"title":  pr.Title,
			"state":  pr.State,
			"url":    pr.HTMLURL,
			"head":   pr.Head,
			"base":   pr.Base,
			"author": pr.Author,
		}
	}

	return map[string]any{
		"prs":   result,
		"count": len(result),
	}, nil
}

type GitHubMergePRTool struct{ BaseTool }

func (t *GitHubMergePRTool) Name() string { return "github_merge_pr" }

func (t *GitHubMergePRTool) Description() string {
	return "Merge a GitHub pull request using the gh CLI"
}

func (t *GitHubMergePRTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number":   map[string]any{"type": "integer", "description": "PR number to merge"},
			"strategy": map[string]any{"type": "string", "description": "Merge strategy (merge, squash, rebase)", "enum": []string{"merge", "squash", "rebase"}},
			"repo":     map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
		"required": []string{"number"},
	}
}

func (t *GitHubMergePRTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	number, _ := input["number"].(int)
	if number == 0 {
		return nil, ErrRequiredField("number")
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

	client := github.NewClient(repoDir)
	if err := client.MergePR(ctx, number, strategy); err != nil {
		return nil, &Error{Code: "GITHUB_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"number":   number,
		"strategy": strategy,
		"merged":   true,
	}, nil
}

type GitHubCreateIssueTool struct{ BaseTool }

func (t *GitHubCreateIssueTool) Name() string { return "github_create_issue" }

func (t *GitHubCreateIssueTool) Description() string {
	return "Create a GitHub issue using the gh CLI"
}

func (t *GitHubCreateIssueTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":  map[string]any{"type": "string", "description": "Issue title"},
			"body":   map[string]any{"type": "string", "description": "Issue body/description"},
			"labels": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Labels to apply"},
			"repo":   map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
		"required": []string{"title"},
	}
}

func (t *GitHubCreateIssueTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	title, _ := input["title"].(string)
	if title == "" {
		return nil, ErrRequiredField("title")
	}

	body, _ := input["body"].(string)
	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	client := github.NewClient(repoDir)

	opts := &github.IssueOptions{
		Title:  title,
		Body:   body,
		Labels: toStringSlice(input["labels"]),
	}

	issue, err := client.CreateIssue(ctx, opts)
	if err != nil {
		return nil, &Error{Code: "GITHUB_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"number": issue.Number,
		"title":  issue.Title,
		"state":  issue.State,
		"url":    issue.HTMLURL,
		"author": issue.Author,
	}, nil
}

type GitHubListIssuesTool struct{ BaseTool }

func (t *GitHubListIssuesTool) Name() string { return "github_list_issues" }

func (t *GitHubListIssuesTool) Description() string {
	return "List GitHub issues using the gh CLI"
}

func (t *GitHubListIssuesTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"state": map[string]any{"type": "string", "description": "Issue state filter (open, closed, all)", "enum": []string{"open", "closed", "all"}},
			"repo":  map[string]any{"type": "string", "description": "Path to the git repo (defaults to current directory)"},
		},
	}
}

func (t *GitHubListIssuesTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	state, _ := input["state"].(string)
	repoDir, _ := input["repo"].(string)
	if repoDir == "" {
		var err error
		repoDir, err = os.Getwd()
		if err != nil {
			repoDir = "."
		}
	}

	client := github.NewClient(repoDir)
	issues, err := client.ListIssues(ctx, state)
	if err != nil {
		return nil, &Error{Code: "GITHUB_ERROR", Message: err.Error()}
	}

	result := make([]map[string]any, len(issues))
	for i, issue := range issues {
		result[i] = map[string]any{
			"number": issue.Number,
			"title":  issue.Title,
			"state":  issue.State,
			"url":    issue.HTMLURL,
			"author": issue.Author,
		}
	}

	return map[string]any{
		"issues": result,
		"count":  len(result),
	}, nil
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
