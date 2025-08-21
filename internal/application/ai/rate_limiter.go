// Package ai provides comprehensive rate limiting and quota management
package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RateLimiter manages request rate limiting and quota enforcement
type RateLimiter struct {
	config      *EnterpriseConfig
	cacheRepo   outbound.CacheRepository
	logger      *zap.Logger
	
	// In-memory rate limiting (for fallback when cache is unavailable)
	userLimits  map[uuid.UUID]*UserRateLimit
	globalLimit *GlobalRateLimit
	
	// Thread safety
	mu          sync.RWMutex
}

// UserRateLimit tracks rate limits for individual users
type UserRateLimit struct {
	UserID          uuid.UUID
	RequestsMinute  int
	RequestsHour    int
	RequestsDay     int
	WindowMinute    time.Time
	WindowHour      time.Time
	WindowDay       time.Time
	QuotaUsed       int64
	QuotaLimit      int64
	QuotaResetDate  time.Time
	IsBlocked       bool
	BlockedUntil    time.Time
	BlockReason     string
}

// GlobalRateLimit tracks system-wide rate limits
type GlobalRateLimit struct {
	RequestsPerSecond int
	RequestsPerMinute int
	RequestsPerHour   int
	WindowSecond      time.Time
	WindowMinute      time.Time
	WindowHour        time.Time
	TotalRequests     int64
	PeakRPS           int
	PeakRPSTime       time.Time
}

// RateLimitRule defines rate limiting rules
type RateLimitRule struct {
	Name            string
	UserID          *uuid.UUID  // nil for global rules
	Feature         string      // specific feature or "*" for all
	RequestsPerMin  int
	RequestsPerHour int
	RequestsPerDay  int
	QuotaPerMonth   int64
	Priority        int         // higher priority rules are applied first
	StartTime       time.Time
	EndTime         time.Time
	IsActive        bool
}

// RateLimitViolation represents a rate limit violation
type RateLimitViolation struct {
	UserID      uuid.UUID
	Feature     string
	ViolationType string // minute, hour, day, quota
	Limit       int
	Current     int
	ResetTime   time.Time
	Timestamp   time.Time
	Action      string // throttle, block, alert
}

// QuotaInfo provides quota information
type QuotaInfo struct {
	UserID        uuid.UUID
	TotalQuota    int64
	UsedQuota     int64
	RemainingQuota int64
	QuotaPeriod   string
	ResetDate     time.Time
	UsagePercent  float64
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *EnterpriseConfig, cacheRepo outbound.CacheRepository, logger *zap.Logger) *RateLimiter {
	namedLogger := logger.Named("rate-limiter")
	
	return &RateLimiter{
		config:      config,
		cacheRepo:   cacheRepo,
		logger:      namedLogger,
		userLimits:  make(map[uuid.UUID]*UserRateLimit),
		globalLimit: &GlobalRateLimit{
			RequestsPerSecond: 100,  // Default global limits
			RequestsPerMinute: 6000,
			RequestsPerHour:   360000,
		},
	}
}

// CheckLimits verifies if a user can make a request
func (rl *RateLimiter) CheckLimits(ctx context.Context, userID uuid.UUID) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	// Check global rate limits first
	if err := rl.checkGlobalLimits(now); err != nil {
		rl.logger.Warn("Global rate limit exceeded", zap.Error(err))
		return err
	}
	
	// Get or create user rate limit
	userLimit := rl.getUserRateLimit(userID)
	
	// Check if user is currently blocked
	if userLimit.IsBlocked && now.Before(userLimit.BlockedUntil) {
		return fmt.Errorf("user is blocked until %v: %s", userLimit.BlockedUntil, userLimit.BlockReason)
	}
	
	// Reset block if expired
	if userLimit.IsBlocked && now.After(userLimit.BlockedUntil) {
		userLimit.IsBlocked = false
		userLimit.BlockReason = ""
		rl.logger.Info("User block expired", zap.String("user_id", userID.String()))
	}
	
	// Check minute limit
	if err := rl.checkMinuteLimit(userLimit, now); err != nil {
		rl.recordViolation(userID, "minute", userLimit.RequestsMinute, rl.config.RequestsPerMinute)
		return err
	}
	
	// Check hour limit
	if err := rl.checkHourLimit(userLimit, now); err != nil {
		rl.recordViolation(userID, "hour", userLimit.RequestsHour, rl.config.RequestsPerHour)
		return err
	}
	
	// Check day limit
	if err := rl.checkDayLimit(userLimit, now); err != nil {
		rl.recordViolation(userID, "day", userLimit.RequestsDay, rl.config.RequestsPerDay)
		return err
	}
	
	// Check quota limit
	if err := rl.checkQuotaLimit(userLimit, now); err != nil {
		rl.recordViolation(userID, "quota", int(userLimit.QuotaUsed), int(userLimit.QuotaLimit))
		return err
	}
	
	// Update counters
	rl.updateUserCounters(userLimit, now)
	rl.updateGlobalCounters(now)
	
	// Persist to cache if available
	rl.persistUserLimitToCache(userID, userLimit)
	
	return nil
}

