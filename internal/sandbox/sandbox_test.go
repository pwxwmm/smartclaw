package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	config := Config{
		Enabled:            true,
		NamespaceIsolation: false,
		NetworkIsolation:   false,
		FilesystemMode:     "off",
	}
	mgr := NewManager(config)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManagerGetStatus(t *testing.T) {
	config := Config{
		Enabled:            true,
		NamespaceIsolation: false,
		NetworkIsolation:   true,
		FilesystemMode:     "readonly",
		AllowedMounts:      []string{"/tmp"},
	}
	mgr := NewManager(config)

	status := mgr.GetStatus()

	if !status.Enabled {
		t.Error("Expected Enabled=true")
	}

	if status.NamespaceIsolation {
		t.Error("Expected NamespaceIsolation=false")
	}

	if !status.NetworkIsolation {
		t.Error("Expected NetworkIsolation=true")
	}

	if status.FilesystemMode != "readonly" {
		t.Errorf("Expected FilesystemMode 'readonly', got '%s'", status.FilesystemMode)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled {
		t.Error("Default config should have Enabled=false")
	}

	if config.NamespaceIsolation {
		t.Error("Default config should have NamespaceIsolation=false")
	}

	if config.NetworkIsolation {
		t.Error("Default config should have NetworkIsolation=false")
	}

	if config.FilesystemMode != "off" {
		t.Errorf("Expected FilesystemMode 'off', got '%s'", config.FilesystemMode)
	}

	if len(config.AllowedMounts) != 0 {
		t.Errorf("Expected empty AllowedMounts, got %v", config.AllowedMounts)
	}
}

func TestStatusString(t *testing.T) {
	status := Status{
		Enabled:            true,
		NamespaceIsolation: false,
		NetworkIsolation:   false,
		FilesystemMode:     "off",
		Platform:           "darwin",
		ContainerEnv:       false,
	}

	str := status.String()
	if str == "" {
		t.Error("Status String() should not be empty")
	}
	if !strings.Contains(str, "Enabled: true") {
		t.Error("Status String() should contain Enabled field")
	}
	if !strings.Contains(str, "Platform: darwin") {
		t.Error("Status String() should contain Platform field")
	}
}

func TestIsLinux(t *testing.T) {
	_ = IsLinux()
}

func TestSupportsNamespaces(t *testing.T) {
	_ = SupportsNamespaces()
}

func TestNewManagerDefaultFields(t *testing.T) {
	mgr := NewManager(Config{})
	status := mgr.GetStatus()
	_ = status.ContainerEnv
	if status.Platform == "" {
		t.Error("Platform should not be empty")
	}
}

func TestManagerWrapCommand(t *testing.T) {
	mgr := NewManager(Config{Enabled: true})
	cmd := exec.Command("echo", "hello")
	err := mgr.WrapCommand(cmd)
	if err != nil {
		t.Errorf("WrapCommand should not error on non-linux, got: %v", err)
	}
}

func TestManagerWrapCommandDisabled(t *testing.T) {
	mgr := NewManager(Config{Enabled: false})
	cmd := exec.Command("echo", "hello")
	err := mgr.WrapCommand(cmd)
	if err != nil {
		t.Errorf("WrapCommand with disabled config should not error, got: %v", err)
	}
}

func TestRunSandboxed(t *testing.T) {
	cmd := exec.Command("echo", "test")
	err := RunSandboxed(cmd, Config{})
	if err != nil {
		t.Errorf("RunSandboxed on non-linux should just run cmd, got: %v", err)
	}
}

func TestDetectContainer(t *testing.T) {
	mgr := NewManager(Config{})
	_ = mgr.detectContainer()
}

func TestManagerGetStatusAllFields(t *testing.T) {
	config := Config{
		Enabled:            true,
		NamespaceIsolation: true,
		NetworkIsolation:   true,
		FilesystemMode:     "readonly",
		AllowedMounts:      []string{"/tmp", "/var"},
	}
	mgr := NewManager(config)
	status := mgr.GetStatus()

	if !status.Enabled {
		t.Error("Expected Enabled=true")
	}
	if !status.NamespaceIsolation {
		t.Error("Expected NamespaceIsolation=true")
	}
	if !status.NetworkIsolation {
		t.Error("Expected NetworkIsolation=true")
	}
	if status.FilesystemMode != "readonly" {
		t.Errorf("Expected FilesystemMode 'readonly', got '%s'", status.FilesystemMode)
	}
}

func TestDefaultCodeSandboxConfig(t *testing.T) {
	config := DefaultCodeSandboxConfig()

	if config.Timeout != defaultTimeout {
		t.Errorf("Expected Timeout=%v, got %v", defaultTimeout, config.Timeout)
	}
	if config.MaxCalls != defaultMaxCalls {
		t.Errorf("Expected MaxCalls=%d, got %d", defaultMaxCalls, config.MaxCalls)
	}
	if config.MaxOutput != defaultMaxOutput {
		t.Errorf("Expected MaxOutput=%d, got %d", defaultMaxOutput, config.MaxOutput)
	}
	if len(config.AllowTools) != len(allowedTools) {
		t.Errorf("Expected %d allowed tools, got %d", len(allowedTools), len(config.AllowTools))
	}
	for i, tool := range config.AllowTools {
		if tool != allowedTools[i] {
			t.Errorf("AllowedTools[%d] = %q, want %q", i, tool, allowedTools[i])
		}
	}
}

func TestDefaultCodeSandboxConfigTimeout(t *testing.T) {
	config := DefaultCodeSandboxConfig()
	if config.Timeout != 300*time.Second {
		t.Errorf("Expected 300s timeout, got %v", config.Timeout)
	}
}

func TestDefaultCodeSandboxConfigMaxCalls(t *testing.T) {
	config := DefaultCodeSandboxConfig()
	if config.MaxCalls != 50 {
		t.Errorf("Expected MaxCalls=50, got %d", config.MaxCalls)
	}
}

func TestDefaultCodeSandboxConfigMaxOutput(t *testing.T) {
	config := DefaultCodeSandboxConfig()
	if config.MaxOutput != 100*1024 {
		t.Errorf("Expected MaxOutput=%d, got %d", 100*1024, config.MaxOutput)
	}
}

func TestNewCodeSandboxDefault(t *testing.T) {
	config := DefaultCodeSandboxConfig()
	cs := NewCodeSandbox(config)
	if cs == nil {
		t.Fatal("NewCodeSandbox returned nil")
	}
	if cs.config.Timeout != config.Timeout {
		t.Error("Config not set correctly")
	}
}

func TestNewCodeSandboxCustom(t *testing.T) {
	config := CodeSandboxConfig{
		Timeout:    10 * time.Second,
		MaxCalls:   5,
		MaxOutput:  1024,
		WorkDir:    "/tmp",
		AllowTools: []string{"read_file", "bash"},
	}
	cs := NewCodeSandbox(config)
	if cs == nil {
		t.Fatal("NewCodeSandbox returned nil")
	}
	if cs.config.Timeout != 10*time.Second {
		t.Errorf("Expected 10s timeout, got %v", cs.config.Timeout)
	}
	if cs.config.MaxCalls != 5 {
		t.Errorf("Expected MaxCalls=5, got %d", cs.config.MaxCalls)
	}
	if cs.config.MaxOutput != 1024 {
		t.Errorf("Expected MaxOutput=1024, got %d", cs.config.MaxOutput)
	}
	if cs.config.WorkDir != "/tmp" {
		t.Errorf("Expected WorkDir=/tmp, got %s", cs.config.WorkDir)
	}
}

func TestNewCodeSandboxZeroConfig(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{})
	if cs == nil {
		t.Fatal("NewCodeSandbox returned nil")
	}
	if cs.config.Timeout != 0 {
		t.Error("Zero config should have zero timeout")
	}
}


