package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ContextFilePriority defines the priority level of a context file.
// Lower values = higher priority. Only one file per priority level is allowed.
type ContextFilePriority int

const (
	// PrioritySoul is the highest priority — SOUL.md defines personality/identity.
	PrioritySoul ContextFilePriority = 1
	// PriorityAgents is the second priority — AGENTS.md provides agent instructions.
	PriorityAgents ContextFilePriority = 2
	// PriorityCursorRules is the lowest priority — .cursorrules provides IDE rules.
	PriorityCursorRules ContextFilePriority = 3
)

// ContextFileConfig controls how context file content is truncated when it
// exceeds the allocated budget.
type ContextFileConfig struct {
	// HeadPercent is the percentage of the budget allocated to the beginning of the file (default: 70).
	HeadPercent int
	// TailPercent is the percentage of the budget allocated to the end of the file (default: 20).
	TailPercent int
	// MiddleSkip controls whether the middle section is omitted when truncating (default: true).
	MiddleSkip bool
}

// DefaultContextFileConfig returns the default truncation configuration:
// head=70%, tail=20%, middle skipped.
func DefaultContextFileConfig() ContextFileConfig {
	return ContextFileConfig{
		HeadPercent: 70,
		TailPercent: 20,
		MiddleSkip:  true,
	}
}

// ContextFile represents a single context file with its priority and loaded content.
type ContextFile struct {
	Path     string
	Priority ContextFilePriority
	Content  string
	Loaded   bool
	Size     int
}

// AllocatedFile is a context file with budget allocation information.
type AllocatedFile struct {
	Path         string
	Priority     ContextFilePriority
	Content      string
	Truncated    bool
	OriginalSize int
	AllocSize    int
}

// ContextFileResolver loads and resolves context files (SOUL.md, AGENTS.md, .cursorrules)
// with a mutual-exclusion guarantee: only one file per priority level.
// Files are resolved in priority order; SOUL.md > AGENTS.md > .cursorrules.
type ContextFileResolver struct {
	files   map[ContextFilePriority]*ContextFile
	config  ContextFileConfig
	baseDir string
	mu      sync.RWMutex
}

// NewContextFileResolver creates a new resolver that looks for context files
// under baseDir. The config controls head/tail truncation behavior.
func NewContextFileResolver(baseDir string, config ContextFileConfig) *ContextFileResolver {
	return &ContextFileResolver{
		files: map[ContextFilePriority]*ContextFile{
			PrioritySoul: {
				Path:     filepath.Join(baseDir, "SOUL.md"),
				Priority: PrioritySoul,
			},
			PriorityAgents: {
				Path:     filepath.Join(baseDir, "AGENTS.md"),
				Priority: PriorityAgents,
			},
			PriorityCursorRules: {
				Path:     filepath.Join(baseDir, ".cursorrules"),
				Priority: PriorityCursorRules,
			},
		},
		config:  config,
		baseDir: baseDir,
	}
}