// ConsumeQuota decrements available quota for a user
func (rl *RateLimiter) ConsumeQuota(ctx context.Context, userID uuid.UUID, amount int64) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	userLimit := rl.getUserRateLimit(userID)
	
	if userLimit.QuotaUsed+amount > userLimit.QuotaLimit {
		return fmt.Errorf("quota exceeded: would use %d, limit is %d", 
			userLimit.QuotaUsed+amount, userLimit.QuotaLimit)
	}
	
	userLimit.QuotaUsed += amount
	rl.persistUserLimitToCache(userID, userLimit)
	
	rl.logger.Debug("Quota consumed",
		zap.String("user_id", userID.String()),
		zap.Int64("amount", amount),
		zap.Int64("remaining", userLimit.QuotaLimit-userLimit.QuotaUsed),
	)
	
	return nil
}

// GetStatus returns current rate limit status for a user
func (rl *RateLimiter) GetStatus(ctx context.Context, userID uuid.UUID) (*RateLimitStatus, error) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	userLimit := rl.getUserRateLimit(userID)
	
	// Calculate reset times
	minuteReset := userLimit.WindowMinute.Add(time.Minute)
	hourReset := userLimit.WindowHour.Add(time.Hour)
	dayReset := userLimit.WindowDay.Add(24 * time.Hour)
	
	status := &RateLimitStatus{
		UserID:              userID,
		RequestsThisMinute:  userLimit.RequestsMinute,
		RequestsThisHour:    userLimit.RequestsHour,
		RequestsThisDay:     userLimit.RequestsDay,
		MinuteLimit:         rl.config.RequestsPerMinute,
		HourLimit:           rl.config.RequestsPerHour,
		DayLimit:            rl.config.RequestsPerDay,
		MinuteReset:         minuteReset,
		HourReset:           hourReset,
		DayReset:            dayReset,
		IsLimited:           false,
	}
	
	// Check if any limits are being approached or exceeded
	if float64(userLimit.RequestsMinute)/float64(rl.config.RequestsPerMinute) > 0.8 ||
		float64(userLimit.RequestsHour)/float64(rl.config.RequestsPerHour) > 0.8 ||
		float64(userLimit.RequestsDay)/float64(rl.config.RequestsPerDay) > 0.8 {
		status.IsLimited = true
	}
	
	return status, nil
}

// GetQuotaInfo returns quota information for a user
func (rl *RateLimiter) GetQuotaInfo(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	userLimit := rl.getUserRateLimit(userID)
	
	usagePercent := 0.0
	if userLimit.QuotaLimit > 0 {
		usagePercent = float64(userLimit.QuotaUsed) / float64(userLimit.QuotaLimit) * 100
	}
	
	return &QuotaInfo{
		UserID:         userID,
		TotalQuota:     userLimit.QuotaLimit,
		UsedQuota:      userLimit.QuotaUsed,
		RemainingQuota: userLimit.QuotaLimit - userLimit.QuotaUsed,
		QuotaPeriod:    "monthly",
		ResetDate:      userLimit.QuotaResetDate,
		UsagePercent:   usagePercent,
	}, nil
}

