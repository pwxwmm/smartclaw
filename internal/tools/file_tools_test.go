package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileToolReadExisting(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &ReadFileTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result, got %T", result)
	}

	if resultMap["path"] == "" {
		t.Error("Expected path in result")
	}

	lines, ok := resultMap["lines"].(int)
	if !ok || lines != 6 {
		t.Errorf("Expected 6 lines, got %v", resultMap["lines"])
	}

	contentStr, _ := resultMap["content"].(string)
	if !strings.Contains(contentStr, "line1") || !strings.Contains(contentStr, "line5") {
		t.Errorf("Expected content to contain line1 and line5, got %q", contentStr)
	}
}

func TestReadFileToolWithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &ReadFileTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":   testFile,
		"offset": 3,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	contentStr, _ := resultMap["content"].(string)
	if strings.Contains(contentStr, "line1") {
		t.Error("Offset=3 should skip first 2 lines")
	}
	if !strings.Contains(contentStr, "line3") {
		t.Error("Offset=3 should start from line3")
	}
}

func TestReadFileToolWithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := &ReadFileTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":  testFile,
		"limit": 2,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	contentStr, _ := resultMap["content"].(string)
	if !strings.Contains(contentStr, "line1") {
		t.Error("Should contain line1")
	}
	if strings.Contains(contentStr, "line3") {
		t.Error("Limit=2 should not include line3")
	}
}

func TestReadFileToolNonExistent(t *testing.T) {
	tool := &ReadFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path": "/nonexistent/file/that/does/not/exist.txt",
	})
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestReadFileToolMissingPath(t *testing.T) {
	tool := &ReadFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing path")
	}
}

func TestReadFileToolEmptyPath(t *testing.T) {
	tool := &ReadFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path": "",
	})
	if err == nil {
		t.Error("Expected error for empty path")
	}
}

func TestReadFileToolSchema(t *testing.T) {
	tool := &ReadFileTool{}
	if tool.Name() != "read_file" {
		t.Errorf("Expected name 'read_file', got '%s'", tool.Name())
	}
	schema := tool.InputSchema()
	if schema == nil {
		t.Error("InputSchema should not be nil")
	}
}

func TestWriteFileToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "output.txt")

	tool := &WriteFileTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":    testFile,
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["written"].(int) != len("hello world") {
		t.Errorf("Expected written=%d, got %d", len("hello world"), resultMap["written"])
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("Expected 'hello world', got %q", string(data))
	}
}

func TestWriteFileToolCreatesDirs(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sub", "dir", "output.txt")

	tool := &WriteFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":    testFile,
		"content": "nested",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(data) != "nested" {
		t.Errorf("Expected 'nested', got %q", string(data))
	}
}

func TestWriteFileToolOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "overwrite.txt")

	os.WriteFile(testFile, []byte("old content"), 0644)

	tool := &WriteFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":    testFile,
		"content": "new content",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	data, _ := os.ReadFile(testFile)
	if string(data) != "new content" {
		t.Errorf("Expected 'new content', got %q", string(data))
	}
}

func TestWriteFileToolMissingPath(t *testing.T) {
	tool := &WriteFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"content": "no path",
	})
	if err == nil {
		t.Error("Expected error for missing path")
	}
}

func TestWriteFileToolSchema(t *testing.T) {
	tool := &WriteFileTool{}
	if tool.Name() != "write_file" {
		t.Errorf("Expected name 'write_file', got '%s'", tool.Name())
	}
}

func TestEditFileToolSimpleReplace(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	tool := &EditFileTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":       testFile,
		"old_string": "hello",
		"new_string": "goodbye",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["replaced"].(int) != 1 {
		t.Errorf("Expected replaced=1, got %d", resultMap["replaced"])
	}

	data, _ := os.ReadFile(testFile)
	if string(data) != "goodbye world" {
		t.Errorf("Expected 'goodbye world', got %q", string(data))
	}
}

func TestEditFileToolReplaceAll(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(testFile, []byte("aaa bbb aaa ccc aaa"), 0644)

	tool := &EditFileTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":        testFile,
		"old_string":  "aaa",
		"new_string":  "xxx",
		"replace_all": true,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["replaced"].(int) != 3 {
		t.Errorf("Expected replaced=3, got %d", resultMap["replaced"])
	}

	data, _ := os.ReadFile(testFile)
	if string(data) != "xxx bbb xxx ccc xxx" {
		t.Errorf("Expected 'xxx bbb xxx ccc xxx', got %q", string(data))
	}
}

func TestEditFileToolNoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "edit.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	tool := &EditFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":       testFile,
		"old_string": "nonexistent",
		"new_string": "replacement",
	})
	if err == nil {
		t.Error("Expected error when old_string not found")
	}
}

func TestEditFileToolEmptyOldString(t *testing.T) {
	tool := &EditFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":       "/tmp/test.txt",
		"old_string": "",
		"new_string": "replacement",
	})
	if err == nil {
		t.Error("Expected error for empty old_string")
	}
}

func TestEditFileToolMissingPath(t *testing.T) {
	tool := &EditFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"old_string": "test",
		"new_string": "replacement",
	})
	if err == nil {
		t.Error("Expected error for missing path")
	}
}

func TestEditFileToolSchema(t *testing.T) {
	tool := &EditFileTool{}
	if tool.Name() != "edit_file" {
		t.Errorf("Expected name 'edit_file', got '%s'", tool.Name())
	}
}

func TestGlobToolFindFiles(t *testing.T) {
	tmpDir := t.TempDir()
	files := []string{"test1.txt", "test2.txt", "other.log"}
	for _, f := range files {
		os.WriteFile(filepath.Join(tmpDir, f), []byte("test"), 0644)
	}

	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.txt",
		"path":    tmpDir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	matches := resultMap["files"].([]string)
	if len(matches) != 2 {
		t.Errorf("Expected 2 .txt files, got %d", len(matches))
	}
}

