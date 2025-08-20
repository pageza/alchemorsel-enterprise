// Package ai provides AI response caching integration with Redis for Ollama
package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/ai/ollama"
	"github.com/alchemorsel/v3/internal/infrastructure/cache"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"go.uber.org/zap"
)

// CachedAIService wraps AI services with intelligent caching
type CachedAIService struct {
	client    outbound.AIService
	aiCache   *cache.AICacheService
	logger    *zap.Logger
	enabled   bool
}

// NewCachedAIService creates a new cached AI service wrapper
func NewCachedAIService(client outbound.AIService, cacheService *cache.CacheService, logger *zap.Logger) *CachedAIService {
	return &CachedAIService{
		client:  client,
		aiCache: cache.NewAICacheService(cacheService, logger),
		logger:  logger.Named("cached-ai"),
		enabled: true,
	}
}

// GenerateRecipe with intelligent caching
func (c *CachedAIService) GenerateRecipe(ctx context.Context, prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	if !c.enabled {
		return c.client.GenerateRecipe(ctx, prompt, constraints)
	}

	// Use cache-first pattern with fallback
	return c.aiCache.GetRecipeGeneration(ctx, prompt, constraints, func(ctx context.Context, prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
		start := time.Now()
		
		response, err := c.client.GenerateRecipe(ctx, prompt, constraints)
		if err != nil {
			return nil, err
		}
		
		processingTime := time.Since(start)
		
		// Cache the response asynchronously
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			if cacheErr := c.aiCache.CacheRecipeGeneration(cacheCtx, prompt, constraints, response, processingTime); cacheErr != nil {
				c.logger.Error("Failed to cache AI recipe response", zap.Error(cacheErr))
			}
		}()
		
		return response, nil
	})
}

// SuggestIngredients with caching
func (c *CachedAIService) SuggestIngredients(ctx context.Context, partial []string) ([]string, error) {
	if !c.enabled {
		return c.client.SuggestIngredients(ctx, partial)
	}

	return c.aiCache.GetIngredientSuggestions(ctx, partial, func(ctx context.Context, partial []string) ([]string, error) {
		start := time.Now()
		
		suggestions, err := c.client.SuggestIngredients(ctx, partial)
		if err != nil {
			return nil, err
		}
		
		processingTime := time.Since(start)
		
		// Cache the suggestions asynchronously
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if cacheErr := c.aiCache.CacheIngredientSuggestions(cacheCtx, partial, suggestions, processingTime); cacheErr != nil {
				c.logger.Error("Failed to cache ingredient suggestions", zap.Error(cacheErr))
			}
		}()
		
		return suggestions, nil
	})
}

// AnalyzeNutrition with caching
func (c *CachedAIService) AnalyzeNutrition(ctx context.Context, ingredients []string) (*outbound.NutritionInfo, error) {
	if !c.enabled {
		return c.client.AnalyzeNutrition(ctx, ingredients)
	}

	return c.aiCache.GetNutritionAnalysis(ctx, ingredients, func(ctx context.Context, ingredients []string) (*outbound.NutritionInfo, error) {
		start := time.Now()
		
		nutrition, err := c.client.AnalyzeNutrition(ctx, ingredients)
		if err != nil {
			return nil, err
		}
		
		processingTime := time.Since(start)
		
		// Cache the nutrition analysis asynchronously
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if cacheErr := c.aiCache.CacheNutritionAnalysis(cacheCtx, ingredients, nutrition, processingTime); cacheErr != nil {
				c.logger.Error("Failed to cache nutrition analysis", zap.Error(cacheErr))
			}
		}()
		
		return nutrition, nil
	})
}

// Pass-through methods for non-cached operations
func (c *CachedAIService) GenerateDescription(ctx context.Context, recipe *outbound.Recipe) (string, error) {
	return c.client.GenerateDescription(ctx, recipe)
}

func (c *CachedAIService) ClassifyRecipe(ctx context.Context, recipe *outbound.Recipe) (*outbound.RecipeClassification, error) {
	return c.client.ClassifyRecipe(ctx, recipe)
}

// EnableCache enables or disables caching
func (c *CachedAIService) EnableCache(enabled bool) {
	c.enabled = enabled
	c.logger.Info("AI caching configuration changed", zap.Bool("enabled", enabled))
}

// GetCacheStats returns AI caching statistics
func (c *CachedAIService) GetCacheStats(ctx context.Context) (*cache.AICacheStats, error) {
	return c.aiCache.GetAIStats(ctx)
}

// InvalidateCache invalidates all AI cache entries
func (c *CachedAIService) InvalidateCache(ctx context.Context) error {
	models := []string{"recipe_generation", "ingredient_suggestions", "nutrition_analysis"}
	
	for _, model := range models {
		if err := c.aiCache.InvalidateAICache(ctx, model); err != nil {
			c.logger.Error("Failed to invalidate AI cache", 
				zap.String("model", model), 
				zap.Error(err))
			return err
		}
	}
	
	c.logger.Info("AI cache invalidated successfully")
	return nil
}

