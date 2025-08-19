// Package ai provides the application layer for AI operations
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/domain/ai"
	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AIService implements AI operations
type AIService struct {
	provider string
	logger   *zap.Logger
}

// NewAIService creates a new AI service
func NewAIService(provider string, logger *zap.Logger) outbound.AIService {
	return &AIService{
		provider: provider,
		logger:   logger.Named("ai-service"),
	}
}

// GenerateRecipe generates a recipe using AI
func (s *AIService) GenerateRecipe(ctx context.Context, prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	s.logger.Info("Generating recipe with AI",
		zap.String("prompt", prompt),
		zap.String("provider", s.provider),
	)

	// For demo purposes, use a mock implementation
	// In production, this would call actual AI services like OpenAI or Anthropic
	if s.provider == "mock" {
		return s.generateMockRecipe(prompt, constraints)
	}

	// TODO: Implement actual AI providers
	return nil, fmt.Errorf("AI provider %s not implemented", s.provider)
}

// SuggestIngredients suggests ingredients based on partial input
func (s *AIService) SuggestIngredients(ctx context.Context, partial []string) ([]string, error) {
	s.logger.Info("Suggesting ingredients", zap.Strings("partial", partial))

	// Mock implementation - suggest complementary ingredients
	suggestions := []string{
		"onion", "garlic", "olive oil", "salt", "pepper",
		"tomatoes", "herbs", "lemon", "cheese", "butter",
	}

	// Filter out already included ingredients
	result := make([]string, 0)
	for _, suggestion := range suggestions {
		include := true
		for _, existing := range partial {
			if strings.ToLower(existing) == strings.ToLower(suggestion) {
				include = false
				break
			}
		}
		if include {
			result = append(result, suggestion)
		}
	}

	// Return first 5 suggestions
	if len(result) > 5 {
		result = result[:5]
	}

	return result, nil
}

// AnalyzeNutrition analyzes nutrition content of ingredients
func (s *AIService) AnalyzeNutrition(ctx context.Context, ingredients []string) (*outbound.NutritionInfo, error) {
	s.logger.Info("Analyzing nutrition", zap.Strings("ingredients", ingredients))

	// Mock nutrition analysis based on ingredient count and type
	calories := len(ingredients) * 50 // Base calories per ingredient
	protein := float64(len(ingredients)) * 2.5
	carbs := float64(len(ingredients)) * 8.0
	fat := float64(len(ingredients)) * 1.5

	// Adjust based on ingredient types (simplified logic)
	for _, ingredient := range ingredients {
		lower := strings.ToLower(ingredient)
		switch {
		case strings.Contains(lower, "meat") || strings.Contains(lower, "chicken") || strings.Contains(lower, "beef"):
			protein += 20
			calories += 100
		case strings.Contains(lower, "oil") || strings.Contains(lower, "butter") || strings.Contains(lower, "cheese"):
			fat += 10
			calories += 80
		case strings.Contains(lower, "rice") || strings.Contains(lower, "pasta") || strings.Contains(lower, "bread"):
			carbs += 30
			calories += 120
		case strings.Contains(lower, "vegetable") || strings.Contains(lower, "fruit"):
			fiber := 2.0
			_ = fiber // Will be added to struct if needed
		}
	}

	return &outbound.NutritionInfo{
		Calories: calories,
		Protein:  protein,
		Carbs:    carbs,
		Fat:      fat,
		Fiber:    5.0,
		Sugar:    10.0,
		Sodium:   500.0,
	}, nil
}

// GenerateDescription generates a description for a recipe
func (s *AIService) GenerateDescription(ctx context.Context, rec *recipe.Recipe) (string, error) {
	s.logger.Info("Generating recipe description")

	// Mock description generation
	descriptions := []string{
		"A delicious and flavorful dish that combines traditional techniques with modern flavors.",
		"This recipe brings together the perfect balance of taste and nutrition for any occasion.",
		"A crowd-pleasing meal that's both easy to prepare and satisfying to eat.",
		"Experience the rich flavors and aromas of this carefully crafted recipe.",
		"A wholesome dish that celebrates fresh ingredients and bold seasonings.",
	}

	// Return a random description
	rand.Seed(time.Now().UnixNano())
	return descriptions[rand.Intn(len(descriptions))], nil
}

