package server

import (
	"net/http"
	"time"

	"github.com/instructkr/smartclaw/internal/observability"
)

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (s *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"tokens_limit": 200000,
		"model":        "sre-model",
		"ws_clients":   s.hub.ClientCount(),
	})
}

func (s *APIServer) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	snapshot := observability.DefaultMetrics.Snapshot()

	writeJSON(w, http.StatusOK, map[string]any{
		"query_count":         snapshot.QueryCount,
		"query_total_time_ms": snapshot.QueryTotalTime.Milliseconds(),
		"cache_hits":          snapshot.CacheHits,
		"cache_misses":        snapshot.CacheMisses,
		"cache_hit_rate":      cacheHitRate(snapshot.CacheHits, snapshot.CacheMisses),
		"total_input_tokens":  snapshot.TotalInputTokens,
		"total_output_tokens": snapshot.TotalOutputTokens,
		"total_cache_read":    snapshot.TotalCacheRead,
		"total_cache_create":  snapshot.TotalCacheCreate,
		"estimated_cost_usd":  estimateCost(snapshot),
		"tool_executions":     snapshot.ToolExecutions,
		"memory_layer_sizes":  snapshot.MemoryLayerSizes,
		"model_query_counts":  snapshot.ModelQueryCounts,
		"timestamp":           time.Now().Format(time.RFC3339),
	})
}