func pythonAvailable() bool {
	_, err := exec.LookPath("python3")
	return err == nil
}

func TestCodeSandboxExecutePythonSimple(t *testing.T) {
	if !pythonAvailable() {
		t.Skip("python3 not available")
	}

	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    30 * time.Second,
		MaxCalls:   10,
		MaxOutput:  10 * 1024,
		AllowTools: allowedTools,
	})

	result, err := cs.Execute(context.Background(), "print('hello from sandbox')", "python", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello from sandbox") {
		t.Errorf("Expected stdout to contain 'hello from sandbox', got %q", result.Stdout)
	}
	if result.TimedOut {
		t.Error("Should not have timed out")
	}
	if result.Truncated {
		t.Error("Should not have been truncated")
	}
}

func TestCodeSandboxExecutePythonSyntaxError(t *testing.T) {
	if !pythonAvailable() {
		t.Skip("python3 not available")
	}

	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    10 * time.Second,
		MaxCalls:   10,
		MaxOutput:  10 * 1024,
		AllowTools: allowedTools,
	})

	result, err := cs.Execute(context.Background(), "invalid python syntax !!", "python", nil)
	if err != nil {
		t.Fatalf("Execute should not return error for runtime errors: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for syntax error")
	}
}

func TestCodeSandboxExecutePythonTimeout(t *testing.T) {
	if !pythonAvailable() {
		t.Skip("python3 not available")
	}
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    2 * time.Second,
		MaxCalls:   10,
		MaxOutput:  10 * 1024,
		AllowTools: allowedTools,
	})

	result, err := cs.Execute(context.Background(), "import time; time.sleep(30)", "python", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.TimedOut {
		t.Error("Expected TimedOut=true")
	}
	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1 on timeout, got %d", result.ExitCode)
	}
}

