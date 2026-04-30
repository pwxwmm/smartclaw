package rl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type DaytonaBackend struct {
	apiURL     string
	apiKey     string
	httpClient *http.Client
}

func NewDaytonaBackend(apiURL, apiKey string) *DaytonaBackend {
	return &DaytonaBackend{
		apiURL: apiURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (db *DaytonaBackend) Name() string {
	return "daytona"
}

func (db *DaytonaBackend) CreateWorkspace(ctx context.Context, config WorkspaceConfig) (string, error) {
	body, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("daytona: create workspace: marshal config: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, db.apiURL+"/workspace", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("daytona: create workspace: %w", err)
	}
	db.setHeaders(req)

	resp, err := db.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("daytona: create workspace: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return "", fmt.Errorf("daytona: create workspace: %w", err)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("daytona: create workspace: decode response: %w", err)
	}

	return result.ID, nil
}

func (db *DaytonaBackend) ExecuteTask(ctx context.Context, workspaceID string, task TaskSpec) (*TaskResult, error) {
	body, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("daytona: execute task: marshal spec: %w", err)
	}

	url := fmt.Sprintf("%s/workspace/%s/execute", db.apiURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("daytona: execute task: %w", err)
	}
	db.setHeaders(req)

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daytona: execute task: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("daytona: execute task: %w", err)
	}

	var result TaskResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("daytona: execute task: decode response: %w", err)
	}

	result.WorkspaceID = workspaceID
	return &result, nil
}

func (db *DaytonaBackend) DestroyWorkspace(ctx context.Context, workspaceID string) error {
	url := fmt.Sprintf("%s/workspace/%s", db.apiURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("daytona: destroy workspace: %w", err)
	}
	db.setHeaders(req)

	resp, err := db.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("daytona: destroy workspace: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return fmt.Errorf("daytona: destroy workspace: %w", err)
	}

	return nil
}

func (db *DaytonaBackend) WorkspaceStatus(ctx context.Context, workspaceID string) (*WorkspaceStatus, error) {
	url := fmt.Sprintf("%s/workspace/%s", db.apiURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("daytona: workspace status: %w", err)
	}
	db.setHeaders(req)

	resp, err := db.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daytona: workspace status: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("daytona: workspace status: %w", err)
	}

	var status WorkspaceStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("daytona: workspace status: decode response: %w", err)
	}

	return &status, nil
}

func (db *DaytonaBackend) ListWorkspaces(ctx context.Context) ([]WorkspaceInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, db.apiURL+"/workspace", nil)
	if err != nil {
		return nil, fmt.Errorf("daytona: list workspaces: %w", err)
	}
	db.setHeaders(req)

	resp, err := db.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("daytona: list workspaces: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, fmt.Errorf("daytona: list workspaces: %w", err)
	}

	var workspaces []WorkspaceInfo
	if err := json.NewDecoder(resp.Body).Decode(&workspaces); err != nil {
		return nil, fmt.Errorf("daytona: list workspaces: decode response: %w", err)
	}

	return workspaces, nil
}

func (db *DaytonaBackend) RunEpisodeRemote(ctx context.Context, envConfig EnvironmentConfig, taskPrompt string) (*EpisodeResult, error) {
	wsID, err := db.CreateWorkspace(ctx, WorkspaceConfig{
		Name:     "rl-episode",
		Timeout:  envConfig.Timeout,
		Labels:   map[string]string{"type": "rl-eval"},
	})
	if err != nil {
		return nil, fmt.Errorf("daytona: run episode remote: %w", err)
	}

	defer func() {
		destroyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = db.DestroyWorkspace(destroyCtx, wsID)
	}()

	args := []string{
		"rl-eval",
		"--task", taskPrompt,
		"--metric", envConfig.SuccessMetric,
		"--max-steps", fmt.Sprintf("%d", envConfig.MaxSteps),
		"--output", "json",
	}

	result, err := db.ExecuteTask(ctx, wsID, TaskSpec{
		Command: "smartclaw",
		Args:    args,
		Timeout: envConfig.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("daytona: run episode remote: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("daytona: run episode remote: exit code %d: %s", result.ExitCode, result.Stderr)
	}

	var episode EpisodeResult
	if err := json.Unmarshal([]byte(result.Stdout), &episode); err != nil {
		return nil, fmt.Errorf("daytona: run episode remote: parse result: %w", err)
	}

	return &episode, nil
}

func (db *DaytonaBackend) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+db.apiKey)
}

type httpError struct {
	StatusCode int
	Body       string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return &httpError{StatusCode: resp.StatusCode, Body: string(body)}
}
