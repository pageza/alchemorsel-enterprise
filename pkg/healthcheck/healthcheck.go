// Package healthcheck provides health and readiness check functionality
// Following the Health Check API pattern for cloud-native applications
package healthcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Status represents the health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDegraded  Status = "degraded"
)

// Check represents a health check
type Check struct {
	Name        string        `json:"name"`
	Status      Status        `json:"status"`
	Message     string        `json:"message,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
	Duration    time.Duration `json:"duration_ms"`
	Metadata    interface{}   `json:"metadata,omitempty"`
}

// Response represents the health check response
type Response struct {
	Status        Status        `json:"status"`
	Version       string        `json:"version"`
	Timestamp     time.Time     `json:"timestamp"`
	Checks        []Check       `json:"checks"`
	TotalDuration time.Duration `json:"total_duration_ms"`
}

// Checker defines the interface for health checks
type Checker interface {
	Check(ctx context.Context) Check
}

// HealthCheck manages health checks
type HealthCheck struct {
	version  string
	checkers map[string]Checker
	logger   *zap.Logger
	mu       sync.RWMutex
	cache    *Response
	cacheTTL time.Duration
}

// New creates a new health check instance
func New(version string, logger *zap.Logger) *HealthCheck {
	return &HealthCheck{
		version:  version,
		checkers: make(map[string]Checker),
		logger:   logger,
		cacheTTL: 5 * time.Second,
	}
}

// Register registers a health checker
func (h *HealthCheck) Register(name string, checker Checker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkers[name] = checker
}

// SetCacheTTL sets the cache TTL for health check responses
func (h *HealthCheck) SetCacheTTL(ttl time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cacheTTL = ttl
}

// Handler returns the HTTP handler for health checks
func (h *HealthCheck) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		response := h.Check(c.Request.Context())

		// Determine HTTP status code
		statusCode := http.StatusOK
		if response.Status == StatusUnhealthy {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, response)
	}
}

// LivenessHandler returns the HTTP handler for liveness checks
func (h *HealthCheck) LivenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Simple liveness check - if the handler responds, the service is alive
		c.JSON(http.StatusOK, gin.H{
			"status":    "alive",
			"timestamp": time.Now(),
		})
	}
}

// ReadinessHandler returns the HTTP handler for readiness checks
func (h *HealthCheck) ReadinessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		response := h.Check(c.Request.Context())

		// Service is ready only if all checks pass
		if response.Status != StatusHealthy {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"reason": "Health checks failed",
				"checks": response.Checks,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "ready",
			"timestamp": time.Now(),
		})
	}
}

// Check performs all health checks
func (h *HealthCheck) Check(ctx context.Context) Response {
	h.mu.RLock()
	// Check cache
	if h.cache != nil && time.Since(h.cache.Timestamp) < h.cacheTTL {
		cached := *h.cache
		h.mu.RUnlock()
		return cached
	}
	h.mu.RUnlock()

	start := time.Now()
	response := Response{
		Version:   h.version,
		Timestamp: start,
		Status:    StatusHealthy,
		Checks:    []Check{},
	}

	// Create context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Run checks concurrently
	var wg sync.WaitGroup
	checksChan := make(chan Check, len(h.checkers))

	h.mu.RLock()
	for name, checker := range h.checkers {
		wg.Add(1)
		go func(n string, c Checker) {
			defer wg.Done()
			check := c.Check(checkCtx)
			check.Name = n
			checksChan <- check
		}(name, checker)
	}
	h.mu.RUnlock()

	// Wait for all checks to complete
	go func() {
		wg.Wait()
		close(checksChan)
	}()

	// Collect results
	for check := range checksChan {
		response.Checks = append(response.Checks, check)

		// Update overall status
		if check.Status == StatusUnhealthy {
			response.Status = StatusUnhealthy
		} else if check.Status == StatusDegraded && response.Status == StatusHealthy {
			response.Status = StatusDegraded
		}
	}

	response.TotalDuration = time.Since(start)

	// Update cache
	h.mu.Lock()
	h.cache = &response
	h.mu.Unlock()

	return response
}

// DatabaseChecker checks database health
type DatabaseChecker struct {
	pool *pgxpool.Pool
}

// NewDatabaseChecker creates a new database checker
func NewDatabaseChecker(pool *pgxpool.Pool) *DatabaseChecker {
	return &DatabaseChecker{pool: pool}
}

// Check performs database health check
func (d *DatabaseChecker) Check(ctx context.Context) Check {
	start := time.Now()
	check := Check{
		Name:        "database",
		LastChecked: start,
	}

	// Perform ping
	err := d.pool.Ping(ctx)
	check.Duration = time.Since(start)

	if err != nil {
		check.Status = StatusUnhealthy
		check.Message = err.Error()
		return check
	}

	// Get pool stats
	stats := d.pool.Stat()
	check.Status = StatusHealthy
	check.Metadata = map[string]interface{}{
		"total_conns":    stats.TotalConns(),
		"idle_conns":     stats.IdleConns(),
		"acquired_conns": stats.AcquiredConns(),
		"max_conns":      stats.MaxConns(),
	}

	// Check connection pool health
	utilizationPercent := float64(stats.AcquiredConns()) / float64(stats.MaxConns()) * 100
	if utilizationPercent > 90 {
		check.Status = StatusDegraded
		check.Message = "High connection pool utilization"
	}

	return check
}

// RedisChecker checks Redis health
type RedisChecker struct {
	client *redis.Client
}

// NewRedisChecker creates a new Redis checker
func NewRedisChecker(client *redis.Client) *RedisChecker {
	return &RedisChecker{client: client}
}

// Check performs Redis health check
func (r *RedisChecker) Check(ctx context.Context) Check {
	start := time.Now()
	check := Check{
		Name:        "redis",
		LastChecked: start,
	}

	// Perform ping
	pong, err := r.client.Ping(ctx).Result()
	check.Duration = time.Since(start)

	if err != nil {
		check.Status = StatusUnhealthy
		check.Message = err.Error()
		return check
	}

	if pong != "PONG" {
		check.Status = StatusUnhealthy
		check.Message = "Unexpected ping response"
		return check
	}

	// Get Redis info
	info, err := r.client.Info(ctx, "server", "clients", "memory").Result()
	if err == nil {
		// Parse and add relevant metrics
		check.Metadata = map[string]interface{}{
			"info": info, // In production, parse this into structured data
		}
	}

	check.Status = StatusHealthy
	return check
}

// DiskChecker checks disk space
type DiskChecker struct {
	path      string
	threshold float64 // percentage threshold for degraded status
}

// NewDiskChecker creates a new disk checker
func NewDiskChecker(path string, threshold float64) *DiskChecker {
	return &DiskChecker{
		path:      path,
		threshold: threshold,
	}
}

// Check performs disk space check
func (d *DiskChecker) Check(ctx context.Context) Check {
	start := time.Now()
	check := Check{
		Name:        "disk",
		LastChecked: start,
	}

	// TODO: Implement actual disk space check
	// This would use syscall.Statfs on Unix systems

	check.Status = StatusHealthy
	check.Duration = time.Since(start)
	check.Metadata = map[string]interface{}{
		"path":      d.path,
		"threshold": d.threshold,
	}

	return check
}

// ExternalServiceChecker checks external service health
type ExternalServiceChecker struct {
	name    string
	url     string
	timeout time.Duration
	client  *http.Client
}

// NewExternalServiceChecker creates a new external service checker
func NewExternalServiceChecker(name, url string, timeout time.Duration) *ExternalServiceChecker {
	return &ExternalServiceChecker{
		name:    name,
		url:     url,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Check performs external service health check
func (e *ExternalServiceChecker) Check(ctx context.Context) Check {
	start := time.Now()
	check := Check{
		Name:        e.name,
		LastChecked: start,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", e.url, nil)
	if err != nil {
		check.Status = StatusUnhealthy
		check.Message = err.Error()
		check.Duration = time.Since(start)
		return check
	}

	// Perform request
	resp, err := e.client.Do(req)
	check.Duration = time.Since(start)

	if err != nil {
		check.Status = StatusUnhealthy
		check.Message = err.Error()
		return check
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		check.Status = StatusHealthy
	} else if resp.StatusCode >= 500 {
		check.Status = StatusUnhealthy
		check.Message = "Service returned error status"
	} else {
		check.Status = StatusDegraded
		check.Message = "Service returned non-success status"
	}

	check.Metadata = map[string]interface{}{
		"status_code": resp.StatusCode,
		"url":         e.url,
	}

	return check
}

// CustomChecker allows for custom health check logic
type CustomChecker struct {
	name  string
	check func(ctx context.Context) (Status, string, interface{})
}

// NewCustomChecker creates a new custom checker
func NewCustomChecker(name string, check func(ctx context.Context) (Status, string, interface{})) *CustomChecker {
	return &CustomChecker{
		name:  name,
		check: check,
	}
}

// Check performs custom health check
func (c *CustomChecker) Check(ctx context.Context) Check {
	start := time.Now()

	status, message, metadata := c.check(ctx)

	return Check{
		Name:        c.name,
		Status:      status,
		Message:     message,
		Metadata:    metadata,
		LastChecked: start,
		Duration:    time.Since(start),
	}
}

// MarshalJSON customizes JSON marshaling for duration
func (c Check) MarshalJSON() ([]byte, error) {
	type Alias Check
	return json.Marshal(&struct {
		Duration float64 `json:"duration_ms"`
		*Alias
	}{
		Duration: float64(c.Duration.Milliseconds()),
		Alias:    (*Alias)(&c),
	})
}

// MarshalJSON customizes JSON marshaling for response
func (r Response) MarshalJSON() ([]byte, error) {
	type Alias Response
	return json.Marshal(&struct {
		TotalDuration float64 `json:"total_duration_ms"`
		*Alias
	}{
		TotalDuration: float64(r.TotalDuration.Milliseconds()),
		Alias:         (*Alias)(&r),
	})
}
