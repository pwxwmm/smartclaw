package plugins

import (
	"context"

	"github.com/instructkr/smartclaw/internal/tools"
)

// PluginToolBridge adapts a ToolPlugin to the tools.Tool interface
// so it can be registered in the global ToolRegistry.
type PluginToolBridge struct {
	pluginName string
	tool       ToolPlugin
}

func (b *PluginToolBridge) Name() string        { return b.tool.ToolName() }
func (b *PluginToolBridge) Description() string  { return b.tool.ToolDescription() }
func (b *PluginToolBridge) InputSchema() map[string]any { return b.tool.ToolInputSchema() }
func (b *PluginToolBridge) Execute(ctx context.Context, input map[string]any) (any, error) {
	return b.tool.ExecuteTool(ctx, input)
}

// conventionToolBridge adapts a ConventionPlugin tool to the tools.Tool interface.
type conventionToolBridge struct {
	cp *ConventionPlugin
}

func (b *conventionToolBridge) Name() string        { return b.cp.Name }
func (b *conventionToolBridge) Description() string  { return b.cp.Manifest.Description }
func (b *conventionToolBridge) InputSchema() map[string]any { return nil }
func (b *conventionToolBridge) Execute(ctx context.Context, input map[string]any) (any, error) {
	tool := NewPluginTool(ToolExtension{
		Name:        b.cp.Name,
		Description: b.cp.Manifest.Description,
		InputSchema: nil,
		Command:     "sh $PLUGIN_DIR/" + b.cp.Manifest.Main,
	}, b.cp.Path)
	return tool.Execute(ctx, input)
}

// RegisterToolsInRegistry registers all plugin tools into the global ToolRegistry
// so the LLM can discover and call them.
func (r *PluginRegistry) RegisterToolsInRegistry(toolReg *tools.ToolRegistry) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, dp := range r.dynLoader.List() {
		if tp, ok := dp.(ToolPlugin); ok {
			toolReg.Register(&PluginToolBridge{pluginName: dp.Name(), tool: tp})
		}
	}

	for _, cp := range r.convLoader.ListAll() {
		if cp.Enabled && cp.Type == PluginTypeTool {
			toolReg.Register(&conventionToolBridge{cp: cp})
		}
	}
}
