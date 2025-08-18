// Package security provides advanced rate limiting and DDoS protection
package security

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RateLimitType represents different types of rate limits
type RateLimitType string

const (
	RateLimitGlobal      RateLimitType = "global"
	RateLimitPerIP       RateLimitType = "per_ip"
	RateLimitPerUser     RateLimitType = "per_user"
	RateLimitPerEndpoint RateLimitType = "per_endpoint"
	RateLimitAPI         RateLimitType = "api"
	RateLimitAuth        RateLimitType = "auth"
	RateLimitUpload      RateLimitType = "upload"
)

// RateLimitConfig defines rate limit configuration
type RateLimitConfig struct {
	Type            RateLimitType `json:"type"`
	Requests        int           `json:"requests"`
	Window          time.Duration `json:"window"`
	BurstSize       int           `json:"burst_size"`
	BlockDuration   time.Duration `json:"block_duration"`
	SkipSuccessful  bool          `json:"skip_successful"`
	SkipPaths       []string      `json:"skip_paths"`
}

// RateLimitService provides rate limiting capabilities
type RateLimitService struct {
	logger      *zap.Logger
	redisClient *redis.Client
	configs     map[RateLimitType]RateLimitConfig
}

// NewRateLimitService creates a new rate limiting service
func NewRateLimitService(logger *zap.Logger, redisClient *redis.Client) *RateLimitService {
	service := &RateLimitService{
		logger:      logger,
		redisClient: redisClient,
		configs:     make(map[RateLimitType]RateLimitConfig),
	}
	
	// Initialize default configurations
	service.initializeDefaultConfigs()
	
	return service
}

// initializeDefaultConfigs sets up default rate limit configurations
func (r *RateLimitService) initializeDefaultConfigs() {
	r.configs[RateLimitGlobal] = RateLimitConfig{
		Type:          RateLimitGlobal,
		Requests:      1000,
		Window:        time.Minute,
		BurstSize:     50,
		BlockDuration: 5 * time.Minute,
		SkipPaths:     []string{"/health", "/metrics", "/ready"},
	}
	
	r.configs[RateLimitPerIP] = RateLimitConfig{
		Type:          RateLimitPerIP,
		Requests:      60,
		Window:        time.Minute,
		BurstSize:     10,
		BlockDuration: 15 * time.Minute,
		SkipPaths:     []string{"/health", "/metrics", "/ready"},
	}
	
	r.configs[RateLimitPerUser] = RateLimitConfig{
		Type:          RateLimitPerUser,
		Requests:      100,
		Window:        time.Minute,
		BurstSize:     20,
		BlockDuration: 10 * time.Minute,
		SkipSuccessful: true,
	}
	
	r.configs[RateLimitAuth] = RateLimitConfig{
		Type:          RateLimitAuth,
		Requests:      5,
		Window:        time.Minute,
		BurstSize:     2,
		BlockDuration: 30 * time.Minute,
	}
	
	r.configs[RateLimitAPI] = RateLimitConfig{
		Type:          RateLimitAPI,
		Requests:      1000,
		Window:        time.Hour,
		BurstSize:     100,
		BlockDuration: time.Hour,
	}
	
	r.configs[RateLimitUpload] = RateLimitConfig{
		Type:          RateLimitUpload,
		Requests:      10,
		Window:        time.Minute,
		BurstSize:     2,
		BlockDuration: 10 * time.Minute,
	}
}

// RateLimitMiddleware creates rate limiting middleware
func (r *RateLimitService) RateLimitMiddleware(limitType RateLimitType) gin.HandlerFunc {
	return func(c *gin.Context) {
		config, exists := r.configs[limitType]
		if !exists {
			r.logger.Warn("Rate limit config not found", zap.String("type", string(limitType)))
			c.Next()
			return
		}
		
		// Skip certain paths
		for _, skipPath := range config.SkipPaths {
			if c.Request.URL.Path == skipPath {
				c.Next()
				return
			}
		}
		
		// Generate rate limit key
		key := r.generateRateLimitKey(c, limitType)
		
		// Check if already blocked
		if blocked, err := r.isBlocked(key); err == nil && blocked {
			r.handleRateLimitExceeded(c, config)
			return
		}
		
		// Check rate limit
		allowed, remaining, resetTime, err := r.checkRateLimit(key, config)
		if err != nil {
			r.logger.Error("Rate limit check failed", zap.Error(err))
			c.Next()
			return
		}
		
		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.Requests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
		
		if !allowed {
			// Block for configured duration
			if config.BlockDuration > 0 {
				r.blockKey(key, config.BlockDuration)
			}
			
			r.handleRateLimitExceeded(c, config)
			return
		}
		
		// Process request
		c.Next()
		
		// Record request (skip successful if configured)
		if !config.SkipSuccessful || c.Writer.Status() >= 400 {
			r.recordRequest(key, config)
		}
	}
}

