package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/instructkr/smartclaw/internal/costguard"
	"github.com/instructkr/smartclaw/internal/mcp"
	"github.com/instructkr/smartclaw/internal/observability"
	"github.com/instructkr/smartclaw/internal/store"
)

type FileNode struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Size     int64      `json:"size,omitempty"`
	Children []FileNode `json:"children,omitempty"`
}

type SessionInfo struct {
	ID           string `json:"id"`
	UserID       string `json:"userId,omitempty"`
	Title        string `json:"title"`
	Model        string `json:"model"`
	MessageCount int    `json:"messageCount"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type StatsResponse struct {
	TokensUsed   int     `json:"tokensUsed"`
	TokensLimit  int     `json:"tokensLimit"`
	Cost         float64 `json:"cost"`
	Model        string  `json:"model"`
	SessionCount int     `json:"sessionCount"`
}

type ChatSearchResult struct {
	SessionID    string `json:"sessionId"`
	SessionTitle string `json:"sessionTitle"`
	Role         string `json:"role"`
	Content      string `json:"content"`
	Timestamp    string `json:"timestamp"`
	MatchIndex   int    `json:"matchIndex"`
}

type MCPCatalogEntry struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Type        string   `json:"type"`
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	Popular     bool     `json:"popular"`
	Installed   bool     `json:"installed"`
}

var MCPCatalog = []MCPCatalogEntry{
	{Name: "filesystem", Description: "File system operations with access controls", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}, Popular: true},
	{Name: "github", Description: "GitHub API - repos, issues, PRs, search", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-github"}, Popular: true},
	{Name: "postgres", Description: "PostgreSQL database queries and schema", Category: "Data", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-postgres"}, Popular: true},
	{Name: "sqlite", Description: "SQLite database exploration and queries", Category: "Data", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-sqlite"}},
	{Name: "fetch", Description: "Web content fetching and search", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-fetch"}, Popular: true},
	{Name: "brave-search", Description: "Web search via Brave Search API", Category: "AI", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-brave-search"}},
	{Name: "memory", Description: "Knowledge graph and persistent memory", Category: "AI", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-memory"}, Popular: true},
	{Name: "puppeteer", Description: "Browser automation via Puppeteer", Category: "Operations", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-puppeteer"}},
	{Name: "slack", Description: "Slack messaging and channel management", Category: "Communication", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-slack"}},
	{Name: "google-maps", Description: "Google Maps directions, places, geocoding", Category: "Productivity", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-google-maps"}},
	{Name: "sequential-thinking", Description: "Structured problem-solving and reasoning", Category: "AI", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-sequential-thinking"}},
	{Name: "everything", Description: "MCP test server with all features", Category: "Development", Type: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-everything"}},
}

var PresetTemplates = []map[string]any{
	{"id": "preset-code-review", "name": "Code Review", "description": "Review code for bugs, performance, and best practices", "category": "Code Quality", "content": "Review this code for bugs, performance issues, and best practices:\n\n{{file}}\n\nFocus on: error handling, security, readability", "variables": []string{"file"}, "isPreset": true},
	{"id": "preset-bug-fix", "name": "Bug Fix", "description": "Analyze and fix bugs in code", "category": "Debugging", "content": "Analyze and fix the bug in this code:\n\n{{file}}\n\nThe issue is: ", "variables": []string{"file"}, "isPreset": true},
	{"id": "preset-doc-generation", "name": "Doc Generation", "description": "Generate documentation for code", "category": "Documentation", "content": "Generate documentation for:\n\n{{file}}\n\nInclude: function descriptions, parameters, return values, usage examples", "variables": []string{"file"}, "isPreset": true},
	{"id": "preset-test-writing", "name": "Test Writing", "description": "Write comprehensive tests for code", "category": "Testing", "content": "Write tests for:\n\n{{file}}\n\nCover: edge cases, error paths, happy path", "variables": []string{"file"}, "isPreset": true},
	{"id": "preset-refactor", "name": "Refactor", "description": "Refactor code for better readability and maintainability", "category": "Code Quality", "content": "Refactor this code for better readability and maintainability:\n\n{{file}}\n\nKeep the same functionality", "variables": []string{"file"}, "isPreset": true},
	{"id": "preset-explain-code", "name": "Explain Code", "description": "Explain what code does step by step", "category": "Learning", "content": "Explain what this code does step by step:\n\n{{file}}", "variables": []string{"file"}, "isPreset": true},
	{"id": "preset-security-audit", "name": "Security Audit", "description": "Perform a security audit on code", "category": "Security", "content": "Perform a security audit on:\n\n{{file}}\n\nCheck for: injection, auth issues, data exposure, misconfigurations", "variables": []string{"file"}, "isPreset": true},
	{"id": "preset-performance-review", "name": "Performance Review", "description": "Review performance of code", "category": "Performance", "content": "Review performance of:\n\n{{file}}\n\nIdentify: bottlenecks, unnecessary allocations, O(n²) patterns", "variables": []string{"file"}, "isPreset": true},
	{"id": "preset-git-commit", "name": "Git Commit Message", "description": "Generate a commit message for changes", "category": "Git", "content": "Generate a commit message for these changes:\n\n{{selection}}", "variables": []string{"selection"}, "isPreset": true},
	{"id": "preset-readme-generator", "name": "README Generator", "description": "Generate a README.md for a project", "category": "Documentation", "content": "Generate a README.md for this project:\n\nKey files: {{file}}\nLanguage: {{language}}", "variables": []string{"file", "language"}, "isPreset": true},
}

func GetCatalogWithInstalledStatus(registry *mcp.MCPServerRegistry) []MCPCatalogEntry {
	installed := make(map[string]bool)
	if registry != nil {
		for _, s := range registry.ListServers() {
			installed[s.Name] = true
		}
	}

	result := make([]MCPCatalogEntry, len(MCPCatalog))
	for i, entry := range MCPCatalog {
		result[i] = entry
		result[i].Installed = installed[entry.Name]
	}
	return result
}

func CacheHitRate(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

const maxQueryLimit = 1000

func clampLimit(limit, defaultLimit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxQueryLimit {
		return maxQueryLimit
	}
	return limit
}

func EstimateCost(snapshot observability.MetricsSnapshot) float64 {
	model := ""
	if len(snapshot.ModelQueryCounts) > 0 {
		for m := range snapshot.ModelQueryCounts {
			model = m
			break
		}
	}
	cg := costguard.NewCostGuard(costguard.DefaultBudgetConfig())
	cost, _ := cg.CalculateCost(model, int(snapshot.TotalInputTokens), int(snapshot.TotalOutputTokens))
	return cost
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func BuildFileTree(root string, maxDepth int) ([]FileNode, error) {
	if maxDepth <= 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var nodes []FileNode
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != ".smartclaw" {
			continue
		}

		node := FileNode{Name: name}

		if entry.IsDir() {
			skipDirs := map[string]bool{
				"node_modules": true, "vendor": true, ".git": true,
				"dist": true, "build": true, "bin": true, "__pycache__": true,
			}
			if skipDirs[name] {
				node.Type = "dir"
				continue
			}

			node.Type = "dir"
			children, err := BuildFileTree(filepath.Join(root, name), maxDepth-1)
			if err == nil {
				node.Children = children
			}
		} else {
			info, err := entry.Info()
			if err == nil {
				node.Size = info.Size()
			}
			node.Type = "file"
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

func GetGitStatus(workDir string) (map[string]string, error) {
	gitDir := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		statusCode := strings.TrimSpace(line[:2])
		path := line[3:]
		if path == "" {
			continue
		}
		result[path] = statusCode
	}
	return result, nil
}

func SearchMessages(dataStore *store.Store, query, userID string, limit int, codeOnly bool) ([]ChatSearchResult, error) {
	if dataStore == nil {
		return nil, nil
	}

	db := dataStore.DB()
	if db == nil {
		return nil, nil
	}

	ftsAvailable := true
	var rows interface {
		Close() error
		Next() bool
		Scan(dest ...any) error
	}

	codeBlockLike := "%" + "```" + "%"
	if codeOnly {
		ftsQuery := query
		var err error
		if userID != "" && userID != "default" {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, f.rank
				FROM messages_fts f
				JOIN messages m ON m.id = f.rowid
				JOIN sessions s ON m.session_id = s.id
				WHERE messages_fts MATCH ? AND s.user_id = ? AND (m.content LIKE ? OR m.tool_name != '')
				ORDER BY f.rank
				LIMIT ?
			`, ftsQuery, userID, codeBlockLike, limit)
		} else {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, f.rank
				FROM messages_fts f
				JOIN messages m ON m.id = f.rowid
				JOIN sessions s ON m.session_id = s.id
				WHERE messages_fts MATCH ? AND (m.content LIKE ? OR m.tool_name != '')
				ORDER BY f.rank
				LIMIT ?
			`, ftsQuery, codeBlockLike, limit)
		}
		if err != nil {
			ftsAvailable = false
		}
	} else {
		ftsQuery := query
		var err error
		if userID != "" && userID != "default" {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, f.rank
				FROM messages_fts f
				JOIN messages m ON m.id = f.rowid
				JOIN sessions s ON m.session_id = s.id
				WHERE messages_fts MATCH ? AND s.user_id = ?
				ORDER BY f.rank
				LIMIT ?
			`, ftsQuery, userID, limit)
		} else {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, f.rank
				FROM messages_fts f
				JOIN messages m ON m.id = f.rowid
				JOIN sessions s ON m.session_id = s.id
				WHERE messages_fts MATCH ?
				ORDER BY f.rank
				LIMIT ?
			`, ftsQuery, limit)
		}
		if err != nil {
			ftsAvailable = false
		}
	}

	if !ftsAvailable {
		likePattern := "%" + query + "%"
		var err error
		if userID != "" && userID != "default" {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, 0
				FROM messages m
				JOIN sessions s ON m.session_id = s.id
				WHERE m.content LIKE ? AND s.user_id = ?
				ORDER BY m.timestamp DESC
				LIMIT ?
			`, likePattern, userID, limit)
		} else {
			rows, err = db.Query(`
				SELECT m.session_id, m.role, m.content, m.timestamp, s.title, 0
				FROM messages m
				JOIN sessions s ON m.session_id = s.id
				WHERE m.content LIKE ?
				ORDER BY m.timestamp DESC
				LIMIT ?
			`, likePattern, limit)
		}
		if err != nil {
			return nil, fmt.Errorf("search query failed: %w", err)
		}
	}
	defer rows.Close()

	var results []ChatSearchResult
	for rows.Next() {
		var sessionID, role, content, sessionTitle string
		var ts string
		var rank float64
		if err := rows.Scan(&sessionID, &role, &content, &ts, &sessionTitle, &rank); err != nil {
			continue
		}

		if codeOnly && !strings.Contains(content, "```") {
			continue
		}

		matchIndex := strings.Index(strings.ToLower(content), strings.ToLower(query))

		excerpt := truncateAroundMatch(content, query, 200)

		parsedTime := ts
		if t, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
			parsedTime = t.Format(time.RFC3339)
		}

		results = append(results, ChatSearchResult{
			SessionID:    sessionID,
			SessionTitle: sessionTitle,
			Role:         role,
			Content:      excerpt,
			Timestamp:    parsedTime,
			MatchIndex:   matchIndex,
		})
	}

	return results, nil
}

