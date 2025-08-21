// Package ai provides helper methods for the enterprise AI service
package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MealPlanResponse represents a generated meal plan
type MealPlanResponse struct {
	Days         int                    `json:"days"`
	TotalBudget  float64               `json:"total_budget"`
	DailyMeals   []DayMealPlan         `json:"daily_meals"`
	ShoppingList []ShoppingListItem    `json:"shopping_list"`
	NutritionSummary *NutritionSummary  `json:"nutrition_summary"`
	GeneratedAt  time.Time             `json:"generated_at"`
}

// DayMealPlan represents meals for a single day
type DayMealPlan struct {
	Day       int                 `json:"day"`
	Date      string             `json:"date"`
	Breakfast *MealPlanMeal      `json:"breakfast,omitempty"`
	Lunch     *MealPlanMeal      `json:"lunch,omitempty"`
	Dinner    *MealPlanMeal      `json:"dinner,omitempty"`
	Snacks    []*MealPlanMeal    `json:"snacks,omitempty"`
	DailyCost float64            `json:"daily_cost"`
}

// MealPlanMeal represents a single meal in the plan
type MealPlanMeal struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Ingredients  []outbound.AIIngredient `json:"ingredients"`
	Instructions []string               `json:"instructions"`
	PrepTime     int                    `json:"prep_time_minutes"`
	CookTime     int                    `json:"cook_time_minutes"`
	Servings     int                    `json:"servings"`
	EstimatedCost float64              `json:"estimated_cost"`
	Nutrition    *outbound.NutritionInfo `json:"nutrition"`
}

// ShoppingListItem represents an item in the shopping list
type ShoppingListItem struct {
	Name         string  `json:"name"`
	Amount       float64 `json:"amount"`
	Unit         string  `json:"unit"`
	Category     string  `json:"category"`
	EstimatedCost float64 `json:"estimated_cost"`
	Priority     string  `json:"priority"` // essential, optional, substitute
}

// NutritionSummary provides nutritional overview for the meal plan
type NutritionSummary struct {
	DailyAverages  *outbound.NutritionInfo `json:"daily_averages"`
	TotalNutrition *outbound.NutritionInfo `json:"total_nutrition"`
	HealthScore    float64                 `json:"health_score"`
	BalanceScore   float64                 `json:"balance_score"`
}

// UsageReport represents usage analytics
type UsageReport struct {
	Period           string                 `json:"period"`
	TotalRequests    int64                  `json:"total_requests"`
	RequestsByType   map[string]int64       `json:"requests_by_type"`
	AverageLatency   time.Duration          `json:"average_latency"`
	CacheHitRate     float64                `json:"cache_hit_rate"`
	ErrorRate        float64                `json:"error_rate"`
	TopUsers         []UserUsage            `json:"top_users"`
	HourlyBreakdown  []HourlyUsage          `json:"hourly_breakdown"`
	GeneratedAt      time.Time              `json:"generated_at"`
}

// UserUsage represents usage by a specific user
type UserUsage struct {
	UserID        uuid.UUID `json:"user_id"`
	RequestCount  int64     `json:"request_count"`
	TotalCost     float64   `json:"total_cost"`
	AverageLatency time.Duration `json:"average_latency"`
}

// HourlyUsage represents usage breakdown by hour
type HourlyUsage struct {
	Hour         int     `json:"hour"`
	RequestCount int64   `json:"request_count"`
	AverageLatency time.Duration `json:"average_latency"`
	ErrorCount   int64   `json:"error_count"`
}

// CostReport represents cost analytics
type CostReport struct {
	Period            string                 `json:"period"`
	TotalCostCents    int                    `json:"total_cost_cents"`
	CostByProvider    map[string]int         `json:"cost_by_provider"`
	CostByFeature     map[string]int         `json:"cost_by_feature"`
	TokensUsed        int64                  `json:"tokens_used"`
	AverageCostPerRequest float64           `json:"average_cost_per_request"`
	AverageRCostPerRequest float64          `json:"average_rcost_per_request"`
	TopUsers          []UserCostSummary      `json:"top_users"`
	BudgetUtilization float64               `json:"budget_utilization"`
	DailyBreakdown    []DailyCost            `json:"daily_breakdown"`
	Projections       *CostProjection        `json:"projections"`
	GeneratedAt       time.Time              `json:"generated_at"`
}

