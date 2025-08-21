// Package ai provides comprehensive usage analytics for AI services
package ai

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UsageAnalytics tracks and analyzes AI service usage patterns
type UsageAnalytics struct {
	cacheRepo       outbound.CacheRepository
	logger          *zap.Logger
	
	// Real-time metrics
	currentMetrics  *RealTimeMetrics
	
	// Historical tracking
	hourlyStats     map[string]*HourlyStats
	dailyStats      map[string]*DailyStats
	featureStats    map[string]*FeatureStats
	userStats       map[uuid.UUID]*UserStats
	
	// Performance tracking
	latencyHistogram map[string][]time.Duration
	errorCounts      map[string]int64
	cacheStats       *CacheStats
	
	// Thread safety
	mu              sync.RWMutex
}

// RealTimeMetrics holds current performance metrics
type RealTimeMetrics struct {
	RequestsPerSecond float64
	AverageLatency    time.Duration
	ErrorRate         float64
	CacheHitRate      float64
	ActiveRequests    int64
	TotalRequests     int64
	TotalErrors       int64
	TotalCacheHits    int64
	TotalCacheMisses  int64
	LastUpdated       time.Time
}

// HourlyStats tracks statistics per hour
type HourlyStats struct {
	Hour            time.Time
	RequestCount    int64
	ErrorCount      int64
	CacheHits       int64
	CacheMisses     int64
	TotalLatency    time.Duration
	AverageLatency  time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	UniqueUsers     map[uuid.UUID]bool
	FeatureUsage    map[string]int64
}

// DailyStats tracks statistics per day
type DailyStats struct {
	Date            time.Time
	RequestCount    int64
	ErrorCount      int64
	CacheHits       int64
	CacheMisses     int64
	TotalLatency    time.Duration
	AverageLatency  time.Duration
	UniqueUsers     int
	TopFeatures     map[string]int64
	PeakHour        int
	PeakRequests    int64
}

// FeatureStats tracks usage of specific features
type FeatureStats struct {
	FeatureName     string
	RequestCount    int64
	ErrorCount      int64
	TotalLatency    time.Duration
	AverageLatency  time.Duration
	SuccessRate     float64
	PopularityRank  int
	QualityScore    float64
	UserCount       int
	LastUsed        time.Time
}

// UserStats tracks individual user behavior
type UserStats struct {
	UserID          uuid.UUID
	TotalRequests   int64
	TotalErrors     int64
	AverageLatency  time.Duration
	FavoriteFeatures map[string]int64
	LastActivity    time.Time
	FirstSeen       time.Time
	SessionCount    int64
	AverageSession  time.Duration
}

// CacheStats tracks caching performance
type CacheStats struct {
	TotalHits       int64
	TotalMisses     int64
	HitRate         float64
	EvictionCount   int64
	TotalSize       int64
	AverageItemSize int64
	TTLHits         int64
	TTLMisses       int64
}

// PerformanceMetrics represents system performance data
type PerformanceMetrics struct {
	Timestamp       time.Time
	RequestsPerMinute int64
	AverageLatency  time.Duration
	P50Latency      time.Duration
	P95Latency      time.Duration
	P99Latency      time.Duration
	ErrorRate       float64
	CacheHitRate    float64
	ThroughputMBps  float64
}

// TrendAnalysis provides trend analysis data
type TrendAnalysis struct {
	Period          string
	TrendDirection  string // increasing, decreasing, stable
	GrowthRate      float64
	Seasonality     map[string]float64
	Predictions     map[string]float64
	Anomalies       []AnomalyDetection
	Recommendations []string
}

// AnomalyDetection represents detected anomalies
type AnomalyDetection struct {
	Timestamp   time.Time
	Metric      string
	Value       float64
	Expected    float64
	Deviation   float64
	Severity    string // low, medium, high
	Description string
}

// NewUsageAnalytics creates a new usage analytics tracker
func NewUsageAnalytics(cacheRepo outbound.CacheRepository, logger *zap.Logger) *UsageAnalytics {
	namedLogger := logger.Named("usage-analytics")
	
	return &UsageAnalytics{
		cacheRepo:        cacheRepo,
		logger:           namedLogger,
		currentMetrics:   &RealTimeMetrics{LastUpdated: time.Now()},
		hourlyStats:      make(map[string]*HourlyStats),
		dailyStats:       make(map[string]*DailyStats),
		featureStats:     make(map[string]*FeatureStats),
		userStats:        make(map[uuid.UUID]*UserStats),
		latencyHistogram: make(map[string][]time.Duration),
		errorCounts:      make(map[string]int64),
		cacheStats:       &CacheStats{},
	}
}

