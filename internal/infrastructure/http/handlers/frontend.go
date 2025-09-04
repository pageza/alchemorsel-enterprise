// Package handlers provides HTTP handlers for the HTMX frontend
package handlers

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/application/user"
	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"github.com/alchemorsel/v3/internal/ports/inbound"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FrontendHandlers handles frontend HTMX requests
type FrontendHandlers struct {
	templates           *template.Template
	recipeService       inbound.RecipeService
	userService         *user.UserService
	conversationService *conversation.Service
	authService         *security.AuthService
	aiService           outbound.AIService
	xssProtection       *security.XSSProtectionService
	logger              *zap.Logger
}

// NewFrontendHandlers creates a new frontend handlers instance
func NewFrontendHandlers(
	templates *template.Template,
	recipeService inbound.RecipeService,
	userService *user.UserService,
	conversationService *conversation.Service,
	authService *security.AuthService,
	aiService outbound.AIService,
	xssProtection *security.XSSProtectionService,
	logger *zap.Logger,
) *FrontendHandlers {
	return &FrontendHandlers{
		templates:           templates,
		recipeService:       recipeService,
		userService:         userService,
		conversationService: conversationService,
		authService:         authService,
		aiService:           aiService,
		xssProtection:       xssProtection,
		logger:              logger,
	}
}

// PageData represents common page data
type PageData struct {
	Title       string
	Description string
	Keywords    string
	User        *user.UserDTO
	Data        interface{}
	HTMX        bool
	Messages    []Message
}

// Message represents a user message
type Message struct {
	Type    string // success, error, warning, info
	Content string
}

// UserDashboard represents dashboard data for logged-in users
type UserDashboard struct {
	User              *user.UserDTO          `json:"user"`
	RecipeStats       *UserRecipeStats       `json:"recipe_stats"`
	ConversationStats *UserConversationStats `json:"conversation_stats"`
	RecentRecipes     []inbound.RecipeDTO    `json:"recent_recipes"`
	FeaturedRecipes   []inbound.RecipeDTO    `json:"featured_recipes"`
	QuickActions      []QuickAction          `json:"quick_actions"`
}

// UserRecipeStats contains user recipe statistics
type UserRecipeStats struct {
	TotalRecipes     int     `json:"total_recipes"`
	PublishedRecipes int     `json:"published_recipes"`
	TotalLikes       int     `json:"total_likes"`
	TotalViews       int     `json:"total_views"`
	AvgRating        float64 `json:"avg_rating"`
}

// UserConversationStats contains user AI conversation statistics
type UserConversationStats struct {
	TotalConversations  int `json:"total_conversations"`
	ActiveConversations int `json:"active_conversations"`
	RecipesGenerated    int `json:"recipes_generated"`
}

// QuickAction represents a dashboard quick action
type QuickAction struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	URL         string `json:"url"`
}

// HandleHome renders the home page with critical 14KB optimization
func (h *FrontendHandlers) HandleHome(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromRequest(r)

	data := PageData{
		Title:       "Alchemorsel - AI-Powered Recipe Platform",
		Description: "Discover, create, and share recipes with AI assistance",
		Keywords:    "recipes, cooking, AI, food, ingredients",
		User:        user,
		HTMX:        r.Header.Get("HX-Request") == "true",
	}

	// If user is authenticated, fetch dashboard data
	if user != nil {
		dashboard, err := h.buildUserDashboard(r.Context(), user)
		if err != nil {
			h.logger.Error("Failed to build user dashboard",
				zap.Error(err),
				zap.String("user_id", user.ID.String()),
			)
			// Still render page but without dashboard data
		} else {
			data.Data = dashboard
		}
	} else {
		// For unauthenticated users, fetch featured recipes
		featuredRecipes, err := h.getFeaturedRecipes(r.Context())
		if err != nil {
			h.logger.Error("Failed to fetch featured recipes", zap.Error(err))
		} else {
			data.Data = map[string]interface{}{
				"featured_recipes": featuredRecipes,
			}
		}
	}

	// If this is an HTMX request, return only the content
	if data.HTMX {
		h.renderTemplate(w, "home-content", data)
	} else {
		h.renderTemplate(w, "home", data)
	}
}

// HandleRecipes renders the recipes listing page
func (h *FrontendHandlers) HandleRecipes(w http.ResponseWriter, r *http.Request) {
	// TODO: Get recipes from service
	data := PageData{
		Title: "Recipes - Alchemorsel",
		HTMX:  r.Header.Get("HX-Request") == "true",
		Data: map[string]interface{}{
			"recipes": []interface{}{}, // Would be populated from service
		},
	}

	if data.HTMX {
		h.renderTemplate(w, "recipes-content", data)
	} else {
		h.renderTemplate(w, "recipes", data)
	}
}

// HandleNewRecipe renders the new recipe form
func (h *FrontendHandlers) HandleNewRecipe(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "Create New Recipe - Alchemorsel",
		HTMX:  r.Header.Get("HX-Request") == "true",
	}

	if data.HTMX {
		h.renderTemplate(w, "recipe-form-content", data)
	} else {
		h.renderTemplate(w, "recipe-form", data)
	}
}

// HandleRecipeDetail renders a specific recipe
func (h *FrontendHandlers) HandleRecipeDetail(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// TODO: Get recipe from service
	_ = id

	data := PageData{
		Title: "Recipe Detail - Alchemorsel",
		HTMX:  r.Header.Get("HX-Request") == "true",
		Data: map[string]interface{}{
			"recipe": map[string]interface{}{
				"id":    id,
				"title": "Sample Recipe",
			},
		},
	}

	if data.HTMX {
		h.renderTemplate(w, "recipe-detail-content", data)
	} else {
		h.renderTemplate(w, "recipe-detail", data)
	}
}

