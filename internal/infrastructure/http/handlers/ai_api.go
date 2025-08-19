// Package handlers provides HTTP handlers for AI API endpoints
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/alchemorsel/v3/internal/infrastructure/http/middleware"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"go.uber.org/zap"
)

// AIAPIHandlers handles AI API requests
type AIAPIHandlers struct {
	aiService outbound.AIService
	logger    *zap.Logger
}

// NewAIAPIHandlers creates a new AI API handlers instance
func NewAIAPIHandlers(
	aiService outbound.AIService,
	logger *zap.Logger,
) *AIAPIHandlers {
	return &AIAPIHandlers{
		aiService: aiService,
		logger:    logger,
	}
}

// GenerateRecipeRequest represents AI recipe generation request
type GenerateRecipeRequest struct {
	Prompt      string   `json:"prompt" validate:"required"`
	MaxCalories int      `json:"max_calories,omitempty"`
	Dietary     []string `json:"dietary,omitempty"`
	Cuisine     string   `json:"cuisine,omitempty"`
	ServingSize int      `json:"serving_size,omitempty"`
}

// SuggestIngredientsRequest represents ingredient suggestion request
type SuggestIngredientsRequest struct {
	Partial []string `json:"partial" validate:"required"`
}

// AnalyzeNutritionRequest represents nutrition analysis request
type AnalyzeNutritionRequest struct {
	Ingredients []string `json:"ingredients" validate:"required"`
}

// GenerateRecipe handles POST /api/v1/ai/generate-recipe
func (h *AIAPIHandlers) GenerateRecipe(w http.ResponseWriter, r *http.Request) {
	// Get user info from context
	userID, exists := middleware.GetUserIDFromContext(r.Context())
	if !exists {
		h.writeErrorJSON(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req GenerateRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if req.Prompt == "" {
		h.writeErrorJSON(w, http.StatusBadRequest, "Prompt is required")
		return
	}

	h.logger.Info("AI recipe generation request",
		zap.String("user_id", userID),
		zap.String("prompt", req.Prompt),
	)

	// Build AI constraints
	constraints := outbound.AIConstraints{
		MaxCalories:   req.MaxCalories,
		Dietary:       req.Dietary,
		Cuisine:       req.Cuisine,
		ServingSize:   req.ServingSize,
	}

	// Call AI service
	aiResponse, err := h.aiService.GenerateRecipe(r.Context(), req.Prompt, constraints)
	if err != nil {
		h.logger.Error("AI recipe generation failed", 
			zap.String("user_id", userID),
			zap.Error(err),
		)
		h.writeErrorJSON(w, http.StatusInternalServerError, "Failed to generate recipe")
		return
	}

	response := APIResponse{
		Success: true,
		Data:    aiResponse,
		Message: "Recipe generated successfully",
	}

	h.writeJSON(w, http.StatusOK, response)
}

// SuggestIngredients handles POST /api/v1/ai/suggest-ingredients
func (h *AIAPIHandlers) SuggestIngredients(w http.ResponseWriter, r *http.Request) {
	// Get user info from context
	userID, exists := middleware.GetUserIDFromContext(r.Context())
	if !exists {
		h.writeErrorJSON(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req SuggestIngredientsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if len(req.Partial) == 0 {
		h.writeErrorJSON(w, http.StatusBadRequest, "At least one ingredient is required")
		return
	}

	h.logger.Info("AI ingredient suggestions request",
		zap.String("user_id", userID),
		zap.Strings("partial", req.Partial),
	)

	// Call AI service
	suggestions, err := h.aiService.SuggestIngredients(r.Context(), req.Partial)
	if err != nil {
		h.logger.Error("AI ingredient suggestions failed",
			zap.String("user_id", userID), 
			zap.Error(err),
		)
		h.writeErrorJSON(w, http.StatusInternalServerError, "Failed to suggest ingredients")
		return
	}

	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"suggestions": suggestions,
		},
		Message: "Ingredient suggestions generated successfully",
	}

	h.writeJSON(w, http.StatusOK, response)
}

// AnalyzeNutrition handles POST /api/v1/ai/analyze-nutrition
func (h *AIAPIHandlers) AnalyzeNutrition(w http.ResponseWriter, r *http.Request) {
	// Get user info from context
	userID, exists := middleware.GetUserIDFromContext(r.Context())
	if !exists {
		h.writeErrorJSON(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req AnalyzeNutritionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if len(req.Ingredients) == 0 {
		h.writeErrorJSON(w, http.StatusBadRequest, "At least one ingredient is required")
		return
	}

	h.logger.Info("AI nutrition analysis request",
		zap.String("user_id", userID),
		zap.Strings("ingredients", req.Ingredients),
	)

	// Call AI service
	nutritionInfo, err := h.aiService.AnalyzeNutrition(r.Context(), req.Ingredients)
	if err != nil {
		h.logger.Error("AI nutrition analysis failed",
			zap.String("user_id", userID),
			zap.Error(err),
		)
		h.writeErrorJSON(w, http.StatusInternalServerError, "Failed to analyze nutrition")
		return
	}

	response := APIResponse{
		Success: true,
		Data:    nutritionInfo,
		Message: "Nutrition analysis completed successfully",
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Helper methods

func (h *AIAPIHandlers) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *AIAPIHandlers) writeErrorJSON(w http.ResponseWriter, status int, message string) {
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	h.writeJSON(w, status, response)
}