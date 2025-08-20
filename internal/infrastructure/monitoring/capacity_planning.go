package monitoring

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// CapacityPlanner handles capacity planning and performance regression detection
type CapacityPlanner struct {
	logger   *zap.Logger
	tracing  *TracingProvider
	storage  CapacityStorage
	analyzer *PerformanceAnalyzer
	
	// Capacity metrics
	capacityUtilization    *prometheus.GaugeVec
	capacityForecast       *prometheus.GaugeVec
	capacityRecommendations *prometheus.GaugeVec
	performanceBaselines   *prometheus.GaugeVec
	regressionAlerts       *prometheus.CounterVec
	costOptimization       *prometheus.GaugeVec
}

// CapacityReport represents a capacity planning report
type CapacityReport struct {
	ServiceName           string                   `json:"service_name"`
	ReportPeriod          string                   `json:"report_period"`
	GeneratedAt           time.Time                `json:"generated_at"`
	CurrentUtilization    ResourceUtilization      `json:"current_utilization"`
	ForecastedUtilization []ForecastPoint          `json:"forecasted_utilization"`
	Recommendations       []CapacityRecommendation `json:"recommendations"`
	CostAnalysis          CostAnalysis             `json:"cost_analysis"`
	PerformanceBaselines  PerformanceBaselines     `json:"performance_baselines"`
	RegressionAnalysis    RegressionAnalysis       `json:"regression_analysis"`
	ScalingEvents         []ScalingEvent           `json:"scaling_events"`
	RiskAssessment        RiskAssessment           `json:"risk_assessment"`
}

// ResourceUtilization tracks current resource usage
type ResourceUtilization struct {
	CPU           ResourceMetric `json:"cpu"`
	Memory        ResourceMetric `json:"memory"`
	Storage       ResourceMetric `json:"storage"`
	Network       ResourceMetric `json:"network"`
	DatabaseConns ResourceMetric `json:"database_connections"`
	QueueDepth    ResourceMetric `json:"queue_depth"`
	Timestamp     time.Time      `json:"timestamp"`
}

// ResourceMetric represents a resource metric
type ResourceMetric struct {
	Current     float64 `json:"current"`
	Average     float64 `json:"average"`
	Peak        float64 `json:"peak"`
	Capacity    float64 `json:"capacity"`
	Utilization float64 `json:"utilization_percent"`
	Trend       string  `json:"trend"` // increasing, decreasing, stable
}

// ForecastPoint represents a forecasted capacity point
type ForecastPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	PredictedUsage     float64   `json:"predicted_usage"`
	ConfidenceInterval float64   `json:"confidence_interval"`
	ResourceType       string    `json:"resource_type"`
}

// CapacityRecommendation suggests capacity changes
type CapacityRecommendation struct {
	Type            string                 `json:"type"` // scale_up, scale_down, optimize, migrate
	Priority        string                 `json:"priority"` // high, medium, low
	Resource        string                 `json:"resource"`
	CurrentValue    float64                `json:"current_value"`
	RecommendedValue float64               `json:"recommended_value"`
	Justification   string                 `json:"justification"`
	EstimatedCost   float64                `json:"estimated_cost_change_monthly_usd"`
	Timeline        string                 `json:"timeline"`
	Confidence      float64                `json:"confidence"`
	Prerequisites   []string               `json:"prerequisites"`
	RiskLevel       string                 `json:"risk_level"`
	Details         map[string]interface{} `json:"details"`
}

// CostAnalysis provides cost optimization insights
type CostAnalysis struct {
	CurrentMonthlyCost   float64              `json:"current_monthly_cost_usd"`
	ForecastedCost       []CostForecastPoint  `json:"forecasted_cost"`
	PotentialSavings     float64              `json:"potential_savings_usd"`
	CostOptimizations    []CostOptimization   `json:"cost_optimizations"`
	ROIAnalysis          ROIAnalysis          `json:"roi_analysis"`
	BudgetUtilization    float64              `json:"budget_utilization_percent"`
	CostPerTransaction   float64              `json:"cost_per_transaction_usd"`
	CostEfficiencyTrend  string               `json:"cost_efficiency_trend"`
}