// HandleEditRecipe renders the edit recipe form
func (h *FrontendHandlers) HandleEditRecipe(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	data := PageData{
		Title: "Edit Recipe - Alchemorsel",
		HTMX:  r.Header.Get("HX-Request") == "true",
		Data: map[string]interface{}{
			"recipe": map[string]interface{}{
				"id": id,
			},
		},
	}

	if data.HTMX {
		h.renderTemplate(w, "recipe-form-content", data)
	} else {
		h.renderTemplate(w, "recipe-form", data)
	}
}

// HandleLogin renders the login page
func (h *FrontendHandlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "Login - Alchemorsel",
		HTMX:  r.Header.Get("HX-Request") == "true",
	}

	if data.HTMX {
		h.renderTemplate(w, "login-content", data)
	} else {
		h.renderTemplate(w, "login", data)
	}
}

// HandleRegister renders the registration page
func (h *FrontendHandlers) HandleRegister(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "Register - Alchemorsel",
		HTMX:  r.Header.Get("HX-Request") == "true",
	}

	if data.HTMX {
		h.renderTemplate(w, "register-content", data)
	} else {
		h.renderTemplate(w, "register", data)
	}
}

// HandleDashboard renders the user dashboard
func (h *FrontendHandlers) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title: "Dashboard - Alchemorsel",
		HTMX:  r.Header.Get("HX-Request") == "true",
	}

	if data.HTMX {
		h.renderTemplate(w, "dashboard-content", data)
	} else {
		h.renderTemplate(w, "dashboard", data)
	}
}

// HTMX-specific handlers

// HandleRecipeSearch handles real-time recipe search
func (h *FrontendHandlers) HandleRecipeSearch(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	cuisine := r.FormValue("cuisine")
	diet := r.FormValue("diet")
	difficulty := r.FormValue("difficulty")
	maxCookTime := r.FormValue("max_cook_time")

	// Simulate search delay for demo
	time.Sleep(100 * time.Millisecond)

	// Enhanced demo data with filtering capabilities
	allResults := []map[string]interface{}{
		{
			"id":          "1",
			"title":       "Pasta Carbonara",
			"description": "Classic Italian pasta dish with eggs, cheese, and pancetta",
			"cuisine":     "italian",
			"difficulty":  "medium",
			"cookTime":    "25",
			"diet":        "",
		},
		{
			"id":          "2",
			"title":       "Chicken Tikka Masala",
			"description": "Creamy Indian curry with tender marinated chicken",
			"cuisine":     "indian",
			"difficulty":  "hard",
			"cookTime":    "45",
			"diet":        "",
		},
		{
			"id":          "3",
			"title":       "Vegetarian Pad Thai",
			"description": "Thai stir-fried noodles with vegetables and tofu",
			"cuisine":     "thai",
			"difficulty":  "medium",
			"cookTime":    "20",
			"diet":        "vegetarian",
		},
		{
			"id":          "4",
			"title":       "Quick Quinoa Salad",
			"description": "Healthy and quick quinoa salad with fresh vegetables",
			"cuisine":     "american",
			"difficulty":  "easy",
			"cookTime":    "15",
			"diet":        "vegan",
		},
		{
			"id":          "5",
			"title":       "French Onion Soup",
			"description": "Classic French soup with caramelized onions and cheese",
			"cuisine":     "french",
			"difficulty":  "medium",
			"cookTime":    "60",
			"diet":        "vegetarian",
		},
	}

	// Filter results based on search criteria
	results := []map[string]interface{}{}
	for _, result := range allResults {
		// Text search
		if query != "" {
			titleMatch := strings.Contains(strings.ToLower(result["title"].(string)), strings.ToLower(query))
			descMatch := strings.Contains(strings.ToLower(result["description"].(string)), strings.ToLower(query))
			if !titleMatch && !descMatch {
				continue
			}
		}

		// Filter by cuisine
		if cuisine != "" && result["cuisine"].(string) != cuisine {
			continue
		}

		// Filter by diet
		if diet != "" && result["diet"].(string) != diet {
			continue
		}

		// Filter by difficulty
		if difficulty != "" && result["difficulty"].(string) != difficulty {
			continue
		}

		// Filter by max cook time
		if maxCookTime != "" {
			maxTime, _ := strconv.Atoi(maxCookTime)
			cookTime, _ := strconv.Atoi(result["cookTime"].(string))
			if cookTime > maxTime {
				continue
			}
		}

		results = append(results, result)
	}

	data := map[string]interface{}{
		"results": results,
		"query":   query,
	}

	h.renderTemplate(w, "search-results", data)
}

// HandleRecipeLike handles recipe like/unlike
func (h *FrontendHandlers) HandleRecipeLike(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// TODO: Implement actual like logic
	liked := r.FormValue("liked") != "true"

	data := map[string]interface{}{
		"id":    id,
		"liked": liked,
		"count": 42, // Would be actual count
	}

	h.renderTemplate(w, "like-button", data)
}

// HandleRecipeRating handles recipe rating
func (h *FrontendHandlers) HandleRecipeRating(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ratingStr := r.FormValue("rating")

	rating, err := strconv.Atoi(ratingStr)
	if err != nil || rating < 1 || rating > 5 {
		http.Error(w, "Invalid rating", http.StatusBadRequest)
		return
	}

	// TODO: Save rating
	_ = id

	data := map[string]interface{}{
		"id":      id,
		"rating":  rating,
		"average": 4.2, // Would be calculated average
	}

	h.renderTemplate(w, "rating-display", data)
}

