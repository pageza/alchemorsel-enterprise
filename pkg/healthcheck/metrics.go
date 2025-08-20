// Package healthcheck metrics integration
// Provides Prometheus metrics for health check monitoring
package healthcheck

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HealthMetrics provides Prometheus metrics for health checks
type HealthMetrics struct {
	// Counter metrics
	checksTotal  *prometheus.CounterVec
	checkErrors  *prometheus.CounterVec
	circuitTrips *prometheus.CounterVec

	// Histogram metrics
	checkDuration *prometheus.HistogramVec

	// Gauge metrics
	healthStatus        *prometheus.GaugeVec
	dependencyStatus    *prometheus.GaugeVec
	circuitBreakerState *prometheus.GaugeVec

	// Summary metrics
	checkDurationSummary *prometheus.SummaryVec

	mu sync.RWMutex
}

// MetricsConfig holds configuration for metrics
type MetricsConfig struct {
	Namespace string
	Subsystem string
	Enabled   bool
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Namespace: "alchemorsel",
		Subsystem: "healthcheck",
		Enabled:   true,
	}
}

// NewHealthMetrics creates a new health metrics instance
func NewHealthMetrics() *HealthMetrics {
	return NewHealthMetricsWithConfig(DefaultMetricsConfig())
}

// NewHealthMetricsWithConfig creates a new health metrics instance with configuration
func NewHealthMetricsWithConfig(config MetricsConfig) *HealthMetrics {
	if !config.Enabled {
		return &HealthMetrics{}
	}

	hm := &HealthMetrics{
		checksTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "checks_total",
				Help:      "Total number of health checks performed",
			},
			[]string{"check_name", "status"},
		),

		checkErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "check_errors_total",
				Help:      "Total number of health check errors",
			},
			[]string{"check_name", "error_type"},
		),

		circuitTrips: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "circuit_trips_total",
				Help:      "Total number of circuit breaker trips",
			},
			[]string{"circuit_name", "reason"},
		),

		checkDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "check_duration_seconds",
				Help:      "Duration of health checks in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"check_name"},
		),

		healthStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "status",
				Help:      "Current health status (0=unhealthy, 1=degraded, 2=healthy)",
			},
			[]string{"check_name"},
		),

		dependencyStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "dependency_status",
				Help:      "Current dependency status (0=unhealthy, 1=degraded, 2=healthy)",
			},
			[]string{"dependency_name", "dependency_type", "critical"},
		),

		circuitBreakerState: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "circuit_breaker_state",
				Help:      "Current circuit breaker state (0=closed, 1=half-open, 2=open)",
			},
			[]string{"circuit_name"},
		),

		checkDurationSummary: promauto.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace:  config.Namespace,
				Subsystem:  config.Subsystem,
				Name:       "check_duration_summary_seconds",
				Help:       "Summary of health check durations in seconds",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
			},
			[]string{"check_name"},
		),
	}

	return hm
}

// RecordCheck records a health check execution
func (hm *HealthMetrics) RecordCheck(status Status, duration time.Duration) {
	if hm.checksTotal == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	statusStr := string(status)
	hm.checksTotal.WithLabelValues("overall", statusStr).Inc()
	hm.checkDuration.WithLabelValues("overall").Observe(duration.Seconds())
	hm.checkDurationSummary.WithLabelValues("overall").Observe(duration.Seconds())
	hm.healthStatus.WithLabelValues("overall").Set(statusToFloat(status))
}

// RecordCheckByName records a health check execution for a specific check
func (hm *HealthMetrics) RecordCheckByName(checkName string, status Status, duration time.Duration) {
	if hm.checksTotal == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	statusStr := string(status)
	hm.checksTotal.WithLabelValues(checkName, statusStr).Inc()
	hm.checkDuration.WithLabelValues(checkName).Observe(duration.Seconds())
	hm.checkDurationSummary.WithLabelValues(checkName).Observe(duration.Seconds())
	hm.healthStatus.WithLabelValues(checkName).Set(statusToFloat(status))
}

// RecordCheckError records a health check error
func (hm *HealthMetrics) RecordCheckError(checkName, errorType string) {
	if hm.checkErrors == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.checkErrors.WithLabelValues(checkName, errorType).Inc()
}

// RecordDependencyStatus records dependency status
func (hm *HealthMetrics) RecordDependencyStatus(name string, depType DependencyType, critical bool, status Status) {
	if hm.dependencyStatus == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	criticalStr := strconv.FormatBool(critical)
	hm.dependencyStatus.WithLabelValues(name, string(depType), criticalStr).Set(statusToFloat(status))
}

// RecordCircuitBreakerState records circuit breaker state
func (hm *HealthMetrics) RecordCircuitBreakerState(name string, state CircuitBreakerState) {
	if hm.circuitBreakerState == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.circuitBreakerState.WithLabelValues(name).Set(circuitStateToFloat(state))
}

