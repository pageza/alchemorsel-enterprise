// Package healthcheck test helpers
// Provides common utilities and helpers for health check testing
package healthcheck

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// TestHealthCheckHelper provides utilities for health check testing
type TestHealthCheckHelper struct {
	t             *testing.T
	logger        *zap.Logger
	postgresContainer testcontainers.Container
	redisContainer    testcontainers.Container
	pgPool        *pgxpool.Pool
	redisClient   *redis.Client
	mu            sync.Mutex
}

// NewTestHealthCheckHelper creates a new test helper
func NewTestHealthCheckHelper(t *testing.T) *TestHealthCheckHelper {
	helper := &TestHealthCheckHelper{
		t:      t,
		logger: zap.NewNop(), // Silent logger for tests
	}

	// Setup cleanup
	t.Cleanup(func() {
		helper.Cleanup()
	})

	return helper
}

// SetupPostgreSQL starts a PostgreSQL container for testing
func (h *TestHealthCheckHelper) SetupPostgreSQL() *pgxpool.Pool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pgPool != nil {
		return h.pgPool
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgres, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:15-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_DB":       "healthcheck_test",
				"POSTGRES_USER":     "test_user",
				"POSTGRES_PASSWORD": "test_password",
			},
			WaitingFor: wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
				wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
					return fmt.Sprintf("postgres://test_user:test_password@%s:%s/healthcheck_test?sslmode=disable",
						host, port.Port())
				}),
			),
			Tmpfs: map[string]string{
				"/var/lib/postgresql/data": "rw,noexec,nosuid,size=512m",
			},
		},
		Started: true,
	})
	require.NoError(h.t, err, "Failed to start PostgreSQL container")

	h.postgresContainer = postgres

	// Get connection details
	host, err := postgres.Host(ctx)
	require.NoError(h.t, err)

	port, err := postgres.MappedPort(ctx, "5432")
	require.NoError(h.t, err)

	dsn := fmt.Sprintf("postgres://test_user:test_password@%s:%s/healthcheck_test?sslmode=disable",
		host, port.Port())

	// Create connection pool
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(h.t, err)

	config.MaxConns = 5
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(h.t, err)

	// Verify connection
	err = pool.Ping(ctx)
	require.NoError(h.t, err)

	h.pgPool = pool
	return pool
}

// SetupRedis starts a Redis container for testing
func (h *TestHealthCheckHelper) SetupRedis() *redis.Client {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.redisClient != nil {
		return h.redisClient
	}

	ctx := context.Background()

	// Start Redis container
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForLog("Ready to accept connections").
					WithStartupTimeout(30*time.Second),
				wait.ForListeningPort("6379/tcp"),
			),
		},
		Started: true,
	})
	require.NoError(h.t, err, "Failed to start Redis container")

	h.redisContainer = redisContainer

	// Get connection details
	host, err := redisContainer.Host(ctx)
	require.NoError(h.t, err)

	port, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(h.t, err)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port.Port()),
		Password: "",
		DB:       0,
	})

	// Verify connection
	pong, err := client.Ping(ctx).Result()
	require.NoError(h.t, err)
	require.Equal(h.t, "PONG", pong)

	h.redisClient = client
	return client
}

// Cleanup stops all containers and closes connections
func (h *TestHealthCheckHelper) Cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	ctx := context.Background()

	if h.pgPool != nil {
		h.pgPool.Close()
		h.pgPool = nil
	}

	if h.redisClient != nil {
		h.redisClient.Close()
		h.redisClient = nil
	}

	if h.postgresContainer != nil {
		if err := h.postgresContainer.Terminate(ctx); err != nil {
			h.t.Logf("Failed to terminate PostgreSQL container: %v", err)
		}
		h.postgresContainer = nil
	}

	if h.redisContainer != nil {
		if err := h.redisContainer.Terminate(ctx); err != nil {
			h.t.Logf("Failed to terminate Redis container: %v", err)
		}
		h.redisContainer = nil
	}
}

