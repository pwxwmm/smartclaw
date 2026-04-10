package observability

import (
	"testing"
	"time"
)

func TestRecordQueryDuration(t *testing.T) {
	m := NewMetrics()
	m.RecordQueryDuration(100*time.Millisecond, "claude-sonnet-4-5")

	snap := m.Snapshot()
	if snap.QueryCount != 1 {
		t.Errorf("expected QueryCount=1, got %d", snap.QueryCount)
	}
	if snap.QueryTotalTime != 100*time.Millisecond {
		t.Errorf("expected QueryTotalTime=100ms, got %v", snap.QueryTotalTime)
	}
	if snap.ModelQueryCounts["claude-sonnet-4-5"] != 1 {
		t.Errorf("expected model count 1, got %d", snap.ModelQueryCounts["claude-sonnet-4-5"])
	}
}

func TestRecordCacheHit(t *testing.T) {
	m := NewMetrics()
	m.RecordCacheHit(true)
	m.RecordCacheHit(true)
	m.RecordCacheHit(false)

	snap := m.Snapshot()
	if snap.CacheHits != 2 {
		t.Errorf("expected CacheHits=2, got %d", snap.CacheHits)
	}
	if snap.CacheMisses != 1 {
		t.Errorf("expected CacheMisses=1, got %d", snap.CacheMisses)
	}
}

func TestRecordTokenUsage(t *testing.T) {
	m := NewMetrics()
	m.RecordTokenUsage(100, 50, 30, 10, "claude-sonnet-4-5")

	snap := m.Snapshot()
	if snap.TotalInputTokens != 100 {
		t.Errorf("expected TotalInputTokens=100, got %d", snap.TotalInputTokens)
	}
	if snap.TotalOutputTokens != 50 {
		t.Errorf("expected TotalOutputTokens=50, got %d", snap.TotalOutputTokens)
	}
	if snap.TotalCacheRead != 30 {
		t.Errorf("expected TotalCacheRead=30, got %d", snap.TotalCacheRead)
	}
	if snap.TotalCacheCreate != 10 {
		t.Errorf("expected TotalCacheCreate=10, got %d", snap.TotalCacheCreate)
	}
}

func TestRecordMemoryLayerSize(t *testing.T) {
	m := NewMetrics()
	m.RecordMemoryLayerSize("soul", 500)
	m.RecordMemoryLayerSize("memory", 2000)

	snap := m.Snapshot()
	if snap.MemoryLayerSizes["soul"] != 500 {
		t.Errorf("expected soul=500, got %d", snap.MemoryLayerSizes["soul"])
	}
	if snap.MemoryLayerSizes["memory"] != 2000 {
		t.Errorf("expected memory=2000, got %d", snap.MemoryLayerSizes["memory"])
	}
}

func TestRecordToolExecution(t *testing.T) {
	m := NewMetrics()
	m.RecordToolExecution("bash", 50*time.Millisecond, true)
	m.RecordToolExecution("bash", 100*time.Millisecond, false)
	m.RecordToolExecution("read_file", 10*time.Millisecond, true)

	snap := m.Snapshot()
	bashStats, ok := snap.ToolExecutions["bash"]
	if !ok {
		t.Fatal("expected bash stats")
	}
	if bashStats.Count != 2 {
		t.Errorf("expected bash Count=2, got %d", bashStats.Count)
	}
	if bashStats.Errors != 1 {
		t.Errorf("expected bash Errors=1, got %d", bashStats.Errors)
	}
	if bashStats.Duration != 150*time.Millisecond {
		t.Errorf("expected bash Duration=150ms, got %v", bashStats.Duration)
	}

	readStats, ok := snap.ToolExecutions["read_file"]
	if !ok {
		t.Fatal("expected read_file stats")
	}
	if readStats.Count != 1 {
		t.Errorf("expected read_file Count=1, got %d", readStats.Count)
	}
}

func TestMetricsReset(t *testing.T) {
	m := NewMetrics()
	m.RecordQueryDuration(100*time.Millisecond, "test")
	m.RecordCacheHit(true)
	m.RecordTokenUsage(10, 5, 0, 0, "test")
	m.RecordToolExecution("bash", 50*time.Millisecond, true)

	m.Reset()

	snap := m.Snapshot()
	if snap.QueryCount != 0 {
		t.Errorf("expected QueryCount=0 after reset, got %d", snap.QueryCount)
	}
	if snap.CacheHits != 0 {
		t.Errorf("expected CacheHits=0 after reset, got %d", snap.CacheHits)
	}
	if len(snap.ToolExecutions) != 0 {
		t.Errorf("expected empty ToolExecutions after reset, got %d", len(snap.ToolExecutions))
	}
}

func TestDefaultMetricsDelegates(t *testing.T) {
	DefaultMetrics.Reset()
	RecordQueryDuration(50*time.Millisecond, "test-model")
	RecordCacheHit(true)
	RecordTokenUsage(10, 5, 0, 0, "test-model")
	RecordMemoryLayerSize("test-layer", 100)
	RecordToolExecution("test-tool", 10*time.Millisecond, true)

	snap := DefaultMetrics.Snapshot()
	if snap.QueryCount != 1 {
		t.Errorf("expected QueryCount=1, got %d", snap.QueryCount)
	}
	if snap.CacheHits != 1 {
		t.Errorf("expected CacheHits=1, got %d", snap.CacheHits)
	}
	if snap.MemoryLayerSizes["test-layer"] != 100 {
		t.Errorf("expected test-layer=100, got %d", snap.MemoryLayerSizes["test-layer"])
	}

	DefaultMetrics.Reset()
}

func TestMetricsSnapshotIsolation(t *testing.T) {
	m := NewMetrics()
	m.RecordQueryDuration(100*time.Millisecond, "test")

	snap := m.Snapshot()
	m.RecordQueryDuration(200*time.Millisecond, "test")

	if snap.QueryCount != 1 {
		t.Errorf("snapshot should not change after further recordings, got %d", snap.QueryCount)
	}
}
