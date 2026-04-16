package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestRegistry(t *testing.T) *MCPServerRegistry {
	t.Helper()
	dir := t.TempDir()
	r := &MCPServerRegistry{
		servers:   make(map[string]*ServerConfig),
		configDir: dir,
	}
	return r
}

func TestMCPServerRegistry_AddServer(t *testing.T) {
	r := newTestRegistry(t)

	cfg := &ServerConfig{
		Name:    "test-server",
		Command: "npx",
		Args:    []string{"-y", "@test/server"},
		Type:    "stdio",
	}

	err := r.AddServer(cfg)
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	got, exists := r.GetServer("test-server")
	if !exists {
		t.Error("server should exist after adding")
	}
	if got.Name != "test-server" {
		t.Errorf("expected name=test-server, got %s", got.Name)
	}
	if got.Command != "npx" {
		t.Errorf("expected command=npx, got %s", got.Command)
	}
}

func TestMCPServerRegistry_AddServer_Duplicate(t *testing.T) {
	r := newTestRegistry(t)

	cfg := &ServerConfig{Name: "dup-server", Command: "cmd1"}
	if err := r.AddServer(cfg); err != nil {
		t.Fatalf("first AddServer: %v", err)
	}

	cfg2 := &ServerConfig{Name: "dup-server", Command: "cmd2"}
	err := r.AddServer(cfg2)
	if err == nil {
		t.Error("expected error for duplicate server name")
	}
}

func TestMCPServerRegistry_RemoveServer(t *testing.T) {
	r := newTestRegistry(t)

	cfg := &ServerConfig{Name: "to-remove", Command: "cmd"}
	r.AddServer(cfg)

	err := r.RemoveServer("to-remove")
	if err != nil {
		t.Fatalf("RemoveServer failed: %v", err)
	}

	_, exists := r.GetServer("to-remove")
	if exists {
		t.Error("server should not exist after removal")
	}
}

func TestMCPServerRegistry_RemoveServer_NotFound(t *testing.T) {
	r := newTestRegistry(t)

	err := r.RemoveServer("nonexistent")
	if err == nil {
		t.Error("expected error for removing nonexistent server")
	}
}

func TestMCPServerRegistry_GetServer(t *testing.T) {
	r := newTestRegistry(t)

	_, exists := r.GetServer("missing")
	if exists {
		t.Error("should not find missing server")
	}

	cfg := &ServerConfig{Name: "found", Command: "cmd"}
	r.AddServer(cfg)

	got, exists := r.GetServer("found")
	if !exists {
		t.Error("should find existing server")
	}
	if got.Name != "found" {
		t.Errorf("expected name=found, got %s", got.Name)
	}
}

func TestMCPServerRegistry_ListServers(t *testing.T) {
	r := newTestRegistry(t)

	r.AddServer(&ServerConfig{Name: "s1", Command: "c1"})
	r.AddServer(&ServerConfig{Name: "s2", Command: "c2"})
	r.AddServer(&ServerConfig{Name: "s3", Command: "c3"})

	list := r.ListServers()
	if len(list) != 3 {
		t.Errorf("expected 3 servers, got %d", len(list))
	}

	names := map[string]bool{}
	for _, s := range list {
		names[s.Name] = true
	}
	for _, n := range []string{"s1", "s2", "s3"} {
		if !names[n] {
			t.Errorf("missing server %s", n)
		}
	}
}

func TestMCPServerRegistry_ListServers_Empty(t *testing.T) {
	r := newTestRegistry(t)

	list := r.ListServers()
	if len(list) != 0 {
		t.Errorf("expected 0 servers, got %d", len(list))
	}
}

func TestMCPServerRegistry_UpdateServer(t *testing.T) {
	r := newTestRegistry(t)

	cfg := &ServerConfig{Name: "updatable", Command: "old-cmd", Args: []string{"old"}, AutoStart: false}
	r.AddServer(cfg)

	err := r.UpdateServer("updatable", map[string]any{
		"command":    "new-cmd",
		"args":       []string{"new", "args"},
		"auto_start": true,
	})
	if err != nil {
		t.Fatalf("UpdateServer failed: %v", err)
	}

	got, _ := r.GetServer("updatable")
	if got.Command != "new-cmd" {
		t.Errorf("expected command=new-cmd, got %s", got.Command)
	}
	if len(got.Args) != 2 || got.Args[0] != "new" {
		t.Errorf("expected args=[new args], got %v", got.Args)
	}
	if !got.AutoStart {
		t.Error("expected AutoStart=true")
	}
}

