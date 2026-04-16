// Package patch provides unified diff/patch generation, parsing, and application
// for Agent file operations. All line numbers are 1-based.
package patch

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DiffLineType represents the kind of change a diff line represents.
type DiffLineType int

const (
	LineContext DiffLineType = iota
	LineAdded
	LineRemoved
)

// DiffLine represents a single line within a diff hunk.
type DiffLine struct {
	Type    DiffLineType
	Content string
	OldLine int // 1-based line number in old file (0 if added)
	NewLine int // 1-based line number in new file (0 if removed)
}

// Hunk represents a contiguous block of changes in a unified diff.
type Hunk struct {
	OldStart int // 1-based start line in old file
	OldCount int // number of old-file lines in this hunk
	NewStart int // 1-based start line in new file
	NewCount int // number of new-file lines in this hunk
	Lines    []DiffLine
}

// FileDiff represents the diff for a single file.
type FileDiff struct {
	OldPath string
	NewPath string
	Hunks   []*Hunk
}

// Diff generates a unified diff between oldContent and newContent.
// oldName and newName are used in the --- and +++ headers.
func Diff(oldName, newName string, oldContent, newContent []byte) []byte {
	oldLines := splitLines(oldContent)
	newLines := splitLines(newContent)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "--- %s\n", oldName)
	fmt.Fprintf(&buf, "+++ %s\n", newName)

	hunks := computeHunks(oldLines, newLines)
	for _, h := range hunks {
		fmt.Fprintf(&buf, "@@ -%d,%d +%d,%d @@\n", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
		for _, l := range h.Lines {
			switch l.Type {
			case LineContext:
				fmt.Fprintf(&buf, " %s", l.Content)
			case LineAdded:
				fmt.Fprintf(&buf, "+%s", l.Content)
			case LineRemoved:
				fmt.Fprintf(&buf, "-%s", l.Content)
			}
			if !strings.HasSuffix(l.Content, "\n") {
				buf.WriteByte('\n')
			}
		}
	}

	return buf.Bytes()
}

// ParseDiff parses unified diff format into FileDiff structures.
func ParseDiff(data []byte) ([]*FileDiff, error) {
	lines := strings.Split(string(data), "\n")
	var diffs []*FileDiff

	i := 0
	for i < len(lines) {
		if !strings.HasPrefix(lines[i], "--- ") {
			i++
			continue
		}

		oldPath := strings.TrimPrefix(lines[i], "--- ")
		i++

		if i >= len(lines) || !strings.HasPrefix(lines[i], "+++ ") {
			return nil, fmt.Errorf("expected +++ header after --- at line %d", i+1)
		}
		newPath := strings.TrimPrefix(lines[i], "+++ ")
		i++

		fd := &FileDiff{OldPath: oldPath, NewPath: newPath}

		for i < len(lines) && strings.HasPrefix(lines[i], "@@ ") {
			h, consumed, err := parseHunk(lines[i:])
			if err != nil {
				return nil, fmt.Errorf("parsing hunk at line %d: %w", i+1, err)
			}
			fd.Hunks = append(fd.Hunks, h)
			i += consumed
		}

		diffs = append(diffs, fd)
	}

	if len(diffs) == 0 {
		return nil, fmt.Errorf("no valid diff found")
	}

	return diffs, nil
}

// Stats returns the number of added and removed lines in a FileDiff.
func Stats(d *FileDiff) (added, removed int) {
	for _, h := range d.Hunks {
		for _, l := range h.Lines {
			switch l.Type {
			case LineAdded:
				added++
			case LineRemoved:
				removed++
			}
		}
	}
	return
}

