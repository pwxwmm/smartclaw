// Package agents defines built-in agent profiles and a registry for
// looking them up at runtime. Each profile describes the tools,
// permissions, prompts, and behavioral constraints for a specific
// class of work (coding, planning, ops).
package agents

import (
	"fmt"

	"github.com/instructkr/smartclaw/internal/tui"
)

// AgentProfile describes the capabilities and constraints of a single
// agent type. The field set mirrors tui.AgentDefinition so that a
// profile can be converted directly into the TUI's internal
// representation via ToAgentDefinition.
type AgentProfile struct {
	// AgentType is the unique identifier for this profile (e.g. "build", "plan", "ops").
	AgentType string
	// WhenToUse is a short human-readable description of when this agent should be selected.
	WhenToUse string
	// SystemPrompt is the full system prompt injected at the start of every conversation.
	SystemPrompt string
	// Tools is an allow-list of tool names. nil means all tools are permitted.
	Tools []string
	// DisallowedTools is a deny-list of tool names that must never be available.
	DisallowedTools []string
	// Model overrides the default LLM model. Empty string means use the global default.
	Model string
	// PermissionMode controls the sandbox/permission level (ask, read-only, workspace-write, danger-full-access).
	PermissionMode tui.PermissionMode
	// Color is the TUI accent colour for this agent (e.g. "blue", "yellow").
	Color string
	// MaxTurns is the maximum number of agent turns per task. 0 means unlimited.
	MaxTurns int
	// Memory defines the scope of the agent's memory (user, project, local).
	Memory tui.AgentMemoryScope
}

// BuildProfile returns the general-purpose coding agent profile.
// It has access to all tools and is designed for feature
// implementation, bug fixes, and refactoring.
func BuildProfile() *AgentProfile {
	return &AgentProfile{
		AgentType: "build",
		WhenToUse: "General coding agent — implements features, fixes bugs, refactors code",
		SystemPrompt: "You are a coding agent focused on implementation. Read files, write code, edit existing " +
			"code, run shell commands, and use LSP tools to navigate and understand the codebase. Produce clean, " +
			"idiomatic code that follows the project's existing patterns and conventions. Verify your changes by " +
			"running relevant tests or build commands before considering a task complete. When a task is ambiguous, " +
			"prefer the simplest correct solution over over-engineering.",
		Tools:           nil,
		DisallowedTools: nil,
		Model:           "",
		PermissionMode:  tui.PermissionModeAsk,
		Color:           "blue",
		MaxTurns:        0,
		Memory:          tui.AgentMemoryProject,
	}
}

// PlanProfile returns the read-only planning/architecture agent
// profile. It can read and analyse the codebase but must never
// modify files or execute arbitrary commands.
func PlanProfile() *AgentProfile {
	return &AgentProfile{
		AgentType: "plan",
		WhenToUse: "Plan agent — analyzes requirements, designs architecture, creates implementation plans (read-only)",
		SystemPrompt: "You are a planning agent focused on analysis and design. Read files, search the codebase, " +
			"and use LSP tools to understand structure and dependencies. You must never write or edit code — your " +
			"output is structured plans, architecture decisions, and implementation roadmaps. Break complex tasks " +
			"into ordered steps, identify risks and dependencies, and specify acceptance criteria for each step. " +
			"Prefer concrete, actionable recommendations over vague suggestions.",
		Tools: []string{
			"read", "glob", "grep",
			"lsp_symbols", "lsp_find_references", "lsp_goto_definition",
			"ast_grep_search",
			"web_fetch", "web_search",
		},
		DisallowedTools: []string{"write", "edit", "bash", "docker_exec", "execute_code"},
		Model:           "",
		PermissionMode:  tui.PermissionModeReadOnly,
		Color:           "yellow",
		MaxTurns:        15,
		Memory:          tui.AgentMemoryProject,
	}
}

// OpsProfile returns the DevOps/infrastructure agent profile.
// It is optimised for CI/CD, deployment, containerisation, and
// infrastructure-as-code workflows.
func OpsProfile() *AgentProfile {
	return &AgentProfile{
		AgentType: "ops",
		WhenToUse: "Ops agent — CI/CD, deployment, infrastructure, containerization",
		SystemPrompt: "You are a DevOps agent focused on infrastructure, CI/CD, deployment, and containerization. " +
			"Run shell commands, read and write configuration files, and manage Docker containers. Prioritise " +
			"production-readiness, security best practices, and idempotent operations that can be safely re-run. " +
			"Always validate changes before applying them and prefer declarative configuration over imperative scripts.",
		Tools: []string{
			"bash", "read", "write", "edit", "glob", "grep", "docker_exec",
		},
		DisallowedTools: []string{"execute_code"},
		Model:           "",
		PermissionMode:  tui.PermissionModeAsk,
		Color:           "orange",
		MaxTurns:        0,
		Memory:          tui.AgentMemoryProject,
	}
}

// ProfileRegistry holds the set of available agent profiles and
// provides lookup and conversion helpers.
type ProfileRegistry struct {
	profiles map[string]*AgentProfile
}

// NewProfileRegistry creates a registry pre-loaded with the three
// built-in profiles: build, plan, and ops.
func NewProfileRegistry() *ProfileRegistry {
	r := &ProfileRegistry{
		profiles: make(map[string]*AgentProfile, 3),
	}
	r.register(BuildProfile())
	r.register(PlanProfile())
	r.register(OpsProfile())
	return r
}

func (r *ProfileRegistry) register(p *AgentProfile) {
	r.profiles[p.AgentType] = p
}

// Get returns the profile identified by name. Returns an error if no
// profile with that name exists.
func (r *ProfileRegistry) Get(name string) (*AgentProfile, error) {
	p, ok := r.profiles[name]
	if !ok {
		return nil, fmt.Errorf("agent profile %q not found", name)
	}
	return p, nil
}

// List returns all registered profiles in an unordered slice.
func (r *ProfileRegistry) List() []*AgentProfile {
	out := make([]*AgentProfile, 0, len(r.profiles))
	for _, p := range r.profiles {
		out = append(out, p)
	}
	return out
}

// ToAgentDefinition converts an AgentProfile into the TUI-layer
// AgentDefinition type, preserving all fields that overlap between
// the two structs.
func ToAgentDefinition(p *AgentProfile) *tui.AgentDefinition {
	return &tui.AgentDefinition{
		AgentType:       p.AgentType,
		WhenToUse:       p.WhenToUse,
		Tools:           p.Tools,
		DisallowedTools: p.DisallowedTools,
		Model:           p.Model,
		PermissionMode:  p.PermissionMode,
		Color:           p.Color,
		MaxTurns:        p.MaxTurns,
		Memory:          p.Memory,
		SystemPrompt:    p.SystemPrompt,
		Source:          tui.AgentSourceBuiltIn,
	}
}