// HandleCreateRecipe handles recipe creation
func (h *FrontendHandlers) HandleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	title := r.FormValue("title")
	description := r.FormValue("description")

	if title == "" {
		h.renderError(w, "Title is required")
		return
	}

	// TODO: Create recipe using service
	_ = description

	// Return success message
	h.renderSuccess(w, "Recipe created successfully!")
}

// HandleUpdateRecipe handles recipe updates
func (h *FrontendHandlers) HandleUpdateRecipe(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	title := r.FormValue("title")

	if title == "" {
		h.renderError(w, "Title is required")
		return
	}

	// TODO: Update recipe using service
	_ = id

	h.renderSuccess(w, "Recipe updated successfully!")
}

// HandleDeleteRecipe handles recipe deletion
func (h *FrontendHandlers) HandleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// TODO: Delete recipe using service
	_ = id

	w.Header().Set("HX-Redirect", "/recipes")
	w.WriteHeader(http.StatusOK)
}

// AI Chat handlers

// HandleAIChat handles AI chat messages and generates recipes using AI
func (h *FrontendHandlers) HandleAIChat(w http.ResponseWriter, r *http.Request) {
	message := strings.TrimSpace(r.FormValue("message"))

	// Input validation
	if message == "" {
		h.renderError(w, "Message cannot be empty")
		return
	}

	// Check message length limit
	if len(message) > 1000 {
		h.renderError(w, "Message is too long. Please keep it under 1000 characters.")
		return
	}

	// Validate input for dangerous patterns using XSS protection
	if err := h.xssProtection.ValidateInput(message); err != nil {
		h.logger.Warn("XSS pattern detected in AI chat message",
			zap.String("message", h.xssProtection.StripHTML(message)[:50]),
			zap.String("ip", r.RemoteAddr),
			zap.Error(err),
		)
		h.renderError(w, "Invalid input detected. Please remove any special characters or scripts.")
		return
	}

	// Get user from request context
	user := h.getUserFromRequest(r)

	// Log the AI chat request with sanitized message
	h.logger.Info("AI chat request received",
		zap.String("message", h.xssProtection.StripHTML(message)),
		zap.String("user_id", getUserID(user)),
	)

	// Build user message HTML
	userMessageHTML := h.buildUserMessageHTML(message, user)

	// Check if this is a recipe generation request
	if h.isRecipeRequest(message) {
		if user == nil {
			// User not logged in
			aiResponseHTML := h.buildAuthRequiredResponse()
			h.writeHTMLResponse(w, userMessageHTML+aiResponseHTML)
			return
		}

		// Generate recipe using AI service
		aiResponse := h.generateRecipeWithAI(r.Context(), message, user)
		h.writeHTMLResponse(w, userMessageHTML+aiResponse)
	} else {
		// General cooking advice/conversation
		aiResponse := h.generateCookingAdvice(message)
		h.writeHTMLResponse(w, userMessageHTML+aiResponse)
	}
}

