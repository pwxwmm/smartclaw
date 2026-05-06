package alertengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

const sopaPollerDefaultBaseURL = "http://localhost:8080"

// SopaAlertPoller periodically fetches alert events from the SOPA API
// and ingests them into the AlertEngine.
type SopaAlertPoller struct {
	engine   *AlertEngine
	baseURL  string
	token    string
	interval time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once

	// Track last seen alert event IDs to avoid re-ingesting.
	lastSeenIDs map[string]bool
	mu          sync.Mutex

	client *http.Client
}

// NewSopaAlertPoller creates a new poller that fetches SOPA alert events
// at the given interval and ingests them into the provided AlertEngine.
func NewSopaAlertPoller(engine *AlertEngine, baseURL, token string, interval time.Duration) *SopaAlertPoller {
	if baseURL == "" {
		baseURL = sopaPollerDefaultBaseURL
	}
	if token == "" {
		token = os.Getenv("SOPA_API_TOKEN")
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &SopaAlertPoller{
		engine:      engine,
		baseURL:     baseURL,
		token:       token,
		interval:    interval,
		stopCh:      make(chan struct{}),
		lastSeenIDs: make(map[string]bool),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Start begins the polling loop in a background goroutine.
func (p *SopaAlertPoller) Start(ctx context.Context) {
	go p.run(ctx)
}

// Stop signals the poller to stop.
func (p *SopaAlertPoller) Stop() {
	p.stopOnce.Do(func() {
		close(p.stopCh)
	})
}

func (p *SopaAlertPoller) run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Do an initial poll immediately.
	if err := p.pollOnce(ctx); err != nil {
		slog.Error("alertengine: SOPA poll error", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			if err := p.pollOnce(ctx); err != nil {
				slog.Error("alertengine: SOPA poll error", "error", err)
			}
		}
	}
}

// pollOnce performs a single poll cycle: fetch alert events, convert to Alert, ingest.
func (p *SopaAlertPoller) pollOnce(ctx context.Context) error {
	events, err := p.fetchAlertEvents(ctx)
	if err != nil {
		return fmt.Errorf("fetch alert events: %w", err)
	}

	p.mu.Lock()
	newCount := 0
	for _, event := range events {
		id, _ := event["id"].(string)
		if id == "" {
			continue
		}
		if p.lastSeenIDs[id] {
			continue
		}
		p.lastSeenIDs[id] = true
		newCount++

		alert := sopaAlertEventToAlert(event)
		p.engine.Ingest(alert)
	}

	// Trim lastSeenIDs to keep max 1000.
	if len(p.lastSeenIDs) > 1000 {
		// Simple strategy: clear and re-add only the current batch IDs.
		// This is acceptable because re-ingesting a few duplicates is harmless
		// (they'll be deduplicated by the engine's fingerprint).
		newMap := make(map[string]bool, len(events))
		for _, event := range events {
			id, _ := event["id"].(string)
			if id != "" {
				newMap[id] = true
			}
		}
		p.lastSeenIDs = newMap
	}
	p.mu.Unlock()

	if newCount > 0 {
		slog.Info("alertengine: ingested new SOPA alert events", "count", newCount)
	}

	return nil
}

// fetchAlertEvents calls GET /api/alert-event/list with actionType=notify.
func (p *SopaAlertPoller) fetchAlertEvents(ctx context.Context) ([]map[string]any, error) {
	url := p.baseURL + "/api/alert-event/list?actionType=notify"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Extract the events list from the response.
	// The SOPA API returns { "data": [...], "total": N } or similar.
	var events []map[string]any
	if data, ok := result["data"]; ok {
		if arr, ok := data.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					events = append(events, m)
				}
			}
		}
	} else if items, ok := result["items"]; ok {
		if arr, ok := items.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					events = append(events, m)
				}
			}
		}
	}

	// Fallback: the response itself might be an array at top level.
	if len(events) == 0 {
		if arr, ok := result["list"].([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					events = append(events, m)
				}
			}
		}
	}

	return events, nil
}

// sopaAlertEventToAlert converts a SOPA alert event to an AlertEngine Alert.
//
// SOPA alert event fields: id, name, source, severity, description,
// conditions, actions, enabled.
func sopaAlertEventToAlert(event map[string]any) Alert {
	name, _ := event["name"].(string)
	source, _ := event["source"].(string)
	description, _ := event["description"].(string)

	// Map SOPA severity to alertengine severity.
	sopaSev, _ := event["severity"].(string)
	severity := mapSopaSeverity(sopaSev)

	// Extract service from source or conditions.
	service := extractService(event)

	// Build labels from conditions.
	labels := make(map[string]string)
	if conditions, ok := event["conditions"].(map[string]any); ok {
		for k, v := range conditions {
			if s, ok := v.(string); ok {
				labels[k] = s
			} else {
				labels[k] = fmt.Sprintf("%v", v)
			}
		}
	}
	labels["sopa_event_source"] = source

	// Build annotations from description.
	annotations := make(map[string]string)
	if description != "" {
		annotations["description"] = description
	}
	if sopaSev != "" {
		annotations["sopa_severity"] = sopaSev
	}
	if actions, ok := event["actions"].(string); ok && actions != "" {
		annotations["actions"] = actions
	}

	return Alert{
		Source:      "sopa",
		Name:        name,
		Severity:    severity,
		Status:      "firing",
		Service:     service,
		Labels:      labels,
		Annotations: annotations,
		FiredAt:     time.Now(),
	}
}

// mapSopaSeverity maps SOPA severity strings to alertengine severity levels.
func mapSopaSeverity(sopaSev string) string {
	switch sopaSev {
	case "critical", "urgent", "P1":
		return "critical"
	case "high", "major", "P2":
		return "high"
	case "medium", "warning", "P3":
		return "medium"
	case "low", "minor", "P4":
		return "low"
	case "info", "informational", "P5":
		return "info"
	default:
		if sopaSev != "" {
			return sopaSev
		}
		return "medium"
	}
}

// extractService tries to extract a service name from the SOPA alert event.
func extractService(event map[string]any) string {
	// Try "service" field first.
	if svc, ok := event["service"].(string); ok && svc != "" {
		return svc
	}

	// Try to extract from conditions.
	if conditions, ok := event["conditions"].(map[string]any); ok {
		for _, key := range []string{"service", "node", "host", "cluster", "instance"} {
			if v, ok := conditions[key].(string); ok && v != "" {
				return v
			}
		}
	}

	// Use source as fallback.
	if source, ok := event["source"].(string); ok && source != "" {
		return source
	}

	return "unknown"
}

// sopaPollerAPICall is a helper for making SOPA API calls from the poller.
// It is self-contained and does not depend on the tools package.
func sopaPollerAPICall(client *http.Client, baseURL, token, method, path string, body, result any) error {
	fullURL := baseURL + path

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("SOPA: failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return fmt.Errorf("SOPA: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("SOPA: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("SOPA: failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("SOPA: API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("SOPA: failed to decode response: %w", err)
		}
	}

	return nil
}
