//go:build desktop
// +build desktop

package desktop

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/config"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/plugins"
	"github.com/instructkr/smartclaw/internal/runtime"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	engine   *runtime.QueryEngine
	memMgr   *memory.MemoryManager
	pluginMgr *plugins.PluginManager
	cfg      *config.Config
	tray     *TrayManager
	mu       sync.RWMutex
	ctx      context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := config.Load("")
	if err != nil {
		slog.Warn("desktop: config load failed, using defaults", "error", err)
		cfg = config.Default()
	}
	a.cfg = cfg

	client := api.NewClientWithModel(cfg.APIKey, cfg.BaseURL, cfg.Model)

	memMgr, err := memory.NewMemoryManager()
	if err != nil {
		slog.Error("desktop: memory manager init failed", "error", err)
	} else {
		a.memMgr = memMgr
	}

	engine := runtime.NewQueryEngine(client, runtime.QueryConfig{
		Model:               cfg.Model,
		MaxTokens:           cfg.MaxTokens,
		EnableLLMCompaction: true,
	})

	if memMgr != nil {
		engine.SetMemoryManager(memMgr)
		memMgr.SetLLMClient(client)
	}

	a.engine = engine
	a.pluginMgr = plugins.NewPluginManager()
	a.tray = NewTrayManager(a)

	slog.Info("desktop: application started")
}

func (a *App) Shutdown(ctx context.Context) {
	if a.engine != nil {
		a.engine.Close()
	}
	if a.memMgr != nil {
		a.memMgr.Close()
	}
	slog.Info("desktop: application shutdown")
}

func (a *App) SendPrompt(prompt string) (string, error) {
	if a.engine == nil {
		return "", fmt.Errorf("engine not initialized")
	}

	result, err := a.engine.Query(a.ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}

	content, ok := result.Message.Content.(string)
	if !ok {
		return fmt.Sprintf("%v", result.Message.Content), nil
	}
	return content, nil
}

func (a *App) GetMemory() (map[string]string, error) {
	if a.memMgr == nil {
		return nil, fmt.Errorf("memory manager not initialized")
	}

	pm := a.memMgr.GetPromptMemory()
	result := make(map[string]string)

	if pm != nil {
		result["MEMORY.md"] = pm.GetMemoryContent()
		result["USER.md"] = pm.GetUserContent()
	}

	home, err := os.UserHomeDir()
	if err == nil {
		soulPath := filepath.Join(home, ".smartclaw", "SOUL.md")
		if data, err := os.ReadFile(soulPath); err == nil {
			result["SOUL.md"] = string(data)
		}
	}

	return result, nil
}

func (a *App) GetPlugins() ([]plugins.Plugin, error) {
	if a.pluginMgr == nil {
		return nil, fmt.Errorf("plugin manager not initialized")
	}

	list := a.pluginMgr.List()
	result := make([]plugins.Plugin, len(list))
	for i, p := range list {
		result[i] = *p
	}
	return result, nil
}

func (a *App) InstallPlugin(source string) error {
	if a.pluginMgr == nil {
		return fmt.Errorf("plugin manager not initialized")
	}

	_, err := a.pluginMgr.Install(source)
	if err != nil {
		return fmt.Errorf("plugin install failed: %w", err)
	}
	return nil
}

func (a *App) GetConfig() (map[string]any, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	result := map[string]any{
		"model":         a.cfg.Model,
		"base_url":      a.cfg.BaseURL,
		"max_tokens":    a.cfg.MaxTokens,
		"temperature":   a.cfg.Temperature,
		"permission":    a.cfg.Permission,
		"log_level":     a.cfg.LogLevel,
		"openai":        a.cfg.OpenAI,
		"show_thinking": a.cfg.ShowThinking,
		"voice_enabled": a.cfg.VoiceEnabled,
		"theme":         a.cfg.Theme,
		"language":      a.cfg.Language,
		"auto_save":     a.cfg.AutoSave,
	}

	if len(a.cfg.Providers) > 0 {
		result["providers"] = a.cfg.Providers
	}

	return result, nil
}

func (a *App) SetConfig(key, value string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	switch key {
	case "model":
		a.cfg.Model = value
	case "base_url":
		a.cfg.BaseURL = value
	case "api_key":
		a.cfg.APIKey = value
	case "permission":
		a.cfg.Permission = value
	case "log_level":
		a.cfg.LogLevel = value
	case "theme":
		a.cfg.Theme = value
	case "language":
		a.cfg.Language = value
	case "editor":
		a.cfg.Editor = value
	case "openai":
		a.cfg.OpenAI = value == "true"
	case "show_thinking":
		a.cfg.ShowThinking = value == "true"
	case "voice_enabled":
		a.cfg.VoiceEnabled = value == "true"
	case "auto_save":
		a.cfg.AutoSave = value == "true"
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home dir: %w", err)
	}
	configPath := filepath.Join(home, ".smartclaw", "config.yaml")

	if err := config.Save(a.cfg, configPath); err != nil {
		return fmt.Errorf("config save failed: %w", err)
	}

	if key == "model" && a.engine != nil {
		a.engine.SetSystemPrompt("")
	}

	slog.Info("desktop: config updated", "key", key, "value", value)
	return nil
}

func (a *App) ShowWindow() {
	if a.ctx != nil {
		wailsruntime.WindowShow(a.ctx)
	}
}

func (a *App) HideWindow() {
	if a.ctx != nil {
		wailsruntime.WindowHide(a.ctx)
	}
}

func (a *App) Quit() {
	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
}

func (a *App) Emit(event string, data interface{}) {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, event, data)
	}
}

func (a *App) Tray() *TrayManager {
	return a.tray
}
