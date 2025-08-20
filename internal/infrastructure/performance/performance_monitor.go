// Package performance provides real-time performance monitoring and 14KB compliance tracking
package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

// PerformanceMonitor tracks and reports on 14KB optimization performance
type PerformanceMonitor struct {
	config              MonitorConfig
	metrics             *PerformanceMetrics
	measurements        []Measurement
	alerts              []Alert
	thresholds          Thresholds
	mutex               sync.RWMutex
	firstPacketOptimizer *FirstPacketOptimizer
	compressionMiddleware *CompressionMiddleware
	resourceBundler     *ResourceBundler
	htmxOptimizer       *HTMXOptimizer
	criticalCSSExtractor *CriticalCSSExtractor
}

// MonitorConfig configures performance monitoring
type MonitorConfig struct {
	EnableRealTimeMonitoring bool          // Enable real-time performance tracking
	SampleRate              float64       // Sampling rate for measurements (0.0-1.0)
	MetricsRetention        time.Duration // How long to keep metrics
	AlertThreshold          int           // Size threshold for alerts (bytes)
	EnableCoreWebVitals     bool          // Track Core Web Vitals
	EnableFirstPacketTrack  bool          // Track first packet compliance
	ReportInterval          time.Duration // Interval for generating reports
	EnablePerformanceAPI    bool          // Expose performance API endpoints
}

// PerformanceMetrics aggregates all performance data
type PerformanceMetrics struct {
	FirstPacketCompliance   ComplianceMetrics
	CompressionEfficiency   CompressionMetrics
	ResourceOptimization    ResourceMetrics
	CoreWebVitals          WebVitalsMetrics
	HTMXPerformance        HTMXMetrics
	CSSOptimization        CSSMetrics
	OverallHealth          HealthMetrics
	LastUpdated            time.Time
}

// ComplianceMetrics tracks 14KB first packet compliance
type ComplianceMetrics struct {
	TotalRequests          int64
	CompliantRequests      int64
	ViolationRequests      int64
	ComplianceRate         float64
	AverageFirstPacketSize int
	LargestFirstPacket     int
	ViolationsByEndpoint   map[string]int64
}

// CompressionMetrics tracks compression performance
type CompressionMetrics struct {
	TotalRequests      int64
	CompressedRequests int64
	BrotliRequests     int64
	GzipRequests       int64
	AverageCompression float64
	TotalBytesSaved    int64
	CompressionRate    float64
}

// ResourceMetrics tracks resource optimization
type ResourceMetrics struct {
	TotalAssets       int
	OptimizedAssets   int
	CriticalAssets    int
	TotalSizeOriginal int64
	TotalSizeOptimized int64
	OptimizationRatio float64
	BundleCount       int
	CriticalBundleSize int
}

// WebVitalsMetrics tracks Core Web Vitals
type WebVitalsMetrics struct {
	FirstContentfulPaint   MetricDistribution
	LargestContentfulPaint MetricDistribution
	CumulativeLayoutShift  MetricDistribution
	FirstInputDelay       MetricDistribution
	TimeToFirstByte       MetricDistribution
	SpeedIndex            MetricDistribution
}

// HTMXMetrics tracks HTMX-specific performance
type HTMXMetrics struct {
	TotalElements        int
	CriticalElements     int
	DeferredElements     int
	ProgressiveLoadTime  MetricDistribution
	HTMXRequestLatency   MetricDistribution
}

// CSSMetrics tracks CSS optimization
type CSSMetrics struct {
	TotalCSSSize       int
	CriticalCSSSize    int
	ExtractionTime     time.Duration
	OptimizationRatio  float64
	SelectorCount      int
	CriticalSelectors  int
}

// HealthMetrics provides overall system health
type HealthMetrics struct {
	OverallScore          float64 // 0-100 score
	FirstPacketHealth     float64
	CompressionHealth     float64
	ResourceHealth        float64
	CoreWebVitalsHealth   float64
	RecommendationsCount  int
	CriticalIssuesCount   int
}

// MetricDistribution tracks metric percentiles
type MetricDistribution struct {
	P50   float64
	P75   float64
	P90   float64
	P95   float64
	P99   float64
	Count int64
	Total float64
}