// SetUserQuota sets a custom quota for a user
func (rl *RateLimiter) SetUserQuota(ctx context.Context, userID uuid.UUID, quota int64) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	userLimit := rl.getUserRateLimit(userID)
	userLimit.QuotaLimit = quota
	
	// If new quota is less than current usage, don't reset usage but log a warning
	if quota < userLimit.QuotaUsed {
		rl.logger.Warn("New quota is less than current usage",
			zap.String("user_id", userID.String()),
			zap.Int64("new_quota", quota),
			zap.Int64("current_usage", userLimit.QuotaUsed),
		)
	}
	
	rl.persistUserLimitToCache(userID, userLimit)
	
	rl.logger.Info("User quota updated",
		zap.String("user_id", userID.String()),
		zap.Int64("new_quota", quota),
	)
	
	return nil
}

// BlockUser temporarily blocks a user
func (rl *RateLimiter) BlockUser(ctx context.Context, userID uuid.UUID, duration time.Duration, reason string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	userLimit := rl.getUserRateLimit(userID)
	userLimit.IsBlocked = true
	userLimit.BlockedUntil = time.Now().Add(duration)
	userLimit.BlockReason = reason
	
	rl.persistUserLimitToCache(userID, userLimit)
	
	rl.logger.Warn("User blocked",
		zap.String("user_id", userID.String()),
		zap.Duration("duration", duration),
		zap.String("reason", reason),
	)
	
	return nil
}

// UnblockUser removes a user block
func (rl *RateLimiter) UnblockUser(ctx context.Context, userID uuid.UUID) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	userLimit := rl.getUserRateLimit(userID)
	userLimit.IsBlocked = false
	userLimit.BlockedUntil = time.Time{}
	userLimit.BlockReason = ""
	
	rl.persistUserLimitToCache(userID, userLimit)
	
	rl.logger.Info("User unblocked", zap.String("user_id", userID.String()))
	
	return nil
}

// UpdateConfig updates the rate limiter configuration
func (rl *RateLimiter) UpdateConfig(config *EnterpriseConfig) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.config = config
	rl.logger.Info("Rate limiter configuration updated")
}

// HealthCheck returns the health status of the rate limiter
func (rl *RateLimiter) HealthCheck() ComponentHealth {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	status := ComponentHealth{
		Status:    "healthy",
		Message:   "Rate limiter operational",
		LastCheck: time.Now(),
		Metrics: map[string]interface{}{
			"tracked_users":      len(rl.userLimits),
			"global_rps":         rl.globalLimit.RequestsPerSecond,
			"peak_rps":           rl.globalLimit.PeakRPS,
			"total_requests":     rl.globalLimit.TotalRequests,
		},
	}
	
	// Check for high load conditions
	blockedUsers := 0
	for _, limit := range rl.userLimits {
		if limit.IsBlocked {
			blockedUsers++
		}
	}
	
	if blockedUsers > 0 {
		status.Metrics["blocked_users"] = blockedUsers
		if blockedUsers > 10 {
			status.Status = "warning"
			status.Message = fmt.Sprintf("%d users are currently blocked", blockedUsers)
		}
	}
	
	return status
}

// Helper methods

func (rl *RateLimiter) getUserRateLimit(userID uuid.UUID) *UserRateLimit {
	if rl.userLimits[userID] == nil {
		now := time.Now()
		rl.userLimits[userID] = &UserRateLimit{
			UserID:         userID,
			WindowMinute:   now.Truncate(time.Minute),
			WindowHour:     now.Truncate(time.Hour),
			WindowDay:      now.Truncate(24 * time.Hour),
			QuotaLimit:     10000, // Default monthly quota
			QuotaResetDate: time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()),
		}
		
		// Try to load from cache
		rl.loadUserLimitFromCache(userID)
	}
	
	return rl.userLimits[userID]
}

