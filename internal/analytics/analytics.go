package analytics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/instructkr/smartclaw/internal/utils"
)

type Event struct {
	Name       string         `json:"name"`
	Timestamp  time.Time      `json:"timestamp"`
	Properties map[string]any `json:"properties,omitempty"`
	UserID     string         `json:"user_id,omitempty"`
	SessionID  string         `json:"session_id,omitempty"`
}

type AnalyticsConfig struct {
	Enabled       bool
	UserID        string
	SessionID     string
	FlushInterval time.Duration
	MaxBatchSize  int
}

type AnalyticsSink interface {
	Send(events []Event) error
}

type Analytics struct {
	config    AnalyticsConfig
	events    []Event
	sinks     []AnalyticsSink
	mu        sync.Mutex
	flushMu   sync.Mutex
	flushSem  chan struct{}
	flushTick *time.Ticker
	stopChan  chan struct{}
}

func NewAnalytics(config AnalyticsConfig) *Analytics {
	if config.FlushInterval == 0 {
		config.FlushInterval = 30 * time.Second
	}
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 100
	}

	a := &Analytics{
		config:   config,
		events:   make([]Event, 0),
		sinks:    make([]AnalyticsSink, 0),
		flushSem: make(chan struct{}, 1),
		stopChan: make(chan struct{}),
	}

	if config.Enabled {
		utils.Go(a.flushLoop)
	}

	return a
}

func (a *Analytics) Track(name string, properties map[string]any) {
	if !a.config.Enabled {
		return
	}

	event := Event{
		Name:       name,
		Timestamp:  time.Now(),
		Properties: properties,
		UserID:     a.config.UserID,
		SessionID:  a.config.SessionID,
	}

	a.mu.Lock()
	a.events = append(a.events, event)
	shouldFlush := len(a.events) >= a.config.MaxBatchSize
	a.mu.Unlock()

	if shouldFlush {
		select {
		case a.flushSem <- struct{}{}:
			utils.Go(func() {
				defer func() { <-a.flushSem }()
				a.Flush()
			})
		default:
		}
	}
}

func (a *Analytics) AddSink(sink AnalyticsSink) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sinks = append(a.sinks, sink)
}

func (a *Analytics) Flush() error {
	a.flushMu.Lock()
	defer a.flushMu.Unlock()

	a.mu.Lock()
	if len(a.events) == 0 {
		a.mu.Unlock()
		return nil
	}

	events := make([]Event, len(a.events))
	copy(events, a.events)
	a.events = make([]Event, 0)
	sinks := make([]AnalyticsSink, len(a.sinks))
	copy(sinks, a.sinks)
	a.mu.Unlock()

	var lastErr error
	for _, sink := range sinks {
		if err := sink.Send(events); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

func (a *Analytics) flushLoop() {
	ticker := time.NewTicker(a.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.Flush()
		case <-a.stopChan:
			a.Flush()
			return
		}
	}
}

func (a *Analytics) Close() {
	close(a.stopChan)
	a.Flush()
}

type ConsoleSink struct{}

func NewConsoleSink() *ConsoleSink {
	return &ConsoleSink{}
}

func (s *ConsoleSink) Send(events []Event) error {
	for _, event := range events {
		data, _ := json.Marshal(event)
		fmt.Printf("[Analytics] %s\n", string(data))
	}
	return nil
}

type FileSink struct {
	path string
	mu   sync.Mutex
}

func NewFileSink(path string) (*FileSink, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	return &FileSink{path: path}, nil
}

func (s *FileSink) Send(events []Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}

	return nil
}

type DatadogSink struct {
	apiKey  string
	appKey  string
	baseURL string
}

func NewDatadogSink(apiKey, appKey string) *DatadogSink {
	return &DatadogSink{
		apiKey:  apiKey,
		appKey:  appKey,
		baseURL: "https://api.datadoghq.com/api/v1",
	}
}

func (s *DatadogSink) Send(events []Event) error {
	return nil
}

type GrowthBookClient struct {
	apiKey    string
	baseURL   string
	cache     map[string]any
	cacheTime map[string]time.Time
	cacheTTL  time.Duration
	mu        sync.RWMutex
}

func NewGrowthBookClient(apiKey string) *GrowthBookClient {
	return &GrowthBookClient{
		apiKey:    apiKey,
		baseURL:   "https://cdn.growthbook.io",
		cache:     make(map[string]any),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  5 * time.Minute,
	}
}

func (g *GrowthBookClient) GetFeatureValue(key string, defaultValue any) any {
	g.mu.RLock()
	if val, ok := g.cache[key]; ok {
		if cacheTime, exists := g.cacheTime[key]; exists && time.Since(cacheTime) < g.cacheTTL {
			g.mu.RUnlock()
			return val
		}
	}
	g.mu.RUnlock()

	val := g.fetchFeature(key, defaultValue)
	if val != nil {
		g.mu.Lock()
		g.cache[key] = val
		g.cacheTime[key] = time.Now()
		g.mu.Unlock()
	}

	return val
}

func (g *GrowthBookClient) fetchFeature(key string, defaultValue any) any {
	return defaultValue
}

func (g *GrowthBookClient) IsOn(key string) bool {
	val := g.GetFeatureValue(key, false)
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}

func (g *GrowthBookClient) ClearCache() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cache = make(map[string]any)
	g.cacheTime = make(map[string]time.Time)
}

type Metrics struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalTokens     int64
	InputTokens     int64
	OutputTokens    int64
	CachedTokens    int64
	TotalLatencyMs  int64
	mu              sync.Mutex
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) RecordRequest(success bool, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests++
	if success {
		m.SuccessRequests++
	} else {
		m.FailedRequests++
	}
	m.TotalLatencyMs += latencyMs
}

func (m *Metrics) RecordTokens(input, output, cached int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.InputTokens += input
	m.OutputTokens += output
	m.CachedTokens += cached
	m.TotalTokens += input + output
}

func (m *Metrics) GetSnapshot() map[string]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	return map[string]int64{
		"total_requests":   m.TotalRequests,
		"success_requests": m.SuccessRequests,
		"failed_requests":  m.FailedRequests,
		"total_tokens":     m.TotalTokens,
		"input_tokens":     m.InputTokens,
		"output_tokens":    m.OutputTokens,
		"cached_tokens":    m.CachedTokens,
		"total_latency_ms": m.TotalLatencyMs,
	}
}

func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests = 0
	m.SuccessRequests = 0
	m.FailedRequests = 0
	m.TotalTokens = 0
	m.InputTokens = 0
	m.OutputTokens = 0
	m.CachedTokens = 0
	m.TotalLatencyMs = 0
}

func (m *Metrics) GetAverageLatency() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.TotalRequests == 0 {
		return 0
	}
	return float64(m.TotalLatencyMs) / float64(m.TotalRequests)
}

func (m *Metrics) GetSuccessRate() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.TotalRequests == 0 {
		return 0
	}
	return float64(m.SuccessRequests) / float64(m.TotalRequests) * 100
}
