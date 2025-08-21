package network

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// LatencyMonitor continuously monitors network latency to various regions
type LatencyMonitor struct {
	logger       *zap.Logger
	measurements map[string]*LatencyData
	mu           sync.RWMutex
	stopCh       chan struct{}
	config       LatencyMonitorConfig
}

// LatencyMonitorConfig holds configuration for latency monitoring
type LatencyMonitorConfig struct {
	ProbeInterval    time.Duration
	ProbeTimeout     time.Duration
	SampleSize       int
	HistoryRetention time.Duration
	Targets          []LatencyTarget
}

// LatencyTarget represents a target to monitor
type LatencyTarget struct {
	Name     string
	Address  string
	Port     int
	Protocol string // "tcp", "udp", "icmp", "http"
	Region   string
	Type     string
}

// LatencyData stores latency measurements for a target
type LatencyData struct {
	Target        LatencyTarget
	Measurements  []LatencyMeasurement
	Average       time.Duration
	Minimum       time.Duration
	Maximum       time.Duration
	P50           time.Duration
	P95           time.Duration
	P99           time.Duration
	LastUpdated   time.Time
	ErrorRate     float64
	mu            sync.RWMutex
}

// LatencyMeasurement represents a single latency measurement
type LatencyMeasurement struct {
	Timestamp time.Time
	Latency   time.Duration
	Success   bool
	Error     string
}

// NewLatencyMonitor creates a new latency monitor
func NewLatencyMonitor(logger *zap.Logger) *LatencyMonitor {
	return &LatencyMonitor{
		logger:       logger,
		measurements: make(map[string]*LatencyData),
		stopCh:       make(chan struct{}),
		config: LatencyMonitorConfig{
			ProbeInterval:    30 * time.Second,
			ProbeTimeout:     5 * time.Second,
			SampleSize:       100,
			HistoryRetention: 24 * time.Hour,
		},
	}
}

// Start begins latency monitoring
func (lm *LatencyMonitor) Start(ctx context.Context, config LatencyMonitorConfig) error {
	lm.config = config
	
	// Initialize monitoring data for each target
	for _, target := range config.Targets {
		lm.measurements[target.Name] = &LatencyData{
			Target:       target,
			Measurements: make([]LatencyMeasurement, 0, config.SampleSize),
		}
	}

	// Start monitoring goroutines
	for _, target := range config.Targets {
		go lm.monitorTarget(ctx, target)
	}

	// Start cleanup goroutine
	go lm.cleanupOldData(ctx)

	lm.logger.Info("Latency monitoring started", 
		zap.Int("targets", len(config.Targets)),
		zap.Duration("interval", config.ProbeInterval))

	return nil
}

// Stop stops latency monitoring
func (lm *LatencyMonitor) Stop() {
	close(lm.stopCh)
	lm.logger.Info("Latency monitoring stopped")
}

// monitorTarget monitors latency for a specific target
func (lm *LatencyMonitor) monitorTarget(ctx context.Context, target LatencyTarget) {
	ticker := time.NewTicker(lm.config.ProbeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lm.stopCh:
			return
		case <-ticker.C:
			lm.probeTarget(target)
		}
	}
}

// probeTarget performs a latency probe to a target
func (lm *LatencyMonitor) probeTarget(target LatencyTarget) {
	start := time.Now()
	var latency time.Duration
	var success bool
	var errorMsg string

	switch target.Protocol {
	case "tcp":
		latency, success, errorMsg = lm.probeTCP(target)
	case "http", "https":
		latency, success, errorMsg = lm.probeHTTP(target)
	case "icmp":
		latency, success, errorMsg = lm.probeICMP(target)
	default:
		latency, success, errorMsg = lm.probeTCP(target)
	}

	measurement := LatencyMeasurement{
		Timestamp: start,
		Latency:   latency,
		Success:   success,
		Error:     errorMsg,
	}

	lm.addMeasurement(target.Name, measurement)

	if !success {
		lm.logger.Debug("Latency probe failed",
			zap.String("target", target.Name),
			zap.String("address", target.Address),
			zap.String("error", errorMsg))
	}
}

// probeTCP performs a TCP connection probe
func (lm *LatencyMonitor) probeTCP(target LatencyTarget) (time.Duration, bool, string) {
	start := time.Now()
	
	conn, err := net.DialTimeout("tcp", 
		fmt.Sprintf("%s:%d", target.Address, target.Port), 
		lm.config.ProbeTimeout)
	
	latency := time.Since(start)
	
	if err != nil {
		return latency, false, err.Error()
	}
	
	conn.Close()
	return latency, true, ""
}

