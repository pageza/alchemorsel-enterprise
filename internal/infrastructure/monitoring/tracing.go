package monitoring

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TracingConfig holds tracing configuration
type TracingConfig struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	JaegerEndpoint  string
	SamplingRate    float64
	Enabled         bool
}

// TracingProvider wraps OpenTelemetry tracing functionality
type TracingProvider struct {
	tracer   trace.Tracer
	provider *sdktrace.TracerProvider
	logger   *zap.Logger
	config   TracingConfig
}

// NewTracingProvider creates a new tracing provider
func NewTracingProvider(config TracingConfig, logger *zap.Logger) (*TracingProvider, error) {
	if !config.Enabled {
		logger.Info("Tracing is disabled")
		return &TracingProvider{
			logger: logger,
			config: config,
		}, nil
	}

	// Create Jaeger exporter
	exporter, err := jaeger.New(
		jaeger.WithCollectorEndpoint(
			jaeger.WithEndpoint(config.JaegerEndpoint),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SamplingRate)),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := tp.Tracer(config.ServiceName)

	logger.Info("Tracing initialized",
		zap.String("service", config.ServiceName),
		zap.String("version", config.ServiceVersion),
		zap.String("environment", config.Environment),
		zap.String("jaeger_endpoint", config.JaegerEndpoint),
		zap.Float64("sampling_rate", config.SamplingRate),
	)

	return &TracingProvider{
		tracer:   tracer,
		provider: tp,
		logger:   logger,
		config:   config,
	}, nil
}

// StartSpan starts a new span with the given name and options
func (t *TracingProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return t.tracer.Start(ctx, name, opts...)
}

// StartHTTPSpan starts a span for HTTP requests
func (t *TracingProvider) StartHTTPSpan(ctx context.Context, method, path string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("%s %s", method, path),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", path),
		),
	)
	return ctx, span
}

// StartDBSpan starts a span for database operations
func (t *TracingProvider) StartDBSpan(ctx context.Context, operation, table string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("db.%s.%s", operation, table),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.operation", operation),
			attribute.String("db.table", table),
			attribute.String("db.system", "postgresql"),
		),
	)
	return ctx, span
}

// StartCacheSpan starts a span for cache operations
func (t *TracingProvider) StartCacheSpan(ctx context.Context, operation, key string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("cache.%s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("cache.operation", operation),
			attribute.String("cache.key", key),
			attribute.String("cache.system", "redis"),
		),
	)
	return ctx, span
}

// StartAISpan starts a span for AI service calls
func (t *TracingProvider) StartAISpan(ctx context.Context, provider, model, operation string) (context.Context, trace.Span) {
	if t.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	ctx, span := t.tracer.Start(ctx, fmt.Sprintf("ai.%s.%s", provider, operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("ai.provider", provider),
			attribute.String("ai.model", model),
			attribute.String("ai.operation", operation),
		),
	)
	return ctx, span
}

// AddSpanAttributes adds attributes to the current span
func (t *TracingProvider) AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetAttributes(attrs...)
	}
}

// AddSpanEvent adds an event to the current span
func (t *TracingProvider) AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// RecordError records an error in the current span
func (t *TracingProvider) RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.RecordError(err)
		span.SetStatus(trace.Status{
			Code:        trace.StatusCodeError,
			Description: err.Error(),
		})
	}
}

// SetSpanStatus sets the status of the current span
func (t *TracingProvider) SetSpanStatus(ctx context.Context, code trace.StatusCode, description string) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetStatus(trace.Status{
			Code:        code,
			Description: description,
		})
	}
}

// GetTraceID returns the trace ID from the context
func (t *TracingProvider) GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID returns the span ID from the context
func (t *TracingProvider) GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// Shutdown gracefully shuts down the tracing provider
func (t *TracingProvider) Shutdown(ctx context.Context) error {
	if t.provider != nil {
		return t.provider.Shutdown(ctx)
	}
	return nil
}

// TraceIDFromContext extracts trace ID from context for logging correlation
func TraceIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanIDFromContext extracts span ID from context for logging correlation
func SpanIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().SpanID().String()
	}
	return ""
}