// Package healthcheck enterprise tests
// Tests for enterprise health check functionality including maintenance mode and system info
package healthcheck

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewEnterpriseHealthCheck(t *testing.T) {
	logger := zap.NewNop()
	version := "1.0.0"

	ehc := NewEnterpriseHealthCheck(version, logger)

	assert.NotNil(t, ehc)
	assert.NotNil(t, ehc.HealthCheck)
	assert.Equal(t, version, ehc.version)
	assert.Equal(t, logger, ehc.logger)
	assert.NotNil(t, ehc.dependencies)
	assert.NotNil(t, ehc.circuitBreakers)
	assert.NotNil(t, ehc.metrics)
	assert.False(t, ehc.maintenanceMode)
	assert.False(t, ehc.gracefulShutdown)
}

func TestEnterpriseHealthCheck_RegisterWithCircuitBreaker(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	checker := NewMockChecker("database").WithStatus(StatusHealthy)
	config := TestCircuitBreakerConfig()

	ehc.RegisterWithCircuitBreaker("database", checker, config)

	// Verify checker was registered
	assert.Len(t, ehc.checkers, 1)
	assert.Contains(t, ehc.checkers, "database")

	// Verify circuit breaker was created
	assert.Len(t, ehc.circuitBreakers, 1)
	assert.Contains(t, ehc.circuitBreakers, "database")

	cb := ehc.circuitBreakers["database"]
	assert.Equal(t, "database", cb.name)
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestEnterpriseHealthCheck_RegisterDependency(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	checker := NewMockChecker("postgres").WithStatus(StatusHealthy)
	dep := CreateTestDependency("postgres", DependencyTypeDatabase, true, []string{}, checker)

	ehc.RegisterDependency(dep)

	deps := ehc.dependencies.GetDependencies()
	assert.Len(t, deps, 1)
	assert.Contains(t, deps, "postgres")
}

func TestEnterpriseHealthCheck_SetMaintenanceMode(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Hour)

	// Enable maintenance mode
	ehc.SetMaintenanceMode(true, "Scheduled maintenance", &startTime, &endTime)

	assert.True(t, ehc.IsMaintenanceMode())

	// Disable maintenance mode
	ehc.SetMaintenanceMode(false, "", nil, nil)

	assert.False(t, ehc.IsMaintenanceMode())
}

func TestEnterpriseHealthCheck_PrepareShutdown(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())

	assert.False(t, ehc.IsShuttingDown())

	ehc.PrepareShutdown()

	assert.True(t, ehc.IsShuttingDown())
}

func TestEnterpriseHealthCheck_CheckWithMode_Standard(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register basic checker
	checker := NewMockChecker("api").WithStatus(StatusHealthy).WithMessage("API OK")
	ehc.Register("api", checker)

	response := ehc.CheckWithMode(ctx, ModeStandard)

	AssertEnterpriseResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
	assert.Len(t, response.Checks, 1)
	assert.Equal(t, "api", response.Checks[0].Name)
	assert.Equal(t, StatusHealthy, response.Checks[0].Status)
}

func TestEnterpriseHealthCheck_CheckWithMode_Deep(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register basic checker
	checker := NewMockChecker("api").WithStatus(StatusHealthy)
	ehc.Register("api", checker)

	// Register dependency
	depChecker := NewMockChecker("database").WithStatus(StatusHealthy)
	dep := CreateTestDependency("postgres", DependencyTypeDatabase, true, []string{}, depChecker)
	ehc.RegisterDependency(dep)

	response := ehc.CheckWithMode(ctx, ModeDeep)

	AssertEnterpriseResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
	assert.Len(t, response.Checks, 1)
	assert.Len(t, response.Dependencies, 1)
	assert.Equal(t, "postgres", response.Dependencies[0].Name)
	assert.Equal(t, StatusHealthy, response.Dependencies[0].Status)
}

func TestEnterpriseHealthCheck_CheckWithMode_Quick(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register basic checker
	checker := NewMockChecker("api").WithStatus(StatusHealthy)
	ehc.Register("api", checker)

	// Register dependency
	depChecker := NewMockChecker("database").WithStatus(StatusHealthy)
	dep := CreateTestDependency("postgres", DependencyTypeDatabase, true, []string{}, depChecker)
	ehc.RegisterDependency(dep)

	response := ehc.CheckWithMode(ctx, ModeQuick)

	AssertEnterpriseResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
	assert.Len(t, response.Checks, 1)
	assert.Empty(t, response.Dependencies) // Dependencies not checked in quick mode
}

