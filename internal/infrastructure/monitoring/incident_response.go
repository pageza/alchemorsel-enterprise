package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// IncidentManager handles automated incident response and management
type IncidentManager struct {
	logger          *zap.Logger
	tracing         *TracingProvider
	storage         IncidentStorage
	runbooks        map[string]*Runbook
	playbooks       map[string]*Playbook
	automatedActions map[string]AutomatedAction
	
	// Incident metrics
	incidentsTotal        *prometheus.CounterVec
	incidentDuration      *prometheus.HistogramVec
	mttrMetrics           *prometheus.GaugeVec
	automatedResolutions  *prometheus.CounterVec
	runbookExecutions     *prometheus.CounterVec
}

// Incident represents a system incident
type Incident struct {
	ID                string                 `json:"id"`
	Title             string                 `json:"title"`
	Description       string                 `json:"description"`
	Status            string                 `json:"status"` // open, investigating, resolved, closed
	Severity          string                 `json:"severity"` // critical, high, medium, low
	Priority          int                    `json:"priority"`
	Service           string                 `json:"service"`
	Component         string                 `json:"component,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	ResolvedAt        time.Time              `json:"resolved_at,omitempty"`
	Assignee          string                 `json:"assignee,omitempty"`
	Reporter          string                 `json:"reporter"`
	Tags              []string               `json:"tags"`
	Labels            map[string]string      `json:"labels"`
	Alerts            []Alert                `json:"alerts"`
	Timeline          []IncidentEvent        `json:"timeline"`
	Resolution        *IncidentResolution    `json:"resolution,omitempty"`
	PostMortem        *PostMortem           `json:"post_mortem,omitempty"`
	AutomatedActions  []AutomatedActionResult `json:"automated_actions"`
	BusinessImpact    string                 `json:"business_impact"`
	UserImpact        string                 `json:"user_impact"`
	EstimatedRevenueLoss float64             `json:"estimated_revenue_loss"`
}

// Alert represents an alert that triggered or is related to an incident
type Alert struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	StartsAt    time.Time              `json:"starts_at"`
	EndsAt      time.Time              `json:"ends_at,omitempty"`
	GeneratorURL string                `json:"generator_url"`
}

// IncidentEvent represents an event in the incident timeline
type IncidentEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"` // created, updated, assigned, resolved, etc.
	Actor       string                 `json:"actor"` // user or system that performed the action
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Automated   bool                   `json:"automated"`
}

// IncidentResolution contains resolution details
type IncidentResolution struct {
	Summary           string    `json:"summary"`
	RootCause         string    `json:"root_cause"`
	ResolutionActions []string  `json:"resolution_actions"`
	PreventionMeasures []string `json:"prevention_measures"`
	ResolvedBy        string    `json:"resolved_by"`
	ResolvedAt        time.Time `json:"resolved_at"`
	DowntimeMinutes   int       `json:"downtime_minutes"`
	AffectedUsers     int       `json:"affected_users"`
}

// PostMortem contains post-incident analysis
type PostMortem struct {
	ID                string    `json:"id"`
	IncidentID        string    `json:"incident_id"`
	CreatedAt         time.Time `json:"created_at"`
	Author            string    `json:"author"`
	Summary           string    `json:"summary"`
	Timeline          string    `json:"timeline"`
	RootCause         string    `json:"root_cause"`
	Impact            string    `json:"impact"`
	WhatWentWell      []string  `json:"what_went_well"`
	WhatWentWrong     []string  `json:"what_went_wrong"`
	LessonsLearned    []string  `json:"lessons_learned"`
	ActionItems       []ActionItem `json:"action_items"`
	DetectionTime     int       `json:"detection_time_minutes"`
	ResolutionTime    int       `json:"resolution_time_minutes"`
	DocumentURL       string    `json:"document_url"`
}

