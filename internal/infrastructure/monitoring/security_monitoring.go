package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// SecurityMonitor handles security event monitoring and threat detection
type SecurityMonitor struct {
	logger         *zap.Logger
	tracing        *TracingProvider
	rateLimiter    *RateLimiter
	threatDetector *ThreatDetector
	anomalyDetector *AnomalyDetector
	
	// Security metrics
	securityEvents        *prometheus.CounterVec
	threatsDetected       *prometheus.CounterVec
	blockedRequests       *prometheus.CounterVec
	authenticationEvents  *prometheus.CounterVec
	suspiciousActivities  *prometheus.CounterVec
	securityScores        *prometheus.GaugeVec
	
	// Rate limiting metrics
	rateLimitHits         *prometheus.CounterVec
	rateLimitBypass       *prometheus.CounterVec
	
	// Anomaly detection metrics
	anomaliesDetected     *prometheus.CounterVec
	behaviorScores        *prometheus.HistogramVec
}

// SecurityEvent represents a security event
type SecurityEvent struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Timestamp   int64                  `json:"timestamp"`
	SourceIP    string                 `json:"source_ip"`
	UserID      string                 `json:"user_id,omitempty"`
	UserAgent   string                 `json:"user_agent"`
	Path        string                 `json:"path"`
	Method      string                 `json:"method"`
	StatusCode  int                    `json:"status_code"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details"`
	ThreatScore float64                `json:"threat_score"`
	Actions     []string               `json:"actions"`
	GeoLocation *GeoLocation           `json:"geo_location,omitempty"`
}

