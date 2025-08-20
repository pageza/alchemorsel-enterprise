// Package healthcheck performance tests
// Tests to ensure health checks complete within timeout and performance requirements
package healthcheck

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Performance test constants
const (
	MaxHealthCheckDuration      = 1 * time.Second    // Max time for a single health check
	MaxEnterpriseCheckDuration  = 2 * time.Second    // Max time for enterprise check with dependencies
	MaxConcurrentCheckDuration  = 3 * time.Second    // Max time for concurrent health checks
	TargetThroughput           = 100                 // Minimum checks per second
	MaxMemoryAllocationMB      = 10                  // Max memory allocation in MB
)

// TestPerformance_SingleHealthCheck tests performance of a single health check
func TestPerformance_SingleHealthCheck(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("fast").WithStatus(StatusHealthy).WithDuration(10 * time.Millisecond)
	hc.Register("fast", checker)

	ctx := context.Background()

	// Warm up
	hc.Check(ctx)

	start := time.Now()
	response := hc.Check(ctx)
	duration := time.Since(start)

	assert.Equal(t, StatusHealthy, response.Status)
	assert.Less(t, duration, MaxHealthCheckDuration, 
		"Single health check took %v, should be less than %v", duration, MaxHealthCheckDuration)

	t.Logf("Single health check completed in %v", duration)
}

// TestPerformance_MultipleHealthChecks tests performance with multiple checkers
func TestPerformance_MultipleHealthChecks(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	
	// Register multiple fast checkers
	numCheckers := 10
	for i := 0; i < numCheckers; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy).WithDuration(10 * time.Millisecond)
		hc.Register(name, checker)
	}

	ctx := context.Background()

	// Warm up
	hc.Check(ctx)

	start := time.Now()
	response := hc.Check(ctx)
	duration := time.Since(start)

	assert.Equal(t, StatusHealthy, response.Status)
	assert.Len(t, response.Checks, numCheckers)
	
	// Should complete in roughly the time of the slowest check (concurrent execution)
	assert.Less(t, duration, 100*time.Millisecond, 
		"Multiple health checks took %v, should complete concurrently", duration)

	t.Logf("Multiple health checks (%d) completed in %v", numCheckers, duration)
}

// TestPerformance_EnterpriseHealthCheck tests enterprise health check performance
func TestPerformance_EnterpriseHealthCheck(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	
	// Register basic checkers
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy).WithDuration(10 * time.Millisecond)
		ehc.Register(name, checker)
	}

	// Register dependencies
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("dep_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy).WithDuration(10 * time.Millisecond)
		dep := CreateTestDependency(name, DependencyTypeService, false, []string{}, checker)
		ehc.RegisterDependency(dep)
	}

	ctx := context.Background()

	// Warm up
	ehc.CheckWithMode(ctx, ModeDeep)

	start := time.Now()
	response := ehc.CheckWithMode(ctx, ModeDeep)
	duration := time.Since(start)

	AssertEnterpriseResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
	assert.Less(t, duration, MaxEnterpriseCheckDuration,
		"Enterprise health check took %v, should be less than %v", duration, MaxEnterpriseCheckDuration)

	t.Logf("Enterprise health check (deep mode) completed in %v", duration)
}

// TestPerformance_HealthCheckThroughput tests health check throughput
func TestPerformance_HealthCheckThroughput(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("fast").WithStatus(StatusHealthy).WithDuration(1 * time.Millisecond)
	hc.Register("fast", checker)

	ctx := context.Background()
	
	// Test duration
	testDuration := 1 * time.Second
	
	// Warm up
	hc.Check(ctx)

	start := time.Now()
	checkCount := 0

	for time.Since(start) < testDuration {
		response := hc.Check(ctx)
		assert.Equal(t, StatusHealthy, response.Status)
		checkCount++
	}

	elapsed := time.Since(start)
	throughput := float64(checkCount) / elapsed.Seconds()

	assert.GreaterOrEqual(t, throughput, float64(TargetThroughput),
		"Throughput was %.2f checks/second, should be at least %d", throughput, TargetThroughput)

	t.Logf("Performed %d health checks in %v (%.2f checks/second)", checkCount, elapsed, throughput)
}

