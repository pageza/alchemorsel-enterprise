package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// AlertingConfig holds alerting configuration
type AlertingConfig struct {
	SlackWebhookURL   string
	PagerDutyKey      string
	EmailSMTPHost     string
	EmailSMTPPort     int
	EmailUsername     string
	EmailPassword     string
	EmailFromAddress  string
	DefaultReceivers  []string
	EscalationPolicy  EscalationPolicy
}

// EscalationPolicy defines how alerts escalate
type EscalationPolicy struct {
	Levels []EscalationLevel
}

type EscalationLevel struct {
	Duration  time.Duration
	Receivers []string
	Actions   []string
}

// AlertManager handles alert dispatching and escalation
type AlertManager struct {
	config    AlertingConfig
	logger    *zap.Logger
	httpClient *http.Client
	activeAlerts map[string]*ActiveAlert
}

// ActiveAlert tracks the state of an active alert
type ActiveAlert struct {
	ID          string
	Name        string
	Severity    string
	StartTime   time.Time
	LastSent    time.Time
	EscalationLevel int
	Acknowledged bool
	Resolved    bool
	Metadata    map[string]interface{}
}

// AlertMessage represents an alert to be sent via alerting channels
type AlertMessage struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Severity    string                 `json:"severity"`
	Service     string                 `json:"service"`
	Environment string                 `json:"environment"`
	Timestamp   time.Time              `json:"timestamp"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Metadata    map[string]interface{} `json:"metadata"`
	RunbookURL  string                 `json:"runbook_url"`
}

// NewAlertManager creates a new alert manager
func NewAlertManager(config AlertingConfig, logger *zap.Logger) *AlertManager {
	return &AlertManager{
		config:     config,
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		activeAlerts: make(map[string]*ActiveAlert),
	}
}

// SendAlert sends an alert through configured channels
func (am *AlertManager) SendAlert(ctx context.Context, alert AlertMessage) error {
	am.logger.Info("Sending alert",
		zap.String("alert_id", alert.ID),
		zap.String("name", alert.Name),
		zap.String("severity", alert.Severity))

	// Track active alert
	activeAlert := &ActiveAlert{
		ID:              alert.ID,
		Name:            alert.Name,
		Severity:        alert.Severity,
		StartTime:       alert.Timestamp,
		LastSent:        time.Now(),
		EscalationLevel: 0,
		Metadata:        alert.Metadata,
	}
	am.activeAlerts[alert.ID] = activeAlert

	// Determine receivers based on severity and service
	receivers := am.determineReceivers(alert)

	// Send to all configured channels
	var lastErr error

	// Send to Slack
	if am.config.SlackWebhookURL != "" {
		if err := am.sendSlackAlert(ctx, alert, receivers); err != nil {
			am.logger.Error("Failed to send Slack alert", zap.Error(err))
			lastErr = err
		}
	}

	// Send to PagerDuty for critical alerts
	if alert.Severity == "critical" && am.config.PagerDutyKey != "" {
		if err := am.sendPagerDutyAlert(ctx, alert); err != nil {
			am.logger.Error("Failed to send PagerDuty alert", zap.Error(err))
			lastErr = err
		}
	}

	// Send email notifications
	if am.config.EmailSMTPHost != "" {
		if err := am.sendEmailAlert(ctx, alert, receivers); err != nil {
			am.logger.Error("Failed to send email alert", zap.Error(err))
			lastErr = err
		}
	}

	// Start escalation timer for critical alerts
	if alert.Severity == "critical" {
		go am.startEscalation(alert.ID)
	}

	return lastErr
}

// sendSlackAlert sends alert to Slack
func (am *AlertManager) sendSlackAlert(ctx context.Context, alert AlertMessage, receivers []string) error {
	color := am.getSlackColor(alert.Severity)
	
	payload := SlackPayload{
		Channel:     "#alerts",
		Username:    "Alchemorsel Alerts",
		IconEmoji:   ":warning:",
		Attachments: []SlackAttachment{
			{
				Color:      color,
				Title:      fmt.Sprintf("%s: %s", alert.Severity, alert.Name),
				Text:       alert.Description,
				Timestamp:  alert.Timestamp.Unix(),
				Fields: []SlackField{
					{Title: "Service", Value: alert.Service, Short: true},
					{Title: "Environment", Value: alert.Environment, Short: true},
					{Title: "Severity", Value: alert.Severity, Short: true},
					{Title: "Alert ID", Value: alert.ID, Short: true},
				},
				Actions: []SlackAction{
					{
						Type: "button",
						Text: "View Dashboard",
						URL:  "https://grafana.alchemorsel.com",
					},
					{
						Type: "button",
						Text: "Acknowledge",
						URL:  fmt.Sprintf("https://alerts.alchemorsel.com/ack/%s", alert.ID),
					},
				},
			},
		},
	}

	if alert.RunbookURL != "" {
		payload.Attachments[0].Actions = append(payload.Attachments[0].Actions, SlackAction{
			Type: "button",
			Text: "Runbook",
			URL:  alert.RunbookURL,
		})
	}

	return am.sendSlackWebhook(ctx, payload)
}

// sendPagerDutyAlert sends alert to PagerDuty
func (am *AlertManager) sendPagerDutyAlert(ctx context.Context, alert AlertMessage) error {
	payload := PagerDutyPayload{
		RoutingKey:  am.config.PagerDutyKey,
		EventAction: "trigger",
		DedupKey:    alert.ID,
		Payload: PagerDutyEvent{
			Summary:   fmt.Sprintf("%s: %s", alert.Name, alert.Description),
			Source:    alert.Service,
			Severity:  mapSeverityToPagerDuty(alert.Severity),
			Component: alert.Service,
			Group:     alert.Environment,
			Class:     "application",
			CustomDetails: map[string]interface{}{
				"alert_id":    alert.ID,
				"environment": alert.Environment,
				"labels":      alert.Labels,
				"annotations": alert.Annotations,
				"runbook_url": alert.RunbookURL,
			},
		},
	}

	return am.sendPagerDutyEvent(ctx, payload)
}

// sendEmailAlert sends alert via email
func (am *AlertManager) sendEmailAlert(ctx context.Context, alert AlertMessage, receivers []string) error {
	subject := fmt.Sprintf("[%s] %s: %s", alert.Environment, alert.Severity, alert.Name)
	body := am.formatEmailBody(alert)

	for _, receiver := range receivers {
		if err := am.sendEmail(ctx, receiver, subject, body); err != nil {
			am.logger.Error("Failed to send email", 
				zap.String("receiver", receiver), 
				zap.Error(err))
		}
	}

	return nil
}

// ResolveAlert marks an alert as resolved
func (am *AlertManager) ResolveAlert(ctx context.Context, alertID string) error {
	activeAlert, exists := am.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}

	activeAlert.Resolved = true
	
	// Send resolution notification
	resolutionAlert := AlertMessage{
		ID:          alertID,
		Name:        "RESOLVED: " + activeAlert.Name,
		Description: fmt.Sprintf("Alert %s has been resolved", activeAlert.Name),
		Severity:    "info",
		Timestamp:   time.Now(),
	}

	// Send resolution notification to Slack
	if am.config.SlackWebhookURL != "" {
		am.sendSlackAlert(ctx, resolutionAlert, am.config.DefaultReceivers)
	}

	// Send PagerDuty resolution
	if am.config.PagerDutyKey != "" {
		payload := PagerDutyPayload{
			RoutingKey:  am.config.PagerDutyKey,
			EventAction: "resolve",
			DedupKey:    alertID,
		}
		am.sendPagerDutyEvent(ctx, payload)
	}

	// Remove from active alerts
	delete(am.activeAlerts, alertID)

	am.logger.Info("Alert resolved", zap.String("alert_id", alertID))
	return nil
}

// AcknowledgeAlert marks an alert as acknowledged
func (am *AlertManager) AcknowledgeAlert(ctx context.Context, alertID, acknowledger string) error {
	activeAlert, exists := am.activeAlerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}

	activeAlert.Acknowledged = true
	
	// Send acknowledgment notification
	ackAlert := AlertMessage{
		ID:          alertID,
		Name:        "ACKNOWLEDGED: " + activeAlert.Name,
		Description: fmt.Sprintf("Alert %s acknowledged by %s", activeAlert.Name, acknowledger),
		Severity:    "info",
		Timestamp:   time.Now(),
	}

	// Send to Slack
	if am.config.SlackWebhookURL != "" {
		am.sendSlackAlert(ctx, ackAlert, am.config.DefaultReceivers)
	}

	// Send PagerDuty acknowledgment
	if am.config.PagerDutyKey != "" {
		payload := PagerDutyPayload{
			RoutingKey:  am.config.PagerDutyKey,
			EventAction: "acknowledge",
			DedupKey:    alertID,
		}
		am.sendPagerDutyEvent(ctx, payload)
	}

	am.logger.Info("Alert acknowledged", 
		zap.String("alert_id", alertID), 
		zap.String("acknowledger", acknowledger))
	return nil
}

// startEscalation starts the escalation process for an alert
func (am *AlertManager) startEscalation(alertID string) {
	for level, escalation := range am.config.EscalationPolicy.Levels {
		time.Sleep(escalation.Duration)
		
		activeAlert, exists := am.activeAlerts[alertID]
		if !exists || activeAlert.Acknowledged || activeAlert.Resolved {
			return // Alert was resolved or acknowledged
		}

		activeAlert.EscalationLevel = level + 1
		
		am.logger.Warn("Escalating alert",
			zap.String("alert_id", alertID),
			zap.Int("level", level+1))

		// Execute escalation actions
		for _, action := range escalation.Actions {
			am.executeEscalationAction(alertID, action)
		}
	}
}

// executeEscalationAction executes an escalation action
func (am *AlertManager) executeEscalationAction(alertID, action string) {
	switch action {
	case "page_oncall":
		// Page the on-call engineer
		am.logger.Info("Paging on-call engineer", zap.String("alert_id", alertID))
	case "notify_management":
		// Notify management
		am.logger.Info("Notifying management", zap.String("alert_id", alertID))
	case "create_incident":
		// Create formal incident
		am.logger.Info("Creating incident", zap.String("alert_id", alertID))
	}
}

// determineReceivers determines who should receive the alert
func (am *AlertManager) determineReceivers(alert AlertMessage) []string {
	// Logic to determine receivers based on alert properties
	receivers := am.config.DefaultReceivers

	// Add service-specific receivers
	switch alert.Service {
	case "database":
		receivers = append(receivers, "dba@alchemorsel.com")
	case "security":
		receivers = append(receivers, "security@alchemorsel.com")
	}

	// Add severity-based receivers
	if alert.Severity == "critical" {
		receivers = append(receivers, "oncall@alchemorsel.com")
	}

	return receivers
}

// Utility functions
func (am *AlertManager) getSlackColor(severity string) string {
	switch severity {
	case "critical":
		return "danger"
	case "warning":
		return "warning"
	case "info":
		return "good"
	default:
		return "#808080"
	}
}

func mapSeverityToPagerDuty(severity string) string {
	switch severity {
	case "critical":
		return "critical"
	case "warning":
		return "warning"
	case "info":
		return "info"
	default:
		return "info"
	}
}

func (am *AlertManager) formatEmailBody(alert AlertMessage) string {
	return fmt.Sprintf(`
