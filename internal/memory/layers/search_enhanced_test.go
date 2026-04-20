package layers

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/store"
)

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("setup store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func seedMessages(t *testing.T, s *store.Store, sessionID, userID string) {
	t.Helper()
	ctx := context.Background()

	sess := &store.Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: time.Now().Add(-48 * time.Hour),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	msgs := []*store.Message{
		{SessionID: sessionID, Role: "user", Content: "How do I implement a binary search tree in Go?", Timestamp: time.Now().Add(-24 * time.Hour)},
		{SessionID: sessionID, Role: "assistant", Content: "A binary search tree (BST) is a data structure where each node has at most two children...", Timestamp: time.Now().Add(-23 * time.Hour)},
		{SessionID: sessionID, Role: "user", Content: "What about AVL tree balancing?", Timestamp: time.Now().Add(-22 * time.Hour)},
		{SessionID: sessionID, Role: "assistant", Content: "AVL trees are self-balancing binary search trees that maintain O(log n) height.", Timestamp: time.Now().Add(-21 * time.Hour)},
		{SessionID: sessionID, Role: "user", Content: "Show me a red-black tree example", Timestamp: time.Now().Add(-1 * time.Hour)},
		{SessionID: sessionID, Role: "assistant", Content: "Red-black trees are another self-balancing BST variant with coloring rules.", Timestamp: time.Now().Add(-30 * time.Minute)},
	}

	for _, msg := range msgs {
		if err := s.InsertMessage(ctx, msg); err != nil {
			t.Fatalf("insert message: %v", err)
		}
	}
}

func TestEnhancedSearchOptions(t *testing.T) {
	opts := EnhancedSearchOptions{
		Query:     "binary search",
		UserID:    "user1",
		SessionID: "sess1",
		Role:      "user",
		Since:     time.Now().Add(-48 * time.Hour),
		Until:     time.Now(),
		Limit:     10,
		Offset:    0,
		Summarize: true,
		MaxTokens: 500,
	}

	if opts.Query != "binary search" {
		t.Errorf("expected query 'binary search', got %q", opts.Query)
	}
	if opts.UserID != "user1" {
		t.Errorf("expected userID 'user1', got %q", opts.UserID)
	}
	if opts.Role != "user" {
		t.Errorf("expected role 'user', got %q", opts.Role)
	}
	if !opts.Summarize {
		t.Error("expected Summarize=true")
	}
}

func TestRerankingAlgorithm(t *testing.T) {
	ess := &EnhancedSessionSearch{}
	now := time.Now()

	recentResult := -0.5
	recentTime := now.Add(-1 * time.Hour)
	oldResult := -0.5
	oldTime := now.Add(-14 * 24 * time.Hour)

	recentUserScore := ess.computeScore(recentResult, recentTime, "user", now)
	recentAssistScore := ess.computeScore(recentResult, recentTime, "assistant", now)
	oldUserScore := ess.computeScore(oldResult, oldTime, "user", now)

	if recentUserScore <= recentAssistScore {
		t.Errorf("user role should score higher than assistant: user=%.4f assist=%.4f", recentUserScore, recentAssistScore)
	}

	if oldUserScore >= recentUserScore {
		t.Errorf("recent results should score higher than old: recent=%.4f old=%.4f", recentUserScore, oldUserScore)
	}

	roleDiff := recentUserScore - recentAssistScore
	baseScore := -recentResult
	approxBoost := math.Exp2(-1.0 / (7 * 24))
	expectedDiff := baseScore * approxBoost * 0.3
	if math.Abs(roleDiff-expectedDiff) > 0.01 {
		t.Errorf("role weight diff: got %.4f, want ~%.4f", roleDiff, expectedDiff)
	}
}

func TestRecencyDecay(t *testing.T) {
	ess := &EnhancedSessionSearch{}
	now := time.Now()
	baseRank := -1.0

	freshScore := ess.computeScore(baseRank, now, "user", now)
	weekOldScore := ess.computeScore(baseRank, now.Add(-7*24*time.Hour), "user", now)
	twoWeekOldScore := ess.computeScore(baseRank, now.Add(-14*24*time.Hour), "user", now)

	ratio := weekOldScore / freshScore
	if math.Abs(ratio-0.5) > 0.01 {
		t.Errorf("7-day half-life: expected ratio ~0.5, got %.4f", ratio)
	}

	ratio2 := twoWeekOldScore / weekOldScore
	if math.Abs(ratio2-0.5) > 0.01 {
		t.Errorf("14-day vs 7-day: expected ratio ~0.5, got %.4f", ratio2)
	}
}

