package state

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/types"
)

func TestNewAppState(t *testing.T) {
	s := NewAppState()
	if s.Config == nil {
		t.Error("Config should be initialized")
	}
	if s.CurrentModel != "claude-sonnet-4-5" {
		t.Errorf("CurrentModel = %q, want claude-sonnet-4-5", s.CurrentModel)
	}
	if s.Sessions == nil {
		t.Error("Sessions map should be initialized")
	}
	if s.Cache == nil {
		t.Error("Cache map should be initialized")
	}
	if s.maxSessions != 1000 {
		t.Errorf("maxSessions = %d, want 1000", s.maxSessions)
	}
}

func TestAppState_SetGetConfig(t *testing.T) {
	s := NewAppState()
	cfg := &types.Config{Model: "test-model", MaxTokens: 4096}
	s.SetConfig(cfg)

	got := s.GetConfig()
	if got.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", got.Model)
	}
	if got.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want 4096", got.MaxTokens)
	}
}

func TestAppState_SetGetModel(t *testing.T) {
	s := NewAppState()
	s.SetModel("gpt-4")

	if got := s.GetModel(); got != "gpt-4" {
		t.Errorf("Model = %q, want gpt-4", got)
	}
}

func TestAppState_CreateSession(t *testing.T) {
	s := NewAppState()
	s.SetModel("test-model")

	sess := s.CreateSession("sess-1")
	if sess == nil {
		t.Fatal("session should not be nil")
	}
	if sess.ID != "sess-1" {
		t.Errorf("ID = %q, want sess-1", sess.ID)
	}
	if sess.Model != "test-model" {
		t.Errorf("Model = %q, want test-model", sess.Model)
	}
	if s.ActiveSession != sess {
		t.Error("ActiveSession should be the created session")
	}
}

func TestAppState_GetSession(t *testing.T) {
	s := NewAppState()
	s.CreateSession("sess-1")

	sess := s.GetSession("sess-1")
	if sess == nil {
		t.Fatal("session should exist")
	}
	if sess.ID != "sess-1" {
		t.Errorf("ID = %q, want sess-1", sess.ID)
	}
}

func TestAppState_GetSession_NotFound(t *testing.T) {
	s := NewAppState()
	sess := s.GetSession("nonexistent")
	if sess != nil {
		t.Error("should return nil for nonexistent session")
	}
}

func TestAppState_GetActiveSession(t *testing.T) {
	s := NewAppState()
	sess := s.CreateSession("sess-1")

	active := s.GetActiveSession()
	if active != sess {
		t.Error("active session should match created session")
	}
}

func TestAppState_SetActiveSession(t *testing.T) {
	s := NewAppState()
	s.CreateSession("sess-1")
	s.CreateSession("sess-2")

	err := s.SetActiveSession("sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.GetActiveSession().ID != "sess-1" {
		t.Error("active session should be sess-1")
	}
}

