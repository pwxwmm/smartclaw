package sandbox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/utils"
)

var (
	defaultTimeout   = 300 * time.Second
	defaultMaxCalls  = 50
	defaultMaxOutput = 100 * 1024 // 100KB
	allowedTools     = []string{"read_file", "write_file", "glob", "grep", "bash", "web_search", "web_fetch"}
	stripEnvSubstrs  = []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL", "PASSWD", "AUTH"}
	safeEnvVars      = []string{"PATH", "HOME", "LANG", "SHELL", "TMPDIR", "USER", "TERM"}
)

type CodeSandboxConfig struct {
	Timeout    time.Duration
	MaxCalls   int
	MaxOutput  int
	WorkDir    string
	AllowTools []string
}

func DefaultCodeSandboxConfig() CodeSandboxConfig {
	return CodeSandboxConfig{
		Timeout:    defaultTimeout,
		MaxCalls:   defaultMaxCalls,
		MaxOutput:  defaultMaxOutput,
		AllowTools: allowedTools,
	}
}

type CodeSandbox struct {
	config    CodeSandboxConfig
	toolCalls int
	mu        sync.Mutex
	output    strings.Builder
}

func NewCodeSandbox(config CodeSandboxConfig) *CodeSandbox {
	return &CodeSandbox{config: config}
}

type rpcRequest struct {
	ID    string         `json:"id"`
	Tool  string         `json:"tool"`
	Input map[string]any `json:"input"`
}

type rpcResponse struct {
	ID     string `json:"id"`
	Result any    `json:"result"`
	Error  string `json:"error,omitempty"`
}

type ExecuteResult struct {
	Stdout    string        `json:"stdout"`
	ExitCode  int           `json:"exit_code"`
	Duration  time.Duration `json:"duration"`
	ToolCalls int           `json:"tool_calls"`
	Truncated bool          `json:"truncated,omitempty"`
	TimedOut  bool          `json:"timed_out,omitempty"`
}

func (cs *CodeSandbox) Execute(ctx context.Context, code string, language string, toolHandler func(tool string, input map[string]any) (any, error)) (*ExecuteResult, error) {
	if language == "" {
		language = "python"
	}

	switch language {
	case "python", "py":
		return cs.executePython(ctx, code, toolHandler)
	case "go":
		return cs.executeGo(ctx, code, toolHandler)
	default:
		return cs.executePython(ctx, code, toolHandler)
	}
}

func (cs *CodeSandbox) executePython(ctx context.Context, code string, toolHandler func(tool string, input map[string]any) (any, error)) (*ExecuteResult, error) {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("smartclaw-rpc-%d.sock", time.Now().UnixNano()))
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("smartclaw-code-%d", time.Now().UnixNano()))
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	scriptPath := filepath.Join(tmpDir, "script.py")
	stubPath := filepath.Join(tmpDir, "smartclaw_tools.py")

	stubCode := generatePythonStub(socketPath, cs.config.AllowTools)
	if err := os.WriteFile(stubPath, []byte(stubCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write stub: %w", err)
	}
	if err := os.WriteFile(scriptPath, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write script: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC socket: %w", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	rpcCtx, rpcCancel := context.WithCancel(ctx)
	defer rpcCancel()

	utils.Go(func() { cs.serveRPC(rpcCtx, listener, toolHandler) })

	cmd := exec.CommandContext(ctx, "python3", scriptPath)
	cmd.Dir = cs.config.WorkDir
	if cmd.Dir == "" {
		cmd.Dir = tmpDir
	}
	cmd.Env = cs.buildEnv()

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start python: %w", err)
	}

	var stdout strings.Builder
	var wg sync.WaitGroup
	wg.Add(1)
	utils.Go(func() {
		defer wg.Done()
		io.Copy(&stdout, stdoutPipe)
	})
	utils.Go(func() { io.Copy(io.Discard, stderrPipe) })

	done := make(chan error, 1)
	utils.Go(func() {
		wg.Wait()
		done <- cmd.Wait()
	})

	var exitCode int
	timedOut := false

	select {
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
	case <-time.After(cs.config.Timeout):
		cmd.Process.Kill()
		timedOut = true
		exitCode = -1
	}

	duration := time.Since(startTime)

	output := stdout.String()
	truncated := len(output) > cs.config.MaxOutput
	if truncated {
		output = output[:cs.config.MaxOutput] + "\n... (output truncated)"
	}

	cs.mu.Lock()
	calls := cs.toolCalls
	cs.mu.Unlock()

	return &ExecuteResult{
		Stdout:    output,
		ExitCode:  exitCode,
		Duration:  duration,
		ToolCalls: calls,
		Truncated: truncated,
		TimedOut:  timedOut,
	}, nil
}

