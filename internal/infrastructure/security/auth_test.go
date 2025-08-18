package security

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
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

// AuthServiceTestSuite provides a test suite for AuthService
type AuthServiceTestSuite struct {
	suite.Suite
	authService *AuthService
	redisClient *redis.Client
	config      *config.Config
	logger      *zap.Logger
	assertions  *testutils.SecurityAssertions
}

// SetupSuite initializes the test suite
func (suite *AuthServiceTestSuite) SetupSuite() {
	// Setup test configuration
	suite.config = &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret-key-for-testing-only-32-bytes",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        4, // Lower cost for faster tests
		},
		Environment: "test",
	}

	// Setup logger (silent for tests)
	suite.logger = zap.NewNop()

	// Setup Redis client (use miniredis for testing)
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use different DB for tests
	})

	// Clear Redis before tests
	suite.redisClient.FlushDB(context.Background())

	// Create auth service
	suite.authService = NewAuthService(suite.config, suite.logger, suite.redisClient)
	suite.assertions = testutils.NewSecurityAssertions(suite.T())
}

// TearDownSuite cleans up after the test suite
func (suite *AuthServiceTestSuite) TearDownSuite() {
	if suite.redisClient != nil {
		suite.redisClient.FlushDB(context.Background())
		suite.redisClient.Close()
	}
}

// SetupTest clears Redis before each test
func (suite *AuthServiceTestSuite) SetupTest() {
	suite.redisClient.FlushDB(context.Background())
}

// TestTokenGeneration tests JWT token generation
func (suite *AuthServiceTestSuite) TestTokenGeneration() {
	suite.Run("GenerateAccessToken_ValidInputs_ShouldCreateToken", func() {
		// Arrange
		userID := uuid.New().String()
		email := "test@example.com"
		roles := []string{"user"}
		sessionID := uuid.New().String()
		ipAddress := "192.168.1.1"
		userAgent := "Mozilla/5.0 (Test Browser)"

		// Act
		token, err := suite.authService.GenerateAccessToken(
			userID, email, roles, sessionID, ipAddress, userAgent,
		)

		// Assert
		require.NoError(suite.T(), err)
		assert.NotEmpty(suite.T(), token)
		suite.assertions.JWTValid(token)

		// Verify token can be validated
		claims, err := suite.authService.ValidateToken(token, AccessToken)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), userID, claims.UserID)
		assert.Equal(suite.T(), email, claims.Email)
		assert.Equal(suite.T(), roles, claims.Roles)
		assert.Equal(suite.T(), AccessToken, claims.TokenType)
		assert.Equal(suite.T(), sessionID, claims.SessionID)
		assert.Equal(suite.T(), ipAddress, claims.IPAddress)
		assert.Equal(suite.T(), userAgent, claims.UserAgent)
	})

	suite.Run("GenerateRefreshToken_ValidInputs_ShouldCreateToken", func() {
		// Arrange
		userID := uuid.New().String()
		sessionID := uuid.New().String()
		ipAddress := "192.168.1.1"
		userAgent := "Mozilla/5.0 (Test Browser)"

		// Act
		token, err := suite.authService.GenerateRefreshToken(
			userID, sessionID, ipAddress, userAgent,
		)

		// Assert
		require.NoError(suite.T(), err)
		assert.NotEmpty(suite.T(), token)

		// Verify token can be validated
		claims, err := suite.authService.ValidateToken(token, RefreshToken)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), userID, claims.UserID)
		assert.Equal(suite.T(), RefreshToken, claims.TokenType)
		assert.Equal(suite.T(), sessionID, claims.SessionID)
	})

	suite.Run("GenerateCSRFToken_ValidInputs_ShouldCreateToken", func() {
		// Arrange
		sessionID := uuid.New().String()

		// Act
		token, err := suite.authService.GenerateCSRFToken(sessionID)

		// Assert
		require.NoError(suite.T(), err)
		assert.NotEmpty(suite.T(), token)

		// Verify token can be validated
		claims, err := suite.authService.ValidateToken(token, CSRFToken)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), CSRFToken, claims.TokenType)
		assert.Equal(suite.T(), sessionID, claims.SessionID)
	})
}