func TestCodeSandboxExecutePythonMaxOutput(t *testing.T) {
	if !pythonAvailable() {
		t.Skip("python3 not available")
	}

	maxOut := 500
	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    30 * time.Second,
		MaxCalls:   10,
		MaxOutput:  maxOut,
		AllowTools: allowedTools,
	})

	code := fmt.Sprintf("print('x' * %d)", maxOut*3)
	result, err := cs.Execute(context.Background(), code, "python", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Truncated {
		t.Error("Expected Truncated=true")
	}
	if len(result.Stdout) > maxOut+50 { // allow some slack for truncation message
		t.Errorf("Output should be truncated to ~%d bytes, got %d", maxOut, len(result.Stdout))
	}
}

func TestCodeSandboxExecutePythonDefaultLanguage(t *testing.T) {
	if !pythonAvailable() {
		t.Skip("python3 not available")
	}

	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    30 * time.Second,
		MaxCalls:   10,
		MaxOutput:  10 * 1024,
		AllowTools: allowedTools,
	})

	result, err := cs.Execute(context.Background(), "print('default lang')", "", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
}

func TestCodeSandboxExecutePythonPyAlias(t *testing.T) {
	if !pythonAvailable() {
		t.Skip("python3 not available")
	}

	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    30 * time.Second,
		MaxCalls:   10,
		MaxOutput:  10 * 1024,
		AllowTools: allowedTools,
	})

	result, err := cs.Execute(context.Background(), "print('py alias')", "py", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
}


func TestCodeSandboxIsToolAllowed(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{
		AllowTools: []string{"read_file", "bash"},
	})

	if !cs.isToolAllowed("read_file") {
		t.Error("read_file should be allowed")
	}
	if !cs.isToolAllowed("bash") {
		t.Error("bash should be allowed")
	}
	if cs.isToolAllowed("delete_everything") {
		t.Error("delete_everything should not be allowed")
	}
	if cs.isToolAllowed("") {
		t.Error("empty tool name should not be allowed")
	}
}

func TestCodeSandboxIsToolAllowedDefaultConfig(t *testing.T) {
	cs := NewCodeSandbox(DefaultCodeSandboxConfig())

	for _, tool := range allowedTools {
		if !cs.isToolAllowed(tool) {
			t.Errorf("Tool %q should be allowed with default config", tool)
		}
	}
}

