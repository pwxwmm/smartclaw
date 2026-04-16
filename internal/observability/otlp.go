package observability

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	otelExporterEnvVar = "OTEL_EXPORTER_OTLP_ENDPOINT"
	serviceName        = "smartclaw"
	tracerName         = "github.com/instructkr/smartclaw"
)

// otlpExporter adapts SmartClaw's SpanExporter to also export via OpenTelemetry.
type otlpExporter struct {
	tracer trace.Tracer
}

// ExportSpans converts SmartClaw spans to OpenTelemetry spans and exports them.
func (e *otlpExporter) ExportSpans(spans []*Span) {
	for _, s := range spans {
		e.exportSpan(s)
	}
}

func (e *otlpExporter) exportSpan(s *Span) {
	if e.tracer == nil {
		return
	}

	ctx := context.Background()
	_, span := e.tracer.Start(ctx, s.Name,
		trace.WithTimestamp(s.StartTime),
	)

	s.mu.Lock()
	for k, v := range s.Attributes {
		span.SetAttributes(toOTELAttr(k, v))
	}
	s.mu.Unlock()

	// Export children as nested spans
	for _, child := range s.Children {
		e.exportChildSpan(span, child)
	}

	span.End(trace.WithTimestamp(s.EndTime))
}

func (e *otlpExporter) exportChildSpan(parent trace.Span, s *Span) {
	ctx := trace.ContextWithSpan(context.Background(), parent)
	_, span := e.tracer.Start(ctx, s.Name,
		trace.WithTimestamp(s.StartTime),
	)

	s.mu.Lock()
	for k, v := range s.Attributes {
		span.SetAttributes(toOTELAttr(k, v))
	}
	s.mu.Unlock()

	for _, child := range s.Children {
		e.exportChildSpan(span, child)
	}

	span.End(trace.WithTimestamp(s.EndTime))
}

func toOTELAttr(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

// InitOTLP initializes the OpenTelemetry OTLP trace exporter.
// It reads the endpoint from OTEL_EXPORTER_OTLP_ENDPOINT.
// If the env var is not set, it returns nil with no error (OTLP is skipped).
// Returns a shutdown function that should be called on application exit.
func InitOTLP() (func(context.Context) error, error) {
	endpoint := os.Getenv(otelExporterEnvVar)
	if endpoint == "" {
		slog.Debug("OTLP endpoint not configured, skipping OTLP initialization")
		return func(ctx context.Context) error { return nil }, nil
	}
	return InitOTLPWithEndpoint(endpoint)
}

// InitOTLPWithEndpoint initializes the OpenTelemetry OTLP trace exporter
// pointing to the given endpoint.
// Returns a shutdown function that should be called on application exit.
func InitOTLPWithEndpoint(endpoint string) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	// Bridge SmartClaw's DefaultTracer to also export via OTLP
	otelExp := &otlpExporter{
		tracer: tp.Tracer(tracerName),
	}
	wireOTLPExporter(otelExp)

	slog.Info("OTLP trace exporter initialized", "endpoint", endpoint)

	shutdown := func(ctx context.Context) error {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown OTLP TracerProvider: %w", err)
		}
		return nil
	}

	return shutdown, nil
}

// wireOTLPExporter wraps the existing DefaultTracer exporter with a multi-exporter
// that exports to both the original exporter and OTLP.
func wireOTLPExporter(otelExp *otlpExporter) {
	DefaultTracer.mu.Lock()
	defer DefaultTracer.mu.Unlock()

	orig := DefaultTracer.exporter
	if orig == nil {
		DefaultTracer.exporter = otelExp
		return
	}

	DefaultTracer.exporter = &multiExporter{
		exporters: []SpanExporter{orig, otelExp},
	}
}

// multiExporter delegates to multiple SpanExporters.
type multiExporter struct {
	exporters []SpanExporter
}

// ExportSpans exports to all underlying exporters.
func (m *multiExporter) ExportSpans(spans []*Span) {
	for _, e := range m.exporters {
		e.ExportSpans(spans)
	}
}