// CostForecastPoint represents a cost forecast
type CostForecastPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	PredictedCost float64   `json:"predicted_cost_usd"`
	Confidence    float64   `json:"confidence"`
}

// CostOptimization suggests cost-saving measures
type CostOptimization struct {
	Type           string  `json:"type"`
	Description    string  `json:"description"`
	PotentialSaving float64 `json:"potential_saving_usd"`
	ImplementationEffort string `json:"implementation_effort"`
	RiskLevel      string  `json:"risk_level"`
}

// ROIAnalysis calculates return on investment
type ROIAnalysis struct {
	InvestmentRequired float64 `json:"investment_required_usd"`
	AnnualSavings     float64 `json:"annual_savings_usd"`
	PaybackPeriod     int     `json:"payback_period_months"`
	ROIPercentage     float64 `json:"roi_percentage"`
}

// PerformanceBaselines tracks performance baselines
type PerformanceBaselines struct {
	ResponseTime    BaselineMetric `json:"response_time_ms"`
	Throughput      BaselineMetric `json:"throughput_rps"`
	ErrorRate       BaselineMetric `json:"error_rate_percent"`
	DatabaseLatency BaselineMetric `json:"database_latency_ms"`
	CacheHitRate    BaselineMetric `json:"cache_hit_rate_percent"`
	LastUpdated     time.Time      `json:"last_updated"`
}

// BaselineMetric represents a performance baseline
type BaselineMetric struct {
	Value           float64   `json:"value"`
	HistoricalMean  float64   `json:"historical_mean"`
	StandardDev     float64   `json:"standard_deviation"`
	PercentileP50   float64   `json:"percentile_p50"`
	PercentileP95   float64   `json:"percentile_p95"`
	PercentileP99   float64   `json:"percentile_p99"`
	TrendDirection  string    `json:"trend_direction"`
	LastMeasured    time.Time `json:"last_measured"`
}

// RegressionAnalysis detects performance regressions
type RegressionAnalysis struct {
	DetectedRegressions []PerformanceRegression `json:"detected_regressions"`
	OverallScore        float64                 `json:"overall_score"`
	TrendAnalysis       string                  `json:"trend_analysis"`
	SeasonalPatterns    []SeasonalPattern       `json:"seasonal_patterns"`
	AnomaliesDetected   int                     `json:"anomalies_detected"`
	LastAnalysis        time.Time               `json:"last_analysis"`
}

// PerformanceRegression represents a detected performance regression
type PerformanceRegression struct {
	ID             string    `json:"id"`
	MetricName     string    `json:"metric_name"`
	DetectedAt     time.Time `json:"detected_at"`
	Severity       string    `json:"severity"` // critical, major, minor
	BaselineValue  float64   `json:"baseline_value"`
	CurrentValue   float64   `json:"current_value"`
	PercentChange  float64   `json:"percent_change"`
	Duration       string    `json:"duration"`
	PossibleCauses []string  `json:"possible_causes"`
	Confidence     float64   `json:"confidence"`
	Status         string    `json:"status"` // active, investigating, resolved
}

// SeasonalPattern represents a seasonal usage pattern
type SeasonalPattern struct {
	Type        string    `json:"type"` // hourly, daily, weekly, monthly
	Pattern     []float64 `json:"pattern"`
	Confidence  float64   `json:"confidence"`
	Impact      string    `json:"impact"` // high, medium, low
	LastUpdated time.Time `json:"last_updated"`
}

// ScalingEvent tracks scaling activities
type ScalingEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"` // manual, automatic
	Action      string                 `json:"action"` // scale_up, scale_down
	Resource    string                 `json:"resource"`
	OldValue    float64                `json:"old_value"`
	NewValue    float64                `json:"new_value"`
	Reason      string                 `json:"reason"`
	Success     bool                   `json:"success"`
	Duration    time.Duration          `json:"duration"`
	CostImpact  float64                `json:"cost_impact_usd"`
	Details     map[string]interface{} `json:"details"`
}