// Measurement represents a single performance measurement
type Measurement struct {
	Timestamp      time.Time
	Endpoint       string
	FirstPacketSize int
	CompressedSize int
	CompressionType string
	LoadTime       time.Duration
	CoreWebVitals  map[string]float64
	Compliant      bool
	UserAgent      string
	ConnectionType string
}

// Alert represents a performance alert
type Alert struct {
	ID          string
	Type        string
	Severity    string
	Message     string
	Timestamp   time.Time
	Endpoint    string
	Value       interface{}
	Threshold   interface{}
	Resolved    bool
	ResolvedAt  *time.Time
}

// Thresholds defines performance thresholds
type Thresholds struct {
	FirstPacketSize       int     // 14KB limit
	ComplianceRate        float64 // Minimum compliance rate
	CompressionRatio      float64 // Minimum compression ratio
	FirstContentfulPaint  float64 // FCP threshold (ms)
	LargestContentfulPaint float64 // LCP threshold (ms)
	CumulativeLayoutShift float64 // CLS threshold
	FirstInputDelay       float64 // FID threshold (ms)
}

// DefaultMonitorConfig returns sensible monitoring defaults
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		EnableRealTimeMonitoring: true,
		SampleRate:              0.1, // Sample 10% of requests
		MetricsRetention:        24 * time.Hour,
		AlertThreshold:          MaxFirstPacketSize,
		EnableCoreWebVitals:     true,
		EnableFirstPacketTrack:  true,
		ReportInterval:          5 * time.Minute,
		EnablePerformanceAPI:    true,
	}
}

