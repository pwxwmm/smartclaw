package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/security"
)

type HookEvent string

const (
	HookPreToolUse         HookEvent = "PreToolUse"
	HookPostToolUse        HookEvent = "PostToolUse"
	HookPostToolUseFailure HookEvent = "PostToolUseFailure"
	HookPreCompact         HookEvent = "PreCompact"
	HookPostCompact        HookEvent = "PostCompact"
	HookSessionStart       HookEvent = "SessionStart"
	HookSessionEnd         HookEvent = "SessionEnd"
	HookStop               HookEvent = "Stop"
	HookStopFailure        HookEvent = "StopFailure"
	HookNotification       HookEvent = "Notification"
	HookPermissionDenied   HookEvent = "PermissionDenied"
	HookPermissionRequest  HookEvent = "PermissionRequest"
	HookUserPromptSubmit   HookEvent = "UserPromptSubmit"
	HookSubagentStart      HookEvent = "SubagentStart"
	HookSubagentStop       HookEvent = "SubagentStop"
	HookTaskCreated        HookEvent = "TaskCreated"
	HookTaskCompleted      HookEvent = "TaskCompleted"
	HookConfigChange       HookEvent = "ConfigChange"
	HookCwdChanged         HookEvent = "CwdChanged"
	HookFileChanged        HookEvent = "FileChanged"
	HookSetup              HookEvent = "Setup"
)

type HookConfig struct {
	Name        string            `json:"name"`
	Event       HookEvent         `json:"event"`
	Command     string            `json:"command"`
	Async       bool              `json:"async,omitempty"`
	AsyncRewake bool              `json:"asyncRewake,omitempty"`
	Enabled     bool              `json:"enabled"`
	Timeout     int               `json:"timeout,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	PluginID    string            `json:"pluginId,omitempty"`
}

type HookInput struct {
	Event          HookEvent      `json:"event"`
	SessionID      string         `json:"session_id"`
	ProjectRoot    string         `json:"project_root"`
	Timestamp      int64          `json:"timestamp"`
	ToolName       string         `json:"tool_name,omitempty"`
	ToolInput      map[string]any `json:"tool_input,omitempty"`
	ToolOutput     any            `json:"tool_output,omitempty"`
	Error          string         `json:"error,omitempty"`
	Message        string         `json:"message,omitempty"`
	AdditionalData map[string]any `json:"additional_data,omitempty"`
}

type HookOutput struct {
	Continue           bool               `json:"continue,omitempty"`
	SuppressOutput     bool               `json:"suppressOutput,omitempty"`
	StopReason         string             `json:"stopReason,omitempty"`
	Decision           string             `json:"decision,omitempty"`
	Reason             string             `json:"reason,omitempty"`
	SystemMessage      string             `json:"systemMessage,omitempty"`
	UpdatedInput       map[string]any     `json:"updatedInput,omitempty"`
	AdditionalContext  string             `json:"additionalContext,omitempty"`
	PermissionDecision string             `json:"permissionDecision,omitempty"`
	PermissionUpdates  []PermissionUpdate `json:"permissionUpdates,omitempty"`
	ExitCode           int                `json:"exitCode"`
	Stdout             string             `json:"stdout"`
	Stderr             string             `json:"stderr"`
}

type PermissionUpdate struct {
	Type   string `json:"type"`
	Rule   string `json:"rule"`
	Action string `json:"action"`
}

type HookResult struct {
	HookName string
	Event    HookEvent
	Output   *HookOutput
	Success  bool
	Error    error
	Duration time.Duration
}

type HookRegistry struct {
	hooks map[HookEvent][]HookConfig
	mu    sync.RWMutex
}

func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		hooks: make(map[HookEvent][]HookConfig),
	}
}

func (r *HookRegistry) Register(hook HookConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.hooks[hook.Event] = append(r.hooks[hook.Event], hook)
}

func (r *HookRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for event, hooks := range r.hooks {
		newHooks := make([]HookConfig, 0)
		for _, h := range hooks {
			if h.Name != name {
				newHooks = append(newHooks, h)
			}
		}
		r.hooks[event] = newHooks
	}
}

func (r *HookRegistry) GetHooks(event HookEvent) []HookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.hooks[event]
}

func (r *HookRegistry) GetAllHooks() map[HookEvent][]HookConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[HookEvent][]HookConfig)
	for k, v := range r.hooks {
		result[k] = v
	}
	return result
}

func (r *HookRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = make(map[HookEvent][]HookConfig)
}

type HookExecutor struct {
	registry    *HookRegistry
	workDir     string
	sessionID   string
	projectRoot string
	timeout     time.Duration
	mu          sync.Mutex
}

func NewHookExecutor(registry *HookRegistry, workDir, sessionID string) *HookExecutor {
	return &HookExecutor{
		registry:    registry,
		workDir:     workDir,
		sessionID:   sessionID,
		projectRoot: workDir,
		timeout:     30 * time.Second,
	}
}

func (e *HookExecutor) Execute(ctx context.Context, event HookEvent, input *HookInput) []HookResult {
	hooks := e.registry.GetHooks(event)
	if len(hooks) == 0 {
		return nil
	}

	if input == nil {
		input = &HookInput{}
	}

	input.Event = event
	input.SessionID = e.sessionID
	input.ProjectRoot = e.projectRoot
	input.Timestamp = time.Now().Unix()

	results := make([]HookResult, 0, len(hooks))

	for _, hook := range hooks {
		if !hook.Enabled {
			continue
		}

		result := e.executeHook(ctx, hook, input)
		results = append(results, result)

		if result.Output != nil && !result.Output.Continue {
			break
		}
	}

	return results
}

func (e *HookExecutor) executeHook(ctx context.Context, hook HookConfig, input *HookInput) HookResult {
	startTime := time.Now()
	result := HookResult{
		HookName: hook.Name,
		Event:    hook.Event,
	}

	if hook.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(hook.Timeout)*time.Millisecond)
		defer cancel()
	}

	inputData, err := json.Marshal(input)
	if err != nil {
		result.Error = fmt.Errorf("failed to marshal hook input: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	if validationResult := security.ValidateCommandSecurity(hook.Command); !validationResult.Allowed {
		result.Error = fmt.Errorf("command rejected by security policy: %s", validationResult.Reason)
		result.Duration = time.Since(startTime)
		return result
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", hook.Command)
	cmd.Dir = e.workDir

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("CLAUDE_HOOK_INPUT=%s", string(inputData)),
		fmt.Sprintf("CLAUDE_SESSION_ID=%s", e.sessionID),
		fmt.Sprintf("CLAUDE_PROJECT_ROOT=%s", e.projectRoot),
	)

	for k, v := range hook.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		result.Error = fmt.Errorf("failed to create stdin pipe: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	go func() {
		defer stdinPipe.Close()
		stdinPipe.Write(inputData)
	}()

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(startTime)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Output = &HookOutput{
				ExitCode: exitErr.ExitCode(),
				Stderr:   string(output),
			}
		}
		result.Error = err
		result.Success = false
		return result
	}

	result.Success = true
	result.Output = &HookOutput{
		ExitCode: 0,
		Stdout:   string(output),
	}

	var hookOutput HookOutput
	if err := json.Unmarshal(output, &hookOutput); err == nil {
		hookOutput.ExitCode = 0
		hookOutput.Stdout = string(output)
		result.Output = &hookOutput
	}

	return result
}

func (e *HookExecutor) ExecuteAsync(ctx context.Context, event HookEvent, input *HookInput, callback func(HookResult)) {
	hooks := e.registry.GetHooks(event)
	if len(hooks) == 0 {
		return
	}

	if input == nil {
		input = &HookInput{}
	}

	input.Event = event
	input.SessionID = e.sessionID
	input.ProjectRoot = e.projectRoot
	input.Timestamp = time.Now().Unix()

	for _, hook := range hooks {
		if !hook.Enabled {
			continue
		}

		go func(h HookConfig) {
			result := e.executeHook(ctx, h, input)
			if callback != nil {
				callback(result)
			}
		}(hook)
	}
}

func (e *HookRegistry) LoadFromConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var config struct {
		Hooks []HookConfig `json:"hooks"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	for _, hook := range config.Hooks {
		if hook.Enabled {
			e.Register(hook)
		}
	}

	return nil
}

