// Package healthcheck metrics tests
// Tests for Prometheus metrics integration and validation
package healthcheck

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHealthMetrics(t *testing.T) {
	metrics := NewHealthMetrics()

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.checksTotal)
	assert.NotNil(t, metrics.checkErrors)
	assert.NotNil(t, metrics.circuitTrips)
	assert.NotNil(t, metrics.checkDuration)
	assert.NotNil(t, metrics.healthStatus)
	assert.NotNil(t, metrics.dependencyStatus)
	assert.NotNil(t, metrics.circuitBreakerState)
	assert.NotNil(t, metrics.checkDurationSummary)
}

func TestNewHealthMetricsWithConfig_Enabled(t *testing.T) {
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}

	metrics := NewHealthMetricsWithConfig(config)

	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.checksTotal)
}

func TestNewHealthMetricsWithConfig_Disabled(t *testing.T) {
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   false,
	}

	metrics := NewHealthMetricsWithConfig(config)

	assert.NotNil(t, metrics)
	assert.Nil(t, metrics.checksTotal) // Should be nil when disabled
}

func TestDefaultMetricsConfig(t *testing.T) {
	config := DefaultMetricsConfig()

	assert.Equal(t, "alchemorsel", config.Namespace)
	assert.Equal(t, "healthcheck", config.Subsystem)
	assert.True(t, config.Enabled)
}

func TestHealthMetrics_RecordCheck(t *testing.T) {
	// Create a custom registry for testing
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	
	// Register metrics with test registry
	registry.MustRegister(
		metrics.checksTotal,
		metrics.checkDuration,
		metrics.healthStatus,
		metrics.checkDurationSummary,
	)

	// Record a check
	duration := 150 * time.Millisecond
	metrics.RecordCheck(StatusHealthy, duration)

	// Verify counter metric
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.checksTotal.WithLabelValues("overall", "healthy")))
	
	// Verify gauge metric
	assert.Equal(t, float64(2), testutil.ToFloat64(metrics.healthStatus.WithLabelValues("overall"))) // StatusHealthy = 2
	
	// Verify histogram metric has been recorded
	histogramValue := testutil.ToFloat64(metrics.checkDuration.WithLabelValues("overall"))
	assert.Greater(t, histogramValue, float64(0))
}

func TestHealthMetrics_RecordCheck_Disabled(t *testing.T) {
	config := MetricsConfig{
		Enabled: false,
	}
	
	metrics := NewHealthMetricsWithConfig(config)

	// Should not panic when metrics are disabled
	metrics.RecordCheck(StatusHealthy, 100*time.Millisecond)
}

func TestHealthMetrics_RecordCheckByName(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.checksTotal, metrics.healthStatus)

	// Record check for specific checker
	metrics.RecordCheckByName("database", StatusUnhealthy, 200*time.Millisecond)

	// Verify metrics
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.checksTotal.WithLabelValues("database", "unhealthy")))
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.healthStatus.WithLabelValues("database"))) // StatusUnhealthy = 0
}

func TestHealthMetrics_RecordCheckError(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.checkErrors)

	// Record error
	metrics.RecordCheckError("database", "connection_timeout")

	// Verify error metric
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.checkErrors.WithLabelValues("database", "connection_timeout")))
}

func TestHealthMetrics_RecordDependencyStatus(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.dependencyStatus)

	// Record dependency status
	metrics.RecordDependencyStatus("postgres", DependencyTypeDatabase, true, StatusHealthy)

	// Verify dependency status metric
	assert.Equal(t, float64(2), testutil.ToFloat64(metrics.dependencyStatus.WithLabelValues("postgres", "database", "true")))
}

func TestHealthMetrics_RecordCircuitBreakerState(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.circuitBreakerState)

	// Record circuit breaker states
	metrics.RecordCircuitBreakerState("db_circuit", StateClosed)
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.circuitBreakerState.WithLabelValues("db_circuit")))

	metrics.RecordCircuitBreakerState("api_circuit", StateHalfOpen)
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.circuitBreakerState.WithLabelValues("api_circuit")))

	metrics.RecordCircuitBreakerState("cache_circuit", StateOpen)
	assert.Equal(t, float64(2), testutil.ToFloat64(metrics.circuitBreakerState.WithLabelValues("cache_circuit")))
}

func TestHealthMetrics_RecordCircuitTrip(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.circuitTrips)

	// Record circuit trip
	metrics.RecordCircuitTrip("database", "failure_threshold_exceeded")

	// Verify circuit trip metric
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.circuitTrips.WithLabelValues("database", "failure_threshold_exceeded")))
}

func TestHealthMetrics_UpdateHealthStatus(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.healthStatus)

	// Update health status
	metrics.UpdateHealthStatus(StatusDegraded)

	// Verify status update
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.healthStatus.WithLabelValues("overall"))) // StatusDegraded = 1
}

