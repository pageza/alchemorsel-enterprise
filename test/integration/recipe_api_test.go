// Package integration provides API integration tests
//go:build integration
// +build integration

package integration

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

	"github.com/alchemorsel/v3/internal/application/recipe"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	"github.com/alchemorsel/v3/internal/infrastructure/http/middleware"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

// RecipeAPITestSuite provides integration tests for Recipe API endpoints
type RecipeAPITestSuite struct {
	suite.Suite
	testDB      *testutils.TestDatabase
	server      *gin.Engine
	authService *security.AuthService
	mocks       *testutils.MockServiceContainer
	assertions  *testutils.ComprehensiveAssertions
	config      *config.Config
	ctx         context.Context
}

// SetupSuite initializes the test suite
func (suite *RecipeAPITestSuite) SetupSuite() {
	suite.ctx = context.Background()
	gin.SetMode(gin.TestMode)
	
	// Setup test database
	suite.testDB = testutils.SetupTestDatabase(suite.T())
	err := suite.testDB.RunMigrations()
	require.NoError(suite.T(), err)
	
	// Setup test config
	suite.config = &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret-key-for-testing-only-32-bytes",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        4,
		},
		Server: config.ServerConfig{
			Port:            ":8080",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
		},
		RateLimit: config.RateLimitConfig{
			RequestsPerMinute: 100,
			BurstSize:         10,
		},
	}
	
	// Setup Redis for auth service
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   2, // Use different DB for API tests
	})
	redisClient.FlushDB(suite.ctx)
	
	// Setup auth service
	logger := zap.NewNop()
	suite.authService = security.NewAuthService(suite.config, logger, redisClient)
	
	// Setup mock services
	suite.mocks = testutils.NewMockServiceContainer()
	
	// Setup comprehensive assertions
	suite.assertions = testutils.NewComprehensiveAssertions(suite.T(), suite.testDB)
	
	// Setup Gin server with middleware and routes
	suite.setupServer()
}

// SetupTest prepares each test
func (suite *RecipeAPITestSuite) SetupTest() {
	// Clean database
	suite.testDB.TruncateAllTables()
	suite.testDB.SeedTestData()
	
	// Reset mocks
	suite.mocks = testutils.NewMockServiceContainer()
}

// setupServer configures the Gin server with all middleware and routes
func (suite *RecipeAPITestSuite) setupServer() {
	suite.server = gin.New()
	
	// Add middleware
	suite.server.Use(gin.Recovery())
	suite.server.Use(middleware.CORS())
	suite.server.Use(middleware.SecurityHeaders())
	suite.server.Use(middleware.RateLimit(suite.config.RateLimit))
	suite.server.Use(middleware.RequestLogger(zap.NewNop()))
	
	// Setup routes
	api := suite.server.Group("/api/v1")
	
	// Public routes
	recipes := api.Group("/recipes")
	{
		recipes.GET("", suite.handleGetRecipes)
		recipes.GET("/:id", suite.handleGetRecipe)
		recipes.GET("/search", suite.handleSearchRecipes)
	}
	
	// Protected routes
	protected := api.Group("")
	protected.Use(suite.authService.AuthMiddleware())
	protected.Use(suite.authService.CSRFMiddleware())
	{
		protected.POST("/recipes", suite.handleCreateRecipe)
		protected.PUT("/recipes/:id", suite.handleUpdateRecipe)
		protected.DELETE("/recipes/:id", suite.handleDeleteRecipe)
		protected.POST("/recipes/:id/publish", suite.handlePublishRecipe)
		protected.POST("/recipes/:id/like", suite.handleLikeRecipe)
		protected.POST("/recipes/:id/rate", suite.handleRateRecipe)
	}
}

// Mock handlers for testing (in real implementation, these would be proper handlers)
func (suite *RecipeAPITestSuite) handleGetRecipes(c *gin.Context) {
	// Mock implementation for testing
	recipes := []map[string]interface{}{
		{
			"id":          uuid.New().String(),
			"title":       "Test Recipe",
			"description": "A test recipe",
			"status":      "published",
		},
	}
	c.JSON(http.StatusOK, gin.H{"recipes": recipes})
}

