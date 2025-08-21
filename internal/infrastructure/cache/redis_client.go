// Package cache provides Redis caching infrastructure for Alchemorsel v3
// Implements ADR-0007: Redis Caching Strategy for cache-first architecture
package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisClient provides Redis connection management with cluster support
type RedisClient struct {
	client         redis.UniversalClient
	config         *config.RedisConfig
	logger         *zap.Logger
	metrics        *RedisMetrics
	healthCheck    *HealthCheck
	circuitBreaker *CircuitBreaker
	mu             sync.RWMutex
}

// RedisMetrics tracks Redis performance and health
type RedisMetrics struct {
	TotalCommands    int64         `json:"total_commands"`
	SuccessfulOps    int64         `json:"successful_ops"`
	FailedOps        int64         `json:"failed_ops"`
	AvgResponseTime  time.Duration `json:"avg_response_time"`
	ConnectionErrors int64         `json:"connection_errors"`
	CacheHits        int64         `json:"cache_hits"`
	CacheMisses      int64         `json:"cache_misses"`
	LastUpdate       time.Time     `json:"last_update"`
	mu               sync.RWMutex
}

// HealthCheck monitors Redis connection health
type HealthCheck struct {
	IsHealthy      bool      `json:"is_healthy"`
	LastCheck      time.Time `json:"last_check"`
	LastError      string    `json:"last_error,omitempty"`
	CheckInterval  time.Duration
	timeout        time.Duration
	checkTicker    *time.Ticker
	stopChan       chan struct{}
	mu             sync.RWMutex
}

// CircuitBreaker implements circuit breaker pattern for Redis
type CircuitBreaker struct {
	maxFailures     int
	timeout         time.Duration
	failures        int
	lastFailureTime time.Time
	state           CircuitState
	mu              sync.RWMutex
}

// CircuitState represents circuit breaker states
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewRedisClient creates a new Redis client with comprehensive configuration
func NewRedisClient(cfg *config.RedisConfig, logger *zap.Logger) (*RedisClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config cannot be nil")
	}

	// Create Redis options
	opts := &redis.UniversalOptions{
		Addrs:        []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Password:     cfg.Password,
		DB:           cfg.Database,
		MaxRetries:   cfg.MaxRetries,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxIdleConns: cfg.MaxIdleConns,
		
		// Connection timeouts
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		
		// Connection lifecycle
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: time.Minute * 5,
		
		// Connection pool settings
		PoolTimeout: time.Second * 10,
	}

	// Configure cluster mode if enabled
	if cfg.EnableCluster && len(cfg.ClusterNodes) > 0 {
		opts.Addrs = cfg.ClusterNodes
		logger.Info("Redis cluster mode enabled", zap.Strings("nodes", cfg.ClusterNodes))
	}

	// Create Redis client
	client := redis.NewUniversalClient(opts)

	// Initialize Redis client wrapper
	redisClient := &RedisClient{
		client:  client,
		config:  cfg,
		logger:  logger,
		metrics: &RedisMetrics{LastUpdate: time.Now()},
		healthCheck: &HealthCheck{
			CheckInterval: time.Second * 30,
			timeout:       time.Second * 5,
			stopChan:      make(chan struct{}),
		},
		circuitBreaker: &CircuitBreaker{
			maxFailures: 5,
			timeout:     time.Second * 30,
			state:       CircuitClosed,
		},
	}

	// Test initial connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if err := redisClient.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Start health check monitoring
	redisClient.startHealthCheck()

	logger.Info("Redis client initialized successfully",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.Int("database", cfg.Database),
		zap.Bool("cluster_enabled", cfg.EnableCluster))

	return redisClient, nil
}

// Ping tests Redis connection
func (r *RedisClient) Ping(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		return fmt.Errorf("redis circuit breaker is open")
	}

	start := time.Now()
	err := r.client.Ping(ctx).Err()
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.logger.Error("Redis ping failed", zap.Error(err))
		return err
	}

	r.circuitBreaker.RecordSuccess()
	return nil
}

