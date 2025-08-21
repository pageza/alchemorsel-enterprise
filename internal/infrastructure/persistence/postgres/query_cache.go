// Package postgres provides Redis-integrated query result caching
package postgres

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// QueryCache provides intelligent query result caching with Redis
type QueryCache struct {
	redis      *redis.Client
	logger     *zap.Logger
	defaultTTL time.Duration
	enabled    bool
	keyPrefix  string
	metrics    *CacheMetrics
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled            bool          `json:"enabled"`
	DefaultTTL         time.Duration `json:"default_ttl"`
	KeyPrefix          string        `json:"key_prefix"`
	MaxKeyLength       int           `json:"max_key_length"`
	CompressionEnabled bool          `json:"compression_enabled"`
}

// CacheMetrics tracks cache performance
type CacheMetrics struct {
	Hits        int64     `json:"hits"`
	Misses      int64     `json:"misses"`
	Sets        int64     `json:"sets"`
	Deletes     int64     `json:"deletes"`
	Errors      int64     `json:"errors"`
	HitRatio    float64   `json:"hit_ratio"`
	LastUpdated time.Time `json:"last_updated"`
}

// CachedQuery represents a cached query with metadata
type CachedQuery struct {
	SQL         string        `json:"sql"`
	Args        []interface{} `json:"args"`
	Result      interface{}   `json:"result"`
	CachedAt    time.Time     `json:"cached_at"`
	TTL         time.Duration `json:"ttl"`
	AccessCount int64         `json:"access_count"`
	Tags        []string      `json:"tags"`
}

// QueryCacheKey represents a cache key structure
type QueryCacheKey struct {
	Hash      string   `json:"hash"`
	SQL       string   `json:"sql"`
	TableHint string   `json:"table_hint"`
	Tags      []string `json:"tags"`
}

// NewQueryCache creates a new query cache instance
func NewQueryCache(redisClient *redis.Client, logger *zap.Logger, config CacheConfig) *QueryCache {
	if config.KeyPrefix == "" {
		config.KeyPrefix = "alchemorsel:query"
	}

	if config.DefaultTTL == 0 {
		config.DefaultTTL = 5 * time.Minute
	}

	return &QueryCache{
		redis:      redisClient,
		logger:     logger,
		defaultTTL: config.DefaultTTL,
		enabled:    config.Enabled,
		keyPrefix:  config.KeyPrefix,
		metrics:    &CacheMetrics{LastUpdated: time.Now()},
	}
}

// Get retrieves a cached query result
func (qc *QueryCache) Get(ctx context.Context, sql string, args []interface{}, dest interface{}) (bool, error) {
	if !qc.enabled {
		return false, nil
	}

	key := qc.generateCacheKey(sql, args)

	start := time.Now()
	defer func() {
		qc.logger.Debug("Cache get operation",
			zap.String("key", key),
			zap.Duration("duration", time.Since(start)),
		)
	}()

	// Get from Redis
	cached, err := qc.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			qc.incrementMisses()
			return false, nil
		}
		qc.incrementErrors()
		return false, fmt.Errorf("redis get error: %w", err)
	}

	// Deserialize cached result
	var cachedQuery CachedQuery
	if err := json.Unmarshal([]byte(cached), &cachedQuery); err != nil {
		qc.incrementErrors()
		return false, fmt.Errorf("failed to unmarshal cached query: %w", err)
	}

	// Copy result to destination
	if err := qc.copyResult(cachedQuery.Result, dest); err != nil {
		qc.incrementErrors()
		return false, fmt.Errorf("failed to copy cached result: %w", err)
	}

	// Update access count asynchronously
	go qc.updateAccessCount(ctx, key)

	qc.incrementHits()
	qc.logger.Debug("Cache hit",
		zap.String("sql", qc.truncateSQL(sql)),
		zap.String("key", key),
	)

	return true, nil
}

