package user

import (
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestJWTTokenGeneration tests JWT token generation functionality
func TestJWTTokenGeneration(t *testing.T) {
	t.Run("GenerateTokens_ValidUser_ShouldCreateTokens", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		jwtSecret := "test-secret-key-for-testing"
		service := NewUserService(nil, nil, jwtSecret, logger)

		testUser, _ := user.NewUser("test@example.com", "Test User", "password123")

		// Act
		accessToken, refreshToken, err := service.generateTokens(testUser)

		// Assert
		require.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
		assert.NotEqual(t, accessToken, refreshToken)

		// Tokens should be valid JWT format (3 parts separated by dots)
		assert.Len(t, strings.Split(accessToken, "."), 3)
		assert.Len(t, strings.Split(refreshToken, "."), 3)
	})

	t.Run("GenerateTokens_ShouldIncludeCorrectClaims", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		jwtSecret := "test-secret-key-for-testing"
		service := NewUserService(nil, nil, jwtSecret, logger)

		testUser, _ := user.NewUser("test@example.com", "Test User", "password123")

		// Act
		accessToken, _, err := service.generateTokens(testUser)

		// Assert
		require.NoError(t, err)

		// Parse token to verify claims
		token, err := jwt.ParseWithClaims(accessToken, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		require.NoError(t, err)
		require.True(t, token.Valid)

		claims, ok := token.Claims.(*JWTClaims)
		require.True(t, ok)

		// Verify claims content
		assert.Equal(t, testUser.ID(), claims.UserID)
		assert.Equal(t, testUser.Email(), claims.Email)
		assert.Equal(t, string(testUser.Role()), claims.Role)
		assert.Equal(t, testUser.ID().String(), claims.Subject)
		assert.True(t, claims.ExpiresAt > time.Now().Unix())
		assert.True(t, claims.IssuedAt <= time.Now().Unix())
		assert.True(t, claims.NotBefore <= time.Now().Unix())
	})

	t.Run("GenerateTokens_AccessAndRefreshTokens_ShouldHaveDifferentExpirations", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		jwtSecret := "test-secret-key-for-testing"
		service := NewUserService(nil, nil, jwtSecret, logger)

		testUser, _ := user.NewUser("test@example.com", "Test User", "password123")

		// Act
		accessToken, refreshToken, err := service.generateTokens(testUser)

		// Assert
		require.NoError(t, err)

		// Parse both tokens
		accessClaims := &JWTClaims{}
		refreshClaims := &JWTClaims{}

		_, err = jwt.ParseWithClaims(accessToken, accessClaims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		require.NoError(t, err)

		_, err = jwt.ParseWithClaims(refreshToken, refreshClaims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		require.NoError(t, err)

		// Refresh token should expire later than access token
		assert.True(t, refreshClaims.ExpiresAt > accessClaims.ExpiresAt)

		// Access token should expire in ~1 hour, refresh in ~7 days
		accessDuration := time.Unix(accessClaims.ExpiresAt, 0).Sub(time.Unix(accessClaims.IssuedAt, 0))
		refreshDuration := time.Unix(refreshClaims.ExpiresAt, 0).Sub(time.Unix(refreshClaims.IssuedAt, 0))

		assert.InDelta(t, time.Hour.Seconds(), accessDuration.Seconds(), 60)               // Within 1 minute
		assert.InDelta(t, (7 * 24 * time.Hour).Seconds(), refreshDuration.Seconds(), 3600) // Within 1 hour
	})
}

// TestJWTTokenValidation tests JWT token validation functionality
func TestJWTTokenValidation(t *testing.T) {
	t.Run("ValidateToken_ValidToken_ShouldReturnClaims", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		jwtSecret := "test-secret-key-for-validation"
		service := NewUserService(nil, nil, jwtSecret, logger)

		testUser, _ := user.NewUser("test@example.com", "Test User", "password123")
		accessToken, _, _ := service.generateTokens(testUser)

		// Act
		claims, err := service.ValidateToken(accessToken)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, claims)
		assert.Equal(t, testUser.ID(), claims.UserID)
		assert.Equal(t, testUser.Email(), claims.Email)
		assert.Equal(t, string(testUser.Role()), claims.Role)
	})

	t.Run("ValidateToken_InvalidSignature_ShouldFail", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		correctSecret := "correct-secret"
		wrongSecret := "wrong-secret"

		// Create token with one secret
		service1 := NewUserService(nil, nil, correctSecret, logger)
		testUser, _ := user.NewUser("test@example.com", "Test User", "password123")
		accessToken, _, _ := service1.generateTokens(testUser)

		// Try to validate with different secret
		service2 := NewUserService(nil, nil, wrongSecret, logger)

		// Act
		claims, err := service2.ValidateToken(accessToken)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("ValidateToken_ExpiredToken_ShouldFail", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		jwtSecret := "test-secret"
		service := NewUserService(nil, nil, jwtSecret, logger)

		// Create an expired token manually
		testUserID := uuid.New()
		expiredClaims := &JWTClaims{
			UserID: testUserID,
			Email:  "test@example.com",
			Role:   string(user.UserRoleUser),
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(-1 * time.Hour).Unix(), // Expired 1 hour ago
				IssuedAt:  time.Now().Add(-2 * time.Hour).Unix(),
				NotBefore: time.Now().Add(-2 * time.Hour).Unix(),
				Subject:   testUserID.String(),
			},
		}

		expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
		expiredTokenString, _ := expiredToken.SignedString([]byte(jwtSecret))

		// Act
		claims, err := service.ValidateToken(expiredTokenString)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "invalid token")
	})

	t.Run("ValidateToken_MalformedToken_ShouldFail", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		service := NewUserService(nil, nil, "secret", logger)

		// Act & Assert for various malformed tokens
		malformedTokens := []string{
			"",                   // Empty
			"not.a.jwt",          // Not enough parts
			"invalid-jwt-format", // Invalid format
			"header.payload",     // Missing signature
			"a.b.c.d",            // Too many parts
		}

		for _, token := range malformedTokens {
			claims, err := service.ValidateToken(token)
			assert.Error(t, err, "Token '%s' should be invalid", token)
			assert.Nil(t, claims, "Claims should be nil for invalid token '%s'", token)
		}
	})

	t.Run("ValidateToken_WrongSigningMethod_ShouldFail", func(t *testing.T) {
		// Arrange
		logger := zaptest.NewLogger(t)
		jwtSecret := "test-secret"
		service := NewUserService(nil, nil, jwtSecret, logger)

		// Create a token with RS256 instead of HS256
		claims := &JWTClaims{
			UserID: uuid.New(),
			Email:  "test@example.com",
			Role:   string(user.UserRoleUser),
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(time.Hour).Unix(),
				IssuedAt:  time.Now().Unix(),
				NotBefore: time.Now().Unix(),
			},
		}

		// This will create a token with the wrong signing method for our validator
		token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
		tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

		// Act
		validatedClaims, err := service.ValidateToken(tokenString)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, validatedClaims)
	})
}

