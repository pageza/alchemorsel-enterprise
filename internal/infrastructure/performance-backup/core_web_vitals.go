// Package performance provides Core Web Vitals monitoring and optimization
package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// CoreWebVitalsMonitor tracks and optimizes Core Web Vitals metrics
type CoreWebVitalsMonitor struct {
	config         CWVConfig
	measurements   []CWVMeasurement
	aggregates     CWVAggregates
	thresholds     CWVThresholds
	alerts         []CWVAlert
	mutex          sync.RWMutex
	lastCalculated time.Time
}

// CWVConfig configures Core Web Vitals monitoring
type CWVConfig struct {
	EnableRealUserMonitoring bool          // Enable RUM collection
	SampleRate              float64       // Percentage of users to monitor (0.0-1.0)
	RetentionPeriod         time.Duration // How long to keep measurements
	AlertThresholds         CWVThresholds // Thresholds for alerts
	EnableSyntheticTests    bool          // Enable synthetic testing
	ReportInterval          time.Duration // Reporting frequency
	EnablePerformanceAPI    bool          // Expose performance API
}

// CWVThresholds defines Core Web Vitals performance thresholds
type CWVThresholds struct {
	LCP CWVThreshold // Largest Contentful Paint
	CLS CWVThreshold // Cumulative Layout Shift
	INP CWVThreshold // Interaction to Next Paint
	FCP CWVThreshold // First Contentful Paint (supplementary)
	TTFB CWVThreshold // Time to First Byte (supplementary)
}

// CWVThreshold represents performance threshold ranges
type CWVThreshold struct {
	Good        float64 // Good performance threshold
	NeedsWork   float64 // Needs improvement threshold
	Poor        float64 // Poor performance threshold
	Unit        string  // Unit of measurement (ms, score, etc.)
	MetricName  string  // Human-readable metric name
}

// CWVMeasurement represents a single Core Web Vitals measurement
type CWVMeasurement struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	URL           string                 `json:"url"`
	UserAgent     string                 `json:"user_agent"`
	ConnectionType string                `json:"connection_type"`
	DeviceType    string                 `json:"device_type"`
	ViewportSize  ViewportSize           `json:"viewport_size"`
	Metrics       map[string]float64     `json:"metrics"`
	Navigation    NavigationTiming       `json:"navigation"`
	Elements      []ElementMeasurement   `json:"elements"`
	Errors        []string               `json:"errors,omitempty"`
	UserID        string                 `json:"user_id,omitempty"`
}

// ViewportSize represents the user's viewport dimensions
type ViewportSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// NavigationTiming provides detailed navigation timing data
type NavigationTiming struct {
	NavigationStart     float64 `json:"navigation_start"`
	DOMContentLoaded    float64 `json:"dom_content_loaded"`
	LoadComplete        float64 `json:"load_complete"`
	FirstPaint          float64 `json:"first_paint"`
	FirstContentfulPaint float64 `json:"first_contentful_paint"`
	LargestContentfulPaint float64 `json:"largest_contentful_paint"`
	CumulativeLayoutShift float64 `json:"cumulative_layout_shift"`
	InteractionToNextPaint float64 `json:"interaction_to_next_paint"`
	TimeToFirstByte     float64 `json:"time_to_first_byte"`
}

// ElementMeasurement tracks individual element performance
type ElementMeasurement struct {
	Selector      string  `json:"selector"`
	ElementType   string  `json:"element_type"`
	LoadTime      float64 `json:"load_time"`
	RenderTime    float64 `json:"render_time"`
	LayoutShift   float64 `json:"layout_shift"`
	IsLCP         bool    `json:"is_lcp"`
	Size          Size    `json:"size"`
	Position      Position `json:"position"`
}

// Size represents element dimensions
type Size struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Position represents element position
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// CWVAggregates stores aggregated Core Web Vitals data
type CWVAggregates struct {
	LCP CWVMetricAggregate `json:"lcp"`
	CLS CWVMetricAggregate `json:"cls"`
	INP CWVMetricAggregate `json:"inp"`
	FCP CWVMetricAggregate `json:"fcp"`
	TTFB CWVMetricAggregate `json:"ttfb"`
	
	TotalMeasurements int       `json:"total_measurements"`
	LastUpdated       time.Time `json:"last_updated"`
	TimeRange         TimeRange `json:"time_range"`
}

