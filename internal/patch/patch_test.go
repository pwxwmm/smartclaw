package patch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffIdentical(t *testing.T) {
	old := []byte("line1\nline2\nline3\n")
	diff := Diff("a.txt", "a.txt", old, old)
	if len(diff) > 0 {
		got := string(diff)
		if strings.Contains(got, "@@") {
			t.Errorf("identical files should produce no hunks, got:\n%s", got)
		}
	}
}

func TestDiffAddLines(t *testing.T) {
	old := []byte("line1\nline2\n")
	new := []byte("line1\ninserted\nline2\n")
	diff := Diff("a.txt", "a.txt", old, new)

	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}
	if len(diffs) == 0 {
		t.Fatal("expected at least one FileDiff")
	}

	added, removed := Stats(diffs[0])
	if added == 0 {
		t.Error("expected some added lines")
	}
	if removed != 0 {
		t.Errorf("expected 0 removed lines, got %d", removed)
	}
}

func TestDiffRemoveLines(t *testing.T) {
	old := []byte("line1\ndeleted\nline2\n")
	new := []byte("line1\nline2\n")
	diff := Diff("a.txt", "a.txt", old, new)

	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}

	added, removed := Stats(diffs[0])
	if added != 0 {
		t.Errorf("expected 0 added lines, got %d", added)
	}
	if removed == 0 {
		t.Error("expected some removed lines")
	}
}

func TestDiffReplaceLines(t *testing.T) {
	old := []byte("line1\nold\nline3\n")
	new := []byte("line1\nnew\nline3\n")
	diff := Diff("a.txt", "a.txt", old, new)

	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}

	added, removed := Stats(diffs[0])
	if added == 0 {
		t.Error("expected some added lines")
	}
	if removed == 0 {
		t.Error("expected some removed lines")
	}
}

func TestDiffEmptyFiles(t *testing.T) {
	diff := Diff("a.txt", "a.txt", []byte{}, []byte{})
	if len(diff) > 0 {
		got := string(diff)
		if strings.Contains(got, "@@") {
			t.Errorf("two empty files should produce no hunks, got:\n%s", got)
		}
	}
}

func TestDiffOldEmpty(t *testing.T) {
	new := []byte("line1\nline2\n")
	diff := Diff("a.txt", "a.txt", []byte{}, new)

	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}
	added, _ := Stats(diffs[0])
	if added != 2 {
		t.Errorf("expected 2 added lines, got %d", added)
	}
}

func TestDiffNewEmpty(t *testing.T) {
	old := []byte("line1\nline2\n")
	diff := Diff("a.txt", "a.txt", old, []byte{})

	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}
	_, removed := Stats(diffs[0])
	if removed != 2 {
		t.Errorf("expected 2 removed lines, got %d", removed)
	}
}

func TestParseDiffRoundTrip(t *testing.T) {
	old := []byte("line1\nold_line\nline3\nline4\n")
	new := []byte("line1\nnew_line\nline3\nline4\nadded\n")
	diff := Diff("original.txt", "modified.txt", old, new)

	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 FileDiff, got %d", len(diffs))
	}

	fd := diffs[0]
	if fd.OldPath != "original.txt" {
		t.Errorf("OldPath = %q, want %q", fd.OldPath, "original.txt")
	}
	if fd.NewPath != "modified.txt" {
		t.Errorf("NewPath = %q, want %q", fd.NewPath, "modified.txt")
	}
	if len(fd.Hunks) == 0 {
		t.Fatal("expected at least one hunk")
	}
}

func TestApplyDiff(t *testing.T) {
	old := []byte("line1\nold\nline3\n")
	new := []byte("line1\nnew\nline3\n")

	diff := Diff("a.txt", "a.txt", old, new)
	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}

	result, applyResult, err := Apply(old, diffs[0])
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if !applyResult.Applied {
		t.Errorf("expected Applied=true, got conflicts: %v", applyResult.Conflicts)
	}

	resultStr := string(result)
	if !strings.Contains(resultStr, "new") {
		t.Errorf("result should contain 'new', got:\n%s", resultStr)
	}
	if strings.Contains(resultStr, "old") {
		t.Errorf("result should not contain 'old', got:\n%s", resultStr)
	}
}

func TestApplyConflict(t *testing.T) {
	content := []byte("different\ncontent\nhere\n")

	fd := &FileDiff{
		OldPath: "a.txt",
		NewPath: "a.txt",
		Hunks: []*Hunk{{
			OldStart: 1,
			OldCount: 1,
			NewStart: 1,
			NewCount: 1,
			Lines: []DiffLine{
				{Type: LineRemoved, Content: "original_first_line\n", OldLine: 1, NewLine: 0},
				{Type: LineAdded, Content: "replacement\n", OldLine: 0, NewLine: 1},
			},
		}},
	}

	_, applyResult, err := Apply(content, fd)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if applyResult.Applied {
		t.Error("expected conflict, but Apply reported success")
	}
	if len(applyResult.Conflicts) == 0 {
		t.Error("expected at least one conflict")
	}
}