// MockChecker provides a configurable mock checker for testing
type MockChecker struct {
	name     string
	status   Status
	message  string
	duration time.Duration
	metadata interface{}
	delay    time.Duration
	err      error
	callCount int
	mu       sync.Mutex
}

// NewMockChecker creates a new mock checker
func NewMockChecker(name string) *MockChecker {
	return &MockChecker{
		name:   name,
		status: StatusHealthy,
	}
}

// WithStatus sets the status to return
func (m *MockChecker) WithStatus(status Status) *MockChecker {
	m.status = status
	return m
}

// WithMessage sets the message to return
func (m *MockChecker) WithMessage(message string) *MockChecker {
	m.message = message
	return m
}

// WithDuration sets the duration to return
func (m *MockChecker) WithDuration(duration time.Duration) *MockChecker {
	m.duration = duration
	return m
}

// WithMetadata sets the metadata to return
func (m *MockChecker) WithMetadata(metadata interface{}) *MockChecker {
	m.metadata = metadata
	return m
}

// WithDelay sets a delay before returning the check result
func (m *MockChecker) WithDelay(delay time.Duration) *MockChecker {
	m.delay = delay
	return m
}

// WithError sets an error condition
func (m *MockChecker) WithError(err error) *MockChecker {
	m.err = err
	return m
}

// Check implements the Checker interface
func (m *MockChecker) Check(ctx context.Context) Check {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++
	start := time.Now()

	// Simulate delay if configured
	if m.delay > 0 {
		timer := time.NewTimer(m.delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Delay completed
		case <-ctx.Done():
			// Context cancelled
			return Check{
				Name:        m.name,
				Status:      StatusUnhealthy,
				Message:     "Context cancelled",
				LastChecked: start,
				Duration:    time.Since(start),
			}
		}
	}

	// Return error condition if configured
	if m.err != nil {
		return Check{
			Name:        m.name,
			Status:      StatusUnhealthy,
			Message:     m.err.Error(),
			LastChecked: start,
			Duration:    time.Since(start),
		}
	}

	return Check{
		Name:        m.name,
		Status:      m.status,
		Message:     m.message,
		LastChecked: start,
		Duration:    m.duration,
		Metadata:    m.metadata,
	}
}

// GetCallCount returns the number of times Check was called
func (m *MockChecker) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ResetCallCount resets the call counter
func (m *MockChecker) ResetCallCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
}

// FailingChecker provides a checker that always fails
type FailingChecker struct {
	name    string
	message string
}

// NewFailingChecker creates a new failing checker
func NewFailingChecker(name, message string) *FailingChecker {
	return &FailingChecker{
		name:    name,
		message: message,
	}
}

// Check implements the Checker interface
func (f *FailingChecker) Check(ctx context.Context) Check {
	return Check{
		Name:        f.name,
		Status:      StatusUnhealthy,
		Message:     f.message,
		LastChecked: time.Now(),
		Duration:    time.Millisecond,
	}
}

// SlowChecker provides a checker that takes a specified amount of time
type SlowChecker struct {
	name     string
	duration time.Duration
}

// NewSlowChecker creates a new slow checker
func NewSlowChecker(name string, duration time.Duration) *SlowChecker {
	return &SlowChecker{
		name:     name,
		duration: duration,
	}
}

// Check implements the Checker interface
func (s *SlowChecker) Check(ctx context.Context) Check {
	start := time.Now()

	timer := time.NewTimer(s.duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return Check{
			Name:        s.name,
			Status:      StatusHealthy,
			Message:     "Slow check completed",
			LastChecked: start,
			Duration:    time.Since(start),
		}
	case <-ctx.Done():
		return Check{
			Name:        s.name,
			Status:      StatusUnhealthy,
			Message:     "Check timed out",
			LastChecked: start,
			Duration:    time.Since(start),
		}
	}
}

