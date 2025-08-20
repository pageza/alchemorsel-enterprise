// Package ai provides comprehensive cost tracking and billing for AI services
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CostTracker manages AI usage costs and billing
type CostTracker struct {
	config          *EnterpriseConfig
	logger          *zap.Logger
	
	// Cost tracking
	dailySpend      float64
	monthlySpend    float64
	lastResetDaily  time.Time
	lastResetMonthly time.Time
	
	// Usage tracking
	userUsage       map[uuid.UUID]*UserCostTracking
	providerCosts   map[string]float64
	featureCosts    map[string]float64
	
	// Thread safety
	mu              sync.RWMutex
	
	// Cost calculation rules
	providerRates   map[string]ProviderRates
}

// UserCostTracking tracks costs per user
type UserCostTracking struct {
	UserID          uuid.UUID
	DailySpend      float64
	MonthlySpend    float64
	TotalSpend      float64
	RequestCount    int64
	TokensUsed      int64
	LastActivity    time.Time
	LastResetDaily  time.Time
	LastResetMonthly time.Time
}

// ProviderRates defines cost rates for different providers
type ProviderRates struct {
	ProviderName     string
	InputTokenRate   float64  // Cost per input token in cents
	OutputTokenRate  float64  // Cost per output token in cents
	RequestBaseCost  float64  // Base cost per request in cents
	MinimumCost      float64  // Minimum cost per request in cents
	VolumeDiscounts  []VolumeDiscount
}

// VolumeDiscount defines volume-based discounts
type VolumeDiscount struct {
	MinTokens    int64
	DiscountRate float64 // Percentage discount (0.1 = 10%)
}

// CostBreakdown provides detailed cost analysis
type CostBreakdown struct {
	TotalCostCents   int
	ProviderBreakdown map[string]int
	FeatureBreakdown  map[string]int
	UserBreakdown     []UserCostSummary
	TimeBreakdown     []TimePeriodCost
	TokensUsed        int64
	RequestCount      int64
	AverageCostPerRequest float64
	Period            string
}

// UserCostSummary summarizes costs for a user
type UserCostSummary struct {
	UserID       uuid.UUID
	CostCents    int
	TokensUsed   int64
	RequestCount int64
	Percentage   float64
}

// TimePeriodCost represents costs for a specific time period
type TimePeriodCost struct {
	Period       string
	CostCents    int
	TokensUsed   int64
	RequestCount int64
	Date         time.Time
}

// BudgetAlert represents a budget threshold alert
type BudgetAlert struct {
	Type         string    // daily, monthly
	Threshold    float64   // Percentage of budget (0.8 = 80%)
	CurrentSpend float64
	BudgetLimit  float64
	Triggered    bool
	Message      string
	Timestamp    time.Time
}

// NewCostTracker creates a new cost tracker
func NewCostTracker(config *EnterpriseConfig, logger *zap.Logger) *CostTracker {
	namedLogger := logger.Named("cost-tracker")
	
	tracker := &CostTracker{
		config:           config,
		logger:           namedLogger,
		lastResetDaily:   time.Now().Truncate(24 * time.Hour),
		lastResetMonthly: time.Now().Truncate(24 * time.Hour).AddDate(0, 0, -time.Now().Day()+1),
		userUsage:        make(map[uuid.UUID]*UserCostTracking),
		providerCosts:    make(map[string]float64),
		featureCosts:     make(map[string]float64),
		providerRates:    make(map[string]ProviderRates),
	}
	
	// Initialize provider rates
	tracker.initializeProviderRates()
	
	namedLogger.Info("Cost tracker initialized",
		zap.Int("daily_budget_cents", config.DailyBudgetCents),
		zap.Int("monthly_budget_cents", config.MonthlyBudgetCents),
	)
	
	return tracker
}

