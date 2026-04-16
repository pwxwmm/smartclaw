package patch

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// Edit represents a line-based replacement operation.
type Edit struct {
	File    string
	Start   int // 1-based line number (0 = insert before line 1)
	End     int // 1-based, inclusive
	Content string
}

// EditResult holds the outcome of an edit application.
type EditResult struct {
	Diff    *FileDiff
	Preview string
}

// EditLines applies a sequence of line edits to content and returns the new
// content plus a FileDiff for rollback. Edits are applied in reverse order
// (highest line numbers first) so that earlier edits don't shift line numbers
// for later ones.
func EditLines(content []byte, edits []Edit) ([]byte, *FileDiff, error) {
	if len(edits) == 0 {
		return content, &FileDiff{}, nil
	}

	sorted := make([]Edit, len(edits))
	copy(sorted, edits)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Start > sorted[j].Start
	})

	lines := splitLines(content)
	oldContent := content

	for _, e := range sorted {
		var newLines []string
		var err error
		if newLines, err = applyEditToLines(lines, e); err != nil {
			return nil, nil, err
		}
		lines = newLines
	}

	var buf strings.Builder
	for _, l := range lines {
		buf.WriteString(l)
	}
	newContent := []byte(buf.String())

	diffData := Diff("original", "modified", oldContent, newContent)
	diffs, err := ParseDiff(diffData)
	if err != nil {
		return newContent, nil, nil
	}

	return newContent, diffs[0], nil
}

// PreviewEdit reads the file at path, applies the edit, and returns a unified
// diff string showing what would change without modifying the file.
func PreviewEdit(path string, edit Edit) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}

	edited, _, err := EditLines(content, []Edit{edit})
	if err != nil {
		return "", err
	}

	diff := Diff(path, path, content, edited)
	return string(diff), nil
}

// ApplyEdit applies a single edit to a file on disk and returns the diff
// for potential rollback.
func ApplyEdit(path string, edit Edit) (*EditResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	edited, fileDiff, err := EditLines(content, []Edit{edit})
	if err != nil {
		return nil, err
	}

	preview := string(Diff(path, path, content, edited))

	if err := os.WriteFile(path, edited, 0o644); err != nil {
		return nil, fmt.Errorf("write file %s: %w", path, err)
	}

	return &EditResult{
		Diff:    fileDiff,
		Preview: preview,
	}, nil
}

func applyEditToLines(lines []string, e Edit) ([]string, error) {
	if e.Start == 0 && e.End == 0 {
		return applyInsertBeforeFirst(lines, e.Content), nil
	}

	if e.Start < 0 || e.End < e.Start {
		return nil, fmt.Errorf("invalid edit range: Start=%d End=%d", e.Start, e.End)
	}

	if e.Start == 0 {
		e.Start = 1
	}

	if e.End > len(lines) {
		e.End = len(lines)
	}

	replacementLines := contentToLines(e.Content)

	var result []string
	result = append(result, lines[:e.Start-1]...)
	result = append(result, replacementLines...)
	result = append(result, lines[e.End:]...)

	return result, nil
}

func applyInsertBeforeFirst(lines []string, content string) []string {
	replacementLines := contentToLines(content)
	var result []string
	result = append(result, replacementLines...)
	result = append(result, lines...)
	return result
}

func contentToLines(content string) []string {
	if content == "" {
		return nil
	}

	hasTrailingNewline := strings.HasSuffix(content, "\n")
	parts := strings.Split(content, "\n")

	if hasTrailingNewline {
		parts = parts[:len(parts)-1]
	}

	var result []string
	for _, p := range parts {
		result = append(result, p+"\n")
	}
	return result
}