// UnreachableServiceChecker simulates an unreachable external service
type UnreachableServiceChecker struct {
	name string
}

// NewUnreachableServiceChecker creates a new unreachable service checker
func NewUnreachableServiceChecker(name string) *UnreachableServiceChecker {
	return &UnreachableServiceChecker{name: name}
}

// Check implements the Checker interface
func (u *UnreachableServiceChecker) Check(ctx context.Context) Check {
	start := time.Now()

	// Try to connect to a non-existent address
	conn, err := net.DialTimeout("tcp", "192.0.2.1:80", 1*time.Second)
	if conn != nil {
		conn.Close()
	}

	return Check{
		Name:        u.name,
		Status:      StatusUnhealthy,
		Message:     fmt.Sprintf("Service unreachable: %v", err),
		LastChecked: start,
		Duration:    time.Since(start),
	}
}

// TestCircuitBreakerConfig provides test configuration for circuit breakers
func TestCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond, // Short timeout for tests
		MaxRequests:      2,
	}
}

// TestMetricsConfig provides test configuration for metrics
func TestMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Namespace: "test",
		Subsystem: "healthcheck",
		Enabled:   true,
	}
}

// WaitForCircuitBreakerState waits for a circuit breaker to reach a specific state
func WaitForCircuitBreakerState(t *testing.T, cb *CircuitBreaker, expectedState CircuitBreakerState, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		if cb.GetState() == expectedState {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	require.Failf(t, "Circuit breaker state timeout", 
		"Expected state %s, got %s", expectedState, cb.GetState())
}

// AssertDependencyOrder verifies that dependencies are returned in topological order
func AssertDependencyOrder(t *testing.T, dependencies []DependencyStatus, expectedOrder []string) {
	require.Len(t, dependencies, len(expectedOrder), "Dependency count mismatch")
	
	for i, expected := range expectedOrder {
		require.Equal(t, expected, dependencies[i].Name, 
			"Dependency order mismatch at position %d", i)
	}
}

// CreateTestDependency creates a test dependency with specified characteristics
func CreateTestDependency(name string, depType DependencyType, critical bool, deps []string, checker Checker) Dependency {
	return NewBasicDependency(name, depType, critical, deps, checker)
}

// AssertCheckResult validates a health check result
func AssertCheckResult(t *testing.T, check Check, expectedStatus Status, expectedName string) {
	require.Equal(t, expectedName, check.Name, "Check name mismatch")
	require.Equal(t, expectedStatus, check.Status, "Check status mismatch")
	require.NotZero(t, check.LastChecked, "LastChecked should be set")
	require.True(t, check.Duration >= 0, "Duration should be non-negative")
}

// AssertResponseStructure validates the structure of a health check response
func AssertResponseStructure(t *testing.T, response Response) {
	require.NotEmpty(t, response.Version, "Version should not be empty")
	require.NotZero(t, response.Timestamp, "Timestamp should be set")
	require.Contains(t, []Status{StatusHealthy, StatusDegraded, StatusUnhealthy}, 
		response.Status, "Status should be valid")
	require.True(t, response.TotalDuration >= 0, "TotalDuration should be non-negative")
	
	for _, check := range response.Checks {
		AssertCheckResult(t, check, check.Status, check.Name)
	}
}

// AssertEnterpriseResponseStructure validates the structure of an enterprise health check response
func AssertEnterpriseResponseStructure(t *testing.T, response EnterpriseResponse) {
	AssertResponseStructure(t, response.Response)
	
	require.NotEmpty(t, response.SystemInfo.Hostname, "Hostname should not be empty")
	require.NotEmpty(t, response.SystemInfo.Platform, "Platform should not be empty")
	require.NotEmpty(t, response.SystemInfo.Architecture, "Architecture should not be empty")
	require.True(t, response.SystemInfo.CPUCores > 0, "CPU cores should be positive")
	require.True(t, response.SystemInfo.Memory.Total > 0, "Memory total should be positive")
}