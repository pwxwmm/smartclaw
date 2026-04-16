package diffengine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
)

// ApplyResult holds the outcome of applying a single DiffBlock.
type ApplyResult struct {
	FilePath     string
	Success      bool
	LinesApplied int
	OriginalHash string
	NewHash      string
	Error        error
	MatchType    string
}

// ApplyDiff reads the file, finds the search lines using exact then fuzzy matching,
// replaces them with the replace lines, and writes back.
func ApplyDiff(filePath string, block DiffBlock) (*ApplyResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	originalHash := hashContent(content)

	result, newContent, err := applyDiffToContent(string(content), block)
	if err != nil {
		return &ApplyResult{
			FilePath:     filePath,
			Success:      false,
			OriginalHash: originalHash,
			Error:        err,
		}, nil
	}

	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write file %s: %w", filePath, err)
	}

	newHash := hashContent([]byte(newContent))
	result.FilePath = filePath
	result.OriginalHash = originalHash
	result.NewHash = newHash

	return result, nil
}

// ApplyDiffs applies multiple DiffBlocks to their respective files.
// Blocks targeting the same file are applied sequentially.
func ApplyDiffs(blocks []DiffBlock) ([]ApplyResult, error) {
	var results []ApplyResult

	fileBlocks := make(map[string][]DiffBlock)
	var fileOrder []string

	for _, b := range blocks {
		fp := b.FilePath
		if _, exists := fileBlocks[fp]; !exists {
			fileOrder = append(fileOrder, fp)
		}
		fileBlocks[fp] = append(fileBlocks[fp], b)
	}

	for _, fp := range fileOrder {
		bs := fileBlocks[fp]
		content, err := os.ReadFile(fp)
		if err != nil {
			for range bs {
				results = append(results, ApplyResult{
					FilePath: fp,
					Success:  false,
					Error:    fmt.Errorf("read file %s: %w", fp, err),
				})
			}
			continue
		}

		currentContent := string(content)
		originalHash := hashContent(content)

		for _, block := range bs {
			result, newContent, err := applyDiffToContent(currentContent, block)
			if err != nil {
				result.FilePath = fp
				result.OriginalHash = originalHash
				results = append(results, *result)
				continue
			}

			currentContent = newContent
			result.FilePath = fp
			result.OriginalHash = originalHash
			results = append(results, *result)
		}

		if err := os.WriteFile(fp, []byte(currentContent), 0644); err != nil {
			return nil, fmt.Errorf("write file %s: %w", fp, err)
		}

		newHash := hashContent([]byte(currentContent))
		for i := range results {
			if results[i].FilePath == fp && results[i].NewHash == "" {
				results[i].NewHash = newHash
			}
		}
	}

	return results, nil
}

// DryRun previews applying a DiffBlock without modifying the file.
func DryRun(filePath string, block DiffBlock) (*ApplyResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	originalHash := hashContent(content)

	result, _, err := applyDiffToContent(string(content), block)
	if err != nil {
		return &ApplyResult{
			FilePath:     filePath,
			Success:      false,
			OriginalHash: originalHash,
			Error:        err,
		}, nil
	}

	result.FilePath = filePath
	result.OriginalHash = originalHash
	return result, nil
}

