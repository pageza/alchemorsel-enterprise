// Package cache provides session and user caching services for fast authentication and preferences
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SessionCacheService provides comprehensive session and user preference caching
type SessionCacheService struct {
	cache      *CacheService
	keyBuilder *KeyBuilder
	config     *SessionCacheConfig
	logger     *zap.Logger
}

// SessionCacheConfig configures session caching behavior
type SessionCacheConfig struct {
	// TTL configurations
	SessionTTL        time.Duration `json:"session_ttl"`
	UserTTL           time.Duration `json:"user_ttl"`
	PreferencesTTL    time.Duration `json:"preferences_ttl"`
	AuthTokenTTL      time.Duration `json:"auth_token_ttl"`
	RefreshTokenTTL   time.Duration `json:"refresh_token_ttl"`
	
	// Security settings
	EncryptSessions   bool          `json:"encrypt_sessions"`
	RequireSecure     bool          `json:"require_secure"`
	MaxSessionsPerUser int          `json:"max_sessions_per_user"`
	
	// Performance settings
	ExtendOnAccess    bool          `json:"extend_on_access"`
	ExtensionWindow   time.Duration `json:"extension_window"`
	CleanupInterval   time.Duration `json:"cleanup_interval"`
	
	// User data caching
	CacheUserProfiles bool          `json:"cache_user_profiles"`
	ProfileTTL        time.Duration `json:"profile_ttl"`
	PreloadPrefs      bool          `json:"preload_prefs"`
}

// CachedSession represents a cached user session
type CachedSession struct {
	ID            string                 `json:"id"`
	UserID        uuid.UUID              `json:"user_id"`
	CreatedAt     time.Time              `json:"created_at"`
	LastAccess    time.Time              `json:"last_access"`
	ExpiresAt     time.Time              `json:"expires_at"`
	IPAddress     string                 `json:"ip_address"`
	UserAgent     string                 `json:"user_agent"`
	Data          map[string]interface{} `json:"data"`
	IsActive      bool                   `json:"is_active"`
	DeviceInfo    *DeviceInfo            `json:"device_info,omitempty"`
	SecurityFlags *SecurityFlags         `json:"security_flags,omitempty"`
}

// CachedUser represents a cached user profile
type CachedUser struct {
	User         *user.User             `json:"user"`
	Preferences  map[string]interface{} `json:"preferences"`
	Settings     map[string]interface{} `json:"settings"`
	CachedAt     time.Time              `json:"cached_at"`
	LastModified time.Time              `json:"last_modified"`
	AccessCount  int64                  `json:"access_count"`
}

// DeviceInfo contains device-specific information
type DeviceInfo struct {
	Type         string `json:"type"`          // mobile, desktop, tablet
	OS           string `json:"os"`            // iOS, Android, Windows, etc.
	Browser      string `json:"browser"`       // Chrome, Safari, Firefox, etc.
	IsMobile     bool   `json:"is_mobile"`
	IsBot        bool   `json:"is_bot"`
	Fingerprint  string `json:"fingerprint"`   // Device fingerprint hash
}

// SecurityFlags contains security-related session information
type SecurityFlags struct {
	Is2FAEnabled     bool      `json:"is_2fa_enabled"`
	RequireReauth    bool      `json:"require_reauth"`
	SuspiciousLogin  bool      `json:"suspicious_login"`
	LastPasswordChange time.Time `json:"last_password_change"`
	FailedAttempts   int       `json:"failed_attempts"`
}

// AuthToken represents a cached authentication token
type AuthToken struct {
	Token      string    `json:"token"`
	UserID     uuid.UUID `json:"user_id"`
	SessionID  string    `json:"session_id"`
	ExpiresAt  time.Time `json:"expires_at"`
	Scopes     []string  `json:"scopes"`
	IsRevoked  bool      `json:"is_revoked"`
}

// NewSessionCacheService creates a new session cache service
func NewSessionCacheService(cache *CacheService, logger *zap.Logger) *SessionCacheService {
	config := DefaultSessionCacheConfig()
	
	service := &SessionCacheService{
		cache:      cache,
		keyBuilder: NewKeyBuilder(),
		config:     config,
		logger:     logger,
	}
	
	// Start cleanup routine
	if config.CleanupInterval > 0 {
		go service.startCleanupRoutine()
	}
	
	return service
}

