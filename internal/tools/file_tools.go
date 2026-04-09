package tools

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read a text file from the workspace" }

func (t *ReadFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":   map[string]interface{}{"type": "string", "description": "The file path to read"},
			"offset": map[string]interface{}{"type": "integer", "description": "Line offset (1-indexed)"},
			"limit":  map[string]interface{}{"type": "integer", "description": "Maximum lines to read"},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return nil, ErrRequiredField("path")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	content, err := ioutil.ReadFile(absPath)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	lines := strings.Split(string(content), "\n")

	offset := 1
	if o, ok := input["offset"].(int); ok && o > 0 {
		offset = o
	}

	limit := 2000
	if l, ok := input["limit"].(int); ok && l > 0 {
		limit = l
	}

	start := offset - 1
	if start > len(lines) {
		start = len(lines)
	}
	end := start + limit
	if end > len(lines) {
		end = len(lines)
	}

	result := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		result = append(result, fmt.Sprintf("%d: %s", i+1, lines[i]))
	}

	return map[string]interface{}{
		"content": strings.Join(result, "\n"),
		"path":    absPath,
		"lines":   len(lines),
	}, nil
}

type WriteFileTool struct{}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write a text file in the workspace" }

func (t *WriteFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string", "description": "The file path to write"},
			"content": map[string]interface{}{"type": "string", "description": "The content to write"},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	path, _ := input["path"].(string)
	content, _ := input["content"].(string)

	if path == "" {
		return nil, ErrRequiredField("path")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, &Error{Code: "MKDIR_ERROR", Message: err.Error()}
	}

	if err := ioutil.WriteFile(absPath, []byte(content), 0644); err != nil {
		return nil, &Error{Code: "WRITE_ERROR", Message: err.Error()}
	}

	return map[string]interface{}{
		"path":    absPath,
		"written": len(content),
	}, nil
}

type EditFileTool struct{}

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string { return "Edit a file using string replacement" }

func (t *EditFileTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":        map[string]interface{}{"type": "string"},
			"old_string":  map[string]interface{}{"type": "string"},
			"new_string":  map[string]interface{}{"type": "string"},
			"replace_all": map[string]interface{}{"type": "boolean"},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	path, _ := input["path"].(string)
	oldStr, _ := input["old_string"].(string)
	newStr, _ := input["new_string"].(string)
	replaceAll, _ := input["replace_all"].(bool)

	if path == "" {
		return nil, ErrRequiredField("path")
	}
	if oldStr == "" {
		return nil, ErrRequiredField("old_string")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	content, err := ioutil.ReadFile(absPath)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	contentStr := string(content)
	replaced := 0

	if replaceAll {
		count := strings.Count(contentStr, oldStr)
		contentStr = strings.ReplaceAll(contentStr, oldStr, newStr)
		replaced = count
	} else {
		idx := strings.Index(contentStr, oldStr)
		if idx == -1 {
			return nil, &Error{Code: "NOT_FOUND", Message: "old_string not found in file"}
		}
		contentStr = strings.Replace(contentStr, oldStr, newStr, 1)
		replaced = 1
	}

	if err := ioutil.WriteFile(absPath, []byte(contentStr), 0644); err != nil {
		return nil, &Error{Code: "WRITE_ERROR", Message: err.Error()}
	}

	return map[string]interface{}{
		"path":     absPath,
		"replaced": replaced,
	}, nil
}

type GlobTool struct{}

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Search for files by pattern" }

func (t *GlobTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern": map[string]interface{}{"type": "string"},
			"path":    map[string]interface{}{"type": "string"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return nil, ErrRequiredField("pattern")
	}

	basePath, _ := input["path"].(string)
	if basePath == "" {
		basePath = "."
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	fullPattern := filepath.Join(absBase, pattern)
	matches, err := filepath.Glob(fullPattern)
	if err != nil {
		return nil, &Error{Code: "GLOB_ERROR", Message: err.Error()}
	}

	return map[string]interface{}{
		"files": matches,
		"count": len(matches),
	}, nil
}

type GrepTool struct{}

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search for text in files using regex" }

func (t *GrepTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pattern":     map[string]interface{}{"type": "string", "description": "Regex pattern to search"},
			"path":        map[string]interface{}{"type": "string", "description": "Base path to search"},
			"include":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "File patterns to include"},
			"exclude":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "File patterns to exclude"},
			"context":     map[string]interface{}{"type": "integer", "description": "Number of context lines"},
			"ignore_case": map[string]interface{}{"type": "boolean", "description": "Case insensitive search"},
			"output_mode": map[string]interface{}{"type": "string", "enum": []string{"content", "files_with_matches", "count"}, "description": "Output format"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Execute(ctx context.Context, input map[string]interface{}) (interface{}, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return nil, ErrRequiredField("pattern")
	}

	basePath, _ := input["path"].(string)
	if basePath == "" {
		basePath = "."
	}

	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	ignoreCase, _ := input["ignore_case"].(bool)
	contextLines, _ := input["context"].(int)
	outputMode, _ := input["output_mode"].(string)
	if outputMode == "" {
		outputMode = "content"
	}

	// Build regex
	var regex *regexp.Regexp
	if ignoreCase {
		regex, err = regexp.Compile("(?i)" + pattern)
	} else {
		regex, err = regexp.Compile(pattern)
	}
	if err != nil {
		return nil, &Error{Code: "INVALID_PATTERN", Message: err.Error()}
	}

	// Handle include/exclude
	includes, _ := input["include"].([]interface{})
	excludes, _ := input["exclude"].([]interface{})

	var matches []GrepMatch
	fileCounts := make(map[string]int)

	err = filepath.Walk(absBase, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories
		if info.IsDir() {
			// Check for ignored directories
			ignoredDirs := []string{".git", "node_modules", "vendor", ".svn", "__pycache__"}
			for _, dir := range ignoredDirs {
				if info.Name() == dir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check include patterns
		if len(includes) > 0 {
			matched := false
			for _, inc := range includes {
				if incStr, ok := inc.(string); ok {
					if matched, _ = filepath.Match(incStr, info.Name()); matched {
						break
					}
				}
			}
			if !matched {
				return nil
			}
		}

		// Check exclude patterns
		if len(excludes) > 0 {
			for _, exc := range excludes {
				if excStr, ok := exc.(string); ok {
					if matched, _ := filepath.Match(excStr, info.Name()); matched {
						return nil
					}
				}
			}
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			if regex.MatchString(line) {
				match := GrepMatch{
					File:    path,
					Line:    lineNum,
					Content: line,
				}

				if outputMode == "content" {
					// Add context lines
					if contextLines > 0 {
						// For simplicity, just add the match
						matches = append(matches, match)
					} else {
						matches = append(matches, match)
					}
				}
				fileCounts[path]++
			}
		}
		return nil
	})

	if err != nil {
		return nil, &Error{Code: "WALK_ERROR", Message: err.Error()}
	}

	// Format output based on mode
	switch outputMode {
	case "count":
		var total int
		for _, count := range fileCounts {
			total += count
		}
		return map[string]interface{}{
			"count": total,
			"files": fileCounts,
		}, nil
	case "files_with_matches":
		var files []string
		for f := range fileCounts {
			files = append(files, f)
		}
		return map[string]interface{}{
			"files": files,
			"count": len(files),
		}, nil
	default:
		return map[string]interface{}{
			"matches": matches,
			"count":   len(matches),
		}, nil
	}
}