func (cs *CodeSandbox) executeGo(ctx context.Context, code string, toolHandler func(tool string, input map[string]any) (any, error)) (*ExecuteResult, error) {
	tmpDir, err := os.MkdirTemp("", "smartclaw-go-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	srcFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(srcFile, []byte(code), 0644); err != nil {
		return nil, fmt.Errorf("failed to write source: %w", err)
	}

	goCtx, cancel := context.WithTimeout(ctx, cs.config.Timeout)
	defer cancel()

	cmd := exec.CommandContext(goCtx, "go", "run", srcFile)
	cmd.Dir = tmpDir
	cmd.Env = cs.buildEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)

	exitCode := 0
	timedOut := false
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			if goCtx.Err() == context.DeadlineExceeded {
				timedOut = true
				exitCode = -1
			} else {
				exitCode = 1
			}
		}
	}

	output := stdout.String()
	if stderr.Len() > 0 && exitCode != 0 {
		output = stdout.String() + "\n" + stderr.String()
	}

	truncated := len(output) > cs.config.MaxOutput
	if truncated {
		output = output[:cs.config.MaxOutput] + "\n... (output truncated)"
	}

	return &ExecuteResult{
		ExitCode:  exitCode,
		Stdout:    output,
		Duration:  duration,
		TimedOut:  timedOut,
		Truncated: truncated,
	}, nil
}

func (cs *CodeSandbox) serveRPC(ctx context.Context, listener net.Listener, toolHandler func(tool string, input map[string]any) (any, error)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			return
		}

		utils.Go(func() { cs.handleConn(ctx, conn, toolHandler) })
	}
}

func (cs *CodeSandbox) handleConn(ctx context.Context, conn net.Conn, toolHandler func(tool string, input map[string]any) (any, error)) {
	defer conn.Close()

	cs.mu.Lock()
	if cs.toolCalls >= cs.config.MaxCalls {
		cs.mu.Unlock()
		resp := rpcResponse{Error: "max tool calls exceeded"}
		json.NewEncoder(conn).Encode(resp)
		return
	}
	cs.toolCalls++
	cs.mu.Unlock()

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return
	}

	var req rpcRequest
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &req); err != nil {
		resp := rpcResponse{ID: "", Error: "invalid request"}
		json.NewEncoder(conn).Encode(resp)
		return
	}

	if !cs.isToolAllowed(req.Tool) {
		resp := rpcResponse{ID: req.ID, Error: fmt.Sprintf("tool %q not allowed in sandbox", req.Tool)}
		json.NewEncoder(conn).Encode(resp)
		return
	}

	result, err := toolHandler(req.Tool, req.Input)
	resp := rpcResponse{ID: req.ID}
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Result = result
	}

	json.NewEncoder(conn).Encode(resp)
}

func (cs *CodeSandbox) isToolAllowed(tool string) bool {
	for _, allowed := range cs.config.AllowTools {
		if tool == allowed {
			return true
		}
	}
	return false
}

func (cs *CodeSandbox) buildEnv() []string {
	currentEnv := os.Environ()
	var safeEnv []string

	for _, env := range currentEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		if cs.isSafeEnvVar(key) {
			safeEnv = append(safeEnv, env)
		}
	}

	safeEnv = append(safeEnv, "PYTHONPATH="+filepath.Dir(safeEnv[0]))
	return safeEnv
}

func (cs *CodeSandbox) isSafeEnvVar(key string) bool {
	for _, safe := range safeEnvVars {
		if key == safe {
			return true
		}
	}
	upperKey := strings.ToUpper(key)
	for _, substr := range stripEnvSubstrs {
		if strings.Contains(upperKey, substr) {
			return false
		}
	}
	return false
}

func generatePythonStub(socketPath string, tools []string) string {
	var funcDefs strings.Builder
	for _, tool := range tools {
		pythonName := strings.ReplaceAll(tool, "-", "_")
		funcDefs.WriteString(fmt.Sprintf(`
def %s(**kwargs):
    return _call_tool("%s", kwargs)
`, pythonName, tool))
	}

	return fmt.Sprintf(`import json
import socket

_SOCKET_PATH = %q

def _call_tool(tool_name, input_args):
    req = {"id": "1", "tool": tool_name, "input": input_args}
    try:
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(_SOCKET_PATH)
        sock.sendall((json.dumps(req) + "\\n").encode())
        data = b""
        while True:
            chunk = sock.recv(4096)
            if not chunk:
                break
            data += chunk
        sock.close()
        resp = json.loads(data.decode())
        if resp.get("error"):
            raise RuntimeError(resp["error"])
        return resp.get("result")
    except Exception as e:
        raise RuntimeError(f"RPC call failed: {e}")

%s
`, socketPath, funcDefs.String())
}
