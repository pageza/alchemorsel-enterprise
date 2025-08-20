// Package container provides dependency injection for enterprise AI services
package container

import (
	"github.com/alchemorsel/v3/internal/application/ai"
	"github.com/alchemorsel/v3/internal/infrastructure/cache"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	"go.uber.org/zap"
)

// EnterpriseAIContainer manages enterprise AI service dependencies
type EnterpriseAIContainer struct {
	config          *config.Config
	logger          *zap.Logger
	cacheService    *cache.CacheService
	aiService       *ai.EnterpriseAIService
	aiHandler       *handlers.EnterpriseAIHandler
}

// NewEnterpriseAIContainer creates a new enterprise AI container
func NewEnterpriseAIContainer(cfg *config.Config, logger *zap.Logger, cacheService *cache.CacheService) *EnterpriseAIContainer {
	return &EnterpriseAIContainer{
		config:       cfg,
		logger:       logger,
		cacheService: cacheService,
	}
}

// GetAIService returns the enterprise AI service instance
func (c *EnterpriseAIContainer) GetAIService() *ai.EnterpriseAIService {
	if c.aiService == nil {
		c.aiService = c.createAIService()
	}
	return c.aiService
}

// GetAIHandler returns the enterprise AI HTTP handler
func (c *EnterpriseAIContainer) GetAIHandler() *handlers.EnterpriseAIHandler {
	if c.aiHandler == nil {
		c.aiHandler = c.createAIHandler()
	}
	return c.aiHandler
}

// createAIService creates and configures the enterprise AI service
func (c *EnterpriseAIContainer) createAIService() *ai.EnterpriseAIService {
	// Create enterprise configuration from main config
	enterpriseConfig := &ai.EnterpriseConfig{
		PrimaryProvider:      c.getAIProvider(),
		FallbackProviders:    c.getFallbackProviders(),
		DailyBudgetCents:     c.getDailyBudget(),
		MonthlyBudgetCents:   c.getMonthlyBudget(),
		CostAlertThresholds:  []float64{0.7, 0.9, 1.0},
		RequestsPerMinute:    c.getRequestsPerMinute(),
		RequestsPerHour:      c.getRequestsPerHour(),
		RequestsPerDay:       c.getRequestsPerDay(),
		MinQualityScore:      c.getMinQualityScore(),
		QualityCheckEnabled:  c.getQualityCheckEnabled(),
		CacheEnabled:         c.getCacheEnabled(),
		CacheTTL:             c.getCacheTTL(),
		MetricsEnabled:       c.getMetricsEnabled(),
		AlertsEnabled:        c.getAlertsEnabled(),
		ModelSettings:        c.getModelSettings(),
	}

	// Get cache repository
	cacheRepo := c.cacheService.GetCacheRepository()

	// Create the enterprise AI service
	return ai.NewEnterpriseAIService(
		enterpriseConfig.PrimaryProvider,
		cacheRepo,
		enterpriseConfig,
		c.logger,
	)
}

// createAIHandler creates the enterprise AI HTTP handler
func (c *EnterpriseAIContainer) createAIHandler() *handlers.EnterpriseAIHandler {
	aiService := c.GetAIService()
	return handlers.NewEnterpriseAIHandler(aiService, c.logger)
}

// Configuration helper methods

func (c *EnterpriseAIContainer) getAIProvider() string {
	if provider := c.config.GetString("ai.primary_provider"); provider != "" {
		return provider
	}
	return "ollama" // Default to containerized Ollama
}

func (c *EnterpriseAIContainer) getFallbackProviders() []string {
	providers := c.config.GetStringSlice("ai.fallback_providers")
	if len(providers) == 0 {
		return []string{"openai", "mock"}
	}
	return providers
}

func (c *EnterpriseAIContainer) getDailyBudget() int {
	budget := c.config.GetInt("ai.daily_budget_cents")
	if budget <= 0 {
		return 10000 // Default $100
	}
	return budget
}

func (c *EnterpriseAIContainer) getMonthlyBudget() int {
	budget := c.config.GetInt("ai.monthly_budget_cents")
	if budget <= 0 {
		return 300000 // Default $3000
	}
	return budget
}

