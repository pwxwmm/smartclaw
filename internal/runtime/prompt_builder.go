package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PromptBuilder assembles and caches the system prompt for a session.
// The system prompt is frozen after initial assembly and only rebuilt
// on explicit events (compact, model switch, skill load).
type PromptBuilder struct {
	mu       sync.RWMutex
	frozen   string
	dirty    bool
	homeDir  string
	persona  string
	memory   string
	skills   []string
	contexts []string
}

// NewPromptBuilder creates a builder that reads from ~/.smartclaw/ files.
func NewPromptBuilder() *PromptBuilder {
	homeDir, _ := os.UserHomeDir()
	return &PromptBuilder{
		homeDir: homeDir,
		dirty:   true,
	}
}

// Build assembles the system prompt. If already built and not dirty,
// returns the cached frozen version (preserving prompt cache).
func (pb *PromptBuilder) Build() string {
	pb.mu.RLock()
	if !pb.dirty && pb.frozen != "" {
		frozen := pb.frozen
		pb.mu.RUnlock()
		return frozen
	}
	pb.mu.RUnlock()

	return pb.rebuild()
}

func (pb *PromptBuilder) rebuild() string {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	var parts []string

	parts = append(parts, pb.basePersona())

	if mem := pb.loadMemoryFile(); mem != "" {
		parts = append(parts, "\n\n<user_memory>\n"+mem+"\n</user_memory>")
	}

	if user := pb.loadUserFile(); user != "" {
		parts = append(parts, "\n\n<user_profile>\n"+user+"\n</user_profile>")
	}

	for _, skill := range pb.skills {
		if skill != "" {
			parts = append(parts, "\n\n<skill>\n"+skill+"\n</skill>")
		}
	}

	for _, ctx := range pb.contexts {
		if ctx != "" {
			parts = append(parts, "\n\n<project_context>\n"+ctx+"\n</project_context>")
		}
	}

	pb.frozen = strings.Join(parts, "")
	pb.dirty = false
	return pb.frozen
}

func (pb *PromptBuilder) basePersona() string {
	if pb.persona != "" {
		return pb.persona
	}
	return "You are SmartClaw, an advanced AI coding assistant. Help the user with their coding tasks with precision and clarity."
}

func (pb *PromptBuilder) loadMemoryFile() string {
	if pb.homeDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(pb.homeDir, ".smartclaw", "MEMORY.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (pb *PromptBuilder) loadUserFile() string {
	if pb.homeDir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(pb.homeDir, ".smartclaw", "USER.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// SetPersona overrides the base personality.
func (pb *PromptBuilder) SetPersona(persona string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.persona = persona
	pb.dirty = true
}

// AddSkill appends a skill document. Marks prompt as dirty.
func (pb *PromptBuilder) AddSkill(skill string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.skills = append(pb.skills, skill)
	pb.dirty = true
}

// RemoveSkill removes a skill by index. Marks prompt as dirty.
func (pb *PromptBuilder) RemoveSkill(idx int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	if idx >= 0 && idx < len(pb.skills) {
		pb.skills = append(pb.skills[:idx], pb.skills[idx+1:]...)
		pb.dirty = true
	}
}

// AddContext appends a project context (e.g., AGENTS.md content). Marks dirty.
func (pb *PromptBuilder) AddContext(content string) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.contexts = append(pb.contexts, content)
	pb.dirty = true
}

// MarkDirty forces a rebuild on next Build() call.
// Use after events that change memory or config (compact, /memory edit, etc.)
func (pb *PromptBuilder) MarkDirty() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.dirty = true
}

// IsDirty returns whether the prompt needs rebuilding.
func (pb *PromptBuilder) IsDirty() bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.dirty
}

// Frozen returns the current frozen prompt without rebuilding.
func (pb *PromptBuilder) Frozen() string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	return pb.frozen
}