// Get retrieves a value from Redis with circuit breaker protection
func (r *RedisClient) Get(ctx context.Context, key string) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		r.metrics.incrementCacheMiss()
		return nil, fmt.Errorf("redis circuit breaker is open")
	}

	start := time.Now()
	result, err := r.client.Get(ctx, key).Bytes()
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err == redis.Nil {
		r.metrics.incrementCacheMiss()
		return nil, ErrKeyNotFound
	}

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.metrics.incrementCacheMiss()
		r.logger.Error("Redis GET failed", zap.String("key", key), zap.Error(err))
		return nil, err
	}

	r.circuitBreaker.RecordSuccess()
	r.metrics.incrementCacheHit()
	return result, nil
}

// Set stores a value in Redis with TTL
func (r *RedisClient) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		return fmt.Errorf("redis circuit breaker is open")
	}

	start := time.Now()
	err := r.client.Set(ctx, key, value, ttl).Err()
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.logger.Error("Redis SET failed", zap.String("key", key), zap.Error(err))
		return err
	}

	r.circuitBreaker.RecordSuccess()
	return nil
}

// SetNX sets a key only if it doesn't exist (atomic operation)
func (r *RedisClient) SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		return false, fmt.Errorf("redis circuit breaker is open")
	}

	start := time.Now()
	result, err := r.client.SetNX(ctx, key, value, ttl).Result()
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.logger.Error("Redis SETNX failed", zap.String("key", key), zap.Error(err))
		return false, err
	}

	r.circuitBreaker.RecordSuccess()
	return result, nil
}

// Delete removes a key from Redis
func (r *RedisClient) Delete(ctx context.Context, keys ...string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		return fmt.Errorf("redis circuit breaker is open")
	}

	start := time.Now()
	err := r.client.Del(ctx, keys...).Err()
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.logger.Error("Redis DEL failed", zap.Strings("keys", keys), zap.Error(err))
		return err
	}

	r.circuitBreaker.RecordSuccess()
	return nil
}

// Exists checks if keys exist in Redis
func (r *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		return 0, fmt.Errorf("redis circuit breaker is open")
	}

	start := time.Now()
	result, err := r.client.Exists(ctx, keys...).Result()
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.logger.Error("Redis EXISTS failed", zap.Strings("keys", keys), zap.Error(err))
		return 0, err
	}

	r.circuitBreaker.RecordSuccess()
	return result, nil
}

// MGet retrieves multiple values efficiently using pipeline
func (r *RedisClient) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		for range keys {
			r.metrics.incrementCacheMiss()
		}
		return nil, fmt.Errorf("redis circuit breaker is open")
	}

	start := time.Now()
	results, err := r.client.MGet(ctx, keys...).Result()
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		for range keys {
			r.metrics.incrementCacheMiss()
		}
		r.logger.Error("Redis MGET failed", zap.Strings("keys", keys), zap.Error(err))
		return nil, err
	}

	r.circuitBreaker.RecordSuccess()

	// Process results
	resultMap := make(map[string][]byte)
	for i, key := range keys {
		if i < len(results) && results[i] != nil {
			if str, ok := results[i].(string); ok {
				resultMap[key] = []byte(str)
				r.metrics.incrementCacheHit()
			} else {
				r.metrics.incrementCacheMiss()
			}
		} else {
			r.metrics.incrementCacheMiss()
		}
	}

	return resultMap, nil
}

// MSet stores multiple key-value pairs efficiently using pipeline
func (r *RedisClient) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		return fmt.Errorf("redis circuit breaker is open")
	}

	pipe := r.client.Pipeline()
	for key, value := range items {
		pipe.Set(ctx, key, value, ttl)
	}

	start := time.Now()
	_, err := pipe.Exec(ctx)
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.logger.Error("Redis MSET failed", zap.Int("items", len(items)), zap.Error(err))
		return err
	}

	r.circuitBreaker.RecordSuccess()
	return nil
}

// Increment atomically increments a counter
func (r *RedisClient) Increment(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		return 0, fmt.Errorf("redis circuit breaker is open")
	}

	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, expiration)

	start := time.Now()
	_, err := pipe.Exec(ctx)
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.logger.Error("Redis INCR failed", zap.String("key", key), zap.Error(err))
		return 0, err
	}

	r.circuitBreaker.RecordSuccess()
	return incr.Val(), nil
}

