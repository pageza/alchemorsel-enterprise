// Package postgres provides comprehensive database performance monitoring dashboard
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// PerformanceDashboard provides real-time database performance monitoring
type PerformanceDashboard struct {
	connectionManager *ConnectionManager
	queryMonitor      *QueryMonitor
	indexOptimizer    *IndexOptimizer
	queryCache        *QueryCache
	logger            *zap.Logger
}

// NewPerformanceDashboard creates a new performance dashboard
func NewPerformanceDashboard(
	cm *ConnectionManager,
	qm *QueryMonitor,
	io *IndexOptimizer,
	qc *QueryCache,
	logger *zap.Logger,
) *PerformanceDashboard {
	return &PerformanceDashboard{
		connectionManager: cm,
		queryMonitor:      qm,
		indexOptimizer:    io,
		queryCache:        qc,
		logger:            logger,
	}
}

// DashboardData represents comprehensive dashboard data
type DashboardData struct {
	Timestamp         time.Time             `json:"timestamp"`
	Overview          PerformanceOverview   `json:"overview"`
	ConnectionMetrics ConnectionMetrics     `json:"connection_metrics"`
	QueryMetrics      QueryAnalysis         `json:"query_metrics"`
	CacheMetrics      CacheMetrics          `json:"cache_metrics"`
	IndexHealth       IndexHealthScore      `json:"index_health"`
	Alerts            []PerformanceAlert    `json:"alerts"`
	Recommendations   []string              `json:"recommendations"`
	TrendData         TrendData             `json:"trend_data"`
}

// PerformanceOverview provides high-level performance summary
type PerformanceOverview struct {
	HealthScore       float64 `json:"health_score"`
	Status            string  `json:"status"`
	QPS               float64 `json:"queries_per_second"`
	AvgResponseTime   time.Duration `json:"avg_response_time"`
	CacheHitRatio     float64 `json:"cache_hit_ratio"`
	IndexUsageRatio   float64 `json:"index_usage_ratio"`
	ConnectionUtil    float64 `json:"connection_utilization"`
	ErrorRate         float64 `json:"error_rate"`
}

// PerformanceAlert represents a performance alert
type PerformanceAlert struct {
	ID          string               `json:"id"`
	Severity    AlertSeverity        `json:"severity"`
	Category    AlertCategory        `json:"category"`
	Title       string               `json:"title"`
	Description string               `json:"description"`
	Metric      string               `json:"metric"`
	Value       interface{}          `json:"value"`
	Threshold   interface{}          `json:"threshold"`
	Timestamp   time.Time            `json:"timestamp"`
	Actions     []RecommendedAction  `json:"actions"`
}

// AlertSeverity represents alert severity levels
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertCategory represents alert categories
type AlertCategory string

const (
	AlertCategoryConnection AlertCategory = "connection"
	AlertCategoryQuery      AlertCategory = "query"
	AlertCategoryCache      AlertCategory = "cache"
	AlertCategoryIndex      AlertCategory = "index"
	AlertCategorySystem     AlertCategory = "system"
)

// RecommendedAction represents a recommended action for an alert
type RecommendedAction struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Urgency     string `json:"urgency"`
}

// TrendData represents performance trends over time
type TrendData struct {
	TimePoints    []time.Time `json:"time_points"`
	QPS           []float64   `json:"qps"`
	ResponseTimes []float64   `json:"response_times"`
	CacheHitRates []float64   `json:"cache_hit_rates"`
	ConnectionUtil []float64  `json:"connection_util"`
	ErrorRates    []float64   `json:"error_rates"`
}

// GetDashboardData retrieves comprehensive dashboard data
func (pd *PerformanceDashboard) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	timestamp := time.Now()
	
	// Gather all metrics
	connMetrics := pd.connectionManager.GetMetrics().GetSnapshot()
	queryAnalysis := pd.queryMonitor.GetQueryAnalysis()
	cacheMetrics := pd.queryCache.GetMetrics()
	
	// Get index analysis (cached for performance)
	indexReport, err := pd.getIndexReport(ctx)
	if err != nil {
		pd.logger.Warn("Failed to get index report", zap.Error(err))
	}
	
	// Calculate overview
	overview := pd.calculateOverview(connMetrics, queryAnalysis, cacheMetrics, indexReport)
	
	// Generate alerts
	alerts := pd.generateAlerts(connMetrics, queryAnalysis, cacheMetrics, indexReport)
	
	// Compile recommendations
	recommendations := pd.compileRecommendations(connMetrics, queryAnalysis, cacheMetrics, indexReport)
	
	// Get trend data
	trendData := pd.getTrendData()
	
	dashboard := &DashboardData{
		Timestamp:         timestamp,
		Overview:          overview,
		ConnectionMetrics: connMetrics,
		QueryMetrics:      queryAnalysis,
		CacheMetrics:      cacheMetrics,
		Alerts:            alerts,
		Recommendations:   recommendations,
		TrendData:         trendData,
	}
	
	if indexReport != nil {
		dashboard.IndexHealth = indexReport.OverallIndexHealth
	}
	
	return dashboard, nil
}

