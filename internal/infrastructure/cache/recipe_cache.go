// Package cache provides specialized recipe caching with search optimization
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RecipeCacheService provides comprehensive recipe caching with cache-first pattern
type RecipeCacheService struct {
	cache      *CacheService
	keyBuilder *KeyBuilder
	config     *RecipeCacheConfig
	logger     *zap.Logger
}

// RecipeCacheConfig configures recipe caching behavior
type RecipeCacheConfig struct {
	// TTL configurations
	RecipeTTL           time.Duration `json:"recipe_ttl"`
	RecipeListTTL       time.Duration `json:"recipe_list_ttl"`
	SearchResultsTTL    time.Duration `json:"search_results_ttl"`
	TrendingTTL         time.Duration `json:"trending_ttl"`
	RecommendationsTTL  time.Duration `json:"recommendations_ttl"`
	
	// Search optimization
	MaxSearchResults    int           `json:"max_search_results"`
	SearchCacheKeys     int           `json:"search_cache_keys"`
	PopularSearchTTL    time.Duration `json:"popular_search_ttl"`
	
	// Recipe list pagination
	MaxPageSize         int           `json:"max_page_size"`
	DefaultPageSize     int           `json:"default_page_size"`
	
	// Performance settings
	BatchSize           int           `json:"batch_size"`
	WarmupEnabled       bool          `json:"warmup_enabled"`
	WarmupPopularCount  int           `json:"warmup_popular_count"`
	
	// Invalidation settings
	InvalidationTags    []string      `json:"invalidation_tags"`
	CascadeInvalidation bool          `json:"cascade_invalidation"`
}

// CachedRecipe represents a cached recipe with metadata
type CachedRecipe struct {
	Recipe      *recipe.Recipe `json:"recipe"`
	CachedAt    time.Time      `json:"cached_at"`
	AccessCount int64          `json:"access_count"`
	Tags        []string       `json:"tags"`
}

// CachedRecipeList represents a cached recipe list
type CachedRecipeList struct {
	Recipes     []*recipe.Recipe `json:"recipes"`
	Total       int              `json:"total"`
	Page        int              `json:"page"`
	Limit       int              `json:"limit"`
	Filters     string           `json:"filters"`
	CachedAt    time.Time        `json:"cached_at"`
	HasMore     bool             `json:"has_more"`
}

// CachedSearchResults represents cached search results
type CachedSearchResults struct {
	Query       string           `json:"query"`
	Results     []*recipe.Recipe `json:"results"`
	Total       int              `json:"total"`
	Suggestions []string         `json:"suggestions"`
	Filters     map[string]interface{} `json:"filters"`
	CachedAt    time.Time        `json:"cached_at"`
	SearchTime  time.Duration    `json:"search_time"`
}

// NewRecipeCacheService creates a new recipe cache service
func NewRecipeCacheService(cache *CacheService, logger *zap.Logger) *RecipeCacheService {
	config := DefaultRecipeCacheConfig()
	
	return &RecipeCacheService{
		cache:      cache,
		keyBuilder: NewKeyBuilder(),
		config:     config,
		logger:     logger,
	}
}

// GetRecipe retrieves a recipe from cache or falls back to repository
func (rcs *RecipeCacheService) GetRecipe(ctx context.Context, id uuid.UUID, fallback func(context.Context, uuid.UUID) (*recipe.Recipe, error)) (*recipe.Recipe, error) {
	cacheKey := rcs.keyBuilder.BuildRecipeKey(id.String())
	
	// Try cache first
	data, err := rcs.cache.Get(ctx, cacheKey)
	if err == nil {
		var cached CachedRecipe
		if err := json.Unmarshal(data, &cached); err == nil {
			// Update access count asynchronously
			go rcs.updateAccessCount(ctx, cacheKey, &cached)
			
			rcs.logger.Debug("Recipe cache hit", 
				zap.String("recipe_id", id.String()),
				zap.Int64("access_count", cached.AccessCount))
			
			return cached.Recipe, nil
		}
		
		rcs.logger.Error("Failed to unmarshal cached recipe", 
			zap.String("recipe_id", id.String()), 
			zap.Error(err))
	}
	
	// Cache miss - use fallback
	if fallback == nil {
		return nil, fmt.Errorf("recipe not found in cache and no fallback provided")
	}
	
	recipeData, err := fallback(ctx, id)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	if err := rcs.CacheRecipe(ctx, recipeData); err != nil {
		rcs.logger.Error("Failed to cache recipe after fallback", 
			zap.String("recipe_id", id.String()), 
			zap.Error(err))
	}
	
	return recipeData, nil
}

