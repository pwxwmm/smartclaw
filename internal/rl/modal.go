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

const defaultModalAPIURL = "https://api.modal.com/v1"

// ModalFunction describes a function to deploy on the Modal platform.
type ModalFunction struct {
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	Code         string   `json:"code"`
	Requirements []string `json:"requirements,omitempty"`
}

// ModalBackend implements ServerlessBackend for the Modal serverless platform.
// Modal maps "workspace" → "app" and "task" → "function call".
type ModalBackend struct {
	apiURL     string
	apiKey     string
	httpClient *http.Client
	appID      string
}

// NewModalBackend creates a new Modal serverless backend.
// If apiURL is empty, it defaults to "https://api.modal.com/v1".
func NewModalBackend(apiURL, apiKey, appID string) *ModalBackend {
	if apiURL == "" {
		apiURL = defaultModalAPIURL
	}
	return &ModalBackend{
		apiURL: apiURL,
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		appID: appID,
	}
}

// Name returns the backend identifier.
func (mb *ModalBackend) Name() string {
	return "modal"
}

// CreateWorkspace creates a new Modal app (workspace).
// POST {apiURL}/apps with {"name": config.Name, "env": config.Env}
func (mb *ModalBackend) CreateWorkspace(ctx context.Context, config WorkspaceConfig) (string, error) {
	payload := map[string]any{
		"name": config.Name,
		"env":  config.Env,
	}
	if config.Image != "" {
		payload["image"] = config.Image
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("modal: create app: marshal: %w", err)
	}

	var result struct {
		AppID string `json:"app_id"`
	}
	if err := mb.doRequest(ctx, http.MethodPost, "/apps", body, &result); err != nil {
		return "", fmt.Errorf("modal: create app: %w", err)
	}

	return result.AppID, nil
}

// ExecuteTask runs a task in the specified workspace (app).
// POST {apiURL}/apps/{workspaceID}/run with the task spec.
func (mb *ModalBackend) ExecuteTask(ctx context.Context, workspaceID string, task TaskSpec) (*TaskResult, error) {
	payload := map[string]any{
		"command": task.Command,
		"env":     task.Env,
	}
	if len(task.Args) > 0 {
		payload["args"] = task.Args
	}
	if task.Timeout > 0 {
		payload["timeout"] = task.Timeout.Seconds()
	}
	if task.WorkDir != "" {
		payload["work_dir"] = task.WorkDir
	}
	if len(task.InputData) > 0 {
		payload["input_data"] = task.InputData
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("modal: execute task: marshal: %w", err)
	}

	path := fmt.Sprintf("/apps/%s/run", workspaceID)
	var result TaskResult
	if err := mb.doRequest(ctx, http.MethodPost, path, body, &result); err != nil {
		return nil, fmt.Errorf("modal: execute task: %w", err)
	}

	result.WorkspaceID = workspaceID
	return &result, nil
}

// DestroyWorkspace tears down a Modal app (workspace).
// DELETE {apiURL}/apps/{workspaceID}
func (mb *ModalBackend) DestroyWorkspace(ctx context.Context, workspaceID string) error {
	path := fmt.Sprintf("/apps/%s", workspaceID)
	if err := mb.doRequest(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("modal: destroy app: %w", err)
	}
	return nil
}

// WorkspaceStatus returns the current status of a Modal app.
// GET {apiURL}/apps/{workspaceID}
func (mb *ModalBackend) WorkspaceStatus(ctx context.Context, workspaceID string) (*WorkspaceStatus, error) {
	path := fmt.Sprintf("/apps/%s", workspaceID)
	var result WorkspaceStatus
	if err := mb.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, fmt.Errorf("modal: workspace status: %w", err)
	}
	return &result, nil
}

