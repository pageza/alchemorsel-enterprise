// Package ai provides response caching functionality for AI completions
package ai

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CachedResponse represents a cached AI response
type CachedResponse struct {
	Content     string                 `json:"content"`
	Quality     float64                `json:"quality"`
	Model       string                 `json:"model"`
	GeneratedAt time.Time              `json:"generated_at"`
	TokensUsed  int                    `json:"tokens_used"`
	Duration    time.Duration          `json:"duration"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ResponseCache provides Redis-based caching for AI responses
type ResponseCache struct {
	redis      *redis.Client
	logger     *zap.Logger
	keyPrefix  string
	defaultTTL time.Duration
	enabled    bool
}

// NewResponseCache creates a new response cache instance
func NewResponseCache(redis *redis.Client, logger *zap.Logger) *ResponseCache {
	return &ResponseCache{
		redis:      redis,
		logger:     logger.Named("response_cache"),
		keyPrefix:  "alchemorsel:ai:response",
		defaultTTL: time.Hour,
		enabled:    true,
	}
}

// SetEnabled enables or disables caching
func (rc *ResponseCache) SetEnabled(enabled bool) {
	rc.enabled = enabled
	rc.logger.Info("Cache enabled status changed", zap.Bool("enabled", enabled))
}

// GenerateCacheKey generates a cache key for the given messages and model
func (rc *ResponseCache) GenerateCacheKey(messages []conversation.ChatMessage, model string) string {
	hasher := sha256.New()
	
	// Include last 3 messages for context (to avoid excessive cache misses)
	contextMessages := messages
	if len(messages) > 3 {
		contextMessages = messages[len(messages)-3:]
	}
	
	// Hash the conversation context
	for _, msg := range contextMessages {
		hasher.Write([]byte(fmt.Sprintf("%s:%s", msg.Role, msg.Content)))
	}
	hasher.Write([]byte(model))
	
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	return fmt.Sprintf("%s:chat:%s", rc.keyPrefix, hash[:16])
}

// Get retrieves a cached response if it exists
func (rc *ResponseCache) Get(ctx context.Context, messages []conversation.ChatMessage, model string) *conversation.GenerationResult {
	if !rc.enabled {
		return nil
	}
	
	key := rc.GenerateCacheKey(messages, model)
	
	data, err := rc.redis.Get(ctx, key).Result()
	if err != nil {
		if err != redis.Nil {
			rc.logger.Warn("Failed to get cached response",
				zap.String("key", key),
				zap.Error(err))
		}
		return nil
	}
	
	var cached CachedResponse
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		rc.logger.Error("Failed to unmarshal cached response",
			zap.String("key", key),
			zap.Error(err))
		return nil
	}
	
	// Convert cached response to GenerationResult
	result := &conversation.GenerationResult{
		Content:    cached.Content,
		Quality:    cached.Quality,
		Duration:   cached.Duration,
		TokensUsed: cached.TokensUsed,
		ModelUsed:  cached.Model,
		Metadata:   cached.Metadata,
	}
	
	rc.logger.Debug("Cache hit",
		zap.String("key", key),
		zap.String("model", model),
		zap.Time("generated_at", cached.GeneratedAt))
	
	return result
}

// Set stores a response in cache with the specified TTL
func (rc *ResponseCache) Set(ctx context.Context, messages []conversation.ChatMessage, model string, result *conversation.GenerationResult, ttl time.Duration) error {
	if !rc.enabled {
		return nil
	}
	
	key := rc.GenerateCacheKey(messages, model)
	
	cached := CachedResponse{
		Content:     result.Content,
		Quality:     result.Quality,
		Model:       model,
		GeneratedAt: time.Now(),
		TokensUsed:  result.TokensUsed,
		Duration:    result.Duration,
		Metadata:    result.Metadata,
	}
	
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}
	
	if err := rc.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		rc.logger.Error("Failed to cache response",
			zap.String("key", key),
			zap.Error(err))
		return fmt.Errorf("failed to set cache: %w", err)
	}
	
	rc.logger.Debug("Cached response",
		zap.String("key", key),
		zap.String("model", model),
		zap.Duration("ttl", ttl),
		zap.Float64("quality", result.Quality))
	
	return nil
}

// Delete removes a cached response
func (rc *ResponseCache) Delete(ctx context.Context, messages []conversation.ChatMessage, model string) error {
	if !rc.enabled {
		return nil
	}
	
	key := rc.GenerateCacheKey(messages, model)
	
	if err := rc.redis.Del(ctx, key).Err(); err != nil {
		rc.logger.Error("Failed to delete cached response",
			zap.String("key", key),
			zap.Error(err))
		return fmt.Errorf("failed to delete cache: %w", err)
	}
	
	rc.logger.Debug("Deleted cached response", zap.String("key", key))
	return nil
}

// Clear clears all cached responses with the given prefix
func (rc *ResponseCache) Clear(ctx context.Context) error {
	if !rc.enabled {
		return nil
	}
	
	pattern := fmt.Sprintf("%s:*", rc.keyPrefix)
	keys, err := rc.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get cache keys: %w", err)
	}
	
	if len(keys) == 0 {
		rc.logger.Info("No cache entries to clear")
		return nil
	}
	
	if err := rc.redis.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to delete cache keys: %w", err)
	}
	
	rc.logger.Info("Cleared cache entries", zap.Int("count", len(keys)))
	return nil
}

// GetStats returns cache statistics
func (rc *ResponseCache) GetStats(ctx context.Context) (*CacheStats, error) {
	if !rc.enabled {
		return &CacheStats{Enabled: false}, nil
	}
	
	pattern := fmt.Sprintf("%s:*", rc.keyPrefix)
	keys, err := rc.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get cache keys: %w", err)
	}
	
	stats := &CacheStats{
		Enabled:    true,
		TotalKeys:  len(keys),
		KeyPrefix:  rc.keyPrefix,
		DefaultTTL: rc.defaultTTL,
	}
	
	// Get memory usage (rough estimate)
	if len(keys) > 0 {
		// Sample a few keys to estimate average size
		sampleSize := 10
		if len(keys) < sampleSize {
			sampleSize = len(keys)
		}
		
		var totalSize int64
		for i := 0; i < sampleSize; i++ {
			val, err := rc.redis.Get(ctx, keys[i]).Result()
			if err == nil {
				totalSize += int64(len(val))
			}
		}
		
		if sampleSize > 0 {
			avgSize := totalSize / int64(sampleSize)
			stats.EstimatedMemoryUsage = avgSize * int64(len(keys))
		}
	}
	
	return stats, nil
}

// CacheStats represents cache statistics
type CacheStats struct {
	Enabled               bool          `json:"enabled"`
	TotalKeys            int           `json:"total_keys"`
	KeyPrefix            string        `json:"key_prefix"`
	DefaultTTL           time.Duration `json:"default_ttl"`
	EstimatedMemoryUsage int64         `json:"estimated_memory_usage_bytes"`
}

// Cleanup removes expired cache entries (Redis handles this automatically, but this can be used for manual cleanup)
func (rc *ResponseCache) Cleanup(ctx context.Context) error {
	if !rc.enabled {
		return nil
	}
	
	// Redis handles TTL expiration automatically, but we can implement
	// additional cleanup logic here if needed
	
	rc.logger.Debug("Cache cleanup completed (Redis handles TTL automatically)")
	return nil
}

// SetDefaultTTL sets the default TTL for cached responses
func (rc *ResponseCache) SetDefaultTTL(ttl time.Duration) {
	rc.defaultTTL = ttl
	rc.logger.Info("Default TTL updated", zap.Duration("ttl", ttl))
}

// GetTTL returns the TTL for a specific cache key
func (rc *ResponseCache) GetTTL(ctx context.Context, messages []conversation.ChatMessage, model string) (time.Duration, error) {
	if !rc.enabled {
		return 0, fmt.Errorf("cache is disabled")
	}
	
	key := rc.GenerateCacheKey(messages, model)
	ttl, err := rc.redis.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}
	
	return ttl, nil
}

// Exists checks if a cache entry exists
func (rc *ResponseCache) Exists(ctx context.Context, messages []conversation.ChatMessage, model string) (bool, error) {
	if !rc.enabled {
		return false, nil
	}
	
	key := rc.GenerateCacheKey(messages, model)
	count, err := rc.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}
	
	return count > 0, nil
}