// calculateOverview calculates high-level performance overview
func (pd *PerformanceDashboard) calculateOverview(
	connMetrics ConnectionMetrics,
	queryAnalysis QueryAnalysis,
	cacheMetrics CacheMetrics,
	indexReport *IndexAnalysisReport,
) PerformanceOverview {
	
	// Calculate health score (0-100)
	healthScore := 100.0
	
	// Connection health (25% weight)
	connUtil := connMetrics.GetConnectionEfficiency()
	if connUtil > 90 {
		healthScore -= 25
	} else if connUtil > 80 {
		healthScore -= 15
	} else if connUtil < 10 {
		healthScore -= 10
	}
	
	// Query health (30% weight)
	if queryAnalysis.FailureRate > 1 {
		healthScore -= 30
	} else if queryAnalysis.SlowQueryRatio > 10 {
		healthScore -= 25
	} else if queryAnalysis.SlowQueryRatio > 5 {
		healthScore -= 15
	}
	
	// Cache health (25% weight)
	if cacheMetrics.HitRatio < 70 {
		healthScore -= 25
	} else if cacheMetrics.HitRatio < 85 {
		healthScore -= 15
	} else if cacheMetrics.HitRatio < 90 {
		healthScore -= 10
	}
	
	// Index health (20% weight)
	if indexReport != nil {
		indexHealthPenalty := (100 - indexReport.OverallIndexHealth.Score) * 0.2
		healthScore -= indexHealthPenalty
	}
	
	if healthScore < 0 {
		healthScore = 0
	}
	
	// Determine status
	status := "healthy"
	if healthScore < 50 {
		status = "critical"
	} else if healthScore < 75 {
		status = "warning"
	}
	
	// Calculate QPS
	qps := 0.0
	if queryAnalysis.TotalQueries > 0 && time.Since(queryAnalysis.Timestamp) > 0 {
		qps = float64(queryAnalysis.TotalQueries) / time.Since(queryAnalysis.Timestamp).Seconds()
	}
	
	// Get index usage ratio
	indexUsageRatio := 0.0
	if indexReport != nil {
		indexUsageRatio = indexReport.OverallIndexHealth.IndexUsageRatio
	}
	
	return PerformanceOverview{
		HealthScore:     healthScore,
		Status:          status,
		QPS:             qps,
		AvgResponseTime: queryAnalysis.AverageQueryTime,
		CacheHitRatio:   cacheMetrics.HitRatio,
		IndexUsageRatio: indexUsageRatio,
		ConnectionUtil:  connUtil,
		ErrorRate:       queryAnalysis.FailureRate,
	}
}