// ClassifyRecipe classifies a recipe's cuisine, category, and difficulty
func (s *AIService) ClassifyRecipe(ctx context.Context, rec *recipe.Recipe) (*outbound.RecipeClassification, error) {
	s.logger.Info("Classifying recipe")

	// Mock classification
	cuisines := []string{"italian", "american", "asian", "mediterranean", "french"}
	categories := []string{"main_course", "appetizer", "dessert", "side_dish"}
	difficulties := []string{"easy", "medium", "hard"}
	dietary := []string{"vegetarian", "gluten_free", "dairy_free"}

	rand.Seed(time.Now().UnixNano())

	return &outbound.RecipeClassification{
		Cuisine:    cuisines[rand.Intn(len(cuisines))],
		Category:   categories[rand.Intn(len(categories))],
		Difficulty: difficulties[rand.Intn(len(difficulties))],
		Dietary:    []string{dietary[rand.Intn(len(dietary))]},
		Confidence: 0.8 + rand.Float64()*0.2, // 0.8 to 1.0
	}, nil
}

// generateMockRecipe generates a mock recipe for demo purposes
func (s *AIService) generateMockRecipe(prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	// Create AI request for tracking
	request, err := ai.NewAIRequest(uuid.New(), prompt, ai.ProviderTypeMock, "recipe-generator-v1")
	if err != nil {
		return nil, fmt.Errorf("failed to create AI request: %w", err)
	}

	// Set constraints as parameters
	constraintsJSON, _ := json.Marshal(constraints)
	request.SetParameter("constraints", string(constraintsJSON))

	// Start processing
	if err := request.StartProcessing(); err != nil {
		return nil, fmt.Errorf("failed to start processing: %w", err)
	}

	// Generate mock recipe based on prompt and constraints
	recipe := s.createMockRecipeFromPrompt(prompt, constraints)

	// Create AI response
	usage := &ai.TokenUsage{
		PromptTokens:     len(strings.Split(prompt, " ")),
		CompletionTokens: 500, // Estimated
		TotalTokens:      len(strings.Split(prompt, " ")) + 500,
	}

	response := ai.NewAIResponse(
		fmt.Sprintf("Generated recipe: %s", recipe.Title),
		usage,
		ai.FinishReasonStop,
	)

	// Complete the request
	if err := request.Complete(response); err != nil {
		return nil, fmt.Errorf("failed to complete AI request: %w", err)
	}

	s.logger.Info("Mock recipe generated successfully",
		zap.String("title", recipe.Title),
		zap.Int("tokens_used", usage.TotalTokens),
	)

	return recipe, nil
}

