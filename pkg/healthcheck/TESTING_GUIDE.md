# Health Check System Testing Guide

## Overview

This guide provides comprehensive information about testing the Alchemorsel v3 health check system. The test suite is designed to ensure robust, reliable, and performant health checking capabilities for enterprise applications.

## Test Architecture

### Test Organization

The health check testing framework is organized into several layers:

1. **Unit Tests** - Test individual components in isolation
2. **Integration Tests** - Test with real PostgreSQL and Redis per ADR-0012
3. **Performance Tests** - Ensure health checks meet performance requirements
4. **End-to-End Tests** - Test complete workflows and scenarios

### Test Files Structure

```
pkg/healthcheck/
├── healthcheck_test.go        # Core health check unit tests
├── enterprise_test.go         # Enterprise feature tests
├── circuit_test.go           # Circuit breaker tests
├── dependencies_test.go      # Dependency management tests
├── metrics_test.go          # Prometheus metrics tests
├── integration_test.go      # Real service integration tests
├── performance_test.go      # Performance and benchmark tests
├── edge_cases_test.go       # Edge cases and error scenarios
├── suite_test.go           # Comprehensive test suites
└── test_helpers.go         # Testing utilities and helpers
```

## Testing Frameworks Used

### Primary Framework
- **Go Standard Testing + Testify** - Comprehensive assertion library
- **Testify Suite** - Organized test suites with setup/teardown

### Integration Testing
- **Testcontainers Go** - Real PostgreSQL and Redis testing
- **Docker Containers** - Isolated test environments

### Performance Testing
- **Go Benchmarks** - Built-in performance measurement
- **Custom Performance Tests** - Throughput and latency validation

### Mocking
- **Custom Mock Implementation** - Tailored health check mocks
- **Configurable Behavior** - Supports various test scenarios

## Test Categories

### 1. Unit Tests

**Coverage Areas:**
- Basic health check functionality
- Enterprise health check features
- Circuit breaker state management
- Dependency graph operations
- Metrics collection
- JSON serialization
- Error handling

**Key Test Files:**
- `healthcheck_test.go` - Core functionality
- `enterprise_test.go` - Enterprise features
- `circuit_test.go` - Circuit breaker logic
- `dependencies_test.go` - Dependency management

### 2. Integration Tests

**Real Service Testing (per ADR-0012):**
- PostgreSQL connection and health checks
- Redis connection and health checks
- External service health checks
- Database failure recovery
- Connection pool monitoring

**Key Features:**
- Uses real PostgreSQL and Redis containers
- Tests actual network connectivity
- Validates real-world scenarios
- Tests failure and recovery patterns

### 3. Performance Tests

**Performance Requirements:**
- Single health check: < 1 second
- Enterprise health check: < 2 seconds
- Concurrent health checks: < 3 seconds
- Minimum throughput: 100 checks/second
- Maximum memory allocation: 10 MB

**Test Scenarios:**
- Single checker performance
- Multiple checker concurrency
- Enterprise mode performance
- Circuit breaker overhead
- Dependency graph traversal
- Metrics collection overhead
- Memory allocation patterns
- Cache effectiveness

### 4. Circuit Breaker Tests

**Scenarios Tested:**
- Failure threshold triggering
- Circuit state transitions (Closed → Open → Half-Open → Closed)
- Recovery behavior
- Timeout configurations
- Concurrent access during state changes
- Error counting and reset logic

### 5. Dependency Tests

**Functionality Tested:**
- Dependency registration
- Topological sorting
- Circular dependency detection
- Critical vs non-critical dependencies
- Dependency status propagation
- Complex dependency chains

### 6. Metrics Tests

**Prometheus Integration:**
- Counter metrics (checks, errors, circuit trips)
- Histogram metrics (check duration)
- Gauge metrics (health status, dependency status)
- Summary metrics (duration percentiles)
- Custom metrics registration
- Metrics middleware functionality

## Running Tests

### Prerequisites

Before running tests, ensure you have:

1. **Go 1.21+** (required for dependency compatibility)
2. **Docker** (for integration tests with testcontainers)
3. **PostgreSQL and Redis** (via testcontainers)

### Environment Setup

```bash
# Update Go version to 1.21+
go mod edit -go=1.21
go mod tidy

# Ensure Docker is running
docker --version
```

### Test Execution Commands

#### All Tests
```bash
# Run all tests with coverage
go test -v -coverprofile=coverage.out ./pkg/healthcheck/

# View coverage report
go tool cover -html=coverage.out
```