// generateAlerts generates performance alerts based on metrics
func (pd *PerformanceDashboard) generateAlerts(
	connMetrics ConnectionMetrics,
	queryAnalysis QueryAnalysis,
	cacheMetrics CacheMetrics,
	indexReport *IndexAnalysisReport,
) []PerformanceAlert {
	
	var alerts []PerformanceAlert
	timestamp := time.Now()
	
	// Connection alerts
	connUtil := connMetrics.GetConnectionEfficiency()
	if connUtil > 90 {
		alerts = append(alerts, PerformanceAlert{
			ID:          "conn_util_critical",
			Severity:    AlertSeverityCritical,
			Category:    AlertCategoryConnection,
			Title:       "Critical Connection Pool Utilization",
			Description: fmt.Sprintf("Connection pool utilization is %.1f%%, indicating potential bottleneck", connUtil),
			Metric:      "connection_utilization",
			Value:       connUtil,
			Threshold:   90.0,
			Timestamp:   timestamp,
			Actions: []RecommendedAction{
				{
					Type:        "increase_pool_size",
					Description: "Increase max_open_conns in database configuration",
					Urgency:     "high",
				},
				{
					Type:        "optimize_queries",
					Description: "Review and optimize long-running queries",
					Urgency:     "high",
				},
			},
		})
	} else if connUtil > 80 {
		alerts = append(alerts, PerformanceAlert{
			ID:          "conn_util_warning",
			Severity:    AlertSeverityWarning,
			Category:    AlertCategoryConnection,
			Title:       "High Connection Pool Utilization",
			Description: fmt.Sprintf("Connection pool utilization is %.1f%%, monitor closely", connUtil),
			Metric:      "connection_utilization",
			Value:       connUtil,
			Threshold:   80.0,
			Timestamp:   timestamp,
			Actions: []RecommendedAction{
				{
					Type:        "monitor",
					Description: "Monitor connection usage patterns",
					Urgency:     "medium",
				},
			},
		})
	}
	
	// Query performance alerts
	if queryAnalysis.SlowQueryRatio > 10 {
		alerts = append(alerts, PerformanceAlert{
			ID:          "slow_query_critical",
			Severity:    AlertSeverityCritical,
			Category:    AlertCategoryQuery,
			Title:       "High Slow Query Ratio",
			Description: fmt.Sprintf("%.1f%% of queries are slow (>100ms)", queryAnalysis.SlowQueryRatio),
			Metric:      "slow_query_ratio",
			Value:       queryAnalysis.SlowQueryRatio,
			Threshold:   10.0,
			Timestamp:   timestamp,
			Actions: []RecommendedAction{
				{
					Type:        "optimize_queries",
					Description: "Identify and optimize slow queries immediately",
					Urgency:     "critical",
				},
				{
					Type:        "add_indexes",
					Description: "Review and add missing database indexes",
					Urgency:     "high",
				},
			},
		})
	}
	
	if queryAnalysis.FailureRate > 1 {
		alerts = append(alerts, PerformanceAlert{
			ID:          "query_failure_critical",
			Severity:    AlertSeverityCritical,
			Category:    AlertCategoryQuery,
			Title:       "High Query Failure Rate",
			Description: fmt.Sprintf("%.1f%% of queries are failing", queryAnalysis.FailureRate),
			Metric:      "query_failure_rate",
			Value:       queryAnalysis.FailureRate,
			Threshold:   1.0,
			Timestamp:   timestamp,
			Actions: []RecommendedAction{
				{
					Type:        "investigate_errors",
					Description: "Investigate query errors and database connectivity",
					Urgency:     "critical",
				},
			},
		})
	}
	
	// Cache alerts
	if cacheMetrics.HitRatio < 70 && cacheMetrics.Hits+cacheMetrics.Misses > 100 {
		alerts = append(alerts, PerformanceAlert{
			ID:          "cache_hit_low",
			Severity:    AlertSeverityWarning,
			Category:    AlertCategoryCache,
			Title:       "Low Cache Hit Ratio",
			Description: fmt.Sprintf("Cache hit ratio is %.1f%%, below target of 90%%", cacheMetrics.HitRatio),
			Metric:      "cache_hit_ratio",
			Value:       cacheMetrics.HitRatio,
			Threshold:   90.0,
			Timestamp:   timestamp,
			Actions: []RecommendedAction{
				{
					Type:        "tune_cache",
					Description: "Review cache TTL settings and increase cache size",
					Urgency:     "medium",
				},
			},
		})
	}
	
	// Index alerts
	if indexReport != nil {
		if indexReport.OverallIndexHealth.IndexUsageRatio < 90 {
			alerts = append(alerts, PerformanceAlert{
				ID:          "index_usage_low",
				Severity:    AlertSeverityWarning,
				Category:    AlertCategoryIndex,
				Title:       "Low Index Usage Ratio",
				Description: fmt.Sprintf("Index usage ratio is %.1f%%, indicating potential missing indexes", indexReport.OverallIndexHealth.IndexUsageRatio),
				Metric:      "index_usage_ratio",
				Value:       indexReport.OverallIndexHealth.IndexUsageRatio,
				Threshold:   90.0,
				Timestamp:   timestamp,
				Actions: []RecommendedAction{
					{
						Type:        "add_indexes",
						Description: "Review and add missing database indexes",
						Urgency:     "medium",
					},
				},
			})
		}
		
		if len(indexReport.UnusedIndexes) > 0 {
			alerts = append(alerts, PerformanceAlert{
				ID:          "unused_indexes",
				Severity:    AlertSeverityInfo,
				Category:    AlertCategoryIndex,
				Title:       "Unused Indexes Detected",
				Description: fmt.Sprintf("Found %d unused indexes consuming storage", len(indexReport.UnusedIndexes)),
				Metric:      "unused_index_count",
				Value:       len(indexReport.UnusedIndexes),
				Threshold:   0,
				Timestamp:   timestamp,
				Actions: []RecommendedAction{
					{
						Type:        "drop_indexes",
						Description: "Review and drop unused indexes to improve write performance",
						Urgency:     "low",
					},
				},
			})
		}
	}
	
	return alerts
}

