package diffengine

import (
	"fmt"
	"regexp"
	"strings"
)

// DiffBlock represents a single search/replace block parsed from LLM output.
type DiffBlock struct {
	FilePath     string
	SearchLines  []string
	ReplaceLines []string
	LineNumber   int
}

// ParseDiffBlocks parses LLM output containing SEARCH/REPLACE blocks.
// Supported formats:
//
//	<<<<<<< SEARCH / <<<<<<< HEAD
//	original code lines
//	=======
//	replacement code lines
//	>>>>>>> REPLACE / >>>>>>> updated
//
// File path annotation: `--- path/to/file.go` before a block.
func ParseDiffBlocks(output string) ([]DiffBlock, error) {
	var blocks []DiffBlock

	lines := strings.Split(output, "\n")

	i := 0
	currentFilePath := ""

	for i < len(lines) {
		line := lines[i]

		if strings.HasPrefix(line, "--- ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			if strings.HasPrefix(path, "a/") {
				path = path[2:]
			}
			currentFilePath = path
			i++
			continue
		}

		if strings.HasPrefix(line, "+++ ") {
			i++
			continue
		}

		if isSearchStart(line) {
			block, consumed, err := parseOneBlock(lines, i, currentFilePath)
			if err != nil {
				return nil, fmt.Errorf("parsing block at line %d: %w", i+1, err)
			}
			blocks = append(blocks, block)
			i += consumed
			continue
		}

		if fp := extractFilePathAnnotation(line); fp != "" {
			currentFilePath = fp
		}

		i++
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no SEARCH/REPLACE blocks found in output")
	}

	return blocks, nil
}

// searchStartRe matches <<<<<<< SEARCH or <<<<<<< HEAD.
var searchStartRe = regexp.MustCompile(`^<{7}\s+(SEARCH|HEAD)\s*$`)

func isSearchStart(line string) bool {
	return searchStartRe.MatchString(strings.TrimSpace(line))
}

var separatorRe = regexp.MustCompile(`^={7}\s*$`)

func isSeparator(line string) bool {
	return separatorRe.MatchString(strings.TrimSpace(line))
}

// replaceEndRe matches >>>>>>> REPLACE, >>>>>>> updated, >>>>>>> ours, >>>>>>> theirs.
var replaceEndRe = regexp.MustCompile(`^>{7}\s+(REPLACE|updated|ours|theirs)\s*$`)

func isReplaceEnd(line string) bool {
	return replaceEndRe.MatchString(strings.TrimSpace(line))
}

func parseOneBlock(lines []string, startIdx int, filePath string) (DiffBlock, int, error) {
	block := DiffBlock{
		FilePath: filePath,
	}

	i := startIdx + 1

	var searchLines []string
	for i < len(lines) {
		if isSeparator(lines[i]) {
			break
		}
		searchLines = append(searchLines, lines[i])
		i++
	}

	if i >= len(lines) {
		return DiffBlock{}, 0, fmt.Errorf("unclosed SEARCH section: missing ======= separator")
	}

	i++

	var replaceLines []string
	for i < len(lines) {
		if isReplaceEnd(lines[i]) {
			break
		}
		replaceLines = append(replaceLines, lines[i])
		i++
	}

	if i >= len(lines) {
		return DiffBlock{}, 0, fmt.Errorf("unclosed REPLACE section: missing >>>>>>> REPLACE marker")
	}

	i++

	block.SearchLines = trimBlankLines(searchLines)
	block.ReplaceLines = trimBlankLines(replaceLines)

	if len(block.SearchLines) == 0 {
		return DiffBlock{}, 0, fmt.Errorf("empty SEARCH section")
	}

	consumed := i - startIdx
	return block, consumed, nil
}

// ParseUnifiedDiff parses standard unified diff format into DiffBlocks.
func ParseUnifiedDiff(output string) ([]DiffBlock, error) {
	lines := strings.Split(output, "\n")
	var blocks []DiffBlock

	i := 0
	currentFilePath := ""

	for i < len(lines) {
		line := lines[i]

		if strings.HasPrefix(line, "--- ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			if strings.HasPrefix(path, "a/") {
				path = path[2:]
			}
			currentFilePath = path
			i++
			continue
		}

		if strings.HasPrefix(line, "+++ ") {
			path := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
			if strings.HasPrefix(path, "b/") {
				path = path[2:]
			}
			if currentFilePath == "" {
				currentFilePath = path
			}
			i++
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			block, consumed, err := parseUnifiedHunk(lines, i, currentFilePath)
			if err != nil {
				return nil, fmt.Errorf("parsing unified hunk at line %d: %w", i+1, err)
			}
			blocks = append(blocks, block)
			i += consumed
			continue
		}

		i++
	}

	if len(blocks) == 0 {
		return nil, fmt.Errorf("no unified diff hunks found in output")
	}

	return blocks, nil
}

// hunkHeaderRe matches @@ -oldStart,oldCount +newStart,newCount @@.
var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func parseUnifiedHunk(lines []string, startIdx int, filePath string) (DiffBlock, int, error) {
	m := hunkHeaderRe.FindStringSubmatch(lines[startIdx])
	if m == nil {
		return DiffBlock{}, 0, fmt.Errorf("invalid hunk header: %s", lines[startIdx])
	}

	lineNum := 0
	if m[1] != "" {
		fmt.Sscanf(m[1], "%d", &lineNum)
	}

	var searchLines, replaceLines []string
	i := startIdx + 1

	for i < len(lines) {
		line := lines[i]

		if strings.HasPrefix(line, "@@ ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "diff ") {
			break
		}

		if len(line) == 0 {
			if i+1 < len(lines) {
				next := lines[i+1]
				if !strings.HasPrefix(next, "+") && !strings.HasPrefix(next, "-") && !strings.HasPrefix(next, " ") && next != "" {
					break
				}
			}
			searchLines = append(searchLines, "")
			replaceLines = append(replaceLines, "")
			i++
			continue
		}

		switch line[0] {
		case ' ':
			searchLines = append(searchLines, line[1:])
			replaceLines = append(replaceLines, line[1:])
		case '-':
			searchLines = append(searchLines, line[1:])
		case '+':
			replaceLines = append(replaceLines, line[1:])
		default:
			break
		}
		i++
	}

	consumed := i - startIdx
	return DiffBlock{
		FilePath:     filePath,
		SearchLines:  searchLines,
		ReplaceLines: replaceLines,
		LineNumber:   lineNum,
	}, consumed, nil
}

func trimBlankLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

// filePathAnnotationRe matches `// File: path.go`, `# path.go`, etc.
var filePathAnnotationRe = regexp.MustCompile(`^(?:#\s*|//\s*File:\s*|//\s*)(\S+\.go\S*)\s*$`)

func extractFilePathAnnotation(line string) string {
	m := filePathAnnotationRe.FindStringSubmatch(line)
	if m != nil {
		return m[1]
	}
	return ""
}