// Set stores a query result in cache
func (qc *QueryCache) Set(ctx context.Context, sql string, args []interface{}, result interface{}, ttl time.Duration) error {
	if !qc.enabled {
		return nil
	}

	if ttl == 0 {
		ttl = qc.defaultTTL
	}

	key := qc.generateCacheKey(sql, args)
	tags := qc.extractTableTags(sql)

	cachedQuery := CachedQuery{
		SQL:         sql,
		Args:        args,
		Result:      result,
		CachedAt:    time.Now(),
		TTL:         ttl,
		AccessCount: 0,
		Tags:        tags,
	}

	serialized, err := json.Marshal(cachedQuery)
	if err != nil {
		qc.incrementErrors()
		return fmt.Errorf("failed to marshal query for cache: %w", err)
	}

	// Store in Redis with TTL
	if err := qc.redis.SetEx(ctx, key, serialized, ttl).Err(); err != nil {
		qc.incrementErrors()
		return fmt.Errorf("redis set error: %w", err)
	}

	// Add to tag indexes for invalidation
	if err := qc.addToTagIndexes(ctx, key, tags, ttl); err != nil {
		qc.logger.Warn("Failed to add to tag indexes", zap.Error(err))
	}

	qc.incrementSets()
	qc.logger.Debug("Cache set",
		zap.String("sql", qc.truncateSQL(sql)),
		zap.String("key", key),
		zap.Duration("ttl", ttl),
		zap.Strings("tags", tags),
	)

	return nil
}

// InvalidateByTags invalidates all cached queries with specific tags
func (qc *QueryCache) InvalidateByTags(ctx context.Context, tags []string) error {
	if !qc.enabled {
		return nil
	}

	var allKeys []string

	for _, tag := range tags {
		tagKey := fmt.Sprintf("%s:tag:%s", qc.keyPrefix, tag)
		keys, err := qc.redis.SMembers(ctx, tagKey).Result()
		if err != nil && err != redis.Nil {
			qc.logger.Error("Failed to get tag members", zap.String("tag", tag), zap.Error(err))
			continue
		}
		allKeys = append(allKeys, keys...)
	}

	if len(allKeys) == 0 {
		return nil
	}

	// Remove duplicates
	uniqueKeys := make(map[string]bool)
	for _, key := range allKeys {
		uniqueKeys[key] = true
	}

	// Delete all keys
	var keysToDelete []string
	for key := range uniqueKeys {
		keysToDelete = append(keysToDelete, key)
	}

	if len(keysToDelete) > 0 {
		if err := qc.redis.Del(ctx, keysToDelete...).Err(); err != nil {
			qc.incrementErrors()
			return fmt.Errorf("failed to delete cached queries: %w", err)
		}

		// Clean up tag indexes
		for _, tag := range tags {
			tagKey := fmt.Sprintf("%s:tag:%s", qc.keyPrefix, tag)
			qc.redis.Del(ctx, tagKey)
		}

		qc.metrics.Deletes += int64(len(keysToDelete))
		qc.logger.Info("Invalidated cached queries by tags",
			zap.Strings("tags", tags),
			zap.Int("keys_deleted", len(keysToDelete)),
		)
	}

	return nil
}

// InvalidateByTable invalidates all cached queries related to a table
func (qc *QueryCache) InvalidateByTable(ctx context.Context, tableName string) error {
	return qc.InvalidateByTags(ctx, []string{tableName})
}

// Clear clears all cached queries
func (qc *QueryCache) Clear(ctx context.Context) error {
	if !qc.enabled {
		return nil
	}

	pattern := fmt.Sprintf("%s:*", qc.keyPrefix)
	keys, err := qc.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get cache keys: %w", err)
	}

	if len(keys) > 0 {
		if err := qc.redis.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}

		qc.logger.Info("Cleared all cached queries", zap.Int("keys_deleted", len(keys)))
	}

	return nil
}

// GetMetrics returns current cache metrics
func (qc *QueryCache) GetMetrics() CacheMetrics {
	qc.updateHitRatio()
	return *qc.metrics
}

