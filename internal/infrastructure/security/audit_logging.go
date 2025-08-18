// Package security provides comprehensive audit logging capabilities
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// AuditLogger provides security audit logging
type AuditLogger struct {
	logger      *zap.Logger
	redisClient *redis.Client
	encryption  *EncryptionService
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger *zap.Logger, redisClient *redis.Client, encryption *EncryptionService) *AuditLogger {
	return &AuditLogger{
		logger:      logger,
		redisClient: redisClient,
		encryption:  encryption,
	}
}

// AuditEvent represents a security audit event
type AuditEvent struct {
	ID           string                 `json:"id"`
	Timestamp    time.Time              `json:"timestamp"`
	UserID       string                 `json:"user_id,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	Action       string                 `json:"action"`
	Resource     string                 `json:"resource"`
	ResourceID   string                 `json:"resource_id,omitempty"`
	Status       AuditStatus            `json:"status"`
	IPAddress    string                 `json:"ip_address,omitempty"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Risk         RiskLevel              `json:"risk_level"`
	Category     AuditCategory          `json:"category"`
	Compliance   []ComplianceFramework  `json:"compliance,omitempty"`
	Geolocation  *Geolocation           `json:"geolocation,omitempty"`
}

// AuditStatus represents the status of an audited action
type AuditStatus string

const (
	StatusSuccess AuditStatus = "success"
	StatusFailure AuditStatus = "failure"
	StatusBlocked AuditStatus = "blocked"
	StatusWarning AuditStatus = "warning"
)

// RiskLevel represents the risk level of an audit event
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// AuditCategory represents categories of audit events
type AuditCategory string

const (
	CategoryAuthentication AuditCategory = "authentication"
	CategoryAuthorization  AuditCategory = "authorization"
	CategoryDataAccess     AuditCategory = "data_access"
	CategoryDataModify     AuditCategory = "data_modify"
	CategorySystemAccess   AuditCategory = "system_access"
	CategoryConfiguration  AuditCategory = "configuration"
	CategorySecurity       AuditCategory = "security"
	CategoryPrivacy        AuditCategory = "privacy"
	CategoryCompliance     AuditCategory = "compliance"
)

// ComplianceFramework represents compliance frameworks
type ComplianceFramework string

const (
	ComplianceSOC2  ComplianceFramework = "SOC2"
	ComplianceGDPR  ComplianceFramework = "GDPR"
	ComplianceCCPA  ComplianceFramework = "CCPA"
	ComplianceHIPAA ComplianceFramework = "HIPAA"
	CompliancePCI   ComplianceFramework = "PCI-DSS"
	ComplianceISO27001 ComplianceFramework = "ISO27001"
)

