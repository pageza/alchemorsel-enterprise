// Package healthcheck test suite
// Comprehensive test suite runner and test organization
package healthcheck

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// HealthCheckTestSuite provides a comprehensive test suite for health check functionality
type HealthCheckTestSuite struct {
	suite.Suite
	logger *zap.Logger
	helper *TestHealthCheckHelper
}

// SetupSuite runs before the entire test suite
func (suite *HealthCheckTestSuite) SetupSuite() {
	suite.logger = zap.NewNop() // Silent logger for tests
}

// SetupTest runs before each test
func (suite *HealthCheckTestSuite) SetupTest() {
	suite.helper = NewTestHealthCheckHelper(suite.T())
}

// TearDownTest runs after each test
func (suite *HealthCheckTestSuite) TearDownTest() {
	if suite.helper != nil {
		suite.helper.Cleanup()
	}
}

// TestBasicHealthCheck tests basic health check functionality
func (suite *HealthCheckTestSuite) TestBasicHealthCheck() {
	hc := New("1.0.0", suite.logger)
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	ctx := suite.helper.CreateContext()
	response := hc.Check(ctx)

	AssertResponseStructure(suite.T(), response)
	suite.Equal(StatusHealthy, response.Status)
	suite.Len(response.Checks, 1)
}

// TestCircuitBreakerIntegration tests circuit breaker integration
func (suite *HealthCheckTestSuite) TestCircuitBreakerIntegration() {
	ehc := NewEnterpriseHealthCheck("1.0.0", suite.logger)
	checker := NewMockChecker("database").WithStatus(StatusHealthy)
	config := TestCircuitBreakerConfig()

	ehc.RegisterWithCircuitBreaker("database", checker, config)

	ctx := suite.helper.CreateContext()
	response := ehc.CheckWithMode(ctx, ModeStandard)

	AssertEnterpriseResponseStructure(suite.T(), response)
	suite.Equal(StatusHealthy, response.Status)
	suite.Len(response.CircuitBreakers, 1)
}

// TestDependencyManagement tests dependency management functionality
func (suite *HealthCheckTestSuite) TestDependencyManagement() {
	dm := NewDependencyManager(suite.logger)

	// Create dependency chain
	dbChecker := NewMockChecker("database").WithStatus(StatusHealthy)
	dbDep := DatabaseDependency("postgres", true, dbChecker)
	dm.Register(dbDep)

	cacheChecker := NewMockChecker("cache").WithStatus(StatusHealthy)
	cacheDep := CreateTestDependency("redis", DependencyTypeCache, false, []string{"postgres"}, cacheChecker)
	dm.Register(cacheDep)

	ctx := suite.helper.CreateContext()
	results := dm.CheckAll(ctx)

	suite.Len(results, 2)
	AssertDependencyOrder(suite.T(), results, []string{"postgres", "redis"})
}

// TestMetricsCollection tests metrics collection functionality
func (suite *HealthCheckTestSuite) TestMetricsCollection() {
	config := TestMetricsConfig()
	metrics := NewHealthMetricsWithConfig(config)

	// Record various metrics
	metrics.RecordCheck(StatusHealthy, suite.helper.GetTestDuration())
	metrics.RecordCheckByName("test", StatusHealthy, suite.helper.GetTestDuration())
	metrics.RecordCheckError("test", "test_error")
	metrics.RecordDependencyStatus("postgres", DependencyTypeDatabase, true, StatusHealthy)
	metrics.RecordCircuitBreakerState("circuit", StateClosed)
	metrics.RecordCircuitTrip("circuit", "test_reason")

	// Verify metrics registry can be created
	registry := metrics.GetMetricsHandler()
	suite.NotNil(registry)
}

// TestPerformanceRequirements tests performance requirements
func (suite *HealthCheckTestSuite) TestPerformanceRequirements() {
	hc := New("1.0.0", suite.logger)
	
	// Register fast checkers
	for i := 0; i < 5; i++ {
		name := suite.helper.GetTestName(i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy).WithDuration(suite.helper.GetTestDuration())
		hc.Register(name, checker)
	}

	ctx := suite.helper.CreateContext()
	
	start := suite.helper.GetStartTime()
	response := hc.Check(ctx)
	duration := suite.helper.GetElapsed(start)

	suite.Equal(StatusHealthy, response.Status)
	suite.Less(duration, MaxHealthCheckDuration)
}

