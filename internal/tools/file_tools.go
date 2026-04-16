package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/instructkr/smartclaw/internal/errors"
	"github.com/instructkr/smartclaw/internal/patch"
)

const maxFileReadSize = 50 * 1024 * 1024 // 50MB

var allowedDirs []string

func SetAllowedDirs(dirs []string) {
	allowedDirs = dirs
}

func isPathAllowed(path string) bool {
	if len(allowedDirs) == 0 {
		return true
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir+string(os.PathSeparator)) || absPath == absDir {
			return true
		}
	}
	return false
}

type ReadFileTool struct{ BaseTool }

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read a text file from the workspace" }

func (t *ReadFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":   map[string]any{"type": "string", "description": "The file path to read"},
			"offset": map[string]any{"type": "integer", "description": "Line offset (1-indexed)"},
			"limit":  map[string]any{"type": "integer", "description": "Maximum lines to read"},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return nil, ErrRequiredField("path")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	if !isPathAllowed(absPath) {
		return nil, apperrors.New("PATH_DENIED", "access denied: path outside allowed directories",
			apperrors.WithCategory(apperrors.CategorySecurity))
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}
	if info.Size() > maxFileReadSize {
		return nil, &Error{Code: "FILE_TOO_LARGE", Message: fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxFileReadSize)}
	}

	content, err := os.ReadFile(absPath)
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

	return map[string]any{
		"content": strings.Join(result, "\n"),
		"path":    absPath,
		"lines":   len(lines),
	}, nil
}

type WriteFileTool struct{ BaseTool }

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write a text file in the workspace" }

func (t *WriteFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "The file path to write"},
			"content": map[string]any{"type": "string", "description": "The content to write"},
		},
		"required": []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	content, _ := input["content"].(string)

	if path == "" {
		return nil, ErrRequiredField("path")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	if !isPathAllowed(absPath) {
		return nil, apperrors.New("PATH_DENIED", "access denied: path outside allowed directories",
			apperrors.WithCategory(apperrors.CategorySecurity))
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, &Error{Code: "MKDIR_ERROR", Message: err.Error()}
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return nil, &Error{Code: "WRITE_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"path":    absPath,
		"written": len(content),
	}, nil
}

type EditFileTool struct{ BaseTool }

func (t *EditFileTool) Name() string        { return "edit_file" }
func (t *EditFileTool) Description() string { return "Edit a file using string replacement" }

func (t *EditFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":        map[string]any{"type": "string"},
			"old_string":  map[string]any{"type": "string"},
			"new_string":  map[string]any{"type": "string"},
			"replace_all": map[string]any{"type": "boolean"},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, input map[string]any) (any, error) {
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

	if !isPathAllowed(absPath) {
		return nil, apperrors.New("PATH_DENIED", "access denied: path outside allowed directories",
			apperrors.WithCategory(apperrors.CategorySecurity))
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}
	if info.Size() > maxFileReadSize {
		return nil, &Error{Code: "FILE_TOO_LARGE", Message: fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxFileReadSize)}
	}

	content, err := os.ReadFile(absPath)
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

	if err := os.WriteFile(absPath, []byte(contentStr), 0644); err != nil {
		return nil, &Error{Code: "WRITE_ERROR", Message: err.Error()}
	}

	return map[string]any{
		"path":     absPath,
		"replaced": replaced,
	}, nil
}

type GlobTool struct{ BaseTool }

func (t *GlobTool) Name() string        { return "glob" }
func (t *GlobTool) Description() string { return "Search for files by pattern" }

func (t *GlobTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string"},
			"path":    map[string]any{"type": "string"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) Execute(ctx context.Context, input map[string]any) (any, error) {
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

	return map[string]any{
		"files": matches,
		"count": len(matches),
	}, nil
}

type GrepTool struct{ BaseTool }

func (t *GrepTool) Name() string        { return "grep" }
func (t *GrepTool) Description() string { return "Search for text in files using regex" }

func (t *GrepTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern":     map[string]any{"type": "string", "description": "Regex pattern to search"},
			"path":        map[string]any{"type": "string", "description": "Base path to search"},
			"include":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "File patterns to include"},
			"exclude":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "File patterns to exclude"},
			"context":     map[string]any{"type": "integer", "description": "Number of context lines"},
			"ignore_case": map[string]any{"type": "boolean", "description": "Case insensitive search"},
			"output_mode": map[string]any{"type": "string", "enum": []string{"content", "files_with_matches", "count"}, "description": "Output format"},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Execute(ctx context.Context, input map[string]any) (any, error) {
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
	includes, _ := input["include"].([]any)
	excludes, _ := input["exclude"].([]any)

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
		return map[string]any{
			"count": total,
			"files": fileCounts,
		}, nil
	case "files_with_matches":
		var files []string
		for f := range fileCounts {
			files = append(files, f)
		}
		return map[string]any{
			"files": files,
			"count": len(files),
		}, nil
	default:
		return map[string]any{
			"matches": matches,
			"count":   len(matches),
		}, nil
	}
}

type LineEditTool struct{ BaseTool }

func (t *LineEditTool) Name() string { return "line_edit" }

func (t *LineEditTool) Description() string {
	return "Edit a file by replacing a range of lines (1-based). Creates a .bak backup. Use preview_file first to see line numbers."
}

