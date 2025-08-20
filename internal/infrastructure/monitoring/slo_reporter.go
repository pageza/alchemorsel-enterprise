package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// SLOReporter handles SLA/SLO tracking and automated reporting
type SLOReporter struct {
	logger  *zap.Logger
	tracing *TracingProvider
	storage SLOStorage
	
	// SLO compliance metrics
	sloCompliance         *prometheus.GaugeVec
	errorBudgetRemaining  *prometheus.GaugeVec
	errorBudgetBurnRate   *prometheus.GaugeVec
	sloViolations         *prometheus.CounterVec
	mttrMetrics           *prometheus.HistogramVec
	mtbfMetrics           *prometheus.HistogramVec
	
	// SLO definitions
	sloDefinitions map[string]*SLODefinition
}

// SLODefinition defines a Service Level Objective
type SLODefinition struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Service         string                 `json:"service"`
	Type            string                 `json:"type"` // availability, latency, error_rate, custom
	Target          float64                `json:"target"`
	Window          string                 `json:"window"` // 1h, 24h, 7d, 30d
	ErrorBudget     float64                `json:"error_budget"`
	AlertThresholds map[string]float64     `json:"alert_thresholds"`
	Query           string                 `json:"query"`
	Labels          map[string]string      `json:"labels"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// SLOReport represents an SLO compliance report
type SLOReport struct {
	SLOName              string                 `json:"slo_name"`
	Service              string                 `json:"service"`
	ReportPeriod         string                 `json:"report_period"`
	GeneratedAt          time.Time              `json:"generated_at"`
	Compliance           SLOCompliance          `json:"compliance"`
	ErrorBudget          ErrorBudget            `json:"error_budget"`
	Incidents            []SLOIncident          `json:"incidents"`
	Recommendations      []string               `json:"recommendations"`
	TrendAnalysis        TrendAnalysis          `json:"trend_analysis"`
	BusinessImpact       BusinessImpact         `json:"business_impact"`
}

// SLOCompliance tracks compliance metrics
type SLOCompliance struct {
	CurrentValue    float64   `json:"current_value"`
	Target          float64   `json:"target"`
	ComplianceRate  float64   `json:"compliance_rate"`
	Status          string    `json:"status"` // healthy, at_risk, breached
	LastBreachTime  time.Time `json:"last_breach_time,omitempty"`
	BreachDuration  int64     `json:"breach_duration_seconds"`
}

// ErrorBudget tracks error budget consumption
type ErrorBudget struct {
	Total           float64 `json:"total"`
	Consumed        float64 `json:"consumed"`
	Remaining       float64 `json:"remaining"`
	RemainingPercent float64 `json:"remaining_percent"`
	BurnRate        float64 `json:"burn_rate"`
	EstimatedDepleted time.Time `json:"estimated_depleted,omitempty"`
}

// SLOIncident represents an SLO violation incident
type SLOIncident struct {
	ID              string                 `json:"id"`
	StartTime       time.Time              `json:"start_time"`
	EndTime         time.Time              `json:"end_time,omitempty"`
	Duration        int64                  `json:"duration_seconds"`
	Severity        string                 `json:"severity"`
	ErrorBudgetConsumed float64           `json:"error_budget_consumed"`
	RootCause       string                 `json:"root_cause,omitempty"`
	Resolution      string                 `json:"resolution,omitempty"`
	Labels          map[string]string      `json:"labels"`
}

// TrendAnalysis provides trend analysis for SLOs
type TrendAnalysis struct {
	Direction       string  `json:"direction"` // improving, degrading, stable
	ChangeRate      float64 `json:"change_rate"`
	Seasonality     string  `json:"seasonality"`
	Forecast        string  `json:"forecast"`
	ConfidenceLevel float64 `json:"confidence_level"`
}

// BusinessImpact calculates business impact of SLO performance
type BusinessImpact struct {
	UserImpact           string  `json:"user_impact"`
	RevenueImpact        float64 `json:"revenue_impact_usd"`
	ReputationScore      float64 `json:"reputation_score"`
	CustomerSatisfaction float64 `json:"customer_satisfaction"`
	CompetitivePosition  string  `json:"competitive_position"`
}

// SLOStorage interface for SLO data persistence
type SLOStorage interface {
	StoreSLOMetric(sloName string, timestamp time.Time, value float64, labels map[string]string) error
	GetSLOHistory(sloName string, window time.Duration) ([]SLODataPoint, error)
	StoreIncident(incident SLOIncident) error
	GetIncidents(sloName string, window time.Duration) ([]SLOIncident, error)
	GetSLODefinition(sloName string) (*SLODefinition, error)
	StoreSLODefinition(definition *SLODefinition) error
}

// SLODataPoint represents a single SLO measurement
type SLODataPoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels"`
}

