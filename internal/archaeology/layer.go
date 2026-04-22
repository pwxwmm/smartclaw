package archaeology

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/instructkr/smartclaw/internal/git"
)

var filePathRe = regexp.MustCompile(`(?:^|[\s'"(])([\w]+(?:/[\w.-]+)+\.go)(?:$|[\s'")\]},;])`)

type ArchaeologyLayer struct {
	rootDir string
}

func NewArchaeologyLayer(rootDir string) *ArchaeologyLayer {
	return &ArchaeologyLayer{rootDir: rootDir}
}

func (al *ArchaeologyLayer) BuildArchaeologyPrompt(ctx context.Context, query string) string {
	files := extractFilePaths(query)
	if len(files) == 0 {
		return ""
	}
	if len(files) > 3 {
		files = files[:3]
	}

	var parts []string
	for _, file := range files {
		entry := al.buildFileEntry(file)
		if entry != "" {
			parts = append(parts, entry)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	result := strings.Join(parts, "\n")
	if len(result) > 800 {
		result = result[:797] + "..."
	}
	return result
}

func (al *ArchaeologyLayer) buildFileEntry(file string) string {
	blameInfos, blameErr := git.GetBlame(al.rootDir, file, 0)
	logEntries, logErr := git.GetFileLog(al.rootDir, file, 10)

	if blameErr != nil && logErr != nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Code Archaeology for %s:\n", file))

	if len(logEntries) > 0 {
		latest := logEntries[0]
		date := truncateDate(latest.Date)
		sb.WriteString(fmt.Sprintf("  Last modified: %s by %s (%s)\n", date, latest.Author, latest.Hash))
	}

	if len(logEntries) > 1 {
		authorCounts := make(map[string]int)
		for _, e := range logEntries {
			authorCounts[e.Author]++
		}
		sb.WriteString(fmt.Sprintf("  Recent history: %d commits, authors: %s\n", len(logEntries), formatAuthorCounts(authorCounts)))
	}

	if len(logEntries) > 0 {
		var keyCommits []string
		limit := 2
		if len(logEntries) < limit {
			limit = len(logEntries)
		}
		for i := 0; i < limit; i++ {
			keyCommits = append(keyCommits, fmt.Sprintf("\"%s\" (%s)", logEntries[i].Subject, logEntries[i].Hash))
		}
		sb.WriteString(fmt.Sprintf("  Key commits: %s\n", strings.Join(keyCommits, ", ")))
	}

	if len(blameInfos) > 0 {
		authorLines := make(map[string]int)
		for _, b := range blameInfos {
			authorLines[b.Author]++
		}
		total := len(blameInfos)
		sb.WriteString(fmt.Sprintf("  Ownership: %s\n", formatOwnership(authorLines, total)))
	}

	return sb.String()
}

func extractFilePaths(query string) []string {
	matches := filePathRe.FindAllStringSubmatch(query, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		f := m[1]
		if !seen[f] {
			seen[f] = true
			result = append(result, f)
		}
	}
	return result
}

func formatAuthorCounts(counts map[string]int) string {
	type kv struct {
		name  string
		count int
	}
	var entries []kv
	for k, v := range counts {
		entries = append(entries, kv{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})
	var parts []string
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf("%s(%d)", e.name, e.count))
	}
	return strings.Join(parts, ", ")
}

func formatOwnership(authorLines map[string]int, total int) string {
	type kv struct {
		name   string
		lines  int
	}
	var entries []kv
	for k, v := range authorLines {
		entries = append(entries, kv{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].lines > entries[j].lines
	})
	var parts []string
	for _, e := range entries {
		pct := e.lines * 100 / total
		parts = append(parts, fmt.Sprintf("%d%% %s", pct, e.name))
	}
	return strings.Join(parts, ", ")
}

func truncateDate(date string) string {
	if len(date) >= 10 {
		return date[:10]
	}
	return date
}