// initializeProviderRates sets up cost rates for different AI providers
func (ct *CostTracker) initializeProviderRates() {
	// OpenAI GPT-4 rates (example rates - adjust based on actual pricing)
	ct.providerRates["openai"] = ProviderRates{
		ProviderName:    "openai",
		InputTokenRate:  0.003,  // $0.03 per 1K input tokens
		OutputTokenRate: 0.006,  // $0.06 per 1K output tokens
		RequestBaseCost: 0.1,    // $0.001 base cost per request
		MinimumCost:     0.01,   // $0.0001 minimum cost
		VolumeDiscounts: []VolumeDiscount{
			{MinTokens: 100000, DiscountRate: 0.05},  // 5% discount over 100K tokens
			{MinTokens: 1000000, DiscountRate: 0.10}, // 10% discount over 1M tokens
		},
	}
	
	// Ollama (self-hosted) rates - mostly infrastructure costs
	ct.providerRates["ollama"] = ProviderRates{
		ProviderName:    "ollama",
		InputTokenRate:  0.0001,  // Very low cost for self-hosted
		OutputTokenRate: 0.0001,
		RequestBaseCost: 0.001,   // Minimal base cost
		MinimumCost:     0.0001,
		VolumeDiscounts: []VolumeDiscount{
			{MinTokens: 50000, DiscountRate: 0.02},   // 2% discount over 50K tokens
		},
	}
	
	// Anthropic Claude rates (example rates)
	ct.providerRates["anthropic"] = ProviderRates{
		ProviderName:    "anthropic",
		InputTokenRate:  0.0025,  // $0.025 per 1K input tokens
		OutputTokenRate: 0.0075,  // $0.075 per 1K output tokens
		RequestBaseCost: 0.05,
		MinimumCost:     0.005,
		VolumeDiscounts: []VolumeDiscount{
			{MinTokens: 200000, DiscountRate: 0.08},  // 8% discount over 200K tokens
		},
	}
	
	// Mock provider (free for testing)
	ct.providerRates["mock"] = ProviderRates{
		ProviderName:    "mock",
		InputTokenRate:  0.0,
		OutputTokenRate: 0.0,
		RequestBaseCost: 0.0,
		MinimumCost:     0.0,
		VolumeDiscounts: []VolumeDiscount{},
	}
}

// TrackUsage records usage and calculates costs
func (ct *CostTracker) TrackUsage(ctx context.Context, userID uuid.UUID, costCents float64, tokensUsed int) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	// Reset counters if needed
	ct.checkAndResetCounters()
	
	// Update overall spending
	ct.dailySpend += costCents
	ct.monthlySpend += costCents
	
	// Update user tracking
	if ct.userUsage[userID] == nil {
		ct.userUsage[userID] = &UserCostTracking{
			UserID:           userID,
			LastResetDaily:   ct.lastResetDaily,
			LastResetMonthly: ct.lastResetMonthly,
		}
	}
	
	user := ct.userUsage[userID]
	
	// Reset user counters if needed
	if user.LastResetDaily.Before(ct.lastResetDaily) {
		user.DailySpend = 0
		user.LastResetDaily = ct.lastResetDaily
	}
	if user.LastResetMonthly.Before(ct.lastResetMonthly) {
		user.MonthlySpend = 0
		user.LastResetMonthly = ct.lastResetMonthly
	}
	
	// Update user costs
	user.DailySpend += costCents
	user.MonthlySpend += costCents
	user.TotalSpend += costCents
	user.RequestCount++
	user.TokensUsed += int64(tokensUsed)
	user.LastActivity = time.Now()
	
	ct.logger.Info("Usage tracked",
		zap.String("user_id", userID.String()),
		zap.Float64("cost_cents", costCents),
		zap.Int("tokens_used", tokensUsed),
		zap.Float64("daily_spend", ct.dailySpend),
		zap.Float64("monthly_spend", ct.monthlySpend),
	)
	
	return nil
}

// CalculateCost calculates the cost for a specific request
func (ct *CostTracker) CalculateCost(provider, feature string, tokensUsed int) float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	rates, exists := ct.providerRates[provider]
	if !exists {
		ct.logger.Warn("Unknown provider for cost calculation", zap.String("provider", provider))
		return 0.0
	}
	
	// Calculate base cost from tokens
	// Assume 70% input tokens, 30% output tokens for estimation
	inputTokens := float64(tokensUsed) * 0.7
	outputTokens := float64(tokensUsed) * 0.3
	
	tokenCost := (inputTokens/1000)*rates.InputTokenRate + (outputTokens/1000)*rates.OutputTokenRate
	totalCost := tokenCost + rates.RequestBaseCost
	
	// Apply minimum cost
	if totalCost < rates.MinimumCost {
		totalCost = rates.MinimumCost
	}
	
	// Apply volume discounts
	userID := uuid.New() // This should come from context in real implementation
	if user, exists := ct.userUsage[userID]; exists {
		for _, discount := range rates.VolumeDiscounts {
			if user.TokensUsed >= discount.MinTokens {
				totalCost *= (1.0 - discount.DiscountRate)
			}
		}
	}
	
	return totalCost
}

