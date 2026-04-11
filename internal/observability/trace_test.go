package observability

import (
	"context"
	"testing"
	"time"
)

type collectExporter struct {
	spans []*Span
}

func (e *collectExporter) ExportSpans(spans []*Span) {
	e.spans = append(e.spans, spans...)
}

func TestStartEndSpan(t *testing.T) {
	exporter := &collectExporter{}
	tracer := NewTracerWithExporter(exporter)
	old := DefaultTracer
	DefaultTracer = tracer
	defer func() { DefaultTracer = old }()

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "test-operation")
	if span == nil {
		t.Fatal("expected non-nil span")
	}

	if span.Name != "test-operation" {
		t.Errorf("expected name 'test-operation', got %q", span.Name)
	}

	if span.StartTime.IsZero() {
		t.Error("expected StartTime to be set")
	}

	EndSpan(span)

	if span.EndTime.IsZero() {
		t.Error("expected EndTime to be set")
	}

	if span.Duration() == 0 {
		t.Error("expected non-zero duration")
	}

	if len(exporter.spans) != 1 {
		t.Fatalf("expected 1 exported span, got %d", len(exporter.spans))
	}
}

func TestSpanWithAttributes(t *testing.T) {
	exporter := &collectExporter{}
	tracer := NewTracerWithExporter(exporter)
	old := DefaultTracer
	DefaultTracer = tracer
	defer func() { DefaultTracer = old }()

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "attr-test",
		Attr("key1", "value1"),
		Attr("key2", 42),
	)
	if span == nil {
		t.Fatal("expected non-nil span")
	}

	if span.Attributes["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", span.Attributes["key1"])
	}
	if span.Attributes["key2"] != 42 {
		t.Errorf("expected key2=42, got %v", span.Attributes["key2"])
	}

	EndSpan(span)
}

func TestSpanFromContext(t *testing.T) {
	old := DefaultTracer
	DefaultTracer = NewTracer()
	defer func() { DefaultTracer = old }()

	ctx := context.Background()

	if s := SpanFromContext(ctx); s != nil {
		t.Error("expected nil span from empty context")
	}

	ctx, span := StartSpan(ctx, "parent")
	if s := SpanFromContext(ctx); s != span {
		t.Error("expected to retrieve the same span from context")
	}

	EndSpan(span)
}

func TestChildSpans(t *testing.T) {
	exporter := &collectExporter{}
	tracer := NewTracerWithExporter(exporter)
	old := DefaultTracer
	DefaultTracer = tracer
	defer func() { DefaultTracer = old }()

	ctx := context.Background()
	ctx, parent := StartSpan(ctx, "parent")
	if parent == nil {
		t.Fatal("expected non-nil parent span")
	}

	_, child := StartSpan(ctx, "child")
	if child == nil {
		t.Fatal("expected non-nil child span")
	}

	if child.Parent != parent {
		t.Error("expected child.Parent == parent")
	}

	if len(parent.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(parent.Children))
	}

	if parent.Children[0] != child {
		t.Error("expected parent.Children[0] == child")
	}

	EndSpan(child)
	EndSpan(parent)

	if len(exporter.spans) != 1 {
		t.Errorf("expected 1 exported root span, got %d", len(exporter.spans))
	}
}

func TestDisabledTracer(t *testing.T) {
	tracer := NewTracer()
	tracer.SetEnabled(false)
	old := DefaultTracer
	DefaultTracer = tracer
	defer func() { DefaultTracer = old }()

	ctx := context.Background()
	ctx, span := StartSpan(ctx, "disabled-test")
	if span != nil {
		t.Error("expected nil span when tracer is disabled")
	}
}

func TestSpanDurationBeforeEnd(t *testing.T) {
	span := &Span{
		Name:      "test",
		StartTime: time.Now(),
	}
	if span.Duration() != 0 {
		t.Error("expected 0 duration for un-ended span")
	}
}

func TestSetAttribute(t *testing.T) {
	span := &Span{
		Name:      "test",
		StartTime: time.Now(),
	}
	span.SetAttribute("foo", "bar")
	if span.Attributes["foo"] != "bar" {
		t.Errorf("expected foo=bar, got %v", span.Attributes["foo"])
	}
}

func TestFormatSpan(t *testing.T) {
	parent := &Span{
		Name:       "parent",
		StartTime:  time.Now(),
		EndTime:    time.Now().Add(100 * time.Millisecond),
		Attributes: map[string]any{"key": "val"},
	}
	child := &Span{
		Name:      "child",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(50 * time.Millisecond),
	}
	parent.Children = append(parent.Children, child)

	output := FormatSpan(parent, "")
	if output == "" {
		t.Error("expected non-empty formatted output")
	}
}

func TestTracerGetCompletedSpans(t *testing.T) {
	tracer := NewTracer()
	tracer.ClearSpans()

	span := &Span{
		Name:      "completed",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(10 * time.Millisecond),
	}
	tracer.spans = append(tracer.spans, span)

	completed := tracer.GetCompletedSpans()
	if len(completed) != 1 {
		t.Errorf("expected 1 completed span, got %d", len(completed))
	}
}

func TestEndSpanNil(t *testing.T) {
	EndSpan(nil)
}

func TestSpanFromContextNil(t *testing.T) {
	if s := SpanFromContext(nil); s != nil {
		t.Error("expected nil for nil context")
	}
}
