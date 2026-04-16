package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIKey       string                    `yaml:"api_key"`
	Model        string                    `yaml:"model"`
	BaseURL      string                    `yaml:"base_url"`
	MaxTokens    int                       `yaml:"max_tokens"`
	Temperature  float64                   `yaml:"temperature"`
	Permission   string                    `yaml:"permission"`
	Plugins      []string                  `yaml:"plugins"`
	MCPServers   map[string]MCPServer      `yaml:"mcp_servers"`
	Hooks        map[string][]Hook         `yaml:"hooks"`
	Custom       map[string]any            `yaml:"custom"`
	SessionDir   string                    `yaml:"session_dir"`
	LogLevel     string                    `yaml:"log_level"`
	VoiceEnabled bool                      `yaml:"voice_enabled"`
	ShowThinking bool                      `yaml:"show_thinking"`
	OpenAI       bool                      `yaml:"openai"`
	Session      string                    `yaml:"session"`
	Editor       string                    `yaml:"editor"`
	Theme        string                    `yaml:"theme"`
	Language     string                    `yaml:"language"`
	AutoSave     bool                      `yaml:"auto_save"`
	Routing      RoutingConfig             `yaml:"routing"`
	Provider     string                    `yaml:"provider,omitempty"`
	Providers    map[string]ProviderConfig `yaml:"providers,omitempty"`
}

type ProviderConfig struct {
	APIKey  string `yaml:"api_key" json:"apiKey"`
	BaseURL string `yaml:"base_url,omitempty" json:"baseUrl,omitempty"`
	Model   string `yaml:"model,omitempty" json:"model,omitempty"`
}

type RoutingConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Strategy     string `yaml:"strategy"`
	FastModel    string `yaml:"fast_model"`
	DefaultModel string `yaml:"default_model"`
	HeavyModel   string `yaml:"heavy_model"`
}

type MCPServer struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

type Hook struct {
	Type    string   `yaml:"type"`
	Command string   `yaml:"command"`
	Tools   []string `yaml:"tools"`
}

var defaultConfig = &Config{
	Model:        "sre-model",
	BaseURL:      "",
	MaxTokens:    4096,
	Temperature:  1.0,
	Permission:   "ask",
	Plugins:      []string{},
	MCPServers:   make(map[string]MCPServer),
	Hooks:        make(map[string][]Hook),
	Custom:       make(map[string]any),
	LogLevel:     "info",
	VoiceEnabled: false,
	ShowThinking: true,
	OpenAI:       true,
	Theme:        "dark",
	Language:     "zh-CN",
	AutoSave:     true,
}

func Default() *Config {
	return defaultConfig
}

func Load(path string) (*Config, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, ".smartclaw", "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConfig, nil
		}
		return nil, err
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	mergeDefaults(config)

	return config, nil
}

func Save(config *Config, path string) error {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path = filepath.Join(home, ".smartclaw", "config.yaml")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func mergeDefaults(config *Config) {
	if config.Model == "" {
		config.Model = defaultConfig.Model
	}

	if config.MaxTokens == 0 {
		config.MaxTokens = defaultConfig.MaxTokens
	}
	if config.Temperature == 0 {
		config.Temperature = defaultConfig.Temperature
	}
	if config.Permission == "" {
		config.Permission = defaultConfig.Permission
	}
	if config.Plugins == nil {
		config.Plugins = []string{}
	}
	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServer)
	}
	if config.Hooks == nil {
		config.Hooks = make(map[string][]Hook)
	}
	if config.Custom == nil {
		config.Custom = make(map[string]any)
	}
	if config.LogLevel == "" {
		config.LogLevel = defaultConfig.LogLevel
	}
}

func GetAPIKey(config *Config) string {
	if config.APIKey != "" {
		return config.APIKey
	}
	return os.Getenv("ANTHROPIC_API_KEY")
}

func GetConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".smartclaw", "config.yaml"), nil
}