// TestJWTClaims tests the JWT claims structure
func TestJWTClaims(t *testing.T) {
	t.Run("JWTClaims_ShouldHaveRequiredFields", func(t *testing.T) {
		// Arrange
		userID := uuid.New()
		email := "test@example.com"
		role := string(user.UserRoleAdmin)

		// Act
		claims := JWTClaims{
			UserID: userID,
			Email:  email,
			Role:   role,
			StandardClaims: jwt.StandardClaims{
				Subject: userID.String(),
			},
		}

		// Assert
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, email, claims.Email)
		assert.Equal(t, role, claims.Role)
		assert.Equal(t, userID.String(), claims.Subject)
	})
}

// BenchmarkTokenGeneration benchmarks token generation performance
func BenchmarkTokenGeneration(b *testing.B) {
	logger := zaptest.NewLogger(b)
	service := NewUserService(nil, nil, "benchmark-secret", logger)
	testUser, _ := user.NewUser("bench@example.com", "Benchmark User", "password123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := service.generateTokens(testUser)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTokenValidation benchmarks token validation performance
func BenchmarkTokenValidation(b *testing.B) {
	logger := zaptest.NewLogger(b)
	service := NewUserService(nil, nil, "benchmark-secret", logger)
	testUser, _ := user.NewUser("bench@example.com", "Benchmark User", "password123")
	accessToken, _, _ := service.generateTokens(testUser)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ValidateToken(accessToken)
		if err != nil {
			b.Fatal(err)
		}
	}
}
