package resilience

import (
	"net/http"
)

type ResilientTransport struct {
	transport   http.RoundTripper
	breaker     *CircuitBreaker
	rateLimiter *RateLimiter
}

func NewResilientTransport(
	transport http.RoundTripper,
	breaker *CircuitBreaker,
	rateLimiter *RateLimiter,
) *ResilientTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &ResilientTransport{
		transport:   transport,
		breaker:     breaker,
		rateLimiter: rateLimiter,
	}
}

func (t *ResilientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.breaker.Allow(); err != nil {
		return nil, err
	}

	if err := t.rateLimiter.Wait(req.Context()); err != nil {
		return nil, err
	}

	resp, err := t.transport.RoundTrip(req)
	if err != nil {
		t.breaker.RecordFailure()
		return nil, err
	}

	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		t.breaker.RecordFailure()
	} else {
		t.breaker.RecordSuccess()
	}

	return resp, nil
}
