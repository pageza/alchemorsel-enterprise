// Package cache provides AI response caching for optimized performance and cost reduction
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/ports/outbound"
	"go.uber.org/zap"
)

// AICacheService provides comprehensive AI response caching with intelligent cache-first pattern
type AICacheService struct {
	cache      *CacheService
	keyBuilder *KeyBuilder
	config     *AICacheConfig
	logger     *zap.Logger
}

// AICacheConfig configures AI response caching behavior
type AICacheConfig struct {
	// TTL configurations for different AI operations
	RecipeGenerationTTL time.Duration `json:"recipe_generation_ttl"`
	IngredientSuggestionTTL time.Duration `json:"ingredient_suggestion_ttl"`
	NutritionAnalysisTTL time.Duration `json:"nutrition_analysis_ttl"`
	DescriptionGenerationTTL time.Duration `json:"description_generation_ttl"`
	ClassificationTTL time.Duration `json:"classification_ttl"`
	
	// Cache behavior
	CacheByPromptHash   bool          `json:"cache_by_prompt_hash"`
	CacheByParameters   bool          `json:"cache_by_parameters"`
	CompressionEnabled  bool          `json:"compression_enabled"`
	MaxPromptLength     int           `json:"max_prompt_length"`
	MaxResponseSize     int64         `json:"max_response_size"`
	
	// Performance optimizations
	BatchCaching        bool          `json:"batch_caching"`
	PreWarmPopular      bool          `json:"pre_warm_popular"`
	AdaptiveTTL         bool          `json:"adaptive_ttl"`
	PopularPromptBonus  time.Duration `json:"popular_prompt_bonus"`
	
	// Quality and safety
	ValidateResponses   bool          `json:"validate_responses"`
	FilterUnsafeContent bool          `json:"filter_unsafe_content"`
	VersionedCaching    bool          `json:"versioned_caching"`
	ModelVersion        string        `json:"model_version"`
}

// CachedAIResponse represents a cached AI response with metadata
type CachedAIResponse struct {
	Model        string                 `json:"model"`
	Prompt       string                 `json:"prompt,omitempty"` // Optional for sensitive prompts
	PromptHash   string                 `json:"prompt_hash"`
	Parameters   map[string]interface{} `json:"parameters"`
	Response     interface{}            `json:"response"`
	Confidence   float64                `json:"confidence,omitempty"`
	TokensUsed   int                    `json:"tokens_used,omitempty"`
	ProcessingTime time.Duration        `json:"processing_time"`
	CachedAt     time.Time              `json:"cached_at"`
	AccessCount  int64                  `json:"access_count"`
	LastAccess   time.Time              `json:"last_access"`
	ModelVersion string                 `json:"model_version"`
	Tags         []string               `json:"tags,omitempty"`
}

// AIPromptStats tracks prompt usage statistics
type AIPromptStats struct {
	PromptHash    string    `json:"prompt_hash"`
	UsageCount    int64     `json:"usage_count"`
	LastUsed      time.Time `json:"last_used"`
	AvgConfidence float64   `json:"avg_confidence"`
	SuccessRate   float64   `json:"success_rate"`
}

// NewAICacheService creates a new AI cache service
func NewAICacheService(cache *CacheService, logger *zap.Logger) *AICacheService {
	config := DefaultAICacheConfig()
	
	return &AICacheService{
		cache:      cache,
		keyBuilder: NewKeyBuilder(),
		config:     config,
		logger:     logger,
	}
}

// CacheRecipeGeneration caches AI recipe generation results
func (acs *AICacheService) CacheRecipeGeneration(ctx context.Context, prompt string, constraints outbound.AIConstraints, response *outbound.AIRecipeResponse, processingTime time.Duration) error {
	if response == nil {
		return fmt.Errorf("cannot cache nil AI response")
	}
	
	cacheKey := acs.buildRecipeGenerationKey(prompt, constraints)
	
	cached := CachedAIResponse{
		Model:          "recipe_generation",
		PromptHash:     acs.hashPrompt(prompt),
		Parameters:     acs.constraintsToParams(constraints),
		Response:       response,
		Confidence:     response.Confidence,
		ProcessingTime: processingTime,
		CachedAt:       time.Now(),
		AccessCount:    0,
		LastAccess:     time.Now(),
		ModelVersion:   acs.config.ModelVersion,
		Tags:           []string{"recipe", "generation", "ai"},
	}
	
	// Store sensitive prompts as hash only
	if !acs.shouldStoreFullPrompt(prompt) {
		cached.Prompt = ""
	} else {
		cached.Prompt = prompt
	}
	
	return acs.cacheAIResponse(ctx, cacheKey, &cached, acs.config.RecipeGenerationTTL)
}