// TrackRequest records a request and its metrics
func (ua *UsageAnalytics) TrackRequest(ctx context.Context, feature string, latency time.Duration, resultSize int) {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	
	now := time.Now()
	userID := ua.extractUserIDFromContext(ctx)
	
	// Update real-time metrics
	ua.updateRealTimeMetrics(latency, false)
	
	// Update hourly stats
	ua.updateHourlyStats(now, feature, userID, latency, false)
	
	// Update daily stats
	ua.updateDailyStats(now, feature, userID, latency, false)
	
	// Update feature stats
	ua.updateFeatureStats(feature, latency, false, resultSize)
	
	// Update user stats
	ua.updateUserStats(userID, feature, latency, false)
	
	// Store latency for histogram
	ua.addLatencySample(feature, latency)
	
	ua.logger.Debug("Request tracked",
		zap.String("feature", feature),
		zap.Duration("latency", latency),
		zap.Int("result_size", resultSize),
		zap.String("user_id", userID.String()),
	)
}

// TrackError records an error occurrence
func (ua *UsageAnalytics) TrackError(ctx context.Context, feature string, errorMessage string) {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	
	now := time.Now()
	userID := ua.extractUserIDFromContext(ctx)
	
	// Update real-time metrics
	ua.updateRealTimeMetrics(0, true)
	
	// Update hourly stats
	ua.updateHourlyStats(now, feature, userID, 0, true)
	
	// Update daily stats
	ua.updateDailyStats(now, feature, userID, 0, true)
	
	// Update feature stats
	ua.updateFeatureStats(feature, 0, true, 0)
	
	// Update user stats
	ua.updateUserStats(userID, feature, 0, true)
	
	// Track error counts
	ua.errorCounts[feature]++
	ua.errorCounts["total"]++
	
	ua.logger.Warn("Error tracked",
		zap.String("feature", feature),
		zap.String("error", errorMessage),
		zap.String("user_id", userID.String()),
	)
}

// TrackCacheHit records a cache hit
func (ua *UsageAnalytics) TrackCacheHit(ctx context.Context, feature string) {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	
	ua.cacheStats.TotalHits++
	ua.currentMetrics.TotalCacheHits++
	
	// Update cache hit rate
	total := ua.cacheStats.TotalHits + ua.cacheStats.TotalMisses
	if total > 0 {
		ua.cacheStats.HitRate = float64(ua.cacheStats.TotalHits) / float64(total)
		ua.currentMetrics.CacheHitRate = ua.cacheStats.HitRate
	}
	
	ua.logger.Debug("Cache hit tracked", zap.String("feature", feature))
}

// TrackCacheMiss records a cache miss
func (ua *UsageAnalytics) TrackCacheMiss(ctx context.Context, feature string) {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	
	ua.cacheStats.TotalMisses++
	ua.currentMetrics.TotalCacheMisses++
	
	// Update cache hit rate
	total := ua.cacheStats.TotalHits + ua.cacheStats.TotalMisses
	if total > 0 {
		ua.cacheStats.HitRate = float64(ua.cacheStats.TotalHits) / float64(total)
		ua.currentMetrics.CacheHitRate = ua.cacheStats.HitRate
	}
	
	ua.logger.Debug("Cache miss tracked", zap.String("feature", feature))
}

// GetRealTimeMetrics returns current performance metrics
func (ua *UsageAnalytics) GetRealTimeMetrics() *RealTimeMetrics {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	metrics := *ua.currentMetrics
	return &metrics
}

// GetPerformanceMetrics returns detailed performance metrics
func (ua *UsageAnalytics) GetPerformanceMetrics(period string) *PerformanceMetrics {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	// Calculate latency percentiles
	allLatencies := []time.Duration{}
	for _, latencies := range ua.latencyHistogram {
		allLatencies = append(allLatencies, latencies...)
	}
	
	sort.Slice(allLatencies, func(i, j int) bool {
		return allLatencies[i] < allLatencies[j]
	})
	
	var p50, p95, p99 time.Duration
	if len(allLatencies) > 0 {
		p50 = allLatencies[len(allLatencies)*50/100]
		p95 = allLatencies[len(allLatencies)*95/100]
		p99 = allLatencies[len(allLatencies)*99/100]
	}
	
	// Calculate requests per minute
	requestsPerMinute := ua.currentMetrics.TotalRequests
	if period == "hour" {
		requestsPerMinute /= 60
	} else if period == "day" {
		requestsPerMinute /= (24 * 60)
	}
	
	return &PerformanceMetrics{
		Timestamp:         time.Now(),
		RequestsPerMinute: requestsPerMinute,
		AverageLatency:    ua.currentMetrics.AverageLatency,
		P50Latency:        p50,
		P95Latency:        p95,
		P99Latency:        p99,
		ErrorRate:         ua.currentMetrics.ErrorRate,
		CacheHitRate:      ua.currentMetrics.CacheHitRate,
		ThroughputMBps:    ua.calculateThroughput(),
	}
}

