// Package healthcheck integration tests
// Tests with real PostgreSQL and Redis connections per ADR-0012
package healthcheck

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestIntegration_DatabaseChecker tests health checking with real PostgreSQL
func TestIntegration_DatabaseChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()

	// Create database checker
	dbChecker := NewDatabaseChecker(pgPool)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test healthy database
	check := dbChecker.Check(ctx)

	assert.Equal(t, "database", check.Name)
	assert.Equal(t, StatusHealthy, check.Status)
	assert.Empty(t, check.Message)
	assert.NotZero(t, check.LastChecked)
	assert.Greater(t, check.Duration, time.Duration(0))
	assert.NotNil(t, check.Metadata)

	// Verify metadata contains connection pool information
	metadata, ok := check.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, metadata, "total_conns")
	assert.Contains(t, metadata, "idle_conns")
	assert.Contains(t, metadata, "acquired_conns")
	assert.Contains(t, metadata, "max_conns")
}

// TestIntegration_DatabaseChecker_ConnectionPool tests connection pool monitoring
func TestIntegration_DatabaseChecker_ConnectionPool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()

	// Create database checker
	dbChecker := NewDatabaseChecker(pgPool)

	ctx := context.Background()

	// Perform multiple checks to simulate load
	for i := 0; i < 10; i++ {
		check := dbChecker.Check(ctx)
		assert.Equal(t, StatusHealthy, check.Status)
		
		// Verify metadata is populated
		metadata, ok := check.Metadata.(map[string]interface{})
		require.True(t, ok)
		
		totalConns := metadata["total_conns"].(int32)
		maxConns := metadata["max_conns"].(int32)
		
		assert.Greater(t, totalConns, int32(0))
		assert.Greater(t, maxConns, int32(0))
		assert.LessOrEqual(t, totalConns, maxConns)
	}
}

// TestIntegration_RedisChecker tests health checking with real Redis
func TestIntegration_RedisChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	redisClient := helper.SetupRedis()

	// Create Redis checker
	redisChecker := NewRedisChecker(redisClient)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test healthy Redis
	check := redisChecker.Check(ctx)

	assert.Equal(t, "redis", check.Name)
	assert.Equal(t, StatusHealthy, check.Status)
	assert.Empty(t, check.Message)
	assert.NotZero(t, check.LastChecked)
	assert.Greater(t, check.Duration, time.Duration(0))
	assert.NotNil(t, check.Metadata)

	// Verify metadata contains Redis info
	metadata, ok := check.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, metadata, "info")
}

// TestIntegration_HealthCheckWithRealDependencies tests complete health check with real services
func TestIntegration_HealthCheckWithRealDependencies(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()
	redisClient := helper.SetupRedis()

	// Create enterprise health check
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())

	// Register real database and Redis health checks
	dbChecker := NewDatabaseChecker(pgPool)
	ehc.Register("database", dbChecker)

	redisChecker := NewRedisChecker(redisClient)
	ehc.Register("redis", redisChecker)

	// Register dependencies
	dbDep := DatabaseDependency("postgres", true, dbChecker)
	ehc.RegisterDependency(dbDep)

	cacheDep := CacheDependency("redis_cache", false, redisChecker)
	ehc.RegisterDependency(cacheDep)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Perform deep health check
	response := ehc.CheckWithMode(ctx, ModeDeep)

	// Verify response structure
	AssertEnterpriseResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)

	// Verify basic checks
	assert.Len(t, response.Checks, 2)
	
	var dbCheck, redisCheck *Check
	for i := range response.Checks {
		if response.Checks[i].Name == "database" {
			dbCheck = &response.Checks[i]
		} else if response.Checks[i].Name == "redis" {
			redisCheck = &response.Checks[i]
		}
	}

	require.NotNil(t, dbCheck)
	assert.Equal(t, StatusHealthy, dbCheck.Status)
	assert.NotNil(t, dbCheck.Metadata)

	require.NotNil(t, redisCheck)
	assert.Equal(t, StatusHealthy, redisCheck.Status)
	assert.NotNil(t, redisCheck.Metadata)

	// Verify dependencies
	assert.Len(t, response.Dependencies, 2)
	
	var dbDependency, cacheDependency *DependencyStatus
	for i := range response.Dependencies {
		if response.Dependencies[i].Name == "postgres" {
			dbDependency = &response.Dependencies[i]
		} else if response.Dependencies[i].Name == "redis_cache" {
			cacheDependency = &response.Dependencies[i]
		}
	}

	require.NotNil(t, dbDependency)
	assert.Equal(t, DependencyTypeDatabase, dbDependency.Type)
	assert.Equal(t, StatusHealthy, dbDependency.Status)
	assert.True(t, dbDependency.Critical)

	require.NotNil(t, cacheDependency)
	assert.Equal(t, DependencyTypeCache, cacheDependency.Type)
	assert.Equal(t, StatusHealthy, cacheDependency.Status)
	assert.False(t, cacheDependency.Critical)
}

