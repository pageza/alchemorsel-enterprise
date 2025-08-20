// Package performance provides Real User Monitoring (RUM) for Core Web Vitals
package performance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/cache"
)

// RUMSystem provides Real User Monitoring for Core Web Vitals
type RUMSystem struct {
	config               RUMConfig
	dataCollector        *DataCollector
	analyticsProcessor   *AnalyticsProcessor
	alertingSystem       *AlertingSystem
	sessionManager       *SessionManager
	cacheClient          *cache.RedisClient
	performanceMetrics   RUMMetrics
	mutex               sync.RWMutex
}

// RUMConfig configures Real User Monitoring
type RUMConfig struct {
	EnableRUM                bool              // Enable RUM collection
	SampleRate              float64           // Sampling rate (0.0-1.0)
	EnableSessionTracking   bool              // Track user sessions
	EnableDeviceDetection   bool              // Detect device characteristics
	EnableNetworkDetection  bool              // Detect network conditions
	MaxSessionDuration      time.Duration     // Maximum session duration
	DataRetentionPeriod     time.Duration     // How long to keep RUM data
	EnableRealTimeAlerts    bool              // Enable real-time alerting
	AlertThresholds         AlertThresholds   // Alert thresholds
	EnableHeatmaps          bool              // Enable interaction heatmaps
	EnableUserJourneys      bool              // Track user journeys
	EnablePerformanceAPI    bool              // Expose performance API
	BatchSize              int               // Batch size for data submission
	FlushInterval          time.Duration     // How often to flush data
}

// DataCollector handles RUM data collection
type DataCollector struct {
	measurementQueue     []RUMMeasurement
	batchProcessor       *BatchProcessor
	fieldConfig          FieldConfig
	userAgent           string
	sessionID           string
	pageViewID          string
}

// AnalyticsProcessor processes and analyzes RUM data
type AnalyticsProcessor struct {
	aggregationEngine    *AggregationEngine
	trendAnalyzer       *TrendAnalyzer
	segmentationEngine  *SegmentationEngine
	anomalyDetector     *AnomalyDetector
}

// AlertingSystem handles real-time alerting
type AlertingSystem struct {
	thresholds          AlertThresholds
	alertChannels       []AlertChannel
	escalationRules     []EscalationRule
	suppressionRules    []SuppressionRule
}

// SessionManager manages user sessions
type SessionManager struct {
	sessions            map[string]*UserSession
	sessionTimeout      time.Duration
	maxSessions         int
	sessionCleanup      time.Duration
}

// RUMMeasurement represents a single RUM measurement
type RUMMeasurement struct {
	ID              string                 `json:"id"`
	SessionID       string                 `json:"session_id"`
	PageViewID      string                 `json:"page_view_id"`
	UserID          string                 `json:"user_id,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
	URL             string                 `json:"url"`
	UserAgent       string                 `json:"user_agent"`
	DeviceInfo      DeviceInfo             `json:"device_info"`
	NetworkInfo     NetworkInfo            `json:"network_info"`
	PerformanceData PerformanceData        `json:"performance_data"`
	InteractionData []InteractionEvent     `json:"interaction_data"`
	ErrorData       []ErrorEvent           `json:"error_data"`
	CustomData      map[string]interface{} `json:"custom_data"`
}

// DeviceInfo contains device characteristics
type DeviceInfo struct {
	Type            string  `json:"type"` // mobile, tablet, desktop
	Model           string  `json:"model"`
	OS              string  `json:"os"`
	OSVersion       string  `json:"os_version"`
	Browser         string  `json:"browser"`
	BrowserVersion  string  `json:"browser_version"`
	ViewportWidth   int     `json:"viewport_width"`
	ViewportHeight  int     `json:"viewport_height"`
	ScreenWidth     int     `json:"screen_width"`
	ScreenHeight    int     `json:"screen_height"`
	PixelRatio      float64 `json:"pixel_ratio"`
	ColorDepth      int     `json:"color_depth"`
	TouchSupport    bool    `json:"touch_support"`
	Orientation     string  `json:"orientation"`
}

// NetworkInfo contains network characteristics
type NetworkInfo struct {
	Type            string  `json:"type"` // wifi, cellular, ethernet, unknown
	EffectiveType   string  `json:"effective_type"` // slow-2g, 2g, 3g, 4g
	Downlink        float64 `json:"downlink"`
	RTT             int     `json:"rtt"`
	SaveData        bool    `json:"save_data"`
	ConnectionSpeed string  `json:"connection_speed"`
}

// PerformanceData contains Core Web Vitals and other metrics
type PerformanceData struct {
	// Core Web Vitals
	LCP             float64 `json:"lcp"`
	CLS             float64 `json:"cls"`
	INP             float64 `json:"inp"`
	
	// Additional metrics
	FCP             float64 `json:"fcp"`
	TTFB            float64 `json:"ttfb"`
	DOMContentLoaded float64 `json:"dom_content_loaded"`
	LoadComplete    float64 `json:"load_complete"`
	
	// Custom metrics
	FirstInputDelay  float64 `json:"first_input_delay"`
	TimeToInteractive float64 `json:"time_to_interactive"`
	SpeedIndex      float64 `json:"speed_index"`
	
	// Resource metrics
	ResourceLoadTime map[string]float64 `json:"resource_load_time"`
	ResourceSize     map[string]int64   `json:"resource_size"`
	
	// Navigation metrics
	NavigationType   string  `json:"navigation_type"`
	RedirectCount    int     `json:"redirect_count"`
	RedirectTime     float64 `json:"redirect_time"`
	
	// Memory metrics
	UsedJSHeapSize   int64 `json:"used_js_heap_size"`
	TotalJSHeapSize  int64 `json:"total_js_heap_size"`
	JSHeapSizeLimit  int64 `json:"js_heap_size_limit"`
}

// InteractionEvent represents a user interaction
type InteractionEvent struct {
	Type        string    `json:"type"` // click, tap, key, scroll
	Target      string    `json:"target"`
	Timestamp   time.Time `json:"timestamp"`
	Duration    float64   `json:"duration"`
	X           int       `json:"x"`
	Y           int       `json:"y"`
	InputDelay  float64   `json:"input_delay"`
	ProcessTime float64   `json:"process_time"`
}

// ErrorEvent represents a JavaScript error
type ErrorEvent struct {
	Message     string    `json:"message"`
	Source      string    `json:"source"`
	Line        int       `json:"line"`
	Column      int       `json:"column"`
	Stack       string    `json:"stack"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
}

