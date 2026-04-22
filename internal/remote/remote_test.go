package remote

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRemoteEnvironment_SetGet(t *testing.T) {
	env := &RemoteEnvironment{Variables: make(map[string]string)}
	env.Set("KEY", "value")
	if env.Get("KEY") != "value" {
		t.Errorf("expected value, got %q", env.Get("KEY"))
	}
}

func TestRemoteEnvironment_Delete(t *testing.T) {
	env := &RemoteEnvironment{Variables: make(map[string]string)}
	env.Set("KEY", "value")
	env.Delete("KEY")
	if env.Get("KEY") != "" {
		t.Errorf("expected empty after delete, got %q", env.Get("KEY"))
	}
}

func TestRemoteEnvironment_DeleteMissing(t *testing.T) {
	env := &RemoteEnvironment{Variables: make(map[string]string)}
	env.Delete("NONEXISTENT")
}

func TestRemoteEnvironment_All(t *testing.T) {
	env := &RemoteEnvironment{Variables: make(map[string]string)}
	env.Set("A", "1")
	env.Set("B", "2")
	all := env.All()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
	if all["A"] != "1" || all["B"] != "2" {
		t.Errorf("unexpected values: %v", all)
	}
}

func TestRemoteEnvironment_AllReturnsCopy(t *testing.T) {
	env := &RemoteEnvironment{Variables: make(map[string]string)}
	env.Set("KEY", "original")
	copy := env.All()
	copy["KEY"] = "modified"
	if env.Get("KEY") != "original" {
		t.Error("All() should return a copy, not a reference")
	}
}

func TestRemoteEnvironment_ConcurrentAccess(t *testing.T) {
	env := &RemoteEnvironment{Variables: make(map[string]string)}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			env.Set("key", "value")
		}(i)
		go func(i int) {
			defer wg.Done()
			env.Get("key")
		}(i)
		go func(i int) {
			defer wg.Done()
			env.All()
		}(i)
	}
	wg.Wait()
}

func TestRemoteSettings_EnableDisable(t *testing.T) {
	rs := &RemoteSettings{}
	if rs.IsEnabled() {
		t.Error("expected disabled by default")
	}
	rs.Enable()
	if !rs.IsEnabled() {
		t.Error("expected enabled after Enable()")
	}
	rs.Disable()
	if rs.IsEnabled() {
		t.Error("expected disabled after Disable()")
	}
}

func TestRemoteSettings_Select(t *testing.T) {
	rs := &RemoteSettings{}
	if rs.GetSelected() != "" {
		t.Errorf("expected empty selected, got %q", rs.GetSelected())
	}
	rs.Select("remote1")
	if rs.GetSelected() != "remote1" {
		t.Errorf("expected remote1, got %q", rs.GetSelected())
	}
}

func TestRemoteSettings_ConcurrentAccess(t *testing.T) {
	rs := &RemoteSettings{}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			rs.Enable()
		}()
		go func() {
			defer wg.Done()
			rs.IsEnabled()
		}()
		go func() {
			defer wg.Done()
			rs.Select("remote")
		}()
	}
	wg.Wait()
}

