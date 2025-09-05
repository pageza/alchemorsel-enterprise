// Package handlers provides HTTP handlers for REST API endpoints
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alchemorsel/v3/internal/ports/inbound"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockRecipeService is a mock implementation of RecipeService
type MockRecipeService struct {
	mock.Mock
}

func (m *MockRecipeService) CreateRecipe(ctx context.Context, cmd inbound.CreateRecipeCommand) (*inbound.RecipeDTO, error) {
	args := m.Called(ctx, cmd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.RecipeDTO), args.Error(1)
}

func (m *MockRecipeService) UpdateRecipe(ctx context.Context, cmd inbound.UpdateRecipeCommand) (*inbound.RecipeDTO, error) {
	args := m.Called(ctx, cmd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.RecipeDTO), args.Error(1)
}

func (m *MockRecipeService) PublishRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	args := m.Called(ctx, recipeID, userID)
	return args.Error(0)
}

func (m *MockRecipeService) ArchiveRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	args := m.Called(ctx, recipeID, userID)
	return args.Error(0)
}

func (m *MockRecipeService) DeleteRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	args := m.Called(ctx, recipeID, userID)
	return args.Error(0)
}

func (m *MockRecipeService) LikeRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	args := m.Called(ctx, recipeID, userID)
	return args.Error(0)
}

func (m *MockRecipeService) UnlikeRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	args := m.Called(ctx, recipeID, userID)
	return args.Error(0)
}

func (m *MockRecipeService) RateRecipe(ctx context.Context, cmd inbound.RateRecipeCommand) error {
	args := m.Called(ctx, cmd)
	return args.Error(0)
}

func (m *MockRecipeService) GetRecipeByID(ctx context.Context, recipeID uuid.UUID) (*inbound.RecipeDTO, error) {
	args := m.Called(ctx, recipeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.RecipeDTO), args.Error(1)
}

func (m *MockRecipeService) GetRecipesByUser(ctx context.Context, userID uuid.UUID, params inbound.PaginationParams) (*inbound.RecipeList, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.RecipeList), args.Error(1)
}

func (m *MockRecipeService) SearchRecipes(ctx context.Context, query inbound.SearchQuery) (*inbound.RecipeList, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.RecipeList), args.Error(1)
}

func (m *MockRecipeService) GetTrendingRecipes(ctx context.Context, params inbound.PaginationParams) (*inbound.RecipeList, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.RecipeList), args.Error(1)
}

func (m *MockRecipeService) GetRecommendedRecipes(ctx context.Context, userID uuid.UUID, params inbound.PaginationParams) (*inbound.RecipeList, error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.RecipeList), args.Error(1)
}

func (m *MockRecipeService) GenerateRecipeWithAI(ctx context.Context, cmd inbound.GenerateRecipeCommand) (*inbound.RecipeDTO, error) {
	args := m.Called(ctx, cmd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.RecipeDTO), args.Error(1)
}

func (m *MockRecipeService) SuggestIngredientSubstitutes(ctx context.Context, ingredientID uuid.UUID) ([]inbound.IngredientDTO, error) {
	args := m.Called(ctx, ingredientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]inbound.IngredientDTO), args.Error(1)
}

func (m *MockRecipeService) AnalyzeNutrition(ctx context.Context, recipeID uuid.UUID) (*inbound.NutritionAnalysis, error) {
	args := m.Called(ctx, recipeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*inbound.NutritionAnalysis), args.Error(1)
}

// Test setup helper
func setupAPIHandlers(t *testing.T) (*APIHandlers, *MockRecipeService) {
	mockRecipeService := &MockRecipeService{}
	logger := zaptest.NewLogger(t)

	handlers := NewAPIHandlers(mockRecipeService, logger)

	return handlers, mockRecipeService
}

// TestNewAPIHandlers tests the constructor
func TestNewAPIHandlers(t *testing.T) {
	mockRecipeService := &MockRecipeService{}
	logger := zaptest.NewLogger(t)

	handlers := NewAPIHandlers(mockRecipeService, logger)

	assert.NotNil(t, handlers)
	assert.Equal(t, mockRecipeService, handlers.recipeService)
	assert.Equal(t, logger, handlers.logger)
}

