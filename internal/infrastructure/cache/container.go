// Package cache provides dependency injection container for cache services
package cache

import (
	"context"
	"fmt"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"go.uber.org/zap"
)

// Container holds all cache-related services and dependencies
type Container struct {
	// Core infrastructure
	RedisClient    *RedisClient
	CacheService   *CacheService
	CacheRepo      outbound.CacheRepository
	
	// Specialized services
	RecipeCache    *RecipeCacheService
	SessionCache   *SessionCacheService
	AICache        *AICacheService
	TemplateCache  *TemplateCacheService
	
	// Middleware and monitoring
	HTTPMiddleware *HTTPCacheMiddleware
	Monitor        *CacheMonitor
	
	// Configuration
	Config         *config.Config
	Logger         *zap.Logger
}

// NewContainer creates and wires up all cache services
func NewContainer(cfg *config.Config, logger *zap.Logger) (*Container, error) {
	container := &Container{
		Config: cfg,
		Logger: logger,
	}
	
	// Initialize Redis client
	redisClient, err := NewRedisClient(&cfg.Redis, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis client: %w", err)
	}
	container.RedisClient = redisClient
	
	// Initialize core cache service
	cacheConfig := DefaultCacheConfig()
	cacheConfig.DefaultTTL = cfg.Redis.ConnMaxLifetime
	container.CacheService = NewCacheService(redisClient, cacheConfig, logger)
	
	// Initialize cache repository
	container.CacheRepo = NewCacheRepository(container.CacheService, logger)
	
	// Initialize specialized cache services
	container.RecipeCache = NewRecipeCacheService(container.CacheService, logger)
	container.SessionCache = NewSessionCacheService(container.CacheService, logger)
	container.AICache = NewAICacheService(container.CacheService, logger)
	container.TemplateCache = NewTemplateCacheService(container.CacheService, logger)
	
	// Initialize HTTP middleware
	container.HTTPMiddleware = NewHTTPCacheMiddleware(container.CacheService, logger)
	
	// Initialize monitoring
	container.Monitor = NewCacheMonitor(container.CacheService, redisClient, logger)
	
	// Start monitoring if enabled
	if cfg.Monitoring.EnableMetrics {
		if err := container.Monitor.Start(); err != nil {
			logger.Error("Failed to start cache monitor", zap.Error(err))
		}
	}
	
	logger.Info("Cache container initialized successfully",
		zap.String("redis_host", cfg.Redis.Host),
		zap.Int("redis_port", cfg.Redis.Port),
		zap.Bool("monitoring_enabled", cfg.Monitoring.EnableMetrics))
	
	return container, nil
}

// Close gracefully shuts down all cache services
func (c *Container) Close() error {
	var errors []error
	
	// Stop monitoring
	if c.Monitor != nil {
		if err := c.Monitor.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop monitor: %w", err))
		}
	}
	
	// Close Redis client
	if c.RedisClient != nil {
		if err := c.RedisClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close Redis client: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("errors during cache container shutdown: %v", errors)
	}
	
	c.Logger.Info("Cache container closed successfully")
	return nil
}

// GetHealthStatus returns the health status of all cache services
func (c *Container) GetHealthStatus() *CacheHealthReport {
	report := &CacheHealthReport{
		Overall: "healthy",
		Services: make(map[string]ServiceHealthStatus),
	}
	
	// Check Redis health
	if redisHealth := c.RedisClient.GetHealthStatus(); redisHealth.IsHealthy {
		report.Services["redis"] = ServiceHealthStatus{
			Status:  "healthy",
			Message: "Redis connection active",
		}
	} else {
		report.Services["redis"] = ServiceHealthStatus{
			Status:  "unhealthy",
			Message: redisHealth.LastError,
		}
		report.Overall = "unhealthy"
	}
	
	// Check cache service health
	cacheStats := c.CacheService.GetStats()
	if cacheStats.HitRatio > 0.5 && cacheStats.TotalErrors < cacheStats.TotalOperations/10 {
		report.Services["cache"] = ServiceHealthStatus{
			Status:  "healthy",
			Message: fmt.Sprintf("Hit ratio: %.2f%%", cacheStats.HitRatio*100),
		}
	} else {
		report.Services["cache"] = ServiceHealthStatus{
			Status:  "degraded",
			Message: fmt.Sprintf("Hit ratio: %.2f%%, Errors: %d", cacheStats.HitRatio*100, cacheStats.TotalErrors),
		}
		if report.Overall == "healthy" {
			report.Overall = "degraded"
		}
	}
	
	// Get monitor health if available
	if c.Monitor != nil {
		if monitorHealth := c.Monitor.GetHealthStatus(); monitorHealth.Overall == HealthStatusHealthy {
			report.Services["monitor"] = ServiceHealthStatus{
				Status:  "healthy",
				Message: "Cache monitoring active",
			}
		} else {
			report.Services["monitor"] = ServiceHealthStatus{
				Status:  string(monitorHealth.Overall),
				Message: "Cache monitoring issues detected",
			}
		}
	}
	
	return report
}