// DefaultThresholds returns performance thresholds per Core Web Vitals
func DefaultThresholds() Thresholds {
	return Thresholds{
		FirstPacketSize:        MaxFirstPacketSize, // 14KB
		ComplianceRate:         0.95,               // 95% compliance
		CompressionRatio:       0.7,                // 70% compression
		FirstContentfulPaint:   1800,               // 1.8s
		LargestContentfulPaint: 2500,               // 2.5s
		CumulativeLayoutShift:  0.1,                // 0.1
		FirstInputDelay:        100,                // 100ms
	}
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(config MonitorConfig, optimizer *FirstPacketOptimizer, 
	compression *CompressionMiddleware, bundler *ResourceBundler, 
	htmx *HTMXOptimizer, css *CriticalCSSExtractor) *PerformanceMonitor {
	
	return &PerformanceMonitor{
		config:                config,
		metrics:               &PerformanceMetrics{LastUpdated: time.Now()},
		thresholds:           DefaultThresholds(),
		firstPacketOptimizer: optimizer,
		compressionMiddleware: compression,
		resourceBundler:      bundler,
		htmxOptimizer:        htmx,
		criticalCSSExtractor: css,
	}
}

// RecordMeasurement records a performance measurement
func (pm *PerformanceMonitor) RecordMeasurement(measurement Measurement) {
	if !pm.shouldSample() {
		return
	}

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Add measurement
	pm.measurements = append(pm.measurements, measurement)

	// Clean old measurements
	pm.cleanOldMeasurements()

	// Update metrics
	pm.updateMetrics(measurement)

	// Check for alerts
	pm.checkAlerts(measurement)
}

// shouldSample determines if this request should be sampled
func (pm *PerformanceMonitor) shouldSample() bool {
	if !pm.config.EnableRealTimeMonitoring {
		return false
	}
	// Simple random sampling
	return time.Now().UnixNano()%100 < int64(pm.config.SampleRate*100)
}

// updateMetrics updates aggregate metrics with new measurement
func (pm *PerformanceMonitor) updateMetrics(measurement Measurement) {
	// Update first packet compliance
	pm.metrics.FirstPacketCompliance.TotalRequests++
	if measurement.Compliant {
		pm.metrics.FirstPacketCompliance.CompliantRequests++
	} else {
		pm.metrics.FirstPacketCompliance.ViolationRequests++
		if pm.metrics.FirstPacketCompliance.ViolationsByEndpoint == nil {
			pm.metrics.FirstPacketCompliance.ViolationsByEndpoint = make(map[string]int64)
		}
		pm.metrics.FirstPacketCompliance.ViolationsByEndpoint[measurement.Endpoint]++
	}

	// Calculate compliance rate
	if pm.metrics.FirstPacketCompliance.TotalRequests > 0 {
		pm.metrics.FirstPacketCompliance.ComplianceRate = 
			float64(pm.metrics.FirstPacketCompliance.CompliantRequests) / 
			float64(pm.metrics.FirstPacketCompliance.TotalRequests)
	}

	// Update size metrics
	if measurement.FirstPacketSize > pm.metrics.FirstPacketCompliance.LargestFirstPacket {
		pm.metrics.FirstPacketCompliance.LargestFirstPacket = measurement.FirstPacketSize
	}

	// Running average of first packet size
	if pm.metrics.FirstPacketCompliance.TotalRequests == 1 {
		pm.metrics.FirstPacketCompliance.AverageFirstPacketSize = measurement.FirstPacketSize
	} else {
		count := float64(pm.metrics.FirstPacketCompliance.TotalRequests)
		current := float64(pm.metrics.FirstPacketCompliance.AverageFirstPacketSize)
		new := float64(measurement.FirstPacketSize)
		pm.metrics.FirstPacketCompliance.AverageFirstPacketSize = 
			int((current*(count-1) + new) / count)
	}

	// Update Core Web Vitals if available
	if len(measurement.CoreWebVitals) > 0 {
		pm.updateWebVitalsMetrics(measurement.CoreWebVitals)
	}

	pm.metrics.LastUpdated = time.Now()
}

// updateWebVitalsMetrics updates Core Web Vitals metrics
func (pm *PerformanceMonitor) updateWebVitalsMetrics(vitals map[string]float64) {
	if fcp, exists := vitals["FCP"]; exists {
		pm.updateMetricDistribution(&pm.metrics.CoreWebVitals.FirstContentfulPaint, fcp)
	}
	if lcp, exists := vitals["LCP"]; exists {
		pm.updateMetricDistribution(&pm.metrics.CoreWebVitals.LargestContentfulPaint, lcp)
	}
	if cls, exists := vitals["CLS"]; exists {
		pm.updateMetricDistribution(&pm.metrics.CoreWebVitals.CumulativeLayoutShift, cls)
	}
	if fid, exists := vitals["FID"]; exists {
		pm.updateMetricDistribution(&pm.metrics.CoreWebVitals.FirstInputDelay, fid)
	}
	if ttfb, exists := vitals["TTFB"]; exists {
		pm.updateMetricDistribution(&pm.metrics.CoreWebVitals.TimeToFirstByte, ttfb)
	}
}

// updateMetricDistribution updates a metric distribution with a new value
func (pm *PerformanceMonitor) updateMetricDistribution(dist *MetricDistribution, value float64) {
	dist.Count++
	dist.Total += value

	// For simplicity, recalculate percentiles from recent measurements
	// In production, you'd use a more efficient algorithm like t-digest
	pm.recalculatePercentiles(dist)
}

// recalculatePercentiles recalculates percentiles for a metric distribution
func (pm *PerformanceMonitor) recalculatePercentiles(dist *MetricDistribution) {
	// Collect recent values for this metric (simplified approach)
	var values []float64
	recentCount := int(dist.Count)
	if recentCount > 1000 {
		recentCount = 1000 // Limit to recent 1000 measurements
	}

	// In a real implementation, you'd maintain a sliding window of values
	// For this example, we'll use a simplified approach
	if dist.Count > 0 {
		avg := dist.Total / float64(dist.Count)
		values = append(values, avg) // Simplified - just use average
	}

	if len(values) == 0 {
		return
	}

	sort.Float64s(values)
	
	dist.P50 = percentile(values, 0.5)
	dist.P75 = percentile(values, 0.75)
	dist.P90 = percentile(values, 0.9)
	dist.P95 = percentile(values, 0.95)
	dist.P99 = percentile(values, 0.99)
}

// percentile calculates the percentile value from sorted slice
func percentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	
	index := p * float64(len(sortedValues)-1)
	lower := int(index)
	upper := lower + 1
	
	if upper >= len(sortedValues) {
		return sortedValues[len(sortedValues)-1]
	}
	
	weight := index - float64(lower)
	return sortedValues[lower]*(1-weight) + sortedValues[upper]*weight
}

