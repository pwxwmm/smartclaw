package acp

import (
	"fmt"
	"strings"
	"sync"

	"github.com/instructkr/smartclaw/internal/mcp"
)

type PermissionLevel string

const (
	PermissionRead    PermissionLevel = "read"
	PermissionWrite   PermissionLevel = "write"
	PermissionExecute PermissionLevel = "execute"
	PermissionAdmin   PermissionLevel = "admin"
)

// PermissionRule governs access to a resource pattern.
// Patterns use colon-separated segments; "*" matches any segment:
// "tools:*", "memory:*", "files:read", "sessions:own".
type PermissionRule struct {
	Resource string         `json:"resource"`
	Level    PermissionLevel `json:"level"`
	Allowed  bool           `json:"allowed"`
}

type PermissionModel struct {
	rules     []PermissionRule
	rolePerms map[string][]PermissionRule
	mu        sync.RWMutex
}

func NewPermissionModel() *PermissionModel {
	return &PermissionModel{
		rules:     make([]PermissionRule, 0),
		rolePerms: make(map[string][]PermissionRule),
	}
}

func (pm *PermissionModel) AddRule(rule PermissionRule) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.rules = append(pm.rules, rule)
}

func (pm *PermissionModel) RemoveRule(resource string, level PermissionLevel) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	filtered := pm.rules[:0]
	for _, r := range pm.rules {
		if !(r.Resource == resource && r.Level == level) {
			filtered = append(filtered, r)
		}
	}
	pm.rules = filtered
}

// Check walks rules in order; first matching rule decides. Wildcard "*" matches any segment.
func (pm *PermissionModel) Check(resource string, level PermissionLevel) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, r := range pm.rules {
		if r.Level == level && resourceMatch(r.Resource, resource) {
			return r.Allowed
		}
	}
	return false
}

func (pm *PermissionModel) DefineRole(name string, rules []PermissionRule) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.rolePerms[name] = rules
}

func (pm *PermissionModel) CheckRole(role, resource string, level PermissionLevel) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if roleRules, ok := pm.rolePerms[role]; ok {
		for _, r := range roleRules {
			if r.Level == level && resourceMatch(r.Resource, resource) {
				return r.Allowed
			}
		}
	}

	for _, r := range pm.rules {
		if r.Level == level && resourceMatch(r.Resource, resource) {
			return r.Allowed
		}
	}
	return false
}

func DefaultPermissions() []PermissionRule {
	return []PermissionRule{
		{Resource: "*:*", Level: PermissionRead, Allowed: true},
		{Resource: "sessions:own", Level: PermissionWrite, Allowed: true},
		{Resource: "tools:*", Level: PermissionExecute, Allowed: true},
		{Resource: "*:*", Level: PermissionAdmin, Allowed: false},
	}
}

func resourceMatch(pattern, resource string) bool {
	if pattern == resource {
		return true
	}

	patternParts := strings.Split(pattern, ":")
	resourceParts := strings.Split(resource, ":")

	if len(patternParts) != len(resourceParts) {
		return false
	}

	for i, p := range patternParts {
		if p != "*" && p != resourceParts[i] {
			return false
		}
	}
	return true
}

type permissionsCheckParams struct {
	Role     string          `json:"role,omitempty"`
	Resource string          `json:"resource"`
	Level    PermissionLevel `json:"level"`
}

func (s *ACPServer) handlePermissionsCheck(req *mcp.JSONRPCRequest) *mcp.JSONRPCResponse {
	var params permissionsCheckParams
	if err := parseParams(req.Params, &params); err != nil {
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &mcp.RPCError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)},
		}
	}

	var allowed bool
	if params.Role != "" {
		allowed = s.permissions.CheckRole(params.Role, params.Resource, params.Level)
	} else {
		allowed = s.permissions.Check(params.Resource, params.Level)
	}

	return &mcp.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"allowed": allowed},
	}
}
