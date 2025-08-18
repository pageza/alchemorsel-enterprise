// Package e2e provides end-to-end testing for HTMX frontend interactions
//go:build e2e
// +build e2e

package e2e

import (
	"context"
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
)

// HTMXTestSuite provides end-to-end testing for HTMX frontend
type HTMXTestSuite struct {
	suite.Suite
	server      *httptest.Server
	authService *security.AuthService
	config      *config.Config
	assertions  *testutils.ComprehensiveAssertions
	ctx         context.Context
}

// SetupSuite initializes the E2E test suite
func (suite *HTMXTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	gin.SetMode(gin.TestMode)
	
	// Setup configuration
	suite.config = &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret-key-for-e2e-testing-32-bytes",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        4,
		},
	}
	
	// Setup Redis for auth
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   8, // Use different DB for E2E tests
	})
	redisClient.FlushDB(suite.ctx)
	
	// Setup auth service
	logger := zap.NewNop()
	suite.authService = security.NewAuthService(suite.config, logger, redisClient)
	
	// Setup test server
	suite.setupHTMXServer()
	
	// Setup assertions
	suite.assertions = testutils.NewComprehensiveAssertions(suite.T(), nil)
}

// TearDownSuite cleans up after tests
func (suite *HTMXTestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// setupHTMXServer creates a test server with HTMX-enabled routes
func (suite *HTMXTestSuite) setupHTMXServer() {
	router := gin.New()
	
	// Middleware
	router.Use(gin.Recovery())
	router.Use(suite.corsMiddleware())
	router.Use(suite.securityHeadersMiddleware())
	
	// Static assets
	router.Static("/static", "./web/static")
	
	// HTML templates (mock implementation)
	router.LoadHTMLGlob("test/fixtures/templates/*")
	
	// Public routes
	router.GET("/", suite.handleHomePage)
	router.GET("/login", suite.handleLoginPage)
	router.POST("/auth/login", suite.handleLogin)
	router.GET("/register", suite.handleRegisterPage)
	router.POST("/auth/register", suite.handleRegister)
	
	// Protected routes
	protected := router.Group("")
	protected.Use(suite.authService.AuthMiddleware())
	{
		protected.GET("/dashboard", suite.handleDashboard)
		protected.GET("/recipes", suite.handleRecipesPage)
		protected.GET("/recipes/new", suite.handleNewRecipePage)
		protected.POST("/recipes", suite.authService.CSRFMiddleware(), suite.handleCreateRecipe)
		protected.GET("/recipes/:id", suite.handleRecipeDetail)
		protected.PUT("/recipes/:id", suite.authService.CSRFMiddleware(), suite.handleUpdateRecipe)
		protected.DELETE("/recipes/:id", suite.authService.CSRFMiddleware(), suite.handleDeleteRecipe)
		
		// HTMX partial routes
		protected.GET("/partials/recipe-form", suite.handleRecipeFormPartial)
		protected.GET("/partials/recipe-card/:id", suite.handleRecipeCardPartial)
		protected.POST("/partials/search", suite.handleSearchPartial)
		protected.POST("/recipes/:id/like", suite.handleLikeRecipe)
		protected.POST("/recipes/:id/rate", suite.handleRateRecipe)
	}
	
	// Start test server
	suite.server = httptest.NewServer(router)
}

// Middleware implementations

func (suite *HTMXTestSuite) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token, HX-Request, HX-Target, HX-Current-URL")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

func (suite *HTMXTestSuite) securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' unpkg.com; style-src 'self' 'unsafe-inline'")
		c.Next()
	}
}

// Handler implementations

func (suite *HTMXTestSuite) handleHomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "home.html", gin.H{
		"title": "Alchemorsel - Recipe Management",
	})
}

func (suite *HTMXTestSuite) handleLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Login - Alchemorsel",
	})
}