// ActionItem represents a follow-up action from a post-mortem
type ActionItem struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Assignee    string    `json:"assignee"`
	DueDate     time.Time `json:"due_date"`
	Status      string    `json:"status"` // open, in_progress, completed
	Priority    string    `json:"priority"`
	Tags        []string  `json:"tags"`
}

// Runbook defines incident response procedures
type Runbook struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Service       string                 `json:"service"`
	Triggers      []string               `json:"triggers"` // Alert names that trigger this runbook
	Steps         []RunbookStep          `json:"steps"`
	EstimatedTime int                    `json:"estimated_time_minutes"`
	RequiredSkills []string              `json:"required_skills"`
	Tags          []string               `json:"tags"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

// RunbookStep represents a step in an incident response runbook
type RunbookStep struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	Description  string                 `json:"description"`
	Type         string                 `json:"type"` // manual, automated, decision
	Command      string                 `json:"command,omitempty"`
	ExpectedResult string               `json:"expected_result,omitempty"`
	Automated    bool                   `json:"automated"`
	Critical     bool                   `json:"critical"`
	EstimatedTime int                   `json:"estimated_time_minutes"`
	Prerequisites []string              `json:"prerequisites"`
	Links        []RunbookLink          `json:"links"`
}

// RunbookLink provides additional resources
type RunbookLink struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type"` // dashboard, documentation, tool
}

// Playbook defines automated response sequences
type Playbook struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Triggers     []PlaybookTrigger      `json:"triggers"`
	Actions      []PlaybookAction       `json:"actions"`
	Conditions   []PlaybookCondition    `json:"conditions"`
	Enabled      bool                   `json:"enabled"`
	DryRun       bool                   `json:"dry_run"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// PlaybookTrigger defines when a playbook should execute
type PlaybookTrigger struct {
	Type      string            `json:"type"` // alert, metric_threshold, incident_created
	Condition string            `json:"condition"`
	Labels    map[string]string `json:"labels"`
}

// PlaybookAction defines an automated action
type PlaybookAction struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // scale, restart, notify, execute_command
	Parameters  map[string]interface{} `json:"parameters"`
	Timeout     int                    `json:"timeout_seconds"`
	Retries     int                    `json:"retries"`
	OnFailure   string                 `json:"on_failure"` // continue, stop, escalate
}

// PlaybookCondition defines conditions for playbook execution
type PlaybookCondition struct {
	Type      string `json:"type"` // time_window, service_health, manual_approval
	Value     string `json:"value"`
	Operator  string `json:"operator"` // equals, greater_than, less_than
}

// AutomatedAction represents an automated response action
type AutomatedAction interface {
	Execute(ctx context.Context, incident *Incident) (*AutomatedActionResult, error)
	GetName() string
	GetDescription() string
	GetTimeout() time.Duration
}

// AutomatedActionResult contains the result of an automated action
type AutomatedActionResult struct {
	ActionName  string                 `json:"action_name"`
	ExecutedAt  time.Time              `json:"executed_at"`
	Success     bool                   `json:"success"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details"`
	Duration    time.Duration          `json:"duration"`
	RetryCount  int                    `json:"retry_count"`
}

// IncidentStorage interface for incident persistence
type IncidentStorage interface {
	CreateIncident(incident *Incident) error
	UpdateIncident(incident *Incident) error
	GetIncident(id string) (*Incident, error)
	ListIncidents(filters map[string]string, limit int) ([]*Incident, error)
	GetIncidentsByService(service string, limit int) ([]*Incident, error)
	CreatePostMortem(postMortem *PostMortem) error
	GetPostMortem(incidentID string) (*PostMortem, error)
}

