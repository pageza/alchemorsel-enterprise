// Package healthcheck unit tests
// Tests for basic health check functionality
package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	version := "1.0.0"

	hc := New(version, logger)

	assert.NotNil(t, hc)
	assert.Equal(t, version, hc.version)
	assert.Equal(t, logger, hc.logger)
	assert.NotNil(t, hc.checkers)
	assert.Equal(t, 5*time.Second, hc.cacheTTL)
}

func TestHealthCheck_Register(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("test")

	hc.Register("test", checker)

	assert.Len(t, hc.checkers, 1)
	assert.Contains(t, hc.checkers, "test")
}

func TestHealthCheck_SetCacheTTL(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ttl := 10 * time.Second

	hc.SetCacheTTL(ttl)

	assert.Equal(t, ttl, hc.cacheTTL)
}

func TestHealthCheck_Check_NoCheckers(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ctx := context.Background()

	response := hc.Check(ctx)

	AssertResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
	assert.Equal(t, "1.0.0", response.Version)
	assert.Empty(t, response.Checks)
}

func TestHealthCheck_Check_SingleHealthyChecker(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ctx := context.Background()
	
	checker := NewMockChecker("database").WithStatus(StatusHealthy).WithMessage("Connection OK")
	hc.Register("database", checker)

	response := hc.Check(ctx)

	AssertResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
	assert.Len(t, response.Checks, 1)
	
	check := response.Checks[0]
	AssertCheckResult(t, check, StatusHealthy, "database")
	assert.Equal(t, "Connection OK", check.Message)
}

func TestHealthCheck_Check_SingleUnhealthyChecker(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ctx := context.Background()
	
	checker := NewMockChecker("database").WithStatus(StatusUnhealthy).WithMessage("Connection failed")
	hc.Register("database", checker)

	response := hc.Check(ctx)

	AssertResponseStructure(t, response)
	assert.Equal(t, StatusUnhealthy, response.Status)
	assert.Len(t, response.Checks, 1)
	
	check := response.Checks[0]
	AssertCheckResult(t, check, StatusUnhealthy, "database")
	assert.Equal(t, "Connection failed", check.Message)
}

func TestHealthCheck_Check_SingleDegradedChecker(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ctx := context.Background()
	
	checker := NewMockChecker("cache").WithStatus(StatusDegraded).WithMessage("High latency")
	hc.Register("cache", checker)

	response := hc.Check(ctx)

	AssertResponseStructure(t, response)
	assert.Equal(t, StatusDegraded, response.Status)
	assert.Len(t, response.Checks, 1)
	
	check := response.Checks[0]
	AssertCheckResult(t, check, StatusDegraded, "cache")
	assert.Equal(t, "High latency", check.Message)
}

func TestHealthCheck_Check_MultipleCheckers(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ctx := context.Background()
	
	// Register multiple checkers with different statuses
	hc.Register("database", NewMockChecker("database").WithStatus(StatusHealthy))
	hc.Register("cache", NewMockChecker("cache").WithStatus(StatusDegraded))
	hc.Register("service", NewMockChecker("service").WithStatus(StatusHealthy))

	response := hc.Check(ctx)

	AssertResponseStructure(t, response)
	assert.Equal(t, StatusDegraded, response.Status) // Should be degraded due to cache
	assert.Len(t, response.Checks, 3)
}

func TestHealthCheck_Check_MultipleCheckersWithUnhealthy(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ctx := context.Background()
	
	// Register multiple checkers with one unhealthy
	hc.Register("database", NewMockChecker("database").WithStatus(StatusUnhealthy))
	hc.Register("cache", NewMockChecker("cache").WithStatus(StatusDegraded))
	hc.Register("service", NewMockChecker("service").WithStatus(StatusHealthy))

	response := hc.Check(ctx)

	AssertResponseStructure(t, response)
	assert.Equal(t, StatusUnhealthy, response.Status) // Should be unhealthy due to database
	assert.Len(t, response.Checks, 3)
}

func TestHealthCheck_Check_ConcurrentExecution(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ctx := context.Background()
	
	// Register checkers with delays to test concurrency
	delay := 50 * time.Millisecond
	hc.Register("slow1", NewMockChecker("slow1").WithDelay(delay))
	hc.Register("slow2", NewMockChecker("slow2").WithDelay(delay))
	hc.Register("slow3", NewMockChecker("slow3").WithDelay(delay))

	start := time.Now()
	response := hc.Check(ctx)
	elapsed := time.Since(start)

	AssertResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
	assert.Len(t, response.Checks, 3)
	
	// Should complete in roughly the delay time (concurrent) rather than 3x delay (sequential)
	assert.Less(t, elapsed, 2*delay, "Checks should run concurrently")
}

func TestHealthCheck_Check_ContextTimeout(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	
	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	
	// Register slow checker
	hc.Register("slow", NewSlowChecker("slow", 100*time.Millisecond))

	response := hc.Check(ctx)

	AssertResponseStructure(t, response)
	// The check should handle timeout gracefully
	assert.Len(t, response.Checks, 1)
}