// DailyCost represents daily cost breakdown
type DailyCost struct {
	Date        string  `json:"date"`
	CostCents   int     `json:"cost_cents"`
	TokensUsed  int64   `json:"tokens_used"`
	RequestCount int64  `json:"request_count"`
}

// CostProjection provides cost forecasting
type CostProjection struct {
	DailyProjection   float64 `json:"daily_projection"`
	WeeklyProjection  float64 `json:"weekly_projection"`
	MonthlyProjection float64 `json:"monthly_projection"`
	Confidence        float64 `json:"confidence"`
}

// QualityReport represents quality assessment metrics
type QualityReport struct {
	Period              string                    `json:"period"`
	AverageQualityScore float64                   `json:"average_quality_score"`
	QualityByFeature    map[string]float64        `json:"quality_by_feature"`
	QualityTrends       []QualityTrend            `json:"quality_trends"`
	LowQualityAlerts    int                       `json:"low_quality_alerts"`
	ImprovementSuggestions []string              `json:"improvement_suggestions"`
	GeneratedAt         time.Time                 `json:"generated_at"`
}

// QualityTrend represents quality trends over time
type QualityTrend struct {
	Date         string  `json:"date"`
	QualityScore float64 `json:"quality_score"`
	SampleSize   int     `json:"sample_size"`
}

// RateLimitStatus represents current rate limiting status
type RateLimitStatus struct {
	UserID              uuid.UUID `json:"user_id"`
	RequestsThisMinute  int       `json:"requests_this_minute"`
	RequestsThisHour    int       `json:"requests_this_hour"`
	RequestsThisDay     int       `json:"requests_this_day"`
	MinuteLimit         int       `json:"minute_limit"`
	HourLimit           int       `json:"hour_limit"`
	DayLimit            int       `json:"day_limit"`
	MinuteReset         time.Time `json:"minute_reset"`
	HourReset           time.Time `json:"hour_reset"`
	DayReset            time.Time `json:"day_reset"`
	IsLimited           bool      `json:"is_limited"`
}

// HealthStatus represents overall service health
type HealthStatus struct {
	ServiceName string                    `json:"service_name"`
	Status      string                    `json:"status"` // healthy, degraded, unhealthy
	Timestamp   time.Time                 `json:"timestamp"`
	Components  map[string]ComponentHealth `json:"components"`
	Version     string                    `json:"version"`
	Uptime      time.Duration             `json:"uptime"`
}

