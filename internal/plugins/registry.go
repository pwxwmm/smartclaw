package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type RegistryStats struct {
	TotalPlugins      int `json:"total_plugins"`
	EnabledPlugins    int `json:"enabled_plugins"`
	DynamicPlugins    int `json:"dynamic_plugins"`
	ConventionPlugins int `json:"convention_plugins"`
	ToolPlugins       int `json:"tool_plugins"`
	HookPlugins       int `json:"hook_plugins"`
}

type PluginRegistry struct {
	manager    *PluginManager
	convLoader *ConventionPluginLoader
	dynLoader  *PluginLoader
	sandbox    *Sandbox
	mu         sync.RWMutex
}

func NewPluginRegistry(pluginDir string) *PluginRegistry {
	manager := NewPluginManager()

	baseDir := pluginDir
	if baseDir == "" {
		home, _ := os.UserHomeDir()
		baseDir = filepath.Join(home, ".smartclaw", "plugins")
	}

	convLoader := NewConventionPluginLoader(baseDir)
	dynLoader := NewPluginLoader(baseDir)
	sandbox := NewSandbox(DefaultSandboxConfig())

	return &PluginRegistry{
		manager:    manager,
		convLoader: convLoader,
		dynLoader:  dynLoader,
		sandbox:    sandbox,
	}
}

func (r *PluginRegistry) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.convLoader.LoadAll(); err != nil {
		return fmt.Errorf("convention loader: %w", err)
	}

	if err := r.dynLoader.LoadAll(ctx); err != nil {
		return fmt.Errorf("dynamic loader: %w", err)
	}

	return nil
}

func (r *PluginRegistry) Install(source string) (*Plugin, error) {
	return r.manager.Install(source)
}

func (r *PluginRegistry) Uninstall(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.manager.Uninstall(name)
}

func (r *PluginRegistry) List() []*Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var result []*Plugin

	for _, p := range r.manager.List() {
		if !seen[p.Name] {
			seen[p.Name] = true
			result = append(result, p)
		}
	}

	for _, cp := range r.convLoader.ListAll() {
		if !seen[cp.Name] {
			seen[cp.Name] = true
			result = append(result, cp.Manifest)
		}
	}

	return result
}

func (r *PluginRegistry) ListDynamic() []PluginInterface {
	return r.dynLoader.List()
}

func (r *PluginRegistry) Get(name string) *Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if p := r.manager.Get(name); p != nil {
		return p
	}

	for _, pt := range []PluginType{PluginTypeTool, PluginTypeCommand, PluginTypeAdapter, PluginTypeMemory, PluginTypeHook} {
		if cp := r.convLoader.Get(pt, name); cp != nil {
			return cp.Manifest
		}
	}

	if dp := r.dynLoader.Get(name); dp != nil {
		return &Plugin{
			Name:    dp.Name(),
			Version: dp.Version(),
			Enabled: true,
		}
	}

	return nil
}

func (r *PluginRegistry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p := r.manager.Get(name); p != nil {
		return r.manager.Enable(name)
	}

	for _, pt := range []PluginType{PluginTypeTool, PluginTypeCommand, PluginTypeAdapter, PluginTypeMemory, PluginTypeHook} {
		if cp := r.convLoader.Get(pt, name); cp != nil {
			return r.convLoader.Enable(pt, name)
		}
	}

	return fmt.Errorf("plugin not found: %s", name)
}

func (r *PluginRegistry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p := r.manager.Get(name); p != nil {
		return r.manager.Disable(name)
	}

	for _, pt := range []PluginType{PluginTypeTool, PluginTypeCommand, PluginTypeAdapter, PluginTypeMemory, PluginTypeHook} {
		if cp := r.convLoader.Get(pt, name); cp != nil {
			return r.convLoader.Disable(pt, name)
		}
	}

	return fmt.Errorf("plugin not found: %s", name)
}