// GetRecipeGeneration retrieves cached recipe generation or calls fallback
func (acs *AICacheService) GetRecipeGeneration(ctx context.Context, prompt string, constraints outbound.AIConstraints, fallback func(context.Context, string, outbound.AIConstraints) (*outbound.AIRecipeResponse, error)) (*outbound.AIRecipeResponse, error) {
	cacheKey := acs.buildRecipeGenerationKey(prompt, constraints)
	
	// Try cache first
	if cached, err := acs.getAIResponse(ctx, cacheKey); err == nil {
		if response, ok := cached.Response.(*outbound.AIRecipeResponse); ok {
			acs.updateAccessStats(ctx, cacheKey, cached)
			
			acs.logger.Debug("AI recipe generation cache hit",
				zap.String("prompt_hash", cached.PromptHash),
				zap.Int64("access_count", cached.AccessCount))
			
			return response, nil
		}
		
		// Invalid response format, remove from cache
		acs.cache.Delete(ctx, cacheKey)
	}
	
	// Cache miss - use fallback
	if fallback == nil {
		return nil, fmt.Errorf("AI response not found in cache and no fallback provided")
	}
	
	start := time.Now()
	response, err := fallback(ctx, prompt, constraints)
	processingTime := time.Since(start)
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	if err := acs.CacheRecipeGeneration(ctx, prompt, constraints, response, processingTime); err != nil {
		acs.logger.Error("Failed to cache AI recipe generation", zap.Error(err))
	}
	
	// Track prompt usage
	go acs.trackPromptUsage(ctx, prompt, response.Confidence)
	
	return response, nil
}

// CacheIngredientSuggestions caches ingredient suggestions
func (acs *AICacheService) CacheIngredientSuggestions(ctx context.Context, partialIngredients []string, suggestions []string, processingTime time.Duration) error {
	cacheKey := acs.buildIngredientSuggestionsKey(partialIngredients)
	
	cached := CachedAIResponse{
		Model:          "ingredient_suggestions",
		PromptHash:     acs.hashStringSlice(partialIngredients),
		Parameters:     map[string]interface{}{"partial": partialIngredients},
		Response:       suggestions,
		ProcessingTime: processingTime,
		CachedAt:       time.Now(),
		AccessCount:    0,
		LastAccess:     time.Now(),
		ModelVersion:   acs.config.ModelVersion,
		Tags:           []string{"ingredients", "suggestions", "ai"},
	}
	
	return acs.cacheAIResponse(ctx, cacheKey, &cached, acs.config.IngredientSuggestionTTL)
}

// GetIngredientSuggestions retrieves cached suggestions or calls fallback
func (acs *AICacheService) GetIngredientSuggestions(ctx context.Context, partialIngredients []string, fallback func(context.Context, []string) ([]string, error)) ([]string, error) {
	cacheKey := acs.buildIngredientSuggestionsKey(partialIngredients)
	
	// Try cache first
	if cached, err := acs.getAIResponse(ctx, cacheKey); err == nil {
		if suggestions, ok := cached.Response.([]string); ok {
			acs.updateAccessStats(ctx, cacheKey, cached)
			
			acs.logger.Debug("AI ingredient suggestions cache hit",
				zap.Strings("partial", partialIngredients),
				zap.Int("suggestions", len(suggestions)))
			
			return suggestions, nil
		}
		
		// Try interface slice conversion
		if rawSuggestions, ok := cached.Response.([]interface{}); ok {
			suggestions := make([]string, len(rawSuggestions))
			for i, v := range rawSuggestions {
				if str, ok := v.(string); ok {
					suggestions[i] = str
				}
			}
			return suggestions, nil
		}
	}
	
	// Cache miss - use fallback
	if fallback == nil {
		return nil, fmt.Errorf("ingredient suggestions not found in cache and no fallback provided")
	}
	
	start := time.Now()
	suggestions, err := fallback(ctx, partialIngredients)
	processingTime := time.Since(start)
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	if err := acs.CacheIngredientSuggestions(ctx, partialIngredients, suggestions, processingTime); err != nil {
		acs.logger.Error("Failed to cache ingredient suggestions", zap.Error(err))
	}
	
	return suggestions, nil
}