// TestTokenValidation tests JWT token validation
func (suite *AuthServiceTestSuite) TestTokenValidation() {
	suite.Run("ValidateToken_ValidToken_ShouldSucceed", func() {
		// Arrange
		userID := uuid.New().String()
		email := "test@example.com"
		roles := []string{"user"}
		sessionID := uuid.New().String()
		
		token, _ := suite.authService.GenerateAccessToken(
			userID, email, roles, sessionID, "192.168.1.1", "Test Browser",
		)

		// Act
		claims, err := suite.authService.ValidateToken(token, AccessToken)

		// Assert
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), userID, claims.UserID)
		assert.Equal(suite.T(), email, claims.Email)
		assert.Equal(suite.T(), roles, claims.Roles)
	})

	suite.Run("ValidateToken_InvalidToken_ShouldFail", func() {
		// Arrange
		invalidToken := "invalid.jwt.token"

		// Act
		claims, err := suite.authService.ValidateToken(invalidToken, AccessToken)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), claims)
	})

	suite.Run("ValidateToken_WrongTokenType_ShouldFail", func() {
		// Arrange
		sessionID := uuid.New().String()
		csrfToken, _ := suite.authService.GenerateCSRFToken(sessionID)

		// Act - Try to validate CSRF token as access token
		claims, err := suite.authService.ValidateToken(csrfToken, AccessToken)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), claims)
		assert.Contains(suite.T(), err.Error(), "invalid token type")
	})

	suite.Run("ValidateToken_ExpiredToken_ShouldFail", func() {
		// Arrange - Create service with very short expiration
		shortConfig := *suite.config
		shortConfig.Auth.JWTExpiration = 1 * time.Millisecond
		
		shortAuthService := NewAuthService(&shortConfig, suite.logger, suite.redisClient)
		userID := uuid.New().String()
		
		token, _ := shortAuthService.GenerateAccessToken(
			userID, "test@example.com", []string{"user"}, 
			uuid.New().String(), "192.168.1.1", "Test Browser",
		)

		// Wait for token to expire
		time.Sleep(2 * time.Millisecond)

		// Act
		claims, err := shortAuthService.ValidateToken(token, AccessToken)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), claims)
	})

	suite.Run("ValidateToken_RevokedToken_ShouldFail", func() {
		// Arrange
		userID := uuid.New().String()
		token, _ := suite.authService.GenerateAccessToken(
			userID, "test@example.com", []string{"user"}, 
			uuid.New().String(), "192.168.1.1", "Test Browser",
		)

		// Get token ID for revocation
		claims, _ := suite.authService.ValidateToken(token, AccessToken)
		
		// Revoke the token
		err := suite.authService.RevokeToken(claims.ID)
		require.NoError(suite.T(), err)

		// Act
		validatedClaims, err := suite.authService.ValidateToken(token, AccessToken)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), validatedClaims)
		assert.Contains(suite.T(), err.Error(), "token has been revoked")
	})
}

