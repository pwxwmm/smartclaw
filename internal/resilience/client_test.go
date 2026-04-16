package resilience

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestResilientTransport_SuccessfulRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	cb := NewCircuitBreaker("test", 5, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(http.DefaultTransport, cb, rl)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if cb.GetState() != StateClosed {
		t.Errorf("circuit breaker state = %v, want Closed after success", cb.GetState())
	}
}

func TestResilientTransport_429Passthrough(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cb := NewCircuitBreaker("test", 5, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(http.DefaultTransport, cb, rl)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("StatusCode = %d, want %d (429 should pass through)", resp.StatusCode, http.StatusTooManyRequests)
	}
}

func TestResilientTransport_5xxPassthrough(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cb := NewCircuitBreaker("test", 5, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(http.DefaultTransport, cb, rl)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := transport.RoundTrip(req)

	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d (5xx should pass through)", resp.StatusCode, http.StatusInternalServerError)
	}
}

func TestResilientTransport_5xxRecordsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cb := NewCircuitBreaker("test", 3, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(http.DefaultTransport, cb, rl)

	for i := 0; i < 3; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip %d error: %v", i, err)
		}
		resp.Body.Close()
	}

	if cb.GetState() != StateOpen {
		t.Errorf("circuit breaker state = %v, want Open after 3 5xx responses", cb.GetState())
	}
}

func TestResilientTransport_429RecordsFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	cb := NewCircuitBreaker("test", 3, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(http.DefaultTransport, cb, rl)

	for i := 0; i < 3; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip %d error: %v", i, err)
		}
		resp.Body.Close()
	}

	if cb.GetState() != StateOpen {
		t.Errorf("circuit breaker state = %v, want Open after 3 429 responses", cb.GetState())
	}
}

func TestResilientTransport_CircuitBreakerOpen(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)

	cb.RecordFailure()
	cb.RecordFailure()

	transport := NewResilientTransport(http.DefaultTransport, cb, rl)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost", nil)

	_, err := transport.RoundTrip(req)
	if err == nil {
		t.Error("RoundTrip should fail when circuit breaker is open")
	}
}

func TestResilientTransport_RateLimiterCalled(t *testing.T) {
	var rateLimiterCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewCircuitBreaker("test", 5, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(http.DefaultTransport, cb, rl)

	for i := 0; i < 5; i++ {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatalf("RoundTrip %d error: %v", i, err)
		}
		resp.Body.Close()
		rateLimiterCalls.Add(1)
	}

	if got := rateLimiterCalls.Load(); got != 5 {
		t.Errorf("requests made = %d, want 5", got)
	}
}

func TestResilientTransport_NilTransportUsesDefault(t *testing.T) {
	cb := NewCircuitBreaker("test", 5, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(nil, cb, rl)

	if transport.transport != http.DefaultTransport {
		t.Error("nil transport should default to http.DefaultTransport")
	}
}

func TestResilientTransport_ConnectionErrorRecordsFailure(t *testing.T) {
	cb := NewCircuitBreaker("test", 2, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(http.DefaultTransport, cb, rl)

	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:1", nil)
		transport.RoundTrip(req)
		cancel()
	}

	if cb.GetState() != StateOpen {
		t.Errorf("circuit breaker state = %v, want Open after connection errors", cb.GetState())
	}
}

func TestResilientTransport_2xxRecordsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cb := NewCircuitBreaker("test", 5, 30*time.Second)
	rl := NewRateLimiter(50, time.Second)
	transport := NewResilientTransport(http.DefaultTransport, cb, rl)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	resp.Body.Close()

	if cb.GetState() != StateClosed {
		t.Errorf("circuit breaker state = %v, want Closed after 2xx", cb.GetState())
	}
}