func TestCodeSandboxIsToolNotAllowed(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{
		AllowTools: []string{},
	})

	if cs.isToolAllowed("read_file") {
		t.Error("No tools should be allowed with empty AllowTools")
	}
}

func TestCodeSandboxBuildEnv(t *testing.T) {
	cs := NewCodeSandbox(DefaultCodeSandboxConfig())
	env := cs.buildEnv()

	if len(env) == 0 {
		t.Error("buildEnv should return at least some env vars")
	}

	hasPythonPath := false
	for _, e := range env {
		if strings.HasPrefix(e, "PYTHONPATH=") {
			hasPythonPath = true
			break
		}
	}
	if !hasPythonPath {
		t.Error("buildEnv should include PYTHONPATH")
	}
}

func TestCodeSandboxIsSafeEnvVar(t *testing.T) {
	cs := NewCodeSandbox(DefaultCodeSandboxConfig())

	safeVars := []string{"PATH", "HOME", "LANG", "SHELL", "TMPDIR", "USER", "TERM"}
	for _, v := range safeVars {
		if !cs.isSafeEnvVar(v) {
			t.Errorf("%q should be a safe env var", v)
		}
	}
}

func TestCodeSandboxIsSafeEnvVarSecret(t *testing.T) {
	cs := NewCodeSandbox(DefaultCodeSandboxConfig())

	unsafeVars := []string{
		"API_KEY", "SECRET_TOKEN", "MY_PASSWORD", "AUTH_HEADER",
		"CREDENTIAL_FILE", "PASSWD_HASH", "AWS_SECRET_KEY",
	}
	for _, v := range unsafeVars {
		if cs.isSafeEnvVar(v) {
			t.Errorf("%q should NOT be a safe env var (contains sensitive substring)", v)
		}
	}
}

func TestCodeSandboxIsSafeEnvVarCaseInsensitive(t *testing.T) {
	cs := NewCodeSandbox(DefaultCodeSandboxConfig())

	if cs.isSafeEnvVar("api_key") {
		t.Error("api_key should not be safe (contains KEY after upper)")
	}
	if cs.isSafeEnvVar("my_token") {
		t.Error("my_token should not be safe (contains TOKEN after upper)")
	}
}

func TestCodeSandboxIsSafeEnvVarNonSensitive(t *testing.T) {
	cs := NewCodeSandbox(DefaultCodeSandboxConfig())

	if cs.isSafeEnvVar("EDITOR") {
		t.Error("EDITOR is not in safeEnvVars and should be denied")
	}
	if cs.isSafeEnvVar("GOPATH") {
		t.Error("GOPATH is not in safeEnvVars and should be denied")
	}
}


func TestRPCRequestParsing(t *testing.T) {
	raw := `{"id":"1","tool":"read_file","input":{"path":"/tmp/test.txt"}}`
	var req rpcRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("Failed to parse rpcRequest: %v", err)
	}
	if req.ID != "1" {
		t.Errorf("Expected ID '1', got %q", req.ID)
	}
	if req.Tool != "read_file" {
		t.Errorf("Expected Tool 'read_file', got %q", req.Tool)
	}
	if req.Input["path"] != "/tmp/test.txt" {
		t.Errorf("Expected input path '/tmp/test.txt', got %v", req.Input["path"])
	}
}

func TestRPCResponseParsing(t *testing.T) {
	resp := rpcResponse{
		ID:     "42",
		Result: "file contents here",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal rpcResponse: %v", err)
	}
	if !strings.Contains(string(data), `"id":"42"`) {
		t.Error("Marshaled response should contain ID")
	}
	if !strings.Contains(string(data), `"result":"file contents here"`) {
		t.Error("Marshaled response should contain result")
	}
}

func TestRPCResponseWithError(t *testing.T) {
	resp := rpcResponse{
		ID:    "err1",
		Error: "tool not allowed in sandbox",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal error response: %v", err)
	}
	if !strings.Contains(string(data), `"error":"tool not allowed in sandbox"`) {
		t.Error("Marshaled error response should contain error field")
	}
}