// RiskAssessment evaluates capacity risks
type RiskAssessment struct {
	OverallRiskLevel  string       `json:"overall_risk_level"`
	RiskFactors       []RiskFactor `json:"risk_factors"`
	MitigationPlans   []string     `json:"mitigation_plans"`
	MonitoringAlerts  []string     `json:"monitoring_alerts"`
	ReviewDate        time.Time    `json:"next_review_date"`
}

// RiskFactor represents a capacity risk
type RiskFactor struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Probability float64 `json:"probability"`
	Impact      string  `json:"impact"`
	Score       float64 `json:"score"`
}

// PerformanceAnalyzer performs statistical analysis on performance data
type PerformanceAnalyzer struct {
	logger *zap.Logger
	window time.Duration
}

// CapacityStorage interface for capacity data persistence
type CapacityStorage interface {
	StoreCapacityReport(report *CapacityReport) error
	GetCapacityHistory(service string, window time.Duration) ([]ResourceUtilization, error)
	StorePerformanceBaseline(service string, baselines *PerformanceBaselines) error
	GetPerformanceBaseline(service string) (*PerformanceBaselines, error)
	StoreScalingEvent(event *ScalingEvent) error
	GetScalingEvents(service string, window time.Duration) ([]ScalingEvent, error)
	StoreRegression(regression *PerformanceRegression) error
	GetActiveRegressions(service string) ([]PerformanceRegression, error)
}

// NewCapacityPlanner creates a new capacity planner
func NewCapacityPlanner(logger *zap.Logger, tracing *TracingProvider, storage CapacityStorage) *CapacityPlanner {
	return &CapacityPlanner{
		logger:  logger,
		tracing: tracing,
		storage: storage,
		analyzer: NewPerformanceAnalyzer(logger, 24*time.Hour),
		
		capacityUtilization: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "capacity_utilization_percent",
			Help: "Current capacity utilization percentage",
		}, []string{"service", "resource_type"}),
		
		capacityForecast: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "capacity_forecast_utilization_percent",
			Help: "Forecasted capacity utilization percentage",
		}, []string{"service", "resource_type", "forecast_horizon"}),
		
		capacityRecommendations: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "capacity_recommendations_total",
			Help: "Number of capacity recommendations by type",
		}, []string{"service", "recommendation_type", "priority"}),
		
		performanceBaselines: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "performance_baseline_value",
			Help: "Performance baseline values",
		}, []string{"service", "metric_type", "statistic"}),
		
		regressionAlerts: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "performance_regressions_detected_total",
			Help: "Total number of performance regressions detected",
		}, []string{"service", "metric_name", "severity"}),
		
		costOptimization: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "cost_optimization_potential_usd",
			Help: "Potential cost optimization savings in USD",
		}, []string{"service", "optimization_type"}),
	}
}

// NewPerformanceAnalyzer creates a new performance analyzer
func NewPerformanceAnalyzer(logger *zap.Logger, window time.Duration) *PerformanceAnalyzer {
	return &PerformanceAnalyzer{
		logger: logger,
		window: window,
	}
}

