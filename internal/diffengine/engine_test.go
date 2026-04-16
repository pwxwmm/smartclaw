package diffengine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDiffBlocksBasic(t *testing.T) {
	input := "--- main.go\n<<<<<<< SEARCH\nfunc old() {\n=======\nfunc new() {\n>>>>>>> REPLACE"

	blocks, err := ParseDiffBlocks(input)
	if err != nil {
		t.Fatalf("ParseDiffBlocks error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].FilePath != "main.go" {
		t.Errorf("FilePath = %q, want %q", blocks[0].FilePath, "main.go")
	}
	if len(blocks[0].SearchLines) != 1 || blocks[0].SearchLines[0] != "func old() {" {
		t.Errorf("SearchLines = %v, want [func old() {]", blocks[0].SearchLines)
	}
	if len(blocks[0].ReplaceLines) != 1 || blocks[0].ReplaceLines[0] != "func new() {" {
		t.Errorf("ReplaceLines = %v, want [func new() {]", blocks[0].ReplaceLines)
	}
}

func TestParseDiffBlocksAiderStyle(t *testing.T) {
	input := "<<<<<<< HEAD\nold line\n=======\nnew line\n>>>>>>> updated"

	blocks, err := ParseDiffBlocks(input)
	if err != nil {
		t.Fatalf("ParseDiffBlocks error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].SearchLines[0] != "old line" {
		t.Errorf("SearchLines[0] = %q, want %q", blocks[0].SearchLines[0], "old line")
	}
	if blocks[0].ReplaceLines[0] != "new line" {
		t.Errorf("ReplaceLines[0] = %q, want %q", blocks[0].ReplaceLines[0], "new line")
	}
}

func TestParseDiffBlocksMultiple(t *testing.T) {
	input := "--- file.go\n<<<<<<< SEARCH\nline1\n=======\nline1a\n>>>>>>> REPLACE\n\n<<<<<<< SEARCH\nline3\n=======\nline3a\n>>>>>>> REPLACE"

	blocks, err := ParseDiffBlocks(input)
	if err != nil {
		t.Fatalf("ParseDiffBlocks error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].FilePath != "file.go" {
		t.Errorf("block[0].FilePath = %q", blocks[0].FilePath)
	}
	if blocks[1].FilePath != "file.go" {
		t.Errorf("block[1].FilePath = %q", blocks[1].FilePath)
	}
}

func TestParseDiffBlocksMultiline(t *testing.T) {
	input := "--- app.go\n<<<<<<< SEARCH\nfunc hello() {\n\tfmt.Println(\"hi\")\n}\n=======\nfunc hello() {\n\tfmt.Println(\"hello, world\")\n}\n>>>>>>> REPLACE"

	blocks, err := ParseDiffBlocks(input)
	if err != nil {
		t.Fatalf("ParseDiffBlocks error: %v", err)
	}
	if len(blocks[0].SearchLines) != 3 {
		t.Errorf("expected 3 search lines, got %d", len(blocks[0].SearchLines))
	}
	if len(blocks[0].ReplaceLines) != 3 {
		t.Errorf("expected 3 replace lines, got %d", len(blocks[0].ReplaceLines))
	}
}

func TestParseDiffBlocksNoBlocks(t *testing.T) {
	_, err := ParseDiffBlocks("just some text")
	if err == nil {
		t.Error("expected error for no blocks")
	}
}

func TestParseDiffBlocksEmptySearch(t *testing.T) {
	input := "<<<<<<< SEARCH\n\n=======\nnew content\n>>>>>>> REPLACE"

	_, err := ParseDiffBlocks(input)
	if err == nil {
		t.Error("expected error for empty search section")
	}
}

func TestParseDiffBlocksUnclosedSearch(t *testing.T) {
	input := "<<<<<<< SEARCH\nsome code\nno separator"

	_, err := ParseDiffBlocks(input)
	if err == nil {
		t.Error("expected error for unclosed search")
	}
}

func TestParseDiffBlocksUnclosedReplace(t *testing.T) {
	input := "<<<<<<< SEARCH\nsome code\n=======\nnew code\nno end marker"

	_, err := ParseDiffBlocks(input)
	if err == nil {
		t.Error("expected error for unclosed replace")
	}
}

func TestParseDiffBlocksFilePathAnnotation(t *testing.T) {
	input := "--- cmd/server/main.go\n<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE"

	blocks, err := ParseDiffBlocks(input)
	if err != nil {
		t.Fatalf("ParseDiffBlocks error: %v", err)
	}
	if blocks[0].FilePath != "cmd/server/main.go" {
		t.Errorf("FilePath = %q, want %q", blocks[0].FilePath, "cmd/server/main.go")
	}
}

func TestParseDiffBlocksAPath(t *testing.T) {
	input := "--- a/internal/handler.go\n<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE"

	blocks, err := ParseDiffBlocks(input)
	if err != nil {
		t.Fatalf("ParseDiffBlocks error: %v", err)
	}
	if blocks[0].FilePath != "internal/handler.go" {
		t.Errorf("FilePath = %q, want %q", blocks[0].FilePath, "internal/handler.go")
	}
}

func TestApplyDiffExactMatch(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc old() {\n\treturn\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"func old() {", "\treturn", "}"},
		ReplaceLines: []string{"func new() {", "\treturn", "}"},
	}

	result, err := ApplyDiff(fp, block)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if result.MatchType != "exact" {
		t.Errorf("MatchType = %q, want %q", result.MatchType, "exact")
	}

	got, _ := os.ReadFile(fp)
	if !strings.Contains(string(got), "func new()") {
		t.Errorf("file should contain 'func new()', got:\n%s", string(got))
	}
}

func TestApplyDiffStrippedMatch(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\n  func old()  {\n    return\n  }\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"func old() {", "return", "}"},
		ReplaceLines: []string{"func new() {", "return", "}"},
	}

	result, err := ApplyDiff(fp, block)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success with fuzzy match, got error: %v", result.Error)
	}
	if result.MatchType != "fuzzy_stripped" {
		t.Errorf("MatchType = %q, want %q", result.MatchType, "fuzzy_stripped")
	}
}