// TestIntegration_CircuitBreakerWithRealServices tests circuit breaker with real services
func TestIntegration_CircuitBreakerWithRealServices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()

	// Create enterprise health check
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())

	// Register database checker with circuit breaker
	dbChecker := NewDatabaseChecker(pgPool)
	config := TestCircuitBreakerConfig()
	ehc.RegisterWithCircuitBreaker("database", dbChecker, config)

	ctx := context.Background()

	// Perform multiple successful checks
	for i := 0; i < 5; i++ {
		response := ehc.CheckWithMode(ctx, ModeStandard)
		assert.Equal(t, StatusHealthy, response.Status)
		
		// Verify circuit breaker status
		assert.Len(t, response.CircuitBreakers, 1)
		cbStatus := response.CircuitBreakers["database"]
		assert.Equal(t, StateClosed, cbStatus.State)
		assert.Equal(t, 0, cbStatus.FailureCount)
	}

	// Close the connection pool to simulate failure
	pgPool.Close()

	// Now checks should fail, but circuit breaker should handle it
	for i := 0; i < config.FailureThreshold+1; i++ {
		response := ehc.CheckWithMode(ctx, ModeStandard)
		
		if i < config.FailureThreshold {
			// Should still try to check
			assert.Equal(t, StatusUnhealthy, response.Status)
			assert.Equal(t, StateClosed, response.CircuitBreakers["database"].State)
		} else {
			// Circuit should be open now
			assert.Equal(t, StateOpen, response.CircuitBreakers["database"].State)
		}
	}
}

// TestIntegration_ExternalServiceChecker tests external service health checking
func TestIntegration_ExternalServiceChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test with a reliable external service (httpbin.org)
	checker := NewExternalServiceChecker("httpbin", "https://httpbin.org/status/200", 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	check := checker.Check(ctx)

	assert.Equal(t, "httpbin", check.Name)
	assert.Equal(t, StatusHealthy, check.Status)
	assert.NotZero(t, check.LastChecked)
	assert.Greater(t, check.Duration, time.Duration(0))
	assert.NotNil(t, check.Metadata)

	// Verify metadata contains status code and URL
	metadata, ok := check.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 200, metadata["status_code"])
	assert.Equal(t, "https://httpbin.org/status/200", metadata["url"])
}

// TestIntegration_ExternalServiceChecker_Failure tests external service failure handling
func TestIntegration_ExternalServiceChecker_Failure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test with service that returns 500
	checker := NewExternalServiceChecker("failing_service", "https://httpbin.org/status/500", 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	check := checker.Check(ctx)

	assert.Equal(t, "failing_service", check.Name)
	assert.Equal(t, StatusUnhealthy, check.Status)
	assert.Equal(t, "Service returned error status", check.Message)

	// Verify metadata
	metadata, ok := check.Metadata.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 500, metadata["status_code"])
}