// GenerateCapacityReport generates a comprehensive capacity planning report
func (cp *CapacityPlanner) GenerateCapacityReport(ctx context.Context, serviceName string, forecastDays int) (*CapacityReport, error) {
	ctx, span := cp.tracing.StartSpan(ctx, "capacity.generate_report")
	defer span.End()
	
	reportPeriod := fmt.Sprintf("%dd", forecastDays)
	
	// Get historical capacity data
	history, err := cp.storage.GetCapacityHistory(serviceName, 30*24*time.Hour) // 30 days of history
	if err != nil {
		return nil, fmt.Errorf("failed to get capacity history: %w", err)
	}
	
	// Get current utilization
	currentUtilization := cp.getCurrentUtilization(history)
	
	// Generate forecasts
	forecastedUtilization := cp.generateForecast(history, forecastDays)
	
	// Get performance baselines
	baselines, err := cp.storage.GetPerformanceBaseline(serviceName)
	if err != nil {
		baselines = &PerformanceBaselines{} // Use empty baselines if not found
	}
	
	// Analyze regressions
	regressionAnalysis := cp.analyzer.AnalyzeRegressions(ctx, serviceName, history, baselines)
	
	// Generate recommendations
	recommendations := cp.generateRecommendations(currentUtilization, forecastedUtilization, baselines)
	
	// Perform cost analysis
	costAnalysis := cp.analyzeCosts(currentUtilization, forecastedUtilization, recommendations)
	
	// Get scaling events
	scalingEvents, err := cp.storage.GetScalingEvents(serviceName, 30*24*time.Hour)
	if err != nil {
		scalingEvents = []ScalingEvent{} // Use empty slice if not found
	}
	
	// Assess risks
	riskAssessment := cp.assessRisks(currentUtilization, forecastedUtilization, regressionAnalysis)
	
	report := &CapacityReport{
		ServiceName:           serviceName,
		ReportPeriod:          reportPeriod,
		GeneratedAt:           time.Now(),
		CurrentUtilization:    currentUtilization,
		ForecastedUtilization: forecastedUtilization,
		Recommendations:       recommendations,
		CostAnalysis:          costAnalysis,
		PerformanceBaselines:  *baselines,
		RegressionAnalysis:    regressionAnalysis,
		ScalingEvents:         scalingEvents,
		RiskAssessment:        riskAssessment,
	}
	
	// Store report
	if err := cp.storage.StoreCapacityReport(report); err != nil {
		cp.logger.Error("Failed to store capacity report", zap.Error(err))
	}
	
	// Update metrics
	cp.updateCapacityMetrics(report)
	
	cp.logger.Info("Generated capacity planning report",
		zap.String("service", serviceName),
		zap.String("period", reportPeriod),
		zap.Int("recommendations", len(recommendations)),
		zap.Int("regressions", len(regressionAnalysis.DetectedRegressions)),
	)
	
	return report, nil
}

// getCurrentUtilization calculates current resource utilization
func (cp *CapacityPlanner) getCurrentUtilization(history []ResourceUtilization) ResourceUtilization {
	if len(history) == 0 {
		return ResourceUtilization{Timestamp: time.Now()}
	}
	
	// Use the most recent data point as current, but calculate averages and peaks
	current := history[len(history)-1]
	
	// Calculate averages and peaks from recent history (last 24 hours)
	recentWindow := time.Now().Add(-24 * time.Hour)
	var cpuValues, memValues, storageValues, networkValues, dbConnValues, queueValues []float64
	
	for _, point := range history {
		if point.Timestamp.After(recentWindow) {
			cpuValues = append(cpuValues, point.CPU.Current)
			memValues = append(memValues, point.Memory.Current)
			storageValues = append(storageValues, point.Storage.Current)
			networkValues = append(networkValues, point.Network.Current)
			dbConnValues = append(dbConnValues, point.DatabaseConns.Current)
			queueValues = append(queueValues, point.QueueDepth.Current)
		}
	}
	
	current.CPU.Average = cp.calculateAverage(cpuValues)
	current.CPU.Peak = cp.calculateMax(cpuValues)
	current.CPU.Trend = cp.calculateTrend(cpuValues)
	
	current.Memory.Average = cp.calculateAverage(memValues)
	current.Memory.Peak = cp.calculateMax(memValues)
	current.Memory.Trend = cp.calculateTrend(memValues)
	
	current.Storage.Average = cp.calculateAverage(storageValues)
	current.Storage.Peak = cp.calculateMax(storageValues)
	current.Storage.Trend = cp.calculateTrend(storageValues)
	
	current.Network.Average = cp.calculateAverage(networkValues)
	current.Network.Peak = cp.calculateMax(networkValues)
	current.Network.Trend = cp.calculateTrend(networkValues)
	
	current.DatabaseConns.Average = cp.calculateAverage(dbConnValues)
	current.DatabaseConns.Peak = cp.calculateMax(dbConnValues)
	current.DatabaseConns.Trend = cp.calculateTrend(dbConnValues)
	
	current.QueueDepth.Average = cp.calculateAverage(queueValues)
	current.QueueDepth.Peak = cp.calculateMax(queueValues)
	current.QueueDepth.Trend = cp.calculateTrend(queueValues)
	
	return current
}

