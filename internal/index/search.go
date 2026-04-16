package index

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type SearchQuery struct {
	Query      string `json:"query"`
	Type       string `json:"type"`
	Language   string `json:"language"`
	MaxResults int    `json:"max_results"`
	Path       string `json:"path,omitempty"`
}

type SearchResult struct {
	Items    []ScoredItem  `json:"items"`
	Total    int           `json:"total"`
	Duration time.Duration `json:"duration"`
}

const (
	keywordWeight  = 0.6
	semanticWeight = 0.4
)

// Search performs hybrid search combining BM25 keyword matching with
// semantic embedding similarity.
func (idx *CodebaseIndex) Search(_ context.Context, query SearchQuery) (*SearchResult, error) {
	start := time.Now()

	if query.MaxResults <= 0 {
		query.MaxResults = 20
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(idx.chunks) == 0 {
		return &SearchResult{Duration: time.Since(start)}, nil
	}

	queryEmbedding := GenerateEmbedding(query.Query)

	type candidate struct {
		chunk     *Chunk
		bm25Score float64
		semScore  float64
	}

	candidates := make(map[string]*candidate)
	for id, chunk := range idx.chunks {
		if query.Type != "" && chunk.Type != query.Type {
			continue
		}
		if query.Path != "" && !strings.HasPrefix(chunk.File, query.Path) {
			continue
		}
		if query.Language != "" {
			fi, exists := idx.files[chunk.File]
			if !exists || fi.Language != query.Language {
				continue
			}
		}
		candidates[id] = &candidate{chunk: chunk}
	}

	queryTerms := tokenize(query.Query)
	if len(queryTerms) > 0 {
		for _, cand := range candidates {
			cand.bm25Score = idx.bm25Score(cand.chunk.Content, queryTerms)
		}
	}

	for _, cand := range candidates {
		if len(cand.chunk.Embedding) > 0 {
			cand.semScore = CosineSimilarity(queryEmbedding, cand.chunk.Embedding)
		}
	}

	var maxBM25, maxSem float64
	for _, cand := range candidates {
		if cand.bm25Score > maxBM25 {
			maxBM25 = cand.bm25Score
		}
		if cand.semScore > maxSem {
			maxSem = cand.semScore
		}
	}

	type scored struct {
		id    string
		score float64
		chunk *Chunk
	}

	var results []scored
	for id, cand := range candidates {
		normBM25 := float64(0)
		if maxBM25 > 0 {
			normBM25 = cand.bm25Score / maxBM25
		}
		normSem := float64(0)
		if maxSem > 0 {
			normSem = cand.semScore / maxSem
		}

		finalScore := keywordWeight*normBM25 + semanticWeight*normSem
		if finalScore > 0.01 {
			results = append(results, scored{id: id, score: finalScore, chunk: cand.chunk})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > query.MaxResults {
		results = results[:query.MaxResults]
	}

	items := make([]ScoredItem, len(results))
	for i, r := range results {
		kind := r.chunk.Type
		if r.chunk.SymbolRef != "" {
			if sym, ok := idx.symbols[r.chunk.SymbolRef]; ok {
				kind = sym.Kind
			}
		}
		items[i] = ScoredItem{
			ID:        r.id,
			Score:     r.score,
			File:      r.chunk.File,
			StartLine: r.chunk.StartLine,
			EndLine:   r.chunk.EndLine,
			Content:   truncateContent(r.chunk.Content, 500),
			Kind:      kind,
		}
	}

	return &SearchResult{
		Items:    items,
		Total:    len(results),
		Duration: time.Since(start),
	}, nil
}

// SearchSymbols searches for symbols by name and kind.
func (idx *CodebaseIndex) SearchSymbols(_ context.Context, query string, kind string, limit int) ([]*Symbol, error) {
	if limit <= 0 {
		limit = 20
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	queryLower := strings.ToLower(query)
	queryEmbedding := GenerateEmbedding(query)

	type scoredSymbol struct {
		sym   *Symbol
		score float64
	}

	var results []scoredSymbol
	for _, sym := range idx.symbols {
		if kind != "" && sym.Kind != kind {
			continue
		}

		nameLower := strings.ToLower(sym.Name)
		qualLower := strings.ToLower(sym.QualifiedName)

		var score float64

		if nameLower == queryLower {
			score = 1.0
		} else if strings.HasPrefix(nameLower, queryLower) {
			score = 0.8
		} else if strings.Contains(nameLower, queryLower) {
			score = 0.6
		} else if strings.Contains(qualLower, queryLower) {
			score = 0.4
		}

		if len(sym.Embedding) > 0 {
			semScore := CosineSimilarity(queryEmbedding, sym.Embedding)
			score = score*0.7 + semScore*0.3
		}

		if score > 0.05 {
			results = append(results, scoredSymbol{sym: sym, score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	symbols := make([]*Symbol, len(results))
	for i, r := range results {
		symbols[i] = r.sym
	}
	return symbols, nil
}

// GetFileContext returns relevant context from a file (imports, types, function signatures).
func (idx *CodebaseIndex) GetFileContext(_ context.Context, filePath string, maxTokens int) string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if maxTokens <= 0 {
		maxTokens = 4000
	}

	var sb strings.Builder
	approxTokens := 0

	fi, exists := idx.files[filePath]
	if !exists {
		return ""
	}

	symbols := make([]*Symbol, 0)
	for _, sym := range idx.symbols {
		if sym.File == filePath {
			symbols = append(symbols, sym)
		}
	}
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].Line < symbols[j].Line
	})

	sb.WriteString("// File: " + filePath + " (" + fi.Language + ", " + fmt.Sprintf("%d", fi.LinesCount) + " lines)\n")
	approxTokens += 10

	for _, sym := range symbols {
		if !sym.Exported && sym.Kind != "function" && sym.Kind != "method" {
			continue
		}

		entry := sym.Signature + "\n"
		if sym.DocComment != "" {
			entry = "// " + sym.DocComment + "\n" + entry
		}

		if approxTokens+len(entry)/4 > maxTokens {
			break
		}

		sb.WriteString(entry)
		approxTokens += len(entry) / 4
	}

	return sb.String()
}

func (idx *CodebaseIndex) bm25Score(docContent string, queryTerms []string) float64 {
	k1 := 1.5
	b := 0.75

	docTerms := tokenize(docContent)
	if len(docTerms) == 0 {
		return 0
	}

	avgDL := idx.averageDocLength()
	dl := float64(len(docTerms))

	tf := make(map[string]float64)
	for _, t := range docTerms {
		tf[t]++
	}

	var score float64
	for _, term := range queryTerms {
		freq, exists := tf[term]
		if !exists {
			continue
		}

		df := idx.docFreq[term]
		idf := math.Log((float64(idx.totalDocs-df) + 0.5) / (float64(df) + 0.5))
		if idf < 0 {
			idf = 0
		}

		tfNorm := (freq * (k1 + 1)) / (freq + k1*(1-b+b*dl/avgDL))
		score += idf * tfNorm
	}

	return score
}

func (idx *CodebaseIndex) averageDocLength() float64 {
	if len(idx.chunks) == 0 {
		return 1
	}
	var total float64
	for _, chunk := range idx.chunks {
		total += float64(len(tokenize(chunk.Content)))
	}
	return total / float64(len(idx.chunks))
}

func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}