// Geolocation represents geographical location information
type Geolocation struct {
	Country     string  `json:"country,omitempty"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
}

// LogAuthentication logs authentication events
func (a *AuditLogger) LogAuthentication(userID, action, ipAddress, userAgent string, success bool, details map[string]interface{}) {
	status := StatusSuccess
	risk := RiskLow
	
	if !success {
		status = StatusFailure
		risk = RiskMedium
		
		// Higher risk for certain failure types
		if action == "login" && details != nil {
			if attempts, ok := details["failed_attempts"].(int); ok && attempts > 3 {
				risk = RiskHigh
			}
		}
	}
	
	event := AuditEvent{
		ID:         a.generateEventID(),
		Timestamp:  time.Now(),
		UserID:     userID,
		Action:     action,
		Resource:   "authentication",
		Status:     status,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
		Risk:       risk,
		Category:   CategoryAuthentication,
		Compliance: []ComplianceFramework{ComplianceSOC2, ComplianceISO27001},
	}
	
	a.logEvent(event)
}

// LogAuthorization logs authorization events
func (a *AuditLogger) LogAuthorization(userID, sessionID, action, resource, resourceID, ipAddress string, allowed bool, roles []string) {
	status := StatusSuccess
	risk := RiskLow
	
	if !allowed {
		status = StatusBlocked
		risk = RiskMedium
	}
	
	details := map[string]interface{}{
		"roles": roles,
	}
	
	event := AuditEvent{
		ID:         a.generateEventID(),
		Timestamp:  time.Now(),
		UserID:     userID,
		SessionID:  sessionID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Status:     status,
		IPAddress:  ipAddress,
		Details:    details,
		Risk:       risk,
		Category:   CategoryAuthorization,
		Compliance: []ComplianceFramework{ComplianceSOC2, ComplianceISO27001},
	}
	
	a.logEvent(event)
}

// LogDataAccess logs data access events
func (a *AuditLogger) LogDataAccess(userID, sessionID, resource, resourceID, action, ipAddress string, sensitive bool, fields []string) {
	risk := RiskLow
	if sensitive {
		risk = RiskMedium
	}
	
	details := map[string]interface{}{
		"fields":    fields,
		"sensitive": sensitive,
	}
	
	compliance := []ComplianceFramework{ComplianceSOC2}
	if sensitive {
		compliance = append(compliance, ComplianceGDPR, ComplianceCCPA)
	}
	
	event := AuditEvent{
		ID:         a.generateEventID(),
		Timestamp:  time.Now(),
		UserID:     userID,
		SessionID:  sessionID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Status:     StatusSuccess,
		IPAddress:  ipAddress,
		Details:    details,
		Risk:       risk,
		Category:   CategoryDataAccess,
		Compliance: compliance,
	}
	
	a.logEvent(event)
}

// LogDataModification logs data modification events
func (a *AuditLogger) LogDataModification(userID, sessionID, resource, resourceID, action, ipAddress string, oldValues, newValues map[string]interface{}) {
	// Encrypt sensitive values for audit trail
	encryptedOld := a.encryptSensitiveFields(oldValues)
	encryptedNew := a.encryptSensitiveFields(newValues)
	
	details := map[string]interface{}{
		"old_values": encryptedOld,
		"new_values": encryptedNew,
		"changed_fields": a.getChangedFields(oldValues, newValues),
	}
	
	event := AuditEvent{
		ID:         a.generateEventID(),
		Timestamp:  time.Now(),
		UserID:     userID,
		SessionID:  sessionID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Status:     StatusSuccess,
		IPAddress:  ipAddress,
		Details:    details,
		Risk:       RiskMedium,
		Category:   CategoryDataModify,
		Compliance: []ComplianceFramework{ComplianceSOC2, ComplianceGDPR, ComplianceCCPA},
	}
	
	a.logEvent(event)
}

// LogSecurityEvent logs security-related events
func (a *AuditLogger) LogSecurityEvent(userID, action, resource, ipAddress, userAgent string, risk RiskLevel, details map[string]interface{}) {
	event := AuditEvent{
		ID:         a.generateEventID(),
		Timestamp:  time.Now(),
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		Status:     StatusWarning,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
		Details:    details,
		Risk:       risk,
		Category:   CategorySecurity,
		Compliance: []ComplianceFramework{ComplianceSOC2, ComplianceISO27001},
	}
	
	a.logEvent(event)
}

// LogDataProcessing logs GDPR data processing events
func (a *AuditLogger) LogDataProcessing(event AuditEvent) {
	event.ID = a.generateEventID()
	event.Timestamp = time.Now()
	event.Category = CategoryPrivacy
	event.Compliance = []ComplianceFramework{ComplianceGDPR, ComplianceCCPA}
	
	a.logEvent(event)
}

// LogSystemAccess logs system-level access events
func (a *AuditLogger) LogSystemAccess(userID, action, resource, ipAddress string, success bool, details map[string]interface{}) {
	status := StatusSuccess
	risk := RiskHigh // System access is always high risk
	
	if !success {
		status = StatusFailure
		risk = RiskCritical
	}
	
	event := AuditEvent{
		ID:         a.generateEventID(),
		Timestamp:  time.Now(),
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		Status:     status,
		IPAddress:  ipAddress,
		Details:    details,
		Risk:       risk,
		Category:   CategorySystemAccess,
		Compliance: []ComplianceFramework{ComplianceSOC2, ComplianceISO27001},
	}
	
	a.logEvent(event)
}

// LogConfigurationChange logs configuration changes
func (a *AuditLogger) LogConfigurationChange(userID, component, setting, oldValue, newValue, ipAddress string) {
	details := map[string]interface{}{
		"component":  component,
		"setting":    setting,
		"old_value":  oldValue,
		"new_value":  newValue,
	}
	
	event := AuditEvent{
		ID:         a.generateEventID(),
		Timestamp:  time.Now(),
		UserID:     userID,
		Action:     "configuration_change",
		Resource:   "system_configuration",
		Status:     StatusSuccess,
		IPAddress:  ipAddress,
		Details:    details,
		Risk:       RiskHigh,
		Category:   CategoryConfiguration,
		Compliance: []ComplianceFramework{ComplianceSOC2, ComplianceISO27001},
	}
	
	a.logEvent(event)
}

// AuditMiddleware provides HTTP request auditing
func (a *AuditLogger) AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Capture request details
		userID := c.GetString("user_id")
		sessionID := c.GetString("session_id")
		
		// Process request
		c.Next()
		
		// Determine if this needs auditing
		if a.shouldAuditRequest(c) {
			duration := time.Since(start)
			
			details := map[string]interface{}{
				"method":       c.Request.Method,
				"path":         c.Request.URL.Path,
				"query":        c.Request.URL.RawQuery,
				"status_code":  c.Writer.Status(),
				"duration_ms":  duration.Milliseconds(),
				"request_size": c.Request.ContentLength,
				"response_size": c.Writer.Size(),
			}
			
			// Add form data for POST/PUT (excluding sensitive fields)
			if c.Request.Method == "POST" || c.Request.Method == "PUT" {
				if form := a.sanitizeFormData(c.Request.Form); len(form) > 0 {
					details["form_data"] = form
				}
			}
			
			risk := a.determineRequestRisk(c)
			category := a.determineRequestCategory(c)
			
			event := AuditEvent{
				ID:         a.generateEventID(),
				Timestamp:  start,
				UserID:     userID,
				SessionID:  sessionID,
				Action:     fmt.Sprintf("%s_%s", c.Request.Method, c.Request.URL.Path),
				Resource:   "http_request",
				Status:     a.determineStatus(c.Writer.Status()),
				IPAddress:  c.ClientIP(),
				UserAgent:  c.Request.UserAgent(),
				Details:    details,
				Risk:       risk,
				Category:   category,
				Compliance: []ComplianceFramework{ComplianceSOC2},
			}
			
			a.logEvent(event)
		}
	}
}

// logEvent stores the audit event
func (a *AuditLogger) logEvent(event AuditEvent) {
	// Log to structured logger
	fields := []zap.Field{
		zap.String("audit_event_id", event.ID),
		zap.String("user_id", event.UserID),
		zap.String("action", event.Action),
		zap.String("resource", event.Resource),
		zap.String("status", string(event.Status)),
		zap.String("risk", string(event.Risk)),
		zap.String("category", string(event.Category)),
		zap.String("ip_address", event.IPAddress),
	}
	
	if event.Details != nil {
		fields = append(fields, zap.Any("details", event.Details))
	}
	
	switch event.Risk {
	case RiskCritical:
		a.logger.Error("Critical audit event", fields...)
	case RiskHigh:
		a.logger.Warn("High risk audit event", fields...)
	case RiskMedium:
		a.logger.Info("Medium risk audit event", fields...)
	default:
		a.logger.Debug("Audit event", fields...)
	}
	
	// Store in Redis for compliance reporting
	a.storeAuditEvent(event)
	
	// Send alerts for high-risk events
	if event.Risk == RiskHigh || event.Risk == RiskCritical {
		a.sendSecurityAlert(event)
	}
}

// storeAuditEvent stores audit event in Redis
func (a *AuditLogger) storeAuditEvent(event AuditEvent) {
	ctx := context.Background()
	
	// Serialize event
	data, err := json.Marshal(event)
	if err != nil {
		a.logger.Error("Failed to marshal audit event", zap.Error(err))
		return
	}
	
	// Store with multiple keys for different access patterns
	
	// By event ID
	key := fmt.Sprintf("audit:event:%s", event.ID)
	a.redisClient.Set(ctx, key, data, 90*24*time.Hour) // 90 days retention
	
	// By user ID
	if event.UserID != "" {
		userKey := fmt.Sprintf("audit:user:%s", event.UserID)
		a.redisClient.ZAdd(ctx, userKey, redis.Z{
			Score:  float64(event.Timestamp.Unix()),
			Member: event.ID,
		})
		a.redisClient.Expire(ctx, userKey, 90*24*time.Hour)
	}
	
	// By category
	categoryKey := fmt.Sprintf("audit:category:%s", event.Category)
	a.redisClient.ZAdd(ctx, categoryKey, redis.Z{
		Score:  float64(event.Timestamp.Unix()),
		Member: event.ID,
	})
	a.redisClient.Expire(ctx, categoryKey, 90*24*time.Hour)
	
	// By risk level
	if event.Risk == RiskHigh || event.Risk == RiskCritical {
		riskKey := fmt.Sprintf("audit:risk:%s", event.Risk)
		a.redisClient.ZAdd(ctx, riskKey, redis.Z{
			Score:  float64(event.Timestamp.Unix()),
			Member: event.ID,
		})
		a.redisClient.Expire(ctx, riskKey, 90*24*time.Hour)
	}
}

// Helper methods

func (a *AuditLogger) generateEventID() string {
	return fmt.Sprintf("audit_%d_%s", time.Now().UnixNano(), GenerateSecureToken(8))
}

func (a *AuditLogger) encryptSensitiveFields(data map[string]interface{}) map[string]interface{} {
	sensitiveFields := []string{"password", "email", "phone", "address", "ssn", "credit_card"}
	
	result := make(map[string]interface{})
	for key, value := range data {
		// Check if field is sensitive
		isSensitive := false
		for _, field := range sensitiveFields {
			if strings.Contains(strings.ToLower(key), field) {
				isSensitive = true
				break
			}
		}
		
		if isSensitive {
			if str, ok := value.(string); ok && str != "" {
				encrypted, err := a.encryption.EncryptStringToBase64(str)
				if err == nil {
					result[key] = fmt.Sprintf("encrypted:%s", encrypted[:20]+"...")
				} else {
					result[key] = "[ENCRYPTION_FAILED]"
				}
			}
		} else {
			result[key] = value
		}
	}
	
	return result
}

func (a *AuditLogger) getChangedFields(oldValues, newValues map[string]interface{}) []string {
	var changed []string
	
	// Check for changed fields
	for key, newVal := range newValues {
		if oldVal, exists := oldValues[key]; !exists || oldVal != newVal {
			changed = append(changed, key)
		}
	}
	
	// Check for removed fields
	for key := range oldValues {
		if _, exists := newValues[key]; !exists {
			changed = append(changed, key)
		}
	}
	
	return changed
}

func (a *AuditLogger) shouldAuditRequest(c *gin.Context) bool {
	// Skip health checks and metrics
	skipPaths := []string{"/health", "/metrics", "/ready", "/favicon.ico"}
	for _, path := range skipPaths {
		if c.Request.URL.Path == path {
			return false
		}
	}
	
	// Always audit write operations
	if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
		return true
	}
	
	// Audit authenticated requests
	if c.GetString("user_id") != "" {
		return true
	}
	
	// Audit failed requests
	if c.Writer.Status() >= 400 {
		return true
	}
	
	return false
}

func (a *AuditLogger) sanitizeFormData(form map[string][]string) map[string]interface{} {
	sensitiveFields := []string{"password", "token", "secret", "key"}
	result := make(map[string]interface{})
	
	for key, values := range form {
		isSensitive := false
		for _, field := range sensitiveFields {
			if strings.Contains(strings.ToLower(key), field) {
				isSensitive = true
				break
			}
		}
		
		if isSensitive {
			result[key] = "[REDACTED]"
		} else {
			if len(values) == 1 {
				result[key] = values[0]
			} else {
				result[key] = values
			}
		}
	}
	
	return result
}

func (a *AuditLogger) determineRequestRisk(c *gin.Context) RiskLevel {
	// High risk operations
	if strings.Contains(c.Request.URL.Path, "/admin") ||
		strings.Contains(c.Request.URL.Path, "/config") ||
		strings.Contains(c.Request.URL.Path, "/system") {
		return RiskHigh
	}
	
	// Medium risk for auth operations
	if strings.Contains(c.Request.URL.Path, "/auth") ||
		strings.Contains(c.Request.URL.Path, "/login") ||
		strings.Contains(c.Request.URL.Path, "/register") {
		return RiskMedium
	}
	
	// Failed requests
	if c.Writer.Status() >= 400 {
		return RiskMedium
	}
	
	return RiskLow
}

func (a *AuditLogger) determineRequestCategory(c *gin.Context) AuditCategory {
	path := c.Request.URL.Path
	
	if strings.Contains(path, "/auth") || strings.Contains(path, "/login") {
		return CategoryAuthentication
	}
	
	if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
		return CategoryDataModify
	}
	
	return CategoryDataAccess
}

func (a *AuditLogger) determineStatus(statusCode int) AuditStatus {
	if statusCode >= 200 && statusCode < 300 {
		return StatusSuccess
	} else if statusCode >= 400 && statusCode < 500 {
		return StatusFailure
	} else if statusCode >= 500 {
		return StatusFailure
	}
	return StatusWarning
}

func (a *AuditLogger) sendSecurityAlert(event AuditEvent) {
	// Implementation would send alerts via email, Slack, PagerDuty, etc.
	a.logger.Error("Security alert triggered",
		zap.String("event_id", event.ID),
		zap.String("risk", string(event.Risk)),
		zap.String("action", event.Action),
		zap.String("user_id", event.UserID),
		zap.String("ip_address", event.IPAddress),
	)
}

// GetAuditEvents retrieves audit events with filtering
func (a *AuditLogger) GetAuditEvents(filters AuditFilters) ([]AuditEvent, error) {
	// Implementation would query Redis and filter results
	// This is a placeholder for the actual implementation
	return []AuditEvent{}, nil
}

// AuditFilters represents filters for audit event queries
type AuditFilters struct {
	UserID     string        `json:"user_id,omitempty"`
	Category   AuditCategory `json:"category,omitempty"`
	Risk       RiskLevel     `json:"risk,omitempty"`
	StartTime  time.Time     `json:"start_time,omitempty"`
	EndTime    time.Time     `json:"end_time,omitempty"`
	IPAddress  string        `json:"ip_address,omitempty"`
	Action     string        `json:"action,omitempty"`
	Status     AuditStatus   `json:"status,omitempty"`
	Limit      int           `json:"limit,omitempty"`
}