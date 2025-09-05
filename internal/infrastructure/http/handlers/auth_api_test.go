// Package handlers provides HTTP handlers for authentication API endpoints
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	userApp "github.com/alchemorsel/v3/internal/application/user"
	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *user.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	args := m.Called(ctx, email)
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

func (m *MockCacheRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCacheRepository) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
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
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCacheRepository) SRem(ctx context.Context, key string, members ...string) error {
	args := m.Called(ctx, key, members)
	return args.Error(0)
}

// TestAuthAPIHandlers provides comprehensive tests for authentication HTTP handlers
func TestAuthAPIHandlers(t *testing.T) {
	// Setup test dependencies
	logger := zaptest.NewLogger(t)

	// Create mock services
	mockUserRepo := &MockUserRepository{}
	mockCacheRepo := &MockCacheRepository{}
	userService := userApp.NewUserService(
		mockUserRepo,
		mockCacheRepo,
		"test-jwt-secret",
		logger,
	)

	// Create test config
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-jwt-secret",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        10,
		},
	}

	// Create a mock Redis client (not used in these tests)
	mockRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use test database
	})

	// Create auth service
	authService := security.NewAuthService(cfg, logger, mockRedis)

	// Create handlers
	handlers := NewAuthAPIHandlers(userService, authService, logger)

	t.Run("Register", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			// Setup request
			reqBody := RegisterRequest{
				Name:     "Test User",
				Email:    "test@example.com",
				Password: "securepassword123",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute
			handlers.Register(w, req)

			// Verify response
			assert.Equal(t, http.StatusCreated, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			assert.Equal(t, "User registered successfully", response.Message)
			assert.NotNil(t, response.User)
			assert.Equal(t, reqBody.Name, response.User.Name)
			assert.Equal(t, reqBody.Email, response.User.Email)
			assert.Equal(t, "user", response.User.Role)
			assert.True(t, response.User.IsActive)
		})

		t.Run("InvalidJSON", func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader("invalid-json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handlers.Register(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.False(t, response.Success)
			assert.Equal(t, "Invalid JSON payload", response.Error)
		})

		t.Run("MissingRequiredFields", func(t *testing.T) {
			testCases := []struct {
				name string
				body RegisterRequest
			}{
				{"MissingName", RegisterRequest{Email: "test@example.com", Password: "password123"}},
				{"MissingEmail", RegisterRequest{Name: "Test User", Password: "password123"}},
				{"MissingPassword", RegisterRequest{Name: "Test User", Email: "test@example.com"}},
				{"EmptyName", RegisterRequest{Name: "", Email: "test@example.com", Password: "password123"}},
				{"EmptyEmail", RegisterRequest{Name: "Test User", Email: "", Password: "password123"}},
				{"EmptyPassword", RegisterRequest{Name: "Test User", Email: "test@example.com", Password: ""}},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					body, _ := json.Marshal(tc.body)
					req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
					req.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()

					handlers.Register(w, req)

					assert.Equal(t, http.StatusBadRequest, w.Code)

					var response AuthResponse
					err := json.NewDecoder(w.Body).Decode(&response)
					require.NoError(t, err)

					assert.False(t, response.Success)
					assert.Equal(t, "Name, email, and password are required", response.Error)
				})
			}
		})
	})

	t.Run("Login", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			// Setup request
			reqBody := LoginRequest{
				Email:    "test@example.com",
				Password: "securepassword123",
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute
			handlers.Login(w, req)

			// Verify response
			assert.Equal(t, http.StatusOK, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			assert.Equal(t, "Login successful", response.Message)
			assert.Equal(t, "mock-jwt-access-token", response.AccessToken)
			assert.Equal(t, "mock-jwt-refresh-token", response.RefreshToken)
			assert.Equal(t, int64(3600), response.ExpiresIn)
			assert.NotNil(t, response.User)
			assert.Equal(t, reqBody.Email, response.User.Email)
		})

		t.Run("InvalidJSON", func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("invalid-json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handlers.Login(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.False(t, response.Success)
			assert.Equal(t, "Invalid JSON payload", response.Error)
		})

		t.Run("MissingCredentials", func(t *testing.T) {
			testCases := []struct {
				name string
				body LoginRequest
			}{
				{"MissingEmail", LoginRequest{Password: "password123"}},
				{"MissingPassword", LoginRequest{Email: "test@example.com"}},
				{"EmptyEmail", LoginRequest{Email: "", Password: "password123"}},
				{"EmptyPassword", LoginRequest{Email: "test@example.com", Password: ""}},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					body, _ := json.Marshal(tc.body)
					req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(body))
					req.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()

					handlers.Login(w, req)

					assert.Equal(t, http.StatusBadRequest, w.Code)

					var response AuthResponse
					err := json.NewDecoder(w.Body).Decode(&response)
					require.NoError(t, err)

					assert.False(t, response.Success)
					assert.Equal(t, "Email and password are required", response.Error)
				})
			}
		})
	})

	t.Run("Logout", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
			w := httptest.NewRecorder()

			handlers.Logout(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			assert.Equal(t, "Logout successful", response.Message)
		})
	})

	t.Run("RefreshToken", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
			w := httptest.NewRecorder()

			handlers.RefreshToken(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			assert.Equal(t, "Token refreshed successfully", response.Message)
			assert.Equal(t, "new-mock-jwt-access-token", response.AccessToken)
			assert.Equal(t, int64(3600), response.ExpiresIn)
		})
	})

	t.Run("GetProfile", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v3/auth/profile", nil)
			req.Header.Set("Authorization", "Bearer valid-jwt-token")
			w := httptest.NewRecorder()

			handlers.GetProfile(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			assert.Equal(t, "Profile retrieved successfully", response.Message)

			// Verify user data in response
			userData, ok := response.Data.(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, "mock-user-id", userData["id"])
			assert.Equal(t, "Mock User", userData["name"])
			assert.Equal(t, "user@example.com", userData["email"])
			assert.Equal(t, "user", userData["role"])
			assert.True(t, userData["is_active"].(bool))
		})

		t.Run("MissingAuthHeader", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v3/auth/profile", nil)
			w := httptest.NewRecorder()

			handlers.GetProfile(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.False(t, response.Success)
			assert.Equal(t, "Missing or invalid authorization header", response.Error)
		})

		t.Run("InvalidAuthHeaderFormat", func(t *testing.T) {
			testCases := []struct {
				name   string
				header string
			}{
				{"NoBearerPrefix", "invalid-token"},
				{"EmptyToken", "Bearer "},
				{"WrongPrefix", "Basic dGVzdDp0ZXN0"},
				{"OnlyBearer", "Bearer"},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					req := httptest.NewRequest("GET", "/api/v3/auth/profile", nil)
					req.Header.Set("Authorization", tc.header)
					w := httptest.NewRecorder()

					handlers.GetProfile(w, req)

					assert.Equal(t, http.StatusUnauthorized, w.Code)

					var response AuthResponse
					err := json.NewDecoder(w.Body).Decode(&response)
					require.NoError(t, err)

					assert.False(t, response.Success)
					assert.Contains(t, []string{
						"Missing or invalid authorization header",
						"Invalid token",
					}, response.Error)
				})
			}
		})

		t.Run("EmptyToken", func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v3/auth/profile", nil)
			req.Header.Set("Authorization", "Bearer ")
			w := httptest.NewRecorder()

			handlers.GetProfile(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.False(t, response.Success)
			assert.Equal(t, "Invalid token", response.Error)
		})
	})

	t.Run("UpdateProfile", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			// Setup request with user context
			updateReq := map[string]string{
				"name": "Updated Name",
			}
			body, _ := json.Marshal(updateReq)

			req := httptest.NewRequest("PUT", "/api/v1/auth/profile", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Add user context
			ctx := context.WithValue(req.Context(), "user_id", "test-user-id")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handlers.UpdateProfile(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.True(t, response.Success)
			assert.Equal(t, "Profile updated successfully", response.Message)
		})

		t.Run("Unauthenticated", func(t *testing.T) {
			updateReq := map[string]string{
				"name": "Updated Name",
			}
			body, _ := json.Marshal(updateReq)

			req := httptest.NewRequest("PUT", "/api/v1/auth/profile", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handlers.UpdateProfile(w, req)

			assert.Equal(t, http.StatusUnauthorized, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.False(t, response.Success)
			assert.Equal(t, "User not authenticated", response.Error)
		})

		t.Run("InvalidJSON", func(t *testing.T) {
			req := httptest.NewRequest("PUT", "/api/v1/auth/profile", strings.NewReader("invalid-json"))
			req.Header.Set("Content-Type", "application/json")

			// Add user context
			ctx := context.WithValue(req.Context(), "user_id", "test-user-id")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handlers.UpdateProfile(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response AuthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.False(t, response.Success)
			assert.Equal(t, "Invalid JSON payload", response.Error)
		})
	})
}

// TestAuthAPIHandlersContentType tests content type handling
func TestAuthAPIHandlersContentType(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockUserRepo := &MockUserRepository{}
	mockCacheRepo := &MockCacheRepository{}
	userService := userApp.NewUserService(mockUserRepo, mockCacheRepo, "test-secret", logger)

	// Create test config
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        10,
		},
	}

	mockRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	authService := security.NewAuthService(cfg, logger, mockRedis)
	handlers := NewAuthAPIHandlers(userService, authService, logger)

	t.Run("AllEndpointsReturnJSON", func(t *testing.T) {
		testCases := []struct {
			name         string
			method       string
			path         string
			handler      func(http.ResponseWriter, *http.Request)
			body         string
			setupRequest func(*http.Request) *http.Request
		}{
			{
				name:    "Register",
				method:  "POST",
				path:    "/api/v1/auth/register",
				handler: handlers.Register,
				body:    `{"name":"Test","email":"test@example.com","password":"password123"}`,
			},
			{
				name:    "Login",
				method:  "POST",
				path:    "/api/v1/auth/login",
				handler: handlers.Login,
				body:    `{"email":"test@example.com","password":"password123"}`,
			},
			{
				name:    "Logout",
				method:  "POST",
				path:    "/api/v1/auth/logout",
				handler: handlers.Logout,
			},
			{
				name:    "RefreshToken",
				method:  "POST",
				path:    "/api/v1/auth/refresh",
				handler: handlers.RefreshToken,
			},
			{
				name:    "GetProfile",
				method:  "GET",
				path:    "/api/v3/auth/profile",
				handler: handlers.GetProfile,
				setupRequest: func(req *http.Request) *http.Request {
					req.Header.Set("Authorization", "Bearer test-token")
					return req
				},
			},
			{
				name:    "UpdateProfile",
				method:  "PUT",
				path:    "/api/v1/auth/profile",
				handler: handlers.UpdateProfile,
				body:    `{"name":"Updated Name"}`,
				setupRequest: func(req *http.Request) *http.Request {
					ctx := context.WithValue(req.Context(), "user_id", "test-user-id")
					return req.WithContext(ctx)
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var req *http.Request
				if tc.body != "" {
					req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req = httptest.NewRequest(tc.method, tc.path, nil)
				}

				if tc.setupRequest != nil {
					req = tc.setupRequest(req)
				}

				w := httptest.NewRecorder()
				tc.handler(w, req)

				// All responses should be JSON
				contentType := w.Header().Get("Content-Type")
				assert.Equal(t, "application/json", contentType, "Response should be JSON")

				// Verify response can be parsed as JSON
				var response interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				assert.NoError(t, err, "Response should be valid JSON")
			})
		}
	})
}

