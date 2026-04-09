package upstreamproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type ProxyConfig struct {
	UpstreamURL    string            `json:"upstream_url"`
	Headers        map[string]string `json:"headers,omitempty"`
	Timeout        time.Duration     `json:"timeout"`
	MaxConnections int               `json:"max_connections"`
	RetryCount     int               `json:"retry_count"`
	CircuitBreaker bool              `json:"circuit_breaker"`
}

type UpstreamProxy struct {
	config       ProxyConfig
	reverseProxy *httputil.ReverseProxy
	transport    *http.Transport
	breaker      *CircuitBreaker
	mu           sync.RWMutex
}

func NewUpstreamProxy(config ProxyConfig) (*UpstreamProxy, error) {
	targetURL, err := url.Parse(config.UpstreamURL)
	if err != nil {
		return nil, fmt.Errorf("parse upstream URL: %w", err)
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxConnections == 0 {
		config.MaxConnections = 100
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}

	transport := &http.Transport{
		MaxIdleConns:        config.MaxConnections,
		MaxIdleConnsPerHost: config.MaxConnections / 2,
		IdleConnTimeout:     config.Timeout,
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.Transport = transport

	proxy := &UpstreamProxy{
		config:       config,
		reverseProxy: reverseProxy,
		transport:    transport,
	}

	if config.CircuitBreaker {
		proxy.breaker = NewCircuitBreaker(5, 30*time.Second)
	}

	return proxy, nil
}

func (p *UpstreamProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.breaker != nil && !p.breaker.Allow() {
		http.Error(w, "circuit breaker open", http.StatusServiceUnavailable)
		return
	}

	for key, value := range p.config.Headers {
		r.Header.Set(key, value)
	}

	p.reverseProxy.ServeHTTP(w, r)
}

func (p *UpstreamProxy) Forward(ctx context.Context, req *http.Request) (*http.Response, error) {
	if p.breaker != nil && !p.breaker.Allow() {
		return nil, fmt.Errorf("circuit breaker open")
	}

	targetURL, err := url.Parse(p.config.UpstreamURL)
	if err != nil {
		return nil, fmt.Errorf("parse upstream URL: %w", err)
	}

	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host
	req.Host = targetURL.Host

	for key, value := range p.config.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{
		Transport: p.transport,
		Timeout:   p.config.Timeout,
	}

	var resp *http.Response
	var lastErr error

	for i := 0; i < p.config.RetryCount; i++ {
		resp, lastErr = client.Do(req.WithContext(ctx))
		if lastErr == nil && resp.StatusCode < 500 {
			if p.breaker != nil {
				p.breaker.RecordSuccess()
			}
			return resp, nil
		}

		if lastErr == nil {
			resp.Body.Close()
		}

		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}

	if p.breaker != nil {
		p.breaker.RecordFailure()
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", p.config.RetryCount, lastErr)
}

func (p *UpstreamProxy) HealthCheck(ctx context.Context) error {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", p.config.UpstreamURL+"/health", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

func (p *UpstreamProxy) UpdateConfig(config ProxyConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	proxy, err := NewUpstreamProxy(config)
	if err != nil {
		return err
	}

	p.config = proxy.config
	p.reverseProxy = proxy.reverseProxy
	p.transport = proxy.transport
	p.breaker = proxy.breaker

	return nil
}

func (p *UpstreamProxy) GetConfig() ProxyConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}