func (suite *RecipeAPITestSuite) handleGetRecipe(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipe ID"})
		return
	}
	
	recipe := map[string]interface{}{
		"id":          id,
		"title":       "Test Recipe",
		"description": "A test recipe",
		"status":      "published",
	}
	c.JSON(http.StatusOK, gin.H{"recipe": recipe})
}

func (suite *RecipeAPITestSuite) handleSearchRecipes(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}
	
	recipes := []map[string]interface{}{
		{
			"id":          uuid.New().String(),
			"title":       fmt.Sprintf("Recipe matching %s", query),
			"description": "A search result",
			"status":      "published",
		},
	}
	c.JSON(http.StatusOK, gin.H{"recipes": recipes})
}

func (suite *RecipeAPITestSuite) handleCreateRecipe(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	
	recipe := map[string]interface{}{
		"id":          uuid.New().String(),
		"title":       req["title"],
		"description": req["description"],
		"status":      "draft",
		"author_id":   c.GetString("user_id"),
	}
	c.JSON(http.StatusCreated, gin.H{"recipe": recipe})
}

func (suite *RecipeAPITestSuite) handleUpdateRecipe(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipe ID"})
		return
	}
	
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	
	recipe := map[string]interface{}{
		"id":          id,
		"title":       req["title"],
		"description": req["description"],
		"status":      "draft",
		"author_id":   c.GetString("user_id"),
	}
	c.JSON(http.StatusOK, gin.H{"recipe": recipe})
}

func (suite *RecipeAPITestSuite) handleDeleteRecipe(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipe ID"})
		return
	}
	
	c.JSON(http.StatusNoContent, nil)
}

func (suite *RecipeAPITestSuite) handlePublishRecipe(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipe ID"})
		return
	}
	
	recipe := map[string]interface{}{
		"id":           id,
		"title":        "Published Recipe",
		"description":  "A published recipe",
		"status":       "published",
		"published_at": time.Now().Format(time.RFC3339),
	}
	c.JSON(http.StatusOK, gin.H{"recipe": recipe})
}

func (suite *RecipeAPITestSuite) handleLikeRecipe(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipe ID"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Recipe liked successfully"})
}

func (suite *RecipeAPITestSuite) handleRateRecipe(c *gin.Context) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid recipe ID"})
		return
	}
	
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}
	
	rating, ok := req["rating"].(float64)
	if !ok || rating < 1 || rating > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rating must be between 1 and 5"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "Recipe rated successfully"})
}

// Helper method to create authenticated request
func (suite *RecipeAPITestSuite) createAuthenticatedRequest(method, url string, body interface{}) (*http.Request, string) {
	// Create test user and session
	userID := uuid.New().String()
	email := "test@example.com"
	roles := []string{"user"}
	sessionID := uuid.New().String()
	ipAddress := "192.168.1.1"
	
	session, err := suite.authService.CreateSession(userID, ipAddress, "Test Browser")
	require.NoError(suite.T(), err)
	
	accessToken, err := suite.authService.GenerateAccessToken(
		userID, email, roles, session.SessionID, ipAddress, "Test Browser",
	)
	require.NoError(suite.T(), err)
	
	csrfToken, err := suite.authService.GenerateCSRFToken(session.SessionID)
	require.NoError(suite.T(), err)
	
	// Create request
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
	
	return req, userID
}

