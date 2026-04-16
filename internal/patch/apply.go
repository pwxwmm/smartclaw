package patch

import (
	"fmt"
	"os"
	"strings"
)

// Conflict represents a hunk that could not be applied cleanly.
type Conflict struct {
	HunkIndex int
	Message   string
}

// ApplyResult holds the outcome of a patch application.
type ApplyResult struct {
	Applied   bool
	Conflicts []Conflict
	BackedUp  string
}

// Apply applies a FileDiff to content and returns the new content.
// If any hunk cannot be applied due to mismatched context, it is recorded
// as a Conflict and the hunk is skipped.
func Apply(content []byte, diff *FileDiff) ([]byte, *ApplyResult, error) {
	result := &ApplyResult{}
	lines := splitLines(content)

	offset := 0

	for hi, hunk := range diff.Hunks {
		newLines, applied, delta := applyHunkWithOffset(lines, hunk, offset)
		if !applied {
			result.Conflicts = append(result.Conflicts, Conflict{
				HunkIndex: hi,
				Message:   fmt.Sprintf("hunk %d: context mismatch at old line %d", hi, hunk.OldStart),
			})
			continue
		}
		lines = newLines
		offset += delta
	}

	result.Applied = len(result.Conflicts) == 0

	var buf strings.Builder
	for _, l := range lines {
		buf.WriteString(l)
	}
	return []byte(buf.String()), result, nil
}

// ApplyFile applies a FileDiff to a file on disk. It creates a .bak backup
// before modification.
func ApplyFile(path string, diff *FileDiff) (*ApplyResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	backupPath := path + ".bak"
	if err := os.WriteFile(backupPath, content, 0o644); err != nil {
		return nil, fmt.Errorf("write backup %s: %w", backupPath, err)
	}

	newContent, result, err := Apply(content, diff)
	if err != nil {
		return nil, err
	}

	result.BackedUp = backupPath

	if result.Applied {
		if err := os.WriteFile(path, newContent, 0o644); err != nil {
			return nil, fmt.Errorf("write file %s: %w", path, err)
		}
	}

	return result, nil
}

// Reverse returns a new FileDiff that, when applied, undoes the original diff.
func Reverse(diff *FileDiff) *FileDiff {
	rev := &FileDiff{
		OldPath: diff.NewPath,
		NewPath: diff.OldPath,
		Hunks:   make([]*Hunk, len(diff.Hunks)),
	}

	for i, h := range diff.Hunks {
		rh := &Hunk{
			OldStart: h.NewStart,
			OldCount: h.NewCount,
			NewStart: h.OldStart,
			NewCount: h.OldCount,
			Lines:    make([]DiffLine, len(h.Lines)),
		}
		for j, l := range h.Lines {
			rl := DiffLine{
				Content: l.Content,
				OldLine: l.NewLine,
				NewLine: l.OldLine,
			}
			switch l.Type {
			case LineAdded:
				rl.Type = LineRemoved
			case LineRemoved:
				rl.Type = LineAdded
			case LineContext:
				rl.Type = LineContext
			}
			rh.Lines[j] = rl
		}
		rev.Hunks[i] = rh
	}

	return rev
}

func applyHunkWithOffset(lines []string, hunk *Hunk, offset int) ([]string, bool, int) {
	if hunk.OldCount == 0 && hunk.OldStart == 0 {
		result := applyPureInsert(lines, hunk)
		return result, true, hunk.NewCount - hunk.OldCount
	}

	startIdx := max(hunk.OldStart-1+offset, 0)
	if startIdx > len(lines) {
		return nil, false, 0
	}

	if !verifyContextAt(lines, hunk, startIdx) {
		return nil, false, 0
	}

	var result []string
	result = append(result, lines[:startIdx]...)

	oldIdx := startIdx
	for _, dl := range hunk.Lines {
		switch dl.Type {
		case LineContext:
			if oldIdx < len(lines) {
				result = append(result, lines[oldIdx])
			}
			oldIdx++
		case LineRemoved:
			oldIdx++
		case LineAdded:
			result = append(result, dl.Content)
		}
	}

	if oldIdx < len(lines) {
		result = append(result, lines[oldIdx:]...)
	}

	delta := hunk.NewCount - hunk.OldCount
	return result, true, delta
}

func applyPureInsert(lines []string, hunk *Hunk) []string {
	insertIdx := min(max(hunk.NewStart-1, 0), len(lines))

	var result []string
	result = append(result, lines[:insertIdx]...)

	for _, dl := range hunk.Lines {
		if dl.Type == LineAdded {
			result = append(result, dl.Content)
		}
	}

	result = append(result, lines[insertIdx:]...)
	return result
}

func verifyContextAt(lines []string, hunk *Hunk, startIdx int) bool {
	idx := startIdx
	for _, dl := range hunk.Lines {
		switch dl.Type {
		case LineContext, LineRemoved:
			if idx < 0 || idx >= len(lines) {
				return false
			}
			if !lineContentEqual(lines[idx], dl.Content) {
				return false
			}
			idx++
		case LineAdded:
		}
	}
	return true
}

func lineContentEqual(a, b string) bool {
	a = strings.TrimRight(a, "\n\r")
	b = strings.TrimRight(b, "\n\r")
	return a == b
}