// Load reads all context files from disk in priority order.
// Files that don't exist are silently skipped (Loaded remains false).
func (r *ContextFileResolver) Load(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	priorities := []ContextFilePriority{PrioritySoul, PriorityAgents, PriorityCursorRules}
	var firstErr error

	for _, p := range priorities {
		cf, ok := r.files[p]
		if !ok {
			continue
		}
		if err := r.loadFile(cf); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// loadFile reads a single ContextFile from disk. Caller must hold r.mu.
func (r *ContextFileResolver) loadFile(cf *ContextFile) error {
	if _, err := os.Stat(cf.Path); os.IsNotExist(err) {
		cf.Loaded = false
		cf.Content = ""
		cf.Size = 0
		return nil
	}

	data, err := os.ReadFile(cf.Path)
	if err != nil {
		cf.Loaded = false
		return fmt.Errorf("context file resolver: read %s: %w", cf.Path, err)
	}

	cf.Content = string(data)
	cf.Size = len(cf.Content)
	cf.Loaded = true
	return nil
}

// Reload re-reads all context files from disk, refreshing their content.
func (r *ContextFileResolver) Reload(ctx context.Context) error {
	return r.Load(ctx)
}

// Resolve returns the content of the highest-priority file that exists and is loaded.
// The returned content is not truncated; use ResolveAll for budget-aware truncation.
func (r *ContextFileResolver) Resolve() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	priorities := []ContextFilePriority{PrioritySoul, PriorityAgents, PriorityCursorRules}
	for _, p := range priorities {
		cf, ok := r.files[p]
		if ok && cf.Loaded && cf.Content != "" {
			return cf.Content
		}
	}
	return ""
}

// ResolveAll returns all loaded files with budget allocation and head/tail truncation.
// maxChars is the total character budget distributed across files by priority.
// Files are allocated budget proportional to their priority; higher priority gets
// more budget. The remaining 10% (100 - HeadPercent - TailPercent) is reserved for
// the truncation marker overhead.
func (r *ContextFileResolver) ResolveAll(maxChars int) []AllocatedFile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	priorities := []ContextFilePriority{PrioritySoul, PriorityAgents, PriorityCursorRules}
	weights := map[ContextFilePriority]float64{
		PrioritySoul:        0.50,
		PriorityAgents:      0.30,
		PriorityCursorRules: 0.20,
	}

	// Collect loaded files and compute total weight.
	var loaded []*ContextFile
	totalWeight := 0.0
	for _, p := range priorities {
		cf, ok := r.files[p]
		if !ok || !cf.Loaded || cf.Content == "" {
			continue
		}
		loaded = append(loaded, cf)
		totalWeight += weights[p]
	}

	if len(loaded) == 0 || maxChars <= 0 {
		return nil
	}

	var result []AllocatedFile
	remaining := maxChars

	for i, cf := range loaded {
		// For the last file, give it all remaining budget.
		var budget int
		if i == len(loaded)-1 {
			budget = remaining
		} else {
			budget = int(float64(maxChars) * (weights[cf.Priority] / totalWeight))
			if budget > remaining {
				budget = remaining
			}
		}
		if budget < 0 {
			budget = 0
		}
		remaining -= budget

		content := cf.Content
		truncated := false
		if len(content) > budget {
			content = truncateWithHeadTail(content, budget, r.config)
			truncated = true
		}

		result = append(result, AllocatedFile{
			Path:         cf.Path,
			Priority:     cf.Priority,
			Content:      content,
			Truncated:    truncated,
			OriginalSize: cf.Size,
			AllocSize:    budget,
		})
	}

	return result
}

// GetFile returns the ContextFile at the given priority, or nil if not found.
func (r *ContextFileResolver) GetFile(priority ContextFilePriority) *ContextFile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.files[priority]
}

// SetFile writes content to the file at the given priority level, creating it
// if necessary, and updates the in-memory state.
func (r *ContextFileResolver) SetFile(priority ContextFilePriority, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cf, ok := r.files[priority]
	if !ok {
		return fmt.Errorf("context file resolver: unknown priority %d", priority)
	}

	if err := os.MkdirAll(filepath.Dir(cf.Path), 0755); err != nil {
		return fmt.Errorf("context file resolver: mkdir for %s: %w", cf.Path, err)
	}

	if err := os.WriteFile(cf.Path, []byte(content), 0644); err != nil {
		return fmt.Errorf("context file resolver: write %s: %w", cf.Path, err)
	}

	cf.Content = content
	cf.Size = len(content)
	cf.Loaded = true
	return nil
}

// SetCursorRulesPath allows overriding the .cursorrules file path,
// e.g. to check the project CWD instead of baseDir.
func (r *ContextFileResolver) SetCursorRulesPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cf, ok := r.files[PriorityCursorRules]; ok {
		cf.Path = path
		// Reset loaded state since path changed.
		cf.Loaded = false
		cf.Content = ""
		cf.Size = 0
	}
}

// truncateWithHeadTail truncates content to fit within maxChars using head/tail allocation.
// It allocates headPercent of maxChars to the beginning, tailPercent to the end,
// and inserts a "[...context truncated...]" marker in between.
// If content fits within maxChars, it is returned as-is.
// If maxChars is 0 or negative, an empty string is returned.
func truncateWithHeadTail(content string, maxChars int, config ContextFileConfig) string {
	if maxChars <= 0 {
		return ""
	}

	if len(content) <= maxChars {
		return content
	}

	const marker = "\n[...context truncated...]\n"
	markerLen := len(marker)

	headPercent := config.HeadPercent
	tailPercent := config.TailPercent
	if headPercent <= 0 {
		headPercent = 70
	}
	if tailPercent <= 0 {
		tailPercent = 20
	}
	if headPercent+tailPercent > 100 {
		headPercent = 70
		tailPercent = 20
	}

	// Available budget after reserving space for the marker.
	available := maxChars - markerLen
	if available <= 0 {
		// Not enough room even for the marker; just return the head.
		if maxChars > 0 {
			return content[:maxChars]
		}
		return ""
	}

	headSize := available * headPercent / 100
	tailSize := available * tailPercent / 100
	// Any remaining chars (the middle percentage) are simply not allocated.

	// Ensure we don't exceed content bounds.
	if headSize > len(content) {
		headSize = len(content)
	}
	if tailSize > len(content)-headSize {
		tailSize = len(content) - headSize
	}

	head := content[:headSize]
	tail := content[len(content)-tailSize:]

	return head + marker + tail
}
