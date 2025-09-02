// Package ai provides a unified AI service that routes between providers
package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"go.uber.org/zap"
)

// ProviderType represents different AI providers
type ProviderType string

const (
	ProviderOllama   ProviderType = "ollama"
	ProviderDeepSeek ProviderType = "deepseek"
	ProviderOpenAI   ProviderType = "openai"
)

// AIProvider represents a generic AI provider interface
type AIProvider interface {
	GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage, temperature float64, maxTokens int) (*conversation.GenerationResult, error)
	TestConnection(ctx context.Context) error
	GetModelInfo() map[string]interface{}
}

// UnifiedAIService manages multiple AI providers with intelligent routing
type UnifiedAIService struct {
	providers        map[ProviderType]AIProvider
	modelManager     *conversation.ModelManager
	responseCache    *ResponseCache
	metricsCollector *MetricsCollector
	config           *config.AIConfig
	logger           *zap.Logger
	defaultProvider  ProviderType
	fallbackProvider ProviderType
}

// NewUnifiedAIService creates a new unified AI service
func NewUnifiedAIService(
	cfg *config.Config,
	modelManager *conversation.ModelManager,
	responseCache *ResponseCache,
	metricsCollector *MetricsCollector,
	logger *zap.Logger,
) (*UnifiedAIService, error) {
	
	service := &UnifiedAIService{
		providers:        make(map[ProviderType]AIProvider),
		modelManager:     modelManager,
		responseCache:    responseCache,
		metricsCollector: metricsCollector,
		config:           &cfg.AI,
		logger:           logger.Named("unified_ai_service"),
		defaultProvider:  ProviderType(cfg.AI.Provider),
		fallbackProvider: ProviderOllama, // Ollama as fallback for cost reasons
	}

	// Initialize providers based on configuration
	if err := service.initializeProviders(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	return service, nil
}

// initializeProviders initializes available AI providers
func (s *UnifiedAIService) initializeProviders(cfg *config.Config) error {
	// Initialize Ollama provider (always available for testing)
	if cfg.AI.OllamaHost != "" {
		ollamaClient := NewOllamaClient(cfg.AI.OllamaHost, cfg.AI.OllamaModel)
		s.providers[ProviderOllama] = ollamaClient
		s.logger.Info("Ollama provider initialized", zap.String("host", cfg.AI.OllamaHost))
	}

	// Initialize DeepSeek provider if API key is provided
	if cfg.AI.DeepSeekAPIKey != "" {
		deepSeekClient := NewDeepSeekClient(
			cfg.AI.DeepSeekAPIKey,
			cfg.AI.DeepSeekBaseURL,
			cfg.AI.DeepSeekModel,
			cfg.AI.DeepSeekTimeout,
			s.logger,
		)
		s.providers[ProviderDeepSeek] = deepSeekClient
		s.logger.Info("DeepSeek provider initialized", 
			zap.String("base_url", cfg.AI.DeepSeekBaseURL),
			zap.String("model", cfg.AI.DeepSeekModel))
	}

	if len(s.providers) == 0 {
		return fmt.Errorf("no AI providers available - check configuration")
	}

	s.logger.Info("Unified AI service initialized", 
		zap.Int("providers", len(s.providers)),
		zap.String("default", string(s.defaultProvider)))

	return nil
}

// GenerateChatCompletion generates a chat completion using the best available provider
func (s *UnifiedAIService) GenerateChatCompletion(
	ctx context.Context,
	messages []conversation.ChatMessage,
	options conversation.GenerationOptions,
) (*conversation.GenerationResult, error) {
	startTime := time.Now()

	// Check cache first if enabled
	if s.responseCache != nil && s.config.EnableCache {
		if cachedResult := s.responseCache.Get(ctx, messages, s.getSelectedModel(options)); cachedResult != nil {
			if s.metricsCollector != nil {
				s.metricsCollector.RecordCacheHit(cachedResult.ModelUsed)
			}
			s.logger.Debug("Cache hit for messages", zap.Int("message_count", len(messages)))
			return cachedResult, nil
		}
		if s.metricsCollector != nil {
			s.metricsCollector.RecordCacheMiss("unknown")
		}
	}

	// Select provider based on model or default to configured provider
	provider, modelName, err := s.selectProvider(options)
	if err != nil {
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}

	// Generate completion
	result, err := provider.GenerateChatCompletion(
		ctx,
		messages,
		options.Temperature,
		options.MaxTokens,
	)
	if err != nil {
		// Try fallback provider if primary fails
		if provider != s.providers[s.fallbackProvider] && s.providers[s.fallbackProvider] != nil {
			s.logger.Warn("Primary provider failed, trying fallback",
				zap.String("primary", string(s.getProviderType(provider))),
				zap.String("fallback", string(s.fallbackProvider)),
				zap.Error(err))

			fallbackResult, fallbackErr := s.providers[s.fallbackProvider].GenerateChatCompletion(
				ctx,
				messages,
				options.Temperature,
				options.MaxTokens,
			)
			if fallbackErr == nil {
				result = fallbackResult
				err = nil
				modelName = "fallback-" + result.ModelUsed
			}
		}

		if err != nil {
			if s.metricsCollector != nil {
				s.metricsCollector.RecordGenerationWithIntent(
					modelName,
					string(options.Intent),
					string(options.Quality),
					time.Since(startTime),
					0,
					0,
					err,
				)
			}
			return nil, fmt.Errorf("all providers failed: %w", err)
		}
	}

	// Update result with correct model name
	if result.ModelUsed == "" {
		result.ModelUsed = modelName
	}

	// Record metrics
	if s.metricsCollector != nil {
		s.metricsCollector.RecordGenerationWithIntent(
			result.ModelUsed,
			string(options.Intent),
			string(options.Quality),
			result.Duration,
			result.TokensUsed,
			result.Quality,
			nil,
		)
	}

	// Update model manager with performance metrics
	if s.modelManager != nil {
		s.modelManager.UpdateModelMetrics(
			result.ModelUsed,
			result.Duration,
			result.TokensUsed,
			result.Quality,
		)
	}

	// Cache result if enabled
	if s.responseCache != nil && s.config.EnableCache && result.Quality >= s.config.QualityThreshold {
		cacheTTL := s.config.CacheTTL
		if cacheTTL == 0 {
			cacheTTL = time.Hour // Default cache TTL
		}
		if cacheErr := s.responseCache.Set(ctx, messages, result.ModelUsed, result, cacheTTL); cacheErr != nil {
			s.logger.Warn("Failed to cache response", zap.Error(cacheErr))
		}
	}

	s.logger.Debug("Chat completion generated",
		zap.String("model", result.ModelUsed),
		zap.Duration("duration", result.Duration),
		zap.Int("tokens", result.TokensUsed),
		zap.Float64("quality", result.Quality))

	return result, nil
}

// selectProvider selects the best provider and model for the request
func (s *UnifiedAIService) selectProvider(options conversation.GenerationOptions) (AIProvider, string, error) {
	// If DeepSeek is configured and we want fast responses, prefer it
	if s.providers[ProviderDeepSeek] != nil {
		if options.Quality == conversation.QualityFast || s.config.Provider == "deepseek" {
			modelName := s.selectDeepSeekModel(options)
			return s.providers[ProviderDeepSeek], modelName, nil
		}
	}

	// Use model manager to select optimal model
	if s.modelManager != nil {
		modelName, err := s.modelManager.SelectOptimalModel(options.Intent, options.Context)
		if err == nil {
			// Check if this is a DeepSeek model
			if strings.HasPrefix(modelName, "deepseek-") && s.providers[ProviderDeepSeek] != nil {
				return s.providers[ProviderDeepSeek], modelName, nil
			}
			// Otherwise use Ollama
			if s.providers[ProviderOllama] != nil {
				return s.providers[ProviderOllama], modelName, nil
			}
		}
	}

	// Fall back to default provider
	if provider, exists := s.providers[s.defaultProvider]; exists {
		modelName := s.getDefaultModel(s.defaultProvider)
		return provider, modelName, nil
	}

	// Use any available provider
	for providerType, provider := range s.providers {
		modelName := s.getDefaultModel(providerType)
		return provider, modelName, nil
	}

	return nil, "", fmt.Errorf("no providers available")
}

// selectDeepSeekModel selects appropriate DeepSeek model based on intent
func (s *UnifiedAIService) selectDeepSeekModel(options conversation.GenerationOptions) string {
	switch options.Intent {
	case conversation.IntentRecipeCreation:
		return "deepseek-recipe"
	case conversation.IntentCookingHelp:
		return "deepseek-help"
	default:
		return "deepseek-chat"
	}
}

// getDefaultModel returns default model for a provider
func (s *UnifiedAIService) getDefaultModel(provider ProviderType) string {
	switch provider {
	case ProviderDeepSeek:
		return s.config.DeepSeekModel
	case ProviderOllama:
		return s.config.OllamaModel
	default:
		return "unknown"
	}
}

// getSelectedModel returns the model name that would be selected for caching
func (s *UnifiedAIService) getSelectedModel(options conversation.GenerationOptions) string {
	_, model, _ := s.selectProvider(options)
	return model
}

// getProviderType returns the provider type for a given provider instance
func (s *UnifiedAIService) getProviderType(provider AIProvider) ProviderType {
	for providerType, p := range s.providers {
		if p == provider {
			return providerType
		}
	}
	return ProviderType("unknown")
}

// TestConnections tests connectivity to all configured providers
func (s *UnifiedAIService) TestConnections(ctx context.Context) map[ProviderType]error {
	results := make(map[ProviderType]error)
	
	for providerType, provider := range s.providers {
		if err := provider.TestConnection(ctx); err != nil {
			results[providerType] = err
			s.logger.Warn("Provider connection test failed",
				zap.String("provider", string(providerType)),
				zap.Error(err))
		} else {
			results[providerType] = nil
			s.logger.Info("Provider connection test passed",
				zap.String("provider", string(providerType)))
		}
	}
	
	return results
}

// GetProviderInfo returns information about all configured providers
func (s *UnifiedAIService) GetProviderInfo() map[ProviderType]map[string]interface{} {
	info := make(map[ProviderType]map[string]interface{})
	
	for providerType, provider := range s.providers {
		info[providerType] = provider.GetModelInfo()
	}
	
	return info
}

// GetHealthStatus returns health status of all providers
func (s *UnifiedAIService) GetHealthStatus(ctx context.Context) map[ProviderType]string {
	status := make(map[ProviderType]string)
	
	for providerType, provider := range s.providers {
		if err := provider.TestConnection(ctx); err != nil {
			status[providerType] = "unhealthy"
		} else {
			status[providerType] = "healthy"
		}
	}
	
	return status
}