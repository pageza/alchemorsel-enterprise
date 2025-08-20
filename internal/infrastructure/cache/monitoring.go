// Package cache provides comprehensive cache monitoring and metrics for performance tracking
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CacheMonitor provides comprehensive cache monitoring and metrics collection
type CacheMonitor struct {
	cache         *CacheService
	redis         *RedisClient
	config        *MonitoringConfig
	logger        *zap.Logger
	metrics       *AggregatedMetrics
	alerts        *AlertManager
	mu            sync.RWMutex
	stopChan      chan struct{}
	isRunning     bool
}

// MonitoringConfig configures cache monitoring behavior
type MonitoringConfig struct {
	// Collection intervals
	MetricsInterval     time.Duration `json:"metrics_interval"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	AlertCheckInterval  time.Duration `json:"alert_check_interval"`
	
	// Performance thresholds
	HitRatioThreshold      float64       `json:"hit_ratio_threshold"`
	AvgResponseThreshold   time.Duration `json:"avg_response_threshold"`
	ErrorRateThreshold     float64       `json:"error_rate_threshold"`
	MemoryUsageThreshold   float64       `json:"memory_usage_threshold"`
	
	// Alert settings
	EnableAlerts           bool          `json:"enable_alerts"`
	AlertCooldown          time.Duration `json:"alert_cooldown"`
	MaxAlertHistory        int           `json:"max_alert_history"`
	
	// Retention settings
	MetricsRetention       time.Duration `json:"metrics_retention"`
	DetailedMetrics        bool          `json:"detailed_metrics"`
	ExportMetrics          bool          `json:"export_metrics"`
	
	// Performance monitoring
	SlowQueryThreshold     time.Duration `json:"slow_query_threshold"`
	TrackHotKeys          bool          `json:"track_hot_keys"`
	TrackKeyPatterns      bool          `json:"track_key_patterns"`
}

// AggregatedMetrics contains comprehensive cache metrics
type AggregatedMetrics struct {
	// Time series metrics
	Timestamp              time.Time     `json:"timestamp"`
	
	// Hit/Miss statistics
	TotalOperations        int64         `json:"total_operations"`
	TotalHits              int64         `json:"total_hits"`
	TotalMisses            int64         `json:"total_misses"`
	HitRatio               float64       `json:"hit_ratio"`
	
	// Performance metrics
	AvgResponseTime        time.Duration `json:"avg_response_time"`
	P95ResponseTime        time.Duration `json:"p95_response_time"`
	P99ResponseTime        time.Duration `json:"p99_response_time"`
	SlowOperations         int64         `json:"slow_operations"`
	
	// Error metrics
	TotalErrors            int64         `json:"total_errors"`
	ErrorRate              float64       `json:"error_rate"`
	ConnectionErrors       int64         `json:"connection_errors"`
	TimeoutErrors          int64         `json:"timeout_errors"`
	
	// Memory and storage
	MemoryUsage            int64         `json:"memory_usage"`
	KeyCount               int64         `json:"key_count"`
	ExpiringKeys           int64         `json:"expiring_keys"`
	StorageEfficiency      float64       `json:"storage_efficiency"`
	
	// Redis-specific metrics
	RedisInfo              *RedisInfo    `json:"redis_info"`
	
	// Service-specific metrics
	RecipeCacheStats       *ServiceStats `json:"recipe_cache_stats"`
	SessionCacheStats      *ServiceStats `json:"session_cache_stats"`
	AICacheStats           *ServiceStats `json:"ai_cache_stats"`
	TemplateCacheStats     *ServiceStats `json:"template_cache_stats"`
	
	// Hot keys and patterns
	HotKeys                []HotKey      `json:"hot_keys,omitempty"`
	KeyPatterns            []KeyPattern  `json:"key_patterns,omitempty"`
}

// RedisInfo contains Redis-specific information
type RedisInfo struct {
	Version                string        `json:"version"`
	UptimeSeconds          int64         `json:"uptime_seconds"`
	ConnectedClients       int64         `json:"connected_clients"`
	UsedMemory             int64         `json:"used_memory"`
	UsedMemoryPeak         int64         `json:"used_memory_peak"`
	MemoryFragmentationRatio float64     `json:"memory_fragmentation_ratio"`
	TotalCommandsProcessed int64         `json:"total_commands_processed"`
	KeyspaceHits           int64         `json:"keyspace_hits"`
	KeyspaceMisses         int64         `json:"keyspace_misses"`
	EvictedKeys            int64         `json:"evicted_keys"`
	ExpiredKeys            int64         `json:"expired_keys"`
}

// ServiceStats contains service-specific cache statistics
type ServiceStats struct {
	ServiceName        string        `json:"service_name"`
	Hits               int64         `json:"hits"`
	Misses             int64         `json:"misses"`
	HitRatio           float64       `json:"hit_ratio"`
	AvgResponseTime    time.Duration `json:"avg_response_time"`
	Errors             int64         `json:"errors"`
	KeyCount           int64         `json:"key_count"`
	StorageUsed        int64         `json:"storage_used"`
}

// HotKey represents a frequently accessed cache key
type HotKey struct {
	Key         string    `json:"key"`
	AccessCount int64     `json:"access_count"`
	LastAccess  time.Time `json:"last_access"`
	TTL         int64     `json:"ttl"`
	Size        int64     `json:"size"`
}

// KeyPattern represents cache key usage patterns
type KeyPattern struct {
	Pattern     string    `json:"pattern"`
	Count       int64     `json:"count"`
	AvgSize     int64     `json:"avg_size"`
	HitRatio    float64   `json:"hit_ratio"`
	LastSeen    time.Time `json:"last_seen"`
}

// Alert represents a cache monitoring alert
type Alert struct {
	ID          string                 `json:"id"`
	Type        AlertType              `json:"type"`
	Severity    AlertSeverity          `json:"severity"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeHitRatio       AlertType = "hit_ratio"
	AlertTypeResponseTime   AlertType = "response_time"
	AlertTypeErrorRate      AlertType = "error_rate"
	AlertTypeMemoryUsage    AlertType = "memory_usage"
	AlertTypeConnection     AlertType = "connection"
	AlertTypeCapacity       AlertType = "capacity"
)