func TestHealthMetrics_UpdateDependencyStatuses(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.dependencyStatus)

	// Update multiple dependency statuses
	statuses := []DependencyStatus{
		{
			Name:     "postgres",
			Type:     DependencyTypeDatabase,
			Status:   StatusHealthy,
			Critical: true,
		},
		{
			Name:     "redis",
			Type:     DependencyTypeCache,
			Status:   StatusDegraded,
			Critical: false,
		},
	}

	metrics.UpdateDependencyStatuses(statuses)

	// Verify both dependency statuses were updated
	assert.Equal(t, float64(2), testutil.ToFloat64(metrics.dependencyStatus.WithLabelValues("postgres", "database", "true")))
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.dependencyStatus.WithLabelValues("redis", "cache", "false")))
}

func TestHealthMetrics_UpdateCircuitBreakerStates(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.circuitBreakerState)

	// Update circuit breaker states
	states := map[string]CircuitBreakerStatus{
		"db_circuit": {
			Name:  "db_circuit",
			State: StateClosed,
		},
		"api_circuit": {
			Name:  "api_circuit",
			State: StateOpen,
		},
	}

	metrics.UpdateCircuitBreakerStates(states)

	// Verify both circuit breaker states were updated
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.circuitBreakerState.WithLabelValues("db_circuit")))
	assert.Equal(t, float64(2), testutil.ToFloat64(metrics.circuitBreakerState.WithLabelValues("api_circuit")))
}

func TestStatusToFloat(t *testing.T) {
	assert.Equal(t, float64(2), statusToFloat(StatusHealthy))
	assert.Equal(t, float64(1), statusToFloat(StatusDegraded))
	assert.Equal(t, float64(0), statusToFloat(StatusUnhealthy))
	assert.Equal(t, float64(-1), statusToFloat(Status("invalid")))
}

func TestCircuitStateToFloat(t *testing.T) {
	assert.Equal(t, float64(0), circuitStateToFloat(StateClosed))
	assert.Equal(t, float64(1), circuitStateToFloat(StateHalfOpen))
	assert.Equal(t, float64(2), circuitStateToFloat(StateOpen))
	assert.Equal(t, float64(-1), circuitStateToFloat(CircuitBreakerState(999)))
}

func TestHealthCheckMetricsMiddleware(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.checksTotal, metrics.checkErrors)

	// Create mock checker
	mockChecker := NewMockChecker("test").WithStatus(StatusHealthy).WithMessage("OK")
	
	// Wrap with metrics middleware
	middleware := NewHealthCheckMetricsMiddleware(metrics, mockChecker)

	// Execute check
	ctx := context.Background()
	result := middleware.Check(ctx)

	// Verify check result
	assert.Equal(t, "test", result.Name)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "OK", result.Message)

	// Verify metrics were recorded
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.checksTotal.WithLabelValues("test", "healthy")))
}

func TestHealthCheckMetricsMiddleware_WithError(t *testing.T) {
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	registry.MustRegister(metrics.checksTotal, metrics.checkErrors)

	// Create failing checker
	mockChecker := NewMockChecker("test").WithStatus(StatusUnhealthy).WithMessage("Failed")
	
	// Wrap with metrics middleware
	middleware := NewHealthCheckMetricsMiddleware(metrics, mockChecker)

	// Execute check
	ctx := context.Background()
	result := middleware.Check(ctx)

	// Verify check result
	assert.Equal(t, StatusUnhealthy, result.Status)

	// Verify metrics were recorded
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.checksTotal.WithLabelValues("test", "unhealthy")))
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.checkErrors.WithLabelValues("test", "health_check_failed")))
}

func TestWithMetrics(t *testing.T) {
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	mockChecker := NewMockChecker("test").WithStatus(StatusHealthy)

	// Test with metrics
	wrappedChecker := WithMetrics(metrics, mockChecker)
	assert.IsType(t, &HealthCheckMetricsMiddleware{}, wrappedChecker)

	// Test with nil metrics
	wrappedChecker = WithMetrics(nil, mockChecker)
	assert.Equal(t, mockChecker, wrappedChecker)
}

func TestHealthMetrics_GetMetricsHandler(t *testing.T) {
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)

	registry := metrics.GetMetricsHandler()

	assert.NotNil(t, registry)
	
	// Gather metrics to verify registry has our metrics
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	// Should have multiple metric families
	assert.Greater(t, len(metricFamilies), 0)

	// Check for expected metric names
	metricNames := make([]string, 0, len(metricFamilies))
	for _, mf := range metricFamilies {
		metricNames = append(metricNames, mf.GetName())
	}

	expectedMetrics := []string{
		"test_health_checks_total",
		"test_health_check_errors_total",
		"test_health_circuit_trips_total",
		"test_health_check_duration_seconds",
		"test_health_status",
		"test_health_dependency_status",
		"test_health_circuit_breaker_state",
		"test_health_check_duration_summary_seconds",
	}

	for _, expected := range expectedMetrics {
		assert.Contains(t, metricNames, expected, "Expected metric %s not found", expected)
	}
}

