// Package cache provides utility components for caching infrastructure
package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Serializer interface for cache data serialization
type Serializer interface {
	Serialize(data interface{}) ([]byte, error)
	Deserialize(data []byte, dest interface{}) error
}

// JSONSerializer implements JSON serialization for cache data
type JSONSerializer struct{}

// NewJSONSerializer creates a new JSON serializer
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize converts data to JSON bytes
func (s *JSONSerializer) Serialize(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// Deserialize converts JSON bytes to data structure
func (s *JSONSerializer) Deserialize(data []byte, dest interface{}) error {
	return json.Unmarshal(data, dest)
}

// Compressor interface for cache data compression
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
}

// GzipCompressor implements gzip compression for cache data
type GzipCompressor struct{}

// NewGzipCompressor creates a new gzip compressor
func NewGzipCompressor() *GzipCompressor {
	return &GzipCompressor{}
}

// Compress compresses data using gzip
func (c *GzipCompressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, fmt.Errorf("failed to write compressed data: %w", err)
	}
	
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}
	
	return buf.Bytes(), nil
}

// Decompress decompresses gzip data
func (c *GzipCompressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()
	
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}
	
	return decompressed, nil
}

// KeyBuilder provides standardized cache key generation
type KeyBuilder struct {
	prefix   string
	separator string
}

// NewKeyBuilder creates a new key builder
func NewKeyBuilder() *KeyBuilder {
	return &KeyBuilder{
		prefix:    "alchemorsel:v3",
		separator: ":",
	}
}

// BuildKey constructs a cache key from components
func (kb *KeyBuilder) BuildKey(components ...string) string {
	parts := make([]string, 0, len(components)+1)
	parts = append(parts, kb.prefix)
	parts = append(parts, components...)
	return strings.Join(parts, kb.separator)
}

// BuildRecipeKey creates a key for recipe data
func (kb *KeyBuilder) BuildRecipeKey(recipeID string) string {
	return kb.BuildKey("recipe", recipeID)
}

// BuildRecipeListKey creates a key for recipe lists with filters
func (kb *KeyBuilder) BuildRecipeListKey(page, limit int, filters map[string]interface{}) string {
	filterHash := kb.hashFilters(filters)
	return kb.BuildKey("recipes", "list", fmt.Sprintf("p%d:l%d:f%s", page, limit, filterHash))
}

// BuildSearchKey creates a key for search results
func (kb *KeyBuilder) BuildSearchKey(query string, filters map[string]interface{}) string {
	filterHash := kb.hashFilters(filters)
	queryHash := kb.hashString(query)
	return kb.BuildKey("search", fmt.Sprintf("q%s:f%s", queryHash, filterHash))
}

// BuildUserKey creates a key for user data
func (kb *KeyBuilder) BuildUserKey(userID string) string {
	return kb.BuildKey("user", userID)
}

// BuildSessionKey creates a key for session data
func (kb *KeyBuilder) BuildSessionKey(sessionID string) string {
	return kb.BuildKey("session", sessionID)
}

// BuildAIKey creates a key for AI response caching
func (kb *KeyBuilder) BuildAIKey(model, prompt string, params map[string]interface{}) string {
	promptHash := kb.hashString(prompt)
	paramsHash := kb.hashFilters(params)
	return kb.BuildKey("ai", model, fmt.Sprintf("p%s:pr%s", promptHash, paramsHash))
}

// BuildTemplateKey creates a key for template caching
func (kb *KeyBuilder) BuildTemplateKey(templateName string, data map[string]interface{}) string {
	dataHash := kb.hashFilters(data)
	return kb.BuildKey("template", templateName, dataHash)
}

// BuildRateLimitKey creates a key for rate limiting
func (kb *KeyBuilder) BuildRateLimitKey(userID, endpoint, window string) string {
	return kb.BuildKey("ratelimit", userID, endpoint, window)
}

// BuildTagKey creates a key for tag-based invalidation
func (kb *KeyBuilder) BuildTagKey(tag string) string {
	return kb.BuildKey("tag", tag)
}

