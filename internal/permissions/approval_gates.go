package permissions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/instructkr/smartclaw/internal/config"
	"gopkg.in/yaml.v3"
)

// ApprovalPolicy defines whether a tool execution requires human approval.
type ApprovalPolicy struct {
	// Mode: "always", "never", "conditional"
	Mode string `json:"mode" yaml:"mode"`
	// Condition is a simple expression evaluated against tool input.
	// Only used when Mode is "conditional".
	// Supported conditions:
	//   "non-readonly"                - bash non-readonly commands require approval
	//   "path_starts_with:/etc/"      - input.path starts with given prefix
	//   "path_matches:/etc/|/var/"    - input.path matches any pipe-separated prefix
	//   "command_matches:rm|sudo|apt" - input.command matches any pipe-separated prefix
	Condition string `json:"condition,omitempty" yaml:"condition,omitempty"`
	// Reason explains why approval is required (shown to user)
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// ApprovalConfig holds per-tool approval policies.
type ApprovalConfig struct {
	// DefaultPolicy is applied to tools without a specific policy.
	// Values: "ask", "auto-approve", "auto-deny"
	DefaultPolicy string                    `json:"default_policy" yaml:"default_policy"`
	ToolPolicies  map[string]ApprovalPolicy `json:"tool_policies" yaml:"tool_policies"`
	// SREMode enables stricter approval for SRE operations.
	// When true, all SOPA tools require approval except read-only ones.
	SREMode bool `json:"sre_mode" yaml:"sre_mode"`
}

// ApprovalGate evaluates whether a tool execution requires approval.
type ApprovalGate struct {
	mu                   sync.RWMutex
	config               ApprovalConfig
	readOnlyBashPrefixes []string
}

// NewApprovalGate creates an ApprovalGate with sensible defaults matching
// the current hardcoded approval behavior in the web handler.
func NewApprovalGate() *ApprovalGate {
	return &ApprovalGate{
		config:               defaultApprovalConfig(),
		readOnlyBashPrefixes: defaultReadOnlyBashPrefixes(),
	}
}

// defaultReadOnlyBashPrefixes returns the list of bash command prefixes that
// are considered read-only (safe to execute without approval).
func defaultReadOnlyBashPrefixes() []string {
	return []string{
		"ls ", "ls\t", "ls$", "ls;",
		"cat ", "head ", "tail ",
		"git status", "git diff", "git log",
		"echo ", "pwd", "which ", "whoami",
		"find ", "grep ", "wc ", "sort ", "uniq ",
		"file ", "stat ", "test ", "env", "printenv",
		"type ", "command -v",
	}
}

// defaultApprovalConfig returns the default approval configuration that
// matches the existing hardcoded behavior in the web handler.
func defaultApprovalConfig() ApprovalConfig {
	return ApprovalConfig{
		DefaultPolicy: "auto-approve",
		ToolPolicies: map[string]ApprovalPolicy{
			"bash": {
				Mode:      "conditional",
				Condition: "non-readonly",
				Reason:    "Destructive shell command requires approval",
			},
			"write_file": {
				Mode:   "always",
				Reason: "File modification requires approval",
			},
			"edit_file": {
				Mode:   "always",
				Reason: "File edit requires approval",
			},
			"sopa_execute_task": {
				Mode:   "always",
				Reason: "SOPA task execution affects production systems",
			},
			"sopa_execute_orchestration": {
				Mode:   "always",
				Reason: "SOPA orchestration affects production systems",
			},
			"sopa_approve_audit": {
				Mode:   "always",
				Reason: "Audit approval has compliance implications",
			},
			"docker_exec": {
				Mode:   "always",
				Reason: "Docker execution requires approval",
			},
		},
	}
}

// NeedsApproval returns whether the given tool execution requires approval
// and a human-readable reason if it does.
func (g *ApprovalGate) NeedsApproval(toolName string, input map[string]any) (bool, string) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.config.SREMode {
		if needs, reason := g.checkSREMode(toolName, input); needs {
			return true, reason
		}
	}

	policy, hasPolicy := g.config.ToolPolicies[toolName]
	if !hasPolicy {
		return g.applyDefaultPolicy(toolName, input)
	}

	return g.evaluatePolicy(policy, toolName, input)
}

// evaluatePolicy evaluates a single ApprovalPolicy for the given tool and input.
func (g *ApprovalGate) evaluatePolicy(policy ApprovalPolicy, toolName string, input map[string]any) (bool, string) {
	switch policy.Mode {
	case "always":
		return true, policy.Reason
	case "never":
		return false, ""
	case "conditional":
		return g.evaluateCondition(policy.Condition, toolName, input, policy.Reason)
	default:
		return false, ""
	}
}

