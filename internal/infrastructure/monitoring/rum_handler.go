package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RUMHandler handles Real User Monitoring data collection
type RUMHandler struct {
	logger       *zap.Logger
	metrics      *MetricsCollector
	tracing      *TracingProvider
	storage      RUMStorage
}

// RUMData represents the structure of RUM data
type RUMData struct {
	Type     string                 `json:"type"`
	Data     []map[string]interface{} `json:"data"`
	Metadata map[string]interface{} `json:"metadata"`
}

// RUMMetric represents a single RUM metric
type RUMMetric struct {
	Name        string                 `json:"name"`
	Timestamp   int64                  `json:"timestamp"`
	SessionID   string                 `json:"session_id"`
	PageViewID  string                 `json:"page_view_id"`
	UserID      string                 `json:"user_id"`
	URL         string                 `json:"url"`
	Environment string                 `json:"environment"`
	Version     string                 `json:"version"`
	Data        map[string]interface{} `json:"data"`
}

// RUMError represents a frontend error
type RUMError struct {
	Type         string `json:"type"`
	Message      string `json:"message"`
	Filename     string `json:"filename"`
	LineNumber   int    `json:"line_number"`
	ColumnNumber int    `json:"column_number"`
	Stack        string `json:"stack"`
	Timestamp    int64  `json:"timestamp"`
	SessionID    string `json:"session_id"`
	PageViewID   string `json:"page_view_id"`
	UserID       string `json:"user_id"`
	URL          string `json:"url"`
	UserAgent    string `json:"user_agent"`
}

// RUMInteraction represents a user interaction
type RUMInteraction struct {
	Type        string                 `json:"type"`
	Element     string                 `json:"element"`
	ID          string                 `json:"id"`
	Class       string                 `json:"class"`
	Text        string                 `json:"text"`
	Href        string                 `json:"href"`
	Coordinates map[string]interface{} `json:"coordinates"`
	Timestamp   int64                  `json:"timestamp"`
	SessionID   string                 `json:"session_id"`
	PageViewID  string                 `json:"page_view_id"`
	UserID      string                 `json:"user_id"`
	URL         string                 `json:"url"`
}

// RUMStorage interface for storing RUM data
type RUMStorage interface {
	StoreMetrics(metrics []RUMMetric) error
	StoreErrors(errors []RUMError) error
	StoreInteractions(interactions []RUMInteraction) error
	GetSessionMetrics(sessionID string) ([]RUMMetric, error)
	GetUserSessions(userID string, limit int) ([]string, error)
	GetErrorStats(timeRange time.Duration) (map[string]int, error)
}

// NewRUMHandler creates a new RUM handler
func NewRUMHandler(logger *zap.Logger, metrics *MetricsCollector, tracing *TracingProvider, storage RUMStorage) *RUMHandler {
	return &RUMHandler{
		logger:  logger,
		metrics: metrics,
		tracing: tracing,
		storage: storage,
	}
}

// HandleMetrics handles RUM metrics collection
func (h *RUMHandler) HandleMetrics(c *gin.Context) {
	ctx, span := h.tracing.StartSpan(c.Request.Context(), "rum.collect_metrics")
	defer span.End()

	var rumData RUMData
	if err := c.ShouldBindJSON(&rumData); err != nil {
		h.logger.Error("Failed to bind RUM data", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Convert to RUM metrics
	var metrics []RUMMetric
	for _, item := range rumData.Data {
		metric := RUMMetric{}
		if data, err := json.Marshal(item); err == nil {
			if err := json.Unmarshal(data, &metric); err == nil {
				metrics = append(metrics, metric)
			}
		}
	}

	if len(metrics) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid metrics found"})
		return
	}

	// Store metrics
	if err := h.storage.StoreMetrics(metrics); err != nil {
		h.logger.Error("Failed to store RUM metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store metrics"})
		return
	}

	// Process metrics for Prometheus
	h.processMetricsForPrometheus(ctx, metrics)

	h.logger.Debug("Stored RUM metrics",
		zap.Int("count", len(metrics)),
		zap.String("user_id", metrics[0].UserID),
		zap.String("session_id", metrics[0].SessionID),
	)

	c.JSON(http.StatusOK, gin.H{"status": "success", "processed": len(metrics)})
}