// GenerateReport creates a comprehensive usage report
func (ua *UsageAnalytics) GenerateReport(ctx context.Context, period string) (*UsageReport, error) {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	report := &UsageReport{
		Period:           period,
		TotalRequests:    ua.currentMetrics.TotalRequests,
		RequestsByType:   make(map[string]int64),
		AverageLatency:   ua.currentMetrics.AverageLatency,
		CacheHitRate:     ua.currentMetrics.CacheHitRate,
		ErrorRate:        ua.currentMetrics.ErrorRate,
		TopUsers:         []UserUsage{},
		HourlyBreakdown:  []HourlyUsage{},
		GeneratedAt:      time.Now(),
	}
	
	// Feature usage breakdown
	for feature, stats := range ua.featureStats {
		report.RequestsByType[feature] = stats.RequestCount
	}
	
	// Top users
	var users []UserUsage
	for userID, stats := range ua.userStats {
		users = append(users, UserUsage{
			UserID:         userID,
			RequestCount:   stats.TotalRequests,
			AverageLatency: stats.AverageLatency,
		})
	}
	
	// Sort users by request count
	sort.Slice(users, func(i, j int) bool {
		return users[i].RequestCount > users[j].RequestCount
	})
	
	// Take top 10 users
	if len(users) > 10 {
		users = users[:10]
	}
	report.TopUsers = users
	
	// Hourly breakdown
	for hourKey, stats := range ua.hourlyStats {
		if hour, err := time.Parse("2006-01-02-15", hourKey); err == nil {
			report.HourlyBreakdown = append(report.HourlyBreakdown, HourlyUsage{
				Hour:           hour.Hour(),
				RequestCount:   stats.RequestCount,
				AverageLatency: stats.AverageLatency,
				ErrorCount:     stats.ErrorCount,
			})
		}
	}
	
	// Sort hourly breakdown by hour
	sort.Slice(report.HourlyBreakdown, func(i, j int) bool {
		return report.HourlyBreakdown[i].Hour < report.HourlyBreakdown[j].Hour
	})
	
	return report, nil
}

// GetTrendAnalysis performs trend analysis on usage patterns
func (ua *UsageAnalytics) GetTrendAnalysis(period string) *TrendAnalysis {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	analysis := &TrendAnalysis{
		Period:          period,
		TrendDirection:  "stable",
		GrowthRate:      0.0,
		Seasonality:     make(map[string]float64),
		Predictions:     make(map[string]float64),
		Anomalies:       []AnomalyDetection{},
		Recommendations: []string{},
	}
	
	// Simplified trend analysis
	// In production, this would use more sophisticated algorithms
	
	// Calculate growth rate based on daily stats
	var dailyRequests []int64
	for _, stats := range ua.dailyStats {
		dailyRequests = append(dailyRequests, stats.RequestCount)
	}
	
	if len(dailyRequests) >= 2 {
		recent := dailyRequests[len(dailyRequests)-1]
		previous := dailyRequests[len(dailyRequests)-2]
		
		if previous > 0 {
			analysis.GrowthRate = float64(recent-previous) / float64(previous) * 100
			
			if analysis.GrowthRate > 5 {
				analysis.TrendDirection = "increasing"
			} else if analysis.GrowthRate < -5 {
				analysis.TrendDirection = "decreasing"
			}
		}
	}
	
	// Generate recommendations
	if analysis.GrowthRate > 20 {
		analysis.Recommendations = append(analysis.Recommendations, 
			"Consider scaling infrastructure due to high growth rate")
	}
	
	if ua.currentMetrics.ErrorRate > 0.05 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Error rate is high, investigate and improve error handling")
	}
	
	if ua.currentMetrics.CacheHitRate < 0.7 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Cache hit rate is low, consider optimizing caching strategy")
	}
	
	// Detect anomalies
	if ua.currentMetrics.AverageLatency > 5*time.Second {
		analysis.Anomalies = append(analysis.Anomalies, AnomalyDetection{
			Timestamp:   time.Now(),
			Metric:      "average_latency",
			Value:       float64(ua.currentMetrics.AverageLatency.Milliseconds()),
			Expected:    1000.0, // 1 second
			Deviation:   float64(ua.currentMetrics.AverageLatency.Milliseconds()) - 1000.0,
			Severity:    "high",
			Description: "Average latency is significantly higher than expected",
		})
	}
	
	return analysis
}

