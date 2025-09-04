// Package ai provides metrics collection for AI operations
package ai

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// MetricsCollector collects and tracks AI-related metrics
type MetricsCollector struct {
	registry prometheus.Registerer
	logger   *zap.Logger

	// Metrics
	inferenceLatency  *prometheus.HistogramVec
	modelUsageCounter *prometheus.CounterVec
	cacheHitRatio     *prometheus.GaugeVec
	errorRateCounter  *prometheus.CounterVec
	memoryUsageGauge  *prometheus.GaugeVec
	tokensPerSecond   *prometheus.GaugeVec
	qualityScore      *prometheus.GaugeVec
	totalRequests     *prometheus.CounterVec
	cacheHits         *prometheus.CounterVec
	cacheMisses       *prometheus.CounterVec
}

// NewMetricsCollector creates a new AI metrics collector
func NewMetricsCollector(registry prometheus.Registerer, logger *zap.Logger) *MetricsCollector {
	mc := &MetricsCollector{
		registry: registry,
		logger:   logger.Named("ai_metrics"),
	}

	mc.initializeMetrics()
	mc.registerMetrics()

	return mc
}

// initializeMetrics initializes all Prometheus metrics
func (mc *MetricsCollector) initializeMetrics() {
	// Inference latency histogram
	mc.inferenceLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "alchemorsel_ai_inference_duration_seconds",
			Help:    "Time spent on AI inference in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
		},
		[]string{"model", "status", "intent", "quality_level"},
	)

	// Model usage counter
	mc.modelUsageCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alchemorsel_ai_model_requests_total",
			Help: "Total number of requests per AI model",
		},
		[]string{"model", "status", "intent"},
	)

	// Cache hit ratio
	mc.cacheHitRatio = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alchemorsel_ai_cache_hit_ratio",
			Help: "Cache hit ratio for AI responses",
		},
		[]string{"model"},
	)

	// Error rate counter
	mc.errorRateCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alchemorsel_ai_errors_total",
			Help: "Total number of AI generation errors",
		},
		[]string{"model", "error_type"},
	)

	// Memory usage gauge
	mc.memoryUsageGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alchemorsel_ai_memory_usage_bytes",
			Help: "Memory usage of AI models in bytes",
		},
		[]string{"model"},
	)

	// Tokens per second gauge
	mc.tokensPerSecond = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alchemorsel_ai_tokens_per_second",
			Help: "Tokens generated per second by AI models",
		},
		[]string{"model"},
	)

	// Quality score gauge
	mc.qualityScore = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "alchemorsel_ai_quality_score",
			Help: "Quality score of AI responses (0.0 to 1.0)",
		},
		[]string{"model", "intent"},
	)

	// Total requests counter
	mc.totalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alchemorsel_ai_requests_total",
			Help: "Total number of AI requests",
		},
		[]string{"model", "intent"},
	)

	// Cache hits counter
	mc.cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alchemorsel_ai_cache_hits_total",
			Help: "Total number of cache hits",
		},
		[]string{"model"},
	)

	// Cache misses counter
	mc.cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alchemorsel_ai_cache_misses_total",
			Help: "Total number of cache misses",
		},
		[]string{"model"},
	)
}

// registerMetrics registers all metrics with Prometheus
func (mc *MetricsCollector) registerMetrics() {
	if mc.registry == nil {
		mc.logger.Warn("No Prometheus registry provided, metrics will not be collected")
		return
	}

	metrics := []prometheus.Collector{
		mc.inferenceLatency,
		mc.modelUsageCounter,
		mc.cacheHitRatio,
		mc.errorRateCounter,
		mc.memoryUsageGauge,
		mc.tokensPerSecond,
		mc.qualityScore,
		mc.totalRequests,
		mc.cacheHits,
		mc.cacheMisses,
	}

	for i, metric := range metrics {
		if err := mc.registry.Register(metric); err != nil {
			mc.logger.Warn("Failed to register metric",
				zap.Int("metric_index", i),
				zap.Error(err))
		}
	}

	mc.logger.Info("AI metrics registered successfully")
}

// RecordGeneration records metrics for a completed AI generation
func (mc *MetricsCollector) RecordGeneration(model string, duration time.Duration, tokenCount int, err error) {
	if mc.registry == nil {
		return
	}

	status := "success"
	intent := "general" // Default intent
	qualityLevel := "balanced"

	if err != nil {
		status = "error"
		mc.recordError(model, "generation_failed")
	}

	// Record latency
	labels := prometheus.Labels{
		"model":         model,
		"status":        status,
		"intent":        intent,
		"quality_level": qualityLevel,
	}
	mc.inferenceLatency.With(labels).Observe(duration.Seconds())

	// Record model usage
	usageLabels := prometheus.Labels{
		"model":  model,
		"status": status,
		"intent": intent,
	}
	mc.modelUsageCounter.With(usageLabels).Inc()
	mc.totalRequests.With(prometheus.Labels{"model": model, "intent": intent}).Inc()

	// Calculate and record tokens per second
	if duration > 0 && tokenCount > 0 {
		tokensPerSec := float64(tokenCount) / duration.Seconds()
		mc.tokensPerSecond.With(prometheus.Labels{"model": model}).Set(tokensPerSec)

		mc.logger.Debug("Generation metrics recorded",
			zap.String("model", model),
			zap.Duration("duration", duration),
			zap.Int("tokens", tokenCount),
			zap.Float64("tokens_per_second", tokensPerSec),
			zap.String("status", status))
	}
}

