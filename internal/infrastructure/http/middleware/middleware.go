// Package middleware provides HTTP middleware components
// following the Chain of Responsibility pattern
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Middleware provides all middleware functions
type Middleware struct {
	config  *config.Config
	logger  *zap.Logger
	limiter *rate.Limiter
	tracer  trace.Tracer
	metrics *Metrics
}

// New creates a new middleware instance
func New(cfg *config.Config, logger *zap.Logger) *Middleware {
	// Create rate limiter
	limiter := rate.NewLimiter(
		rate.Limit(cfg.RateLimit.RequestsPerMin)/60,
		cfg.RateLimit.BurstSize,
	)
	
	// Create tracer
	tracer := otel.Tracer("alchemorsel")
	
	// Initialize metrics
	metrics := NewMetrics()
	
	return &Middleware{
		config:  cfg,
		logger:  logger,
		limiter: limiter,
		tracer:  tracer,
		metrics: metrics,
	}
}

// RequestID adds a unique request ID to the context
func (m *Middleware) RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID exists in header
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		
		// Set request ID in context and response header
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		
		c.Next()
	}
}

// Logger provides structured logging for requests
func (m *Middleware) Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		
		// Process request
		c.Next()
		
		// Skip logging for health checks
		if path == m.config.Monitoring.HealthCheckPath || path == m.config.Monitoring.ReadinessPath {
			return
		}
		
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()
		
		if raw != "" {
			path = path + "?" + raw
		}
		
		// Create log fields
		fields := []zap.Field{
			zap.String("request_id", c.GetString("request_id")),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("ip", clientIP),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
		}
		
		// Add user ID if authenticated
		if userID, exists := c.Get("user_id"); exists {
			fields = append(fields, zap.String("user_id", userID.(string)))
		}
		
		// Log based on status code
		switch {
		case statusCode >= 500:
			m.logger.Error("Server error", append(fields, zap.String("error", errorMessage))...)
		case statusCode >= 400:
			m.logger.Warn("Client error", append(fields, zap.String("error", errorMessage))...)
		case statusCode >= 300:
			m.logger.Info("Redirection", fields...)
		default:
			m.logger.Info("Request completed", fields...)
		}
		
		// Update metrics
		m.metrics.RecordRequest(method, path, statusCode, latency)
	}
}

// Recovery recovers from panics and returns 500 error
func (m *Middleware) Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				m.logger.Error("Panic recovered",
					zap.String("request_id", c.GetString("request_id")),
					zap.Any("error", err),
					zap.String("stack", string(debug.Stack())),
				)
				
				// Return error response
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
					"request_id": c.GetString("request_id"),
				})
			}
		}()
		
		c.Next()
	}
}

// CORS handles Cross-Origin Resource Sharing
func (m *Middleware) CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Server.EnableCORS {
			c.Next()
			return
		}
		
		origin := c.Request.Header.Get("Origin")
		
		// Check if origin is allowed
		if m.isOriginAllowed(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Max-Age", "86400")
		}
		
		// Handle preflight request
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// RateLimit implements rate limiting
func (m *Middleware) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.RateLimit.Enable {
			c.Next()
			return
		}
		
		// Use IP-based rate limiting
		// In production, consider user-based rate limiting
		if !m.limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": "60",
			})
			return
		}
		
		c.Next()
	}
}

// Tracing adds distributed tracing
func (m *Middleware) Tracing() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Monitoring.EnableTracing {
			c.Next()
			return
		}
		
		// Start span
		ctx, span := m.tracer.Start(
			c.Request.Context(),
			fmt.Sprintf("%s %s", c.Request.Method, c.FullPath()),
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.url", c.Request.URL.String()),
				attribute.String("http.user_agent", c.Request.UserAgent()),
				attribute.String("request.id", c.GetString("request_id")),
			),
		)
		defer span.End()
		
		// Update request context
		c.Request = c.Request.WithContext(ctx)
		
		// Process request
		c.Next()
		
		// Add response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int("http.response_size", c.Writer.Size()),
		)
		
		// Record errors
		if len(c.Errors) > 0 {
			span.RecordError(c.Errors.Last())
		}
	}
}

// Security adds security headers
func (m *Middleware) Security() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Set CSP header for production
		if m.config.IsProduction() {
			c.Header("Content-Security-Policy", 
				"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self' data:; "+
				"connect-src 'self';")
		}
		
		// Remove server header
		c.Header("Server", "")
		
		c.Next()
	}
}

// Timeout adds request timeout
func (m *Middleware) Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		
		// Update request context
		c.Request = c.Request.WithContext(ctx)
		
		// Create channel to track completion
		done := make(chan struct{})
		
		go func() {
			c.Next()
			close(done)
		}()
		
		select {
		case <-done:
			// Request completed successfully
		case <-ctx.Done():
			// Timeout occurred
			c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{
				"error": "Request timeout",
			})
		}
	}
}

// ErrorHandler handles errors in a consistent way
func (m *Middleware) ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		
		// Check if there are any errors
		if len(c.Errors) == 0 {
			return
		}
		
		// Get the last error
		err := c.Errors.Last()
		
		// Convert to app error if possible
		var appErr *errors.AppError
		if e, ok := err.Err.(*errors.AppError); ok {
			appErr = e
		} else {
			// Create generic app error
			appErr = errors.NewAppError(
				errors.CodeInternal,
				"An unexpected error occurred",
				err.Error(),
			)
		}
		
		// Log the error
		m.logger.Error("Request error",
			zap.String("request_id", c.GetString("request_id")),
			zap.String("code", string(appErr.Code)),
			zap.String("message", appErr.Message),
			zap.String("details", appErr.Details),
		)
		
		// Send error response
		c.JSON(appErr.StatusCode(), gin.H{
			"error": gin.H{
				"code":    appErr.Code,
				"message": appErr.Message,
				"request_id": c.GetString("request_id"),
			},
		})
	}
}

// Compression adds response compression
func (m *Middleware) Compression() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.config.Server.EnableCompression {
			c.Next()
			return
		}
		
		// Check if client accepts gzip
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		
		// Set gzip writer
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		
		// TODO: Implement actual gzip compression
		// This would typically use a gzip writer wrapper
		
		c.Next()
	}
}

// isOriginAllowed checks if origin is in allowed list
func (m *Middleware) isOriginAllowed(origin string) bool {
	// Allow all origins in development
	if m.config.IsDevelopment() {
		return true
	}
	
	// Check against allowed origins
	for _, allowed := range m.config.Server.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	
	return false
}

// Metrics for monitoring
type Metrics struct {
	requestDuration *prometheus.HistogramVec
	requestCount    *prometheus.CounterVec
	activeRequests  prometheus.Gauge
}

// NewMetrics creates new metrics
func NewMetrics() *Metrics {
	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	
	requestCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	
	activeRequests := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_active_requests",
			Help: "Number of active HTTP requests",
		},
	)
	
	// Register metrics
	prometheus.MustRegister(requestDuration, requestCount, activeRequests)
	
	return &Metrics{
		requestDuration: requestDuration,
		requestCount:    requestCount,
		activeRequests:  activeRequests,
	}
}

// RecordRequest records request metrics
func (m *Metrics) RecordRequest(method, path string, status int, duration time.Duration) {
	statusStr := fmt.Sprintf("%d", status)
	m.requestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	m.requestCount.WithLabelValues(method, path, statusStr).Inc()
}