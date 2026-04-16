package memory

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

func newTestMemoryManager(t *testing.T) *MemoryManager {
	t.Helper()
	dir := t.TempDir()
	return newTestMemoryManagerWithDir(t, dir)
}

func newTestMemoryManagerWithDir(t *testing.T, dir string) *MemoryManager {
	t.Helper()
	mm, err := NewMemoryManagerWithDir(dir)
	if err != nil {
		t.Fatalf("NewMemoryManagerWithDir error: %v", err)
	}
	t.Cleanup(func() { mm.Close() })
	return mm
}

func TestNewMemoryManagerWithDir(t *testing.T) {
	mm := newTestMemoryManager(t)
	if mm == nil {
		t.Fatal("expected non-nil MemoryManager")
	}
}

func TestNewMemoryManagerDefault(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	mm, err := NewMemoryManager()
	if err != nil {
		t.Fatalf("NewMemoryManager error: %v", err)
	}
	defer mm.Close()

	if mm == nil {
		t.Fatal("expected non-nil MemoryManager")
	}
}

func TestGetStore(t *testing.T) {
	mm := newTestMemoryManager(t)

	s := mm.GetStore()
	if s == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestGetPromptMemory(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	if pm == nil {
		t.Fatal("expected non-nil prompt memory")
	}
}

func TestGetIncidentMemory(t *testing.T) {
	mm := newTestMemoryManager(t)

	im := mm.GetIncidentMemory()
	if im == nil {
		t.Fatal("expected non-nil incident memory")
	}
}

func TestGetSessionSearch(t *testing.T) {
	mm := newTestMemoryManager(t)

	ss := mm.GetSessionSearch()
	if ss == nil {
		t.Fatal("expected non-nil session search")
	}
}

func TestGetSkillMemory(t *testing.T) {
	mm := newTestMemoryManager(t)

	sm := mm.GetSkillMemory()
	if sm == nil {
		t.Fatal("expected non-nil skill memory")
	}
}

func TestGetUserModel(t *testing.T) {
	mm := newTestMemoryManager(t)

	um := mm.GetUserModel()
	if um == nil {
		t.Fatal("expected non-nil user model")
	}
}

func TestGetSoulMD(t *testing.T) {
	mm := newTestMemoryManager(t)

	soul := mm.GetSoulMD()
	if soul == nil {
		t.Fatal("expected non-nil SOUL.md managed file")
	}
}

func TestGetAgentsMD(t *testing.T) {
	mm := newTestMemoryManager(t)

	agents := mm.GetAgentsMD()
	if agents == nil {
		t.Fatal("expected non-nil AGENTS.md managed file")
	}
}

func TestBuildSystemContextEmpty(t *testing.T) {
	mm := newTestMemoryManager(t)

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context (default templates are loaded)")
	}
}

func TestBuildSystemContextWithMemory(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("Test memory content")

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context with memory content")
	}
}

func TestBuildSystemContextWithQuery(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("Test memory")

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "test query")
	if result == "" {
		t.Fatal("expected non-empty context with query")
	}
}

func TestSetAndGetBudget(t *testing.T) {
	mm := newTestMemoryManager(t)

	customBudget := ContextBudget{
		MaxChars: 5000,
		Layers: []BudgetLayer{
			{Name: LayerMemory, Weight: 1.0, MinChars: 0, MaxChars: 5000},
		},
	}
	mm.SetBudget(customBudget)

	got := mm.GetBudget()
	if got.MaxChars != 5000 {
		t.Fatalf("expected MaxChars 5000, got %d", got.MaxChars)
	}
}

func TestContextAwareSkills(t *testing.T) {
	mm := newTestMemoryManager(t)

	if mm.IsContextAwareSkills() {
		t.Fatal("expected context-aware skills to be disabled by default")
	}

	mm.EnableContextAwareSkills(true)
	if !mm.IsContextAwareSkills() {
		t.Fatal("expected context-aware skills to be enabled")
	}

	mm.EnableContextAwareSkills(false)
	if mm.IsContextAwareSkills() {
		t.Fatal("expected context-aware skills to be disabled")
	}
}

func TestSnapshot(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("snapshot memory")
	pm.UpdateUserProfile("snapshot user")

	sessionID := "test-session-1"
	mm.FreezeSnapshot(sessionID)

	snap := mm.GetSnapshot(sessionID)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.MemoryContent != "snapshot memory" {
		t.Fatalf("expected memory content %q, got %q", "snapshot memory", snap.MemoryContent)
	}
	if snap.UserContent != "snapshot user" {
		t.Fatalf("expected user content %q, got %q", "snapshot user", snap.UserContent)
	}
}

