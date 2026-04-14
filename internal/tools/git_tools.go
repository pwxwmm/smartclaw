package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/instructkr/smartclaw/internal/git"
)

type GitAITool struct{ BaseTool }

func (g *GitAITool) Name() string	{ return "git_ai" }

func (g *GitAITool) Description() string {
	return "AI-powered git operations: generate commit messages, review changes, create PR descriptions"
}

func (g *GitAITool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":		"string",
				"enum":		[]string{"commit_message", "review", "pr_description"},
				"description":	"The git AI action to perform",
			},
			"diff": map[string]any{
				"type":		"string",
				"description":	"The git diff content (optional, will use working directory if not provided)",
			},
			"files": map[string]any{
				"type":		"array",
				"items":	map[string]any{"type": "string"},
				"description":	"List of changed files for context",
			},
		},
		"required":	[]string{"action"},
	}
}

func (g *GitAITool) Execute(ctx context.Context, input map[string]any) (any, error) {
	action, _ := input["action"].(string)
	diff, _ := input["diff"].(string)

	var files []string
	if raw, ok := input["files"].([]any); ok {
		for _, f := range raw {
			if s, ok := f.(string); ok {
				files = append(files, s)
			}
		}
	}

	switch action {
	case "commit_message":
		return g.generateCommitMessage(diff, files)
	case "review":
		return g.reviewChanges(diff, files)
	case "pr_description":
		return g.generatePRDescription(diff, files)
	default:
		return nil, fmt.Errorf("unknown action: %s (valid: commit_message, review, pr_description)", action)
	}
}

func (g *GitAITool) generateCommitMessage(diff string, files []string) (any, error) {
	commitType := "chore"
	scope := ""
	subject := "update files"

	if len(files) > 0 {
		scope = extractScope(files[0])
	}

	lines := strings.Split(diff, "\n")
	added, removed := 0, 0
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed++
		}
	}

	for _, f := range files {
		lower := strings.ToLower(f)
		switch {
		case strings.Contains(lower, "test") || strings.HasSuffix(lower, "_test.go"):
			commitType = "test"
			subject = fmt.Sprintf("add/update tests for %s", scope)
		case strings.Contains(lower, "doc") || strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt"):
			commitType = "docs"
			subject = fmt.Sprintf("update documentation for %s", scope)
		case strings.Contains(lower, "fix") || strings.Contains(lower, "bug"):
			commitType = "fix"
			subject = fmt.Sprintf("fix issue in %s", scope)
		case strings.Contains(lower, "config") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".json"):
			commitType = "chore"
			subject = fmt.Sprintf("update configuration for %s", scope)
		case strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".py"):
			commitType = "feat"
			subject = fmt.Sprintf("update %s", scope)
		}
	}

	if added > 0 && removed == 0 {
		subject = fmt.Sprintf("add %s", scope)
	} else if removed > 0 && added == 0 {
		subject = fmt.Sprintf("remove code from %s", scope)
	}

	msg := fmt.Sprintf("%s(%s): %s", commitType, scope, subject)
	if scope == "" {
		msg = fmt.Sprintf("%s: %s", commitType, subject)
	}

	body := fmt.Sprintf("Changed %d file(s): +%d -%d lines", len(files), added, removed)

	return map[string]any{
		"commit_message":	msg,
		"body":			body,
		"type":			commitType,
		"scope":		scope,
		"files_changed":	len(files),
		"lines_added":		added,
		"lines_removed":	removed,
	}, nil
}

func (g *GitAITool) reviewChanges(diff string, files []string) (any, error) {
	findings := []map[string]any{}

	lines := strings.Split(diff, "\n")
	for i, line := range lines {
		trimmed := strings.TrimPrefix(strings.TrimPrefix(line, "+"), " ")
		lower := strings.ToLower(trimmed)

		switch {
		case strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "api_key"):
			findings = append(findings, map[string]any{
				"severity":	"critical",
				"category":	"security",
				"description":	"Potential secret or credential exposed in code",
				"line":		i + 1,
			})
		case strings.Contains(lower, "todo") || strings.Contains(lower, "fixme") || strings.Contains(lower, "hack"):
			findings = append(findings, map[string]any{
				"severity":	"warning",
				"category":	"code_quality",
				"description":	"TODO/FIXME/HACK comment found",
				"line":		i + 1,
			})
		case strings.Contains(lower, "fmt.sprintf") && strings.Contains(lower, "%s") && (strings.Contains(lower, "sql") || strings.Contains(lower, "query")):
			findings = append(findings, map[string]any{
				"severity":	"critical",
				"category":	"security",
				"description":	"Potential SQL injection via string formatting",
				"line":		i + 1,
			})
		case strings.Contains(lower, "panic("):
			findings = append(findings, map[string]any{
				"severity":	"warning",
				"category":	"error_handling",
				"description":	"Unhandled panic() call",
				"line":		i + 1,
			})
		case strings.Contains(lower, "catch(e) {}") || strings.Contains(lower, "catch (e) {}") || strings.Contains(lower, "catch(e){}"):
			findings = append(findings, map[string]any{
				"severity":	"warning",
				"category":	"error_handling",
				"description":	"Empty catch block",
				"line":		i + 1,
			})
		}
	}

	if len(findings) == 0 {
		findings = append(findings, map[string]any{
			"severity":	"info",
			"category":	"general",
			"description":	"No obvious issues found in the diff. Consider reviewing manually for context-specific concerns.",
		})
	}

	return map[string]any{
		"review":	findings,
		"files_review":	len(files),
		"total_lines":	len(lines),
		"summary":	fmt.Sprintf("Reviewed %d files, found %d findings", len(files), len(findings)),
	}, nil
}