func TestReverseDiff(t *testing.T) {
	old := []byte("line1\nold\nline3\n")
	new := []byte("line1\nnew\nline3\n")

	diff := Diff("a.txt", "a.txt", old, new)
	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}

	rev := Reverse(diffs[0])
	result, applyResult, err := Apply(new, rev)
	if err != nil {
		t.Fatalf("Apply reversed diff error: %v", err)
	}
	if !applyResult.Applied {
		t.Errorf("reverse Apply failed: %v", applyResult.Conflicts)
	}

	if string(result) != string(old) {
		t.Errorf("reverse apply didn't restore original.\ngot:  %q\nwant: %q", string(result), string(old))
	}
}

func TestApplyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("line1\nline2\nline3\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	newContent := []byte("line1\nreplaced\nline3\n")
	diff := Diff("a.txt", "a.txt", content, newContent)
	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}

	applyResult, err := ApplyFile(path, diffs[0])
	if err != nil {
		t.Fatalf("ApplyFile error: %v", err)
	}
	if !applyResult.Applied {
		t.Errorf("ApplyFile not applied: %v", applyResult.Conflicts)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "replaced") {
		t.Errorf("file should contain 'replaced', got:\n%s", string(got))
	}

	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("backup file error: %v", err)
	}
	if string(backup) != string(content) {
		t.Errorf("backup doesn't match original.\ngot:  %q\nwant: %q", string(backup), string(content))
	}
}

func TestEditLinesBasic(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\n")
	edit := Edit{
		Start:   2,
		End:     3,
		Content: "replaced\n",
	}

	result, fileDiff, err := EditLines(content, []Edit{edit})
	if err != nil {
		t.Fatalf("EditLines error: %v", err)
	}

	got := string(result)
	want := "line1\nreplaced\nline4\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if fileDiff == nil {
		t.Error("expected non-nil FileDiff")
	}
}

func TestEditLinesInsert(t *testing.T) {
	content := []byte("line1\nline2\n")
	edit := Edit{
		Start:   2,
		End:     2,
		Content: "inserted\n",
	}

	result, _, err := EditLines(content, []Edit{edit})
	if err != nil {
		t.Fatalf("EditLines error: %v", err)
	}

	got := string(result)
	if !strings.Contains(got, "inserted") {
		t.Errorf("result should contain 'inserted', got:\n%s", got)
	}
}

func TestEditLinesInsertBeforeFirst(t *testing.T) {
	content := []byte("line1\nline2\n")
	edit := Edit{
		Start:   0,
		End:     0,
		Content: "before_first\n",
	}

	result, _, err := EditLines(content, []Edit{edit})
	if err != nil {
		t.Fatalf("EditLines error: %v", err)
	}

	got := string(result)
	if !strings.HasPrefix(got, "before_first\n") {
		t.Errorf("result should start with 'before_first', got:\n%s", got)
	}
}

func TestEditLinesEmptyContent(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")
	edit := Edit{
		Start:   2,
		End:     2,
		Content: "",
	}

	result, _, err := EditLines(content, []Edit{edit})
	if err != nil {
		t.Fatalf("EditLines error: %v", err)
	}

	got := string(result)
	want := "line1\nline3\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEditLinesMultipleEdits(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\nline5\n")
	edits := []Edit{
		{Start: 2, End: 2, Content: "edited2\n"},
		{Start: 4, End: 4, Content: "edited4\n"},
	}

	result, _, err := EditLines(content, edits)
	if err != nil {
		t.Fatalf("EditLines error: %v", err)
	}

	got := string(result)
	if !strings.Contains(got, "edited2") {
		t.Error("result should contain 'edited2'")
	}
	if !strings.Contains(got, "edited4") {
		t.Error("result should contain 'edited4'")
	}
}

func TestEditLinesDeleteRange(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\n")
	edit := Edit{
		Start:   2,
		End:     3,
		Content: "",
	}

	result, _, err := EditLines(content, []Edit{edit})
	if err != nil {
		t.Fatalf("EditLines error: %v", err)
	}

	got := string(result)
	want := "line1\nline4\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPreviewEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("line1\nline2\nline3\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	edit := Edit{Start: 2, End: 2, Content: "replaced\n"}
	preview, err := PreviewEdit(path, edit)
	if err != nil {
		t.Fatalf("PreviewEdit error: %v", err)
	}

	if !strings.Contains(preview, "replaced") {
		t.Errorf("preview should contain 'replaced', got:\n%s", preview)
	}

	got, _ := os.ReadFile(path)
	if string(got) != string(content) {
		t.Error("PreviewEdit should not modify the file")
	}
}

func TestApplyEdit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("line1\nline2\nline3\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	edit := Edit{Start: 2, End: 2, Content: "replaced\n"}
	editResult, err := ApplyEdit(path, edit)
	if err != nil {
		t.Fatalf("ApplyEdit error: %v", err)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "replaced") {
		t.Errorf("file should contain 'replaced', got:\n%s", string(got))
	}
	if editResult.Diff == nil {
		t.Error("expected non-nil Diff in EditResult")
	}
	if editResult.Preview == "" {
		t.Error("expected non-empty Preview in EditResult")
	}
}