// UserSession represents a user session
type UserSession struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"user_id"`
	StartTime     time.Time              `json:"start_time"`
	LastActivity  time.Time              `json:"last_activity"`
	PageViews     []PageView             `json:"page_views"`
	DeviceInfo    DeviceInfo             `json:"device_info"`
	NetworkInfo   NetworkInfo            `json:"network_info"`
	Measurements  []RUMMeasurement       `json:"measurements"`
	CustomData    map[string]interface{} `json:"custom_data"`
}

// PageView represents a page view within a session
type PageView struct {
	ID            string              `json:"id"`
	URL           string              `json:"url"`
	Title         string              `json:"title"`
	Timestamp     time.Time           `json:"timestamp"`
	LoadTime      float64             `json:"load_time"`
	TimeOnPage    time.Duration       `json:"time_on_page"`
	ScrollDepth   float64             `json:"scroll_depth"`
	Interactions  []InteractionEvent  `json:"interactions"`
	ExitType      string              `json:"exit_type"`
}

// BatchProcessor handles batched data submission
type BatchProcessor struct {
	batchSize     int
	flushInterval time.Duration
	pendingData   []RUMMeasurement
	lastFlush     time.Time
}

// FieldConfig configures which fields to collect
type FieldConfig struct {
	CoreWebVitals     bool
	ResourceTiming    bool
	UserInteractions  bool
	ErrorTracking     bool
	CustomMetrics     bool
	DeviceInfo        bool
	NetworkInfo       bool
	MemoryInfo        bool
}

// AggregationEngine aggregates RUM data
type AggregationEngine struct {
	timeWindows    []time.Duration
	aggregateBy    []string
	calculations   []string
}

// TrendAnalyzer analyzes performance trends
type TrendAnalyzer struct {
	lookbackPeriod time.Duration
	trendTypes     []string
	seasonality    bool
}

// SegmentationEngine segments users and sessions
type SegmentationEngine struct {
	segments       []Segment
	customFilters  []Filter
}

// AnomalyDetector detects performance anomalies
type AnomalyDetector struct {
	algorithms     []string
	sensitivity    float64
	minDataPoints  int
}

// Segment represents a user segment
type Segment struct {
	Name      string
	Filters   []Filter
	Count     int
	Metrics   map[string]float64
}

// Filter represents a data filter
type Filter struct {
	Field    string
	Operator string
	Value    interface{}
}

// AlertThresholds defines alerting thresholds
type AlertThresholds struct {
	LCP  ThresholdConfig `json:"lcp"`
	CLS  ThresholdConfig `json:"cls"`
	INP  ThresholdConfig `json:"inp"`
	FCP  ThresholdConfig `json:"fcp"`
	TTFB ThresholdConfig `json:"ttfb"`
}

// ThresholdConfig configures a metric threshold
type ThresholdConfig struct {
	Warning   float64 `json:"warning"`
	Critical  float64 `json:"critical"`
	Enabled   bool    `json:"enabled"`
	Window    string  `json:"window"`
	MinSamples int    `json:"min_samples"`
}

// AlertChannel represents an alert destination
type AlertChannel struct {
	Type   string
	Config map[string]interface{}
}

// EscalationRule defines alert escalation
type EscalationRule struct {
	Condition string
	Delay     time.Duration
	Channel   string
}

// SuppressionRule defines alert suppression
type SuppressionRule struct {
	Condition string
	Duration  time.Duration
}

// RUMMetrics tracks RUM system performance
type RUMMetrics struct {
	TotalMeasurements    int64
	ProcessedMeasurements int64
	DroppedMeasurements  int64
	ActiveSessions       int
	TotalSessions        int64
	AverageSessionLength time.Duration
	DataVolume           int64
	ProcessingLatency    time.Duration
	AlertsTriggered      int64
	LastUpdate           time.Time
}

// DefaultRUMConfig returns sensible RUM defaults
func DefaultRUMConfig() RUMConfig {
	return RUMConfig{
		EnableRUM:              true,
		SampleRate:            0.1, // Sample 10% of users
		EnableSessionTracking: true,
		EnableDeviceDetection: true,
		EnableNetworkDetection: true,
		MaxSessionDuration:    30 * time.Minute,
		DataRetentionPeriod:   30 * 24 * time.Hour, // 30 days
		EnableRealTimeAlerts:  true,
		AlertThresholds: AlertThresholds{
			LCP: ThresholdConfig{
				Warning:   2500,
				Critical:  4000,
				Enabled:   true,
				Window:    "5m",
				MinSamples: 10,
			},
			CLS: ThresholdConfig{
				Warning:   0.1,
				Critical:  0.25,
				Enabled:   true,
				Window:    "5m",
				MinSamples: 10,
			},
			INP: ThresholdConfig{
				Warning:   200,
				Critical:  500,
				Enabled:   true,
				Window:    "5m",
				MinSamples: 10,
			},
		},
		EnableHeatmaps:      true,
		EnableUserJourneys:  true,
		EnablePerformanceAPI: true,
		BatchSize:          50,
		FlushInterval:      30 * time.Second,
	}
}

// NewRUMSystem creates a new RUM system
func NewRUMSystem(config RUMConfig) *RUMSystem {
	dataCollector := &DataCollector{
		measurementQueue: []RUMMeasurement{},
		batchProcessor: &BatchProcessor{
			batchSize:     config.BatchSize,
			flushInterval: config.FlushInterval,
			pendingData:   []RUMMeasurement{},
		},
		fieldConfig: FieldConfig{
			CoreWebVitals:    true,
			ResourceTiming:   true,
			UserInteractions: true,
			ErrorTracking:    true,
			CustomMetrics:    true,
			DeviceInfo:       config.EnableDeviceDetection,
			NetworkInfo:      config.EnableNetworkDetection,
			MemoryInfo:       true,
		},
	}

	analyticsProcessor := &AnalyticsProcessor{
		aggregationEngine: &AggregationEngine{
			timeWindows:  []time.Duration{time.Minute, 5 * time.Minute, time.Hour, 24 * time.Hour},
			aggregateBy:  []string{"device_type", "browser", "country", "page"},
			calculations: []string{"p50", "p75", "p90", "p95", "p99", "mean", "count"},
		},
		trendAnalyzer: &TrendAnalyzer{
			lookbackPeriod: 7 * 24 * time.Hour,
			trendTypes:     []string{"improving", "degrading", "stable"},
			seasonality:    true,
		},
		segmentationEngine: &SegmentationEngine{
			segments: []Segment{},
		},
		anomalyDetector: &AnomalyDetector{
			algorithms:    []string{"statistical", "ml-based"},
			sensitivity:   0.8,
			minDataPoints: 30,
		},
	}

	alertingSystem := &AlertingSystem{
		thresholds: config.AlertThresholds,
		alertChannels: []AlertChannel{
			{Type: "webhook", Config: map[string]interface{}{"url": "/api/alerts"}},
		},
	}

	sessionManager := &SessionManager{
		sessions:       make(map[string]*UserSession),
		sessionTimeout: config.MaxSessionDuration,
		maxSessions:    10000,
		sessionCleanup: 5 * time.Minute,
	}

	return &RUMSystem{
		config:             config,
		dataCollector:      dataCollector,
		analyticsProcessor: analyticsProcessor,
		alertingSystem:     alertingSystem,
		sessionManager:     sessionManager,
		performanceMetrics: RUMMetrics{},
	}
}

// CollectMeasurement collects a RUM measurement
func (rum *RUMSystem) CollectMeasurement(measurement RUMMeasurement) error {
	if !rum.config.EnableRUM {
		return nil
	}

	// Apply sampling
	if !rum.shouldSample() {
		return nil
	}

	rum.mutex.Lock()
	defer rum.mutex.Unlock()

	// Validate measurement
	if err := rum.validateMeasurement(measurement); err != nil {
		rum.performanceMetrics.DroppedMeasurements++
		return fmt.Errorf("invalid measurement: %w", err)
	}

	// Add to collection queue
	rum.dataCollector.measurementQueue = append(rum.dataCollector.measurementQueue, measurement)

	// Update session
	if rum.config.EnableSessionTracking {
		rum.updateSession(measurement)
	}

	// Process for real-time alerts
	if rum.config.EnableRealTimeAlerts {
		rum.checkRealTimeAlerts(measurement)
	}

	// Update metrics
	rum.performanceMetrics.TotalMeasurements++
	rum.performanceMetrics.LastUpdate = time.Now()

	// Flush if batch is full
	if len(rum.dataCollector.measurementQueue) >= rum.config.BatchSize {
		return rum.flushMeasurements()
	}

	return nil
}

// shouldSample determines if this measurement should be collected
func (rum *RUMSystem) shouldSample() bool {
	// Simple random sampling
	return time.Now().UnixNano()%100 < int64(rum.config.SampleRate*100)
}

// validateMeasurement validates a RUM measurement
func (rum *RUMSystem) validateMeasurement(measurement RUMMeasurement) error {
	if measurement.URL == "" {
		return fmt.Errorf("URL is required")
	}

	if measurement.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}

	// Validate Core Web Vitals ranges
	if measurement.PerformanceData.LCP < 0 || measurement.PerformanceData.LCP > 60000 {
		return fmt.Errorf("LCP value out of range: %f", measurement.PerformanceData.LCP)
	}

	if measurement.PerformanceData.CLS < 0 || measurement.PerformanceData.CLS > 10 {
		return fmt.Errorf("CLS value out of range: %f", measurement.PerformanceData.CLS)
	}

	if measurement.PerformanceData.INP < 0 || measurement.PerformanceData.INP > 10000 {
		return fmt.Errorf("INP value out of range: %f", measurement.PerformanceData.INP)
	}

	return nil
}

// updateSession updates the user session with new measurement
func (rum *RUMSystem) updateSession(measurement RUMMeasurement) {
	sessionID := measurement.SessionID
	if sessionID == "" {
		return
	}

	session, exists := rum.sessionManager.sessions[sessionID]
	if !exists {
		// Create new session
		session = &UserSession{
			ID:           sessionID,
			UserID:       measurement.UserID,
			StartTime:    measurement.Timestamp,
			LastActivity: measurement.Timestamp,
			PageViews:    []PageView{},
			DeviceInfo:   measurement.DeviceInfo,
			NetworkInfo:  measurement.NetworkInfo,
			Measurements: []RUMMeasurement{},
			CustomData:   make(map[string]interface{}),
		}
		rum.sessionManager.sessions[sessionID] = session
		rum.performanceMetrics.TotalSessions++
	}

	// Update session
	session.LastActivity = measurement.Timestamp
	session.Measurements = append(session.Measurements, measurement)

	// Update page view if new page
	if rum.isNewPageView(session, measurement.URL) {
		pageView := PageView{
			ID:        measurement.PageViewID,
			URL:       measurement.URL,
			Timestamp: measurement.Timestamp,
		}
		session.PageViews = append(session.PageViews, pageView)
	}

	// Update active sessions count
	rum.updateActiveSessionsCount()
}

// isNewPageView determines if this is a new page view
func (rum *RUMSystem) isNewPageView(session *UserSession, url string) bool {
	if len(session.PageViews) == 0 {
		return true
	}
	
	lastPageView := session.PageViews[len(session.PageViews)-1]
	return lastPageView.URL != url
}

// updateActiveSessionsCount updates the count of active sessions
func (rum *RUMSystem) updateActiveSessionsCount() {
	cutoff := time.Now().Add(-rum.sessionManager.sessionTimeout)
	activeCount := 0
	
	for _, session := range rum.sessionManager.sessions {
		if session.LastActivity.After(cutoff) {
			activeCount++
		}
	}
	
	rum.performanceMetrics.ActiveSessions = activeCount
}

// checkRealTimeAlerts checks for real-time alert conditions
func (rum *RUMSystem) checkRealTimeAlerts(measurement RUMMeasurement) {
	perfData := measurement.PerformanceData

	// Check LCP threshold
	if rum.alertingSystem.thresholds.LCP.Enabled {
		if perfData.LCP > rum.alertingSystem.thresholds.LCP.Critical {
			rum.triggerAlert("LCP", "critical", perfData.LCP, measurement)
		} else if perfData.LCP > rum.alertingSystem.thresholds.LCP.Warning {
			rum.triggerAlert("LCP", "warning", perfData.LCP, measurement)
		}
	}

	// Check CLS threshold
	if rum.alertingSystem.thresholds.CLS.Enabled {
		if perfData.CLS > rum.alertingSystem.thresholds.CLS.Critical {
			rum.triggerAlert("CLS", "critical", perfData.CLS, measurement)
		} else if perfData.CLS > rum.alertingSystem.thresholds.CLS.Warning {
			rum.triggerAlert("CLS", "warning", perfData.CLS, measurement)
		}
	}

	// Check INP threshold
	if rum.alertingSystem.thresholds.INP.Enabled {
		if perfData.INP > rum.alertingSystem.thresholds.INP.Critical {
			rum.triggerAlert("INP", "critical", perfData.INP, measurement)
		} else if perfData.INP > rum.alertingSystem.thresholds.INP.Warning {
			rum.triggerAlert("INP", "warning", perfData.INP, measurement)
		}
	}
}

// triggerAlert triggers a performance alert
func (rum *RUMSystem) triggerAlert(metric string, severity string, value float64, measurement RUMMeasurement) {
	alert := Alert{
		ID:        fmt.Sprintf("%s_%s_%d", metric, severity, time.Now().UnixNano()),
		Metric:    metric,
		Severity:  severity,
		Value:     value,
		Threshold: rum.getThresholdValue(metric, severity),
		URL:       measurement.URL,
		UserAgent: measurement.UserAgent,
		Timestamp: time.Now(),
		SessionID: measurement.SessionID,
	}

	// Send alert through configured channels
	for _, channel := range rum.alertingSystem.alertChannels {
		rum.sendAlert(alert, channel)
	}

	rum.performanceMetrics.AlertsTriggered++
}

// Alert represents a performance alert
type Alert struct {
	ID        string    `json:"id"`
	Metric    string    `json:"metric"`
	Severity  string    `json:"severity"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	URL       string    `json:"url"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
}

// getThresholdValue gets the threshold value for a metric and severity
func (rum *RUMSystem) getThresholdValue(metric string, severity string) float64 {
	switch metric {
	case "LCP":
		if severity == "critical" {
			return rum.alertingSystem.thresholds.LCP.Critical
		}
		return rum.alertingSystem.thresholds.LCP.Warning
	case "CLS":
		if severity == "critical" {
			return rum.alertingSystem.thresholds.CLS.Critical
		}
		return rum.alertingSystem.thresholds.CLS.Warning
	case "INP":
		if severity == "critical" {
			return rum.alertingSystem.thresholds.INP.Critical
		}
		return rum.alertingSystem.thresholds.INP.Warning
	default:
		return 0
	}
}

// sendAlert sends an alert through the specified channel
func (rum *RUMSystem) sendAlert(alert Alert, channel AlertChannel) {
	// Implementation would depend on channel type (webhook, email, slack, etc.)
	switch channel.Type {
	case "webhook":
		// Send webhook
	case "email":
		// Send email
	case "slack":
		// Send Slack message
	}
}

// flushMeasurements flushes pending measurements
func (rum *RUMSystem) flushMeasurements() error {
	if len(rum.dataCollector.measurementQueue) == 0 {
		return nil
	}

	// Process measurements
	measurements := make([]RUMMeasurement, len(rum.dataCollector.measurementQueue))
	copy(measurements, rum.dataCollector.measurementQueue)

	// Clear queue
	rum.dataCollector.measurementQueue = rum.dataCollector.measurementQueue[:0]

	// Process asynchronously
	go func() {
		rum.processMeasurements(measurements)
	}()

	rum.performanceMetrics.ProcessedMeasurements += int64(len(measurements))
	return nil
}

// processMeasurements processes a batch of measurements
func (rum *RUMSystem) processMeasurements(measurements []RUMMeasurement) {
	startTime := time.Now()

	// Aggregate data
	rum.analyticsProcessor.aggregationEngine.aggregate(measurements)

	// Analyze trends
	rum.analyticsProcessor.trendAnalyzer.analyze(measurements)

	// Detect anomalies
	rum.analyticsProcessor.anomalyDetector.detect(measurements)

	// Update processing latency
	rum.performanceMetrics.ProcessingLatency = time.Since(startTime)
}

// GetAnalytics returns analytics data for a time period
func (rum *RUMSystem) GetAnalytics(start, end time.Time, filters map[string]interface{}) (*AnalyticsResult, error) {
	rum.mutex.RLock()
	defer rum.mutex.RUnlock()

	// Filter measurements by time range and criteria
	var filteredMeasurements []RUMMeasurement
	for _, measurement := range rum.dataCollector.measurementQueue {
		if measurement.Timestamp.After(start) && measurement.Timestamp.Before(end) {
			if rum.matchesFilters(measurement, filters) {
				filteredMeasurements = append(filteredMeasurements, measurement)
			}
		}
	}

	// Calculate analytics
	return rum.calculateAnalytics(filteredMeasurements), nil
}

// AnalyticsResult represents analytics results
type AnalyticsResult struct {
	TimeRange    TimeRange             `json:"time_range"`
	TotalSamples int                   `json:"total_samples"`
	Metrics      map[string]MetricData `json:"metrics"`
	Segments     []SegmentData         `json:"segments"`
	Trends       []TrendData           `json:"trends"`
	Heatmaps     []HeatmapData         `json:"heatmaps"`
}

// MetricData represents metric analytics
type MetricData struct {
	P50    float64 `json:"p50"`
	P75    float64 `json:"p75"`
	P90    float64 `json:"p90"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	Mean   float64 `json:"mean"`
	Count  int     `json:"count"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

// SegmentData represents segment analytics
type SegmentData struct {
	Name    string             `json:"name"`
	Count   int                `json:"count"`
	Metrics map[string]float64 `json:"metrics"`
}

// TrendData represents trend analytics
type TrendData struct {
	Metric    string      `json:"metric"`
	Direction string      `json:"direction"`
	Change    float64     `json:"change"`
	Points    []DataPoint `json:"points"`
}

// HeatmapData represents heatmap analytics
type HeatmapData struct {
	URL    string           `json:"url"`
	Points []HeatmapPoint   `json:"points"`
}

// DataPoint represents a single data point
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// HeatmapPoint represents a heatmap point
type HeatmapPoint struct {
	X         int     `json:"x"`
	Y         int     `json:"y"`
	Intensity float64 `json:"intensity"`
	Count     int     `json:"count"`
}

// matchesFilters checks if a measurement matches the given filters
func (rum *RUMSystem) matchesFilters(measurement RUMMeasurement, filters map[string]interface{}) bool {
	for key, value := range filters {
		switch key {
		case "device_type":
			if measurement.DeviceInfo.Type != value.(string) {
				return false
			}
		case "browser":
			if measurement.DeviceInfo.Browser != value.(string) {
				return false
			}
		case "url":
			if measurement.URL != value.(string) {
				return false
			}
		}
	}
	return true
}

// calculateAnalytics calculates analytics from measurements
func (rum *RUMSystem) calculateAnalytics(measurements []RUMMeasurement) *AnalyticsResult {
	if len(measurements) == 0 {
		return &AnalyticsResult{
			TotalSamples: 0,
			Metrics:      make(map[string]MetricData),
		}
	}

	result := &AnalyticsResult{
		TimeRange: TimeRange{
			Start: measurements[0].Timestamp,
			End:   measurements[len(measurements)-1].Timestamp,
		},
		TotalSamples: len(measurements),
		Metrics:      make(map[string]MetricData),
	}

	// Calculate metrics for each Core Web Vital
	result.Metrics["LCP"] = rum.calculateMetricData(measurements, "LCP")
	result.Metrics["CLS"] = rum.calculateMetricData(measurements, "CLS")
	result.Metrics["INP"] = rum.calculateMetricData(measurements, "INP")
	result.Metrics["FCP"] = rum.calculateMetricData(measurements, "FCP")
	result.Metrics["TTFB"] = rum.calculateMetricData(measurements, "TTFB")

	return result
}

// calculateMetricData calculates metric data for a specific metric
func (rum *RUMSystem) calculateMetricData(measurements []RUMMeasurement, metric string) MetricData {
	var values []float64

	for _, measurement := range measurements {
		var value float64
		switch metric {
		case "LCP":
			value = measurement.PerformanceData.LCP
		case "CLS":
			value = measurement.PerformanceData.CLS
		case "INP":
			value = measurement.PerformanceData.INP
		case "FCP":
			value = measurement.PerformanceData.FCP
		case "TTFB":
			value = measurement.PerformanceData.TTFB
		}
		
		if value > 0 {
			values = append(values, value)
		}
	}

	if len(values) == 0 {
		return MetricData{}
	}

	sort.Float64s(values)

	return MetricData{
		P50:   percentile(values, 0.5),
		P75:   percentile(values, 0.75),
		P90:   percentile(values, 0.9),
		P95:   percentile(values, 0.95),
		P99:   percentile(values, 0.99),
		Mean:  mean(values),
		Count: len(values),
		Min:   values[0],
		Max:   values[len(values)-1],
	}
}

// percentile calculates percentile from sorted values
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

// mean calculates the mean of values
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	
	return sum / float64(len(values))
}

// StartCollection starts RUM data collection
func (rum *RUMSystem) StartCollection(ctx context.Context) {
	// Start background processes
	go rum.sessionCleanup(ctx)
	go rum.dataFlushScheduler(ctx)
	go rum.metricsCollector(ctx)
}

// sessionCleanup cleans up expired sessions
func (rum *RUMSystem) sessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(rum.sessionManager.sessionCleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rum.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredSessions removes expired sessions
func (rum *RUMSystem) cleanupExpiredSessions() {
	rum.mutex.Lock()
	defer rum.mutex.Unlock()

	cutoff := time.Now().Add(-rum.sessionManager.sessionTimeout)
	
	for sessionID, session := range rum.sessionManager.sessions {
		if session.LastActivity.Before(cutoff) {
			delete(rum.sessionManager.sessions, sessionID)
		}
	}
}

// dataFlushScheduler schedules periodic data flushes
func (rum *RUMSystem) dataFlushScheduler(ctx context.Context) {
	ticker := time.NewTicker(rum.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rum.flushMeasurements()
		}
	}
}

// metricsCollector updates system metrics
func (rum *RUMSystem) metricsCollector(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rum.updateSystemMetrics()
		}
	}
}

// updateSystemMetrics updates internal system metrics
func (rum *RUMSystem) updateSystemMetrics() {
	rum.mutex.Lock()
	defer rum.mutex.Unlock()

	// Calculate average session length
	if len(rum.sessionManager.sessions) > 0 {
		totalDuration := time.Duration(0)
		activeSessions := 0
		
		for _, session := range rum.sessionManager.sessions {
			if !session.LastActivity.IsZero() {
				duration := session.LastActivity.Sub(session.StartTime)
				totalDuration += duration
				activeSessions++
			}
		}
		
		if activeSessions > 0 {
			rum.performanceMetrics.AverageSessionLength = totalDuration / time.Duration(activeSessions)
		}
	}

	// Calculate data volume
	dataSize := int64(0)
	for _, measurement := range rum.dataCollector.measurementQueue {
		// Rough estimate of measurement size
		dataSize += 1024 // 1KB per measurement estimate
	}
	rum.performanceMetrics.DataVolume = dataSize
}

// HTTPHandler returns HTTP handlers for RUM APIs
func (rum *RUMSystem) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rum/collect":
			rum.handleCollect(w, r)
		case "/rum/analytics":
			rum.handleAnalytics(w, r)
		case "/rum/sessions":
			rum.handleSessions(w, r)
		case "/rum/metrics":
			rum.handleMetrics(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func (rum *RUMSystem) handleCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var measurement RUMMeasurement
	if err := json.NewDecoder(r.Body).Decode(&measurement); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := rum.CollectMeasurement(measurement); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "collected"})
}

func (rum *RUMSystem) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for time range and filters
	start := time.Now().Add(-24 * time.Hour) // Default to last 24 hours
	end := time.Now()
	filters := make(map[string]interface{})

	analytics, err := rum.GetAnalytics(start, end, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

func (rum *RUMSystem) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rum.sessionManager.sessions)
}

func (rum *RUMSystem) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rum.performanceMetrics)
}

// GetClientScript returns the client-side RUM collection script
func (rum *RUMSystem) GetClientScript() string {
	return `