func TestAppState_SetActiveSession_NotFound(t *testing.T) {
	s := NewAppState()
	err := s.SetActiveSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestAppState_AddMessage(t *testing.T) {
	s := NewAppState()
	s.CreateSession("sess-1")

	msg := types.Message{Role: "user", Content: "hello"}
	err := s.AddMessage("sess-1", msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sess := s.GetSession("sess-1")
	if len(sess.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(sess.Messages))
	}
	if sess.Messages[0].Content != "hello" {
		t.Errorf("Content = %v, want hello", sess.Messages[0].Content)
	}
}

func TestAppState_AddMessage_SessionNotFound(t *testing.T) {
	s := NewAppState()
	err := s.AddMessage("nonexistent", types.Message{Role: "user", Content: "test"})
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestAppState_SaveLoadSession(t *testing.T) {
	s := NewAppState()
	s.CreateSession("sess-1")
	s.AddMessage("sess-1", types.Message{Role: "user", Content: "hello"})

	tmpDir := t.TempDir()
	path := tmpDir + "/session.json"

	err := s.SaveSession(path)
	if err != nil {
		t.Fatalf("SaveSession error: %v", err)
	}

	s2 := NewAppState()
	err = s2.LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession error: %v", err)
	}

	loaded := s2.GetSession("sess-1")
	if loaded == nil {
		t.Fatal("loaded session should exist")
	}
	if loaded.ID != "sess-1" {
		t.Errorf("ID = %q, want sess-1", loaded.ID)
	}
}

func TestAppState_LoadSession_FileNotFound(t *testing.T) {
	s := NewAppState()
	err := s.LoadSession("/nonexistent/path/session.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestAppState_SaveSession_NoActiveSession(t *testing.T) {
	s := NewAppState()
	tmpDir := t.TempDir()
	path := tmpDir + "/session.json"

	err := s.SaveSession(path)
	if err != nil {
		t.Fatalf("SaveSession with nil active session should not error: %v", err)
	}

	data, err := json.MarshalIndent(nil, "", "  ")
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	_ = data
}

func TestAppState_ListSessions(t *testing.T) {
	s := NewAppState()
	s.CreateSession("sess-1")
	s.CreateSession("sess-2")

	list := s.ListSessions()
	if len(list) != 2 {
		t.Errorf("ListSessions length = %d, want 2", len(list))
	}
}

func TestAppState_DeleteSession(t *testing.T) {
	s := NewAppState()
	s.CreateSession("sess-1")
	s.CreateSession("sess-2")

	err := s.DeleteSession("sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.GetSession("sess-1") != nil {
		t.Error("session should be deleted")
	}
}

func TestAppState_DeleteSession_ActiveSession(t *testing.T) {
	s := NewAppState()
	s.CreateSession("sess-1")

	err := s.DeleteSession("sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.GetActiveSession() != nil {
		t.Error("active session should be nil after deletion")
	}
}

func TestAppState_DeleteSession_NotFound(t *testing.T) {
	s := NewAppState()
	err := s.DeleteSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestAppState_CacheSetGet(t *testing.T) {
	s := NewAppState()
	s.SetCache("key1", "value1", 0)

	val, ok := s.GetCache("key1")
	if !ok {
		t.Error("cache key should exist")
	}
	if val != "value1" {
		t.Errorf("value = %v, want value1", val)
	}
}

func TestAppState_CacheGet_Miss(t *testing.T) {
	s := NewAppState()
	_, ok := s.GetCache("nonexistent")
	if ok {
		t.Error("should not find nonexistent key")
	}
}

func TestAppState_CacheTTL_Expiry(t *testing.T) {
	s := NewAppState()
	s.SetCache("key1", "value1", 1*time.Nanosecond)

	time.Sleep(10 * time.Millisecond)

	_, ok := s.GetCache("key1")
	if ok {
		t.Error("expired cache entry should not be returned")
	}
}

func TestAppState_CacheTTL_NotExpired(t *testing.T) {
	s := NewAppState()
	s.SetCache("key1", "value1", 1*time.Hour)

	val, ok := s.GetCache("key1")
	if !ok {
		t.Error("non-expired cache entry should be found")
	}
	if val != "value1" {
		t.Errorf("value = %v, want value1", val)
	}
}

func TestAppState_CacheTTL_Zero(t *testing.T) {
	s := NewAppState()
	s.SetCache("key1", "value1", 0)

	val, ok := s.GetCache("key1")
	if !ok {
		t.Error("zero TTL cache entry should not expire")
	}
	if val != "value1" {
		t.Errorf("value = %v, want value1", val)
	}
}

func TestAppState_DeleteCache(t *testing.T) {
	s := NewAppState()
	s.SetCache("key1", "value1", 0)
	s.DeleteCache("key1")

	_, ok := s.GetCache("key1")
	if ok {
		t.Error("deleted cache key should not exist")
	}
}

func TestAppState_ClearCache(t *testing.T) {
	s := NewAppState()
	s.SetCache("key1", "value1", 0)
	s.SetCache("key2", "value2", 0)
	s.ClearCache()

	if len(s.Cache) != 0 {
		t.Errorf("Cache length = %d, want 0", len(s.Cache))
	}
}

func TestAppState_CleanupCache(t *testing.T) {
	s := NewAppState()
	s.SetCache("expired", "val", 1*time.Nanosecond)
	s.SetCache("valid", "val", 1*time.Hour)

	time.Sleep(10 * time.Millisecond)

	removed := s.CleanupCache()
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	_, ok := s.GetCache("valid")
	if !ok {
		t.Error("valid cache entry should still exist")
	}
}

func TestAppState_MaxSessionEviction(t *testing.T) {
	s := NewAppState()
	s.maxSessions = 3

	s.CreateSession("sess-1")
	time.Sleep(1 * time.Millisecond)
	s.CreateSession("sess-2")
	time.Sleep(1 * time.Millisecond)
	s.CreateSession("sess-3")

	s.CreateSession("sess-4")

	if len(s.Sessions) > 3 {
		t.Errorf("Sessions count = %d, should not exceed maxSessions=3", len(s.Sessions))
	}

	if s.GetSession("sess-4") == nil {
		t.Error("newly created session should exist")
	}
}

func TestContext_SetGet(t *testing.T) {
	c := NewContext()
	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok {
		t.Error("key should exist")
	}
	if val != "value1" {
		t.Errorf("value = %v, want value1", val)
	}
}

func TestContext_Get_Missing(t *testing.T) {
	c := NewContext()
	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("missing key should return false")
	}
}

func TestContext_Delete(t *testing.T) {
	c := NewContext()
	c.Set("key1", "value1")
	c.Delete("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("deleted key should not exist")
	}
}

func TestContext_Keys(t *testing.T) {
	c := NewContext()
	c.Set("a", 1)
	c.Set("b", 2)

	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys length = %d, want 2", len(keys))
	}
}

func TestContext_Clear(t *testing.T) {
	c := NewContext()
	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Clear()

	_, ok := c.Get("key1")
	if ok {
		t.Error("cleared context should have no keys")
	}
}

func TestContext_PushPop(t *testing.T) {
	c := NewContext()
	c.Set("key1", "original")

	c.Push()
	c.Set("key1", "modified")
	c.Set("key2", "new")

	if val, _ := c.Get("key1"); val != "modified" {
		t.Errorf("before pop: key1 = %v, want modified", val)
	}

	c.Pop()

	if val, _ := c.Get("key1"); val != "original" {
		t.Errorf("after pop: key1 = %v, want original", val)
	}

	if _, ok := c.Get("key2"); ok {
		t.Error("after pop: key2 should not exist")
	}
}

func TestContext_Pop_EmptyStack(t *testing.T) {
	c := NewContext()
	c.Set("key1", "value1")

	c.Pop()

	if val, ok := c.Get("key1"); !ok || val != "value1" {
		t.Error("pop on empty stack should be a no-op")
	}
}

func TestContext_Snapshot(t *testing.T) {
	c := NewContext()
	c.Set("key1", "value1")
	c.Set("key2", "value2")

	snap := c.Snapshot()
	if len(snap) != 2 {
		t.Errorf("snapshot length = %d, want 2", len(snap))
	}

	snap["key1"] = "modified"
	if val, _ := c.Get("key1"); val != "value1" {
		t.Error("snapshot should be a copy, not a reference")
	}
}

func TestNewStateManager(t *testing.T) {
	sm := NewStateManager()
	if sm.GetState() == nil {
		t.Error("state should not be nil")
	}
	if sm.GetContext() == nil {
		t.Error("context should not be nil")
	}
}

func TestAppState_ConcurrentAccess(t *testing.T) {
	s := NewAppState()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sid := "sess-" + string(rune('0'+id%10))
			s.CreateSession(sid)
			_ = s.GetSession(sid)
			s.SetCache("key-"+string(rune('0'+id%10)), id, 0)
			_, _ = s.GetCache("key-" + string(rune('0'+id%10)))
		}(i)
	}

	wg.Wait()
}

