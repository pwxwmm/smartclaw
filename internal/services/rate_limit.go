package services

import (
	"sync"
	"time"
)

type RateLimit struct {
	Limit     int
	Remaining int
	ResetAt   time.Time
}

type RateLimitService struct {
	limits map[string]*RateLimit
	mu     sync.RWMutex
}

func NewRateLimitService() *RateLimitService {
	return &RateLimitService{
		limits: make(map[string]*RateLimit),
	}
}

func (s *RateLimitService) SetLimit(key string, limit, remaining int, resetAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.limits[key] = &RateLimit{
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}
}

func (s *RateLimitService) GetLimit(key string) *RateLimit {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.limits[key]
}

func (s *RateLimitService) IsLimited(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit, exists := s.limits[key]
	if !exists {
		return false
	}

	if limit.Remaining <= 0 && time.Now().Before(limit.ResetAt) {
		return true
	}

	return false
}

func (s *RateLimitService) Decrement(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit, exists := s.limits[key]; exists && limit.Remaining > 0 {
		limit.Remaining--
	}
}

func (s *RateLimitService) GetWaitTime(key string) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit, exists := s.limits[key]
	if !exists {
		return 0
	}

	if limit.Remaining > 0 {
		return 0
	}

	waitTime := time.Until(limit.ResetAt)
	if waitTime < 0 {
		return 0
	}

	return waitTime
}

type RateLimitMessage struct {
	Type    string
	Message string
}

func (s *RateLimitService) GetRateLimitMessage(key string) *RateLimitMessage {
	if !s.IsLimited(key) {
		return nil
	}

	limit := s.GetLimit(key)
	return &RateLimitMessage{
		Type:    "rate_limited",
		Message: "Rate limit exceeded. Please wait " + time.Until(limit.ResetAt).Round(time.Second).String(),
	}
}

type MockRateLimitService struct {
	enabled bool
	limits  map[string]*RateLimit
	mu      sync.RWMutex
}

func NewMockRateLimitService() *MockRateLimitService {
	return &MockRateLimitService{
		enabled: false,
		limits:  make(map[string]*RateLimit),
	}
}

func (s *MockRateLimitService) Enable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = true
}

func (s *MockRateLimitService) Disable() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = false
}

func (s *MockRateLimitService) SetMockLimit(key string, remaining int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.limits[key] = &RateLimit{
		Limit:     100,
		Remaining: remaining,
		ResetAt:   time.Now().Add(time.Hour),
	}
}

func (s *MockRateLimitService) IsLimited(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enabled {
		return false
	}

	limit, exists := s.limits[key]
	if !exists {
		return false
	}

	return limit.Remaining <= 0
}