// CacheNutritionAnalysis caches nutrition analysis results
func (acs *AICacheService) CacheNutritionAnalysis(ctx context.Context, ingredients []string, nutrition *outbound.NutritionInfo, processingTime time.Duration) error {
	if nutrition == nil {
		return fmt.Errorf("cannot cache nil nutrition info")
	}
	
	cacheKey := acs.buildNutritionAnalysisKey(ingredients)
	
	cached := CachedAIResponse{
		Model:          "nutrition_analysis",
		PromptHash:     acs.hashStringSlice(ingredients),
		Parameters:     map[string]interface{}{"ingredients": ingredients},
		Response:       nutrition,
		ProcessingTime: processingTime,
		CachedAt:       time.Now(),
		AccessCount:    0,
		LastAccess:     time.Now(),
		ModelVersion:   acs.config.ModelVersion,
		Tags:           []string{"nutrition", "analysis", "ai"},
	}
	
	return acs.cacheAIResponse(ctx, cacheKey, &cached, acs.config.NutritionAnalysisTTL)
}

// GetNutritionAnalysis retrieves cached nutrition analysis or calls fallback
func (acs *AICacheService) GetNutritionAnalysis(ctx context.Context, ingredients []string, fallback func(context.Context, []string) (*outbound.NutritionInfo, error)) (*outbound.NutritionInfo, error) {
	cacheKey := acs.buildNutritionAnalysisKey(ingredients)
	
	// Try cache first
	if cached, err := acs.getAIResponse(ctx, cacheKey); err == nil {
		// Handle both direct struct and map[string]interface{} formats
		var nutrition *outbound.NutritionInfo
		
		if n, ok := cached.Response.(*outbound.NutritionInfo); ok {
			nutrition = n
		} else if dataMap, ok := cached.Response.(map[string]interface{}); ok {
			// Convert map to struct
			data, err := json.Marshal(dataMap)
			if err == nil {
				nutrition = &outbound.NutritionInfo{}
				json.Unmarshal(data, nutrition)
			}
		}
		
		if nutrition != nil {
			acs.updateAccessStats(ctx, cacheKey, cached)
			
			acs.logger.Debug("AI nutrition analysis cache hit",
				zap.Strings("ingredients", ingredients),
				zap.Int("calories", nutrition.Calories))
			
			return nutrition, nil
		}
		
		// Invalid response format, remove from cache
		acs.cache.Delete(ctx, cacheKey)
	}
	
	// Cache miss - use fallback
	if fallback == nil {
		return nil, fmt.Errorf("nutrition analysis not found in cache and no fallback provided")
	}
	
	start := time.Now()
	nutrition, err := fallback(ctx, ingredients)
	processingTime := time.Since(start)
	
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	if err := acs.CacheNutritionAnalysis(ctx, ingredients, nutrition, processingTime); err != nil {
		acs.logger.Error("Failed to cache nutrition analysis", zap.Error(err))
	}
	
	return nutrition, nil
}

// InvalidateAICache removes AI responses for a specific model or pattern
func (acs *AICacheService) InvalidateAICache(ctx context.Context, model string) error {
	pattern := acs.keyBuilder.BuildKey("ai", model, "*")
	
	if err := acs.cache.InvalidateByPattern(ctx, pattern); err != nil {
		acs.logger.Error("Failed to invalidate AI cache", 
			zap.String("model", model), 
			zap.Error(err))
		return err
	}
	
	acs.logger.Info("AI cache invalidated", zap.String("model", model))
	return nil
}

