package memory

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/store"
)

// --- ProviderConfig tests ---

func TestProviderConfigGetString(t *testing.T) {
	cfg := ProviderConfig{"host": "localhost", "port": 8080}

	if v := cfg.GetString("host", "default"); v != "localhost" {
		t.Fatalf("expected %q, got %q", "localhost", v)
	}
	if v := cfg.GetString("missing", "fallback"); v != "fallback" {
		t.Fatalf("expected %q, got %q", "fallback", v)
	}
	if v := cfg.GetString("port", "default"); v != "default" {
		t.Fatalf("expected %q for wrong-type key, got %q", "default", v)
	}
}

func TestProviderConfigGetInt(t *testing.T) {
	cfg := ProviderConfig{"port": 8080, "ratio": 0.5, "big": int64(999)}

	if v := cfg.GetInt("port", 0); v != 8080 {
		t.Fatalf("expected 8080, got %d", v)
	}
	if v := cfg.GetInt("missing", 42); v != 42 {
		t.Fatalf("expected 42, got %d", v)
	}
	if v := cfg.GetInt("ratio", 0); v != 0 {
		t.Fatalf("expected 0 for float64 key, got %d", v)
	}
	if v := cfg.GetInt("big", 0); v != 999 {
		t.Fatalf("expected 999 for int64 key, got %d", v)
	}
}

func TestProviderConfigGetDuration(t *testing.T) {
	d := 5 * time.Second
	cfg := ProviderConfig{"timeout": d}

	if v := cfg.GetDuration("timeout", 0); v != d {
		t.Fatalf("expected %v, got %v", d, v)
	}
	if v := cfg.GetDuration("missing", time.Minute); v != time.Minute {
		t.Fatalf("expected %v, got %v", time.Minute, v)
	}
}

// --- ProviderRegistry tests ---

func TestProviderRegistryRegisterAndGet(t *testing.T) {
	reg := NewProviderRegistry()
	p := &mockProvider{name: "test"}

	if err := reg.Register("test", p); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	got, err := reg.Get("test")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if got != p {
		t.Fatal("expected same provider instance")
	}
}

func TestProviderRegistryRegisterDuplicate(t *testing.T) {
	reg := NewProviderRegistry()
	p := &mockProvider{name: "test"}

	if err := reg.Register("test", p); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	if err := reg.Register("test", p); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestProviderRegistryRegisterEmptyName(t *testing.T) {
	reg := NewProviderRegistry()
	if err := reg.Register("", &mockProvider{name: "x"}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestProviderRegistryRegisterNilProvider(t *testing.T) {
	reg := NewProviderRegistry()
	if err := reg.Register("nil", nil); err == nil {
		t.Fatal("expected error for nil provider")
	}
}

func TestProviderRegistryGetMissing(t *testing.T) {
	reg := NewProviderRegistry()
	if _, err := reg.Get("missing"); err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestProviderRegistryList(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("charlie", &mockProvider{name: "charlie"})
	reg.Register("alpha", &mockProvider{name: "alpha"})
	reg.Register("bravo", &mockProvider{name: "bravo"})

	names := reg.List()
	expected := []string{"alpha", "bravo", "charlie"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d", len(expected), len(names))
	}
	for i, n := range expected {
		if names[i] != n {
			t.Fatalf("expected names[%d] = %q, got %q", i, n, names[i])
		}
	}
}

func TestProviderRegistryDefault(t *testing.T) {
	reg := NewProviderRegistry()
	p1 := &mockProvider{name: "first"}
	p2 := &mockProvider{name: "second"}

	if err := reg.Register("first", p1); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	def, err := reg.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault error: %v", err)
	}
	if def != p1 {
		t.Fatal("expected first provider as default")
	}

	if err := reg.Register("second", p2); err != nil {
		t.Fatalf("Register error: %v", err)
	}

	if err := reg.SetDefault("second"); err != nil {
		t.Fatalf("SetDefault error: %v", err)
	}

	def, err = reg.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault error: %v", err)
	}
	if def != p2 {
		t.Fatal("expected second provider as default after SetDefault")
	}
}

func TestProviderRegistrySetDefaultMissing(t *testing.T) {
	reg := NewProviderRegistry()
	if err := reg.SetDefault("nonexistent"); err == nil {
		t.Fatal("expected error for setting nonexistent default")
	}
}

func TestProviderRegistryNoDefault(t *testing.T) {
	reg := NewProviderRegistry()
	if _, err := reg.GetDefault(); err == nil {
		t.Fatal("expected error when no default set")
	}
}

func TestProviderRegistryUnregister(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("test", &mockProvider{name: "test"})

	if err := reg.Unregister("test"); err != nil {
		t.Fatalf("Unregister error: %v", err)
	}

	if _, err := reg.Get("test"); err == nil {
		t.Fatal("expected error after unregister")
	}
}

func TestProviderRegistryUnregisterMissing(t *testing.T) {
	reg := NewProviderRegistry()
	if err := reg.Unregister("missing"); err == nil {
		t.Fatal("expected error for unregistering missing provider")
	}
}

func TestProviderRegistryUnregisterClearsDefault(t *testing.T) {
	reg := NewProviderRegistry()
	reg.Register("only", &mockProvider{name: "only"})

	reg.Unregister("only")

	if _, err := reg.GetDefault(); err == nil {
		t.Fatal("expected error after unregistering default provider")
	}
}

func TestProviderRegistryConcurrentAccess(t *testing.T) {
	reg := NewProviderRegistry()
	var wg sync.WaitGroup
	const count = 50

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('a' + i%26))
			if i%3 == 0 {
				reg.Register(name, &mockProvider{name: name})
			} else if i%3 == 1 {
				reg.Get(name)
			} else {
				reg.List()
			}
		}(i)
	}
	wg.Wait()
}