// generateRateLimitKey generates a unique key for rate limiting
func (r *RateLimitService) generateRateLimitKey(c *gin.Context, limitType RateLimitType) string {
	switch limitType {
	case RateLimitGlobal:
		return "rate_limit:global"
	case RateLimitPerIP:
		return fmt.Sprintf("rate_limit:ip:%s", c.ClientIP())
	case RateLimitPerUser:
		userID := c.GetString("user_id")
		if userID == "" {
			return fmt.Sprintf("rate_limit:ip:%s", c.ClientIP())
		}
		return fmt.Sprintf("rate_limit:user:%s", userID)
	case RateLimitPerEndpoint:
		return fmt.Sprintf("rate_limit:endpoint:%s:%s", c.Request.Method, c.FullPath())
	case RateLimitAuth:
		return fmt.Sprintf("rate_limit:auth:%s", c.ClientIP())
	case RateLimitAPI:
		userID := c.GetString("user_id")
		if userID == "" {
			return fmt.Sprintf("rate_limit:api:ip:%s", c.ClientIP())
		}
		return fmt.Sprintf("rate_limit:api:user:%s", userID)
	case RateLimitUpload:
		userID := c.GetString("user_id")
		if userID == "" {
			return fmt.Sprintf("rate_limit:upload:ip:%s", c.ClientIP())
		}
		return fmt.Sprintf("rate_limit:upload:user:%s", userID)
	default:
		return fmt.Sprintf("rate_limit:unknown:%s", c.ClientIP())
	}
}

// checkRateLimit checks if request is within rate limit using sliding window
func (r *RateLimitService) checkRateLimit(key string, config RateLimitConfig) (bool, int, time.Time, error) {
	ctx := context.Background()
	now := time.Now()
	windowStart := now.Add(-config.Window)
	
	// Use Redis sorted set for sliding window
	pipe := r.redisClient.TxPipeline()
	
	// Remove old entries
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
	
	// Count current requests in window
	pipe.ZCard(ctx, key)
	
	// Add current request
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})
	
	// Set expiration
	pipe.Expire(ctx, key, config.Window*2)
	
	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, now, fmt.Errorf("rate limit check failed: %w", err)
	}
	
	// Get count from results
	count := results[1].(*redis.IntCmd).Val()
	
	// Check if within limit
	allowed := count <= int64(config.Requests)
	remaining := config.Requests - int(count)
	if remaining < 0 {
		remaining = 0
	}
	
	resetTime := now.Add(config.Window)
	
	return allowed, remaining, resetTime, nil
}

// recordRequest records a request for rate limiting
func (r *RateLimitService) recordRequest(key string, config RateLimitConfig) {
	ctx := context.Background()
	now := time.Now()
	
	// Add to sorted set
	r.redisClient.ZAdd(ctx, key, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})
	
	// Set expiration
	r.redisClient.Expire(ctx, key, config.Window*2)
}

// blockKey blocks a key for specified duration
func (r *RateLimitService) blockKey(key string, duration time.Duration) {
	ctx := context.Background()
	blockKey := fmt.Sprintf("%s:blocked", key)
	
	r.redisClient.Set(ctx, blockKey, "1", duration)
}

// isBlocked checks if a key is currently blocked
func (r *RateLimitService) isBlocked(key string) (bool, error) {
	ctx := context.Background()
	blockKey := fmt.Sprintf("%s:blocked", key)
	
	exists, err := r.redisClient.Exists(ctx, blockKey).Result()
	return exists > 0, err
}

// handleRateLimitExceeded handles rate limit exceeded responses
func (r *RateLimitService) handleRateLimitExceeded(c *gin.Context, config RateLimitConfig) {
	r.logger.Warn("Rate limit exceeded",
		zap.String("ip", c.ClientIP()),
		zap.String("user_id", c.GetString("user_id")),
		zap.String("path", c.Request.URL.Path),
		zap.String("user_agent", c.Request.UserAgent()),
		zap.String("type", string(config.Type)),
	)
	
	c.Header("Retry-After", strconv.Itoa(int(config.BlockDuration.Seconds())))
	
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error":   "Rate limit exceeded",
		"message": "Too many requests. Please try again later.",
		"retry_after": config.BlockDuration.Seconds(),
	})
	c.Abort()
}