// GetAIStats returns AI caching statistics
func (acs *AICacheService) GetAIStats(ctx context.Context) (*AICacheStats, error) {
	// This would aggregate statistics from Redis
	// For now, return basic stats from cache service
	cacheStats := acs.cache.GetStats()
	
	return &AICacheStats{
		TotalResponses:   cacheStats.TotalHits,
		CacheHits:        cacheStats.TotalHits,
		CacheMisses:      cacheStats.TotalMisses,
		HitRatio:         cacheStats.HitRatio,
		AvgResponseTime:  cacheStats.AvgReadTime,
		StorageUsed:      0, // Would need Redis memory usage
		LastReset:        cacheStats.LastReset,
	}, nil
}

// Helper methods

func (acs *AICacheService) buildRecipeGenerationKey(prompt string, constraints outbound.AIConstraints) string {
	params := acs.constraintsToParams(constraints)
	return acs.keyBuilder.BuildAIKey("recipe_generation", prompt, params)
}

func (acs *AICacheService) buildIngredientSuggestionsKey(ingredients []string) string {
	prompt := strings.Join(ingredients, ",")
	return acs.keyBuilder.BuildAIKey("ingredient_suggestions", prompt, nil)
}

func (acs *AICacheService) buildNutritionAnalysisKey(ingredients []string) string {
	prompt := strings.Join(ingredients, ",")
	return acs.keyBuilder.BuildAIKey("nutrition_analysis", prompt, nil)
}

func (acs *AICacheService) constraintsToParams(constraints outbound.AIConstraints) map[string]interface{} {
	return map[string]interface{}{
		"max_calories":      constraints.MaxCalories,
		"dietary":          constraints.Dietary,
		"cuisine":          constraints.Cuisine,
		"serving_size":     constraints.ServingSize,
		"cooking_time":     constraints.CookingTime,
		"skill_level":      constraints.SkillLevel,
		"equipment":        constraints.Equipment,
		"avoid_ingredients": constraints.AvoidIngredients,
	}
}

func (acs *AICacheService) hashPrompt(prompt string) string {
	hasher := sha256.New()
	hasher.Write([]byte(prompt))
	return fmt.Sprintf("%x", hasher.Sum(nil))[:16]
}

func (acs *AICacheService) hashStringSlice(slice []string) string {
	combined := strings.Join(slice, "|")
	return acs.hashPrompt(combined)
}

func (acs *AICacheService) shouldStoreFullPrompt(prompt string) bool {
	// Don't store prompts that are too long or might contain sensitive data
	if len(prompt) > acs.config.MaxPromptLength {
		return false
	}
	
	// Check for potential PII patterns
	sensitivePatterns := []string{
		"email", "phone", "address", "ssn", "credit card",
		"password", "secret", "token", "key",
	}
	
	promptLower := strings.ToLower(prompt)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(promptLower, pattern) {
			return false
		}
	}
	
	return true
}

func (acs *AICacheService) cacheAIResponse(ctx context.Context, cacheKey string, cached *CachedAIResponse, ttl time.Duration) error {
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal AI response: %w", err)
	}
	
	// Check response size
	if int64(len(data)) > acs.config.MaxResponseSize {
		return fmt.Errorf("AI response too large to cache: %d bytes", len(data))
	}
	
	// Apply adaptive TTL if enabled
	if acs.config.AdaptiveTTL {
		ttl = acs.calculateAdaptiveTTL(cached, ttl)
	}
	
	tags := append(cached.Tags, "ai_response")
	
	if err := acs.cache.SetWithTags(ctx, cacheKey, data, ttl, tags); err != nil {
		return fmt.Errorf("failed to cache AI response: %w", err)
	}
	
	acs.logger.Debug("AI response cached",
		zap.String("model", cached.Model),
		zap.String("key", cacheKey),
		zap.Duration("ttl", ttl),
		zap.Duration("processing_time", cached.ProcessingTime))
	
	return nil
}

