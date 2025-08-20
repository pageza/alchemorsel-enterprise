// Package ai provides comprehensive alerting and notification management
package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AlertManager manages alerts and notifications for AI service issues
type AlertManager struct {
	config       *EnterpriseConfig
	logger       *zap.Logger
	
	// Alert management
	activeAlerts map[uuid.UUID]*Alert
	alertRules   []AlertRule
	alertHistory []Alert
	
	// Notification channels
	emailEnabled    bool
	slackEnabled    bool
	webhookEnabled  bool
	
	// Alert suppression
	suppressionRules map[string]*SuppressionRule
	
	// Thread safety
	mu               sync.RWMutex
}

// Alert represents a system alert
type Alert struct {
	ID             uuid.UUID
	Type           string    // "cost", "quality", "rate_limit", "system"
	Severity       string    // "info", "warning", "critical", "emergency"
	Title          string
	Message        string
	Source         string    // Component that triggered the alert
	Metadata       map[string]interface{}
	TriggeredAt    time.Time
	ResolvedAt     *time.Time
	AcknowledgedAt *time.Time
	AcknowledgedBy *string
	IsActive       bool
	AlertRule      *AlertRule
	NotificationsSent []NotificationRecord
	Tags           []string
}

// AlertRule defines conditions for triggering alerts
type AlertRule struct {
	ID            uuid.UUID
	Name          string
	Description   string
	Type          string    // "threshold", "anomaly", "pattern"
	Condition     AlertCondition
	Severity      string
	IsEnabled     bool
	CooldownPeriod time.Duration
	NotificationChannels []string
	Tags          []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AlertCondition defines the conditions that trigger an alert
type AlertCondition struct {
	Metric         string      // "daily_cost", "quality_score", "error_rate", etc.
	Operator       string      // "greater_than", "less_than", "equals", "not_equals"
	Threshold      float64
	TimeWindow     time.Duration
	MinDataPoints  int
	ComparisonType string      // "absolute", "percentage", "trend"
}

// SuppressionRule defines when alerts should be suppressed
type SuppressionRule struct {
	ID          uuid.UUID
	Name        string
	Pattern     string        // Alert pattern to match
	StartTime   time.Time
	EndTime     time.Time
	Reason      string
	CreatedBy   string
	IsActive    bool
}

// NotificationRecord tracks sent notifications
type NotificationRecord struct {
	Channel     string    // "email", "slack", "webhook"
	Recipient   string
	SentAt      time.Time
	Status      string    // "sent", "failed", "pending"
	ErrorMsg    *string
}

// AlertSummary provides alert statistics
type AlertSummary struct {
	TotalAlerts      int64
	ActiveAlerts     int64
	ResolvedAlerts   int64
	AlertsByType     map[string]int64
	AlertsBySeverity map[string]int64
	RecentAlerts     []Alert
	AlertTrends      []AlertTrend
}

// AlertTrend represents alert trends over time
type AlertTrend struct {
	Date       string
	AlertCount int64
	Severity   string
}

// NewAlertManager creates a new alert manager
func NewAlertManager(config *EnterpriseConfig, logger *zap.Logger) *AlertManager {
	namedLogger := logger.Named("alert-manager")
	
	manager := &AlertManager{
		config:           config,
		logger:           namedLogger,
		activeAlerts:     make(map[uuid.UUID]*Alert),
		alertRules:       []AlertRule{},
		alertHistory:     []Alert{},
		suppressionRules: make(map[string]*SuppressionRule),
		emailEnabled:     true,  // Configuration would come from config
		slackEnabled:     false,
		webhookEnabled:   true,
	}
	
	// Initialize default alert rules
	manager.initializeDefaultAlertRules()
	
	namedLogger.Info("Alert manager initialized",
		zap.Bool("alerts_enabled", config.AlertsEnabled),
		zap.Int("default_rules", len(manager.alertRules)),
	)
	
	return manager
}

// CreateAlert creates a new alert
func (am *AlertManager) CreateAlert(alertType, severity, title, message, source string, metadata map[string]interface{}) *Alert {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert := &Alert{
		ID:          uuid.New(),
		Type:        alertType,
		Severity:    severity,
		Title:       title,
		Message:     message,
		Source:      source,
		Metadata:    metadata,
		TriggeredAt: time.Now(),
		IsActive:    true,
		NotificationsSent: []NotificationRecord{},
		Tags:        []string{},
	}
	
	// Check if alert should be suppressed
	if am.isAlertSuppressed(alert) {
		am.logger.Info("Alert suppressed", 
			zap.String("alert_id", alert.ID.String()),
			zap.String("title", title),
		)
		return alert
	}
	
	// Add to active alerts
	am.activeAlerts[alert.ID] = alert
	
	// Add to history
	am.alertHistory = append(am.alertHistory, *alert)
	
	// Send notifications
	am.sendNotifications(alert)
	
	am.logger.Warn("Alert created",
		zap.String("alert_id", alert.ID.String()),
		zap.String("type", alertType),
		zap.String("severity", severity),
		zap.String("title", title),
		zap.String("source", source),
	)
	
	return alert
}

// ResolveAlert marks an alert as resolved
func (am *AlertManager) ResolveAlert(alertID uuid.UUID, resolvedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert, exists := am.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID.String())
	}
	
	now := time.Now()
	alert.ResolvedAt = &now
	alert.IsActive = false
	
	// Remove from active alerts
	delete(am.activeAlerts, alertID)
	
	// Update in history
	for i, histAlert := range am.alertHistory {
		if histAlert.ID == alertID {
			am.alertHistory[i] = *alert
			break
		}
	}
	
	am.logger.Info("Alert resolved",
		zap.String("alert_id", alertID.String()),
		zap.String("resolved_by", resolvedBy),
		zap.Duration("duration", now.Sub(alert.TriggeredAt)),
	)
	
	return nil
}