// --- Global registry tests ---

func TestGlobalRegistryRegisterAndGet(t *testing.T) {
	p := &mockProvider{name: "global-test"}
	if err := RegisterProvider("global-test", p); err != nil {
		t.Fatalf("RegisterProvider error: %v", err)
	}
	defer globalRegistry.Unregister("global-test")

	got, err := GetProvider("global-test")
	if err != nil {
		t.Fatalf("GetProvider error: %v", err)
	}
	if got != p {
		t.Fatal("expected same provider instance")
	}
}

func TestGlobalRegistryList(t *testing.T) {
	names := ListProviders()
	if names == nil {
		t.Fatal("expected non-nil list")
	}
}

// --- BuiltinProvider tests ---

func newTestBuiltinProvider(t *testing.T) *BuiltinProvider {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "builtin-provider-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	store, err := NewMemoryStore()
	if err != nil {
		t.Fatalf("NewMemoryStore error: %v", err)
	}

	bp := NewBuiltinProvider(store, nil)
	if err := bp.Initialize(context.Background(), nil); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	t.Cleanup(func() { bp.Close() })

	return bp
}

func TestBuiltinProviderName(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	if bp.Name() != "builtin" {
		t.Fatalf("expected name %q, got %q", "builtin", bp.Name())
	}
}

func TestBuiltinProviderStoreAndRetrieve(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	if err := bp.Store(ctx, "key1", "value1", 0, nil); err != nil {
		t.Fatalf("Store error: %v", err)
	}

	val, err := bp.Retrieve(ctx, "key1")
	if err != nil {
		t.Fatalf("Retrieve error: %v", err)
	}
	if val != "value1" {
		t.Fatalf("expected %q, got %q", "value1", val)
	}
}

func TestBuiltinProviderRetrieveMissing(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	_, err := bp.Retrieve(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error retrieving missing key")
	}
}

func TestBuiltinProviderDelete(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	bp.Store(ctx, "key1", "value1", 0, nil)

	if err := bp.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	_, err := bp.Retrieve(ctx, "key1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestBuiltinProviderDeleteMissing(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	if err := bp.Delete(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error deleting missing key")
	}
}

func TestBuiltinProviderList(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	bp.Store(ctx, "prefix_key1", "v1", 0, nil)
	bp.Store(ctx, "prefix_key2", "v2", 0, nil)
	bp.Store(ctx, "other_key", "v3", 0, nil)

	all, err := bp.List(ctx, "")
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(all))
	}

	prefixed, err := bp.List(ctx, "prefix_")
	if err != nil {
		t.Fatalf("List with prefix error: %v", err)
	}
	if len(prefixed) != 2 {
		t.Fatalf("expected 2 prefixed keys, got %d", len(prefixed))
	}
}

func TestBuiltinProviderSearchWithStore(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	bp.Store(ctx, "search-key", "searchable value", 0, []string{"tag1"})

	results, err := bp.Search(ctx, "search-key", 10)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) < 1 {
		t.Fatalf("expected at least 1 result, got %d", len(results))
	}

	found := false
	for _, r := range results {
		if r.Key == "search-key" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected to find 'search-key' in results")
	}
}

func TestBuiltinProviderStoreWithTags(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	tags := []string{"important", "project-x"}
	if err := bp.Store(ctx, "tagged-key", "value", 0, tags); err != nil {
		t.Fatalf("Store with tags error: %v", err)
	}

	val, err := bp.Retrieve(ctx, "tagged-key")
	if err != nil {
		t.Fatalf("Retrieve error: %v", err)
	}
	if val != "value" {
		t.Fatalf("expected %q, got %q", "value", val)
	}
}

