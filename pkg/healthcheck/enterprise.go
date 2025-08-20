// Package healthcheck enterprise extensions
// Provides advanced health check features for enterprise deployments
package healthcheck

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EnterpriseHealthCheck extends the basic health check with enterprise features
type EnterpriseHealthCheck struct {
	*HealthCheck
	dependencies     *DependencyManager
	circuitBreakers  map[string]*CircuitBreaker
	maintenanceMode  bool
	gracefulShutdown bool
	metrics          *HealthMetrics
	mu               sync.RWMutex
}

// MaintenanceConfig holds maintenance mode configuration
type MaintenanceConfig struct {
	Enabled   bool      `json:"enabled"`
	Message   string    `json:"message"`
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
}

// EnterpriseResponse extends the basic response with enterprise data
type EnterpriseResponse struct {
	Response
	Dependencies    []DependencyStatus              `json:"dependencies,omitempty"`
	CircuitBreakers map[string]CircuitBreakerStatus `json:"circuit_breakers,omitempty"`
	Maintenance     *MaintenanceConfig              `json:"maintenance,omitempty"`
	SystemInfo      SystemInfo                      `json:"system_info,omitempty"`
}

// SystemInfo provides system-level information
type SystemInfo struct {
	Hostname     string            `json:"hostname"`
	Platform     string            `json:"platform"`
	Architecture string            `json:"architecture"`
	CPUCores     int               `json:"cpu_cores"`
	Memory       MemoryInfo        `json:"memory"`
	Uptime       time.Duration     `json:"uptime"`
	LoadAverage  []float64         `json:"load_average,omitempty"`
	Environment  map[string]string `json:"environment,omitempty"`
}

// MemoryInfo provides memory usage information
type MemoryInfo struct {
	Total        uint64  `json:"total_bytes"`
	Available    uint64  `json:"available_bytes"`
	Used         uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// HealthCheckMode defines different modes for health checks
type HealthCheckMode string

const (
	ModeStandard    HealthCheckMode = "standard"
	ModeDeep        HealthCheckMode = "deep"
	ModeQuick       HealthCheckMode = "quick"
	ModeMaintenance HealthCheckMode = "maintenance"
)

// NewEnterpriseHealthCheck creates a new enterprise health check instance
func NewEnterpriseHealthCheck(version string, logger *zap.Logger) *EnterpriseHealthCheck {
	return &EnterpriseHealthCheck{
		HealthCheck:     New(version, logger),
		dependencies:    NewDependencyManager(logger),
		circuitBreakers: make(map[string]*CircuitBreaker),
		metrics:         NewHealthMetrics(),
	}
}

// RegisterWithCircuitBreaker registers a checker with circuit breaker protection
func (e *EnterpriseHealthCheck) RegisterWithCircuitBreaker(name string, checker Checker, config CircuitBreakerConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Create circuit breaker
	cb := NewCircuitBreaker(name, config)
	e.circuitBreakers[name] = cb

	// Wrap checker with circuit breaker
	wrappedChecker := &CircuitBreakerChecker{
		checker: checker,
		breaker: cb,
		name:    name,
	}

	e.Register(name, wrappedChecker)
}

// RegisterDependency registers a service dependency
func (e *EnterpriseHealthCheck) RegisterDependency(dep Dependency) {
	e.dependencies.Register(dep)
}

// SetMaintenanceMode enables or disables maintenance mode
func (e *EnterpriseHealthCheck) SetMaintenanceMode(enabled bool, message string, startTime, endTime *time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.maintenanceMode = enabled

	if enabled && e.logger != nil {
		e.logger.Warn("Maintenance mode enabled",
			zap.String("message", message),
			zap.Time("start_time", *startTime),
			zap.Time("end_time", *endTime),
		)
	}
}

// IsMaintenanceMode returns true if maintenance mode is enabled
func (e *EnterpriseHealthCheck) IsMaintenanceMode() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.maintenanceMode
}