// CacheRecipe stores a recipe in cache with tags for invalidation
func (rcs *RecipeCacheService) CacheRecipe(ctx context.Context, r *recipe.Recipe) error {
	if r == nil || r.ID == uuid.Nil {
		return fmt.Errorf("invalid recipe for caching")
	}
	
	cacheKey := rcs.keyBuilder.BuildRecipeKey(r.ID.String())
	
	// Create cached recipe with metadata
	cached := CachedRecipe{
		Recipe:      r,
		CachedAt:    time.Now(),
		AccessCount: 0,
		Tags:        rcs.generateRecipeTags(r),
	}
	
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal recipe for cache: %w", err)
	}
	
	// Store with tags for invalidation
	tags := append([]string{
		"recipe",
		fmt.Sprintf("user:%s", r.AuthorID.String()),
		fmt.Sprintf("status:%s", r.Status),
	}, rcs.generateCuisineTags(r)...)
	
	if err := rcs.cache.SetWithTags(ctx, cacheKey, data, rcs.config.RecipeTTL, tags); err != nil {
		return fmt.Errorf("failed to cache recipe: %w", err)
	}
	
	rcs.logger.Debug("Recipe cached successfully", 
		zap.String("recipe_id", r.ID.String()),
		zap.Strings("tags", tags))
	
	return nil
}

// GetRecipeList retrieves a paginated recipe list from cache or fallback
func (rcs *RecipeCacheService) GetRecipeList(ctx context.Context, criteria outbound.SearchCriteria, fallback func(context.Context, outbound.SearchCriteria) ([]*recipe.Recipe, int, error)) ([]*recipe.Recipe, int, error) {
	// Normalize and validate criteria
	criteria = rcs.normalizeCriteria(criteria)
	
	cacheKey := rcs.buildRecipeListKey(criteria)
	
	// Try cache first
	data, err := rcs.cache.Get(ctx, cacheKey)
	if err == nil {
		var cached CachedRecipeList
		if err := json.Unmarshal(data, &cached); err == nil {
			rcs.logger.Debug("Recipe list cache hit", 
				zap.String("filters", cached.Filters),
				zap.Int("count", len(cached.Recipes)))
			
			return cached.Results, cached.Total, nil
		}
		
		rcs.logger.Error("Failed to unmarshal cached recipe list", zap.Error(err))
	}
	
	// Cache miss - use fallback
	if fallback == nil {
		return nil, 0, fmt.Errorf("recipe list not found in cache and no fallback provided")
	}
	
	recipes, total, err := fallback(ctx, criteria)
	if err != nil {
		return nil, 0, err
	}
	
	// Cache the result
	if err := rcs.CacheRecipeList(ctx, recipes, total, criteria); err != nil {
		rcs.logger.Error("Failed to cache recipe list after fallback", zap.Error(err))
	}
	
	return recipes, total, nil
}

// CacheRecipeList stores a recipe list in cache
func (rcs *RecipeCacheService) CacheRecipeList(ctx context.Context, recipes []*recipe.Recipe, total int, criteria outbound.SearchCriteria) error {
	cacheKey := rcs.buildRecipeListKey(criteria)
	
	cached := CachedRecipeList{
		Recipes:  recipes,
		Total:    total,
		Page:     criteria.Offset / criteria.Limit,
		Limit:    criteria.Limit,
		Filters:  rcs.serializeCriteria(criteria),
		CachedAt: time.Now(),
		HasMore:  (criteria.Offset + criteria.Limit) < total,
	}
	
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal recipe list for cache: %w", err)
	}
	
	// Generate tags for invalidation
	tags := []string{"recipe_list"}
	if criteria.AuthorID != nil {
		tags = append(tags, fmt.Sprintf("user:%s", criteria.AuthorID.String()))
	}
	for _, cuisine := range criteria.Cuisines {
		tags = append(tags, fmt.Sprintf("cuisine:%s", cuisine))
	}
	for _, category := range criteria.Categories {
		tags = append(tags, fmt.Sprintf("category:%s", category))
	}
	
	if err := rcs.cache.SetWithTags(ctx, cacheKey, data, rcs.config.RecipeListTTL, tags); err != nil {
		return fmt.Errorf("failed to cache recipe list: %w", err)
	}
	
	rcs.logger.Debug("Recipe list cached successfully", 
		zap.String("key", cacheKey),
		zap.Int("count", len(recipes)),
		zap.Strings("tags", tags))
	
	return nil
}

