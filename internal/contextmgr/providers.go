package contextmgr

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/git"
	"github.com/instructkr/smartclaw/internal/index"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/utils"
)

type ContextItem struct {
	Source     string    // provider name
	Type       string    // "symbol", "file", "snippet", "memory", "git_diff", "search_result"
	Content    string    // actual text content
	FilePath   string    // optional file path
	StartLine  int       // optional line range start
	EndLine    int       // optional line range end
	Relevance  float64   // computed relevance score
	TokenCount int       // estimated tokens
	Timestamp  time.Time // when item was created/modified
}

type ScoredContextItem struct {
	Item      ContextItem
	Relevance float64
}

type ContextProvider interface {
	Name() string
	Provide(ctx context.Context, query string, budget int) ([]ContextItem, error)
}

// FileProvider reads files mentioned in the query or recently accessed.
type FileProvider struct {
	WorkDir     string
	RecentFiles []string
	MaxFileSize int
}

func NewFileProvider(workDir string) *FileProvider {
	return &FileProvider{
		WorkDir:     workDir,
		MaxFileSize: 50000,
	}
}

func (fp *FileProvider) Name() string { return "files" }

func (fp *FileProvider) Provide(_ context.Context, query string, budget int) ([]ContextItem, error) {
	candidates := fp.extractFilePaths(query)
	candidates = append(candidates, fp.RecentFiles...)

	seen := make(map[string]bool)
	var items []ContextItem
	tokenBudget := budget

	for _, path := range candidates {
		if seen[path] || tokenBudget <= 0 {
			continue
		}
		seen[path] = true

		absPath := path
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(fp.WorkDir, path)
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		content := string(data)
		if len(content) > fp.MaxFileSize {
			content = content[:fp.MaxFileSize]
		}

		tokens := utils.CountTokens(content)
		if tokens > tokenBudget {
			maxChars := tokenBudget * 4
			if maxChars < len(content) {
				content = content[:maxChars]
			}
			tokens = utils.CountTokens(content)
		}

		items = append(items, ContextItem{
			Source:     fp.Name(),
			Type:       "file",
			Content:    content,
			FilePath:   path,
			TokenCount: tokens,
			Timestamp:  time.Now(),
		})
		tokenBudget -= tokens
	}

	return items, nil
}

func (fp *FileProvider) extractFilePaths(query string) []string {
	var paths []string
	fields := strings.Fields(query)
	for _, f := range fields {
		clean := strings.Trim(f, "\"'`(),;")
		if isLikelyFilePath(clean) {
			paths = append(paths, clean)
		}
	}
	return paths
}

func isLikelyFilePath(s string) bool {
	if len(s) < 3 {
		return false
	}
	hasExt := strings.Contains(s, ".") && !strings.HasPrefix(s, "http")
	hasSlash := strings.Contains(s, "/")
	return hasExt || hasSlash
}

// SymbolProvider extracts relevant symbols from the codebase index.
type SymbolProvider struct {
	Index   *index.CodebaseIndex
	WorkDir string
}

func NewSymbolProvider(idx *index.CodebaseIndex, workDir string) *SymbolProvider {
	return &SymbolProvider{Index: idx, WorkDir: workDir}
}

func (sp *SymbolProvider) Name() string { return "files" }

func (sp *SymbolProvider) Provide(_ context.Context, query string, budget int) ([]ContextItem, error) {
	if sp.Index == nil {
		return sp.fallbackGrep(query, budget)
	}

	queryEmbedding := index.GenerateEmbedding(query)
	allEmbeddings := sp.Index.ChunkEmbeddings()

	results := index.SimilaritySearch(queryEmbedding, allEmbeddings, 20)
	if len(results) == 0 {
		return sp.fallbackGrep(query, budget)
	}

	var items []ScoredContextItem
	for _, r := range results {
		chunk, ok := sp.Index.GetChunk(r.ID)
		if !ok {
			continue
		}

		tokens := utils.CountTokens(chunk.Content)
		items = append(items, ScoredContextItem{
			Item: ContextItem{
				Source:     sp.Name(),
				Type:       "symbol",
				Content:    chunk.Content,
				FilePath:   chunk.File,
				StartLine:  chunk.StartLine,
				EndLine:    chunk.EndLine,
				Relevance:  r.Score,
				TokenCount: tokens,
				Timestamp:  time.Now(),
			},
			Relevance: r.Score,
		})
	}

	sortScored(items)

	var selected []ContextItem
	used := 0
	for _, si := range items {
		if used+si.Item.TokenCount > budget {
			break
		}
		selected = append(selected, si.Item)
		used += si.Item.TokenCount
	}

	return selected, nil
}