func Exists() bool {
	path, err := GetConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func Set(key string, value any) error {
	config, err := Load("")
	if err != nil {
		return err
	}

	switch strings.ToLower(key) {
	case "api_key":
		config.APIKey = fmt.Sprint(value)
	case "model":
		config.Model = fmt.Sprint(value)
	case "base_url":
		config.BaseURL = fmt.Sprint(value)
	case "max_tokens":
		if v, ok := value.(int); ok {
			config.MaxTokens = v
		}
	case "temperature":
		if v, ok := value.(float64); ok {
			config.Temperature = v
		}
	case "permission":
		config.Permission = fmt.Sprint(value)
	case "log_level":
		config.LogLevel = fmt.Sprint(value)
	case "voice_enabled":
		if v, ok := value.(bool); ok {
			config.VoiceEnabled = v
		}
	case "show_thinking":
		if v, ok := value.(bool); ok {
			config.ShowThinking = v
		}
	default:
		config.Custom[key] = value
	}

	return Save(config, "")
}

func Get(key string) (any, error) {
	config, err := Load("")
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(key) {
	case "api_key":
		return config.APIKey, nil
	case "model":
		return config.Model, nil
	case "base_url":
		return config.BaseURL, nil
	case "max_tokens":
		return config.MaxTokens, nil
	case "temperature":
		return config.Temperature, nil
	case "permission":
		return config.Permission, nil
	case "log_level":
		return config.LogLevel, nil
	case "voice_enabled":
		return config.VoiceEnabled, nil
	case "show_thinking":
		return config.ShowThinking, nil
	case "editor":
		return config.Editor, nil
	case "theme":
		return config.Theme, nil
	case "language":
		return config.Language, nil
	case "auto_save":
		return config.AutoSave, nil
	default:
		if v, ok := config.Custom[key]; ok {
			return v, nil
		}
		return nil, fmt.Errorf("config key not found: %s", key)
	}
}

func Reset(key string) error {
	if key == "" || key == "all" {
		return Save(defaultConfig, "")
	}

	config, err := Load("")
	if err != nil {
		return err
	}

	switch strings.ToLower(key) {
	case "model":
		config.Model = defaultConfig.Model
	case "base_url":
		config.BaseURL = defaultConfig.BaseURL
	case "max_tokens":
		config.MaxTokens = defaultConfig.MaxTokens
	case "temperature":
		config.Temperature = defaultConfig.Temperature
	case "permission":
		config.Permission = defaultConfig.Permission
	case "log_level":
		config.LogLevel = defaultConfig.LogLevel
	case "theme":
		config.Theme = defaultConfig.Theme
	case "language":
		config.Language = defaultConfig.Language
	default:
		delete(config.Custom, key)
	}

	return Save(config, "")
}

func List() ([]string, error) {
	config, err := Load("")
	if err != nil {
		return nil, err
	}

	var keys []string
	keys = append(keys, "api_key", "model", "base_url", "max_tokens", "temperature",
		"permission", "log_level", "voice_enabled", "show_thinking", "editor",
		"theme", "language", "auto_save")

	for key := range config.Custom {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys, nil
}

func Show() (string, error) {
	config, err := Load("")
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("╭─────────────────────────────────────────────────╮\n")
	sb.WriteString("│               SmartClaw 配置                     │\n")
	sb.WriteString("╰─────────────────────────────────────────────────╯\n\n")

	sb.WriteString("  核心配置:\n")
	sb.WriteString(fmt.Sprintf("    model:        %s\n", config.Model))
	sb.WriteString(fmt.Sprintf("    base_url:     %s\n", config.BaseURL))
	sb.WriteString(fmt.Sprintf("    max_tokens:   %d\n", config.MaxTokens))
	sb.WriteString(fmt.Sprintf("    temperature:  %.2f\n", config.Temperature))
	sb.WriteString(fmt.Sprintf("    permission:   %s\n", config.Permission))

	sb.WriteString("\n  显示配置:\n")
	sb.WriteString(fmt.Sprintf("    theme:        %s\n", config.Theme))
	sb.WriteString(fmt.Sprintf("    language:     %s\n", config.Language))
	sb.WriteString(fmt.Sprintf("    show_thinking: %v\n", config.ShowThinking))
	sb.WriteString(fmt.Sprintf("    log_level:    %s\n", config.LogLevel))

	sb.WriteString("\n  功能配置:\n")
	sb.WriteString(fmt.Sprintf("    voice_enabled: %v\n", config.VoiceEnabled))
	sb.WriteString(fmt.Sprintf("    auto_save:    %v\n", config.AutoSave))
	sb.WriteString(fmt.Sprintf("    editor:       %s\n", config.Editor))

	if len(config.MCPServers) > 0 {
		sb.WriteString("\n  MCP 服务器:\n")
		for name, server := range config.MCPServers {
			sb.WriteString(fmt.Sprintf("    %s: %s\n", name, server.Command))
		}
	}

	if len(config.Plugins) > 0 {
		sb.WriteString("\n  插件:\n")
		for _, plugin := range config.Plugins {
			sb.WriteString(fmt.Sprintf("    - %s\n", plugin))
		}
	}

	if len(config.Custom) > 0 {
		sb.WriteString("\n  自定义配置:\n")
		for key, value := range config.Custom {
			sb.WriteString(fmt.Sprintf("    %s: %v\n", key, value))
		}
	}

	return sb.String(), nil
}

func Export(path string, format string) error {
	config, err := Load("")
	if err != nil {
		return err
	}

	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(config, "", "  ")
	case "yaml", "yml":
		data, err = yaml.Marshal(config)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return err
	}

	if path == "" {
		home, _ := os.UserHomeDir()
		ext := "yaml"
		if format == "json" {
			ext = "json"
		}
		path = filepath.Join(home, ".smartclaw", "exports", fmt.Sprintf("config_export.%s", ext))
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func Import(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var config Config

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return err
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	mergeDefaults(&config)

	return Save(&config, "")
}

func Validate(config *Config) []string {
	var errors []string

	if config.Model == "" {
		errors = append(errors, "model is required")
	}

	if config.MaxTokens < 1 || config.MaxTokens > 200000 {
		errors = append(errors, "max_tokens must be between 1 and 200000")
	}

	if config.Temperature < 0 || config.Temperature > 2 {
		errors = append(errors, "temperature must be between 0 and 2")
	}

	validPermissions := map[string]bool{
		"ask": true, "read-only": true, "workspace-write": true, "danger-full-access": true,
	}
	if !validPermissions[config.Permission] {
		errors = append(errors, "invalid permission mode")
	}

	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[config.LogLevel] {
		errors = append(errors, "invalid log level")
	}

	return errors
}

func (c *Config) ResolveProvider(name string) (apiKey, baseURL, model string, err error) {
	if name == "" || name == "default" {
		return c.APIKey, c.BaseURL, c.Model, nil
	}

	if c.Providers != nil {
		if pc, ok := c.Providers[name]; ok {
			model = pc.Model
			if model == "" {
				model = c.Model
			}
			return pc.APIKey, pc.BaseURL, model, nil
		}
	}

	return "", "", "", fmt.Errorf("provider %q not found", name)
}