// GetMetrics returns comprehensive cache metrics
func (c *Container) GetMetrics() *ComprehensiveMetrics {
	metrics := &ComprehensiveMetrics{}
	
	// Core cache metrics
	if c.CacheService != nil {
		metrics.Cache = c.CacheService.GetStats()
	}
	
	// Redis metrics
	if c.RedisClient != nil {
		metrics.Redis = c.RedisClient.GetMetrics()
	}
	
	// Monitoring metrics
	if c.Monitor != nil {
		metrics.Monitoring = c.Monitor.GetMetrics()
	}
	
	return metrics
}

// CacheHealthReport represents the health status of cache services
type CacheHealthReport struct {
	Overall  string                           `json:"overall"`
	Services map[string]ServiceHealthStatus   `json:"services"`
}

// ServiceHealthStatus represents individual service health
type ServiceHealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ComprehensiveMetrics contains metrics from all cache services
type ComprehensiveMetrics struct {
	Cache      *CacheStats       `json:"cache,omitempty"`
	Redis      *RedisMetrics     `json:"redis,omitempty"`
	Monitoring *AggregatedMetrics `json:"monitoring,omitempty"`
}

// CacheManagerInterface defines the interface for cache management operations
type CacheManagerInterface interface {
	// Core operations
	InvalidateAll() error
	WarmupCache() error
	
	// Service-specific operations
	InvalidateRecipes() error
	InvalidateUsers() error
	InvalidateAI() error
	InvalidateTemplates() error
	
	// Monitoring
	GetStats() *ComprehensiveMetrics
	GetHealth() *CacheHealthReport
}

// Implement CacheManagerInterface

// InvalidateAll clears all cache data
func (c *Container) InvalidateAll() error {
	c.Logger.Info("Invalidating all cache data")
	
	ctx := context.Background()
	
	// This would be a Redis FLUSHDB in production
	// For safety, we'll invalidate by patterns instead
	patterns := []string{
		"alchemorsel:v3:*",
	}
	
	for _, pattern := range patterns {
		if err := c.CacheService.InvalidateByPattern(ctx, pattern); err != nil {
			c.Logger.Error("Failed to invalidate pattern", 
				zap.String("pattern", pattern), 
				zap.Error(err))
		}
	}
	
	return nil
}

// WarmupCache preloads frequently accessed data
func (c *Container) WarmupCache() error {
	c.Logger.Info("Starting cache warmup")
	
	// This would typically load popular recipes, common searches, etc.
	// Implementation would depend on application metrics and usage patterns
	
	c.Logger.Info("Cache warmup completed")
	return nil
}

// InvalidateRecipes clears recipe-related cache
func (c *Container) InvalidateRecipes() error {
	ctx := context.Background()
	return c.CacheService.InvalidateByTag(ctx, "recipe")
}

// InvalidateUsers clears user-related cache
func (c *Container) InvalidateUsers() error {
	ctx := context.Background()
	return c.CacheService.InvalidateByTag(ctx, "user")
}

// InvalidateAI clears AI-related cache
func (c *Container) InvalidateAI() error {
	ctx := context.Background()
	return c.CacheService.InvalidateByTag(ctx, "ai")
}

// InvalidateTemplates clears template cache
func (c *Container) InvalidateTemplates() error {
	ctx := context.Background()
	return c.CacheService.InvalidateByTag(ctx, "template")
}

// GetStats returns comprehensive metrics
func (c *Container) GetStats() *ComprehensiveMetrics {
	return c.GetMetrics()
}

// GetHealth returns health status
func (c *Container) GetHealth() *CacheHealthReport {
	return c.GetHealthStatus()
}