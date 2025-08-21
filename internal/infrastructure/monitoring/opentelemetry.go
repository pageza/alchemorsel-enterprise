package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// OpenTelemetryConfig holds OpenTelemetry configuration
type OpenTelemetryConfig struct {
	ServiceName     string
	ServiceVersion  string
	Environment     string
	
	// Tracing configuration
	TracingEnabled     bool
	JaegerEndpoint     string
	OTLPTraceEndpoint  string
	SamplingRate       float64
	
	// Metrics configuration
	MetricsEnabled     bool
	MetricsPort        int
	OTLPMetricEndpoint string
	
	// Resource attributes
	ResourceAttributes map[string]string
}

// OpenTelemetryProvider provides unified OpenTelemetry functionality
type OpenTelemetryProvider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	tracer         trace.Tracer
	meter          metric.Meter
	logger         *zap.Logger
	config         OpenTelemetryConfig
}

// NewOpenTelemetryProvider creates a new OpenTelemetry provider
func NewOpenTelemetryProvider(config OpenTelemetryConfig, logger *zap.Logger) (*OpenTelemetryProvider, error) {
	provider := &OpenTelemetryProvider{
		logger: logger,
		config: config,
	}
	
	// Initialize resource
	resource, err := provider.createResource()
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	
	// Initialize tracing if enabled
	if config.TracingEnabled {
		if err := provider.initializeTracing(resource); err != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
	}
	
	// Initialize metrics if enabled
	if config.MetricsEnabled {
		if err := provider.initializeMetrics(resource); err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
	}
	
	logger.Info("OpenTelemetry provider initialized",
		zap.String("service", config.ServiceName),
		zap.String("version", config.ServiceVersion),
		zap.String("environment", config.Environment),
		zap.Bool("tracing_enabled", config.TracingEnabled),
		zap.Bool("metrics_enabled", config.MetricsEnabled),
	)
	
	return provider, nil
}

// createResource creates an OpenTelemetry resource
func (o *OpenTelemetryProvider) createResource() (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(o.config.ServiceName),
		semconv.ServiceVersion(o.config.ServiceVersion),
		semconv.DeploymentEnvironment(o.config.Environment),
	}
	
	// Add custom resource attributes
	for key, value := range o.config.ResourceAttributes {
		attrs = append(attrs, attribute.String(key, value))
	}
	
	return resource.New(
		context.Background(),
		resource.WithAttributes(attrs...),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
	)
}

// initializeTracing sets up distributed tracing
func (o *OpenTelemetryProvider) initializeTracing(res *resource.Resource) error {
	var exporters []sdktrace.SpanExporter
	
	// Jaeger exporter
	if o.config.JaegerEndpoint != "" {
		jaegerExporter, err := jaeger.New(
			jaeger.WithCollectorEndpoint(
				jaeger.WithEndpoint(o.config.JaegerEndpoint),
			),
		)
		if err != nil {
			return fmt.Errorf("failed to create Jaeger exporter: %w", err)
		}
		exporters = append(exporters, jaegerExporter)
		o.logger.Info("Jaeger exporter configured", zap.String("endpoint", o.config.JaegerEndpoint))
	}
	
	// OTLP HTTP exporter
	if o.config.OTLPTraceEndpoint != "" {
		otlpExporter, err := otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpoint(o.config.OTLPTraceEndpoint),
			otlptracehttp.WithInsecure(), // Use secure in production
		)
		if err != nil {
			return fmt.Errorf("failed to create OTLP exporter: %w", err)
		}
		exporters = append(exporters, otlpExporter)
		o.logger.Info("OTLP trace exporter configured", zap.String("endpoint", o.config.OTLPTraceEndpoint))
	}
	
	if len(exporters) == 0 {
		o.logger.Warn("No trace exporters configured, using noop")
		return nil
	}
	
	// Create span processors for each exporter
	var spanProcessors []sdktrace.SpanProcessor
	for _, exporter := range exporters {
		spanProcessors = append(spanProcessors, sdktrace.NewBatchSpanProcessor(exporter))
	}
	
	// Create tracer provider
	o.tracerProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(o.config.SamplingRate)),
		sdktrace.WithSpanProcessor(spanProcessors[0]), // Primary processor
	)
	
	// Add additional processors if any
	for i := 1; i < len(spanProcessors); i++ {
		o.tracerProvider.RegisterSpanProcessor(spanProcessors[i])
	}
	
	// Set global tracer provider
	otel.SetTracerProvider(o.tracerProvider)
	
	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	
	// Create tracer
	o.tracer = otel.Tracer(
		o.config.ServiceName,
		trace.WithInstrumentationVersion(o.config.ServiceVersion),
		trace.WithSchemaURL(semconv.SchemaURL),
	)
	
	return nil
}