func TestApplyAndReverseRoundTrip(t *testing.T) {
	old := []byte("alpha\nbeta\ngamma\ndelta\n")
	new := []byte("alpha\nBETA\ngamma\nDELTA\n")

	diff := Diff("f.txt", "f.txt", old, new)
	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}

	applied, applyResult, err := Apply(old, diffs[0])
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if !applyResult.Applied {
		t.Fatalf("Apply failed: %v", applyResult.Conflicts)
	}
	if string(applied) != string(new) {
		t.Errorf("Apply didn't produce expected result.\ngot:  %q\nwant: %q", string(applied), string(new))
	}

	rev := Reverse(diffs[0])
	restored, revResult, err := Apply(applied, rev)
	if err != nil {
		t.Fatalf("reverse Apply error: %v", err)
	}
	if !revResult.Applied {
		t.Fatalf("reverse Apply failed: %v", revResult.Conflicts)
	}
	if string(restored) != string(old) {
		t.Errorf("reverse didn't restore original.\ngot:  %q\nwant: %q", string(restored), string(old))
	}
}

func TestStatsEmpty(t *testing.T) {
	fd := &FileDiff{
		OldPath: "a.txt",
		NewPath: "a.txt",
		Hunks:   []*Hunk{},
	}
	added, removed := Stats(fd)
	if added != 0 || removed != 0 {
		t.Errorf("expected (0,0), got (%d,%d)", added, removed)
	}
}

func TestEditLinesEmptyFile(t *testing.T) {
	content := []byte("")
	edit := Edit{
		Start:   0,
		End:     0,
		Content: "new line\n",
	}

	result, _, err := EditLines(content, []Edit{edit})
	if err != nil {
		t.Fatalf("EditLines error: %v", err)
	}

	got := string(result)
	want := "new line\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a\nb\nc\n", 3},
		{"a\nb\nc", 3},
		{"", 0},
		{"single\n", 1},
	}

	for _, tc := range tests {
		got := splitLines([]byte(tc.input))
		if len(got) != tc.want {
			t.Errorf("splitLines(%q) = %d lines, want %d", tc.input, len(got), tc.want)
		}
	}
}

func TestContentToLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a\nb\n", 2},
		{"a\nb", 2},
		{"", 0},
		{"single\n", 1},
	}

	for _, tc := range tests {
		got := contentToLines(tc.input)
		if len(got) != tc.want {
			t.Errorf("contentToLines(%q) = %d lines, want %d", tc.input, len(got), tc.want)
		}
	}
}

func TestDiffMultiHunk(t *testing.T) {
	var oldBuf, newBuf strings.Builder
	for i := 0; i < 20; i++ {
		oldBuf.WriteString(fmt.Sprintf("line%d\n", i))
		newBuf.WriteString(fmt.Sprintf("line%d\n", i))
	}
	oldBuf.WriteString("old_block_1\n")
	newBuf.WriteString("new_block_1\n")
	for i := 20; i < 40; i++ {
		oldBuf.WriteString(fmt.Sprintf("line%d\n", i))
		newBuf.WriteString(fmt.Sprintf("line%d\n", i))
	}
	oldBuf.WriteString("old_block_2\n")
	newBuf.WriteString("new_block_2\n")
	for i := 40; i < 60; i++ {
		oldBuf.WriteString(fmt.Sprintf("line%d\n", i))
		newBuf.WriteString(fmt.Sprintf("line%d\n", i))
	}

	diff := Diff("a.txt", "a.txt", []byte(oldBuf.String()), []byte(newBuf.String()))
	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}
	if len(diffs) == 0 || len(diffs[0].Hunks) < 2 {
		t.Errorf("expected multiple hunks, got %d", len(diffs))
	}
}

func TestEditLinesInvalidRange(t *testing.T) {
	content := []byte("line1\nline2\n")
	edit := Edit{
		Start:   3,
		End:     2,
		Content: "x\n",
	}
	_, _, err := EditLines(content, []Edit{edit})
	if err == nil {
		t.Error("expected error for invalid edit range")
	}
}

func TestDiffMergedHunksContext(t *testing.T) {
	old := []byte("alpha\nbeta\ngamma\ndelta\n")
	new := []byte("alpha\nBETA\ngamma\nDELTA\n")

	diff := Diff("a.txt", "a.txt", old, new)
	diffs, err := ParseDiff(diff)
	if err != nil {
		t.Fatalf("ParseDiff error: %v", err)
	}

	result, applyResult, err := Apply(old, diffs[0])
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if !applyResult.Applied {
		t.Fatalf("Apply failed: %v", applyResult.Conflicts)
	}
	if string(result) != string(new) {
		t.Errorf("got %q, want %q", string(result), string(new))
	}

	for _, h := range diffs[0].Hunks {
		for _, l := range h.Lines {
			trimmed := strings.TrimRight(l.Content, "\n")
			if trimmed == "gamma" && l.Type == LineRemoved {
				t.Error("gamma should not be marked as removed - it's unchanged")
			}
		}
	}
}
