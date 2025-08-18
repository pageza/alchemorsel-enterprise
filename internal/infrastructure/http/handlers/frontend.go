// Package handlers provides HTTP handlers for the HTMX frontend
package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/application/user"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"github.com/alchemorsel/v3/internal/ports/inbound"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// FrontendHandlers handles frontend HTMX requests
type FrontendHandlers struct {
	templates     *template.Template
	recipeService inbound.RecipeService
	userService   *user.UserService
	authService   *security.AuthService
	logger        *zap.Logger
}

// NewFrontendHandlers creates a new frontend handlers instance
func NewFrontendHandlers(
	templates *template.Template,
	recipeService inbound.RecipeService,
	userService *user.UserService,
	authService *security.AuthService,
	logger *zap.Logger,
) *FrontendHandlers {
	return &FrontendHandlers{
		templates:     templates,
		recipeService: recipeService,
		userService:   userService,
		authService:   authService,
		logger:        logger,
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

// HandleHome renders the home page with critical 14KB optimization
func (h *FrontendHandlers) HandleHome(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:       "Alchemorsel - AI-Powered Recipe Platform",
		Description: "Discover, create, and share recipes with AI assistance",
		Keywords:    "recipes, cooking, AI, food, ingredients",
		User:        h.getUserFromRequest(r),
		HTMX:        r.Header.Get("HX-Request") == "true",
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
			"id": "1", 
			"title": "Pasta Carbonara", 
			"description": "Classic Italian pasta dish with eggs, cheese, and pancetta",
			"cuisine": "italian",
			"difficulty": "medium",
			"cookTime": "25",
			"diet": "",
		},
		{
			"id": "2", 
			"title": "Chicken Tikka Masala", 
			"description": "Creamy Indian curry with tender marinated chicken",
			"cuisine": "indian",
			"difficulty": "hard",
			"cookTime": "45",
			"diet": "",
		},
		{
			"id": "3", 
			"title": "Vegetarian Pad Thai", 
			"description": "Thai stir-fried noodles with vegetables and tofu",
			"cuisine": "thai",
			"difficulty": "medium",
			"cookTime": "20",
			"diet": "vegetarian",
		},
		{
			"id": "4", 
			"title": "Quick Quinoa Salad", 
			"description": "Healthy and quick quinoa salad with fresh vegetables",
			"cuisine": "american",
			"difficulty": "easy",
			"cookTime": "15",
			"diet": "vegan",
		},
		{
			"id": "5", 
			"title": "French Onion Soup", 
			"description": "Classic French soup with caramelized onions and cheese",
			"cuisine": "french",
			"difficulty": "medium",
			"cookTime": "60",
			"diet": "vegetarian",
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
		"id":     id,
		"rating": rating,
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

// HandleAIChat handles AI chat messages
func (h *FrontendHandlers) HandleAIChat(w http.ResponseWriter, r *http.Request) {
	message := r.FormValue("message")
	
	if message == "" {
		h.renderError(w, "Message cannot be empty")
		return
	}

	// TODO: Process with AI service
	response := fmt.Sprintf("I understand you want help with: %s. Let me suggest some recipes!", message)

	data := map[string]interface{}{
		"message":  message,
		"response": response,
		"timestamp": time.Now(),
	}

	h.renderTemplate(w, "chat-message", data)
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
	
	if email == "" || password == "" {
		h.renderError(w, "Email and password are required")
		return
	}
	
	// Authenticate user
	authResponse, err := h.userService.Login(r.Context(), user.LoginCommand{
		Email:    email,
		Password: password,
	})
	if err != nil {
		h.renderError(w, "Invalid credentials")
		return
	}
	
	// Create session
	session, err := h.authService.CreateSession(
		authResponse.User.ID.String(),
		r.RemoteAddr,
		r.UserAgent(),
	)
	if err != nil {
		h.logger.Error("Failed to create session", zap.Error(err))
		h.renderError(w, "Login failed")
		return
	}
	
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
		Value:    authResponse.AccessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Enable in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   authResponse.ExpiresIn,
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
	authResponse, err := h.userService.Register(r.Context(), user.RegisterCommand{
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
		authResponse.User.ID.String(),
		r.RemoteAddr,
		r.UserAgent(),
	)
	if err != nil {
		h.logger.Error("Failed to create session", zap.Error(err))
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
		Value:    authResponse.AccessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Enable in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		MaxAge:   authResponse.ExpiresIn,
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
	// Try to get from context first (set by middleware)
	if userID := r.Context().Value("user_id"); userID != nil {
		if userIDStr, ok := userID.(string); ok {
			userDTO, err := h.userService.GetUserByID(r.Context(), parseUUID(userIDStr))
			if err == nil {
				return userDTO
			}
		}
	}
	
	// Fallback: try to validate token from cookie
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return nil
	}
	
	claims, err := h.userService.ValidateToken(cookie.Value)
	if err != nil {
		return nil
	}
	
	userDTO, err := h.userService.GetUserByID(r.Context(), claims.UserID)
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