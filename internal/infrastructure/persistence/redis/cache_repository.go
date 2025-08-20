// Package redis provides Redis repository implementations with cache-first architecture
package redis

import (
	"context"
	"time"
	
	"github.com/alchemorsel/v3/internal/infrastructure/cache"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"go.uber.org/zap"
)

// CacheRepository implements the cache repository interface using cache-first pattern
type CacheRepository struct {
	cacheService *cache.CacheService
	logger       *zap.Logger
}

// NewCacheRepository creates a new cache repository with cache-first implementation
func NewCacheRepository(cacheService *cache.CacheService, logger *zap.Logger) outbound.CacheRepository {
	return &CacheRepository{
		cacheService: cacheService,
		logger:       logger,
	}
}

// Get retrieves a value from cache using cache-first pattern
func (r *CacheRepository) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := r.cacheService.Get(ctx, key)
	if err != nil {
		r.logger.Debug("Cache get failed", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return data, nil
}

// Set stores a value in cache with TTL
func (r *CacheRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	err := r.cacheService.Set(ctx, key, value, ttl)
	if err != nil {
		r.logger.Error("Cache set failed", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

// Delete removes a value from cache
func (r *CacheRepository) Delete(ctx context.Context, key string) error {
	err := r.cacheService.Delete(ctx, key)
	if err != nil {
		r.logger.Error("Cache delete failed", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

// Exists checks if keys exist in cache
func (r *CacheRepository) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := r.cacheService.Exists(ctx, key)
	if err != nil {
		r.logger.Error("Cache exists check failed", zap.String("key", key), zap.Error(err))
		return false, err
	}
	return exists[key], nil
}

// MGet retrieves multiple values efficiently
func (r *CacheRepository) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	result, err := r.cacheService.MGet(ctx, keys)
	if err != nil {
		r.logger.Error("Cache mget failed", zap.Strings("keys", keys), zap.Error(err))
		return nil, err
	}
	return result, nil
}

// MSet stores multiple values efficiently
func (r *CacheRepository) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	err := r.cacheService.MSet(ctx, items, ttl)
	if err != nil {
		r.logger.Error("Cache mset failed", zap.Int("count", len(items)), zap.Error(err))
		return err
	}
	return nil
}

// Increment atomically increments a counter
func (r *CacheRepository) Increment(ctx context.Context, key string) (int64, error) {
	// Default 1 hour expiration for counters
	return r.cacheService.redis.Increment(ctx, key, time.Hour)
}

// Decrement atomically decrements a counter
func (r *CacheRepository) Decrement(ctx context.Context, key string) (int64, error) {
	// Use Redis DECR operation through the cache service
	result, err := r.cacheService.redis.client.Decr(ctx, key).Result()
	if err != nil {
		r.logger.Error("Cache decrement failed", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return result, nil
}

// SAdd adds members to a set
func (r *CacheRepository) SAdd(ctx context.Context, key string, members ...string) error {
	err := r.cacheService.redis.client.SAdd(ctx, key, members).Err()
	if err != nil {
		r.logger.Error("Cache sadd failed", zap.String("key", key), zap.Strings("members", members), zap.Error(err))
		return err
	}
	return nil
}

// SMembers retrieves all members of a set
func (r *CacheRepository) SMembers(ctx context.Context, key string) ([]string, error) {
	members, err := r.cacheService.redis.client.SMembers(ctx, key).Result()
	if err != nil {
		r.logger.Error("Cache smembers failed", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return members, nil
}

// SRem removes members from a set
func (r *CacheRepository) SRem(ctx context.Context, key string, members ...string) error {
	err := r.cacheService.redis.client.SRem(ctx, key, members).Err()
	if err != nil {
		r.logger.Error("Cache srem failed", zap.String("key", key), zap.Strings("members", members), zap.Error(err))
		return err
	}
	return nil
}