// TestAuthAPIHandlersEdgeCases tests edge cases and error conditions
func TestAuthAPIHandlersEdgeCases(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockUserRepo := &MockUserRepository{}
	mockCacheRepo := &MockCacheRepository{}
	userService := userApp.NewUserService(mockUserRepo, mockCacheRepo, "test-secret", logger)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        10,
		},
	}

	mockRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	authService := security.NewAuthService(cfg, logger, mockRedis)
	handlers := NewAuthAPIHandlers(userService, authService, logger)

	t.Run("VeryLongTokenInGetProfile", func(t *testing.T) {
		// Test token truncation logic in GetProfile
		longToken := strings.Repeat("a", 1000)

		req := httptest.NewRequest("GET", "/api/v3/auth/profile", nil)
		req.Header.Set("Authorization", "Bearer "+longToken)
		w := httptest.NewRecorder()

		handlers.GetProfile(w, req)

		// Should still handle long tokens gracefully
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SpecialCharactersInRegistration", func(t *testing.T) {
		reqBody := RegisterRequest{
			Name:     "Test User 测试 🚀",
			Email:    "test+special@example.com",
			Password: "Pässwörd123!@#",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handlers.Register(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response AuthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, reqBody.Name, response.User.Name)
		assert.Equal(t, reqBody.Email, response.User.Email)
	})

	t.Run("TimestampAccuracy", func(t *testing.T) {
		beforeRequest := time.Now()

		reqBody := RegisterRequest{
			Name:     "Test User",
			Email:    "test@example.com",
			Password: "password123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handlers.Register(w, req)

		afterRequest := time.Now()

		var response AuthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		// Verify timestamp is reasonable
		createdAt := response.User.CreatedAt
		assert.True(t, createdAt.After(beforeRequest) || createdAt.Equal(beforeRequest))
		assert.True(t, createdAt.Before(afterRequest) || createdAt.Equal(afterRequest))
	})
}

// TestAuthAPIHandlersIntegration tests integration scenarios
func TestAuthAPIHandlersIntegration(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockUserRepo := &MockUserRepository{}
	mockCacheRepo := &MockCacheRepository{}
	userService := userApp.NewUserService(mockUserRepo, mockCacheRepo, "test-secret", logger)

	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        10,
		},
	}

	mockRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})

	authService := security.NewAuthService(cfg, logger, mockRedis)
	handlers := NewAuthAPIHandlers(userService, authService, logger)

	t.Run("CompleteAuthFlow", func(t *testing.T) {
		// 1. Register
		reqBody := RegisterRequest{
			Name:     "Integration Test User",
			Email:    "integration@example.com",
			Password: "securepassword123",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handlers.Register(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 2. Login
		loginReq := LoginRequest{
			Email:    "integration@example.com",
			Password: "securepassword123",
		}
		loginBody, _ := json.Marshal(loginReq)

		loginHttpReq := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBuffer(loginBody))
		loginHttpReq.Header.Set("Content-Type", "application/json")
		loginW := httptest.NewRecorder()

		handlers.Login(loginW, loginHttpReq)
		assert.Equal(t, http.StatusOK, loginW.Code)

		var loginResponse AuthResponse
		err := json.NewDecoder(loginW.Body).Decode(&loginResponse)
		require.NoError(t, err)

		assert.True(t, loginResponse.Success)
		assert.NotEmpty(t, loginResponse.AccessToken)

		// 3. Get Profile (using mock token)
		profileReq := httptest.NewRequest("GET", "/api/v3/auth/profile", nil)
		profileReq.Header.Set("Authorization", "Bearer "+loginResponse.AccessToken)
		profileW := httptest.NewRecorder()

		handlers.GetProfile(profileW, profileReq)
		assert.Equal(t, http.StatusOK, profileW.Code)

		// 4. Logout
		logoutReq := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
		logoutW := httptest.NewRecorder()

		handlers.Logout(logoutW, logoutReq)
		assert.Equal(t, http.StatusOK, logoutW.Code)
	})
}
