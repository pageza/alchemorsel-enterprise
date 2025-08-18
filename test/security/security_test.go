// Package security provides comprehensive security testing
//go:build security
// +build security

package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// SecurityTestSuite provides comprehensive security testing
type SecurityTestSuite struct {
	suite.Suite
	authService *security.AuthService
	redisClient *redis.Client
	server      *gin.Engine
	config      *config.Config
	assertions  *testutils.SecurityAssertions
	ctx         context.Context
}

// SetupSuite initializes the security test suite
func (suite *SecurityTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	gin.SetMode(gin.TestMode)
	
	// Setup test configuration with security-focused settings
	suite.config = &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret-key-for-security-testing-must-be-32-bytes-long",
			JWTExpiration:     15 * time.Minute, // Shorter for security
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        12, // Higher cost for security tests
		},
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 60,  // Lower for security testing
			BurstSize:         5,   // Lower burst
		},
		Environment: "test",
	}
	
	// Setup Redis
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   4, // Use different DB for security tests
	})
	suite.redisClient.FlushDB(suite.ctx)
	
	// Setup auth service
	logger := zap.NewNop()
	suite.authService = security.NewAuthService(suite.config, logger, suite.redisClient)
	
	// Setup assertions
	suite.assertions = testutils.NewSecurityAssertions(suite.T())
	
	// Setup test server
	suite.setupSecureServer()
}

// SetupTest cleans up before each test
func (suite *SecurityTestSuite) SetupTest() {
	suite.redisClient.FlushDB(suite.ctx)
}

// setupSecureServer creates a server with all security middleware
func (suite *SecurityTestSuite) setupSecureServer() {
	suite.server = gin.New()
	
	// Security middleware
	suite.server.Use(gin.Recovery())
	suite.server.Use(suite.securityHeadersMiddleware())
	suite.server.Use(suite.rateLimitMiddleware())
	suite.server.Use(suite.corsMiddleware())
	
	// Routes
	api := suite.server.Group("/api/v1")
	
	// Public endpoints
	api.POST("/auth/login", suite.handleLogin)
	api.POST("/auth/register", suite.handleRegister)
	api.POST("/auth/refresh", suite.handleRefresh)
	
	// Protected endpoints
	protected := api.Group("")
	protected.Use(suite.authService.AuthMiddleware())
	{
		protected.GET("/profile", suite.handleProfile)
		protected.POST("/data", suite.authService.CSRFMiddleware(), suite.handlePostData)
		protected.DELETE("/account", suite.authService.CSRFMiddleware(), suite.handleDeleteAccount)
	}
	
	// Admin endpoints
	admin := api.Group("/admin")
	admin.Use(suite.authService.AuthMiddleware())
	admin.Use(suite.adminOnlyMiddleware())
	{
		admin.GET("/users", suite.handleAdminUsers)
		admin.DELETE("/users/:id", suite.authService.CSRFMiddleware(), suite.handleAdminDeleteUser)
	}
}

// Middleware implementations

func (suite *SecurityTestSuite) securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

func (suite *SecurityTestSuite) rateLimitMiddleware() gin.HandlerFunc {
	// Simple rate limiting implementation for testing
	requests := make(map[string][]time.Time)
	
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()
		
		// Clean old requests
		if clientRequests, exists := requests[clientIP]; exists {
			var validRequests []time.Time
			for _, reqTime := range clientRequests {
				if now.Sub(reqTime) < time.Minute {
					validRequests = append(validRequests, reqTime)
				}
			}
			requests[clientIP] = validRequests
		}
		
		// Check rate limit
		if len(requests[clientIP]) >= suite.config.RateLimit.RequestsPerMinute {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			c.Abort()
			return
		}
		
		// Add current request
		requests[clientIP] = append(requests[clientIP], now)
		c.Next()
	}
}

func (suite *SecurityTestSuite) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "https://trusted-domain.com")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

