package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// LoadStrategy defines how a plugin is loaded from a file path.
type LoadStrategy interface {
	CanLoad(path string) bool
	Load(ctx context.Context, path string) (PluginInterface, error)
	Name() string
}

// PluginLoader dynamically loads plugins from a directory using registered strategies.
type PluginLoader struct {
	plugins    map[string]PluginInterface
	pluginDir  string
	mu         sync.RWMutex
	strategies []LoadStrategy
}

// NewPluginLoader creates a PluginLoader that scans pluginDir using the
// built-in strategies (native .so, WASM, script).
func NewPluginLoader(pluginDir string) *PluginLoader {
	return &PluginLoader{
		plugins:   make(map[string]PluginInterface),
		pluginDir: pluginDir,
		strategies: []LoadStrategy{
			&NativeStrategy{},
			&WASMStrategy{},
			&ScriptStrategy{},
		},
	}
}

// LoadPlugin attempts to load a plugin by name from the plugin directory.
// It tries each registered strategy in order until one succeeds.
func (l *PluginLoader) LoadPlugin(ctx context.Context, name string) (PluginInterface, error) {
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("read plugin dir: %w", err)
	}

	for _, entry := range entries {
		baseName := entry.Name()
		// Match by stem (without extension) for files, or by directory name.
		stem := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		if stem != name && baseName != name {
			continue
		}

		fullPath := filepath.Join(l.pluginDir, baseName)

		// If it's a directory, look for entry point files inside.
		if entry.IsDir() {
			if p, err := l.tryLoadDir(ctx, fullPath, name); err == nil && p != nil {
				l.mu.Lock()
				l.plugins[p.Name()] = p
				l.mu.Unlock()
				return p, nil
			}
			continue
		}

		p, err := l.tryLoadFile(ctx, fullPath)
		if err != nil {
			continue
		}

		l.mu.Lock()
		l.plugins[p.Name()] = p
		l.mu.Unlock()
		return p, nil
	}

	return nil, fmt.Errorf("plugin %q not found in %s", name, l.pluginDir)
}

// LoadAll scans the plugin directory and loads every discoverable plugin.
func (l *PluginLoader) LoadAll(ctx context.Context) error {
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read plugin dir: %w", err)
	}

	var loadErrs []error

	for _, entry := range entries {
		fullPath := filepath.Join(l.pluginDir, entry.Name())

		if entry.IsDir() {
			p, err := l.tryLoadDir(ctx, fullPath, entry.Name())
			if err != nil || p == nil {
				continue
			}
			l.mu.Lock()
			l.plugins[p.Name()] = p
			l.mu.Unlock()
			continue
		}

		p, err := l.tryLoadFile(ctx, fullPath)
		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}

		l.mu.Lock()
		l.plugins[p.Name()] = p
		l.mu.Unlock()
	}

	if len(loadErrs) > 0 {
		return fmt.Errorf("errors loading plugins: %v", loadErrs)
	}
	return nil
}

// Get returns a previously loaded plugin by name, or nil.
func (l *PluginLoader) Get(name string) PluginInterface {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.plugins[name]
}

// List returns all loaded plugins.
func (l *PluginLoader) List() []PluginInterface {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]PluginInterface, 0, len(l.plugins))
	for _, p := range l.plugins {
		result = append(result, p)
	}
	return result
}

func (l *PluginLoader) tryLoadFile(ctx context.Context, path string) (PluginInterface, error) {
	for _, strategy := range l.strategies {
		if strategy.CanLoad(path) {
			p, err := strategy.Load(ctx, path)
			if err != nil {
				continue
			}
			return p, nil
		}
	}
	return nil, fmt.Errorf("no strategy can load %s", path)
}

func (l *PluginLoader) tryLoadDir(ctx context.Context, dirPath, dirName string) (PluginInterface, error) {
	// Check for entry-point files inside directory.
	candidates := []string{
		filepath.Join(dirPath, dirName+".so"),
		filepath.Join(dirPath, dirName+".wasm"),
		filepath.Join(dirPath, "main.sh"),
		filepath.Join(dirPath, "main.py"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		p, err := l.tryLoadFile(ctx, candidate)
		if err != nil {
			continue
		}
		return p, nil
	}

	return nil, fmt.Errorf("no loadable entry point in %s", dirPath)
}

// --- WASM Strategy ---

// WASMStrategy loads .wasm modules.
type WASMStrategy struct{}

func (s *WASMStrategy) CanLoad(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".wasm")
}

func (s *WASMStrategy) Name() string { return "wasm" }

