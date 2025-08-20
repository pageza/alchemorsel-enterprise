// Package healthcheck edge cases and error handling tests
// Tests for comprehensive error handling and edge case validation
package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestEdgeCase_NilContext tests behavior with nil context
func TestEdgeCase_NilContext(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	// This should not panic, but use background context internally
	response := hc.Check(nil)
	assert.Equal(t, StatusHealthy, response.Status)
}

// TestEdgeCase_CancelledContext tests behavior with cancelled context
func TestEdgeCase_CancelledContext(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewSlowChecker("slow", 1*time.Second)
	hc.Register("slow", checker)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	response := hc.Check(ctx)
	
	// Should handle cancelled context gracefully
	assert.Len(t, response.Checks, 1)
	check := response.Checks[0]
	assert.Equal(t, "slow", check.Name)
	// Status could be unhealthy due to cancellation
}

// TestEdgeCase_EmptyVersion tests health check with empty version
func TestEdgeCase_EmptyVersion(t *testing.T) {
	hc := New("", zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	ctx := context.Background()
	response := hc.Check(ctx)

	assert.Equal(t, "", response.Version)
	assert.Equal(t, StatusHealthy, response.Status)
}

// TestEdgeCase_NilLogger tests health check with nil logger
func TestEdgeCase_NilLogger(t *testing.T) {
	hc := New("1.0.0", nil)
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	ctx := context.Background()
	
	// Should not panic with nil logger
	response := hc.Check(ctx)
	assert.Equal(t, StatusHealthy, response.Status)
}

// TestEdgeCase_DuplicateCheckerRegistration tests registering the same checker multiple times
func TestEdgeCase_DuplicateCheckerRegistration(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker1 := NewMockChecker("first").WithStatus(StatusHealthy).WithMessage("First")
	checker2 := NewMockChecker("second").WithStatus(StatusDegraded).WithMessage("Second")

	hc.Register("duplicate", checker1)
	hc.Register("duplicate", checker2) // Should overwrite

	ctx := context.Background()
	response := hc.Check(ctx)

	assert.Len(t, response.Checks, 1)
	check := response.Checks[0]
	assert.Equal(t, "duplicate", check.Name)
	assert.Equal(t, StatusDegraded, check.Status) // Should be the second checker
	assert.Equal(t, "Second", check.Message)
}

// TestEdgeCase_VeryLongCheckerName tests checker with very long name
func TestEdgeCase_VeryLongCheckerName(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	
	longName := string(make([]rune, 1000))
	for i := range longName {
		longName = longName[:i] + "a" + longName[i+1:]
	}
	
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register(longName, checker)

	ctx := context.Background()
	response := hc.Check(ctx)

	assert.Len(t, response.Checks, 1)
	assert.Equal(t, longName, response.Checks[0].Name)
}

// TestEdgeCase_CheckerPanic tests handling of panics in checkers
func TestEdgeCase_CheckerPanic(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	
	panicChecker := &PanicChecker{name: "panic_checker"}
	hc.Register("panic", panicChecker)

	ctx := context.Background()
	
	// Should not crash the entire health check
	response := hc.Check(ctx)
	
	// The panicking checker should be handled gracefully
	assert.Len(t, response.Checks, 1)
	// Overall status might be unhealthy due to panic
}

// TestEdgeCase_ZeroCacheTTL tests health check with zero cache TTL
func TestEdgeCase_ZeroCacheTTL(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	hc.SetCacheTTL(0) // No caching
	
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	ctx := context.Background()

	// All calls should execute the checker
	response1 := hc.Check(ctx)
	response2 := hc.Check(ctx)

	assert.NotEqual(t, response1.Timestamp, response2.Timestamp)
	assert.Equal(t, 2, checker.GetCallCount())
}

// TestEdgeCase_NegativeCacheTTL tests health check with negative cache TTL
func TestEdgeCase_NegativeCacheTTL(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	hc.SetCacheTTL(-1 * time.Second) // Negative TTL
	
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	ctx := context.Background()
	
	// Should handle negative TTL gracefully (probably no caching)
	response1 := hc.Check(ctx)
	response2 := hc.Check(ctx)

	// Behavior may vary, but should not crash
	assert.Equal(t, StatusHealthy, response1.Status)
	assert.Equal(t, StatusHealthy, response2.Status)
}

// TestEdgeCase_VeryLargeCacheTTL tests health check with very large cache TTL
func TestEdgeCase_VeryLargeCacheTTL(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	hc.SetCacheTTL(100 * 365 * 24 * time.Hour) // 100 years
	
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	ctx := context.Background()

	response1 := hc.Check(ctx)
	response2 := hc.Check(ctx)

	// Should use cache for very long time
	assert.Equal(t, response1.Timestamp, response2.Timestamp)
	assert.Equal(t, 1, checker.GetCallCount())
}

// TestEdgeCase_CircuitBreakerInvalidConfig tests circuit breaker with invalid configuration
func TestEdgeCase_CircuitBreakerInvalidConfig(t *testing.T) {
	config := CircuitBreakerConfig{
		FailureThreshold: -1,  // Invalid
		SuccessThreshold: -1,  // Invalid
		Timeout:          -1,  // Invalid
		MaxRequests:      -1,  // Invalid
	}

	cb := NewCircuitBreaker("test", config)

	// Should have set defaults for invalid values
	assert.Greater(t, cb.config.FailureThreshold, 0)
	assert.Greater(t, cb.config.SuccessThreshold, 0)
	assert.Greater(t, cb.config.Timeout, time.Duration(0))
	assert.Greater(t, cb.config.MaxRequests, 0)
}

// TestEdgeCase_CircuitBreakerZeroConfig tests circuit breaker with zero configuration
func TestEdgeCase_CircuitBreakerZeroConfig(t *testing.T) {
	config := CircuitBreakerConfig{} // All zeros

	cb := NewCircuitBreaker("test", config)

	// Should have set defaults for zero values
	assert.Greater(t, cb.config.FailureThreshold, 0)
	assert.Greater(t, cb.config.SuccessThreshold, 0)
	assert.Greater(t, cb.config.Timeout, time.Duration(0))
	assert.Greater(t, cb.config.MaxRequests, 0)
}

// TestEdgeCase_DependencyManagerNilLogger tests dependency manager with nil logger
func TestEdgeCase_DependencyManagerNilLogger(t *testing.T) {
	dm := NewDependencyManager(nil)
	
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	dep := CreateTestDependency("test_dep", DependencyTypeDatabase, true, []string{}, checker)

	// Should not panic with nil logger
	dm.Register(dep)
	
	ctx := context.Background()
	results := dm.CheckAll(ctx)
	
	assert.Len(t, results, 1)
	assert.Equal(t, "test_dep", results[0].Name)
}

// TestEdgeCase_DependencyWithEmptyName tests dependency with empty name
func TestEdgeCase_DependencyWithEmptyName(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	checker := NewMockChecker("").WithStatus(StatusHealthy)
	dep := CreateTestDependency("", DependencyTypeDatabase, true, []string{}, checker)

	dm.Register(dep)
	
	ctx := context.Background()
	results := dm.CheckAll(ctx)
	
	assert.Len(t, results, 1)
	assert.Equal(t, "", results[0].Name)
}

// TestEdgeCase_DependencyWithNilChecker tests dependency with nil checker
func TestEdgeCase_DependencyWithNilChecker(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	dep := CreateTestDependency("test_dep", DependencyTypeDatabase, true, []string{}, nil)

	// Should handle nil checker gracefully
	dm.Register(dep)
	
	ctx := context.Background()
	
	// This might panic or handle gracefully depending on implementation
	// The test verifies the behavior
	require.NotPanics(t, func() {
		dm.CheckAll(ctx)
	})
}

// TestEdgeCase_SelfReferencingDependency tests dependency that references itself
func TestEdgeCase_SelfReferencingDependency(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	checker := NewMockChecker("self").WithStatus(StatusHealthy)
	dep := CreateTestDependency("self_ref", DependencyTypeService, false, []string{"self_ref"}, checker)

	dm.Register(dep)
	
	// Should detect self-reference as circular dependency
	err := dm.ValidateGraph()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

// TestEdgeCase_DependencyOnNonExistentDependency tests dependency on non-existent dependency
func TestEdgeCase_DependencyOnNonExistentDependency(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	dep := CreateTestDependency("test_dep", DependencyTypeService, false, []string{"nonexistent"}, checker)

	dm.Register(dep)
	
	ctx := context.Background()
	results := dm.CheckAll(ctx)
	
	// Should handle missing dependency gracefully
	assert.Len(t, results, 1)
	assert.Equal(t, "test_dep", results[0].Name)
	// Status should remain healthy since missing dependency doesn't affect health
}

// TestEdgeCase_MetricsWithNilValues tests metrics with nil or invalid values
func TestEdgeCase_MetricsWithNilValues(t *testing.T) {
	config := TestMetricsConfig()
	metrics := NewHealthMetricsWithConfig(config)

	// Should handle invalid status gracefully
	metrics.RecordCheck(Status("invalid"), time.Millisecond)
	
	// Should handle negative duration
	metrics.RecordCheck(StatusHealthy, -time.Millisecond)
	
	// Should handle very large duration
	metrics.RecordCheck(StatusHealthy, 24*time.Hour)
	
	// Should handle empty strings
	metrics.RecordCheckByName("", StatusHealthy, time.Millisecond)
	metrics.RecordCheckError("", "")
	metrics.RecordDependencyStatus("", DependencyType("invalid"), false, Status("invalid"))
	metrics.RecordCircuitBreakerState("", CircuitBreakerState(999))
	metrics.RecordCircuitTrip("", "")
}

// TestEdgeCase_EnterpriseHealthCheckMaintenanceTransitions tests maintenance mode transitions
func TestEdgeCase_EnterpriseHealthCheckMaintenanceTransitions(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Hour)

	// Multiple maintenance mode transitions
	for i := 0; i < 10; i++ {
		ehc.SetMaintenanceMode(true, "Maintenance", &startTime, &endTime)
		assert.True(t, ehc.IsMaintenanceMode())
		
		ehc.SetMaintenanceMode(false, "", nil, nil)
		assert.False(t, ehc.IsMaintenanceMode())
	}
}

// TestEdgeCase_EnterpriseHealthCheckWithNilTimes tests maintenance mode with nil times
func TestEdgeCase_EnterpriseHealthCheckWithNilTimes(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())

	// Should handle nil times gracefully
	ehc.SetMaintenanceMode(true, "Maintenance", nil, nil)
	assert.True(t, ehc.IsMaintenanceMode())
}