// NewSLOReporter creates a new SLO reporter
func NewSLOReporter(logger *zap.Logger, tracing *TracingProvider, storage SLOStorage) *SLOReporter {
	reporter := &SLOReporter{
		logger:  logger,
		tracing: tracing,
		storage: storage,
		
		sloCompliance: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "slo_compliance_percentage",
			Help: "SLO compliance percentage",
		}, []string{"slo_name", "service", "window"}),
		
		errorBudgetRemaining: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "slo_error_budget_remaining_percentage",
			Help: "Remaining error budget percentage",
		}, []string{"slo_name", "service", "window"}),
		
		errorBudgetBurnRate: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "slo_error_budget_burn_rate",
			Help: "Error budget burn rate",
		}, []string{"slo_name", "service", "window"}),
		
		sloViolations: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "slo_violations_total",
			Help: "Total number of SLO violations",
		}, []string{"slo_name", "service", "severity"}),
		
		mttrMetrics: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "slo_mean_time_to_recovery_seconds",
			Help:    "Mean time to recovery from SLO violations",
			Buckets: []float64{60, 300, 900, 1800, 3600, 7200, 14400, 28800},
		}, []string{"slo_name", "service"}),
		
		mtbfMetrics: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "slo_mean_time_between_failures_seconds",
			Help:    "Mean time between SLO failures",
			Buckets: []float64{3600, 21600, 86400, 604800, 2629746}, // 1h to 1 month
		}, []string{"slo_name", "service"}),
		
		sloDefinitions: make(map[string]*SLODefinition),
	}
	
	// Initialize default SLOs
	reporter.initializeDefaultSLOs()
	
	return reporter
}

