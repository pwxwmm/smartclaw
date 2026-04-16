package timetravel

import (
	"context"
	"fmt"
	"sync"

	"github.com/instructkr/smartclaw/internal/tools"
)

var (
	defaultEngineMu sync.RWMutex
	defaultEngine   *TimeTravelEngine
)

func SetTimeTravelEngine(e *TimeTravelEngine) {
	defaultEngineMu.Lock()
	defer defaultEngineMu.Unlock()
	defaultEngine = e
}

func DefaultTimeTravelEngine() *TimeTravelEngine {
	defaultEngineMu.RLock()
	defer defaultEngineMu.RUnlock()
	return defaultEngine
}

func InitTimeTravelEngine(rs RecordingStore, is IncidentStore) *TimeTravelEngine {
	e := NewTimeTravelEngine()
	if rs != nil {
		e.SetRecordingStore(rs)
	}
	if is != nil {
		e.SetIncidentStore(is)
	}
	SetTimeTravelEngine(e)
	return e
}

func RegisterAllTools() {
	tools.Register(&ReplayIncidentTool{})
}

type ReplayIncidentTool struct{}

func (t *ReplayIncidentTool) Name() string { return "replay_incident" }

func (t *ReplayIncidentTool) Description() string {
	return "Replay and analyze a past incident or session recording for investigation review and what-if analysis. Use for post-incident review and learning from past incidents."
}

func (t *ReplayIncidentTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"source": map[string]any{
				"type":        "string",
				"description": "What to replay: 'incident' (incident ID) or 'recording' (file path)",
				"enum":        []string{"incident", "recording"},
			},
			"source_id": map[string]any{
				"type":        "string",
				"description": "Incident ID or recording file path",
			},
			"analyze": map[string]any{
				"type":        "boolean",
				"description": "Whether to run timeline analysis",
				"default":     true,
			},
		},
		"required": []string{"source", "source_id"},
	}
}

func (t *ReplayIncidentTool) Execute(ctx context.Context, input map[string]any) (any, error) {
	source, _ := input["source"].(string)
	sourceID, _ := input["source_id"].(string)
	analyze := true
	if v, ok := input["analyze"].(bool); ok {
		analyze = v
	}

	if source == "" {
		return nil, fmt.Errorf("source is required")
	}
	if sourceID == "" {
		return nil, fmt.Errorf("source_id is required")
	}

	engine := DefaultTimeTravelEngine()
	if engine == nil {
		return nil, fmt.Errorf("time travel engine not initialized")
	}

	var session *ReplaySession
	var err error

	switch source {
	case "incident":
		session, err = engine.ReplayIncident(ctx, sourceID)
	case "recording":
		session, err = engine.ReplayRecording(ctx, sourceID)
	default:
		return nil, fmt.Errorf("invalid source '%s'; must be 'incident' or 'recording'", source)
	}

	if err != nil {
		return nil, err
	}

	if !analyze {
		session.Summary = nil
	}

	return session, nil
}