// GetFeaturePopularity returns feature usage rankings
func (ua *UsageAnalytics) GetFeaturePopularity() map[string]*FeatureStats {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	result := make(map[string]*FeatureStats)
	for feature, stats := range ua.featureStats {
		statsCopy := *stats
		result[feature] = &statsCopy
	}
	
	return result
}

// GetUserBehaviorInsights analyzes user behavior patterns
func (ua *UsageAnalytics) GetUserBehaviorInsights() map[string]interface{} {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	insights := make(map[string]interface{})
	
	// Calculate average session duration
	totalSessionTime := time.Duration(0)
	sessionCount := int64(0)
	
	for _, user := range ua.userStats {
		totalSessionTime += user.AverageSession
		sessionCount += user.SessionCount
	}
	
	if sessionCount > 0 {
		insights["average_session_duration"] = totalSessionTime / time.Duration(sessionCount)
	}
	
	// Most active users
	activeUsers := 0
	for _, user := range ua.userStats {
		if time.Since(user.LastActivity) < 24*time.Hour {
			activeUsers++
		}
	}
	insights["active_users_24h"] = activeUsers
	
	// Feature preferences
	featurePopularity := make(map[string]int64)
	for _, user := range ua.userStats {
		for feature, count := range user.FavoriteFeatures {
			featurePopularity[feature] += count
		}
	}
	insights["feature_popularity"] = featurePopularity
	
	return insights
}

// HealthCheck returns the health status of the usage analytics
func (ua *UsageAnalytics) HealthCheck() ComponentHealth {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	
	status := ComponentHealth{
		Status:    "healthy",
		Message:   "Usage analytics operational",
		LastCheck: time.Now(),
		Metrics: map[string]interface{}{
			"total_requests":    ua.currentMetrics.TotalRequests,
			"error_rate":        ua.currentMetrics.ErrorRate,
			"cache_hit_rate":    ua.currentMetrics.CacheHitRate,
			"average_latency":   ua.currentMetrics.AverageLatency.String(),
			"tracked_users":     len(ua.userStats),
			"tracked_features":  len(ua.featureStats),
		},
	}
	
	// Check for concerning metrics
	if ua.currentMetrics.ErrorRate > 0.1 {
		status.Status = "warning"
		status.Message = "High error rate detected"
	}
	
	if ua.currentMetrics.AverageLatency > 10*time.Second {
		status.Status = "warning"
		status.Message = "High average latency detected"
	}
	
	return status
}

// Helper methods

func (ua *UsageAnalytics) extractUserIDFromContext(ctx context.Context) uuid.UUID {
	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(uuid.UUID); ok {
			return id
		}
		if idStr, ok := userID.(string); ok {
			if id, err := uuid.Parse(idStr); err == nil {
				return id
			}
		}
	}
	
	// Return a default UUID for anonymous users
	return uuid.New()
}

func (ua *UsageAnalytics) updateRealTimeMetrics(latency time.Duration, isError bool) {
	ua.currentMetrics.TotalRequests++
	
	if isError {
		ua.currentMetrics.TotalErrors++
	}
	
	// Update error rate
	if ua.currentMetrics.TotalRequests > 0 {
		ua.currentMetrics.ErrorRate = float64(ua.currentMetrics.TotalErrors) / float64(ua.currentMetrics.TotalRequests)
	}
	
	// Update average latency (simple moving average)
	if latency > 0 {
		if ua.currentMetrics.AverageLatency == 0 {
			ua.currentMetrics.AverageLatency = latency
		} else {
			// Exponential moving average with alpha = 0.1
			ua.currentMetrics.AverageLatency = time.Duration(
				0.9*float64(ua.currentMetrics.AverageLatency) + 0.1*float64(latency),
			)
		}
	}
	
	ua.currentMetrics.LastUpdated = time.Now()
}