// ComponentHealth represents health of individual components
type ComponentHealth struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	LastCheck time.Time `json:"last_check"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// Helper methods for EnterpriseAIService

// generateRecipeWithFallback generates recipe with provider fallback
func (s *EnterpriseAIService) generateRecipeWithFallback(ctx context.Context, prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	// Try primary provider
	response, err := s.client.GenerateRecipe(ctx, prompt, constraints)
	if err != nil {
		s.logger.Warn("Primary AI provider failed, trying fallback",
			zap.String("primary_provider", s.provider),
			zap.Error(err))
		
		// Try fallback providers
		for _, provider := range s.config.FallbackProviders {
			var fallbackClient outbound.AIService
			switch provider {
			case "ollama":
				if s.ollamaClient != nil {
					fallbackClient = s.ollamaClient
				}
			case "openai":
				if s.openaiClient != nil {
					fallbackClient = s.openaiClient
				}
			}
			
			if fallbackClient != nil {
				s.logger.Info("Trying fallback provider", zap.String("provider", provider))
				if fallbackResponse, fallbackErr := fallbackClient.GenerateRecipe(ctx, prompt, constraints); fallbackErr == nil {
					s.logger.Info("Fallback provider succeeded", zap.String("provider", provider))
					return fallbackResponse, nil
				}
			}
		}
		
		// Final fallback to enhanced mock
		s.logger.Warn("All AI providers failed, using enhanced mock fallback")
		return s.generateEnhancedMockRecipe(prompt, constraints)
	}
	
	return response, nil
}

// buildCacheKey creates a consistent cache key
func (s *EnterpriseAIService) buildCacheKey(prefix string, parts ...interface{}) string {
	keyParts := []string{prefix}
	for _, part := range parts {
		keyParts = append(keyParts, fmt.Sprintf("%v", part))
	}
	
	key := strings.Join(keyParts, ":")
	
	// Hash long keys to ensure they fit in cache key limits
	if len(key) > 200 {
		hasher := sha256.New()
		hasher.Write([]byte(key))
		return prefix + ":" + hex.EncodeToString(hasher.Sum(nil))[:32]
	}
	
	return key
}

// extractUserIDFromContext extracts user ID from context
func (s *EnterpriseAIService) extractUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(uuid.UUID); ok {
			return id, nil
		}
		if idStr, ok := userID.(string); ok {
			return uuid.Parse(idStr)
		}
	}
	
	// Return a default UUID for anonymous users
	return uuid.New(), nil
}

// getCachedResponse retrieves cached recipe response
func (s *EnterpriseAIService) getCachedResponse(ctx context.Context, key string) (*outbound.AIRecipeResponse, error) {
	data, err := s.cacheRepo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	var response outbound.AIRecipeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	
	return &response, nil
}

// cacheResponse stores recipe response in cache
func (s *EnterpriseAIService) cacheResponse(ctx context.Context, key string, response *outbound.AIRecipeResponse) {
	data, err := json.Marshal(response)
	if err != nil {
		s.logger.Warn("Failed to marshal response for caching", zap.Error(err))
		return
	}
	
	if err := s.cacheRepo.Set(ctx, key, data, s.config.CacheTTL); err != nil {
		s.logger.Warn("Failed to cache response", zap.Error(err))
	}
}

// getCachedStringSlice retrieves cached string slice
func (s *EnterpriseAIService) getCachedStringSlice(ctx context.Context, key string) ([]string, error) {
	data, err := s.cacheRepo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	var result []string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	
	return result, nil
}

// cacheStringSlice stores string slice in cache
func (s *EnterpriseAIService) cacheStringSlice(ctx context.Context, key string, data []string) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		s.logger.Warn("Failed to marshal string slice for caching", zap.Error(err))
		return
	}
	
	if err := s.cacheRepo.Set(ctx, key, jsonData, s.config.CacheTTL); err != nil {
		s.logger.Warn("Failed to cache string slice", zap.Error(err))
	}
}

// getCachedNutrition retrieves cached nutrition info
func (s *EnterpriseAIService) getCachedNutrition(ctx context.Context, key string) (*outbound.NutritionInfo, error) {
	data, err := s.cacheRepo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	var nutrition outbound.NutritionInfo
	if err := json.Unmarshal(data, &nutrition); err != nil {
		return nil, err
	}
	
	return &nutrition, nil
}

// cacheNutrition stores nutrition info in cache
func (s *EnterpriseAIService) cacheNutrition(ctx context.Context, key string, nutrition *outbound.NutritionInfo) {
	data, err := json.Marshal(nutrition)
	if err != nil {
		s.logger.Warn("Failed to marshal nutrition for caching", zap.Error(err))
		return
	}
	
	if err := s.cacheRepo.Set(ctx, key, data, s.config.CacheTTL); err != nil {
		s.logger.Warn("Failed to cache nutrition", zap.Error(err))
	}
}

// getCachedMealPlan retrieves cached meal plan
func (s *EnterpriseAIService) getCachedMealPlan(ctx context.Context, key string) (*MealPlanResponse, error) {
	data, err := s.cacheRepo.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	
	var mealPlan MealPlanResponse
	if err := json.Unmarshal(data, &mealPlan); err != nil {
		return nil, err
	}
	
	return &mealPlan, nil
}

// cacheMealPlan stores meal plan in cache
func (s *EnterpriseAIService) cacheMealPlan(ctx context.Context, key string, mealPlan *MealPlanResponse) {
	data, err := json.Marshal(mealPlan)
	if err != nil {
		s.logger.Warn("Failed to marshal meal plan for caching", zap.Error(err))
		return
	}
	
	if err := s.cacheRepo.Set(ctx, key, data, s.config.CacheTTL); err != nil {
		s.logger.Warn("Failed to cache meal plan", zap.Error(err))
	}
}

// estimateTokenUsage estimates token usage for cost calculation
func (s *EnterpriseAIService) estimateTokenUsage(prompt string, response *outbound.AIRecipeResponse) int {
	// Simple estimation based on text length
	promptTokens := len(strings.Split(prompt, " "))
	
	responseText := response.Title + " " + response.Description + " " + strings.Join(response.Instructions, " ")
	for _, ingredient := range response.Ingredients {
		responseText += " " + ingredient.Name + " " + ingredient.Unit
	}
	responseTokens := len(strings.Split(responseText, " "))
	
	// Apply a multiplier for tokenization overhead
	return int(float64(promptTokens+responseTokens) * 1.3)
}

// buildIngredientSuggestionPrompt creates enhanced ingredient suggestion prompt
func (s *EnterpriseAIService) buildIngredientSuggestionPrompt(partial []string, cuisine string, dietary []string) string {
	prompt := "Suggest complementary ingredients for: " + strings.Join(partial, ", ")
	
	if cuisine != "" {
		prompt += ". Focus on " + cuisine + " cuisine ingredients."
	}
	
	if len(dietary) > 0 {
		prompt += " Ensure suggestions are compatible with: " + strings.Join(dietary, ", ") + "."
	}
	
	prompt += " Provide 5-10 ingredient suggestions that work well together."
	
	return prompt
}

// generateSmartIngredientFallback provides enhanced ingredient suggestions
func (s *EnterpriseAIService) generateSmartIngredientFallback(partial []string, cuisine string, dietary []string) []string {
	// Enhanced fallback logic based on cuisine and dietary restrictions
	suggestions := []string{}
	
	// Base ingredients by cuisine
	cuisineIngredients := map[string][]string{
		"italian":     {"tomatoes", "basil", "mozzarella", "olive oil", "parmesan", "garlic", "oregano"},
		"asian":       {"soy sauce", "ginger", "garlic", "scallions", "sesame oil", "rice vinegar", "chili"},
		"mexican":     {"cumin", "paprika", "lime", "cilantro", "jalapeños", "onions", "bell peppers"},
		"indian":      {"garam masala", "turmeric", "cumin", "coriander", "ginger", "garlic", "tomatoes"},
		"french":      {"butter", "herbs de provence", "white wine", "shallots", "cream", "thyme"},
		"mediterranean": {"olive oil", "lemon", "feta", "olives", "tomatoes", "oregano", "cucumber"},
	}
	
	// Add cuisine-specific ingredients
	if ingredients, exists := cuisineIngredients[strings.ToLower(cuisine)]; exists {
		suggestions = append(suggestions, ingredients...)
	}
	
	// Add general complementary ingredients
	baseIngredients := []string{
		"onion", "garlic", "olive oil", "salt", "pepper", "lemon", "herbs",
		"tomatoes", "carrots", "celery", "potatoes", "cheese", "butter",
	}
	suggestions = append(suggestions, baseIngredients...)
	
	// Filter based on dietary restrictions
	if len(dietary) > 0 {
		filtered := []string{}
		for _, ingredient := range suggestions {
			if s.isDietaryCompliant(ingredient, dietary) {
				filtered = append(filtered, ingredient)
			}
		}
		suggestions = filtered
	}
	
	// Remove duplicates and already included ingredients
	final := []string{}
	seen := make(map[string]bool)
	
	for _, ingredient := range suggestions {
		lower := strings.ToLower(ingredient)
		if !seen[lower] {
			// Check if not already in partial list
			found := false
			for _, existing := range partial {
				if strings.ToLower(existing) == lower {
					found = true
					break
				}
			}
			if !found {
				final = append(final, ingredient)
				seen[lower] = true
			}
		}
	}
	
	// Return first 8 suggestions
	if len(final) > 8 {
		final = final[:8]
	}
	
	return final
}

// isDietaryCompliant checks if ingredient meets dietary restrictions
func (s *EnterpriseAIService) isDietaryCompliant(ingredient string, dietary []string) bool {
	lower := strings.ToLower(ingredient)
	
	for _, restriction := range dietary {
		switch strings.ToLower(restriction) {
		case "vegetarian":
			if s.isMeat(lower) {
				return false
			}
		case "vegan":
			if s.isMeat(lower) || s.isDairy(lower) || s.isEgg(lower) {
				return false
			}
		case "gluten_free", "gluten-free":
			if s.containsGluten(lower) {
				return false
			}
		case "dairy_free", "dairy-free":
			if s.isDairy(lower) {
				return false
			}
		case "nut_free", "nut-free":
			if s.containsNuts(lower) {
				return false
			}
		}
	}
	
	return true
}

// Helper methods for dietary compliance checking
func (s *EnterpriseAIService) isMeat(ingredient string) bool {
	meats := []string{"beef", "chicken", "pork", "lamb", "turkey", "fish", "salmon", "tuna", "bacon", "ham"}
	for _, meat := range meats {
		if strings.Contains(ingredient, meat) {
			return true
		}
	}
	return false
}

func (s *EnterpriseAIService) isDairy(ingredient string) bool {
	dairy := []string{"milk", "cheese", "butter", "cream", "yogurt", "mozzarella", "parmesan", "cheddar"}
	for _, item := range dairy {
		if strings.Contains(ingredient, item) {
			return true
		}
	}
	return false
}

func (s *EnterpriseAIService) isEgg(ingredient string) bool {
	return strings.Contains(ingredient, "egg")
}

func (s *EnterpriseAIService) containsGluten(ingredient string) bool {
	gluten := []string{"wheat", "flour", "bread", "pasta", "barley", "rye", "oats"}
	for _, item := range gluten {
		if strings.Contains(ingredient, item) {
			return true
		}
	}
	return false
}

func (s *EnterpriseAIService) containsNuts(ingredient string) bool {
	nuts := []string{"almond", "peanut", "walnut", "pecan", "cashew", "pistachio", "hazelnut"}
	for _, nut := range nuts {
		if strings.Contains(ingredient, nut) {
			return true
		}
	}
	return false
}

// generateEnhancedNutritionFallback provides better nutrition estimation
func (s *EnterpriseAIService) generateEnhancedNutritionFallback(ingredients []string, servings int) *outbound.NutritionInfo {
	// Enhanced nutrition database with more accurate values
	nutritionDB := map[string]*outbound.NutritionInfo{
		"chicken breast": {Calories: 165, Protein: 31, Carbs: 0, Fat: 3.6, Fiber: 0, Sugar: 0, Sodium: 74},
		"salmon":        {Calories: 208, Protein: 22, Carbs: 0, Fat: 12, Fiber: 0, Sugar: 0, Sodium: 70},
		"beef":          {Calories: 250, Protein: 26, Carbs: 0, Fat: 15, Fiber: 0, Sugar: 0, Sodium: 72},
		"rice":          {Calories: 130, Protein: 2.7, Carbs: 28, Fat: 0.3, Fiber: 0.4, Sugar: 0.1, Sodium: 1},
		"pasta":         {Calories: 131, Protein: 5, Carbs: 25, Fat: 1.1, Fiber: 1.8, Sugar: 0.8, Sodium: 1},
		"bread":         {Calories: 265, Protein: 9, Carbs: 49, Fat: 3.2, Fiber: 2.7, Sugar: 5, Sodium: 491},
		"olive oil":     {Calories: 884, Protein: 0, Carbs: 0, Fat: 100, Fiber: 0, Sugar: 0, Sodium: 2},
		"butter":        {Calories: 717, Protein: 0.85, Carbs: 0.06, Fat: 81, Fiber: 0, Sugar: 0.06, Sodium: 11},
		"cheese":        {Calories: 402, Protein: 25, Carbs: 1.3, Fat: 33, Fiber: 0, Sugar: 0.5, Sodium: 621},
		"tomatoes":      {Calories: 18, Protein: 0.9, Carbs: 3.9, Fat: 0.2, Fiber: 1.2, Sugar: 2.6, Sodium: 5},
		"onion":         {Calories: 40, Protein: 1.1, Carbs: 9.3, Fat: 0.1, Fiber: 1.7, Sugar: 4.2, Sodium: 4},
		"garlic":        {Calories: 149, Protein: 6.4, Carbs: 33, Fat: 0.5, Fiber: 2.1, Sugar: 1, Sodium: 17},
		"carrots":       {Calories: 41, Protein: 0.9, Carbs: 9.6, Fat: 0.2, Fiber: 2.8, Sugar: 4.7, Sodium: 69},
		"broccoli":      {Calories: 34, Protein: 2.8, Carbs: 7, Fat: 0.4, Fiber: 2.6, Sugar: 1.5, Sodium: 33},
		"spinach":       {Calories: 23, Protein: 2.9, Carbs: 3.6, Fat: 0.4, Fiber: 2.2, Sugar: 0.4, Sodium: 79},
	}
	
	totalNutrition := &outbound.NutritionInfo{}
	
	for _, ingredient := range ingredients {
		lower := strings.ToLower(ingredient)
		
		// Try exact match first
		if nutrition, exists := nutritionDB[lower]; exists {
			s.addNutrition(totalNutrition, nutrition)
			continue
		}
		
		// Try partial matches
		found := false
		for key, nutrition := range nutritionDB {
			if strings.Contains(lower, key) || strings.Contains(key, lower) {
				s.addNutrition(totalNutrition, nutrition)
				found = true
				break
			}
		}
		
		// Default values for unknown ingredients
		if !found {
			defaultNutrition := &outbound.NutritionInfo{
				Calories: 50,
				Protein:  2.0,
				Carbs:    8.0,
				Fat:      1.0,
				Fiber:    1.5,
				Sugar:    2.0,
				Sodium:   50.0,
			}
			s.addNutrition(totalNutrition, defaultNutrition)
		}
	}
	
	// Adjust for realistic recipe portions
	s.scaleNutrition(totalNutrition, 0.7) // Assume 70% of raw ingredient values
	
	return totalNutrition
}

// addNutrition adds nutrition values together
func (s *EnterpriseAIService) addNutrition(total, add *outbound.NutritionInfo) {
	total.Calories += add.Calories
	total.Protein += add.Protein
	total.Carbs += add.Carbs
	total.Fat += add.Fat
	total.Fiber += add.Fiber
	total.Sugar += add.Sugar
	total.Sodium += add.Sodium
}

// scaleNutrition scales nutrition values by a factor
func (s *EnterpriseAIService) scaleNutrition(nutrition *outbound.NutritionInfo, factor float64) {
	nutrition.Calories = int(float64(nutrition.Calories) * factor)
	nutrition.Protein *= factor
	nutrition.Carbs *= factor
	nutrition.Fat *= factor
	nutrition.Fiber *= factor
	nutrition.Sugar *= factor
	nutrition.Sodium *= factor
}

// adjustNutritionForServings adjusts nutrition values for serving size
func (s *EnterpriseAIService) adjustNutritionForServings(nutrition *outbound.NutritionInfo, servings int) {
	if servings <= 0 {
		servings = 1
	}
	factor := 1.0 / float64(servings)
	s.scaleNutrition(nutrition, factor)
}

// Additional helper methods continue in enterprise_service_prompts.go due to file size