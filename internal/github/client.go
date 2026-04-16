package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type Repository struct {
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"fullName"`
	Description   string `json:"description"`
	DefaultBranch string `json:"defaultBranch"`
	HTMLURL       string `json:"url"`
}

type repoOwnerJSON struct {
	Login string `json:"login"`
}

type defaultBranchRefJSON struct {
	Name string `json:"name"`
}

type repoViewJSON struct {
	Owner            repoOwnerJSON        `json:"owner"`
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	URL              string               `json:"url"`
	DefaultBranchRef defaultBranchRefJSON `json:"defaultBranchRef"`
}

// Client wraps the gh CLI for GitHub operations.
type Client struct {
	repoDir string // path to the git repo
}

// NewClient creates a new GitHub client that operates on the given repo directory.
func NewClient(repoDir string) *Client {
	return &Client{repoDir: repoDir}
}

// CheckInstalled verifies gh CLI is available and authenticated.
func (c *Client) CheckInstalled(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found in PATH: %w", err)
	}

	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	cmd.Dir = c.repoDir

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh CLI not authenticated: run 'gh auth login' first")
	}

	return nil
}

func (c *Client) RepoInfo(ctx context.Context) (*Repository, error) {
	out, err := c.runGH(ctx, "repo", "view",
		"--json", "owner,name,description,defaultBranchRef,url")
	if err != nil {
		return nil, fmt.Errorf("repo info: %w", err)
	}

	var view repoViewJSON
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		return nil, fmt.Errorf("repo info: parse JSON: %w", err)
	}

	return &Repository{
		Owner:         view.Owner.Login,
		Name:          view.Name,
		FullName:      view.Owner.Login + "/" + view.Name,
		Description:   view.Description,
		DefaultBranch: view.DefaultBranchRef.Name,
		HTMLURL:       view.URL,
	}, nil
}