// TestPublicEndpoints tests publicly accessible endpoints
func (suite *RecipeAPITestSuite) TestPublicEndpoints() {
	suite.Run("GetRecipes_NoAuth_ShouldSucceed", func() {
		// Arrange
		req := httptest.NewRequest("GET", "/api/v1/recipes", nil)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusOK)
		suite.assertions.HTTP.SecurityHeaders(w.Result())
		
		var response map[string]interface{}
		suite.assertions.HTTP.JSONResponse(w.Result(), &response)
		
		recipes, exists := response["recipes"]
		assert.True(suite.T(), exists, "Response should contain recipes")
		assert.NotEmpty(suite.T(), recipes, "Should return recipes")
	})

	suite.Run("GetRecipe_ValidID_ShouldSucceed", func() {
		// Arrange
		recipeID := uuid.New().String()
		req := httptest.NewRequest("GET", "/api/v1/recipes/"+recipeID, nil)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusOK)
		
		var response map[string]interface{}
		suite.assertions.HTTP.JSONResponse(w.Result(), &response)
		
		recipe, exists := response["recipe"]
		assert.True(suite.T(), exists, "Response should contain recipe")
		assert.NotNil(suite.T(), recipe, "Recipe should not be nil")
	})

	suite.Run("GetRecipe_InvalidID_ShouldReturnBadRequest", func() {
		// Arrange
		req := httptest.NewRequest("GET", "/api/v1/recipes/invalid-id", nil)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusBadRequest)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "Invalid recipe ID")
	})

	suite.Run("SearchRecipes_WithQuery_ShouldReturnResults", func() {
		// Arrange
		req := httptest.NewRequest("GET", "/api/v1/recipes/search?q=pasta", nil)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusOK)
		
		var response map[string]interface{}
		suite.assertions.HTTP.JSONResponse(w.Result(), &response)
		
		recipes, exists := response["recipes"]
		assert.True(suite.T(), exists, "Response should contain recipes")
		assert.NotEmpty(suite.T(), recipes, "Should return search results")
	})

	suite.Run("SearchRecipes_NoQuery_ShouldReturnBadRequest", func() {
		// Arrange
		req := httptest.NewRequest("GET", "/api/v1/recipes/search", nil)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusBadRequest)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "Search query required")
	})
}

// TestProtectedEndpoints tests authentication-required endpoints
func (suite *RecipeAPITestSuite) TestProtectedEndpoints() {
	suite.Run("CreateRecipe_WithAuth_ShouldSucceed", func() {
		// Arrange
		recipeData := map[string]interface{}{
			"title":       "New Recipe",
			"description": "A new recipe description",
		}
		
		req, userID := suite.createAuthenticatedRequest("POST", "/api/v1/recipes", recipeData)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusCreated)
		
		var response map[string]interface{}
		suite.assertions.HTTP.JSONResponse(w.Result(), &response)
		
		recipe := response["recipe"].(map[string]interface{})
		assert.Equal(suite.T(), "New Recipe", recipe["title"])
		assert.Equal(suite.T(), userID, recipe["author_id"])
		assert.Equal(suite.T(), "draft", recipe["status"])
	})

	suite.Run("CreateRecipe_NoAuth_ShouldReturnUnauthorized", func() {
		// Arrange
		recipeData := map[string]interface{}{
			"title":       "New Recipe",
			"description": "A new recipe description",
		}
		jsonBody, _ := json.Marshal(recipeData)
		req := httptest.NewRequest("POST", "/api/v1/recipes", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusUnauthorized)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "Authorization header required")
	})

	suite.Run("CreateRecipe_InvalidToken_ShouldReturnUnauthorized", func() {
		// Arrange
		recipeData := map[string]interface{}{
			"title":       "New Recipe",
			"description": "A new recipe description",
		}
		jsonBody, _ := json.Marshal(recipeData)
		req := httptest.NewRequest("POST", "/api/v1/recipes", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer invalid-token")
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusUnauthorized)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "Invalid or expired token")
	})

	suite.Run("UpdateRecipe_WithAuth_ShouldSucceed", func() {
		// Arrange
		recipeID := uuid.New().String()
		updateData := map[string]interface{}{
			"title":       "Updated Recipe",
			"description": "Updated description",
		}
		
		req, userID := suite.createAuthenticatedRequest("PUT", "/api/v1/recipes/"+recipeID, updateData)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusOK)
		
		var response map[string]interface{}
		suite.assertions.HTTP.JSONResponse(w.Result(), &response)
		
		recipe := response["recipe"].(map[string]interface{})
		assert.Equal(suite.T(), "Updated Recipe", recipe["title"])
		assert.Equal(suite.T(), userID, recipe["author_id"])
	})

	suite.Run("DeleteRecipe_WithAuth_ShouldSucceed", func() {
		// Arrange
		recipeID := uuid.New().String()
		req, _ := suite.createAuthenticatedRequest("DELETE", "/api/v1/recipes/"+recipeID, nil)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusNoContent)
	})
}