func (acs *AICacheService) getAIResponse(ctx context.Context, cacheKey string) (*CachedAIResponse, error) {
	data, err := acs.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}
	
	var cached CachedAIResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AI response: %w", err)
	}
	
	// Validate model version if versioned caching is enabled
	if acs.config.VersionedCaching && cached.ModelVersion != acs.config.ModelVersion {
		// Remove outdated cache entry
		go acs.cache.Delete(context.Background(), cacheKey)
		return nil, fmt.Errorf("cached response from different model version")
	}
	
	return &cached, nil
}

func (acs *AICacheService) updateAccessStats(ctx context.Context, cacheKey string, cached *CachedAIResponse) {
	cached.AccessCount++
	cached.LastAccess = time.Now()
	
	// Update cache asynchronously
	go func() {
		data, err := json.Marshal(cached)
		if err != nil {
			return
		}
		
		ttl := acs.calculateRemainingTTL(cached)
		if ttl > 0 {
			acs.cache.Set(context.Background(), cacheKey, data, ttl)
		}
	}()
}

func (acs *AICacheService) calculateAdaptiveTTL(cached *CachedAIResponse, baseTTL time.Duration) time.Duration {
	// Extend TTL for high-confidence responses
	if cached.Confidence > 0.9 {
		baseTTL = time.Duration(float64(baseTTL) * 1.5)
	}
	
	// Extend TTL for fast responses (indicates simple/cached upstream)
	if cached.ProcessingTime < time.Second {
		baseTTL = time.Duration(float64(baseTTL) * 1.2)
	}
	
	return baseTTL
}

func (acs *AICacheService) calculateRemainingTTL(cached *CachedAIResponse) time.Duration {
	// Estimate remaining TTL based on cache type
	elapsed := time.Since(cached.CachedAt)
	
	var originalTTL time.Duration
	switch cached.Model {
	case "recipe_generation":
		originalTTL = acs.config.RecipeGenerationTTL
	case "ingredient_suggestions":
		originalTTL = acs.config.IngredientSuggestionTTL
	case "nutrition_analysis":
		originalTTL = acs.config.NutritionAnalysisTTL
	default:
		originalTTL = time.Hour
	}
	
	remaining := originalTTL - elapsed
	if remaining < 0 {
		return 0
	}
	
	return remaining
}

func (acs *AICacheService) trackPromptUsage(ctx context.Context, prompt string, confidence float64) {
	promptHash := acs.hashPrompt(prompt)
	statsKey := acs.keyBuilder.BuildKey("ai_stats", promptHash)
	
	// This would update prompt usage statistics
	// Implementation would depend on specific requirements
	acs.logger.Debug("Prompt usage tracked",
		zap.String("prompt_hash", promptHash),
		zap.Float64("confidence", confidence))
}

// AICacheStats represents AI caching statistics
type AICacheStats struct {
	TotalResponses  int64         `json:"total_responses"`
	CacheHits       int64         `json:"cache_hits"`
	CacheMisses     int64         `json:"cache_misses"`
	HitRatio        float64       `json:"hit_ratio"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	StorageUsed     int64         `json:"storage_used"`
	LastReset       time.Time     `json:"last_reset"`
}

// DefaultAICacheConfig returns default AI cache configuration
func DefaultAICacheConfig() *AICacheConfig {
	return &AICacheConfig{
		RecipeGenerationTTL:     time.Hour * 2,
		IngredientSuggestionTTL: time.Hour * 6,
		NutritionAnalysisTTL:    time.Hour * 12,
		DescriptionGenerationTTL: time.Hour,
		ClassificationTTL:       time.Hour * 4,
		CacheByPromptHash:       true,
		CacheByParameters:       true,
		CompressionEnabled:      true,
		MaxPromptLength:         2000,
		MaxResponseSize:         1024 * 1024, // 1MB
		BatchCaching:            true,
		PreWarmPopular:          false,
		AdaptiveTTL:             true,
		PopularPromptBonus:      time.Hour,
		ValidateResponses:       true,
		FilterUnsafeContent:     true,
		VersionedCaching:        true,
		ModelVersion:            "v1.0",
	}
}