// SearchRecipes performs cached recipe search with optimized result storage
func (rcs *RecipeCacheService) SearchRecipes(ctx context.Context, query string, filters map[string]interface{}, fallback func(context.Context, string, map[string]interface{}) ([]*recipe.Recipe, int, []string, error)) ([]*recipe.Recipe, int, []string, error) {
	// Normalize query and filters
	normalizedQuery := strings.TrimSpace(strings.ToLower(query))
	normalizedFilters := rcs.normalizeFilters(filters)
	
	cacheKey := rcs.keyBuilder.BuildSearchKey(normalizedQuery, normalizedFilters)
	
	// Try cache first
	data, err := rcs.cache.Get(ctx, cacheKey)
	if err == nil {
		var cached CachedSearchResults
		if err := json.Unmarshal(data, &cached); err == nil {
			rcs.logger.Debug("Search cache hit", 
				zap.String("query", query),
				zap.Int("results", len(cached.Results)))
			
			return cached.Results, cached.Total, cached.Suggestions, nil
		}
		
		rcs.logger.Error("Failed to unmarshal cached search results", zap.Error(err))
	}
	
	// Cache miss - use fallback
	if fallback == nil {
		return nil, 0, nil, fmt.Errorf("search results not found in cache and no fallback provided")
	}
	
	start := time.Now()
	results, total, suggestions, err := fallback(ctx, query, filters)
	searchTime := time.Since(start)
	
	if err != nil {
		return nil, 0, nil, err
	}
	
	// Cache the results
	if err := rcs.CacheSearchResults(ctx, query, filters, results, total, suggestions, searchTime); err != nil {
		rcs.logger.Error("Failed to cache search results", zap.Error(err))
	}
	
	return results, total, suggestions, nil
}

// CacheSearchResults stores search results in cache
func (rcs *RecipeCacheService) CacheSearchResults(ctx context.Context, query string, filters map[string]interface{}, results []*recipe.Recipe, total int, suggestions []string, searchTime time.Duration) error {
	normalizedQuery := strings.TrimSpace(strings.ToLower(query))
	normalizedFilters := rcs.normalizeFilters(filters)
	
	cacheKey := rcs.keyBuilder.BuildSearchKey(normalizedQuery, normalizedFilters)
	
	cached := CachedSearchResults{
		Query:       normalizedQuery,
		Results:     results,
		Total:       total,
		Suggestions: suggestions,
		Filters:     normalizedFilters,
		CachedAt:    time.Now(),
		SearchTime:  searchTime,
	}
	
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal search results for cache: %w", err)
	}
	
	// Use shorter TTL for search results
	ttl := rcs.config.SearchResultsTTL
	
	// Popular searches get longer TTL
	if rcs.isPopularSearch(query) {
		ttl = rcs.config.PopularSearchTTL
	}
	
	tags := []string{"search", "recipe_search"}
	
	if err := rcs.cache.SetWithTags(ctx, cacheKey, data, ttl, tags); err != nil {
		return fmt.Errorf("failed to cache search results: %w", err)
	}
	
	rcs.logger.Debug("Search results cached", 
		zap.String("query", query),
		zap.Int("results", len(results)),
		zap.Duration("ttl", ttl))
	
	// Update search popularity tracking
	go rcs.trackSearchPopularity(ctx, normalizedQuery)
	
	return nil
}

