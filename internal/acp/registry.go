package acp

import (
	"fmt"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/mcp"
)

type ServiceEntry struct {
	Name          string         `json:"name"`
	Version       string         `json:"version"`
	Endpoint      string         `json:"endpoint"`
	Capabilities  []string       `json:"capabilities"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	RegisteredAt  time.Time      `json:"registeredAt"`
	LastHeartbeat time.Time      `json:"lastHeartbeat"`
}

type ServiceRegistry struct {
	services map[string]*ServiceEntry
	mu       sync.RWMutex
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]*ServiceEntry),
	}
}

func (sr *ServiceRegistry) Register(entry ServiceEntry) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if entry.Name == "" {
		return fmt.Errorf("service name is required: %w", errInvalidParams)
	}

	now := time.Now()
	entry.RegisteredAt = now
	entry.LastHeartbeat = now
	sr.services[entry.Name] = &entry
	return nil
}

func (sr *ServiceRegistry) Deregister(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.services, name)
}

func (sr *ServiceRegistry) Get(name string) *ServiceEntry {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.services[name]
}

func (sr *ServiceRegistry) List() []*ServiceEntry {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	result := make([]*ServiceEntry, 0, len(sr.services))
	for _, svc := range sr.services {
		result = append(result, svc)
	}
	return result
}

func (sr *ServiceRegistry) FindByCapability(capability string) []*ServiceEntry {
	sr.mu.RLock()
	defer sr.mu.RUnlock()

	var result []*ServiceEntry
	for _, svc := range sr.services {
		for _, c := range svc.Capabilities {
			if c == capability {
				result = append(result, svc)
				break
			}
		}
	}
	return result
}

func (sr *ServiceRegistry) Heartbeat(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	if svc, ok := sr.services[name]; ok {
		svc.LastHeartbeat = time.Now()
	}
}

func (sr *ServiceRegistry) PruneStale(maxAge time.Duration) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for name, svc := range sr.services {
		if svc.LastHeartbeat.Before(cutoff) {
			delete(sr.services, name)
		}
	}
}

var errInvalidParams = fmt.Errorf("invalid parameters")

type servicesRegisterParams struct {
	Name         string         `json:"name"`
	Version      string         `json:"version"`
	Endpoint     string         `json:"endpoint"`
	Capabilities []string       `json:"capabilities"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type servicesFindParams struct {
	Capability string `json:"capability"`
}

func (s *ACPServer) handleServicesRegister(req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	var params servicesRegisterParams
	if err := parseParams(req.Params, &params); err != nil {
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	entry := ServiceEntry{
		Name:         params.Name,
		Version:      params.Version,
		Endpoint:     params.Endpoint,
		Capabilities: params.Capabilities,
		Metadata:     params.Metadata,
	}

	if err := s.services.Register(entry); err != nil {
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32602, Message: err.Error()},
		}
	}

	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"registered": true},
	}
}

func (s *ACPServer) handleServicesList(req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	services := s.services.List()
	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"services": services},
	}
}

func (s *ACPServer) handleServicesFind(req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	var params servicesFindParams
	if err := parseParams(req.Params, &params); err != nil {
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	found := s.services.FindByCapability(params.Capability)
	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"services": found},
	}
}