func (ua *UsageAnalytics) updateHourlyStats(now time.Time, feature string, userID uuid.UUID, latency time.Duration, isError bool) {
	hourKey := now.Format("2006-01-02-15")
	
	if ua.hourlyStats[hourKey] == nil {
		ua.hourlyStats[hourKey] = &HourlyStats{
			Hour:         now.Truncate(time.Hour),
			UniqueUsers:  make(map[uuid.UUID]bool),
			FeatureUsage: make(map[string]int64),
		}
	}
	
	stats := ua.hourlyStats[hourKey]
	stats.RequestCount++
	
	if isError {
		stats.ErrorCount++
	}
	
	stats.UniqueUsers[userID] = true
	stats.FeatureUsage[feature]++
	
	if latency > 0 {
		stats.TotalLatency += latency
		stats.AverageLatency = stats.TotalLatency / time.Duration(stats.RequestCount)
		
		if stats.MinLatency == 0 || latency < stats.MinLatency {
			stats.MinLatency = latency
		}
		if latency > stats.MaxLatency {
			stats.MaxLatency = latency
		}
	}
}

func (ua *UsageAnalytics) updateDailyStats(now time.Time, feature string, userID uuid.UUID, latency time.Duration, isError bool) {
	dayKey := now.Format("2006-01-02")
	
	if ua.dailyStats[dayKey] == nil {
		ua.dailyStats[dayKey] = &DailyStats{
			Date:         now.Truncate(24 * time.Hour),
			TopFeatures:  make(map[string]int64),
		}
	}
	
	stats := ua.dailyStats[dayKey]
	stats.RequestCount++
	
	if isError {
		stats.ErrorCount++
	}
	
	stats.TopFeatures[feature]++
	
	if latency > 0 {
		stats.TotalLatency += latency
		stats.AverageLatency = stats.TotalLatency / time.Duration(stats.RequestCount)
	}
}

func (ua *UsageAnalytics) updateFeatureStats(feature string, latency time.Duration, isError bool, resultSize int) {
	if ua.featureStats[feature] == nil {
		ua.featureStats[feature] = &FeatureStats{
			FeatureName: feature,
			LastUsed:    time.Now(),
		}
	}
	
	stats := ua.featureStats[feature]
	stats.RequestCount++
	stats.LastUsed = time.Now()
	
	if isError {
		stats.ErrorCount++
	}
	
	// Update success rate
	if stats.RequestCount > 0 {
		stats.SuccessRate = 1.0 - (float64(stats.ErrorCount) / float64(stats.RequestCount))
	}
	
	// Update average latency
	if latency > 0 {
		stats.TotalLatency += latency
		stats.AverageLatency = stats.TotalLatency / time.Duration(stats.RequestCount)
	}
}

func (ua *UsageAnalytics) updateUserStats(userID uuid.UUID, feature string, latency time.Duration, isError bool) {
	if ua.userStats[userID] == nil {
		ua.userStats[userID] = &UserStats{
			UserID:           userID,
			FavoriteFeatures: make(map[string]int64),
			FirstSeen:        time.Now(),
			LastActivity:     time.Now(),
		}
	}
	
	stats := ua.userStats[userID]
	stats.TotalRequests++
	stats.LastActivity = time.Now()
	stats.FavoriteFeatures[feature]++
	
	if isError {
		stats.TotalErrors++
	}
	
	// Update average latency
	if latency > 0 {
		if stats.AverageLatency == 0 {
			stats.AverageLatency = latency
		} else {
			// Exponential moving average
			stats.AverageLatency = time.Duration(
				0.9*float64(stats.AverageLatency) + 0.1*float64(latency),
			)
		}
	}
}

func (ua *UsageAnalytics) addLatencySample(feature string, latency time.Duration) {
	if ua.latencyHistogram[feature] == nil {
		ua.latencyHistogram[feature] = []time.Duration{}
	}
	
	ua.latencyHistogram[feature] = append(ua.latencyHistogram[feature], latency)
	
	// Keep only the last 1000 samples per feature to avoid memory issues
	if len(ua.latencyHistogram[feature]) > 1000 {
		ua.latencyHistogram[feature] = ua.latencyHistogram[feature][len(ua.latencyHistogram[feature])-1000:]
	}
}

func (ua *UsageAnalytics) calculateThroughput() float64 {
	// Simple throughput calculation based on recent activity
	// In production, this would be more sophisticated
	return float64(ua.currentMetrics.TotalRequests) / 60.0 // requests per minute
}