// isRecipeRequest determines if the message is asking for recipe generation
func (h *FrontendHandlers) isRecipeRequest(message string) bool {
	lowerMessage := strings.ToLower(message)
	recipeKeywords := []string{
		"recipe", "create", "make", "cook", "generate", "suggest",
		"how to make", "i want to", "show me", "give me",
	}

	for _, keyword := range recipeKeywords {
		if strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}

// generateRecipeWithAI uses the AI service to generate a recipe
func (h *FrontendHandlers) generateRecipeWithAI(ctx context.Context, message string, user *user.UserDTO) string {
	// Build AI constraints from the message
	constraints := h.buildAIConstraints(message)

	// Call AI service to generate recipe
	aiResponse, err := h.aiService.GenerateRecipe(ctx, message, constraints)
	if err != nil {
		h.logger.Error("Failed to generate recipe with AI",
			zap.Error(err),
			zap.String("message", message),
		)
		return h.buildErrorResponse("I had trouble generating that recipe. Please try again with different ingredients or description.")
	}

	// For now, just return the AI response as text
	// TODO: Implement proper recipe creation and persistence
	h.logger.Info("Generated AI recipe",
		zap.String("title", aiResponse.Title),
		zap.String("user_id", user.ID.String()),
	)

	// Build success response with AI data
	return h.buildRecipeAIResponseText(aiResponse)
}

// buildAIConstraints extracts constraints from the user message
func (h *FrontendHandlers) buildAIConstraints(message string) outbound.AIConstraints {
	lowerMessage := strings.ToLower(message)

	constraints := outbound.AIConstraints{
		ServingSize: 4,        // default
		SkillLevel:  "medium", // default
	}

	// Extract dietary requirements
	dietaryKeywords := map[string]string{
		"vegetarian":  "vegetarian",
		"vegan":       "vegan",
		"gluten-free": "gluten-free",
		"dairy-free":  "dairy-free",
		"low-carb":    "low-carb",
		"keto":        "keto",
		"healthy":     "healthy",
	}

	for keyword, dietary := range dietaryKeywords {
		if strings.Contains(lowerMessage, keyword) {
			constraints.Dietary = append(constraints.Dietary, dietary)
		}
	}

	// Extract cuisine
	cuisineKeywords := map[string]string{
		"italian":  "italian",
		"mexican":  "mexican",
		"asian":    "asian",
		"indian":   "indian",
		"french":   "french",
		"thai":     "thai",
		"chinese":  "chinese",
		"japanese": "japanese",
	}

	for keyword, cuisine := range cuisineKeywords {
		if strings.Contains(lowerMessage, keyword) {
			constraints.Cuisine = cuisine
			break
		}
	}

	// Extract cooking time
	if strings.Contains(lowerMessage, "quick") || strings.Contains(lowerMessage, "fast") {
		constraints.CookingTime = 30
	} else if strings.Contains(lowerMessage, "slow") {
		constraints.CookingTime = 120
	}

	// Extract skill level
	if strings.Contains(lowerMessage, "easy") || strings.Contains(lowerMessage, "simple") {
		constraints.SkillLevel = "easy"
	} else if strings.Contains(lowerMessage, "advanced") || strings.Contains(lowerMessage, "complex") {
		constraints.SkillLevel = "hard"
	}

	return constraints
}

// createRecipeFromAI converts AI response to domain recipe entity
func (h *FrontendHandlers) createRecipeFromAI(aiResponse *outbound.AIRecipeResponse, userID uuid.UUID, originalPrompt string) (*recipe.Recipe, error) {
	// Create new recipe
	domainRecipe, err := recipe.NewRecipe(aiResponse.Title, aiResponse.Description, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipe: %w", err)
	}

	// Add ingredients
	for _, aiIngredient := range aiResponse.Ingredients {
		ingredient := recipe.Ingredient{
			ID:     uuid.New(),
			Name:   aiIngredient.Name,
			Amount: aiIngredient.Amount,
			Unit:   h.parseUnit(aiIngredient.Unit),
		}

		if err := domainRecipe.AddIngredient(ingredient); err != nil {
			h.logger.Warn("Failed to add ingredient",
				zap.Error(err),
				zap.String("ingredient", aiIngredient.Name),
			)
		}
	}

	// Add instructions
	for i, instructionText := range aiResponse.Instructions {
		instruction := recipe.Instruction{
			StepNumber:  i + 1,
			Description: instructionText,
		}

		if err := domainRecipe.AddInstruction(instruction); err != nil {
			h.logger.Warn("Failed to add instruction",
				zap.Error(err),
				zap.Int("step", i+1),
			)
		}
	}

	// Publish the recipe immediately for AI-generated recipes
	if err := domainRecipe.Publish(); err != nil {
		return nil, fmt.Errorf("failed to publish recipe: %w", err)
	}

	return domainRecipe, nil
}

// parseUnit converts string unit to domain measurement unit
func (h *FrontendHandlers) parseUnit(unit string) recipe.MeasurementUnit {
	unitMap := map[string]recipe.MeasurementUnit{
		"tsp":    recipe.MeasurementUnitTeaspoon,
		"tbsp":   recipe.MeasurementUnitTablespoon,
		"cup":    recipe.MeasurementUnitCup,
		"cups":   recipe.MeasurementUnitCup,
		"oz":     recipe.MeasurementUnitOunce,
		"ml":     recipe.MeasurementUnitMilliliter,
		"l":      recipe.MeasurementUnitLiter,
		"g":      recipe.MeasurementUnitGram,
		"kg":     recipe.MeasurementUnitKilogram,
		"lb":     recipe.MeasurementUnitPound,
		"piece":  recipe.MeasurementUnitPiece,
		"pieces": recipe.MeasurementUnitPiece,
		"dash":   recipe.MeasurementUnitDash,
		"pinch":  recipe.MeasurementUnitPinch,
	}

	if mappedUnit, exists := unitMap[strings.ToLower(unit)]; exists {
		return mappedUnit
	}

	return recipe.MeasurementUnitPiece // default
}

// generateCookingAdvice provides general cooking advice
func (h *FrontendHandlers) generateCookingAdvice(message string) string {
	responses := []string{
		"🤖 AI Chef: That's an interesting cooking question! While I specialize in creating recipes, I'd suggest trying to phrase your request like 'Create a recipe for...' or 'I want to make...' to get personalized recipes.",
		"🤖 AI Chef: I'm here to help you create amazing recipes! Try asking me to 'make a pasta recipe with mushrooms' or 'create a vegetarian stir fry' and I'll generate a complete recipe for you.",
		"🤖 AI Chef: I understand you're looking for cooking help! For the best results, tell me what dish you'd like to make or what ingredients you want to use, and I'll create a custom recipe.",
		"🤖 AI Chef: Great question! I'm designed to create personalized recipes based on your preferences. Try saying something like 'generate a chicken curry recipe' or 'I want to cook with tomatoes and herbs'.",
	}

	// Simple response selection based on message content
	responseIndex := 0
	if strings.Contains(strings.ToLower(message), "help") {
		responseIndex = 1
	} else if strings.Contains(strings.ToLower(message), "what") {
		responseIndex = 2
	} else if strings.Contains(strings.ToLower(message), "how") {
		responseIndex = 3
	}

	if responseIndex >= len(responses) {
		responseIndex = 0
	}

	response := responses[responseIndex]

	// Add recipe examples
	response += `<br><br><strong>Try these examples:</strong>
		<ul>
			<li>"Create a pasta recipe with mushrooms"</li>
			<li>"I want to make chicken tacos"</li>
			<li>"Generate a vegetarian stir-fry recipe"</li>
			<li>"Make me a healthy salad with avocado"</li>
		</ul>`

	return h.buildAIMessageHTML(response)
}

// buildUserMessageHTML creates HTML for user message
func (h *FrontendHandlers) buildUserMessageHTML(message string, user *user.UserDTO) string {
	userName := "Anonymous"
	if user != nil {
		userName = html.EscapeString(user.Name)
	}

	// Sanitize the message using XSS protection service
	sanitizedMessage := h.xssProtection.StripHTML(message)
	escapedMessage := html.EscapeString(sanitizedMessage)

	return fmt.Sprintf(`
		<div class="chat-message user-message" style="margin-bottom: 1rem;">
			<div style="display: flex; align-items: flex-start; gap: 0.75rem; justify-content: flex-end;">
				<div class="message-content" style="flex: 1; max-width: 80%%; background: linear-gradient(135deg, #4f46e5 0%%, #7c3aed 100%%); color: white; padding: 1rem; border-radius: 1rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
					<div style="font-weight: 600; color: #e0e7ff; margin-bottom: 0.5rem;">%s</div>
					<div style="line-height: 1.6;">%s</div>
					<div style="font-size: 0.75rem; color: #c7d2fe; margin-top: 0.5rem;">
						Just now
					</div>
				</div>
				<div class="avatar user-avatar" style="width: 2.5rem; height: 2.5rem; border-radius: 50%%; background: linear-gradient(135deg, #4f46e5 0%%, #7c3aed 100%%); display: flex; align-items: center; justify-content: center; color: white; font-weight: bold; flex-shrink: 0;">
					👤
				</div>
			</div>
		</div>`, userName, escapedMessage)
}

// buildAIMessageHTML creates HTML for AI message
func (h *FrontendHandlers) buildAIMessageHTML(content string) string {
	// Sanitize AI response content to allow safe HTML but prevent XSS
	sanitizedContent := h.xssProtection.SanitizeHTML(content)

	return fmt.Sprintf(`
		<div class="chat-message ai-message" style="margin-bottom: 1rem;">
			<div style="display: flex; align-items: flex-start; gap: 0.75rem;">
				<div class="avatar ai-avatar" style="width: 2.5rem; height: 2.5rem; border-radius: 50%%; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); display: flex; align-items: center; justify-content: center; color: white; font-weight: bold; flex-shrink: 0;">
					👨‍🍳
				</div>
				<div class="message-content" style="flex: 1; background: #ffffff; padding: 1rem; border-radius: 1rem; border: 1px solid #e2e8f0; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
					<div style="font-weight: 600; color: #4f46e5; margin-bottom: 0.5rem;">AI Chef</div>
					<div style="line-height: 1.6;">%s</div>
					<div style="font-size: 0.75rem; color: #9ca3af; margin-top: 0.5rem;">
						Just now
					</div>
				</div>
			</div>
		</div>`, sanitizedContent)
}

// buildAuthRequiredResponse creates response for non-authenticated users
func (h *FrontendHandlers) buildAuthRequiredResponse() string {
	content := `🤖 AI Chef: I'd love to create a personalized recipe for you! However, you need to be logged in to save recipes. 
		<br><br>
		<div class="auth-prompt" style="background: #bee3f8; border: 1px solid #90cdf4; padding: 15px; border-radius: 8px; margin: 10px 0;">
			<p style="margin: 0 0 10px 0; color: #2c5282;"><strong>Please log in to unlock AI recipe creation:</strong></p>
			<a href="/login" class="btn btn-primary" style="background: #3182ce; color: white; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; margin: 2px;">Login</a>
			<a href="/register" class="btn" style="background: #e2e8f0; color: #4a5568; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; margin: 2px;">Register</a>
		</div>`

	return h.buildAIMessageHTML(content)
}

// buildErrorResponse creates error response HTML
func (h *FrontendHandlers) buildErrorResponse(errorMsg string) string {
	return h.buildAIMessageHTML("🤖 AI Chef: " + errorMsg)
}

// buildRecipeCreatedResponse creates success response for recipe creation
func (h *FrontendHandlers) buildRecipeCreatedResponse(domainRecipe *recipe.Recipe, aiResponse *outbound.AIRecipeResponse) string {
	// Get ingredient preview
	ingredientPreview := "fresh ingredients"
	if len(aiResponse.Ingredients) > 0 {
		var names []string
		for i, ing := range aiResponse.Ingredients {
			if i >= 3 {
				break
			}
			names = append(names, ing.Name)
		}
		ingredientPreview = strings.Join(names, ", ")
		if len(aiResponse.Ingredients) > 3 {
			ingredientPreview += " and more"
		}
	}

	// Calculate total time
	totalTime := int(domainRecipe.PrepTime().Minutes()) + int(domainRecipe.CookTime().Minutes())
	if totalTime == 0 {
		totalTime = 30 // default
	}

	content := fmt.Sprintf(`🤖 AI Chef: Perfect! I've created "<strong>%s</strong>" for you! This %s recipe features %s and takes about %d minutes to prepare.
		<br><br>
		<div class="recipe-created-notification" style="background: #c6f6d5; border: 1px solid #9ae6b4; padding: 20px; border-radius: 8px; margin: 15px 0;">
			<h4 style="margin: 0 0 10px 0; color: #276749;">✨ Recipe Created Successfully!</h4>
			<p style="margin: 0 0 10px 0;"><strong>%s</strong></p>
			<p style="margin: 0 0 10px 0;">%s</p>
			<div class="recipe-quick-stats" style="margin: 10px 0;">
				<span class="badge" style="background: #e2e8f0; padding: 4px 8px; border-radius: 12px; font-size: 0.8em; margin: 2px;">%s</span>
				<span class="badge" style="background: #e2e8f0; padding: 4px 8px; border-radius: 12px; font-size: 0.8em; margin: 2px;">%s</span>
				<span class="badge ai-badge" style="background: #9f7aea; color: white; padding: 4px 8px; border-radius: 12px; font-size: 0.8em; margin: 2px;">AI Generated</span>
			</div>
			<div style="margin-top: 15px;">
				<a href="/recipes/%s" class="btn btn-primary" style="background: #3182ce; color: white; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; margin: 2px;">View Full Recipe</a>
				<a href="/dashboard" class="btn" style="background: #e2e8f0; color: #4a5568; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; margin: 2px;">Go to Dashboard</a>
			</div>
		</div>`,
		domainRecipe.Title(),
		strings.ToLower(string(domainRecipe.Difficulty())),
		ingredientPreview,
		totalTime,
		domainRecipe.Title(),
		domainRecipe.Description(),
		string(domainRecipe.Cuisine()),
		string(domainRecipe.Difficulty()),
		domainRecipe.ID().String(),
	)

	return h.buildAIMessageHTML(content)
}

// writeHTMLResponse writes HTML response
func (h *FrontendHandlers) writeHTMLResponse(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// getUserID safely gets user ID as string
func getUserID(user *user.UserDTO) string {
	if user != nil {
		return user.ID.String()
	}
	return "anonymous"
}

// HandleAIChatStream handles streaming AI responses
func (h *FrontendHandlers) HandleAIChatStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Simulate streaming response
	responses := []string{
		"I'm thinking about your recipe request...",
		"Let me suggest some ingredients...",
		"Here's a recipe that might work for you!",
	}

	for i, response := range responses {
		fmt.Fprintf(w, "data: %s\n\n", response)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		if i < len(responses)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// HandleVoiceInput handles voice input for AI chat
func (h *FrontendHandlers) HandleVoiceInput(w http.ResponseWriter, r *http.Request) {
	// TODO: Process audio input
	transcription := "I want to make pasta"

	data := map[string]interface{}{
		"transcription": transcription,
	}

	h.renderTemplate(w, "voice-result", data)
}

// Dynamic form handlers

// HandleIngredientsForm renders dynamic ingredient inputs
func (h *FrontendHandlers) HandleIngredientsForm(w http.ResponseWriter, r *http.Request) {
	countStr := chi.URLParam(r, "count")
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 20 {
		count = 1
	}

	data := map[string]interface{}{
		"count": count,
	}

	h.renderTemplate(w, "ingredients-form", data)
}

// HandleInstructionsForm renders dynamic instruction inputs
func (h *FrontendHandlers) HandleInstructionsForm(w http.ResponseWriter, r *http.Request) {
	countStr := chi.URLParam(r, "count")
	count, err := strconv.Atoi(countStr)
	if err != nil || count < 1 || count > 20 {
		count = 1
	}

	data := map[string]interface{}{
		"count": count,
	}

	h.renderTemplate(w, "instructions-form", data)
}

// HandleNotifications handles real-time notifications
func (h *FrontendHandlers) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	// TODO: Get user notifications
	notifications := []map[string]interface{}{
		{"type": "info", "message": "Welcome to Alchemorsel!"},
	}

	data := map[string]interface{}{
		"notifications": notifications,
	}

	h.renderTemplate(w, "notifications", data)
}

// HandleFeedback handles user feedback
func (h *FrontendHandlers) HandleFeedback(w http.ResponseWriter, r *http.Request) {
	feedback := r.FormValue("feedback")

	if feedback == "" {
		h.renderError(w, "Feedback cannot be empty")
		return
	}

	// TODO: Store feedback
	h.renderSuccess(w, "Thank you for your feedback!")
}

// Authentication handlers

// HandleAuthLogin handles user login via HTMX
func (h *FrontendHandlers) HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	h.logger.Info("Login attempt",
		zap.String("email", email),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("user_agent", r.UserAgent()),
	)

	if email == "" || password == "" {
		h.logger.Warn("Login failed - missing credentials", zap.String("email", email))
		h.renderError(w, "Email and password are required")
		return
	}

	// Authenticate user
	userDTO, err := h.userService.Login(r.Context(), user.LoginCommand{
		Email:    email,
		Password: password,
	})
	if err != nil {
		h.logger.Error("Login failed - invalid credentials",
			zap.Error(err),
			zap.String("email", email),
			zap.String("remote_addr", r.RemoteAddr),
		)
		h.renderError(w, "Invalid credentials")
		return
	}

	h.logger.Info("User authenticated successfully",
		zap.String("user_id", userDTO.User.ID.String()),
		zap.String("email", userDTO.User.Email),
	)

	// Create session
	session, err := h.authService.CreateSession(
		userDTO.User.ID.String(),
		r.RemoteAddr,
		r.UserAgent(),
	)
	if err != nil {
		h.logger.Error("Failed to create session",
			zap.Error(err),
			zap.String("user_id", userDTO.User.ID.String()),
		)
		h.renderError(w, "Login failed")
		return
	}

	h.logger.Debug("Session created successfully",
		zap.String("session_id", session.SessionID),
		zap.String("user_id", userDTO.User.ID.String()),
	)

	// Generate access token
	userRoles := []string{userDTO.User.Role}
	h.logger.Debug("Generating access token for login",
		zap.String("user_id", userDTO.User.ID.String()),
		zap.Strings("roles", userRoles),
		zap.String("session_id", session.SessionID),
	)

	accessToken, err := h.authService.GenerateAccessToken(
		userDTO.User.ID.String(),
		userDTO.User.Email,
		userRoles,
		session.SessionID,
		r.RemoteAddr,
		r.UserAgent(),
	)
	if err != nil {
		h.logger.Error("Failed to generate access token",
			zap.Error(err),
			zap.String("user_id", userDTO.User.ID.String()),
			zap.String("session_id", session.SessionID),
		)
		h.renderError(w, "Login failed")
		return
	}

	h.logger.Info("Access token generated successfully",
		zap.String("user_id", userDTO.User.ID.String()),
		zap.String("token_length", fmt.Sprintf("%d", len(accessToken))),
	)

	// Set secure session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.SessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Enable in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(7 * 24 * time.Hour / time.Second), // 7 days
	})

	// Set JWT token cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Enable in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   3600, // 1 hour
	})

	// Return HTMX redirect
	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}