func (e *HookRegistry) SaveToConfig(configPath string) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	allHooks := make([]HookConfig, 0)
	for _, hooks := range e.hooks {
		allHooks = append(allHooks, hooks...)
	}

	config := struct {
		Hooks []HookConfig `json:"hooks"`
	}{
		Hooks: allHooks,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

type HookManager struct {
	registry  *HookRegistry
	executor  *HookExecutor
	configDir string
	mu        sync.Mutex
}

func NewHookManager(workDir, sessionID string) *HookManager {
	home, _ := os.UserHomeDir()
	configDir := fmt.Sprintf("%s/.claude/hooks", home)

	registry := NewHookRegistry()
	executor := NewHookExecutor(registry, workDir, sessionID)

	return &HookManager{
		registry:  registry,
		executor:  executor,
		configDir: configDir,
	}
}

func (m *HookManager) ExecutePreToolUse(ctx context.Context, toolName string, toolInput map[string]any) ([]HookResult, error) {
	input := &HookInput{
		ToolName:  toolName,
		ToolInput: toolInput,
	}

	results := m.executor.Execute(ctx, HookPreToolUse, input)

	for _, result := range results {
		if result.Output != nil && result.Output.Decision == "block" {
			return results, fmt.Errorf("hook blocked: %s", result.Output.Reason)
		}
	}

	return results, nil
}

func (m *HookManager) ExecutePostToolUse(ctx context.Context, toolName string, toolInput map[string]any, output any) []HookResult {
	input := &HookInput{
		ToolName:   toolName,
		ToolInput:  toolInput,
		ToolOutput: output,
	}

	return m.executor.Execute(ctx, HookPostToolUse, input)
}

func (m *HookManager) ExecutePostToolUseFailure(ctx context.Context, toolName string, toolInput map[string]any, errMsg string) []HookResult {
	input := &HookInput{
		ToolName:  toolName,
		ToolInput: toolInput,
		Error:     errMsg,
	}

	return m.executor.Execute(ctx, HookPostToolUseFailure, input)
}

func (m *HookManager) ExecutePreCompact(ctx context.Context) []HookResult {
	return m.executor.Execute(ctx, HookPreCompact, nil)
}

func (m *HookManager) ExecutePostCompact(ctx context.Context) []HookResult {
	return m.executor.Execute(ctx, HookPostCompact, nil)
}

func (m *HookManager) ExecuteSessionStart(ctx context.Context) []HookResult {
	return m.executor.Execute(ctx, HookSessionStart, nil)
}

func (m *HookManager) ExecuteSessionEnd(ctx context.Context) []HookResult {
	return m.executor.Execute(ctx, HookSessionEnd, nil)
}

func (m *HookManager) ExecuteStop(ctx context.Context, message string) []HookResult {
	input := &HookInput{
		Message: message,
	}
	return m.executor.Execute(ctx, HookStop, input)
}

func (m *HookManager) ExecuteNotification(ctx context.Context, message string) []HookResult {
	input := &HookInput{
		Message: message,
	}
	return m.executor.Execute(ctx, HookNotification, input)
}

func (m *HookManager) ExecutePermissionDenied(ctx context.Context, toolName string, reason string) []HookResult {
	input := &HookInput{
		ToolName: toolName,
		Error:    reason,
	}
	return m.executor.Execute(ctx, HookPermissionDenied, input)
}

func (m *HookManager) ExecutePermissionRequest(ctx context.Context, toolName string, toolInput map[string]any) []HookResult {
	input := &HookInput{
		ToolName:  toolName,
		ToolInput: toolInput,
	}
	return m.executor.Execute(ctx, HookPermissionRequest, input)
}

func (m *HookManager) ExecuteUserPromptSubmit(ctx context.Context, message string) []HookResult {
	input := &HookInput{
		Message: message,
	}
	return m.executor.Execute(ctx, HookUserPromptSubmit, input)
}

func (m *HookManager) ExecuteSubagentStart(ctx context.Context, agentID string) []HookResult {
	input := &HookInput{
		AdditionalData: map[string]any{
			"agent_id": agentID,
		},
	}
	return m.executor.Execute(ctx, HookSubagentStart, input)
}

func (m *HookManager) ExecuteSubagentStop(ctx context.Context, agentID string) []HookResult {
	input := &HookInput{
		AdditionalData: map[string]any{
			"agent_id": agentID,
		},
	}
	return m.executor.Execute(ctx, HookSubagentStop, input)
}

func (m *HookManager) ExecuteTaskCreated(ctx context.Context, taskID string) []HookResult {
	input := &HookInput{
		AdditionalData: map[string]any{
			"task_id": taskID,
		},
	}
	return m.executor.Execute(ctx, HookTaskCreated, input)
}

func (m *HookManager) ExecuteTaskCompleted(ctx context.Context, taskID string) []HookResult {
	input := &HookInput{
		AdditionalData: map[string]any{
			"task_id": taskID,
		},
	}
	return m.executor.Execute(ctx, HookTaskCompleted, input)
}

func (m *HookManager) ExecuteConfigChange(ctx context.Context, key string, oldValue, newValue any) []HookResult {
	input := &HookInput{
		AdditionalData: map[string]any{
			"key":       key,
			"old_value": oldValue,
			"new_value": newValue,
		},
	}
	return m.executor.Execute(ctx, HookConfigChange, input)
}

func (m *HookManager) ExecuteCwdChanged(ctx context.Context, oldDir, newDir string) []HookResult {
	input := &HookInput{
		AdditionalData: map[string]any{
			"old_dir": oldDir,
			"new_dir": newDir,
		},
	}
	return m.executor.Execute(ctx, HookCwdChanged, input)
}

func (m *HookManager) ExecuteFileChanged(ctx context.Context, filePath string, changeType string) []HookResult {
	input := &HookInput{
		AdditionalData: map[string]any{
			"file_path":   filePath,
			"change_type": changeType,
		},
	}
	return m.executor.Execute(ctx, HookFileChanged, input)
}

func (m *HookManager) RegisterHook(hook HookConfig) {
	m.registry.Register(hook)
}

func (m *HookManager) UnregisterHook(name string) {
	m.registry.Unregister(name)
}

func (m *HookManager) GetHooks(event HookEvent) []HookConfig {
	return m.registry.GetHooks(event)
}

func (m *HookManager) LoadConfig() error {
	configPath := fmt.Sprintf("%s/hooks.json", m.configDir)
	return m.registry.LoadFromConfig(configPath)
}

func (m *HookManager) SaveConfig() error {
	os.MkdirAll(m.configDir, 0755)
	configPath := fmt.Sprintf("%s/hooks.json", m.configDir)
	return m.registry.SaveToConfig(configPath)
}

func (m *HookManager) GetRegistry() *HookRegistry {
	return m.registry
}