func TestSnapshotNotOverwritten(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("first memory")

	sessionID := "test-session-2"
	mm.FreezeSnapshot(sessionID)

	pm.UpdateMemory("updated memory")
	mm.FreezeSnapshot(sessionID)

	snap := mm.GetSnapshot(sessionID)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.MemoryContent != "first memory" {
		t.Fatalf("expected original memory content, got %q", snap.MemoryContent)
	}
}

func TestClearSnapshot(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("temp memory")

	sessionID := "test-session-3"
	mm.FreezeSnapshot(sessionID)
	mm.ClearSnapshot(sessionID)

	snap := mm.GetSnapshot(sessionID)
	if snap != nil {
		t.Fatal("expected nil snapshot after clear")
	}
}

func TestGetSnapshotNonexistent(t *testing.T) {
	mm := newTestMemoryManager(t)

	snap := mm.GetSnapshot("nonexistent")
	if snap != nil {
		t.Fatal("expected nil for nonexistent snapshot")
	}
}

func TestBuildSystemContextWithSnapshot(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("frozen memory")
	pm.UpdateUserProfile("frozen user")

	sessionID := "test-session-4"
	mm.FreezeSnapshot(sessionID)

	pm.UpdateMemory("updated memory")
	pm.UpdateUserProfile("updated user")

	ctx := context.Background()
	result := mm.BuildSystemContextWithSnapshot(ctx, "", sessionID)
	if result == "" {
		t.Fatal("expected non-empty context with snapshot")
	}
}

func TestBuildSystemContextWithSnapshotMissing(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("current memory")

	ctx := context.Background()
	result := mm.BuildSystemContextWithSnapshot(ctx, "", "nonexistent-session")
	if result == "" {
		t.Fatal("expected non-empty context (falls back to live memory)")
	}
}

func TestReload(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("before reload")

	err := mm.Reload()
	if err != nil {
		t.Fatalf("Reload error: %v", err)
	}
}

func TestClose(t *testing.T) {
	dir := t.TempDir()
	mm, err := NewMemoryManagerWithDir(dir)
	if err != nil {
		t.Fatalf("NewMemoryManagerWithDir error: %v", err)
	}

	err = mm.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestForUserDefault(t *testing.T) {
	mm := newTestMemoryManager(t)

	um := mm.ForUser("default")
	if um == nil {
		t.Fatal("expected non-nil UserMemory")
	}
	if um.UserID() != "default" {
		t.Fatalf("expected UserID %q, got %q", "default", um.UserID())
	}
}

func TestForUserEmpty(t *testing.T) {
	mm := newTestMemoryManager(t)

	um := mm.ForUser("")
	if um == nil {
		t.Fatal("expected non-nil UserMemory")
	}
	if um.UserID() != "default" {
		t.Fatalf("expected UserID %q for empty user, got %q", "default", um.UserID())
	}
}

func TestForUserSpecific(t *testing.T) {
	mm := newTestMemoryManager(t)

	um := mm.ForUser("user-42")
	if um == nil {
		t.Fatal("expected non-nil UserMemory")
	}
	if um.UserID() != "user-42" {
		t.Fatalf("expected UserID %q, got %q", "user-42", um.UserID())
	}
}

func TestForUserGetPromptMemory(t *testing.T) {
	mm := newTestMemoryManager(t)

	um := mm.ForUser("user-1")
	pm := um.GetPromptMemory()
	if pm == nil {
		t.Fatal("expected non-nil prompt memory for user")
	}
}

func TestForUserBuildPrompt(t *testing.T) {
	mm := newTestMemoryManager(t)

	um := mm.ForUser("user-1")
	prompt := um.BuildPrompt()
	_ = prompt
}

func TestForUserBuildSystemContext(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("user context test")

	um := mm.ForUser("user-1")
	ctx := context.Background()
	result := um.BuildSystemContext(ctx, "test query")
	_ = result
}

func TestNewMemoryManagerWithComponents(t *testing.T) {
	dir := t.TempDir()
	pm, err := layers.NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir error: %v", err)
	}

	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir error: %v", err)
	}
	defer s.Close()

	sm := layers.NewSkillProceduralMemory(filepath.Join(dir, "skills"), nil)

	mm := NewMemoryManagerWithComponents(pm, s, sm)
	if mm == nil {
		t.Fatal("expected non-nil MemoryManager")
	}
	if mm.GetPromptMemory() != pm {
		t.Fatal("expected same prompt memory instance")
	}
	if mm.GetStore() != s {
		t.Fatal("expected same store instance")
	}
	if mm.GetSkillMemory() != sm {
		t.Fatal("expected same skill memory instance")
	}
}