// generateForecast generates capacity forecasts using time series analysis
func (cp *CapacityPlanner) generateForecast(history []ResourceUtilization, forecastDays int) []ForecastPoint {
	var forecasts []ForecastPoint
	
	if len(history) < 7 { // Need at least a week of data for forecasting
		return forecasts
	}
	
	resourceTypes := []string{"cpu", "memory", "storage", "network", "database_connections", "queue_depth"}
	
	for _, resourceType := range resourceTypes {
		values := cp.extractResourceValues(history, resourceType)
		
		// Simple linear regression forecast (in production, use more sophisticated methods)
		forecast := cp.linearRegression(values, forecastDays)
		
		for i, point := range forecast {
			forecasts = append(forecasts, ForecastPoint{
				Timestamp:          time.Now().Add(time.Duration(i) * 24 * time.Hour),
				PredictedUsage:     point.Value,
				ConfidenceInterval: point.Confidence,
				ResourceType:       resourceType,
			})
		}
	}
	
	return forecasts
}

// generateRecommendations generates capacity recommendations based on analysis
func (cp *CapacityPlanner) generateRecommendations(current ResourceUtilization, forecast []ForecastPoint, baselines *PerformanceBaselines) []CapacityRecommendation {
	var recommendations []CapacityRecommendation
	
	// CPU recommendations
	if current.CPU.Utilization > 80 {
		recommendations = append(recommendations, CapacityRecommendation{
			Type:             "scale_up",
			Priority:         "high",
			Resource:         "cpu",
			CurrentValue:     current.CPU.Current,
			RecommendedValue: current.CPU.Current * 1.5,
			Justification:    "CPU utilization is above 80%, indicating resource constraint",
			EstimatedCost:    200.0,
			Timeline:         "immediate",
			Confidence:       0.9,
			RiskLevel:        "medium",
		})
	}
	
	// Memory recommendations
	if current.Memory.Utilization > 85 {
		recommendations = append(recommendations, CapacityRecommendation{
			Type:             "scale_up",
			Priority:         "critical",
			Resource:         "memory",
			CurrentValue:     current.Memory.Current,
			RecommendedValue: current.Memory.Current * 1.3,
			Justification:    "Memory utilization is above 85%, risk of OOM errors",
			EstimatedCost:    150.0,
			Timeline:         "immediate",
			Confidence:       0.95,
			RiskLevel:        "high",
		})
	}
	
	// Storage recommendations
	if current.Storage.Utilization > 90 {
		recommendations = append(recommendations, CapacityRecommendation{
			Type:             "scale_up",
			Priority:         "high",
			Resource:         "storage",
			CurrentValue:     current.Storage.Current,
			RecommendedValue: current.Storage.Current * 1.2,
			Justification:    "Storage utilization is above 90%, risk of disk full",
			EstimatedCost:    100.0,
			Timeline:         "within_week",
			Confidence:       0.85,
			RiskLevel:        "high",
		})
	}
	
	// Database connection recommendations
	if current.DatabaseConns.Utilization > 75 {
		recommendations = append(recommendations, CapacityRecommendation{
			Type:             "optimize",
			Priority:         "medium",
			Resource:         "database_connections",
			CurrentValue:     current.DatabaseConns.Current,
			RecommendedValue: current.DatabaseConns.Capacity * 0.6,
			Justification:    "Database connection pool is highly utilized, consider connection optimization",
			EstimatedCost:    0.0,
			Timeline:         "within_month",
			Confidence:       0.7,
			RiskLevel:        "medium",
		})
	}
	
	// Look for underutilized resources
	if current.CPU.Utilization < 30 && current.Memory.Utilization < 30 {
		recommendations = append(recommendations, CapacityRecommendation{
			Type:             "scale_down",
			Priority:         "low",
			Resource:         "compute",
			CurrentValue:     current.CPU.Current,
			RecommendedValue: current.CPU.Current * 0.7,
			Justification:    "CPU and memory are underutilized, consider downsizing",
			EstimatedCost:    -300.0, // Negative indicates savings
			Timeline:         "within_quarter",
			Confidence:       0.6,
			RiskLevel:        "low",
		})
	}
	
	// Sort by priority
	sort.Slice(recommendations, func(i, j int) bool {
		priorityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
		return priorityOrder[recommendations[i].Priority] < priorityOrder[recommendations[j].Priority]
	})
	
	return recommendations
}