// AlertSeverity represents alert severity levels
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertManager handles cache monitoring alerts
type AlertManager struct {
	alerts     map[string]*Alert
	history    []*Alert
	cooldowns  map[AlertType]time.Time
	config     *MonitoringConfig
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewCacheMonitor creates a new cache monitor
func NewCacheMonitor(cache *CacheService, redis *RedisClient, logger *zap.Logger) *CacheMonitor {
	config := DefaultMonitoringConfig()
	
	monitor := &CacheMonitor{
		cache:    cache,
		redis:    redis,
		config:   config,
		logger:   logger,
		metrics:  &AggregatedMetrics{},
		alerts:   NewAlertManager(config, logger),
		stopChan: make(chan struct{}),
	}
	
	return monitor
}

// Start begins cache monitoring
func (cm *CacheMonitor) Start() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if cm.isRunning {
		return fmt.Errorf("cache monitor is already running")
	}
	
	cm.isRunning = true
	
	// Start monitoring routines
	go cm.metricsCollectionLoop()
	go cm.healthCheckLoop()
	
	if cm.config.EnableAlerts {
		go cm.alertCheckLoop()
	}
	
	cm.logger.Info("Cache monitor started",
		zap.Duration("metrics_interval", cm.config.MetricsInterval),
		zap.Duration("health_check_interval", cm.config.HealthCheckInterval),
		zap.Bool("alerts_enabled", cm.config.EnableAlerts))
	
	return nil
}

// Stop stops cache monitoring
func (cm *CacheMonitor) Stop() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if !cm.isRunning {
		return fmt.Errorf("cache monitor is not running")
	}
	
	close(cm.stopChan)
	cm.isRunning = false
	
	cm.logger.Info("Cache monitor stopped")
	return nil
}

// GetMetrics returns current aggregated metrics
func (cm *CacheMonitor) GetMetrics() *AggregatedMetrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	metrics := *cm.metrics
	return &metrics
}

// GetAlerts returns current active alerts
func (cm *CacheMonitor) GetAlerts() []*Alert {
	return cm.alerts.GetActiveAlerts()
}

// GetAlertHistory returns alert history
func (cm *CacheMonitor) GetAlertHistory() []*Alert {
	return cm.alerts.GetHistory()
}

// ResolveAlert marks an alert as resolved
func (cm *CacheMonitor) ResolveAlert(alertID string) error {
	return cm.alerts.ResolveAlert(alertID)
}