// CreateSession creates and caches a new user session
func (scs *SessionCacheService) CreateSession(ctx context.Context, userID uuid.UUID, sessionID string, deviceInfo *DeviceInfo, duration time.Duration) (*CachedSession, error) {
	if duration <= 0 {
		duration = scs.config.SessionTTL
	}
	
	now := time.Now()
	session := &CachedSession{
		ID:         sessionID,
		UserID:     userID,
		CreatedAt:  now,
		LastAccess: now,
		ExpiresAt:  now.Add(duration),
		Data:       make(map[string]interface{}),
		IsActive:   true,
		DeviceInfo: deviceInfo,
		SecurityFlags: &SecurityFlags{
			FailedAttempts: 0,
		},
	}
	
	// Check session limits
	if err := scs.enforceSessionLimits(ctx, userID); err != nil {
		scs.logger.Warn("Failed to enforce session limits", 
			zap.String("user_id", userID.String()), 
			zap.Error(err))
	}
	
	// Cache the session
	if err := scs.cacheSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to cache session: %w", err)
	}
	
	// Track user session count
	userSessionsKey := scs.keyBuilder.BuildKey("user_sessions", userID.String())
	if _, err := scs.cache.redis.client.SAdd(ctx, userSessionsKey, sessionID).Result(); err != nil {
		scs.logger.Error("Failed to track user session", 
			zap.String("user_id", userID.String()), 
			zap.Error(err))
	}
	
	scs.logger.Info("Session created", 
		zap.String("session_id", sessionID),
		zap.String("user_id", userID.String()),
		zap.Duration("ttl", duration))
	
	return session, nil
}

// GetSession retrieves a session from cache
func (scs *SessionCacheService) GetSession(ctx context.Context, sessionID string) (*CachedSession, error) {
	sessionKey := scs.keyBuilder.BuildSessionKey(sessionID)
	
	data, err := scs.cache.Get(ctx, sessionKey)
	if err != nil {
		if err == ErrKeyNotFound {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	
	var session CachedSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	
	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		// Clean up expired session
		go scs.DeleteSession(context.Background(), sessionID)
		return nil, fmt.Errorf("session expired")
	}
	
	// Check if session is active
	if !session.IsActive {
		return nil, fmt.Errorf("session is inactive")
	}
	
	// Update last access time if extension is enabled
	if scs.config.ExtendOnAccess {
		session.LastAccess = time.Now()
		
		// Extend expiration if within extension window
		timeToExpiry := session.ExpiresAt.Sub(time.Now())
		if timeToExpiry < scs.config.ExtensionWindow {
			session.ExpiresAt = time.Now().Add(scs.config.SessionTTL)
			
			// Update cache asynchronously
			go func() {
				if err := scs.cacheSession(context.Background(), &session); err != nil {
					scs.logger.Error("Failed to extend session", 
						zap.String("session_id", sessionID), 
						zap.Error(err))
				}
			}()
		}
	}
	
	return &session, nil
}

// UpdateSession updates session data
func (scs *SessionCacheService) UpdateSession(ctx context.Context, sessionID string, updates map[string]interface{}) error {
	session, err := scs.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	
	// Update session data
	for key, value := range updates {
		session.Data[key] = value
	}
	
	session.LastAccess = time.Now()
	
	return scs.cacheSession(ctx, session)
}

// DeleteSession removes a session from cache
func (scs *SessionCacheService) DeleteSession(ctx context.Context, sessionID string) error {
	sessionKey := scs.keyBuilder.BuildSessionKey(sessionID)
	
	// Get session to find user ID for cleanup
	if session, err := scs.GetSession(ctx, sessionID); err == nil {
		// Remove from user sessions set
		userSessionsKey := scs.keyBuilder.BuildKey("user_sessions", session.UserID.String())
		if err := scs.cache.redis.client.SRem(ctx, userSessionsKey, sessionID).Err(); err != nil {
			scs.logger.Error("Failed to remove session from user set", 
				zap.String("session_id", sessionID), 
				zap.Error(err))
		}
	}
	
	// Delete session
	if err := scs.cache.Delete(ctx, sessionKey); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	
	scs.logger.Info("Session deleted", zap.String("session_id", sessionID))
	return nil
}

// DeleteUserSessions removes all sessions for a user
func (scs *SessionCacheService) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	userSessionsKey := scs.keyBuilder.BuildKey("user_sessions", userID.String())
	
	// Get all session IDs for the user
	sessionIDs, err := scs.cache.redis.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}
	
	// Delete all sessions
	for _, sessionID := range sessionIDs {
		if err := scs.DeleteSession(ctx, sessionID); err != nil {
			scs.logger.Error("Failed to delete user session", 
				zap.String("user_id", userID.String()),
				zap.String("session_id", sessionID),
				zap.Error(err))
		}
	}
	
	// Clean up user sessions set
	if err := scs.cache.Delete(ctx, userSessionsKey); err != nil {
		scs.logger.Error("Failed to delete user sessions set", 
			zap.String("user_id", userID.String()), 
			zap.Error(err))
	}
	
	scs.logger.Info("All user sessions deleted", 
		zap.String("user_id", userID.String()),
		zap.Int("count", len(sessionIDs)))
	
	return nil
}

