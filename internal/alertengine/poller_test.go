package alertengine

import (
	"testing"
	"time"
)

func TestSopaAlertEventToAlert(t *testing.T) {
	event := map[string]any{
		"id":          "evt-001",
		"name":        "GPU Memory High",
		"source":      "prometheus",
		"severity":    "critical",
		"description": "GPU memory usage exceeds 95%",
		"conditions": map[string]any{
			"service": "gpu-cluster-01",
			"region":  "us-east",
		},
		"actions": "notify",
		"enabled": true,
	}

	alert := sopaAlertEventToAlert(event)

	if alert.Source != "sopa" {
		t.Errorf("expected source 'sopa', got %q", alert.Source)
	}
	if alert.Name != "GPU Memory High" {
		t.Errorf("expected name 'GPU Memory High', got %q", alert.Name)
	}
	if alert.Severity != "critical" {
		t.Errorf("expected severity 'critical', got %q", alert.Severity)
	}
	if alert.Service != "gpu-cluster-01" {
		t.Errorf("expected service 'gpu-cluster-01', got %q", alert.Service)
	}
	if alert.Status != "firing" {
		t.Errorf("expected status 'firing', got %q", alert.Status)
	}
	if alert.Labels["region"] != "us-east" {
		t.Errorf("expected label region='us-east', got %q", alert.Labels["region"])
	}
	if alert.Labels["sopa_event_source"] != "prometheus" {
		t.Errorf("expected label sopa_event_source='prometheus', got %q", alert.Labels["sopa_event_source"])
	}
	if alert.Annotations["description"] != "GPU memory usage exceeds 95%" {
		t.Errorf("expected annotation description, got %q", alert.Annotations["description"])
	}
	if alert.Annotations["sopa_severity"] != "critical" {
		t.Errorf("expected annotation sopa_severity='critical', got %q", alert.Annotations["sopa_severity"])
	}
}

func TestSopaAlertEventToAlert_SeverityMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "critical"},
		{"urgent", "critical"},
		{"P1", "critical"},
		{"high", "high"},
		{"major", "high"},
		{"P2", "high"},
		{"medium", "medium"},
		{"warning", "medium"},
		{"P3", "medium"},
		{"low", "low"},
		{"minor", "low"},
		{"P4", "low"},
		{"info", "info"},
		{"informational", "info"},
		{"P5", "info"},
		{"", "medium"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := mapSopaSeverity(tt.input)
		if result != tt.expected {
			t.Errorf("mapSopaSeverity(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestSopaAlertEventToAlert_ServiceExtraction(t *testing.T) {
	tests := []struct {
		name     string
		event    map[string]any
		expected string
	}{
		{
			name: "from service field",
			event: map[string]any{
				"service": "my-service",
				"source":  "prometheus",
			},
			expected: "my-service",
		},
		{
			name: "from conditions node",
			event: map[string]any{
				"source": "prometheus",
				"conditions": map[string]any{
					"node": "node3",
				},
			},
			expected: "node3",
		},
		{
			name: "from conditions cluster",
			event: map[string]any{
				"source": "prometheus",
				"conditions": map[string]any{
					"cluster": "gpu-cluster-01",
				},
			},
			expected: "gpu-cluster-01",
		},
		{
			name: "from source fallback",
			event: map[string]any{
				"source": "datadog",
			},
			expected: "datadog",
		},
		{
			name:     "unknown when empty",
			event:    map[string]any{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractService(tt.event)
			if result != tt.expected {
				t.Errorf("extractService() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNewSopaAlertPoller_Defaults(t *testing.T) {
	engine := NewAlertEngine()
	poller := NewSopaAlertPoller(engine, "", "", 0)

	if poller.baseURL != sopaPollerDefaultBaseURL {
		t.Errorf("expected default baseURL %q, got %q", sopaPollerDefaultBaseURL, poller.baseURL)
	}
	if poller.interval != 30*time.Second {
		t.Errorf("expected default interval 30s, got %v", poller.interval)
	}
	if poller.engine != engine {
		t.Error("expected engine to be set")
	}
}