// HandleErrors handles RUM error collection
func (h *RUMHandler) HandleErrors(c *gin.Context) {
	ctx, span := h.tracing.StartSpan(c.Request.Context(), "rum.collect_errors")
	defer span.End()

	var rumData RUMData
	if err := c.ShouldBindJSON(&rumData); err != nil {
		h.logger.Error("Failed to bind RUM error data", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Convert to RUM errors
	var errors []RUMError
	for _, item := range rumData.Data {
		rumError := RUMError{}
		if data, err := json.Marshal(item); err == nil {
			if err := json.Unmarshal(data, &rumError); err == nil {
				errors = append(errors, rumError)
			}
		}
	}

	if len(errors) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid errors found"})
		return
	}

	// Store errors
	if err := h.storage.StoreErrors(errors); err != nil {
		h.logger.Error("Failed to store RUM errors", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store errors"})
		return
	}

	// Process errors for monitoring
	h.processErrorsForMonitoring(ctx, errors)

	h.logger.Warn("Received frontend errors",
		zap.Int("count", len(errors)),
		zap.String("user_id", errors[0].UserID),
		zap.String("session_id", errors[0].SessionID),
	)

	c.JSON(http.StatusOK, gin.H{"status": "success", "processed": len(errors)})
}

// HandleInteractions handles RUM interaction collection
func (h *RUMHandler) HandleInteractions(c *gin.Context) {
	ctx, span := h.tracing.StartSpan(c.Request.Context(), "rum.collect_interactions")
	defer span.End()

	var rumData RUMData
	if err := c.ShouldBindJSON(&rumData); err != nil {
		h.logger.Error("Failed to bind RUM interaction data", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Convert to RUM interactions
	var interactions []RUMInteraction
	for _, item := range rumData.Data {
		interaction := RUMInteraction{}
		if data, err := json.Marshal(item); err == nil {
			if err := json.Unmarshal(data, &interaction); err == nil {
				interactions = append(interactions, interaction)
			}
		}
	}

	if len(interactions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid interactions found"})
		return
	}

	// Store interactions
	if err := h.storage.StoreInteractions(interactions); err != nil {
		h.logger.Error("Failed to store RUM interactions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store interactions"})
		return
	}

	// Process interactions for business metrics
	h.processInteractionsForMetrics(ctx, interactions)

	h.logger.Debug("Stored RUM interactions",
		zap.Int("count", len(interactions)),
		zap.String("user_id", interactions[0].UserID),
		zap.String("session_id", interactions[0].SessionID),
	)

	c.JSON(http.StatusOK, gin.H{"status": "success", "processed": len(interactions)})
}

// processMetricsForPrometheus processes RUM metrics for Prometheus
func (h *RUMHandler) processMetricsForPrometheus(ctx context.Context, metrics []RUMMetric) {
	for _, metric := range metrics {
		switch metric.Name {
		case "web_vitals.lcp":
			if value, ok := metric.Data["value"].(float64); ok {
				h.recordWebVitalMetric(ctx, "lcp", value, metric.URL)
			}
		case "web_vitals.fid":
			if value, ok := metric.Data["value"].(float64); ok {
				h.recordWebVitalMetric(ctx, "fid", value, metric.URL)
			}
		case "web_vitals.cls":
			if value, ok := metric.Data["value"].(float64); ok {
				h.recordWebVitalMetric(ctx, "cls", value, metric.URL)
			}
		case "web_vitals.ttfb":
			if value, ok := metric.Data["value"].(float64); ok {
				h.recordWebVitalMetric(ctx, "ttfb", value, metric.URL)
			}
		case "web_vitals.fcp":
			if value, ok := metric.Data["value"].(float64); ok {
				h.recordWebVitalMetric(ctx, "fcp", value, metric.URL)
			}
		case "page.load":
			h.processPageLoadMetric(ctx, metric)
		case "resource.timing":
			h.processResourceTimingMetric(ctx, metric)
		case "business.event":
			h.processBusinessEventMetric(ctx, metric)
		case "feature.usage":
			h.processFeatureUsageMetric(ctx, metric)
		}
	}
}

// recordWebVitalMetric records Web Vitals metrics
func (h *RUMHandler) recordWebVitalMetric(ctx context.Context, vital string, value float64, url string) {
	// Record in Prometheus metrics
	if h.metrics != nil {
		// Add Web Vitals metrics to MetricsCollector if not present
		h.logger.Debug("Recording Web Vital",
			zap.String("vital", vital),
			zap.Float64("value", value),
			zap.String("url", url),
		)
	}
}

// processPageLoadMetric processes page load metrics
func (h *RUMHandler) processPageLoadMetric(ctx context.Context, metric RUMMetric) {
	if timing, ok := metric.Data["timing"].(map[string]interface{}); ok {
		if loadComplete, ok := timing["load_complete"].(float64); ok {
			h.logger.Debug("Page load completed",
				zap.Float64("duration_ms", loadComplete),
				zap.String("url", metric.URL),
				zap.String("user_id", metric.UserID),
			)
		}
	}
}

// processResourceTimingMetric processes resource timing metrics
func (h *RUMHandler) processResourceTimingMetric(ctx context.Context, metric RUMMetric) {
	if resourceType, ok := metric.Data["type"].(string); ok {
		if duration, ok := metric.Data["duration"].(float64); ok {
			h.logger.Debug("Resource loaded",
				zap.String("type", resourceType),
				zap.Float64("duration_ms", duration),
				zap.String("name", fmt.Sprintf("%v", metric.Data["name"])),
			)
		}
	}
}

// processBusinessEventMetric processes business event metrics
func (h *RUMHandler) processBusinessEventMetric(ctx context.Context, metric RUMMetric) {
	if event, ok := metric.Data["event"].(string); ok {
		h.logger.Info("Business event tracked",
			zap.String("event", event),
			zap.String("user_id", metric.UserID),
			zap.String("url", metric.URL),
		)

		// Record business metrics
		switch event {
		case "recipe_created":
			if h.metrics != nil {
				h.metrics.RecipeCreated()
			}
		case "recipe_viewed":
			if h.metrics != nil {
				h.metrics.RecipeViewed()
			}
		case "user_registered":
			if h.metrics != nil {
				h.metrics.UserRegistered()
			}
		}
	}
}

// processFeatureUsageMetric processes feature usage metrics
func (h *RUMHandler) processFeatureUsageMetric(ctx context.Context, metric RUMMetric) {
	if feature, ok := metric.Data["feature"].(string); ok {
		if action, ok := metric.Data["action"].(string); ok {
			h.logger.Debug("Feature usage tracked",
				zap.String("feature", feature),
				zap.String("action", action),
				zap.String("user_id", metric.UserID),
			)
		}
	}
}

// processErrorsForMonitoring processes frontend errors for monitoring
func (h *RUMHandler) processErrorsForMonitoring(ctx context.Context, errors []RUMError) {
	for _, err := range errors {
		h.logger.Error("Frontend error",
			zap.String("type", err.Type),
			zap.String("message", err.Message),
			zap.String("filename", err.Filename),
			zap.Int("line", err.LineNumber),
			zap.String("user_id", err.UserID),
			zap.String("url", err.URL),
		)

		// Record error metrics
		if h.metrics != nil {
			h.metrics.RecordError("frontend", err.Type)
		}

		// Record error in tracing
		if h.tracing != nil {
			h.tracing.AddSpanEvent(ctx, "frontend_error",
				map[string]interface{}{
					"error.type":    err.Type,
					"error.message": err.Message,
					"error.file":    err.Filename,
					"user.id":       err.UserID,
				},
			)
		}
	}
}

// processInteractionsForMetrics processes user interactions for business metrics
func (h *RUMHandler) processInteractionsForMetrics(ctx context.Context, interactions []RUMInteraction) {
	for _, interaction := range interactions {
		h.logger.Debug("User interaction",
			zap.String("type", interaction.Type),
			zap.String("element", interaction.Element),
			zap.String("text", interaction.Text),
			zap.String("user_id", interaction.UserID),
		)

		// Track specific interactions for business metrics
		if interaction.Element == "button" || interaction.Element == "a" {
			// Track button/link clicks for engagement metrics
		}
	}
}

// GetSessionAnalytics returns analytics for a specific session
func (h *RUMHandler) GetSessionAnalytics(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session ID required"})
		return
	}

	metrics, err := h.storage.GetSessionMetrics(sessionID)
	if err != nil {
		h.logger.Error("Failed to get session metrics", zap.Error(err), zap.String("session_id", sessionID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve session data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"metrics":    metrics,
	})
}

// GetUserSessions returns sessions for a specific user
func (h *RUMHandler) GetUserSessions(c *gin.Context) {
	userID := c.Param("userId")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}

	sessions, err := h.storage.GetUserSessions(userID, limit)
	if err != nil {
		h.logger.Error("Failed to get user sessions", zap.Error(err), zap.String("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve user sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":  userID,
		"sessions": sessions,
	})
}

// GetErrorStats returns error statistics
func (h *RUMHandler) GetErrorStats(c *gin.Context) {
	timeRangeStr := c.DefaultQuery("timeRange", "24h")
	timeRange, err := time.ParseDuration(timeRangeStr)
	if err != nil {
		timeRange = 24 * time.Hour
	}

	stats, err := h.storage.GetErrorStats(timeRange)
	if err != nil {
		h.logger.Error("Failed to get error stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve error statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"time_range": timeRangeStr,
		"stats":      stats,
	})
}