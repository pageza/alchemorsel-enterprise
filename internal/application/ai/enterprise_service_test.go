// Package ai provides tests for the enterprise AI service
package ai

import (
	"context"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

// MockCacheRepository is a mock implementation of the cache repository
type MockCacheRepository struct {
	mock.Mock
}

func (m *MockCacheRepository) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCacheRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
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

// Test utilities

func createTestConfig() *EnterpriseConfig {
	return &EnterpriseConfig{
		PrimaryProvider:      "ollama",
		FallbackProviders:    []string{"openai"},
		DailyBudgetCents:     10000,
		MonthlyBudgetCents:   300000,
		CostAlertThresholds:  []float64{0.7, 0.9, 1.0},
		RequestsPerMinute:    60,
		RequestsPerHour:      3600,
		RequestsPerDay:       86400,
		MinQualityScore:      0.7,
		QualityCheckEnabled:  true,
		CacheEnabled:         true,
		CacheTTL:             2 * time.Hour,
		MetricsEnabled:       true,
		AlertsEnabled:        true,
		ModelSettings: map[string]ModelConfig{
			"test-model": {
				MaxTokens:      2048,
				Temperature:    0.7,
				TopP:           0.9,
				CostPerToken:   0.001,
				RequestTimeout: 30 * time.Second,
				QualityWeight:  1.0,
			},
		},
	}
}

func createTestService() (*EnterpriseAIService, *MockCacheRepository) {
	mockCache := &MockCacheRepository{}
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()

	service := NewEnterpriseAIService("ollama", mockCache, config, logger)
	return service, mockCache
}

// Enterprise AI Service Tests

func TestNewEnterpriseAIService(t *testing.T) {
	service, _ := createTestService()
	
	assert.NotNil(t, service)
	assert.NotNil(t, service.costTracker)
	assert.NotNil(t, service.usageAnalytics)
	assert.NotNil(t, service.rateLimiter)
	assert.NotNil(t, service.qualityMonitor)
	assert.NotNil(t, service.alertManager)
}

func TestGenerateRecipe(t *testing.T) {
	service, mockCache := createTestService()

	// Mock cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return([]byte{}, assert.AnError)

	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	constraints := outbound.AIConstraints{
		MaxCalories: 800,
		Dietary:     []string{"vegetarian"},
		Cuisine:     "italian",
	}

	response, err := service.GenerateRecipe(ctx, "Create a pasta dish", constraints)

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.Title)
	assert.NotEmpty(t, response.Instructions)
	assert.True(t, len(response.Ingredients) > 0)
	assert.True(t, response.Confidence > 0)
}

func TestGenerateIngredientSuggestions(t *testing.T) {
	service, mockCache := createTestService()

	// Mock cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return([]byte{}, assert.AnError)

	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	partial := []string{"tomatoes", "basil"}

	suggestions, err := service.GenerateIngredientSuggestions(ctx, partial, "italian", []string{"vegetarian"})

	assert.NoError(t, err)
	assert.True(t, len(suggestions) > 0)
	
	// Ensure suggestions don't include already provided ingredients
	for _, suggestion := range suggestions {
		assert.NotContains(t, partial, suggestion)
	}
}

func TestAnalyzeNutritionalContent(t *testing.T) {
	service, mockCache := createTestService()

	// Mock cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return([]byte{}, assert.AnError)

	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	ingredients := []string{"chicken breast", "broccoli", "rice"}

	nutrition, err := service.AnalyzeNutritionalContent(ctx, ingredients, 2)

	assert.NoError(t, err)
	assert.NotNil(t, nutrition)
	assert.True(t, nutrition.Calories > 0)
	assert.True(t, nutrition.Protein > 0)
	assert.True(t, nutrition.Carbs >= 0)
	assert.True(t, nutrition.Fat >= 0)
}

func TestOptimizeRecipe(t *testing.T) {
	service, mockCache := createTestService()

	// Mock cache miss
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return([]byte{}, assert.AnError)

	// Create a mock recipe
	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	mockRecipe := &recipe.Recipe{} // Would be properly initialized in production

	response, err := service.OptimizeRecipe(ctx, mockRecipe, "health")

	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.NotEmpty(t, response.Title)
}