// GetHealthStatus returns comprehensive health status
func (cm *CacheMonitor) GetHealthStatus() *HealthStatus {
	metrics := cm.GetMetrics()
	alerts := cm.GetAlerts()
	
	status := &HealthStatus{
		Timestamp: time.Now(),
		Overall:   HealthStatusHealthy,
		Components: map[string]ComponentHealth{
			"redis": {
				Status:  HealthStatusHealthy,
				Message: "Redis connection healthy",
			},
			"cache": {
				Status:  HealthStatusHealthy,
				Message: "Cache service operating normally",
			},
		},
		Metrics: metrics,
	}
	
	// Check Redis health
	if redisHealth := cm.redis.GetHealthStatus(); !redisHealth.IsHealthy {
		status.Overall = HealthStatusUnhealthy
		status.Components["redis"] = ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: redisHealth.LastError,
		}
	}
	
	// Check cache performance
	if metrics.HitRatio < cm.config.HitRatioThreshold {
		if status.Overall == HealthStatusHealthy {
			status.Overall = HealthStatusDegraded
		}
		status.Components["cache"] = ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: fmt.Sprintf("Hit ratio below threshold: %.2f%%", metrics.HitRatio*100),
		}
	}
	
	// Check for critical alerts
	for _, alert := range alerts {
		if alert.Severity == AlertSeverityCritical && !alert.Resolved {
			status.Overall = HealthStatusUnhealthy
			break
		}
	}
	
	return status
}

// Monitoring loops

func (cm *CacheMonitor) metricsCollectionLoop() {
	ticker := time.NewTicker(cm.config.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			cm.collectMetrics()
		case <-cm.stopChan:
			return
		}
	}
}

func (cm *CacheMonitor) healthCheckLoop() {
	ticker := time.NewTicker(cm.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			cm.performHealthCheck()
		case <-cm.stopChan:
			return
		}
	}
}

func (cm *CacheMonitor) alertCheckLoop() {
	ticker := time.NewTicker(cm.config.AlertCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			cm.checkAlerts()
		case <-cm.stopChan:
			return
		}
	}
}

func (cm *CacheMonitor) collectMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	
	// Collect cache service metrics
	cacheStats := cm.cache.GetStats()
	
	// Collect Redis metrics
	redisMetrics := cm.redis.GetMetrics()
	
	// Collect Redis INFO
	redisInfo := cm.collectRedisInfo(ctx)
	
	// Calculate derived metrics
	hitRatio := float64(0)
	if total := cacheStats.TotalHits + cacheStats.TotalMisses; total > 0 {
		hitRatio = float64(cacheStats.TotalHits) / float64(total)
	}
	
	errorRate := float64(0)
	if cacheStats.TotalOperations > 0 {
		errorRate = float64(cacheStats.TotalErrors) / float64(cacheStats.TotalOperations)
	}
	
	// Update aggregated metrics
	cm.mu.Lock()
	cm.metrics = &AggregatedMetrics{
		Timestamp:        time.Now(),
		TotalOperations:  cacheStats.TotalOperations,
		TotalHits:        cacheStats.TotalHits,
		TotalMisses:      cacheStats.TotalMisses,
		HitRatio:         hitRatio,
		AvgResponseTime:  cacheStats.AvgReadTime,
		TotalErrors:      cacheStats.TotalErrors,
		ErrorRate:        errorRate,
		ConnectionErrors: redisMetrics.ConnectionErrors,
		RedisInfo:        redisInfo,
	}
	
	// Collect service-specific metrics if detailed metrics are enabled
	if cm.config.DetailedMetrics {
		cm.metrics.RecipeCacheStats = cm.collectServiceStats("recipe")
		cm.metrics.SessionCacheStats = cm.collectServiceStats("session")
		cm.metrics.AICacheStats = cm.collectServiceStats("ai")
		cm.metrics.TemplateCacheStats = cm.collectServiceStats("template")
	}
	
	// Collect hot keys if enabled
	if cm.config.TrackHotKeys {
		cm.metrics.HotKeys = cm.collectHotKeys(ctx)
	}
	
	// Collect key patterns if enabled
	if cm.config.TrackKeyPatterns {
		cm.metrics.KeyPatterns = cm.collectKeyPatterns(ctx)
	}
	
	cm.mu.Unlock()
	
	// Export metrics if enabled
	if cm.config.ExportMetrics {
		cm.exportMetrics()
	}
	
	cm.logger.Debug("Metrics collected",
		zap.Float64("hit_ratio", hitRatio),
		zap.Duration("avg_response_time", cacheStats.AvgReadTime),
		zap.Int64("operations", cacheStats.TotalOperations))
}