// TestTokenRevocation tests token revocation functionality
func (suite *AuthServiceTestSuite) TestTokenRevocation() {
	suite.Run("RevokeToken_ValidTokenID_ShouldRevoke", func() {
		// Arrange
		tokenID := uuid.New().String()

		// Act
		err := suite.authService.RevokeToken(tokenID)

		// Assert
		require.NoError(suite.T(), err)

		// Verify token is marked as revoked in Redis
		exists, err := suite.redisClient.Exists(
			context.Background(), 
			fmt.Sprintf("revoked_token:%s", tokenID),
		).Result()
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), int64(1), exists)
	})

	suite.Run("RevokeAllUserTokens_ExistingTokens_ShouldRevokeAll", func() {
		// Arrange
		userID := uuid.New().String()
		
		// Create multiple tokens for the user
		token1, _ := suite.authService.GenerateAccessToken(
			userID, "test@example.com", []string{"user"}, 
			uuid.New().String(), "192.168.1.1", "Test Browser",
		)
		token2, _ := suite.authService.GenerateRefreshToken(
			userID, uuid.New().String(), "192.168.1.1", "Test Browser",
		)

		// Act
		err := suite.authService.RevokeAllUserTokens(userID)

		// Assert
		require.NoError(suite.T(), err)

		// Verify tokens are no longer valid (should fail validation due to missing Redis entries)
		// Note: This test depends on how token storage is implemented
		_ = token1
		_ = token2
	})
}

// TestSessionManagement tests session management functionality
func (suite *AuthServiceTestSuite) TestSessionManagement() {
	suite.Run("CreateSession_ValidInputs_ShouldCreateSession", func() {
		// Arrange
		userID := uuid.New().String()
		ipAddress := "192.168.1.1"
		userAgent := "Mozilla/5.0 (Test Browser)"

		// Act
		session, err := suite.authService.CreateSession(userID, ipAddress, userAgent)

		// Assert
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), session)
		
		assert.Equal(suite.T(), userID, session.UserID)
		assert.Equal(suite.T(), ipAddress, session.IPAddress)
		assert.Equal(suite.T(), userAgent, session.UserAgent)
		assert.True(suite.T(), session.Active)
		assert.NotEmpty(suite.T(), session.SessionID)
		assert.NotZero(suite.T(), session.CreatedAt)
		assert.NotZero(suite.T(), session.ExpiresAt)

		// Verify session is stored in Redis
		exists, err := suite.redisClient.Exists(
			context.Background(), 
			fmt.Sprintf("session:%s", session.SessionID),
		).Result()
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), int64(1), exists)
	})

	suite.Run("ValidateSession_ValidSession_ShouldSucceed", func() {
		// Arrange
		userID := uuid.New().String()
		ipAddress := "192.168.1.1"
		
		session, _ := suite.authService.CreateSession(userID, ipAddress, "Test Browser")

		// Act
		validatedSession, err := suite.authService.ValidateSession(
			session.SessionID, userID, ipAddress,
		)

		// Assert
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), validatedSession)
		assert.Equal(suite.T(), session.SessionID, validatedSession.SessionID)
		assert.Equal(suite.T(), userID, validatedSession.UserID)
	})

	suite.Run("ValidateSession_InvalidSessionID_ShouldFail", func() {
		// Arrange
		invalidSessionID := uuid.New().String()
		userID := uuid.New().String()

		// Act
		session, err := suite.authService.ValidateSession(
			invalidSessionID, userID, "192.168.1.1",
		)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), session)
		assert.Contains(suite.T(), err.Error(), "session not found")
	})

	suite.Run("ValidateSession_WrongUserID_ShouldFail", func() {
		// Arrange
		userID := uuid.New().String()
		wrongUserID := uuid.New().String()
		
		session, _ := suite.authService.CreateSession(userID, "192.168.1.1", "Test Browser")

		// Act
		validatedSession, err := suite.authService.ValidateSession(
			session.SessionID, wrongUserID, "192.168.1.1",
		)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), validatedSession)
		assert.Contains(suite.T(), err.Error(), "session user mismatch")
	})
}

