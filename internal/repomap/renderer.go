package repomap

import (
	"sort"
	"strings"
)

const (
	charsPerToken = 4
	maxSigLen     = 120
)

// Render produces a ranked, token-bounded summary of the codebase.
//
// Files are sorted by rank descending. Within each file, symbols are listed as
// "kind name signature" lines. Signatures exceeding maxSigLen chars are truncated.
// Output stops when approximately maxTokens worth of characters have been emitted.
func Render(ranks map[string]float64, symbols map[string][]Symbol, maxTokens int) string {
	if len(ranks) == 0 || len(symbols) == 0 {
		return ""
	}

	maxChars := maxTokens * charsPerToken

	// Sort files by rank descending, break ties by path for determinism
	type fileRank struct {
		path string
		rank float64
	}
	files := make([]fileRank, 0, len(ranks))
	for path, rank := range ranks {
		if _, ok := symbols[path]; ok {
			files = append(files, fileRank{path: path, rank: rank})
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].rank != files[j].rank {
			return files[i].rank > files[j].rank
		}
		return files[i].path < files[j].path
	})

	var b strings.Builder
	chars := 0

	for _, fr := range files {
		syms := symbols[fr.path]

		header := fr.path + ":\n"
		if chars+len(header) > maxChars {
			break
		}
		b.WriteString(header)
		chars += len(header)

		sort.Slice(syms, func(i, j int) bool {
			return syms[i].Line < syms[j].Line
		})

		for _, s := range syms {
			sig := s.Signature
			if len(sig) > maxSigLen {
				sig = sig[:maxSigLen-3] + "..."
			}
			line := "  " + s.Kind + " " + s.Name + " " + sig + "\n"
			if chars+len(line) > maxChars {
				return b.String()
			}
			b.WriteString(line)
			chars += len(line)
		}

		b.WriteString("\n")
		chars++
	}

	return b.String()
}