func runHandleConnTest(t *testing.T, cs *CodeSandbox, req rpcRequest, toolHandler func(tool string, input map[string]any) (any, error)) rpcResponse {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("test-hc-%d.sock", time.Now().UnixNano()))
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		cs.handleConn(ctx, conn, toolHandler)
	}()

	time.Sleep(10 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	data, _ := json.Marshal(req)
	conn.Write(append(data, '\n'))

	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)

	var resp rpcResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	return resp
}

func TestCodeSandboxHandleConnDisallowedTool(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{
		AllowTools: []string{"read_file"},
		MaxCalls:   10,
	})

	resp := runHandleConnTest(t, cs, rpcRequest{
		ID:    "test1",
		Tool:  "delete_everything",
		Input: map[string]any{},
	}, func(tool string, input map[string]any) (any, error) {
		return "should not be called", nil
	})

	if resp.Error == "" {
		t.Error("Expected error for disallowed tool")
	}
	if !strings.Contains(resp.Error, "not allowed") {
		t.Errorf("Error should mention 'not allowed', got: %q", resp.Error)
	}
}

func TestCodeSandboxHandleConnAllowedTool(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{
		AllowTools: []string{"read_file"},
		MaxCalls:   10,
	})

	toolCalled := false
	resp := runHandleConnTest(t, cs, rpcRequest{
		ID:    "test2",
		Tool:  "read_file",
		Input: map[string]any{"path": "/tmp/test.txt"},
	}, func(tool string, input map[string]any) (any, error) {
		toolCalled = true
		return "file contents", nil
	})

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %q", resp.Error)
	}
	if !toolCalled {
		t.Error("Tool handler should have been called")
	}
}

func TestCodeSandboxHandleConnInvalidJSON(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{
		AllowTools: []string{"read_file"},
		MaxCalls:   10,
	})

	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf("test-inv-%d.sock", time.Now().UnixNano()))
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		cs.handleConn(ctx, conn, func(tool string, input map[string]any) (any, error) {
			return nil, nil
		})
	}()

	time.Sleep(10 * time.Millisecond)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	conn.Write([]byte("not json\n"))

	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)

	var resp rpcResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.Error != "invalid request" {
		t.Errorf("Expected 'invalid request' error, got: %q", resp.Error)
	}
}

func TestCodeSandboxHandleConnMaxCalls(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{
		AllowTools: []string{"read_file"},
		MaxCalls:   0,
	})

	resp := runHandleConnTest(t, cs, rpcRequest{
		ID:    "maxtest",
		Tool:  "read_file",
		Input: map[string]any{},
	}, func(tool string, input map[string]any) (any, error) {
		return nil, nil
	})

	if !strings.Contains(resp.Error, "max tool calls") {
		t.Errorf("Expected max tool calls error, got: %q", resp.Error)
	}
}

func TestCodeSandboxHandleConnToolHandlerError(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{
		AllowTools: []string{"read_file"},
		MaxCalls:   10,
	})

	resp := runHandleConnTest(t, cs, rpcRequest{
		ID:    "errtest",
		Tool:  "read_file",
		Input: map[string]any{},
	}, func(tool string, input map[string]any) (any, error) {
		return nil, fmt.Errorf("file not found")
	})

	if resp.Error != "file not found" {
		t.Errorf("Expected 'file not found' error, got: %q", resp.Error)
	}
}


func TestExecuteResultFields(t *testing.T) {
	result := &ExecuteResult{
		Stdout:    "hello",
		ExitCode:  0,
		Duration:  time.Second,
		ToolCalls: 3,
		Truncated: false,
		TimedOut:  false,
	}

	if result.Stdout != "hello" {
		t.Errorf("Expected Stdout 'hello', got %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected ExitCode 0, got %d", result.ExitCode)
	}
	if result.ToolCalls != 3 {
		t.Errorf("Expected ToolCalls 3, got %d", result.ToolCalls)
	}
}


