package contextmgr

import (
	"context"
	"testing"
	"time"
)

func TestRelevanceScorer_Recency_NewerIsHigher(t *testing.T) {
	rs := NewRelevanceScorer()
	ctx := context.Background()

	old := ContextItem{
		Source:    "files",
		Type:     "file",
		Content:  "old content database query",
		Timestamp: time.Now().Add(-1 * time.Hour),
	}
	recent := ContextItem{
		Source:    "files",
		Type:     "file",
		Content:  "recent content database query",
		Timestamp: time.Now().Add(-1 * time.Minute),
	}

	oldScore := rs.Score(ctx, old, "database query")
	recentScore := rs.Score(ctx, recent, "database query")

	if recentScore <= oldScore {
		t.Errorf("expected recent score > old score, got recent=%.4f old=%.4f", recentScore, oldScore)
	}
}

func TestRelevanceScorer_Recency_ZeroTimestamp(t *testing.T) {
	rs := NewRelevanceScorer()
	ctx := context.Background()

	item := ContextItem{
		Source:   "files",
		Type:    "file",
		Content: "test content",
	}

	score := rs.Score(ctx, item, "test")
	if score <= 0 {
		t.Errorf("zero-timestamp item should still get a score, got %.4f", score)
	}
}

func TestRelevanceScorer_Frequency(t *testing.T) {
	rs := NewRelevanceScorer()
	ctx := context.Background()

	item := ContextItem{
		Source:   "files",
		Type:    "file",
		Content: "content with keyword",
		FilePath: "main.go",
	}

	rs.RecordSeen("main.go:file")
	rs.RecordSeen("main.go:file")
	rs.RecordSeen("main.go:file")

	scoreWithFreq := rs.Score(ctx, item, "keyword")

	rs2 := NewRelevanceScorer()
	scoreWithoutFreq := rs2.Score(ctx, item, "keyword")

	if scoreWithFreq <= scoreWithoutFreq {
		t.Errorf("expected frequency to boost score, got with=%.4f without=%.4f",
			scoreWithFreq, scoreWithoutFreq)
	}
}

func TestRelevanceScorer_KeywordOverlap(t *testing.T) {
	rs := NewRelevanceScorer()
	ctx := context.Background()

	highOverlap := ContextItem{
		Source:   "files",
		Type:    "file",
		Content: "database connection pool configuration",
	}
	lowOverlap := ContextItem{
		Source:   "files",
		Type:    "file",
		Content: "unrelated stuff about weather",
	}

	query := "database connection pool"

	highScore := rs.Score(ctx, highOverlap, query)
	lowScore := rs.Score(ctx, lowOverlap, query)

	if highScore <= lowScore {
		t.Errorf("expected high overlap > low overlap, got high=%.4f low=%.4f", highScore, lowScore)
	}
}

func TestRelevanceScorer_KeywordOverlap_EmptyQuery(t *testing.T) {
	rs := NewRelevanceScorer()
	ctx := context.Background()

	item := ContextItem{
		Source:   "files",
		Type:    "file",
		Content: "some content",
	}

	score := rs.Score(ctx, item, "")
	if score < 0 {
		t.Errorf("score should be non-negative for empty query, got %.4f", score)
	}
}

func TestRelevanceScorer_TypeScore(t *testing.T) {
	rs := NewRelevanceScorer()
	ctx := context.Background()

	symbol := ContextItem{Source: "symbols", Type: "symbol", Content: "test", Timestamp: time.Now()}
	file := ContextItem{Source: "files", Type: "file", Content: "test", Timestamp: time.Now()}
	memory := ContextItem{Source: "memory", Type: "memory", Content: "test", Timestamp: time.Now()}
	gitDiff := ContextItem{Source: "git", Type: "git_diff", Content: "test", Timestamp: time.Now()}

	symbolScore := rs.Score(ctx, symbol, "test")
	fileScore := rs.Score(ctx, file, "test")
	memoryScore := rs.Score(ctx, memory, "test")
	gitScore := rs.Score(ctx, gitDiff, "test")

	if symbolScore <= fileScore {
		t.Errorf("symbol type should score higher than file, got symbol=%.4f file=%.4f", symbolScore, fileScore)
	}
	if fileScore <= memoryScore {
		t.Errorf("file type should score higher than memory, got file=%.4f memory=%.4f", fileScore, memoryScore)
	}
	if memoryScore <= gitScore {
		t.Errorf("memory type should score higher than git_diff, got memory=%.4f git=%.4f", memoryScore, gitScore)
	}
}

func TestScoreItems_ReturnsSorted(t *testing.T) {
	rs := NewRelevanceScorer()
	ctx := context.Background()

	items := []ContextItem{
		{Source: "git", Type: "git_diff", Content: "unrelated", Timestamp: time.Now()},
		{Source: "symbols", Type: "symbol", Content: "database connection pool", Timestamp: time.Now()},
		{Source: "files", Type: "file", Content: "database setup", Timestamp: time.Now()},
	}

	scored := rs.ScoreItems(ctx, items, "database connection pool")

	for i := 1; i < len(scored); i++ {
		if scored[i].Relevance > scored[i-1].Relevance {
			t.Errorf("ScoreItems not sorted descending: [%d]=%.4f > [%d]=%.4f",
				i, scored[i].Relevance, i-1, scored[i-1].Relevance)
		}
	}
}

func TestScoreItems_Empty(t *testing.T) {
	rs := NewRelevanceScorer()
	ctx := context.Background()

	scored := rs.ScoreItems(ctx, nil, "query")
	if len(scored) != 0 {
		t.Errorf("expected 0 items, got %d", len(scored))
	}
}