func TestBuiltinProviderStoreWithTTL(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	if err := bp.Store(ctx, "ttl-key", "expires", 50*time.Millisecond, nil); err != nil {
		t.Fatalf("Store with TTL error: %v", err)
	}

	val, err := bp.Retrieve(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("Retrieve error before expiry: %v", err)
	}
	if val != "expires" {
		t.Fatalf("expected %q, got %q", "expires", val)
	}

	time.Sleep(100 * time.Millisecond)

	_, err = bp.Retrieve(ctx, "ttl-key")
	if err == nil {
		t.Fatal("expected error after TTL expiry")
	}
}

func TestBuiltinProviderNilStore(t *testing.T) {
	bp := NewBuiltinProvider(nil, nil)
	ctx := context.Background()

	if err := bp.Initialize(ctx, nil); err == nil {
		t.Fatal("expected error initializing with nil store")
	}

	if err := bp.Store(ctx, "key", "val", 0, nil); err == nil {
		t.Fatal("expected error storing with nil store")
	}

	if _, err := bp.Retrieve(ctx, "key"); err == nil {
		t.Fatal("expected error retrieving with nil store")
	}

	if err := bp.Delete(ctx, "key"); err == nil {
		t.Fatal("expected error deleting with nil store")
	}

	if _, err := bp.List(ctx, ""); err == nil {
		t.Fatal("expected error listing with nil store")
	}
}

func TestBuiltinProviderSearchLimit(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		bp.Store(ctx, "key"+string(rune('0'+i)), "value", 0, nil)
	}

	results, err := bp.Search(ctx, "key", 2)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(results) > 2 {
		t.Fatalf("expected at most 2 results, got %d", len(results))
	}
}

// --- Lifecycle hook tests ---

func TestBuiltinProviderOnStoreHook(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	var called bool
	var capturedKey string
	var capturedValue any
	var capturedTags []string

	bp.SetOnStoreHook(func(_ context.Context, key string, value any, _ time.Duration, tags []string) {
		called = true
		capturedKey = key
		capturedValue = value
		capturedTags = tags
	})

	tags := []string{"hook-test"}
	bp.Store(ctx, "hook-key", "hook-value", 0, tags)

	if !called {
		t.Fatal("expected OnStoreHook to be called")
	}
	if capturedKey != "hook-key" {
		t.Fatalf("expected key %q, got %q", "hook-key", capturedKey)
	}
	if capturedValue != "hook-value" {
		t.Fatalf("expected value %q, got %q", "hook-value", capturedValue)
	}
	if len(capturedTags) != 1 || capturedTags[0] != "hook-test" {
		t.Fatalf("expected tags %v, got %v", tags, capturedTags)
	}
}

func TestBuiltinProviderOnRetrieveHook(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	bp.Store(ctx, "rkey", "rvalue", 0, nil)

	var called bool
	var capturedKey string
	var capturedValue any

	bp.SetOnRetrieveHook(func(_ context.Context, key string, value any) {
		called = true
		capturedKey = key
		capturedValue = value
	})

	bp.Retrieve(ctx, "rkey")

	if !called {
		t.Fatal("expected OnRetrieveHook to be called")
	}
	if capturedKey != "rkey" {
		t.Fatalf("expected key %q, got %q", "rkey", capturedKey)
	}
	if capturedValue != "rvalue" {
		t.Fatalf("expected value %q, got %q", "rvalue", capturedValue)
	}
}

func TestBuiltinProviderOnDeleteHook(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	bp.Store(ctx, "dkey", "dvalue", 0, nil)

	var called bool
	var capturedKey string

	bp.SetOnDeleteHook(func(_ context.Context, key string) {
		called = true
		capturedKey = key
	})

	bp.Delete(ctx, "dkey")

	if !called {
		t.Fatal("expected OnDeleteHook to be called")
	}
	if capturedKey != "dkey" {
		t.Fatalf("expected key %q, got %q", "dkey", capturedKey)
	}
}

func TestBuiltinProviderOnSearchHook(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	bp.Store(ctx, "skey", "svalue", 0, nil)

	var called bool
	var capturedQuery string

	bp.SetOnSearchHook(func(_ context.Context, query string, _ []SearchResult) {
		called = true
		capturedQuery = query
	})

	bp.Search(ctx, "skey", 10)

	if !called {
		t.Fatal("expected OnSearchHook to be called")
	}
	if capturedQuery != "skey" {
		t.Fatalf("expected query %q, got %q", "skey", capturedQuery)
	}
}