// InvalidateRecipe removes a recipe and related cached data
func (rcs *RecipeCacheService) InvalidateRecipe(ctx context.Context, recipeID uuid.UUID) error {
	// Direct recipe cache
	recipeKey := rcs.keyBuilder.BuildRecipeKey(recipeID.String())
	if err := rcs.cache.Delete(ctx, recipeKey); err != nil {
		rcs.logger.Error("Failed to invalidate recipe cache", 
			zap.String("recipe_id", recipeID.String()), 
			zap.Error(err))
	}
	
	// Invalidate by tags if cascade invalidation is enabled
	if rcs.config.CascadeInvalidation {
		// This would typically involve getting the recipe to determine its tags
		// For now, we'll invalidate common related caches
		tags := []string{
			"recipe_list",
			"search",
			"trending",
			"recommendations",
		}
		
		for _, tag := range tags {
			if err := rcs.cache.InvalidateByTag(ctx, tag); err != nil {
				rcs.logger.Error("Failed to invalidate by tag", 
					zap.String("tag", tag), 
					zap.Error(err))
			}
		}
	}
	
	rcs.logger.Info("Recipe invalidated", zap.String("recipe_id", recipeID.String()))
	return nil
}

// InvalidateUserRecipes removes all cached data for a user's recipes
func (rcs *RecipeCacheService) InvalidateUserRecipes(ctx context.Context, userID uuid.UUID) error {
	userTag := fmt.Sprintf("user:%s", userID.String())
	
	if err := rcs.cache.InvalidateByTag(ctx, userTag); err != nil {
		rcs.logger.Error("Failed to invalidate user recipes", 
			zap.String("user_id", userID.String()), 
			zap.Error(err))
		return err
	}
	
	rcs.logger.Info("User recipes invalidated", zap.String("user_id", userID.String()))
	return nil
}

// WarmupPopularRecipes pre-loads popular recipes into cache
func (rcs *RecipeCacheService) WarmupPopularRecipes(ctx context.Context, loader func(context.Context, int) ([]*recipe.Recipe, error)) error {
	if !rcs.config.WarmupEnabled {
		return nil
	}
	
	rcs.logger.Info("Starting recipe cache warmup", 
		zap.Int("count", rcs.config.WarmupPopularCount))
	
	start := time.Now()
	
	recipes, err := loader(ctx, rcs.config.WarmupPopularCount)
	if err != nil {
		return fmt.Errorf("failed to load popular recipes for warmup: %w", err)
	}
	
	warmed := 0
	for _, r := range recipes {
		if err := rcs.CacheRecipe(ctx, r); err != nil {
			rcs.logger.Error("Failed to warm recipe cache", 
				zap.String("recipe_id", r.ID.String()), 
				zap.Error(err))
		} else {
			warmed++
		}
	}
	
	duration := time.Since(start)
	rcs.logger.Info("Recipe cache warmup completed", 
		zap.Int("total", len(recipes)),
		zap.Int("warmed", warmed),
		zap.Duration("duration", duration))
	
	return nil
}

// Helper methods

func (rcs *RecipeCacheService) updateAccessCount(ctx context.Context, cacheKey string, cached *CachedRecipe) {
	cached.AccessCount++
	
	data, err := json.Marshal(cached)
	if err != nil {
		rcs.logger.Error("Failed to marshal updated recipe cache", zap.Error(err))
		return
	}
	
	// Update cache with new access count
	if err := rcs.cache.Set(ctx, cacheKey, data, rcs.config.RecipeTTL); err != nil {
		rcs.logger.Error("Failed to update recipe access count", zap.Error(err))
	}
}

func (rcs *RecipeCacheService) generateRecipeTags(r *recipe.Recipe) []string {
	tags := []string{
		"recipe",
		fmt.Sprintf("user:%s", r.AuthorID.String()),
		fmt.Sprintf("status:%s", r.Status),
	}
	
	// Add cuisine tags
	tags = append(tags, rcs.generateCuisineTags(r)...)
	
	// Add category tags
	for _, category := range r.Categories {
		tags = append(tags, fmt.Sprintf("category:%s", category))
	}
	
	// Add difficulty tag
	tags = append(tags, fmt.Sprintf("difficulty:%s", r.Difficulty))
	
	return tags
}

func (rcs *RecipeCacheService) generateCuisineTags(r *recipe.Recipe) []string {
	tags := make([]string, 0, len(r.Cuisines))
	for _, cuisine := range r.Cuisines {
		tags = append(tags, fmt.Sprintf("cuisine:%s", cuisine))
	}
	return tags
}