// CacheUser stores user profile and preferences
func (scs *SessionCacheService) CacheUser(ctx context.Context, u *user.User, preferences, settings map[string]interface{}) error {
	if !scs.config.CacheUserProfiles {
		return nil
	}
	
	userKey := scs.keyBuilder.BuildUserKey(u.ID.String())
	
	cached := CachedUser{
		User:         u,
		Preferences:  preferences,
		Settings:     settings,
		CachedAt:     time.Now(),
		LastModified: u.UpdatedAt,
		AccessCount:  0,
	}
	
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal user for cache: %w", err)
	}
	
	if err := scs.cache.Set(ctx, userKey, data, scs.config.UserTTL); err != nil {
		return fmt.Errorf("failed to cache user: %w", err)
	}
	
	// Cache preferences separately for quick access
	if scs.config.PreloadPrefs && len(preferences) > 0 {
		prefsKey := scs.keyBuilder.BuildKey("user_prefs", u.ID.String())
		prefsData, err := json.Marshal(preferences)
		if err == nil {
			scs.cache.Set(ctx, prefsKey, prefsData, scs.config.PreferencesTTL)
		}
	}
	
	scs.logger.Debug("User cached", zap.String("user_id", u.ID.String()))
	return nil
}

// GetUser retrieves cached user data
func (scs *SessionCacheService) GetUser(ctx context.Context, userID uuid.UUID) (*CachedUser, error) {
	if !scs.config.CacheUserProfiles {
		return nil, fmt.Errorf("user caching disabled")
	}
	
	userKey := scs.keyBuilder.BuildUserKey(userID.String())
	
	data, err := scs.cache.Get(ctx, userKey)
	if err != nil {
		return nil, err
	}
	
	var cached CachedUser
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached user: %w", err)
	}
	
	// Update access count asynchronously
	go scs.incrementUserAccessCount(ctx, userKey, &cached)
	
	return &cached, nil
}

// GetUserPreferences retrieves cached user preferences
func (scs *SessionCacheService) GetUserPreferences(ctx context.Context, userID uuid.UUID) (map[string]interface{}, error) {
	prefsKey := scs.keyBuilder.BuildKey("user_prefs", userID.String())
	
	data, err := scs.cache.Get(ctx, prefsKey)
	if err != nil {
		// Try to get from user cache
		if cached, err := scs.GetUser(ctx, userID); err == nil {
			return cached.Preferences, nil
		}
		return nil, err
	}
	
	var preferences map[string]interface{}
	if err := json.Unmarshal(data, &preferences); err != nil {
		return nil, fmt.Errorf("failed to unmarshal preferences: %w", err)
	}
	
	return preferences, nil
}

// UpdateUserPreferences updates cached user preferences
func (scs *SessionCacheService) UpdateUserPreferences(ctx context.Context, userID uuid.UUID, preferences map[string]interface{}) error {
	prefsKey := scs.keyBuilder.BuildKey("user_prefs", userID.String())
	
	data, err := json.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("failed to marshal preferences: %w", err)
	}
	
	if err := scs.cache.Set(ctx, prefsKey, data, scs.config.PreferencesTTL); err != nil {
		return fmt.Errorf("failed to cache preferences: %w", err)
	}
	
	// Update in user cache if exists
	if scs.config.CacheUserProfiles {
		go scs.updateUserCachePreferences(ctx, userID, preferences)
	}
	
	return nil
}

// InvalidateUser removes all cached data for a user
func (scs *SessionCacheService) InvalidateUser(ctx context.Context, userID uuid.UUID) error {
	// Delete user cache
	userKey := scs.keyBuilder.BuildUserKey(userID.String())
	scs.cache.Delete(ctx, userKey)
	
	// Delete preferences cache
	prefsKey := scs.keyBuilder.BuildKey("user_prefs", userID.String())
	scs.cache.Delete(ctx, prefsKey)
	
	// Delete all user sessions
	scs.DeleteUserSessions(ctx, userID)
	
	scs.logger.Info("User invalidated", zap.String("user_id", userID.String()))
	return nil
}

// CacheAuthToken stores an authentication token
func (scs *SessionCacheService) CacheAuthToken(ctx context.Context, token string, userID uuid.UUID, sessionID string, scopes []string, duration time.Duration) error {
	if duration <= 0 {
		duration = scs.config.AuthTokenTTL
	}
	
	authToken := AuthToken{
		Token:     token,
		UserID:    userID,
		SessionID: sessionID,
		ExpiresAt: time.Now().Add(duration),
		Scopes:    scopes,
		IsRevoked: false,
	}
	
	tokenKey := scs.keyBuilder.BuildKey("auth_token", token)
	
	data, err := json.Marshal(authToken)
	if err != nil {
		return fmt.Errorf("failed to marshal auth token: %w", err)
	}
	
	if err := scs.cache.Set(ctx, tokenKey, data, duration); err != nil {
		return fmt.Errorf("failed to cache auth token: %w", err)
	}
	
	return nil
}