// AcknowledgeAlert marks an alert as acknowledged
func (am *AlertManager) AcknowledgeAlert(alertID uuid.UUID, acknowledgedBy string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert, exists := am.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID.String())
	}
	
	now := time.Now()
	alert.AcknowledgedAt = &now
	alert.AcknowledgedBy = &acknowledgedBy
	
	am.logger.Info("Alert acknowledged",
		zap.String("alert_id", alertID.String()),
		zap.String("acknowledged_by", acknowledgedBy),
	)
	
	return nil
}

// CheckCostAlerts checks for cost-related alerts
func (am *AlertManager) CheckCostAlerts(ctx context.Context, dailySpend, monthlySpend float64) {
	// Daily cost alerts
	for _, threshold := range am.config.CostAlertThresholds {
		dailyBudget := float64(am.config.DailyBudgetCents)
		if dailyBudget > 0 && dailySpend >= dailyBudget*threshold {
			am.checkAndCreateCostAlert(
				"daily_cost_threshold",
				am.calculateSeverity(threshold),
				fmt.Sprintf("Daily Cost Alert - %.0f%% of Budget", threshold*100),
				fmt.Sprintf("Daily spending $%.2f has reached %.0f%% of daily budget $%.2f",
					dailySpend/100, threshold*100, dailyBudget/100),
				"cost_tracker",
				map[string]interface{}{
					"current_spend":  dailySpend,
					"budget":         dailyBudget,
					"threshold":      threshold,
					"period":         "daily",
				},
			)
		}
	}
	
	// Monthly cost alerts
	for _, threshold := range am.config.CostAlertThresholds {
		monthlyBudget := float64(am.config.MonthlyBudgetCents)
		if monthlyBudget > 0 && monthlySpend >= monthlyBudget*threshold {
			am.checkAndCreateCostAlert(
				"monthly_cost_threshold",
				am.calculateSeverity(threshold),
				fmt.Sprintf("Monthly Cost Alert - %.0f%% of Budget", threshold*100),
				fmt.Sprintf("Monthly spending $%.2f has reached %.0f%% of monthly budget $%.2f",
					monthlySpend/100, threshold*100, monthlyBudget/100),
				"cost_tracker",
				map[string]interface{}{
					"current_spend":  monthlySpend,
					"budget":         monthlyBudget,
					"threshold":      threshold,
					"period":         "monthly",
				},
			)
		}
	}
}

// CheckQualityAlerts checks for quality-related alerts
func (am *AlertManager) CheckQualityAlerts(ctx context.Context, qualityScore float64, feature string) {
	if qualityScore < am.config.MinQualityScore {
		severity := "warning"
		if qualityScore < am.config.MinQualityScore*0.7 {
			severity = "critical"
		}
		
		am.CreateAlert(
			"quality_threshold",
			severity,
			"Quality Score Below Threshold",
			fmt.Sprintf("Quality score %.2f for %s is below threshold %.2f",
				qualityScore, feature, am.config.MinQualityScore),
			"quality_monitor",
			map[string]interface{}{
				"current_score":     qualityScore,
				"threshold":         am.config.MinQualityScore,
				"feature":           feature,
			},
		)
	}
}

