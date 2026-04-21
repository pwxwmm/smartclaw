package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/config"
)

// PluginType defines the category of a convention-based plugin.
type PluginType string

const (
	PluginTypeTool    PluginType = "tool"
	PluginTypeCommand PluginType = "command"
	PluginTypeAdapter PluginType = "adapter"
	PluginTypeMemory  PluginType = "memory"
	PluginTypeHook    PluginType = "hook"
)

// validPluginTypes is the set of allowed directory/type values.
var validPluginTypes = map[PluginType]bool{
	PluginTypeTool:    true,
	PluginTypeCommand: true,
	PluginTypeAdapter: true,
	PluginTypeMemory:  true,
	PluginTypeHook:    true,
}

// conventionManifest extends the base Plugin manifest with a required type field.
type conventionManifest struct {
	Plugin
	Type string `json:"type"`
}

// ConventionPlugin represents a plugin loaded via the convention-based directory structure.
type ConventionPlugin struct {
	Type     PluginType
	Name     string
	Path     string
	Manifest *Plugin
	Config   map[string]string
	Enabled  bool
	LoadedAt time.Time
}

// ConventionPluginLoader discovers and loads plugins from a convention-based
// directory layout: <baseDir>/<type>/<name>/plugin.json
type ConventionPluginLoader struct {
	baseDir string
	plugins map[PluginType]map[string]*ConventionPlugin
	mu      sync.RWMutex
}

// NewConventionPluginLoader creates a new loader rooted at baseDir.
// The baseDir is typically ~/.smartclaw/plugins.
func NewConventionPluginLoader(baseDir string) *ConventionPluginLoader {
	return &ConventionPluginLoader{
		baseDir: baseDir,
		plugins: make(map[PluginType]map[string]*ConventionPlugin),
	}
}

// LoadAll scans all type subdirectories under baseDir and loads every plugin
// that contains a valid plugin.json manifest.
func (l *ConventionPluginLoader) LoadAll() error {
	for pt := range validPluginTypes {
		typeDir := filepath.Join(l.baseDir, string(pt))
		entries, err := os.ReadDir(typeDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read directory %s: %w", typeDir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if _, err := l.Load(pt, entry.Name()); err != nil {
				continue
			}
		}
	}
	return nil
}

// Load reads and validates a specific plugin from the convention directory.
// The plugin.json must contain a "type" field matching the directory type.
func (l *ConventionPluginLoader) Load(pluginType PluginType, name string) (*ConventionPlugin, error) {
	if !validPluginTypes[pluginType] {
		return nil, fmt.Errorf("invalid plugin type: %s", pluginType)
	}

	pluginDir := filepath.Join(l.baseDir, string(pluginType), name)
	manifestPath := filepath.Join(pluginDir, "plugin.json")

	manifest, err := config.LoadJSON[conventionManifest](manifestPath)
	if err != nil {
		return nil, fmt.Errorf("load plugin manifest %s: %w", manifestPath, err)
	}

	if manifest.Type != string(pluginType) {
		return nil, fmt.Errorf("type mismatch: directory type is %q but manifest declares %q", pluginType, manifest.Type)
	}

	cp := &ConventionPlugin{
		Type:     pluginType,
		Name:     name,
		Path:     pluginDir,
		Manifest: &manifest.Plugin,
		Config:   manifest.Plugin.Config,
		Enabled:  manifest.Plugin.Enabled,
		LoadedAt: time.Now(),
	}

	l.mu.Lock()
	if l.plugins[pluginType] == nil {
		l.plugins[pluginType] = make(map[string]*ConventionPlugin)
	}
	l.plugins[pluginType][name] = cp
	l.mu.Unlock()

	return cp, nil
}

// Get returns a previously loaded plugin by type and name, or nil.
func (l *ConventionPluginLoader) Get(pluginType PluginType, name string) *ConventionPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if m, ok := l.plugins[pluginType]; ok {
		return m[name]
	}
	return nil
}

// ListByType returns all loaded plugins of a given type.
func (l *ConventionPluginLoader) ListByType(pluginType PluginType) []*ConventionPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()

	m, ok := l.plugins[pluginType]
	if !ok {
		return nil
	}

	result := make([]*ConventionPlugin, 0, len(m))
	for _, p := range m {
		result = append(result, p)
	}
	return result
}

// ListAll returns all loaded plugins across every type.
func (l *ConventionPluginLoader) ListAll() []*ConventionPlugin {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*ConventionPlugin
	for _, m := range l.plugins {
		for _, p := range m {
			result = append(result, p)
		}
	}
	return result
}

// Enable marks a plugin as enabled.
func (l *ConventionPluginLoader) Enable(pluginType PluginType, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	m, ok := l.plugins[pluginType]
	if !ok {
		return fmt.Errorf("plugin not found: %s/%s", pluginType, name)
	}
	cp, ok := m[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s/%s", pluginType, name)
	}

	cp.Enabled = true
	cp.Manifest.Enabled = true
	return nil
}

// Disable marks a plugin as disabled.
func (l *ConventionPluginLoader) Disable(pluginType PluginType, name string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	m, ok := l.plugins[pluginType]
	if !ok {
		return fmt.Errorf("plugin not found: %s/%s", pluginType, name)
	}
	cp, ok := m[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s/%s", pluginType, name)
	}

	cp.Enabled = false
	cp.Manifest.Enabled = false
	return nil
}

// GetTools is a convenience method that returns all tool plugins.
func (l *ConventionPluginLoader) GetTools() []*ConventionPlugin {
	return l.ListByType(PluginTypeTool)
}

// GetCommands is a convenience method that returns all command plugins.
func (l *ConventionPluginLoader) GetCommands() []*ConventionPlugin {
	return l.ListByType(PluginTypeCommand)
}