#### Unit Tests Only
```bash
# Run unit tests (fast)
go test -v -short ./pkg/healthcheck/
```

#### Integration Tests Only
```bash
# Run integration tests (requires Docker)
go test -v -run="TestIntegration" ./pkg/healthcheck/
```

#### Performance Tests
```bash
# Run performance tests
go test -v -run="TestPerformance" ./pkg/healthcheck/

# Run benchmarks
go test -bench=. -benchmem ./pkg/healthcheck/
```

#### Specific Test Suites
```bash
# Run comprehensive test suite
go test -v -run="TestSuite_RunAll" ./pkg/healthcheck/

# Run integration test suite
go test -v -run="TestIntegrationSuite_RunAll" ./pkg/healthcheck/

# Run benchmark suite
go test -v -run="TestBenchmarkSuite_RunAll" ./pkg/healthcheck/
```

### Test Configuration

#### Short Mode
Use `-short` flag to skip integration tests:
```bash
go test -short ./pkg/healthcheck/
```

#### Verbose Output
Use `-v` flag for detailed test output:
```bash
go test -v ./pkg/healthcheck/
```

#### Coverage Analysis
```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./pkg/healthcheck/

# View coverage in browser
go tool cover -html=coverage.out

# Get coverage percentage
go tool cover -func=coverage.out
```

## Test Helpers and Utilities

### TestHealthCheckHelper

Central helper for test setup and utilities:

```go
helper := NewTestHealthCheckHelper(t)

// Setup real services
pgPool := helper.SetupPostgreSQL()
redisClient := helper.SetupRedis()

// Automatic cleanup on test completion
// helper.Cleanup() called automatically
```

### Mock Checkers

Configurable mock implementations:

```go
// Basic mock
checker := NewMockChecker("service").WithStatus(StatusHealthy)

// Advanced configuration
checker := NewMockChecker("service").
    WithStatus(StatusDegraded).
    WithMessage("High latency").
    WithDelay(100 * time.Millisecond).
    WithMetadata(map[string]interface{}{"latency": "100ms"})
```

### Performance Testing Helpers

```go
// Circuit breaker configuration for tests
config := TestCircuitBreakerConfig()

// Metrics configuration for tests
metricsConfig := TestMetricsConfig()

// Wait for circuit breaker state
WaitForCircuitBreakerState(t, circuitBreaker, StateOpen, 1*time.Second)
```

## Test Data and Fixtures

### Test Configurations

```go
// Circuit breaker test config
config := CircuitBreakerConfig{
    FailureThreshold: 3,
    SuccessThreshold: 2,
    Timeout:          100 * time.Millisecond,
    MaxRequests:      2,
}

// Metrics test config
metricsConfig := MetricsConfig{
    Namespace: "test",
    Subsystem: "healthcheck",
    Enabled:   true,
}
```

### Test Dependencies

```go
// Database dependency
dbDep := DatabaseDependency("postgres", true, dbChecker)

// Cache dependency with dependencies
cacheDep := CreateTestDependency("redis", DependencyTypeCache, false, []string{"postgres"}, redisChecker)

// Service dependency chain
apiDep := ServiceDependency("api", false, []string{"redis"}, apiChecker)
```

## Assertions and Validations

### Response Structure Validation

```go
// Basic response validation
AssertResponseStructure(t, response)

// Enterprise response validation
AssertEnterpriseResponseStructure(t, response)

// Check result validation
AssertCheckResult(t, check, StatusHealthy, "service_name")
```

### Dependency Order Validation

```go
// Verify topological order
AssertDependencyOrder(t, dependencies, []string{"postgres", "redis", "api"})
```

### Performance Assertions

```go
// Duration assertions
assert.Less(t, duration, MaxHealthCheckDuration)

// Throughput assertions
assert.GreaterOrEqual(t, throughput, float64(TargetThroughput))
```

## Common Test Patterns

### 1. Basic Health Check Test

```go
func TestBasicHealthCheck(t *testing.T) {
    hc := New("1.0.0", zap.NewNop())
    checker := NewMockChecker("test").WithStatus(StatusHealthy)
    hc.Register("test", checker)
    
    response := hc.Check(context.Background())
    
    assert.Equal(t, StatusHealthy, response.Status)
    assert.Len(t, response.Checks, 1)
}
```

### 2. Integration Test with Real Services

```go
func TestDatabaseIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    helper := NewTestHealthCheckHelper(t)
    pgPool := helper.SetupPostgreSQL()
    
    checker := NewDatabaseChecker(pgPool)
    check := checker.Check(context.Background())
    
    assert.Equal(t, StatusHealthy, check.Status)
}
```

