package remote

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type RemoteConfig struct {
	Host            string            `json:"host"`
	Port            int               `json:"port"`
	User            string            `json:"user"`
	PrivateKey      string            `json:"private_key"`
	RemoteAgentPort int               `json:"remote_agent_port"`
	Timeout         time.Duration     `json:"timeout"`
	Env             map[string]string `json:"env"`
}

type RemoteConnection struct {
	Config *RemoteConfig
	Conn   net.Conn
	mu     sync.RWMutex
}

func NewRemoteConnection(config *RemoteConfig) *RemoteConnection {
	return &RemoteConnection{
		Config: config,
	}
}

func (rc *RemoteConnection) Connect() error {
	addr := fmt.Sprintf("%s:%d", rc.Config.Host, rc.Config.Port)

	conn, err := net.DialTimeout("tcp", addr, rc.Config.Timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to remote: %w", err)
	}

	rc.mu.Lock()
	rc.Conn = conn
	rc.mu.Unlock()

	return nil
}

func (rc *RemoteConnection) Disconnect() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.Conn != nil {
		return rc.Conn.Close()
	}
	return nil
}

func (rc *RemoteConnection) Send(data []byte) error {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if rc.Conn == nil {
		return fmt.Errorf("not connected")
	}

	_, err := rc.Conn.Write(data)
	return err
}

func (rc *RemoteConnection) Receive() ([]byte, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if rc.Conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	buf := make([]byte, 4096)
	n, err := rc.Conn.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

func (rc *RemoteConnection) IsConnected() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.Conn != nil
}

type RemoteManager struct {
	connections map[string]*RemoteConnection
	configs     map[string]*RemoteConfig
	mu          sync.RWMutex
}

func NewRemoteManager() *RemoteManager {
	return &RemoteManager{
		connections: make(map[string]*RemoteConnection),
		configs:     make(map[string]*RemoteConfig),
	}
}

func (rm *RemoteManager) Add(name string, config *RemoteConfig) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.configs[name] = config
	return nil
}

func (rm *RemoteManager) Get(name string) (*RemoteConnection, error) {
	rm.mu.RLock()
	config, ok := rm.configs[name]
	rm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("remote not found: %s", name)
	}

	conn := NewRemoteConnection(config)
	if err := conn.Connect(); err != nil {
		return nil, err
	}

	rm.mu.Lock()
	rm.connections[name] = conn
	rm.mu.Unlock()

	return conn, nil
}

func (rm *RemoteManager) Disconnect(name string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	conn, ok := rm.connections[name]
	if !ok {
		return nil
	}

	err := conn.Disconnect()
	delete(rm.connections, name)
	return err
}

func (rm *RemoteManager) List() []string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	names := make([]string, 0, len(rm.configs))
	for name := range rm.configs {
		names = append(names, name)
	}
	return names
}

type RemoteExec struct {
	Command string
	Args    []string
	Env     map[string]string
	Dir     string
}

type RemoteExecResult struct {
	Output   string
	ExitCode int
	Error    string
}

func (rm *RemoteManager) Execute(name string, exec *RemoteExec) (*RemoteExecResult, error) {
	conn, err := rm.Get(name)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(exec)
	if err != nil {
		return nil, err
	}

	if err := conn.Send(data); err != nil {
		return nil, err
	}

	response, err := conn.Receive()
	if err != nil {
		return nil, err
	}

	var result RemoteExecResult
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

type RemoteFileTransfer struct {
	Source      string
	Destination string
	IsUpload    bool
}

func (rm *RemoteManager) TransferFile(name string, transfer *RemoteFileTransfer) error {
	conn, err := rm.Get(name)
	if err != nil {
		return err
	}

	data, err := json.Marshal(transfer)
	if err != nil {
		return err
	}

	return conn.Send(data)
}

type RemoteEnvironment struct {
	Variables map[string]string
	mu        sync.RWMutex
}

func (re *RemoteEnvironment) Set(key, value string) {
	re.mu.Lock()
	defer re.mu.Unlock()
	re.Variables[key] = value
}

func (re *RemoteEnvironment) Get(key string) string {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.Variables[key]
}

func (re *RemoteEnvironment) Delete(key string) {
	re.mu.Lock()
	defer re.mu.Unlock()
	delete(re.Variables, key)
}

func (re *RemoteEnvironment) All() map[string]string {
	re.mu.RLock()
	defer re.mu.RUnlock()

	copyVars := make(map[string]string)
	for k, v := range re.Variables {
		copyVars[k] = v
	}
	return copyVars
}

func (re *RemoteEnvironment) FromRemote(name string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.Variables = make(map[string]string)
	return nil
}

var globalRemoteManager *RemoteManager

func InitRemoteManager() {
	globalRemoteManager = NewRemoteManager()
}

func GetRemoteManager() *RemoteManager {
	if globalRemoteManager == nil {
		globalRemoteManager = NewRemoteManager()
	}
	return globalRemoteManager
}

type RemoteSettings struct {
	RemoteEnabled bool
	Selected      string
	mu            sync.RWMutex
}

func (rs *RemoteSettings) Enable() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.RemoteEnabled = true
}

func (rs *RemoteSettings) Disable() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.RemoteEnabled = false
}

func (rs *RemoteSettings) IsEnabled() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.RemoteEnabled
}

func (rs *RemoteSettings) Select(name string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.Selected = name
}

func (rs *RemoteSettings) GetSelected() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.Selected
}

func LoadRemoteConfig(path string) (*RemoteConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config RemoteConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func SaveRemoteConfig(path string, config *RemoteConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