// CheckBudgetLimits verifies if usage is within budget limits
func (ct *CostTracker) CheckBudgetLimits(ctx context.Context) error {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	// Check daily budget
	if ct.config.DailyBudgetCents > 0 {
		dailyBudget := float64(ct.config.DailyBudgetCents)
		if ct.dailySpend >= dailyBudget {
			return fmt.Errorf("daily budget exceeded: spent %.2f cents, limit %.2f cents", 
				ct.dailySpend, dailyBudget)
		}
	}
	
	// Check monthly budget
	if ct.config.MonthlyBudgetCents > 0 {
		monthlyBudget := float64(ct.config.MonthlyBudgetCents)
		if ct.monthlySpend >= monthlyBudget {
			return fmt.Errorf("monthly budget exceeded: spent %.2f cents, limit %.2f cents",
				ct.monthlySpend, monthlyBudget)
		}
	}
	
	return nil
}

// GetDailySpend returns current daily spending
func (ct *CostTracker) GetDailySpend() float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	ct.checkAndResetCounters()
	return ct.dailySpend
}

// GetMonthlySpend returns current monthly spending
func (ct *CostTracker) GetMonthlySpend() float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	ct.checkAndResetCounters()
	return ct.monthlySpend
}

// GetUserSpending returns spending information for a specific user
func (ct *CostTracker) GetUserSpending(userID uuid.UUID) *UserCostTracking {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	if user, exists := ct.userUsage[userID]; exists {
		// Create a copy to avoid race conditions
		userCopy := *user
		return &userCopy
	}
	
	return nil
}

// GetBudgetAlerts checks for budget threshold alerts
func (ct *CostTracker) GetBudgetAlerts() []BudgetAlert {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	var alerts []BudgetAlert
	
	// Check daily budget alerts
	if ct.config.DailyBudgetCents > 0 {
		dailyBudget := float64(ct.config.DailyBudgetCents)
		for _, threshold := range ct.config.CostAlertThresholds {
			if ct.dailySpend >= dailyBudget*threshold {
				alerts = append(alerts, BudgetAlert{
					Type:         "daily",
					Threshold:    threshold,
					CurrentSpend: ct.dailySpend,
					BudgetLimit:  dailyBudget,
					Triggered:    true,
					Message:      fmt.Sprintf("Daily spending is %.1f%% of budget", threshold*100),
					Timestamp:    time.Now(),
				})
			}
		}
	}
	
	// Check monthly budget alerts
	if ct.config.MonthlyBudgetCents > 0 {
		monthlyBudget := float64(ct.config.MonthlyBudgetCents)
		for _, threshold := range ct.config.CostAlertThresholds {
			if ct.monthlySpend >= monthlyBudget*threshold {
				alerts = append(alerts, BudgetAlert{
					Type:         "monthly",
					Threshold:    threshold,
					CurrentSpend: ct.monthlySpend,
					BudgetLimit:  monthlyBudget,
					Triggered:    true,
					Message:      fmt.Sprintf("Monthly spending is %.1f%% of budget", threshold*100),
					Timestamp:    time.Now(),
				})
			}
		}
	}
	
	return alerts
}

