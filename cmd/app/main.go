// Package main provides the complete Alchemorsel v3 application with authentication
package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Email        string    `json:"email" gorm:"uniqueIndex"`
	Name         string    `json:"name" gorm:"column:full_name"`
	PasswordHash string    `json:"-" gorm:"column:password_hash"`
	Role         string    `json:"role" gorm:"default:'user'"`
	IsActive     bool      `json:"is_active" gorm:"column:is_active;default:true"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Recipe represents a recipe in the system
type Recipe struct {
	ID              uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	AuthorID        uuid.UUID `json:"author_id" gorm:"type:uuid"`
	Author          User      `json:"author" gorm:"foreignKey:AuthorID"`
	Cuisine         string    `json:"cuisine"`
	Difficulty      string    `json:"difficulty"`
	PrepTimeMinutes int       `json:"prep_time_minutes" gorm:"column:prep_time_minutes"`
	CookTimeMinutes int       `json:"cook_time_minutes" gorm:"column:cook_time_minutes"`
	Servings        int       `json:"servings"`
	LikesCount      int       `json:"likes_count" gorm:"column:likes_count;default:0"`
	ViewsCount      int       `json:"views_count" gorm:"column:views_count;default:0"`
	AverageRating   float64   `json:"average_rating" gorm:"column:average_rating;default:0.0"`
	Status          string    `json:"status" gorm:"default:'published'"`
	AIGenerated     bool      `json:"ai_generated" gorm:"column:ai_generated;default:false"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Ingredient represents a recipe ingredient
type Ingredient struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RecipeID   uuid.UUID `json:"recipe_id" gorm:"type:uuid"`
	Name       string    `json:"name"`
	Amount     float64   `json:"amount"`
	Unit       string    `json:"unit"`
	Optional   bool      `json:"optional" gorm:"default:false"`
	Notes      string    `json:"notes"`
	OrderIndex int       `json:"order_index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Instruction represents a recipe instruction step
type Instruction struct {
	ID               uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RecipeID         uuid.UUID `json:"recipe_id" gorm:"type:uuid"`
	StepNumber       int       `json:"step_number"`
	Description      string    `json:"description"`
	DurationMinutes  int       `json:"duration_minutes"`
	TemperatureValue float64   `json:"temperature_value"`
	TemperatureUnit  string    `json:"temperature_unit"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RecipeTag represents a recipe tag
type RecipeTag struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	RecipeID  uuid.UUID `json:"recipe_id" gorm:"type:uuid"`
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"created_at"`
}

// Session represents a user session
type Session struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid"`
	User      User      `json:"user" gorm:"foreignKey:UserID"`
	Token     string    `json:"token" gorm:"uniqueIndex"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// RecipeIngredient represents a single ingredient
type RecipeIngredient struct {
	Name   string `json:"name"`
	Amount string `json:"amount"`
	Unit   string `json:"unit,omitempty"`
}

// RecipeInstruction represents a single instruction step
type RecipeInstruction struct {
	Step int    `json:"step"`
	Text string `json:"text"`
}

// AIRecipeRequest represents the parsed intent from user message
type AIRecipeRequest struct {
	Intent      string   `json:"intent"`
	MainDish    string   `json:"main_dish"`
	Ingredients []string `json:"ingredients"`
	Cuisine     string   `json:"cuisine"`
	Difficulty  string   `json:"difficulty"`
	DietaryReqs []string `json:"dietary_requirements"`
}

var (
	db        *gorm.DB
	templates *template.Template
	jwtSecret = []byte("your-secret-key-change-in-production")
	
	// Recipe creation patterns for intent detection
	recipeIntentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(create|make|generate|cook|recipe for|how to make)\b.*\b(recipe|dish|food)\b`),
		regexp.MustCompile(`(?i)\bi want to (make|cook|create|prepare)\b`),
		regexp.MustCompile(`(?i)\brecipe (for|with|using)\b`),
		regexp.MustCompile(`(?i)\b(show me|give me|suggest) (a|some)? ?recipe\b`),
	}
	
	// Common cuisines for classification
	cuisineKeywords = map[string][]string{
		"italian":   {"pasta", "pizza", "spaghetti", "lasagna", "carbonara", "bolognese", "risotto", "italian"},
		"asian":     {"stir fry", "fried rice", "noodles", "soy sauce", "ginger", "asian", "chinese", "japanese"},
		"mexican":   {"tacos", "burritos", "salsa", "guacamole", "mexican", "spanish", "peppers", "beans"},
		"american":  {"burger", "bbq", "steak", "american", "sandwich", "fries"},
		"indian":    {"curry", "indian", "spices", "turmeric", "cumin", "tikka", "naan", "rice"},
		"french":    {"french", "butter", "wine", "herbs", "croissant", "baguette", "cheese"},
		"fusion":    {"fusion", "mix", "combination", "blend"},
	}
	
	// Sample recipe templates for AI generation
	recipeTemplates = map[string]map[string]interface{}{
		"pasta": {
			"base_ingredients": []string{"pasta", "olive oil", "garlic", "onion"},
			"cooking_methods":  []string{"boil", "sauté", "simmer", "toss"},
			"prep_time":        15,
			"cook_time":        20,
		},
		"stir_fry": {
			"base_ingredients": []string{"vegetables", "oil", "soy sauce", "garlic", "ginger"},
			"cooking_methods":  []string{"heat oil", "stir fry", "season", "serve over rice"},
			"prep_time":        10,
			"cook_time":        15,
		},
		"soup": {
			"base_ingredients": []string{"broth", "vegetables", "herbs", "seasoning"},
			"cooking_methods":  []string{"sauté vegetables", "add broth", "simmer", "season to taste"},
			"prep_time":        15,
			"cook_time":        30,
		},
		"salad": {
			"base_ingredients": []string{"mixed greens", "dressing", "vegetables"},
			"cooking_methods":  []string{"wash greens", "chop vegetables", "mix", "dress"},
			"prep_time":        10,
			"cook_time":        0,
		},
	}
)

// AI Recipe Generation Helper Functions

// parseRecipeIntent analyzes user message to detect recipe creation intent
func parseRecipeIntent(message string) (*AIRecipeRequest, bool) {
	message = strings.ToLower(strings.TrimSpace(message))
	
	// Check if message contains recipe creation intent
	isRecipeRequest := false
	for _, pattern := range recipeIntentPatterns {
		if pattern.MatchString(message) {
			isRecipeRequest = true
			break
		}
	}
	
	if !isRecipeRequest {
		return nil, false
	}
	
	request := &AIRecipeRequest{
		Intent:      "create_recipe",
		Difficulty:  "medium", // default
		Ingredients: []string{},
		DietaryReqs: []string{},
	}
	
	// Extract main dish/food type
	request.MainDish = extractMainDish(message)
	
	// Extract ingredients mentioned
	request.Ingredients = extractIngredients(message)
	
	// Classify cuisine
	request.Cuisine = classifyCuisine(message)
	
	// Extract dietary requirements
	request.DietaryReqs = extractDietaryRequirements(message)
	
	// Determine difficulty based on complexity
	if len(request.Ingredients) > 8 || strings.Contains(message, "complex") || strings.Contains(message, "advanced") {
		request.Difficulty = "hard"
	} else if len(request.Ingredients) < 4 || strings.Contains(message, "simple") || strings.Contains(message, "easy") || strings.Contains(message, "quick") {
		request.Difficulty = "easy"
	}
	
	return request, true
}