// TestCSRFProtection tests CSRF protection
func (suite *RecipeAPITestSuite) TestCSRFProtection() {
	suite.Run("PostRequest_NoCSRFToken_ShouldReturnForbidden", func() {
		// Arrange
		userID := uuid.New().String()
		email := "test@example.com"
		roles := []string{"user"}
		sessionID := uuid.New().String()
		
		session, err := suite.authService.CreateSession(userID, "192.168.1.1", "Test Browser")
		require.NoError(suite.T(), err)
		
		accessToken, err := suite.authService.GenerateAccessToken(
			userID, email, roles, session.SessionID, "192.168.1.1", "Test Browser",
		)
		require.NoError(suite.T(), err)
		
		recipeData := map[string]interface{}{
			"title":       "New Recipe",
			"description": "A new recipe description",
		}
		jsonBody, _ := json.Marshal(recipeData)
		
		req := httptest.NewRequest("POST", "/api/v1/recipes", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		// Note: No CSRF token provided
		
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusForbidden)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "CSRF token required")
	})

	suite.Run("PostRequest_InvalidCSRFToken_ShouldReturnForbidden", func() {
		// Arrange
		userID := uuid.New().String()
		email := "test@example.com"
		roles := []string{"user"}
		
		session, err := suite.authService.CreateSession(userID, "192.168.1.1", "Test Browser")
		require.NoError(suite.T(), err)
		
		accessToken, err := suite.authService.GenerateAccessToken(
			userID, email, roles, session.SessionID, "192.168.1.1", "Test Browser",
		)
		require.NoError(suite.T(), err)
		
		recipeData := map[string]interface{}{
			"title":       "New Recipe",
			"description": "A new recipe description",
		}
		jsonBody, _ := json.Marshal(recipeData)
		
		req := httptest.NewRequest("POST", "/api/v1/recipes", bytes.NewBuffer(jsonBody))
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("X-CSRF-Token", "invalid-csrf-token")
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusForbidden)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "Invalid CSRF token")
	})
}

// TestInputValidation tests input validation and sanitization
func (suite *RecipeAPITestSuite) TestInputValidation() {
	suite.Run("CreateRecipe_InvalidJSON_ShouldReturnBadRequest", func() {
		// Arrange
		req, _ := suite.createAuthenticatedRequest("POST", "/api/v1/recipes", nil)
		req.Body = http.NoBody
		req.Header.Set("Content-Type", "application/json")
		
		// Override body with invalid JSON
		req = httptest.NewRequest("POST", "/api/v1/recipes", strings.NewReader("invalid json"))
		req.Header.Set("Authorization", req.Header.Get("Authorization"))
		req.Header.Set("X-CSRF-Token", req.Header.Get("X-CSRF-Token"))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusBadRequest)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "Invalid JSON")
	})

	suite.Run("RateRecipe_InvalidRating_ShouldReturnBadRequest", func() {
		// Arrange
		recipeID := uuid.New().String()
		invalidRating := map[string]interface{}{
			"rating": 6, // Invalid - should be 1-5
		}
		
		req, _ := suite.createAuthenticatedRequest("POST", "/api/v1/recipes/"+recipeID+"/rate", invalidRating)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusBadRequest)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "Rating must be between 1 and 5")
	})

	suite.Run("GetRecipe_SQLInjectionAttempt_ShouldBeHandledSafely", func() {
		// Arrange - Try SQL injection in URL parameter
		maliciousID := "'; DROP TABLE recipes; --"
		req := httptest.NewRequest("GET", "/api/v1/recipes/"+maliciousID, nil)
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusBadRequest)
		suite.assertions.HTTP.ErrorResponse(w.Result(), "Invalid recipe ID")
		
		// Verify database is still intact
		count, err := suite.testDB.DB.Query("SELECT COUNT(*) FROM recipes")
		require.NoError(suite.T(), err)
		count.Close()
	})
}

// TestRateLimiting tests rate limiting functionality
func (suite *RecipeAPITestSuite) TestRateLimiting() {
	suite.Run("ExcessiveRequests_ShouldBeRateLimited", func() {
		if testing.Short() {
			suite.T().Skip("Skipping rate limit test in short mode")
		}
		
		// Arrange - Make requests beyond rate limit
		const requestCount = 105 // Above the 100/minute limit
		
		// Act & Assert
		successCount := 0
		rateLimitedCount := 0
		
		for i := 0; i < requestCount; i++ {
			req := httptest.NewRequest("GET", "/api/v1/recipes", nil)
			w := httptest.NewRecorder()
			
			suite.server.ServeHTTP(w, req)
			
			if w.Code == http.StatusOK {
				successCount++
			} else if w.Code == http.StatusTooManyRequests {
				rateLimitedCount++
			}
		}
		
		assert.Greater(suite.T(), rateLimitedCount, 0, "Some requests should be rate limited")
		assert.LessOrEqual(suite.T(), successCount, 100, "Should not exceed rate limit")
	})
}

