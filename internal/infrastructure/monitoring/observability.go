package monitoring

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ObservabilityConfig holds configuration for all observability components
type ObservabilityConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	
	// Metrics configuration
	MetricsEnabled bool
	MetricsPort    int
	
	// Tracing configuration
	TracingConfig TracingConfig
	
	// Logging configuration
	LoggingConfig LogConfig
}

// ObservabilityProvider provides unified access to all observability components
type ObservabilityProvider struct {
	Metrics *MetricsCollector
	Tracing *TracingProvider
	Logger  *Logger
	config  ObservabilityConfig
}

// NewObservabilityProvider creates a new observability provider with all components
func NewObservabilityProvider(config ObservabilityConfig) (*ObservabilityProvider, error) {
	// Initialize logger first
	logger, err := NewLogger(config.LoggingConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize metrics collector
	var metrics *MetricsCollector
	if config.MetricsEnabled {
		metrics = NewMetricsCollector(logger.Logger)
		logger.Info("Metrics collection enabled", zap.Int("port", config.MetricsPort))
	}

	// Initialize tracing provider
	tracing, err := NewTracingProvider(config.TracingConfig, logger.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	logger.Info("Observability provider initialized",
		zap.String("service", config.ServiceName),
		zap.String("version", config.ServiceVersion),
		zap.String("environment", config.Environment),
		zap.Bool("metrics_enabled", config.MetricsEnabled),
		zap.Bool("tracing_enabled", config.TracingConfig.Enabled),
	)

	return &ObservabilityProvider{
		Metrics: metrics,
		Tracing: tracing,
		Logger:  logger,
		config:  config,
	}, nil
}

// StartUptimeTracking starts background goroutines for uptime tracking
func (o *ObservabilityProvider) StartUptimeTracking(ctx context.Context) {
	if o.Metrics != nil {
		go o.Metrics.StartUptimeCounter(ctx)
		o.Logger.Info("Uptime tracking started")
	}
}

// Shutdown gracefully shuts down all observability components
func (o *ObservabilityProvider) Shutdown(ctx context.Context) error {
	o.Logger.Info("Shutting down observability provider")

	if o.Tracing != nil {
		if err := o.Tracing.Shutdown(ctx); err != nil {
			o.Logger.Error("Failed to shutdown tracing provider", zap.Error(err))
			return err
		}
	}

	if o.Logger != nil {
		o.Logger.Sync()
	}

	return nil
}

// HealthCheck represents a health check result
type HealthCheck struct {
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
}

// HealthChecker interface for implementing health checks
type HealthChecker interface {
	Check(ctx context.Context) HealthCheck
}

// HealthCheckManager manages application health checks
type HealthCheckManager struct {
	checks   map[string]HealthChecker
	logger   *Logger
	tracing  *TracingProvider
	metrics  *MetricsCollector
}

// NewHealthCheckManager creates a new health check manager
func NewHealthCheckManager(logger *Logger, tracing *TracingProvider, metrics *MetricsCollector) *HealthCheckManager {
	return &HealthCheckManager{
		checks:  make(map[string]HealthChecker),
		logger:  logger,
		tracing: tracing,
		metrics: metrics,
	}
}

// RegisterCheck registers a health check
func (h *HealthCheckManager) RegisterCheck(name string, checker HealthChecker) {
	h.checks[name] = checker
	h.logger.Info("Health check registered", zap.String("name", name))
}

// CheckAll runs all registered health checks
func (h *HealthCheckManager) CheckAll(ctx context.Context) map[string]HealthCheck {
	ctx, span := h.tracing.StartSpan(ctx, "health.check_all")
	defer span.End()

	results := make(map[string]HealthCheck)
	
	for name, checker := range h.checks {
		checkCtx, checkSpan := h.tracing.StartSpan(ctx, fmt.Sprintf("health.check.%s", name))
		
		start := time.Now()
		result := checker.Check(checkCtx)
		duration := time.Since(start)
		
		result.Name = name
		result.Timestamp = time.Now()
		result.Duration = duration
		
		results[name] = result
		
		// Log result
		logger := h.logger.WithContext(checkCtx)
		if result.Status == "healthy" {
			logger.Debug("Health check passed",
				zap.String("check", name),
				zap.Duration("duration", duration),
			)
		} else {
			logger.Warn("Health check failed",
				zap.String("check", name),
				zap.String("status", result.Status),
				zap.String("message", result.Message),
				zap.Duration("duration", duration),
			)
		}
		
		// Record metrics
		if h.metrics != nil {
			status := "success"
			if result.Status != "healthy" {
				status = "failure"
			}
			// This would require adding health check metrics to MetricsCollector
		}
		
		checkSpan.End()
	}
	
	return results
}

// Check runs a specific health check
func (h *HealthCheckManager) Check(ctx context.Context, name string) (HealthCheck, error) {
	checker, exists := h.checks[name]
	if !exists {
		return HealthCheck{}, fmt.Errorf("health check '%s' not found", name)
	}
	
	ctx, span := h.tracing.StartSpan(ctx, fmt.Sprintf("health.check.%s", name))
	defer span.End()
	
	start := time.Now()
	result := checker.Check(ctx)
	duration := time.Since(start)
	
	result.Name = name
	result.Timestamp = time.Now()
	result.Duration = duration
	
	return result, nil
}

// DatabaseHealthChecker implements health check for database
type DatabaseHealthChecker struct {
	db interface {
		Ping() error
	}
}

// NewDatabaseHealthChecker creates a new database health checker
func NewDatabaseHealthChecker(db interface{ Ping() error }) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{db: db}
}

// Check implements HealthChecker interface
func (d *DatabaseHealthChecker) Check(ctx context.Context) HealthCheck {
	err := d.db.Ping()
	if err != nil {
		return HealthCheck{
			Status:  "unhealthy",
			Message: "Database connection failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}
	
	return HealthCheck{
		Status:  "healthy",
		Message: "Database connection successful",
	}
}

// RedisHealthChecker implements health check for Redis
type RedisHealthChecker struct {
	redis interface {
		Ping(ctx context.Context) error
	}
}

// NewRedisHealthChecker creates a new Redis health checker
func NewRedisHealthChecker(redis interface{ Ping(ctx context.Context) error }) *RedisHealthChecker {
	return &RedisHealthChecker{redis: redis}
}

// Check implements HealthChecker interface
func (r *RedisHealthChecker) Check(ctx context.Context) HealthCheck {
	err := r.redis.Ping(ctx)
	if err != nil {
		return HealthCheck{
			Status:  "unhealthy",
			Message: "Redis connection failed",
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}
	
	return HealthCheck{
		Status:  "healthy",
		Message: "Redis connection successful",
	}
}

// ExternalServiceHealthChecker implements health check for external services
type ExternalServiceHealthChecker struct {
	name string
	url  string
	httpClient interface {
		Get(url string) error
	}
}

// NewExternalServiceHealthChecker creates a new external service health checker
func NewExternalServiceHealthChecker(name, url string, client interface{ Get(url string) error }) *ExternalServiceHealthChecker {
	return &ExternalServiceHealthChecker{
		name:       name,
		url:        url,
		httpClient: client,
	}
}

// Check implements HealthChecker interface
func (e *ExternalServiceHealthChecker) Check(ctx context.Context) HealthCheck {
	err := e.httpClient.Get(e.url)
	if err != nil {
		return HealthCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("%s service unavailable", e.name),
			Details: map[string]interface{}{
				"url":   e.url,
				"error": err.Error(),
			},
		}
	}
	
	return HealthCheck{
		Status:  "healthy",
		Message: fmt.Sprintf("%s service available", e.name),
		Details: map[string]interface{}{
			"url": e.url,
		},
	}
}