// extractMainDish tries to identify the main dish from the message
func extractMainDish(message string) string {
	dishPatterns := map[string]*regexp.Regexp{
		"pasta":     regexp.MustCompile(`\b(pasta|spaghetti|linguine|fettuccine|penne|rigatoni|carbonara|bolognese)\b`),
		"pizza":     regexp.MustCompile(`\bpizza\b`),
		"stir fry":  regexp.MustCompile(`\bstir.?fry\b`),
		"soup":      regexp.MustCompile(`\bsoup\b`),
		"salad":     regexp.MustCompile(`\bsalad\b`),
		"tacos":     regexp.MustCompile(`\btacos?\b`),
		"burger":    regexp.MustCompile(`\bburgers?\b`),
		"sandwich":  regexp.MustCompile(`\bsandwichs?\b`),
		"curry":     regexp.MustCompile(`\bcurry\b`),
		"rice":      regexp.MustCompile(`\bfried rice|rice bowl\b`),
		"chicken":   regexp.MustCompile(`\bchicken\b`),
		"beef":      regexp.MustCompile(`\bbeef|steak\b`),
		"fish":      regexp.MustCompile(`\bfish|salmon|tuna\b`),
		"vegetables": regexp.MustCompile(`\bvegetarian|veggies|vegetables\b`),
	}
	
	for dish, pattern := range dishPatterns {
		if pattern.MatchString(message) {
			return dish
		}
	}
	
	return "dish" // default
}

// extractIngredients identifies ingredients mentioned in the message
func extractIngredients(message string) []string {
	ingredientPatterns := map[string]*regexp.Regexp{
		"chicken":    regexp.MustCompile(`\bchicken\b`),
		"beef":       regexp.MustCompile(`\bbeef\b`),
		"pork":       regexp.MustCompile(`\bpork\b`),
		"fish":       regexp.MustCompile(`\bfish|salmon|tuna|cod\b`),
		"pasta":      regexp.MustCompile(`\bpasta|noodles\b`),
		"rice":       regexp.MustCompile(`\brice\b`),
		"tomatoes":   regexp.MustCompile(`\btomato(es)?\b`),
		"onions":     regexp.MustCompile(`\bonions?\b`),
		"garlic":     regexp.MustCompile(`\bgarlic\b`),
		"mushrooms":  regexp.MustCompile(`\bmushrooms?\b`),
		"peppers":    regexp.MustCompile(`\bpeppers?|bell peppers?\b`),
		"cheese":     regexp.MustCompile(`\bcheese\b`),
		"eggs":       regexp.MustCompile(`\beggs?\b`),
		"spinach":    regexp.MustCompile(`\bspinach\b`),
		"broccoli":   regexp.MustCompile(`\bbroccoli\b`),
		"carrots":    regexp.MustCompile(`\bcarrots?\b`),
		"potatoes":   regexp.MustCompile(`\bpotato(es)?\b`),
		"beans":      regexp.MustCompile(`\bbeans?\b`),
		"herbs":      regexp.MustCompile(`\bherbs?|basil|oregano|thyme|parsley\b`),
		"spices":     regexp.MustCompile(`\bspices?|cumin|paprika|turmeric\b`),
	}
	
	var ingredients []string
	for ingredient, pattern := range ingredientPatterns {
		if pattern.MatchString(message) {
			ingredients = append(ingredients, ingredient)
		}
	}
	
	return ingredients
}

// classifyCuisine determines the cuisine type based on keywords
func classifyCuisine(message string) string {
	for cuisine, keywords := range cuisineKeywords {
		for _, keyword := range keywords {
			if strings.Contains(message, keyword) {
				return cuisine
			}
		}
	}
	return "fusion" // default to fusion if can't determine
}

// extractDietaryRequirements identifies dietary restrictions
func extractDietaryRequirements(message string) []string {
	dietaryPatterns := map[string]*regexp.Regexp{
		"vegetarian": regexp.MustCompile(`\bvegetarian|veggie\b`),
		"vegan":      regexp.MustCompile(`\bvegan\b`),
		"gluten-free": regexp.MustCompile(`\bgluten.?free|no gluten\b`),
		"dairy-free": regexp.MustCompile(`\bdairy.?free|no dairy|lactose.?free\b`),
		"low-carb":   regexp.MustCompile(`\blow.?carb|keto\b`),
		"healthy":    regexp.MustCompile(`\bhealthy|nutritious|light\b`),
	}
	
	var requirements []string
	for req, pattern := range dietaryPatterns {
		if pattern.MatchString(message) {
			requirements = append(requirements, req)
		}
	}
	
	return requirements
}

// generateRecipe creates a structured recipe based on the AI request
func generateRecipe(request *AIRecipeRequest, userID uuid.UUID) (*Recipe, error) {
	if request == nil {
		return nil, fmt.Errorf("invalid recipe request")
	}
	
	// Generate recipe title
	title := generateRecipeTitle(request)
	
	// Generate description
	description := generateRecipeDescription(request)
	
	// Note: ingredients, instructions, and tags will be generated when saving the recipe
	
	// Get template info for timing
	templateKey := getTemplateKey(request.MainDish)
	template := recipeTemplates[templateKey]
	
	prepTime := 20  // default
	cookTime := 25  // default
	
	if template != nil {
		if pt, ok := template["prep_time"].(int); ok {
			prepTime = pt
		}
		if ct, ok := template["cook_time"].(int); ok {
			cookTime = ct
		}
	}
	
	// Adjust timing based on difficulty
	switch request.Difficulty {
	case "easy":
		prepTime = max(prepTime-5, 5)
		cookTime = max(cookTime-5, 0)
	case "hard":
		prepTime += 15
		cookTime += 10
	}
	
	recipe := &Recipe{
		Title:           title,
		Description:     description,
		AuthorID:        userID,
		Cuisine:         request.Cuisine,
		Difficulty:      request.Difficulty,
		PrepTimeMinutes: prepTime,
		CookTimeMinutes: cookTime,
		Servings:        4, // default servings
		LikesCount:      0,
		ViewsCount:      0,
		AverageRating:   0.0,
		Status:          "published",
		AIGenerated:     true,
	}
	
	return recipe, nil
}

// generateRecipeTitle creates an appropriate title
func generateRecipeTitle(request *AIRecipeRequest) string {
	adjectives := []string{"Delicious", "Amazing", "Perfect", "Classic", "Authentic", "Homestyle", "Gourmet", "Simple"}
	
	var titleParts []string
	
	// Add dietary requirements
	for _, req := range request.DietaryReqs {
		if req == "vegetarian" || req == "vegan" || req == "gluten-free" {
			titleParts = append(titleParts, strings.Title(req))
		}
	}
	
	// Add cuisine if specific
	if request.Cuisine != "fusion" && request.Cuisine != "" {
		titleParts = append(titleParts, strings.Title(request.Cuisine))
	}
	
	// Add main dish
	mainDish := strings.Title(request.MainDish)
	if len(request.Ingredients) > 0 {
		// Include primary ingredient in title
		primaryIngredient := request.Ingredients[0]
		if primaryIngredient != request.MainDish {
			mainDish = fmt.Sprintf("%s %s", strings.Title(primaryIngredient), mainDish)
		}
	}
	
	titleParts = append(titleParts, mainDish)
	
	// Add adjective sometimes
	if rand.Float32() < 0.7 {
		adjective := adjectives[rand.Intn(len(adjectives))]
		titleParts = []string{adjective, strings.Join(titleParts, " ")}
	}
	
	return strings.Join(titleParts, " ")
}

