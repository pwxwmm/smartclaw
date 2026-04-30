package plugins

import "context"

// PluginInterface is the base interface that all plugins must implement.
// It provides lifecycle management (init/shutdown) and identity (name/version).
type PluginInterface interface {
	// Init initializes the plugin with the provided configuration.
	Init(ctx context.Context, config map[string]any) error

	// Name returns the unique identifier for this plugin.
	Name() string

	// Version returns the semantic version of the plugin.
	Version() string

	// Shutdown gracefully terminates the plugin, releasing any resources.
	Shutdown(ctx context.Context) error
}

// ToolPlugin extends PluginInterface with tool capabilities.
// Tools are invocable functions with typed input schemas.
type ToolPlugin interface {
	PluginInterface

	// ToolName returns the name of the tool provided by this plugin.
	ToolName() string

	// ToolDescription returns a human-readable description of what the tool does.
	ToolDescription() string

	// ToolInputSchema returns the JSON Schema describing the tool's input parameters.
	ToolInputSchema() map[string]any

	// ExecuteTool runs the tool with the given input and returns the result.
	ExecuteTool(ctx context.Context, input map[string]any) (any, error)
}

// HookPlugin extends PluginInterface with hook/event capabilities.
// Hooks allow plugins to react to system events.
type HookPlugin interface {
	PluginInterface

	// HookEvents returns the list of event names this plugin subscribes to.
	HookEvents() []string

	// HandleHook is called when a subscribed event fires.
	HandleHook(ctx context.Context, event string, data map[string]any) (map[string]any, error)
}

// MemoryPlugin extends PluginInterface with memory provider capabilities.
// Memory plugins provide queryable storage backends.
type MemoryPlugin interface {
	PluginInterface

	// MemoryName returns the name of the memory provider.
	MemoryName() string

	// QueryMemory searches the memory store for entries matching the query.
	QueryMemory(ctx context.Context, query string, limit int) ([]MemoryResult, error)

	// StoreMemory persists a key-value pair into the memory store.
	StoreMemory(ctx context.Context, key, value string) error
}

// MemoryResult represents a single result from a memory query.
type MemoryResult struct {
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Relevance  float64 `json:"relevance"`
	Source     string  `json:"source"`
}

// PluginCapability represents a capability that a plugin provides.
type PluginCapability string

const (
	// CapabilityTool indicates the plugin provides a tool.
	CapabilityTool PluginCapability = "tool"

	// CapabilityHook indicates the plugin handles hooks.
	CapabilityHook PluginCapability = "hook"

	// CapabilityMemory indicates the plugin provides a memory backend.
	CapabilityMemory PluginCapability = "memory"
)

// PluginInfo describes a plugin's metadata and capabilities.
type PluginInfo struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Author       string            `json:"author"`
	Capabilities []PluginCapability `json:"capabilities"`
	ConfigSchema map[string]any    `json:"config_schema,omitempty"`
}

// GetPluginInfo inspects a plugin and returns its metadata by type-asserting
// against the known capability interfaces (ToolPlugin, HookPlugin, MemoryPlugin).
func GetPluginInfo(p PluginInterface) PluginInfo {
	info := PluginInfo{
		Name:    p.Name(),
		Version: p.Version(),
	}

	if tp, ok := p.(ToolPlugin); ok {
		info.Capabilities = append(info.Capabilities, CapabilityTool)
		info.Description = tp.ToolDescription()
	}

	if hp, ok := p.(HookPlugin); ok {
		info.Capabilities = append(info.Capabilities, CapabilityHook)
		if info.Description == "" {
			info.Description = "Hook plugin: " + hp.Name()
		}
	}

	if mp, ok := p.(MemoryPlugin); ok {
		info.Capabilities = append(info.Capabilities, CapabilityMemory)
		if info.Description == "" {
			info.Description = "Memory plugin: " + mp.MemoryName()
		}
	}

	return info
}
