package permissions

import (
	"sort"
	"sync"
)

// AgentPermissionMode extends the base PermissionMode with agent-specific modes.
// While PermissionMode controls the overall permission engine behavior,
// AgentPermissionMode defines the access profile for a specific agent type.
type AgentPermissionMode string

const (
	// AgentModeFullAccess allows all tools without restriction.
	AgentModeFullAccess AgentPermissionMode = "full-access"
	// AgentModeReadOnly denies all write/edit/bash tools — agent can only observe.
	AgentModeReadOnly AgentPermissionMode = "read-only"
	// AgentModeRestricted only allows tools that are explicitly listed in AllowedTools.
	AgentModeRestricted AgentPermissionMode = "restricted"
	// AgentModeAsk allows tools by default but asks for confirmation on dangerous operations.
	AgentModeAsk AgentPermissionMode = "ask"
)

// readOnlyDeniedTools is the set of tools always denied in read-only mode.
var readOnlyDeniedTools = map[string]bool{
	"write_file":   true,
	"edit_file":    true,
	"bash":         true,
	"docker_exec":  true,
	"execute_code": true,
	"repl":         true,
}

// AgentPermissionSet defines tool access rules for a specific agent.
// An agent's effective permissions are determined by the interaction of
// AllowedTools, DisallowedTools, and Mode.
type AgentPermissionSet struct {
	// AgentType is the unique identifier for the agent (e.g. "build", "plan", "ops").
	AgentType string
	// AllowedTools is an allowlist of tool names. If nil or empty, all tools are
	// allowed unless disallowed. Only consulted when non-empty.
	AllowedTools map[string]bool
	// DisallowedTools is a denylist that takes precedence over AllowedTools.
	// A tool listed here is always denied regardless of other settings.
	DisallowedTools map[string]bool
	// Mode determines the default access profile for the agent.
	Mode AgentPermissionMode
	// BashRequireConfirm means bash commands require user confirmation before execution.
	BashRequireConfirm bool
}

// NewAgentPermissionSet creates a permission set from agent definition fields.
// allowedTools and disallowedTools are slices of tool name strings converted
// to lookup maps.
func NewAgentPermissionSet(agentType string, allowedTools []string, disallowedTools []string, mode AgentPermissionMode) *AgentPermissionSet {
	aps := &AgentPermissionSet{
		AgentType:       agentType,
		AllowedTools:    make(map[string]bool, len(allowedTools)),
		DisallowedTools: make(map[string]bool, len(disallowedTools)),
		Mode:            mode,
	}

	for _, t := range allowedTools {
		aps.AllowedTools[t] = true
	}
	for _, t := range disallowedTools {
		aps.DisallowedTools[t] = true
	}

	return aps
}

// IsToolAllowed checks if a tool is permitted for this agent.
// The check proceeds in the following order:
//  1. If mode is full-access AND DisallowedTools is empty, return true.
//  2. If tool is in DisallowedTools, return false.
//  3. If AllowedTools is non-empty AND tool is NOT in AllowedTools, return false.
//  4. If mode is read-only, deny tools in the readOnlyDeniedTools set.
//  5. Otherwise return true.
func (aps *AgentPermissionSet) IsToolAllowed(toolName string) bool {
	// 1. Full-access with no denylist: allow everything.
	if aps.Mode == AgentModeFullAccess && len(aps.DisallowedTools) == 0 {
		return true
	}

	// 2. Explicit deny takes precedence.
	if aps.DisallowedTools[toolName] {
		return false
	}

	// 3. If there is an allowlist, tool must be on it.
	if len(aps.AllowedTools) > 0 && !aps.AllowedTools[toolName] {
		return false
	}

	// 4. Read-only mode denies write/edit/bash/docker/execute tools by default.
	if aps.Mode == AgentModeReadOnly {
		if readOnlyDeniedTools[toolName] {
			return false
		}
	}

	return true
}

// IsToolRestricted checks if a tool is explicitly disallowed for this agent.
// This only checks the DisallowedTools denylist, not mode-based restrictions.
func (aps *AgentPermissionSet) IsToolRestricted(toolName string) bool {
	return aps.DisallowedTools[toolName]
}