func TestMCPServerRegistry_UpdateServer_NotFound(t *testing.T) {
	r := newTestRegistry(t)

	err := r.UpdateServer("missing", map[string]any{"command": "cmd"})
	if err == nil {
		t.Error("expected error for updating nonexistent server")
	}
}

func TestMCPServerRegistry_UpdateServer_IgnoresUnknownFields(t *testing.T) {
	r := newTestRegistry(t)

	cfg := &ServerConfig{Name: "test", Command: "cmd"}
	r.AddServer(cfg)

	err := r.UpdateServer("test", map[string]any{
		"unknown_field": "value",
	})
	if err != nil {
		t.Fatalf("UpdateServer should not fail for unknown fields: %v", err)
	}

	got, _ := r.GetServer("test")
	if got.Command != "cmd" {
		t.Error("command should remain unchanged")
	}
}

func TestMCPServerRegistry_PersistAndLoad(t *testing.T) {
	dir := t.TempDir()

	r1 := &MCPServerRegistry{
		servers:   make(map[string]*ServerConfig),
		configDir: dir,
	}

	r1.AddServer(&ServerConfig{
		Name:    "persist-test",
		Command: "npx",
		Args:    []string{"-y", "@test/server"},
		Type:    "stdio",
		Env:     map[string]string{"KEY": "value"},
	})

	r2 := &MCPServerRegistry{
		servers:   make(map[string]*ServerConfig),
		configDir: dir,
	}
	r2.load()

	got, exists := r2.GetServer("persist-test")
	if !exists {
		t.Fatal("server should persist across registry instances")
	}
	if got.Command != "npx" {
		t.Errorf("expected command=npx, got %s", got.Command)
	}
	if got.Env["KEY"] != "value" {
		t.Errorf("expected env KEY=value, got %s", got.Env["KEY"])
	}
}

func TestMCPServerRegistry_PersistFile(t *testing.T) {
	r := newTestRegistry(t)

	r.AddServer(&ServerConfig{Name: "s1", Command: "c1"})
	r.AddServer(&ServerConfig{Name: "s2", Command: "c2"})

	data, err := os.ReadFile(filepath.Join(r.configDir, "servers.json"))
	if err != nil {
		t.Fatalf("read servers.json: %v", err)
	}

	var servers []*ServerConfig
	if err := json.Unmarshal(data, &servers); err != nil {
		t.Fatalf("unmarshal servers.json: %v", err)
	}

	if len(servers) != 2 {
		t.Errorf("expected 2 servers in file, got %d", len(servers))
	}
}

func TestMCPAuthManager_StartFlow(t *testing.T) {
	m := NewMCPAuthManager()

	flow := m.StartFlow("test-server", "https://auth.example.com/oauth")
	if flow == nil {
		t.Fatal("StartFlow returned nil")
	}
	if flow.ServerName != "test-server" {
		t.Errorf("expected ServerName=test-server, got %s", flow.ServerName)
	}
	if flow.AuthURL != "https://auth.example.com/oauth" {
		t.Errorf("unexpected AuthURL: %s", flow.AuthURL)
	}
	if flow.Completed {
		t.Error("new flow should not be completed")
	}
	if flow.Token != "" {
		t.Error("new flow should not have a token")
	}
}

func TestMCPAuthManager_GetFlow(t *testing.T) {
	m := NewMCPAuthManager()

	_, exists := m.GetFlow("missing")
	if exists {
		t.Error("should not find missing flow")
	}

	m.StartFlow("test-server", "https://auth.example.com")
	flow, exists := m.GetFlow("test-server")
	if !exists {
		t.Error("should find existing flow")
	}
	if flow.AuthURL != "https://auth.example.com" {
		t.Errorf("unexpected AuthURL: %s", flow.AuthURL)
	}
}

