package gateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/learning"
	"github.com/instructkr/smartclaw/internal/memory"
	"github.com/instructkr/smartclaw/internal/memory/layers"
	"github.com/instructkr/smartclaw/internal/runtime"
	"github.com/instructkr/smartclaw/internal/store"
)

func newTestGateway(t *testing.T) (*Gateway, string) {
	t.Helper()
	dir := t.TempDir()

	s, err := store.NewStoreWithDir(dir)
	if err != nil {
		t.Fatalf("NewStoreWithDir: %v", err)
	}

	pm, err := layers.NewPromptMemoryWithDir(dir)
	if err != nil {
		t.Fatalf("NewPromptMemoryWithDir: %v", err)
	}

	mm := memory.NewMemoryManagerWithComponents(pm, s, nil)

	factory := func() *runtime.QueryEngine {
		return runtime.NewQueryEngine(nil, runtime.QueryConfig{})
	}

	gw := NewGateway(factory, mm, nil)
	t.Cleanup(func() { gw.Close() })

	return gw, dir
}

func TestNewGateway(t *testing.T) {
	gw, _ := newTestGateway(t)
	if gw == nil {
		t.Fatal("gateway should not be nil")
	}
	if gw.GetRouter() == nil {
		t.Error("router should not be nil")
	}
	if gw.GetDelivery() == nil {
		t.Error("delivery should not be nil")
	}
	if gw.GetCronTrigger() == nil {
		t.Error("cron trigger should not be nil")
	}
}

func TestSessionRouter_Route(t *testing.T) {
	gw, _ := newTestGateway(t)
	router := gw.GetRouter()

	session := router.Route("user-1")
	if session == nil {
		t.Fatal("session should not be nil")
	}
	if session.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", session.UserID, "user-1")
	}
	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
}

func TestSessionRouter_SameUserReturnsSameSession(t *testing.T) {
	gw, _ := newTestGateway(t)
	router := gw.GetRouter()

	s1 := router.Route("user-1")
	s2 := router.Route("user-1")

	if s1.ID != s2.ID {
		t.Errorf("same user should get same session within 30min, got %q vs %q", s1.ID, s2.ID)
	}
}

func TestDeliveryManager_RegisterAndDeliver(t *testing.T) {
	gw, _ := newTestGateway(t)
	dm := gw.GetDelivery()

	delivered := false
	dm.RegisterAdapter(&mockAdapter{
		name: "test",
		sendFunc: func(userID string, response *GatewayResponse) error {
			delivered = true
			return nil
		},
	})

	platforms := dm.ListPlatforms()
	if len(platforms) != 1 {
		t.Errorf("platforms count = %d, want 1", len(platforms))
	}

	dm.Deliver("user-1", "test", &GatewayResponse{Content: "hello"})
	if !delivered {
		t.Error("should have delivered via adapter")
	}
}

func TestCronTrigger_ScheduleAndList(t *testing.T) {
	gw, dir := newTestGateway(t)
	ct := gw.GetCronTrigger()

	cronDir := filepath.Join(dir, "cron")
	ct.cronDir = cronDir

	if err := ct.ScheduleCron("test-cron-1", "user-1", "check CI status", "*/5 * * * *", "terminal"); err != nil {
		t.Fatalf("ScheduleCron: %v", err)
	}

	tasks, err := ct.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks count = %d, want 1", len(tasks))
	}
	if tasks[0].ID != "test-cron-1" {
		t.Errorf("task ID = %q, want %q", tasks[0].ID, "test-cron-1")
	}
}

func TestCronTrigger_DeleteTask(t *testing.T) {
	gw, dir := newTestGateway(t)
	ct := gw.GetCronTrigger()

	cronDir := filepath.Join(dir, "cron")
	ct.cronDir = cronDir

	ct.ScheduleCron("del-test", "user-1", "test", "*/5 * * * *", "terminal")

	if err := ct.DeleteTask("del-test"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	tasks, _ := ct.ListTasks()
	if len(tasks) != 0 {
		t.Errorf("tasks count after delete = %d, want 0", len(tasks))
	}
}

func TestCronTrigger_DisableTask(t *testing.T) {
	gw, dir := newTestGateway(t)
	ct := gw.GetCronTrigger()

	cronDir := filepath.Join(dir, "cron")
	ct.cronDir = cronDir

	ct.ScheduleCron("dis-test", "user-1", "test", "*/5 * * * *", "terminal")

	if err := ct.DisableTask("dis-test"); err != nil {
		t.Fatalf("DisableTask: %v", err)
	}

	tasks, _ := ct.ListTasks()
	if len(tasks) != 1 && tasks[0].Enabled {
		t.Error("task should be disabled")
	}
}

func TestGateway_Close(t *testing.T) {
	gw, _ := newTestGateway(t)
	if err := gw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestLearningLoopDisabled(t *testing.T) {
	dir := t.TempDir()
	pm, _ := layers.NewPromptMemoryWithDir(dir)
	s, _ := store.NewStoreWithDir(dir)
	mm := memory.NewMemoryManagerWithComponents(pm, s, nil)

	factory := func() *runtime.QueryEngine {
		return runtime.NewQueryEngine(nil, runtime.QueryConfig{})
	}

	gw := NewGateway(factory, mm, (*learning.LearningLoop)(nil))
	if gw.learning != nil {
		t.Error("learning loop should be nil when passed nil")
	}
	gw.Close()
}

type mockAdapter struct {
	name     string
	sendFunc func(userID string, response *GatewayResponse) error
}

func (m *mockAdapter) Send(userID string, response *GatewayResponse) error {
	if m.sendFunc != nil {
		return m.sendFunc(userID, response)
	}
	return nil
}

func (m *mockAdapter) Name() string {
	return m.name
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func init() {
	_ = time.Now
}