// TestListRecipes tests the ListRecipes handler
func TestListRecipes(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful list recipes",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipes retrieved successfully", response.Message)
				assert.NotNil(t, response.Data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v3/recipes", nil)
			w := httptest.NewRecorder()

			handlers.ListRecipes(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestCreateRecipe tests the CreateRecipe handler
func TestCreateRecipe(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful recipe creation",
			method:         "POST",
			body:           map[string]interface{}{"title": "Test Recipe", "description": "A test recipe"},
			expectedStatus: http.StatusCreated,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe created successfully", response.Message)
			},
		},
		{
			name:           "empty body",
			method:         "POST",
			body:           nil,
			expectedStatus: http.StatusCreated,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe created successfully", response.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			if tt.body != nil {
				err := json.NewEncoder(&body).Encode(tt.body)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, "/api/v3/recipes", &body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handlers.CreateRecipe(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestGetRecipe tests the GetRecipe handler
func TestGetRecipe(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful recipe retrieval",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe retrieved successfully", response.Message)
				assert.NotNil(t, response.Data)

				data, ok := response.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "1", data["id"])
				assert.Equal(t, "Sample Recipe", data["title"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v3/recipes/1", nil)
			w := httptest.NewRecorder()

			handlers.GetRecipe(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestUpdateRecipe tests the UpdateRecipe handler
func TestUpdateRecipe(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful recipe update",
			method:         "PUT",
			body:           map[string]interface{}{"title": "Updated Recipe", "description": "An updated recipe"},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe updated successfully", response.Message)
			},
		},
		{
			name:           "empty body",
			method:         "PUT",
			body:           nil,
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe updated successfully", response.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			if tt.body != nil {
				err := json.NewEncoder(&body).Encode(tt.body)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, "/api/v3/recipes/1", &body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handlers.UpdateRecipe(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestDeleteRecipe tests the DeleteRecipe handler
func TestDeleteRecipe(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful recipe deletion",
			method:         "DELETE",
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe deleted successfully", response.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v3/recipes/1", nil)
			w := httptest.NewRecorder()

			handlers.DeleteRecipe(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestLikeRecipe tests the LikeRecipe handler
func TestLikeRecipe(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful recipe like",
			method:         "POST",
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe liked successfully", response.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v3/recipes/1/like", nil)
			w := httptest.NewRecorder()

			handlers.LikeRecipe(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestRateRecipe tests the RateRecipe handler
func TestRateRecipe(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful recipe rating",
			method:         "POST",
			body:           map[string]interface{}{"rating": 5, "comment": "Great recipe!"},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe rated successfully", response.Message)
			},
		},
		{
			name:           "empty body",
			method:         "POST",
			body:           nil,
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Recipe rated successfully", response.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body bytes.Buffer
			if tt.body != nil {
				err := json.NewEncoder(&body).Encode(tt.body)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, "/api/v1/recipes/1/rating", &body)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handlers.RateRecipe(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestGetUserRecipes tests the GetUserRecipes handler
func TestGetUserRecipes(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful user recipes retrieval",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "User recipes retrieved successfully", response.Message)
				assert.NotNil(t, response.Data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/users/1/recipes", nil)
			w := httptest.NewRecorder()

			handlers.GetUserRecipes(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestGetUserFavorites tests the GetUserFavorites handler
func TestGetUserFavorites(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful user favorites retrieval",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "User favorites retrieved successfully", response.Message)
				assert.NotNil(t, response.Data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/users/1/favorites", nil)
			w := httptest.NewRecorder()

			handlers.GetUserFavorites(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestHealthCheck tests the HealthCheck handler
func TestHealthCheck(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		method         string
		expectedStatus int
		expectedBody   func(t *testing.T, response APIResponse)
	}{
		{
			name:           "successful health check",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, response APIResponse) {
				assert.True(t, response.Success)
				assert.Equal(t, "Service is healthy", response.Message)
				assert.NotNil(t, response.Data)

				data, ok := response.Data.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "healthy", data["status"])
				assert.Equal(t, "v3.0.0", data["version"])
				assert.NotNil(t, data["timestamp"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v3/health", nil)
			w := httptest.NewRecorder()

			handlers.HealthCheck(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.expectedBody(t, response)
		})
	}
}

// TestWriteJSON tests the writeJSON helper method
func TestWriteJSON(t *testing.T) {
	handlers, _ := setupAPIHandlers(t)

	tests := []struct {
		name           string
		status         int
		data           interface{}
		expectedStatus int
		expectedBody   interface{}
	}{
		{
			name:           "successful JSON write",
			status:         http.StatusOK,
			data:           APIResponse{Success: true, Message: "Test message"},
			expectedStatus: http.StatusOK,
			expectedBody:   APIResponse{Success: true, Message: "Test message"},
		},
		{
			name:           "JSON write with data",
			status:         http.StatusCreated,
			data:           APIResponse{Success: true, Data: map[string]string{"key": "value"}, Message: "Created"},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			handlers.writeJSON(w, tt.status, tt.data)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			if tt.expectedBody != nil {
				var response APIResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				expected := tt.expectedBody.(APIResponse)
				assert.Equal(t, expected.Success, response.Success)
				assert.Equal(t, expected.Message, response.Message)
			}
		})
	}
}