func TestEnterpriseHealthCheck_CheckWithMode_MaintenanceMode(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Enable maintenance mode
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Hour)
	ehc.SetMaintenanceMode(true, "Scheduled maintenance", &startTime, &endTime)

	// Register checker
	checker := NewMockChecker("api").WithStatus(StatusHealthy)
	ehc.Register("api", checker)

	// Check with standard mode - should be degraded
	response := ehc.CheckWithMode(ctx, ModeStandard)
	assert.Equal(t, StatusDegraded, response.Status)
	assert.NotNil(t, response.Maintenance)
	assert.True(t, response.Maintenance.Enabled)
	assert.Equal(t, "System in maintenance mode", response.Maintenance.Message)

	// Check with maintenance mode - should be healthy
	response = ehc.CheckWithMode(ctx, ModeMaintenance)
	assert.Equal(t, StatusHealthy, response.Status)
	assert.NotNil(t, response.Maintenance)
	assert.True(t, response.Maintenance.Enabled)
}

func TestEnterpriseHealthCheck_CheckWithMode_CriticalDependencyFailure(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register basic checker
	checker := NewMockChecker("api").WithStatus(StatusHealthy)
	ehc.Register("api", checker)

	// Register critical dependency that fails
	depChecker := NewMockChecker("database").WithStatus(StatusUnhealthy).WithMessage("Connection failed")
	dep := CreateTestDependency("postgres", DependencyTypeDatabase, true, []string{}, depChecker)
	ehc.RegisterDependency(dep)

	response := ehc.CheckWithMode(ctx, ModeStandard)

	assert.Equal(t, StatusUnhealthy, response.Status) // Should be unhealthy due to critical dependency
	assert.Len(t, response.Dependencies, 1)
	assert.Equal(t, StatusUnhealthy, response.Dependencies[0].Status)
	assert.True(t, response.Dependencies[0].Critical)
}

func TestEnterpriseHealthCheck_CheckWithMode_NonCriticalDependencyFailure(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register basic checker
	checker := NewMockChecker("api").WithStatus(StatusHealthy)
	ehc.Register("api", checker)

	// Register non-critical dependency that fails
	depChecker := NewMockChecker("cache").WithStatus(StatusUnhealthy).WithMessage("Cache unavailable")
	dep := CreateTestDependency("redis", DependencyTypeCache, false, []string{}, depChecker)
	ehc.RegisterDependency(dep)

	response := ehc.CheckWithMode(ctx, ModeStandard)

	assert.Equal(t, StatusHealthy, response.Status) // Should remain healthy since dependency is not critical
	assert.Len(t, response.Dependencies, 1)
	assert.Equal(t, StatusUnhealthy, response.Dependencies[0].Status)
	assert.False(t, response.Dependencies[0].Critical)
}

func TestEnterpriseHealthCheck_CheckWithMode_CircuitBreakers(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register checker with circuit breaker
	checker := NewMockChecker("database").WithStatus(StatusHealthy)
	config := TestCircuitBreakerConfig()
	ehc.RegisterWithCircuitBreaker("database", checker, config)

	response := ehc.CheckWithMode(ctx, ModeStandard)

	assert.Equal(t, StatusHealthy, response.Status)
	assert.Len(t, response.CircuitBreakers, 1)
	assert.Contains(t, response.CircuitBreakers, "database")
	
	cbStatus := response.CircuitBreakers["database"]
	assert.Equal(t, "database", cbStatus.Name)
	assert.Equal(t, StateClosed, cbStatus.State)
}

