package httpclient

import (
	"net/http"
	"time"
)

const DefaultTimeout = 30 * time.Second

// NewClient creates an HTTP client with the given timeout.
// If timeout is 0, DefaultTimeout (30s) is used.
func NewClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	return &http.Client{
		Timeout: timeout,
	}
}

// DefaultClient returns an HTTP client with the default timeout.
func DefaultClient() *http.Client {
	return NewClient(DefaultTimeout)
}