// TestErrorHandling tests comprehensive error handling
func (suite *HealthCheckTestSuite) TestErrorHandling() {
	hc := New("1.0.0", suite.logger)
	
	// Register failing checker
	failingChecker := NewFailingChecker("failing", "Test failure")
	hc.Register("failing", failingChecker)

	ctx := suite.helper.CreateContext()
	response := hc.Check(ctx)

	suite.Equal(StatusUnhealthy, response.Status)
	suite.Len(response.Checks, 1)
	suite.Equal(StatusUnhealthy, response.Checks[0].Status)
	suite.Contains(response.Checks[0].Message, "Test failure")
}

// TestConcurrentAccess tests concurrent access scenarios
func (suite *HealthCheckTestSuite) TestConcurrentAccess() {
	hc := New("1.0.0", suite.logger)
	checker := NewMockChecker("concurrent").WithStatus(StatusHealthy)
	hc.Register("concurrent", checker)

	ctx := suite.helper.CreateContext()
	
	// Run concurrent health checks
	suite.helper.RunConcurrentChecks(hc, ctx, 10, 5)
	
	// Verify final state
	response := hc.Check(ctx)
	suite.Equal(StatusHealthy, response.Status)
}

// Helper methods for the test suite

// CreateContext creates a test context
func (h *TestHealthCheckHelper) CreateContext() context.Context {
	return context.Background()
}

// GetTestDuration returns a standard test duration
func (h *TestHealthCheckHelper) GetTestDuration() time.Duration {
	return 10 * time.Millisecond
}

// GetTestName generates a test name
func (h *TestHealthCheckHelper) GetTestName(index int) string {
	return fmt.Sprintf("test_%d", index)
}

// GetStartTime returns current time for performance testing
func (h *TestHealthCheckHelper) GetStartTime() time.Time {
	return time.Now()
}

// GetElapsed calculates elapsed time
func (h *TestHealthCheckHelper) GetElapsed(start time.Time) time.Duration {
	return time.Since(start)
}

// RunConcurrentChecks runs health checks concurrently
func (h *TestHealthCheckHelper) RunConcurrentChecks(hc *HealthCheck, ctx context.Context, numGoroutines, numChecks int) {
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < numChecks; j++ {
				hc.Check(ctx)
			}
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestSuite_RunAll runs the comprehensive test suite
func TestSuite_RunAll(t *testing.T) {
	suite.Run(t, new(HealthCheckTestSuite))
}

// IntegrationTestSuite provides integration tests with real services
type IntegrationTestSuite struct {
	suite.Suite
	helper *TestHealthCheckHelper
}

// SetupTest runs before each integration test
func (suite *IntegrationTestSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("Skipping integration test in short mode")
	}
	suite.helper = NewTestHealthCheckHelper(suite.T())
}

// TearDownTest runs after each integration test
func (suite *IntegrationTestSuite) TearDownTest() {
	if suite.helper != nil {
		suite.helper.Cleanup()
	}
}

// TestDatabaseIntegration tests database integration
func (suite *IntegrationTestSuite) TestDatabaseIntegration() {
	pgPool := suite.helper.SetupPostgreSQL()
	dbChecker := NewDatabaseChecker(pgPool)

	ctx := context.Background()
	check := dbChecker.Check(ctx)

	suite.Equal("database", check.Name)
	suite.Equal(StatusHealthy, check.Status)
	suite.NotNil(check.Metadata)
}

// TestRedisIntegration tests Redis integration
func (suite *IntegrationTestSuite) TestRedisIntegration() {
	redisClient := suite.helper.SetupRedis()
	redisChecker := NewRedisChecker(redisClient)

	ctx := context.Background()
	check := redisChecker.Check(ctx)

	suite.Equal("redis", check.Name)
	suite.Equal(StatusHealthy, check.Status)
	suite.NotNil(check.Metadata)
}