func TestNewMemoryManagerWithComponentsNilStore(t *testing.T) {
	dir := t.TempDir()
	pm, err := layers.NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir error: %v", err)
	}

	mm := NewMemoryManagerWithComponents(pm, nil, nil)
	if mm == nil {
		t.Fatal("expected non-nil MemoryManager")
	}
	if mm.GetStore() != nil {
		t.Fatal("expected nil store")
	}
	if mm.GetSessionSearch() != nil {
		t.Fatal("expected nil session search when store is nil")
	}
}

func TestJoinParts(t *testing.T) {
	tests := []struct {
		name   string
		parts  []string
		expect string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"hello"}, "hello"},
		{"multiple", []string{"a", "b"}, "a\n\nb"},
		{"with_empty", []string{"a", "", "b"}, "a\n\nb"},
		{"all_empty", []string{"", ""}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinParts(tt.parts)
			if got != tt.expect {
				t.Fatalf("expected %q, got %q", tt.expect, got)
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name   string
		query  string
		expect int
	}{
		{"empty", "", 0},
		{"stop_words_only", "the a an is are", 0},
		{"mixed", "how to deploy the application", 3},
		{"with_punctuation", "Deploy, the application!", 2},
		{"long_query", "one two three four five six seven eight nine ten eleven twelve", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kw := extractKeywords(tt.query)
			if len(kw) != tt.expect {
				t.Fatalf("expected %d keywords, got %d: %v", tt.expect, len(kw), kw)
			}
		})
	}
}

func TestConcurrentSnapshotAccess(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("concurrent test")

	var wg sync.WaitGroup
	const count = 20

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "session-concurrent"
			if i%3 == 0 {
				mm.FreezeSnapshot(id)
			} else if i%3 == 1 {
				mm.GetSnapshot(id)
			} else {
				mm.ClearSnapshot(id)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentBuildSystemContext(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("concurrent context")

	var wg sync.WaitGroup
	const count = 10

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_ = mm.BuildSystemContext(ctx, "test query")
		}()
	}
	wg.Wait()
}

func TestBuildBundledSkillSummaries(t *testing.T) {
	summaries := buildBundledSkillSummaries()
	if len(summaries) == 0 {
		t.Fatal("expected non-empty bundled skill summaries")
	}

	for name, summary := range summaries {
		if summary.Name != name {
			t.Fatalf("expected summary name %q, got %q", name, summary.Name)
		}
		if summary.Description == "" {
			t.Fatalf("expected non-empty description for %q", name)
		}
		if summary.Source != "bundled" {
			t.Fatalf("expected source 'bundled' for %q, got %q", name, summary.Source)
		}
	}
}

func TestForUserSearch(t *testing.T) {
	mm := newTestMemoryManager(t)

	um := mm.ForUser("user-1")
	ctx := context.Background()
	_, err := um.Search(ctx, "test query", 5)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
}

func TestForUserGetUserModel(t *testing.T) {
	mm := newTestMemoryManager(t)

	um := mm.ForUser("user-1")
	uml := um.GetUserModel()
	if uml == nil {
		t.Fatal("expected non-nil user model for specific user")
	}
}

func TestBuildSystemContextWithSoulMD(t *testing.T) {
	dir := t.TempDir()
	mm := newTestMemoryManagerWithDir(t, dir)

	soulContent := "# My Soul\nI am a helpful assistant."
	soulPath := filepath.Join(dir, "SOUL.md")
	if err := os.WriteFile(soulPath, []byte(soulContent), 0644); err != nil {
		t.Fatal(err)
	}

	mm.GetSoulMD().Read()

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context with SOUL.md")
	}
}

func TestBuildSystemContextWithAgentsMD(t *testing.T) {
	dir := t.TempDir()
	mm := newTestMemoryManagerWithDir(t, dir)

	agentsContent := "# Agents\nAvailable agents list."
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte(agentsContent), 0644); err != nil {
		t.Fatal(err)
	}

	mm.GetAgentsMD().Read()

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context with AGENTS.md")
	}
}

func TestBudgetAllocationInContext(t *testing.T) {
	mm := newTestMemoryManager(t)

	pm := mm.GetPromptMemory()
	pm.UpdateMemory("Memory layer content for budget testing")

	customBudget := ContextBudget{
		MaxChars: 500,
		Layers: []BudgetLayer{
			{Name: LayerMemory, Weight: 0.3, MinChars: 0, MaxChars: 500},
			{Name: LayerUser, Weight: 0.3, MinChars: 0, MaxChars: 500},
			{Name: LayerSkills, Weight: 0.4, MinChars: 0, MaxChars: 500},
		},
	}
	mm.SetBudget(customBudget)

	ctx := context.Background()
	result := mm.BuildSystemContext(ctx, "")
	if result == "" {
		t.Fatal("expected non-empty context")
	}
}
