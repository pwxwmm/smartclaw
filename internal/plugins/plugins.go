package plugins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Plugin struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Description  string            `json:"description"`
	Main         string            `json:"main"`
	Commands     []string          `json:"commands,omitempty"`
	Tools        []string          `json:"tools,omitempty"`
	Hooks        []string          `json:"hooks,omitempty"`
	Config       map[string]string `json:"config,omitempty"`
	Enabled      bool              `json:"enabled"`
	Path         string            `json:"-"`
	Dependencies []Dependency      `json:"dependencies,omitempty"`
	Author       string            `json:"author,omitempty"`
	Repository   string            `json:"repository,omitempty"`
	InstalledAt  time.Time         `json:"installed_at,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at,omitempty"`
}

type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type PluginSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type"`
}

type MarketplacePlugin struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Repository  string   `json:"repository"`
	Tags        []string `json:"tags"`
	Downloads   int      `json:"downloads"`
}

type PluginManager struct {
	plugins     map[string]*Plugin
	sources     []PluginSource
	pluginsDir  string
	mu          sync.RWMutex
	onInstall   func(*Plugin)
	onUninstall func(*Plugin)
	onEnable    func(*Plugin)
	onDisable   func(*Plugin)
}

func NewPluginManager() *PluginManager {
	home, _ := os.UserHomeDir()
	pluginsDir := filepath.Join(home, ".smartclaw", "plugins")

	pm := &PluginManager{
		plugins:    make(map[string]*Plugin),
		pluginsDir: pluginsDir,
		sources: []PluginSource{
			{Name: "github", URL: "https://api.github.com/repos", Type: "github"},
		},
	}

	os.MkdirAll(pluginsDir, 0755)
	pm.loadAll()
	return pm
}

func (pm *PluginManager) loadAll() {
	entries, err := os.ReadDir(pm.pluginsDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(pm.pluginsDir, entry.Name())
		if _, err := pm.Load(pluginPath); err != nil {
			continue
		}
	}
}

func (pm *PluginManager) Load(path string) (*Plugin, error) {
	manifestPath := filepath.Join(path, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin manifest: %w", err)
	}

	var plugin Plugin
	if err := json.Unmarshal(data, &plugin); err != nil {
		return nil, fmt.Errorf("failed to parse plugin manifest: %w", err)
	}

	plugin.Path = path
	if plugin.InstalledAt.IsZero() {
		plugin.InstalledAt = time.Now()
	}

	pm.mu.Lock()
	pm.plugins[plugin.Name] = &plugin
	pm.mu.Unlock()

	return &plugin, nil
}

func (pm *PluginManager) LoadAll(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(dir, entry.Name())
		if _, err := pm.Load(pluginPath); err != nil {
			continue
		}
	}

	return nil
}

func (pm *PluginManager) Install(source string) (*Plugin, error) {
	var plugin *Plugin
	var err error

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		plugin, err = pm.installFromURL(source)
	} else if strings.HasPrefix(source, "github:") {
		plugin, err = pm.installFromGitHub(strings.TrimPrefix(source, "github:"))
	} else {
		plugin, err = pm.installFromLocal(source)
	}

	if err != nil {
		return nil, err
	}

	plugin.InstalledAt = time.Now()
	plugin.Enabled = true

	if pm.onInstall != nil {
		pm.onInstall(plugin)
	}

	return plugin, nil
}

func (pm *PluginManager) installFromURL(url string) (*Plugin, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download plugin: status %d", resp.StatusCode)
	}

	pluginName := filepath.Base(url)
	pluginDir := filepath.Join(pm.pluginsDir, pluginName)

	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return nil, err
	}

	manifestPath := filepath.Join(pluginDir, "plugin.json")
	file, err := os.Create(manifestPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return nil, err
	}

	return pm.Load(pluginDir)
}

func (pm *PluginManager) installFromGitHub(repo string) (*Plugin, error) {
	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/plugin.json", repo)
	return pm.installFromURL(url)
}

func (pm *PluginManager) installFromLocal(path string) (*Plugin, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	pluginName := filepath.Base(absPath)
	targetDir := filepath.Join(pm.pluginsDir, pluginName)

	if err := copyDir(absPath, targetDir); err != nil {
		return nil, err
	}

	return pm.Load(targetDir)
}

func (pm *PluginManager) Update(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, ok := pm.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if plugin.Repository == "" {
		return fmt.Errorf("plugin has no repository URL")
	}

	updated, err := pm.installFromURL(plugin.Repository + "/plugin.json")
	if err != nil {
		return err
	}

	updated.UpdatedAt = time.Now()
	pm.plugins[name] = updated

	return nil
}

func (pm *PluginManager) Uninstall(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, ok := pm.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	if pm.onUninstall != nil {
		pm.onUninstall(plugin)
	}

	if err := os.RemoveAll(plugin.Path); err != nil {
		return err
	}

	delete(pm.plugins, name)
	return nil
}

func (pm *PluginManager) Search(query string) ([]MarketplacePlugin, error) {
	return []MarketplacePlugin{
		{
			Name:        "example-plugin",
			Version:     "1.0.0",
			Description: "An example plugin",
			Author:      "weimengmeng 天气晴",
			Repository:  "https://github.com/example/plugin",
			Tags:        []string{"example", "demo"},
			Downloads:   100,
		},
	}, nil
}

func (pm *PluginManager) CheckDependencies(name string) ([]Dependency, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plugin, ok := pm.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", name)
	}

	var missing []Dependency
	for _, dep := range plugin.Dependencies {
		if _, ok := pm.plugins[dep.Name]; !ok {
			missing = append(missing, dep)
		}
	}

	return missing, nil
}

func (pm *PluginManager) Get(name string) *Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.plugins[name]
}

func (pm *PluginManager) List() []*Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*Plugin, 0, len(pm.plugins))
	for _, p := range pm.plugins {
		result = append(result, p)
	}
	return result
}

func (pm *PluginManager) Enable(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, ok := pm.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	plugin.Enabled = true

	if pm.onEnable != nil {
		pm.onEnable(plugin)
	}

	return nil
}

func (pm *PluginManager) Disable(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, ok := pm.plugins[name]
	if !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	plugin.Enabled = false

	if pm.onDisable != nil {
		pm.onDisable(plugin)
	}

	return nil
}

func (pm *PluginManager) Unload(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, ok := pm.plugins[name]; !ok {
		return fmt.Errorf("plugin not found: %s", name)
	}

	delete(pm.plugins, name)
	return nil
}

func (pm *PluginManager) SetLifecycleHooks(
	onInstall func(*Plugin),
	onUninstall func(*Plugin),
	onEnable func(*Plugin),
	onDisable func(*Plugin),
) {
	pm.onInstall = onInstall
	pm.onUninstall = onUninstall
	pm.onEnable = onEnable
	pm.onDisable = onDisable
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}