// initializeDefaultSLOs sets up default SLO definitions
func (s *SLOReporter) initializeDefaultSLOs() {
	defaultSLOs := []*SLODefinition{
		{
			Name:        "api_availability",
			Description: "API availability SLO - 99.9% uptime",
			Service:     "alchemorsel-api",
			Type:        "availability",
			Target:      99.9,
			Window:      "30d",
			ErrorBudget: 0.1,
			AlertThresholds: map[string]float64{
				"warning":  99.5,
				"critical": 99.0,
			},
			Query:  `slo:api_availability_5m`,
			Labels: map[string]string{"tier": "critical"},
		},
		{
			Name:        "api_latency_p95",
			Description: "API P95 latency SLO - 95% of requests under 500ms",
			Service:     "alchemorsel-api",
			Type:        "latency",
			Target:      500.0,
			Window:      "30d",
			ErrorBudget: 5.0,
			AlertThresholds: map[string]float64{
				"warning":  600,
				"critical": 1000,
			},
			Query:  `slo:api_latency_p95`,
			Labels: map[string]string{"tier": "critical"},
		},
		{
			Name:        "api_error_rate",
			Description: "API error rate SLO - less than 0.1% error rate",
			Service:     "alchemorsel-api",
			Type:        "error_rate",
			Target:      0.1,
			Window:      "30d",
			ErrorBudget: 0.1,
			AlertThresholds: map[string]float64{
				"warning":  0.5,
				"critical": 1.0,
			},
			Query:  `slo:api_error_rate`,
			Labels: map[string]string{"tier": "critical"},
		},
		{
			Name:        "ai_response_time",
			Description: "AI service response time SLO - 95% under 2 seconds",
			Service:     "ollama",
			Type:        "latency",
			Target:      2000.0,
			Window:      "30d",
			ErrorBudget: 5.0,
			AlertThresholds: map[string]float64{
				"warning":  3000,
				"critical": 5000,
			},
			Query:  `slo:ai_latency_p95`,
			Labels: map[string]string{"tier": "important"},
		},
		{
			Name:        "database_query_latency",
			Description: "Database query latency SLO - 95% under 100ms",
			Service:     "postgresql",
			Type:        "latency",
			Target:      100.0,
			Window:      "30d",
			ErrorBudget: 5.0,
			AlertThresholds: map[string]float64{
				"warning":  200,
				"critical": 500,
			},
			Query:  `slo:db_query_latency_p95`,
			Labels: map[string]string{"tier": "critical"},
		},
		{
			Name:        "cache_hit_ratio",
			Description: "Cache hit ratio SLO - 90% hit rate",
			Service:     "redis",
			Type:        "custom",
			Target:      90.0,
			Window:      "30d",
			ErrorBudget: 10.0,
			AlertThresholds: map[string]float64{
				"warning":  85,
				"critical": 80,
			},
			Query:  `slo:cache_hit_ratio`,
			Labels: map[string]string{"tier": "important"},
		},
	}
	
	for _, slo := range defaultSLOs {
		slo.CreatedAt = time.Now()
		slo.UpdatedAt = time.Now()
		s.sloDefinitions[slo.Name] = slo
		
		// Store in persistent storage
		if err := s.storage.StoreSLODefinition(slo); err != nil {
			s.logger.Error("Failed to store SLO definition", 
				zap.String("slo_name", slo.Name),
				zap.Error(err),
			)
		}
	}
}

// GenerateReport generates a comprehensive SLO report
func (s *SLOReporter) GenerateReport(ctx context.Context, sloName, window string) (*SLOReport, error) {
	ctx, span := s.tracing.StartSpan(ctx, "slo.generate_report")
	defer span.End()
	
	definition, exists := s.sloDefinitions[sloName]
	if !exists {
		return nil, fmt.Errorf("SLO definition not found: %s", sloName)
	}
	
	windowDuration, err := time.ParseDuration(window)
	if err != nil {
		return nil, fmt.Errorf("invalid window duration: %w", err)
	}
	
	// Get SLO history data
	history, err := s.storage.GetSLOHistory(sloName, windowDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to get SLO history: %w", err)
	}
	
	// Get incidents
	incidents, err := s.storage.GetIncidents(sloName, windowDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to get incidents: %w", err)
	}
	
	// Calculate compliance metrics
	compliance := s.calculateCompliance(definition, history)
	errorBudget := s.calculateErrorBudget(definition, history, windowDuration)
	trendAnalysis := s.analyzeTrend(history)
	businessImpact := s.calculateBusinessImpact(definition, compliance, incidents)
	recommendations := s.generateRecommendations(definition, compliance, errorBudget, incidents)
	
	report := &SLOReport{
		SLOName:         sloName,
		Service:         definition.Service,
		ReportPeriod:    window,
		GeneratedAt:     time.Now(),
		Compliance:      compliance,
		ErrorBudget:     errorBudget,
		Incidents:       incidents,
		Recommendations: recommendations,
		TrendAnalysis:   trendAnalysis,
		BusinessImpact:  businessImpact,
	}
	
	s.logger.Info("Generated SLO report",
		zap.String("slo_name", sloName),
		zap.String("window", window),
		zap.Float64("compliance_rate", compliance.ComplianceRate),
		zap.Float64("error_budget_remaining", errorBudget.RemainingPercent),
	)
	
	return report, nil
}

