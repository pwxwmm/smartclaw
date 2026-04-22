package adapters

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/instructkr/smartclaw/internal/tools"
)

func TestTopologyAdapterNilGraph(t *testing.T) {
	t.Parallel()

	a := TopologyAdapter{Graph: nil}
	_, err := a.GetNeighbors("svc1", 1)
	if err == nil {
		t.Error("expected error with nil graph")
	}
}

func TestTopologyAdapterGetNodeHealthNilGraph(t *testing.T) {
	t.Parallel()

	a := TopologyAdapter{Graph: nil}
	health, err := a.GetNodeHealth("svc1")
	if err == nil {
		t.Error("expected error with nil graph")
	}
	if health != "unknown" {
		t.Errorf("health = %q, want %q", health, "unknown")
	}
}

func TestIncidentAdapterNil(t *testing.T) {
	t.Parallel()

	a := IncidentAdapter{IM: nil}
	incidents, err := a.GetRecentIncidents("svc1", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if incidents != nil {
		t.Errorf("incidents = %v, want nil", incidents)
	}
}

func TestIncidentAdapterGetSLOStatusNil(t *testing.T) {
	t.Parallel()

	a := IncidentAdapter{IM: nil}
	slo, err := a.GetSLOStatus("svc1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if slo != nil {
		t.Errorf("slo = %v, want nil", slo)
	}
}

func TestSLOProviderAdapterNil(t *testing.T) {
	t.Parallel()

	a := SLOProviderAdapter{IM: nil}
	slo, err := a.GetSLOStatus("svc1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if slo != nil {
		t.Errorf("slo = %v, want nil", slo)
	}
}

func TestCommanderAdapterExecuteCommand(t *testing.T) {
	t.Parallel()

	a := NewCommanderAdapter(nil)
	result, err := a.ExecuteCommand(context.Background(), "echo hello", 5*time.Second)
	if err != nil {
		t.Fatalf("ExecuteCommand() error: %v", err)
	}
	if result == "" {
		t.Error("result is empty")
	}
}

func TestCommanderAdapterExecuteTool(t *testing.T) {
	t.Parallel()

	a := NewCommanderAdapter(nil)
	_, err := a.ExecuteTool(context.Background(), "bash", map[string]any{"cmd": "ls"})
	if err == nil {
		t.Error("expected error when no registry configured")
	}
}

func TestCommanderAdapterExecuteToolWithRegistry(t *testing.T) {
	t.Parallel()

	reg := tools.NewRegistryWithoutCache()
	a := NewCommanderAdapter(reg)
	_, err := a.ExecuteTool(context.Background(), "nonexistent_tool", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestCronSchedulerAdapter(t *testing.T) {
	t.Parallel()

	a := NewCronSchedulerAdapter()
	if err := a.ScheduleCron("test-cron", "* * * * *", func() {}); err != nil {
		t.Fatalf("ScheduleCron() error: %v", err)
	}
	if len(a.entries) != 1 {
		t.Errorf("entries count = %d, want 1", len(a.entries))
	}

	if err := a.UnscheduleCron("test-cron"); err != nil {
		t.Fatalf("UnscheduleCron() error: %v", err)
	}
	if len(a.entries) != 0 {
		t.Errorf("entries count after unschedule = %d, want 0", len(a.entries))
	}
}

func TestAlertProviderAdapterNil(t *testing.T) {
	t.Parallel()

	a := AlertProviderAdapter{Engine: nil}
	count, err := a.GetActiveAlertCount("svc1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestRecordingStoreAdapterListRecordings(t *testing.T) {
	t.Parallel()

	a := RecordingStoreAdapter{Store: nil}
	recordings, err := a.ListRecordings()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	_ = recordings
}

func TestTTIncidentStoreAdapterNil(t *testing.T) {
	t.Parallel()

	a := TTIncidentStoreAdapter{IM: nil}
	timeline, err := a.GetIncidentTimeline("inc-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if timeline != nil {
		t.Errorf("timeline = %v, want nil", timeline)
	}

	inc, err := a.GetIncident("inc-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if inc != nil {
		t.Errorf("incident = %v, want nil", inc)
	}
}

func TestFPIncidentStoreAdapterNil(t *testing.T) {
	t.Parallel()

	a := FPIncidentStoreAdapter{IM: nil}
	inc, err := a.GetIncident("inc-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if inc != nil {
		t.Errorf("incident = %v, want nil", inc)
	}

	list, err := a.ListIncidents(10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if list != nil {
		t.Errorf("list = %v, want nil", list)
	}
}

func TestAgentRunnerAdapterNil(t *testing.T) {
	t.Parallel()

	a := AgentRunnerAdapter{API: nil}
	_, err := a.RunAgent(context.Background(), "infra", "analyze this", nil)
	if err == nil {
		t.Error("expected error with nil API client")
	}
}

func TestInnovationShutdown(t *testing.T) {
	t.Parallel()

	closer := NewInnovationShutdown()
	if closer == nil {
		t.Fatal("NewInnovationShutdown() returned nil")
	}

	var _ io.Closer = closer
}
