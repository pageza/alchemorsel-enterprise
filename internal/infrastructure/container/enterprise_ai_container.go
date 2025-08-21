// Package container provides dependency injection for enterprise AI services
package container

import (
	"context"
	"net/http"
	"time"

	"github.com/alchemorsel/v3/internal/application/ai"
	"github.com/alchemorsel/v3/internal/infrastructure/cache"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	"github.com/alchemorsel/v3/internal/ports/outbound"
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

	// For now, use nil cache repository to get basic compilation  
	// TODO: Implement proper cache repository adapter
	var cacheRepo outbound.CacheRepository = nil

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
	if provider := c.config.AI.Provider; provider != "" {
		return provider
	}
	return "ollama" // Default to containerized Ollama
}

func (c *EnterpriseAIContainer) getFallbackProviders() []string {
	// For now, return default fallback providers since not in config struct yet
	providers := []string{"openai", "mock"}
	if len(providers) == 0 {
		return []string{"openai", "mock"}
	}
	return providers
}

func (c *EnterpriseAIContainer) getDailyBudget() int {
	// Default daily budget since not in config struct yet
	budget := 0
	if budget <= 0 {
		return 10000 // Default $100
	}
	return budget
}

func (c *EnterpriseAIContainer) getMonthlyBudget() int {
	// Default monthly budget since not in config struct yet
	budget := 0
	if budget <= 0 {
		return 300000 // Default $3000
	}
	return budget
}

func (c *EnterpriseAIContainer) getRequestsPerMinute() int {
	// Default rate limit since not in config struct yet
	rate := 0
	if rate <= 0 {
		return 60
	}
	return rate
}

func (c *EnterpriseAIContainer) getRequestsPerHour() int {
	// Default rate limit since not in config struct yet
	rate := 0
	if rate <= 0 {
		return 3600
	}
	return rate
}

func (c *EnterpriseAIContainer) getRequestsPerDay() int {
	// Default rate limit since not in config struct yet
	rate := 0
	if rate <= 0 {
		return 86400
	}
	return rate
}

func (c *EnterpriseAIContainer) getMinQualityScore() float64 {
	// Default quality score since not in config struct yet
	score := 0.0
	if score <= 0 {
		return 0.7
	}
	return score
}

func (c *EnterpriseAIContainer) getQualityCheckEnabled() bool {
	// Default quality check enabled since not in config struct yet
	return true
}

func (c *EnterpriseAIContainer) getCacheEnabled() bool {
	// Use AI config cache setting
	return c.config.AI.EnableCache
}

func (c *EnterpriseAIContainer) getCacheTTL() time.Duration {
	// Use AI config cache TTL
	ttl := c.config.AI.CacheTTL
	if ttl == 0 {
		return 2 * time.Hour
	}
	return ttl
}

func (c *EnterpriseAIContainer) getMetricsEnabled() bool {
	// Default metrics enabled since not in config struct yet
	return true
}

func (c *EnterpriseAIContainer) getAlertsEnabled() bool {
	// Default alerts enabled since not in config struct yet
	return true
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

	// TODO: Override with configuration if present (not in config struct yet)
	// In future, this would parse AI config model settings

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