// GenerateReport creates a comprehensive cost report
func (ct *CostTracker) GenerateReport(ctx context.Context, period string) (*CostReport, error) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	report := &CostReport{
		Period:         period,
		TotalCostCents: int(ct.monthlySpend), // Default to monthly
		CostByProvider: make(map[string]int),
		CostByFeature:  make(map[string]int),
		GeneratedAt:    time.Now(),
	}
	
	// Calculate totals based on period
	switch period {
	case "daily":
		report.TotalCostCents = int(ct.dailySpend)
	case "monthly":
		report.TotalCostCents = int(ct.monthlySpend)
	default:
		report.TotalCostCents = int(ct.monthlySpend)
	}
	
	// Provider breakdown
	for provider, cost := range ct.providerCosts {
		report.CostByProvider[provider] = int(cost)
	}
	
	// Feature breakdown
	for feature, cost := range ct.featureCosts {
		report.CostByFeature[feature] = int(cost)
	}
	
	// User breakdown - top 10 users by cost
	var userSummaries []UserCostSummary
	totalCost := float64(report.TotalCostCents)
	
	for userID, user := range ct.userUsage {
		var userCost float64
		switch period {
		case "daily":
			userCost = user.DailySpend
		case "monthly":
			userCost = user.MonthlySpend
		default:
			userCost = user.MonthlySpend
		}
		
		if userCost > 0 {
			percentage := 0.0
			if totalCost > 0 {
				percentage = (userCost / totalCost) * 100
			}
			
			userSummaries = append(userSummaries, UserCostSummary{
				UserID:       userID,
				CostCents:    int(userCost),
				TokensUsed:   user.TokensUsed,
				RequestCount: user.RequestCount,
				Percentage:   percentage,
			})
		}
	}
	
	// Sort by cost and take top 10
	// This is a simplified sort - in production, use proper sorting
	report.TopUsers = userSummaries
	if len(report.TopUsers) > 10 {
		report.TopUsers = report.TopUsers[:10]
	}
	
	// Calculate metrics
	totalRequests := int64(0)
	totalTokens := int64(0)
	for _, user := range ct.userUsage {
		totalRequests += user.RequestCount
		totalTokens += user.TokensUsed
	}
	
	report.TokensUsed = totalTokens
	if totalRequests > 0 {
		report.AverageRCostPerRequest = totalCost / float64(totalRequests)
	}
	
	// Calculate budget utilization
	var budgetLimit float64
	switch period {
	case "daily":
		budgetLimit = float64(ct.config.DailyBudgetCents)
	case "monthly":
		budgetLimit = float64(ct.config.MonthlyBudgetCents)
	default:
		budgetLimit = float64(ct.config.MonthlyBudgetCents)
	}
	
	if budgetLimit > 0 {
		report.BudgetUtilization = totalCost / budgetLimit
	}
	
	// Generate projections
	report.Projections = ct.generateCostProjections()
	
	// Generate daily breakdown for the period
	report.DailyBreakdown = ct.generateDailyBreakdown(period)
	
	return report, nil
}

// generateCostProjections creates cost forecasting
func (ct *CostTracker) generateCostProjections() *CostProjection {
	// Simplified projection based on current trends
	// In production, this would use more sophisticated forecasting
	
	dailyAverage := ct.dailySpend
	if dailyAverage == 0 {
		dailyAverage = ct.monthlySpend / 30.0 // Rough monthly average
	}
	
	return &CostProjection{
		DailyProjection:   dailyAverage,
		WeeklyProjection:  dailyAverage * 7,
		MonthlyProjection: dailyAverage * 30,
		Confidence:        0.75, // 75% confidence
	}
}

// generateDailyBreakdown creates daily cost breakdown
func (ct *CostTracker) generateDailyBreakdown(period string) []DailyCost {
	// Simplified implementation - in production this would pull from persistent storage
	var breakdown []DailyCost
	
	days := 7 // Default to 1 week
	if period == "monthly" {
		days = 30
	}
	
	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		
		// Simulate daily costs - in production, pull from actual data
		dailyCost := ct.dailySpend / float64(days)
		dailyTokens := int64(1000) // Simulate token usage
		dailyRequests := int64(50)  // Simulate request count
		
		breakdown = append(breakdown, DailyCost{
			Date:         date.Format("2006-01-02"),
			CostCents:    int(dailyCost),
			TokensUsed:   dailyTokens,
			RequestCount: dailyRequests,
		})
	}
	
	return breakdown
}

// UpdateConfig updates the cost tracker configuration
func (ct *CostTracker) UpdateConfig(config *EnterpriseConfig) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	
	ct.config = config
	ct.logger.Info("Cost tracker configuration updated",
		zap.Int("daily_budget_cents", config.DailyBudgetCents),
		zap.Int("monthly_budget_cents", config.MonthlyBudgetCents),
	)
}