// CWVMetricAggregate represents aggregated data for a single metric
type CWVMetricAggregate struct {
	P50         float64            `json:"p50"`
	P75         float64            `json:"p75"`
	P90         float64            `json:"p90"`
	P95         float64            `json:"p95"`
	P99         float64            `json:"p99"`
	Mean        float64            `json:"mean"`
	Count       int64              `json:"count"`
	Distribution CWVDistribution   `json:"distribution"`
	Trends      []CWVTrendPoint    `json:"trends"`
	ByDevice    map[string]float64 `json:"by_device"`
	ByConnection map[string]float64 `json:"by_connection"`
}

// CWVDistribution shows distribution across performance buckets
type CWVDistribution struct {
	Good      int64   `json:"good"`
	NeedsWork int64   `json:"needs_work"`
	Poor      int64   `json:"poor"`
	GoodRate  float64 `json:"good_rate"`
	PoorRate  float64 `json:"poor_rate"`
}

// CWVTrendPoint represents a point in the performance trend
type CWVTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Count     int64     `json:"count"`
}

// TimeRange represents the time range for aggregated data
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CWVAlert represents a Core Web Vitals performance alert
type CWVAlert struct {
	ID          string    `json:"id"`
	MetricName  string    `json:"metric_name"`
	Severity    string    `json:"severity"` // warning, critical
	Message     string    `json:"message"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	URL         string    `json:"url,omitempty"`
	DeviceType  string    `json:"device_type,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// DefaultCWVThresholds returns Google's Core Web Vitals thresholds
func DefaultCWVThresholds() CWVThresholds {
	return CWVThresholds{
		LCP: CWVThreshold{
			Good:       2500,  // 2.5 seconds
			NeedsWork:  4000,  // 4.0 seconds
			Poor:       4000,  // > 4.0 seconds
			Unit:       "ms",
			MetricName: "Largest Contentful Paint",
		},
		CLS: CWVThreshold{
			Good:       0.1,   // 0.1
			NeedsWork:  0.25,  // 0.25
			Poor:       0.25,  // > 0.25
			Unit:       "score",
			MetricName: "Cumulative Layout Shift",
		},
		INP: CWVThreshold{
			Good:       200,   // 200ms
			NeedsWork:  500,   // 500ms
			Poor:       500,   // > 500ms
			Unit:       "ms",
			MetricName: "Interaction to Next Paint",
		},
		FCP: CWVThreshold{
			Good:       1800,  // 1.8 seconds
			NeedsWork:  3000,  // 3.0 seconds
			Poor:       3000,  // > 3.0 seconds
			Unit:       "ms",
			MetricName: "First Contentful Paint",
		},
		TTFB: CWVThreshold{
			Good:       800,   // 800ms
			NeedsWork:  1800,  // 1.8 seconds
			Poor:       1800,  // > 1.8 seconds
			Unit:       "ms",
			MetricName: "Time to First Byte",
		},
	}
}

// DefaultCWVConfig returns sensible defaults for CWV monitoring
func DefaultCWVConfig() CWVConfig {
	return CWVConfig{
		EnableRealUserMonitoring: true,
		SampleRate:              0.05, // Monitor 5% of users
		RetentionPeriod:         30 * 24 * time.Hour, // 30 days
		AlertThresholds:         DefaultCWVThresholds(),
		EnableSyntheticTests:    true,
		ReportInterval:          5 * time.Minute,
		EnablePerformanceAPI:    true,
	}
}

// NewCoreWebVitalsMonitor creates a new Core Web Vitals monitor
func NewCoreWebVitalsMonitor(config CWVConfig) *CoreWebVitalsMonitor {
	return &CoreWebVitalsMonitor{
		config:     config,
		thresholds: config.AlertThresholds,
		aggregates: CWVAggregates{
			LCP:  CWVMetricAggregate{ByDevice: make(map[string]float64), ByConnection: make(map[string]float64)},
			CLS:  CWVMetricAggregate{ByDevice: make(map[string]float64), ByConnection: make(map[string]float64)},
			INP:  CWVMetricAggregate{ByDevice: make(map[string]float64), ByConnection: make(map[string]float64)},
			FCP:  CWVMetricAggregate{ByDevice: make(map[string]float64), ByConnection: make(map[string]float64)},
			TTFB: CWVMetricAggregate{ByDevice: make(map[string]float64), ByConnection: make(map[string]float64)},
		},
	}
}

// RecordMeasurement records a new Core Web Vitals measurement
func (cwv *CoreWebVitalsMonitor) RecordMeasurement(measurement CWVMeasurement) error {
	// Validate measurement
	if err := cwv.validateMeasurement(measurement); err != nil {
		return fmt.Errorf("invalid measurement: %w", err)
	}

	// Apply sampling
	if !cwv.shouldSample() {
		return nil
	}

	cwv.mutex.Lock()
	defer cwv.mutex.Unlock()

	// Add timestamp if not provided
	if measurement.Timestamp.IsZero() {
		measurement.Timestamp = time.Now()
	}

	// Generate ID if not provided
	if measurement.ID == "" {
		measurement.ID = fmt.Sprintf("cwv_%d_%d", measurement.Timestamp.UnixNano(), 
			len(cwv.measurements))
	}

	// Store measurement
	cwv.measurements = append(cwv.measurements, measurement)

	// Clean old measurements
	cwv.cleanOldMeasurements()

	// Check for alerts
	cwv.checkAlerts(measurement)

	// Trigger recalculation of aggregates
	cwv.scheduleRecalculation()

	return nil
}

// validateMeasurement validates a Core Web Vitals measurement
func (cwv *CoreWebVitalsMonitor) validateMeasurement(measurement CWVMeasurement) error {
	if measurement.URL == "" {
		return fmt.Errorf("URL is required")
	}

	// Validate metric values
	metrics := measurement.Metrics
	if lcp, exists := metrics["LCP"]; exists && (lcp < 0 || lcp > 60000) {
		return fmt.Errorf("LCP value %f is out of valid range", lcp)
	}
	if cls, exists := metrics["CLS"]; exists && (cls < 0 || cls > 10) {
		return fmt.Errorf("CLS value %f is out of valid range", cls)
	}
	if inp, exists := metrics["INP"]; exists && (inp < 0 || inp > 10000) {
		return fmt.Errorf("INP value %f is out of valid range", inp)
	}

	return nil
}

// shouldSample determines if this measurement should be recorded
func (cwv *CoreWebVitalsMonitor) shouldSample() bool {
	if !cwv.config.EnableRealUserMonitoring {
		return false
	}
	
	// Simple random sampling based on timestamp
	return float64(time.Now().UnixNano()%100)/100.0 < cwv.config.SampleRate
}

// cleanOldMeasurements removes measurements older than retention period
func (cwv *CoreWebVitalsMonitor) cleanOldMeasurements() {
	cutoff := time.Now().Add(-cwv.config.RetentionPeriod)
	filtered := make([]CWVMeasurement, 0, len(cwv.measurements))
	
	for _, measurement := range cwv.measurements {
		if measurement.Timestamp.After(cutoff) {
			filtered = append(filtered, measurement)
		}
	}
	
	cwv.measurements = filtered
}

// checkAlerts checks measurements against thresholds and generates alerts
func (cwv *CoreWebVitalsMonitor) checkAlerts(measurement CWVMeasurement) {
	timestamp := time.Now()
	
	// Check LCP
	if lcp, exists := measurement.Metrics["LCP"]; exists {
		if lcp > cwv.thresholds.LCP.Poor {
			alert := CWVAlert{
				ID:         fmt.Sprintf("lcp_%d", timestamp.UnixNano()),
				MetricName: "LCP",
				Severity:   "critical",
				Message:    fmt.Sprintf("LCP of %.0fms exceeds poor threshold of %.0fms", lcp, cwv.thresholds.LCP.Poor),
				Value:      lcp,
				Threshold:  cwv.thresholds.LCP.Poor,
				URL:        measurement.URL,
				DeviceType: measurement.DeviceType,
				Timestamp:  timestamp,
			}
			cwv.alerts = append(cwv.alerts, alert)
		} else if lcp > cwv.thresholds.LCP.Good {
			alert := CWVAlert{
				ID:         fmt.Sprintf("lcp_%d", timestamp.UnixNano()),
				MetricName: "LCP",
				Severity:   "warning",
				Message:    fmt.Sprintf("LCP of %.0fms needs improvement (target: <%.0fms)", lcp, cwv.thresholds.LCP.Good),
				Value:      lcp,
				Threshold:  cwv.thresholds.LCP.Good,
				URL:        measurement.URL,
				DeviceType: measurement.DeviceType,
				Timestamp:  timestamp,
			}
			cwv.alerts = append(cwv.alerts, alert)
		}
	}

	// Check CLS
	if cls, exists := measurement.Metrics["CLS"]; exists {
		if cls > cwv.thresholds.CLS.Poor {
			alert := CWVAlert{
				ID:         fmt.Sprintf("cls_%d", timestamp.UnixNano()),
				MetricName: "CLS",
				Severity:   "critical",
				Message:    fmt.Sprintf("CLS of %.3f exceeds poor threshold of %.3f", cls, cwv.thresholds.CLS.Poor),
				Value:      cls,
				Threshold:  cwv.thresholds.CLS.Poor,
				URL:        measurement.URL,
				DeviceType: measurement.DeviceType,
				Timestamp:  timestamp,
			}
			cwv.alerts = append(cwv.alerts, alert)
		} else if cls > cwv.thresholds.CLS.Good {
			alert := CWVAlert{
				ID:         fmt.Sprintf("cls_%d", timestamp.UnixNano()),
				MetricName: "CLS",
				Severity:   "warning",
				Message:    fmt.Sprintf("CLS of %.3f needs improvement (target: <%.3f)", cls, cwv.thresholds.CLS.Good),
				Value:      cls,
				Threshold:  cwv.thresholds.CLS.Good,
				URL:        measurement.URL,
				DeviceType: measurement.DeviceType,
				Timestamp:  timestamp,
			}
			cwv.alerts = append(cwv.alerts, alert)
		}
	}

	// Check INP
	if inp, exists := measurement.Metrics["INP"]; exists {
		if inp > cwv.thresholds.INP.Poor {
			alert := CWVAlert{
				ID:         fmt.Sprintf("inp_%d", timestamp.UnixNano()),
				MetricName: "INP",
				Severity:   "critical",
				Message:    fmt.Sprintf("INP of %.0fms exceeds poor threshold of %.0fms", inp, cwv.thresholds.INP.Poor),
				Value:      inp,
				Threshold:  cwv.thresholds.INP.Poor,
				URL:        measurement.URL,
				DeviceType: measurement.DeviceType,
				Timestamp:  timestamp,
			}
			cwv.alerts = append(cwv.alerts, alert)
		} else if inp > cwv.thresholds.INP.Good {
			alert := CWVAlert{
				ID:         fmt.Sprintf("inp_%d", timestamp.UnixNano()),
				MetricName: "INP",
				Severity:   "warning",
				Message:    fmt.Sprintf("INP of %.0fms needs improvement (target: <%.0fms)", inp, cwv.thresholds.INP.Good),
				Value:      inp,
				Threshold:  cwv.thresholds.INP.Good,
				URL:        measurement.URL,
				DeviceType: measurement.DeviceType,
				Timestamp:  timestamp,
			}
			cwv.alerts = append(cwv.alerts, alert)
		}
	}
}

// scheduleRecalculation schedules aggregate recalculation
func (cwv *CoreWebVitalsMonitor) scheduleRecalculation() {
	// Simple approach: recalculate if it's been more than 1 minute
	if time.Since(cwv.lastCalculated) > time.Minute {
		go cwv.RecalculateAggregates()
	}
}

// RecalculateAggregates recalculates all aggregate statistics
func (cwv *CoreWebVitalsMonitor) RecalculateAggregates() {
	cwv.mutex.Lock()
	defer cwv.mutex.Unlock()

	if len(cwv.measurements) == 0 {
		return
	}

	// Calculate aggregates for each metric
	cwv.aggregates.LCP = cwv.calculateMetricAggregate("LCP")
	cwv.aggregates.CLS = cwv.calculateMetricAggregate("CLS")
	cwv.aggregates.INP = cwv.calculateMetricAggregate("INP")
	cwv.aggregates.FCP = cwv.calculateMetricAggregate("FCP")
	cwv.aggregates.TTFB = cwv.calculateMetricAggregate("TTFB")

	// Update metadata
	cwv.aggregates.TotalMeasurements = len(cwv.measurements)
	cwv.aggregates.LastUpdated = time.Now()
	
	if len(cwv.measurements) > 0 {
		cwv.aggregates.TimeRange.Start = cwv.measurements[0].Timestamp
		cwv.aggregates.TimeRange.End = cwv.measurements[len(cwv.measurements)-1].Timestamp
	}

	cwv.lastCalculated = time.Now()
}

// calculateMetricAggregate calculates aggregate statistics for a specific metric
func (cwv *CoreWebVitalsMonitor) calculateMetricAggregate(metricName string) CWVMetricAggregate {
	var values []float64
	deviceMap := make(map[string][]float64)
	connectionMap := make(map[string][]float64)

	// Collect values
	for _, measurement := range cwv.measurements {
		if value, exists := measurement.Metrics[metricName]; exists && !math.IsNaN(value) && value >= 0 {
			values = append(values, value)
			
			// Group by device
			if measurement.DeviceType != "" {
				deviceMap[measurement.DeviceType] = append(deviceMap[measurement.DeviceType], value)
			}
			
			// Group by connection
			if measurement.ConnectionType != "" {
				connectionMap[measurement.ConnectionType] = append(connectionMap[measurement.ConnectionType], value)
			}
		}
	}

	if len(values) == 0 {
		return CWVMetricAggregate{}
	}

	// Sort values for percentile calculations
	sort.Float64s(values)

	// Calculate percentiles
	aggregate := CWVMetricAggregate{
		P50:   percentile(values, 0.5),
		P75:   percentile(values, 0.75),
		P90:   percentile(values, 0.9),
		P95:   percentile(values, 0.95),
		P99:   percentile(values, 0.99),
		Count: int64(len(values)),
	}

	// Calculate mean
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	aggregate.Mean = sum / float64(len(values))

	// Calculate distribution
	aggregate.Distribution = cwv.calculateDistribution(metricName, values)

	// Calculate device breakdowns
	aggregate.ByDevice = make(map[string]float64)
	for device, deviceValues := range deviceMap {
		if len(deviceValues) > 0 {
			sort.Float64s(deviceValues)
			aggregate.ByDevice[device] = percentile(deviceValues, 0.75) // Use P75 for comparison
		}
	}

	// Calculate connection breakdowns
	aggregate.ByConnection = make(map[string]float64)
	for connection, connectionValues := range connectionMap {
		if len(connectionValues) > 0 {
			sort.Float64s(connectionValues)
			aggregate.ByConnection[connection] = percentile(connectionValues, 0.75)
		}
	}

	return aggregate
}

// calculateDistribution calculates performance distribution buckets
func (cwv *CoreWebVitalsMonitor) calculateDistribution(metricName string, values []float64) CWVDistribution {
	var threshold CWVThreshold
	
	switch metricName {
	case "LCP":
		threshold = cwv.thresholds.LCP
	case "CLS":
		threshold = cwv.thresholds.CLS
	case "INP":
		threshold = cwv.thresholds.INP
	case "FCP":
		threshold = cwv.thresholds.FCP
	case "TTFB":
		threshold = cwv.thresholds.TTFB
	default:
		return CWVDistribution{}
	}

	var good, needsWork, poor int64
	
	for _, value := range values {
		if value <= threshold.Good {
			good++
		} else if value <= threshold.NeedsWork {
			needsWork++
		} else {
			poor++
		}
	}

	total := int64(len(values))
	
	return CWVDistribution{
		Good:      good,
		NeedsWork: needsWork,
		Poor:      poor,
		GoodRate:  float64(good) / float64(total),
		PoorRate:  float64(poor) / float64(total),
	}
}

// percentile calculates the percentile value from sorted slice
func percentile(sortedValues []float64, p float64) float64 {
	if len(sortedValues) == 0 {
		return 0
	}
	
	if len(sortedValues) == 1 {
		return sortedValues[0]
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

// GetAggregates returns current aggregate statistics
func (cwv *CoreWebVitalsMonitor) GetAggregates() CWVAggregates {
	cwv.mutex.RLock()
	defer cwv.mutex.RUnlock()
	return cwv.aggregates
}

// GetAlerts returns current alerts
func (cwv *CoreWebVitalsMonitor) GetAlerts() []CWVAlert {
	cwv.mutex.RLock()
	defer cwv.mutex.RUnlock()
	return cwv.alerts
}

// GetMeasurements returns raw measurements with optional filtering
func (cwv *CoreWebVitalsMonitor) GetMeasurements(limit int, offset int) []CWVMeasurement {
	cwv.mutex.RLock()
	defer cwv.mutex.RUnlock()

	total := len(cwv.measurements)
	if offset >= total {
		return []CWVMeasurement{}
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return cwv.measurements[offset:end]
}

// GenerateReport generates a comprehensive Core Web Vitals report
func (cwv *CoreWebVitalsMonitor) GenerateReport() string {
	cwv.mutex.RLock()
	defer cwv.mutex.RUnlock()

	aggregates := cwv.aggregates
	
	return fmt.Sprintf(`=== Core Web Vitals Report ===
Generated: %s
Time Range: %s to %s
Total Measurements: %d

=== Largest Contentful Paint (LCP) ===
P75: %.0fms (target: <%.0fms)
P95: %.0fms
Good Rate: %.1f%% | Poor Rate: %.1f%%
Status: %s

=== Cumulative Layout Shift (CLS) ===
P75: %.3f (target: <%.3f)
P95: %.3f
Good Rate: %.1f%% | Poor Rate: %.1f%%
Status: %s

=== Interaction to Next Paint (INP) ===
P75: %.0fms (target: <%.0fms)
P95: %.0fms
Good Rate: %.1f%% | Poor Rate: %.1f%%
Status: %s

=== Performance by Device Type ===
LCP Mobile: %.0fms | Desktop: %.0fms
CLS Mobile: %.3f | Desktop: %.3f
INP Mobile: %.0fms | Desktop: %.0fms

=== Active Alerts ===
%s

=== Recommendations ===
%s
`,
		time.Now().Format(time.RFC3339),
		aggregates.TimeRange.Start.Format(time.RFC3339),
		aggregates.TimeRange.End.Format(time.RFC3339),
		aggregates.TotalMeasurements,
		
		// LCP
		aggregates.LCP.P75,
		cwv.thresholds.LCP.Good,
		aggregates.LCP.P95,
		aggregates.LCP.Distribution.GoodRate*100,
		aggregates.LCP.Distribution.PoorRate*100,
		cwv.getMetricStatus("LCP", aggregates.LCP.P75),
		
		// CLS
		aggregates.CLS.P75,
		cwv.thresholds.CLS.Good,
		aggregates.CLS.P95,
		aggregates.CLS.Distribution.GoodRate*100,
		aggregates.CLS.Distribution.PoorRate*100,
		cwv.getMetricStatus("CLS", aggregates.CLS.P75),
		
		// INP
		aggregates.INP.P75,
		cwv.thresholds.INP.Good,
		aggregates.INP.P95,
		aggregates.INP.Distribution.GoodRate*100,
		aggregates.INP.Distribution.PoorRate*100,
		cwv.getMetricStatus("INP", aggregates.INP.P75),
		
		// Device breakdown
		aggregates.LCP.ByDevice["mobile"],
		aggregates.LCP.ByDevice["desktop"],
		aggregates.CLS.ByDevice["mobile"],
		aggregates.CLS.ByDevice["desktop"],
		aggregates.INP.ByDevice["mobile"],
		aggregates.INP.ByDevice["desktop"],
		
		// Alerts
		cwv.formatActiveAlerts(),
		
		// Recommendations
		cwv.generateRecommendations(),
	)
}

// getMetricStatus returns the status of a metric based on its value
func (cwv *CoreWebVitalsMonitor) getMetricStatus(metricName string, value float64) string {
	var threshold CWVThreshold
	
	switch metricName {
	case "LCP":
		threshold = cwv.thresholds.LCP
	case "CLS":
		threshold = cwv.thresholds.CLS
	case "INP":
		threshold = cwv.thresholds.INP
	case "FCP":
		threshold = cwv.thresholds.FCP
	case "TTFB":
		threshold = cwv.thresholds.TTFB
	default:
		return "Unknown"
	}

	if value <= threshold.Good {
		return "Good ✅"
	} else if value <= threshold.NeedsWork {
		return "Needs Improvement ⚠️"
	} else {
		return "Poor ❌"
	}
}

// formatActiveAlerts formats active alerts for the report
func (cwv *CoreWebVitalsMonitor) formatActiveAlerts() string {
	activeAlerts := make([]CWVAlert, 0)
	for _, alert := range cwv.alerts {
		if !alert.Resolved {
			activeAlerts = append(activeAlerts, alert)
		}
	}

	if len(activeAlerts) == 0 {
		return "No active alerts"
	}

	var alertsStr strings.Builder
	for _, alert := range activeAlerts {
		alertsStr.WriteString(fmt.Sprintf("- [%s] %s: %s\n", 
			alert.Severity, alert.MetricName, alert.Message))
	}

	return alertsStr.String()
}

// generateRecommendations generates optimization recommendations
func (cwv *CoreWebVitalsMonitor) generateRecommendations() string {
	var recommendations []string
	aggregates := cwv.aggregates

	// LCP recommendations
	if aggregates.LCP.P75 > cwv.thresholds.LCP.Good {
		recommendations = append(recommendations, 
			"• Optimize LCP: Improve server response times and optimize critical resources")
		if aggregates.LCP.P75 > 4000 {
			recommendations = append(recommendations, 
				"• Critical: LCP is very poor - consider image optimization and resource prioritization")
		}
	}

	// CLS recommendations
	if aggregates.CLS.P75 > cwv.thresholds.CLS.Good {
		recommendations = append(recommendations, 
			"• Reduce CLS: Add explicit dimensions to images and reserve space for dynamic content")
		if aggregates.CLS.P75 > 0.25 {
			recommendations = append(recommendations, 
				"• Critical: High layout shift - implement layout stability measures")
		}
	}

	// INP recommendations
	if aggregates.INP.P75 > cwv.thresholds.INP.Good {
		recommendations = append(recommendations, 
			"• Improve INP: Optimize JavaScript execution and reduce main thread blocking")
		if aggregates.INP.P75 > 500 {
			recommendations = append(recommendations, 
				"• Critical: Very slow interactions - implement progressive enhancement")
		}
	}

	// Device-specific recommendations
	if mobileLCP, mobileExists := aggregates.LCP.ByDevice["mobile"]; mobileExists {
		if desktopLCP, desktopExists := aggregates.LCP.ByDevice["desktop"]; desktopExists {
			if mobileLCP > desktopLCP*1.5 {
				recommendations = append(recommendations, 
					"• Mobile performance significantly worse than desktop - focus on mobile optimization")
			}
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "• All Core Web Vitals are performing well!")
	}

	return strings.Join(recommendations, "\n")
}

// HTTPHandler returns HTTP handlers for Core Web Vitals APIs
func (cwv *CoreWebVitalsMonitor) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cwv/report":
			cwv.handleReport(w, r)
		case "/cwv/aggregates":
			cwv.handleAggregates(w, r)
		case "/cwv/alerts":
			cwv.handleAlerts(w, r)
		case "/cwv/measurements":
			cwv.handleMeasurements(w, r)
		case "/cwv/record":
			cwv.handleRecord(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func (cwv *CoreWebVitalsMonitor) handleReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(cwv.GenerateReport()))
}

func (cwv *CoreWebVitalsMonitor) handleAggregates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cwv.GetAggregates())
}

func (cwv *CoreWebVitalsMonitor) handleAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cwv.GetAlerts())
}

func (cwv *CoreWebVitalsMonitor) handleMeasurements(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	measurements := cwv.GetMeasurements(100, 0) // Return latest 100 measurements
	json.NewEncoder(w).Encode(measurements)
}

func (cwv *CoreWebVitalsMonitor) handleRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var measurement CWVMeasurement
	if err := json.NewDecoder(r.Body).Decode(&measurement); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := cwv.RecordMeasurement(measurement); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "recorded"})
}

// StartReporting starts periodic reporting and aggregation
func (cwv *CoreWebVitalsMonitor) StartReporting(ctx context.Context) {
	ticker := time.NewTicker(cwv.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cwv.RecalculateAggregates()
		}
	}
}