// TestPerformance_ConcurrentHealthChecks tests concurrent health check performance
func TestPerformance_ConcurrentHealthChecks(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("concurrent").WithStatus(StatusHealthy).WithDuration(10 * time.Millisecond)
	hc.Register("concurrent", checker)

	ctx := context.Background()
	
	numGoroutines := 50
	numChecksPerGoroutine := 10
	
	// Warm up
	hc.Check(ctx)

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numChecksPerGoroutine; j++ {
				response := hc.Check(ctx)
				assert.Equal(t, StatusHealthy, response.Status)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalChecks := numGoroutines * numChecksPerGoroutine
	assert.Less(t, duration, MaxConcurrentCheckDuration,
		"Concurrent health checks took %v, should be less than %v", duration, MaxConcurrentCheckDuration)

	throughput := float64(totalChecks) / duration.Seconds()
	t.Logf("Concurrent health checks: %d total in %v (%.2f checks/second)", 
		totalChecks, duration, throughput)
}

// TestPerformance_CircuitBreakerOverhead tests circuit breaker performance overhead
func TestPerformance_CircuitBreakerOverhead(t *testing.T) {
	// Test without circuit breaker
	checker := NewMockChecker("test").WithStatus(StatusHealthy).WithDuration(1 * time.Millisecond)
	
	ctx := context.Background()
	iterations := 1000

	// Measure baseline performance
	start := time.Now()
	for i := 0; i < iterations; i++ {
		checker.Check(ctx)
	}
	baselineDuration := time.Since(start)

	// Test with circuit breaker
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)
	
	cbChecker := &CircuitBreakerChecker{
		checker: checker,
		breaker: cb,
		name:    "test",
	}

	start = time.Now()
	for i := 0; i < iterations; i++ {
		cbChecker.Check(ctx)
	}
	cbDuration := time.Since(start)

	overhead := cbDuration - baselineDuration
	overheadPercentage := float64(overhead) / float64(baselineDuration) * 100

	// Circuit breaker should add minimal overhead (less than 50%)
	assert.Less(t, overheadPercentage, 50.0,
		"Circuit breaker overhead is %.2f%%, should be less than 50%%", overheadPercentage)

	t.Logf("Circuit breaker overhead: %v (%.2f%%) for %d operations", 
		overhead, overheadPercentage, iterations)
}

// TestPerformance_DependencyGraphTraversal tests dependency graph performance
func TestPerformance_DependencyGraphTraversal(t *testing.T) {
	dm := NewDependencyManager(zap.NewNop())
	
	// Create complex dependency graph
	numDependencies := 50
	
	for i := 0; i < numDependencies; i++ {
		name := fmt.Sprintf("dep_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy).WithDuration(1 * time.Millisecond)
		
		// Create some dependencies to previous nodes
		var deps []string
		if i > 0 {
			// Each node depends on 1-3 previous nodes
			numDeps := min(3, i)
			for j := 0; j < numDeps; j++ {
				deps = append(deps, fmt.Sprintf("dep_%d", i-j-1))
			}
		}
		
		dep := CreateTestDependency(name, DependencyTypeService, false, deps, checker)
		dm.Register(dep)
	}

	ctx := context.Background()

	// Warm up
	dm.CheckAll(ctx)

	start := time.Now()
	results := dm.CheckAll(ctx)
	duration := time.Since(start)

	assert.Len(t, results, numDependencies)
	
	// Should complete in reasonable time even with complex graph
	maxExpectedDuration := time.Duration(numDependencies) * 10 * time.Millisecond
	assert.Less(t, duration, maxExpectedDuration,
		"Dependency graph traversal took %v, should be less than %v", duration, maxExpectedDuration)

	t.Logf("Dependency graph traversal (%d dependencies) completed in %v", numDependencies, duration)
}

// TestPerformance_MetricsOverhead tests metrics collection performance overhead
func TestPerformance_MetricsOverhead(t *testing.T) {
	checker := NewMockChecker("test").WithStatus(StatusHealthy).WithDuration(1 * time.Millisecond)
	
	ctx := context.Background()
	iterations := 1000

	// Measure baseline performance without metrics
	start := time.Now()
	for i := 0; i < iterations; i++ {
		checker.Check(ctx)
	}
	baselineDuration := time.Since(start)

	// Test with metrics enabled
	config := TestMetricsConfig()
	metrics := NewHealthMetricsWithConfig(config)
	wrappedChecker := WithMetrics(metrics, checker)

	start = time.Now()
	for i := 0; i < iterations; i++ {
		wrappedChecker.Check(ctx)
	}
	metricsDuration := time.Since(start)

	overhead := metricsDuration - baselineDuration
	overheadPercentage := float64(overhead) / float64(baselineDuration) * 100

	// Metrics should add minimal overhead (less than 30%)
	assert.Less(t, overheadPercentage, 30.0,
		"Metrics overhead is %.2f%%, should be less than 30%%", overheadPercentage)

	t.Logf("Metrics collection overhead: %v (%.2f%%) for %d operations", 
		overhead, overheadPercentage, iterations)
}

// TestPerformance_HealthCheckWithTimeout tests timeout behavior
func TestPerformance_HealthCheckWithTimeout(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	
	// Register slow checker that takes longer than timeout
	slowChecker := NewSlowChecker("slow", 2*time.Second)
	hc.Register("slow", slowChecker)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	response := hc.Check(ctx)
	duration := time.Since(start)

	// Should complete within timeout + small buffer
	assert.Less(t, duration, 1*time.Second,
		"Health check with timeout took %v, should respect timeout", duration)

	// The slow checker should be cancelled
	assert.Len(t, response.Checks, 1)
	check := response.Checks[0]
	assert.Equal(t, "slow", check.Name)
	// Status might be unhealthy due to timeout

	t.Logf("Health check with timeout completed in %v", duration)
}

// TestPerformance_MemoryAllocation tests memory allocation during health checks
func TestPerformance_MemoryAllocation(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	
	// Register multiple checkers
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		hc.Register(name, checker)
	}

	ctx := context.Background()
	
	// Warm up to avoid initialization allocations
	for i := 0; i < 10; i++ {
		hc.Check(ctx)
	}

	// Force garbage collection
	runtime.GC()
	runtime.GC()

	// Measure memory before
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	// Perform health checks
	iterations := 1000
	for i := 0; i < iterations; i++ {
		hc.Check(ctx)
	}

	// Force garbage collection and measure memory after
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Calculate memory allocation
	allocatedBytes := m2.TotalAlloc - m1.TotalAlloc
	allocatedMB := float64(allocatedBytes) / 1024 / 1024

	// Should not allocate excessive memory
	assert.Less(t, allocatedMB, float64(MaxMemoryAllocationMB),
		"Health checks allocated %.2f MB, should be less than %d MB", allocatedMB, MaxMemoryAllocationMB)

	avgBytesPerCheck := float64(allocatedBytes) / float64(iterations)
	t.Logf("Memory allocation: %.2f MB total, %.2f bytes per check", allocatedMB, avgBytesPerCheck)
}