// generateCacheKey generates a unique cache key for a query
func (qc *QueryCache) generateCacheKey(sql string, args []interface{}) string {
	// Normalize SQL (remove extra whitespace, convert to lowercase)
	normalizedSQL := strings.ToLower(strings.Join(strings.Fields(sql), " "))

	// Create a hash of SQL + args
	hasher := md5.New()
	hasher.Write([]byte(normalizedSQL))

	// Add arguments to hash
	for _, arg := range args {
		argBytes, _ := json.Marshal(arg)
		hasher.Write(argBytes)
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Extract table hint for better organization
	tableHint := qc.extractTableHint(normalizedSQL)

	return fmt.Sprintf("%s:%s:%s", qc.keyPrefix, tableHint, hash)
}

// extractTableHint extracts the primary table from SQL for key organization
func (qc *QueryCache) extractTableHint(sql string) string {
	sql = strings.ToLower(sql)

	// Try to extract table name from common patterns
	if strings.Contains(sql, "from ") {
		parts := strings.Split(sql, "from ")
		if len(parts) > 1 {
			tablePart := strings.TrimSpace(parts[1])
			tableWords := strings.Fields(tablePart)
			if len(tableWords) > 0 {
				return strings.Trim(tableWords[0], "\"'`")
			}
		}
	}

	if strings.Contains(sql, "update ") {
		parts := strings.Split(sql, "update ")
		if len(parts) > 1 {
			tablePart := strings.TrimSpace(parts[1])
			tableWords := strings.Fields(tablePart)
			if len(tableWords) > 0 {
				return strings.Trim(tableWords[0], "\"'`")
			}
		}
	}

	if strings.Contains(sql, "insert into ") {
		parts := strings.Split(sql, "insert into ")
		if len(parts) > 1 {
			tablePart := strings.TrimSpace(parts[1])
			tableWords := strings.Fields(tablePart)
			if len(tableWords) > 0 {
				return strings.Trim(tableWords[0], "\"'`")
			}
		}
	}

	return "unknown"
}

// extractTableTags extracts table names from SQL for tagging
func (qc *QueryCache) extractTableTags(sql string) []string {
	sql = strings.ToLower(sql)
	var tags []string

	// Common table names in our schema
	tables := []string{
		"users", "recipes", "ingredients", "instructions", "recipe_ratings",
		"recipe_likes", "recipe_views", "collections", "notifications",
		"user_follows", "comments", "activities", "ai_requests",
	}

	for _, table := range tables {
		if strings.Contains(sql, table) {
			tags = append(tags, table)
		}
	}

	return tags
}

// copyResult copies cached result to destination
func (qc *QueryCache) copyResult(src, dest interface{}) error {
	destValue := reflect.ValueOf(dest)

	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be a pointer")
	}

	destValue = destValue.Elem()

	// Convert through JSON for type safety
	jsonData, err := json.Marshal(src)
	if err != nil {
		return err
	}

	return json.Unmarshal(jsonData, dest)
}

// addToTagIndexes adds cache key to tag indexes for invalidation
func (qc *QueryCache) addToTagIndexes(ctx context.Context, key string, tags []string, ttl time.Duration) error {
	for _, tag := range tags {
		tagKey := fmt.Sprintf("%s:tag:%s", qc.keyPrefix, tag)
		if err := qc.redis.SAdd(ctx, tagKey, key).Err(); err != nil {
			return err
		}
		// Set TTL slightly longer than cache TTL to handle cleanup
		qc.redis.Expire(ctx, tagKey, ttl+time.Minute)
	}
	return nil
}

// updateAccessCount updates the access count for a cached query
func (qc *QueryCache) updateAccessCount(ctx context.Context, key string) {
	// This is a best-effort update, so we ignore errors
	qc.redis.HIncrBy(ctx, key+":meta", "access_count", 1)
}

// truncateSQL truncates SQL for logging
func (qc *QueryCache) truncateSQL(sql string) string {
	if len(sql) > 100 {
		return sql[:100] + "..."
	}
	return sql
}

// Metrics update methods
func (qc *QueryCache) incrementHits() {
	qc.metrics.Hits++
	qc.metrics.LastUpdated = time.Now()
}

func (qc *QueryCache) incrementMisses() {
	qc.metrics.Misses++
	qc.metrics.LastUpdated = time.Now()
}