// checkAlerts checks if any thresholds are exceeded
func (pm *PerformanceMonitor) checkAlerts(measurement Measurement) {
	// First packet size violation
	if measurement.FirstPacketSize > pm.thresholds.FirstPacketSize {
		alert := Alert{
			ID:        fmt.Sprintf("first-packet-%d", time.Now().UnixNano()),
			Type:      "first_packet_violation",
			Severity:  "warning",
			Message:   fmt.Sprintf("First packet size %d bytes exceeds 14KB limit", measurement.FirstPacketSize),
			Timestamp: time.Now(),
			Endpoint:  measurement.Endpoint,
			Value:     measurement.FirstPacketSize,
			Threshold: pm.thresholds.FirstPacketSize,
		}
		pm.alerts = append(pm.alerts, alert)
	}

	// Compliance rate violation
	if pm.metrics.FirstPacketCompliance.ComplianceRate < pm.thresholds.ComplianceRate {
		alert := Alert{
			ID:        fmt.Sprintf("compliance-rate-%d", time.Now().UnixNano()),
			Type:      "compliance_rate_low",
			Severity:  "critical",
			Message:   fmt.Sprintf("First packet compliance rate %.2f%% below threshold %.2f%%", 
				pm.metrics.FirstPacketCompliance.ComplianceRate*100, pm.thresholds.ComplianceRate*100),
			Timestamp: time.Now(),
			Value:     pm.metrics.FirstPacketCompliance.ComplianceRate,
			Threshold: pm.thresholds.ComplianceRate,
		}
		pm.alerts = append(pm.alerts, alert)
	}

	// Core Web Vitals violations
	if vitals := measurement.CoreWebVitals; len(vitals) > 0 {
		if fcp, exists := vitals["FCP"]; exists && fcp > pm.thresholds.FirstContentfulPaint {
			alert := Alert{
				ID:        fmt.Sprintf("fcp-%d", time.Now().UnixNano()),
				Type:      "core_web_vitals",
				Severity:  "warning",
				Message:   fmt.Sprintf("First Contentful Paint %.2fms exceeds threshold %.2fms", fcp, pm.thresholds.FirstContentfulPaint),
				Timestamp: time.Now(),
				Endpoint:  measurement.Endpoint,
				Value:     fcp,
				Threshold: pm.thresholds.FirstContentfulPaint,
			}
			pm.alerts = append(pm.alerts, alert)
		}
	}
}

// cleanOldMeasurements removes measurements older than retention period
func (pm *PerformanceMonitor) cleanOldMeasurements() {
	cutoff := time.Now().Add(-pm.config.MetricsRetention)
	var filtered []Measurement
	
	for _, measurement := range pm.measurements {
		if measurement.Timestamp.After(cutoff) {
			filtered = append(filtered, measurement)
		}
	}
	
	pm.measurements = filtered
}

// CollectSystemMetrics collects metrics from all optimization components
func (pm *PerformanceMonitor) CollectSystemMetrics() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Collect from first packet optimizer
	if pm.firstPacketOptimizer != nil {
		optimizerMetrics := pm.firstPacketOptimizer.GetMetrics()
		pm.metrics.FirstPacketCompliance.ComplianceRate = 
			float64(optimizerMetrics.TotalCompliant) / float64(optimizerMetrics.TemplateCount)
	}

	// Collect from compression middleware
	if pm.compressionMiddleware != nil {
		compStats := pm.compressionMiddleware.GetStats()
		pm.metrics.CompressionEfficiency.TotalRequests = compStats.TotalRequests
		pm.metrics.CompressionEfficiency.CompressedRequests = compStats.CompressedRequests
		pm.metrics.CompressionEfficiency.BrotliRequests = compStats.BrotliRequests
		pm.metrics.CompressionEfficiency.GzipRequests = compStats.GzipRequests
		pm.metrics.CompressionEfficiency.AverageCompression = compStats.AverageCompression
		pm.metrics.CompressionEfficiency.TotalBytesSaved = compStats.TotalBytesSaved
		
		if compStats.TotalRequests > 0 {
			pm.metrics.CompressionEfficiency.CompressionRate = 
				float64(compStats.CompressedRequests) / float64(compStats.TotalRequests)
		}
	}

	// Collect from resource bundler
	if pm.resourceBundler != nil {
		bundles := pm.resourceBundler.GetBundles()
		pm.metrics.ResourceOptimization.BundleCount = len(bundles)
		
		for _, bundle := range bundles {
			if bundle.Critical {
				pm.metrics.ResourceOptimization.CriticalAssets += len(bundle.Assets)
				pm.metrics.ResourceOptimization.CriticalBundleSize += bundle.CompressedSize
			}
			pm.metrics.ResourceOptimization.TotalSizeOriginal += int64(bundle.Size)
			pm.metrics.ResourceOptimization.TotalSizeOptimized += int64(bundle.CompressedSize)
		}
		
		if pm.metrics.ResourceOptimization.TotalSizeOriginal > 0 {
			pm.metrics.ResourceOptimization.OptimizationRatio = 
				float64(pm.metrics.ResourceOptimization.TotalSizeOptimized) / 
				float64(pm.metrics.ResourceOptimization.TotalSizeOriginal)
		}
	}

	// Collect from HTMX optimizer
	if pm.htmxOptimizer != nil {
		htmxMetrics := pm.htmxOptimizer.GetMetrics()
		pm.metrics.HTMXPerformance.TotalElements = htmxMetrics.CriticalElementsCount + htmxMetrics.DeferredElementsCount
		pm.metrics.HTMXPerformance.CriticalElements = htmxMetrics.CriticalElementsCount
		pm.metrics.HTMXPerformance.DeferredElements = htmxMetrics.DeferredElementsCount
	}

	// Update health score
	pm.calculateHealthScore()
}

