package tools

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"log/slog"
)

// IntentPattern maps user intent keywords to likely needed files/context.
type IntentPattern struct {
	Keywords     []string `json:"keywords"`
	LikelyFiles  []string `json:"likely_files"`
	LikelyDirs   []string `json:"likely_dirs"`
	LikelyTools  []string `json:"likely_tools"`
	FilePatterns []string `json:"file_patterns"`
}

// PrefetchResult contains prefetched context.
type PrefetchResult struct {
	Files     map[string]string `json:"files"`
	ToolHints []string          `json:"tool_hints"`
	Skipped   int               `json:"skipped"`
}

// PredictivePrefetcher analyzes user intent and preloads relevant context.
type PredictivePrefetcher struct {
	mu       sync.RWMutex
	patterns []IntentPattern
	workDir  string
	enabled  bool
}

// NewPredictivePrefetcher creates a prefetcher with built-in intent patterns.
func NewPredictivePrefetcher(workDir string) *PredictivePrefetcher {
	pp := &PredictivePrefetcher{
		workDir: workDir,
		enabled: true,
	}
	pp.initPatterns()
	return pp
}

// Prefetch analyzes the user's message and returns preloaded file contents.
func (pp *PredictivePrefetcher) Prefetch(query string, maxFiles int) *PrefetchResult {
	if !pp.enabled || query == "" {
		return &PrefetchResult{Files: make(map[string]string)}
	}

	if maxFiles <= 0 {
		maxFiles = 5
	}

	queryLower := strings.ToLower(query)
	var matchedFiles []string
	var toolHints []string

	pp.mu.RLock()
	for _, pat := range pp.patterns {
		if pp.matchIntent(queryLower, pat.Keywords) {
			matchedFiles = append(matchedFiles, pat.LikelyFiles...)
			for _, dir := range pat.LikelyDirs {
				matchedFiles = append(matchedFiles, pp.expandDir(dir)...)
			}
			for _, fp := range pat.FilePatterns {
				matchedFiles = append(matchedFiles, pp.expandGlob(fp)...)
			}
			toolHints = append(toolHints, pat.LikelyTools...)
		}
	}
	pp.mu.RUnlock()

	matchedFiles = dedup(matchedFiles)

	result := &PrefetchResult{
		Files:     make(map[string]string),
		ToolHints: dedup(toolHints),
	}

	loaded := 0
	skipped := 0
	for _, f := range matchedFiles {
		if loaded >= maxFiles {
			skipped = len(matchedFiles) - loaded
			break
		}

		fullPath := f
		if !filepath.IsAbs(f) {
			fullPath = filepath.Join(pp.workDir, f)
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		content := string(data)
		if len(content) > 10000 {
			content = content[:10000] + "\n... (truncated)"
		}

		result.Files[f] = content
		loaded++

		slog.Debug("prefetch: loaded file", "file", f, "size", len(data))
	}

	result.Skipped = skipped
	return result
}

// AddPattern registers a custom intent pattern.
func (pp *PredictivePrefetcher) AddPattern(pattern IntentPattern) {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.patterns = append(pp.patterns, pattern)
}

// IsEnabled returns whether the prefetcher is active.
func (pp *PredictivePrefetcher) IsEnabled() bool {
	return pp.enabled
}

// SetEnabled toggles the prefetcher.
func (pp *PredictivePrefetcher) SetEnabled(enabled bool) {
	pp.enabled = enabled
}

func (pp *PredictivePrefetcher) matchIntent(query string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(query, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func (pp *PredictivePrefetcher) expandDir(dir string) []string {
	fullPath := dir
	if !filepath.IsAbs(dir) {
		fullPath = filepath.Join(pp.workDir, dir)
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil
	}

	var files []string
	skipExts := map[string]bool{".lock": true, ".log": true, ".min.js": true, ".min.css": true, ".map": true}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if skipExts[ext] {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
		if len(files) > 10 {
			break
		}
	}
	return files
}

func (pp *PredictivePrefetcher) expandGlob(pattern string) []string {
	fullPattern := pattern
	if !filepath.IsAbs(pattern) {
		fullPattern = filepath.Join(pp.workDir, pattern)
	}

	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil
	}

	if len(matches) > 10 {
		matches = matches[:10]
	}
	return matches
}

func (pp *PredictivePrefetcher) initPatterns() {
	pp.patterns = []IntentPattern{
		{
			Keywords:    []string{"build", "compile", "make", "build error", "compilation"},
			LikelyFiles: []string{"Makefile", "go.mod", "package.json", "Cargo.toml", "build.gradle", "pom.xml"},
			LikelyTools: []string{"bash"},
		},
		{
			Keywords:     []string{"test", "testing", "unit test", "integration test"},
			LikelyDirs:   []string{"test", "tests", "__tests__"},
			FilePatterns: []string{"*_test.go", "*_test.py", "*.test.js", "*.test.ts", "*.spec.ts"},
			LikelyTools:  []string{"bash"},
		},
		{
			Keywords:    []string{"deploy", "deployment", "release", "publish"},
			LikelyFiles: []string{"Dockerfile", "docker-compose.yml", ".github/workflows", "k8s", "deploy"},
			LikelyDirs:  []string{".github/workflows", "k8s", "deploy"},
			LikelyTools: []string{"bash"},
		},
		{
			Keywords:     []string{"auth", "login", "jwt", "oauth", "session", "password"},
			FilePatterns: []string{"*auth*", "*login*", "*session*", "*jwt*", "*middleware*"},
			LikelyTools:  []string{"grep", "read_file"},
		},
		{
			Keywords:     []string{"api", "endpoint", "route", "handler", "controller"},
			LikelyDirs:   []string{"api", "routes", "handlers", "controllers"},
			FilePatterns: []string{"*route*", "*handler*", "*controller*", "*api*"},
			LikelyTools:  []string{"grep", "read_file"},
		},
		{
			Keywords:    []string{"config", "configuration", "settings", "env"},
			LikelyFiles: []string{".env", "config.yaml", "config.json", "config.toml", "settings.json"},
			LikelyTools: []string{"read_file"},
		},
		{
			Keywords:     []string{"database", "sql", "migration", "schema", "model"},
			LikelyDirs:   []string{"migrations", "db", "models", "schema"},
			FilePatterns: []string{"*.sql", "*migration*", "*schema*", "*model*"},
			LikelyTools:  []string{"grep", "read_file"},
		},
		{
			Keywords:    []string{"bug", "fix", "error", "crash", "panic", "nil pointer"},
			LikelyFiles: []string{"go.mod", "package.json"},
			LikelyTools: []string{"bash", "grep", "glob"},
		},
		{
			Keywords:    []string{"refactor", "clean", "restructure", "move"},
			LikelyTools: []string{"grep", "glob", "read_file"},
		},
		{
			Keywords:    []string{"docker", "container", "image"},
			LikelyFiles: []string{"Dockerfile", "docker-compose.yml", ".dockerignore"},
			LikelyTools: []string{"bash"},
		},
		{
			Keywords:    []string{"git", "commit", "branch", "merge", "rebase"},
			LikelyTools: []string{"bash"},
		},
		{
			Keywords:     []string{"security", "vulnerability", "cve", "xss", "csrf", "injection"},
			FilePatterns: []string{"*auth*", "*security*", "*middleware*", "*sanitiz*"},
			LikelyTools:  []string{"grep"},
		},
	}
}

func dedup(slice []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