// generateRecipeDescription creates a description
func generateRecipeDescription(request *AIRecipeRequest) string {
	descriptions := []string{
		"A delightful %s recipe that brings together the perfect combination of flavors.",
		"This %s dish is perfect for any occasion and sure to impress your guests.",
		"A hearty and satisfying %s recipe that's both delicious and easy to make.",
		"Experience the authentic taste of %s cuisine with this amazing recipe.",
		"A modern twist on the classic %s that's packed with flavor and nutrition.",
	}
	
	template := descriptions[rand.Intn(len(descriptions))]
	cuisineDesc := request.Cuisine
	if cuisineDesc == "fusion" {
		cuisineDesc = "fusion"
	}
	
	description := fmt.Sprintf(template, cuisineDesc)
	
	// Add ingredient highlights
	if len(request.Ingredients) > 0 {
		ingredientList := strings.Join(request.Ingredients[:min(3, len(request.Ingredients))], ", ")
		description += fmt.Sprintf(" Featuring %s and other fresh ingredients.", ingredientList)
	}
	
	// Add dietary info
	if len(request.DietaryReqs) > 0 {
		dietInfo := strings.Join(request.DietaryReqs, " and ")
		description += fmt.Sprintf(" This recipe is %s-friendly.", dietInfo)
	}
	
	return description
}

// generateIngredientsList creates a realistic ingredients list
func generateIngredientsList(request *AIRecipeRequest) []RecipeIngredient {
	var ingredients []RecipeIngredient
	
	// Get base ingredients from template
	templateKey := getTemplateKey(request.MainDish)
	if template, exists := recipeTemplates[templateKey]; exists {
		if baseIngredients, ok := template["base_ingredients"].([]string); ok {
			for _, ingredient := range baseIngredients {
				ingredients = append(ingredients, RecipeIngredient{
					Name:   ingredient,
					Amount: getRandomAmount(ingredient),
					Unit:   getUnit(ingredient),
				})
			}
		}
	}
	
	// Add user-specified ingredients
	for _, userIngredient := range request.Ingredients {
		// Check if already added from base
		exists := false
		for _, existing := range ingredients {
			if strings.Contains(strings.ToLower(existing.Name), strings.ToLower(userIngredient)) ||
				strings.Contains(strings.ToLower(userIngredient), strings.ToLower(existing.Name)) {
				exists = true
				break
			}
		}
		
		if !exists {
			ingredients = append(ingredients, RecipeIngredient{
				Name:   userIngredient,
				Amount: getRandomAmount(userIngredient),
				Unit:   getUnit(userIngredient),
			})
		}
	}
	
	// Add common seasonings if not present
	seasonings := []string{"salt", "black pepper", "olive oil"}
	for _, seasoning := range seasonings {
		exists := false
		for _, existing := range ingredients {
			if strings.Contains(strings.ToLower(existing.Name), seasoning) {
				exists = true
				break
			}
		}
		if !exists {
			ingredients = append(ingredients, RecipeIngredient{
				Name:   seasoning,
				Amount: getRandomAmount(seasoning),
				Unit:   getUnit(seasoning),
			})
		}
	}
	
	return ingredients
}

// generateInstructions creates cooking instructions
func generateInstructions(request *AIRecipeRequest) []RecipeInstruction {
	var instructions []RecipeInstruction
	step := 1
	
	// Get cooking methods from template
	templateKey := getTemplateKey(request.MainDish)
	if template, exists := recipeTemplates[templateKey]; exists {
		if methods, ok := template["cooking_methods"].([]string); ok {
			for _, method := range methods {
				instructions = append(instructions, RecipeInstruction{
					Step: step,
					Text: strings.Title(method) + ".",
				})
				step++
			}
		}
	}
	
	// Add generic final steps if needed
	if len(instructions) < 3 {
		genericSteps := []string{
			"Prepare all ingredients and have them ready",
			"Cook according to recipe specifications",
			"Season to taste and serve hot",
		}
		
		for i, genericStep := range genericSteps {
			if step-1+i >= len(instructions) {
				instructions = append(instructions, RecipeInstruction{
					Step: step + i,
					Text: genericStep + ".",
				})
			}
		}
	}
	
	return instructions
}

// generateTags creates relevant tags
func generateTags(request *AIRecipeRequest) []string {
	tags := []string{"ai-generated"}
	
	// Add cuisine tag
	if request.Cuisine != "" {
		tags = append(tags, request.Cuisine)
	}
	
	// Add difficulty tag
	tags = append(tags, request.Difficulty)
	
	// Add dietary requirement tags
	tags = append(tags, request.DietaryReqs...)
	
	// Add main dish tag
	if request.MainDish != "" && request.MainDish != "dish" {
		tags = append(tags, request.MainDish)
	}
	
	// Add ingredient tags for primary ingredients
	for i, ingredient := range request.Ingredients {
		if i < 3 { // Only add first 3 to avoid too many tags
			tags = append(tags, ingredient)
		}
	}
	
	return tags
}

// Helper functions
func getTemplateKey(mainDish string) string {
	dishMapping := map[string]string{
		"pasta":      "pasta",
		"spaghetti":  "pasta", 
		"linguine":   "pasta",
		"stir fry":   "stir_fry",
		"soup":       "soup",
		"salad":      "salad",
	}
	
	if key, exists := dishMapping[mainDish]; exists {
		return key
	}
	
	// Default based on ingredients or type
	if strings.Contains(mainDish, "pasta") || strings.Contains(mainDish, "noodle") {
		return "pasta"
	}
	if strings.Contains(mainDish, "soup") || strings.Contains(mainDish, "broth") {
		return "soup"
	}
	if strings.Contains(mainDish, "salad") || strings.Contains(mainDish, "greens") {
		return "salad"
	}
	
	return "stir_fry" // default
}

func getRandomAmount(ingredient string) string {
	// Predefined amounts for common ingredients
	amounts := map[string][]string{
		"pasta":       {"300g", "400g", "1 lb"},
		"rice":        {"1 cup", "1.5 cups", "2 cups"},
		"chicken":     {"2 lbs", "1 lb", "3 lbs"},
		"beef":        {"1 lb", "2 lbs", "1.5 lbs"},
		"onions":      {"1 medium", "2 medium", "1 large"},
		"garlic":      {"3 cloves", "4 cloves", "2 cloves"},
		"tomatoes":    {"2 medium", "3 medium", "1 can"},
		"oil":         {"2 tbsp", "3 tbsp", "1/4 cup"},
		"olive oil":   {"2 tbsp", "3 tbsp", "1/4 cup"},
		"salt":        {"to taste", "1 tsp", "1/2 tsp"},
		"pepper":      {"to taste", "1/2 tsp", "1/4 tsp"},
		"black pepper": {"to taste", "1/2 tsp", "1/4 tsp"},
		"cheese":      {"1/2 cup", "1 cup", "1/4 cup"},
		"mushrooms":   {"8 oz", "1 lb", "1/2 lb"},
		"herbs":       {"1 tsp", "2 tsp", "1 tbsp"},
		"spices":      {"1 tsp", "1/2 tsp", "2 tsp"},
	}
	
	// Check for exact matches or partial matches
	for key, amountList := range amounts {
		if strings.Contains(strings.ToLower(ingredient), key) {
			return amountList[rand.Intn(len(amountList))]
		}
	}
	
	// Default amounts based on ingredient type
	defaultAmounts := []string{"1 cup", "2 cups", "1/2 cup", "1 tbsp", "2 tbsp", "1 tsp", "2 medium", "1 large"}
	return defaultAmounts[rand.Intn(len(defaultAmounts))]
}