// DDoSProtectionMiddleware provides DDoS protection
func (r *RateLimitService) DDoSProtectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		
		// Check for rapid fire requests (potential bot)
		rapidFireKey := fmt.Sprintf("rapid_fire:%s", ip)
		rapidFireCount, err := r.redisClient.Incr(context.Background(), rapidFireKey).Result()
		if err == nil {
			r.redisClient.Expire(context.Background(), rapidFireKey, 10*time.Second)
			
			// If more than 20 requests in 10 seconds, block for 1 hour
			if rapidFireCount > 20 {
				r.blockKey(fmt.Sprintf("rate_limit:ip:%s", ip), time.Hour)
				
				r.logger.Warn("DDoS protection triggered - rapid fire",
					zap.String("ip", ip),
					zap.Int64("count", rapidFireCount),
				)
				
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "DDoS protection activated",
				})
				c.Abort()
				return
			}
		}
		
		// Check for suspicious user agents
		userAgent := c.Request.UserAgent()
		if r.isSuspiciousUserAgent(userAgent) {
			r.logger.Warn("Suspicious user agent detected",
				zap.String("ip", ip),
				zap.String("user_agent", userAgent),
			)
			
			// Apply stricter rate limiting
			suspiciousKey := fmt.Sprintf("rate_limit:suspicious:%s", ip)
			allowed, _, _, _ := r.checkRateLimit(suspiciousKey, RateLimitConfig{
				Requests:      10,
				Window:        time.Minute,
				BlockDuration: 30 * time.Minute,
			})
			
			if !allowed {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded",
				})
				c.Abort()
				return
			}
		}
		
		c.Next()
	}
}

// isSuspiciousUserAgent checks for suspicious user agents
func (r *RateLimitService) isSuspiciousUserAgent(userAgent string) bool {
	suspiciousPatterns := []string{
		"bot", "crawler", "spider", "scraper", "scanner",
		"curl", "wget", "python", "go-http-client",
		"masscan", "nmap", "sqlmap", "nikto", "burp",
		"postman", "insomnia", // API testing tools
	}
	
	userAgentLower := strings.ToLower(userAgent)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(userAgentLower, pattern) {
			return true
		}
	}
	
	return false
}

// GetRateLimitStats returns rate limit statistics
func (r *RateLimitService) GetRateLimitStats(limitType RateLimitType, identifier string) (map[string]interface{}, error) {
	config, exists := r.configs[limitType]
	if !exists {
		return nil, fmt.Errorf("rate limit config not found")
	}
	
	key := fmt.Sprintf("rate_limit:%s:%s", limitType, identifier)
	ctx := context.Background()
	
	// Get current count
	now := time.Now()
	windowStart := now.Add(-config.Window)
	
	count, err := r.redisClient.ZCount(ctx, key, strconv.FormatInt(windowStart.UnixNano(), 10), "+inf").Result()
	if err != nil {
		return nil, err
	}
	
	// Check if blocked
	blocked, _ := r.isBlocked(key)
	
	stats := map[string]interface{}{
		"limit":     config.Requests,
		"window":    config.Window.String(),
		"current":   count,
		"remaining": max(0, config.Requests-int(count)),
		"blocked":   blocked,
		"reset_at":  now.Add(config.Window),
	}
	
	return stats, nil
}

// UpdateRateLimitConfig updates rate limit configuration
func (r *RateLimitService) UpdateRateLimitConfig(limitType RateLimitType, config RateLimitConfig) {
	r.configs[limitType] = config
}

// ClearRateLimit clears rate limit for a specific key
func (r *RateLimitService) ClearRateLimit(limitType RateLimitType, identifier string) error {
	key := fmt.Sprintf("rate_limit:%s:%s", limitType, identifier)
	blockKey := fmt.Sprintf("%s:blocked", key)
	
	ctx := context.Background()
	
	// Remove rate limit data and block
	pipe := r.redisClient.TxPipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, blockKey)
	
	_, err := pipe.Exec(ctx)
	return err
}

// WhitelistIP adds an IP to the whitelist (bypasses rate limiting)
func (r *RateLimitService) WhitelistIP(ip string, duration time.Duration) error {
	ctx := context.Background()
	key := fmt.Sprintf("whitelist:ip:%s", ip)
	
	return r.redisClient.Set(ctx, key, "1", duration).Err()
}

// IsWhitelisted checks if an IP is whitelisted
func (r *RateLimitService) IsWhitelisted(ip string) bool {
	ctx := context.Background()
	key := fmt.Sprintf("whitelist:ip:%s", ip)
	
	exists, err := r.redisClient.Exists(ctx, key).Result()
	return err == nil && exists > 0
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}