### 3. Circuit Breaker Test

```go
func TestCircuitBreakerFailure(t *testing.T) {
    config := TestCircuitBreakerConfig()
    cb := NewCircuitBreaker("test", config)
    
    // Trigger failures
    for i := 0; i < config.FailureThreshold; i++ {
        _, err := cb.Execute(func() (interface{}, error) {
            return nil, errors.New("test failure")
        })
        assert.Error(t, err)
    }
    
    assert.Equal(t, StateOpen, cb.GetState())
}
```

### 4. Performance Test

```go
func TestPerformanceRequirement(t *testing.T) {
    hc := New("1.0.0", zap.NewNop())
    checker := NewMockChecker("fast").WithStatus(StatusHealthy)
    hc.Register("fast", checker)
    
    start := time.Now()
    response := hc.Check(context.Background())
    duration := time.Since(start)
    
    assert.Equal(t, StatusHealthy, response.Status)
    assert.Less(t, duration, MaxHealthCheckDuration)
}
```

## Troubleshooting

### Common Issues

1. **Test Containers Fail to Start**
   - Ensure Docker is running
   - Check Docker permissions
   - Verify available ports

2. **Integration Tests Timeout**
   - Increase test timeouts
   - Check network connectivity
   - Verify container resource limits

3. **Performance Tests Fail**
   - Check system load
   - Verify test environment consistency
   - Review performance thresholds

4. **Coverage Issues**
   - Run tests with `-coverprofile`
   - Check for untested code paths
   - Review test completeness

### Debugging Tips

1. **Enable Verbose Logging**
   ```go
   logger := zap.NewDevelopment()
   hc := New("1.0.0", logger)
   ```

2. **Use Test Helpers**
   ```go
   helper := NewTestHealthCheckHelper(t)
   helper.SetupPostgreSQL() // Provides detailed setup logs
   ```

3. **Check Container Logs**
   ```bash
   docker logs <container_id>
   ```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Health Check Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      redis:
        image: redis:7
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: '1.21'
    
    - name: Run Unit Tests
      run: go test -short -v ./pkg/healthcheck/
    
    - name: Run Integration Tests
      run: go test -v ./pkg/healthcheck/
    
    - name: Run Performance Tests
      run: go test -run="TestPerformance" -v ./pkg/healthcheck/
    
    - name: Generate Coverage
      run: |
        go test -coverprofile=coverage.out ./pkg/healthcheck/
        go tool cover -func=coverage.out
```

### Test Coverage Requirements

- **Target Coverage**: 90%+
- **Minimum Coverage**: 85%
- **Critical Path Coverage**: 100%

### Performance Benchmarks

Regular performance monitoring:

```bash
# Automated benchmark comparison
go test -bench=. -count=5 -benchmem ./pkg/healthcheck/ > current.bench
benchcmp baseline.bench current.bench
```

## Best Practices

### Test Design

1. **Isolation** - Each test should be independent
2. **Repeatability** - Tests should produce consistent results
3. **Fast Execution** - Unit tests should complete quickly
4. **Clear Assertions** - Use descriptive assertion messages
5. **Comprehensive Coverage** - Test happy paths, edge cases, and error conditions

### Mock Usage

1. **Realistic Behavior** - Mocks should simulate real component behavior
2. **Configurable** - Support various test scenarios
3. **Verification** - Verify mock interactions when relevant
4. **Reset State** - Reset mocks between tests

### Integration Testing

1. **Real Services** - Use actual PostgreSQL and Redis per ADR-0012
2. **Container Management** - Proper lifecycle management
3. **Resource Cleanup** - Ensure containers are cleaned up
4. **Network Isolation** - Tests should not interfere with each other

### Performance Testing

1. **Baseline Establishment** - Establish performance baselines
2. **Consistent Environment** - Use consistent test environments
3. **Multiple Iterations** - Run multiple iterations for accuracy
4. **Resource Monitoring** - Monitor memory and CPU usage

## Conclusion

This comprehensive test suite ensures the Alchemorsel v3 health check system meets enterprise reliability and performance requirements. The testing framework provides:

- **Complete Coverage** - All components and scenarios tested
- **Real-World Validation** - Integration with actual services
- **Performance Assurance** - Meets strict performance requirements
- **Maintainability** - Well-organized and documented tests
- **CI/CD Ready** - Integrated with automated testing pipelines

The test suite serves as both validation and documentation of the health check system's capabilities, ensuring robust operation in production environments.