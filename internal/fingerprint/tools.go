package fingerprint

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/tools"
)

var (
	defaultEngineMu sync.RWMutex
	defaultEngine   *FingerprintEngine
)

func SetFingerprintEngine(e *FingerprintEngine) {
	defaultEngineMu.Lock()
	defer defaultEngineMu.Unlock()
	defaultEngine = e
}

func DefaultFingerprintEngine() *FingerprintEngine {
	defaultEngineMu.RLock()
	defer defaultEngineMu.RUnlock()
	return defaultEngine
}

type FingerprintSearchTool struct {
	tools.BaseTool
}

func (t *FingerprintSearchTool) Name() string { return "fingerprint_search" }

func (t *FingerprintSearchTool) Description() string {
	return "Find similar past incidents using fingerprint-based similarity search. Provide an incident_id to search by stored fingerprint, or provide a title for ad-hoc search."
}

func (t *FingerprintSearchTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"incident_id": map[string]any{
				"type":        "string",
				"description": "Find incidents similar to this incident ID (uses stored fingerprint)",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Title for ad-hoc fingerprint generation (used if incident_id not provided)",
			},
			"severity": map[string]any{
				"type":        "string",
				"description": "Filter by severity (info, low, medium, high, critical)",
			},
			"service": map[string]any{
				"type":        "string",
				"description": "Filter by primary service name",
			},
			"threshold": map[string]any{
				"type":        "number",
				"description": "Minimum similarity score (0.0-1.0, default 0.7)",
				"default":     0.7,
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of results (default 10)",
				"default":     10,
			},
		},
	}
}

func (t *FingerprintSearchTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	engine := DefaultFingerprintEngine()
	if engine == nil {
		return nil, fmt.Errorf("fingerprint engine not initialized")
	}

	incidentID, _ := input["incident_id"].(string)
	title, _ := input["title"].(string)
	severity, _ := input["severity"].(string)
	service, _ := input["service"].(string)

	threshold := config.Threshold
	if v, ok := input["threshold"].(float64); ok && v > 0 {
		threshold = v
	}

	limit := 10
	if v, ok := input["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	var results []SimilarityResult
	var err error

	if incidentID != "" {
		results, err = engine.SearchSimilar(incidentID, threshold, limit)
		if err != nil {
			return nil, fmt.Errorf("fingerprint search failed: %w", err)
		}
	} else if title != "" {
		data := IncidentData{
			ID:        "ad-hoc-query",
			Title:     title,
			Severity:  severity,
			Service:   service,
			StartedAt: time.Now().UTC(),
		}
		fp := GenerateFingerprint(data)
		results, err = engine.SearchByVector(fp.Vector, threshold, limit)
		if err != nil {
			return nil, fmt.Errorf("fingerprint search failed: %w", err)
		}
	} else {
		return nil, fmt.Errorf("either incident_id or title must be provided")
	}

	if severity != "" {
		filtered := make([]SimilarityResult, 0, len(results))
		for _, r := range results {
			if r.IncidentSeverity == severity {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	if service != "" {
		filtered := make([]SimilarityResult, 0, len(results))
		for _, r := range results {
			if r.IncidentService == service {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) == 0 {
			filtered = results
		}
		results = filtered
	}

	return map[string]any{
		"results": results,
		"count":   len(results),
		"query": map[string]any{
			"incident_id": incidentID,
			"title":       title,
			"threshold":   threshold,
			"limit":       limit,
		},
	}, nil
}

func RegisterTools(registry *tools.ToolRegistry) {
	registry.Register(&FingerprintSearchTool{})
}