// analyzeCosts performs cost analysis and optimization
func (cp *CapacityPlanner) analyzeCosts(current ResourceUtilization, forecast []ForecastPoint, recommendations []CapacityRecommendation) CostAnalysis {
	// Simplified cost calculation - in production, use actual cloud pricing APIs
	currentMonthlyCost := cp.calculateCurrentCost(current)
	
	// Forecast costs based on resource projections
	var forecastedCosts []CostForecastPoint
	for i := 1; i <= 12; i++ { // 12 months forecast
		timestamp := time.Now().AddDate(0, i, 0)
		predictedCost := currentMonthlyCost * (1.0 + 0.05*float64(i)) // 5% monthly growth
		
		forecastedCosts = append(forecastedCosts, CostForecastPoint{
			Timestamp:     timestamp,
			PredictedCost: predictedCost,
			Confidence:    0.7,
		})
	}
	
	// Calculate potential savings from recommendations
	var potentialSavings float64
	var optimizations []CostOptimization
	
	for _, rec := range recommendations {
		if rec.EstimatedCost < 0 { // Negative cost means savings
			potentialSavings += math.Abs(rec.EstimatedCost)
			
			optimizations = append(optimizations, CostOptimization{
				Type:                rec.Type,
				Description:         rec.Justification,
				PotentialSaving:     math.Abs(rec.EstimatedCost),
				ImplementationEffort: "medium",
				RiskLevel:           rec.RiskLevel,
			})
		}
	}
	
	roi := ROIAnalysis{
		InvestmentRequired: 500.0, // Example investment
		AnnualSavings:     potentialSavings * 12,
		PaybackPeriod:     6,
		ROIPercentage:     150.0,
	}
	
	return CostAnalysis{
		CurrentMonthlyCost:   currentMonthlyCost,
		ForecastedCost:       forecastedCosts,
		PotentialSavings:     potentialSavings,
		CostOptimizations:    optimizations,
		ROIAnalysis:          roi,
		BudgetUtilization:    75.0, // Example
		CostPerTransaction:   0.05,  // Example
		CostEfficiencyTrend:  "improving",
	}
}

// assessRisks evaluates capacity planning risks
func (cp *CapacityPlanner) assessRisks(current ResourceUtilization, forecast []ForecastPoint, regressions RegressionAnalysis) RiskAssessment {
	var riskFactors []RiskFactor
	var overallRiskLevel string
	var mitigationPlans []string
	
	// Assess high utilization risks
	if current.CPU.Utilization > 80 || current.Memory.Utilization > 80 {
		riskFactors = append(riskFactors, RiskFactor{
			Type:        "resource_exhaustion",
			Description: "High resource utilization may lead to performance degradation",
			Probability: 0.8,
			Impact:      "high",
			Score:       8.0,
		})
		mitigationPlans = append(mitigationPlans, "Scale up resources proactively")
	}
	
	// Assess regression risks
	if len(regressions.DetectedRegressions) > 0 {
		riskFactors = append(riskFactors, RiskFactor{
			Type:        "performance_regression",
			Description: "Performance regressions detected that may impact user experience",
			Probability: 0.7,
			Impact:      "medium",
			Score:       7.0,
		})
		mitigationPlans = append(mitigationPlans, "Investigate and resolve performance regressions")
	}
	
	// Calculate overall risk level
	totalScore := 0.0
	for _, factor := range riskFactors {
		totalScore += factor.Score
	}
	
	avgScore := totalScore / float64(len(riskFactors))
	if avgScore > 7.0 {
		overallRiskLevel = "high"
	} else if avgScore > 4.0 {
		overallRiskLevel = "medium"
	} else {
		overallRiskLevel = "low"
	}
	
	return RiskAssessment{
		OverallRiskLevel: overallRiskLevel,
		RiskFactors:      riskFactors,
		MitigationPlans:  mitigationPlans,
		MonitoringAlerts: []string{"high_cpu_utilization", "memory_pressure", "performance_regression"},
		ReviewDate:       time.Now().AddDate(0, 1, 0), // Review monthly
	}
}

