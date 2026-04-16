package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

func NewAPIError(errType, message string) *APIError {
	return &APIError{
		Type:    errType,
		Message: message,
	}
}

func NewAPIErrorWithCode(errType, message string, code int) *APIError {
	return &APIError{
		Type:    errType,
		Message: message,
		Code:    code,
	}
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ErrorHandler struct {
	logger *log.Logger
}

func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		logger: log.New(os.Stderr, "[error] ", log.LstdFlags),
	}
}

func (h *ErrorHandler) HandleError(err error) {
	h.logger.Printf("%v", err)
}

func (h *ErrorHandler) IsRetryable(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Code == 429 || apiErr.Code >= 500
	}
	return false
}

type RetryConfig struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:   3,
		InitialDelay: time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

func (c *RetryConfig) CalculateDelay(attempt int) time.Duration {
	delay := c.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * c.Multiplier)
		if delay > c.MaxDelay {
			delay = c.MaxDelay
		}
	}
	jitter := time.Duration(rand.Int63n(int64(delay / 2)))
	return delay - delay/4 + jitter
}

type RetryableRequest struct {
	URL         string
	Method      string
	Headers     map[string]string
	Body        []byte
	RetryConfig *RetryConfig
}

func (r *RetryableRequest) Execute(ctx context.Context, client *http.Client) (*http.Response, error) {
	var lastErr error

	// Save body bytes once so each retry can reset the request body.
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes = r.Body
	}

	for attempt := 0; attempt <= r.RetryConfig.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, nil)
		if err != nil {
			return nil, err
		}

		for k, v := range r.Headers {
			req.Header.Set(k, v)
		}

		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(bodyBytes)), nil
			}
			req.ContentLength = int64(len(bodyBytes))
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < r.RetryConfig.MaxRetries {
				delay := r.RetryConfig.CalculateDelay(attempt)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			return nil, err
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = NewAPIErrorWithCode("rate_limit", "rate limited", resp.StatusCode)
			if attempt < r.RetryConfig.MaxRetries {
				delay := r.RetryConfig.CalculateDelay(attempt)
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			return nil, lastErr
		}

		return resp, nil
	}

	return nil, lastErr
}

type LoggingConfig struct {
	LogFile    string
	LogLevel   string
	MaxSizeMB  int
	MaxBackups int
	Compress   bool
}

func (c *LoggingConfig) Validate() error {
	if c.LogLevel != "debug" && c.LogLevel != "info" && c.LogLevel != "warn" && c.LogLevel != "error" {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}
	return nil
}

type APILogger struct {
	logger   *log.Logger
	logFile  *os.File
	logLevel string
}

func NewAPILogger(config *LoggingConfig) (*APILogger, error) {
	logger := log.New(os.Stderr, "[api] ", log.LstdFlags)

	if config.LogFile != "" {
		logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		logger.SetOutput(io.MultiWriter(os.Stderr, logFile))
	}

	return &APILogger{
		logger:   logger,
		logFile:  nil,
		logLevel: config.LogLevel,
	}, nil
}

func (l *APILogger) Debug(msg string) {
	if l.logLevel == "debug" {
		l.logger.Printf("[DEBUG] %s", msg)
	}
}

func (l *APILogger) Info(msg string) {
	l.logger.Printf("[INFO] %s", msg)
}

func (l *APILogger) Warn(msg string) {
	l.logger.Printf("[WARN] %s", msg)
}

func (l *APILogger) Error(msg string) {
	l.logger.Printf("[ERROR] %s", msg)
}

func (l *APILogger) LogRequest(req *http.Request) {
	l.Debug(fmt.Sprintf("Request: %s %s", req.Method, req.URL.String()))
}

func (l *APILogger) LogResponse(resp *http.Response) {
	l.Debug(fmt.Sprintf("Response: %d %s", resp.StatusCode, resp.Status))
}

func (l *APILogger) Close() error {
	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

type BootstrapConfig struct {
	APIEndpoint string
	Timeout     time.Duration
	RetryConfig *RetryConfig
}

func DefaultBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{
		APIEndpoint: "https://api.anthropic.com",
		Timeout:     30 * time.Second,
		RetryConfig: DefaultRetryConfig(),
	}
}

type BootstrapState struct {
	Initialized   bool
	LastCheckedAt time.Time
	Version       string
}

func (s *BootstrapState) IsHealthy() bool {
	return s.Initialized && time.Since(s.LastCheckedAt) < 5*time.Minute
}

type BootstrapService struct {
	config *BootstrapConfig
	state  *BootstrapState
	logger *APILogger
}

func NewBootstrapService(config *BootstrapConfig, logger *APILogger) *BootstrapService {
	return &BootstrapService{
		config: config,
		state: &BootstrapState{
			Initialized: false,
		},
		logger: logger,
	}
}

func (s *BootstrapService) Initialize(ctx context.Context) error {
	s.logger.Info("Initializing bootstrap service...")

	s.state.Initialized = true
	s.state.LastCheckedAt = time.Now()
	s.state.Version = "1.0.0"

	s.logger.Info("Bootstrap service initialized")
	return nil
}

func (s *BootstrapService) CheckHealth(ctx context.Context) error {
	s.state.LastCheckedAt = time.Now()

	if !s.state.Initialized {
		return NewAPIError("not_initialized", "bootstrap service not initialized")
	}

	return nil
}

func (s *BootstrapService) GetState() *BootstrapState {
	return s.state
}

type PromptCacheBreaks struct {
	breakTokens map[string]time.Time
	mu          sync.RWMutex
}

func NewPromptCacheBreaks() *PromptCacheBreaks {
	return &PromptCacheBreaks{
		breakTokens: make(map[string]time.Time),
	}
}

func (p *PromptCacheBreaks) MarkBreaks(identifiers []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for _, id := range identifiers {
		p.breakTokens[id] = now
	}
}

func (p *PromptCacheBreaks) ShouldBreakCache(identifier string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if breakTime, exists := p.breakTokens[identifier]; exists {
		return time.Since(breakTime) < 24*time.Hour
	}
	return false
}

func (p *PromptCacheBreaks) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.breakTokens = make(map[string]time.Time)
}