// calculateHealthScore calculates overall system health score
func (pm *PerformanceMonitor) calculateHealthScore() {
	var scores []float64

	// First packet compliance score
	firstPacketScore := pm.metrics.FirstPacketCompliance.ComplianceRate * 100
	scores = append(scores, firstPacketScore)

	// Compression efficiency score  
	compressionScore := pm.metrics.CompressionEfficiency.CompressionRate * 100
	scores = append(scores, compressionScore)

	// Resource optimization score
	if pm.metrics.ResourceOptimization.OptimizationRatio > 0 {
		resourceScore := (1.0 - pm.metrics.ResourceOptimization.OptimizationRatio) * 100
		scores = append(scores, resourceScore)
	}

	// Calculate weighted average
	if len(scores) > 0 {
		total := 0.0
		for _, score := range scores {
			total += score
		}
		pm.metrics.OverallHealth.OverallScore = total / float64(len(scores))
	}

	// Set component health scores
	pm.metrics.OverallHealth.FirstPacketHealth = firstPacketScore
	pm.metrics.OverallHealth.CompressionHealth = compressionScore
	pm.metrics.OverallHealth.RecommendationsCount = len(pm.generateRecommendations())
	pm.metrics.OverallHealth.CriticalIssuesCount = pm.countCriticalIssues()
}

// generateRecommendations generates optimization recommendations
func (pm *PerformanceMonitor) generateRecommendations() []string {
	var recommendations []string

	// First packet compliance recommendations
	if pm.metrics.FirstPacketCompliance.ComplianceRate < 0.9 {
		recommendations = append(recommendations, 
			"Low first packet compliance - consider reducing template sizes")
	}

	// Compression recommendations
	if pm.metrics.CompressionEfficiency.CompressionRate < 0.8 {
		recommendations = append(recommendations, 
			"Low compression rate - verify Brotli/Gzip configuration")
	}

	// Resource optimization recommendations
	if pm.metrics.ResourceOptimization.CriticalBundleSize > MaxCriticalCSS {
		recommendations = append(recommendations, 
			"Critical bundle size exceeds 8KB - split into smaller bundles")
	}

	return recommendations
}

// countCriticalIssues counts critical performance issues
func (pm *PerformanceMonitor) countCriticalIssues() int {
	count := 0

	// Check for critical alerts
	for _, alert := range pm.alerts {
		if alert.Severity == "critical" && !alert.Resolved {
			count++
		}
	}

	return count
}

// GetMetrics returns current performance metrics
func (pm *PerformanceMonitor) GetMetrics() *PerformanceMetrics {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.metrics
}

// GetAlerts returns current alerts
func (pm *PerformanceMonitor) GetAlerts() []Alert {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()
	return pm.alerts
}