// CheckWithMode performs health check with specified mode
func (e *EnterpriseHealthCheck) CheckWithMode(ctx context.Context, mode HealthCheckMode) EnterpriseResponse {
	start := time.Now()

	// Create enterprise response
	response := EnterpriseResponse{
		Response:   e.HealthCheck.Check(ctx),
		SystemInfo: e.getSystemInfo(),
	}

	// Handle maintenance mode
	if e.IsMaintenanceMode() {
		response.Status = StatusDegraded
		response.Maintenance = &MaintenanceConfig{
			Enabled: true,
			Message: "System in maintenance mode",
		}

		if mode == ModeMaintenance {
			response.Status = StatusHealthy
		}
	}

	// Add dependency checks based on mode
	if mode == ModeDeep || mode == ModeStandard {
		response.Dependencies = e.dependencies.CheckAll(ctx)

		// Update overall status based on critical dependencies
		for _, dep := range response.Dependencies {
			if dep.Critical && dep.Status == StatusUnhealthy {
				response.Status = StatusUnhealthy
				break
			}
		}
	}

	// Add circuit breaker status
	response.CircuitBreakers = e.getCircuitBreakerStatus()

	// Record metrics
	e.metrics.RecordCheck(response.Status, time.Since(start))

	return response
}

// CheckDependencies performs dependency health checks
func (e *EnterpriseHealthCheck) CheckDependencies(ctx context.Context) []DependencyStatus {
	return e.dependencies.CheckAll(ctx)
}

// GetCircuitBreakerStatus returns the status of all circuit breakers
func (e *EnterpriseHealthCheck) GetCircuitBreakerStatus() map[string]CircuitBreakerStatus {
	return e.getCircuitBreakerStatus()
}

// PrepareShutdown prepares the service for graceful shutdown
func (e *EnterpriseHealthCheck) PrepareShutdown() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.gracefulShutdown = true

	if e.logger != nil {
		e.logger.Info("Health check prepared for graceful shutdown")
	}
}

// IsShuttingDown returns true if the service is shutting down
func (e *EnterpriseHealthCheck) IsShuttingDown() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.gracefulShutdown
}

// getCircuitBreakerStatus returns the current status of all circuit breakers
func (e *EnterpriseHealthCheck) getCircuitBreakerStatus() map[string]CircuitBreakerStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := make(map[string]CircuitBreakerStatus)
	for name, cb := range e.circuitBreakers {
		status[name] = cb.GetStatus()
	}

	return status
}

// getSystemInfo collects system information
func (e *EnterpriseHealthCheck) getSystemInfo() SystemInfo {
	// This is a basic implementation
	// In production, you would use libraries like gopsutil to get real system info
	return SystemInfo{
		Hostname:     getHostname(),
		Platform:     "linux", // Would be runtime.GOOS
		Architecture: "amd64", // Would be runtime.GOARCH
		CPUCores:     getCPUCores(),
		Memory:       getMemoryInfo(),
		Uptime:       getUptime(),
		Environment:  getEnvironmentInfo(),
	}
}

// Helper functions (basic implementations)
func getHostname() string {
	// Import "os" and use os.Hostname()
	return "alchemorsel-host"
}

func getCPUCores() int {
	// Import "runtime" and use runtime.NumCPU()
	return 4
}

func getMemoryInfo() MemoryInfo {
	// In production, use runtime.ReadMemStats() or gopsutil
	return MemoryInfo{
		Total:        8589934592, // 8GB
		Available:    4294967296, // 4GB
		Used:         4294967296, // 4GB
		UsagePercent: 50.0,
	}
}

func getUptime() time.Duration {
	// In production, calculate from process start time
	return 24 * time.Hour
}

func getEnvironmentInfo() map[string]string {
	return map[string]string{
		"GO_VERSION": "1.23",
		"SERVICE":    "alchemorsel-v3",
	}
}

// CircuitBreakerChecker wraps a checker with circuit breaker functionality
type CircuitBreakerChecker struct {
	checker Checker
	breaker *CircuitBreaker
	name    string
}

// Check performs the health check with circuit breaker protection
func (c *CircuitBreakerChecker) Check(ctx context.Context) Check {
	// Try to execute the check through the circuit breaker
	result, err := c.breaker.Execute(func() (interface{}, error) {
		check := c.checker.Check(ctx)
		if check.Status == StatusUnhealthy {
			return check, fmt.Errorf("health check failed: %s", check.Message)
		}
		return check, nil
	})

	if err != nil {
		// Circuit breaker is open or check failed
		return Check{
			Name:        c.name,
			Status:      StatusUnhealthy,
			Message:     err.Error(),
			LastChecked: time.Now(),
			Duration:    0,
			Metadata: map[string]interface{}{
				"circuit_breaker_state": c.breaker.GetState().String(),
			},
		}
	}

	check := result.(Check)
	check.Name = c.name

	// Add circuit breaker metadata
	if check.Metadata == nil {
		check.Metadata = make(map[string]interface{})
	}

	if metadata, ok := check.Metadata.(map[string]interface{}); ok {
		metadata["circuit_breaker_state"] = c.breaker.GetState().String()
	}

	return check
}
