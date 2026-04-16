package srecoder

import (
	"fmt"
	"strings"
	"sync"

	"github.com/instructkr/smartclaw/internal/alertengine"
	"github.com/instructkr/smartclaw/internal/autoremediation"
	"github.com/instructkr/smartclaw/internal/changerisk"
	"github.com/instructkr/smartclaw/internal/topology"
)

// SRECodingMode enriches code generation with SRE context. When enabled,
// it connects the coding workflow to SmartClaw's SRE infrastructure packages:
// topology (blast radius), changerisk (risk assessment), alertengine (active
// alerts), and autoremediation (runbooks).
type SRECodingMode struct {
	mu       sync.RWMutex
	enabled  bool
	topology *topology.TopologyGraph
	risk     *changerisk.ChangeRiskChecker
	alerts   *alertengine.AlertEngine
	runbooks *autoremediation.RemediationEngine
}

// Option configures an SRECodingMode instance.
type Option func(*SRECodingMode)

// WithTopology sets the topology graph for blast radius calculations.
func WithTopology(g *topology.TopologyGraph) Option {
	return func(m *SRECodingMode) { m.topology = g }
}

// WithChangeRisk sets the change risk checker.
func WithChangeRisk(c *changerisk.ChangeRiskChecker) Option {
	return func(m *SRECodingMode) { m.risk = c }
}

// WithAlerts sets the alert engine for active alert awareness.
func WithAlerts(e *alertengine.AlertEngine) Option {
	return func(m *SRECodingMode) { m.alerts = e }
}

// WithRunbooks sets the remediation engine for runbook lookups.
func WithRunbooks(e *autoremediation.RemediationEngine) Option {
	return func(m *SRECodingMode) { m.runbooks = e }
}

// NewSRECodingMode creates a new SRECodingMode with the given options.
// If no options are provided, it attempts to use the global singletons
// from each SRE package. Missing components are handled gracefully.
func NewSRECodingMode(opts ...Option) *SRECodingMode {
	m := &SRECodingMode{}
	for _, opt := range opts {
		opt(m)
	}

	// Fall back to global singletons if not explicitly provided.
	if m.topology == nil {
		m.topology = topology.DefaultTopology()
	}
	if m.risk == nil {
		m.risk = changerisk.DefaultChangeRiskChecker()
	}
	if m.alerts == nil {
		m.alerts = alertengine.DefaultAlertEngine()
	}
	if m.runbooks == nil {
		m.runbooks = autoremediation.DefaultRemediationEngine()
	}

	return m
}

// IsEnabled returns whether SRE-aware coding mode is active.
func (m *SRECodingMode) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// Enable activates SRE-aware coding mode.
func (m *SRECodingMode) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
}

// Disable deactivates SRE-aware coding mode.
func (m *SRECodingMode) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// GetTopology returns the configured topology graph (may be nil).
func (m *SRECodingMode) GetTopology() *topology.TopologyGraph {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.topology
}

// GetChangeRisk returns the configured change risk checker (may be nil).
func (m *SRECodingMode) GetChangeRisk() *changerisk.ChangeRiskChecker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.risk
}

// GetAlerts returns the configured alert engine (may be nil).
func (m *SRECodingMode) GetAlerts() *alertengine.AlertEngine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts
}

// GetRunbooks returns the configured remediation engine (may be nil).
func (m *SRECodingMode) GetRunbooks() *autoremediation.RemediationEngine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runbooks
}