// WarmupCache pre-warms the cache with common requests
func (c *CachedAIService) WarmupCache(ctx context.Context) error {
	c.logger.Info("Starting AI cache warmup")
	
	// Common recipe prompts for warmup
	warmupPrompts := []struct {
		prompt      string
		constraints outbound.AIConstraints
	}{
		{
			prompt: "pasta with tomatoes",
			constraints: outbound.AIConstraints{
				MaxCalories: 500,
				Dietary:     []string{"vegetarian"},
			},
		},
		{
			prompt: "chicken and vegetables",
			constraints: outbound.AIConstraints{
				CookingTime: 30,
				SkillLevel:  "easy",
			},
		},
		{
			prompt: "healthy salad",
			constraints: outbound.AIConstraints{
				MaxCalories: 300,
				Dietary:     []string{"vegan", "gluten_free"},
			},
		},
	}
	
	// Common ingredient suggestions for warmup
	warmupIngredients := [][]string{
		{"chicken", "garlic"},
		{"pasta", "tomatoes"},
		{"vegetables", "olive oil"},
		{"fish", "lemon"},
	}
	
	var successCount int
	
	// Warmup recipe generation cache
	for i, warmup := range warmupPrompts {
		select {
		case <-ctx.Done():
			c.logger.Info("Cache warmup cancelled", zap.Int("completed", i))
			return ctx.Err()
		default:
		}
		
		_, err := c.GenerateRecipe(ctx, warmup.prompt, warmup.constraints)
		if err != nil {
			c.logger.Warn("Cache warmup failed for recipe", 
				zap.String("prompt", warmup.prompt), 
				zap.Error(err))
		} else {
			successCount++
		}
		
		// Small delay to avoid overwhelming the AI service
		time.Sleep(1 * time.Second)
	}
	
	// Warmup ingredient suggestions cache
	for i, ingredients := range warmupIngredients {
		select {
		case <-ctx.Done():
			c.logger.Info("Cache warmup cancelled", zap.Int("completed", len(warmupPrompts)+i))
			return ctx.Err()
		default:
		}
		
		_, err := c.SuggestIngredients(ctx, ingredients)
		if err != nil {
			c.logger.Warn("Cache warmup failed for ingredients", 
				zap.Strings("ingredients", ingredients), 
				zap.Error(err))
		} else {
			successCount++
		}
		
		time.Sleep(500 * time.Millisecond)
	}
	
	c.logger.Info("AI cache warmup completed", 
		zap.Int("successful_requests", successCount),
		zap.Int("total_requests", len(warmupPrompts)+len(warmupIngredients)))
	
	return nil
}

// OllamaOptimizedService provides Ollama-specific optimizations
type OllamaOptimizedService struct {
	*CachedAIService
	ollamaClient *ollama.Client
	modelCache   map[string]time.Time
	logger       *zap.Logger
}

// NewOllamaOptimizedService creates an optimized service specifically for Ollama
func NewOllamaOptimizedService(ollamaClient *ollama.Client, cacheService *cache.CacheService, logger *zap.Logger) *OllamaOptimizedService {
	cachedService := NewCachedAIService(ollamaClient, cacheService, logger)
	
	return &OllamaOptimizedService{
		CachedAIService: cachedService,
		ollamaClient:    ollamaClient,
		modelCache:      make(map[string]time.Time),
		logger:          logger.Named("ollama-optimized"),
	}
}

// PreloadModel ensures a specific model is loaded in Ollama memory
func (o *OllamaOptimizedService) PreloadModel(ctx context.Context, modelName string) error {
	// Check if model was recently preloaded
	if lastPreload, exists := o.modelCache[modelName]; exists {
		if time.Since(lastPreload) < 5*time.Minute {
			o.logger.Debug("Model recently preloaded, skipping", zap.String("model", modelName))
			return nil
		}
	}
	
	o.logger.Info("Preloading model", zap.String("model", modelName))
	
	// Simple test prompt to load model into memory
	testPrompt := "test"
	_, err := o.ollamaClient.SuggestIngredients(ctx, []string{testPrompt})
	
	if err != nil {
		o.logger.Error("Failed to preload model", 
			zap.String("model", modelName), 
			zap.Error(err))
		return fmt.Errorf("failed to preload model %s: %w", modelName, err)
	}
	
	// Update cache
	o.modelCache[modelName] = time.Now()
	
	o.logger.Info("Model preloaded successfully", zap.String("model", modelName))
	return nil
}

// OptimizeForOllama applies Ollama-specific optimizations
func (o *OllamaOptimizedService) OptimizeForOllama(ctx context.Context) error {
	o.logger.Info("Applying Ollama-specific optimizations")
	
	// Preload primary model
	if err := o.PreloadModel(ctx, "llama3.2:3b"); err != nil {
		o.logger.Warn("Failed to preload primary model", zap.Error(err))
	}
	
	// Warmup cache with smaller, faster prompts optimized for Ollama
	if err := o.WarmupCache(ctx); err != nil {
		o.logger.Warn("Failed to warmup cache", zap.Error(err))
	}
	
	o.logger.Info("Ollama optimizations applied successfully")
	return nil
}

// GetModelStatus returns the status of loaded models
func (o *OllamaOptimizedService) GetModelStatus() map[string]interface{} {
	status := make(map[string]interface{})
	
	for model, lastPreload := range o.modelCache {
		status[model] = map[string]interface{}{
			"last_preload": lastPreload.Format(time.RFC3339),
			"age_minutes":  time.Since(lastPreload).Minutes(),
			"likely_loaded": time.Since(lastPreload) < 10*time.Minute,
		}
	}
	
	return status
}