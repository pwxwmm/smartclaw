package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Attribute is a key-value pair attached to a span.
type Attribute struct {
	Key   string
	Value interface{}
}

// Attr creates an Attribute from a key and value.
func Attr(key string, value interface{}) Attribute {
	return Attribute{Key: key, Value: value}
}

// Span represents a unit of work in a trace.
type Span struct {
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Attributes map[string]interface{}
	Children   []*Span
	Parent     *Span
	mu         sync.Mutex
}

// SetAttribute sets an attribute on the span.
func (s *Span) SetAttribute(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Attributes == nil {
		s.Attributes = make(map[string]interface{})
	}
	s.Attributes[key] = value
}

// Duration returns the span duration. Returns 0 if the span hasn't ended.
func (s *Span) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return 0
	}
	return s.EndTime.Sub(s.StartTime)
}

// AddChild adds a child span.
func (s *Span) AddChild(child *Span) {
	s.mu.Lock()
	defer s.mu.Unlock()
	child.Parent = s
	s.Children = append(s.Children, child)
}

// SpanExporter exports completed spans.
type SpanExporter interface {
	ExportSpans(spans []*Span)
}

// StdoutExporter logs completed spans via slog.
type StdoutExporter struct{}

// ExportSpans logs spans using slog.
func (e *StdoutExporter) ExportSpans(spans []*Span) {
	for _, s := range spans {
		attrs := make([]any, 0, len(s.Attributes)*2+2)
		attrs = append(attrs, "name", s.Name, "duration", s.Duration())
		for k, v := range s.Attributes {
			attrs = append(attrs, k, v)
		}
		slog.Debug("trace: span completed", attrs...)
	}
}

type contextKey struct{}

var spanContextKey = contextKey{}

// Tracer is the global tracing system.
type Tracer struct {
	exporter SpanExporter
	spans    []*Span
	mu       sync.Mutex
	enabled  bool
}

// DefaultTracer is the global default tracer instance.
var DefaultTracer = NewTracer()

// NewTracer creates a new Tracer with a StdoutExporter.
func NewTracer() *Tracer {
	return &Tracer{
		exporter: &StdoutExporter{},
		spans:    make([]*Span, 0),
		enabled:  true,
	}
}

// NewTracerWithExporter creates a new Tracer with a custom exporter.
func NewTracerWithExporter(exporter SpanExporter) *Tracer {
	return &Tracer{
		exporter: exporter,
		spans:    make([]*Span, 0),
		enabled:  true,
	}
}

// SetEnabled enables or disables tracing.
func (t *Tracer) SetEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.enabled = enabled
}

// IsEnabled returns whether tracing is enabled.
func (t *Tracer) IsEnabled() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.enabled
}

// StartSpan starts a new span. If there is a span in the context, the new span
// is added as a child. The span is stored in the returned context.
func StartSpan(ctx context.Context, name string, attrs ...Attribute) (context.Context, *Span) {
	if !DefaultTracer.IsEnabled() {
		return ctx, nil
	}

	span := &Span{
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}, len(attrs)),
	}

	for _, a := range attrs {
		span.Attributes[a.Key] = a.Value
	}

	if parent := SpanFromContext(ctx); parent != nil {
		parent.AddChild(span)
	}

	DefaultTracer.mu.Lock()
	DefaultTracer.spans = append(DefaultTracer.spans, span)
	DefaultTracer.mu.Unlock()

	return context.WithValue(ctx, spanContextKey, span), span
}

// EndSpan ends a span and exports it if it has no parent (root span).
func EndSpan(span *Span) {
	if span == nil {
		return
	}

	span.mu.Lock()
	span.EndTime = time.Now()
	span.mu.Unlock()

	if span.Parent == nil {
		DefaultTracer.mu.Lock()
		exporter := DefaultTracer.exporter
		DefaultTracer.mu.Unlock()

		if exporter != nil {
			exporter.ExportSpans([]*Span{span})
		}
	}
}

// SpanFromContext retrieves the current span from context.
func SpanFromContext(ctx context.Context) *Span {
	if ctx == nil {
		return nil
	}
	span, _ := ctx.Value(spanContextKey).(*Span)
	return span
}

// GetCompletedSpans returns all completed root spans.
func (t *Tracer) GetCompletedSpans() []*Span {
	t.mu.Lock()
	defer t.mu.Unlock()

	var completed []*Span
	for _, s := range t.spans {
		if s.Parent == nil && !s.EndTime.IsZero() {
			completed = append(completed, s)
		}
	}
	return completed
}

// ClearSpans removes all stored spans.
func (t *Tracer) ClearSpans() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spans = t.spans[:0]
}

// FormatSpan formats a span and its children as a hierarchical string.
func FormatSpan(span *Span, indent string) string {
	if span == nil {
		return ""
	}

	result := fmt.Sprintf("%s[%s] %s", indent, span.Name, span.Duration())
	if len(span.Attributes) > 0 {
		result += fmt.Sprintf(" attrs=%v", span.Attributes)
	}
	result += "\n"

	for _, child := range span.Children {
		result += FormatSpan(child, indent+"  ")
	}

	return result
}