func applyDiffToContent(content string, block DiffBlock) (*ApplyResult, string, error) {
	fileLines := strings.Split(content, "\n")

	// Remove trailing empty line from split if content ends with newline
	if len(fileLines) > 0 && fileLines[len(fileLines)-1] == "" {
		fileLines = fileLines[:len(fileLines)-1]
	}

	matchIdx, matchType, err := findMatch(fileLines, block.SearchLines, block.LineNumber, true)
	if err != nil {
		return &ApplyResult{
			Success:      false,
			MatchType:    "none",
			LinesApplied: 0,
			Error:        err,
		}, content, nil
	}

	adjustedReplace := adjustIndentation(fileLines, matchIdx, block.SearchLines, block.ReplaceLines)

	newLines := make([]string, 0, len(fileLines)+len(adjustedReplace)-len(block.SearchLines))
	newLines = append(newLines, fileLines[:matchIdx]...)
	newLines = append(newLines, adjustedReplace...)
	newLines = append(newLines, fileLines[matchIdx+len(block.SearchLines):]...)

	newContent := strings.Join(newLines, "\n")
	if strings.HasSuffix(content, "\n") {
		newContent += "\n"
	}

	return &ApplyResult{
		Success:      true,
		LinesApplied: len(block.SearchLines),
		MatchType:    matchType,
	}, newContent, nil
}

// findMatch attempts exact match first, then fuzzy match.
func findMatch(fileLines, searchLines []string, lineHint int, fuzzy bool) (int, string, error) {
	if len(searchLines) == 0 {
		return 0, "", fmt.Errorf("empty search lines")
	}

	// Try exact match first
	if idx := exactMatch(fileLines, searchLines, lineHint); idx >= 0 {
		return idx, "exact", nil
	}

	if !fuzzy {
		return 0, "", fmt.Errorf("exact match not found for %d search lines", len(searchLines))
	}

	// Try fuzzy: stripped whitespace
	if idx := strippedMatch(fileLines, searchLines, lineHint); idx >= 0 {
		return idx, "fuzzy_stripped", nil
	}

	// Try fuzzy: relative indentation
	if idx := relativeIndentMatch(fileLines, searchLines, lineHint); idx >= 0 {
		return idx, "fuzzy_indent", nil
	}

	// Try fuzzy: Levenshtein-like tolerance
	if idx := fuzzyCharMatch(fileLines, searchLines, lineHint, 2); idx >= 0 {
		return idx, "fuzzy_char", nil
	}

	return 0, "", fmt.Errorf("no match found for %d search lines (tried exact, stripped, indent, char-tolerant)", len(searchLines))
}

func exactMatch(fileLines, searchLines []string, lineHint int) int {
	if len(searchLines) > len(fileLines) {
		return -1
	}

	start := 0
	if lineHint > 0 {
		start = max(lineHint-2, 0)
	}
	end := len(fileLines) - len(searchLines) + 1

	for i := start; i < end; i++ {
		if linesEqual(fileLines[i:i+len(searchLines)], searchLines) {
			return i
		}
	}

	if lineHint > 0 {
		for i := 0; i < start; i++ {
			if linesEqual(fileLines[i:i+len(searchLines)], searchLines) {
				return i
			}
		}
	}

	return -1
}

func strippedMatch(fileLines, searchLines []string, lineHint int) int {
	strippedSearch := stripLines(searchLines)
	if len(strippedSearch) > len(fileLines) {
		return -1
	}

	start := 0
	if lineHint > 0 {
		start = max(lineHint-2, 0)
	}
	end := len(fileLines) - len(strippedSearch) + 1

	for i := start; i < end; i++ {
		fileSlice := fileLines[i : i+len(strippedSearch)]
		if strippedLinesEqual(fileSlice, strippedSearch) {
			return i
		}
	}

	if lineHint > 0 {
		for i := 0; i < start; i++ {
			fileSlice := fileLines[i : i+len(strippedSearch)]
			if strippedLinesEqual(fileSlice, strippedSearch) {
				return i
			}
		}
	}

	return -1
}

func relativeIndentMatch(fileLines, searchLines []string, lineHint int) int {
	relSearch := relativeIndentLines(searchLines)
	if len(relSearch) > len(fileLines) {
		return -1
	}

	start := 0
	if lineHint > 0 {
		start = max(lineHint-2, 0)
	}
	end := len(fileLines) - len(relSearch) + 1

	for i := start; i < end; i++ {
		fileSlice := fileLines[i : i+len(relSearch)]
		if relativeIndentEqual(fileSlice, relSearch) {
			return i
		}
	}

	if lineHint > 0 {
		for i := 0; i < start; i++ {
			fileSlice := fileLines[i : i+len(relSearch)]
			if relativeIndentEqual(fileSlice, relSearch) {
				return i
			}
		}
	}

	return -1
}

