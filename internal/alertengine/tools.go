package alertengine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/tools"
)

var (
	defaultEngineMu sync.RWMutex
	defaultEngine   *AlertEngine
)

// SetAlertEngine sets the global alert engine singleton.
func SetAlertEngine(e *AlertEngine) {
	defaultEngineMu.Lock()
	defer defaultEngineMu.Unlock()
	defaultEngine = e
}

// DefaultAlertEngine returns the global alert engine singleton.
func DefaultAlertEngine() *AlertEngine {
	defaultEngineMu.RLock()
	defer defaultEngineMu.RUnlock()
	return defaultEngine
}

// InitAlertEngine creates a new AlertEngine, optionally configures
// a topology provider, and sets it as the global singleton.
func InitAlertEngine(topologyProvider TopologyProvider) *AlertEngine {
	e := NewAlertEngine()
	if topologyProvider != nil {
		e.SetTopologyProvider(topologyProvider)
	}
	SetAlertEngine(e)
	return e
}

// AlertIngestTool ingests one or more alerts into the engine.
type AlertIngestTool struct{}

func (t *AlertIngestTool) Name() string { return "alert_ingest" }

func (t *AlertIngestTool) Description() string {
	return "Ingest one or more alerts into the alert deduplication and correlation engine. Each alert is fingerprinted and deduplicated automatically."
}

func (t *AlertIngestTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"alerts": map[string]any{
				"type":        "array",
				"description": "Array of alert objects to ingest",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"source":      map[string]any{"type": "string", "description": "Alert source (prometheus, datadog, pagerduty, zabbix, cloudwatch, custom)"},
						"name":        map[string]any{"type": "string", "description": "Alert name/rule"},
						"severity":    map[string]any{"type": "string", "description": "Severity level (critical, high, medium, low, info)"},
						"service":     map[string]any{"type": "string", "description": "Affected service"},
						"status":      map[string]any{"type": "string", "description": "Alert status (firing, resolved, acknowledged, silenced)"},
						"labels":      map[string]any{"type": "object", "description": "Labels/tags from source"},
						"annotations": map[string]any{"type": "object", "description": "Annotations (description, runbook_url, etc.)"},
						"fired_at":    map[string]any{"type": "string", "description": "Time alert fired (RFC3339)"},
					},
					"required": []string{"source", "name", "severity", "service"},
				},
			},
		},
		"required": []string{"alerts"},
	}
}

func (t *AlertIngestTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	engine := DefaultAlertEngine()
	if engine == nil {
		return nil, fmt.Errorf("alert engine not initialized; call InitAlertEngine first")
	}

	alertsRaw, ok := input["alerts"]
	if !ok {
		return nil, fmt.Errorf("alerts field is required")
	}

	alertsSlice, ok := alertsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("alerts must be an array")
	}

	alerts := make([]Alert, 0, len(alertsSlice))
	for i, aRaw := range alertsSlice {
		aMap, ok := aRaw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("alerts[%d] must be an object", i)
		}

		alert, err := parseAlert(aMap)
		if err != nil {
			return nil, fmt.Errorf("alerts[%d]: %w", i, err)
		}
		alerts = append(alerts, alert)
	}

	results := engine.IngestBatch(alerts)
	return map[string]any{
		"ingested": len(alerts),
		"deduped":  results,
	}, nil
}

// AlertQueryTool queries deduped alerts with optional filters.
type AlertQueryTool struct{}

func (t *AlertQueryTool) Name() string { return "alert_query" }

func (t *AlertQueryTool) Description() string {
	return "Query deduped alerts from the alert engine with optional filters for service, severity, and time range."
}

func (t *AlertQueryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"service":  map[string]any{"type": "string", "description": "Filter by service name"},
			"severity": map[string]any{"type": "string", "description": "Filter by severity level"},
			"since":    map[string]any{"type": "string", "description": "Filter alerts since this time (RFC3339)"},
			"limit":    map[string]any{"type": "integer", "description": "Maximum results to return", "default": 50},
		},
	}
}

func (t *AlertQueryTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	engine := DefaultAlertEngine()
	if engine == nil {
		return nil, fmt.Errorf("alert engine not initialized; call InitAlertEngine first")
	}

	service, _ := input["service"].(string)
	severity, _ := input["severity"].(string)

	var since time.Time
	if sinceStr, ok := input["since"].(string); ok && sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return nil, fmt.Errorf("invalid since time: %w", err)
		}
		since = t
	}

	limit := 50
	if v, ok := input["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	results := engine.Query(service, severity, since)
	if len(results) > limit {
		results = results[:limit]
	}

	return map[string]any{
		"alerts": results,
		"count":  len(results),
	}, nil
}

// AlertCorrelateTool runs the full correlation pipeline.
type AlertCorrelateTool struct{}

func (t *AlertCorrelateTool) Name() string { return "alert_correlate" }

func (t *AlertCorrelateTool) Description() string {
	return "Run the full alert correlation pipeline (fingerprint dedup → time-window grouping → topology-aware correlation) on current alerts."
}

func (t *AlertCorrelateTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"include_topology": map[string]any{
				"type":        "boolean",
				"description": "Include topology-aware correlation stage",
				"default":     true,
			},
		},
	}
}

func (t *AlertCorrelateTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	engine := DefaultAlertEngine()
	if engine == nil {
		return nil, fmt.Errorf("alert engine not initialized; call InitAlertEngine first")
	}

	includeTopology := true
	if v, ok := input["include_topology"].(bool); ok {
		includeTopology = v
	}

	if !includeTopology {
		engine.SetTopologyProvider(nil)
	}

	result := engine.Correlate()
	return result, nil
}

func RegisterAllTools() {
	tools.Register(&AlertIngestTool{})
	tools.Register(&AlertQueryTool{})
	tools.Register(&AlertCorrelateTool{})
}

func parseAlert(m map[string]any) (Alert, error) {
	source, _ := m["source"].(string)
	name, _ := m["name"].(string)
	severity, _ := m["severity"].(string)
	service, _ := m["service"].(string)
	status, _ := m["status"].(string)

	if source == "" {
		return Alert{}, fmt.Errorf("source is required")
	}
	if name == "" {
		return Alert{}, fmt.Errorf("name is required")
	}
	if severity == "" {
		return Alert{}, fmt.Errorf("severity is required")
	}
	if service == "" {
		return Alert{}, fmt.Errorf("service is required")
	}

	if status == "" {
		status = "firing"
	}

	labels := make(map[string]string)
	if l, ok := m["labels"].(map[string]any); ok {
		for k, v := range l {
			if s, ok := v.(string); ok {
				labels[k] = s
			}
		}
	}

	annotations := make(map[string]string)
	if a, ok := m["annotations"].(map[string]any); ok {
		for k, v := range a {
			if s, ok := v.(string); ok {
				annotations[k] = s
			}
		}
	}

	firedAt := time.Now()
	if fa, ok := m["fired_at"].(string); ok && fa != "" {
		t, err := time.Parse(time.RFC3339, fa)
		if err != nil {
			return Alert{}, fmt.Errorf("invalid fired_at time: %w", err)
		}
		firedAt = t
	}

	return Alert{
		Source:      source,
		Name:        name,
		Severity:    severity,
		Status:      status,
		Service:     service,
		Labels:      labels,
		Annotations: annotations,
		FiredAt:     firedAt,
	}, nil
}