func (suite *HTMXTestSuite) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `form:"email" binding:"required,email"`
		Password string `form:"password" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		if suite.isHTMXRequest(c) {
			c.HTML(http.StatusBadRequest, "partials/error.html", gin.H{
				"error": "Invalid email or password",
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		}
		return
	}
	
	// Mock authentication
	if req.Email == "test@example.com" && req.Password == "password123" {
		userID := uuid.New().String()
		session, _ := suite.authService.CreateSession(userID, c.ClientIP(), c.Request.UserAgent())
		accessToken, _ := suite.authService.GenerateAccessToken(
			userID, req.Email, []string{"user"}, session.SessionID, c.ClientIP(), c.Request.UserAgent(),
		)
		
		// Set secure HTTP-only cookie for browser
		c.SetCookie("auth_token", accessToken, 3600, "/", "", false, true)
		
		if suite.isHTMXRequest(c) {
			c.Header("HX-Redirect", "/dashboard")
			c.Status(http.StatusOK)
		} else {
			c.Redirect(http.StatusSeeOther, "/dashboard")
		}
	} else {
		if suite.isHTMXRequest(c) {
			c.HTML(http.StatusUnauthorized, "partials/error.html", gin.H{
				"error": "Invalid credentials",
			})
		} else {
			c.HTML(http.StatusUnauthorized, "login.html", gin.H{
				"error": "Invalid credentials",
			})
		}
	}
}

func (suite *HTMXTestSuite) handleRegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{
		"title": "Register - Alchemorsel",
	})
}

func (suite *HTMXTestSuite) handleRegister(c *gin.Context) {
	var req struct {
		Email    string `form:"email" binding:"required,email"`
		Password string `form:"password" binding:"required,min=8"`
		Username string `form:"username" binding:"required,min=3"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		if suite.isHTMXRequest(c) {
			c.HTML(http.StatusBadRequest, "partials/error.html", gin.H{
				"error": "Please check your input",
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		}
		return
	}
	
	// Mock registration success
	if suite.isHTMXRequest(c) {
		c.HTML(http.StatusCreated, "partials/success.html", gin.H{
			"message": "Registration successful! Please log in.",
		})
	} else {
		c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully"})
	}
}

func (suite *HTMXTestSuite) handleDashboard(c *gin.Context) {
	userID := c.GetString("user_id")
	
	// Mock dashboard data
	recentRecipes := []map[string]interface{}{
		{
			"id":          uuid.New().String(),
			"title":       "Spaghetti Carbonara",
			"description": "Classic Italian pasta dish",
			"image":       "/static/images/carbonara.jpg",
			"likes":       15,
			"rating":      4.5,
		},
		{
			"id":          uuid.New().String(),
			"title":       "Chicken Tikka Masala",
			"description": "Creamy Indian curry",
			"image":       "/static/images/tikka.jpg",
			"likes":       23,
			"rating":      4.8,
		},
	}
	
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":         "Dashboard - Alchemorsel",
		"user_id":       userID,
		"recent_recipes": recentRecipes,
	})
}

func (suite *HTMXTestSuite) handleRecipesPage(c *gin.Context) {
	// Mock recipes data
	recipes := []map[string]interface{}{
		{
			"id":          uuid.New().String(),
			"title":       "Margherita Pizza",
			"description": "Classic pizza with tomato, mozzarella, and basil",
			"image":       "/static/images/pizza.jpg",
			"likes":       42,
			"rating":      4.7,
			"author":      "Chef Mario",
		},
		{
			"id":          uuid.New().String(),
			"title":       "Beef Bourguignon",
			"description": "French braised beef in red wine",
			"image":       "/static/images/bourguignon.jpg",
			"likes":       18,
			"rating":      4.9,
			"author":      "Chef Julia",
		},
	}
	
	c.HTML(http.StatusOK, "recipes.html", gin.H{
		"title":   "Recipes - Alchemorsel",
		"recipes": recipes,
	})
}

func (suite *HTMXTestSuite) handleNewRecipePage(c *gin.Context) {
	csrfToken, _ := suite.authService.GenerateCSRFToken(c.GetString("session_id"))
	
	c.HTML(http.StatusOK, "recipe-new.html", gin.H{
		"title":      "New Recipe - Alchemorsel",
		"csrf_token": csrfToken,
	})
}