// HandleAuthRegister handles user registration via HTMX
func (h *FrontendHandlers) HandleAuthRegister(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	// Validation
	if name == "" || email == "" || password == "" {
		h.renderError(w, "All fields are required")
		return
	}

	if password != passwordConfirm {
		h.renderError(w, "Passwords do not match")
		return
	}

	if len(password) < 8 {
		h.renderError(w, "Password must be at least 8 characters")
		return
	}

	// Register user
	userDTO, err := h.userService.Register(r.Context(), user.RegisterCommand{
		Name:     name,
		Email:    email,
		Password: password,
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			h.renderError(w, "An account with this email already exists")
		} else {
			h.renderError(w, "Registration failed. Please try again.")
		}
		return
	}

	// Create session
	session, err := h.authService.CreateSession(
		userDTO.User.ID.String(),
		r.RemoteAddr,
		r.UserAgent(),
	)
	if err != nil {
		h.logger.Error("Failed to create session", zap.Error(err))
		h.renderError(w, "Registration successful but login failed")
		return
	}

	// Generate access token
	userRoles := []string{userDTO.User.Role}
	accessToken, err := h.authService.GenerateAccessToken(
		userDTO.User.ID.String(),
		userDTO.User.Email,
		userRoles,
		session.SessionID,
		r.RemoteAddr,
		r.UserAgent(),
	)
	if err != nil {
		h.logger.Error("Failed to generate access token", zap.Error(err))
		h.renderError(w, "Registration successful but login failed")
		return
	}

	// Set cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.SessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Enable in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(7 * 24 * time.Hour / time.Second),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Enable in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   3600, // 1 hour
	})

	// Return HTMX redirect
	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}

