package index

import (
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

const embeddingDims = 128

// GenerateEmbedding creates a fixed-dimension vector from text using
// a hash-based approach (FNV-64). Zero external dependencies, works offline.
func GenerateEmbedding(text string) []float64 {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return make([]float64, embeddingDims)
	}

	vec := make([]float64, embeddingDims)
	h := fnv.New64a()

	for _, token := range tokens {
		h.Reset()
		h.Write([]byte(token))
		hashVal := h.Sum64()

		for i := 0; i < embeddingDims; i++ {
			seed := hashVal ^ uint64(i)*0x9e3779b97f4a7c15
			seed = (seed >> 33) * 0xff51afd7ed558ccd
			seed = (seed >> 33) * 0xc4ceb9fe1a85ec53
			seed = seed >> 33

			bucket := float64(int64(seed)%4 - 1)
			vec[i] += bucket
		}
	}

	normalizeVector(vec)
	return vec
}

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ScoredItem represents a search result with a relevance score.
type ScoredItem struct {
	ID        string  `json:"id"`
	Score     float64 `json:"score"`
	File      string  `json:"file"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
	Content   string  `json:"content"`
	Kind      string  `json:"kind"`
}

// SimilaritySearch finds the top-K most similar items by embedding.
func SimilaritySearch(query []float64, candidates map[string][]float64, topK int) []ScoredItem {
	type scored struct {
		id    string
		score float64
	}

	var results []scored
	for id, candidate := range candidates {
		score := CosineSimilarity(query, candidate)
		if score > 0.01 {
			results = append(results, scored{id: id, score: score})
		}
	}

	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	output := make([]ScoredItem, len(results))
	for i, r := range results {
		output[i] = ScoredItem{
			ID:    r.id,
			Score: r.score,
		}
	}
	return output
}

// tokenize splits text into lowercase tokens suitable for indexing.
func tokenize(text string) []string {
	text = strings.ToLower(text)

	var sb strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune(' ')
		}
	}

	var tokens []string
	for _, word := range strings.Fields(sb.String()) {
		if len(word) > 2 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// normalizeVector scales a vector to unit length in-place.
func normalizeVector(vec []float64) {
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return
	}
	for i := range vec {
		vec[i] /= norm
	}
}
