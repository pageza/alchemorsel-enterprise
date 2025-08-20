package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// BusinessMetricsCollector collects and tracks business-specific metrics
type BusinessMetricsCollector struct {
	logger  *zap.Logger
	tracing *TracingProvider
	
	// User metrics
	usersActive           prometheus.Gauge
	userRegistrations     *prometheus.CounterVec
	userLogins            *prometheus.CounterVec
	userSessions          prometheus.Histogram
	userRetention         *prometheus.GaugeVec
	
	// Recipe metrics
	recipesCreated        *prometheus.CounterVec
	recipesViewed         *prometheus.CounterVec
	recipesShared         *prometheus.CounterVec
	recipesRated          *prometheus.CounterVec
	recipeCreationTime    prometheus.Histogram
	
	// Search and discovery metrics
	searchQueries         *prometheus.CounterVec
	searchResults         prometheus.Histogram
	searchConversions     *prometheus.CounterVec
	
	// AI service metrics
	aiRequests            *prometheus.CounterVec
	aiResponseTime        *prometheus.HistogramVec
	aiCosts               *prometheus.CounterVec
	aiQualityScores       *prometheus.HistogramVec
	aiModelUsage          *prometheus.CounterVec
	
	// Engagement metrics
	pageViews             *prometheus.CounterVec
	sessionDuration       prometheus.Histogram
	bounceRate            *prometheus.GaugeVec
	userActions           *prometheus.CounterVec
	featureUsage          *prometheus.CounterVec
	
	// Conversion metrics
	conversionFunnels     *prometheus.CounterVec
	revenueMetrics        *prometheus.CounterVec
	subscriptionMetrics   *prometheus.GaugeVec
	
	// Performance impact on business
	performanceImpact     *prometheus.GaugeVec
	errorImpact           *prometheus.CounterVec
	
	// Content metrics
	contentCreation       *prometheus.CounterVec
	contentEngagement     *prometheus.CounterVec
	contentQuality        *prometheus.HistogramVec
}

// BusinessEvent represents a business event
type BusinessEvent struct {
	Type        string                 `json:"type"`
	UserID      string                 `json:"user_id"`
	SessionID   string                 `json:"session_id"`
	Timestamp   int64                  `json:"timestamp"`
	Properties  map[string]interface{} `json:"properties"`
	Value       float64                `json:"value,omitempty"`
	Currency    string                 `json:"currency,omitempty"`
	Environment string                 `json:"environment"`
}

// UserMetrics represents user-specific metrics
type UserMetrics struct {
	UserID              string    `json:"user_id"`
	RegistrationDate    time.Time `json:"registration_date"`
	LastLoginDate       time.Time `json:"last_login_date"`
	SessionCount        int       `json:"session_count"`
	TotalSessionTime    int64     `json:"total_session_time"`
	RecipesCreated      int       `json:"recipes_created"`
	RecipesViewed       int       `json:"recipes_viewed"`
	SearchQueries       int       `json:"search_queries"`
	AIRequestsCount     int       `json:"ai_requests_count"`
	LifetimeValue       float64   `json:"lifetime_value"`
	SubscriptionStatus  string    `json:"subscription_status"`
}