// GetSystemPromptAddition returns additional system prompt text that instructs
// the LLM about SRE awareness. This should be appended to the system prompt
// when SRE mode is enabled.
func (m *SRECodingMode) GetSystemPromptAddition() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("\n\n## SRE-Aware Coding Mode\n\n")
	sb.WriteString("You are operating in SRE-aware mode. When writing or modifying code:\n\n")
	sb.WriteString("1. **Consider operational impact**: What services are affected by this change? ")
	sb.WriteString("Use the impact analysis tools to understand blast radius.\n")
	sb.WriteString("2. **Add appropriate error handling**: Circuit breakers, retries with exponential backoff, timeouts with context propagation.\n")
	sb.WriteString("3. **Add health check endpoints**: Expose /healthz (liveness) and /readyz (readiness) for new services.\n")
	sb.WriteString("4. **Add metrics/observability**: Prometheus metrics (counter, gauge, histogram), structured logging with trace IDs, tracing spans.\n")
	sb.WriteString("5. **Consider graceful degradation patterns**: Fallback responses, feature flags, bulkhead isolation.\n")
	sb.WriteString("6. **Document runbook-worthy operations**: Any manual intervention steps should be documented as runbook entries.\n")
	sb.WriteString("7. **Reference existing runbooks**: When modifying failure-prone areas, check for and reference existing runbooks.\n\n")

	// Add dynamic context about available SRE infrastructure.
	sb.WriteString("### Available SRE Infrastructure\n\n")

	if m.topology != nil {
		stats := m.topology.Stats()
		sb.WriteString(fmt.Sprintf("- **Topology**: %d services, %d dependencies tracked\n", stats.NodeCount, stats.EdgeCount))
		if len(stats.HealthCounts) > 0 {
			var parts []string
			for status, count := range stats.HealthCounts {
				parts = append(parts, fmt.Sprintf("%d %s", count, status))
			}
			sb.WriteString(fmt.Sprintf("  - Health: %s\n", strings.Join(parts, ", ")))
		}
	} else {
		sb.WriteString("- **Topology**: Not configured (blast radius analysis unavailable)\n")
	}

	if m.risk != nil {
		sb.WriteString("- **Change Risk**: Risk assessment available (blast radius, incident history, SLO burn, change failure rate)\n")
	} else {
		sb.WriteString("- **Change Risk**: Not configured\n")
	}

	if m.alerts != nil {
		alertStats := m.alerts.Stats()
		sb.WriteString(fmt.Sprintf("- **Alerts**: %d active deduped alerts, %d alert groups\n", alertStats.TotalDeduped, alertStats.TotalGroups))
	} else {
		sb.WriteString("- **Alerts**: Not configured\n")
	}

	if m.runbooks != nil {
		actions := m.runbooks.ListActions("")
		activeCount := 0
		for _, a := range actions {
			if a.Status == autoremediation.ActionRunning || a.Status == autoremediation.ActionPending {
				activeCount++
			}
		}
		sb.WriteString(fmt.Sprintf("- **Runbooks**: Auto-remediation engine available (%d active actions)\n", activeCount))
	} else {
		sb.WriteString("- **Runbooks**: Not configured\n")
	}

	sb.WriteString("\nWhen you modify code, automatically run impact analysis and suggest SRE improvements.\n")

	return sb.String()
}

// Status returns a human-readable status string for the SRE mode.
func (m *SRECodingMode) Status() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return "SRE-aware coding mode: OFF"
	}

	var parts []string
	parts = append(parts, "SRE-aware coding mode: ON")

	if m.topology != nil {
		stats := m.topology.Stats()
		parts = append(parts, fmt.Sprintf("topology(%d nodes)", stats.NodeCount))
	} else {
		parts = append(parts, "topology(n/a)")
	}

	if m.risk != nil {
		parts = append(parts, "risk-checker(available)")
	} else {
		parts = append(parts, "risk-checker(n/a)")
	}

	if m.alerts != nil {
		s := m.alerts.Stats()
		parts = append(parts, fmt.Sprintf("alerts(%d active)", s.TotalDeduped))
	} else {
		parts = append(parts, "alerts(n/a)")
	}

	if m.runbooks != nil {
		parts = append(parts, "runbooks(available)")
	} else {
		parts = append(parts, "runbooks(n/a)")
	}

	return strings.Join(parts, " | ")
}

// Global singleton for the SRE coding mode.
var (
	globalModeMu sync.RWMutex
	globalMode   *SRECodingMode
)

// SetGlobalMode sets the global SRE coding mode instance.
func SetGlobalMode(m *SRECodingMode) {
	globalModeMu.Lock()
	defer globalModeMu.Unlock()
	globalMode = m
}

// GetGlobalMode returns the global SRE coding mode instance.
// Returns nil if not initialized.
func GetGlobalMode() *SRECodingMode {
	globalModeMu.RLock()
	defer globalModeMu.RUnlock()
	return globalMode
}

// InitGlobalMode initializes the global SRE coding mode singleton.
func InitGlobalMode() *SRECodingMode {
	m := NewSRECodingMode()
	SetGlobalMode(m)
	return m
}
