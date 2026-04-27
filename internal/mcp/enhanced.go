package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/config"
)

type ServerConfig struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Type        string            `json:"type"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	AutoStart   bool              `json:"auto_start"`
	Description string            `json:"description,omitempty"`
}

type MCPServerRegistry struct {
	servers   map[string]*ServerConfig
	clients   map[string]*McpClient
	configDir string
	mu        sync.RWMutex
}

func NewMCPServerRegistry() *MCPServerRegistry {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".smartclaw", "mcp")

	registry := &MCPServerRegistry{
		servers:   make(map[string]*ServerConfig),
		clients:   make(map[string]*McpClient),
		configDir: configDir,
	}

	registry.load()
	return registry
}

func (r *MCPServerRegistry) load() error {
	path := filepath.Join(r.configDir, "servers.json")
	servers, err := config.LoadJSON[[]*ServerConfig](path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, s := range *servers {
		r.servers[s.Name] = s
	}

	return nil
}

func (r *MCPServerRegistry) save() error {
	if err := os.MkdirAll(r.configDir, 0755); err != nil {
		return err
	}

	servers := make([]*ServerConfig, 0, len(r.servers))
	for _, s := range r.servers {
		servers = append(servers, s)
	}

	path := filepath.Join(r.configDir, "servers.json")
	return config.SaveJSON(path, &servers)
}

func (r *MCPServerRegistry) AddServer(config *ServerConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.servers[config.Name]; exists {
		return fmt.Errorf("server already exists: %s", config.Name)
	}

	r.servers[config.Name] = config
	return r.save()
}

func (r *MCPServerRegistry) RemoveServer(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.servers[name]; !exists {
		return fmt.Errorf("server not found: %s", name)
	}

	delete(r.servers, name)
	return r.save()
}

func (r *MCPServerRegistry) GetServer(name string) (*ServerConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.servers[name]
	return config, exists
}

func (r *MCPServerRegistry) ListServers() []*ServerConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ServerConfig, 0, len(r.servers))
	for _, s := range r.servers {
		result = append(result, s)
	}

	return result
}

func (r *MCPServerRegistry) UpdateServer(name string, updates map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, exists := r.servers[name]
	if !exists {
		return fmt.Errorf("server not found: %s", name)
	}

	if cmd, ok := updates["command"].(string); ok {
		config.Command = cmd
	}
	if args, ok := updates["args"].([]string); ok {
		config.Args = args
	}
	if autoStart, ok := updates["auto_start"].(bool); ok {
		config.AutoStart = autoStart
	}

	return r.save()
}

func (r *MCPServerRegistry) StartServer(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, exists := r.servers[name]
	if !exists {
		return fmt.Errorf("server not found: %s", name)
	}

	if client, alreadyRunning := r.clients[name]; alreadyRunning && client.IsReady() {
		return fmt.Errorf("server already running: %s", name)
	}

	mcpConfig := &McpServerConfig{
		Name:      config.Name,
		Transport: config.Type,
		Command:   config.Command,
		Args:      config.Args,
		Env:       config.Env,
		URL:       config.URL,
		Headers:   config.Headers,
	}

	client, err := NewClientFromConfig(ctx, mcpConfig)
	if err != nil {
		return fmt.Errorf("failed to start server %s: %w", name, err)
	}

	r.clients[name] = client
	return nil
}

func (r *MCPServerRegistry) StopServer(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, exists := r.clients[name]
	if !exists {
		return fmt.Errorf("server not running: %s", name)
	}

	if err := client.Disconnect(); err != nil {
		return fmt.Errorf("failed to stop server %s: %w", name, err)
	}

	delete(r.clients, name)
	return nil
}

func (r *MCPServerRegistry) IsServerRunning(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	client, exists := r.clients[name]
	if !exists {
		return false
	}
	return client.IsReady()
}

func (r *MCPServerRegistry) GetClient(name string) (*McpClient, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	client, exists := r.clients[name]
	return client, exists
}

type MCPOAuthFlow struct {
	ServerName string
	AuthURL    string
	Token      string
	ExpiresAt  time.Time
	Completed  bool
}

type MCPAuthManager struct {
	flows map[string]*MCPOAuthFlow
	mu    sync.RWMutex
}

func NewMCPAuthManager() *MCPAuthManager {
	return &MCPAuthManager{
		flows: make(map[string]*MCPOAuthFlow),
	}
}

func (m *MCPAuthManager) StartFlow(serverName, authURL string) *MCPOAuthFlow {
	m.mu.Lock()
	defer m.mu.Unlock()

	flow := &MCPOAuthFlow{
		ServerName: serverName,
		AuthURL:    authURL,
		Completed:  false,
	}

	m.flows[serverName] = flow
	return flow
}

func (m *MCPAuthManager) CompleteFlow(serverName, token string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	flow, exists := m.flows[serverName]
	if !exists {
		return fmt.Errorf("flow not found: %s", serverName)
	}

	flow.Token = token
	flow.ExpiresAt = expiresAt
	flow.Completed = true

	return nil
}

func (m *MCPAuthManager) GetFlow(serverName string) (*MCPOAuthFlow, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flow, exists := m.flows[serverName]
	return flow, exists
}

func (m *MCPAuthManager) IsAuthenticated(serverName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flow, exists := m.flows[serverName]
	if !exists {
		return false
	}

	return flow.Completed && time.Now().Before(flow.ExpiresAt)
}

func (m *MCPAuthManager) GetToken(serverName string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flow, exists := m.flows[serverName]
	if !exists || !flow.Completed {
		return "", false
	}

	if time.Now().After(flow.ExpiresAt) {
		return "", false
	}

	return flow.Token, true
}

type MCPChannel struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	ServerName string    `json:"server_name"`
	Allowlist  []string  `json:"allowlist"`
	CreatedAt  time.Time `json:"created_at"`
}

type MCPChannelManager struct {
	channels map[string]*MCPChannel
	mu       sync.RWMutex
}

func NewMCPChannelManager() *MCPChannelManager {
	return &MCPChannelManager{
		channels: make(map[string]*MCPChannel),
	}
}

func (m *MCPChannelManager) CreateChannel(name, serverName string, allowlist []string) *MCPChannel {
	m.mu.Lock()
	defer m.mu.Unlock()

	channel := &MCPChannel{
		ID:         fmt.Sprintf("ch_%d", time.Now().UnixNano()),
		Name:       name,
		ServerName: serverName,
		Allowlist:  allowlist,
		CreatedAt:  time.Now(),
	}

	m.channels[channel.ID] = channel
	return channel
}

func (m *MCPChannelManager) GetChannel(id string) (*MCPChannel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, exists := m.channels[id]
	return channel, exists
}

func (m *MCPChannelManager) ListChannels() []*MCPChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MCPChannel, 0, len(m.channels))
	for _, ch := range m.channels {
		result = append(result, ch)
	}

	return result
}

func (m *MCPChannelManager) IsToolAllowed(channelID, toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channel, exists := m.channels[channelID]
	if !exists {
		return false
	}

	for _, t := range channel.Allowlist {
		if t == toolName || t == "*" {
			return true
		}
	}

	return false
}

func expandEnv(value string) string {
	return os.ExpandEnv(value)
}

func expandEnvMap(m map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[k] = expandEnv(v)
	}
	return result
}

func normalizeServerName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func buildMCPToolName(serverName, toolName string) string {
	return fmt.Sprintf("mcp__%s__%s", normalizeServerName(serverName), toolName)
}

func parseMCPToolName(fullName string) (serverName, toolName string, ok bool) {
	parts := strings.Split(fullName, "__")
	if len(parts) != 3 || parts[0] != "mcp" {
		return "", "", false
	}
	return parts[1], parts[2], true
}