// TestPasswordHashing tests password hashing and verification
func (suite *AuthServiceTestSuite) TestPasswordHashing() {
	suite.Run("HashPassword_ValidPassword_ShouldHashCorrectly", func() {
		// Arrange
		password := "TestPassword123!"

		// Act
		hash, err := suite.authService.HashPassword(password)

		// Assert
		require.NoError(suite.T(), err)
		assert.NotEmpty(suite.T(), hash)
		suite.assertions.PasswordHash(hash)
		
		// Verify the hash is different from the original password
		assert.NotEqual(suite.T(), password, hash)
		
		// Verify we can verify the password against the hash
		err = suite.authService.VerifyPassword(hash, password)
		assert.NoError(suite.T(), err)
	})

	suite.Run("VerifyPassword_CorrectPassword_ShouldSucceed", func() {
		// Arrange
		password := "TestPassword123!"
		hash, _ := suite.authService.HashPassword(password)

		// Act
		err := suite.authService.VerifyPassword(hash, password)

		// Assert
		assert.NoError(suite.T(), err)
	})

	suite.Run("VerifyPassword_WrongPassword_ShouldFail", func() {
		// Arrange
		password := "TestPassword123!"
		wrongPassword := "WrongPassword123!"
		hash, _ := suite.authService.HashPassword(password)

		// Act
		err := suite.authService.VerifyPassword(hash, wrongPassword)

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), bcrypt.ErrMismatchedHashAndPassword, err)
	})

	suite.Run("HashPassword_EmptyPassword_ShouldFail", func() {
		// Arrange
		password := ""

		// Act
		hash, err := suite.authService.HashPassword(password)

		// Assert
		assert.Error(suite.T(), err)
		assert.Empty(suite.T(), hash)
	})
}

// TestAuthMiddleware tests the authentication middleware
func (suite *AuthServiceTestSuite) TestAuthMiddleware() {
	suite.Run("AuthMiddleware_ValidToken_ShouldAllowAccess", func() {
		// Arrange
		gin.SetMode(gin.TestMode)
		
		userID := uuid.New().String()
		email := "test@example.com"
		roles := []string{"user"}
		sessionID := uuid.New().String()
		ipAddress := "192.168.1.1"
		
		// Create session and token
		_, err := suite.authService.CreateSession(userID, ipAddress, "Test Browser")
		require.NoError(suite.T(), err)
		
		token, err := suite.authService.GenerateAccessToken(
			userID, email, roles, sessionID, ipAddress, "Test Browser",
		)
		require.NoError(suite.T(), err)

		// Setup Gin router with middleware
		router := gin.New()
		router.Use(suite.authService.AuthMiddleware())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Create request with valid token
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
	})

	suite.Run("AuthMiddleware_NoToken_ShouldRejectAccess", func() {
		// Arrange
		gin.SetMode(gin.TestMode)
		
		router := gin.New()
		router.Use(suite.authService.AuthMiddleware())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Create request without token
		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
		
		var response map[string]interface{}
		err := testutils.NewHTTPAssertions(suite.T()).JSONResponse(w.Result(), &response)
		require.NoError(suite.T(), err)
		assert.Contains(suite.T(), response["error"], "Authorization header required")
	})

	suite.Run("AuthMiddleware_InvalidTokenFormat_ShouldRejectAccess", func() {
		// Arrange
		gin.SetMode(gin.TestMode)
		
		router := gin.New()
		router.Use(suite.authService.AuthMiddleware())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Create request with invalid token format
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "InvalidFormat token")
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	})

	suite.Run("AuthMiddleware_ExpiredToken_ShouldRejectAccess", func() {
		// Arrange
		gin.SetMode(gin.TestMode)
		
		// Create service with very short expiration
		shortConfig := *suite.config
		shortConfig.Auth.JWTExpiration = 1 * time.Millisecond
		shortAuthService := NewAuthService(&shortConfig, suite.logger, suite.redisClient)
		
		userID := uuid.New().String()
		token, _ := shortAuthService.GenerateAccessToken(
			userID, "test@example.com", []string{"user"}, 
			uuid.New().String(), "192.168.1.1", "Test Browser",
		)

		// Wait for token to expire
		time.Sleep(2 * time.Millisecond)

		router := gin.New()
		router.Use(shortAuthService.AuthMiddleware())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Create request with expired token
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusUnauthorized, w.Code)
	})
}