func TestApplyDiffIndentMatch(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc main() {\n\tif true {\n\t\treturn\n\t}\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"if true {", "  return", "}"},
		ReplaceLines: []string{"if false {", "  return", "}"},
	}

	result, err := ApplyDiff(fp, block)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success with indent match, got error: %v", result.Error)
	}
}

func TestApplyDiffCharTolerant(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc helloWorld() {\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"func helloWrld() {"},
		ReplaceLines: []string{"func goodbye() {"},
	}

	result, err := ApplyDiff(fp, block)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success with char-tolerant match, got error: %v", result.Error)
	}
}

func TestApplyDiffNoMatch(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc something() {\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"this line does not exist anywhere"},
		ReplaceLines: []string{"replacement"},
	}

	result, err := ApplyDiff(fp, block)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for no match")
	}

	got, _ := os.ReadFile(fp)
	if strings.Contains(string(got), "replacement") {
		t.Error("file should not be modified on failed match")
	}
}

func TestApplyDiffsMultiple(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc a() {\n}\n\nfunc b() {\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	blocks := []DiffBlock{
		{
			FilePath:     fp,
			SearchLines:  []string{"func a() {", "}"},
			ReplaceLines: []string{"func alpha() {", "}"},
		},
		{
			FilePath:     fp,
			SearchLines:  []string{"func b() {", "}"},
			ReplaceLines: []string{"func beta() {", "}"},
		},
	}

	results, err := ApplyDiffs(blocks)
	if err != nil {
		t.Fatalf("ApplyDiffs error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if !r.Success {
			t.Errorf("result[%d] failed: %v", i, r.Error)
		}
	}

	got, _ := os.ReadFile(fp)
	s := string(got)
	if !strings.Contains(s, "func alpha()") {
		t.Error("file should contain 'func alpha()'")
	}
	if !strings.Contains(s, "func beta()") {
		t.Error("file should contain 'func beta()'")
	}
}

func TestDryRunNoModify(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc old() {\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"func old() {", "}"},
		ReplaceLines: []string{"func new() {", "}"},
	}

	result, err := DryRun(fp, block)
	if err != nil {
		t.Fatalf("DryRun error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}

	got, _ := os.ReadFile(fp)
	if string(got) != content {
		t.Error("DryRun should not modify the file")
	}
}

func TestVerifyGoFileValid(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "valid.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := VerifyGoFile(fp)
	if !result.Valid {
		t.Errorf("valid Go file should pass verification, issues: %v", result.Issues)
	}
}

func TestVerifyGoFileSyntaxError(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "invalid.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\"\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := VerifyGoFile(fp)
	if result.Valid {
		t.Error("syntax error should fail verification")
	}
}

func TestVerifyGoFileUnmatchedBraces(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "unmatched.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result := VerifyGoFile(fp)
	if result.Valid {
		t.Error("unmatched braces should fail verification")
	}
}

func TestVerifyGenericFile(t *testing.T) {
	dir := t.TempDir()

	fp := filepath.Join(dir, "test.txt")
	os.WriteFile(fp, []byte("hello world\n"), 0644)

	result := VerifyGenericFile(fp)
	if !result.Valid {
		t.Errorf("valid text file should pass, issues: %v", result.Issues)
	}
}

func TestVerifyGenericFileEmpty(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "empty.txt")
	os.WriteFile(fp, []byte(""), 0644)

	result := VerifyGenericFile(fp)
	if result.Valid {
		t.Error("empty file should fail verification")
	}
}

func TestVerifyGenericFileBinary(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "binary.bin")
	os.WriteFile(fp, []byte{0x00, 0x01, 0x02, 0x03}, 0644)

	result := VerifyGenericFile(fp)
	if result.Valid {
		t.Error("binary file should fail verification")
	}
}