func (rl *RateLimiter) checkGlobalLimits(now time.Time) error {
	// Reset counters if windows have passed
	if now.Truncate(time.Second).After(rl.globalLimit.WindowSecond) {
		rl.globalLimit.RequestsPerSecond = 0
		rl.globalLimit.WindowSecond = now.Truncate(time.Second)
	}
	
	if now.Truncate(time.Minute).After(rl.globalLimit.WindowMinute) {
		rl.globalLimit.RequestsPerMinute = 0
		rl.globalLimit.WindowMinute = now.Truncate(time.Minute)
	}
	
	if now.Truncate(time.Hour).After(rl.globalLimit.WindowHour) {
		rl.globalLimit.RequestsPerHour = 0
		rl.globalLimit.WindowHour = now.Truncate(time.Hour)
	}
	
	// Check limits (using conservative global limits)
	if rl.globalLimit.RequestsPerSecond >= 1000 {
		return fmt.Errorf("global rate limit exceeded: %d requests per second", rl.globalLimit.RequestsPerSecond)
	}
	
	if rl.globalLimit.RequestsPerMinute >= 60000 {
		return fmt.Errorf("global rate limit exceeded: %d requests per minute", rl.globalLimit.RequestsPerMinute)
	}
	
	if rl.globalLimit.RequestsPerHour >= 3600000 {
		return fmt.Errorf("global rate limit exceeded: %d requests per hour", rl.globalLimit.RequestsPerHour)
	}
	
	return nil
}

func (rl *RateLimiter) checkMinuteLimit(userLimit *UserRateLimit, now time.Time) error {
	// Reset counter if window has passed
	if now.Truncate(time.Minute).After(userLimit.WindowMinute) {
		userLimit.RequestsMinute = 0
		userLimit.WindowMinute = now.Truncate(time.Minute)
	}
	
	if userLimit.RequestsMinute >= rl.config.RequestsPerMinute {
		return fmt.Errorf("minute rate limit exceeded: %d/%d requests", 
			userLimit.RequestsMinute, rl.config.RequestsPerMinute)
	}
	
	return nil
}

func (rl *RateLimiter) checkHourLimit(userLimit *UserRateLimit, now time.Time) error {
	// Reset counter if window has passed
	if now.Truncate(time.Hour).After(userLimit.WindowHour) {
		userLimit.RequestsHour = 0
		userLimit.WindowHour = now.Truncate(time.Hour)
	}
	
	if userLimit.RequestsHour >= rl.config.RequestsPerHour {
		return fmt.Errorf("hour rate limit exceeded: %d/%d requests",
			userLimit.RequestsHour, rl.config.RequestsPerHour)
	}
	
	return nil
}

func (rl *RateLimiter) checkDayLimit(userLimit *UserRateLimit, now time.Time) error {
	// Reset counter if window has passed
	if now.Truncate(24*time.Hour).After(userLimit.WindowDay) {
		userLimit.RequestsDay = 0
		userLimit.WindowDay = now.Truncate(24 * time.Hour)
	}
	
	if userLimit.RequestsDay >= rl.config.RequestsPerDay {
		return fmt.Errorf("day rate limit exceeded: %d/%d requests",
			userLimit.RequestsDay, rl.config.RequestsPerDay)
	}
	
	return nil
}

func (rl *RateLimiter) checkQuotaLimit(userLimit *UserRateLimit, now time.Time) error {
	// Reset quota if month has passed
	if now.After(userLimit.QuotaResetDate) {
		userLimit.QuotaUsed = 0
		userLimit.QuotaResetDate = time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
		rl.logger.Info("User quota reset",
			zap.String("user_id", userLimit.UserID.String()),
			zap.Time("next_reset", userLimit.QuotaResetDate),
		)
	}
	
	if userLimit.QuotaUsed >= userLimit.QuotaLimit {
		return fmt.Errorf("quota limit exceeded: %d/%d used",
			userLimit.QuotaUsed, userLimit.QuotaLimit)
	}
	
	return nil
}

func (rl *RateLimiter) updateUserCounters(userLimit *UserRateLimit, now time.Time) {
	userLimit.RequestsMinute++
	userLimit.RequestsHour++
	userLimit.RequestsDay++
}

