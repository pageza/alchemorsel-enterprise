// Package handlers provides HTTP handlers for AI API endpoints
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockAIService is a mock implementation of outbound.AIService
type MockAIService struct {
	mock.Mock
}

func (m *MockAIService) GenerateRecipe(ctx context.Context, prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	args := m.Called(ctx, prompt, constraints)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.AIRecipeResponse), args.Error(1)
}

func (m *MockAIService) SuggestIngredients(ctx context.Context, partial []string) ([]string, error) {
	args := m.Called(ctx, partial)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAIService) AnalyzeNutrition(ctx context.Context, ingredients []string) (*outbound.NutritionInfo, error) {
	args := m.Called(ctx, ingredients)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.NutritionInfo), args.Error(1)
}

func (m *MockAIService) GenerateDescription(ctx context.Context, r *recipe.Recipe) (string, error) {
	args := m.Called(ctx, r)
	return args.String(0), args.Error(1)
}

func (m *MockAIService) ClassifyRecipe(ctx context.Context, r *recipe.Recipe) (*outbound.RecipeClassification, error) {
	args := m.Called(ctx, r)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.RecipeClassification), args.Error(1)
}

// Test setup helper
func setupAIAPIHandlers(t *testing.T) (*AIAPIHandlers, *MockAIService) {
	mockAIService := &MockAIService{}
	logger := zaptest.NewLogger(t)

	handlers := NewAIAPIHandlers(mockAIService, logger)

	return handlers, mockAIService
}

// Helper to add user ID to context (simulating middleware)
func addUserIDToContext(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), "user_id", userID)
	return r.WithContext(ctx)
}

// TestNewAIAPIHandlers tests the constructor
func TestNewAIAPIHandlers(t *testing.T) {
	handlers, mockService := setupAIAPIHandlers(t)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockService, handlers.aiService)
	assert.NotNil(t, handlers.logger)
}

// TestGenerateRecipe tests the GenerateRecipe handler
func TestGenerateRecipe(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		setupMocks     func(*MockAIService)
		mockUserID     string
		expectedStatus int
		checkResponse  func(t *testing.T, response APIResponse)
	}{
		{
			name: "successful recipe generation",
			setupRequest: func() *http.Request {
				reqBody := GenerateRecipeRequest{
					Prompt:      "Make me a chocolate cake",
					MaxCalories: 500,
					Dietary:     []string{"vegetarian"},
					Cuisine:     "French",
					ServingSize: 4,
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/generate-recipe", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks: func(m *MockAIService) {
				constraints := outbound.AIConstraints{
					MaxCalories: 500,
					Dietary:     []string{"vegetarian"},
					Cuisine:     "French",
					ServingSize: 4,
				}
				aiResponse := &outbound.AIRecipeResponse{
					Title:       "French Chocolate Cake",
					Description: "A delicious French chocolate cake",
					Ingredients: []outbound.AIIngredient{
						{Name: "Chocolate", Amount: 200, Unit: "g"},
						{Name: "Flour", Amount: 150, Unit: "g"},
					},
					Instructions: []string{"Mix ingredients", "Bake for 30 minutes"},
					Nutrition:    &outbound.NutritionInfo{Calories: 450, Protein: 8.5},
					Tags:         []string{"dessert", "chocolate"},
					Confidence:   0.95,
				}
				m.On("GenerateRecipe", mock.Anything, "Make me a chocolate cake", constraints).Return(aiResponse, nil)
			},
			mockUserID:     "user-123",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe generated successfully", response.Message)
				assert.NotNil(t, response.Data)

				// The response data will be serialized as a map, not the original struct
				// due to JSON serialization through the API response
				data := response.Data
				assert.NotNil(t, data)
			},
		},
		{
			name: "missing prompt",
			setupRequest: func() *http.Request {
				reqBody := GenerateRecipeRequest{
					Prompt:      "",
					MaxCalories: 500,
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/generate-recipe", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks:     func(m *MockAIService) {},
			mockUserID:     "user-123",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "Prompt is required", response.Error)
			},
		},
		{
			name: "invalid JSON",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("POST", "/api/v1/ai/generate-recipe", bytes.NewBuffer([]byte("invalid json")))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks:     func(m *MockAIService) {},
			mockUserID:     "user-123",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "Invalid JSON payload", response.Error)
			},
		},
		{
			name: "AI service error",
			setupRequest: func() *http.Request {
				reqBody := GenerateRecipeRequest{
					Prompt: "Make me a cake",
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/generate-recipe", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks: func(m *MockAIService) {
				m.On("GenerateRecipe", mock.Anything, "Make me a cake", mock.AnythingOfType("outbound.AIConstraints")).Return(nil, errors.New("AI service unavailable"))
			},
			mockUserID:     "user-123",
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "Failed to generate recipe", response.Error)
			},
		},
		{
			name: "unauthenticated request",
			setupRequest: func() *http.Request {
				reqBody := GenerateRecipeRequest{
					Prompt: "Make me a cake",
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/generate-recipe", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks:     func(m *MockAIService) {},
			mockUserID:     "", // No user ID
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "User not authenticated", response.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers, mockService := setupAIAPIHandlers(t)
			tt.setupMocks(mockService)

			req := tt.setupRequest()
			if tt.mockUserID != "" {
				req = addUserIDToContext(req, tt.mockUserID)
			}
			w := httptest.NewRecorder()

			handlers.GenerateRecipe(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.checkResponse(t, response)
			mockService.AssertExpectations(t)
		})
	}
}

