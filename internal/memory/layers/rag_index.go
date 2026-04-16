package layers

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"
)

// SparseVector represents a TF-IDF sparse vector for semantic matching.
type SparseVector struct {
	Terms map[string]float64
	Norm  float64
}

// RAGIndex provides semantic search over session history using TF-IDF vectors.
type RAGIndex struct {
	mu            sync.RWMutex
	documents     []RAGDocument
	termDocFreq   map[string]int
	totalDocs     int
	sessionSearch *SessionSearch
	codeIndexer   CodebaseIndexer
}

// RAGDocument represents an indexed document (session fragment).
type RAGDocument struct {
	ID        string
	SessionID string
	Role      string
	Content   string
	Timestamp time.Time
	Vector    *SparseVector
}

// RAGSearchResult is a semantic search result with relevance scoring.
type RAGSearchResult struct {
	Document RAGDocument `json:"document"`
	Score    float64     `json:"score"`
	Excerpt  string      `json:"excerpt"`
}

// NewRAGIndex creates a new RAG index backed by the session search layer.
func NewRAGIndex(ss *SessionSearch) *RAGIndex {
	return &RAGIndex{
		documents:     make([]RAGDocument, 0),
		termDocFreq:   make(map[string]int),
		sessionSearch: ss,
	}
}

// IndexSession indexes all messages from a session into the RAG index.
func (ri *RAGIndex) IndexSession(ctx context.Context, sessionID string) error {
	if ri.sessionSearch == nil {
		return fmt.Errorf("RAG index: session search not available")
	}

	fragments, err := ri.sessionSearch.Search(ctx, "", 100)
	if err != nil {
		return fmt.Errorf("RAG index: search failed: %w", err)
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	for _, frag := range fragments {
		if frag.SessionID != sessionID && sessionID != "" {
			continue
		}

		doc := RAGDocument{
			ID:        fmt.Sprintf("%s_%d", frag.SessionID, frag.SourceTurn),
			SessionID: frag.SessionID,
			Role:      frag.Role,
			Content:   frag.Content,
			Timestamp: frag.Timestamp,
		}

		vector := ri.computeTFIDF(doc.Content)
		doc.Vector = vector
		ri.documents = append(ri.documents, doc)
	}

	ri.recomputeDF()

	slog.Info("RAG index: indexed session", "session_id", sessionID, "documents", len(ri.documents))
	return nil
}

// IndexAll indexes all available session fragments.
func (ri *RAGIndex) IndexAll(ctx context.Context) error {
	if ri.sessionSearch == nil {
		return fmt.Errorf("RAG index: session search not available")
	}

	fragments, err := ri.sessionSearch.Search(ctx, "", 500)
	if err != nil {
		return fmt.Errorf("RAG index: search failed: %w", err)
	}

	ri.mu.Lock()
	defer ri.mu.Unlock()

	ri.documents = ri.documents[:0]
	ri.termDocFreq = make(map[string]int)

	for _, frag := range fragments {
		doc := RAGDocument{
			ID:        fmt.Sprintf("%s_%d", frag.SessionID, frag.SourceTurn),
			SessionID: frag.SessionID,
			Role:      frag.Role,
			Content:   frag.Content,
			Timestamp: frag.Timestamp,
		}
		vector := ri.computeTFIDF(doc.Content)
		doc.Vector = vector
		ri.documents = append(ri.documents, doc)
	}

	ri.recomputeDF()

	slog.Info("RAG index: indexed all sessions", "documents", len(ri.documents))
	return nil
}

// Search performs semantic search using TF-IDF cosine similarity.
func (ri *RAGIndex) Search(query string, maxResults int) []RAGSearchResult {
	if maxResults <= 0 {
		maxResults = 5
	}

	ri.mu.RLock()
	defer ri.mu.RUnlock()

	if len(ri.documents) == 0 {
		return nil
	}

	queryVector := ri.computeTFIDF(query)
	if queryVector.Norm == 0 {
		return nil
	}

	type scored struct {
		doc   RAGDocument
		score float64
	}

	var results []scored
	for _, doc := range ri.documents {
		if doc.Vector == nil || doc.Vector.Norm == 0 {
			continue
		}

		similarity := cosineSimilarity(queryVector, doc.Vector)
		if similarity > 0.01 {
			results = append(results, scored{doc: doc, score: similarity})
		}
	}

	// Sort by score descending
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	output := make([]RAGSearchResult, 0, len(results))
	for _, r := range results {
		excerpt := r.doc.Content
		if len(excerpt) > 300 {
			excerpt = excerpt[:300] + "..."
		}
		output = append(output, RAGSearchResult{
			Document: r.doc,
			Score:    r.score,
			Excerpt:  excerpt,
		})
	}

	return output
}

// DocumentCount returns the number of indexed documents.
func (ri *RAGIndex) DocumentCount() int {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return len(ri.documents)
}

// computeTFIDF computes a TF-IDF sparse vector for the given text.
func (ri *RAGIndex) computeTFIDF(text string) *SparseVector {
	terms := tokenize(text)
	if len(terms) == 0 {
		return &SparseVector{Terms: make(map[string]float64)}
	}

	// Term frequency
	tf := make(map[string]float64)
	for _, t := range terms {
		tf[t]++
	}
	totalTerms := float64(len(terms))
	for t, count := range tf {
		tf[t] = count / totalTerms
	}

	// TF-IDF
	vector := &SparseVector{Terms: make(map[string]float64)}
	var normSq float64

	ri.mu.RLock()
	for t, freq := range tf {
		idf := 1.0
		if ri.totalDocs > 0 {
			df := ri.termDocFreq[t]
			idf = math.Log(float64(1+ri.totalDocs) / float64(1+df))
		}
		score := freq * idf
		vector.Terms[t] = score
		normSq += score * score
	}
	ri.mu.RUnlock()

	vector.Norm = math.Sqrt(normSq)
	return vector
}

func (ri *RAGIndex) recomputeDF() {
	ri.termDocFreq = make(map[string]int)
	ri.totalDocs = len(ri.documents)

	for _, doc := range ri.documents {
		seen := make(map[string]bool)
		terms := tokenize(doc.Content)
		for _, t := range terms {
			if !seen[t] {
				ri.termDocFreq[t]++
				seen[t] = true
			}
		}
	}
}

func cosineSimilarity(a, b *SparseVector) float64 {
	if a.Norm == 0 || b.Norm == 0 {
		return 0
	}

	var dotProduct float64
	for term, weight := range a.Terms {
		if bWeight, ok := b.Terms[term]; ok {
			dotProduct += weight * bWeight
		}
	}

	return dotProduct / (a.Norm * b.Norm)
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, "_", " ")

	var tokens []string
	for _, word := range strings.Fields(text) {
		clean := strings.Trim(word, ".,;:!?()[]{}\"'/\\")
		if len(clean) > 2 {
			tokens = append(tokens, clean)
		}
	}
	return tokens
}

