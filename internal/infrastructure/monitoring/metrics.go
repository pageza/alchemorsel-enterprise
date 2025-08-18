package monitoring

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// MetricsCollector handles Prometheus metrics collection
type MetricsCollector struct {
	logger *zap.Logger
	
	// HTTP metrics
	httpRequestsTotal     *prometheus.CounterVec
	httpRequestDuration   *prometheus.HistogramVec
	httpRequestSize       *prometheus.HistogramVec
	httpResponseSize      *prometheus.HistogramVec
	
	// Business metrics
	recipesCreatedTotal    prometheus.Counter
	recipesViewedTotal     prometheus.Counter
	usersRegisteredTotal   prometheus.Counter
	aiRequestsTotal        *prometheus.CounterVec
	aiRequestDuration     *prometheus.HistogramVec
	
	// System metrics
	dbConnectionsActive    prometheus.Gauge
	dbConnectionsIdle      prometheus.Gauge
	dbQueryDuration       *prometheus.HistogramVec
	cacheHitRatio         *prometheus.GaugeVec
	cacheOperations       *prometheus.CounterVec
	
	// SLA/SLO metrics
	uptimeSeconds         prometheus.Counter
	errorRateTotal        *prometheus.CounterVec
	latencyP95            prometheus.Histogram
	latencyP99            prometheus.Histogram
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger *zap.Logger) *MetricsCollector {
	return &MetricsCollector{
		logger: logger,
		
		// HTTP metrics
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status_code"},
		),
		httpRequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "HTTP request size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method", "path"},
		),
		httpResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method", "path", "status_code"},
		),
		
		// Business metrics
		recipesCreatedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "recipes_created_total",
				Help: "Total number of recipes created",
			},
		),
		recipesViewedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "recipes_viewed_total",
				Help: "Total number of recipe views",
			},
		),
		usersRegisteredTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "users_registered_total",
				Help: "Total number of users registered",
			},
		),
		aiRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_requests_total",
				Help: "Total number of AI requests",
			},
			[]string{"provider", "model", "status"},
		),
		aiRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ai_request_duration_seconds",
				Help:    "AI request duration in seconds",
				Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
			},
			[]string{"provider", "model"},
		),
		
		// System metrics
		dbConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_active",
				Help: "Number of active database connections",
			},
		),
		dbConnectionsIdle: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_idle",
				Help: "Number of idle database connections",
			},
		),
		dbQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
			},
			[]string{"operation", "table"},
		),
		cacheHitRatio: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cache_hit_ratio",
				Help: "Cache hit ratio",
			},
			[]string{"cache_type"},
		),
		cacheOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cache_operations_total",
				Help: "Total number of cache operations",
			},
			[]string{"operation", "cache_type", "status"},
		),
		
		// SLA/SLO metrics
		uptimeSeconds: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "uptime_seconds_total",
				Help: "Total uptime in seconds",
			},
		),
		errorRateTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "error_rate_total",
				Help: "Total error rate",
			},
			[]string{"service", "error_type"},
		),
		latencyP95: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "latency_p95_seconds",
				Help:    "95th percentile latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
		),
		latencyP99: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "latency_p99_seconds",
				Help:    "99th percentile latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
		),
	}
}

// HTTPMiddleware creates a Gin middleware for HTTP metrics collection
func (m *MetricsCollector) HTTPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Record request size
		if c.Request.ContentLength > 0 {
			m.httpRequestSize.WithLabelValues(
				c.Request.Method,
				c.FullPath(),
			).Observe(float64(c.Request.ContentLength))
		}
		
		// Process request
		c.Next()
		
		// Record metrics after request processing
		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(c.Writer.Status())
		
		m.httpRequestsTotal.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			statusCode,
		).Inc()
		
		m.httpRequestDuration.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			statusCode,
		).Observe(duration)
		
		// Record response size
		m.httpResponseSize.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			statusCode,
		).Observe(float64(c.Writer.Size()))
		
		// Record latency percentiles
		m.latencyP95.Observe(duration)
		m.latencyP99.Observe(duration)
		
		// Record errors
		if c.Writer.Status() >= 400 {
			errorType := "client_error"
			if c.Writer.Status() >= 500 {
				errorType = "server_error"
			}
			m.errorRateTotal.WithLabelValues("http", errorType).Inc()
		}
	}
}

// Business metric methods
func (m *MetricsCollector) RecipeCreated() {
	m.recipesCreatedTotal.Inc()
}

func (m *MetricsCollector) RecipeViewed() {
	m.recipesViewedTotal.Inc()
}

func (m *MetricsCollector) UserRegistered() {
	m.usersRegisteredTotal.Inc()
}

func (m *MetricsCollector) AIRequest(provider, model, status string, duration time.Duration) {
	m.aiRequestsTotal.WithLabelValues(provider, model, status).Inc()
	m.aiRequestDuration.WithLabelValues(provider, model).Observe(duration.Seconds())
}

// System metric methods
func (m *MetricsCollector) UpdateDBConnections(active, idle int) {
	m.dbConnectionsActive.Set(float64(active))
	m.dbConnectionsIdle.Set(float64(idle))
}

func (m *MetricsCollector) DBQuery(operation, table string, duration time.Duration) {
	m.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

func (m *MetricsCollector) CacheOperation(operation, cacheType, status string) {
	m.cacheOperations.WithLabelValues(operation, cacheType, status).Inc()
}

func (m *MetricsCollector) UpdateCacheHitRatio(cacheType string, ratio float64) {
	m.cacheHitRatio.WithLabelValues(cacheType).Set(ratio)
}

func (m *MetricsCollector) RecordError(service, errorType string) {
	m.errorRateTotal.WithLabelValues(service, errorType).Inc()
}

// StartUptimeCounter starts the uptime counter
func (m *MetricsCollector) StartUptimeCounter(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.uptimeSeconds.Inc()
		}
	}
}

// Handler returns the Prometheus metrics HTTP handler
func (m *MetricsCollector) Handler() http.Handler {
	return promhttp.Handler()
}