// TestSuggestIngredients tests the SuggestIngredients handler
func TestSuggestIngredients(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		setupMocks     func(*MockAIService)
		mockUserID     string
		expectedStatus int
		checkResponse  func(t *testing.T, response APIResponse)
	}{
		{
			name: "successful ingredient suggestions",
			setupRequest: func() *http.Request {
				reqBody := SuggestIngredientsRequest{
					Partial: []string{"chicken", "garlic"},
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/suggest-ingredients", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks: func(m *MockAIService) {
				suggestions := []string{"chicken breast", "chicken thigh", "garlic powder", "fresh garlic"}
				m.On("SuggestIngredients", mock.Anything, []string{"chicken", "garlic"}).Return(suggestions, nil)
			},
			mockUserID:     "user-123",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Ingredient suggestions generated successfully", response.Message)
				assert.NotNil(t, response.Data)

				// The response will be serialized as a map through JSON
				data := response.Data
				assert.NotNil(t, data)
			},
		},
		{
			name: "empty partial ingredients",
			setupRequest: func() *http.Request {
				reqBody := SuggestIngredientsRequest{
					Partial: []string{},
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/suggest-ingredients", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks:     func(m *MockAIService) {},
			mockUserID:     "user-123",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "At least one ingredient is required", response.Error)
			},
		},
		{
			name: "AI service error",
			setupRequest: func() *http.Request {
				reqBody := SuggestIngredientsRequest{
					Partial: []string{"tomato"},
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/suggest-ingredients", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks: func(m *MockAIService) {
				m.On("SuggestIngredients", mock.Anything, []string{"tomato"}).Return(nil, errors.New("AI service error"))
			},
			mockUserID:     "user-123",
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "Failed to suggest ingredients", response.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers, mockService := setupAIAPIHandlers(t)
			tt.setupMocks(mockService)

			req := tt.setupRequest()
			if tt.mockUserID != "" {
				req = addUserIDToContext(req, tt.mockUserID)
			}
			w := httptest.NewRecorder()

			handlers.SuggestIngredients(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.checkResponse(t, response)
			mockService.AssertExpectations(t)
		})
	}
}

// TestAnalyzeNutrition tests the AnalyzeNutrition handler
func TestAnalyzeNutrition(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		setupMocks     func(*MockAIService)
		mockUserID     string
		expectedStatus int
		checkResponse  func(t *testing.T, response APIResponse)
	}{
		{
			name: "successful nutrition analysis",
			setupRequest: func() *http.Request {
				reqBody := AnalyzeNutritionRequest{
					Ingredients: []string{"100g chicken breast", "1 cup rice", "2 tbsp olive oil"},
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/analyze-nutrition", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks: func(m *MockAIService) {
				nutritionInfo := &outbound.NutritionInfo{
					Calories: 520,
					Protein:  31.5,
					Carbs:    45.2,
					Fat:      14.8,
					Fiber:    2.1,
					Sugar:    0.5,
					Sodium:   85.3,
				}
				m.On("AnalyzeNutrition", mock.Anything, []string{"100g chicken breast", "1 cup rice", "2 tbsp olive oil"}).Return(nutritionInfo, nil)
			},
			mockUserID:     "user-123",
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Nutrition analysis completed successfully", response.Message)
				assert.NotNil(t, response.Data)

				// The response will be serialized through JSON
				data := response.Data
				assert.NotNil(t, data)
			},
		},
		{
			name: "empty ingredients list",
			setupRequest: func() *http.Request {
				reqBody := AnalyzeNutritionRequest{
					Ingredients: []string{},
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/analyze-nutrition", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks:     func(m *MockAIService) {},
			mockUserID:     "user-123",
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "At least one ingredient is required", response.Error)
			},
		},
		{
			name: "AI service error",
			setupRequest: func() *http.Request {
				reqBody := AnalyzeNutritionRequest{
					Ingredients: []string{"invalid ingredient"},
				}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/api/v1/ai/analyze-nutrition", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			setupMocks: func(m *MockAIService) {
				m.On("AnalyzeNutrition", mock.Anything, []string{"invalid ingredient"}).Return(nil, errors.New("unable to analyze ingredient"))
			},
			mockUserID:     "user-123",
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, response APIResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "Failed to analyze nutrition", response.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers, mockService := setupAIAPIHandlers(t)
			tt.setupMocks(mockService)

			req := tt.setupRequest()
			if tt.mockUserID != "" {
				req = addUserIDToContext(req, tt.mockUserID)
			}
			w := httptest.NewRecorder()

			handlers.AnalyzeNutrition(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.checkResponse(t, response)
			mockService.AssertExpectations(t)
		})
	}
}

// TestAIAPIWriteJSON tests the writeJSON helper method
func TestAIAPIWriteJSON(t *testing.T) {
	handlers, _ := setupAIAPIHandlers(t)

	t.Run("successful JSON write", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := APIResponse{
			Success: true,
			Data:    "test data",
			Message: "test message",
		}

		handlers.writeJSON(w, http.StatusOK, data)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response APIResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, "test data", response.Data)
		assert.Equal(t, "test message", response.Message)
	})
}

// TestAIAPIWriteErrorJSON tests the writeErrorJSON helper method
func TestAIAPIWriteErrorJSON(t *testing.T) {
	handlers, _ := setupAIAPIHandlers(t)

	t.Run("error JSON write", func(t *testing.T) {
		w := httptest.NewRecorder()

		handlers.writeErrorJSON(w, http.StatusBadRequest, "test error")

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response APIResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Equal(t, "test error", response.Error)
	})
}