// createMockRecipeFromPrompt creates a mock recipe based on the prompt
func (s *AIService) createMockRecipeFromPrompt(prompt string, constraints outbound.AIConstraints) *outbound.AIRecipeResponse {
	prompt = strings.ToLower(prompt)

	// Determine recipe type based on prompt
	var title, description string
	var ingredients []outbound.AIIngredient
	var instructions []string
	var tags []string

	if strings.Contains(prompt, "pasta") || strings.Contains(prompt, "spaghetti") {
		title = "AI-Generated Creamy Pasta Delight"
		description = "A rich and creamy pasta dish generated by AI, perfect for any weeknight dinner."
		ingredients = []outbound.AIIngredient{
			{Name: "Pasta", Amount: 400, Unit: "g"},
			{Name: "Heavy cream", Amount: 200, Unit: "ml"},
			{Name: "Parmesan cheese", Amount: 100, Unit: "g"},
			{Name: "Garlic", Amount: 3, Unit: "cloves"},
			{Name: "Olive oil", Amount: 2, Unit: "tbsp"},
		}
		instructions = []string{
			"Cook pasta according to package directions until al dente",
			"Heat olive oil in a large skillet and sauté minced garlic",
			"Add heavy cream and simmer for 2-3 minutes",
			"Stir in grated Parmesan cheese until melted",
			"Toss with cooked pasta and serve immediately",
		}
		tags = []string{"pasta", "creamy", "ai-generated", "quick"}
	} else if strings.Contains(prompt, "chicken") {
		title = "AI-Crafted Herb-Crusted Chicken"
		description = "Tender chicken breast with a flavorful herb crust, designed by artificial intelligence."
		ingredients = []outbound.AIIngredient{
			{Name: "Chicken breast", Amount: 4, Unit: "pieces"},
			{Name: "Fresh herbs", Amount: 0.25, Unit: "cup"},
			{Name: "Breadcrumbs", Amount: 0.5, Unit: "cup"},
			{Name: "Olive oil", Amount: 3, Unit: "tbsp"},
			{Name: "Lemon", Amount: 1, Unit: "piece"},
		}
		instructions = []string{
			"Preheat oven to 375°F (190°C)",
			"Mix herbs, breadcrumbs, and olive oil in a bowl",
			"Coat chicken breasts with the herb mixture",
			"Bake for 25-30 minutes until internal temperature reaches 165°F",
			"Serve with lemon wedges",
		}
		tags = []string{"chicken", "herbs", "ai-generated", "healthy"}
	} else if strings.Contains(prompt, "vegetarian") || strings.Contains(prompt, "vegan") {
		title = "AI-Designed Rainbow Vegetable Bowl"
		description = "A colorful and nutritious vegetable bowl created by AI to maximize flavor and nutrition."
		ingredients = []outbound.AIIngredient{
			{Name: "Quinoa", Amount: 1, Unit: "cup"},
			{Name: "Bell peppers", Amount: 2, Unit: "pieces"},
			{Name: "Zucchini", Amount: 1, Unit: "piece"},
			{Name: "Cherry tomatoes", Amount: 200, Unit: "g"},
			{Name: "Avocado", Amount: 1, Unit: "piece"},
		}
		instructions = []string{
			"Cook quinoa according to package directions",
			"Chop all vegetables into bite-sized pieces",
			"Sauté bell peppers and zucchini until tender",
			"Combine cooked quinoa, sautéed vegetables, and fresh tomatoes",
			"Top with sliced avocado and serve",
		}
		tags = []string{"vegetarian", "healthy", "ai-generated", "colorful"}
	} else {
		// Default fusion recipe
		title = "AI-Fusion Mystery Dish"
		description = "An innovative fusion dish created by AI, combining unexpected flavors for a unique culinary experience."
		ingredients = []outbound.AIIngredient{
			{Name: "Main protein", Amount: 300, Unit: "g"},
			{Name: "Seasonal vegetables", Amount: 2, Unit: "cups"},
			{Name: "Aromatic spices", Amount: 1, Unit: "tsp"},
			{Name: "Cooking oil", Amount: 2, Unit: "tbsp"},
			{Name: "Fresh herbs", Amount: 0.25, Unit: "cup"},
		}
		instructions = []string{
			"Prepare all ingredients according to their specific requirements",
			"Heat oil in a large pan over medium-high heat",
			"Add protein and cook until nearly done",
			"Add vegetables and spices, stir-fry until tender",
			"Garnish with fresh herbs before serving",
		}
		tags = []string{"fusion", "ai-generated", "creative", "experimental"}
	}

	// Apply dietary constraints
	if len(constraints.Dietary) > 0 {
		for _, diet := range constraints.Dietary {
			if !contains(tags, diet) {
				tags = append(tags, diet)
			}
		}
	}

	// Calculate mock nutrition
	nutrition := &outbound.NutritionInfo{
		Calories: len(ingredients) * 60,
		Protein:  float64(len(ingredients)) * 8.0,
		Carbs:    float64(len(ingredients)) * 12.0,
		Fat:      float64(len(ingredients)) * 5.0,
		Fiber:    8.0,
		Sugar:    5.0,
		Sodium:   400.0,
	}

	// Adjust for max calories constraint
	if constraints.MaxCalories > 0 && nutrition.Calories > constraints.MaxCalories {
		ratio := float64(constraints.MaxCalories) / float64(nutrition.Calories)
		nutrition.Calories = constraints.MaxCalories
		nutrition.Protein *= ratio
		nutrition.Carbs *= ratio
		nutrition.Fat *= ratio
	}

	return &outbound.AIRecipeResponse{
		Title:        title,
		Description:  description,
		Ingredients:  ingredients,
		Instructions: instructions,
		Nutrition:    nutrition,
		Tags:         tags,
		Confidence:   0.85, // High confidence for mock recipes
	}
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}