// splitLines splits content into lines, preserving newline terminators.
func splitLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	result := strings.SplitAfter(string(content), "\n")
	if len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return result
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func parseHunk(lines []string) (*Hunk, int, error) {
	if len(lines) == 0 {
		return nil, 0, fmt.Errorf("empty lines")
	}

	m := hunkHeaderRe.FindStringSubmatch(lines[0])
	if m == nil {
		return nil, 0, fmt.Errorf("invalid hunk header: %s", lines[0])
	}

	oldStart, _ := strconv.Atoi(m[1])
	oldCount := 1
	if m[2] != "" {
		oldCount, _ = strconv.Atoi(m[2])
	}
	newStart, _ := strconv.Atoi(m[3])
	newCount := 1
	if m[4] != "" {
		newCount, _ = strconv.Atoi(m[4])
	}

	h := &Hunk{
		OldStart: oldStart,
		OldCount: oldCount,
		NewStart: newStart,
		NewCount: newCount,
	}

	consumed := 1
	oldLine := oldStart
	newLine := newStart

	for consumed < len(lines) {
		line := lines[consumed]
		if strings.HasPrefix(line, "@@ ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "diff ") {
			break
		}
		if line == "" && len(h.Lines) >= oldCount+newCount {
			break
		}

		if strings.HasPrefix(line, "+") {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineAdded,
				Content: line[1:] + "\n",
				OldLine: 0,
				NewLine: newLine,
			})
			newLine++
		} else if strings.HasPrefix(line, "-") {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineRemoved,
				Content: line[1:] + "\n",
				OldLine: oldLine,
				NewLine: 0,
			})
			oldLine++
		} else if strings.HasPrefix(line, " ") {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineContext,
				Content: line[1:] + "\n",
				OldLine: oldLine,
				NewLine: newLine,
			})
			oldLine++
			newLine++
		} else if line == "\\" {
			consumed++
		} else {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineContext,
				Content: line + "\n",
				OldLine: oldLine,
				NewLine: newLine,
			})
			oldLine++
			newLine++
		}
		consumed++

		if oldLine > oldStart+oldCount-1 && newLine > newStart+newCount-1 {
			ctxCount := 0
			addCount := 0
			remCount := 0
			for _, l := range h.Lines {
				switch l.Type {
				case LineContext:
					ctxCount++
				case LineAdded:
					addCount++
				case LineRemoved:
					remCount++
				}
			}
			if ctxCount+remCount >= oldCount && ctxCount+addCount >= newCount {
				break
			}
		}
	}

	return h, consumed, nil
}

func computeHunks(oldLines, newLines []string) []*Hunk {
	if len(oldLines) == 0 && len(newLines) == 0 {
		return nil
	}

	if len(oldLines) == 0 {
		h := &Hunk{
			OldStart: 0,
			OldCount: 0,
			NewStart: 1,
			NewCount: len(newLines),
		}
		for i, nl := range newLines {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineAdded,
				Content: nl,
				OldLine: 0,
				NewLine: i + 1,
			})
		}
		return []*Hunk{h}
	}

	if len(newLines) == 0 {
		h := &Hunk{
			OldStart: 1,
			OldCount: len(oldLines),
			NewStart: 0,
			NewCount: 0,
		}
		for i, ol := range oldLines {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineRemoved,
				Content: ol,
				OldLine: i + 1,
				NewLine: 0,
			})
		}
		return []*Hunk{h}
	}

	lcs := computeLCS(oldLines, newLines)

	if len(lcs) == 0 {
		h := &Hunk{
			OldStart: 1,
			OldCount: len(oldLines),
			NewStart: 1,
			NewCount: len(newLines),
		}
		for i, ol := range oldLines {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineRemoved,
				Content: ol,
				OldLine: i + 1,
				NewLine: 0,
			})
		}
		for i, nl := range newLines {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineAdded,
				Content: nl,
				OldLine: 0,
				NewLine: i + 1,
			})
		}
		return []*Hunk{h}
	}

	return buildHunksFromLCS(oldLines, newLines, lcs)
}