// NewBusinessMetricsCollector creates a new business metrics collector
func NewBusinessMetricsCollector(logger *zap.Logger, tracing *TracingProvider) *BusinessMetricsCollector {
	return &BusinessMetricsCollector{
		logger:  logger,
		tracing: tracing,
		
		// User metrics
		usersActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "business_users_active_total",
			Help: "Number of currently active users",
		}),
		userRegistrations: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_user_registrations_total",
			Help: "Total number of user registrations",
		}, []string{"method", "source", "environment"}),
		userLogins: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_user_logins_total",
			Help: "Total number of user logins",
		}, []string{"method", "success", "environment"}),
		userSessions: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "business_user_session_duration_seconds",
			Help:    "User session duration in seconds",
			Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400}, // 1m to 4h
		}),
		userRetention: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "business_user_retention_rate",
			Help: "User retention rate by time period",
		}, []string{"period", "cohort"}),
		
		// Recipe metrics
		recipesCreated: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_recipes_created_total",
			Help: "Total number of recipes created",
		}, []string{"user_type", "category", "source", "environment"}),
		recipesViewed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_recipes_viewed_total",
			Help: "Total number of recipe views",
		}, []string{"recipe_category", "view_type", "environment"}),
		recipesShared: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_recipes_shared_total",
			Help: "Total number of recipe shares",
		}, []string{"platform", "recipe_category", "environment"}),
		recipesRated: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_recipes_rated_total",
			Help: "Total number of recipe ratings",
		}, []string{"rating", "recipe_category", "environment"}),
		recipeCreationTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "business_recipe_creation_duration_seconds",
			Help:    "Time taken to create a recipe",
			Buckets: []float64{30, 60, 120, 300, 600, 1200}, // 30s to 20m
		}),
		
		// Search and discovery metrics
		searchQueries: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_search_queries_total",
			Help: "Total number of search queries",
		}, []string{"query_type", "results_found", "environment"}),
		searchResults: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "business_search_results_count",
			Help:    "Number of search results returned",
			Buckets: []float64{0, 1, 5, 10, 25, 50, 100, 500},
		}),
		searchConversions: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_search_conversions_total",
			Help: "Total number of search conversions",
		}, []string{"conversion_type", "query_type", "environment"}),
		
		// AI service metrics
		aiRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_ai_requests_total",
			Help: "Total number of AI requests",
		}, []string{"model", "request_type", "status", "environment"}),
		aiResponseTime: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "business_ai_response_duration_seconds",
			Help:    "AI service response time",
			Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
		}, []string{"model", "request_type"}),
		aiCosts: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_ai_costs_total_usd",
			Help: "Total AI costs in USD",
		}, []string{"model", "request_type", "environment"}),
		aiQualityScores: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "business_ai_quality_score",
			Help:    "AI response quality scores",
			Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		}, []string{"model", "request_type"}),
		aiModelUsage: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_ai_model_usage_total",
			Help: "Total usage count by AI model",
		}, []string{"model", "provider", "environment"}),
		
		// Engagement metrics
		pageViews: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_page_views_total",
			Help: "Total number of page views",
		}, []string{"page_type", "user_type", "environment"}),
		sessionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "business_session_duration_seconds",
			Help:    "Session duration in seconds",
			Buckets: []float64{30, 60, 300, 600, 1200, 1800, 3600},
		}),
		bounceRate: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "business_bounce_rate_percentage",
			Help: "Bounce rate percentage",
		}, []string{"page_type", "traffic_source"}),
		userActions: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_user_actions_total",
			Help: "Total number of user actions",
		}, []string{"action_type", "page_type", "user_type", "environment"}),
		featureUsage: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_feature_usage_total",
			Help: "Total feature usage count",
		}, []string{"feature", "user_type", "environment"}),
		
		// Conversion metrics
		conversionFunnels: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_conversion_funnel_total",
			Help: "Conversion funnel step completions",
		}, []string{"funnel", "step", "user_type", "environment"}),
		revenueMetrics: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_revenue_total_usd",
			Help: "Total revenue in USD",
		}, []string{"revenue_type", "subscription_tier", "environment"}),
		subscriptionMetrics: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "business_subscriptions_active",
			Help: "Number of active subscriptions",
		}, []string{"tier", "status", "environment"}),
		
		// Performance impact on business
		performanceImpact: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "business_performance_impact_score",
			Help: "Performance impact on business metrics",
		}, []string{"metric_type", "impact_category"}),
		errorImpact: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_error_impact_total",
			Help: "Business impact of errors",
		}, []string{"error_type", "business_function", "severity"}),
		
		// Content metrics
		contentCreation: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_content_creation_total",
			Help: "Total content creation events",
		}, []string{"content_type", "creator_type", "environment"}),
		contentEngagement: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "business_content_engagement_total",
			Help: "Total content engagement events",
		}, []string{"content_type", "engagement_type", "environment"}),
		contentQuality: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "business_content_quality_score",
			Help:    "Content quality scores",
			Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		}, []string{"content_type", "quality_metric"}),
	}
}