// BuildMetricsKey creates a key for metrics data
func (kb *KeyBuilder) BuildMetricsKey(metric, timeframe string) string {
	return kb.BuildKey("metrics", metric, timeframe)
}

// hashFilters creates a consistent hash from filter parameters
func (kb *KeyBuilder) hashFilters(filters map[string]interface{}) string {
	if len(filters) == 0 {
		return "none"
	}
	
	// Sort keys for consistent hashing
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// Build filter string
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := fmt.Sprintf("%v", filters[key])
		parts = append(parts, fmt.Sprintf("%s=%s", key, url.QueryEscape(value)))
	}
	
	filterString := strings.Join(parts, "&")
	return kb.hashString(filterString)
}

// hashString creates a MD5 hash of a string (first 8 characters for brevity)
func (kb *KeyBuilder) hashString(s string) string {
	hash := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", hash)[:8]
}

// CacheInvalidator handles complex invalidation scenarios
type CacheInvalidator struct {
	redis       *RedisClient
	localCache  *LocalCache
	logger      *zap.Logger
	batchSize   int
	timeout     time.Duration
}

// NewCacheInvalidator creates a new cache invalidator
func NewCacheInvalidator(redis *RedisClient, localCache *LocalCache, logger *zap.Logger) *CacheInvalidator {
	return &CacheInvalidator{
		redis:      redis,
		localCache: localCache,
		logger:     logger,
		batchSize:  100,
		timeout:    time.Second * 30,
	}
}

// InvalidateByTag removes all cache entries associated with specific tags
func (ci *CacheInvalidator) InvalidateByTag(ctx context.Context, tags ...string) error {
	ctx, cancel := context.WithTimeout(ctx, ci.timeout)
	defer cancel()

	ci.logger.Info("Starting tag-based cache invalidation", zap.Strings("tags", tags))
	
	totalInvalidated := 0
	for _, tag := range tags {
		tagKey := (&KeyBuilder{}).BuildTagKey(tag)
		
		// Get all keys associated with this tag
		keysToInvalidate, err := ci.redis.client.SMembers(ctx, tagKey).Result()
		if err != nil {
			ci.logger.Error("Failed to get keys for tag", zap.String("tag", tag), zap.Error(err))
			continue
		}
		
		if len(keysToInvalidate) == 0 {
			continue
		}
		
		// Invalidate keys in batches
		for i := 0; i < len(keysToInvalidate); i += ci.batchSize {
			end := i + ci.batchSize
			if end > len(keysToInvalidate) {
				end = len(keysToInvalidate)
			}
			
			batch := keysToInvalidate[i:end]
			
			// Remove from local cache
			for _, key := range batch {
				ci.localCache.Delete(key)
			}
			
			// Remove from Redis
			if err := ci.redis.Delete(ctx, batch...); err != nil {
				ci.logger.Error("Failed to delete cache batch", 
					zap.String("tag", tag),
					zap.Strings("keys", batch),
					zap.Error(err))
			} else {
				totalInvalidated += len(batch)
			}
		}
		
		// Clean up the tag set
		if err := ci.redis.Delete(ctx, tagKey); err != nil {
			ci.logger.Error("Failed to delete tag key", zap.String("tag", tag), zap.Error(err))
		}
	}
	
	ci.logger.Info("Tag-based cache invalidation completed",
		zap.Strings("tags", tags),
		zap.Int("keys_invalidated", totalInvalidated))
	
	return nil
}