// calculateCompliance calculates SLO compliance metrics
func (s *SLOReporter) calculateCompliance(definition *SLODefinition, history []SLODataPoint) SLOCompliance {
	if len(history) == 0 {
		return SLOCompliance{
			Status: "unknown",
		}
	}
	
	// Get current value (latest data point)
	currentValue := history[len(history)-1].Value
	
	// Calculate compliance rate based on SLO type
	var complianceRate float64
	var status string
	var lastBreachTime time.Time
	var breachDuration int64
	
	switch definition.Type {
	case "availability":
		complianceRate = currentValue
	case "latency":
		// For latency, compliance is when value is below target
		if currentValue <= definition.Target {
			complianceRate = 100.0
		} else {
			complianceRate = (definition.Target / currentValue) * 100.0
		}
	case "error_rate":
		// For error rate, compliance is when value is below target
		if currentValue <= definition.Target {
			complianceRate = 100.0
		} else {
			complianceRate = (definition.Target / currentValue) * 100.0
		}
	case "custom":
		// Custom SLOs use direct value comparison
		complianceRate = (currentValue / definition.Target) * 100.0
	}
	
	// Determine status
	if complianceRate >= definition.Target {
		status = "healthy"
	} else if complianceRate >= definition.AlertThresholds["warning"] {
		status = "at_risk"
	} else {
		status = "breached"
	}
	
	// Find last breach
	for i := len(history) - 1; i >= 0; i-- {
		point := history[i]
		if s.isBreach(definition, point.Value) {
			if lastBreachTime.IsZero() {
				lastBreachTime = point.Timestamp
			}
			breachDuration += 60 // Assuming 1-minute intervals
		} else if !lastBreachTime.IsZero() {
			break
		}
	}
	
	return SLOCompliance{
		CurrentValue:    currentValue,
		Target:          definition.Target,
		ComplianceRate:  complianceRate,
		Status:          status,
		LastBreachTime:  lastBreachTime,
		BreachDuration:  breachDuration,
	}
}

// calculateErrorBudget calculates error budget consumption
func (s *SLOReporter) calculateErrorBudget(definition *SLODefinition, history []SLODataPoint, window time.Duration) ErrorBudget {
	totalBudget := definition.ErrorBudget
	consumed := 0.0
	
	// Calculate consumption based on breaches
	for _, point := range history {
		if s.isBreach(definition, point.Value) {
			// Each breach consumes error budget proportionally
			consumed += (1.0 / float64(len(history))) * totalBudget
		}
	}
	
	remaining := totalBudget - consumed
	remainingPercent := (remaining / totalBudget) * 100.0
	
	// Calculate burn rate (consumption per hour)
	windowHours := window.Hours()
	burnRate := consumed / windowHours
	
	var estimatedDepleted time.Time
	if burnRate > 0 {
		hoursUntilDepletion := remaining / burnRate
		estimatedDepleted = time.Now().Add(time.Duration(hoursUntilDepletion) * time.Hour)
	}
	
	return ErrorBudget{
		Total:            totalBudget,
		Consumed:         consumed,
		Remaining:        remaining,
		RemainingPercent: remainingPercent,
		BurnRate:         burnRate,
		EstimatedDepleted: estimatedDepleted,
	}
}

