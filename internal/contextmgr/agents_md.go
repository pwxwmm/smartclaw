package contextmgr

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AgentsMDScope defines the scope level for an AGENTS.md file.
type AgentsMDScope string

const (
	// ScopeProject is AGENTS.md in the project root (highest priority).
	ScopeProject AgentsMDScope = "project"
	// ScopeDirectory is AGENTS.md in a subdirectory (discovered on file read).
	ScopeDirectory AgentsMDScope = "directory"
	// ScopeUser is AGENTS.md in ~/.smartclaw/AGENTS.md (global user context).
	ScopeUser AgentsMDScope = "user"
	// ScopeWorkspace is AGENTS.md in a parent workspace dir.
	ScopeWorkspace AgentsMDScope = "workspace"
)

// scopeOrder defines the resolution precedence. Earlier entries take priority.
var scopeOrder = []AgentsMDScope{ScopeProject, ScopeDirectory, ScopeUser, ScopeWorkspace}

// scopeLabel returns the human-readable section header for a scope.
func scopeLabel(scope AgentsMDScope) string {
	switch scope {
	case ScopeProject:
		return "Project Context"
	case ScopeDirectory:
		return "Directory Context"
	case ScopeUser:
		return "User Context"
	case ScopeWorkspace:
		return "Workspace Context"
	default:
		return string(scope)
	}
}

// AgentsMDFile represents a single loaded AGENTS.md file with its scope.
type AgentsMDFile struct {
	Path    string
	Scope   AgentsMDScope
	Content string
	Loaded  bool
	Size    int
}

// AgentsMDHierarchy manages the loading and merging of AGENTS.md files
// from multiple scope levels with proper precedence.
//
// Resolution order (first match wins per scope):
//  1. Project: <project-root>/AGENTS.md
//  2. Directory: <subdir>/AGENTS.md (discovered dynamically when reading files)
//  3. User: ~/.smartclaw/AGENTS.md
//  4. Workspace: parent directories up to filesystem root
type AgentsMDHierarchy struct {
	files      map[string]*AgentsMDFile // key = absolute path
	projectDir string
	userDir    string
	mu         sync.RWMutex
}

// NewAgentsMDHierarchy creates a new hierarchy resolver.
// projectDir is the project root directory.
// userDir is the user config directory (typically ~/.smartclaw/).
func NewAgentsMDHierarchy(projectDir, userDir string) *AgentsMDHierarchy {
	return &AgentsMDHierarchy{
		files:      make(map[string]*AgentsMDFile),
		projectDir: filepath.Clean(projectDir),
		userDir:    filepath.Clean(userDir),
	}
}

