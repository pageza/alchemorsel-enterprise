package user

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

// Mock implementations for User Service integration tests

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	mock.Mock
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{}
}

func (m *MockUserRepository) Create(ctx context.Context, user *user.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *user.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockCacheRepository is a mock implementation of CacheRepository
type MockCacheRepository struct {
	mock.Mock
}

func NewMockCacheRepository() *MockCacheRepository {
	return &MockCacheRepository{}
}

func (m *MockCacheRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCacheRepository) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCacheRepository) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCacheRepository) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockCacheRepository) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string][]byte), args.Error(1)
}

func (m *MockCacheRepository) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	args := m.Called(ctx, items, ttl)
	return args.Error(0)
}

func (m *MockCacheRepository) Increment(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCacheRepository) Decrement(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCacheRepository) SAdd(ctx context.Context, key string, members ...string) error {
	args := m.Called(ctx, key, members)
	return args.Error(0)
}

func (m *MockCacheRepository) SMembers(ctx context.Context, key string) ([]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCacheRepository) SRem(ctx context.Context, key string, members ...string) error {
	args := m.Called(ctx, key, members)
	return args.Error(0)
}

// User Service Integration Tests

// TestUserServiceRegistration tests user registration workflow
func TestUserServiceRegistration(t *testing.T) {
	t.Run("Register_NewUser_ShouldSucceed", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		cmd := RegisterCommand{
			Email:    "test@example.com",
			Name:     "Test User",
			Password: "password123",
		}

		// Mock: User doesn't exist yet
		mockUserRepo.On("FindByEmail", ctx, cmd.Email).Return(nil, errors.New("user not found")).Once()

		// Mock: User creation succeeds
		mockUserRepo.On("Create", ctx, mock.MatchedBy(func(u *user.User) bool {
			return u.Email() == cmd.Email && u.Name() == cmd.Name
		})).Return(nil).Once()

		// Act
		response, err := service.Register(ctx, cmd)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, cmd.Email, response.User.Email)
		assert.Equal(t, cmd.Name, response.User.Name)
		assert.NotEmpty(t, response.AccessToken)
		assert.NotEmpty(t, response.RefreshToken)
		assert.Equal(t, 3600, response.ExpiresIn)
		// UserDTO doesn't expose IsActive/IsVerified fields - check the underlying entity properties
		assert.NotEmpty(t, response.User.ID)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Register_ExistingUser_ShouldFail", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		cmd := RegisterCommand{
			Email:    "existing@example.com",
			Name:     "Existing User",
			Password: "password123",
		}

		// Mock: User already exists
		existingUser, _ := user.NewUser(cmd.Email, cmd.Name, "oldpassword")
		mockUserRepo.On("FindByEmail", ctx, cmd.Email).Return(existingUser, nil).Once()

		// Act
		response, err := service.Register(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "already exists")

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Register_RepositoryError_ShouldFail", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		cmd := RegisterCommand{
			Email:    "test@example.com",
			Name:     "Test User",
			Password: "password123",
		}

		// Mock: User doesn't exist
		mockUserRepo.On("FindByEmail", ctx, cmd.Email).Return(nil, errors.New("user not found")).Once()

		// Mock: Repository save fails
		mockUserRepo.On("Create", ctx, mock.Anything).Return(errors.New("database error")).Once()

		// Act
		response, err := service.Register(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "failed to save user")

		mockUserRepo.AssertExpectations(t)
	})
}