// TrackBusinessEvent tracks a generic business event
func (bm *BusinessMetricsCollector) TrackBusinessEvent(ctx context.Context, event BusinessEvent) {
	ctx, span := bm.tracing.StartSpan(ctx, "business.track_event")
	defer span.End()

	bm.logger.Debug("Tracking business event",
		zap.String("type", event.Type),
		zap.String("user_id", event.UserID),
		zap.Float64("value", event.Value),
	)

	switch event.Type {
	case "user_registered":
		bm.trackUserRegistration(ctx, event)
	case "user_login":
		bm.trackUserLogin(ctx, event)
	case "recipe_created":
		bm.trackRecipeCreated(ctx, event)
	case "recipe_viewed":
		bm.trackRecipeViewed(ctx, event)
	case "search_query":
		bm.trackSearchQuery(ctx, event)
	case "ai_request":
		bm.trackAIRequest(ctx, event)
	case "feature_used":
		bm.trackFeatureUsage(ctx, event)
	case "conversion_step":
		bm.trackConversionStep(ctx, event)
	case "revenue":
		bm.trackRevenue(ctx, event)
	default:
		bm.logger.Warn("Unknown business event type", zap.String("type", event.Type))
	}

	// Add trace information
	if bm.tracing != nil {
		bm.tracing.AddSpanEvent(ctx, "business_event_tracked",
			"event.type", event.Type,
			"user.id", event.UserID,
			"event.value", fmt.Sprintf("%.2f", event.Value),
		)
	}
}

// trackUserRegistration tracks user registration events
func (bm *BusinessMetricsCollector) trackUserRegistration(ctx context.Context, event BusinessEvent) {
	method := bm.getStringProperty(event.Properties, "method", "email")
	source := bm.getStringProperty(event.Properties, "source", "direct")
	
	bm.userRegistrations.WithLabelValues(method, source, event.Environment).Inc()
	
	bm.logger.Info("User registered",
		zap.String("user_id", event.UserID),
		zap.String("method", method),
		zap.String("source", source),
	)
}

// trackUserLogin tracks user login events
func (bm *BusinessMetricsCollector) trackUserLogin(ctx context.Context, event BusinessEvent) {
	method := bm.getStringProperty(event.Properties, "method", "email")
	success := bm.getStringProperty(event.Properties, "success", "true")
	
	bm.userLogins.WithLabelValues(method, success, event.Environment).Inc()
}

// trackRecipeCreated tracks recipe creation events
func (bm *BusinessMetricsCollector) trackRecipeCreated(ctx context.Context, event BusinessEvent) {
	userType := bm.getStringProperty(event.Properties, "user_type", "registered")
	category := bm.getStringProperty(event.Properties, "category", "general")
	source := bm.getStringProperty(event.Properties, "source", "manual")
	
	bm.recipesCreated.WithLabelValues(userType, category, source, event.Environment).Inc()
	
	// Track creation time if provided
	if creationTime, ok := event.Properties["creation_time_seconds"].(float64); ok {
		bm.recipeCreationTime.Observe(creationTime)
	}
}

// trackRecipeViewed tracks recipe view events
func (bm *BusinessMetricsCollector) trackRecipeViewed(ctx context.Context, event BusinessEvent) {
	category := bm.getStringProperty(event.Properties, "category", "general")
	viewType := bm.getStringProperty(event.Properties, "view_type", "detail")
	
	bm.recipesViewed.WithLabelValues(category, viewType, event.Environment).Inc()
}

