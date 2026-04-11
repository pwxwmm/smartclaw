package services

import (
	"context"
	"sync"
	"time"
)

type Event struct {
	Name      string
	Timestamp time.Time
	Metadata  map[string]any
}

type AnalyticsService struct {
	enabled bool
	events  []Event
	mu      sync.Mutex
	sink    EventSink
}

type EventSink interface {
	Flush(events []Event) error
}

func NewAnalyticsService(sink EventSink) *AnalyticsService {
	return &AnalyticsService{
		enabled: true,
		events:  make([]Event, 0),
		sink:    sink,
	}
}

func (s *AnalyticsService) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

func (s *AnalyticsService) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

func (s *AnalyticsService) LogEvent(name string, metadata map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return
	}

	event := Event{
		Name:      name,
		Timestamp: time.Now(),
		Metadata:  metadata,
	}

	s.events = append(s.events, event)
}

func (s *AnalyticsService) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.events) == 0 {
		return nil
	}

	if s.sink == nil {
		s.events = s.events[:0]
		return nil
	}

	if err := s.sink.Flush(s.events); err != nil {
		return err
	}

	s.events = s.events[:0]
	return nil
}

func (s *AnalyticsService) GetEvents() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Event, len(s.events))
	copy(result, s.events)
	return result
}

func (s *AnalyticsService) ClearEvents() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = s.events[:0]
}

type Config struct {
	Enabled      bool
	OptOut       bool
	SinkEndpoint string
	BatchSize    int
	FlushTimeout time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		Enabled:      true,
		OptOut:       false,
		BatchSize:    100,
		FlushTimeout: 30 * time.Second,
	}
}

type GrowthBookClient struct {
	features map[string]any
}

func NewGrowthBookClient() *GrowthBookClient {
	return &GrowthBookClient{
		features: make(map[string]any),
	}
}

func (c *GrowthBookClient) GetFeatureValue(key string, defaultValue any) any {
	if val, ok := c.features[key]; ok {
		return val
	}
	return defaultValue
}

func (c *GrowthBookClient) SetFeature(key string, value any) {
	c.features[key] = value
}