// TestIntegration_ExternalServiceChecker_Timeout tests timeout handling
func TestIntegration_ExternalServiceChecker_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test with service that delays response
	checker := NewExternalServiceChecker("slow_service", "https://httpbin.org/delay/3", 1*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	check := checker.Check(ctx)

	assert.Equal(t, "slow_service", check.Name)
	assert.Equal(t, StatusUnhealthy, check.Status)
	assert.Contains(t, check.Message, "timeout")
}

// TestIntegration_DependencyGraphWithRealServices tests dependency graph with real services
func TestIntegration_DependencyGraphWithRealServices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()
	redisClient := helper.SetupRedis()

	// Create dependency manager
	dm := NewDependencyManager(zap.NewNop())

	// Create dependency chain: database -> cache -> api
	dbChecker := NewDatabaseChecker(pgPool)
	dbDep := DatabaseDependency("postgres", true, dbChecker)
	dm.Register(dbDep)

	redisChecker := NewRedisChecker(redisClient)
	cacheDep := CreateTestDependency("redis", DependencyTypeCache, false, []string{"postgres"}, redisChecker)
	dm.Register(cacheDep)

	// Mock API service that depends on cache
	apiChecker := NewMockChecker("api").WithStatus(StatusHealthy)
	apiDep := CreateTestDependency("api_service", DependencyTypeService, false, []string{"redis"}, apiChecker)
	dm.Register(apiDep)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check all dependencies
	results := dm.CheckAll(ctx)

	require.Len(t, results, 3)

	// Verify topological order
	AssertDependencyOrder(t, results, []string{"postgres", "redis", "api_service"})

	// Verify all are healthy
	for _, result := range results {
		assert.Equal(t, StatusHealthy, result.Status, "Dependency %s should be healthy", result.Name)
	}
}

// TestIntegration_MetricsWithRealServices tests metrics collection with real services
func TestIntegration_MetricsWithRealServices(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()

	// Create metrics
	config := TestMetricsConfig()
	metrics := NewHealthMetricsWithConfig(config)

	// Create health check with metrics
	hc := New("1.0.0", zap.NewNop())
	
	// Register database checker with metrics middleware
	dbChecker := NewDatabaseChecker(pgPool)
	wrappedChecker := WithMetrics(metrics, dbChecker)
	hc.Register("database", wrappedChecker)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform several checks
	for i := 0; i < 5; i++ {
		response := hc.Check(ctx)
		assert.Equal(t, StatusHealthy, response.Status)
	}

	// Verify metrics were recorded
	// Note: In a real integration test, you would check the metrics registry
	// Here we just verify the checks completed successfully
}

// TestIntegration_FullHealthCheckSystem tests the complete health check system
func TestIntegration_FullHealthCheckSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()
	redisClient := helper.SetupRedis()

	// Create enterprise health check with all features
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())

	// Register basic health checks
	dbChecker := NewDatabaseChecker(pgPool)
	redisChecker := NewRedisChecker(redisClient)
	
	ehc.Register("database", dbChecker)
	ehc.Register("redis", redisChecker)

	// Register circuit breaker protected service
	apiChecker := NewExternalServiceChecker("api", "https://httpbin.org/status/200", 5*time.Second)
	config := TestCircuitBreakerConfig()
	ehc.RegisterWithCircuitBreaker("external_api", apiChecker, config)

	// Register dependencies
	dbDep := DatabaseDependency("postgres_db", true, dbChecker)
	ehc.RegisterDependency(dbDep)

	cacheDep := CacheDependency("redis_cache", false, redisChecker)
	ehc.RegisterDependency(cacheDep)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Test different modes
	modes := []HealthCheckMode{ModeQuick, ModeStandard, ModeDeep}

	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			response := ehc.CheckWithMode(ctx, mode)

			// Verify response structure
			AssertEnterpriseResponseStructure(t, response)
			assert.Equal(t, StatusHealthy, response.Status)

			// Verify checks are present
			assert.GreaterOrEqual(t, len(response.Checks), 2) // At least database and redis

			// Verify dependencies based on mode
			if mode == ModeDeep || mode == ModeStandard {
				assert.Len(t, response.Dependencies, 2)
			} else {
				assert.Empty(t, response.Dependencies)
			}

			// Verify circuit breakers
			assert.Len(t, response.CircuitBreakers, 1)
			assert.Contains(t, response.CircuitBreakers, "external_api")

			// Verify system info
			assert.NotEmpty(t, response.SystemInfo.Hostname)
			assert.Greater(t, response.SystemInfo.CPUCores, 0)
		})
	}
}

