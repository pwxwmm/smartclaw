package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type DockerSandboxTool struct{ BaseTool }

func (t *DockerSandboxTool) Name() string { return "docker_exec" }
func (t *DockerSandboxTool) Description() string {
	return "Execute a command inside an isolated Docker container. The project directory is mounted at /workspace. Supports both one-shot and session-persistent containers."
}

func (t *DockerSandboxTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to execute inside the container",
			},
			"image": map[string]any{
				"type":        "string",
				"default":     "smartclaw/runtime:latest",
				"description": "Docker image to use",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"default":     120,
				"description": "Timeout in seconds",
			},
			"session": map[string]any{
				"type":        "string",
				"description": "Session ID for persistent container. If set, reuses an existing container for this session. Omit for one-shot execution.",
			},
			"action": map[string]any{
				"type":        "string",
				"default":     "exec",
				"description": "Action: exec (run command), start (create session container), stop (destroy session container)",
			},
		},
		"required": []string{"command"},
	}
}

func (t *DockerSandboxTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker is not installed or not in PATH")
	}

	action, _ := input["action"].(string)
	if action == "" {
		action = "exec"
	}

	sessionID, _ := input["session"].(string)

	switch action {
	case "start":
		return t.startSession(ctx, input, sessionID)
	case "stop":
		return t.stopSession(ctx, sessionID)
	default:
		if sessionID != "" {
			return t.execInSession(ctx, input, sessionID)
		}
		return t.execOneShot(ctx, input)
	}
}

func (t *DockerSandboxTool) execOneShot(ctx context.Context, input map[string]any) (any, error) {
	command, _ := input["command"].(string)
	if command == "" {
		return nil, ErrRequiredField("command")
	}

	image, _ := input["image"].(string)
	if image == "" {
		image = "smartclaw/runtime:latest"
	}

	timeoutSec := 120
	if ts, ok := input["timeout"].(int); ok && ts > 0 {
		timeoutSec = ts
	}

	workDir, _ := os.Getwd()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	containerName := fmt.Sprintf("smartclaw-exec-%d", time.Now().UnixNano())

	createCmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", containerName,
		"-v", workDir+":/workspace",
		"-w", "/workspace",
		"--network", "host",
		"--rm",
		image,
		"sleep", "infinity",
	)
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker create failed: %w (%s)", err, strings.TrimSpace(string(createOutput)))
	}

	defer func() {
		killCmd := exec.Command("docker", "kill", containerName)
		if err := killCmd.Run(); err != nil {
			slog.Warn("failed to kill container", "error", err, "container", containerName)
		}
	}()

	return execInContainer(ctx, containerName, command)
}

func (t *DockerSandboxTool) startSession(ctx context.Context, input map[string]any, sessionID string) (any, error) {
	if sessionID == "" {
		sessionID = fmt.Sprintf("sc-session-%d", time.Now().UnixNano())
	}

	image, _ := input["image"].(string)
	if image == "" {
		image = "smartclaw/runtime:latest"
	}

	containerName := "smartclaw-" + sessionID

	if existing := getContainerState(containerName); existing == "running" {
		return map[string]any{
			"session_id":   sessionID,
			"container_id": containerName,
			"status":       "already_running",
		}, nil
	}

	workDir, _ := os.Getwd()

	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", containerName,
		"-v", workDir+":/workspace",
		"-w", "/workspace",
		"--network", "host",
		image,
		"sleep", "infinity",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker session start failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}

	sessions.Store(sessionID, containerName)
	slog.Info("docker: session container started", "session", sessionID, "container", containerName)

	return map[string]any{
		"session_id":   sessionID,
		"container_id": containerName,
		"status":       "started",
		"image":        image,
	}, nil
}

func (t *DockerSandboxTool) stopSession(ctx context.Context, sessionID string) (any, error) {
	if sessionID == "" {
		return nil, ErrRequiredField("session")
	}

	containerName, ok := sessions.Load(sessionID)
	if !ok {
		containerName = "smartclaw-" + sessionID
	}

	killCmd := exec.CommandContext(ctx, "docker", "kill", fmt.Sprint(containerName))
	if err := killCmd.Run(); err != nil {
		slog.Warn("failed to kill container", "error", err, "container", containerName)
	}

	rmCmd := exec.CommandContext(ctx, "docker", "rm", "-f", fmt.Sprint(containerName))
	if err := rmCmd.Run(); err != nil {
		slog.Warn("failed to remove container", "error", err, "container", containerName)
	}

	sessions.Delete(sessionID)
	slog.Info("docker: session container stopped", "session", sessionID)

	return map[string]any{
		"session_id": sessionID,
		"status":     "stopped",
	}, nil
}

func (t *DockerSandboxTool) execInSession(ctx context.Context, input map[string]any, sessionID string) (any, error) {
	command, _ := input["command"].(string)
	if command == "" {
		return nil, ErrRequiredField("command")
	}

	timeoutSec := 120
	if ts, ok := input["timeout"].(int); ok && ts > 0 {
		timeoutSec = ts
	}

	containerName, ok := sessions.Load(sessionID)
	if !ok {
		containerName = "smartclaw-" + sessionID
	}

	state := getContainerState(fmt.Sprint(containerName))
	if state != "running" {
		return nil, fmt.Errorf("session container %q is not running (state: %s). Use action=start first.", sessionID, state)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	return execInContainer(ctx, fmt.Sprint(containerName), command)
}

func execInContainer(ctx context.Context, containerName, command string) (any, error) {
	startTime := time.Now()

	execCmd := exec.CommandContext(ctx, "docker", "exec", containerName, "bash", "-c", command)
	output, err := execCmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	timedOut := ctx.Err() == context.DeadlineExceeded

	return map[string]any{
		"stdout":    string(output),
		"exit_code": exitCode,
		"duration":  time.Since(startTime).String(),
		"timed_out": timedOut,
	}, nil
}

var sessions sync.Map

func getContainerState(containerName string) string {
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "not_found"
	}
	return strings.TrimSpace(string(output))
}