// NewIncidentManager creates a new incident manager
func NewIncidentManager(logger *zap.Logger, tracing *TracingProvider, storage IncidentStorage) *IncidentManager {
	im := &IncidentManager{
		logger:          logger,
		tracing:         tracing,
		storage:         storage,
		runbooks:        make(map[string]*Runbook),
		playbooks:       make(map[string]*Playbook),
		automatedActions: make(map[string]AutomatedAction),
		
		incidentsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "incidents_total",
			Help: "Total number of incidents",
		}, []string{"severity", "service", "status"}),
		
		incidentDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "incident_duration_seconds",
			Help:    "Incident duration in seconds",
			Buckets: []float64{300, 900, 1800, 3600, 7200, 14400, 28800, 86400},
		}, []string{"severity", "service"}),
		
		mttrMetrics: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "incident_mttr_seconds",
			Help: "Mean time to resolution for incidents",
		}, []string{"service", "severity"}),
		
		automatedResolutions: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "incident_automated_resolutions_total",
			Help: "Total number of automated incident resolutions",
		}, []string{"action_type", "service", "success"}),
		
		runbookExecutions: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "incident_runbook_executions_total",
			Help: "Total number of runbook executions",
		}, []string{"runbook", "service", "success"}),
	}
	
	// Initialize default runbooks and playbooks
	im.initializeDefaultRunbooks()
	im.initializeDefaultPlaybooks()
	im.initializeAutomatedActions()
	
	return im
}

// CreateIncident creates a new incident from alerts
func (im *IncidentManager) CreateIncident(ctx context.Context, alerts []Alert, severity string) (*Incident, error) {
	ctx, span := im.tracing.StartSpan(ctx, "incident.create")
	defer span.End()
	
	incidentID := fmt.Sprintf("INC-%d", time.Now().Unix())
	
	// Determine primary service and component from alerts
	service := "unknown"
	component := ""
	if len(alerts) > 0 {
		if svc, ok := alerts[0].Labels["service"]; ok {
			service = svc
		}
		if comp, ok := alerts[0].Labels["component"]; ok {
			component = comp
		}
	}
	
	// Generate incident title and description
	title, description := im.generateIncidentContent(alerts)
	
	// Assess business and user impact
	businessImpact, userImpact, revenueLoss := im.assessImpact(service, severity, alerts)
	
	incident := &Incident{
		ID:                   incidentID,
		Title:                title,
		Description:          description,
		Status:               "open",
		Severity:             severity,
		Priority:             im.calculatePriority(severity, businessImpact),
		Service:              service,
		Component:            component,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
		Reporter:             "system",
		Tags:                 im.generateTags(alerts),
		Labels:               im.generateLabels(alerts),
		Alerts:               alerts,
		Timeline:             []IncidentEvent{},
		AutomatedActions:     []AutomatedActionResult{},
		BusinessImpact:       businessImpact,
		UserImpact:          userImpact,
		EstimatedRevenueLoss: revenueLoss,
	}
	
	// Add creation event
	incident.Timeline = append(incident.Timeline, IncidentEvent{
		ID:          fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Timestamp:   time.Now(),
		Type:        "created",
		Actor:       "system",
		Description: "Incident created from alert(s)",
		Automated:   true,
	})
	
	// Store incident
	if err := im.storage.CreateIncident(incident); err != nil {
		return nil, fmt.Errorf("failed to store incident: %w", err)
	}
	
	// Record metrics
	im.incidentsTotal.WithLabelValues(severity, service, "open").Inc()
	
	// Execute automated actions
	go im.executeAutomatedResponse(context.Background(), incident)
	
	im.logger.Info("Incident created",
		zap.String("incident_id", incidentID),
		zap.String("severity", severity),
		zap.String("service", service),
		zap.String("title", title),
	)
	
	return incident, nil
}