func (t *LineEditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "File path to edit"},
			"start":   map[string]any{"type": "integer", "description": "Start line (1-based, inclusive)"},
			"end":     map[string]any{"type": "integer", "description": "End line (1-based, inclusive). Use same as start for single line edit."},
			"content": map[string]any{"type": "string", "description": "New content for the line range (may be multi-line)"},
			"dry_run": map[string]any{"type": "boolean", "description": "If true, preview changes without applying"},
		},
		"required": []string{"path", "start", "end", "content"},
	}
}

func (t *LineEditTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return nil, ErrRequiredField("path")
	}

	start, _ := input["start"].(int)
	end, _ := input["end"].(int)
	content, _ := input["content"].(string)
	dryRun, _ := input["dry_run"].(bool)

	if start < 1 {
		return nil, &Error{Code: "INVALID_RANGE", Message: "start must be >= 1"}
	}
	if end < start {
		return nil, &Error{Code: "INVALID_RANGE", Message: "end must be >= start"}
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	if !isPathAllowed(absPath) {
		return nil, apperrors.New("PATH_DENIED", "access denied: path outside allowed directories",
			apperrors.WithCategory(apperrors.CategorySecurity))
	}

	edit := patch.Edit{
		File:    absPath,
		Start:   start,
		End:     end,
		Content: content,
	}

	if dryRun {
		preview, err := patch.PreviewEdit(absPath, edit)
		if err != nil {
			return nil, &Error{Code: "PREVIEW_ERROR", Message: err.Error()}
		}

		return map[string]any{
			"dry_run": true,
			"path":    absPath,
			"diff":    preview,
		}, nil
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}
	if info.Size() > maxFileReadSize {
		return nil, &Error{Code: "FILE_TOO_LARGE", Message: fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxFileReadSize)}
	}

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	bakPath := absPath + ".bak"
	if err := os.WriteFile(bakPath, raw, 0644); err != nil {
		return nil, &Error{Code: "BACKUP_ERROR", Message: fmt.Sprintf("failed to create backup: %v", err)}
	}

	result, err := patch.ApplyEdit(absPath, edit)
	if err != nil {
		return nil, &Error{Code: "APPLY_ERROR", Message: err.Error()}
	}

	var rollbackDiff string
	if result.Diff != nil {
		reversed := patch.Reverse(result.Diff)
		rollbackDiff = string(patch.Diff(reversed.OldPath, reversed.NewPath, raw, nil))
	}

	added, removed := patch.Stats(result.Diff)

	return map[string]any{
		"path":          absPath,
		"backup":        bakPath,
		"diff":          result.Preview,
		"lines_added":   added,
		"lines_removed": removed,
		"rollback":      rollbackDiff,
	}, nil
}

// PreviewFileTool shows a file with line numbers for a specified range.
type PreviewFileTool struct{ BaseTool }

func (t *PreviewFileTool) Name() string { return "preview_file" }

func (t *PreviewFileTool) Description() string {
	return "Preview a file with line numbers for the specified range. Useful before using line_edit to identify exact line numbers."
}

func (t *PreviewFileTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":          map[string]any{"type": "string", "description": "File path to preview"},
			"start":         map[string]any{"type": "integer", "description": "Start line (1-based, optional)"},
			"end":           map[string]any{"type": "integer", "description": "End line (1-based, optional)"},
			"context_lines": map[string]any{"type": "integer", "description": "Lines of context around range (default 3)"},
		},
		"required": []string{"path"},
	}
}

func (t *PreviewFileTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return nil, ErrRequiredField("path")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}

	if !isPathAllowed(absPath) {
		return nil, apperrors.New("PATH_DENIED", "access denied: path outside allowed directories",
			apperrors.WithCategory(apperrors.CategorySecurity))
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, &Error{Code: "PATH_ERROR", Message: err.Error()}
	}
	if info.Size() > maxFileReadSize {
		return nil, &Error{Code: "FILE_TOO_LARGE", Message: fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), maxFileReadSize)}
	}

	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, &Error{Code: "READ_ERROR", Message: err.Error()}
	}

	lines := strings.Split(string(raw), "\n")
	totalLines := len(lines)
	if totalLines > 0 && lines[totalLines-1] == "" {
		totalLines--
	}

	start := 1
	end := totalLines

	if s, ok := input["start"].(int); ok && s > 0 {
		start = s
	}
	if e, ok := input["end"].(int); ok && e > 0 {
		end = e
	}

	contextLines := 3
	if c, ok := input["context_lines"].(int); ok && c >= 0 {
		contextLines = c
	}

	if _, hasStart := input["start"]; hasStart {
		start = start - contextLines
		if start < 1 {
			start = 1
		}
	}
	if _, hasEnd := input["end"]; hasEnd {
		end = end + contextLines
		if end > totalLines {
			end = totalLines
		}
	}

	if start < 1 {
		start = 1
	}
	if end > totalLines {
		end = totalLines
	}

	var result []string
	for i := start; i <= end; i++ {
		result = append(result, fmt.Sprintf("%d: %s", i, lines[i-1]))
	}

	info, _ = os.Stat(absPath)
	var modTime string
	if info != nil {
		modTime = info.ModTime().Format(time.RFC3339)
	}

	return map[string]any{
		"content":     strings.Join(result, "\n"),
		"path":        absPath,
		"total_lines": totalLines,
		"showing":     fmt.Sprintf("%d-%d", start, end),
		"modified":    modTime,
	}, nil
}
