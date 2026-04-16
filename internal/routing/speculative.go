package routing

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/api"
	"github.com/instructkr/smartclaw/internal/constants"
	"github.com/instructkr/smartclaw/internal/utils"
)

type SpeculativeResult struct {
	FastResult   *api.MessageResponse
	SlowResult   *api.MessageResponse
	UsedModel    string
	Similarity   float64
	FastDuration time.Duration
	SlowDuration time.Duration
}

type SpeculativeExecutor struct {
	primaryClient    *api.Client
	secondaryClient  *api.Client
	maxDiffThreshold float64
	enabled          bool
	mu               sync.Mutex
}

func NewSpeculativeExecutor(primary, secondary *api.Client) *SpeculativeExecutor {
	return &SpeculativeExecutor{
		primaryClient:    primary,
		secondaryClient:  secondary,
		maxDiffThreshold: constants.SpeculativeSimilarityThreshold,
		enabled:          false,
	}
}

func (se *SpeculativeExecutor) SetEnabled(enabled bool) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.enabled = enabled
}

func (se *SpeculativeExecutor) IsEnabled() bool {
	se.mu.Lock()
	defer se.mu.Unlock()
	return se.enabled
}

func (se *SpeculativeExecutor) SetThreshold(threshold float64) {
	se.mu.Lock()
	defer se.mu.Unlock()
	se.maxDiffThreshold = threshold
}

// ShouldSpeculate returns true for short queries with moderate complexity.
// Very simple queries don't need verification; very complex queries need the
// primary model alone.
func ShouldSpeculate(query string, complexityScore float64) bool {
	if len(query) > 500 {
		return false
	}
	if complexityScore < 0.2 {
		return false
	}
	if complexityScore > 0.5 {
		return false
	}
	return true
}

type modelResult struct {
	resp     *api.MessageResponse
	duration time.Duration
	err      error
}

func (se *SpeculativeExecutor) Execute(ctx context.Context, messages []api.Message, systemPrompt string) (*SpeculativeResult, error) {
	fastCh := make(chan modelResult, 1)
	slowCh := make(chan modelResult, 1)

	var wg sync.WaitGroup
	wg.Add(2)

	utils.Go(func() {
		defer wg.Done()
		start := time.Now()
		select {
		case <-ctx.Done():
			fastCh <- modelResult{err: ctx.Err()}
		default:
			resp, err := se.secondaryClient.CreateMessageWithSystem(ctx, messages, systemPrompt)
			fastCh <- modelResult{resp: resp, duration: time.Since(start), err: err}
		}
	})

	utils.Go(func() {
		defer wg.Done()
		start := time.Now()
		select {
		case <-ctx.Done():
			slowCh <- modelResult{err: ctx.Err()}
		default:
			resp, err := se.primaryClient.CreateMessageWithSystem(ctx, messages, systemPrompt)
			slowCh <- modelResult{resp: resp, duration: time.Since(start), err: err}
		}
	})

	fastResult := <-fastCh
	slowResult := <-slowCh
	wg.Wait()

	if fastResult.err != nil && slowResult.err != nil {
		return nil, fastResult.err
	}

	if fastResult.err != nil {
		return &SpeculativeResult{
			SlowResult:   slowResult.resp,
			UsedModel:    "slow",
			SlowDuration: slowResult.duration,
		}, nil
	}

	if slowResult.err != nil {
		return &SpeculativeResult{
			FastResult:   fastResult.resp,
			UsedModel:    "fast",
			FastDuration: fastResult.duration,
			Similarity:   1.0,
		}, nil
	}

	fastText := extractText(fastResult.resp)
	slowText := extractText(slowResult.resp)
	similarity := textSimilarity(fastText, slowText)

	result := &SpeculativeResult{
		FastResult:   fastResult.resp,
		SlowResult:   slowResult.resp,
		Similarity:   similarity,
		FastDuration: fastResult.duration,
		SlowDuration: slowResult.duration,
	}

	if similarity >= se.maxDiffThreshold {
		result.UsedModel = "fast"
		slog.Debug("speculative: using fast model", "similarity", similarity, "threshold", se.maxDiffThreshold)
	} else {
		result.UsedModel = "slow"
		slog.Debug("speculative: using slow model (low similarity)", "similarity", similarity, "threshold", se.maxDiffThreshold)
	}

	return result, nil
}

func extractText(resp *api.MessageResponse) string {
	if resp == nil {
		return ""
	}
	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}

// textSimilarity computes word overlap ratio between two strings.
func textSimilarity(a, b string) float64 {
	aWords := wordSet(a)
	bWords := wordSet(b)

	if len(aWords) == 0 && len(bWords) == 0 {
		return 1.0
	}
	if len(aWords) == 0 || len(bWords) == 0 {
		return 0.0
	}

	common := 0
	for w := range aWords {
		if bWords[w] {
			common++
		}
	}

	maxLen := len(aWords)
	if len(bWords) > maxLen {
		maxLen = len(bWords)
	}

	return float64(common) / float64(maxLen)
}

func wordSet(s string) map[string]bool {
	words := strings.Fields(strings.ToLower(s))
	set := make(map[string]bool, len(words))
	for _, w := range words {
		set[w] = true
	}
	return set
}
