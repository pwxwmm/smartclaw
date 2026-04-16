package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/instructkr/smartclaw/internal/observability"
)

type AuditQueryTool struct{ BaseTool }

func (t *AuditQueryTool) Name() string        { return "audit_query" }
func (t *AuditQueryTool) Description() string {
	return "Query the append-only audit trail with filters. Returns matching audit entries."
}
func (t *AuditQueryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":       map[string]any{"type": "string", "description": "Filter by entry type: tool_execution, tool_approval, tool_denial, command, chat"},
			"tool":       map[string]any{"type": "string", "description": "Filter by tool name"},
			"actor":      map[string]any{"type": "string", "description": "Filter by actor"},
			"success":    map[string]any{"type": "boolean", "description": "Filter by success status"},
			"start_time": map[string]any{"type": "string", "description": "Start time (RFC3339)"},
			"end_time":   map[string]any{"type": "string", "description": "End time (RFC3339)"},
			"limit":      map[string]any{"type": "integer", "default": 50, "description": "Max entries to return (0-1000)"},
		},
	}
}

func (t *AuditQueryTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	if observability.DefaultAuditLogger == nil {
		return nil, fmt.Errorf("audit logger not initialized")
	}

	filters := observability.AuditFilter{}

	if v, ok := input["type"].(string); ok && v != "" {
		filters.Type = v
	}
	if v, ok := input["tool"].(string); ok && v != "" {
		filters.Tool = v
	}
	if v, ok := input["actor"].(string); ok && v != "" {
		filters.Actor = v
	}
	if v, ok := input["success"].(bool); ok {
		filters.Success = &v
	}
	if v, ok := input["start_time"].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filters.StartTime = &t
		}
	}
	if v, ok := input["end_time"].(string); ok && v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filters.EndTime = &t
		}
	}

	limit := 50
	if v, ok := input["limit"].(int); ok && v > 0 && v <= 1000 {
		limit = v
	}

	entries, err := observability.DefaultAuditLogger.Query(filters)
	if err != nil {
		return nil, fmt.Errorf("audit query failed: %w", err)
	}

	if len(entries) > limit {
		entries = entries[:limit]
	}

	return map[string]any{
		"entries": entries,
		"count":   len(entries),
	}, nil
}

type AuditStatsTool struct{ BaseTool }

func (t *AuditStatsTool) Name() string        { return "audit_stats" }
func (t *AuditStatsTool) Description() string {
	return "Get audit trail statistics: total entries, counts by type and tool, approval and error rates."
}
func (t *AuditStatsTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *AuditStatsTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	if observability.DefaultAuditLogger == nil {
		return nil, fmt.Errorf("audit logger not initialized")
	}

	stats := observability.DefaultAuditLogger.Stats()
	return stats, nil
}
