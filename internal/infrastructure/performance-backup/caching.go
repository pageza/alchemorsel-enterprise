package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/cache"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CacheConfig holds caching configuration
type CacheConfig struct {
	DefaultTTL      time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	CompressionEnabled bool
	DistributedCache   bool
}

// CacheManager manages multiple cache layers (legacy wrapper for new cache infrastructure)
type CacheManager struct {
	cacheService *cache.CacheService
	localCache   *MemoryCache
	config       CacheConfig
	logger       *zap.Logger
	metrics      CacheMetrics
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	Hits        int64
	Misses      int64
	Errors      int64
	Operations  int64
	TotalTime   time.Duration
}

// NewCacheManager creates a new cache manager using the new cache infrastructure
func NewCacheManager(cacheService *cache.CacheService, config CacheConfig, logger *zap.Logger) *CacheManager {
	return &CacheManager{
		cacheService: cacheService,
		localCache:   NewMemoryCache(1000), // 1000 item limit
		config:       config,
		logger:       logger,
	}
}

// Get retrieves a value from cache with multi-layer support
func (c *CacheManager) Get(ctx context.Context, key string, dest interface{}) error {
	start := time.Now()
	defer func() {
		c.metrics.Operations++
		c.metrics.TotalTime += time.Since(start)
	}()

	// Try local cache first (L1)
	if data, found := c.localCache.Get(key); found {
		c.metrics.Hits++
		return json.Unmarshal(data, dest)
	}

	// Try Redis cache (L2) via cache service
	data, err := c.cacheService.Get(ctx, key)
	if err != nil {
		c.metrics.Misses++
		return ErrCacheKeyNotFound
	}

	// Populate local cache for next access
	c.localCache.Set(key, []byte(data), c.config.DefaultTTL)
	
	c.metrics.Hits++
	return json.Unmarshal([]byte(data), dest)
}

// Set stores a value in cache with multi-layer support
func (c *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.metrics.Operations++
		c.metrics.TotalTime += time.Since(start)
	}()

	data, err := json.Marshal(value)
	if err != nil {
		c.metrics.Errors++
		return err
	}

	// Store in local cache (L1)
	c.localCache.Set(key, data, ttl)

	// Store in Redis cache (L2) via cache service
	err = c.cacheService.Set(ctx, key, string(data), ttl)
	if err != nil {
		c.metrics.Errors++
		c.logger.Error("Redis cache set error", zap.String("key", key), zap.Error(err))
		return err
	}

	return nil
}

// Delete removes a value from all cache layers
func (c *CacheManager) Delete(ctx context.Context, key string) error {
	// Remove from local cache
	c.localCache.Delete(key)

	// Remove from Redis cache via cache service
	err := c.cacheService.Delete(ctx, key)
	if err != nil {
		c.metrics.Errors++
		c.logger.Error("Redis cache delete error", zap.String("key", key), zap.Error(err))
		return err
	}

	return nil
}

// GetMulti retrieves multiple values efficiently
func (c *CacheManager) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	start := time.Now()
	defer func() {
		c.metrics.Operations++
		c.metrics.TotalTime += time.Since(start)
	}()

	results := make(map[string][]byte)
	missingKeys := make([]string, 0, len(keys))

	// Check local cache first
	for _, key := range keys {
		if data, found := c.localCache.Get(key); found {
			results[key] = data
			c.metrics.Hits++
		} else {
			missingKeys = append(missingKeys, key)
		}
	}

	// Fetch missing keys from Redis via cache service
	if len(missingKeys) > 0 {
		for _, key := range missingKeys {
			if data, err := c.cacheService.Get(ctx, key); err == nil {
				results[key] = []byte(data)
				c.localCache.Set(key, []byte(data), c.config.DefaultTTL)
				c.metrics.Hits++
			} else {
				c.metrics.Misses++
			}
		}
	}

	return results, nil
}

// SetMulti stores multiple values efficiently
func (c *CacheManager) SetMulti(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	start := time.Now()
	defer func() {
		c.metrics.Operations++
		c.metrics.TotalTime += time.Since(start)
	}()

	for key, value := range items {
		data, err := json.Marshal(value)
		if err != nil {
			c.metrics.Errors++
			return err
		}

		// Store in local cache
		c.localCache.Set(key, data, ttl)
		
		// Store in Redis cache via cache service
		if err := c.cacheService.Set(ctx, key, string(data), ttl); err != nil {
			c.metrics.Errors++
			c.logger.Error("Redis cache set error in multi-set", zap.String("key", key), zap.Error(err))
			return err
		}
	}

	return nil
}

// Invalidate removes cache entries matching a pattern
func (c *CacheManager) Invalidate(ctx context.Context, pattern string) error {
	// Clear local cache entries matching pattern
	c.localCache.InvalidatePattern(pattern)

	// Note: Cache service doesn't support pattern-based deletion
	// This is a simplified approach - in production, consider using a prefix-based approach
	c.logger.Warn("Pattern invalidation not fully supported with cache service", zap.String("pattern", pattern))

	return nil
}