func TestHealthCheck_Check_WithMetadata(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	ctx := context.Background()
	
	metadata := map[string]interface{}{
		"connections": 10,
		"version":     "5.7",
	}
	
	checker := NewMockChecker("database").
		WithStatus(StatusHealthy).
		WithMetadata(metadata)
	hc.Register("database", checker)

	response := hc.Check(ctx)

	AssertResponseStructure(t, response)
	assert.Len(t, response.Checks, 1)
	
	check := response.Checks[0]
	assert.Equal(t, metadata, check.Metadata)
}

func TestHealthCheck_Check_Caching(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	hc.SetCacheTTL(100 * time.Millisecond)
	
	ctx := context.Background()
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	// First call
	response1 := hc.Check(ctx)
	timestamp1 := response1.Timestamp

	// Second call immediately - should be cached
	response2 := hc.Check(ctx)
	timestamp2 := response2.Timestamp

	assert.Equal(t, timestamp1, timestamp2, "Response should be cached")
	assert.Equal(t, 1, checker.GetCallCount(), "Checker should only be called once")

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third call - should not be cached
	response3 := hc.Check(ctx)
	timestamp3 := response3.Timestamp

	assert.NotEqual(t, timestamp1, timestamp3, "Response should not be cached after TTL")
	assert.Equal(t, 2, checker.GetCallCount(), "Checker should be called again after cache expiry")
}

func TestHealthCheck_Handler(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", hc.Handler())

	// Test healthy response
	req, _ := http.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response Response
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	
	AssertResponseStructure(t, response)
	assert.Equal(t, StatusHealthy, response.Status)
}

func TestHealthCheck_Handler_Unhealthy(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusUnhealthy)
	hc.Register("test", checker)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", hc.Handler())

	// Test unhealthy response
	req, _ := http.NewRequest("GET", "/health", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	
	var response Response
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	
	AssertResponseStructure(t, response)
	assert.Equal(t, StatusUnhealthy, response.Status)
}

func TestHealthCheck_LivenessHandler(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/liveness", hc.LivenessHandler())

	// Test liveness response
	req, _ := http.NewRequest("GET", "/liveness", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "alive", response["status"])
	assert.Contains(t, response, "timestamp")
}

func TestHealthCheck_ReadinessHandler_Ready(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/readiness", hc.ReadinessHandler())

	// Test ready response
	req, _ := http.NewRequest("GET", "/readiness", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "ready", response["status"])
	assert.Contains(t, response, "timestamp")
}

func TestHealthCheck_ReadinessHandler_NotReady(t *testing.T) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusUnhealthy)
	hc.Register("test", checker)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/readiness", hc.ReadinessHandler())

	// Test not ready response
	req, _ := http.NewRequest("GET", "/readiness", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusServiceUnavailable, resp.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "not_ready", response["status"])
	assert.Equal(t, "Health checks failed", response["reason"])
	assert.Contains(t, response, "checks")
}

func TestCheck_MarshalJSON(t *testing.T) {
	check := Check{
		Name:        "test",
		Status:      StatusHealthy,
		Message:     "OK",
		LastChecked: time.Now(),
		Duration:    100 * time.Millisecond,
		Metadata:    map[string]interface{}{"version": "1.0"},
	}

	data, err := json.Marshal(check)
	require.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "test", unmarshaled["name"])
	assert.Equal(t, "healthy", unmarshaled["status"])
	assert.Equal(t, "OK", unmarshaled["message"])
	assert.Equal(t, float64(100), unmarshaled["duration_ms"])
	assert.Contains(t, unmarshaled, "last_checked")
	assert.Contains(t, unmarshaled, "metadata")
}

func TestResponse_MarshalJSON(t *testing.T) {
	response := Response{
		Status:        StatusHealthy,
		Version:       "1.0.0",
		Timestamp:     time.Now(),
		TotalDuration: 250 * time.Millisecond,
		Checks: []Check{
			{
				Name:        "test",
				Status:      StatusHealthy,
				Duration:    100 * time.Millisecond,
				LastChecked: time.Now(),
			},
		},
	}

	data, err := json.Marshal(response)
	require.NoError(t, err)

	var unmarshaled map[string]interface{}
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, "healthy", unmarshaled["status"])
	assert.Equal(t, "1.0.0", unmarshaled["version"])
	assert.Equal(t, float64(250), unmarshaled["total_duration_ms"])
	assert.Contains(t, unmarshaled, "timestamp")
	assert.Contains(t, unmarshaled, "checks")
}

// Benchmark tests
func BenchmarkHealthCheck_Check_SingleChecker(b *testing.B) {
	hc := New("1.0.0", zap.NewNop())
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.Check(ctx)
	}
}

func BenchmarkHealthCheck_Check_MultipleCheckers(b *testing.B) {
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

func BenchmarkHealthCheck_Check_WithCaching(b *testing.B) {
	hc := New("1.0.0", zap.NewNop())
	hc.SetCacheTTL(1 * time.Hour) // Long cache for benchmarking
	
	checker := NewMockChecker("test").WithStatus(StatusHealthy)
	hc.Register("test", checker)
	
	ctx := context.Background()
	
	// Prime the cache
	hc.Check(ctx)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hc.Check(ctx)
	}
}