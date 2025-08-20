# ADR-0019: Logging and Monitoring Standards

## Status
Accepted

## Context
Alchemorsel v3 requires comprehensive observability to ensure system reliability, performance optimization, and effective troubleshooting. Without proper logging and monitoring, issues are discovered too late, root cause analysis is difficult, and system optimization becomes guesswork.

Observability requirements:
- Application performance monitoring and alerting
- Error tracking and debugging information
- User behavior analytics and business metrics
- Infrastructure health and resource utilization
- Security event monitoring and audit trails
- Cost optimization through resource usage insights

Current challenges:
- Inconsistent logging formats across components
- No centralized log aggregation or searching
- Limited visibility into system performance
- Reactive rather than proactive issue detection
- Difficult debugging in production environments

## Decision
We will implement a comprehensive logging and monitoring framework with structured logging, centralized aggregation, and proactive alerting across all system components.

**Logging Standards:**

**Structured Logging Format (JSON):**
```json
{
  "timestamp": "2024-01-01T12:00:00.000Z",
  "level": "INFO",
  "service": "alchemorsel-web",
  "version": "v1.0.0",
  "request_id": "req-uuid-here",
  "user_id": "user-123",
  "message": "User authentication successful",
  "context": {
    "method": "POST",
    "path": "/api/v1/auth/login",
    "duration_ms": 150,
    "status_code": 200
  },
  "metadata": {
    "environment": "production",
    "instance_id": "web-01"
  }
}
```

**Log Levels and Usage:**
- **ERROR**: System errors requiring immediate attention
- **WARN**: Recoverable errors or unusual conditions
- **INFO**: Important system events and user actions
- **DEBUG**: Detailed information for troubleshooting (dev only)
- **TRACE**: Very detailed execution information (dev only)

**Go Logging Implementation:**
```go
// pkg/logger/logger.go
package logger

import (
    "context"
    "github.com/sirupsen/logrus"
    "os"
)

type Logger struct {
    *logrus.Logger
    service string
    version string
}

func NewLogger(service, version string) *Logger {
    log := logrus.New()
    log.SetFormatter(&logrus.JSONFormatter{
        TimestampFormat: time.RFC3339Nano,
    })
    
    // Set log level based on environment
    if os.Getenv("APP_ENV") == "development" {
        log.SetLevel(logrus.DebugLevel)
    } else {
        log.SetLevel(logrus.InfoLevel)
    }
    
    return &Logger{
        Logger:  log,
        service: service,
        version: version,
    }
}

func (l *Logger) WithRequest(ctx context.Context) *logrus.Entry {
    entry := l.WithFields(logrus.Fields{
        "service": l.service,
        "version": l.version,
    })
    
    // Add request context if available
    if requestID := ctx.Value("request_id"); requestID != nil {
        entry = entry.WithField("request_id", requestID)
    }
    
    if userID := ctx.Value("user_id"); userID != nil {
        entry = entry.WithField("user_id", userID)
    }
    
    return entry
}
```

**HTTP Request Logging Middleware:**
```go
func LoggingMiddleware(logger *logger.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            requestID := uuid.New().String()
            
            ctx := context.WithValue(r.Context(), "request_id", requestID)
            r = r.WithContext(ctx)
            
            // Wrap response writer to capture status code
            wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
            
            next.ServeHTTP(wrapped, r)
            
            duration := time.Since(start)
            
            logger.WithRequest(ctx).WithFields(logrus.Fields{
                "method":      r.Method,
                "path":        r.URL.Path,
                "status_code": wrapped.statusCode,
                "duration_ms": duration.Milliseconds(),
                "user_agent":  r.UserAgent(),
                "remote_addr": r.RemoteAddr,
            }).Info("HTTP request completed")
        })
    }
}
```

**Monitoring Architecture:**

**Application Metrics:**
```go
// pkg/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Help: "HTTP request duration in seconds",
        },
        []string{"method", "path", "status_code"},
    )
    
    databaseConnections = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "database_connections_active",
            Help: "Number of active database connections",
        },
        []string{"database"},
    )
    
    aiRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_requests_total",
            Help: "Total number of AI service requests",
        },
        []string{"model", "status"},
    )
)
```

**Docker Compose Monitoring Stack:**
```yaml
services:
  # Application services...
  
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    networks:
      - alchemorsel-network

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3001:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/dashboards:/var/lib/grafana/dashboards
    networks:
      - alchemorsel-network

  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"
    volumes:
      - ./monitoring/loki.yml:/etc/loki/local-config.yaml
    networks:
      - alchemorsel-network

volumes:
  prometheus_data:
  grafana_data:
```

**Alerting Rules:**
```yaml
# monitoring/alerts.yml
groups:
  - name: alchemorsel-alerts
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status_code=~"5.."}[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          
      - alert: DatabaseConnectionsHigh
        expr: database_connections_active > 80
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Database connection pool nearly exhausted"

      - alert: ResponseTimeHigh
        expr: histogram_quantile(0.95, http_request_duration_seconds) > 2.0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "95th percentile response time above 2 seconds"
```

**Log Aggregation and Search:**
- Loki for log aggregation and querying
- Grafana for log visualization and dashboards
- Log retention: 30 days for production, 7 days for development
- Structured queries for efficient log searching

**Dashboard Requirements:**
- System overview: requests/sec, error rate, response time
- Database performance: query time, connection pool, slow queries
- AI service metrics: request count, response time, model usage
- Infrastructure: CPU, memory, disk usage, container health
- Business metrics: user signups, active sessions, feature usage

## Consequences

### Positive
- Comprehensive visibility into system behavior and performance
- Proactive issue detection with automated alerting
- Fast troubleshooting with centralized log aggregation
- Data-driven optimization and capacity planning
- Improved system reliability and user experience
- Audit trail for security and compliance requirements

### Negative
- Additional infrastructure complexity and resource usage
- Learning curve for monitoring tools and query languages
- Storage costs for metrics and log retention
- Alert fatigue if thresholds not properly tuned
- Performance impact from metrics collection

### Neutral
- Industry standard observability practices
- Compatible with cloud monitoring services
- Foundation for future advanced analytics and alerting