func computeLCS(oldLines, newLines []string) [][2]int {
	m, n := len(oldLines), len(newLines)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	var matches [][2]int
	i, j := m, n
	for i > 0 && j > 0 {
		if oldLines[i-1] == newLines[j-1] {
			matches = append(matches, [2]int{i - 1, j - 1})
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	for l, r := 0, len(matches)-1; l < r; l, r = l+1, r-1 {
		matches[l], matches[r] = matches[r], matches[l]
	}

	return matches
}

func buildHunksFromLCS(oldLines, newLines []string, lcs [][2]int) []*Hunk {
	const ctx = 3

	type editGroup struct {
		oldStart, oldEnd int
		newStart, newEnd int
	}

	var groups []editGroup

	if lcs[0][0] > 0 || lcs[0][1] > 0 {
		groups = append(groups, editGroup{
			oldStart: 0, oldEnd: lcs[0][0],
			newStart: 0, newEnd: lcs[0][1],
		})
	}

	for i := 1; i < len(lcs); i++ {
		os, ns := lcs[i-1][0]+1, lcs[i-1][1]+1
		oe, ne := lcs[i][0], lcs[i][1]
		if os < oe || ns < ne {
			groups = append(groups, editGroup{
				oldStart: os, oldEnd: oe,
				newStart: ns, newEnd: ne,
			})
		}
	}

	lastO, lastN := lcs[len(lcs)-1][0]+1, lcs[len(lcs)-1][1]+1
	if lastO < len(oldLines) || lastN < len(newLines) {
		groups = append(groups, editGroup{
			oldStart: lastO, oldEnd: len(oldLines),
			newStart: lastN, newEnd: len(newLines),
		})
	}

	if len(groups) == 0 {
		return nil
	}

	merged := []editGroup{groups[0]}
	for i := 1; i < len(groups); i++ {
		prev := &merged[len(merged)-1]
		cur := groups[i]
		gap := cur.oldStart - prev.oldEnd
		if gap < 2*ctx {
			prev.oldEnd = cur.oldEnd
			prev.newEnd = cur.newEnd
		} else {
			merged = append(merged, cur)
		}
	}

	var hunks []*Hunk
	for _, g := range merged {
		h := buildHunk(oldLines, newLines, lcs, g, ctx)
		hunks = append(hunks, h)
	}

	return hunks
}

func buildHunk(oldLines, newLines []string, lcs [][2]int, g struct{ oldStart, oldEnd, newStart, newEnd int }, ctxLines int) *Hunk {
	h := &Hunk{}

	ctxOldStart := max(g.oldStart-ctxLines, 0)
	ctxNewStart := max(g.newStart-ctxLines, 0)

	for _, m := range lcs {
		if m[0] < g.oldStart && m[0] >= g.oldStart-ctxLines && m[0] >= ctxOldStart {
			ctxOldStart = m[0]
			ctxNewStart = m[1]
			break
		}
	}

	ctxOldEnd := min(g.oldEnd+ctxLines, len(oldLines))
	ctxNewEnd := min(g.newEnd+ctxLines, len(newLines))

	for _, m := range lcs {
		if m[0] >= g.oldEnd && m[0] < ctxOldEnd {
			ctxOldEnd = m[0] + 1
			ctxNewEnd = m[1] + 1
			break
		}
	}

	relevantMatches := make([][2]int, 0, len(lcs))
	for _, m := range lcs {
		if m[0] >= ctxOldStart && m[0] < ctxOldEnd {
			relevantMatches = append(relevantMatches, m)
		}
	}

	oi := ctxOldStart
	ni := ctxNewStart

	for _, m := range relevantMatches {
		for oi < m[0] {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineRemoved,
				Content: oldLines[oi],
				OldLine: oi + 1,
				NewLine: 0,
			})
			oi++
		}
		for ni < m[1] {
			h.Lines = append(h.Lines, DiffLine{
				Type:    LineAdded,
				Content: newLines[ni],
				OldLine: 0,
				NewLine: ni + 1,
			})
			ni++
		}
		h.Lines = append(h.Lines, DiffLine{
			Type:    LineContext,
			Content: oldLines[oi],
			OldLine: oi + 1,
			NewLine: ni + 1,
		})
		oi++
		ni++
	}

	for oi < ctxOldEnd {
		h.Lines = append(h.Lines, DiffLine{
			Type:    LineRemoved,
			Content: oldLines[oi],
			OldLine: oi + 1,
			NewLine: 0,
		})
		oi++
	}
	for ni < ctxNewEnd {
		h.Lines = append(h.Lines, DiffLine{
			Type:    LineAdded,
			Content: newLines[ni],
			OldLine: 0,
			NewLine: ni + 1,
		})
		ni++
	}

	oldCount := 0
	newCount := 0
	for _, l := range h.Lines {
		switch l.Type {
		case LineContext:
			oldCount++
			newCount++
		case LineRemoved:
			oldCount++
		case LineAdded:
			newCount++
		}
	}

	h.OldStart = 0
	h.NewStart = 0
	for _, l := range h.Lines {
		if l.OldLine > 0 {
			h.OldStart = l.OldLine
			break
		}
	}
	for _, l := range h.Lines {
		if l.NewLine > 0 {
			h.NewStart = l.NewLine
			break
		}
	}

	h.OldCount = oldCount
	h.NewCount = newCount

	return h
}