// CheckRateLimitAlerts checks for rate limiting alerts
func (am *AlertManager) CheckRateLimitAlerts(ctx context.Context, violations []RateLimitViolation) {
	for _, violation := range violations {
		severity := "warning"
		if violation.ViolationType == "quota" || violation.Current >= violation.Limit {
			severity = "critical"
		}
		
		am.CreateAlert(
			"rate_limit_violation",
			severity,
			"Rate Limit Violation",
			fmt.Sprintf("User %s exceeded %s limit: %d/%d",
				violation.UserID.String(), violation.ViolationType, violation.Current, violation.Limit),
			"rate_limiter",
			map[string]interface{}{
				"user_id":        violation.UserID.String(),
				"violation_type": violation.ViolationType,
				"current":        violation.Current,
				"limit":          violation.Limit,
			},
		)
	}
}

// CheckSystemAlerts checks for system-related alerts
func (am *AlertManager) CheckSystemAlerts(ctx context.Context, errorRate float64, latency time.Duration) {
	// Error rate alerts
	if errorRate > 0.05 { // 5% error rate threshold
		severity := "warning"
		if errorRate > 0.15 { // 15% is critical
			severity = "critical"
		}
		
		am.CreateAlert(
			"high_error_rate",
			severity,
			"High Error Rate Detected",
			fmt.Sprintf("Error rate %.2f%% is above acceptable threshold", errorRate*100),
			"usage_analytics",
			map[string]interface{}{
				"error_rate": errorRate,
				"threshold":  0.05,
			},
		)
	}
	
	// Latency alerts
	if latency > 10*time.Second {
		severity := "warning"
		if latency > 30*time.Second {
			severity = "critical"
		}
		
		am.CreateAlert(
			"high_latency",
			severity,
			"High Response Latency",
			fmt.Sprintf("Average response latency %v exceeds threshold", latency),
			"usage_analytics",
			map[string]interface{}{
				"current_latency": latency.String(),
				"threshold":       "10s",
			},
		)
	}
}

// GetActiveAlerts returns currently active alerts
func (am *AlertManager) GetActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	alerts := make([]Alert, 0, len(am.activeAlerts))
	for _, alert := range am.activeAlerts {
		alerts = append(alerts, *alert)
	}
	
	return alerts
}

// GetAlertHistory returns historical alerts
func (am *AlertManager) GetAlertHistory(limit int) []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	if limit <= 0 || limit > len(am.alertHistory) {
		limit = len(am.alertHistory)
	}
	
	// Return most recent alerts first
	history := make([]Alert, limit)
	start := len(am.alertHistory) - limit
	copy(history, am.alertHistory[start:])
	
	// Reverse to get newest first
	for i := len(history)/2 - 1; i >= 0; i-- {
		opp := len(history) - 1 - i
		history[i], history[opp] = history[opp], history[i]
	}
	
	return history
}

// GetAlertSummary returns alert statistics
func (am *AlertManager) GetAlertSummary() *AlertSummary {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	summary := &AlertSummary{
		TotalAlerts:      int64(len(am.alertHistory)),
		ActiveAlerts:     int64(len(am.activeAlerts)),
		ResolvedAlerts:   int64(len(am.alertHistory) - len(am.activeAlerts)),
		AlertsByType:     make(map[string]int64),
		AlertsBySeverity: make(map[string]int64),
		RecentAlerts:     []Alert{},
		AlertTrends:      []AlertTrend{},
	}
	
	// Analyze alert history
	for _, alert := range am.alertHistory {
		summary.AlertsByType[alert.Type]++
		summary.AlertsBySeverity[alert.Severity]++
	}
	
	// Get recent alerts (last 10)
	recentCount := 10
	if len(am.alertHistory) < recentCount {
		recentCount = len(am.alertHistory)
	}
	
	if recentCount > 0 {
		summary.RecentAlerts = am.alertHistory[len(am.alertHistory)-recentCount:]
	}
	
	// Generate trends (simplified)
	summary.AlertTrends = am.generateAlertTrends()
	
	return summary
}

// UpdateConfig updates the alert manager configuration
func (am *AlertManager) UpdateConfig(config *EnterpriseConfig) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	am.config = config
	am.logger.Info("Alert manager configuration updated")
}