Alert: %s
Severity: %s
Service: %s
Environment: %s
Description: %s

Time: %s
Alert ID: %s

Labels: %v
Annotations: %v

Dashboard: https://grafana.alchemorsel.com
Runbook: %s
`, 
		alert.Name, alert.Severity, alert.Service, alert.Environment, 
		alert.Description, alert.Timestamp.Format(time.RFC3339), 
		alert.ID, alert.Labels, alert.Annotations, alert.RunbookURL)
}

// Data structures for external services
type SlackPayload struct {
	Channel     string            `json:"channel"`
	Username    string            `json:"username"`
	IconEmoji   string            `json:"icon_emoji"`
	Attachments []SlackAttachment `json:"attachments"`
}

type SlackAttachment struct {
	Color     string        `json:"color"`
	Title     string        `json:"title"`
	Text      string        `json:"text"`
	Timestamp int64         `json:"ts"`
	Fields    []SlackField  `json:"fields"`
	Actions   []SlackAction `json:"actions"`
}

type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type SlackAction struct {
	Type string `json:"type"`
	Text string `json:"text"`
	URL  string `json:"url"`
}

type PagerDutyPayload struct {
	RoutingKey  string         `json:"routing_key"`
	EventAction string         `json:"event_action"`
	DedupKey    string         `json:"dedup_key"`
	Payload     PagerDutyEvent `json:"payload"`
}

type PagerDutyEvent struct {
	Summary       string                 `json:"summary"`
	Source        string                 `json:"source"`
	Severity      string                 `json:"severity"`
	Component     string                 `json:"component"`
	Group         string                 `json:"group"`
	Class         string                 `json:"class"`
	CustomDetails map[string]interface{} `json:"custom_details"`
}

// External service integration methods
func (am *AlertManager) sendSlackWebhook(ctx context.Context, payload SlackPayload) error {
	// Implementation would send HTTP POST to Slack webhook
	return nil
}

func (am *AlertManager) sendPagerDutyEvent(ctx context.Context, payload PagerDutyPayload) error {
	// Implementation would send HTTP POST to PagerDuty Events API
	return nil
}

func (am *AlertManager) sendEmail(ctx context.Context, to, subject, body string) error {
	// Implementation would send SMTP email
	return nil
}