// TestUserServiceLogin tests user login workflow
func TestUserServiceLogin(t *testing.T) {
	t.Run("Login_ValidCredentials_ShouldSucceed", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		email := "test@example.com"
		password := "password123"

		// Create a test user
		testUser, _ := user.NewUser(email, "Test User", password)

		cmd := LoginCommand{
			Email:    email,
			Password: password,
		}

		// Mock: Find user by email
		mockUserRepo.On("FindByEmail", ctx, email).Return(testUser, nil).Once()

		// Mock: Update user after login (last login timestamp)
		mockUserRepo.On("Update", ctx, mock.MatchedBy(func(u *user.User) bool {
			return u.Email() == email && u.LastLoginAt() != nil
		})).Return(nil).Once()

		// Act
		response, err := service.Login(ctx, cmd)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, response)

		assert.Equal(t, email, response.User.Email)
		assert.Equal(t, "Test User", response.User.Name)
		assert.NotEmpty(t, response.AccessToken)
		assert.NotEmpty(t, response.RefreshToken)
		assert.Equal(t, 3600, response.ExpiresIn)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Login_InvalidPassword_ShouldFail", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		email := "test@example.com"
		correctPassword := "correctpassword"
		wrongPassword := "wrongpassword"

		// Create user with correct password
		testUser, _ := user.NewUser(email, "Test User", correctPassword)

		cmd := LoginCommand{
			Email:    email,
			Password: wrongPassword, // Wrong password
		}

		// Mock: Find user by email
		mockUserRepo.On("FindByEmail", ctx, email).Return(testUser, nil).Once()

		// Act
		response, err := service.Login(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid credentials")

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Login_UserNotFound_ShouldFail", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		cmd := LoginCommand{
			Email:    "nonexistent@example.com",
			Password: "password123",
		}

		// Mock: User not found
		mockUserRepo.On("FindByEmail", ctx, cmd.Email).Return(nil, errors.New("user not found")).Once()

		// Act
		response, err := service.Login(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "invalid credentials")

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("Login_InactiveUser_ShouldFail", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		email := "inactive@example.com"
		password := "password123"

		// Create user and deactivate
		testUser, _ := user.NewUser(email, "Inactive User", password)
		testUser.Deactivate()

		cmd := LoginCommand{
			Email:    email,
			Password: password,
		}

		// Mock: Find deactivated user
		mockUserRepo.On("FindByEmail", ctx, email).Return(testUser, nil).Once()

		// Act
		response, err := service.Login(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, response)
		assert.Contains(t, err.Error(), "account is deactivated")

		mockUserRepo.AssertExpectations(t)
	})
}

// TestUserServicePasswordChange tests password change workflow
func TestUserServicePasswordChange(t *testing.T) {
	t.Run("ChangePassword_ValidCurrentPassword_ShouldSucceed", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		userID := uuid.New()
		currentPassword := "oldpassword123"
		newPassword := "newpassword456"

		// Create user with current password
		testUser, _ := user.NewUser("test@example.com", "Test User", currentPassword)

		// Mock: Find user by ID
		mockUserRepo.On("FindByID", ctx, userID).Return(testUser, nil).Once()

		// Mock: Update user with new password
		mockUserRepo.On("Update", ctx, mock.MatchedBy(func(u *user.User) bool {
			// Verify the new password works and old doesn't
			return u.CheckPassword(newPassword) == nil && u.CheckPassword(currentPassword) != nil
		})).Return(nil).Once()

		// Act
		err := service.ChangePassword(ctx, userID, currentPassword, newPassword)

		// Assert
		assert.NoError(t, err)
		mockUserRepo.AssertExpectations(t)
	})

	t.Run("ChangePassword_InvalidCurrentPassword_ShouldFail", func(t *testing.T) {
		// Arrange
		mockUserRepo := NewMockUserRepository()
		mockCache := NewMockCacheRepository()
		logger := zaptest.NewLogger(t)
		service := NewUserService(mockUserRepo, mockCache, "test-secret", logger)

		ctx := context.Background()
		userID := uuid.New()
		currentPassword := "correctpassword"
		wrongCurrentPassword := "wrongpassword"
		newPassword := "newpassword456"

		// Create user with current password
		testUser, _ := user.NewUser("test@example.com", "Test User", currentPassword)

		// Mock: Find user by ID
		mockUserRepo.On("FindByID", ctx, userID).Return(testUser, nil).Once()

		// Act
		err := service.ChangePassword(ctx, userID, wrongCurrentPassword, newPassword)

		// Assert
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "current password is incorrect")

		mockUserRepo.AssertExpectations(t)
	})
}