func TestAppState_CacheWithDifferentValueTypes(t *testing.T) {
	s := NewAppState()

	s.SetCache("int_key", 42, 0)
	s.SetCache("map_key", map[string]string{"a": "b"}, 0)
	s.SetCache("slice_key", []int{1, 2, 3}, 0)
	s.SetCache("bool_key", true, 0)

	if val, ok := s.GetCache("int_key"); !ok || val.(int) != 42 {
		t.Errorf("int_key: got %v, %v", val, ok)
	}
	if val, ok := s.GetCache("map_key"); !ok {
		t.Error("map_key should exist")
	} else if m, ok2 := val.(map[string]string); !ok2 || m["a"] != "b" {
		t.Errorf("map_key: got %v", val)
	}
	if val, ok := s.GetCache("slice_key"); !ok {
		t.Error("slice_key should exist")
	} else if sl, ok2 := val.([]int); !ok2 || len(sl) != 3 {
		t.Errorf("slice_key: got %v", val)
	}
	if val, ok := s.GetCache("bool_key"); !ok || val.(bool) != true {
		t.Errorf("bool_key: got %v, %v", val, ok)
	}
}

func TestAppState_CacheOverwrite(t *testing.T) {
	s := NewAppState()
	s.SetCache("key", "original", 0)
	s.SetCache("key", "overwritten", 0)

	val, ok := s.GetCache("key")
	if !ok || val != "overwritten" {
		t.Errorf("got %v, want overwritten", val)
	}
}