func fuzzyCharMatch(fileLines, searchLines []string, lineHint int, maxDist int) int {
	if len(searchLines) > len(fileLines) {
		return -1
	}

	start := 0
	if lineHint > 0 {
		start = max(lineHint-3, 0)
	}
	end := len(fileLines) - len(searchLines) + 1

	for i := start; i < end; i++ {
		fileSlice := fileLines[i : i+len(searchLines)]
		if fuzzyLinesEqual(fileSlice, searchLines, maxDist) {
			return i
		}
	}

	if lineHint > 0 {
		for i := 0; i < start; i++ {
			fileSlice := fileLines[i : i+len(searchLines)]
			if fuzzyLinesEqual(fileSlice, searchLines, maxDist) {
				return i
			}
		}
	}

	return -1
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func strippedLinesEqual(fileLines, strippedSearch []string) bool {
	if len(fileLines) != len(strippedSearch) {
		return false
	}
	for i := range fileLines {
		if strings.TrimSpace(fileLines[i]) != strippedSearch[i] {
			return false
		}
	}
	return true
}

func stripLines(lines []string) []string {
	result := make([]string, len(lines))
	for i, l := range lines {
		result[i] = strings.TrimSpace(l)
	}
	return result
}

func relativeIndentLines(lines []string) []string {
	minIndent := -1
	for _, l := range lines {
		trimmed := strings.TrimLeft(l, " \t")
		if trimmed == "" {
			continue
		}
		indent := len(l) - len(trimmed)
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
	}

	result := make([]string, len(lines))
	for i, l := range lines {
		trimmed := strings.TrimLeft(l, " \t")
		if trimmed == "" {
			result[i] = ""
			continue
		}
		indent := len(l) - len(trimmed)
		relIndent := indent - minIndent
		if relIndent < 0 {
			relIndent = 0
		}
		result[i] = strings.Repeat(" ", relIndent) + trimmed
	}
	return result
}

func relativeIndentEqual(fileLines, relSearch []string) bool {
	if len(fileLines) != len(relSearch) {
		return false
	}

	fileRel := relativeIndentLines(fileLines)
	return linesEqual(fileRel, relSearch)
}

func fuzzyLinesEqual(fileLines, searchLines []string, maxDist int) bool {
	if len(fileLines) != len(searchLines) {
		return false
	}
	for i := range fileLines {
		if editDistance(fileLines[i], searchLines[i]) > maxDist {
			return false
		}
	}
	return true
}

// editDistance computes the Levenshtein edit distance between two strings.
func editDistance(a, b string) int {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)

	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)

	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

// adjustIndentation adjusts the replace lines to match the indentation of the
// original matched block in the file.
func adjustIndentation(fileLines []string, matchIdx int, searchLines, replaceLines []string) []string {
	if len(replaceLines) == 0 {
		return replaceLines
	}

	fileIndent := getIndent(fileLines[matchIdx])
	searchIndent := getIndent(searchLines[0])

	indentDelta := len(fileIndent) - len(searchIndent)
	if indentDelta == 0 {
		return replaceLines
	}

	result := make([]string, len(replaceLines))
	for i, line := range replaceLines {
		if strings.TrimSpace(line) == "" {
			result[i] = line
			continue
		}
		lineIndent := getIndent(line)
		newIndentLen := len(lineIndent) + indentDelta
		if newIndentLen < 0 {
			newIndentLen = 0
		}
		result[i] = strings.Repeat(" ", newIndentLen) + strings.TrimLeft(line, " \t")
	}
	return result
}

func getIndent(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

func hashContent(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h[:8])
}