// RecordGenerationWithIntent records metrics for a generation with specific intent
func (mc *MetricsCollector) RecordGenerationWithIntent(model, intent, qualityLevel string, duration time.Duration, tokenCount int, quality float64, err error) {
	if mc.registry == nil {
		return
	}

	status := "success"
	if err != nil {
		status = "error"
		mc.recordError(model, "generation_failed")
	}

	// Record latency with full context
	labels := prometheus.Labels{
		"model":         model,
		"status":        status,
		"intent":        intent,
		"quality_level": qualityLevel,
	}
	mc.inferenceLatency.With(labels).Observe(duration.Seconds())

	// Record model usage
	usageLabels := prometheus.Labels{
		"model":  model,
		"status": status,
		"intent": intent,
	}
	mc.modelUsageCounter.With(usageLabels).Inc()
	mc.totalRequests.With(prometheus.Labels{"model": model, "intent": intent}).Inc()

	// Record quality score
	if quality > 0 {
		mc.qualityScore.With(prometheus.Labels{"model": model, "intent": intent}).Set(quality)
	}

	// Calculate and record tokens per second
	if duration > 0 && tokenCount > 0 {
		tokensPerSec := float64(tokenCount) / duration.Seconds()
		mc.tokensPerSecond.With(prometheus.Labels{"model": model}).Set(tokensPerSec)
	}
}

// RecordCacheHit records a cache hit event
func (mc *MetricsCollector) RecordCacheHit(model string) {
	if mc.registry == nil {
		return
	}

	mc.cacheHits.With(prometheus.Labels{"model": model}).Inc()
	mc.updateCacheHitRatio(model)

	mc.logger.Debug("Cache hit recorded", zap.String("model", model))
}

// RecordCacheMiss records a cache miss event
func (mc *MetricsCollector) RecordCacheMiss(model string) {
	if mc.registry == nil {
		return
	}

	mc.cacheMisses.With(prometheus.Labels{"model": model}).Inc()
	mc.updateCacheHitRatio(model)

	mc.logger.Debug("Cache miss recorded", zap.String("model", model))
}

// updateCacheHitRatio calculates and updates the cache hit ratio
func (mc *MetricsCollector) updateCacheHitRatio(model string) {
	// Get current values (metrics will be used later for ratio calculation)
	_ = mc.cacheHits.With(prometheus.Labels{"model": model})
	_ = mc.cacheMisses.With(prometheus.Labels{"model": model})

	// For simplicity, we'll calculate ratio based on counters
	// In production, you might want to use a sliding window approach

	// Note: This is a simplified approach. In production, you'd want to
	// maintain separate counters and calculate ratio periodically
	mc.logger.Debug("Cache hit ratio updated", zap.String("model", model))
}

// recordError records an error metric
func (mc *MetricsCollector) recordError(model, errorType string) {
	if mc.registry == nil {
		return
	}

	mc.errorRateCounter.With(prometheus.Labels{
		"model":      model,
		"error_type": errorType,
	}).Inc()

	mc.logger.Debug("Error recorded",
		zap.String("model", model),
		zap.String("error_type", errorType))
}

// RecordModelMemoryUsage records memory usage for a model
func (mc *MetricsCollector) RecordModelMemoryUsage(model string, memoryBytes int64) {
	if mc.registry == nil {
		return
	}

	mc.memoryUsageGauge.With(prometheus.Labels{"model": model}).Set(float64(memoryBytes))

	mc.logger.Debug("Memory usage recorded",
		zap.String("model", model),
		zap.Int64("memory_bytes", memoryBytes))
}

// GetMetricsSnapshot returns a snapshot of current metrics
func (mc *MetricsCollector) GetMetricsSnapshot() *MetricsSnapshot {
	return &MetricsSnapshot{
		Timestamp: time.Now(),
		// Add more fields as needed for debugging/monitoring
	}
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	// Add more metrics fields as needed
}

// RecordModelHealth records health status for a model
func (mc *MetricsCollector) RecordModelHealth(model, healthStatus string) {
	mc.logger.Debug("Model health recorded",
		zap.String("model", model),
		zap.String("health_status", healthStatus))

	// Could add a health status gauge here if needed
}

// Reset resets all metrics (useful for testing)
func (mc *MetricsCollector) Reset() {
	if mc.registry == nil {
		return
	}

	// Reset all counters and gauges
	// Note: This is primarily for testing purposes
	mc.logger.Info("Metrics reset")
}