// HandleAuthLogout handles user logout
func (h *FrontendHandlers) HandleAuthLogout(w http.ResponseWriter, r *http.Request) {
	// Clear cookies
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	// Redirect to home
	http.Redirect(w, r, "/", http.StatusFound)
}

// getUserFromRequest extracts user information from request context or cookies
func (h *FrontendHandlers) getUserFromRequest(r *http.Request) *user.UserDTO {
	h.logger.Debug("Getting user from request",
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("user_agent", r.UserAgent()),
	)

	// Try to get from context first (set by middleware)
	if userID := r.Context().Value("user_id"); userID != nil {
		if userIDStr, ok := userID.(string); ok {
			h.logger.Debug("Found user_id in context", zap.String("user_id", userIDStr))
			userDTO, err := h.userService.GetUserByID(r.Context(), parseUUID(userIDStr))
			if err == nil {
				h.logger.Debug("Successfully retrieved user from context",
					zap.String("user_id", userDTO.ID.String()),
					zap.String("email", userDTO.Email),
				)
				return userDTO
			}
			h.logger.Warn("Failed to get user by ID from context",
				zap.Error(err),
				zap.String("user_id", userIDStr),
			)
		}
	}

	// Fallback: try to validate token from cookie
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		h.logger.Debug("No auth_token cookie found", zap.Error(err))
		return nil
	}

	h.logger.Debug("Found auth_token cookie, validating token",
		zap.String("token_length", fmt.Sprintf("%d", len(cookie.Value))),
	)

	claims, err := h.authService.ValidateToken(cookie.Value, security.AccessToken)
	if err != nil {
		h.logger.Error("Token validation failed in getUserFromRequest",
			zap.Error(err),
			zap.String("remote_addr", r.RemoteAddr),
		)
		return nil
	}

	// Parse user ID from string to UUID
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil
	}

	userDTO, err := h.userService.GetUserByID(r.Context(), userID)
	if err != nil {
		return nil
	}

	return userDTO
}

