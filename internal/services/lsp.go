package services

import (
	"context"
	"fmt"
	"sync"
)

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

type LSPServerManager struct {
	servers map[string]*LSPServerInstance
	mu      sync.RWMutex
}

func NewLSPServerManager() *LSPServerManager {
	return &LSPServerManager{
		servers: make(map[string]*LSPServerInstance),
	}
}

type LSPServerInstance struct {
	ID       string
	Language string
	Command  string
	Args     []string
	Status   string
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

func (m *LSPServerManager) Stop(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	server, exists := m.servers[id]
	if !exists {
		return fmt.Errorf("server not found: %s", id)
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
		server.Status = "stopped"
		delete(m.servers, id)
	}

	return nil
}

type LSPConfig struct {
	Servers map[string]ServerConfig
}

type ServerConfig struct {
	Command string
	Args    []string
	Env     map[string]string
}