// AnalyzeRegressions detects performance regressions
func (pa *PerformanceAnalyzer) AnalyzeRegressions(ctx context.Context, serviceName string, history []ResourceUtilization, baselines *PerformanceBaselines) RegressionAnalysis {
	var detectedRegressions []PerformanceRegression
	var overallScore float64 = 100.0 // Start with perfect score
	
	// Analyze response time regression (simplified)
	if baselines.ResponseTime.Value > 0 {
		recentResponseTime := pa.calculateRecentAverage(history, "response_time")
		percentChange := ((recentResponseTime - baselines.ResponseTime.HistoricalMean) / baselines.ResponseTime.HistoricalMean) * 100
		
		if percentChange > 20 { // 20% increase considered regression
			regression := PerformanceRegression{
				ID:             fmt.Sprintf("reg_%s_%d", serviceName, time.Now().Unix()),
				MetricName:     "response_time",
				DetectedAt:     time.Now(),
				Severity:       pa.calculateSeverity(percentChange),
				BaselineValue:  baselines.ResponseTime.HistoricalMean,
				CurrentValue:   recentResponseTime,
				PercentChange:  percentChange,
				Duration:       "ongoing",
				PossibleCauses: []string{"increased load", "database performance", "external dependencies"},
				Confidence:     0.85,
				Status:         "active",
			}
			
			detectedRegressions = append(detectedRegressions, regression)
			overallScore -= percentChange // Reduce score based on regression severity
		}
	}
	
	// Analyze throughput regression
	if baselines.Throughput.Value > 0 {
		recentThroughput := pa.calculateRecentAverage(history, "throughput")
		percentChange := ((baselines.Throughput.HistoricalMean - recentThroughput) / baselines.Throughput.HistoricalMean) * 100
		
		if percentChange > 15 { // 15% decrease considered regression
			regression := PerformanceRegression{
				ID:             fmt.Sprintf("reg_%s_%d", serviceName, time.Now().Unix()),
				MetricName:     "throughput",
				DetectedAt:     time.Now(),
				Severity:       pa.calculateSeverity(percentChange),
				BaselineValue:  baselines.Throughput.HistoricalMean,
				CurrentValue:   recentThroughput,
				PercentChange:  -percentChange, // Negative because throughput decreased
				Duration:       "ongoing",
				PossibleCauses: []string{"resource constraints", "configuration changes", "code regression"},
				Confidence:     0.8,
				Status:         "active",
			}
			
			detectedRegressions = append(detectedRegressions, regression)
			overallScore -= percentChange
		}
	}
	
	// Ensure score doesn't go below 0
	if overallScore < 0 {
		overallScore = 0
	}
	
	return RegressionAnalysis{
		DetectedRegressions: detectedRegressions,
		OverallScore:       overallScore,
		TrendAnalysis:      "stable", // Simplified
		SeasonalPatterns:   []SeasonalPattern{}, // Would require more complex analysis
		AnomaliesDetected:  len(detectedRegressions),
		LastAnalysis:       time.Now(),
	}
}

// Helper methods
func (cp *CapacityPlanner) calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func (cp *CapacityPlanner) calculateMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	
	max := values[0]
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return max
}

func (cp *CapacityPlanner) calculateTrend(values []float64) string {
	if len(values) < 2 {
		return "stable"
	}
	
	first := values[0]
	last := values[len(values)-1]
	change := (last - first) / first
	
	if change > 0.1 {
		return "increasing"
	} else if change < -0.1 {
		return "decreasing"
	}
	return "stable"
}

func (cp *CapacityPlanner) extractResourceValues(history []ResourceUtilization, resourceType string) []float64 {
	var values []float64
	
	for _, point := range history {
		switch resourceType {
		case "cpu":
			values = append(values, point.CPU.Current)
		case "memory":
			values = append(values, point.Memory.Current)
		case "storage":
			values = append(values, point.Storage.Current)
		case "network":
			values = append(values, point.Network.Current)
		case "database_connections":
			values = append(values, point.DatabaseConns.Current)
		case "queue_depth":
			values = append(values, point.QueueDepth.Current)
		}
	}
	
	return values
}