func getUnit(ingredient string) string {
	// Most amounts already include units, so return empty for now
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	// Initialize random seed for recipe generation
	rand.Seed(time.Now().UnixNano())
	
	fmt.Println(`
 █████╗ ██╗      ██████╗██╗  ██╗███████╗███╗   ███╗ ██████╗ ██████╗ ███████╗███████╗██╗     
██╔══██╗██║     ██╔════╝██║  ██║██╔════╝████╗ ████║██╔═══██╗██╔══██╗██╔════╝██╔════╝██║     
███████║██║     ██║     ███████║█████╗  ██╔████╔██║██║   ██║██████╔╝███████╗█████╗  ██║     
██╔══██║██║     ██║     ██╔══██║██╔══╝  ██║╚██╔╝██║██║   ██║██╔══██╗╚════██║██╔══╝  ██║     
██║  ██║███████╗╚██████╗██║  ██║███████╗██║ ╚═╝ ██║╚██████╔╝██║  ██║███████║███████╗███████╗
╚═╝  ╚═╝╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝
                                      v3.0.0 - Enterprise Recipe Platform                                      
	`)

	// Initialize database
	initDatabase()

	// Initialize templates
	initTemplates()

	// Setup router
	r := setupRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Alchemorsel v3 server starting on http://localhost:%s\n", port)
	fmt.Println("✅ Features: PostgreSQL Database, Real Authentication, Protected Routes")
	fmt.Println("👤 Demo accounts: chef@alchemorsel.com / user@alchemorsel.com (password: password)")
	fmt.Println("🔒 Protected routes: /dashboard, /recipes/new require authentication")
	fmt.Println("🐘 Database: PostgreSQL (start with: docker-compose -f docker-compose.dev.yml up -d)")

	log.Fatal(http.ListenAndServe(":"+port, r))
}

func initDatabase() {
	// Get database URL from environment or use default
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://alchemorsel:alchemorsel_dev_password@localhost:5434/alchemorsel_dev?sslmode=disable"
	}

	var err error
	db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Printf("⚠️  PostgreSQL not available, trying to start containers...")
		
		// Try to start PostgreSQL with Docker Compose
		fmt.Println("🐳 Starting PostgreSQL with Docker Compose...")
		os.Chdir("/home/hermes/alchemorsel-v3")
		if err := startPostgreSQL(); err != nil {
			log.Fatal("❌ Failed to start PostgreSQL:", err)
		}
		
		// Wait a bit for PostgreSQL to start
		fmt.Println("⏳ Waiting for PostgreSQL to be ready...")
		time.Sleep(10 * time.Second)
		
		// Try to connect again
		db, err = gorm.Open(postgres.Open(dbURL), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			log.Fatal("❌ Failed to connect to PostgreSQL:", err)
		}
	}

	// Auto migrate with error handling for constraint conflicts
	err = safeAutoMigrate(db)
	if err != nil {
		log.Fatal("❌ Failed to migrate database:", err)
	}

	// Seed demo data
	seedDatabase()
	
	fmt.Println("✅ Database connected and migrated successfully")
}

func safeAutoMigrate(db *gorm.DB) error {
	// Create tables if they don't exist first
	if !db.Migrator().HasTable(&User{}) {
		if err := db.Migrator().CreateTable(&User{}); err != nil {
			return fmt.Errorf("failed to create users table: %w", err)
		}
	}
	
	if !db.Migrator().HasTable(&Recipe{}) {
		if err := db.Migrator().CreateTable(&Recipe{}); err != nil {
			return fmt.Errorf("failed to create recipes table: %w", err)
		}
	}
	
	if !db.Migrator().HasTable(&Session{}) {
		if err := db.Migrator().CreateTable(&Session{}); err != nil {
			return fmt.Errorf("failed to create sessions table: %w", err)
		}
	}

	// Now run AutoMigrate to handle any schema changes
	// This might fail on constraint operations, so we'll handle it gracefully
	err := db.AutoMigrate(&User{}, &Recipe{}, &Session{}, &Ingredient{}, &Instruction{}, &RecipeTag{})
	if err != nil {
		// Log the error but don't fail if it's a constraint issue
		log.Printf("⚠️  Auto-migration warning (continuing anyway): %v", err)
		// Check if tables exist and have basic structure
		if !db.Migrator().HasTable(&User{}) || !db.Migrator().HasTable(&Recipe{}) || !db.Migrator().HasTable(&Session{}) {
			return fmt.Errorf("tables don't exist and migration failed: %w", err)
		}
	}
	
	return nil
}

func startPostgreSQL() error {
	// This would start PostgreSQL using Docker Compose
	// For now, we'll just show the command
	fmt.Println("💡 Please run: docker-compose -f docker-compose.dev.yml up -d")
	fmt.Println("   Then restart the application")
	return fmt.Errorf("PostgreSQL containers need to be started manually")
}

func seedDatabase() {
	// Check if data already exists
	var userCount int64
	db.Model(&User{}).Count(&userCount)
	if userCount > 0 {
		return // Already seeded
	}

	// Create demo users
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	
	users := []User{
		{
			Email:        "chef@alchemorsel.com",
			Name:         "Chef Demo",
			PasswordHash: string(hashedPassword),
			Role:         "chef",
			IsActive:     true,
		},
		{
			Email:        "user@alchemorsel.com",
			Name:         "Home Cook",
			PasswordHash: string(hashedPassword),
			Role:         "user",
			IsActive:     true,
		},
	}

	for _, user := range users {
		db.Create(&user)
	}

	// Create demo recipes
	recipes := []Recipe{
		{
			Title:           "Classic Spaghetti Carbonara",
			Description:     "A traditional Italian pasta dish with eggs, cheese, pancetta, and pepper",
			AuthorID:        users[0].ID,
			Cuisine:         "italian",
			Difficulty:      "medium",
			PrepTimeMinutes: 10,
			CookTimeMinutes: 15,
			Servings:        4,
			LikesCount:      42,
			ViewsCount:      156,
			AverageRating:   4.8,
			Status:          "published",
			AIGenerated:     false,
		},
		{
			Title:           "AI-Generated Fusion Tacos",
			Description:     "Creative fusion tacos combining Korean and Mexican flavors",
			AuthorID:        users[0].ID,
			Cuisine:         "fusion",
			Difficulty:      "medium",
			PrepTimeMinutes: 20,
			CookTimeMinutes: 15,
			Servings:        4,
			LikesCount:      28,
			ViewsCount:      89,
			AverageRating:   4.3,
			Status:          "published",
			AIGenerated:     true,
		},
	}

	for _, recipe := range recipes {
		db.Create(&recipe)
	}

	fmt.Println("✅ Database seeded with demo data")
}

func initTemplates() {
	var err error
	templates, err = template.ParseGlob("internal/infrastructure/http/server/templates/**/*.html")
	if err != nil {
		templates, err = template.ParseGlob("internal/infrastructure/http/server/templates/*/*.html")
		if err != nil {
			log.Printf("Warning: Could not load templates: %v", err)
			templates = template.New("")
		}
	}
}

func setupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(corsMiddleware)

	// Add authentication context to all requests
	r.Use(authContextMiddleware)

	// Serve static files
	fileServer := http.FileServer(http.Dir("internal/infrastructure/http/server/static/"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Public routes
	r.Get("/", handleHome)
	r.Get("/login", redirectIfAuthenticated(handleLogin))
	r.Get("/register", redirectIfAuthenticated(handleRegister))
	r.Get("/recipes", handleRecipes)
	r.Get("/recipes/{id}", handleRecipeDetail)
	r.Post("/ai/chat", handleAIChat)

	// Authentication routes
	r.Post("/auth/login", handleAuthLogin)
	r.Post("/auth/register", handleAuthRegister)
	r.Post("/auth/logout", handleAuthLogout)

	// Protected routes - require authentication
	r.Group(func(r chi.Router) {
		r.Use(requireAuth)
		r.Get("/dashboard", handleDashboard)
		r.Get("/recipes/new", handleNewRecipe)
		r.Post("/recipes", handleCreateRecipe)
		r.Get("/profile", handleProfile)
	})

	// HTMX endpoints
	r.Route("/htmx", func(r chi.Router) {
		r.Post("/recipes/search", handleRecipeSearch)
		
		// Protected HTMX endpoints
		r.Group(func(r chi.Router) {
			r.Use(requireAuth)
			r.Post("/recipes/{id}/like", handleRecipeLike)
		})
	})

	return r
}

// Middleware

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		
		if r.Method == "OPTIONS" {
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func authContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get user from session token
		var user *User
		
		if cookie, err := r.Cookie("session_token"); err == nil {
			log.Printf("Found session cookie for %s %s (HTMX: %v)", r.Method, r.URL.Path, isHTMXRequest(r))
			if claims, err := validateJWT(cookie.Value); err == nil {
				if dbUser, err := getUserByID(claims.UserID); err == nil {
					user = dbUser
					log.Printf("Authenticated user: %s (%s) for %s %s", user.Name, user.Email, r.Method, r.URL.Path)
				} else {
					log.Printf("User not found in database for token claims: %v", err)
				}
			} else {
				log.Printf("JWT validation failed: %v", err)
			}
		} else {
			log.Printf("No session cookie found for %s %s (HTMX: %v)", r.Method, r.URL.Path, isHTMXRequest(r))
		}
		
		// Add user to context
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromContext(r.Context())
		if user == nil {
			if isHTMXRequest(r) {
				w.Header().Set("HX-Redirect", "/login")
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func redirectIfAuthenticated(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := getUserFromContext(r.Context())
		if user != nil {
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
		handler(w, r)
	}
}

// JWT Claims
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	jwt.StandardClaims
}

// Auth helpers

func setSessionCookie(w http.ResponseWriter, token string) {
	// Determine if we're in a secure environment
	secure := false // Set to true in production with HTTPS
	
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode, // Lax mode for HTMX compatibility
		MaxAge:   86400, // 24 hours - matches JWT expiration
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode, // Lax mode for HTMX compatibility
		MaxAge:   -1,
	})
}

func createJWT(user *User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
			IssuedAt:  time.Now().Unix(),
			NotBefore: time.Now().Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func validateJWT(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("empty token")
	}
	
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	
	if err != nil {
		log.Printf("JWT validation error: %v", err)
		return nil, err
	}
	
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	
	// Check if token is expired
	if claims.ExpiresAt < time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}
	
	return claims, nil
}

func getUserByID(id uuid.UUID) (*User, error) {
	var user User
	err := db.Where("id = ? AND is_active = ?", id, true).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func getUserByEmail(email string) (*User, error) {
	var user User
	err := db.Where("email = ? AND is_active = ?", email, true).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func getUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value("user").(*User); ok {
		return user
	}
	return nil
}

func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// Handlers

func handleHome(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	data := map[string]interface{}{
		"Title":       "Home - Alchemorsel v3",
		"Description": "AI-Powered Recipe Platform",
		"User":        user,
		"IsAuthenticated": user != nil,
	}
	renderTemplate(w, "home", data)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Login - Alchemorsel v3",
		"User":  nil,
		"IsAuthenticated": false,
	}
	renderTemplate(w, "login", data)
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Register - Alchemorsel v3",
		"User":  nil,
		"IsAuthenticated": false,
	}
	renderTemplate(w, "register", data)
}

func handleRecipes(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	// Get recipes from database
	var recipes []Recipe
	db.Preload("Author").Order("created_at DESC").Find(&recipes)
	
	data := map[string]interface{}{
		"Title":   "Recipes - Alchemorsel v3",
		"User":    user,
		"IsAuthenticated": user != nil,
		"Recipes": recipes,
	}
	renderTemplate(w, "recipes", data)
}

func handleRecipeDetail(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	recipeID := chi.URLParam(r, "id")
	
	var recipe Recipe
	err := db.Preload("Author").Where("id = ?", recipeID).First(&recipe).Error
	if err != nil {
		http.NotFound(w, r)
		return
	}
	
	// Increment view count
	db.Model(&recipe).Update("views_count", recipe.ViewsCount+1)
	
	data := map[string]interface{}{
		"Title":  recipe.Title + " - Alchemorsel v3",
		"User":   user,
		"IsAuthenticated": user != nil,
		"Recipe": recipe,
	}
	renderTemplate(w, "recipe-detail", data)
}

func handleNewRecipe(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	data := map[string]interface{}{
		"Title": "Create Recipe - Alchemorsel v3",
		"User":  user,
		"IsAuthenticated": true,
	}
	renderTemplate(w, "recipe-form", data)
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	// Get user's recipes
	var userRecipes []Recipe
	db.Where("author_id = ?", user.ID).Order("created_at DESC").Find(&userRecipes)
	
	// Get user stats
	var totalLikes int64
	db.Model(&Recipe{}).Where("author_id = ?", user.ID).Select("COALESCE(SUM(likes_count), 0)").Scan(&totalLikes)
	
	data := map[string]interface{}{
		"Title": "Dashboard - Alchemorsel v3",
		"User":  user,
		"IsAuthenticated": true,
		"UserRecipes": userRecipes,
		"Stats": map[string]interface{}{
			"RecipeCount": len(userRecipes),
			"TotalLikes":  totalLikes,
			"Followers":   0, // Would implement followers system
			"Following":   0, // Would implement following system
		},
	}
	renderTemplate(w, "dashboard", data)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	data := map[string]interface{}{
		"Title": "Profile - Alchemorsel v3",
		"User":  user,
		"IsAuthenticated": true,
	}
	renderTemplate(w, "profile", data)
}

// Authentication handlers

func handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	
	if email == "" || password == "" {
		renderError(w, "Email and password are required")
		return
	}
	
	// Get user from database
	user, err := getUserByEmail(email)
	if err != nil {
		renderError(w, "Invalid credentials")
		return
	}
	
	// Check password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		renderError(w, "Invalid credentials")
		return
	}
	
	// Create JWT token
	token, err := createJWT(user)
	if err != nil {
		renderError(w, "Login failed")
		return
	}
	
	// Set secure cookie with better HTMX compatibility
	setSessionCookie(w, token)
	
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/dashboard")
		return
	}
	
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")
	
	// Validation
	if name == "" || email == "" || password == "" {
		renderError(w, "All fields are required")
		return
	}
	
	if password != passwordConfirm {
		renderError(w, "Passwords do not match")
		return
	}
	
	if len(password) < 8 {
		renderError(w, "Password must be at least 8 characters")
		return
	}
	
	// Check if user already exists
	if _, err := getUserByEmail(email); err == nil {
		renderError(w, "User with this email already exists")
		return
	}
	
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		renderError(w, "Registration failed")
		return
	}
	
	// Create user
	user := User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hashedPassword),
		Role:         "user",
		IsActive:     true,
	}
	
	err = db.Create(&user).Error
	if err != nil {
		renderError(w, "Registration failed")
		return
	}
	
	// Create JWT token
	token, err := createJWT(&user)
	if err != nil {
		renderError(w, "Registration successful but login failed")
		return
	}
	
	// Set secure cookie with better HTMX compatibility
	setSessionCookie(w, token)
	
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/dashboard")
		return
	}
	
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	// Clear cookie
	clearSessionCookie(w)
	
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/")
		return
	}
	
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HTMX handlers

