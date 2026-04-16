package observability

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	queriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "smartclaw_queries_total",
		Help: "Total number of queries processed",
	}, []string{"model"})

	queryDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "smartclaw_query_duration_seconds",
		Help:    "Query duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"model"})

	tokensTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "smartclaw_tokens_total",
		Help: "Total tokens used",
	}, []string{"model", "type"}) // type: input, output, cache_read, cache_create

	toolExecutions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "smartclaw_tool_executions_total",
		Help: "Total tool executions",
	}, []string{"tool", "status"}) // status: success, error

	toolDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "smartclaw_tool_duration_seconds",
		Help:    "Tool execution duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"tool"})

	cacheOperations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "smartclaw_cache_operations_total",
		Help: "Cache hit/miss operations",
	}, []string{"result"}) // result: hit, miss

	memoryLayerSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "smartclaw_memory_layer_size_chars",
		Help: "Memory layer size in characters",
	}, []string{"layer"})

	activeSessions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "smartclaw_active_sessions",
		Help: "Number of active sessions",
	})

	promOnce sync.Once
)

// InitPrometheus registers all Prometheus metrics with the default registry.
// It is safe to call multiple times; subsequent calls are no-ops.
func InitPrometheus() {
	promOnce.Do(func() {
		prometheus.MustRegister(queriesTotal)
		prometheus.MustRegister(queryDuration)
		prometheus.MustRegister(tokensTotal)
		prometheus.MustRegister(toolExecutions)
		prometheus.MustRegister(toolDuration)
		prometheus.MustRegister(cacheOperations)
		prometheus.MustRegister(memoryLayerSize)
		prometheus.MustRegister(activeSessions)
	})
}

// PrometheusHandler returns an http.Handler that serves Prometheus metrics.
func PrometheusHandler() http.Handler {
	return promhttp.Handler()
}

// RecordPrometheusQuery records a query duration in Prometheus metrics.
func RecordPrometheusQuery(duration time.Duration, model string) {
	queriesTotal.WithLabelValues(model).Inc()
	queryDuration.WithLabelValues(model).Observe(duration.Seconds())
}

// RecordPrometheusTokens records token usage in Prometheus metrics.
func RecordPrometheusTokens(input, output, cacheRead, cacheCreate int, model string) {
	tokensTotal.WithLabelValues(model, "input").Add(float64(input))
	tokensTotal.WithLabelValues(model, "output").Add(float64(output))
	tokensTotal.WithLabelValues(model, "cache_read").Add(float64(cacheRead))
	tokensTotal.WithLabelValues(model, "cache_create").Add(float64(cacheCreate))
}

// RecordPrometheusToolExecution records a tool execution in Prometheus metrics.
func RecordPrometheusToolExecution(toolName string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	toolExecutions.WithLabelValues(toolName, status).Inc()
	toolDuration.WithLabelValues(toolName).Observe(duration.Seconds())
}

// RecordPrometheusCacheOperation records a cache hit/miss in Prometheus metrics.
func RecordPrometheusCacheOperation(hit bool) {
	result := "miss"
	if hit {
		result = "hit"
	}
	cacheOperations.WithLabelValues(result).Inc()
}

// RecordPrometheusMemoryLayerSize records memory layer size in Prometheus metrics.
func RecordPrometheusMemoryLayerSize(layerName string, chars int) {
	memoryLayerSize.WithLabelValues(layerName).Set(float64(chars))
}

// SetPrometheusActiveSessions sets the active sessions gauge.
func SetPrometheusActiveSessions(count float64) {
	activeSessions.Set(count)
}