// FormatRAGResults formats RAG search results for injection into prompts.
func FormatRAGResults(results []RAGSearchResult, maxChars int) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Relevant Session History (Semantic Search)\n\n")

	usedChars := 0
	for i, r := range results {
		entry := fmt.Sprintf("- **[%.2f]** %s: %s\n", r.Score, r.Document.Role, r.Excerpt)
		if usedChars+len(entry) > maxChars {
			break
		}
		sb.WriteString(entry)
		usedChars += len(entry)

		if i >= 4 {
			break
		}
	}

	return sb.String()
}

// CodeSearchResult represents a code search result from the codebase index.
type CodeSearchResult struct {
	File    string  `json:"file"`
	Line    int     `json:"line"`
	EndLine int     `json:"end_line"`
	Content string  `json:"content"`
	Kind    string  `json:"kind"`
	Score   float64 `json:"score"`
}

// CodebaseIndexer is an interface for the codebase index backend.
type CodebaseIndexer interface {
	SearchCode(query string, maxResults int) []CodeSearchResult
}

// SearchCode delegates to a codebase index if available, returning code search results.
func (ri *RAGIndex) SearchCode(query string, maxResults int) []CodeSearchResult {
	if ri.codeIndexer != nil {
		return ri.codeIndexer.SearchCode(query, maxResults)
	}
	return nil
}

// SetCodeIndexer sets the codebase index backend for code search.
func (ri *RAGIndex) SetCodeIndexer(indexer CodebaseIndexer) {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	ri.codeIndexer = indexer
}