<script>
// Real User Monitoring (RUM) Client
(function() {
  'use strict';
  
  class RUMCollector {
    constructor() {
      this.sessionID = this.generateSessionID();
      this.pageViewID = this.generatePageViewID();
      this.measurements = [];
      this.observers = [];
      this.config = {
        sampleRate: 0.1,
        batchSize: 10,
        flushInterval: 30000,
        endpoint: '/api/rum/collect'
      };
      
      this.init();
    }
    
    init() {
      this.setupPerformanceObserver();
      this.setupUserInteractionTracking();
      this.setupErrorTracking();
      this.collectInitialMetrics();
      this.scheduleFlush();
    }
    
    generateSessionID() {
      return 'session_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
    }
    
    generatePageViewID() {
      return 'pageview_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
    }
    
    setupPerformanceObserver() {
      if (!('PerformanceObserver' in window)) return;
      
      // Core Web Vitals observer
      try {
        const observer = new PerformanceObserver((entryList) => {
          entryList.getEntries().forEach(entry => {
            this.collectPerformanceEntry(entry);
          });
        });
        
        observer.observe({ 
          entryTypes: ['largest-contentful-paint', 'layout-shift', 'first-input', 'navigation', 'paint']
        });
        
        this.observers.push(observer);
      } catch (e) {
        console.warn('Performance Observer not fully supported');
      }
    }
    
    collectPerformanceEntry(entry) {
      const measurement = {
        id: this.generateMeasurementID(),
        session_id: this.sessionID,
        page_view_id: this.pageViewID,
        timestamp: new Date().toISOString(),
        url: window.location.href,
        user_agent: navigator.userAgent,
        device_info: this.getDeviceInfo(),
        network_info: this.getNetworkInfo(),
        performance_data: this.getPerformanceData(entry),
        interaction_data: [],
        error_data: [],
        custom_data: {}
      };
      
      this.addMeasurement(measurement);
    }
    
    getDeviceInfo() {
      return {
        type: this.getDeviceType(),
        viewport_width: window.innerWidth,
        viewport_height: window.innerHeight,
        screen_width: screen.width,
        screen_height: screen.height,
        pixel_ratio: window.devicePixelRatio || 1,
        touch_support: 'ontouchstart' in window,
        orientation: screen.orientation ? screen.orientation.type : 'unknown'
      };
    }
    
    getDeviceType() {
      const width = window.innerWidth;
      if (width <= 768) return 'mobile';
      if (width <= 1024) return 'tablet';
      return 'desktop';
    }
    
    getNetworkInfo() {
      const connection = navigator.connection || navigator.mozConnection || navigator.webkitConnection;
      
      if (connection) {
        return {
          type: connection.type || 'unknown',
          effective_type: connection.effectiveType || 'unknown',
          downlink: connection.downlink || 0,
          rtt: connection.rtt || 0,
          save_data: connection.saveData || false
        };
      }
      
      return {
        type: 'unknown',
        effective_type: 'unknown',
        downlink: 0,
        rtt: 0,
        save_data: false
      };
    }
    
    getPerformanceData(entry) {
      const data = {
        lcp: 0,
        cls: 0,
        inp: 0,
        fcp: 0,
        ttfb: 0
      };
      
      if (entry.entryType === 'largest-contentful-paint') {
        data.lcp = entry.startTime;
      } else if (entry.entryType === 'layout-shift' && !entry.hadRecentInput) {
        data.cls = entry.value;
      } else if (entry.entryType === 'first-input') {
        data.inp = entry.processingStart - entry.startTime;
      } else if (entry.entryType === 'paint') {
        if (entry.name === 'first-contentful-paint') {
          data.fcp = entry.startTime;
        }
      } else if (entry.entryType === 'navigation') {
        data.ttfb = entry.responseStart - entry.fetchStart;
      }
      
      return data;
    }
    
    setupUserInteractionTracking() {
      const interactionTypes = ['click', 'keydown', 'scroll'];
      
      interactionTypes.forEach(type => {
        document.addEventListener(type, (event) => {
          this.recordInteraction(event);
        }, { passive: true });
      });
    }
    
    recordInteraction(event) {
      const interaction = {
        type: event.type,
        target: this.getElementSelector(event.target),
        timestamp: new Date().toISOString(),
        x: event.clientX || 0,
        y: event.clientY || 0
      };
      
      // Add to current measurement or create new one
      this.addInteractionData(interaction);
    }
    
    getElementSelector(element) {
      if (element.id) return '#' + element.id;
      if (element.className) return '.' + element.className.split(' ')[0];
      return element.tagName.toLowerCase();
    }
    
    setupErrorTracking() {
      window.addEventListener('error', (event) => {
        this.recordError({
          message: event.message,
          source: event.filename,
          line: event.lineno,
          column: event.colno,
          stack: event.error ? event.error.stack : '',
          timestamp: new Date().toISOString(),
          type: 'javascript'
        });
      });
      
      window.addEventListener('unhandledrejection', (event) => {
        this.recordError({
          message: event.reason.toString(),
          source: 'promise',
          timestamp: new Date().toISOString(),
          type: 'unhandled_promise'
        });
      });
    }
    
    recordError(error) {
      this.addErrorData(error);
    }
    
    collectInitialMetrics() {
      // Collect initial page load metrics
      if (performance.timing) {
        const timing = performance.timing;
        const measurement = {
          id: this.generateMeasurementID(),
          session_id: this.sessionID,
          page_view_id: this.pageViewID,
          timestamp: new Date().toISOString(),
          url: window.location.href,
          user_agent: navigator.userAgent,
          device_info: this.getDeviceInfo(),
          network_info: this.getNetworkInfo(),
          performance_data: {
            dom_content_loaded: timing.domContentLoadedEventEnd - timing.navigationStart,
            load_complete: timing.loadEventEnd - timing.navigationStart,
            ttfb: timing.responseStart - timing.fetchStart
          },
          interaction_data: [],
          error_data: [],
          custom_data: {}
        };
        
        this.addMeasurement(measurement);
      }
    }
    
    generateMeasurementID() {
      return 'measurement_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
    }
    
    addMeasurement(measurement) {
      this.measurements.push(measurement);
      
      if (this.measurements.length >= this.config.batchSize) {
        this.flush();
      }
    }
    
    addInteractionData(interaction) {
      // Add to the most recent measurement or create new one
      if (this.measurements.length > 0) {
        const lastMeasurement = this.measurements[this.measurements.length - 1];
        lastMeasurement.interaction_data.push(interaction);
      }
    }
    
    addErrorData(error) {
      // Add to the most recent measurement or create new one
      if (this.measurements.length > 0) {
        const lastMeasurement = this.measurements[this.measurements.length - 1];
        lastMeasurement.error_data.push(error);
      }
    }
    
    flush() {
      if (this.measurements.length === 0) return;
      
      const batch = this.measurements.splice(0);
      
      // Send to server
      fetch(this.config.endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ measurements: batch }),
        keepalive: true
      }).catch(error => {
        console.warn('RUM data submission failed:', error);
      });
    }
    
    scheduleFlush() {
      setInterval(() => {
        this.flush();
      }, this.config.flushInterval);
      
      // Flush on page unload
      window.addEventListener('beforeunload', () => {
        this.flush();
      });
    }
  }
  
  // Initialize RUM collector
  if (Math.random() < 0.1) { // 10% sampling
    window.rumCollector = new RUMCollector();
  }
})();
</script>`
}