// Load reads all AGENTS.md files from project root, user dir, and workspace parents.
// Files that do not exist are silently skipped (not an error).
// Files already loaded at a higher priority scope are not re-included.
func (h *AgentsMDHierarchy) Load(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var firstErr error

	// 1. Project scope: <projectDir>/AGENTS.md
	if err := h.loadFile(filepath.Join(h.projectDir, "AGENTS.md"), ScopeProject); err != nil && firstErr == nil {
		firstErr = err
	}

	// 2. User scope: <userDir>/AGENTS.md
	if err := h.loadFile(filepath.Join(h.userDir, "AGENTS.md"), ScopeUser); err != nil && firstErr == nil {
		firstErr = err
	}

	// 3. Workspace scope: walk upward from projectDir looking for AGENTS.md
	//    in parent directories up to filesystem root.
	if err := h.loadWorkspaceParents(); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

// LoadForDirectory discovers and loads AGENTS.md files from a specific directory
// and its parent directories up to projectDir. Used for dynamic injection when
// reading files in subdirectories.
//
// Only loads files that have not already been loaded at a higher scope.
func (h *AgentsMDHierarchy) LoadForDirectory(ctx context.Context, dir string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	dir = filepath.Clean(dir)

	current := dir
	for {
		if current == h.projectDir || strings.HasPrefix(h.projectDir, current+string(filepath.Separator)) {
			break
		}

		agentsPath := filepath.Join(current, "AGENTS.md")
		if _, alreadyLoaded := h.files[agentsPath]; !alreadyLoaded {
			if err := h.loadFile(agentsPath, ScopeDirectory); err != nil {
				slog.Debug("agents_md: failed to load directory AGENTS.md", "path", agentsPath, "error", err)
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return nil
}

// Resolve returns the merged content of all AGENTS.md files in scope order.
// Project-level content comes first, then directory-level, then user-level, then workspace.
// Each section is labeled with its source for transparency.
func (h *AgentsMDHierarchy) Resolve() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.resolveWithBudget(0)
}

// ResolveForFile returns merged AGENTS.md content relevant to a specific file.
// This walks upward from the file's directory, loading any AGENTS.md found along the way.
// Files already loaded at a higher scope are not re-included.
func (h *AgentsMDHierarchy) ResolveForFile(ctx context.Context, filePath string) string {
	// Load directory-level AGENTS.md files relevant to this file.
	_ = h.LoadForDirectory(ctx, filepath.Dir(filepath.Clean(filePath)))

	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.resolveWithBudget(0)
}

// List returns all loaded AGENTS.md files with their metadata,
// ordered by scope precedence.
func (h *AgentsMDHierarchy) List() []*AgentsMDFile {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.orderedFiles()
}

// Reload re-reads all files from disk, preserving the scope assignments.
func (h *AgentsMDHierarchy) Reload(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	paths := make(map[string]AgentsMDScope)
	for path, f := range h.files {
		paths[path] = f.Scope
	}
	h.files = make(map[string]*AgentsMDFile)

	var firstErr error

	for path, scope := range paths {
		if err := h.loadFile(path, scope); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// GetFile returns a specific loaded file by its absolute path, or nil if not found.
func (h *AgentsMDHierarchy) GetFile(path string) *AgentsMDFile {
	h.mu.RLock()
	defer h.mu.RUnlock()

	absPath := path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(h.projectDir, absPath)
	}
	return h.files[filepath.Clean(absPath)]
}

// loadFile reads a single AGENTS.md from disk. Caller must hold h.mu.
func (h *AgentsMDHierarchy) loadFile(absPath string, scope AgentsMDScope) error {
	absPath = filepath.Clean(absPath)

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("agents_md: read %s: %w", absPath, err)
	}

	h.files[absPath] = &AgentsMDFile{
		Path:    absPath,
		Scope:   scope,
		Content: string(data),
		Loaded:  true,
		Size:    len(data),
	}

	slog.Debug("agents_md: loaded", "path", absPath, "scope", scope, "size", len(data))
	return nil
}

// loadWorkspaceParents walks from projectDir upward to the filesystem root,
// loading any AGENTS.md found in parent directories. Caller must hold h.mu.
func (h *AgentsMDHierarchy) loadWorkspaceParents() error {
	current := filepath.Dir(h.projectDir)
	var firstErr error

	for {
		if current == "." || current == string(filepath.Separator) {
			break
		}

		agentsPath := filepath.Join(current, "AGENTS.md")
		if _, alreadyLoaded := h.files[agentsPath]; !alreadyLoaded {
			if err := h.loadFile(agentsPath, ScopeWorkspace); err != nil && firstErr == nil {
				firstErr = err
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return firstErr
}

// orderedFiles returns loaded files sorted by scope precedence.
// Caller must hold h.mu (at least RLock).
func (h *AgentsMDHierarchy) orderedFiles() []*AgentsMDFile {
	byScope := make(map[AgentsMDScope][]*AgentsMDFile)
	for _, f := range h.files {
		if f.Loaded {
			byScope[f.Scope] = append(byScope[f.Scope], f)
		}
	}

	var result []*AgentsMDFile
	for _, scope := range scopeOrder {
		result = append(result, byScope[scope]...)
	}
	return result
}

// resolveWithBudget merges all loaded AGENTS.md files in scope order with
// section headers. If budget > 0, lower-priority scopes are truncated first.
// Project scope content is never truncated.
func (h *AgentsMDHierarchy) resolveWithBudget(budget int) string {
	files := h.orderedFiles()
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder

	for _, f := range files {
		if f.Content == "" {
			continue
		}

		header := fmt.Sprintf("## %s (%s)", scopeLabel(f.Scope), f.Path)
		sb.WriteString(header)
		sb.WriteString("\n")
		sb.WriteString(f.Content)
		if !strings.HasSuffix(f.Content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// AgentsMDProvider wraps AgentsMDHierarchy as a ContextProvider.
type AgentsMDProvider struct {
	Hierarchy *AgentsMDHierarchy
}

// NewAgentsMDProvider creates a new provider backed by the given hierarchy.
func NewAgentsMDProvider(hierarchy *AgentsMDHierarchy) *AgentsMDProvider {
	return &AgentsMDProvider{Hierarchy: hierarchy}
}

// Name returns the provider name for context assembly.
func (p *AgentsMDProvider) Name() string { return "agents_md" }

// Provide returns AGENTS.md content as ContextItems, respecting the token budget.
// If the query references a file path, directory-scoped AGENTS.md files relevant
// to that path are dynamically loaded before resolving.
func (p *AgentsMDProvider) Provide(ctx context.Context, query string, budget int) ([]ContextItem, error) {
	if p.Hierarchy == nil {
		return nil, nil
	}

	// If the query looks like a file path, resolve directory-scoped context.
	if isLikelyFilePath(query) {
		absPath := query
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(p.Hierarchy.projectDir, query)
		}
		_ = p.Hierarchy.LoadForDirectory(ctx, filepath.Dir(absPath))
	}

	content := p.Hierarchy.Resolve()
	if content == "" {
		return nil, nil
	}

	tokens := EstimateTokens(content)

	// If over budget, truncate lower-priority scopes.
	if budget > 0 && tokens > budget {
		content = p.truncateToBudget(budget)
		tokens = EstimateTokens(content)
	}

	return []ContextItem{
		{
			Source:     p.Name(),
			Type:       "agents_md",
			Content:    content,
			TokenCount: tokens,
			Timestamp:  time.Now(),
		},
	}, nil
}

// truncateToBudget removes content from lower-priority scopes until the
// merged content fits within the budget. Project scope is never truncated.
func (p *AgentsMDProvider) truncateToBudget(budget int) string {
	p.Hierarchy.mu.RLock()
	defer p.Hierarchy.mu.RUnlock()

	files := p.Hierarchy.orderedFiles()

	// Work backwards through scopes, truncating from lowest priority.
	// Build content from highest priority down, respecting budget.
	maxChars := budget * 4 // rough token-to-char estimate

	var sb strings.Builder
	remaining := maxChars

	for _, f := range files {
		if f.Content == "" {
			continue
		}

		header := fmt.Sprintf("## %s (%s)", scopeLabel(f.Scope), f.Path)
		sectionSize := len(header) + 1 + len(f.Content) + 2 // header\n + content + \n\n

		if f.Scope == ScopeProject {
			// Project scope is never truncated.
			sb.WriteString(header)
			sb.WriteString("\n")
			sb.WriteString(f.Content)
			if !strings.HasSuffix(f.Content, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
			remaining -= sectionSize
			continue
		}

		// For non-project scopes, respect remaining budget.
		if remaining <= 0 {
			continue
		}

		content := f.Content
		if sectionSize > remaining {
			// Truncate content to fit.
			available := remaining - len(header) - 3 // header\n\n\n
			if available <= 0 {
				continue
			}
			if available < len(content) {
				content = content[:available]
			}
		}

		sb.WriteString(header)
		sb.WriteString("\n")
		sb.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
		remaining -= sectionSize
	}

	return sb.String()
}
