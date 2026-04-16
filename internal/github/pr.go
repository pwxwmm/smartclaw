package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type PROptions struct {
	Title  string
	Body   string
	Head   string
	Base   string
	Draft  bool
	Labels []string
}

type PR struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"url"`
	Head    string `json:"headRefName"`
	Base    string `json:"baseRefName"`
	Author  string `json:"author"`
}

type prAuthor struct {
	Login string `json:"login"`
}

type prJSON struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	HTMLURL     string   `json:"url"`
	HeadRefName string   `json:"headRefName"`
	BaseRefName string   `json:"baseRefName"`
	Author      prAuthor `json:"author"`
}

func (p prJSON) toPR() *PR {
	return &PR{
		Number:  p.Number,
		Title:   p.Title,
		State:   p.State,
		HTMLURL: p.HTMLURL,
		Head:    p.HeadRefName,
		Base:    p.BaseRefName,
		Author:  p.Author.Login,
	}
}

func (c *Client) CreatePR(ctx context.Context, opts *PROptions) (*PR, error) {
	args := []string{"pr", "create",
		"--title", opts.Title,
		"--body", opts.Body,
	}

	if opts.Head != "" {
		args = append(args, "--head", opts.Head)
	}
	if opts.Base != "" {
		args = append(args, "--base", opts.Base)
	}
	if opts.Draft {
		args = append(args, "--draft")
	}
	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}

	url := strings.TrimSpace(out)
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return nil, fmt.Errorf("create PR: unexpected output: %s", url)
	}

	prNum, err := atoi(parts[len(parts)-1])
	if err != nil {
		return nil, fmt.Errorf("create PR: parse number: %w", err)
	}

	return c.GetPR(ctx, prNum)
}

func (c *Client) ListPRs(ctx context.Context, state string) ([]*PR, error) {
	args := []string{"pr", "list", "--json", "number,title,state,url,headRefName,baseRefName,author"}
	if state != "" {
		args = append(args, "--state", state)
	}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}

	var items []prJSON
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("list PRs: parse JSON: %w", err)
	}

	result := make([]*PR, len(items))
	for i, item := range items {
		result[i] = item.toPR()
	}
	return result, nil
}

func (c *Client) GetPR(ctx context.Context, number int) (*PR, error) {
	out, err := c.runGH(ctx, "pr", "view", fmt.Sprintf("%d", number),
		"--json", "number,title,state,url,headRefName,baseRefName,author")
	if err != nil {
		return nil, fmt.Errorf("get PR %d: %w", number, err)
	}

	var item prJSON
	if err := json.Unmarshal([]byte(out), &item); err != nil {
		return nil, fmt.Errorf("get PR %d: parse JSON: %w", number, err)
	}

	return item.toPR(), nil
}

func (c *Client) MergePR(ctx context.Context, number int, strategy string) error {
	flag := "--merge"
	switch strategy {
	case "squash":
		flag = "--squash"
	case "rebase":
		flag = "--rebase"
	}

	_, err := c.runGH(ctx, "pr", "merge", fmt.Sprintf("%d", number), flag)
	if err != nil {
		return fmt.Errorf("merge PR %d: %w", number, err)
	}
	return nil
}

func (c *Client) ReviewPR(ctx context.Context, number int, body string, approve bool) error {
	reviewFlag := "--request-changes"
	if approve {
		reviewFlag = "--approve"
	}

	args := []string{"pr", "review", fmt.Sprintf("%d", number), reviewFlag, "--body", body}
	_, err := c.runGH(ctx, args...)
	if err != nil {
		return fmt.Errorf("review PR %d: %w", number, err)
	}
	return nil
}

func atoi(s string) (int, error) {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", s, err)
	}
	return n, nil
}