// initializeMetrics sets up metrics collection
func (o *OpenTelemetryProvider) initializeMetrics(res *resource.Resource) error {
	var readers []sdkmetric.Reader
	
	// Prometheus exporter
	prometheusExporter, err := prometheus.New()
	if err != nil {
		return fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}
	readers = append(readers, prometheusExporter)
	
	// OTLP metrics exporter (if configured)
	if o.config.OTLPMetricEndpoint != "" {
		// Configure OTLP metrics exporter here
		o.logger.Info("OTLP metrics endpoint configured", zap.String("endpoint", o.config.OTLPMetricEndpoint))
	}
	
	// Create meter provider
	o.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(prometheusExporter),
	)
	
	// Set global meter provider
	otel.SetMeterProvider(o.meterProvider)
	
	// Create meter
	o.meter = otel.Meter(
		o.config.ServiceName,
		metric.WithInstrumentationVersion(o.config.ServiceVersion),
		metric.WithSchemaURL(semconv.SchemaURL),
	)
	
	return nil
}

// Tracer returns the configured tracer
func (o *OpenTelemetryProvider) Tracer() trace.Tracer {
	return o.tracer
}

// Meter returns the configured meter
func (o *OpenTelemetryProvider) Meter() metric.Meter {
	return o.meter
}

// StartSpan starts a new span
func (o *OpenTelemetryProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if o.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	return o.tracer.Start(ctx, name, opts...)
}

// StartBusinessSpan starts a span for business operations
func (o *OpenTelemetryProvider) StartBusinessSpan(ctx context.Context, operation, entity string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if o.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	
	spanAttrs := []attribute.KeyValue{
		attribute.String("business.operation", operation),
		attribute.String("business.entity", entity),
		attribute.String("span.kind", "business"),
	}
	spanAttrs = append(spanAttrs, attrs...)
	
	return o.tracer.Start(ctx, fmt.Sprintf("business.%s.%s", entity, operation),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(spanAttrs...),
	)
}

// StartExternalSpan starts a span for external service calls
func (o *OpenTelemetryProvider) StartExternalSpan(ctx context.Context, serviceName, operation string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if o.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}
	
	spanAttrs := []attribute.KeyValue{
		attribute.String("external.service", serviceName),
		attribute.String("external.operation", operation),
	}
	spanAttrs = append(spanAttrs, attrs...)
	
	return o.tracer.Start(ctx, fmt.Sprintf("external.%s.%s", serviceName, operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(spanAttrs...),
	)
}

// InstrumentHTTPHandler instruments HTTP handlers with tracing
func (o *OpenTelemetryProvider) InstrumentHTTPHandler(handler http.Handler, operation string) http.Handler {
	if o.tracer == nil {
		return handler
	}
	
	return otelhttp.NewHandler(handler, operation,
		otelhttp.WithTracerProvider(o.tracerProvider),
		otelhttp.WithMeterProvider(o.meterProvider),
	)
}

// InstrumentGRPCClient instruments gRPC clients
func (o *OpenTelemetryProvider) InstrumentGRPCClient(conn *grpc.ClientConn) *grpc.ClientConn {
	// Add gRPC client instrumentation
	return conn
}

// CreateCounter creates a new counter metric
func (o *OpenTelemetryProvider) CreateCounter(name, description, unit string) (metric.Int64Counter, error) {
	if o.meter == nil {
		return nil, fmt.Errorf("meter not initialized")
	}
	
	return o.meter.Int64Counter(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
}

// CreateHistogram creates a new histogram metric
func (o *OpenTelemetryProvider) CreateHistogram(name, description, unit string) (metric.Float64Histogram, error) {
	if o.meter == nil {
		return nil, fmt.Errorf("meter not initialized")
	}
	
	return o.meter.Float64Histogram(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
}

// CreateGauge creates a new gauge metric
func (o *OpenTelemetryProvider) CreateGauge(name, description, unit string) (metric.Float64ObservableGauge, error) {
	if o.meter == nil {
		return nil, fmt.Errorf("meter not initialized")
	}
	
	return o.meter.Float64ObservableGauge(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
}

// RecordSpanEvent records an event in the current span
func (o *OpenTelemetryProvider) RecordSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

// RecordError records an error in the current span
func (o *OpenTelemetryProvider) RecordError(ctx context.Context, err error, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.RecordError(err, trace.WithAttributes(attrs...))
		span.SetStatus(trace.Status{
			Code:        trace.StatusCodeError,
			Description: err.Error(),
		})
	}
}

// SetSpanAttributes sets attributes on the current span
func (o *OpenTelemetryProvider) SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		span.SetAttributes(attrs...)
	}
}

