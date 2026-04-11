package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/hooks"
	"github.com/instructkr/smartclaw/internal/tools"
)

type ExtensionType string

const (
	ExtensionTypeHook    ExtensionType = "hook"
	ExtensionTypeTool    ExtensionType = "tool"
	ExtensionTypeCommand ExtensionType = "command"
	ExtensionTypeMCP     ExtensionType = "mcp"
)

type Extension struct {
	Name       string                 `json:"name"`
	Type       ExtensionType          `json:"type"`
	PluginName string                 `json:"plugin_name"`
	Enabled    bool                   `json:"enabled"`
	Config     map[string]any `json:"config,omitempty"`
	Handler    any            `json:"-"`
	LoadedAt   time.Time              `json:"loaded_at"`
}

type HookExtension struct {
	Name    string          `json:"name"`
	Event   hooks.HookEvent `json:"event"`
	Command string          `json:"command"`
	Timeout int             `json:"timeout,omitempty"`
}

type ToolExtension struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	Command     string                 `json:"command"`
}

type CommandExtension struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Command     string `json:"command"`
}

type ExtensionLoader struct {
	toolRegistry     *tools.ToolRegistry
	hookRegistry     *hooks.HookRegistry
	loadedExtensions map[string]*Extension
	mu               sync.RWMutex
}

func NewExtensionLoader(toolRegistry *tools.ToolRegistry, hookRegistry *hooks.HookRegistry) *ExtensionLoader {
	return &ExtensionLoader{
		toolRegistry:     toolRegistry,
		hookRegistry:     hookRegistry,
		loadedExtensions: make(map[string]*Extension),
	}
}

func (l *ExtensionLoader) LoadExtensions(ctx context.Context, p *Plugin) error {
	if !p.Enabled {
		return fmt.Errorf("plugin %s is not enabled", p.Name)
	}

	var errors []error

	if err := l.LoadHookExtensions(ctx, p); err != nil {
		errors = append(errors, fmt.Errorf("hooks: %w", err))
	}

	if err := l.LoadToolExtensions(ctx, p); err != nil {
		errors = append(errors, fmt.Errorf("tools: %w", err))
	}

	if err := l.LoadCommandExtensions(ctx, p); err != nil {
		errors = append(errors, fmt.Errorf("commands: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("extension loading errors: %v", errors)
	}

	return nil
}

func (l *ExtensionLoader) LoadHookExtensions(ctx context.Context, p *Plugin) error {
	extPath := filepath.Join(p.Path, "hooks.json")
	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("read hooks.json: %w", err)
	}

	var hookExts []HookExtension
	if err := json.Unmarshal(data, &hookExts); err != nil {
		return fmt.Errorf("parse hooks.json: %w", err)
	}

	for _, hookExt := range hookExts {
		hookConfig := hooks.HookConfig{
			Name:    fmt.Sprintf("%s.%s", p.Name, hookExt.Name),
			Event:   hookExt.Event,
			Command: hookExt.Command,
			Enabled: true,
			Timeout: hookExt.Timeout,
		}

		if l.hookRegistry != nil {
			l.hookRegistry.Register(hookConfig)
		}

		ext := &Extension{
			Name:       hookExt.Name,
			Type:       ExtensionTypeHook,
			PluginName: p.Name,
			Enabled:    true,
			LoadedAt:   time.Now(),
		}

		l.mu.Lock()
		l.loadedExtensions[fmt.Sprintf("%s.%s", p.Name, hookExt.Name)] = ext
		l.mu.Unlock()
	}

	return nil
}

func (l *ExtensionLoader) LoadToolExtensions(ctx context.Context, p *Plugin) error {
	extPath := filepath.Join(p.Path, "tools.json")
	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("read tools.json: %w", err)
	}

	var toolExts []ToolExtension
	if err := json.Unmarshal(data, &toolExts); err != nil {
		return fmt.Errorf("parse tools.json: %w", err)
	}

	for _, toolExt := range toolExts {
		tool := NewPluginTool(toolExt, p.Path)

		if l.toolRegistry != nil {
			l.toolRegistry.Register(tool)
		}

		ext := &Extension{
			Name:       toolExt.Name,
			Type:       ExtensionTypeTool,
			PluginName: p.Name,
			Enabled:    true,
			LoadedAt:   time.Now(),
		}

		l.mu.Lock()
		l.loadedExtensions[fmt.Sprintf("%s.%s", p.Name, toolExt.Name)] = ext
		l.mu.Unlock()
	}

	return nil
}

func (l *ExtensionLoader) LoadCommandExtensions(ctx context.Context, p *Plugin) error {
	extPath := filepath.Join(p.Path, "commands.json")
	if _, err := os.Stat(extPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(extPath)
	if err != nil {
		return fmt.Errorf("read commands.json: %w", err)
	}

	var cmdExts []CommandExtension
	if err := json.Unmarshal(data, &cmdExts); err != nil {
		return fmt.Errorf("parse commands.json: %w", err)
	}

	for _, cmdExt := range cmdExts {
		ext := &Extension{
			Name:       cmdExt.Name,
			Type:       ExtensionTypeCommand,
			PluginName: p.Name,
			Enabled:    true,
			Config: map[string]any{
				"description": cmdExt.Description,
				"command":     cmdExt.Command,
			},
			LoadedAt: time.Now(),
		}

		l.mu.Lock()
		l.loadedExtensions[fmt.Sprintf("%s.%s", p.Name, cmdExt.Name)] = ext
		l.mu.Unlock()
	}

	return nil
}

func (l *ExtensionLoader) UnloadExtensions(pluginName string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var errors []error

	for key, ext := range l.loadedExtensions {
		if ext.PluginName != pluginName {
			continue
		}

		switch ext.Type {
		case ExtensionTypeHook:
			if l.hookRegistry != nil {
				l.hookRegistry.Unregister(ext.Name)
			}
		case ExtensionTypeTool:
			if l.toolRegistry != nil {
				l.toolRegistry.Unregister(ext.Name)
			}
		}

		delete(l.loadedExtensions, key)
	}

	if len(errors) > 0 {
		return fmt.Errorf("extension unloading errors: %v", errors)
	}

	return nil
}

func (l *ExtensionLoader) GetLoadedExtensions() []*Extension {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*Extension, 0, len(l.loadedExtensions))
	for _, ext := range l.loadedExtensions {
		result = append(result, ext)
	}
	return result
}

func (l *ExtensionLoader) GetExtensionsByPlugin(pluginName string) []*Extension {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*Extension, 0)
	for _, ext := range l.loadedExtensions {
		if ext.PluginName == pluginName {
			result = append(result, ext)
		}
	}
	return result
}

type PluginTool struct {
	name        string
	description string
	inputSchema map[string]any
	command     string
	pluginPath  string
}

func NewPluginTool(ext ToolExtension, pluginPath string) *PluginTool {
	return &PluginTool{
		name:        ext.Name,
		description: ext.Description,
		inputSchema: ext.InputSchema,
		command:     ext.Command,
		pluginPath:  pluginPath,
	}
}

func (t *PluginTool) Name() string {
	return t.name
}

func (t *PluginTool) Description() string {
	return t.description
}

func (t *PluginTool) InputSchema() map[string]any {
	return t.inputSchema
}

func (t *PluginTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", t.command)
	cmd.Dir = t.pluginPath
	cmd.Stdin = bytes.NewReader(inputJSON)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("tool failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("tool failed: %w", err)
	}

	var result any
	if err := json.Unmarshal(output, &result); err != nil {
		return map[string]any{"output": string(output)}, nil
	}

	return result, nil
}