// probeHTTP performs an HTTP probe
func (lm *LatencyMonitor) probeHTTP(target LatencyTarget) (time.Duration, bool, string) {
	start := time.Now()
	
	client := &http.Client{
		Timeout: lm.config.ProbeTimeout,
	}
	
	protocol := target.Protocol
	if protocol == "" {
		protocol = "http"
	}
	
	url := fmt.Sprintf("%s://%s:%d/health", protocol, target.Address, target.Port)
	resp, err := client.Head(url)
	
	latency := time.Since(start)
	
	if err != nil {
		return latency, false, err.Error()
	}
	
	defer resp.Body.Close()
	
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return latency, true, ""
	}
	
	return latency, false, fmt.Sprintf("HTTP %d", resp.StatusCode)
}

// probeICMP performs an ICMP ping probe
func (lm *LatencyMonitor) probeICMP(target LatencyTarget) (time.Duration, bool, string) {
	// ICMP requires raw sockets and additional privileges
	// This is a simplified implementation that falls back to TCP
	return lm.probeTCP(target)
}

// addMeasurement adds a new measurement to the target's data
func (lm *LatencyMonitor) addMeasurement(targetName string, measurement LatencyMeasurement) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	data, exists := lm.measurements[targetName]
	if !exists {
		return
	}

	data.mu.Lock()
	defer data.mu.Unlock()

	// Add the measurement
	data.Measurements = append(data.Measurements, measurement)

	// Keep only the latest N measurements
	if len(data.Measurements) > lm.config.SampleSize {
		data.Measurements = data.Measurements[1:]
	}

	// Update statistics
	lm.updateStatistics(data)
}

// updateStatistics recalculates statistics for latency data
func (lm *LatencyMonitor) updateStatistics(data *LatencyData) {
	if len(data.Measurements) == 0 {
		return
	}

	// Filter successful measurements for latency calculations
	successfulMeasurements := make([]time.Duration, 0)
	successCount := 0

	for _, m := range data.Measurements {
		if m.Success {
			successfulMeasurements = append(successfulMeasurements, m.Latency)
			successCount++
		}
	}

	// Calculate error rate
	data.ErrorRate = 1.0 - (float64(successCount) / float64(len(data.Measurements)))

	if len(successfulMeasurements) == 0 {
		data.Average = 0
		data.Minimum = 0
		data.Maximum = 0
		data.P50 = 0
		data.P95 = 0
		data.P99 = 0
		data.LastUpdated = time.Now()
		return
	}

	// Sort for percentile calculations
	sort.Slice(successfulMeasurements, func(i, j int) bool {
		return successfulMeasurements[i] < successfulMeasurements[j]
	})

	// Calculate statistics
	data.Minimum = successfulMeasurements[0]
	data.Maximum = successfulMeasurements[len(successfulMeasurements)-1]

	// Calculate average
	var total time.Duration
	for _, latency := range successfulMeasurements {
		total += latency
	}
	data.Average = total / time.Duration(len(successfulMeasurements))

	// Calculate percentiles
	data.P50 = lm.calculatePercentile(successfulMeasurements, 50)
	data.P95 = lm.calculatePercentile(successfulMeasurements, 95)
	data.P99 = lm.calculatePercentile(successfulMeasurements, 99)

	data.LastUpdated = time.Now()
}

// calculatePercentile calculates the specified percentile from sorted latency data
func (lm *LatencyMonitor) calculatePercentile(sortedLatencies []time.Duration, percentile int) time.Duration {
	if len(sortedLatencies) == 0 {
		return 0
	}

	index := (percentile * len(sortedLatencies)) / 100
	if index >= len(sortedLatencies) {
		index = len(sortedLatencies) - 1
	}

	return sortedLatencies[index]
}

// GetAverageLatency returns the average latency for a region and request type
func (lm *LatencyMonitor) GetAverageLatency(region, requestType string) time.Duration {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	// Find matching target
	targetKey := fmt.Sprintf("%s_%s", region, requestType)
	if data, exists := lm.measurements[targetKey]; exists {
		data.mu.RLock()
		defer data.mu.RUnlock()
		return data.Average
	}

	// If no specific match, find region match
	for _, data := range lm.measurements {
		if data.Target.Region == region {
			data.mu.RLock()
			average := data.Average
			data.mu.RUnlock()
			return average
		}
	}

	return 0
}