// analyzeTrend analyzes SLO performance trends
func (s *SLOReporter) analyzeTrend(history []SLODataPoint) TrendAnalysis {
	if len(history) < 2 {
		return TrendAnalysis{
			Direction:       "unknown",
			ConfidenceLevel: 0.0,
		}
	}
	
	// Simple trend analysis using linear regression
	n := len(history)
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	
	for i, point := range history {
		x := float64(i)
		y := point.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	// Calculate slope
	slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
	
	var direction string
	changeRate := math.Abs(slope)
	
	if slope > 0.1 {
		direction = "improving"
	} else if slope < -0.1 {
		direction = "degrading"
	} else {
		direction = "stable"
	}
	
	// Calculate R-squared for confidence
	meanY := sumY / float64(n)
	ssRes, ssTot := 0.0, 0.0
	
	for i, point := range history {
		predicted := slope*float64(i) + (sumY-slope*sumX)/float64(n)
		ssRes += math.Pow(point.Value-predicted, 2)
		ssTot += math.Pow(point.Value-meanY, 2)
	}
	
	rSquared := 1.0 - (ssRes/ssTot)
	
	return TrendAnalysis{
		Direction:       direction,
		ChangeRate:      changeRate,
		Seasonality:     "none", // Would require more sophisticated analysis
		Forecast:        "stable", // Would require more sophisticated analysis
		ConfidenceLevel: rSquared,
	}
}

// calculateBusinessImpact calculates business impact of SLO performance
func (s *SLOReporter) calculateBusinessImpact(definition *SLODefinition, compliance SLOCompliance, incidents []SLOIncident) BusinessImpact {
	// Simplified business impact calculation
	var userImpact string
	var revenueImpact float64
	reputationScore := 1.0
	customerSatisfaction := 0.95
	
	if compliance.Status == "breached" {
		userImpact = "high"
		revenueImpact = 1000.0 // Example calculation
		reputationScore = 0.7
		customerSatisfaction = 0.8
	} else if compliance.Status == "at_risk" {
		userImpact = "medium"
		revenueImpact = 100.0
		reputationScore = 0.85
		customerSatisfaction = 0.9
	} else {
		userImpact = "low"
		revenueImpact = 0.0
	}
	
	// Calculate total revenue impact from incidents
	for _, incident := range incidents {
		if incident.Duration > 3600 { // More than 1 hour
			revenueImpact += float64(incident.Duration) * 0.1 // $0.1 per second
		}
	}
	
	return BusinessImpact{
		UserImpact:           userImpact,
		RevenueImpact:        revenueImpact,
		ReputationScore:      reputationScore,
		CustomerSatisfaction: customerSatisfaction,
		CompetitivePosition:  "strong", // Would require market analysis
	}
}

// generateRecommendations generates actionable recommendations
func (s *SLOReporter) generateRecommendations(definition *SLODefinition, compliance SLOCompliance, errorBudget ErrorBudget, incidents []SLOIncident) []string {
	var recommendations []string
	
	if compliance.Status == "breached" {
		recommendations = append(recommendations, 
			"URGENT: SLO is currently breached. Investigate and resolve immediately.")
	}
	
	if errorBudget.RemainingPercent < 20 {
		recommendations = append(recommendations,
			"Error budget is running low. Consider reducing deployment velocity.")
	}
	
	if errorBudget.BurnRate > 1.0 {
		recommendations = append(recommendations,
			"High error budget burn rate detected. Review recent changes and system health.")
	}
	
	if len(incidents) > 5 {
		recommendations = append(recommendations,
			"High incident frequency. Invest in reliability improvements and automation.")
	}
	
	// Service-specific recommendations
	switch definition.Service {
	case "alchemorsel-api":
		if definition.Type == "latency" && compliance.CurrentValue > definition.Target {
			recommendations = append(recommendations,
				"API latency is high. Consider optimizing database queries, adding caching, or scaling horizontally.")
		}
	case "postgresql":
		if definition.Type == "latency" && compliance.CurrentValue > definition.Target {
			recommendations = append(recommendations,
				"Database queries are slow. Review slow query logs, add indexes, or optimize queries.")
		}
	case "redis":
		if definition.Type == "custom" && compliance.CurrentValue < definition.Target {
			recommendations = append(recommendations,
				"Cache hit rate is low. Review cache strategy, increase cache size, or optimize cache keys.")
		}
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"SLO performance is healthy. Continue current practices and monitor for any degradation.")
	}
	
	return recommendations
}