// GetStats returns cache performance statistics
func (c *CacheManager) GetStats() CacheStats {
	hitRatio := float64(0)
	if c.metrics.Operations > 0 {
		hitRatio = float64(c.metrics.Hits) / float64(c.metrics.Hits+c.metrics.Misses)
	}

	avgResponseTime := time.Duration(0)
	if c.metrics.Operations > 0 {
		avgResponseTime = c.metrics.TotalTime / time.Duration(c.metrics.Operations)
	}

	return CacheStats{
		Hits:            c.metrics.Hits,
		Misses:          c.metrics.Misses,
		Errors:          c.metrics.Errors,
		Operations:      c.metrics.Operations,
		HitRatio:        hitRatio,
		AvgResponseTime: avgResponseTime,
		LocalCacheSize:  c.localCache.Size(),
	}
}

// CacheStats represents cache performance statistics
type CacheStats struct {
	Hits            int64         `json:"hits"`
	Misses          int64         `json:"misses"`
	Errors          int64         `json:"errors"`
	Operations      int64         `json:"operations"`
	HitRatio        float64       `json:"hit_ratio"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	LocalCacheSize  int           `json:"local_cache_size"`
}

// Recipe-specific cache methods
func (c *CacheManager) GetRecipe(ctx context.Context, recipeID string) (*Recipe, error) {
	key := fmt.Sprintf("recipe:%s", recipeID)
	var recipe Recipe
	err := c.Get(ctx, key, &recipe)
	return &recipe, err
}

func (c *CacheManager) SetRecipe(ctx context.Context, recipe *Recipe) error {
	key := fmt.Sprintf("recipe:%s", recipe.ID)
	return c.Set(ctx, key, recipe, 1*time.Hour)
}

func (c *CacheManager) GetRecipeList(ctx context.Context, page, limit int, filters string) (*RecipeList, error) {
	key := fmt.Sprintf("recipes:list:%d:%d:%s", page, limit, filters)
	var list RecipeList
	err := c.Get(ctx, key, &list)
	return &list, err
}

func (c *CacheManager) SetRecipeList(ctx context.Context, list *RecipeList, page, limit int, filters string) error {
	key := fmt.Sprintf("recipes:list:%d:%d:%s", page, limit, filters)
	return c.Set(ctx, key, list, 15*time.Minute)
}

func (c *CacheManager) InvalidateRecipeCache(ctx context.Context, recipeID string) error {
	// Invalidate specific recipe
	err := c.Delete(ctx, fmt.Sprintf("recipe:%s", recipeID))
	if err != nil {
		return err
	}

	// Invalidate recipe lists
	return c.Invalidate(ctx, "recipes:list:*")
}

// User-specific cache methods
func (c *CacheManager) GetUserProfile(ctx context.Context, userID string) (*UserProfile, error) {
	key := fmt.Sprintf("user:profile:%s", userID)
	var profile UserProfile
	err := c.Get(ctx, key, &profile)
	return &profile, err
}

func (c *CacheManager) SetUserProfile(ctx context.Context, profile *UserProfile) error {
	key := fmt.Sprintf("user:profile:%s", profile.ID)
	return c.Set(ctx, key, profile, 30*time.Minute)
}

// Session cache methods
func (c *CacheManager) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	var session Session
	err := c.Get(ctx, key, &session)
	return &session, err
}

func (c *CacheManager) SetSession(ctx context.Context, session *Session) error {
	key := fmt.Sprintf("session:%s", session.ID)
	return c.Set(ctx, key, session, 24*time.Hour)
}

// Rate limiting cache methods
func (c *CacheManager) GetRateLimit(ctx context.Context, key string) (int, error) {
	var count int
	err := c.Get(ctx, fmt.Sprintf("ratelimit:%s", key), &count)
	return count, err
}

func (c *CacheManager) IncrementRateLimit(ctx context.Context, key string, window time.Duration) (int, error) {
	redisKey := fmt.Sprintf("ratelimit:%s", key)
	
	// Use cache service for rate limiting (simplified approach)
	var currentCount int
	if data, err := c.cacheService.Get(ctx, redisKey); err == nil {
		json.Unmarshal([]byte(data), &currentCount)
	}
	currentCount++
	
	countData, _ := json.Marshal(currentCount)
	err := c.cacheService.Set(ctx, redisKey, string(countData), window)
	if err != nil {
		return 0, err
	}
	
	return currentCount, nil
}

// Errors
var (
	ErrCacheKeyNotFound = fmt.Errorf("cache key not found")
)

// Data structures for caching
type Recipe struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Ingredients []string  `json:"ingredients"`
	CreatedAt   time.Time `json:"created_at"`
}

type RecipeList struct {
	Recipes []Recipe `json:"recipes"`
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	Limit   int      `json:"limit"`
}

type UserProfile struct {
	ID       string    `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Settings map[string]interface{} `json:"settings"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Data      map[string]interface{} `json:"data"`
}