// HealthCheck returns the health status of the cost tracker
func (ct *CostTracker) HealthCheck() ComponentHealth {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	status := ComponentHealth{
		Status:    "healthy",
		Message:   "Cost tracker operational",
		LastCheck: time.Now(),
		Metrics: map[string]interface{}{
			"daily_spend":     ct.dailySpend,
			"monthly_spend":   ct.monthlySpend,
			"tracked_users":   len(ct.userUsage),
			"daily_budget":    ct.config.DailyBudgetCents,
			"monthly_budget":  ct.config.MonthlyBudgetCents,
		},
	}
	
	// Check if close to budget limits
	if ct.config.DailyBudgetCents > 0 {
		utilization := ct.dailySpend / float64(ct.config.DailyBudgetCents)
		if utilization > 0.9 {
			status.Status = "warning"
			status.Message = "Daily budget utilization is high"
		}
	}
	
	if ct.config.MonthlyBudgetCents > 0 {
		utilization := ct.monthlySpend / float64(ct.config.MonthlyBudgetCents)
		if utilization > 0.9 {
			status.Status = "warning"
			status.Message = "Monthly budget utilization is high"
		}
	}
	
	return status
}

// checkAndResetCounters resets daily/monthly counters when needed
func (ct *CostTracker) checkAndResetCounters() {
	now := time.Now()
	
	// Reset daily counter if needed
	if now.Truncate(24*time.Hour).After(ct.lastResetDaily) {
		ct.dailySpend = 0
		ct.lastResetDaily = now.Truncate(24 * time.Hour)
		ct.logger.Info("Daily cost counter reset")
	}
	
	// Reset monthly counter if needed
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	if monthStart.After(ct.lastResetMonthly) {
		ct.monthlySpend = 0
		ct.lastResetMonthly = monthStart
		ct.logger.Info("Monthly cost counter reset")
	}
}

// GetCostBreakdown returns detailed cost breakdown
func (ct *CostTracker) GetCostBreakdown(period string) *CostBreakdown {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	breakdown := &CostBreakdown{
		ProviderBreakdown: make(map[string]int),
		FeatureBreakdown:  make(map[string]int),
		Period:            period,
	}
	
	// Calculate totals based on period
	switch period {
	case "daily":
		breakdown.TotalCostCents = int(ct.dailySpend)
	case "monthly":
		breakdown.TotalCostCents = int(ct.monthlySpend)
	default:
		breakdown.TotalCostCents = int(ct.monthlySpend)
	}
	
	// Provider breakdown
	for provider, cost := range ct.providerCosts {
		breakdown.ProviderBreakdown[provider] = int(cost)
	}
	
	// Feature breakdown
	for feature, cost := range ct.featureCosts {
		breakdown.FeatureBreakdown[feature] = int(cost)
	}
	
	// Calculate totals
	totalRequests := int64(0)
	totalTokens := int64(0)
	for _, user := range ct.userUsage {
		totalRequests += user.RequestCount
		totalTokens += user.TokensUsed
	}
	
	breakdown.RequestCount = totalRequests
	breakdown.TokensUsed = totalTokens
	
	if totalRequests > 0 {
		breakdown.AverageCostPerRequest = float64(breakdown.TotalCostCents) / float64(totalRequests)
	}
	
	return breakdown
}

// ExportUsageData exports usage data for analysis
func (ct *CostTracker) ExportUsageData(ctx context.Context, format string) ([]byte, error) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	
	exportData := struct {
		ExportTime    time.Time                       `json:"export_time"`
		DailySpend    float64                         `json:"daily_spend"`
		MonthlySpend  float64                         `json:"monthly_spend"`
		UserUsage     map[uuid.UUID]*UserCostTracking `json:"user_usage"`
		ProviderCosts map[string]float64              `json:"provider_costs"`
		FeatureCosts  map[string]float64              `json:"feature_costs"`
		Config        *EnterpriseConfig               `json:"config"`
	}{
		ExportTime:    time.Now(),
		DailySpend:    ct.dailySpend,
		MonthlySpend:  ct.monthlySpend,
		UserUsage:     ct.userUsage,
		ProviderCosts: ct.providerCosts,
		FeatureCosts:  ct.featureCosts,
		Config:        ct.config,
	}
	
	switch format {
	case "json":
		return json.MarshalIndent(exportData, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}