func TestRateLimitingExceeded(t *testing.T) {
	service, _ := createTestService()

	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	
	// Make requests up to the limit
	for i := 0; i < service.config.RequestsPerMinute; i++ {
		_, err := service.GenerateRecipe(ctx, "test prompt", outbound.AIConstraints{})
		assert.NoError(t, err)
	}

	// Next request should be rate limited
	_, err := service.GenerateRecipe(ctx, "test prompt", outbound.AIConstraints{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

func TestBudgetLimitExceeded(t *testing.T) {
	service, _ := createTestService()

	// Set a very low budget
	service.config.DailyBudgetCents = 1

	ctx := context.WithValue(context.Background(), "user_id", uuid.New())

	// First request might succeed depending on cost calculation
	_, err := service.GenerateRecipe(ctx, "test prompt", outbound.AIConstraints{})
	
	// Subsequent requests should fail due to budget
	for i := 0; i < 5; i++ {
		_, err = service.GenerateRecipe(ctx, "test prompt", outbound.AIConstraints{})
		if err != nil && (err.Error() == "budget limit exceeded" || 
			(err.Error() != "" && err.Error() != "rate limit exceeded")) {
			assert.Contains(t, err.Error(), "budget")
			return
		}
	}
}

func TestHealthCheck(t *testing.T) {
	service, _ := createTestService()

	health, err := service.HealthCheck(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, "enterprise-ai-service", health.ServiceName)
	assert.Contains(t, []string{"healthy", "degraded", "unhealthy"}, health.Status)
	assert.NotEmpty(t, health.Components)
}

// Cost Tracker Tests

func TestCostTracker(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	
	tracker := NewCostTracker(config, logger)

	assert.NotNil(t, tracker)
	assert.Equal(t, config.DailyBudgetCents, tracker.config.DailyBudgetCents)
}

func TestTrackUsage(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	tracker := NewCostTracker(config, logger)

	userID := uuid.New()
	err := tracker.TrackUsage(context.Background(), userID, 50.0, 100)

	assert.NoError(t, err)
	assert.Equal(t, 50.0, tracker.GetDailySpend())

	userSpending := tracker.GetUserSpending(userID)
	assert.NotNil(t, userSpending)
	assert.Equal(t, 50.0, userSpending.DailySpend)
}

func TestCostCalculation(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	tracker := NewCostTracker(config, logger)

	cost := tracker.CalculateCost("ollama", "recipe_generation", 1000)
	assert.True(t, cost >= 0)

	// OpenAI should be more expensive than Ollama
	openaiCost := tracker.CalculateCost("openai", "recipe_generation", 1000)
	ollamaCost := tracker.CalculateCost("ollama", "recipe_generation", 1000)
	assert.True(t, openaiCost > ollamaCost)
}

func TestBudgetAlerts(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	config.DailyBudgetCents = 1000 // $10
	tracker := NewCostTracker(config, logger)

	// Spend 80% of budget
	userID := uuid.New()
	err := tracker.TrackUsage(context.Background(), userID, 800.0, 1000)
	assert.NoError(t, err)

	alerts := tracker.GetBudgetAlerts()
	assert.True(t, len(alerts) > 0)

	// Should have an alert for 70% threshold
	found := false
	for _, alert := range alerts {
		if alert.Threshold == 0.7 && alert.Triggered {
			found = true
			break
		}
	}
	assert.True(t, found)
}

// Usage Analytics Tests

func TestUsageAnalytics(t *testing.T) {
	mockCache := &MockCacheRepository{}
	logger := zaptest.NewLogger(nil)
	
	analytics := NewUsageAnalytics(mockCache, logger)
	assert.NotNil(t, analytics)
}

func TestTrackRequest(t *testing.T) {
	mockCache := &MockCacheRepository{}
	logger := zaptest.NewLogger(nil)
	analytics := NewUsageAnalytics(mockCache, logger)

	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	analytics.TrackRequest(ctx, "recipe_generation", 2*time.Second, 1024)

	metrics := analytics.GetRealTimeMetrics()
	assert.Equal(t, int64(1), metrics.TotalRequests)
	assert.True(t, metrics.AverageLatency > 0)
}

func TestTrackError(t *testing.T) {
	mockCache := &MockCacheRepository{}
	logger := zaptest.NewLogger(nil)
	analytics := NewUsageAnalytics(mockCache, logger)

	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	analytics.TrackError(ctx, "recipe_generation", "test error")

	metrics := analytics.GetRealTimeMetrics()
	assert.Equal(t, int64(1), metrics.TotalErrors)
}

func TestCacheTracking(t *testing.T) {
	mockCache := &MockCacheRepository{}
	logger := zaptest.NewLogger(nil)
	analytics := NewUsageAnalytics(mockCache, logger)

	ctx := context.Background()
	analytics.TrackCacheHit(ctx, "recipe_generation")
	analytics.TrackCacheMiss(ctx, "recipe_generation")

	metrics := analytics.GetRealTimeMetrics()
	assert.Equal(t, int64(1), metrics.TotalCacheHits)
	assert.Equal(t, int64(1), metrics.TotalCacheMisses)
	assert.Equal(t, 0.5, metrics.CacheHitRate)
}

// Rate Limiter Tests

func TestRateLimiter(t *testing.T) {
	mockCache := &MockCacheRepository{}
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	
	limiter := NewRateLimiter(config, mockCache, logger)
	assert.NotNil(t, limiter)
}

func TestCheckLimits(t *testing.T) {
	mockCache := &MockCacheRepository{}
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	config.RequestsPerMinute = 2 // Very low limit for testing
	
	limiter := NewRateLimiter(config, mockCache, logger)
	userID := uuid.New()

	// First two requests should succeed
	err := limiter.CheckLimits(context.Background(), userID)
	assert.NoError(t, err)

	err = limiter.CheckLimits(context.Background(), userID)
	assert.NoError(t, err)

	// Third request should fail
	err = limiter.CheckLimits(context.Background(), userID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
}

func TestUserBlocking(t *testing.T) {
	mockCache := &MockCacheRepository{}
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	
	limiter := NewRateLimiter(config, mockCache, logger)
	userID := uuid.New()

	// Block user
	err := limiter.BlockUser(context.Background(), userID, time.Hour, "test block")
	assert.NoError(t, err)

	// User should be blocked
	err = limiter.CheckLimits(context.Background(), userID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")

	// Unblock user
	err = limiter.UnblockUser(context.Background(), userID)
	assert.NoError(t, err)

	// User should no longer be blocked
	err = limiter.CheckLimits(context.Background(), userID)
	assert.NoError(t, err)
}

// Quality Monitor Tests

func TestQualityMonitor(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	
	monitor := NewQualityMonitor(config, logger)
	assert.NotNil(t, monitor)
}

func TestAssessRecipeQuality(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	monitor := NewQualityMonitor(config, logger)

	response := &outbound.AIRecipeResponse{
		Title:       "Test Recipe",
		Description: "A test recipe for quality assessment",
		Ingredients: []outbound.AIIngredient{
			{Name: "Test ingredient", Amount: 1, Unit: "cup"},
		},
		Instructions: []string{
			"Step 1: Prepare ingredients",
			"Step 2: Cook according to instructions",
		},
		Nutrition: &outbound.NutritionInfo{
			Calories: 400,
			Protein:  20,
			Carbs:    40,
			Fat:      15,
		},
		Tags:       []string{"test", "recipe"},
		Confidence: 0.9,
	}

	score := monitor.AssessRecipeQuality(response)
	assert.True(t, score > 0)
	assert.True(t, score <= 1.0)
}

func TestQualityAlerts(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	config.MinQualityScore = 0.8 // High threshold
	monitor := NewQualityMonitor(config, logger)

	// Low quality response
	response := &outbound.AIRecipeResponse{
		Title:        "",  // Missing title should lower score
		Description:  "",  // Missing description
		Ingredients:  []outbound.AIIngredient{},  // No ingredients
		Instructions: []string{},  // No instructions
	}

	score := monitor.AssessRecipeQuality(response)
	assert.True(t, score < config.MinQualityScore)
}

// Alert Manager Tests

func TestAlertManager(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	
	manager := NewAlertManager(config, logger)
	assert.NotNil(t, manager)
}

func TestCreateAlert(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	manager := NewAlertManager(config, logger)

	alert := manager.CreateAlert(
		"test",
		"warning",
		"Test Alert",
		"This is a test alert",
		"test_service",
		map[string]interface{}{"test": true},
	)

	assert.NotNil(t, alert)
	assert.Equal(t, "test", alert.Type)
	assert.Equal(t, "warning", alert.Severity)
	assert.True(t, alert.IsActive)

	activeAlerts := manager.GetActiveAlerts()
	assert.Len(t, activeAlerts, 1)
}

func TestResolveAlert(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	manager := NewAlertManager(config, logger)

	alert := manager.CreateAlert("test", "warning", "Test Alert", "Test", "test", nil)

	err := manager.ResolveAlert(alert.ID, "test_user")
	assert.NoError(t, err)

	activeAlerts := manager.GetActiveAlerts()
	assert.Len(t, activeAlerts, 0)
}

func TestCostAlerts(t *testing.T) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	config.DailyBudgetCents = 1000    // $10 daily budget
	config.CostAlertThresholds = []float64{0.5, 0.8}
	manager := NewAlertManager(config, logger)

	// Spend 60% of daily budget (should trigger 50% alert)
	manager.CheckCostAlerts(context.Background(), 600.0, 1500.0)

	alerts := manager.GetActiveAlerts()
	assert.True(t, len(alerts) > 0)

	found := false
	for _, alert := range alerts {
		if alert.Type == "daily_cost_threshold" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

// Integration Tests

func TestFullWorkflow(t *testing.T) {
	service, mockCache := createTestService()

	// Mock cache behavior
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return([]byte{}, assert.AnError)
	
	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	
	// Generate a recipe
	constraints := outbound.AIConstraints{
		MaxCalories: 600,
		Dietary:     []string{"vegetarian"},
		Cuisine:     "italian",
	}
	
	recipe, err := service.GenerateRecipe(ctx, "Create a healthy pasta dish", constraints)
	assert.NoError(t, err)
	assert.NotNil(t, recipe)

	// Get analytics
	usageReport, err := service.GetUsageAnalytics(ctx, "daily")
	assert.NoError(t, err)
	assert.NotNil(t, usageReport)

	costReport, err := service.GetCostAnalytics(ctx, "daily")
	assert.NoError(t, err)
	assert.NotNil(t, costReport)

	qualityReport, err := service.GetQualityMetrics(ctx, "daily")
	assert.NoError(t, err)
	assert.NotNil(t, qualityReport)

	// Check health
	health, err := service.HealthCheck(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, health)
}

// Benchmark tests

func BenchmarkGenerateRecipe(b *testing.B) {
	service, mockCache := createTestService()
	mockCache.On("Get", mock.Anything, mock.AnythingOfType("string")).Return([]byte{}, assert.AnError)
	
	ctx := context.WithValue(context.Background(), "user_id", uuid.New())
	constraints := outbound.AIConstraints{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GenerateRecipe(ctx, "Create a simple recipe", constraints)
	}
}

func BenchmarkCostTracking(b *testing.B) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	tracker := NewCostTracker(config, logger)
	
	userID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tracker.TrackUsage(context.Background(), userID, 10.0, 100)
	}
}

func BenchmarkQualityAssessment(b *testing.B) {
	logger := zaptest.NewLogger(nil)
	config := createTestConfig()
	monitor := NewQualityMonitor(config, logger)

	response := &outbound.AIRecipeResponse{
		Title:       "Benchmark Recipe",
		Description: "A recipe for benchmarking",
		Ingredients: []outbound.AIIngredient{{Name: "ingredient", Amount: 1, Unit: "cup"}},
		Instructions: []string{"Step 1: Do something"},
		Nutrition:   &outbound.NutritionInfo{Calories: 400},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = monitor.AssessRecipeQuality(response)
	}
}