// InvalidateByPattern removes cache entries matching a pattern
func (ci *CacheInvalidator) InvalidateByPattern(ctx context.Context, pattern string) error {
	ctx, cancel := context.WithTimeout(ctx, ci.timeout)
	defer cancel()

	ci.logger.Info("Starting pattern-based cache invalidation", zap.String("pattern", pattern))
	
	// Find matching keys in Redis
	keysToInvalidate, err := ci.redis.ScanKeys(ctx, pattern)
	if err != nil {
		ci.logger.Error("Failed to scan keys for pattern", zap.String("pattern", pattern), zap.Error(err))
		return err
	}
	
	if len(keysToInvalidate) == 0 {
		ci.logger.Info("No keys found matching pattern", zap.String("pattern", pattern))
		return nil
	}
	
	// Invalidate local cache
	ci.localCache.InvalidatePattern(pattern)
	
	// Invalidate Redis cache in batches
	totalInvalidated := 0
	for i := 0; i < len(keysToInvalidate); i += ci.batchSize {
		end := i + ci.batchSize
		if end > len(keysToInvalidate) {
			end = len(keysToInvalidate)
		}
		
		batch := keysToInvalidate[i:end]
		
		if err := ci.redis.Delete(ctx, batch...); err != nil {
			ci.logger.Error("Failed to delete cache batch", 
				zap.String("pattern", pattern),
				zap.Strings("keys", batch),
				zap.Error(err))
		} else {
			totalInvalidated += len(batch)
		}
	}
	
	ci.logger.Info("Pattern-based cache invalidation completed",
		zap.String("pattern", pattern),
		zap.Int("keys_invalidated", totalInvalidated))
	
	return nil
}

// InvalidateRecipeRelated invalidates all recipe-related cache entries
func (ci *CacheInvalidator) InvalidateRecipeRelated(ctx context.Context, recipeID string) error {
	patterns := []string{
		fmt.Sprintf("alchemorsel:v3:recipe:%s", recipeID),
		"alchemorsel:v3:recipes:list:*",
		"alchemorsel:v3:search:*",
		"alchemorsel:v3:ai:*recipe*",
	}
	
	for _, pattern := range patterns {
		if err := ci.InvalidateByPattern(ctx, pattern); err != nil {
			ci.logger.Error("Failed to invalidate recipe-related cache", 
				zap.String("recipe_id", recipeID),
				zap.String("pattern", pattern),
				zap.Error(err))
		}
	}
	
	return nil
}

// InvalidateUserRelated invalidates all user-related cache entries
func (ci *CacheInvalidator) InvalidateUserRelated(ctx context.Context, userID string) error {
	patterns := []string{
		fmt.Sprintf("alchemorsel:v3:user:%s", userID),
		fmt.Sprintf("alchemorsel:v3:session:%s:*", userID),
		fmt.Sprintf("alchemorsel:v3:recipes:list:*user:%s*", userID),
	}
	
	for _, pattern := range patterns {
		if err := ci.InvalidateByPattern(ctx, pattern); err != nil {
			ci.logger.Error("Failed to invalidate user-related cache", 
				zap.String("user_id", userID),
				zap.String("pattern", pattern),
				zap.Error(err))
		}
	}
	
	return nil
}

// CacheWarmer provides cache warming functionality
type CacheWarmer struct {
	cache   *CacheService
	logger  *zap.Logger
}

// NewCacheWarmer creates a new cache warmer
func NewCacheWarmer(cache *CacheService, logger *zap.Logger) *CacheWarmer {
	return &CacheWarmer{
		cache:  cache,
		logger: logger,
	}
}

// WarmupRecipes loads popular recipes into cache
func (cw *CacheWarmer) WarmupRecipes(ctx context.Context, recipeIDs []string) error {
	cw.logger.Info("Starting recipe cache warmup", zap.Int("count", len(recipeIDs)))
	
	// This would typically load recipes from the database
	// For now, we'll simulate the warming process
	warmed := 0
	for _, recipeID := range recipeIDs {
		// In a real implementation, you would:
		// 1. Load recipe from database
		// 2. Store in cache with appropriate TTL
		// 3. Handle errors gracefully
		
		key := (&KeyBuilder{}).BuildRecipeKey(recipeID)
		// Simulate recipe data
		recipeData := map[string]interface{}{
			"id":          recipeID,
			"title":       fmt.Sprintf("Recipe %s", recipeID),
			"warmed_at":   time.Now(),
		}
		
		data, err := json.Marshal(recipeData)
		if err != nil {
			cw.logger.Error("Failed to marshal recipe data", zap.String("recipe_id", recipeID), zap.Error(err))
			continue
		}
		
		if err := cw.cache.Set(ctx, key, data, cw.cache.config.RecipeTTL); err != nil {
			cw.logger.Error("Failed to warm recipe cache", zap.String("recipe_id", recipeID), zap.Error(err))
			continue
		}
		
		warmed++
	}
	
	cw.logger.Info("Recipe cache warmup completed", 
		zap.Int("total", len(recipeIDs)),
		zap.Int("warmed", warmed))
	
	return nil
}