// ShouldConfirmBash returns true if bash commands need user confirmation
// before execution for this agent.
func (aps *AgentPermissionSet) ShouldConfirmBash() bool {
	return aps.BashRequireConfirm
}

// AgentPermissionManager manages per-agent permission sets.
// It provides a central registry for looking up which tools each agent type
// is allowed to use, and supports thread-safe registration and lookup.
type AgentPermissionManager struct {
	sets map[string]*AgentPermissionSet
	mu   sync.RWMutex
}

// NewAgentPermissionManager creates a new manager with built-in agent defaults.
// The three core agents are pre-registered:
//   - "build": full-access, no restrictions
//   - "plan":  read-only mode, disallowed: write_file, edit_file, bash, docker_exec, execute_code, repl
//   - "ops":   restricted mode, allowed: bash, read_file, write_file, edit_file, glob, grep, docker_exec;
//     bash requires confirmation
func NewAgentPermissionManager() *AgentPermissionManager {
	m := &AgentPermissionManager{
		sets: make(map[string]*AgentPermissionSet),
	}

	// build: full access, no restrictions
	m.Register(&AgentPermissionSet{
		AgentType:        "build",
		AllowedTools:     nil,
		DisallowedTools:  nil,
		Mode:             AgentModeFullAccess,
		BashRequireConfirm: false,
	})

	// plan: read-only, cannot write or execute
	m.Register(&AgentPermissionSet{
		AgentType: "plan",
		AllowedTools:     nil,
		DisallowedTools: map[string]bool{
			"write_file":   true,
			"edit_file":    true,
			"bash":         true,
			"docker_exec":  true,
			"execute_code": true,
			"repl":         true,
		},
		Mode:             AgentModeReadOnly,
		BashRequireConfirm: false,
	})

	// ops: restricted to operational tools, bash requires confirmation
	m.Register(&AgentPermissionSet{
		AgentType: "ops",
		AllowedTools: map[string]bool{
			"bash":        true,
			"read_file":   true,
			"write_file":  true,
			"edit_file":   true,
			"glob":        true,
			"grep":        true,
			"docker_exec": true,
		},
		DisallowedTools:    nil,
		Mode:               AgentModeRestricted,
		BashRequireConfirm: true,
	})

	return m
}

// Register adds or replaces the permission set for an agent type.
// If a set with the same AgentType already exists it is overwritten.
func (m *AgentPermissionManager) Register(set *AgentPermissionSet) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sets[set.AgentType] = set
}

// Get retrieves the permission set for an agent type.
// Returns nil if no set is registered for the given agentType.
func (m *AgentPermissionManager) Get(agentType string) *AgentPermissionSet {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sets[agentType]
}

// IsToolAllowed checks whether a tool is permitted for a specific agent type.
// If no permission set is registered for the agentType, the tool is allowed
// by default (fail-open).
func (m *AgentPermissionManager) IsToolAllowed(agentType string, toolName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	set, ok := m.sets[agentType]
	if !ok {
		// Unknown agent: allow by default.
		return true
	}
	return set.IsToolAllowed(toolName)
}

// ShouldConfirmBash checks if bash commands need user confirmation for a
// specific agent type. Returns false for unknown agent types.
func (m *AgentPermissionManager) ShouldConfirmBash(agentType string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	set, ok := m.sets[agentType]
	if !ok {
		return false
	}
	return set.ShouldConfirmBash()
}

// FilterTools returns the list of tools that are allowed for an agent from a
// full tool list. The returned slice is sorted lexicographically for
// deterministic output.
func (m *AgentPermissionManager) FilterTools(agentType string, allTools []string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	set, ok := m.sets[agentType]
	if !ok {
		// Unknown agent: return all tools.
		return allTools
	}

	allowed := make([]string, 0, len(allTools))
	for _, t := range allTools {
		if set.IsToolAllowed(t) {
			allowed = append(allowed, t)
		}
	}

	sort.Strings(allowed)
	return allowed
}