func TestRemoteManager_Add(t *testing.T) {
	rm := NewRemoteManager()
	config := &RemoteConfig{Host: "localhost", Port: 22}
	err := rm.Add("test", config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoteManager_List(t *testing.T) {
	rm := NewRemoteManager()
	rm.Add("remote1", &RemoteConfig{Host: "host1"})
	rm.Add("remote2", &RemoteConfig{Host: "host2"})
	names := rm.List()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestRemoteManager_Get_NotFound(t *testing.T) {
	rm := NewRemoteManager()
	_, err := rm.Get("nonexistent")
	if err == nil {
		t.Error("expected error for unknown name")
	}
}

func TestRemoteManager_Disconnect_NotFound(t *testing.T) {
	rm := NewRemoteManager()
	err := rm.Disconnect("nonexistent")
	if err != nil {
		t.Errorf("expected nil for disconnect on unknown name, got %v", err)
	}
}

func TestRemoteConnection_IsConnected_Initially(t *testing.T) {
	rc := NewRemoteConnection(&RemoteConfig{Host: "localhost", Port: 22})
	if rc.IsConnected() {
		t.Error("expected not connected initially")
	}
}

func TestRemoteConnection_Send_NotConnected(t *testing.T) {
	rc := NewRemoteConnection(&RemoteConfig{Host: "localhost", Port: 22})
	err := rc.Send([]byte("data"))
	if err == nil {
		t.Error("expected error when sending on disconnected connection")
	}
}

func TestRemoteConnection_Receive_NotConnected(t *testing.T) {
	rc := NewRemoteConnection(&RemoteConfig{Host: "localhost", Port: 22})
	_, err := rc.Receive()
	if err == nil {
		t.Error("expected error when receiving on disconnected connection")
	}
}

func TestRemoteConnection_Connect_InvalidHost(t *testing.T) {
	rc := NewRemoteConnection(&RemoteConfig{
		Host:    "256.256.256.256",
		Port:    99999,
		Timeout: time.Second,
	})
	err := rc.Connect()
	if err == nil {
		t.Error("expected error connecting to invalid host")
	}
}

func TestRemoteConnection_NetPipe(t *testing.T) {
	server, client := net.Pipe()

	rc := NewRemoteConnection(&RemoteConfig{Host: "pipe", Port: 0})
	rc.mu.Lock()
	rc.Conn = client
	rc.mu.Unlock()

	if !rc.IsConnected() {
		t.Error("expected connected after setting Conn")
	}

	go func() {
		server.Write([]byte("hello from server"))
		server.Close()
	}()

	data, err := rc.Receive()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello from server" {
		t.Errorf("expected 'hello from server', got %q", string(data))
	}

	client.Close()
}

func TestRemoteConnection_Disconnect(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()

	rc := NewRemoteConnection(&RemoteConfig{Host: "pipe", Port: 0})
	rc.mu.Lock()
	rc.Conn = client
	rc.mu.Unlock()

	err := rc.Disconnect()
	if err != nil {
		t.Errorf("unexpected error on disconnect: %v", err)
	}
}

func TestRemoteConnection_Disconnect_NotConnected(t *testing.T) {
	rc := NewRemoteConnection(&RemoteConfig{Host: "pipe", Port: 0})
	err := rc.Disconnect()
	if err != nil {
		t.Errorf("expected nil for disconnect when not connected, got %v", err)
	}
}

func TestLoadRemoteConfig_FileNotExist(t *testing.T) {
	_, err := LoadRemoteConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestSaveAndLoadRemoteConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	config := &RemoteConfig{
		Host:            "example.com",
		Port:            22,
		User:            "testuser",
		PrivateKey:      "key-data",
		RemoteAgentPort: 8080,
		Timeout:         30 * time.Second,
		Env:             map[string]string{"FOO": "bar"},
	}

	err := SaveRemoteConfig(path, config)
	if err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	loaded, err := LoadRemoteConfig(path)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}

	if loaded.Host != "example.com" {
		t.Errorf("expected host example.com, got %q", loaded.Host)
	}
	if loaded.Port != 22 {
		t.Errorf("expected port 22, got %d", loaded.Port)
	}
	if loaded.User != "testuser" {
		t.Errorf("expected user testuser, got %q", loaded.User)
	}
	if loaded.PrivateKey != "key-data" {
		t.Errorf("expected private key key-data, got %q", loaded.PrivateKey)
	}
	if loaded.RemoteAgentPort != 8080 {
		t.Errorf("expected remote agent port 8080, got %d", loaded.RemoteAgentPort)
	}
	if loaded.Env["FOO"] != "bar" {
		t.Errorf("expected env FOO=bar, got %q", loaded.Env["FOO"])
	}
}

func TestSaveRemoteConfig_InvalidPath(t *testing.T) {
	config := &RemoteConfig{Host: "test"}
	err := SaveRemoteConfig("/nonexistent/dir/config.json", config)
	if err == nil {
		t.Error("expected error saving to invalid path")
	}

	_ = config
}

func TestLoadRemoteConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not-json"), 0644)

	_, err := LoadRemoteConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSaveRemoteConfig_JsonRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.json")

	original := &RemoteConfig{
		Host:       "host.example.com",
		Port:       2222,
		User:       "deploy",
		PrivateKey: "-----BEGIN KEY-----\nabc\n-----END KEY-----",
		Env:        map[string]string{"A": "1", "B": "2"},
	}

	SaveRemoteConfig(path, original)

	data, _ := os.ReadFile(path)
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if raw["host"] != "host.example.com" {
		t.Errorf("expected host in JSON, got %v", raw["host"])
	}
	if raw["port"].(float64) != 2222 {
		t.Errorf("expected port 2222 in JSON, got %v", raw["port"])
	}
}

func TestGetRemoteManager_Singleton(t *testing.T) {
	globalRemoteManager = nil
	m1 := GetRemoteManager()
	m2 := GetRemoteManager()
	if m1 != m2 {
		t.Error("expected same instance from GetRemoteManager")
	}
	globalRemoteManager = nil
}

func TestInitRemoteManager_Resets(t *testing.T) {
	old := globalRemoteManager
	InitRemoteManager()
	if globalRemoteManager == old && old != nil {
		t.Error("expected InitRemoteManager to create new instance")
	}
	globalRemoteManager = nil
}

func TestGetRemoteManager_CreatesOnNil(t *testing.T) {
	globalRemoteManager = nil
	m := GetRemoteManager()
	if m == nil {
		t.Error("expected non-nil manager")
	}
	if globalRemoteManager == nil {
		t.Error("expected global to be set")
	}
	globalRemoteManager = nil
}

func TestRemoteManager_Get_ConnectionError(t *testing.T) {
	rm := NewRemoteManager()
	rm.Add("bad", &RemoteConfig{
		Host:    "256.256.256.256",
		Port:    99999,
		Timeout: time.Second,
	})
	_, err := rm.Get("bad")
	if err == nil {
		t.Error("expected error connecting to invalid host")
	}
}

func TestRemoteManager_Disconnect_ExistingConnection(t *testing.T) {
	rm := NewRemoteManager()
	server, client := net.Pipe()
	defer server.Close()

	config := &RemoteConfig{Host: "pipe", Port: 0}
	rm.Add("pipe-remote", config)

	conn := NewRemoteConnection(config)
	conn.mu.Lock()
	conn.Conn = client
	conn.mu.Unlock()

	rm.mu.Lock()
	rm.connections["pipe-remote"] = conn
	rm.mu.Unlock()

	err := rm.Disconnect("pipe-remote")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	rm.mu.RLock()
	_, exists := rm.connections["pipe-remote"]
	rm.mu.RUnlock()
	if exists {
		t.Error("expected connection to be removed from map")
	}
}