func truncateAroundMatch(content, query string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	idx := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if idx < 0 {
		return content[:maxLen] + "..."
	}

	half := maxLen / 2
	start := idx - half
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(content) {
		end = len(content)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}

	result := content[start:end]
	if start > 0 {
		result = "..." + result
	}
	if end < len(content) {
		result = result + "..."
	}
	return result
}

func LoadCustomTemplates(dataStore *store.Store) ([]map[string]any, error) {
	if dataStore == nil {
		return nil, fmt.Errorf("database not available")
	}

	rows, err := dataStore.DB().Query(
		`SELECT id, user_id, name, description, category, content, variables, created_at, updated_at FROM prompt_templates WHERE user_id=? ORDER BY updated_at DESC`,
		"default",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []map[string]any
	for rows.Next() {
		var id, userID, name, description, category, content, variablesJSON string
		var createdAt, updatedAt string
		if err := rows.Scan(&id, &userID, &name, &description, &category, &content, &variablesJSON, &createdAt, &updatedAt); err != nil {
			continue
		}

		var variables []string
		if err := json.Unmarshal([]byte(variablesJSON), &variables); err != nil {
			variables = ExtractVariables(content)
		}

		templates = append(templates, map[string]any{
			"id":          id,
			"name":        name,
			"description": description,
			"category":    category,
			"content":     content,
			"variables":   variables,
			"isPreset":    false,
			"createdAt":   createdAt,
			"updatedAt":   updatedAt,
		})
	}

	return templates, nil
}

func ExtractVariables(content string) []string {
	var vars []string
	seen := map[string]bool{}
	start := 0
	for {
		idx := indexOfDoubleBrace(content, start)
		if idx == -1 {
			break
		}
		end := indexOfDoubleBraceClose(content, idx+2)
		if end == -1 {
			break
		}
		name := content[idx+2 : end]
		if name != "" && !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
		start = end + 2
	}
	return vars
}

func indexOfDoubleBrace(s string, start int) int {
	for i := start; i <= len(s)-2; i++ {
		if s[i] == '{' && s[i+1] == '{' {
			return i
		}
	}
	return -1
}

func indexOfDoubleBraceClose(s string, start int) int {
	for i := start; i <= len(s)-2; i++ {
		if s[i] == '}' && s[i+1] == '}' {
			return i
		}
	}
	return -1
}