// TestEdgeCase_MockCheckerEdgeCases tests mock checker edge cases
func TestEdgeCase_MockCheckerEdgeCases(t *testing.T) {
	ctx := context.Background()

	// Test mock checker with very long delay
	checker := NewMockChecker("slow").WithDelay(10 * time.Second)
	
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	
	start := time.Now()
	result := checker.Check(ctxWithTimeout)
	duration := time.Since(start)
	
	// Should respect context timeout
	assert.Less(t, duration, 1*time.Second)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "Context cancelled")
}

// TestEdgeCase_HealthCheckJSONMarshaling tests JSON marshaling edge cases
func TestEdgeCase_HealthCheckJSONMarshaling(t *testing.T) {
	// Test with extreme values
	check := Check{
		Name:        "test",
		Status:      Status("invalid_status"),
		Message:     string(make([]rune, 10000)), // Very long message
		LastChecked: time.Time{},                // Zero time
		Duration:    -time.Second,               // Negative duration
		Metadata: map[string]interface{}{
			"nil_value":    nil,
			"complex_data": map[string]interface{}{"nested": []int{1, 2, 3}},
		},
	}

	// Should not panic during marshaling
	data, err := check.MarshalJSON()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TestEdgeCase_ResponseJSONMarshaling tests response JSON marshaling edge cases
func TestEdgeCase_ResponseJSONMarshaling(t *testing.T) {
	response := Response{
		Status:        Status("custom_status"),
		Version:       "",
		Timestamp:     time.Time{}, // Zero time
		TotalDuration: -time.Second, // Negative duration
		Checks:        []Check{},    // Empty checks
	}

	// Should not panic during marshaling
	data, err := response.MarshalJSON()
	assert.NoError(t, err)
	assert.NotEmpty(t, data)
}

// TestEdgeCase_ConcurrentModification tests concurrent modification scenarios
func TestEdgeCase_ConcurrentModification(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	
	// Start health checks in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				hc.Check(context.Background())
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	// Concurrently register and unregister checkers
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		hc.Register(name, checker)
		
		if i%2 == 0 {
			// Remove previous checker
			prevName := fmt.Sprintf("checker_%d", i-1)
			if i > 0 {
				hc.Register(prevName, nil) // Effectively removes by overwriting with nil
			}
		}
	}

	// Final check should work
	response := hc.Check(context.Background())
	assert.NotNil(t, response)
}