// RecordCircuitTrip records a circuit breaker trip
func (hm *HealthMetrics) RecordCircuitTrip(name, reason string) {
	if hm.circuitTrips == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.circuitTrips.WithLabelValues(name, reason).Inc()
}

// UpdateHealthStatus updates the overall health status gauge
func (hm *HealthMetrics) UpdateHealthStatus(status Status) {
	if hm.healthStatus == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.healthStatus.WithLabelValues("overall").Set(statusToFloat(status))
}

// UpdateDependencyStatuses updates all dependency status gauges
func (hm *HealthMetrics) UpdateDependencyStatuses(statuses []DependencyStatus) {
	if hm.dependencyStatus == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	for _, status := range statuses {
		criticalStr := strconv.FormatBool(status.Critical)
		hm.dependencyStatus.WithLabelValues(status.Name, string(status.Type), criticalStr).Set(statusToFloat(status.Status))
	}
}

// UpdateCircuitBreakerStates updates all circuit breaker state gauges
func (hm *HealthMetrics) UpdateCircuitBreakerStates(states map[string]CircuitBreakerStatus) {
	if hm.circuitBreakerState == nil {
		return
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	for name, status := range states {
		hm.circuitBreakerState.WithLabelValues(name).Set(circuitStateToFloat(status.State))
	}
}

// statusToFloat converts a Status to a float for Prometheus metrics
func statusToFloat(status Status) float64 {
	switch status {
	case StatusHealthy:
		return 2
	case StatusDegraded:
		return 1
	case StatusUnhealthy:
		return 0
	default:
		return -1
	}
}

// circuitStateToFloat converts a CircuitBreakerState to a float for Prometheus metrics
func circuitStateToFloat(state CircuitBreakerState) float64 {
	switch state {
	case StateClosed:
		return 0
	case StateHalfOpen:
		return 1
	case StateOpen:
		return 2
	default:
		return -1
	}
}

// MetricsCollector implements prometheus.Collector for custom metrics collection
type MetricsCollector struct {
	healthCheck *EnterpriseHealthCheck
	metrics     *HealthMetrics
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(healthCheck *EnterpriseHealthCheck, metrics *HealthMetrics) *MetricsCollector {
	return &MetricsCollector{
		healthCheck: healthCheck,
		metrics:     metrics,
	}
}

// Describe implements prometheus.Collector
func (mc *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	// This would typically describe custom metrics
}

// Collect implements prometheus.Collector
func (mc *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	// This would typically collect custom metrics
	// For now, we rely on the auto-registered metrics
}

// HealthCheckMetricsMiddleware provides middleware for automatic metrics collection
type HealthCheckMetricsMiddleware struct {
	metrics *HealthMetrics
	next    Checker
}

// NewHealthCheckMetricsMiddleware creates middleware for metrics collection
func NewHealthCheckMetricsMiddleware(metrics *HealthMetrics, next Checker) *HealthCheckMetricsMiddleware {
	return &HealthCheckMetricsMiddleware{
		metrics: metrics,
		next:    next,
	}
}

// Check implements Checker interface with metrics collection
func (hm *HealthCheckMetricsMiddleware) Check(ctx context.Context) Check {
	start := time.Now()

	check := hm.next.Check(ctx)
	duration := time.Since(start)

	// Record metrics
	if hm.metrics != nil {
		hm.metrics.RecordCheckByName(check.Name, check.Status, duration)

		if check.Status == StatusUnhealthy {
			hm.metrics.RecordCheckError(check.Name, "health_check_failed")
		}
	}

	return check
}

// WithMetrics wraps a checker with metrics middleware
func WithMetrics(metrics *HealthMetrics, checker Checker) Checker {
	if metrics == nil {
		return checker
	}
	return NewHealthCheckMetricsMiddleware(metrics, checker)
}

// RegisterCustomMetrics allows registration of custom metrics
func (hm *HealthMetrics) RegisterCustomMetrics(registry *prometheus.Registry) error {
	if hm.checksTotal == nil {
		return nil // Metrics disabled
	}

	// Register custom collectors if needed
	collector := NewMetricsCollector(nil, hm)
	return registry.Register(collector)
}

// GetMetricsHandler returns an HTTP handler for Prometheus metrics
func (hm *HealthMetrics) GetMetricsHandler() *prometheus.Registry {
	registry := prometheus.NewRegistry()

	// Register all metrics with the registry
	if hm.checksTotal != nil {
		registry.MustRegister(
			hm.checksTotal,
			hm.checkErrors,
			hm.circuitTrips,
			hm.checkDuration,
			hm.healthStatus,
			hm.dependencyStatus,
			hm.circuitBreakerState,
			hm.checkDurationSummary,
		)
	}

	return registry
}
