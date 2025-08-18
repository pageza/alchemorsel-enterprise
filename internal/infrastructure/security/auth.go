// Package security provides enterprise-grade authentication and authorization
package security

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// AuthService provides authentication and authorization services
type AuthService struct {
	config      *config.Config
	logger      *zap.Logger
	redisClient *redis.Client
	jwtSecret   []byte
}

// NewAuthService creates a new authentication service
func NewAuthService(cfg *config.Config, logger *zap.Logger, redisClient *redis.Client) *AuthService {
	return &AuthService{
		config:      cfg,
		logger:      logger,
		redisClient: redisClient,
		jwtSecret:   []byte(cfg.Auth.JWTSecret),
	}
}

// TokenType represents different types of JWT tokens
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
	CSRFToken    TokenType = "csrf"
)

// Claims represents JWT claims structure
type Claims struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	TokenType TokenType `json:"token_type"`
	SessionID string    `json:"session_id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	jwt.RegisteredClaims
}

// AuthRequest represents authentication request
type AuthRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	CSRFToken    string    `json:"csrf_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

// UserInfo represents user information in auth response
type UserInfo struct {
	ID    string   `json:"id"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// SessionInfo represents session information
type SessionInfo struct {
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	Active    bool      `json:"active"`
}

// GenerateAccessToken creates a new access token
func (a *AuthService) GenerateAccessToken(userID, email string, roles []string, sessionID, ipAddress, userAgent string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		Email:     email,
		Roles:     roles,
		TokenType: AccessToken,
		SessionID: sessionID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "alchemorsel",
			Subject:   userID,
			Audience:  []string{"alchemorsel-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.Auth.JWTExpiration)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// Store token in Redis for tracking and revocation
	if err := a.storeTokenInRedis(claims.ID, tokenString, userID, sessionID, a.config.Auth.JWTExpiration); err != nil {
		a.logger.Warn("Failed to store token in Redis", zap.Error(err))
	}

	return tokenString, nil
}

// GenerateRefreshToken creates a new refresh token
func (a *AuthService) GenerateRefreshToken(userID, sessionID, ipAddress, userAgent string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		TokenType: RefreshToken,
		SessionID: sessionID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "alchemorsel",
			Subject:   userID,
			Audience:  []string{"alchemorsel-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.Auth.RefreshExpiration)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	// Store refresh token in Redis
	if err := a.storeTokenInRedis(claims.ID, tokenString, userID, sessionID, a.config.Auth.RefreshExpiration); err != nil {
		a.logger.Warn("Failed to store refresh token in Redis", zap.Error(err))
	}

	return tokenString, nil
}

// GenerateCSRFToken creates a CSRF token for HTMX protection
func (a *AuthService) GenerateCSRFToken(sessionID string) (string, error) {
	now := time.Now()
	claims := &Claims{
		TokenType: CSRFToken,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "alchemorsel",
			Audience:  []string{"alchemorsel-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)), // 24 hour CSRF token
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

// ValidateToken validates and parses a JWT token
func (a *AuthService) ValidateToken(tokenString string, expectedType TokenType) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate token type
	if claims.TokenType != expectedType {
		return nil, fmt.Errorf("invalid token type: expected %s, got %s", expectedType, claims.TokenType)
	}

	// Check if token is revoked (only for access and refresh tokens)
	if expectedType != CSRFToken {
		if revoked, err := a.isTokenRevoked(claims.ID); err != nil {
			a.logger.Warn("Failed to check token revocation", zap.Error(err))
		} else if revoked {
			return nil, fmt.Errorf("token has been revoked")
		}
	}

	return claims, nil
}

// RevokeToken revokes a token by adding it to the revocation list
func (a *AuthService) RevokeToken(tokenID string) error {
	ctx := context.Background()
	key := fmt.Sprintf("revoked_token:%s", tokenID)
	
	// Set with expiration matching the longest possible token lifetime
	return a.redisClient.Set(ctx, key, "revoked", a.config.Auth.RefreshExpiration).Err()
}

// RevokeAllUserTokens revokes all tokens for a specific user
func (a *AuthService) RevokeAllUserTokens(userID string) error {
	ctx := context.Background()
	pattern := fmt.Sprintf("token:%s:*", userID)
	
	keys, err := a.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to find user tokens: %w", err)
	}

	if len(keys) > 0 {
		return a.redisClient.Del(ctx, keys...).Err()
	}

	return nil
}

// CreateSession creates a new user session
func (a *AuthService) CreateSession(userID, ipAddress, userAgent string) (*SessionInfo, error) {
	sessionID := uuid.New().String()
	now := time.Now()
	
	session := &SessionInfo{
		UserID:    userID,
		SessionID: sessionID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		CreatedAt: now,
		ExpiresAt: now.Add(a.config.Auth.RefreshExpiration),
		Active:    true,
	}

	// Store session in Redis
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	
	if err := a.redisClient.HSet(ctx, sessionKey, map[string]interface{}{
		"user_id":    session.UserID,
		"ip_address": session.IPAddress,
		"user_agent": session.UserAgent,
		"created_at": session.CreatedAt.Unix(),
		"expires_at": session.ExpiresAt.Unix(),
		"active":     session.Active,
	}).Err(); err != nil {
		return nil, fmt.Errorf("failed to store session: %w", err)
	}

	// Set expiration
	a.redisClient.Expire(ctx, sessionKey, a.config.Auth.RefreshExpiration)

	return session, nil
}

// ValidateSession validates if a session is still active
func (a *AuthService) ValidateSession(sessionID, userID, ipAddress string) (*SessionInfo, error) {
	ctx := context.Background()
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	
	result, err := a.redisClient.HGetAll(ctx, sessionKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("session not found")
	}

	// Validate session belongs to user
	if result["user_id"] != userID {
		return nil, fmt.Errorf("session user mismatch")
	}

	// Security check: validate IP address consistency (can be disabled for mobile users)
	if a.config.IsProduction() && result["ip_address"] != ipAddress {
		a.logger.Warn("Session IP address mismatch",
			zap.String("session_id", sessionID),
			zap.String("stored_ip", result["ip_address"]),
			zap.String("request_ip", ipAddress),
		)
	}

	return &SessionInfo{
		UserID:    result["user_id"],
		SessionID: sessionID,
		IPAddress: result["ip_address"],
		UserAgent: result["user_agent"],
		Active:    result["active"] == "true",
	}, nil
}

// HashPassword securely hashes a password using bcrypt
func (a *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), a.config.Auth.BCryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against its hash
func (a *AuthService) VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// storeTokenInRedis stores token metadata in Redis for tracking
func (a *AuthService) storeTokenInRedis(tokenID, tokenString, userID, sessionID string, expiration time.Duration) error {
	ctx := context.Background()
	tokenKey := fmt.Sprintf("token:%s:%s", userID, tokenID)
	
	return a.redisClient.HSet(ctx, tokenKey, map[string]interface{}{
		"token":      tokenString,
		"session_id": sessionID,
		"created_at": time.Now().Unix(),
	}).Err()
}

// isTokenRevoked checks if a token has been revoked
func (a *AuthService) isTokenRevoked(tokenID string) (bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("revoked_token:%s", tokenID)
	
	exists, err := a.redisClient.Exists(ctx, key).Result()
	return exists > 0, err
}

// AuthMiddleware provides JWT authentication middleware
func (a *AuthService) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Check Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		// Validate token
		claims, err := a.ValidateToken(parts[1], AccessToken)
		if err != nil {
			a.logger.Info("Token validation failed",
				zap.String("error", err.Error()),
				zap.String("ip", c.ClientIP()),
				zap.String("user_agent", c.Request.UserAgent()),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Validate session
		session, err := a.ValidateSession(claims.SessionID, claims.UserID, c.ClientIP())
		if err != nil {
			a.logger.Info("Session validation failed",
				zap.String("error", err.Error()),
				zap.String("session_id", claims.SessionID),
				zap.String("user_id", claims.UserID),
			)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
			c.Abort()
			return
		}

		if !session.Active {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session inactive"})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Set("user_roles", claims.Roles)
		c.Set("session_id", claims.SessionID)

		c.Next()
	}
}

// CSRFMiddleware provides CSRF protection for HTMX requests
func (a *AuthService) CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF check for GET, HEAD, OPTIONS
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Check for CSRF token in header or form
		csrfToken := c.GetHeader("X-CSRF-Token")
		if csrfToken == "" {
			csrfToken = c.PostForm("csrf_token")
		}

		if csrfToken == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token required"})
			c.Abort()
			return
		}

		// Validate CSRF token
		claims, err := a.ValidateToken(csrfToken, CSRFToken)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid CSRF token"})
			c.Abort()
			return
		}

		// Validate session ID matches
		sessionID := c.GetString("session_id")
		if sessionID != "" && claims.SessionID != sessionID {
			c.JSON(http.StatusForbidden, gin.H{"error": "CSRF token session mismatch"})
			c.Abort()
			return
		}

		c.Next()
	}
}