func (g *GitAITool) generatePRDescription(diff string, files []string) (any, error) {
	sections := map[string][]string{
		"added":	{},
		"modified":	{},
		"removed":	{},
	}

	lines := strings.Split(diff, "\n")
	added, removed := 0, 0
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			added++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removed++
		}
	}

	for _, f := range files {
		lower := strings.ToLower(f)
		switch {
		case strings.Contains(lower, "test"):
			sections["modified"] = append(sections["modified"], fmt.Sprintf("- Updated test: `%s`", f))
		case strings.Contains(lower, "doc") || strings.HasSuffix(lower, ".md"):
			sections["modified"] = append(sections["modified"], fmt.Sprintf("- Updated docs: `%s`", f))
		default:
			sections["modified"] = append(sections["modified"], fmt.Sprintf("- Changed: `%s`", f))
		}
	}

	var desc strings.Builder
	desc.WriteString("## Summary\n\n")
	desc.WriteString(fmt.Sprintf("This PR modifies %d file(s) with +%d/-%d lines.\n\n", len(files), added, removed))

	if len(sections["added"]) > 0 {
		desc.WriteString("## Added\n")
		for _, s := range sections["added"] {
			desc.WriteString(s + "\n")
		}
		desc.WriteString("\n")
	}

	if len(sections["modified"]) > 0 {
		desc.WriteString("## Changed\n")
		for _, s := range sections["modified"] {
			desc.WriteString(s + "\n")
		}
		desc.WriteString("\n")
	}

	if len(sections["removed"]) > 0 {
		desc.WriteString("## Removed\n")
		for _, s := range sections["removed"] {
			desc.WriteString(s + "\n")
		}
		desc.WriteString("\n")
	}

	desc.WriteString("## Testing\n\n- [ ] Manual testing completed\n- [ ] Automated tests pass\n")

	return map[string]any{
		"description":		desc.String(),
		"files_changed":	len(files),
		"lines_added":		added,
		"lines_removed":	removed,
	}, nil
}

func extractScope(filePath string) string {
	parts := strings.Split(filePath, "/")
	if len(parts) > 1 {
		return parts[len(parts)-2]
	}

	name := parts[0]
	name = strings.TrimSuffix(name, filepath.Ext(name))
	if name == "" {
		return "general"
	}
	return name
}

type GitStatusTool struct{ BaseTool }

func (g *GitStatusTool) Name() string	{ return "git_status" }
func (g *GitStatusTool) Description() string {
	return "Get git status for the current working directory"
}

func (g *GitStatusTool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":		"string",
				"description":	"Working directory path",
			},
		},
	}
}

func (g *GitStatusTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	if path == "" {
		path = "."
	}

	gitCtx, err := getGitContext(path)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"is_repo":		gitCtx.IsRepo,
		"branch":		gitCtx.Branch,
		"has_changes":		gitCtx.HasChanges,
		"staged_files":		gitCtx.StagedFiles,
		"changed_files":	gitCtx.ChangedFiles,
	}, nil
}

type GitDiffTool struct{ BaseTool }

func (g *GitDiffTool) Name() string		{ return "git_diff" }
func (g *GitDiffTool) Description() string	{ return "Get git diff for the current working directory" }

func (g *GitDiffTool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":		"string",
				"description":	"Working directory path",
			},
			"staged": map[string]any{
				"type":		"boolean",
				"description":	"Whether to get staged diff",
			},
		},
	}
}

func (g *GitDiffTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	staged, _ := input["staged"].(bool)
	if path == "" {
		path = "."
	}

	diff, err := getGitDiff(path, staged)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"diff":		diff,
		"staged":	staged,
	}, nil
}

type GitLogTool struct{ BaseTool }

func (g *GitLogTool) Name() string		{ return "git_log" }
func (g *GitLogTool) Description() string	{ return "Get recent git commit log" }

func (g *GitLogTool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":		"string",
				"description":	"Working directory path",
			},
			"count": map[string]any{
				"type":		"number",
				"description":	"Number of commits to show",
			},
		},
	}
}

func (g *GitLogTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	count := 10
	if c, ok := input["count"].(float64); ok && c > 0 {
		count = int(c)
	}
	if path == "" {
		path = "."
	}

	log, err := getGitLog(path, count)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"log":		log,
		"count":	count,
	}, nil
}

type GitContextResult struct {
	IsRepo		bool
	Branch		string
	HasChanges	bool
	StagedFiles	[]string
	ChangedFiles	[]string
}

func getGitContext(dir string) (*GitContextResult, error) {
	ctx, err := git.GetContext(dir)
	if err != nil {
		return nil, err
	}
	return &GitContextResult{
		IsRepo:		ctx.IsRepo,
		Branch:		ctx.Branch,
		HasChanges:	ctx.HasChanges,
		StagedFiles:	ctx.StagedFiles,
		ChangedFiles:	ctx.ChangedFiles,
	}, nil
}

func getGitDiff(dir string, staged bool) (string, error) {
	return git.GetDiff(dir, staged)
}

func getGitLog(dir string, count int) (string, error) {
	return git.GetLog(dir, count)
}

var _ = filepath.Ext