// GeoLocation represents geographical location data
type GeoLocation struct {
	Country     string  `json:"country"`
	Region      string  `json:"region"`
	City        string  `json:"city"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	ISP         string  `json:"isp"`
	Organization string `json:"organization"`
}

// ThreatDetector detects various security threats
type ThreatDetector struct {
	logger                *zap.Logger
	suspiciousIPs         sync.Map // IP -> SuspiciousActivity
	failedLoginAttempts   sync.Map // IP -> LoginAttempts
	rateLimitViolations   sync.Map // IP -> RateViolations
	sqlInjectionPatterns  []string
	xssPatterns          []string
	pathTraversalPatterns []string
	blacklistedIPs       sync.Map // IP -> bool
	whitelistedIPs       sync.Map // IP -> bool
}

// SuspiciousActivity tracks suspicious activity for an IP
type SuspiciousActivity struct {
	IP                  string    `json:"ip"`
	LastSeen           time.Time `json:"last_seen"`
	ThreatScore        float64   `json:"threat_score"`
	FailedLogins       int       `json:"failed_logins"`
	RateLimitViolations int      `json:"rate_limit_violations"`
	SQLInjectionAttempts int     `json:"sql_injection_attempts"`
	XSSAttempts        int       `json:"xss_attempts"`
	PathTraversalAttempts int    `json:"path_traversal_attempts"`
	UnknownEndpoints   int       `json:"unknown_endpoints"`
	SuspiciousUserAgents int     `json:"suspicious_user_agents"`
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	limits    map[string]*RateLimit // endpoint -> limit
	requests  sync.Map              // IP:endpoint -> RequestTracker
	logger    *zap.Logger
}

// RateLimit defines rate limiting configuration
type RateLimit struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	BurstSize        int           `json:"burst_size"`
	WindowSize       time.Duration `json:"window_size"`
}

// RequestTracker tracks requests for rate limiting
type RequestTracker struct {
	Count       int       `json:"count"`
	WindowStart time.Time `json:"window_start"`
	LastRequest time.Time `json:"last_request"`
}

// AnomalyDetector detects behavioral anomalies
type AnomalyDetector struct {
	logger       *zap.Logger
	userBehavior sync.Map // userID -> UserBehaviorProfile
	baselines    map[string]float64 // metric -> baseline value
}

// UserBehaviorProfile tracks user behavior patterns
type UserBehaviorProfile struct {
	UserID                string    `json:"user_id"`
	AverageRequestRate    float64   `json:"average_request_rate"`
	TypicalEndpoints      []string  `json:"typical_endpoints"`
	TypicalTimeWindows    []int     `json:"typical_time_windows"` // hours of day
	AverageSessionLength  float64   `json:"average_session_length"`
	DeviceFingerprints    []string  `json:"device_fingerprints"`
	LocationHistory       []string  `json:"location_history"`
	LastUpdated          time.Time `json:"last_updated"`
}

// NewSecurityMonitor creates a new security monitor
func NewSecurityMonitor(logger *zap.Logger, tracing *TracingProvider) *SecurityMonitor {
	return &SecurityMonitor{
		logger:         logger,
		tracing:        tracing,
		rateLimiter:    NewRateLimiter(logger),
		threatDetector: NewThreatDetector(logger),
		anomalyDetector: NewAnomalyDetector(logger),
		
		securityEvents: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "security_events_total",
			Help: "Total number of security events",
		}, []string{"type", "severity", "source_ip", "action"}),
		
		threatsDetected: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "security_threats_detected_total",
			Help: "Total number of threats detected",
		}, []string{"threat_type", "severity", "source"}),
		
		blockedRequests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "security_blocked_requests_total",
			Help: "Total number of blocked requests",
		}, []string{"reason", "source_ip", "endpoint"}),
		
		authenticationEvents: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "security_authentication_events_total",
			Help: "Total number of authentication events",
		}, []string{"event_type", "status", "method"}),
		
		suspiciousActivities: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "security_suspicious_activities_total",
			Help: "Total number of suspicious activities",
		}, []string{"activity_type", "severity", "source_ip"}),
		
		securityScores: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "security_threat_scores",
			Help: "Threat scores for various entities",
		}, []string{"entity_type", "entity_id"}),
		
		rateLimitHits: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "security_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		}, []string{"endpoint", "source_ip", "action"}),
		
		rateLimitBypass: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "security_rate_limit_bypass_total",
			Help: "Total number of rate limit bypass attempts",
		}, []string{"method", "source_ip"}),
		
		anomaliesDetected: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "security_anomalies_detected_total",
			Help: "Total number of behavioral anomalies detected",
		}, []string{"anomaly_type", "user_id", "severity"}),
		
		behaviorScores: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "security_behavior_scores",
			Help:    "Behavioral anomaly scores",
			Buckets: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		}, []string{"user_id", "behavior_type"}),
	}
}

// SecurityMiddleware creates a Gin middleware for security monitoring
func (sm *SecurityMonitor) SecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		ctx, span := sm.tracing.StartSpan(c.Request.Context(), "security.monitor_request")
		defer span.End()
		
		// Extract request information
		sourceIP := getClientIP(c)
		userAgent := c.GetHeader("User-Agent")
		path := c.Request.URL.Path
		method := c.Request.Method
		userID := getUserIDFromContext(c)
		
		// Check rate limits
		if !sm.rateLimiter.CheckRateLimit(sourceIP, path) {
			sm.recordSecurityEvent(ctx, SecurityEvent{
				Type:        "rate_limit_exceeded",
				Severity:    "medium",
				SourceIP:    sourceIP,
				UserID:      userID,
				Path:        path,
				Method:      method,
				Description: "Rate limit exceeded",
				ThreatScore: 0.3,
			})
			
			sm.blockedRequests.WithLabelValues("rate_limit", sourceIP, path).Inc()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			c.Abort()
			return
		}
		
		// Check for malicious patterns in request
		threatScore := sm.threatDetector.AnalyzeRequest(c.Request)
		if threatScore > 0.5 {
			sm.recordSecurityEvent(ctx, SecurityEvent{
				Type:        "malicious_request",
				Severity:    getSeverityFromScore(threatScore),
				SourceIP:    sourceIP,
				UserID:      userID,
				Path:        path,
				Method:      method,
				UserAgent:   userAgent,
				Description: "Potentially malicious request detected",
				ThreatScore: threatScore,
			})
			
			if threatScore > 0.8 {
				sm.blockedRequests.WithLabelValues("high_threat", sourceIP, path).Inc()
				c.JSON(http.StatusForbidden, gin.H{"error": "Request blocked"})
				c.Abort()
				return
			}
		}
		
		// Process the request
		c.Next()
		
		// Analyze response for security events
		duration := time.Since(start)
		statusCode := c.Writer.Status()
		
		// Check for authentication events
		if isAuthEndpoint(path) {
			sm.recordAuthenticationEvent(ctx, method, statusCode, sourceIP, userID)
		}
		
		// Check for suspicious behavior patterns
		if userID != "" {
			sm.anomalyDetector.AnalyzeUserBehavior(userID, path, duration, sourceIP)
		}
		
		// Update threat detection with response information
		sm.threatDetector.UpdateActivity(sourceIP, path, statusCode, userAgent)
		
		// Record general security metrics
		if statusCode >= 400 {
			sm.recordSecurityEvent(ctx, SecurityEvent{
				Type:        "error_response",
				Severity:    "low",
				SourceIP:    sourceIP,
				UserID:      userID,
				Path:        path,
				Method:      method,
				StatusCode:  statusCode,
				Description: fmt.Sprintf("HTTP error response: %d", statusCode),
				ThreatScore: getErrorThreatScore(statusCode),
			})
		}
	}
}

// NewThreatDetector creates a new threat detector
func NewThreatDetector(logger *zap.Logger) *ThreatDetector {
	return &ThreatDetector{
		logger: logger,
		sqlInjectionPatterns: []string{
			`(?i)(union|select|insert|update|delete|drop|create|alter).*?(from|into|table|database)`,
			`(?i)(or|and)\s+\d+\s*=\s*\d+`,
			`(?i)(or|and)\s+['"]\w+['"]?\s*=\s*['"]\w+['"]?`,
			`(?i)(exec|execute|sp_|xp_)`,
			`(?i)(script|javascript|vbscript|onload|onerror)`,
		},
		xssPatterns: []string{
			`(?i)<script.*?>.*?</script>`,
			`(?i)javascript:`,
			`(?i)on\w+\s*=`,
			`(?i)<iframe.*?>`,
			`(?i)document\.cookie`,
			`(?i)alert\s*\(`,
		},
		pathTraversalPatterns: []string{
			`\.\.\/`,
			`\.\.\\`,
			`%2e%2e%2f`,
			`%2e%2e%5c`,
			`\/etc\/passwd`,
			`\/windows\/system32`,
		},
	}
}