func TestMCPAuthManager_CompleteFlow(t *testing.T) {
	m := NewMCPAuthManager()

	m.StartFlow("test-server", "https://auth.example.com")
	expiresAt := time.Now().Add(1 * time.Hour)

	err := m.CompleteFlow("test-server", "oauth-token-123", expiresAt)
	if err != nil {
		t.Fatalf("CompleteFlow failed: %v", err)
	}

	flow, _ := m.GetFlow("test-server")
	if !flow.Completed {
		t.Error("flow should be completed")
	}
	if flow.Token != "oauth-token-123" {
		t.Errorf("expected token=oauth-token-123, got %s", flow.Token)
	}
}

func TestMCPAuthManager_CompleteFlow_NotFound(t *testing.T) {
	m := NewMCPAuthManager()

	err := m.CompleteFlow("missing", "token", time.Now().Add(time.Hour))
	if err == nil {
		t.Error("expected error for completing nonexistent flow")
	}
}

func TestMCPAuthManager_IsAuthenticated(t *testing.T) {
	m := NewMCPAuthManager()

	if m.IsAuthenticated("missing") {
		t.Error("missing flow should not be authenticated")
	}

	m.StartFlow("test-server", "https://auth.example.com")

	if m.IsAuthenticated("test-server") {
		t.Error("incomplete flow should not be authenticated")
	}

	m.CompleteFlow("test-server", "token", time.Now().Add(1*time.Hour))

	if !m.IsAuthenticated("test-server") {
		t.Error("completed flow with valid token should be authenticated")
	}
}

func TestMCPAuthManager_IsAuthenticated_Expired(t *testing.T) {
	m := NewMCPAuthManager()

	m.StartFlow("test-server", "https://auth.example.com")
	m.CompleteFlow("test-server", "token", time.Now().Add(-1*time.Hour))

	if m.IsAuthenticated("test-server") {
		t.Error("expired flow should not be authenticated")
	}
}

func TestMCPAuthManager_GetToken(t *testing.T) {
	m := NewMCPAuthManager()

	token, ok := m.GetToken("missing")
	if ok {
		t.Error("missing flow should not return token")
	}

	m.StartFlow("test-server", "https://auth.example.com")
	token, ok = m.GetToken("test-server")
	if ok {
		t.Error("incomplete flow should not return token")
	}

	m.CompleteFlow("test-server", "my-token", time.Now().Add(1*time.Hour))
	token, ok = m.GetToken("test-server")
	if !ok {
		t.Error("completed flow should return token")
	}
	if token != "my-token" {
		t.Errorf("expected token=my-token, got %s", token)
	}
}

func TestMCPAuthManager_GetToken_Expired(t *testing.T) {
	m := NewMCPAuthManager()

	m.StartFlow("test-server", "https://auth.example.com")
	m.CompleteFlow("test-server", "token", time.Now().Add(-1*time.Hour))

	_, ok := m.GetToken("test-server")
	if ok {
		t.Error("expired token should not be returned")
	}
}

func TestMCPChannelManager_CreateChannel(t *testing.T) {
	m := NewMCPChannelManager()

	ch := m.CreateChannel("test-channel", "test-server", []string{"tool1", "tool2"})
	if ch == nil {
		t.Fatal("CreateChannel returned nil")
	}
	if ch.Name != "test-channel" {
		t.Errorf("expected Name=test-channel, got %s", ch.Name)
	}
	if ch.ServerName != "test-server" {
		t.Errorf("expected ServerName=test-server, got %s", ch.ServerName)
	}
	if len(ch.Allowlist) != 2 {
		t.Errorf("expected 2 tools in allowlist, got %d", len(ch.Allowlist))
	}
	if ch.ID == "" {
		t.Error("channel ID should not be empty")
	}
	if ch.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestMCPChannelManager_GetChannel(t *testing.T) {
	m := NewMCPChannelManager()

	_, exists := m.GetChannel("missing")
	if exists {
		t.Error("should not find missing channel")
	}

	ch := m.CreateChannel("test", "server", nil)
	got, exists := m.GetChannel(ch.ID)
	if !exists {
		t.Error("should find existing channel")
	}
	if got.Name != "test" {
		t.Errorf("expected Name=test, got %s", got.Name)
	}
}

func TestMCPChannelManager_ListChannels(t *testing.T) {
	m := NewMCPChannelManager()

	ch1 := m.CreateChannel("ch1", "s1", nil)
	time.Sleep(time.Millisecond)
	ch2 := m.CreateChannel("ch2", "s2", nil)

	list := m.ListChannels()
	if len(list) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(list))
	}

	ids := map[string]bool{ch1.ID: true, ch2.ID: true}
	for _, ch := range list {
		if !ids[ch.ID] {
			t.Errorf("unexpected channel ID: %s", ch.ID)
		}
	}
}