func (rcs *RecipeCacheService) normalizeCriteria(criteria outbound.SearchCriteria) outbound.SearchCriteria {
	// Ensure reasonable limits
	if criteria.Limit <= 0 || criteria.Limit > rcs.config.MaxPageSize {
		criteria.Limit = rcs.config.DefaultPageSize
	}
	
	if criteria.Offset < 0 {
		criteria.Offset = 0
	}
	
	// Normalize order direction
	if criteria.OrderDir != "asc" && criteria.OrderDir != "desc" {
		criteria.OrderDir = "desc"
	}
	
	return criteria
}

func (rcs *RecipeCacheService) buildRecipeListKey(criteria outbound.SearchCriteria) string {
	// Create a consistent key from criteria
	filters := map[string]interface{}{
		"offset":    criteria.Offset,
		"limit":     criteria.Limit,
		"order_by":  criteria.OrderBy,
		"order_dir": criteria.OrderDir,
	}
	
	if criteria.AuthorID != nil {
		filters["author_id"] = criteria.AuthorID.String()
	}
	
	if len(criteria.Cuisines) > 0 {
		sort.Strings(criteria.Cuisines)
		filters["cuisines"] = strings.Join(criteria.Cuisines, ",")
	}
	
	if len(criteria.Categories) > 0 {
		sort.Strings(criteria.Categories)
		filters["categories"] = strings.Join(criteria.Categories, ",")
	}
	
	if len(criteria.Difficulty) > 0 {
		sort.Strings(criteria.Difficulty)
		filters["difficulty"] = strings.Join(criteria.Difficulty, ",")
	}
	
	if criteria.MinRating != nil {
		filters["min_rating"] = *criteria.MinRating
	}
	
	if criteria.MaxTime != nil {
		filters["max_time"] = *criteria.MaxTime
	}
	
	return rcs.keyBuilder.BuildRecipeListKey(criteria.Offset/criteria.Limit, criteria.Limit, filters)
}

func (rcs *RecipeCacheService) serializeCriteria(criteria outbound.SearchCriteria) string {
	data, _ := json.Marshal(criteria)
	return string(data)
}

func (rcs *RecipeCacheService) normalizeFilters(filters map[string]interface{}) map[string]interface{} {
	if filters == nil {
		return make(map[string]interface{})
	}
	
	normalized := make(map[string]interface{})
	for k, v := range filters {
		normalized[strings.ToLower(k)] = v
	}
	
	return normalized
}

func (rcs *RecipeCacheService) isPopularSearch(query string) bool {
	// This would typically check against a popularity tracking system
	// For now, we'll use simple heuristics
	popularTerms := []string{
		"chicken", "pasta", "salad", "soup", "dessert",
		"vegetarian", "vegan", "gluten-free", "quick", "easy",
	}
	
	queryLower := strings.ToLower(query)
	for _, term := range popularTerms {
		if strings.Contains(queryLower, term) {
			return true
		}
	}
	
	return false
}

func (rcs *RecipeCacheService) trackSearchPopularity(ctx context.Context, query string) {
	// Increment search count
	countKey := rcs.keyBuilder.BuildKey("search_count", query)
	
	if _, err := rcs.cache.redis.Increment(ctx, countKey, time.Hour*24); err != nil {
		rcs.logger.Error("Failed to track search popularity", 
			zap.String("query", query), 
			zap.Error(err))
	}
}

// DefaultRecipeCacheConfig returns default recipe cache configuration
func DefaultRecipeCacheConfig() *RecipeCacheConfig {
	return &RecipeCacheConfig{
		RecipeTTL:           time.Hour * 2,
		RecipeListTTL:       time.Minute * 15,
		SearchResultsTTL:    time.Minute * 10,
		TrendingTTL:         time.Hour,
		RecommendationsTTL:  time.Minute * 30,
		MaxSearchResults:    100,
		SearchCacheKeys:     1000,
		PopularSearchTTL:    time.Hour,
		MaxPageSize:         50,
		DefaultPageSize:     20,
		BatchSize:           25,
		WarmupEnabled:       true,
		WarmupPopularCount:  50,
		InvalidationTags:    []string{"recipe", "recipe_list", "search"},
		CascadeInvalidation: true,
	}
}