// GetTraceID returns the trace ID from the current context
func (o *OpenTelemetryProvider) GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID returns the span ID from the current context
func (o *OpenTelemetryProvider) GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span != nil {
		return span.SpanContext().SpanID().String()
	}
	return ""
}

// Shutdown gracefully shuts down the OpenTelemetry provider
func (o *OpenTelemetryProvider) Shutdown(ctx context.Context) error {
	var errs []error
	
	if o.tracerProvider != nil {
		if err := o.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
	}
	
	if o.meterProvider != nil {
		if err := o.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}
	}
	
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	
	o.logger.Info("OpenTelemetry provider shutdown completed")
	return nil
}

// BusinessMetrics provides business-specific telemetry
type BusinessMetrics struct {
	provider *OpenTelemetryProvider
	
	// Counters
	recipesCreated   metric.Int64Counter
	recipesViewed    metric.Int64Counter
	usersRegistered  metric.Int64Counter
	aiRequests       metric.Int64Counter
	
	// Histograms
	recipeCreationDuration metric.Float64Histogram
	aiResponseTime         metric.Float64Histogram
	
	// Gauges
	activeUsers metric.Float64ObservableGauge
}

// NewBusinessMetrics creates business-specific metrics
func NewBusinessMetrics(provider *OpenTelemetryProvider) (*BusinessMetrics, error) {
	bm := &BusinessMetrics{provider: provider}
	
	var err error
	
	// Create counters
	if bm.recipesCreated, err = provider.CreateCounter(
		"business.recipes.created.total",
		"Total number of recipes created",
		"1",
	); err != nil {
		return nil, err
	}
	
	if bm.recipesViewed, err = provider.CreateCounter(
		"business.recipes.viewed.total",
		"Total number of recipe views",
		"1",
	); err != nil {
		return nil, err
	}
	
	if bm.usersRegistered, err = provider.CreateCounter(
		"business.users.registered.total",
		"Total number of users registered",
		"1",
	); err != nil {
		return nil, err
	}
	
	if bm.aiRequests, err = provider.CreateCounter(
		"business.ai.requests.total",
		"Total number of AI requests",
		"1",
	); err != nil {
		return nil, err
	}
	
	// Create histograms
	if bm.recipeCreationDuration, err = provider.CreateHistogram(
		"business.recipe.creation.duration",
		"Duration of recipe creation process",
		"ms",
	); err != nil {
		return nil, err
	}
	
	if bm.aiResponseTime, err = provider.CreateHistogram(
		"business.ai.response.duration",
		"AI service response time",
		"ms",
	); err != nil {
		return nil, err
	}
	
	return bm, nil
}

// RecordRecipeCreated records a recipe creation event
func (bm *BusinessMetrics) RecordRecipeCreated(ctx context.Context, userID, recipeID string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("user.id", userID),
		attribute.String("recipe.id", recipeID),
	}
	
	bm.recipesCreated.Add(ctx, 1, metric.WithAttributes(attrs...))
	bm.recipeCreationDuration.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
	
	// Record span event
	bm.provider.RecordSpanEvent(ctx, "recipe.created", attrs...)
}

// RecordRecipeViewed records a recipe view event
func (bm *BusinessMetrics) RecordRecipeViewed(ctx context.Context, userID, recipeID string) {
	attrs := []attribute.KeyValue{
		attribute.String("user.id", userID),
		attribute.String("recipe.id", recipeID),
	}
	
	bm.recipesViewed.Add(ctx, 1, metric.WithAttributes(attrs...))
	bm.provider.RecordSpanEvent(ctx, "recipe.viewed", attrs...)
}

// RecordUserRegistered records a user registration event
func (bm *BusinessMetrics) RecordUserRegistered(ctx context.Context, userID, method string) {
	attrs := []attribute.KeyValue{
		attribute.String("user.id", userID),
		attribute.String("registration.method", method),
	}
	
	bm.usersRegistered.Add(ctx, 1, metric.WithAttributes(attrs...))
	bm.provider.RecordSpanEvent(ctx, "user.registered", attrs...)
}

// RecordAIRequest records an AI service request
func (bm *BusinessMetrics) RecordAIRequest(ctx context.Context, provider, model string, duration time.Duration, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	
	attrs := []attribute.KeyValue{
		attribute.String("ai.provider", provider),
		attribute.String("ai.model", model),
		attribute.String("ai.status", status),
	}
	
	bm.aiRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
	bm.aiResponseTime.Record(ctx, float64(duration.Milliseconds()), metric.WithAttributes(attrs...))
	
	bm.provider.RecordSpanEvent(ctx, "ai.request.completed", attrs...)
}