// TestEdgeCase_ResourceExhaustion tests behavior under resource constraints
func TestEdgeCase_ResourceExhaustion(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	
	// Register many checkers to test resource usage
	numCheckers := 1000
	for i := 0; i < numCheckers; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		hc.Register(name, checker)
	}

	ctx := context.Background()
	
	// Should handle large number of checkers
	start := time.Now()
	response := hc.Check(ctx)
	duration := time.Since(start)
	
	assert.Equal(t, StatusHealthy, response.Status)
	assert.Len(t, response.Checks, numCheckers)
	
	// Should complete in reasonable time despite many checkers
	assert.Less(t, duration, 5*time.Second)
}

// PanicChecker is a test checker that panics
type PanicChecker struct {
	name string
}

func (p *PanicChecker) Check(ctx context.Context) Check {
	panic("test panic in checker")
}

// TestEdgeCase_SystemInfoEdgeCases tests system info edge cases
func TestEdgeCase_SystemInfoEdgeCases(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	response := ehc.CheckWithMode(ctx, ModeStandard)

	// System info should have reasonable defaults even if system calls fail
	systemInfo := response.SystemInfo
	assert.NotEmpty(t, systemInfo.Hostname)
	assert.NotEmpty(t, systemInfo.Platform)
	assert.NotEmpty(t, systemInfo.Architecture)
	assert.Greater(t, systemInfo.CPUCores, 0)
	assert.Greater(t, systemInfo.Memory.Total, uint64(0))
	assert.NotNil(t, systemInfo.Environment)
}

// TestEdgeCase_CircuitBreakerConcurrentStateChanges tests concurrent state changes
func TestEdgeCase_CircuitBreakerConcurrentStateChanges(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Run concurrent operations
	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			if id%4 == 0 {
				cb.Reset()
			} else if id%4 == 1 {
				cb.ForceOpen()
			} else if id%4 == 2 {
				cb.ForceClose()
			} else {
				cb.Execute(func() (interface{}, error) {
					return "ok", nil
				})
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Should not panic and should be in a valid state
	status := cb.GetStatus()
	assert.Contains(t, []CircuitBreakerState{StateClosed, StateHalfOpen, StateOpen}, status.State)
}