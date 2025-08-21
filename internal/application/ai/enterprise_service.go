// Package ai provides enterprise-grade AI services with comprehensive cost tracking
package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/infrastructure/ai/ollama"
	"github.com/alchemorsel/v3/internal/infrastructure/ai/openai"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EnterpriseAIService provides production-ready AI features with comprehensive cost tracking
type EnterpriseAIService struct {
	// Core components
	provider         string
	client           outbound.AIService
	ollamaClient     *ollama.Client
	openaiClient     *openai.Client
	cacheRepo        outbound.CacheRepository
	logger           *zap.Logger

	// Cost tracking components
	costTracker      *CostTracker
	usageAnalytics   *UsageAnalytics
	rateLimiter      *RateLimiter
	qualityMonitor   *QualityMonitor
	alertManager     *AlertManager

	// Configuration
	config           *EnterpriseConfig
	
	// Thread safety
	mu               sync.RWMutex
	requestCounter   int64
}

// EnterpriseConfig holds enterprise AI service configuration
type EnterpriseConfig struct {
	// Provider settings
	PrimaryProvider   string
	FallbackProviders []string
	ModelSettings     map[string]ModelConfig
	
	// Cost management
	DailyBudgetCents     int
	MonthlyBudgetCents   int
	CostAlertThresholds  []float64
	
	// Rate limiting
	RequestsPerMinute    int
	RequestsPerHour      int
	RequestsPerDay       int
	
	// Quality settings
	MinQualityScore      float64
	QualityCheckEnabled  bool
	
	// Features
	CacheEnabled         bool
	CacheTTL             time.Duration
	MetricsEnabled       bool
	AlertsEnabled        bool
}

// ModelConfig holds configuration for specific AI models
type ModelConfig struct {
	MaxTokens        int
	Temperature      float64
	TopP             float64
	CostPerToken     float64 // Cost in cents per token
	RequestTimeout   time.Duration
	QualityWeight    float64
}

// NewEnterpriseAIService creates a new enterprise AI service
func NewEnterpriseAIService(
	provider string,
	cacheRepo outbound.CacheRepository,
	config *EnterpriseConfig,
	logger *zap.Logger,
) *EnterpriseAIService {
	namedLogger := logger.Named("enterprise-ai-service")
	
	// Create clients for all supported providers
	ollamaClient := ollama.NewClient(namedLogger)
	openaiClient := openai.NewClient(namedLogger)
	
	// Set default config if not provided
	if config == nil {
		config = &EnterpriseConfig{
			PrimaryProvider:      "ollama",
			FallbackProviders:    []string{"openai"},
			DailyBudgetCents:     10000,   // $100
			MonthlyBudgetCents:   300000,  // $3000
			CostAlertThresholds:  []float64{0.7, 0.9, 1.0},
			RequestsPerMinute:    60,
			RequestsPerHour:      3600,
			RequestsPerDay:       86400,
			MinQualityScore:      0.7,
			QualityCheckEnabled:  true,
			CacheEnabled:         true,
			CacheTTL:             2 * time.Hour,
			MetricsEnabled:       true,
			AlertsEnabled:        true,
			ModelSettings: map[string]ModelConfig{
				"llama3.2:3b": {
					MaxTokens:      2048,
					Temperature:    0.7,
					TopP:           0.9,
					CostPerToken:   0.001, // $0.001 per token
					RequestTimeout: 30 * time.Second,
					QualityWeight:  1.0,
				},
				"gpt-4": {
					MaxTokens:      4096,
					Temperature:    0.7,
					TopP:           0.9,
					CostPerToken:   0.03, // $0.03 per token
					RequestTimeout: 60 * time.Second,
					QualityWeight:  1.2,
				},
			},
		}
	}
	
	// Determine the active provider
	if provider == "" {
		provider = config.PrimaryProvider
	}
	
	// Select primary client based on provider
	var activeClient outbound.AIService
	switch provider {
	case "ollama":
		activeClient = ollamaClient
	case "openai":
		activeClient = openaiClient
	default:
		namedLogger.Warn("Unknown AI provider, defaulting to Ollama", zap.String("provider", provider))
		activeClient = ollamaClient
		provider = "ollama"
	}
	
	service := &EnterpriseAIService{
		provider:       provider,
		client:         activeClient,
		ollamaClient:   ollamaClient,
		openaiClient:   openaiClient,
		cacheRepo:      cacheRepo,
		logger:         namedLogger,
		config:         config,
		requestCounter: 0,
	}
	
	// Initialize enterprise components
	service.costTracker = NewCostTracker(config, namedLogger)
	service.usageAnalytics = NewUsageAnalytics(cacheRepo, namedLogger)
	service.rateLimiter = NewRateLimiter(config, cacheRepo, namedLogger)
	service.qualityMonitor = NewQualityMonitor(config, namedLogger)
	service.alertManager = NewAlertManager(config, namedLogger)
	
	namedLogger.Info("Enterprise AI service initialized",
		zap.String("primary_provider", provider),
		zap.Strings("fallback_providers", config.FallbackProviders),
		zap.Int("daily_budget_cents", config.DailyBudgetCents),
		zap.Bool("cache_enabled", config.CacheEnabled),
		zap.Bool("quality_monitoring", config.QualityCheckEnabled),
	)
	
	return service
}