func (cm *CacheMonitor) collectRedisInfo(ctx context.Context) *RedisInfo {
	// Collect Redis INFO command data
	info, err := cm.redis.client.Info(ctx).Result()
	if err != nil {
		cm.logger.Error("Failed to collect Redis INFO", zap.Error(err))
		return &RedisInfo{}
	}
	
	// Parse Redis INFO (simplified parsing)
	redisInfo := &RedisInfo{}
	
	// In a real implementation, you would parse the INFO string
	// For now, return basic info from Redis metrics
	redisMetrics := cm.redis.GetMetrics()
	redisInfo.TotalCommandsProcessed = redisMetrics.TotalCommands
	redisInfo.KeyspaceHits = redisMetrics.CacheHits
	redisInfo.KeyspaceMisses = redisMetrics.CacheMisses
	
	return redisInfo
}

func (cm *CacheMonitor) collectServiceStats(serviceName string) *ServiceStats {
	// This would collect service-specific statistics
	// For now, return placeholder data
	return &ServiceStats{
		ServiceName:     serviceName,
		Hits:            0,
		Misses:          0,
		HitRatio:        0.0,
		AvgResponseTime: 0,
		Errors:          0,
		KeyCount:        0,
		StorageUsed:     0,
	}
}

func (cm *CacheMonitor) collectHotKeys(ctx context.Context) []HotKey {
	// This would collect hot key statistics from Redis
	// Implementation would use Redis monitoring or custom tracking
	return []HotKey{}
}

func (cm *CacheMonitor) collectKeyPatterns(ctx context.Context) []KeyPattern {
	// This would analyze key patterns
	// Implementation would scan keys and group by patterns
	return []KeyPattern{}
}

func (cm *CacheMonitor) performHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	
	// Test Redis connectivity
	if err := cm.redis.Ping(ctx); err != nil {
		cm.logger.Error("Redis health check failed", zap.Error(err))
		cm.alerts.CreateAlert(AlertTypeConnection, AlertSeverityCritical,
			"Redis connection failed", map[string]interface{}{
				"error": err.Error(),
			})
	}
	
	// Test cache operations
	testKey := "health_check_" + fmt.Sprintf("%d", time.Now().Unix())
	testData := []byte("health_check_data")
	
	if err := cm.cache.Set(ctx, testKey, testData, time.Minute); err != nil {
		cm.logger.Error("Cache write health check failed", zap.Error(err))
		cm.alerts.CreateAlert(AlertTypeConnection, AlertSeverityWarning,
			"Cache write operation failed", map[string]interface{}{
				"error": err.Error(),
			})
	}
	
	if _, err := cm.cache.Get(ctx, testKey); err != nil {
		cm.logger.Error("Cache read health check failed", zap.Error(err))
		cm.alerts.CreateAlert(AlertTypeConnection, AlertSeverityWarning,
			"Cache read operation failed", map[string]interface{}{
				"error": err.Error(),
			})
	}
	
	// Clean up test key
	cm.cache.Delete(ctx, testKey)
}

func (cm *CacheMonitor) checkAlerts() {
	metrics := cm.GetMetrics()
	
	// Check hit ratio
	if metrics.HitRatio < cm.config.HitRatioThreshold {
		cm.alerts.CreateAlert(AlertTypeHitRatio, AlertSeverityWarning,
			fmt.Sprintf("Hit ratio below threshold: %.2f%%", metrics.HitRatio*100),
			map[string]interface{}{
				"hit_ratio":  metrics.HitRatio,
				"threshold": cm.config.HitRatioThreshold,
			})
	}
	
	// Check response time
	if metrics.AvgResponseTime > cm.config.AvgResponseThreshold {
		cm.alerts.CreateAlert(AlertTypeResponseTime, AlertSeverityWarning,
			fmt.Sprintf("Average response time above threshold: %v", metrics.AvgResponseTime),
			map[string]interface{}{
				"avg_response_time": metrics.AvgResponseTime.String(),
				"threshold":        cm.config.AvgResponseThreshold.String(),
			})
	}
	
	// Check error rate
	if metrics.ErrorRate > cm.config.ErrorRateThreshold {
		cm.alerts.CreateAlert(AlertTypeErrorRate, AlertSeverityCritical,
			fmt.Sprintf("Error rate above threshold: %.2f%%", metrics.ErrorRate*100),
			map[string]interface{}{
				"error_rate": metrics.ErrorRate,
				"threshold":  cm.config.ErrorRateThreshold,
			})
	}
}

