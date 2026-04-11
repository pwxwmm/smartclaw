package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ServerConfig struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env,omitempty"`
	Type        string            `json:"type"`
	URL         string            `json:"url,omitempty"`
	AutoStart   bool              `json:"auto_start"`
	Description string            `json:"description,omitempty"`
}

type MCPServerRegistry struct {
	servers   map[string]*ServerConfig
	configDir string
	mu        sync.RWMutex
}

func NewMCPServerRegistry() *MCPServerRegistry {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".smartclaw", "mcp")

	registry := &MCPServerRegistry{
		servers:   make(map[string]*ServerConfig),
		configDir: configDir,
	}

	registry.load()
	return registry
}

func (r *MCPServerRegistry) load() error {
	path := filepath.Join(r.configDir, "servers.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var servers []*ServerConfig
	if err := json.Unmarshal(data, &servers); err != nil {
		return err
	}

	for _, s := range servers {
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

	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(r.configDir, "servers.json")
	return os.WriteFile(path, data, 0644)
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

type InProcessTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	mu     sync.Mutex
}

func NewInProcessTransport(command string, args []string, env map[string]string) (*InProcessTransport, error) {
	cmd := exec.Command(command, args...)

	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &InProcessTransport{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
	}, nil
}

func (t *InProcessTransport) Send(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	_, err := t.stdin.Write(data)
	_, err2 := t.stdin.Write([]byte("\n"))
	if err != nil {
		return err
	}
	return err2
}

func (t *InProcessTransport) Receive() ([]byte, error) {
	buf := make([]byte, 4096)
	n, err := t.stdout.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (t *InProcessTransport) Close() error {
	if t.cmd.Process != nil {
		t.cmd.Process.Kill()
	}
	return nil
}