func handleAIChat(w http.ResponseWriter, r *http.Request) {
	message := r.FormValue("message")
	user := getUserFromContext(r.Context())
	
	if message == "" {
		renderHTMXError(w, "Message cannot be empty")
		return
	}
	
	// Start building HTML response with user message
	userMessageHTML := fmt.Sprintf(`
		<div class="chat-message user-message">
			<div class="message-content">%s</div>
			<div class="message-author">%s</div>
			<div class="message-timestamp">Just now</div>
		</div>`, message, getUserName(user))
	
	// Parse message for recipe creation intent
	recipeRequest, isRecipeRequest := parseRecipeIntent(message)
	
	var aiResponseHTML string
	
	if isRecipeRequest && user != nil {
		// Generate recipe using AI
		recipe, err := generateRecipe(recipeRequest, user.ID)
		if err != nil {
			log.Printf("Error generating recipe: %v", err)
			response := "🤖 AI Chef: I had trouble generating that recipe. Please try again with different ingredients or description."
			aiResponseHTML = fmt.Sprintf(`
				<div class="chat-message ai-message">
					<div class="message-content">%s</div>
					<div class="message-author">AI Chef</div>
					<div class="message-timestamp">Just now</div>
				</div>`, response)
		} else {
			// Save recipe to database
			err = db.Create(recipe).Error
			if err != nil {
				log.Printf("Error saving recipe to database: %v", err)
				response := "🤖 AI Chef: I created a great recipe for you, but couldn't save it right now. Please try again."
				aiResponseHTML = fmt.Sprintf(`
					<div class="chat-message ai-message">
						<div class="message-content">%s</div>
						<div class="message-author">AI Chef</div>
						<div class="message-timestamp">Just now</div>
					</div>`, response)
			} else {
				// Save ingredients, instructions, and tags
				ingredients := generateIngredientsList(recipeRequest)
				for i, ing := range ingredients {
					ingredient := Ingredient{
						RecipeID:   recipe.ID,
						Name:       ing.Name,
						Amount:     1.0, // simplified for now
						Unit:       ing.Unit,
						OrderIndex: i + 1,
					}
					db.Create(&ingredient)
				}
				
				instructions := generateInstructions(recipeRequest)
				for _, inst := range instructions {
					instruction := Instruction{
						RecipeID:    recipe.ID,
						StepNumber:  inst.Step,
						Description: inst.Text,
					}
					db.Create(&instruction)
				}
				
				tags := generateTags(recipeRequest)
				for _, tag := range tags {
					recipeTag := RecipeTag{
						RecipeID: recipe.ID,
						Tag:      tag,
					}
					db.Create(&recipeTag)
				}
				// Successfully created and saved recipe
				log.Printf("Successfully created AI recipe: %s (ID: %s)", recipe.Title, recipe.ID)
				
				// Create success response with link to recipe
				response := fmt.Sprintf(`🤖 AI Chef: Perfect! I've created "<strong>%s</strong>" for you! This %s %s recipe features %s and takes about %d minutes to prepare.
					<br><br>
					<div class="recipe-created-notification">
						<h4>✨ Recipe Created Successfully!</h4>
						<p><strong>%s</strong></p>
						<p>%s</p>
						<div class="recipe-quick-stats">
							<span class="badge">%s</span>
							<span class="badge">%s</span>
							<span class="badge ai-badge">AI Generated</span>
						</div>
						<div style="margin-top: 15px;">
							<a href="/recipes/%s" class="btn btn-primary">View Full Recipe</a>
							<a href="/dashboard" class="btn">Go to Dashboard</a>
						</div>
					</div>`,
					recipe.Title,
					recipe.Difficulty,
					recipe.Cuisine,
					getIngredientPreview(recipeRequest),
					recipe.PrepTimeMinutes+recipe.CookTimeMinutes,
					recipe.Title,
					recipe.Description,
					recipe.Cuisine,
					recipe.Difficulty,
					recipe.ID)
				
				aiResponseHTML = fmt.Sprintf(`
					<div class="chat-message ai-message">
						<div class="message-content">%s</div>
						<div class="message-author">AI Chef</div>
						<div class="message-timestamp">Just now</div>
					</div>`, response)
			}
		}
	} else if isRecipeRequest && user == nil {
		// User not logged in but wants to create recipe
		response := `🤖 AI Chef: I'd love to create a personalized recipe for you! However, you need to be logged in to save recipes. 
			<br><br>
			<div class="auth-prompt">
				<p><strong>Please log in to unlock AI recipe creation:</strong></p>
				<a href="/login" class="btn btn-primary">Login</a>
				<a href="/register" class="btn">Register</a>
			</div>`
		
		aiResponseHTML = fmt.Sprintf(`
			<div class="chat-message ai-message">
				<div class="message-content">%s</div>
				<div class="message-author">AI Chef</div>
				<div class="message-timestamp">Just now</div>
			</div>`, response)
	} else {
		// Not a recipe request, provide general cooking advice
		responses := []string{
			"🤖 AI Chef: That's an interesting cooking question! While I specialize in creating recipes, I'd suggest trying to phrase your request like 'Create a recipe for...' or 'I want to make...' to get personalized recipes.",
			"🤖 AI Chef: I'm here to help you create amazing recipes! Try asking me to 'make a pasta recipe with mushrooms' or 'create a vegetarian stir fry' and I'll generate a complete recipe for you.",
			"🤖 AI Chef: I understand you're looking for cooking help! For the best results, tell me what dish you'd like to make or what ingredients you want to use, and I'll create a custom recipe.",
			"🤖 AI Chef: Great question! I'm designed to create personalized recipes based on your preferences. Try saying something like 'generate a chicken curry recipe' or 'I want to cook with tomatoes and herbs'.",
		}
		
		response := responses[rand.Intn(len(responses))]
		
		// Add recipe examples
		response += `<br><br><strong>Try these examples:</strong>
			<ul>
				<li>"Create a pasta recipe with mushrooms"</li>
				<li>"I want to make chicken tacos"</li>
				<li>"Generate a vegetarian stir-fry recipe"</li>
				<li>"Make me a healthy salad with avocado"</li>
			</ul>`
		
		aiResponseHTML = fmt.Sprintf(`
			<div class="chat-message ai-message">
				<div class="message-content">%s</div>
				<div class="message-author">AI Chef</div>
				<div class="message-timestamp">Just now</div>
			</div>`, response)
	}
	
	// Combine user message and AI response
	fullHTML := userMessageHTML + aiResponseHTML
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(fullHTML))
}

// Helper function to get a preview of ingredients for display
func getIngredientPreview(recipeRequest *AIRecipeRequest) string {
	if len(recipeRequest.Ingredients) == 0 {
		return "fresh ingredients"
	}
	
	var names []string
	for i, ingredient := range recipeRequest.Ingredients {
		if i >= 3 { // Only show first 3
			break
		}
		names = append(names, ingredient)
	}
	
	if len(recipeRequest.Ingredients) > 3 {
		return strings.Join(names, ", ") + " and more"
	}
	
	return strings.Join(names, ", ")
}

