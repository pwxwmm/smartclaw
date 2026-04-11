package services

import (
	"context"
	"fmt"
	"sync"
)

// LSPClient defines the interface for LSP client operations
type LSPClient interface {
	Initialize(ctx context.Context, rootPath string) error
	DidOpen(ctx context.Context, filePath, languageID, content string) error
	DidChange(ctx context.Context, filePath, content string) error
	GotoDefinition(ctx context.Context, filePath string, line, character int) (any, error)
	FindReferences(ctx context.Context, filePath string, line, character int) (any, error)
	DocumentSymbols(ctx context.Context, filePath string) (any, error)
	Hover(ctx context.Context, filePath string, line, character int) (any, error)
	Rename(ctx context.Context, filePath string, line, character int, newName string) (any, error)
	Completion(ctx context.Context, filePath string, line, character int) (any, error)
	Close() error
}

type DiagnosticRegistry struct {
	diagnostics map[string][]Diagnostic
	mu          sync.RWMutex
}

func NewDiagnosticRegistry() *DiagnosticRegistry {
	return &DiagnosticRegistry{
		diagnostics: make(map[string][]Diagnostic),
	}
}

func (r *DiagnosticRegistry) Add(file string, diag Diagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.diagnostics[file] = append(r.diagnostics[file], diag)
}

func (r *DiagnosticRegistry) Get(file string) []Diagnostic {
	r.mu.RLock()
	defer r.mu.RUnlock()

	diags := r.diagnostics[file]
	result := make([]Diagnostic, len(diags))
	copy(result, diags)
	return result
}

func (r *DiagnosticRegistry) GetAll() map[string][]Diagnostic {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]Diagnostic)
	for k, v := range r.diagnostics {
		result[k] = v
	}
	return result
}

func (r *DiagnosticRegistry) Clear(file string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.diagnostics, file)
}

func (r *DiagnosticRegistry) ClearAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.diagnostics = make(map[string][]Diagnostic)
}

func (r *DiagnosticRegistry) HasErrors(file string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, diag := range r.diagnostics[file] {
		if diag.Severity == "error" {
			return true
		}
	}
	return false
}

func (r *DiagnosticRegistry) SetDiagnostics(file string, diags []Diagnostic) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.diagnostics[file] = diags
}

type LSPServerInstance struct {
	ID       string
	Language string
	Command  string
	Args     []string
	Status   string
	Client   LSPClient
}

type LSPServerManager struct {
	servers map[string]*LSPServerInstance
	mu      sync.RWMutex
}

func NewLSPServerManager() *LSPServerManager {
	return &LSPServerManager{
		servers: make(map[string]*LSPServerInstance),
	}
}

func (m *LSPServerManager) Start(ctx context.Context, language, command string, args []string) (*LSPServerInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("lsp_%s_%d", language, len(m.servers))

	server := &LSPServerInstance{
		ID:       id,
		Language: language,
		Command:  command,
		Args:     args,
		Status:   "running",
	}

	m.servers[id] = server
	return server, nil
}

func (m *LSPServerManager) StartWithClient(ctx context.Context, language string, client LSPClient) (*LSPServerInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("lsp_%s_%d", language, len(m.servers))

	server := &LSPServerInstance{
		ID:       id,
		Language: language,
		Status:   "running",
		Client:   client,
	}

	m.servers[id] = server
	return server, nil
}

func (m *LSPServerManager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, exists := m.servers[id]
	if !exists {
		return fmt.Errorf("server not found: %s", id)
	}

	if server.Client != nil {
		server.Client.Close()
	}

	server.Status = "stopped"
	delete(m.servers, id)
	return nil
}

func (m *LSPServerManager) Get(id string) (*LSPServerInstance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	server, exists := m.servers[id]
	return server, exists
}

func (m *LSPServerManager) List() []*LSPServerInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LSPServerInstance, 0, len(m.servers))
	for _, server := range m.servers {
		result = append(result, server)
	}
	return result
}

func (m *LSPServerManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, server := range m.servers {
		if server.Client != nil {
			server.Client.Close()
		}
		server.Status = "stopped"
		delete(m.servers, id)
	}

	return nil
}

func (m *LSPServerManager) GotoDefinition(ctx context.Context, id, filePath string, line, character int) (any, error) {
	m.mu.RLock()
	server, exists := m.servers[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", id)
	}

	if server.Client == nil {
		return nil, fmt.Errorf("server %s has no LSP client attached", id)
	}

	return server.Client.GotoDefinition(ctx, filePath, line, character)
}

func (m *LSPServerManager) FindReferences(ctx context.Context, id, filePath string, line, character int) (any, error) {
	m.mu.RLock()
	server, exists := m.servers[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", id)
	}

	if server.Client == nil {
		return nil, fmt.Errorf("server %s has no LSP client attached", id)
	}

	return server.Client.FindReferences(ctx, filePath, line, character)
}

func (m *LSPServerManager) Hover(ctx context.Context, id, filePath string, line, character int) (any, error) {
	m.mu.RLock()
	server, exists := m.servers[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", id)
	}

	if server.Client == nil {
		return nil, fmt.Errorf("server %s has no LSP client attached", id)
	}

	return server.Client.Hover(ctx, filePath, line, character)
}

func (m *LSPServerManager) DocumentSymbols(ctx context.Context, id, filePath string) (any, error) {
	m.mu.RLock()
	server, exists := m.servers[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", id)
	}

	if server.Client == nil {
		return nil, fmt.Errorf("server %s has no LSP client attached", id)
	}

	return server.Client.DocumentSymbols(ctx, filePath)
}

func (m *LSPServerManager) Completion(ctx context.Context, id, filePath string, line, character int) (any, error) {
	m.mu.RLock()
	server, exists := m.servers[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", id)
	}

	if server.Client == nil {
		return nil, fmt.Errorf("server %s has no LSP client attached", id)
	}

	return server.Client.Completion(ctx, filePath, line, character)
}

func (m *LSPServerManager) Rename(ctx context.Context, id, filePath string, line, character int, newName string) (any, error) {
	m.mu.RLock()
	server, exists := m.servers[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", id)
	}

	if server.Client == nil {
		return nil, fmt.Errorf("server %s has no LSP client attached", id)
	}

	return server.Client.Rename(ctx, filePath, line, character, newName)
}

type LSPConfig struct {
	Servers map[string]ServerConfig
}

type ServerConfig struct {
	Command string
	Args    []string
	Env     map[string]string
}