func TestAppState_CacheExpiredEntryNotCleanedUntilGet(t *testing.T) {
	s := NewAppState()
	s.SetCache("key", "value", 1*time.Nanosecond)
	time.Sleep(10 * time.Millisecond)

	_, ok := s.GetCache("key")
	if ok {
		t.Error("expired entry should return false from GetCache")
	}
	if len(s.Cache) != 1 {
		t.Errorf("Cache length = %d, want 1 (expired but not cleaned)", len(s.Cache))
	}
}

func TestContext_MultiplePushPop(t *testing.T) {
	c := NewContext()
	c.Set("level", 0)

	c.Push()
	c.Set("level", 1)

	c.Push()
	c.Set("level", 2)

	if val, _ := c.Get("level"); val != 2 {
		t.Errorf("inner: level = %v, want 2", val)
	}

	c.Pop()
	if val, _ := c.Get("level"); val != 1 {
		t.Errorf("middle: level = %v, want 1", val)
	}

	c.Pop()
	if val, _ := c.Get("level"); val != 0 {
		t.Errorf("outer: level = %v, want 0", val)
	}
}

func TestContext_SetOverwrites(t *testing.T) {
	c := NewContext()
	c.Set("key", "original")
	c.Set("key", "updated")

	val, ok := c.Get("key")
	if !ok || val != "updated" {
		t.Errorf("got %v, want updated", val)
	}
}

func TestContext_DeleteNonExistent(t *testing.T) {
	c := NewContext()
	c.Delete("nonexistent")
	if len(c.Keys()) != 0 {
		t.Error("deleting nonexistent key should not add it")
	}
}

func TestContext_PushPopIsolation(t *testing.T) {
	c := NewContext()
	c.Set("a", 1)

	c.Push()
	c.Set("b", 2)

	c.Pop()

	if _, ok := c.Get("b"); ok {
		t.Error("key added after push should be gone after pop")
	}
	if val, ok := c.Get("a"); !ok || val != 1 {
		t.Errorf("original key should be preserved, got %v", val)
	}
}

func TestStateManager_AccessState(t *testing.T) {
	sm := NewStateManager()
	state := sm.GetState()
	if state == nil {
		t.Fatal("state should not be nil")
	}
	if state.CurrentModel != "claude-sonnet-4-5" {
		t.Errorf("default model = %q, want claude-sonnet-4-5", state.CurrentModel)
	}
}

func TestStateManager_AccessContext(t *testing.T) {
	sm := NewStateManager()
	ctx := sm.GetContext()
	if ctx == nil {
		t.Fatal("context should not be nil")
	}
	ctx.Set("test", "value")
	if val, ok := ctx.Get("test"); !ok || val != "value" {
		t.Errorf("got %v, want value", val)
	}
}

func TestAppState_CreateSessionSetsModel(t *testing.T) {
	s := NewAppState()
	s.SetModel("custom-model")
	sess := s.CreateSession("s1")
	if sess.Model != "custom-model" {
		t.Errorf("session model = %q, want custom-model", sess.Model)
	}
}

func TestAppState_SaveLoadSessionWithMessages(t *testing.T) {
	s := NewAppState()
	s.CreateSession("sess-msg")
	s.AddMessage("sess-msg", types.Message{Role: "user", Content: "hello"})
	s.AddMessage("sess-msg", types.Message{Role: "assistant", Content: "world"})

	tmpDir := t.TempDir()
	path := tmpDir + "/session.json"

	if err := s.SaveSession(path); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	s2 := NewAppState()
	if err := s2.LoadSession(path); err != nil {
		t.Fatalf("LoadSession: %v", err)
	}

	loaded := s2.GetSession("sess-msg")
	if loaded == nil {
		t.Fatal("session should exist after load")
	}
	if len(loaded.Messages) != 2 {
		t.Errorf("messages count = %d, want 2", len(loaded.Messages))
	}
}

func TestGlobalStateManager_Init(t *testing.T) {
	Init()
	sm := Get()
	if sm == nil {
		t.Fatal("Get() should not return nil after Init()")
	}
	if sm.GetState() == nil {
		t.Error("state should be initialized")
	}
}