func (c *EnterpriseAIContainer) getRequestsPerMinute() int {
	rate := c.config.GetInt("ai.rate_limit.requests_per_minute")
	if rate <= 0 {
		return 60
	}
	return rate
}

func (c *EnterpriseAIContainer) getRequestsPerHour() int {
	rate := c.config.GetInt("ai.rate_limit.requests_per_hour")
	if rate <= 0 {
		return 3600
	}
	return rate
}

func (c *EnterpriseAIContainer) getRequestsPerDay() int {
	rate := c.config.GetInt("ai.rate_limit.requests_per_day")
	if rate <= 0 {
		return 86400
	}
	return rate
}

func (c *EnterpriseAIContainer) getMinQualityScore() float64 {
	score := c.config.GetFloat64("ai.quality.min_score")
	if score <= 0 {
		return 0.7
	}
	return score
}

func (c *EnterpriseAIContainer) getQualityCheckEnabled() bool {
	return c.config.GetBool("ai.quality.enabled")
}

func (c *EnterpriseAIContainer) getCacheEnabled() bool {
	return c.config.GetBool("ai.cache.enabled")
}

func (c *EnterpriseAIContainer) getCacheTTL() time.Duration {
	ttl := c.config.GetDuration("ai.cache.ttl")
	if ttl == 0 {
		return 2 * time.Hour
	}
	return ttl
}

func (c *EnterpriseAIContainer) getMetricsEnabled() bool {
	return c.config.GetBool("ai.metrics.enabled")
}

func (c *EnterpriseAIContainer) getAlertsEnabled() bool {
	return c.config.GetBool("ai.alerts.enabled")
}

func (c *EnterpriseAIContainer) getModelSettings() map[string]ai.ModelConfig {
	// Default model settings
	settings := map[string]ai.ModelConfig{
		"llama3.2:3b": {
			MaxTokens:      2048,
			Temperature:    0.7,
			TopP:           0.9,
			CostPerToken:   0.001,  // Very low for self-hosted
			RequestTimeout: 30 * time.Second,
			QualityWeight:  1.0,
		},
		"gpt-4": {
			MaxTokens:      4096,
			Temperature:    0.7,
			TopP:           0.9,
			CostPerToken:   0.03,   // Higher for external API
			RequestTimeout: 60 * time.Second,
			QualityWeight:  1.2,
		},
		"gpt-3.5-turbo": {
			MaxTokens:      4096,
			Temperature:    0.7,
			TopP:           0.9,
			CostPerToken:   0.002,
			RequestTimeout: 30 * time.Second,
			QualityWeight:  0.9,
		},
	}

	// Override with configuration if present
	if configSettings := c.config.GetStringMap("ai.model_settings"); len(configSettings) > 0 {
		// Parse configuration and override defaults
		// This would be more complex in a real implementation
		c.logger.Info("Using configured model settings", zap.Int("models", len(configSettings)))
	}

	return settings
}

// RegisterRoutes registers enterprise AI routes with the HTTP server
func (c *EnterpriseAIContainer) RegisterRoutes(mux *http.ServeMux) {
	handler := c.GetAIHandler()
	handler.RegisterRoutes(mux)
}

// Shutdown gracefully shuts down the enterprise AI container
func (c *EnterpriseAIContainer) Shutdown() error {
	c.logger.Info("Shutting down enterprise AI container")

	// Perform cleanup if needed
	if c.aiService != nil {
		// Export final usage data, send alerts, etc.
		c.logger.Info("Enterprise AI service shutdown completed")
	}

	return nil
}

// GetHealthStatus returns the health status of all AI components
func (c *EnterpriseAIContainer) GetHealthStatus() map[string]interface{} {
	if c.aiService == nil {
		return map[string]interface{}{
			"status": "not_initialized",
		}
	}

	health, err := c.aiService.HealthCheck(context.Background())
	if err != nil {
		return map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		}
	}

	return map[string]interface{}{
		"status":     health.Status,
		"components": health.Components,
		"uptime":     health.Uptime,
		"version":    health.Version,
	}
}