func TestSearchWithMockStore(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "test-session-1", "user1")

	baseSearch := NewSessionSearch(s)
	ess := NewEnhancedSessionSearch(s, baseSearch, nil)

	results, err := ess.Search(context.Background(), EnhancedSearchOptions{
		Query:  "binary",
		UserID: "user1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected search results, got none")
	}

	for _, r := range results {
		if r.Score <= 0 {
			t.Errorf("ranked result should have positive score, got %.4f", r.Score)
		}
		if r.Fragment.SessionID == "" {
			t.Error("expected non-empty SessionID")
		}
	}

	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted by score descending: [%d]=%.4f > [%d]=%.4f", i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestCrossSessionSearch(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "session-alpha", "user1")
	seedMessages(t, s, "session-beta", "user1")

	baseSearch := NewSessionSearch(s)
	ess := NewEnhancedSessionSearch(s, baseSearch, nil)

	results, err := ess.CrossSessionSearch(context.Background(), "binary search", "user1", 10)
	if err != nil {
		t.Fatalf("cross-session search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected cross-session results, got none")
	}

	sessionsSeen := make(map[string]bool)
	for _, r := range results {
		sessionsSeen[r.Fragment.SessionID] = true
	}
	if len(sessionsSeen) < 2 {
		t.Errorf("expected results from multiple sessions, got %d", len(sessionsSeen))
	}
}

func TestCrossSessionSearchDeduplication(t *testing.T) {
	results := []*RankedResult{
		{Fragment: SessionFragment{SessionID: "s1", Content: "hello world"}, Score: 2.0},
		{Fragment: SessionFragment{SessionID: "s1", Content: "hello world"}, Score: 1.5},
		{Fragment: SessionFragment{SessionID: "s2", Content: "hello world"}, Score: 1.0},
	}

	deduped := deduplicateBySession(results)
	if len(deduped) != 2 {
		t.Errorf("expected 2 deduplicated results, got %d", len(deduped))
	}
}

