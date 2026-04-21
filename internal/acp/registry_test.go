package acp

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestServiceRegistry_RegisterAndGet(t *testing.T) {
	sr := NewServiceRegistry()

	entry := ServiceEntry{
		Name:         "tool-runner",
		Version:      "1.0.0",
		Endpoint:     "localhost:8080",
		Capabilities: []string{"bash", "python"},
		Metadata:     map[string]any{"region": "us-east"},
	}

	if err := sr.Register(entry); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got := sr.Get("tool-runner")
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", got.Version, "1.0.0")
	}
	if got.RegisteredAt.IsZero() {
		t.Error("RegisteredAt should be set")
	}
}

func TestServiceRegistry_RegisterEmptyName(t *testing.T) {
	sr := NewServiceRegistry()
	err := sr.Register(ServiceEntry{Name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestServiceRegistry_Deregister(t *testing.T) {
	sr := NewServiceRegistry()
	sr.Register(ServiceEntry{Name: "svc1", Version: "1.0"})

	if sr.Get("svc1") == nil {
		t.Fatal("svc1 should exist")
	}

	sr.Deregister("svc1")
	if sr.Get("svc1") != nil {
		t.Error("svc1 should be gone after Deregister")
	}
}

func TestServiceRegistry_DeregisterNonExistent(t *testing.T) {
	sr := NewServiceRegistry()
	sr.Deregister("nope")
}

func TestServiceRegistry_List(t *testing.T) {
	sr := NewServiceRegistry()
	sr.Register(ServiceEntry{Name: "a", Version: "1"})
	sr.Register(ServiceEntry{Name: "b", Version: "2"})

	list := sr.List()
	if len(list) != 2 {
		t.Errorf("List returned %d items, want 2", len(list))
	}
}

func TestServiceRegistry_FindByCapability(t *testing.T) {
	sr := NewServiceRegistry()
	sr.Register(ServiceEntry{
		Name:         "runner",
		Capabilities: []string{"bash", "python"},
	})
	sr.Register(ServiceEntry{
		Name:         "browser",
		Capabilities: []string{"chromium"},
	})

	found := sr.FindByCapability("bash")
	if len(found) != 1 {
		t.Fatalf("FindByCapability(bash) = %d, want 1", len(found))
	}
	if found[0].Name != "runner" {
		t.Errorf("found %q, want runner", found[0].Name)
	}

	none := sr.FindByCapability("nonexistent")
	if len(none) != 0 {
		t.Errorf("FindByCapability(nonexistent) = %d, want 0", len(none))
	}
}

func TestServiceRegistry_Heartbeat(t *testing.T) {
	sr := NewServiceRegistry()
	sr.Register(ServiceEntry{Name: "svc", Version: "1"})

	original := sr.Get("svc").LastHeartbeat
	time.Sleep(10 * time.Millisecond)

	sr.Heartbeat("svc")
	updated := sr.Get("svc").LastHeartbeat

	if !updated.After(original) {
		t.Error("Heartbeat should update LastHeartbeat")
	}
}

func TestServiceRegistry_HeartbeatNonExistent(t *testing.T) {
	sr := NewServiceRegistry()
	sr.Heartbeat("nope")
}

func TestServiceRegistry_PruneStale(t *testing.T) {
	sr := NewServiceRegistry()

	sr.Register(ServiceEntry{Name: "fresh", Version: "1"})
	sr.Register(ServiceEntry{Name: "stale", Version: "1"})

	sr.mu.Lock()
	sr.services["stale"].LastHeartbeat = time.Now().Add(-2 * time.Hour)
	sr.mu.Unlock()

	sr.PruneStale(time.Hour)

	if sr.Get("fresh") == nil {
		t.Error("fresh service should remain")
	}
	if sr.Get("stale") != nil {
		t.Error("stale service should be pruned")
	}
}

func TestServiceRegistry_ConcurrentAccess(t *testing.T) {
	sr := NewServiceRegistry()
	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("svc-%d", i)
			sr.Register(ServiceEntry{Name: name, Version: "1"})
		}()
	}

	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("svc-%d", i)
			sr.Heartbeat(name)
		}()
	}

	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sr.List()
			sr.FindByCapability(fmt.Sprintf("cap-%d", i))
		}()
	}

	wg.Wait()

	list := sr.List()
	if len(list) != 50 {
		t.Errorf("expected 50 services, got %d", len(list))
	}
}