// AnalyzeRequest analyzes a request for threats
func (td *ThreatDetector) AnalyzeRequest(req *http.Request) float64 {
	var threatScore float64
	
	// Check for SQL injection
	if td.checkSQLInjection(req) {
		threatScore += 0.4
	}
	
	// Check for XSS attempts
	if td.checkXSS(req) {
		threatScore += 0.3
	}
	
	// Check for path traversal
	if td.checkPathTraversal(req) {
		threatScore += 0.3
	}
	
	// Check for suspicious user agent
	if td.checkSuspiciousUserAgent(req.UserAgent()) {
		threatScore += 0.2
	}
	
	// Check for unusual request patterns
	if td.checkUnusualPatterns(req) {
		threatScore += 0.1
	}
	
	return threatScore
}

// checkSQLInjection checks for SQL injection patterns
func (td *ThreatDetector) checkSQLInjection(req *http.Request) bool {
	targets := []string{
		req.URL.RawQuery,
		req.URL.Path,
		req.Header.Get("User-Agent"),
		req.Header.Get("Referer"),
	}
	
	for _, target := range targets {
		for _, pattern := range td.sqlInjectionPatterns {
			if matched, _ := regexp.MatchString(pattern, target); matched {
				return true
			}
		}
	}
	
	return false
}

// checkXSS checks for XSS patterns
func (td *ThreatDetector) checkXSS(req *http.Request) bool {
	targets := []string{
		req.URL.RawQuery,
		req.URL.Path,
		req.Header.Get("User-Agent"),
		req.Header.Get("Referer"),
	}
	
	for _, target := range targets {
		for _, pattern := range td.xssPatterns {
			if matched, _ := regexp.MatchString(pattern, target); matched {
				return true
			}
		}
	}
	
	return false
}

// checkPathTraversal checks for path traversal attempts
func (td *ThreatDetector) checkPathTraversal(req *http.Request) bool {
	path := req.URL.Path
	query := req.URL.RawQuery
	
	for _, pattern := range td.pathTraversalPatterns {
		if matched, _ := regexp.MatchString(pattern, path); matched {
			return true
		}
		if matched, _ := regexp.MatchString(pattern, query); matched {
			return true
		}
	}
	
	return false
}

// checkSuspiciousUserAgent checks for suspicious user agents
func (td *ThreatDetector) checkSuspiciousUserAgent(userAgent string) bool {
	suspiciousAgents := []string{
		"sqlmap", "nikto", "nmap", "masscan", "zap", "w3af",
		"gobuster", "dirb", "dirbuster", "wpscan", "burp",
		"python-requests", "curl/", "wget/", "libwww", "lwp",
	}
	
	userAgentLower := strings.ToLower(userAgent)
	for _, agent := range suspiciousAgents {
		if strings.Contains(userAgentLower, agent) {
			return true
		}
	}
	
	return false
}