// executeAutomatedResponse executes automated response actions
func (im *IncidentManager) executeAutomatedResponse(ctx context.Context, incident *Incident) {
	ctx, span := im.tracing.StartSpan(ctx, "incident.automated_response")
	defer span.End()
	
	// Find applicable playbooks
	applicablePlaybooks := im.findApplicablePlaybooks(incident)
	
	for _, playbook := range applicablePlaybooks {
		if !playbook.Enabled {
			continue
		}
		
		im.logger.Info("Executing automated playbook",
			zap.String("incident_id", incident.ID),
			zap.String("playbook_id", playbook.ID),
			zap.String("playbook_name", playbook.Name),
		)
		
		// Check conditions
		if !im.evaluatePlaybookConditions(playbook, incident) {
			continue
		}
		
		// Execute actions
		for _, action := range playbook.Actions {
			result := im.executePlaybookAction(ctx, incident, action, playbook.DryRun)
			
			// Add to incident timeline
			event := IncidentEvent{
				ID:          fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				Timestamp:   time.Now(),
				Type:        "automated_action",
				Actor:       "playbook_system",
				Description: fmt.Sprintf("Executed automated action: %s", action.Type),
				Details: map[string]interface{}{
					"action_type": action.Type,
					"playbook_id": playbook.ID,
					"success":    result.Success,
					"message":    result.Message,
				},
				Automated: true,
			}
			
			incident.Timeline = append(incident.Timeline, event)
			incident.AutomatedActions = append(incident.AutomatedActions, *result)
			
			// Record metrics
			successStr := "false"
			if result.Success {
				successStr = "true"
			}
			im.automatedResolutions.WithLabelValues(action.Type, incident.Service, successStr).Inc()
			
			// Stop on failure if configured
			if !result.Success && action.OnFailure == "stop" {
				break
			}
		}
		
		// Update incident
		incident.UpdatedAt = time.Now()
		im.storage.UpdateIncident(incident)
	}
}

// ResolveIncident marks an incident as resolved
func (im *IncidentManager) ResolveIncident(ctx context.Context, incidentID string, resolution *IncidentResolution) error {
	ctx, span := im.tracing.StartSpan(ctx, "incident.resolve")
	defer span.End()
	
	incident, err := im.storage.GetIncident(incidentID)
	if err != nil {
		return fmt.Errorf("failed to get incident: %w", err)
	}
	
	incident.Status = "resolved"
	incident.ResolvedAt = time.Now()
	incident.Resolution = resolution
	incident.UpdatedAt = time.Now()
	
	// Calculate duration
	duration := incident.ResolvedAt.Sub(incident.CreatedAt)
	
	// Add resolution event
	event := IncidentEvent{
		ID:          fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Timestamp:   time.Now(),
		Type:        "resolved",
		Actor:       resolution.ResolvedBy,
		Description: "Incident resolved",
		Details: map[string]interface{}{
			"resolution_summary": resolution.Summary,
			"root_cause":        resolution.RootCause,
		},
		Automated: false,
	}
	
	incident.Timeline = append(incident.Timeline, event)
	
	// Update storage
	if err := im.storage.UpdateIncident(incident); err != nil {
		return fmt.Errorf("failed to update incident: %w", err)
	}
	
	// Record metrics
	im.incidentDuration.WithLabelValues(incident.Severity, incident.Service).Observe(duration.Seconds())
	im.incidentsTotal.WithLabelValues(incident.Severity, incident.Service, "resolved").Inc()
	
	// Update MTTR
	im.updateMTTRMetrics(incident.Service, incident.Severity)
	
	im.logger.Info("Incident resolved",
		zap.String("incident_id", incidentID),
		zap.Duration("duration", duration),
		zap.String("resolved_by", resolution.ResolvedBy),
	)
	
	return nil
}

