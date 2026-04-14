package tools

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ASTGrepTool struct{ BaseTool }

func (t *ASTGrepTool) Name() string		{ return "ast_grep" }
func (t *ASTGrepTool) Description() string	{ return "Search code using AST patterns" }

func (t *ASTGrepTool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":		"string",
				"description":	"AST pattern to search for",
			},
			"lang": map[string]any{
				"type":		"string",
				"description":	"Language (go, typescript, python, etc)",
			},
			"paths": map[string]any{
				"type":		"array",
				"items":	map[string]any{"type": "string"},
				"description":	"Paths to search",
			},
			"rewrite": map[string]any{
				"type":		"string",
				"description":	"Replacement pattern",
			},
		},
		"required":	[]string{"pattern", "lang"},
	}
}

func (t *ASTGrepTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	pattern, _ := input["pattern"].(string)
	lang, _ := input["lang"].(string)
	rewrite, _ := input["rewrite"].(string)

	if pattern == "" {
		return nil, ErrRequiredField("pattern")
	}
	if lang == "" {
		return nil, ErrRequiredField("lang")
	}

	paths := []string{"."}
	if p, ok := input["paths"].([]any); ok {
		for _, path := range p {
			if s, ok := path.(string); ok {
				paths = append(paths, s)
			}
		}
	}

	return t.searchAST(pattern, lang, paths, rewrite)
}

func (t *ASTGrepTool) searchAST(pattern, lang string, paths []string, rewrite string) (any, error) {
	extensions := t.getExtensionsForLang(lang)
	if len(extensions) == 0 {
		return nil, &Error{Code: "UNSUPPORTED_LANG", Message: "Unsupported language: " + lang}
	}

	matches := make([]map[string]any, 0)

	for _, basePath := range paths {
		err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			for _, allowedExt := range extensions {
				if ext == allowedExt {
					fileMatches := t.searchFile(path, pattern, rewrite)
					matches = append(matches, fileMatches...)
					break
				}
			}
			return nil
		})
		if err != nil {
			continue
		}
	}

	return map[string]any{
		"matches":	matches,
		"count":	len(matches),
		"lang":		lang,
	}, nil
}

func (t *ASTGrepTool) getExtensionsForLang(lang string) []string {
	switch strings.ToLower(lang) {
	case "go":
		return []string{".go"}
	case "typescript", "ts":
		return []string{".ts", ".tsx"}
	case "javascript", "js":
		return []string{".js", ".jsx"}
	case "python", "py":
		return []string{".py"}
	case "rust", "rs":
		return []string{".rs"}
	case "java":
		return []string{".java"}
	case "c":
		return []string{".c", ".h"}
	case "cpp", "c++":
		return []string{".cpp", ".cc", ".cxx", ".hpp", ".h"}
	default:
		return []string{}
	}
}

func (t *ASTGrepTool) searchFile(path, pattern, rewrite string) []map[string]any {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(content), "\n")
	var matches []map[string]any

	regexPattern := t.patternToRegex(pattern)

	for i, line := range lines {
		if regexPattern.MatchString(line) {
			match := map[string]any{
				"file":		path,
				"line":		i + 1,
				"content":	strings.TrimSpace(line),
			}

			if rewrite != "" {
				newLine := regexPattern.ReplaceAllString(line, rewrite)
				match["replacement"] = strings.TrimSpace(newLine)
			}

			matches = append(matches, match)
		}
	}

	return matches
}

func (t *ASTGrepTool) patternToRegex(pattern string) *regexp.Regexp {
	pattern = strings.ReplaceAll(pattern, "$$$", ".*")
	pattern = strings.ReplaceAll(pattern, "$VAR", `\w+`)
	pattern = regexp.QuoteMeta(pattern)
	pattern = strings.ReplaceAll(pattern, `\.\*`, ".*")
	pattern = strings.ReplaceAll(pattern, `\\w\+`, `\w+`)

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return regexp.MustCompile(regexp.QuoteMeta(pattern))
	}
	return regex
}

type CodeSearchTool struct{ BaseTool }

func (t *CodeSearchTool) Name() string		{ return "code_search" }
func (t *CodeSearchTool) Description() string	{ return "Search code with semantic understanding" }

func (t *CodeSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type":	"object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":		"string",
				"description":	"Search query",
			},
			"type": map[string]any{
				"type":		"string",
				"description":	"Type of code to find (function, class, variable, etc)",
			},
			"path": map[string]any{
				"type":		"string",
				"description":	"Base path to search",
			},
		},
		"required":	[]string{"query"},
	}
}

func (t *CodeSearchTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	query, _ := input["query"].(string)
	if query == "" {
		return nil, ErrRequiredField("query")
	}

	codeType, _ := input["type"].(string)
	basePath, _ := input["path"].(string)
	if basePath == "" {
		basePath = "."
	}

	results := make([]map[string]any, 0)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !t.isCodeFile(path) {
			return nil
		}

		fileResults := t.searchInFile(path, query, codeType)
		results = append(results, fileResults...)

		return nil
	})

	if err != nil {
		return nil, &Error{Code: "SEARCH_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"results":	results,
		"count":	len(results),
		"query":	query,
	}, nil
}

func (t *CodeSearchTool) isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExts := map[string]bool{
		".go":	true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true,
		".py":	true, ".rs": true, ".java": true, ".c": true, ".cpp": true,
		".h":	true, ".hpp": true, ".cs": true, ".rb": true, ".php": true,
	}
	return codeExts[ext]
}

func (t *CodeSearchTool) searchInFile(path, query, codeType string) []map[string]any {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var results []map[string]any
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
			if codeType == "" || t.matchesType(line, codeType) {
				results = append(results, map[string]any{
					"file":		path,
					"line":		i + 1,
					"content":	strings.TrimSpace(line),
					"type":		t.detectType(line),
				})
			}
		}
	}

	return results
}

func (t *CodeSearchTool) matchesType(line, codeType string) bool {
	line = strings.ToLower(line)
	codeType = strings.ToLower(codeType)

	switch codeType {
	case "function", "func", "method":
		return strings.Contains(line, "func ") || strings.Contains(line, "function ") || strings.Contains(line, "def ")
	case "class", "struct", "interface":
		return strings.Contains(line, "class ") || strings.Contains(line, "struct ") || strings.Contains(line, "interface ")
	case "variable", "var", "const":
		return strings.Contains(line, "var ") || strings.Contains(line, "const ") || strings.Contains(line, "let ")
	default:
		return true
	}
}

func (t *CodeSearchTool) detectType(line string) string {
	line = strings.ToLower(line)
	switch {
	case strings.Contains(line, "func ") || strings.Contains(line, "function ") || strings.Contains(line, "def "):
		return "function"
	case strings.Contains(line, "class ") || strings.Contains(line, "struct ") || strings.Contains(line, "interface "):
		return "class"
	case strings.Contains(line, "var ") || strings.Contains(line, "const ") || strings.Contains(line, "let "):
		return "variable"
	default:
		return "code"
	}
}

func init() {
}