func TestEnterpriseHealthCheck_CheckDependencies(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register multiple dependencies
	dbChecker := NewMockChecker("database").WithStatus(StatusHealthy)
	dbDep := CreateTestDependency("postgres", DependencyTypeDatabase, true, []string{}, dbChecker)
	ehc.RegisterDependency(dbDep)

	cacheChecker := NewMockChecker("cache").WithStatus(StatusDegraded)
	cacheDep := CreateTestDependency("redis", DependencyTypeCache, false, []string{}, cacheChecker)
	ehc.RegisterDependency(cacheDep)

	dependencies := ehc.CheckDependencies(ctx)

	assert.Len(t, dependencies, 2)
	
	// Find each dependency
	var dbDependency, cacheDependency *DependencyStatus
	for i := range dependencies {
		if dependencies[i].Name == "postgres" {
			dbDependency = &dependencies[i]
		} else if dependencies[i].Name == "redis" {
			cacheDependency = &dependencies[i]
		}
	}

	require.NotNil(t, dbDependency)
	assert.Equal(t, DependencyTypeDatabase, dbDependency.Type)
	assert.Equal(t, StatusHealthy, dbDependency.Status)
	assert.True(t, dbDependency.Critical)

	require.NotNil(t, cacheDependency)
	assert.Equal(t, DependencyTypeCache, cacheDependency.Type)
	assert.Equal(t, StatusDegraded, cacheDependency.Status)
	assert.False(t, cacheDependency.Critical)
}

func TestEnterpriseHealthCheck_GetCircuitBreakerStatus(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())

	// Register multiple checkers with circuit breakers
	checker1 := NewMockChecker("database").WithStatus(StatusHealthy)
	config1 := TestCircuitBreakerConfig()
	ehc.RegisterWithCircuitBreaker("database", checker1, config1)

	checker2 := NewMockChecker("api").WithStatus(StatusHealthy)
	config2 := TestCircuitBreakerConfig()
	ehc.RegisterWithCircuitBreaker("external_api", checker2, config2)

	status := ehc.GetCircuitBreakerStatus()

	assert.Len(t, status, 2)
	assert.Contains(t, status, "database")
	assert.Contains(t, status, "external_api")

	dbStatus := status["database"]
	assert.Equal(t, "database", dbStatus.Name)
	assert.Equal(t, StateClosed, dbStatus.State)

	apiStatus := status["external_api"]
	assert.Equal(t, "external_api", apiStatus.Name)
	assert.Equal(t, StateClosed, apiStatus.State)
}

func TestCircuitBreakerChecker_Check_Success(t *testing.T) {
	checker := NewMockChecker("database").WithStatus(StatusHealthy).WithMessage("OK")
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("database", config)

	cbChecker := &CircuitBreakerChecker{
		checker: checker,
		breaker: cb,
		name:    "database",
	}

	ctx := context.Background()
	result := cbChecker.Check(ctx)

	assert.Equal(t, "database", result.Name)
	assert.Equal(t, StatusHealthy, result.Status)
	assert.Equal(t, "OK", result.Message)
	assert.NotNil(t, result.Metadata)
	
	if metadata, ok := result.Metadata.(map[string]interface{}); ok {
		assert.Equal(t, "closed", metadata["circuit_breaker_state"])
	}
}

func TestCircuitBreakerChecker_Check_Failure(t *testing.T) {
	checker := NewMockChecker("database").WithStatus(StatusUnhealthy).WithMessage("Connection failed")
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("database", config)

	cbChecker := &CircuitBreakerChecker{
		checker: checker,
		breaker: cb,
		name:    "database",
	}

	ctx := context.Background()
	result := cbChecker.Check(ctx)

	assert.Equal(t, "database", result.Name)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "health check failed")
}

func TestCircuitBreakerChecker_Check_CircuitOpen(t *testing.T) {
	checker := NewFailingChecker("database", "Connection failed")
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("database", config)

	cbChecker := &CircuitBreakerChecker{
		checker: checker,
		breaker: cb,
		name:    "database",
	}

	ctx := context.Background()

	// Trip the circuit breaker
	for i := 0; i < config.FailureThreshold; i++ {
		cbChecker.Check(ctx)
	}

	// Next check should be rejected
	result := cbChecker.Check(ctx)

	assert.Equal(t, "database", result.Name)
	assert.Equal(t, StatusUnhealthy, result.Status)
	assert.Contains(t, result.Message, "circuit breaker")
	assert.NotNil(t, result.Metadata)
	
	if metadata, ok := result.Metadata.(map[string]interface{}); ok {
		assert.Equal(t, "open", metadata["circuit_breaker_state"])
	}
}