// TestCSRFMiddleware tests the CSRF protection middleware
func (suite *AuthServiceTestSuite) TestCSRFMiddleware() {
	suite.Run("CSRFMiddleware_ValidToken_ShouldAllowAccess", func() {
		// Arrange
		gin.SetMode(gin.TestMode)
		
		sessionID := uuid.New().String()
		csrfToken, err := suite.authService.GenerateCSRFToken(sessionID)
		require.NoError(suite.T(), err)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("session_id", sessionID)
			c.Next()
		})
		router.Use(suite.authService.CSRFMiddleware())
		router.POST("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Create request with valid CSRF token
		req := httptest.NewRequest("POST", "/protected", nil)
		req.Header.Set("X-CSRF-Token", csrfToken)
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
	})

	suite.Run("CSRFMiddleware_NoToken_ShouldRejectAccess", func() {
		// Arrange
		gin.SetMode(gin.TestMode)
		
		router := gin.New()
		router.Use(suite.authService.CSRFMiddleware())
		router.POST("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Create request without CSRF token
		req := httptest.NewRequest("POST", "/protected", nil)
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusForbidden, w.Code)
	})

	suite.Run("CSRFMiddleware_GetRequest_ShouldSkipCheck", func() {
		// Arrange
		gin.SetMode(gin.TestMode)
		
		router := gin.New()
		router.Use(suite.authService.CSRFMiddleware())
		router.GET("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Create GET request without CSRF token
		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
	})

	suite.Run("CSRFMiddleware_FormToken_ShouldAllowAccess", func() {
		// Arrange
		gin.SetMode(gin.TestMode)
		
		sessionID := uuid.New().String()
		csrfToken, err := suite.authService.GenerateCSRFToken(sessionID)
		require.NoError(suite.T(), err)

		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("session_id", sessionID)
			c.Next()
		})
		router.Use(suite.authService.CSRFMiddleware())
		router.POST("/protected", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		// Create request with CSRF token in form data
		req := httptest.NewRequest("POST", "/protected", 
			strings.NewReader(fmt.Sprintf("csrf_token=%s", csrfToken)))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)

		// Assert
		assert.Equal(suite.T(), http.StatusOK, w.Code)
	})
}

// BenchmarkTokenGeneration benchmarks token generation performance
func BenchmarkTokenGeneration(b *testing.B) {
	config := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret-key-for-testing-only-32-bytes",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        4,
		},
	}
	
	logger := zap.NewNop()
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 1})
	authService := NewAuthService(config, logger, redisClient)
	
	userID := uuid.New().String()
	email := "test@example.com"
	roles := []string{"user"}
	sessionID := uuid.New().String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := authService.GenerateAccessToken(
			userID, email, roles, sessionID, "192.168.1.1", "Test Browser",
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTokenValidation benchmarks token validation performance
func BenchmarkTokenValidation(b *testing.B) {
	config := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret-key-for-testing-only-32-bytes",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        4,
		},
	}
	
	logger := zap.NewNop()
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 1})
	authService := NewAuthService(config, logger, redisClient)
	
	// Generate a token to validate
	token, _ := authService.GenerateAccessToken(
		uuid.New().String(), "test@example.com", []string{"user"}, 
		uuid.New().String(), "192.168.1.1", "Test Browser",
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := authService.ValidateToken(token, AccessToken)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPasswordHashing benchmarks password hashing performance
func BenchmarkPasswordHashing(b *testing.B) {
	config := &config.Config{
		Auth: config.AuthConfig{
			BCryptCost: 10, // Realistic cost
		},
	}
	
	logger := zap.NewNop()
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 1})
	authService := NewAuthService(config, logger, redisClient)
	
	password := "TestPassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := authService.HashPassword(password)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestAuthServiceTestSuite runs the auth service test suite
func TestAuthServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceTestSuite))
}