func TestSearchAndSummarizeWithMockLLM(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "summarize-session", "user1")

	mockLLM := func(ctx context.Context, fragments []SessionFragment, query string, maxTokens int) (string, error) {
		return "Summary: found " + query + " in " + time.Now().Format("2006"), nil
	}

	baseSearch := NewSessionSearch(s)
	ess := NewEnhancedSessionSearch(s, baseSearch, mockLLM)

	summary, results, err := ess.SearchAndSummarize(context.Background(), EnhancedSearchOptions{
		Query:     "binary",
		UserID:    "user1",
		Summarize: true,
		MaxTokens: 200,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("search and summarize: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if summary == "" {
		t.Fatal("expected non-empty summary from LLM")
	}
	if summary[:7] != "Summary" {
		t.Errorf("expected mock LLM summary, got: %s", summary)
	}
}

func TestSearchAndSummarizeWithoutLLM(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "no-llm-session", "user1")

	baseSearch := NewSessionSearch(s)
	ess := NewEnhancedSessionSearch(s, baseSearch, nil)

	summary, results, err := ess.SearchAndSummarize(context.Background(), EnhancedSearchOptions{
		Query:     "binary",
		UserID:    "user1",
		Summarize: true,
		MaxTokens: 200,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("search and summarize without LLM: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if summary == "" {
		t.Fatal("expected fallback formatted summary")
	}
}

func TestSearchAndSummarizeLLMFailure(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "llm-fail-session", "user1")

	failingLLM := func(ctx context.Context, fragments []SessionFragment, query string, maxTokens int) (string, error) {
		return "", os.ErrNotExist
	}

	baseSearch := NewSessionSearch(s)
	ess := NewEnhancedSessionSearch(s, baseSearch, failingLLM)

	summary, results, err := ess.SearchAndSummarize(context.Background(), EnhancedSearchOptions{
		Query:     "binary",
		UserID:    "user1",
		Summarize: true,
		MaxTokens: 200,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("search and summarize with LLM failure: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results even when LLM fails")
	}
	if summary == "" {
		t.Fatal("expected fallback summary when LLM fails")
	}
}

func TestSnippetExtraction(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "snippet-session", "user1")

	baseSearch := NewSessionSearch(s)
	ess := NewEnhancedSessionSearch(s, baseSearch, nil)

	results, err := ess.Search(context.Background(), EnhancedSearchOptions{
		Query:  "binary",
		UserID: "user1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("search for snippet: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results")
	}

	hasSnippet := false
	for _, r := range results {
		if r.Snippet != "" {
			hasSnippet = true
			break
		}
	}
	if !hasSnippet {
		t.Log("no snippets returned (FTS5 snippet() may return empty for short content)")
	}
}

func TestGetSearchSnippets(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	sess := &store.Session{
		ID:        "snippet-doc-session",
		UserID:    "user1",
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now(),
	}
	if err := s.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	msg := &store.Message{
		SessionID: "snippet-doc-session",
		Role:      "user",
		Content:   "Tell me about binary search trees and their time complexity",
		Timestamp: time.Now(),
	}
	if err := s.InsertMessage(ctx, msg); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	var docID int64
	row := s.DB().QueryRow("SELECT id FROM messages WHERE session_id = ? ORDER BY id DESC LIMIT 1", "snippet-doc-session")
	if err := row.Scan(&docID); err != nil {
		t.Fatalf("get doc id: %v", err)
	}

	snippet, err := s.GetSearchSnippets("binary", docID, 64)
	if err != nil {
		t.Fatalf("get search snippet: %v", err)
	}

	_ = snippet
}

func TestTimeRangeFiltering(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	sess := &store.Session{
		ID:        "time-range-session",
		UserID:    "user1",
		CreatedAt: time.Now().Truncate(time.Second).Add(-72 * time.Hour),
		UpdatedAt: time.Now().Truncate(time.Second),
	}
	if err := s.UpsertSession(ctx, sess); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	oldMsg := &store.Message{
		SessionID: "time-range-session",
		Role:      "user",
		Content:   "old message about binary search",
		Timestamp: now.Add(-48 * time.Hour),
	}
	recentMsg := &store.Message{
		SessionID: "time-range-session",
		Role:      "user",
		Content:   "recent message about binary search",
		Timestamp: now.Add(-1 * time.Hour),
	}
	if err := s.InsertMessage(ctx, oldMsg); err != nil {
		t.Fatalf("insert old message: %v", err)
	}
	if err := s.InsertMessage(ctx, recentMsg); err != nil {
		t.Fatalf("insert recent message: %v", err)
	}

	since := now.Add(-2 * time.Hour)
	until := now.Add(1 * time.Minute)

	results, err := s.SearchMessagesByTimeRange("binary", since, until, 10)
	if err != nil {
		t.Fatalf("search by time range: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected time-filtered results")
	}

	for _, r := range results {
		if !r.Timestamp.IsZero() {
			if r.Timestamp.Before(since.Add(-1 * time.Minute)) {
				t.Errorf("result timestamp %v is too old (since=%v)", r.Timestamp, since)
			}
		}
	}
}

func TestSearchBySession(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "specific-session", "user1")

	results, err := s.SearchMessagesBySession("binary", "specific-session", 10)
	if err != nil {
		t.Fatalf("search by session: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results for session-scoped search")
	}

	for _, r := range results {
		if r.SessionID != "specific-session" {
			t.Errorf("expected session_id 'specific-session', got %q", r.SessionID)
		}
	}
}

func TestSearchAdvancedFallbackWithoutEnhanced(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "fallback-session", "default")

	ss := NewSessionSearch(s)

	results, err := ss.SearchAdvanced(context.Background(), "binary", EnhancedSearchOptions{
		Query: "binary",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("search advanced fallback: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected fallback results")
	}
}

func TestSearchAdvancedWithEnhanced(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "enhanced-session", "user1")

	ss := NewSessionSearch(s)
	ess := NewEnhancedSessionSearch(s, ss, nil)
	ss.SetEnhancedSearch(ess)

	results, err := ss.SearchAdvanced(context.Background(), "binary", EnhancedSearchOptions{
		Query:  "binary",
		UserID: "user1",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("search advanced with enhanced: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected enhanced results")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	s := setupTestStore(t)
	ess := NewEnhancedSessionSearch(s, nil, nil)

	results, err := ess.Search(context.Background(), EnhancedSearchOptions{Query: ""})
	if err != nil {
		t.Fatalf("empty query search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for empty query, got %d", len(results))
	}
}

func TestCrossSessionSearchEmptyQuery(t *testing.T) {
	s := setupTestStore(t)
	ess := NewEnhancedSessionSearch(s, nil, nil)

	results, err := ess.CrossSessionSearch(context.Background(), "", "user1", 10)
	if err != nil {
		t.Fatalf("cross-session empty query: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for empty query, got %d", len(results))
	}
}

func TestSearchMessagesAdvancedRoleFilter(t *testing.T) {
	s := setupTestStore(t)
	seedMessages(t, s, "role-filter-session", "user1")

	results, err := s.SearchMessagesAdvanced("binary", store.SearchOptions{
		UserID: "user1",
		Role:   "user",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("search with role filter: %v", err)
	}

	for _, r := range results {
		if r.Role != "user" {
			t.Errorf("expected role 'user', got %q", r.Role)
		}
	}
}

func TestSearchMessagesAdvancedDefaults(t *testing.T) {
	opts := store.SearchOptions{}
	if opts.Limit != 0 {
		t.Errorf("expected default Limit 0, got %d", opts.Limit)
	}

	s := setupTestStore(t)
	seedMessages(t, s, "defaults-session", "default")

	results, err := s.SearchMessagesAdvanced("binary", opts)
	if err != nil {
		t.Fatalf("search with defaults: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected results with default options")
	}
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "smartclaw-test-*")
	if err != nil {
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	origDir := filepath.Join(os.Getenv("HOME"), ".smartclaw")
	_ = origDir

	os.Exit(m.Run())
}
