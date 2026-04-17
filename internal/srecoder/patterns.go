package srecoder

import (
	"strings"
)

type SREPattern struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	GoTemplate  string `json:"go_template"`
	Category    string `json:"category"`
}

func GetPatterns() []SREPattern {
	return []SREPattern{
		{
			Name:        "circuit_breaker",
			Description: "Circuit breaker pattern with configurable failure threshold and reset timeout",
			Category:    "circuit-breaker",
			GoTemplate: `// CircuitBreaker prevents cascading failures by stopping calls to a failing service.
type CircuitBreaker struct {
	mu           sync.Mutex
	state        string // "closed", "open", "half-open"
	failures     int
	threshold    int
	resetTimeout time.Duration
	lastFailure  time.Time
}

func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        "closed",
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "closed":
		return true
	case "open":
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = "half-open"
			return true
		}
		return false
	case "half-open":
		return true
	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold {
		cb.state = "open"
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = "closed"
}`,
		},
		{
			Name:        "retry_with_backoff",
			Description: "Retry with exponential backoff and jitter to prevent thundering herd",
			Category:    "retry",
			GoTemplate: `func RetryWithBackoff(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt == maxRetries {
			break
		}

		backoff := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
		delay := backoff + jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}`,
		},
		{
			Name:        "timeout_with_context",
			Description: "Context propagation with explicit timeout for downstream calls",
			Category:    "timeout",
			GoTemplate: `func CallWithTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn(ctx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("operation timed out after %s: %w", timeout, ctx.Err())
	}
}`,
		},
		{
			Name:        "health_check_handler",
			Description: "HTTP health check endpoints for liveness and readiness probes",
			Category:    "healthcheck",
			GoTemplate: `type HealthChecker interface {
	IsAlive() bool
	IsReady() bool
}

func HealthHandler(checker HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checker.IsAlive() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(` + "`" + `{"status":"unhealthy"}` + "`" + `))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(` + "`" + `{"status":"healthy"}` + "`" + `))
	}
}

func ReadyHandler(checker HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !checker.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(` + "`" + `{"status":"not_ready"}` + "`" + `))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(` + "`" + `{"status":"ready"}` + "`" + `))
	}
}`,
		},
		{
			Name:        "prometheus_metrics",
			Description: "Prometheus counter, gauge, and histogram instrumentation",
			Category:    "metrics",
			GoTemplate: `var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_connections",
			Help: "Number of active connections",
		},
		[]string{"service"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal, requestDuration, activeConnections)
}`,
		},
		{
			Name:        "graceful_shutdown",
			Description: "Signal handling with graceful drain for zero-downtime shutdown",
			Category:    "grading",
			GoTemplate: `func GracefulShutdown(server *http.Server, timeout time.Duration) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("received %s, shutting down gracefully...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}

	log.Println("server stopped")
}`,
		},
		{
			Name:        "rate_limiter",
			Description: "Token bucket rate limiter for controlling request throughput",
			Category:    "rate-limiter",
			GoTemplate: `type RateLimiter struct {
	mu       sync.Mutex
	tokens   float64
	maxTokens float64
	rate     float64
	lastTime time.Time
}

func NewRateLimiter(rate float64, burst int) *RateLimiter {
	return &RateLimiter{
		tokens:    float64(burst),
		maxTokens: float64(burst),
		rate:      rate,
		lastTime:  time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.tokens = math.Min(rl.maxTokens, rl.tokens+elapsed*rl.rate)
	rl.lastTime = now

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}`,
		},
		{
			Name:        "bulkhead",
			Description: "Goroutine pool pattern for isolating resources and preventing goroutine leaks",
			Category:    "bulkhead",
			GoTemplate: `type Bulkhead struct {
	sem chan struct{}
}

func NewBulkhead(maxConcurrency int) *Bulkhead {
	return &Bulkhead{
		sem: make(chan struct{}, maxConcurrency),
	}
}

func (b *Bulkhead) Execute(ctx context.Context, fn func() error) error {
	select {
	case b.sem <- struct{}{}:
		defer func() { <-b.sem }()
		return fn()
	case <-ctx.Done():
		return fmt.Errorf("bulkhead full: %w", ctx.Err())
	}
}`,
		},
		{
			Name:        "fallback_degradation",
			Description: "Fallback and graceful degradation pattern for serving degraded responses",
			Category:    "grading",
			GoTemplate: `type FallbackHandler struct {
	primary   func(ctx context.Context) (any, error)
	fallback  func(ctx context.Context) (any, error)
	cb        *CircuitBreaker
}

func NewFallbackHandler(primary, fallback func(ctx context.Context) (any, error), threshold int, resetTimeout time.Duration) *FallbackHandler {
	return &FallbackHandler{
		primary:  primary,
		fallback: fallback,
		cb:       NewCircuitBreaker(threshold, resetTimeout),
	}
}

func (h *FallbackHandler) Execute(ctx context.Context) (any, error) {
	if !h.cb.Allow() {
		return h.fallback(ctx)
	}

	result, err := h.primary(ctx)
	if err != nil {
		h.cb.RecordFailure()
		fallbackResult, fbErr := h.fallback(ctx)
		if fbErr != nil {
			return nil, fmt.Errorf("primary: %w; fallback: %v", err, fbErr)
		}
		return fallbackResult, nil
	}

	h.cb.RecordSuccess()
	return result, nil
}`,
		},
		{
			Name:        "structured_logging",
			Description: "Structured logging with trace ID propagation for distributed tracing",
			Category:    "logging",
			GoTemplate: `type Logger struct {
	logger *slog.Logger
}

func NewLogger(service string) *Logger {
	return &Logger{
		logger: slog.Default().With("service", service),
	}
}

func (l *Logger) WithTrace(traceID string) *Logger {
	return &Logger{
		logger: l.logger.With("trace_id", traceID),
	}
}

func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

func (l *Logger) Error(msg string, err error, args ...any) {
	allArgs := append([]any{"error", err}, args...)
	l.logger.Error(msg, allArgs...)
}

func TraceIDFromContext(ctx context.Context) string {
	if span := trace.SpanFromContext(ctx); span != nil {
		return span.SpanContext().TraceID().String()
	}
	return ""
}`,
		},
	}
}