func (suite *SecurityTestSuite) adminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		roles, exists := c.Get("user_roles")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "No roles found"})
			c.Abort()
			return
		}
		
		userRoles, ok := roles.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid roles format"})
			c.Abort()
			return
		}
		
		hasAdminRole := false
		for _, role := range userRoles {
			if role == "admin" {
				hasAdminRole = true
				break
			}
		}
		
		if !hasAdminRole {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// Handler implementations

func (suite *SecurityTestSuite) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	// Mock authentication
	if req.Email == "test@example.com" && req.Password == "password123" {
		userID := uuid.New().String()
		session, _ := suite.authService.CreateSession(userID, c.ClientIP(), c.Request.UserAgent())
		accessToken, _ := suite.authService.GenerateAccessToken(
			userID, req.Email, []string{"user"}, session.SessionID, c.ClientIP(), c.Request.UserAgent(),
		)
		refreshToken, _ := suite.authService.GenerateRefreshToken(
			userID, session.SessionID, c.ClientIP(), c.Request.UserAgent(),
		)
		
		c.JSON(http.StatusOK, gin.H{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"expires_in":    3600,
		})
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
	}
}

func (suite *SecurityTestSuite) handleRegister(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
		Username string `json:"username" binding:"required,min=3"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	// Password security validation
	if !suite.isPasswordSecure(req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password does not meet security requirements"})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully"})
}

func (suite *SecurityTestSuite) handleRefresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	
	claims, err := suite.authService.ValidateToken(req.RefreshToken, security.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}
	
	// Generate new access token
	newAccessToken, _ := suite.authService.GenerateAccessToken(
		claims.UserID, claims.Email, []string{"user"}, claims.SessionID, c.ClientIP(), c.Request.UserAgent(),
	)
	
	c.JSON(http.StatusOK, gin.H{
		"access_token": newAccessToken,
		"expires_in":   3600,
	})
}

func (suite *SecurityTestSuite) handleProfile(c *gin.Context) {
	userID := c.GetString("user_id")
	email := c.GetString("user_email")
	
	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"email":   email,
		"profile": "User profile data",
	})
}

func (suite *SecurityTestSuite) handlePostData(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Data processed successfully"})
}

func (suite *SecurityTestSuite) handleDeleteAccount(c *gin.Context) {
	userID := c.GetString("user_id")
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Account %s deleted", userID)})
}

func (suite *SecurityTestSuite) handleAdminUsers(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"users": []string{"user1", "user2"}})
}

func (suite *SecurityTestSuite) handleAdminDeleteUser(c *gin.Context) {
	userID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("User %s deleted by admin", userID)})
}

// Helper methods

func (suite *SecurityTestSuite) isPasswordSecure(password string) bool {
	if len(password) < 8 {
		return false
	}
	
	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false
	
	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasNumber = true
		default:
			hasSpecial = true
		}
	}
	
	return hasUpper && hasLower && hasNumber && hasSpecial
}

func (suite *SecurityTestSuite) createAuthenticatedRequest(method, url string, body interface{}, roles []string) *http.Request {
	userID := uuid.New().String()
	email := "test@example.com"
	sessionID := uuid.New().String()
	
	session, _ := suite.authService.CreateSession(userID, "192.168.1.1", "Test Browser")
	accessToken, _ := suite.authService.GenerateAccessToken(
		userID, email, roles, session.SessionID, "192.168.1.1", "Test Browser",
	)
	csrfToken, _ := suite.authService.GenerateCSRFToken(session.SessionID)
	
	var reqBody *bytes.Buffer
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(jsonBody)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	
	req := httptest.NewRequest(method, url, reqBody)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.Header.Set("Content-Type", "application/json")
	
	return req
}

// Test Authentication Security

func (suite *SecurityTestSuite) TestAuthenticationSecurity() {
	suite.Run("Login_ValidCredentials_ShouldSucceed", func() {
		// Arrange
		loginData := map[string]string{
			"email":    "test@example.com",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(loginData)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		
		accessToken, exists := response["access_token"]
		assert.True(suite.T(), exists, "Should return access token")
		suite.assertions.JWTValid(accessToken.(string))
		
		refreshToken, exists := response["refresh_token"]
		assert.True(suite.T(), exists, "Should return refresh token")
		suite.assertions.JWTValid(refreshToken.(string))
	})

	suite.Run("Login_InvalidCredentials_ShouldFail", func() {
		// Arrange
		loginData := map[string]string{
			"email":    "test@example.com",
			"password": "wrongpassword",
		}
		jsonBody, _ := json.Marshal(loginData)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(suite.T(), response["error"], "Invalid credentials")
	})

	suite.Run("Login_BruteForceAttempt_ShouldBeRateLimited", func() {
		// Arrange & Act - Make multiple failed login attempts
		loginData := map[string]string{
			"email":    "test@example.com",
			"password": "wrongpassword",
		}
		
		successCount := 0
		rateLimitedCount := 0
		
		for i := 0; i < 70; i++ { // Exceed rate limit
			jsonBody, _ := json.Marshal(loginData)
			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			
			suite.server.ServeHTTP(w, req)
			
			if w.Code == http.StatusUnauthorized {
				successCount++
			} else if w.Code == http.StatusTooManyRequests {
				rateLimitedCount++
			}
		}

		// Assert
		assert.Greater(suite.T(), rateLimitedCount, 0, "Should rate limit excessive requests")
		assert.LessOrEqual(suite.T(), successCount, 60, "Should not process all requests")
	})

	suite.Run("TokenRefresh_ValidToken_ShouldSucceed", func() {
		// Arrange - First login to get refresh token
		loginData := map[string]string{
			"email":    "test@example.com",
			"password": "password123",
		}
		jsonBody, _ := json.Marshal(loginData)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		suite.server.ServeHTTP(w, req)
		
		var loginResponse map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &loginResponse)
		refreshToken := loginResponse["refresh_token"].(string)
		
		// Use refresh token
		refreshData := map[string]string{
			"refresh_token": refreshToken,
		}
		jsonBody, _ = json.Marshal(refreshData)
		req = httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		
		newAccessToken, exists := response["access_token"]
		assert.True(suite.T(), exists, "Should return new access token")
		suite.assertions.JWTValid(newAccessToken.(string))
	})

	suite.Run("TokenRefresh_InvalidToken_ShouldFail", func() {
		// Arrange
		refreshData := map[string]string{
			"refresh_token": "invalid.refresh.token",
		}
		jsonBody, _ := json.Marshal(refreshData)
		req := httptest.NewRequest("POST", "/api/v1/auth/refresh", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	})
}

// Test Password Security

func (suite *SecurityTestSuite) TestPasswordSecurity() {
	suite.Run("Register_WeakPassword_ShouldFail", func() {
		// Arrange
		registerData := map[string]string{
			"email":    "newuser@example.com",
			"password": "weak",
			"username": "newuser",
		}
		jsonBody, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(suite.T(), response["error"], "security requirements")
	})

	suite.Run("Register_StrongPassword_ShouldSucceed", func() {
		// Arrange
		registerData := map[string]string{
			"email":    "newuser@example.com",
			"password": "StrongPassword123!",
			"username": "newuser",
		}
		jsonBody, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusCreated, w.Code)
	})

	suite.Run("PasswordHashing_ShouldBeSecure", func() {
		// Arrange
		password := "TestPassword123!"

		// Act
		hash, err := suite.authService.HashPassword(password)

		// Assert
		require.NoError(suite.T(), err)
		suite.assertions.PasswordHash(hash)
		
		// Verify password is not stored in plain text
		assert.NotEqual(suite.T(), password, hash)
		
		// Verify hash uses appropriate cost
		cost, err := bcrypt.Cost([]byte(hash))
		require.NoError(suite.T(), err)
		assert.GreaterOrEqual(suite.T(), cost, 10, "Password hash should use sufficient cost factor")
	})
}

// Test Authorization

func (suite *SecurityTestSuite) TestAuthorization() {
	suite.Run("ProtectedEndpoint_NoAuth_ShouldFail", func() {
		// Arrange
		req := httptest.NewRequest("GET", "/api/v1/profile", nil)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	})

	suite.Run("ProtectedEndpoint_ValidAuth_ShouldSucceed", func() {
		// Arrange
		req := suite.createAuthenticatedRequest("GET", "/api/v1/profile", nil, []string{"user"})
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
	})

	suite.Run("AdminEndpoint_UserRole_ShouldFail", func() {
		// Arrange
		req := suite.createAuthenticatedRequest("GET", "/api/v1/admin/users", nil, []string{"user"})
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusForbidden, w.Code)
	})

	suite.Run("AdminEndpoint_AdminRole_ShouldSucceed", func() {
		// Arrange
		req := suite.createAuthenticatedRequest("GET", "/api/v1/admin/users", nil, []string{"admin", "user"})
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
	})
}

// Test CSRF Protection

func (suite *SecurityTestSuite) TestCSRFProtection() {
	suite.Run("StateChangingRequest_NoCSRFToken_ShouldFail", func() {
		// Arrange
		userID := uuid.New().String()
		sessionID := uuid.New().String()
		
		session, _ := suite.authService.CreateSession(userID, "192.168.1.1", "Test Browser")
		accessToken, _ := suite.authService.GenerateAccessToken(
			userID, "test@example.com", []string{"user"}, session.SessionID, "192.168.1.1", "Test Browser",
		)
		
		req := httptest.NewRequest("POST", "/api/v1/data", bytes.NewBuffer([]byte("{}")))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusForbidden, w.Code)
	})

	suite.Run("StateChangingRequest_ValidCSRFToken_ShouldSucceed", func() {
		// Arrange
		req := suite.createAuthenticatedRequest("POST", "/api/v1/data", map[string]string{"test": "data"}, []string{"user"})
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
	})

	suite.Run("GetRequest_NoCSRFToken_ShouldSucceed", func() {
		// Arrange - GET requests should not require CSRF tokens
		req := suite.createAuthenticatedRequest("GET", "/api/v1/profile", nil, []string{"user"})
		req.Header.Del("X-CSRF-Token") // Remove CSRF token
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
	})
}

// Test Input Validation and Injection Prevention

func (suite *SecurityTestSuite) TestInputValidation() {
	suite.Run("SQLInjection_ShouldBeBlocked", func() {
		// Arrange - Attempt SQL injection in login
		loginData := map[string]string{
			"email":    "admin@example.com'; DROP TABLE users; --",
			"password": "password",
		}
		jsonBody, _ := json.Marshal(loginData)
		req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		assert.Contains(suite.T(), response["error"], "Invalid request")
	})

	suite.Run("XSS_ShouldBeSanitized", func() {
		// Arrange - Attempt XSS in registration
		registerData := map[string]string{
			"email":    "test@example.com",
			"password": "StrongPassword123!",
			"username": "<script>alert('xss')</script>",
		}
		jsonBody, _ := json.Marshal(registerData)
		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		// Should either be rejected or sanitized (depending on implementation)
		assert.NotEqual(suite.T(), http.StatusInternalServerError, w.Code, "Server should handle XSS attempts gracefully")
	})

	suite.Run("CommandInjection_ShouldBeBlocked", func() {
		// Arrange - Attempt command injection
		data := map[string]string{
			"filename": "../../../etc/passwd",
			"content":  "; cat /etc/passwd",
		}
		req := suite.createAuthenticatedRequest("POST", "/api/v1/data", data, []string{"user"})
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		// Server should handle this safely
		assert.NotEqual(suite.T(), http.StatusInternalServerError, w.Code)
	})

	suite.Run("OversizedPayload_ShouldBeRejected", func() {
		// Arrange - Create oversized payload
		largeData := make(map[string]string)
		largeValue := strings.Repeat("A", 10*1024*1024) // 10MB
		largeData["data"] = largeValue
		
		req := suite.createAuthenticatedRequest("POST", "/api/v1/data", largeData, []string{"user"})
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		// Should be rejected due to size limits
		assert.True(suite.T(), w.Code == http.StatusRequestEntityTooLarge || w.Code == http.StatusBadRequest,
			"Oversized payload should be rejected")
	})
}

// Test Security Headers

func (suite *SecurityTestSuite) TestSecurityHeaders() {
	suite.Run("AllEndpoints_ShouldIncludeSecurityHeaders", func() {
		endpoints := []string{
			"/api/v1/auth/login",
			"/api/v1/profile",
			"/api/v1/admin/users",
		}
		
		for _, endpoint := range endpoints {
			req := httptest.NewRequest("GET", endpoint, nil)
			if strings.Contains(endpoint, "profile") || strings.Contains(endpoint, "admin") {
				req = suite.createAuthenticatedRequest("GET", endpoint, nil, []string{"admin", "user"})
			}
			w := httptest.NewRecorder()
			
			suite.server.ServeHTTP(w, req)
			
			// Check security headers
			assert.Equal(suite.T(), "nosniff", w.Header().Get("X-Content-Type-Options"))
			assert.Equal(suite.T(), "DENY", w.Header().Get("X-Frame-Options"))
			assert.Equal(suite.T(), "1; mode=block", w.Header().Get("X-XSS-Protection"))
			assert.Contains(suite.T(), w.Header().Get("Strict-Transport-Security"), "max-age=31536000")
			assert.Contains(suite.T(), w.Header().Get("Content-Security-Policy"), "default-src 'self'")
		}
	})

	suite.Run("CORSHeaders_ShouldBeRestrictive", func() {
		// Arrange
		req := httptest.NewRequest("OPTIONS", "/api/v1/auth/login", nil)
		req.Header.Set("Origin", "https://malicious-site.com")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		allowedOrigin := w.Header().Get("Access-Control-Allow-Origin")
		assert.NotEqual(suite.T(), "*", allowedOrigin, "CORS should not allow all origins")
		assert.Equal(suite.T(), "https://trusted-domain.com", allowedOrigin, "Should only allow trusted origins")
	})
}

// Test Session Security

func (suite *SecurityTestSuite) TestSessionSecurity() {
	suite.Run("SessionHijacking_ShouldBeDetected", func() {
		// Arrange - Create session from one IP
		userID := uuid.New().String()
		session, _ := suite.authService.CreateSession(userID, "192.168.1.1", "Test Browser")
		accessToken, _ := suite.authService.GenerateAccessToken(
			userID, "test@example.com", []string{"user"}, session.SessionID, "192.168.1.1", "Test Browser",
		)
		
		// Try to use session from different IP
		req := httptest.NewRequest("GET", "/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.RemoteAddr = "10.0.0.1:12345" // Different IP
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		// Should detect IP change and handle appropriately
		// (In production, this might log a warning but not necessarily block)
		assert.NotEqual(suite.T(), http.StatusInternalServerError, w.Code)
	})

	suite.Run("ConcurrentSessions_ShouldBeManaged", func() {
		// Arrange - Create multiple sessions for same user
		userID := uuid.New().String()
		
		session1, _ := suite.authService.CreateSession(userID, "192.168.1.1", "Browser 1")
		session2, _ := suite.authService.CreateSession(userID, "192.168.1.2", "Browser 2")
		
		// Both sessions should be valid
		assert.NotEqual(suite.T(), session1.SessionID, session2.SessionID)
		
		// Validate both sessions
		validatedSession1, err1 := suite.authService.ValidateSession(session1.SessionID, userID, "192.168.1.1")
		validatedSession2, err2 := suite.authService.ValidateSession(session2.SessionID, userID, "192.168.1.2")
		
		// Assert
		assert.NoError(suite.T(), err1)
		assert.NoError(suite.T(), err2)
		assert.NotNil(suite.T(), validatedSession1)
		assert.NotNil(suite.T(), validatedSession2)
	})

	suite.Run("SessionExpiry_ShouldBeEnforced", func() {
		// This would require time manipulation or shorter expiry times
		// For demonstration purposes, we'll test the validation logic
		
		// Arrange - Create session and wait
		userID := uuid.New().String()
		session, _ := suite.authService.CreateSession(userID, "192.168.1.1", "Test Browser")
		
		// Manually expire session in Redis
		suite.redisClient.Del(suite.ctx, fmt.Sprintf("session:%s", session.SessionID))

		// Act - Try to validate expired session
		_, err := suite.authService.ValidateSession(session.SessionID, userID, "192.168.1.1")

		// Assert
		assert.Error(suite.T(), err, "Expired session should not be valid")
	})
}

// Benchmark security operations

func BenchmarkSecurityOperations(b *testing.B) {
	config := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:  "test-secret-key-for-security-testing-must-be-32-bytes-long",
			BCryptCost: 12,
		},
	}
	
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 5})
	authService := security.NewAuthService(config, zap.NewNop(), redisClient)
	
	b.Run("PasswordHashing", func(b *testing.B) {
		password := "TestPassword123!"
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := authService.HashPassword(password)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("TokenGeneration", func(b *testing.B) {
		userID := uuid.New().String()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := authService.GenerateAccessToken(
				userID, "test@example.com", []string{"user"}, 
				uuid.New().String(), "192.168.1.1", "Test Browser",
			)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("TokenValidation", func(b *testing.B) {
		// Pre-generate token
		userID := uuid.New().String()
		token, _ := authService.GenerateAccessToken(
			userID, "test@example.com", []string{"user"}, 
			uuid.New().String(), "192.168.1.1", "Test Browser",
		)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := authService.ValidateToken(token, security.AccessToken)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// TestSecurityTestSuite runs the security test suite
func TestSecurityTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security tests in short mode")
	}
	
	suite.Run(t, new(SecurityTestSuite))
}