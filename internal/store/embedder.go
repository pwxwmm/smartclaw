package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strings"
	"sync"
	"unicode"
)

type Embedder interface {
	Embed(ctx context.Context, text string) (Vector, error)
	EmbedBatch(ctx context.Context, texts []string) ([]Vector, error)
	Dimensions() int
}

const hashEmbedderDims = 256

type HashEmbedder struct {
	mu     sync.Mutex
	idf    map[string]float64
	docCount int
}

func NewHashEmbedder() *HashEmbedder {
	return &HashEmbedder{
		idf: make(map[string]float64),
	}
}

func (h *HashEmbedder) Dimensions() int {
	return hashEmbedderDims
}

func (h *HashEmbedder) Embed(ctx context.Context, text string) (Vector, error) {
	tokens := tokenize(text)
	vec := make(Vector, hashEmbedderDims)

	h.mu.Lock()
	h.docCount++
	for _, tok := range tokens {
		h.idf[tok]++
	}
	h.mu.Unlock()

	tf := make(map[string]float64)
	for _, tok := range tokens {
		tf[tok]++
	}
	total := float64(len(tokens))
	if total == 0 {
		return vec, nil
	}

	for tok, count := range tf {
		h.mu.Lock()
		df := h.idf[tok]
		h.mu.Unlock()
		idf := math.Log1p(float64(h.docCount) / (df + 1))

		pos := hashToken(tok) % hashEmbedderDims
		weight := (count / total) * idf
		vec[pos] += weight

		pos2 := (hashToken(tok) >> 8) % hashEmbedderDims
		vec[pos2] += weight * 0.5
	}

	return Normalize(vec), nil
}

func (h *HashEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]Vector, error) {
	result := make([]Vector, len(texts))
	for i, text := range texts {
		v, err := h.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}

type APIEmbedder struct {
	url        string
	model      string
	apiKey     string
	dims       int
	fallback   *HashEmbedder
	httpClient *http.Client
}

func NewAPIEmbedder(url, model, apiKey string) *APIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &APIEmbedder{
		url:        url,
		model:      model,
		apiKey:     apiKey,
		dims:       1536,
		fallback:   NewHashEmbedder(),
		httpClient: &http.Client{},
	}
}

func (a *APIEmbedder) Dimensions() int {
	return a.dims
}

type embeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func (a *APIEmbedder) Embed(ctx context.Context, text string) (Vector, error) {
	vec, err := a.callAPI(ctx, text)
	if err != nil {
		slog.Warn("embedder: API call failed, using hash fallback", "error", err)
		return a.fallback.Embed(ctx, text)
	}
	return vec, nil
}

func (a *APIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([]Vector, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	results := make([]Vector, len(texts))
	for i, text := range texts {
		v, err := a.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		results[i] = v
	}
	return results, nil
}

func (a *APIEmbedder) callAPI(ctx context.Context, text string) (Vector, error) {
	reqBody := embeddingRequest{
		Input: text,
		Model: a.model,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+a.apiKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedder: API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedder: API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("embedder: decode response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("embedder: empty response")
	}

	vec := Vector(embResp.Data[0].Embedding)
	if a.dims != len(vec) {
		a.dims = len(vec)
	}
	return vec, nil
}

func NewDefaultEmbedder() Embedder {
	url := os.Getenv("SMARTCLAW_EMBEDDING_URL")
	model := os.Getenv("SMARTCLAW_EMBEDDING_MODEL")
	apiKey := os.Getenv("SMARTCLAW_EMBEDDING_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if url != "" {
		return NewAPIEmbedder(url, model, apiKey)
	}
	return NewHashEmbedder()
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else {
			if current.Len() > 1 {
				tokens = append(tokens, current.String())
			}
			current.Reset()
		}
	}
	if current.Len() > 1 {
		tokens = append(tokens, current.String())
	}

	var filtered []string
	for _, t := range tokens {
		if !isStopWord(t) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func isStopWord(w string) bool {
	stops := map[string]bool{
		"the": true, "and": true, "for": true, "are": true, "but": true,
		"not": true, "you": true, "all": true, "can": true, "had": true,
		"her": true, "was": true, "one": true, "our": true, "out": true,
		"has": true, "have": true, "this": true, "that": true, "with": true,
		"from": true, "they": true, "been": true, "will": true, "each": true,
		"make": true, "like": true, "some": true, "long": true, "very": true,
		"after": true, "just": true, "than": true, "them": true, "also": true,
	}
	return stops[w]
}

func hashToken(s string) int {
	h := fnvHash32(s)
	return int(h)
}

func fnvHash32(s string) uint32 {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return h
}