// TestFullSystemIntegration tests complete system integration
func (suite *IntegrationTestSuite) TestFullSystemIntegration() {
	pgPool := suite.helper.SetupPostgreSQL()
	redisClient := suite.helper.SetupRedis()

	ehc := NewEnterpriseHealthCheck("1.0.0", zap.NewNop())

	// Register real services
	dbChecker := NewDatabaseChecker(pgPool)
	redisChecker := NewRedisChecker(redisClient)
	
	ehc.Register("database", dbChecker)
	ehc.Register("redis", redisChecker)

	// Register dependencies
	dbDep := DatabaseDependency("postgres", true, dbChecker)
	ehc.RegisterDependency(dbDep)

	cacheDep := CacheDependency("redis_cache", false, redisChecker)
	ehc.RegisterDependency(cacheDep)

	ctx := context.Background()
	response := ehc.CheckWithMode(ctx, ModeDeep)

	AssertEnterpriseResponseStructure(suite.T(), response)
	suite.Equal(StatusHealthy, response.Status)
	suite.Len(response.Checks, 2)
	suite.Len(response.Dependencies, 2)
}

// TestIntegrationSuite_RunAll runs the integration test suite
func TestIntegrationSuite_RunAll(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

// BenchmarkSuite provides performance benchmarks
type BenchmarkSuite struct {
	suite.Suite
	hc     *HealthCheck
	ehc    *EnterpriseHealthCheck
	helper *TestHealthCheckHelper
}

// SetupSuite runs before benchmark suite
func (suite *BenchmarkSuite) SetupSuite() {
	suite.helper = NewTestHealthCheckHelper(suite.T())
	
	// Setup basic health check
	suite.hc = New("1.0.0", zap.NewNop())
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		suite.hc.Register(name, checker)
	}

	// Setup enterprise health check
	suite.ehc = NewEnterpriseHealthCheck("1.0.0", zap.NewNop())
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("checker_%d", i)
		checker := NewMockChecker(name).WithStatus(StatusHealthy)
		suite.ehc.Register(name, checker)

		depName := fmt.Sprintf("dep_%d", i)
		depChecker := NewMockChecker(depName).WithStatus(StatusHealthy)
		dep := CreateTestDependency(depName, DependencyTypeService, false, []string{}, depChecker)
		suite.ehc.RegisterDependency(dep)
	}
}

// TearDownSuite runs after benchmark suite
func (suite *BenchmarkSuite) TearDownSuite() {
	if suite.helper != nil {
		suite.helper.Cleanup()
	}
}

// TestBenchmarkBasicHealthCheck benchmarks basic health checks
func (suite *BenchmarkSuite) TestBenchmarkBasicHealthCheck() {
	ctx := context.Background()
	
	// Warm up
	suite.hc.Check(ctx)
	
	// Simple performance test
	start := time.Now()
	for i := 0; i < 100; i++ {
		response := suite.hc.Check(ctx)
		suite.Equal(StatusHealthy, response.Status)
	}
	duration := time.Since(start)
	
	avgDuration := duration / 100
	suite.Less(avgDuration, 10*time.Millisecond, "Average health check should be under 10ms")
}

// TestBenchmarkEnterpriseHealthCheck benchmarks enterprise health checks
func (suite *BenchmarkSuite) TestBenchmarkEnterpriseHealthCheck() {
	ctx := context.Background()
	
	// Warm up
	suite.ehc.CheckWithMode(ctx, ModeDeep)
	
	// Performance test
	start := time.Now()
	for i := 0; i < 50; i++ {
		response := suite.ehc.CheckWithMode(ctx, ModeDeep)
		AssertEnterpriseResponseStructure(suite.T(), response)
		suite.Equal(StatusHealthy, response.Status)
	}
	duration := time.Since(start)
	
	avgDuration := duration / 50
	suite.Less(avgDuration, 50*time.Millisecond, "Average enterprise health check should be under 50ms")
}

// TestBenchmarkSuite_RunAll runs the benchmark suite
func TestBenchmarkSuite_RunAll(t *testing.T) {
	suite.Run(t, new(BenchmarkSuite))
}

// Add missing imports
import (
	"context"
	"fmt"
	"time"
)