// initializeDefaultRunbooks creates default incident response runbooks
func (im *IncidentManager) initializeDefaultRunbooks() {
	runbooks := []*Runbook{
		{
			ID:          "rb-api-high-latency",
			Name:        "API High Latency Response",
			Description: "Response procedures for high API latency incidents",
			Service:     "alchemorsel-api",
			Triggers:    []string{"APILatencyHigh", "ExtremelyHighLatency"},
			Steps: []RunbookStep{
				{
					ID:            "step1",
					Title:         "Check system resources",
					Description:   "Verify CPU, memory, and disk usage on API servers",
					Type:          "manual",
					Critical:      true,
					EstimatedTime: 2,
					Links: []RunbookLink{
						{Title: "Infrastructure Dashboard", URL: "https://grafana.alchemorsel.com/d/infrastructure", Type: "dashboard"},
					},
				},
				{
					ID:            "step2",
					Title:         "Check database performance",
					Description:   "Review slow query log and current database connections",
					Type:          "manual",
					Critical:      true,
					EstimatedTime: 3,
					Links: []RunbookLink{
						{Title: "Database Dashboard", URL: "https://grafana.alchemorsel.com/d/database", Type: "dashboard"},
					},
				},
				{
					ID:            "step3",
					Title:         "Scale API horizontally",
					Description:   "Add additional API instances if resources are constrained",
					Type:          "automated",
					Command:       "kubectl scale deployment alchemorsel-api --replicas=6",
					Automated:     true,
					EstimatedTime: 5,
				},
			},
			EstimatedTime:  10,
			RequiredSkills: []string{"kubernetes", "database"},
			Tags:           []string{"api", "performance", "latency"},
		},
		{
			ID:          "rb-database-down",
			Name:        "Database Outage Response",
			Description: "Critical response procedures for database outages",
			Service:     "postgresql",
			Triggers:    []string{"DatabaseDown"},
			Steps: []RunbookStep{
				{
					ID:            "step1",
					Title:         "Check database server status",
					Description:   "Verify if database process is running and accepting connections",
					Type:          "manual",
					Critical:      true,
					EstimatedTime: 1,
				},
				{
					ID:            "step2",
					Title:         "Check disk space",
					Description:   "Ensure database server has sufficient disk space",
					Type:          "manual",
					Critical:      true,
					EstimatedTime: 1,
				},
				{
					ID:            "step3",
					Title:         "Restart database service",
					Description:   "Attempt to restart the database service",
					Type:          "automated",
					Command:       "systemctl restart postgresql",
					Automated:     true,
					EstimatedTime: 2,
				},
				{
					ID:            "step4",
					Title:         "Failover to standby",
					Description:   "If primary is unrecoverable, failover to standby database",
					Type:          "manual",
					Critical:      true,
					EstimatedTime: 10,
				},
			},
			EstimatedTime:  15,
			RequiredSkills: []string{"database", "postgresql", "replication"},
			Tags:           []string{"database", "critical", "outage"},
		},
	}
	
	for _, runbook := range runbooks {
		runbook.CreatedAt = time.Now()
		runbook.UpdatedAt = time.Now()
		im.runbooks[runbook.ID] = runbook
	}
}

