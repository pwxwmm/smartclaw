package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewContextFileResolver(t *testing.T) {
	dir := t.TempDir()
	config := DefaultContextFileConfig()

	r := NewContextFileResolver(dir, config)

	if r == nil {
		t.Fatal("expected non-nil resolver")
	}
	if config.HeadPercent != 70 {
		t.Errorf("expected HeadPercent=70, got %d", config.HeadPercent)
	}
	if config.TailPercent != 20 {
		t.Errorf("expected TailPercent=20, got %d", config.TailPercent)
	}
	if !config.MiddleSkip {
		t.Error("expected MiddleSkip=true")
	}

	soulFile := r.GetFile(PrioritySoul)
	if soulFile == nil {
		t.Fatal("expected SOUL file entry")
	}
	if soulFile.Path != filepath.Join(dir, "SOUL.md") {
		t.Errorf("unexpected SOUL path: %s", soulFile.Path)
	}
	if soulFile.Loaded {
		t.Error("expected SOUL not loaded yet")
	}
}

func TestLoadAllFiles(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("soul content"), 0644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents content"), 0644)
	os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("cursor rules"), 0644)

	if err := r.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	soul := r.GetFile(PrioritySoul)
	if !soul.Loaded || soul.Content != "soul content" {
		t.Errorf("SOUL not loaded correctly: loaded=%v content=%q", soul.Loaded, soul.Content)
	}

	agents := r.GetFile(PriorityAgents)
	if !agents.Loaded || agents.Content != "agents content" {
		t.Errorf("AGENTS not loaded correctly: loaded=%v content=%q", agents.Loaded, agents.Content)
	}

	cursor := r.GetFile(PriorityCursorRules)
	if !cursor.Loaded || cursor.Content != "cursor rules" {
		t.Errorf(".cursorrules not loaded correctly: loaded=%v content=%q", cursor.Loaded, cursor.Content)
	}
}

func TestLoadOnlySoulMD(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("only soul"), 0644)

	if err := r.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	soul := r.GetFile(PrioritySoul)
	if !soul.Loaded || soul.Content != "only soul" {
		t.Errorf("SOUL not loaded: loaded=%v content=%q", soul.Loaded, soul.Content)
	}

	agents := r.GetFile(PriorityAgents)
	if agents.Loaded {
		t.Error("AGENTS should not be loaded when file doesn't exist")
	}

	cursor := r.GetFile(PriorityCursorRules)
	if cursor.Loaded {
		t.Error(".cursorrules should not be loaded when file doesn't exist")
	}
}

func TestResolveHighestPriority(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("soul"), 0644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents"), 0644)
	os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("cursor"), 0644)

	r.Load(context.Background())

	result := r.Resolve()
	if result != "soul" {
		t.Errorf("expected highest priority (soul) content, got %q", result)
	}
}

func TestResolveFallback(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	// Only AGENTS.md exists, no SOUL.md.
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agents only"), 0644)

	r.Load(context.Background())

	result := r.Resolve()
	if result != "agents only" {
		t.Errorf("expected agents fallback content, got %q", result)
	}
}

func TestResolveNone(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	r.Load(context.Background())

	result := r.Resolve()
	if result != "" {
		t.Errorf("expected empty string when no files exist, got %q", result)
	}
}

func TestResolveAllBudgetAllocation(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte(strings.Repeat("s", 100)), 0644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(strings.Repeat("a", 100)), 0644)
	os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte(strings.Repeat("c", 100)), 0644)

	r.Load(context.Background())

	allocated := r.ResolveAll(200)
	if len(allocated) != 3 {
		t.Fatalf("expected 3 allocated files, got %d", len(allocated))
	}

	// Verify priority ordering.
	if allocated[0].Priority != PrioritySoul {
		t.Errorf("expected first file to be Soul, got %d", allocated[0].Priority)
	}
	if allocated[1].Priority != PriorityAgents {
		t.Errorf("expected second file to be Agents, got %d", allocated[1].Priority)
	}
	if allocated[2].Priority != PriorityCursorRules {
		t.Errorf("expected third file to be CursorRules, got %d", allocated[2].Priority)
	}

	// Verify total fits within budget (approximately).
	total := 0
	for _, a := range allocated {
		total += len(a.Content)
	}
	if total > 250 { // Allow some slack for truncation markers
		t.Errorf("total content %d exceeds budget 200 significantly", total)
	}
}

func TestResolveAllNoFiles(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	r.Load(context.Background())

	allocated := r.ResolveAll(1000)
	if allocated != nil {
		t.Errorf("expected nil when no files loaded, got %v", allocated)
	}
}

func TestResolveAllZeroBudget(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("soul"), 0644)
	r.Load(context.Background())

	allocated := r.ResolveAll(0)
	if allocated != nil {
		t.Errorf("expected nil for zero budget, got %v", allocated)
	}
}

func TestTruncateWithHeadTailFits(t *testing.T) {
	config := DefaultContextFileConfig()
	content := "short"

	result := truncateWithHeadTail(content, 100, config)
	if result != content {
		t.Errorf("expected content as-is when it fits, got %q", result)
	}
}

func TestTruncateWithHeadTailTruncated(t *testing.T) {
	config := DefaultContextFileConfig()
	content := strings.Repeat("x", 200)

	result := truncateWithHeadTail(content, 100, config)
	if len(result) > 150 { // Some slack for marker
		t.Errorf("result too long: %d chars", len(result))
	}
	if !strings.Contains(result, "[...context truncated...]") {
		t.Error("expected truncation marker in result")
	}
	// Verify head portion is from the start.
	if !strings.HasPrefix(result, "xxx") {
		t.Error("expected result to start with head portion")
	}
	// Verify tail portion is from the end.
	if !strings.HasSuffix(result, "xxx") {
		t.Error("expected result to end with tail portion")
	}
}

