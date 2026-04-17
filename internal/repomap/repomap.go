package repomap

import (
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultDamping    = 0.85
	defaultIterations = 100
	personalizeWeight = 100.0
	staleInterval     = 5 * time.Minute
)

// RepoMap generates a ranked summary of codebase symbols using PageRank.
// It gives the LLM "peripheral vision" of the entire codebase.
type RepoMap struct {
	rootPath    string
	symbols     map[string][]Symbol
	ranks       map[string]float64
	adjacency   map[string][]string
	mu          sync.RWMutex
	lastRefresh time.Time
}

// NewRepoMap creates a new RepoMap for the given root directory.
func NewRepoMap(rootPath string) *RepoMap {
	return &RepoMap{
		rootPath: rootPath,
	}
}

// GetMap returns a ranked, token-bounded summary of the codebase.
// chatFiles are given 100x personalization weight so they appear prominently.
// Symbols are re-extracted if the cache is stale (>5 minutes).
func (rm *RepoMap) GetMap(chatFiles []string, maxTokens int) (string, error) {
	rm.mu.Lock()
	if rm.symbols == nil || time.Since(rm.lastRefresh) > staleInterval {
		rm.mu.Unlock()
		if err := rm.Refresh(); err != nil {
			return "", err
		}
	} else {
		rm.mu.Unlock()
	}

	rm.mu.RLock()
	symbols := rm.symbols
	adj := rm.adjacency
	rm.mu.RUnlock()

	if len(symbols) == 0 {
		return "", nil
	}

	// Build personalization: chat files get 100x weight
	personalization := make(map[string]float64)
	for path := range symbols {
		personalization[path] = 1.0
	}
	for _, f := range chatFiles {
		// Normalize to relative path
		rel := f
		if filepath.IsAbs(f) {
			if r, err := filepath.Rel(rm.rootPath, f); err == nil {
				rel = r
			}
		}
		if _, ok := symbols[rel]; ok {
			personalization[rel] = personalizeWeight
		}
	}

	ranks := PageRank(adj, personalization, defaultDamping, defaultIterations)

	rm.mu.Lock()
	rm.ranks = ranks
	rm.mu.Unlock()

	return Render(ranks, symbols, maxTokens), nil
}

// Refresh forces a re-extraction of all symbols and rebuilds the adjacency graph.
func (rm *RepoMap) Refresh() error {
	symbols, err := ExtractSymbols(rm.rootPath)
	if err != nil {
		return fmt.Errorf("extract symbols: %w", err)
	}

	adj := buildAdjacency(rm.rootPath, symbols)

	rm.mu.Lock()
	rm.symbols = symbols
	rm.adjacency = adj
	rm.lastRefresh = time.Now()
	rm.mu.Unlock()

	return nil
}

// buildAdjacency constructs a file-level dependency graph from import declarations.
// Each file that imports another file's package gets a directed edge.
func buildAdjacency(rootPath string, symbols map[string][]Symbol) map[string][]string {
	pkgFiles := make(map[string][]string)
	for path := range symbols {
		dir := filepath.Dir(path)
		pkgFiles[dir] = append(pkgFiles[dir], path)
	}

	adj := make(map[string][]string)
	fset := token.NewFileSet()

	for path := range symbols {
		absPath := filepath.Join(rootPath, path)
		file, err := parser.ParseFile(fset, absPath, nil, parser.ImportsOnly)
		if err != nil {
			continue
		}

		seen := make(map[string]bool)
		for _, imp := range file.Imports {
			if imp.Path == nil {
				continue
			}
			importPath := strings.Trim(imp.Path.Value, `"`)
			// Only consider internal project imports
			dir := importPathToDir(importPath)
			if dir == "" {
				continue
			}
			for _, target := range pkgFiles[dir] {
				if target != path && !seen[target] {
					seen[target] = true
					adj[path] = append(adj[path], target)
				}
			}
		}
	}

	return adj
}

// importPathToDir converts a Go import path like "github.com/instructkr/smartclaw/internal/foo"
// to a relative directory like "internal/foo". Returns "" if not a project-internal import.
func importPathToDir(importPath string) string {
	const modulePrefix = "github.com/instructkr/smartclaw/"
	if !strings.HasPrefix(importPath, modulePrefix) {
		return ""
	}
	return strings.TrimPrefix(importPath, modulePrefix)
}