// trackSearchQuery tracks search query events
func (bm *BusinessMetricsCollector) trackSearchQuery(ctx context.Context, event BusinessEvent) {
	queryType := bm.getStringProperty(event.Properties, "query_type", "text")
	resultsFound := bm.getStringProperty(event.Properties, "results_found", "true")
	
	bm.searchQueries.WithLabelValues(queryType, resultsFound, event.Environment).Inc()
	
	// Track number of results if provided
	if resultCount, ok := event.Properties["result_count"].(float64); ok {
		bm.searchResults.Observe(resultCount)
	}
}

// trackAIRequest tracks AI service request events
func (bm *BusinessMetricsCollector) trackAIRequest(ctx context.Context, event BusinessEvent) {
	model := bm.getStringProperty(event.Properties, "model", "unknown")
	requestType := bm.getStringProperty(event.Properties, "request_type", "completion")
	status := bm.getStringProperty(event.Properties, "status", "success")
	
	bm.aiRequests.WithLabelValues(model, requestType, status, event.Environment).Inc()
	
	// Track response time if provided
	if responseTime, ok := event.Properties["response_time_seconds"].(float64); ok {
		bm.aiResponseTime.WithLabelValues(model, requestType).Observe(responseTime)
	}
	
	// Track cost if provided
	if cost, ok := event.Properties["cost_usd"].(float64); ok {
		bm.aiCosts.WithLabelValues(model, requestType, event.Environment).Add(cost)
	}
	
	// Track quality score if provided
	if quality, ok := event.Properties["quality_score"].(float64); ok {
		bm.aiQualityScores.WithLabelValues(model, requestType).Observe(quality)
	}
}

// trackFeatureUsage tracks feature usage events
func (bm *BusinessMetricsCollector) trackFeatureUsage(ctx context.Context, event BusinessEvent) {
	feature := bm.getStringProperty(event.Properties, "feature", "unknown")
	userType := bm.getStringProperty(event.Properties, "user_type", "registered")
	
	bm.featureUsage.WithLabelValues(feature, userType, event.Environment).Inc()
}

// trackConversionStep tracks conversion funnel steps
func (bm *BusinessMetricsCollector) trackConversionStep(ctx context.Context, event BusinessEvent) {
	funnel := bm.getStringProperty(event.Properties, "funnel", "default")
	step := bm.getStringProperty(event.Properties, "step", "unknown")
	userType := bm.getStringProperty(event.Properties, "user_type", "registered")
	
	bm.conversionFunnels.WithLabelValues(funnel, step, userType, event.Environment).Inc()
}

// trackRevenue tracks revenue events
func (bm *BusinessMetricsCollector) trackRevenue(ctx context.Context, event BusinessEvent) {
	revenueType := bm.getStringProperty(event.Properties, "revenue_type", "subscription")
	tier := bm.getStringProperty(event.Properties, "subscription_tier", "basic")
	
	bm.revenueMetrics.WithLabelValues(revenueType, tier, event.Environment).Add(event.Value)
}

// RecordUserMetrics records comprehensive user metrics
func (bm *BusinessMetricsCollector) RecordUserMetrics(ctx context.Context, metrics UserMetrics) {
	// Record session duration
	if metrics.TotalSessionTime > 0 {
		bm.userSessions.Observe(float64(metrics.TotalSessionTime))
	}
	
	// Update user retention (this would typically be calculated periodically)
	// For now, we'll just log the metrics
	bm.logger.Debug("User metrics recorded",
		zap.String("user_id", metrics.UserID),
		zap.Int("recipes_created", metrics.RecipesCreated),
		zap.Int("recipes_viewed", metrics.RecipesViewed),
		zap.Float64("lifetime_value", metrics.LifetimeValue),
	)
}