func TestTruncateWithHeadTailHeadTailOnly(t *testing.T) {
	config := ContextFileConfig{HeadPercent: 50, TailPercent: 50, MiddleSkip: true}
	content := strings.Repeat("a", 200)

	result := truncateWithHeadTail(content, 100, config)
	if !strings.Contains(result, "[...context truncated...]") {
		t.Error("expected truncation marker")
	}
	// 50% head + 50% tail of available (100 - markerLen).
	// Should have roughly equal head and tail.
	parts := strings.SplitN(result, "[...context truncated...]", 2)
	if len(parts) != 2 {
		t.Fatal("expected exactly 2 parts split by marker")
	}
	// They should be roughly the same size.
	headLen := len(parts[0])
	tailLen := len(parts[1])
	diff := headLen - tailLen
	if diff < 0 {
		diff = -diff
	}
	if diff > 10 {
		t.Errorf("head (%d) and tail (%d) sizes too different", headLen, tailLen)
	}
}

func TestTruncateWithHeadTailZeroOrNegative(t *testing.T) {
	config := DefaultContextFileConfig()
	content := "some content"

	if result := truncateWithHeadTail(content, 0, config); result != "" {
		t.Errorf("expected empty for maxChars=0, got %q", result)
	}
	if result := truncateWithHeadTail(content, -5, config); result != "" {
		t.Errorf("expected empty for negative maxChars, got %q", result)
	}
}

func TestTruncateWithHeadTailVerySmallBudget(t *testing.T) {
	config := DefaultContextFileConfig()
	content := strings.Repeat("y", 200)

	// With a very small budget, we might not have room for the marker.
	result := truncateWithHeadTail(content, 10, config)
	if len(result) > 10 {
		t.Errorf("result %d chars exceeds budget of 10", len(result))
	}
}

func TestSetFileCreatesAndWrites(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	if err := r.SetFile(PrioritySoul, "new soul content"); err != nil {
		t.Fatalf("SetFile failed: %v", err)
	}

	soul := r.GetFile(PrioritySoul)
	if !soul.Loaded {
		t.Error("expected SOUL to be loaded after SetFile")
	}
	if soul.Content != "new soul content" {
		t.Errorf("expected 'new soul content', got %q", soul.Content)
	}

	// Verify on disk.
	data, err := os.ReadFile(filepath.Join(dir, "SOUL.md"))
	if err != nil {
		t.Fatalf("failed to read SOUL.md from disk: %v", err)
	}
	if string(data) != "new soul content" {
		t.Errorf("disk content mismatch: %q", string(data))
	}
}

func TestSetFileUnknownPriority(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	err := r.SetFile(ContextFilePriority(99), "content")
	if err == nil {
		t.Error("expected error for unknown priority")
	}
}

func TestReloadAfterExternalChange(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("original"), 0644)
	r.Load(context.Background())

	if soul := r.GetFile(PrioritySoul); soul.Content != "original" {
		t.Fatalf("initial load: expected 'original', got %q", soul.Content)
	}

	// External change.
	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("updated"), 0644)

	if err := r.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if soul := r.GetFile(PrioritySoul); soul.Content != "updated" {
		t.Errorf("after reload: expected 'updated', got %q", soul.Content)
	}
}

func TestMutualExclusion(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	// Each priority can only have one file.
	soul1 := r.GetFile(PrioritySoul)
	soul2 := r.GetFile(PrioritySoul)
	if soul1 != soul2 {
		t.Error("expected same pointer for same priority — mutual exclusion")
	}

	// Setting a file at an existing priority overwrites it.
	r.SetFile(PrioritySoul, "first")
	r.SetFile(PrioritySoul, "second")

	soul := r.GetFile(PrioritySoul)
	if soul.Content != "second" {
		t.Errorf("expected 'second' after overwrite, got %q", soul.Content)
	}
}

func TestSetCursorRulesPath(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	// Create a .cursorrules in a different directory.
	otherDir := t.TempDir()
	os.WriteFile(filepath.Join(otherDir, ".cursorrules"), []byte("project rules"), 0644)

	r.SetCursorRulesPath(filepath.Join(otherDir, ".cursorrules"))
	r.Load(context.Background())

	cursor := r.GetFile(PriorityCursorRules)
	if !cursor.Loaded || cursor.Content != "project rules" {
		t.Errorf("expected project rules content, got loaded=%v content=%q", cursor.Loaded, cursor.Content)
	}
	if cursor.Path != filepath.Join(otherDir, ".cursorrules") {
		t.Errorf("expected path to be updated: %s", cursor.Path)
	}
}

func TestResolveAllWithSingleFile(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte(strings.Repeat("s", 50)), 0644)
	r.Load(context.Background())

	allocated := r.ResolveAll(100)
	if len(allocated) != 1 {
		t.Fatalf("expected 1 allocated file, got %d", len(allocated))
	}
	if allocated[0].Priority != PrioritySoul {
		t.Errorf("expected Soul priority, got %d", allocated[0].Priority)
	}
	if allocated[0].Truncated {
		t.Error("50 chars should not be truncated with budget 100")
	}
}

func TestResolveAllContentFitsExactly(t *testing.T) {
	dir := t.TempDir()
	r := NewContextFileResolver(dir, DefaultContextFileConfig())

	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte("soul"), 0644)
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("agent"), 0644)

	r.Load(context.Background())

	allocated := r.ResolveAll(1000)
	if len(allocated) != 2 {
		t.Fatalf("expected 2 allocated files, got %d", len(allocated))
	}
	for _, a := range allocated {
		if a.Truncated {
			t.Errorf("file %s should not be truncated with large budget", a.Path)
		}
	}
}