type ForecastValue struct {
	Value      float64
	Confidence float64
}

func (cp *CapacityPlanner) linearRegression(values []float64, forecastDays int) []ForecastValue {
	var forecast []ForecastValue
	
	if len(values) < 2 {
		return forecast
	}
	
	// Simple linear regression
	n := float64(len(values))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n
	
	// Generate forecast
	for i := 0; i < forecastDays; i++ {
		x := float64(len(values) + i)
		predictedValue := slope*x + intercept
		confidence := 0.8 - (float64(i) * 0.05) // Confidence decreases with distance
		
		if confidence < 0.2 {
			confidence = 0.2 // Minimum confidence
		}
		
		forecast = append(forecast, ForecastValue{
			Value:      predictedValue,
			Confidence: confidence,
		})
	}
	
	return forecast
}

func (cp *CapacityPlanner) calculateCurrentCost(current ResourceUtilization) float64 {
	// Simplified cost calculation
	cpuCost := current.CPU.Current * 0.10    // $0.10 per CPU unit
	memoryCost := current.Memory.Current * 0.05  // $0.05 per GB
	storageCost := current.Storage.Current * 0.02  // $0.02 per GB
	networkCost := current.Network.Current * 0.01  // $0.01 per GB
	
	return cpuCost + memoryCost + storageCost + networkCost
}

func (pa *PerformanceAnalyzer) calculateRecentAverage(history []ResourceUtilization, metric string) float64 {
	// This would extract the specific metric from history and calculate average
	// Simplified implementation
	return 100.0 // Placeholder
}

func (pa *PerformanceAnalyzer) calculateSeverity(percentChange float64) string {
	if percentChange > 50 {
		return "critical"
	} else if percentChange > 30 {
		return "major"
	}
	return "minor"
}

func (cp *CapacityPlanner) updateCapacityMetrics(report *CapacityReport) {
	service := report.ServiceName
	
	// Update utilization metrics
	cp.capacityUtilization.WithLabelValues(service, "cpu").Set(report.CurrentUtilization.CPU.Utilization)
	cp.capacityUtilization.WithLabelValues(service, "memory").Set(report.CurrentUtilization.Memory.Utilization)
	cp.capacityUtilization.WithLabelValues(service, "storage").Set(report.CurrentUtilization.Storage.Utilization)
	
	// Update recommendation metrics
	for _, rec := range report.Recommendations {
		cp.capacityRecommendations.WithLabelValues(service, rec.Type, rec.Priority).Inc()
	}
	
	// Update cost optimization metrics
	for _, opt := range report.CostAnalysis.CostOptimizations {
		cp.costOptimization.WithLabelValues(service, opt.Type).Set(opt.PotentialSaving)
	}
	
	// Update regression metrics
	for _, regression := range report.RegressionAnalysis.DetectedRegressions {
		cp.regressionAlerts.WithLabelValues(service, regression.MetricName, regression.Severity).Inc()
	}
}

// HTTP Handlers
func (cp *CapacityPlanner) GetCapacityReport(c *gin.Context) {
	serviceName := c.Param("service")
	forecastDays := 30 // Default
	
	if days := c.Query("forecast_days"); days != "" {
		if parsed, err := strconv.Atoi(days); err == nil {
			forecastDays = parsed
		}
	}
	
	report, err := cp.GenerateCapacityReport(c.Request.Context(), serviceName, forecastDays)
	if err != nil {
		cp.logger.Error("Failed to generate capacity report",
			zap.String("service", serviceName),
			zap.Error(err),
		)
		c.JSON(500, gin.H{"error": "Failed to generate capacity report"})
		return
	}
	
	c.JSON(200, report)
}

func (cp *CapacityPlanner) GetPerformanceRegressions(c *gin.Context) {
	serviceName := c.Param("service")
	
	regressions, err := cp.storage.GetActiveRegressions(serviceName)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get regressions"})
		return
	}
	
	c.JSON(200, gin.H{
		"service":     serviceName,
		"regressions": regressions,
		"count":       len(regressions),
	})
}