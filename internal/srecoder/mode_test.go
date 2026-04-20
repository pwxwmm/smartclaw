package srecoder

import (
	"strings"
	"testing"
)

func TestSRECodingMode_DisabledByDefault(t *testing.T) {
	m := NewSRECodingMode()
	if m.IsEnabled() {
		t.Error("expected SRE mode to be disabled by default")
	}
}

func TestSRECodingMode_Enable(t *testing.T) {
	m := NewSRECodingMode()
	m.Enable()
	if !m.IsEnabled() {
		t.Error("expected SRE mode to be enabled after Enable()")
	}
}

func TestSRECodingMode_Disable(t *testing.T) {
	m := NewSRECodingMode()
	m.Enable()
	if !m.IsEnabled() {
		t.Error("expected enabled after Enable()")
	}
	m.Disable()
	if m.IsEnabled() {
		t.Error("expected disabled after Disable()")
	}
}

func TestSRECodingMode_GetSystemPromptAddition_Disabled(t *testing.T) {
	m := NewSRECodingMode()
	addition := m.GetSystemPromptAddition()
	if addition != "" {
		t.Errorf("expected empty string when disabled, got %q", addition)
	}
}

func TestSRECodingMode_GetSystemPromptAddition_Enabled(t *testing.T) {
	m := NewSRECodingMode()
	m.Enable()
	addition := m.GetSystemPromptAddition()

	if addition == "" {
		t.Error("expected non-empty system prompt addition when enabled")
	}
	if !strings.Contains(addition, "SRE-Aware Coding Mode") {
		t.Error("expected prompt to contain 'SRE-Aware Coding Mode'")
	}
	if !strings.Contains(addition, "operational impact") {
		t.Error("expected prompt to mention operational impact")
	}
	if !strings.Contains(addition, "error handling") {
		t.Error("expected prompt to mention error handling")
	}
	if !strings.Contains(addition, "health check") {
		t.Error("expected prompt to mention health check")
	}
	if !strings.Contains(addition, "metrics") {
		t.Error("expected prompt to mention metrics")
	}
}

func TestSRECodingMode_GetSystemPromptAddition_TopologyInfo(t *testing.T) {
	m := NewSRECodingMode()
	m.Enable()
	addition := m.GetSystemPromptAddition()

	if !strings.Contains(addition, "Topology") {
		t.Error("expected prompt to contain Topology info")
	}
}

func TestSRECodingMode_GetSystemPromptAddition_AlertsInfo(t *testing.T) {
	m := NewSRECodingMode()
	m.Enable()
	addition := m.GetSystemPromptAddition()

	if !strings.Contains(addition, "Alerts") {
		t.Error("expected prompt to contain Alerts info")
	}
}

func TestSRECodingMode_Status_Disabled(t *testing.T) {
	m := NewSRECodingMode()
	status := m.Status()
	if status != "SRE-aware coding mode: OFF" {
		t.Errorf("unexpected status: %s", status)
	}
}

func TestSRECodingMode_Status_Enabled(t *testing.T) {
	m := NewSRECodingMode()
	m.Enable()
	status := m.Status()
	if !strings.Contains(status, "SRE-aware coding mode: ON") {
		t.Errorf("expected ON status, got: %s", status)
	}
}

func TestSRECodingMode_ConcurrentAccess(t *testing.T) {
	m := NewSRECodingMode()
	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			m.Enable()
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			m.Disable()
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 100; i++ {
			_ = m.IsEnabled()
		}
		done <- struct{}{}
	}()

	for i := 0; i < 3; i++ {
		<-done
	}
}