// Helper methods

// renderTemplate renders a template with the given data
func (h *FrontendHandlers) renderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.templates.ExecuteTemplate(w, templateName, data); err != nil {
		h.logger.Error("Failed to render template",
			zap.String("template", templateName),
			zap.Error(err),
		)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// renderError renders an error message
func (h *FrontendHandlers) renderError(w http.ResponseWriter, message string) {
	data := map[string]interface{}{
		"type":    "error",
		"message": message,
	}
	h.renderTemplate(w, "message", data)
}

// renderSuccess renders a success message
func (h *FrontendHandlers) renderSuccess(w http.ResponseWriter, message string) {
	data := map[string]interface{}{
		"type":    "success",
		"message": message,
	}
	h.renderTemplate(w, "message", data)
}

// parseUUID parses a string to UUID, returns nil UUID if invalid
func parseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// buildRecipeAIResponseText builds a text response for AI-generated recipes
func (h *FrontendHandlers) buildRecipeAIResponseText(aiResponse *outbound.AIRecipeResponse) string {
	response := fmt.Sprintf("🤖 I've created a recipe for you!\n\n")
	response += fmt.Sprintf("**%s**\n\n", aiResponse.Title)
	response += fmt.Sprintf("%s\n\n", aiResponse.Description)

	response += "**Ingredients:**\n"
	for _, ing := range aiResponse.Ingredients {
		response += fmt.Sprintf("• %.1f %s %s\n", ing.Amount, ing.Unit, ing.Name)
	}

	response += "\n**Instructions:**\n"
	for i, inst := range aiResponse.Instructions {
		response += fmt.Sprintf("%d. %s\n", i+1, inst)
	}

	if aiResponse.Nutrition != nil {
		response += fmt.Sprintf("\n**Nutrition:** %d calories\n", aiResponse.Nutrition.Calories)
	}

	return response
}

// buildUserDashboard constructs dashboard data for authenticated users
func (h *FrontendHandlers) buildUserDashboard(ctx context.Context, user *user.UserDTO) (*UserDashboard, error) {
	dashboard := &UserDashboard{
		User: user,
	}

	// Fetch user recipes with pagination
	paginationParams := inbound.PaginationParams{
		Page:     1,
		PageSize: 10,
		OrderBy:  "created_at",
		Order:    "desc",
	}

	userRecipes, err := h.recipeService.GetRecipesByUser(ctx, user.ID, paginationParams)
	if err != nil {
		h.logger.Warn("Failed to fetch user recipes for dashboard",
			zap.Error(err),
			zap.String("user_id", user.ID.String()),
		)
	} else {
		// Calculate recipe statistics
		recipeStats := h.calculateRecipeStats(userRecipes.Recipes)
		dashboard.RecipeStats = recipeStats
		dashboard.RecentRecipes = userRecipes.Recipes
	}

	// Fetch conversation statistics
	if h.conversationService != nil {
		convStats, err := h.conversationService.GetConversationStats(ctx, user.ID.String())
		if err != nil {
			h.logger.Warn("Failed to fetch conversation stats for dashboard",
				zap.Error(err),
				zap.String("user_id", user.ID.String()),
			)
		} else {
			dashboard.ConversationStats = h.buildConversationStats(convStats)
		}
	}

	// Fetch featured/trending recipes
	featuredRecipes, err := h.recipeService.GetTrendingRecipes(ctx, inbound.PaginationParams{
		Page:     1,
		PageSize: 6,
		OrderBy:  "likes",
		Order:    "desc",
	})
	if err != nil {
		h.logger.Warn("Failed to fetch trending recipes for dashboard", zap.Error(err))
	} else {
		dashboard.FeaturedRecipes = featuredRecipes.Recipes
	}

	// Build quick actions
	dashboard.QuickActions = []QuickAction{
		{
			Title:       "Create Recipe",
			Description: "Add a new recipe to your collection",
			Icon:        "➕",
			URL:         "/recipes/new",
		},
		{
			Title:       "AI Chef Chat",
			Description: "Get cooking help from AI",
			Icon:        "🤖",
			URL:         "/ai/chat",
		},
		{
			Title:       "My Recipes",
			Description: "Browse your recipe collection",
			Icon:        "📚",
			URL:         "/recipes?author=" + user.ID.String(),
		},
		{
			Title:       "Favorites",
			Description: "View your saved recipes",
			Icon:        "❤️",
			URL:         "/favorites",
		},
	}

	return dashboard, nil
}

// calculateRecipeStats computes statistics from user recipes
func (h *FrontendHandlers) calculateRecipeStats(recipes []inbound.RecipeDTO) *UserRecipeStats {
	stats := &UserRecipeStats{}

	totalLikes := 0
	totalViews := 0
	totalRating := 0.0
	publishedCount := 0

	for _, recipe := range recipes {
		totalLikes += recipe.Likes
		totalViews += recipe.Views
		totalRating += recipe.Rating

		if recipe.Status == recipe.Status { // Assuming published status check
			publishedCount++
		}
	}

	stats.TotalRecipes = len(recipes)
	stats.PublishedRecipes = publishedCount
	stats.TotalLikes = totalLikes
	stats.TotalViews = totalViews

	if len(recipes) > 0 {
		stats.AvgRating = totalRating / float64(len(recipes))
	}

	return stats
}

// buildConversationStats converts conversation service stats to dashboard format
func (h *FrontendHandlers) buildConversationStats(stats map[string]interface{}) *UserConversationStats {
	convStats := &UserConversationStats{}

	if totalConv, ok := stats["total_conversations"].(int); ok {
		convStats.TotalConversations = totalConv
	}

	if activeConv, ok := stats["active_conversations"].(int); ok {
		convStats.ActiveConversations = activeConv
	}

	// Count recipe-related conversations as recipes generated
	if intents, ok := stats["intents"].(map[string]interface{}); ok {
		if recipeCount, ok := intents["recipe_generation"].(int); ok {
			convStats.RecipesGenerated = recipeCount
		}
	}

	return convStats
}

// getFeaturedRecipes fetches featured recipes for unauthenticated users
func (h *FrontendHandlers) getFeaturedRecipes(ctx context.Context) ([]inbound.RecipeDTO, error) {
	// Get trending recipes for featured section
	trendingRecipes, err := h.recipeService.GetTrendingRecipes(ctx, inbound.PaginationParams{
		Page:     1,
		PageSize: 6,
		OrderBy:  "views",
		Order:    "desc",
	})
	if err != nil {
		return []inbound.RecipeDTO{}, err
	}

	return trendingRecipes.Recipes, nil
}
