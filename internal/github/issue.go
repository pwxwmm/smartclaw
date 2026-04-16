package github

import (
	"context"
	"encoding/json"
	"fmt"
)

type IssueOptions struct {
	Title  string
	Body   string
	Labels []string
}

type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	HTMLURL string `json:"url"`
	Author  string `json:"author"`
}

type issueAuthor struct {
	Login string `json:"login"`
}

type issueJSON struct {
	Number  int         `json:"number"`
	Title   string      `json:"title"`
	State   string      `json:"state"`
	HTMLURL string      `json:"url"`
	Author  issueAuthor `json:"author"`
}

func (i issueJSON) toIssue() *Issue {
	return &Issue{
		Number:  i.Number,
		Title:   i.Title,
		State:   i.State,
		HTMLURL: i.HTMLURL,
		Author:  i.Author.Login,
	}
}

func (c *Client) CreateIssue(ctx context.Context, opts *IssueOptions) (*Issue, error) {
	args := []string{"issue", "create",
		"--title", opts.Title,
		"--body", opts.Body,
		"--json", "number,title,state,url,author",
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	var item issueJSON
	if err := json.Unmarshal([]byte(out), &item); err != nil {
		return nil, fmt.Errorf("create issue: parse JSON: %w", err)
	}

	return item.toIssue(), nil
}

func (c *Client) ListIssues(ctx context.Context, state string) ([]*Issue, error) {
	args := []string{"issue", "list", "--json", "number,title,state,url,author"}
	if state != "" {
		args = append(args, "--state", state)
	}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	var items []issueJSON
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		return nil, fmt.Errorf("list issues: parse JSON: %w", err)
	}

	result := make([]*Issue, len(items))
	for i, item := range items {
		result[i] = item.toIssue()
	}
	return result, nil
}

func (c *Client) CommentIssue(ctx context.Context, number int, body string) error {
	_, err := c.runGH(ctx, "issue", "comment", fmt.Sprintf("%d", number), "--body", body)
	if err != nil {
		return fmt.Errorf("comment issue %d: %w", number, err)
	}
	return nil
}