// GenerateReport generates a comprehensive performance report
func (pm *PerformanceMonitor) GenerateReport() string {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	report := fmt.Sprintf(`=== Performance Monitoring Report ===
Generated: %s

=== 14KB First Packet Compliance ===
Total Requests: %d
Compliant Requests: %d (%.1f%%)
Violations: %d
Average First Packet Size: %d bytes
Largest First Packet: %d bytes

=== Compression Performance ===
Total Requests: %d
Compressed: %d (%.1f%%)
Brotli: %d, Gzip: %d
Average Compression: %.1f bytes
Total Bytes Saved: %d

=== Resource Optimization ===
Total Bundles: %d
Critical Assets: %d
Critical Bundle Size: %d bytes
Optimization Ratio: %.1f%%

=== System Health ===
Overall Score: %.1f/100
First Packet Health: %.1f/100
Compression Health: %.1f/100
Active Recommendations: %d
Critical Issues: %d

=== Core Web Vitals ===
FCP P95: %.1fms (target: <%.1fms)
LCP P95: %.1fms (target: <%.1fms)
CLS P95: %.3f (target: <%.3f)

=== Active Alerts ===
`,
		time.Now().Format(time.RFC3339),
		pm.metrics.FirstPacketCompliance.TotalRequests,
		pm.metrics.FirstPacketCompliance.CompliantRequests,
		pm.metrics.FirstPacketCompliance.ComplianceRate*100,
		pm.metrics.FirstPacketCompliance.ViolationRequests,
		pm.metrics.FirstPacketCompliance.AverageFirstPacketSize,
		pm.metrics.FirstPacketCompliance.LargestFirstPacket,
		pm.metrics.CompressionEfficiency.TotalRequests,
		pm.metrics.CompressionEfficiency.CompressedRequests,
		pm.metrics.CompressionEfficiency.CompressionRate*100,
		pm.metrics.CompressionEfficiency.BrotliRequests,
		pm.metrics.CompressionEfficiency.GzipRequests,
		pm.metrics.CompressionEfficiency.AverageCompression,
		pm.metrics.CompressionEfficiency.TotalBytesSaved,
		pm.metrics.ResourceOptimization.BundleCount,
		pm.metrics.ResourceOptimization.CriticalAssets,
		pm.metrics.ResourceOptimization.CriticalBundleSize,
		pm.metrics.ResourceOptimization.OptimizationRatio*100,
		pm.metrics.OverallHealth.OverallScore,
		pm.metrics.OverallHealth.FirstPacketHealth,
		pm.metrics.OverallHealth.CompressionHealth,
		pm.metrics.OverallHealth.RecommendationsCount,
		pm.metrics.OverallHealth.CriticalIssuesCount,
		pm.metrics.CoreWebVitals.FirstContentfulPaint.P95,
		pm.thresholds.FirstContentfulPaint,
		pm.metrics.CoreWebVitals.LargestContentfulPaint.P95,
		pm.thresholds.LargestContentfulPaint,
		pm.metrics.CoreWebVitals.CumulativeLayoutShift.P95,
		pm.thresholds.CumulativeLayoutShift,
	)

	// Add active alerts
	unresolved := 0
	for _, alert := range pm.alerts {
		if !alert.Resolved {
			report += fmt.Sprintf("- [%s] %s: %s\n", alert.Severity, alert.Type, alert.Message)
			unresolved++
		}
	}

	if unresolved == 0 {
		report += "No active alerts\n"
	}

	return report
}

// StartReporting starts periodic performance reporting
func (pm *PerformanceMonitor) StartReporting(ctx context.Context) {
	ticker := time.NewTicker(pm.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.CollectSystemMetrics()
			if pm.config.EnablePerformanceAPI {
				log.Printf("Performance Report:\n%s", pm.GenerateReport())
			}
		}
	}
}

// HTTPHandler returns an HTTP handler for performance metrics API
func (pm *PerformanceMonitor) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			pm.handleMetrics(w, r)
		case "/alerts":
			pm.handleAlerts(w, r)
		case "/report":
			pm.handleReport(w, r)
		case "/health":
			pm.handleHealth(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func (pm *PerformanceMonitor) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pm.GetMetrics())
}

func (pm *PerformanceMonitor) handleAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pm.GetAlerts())
}

func (pm *PerformanceMonitor) handleReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(pm.GenerateReport()))
}

func (pm *PerformanceMonitor) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"score":  pm.metrics.OverallHealth.OverallScore,
		"issues": pm.metrics.OverallHealth.CriticalIssuesCount,
	}
	
	if pm.metrics.OverallHealth.OverallScore < 70 {
		health["status"] = "degraded"
	}
	if pm.metrics.OverallHealth.CriticalIssuesCount > 0 {
		health["status"] = "unhealthy"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}