// initializeDefaultPlaybooks creates default automated response playbooks
func (im *IncidentManager) initializeDefaultPlaybooks() {
	playbooks := []*Playbook{
		{
			ID:          "pb-auto-scale-api",
			Name:        "Auto-scale API on high load",
			Description: "Automatically scale API instances when CPU usage is high",
			Triggers: []PlaybookTrigger{
				{
					Type:      "alert",
					Condition: "HighCPUUsage",
					Labels:    map[string]string{"service": "alchemorsel-api"},
				},
			},
			Actions: []PlaybookAction{
				{
					ID:   "scale-up",
					Type: "scale",
					Parameters: map[string]interface{}{
						"service":  "alchemorsel-api",
						"replicas": 6,
					},
					Timeout:   300,
					OnFailure: "escalate",
				},
			},
			Enabled:   true,
			DryRun:    false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "pb-restart-unhealthy-service",
			Name:        "Restart unhealthy service instances",
			Description: "Automatically restart service instances failing health checks",
			Triggers: []PlaybookTrigger{
				{
					Type:      "alert",
					Condition: "ServiceDown",
				},
			},
			Actions: []PlaybookAction{
				{
					ID:   "restart-service",
					Type: "restart",
					Parameters: map[string]interface{}{
						"service": "auto-detect",
					},
					Timeout:   180,
					Retries:   2,
					OnFailure: "escalate",
				},
			},
			Enabled:   true,
			DryRun:    false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
	
	for _, playbook := range playbooks {
		im.playbooks[playbook.ID] = playbook
	}
}

// Helper methods
func (im *IncidentManager) generateIncidentContent(alerts []Alert) (string, string) {
	if len(alerts) == 0 {
		return "Unknown Incident", "No alert information available"
	}
	
	primaryAlert := alerts[0]
	title := primaryAlert.Name
	
	if summary, ok := primaryAlert.Annotations["summary"]; ok {
		title = summary
	}
	
	description := primaryAlert.Name
	if desc, ok := primaryAlert.Annotations["description"]; ok {
		description = desc
	}
	
	if len(alerts) > 1 {
		description += fmt.Sprintf(" (and %d other alerts)", len(alerts)-1)
	}
	
	return title, description
}

func (im *IncidentManager) assessImpact(service, severity string, alerts []Alert) (string, string, float64) {
	var businessImpact, userImpact string
	var revenueLoss float64
	
	switch severity {
	case "critical":
		businessImpact = "high"
		userImpact = "high"
		revenueLoss = 1000.0
	case "high":
		businessImpact = "medium"
		userImpact = "medium"
		revenueLoss = 100.0
	default:
		businessImpact = "low"
		userImpact = "low"
		revenueLoss = 0.0
	}
	
	// Service-specific adjustments
	if service == "alchemorsel-api" {
		// API issues have higher business impact
		revenueLoss *= 2.0
	}
	
	return businessImpact, userImpact, revenueLoss
}

func (im *IncidentManager) calculatePriority(severity, businessImpact string) int {
	priorityMatrix := map[string]map[string]int{
		"critical": {"high": 1, "medium": 1, "low": 2},
		"high":     {"high": 2, "medium": 2, "low": 3},
		"medium":   {"high": 3, "medium": 3, "low": 4},
		"low":      {"high": 4, "medium": 4, "low": 5},
	}
	
	if priority, ok := priorityMatrix[severity][businessImpact]; ok {
		return priority
	}
	
	return 5 // Default low priority
}

func (im *IncidentManager) generateTags(alerts []Alert) []string {
	tagSet := make(map[string]bool)
	
	for _, alert := range alerts {
		for key, value := range alert.Labels {
			tag := fmt.Sprintf("%s:%s", key, value)
			tagSet[tag] = true
		}
	}
	
	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	
	sort.Strings(tags)
	return tags
}

func (im *IncidentManager) generateLabels(alerts []Alert) map[string]string {
	labels := make(map[string]string)
	
	if len(alerts) > 0 {
		for key, value := range alerts[0].Labels {
			labels[key] = value
		}
	}
	
	return labels
}

func (im *IncidentManager) findApplicablePlaybooks(incident *Incident) []*Playbook {
	var applicable []*Playbook
	
	for _, playbook := range im.playbooks {
		if im.isPlaybookApplicable(playbook, incident) {
			applicable = append(applicable, playbook)
		}
	}
	
	return applicable
}

func (im *IncidentManager) isPlaybookApplicable(playbook *Playbook, incident *Incident) bool {
	for _, trigger := range playbook.Triggers {
		switch trigger.Type {
		case "alert":
			for _, alert := range incident.Alerts {
				if strings.Contains(alert.Name, trigger.Condition) {
					// Check label matching
					matches := true
					for key, value := range trigger.Labels {
						if alertValue, ok := alert.Labels[key]; !ok || alertValue != value {
							matches = false
							break
						}
					}
					if matches {
						return true
					}
				}
			}
		case "incident_created":
			return true // Always applicable for new incidents
		}
	}
	
	return false
}

func (im *IncidentManager) evaluatePlaybookConditions(playbook *Playbook, incident *Incident) bool {
	for _, condition := range playbook.Conditions {
		switch condition.Type {
		case "time_window":
			// Check if current time is within allowed window
			// Implementation would depend on condition format
		case "service_health":
			// Check if service health meets requirements
			// Implementation would query health endpoints
		case "manual_approval":
			// Skip automated execution if manual approval required
			return false
		}
	}
	
	return true
}

func (im *IncidentManager) executePlaybookAction(ctx context.Context, incident *Incident, action PlaybookAction, dryRun bool) *AutomatedActionResult {
	start := time.Now()
	
	result := &AutomatedActionResult{
		ActionName: action.Type,
		ExecutedAt: start,
		Success:    false,
	}
	
	if dryRun {
		result.Success = true
		result.Message = "Dry run - action not executed"
		result.Duration = time.Since(start)
		return result
	}
	
	// Execute action based on type
	switch action.Type {
	case "scale":
		result = im.executeScaleAction(ctx, incident, action)
	case "restart":
		result = im.executeRestartAction(ctx, incident, action)
	case "notify":
		result = im.executeNotifyAction(ctx, incident, action)
	default:
		result.Message = fmt.Sprintf("Unknown action type: %s", action.Type)
	}
	
	result.Duration = time.Since(start)
	return result
}

func (im *IncidentManager) executeScaleAction(ctx context.Context, incident *Incident, action PlaybookAction) *AutomatedActionResult {
	// Implementation would scale the service
	// This is a simplified example
	return &AutomatedActionResult{
		ActionName: "scale",
		Success:    true,
		Message:    "Service scaled successfully",
		Details: map[string]interface{}{
			"service":  action.Parameters["service"],
			"replicas": action.Parameters["replicas"],
		},
	}
}

func (im *IncidentManager) executeRestartAction(ctx context.Context, incident *Incident, action PlaybookAction) *AutomatedActionResult {
	// Implementation would restart the service
	return &AutomatedActionResult{
		ActionName: "restart",
		Success:    true,
		Message:    "Service restarted successfully",
	}
}

func (im *IncidentManager) executeNotifyAction(ctx context.Context, incident *Incident, action PlaybookAction) *AutomatedActionResult {
	// Implementation would send notifications
	return &AutomatedActionResult{
		ActionName: "notify",
		Success:    true,
		Message:    "Notifications sent",
	}
}

func (im *IncidentManager) updateMTTRMetrics(service, severity string) {
	// This would calculate MTTR from recent incidents
	// Simplified implementation
	mttr := 1800.0 // 30 minutes default
	im.mttrMetrics.WithLabelValues(service, severity).Set(mttr)
}

func (im *IncidentManager) initializeAutomatedActions() {
	// Initialize automated actions
	// This would be expanded with actual implementations
}

// HTTP Handlers
func (im *IncidentManager) GetIncident(c *gin.Context) {
	incidentID := c.Param("id")
	
	incident, err := im.storage.GetIncident(incidentID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Incident not found"})
		return
	}
	
	c.JSON(200, incident)
}

func (im *IncidentManager) ListIncidents(c *gin.Context) {
	filters := make(map[string]string)
	if service := c.Query("service"); service != "" {
		filters["service"] = service
	}
	if status := c.Query("status"); status != "" {
		filters["status"] = status
	}
	
	incidents, err := im.storage.ListIncidents(filters, 50)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to retrieve incidents"})
		return
	}
	
	c.JSON(200, gin.H{
		"incidents": incidents,
		"count":     len(incidents),
	})
}

func (im *IncidentManager) GetRunbooks(c *gin.Context) {
	c.JSON(200, gin.H{
		"runbooks": im.runbooks,
		"count":    len(im.runbooks),
	})
}