func GetPatternForCode(code string) []SREPattern {
	var recommended []SREPattern

	hasHTTPOperations := strings.Contains(code, "http.") || strings.Contains(code, "http.Client") ||
		strings.Contains(code, "net/http") || strings.Contains(code, "HandlerFunc")
	hasGoroutines := strings.Contains(code, "go func") || strings.Contains(code, "goroutine")
	hasExternalCalls := strings.Contains(code, "Dial") || strings.Contains(code, "Request") ||
		strings.Contains(code, "sql.") || strings.Contains(code, "redis") ||
		strings.Contains(code, "grpc")
	hasContext := strings.Contains(code, "context.") || strings.Contains(code, "ctx context.Context")
	hasMain := strings.Contains(code, "func main()") || strings.Contains(code, "http.ListenAndServe")
	hasErrors := strings.Contains(code, "error") || strings.Contains(code, "err != nil")

	allPatterns := GetPatterns()
	patternMap := map[string]*SREPattern{}
	for i := range allPatterns {
		patternMap[allPatterns[i].Name] = &allPatterns[i]
	}

	if hasHTTPOperations || hasMain {
		if p, ok := patternMap["health_check_handler"]; ok {
			recommended = append(recommended, *p)
		}
	}

	if hasExternalCalls {
		if p, ok := patternMap["circuit_breaker"]; ok {
			recommended = append(recommended, *p)
		}
		if p, ok := patternMap["retry_with_backoff"]; ok {
			recommended = append(recommended, *p)
		}
	}

	if hasContext {
		if p, ok := patternMap["timeout_with_context"]; ok {
			recommended = append(recommended, *p)
		}
	}

	if hasHTTPOperations {
		if p, ok := patternMap["prometheus_metrics"]; ok {
			recommended = append(recommended, *p)
		}
	}

	if hasMain {
		if p, ok := patternMap["graceful_shutdown"]; ok {
			recommended = append(recommended, *p)
		}
	}

	if hasHTTPOperations || hasExternalCalls {
		if p, ok := patternMap["rate_limiter"]; ok {
			recommended = append(recommended, *p)
		}
	}

	if hasGoroutines {
		if p, ok := patternMap["bulkhead"]; ok {
			recommended = append(recommended, *p)
		}
	}

	if hasExternalCalls && hasErrors {
		if p, ok := patternMap["fallback_degradation"]; ok {
			recommended = append(recommended, *p)
		}
	}

	if hasErrors || hasHTTPOperations {
		if p, ok := patternMap["structured_logging"]; ok {
			recommended = append(recommended, *p)
		}
	}

	return recommended
}

func ApplyPattern(pattern SREPattern, code string) (string, error) {
	insertPoint := findInsertionPoint(code)
	if insertPoint == -1 {
		return code + "\n\n" + pattern.GoTemplate, nil
	}

	lines := strings.Split(code, "\n")
	before := lines[:insertPoint]
	after := lines[insertPoint:]

	result := strings.Join(before, "\n") + "\n\n" + pattern.GoTemplate + "\n\n" + strings.Join(after, "\n")
	return result, nil
}

func findInsertionPoint(code string) int {
	lines := strings.Split(code, "\n")

	lastImportEnd := -1
	inImportBlock := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "import (" {
			inImportBlock = true
			continue
		}
		if inImportBlock && trimmed == ")" {
			inImportBlock = false
			lastImportEnd = i
			continue
		}
		if !inImportBlock && strings.HasPrefix(trimmed, "import ") {
			lastImportEnd = i
		}
	}

	if lastImportEnd != -1 {
		return lastImportEnd + 1
	}

	for i, line := range lines {
		if strings.HasPrefix(line, "package ") {
			return i + 1
		}
	}

	return -1
}