func (sp *SymbolProvider) fallbackGrep(query string, budget int) ([]ContextItem, error) {
	terms := tokenizeLower(query)
	if len(terms) == 0 {
		return nil, nil
	}

	pattern := terms[0]
	cmd := exec.Command("grep", "-rn", "--include=*.go", "-m", "5", pattern, sp.WorkDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var items []ContextItem
	used := 0

	for _, line := range lines {
		if used >= budget {
			break
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		filePath := parts[0]
		content := parts[2]
		tokens := utils.CountTokens(content)
		items = append(items, ContextItem{
			Source:     sp.Name(),
			Type:       "snippet",
			Content:    content,
			FilePath:   filePath,
			TokenCount: tokens,
			Timestamp:  time.Now(),
		})
		used += tokens
	}

	return items, nil
}

// MemoryProvider wraps the memory manager for L1-L4 memories.
type MemoryProvider struct {
	MemoryManager *memory.MemoryManager
}

func NewMemoryProvider(mm *memory.MemoryManager) *MemoryProvider {
	return &MemoryProvider{MemoryManager: mm}
}

func (mp *MemoryProvider) Name() string { return "memory" }

func (mp *MemoryProvider) Provide(ctx context.Context, query string, budget int) ([]ContextItem, error) {
	if mp.MemoryManager == nil {
		return nil, nil
	}

	systemCtx := mp.MemoryManager.BuildSystemContext(ctx, query)
	if systemCtx == "" {
		return nil, nil
	}

	tokens := utils.CountTokens(systemCtx)
	if tokens > budget {
		maxChars := budget * 4
		if maxChars < len(systemCtx) {
			systemCtx = systemCtx[:maxChars]
		}
		tokens = utils.CountTokens(systemCtx)
	}

	return []ContextItem{
		{
			Source:     mp.Name(),
			Type:       "memory",
			Content:    systemCtx,
			TokenCount: tokens,
			Timestamp:  time.Now(),
		},
	}, nil
}

// GitProvider provides recent git diff and branch information.
type GitProvider struct {
	GitContext *git.Context
}

func NewGitProvider(gc *git.Context) *GitProvider {
	return &GitProvider{GitContext: gc}
}

func (gp *GitProvider) Name() string { return "git" }

func (gp *GitProvider) Provide(_ context.Context, _ string, budget int) ([]ContextItem, error) {
	if gp.GitContext == nil || !gp.GitContext.IsRepo {
		return nil, nil
	}

	var items []ContextItem
	used := 0

	branchInfo := gp.GitContext.String()
	bt := utils.CountTokens(branchInfo)
	if bt <= budget {
		items = append(items, ContextItem{
			Source:     gp.Name(),
			Type:       "git_diff",
			Content:    branchInfo,
			TokenCount: bt,
			Timestamp:  time.Now(),
		})
		used += bt
	}

	diff, err := git.GetDiff(gp.GitContext.RootDir, false)
	if err == nil && diff != "" && used < budget {
		dt := utils.CountTokens(diff)
		maxDiffTokens := budget - used
		if dt > maxDiffTokens {
			maxChars := maxDiffTokens * 4
			if maxChars < len(diff) {
				diff = diff[:maxChars]
			}
			dt = utils.CountTokens(diff)
		}
		if dt > 0 {
			items = append(items, ContextItem{
				Source:     gp.Name(),
				Type:       "git_diff",
				Content:    fmt.Sprintf("Unstaged diff:\n%s", diff),
				TokenCount: dt,
				FilePath:   "",
				Timestamp:  time.Now(),
			})
			used += dt
		}
	}

	return items, nil
}

// SearchProvider searches the codebase for relevant code snippets.
type SearchProvider struct {
	WorkDir    string
	MaxResults int
}

func NewSearchProvider(workDir string) *SearchProvider {
	return &SearchProvider{
		WorkDir:    workDir,
		MaxResults: 10,
	}
}

func (sp *SearchProvider) Name() string { return "search" }

func (sp *SearchProvider) Provide(_ context.Context, query string, budget int) ([]ContextItem, error) {
	terms := tokenizeLower(query)
	if len(terms) == 0 {
		return nil, nil
	}

	pattern := terms[0]
	if len(terms) > 1 {
		pattern = terms[0] + "|" + terms[1]
	}

	args := []string{
		"grep", "-rnE",
		"--include=*.go", "--include=*.py", "--include=*.ts", "--include=*.js",
		"-m", fmt.Sprintf("%d", sp.MaxResults),
		pattern, sp.WorkDir,
	}

	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("contextmgr: search provider grep failed", "error", err)
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var items []ContextItem
	used := 0

	for _, line := range lines {
		if used >= budget || line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		content := parts[2]
		tokens := utils.CountTokens(content)
		if used+tokens > budget {
			break
		}
		items = append(items, ContextItem{
			Source:     sp.Name(),
			Type:       "search_result",
			Content:    content,
			FilePath:   parts[0],
			StartLine:  0,
			TokenCount: tokens,
			Timestamp:  time.Now(),
		})
		used += tokens
	}

	return items, nil
}