func (qc *QueryCache) incrementSets() {
	qc.metrics.Sets++
	qc.metrics.LastUpdated = time.Now()
}

func (qc *QueryCache) incrementErrors() {
	qc.metrics.Errors++
	qc.metrics.LastUpdated = time.Now()
}

func (qc *QueryCache) updateHitRatio() {
	total := qc.metrics.Hits + qc.metrics.Misses
	if total > 0 {
		qc.metrics.HitRatio = float64(qc.metrics.Hits) / float64(total) * 100
	}
}

// CachedGORMPlugin provides GORM plugin for automatic query caching
type CachedGORMPlugin struct {
	cache *QueryCache
}

// NewCachedGORMPlugin creates a new GORM caching plugin
func NewCachedGORMPlugin(cache *QueryCache) *CachedGORMPlugin {
	return &CachedGORMPlugin{cache: cache}
}

// Name returns the plugin name
func (p *CachedGORMPlugin) Name() string {
	return "query_cache"
}

// Initialize initializes the plugin
func (p *CachedGORMPlugin) Initialize(db *gorm.DB) error {
	// Register callbacks for query caching
	err := db.Callback().Query().Before("gorm:query").Register("cache:before", p.beforeQuery)
	if err != nil {
		return err
	}

	err = db.Callback().Query().After("gorm:query").Register("cache:after", p.afterQuery)
	if err != nil {
		return err
	}

	// Register callbacks for cache invalidation
	err = db.Callback().Create().After("gorm:create").Register("cache:invalidate", p.invalidateCache)
	if err != nil {
		return err
	}

	err = db.Callback().Update().After("gorm:update").Register("cache:invalidate", p.invalidateCache)
	if err != nil {
		return err
	}

	err = db.Callback().Delete().After("gorm:delete").Register("cache:invalidate", p.invalidateCache)
	if err != nil {
		return err
	}

	return nil
}

// beforeQuery checks cache before executing query
func (p *CachedGORMPlugin) beforeQuery(db *gorm.DB) {
	// Skip for non-SELECT queries
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(db.Statement.SQL.String())), "SELECT") {
		return
	}

	// Skip if caching is disabled for this query
	if _, ok := db.InstanceGet("skip_cache"); ok {
		return
	}

	// Try to get from cache
	var result interface{}
	found, err := p.cache.Get(context.Background(), db.Statement.SQL.String(), db.Statement.Vars, &result)
	if err != nil {
		// Log error but continue with normal query
		return
	}

	if found {
		// Set cached result and skip actual query
		db.Statement.Dest = result
		db.InstanceSet("cached_result", true)
	}
}

// afterQuery caches query result after execution
func (p *CachedGORMPlugin) afterQuery(db *gorm.DB) {
	// Skip if result was from cache
	if _, ok := db.InstanceGet("cached_result"); ok {
		return
	}

	// Skip for non-SELECT queries
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(db.Statement.SQL.String())), "SELECT") {
		return
	}

	// Skip if caching is disabled for this query
	if _, ok := db.InstanceGet("skip_cache"); ok {
		return
	}

	// Skip if there was an error
	if db.Error != nil {
		return
	}

	// Cache the result
	ttl := p.cache.defaultTTL
	if customTTL, ok := db.InstanceGet("cache_ttl"); ok {
		if duration, ok := customTTL.(time.Duration); ok {
			ttl = duration
		}
	}

	err := p.cache.Set(context.Background(), db.Statement.SQL.String(), db.Statement.Vars, db.Statement.Dest, ttl)
	if err != nil {
		// Log error but don't fail the query
		p.cache.logger.Error("Failed to cache query result", zap.Error(err))
	}
}

// invalidateCache invalidates cache after data modifications
func (p *CachedGORMPlugin) invalidateCache(db *gorm.DB) {
	if db.Statement == nil || db.Statement.Table == "" {
		return
	}

	// Invalidate cache for the affected table
	err := p.cache.InvalidateByTable(context.Background(), db.Statement.Table)
	if err != nil {
		p.cache.logger.Error("Failed to invalidate cache",
			zap.String("table", db.Statement.Table),
			zap.Error(err))
	}
}