func TestHealthMetrics_ConcurrentAccess(t *testing.T) {
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)

	// Run concurrent metric recording
	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Record various metrics concurrently
			metrics.RecordCheck(StatusHealthy, time.Millisecond)
			metrics.RecordCheckByName("test", StatusHealthy, time.Millisecond)
			metrics.RecordCheckError("test", "error")
			metrics.RecordDependencyStatus("dep", DependencyTypeDatabase, true, StatusHealthy)
			metrics.RecordCircuitBreakerState("circuit", StateClosed)
			metrics.RecordCircuitTrip("circuit", "reason")
			metrics.UpdateHealthStatus(StatusHealthy)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify metrics were recorded (should not panic or race)
	registry := prometheus.NewRegistry()
	registry.MustRegister(metrics.checksTotal)
	
	// Should have recorded checks
	checkCount := testutil.ToFloat64(metrics.checksTotal.WithLabelValues("overall", "healthy"))
	assert.Equal(t, float64(numGoroutines), checkCount)
}

func TestMetricsCollector(t *testing.T) {
	config := MetricsConfig{
		Namespace: "test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	collector := NewMetricsCollector(nil, metrics)

	assert.NotNil(t, collector)
	assert.Equal(t, metrics, collector.metrics)

	// Test Describe and Collect don't panic
	ch := make(chan *prometheus.Desc, 10)
	collector.Describe(ch)
	close(ch)

	metricsCh := make(chan prometheus.Metric, 10)
	collector.Collect(metricsCh)
	close(metricsCh)
}

func TestHealthMetrics_IntegrationWithPrometheus(t *testing.T) {
	// Create a custom registry to avoid conflicts with other tests
	registry := prometheus.NewRegistry()
	
	config := MetricsConfig{
		Namespace: "integration_test",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)

	// Register metrics
	registry.MustRegister(
		metrics.checksTotal,
		metrics.checkErrors,
		metrics.checkDuration,
		metrics.healthStatus,
		metrics.dependencyStatus,
		metrics.circuitBreakerState,
		metrics.circuitTrips,
		metrics.checkDurationSummary,
	)

	// Record various metrics
	metrics.RecordCheck(StatusHealthy, 100*time.Millisecond)
	metrics.RecordCheck(StatusUnhealthy, 200*time.Millisecond)
	metrics.RecordCheckByName("database", StatusHealthy, 50*time.Millisecond)
	metrics.RecordCheckError("cache", "timeout")
	metrics.RecordDependencyStatus("postgres", DependencyTypeDatabase, true, StatusHealthy)
	metrics.RecordCircuitBreakerState("api_circuit", StateOpen)
	metrics.RecordCircuitTrip("db_circuit", "failure_threshold")

	// Gather all metrics
	metricFamilies, err := registry.Gather()
	require.NoError(t, err)

	// Convert to text format for inspection
	var buffer strings.Builder
	for _, mf := range metricFamilies {
		buffer.WriteString(mf.String())
	}

	output := buffer.String()

	// Verify key metrics are present
	assert.Contains(t, output, "integration_test_health_checks_total")
	assert.Contains(t, output, "integration_test_health_check_errors_total")
	assert.Contains(t, output, "integration_test_health_check_duration_seconds")
	assert.Contains(t, output, "integration_test_health_status")
	assert.Contains(t, output, "integration_test_health_dependency_status")
	assert.Contains(t, output, "integration_test_health_circuit_breaker_state")
	assert.Contains(t, output, "integration_test_health_circuit_trips_total")

	// Verify some metric values
	assert.Contains(t, output, `status="healthy"`)
	assert.Contains(t, output, `status="unhealthy"`)
	assert.Contains(t, output, `dependency_name="postgres"`)
	assert.Contains(t, output, `circuit_name="api_circuit"`)
}

// Benchmark tests
func BenchmarkHealthMetrics_RecordCheck(b *testing.B) {
	config := MetricsConfig{
		Namespace: "bench",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordCheck(StatusHealthy, time.Millisecond)
	}
}

func BenchmarkHealthMetrics_RecordCheckByName(b *testing.B) {
	config := MetricsConfig{
		Namespace: "bench",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordCheckByName("test", StatusHealthy, time.Millisecond)
	}
}

func BenchmarkHealthMetrics_RecordDependencyStatus(b *testing.B) {
	config := MetricsConfig{
		Namespace: "bench",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordDependencyStatus("test", DependencyTypeDatabase, true, StatusHealthy)
	}
}

func BenchmarkHealthCheckMetricsMiddleware_Check(b *testing.B) {
	config := MetricsConfig{
		Namespace: "bench",
		Subsystem: "health",
		Enabled:   true,
	}
	
	metrics := NewHealthMetricsWithConfig(config)
	mockChecker := NewMockChecker("test").WithStatus(StatusHealthy)
	middleware := NewHealthCheckMetricsMiddleware(metrics, mockChecker)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		middleware.Check(ctx)
	}
}