// GenerateRecipe generates a recipe with enterprise features
func (s *EnterpriseAIService) GenerateRecipe(ctx context.Context, prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	s.mu.Lock()
	requestID := s.requestCounter
	s.requestCounter++
	s.mu.Unlock()
	
	startTime := time.Now()
	
	s.logger.Info("Starting enterprise recipe generation",
		zap.Int64("request_id", requestID),
		zap.String("prompt", prompt),
		zap.String("provider", s.provider),
	)
	
	// Check rate limits
	userID, _ := s.extractUserIDFromContext(ctx)
	if err := s.rateLimiter.CheckLimits(ctx, userID); err != nil {
		s.logger.Warn("Rate limit exceeded", zap.Error(err), zap.String("user_id", userID.String()))
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Check budget limits
	if err := s.costTracker.CheckBudgetLimits(ctx); err != nil {
		s.logger.Warn("Budget limit exceeded", zap.Error(err))
		return nil, fmt.Errorf("budget limit exceeded: %w", err)
	}
	
	// Try cache first
	cacheKey := s.buildCacheKey("recipe", prompt, constraints)
	if s.config.CacheEnabled {
		if cached, err := s.getCachedResponse(ctx, cacheKey); err == nil {
			s.logger.Info("Recipe served from cache",
				zap.Int64("request_id", requestID),
				zap.Duration("response_time", time.Since(startTime)),
			)
			
			// Track cache hit
			s.usageAnalytics.TrackCacheHit(ctx, "recipe_generation")
			return cached, nil
		}
	}
	
	// Generate recipe with fallback support
	response, err := s.generateRecipeWithFallback(ctx, prompt, constraints)
	if err != nil {
		s.logger.Error("Recipe generation failed", zap.Error(err), zap.Int64("request_id", requestID))
		s.usageAnalytics.TrackError(ctx, "recipe_generation", err.Error())
		return nil, err
	}
	
	// Quality assessment
	qualityScore := s.qualityMonitor.AssessRecipeQuality(response)
	if s.config.QualityCheckEnabled && qualityScore < s.config.MinQualityScore {
		s.logger.Warn("Recipe quality below threshold",
			zap.Float64("quality_score", qualityScore),
			zap.Float64("min_threshold", s.config.MinQualityScore),
		)
		
		// Try to regenerate once with different parameters
		if retryResponse, retryErr := s.generateRecipeWithFallback(ctx, prompt+" (high quality required)", constraints); retryErr == nil {
			retryQuality := s.qualityMonitor.AssessRecipeQuality(retryResponse)
			if retryQuality > qualityScore {
				response = retryResponse
				qualityScore = retryQuality
			}
		}
	}
	
	// Cache successful response
	if s.config.CacheEnabled && err == nil {
		s.cacheResponse(ctx, cacheKey, response)
	}
	
	// Track usage and costs
	duration := time.Since(startTime)
	s.usageAnalytics.TrackRequest(ctx, "recipe_generation", duration, len(response.Instructions))
	
	// Calculate and track costs (estimated)
	estimatedTokens := s.estimateTokenUsage(prompt, response)
	cost := s.costTracker.CalculateCost(s.provider, "recipe_generation", estimatedTokens)
	s.costTracker.TrackUsage(ctx, userID, cost, estimatedTokens)
	
	// Check for cost alerts
	s.alertManager.CheckCostAlerts(ctx, s.costTracker.GetDailySpend(), s.costTracker.GetMonthlySpend())
	
	s.logger.Info("Recipe generation completed",
		zap.Int64("request_id", requestID),
		zap.Duration("response_time", duration),
		zap.Float64("quality_score", qualityScore),
		zap.Int("estimated_tokens", estimatedTokens),
		zap.Float64("estimated_cost_cents", cost),
	)
	
	return response, nil
}

// GenerateIngredientSuggestions provides smart ingredient recommendations
func (s *EnterpriseAIService) GenerateIngredientSuggestions(ctx context.Context, partial []string, cuisine string, dietary []string) ([]string, error) {
	startTime := time.Now()
	
	// Check rate limits
	userID, _ := s.extractUserIDFromContext(ctx)
	if err := s.rateLimiter.CheckLimits(ctx, userID); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Build enhanced prompt
	prompt := s.buildIngredientSuggestionPrompt(partial, cuisine, dietary)
	
	// Try cache first
	cacheKey := s.buildCacheKey("ingredients", prompt)
	if s.config.CacheEnabled {
		if cached, err := s.getCachedStringSlice(ctx, cacheKey); err == nil {
			s.usageAnalytics.TrackCacheHit(ctx, "ingredient_suggestions")
			return cached, nil
		}
	}
	
	// Generate suggestions
	suggestions, err := s.client.SuggestIngredients(ctx, partial)
	if err != nil {
		// Fallback with enhanced logic
		suggestions = s.generateSmartIngredientFallback(partial, cuisine, dietary)
	}
	
	// Cache result
	if s.config.CacheEnabled {
		s.cacheStringSlice(ctx, cacheKey, suggestions)
	}
	
	// Track usage
	duration := time.Since(startTime)
	s.usageAnalytics.TrackRequest(ctx, "ingredient_suggestions", duration, len(suggestions))
	
	return suggestions, nil
}

// AnalyzeNutritionalContent provides comprehensive nutritional analysis
func (s *EnterpriseAIService) AnalyzeNutritionalContent(ctx context.Context, ingredients []string, servings int) (*outbound.NutritionInfo, error) {
	startTime := time.Now()
	
	// Check rate limits
	userID, _ := s.extractUserIDFromContext(ctx)
	if err := s.rateLimiter.CheckLimits(ctx, userID); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Build cache key
	cacheKey := s.buildCacheKey("nutrition", strings.Join(ingredients, ","), fmt.Sprintf("servings:%d", servings))
	
	// Try cache first
	if s.config.CacheEnabled {
		if cached, err := s.getCachedNutrition(ctx, cacheKey); err == nil {
			s.usageAnalytics.TrackCacheHit(ctx, "nutrition_analysis")
			return cached, nil
		}
	}
	
	// Analyze nutrition
	nutrition, err := s.client.AnalyzeNutrition(ctx, ingredients)
	if err != nil {
		// Enhanced fallback with better nutrition estimation
		nutrition = s.generateEnhancedNutritionFallback(ingredients, servings)
	}
	
	// Adjust for servings
	if servings > 0 && servings != 1 {
		s.adjustNutritionForServings(nutrition, servings)
	}
	
	// Cache result
	if s.config.CacheEnabled {
		s.cacheNutrition(ctx, cacheKey, nutrition)
	}
	
	// Track usage
	duration := time.Since(startTime)
	s.usageAnalytics.TrackRequest(ctx, "nutrition_analysis", duration, len(ingredients))
	
	return nutrition, nil
}

// OptimizeRecipe optimizes an existing recipe for health, cost, or taste
func (s *EnterpriseAIService) OptimizeRecipe(ctx context.Context, rec *recipe.Recipe, optimizationType string) (*outbound.AIRecipeResponse, error) {
	startTime := time.Now()
	
	// Check rate limits and budget
	userID, _ := s.extractUserIDFromContext(ctx)
	if err := s.rateLimiter.CheckLimits(ctx, userID); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	if err := s.costTracker.CheckBudgetLimits(ctx); err != nil {
		return nil, fmt.Errorf("budget limit exceeded: %w", err)
	}
	
	// Build optimization prompt
	prompt := s.buildOptimizationPrompt(rec, optimizationType)
	
	// Try cache first
	cacheKey := s.buildCacheKey("optimize", rec.ID().String(), optimizationType)
	if s.config.CacheEnabled {
		if cached, err := s.getCachedResponse(ctx, cacheKey); err == nil {
			s.usageAnalytics.TrackCacheHit(ctx, "recipe_optimization")
			return cached, nil
		}
	}
	
	// Generate optimized recipe
	constraints := outbound.AIConstraints{
		MaxCalories: 800, // Default reasonable limit
		Dietary:     []string{}, // Will be enhanced based on optimization type
	}
	
	response, err := s.generateRecipeWithFallback(ctx, prompt, constraints)
	if err != nil {
		s.usageAnalytics.TrackError(ctx, "recipe_optimization", err.Error())
		return nil, err
	}
	
	// Quality assessment for optimization
	qualityScore := s.qualityMonitor.AssessOptimizationQuality(response, optimizationType)
	
	// Cache result
	if s.config.CacheEnabled {
		s.cacheResponse(ctx, cacheKey, response)
	}
	
	// Track usage and costs
	duration := time.Since(startTime)
	estimatedTokens := s.estimateTokenUsage(prompt, response)
	cost := s.costTracker.CalculateCost(s.provider, "recipe_optimization", estimatedTokens)
	
	s.usageAnalytics.TrackRequest(ctx, "recipe_optimization", duration, len(response.Instructions))
	s.costTracker.TrackUsage(ctx, userID, cost, estimatedTokens)
	
	s.logger.Info("Recipe optimization completed",
		zap.String("recipe_id", rec.ID().String()),
		zap.String("optimization_type", optimizationType),
		zap.Duration("response_time", duration),
		zap.Float64("quality_score", qualityScore),
	)
	
	return response, nil
}

// AdaptRecipeForDiet adapts a recipe for specific dietary restrictions
func (s *EnterpriseAIService) AdaptRecipeForDiet(ctx context.Context, rec *recipe.Recipe, dietaryRestrictions []string) (*outbound.AIRecipeResponse, error) {
	startTime := time.Now()
	
	// Check rate limits
	userID, _ := s.extractUserIDFromContext(ctx)
	if err := s.rateLimiter.CheckLimits(ctx, userID); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Build dietary adaptation prompt
	prompt := s.buildDietaryAdaptationPrompt(rec, dietaryRestrictions)
	
	// Try cache first
	cacheKey := s.buildCacheKey("dietary_adapt", rec.ID().String(), strings.Join(dietaryRestrictions, ","))
	if s.config.CacheEnabled {
		if cached, err := s.getCachedResponse(ctx, cacheKey); err == nil {
			s.usageAnalytics.TrackCacheHit(ctx, "dietary_adaptation")
			return cached, nil
		}
	}
	
	// Generate adapted recipe
	constraints := outbound.AIConstraints{
		Dietary: dietaryRestrictions,
	}
	
	response, err := s.generateRecipeWithFallback(ctx, prompt, constraints)
	if err != nil {
		s.usageAnalytics.TrackError(ctx, "dietary_adaptation", err.Error())
		return nil, err
	}
	
	// Ensure dietary compliance
	s.validateDietaryCompliance(response, dietaryRestrictions)
	
	// Cache result
	if s.config.CacheEnabled {
		s.cacheResponse(ctx, cacheKey, response)
	}
	
	// Track usage
	duration := time.Since(startTime)
	s.usageAnalytics.TrackRequest(ctx, "dietary_adaptation", duration, len(response.Instructions))
	
	return response, nil
}

// GenerateMealPlan creates a comprehensive meal plan
func (s *EnterpriseAIService) GenerateMealPlan(ctx context.Context, days int, dietary []string, budget float64) (*MealPlanResponse, error) {
	startTime := time.Now()
	
	// Check rate limits and budget
	userID, _ := s.extractUserIDFromContext(ctx)
	if err := s.rateLimiter.CheckLimits(ctx, userID); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}
	
	// Try cache first
	cacheKey := s.buildCacheKey("meal_plan", fmt.Sprintf("days:%d", days), strings.Join(dietary, ","), fmt.Sprintf("budget:%.2f", budget))
	if s.config.CacheEnabled {
		if cached, err := s.getCachedMealPlan(ctx, cacheKey); err == nil {
			s.usageAnalytics.TrackCacheHit(ctx, "meal_planning")
			return cached, nil
		}
	}
	
	// Generate meal plan
	mealPlan := s.generateMealPlanFallback(days, dietary, budget)
	
	// Cache result
	if s.config.CacheEnabled {
		s.cacheMealPlan(ctx, cacheKey, mealPlan)
	}
	
	// Track usage
	duration := time.Since(startTime)
	s.usageAnalytics.TrackRequest(ctx, "meal_planning", duration, days)
	
	return mealPlan, nil
}