// ValidateAuthToken validates a cached authentication token
func (scs *SessionCacheService) ValidateAuthToken(ctx context.Context, token string) (*AuthToken, error) {
	tokenKey := scs.keyBuilder.BuildKey("auth_token", token)
	
	data, err := scs.cache.Get(ctx, tokenKey)
	if err != nil {
		return nil, fmt.Errorf("token not found or expired")
	}
	
	var authToken AuthToken
	if err := json.Unmarshal(data, &authToken); err != nil {
		return nil, fmt.Errorf("failed to unmarshal auth token: %w", err)
	}
	
	if authToken.IsRevoked {
		return nil, fmt.Errorf("token is revoked")
	}
	
	if time.Now().After(authToken.ExpiresAt) {
		return nil, fmt.Errorf("token is expired")
	}
	
	return &authToken, nil
}

// RevokeAuthToken revokes an authentication token
func (scs *SessionCacheService) RevokeAuthToken(ctx context.Context, token string) error {
	tokenKey := scs.keyBuilder.BuildKey("auth_token", token)
	return scs.cache.Delete(ctx, tokenKey)
}

// Helper methods

func (scs *SessionCacheService) cacheSession(ctx context.Context, session *CachedSession) error {
	sessionKey := scs.keyBuilder.BuildSessionKey(session.ID)
	
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	
	ttl := session.ExpiresAt.Sub(time.Now())
	if ttl <= 0 {
		ttl = scs.config.SessionTTL
	}
	
	return scs.cache.Set(ctx, sessionKey, data, ttl)
}

func (scs *SessionCacheService) enforceSessionLimits(ctx context.Context, userID uuid.UUID) error {
	if scs.config.MaxSessionsPerUser <= 0 {
		return nil
	}
	
	userSessionsKey := scs.keyBuilder.BuildKey("user_sessions", userID.String())
	
	// Get current session count
	sessionIDs, err := scs.cache.redis.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return err
	}
	
	if len(sessionIDs) >= scs.config.MaxSessionsPerUser {
		// Remove oldest sessions
		excess := len(sessionIDs) - scs.config.MaxSessionsPerUser + 1
		for i := 0; i < excess && i < len(sessionIDs); i++ {
			if err := scs.DeleteSession(ctx, sessionIDs[i]); err != nil {
				scs.logger.Error("Failed to delete excess session", 
					zap.String("session_id", sessionIDs[i]), 
					zap.Error(err))
			}
		}
	}
	
	return nil
}

func (scs *SessionCacheService) incrementUserAccessCount(ctx context.Context, userKey string, cached *CachedUser) {
	cached.AccessCount++
	
	data, err := json.Marshal(cached)
	if err != nil {
		return
	}
	
	scs.cache.Set(ctx, userKey, data, scs.config.UserTTL)
}

func (scs *SessionCacheService) updateUserCachePreferences(ctx context.Context, userID uuid.UUID, preferences map[string]interface{}) {
	userKey := scs.keyBuilder.BuildUserKey(userID.String())
	
	if cached, err := scs.GetUser(ctx, userID); err == nil {
		cached.Preferences = preferences
		cached.LastModified = time.Now()
		
		data, err := json.Marshal(cached)
		if err == nil {
			scs.cache.Set(ctx, userKey, data, scs.config.UserTTL)
		}
	}
}

func (scs *SessionCacheService) startCleanupRoutine() {
	ticker := time.NewTicker(scs.config.CleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		scs.cleanupExpiredSessions()
	}
}

func (scs *SessionCacheService) cleanupExpiredSessions() {
	// This would scan for expired sessions and clean them up
	// Implementation would depend on specific Redis patterns used
	scs.logger.Debug("Running session cleanup")
}

// DefaultSessionCacheConfig returns default session cache configuration
func DefaultSessionCacheConfig() *SessionCacheConfig {
	return &SessionCacheConfig{
		SessionTTL:         time.Hour * 24,
		UserTTL:            time.Hour,
		PreferencesTTL:     time.Hour * 6,
		AuthTokenTTL:       time.Hour,
		RefreshTokenTTL:    time.Hour * 24 * 7, // 7 days
		EncryptSessions:    false,
		RequireSecure:      true,
		MaxSessionsPerUser: 5,
		ExtendOnAccess:     true,
		ExtensionWindow:    time.Hour,
		CleanupInterval:    time.Hour,
		CacheUserProfiles:  true,
		ProfileTTL:         time.Hour,
		PreloadPrefs:       true,
	}
}