// compileRecommendations compiles all performance recommendations
func (pd *PerformanceDashboard) compileRecommendations(
	connMetrics ConnectionMetrics,
	queryAnalysis QueryAnalysis,
	cacheMetrics CacheMetrics,
	indexReport *IndexAnalysisReport,
) []string {
	
	var recommendations []string
	
	// Add connection recommendations
	recommendations = append(recommendations, connMetrics.GetRecommendations()...)
	
	// Add query recommendations
	recommendations = append(recommendations, queryAnalysis.Recommendations...)
	
	// Add index recommendations
	if indexReport != nil {
		recommendations = append(recommendations, indexReport.Recommendations...)
	}
	
	// Add cache-specific recommendations
	if cacheMetrics.HitRatio < 90 && cacheMetrics.Hits+cacheMetrics.Misses > 100 {
		recommendations = append(recommendations, "Consider increasing cache TTL or implementing more aggressive caching")
	}
	
	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, rec := range recommendations {
		if !seen[rec] {
			seen[rec] = true
			unique = append(unique, rec)
		}
	}
	
	return unique
}

// getIndexReport gets cached index analysis report
func (pd *PerformanceDashboard) getIndexReport(ctx context.Context) (*IndexAnalysisReport, error) {
	// Index analysis is expensive, so we cache it for 10 minutes
	if pd.indexOptimizer != nil {
		return pd.indexOptimizer.AnalyzeIndexes(ctx)
	}
	return nil, nil
}

// getTrendData gets historical trend data (simplified for demo)
func (pd *PerformanceDashboard) getTrendData() TrendData {
	// In a real implementation, this would fetch historical data from a time-series database
	// For now, we return mock data showing the current state
	now := time.Now()
	return TrendData{
		TimePoints:     []time.Time{now.Add(-1 * time.Hour), now.Add(-30 * time.Minute), now},
		QPS:            []float64{50.0, 75.0, 100.0},
		ResponseTimes:  []float64{45.0, 55.0, 65.0},
		CacheHitRates:  []float64{88.0, 91.0, 93.0},
		ConnectionUtil: []float64{60.0, 70.0, 75.0},
		ErrorRates:     []float64{0.1, 0.2, 0.1},
	}
}

// ExportDashboardData exports dashboard data as JSON
func (pd *PerformanceDashboard) ExportDashboardData(ctx context.Context) ([]byte, error) {
	data, err := pd.GetDashboardData(ctx)
	if err != nil {
		return nil, err
	}
	
	return json.MarshalIndent(data, "", "  ")
}

// GetHealthStatus returns simplified health status
func (pd *PerformanceDashboard) GetHealthStatus(ctx context.Context) (string, float64, error) {
	data, err := pd.GetDashboardData(ctx)
	if err != nil {
		return "unknown", 0, err
	}
	
	return data.Overview.Status, data.Overview.HealthScore, nil
}

// StartMonitoring starts continuous performance monitoring
func (pd *PerformanceDashboard) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	pd.logger.Info("Starting database performance monitoring",
		zap.Duration("interval", interval))
	
	for {
		select {
		case <-ctx.Done():
			pd.logger.Info("Stopping database performance monitoring")
			return
		case <-ticker.C:
			pd.performHealthCheck(ctx)
		}
	}
}

// performHealthCheck performs periodic health check and logging
func (pd *PerformanceDashboard) performHealthCheck(ctx context.Context) {
	status, score, err := pd.GetHealthStatus(ctx)
	if err != nil {
		pd.logger.Error("Failed to get health status", zap.Error(err))
		return
	}
	
	if status == "critical" {
		pd.logger.Error("Database performance critical",
			zap.String("status", status),
			zap.Float64("health_score", score))
	} else if status == "warning" {
		pd.logger.Warn("Database performance degraded",
			zap.String("status", status),
			zap.Float64("health_score", score))
	} else {
		pd.logger.Debug("Database performance healthy",
			zap.String("status", status),
			zap.Float64("health_score", score))
	}
}