// evaluateCondition checks a simple condition expression against tool input.
func (g *ApprovalGate) evaluateCondition(condition, toolName string, input map[string]any, reason string) (bool, string) {
	switch {
	case condition == "non-readonly":
		if toolName == "bash" {
			command, _ := input["command"].(string)
			if g.IsReadOnlyBashCommand(command) {
				return false, ""
			}
			return true, reason
		}
		return true, reason

	case strings.HasPrefix(condition, "path_starts_with:"):
		prefix := strings.TrimPrefix(condition, "path_starts_with:")
		pathVal := extractStringInput(input, "path", "file_path", "filepath", "filename")
		if pathVal != "" && strings.HasPrefix(pathVal, prefix) {
			return true, reason
		}
		return false, ""

	case strings.HasPrefix(condition, "path_matches:"):
		patterns := strings.Split(strings.TrimPrefix(condition, "path_matches:"), "|")
		pathVal := extractStringInput(input, "path", "file_path", "filepath", "filename")
		if pathVal != "" {
			for _, p := range patterns {
				if strings.HasPrefix(pathVal, strings.TrimSpace(p)) {
					return true, reason
				}
			}
		}
		return false, ""

	case strings.HasPrefix(condition, "command_matches:"):
		patterns := strings.Split(strings.TrimPrefix(condition, "command_matches:"), "|")
		cmdVal := extractStringInput(input, "command")
		if cmdVal != "" {
			trimmed := strings.TrimSpace(cmdVal)
			for _, p := range patterns {
				if strings.HasPrefix(trimmed, strings.TrimSpace(p)) {
					return true, reason
				}
			}
		}
		return false, ""

	default:
		return true, reason
	}
}

// checkSREMode applies additional approval requirements when SRE mode is enabled.
func (g *ApprovalGate) checkSREMode(toolName string, _ map[string]any) (bool, string) {
	if isSOPAReadOnlyTool(toolName) {
		return false, ""
	}

	if strings.HasPrefix(toolName, "sopa_") {
		return true, fmt.Sprintf("SOPA tool %q requires approval in SRE mode", toolName)
	}

	sreRequireApproval := map[string]string{
		"remote_trigger": "Remote execution requires approval in SRE mode",
		"execute_code":   "Code execution requires approval in SRE mode",
	}
	if reason, ok := sreRequireApproval[toolName]; ok {
		return true, reason
	}

	return false, ""
}

// isSOPAReadOnlyTool returns true for read-only SOPA tools.
func isSOPAReadOnlyTool(toolName string) bool {
	if !strings.HasPrefix(toolName, "sopa_") {
		return false
	}
	return strings.HasPrefix(toolName, "sopa_list_") || strings.HasPrefix(toolName, "sopa_get_")
}

// applyDefaultPolicy handles tools that don't have a specific policy defined.
func (g *ApprovalGate) applyDefaultPolicy(toolName string, _ map[string]any) (bool, string) {
	switch g.config.DefaultPolicy {
	case "ask":
		return true, fmt.Sprintf("Tool %q requires approval (default policy: ask)", toolName)
	case "auto-approve":
		return false, ""
	case "auto-deny":
		return true, fmt.Sprintf("Tool %q is denied (default policy: auto-deny)", toolName)
	default:
		return false, ""
	}
}

// IsReadOnlyBashCommand checks whether a bash command is read-only.
func (g *ApprovalGate) IsReadOnlyBashCommand(command string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	trimmed := strings.TrimSpace(command)
	for _, prefix := range g.readOnlyBashPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
		if trimmed == strings.TrimSpace(prefix) {
			return true
		}
	}
	return false
}

// AddToolPolicy adds or updates a policy for a specific tool programmatically.
func (g *ApprovalGate) AddToolPolicy(toolName string, policy ApprovalPolicy) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.config.ToolPolicies[toolName] = policy
}

// SetSREMode toggles SRE mode which enforces stricter approval.
func (g *ApprovalGate) SetSREMode(enabled bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.config.SREMode = enabled
}

// GetConfig returns a copy of the current approval configuration.
func (g *ApprovalGate) GetConfig() ApprovalConfig {
	g.mu.RLock()
	defer g.mu.RUnlock()

	policies := make(map[string]ApprovalPolicy, len(g.config.ToolPolicies))
	for k, v := range g.config.ToolPolicies {
		policies[k] = v
	}
	return ApprovalConfig{
		DefaultPolicy: g.config.DefaultPolicy,
		ToolPolicies:  policies,
		SREMode:       g.config.SREMode,
	}
}

// LoadConfig loads approval gate configuration from a YAML or JSON file.
func (g *ApprovalGate) LoadConfig(path string) error {
	var cfg ApprovalConfig
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading approval gates config: %w", err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parsing approval gates YAML: %w", err)
		}
	case ".json":
		loaded, err := config.LoadJSON[ApprovalConfig](path)
		if err != nil {
			return fmt.Errorf("parsing approval gates JSON: %w", err)
		}
		cfg = *loaded
	default:
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading approval gates config: %w", err)
		}
		if yamlErr := yaml.Unmarshal(data, &cfg); yamlErr != nil {
			if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
				return fmt.Errorf("parsing approval gates config: %w", errors.Join(yamlErr, jsonErr))
			}
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if cfg.DefaultPolicy != "" {
		g.config.DefaultPolicy = cfg.DefaultPolicy
	}
	if cfg.ToolPolicies != nil {
		for name, policy := range cfg.ToolPolicies {
			g.config.ToolPolicies[name] = policy
		}
	}
	g.config.SREMode = cfg.SREMode

	return nil
}

// LoadConfigFromDefaultPath attempts to load config from
// ~/.smartclaw/approval_gates.yaml. Returns nil if the file doesn't exist.
func (g *ApprovalGate) LoadConfigFromDefaultPath() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	configPath := filepath.Join(homeDir, ".smartclaw", "approval_gates.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil
	}

	return g.LoadConfig(configPath)
}

// extractStringInput retrieves a string value from the input map,
// trying each key in order until one is found.
func extractStringInput(input map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := input[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