func (s *WASMStrategy) Load(ctx context.Context, path string) (PluginInterface, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("wasm file not found: %w", err)
	}

	return &WASMPlugin{
		modulePath: path,
		name:       strings.TrimSuffix(filepath.Base(path), ".wasm"),
		version:    "0.0.1",
	}, nil
}

// WASMPlugin wraps a WASM module and implements PluginInterface.
// Actual WASM execution is a placeholder; the module path is recorded.
type WASMPlugin struct {
	modulePath string
	name       string
	version    string
	config     map[string]any
}

func (p *WASMPlugin) Init(_ context.Context, config map[string]any) error {
	p.config = config
	return nil
}

func (p *WASMPlugin) Name() string    { return p.name }
func (p *WASMPlugin) Version() string { return p.version }

func (p *WASMPlugin) Shutdown(_ context.Context) error {
	p.config = nil
	return nil
}

// --- Script Strategy ---

// ScriptStrategy loads script-based plugins (.sh, .py) or directories
// containing main.sh / main.py.
type ScriptStrategy struct{}

func (s *ScriptStrategy) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".sh" || ext == ".py" {
		return true
	}
	// Also accept directories containing main.sh or main.py.
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		if _, err := os.Stat(filepath.Join(path, "main.sh")); err == nil {
			return true
		}
		if _, err := os.Stat(filepath.Join(path, "main.py")); err == nil {
			return true
		}
	}
	return false
}

func (s *ScriptStrategy) Name() string { return "script" }

func (s *ScriptStrategy) Load(ctx context.Context, path string) (PluginInterface, error) {
	var interpreter string
	var scriptPath string

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".sh":
		interpreter = "sh"
		scriptPath = path
	case ".py":
		interpreter = "python3"
		scriptPath = path
	default:
		// Directory: check for main.sh or main.py
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			if sh := filepath.Join(path, "main.sh"); fileExists(sh) {
				interpreter = "sh"
				scriptPath = sh
			} else if py := filepath.Join(path, "main.py"); fileExists(py) {
				interpreter = "python3"
				scriptPath = py
			}
		}
	}

	if scriptPath == "" {
		return nil, fmt.Errorf("no script entry point found at %s", path)
	}

	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if name == "main" {
		name = filepath.Base(filepath.Dir(path))
	}

	var description string
	dir := filepath.Dir(scriptPath)
	if data, err := os.ReadFile(filepath.Join(dir, "plugin.json")); err == nil {
		var manifest struct {
			Description string `json:"description"`
		}
		if json.Unmarshal(data, &manifest) == nil {
			description = manifest.Description
		}
	}

	return &ScriptPlugin{
		name:        name,
		version:     "0.0.1",
		description: description,
		interpreter: interpreter,
		scriptPath:  scriptPath,
	}, nil
}

// ScriptPlugin wraps a script file and implements PluginInterface and
// ToolPlugin by executing it via exec.Command.
type ScriptPlugin struct {
	name        string
	version     string
	description string
	interpreter string
	scriptPath  string
	config      map[string]any
}

func (p *ScriptPlugin) Init(_ context.Context, config map[string]any) error {
	p.config = config
	return nil
}

func (p *ScriptPlugin) Name() string    { return p.name }
func (p *ScriptPlugin) Version() string { return p.version }

func (p *ScriptPlugin) Shutdown(_ context.Context) error {
	p.config = nil
	return nil
}

// Execute runs the script with the given arguments via exec.Command.
func (p *ScriptPlugin) Execute(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, p.interpreter, append([]string{p.scriptPath}, args...)...)
	cmd.Dir = filepath.Dir(p.scriptPath)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("script failed: %w: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("script failed: %w", err)
	}
	return string(output), nil
}

// ToolName returns the name of the tool provided by this script plugin.
func (p *ScriptPlugin) ToolName() string { return p.name }

// ToolDescription returns a human-readable description of what the script tool does.
func (p *ScriptPlugin) ToolDescription() string {
	if p.description != "" {
		return p.description
	}
	return "Script plugin: " + p.name
}

// ToolInputSchema returns the JSON Schema describing the script tool's input parameters.
func (p *ScriptPlugin) ToolInputSchema() map[string]any {
	return map[string]any{
		"type": "object",
	}
}

// ExecuteTool runs the script tool with the given input and returns the result.
func (p *ScriptPlugin) ExecuteTool(ctx context.Context, input map[string]any) (any, error) {
	args := make([]string, 0, len(input))
	for k, v := range input {
		args = append(args, fmt.Sprintf("%s=%v", k, v))
	}
	result, err := p.Execute(ctx, args...)
	if err != nil {
		return nil, err
	}
	return map[string]any{"output": result}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