// checkUnusualPatterns checks for unusual request patterns
func (td *ThreatDetector) checkUnusualPatterns(req *http.Request) bool {
	// Check for unusual content types
	contentType := req.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") && req.Method != "POST" {
		return true
	}
	
	// Check for unusual headers
	suspiciousHeaders := []string{
		"X-Forwarded-For", "X-Real-IP", "X-Originating-IP",
		"X-Remote-IP", "X-Client-IP", "Client-IP",
	}
	
	headerCount := 0
	for _, header := range suspiciousHeaders {
		if req.Header.Get(header) != "" {
			headerCount++
		}
	}
	
	// Multiple forwarding headers might indicate proxy abuse
	return headerCount > 2
}

// UpdateActivity updates suspicious activity tracking
func (td *ThreatDetector) UpdateActivity(sourceIP, path string, statusCode int, userAgent string) {
	activityKey := sourceIP
	
	var activity SuspiciousActivity
	if val, ok := td.suspiciousIPs.Load(activityKey); ok {
		activity = val.(SuspiciousActivity)
	} else {
		activity = SuspiciousActivity{
			IP: sourceIP,
		}
	}
	
	activity.LastSeen = time.Now()
	
	// Update counters based on response
	if statusCode == 401 || statusCode == 403 {
		activity.FailedLogins++
		activity.ThreatScore += 0.1
	}
	
	if statusCode == 404 {
		activity.UnknownEndpoints++
		activity.ThreatScore += 0.05
	}
	
	if td.checkSuspiciousUserAgent(userAgent) {
		activity.SuspiciousUserAgents++
		activity.ThreatScore += 0.1
	}
	
	td.suspiciousIPs.Store(activityKey, activity)
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(logger *zap.Logger) *RateLimiter {
	rl := &RateLimiter{
		logger: logger,
		limits: make(map[string]*RateLimit),
	}
	
	// Set default rate limits
	rl.SetRateLimit("/api/v1/auth/login", &RateLimit{
		RequestsPerMinute: 5,
		BurstSize:        2,
		WindowSize:       time.Minute,
	})
	
	rl.SetRateLimit("/api/v1/auth/register", &RateLimit{
		RequestsPerMinute: 3,
		BurstSize:        1,
		WindowSize:       time.Minute,
	})
	
	rl.SetRateLimit("*", &RateLimit{ // Default limit
		RequestsPerMinute: 60,
		BurstSize:        10,
		WindowSize:       time.Minute,
	})
	
	return rl
}

// SetRateLimit sets rate limit for an endpoint
func (rl *RateLimiter) SetRateLimit(endpoint string, limit *RateLimit) {
	rl.limits[endpoint] = limit
}

// CheckRateLimit checks if request is within rate limits
func (rl *RateLimiter) CheckRateLimit(sourceIP, endpoint string) bool {
	limit := rl.getRateLimitForEndpoint(endpoint)
	if limit == nil {
		return true // No limit configured
	}
	
	key := fmt.Sprintf("%s:%s", sourceIP, endpoint)
	now := time.Now()
	
	var tracker RequestTracker
	if val, ok := rl.requests.Load(key); ok {
		tracker = val.(RequestTracker)
	} else {
		tracker = RequestTracker{
			WindowStart: now,
		}
	}
	
	// Reset window if expired
	if now.Sub(tracker.WindowStart) > limit.WindowSize {
		tracker = RequestTracker{
			Count:       0,
			WindowStart: now,
		}
	}
	
	// Check if within limits
	if tracker.Count >= limit.RequestsPerMinute {
		// Check burst allowance
		if tracker.Count >= limit.RequestsPerMinute+limit.BurstSize {
			return false
		}
		
		// Check burst timing
		if now.Sub(tracker.LastRequest) < time.Minute/time.Duration(limit.BurstSize) {
			return false
		}
	}
	
	tracker.Count++
	tracker.LastRequest = now
	rl.requests.Store(key, tracker)
	
	return true
}

// getRateLimitForEndpoint gets rate limit configuration for endpoint
func (rl *RateLimiter) getRateLimitForEndpoint(endpoint string) *RateLimit {
	if limit, ok := rl.limits[endpoint]; ok {
		return limit
	}
	
	// Return default limit
	return rl.limits["*"]
}

// NewAnomalyDetector creates a new anomaly detector
func NewAnomalyDetector(logger *zap.Logger) *AnomalyDetector {
	return &AnomalyDetector{
		logger: logger,
		baselines: map[string]float64{
			"average_request_rate": 10.0,    // requests per minute
			"session_length":      1800.0,   // 30 minutes
			"endpoint_diversity":  0.7,      // 70% of requests to common endpoints
		},
	}
}

// AnalyzeUserBehavior analyzes user behavior for anomalies
func (ad *AnomalyDetector) AnalyzeUserBehavior(userID, path string, duration time.Duration, sourceIP string) {
	var profile UserBehaviorProfile
	if val, ok := ad.userBehavior.Load(userID); ok {
		profile = val.(UserBehaviorProfile)
	} else {
		profile = UserBehaviorProfile{
			UserID:              userID,
			TypicalEndpoints:    []string{},
			TypicalTimeWindows:  []int{},
			DeviceFingerprints:  []string{},
			LocationHistory:     []string{},
		}
	}
	
	// Update behavior profile
	profile.LastUpdated = time.Now()
	
	// Check for anomalies
	anomalies := ad.detectAnomalies(profile, path, duration, sourceIP)
	
	// Log anomalies
	for _, anomaly := range anomalies {
		ad.logger.Warn("Behavioral anomaly detected",
			zap.String("user_id", userID),
			zap.String("anomaly_type", anomaly.Type),
			zap.Float64("score", anomaly.Score),
			zap.String("description", anomaly.Description),
		)
	}
	
	// Store updated profile
	ad.userBehavior.Store(userID, profile)
}

// Anomaly represents a detected anomaly
type Anomaly struct {
	Type        string  `json:"type"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
}

// detectAnomalies detects behavioral anomalies
func (ad *AnomalyDetector) detectAnomalies(profile UserBehaviorProfile, path string, duration time.Duration, sourceIP string) []Anomaly {
	var anomalies []Anomaly
	
	// Check for unusual endpoint access
	if !ad.isTypicalEndpoint(profile.TypicalEndpoints, path) {
		anomalies = append(anomalies, Anomaly{
			Type:        "unusual_endpoint",
			Score:       0.3,
			Description: fmt.Sprintf("User accessed unusual endpoint: %s", path),
			Severity:    "low",
		})
	}
	
	// Check for unusual timing
	hour := time.Now().Hour()
	if !ad.isTypicalTimeWindow(profile.TypicalTimeWindows, hour) {
		anomalies = append(anomalies, Anomaly{
			Type:        "unusual_timing",
			Score:       0.2,
			Description: fmt.Sprintf("User activity at unusual time: %d:00", hour),
			Severity:    "low",
		})
	}
	
	// Check for location anomalies (simplified IP-based check)
	if !ad.isTypicalLocation(profile.LocationHistory, sourceIP) {
		anomalies = append(anomalies, Anomaly{
			Type:        "unusual_location",
			Score:       0.5,
			Description: fmt.Sprintf("User activity from unusual location: %s", sourceIP),
			Severity:    "medium",
		})
	}
	
	return anomalies
}

// Helper methods

func getClientIP(c *gin.Context) string {
	// Check various headers for real IP
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		// Take the first IP in case of comma-separated list
		if idx := strings.Index(ip, ","); idx != -1 {
			return strings.TrimSpace(ip[:idx])
		}
		return ip
	}
	
	// Fallback to RemoteAddr
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}

func getUserIDFromContext(c *gin.Context) string {
	if userID, exists := c.Get("user_id"); exists {
		return userID.(string)
	}
	return ""
}

func getSeverityFromScore(score float64) string {
	if score >= 0.8 {
		return "critical"
	} else if score >= 0.6 {
		return "high"
	} else if score >= 0.4 {
		return "medium"
	} else {
		return "low"
	}
}

func getErrorThreatScore(statusCode int) float64 {
	switch {
	case statusCode == 401 || statusCode == 403:
		return 0.2
	case statusCode == 404:
		return 0.1
	case statusCode >= 500:
		return 0.05
	default:
		return 0.0
	}
}

func isAuthEndpoint(path string) bool {
	authEndpoints := []string{"/auth/", "/login", "/register", "/logout", "/password"}
	for _, endpoint := range authEndpoints {
		if strings.Contains(path, endpoint) {
			return true
		}
	}
	return false
}

func (sm *SecurityMonitor) recordSecurityEvent(ctx context.Context, event SecurityEvent) {
	// Set timestamp if not provided
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().Unix()
	}
	
	// Generate ID if not provided
	if event.ID == "" {
		event.ID = fmt.Sprintf("sec_%d_%s", event.Timestamp, generateRandomString(8))
	}
	
	// Record metrics
	sm.securityEvents.WithLabelValues(
		event.Type,
		event.Severity,
		event.SourceIP,
		strings.Join(event.Actions, ","),
	).Inc()
	
	// Record threat score
	if event.SourceIP != "" {
		sm.securityScores.WithLabelValues("ip", event.SourceIP).Set(event.ThreatScore)
	}
	if event.UserID != "" {
		sm.securityScores.WithLabelValues("user", event.UserID).Set(event.ThreatScore)
	}
	
	// Log the event
	sm.logger.Warn("Security event recorded",
		zap.String("event_id", event.ID),
		zap.String("type", event.Type),
		zap.String("severity", event.Severity),
		zap.String("source_ip", event.SourceIP),
		zap.String("user_id", event.UserID),
		zap.Float64("threat_score", event.ThreatScore),
		zap.String("description", event.Description),
	)
	
	// Add tracing information
	if sm.tracing != nil {
		sm.tracing.AddSpanEvent(ctx, "security_event",
			"event.id", event.ID,
			"event.type", event.Type,
			"event.severity", event.Severity,
			"event.threat_score", fmt.Sprintf("%.2f", event.ThreatScore),
		)
	}
}

func (sm *SecurityMonitor) recordAuthenticationEvent(ctx context.Context, method string, statusCode int, sourceIP, userID string) {
	var eventType, status string
	
	if strings.Contains(method, "login") {
		eventType = "login_attempt"
	} else if strings.Contains(method, "register") {
		eventType = "registration_attempt"
	} else {
		eventType = "auth_attempt"
	}
	
	if statusCode < 400 {
		status = "success"
	} else {
		status = "failure"
	}
	
	sm.authenticationEvents.WithLabelValues(eventType, status, "password").Inc()
	
	// Record failed login attempts for threat detection
	if status == "failure" && eventType == "login_attempt" {
		sm.threatDetector.recordFailedLogin(sourceIP, userID)
	}
}

func (td *ThreatDetector) recordFailedLogin(sourceIP, userID string) {
	// Update failed login tracking
	key := sourceIP
	attempts := 1
	
	if val, ok := td.failedLoginAttempts.Load(key); ok {
		attempts = val.(int) + 1
	}
	
	td.failedLoginAttempts.Store(key, attempts)
	
	// Update suspicious activity
	var activity SuspiciousActivity
	if val, ok := td.suspiciousIPs.Load(sourceIP); ok {
		activity = val.(SuspiciousActivity)
	} else {
		activity = SuspiciousActivity{IP: sourceIP}
	}
	
	activity.FailedLogins = attempts
	activity.ThreatScore += 0.1
	activity.LastSeen = time.Now()
	
	td.suspiciousIPs.Store(sourceIP, activity)
}

// Helper functions for anomaly detection
func (ad *AnomalyDetector) isTypicalEndpoint(typicalEndpoints []string, path string) bool {
	for _, endpoint := range typicalEndpoints {
		if endpoint == path {
			return true
		}
	}
	return len(typicalEndpoints) == 0 // If no history, consider it normal
}

func (ad *AnomalyDetector) isTypicalTimeWindow(typicalWindows []int, hour int) bool {
	for _, window := range typicalWindows {
		if window == hour {
			return true
		}
	}
	return len(typicalWindows) == 0 // If no history, consider it normal
}

func (ad *AnomalyDetector) isTypicalLocation(locationHistory []string, sourceIP string) bool {
	// Simplified check - in practice, you'd use GeoIP databases
	for _, location := range locationHistory {
		if location == sourceIP {
			return true
		}
	}
	return len(locationHistory) == 0 // If no history, consider it normal
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// Security event handler functions
func (sm *SecurityMonitor) GetSecurityEvents(c *gin.Context) {
	// This would typically query a database or time-series store
	events := []SecurityEvent{
		// Example events would be returned here
	}
	
	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"count":  len(events),
	})
}

func (sm *SecurityMonitor) GetThreatSummary(c *gin.Context) {
	// Return current threat landscape summary
	summary := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"threats": map[string]interface{}{
			"high_risk_ips":      "Count of high-risk IPs",
			"blocked_requests":   "Count of blocked requests today",
			"failed_logins":      "Count of failed login attempts",
			"anomalies_detected": "Count of behavioral anomalies",
		},
	}
	
	c.JSON(http.StatusOK, summary)
}