// TestSecurityHeaders tests security headers
func (suite *RecipeAPITestSuite) TestSecurityHeaders() {
	suite.Run("AllEndpoints_ShouldIncludeSecurityHeaders", func() {
		endpoints := []string{
			"/api/v1/recipes",
			"/api/v1/recipes/" + uuid.New().String(),
			"/api/v1/recipes/search?q=test",
		}
		
		for _, endpoint := range endpoints {
			req := httptest.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			
			suite.server.ServeHTTP(w, req)
			
			suite.assertions.HTTP.SecurityHeaders(w.Result(), 
				"Endpoint %s should include security headers", endpoint)
		}
	})

	suite.Run("ProtectedEndpoints_ShouldIncludeAdditionalHeaders", func() {
		// Arrange
		req, _ := suite.createAuthenticatedRequest("POST", "/api/v1/recipes", map[string]interface{}{
			"title": "Test Recipe",
		})
		w := httptest.NewRecorder()

		// Act
		suite.server.ServeHTTP(w, req)

		// Assert
		suite.assertions.HTTP.SecurityHeaders(w.Result())
		// Additional checks for protected endpoints
		suite.assertions.HTTP.HasHeader(w.Result(), "X-Content-Type-Options")
		suite.assertions.HTTP.Header(w.Result(), "X-Content-Type-Options", "nosniff")
	})
}

// TestConcurrentRequests tests concurrent request handling
func (suite *RecipeAPITestSuite) TestConcurrentRequests() {
	suite.Run("ConcurrentRequests_ShouldBeHandledCorrectly", func() {
		const numRequests = 10
		responses := make(chan *http.Response, numRequests)
		
		// Launch concurrent requests
		for i := 0; i < numRequests; i++ {
			go func() {
				req := httptest.NewRequest("GET", "/api/v1/recipes", nil)
				w := httptest.NewRecorder()
				
				suite.server.ServeHTTP(w, req)
				responses <- w.Result()
			}()
		}
		
		// Collect responses
		successCount := 0
		for i := 0; i < numRequests; i++ {
			resp := <-responses
			if resp.StatusCode == http.StatusOK {
				successCount++
			}
		}
		
		assert.Equal(suite.T(), numRequests, successCount, 
			"All concurrent requests should succeed")
	})
}

// TestAPIPerformance tests API performance characteristics
func (suite *RecipeAPITestSuite) TestAPIPerformance() {
	suite.Run("SimpleEndpoint_ShouldRespondQuickly", func() {
		// Arrange
		req := httptest.NewRequest("GET", "/api/v1/recipes", nil)
		w := httptest.NewRecorder()

		// Act
		start := time.Now()
		suite.server.ServeHTTP(w, req)
		duration := time.Since(start)

		// Assert
		suite.assertions.HTTP.StatusCode(w.Result(), http.StatusOK)
		suite.assertions.Performance.ResponseTime(duration, 100*time.Millisecond,
			"Simple endpoint should respond within 100ms")
	})
}

// BenchmarkAPIEndpoints benchmarks API endpoint performance
func BenchmarkAPIEndpoints(b *testing.B) {
	// Setup
	gin.SetMode(gin.TestMode)
	testDB := testutils.SetupTestDatabase(&testing.T{})
	defer testDB.Cleanup()
	
	config := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "test-secret-key-for-testing-only-32-bytes",
			JWTExpiration: time.Hour,
			BCryptCost:    4,
		},
	}
	
	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 3})
	authService := security.NewAuthService(config, zap.NewNop(), redisClient)
	
	server := gin.New()
	server.GET("/api/v1/recipes", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"recipes": []string{}})
	})
	
	b.Run("GetRecipes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			req := httptest.NewRequest("GET", "/api/v1/recipes", nil)
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)
		}
	})
}

// TestRecipeAPITestSuite runs the API integration test suite
func TestRecipeAPITestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API integration tests in short mode")
	}
	
	suite.Run(t, new(RecipeAPITestSuite))
}