func (r *PluginRegistry) ExecuteTool(ctx context.Context, pluginName string, input map[string]any) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if dp := r.dynLoader.Get(pluginName); dp != nil {
		if tp, ok := dp.(ToolPlugin); ok {
			return r.sandbox.ExecuteTool(ctx, tp, input)
		}
	}

	if cp := r.convLoader.Get(PluginTypeTool, pluginName); cp != nil && cp.Enabled {
		tool := NewPluginTool(ToolExtension{
			Name:        pluginName,
			Description: cp.Manifest.Description,
			InputSchema: nil,
			Command:     "sh $PLUGIN_DIR/" + cp.Manifest.Main,
		}, cp.Path)
		return tool.Execute(ctx, input)
	}

	if p := r.manager.Get(pluginName); p != nil && p.Enabled {
		tool := NewPluginTool(ToolExtension{
			Name:        pluginName,
			Description: p.Description,
			InputSchema: nil,
			Command:     "sh $PLUGIN_DIR/" + p.Main,
		}, p.Path)
		return tool.Execute(ctx, input)
	}

	return nil, fmt.Errorf("tool plugin not found: %s", pluginName)
}

func (r *PluginRegistry) Search(query string) ([]MarketplacePlugin, error) {
	return r.manager.Search(query)
}

func (r *PluginRegistry) GetStats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var stats RegistryStats

	managerPlugins := r.manager.List()
	stats.TotalPlugins += len(managerPlugins)
	for _, p := range managerPlugins {
		if p.Enabled {
			stats.EnabledPlugins++
		}
		if len(p.Tools) > 0 {
			stats.ToolPlugins++
		}
		if len(p.Hooks) > 0 {
			stats.HookPlugins++
		}
	}

	conventionPlugins := r.convLoader.ListAll()
	stats.ConventionPlugins = len(conventionPlugins)
	for _, cp := range conventionPlugins {
		if !isDuplicatePlugin(cp.Name, managerPlugins) {
			stats.TotalPlugins++
			if cp.Enabled {
				stats.EnabledPlugins++
			}
		}
		if cp.Type == PluginTypeTool {
			stats.ToolPlugins++
		}
		if cp.Type == PluginTypeHook {
			stats.HookPlugins++
		}
	}

	dynamicPlugins := r.dynLoader.List()
	stats.DynamicPlugins = len(dynamicPlugins)
	for _, dp := range dynamicPlugins {
		if !isDuplicatePlugin(dp.Name(), managerPlugins) {
			stats.TotalPlugins++
		}
		if _, ok := dp.(ToolPlugin); ok {
			stats.ToolPlugins++
		}
		if _, ok := dp.(HookPlugin); ok {
			stats.HookPlugins++
		}
	}

	return stats
}

func (r *PluginRegistry) Manager() *PluginManager {
	return r.manager
}

func (r *PluginRegistry) ConvLoader() *ConventionPluginLoader {
	return r.convLoader
}

func (r *PluginRegistry) DynLoader() *PluginLoader {
	return r.dynLoader
}

func (r *PluginRegistry) Sandbox() *Sandbox {
	return r.sandbox
}

type RegistrySource string

const (
	SourceInstalled  RegistrySource = "installed"
	SourceConvention RegistrySource = "convention"
	SourceDynamic    RegistrySource = "dynamic"
)

func (r *PluginRegistry) GetPluginSource(name string) RegistrySource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.manager.Get(name) != nil {
		return SourceInstalled
	}

	for _, pt := range []PluginType{PluginTypeTool, PluginTypeCommand, PluginTypeAdapter, PluginTypeMemory, PluginTypeHook} {
		if r.convLoader.Get(pt, name) != nil {
			return SourceConvention
		}
	}

	if r.dynLoader.Get(name) != nil {
		return SourceDynamic
	}

	return SourceInstalled
}

func isDuplicatePlugin(name string, managerPlugins []*Plugin) bool {
	for _, p := range managerPlugins {
		if p.Name == name {
			return true
		}
	}
	return false
}