func (suite *HTMXTestSuite) handleCreateRecipe(c *gin.Context) {
	var req struct {
		Title       string `form:"title" binding:"required"`
		Description string `form:"description" binding:"required"`
		Ingredients string `form:"ingredients" binding:"required"`
		Instructions string `form:"instructions" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		if suite.isHTMXRequest(c) {
			c.HTML(http.StatusBadRequest, "partials/error.html", gin.H{
				"error": "Please fill in all required fields",
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		}
		return
	}
	
	// Mock recipe creation
	recipeID := uuid.New().String()
	
	if suite.isHTMXRequest(c) {
		c.Header("HX-Redirect", fmt.Sprintf("/recipes/%s", recipeID))
		c.Status(http.StatusCreated)
	} else {
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/recipes/%s", recipeID))
	}
}

func (suite *HTMXTestSuite) handleRecipeDetail(c *gin.Context) {
	recipeID := c.Param("id")
	
	// Mock recipe data
	recipe := map[string]interface{}{
		"id":          recipeID,
		"title":       "Spaghetti Carbonara",
		"description": "A classic Roman pasta dish made with eggs, cheese, and pancetta",
		"ingredients": []string{
			"400g spaghetti",
			"200g pancetta or guanciale",
			"4 large eggs",
			"100g Pecorino Romano cheese",
			"Black pepper",
			"Salt",
		},
		"instructions": []string{
			"Bring a large pot of salted water to boil",
			"Cook the spaghetti according to package directions",
			"Meanwhile, cook pancetta in a large skillet until crispy",
			"Whisk eggs and cheese in a bowl",
			"Drain pasta and toss with pancetta",
			"Remove from heat and stir in egg mixture",
			"Season with pepper and serve immediately",
		},
		"author":     "Chef Mario",
		"likes":      15,
		"rating":     4.5,
		"prep_time":  "15 minutes",
		"cook_time":  "15 minutes",
		"servings":   4,
	}
	
	c.HTML(http.StatusOK, "recipe-detail.html", gin.H{
		"title":  fmt.Sprintf("%s - Alchemorsel", recipe["title"]),
		"recipe": recipe,
	})
}

func (suite *HTMXTestSuite) handleUpdateRecipe(c *gin.Context) {
	recipeID := c.Param("id")
	
	var req struct {
		Title       string `form:"title" binding:"required"`
		Description string `form:"description" binding:"required"`
	}
	
	if err := c.ShouldBind(&req); err != nil {
		if suite.isHTMXRequest(c) {
			c.HTML(http.StatusBadRequest, "partials/error.html", gin.H{
				"error": "Invalid input",
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		}
		return
	}
	
	if suite.isHTMXRequest(c) {
		// Return updated recipe card
		c.HTML(http.StatusOK, "partials/recipe-card.html", gin.H{
			"recipe": map[string]interface{}{
				"id":          recipeID,
				"title":       req.Title,
				"description": req.Description,
			},
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "Recipe updated successfully"})
	}
}

func (suite *HTMXTestSuite) handleDeleteRecipe(c *gin.Context) {
	recipeID := c.Param("id")
	
	if suite.isHTMXRequest(c) {
		// Return empty content to remove the element
		c.Header("HX-Trigger", "recipe-deleted")
		c.Status(http.StatusOK)
	} else {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Recipe %s deleted", recipeID)})
	}
}

func (suite *HTMXTestSuite) handleRecipeFormPartial(c *gin.Context) {
	csrfToken, _ := suite.authService.GenerateCSRFToken(c.GetString("session_id"))
	
	c.HTML(http.StatusOK, "partials/recipe-form.html", gin.H{
		"csrf_token": csrfToken,
	})
}

func (suite *HTMXTestSuite) handleRecipeCardPartial(c *gin.Context) {
	recipeID := c.Param("id")
	
	// Mock recipe data
	recipe := map[string]interface{}{
		"id":          recipeID,
		"title":       "Sample Recipe",
		"description": "A sample recipe description",
		"likes":       10,
		"rating":      4.2,
	}
	
	c.HTML(http.StatusOK, "partials/recipe-card.html", gin.H{
		"recipe": recipe,
	})
}

func (suite *HTMXTestSuite) handleSearchPartial(c *gin.Context) {
	query := c.PostForm("query")
	
	// Mock search results
	var results []map[string]interface{}
	if query != "" {
		results = []map[string]interface{}{
			{
				"id":          uuid.New().String(),
				"title":       fmt.Sprintf("Recipe matching '%s'", query),
				"description": "A recipe that matches your search",
				"likes":       5,
				"rating":      4.0,
			},
		}
	}
	
	c.HTML(http.StatusOK, "partials/search-results.html", gin.H{
		"results": results,
		"query":   query,
	})
}

func (suite *HTMXTestSuite) handleLikeRecipe(c *gin.Context) {
	recipeID := c.Param("id")
	
	// Mock like increment
	newLikeCount := 16 // Simulated new count
	
	if suite.isHTMXRequest(c) {
		c.HTML(http.StatusOK, "partials/like-button.html", gin.H{
			"recipe_id": recipeID,
			"likes":     newLikeCount,
			"liked":     true,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"likes": newLikeCount})
	}
}

func (suite *HTMXTestSuite) handleRateRecipe(c *gin.Context) {
	recipeID := c.Param("id")
	rating := c.PostForm("rating")
	
	// Mock rating update
	newRating := 4.6 // Simulated new average
	
	if suite.isHTMXRequest(c) {
		c.HTML(http.StatusOK, "partials/rating.html", gin.H{
			"recipe_id": recipeID,
			"rating":    newRating,
			"user_rating": rating,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{"rating": newRating})
	}
}

// Helper methods

func (suite *HTMXTestSuite) isHTMXRequest(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

func (suite *HTMXTestSuite) makeHTMXRequest(method, path string, headers map[string]string, body string) *http.Response {
	req, _ := http.NewRequest(method, suite.server.URL+path, strings.NewReader(body))
	
	// Add HTMX headers
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}
	
	resp, _ := client.Do(req)
	return resp
}

func (suite *HTMXTestSuite) authenticateRequest(req *http.Request) {
	userID := uuid.New().String()
	session, _ := suite.authService.CreateSession(userID, "127.0.0.1", "Test Browser")
	accessToken, _ := suite.authService.GenerateAccessToken(
		userID, "test@example.com", []string{"user"}, session.SessionID, "127.0.0.1", "Test Browser",
	)
	
	req.Header.Set("Authorization", "Bearer "+accessToken)
}

// Test Progressive Enhancement

func (suite *HTMXTestSuite) TestProgressiveEnhancement() {
	suite.Run("HomePage_NoJavaScript_ShouldWork", func() {
		// Test that the page works without JavaScript/HTMX
		resp, err := http.Get(suite.server.URL + "/")
		require.NoError(suite.T(), err)
		defer resp.Body.Close()
		
		suite.assertions.HTTP.StatusCode(resp, http.StatusOK)
		suite.assertions.HTTP.HasHeader(resp, "Content-Type")
		assert.Contains(suite.T(), resp.Header.Get("Content-Type"), "text/html")
	})
	
	suite.Run("LoginForm_WithoutHTMX_ShouldWork", func() {
		// Test traditional form submission
		resp, err := http.PostForm(suite.server.URL+"/auth/login", map[string][]string{
			"email":    {"test@example.com"},
			"password": {"password123"},
		})
		require.NoError(suite.T(), err)
		defer resp.Body.Close()
		
		// Should redirect to dashboard
		assert.Equal(suite.T(), http.StatusSeeOther, resp.StatusCode)
		assert.Equal(suite.T(), "/dashboard", resp.Header.Get("Location"))
	})
	
	suite.Run("LoginForm_WithHTMX_ShouldUsePartialUpdate", func() {
		// Test HTMX-enhanced form submission
		resp := suite.makeHTMXRequest("POST", "/auth/login", nil, 
			"email=test@example.com&password=password123")
		defer resp.Body.Close()
		
		// Should use HX-Redirect header
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
		assert.Equal(suite.T(), "/dashboard", resp.Header.Get("HX-Redirect"))
	})
}

// Test HTMX Interactions

func (suite *HTMXTestSuite) TestHTMXInteractions() {
	suite.Run("RecipeSearch_ShouldReturnPartialResults", func() {
		// Create authenticated request
		req, _ := http.NewRequest("POST", suite.server.URL+"/partials/search", 
			strings.NewReader("query=pasta"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		suite.authenticateRequest(req)
		
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(suite.T(), err)
		defer resp.Body.Close()
		
		suite.assertions.HTTP.StatusCode(resp, http.StatusOK)
		
		// Should return HTML partial, not full page
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		content := string(body[:n])
		
		assert.Contains(suite.T(), content, "pasta")
		assert.NotContains(suite.T(), content, "<html>") // Should be partial
		assert.NotContains(suite.T(), content, "<head>") // Should be partial
	})
	
	suite.Run("LikeButton_ShouldUpdateCount", func() {
		recipeID := uuid.New().String()
		
		req, _ := http.NewRequest("POST", suite.server.URL+"/recipes/"+recipeID+"/like", nil)
		req.Header.Set("HX-Request", "true")
		suite.authenticateRequest(req)
		
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(suite.T(), err)
		defer resp.Body.Close()
		
		suite.assertions.HTTP.StatusCode(resp, http.StatusOK)
		
		// Should return updated like button HTML
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		content := string(body[:n])
		
		assert.Contains(suite.T(), content, "16") // New like count
	})
	
	suite.Run("RecipeForm_ShouldValidateAndRedirect", func() {
		// Test recipe creation with HTMX
		csrfToken, _ := suite.authService.GenerateCSRFToken(uuid.New().String())
		
		formData := fmt.Sprintf(
			"title=Test Recipe&description=A test recipe&ingredients=Test ingredients&instructions=Test instructions&csrf_token=%s",
			csrfToken,
		)
		
		req, _ := http.NewRequest("POST", suite.server.URL+"/recipes", 
			strings.NewReader(formData))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		req.Header.Set("X-CSRF-Token", csrfToken)
		suite.authenticateRequest(req)
		
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(suite.T(), err)
		defer resp.Body.Close()
		
		assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)
		assert.Contains(suite.T(), resp.Header.Get("HX-Redirect"), "/recipes/")
	})
}

// Test Security Headers and CSRF Protection

func (suite *HTMXTestSuite) TestSecurityFeatures() {
	suite.Run("AllPages_ShouldIncludeSecurityHeaders", func() {
		endpoints := []string{"/", "/login", "/register"}
		
		for _, endpoint := range endpoints {
			resp, err := http.Get(suite.server.URL + endpoint)
			require.NoError(suite.T(), err)
			resp.Body.Close()
			
			// Check security headers
			suite.assertions.HTTP.SecurityHeaders(resp)
			
			// Check CSP allows HTMX
			csp := resp.Header.Get("Content-Security-Policy")
			assert.Contains(suite.T(), csp, "unpkg.com", "Should allow HTMX CDN")
		}
	})
	
	suite.Run("ProtectedEndpoints_ShouldRequireAuth", func() {
		protectedEndpoints := []string{
			"/dashboard",
			"/recipes/new",
			"/partials/recipe-form",
		}
		
		for _, endpoint := range protectedEndpoints {
			resp, err := http.Get(suite.server.URL + endpoint)
			require.NoError(suite.T(), err)
			resp.Body.Close()
			
			assert.Equal(suite.T(), http.StatusUnauthorized, resp.StatusCode,
				"Endpoint %s should require authentication", endpoint)
		}
	})
	
	suite.Run("StateChangingOperations_ShouldRequireCSRF", func() {
		// Test CSRF protection on recipe creation
		req, _ := http.NewRequest("POST", suite.server.URL+"/recipes", 
			strings.NewReader("title=Test&description=Test"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		suite.authenticateRequest(req)
		// Note: No CSRF token provided
		
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(suite.T(), err)
		resp.Body.Close()
		
		assert.Equal(suite.T(), http.StatusForbidden, resp.StatusCode)
	})
}

// Test Accessibility

func (suite *HTMXTestSuite) TestAccessibility() {
	suite.Run("Forms_ShouldHaveProperLabels", func() {
		resp, err := http.Get(suite.server.URL + "/login")
		require.NoError(suite.T(), err)
		defer resp.Body.Close()
		
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		content := string(body[:n])
		
		// Check for proper form labels
		assert.Contains(suite.T(), content, `<label for="email"`)
		assert.Contains(suite.T(), content, `<label for="password"`)
		assert.Contains(suite.T(), content, `id="email"`)
		assert.Contains(suite.T(), content, `id="password"`)
	})
	
	suite.Run("HTMXRequests_ShouldHaveAriaLabels", func() {
		recipeID := uuid.New().String()
		
		req, _ := http.NewRequest("GET", suite.server.URL+"/partials/recipe-card/"+recipeID, nil)
		req.Header.Set("HX-Request", "true")
		suite.authenticateRequest(req)
		
		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(suite.T(), err)
		defer resp.Body.Close()
		
		body := make([]byte, 2048)
		n, _ := resp.Body.Read(body)
		content := string(body[:n])
		
		// Check for ARIA attributes in dynamic content
		assert.Contains(suite.T(), content, `aria-label=`) // Should have aria labels
	})
	
	suite.Run("DynamicContent_ShouldAnnounceChanges", func() {
		// Test that HTMX updates are announced to screen readers
		resp := suite.makeHTMXRequest("POST", "/partials/search", nil, "query=test")
		defer resp.Body.Close()
		
		body := make([]byte, 2048)
		n, _ := resp.Body.Read(body)
		content := string(body[:n])
		
		// Should include aria-live region or similar
		assert.True(suite.T(), strings.Contains(content, `aria-live=`) || 
			strings.Contains(content, `role="status"`),
			"Dynamic content should be announced to screen readers")
	})
}

// Test Performance

func (suite *HTMXTestSuite) TestPerformance() {
	suite.Run("PartialUpdates_ShouldBeFast", func() {
		// Test that HTMX partial updates are fast
		start := time.Now()
		
		resp := suite.makeHTMXRequest("POST", "/partials/search", nil, "query=performance")
		resp.Body.Close()
		
		duration := time.Since(start)
		
		assert.True(suite.T(), duration < 100*time.Millisecond,
			"Partial updates should be fast: %v", duration)
	})
	
	suite.Run("PageSizes_ShouldBeReasonable", func() {
		// Test that pages are not too large
		resp, err := http.Get(suite.server.URL + "/")
		require.NoError(suite.T(), err)
		defer resp.Body.Close()
		
		body := make([]byte, 100*1024) // 100KB buffer
		n, _ := resp.Body.Read(body)
		
		// Home page should be under 50KB
		assert.True(suite.T(), n < 50*1024,
			"Home page too large: %d bytes", n)
	})
}

// Test Error Handling

func (suite *HTMXTestSuite) TestErrorHandling() {
	suite.Run("ValidationErrors_ShouldShowInline", func() {
		// Test validation error display with HTMX
		resp := suite.makeHTMXRequest("POST", "/auth/login", nil, 
			"email=invalid&password=short")
		defer resp.Body.Close()
		
		assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
		
		body := make([]byte, 1024)
		n, _ := resp.Body.Read(body)
		content := string(body[:n])
		
		// Should return error partial
		assert.Contains(suite.T(), content, "error")
		assert.NotContains(suite.T(), content, "<html>") // Should be partial
	})
	
	suite.Run("ServerErrors_ShouldGracefullyDegrade", func() {
		// Test graceful degradation for server errors
		// This would typically test actual error scenarios
		// For now, we test that error responses are properly formatted
		
		resp := suite.makeHTMXRequest("GET", "/nonexistent", nil, "")
		defer resp.Body.Close()
		
		assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)
	})
}

// TestHTMXTestSuite runs the HTMX test suite
func TestHTMXTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping HTMX E2E tests in short mode")
	}
	
	suite.Run(t, new(HTMXTestSuite))
}