// GetLatencyData returns complete latency data for a target
func (lm *LatencyMonitor) GetLatencyData(targetName string) (*LatencyData, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if data, exists := lm.measurements[targetName]; exists {
		data.mu.RLock()
		defer data.mu.RUnlock()

		// Return a copy to avoid race conditions
		return &LatencyData{
			Target:      data.Target,
			Average:     data.Average,
			Minimum:     data.Minimum,
			Maximum:     data.Maximum,
			P50:         data.P50,
			P95:         data.P95,
			P99:         data.P99,
			LastUpdated: data.LastUpdated,
			ErrorRate:   data.ErrorRate,
		}, true
	}

	return nil, false
}

// GetAllLatencyData returns latency data for all targets
func (lm *LatencyMonitor) GetAllLatencyData() map[string]*LatencyData {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	result := make(map[string]*LatencyData)
	for name, data := range lm.measurements {
		data.mu.RLock()
		result[name] = &LatencyData{
			Target:      data.Target,
			Average:     data.Average,
			Minimum:     data.Minimum,
			Maximum:     data.Maximum,
			P50:         data.P50,
			P95:         data.P95,
			P99:         data.P99,
			LastUpdated: data.LastUpdated,
			ErrorRate:   data.ErrorRate,
		}
		data.mu.RUnlock()
	}

	return result
}

// cleanupOldData removes old measurements to prevent memory leaks
func (lm *LatencyMonitor) cleanupOldData(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-lm.stopCh:
			return
		case <-ticker.C:
			lm.performCleanup()
		}
	}
}

// performCleanup removes measurements older than the retention period
func (lm *LatencyMonitor) performCleanup() {
	cutoff := time.Now().Add(-lm.config.HistoryRetention)

	lm.mu.Lock()
	defer lm.mu.Unlock()

	for _, data := range lm.measurements {
		data.mu.Lock()
		
		// Filter out old measurements
		filtered := make([]LatencyMeasurement, 0)
		for _, measurement := range data.Measurements {
			if measurement.Timestamp.After(cutoff) {
				filtered = append(filtered, measurement)
			}
		}
		
		data.Measurements = filtered
		lm.updateStatistics(data)
		
		data.mu.Unlock()
	}
}

// GetLatencyMetrics returns metrics suitable for Prometheus
func (lm *LatencyMonitor) GetLatencyMetrics() map[string]map[string]float64 {
	allData := lm.GetAllLatencyData()
	metrics := make(map[string]map[string]float64)

	for targetName, data := range allData {
		metrics[targetName] = map[string]float64{
			"average_ms":  float64(data.Average.Nanoseconds()) / 1e6,
			"minimum_ms":  float64(data.Minimum.Nanoseconds()) / 1e6,
			"maximum_ms":  float64(data.Maximum.Nanoseconds()) / 1e6,
			"p50_ms":      float64(data.P50.Nanoseconds()) / 1e6,
			"p95_ms":      float64(data.P95.Nanoseconds()) / 1e6,
			"p99_ms":      float64(data.P99.Nanoseconds()) / 1e6,
			"error_rate":  data.ErrorRate,
		}
	}

	return metrics
}

// SetLatencyTargets updates the monitoring targets
func (lm *LatencyMonitor) SetLatencyTargets(targets []LatencyTarget) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Clear existing measurements
	lm.measurements = make(map[string]*LatencyData)

	// Initialize new targets
	for _, target := range targets {
		lm.measurements[target.Name] = &LatencyData{
			Target:       target,
			Measurements: make([]LatencyMeasurement, 0, lm.config.SampleSize),
		}
	}

	lm.config.Targets = targets
	lm.logger.Info("Latency targets updated", zap.Int("count", len(targets)))
}

// GetTargetHealth returns health status for all targets
func (lm *LatencyMonitor) GetTargetHealth() map[string]TargetHealth {
	allData := lm.GetAllLatencyData()
	health := make(map[string]TargetHealth)

	for targetName, data := range allData {
		status := HealthStatusHealthy
		
		// Determine health based on error rate and latency
		if data.ErrorRate > 0.1 { // More than 10% errors
			status = HealthStatusUnhealthy
		} else if data.ErrorRate > 0.05 || data.Average > 500*time.Millisecond {
			status = HealthStatusDegraded
		}

		health[targetName] = TargetHealth{
			Status:      status,
			ErrorRate:   data.ErrorRate,
			Latency:     data.Average,
			LastChecked: data.LastUpdated,
		}
	}

	return health
}

// TargetHealth represents the health status of a monitored target
type TargetHealth struct {
	Status      HealthStatus
	ErrorRate   float64
	Latency     time.Duration
	LastChecked time.Time
}