// TestIntegration_HealthCheckUnderLoad tests health check performance under load
func TestIntegration_HealthCheckUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()
	redisClient := helper.SetupRedis()

	// Create health check
	hc := New("1.0.0", zap.NewNop())
	
	dbChecker := NewDatabaseChecker(pgPool)
	redisChecker := NewRedisChecker(redisClient)
	
	hc.Register("database", dbChecker)
	hc.Register("redis", redisChecker)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Perform concurrent health checks
	numGoroutines := 50
	numChecksPerGoroutine := 10
	
	done := make(chan bool, numGoroutines)
	
	start := time.Now()
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			
			for j := 0; j < numChecksPerGoroutine; j++ {
				response := hc.Check(ctx)
				assert.Equal(t, StatusHealthy, response.Status)
				assert.Len(t, response.Checks, 2)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	elapsed := time.Since(start)
	totalChecks := numGoroutines * numChecksPerGoroutine
	
	t.Logf("Performed %d health checks in %v (%.2f checks/second)", 
		totalChecks, elapsed, float64(totalChecks)/elapsed.Seconds())

	// Verify reasonable performance (should complete within reasonable time)
	assert.Less(t, elapsed, 30*time.Second, "Health checks took too long under load")
}

// TestIntegration_DatabaseFailureRecovery tests database failure and recovery scenarios
func TestIntegration_DatabaseFailureRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	pgPool := helper.SetupPostgreSQL()

	// Create health check
	hc := New("1.0.0", zap.NewNop())
	dbChecker := NewDatabaseChecker(pgPool)
	hc.Register("database", dbChecker)

	ctx := context.Background()

	// Verify initial healthy state
	response := hc.Check(ctx)
	assert.Equal(t, StatusHealthy, response.Status)

	// Simulate failure by closing connection pool
	pgPool.Close()

	// Health check should now fail
	response = hc.Check(ctx)
	assert.Equal(t, StatusUnhealthy, response.Status)
	assert.Len(t, response.Checks, 1)
	assert.Equal(t, StatusUnhealthy, response.Checks[0].Status)
	assert.NotEmpty(t, response.Checks[0].Message)

	// Recovery would require setting up a new connection
	// In a real scenario, the application would reconnect automatically
}

// TestIntegration_RedisFailureHandling tests Redis failure scenarios
func TestIntegration_RedisFailureHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	helper := NewTestHealthCheckHelper(t)
	redisClient := helper.SetupRedis()

	// Create health check
	hc := New("1.0.0", zap.NewNop())
	redisChecker := NewRedisChecker(redisClient)
	hc.Register("redis", redisChecker)

	ctx := context.Background()

	// Verify initial healthy state
	response := hc.Check(ctx)
	assert.Equal(t, StatusHealthy, response.Status)

	// Simulate failure by closing Redis client
	redisClient.Close()

	// Health check should now fail
	response = hc.Check(ctx)
	assert.Equal(t, StatusUnhealthy, response.Status)
	assert.Len(t, response.Checks, 1)
	assert.Equal(t, StatusUnhealthy, response.Checks[0].Status)
	assert.NotEmpty(t, response.Checks[0].Message)
}