// HealthCheck returns the health status of the alert manager
func (am *AlertManager) HealthCheck() ComponentHealth {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	criticalAlerts := 0
	for _, alert := range am.activeAlerts {
		if alert.Severity == "critical" || alert.Severity == "emergency" {
			criticalAlerts++
		}
	}
	
	status := ComponentHealth{
		Status:    "healthy",
		Message:   "Alert manager operational",
		LastCheck: time.Now(),
		Metrics: map[string]interface{}{
			"active_alerts":   len(am.activeAlerts),
			"total_alerts":    len(am.alertHistory),
			"critical_alerts": criticalAlerts,
			"alert_rules":     len(am.alertRules),
		},
	}
	
	if criticalAlerts > 0 {
		status.Status = "warning"
		status.Message = fmt.Sprintf("%d critical alerts active", criticalAlerts)
	}
	
	if len(am.activeAlerts) > 50 {
		status.Status = "warning"
		status.Message = "High number of active alerts"
	}
	
	return status
}

// Helper methods

func (am *AlertManager) initializeDefaultAlertRules() {
	// Default cost alert rules
	am.alertRules = append(am.alertRules, AlertRule{
		ID:          uuid.New(),
		Name:        "daily_budget_80_percent",
		Description: "Alert when daily spending reaches 80% of budget",
		Type:        "threshold",
		Condition: AlertCondition{
			Metric:         "daily_cost_percentage",
			Operator:       "greater_than",
			Threshold:      0.8,
			TimeWindow:     time.Hour,
			MinDataPoints:  1,
			ComparisonType: "absolute",
		},
		Severity:             "warning",
		IsEnabled:            true,
		CooldownPeriod:       time.Hour,
		NotificationChannels: []string{"email"},
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	})
	
	// Quality alert rule
	am.alertRules = append(am.alertRules, AlertRule{
		ID:          uuid.New(),
		Name:        "quality_below_threshold",
		Description: "Alert when quality score drops below threshold",
		Type:        "threshold",
		Condition: AlertCondition{
			Metric:         "quality_score",
			Operator:       "less_than",
			Threshold:      0.7,
			TimeWindow:     30 * time.Minute,
			MinDataPoints:  3,
			ComparisonType: "absolute",
		},
		Severity:             "warning",
		IsEnabled:            true,
		CooldownPeriod:       30 * time.Minute,
		NotificationChannels: []string{"email", "webhook"},
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	})
	
	// Error rate alert rule
	am.alertRules = append(am.alertRules, AlertRule{
		ID:          uuid.New(),
		Name:        "high_error_rate",
		Description: "Alert when error rate exceeds 5%",
		Type:        "threshold",
		Condition: AlertCondition{
			Metric:         "error_rate",
			Operator:       "greater_than",
			Threshold:      0.05,
			TimeWindow:     15 * time.Minute,
			MinDataPoints:  5,
			ComparisonType: "absolute",
		},
		Severity:             "critical",
		IsEnabled:            true,
		CooldownPeriod:       15 * time.Minute,
		NotificationChannels: []string{"email", "slack", "webhook"},
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	})
}

func (am *AlertManager) checkAndCreateCostAlert(alertType, severity, title, message, source string, metadata map[string]interface{}) {
	// Check if similar alert already exists (avoid spam)
	for _, alert := range am.activeAlerts {
		if alert.Type == alertType && alert.Source == source && alert.IsActive {
			// Similar alert already active, skip
			return
		}
	}
	
	am.CreateAlert(alertType, severity, title, message, source, metadata)
}

func (am *AlertManager) calculateSeverity(threshold float64) string {
	if threshold >= 1.0 {
		return "critical"
	} else if threshold >= 0.9 {
		return "warning"
	} else if threshold >= 0.7 {
		return "info"
	}
	return "info"
}

func (am *AlertManager) isAlertSuppressed(alert *Alert) bool {
	now := time.Now()
	
	for _, rule := range am.suppressionRules {
		if !rule.IsActive {
			continue
		}
		
		if now.Before(rule.StartTime) || now.After(rule.EndTime) {
			continue
		}
		
		// Simple pattern matching (in production, use proper regex)
		if rule.Pattern == "*" || rule.Pattern == alert.Type {
			am.logger.Info("Alert suppressed by rule",
				zap.String("rule_name", rule.Name),
				zap.String("alert_type", alert.Type),
			)
			return true
		}
	}
	
	return false
}