func (rl *RateLimiter) updateGlobalCounters(now time.Time) {
	rl.globalLimit.RequestsPerSecond++
	rl.globalLimit.RequestsPerMinute++
	rl.globalLimit.RequestsPerHour++
	rl.globalLimit.TotalRequests++
	
	// Track peak RPS
	if rl.globalLimit.RequestsPerSecond > rl.globalLimit.PeakRPS {
		rl.globalLimit.PeakRPS = rl.globalLimit.RequestsPerSecond
		rl.globalLimit.PeakRPSTime = now
	}
}

func (rl *RateLimiter) recordViolation(userID uuid.UUID, violationType string, current, limit int) {
	violation := RateLimitViolation{
		UserID:        userID,
		ViolationType: violationType,
		Current:       current,
		Limit:         limit,
		Timestamp:     time.Now(),
		Action:        "throttle",
	}
	
	// Log violation
	rl.logger.Warn("Rate limit violation",
		zap.String("user_id", userID.String()),
		zap.String("type", violationType),
		zap.Int("current", current),
		zap.Int("limit", limit),
	)
	
	// Store violation for analytics (simplified - in production, store in persistent storage)
	_ = violation
}

func (rl *RateLimiter) persistUserLimitToCache(userID uuid.UUID, userLimit *UserRateLimit) {
	if rl.cacheRepo == nil {
		return
	}
	
	key := fmt.Sprintf("rate_limit:user:%s", userID.String())
	
	// Serialize user limit data
	data := map[string]interface{}{
		"requests_minute":   userLimit.RequestsMinute,
		"requests_hour":     userLimit.RequestsHour,
		"requests_day":      userLimit.RequestsDay,
		"window_minute":     userLimit.WindowMinute,
		"window_hour":       userLimit.WindowHour,
		"window_day":        userLimit.WindowDay,
		"quota_used":        userLimit.QuotaUsed,
		"quota_limit":       userLimit.QuotaLimit,
		"quota_reset_date":  userLimit.QuotaResetDate,
		"is_blocked":        userLimit.IsBlocked,
		"blocked_until":     userLimit.BlockedUntil,
		"block_reason":      userLimit.BlockReason,
	}
	
	// This is simplified - in production, use proper serialization
	_ = data
	
	// Set with appropriate TTL
	ctx := context.Background()
	if err := rl.cacheRepo.Set(ctx, key, []byte("serialized_data"), 24*time.Hour); err != nil {
		rl.logger.Warn("Failed to persist user limit to cache", zap.Error(err))
	}
}

func (rl *RateLimiter) loadUserLimitFromCache(userID uuid.UUID) {
	if rl.cacheRepo == nil {
		return
	}
	
	key := fmt.Sprintf("rate_limit:user:%s", userID.String())
	ctx := context.Background()
	
	if data, err := rl.cacheRepo.Get(ctx, key); err == nil {
		// Deserialize and populate user limit
		// This is simplified - in production, use proper deserialization
		_ = data
		
		rl.logger.Debug("Loaded user limit from cache", zap.String("user_id", userID.String()))
	}
}

// GetRateLimitStatistics returns system-wide rate limiting statistics
func (rl *RateLimiter) GetRateLimitStatistics() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	blockedUsers := 0
	totalQuotaUsed := int64(0)
	totalQuotaLimit := int64(0)
	
	for _, limit := range rl.userLimits {
		if limit.IsBlocked {
			blockedUsers++
		}
		totalQuotaUsed += limit.QuotaUsed
		totalQuotaLimit += limit.QuotaLimit
	}
	
	quotaUtilization := 0.0
	if totalQuotaLimit > 0 {
		quotaUtilization = float64(totalQuotaUsed) / float64(totalQuotaLimit) * 100
	}
	
	return map[string]interface{}{
		"total_users":        len(rl.userLimits),
		"blocked_users":      blockedUsers,
		"total_requests":     rl.globalLimit.TotalRequests,
		"peak_rps":           rl.globalLimit.PeakRPS,
		"peak_rps_time":      rl.globalLimit.PeakRPSTime,
		"quota_utilization":  quotaUtilization,
		"total_quota_used":   totalQuotaUsed,
		"total_quota_limit":  totalQuotaLimit,
	}
}