// ListWorkspaces returns all active Modal apps.
// GET {apiURL}/apps
func (mb *ModalBackend) ListWorkspaces(ctx context.Context) ([]WorkspaceInfo, error) {
	var result []WorkspaceInfo
	if err := mb.doRequest(ctx, http.MethodGet, "/apps", nil, &result); err != nil {
		return nil, fmt.Errorf("modal: list workspaces: %w", err)
	}
	return result, nil
}

// DeployFunction deploys a Python/Go function to Modal.
// POST {apiURL}/functions with the function spec, returns the function ID.
func (mb *ModalBackend) DeployFunction(ctx context.Context, fn ModalFunction) (string, error) {
	body, err := json.Marshal(fn)
	if err != nil {
		return "", fmt.Errorf("modal: deploy function: marshal: %w", err)
	}

	var result struct {
		FunctionID string `json:"function_id"`
	}
	if err := mb.doRequest(ctx, http.MethodPost, "/functions", body, &result); err != nil {
		return "", fmt.Errorf("modal: deploy function: %w", err)
	}

	return result.FunctionID, nil
}

// InvokeFunction invokes a deployed Modal function with input data.
// POST {apiURL}/functions/{functionID}/invoke with input, returns TaskResult.
func (mb *ModalBackend) InvokeFunction(ctx context.Context, functionID string, input []byte) (*TaskResult, error) {
	payload := map[string]any{
		"input": input,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("modal: invoke function: marshal: %w", err)
	}

	path := fmt.Sprintf("/functions/%s/invoke", functionID)
	var result TaskResult
	if err := mb.doRequest(ctx, http.MethodPost, path, body, &result); err != nil {
		return nil, fmt.Errorf("modal: invoke function: %w", err)
	}

	return &result, nil
}

// RunEpisodeRemote runs a full RL episode remotely on Modal.
// It creates a Modal app, deploys an RL evaluation function, invokes it with
// the task prompt, parses the EpisodeResult, and cleans up the app.
func (mb *ModalBackend) RunEpisodeRemote(ctx context.Context, envConfig EnvironmentConfig, taskPrompt string) (*EpisodeResult, error) {
	wsConfig := WorkspaceConfig{
		Name:    fmt.Sprintf("rl-episode-%d", time.Now().UnixNano()),
		Timeout: envConfig.Timeout,
		Labels: map[string]string{
			"type":   "rl-episode",
			"metric": envConfig.SuccessMetric,
		},
	}

	workspaceID, err := mb.CreateWorkspace(ctx, wsConfig)
	if err != nil {
		return nil, fmt.Errorf("modal: run episode: %w", err)
	}

	// Ensure cleanup on any exit path
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = mb.DestroyWorkspace(cleanupCtx, workspaceID)
	}()

	// Deploy the RL evaluation function
	evalFn := ModalFunction{
		Name:  "rl-eval",
		Image: "python:3.11-slim",
		Code: fmt.Sprintf(
			`smartclaw rl-eval --task %q --metric %s --max-steps %d --output json`,
			taskPrompt, envConfig.SuccessMetric, envConfig.MaxSteps,
		),
		Requirements: []string{"smartclaw"},
	}

	functionID, err := mb.DeployFunction(ctx, evalFn)
	if err != nil {
		return nil, fmt.Errorf("modal: run episode: deploy: %w", err)
	}

	// Invoke the evaluation function
	taskResult, err := mb.InvokeFunction(ctx, functionID, []byte(taskPrompt))
	if err != nil {
		return nil, fmt.Errorf("modal: run episode: invoke: %w", err)
	}

	// Parse the EpisodeResult from the task output
	var episode EpisodeResult
	if err := json.Unmarshal([]byte(taskResult.Stdout), &episode); err != nil {
		return nil, fmt.Errorf("modal: run episode: parse result: %w", err)
	}

	return &episode, nil
}

// doRequest executes an HTTP request against the Modal API.
// If out is non-nil, the response body is JSON-decoded into out.
func (mb *ModalBackend) doRequest(ctx context.Context, method, path string, body []byte, out any) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, mb.apiURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+mb.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := mb.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