func TestGlobToolNoMatches(t *testing.T) {
	tmpDir := t.TempDir()

	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.xyz",
		"path":    tmpDir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	matches := resultMap["files"].([]string)
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches, got %d", len(matches))
	}
	if resultMap["count"].(int) != 0 {
		t.Errorf("Expected count=0, got %d", resultMap["count"])
	}
}

func TestGlobToolInvalidPattern(t *testing.T) {
	tool := &GlobTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "[invalid",
	})
	if err == nil {
		t.Error("Expected error for invalid glob pattern")
	}
}

func TestGlobToolMissingPattern(t *testing.T) {
	tool := &GlobTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing pattern")
	}
}

func TestGlobToolDefaultPath(t *testing.T) {
	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.go",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resultMap := result.(map[string]any)
	if resultMap["count"].(int) == 0 {
		t.Error("Expected at least some .go files in current dir")
	}
}

func TestGrepToolSearchPattern(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1 hello\nline2 world\nline3 hello world\n"
	os.WriteFile(testFile, []byte(content), 0644)

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    tmpDir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	count := resultMap["count"].(int)
	if count != 2 {
		t.Errorf("Expected 2 matches, got %d", count)
	}
}

func TestGrepToolNoMatches(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "zzzznotfound",
		"path":    tmpDir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["count"].(int) != 0 {
		t.Errorf("Expected 0 matches, got %d", resultMap["count"])
	}
}

func TestGrepToolRegex(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "test123\ntest456\nnope\n"
	os.WriteFile(testFile, []byte(content), 0644)

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": `test\d+`,
		"path":    tmpDir,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["count"].(int) != 2 {
		t.Errorf("Expected 2 regex matches, got %d", resultMap["count"])
	}
}

func TestGrepToolCaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello World\nhello world\nHELLO WORLD\n"
	os.WriteFile(testFile, []byte(content), 0644)

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern":     "hello",
		"path":        tmpDir,
		"ignore_case": true,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["count"].(int) != 3 {
		t.Errorf("Expected 3 case-insensitive matches, got %d", resultMap["count"])
	}
}

func TestGrepToolInvalidPattern(t *testing.T) {
	tool := &GrepTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "[invalid",
	})
	if err == nil {
		t.Error("Expected error for invalid regex pattern")
	}
}

func TestGrepToolMissingPattern(t *testing.T) {
	tool := &GrepTool{}
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("Expected error for missing pattern")
	}
}

func TestGrepToolOutputModeFilesWithMatches(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("no match here"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("hello again"), 0644)

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern":      "hello",
		"path":         tmpDir,
		"output_mode":  "files_with_matches",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	count := resultMap["count"].(int)
	if count != 2 {
		t.Errorf("Expected 2 files with matches, got %d", count)
	}
}

func TestGrepToolOutputModeCount(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("hello\nhello\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("hello\n"), 0644)

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern":      "hello",
		"path":         tmpDir,
		"output_mode":  "count",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	count := resultMap["count"].(int)
	if count != 3 {
		t.Errorf("Expected 3 total matches, got %d", count)
	}
}

func TestGrepToolIncludeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("hello"), 0644)

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    tmpDir,
		"include": []any{"*.go"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	count := resultMap["count"].(int)
	if count != 1 {
		t.Errorf("Expected 1 match in .go files only, got %d", count)
	}
}

func TestGrepToolExcludeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("hello"), 0644)

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    tmpDir,
		"exclude": []any{"*.txt"},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	count := resultMap["count"].(int)
	if count != 1 {
		t.Errorf("Expected 1 match excluding .txt files, got %d", count)
	}
}

func TestIsPathAllowedNoDirs(t *testing.T) {
	if !isPathAllowed("/any/path") {
		t.Error("With no allowedDirs, all paths should be allowed")
	}
}

func TestIsPathAllowedWithDirs(t *testing.T) {
	original := allowedDirs
	defer func() { allowedDirs = original }()

	tmpDir := t.TempDir()
	allowedDirs = []string{tmpDir}

	if !isPathAllowed(filepath.Join(tmpDir, "file.txt")) {
		t.Error("Path within allowed dir should be allowed")
	}
	if isPathAllowed("/other/path/file.txt") {
		t.Error("Path outside allowed dir should not be allowed")
	}
}

func TestPreviewFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "preview.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(testFile, []byte(content), 0644)

	tool := &PreviewFileTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": testFile,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["total_lines"] == nil {
		t.Error("Expected total_lines in result")
	}
}

func TestPreviewFileToolWithRange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "preview.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	os.WriteFile(testFile, []byte(content), 0644)

	tool := &PreviewFileTool{}
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":  testFile,
		"start": 2,
		"end":   3,
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resultMap := result.(map[string]any)
	contentStr, _ := resultMap["content"].(string)
	if !strings.Contains(contentStr, "line2") {
		t.Error("Should contain line2")
	}
}

func TestPreviewFileToolNonExistent(t *testing.T) {
	tool := &PreviewFileTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"path": "/nonexistent/preview.txt",
	})
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestDiffEditToolMissingFilePath(t *testing.T) {
	tool := &DiffEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"diff_content": "some content",
	})
	if err == nil {
		t.Error("Expected error for missing file_path")
	}
}

func TestDiffEditToolMissingDiffContent(t *testing.T) {
	tool := &DiffEditTool{}
	_, err := tool.Execute(context.Background(), map[string]any{
		"file_path": "/tmp/test.txt",
	})
	if err == nil {
		t.Error("Expected error for missing diff_content")
	}
}