func TestVerifyFileGoExtension(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	os.WriteFile(fp, []byte("package main\n"), 0644)

	result := VerifyFile(fp)
	if !result.Valid {
		t.Errorf("valid .go file should pass, issues: %v", result.Issues)
	}
}

func TestRollback(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	original := "package main\n\nfunc old() {\n}\n"
	modified := "package main\n\nfunc new() {\n}\n"
	os.WriteFile(fp, []byte(modified), 0644)

	err := Rollback(fp, original)
	if err != nil {
		t.Fatalf("Rollback error: %v", err)
	}

	got, _ := os.ReadFile(fp)
	if string(got) != original {
		t.Errorf("rollback didn't restore original.\ngot:  %q\nwant: %q", string(got), original)
	}
}

func TestEngineApplySuccess(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc old() {\n\treturn\n}\n"
	os.WriteFile(fp, []byte(content), 0644)

	engine := NewDiffEngine(WithVerifyAfterApply(false))
	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"func old() {", "\treturn", "}"},
		ReplaceLines: []string{"func new() {", "\treturn", "}"},
	}

	result, err := engine.Apply(context.Background(), fp, block)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got: %v", result.Error)
	}
}

func TestEngineAutoRollback(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n"
	os.WriteFile(fp, []byte(content), 0644)

	engine := NewDiffEngine(
		WithVerifyAfterApply(true),
		WithAutoRollback(true),
	)

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"func main() {", "\tprintln(\"hi\")", "}"},
		ReplaceLines: []string{"func main() {", "\tprintln("},
	}

	result, err := engine.Apply(context.Background(), fp, block)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Success {
		t.Error("expected failure due to syntax error after apply")
	}

	got, _ := os.ReadFile(fp)
	if string(got) != content {
		t.Error("file should be rolled back to original after verification failure")
	}
}

