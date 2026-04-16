package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Workflow struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}

type WorkflowRun struct {
	ID         int    `json:"databaseId"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"url"`
}

func (c *Client) ListWorkflows(ctx context.Context) ([]*Workflow, error) {
	out, err := c.runGH(ctx, "workflow", "list", "--json", "id,name,state")
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	var workflows []*Workflow
	if err := json.Unmarshal([]byte(out), &workflows); err != nil {
		return nil, fmt.Errorf("list workflows: parse JSON: %w", err)
	}

	return workflows, nil
}

func (c *Client) TriggerWorkflow(ctx context.Context, workflowID string, ref string, inputs map[string]string) error {
	args := []string{"workflow", "run", workflowID}
	if ref != "" {
		args = append(args, "--ref", ref)
	}
	for key, value := range inputs {
		args = append(args, "-f", fmt.Sprintf("%s=%s", key, value))
	}

	_, err := c.runGH(ctx, args...)
	if err != nil {
		return fmt.Errorf("trigger workflow %s: %w", workflowID, err)
	}
	return nil
}

func (c *Client) ListRuns(ctx context.Context, workflowID string) ([]*WorkflowRun, error) {
	args := []string{"run", "list", "--json", "databaseId,name,status,conclusion,url"}
	if workflowID != "" {
		args = append(args, "--workflow", workflowID)
	}

	out, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}

	var runs []*WorkflowRun
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &runs); err != nil {
		return nil, fmt.Errorf("list runs: parse JSON: %w", err)
	}

	return runs, nil
}