// ScanKeys scans for keys matching a pattern
func (r *RedisClient) ScanKeys(ctx context.Context, pattern string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.circuitBreaker.AllowRequest() {
		return nil, fmt.Errorf("redis circuit breaker is open")
	}

	var keys []string
	start := time.Now()
	
	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	
	err := iter.Err()
	duration := time.Since(start)

	r.updateMetrics(err, duration)

	if err != nil {
		r.circuitBreaker.RecordFailure()
		r.logger.Error("Redis SCAN failed", zap.String("pattern", pattern), zap.Error(err))
		return nil, err
	}

	r.circuitBreaker.RecordSuccess()
	return keys, nil
}

// GetMetrics returns current Redis metrics
func (r *RedisClient) GetMetrics() *RedisMetrics {
	r.metrics.mu.RLock()
	defer r.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	return &RedisMetrics{
		TotalCommands:    r.metrics.TotalCommands,
		SuccessfulOps:    r.metrics.SuccessfulOps,
		FailedOps:        r.metrics.FailedOps,
		AvgResponseTime:  r.metrics.AvgResponseTime,
		ConnectionErrors: r.metrics.ConnectionErrors,
		CacheHits:        r.metrics.CacheHits,
		CacheMisses:      r.metrics.CacheMisses,
		LastUpdate:       r.metrics.LastUpdate,
	}
}

// GetHealthStatus returns health check status
func (r *RedisClient) GetHealthStatus() *HealthCheck {
	r.healthCheck.mu.RLock()
	defer r.healthCheck.mu.RUnlock()

	return &HealthCheck{
		IsHealthy: r.healthCheck.IsHealthy,
		LastCheck: r.healthCheck.LastCheck,
		LastError: r.healthCheck.LastError,
	}
}

// Close closes the Redis client connection
func (r *RedisClient) Close() error {
	// Stop health check
	close(r.healthCheck.stopChan)
	if r.healthCheck.checkTicker != nil {
		r.healthCheck.checkTicker.Stop()
	}

	// Close Redis connection
	return r.client.Close()
}

// Internal helper methods

func (r *RedisClient) updateMetrics(err error, duration time.Duration) {
	r.metrics.mu.Lock()
	defer r.metrics.mu.Unlock()

	r.metrics.TotalCommands++
	if err != nil {
		r.metrics.FailedOps++
		if err != redis.Nil {
			r.metrics.ConnectionErrors++
		}
	} else {
		r.metrics.SuccessfulOps++
	}

	// Update average response time using exponential moving average
	if r.metrics.TotalCommands == 1 {
		r.metrics.AvgResponseTime = duration
	} else {
		// α = 0.1 for exponential moving average
		alpha := 0.1
		r.metrics.AvgResponseTime = time.Duration(float64(r.metrics.AvgResponseTime)*(1-alpha) + float64(duration)*alpha)
	}

	r.metrics.LastUpdate = time.Now()
}

func (r *RedisClient) startHealthCheck() {
	r.healthCheck.checkTicker = time.NewTicker(r.healthCheck.CheckInterval)

	go func() {
		for {
			select {
			case <-r.healthCheck.checkTicker.C:
				r.performHealthCheck()
			case <-r.healthCheck.stopChan:
				return
			}
		}
	}()
}

func (r *RedisClient) performHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), r.healthCheck.timeout)
	defer cancel()

	err := r.Ping(ctx)

	r.healthCheck.mu.Lock()
	r.healthCheck.LastCheck = time.Now()
	r.healthCheck.IsHealthy = err == nil
	if err != nil {
		r.healthCheck.LastError = err.Error()
	} else {
		r.healthCheck.LastError = ""
	}
	r.healthCheck.mu.Unlock()
}

// Circuit breaker methods

// AllowRequest checks if requests are allowed based on circuit state
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = CircuitHalfOpen
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = CircuitClosed
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailureTime = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// Metrics helper methods

func (m *RedisMetrics) incrementCacheHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CacheHits++
}

func (m *RedisMetrics) incrementCacheMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CacheMisses++
}

// GetCacheHitRatio calculates cache hit ratio
func (m *RedisMetrics) GetCacheHitRatio() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.CacheHits + m.CacheMisses
	if total == 0 {
		return 0.0
	}
	return float64(m.CacheHits) / float64(total)
}

// Common errors
var (
	ErrKeyNotFound = fmt.Errorf("key not found in cache")
)