func TestEngineApplyFromOutput(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc old() {\n\treturn\n}\n"
	os.WriteFile(fp, []byte(content), 0644)

	engine := NewDiffEngine(WithVerifyAfterApply(false))
	output := "--- " + fp + "\n<<<<<<< SEARCH\nfunc old() {\n\treturn\n}\n=======\nfunc new() {\n\treturn\n}\n>>>>>>> REPLACE"

	results, err := engine.ApplyFromOutput(context.Background(), output)
	if err != nil {
		t.Fatalf("ApplyFromOutput error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Errorf("expected success, got: %v", results[0].Error)
	}

	got, _ := os.ReadFile(fp)
	if !strings.Contains(string(got), "func new()") {
		t.Error("file should contain 'func new()'")
	}
}

func TestEngineApplyFromOutputMultipleBlocks(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc a() {\n}\n\nfunc b() {\n}\n"
	os.WriteFile(fp, []byte(content), 0644)

	engine := NewDiffEngine(WithVerifyAfterApply(false))
	output := "--- " + fp + "\n<<<<<<< SEARCH\nfunc a() {\n}\n=======\nfunc alpha() {\n}\n>>>>>>> REPLACE\n\n<<<<<<< SEARCH\nfunc b() {\n}\n=======\nfunc beta() {\n}\n>>>>>>> REPLACE"

	results, err := engine.ApplyFromOutput(context.Background(), output)
	if err != nil {
		t.Fatalf("ApplyFromOutput error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestEngineDryRunFromOutput(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc old() {\n}\n"
	os.WriteFile(fp, []byte(content), 0644)

	engine := NewDiffEngine()
	output := "--- " + fp + "\n<<<<<<< SEARCH\nfunc old() {\n}\n=======\nfunc new() {\n}\n>>>>>>> REPLACE"

	results, err := engine.DryRunFromOutput(context.Background(), output)
	if err != nil {
		t.Fatalf("DryRunFromOutput error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	got, _ := os.ReadFile(fp)
	if string(got) != content {
		t.Error("DryRunFromOutput should not modify file")
	}
}

func TestEngineApplyNoFilePath(t *testing.T) {
	engine := NewDiffEngine()
	_, err := engine.ApplyBlocks(context.Background(), []DiffBlock{
		{SearchLines: []string{"x"}, ReplaceLines: []string{"y"}},
	})
	if err == nil {
		t.Error("expected error for block with no file path")
	}
}

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"hello", "hello", 0},
		{"hello", "helo", 1},
		{"hello", "hallo", 1},
		{"abc", "xyz", 3},
		{"", "abc", 3},
		{"abc", "", 3},
	}

	for _, tc := range tests {
		got := editDistance(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestApplyDiffIndentAdjustment(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\nfunc main() {\n\tif true {\n\t\treturn\n\t}\n}\n"
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"if true {", "return", "}"},
		ReplaceLines: []string{"if false {", "return", "}"},
	}

	result, err := ApplyDiff(fp, block)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got: %v", result.Error)
	}

	got, _ := os.ReadFile(fp)
	s := string(got)
	if !strings.Contains(s, "if false {") {
		t.Errorf("file should contain 'if false {', got:\n%s", s)
	}
	if !strings.Contains(s, "\t\treturn") {
		t.Errorf("indentation should be preserved, got:\n%s", s)
	}
}

func TestHashContent(t *testing.T) {
	h1 := hashContent([]byte("hello"))
	h2 := hashContent([]byte("hello"))
	h3 := hashContent([]byte("world"))

	if h1 != h2 {
		t.Errorf("same content should produce same hash: %q vs %q", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("different content should produce different hash: %q vs %q", h1, h3)
	}
}

func TestTrimBlankLines(t *testing.T) {
	tests := []struct {
		input []string
		want  int
	}{
		{[]string{"", "hello", ""}, 1},
		{[]string{"hello", "world"}, 2},
		{[]string{"", "", ""}, 0},
		{[]string{"a", "", "b"}, 3},
	}

	for _, tc := range tests {
		got := trimBlankLines(tc.input)
		if len(got) != tc.want {
			t.Errorf("trimBlankLines(%v) = %d lines, want %d", tc.input, len(got), tc.want)
		}
	}
}

func TestParseUnifiedDiff(t *testing.T) {
	input := "--- a/main.go\n+++ b/main.go\n@@ -3,1 +3,1 @@\n-func old() {\n+func new() {"

	blocks, err := ParseUnifiedDiff(input)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].FilePath != "main.go" {
		t.Errorf("FilePath = %q, want %q", blocks[0].FilePath, "main.go")
	}
	if len(blocks[0].SearchLines) != 1 || blocks[0].SearchLines[0] != "func old() {" {
		t.Errorf("SearchLines = %v", blocks[0].SearchLines)
	}
	if len(blocks[0].ReplaceLines) != 1 || blocks[0].ReplaceLines[0] != "func new() {" {
		t.Errorf("ReplaceLines = %v", blocks[0].ReplaceLines)
	}
	if blocks[0].LineNumber != 3 {
		t.Errorf("LineNumber = %d, want 3", blocks[0].LineNumber)
	}
}

func TestParseUnifiedDiffWithContext(t *testing.T) {
	input := "--- a/main.go\n+++ b/main.go\n@@ -1,5 +1,5 @@\n package main\n \n-func old() {\n+func new() {\n }"

	blocks, err := ParseUnifiedDiff(input)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff error: %v", err)
	}
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	hasPackageInSearch := false
	for _, l := range blocks[0].SearchLines {
		if strings.TrimSpace(l) == "package main" {
			hasPackageInSearch = true
		}
	}
	if !hasPackageInSearch {
		t.Error("context line 'package main' should appear in SearchLines")
	}
}

func TestParseUnifiedDiffNoHunks(t *testing.T) {
	_, err := ParseUnifiedDiff("no diff content here")
	if err == nil {
		t.Error("expected error for no hunks")
	}
}

func TestApplyDiffEmptySearchLines(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	os.WriteFile(fp, []byte("package main\n"), 0644)

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{},
		ReplaceLines: []string{"new line"},
	}

	result, err := ApplyDiff(fp, block)
	if err != nil {
		t.Fatalf("ApplyDiff error: %v", err)
	}
	if result.Success {
		t.Error("empty search lines should fail")
	}
}

func TestEngineWithFuzzyDisabled(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "test.go")
	content := "package main\n\n  func old()  {\n  }\n"
	os.WriteFile(fp, []byte(content), 0644)

	engine := NewDiffEngine(
		WithFuzzyMatch(false),
		WithVerifyAfterApply(false),
	)

	block := DiffBlock{
		FilePath:     fp,
		SearchLines:  []string{"func old() {"},
		ReplaceLines: []string{"func new() {"},
	}

	result, err := engine.Apply(context.Background(), fp, block)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if result.Success {
		t.Error("exact-only match should fail when whitespace differs")
	}
}
