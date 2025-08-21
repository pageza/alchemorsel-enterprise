// Package openai provides OpenAI GPT integration for recipe generation
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"go.uber.org/zap"
)

// Client implements the AIService interface using OpenAI API
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
	logger  *zap.Logger
}

// NewClient creates a new OpenAI client
func NewClient(logger *zap.Logger) *Client {
	// Try to get API key from environment variable first
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		// Also try the prefixed version that would come from config
		apiKey = os.Getenv("ALCHEMORSEL_AI_OPENAI_KEY")
	}
	
	var baseURL string
	if apiKey == "" {
		logger.Info("OpenAI API key not found, using local Ollama (llama3.2:3b) for AI features")
		baseURL = "http://localhost:11434/v1"
		apiKey = "ollama" // Dummy key for Ollama
	} else {
		logger.Info("OpenAI client initialized with API key")
		baseURL = "https://api.openai.com/v1"
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// OpenAI API structures
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Recipe generation response structure for parsing JSON
type RecipeGenResponse struct {
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	Cuisine      string       `json:"cuisine"`
	Difficulty   string       `json:"difficulty"`
	PrepTime     int          `json:"prep_time_minutes"`
	CookTime     int          `json:"cook_time_minutes"`
	Servings     int          `json:"servings"`
	Ingredients  []Ingredient `json:"ingredients"`
	Instructions []string     `json:"instructions"`
	Tags         []string     `json:"tags"`
	Nutrition    *Nutrition   `json:"nutrition,omitempty"`
}

type Ingredient struct {
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"`
}

type Nutrition struct {
	Calories int     `json:"calories"`
	Protein  float64 `json:"protein_g"`
	Carbs    float64 `json:"carbs_g"`
	Fat      float64 `json:"fat_g"`
	Fiber    float64 `json:"fiber_g"`
}

// GenerateRecipe generates a recipe using OpenAI GPT
func (c *Client) GenerateRecipe(ctx context.Context, prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	if c.apiKey == "" {
		return c.generateMockRecipe(prompt, constraints)
	}

	// Create a detailed prompt for recipe generation
	systemPrompt := c.buildSystemPrompt(constraints)
	userPrompt := c.buildUserPrompt(prompt, constraints)

	// Call OpenAI API
	response, err := c.callOpenAI(ctx, systemPrompt, userPrompt)
	if err != nil {
		c.logger.Error("OpenAI API call failed", zap.Error(err))
		// Fallback to mock recipe
		return c.generateMockRecipe(prompt, constraints)
	}

	// Parse the response
	aiResponse, err := c.parseRecipeResponse(response)
	if err != nil {
		c.logger.Error("Failed to parse OpenAI response", zap.Error(err))
		// Fallback to mock recipe
		return c.generateMockRecipe(prompt, constraints)
	}

	return aiResponse, nil
}

// buildSystemPrompt creates the system prompt for recipe generation
func (c *Client) buildSystemPrompt(constraints outbound.AIConstraints) string {
	systemPrompt := `You are an expert chef and recipe developer. Your task is to create detailed, practical recipes that are easy to follow. 

CRITICAL: You must respond with ONLY a valid JSON object in the exact format shown below. Do not include any explanatory text, markdown formatting, or other content outside the JSON.

Required JSON format:
{
  "title": "Recipe Name",
  "description": "Brief description of the dish",
  "cuisine": "cuisine_type",
  "difficulty": "easy|medium|hard",
  "prep_time_minutes": 15,
  "cook_time_minutes": 25,
  "servings": 4,
  "ingredients": [
    {
      "name": "ingredient name",
      "amount": 1.5,
      "unit": "cups"
    }
  ],
  "instructions": [
    "Step 1: Detailed instruction",
    "Step 2: Next step"
  ],
  "tags": ["tag1", "tag2"],
  "nutrition": {
    "calories": 350,
    "protein_g": 25.0,
    "carbs_g": 30.0,
    "fat_g": 15.0,
    "fiber_g": 5.0
  }
}`

	// Add constraints to the system prompt
	if len(constraints.Dietary) > 0 {
		systemPrompt += fmt.Sprintf("\n\nDietary restrictions: %s", strings.Join(constraints.Dietary, ", "))
	}
	if constraints.MaxCalories > 0 {
		systemPrompt += fmt.Sprintf("\nMaximum calories per serving: %d", constraints.MaxCalories)
	}
	if constraints.CookingTime > 0 {
		systemPrompt += fmt.Sprintf("\nMaximum total cooking time: %d minutes", constraints.CookingTime)
	}
	if constraints.SkillLevel != "" {
		systemPrompt += fmt.Sprintf("\nSkill level: %s", constraints.SkillLevel)
	}
	if len(constraints.AvoidIngredients) > 0 {
		systemPrompt += fmt.Sprintf("\nAvoid these ingredients: %s", strings.Join(constraints.AvoidIngredients, ", "))
	}

	systemPrompt += "\n\nRemember: Respond with ONLY valid JSON. No additional text or formatting."

	return systemPrompt
}

// buildUserPrompt creates the user prompt for recipe generation
func (c *Client) buildUserPrompt(prompt string, constraints outbound.AIConstraints) string {
	userPrompt := fmt.Sprintf("Create a recipe for: %s", prompt)

	if constraints.Cuisine != "" {
		userPrompt += fmt.Sprintf("\nCuisine: %s", constraints.Cuisine)
	}
	if constraints.ServingSize > 0 {
		userPrompt += fmt.Sprintf("\nServings: %d", constraints.ServingSize)
	}

	return userPrompt
}

// callOpenAI makes the actual API call to OpenAI or Ollama
func (c *Client) callOpenAI(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Use llama3.2:3b for Ollama, gpt-3.5-turbo for OpenAI
	model := "gpt-3.5-turbo"
	if strings.Contains(c.baseURL, "localhost:11434") {
		model = "llama3.2:3b"
	}
	
	reqBody := ChatCompletionRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.7,
		MaxTokens:   1500,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	c.logger.Info("OpenAI API call successful",
		zap.Int("prompt_tokens", chatResp.Usage.PromptTokens),
		zap.Int("completion_tokens", chatResp.Usage.CompletionTokens),
		zap.Int("total_tokens", chatResp.Usage.TotalTokens),
	)

	return chatResp.Choices[0].Message.Content, nil
}

// parseRecipeResponse parses the JSON response from OpenAI
func (c *Client) parseRecipeResponse(response string) (*outbound.AIRecipeResponse, error) {
	// Clean the response - sometimes GPT includes extra text
	response = strings.TrimSpace(response)
	
	// Find JSON content between braces
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no valid JSON found in response")
	}
	
	jsonStr := response[start : end+1]

	var recipeResp RecipeGenResponse
	if err := json.Unmarshal([]byte(jsonStr), &recipeResp); err != nil {
		c.logger.Error("Failed to parse JSON response", 
			zap.Error(err), 
			zap.String("response", jsonStr))
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Convert to our response format
	aiIngredients := make([]outbound.AIIngredient, len(recipeResp.Ingredients))
	for i, ing := range recipeResp.Ingredients {
		aiIngredients[i] = outbound.AIIngredient{
			Name:   ing.Name,
			Amount: ing.Amount,
			Unit:   ing.Unit,
		}
	}

	aiResponse := &outbound.AIRecipeResponse{
		Title:        recipeResp.Title,
		Description:  recipeResp.Description,
		Ingredients:  aiIngredients,
		Instructions: recipeResp.Instructions,
		Tags:         recipeResp.Tags,
		Confidence:   0.8, // Default confidence for OpenAI responses
	}

	if recipeResp.Nutrition != nil {
		aiResponse.Nutrition = &outbound.NutritionInfo{
			Calories: recipeResp.Nutrition.Calories,
			Protein:  recipeResp.Nutrition.Protein,
			Carbs:    recipeResp.Nutrition.Carbs,
			Fat:      recipeResp.Nutrition.Fat,
			Fiber:    recipeResp.Nutrition.Fiber,
		}
	}

	return aiResponse, nil
}

// generateMockRecipe creates a fallback recipe when OpenAI is unavailable
func (c *Client) generateMockRecipe(prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	c.logger.Info("Generating mock recipe as fallback", zap.String("prompt", prompt))

	// Simple pattern matching for basic recipe generation
	title := "Delicious " + extractMainDish(prompt)
	if constraints.Cuisine != "" {
		title = strings.Title(constraints.Cuisine) + " " + title
	}

	description := fmt.Sprintf("A wonderful recipe inspired by your request: %s", prompt)
	if len(constraints.Dietary) > 0 {
		description += fmt.Sprintf(" This recipe is %s-friendly.", strings.Join(constraints.Dietary, " and "))
	}

	// Basic ingredients based on the prompt
	ingredients := generateMockIngredients(prompt, constraints)
	instructions := generateMockInstructions(prompt, constraints)
	tags := generateMockTags(prompt, constraints)

	return &outbound.AIRecipeResponse{
		Title:        title,
		Description:  description,
		Ingredients:  ingredients,
		Instructions: instructions,
		Tags:         tags,
		Confidence:   0.6, // Lower confidence for mock recipes
		Nutrition: &outbound.NutritionInfo{
			Calories: 350,
			Protein:  20.0,
			Carbs:    45.0,
			Fat:      12.0,
			Fiber:    5.0,
		},
	}, nil
}

// Helper functions for mock recipe generation
func extractMainDish(prompt string) string {
	prompt = strings.ToLower(prompt)
	dishes := map[string]string{
		"pasta":     "Pasta",
		"chicken":   "Chicken Dish",
		"beef":      "Beef Dish", 
		"fish":      "Fish Dish",
		"vegetable": "Vegetable Dish",
		"salad":     "Salad",
		"soup":      "Soup",
		"stir fry":  "Stir Fry",
		"curry":     "Curry",
		"pizza":     "Pizza",
		"sandwich":  "Sandwich",
		"burger":    "Burger",
		"taco":      "Tacos",
	}

	for keyword, dish := range dishes {
		if strings.Contains(prompt, keyword) {
			return dish
		}
	}
	return "Dish"
}

func generateMockIngredients(prompt string, constraints outbound.AIConstraints) []outbound.AIIngredient {
	// Basic ingredient set based on common patterns
	baseIngredients := []outbound.AIIngredient{
		{Name: "olive oil", Amount: 2, Unit: "tbsp"},
		{Name: "salt", Amount: 1, Unit: "tsp"},
		{Name: "black pepper", Amount: 0.5, Unit: "tsp"},
	}

	// Add specific ingredients based on prompt keywords
	prompt = strings.ToLower(prompt)
	if strings.Contains(prompt, "chicken") {
		baseIngredients = append(baseIngredients, outbound.AIIngredient{Name: "chicken breast", Amount: 1, Unit: "lb"})
	}
	if strings.Contains(prompt, "pasta") {
		baseIngredients = append(baseIngredients, outbound.AIIngredient{Name: "pasta", Amount: 8, Unit: "oz"})
	}
	if strings.Contains(prompt, "tomato") {
		baseIngredients = append(baseIngredients, outbound.AIIngredient{Name: "diced tomatoes", Amount: 1, Unit: "can"})
	}
	if strings.Contains(prompt, "onion") {
		baseIngredients = append(baseIngredients, outbound.AIIngredient{Name: "yellow onion", Amount: 1, Unit: "medium"})
	}
	if strings.Contains(prompt, "garlic") {
		baseIngredients = append(baseIngredients, outbound.AIIngredient{Name: "garlic", Amount: 3, Unit: "cloves"})
	}

	// Add default ingredients if list is too short
	if len(baseIngredients) < 5 {
		defaultIngredients := []outbound.AIIngredient{
			{Name: "fresh herbs", Amount: 2, Unit: "tbsp"},
			{Name: "butter", Amount: 2, Unit: "tbsp"},
		}
		baseIngredients = append(baseIngredients, defaultIngredients...)
	}

	return baseIngredients
}

func generateMockInstructions(prompt string, constraints outbound.AIConstraints) []string {
	return []string{
		"Heat olive oil in a large pan over medium heat.",
		"Add main ingredients and cook according to recipe requirements.",
		"Season with salt and pepper to taste.",
		"Continue cooking until ingredients are properly prepared.",
		"Serve hot and enjoy!",
	}
}

func generateMockTags(prompt string, constraints outbound.AIConstraints) []string {
	tags := []string{"ai-generated", "easy"}
	
	if constraints.Cuisine != "" {
		tags = append(tags, constraints.Cuisine)
	}
	
	for _, dietary := range constraints.Dietary {
		tags = append(tags, dietary)
	}
	
	return tags
}

// Other AIService interface methods with basic implementations
func (c *Client) SuggestIngredients(ctx context.Context, partial []string) ([]string, error) {
	// Simple suggestions based on common ingredient pairings
	suggestions := []string{
		"onions", "garlic", "tomatoes", "herbs", "spices",
		"olive oil", "butter", "salt", "pepper", "lemon",
	}
	return suggestions, nil
}

func (c *Client) AnalyzeNutrition(ctx context.Context, ingredients []string) (*outbound.NutritionInfo, error) {
	// Mock nutrition analysis
	return &outbound.NutritionInfo{
		Calories: 350,
		Protein:  20.0,
		Carbs:    45.0,
		Fat:      12.0,
		Fiber:    5.0,
		Sugar:    8.0,
		Sodium:   600.0,
	}, nil
}

func (c *Client) GenerateDescription(ctx context.Context, recipe *recipe.Recipe) (string, error) {
	return fmt.Sprintf("A delicious %s recipe that's perfect for any occasion.", recipe.Title()), nil
}

func (c *Client) ClassifyRecipe(ctx context.Context, recipe *recipe.Recipe) (*outbound.RecipeClassification, error) {
	return &outbound.RecipeClassification{
		Cuisine:    "fusion",
		Category:   "main-dish",
		Difficulty: "medium",
		Dietary:    []string{},
		Confidence: 0.7,
	}, nil
}