func TestGeneratePythonStub(t *testing.T) {
	stub := generatePythonStub("/tmp/test.sock", []string{"read_file", "bash"})

	if !strings.Contains(stub, "import json") {
		t.Error("Python stub should import json")
	}
	if !strings.Contains(stub, "import socket") {
		t.Error("Python stub should import socket")
	}
	if !strings.Contains(stub, "/tmp/test.sock") {
		t.Error("Python stub should contain socket path")
	}
	if !strings.Contains(stub, "def read_file(") {
		t.Error("Python stub should define read_file function")
	}
	if !strings.Contains(stub, "def bash(") {
		t.Error("Python stub should define bash function")
	}
	if !strings.Contains(stub, `_call_tool("read_file"`) {
		t.Error("Python stub read_file should call _call_tool with 'read_file'")
	}
}

func TestGeneratePythonStubEmpty(t *testing.T) {
	stub := generatePythonStub("/tmp/test.sock", []string{})
	if !strings.Contains(stub, "import json") {
		t.Error("Python stub should still import json even with no tools")
	}
}

func TestGeneratePythonStubHyphenatedTool(t *testing.T) {
	stub := generatePythonStub("/tmp/test.sock", []string{"web-search"})
	if !strings.Contains(stub, "def web_search(") {
		t.Error("Hyphenated tool name should be converted to underscore")
	}
}


func TestCodeSandboxServeRPC(t *testing.T) {
	cs := NewCodeSandbox(CodeSandboxConfig{
		AllowTools: []string{"bash"},
		MaxCalls:   10,
	})

	listener, err := net.Listen("unix", filepath.Join(os.TempDir(), fmt.Sprintf("test-rpc-%d.sock", time.Now().UnixNano())))
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()
	defer os.Remove(listener.Addr().String())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlerCalled := false
	toolHandler := func(tool string, input map[string]any) (any, error) {
		handlerCalled = true
		return "ok", nil
	}

	go cs.serveRPC(ctx, listener, toolHandler)

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("unix", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	req := rpcRequest{ID: "rpc1", Tool: "bash", Input: map[string]any{"command": "echo hello"}}
	data, _ := json.Marshal(req)
	conn.Write(append(data, '\n'))

	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)

	var resp rpcResponse
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.Error != "" {
		t.Errorf("Expected no error, got: %q", resp.Error)
	}
	if !handlerCalled {
		t.Error("Tool handler should have been called")
	}
}


func TestCodeSandboxExecuteCancelledContext(t *testing.T) {
	if !pythonAvailable() {
		t.Skip("python3 not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    30 * time.Second,
		MaxCalls:   10,
		MaxOutput:  10 * 1024,
		AllowTools: allowedTools,
	})

	_, err := cs.Execute(ctx, "print('should not run')", "python", nil)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}


func TestCodeSandboxExecuteGo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Go execution test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}

	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    60 * time.Second,
		MaxCalls:   10,
		MaxOutput:  10 * 1024,
		AllowTools: allowedTools,
	})

	code := `package main
import "fmt"
func main() {
	fmt.Println("hello from go sandbox")
}`
	result, err := cs.Execute(context.Background(), code, "go", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "hello from go sandbox") {
		t.Errorf("Expected stdout to contain 'hello from go sandbox', got %q", result.Stdout)
	}
}

func TestCodeSandboxExecuteGoError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Go execution test in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}

	cs := NewCodeSandbox(CodeSandboxConfig{
		Timeout:    60 * time.Second,
		MaxCalls:   10,
		MaxOutput:  10 * 1024,
		AllowTools: allowedTools,
	})

	code := `package main
import "fmt"
func main() {
	fmt.Println("about to fail")
	panic("test panic")
}`
	result, err := cs.Execute(context.Background(), code, "go", nil)
	if err != nil {
		t.Fatalf("Execute should not error on runtime panics: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("Expected non-zero exit code for panic")
	}
}