func TestEnterpriseHealthCheck_SystemInfo(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	response := ehc.CheckWithMode(ctx, ModeStandard)

	systemInfo := response.SystemInfo
	assert.NotEmpty(t, systemInfo.Hostname)
	assert.NotEmpty(t, systemInfo.Platform)
	assert.NotEmpty(t, systemInfo.Architecture)
	assert.Greater(t, systemInfo.CPUCores, 0)
	assert.Greater(t, systemInfo.Memory.Total, uint64(0))
	assert.NotZero(t, systemInfo.Uptime)
	assert.NotNil(t, systemInfo.Environment)
}

func TestEnterpriseHealthCheck_ComplexScenario(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register basic health checks
	apiChecker := NewMockChecker("api").WithStatus(StatusHealthy)
	ehc.Register("api", apiChecker)

	// Register dependencies with relationships
	dbChecker := NewMockChecker("database").WithStatus(StatusHealthy)
	dbDep := CreateTestDependency("postgres", DependencyTypeDatabase, true, []string{}, dbChecker)
	ehc.RegisterDependency(dbDep)

	cacheChecker := NewMockChecker("cache").WithStatus(StatusHealthy)
	cacheDep := CreateTestDependency("redis", DependencyTypeCache, false, []string{"postgres"}, cacheChecker)
	ehc.RegisterDependency(cacheDep)

	// Register services with circuit breakers
	serviceChecker := NewMockChecker("service").WithStatus(StatusHealthy)
	config := TestCircuitBreakerConfig()
	ehc.RegisterWithCircuitBreaker("external_service", serviceChecker, config)

	// Perform deep check
	response := ehc.CheckWithMode(ctx, ModeDeep)

	// Verify comprehensive response
	AssertEnterpriseResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
	
	// Should have basic checks
	assert.Len(t, response.Checks, 2) // api + external_service (with circuit breaker)
	
	// Should have dependencies in topological order
	assert.Len(t, response.Dependencies, 2)
	AssertDependencyOrder(t, response.Dependencies, []string{"postgres", "redis"})
	
	// Should have circuit breaker status
	assert.Len(t, response.CircuitBreakers, 1)
	assert.Contains(t, response.CircuitBreakers, "external_service")
	
	// Should have system info
	assert.NotEmpty(t, response.SystemInfo.Hostname)
}

func TestEnterpriseHealthCheck_ConcurrentAccess(t *testing.T) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register some checkers and dependencies
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		ehc.Register(name, checker)

		depName := fmt.Sprintf("dep_%d", i)
		depChecker := NewMockChecker(depName).WithStatus(StatusHealthy)
		dep := CreateTestDependency(depName, DependencyTypeService, false, []string{}, depChecker)
		ehc.RegisterDependency(dep)
	}

	// Run concurrent operations
	numGoroutines := 50
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			mode := ModeStandard
			if id%3 == 0 {
				mode = ModeDeep
			} else if id%3 == 1 {
				mode = ModeQuick
			}

			response := ehc.CheckWithMode(ctx, mode)
			AssertEnterpriseResponseStructure(t, response)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Final check to ensure state is consistent
	response := ehc.CheckWithMode(ctx, ModeDeep)
	AssertEnterpriseResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
}

// Benchmark tests
func BenchmarkEnterpriseHealthCheck_CheckWithMode_Standard(b *testing.B) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register multiple checkers
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		ehc.Register(name, checker)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ehc.CheckWithMode(ctx, ModeStandard)
	}
}

func BenchmarkEnterpriseHealthCheck_CheckWithMode_Deep(b *testing.B) {
	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	ctx := context.Background()

	// Register checkers and dependencies
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		ehc.Register(name, checker)

		depName := fmt.Sprintf("dep_%d", i)
		depChecker := NewMockChecker(depName).WithStatus(StatusHealthy)
		dep := CreateTestDependency(depName, DependencyTypeService, false, []string{}, depChecker)
		ehc.RegisterDependency(dep)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ehc.CheckWithMode(ctx, ModeDeep)
	}
}

func BenchmarkCircuitBreakerChecker_Check(b *testing.B) {
	checker := NewMockChecker("database").WithStatus(StatusHealthy)
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("database", config)

	cbChecker := &CircuitBreakerChecker{
		checker: checker,
		breaker: cb,
		name:    "database",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cbChecker.Check(ctx)
	}
}