func TestBuiltinProviderHooksNotCalledOnFailure(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	var retrieveCalled, deleteCalled bool

	bp.SetOnRetrieveHook(func(context.Context, string, any) { retrieveCalled = true })
	bp.SetOnDeleteHook(func(context.Context, string) { deleteCalled = true })

	bp.Delete(ctx, "nonexistent-key")
	if deleteCalled {
		t.Fatal("expected OnDeleteHook NOT to be called on failed delete")
	}

	bp.Retrieve(ctx, "nonexistent-key")
	if retrieveCalled {
		t.Fatal("expected OnRetrieveHook NOT to be called on failed retrieve")
	}
}

// --- MemoryManager provider integration tests ---

func TestMemoryManagerHasBuiltinProvider(t *testing.T) {
	mm := newTestMemoryManager(t)

	p := mm.GetProvider("builtin")
	if p == nil {
		t.Fatal("expected builtin provider to be registered")
	}
	if p.Name() != "builtin" {
		t.Fatalf("expected provider name %q, got %q", "builtin", p.Name())
	}
}

func TestMemoryManagerRegisterProvider(t *testing.T) {
	mm := newTestMemoryManager(t)

	custom := &mockProvider{name: "custom"}
	mm.RegisterProvider("custom", custom)

	got := mm.GetProvider("custom")
	if got != custom {
		t.Fatal("expected same custom provider instance")
	}
}

func TestMemoryManagerGetProviderMissing(t *testing.T) {
	mm := newTestMemoryManager(t)

	got := mm.GetProvider("nonexistent")
	if got != nil {
		t.Fatal("expected nil for missing provider")
	}
}

func TestMemoryManagerListProviderNames(t *testing.T) {
	mm := newTestMemoryManager(t)

	names := mm.ListProviderNames()
	if len(names) < 1 {
		t.Fatal("expected at least one provider (builtin)")
	}

	found := false
	for _, n := range names {
		if n == "builtin" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'builtin' in provider names")
	}
}

func TestMemoryManagerWithComponentsHasBuiltin(t *testing.T) {
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

	p := mm.GetProvider("builtin")
	if p == nil {
		t.Fatal("expected builtin provider in MemoryManagerWithComponents")
	}
}

// --- SearchResult tests ---

func TestSearchResultFields(t *testing.T) {
	now := time.Now()
	sr := SearchResult{
		Key:       "test-key",
		Value:     "test-value",
		Relevance: 0.95,
		Tags:      []string{"tag1", "tag2"},
		CreatedAt: now,
	}

	if sr.Key != "test-key" {
		t.Fatalf("expected Key %q, got %q", "test-key", sr.Key)
	}
	if sr.Value != "test-value" {
		t.Fatalf("expected Value %q, got %q", "test-value", sr.Value)
	}
	if sr.Relevance != 0.95 {
		t.Fatalf("expected Relevance 0.95, got %f", sr.Relevance)
	}
	if len(sr.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(sr.Tags))
	}
	if !sr.CreatedAt.Equal(now) {
		t.Fatalf("expected CreatedAt %v, got %v", now, sr.CreatedAt)
	}
}

// --- Concurrent access safety tests ---

func TestBuiltinProviderConcurrentStoreRetrieve(t *testing.T) {
	bp := newTestBuiltinProvider(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	const count = 20

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := "concurrent-key"
			bp.Store(ctx, key, i, 0, nil)
			bp.Retrieve(ctx, key)
		}(i)
	}
	wg.Wait()
}

func TestProviderRegistryConcurrentRegisterGet(t *testing.T) {
	reg := NewProviderRegistry()
	var wg sync.WaitGroup
	const count = 100

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := string(rune('a' + i%26))
			reg.Register(name, &mockProvider{name: name})
			reg.Get(name)
			reg.List()
		}(i)
	}
	wg.Wait()
}

// --- mock provider for testing ---

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string                                                  { return m.name }
func (m *mockProvider) Initialize(_ context.Context, _ ProviderConfig) error          { return nil }
func (m *mockProvider) Store(_ context.Context, _ string, _ any, _ time.Duration, _ []string) error {
	return nil
}
func (m *mockProvider) Retrieve(_ context.Context, _ string) (any, error)             { return nil, nil }
func (m *mockProvider) Search(_ context.Context, _ string, _ int) ([]SearchResult, error) {
	return nil, nil
}
func (m *mockProvider) Delete(_ context.Context, _ string) error                      { return nil }
func (m *mockProvider) List(_ context.Context, _ string) ([]string, error)            { return nil, nil }
func (m *mockProvider) Close() error                                                  { return nil }