func (cm *CacheMonitor) exportMetrics() {
	// This would export metrics to external systems
	// Implementation would depend on monitoring infrastructure
	cm.logger.Debug("Metrics exported")
}

// HealthStatus represents overall system health
type HealthStatus struct {
	Timestamp  time.Time                    `json:"timestamp"`
	Overall    HealthStatusType             `json:"overall"`
	Components map[string]ComponentHealth   `json:"components"`
	Metrics    *AggregatedMetrics           `json:"metrics"`
}

// ComponentHealth represents individual component health
type ComponentHealth struct {
	Status  HealthStatusType `json:"status"`
	Message string           `json:"message"`
}

// HealthStatusType represents health status types
type HealthStatusType string

const (
	HealthStatusHealthy   HealthStatusType = "healthy"
	HealthStatusDegraded  HealthStatusType = "degraded"
	HealthStatusUnhealthy HealthStatusType = "unhealthy"
)

// Alert Manager implementation

// NewAlertManager creates a new alert manager
func NewAlertManager(config *MonitoringConfig, logger *zap.Logger) *AlertManager {
	return &AlertManager{
		alerts:    make(map[string]*Alert),
		history:   make([]*Alert, 0),
		cooldowns: make(map[AlertType]time.Time),
		config:    config,
		logger:    logger,
	}
}

// CreateAlert creates a new alert if not in cooldown
func (am *AlertManager) CreateAlert(alertType AlertType, severity AlertSeverity, message string, metadata map[string]interface{}) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	// Check cooldown
	if cooldown, exists := am.cooldowns[alertType]; exists && time.Since(cooldown) < am.config.AlertCooldown {
		return
	}
	
	alertID := fmt.Sprintf("%s_%d", alertType, time.Now().Unix())
	alert := &Alert{
		ID:        alertID,
		Type:      alertType,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
		Resolved:  false,
		Metadata:  metadata,
	}
	
	am.alerts[alertID] = alert
	am.history = append(am.history, alert)
	am.cooldowns[alertType] = time.Now()
	
	// Trim history if needed
	if len(am.history) > am.config.MaxAlertHistory {
		am.history = am.history[1:]
	}
	
	am.logger.Warn("Alert created",
		zap.String("id", alertID),
		zap.String("type", string(alertType)),
		zap.String("severity", string(severity)),
		zap.String("message", message))
}

// GetActiveAlerts returns all active alerts
func (am *AlertManager) GetActiveAlerts() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	alerts := make([]*Alert, 0, len(am.alerts))
	for _, alert := range am.alerts {
		if !alert.Resolved {
			alerts = append(alerts, alert)
		}
	}
	
	return alerts
}

// GetHistory returns alert history
func (am *AlertManager) GetHistory() []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	// Return a copy of the history
	history := make([]*Alert, len(am.history))
	copy(history, am.history)
	return history
}

// ResolveAlert marks an alert as resolved
func (am *AlertManager) ResolveAlert(alertID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert, exists := am.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}
	
	if alert.Resolved {
		return fmt.Errorf("alert already resolved: %s", alertID)
	}
	
	now := time.Now()
	alert.Resolved = true
	alert.ResolvedAt = &now
	
	am.logger.Info("Alert resolved", zap.String("id", alertID))
	return nil
}

// DefaultMonitoringConfig returns default monitoring configuration
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		MetricsInterval:      time.Minute,
		HealthCheckInterval:  time.Minute * 5,
		AlertCheckInterval:   time.Minute * 2,
		HitRatioThreshold:    0.8,   // 80%
		AvgResponseThreshold: time.Millisecond * 100,
		ErrorRateThreshold:   0.05,  // 5%
		MemoryUsageThreshold: 0.85,  // 85%
		EnableAlerts:         true,
		AlertCooldown:        time.Minute * 15,
		MaxAlertHistory:      100,
		MetricsRetention:     time.Hour * 24,
		DetailedMetrics:      true,
		ExportMetrics:        false,
		SlowQueryThreshold:   time.Millisecond * 500,
		TrackHotKeys:         true,
		TrackKeyPatterns:     true,
	}
}