func TestMCPChannelManager_ListChannels_Empty(t *testing.T) {
	m := NewMCPChannelManager()

	list := m.ListChannels()
	if len(list) != 0 {
		t.Errorf("expected 0 channels, got %d", len(list))
	}
}

func TestMCPChannelManager_IsToolAllowed(t *testing.T) {
	m := NewMCPChannelManager()

	ch := m.CreateChannel("test", "server", []string{"read_file", "write_file"})

	if !m.IsToolAllowed(ch.ID, "read_file") {
		t.Error("read_file should be allowed")
	}
	if m.IsToolAllowed(ch.ID, "delete_file") {
		t.Error("delete_file should not be allowed")
	}
}

func TestMCPChannelManager_IsToolAllowed_Wildcard(t *testing.T) {
	m := NewMCPChannelManager()

	ch := m.CreateChannel("test", "server", []string{"*"})

	if !m.IsToolAllowed(ch.ID, "any_tool") {
		t.Error("wildcard should allow any tool")
	}
	if !m.IsToolAllowed(ch.ID, "another_tool") {
		t.Error("wildcard should allow any tool")
	}
}

func TestMCPChannelManager_IsToolAllowed_MissingChannel(t *testing.T) {
	m := NewMCPChannelManager()

	if m.IsToolAllowed("missing", "tool") {
		t.Error("missing channel should not allow any tool")
	}
}

func TestNormalizeServerName(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"My Server", "my-server"},
		{"test_server", "test-server"},
		{"TEST", "test"},
		{"already-lower", "already-lower"},
	}

	for _, tt := range tests {
		got := normalizeServerName(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeServerName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestBuildMCPToolName(t *testing.T) {
	name := buildMCPToolName("My Server", "read")
	if name != "mcp__my-server__read" {
		t.Errorf("expected mcp__my-server__read, got %s", name)
	}
}

func TestParseMCPToolName(t *testing.T) {
	server, tool, ok := parseMCPToolName("mcp__my-server__read")
	if !ok {
		t.Fatal("should parse valid tool name")
	}
	if server != "my-server" {
		t.Errorf("expected server=my-server, got %s", server)
	}
	if tool != "read" {
		t.Errorf("expected tool=read, got %s", tool)
	}
}

func TestParseMCPToolName_Invalid(t *testing.T) {
	_, _, ok := parseMCPToolName("invalid")
	if ok {
		t.Error("should not parse invalid tool name")
	}

	_, _, ok = parseMCPToolName("mcp__onlyone")
	if ok {
		t.Error("should not parse tool name with wrong number of parts")
	}

	_, _, ok = parseMCPToolName("other__my-server__read")
	if ok {
		t.Error("should not parse tool name without 'mcp' prefix")
	}
}

func TestExpandEnv(t *testing.T) {
	os.Setenv("TEST_MCP_VAR", "hello")
	defer os.Unsetenv("TEST_MCP_VAR")

	result := expandEnv("$TEST_MCP_VAR")
	if result != "hello" {
		t.Errorf("expected hello, got %s", result)
	}
}

func TestExpandEnvMap(t *testing.T) {
	os.Setenv("TEST_MCP_KEY", "value1")
	defer os.Unsetenv("TEST_MCP_KEY")

	input := map[string]string{
		"key1": "$TEST_MCP_KEY",
		"key2": "plain",
	}

	result := expandEnvMap(input)
	if result["key1"] != "value1" {
		t.Errorf("expected value1, got %s", result["key1"])
	}
	if result["key2"] != "plain" {
		t.Errorf("expected plain, got %s", result["key2"])
	}
}