// TestPerformance_CacheEffectiveness tests health check caching performance
func TestPerformance_CacheEffectiveness(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	hc.SetCacheTTL(100 * time.Millisecond)
	
	// Register slow checker
	checker := NewMockChecker("slow").WithStatus(StatusHealthy).WithDuration(50 * time.Millisecond)
	hc.Register("slow", checker)

	ctx := context.Background()

	// First call - should be slow
	start := time.Now()
	response1 := hc.Check(ctx)
	duration1 := time.Since(start)

	// Second call immediately - should be fast (cached)
	start = time.Now()
	response2 := hc.Check(ctx)
	duration2 := time.Since(start)

	assert.Equal(t, response1.Timestamp, response2.Timestamp, "Second response should be cached")
	assert.Less(t, duration2, duration1/10, "Cached response should be much faster")

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call - should be slow again
	start = time.Now()
	response3 := hc.Check(ctx)
	duration3 := time.Since(start)

	assert.NotEqual(t, response1.Timestamp, response3.Timestamp, "Third response should not be cached")
	assert.Greater(t, duration3, duration2*10, "Non-cached response should be slower")

	t.Logf("Cache effectiveness: first=%v, cached=%v, expired=%v", duration1, duration2, duration3)
}

// Benchmark tests for automated performance monitoring

func BenchmarkHealthCheck_SingleChecker(b *testing.B) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.Check(ctx)
	}
}

func BenchmarkHealthCheck_MultipleCheckers(b *testing.B) {
	hc := New("1.0.0", zap.NewNop())
	
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		hc.Register(name, checker)
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.Check(ctx)
	}
}

func BenchmarkEnterpriseHealthCheck_Standard(b *testing.B) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		ehc.Register(name, checker)
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ehc.CheckWithMode(ctx, ModeStandard)
	}
}

func BenchmarkEnterpriseHealthCheck_Deep(b *testing.B) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		ehc.Register(name, checker)
		
		depName := fmt.Sprintf("dep_%d", i)
		depChecker := NewMockChecker(depName).WithStatus(StatusHealthy)
		dep := CreateTestDependency(depName, DependencyTypeService, false, []string{}, depChecker)
		ehc.RegisterDependency(dep)
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ehc.CheckWithMode(ctx, ModeDeep)
	}
}

func BenchmarkCircuitBreaker_Execute(b *testing.B) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)
	
	successFunc := func() (interface{}, error) {
		return "success", nil
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(successFunc)
	}
}

func BenchmarkDependencyManager_CheckAll(b *testing.B) {
	dm := NewDependencyManager(zap.NewNop())
	
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("dep_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		dep := CreateTestDependency(name, DependencyTypeService, false, []string{}, checker)
		dm.Register(dep)
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dm.CheckAll(ctx)
	}
}

func BenchmarkMetrics_RecordCheck(b *testing.B) {
	config := TestMetricsConfig()
	metrics := NewHealthMetricsWithConfig(config)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordCheck(StatusHealthy, time.Millisecond)
	}
}

// Helper function for min calculation
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}