// GetUsageAnalytics returns usage analytics for the specified period
func (s *EnterpriseAIService) GetUsageAnalytics(ctx context.Context, period string) (*UsageReport, error) {
	return s.usageAnalytics.GenerateReport(ctx, period)
}

// GetCostAnalytics returns cost analytics and spending breakdown
func (s *EnterpriseAIService) GetCostAnalytics(ctx context.Context, period string) (*CostReport, error) {
	return s.costTracker.GenerateReport(ctx, period)
}

// GetQualityMetrics returns quality assessment metrics
func (s *EnterpriseAIService) GetQualityMetrics(ctx context.Context, period string) (*QualityReport, error) {
	return s.qualityMonitor.GetQualityReport(period), nil
}

// GetRateLimitStatus returns current rate limit status for a user
func (s *EnterpriseAIService) GetRateLimitStatus(ctx context.Context, userID uuid.UUID) (*RateLimitStatus, error) {
	return s.rateLimiter.GetStatus(ctx, userID)
}

// UpdateConfiguration updates the service configuration
func (s *EnterpriseAIService) UpdateConfiguration(ctx context.Context, newConfig *EnterpriseConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.config = newConfig
	
	// Update component configurations
	s.costTracker.UpdateConfig(newConfig)
	s.rateLimiter.UpdateConfig(newConfig)
	s.qualityMonitor.UpdateConfig(newConfig)
	s.alertManager.UpdateConfig(newConfig)
	
	s.logger.Info("Enterprise AI configuration updated")
	return nil
}

// Health check for enterprise AI service
func (s *EnterpriseAIService) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	status := &HealthStatus{
		ServiceName:    "enterprise-ai-service",
		Status:         "healthy",
		Timestamp:      time.Now(),
		Components:     make(map[string]ComponentHealth),
	}
	
	// Check each component
	status.Components["cost_tracker"] = s.costTracker.HealthCheck()
	status.Components["usage_analytics"] = s.usageAnalytics.HealthCheck()
	status.Components["rate_limiter"] = s.rateLimiter.HealthCheck()
	status.Components["quality_monitor"] = s.qualityMonitor.HealthCheck()
	status.Components["alert_manager"] = s.alertManager.HealthCheck()
	
	// Check AI providers
	if err := s.checkProviderHealth(ctx); err != nil {
		status.Status = "degraded"
		status.Components["ai_providers"] = ComponentHealth{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	} else {
		status.Components["ai_providers"] = ComponentHealth{
			Status:  "healthy",
			Message: "All providers operational",
		}
	}
	
	return status, nil
}

// Helper methods will be implemented in the next file
// Due to file size constraints, continuing in enterprise_service_helpers.go