func handleRecipeSearch(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	
	if query == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<div>Please enter a search term</div>"))
		return
	}
	
	// Search recipes in database
	var recipes []Recipe
	db.Preload("Author").Where("title ILIKE ? OR description ILIKE ?", "%"+query+"%", "%"+query+"%").Find(&recipes)
	
	if len(recipes) == 0 {
		html := fmt.Sprintf(`<div class="search-results">
			<h3>No results found for "%s"</h3>
			<p>Try searching for different keywords.</p>
		</div>`, query)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
		return
	}
	
	// Render search results
	html := fmt.Sprintf(`<div class="search-results">
		<h3>Search Results for "%s" (%d found)</h3>
		<div class="recipe-grid">`, query, len(recipes))
	
	for _, recipe := range recipes {
		aiBadge := ""
		if recipe.AIGenerated {
			aiBadge = `<span class="badge ai-badge">AI Generated</span>`
		}
		
		html += fmt.Sprintf(`
			<div class="recipe-card">
				<h4><a href="/recipes/%s">%s</a></h4>
				<p>%s</p>
				<div class="recipe-meta">
					<span class="badge">%s</span>
					<span class="badge">%s</span>
					%s
				</div>
				<div class="recipe-stats">
					<small>👤 %s | ❤️ %d likes | ⭐ %.1f/5</small>
				</div>
			</div>`,
			recipe.ID, recipe.Title, recipe.Description,
			recipe.Cuisine, recipe.Difficulty, aiBadge,
			recipe.Author.Name, recipe.LikesCount, recipe.AverageRating)
	}
	
	html += "</div></div>"
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func handleRecipeLike(w http.ResponseWriter, r *http.Request) {
	_ = getUserFromContext(r.Context()) // TODO: Track individual user likes
	recipeID := chi.URLParam(r, "id")
	
	// In a real app, you'd track individual likes per user
	// For now, just increment the like count
	var recipe Recipe
	err := db.Where("id = ?", recipeID).First(&recipe).Error
	if err != nil {
		renderHTMXError(w, "Recipe not found")
		return
	}
	
	// Increment likes
	recipe.LikesCount++
	db.Save(&recipe)
	
	// Return updated like button
	html := fmt.Sprintf(`
		<button class="btn btn-sm btn-primary" disabled>
			❤️ %d
		</button>
	`, recipe.LikesCount)
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func handleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	
	title := r.FormValue("title")
	description := r.FormValue("description")
	cuisine := r.FormValue("cuisine")
	difficulty := r.FormValue("difficulty")
	
	if title == "" || description == "" {
		renderError(w, "Title and description are required")
		return
	}
	
	recipe := Recipe{
		Title:           title,
		Description:     description,
		AuthorID:        user.ID,
		Cuisine:         cuisine,
		Difficulty:      difficulty,
		PrepTimeMinutes: 0,
		CookTimeMinutes: 0,
		Servings:        4,
		Status:          "published",
	}
	
	err := db.Create(&recipe).Error
	if err != nil {
		renderError(w, "Failed to create recipe")
		return
	}
	
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// Helper functions

func getUserName(user *User) string {
	if user != nil {
		return user.Name
	}
	return "Anonymous"
}

func renderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	if templates.Lookup(templateName) == nil {
		// Template not found, render a basic page with dynamic navigation
		user := data.(map[string]interface{})["User"]
		isAuth := data.(map[string]interface{})["IsAuthenticated"].(bool)
		
		navLinks := ""
		if isAuth {
			navLinks = `
				<a href="/dashboard" class="btn">Dashboard</a>
				<a href="/recipes" class="btn">Recipes</a>
				<a href="/recipes/new" class="btn">Create</a>
				<a href="/profile" class="btn">Profile</a>
				<form method="post" action="/auth/logout" style="display: inline;">
					<button type="submit" class="btn">Logout</button>
				</form>
			`
		} else {
			navLinks = `
				<a href="/recipes" class="btn">Recipes</a>
				<a href="/login" class="btn">Login</a>
				<a href="/register" class="btn">Register</a>
			`
		}
		
		html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>%s</title>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<script src="https://unpkg.com/htmx.org@1.9.6"></script>
	<style>
		body { font-family: system-ui; margin: 0; padding: 20px; background: #f5f5f5; }
		.container { max-width: 1200px; margin: 0 auto; }
		.header { background: #2d3748; color: white; padding: 1rem; margin: -20px -20px 20px; }
		.nav { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 10px; }
		.nav-links { display: flex; gap: 10px; flex-wrap: wrap; align-items: center; }
		.user-info { color: #a0aec0; font-size: 0.9em; margin-right: 15px; }
		.card { background: white; padding: 20px; margin: 20px 0; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
		.btn { background: #3182ce; color: white; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; margin: 2px; }
		.btn:hover { background: #2c5282; }
		.btn-danger { background: #e53e3e; }
		.btn-danger:hover { background: #c53030; }
		.form-group { margin: 15px 0; }
		.form-input { width: 100%%; padding: 10px; border: 1px solid #ddd; border-radius: 4px; box-sizing: border-box; }
		.recipe-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
		.recipe-card { border: 1px solid #eee; padding: 15px; border-radius: 8px; background: white; }
		.recipe-card h4 a { text-decoration: none; color: #2d3748; }
		.recipe-card h4 a:hover { color: #3182ce; }
		.badge { background: #e2e8f0; padding: 4px 8px; border-radius: 12px; font-size: 0.8em; margin: 2px; }
		.ai-badge { background: #9f7aea; color: white; }
		.chat-interface { background: #f8f9fa; border-radius: 8px; padding: 20px; margin: 20px 0; }
		.chat-message { background: white; padding: 15px; margin: 10px 0; border-radius: 8px; border-left: 4px solid #3182ce; }
		.ai-message { border-left-color: #9f7aea; }
		.user-message { border-left-color: #48bb78; }
		.message-author { font-weight: bold; font-size: 0.9em; color: #4a5568; }
		.message-timestamp { font-size: 0.8em; color: #718096; margin-top: 5px; }
		.error { background: #fed7d7; color: #9b2c2c; padding: 10px; border-radius: 4px; margin: 10px 0; }
		.success { background: #c6f6d5; color: #276749; padding: 10px; border-radius: 4px; margin: 10px 0; }
		.protected-notice { background: #bee3f8; color: #2c5282; padding: 10px; border-radius: 4px; margin: 10px 0; }
		.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; }
		.stat-card { background: #f7fafc; padding: 15px; border-radius: 8px; text-align: center; }
		.stat-number { font-size: 2em; font-weight: bold; color: #3182ce; }
		.stat-label { color: #718096; font-size: 0.9em; }
		.recipe-created-notification { background: #c6f6d5; border: 1px solid #9ae6b4; padding: 20px; border-radius: 8px; margin: 15px 0; }
		.recipe-created-notification h4 { margin: 0 0 10px 0; color: #276749; }
		.recipe-quick-stats { margin: 10px 0; }
		.auth-prompt { background: #bee3f8; border: 1px solid #90cdf4; padding: 15px; border-radius: 8px; margin: 10px 0; }
		.auth-prompt p { margin: 0 0 10px 0; color: #2c5282; }
	</style>
</head>
<body>
	<div class="header">
		<div class="container">
			<div class="nav">
				<h1>🍽️ Alchemorsel v3</h1>
				<div class="nav-links">
					%s
					%s
				</div>
			</div>
		</div>
	</div>
	<div class="container">
		%s
	</div>
</body>
</html>
		`, templateName, getUserInfoDisplay(user), navLinks, getPageContent(templateName, data))
		
		w.Write([]byte(html))
		return
	}
	
	if err := templates.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getUserInfoDisplay(user interface{}) string {
	if u, ok := user.(*User); ok && u != nil {
		return fmt.Sprintf(`<span class="user-info">Welcome, %s (%s)</span>`, u.Name, u.Role)
	}
	return ""
}

func getPageContent(templateName string, data interface{}) string {
	dataMap := data.(map[string]interface{})
	isAuth := dataMap["IsAuthenticated"].(bool)
	
	switch templateName {
	case "home":
		content := `
			<div class="card">
				<h2>🏠 Welcome to Alchemorsel v3</h2>
				<p>Enterprise Recipe Platform with Real Authentication & Database</p>
		`
		
		if !isAuth {
			content += `
				<div class="protected-notice">
					🔒 <strong>Authentication Required:</strong> Some features require you to <a href="/login">login</a> or <a href="/register">register</a> first.
				</div>
			`
		}
		
		content += `
			</div>
			
			<div class="chat-interface">
				<h3>🤖 AI Chef Assistant</h3>
				<p>Ask me anything about cooking, recipes, or ingredients!</p>
				<form hx-post="/ai/chat" hx-target="#chat-messages" hx-swap="beforeend">
					<div class="form-group">
						<input type="text" name="message" class="form-input" placeholder="What would you like to cook today?" required>
					</div>
					<button type="submit" class="btn">Send Message</button>
				</form>
				<div id="chat-messages"></div>
			</div>
			
			<div class="card">
				<h3>🔍 Recipe Search</h3>
				<form hx-post="/htmx/recipes/search" hx-target="#search-results">
					<div class="form-group">
						<input type="text" name="q" class="form-input" placeholder="Search recipes..." autocomplete="off">
					</div>
					<button type="submit" class="btn">Search</button>
				</form>
				<div id="search-results"></div>
			</div>
		`
		return content
		
	case "login":
		return `
			<div class="card">
				<h2>🔐 Login</h2>
				<form method="post" action="/auth/login">
					<div class="form-group">
						<label>Email:</label>
						<input type="email" name="email" class="form-input" required>
					</div>
					<div class="form-group">
						<label>Password:</label>
						<input type="password" name="password" class="form-input" required>
					</div>
					<button type="submit" class="btn">Login</button>
					<a href="/register" class="btn">Register Instead</a>
				</form>
				
				<div style="margin-top: 20px; padding: 15px; background: #f0f7ff; border-radius: 4px;">
					<h4>Demo Accounts:</h4>
					<ul>
						<li><strong>Chef:</strong> chef@alchemorsel.com / password</li>
						<li><strong>User:</strong> user@alchemorsel.com / password</li>
					</ul>
				</div>
			</div>
		`
		
	case "register":
		return `
			<div class="card">
				<h2>📝 Register</h2>
				<form method="post" action="/auth/register">
					<div class="form-group">
						<label>Name:</label>
						<input type="text" name="name" class="form-input" required>
					</div>
					<div class="form-group">
						<label>Email:</label>
						<input type="email" name="email" class="form-input" required>
					</div>
					<div class="form-group">
						<label>Password:</label>
						<input type="password" name="password" class="form-input" required>
					</div>
					<div class="form-group">
						<label>Confirm Password:</label>
						<input type="password" name="password_confirm" class="form-input" required>
					</div>
					<button type="submit" class="btn">Register</button>
					<a href="/login" class="btn">Login Instead</a>
				</form>
			</div>
		`
		
	case "recipes":
		recipesData, _ := dataMap["Recipes"].([]Recipe)
		html := `<div class="card"><h2>📖 All Recipes</h2></div><div class="recipe-grid">`
		
		if len(recipesData) == 0 {
			html += `<div class="card"><p>No recipes found. Be the first to <a href="/recipes/new">create one</a>!</p></div>`
		} else {
			for _, recipe := range recipesData {
				aiBadge := ""
				if recipe.AIGenerated {
					aiBadge = `<span class="badge ai-badge">AI Generated</span>`
				}
				
				html += fmt.Sprintf(`
					<div class="recipe-card">
						<h3><a href="/recipes/%s">%s</a></h3>
						<p>%s</p>
						<div style="margin: 10px 0;">
							<span class="badge">%s</span>
							<span class="badge">%s</span>
							%s
						</div>
						<div style="margin-top: 10px;">
							<small>👤 %s | ❤️ %d likes | ⭐ %.1f/5 | 👁️ %d views</small>
						</div>
					</div>`,
					recipe.ID, recipe.Title, recipe.Description,
					recipe.Cuisine, recipe.Difficulty, aiBadge,
					recipe.Author.Name, recipe.LikesCount, recipe.AverageRating, recipe.ViewsCount)
			}
		}
		html += "</div>"
		return html
		
	case "recipe-form":
		if !isAuth {
			return `<div class="card">
				<div class="error">🔒 You must be logged in to create recipes. <a href="/login">Login here</a></div>
			</div>`
		}
		
		return `
			<div class="card">
				<h2>➕ Create New Recipe</h2>
				<form method="post" action="/recipes">
					<div class="form-group">
						<label>Recipe Title:</label>
						<input type="text" name="title" class="form-input" required>
					</div>
					<div class="form-group">
						<label>Description:</label>
						<textarea name="description" class="form-input" rows="3" required></textarea>
					</div>
					<div class="form-group">
						<label>Cuisine:</label>
						<select name="cuisine" class="form-input">
							<option value="italian">Italian</option>
							<option value="asian">Asian</option>
							<option value="mexican">Mexican</option>
							<option value="american">American</option>
							<option value="fusion">Fusion</option>
							<option value="indian">Indian</option>
							<option value="french">French</option>
						</select>
					</div>
					<div class="form-group">
						<label>Difficulty:</label>
						<select name="difficulty" class="form-input">
							<option value="easy">Easy</option>
							<option value="medium">Medium</option>
							<option value="hard">Hard</option>
						</select>
					</div>
					<button type="submit" class="btn">Create Recipe</button>
					<a href="/dashboard" class="btn">Cancel</a>
				</form>
			</div>
		`
		
	case "dashboard":
		if !isAuth {
			return `<div class="card">
				<div class="error">🔒 You must be logged in to view your dashboard. <a href="/login">Login here</a></div>
			</div>`
		}
		
		user := dataMap["User"].(*User)
		stats := dataMap["Stats"].(map[string]interface{})
		userRecipes, _ := dataMap["UserRecipes"].([]Recipe)
		
		html := fmt.Sprintf(`
			<div class="card">
				<h2>👤 Welcome back, %s!</h2>
				<p>Role: <strong>%s</strong> | Member since: %s</p>
			</div>
			
			<div class="card">
				<h3>📊 Your Statistics</h3>
				<div class="stats-grid">
					<div class="stat-card">
						<div class="stat-number">%d</div>
						<div class="stat-label">Recipes Created</div>
					</div>
					<div class="stat-card">
						<div class="stat-number">%d</div>
						<div class="stat-label">Total Likes</div>
					</div>
					<div class="stat-card">
						<div class="stat-number">%d</div>
						<div class="stat-label">Followers</div>
					</div>
					<div class="stat-card">
						<div class="stat-number">%d</div>
						<div class="stat-label">Following</div>
					</div>
				</div>
			</div>
			
			<div class="card">
				<h3>📝 Your Recipes</h3>
				<a href="/recipes/new" class="btn">Create New Recipe</a>
		`, user.Name, user.Role, user.CreatedAt.Format("Jan 2, 2006"),
			stats["RecipeCount"], stats["TotalLikes"], stats["Followers"], stats["Following"])
		
		if len(userRecipes) == 0 {
			html += `<p style="margin-top: 20px;">You haven't created any recipes yet. <a href="/recipes/new">Create your first recipe</a>!</p>`
		} else {
			html += `<div class="recipe-grid" style="margin-top: 20px;">`
			for _, recipe := range userRecipes {
				aiBadge := ""
				if recipe.AIGenerated {
					aiBadge = `<span class="badge ai-badge">AI Generated</span>`
				}
				
				html += fmt.Sprintf(`
					<div class="recipe-card">
						<h4><a href="/recipes/%s">%s</a></h4>
						<p>%s</p>
						<div>
							<span class="badge">%s</span>
							<span class="badge">%s</span>
							%s
						</div>
						<div style="margin-top: 10px;">
							<small>❤️ %d likes | ⭐ %.1f/5 | 👁️ %d views</small>
						</div>
						<div style="margin-top: 10px;">
							<small>Created: %s</small>
						</div>
					</div>`,
					recipe.ID, recipe.Title, recipe.Description,
					recipe.Cuisine, recipe.Difficulty, aiBadge,
					recipe.LikesCount, recipe.AverageRating, recipe.ViewsCount,
					recipe.CreatedAt.Format("Jan 2, 2006"))
			}
			html += "</div>"
		}
		html += "</div>"
		return html
		
	default:
		return "<div class=\"card\"><p>Page content would go here.</p></div>"
	}
}

func renderError(w http.ResponseWriter, message string) {
	html := fmt.Sprintf(`<div class="error">❌ %s</div>`, message)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func renderHTMXError(w http.ResponseWriter, message string) {
	html := fmt.Sprintf(`<div class="error">❌ %s</div>`, message)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}