// isBreach checks if a value represents an SLO breach
func (s *SLOReporter) isBreach(definition *SLODefinition, value float64) bool {
	switch definition.Type {
	case "availability":
		return value < definition.Target
	case "latency", "error_rate":
		return value > definition.Target
	case "custom":
		return value < definition.Target
	default:
		return false
	}
}

// UpdateSLOMetrics updates SLO compliance metrics
func (s *SLOReporter) UpdateSLOMetrics(ctx context.Context) error {
	ctx, span := s.tracing.StartSpan(ctx, "slo.update_metrics")
	defer span.End()
	
	for sloName, definition := range s.sloDefinitions {
		windowDuration, err := time.ParseDuration(definition.Window)
		if err != nil {
			continue
		}
		
		history, err := s.storage.GetSLOHistory(sloName, windowDuration)
		if err != nil {
			s.logger.Error("Failed to get SLO history for metrics update",
				zap.String("slo_name", sloName),
				zap.Error(err),
			)
			continue
		}
		
		if len(history) == 0 {
			continue
		}
		
		compliance := s.calculateCompliance(definition, history)
		errorBudget := s.calculateErrorBudget(definition, history, windowDuration)
		
		// Update Prometheus metrics
		s.sloCompliance.WithLabelValues(sloName, definition.Service, definition.Window).Set(compliance.ComplianceRate)
		s.errorBudgetRemaining.WithLabelValues(sloName, definition.Service, definition.Window).Set(errorBudget.RemainingPercent)
		s.errorBudgetBurnRate.WithLabelValues(sloName, definition.Service, definition.Window).Set(errorBudget.BurnRate)
		
		// Record violations if breached
		if compliance.Status == "breached" {
			s.sloViolations.WithLabelValues(sloName, definition.Service, "critical").Inc()
		}
	}
	
	return nil
}

// HTTP Handlers

// GetSLOReport generates and returns an SLO report
func (s *SLOReporter) GetSLOReport(c *gin.Context) {
	sloName := c.Param("sloName")
	window := c.DefaultQuery("window", "24h")
	
	report, err := s.GenerateReport(c.Request.Context(), sloName, window)
	if err != nil {
		s.logger.Error("Failed to generate SLO report",
			zap.String("slo_name", sloName),
			zap.String("window", window),
			zap.Error(err),
		)
		c.JSON(500, gin.H{"error": "Failed to generate report"})
		return
	}
	
	c.JSON(200, report)
}

// GetAllSLOs returns all SLO definitions
func (s *SLOReporter) GetAllSLOs(c *gin.Context) {
	c.JSON(200, gin.H{
		"slos": s.sloDefinitions,
		"count": len(s.sloDefinitions),
	})
}

// GetSLODashboard returns dashboard data for SLOs
func (s *SLOReporter) GetSLODashboard(c *gin.Context) {
	dashboard := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"slos": make(map[string]interface{}),
	}
	
	for sloName, definition := range s.sloDefinitions {
		windowDuration, _ := time.ParseDuration(definition.Window)
		history, _ := s.storage.GetSLOHistory(sloName, windowDuration)
		
		if len(history) > 0 {
			compliance := s.calculateCompliance(definition, history)
			errorBudget := s.calculateErrorBudget(definition, history, windowDuration)
			
			dashboard["slos"].(map[string]interface{})[sloName] = map[string]interface{}{
				"name": definition.Name,
				"service": definition.Service,
				"compliance_rate": compliance.ComplianceRate,
				"status": compliance.Status,
				"error_budget_remaining": errorBudget.RemainingPercent,
				"current_value": compliance.CurrentValue,
				"target": definition.Target,
			}
		}
	}
	
	c.JSON(200, dashboard)
}