// WarmupSearchResults loads popular search results into cache
func (cw *CacheWarmer) WarmupSearchResults(ctx context.Context, searches []SearchQuery) error {
	cw.logger.Info("Starting search cache warmup", zap.Int("count", len(searches)))
	
	warmed := 0
	for _, search := range searches {
		key := (&KeyBuilder{}).BuildSearchKey(search.Query, search.Filters)
		
		// Simulate search results
		searchResults := map[string]interface{}{
			"query":     search.Query,
			"results":   []string{"recipe1", "recipe2", "recipe3"}, // Simulated
			"total":     3,
			"warmed_at": time.Now(),
		}
		
		data, err := json.Marshal(searchResults)
		if err != nil {
			cw.logger.Error("Failed to marshal search results", zap.String("query", search.Query), zap.Error(err))
			continue
		}
		
		if err := cw.cache.Set(ctx, key, data, cw.cache.config.SearchTTL); err != nil {
			cw.logger.Error("Failed to warm search cache", zap.String("query", search.Query), zap.Error(err))
			continue
		}
		
		warmed++
	}
	
	cw.logger.Info("Search cache warmup completed", 
		zap.Int("total", len(searches)),
		zap.Int("warmed", warmed))
	
	return nil
}

// SearchQuery represents a search query for cache warming
type SearchQuery struct {
	Query   string                 `json:"query"`
	Filters map[string]interface{} `json:"filters"`
}

// CacheHealthChecker monitors cache health
type CacheHealthChecker struct {
	cache  *CacheService
	redis  *RedisClient
	logger *zap.Logger
}

// NewCacheHealthChecker creates a new cache health checker
func NewCacheHealthChecker(cache *CacheService, redis *RedisClient, logger *zap.Logger) *CacheHealthChecker {
	return &CacheHealthChecker{
		cache:  cache,
		redis:  redis,
		logger: logger,
	}
}

// CheckHealth performs comprehensive cache health checks
func (chc *CacheHealthChecker) CheckHealth(ctx context.Context) *CacheHealthStatus {
	status := &CacheHealthStatus{
		Timestamp: time.Now(),
	}
	
	// Check Redis connectivity
	if err := chc.redis.Ping(ctx); err != nil {
		status.RedisHealthy = false
		status.Errors = append(status.Errors, fmt.Sprintf("Redis ping failed: %v", err))
	} else {
		status.RedisHealthy = true
	}
	
	// Check cache performance
	stats := chc.cache.GetStats()
	status.CacheStats = stats
	
	// Check hit ratio (should be above threshold)
	if stats.HitRatio < 0.8 { // 80% hit ratio threshold
		status.Warnings = append(status.Warnings, 
			fmt.Sprintf("Cache hit ratio below threshold: %.2f%%", stats.HitRatio*100))
	}
	
	// Check average response time
	if stats.AvgReadTime > time.Millisecond*100 {
		status.Warnings = append(status.Warnings, 
			fmt.Sprintf("Average read time above threshold: %v", stats.AvgReadTime))
	}
	
	// Overall health
	status.OverallHealthy = status.RedisHealthy && len(status.Errors) == 0
	
	return status
}

// CacheHealthStatus represents cache health status
type CacheHealthStatus struct {
	Timestamp      time.Time   `json:"timestamp"`
	OverallHealthy bool        `json:"overall_healthy"`
	RedisHealthy   bool        `json:"redis_healthy"`
	CacheStats     *CacheStats `json:"cache_stats"`
	Warnings       []string    `json:"warnings,omitempty"`
	Errors         []string    `json:"errors,omitempty"`
}