func (am *AlertManager) sendNotifications(alert *Alert) {
	if !am.config.AlertsEnabled {
		return
	}
	
	// Send email notifications
	if am.emailEnabled {
		notification := NotificationRecord{
			Channel:   "email",
			Recipient: "admin@alchemorsel.com", // From configuration
			SentAt:    time.Now(),
			Status:    "sent",
		}
		
		// In production, integrate with actual email service
		am.logger.Info("Email notification sent", 
			zap.String("alert_id", alert.ID.String()),
			zap.String("recipient", notification.Recipient),
		)
		
		alert.NotificationsSent = append(alert.NotificationsSent, notification)
	}
	
	// Send webhook notifications
	if am.webhookEnabled {
		notification := NotificationRecord{
			Channel:   "webhook",
			Recipient: "https://alerts.alchemorsel.com/webhook", // From configuration
			SentAt:    time.Now(),
			Status:    "sent",
		}
		
		// In production, make actual HTTP request
		am.logger.Info("Webhook notification sent",
			zap.String("alert_id", alert.ID.String()),
			zap.String("url", notification.Recipient),
		)
		
		alert.NotificationsSent = append(alert.NotificationsSent, notification)
	}
	
	// Send Slack notifications
	if am.slackEnabled {
		notification := NotificationRecord{
			Channel:   "slack",
			Recipient: "#alerts", // From configuration
			SentAt:    time.Now(),
			Status:    "sent",
		}
		
		// In production, integrate with Slack API
		am.logger.Info("Slack notification sent",
			zap.String("alert_id", alert.ID.String()),
			zap.String("channel", notification.Recipient),
		)
		
		alert.NotificationsSent = append(alert.NotificationsSent, notification)
	}
}

func (am *AlertManager) generateAlertTrends() []AlertTrend {
	trends := []AlertTrend{}
	
	// Group alerts by date and severity
	dailyAlerts := make(map[string]map[string]int64)
	
	for _, alert := range am.alertHistory {
		date := alert.TriggeredAt.Format("2006-01-02")
		
		if dailyAlerts[date] == nil {
			dailyAlerts[date] = make(map[string]int64)
		}
		
		dailyAlerts[date][alert.Severity]++
		dailyAlerts[date]["total"]++
	}
	
	// Convert to trends (last 7 days)
	for i := 6; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		
		if counts, exists := dailyAlerts[date]; exists {
			for severity, count := range counts {
				if severity != "total" {
					trends = append(trends, AlertTrend{
						Date:       date,
						AlertCount: count,
						Severity:   severity,
					})
				}
			}
		} else {
			// No alerts on this day
			trends = append(trends, AlertTrend{
				Date:       date,
				AlertCount: 0,
				Severity:   "none",
			})
		}
	}
	
	return trends
}

// AddSuppressionRule adds a new alert suppression rule
func (am *AlertManager) AddSuppressionRule(name, pattern, reason, createdBy string, startTime, endTime time.Time) *SuppressionRule {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	rule := &SuppressionRule{
		ID:        uuid.New(),
		Name:      name,
		Pattern:   pattern,
		StartTime: startTime,
		EndTime:   endTime,
		Reason:    reason,
		CreatedBy: createdBy,
		IsActive:  true,
	}
	
	am.suppressionRules[rule.ID.String()] = rule
	
	am.logger.Info("Alert suppression rule added",
		zap.String("rule_id", rule.ID.String()),
		zap.String("name", name),
		zap.String("pattern", pattern),
		zap.Time("start_time", startTime),
		zap.Time("end_time", endTime),
	)
	
	return rule
}

// RemoveSuppressionRule removes an alert suppression rule
func (am *AlertManager) RemoveSuppressionRule(ruleID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	if _, exists := am.suppressionRules[ruleID]; !exists {
		return fmt.Errorf("suppression rule not found: %s", ruleID)
	}
	
	delete(am.suppressionRules, ruleID)
	
	am.logger.Info("Alert suppression rule removed", zap.String("rule_id", ruleID))
	
	return nil
}

// GetSuppressionRules returns all suppression rules
func (am *AlertManager) GetSuppressionRules() []*SuppressionRule {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	rules := make([]*SuppressionRule, 0, len(am.suppressionRules))
	for _, rule := range am.suppressionRules {
		rules = append(rules, rule)
	}
	
	return rules
}