// RecordPerformanceImpact records the impact of performance issues on business metrics
func (bm *BusinessMetricsCollector) RecordPerformanceImpact(ctx context.Context, metricType, impactCategory string, score float64) {
	bm.performanceImpact.WithLabelValues(metricType, impactCategory).Set(score)
}

// RecordErrorImpact records the business impact of errors
func (bm *BusinessMetricsCollector) RecordErrorImpact(ctx context.Context, errorType, businessFunction, severity string) {
	bm.errorImpact.WithLabelValues(errorType, businessFunction, severity).Inc()
}

// GetBusinessMetrics returns current business metrics as JSON
func (bm *BusinessMetricsCollector) GetBusinessMetrics(c *gin.Context) {
	// This would typically query a time-series database
	// For now, return current counter values
	metrics := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"metrics": map[string]interface{}{
			"users_active": "Current gauge value would be here",
			"recipes_created_24h": "Sum of counter values for last 24h",
			"ai_requests_1h": "Sum of AI requests for last hour",
			"revenue_today": "Today's revenue sum",
		},
	}
	
	c.JSON(200, metrics)
}

// GetUserEngagementReport generates a user engagement report
func (bm *BusinessMetricsCollector) GetUserEngagementReport(c *gin.Context) {
	timeRange := c.DefaultQuery("timeRange", "24h")
	
	// This would typically query a time-series database
	report := map[string]interface{}{
		"time_range": timeRange,
		"engagement_metrics": map[string]interface{}{
			"daily_active_users": "DAU count",
			"average_session_duration": "Average session time",
			"bounce_rate": "Bounce rate percentage",
			"feature_adoption": map[string]interface{}{
				"ai_features": "AI feature usage percentage",
				"recipe_creation": "Recipe creation rate",
				"search_usage": "Search usage rate",
			},
		},
	}
	
	c.JSON(200, report)
}

// Helper methods

func (bm *BusinessMetricsCollector) getStringProperty(properties map[string]interface{}, key, defaultValue string) string {
	if val, ok := properties[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (bm *BusinessMetricsCollector) getFloatProperty(properties map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := properties[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				return parsed
			}
		}
	}
	return defaultValue
}

// BusinessEventHandler handles HTTP requests for business events
type BusinessEventHandler struct {
	collector *BusinessMetricsCollector
	logger    *zap.Logger
}

// NewBusinessEventHandler creates a new business event handler
func NewBusinessEventHandler(collector *BusinessMetricsCollector, logger *zap.Logger) *BusinessEventHandler {
	return &BusinessEventHandler{
		collector: collector,
		logger:    logger,
	}
}

// HandleEvent handles business event tracking HTTP requests
func (h *BusinessEventHandler) HandleEvent(c *gin.Context) {
	var event BusinessEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		h.logger.Error("Failed to bind business event", zap.Error(err))
		c.JSON(400, gin.H{"error": "Invalid event format"})
		return
	}

	// Set default values
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	if event.Environment == "" {
		event.Environment = "production"
	}

	// Track the event
	h.collector.TrackBusinessEvent(c.Request.Context(), event)
	
	c.JSON(200, gin.H{"status": "success"})
}

// HandleBatchEvents handles batch business event tracking
func (h *BusinessEventHandler) HandleBatchEvents(c *gin.Context) {
	var events []BusinessEvent
	if err := c.ShouldBindJSON(&events); err != nil {
		h.logger.Error("Failed to bind business events batch", zap.Error(err))
		c.JSON(400, gin.H{"error": "Invalid events format"})
		return
	}

	processed := 0
	for _, event := range events {
		if event.Timestamp == 0 {
			event.Timestamp = time.Now().Unix()
		}
		if event.Environment == "" {
			event.Environment = "production"
		}
		
		h.collector.TrackBusinessEvent(c.Request.Context(), event)
		processed++
	}
	
	c.JSON(200, gin.H{
		"status": "success",
		"processed": processed,
	})
}