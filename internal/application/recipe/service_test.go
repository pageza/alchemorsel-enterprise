// Package recipe provides the application layer for recipe management
package recipe

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/alchemorsel/v3/internal/ports/inbound"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockRecipeRepository is a mock implementation of RecipeRepository
type MockRecipeRepository struct {
	mock.Mock
}

func (m *MockRecipeRepository) Create(ctx context.Context, recipe *recipe.Recipe) error {
	args := m.Called(ctx, recipe)
	return args.Error(0)
}

func (m *MockRecipeRepository) Update(ctx context.Context, recipe *recipe.Recipe) error {
	args := m.Called(ctx, recipe)
	return args.Error(0)
}

func (m *MockRecipeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRecipeRepository) FindByID(ctx context.Context, id uuid.UUID) (*recipe.Recipe, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*recipe.Recipe), args.Error(1)
}

func (m *MockRecipeRepository) FindByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*recipe.Recipe, int, error) {
	args := m.Called(ctx, userID, offset, limit)
	return args.Get(0).([]*recipe.Recipe), args.Int(1), args.Error(2)
}

func (m *MockRecipeRepository) FindPublished(ctx context.Context, offset, limit int) ([]*recipe.Recipe, int, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*recipe.Recipe), args.Int(1), args.Error(2)
}

func (m *MockRecipeRepository) FindByStatus(ctx context.Context, status recipe.RecipeStatus, offset, limit int) ([]*recipe.Recipe, int, error) {
	args := m.Called(ctx, status, offset, limit)
	return args.Get(0).([]*recipe.Recipe), args.Int(1), args.Error(2)
}

func (m *MockRecipeRepository) Search(ctx context.Context, criteria outbound.SearchCriteria) ([]*recipe.Recipe, int, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).([]*recipe.Recipe), args.Int(1), args.Error(2)
}

func (m *MockRecipeRepository) FindTrending(ctx context.Context, since time.Time, limit int) ([]*recipe.Recipe, error) {
	args := m.Called(ctx, since, limit)
	return args.Get(0).([]*recipe.Recipe), args.Error(1)
}

func (m *MockRecipeRepository) FindRecommended(ctx context.Context, userID uuid.UUID, limit int) ([]*recipe.Recipe, error) {
	args := m.Called(ctx, userID, limit)
	return args.Get(0).([]*recipe.Recipe), args.Error(1)
}

func (m *MockRecipeRepository) FindByIDs(ctx context.Context, ids []uuid.UUID) ([]*recipe.Recipe, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).([]*recipe.Recipe), args.Error(1)
}

func (m *MockRecipeRepository) BulkCreate(ctx context.Context, recipes []*recipe.Recipe) error {
	args := m.Called(ctx, recipes)
	return args.Error(0)
}

func (m *MockRecipeRepository) UpdateWithVersion(ctx context.Context, recipe *recipe.Recipe, expectedVersion int64) error {
	args := m.Called(ctx, recipe, expectedVersion)
	return args.Error(0)
}

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *user.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Update(ctx context.Context, user *user.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockUserRepository) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*user.User), args.Error(1)
}

func (m *MockUserRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockCacheRepository is a mock implementation of CacheRepository
type MockCacheRepository struct {
	mock.Mock
}

func (m *MockCacheRepository) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
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

// MockAIService is a mock implementation of AIService
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
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAIService) AnalyzeNutrition(ctx context.Context, ingredients []string) (*outbound.NutritionInfo, error) {
	args := m.Called(ctx, ingredients)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.NutritionInfo), args.Error(1)
}

func (m *MockAIService) GenerateDescription(ctx context.Context, recipe *recipe.Recipe) (string, error) {
	args := m.Called(ctx, recipe)
	return args.String(0), args.Error(1)
}

func (m *MockAIService) ClassifyRecipe(ctx context.Context, recipe *recipe.Recipe) (*outbound.RecipeClassification, error) {
	args := m.Called(ctx, recipe)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*outbound.RecipeClassification), args.Error(1)
}

// MockMessageBus is a mock implementation of MessageBus
type MockMessageBus struct {
	mock.Mock
}

func (m *MockMessageBus) Publish(ctx context.Context, topic string, message outbound.Message) error {
	args := m.Called(ctx, topic, message)
	return args.Error(0)
}

func (m *MockMessageBus) PublishBatch(ctx context.Context, topic string, messages []outbound.Message) error {
	args := m.Called(ctx, topic, messages)
	return args.Error(0)
}

func (m *MockMessageBus) Subscribe(ctx context.Context, topic string, handler outbound.MessageHandler) error {
	args := m.Called(ctx, topic, handler)
	return args.Error(0)
}

func (m *MockMessageBus) Unsubscribe(ctx context.Context, topic string) error {
	args := m.Called(ctx, topic)
	return args.Error(0)
}

// TestRecipeServiceCreation tests recipe creation functionality
func TestRecipeServiceCreation(t *testing.T) {
	t.Run("CreateRecipe_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		authorID := uuid.New()
		cmd := inbound.CreateRecipeCommand{
			Title:       "Test Recipe",
			Description: "A test recipe",
			AuthorID:    authorID,
			Ingredients: []inbound.CreateIngredientCommand{
				{
					Name:   "Test Ingredient",
					Amount: 1.0,
					Unit:   "cup",
				},
			},
			Instructions: []inbound.CreateInstructionCommand{
				{
					Description: "Test instruction",
				},
			},
		}

		// Setup expectations
		mockUserRepo.On("Exists", ctx, authorID).Return(true, nil)
		mockRecipeRepo.On("Create", ctx, mock.AnythingOfType("*recipe.Recipe")).Return(nil)
		mockEvents.On("Publish", ctx, mock.Anything, mock.Anything).Return(nil).Maybe()

		// Act
		result, err := service.CreateRecipe(ctx, cmd)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, cmd.Title, result.Title)
		assert.NotEqual(t, uuid.Nil, result.ID)

		mockUserRepo.AssertExpectations(t)
		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("CreateRecipe_UserNotFound", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		authorID := uuid.New()
		cmd := inbound.CreateRecipeCommand{
			Title:       "Test Recipe",
			Description: "A test recipe",
			AuthorID:    authorID,
		}

		// Setup expectations
		mockUserRepo.On("Exists", ctx, authorID).Return(false, nil)

		// Act
		result, err := service.CreateRecipe(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "User not found")

		mockUserRepo.AssertExpectations(t)
		mockRecipeRepo.AssertNotCalled(t, "Create")
	})

	t.Run("CreateRecipe_InvalidTitle", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		authorID := uuid.New()
		cmd := inbound.CreateRecipeCommand{
			Title:       "", // Invalid - empty title
			Description: "A test recipe",
			AuthorID:    authorID,
		}

		// Setup expectations
		mockUserRepo.On("Exists", ctx, authorID).Return(true, nil)

		// Act
		result, err := service.CreateRecipe(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)

		mockUserRepo.AssertExpectations(t)
		mockRecipeRepo.AssertNotCalled(t, "Create")
	})
}

// TestRecipeServiceUpdate tests recipe update functionality
func TestRecipeServiceUpdate(t *testing.T) {
	t.Run("UpdateRecipe_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		authorID := uuid.New()
		newTitle := "Updated Title"

		// Create existing recipe
		existingRecipe, _ := recipe.NewRecipe("Original Title", "Description", authorID)

		cmd := inbound.UpdateRecipeCommand{
			RecipeID: recipeID,
			UserID:   authorID,
			Title:    &newTitle,
		}

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)
		mockRecipeRepo.On("Update", ctx, mock.AnythingOfType("*recipe.Recipe")).Return(nil)
		mockCache.On("Delete", mock.Anything, mock.Anything).Return(nil)
		mockEvents.On("Publish", ctx, mock.Anything, mock.Anything).Return(nil).Maybe()

		// Act
		result, err := service.UpdateRecipe(ctx, cmd)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)

		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("UpdateRecipe_Unauthorized", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		authorID := uuid.New()
		differentUserID := uuid.New()
		newTitle := "Updated Title"

		// Create existing recipe
		existingRecipe, _ := recipe.NewRecipe("Original Title", "Description", authorID)

		cmd := inbound.UpdateRecipeCommand{
			RecipeID: recipeID,
			UserID:   differentUserID, // Different user trying to update
			Title:    &newTitle,
		}

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)

		// Act
		result, err := service.UpdateRecipe(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "Insufficient permissions")

		mockRecipeRepo.AssertExpectations(t)
		mockRecipeRepo.AssertNotCalled(t, "Update")
	})

	t.Run("UpdateRecipe_NotFound", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		userID := uuid.New()
		newTitle := "Updated Title"

		cmd := inbound.UpdateRecipeCommand{
			RecipeID: recipeID,
			UserID:   userID,
			Title:    &newTitle,
		}

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(nil, nil)

		// Act
		result, err := service.UpdateRecipe(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "Recipe not found")

		mockRecipeRepo.AssertExpectations(t)
	})
}

// TestRecipeServicePublishing tests recipe publishing functionality
func TestRecipeServicePublishing(t *testing.T) {
	t.Run("PublishRecipe_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		authorID := uuid.New()

		// Create valid recipe for publishing
		existingRecipe, _ := recipe.NewRecipe("Test Recipe", "Description", authorID)
		existingRecipe.AddIngredient(recipe.Ingredient{
			ID:     uuid.New(),
			Name:   "Test Ingredient",
			Amount: 1.0,
			Unit:   "cup",
		})
		existingRecipe.AddInstruction(recipe.Instruction{
			Description: "Test instruction",
			Duration:    5 * time.Minute,
		})
		existingRecipe.UpdateServings(4)

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)
		mockRecipeRepo.On("Update", ctx, mock.AnythingOfType("*recipe.Recipe")).Return(nil)
		mockEvents.On("Publish", ctx, mock.Anything, mock.Anything).Return(nil).Maybe()

		// Act
		err := service.PublishRecipe(ctx, recipeID, authorID)

		// Assert
		require.NoError(t, err)

		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("PublishRecipe_MissingIngredients", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		authorID := uuid.New()

		// Create recipe without ingredients
		existingRecipe, _ := recipe.NewRecipe("Test Recipe", "Description", authorID)

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)

		// Act
		err := service.PublishRecipe(ctx, recipeID, authorID)

		// Assert
		assert.Error(t, err)

		mockRecipeRepo.AssertExpectations(t)
		mockRecipeRepo.AssertNotCalled(t, "Update")
	})
}

// TestRecipeServiceSocialFeatures tests social features
func TestRecipeServiceSocialFeatures(t *testing.T) {
	t.Run("LikeRecipe_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		userID := uuid.New()
		authorID := uuid.New()

		existingRecipe, _ := recipe.NewRecipe("Test Recipe", "Description", authorID)

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)
		mockRecipeRepo.On("Update", ctx, mock.AnythingOfType("*recipe.Recipe")).Return(nil)
		mockEvents.On("Publish", ctx, mock.Anything, mock.Anything).Return(nil).Maybe()

		// Act
		err := service.LikeRecipe(ctx, recipeID, userID)

		// Assert
		require.NoError(t, err)

		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("RateRecipe_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		userID := uuid.New()
		authorID := uuid.New()

		existingRecipe, _ := recipe.NewRecipe("Test Recipe", "Description", authorID)

		cmd := inbound.RateRecipeCommand{
			RecipeID: recipeID,
			UserID:   userID,
			Rating:   5,
			Comment:  "Great recipe!",
		}

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)
		mockRecipeRepo.On("Update", ctx, mock.AnythingOfType("*recipe.Recipe")).Return(nil)

		// Act
		err := service.RateRecipe(ctx, cmd)

		// Assert
		require.NoError(t, err)

		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("RateRecipe_InvalidRating", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		userID := uuid.New()
		authorID := uuid.New()

		existingRecipe, _ := recipe.NewRecipe("Test Recipe", "Description", authorID)

		cmd := inbound.RateRecipeCommand{
			RecipeID: recipeID,
			UserID:   userID,
			Rating:   0, // Invalid rating
			Comment:  "Great recipe!",
		}

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)

		// Act
		err := service.RateRecipe(ctx, cmd)

		// Assert
		assert.Error(t, err)

		mockRecipeRepo.AssertExpectations(t)
		mockRecipeRepo.AssertNotCalled(t, "Update")
	})
}

// TestRecipeServiceRetrieval tests recipe retrieval functionality
func TestRecipeServiceRetrieval(t *testing.T) {
	t.Run("GetRecipeByID_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		authorID := uuid.New()

		existingRecipe, _ := recipe.NewRecipe("Test Recipe", "Description", authorID)

		// Setup expectations - cache miss, then fetch from repo
		cacheKey := "recipe:" + recipeID.String()
		mockCache.On("Get", ctx, cacheKey).Return(nil, assert.AnError).Once()
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)
		mockCache.On("Set", ctx, mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("time.Duration")).Return(nil)

		// Act
		result, err := service.GetRecipeByID(ctx, recipeID)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, existingRecipe.ID(), result.ID)
		assert.Equal(t, existingRecipe.Title(), result.Title)

		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("GetRecipesByUser_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		userID := uuid.New()
		params := inbound.PaginationParams{
			Page:     0,
			PageSize: 10,
		}

		// Create test recipes
		recipes := []*recipe.Recipe{}
		for i := 0; i < 3; i++ {
			r, _ := recipe.NewRecipe(fmt.Sprintf("Recipe %d", i), "Description", userID)
			recipes = append(recipes, r)
		}

		// Setup expectations
		mockUserRepo.On("Exists", ctx, userID).Return(true, nil)
		mockRecipeRepo.On("FindByUserID", ctx, userID, 0, 10).Return(recipes, 3, nil)

		// Act
		result, err := service.GetRecipesByUser(ctx, userID, params)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 3, result.Total)
		assert.Len(t, result.Recipes, 3)

		mockUserRepo.AssertExpectations(t)
		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("SearchRecipes_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		query := inbound.SearchQuery{
			Text: "pasta",
			Pagination: inbound.PaginationParams{
				Page:     0,
				PageSize: 10,
			},
		}

		// Create test recipes
		recipes := []*recipe.Recipe{}
		for i := 0; i < 2; i++ {
			r, _ := recipe.NewRecipe(fmt.Sprintf("Pasta Recipe %d", i), "Description", uuid.New())
			recipes = append(recipes, r)
		}

		// Setup expectations
		mockRecipeRepo.On("Search", ctx, mock.AnythingOfType("outbound.SearchCriteria")).Return(recipes, 2, nil)

		// Act
		result, err := service.SearchRecipes(ctx, query)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 2, result.Total)
		assert.Len(t, result.Recipes, 2)

		mockRecipeRepo.AssertExpectations(t)
	})
}

// TestRecipeServiceAI tests AI-related functionality
func TestRecipeServiceAI(t *testing.T) {
	t.Run("GenerateRecipeWithAI_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		userID := uuid.New()
		cmd := inbound.GenerateRecipeCommand{
			UserID:      userID,
			Prompt:      "Generate a pasta recipe",
			MaxCalories: 500,
			Dietary:     []string{"vegetarian"},
			Cuisine:     recipe.CuisineTypeItalian,
		}

		aiResponse := &outbound.AIRecipeResponse{
			Title:       "AI Generated Pasta",
			Description: "A delicious AI-generated pasta recipe",
			Ingredients: []outbound.AIIngredient{
				{Name: "Pasta", Amount: 200, Unit: "g"},
			},
			Instructions: []string{"Cook pasta"},
			Confidence:   0.95,
		}

		// Setup expectations
		mockUserRepo.On("Exists", ctx, userID).Return(true, nil)
		mockAI.On("GenerateRecipe", ctx, cmd.Prompt, mock.AnythingOfType("outbound.AIConstraints")).Return(aiResponse, nil)
		mockRecipeRepo.On("Create", ctx, mock.AnythingOfType("*recipe.Recipe")).Return(nil)

		// Act
		result, err := service.GenerateRecipeWithAI(ctx, cmd)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, aiResponse.Title, result.Title)

		mockUserRepo.AssertExpectations(t)
		mockAI.AssertExpectations(t)
		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("GenerateRecipeWithAI_AIServiceError", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		userID := uuid.New()
		cmd := inbound.GenerateRecipeCommand{
			UserID: userID,
			Prompt: "Generate a pasta recipe",
		}

		// Setup expectations
		mockUserRepo.On("Exists", ctx, userID).Return(true, nil)
		mockAI.On("GenerateRecipe", ctx, cmd.Prompt, mock.AnythingOfType("outbound.AIConstraints")).Return(nil, assert.AnError)

		// Act
		result, err := service.GenerateRecipeWithAI(ctx, cmd)

		// Assert
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "External service error")

		mockUserRepo.AssertExpectations(t)
		mockAI.AssertExpectations(t)
		mockRecipeRepo.AssertNotCalled(t, "Create")
	})
}

// TestRecipeServiceArchiving tests recipe archiving functionality
func TestRecipeServiceArchiving(t *testing.T) {
	t.Run("ArchiveRecipe_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		authorID := uuid.New()

		// Create published recipe that can be archived
		existingRecipe, _ := recipe.NewRecipe("Test Recipe", "Description", authorID)
		existingRecipe.AddIngredient(recipe.Ingredient{
			ID:     uuid.New(),
			Name:   "Test Ingredient",
			Amount: 1.0,
			Unit:   "cup",
		})
		existingRecipe.AddInstruction(recipe.Instruction{
			Description: "Test instruction",
			Duration:    5 * time.Minute,
		})
		existingRecipe.UpdateServings(4)
		existingRecipe.Publish()

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)
		mockRecipeRepo.On("Update", ctx, mock.AnythingOfType("*recipe.Recipe")).Return(nil)
		mockEvents.On("Publish", ctx, mock.Anything, mock.Anything).Return(nil).Maybe()

		// Act
		err := service.ArchiveRecipe(ctx, recipeID, authorID)

		// Assert
		require.NoError(t, err)

		mockRecipeRepo.AssertExpectations(t)
	})

	t.Run("DeleteRecipe_Success", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		logger := zaptest.NewLogger(t)

		mockRecipeRepo := &MockRecipeRepository{}
		mockUserRepo := &MockUserRepository{}
		mockCache := &MockCacheRepository{}
		mockAI := &MockAIService{}
		mockEvents := &MockMessageBus{}

		service := NewRecipeService(
			mockRecipeRepo,
			mockUserRepo,
			mockCache,
			mockAI,
			mockEvents,
			logger,
		)

		recipeID := uuid.New()
		authorID := uuid.New()

		existingRecipe, _ := recipe.NewRecipe("Test Recipe", "Description", authorID)

		// Setup expectations
		mockRecipeRepo.On("FindByID", ctx, recipeID).Return(existingRecipe, nil)
		mockRecipeRepo.On("Delete", ctx, recipeID).Return(nil)
		mockCache.On("Delete", mock.Anything, mock.AnythingOfType("string")).Return(nil)

		// Act
		err := service.DeleteRecipe(ctx, recipeID, authorID)

		// Assert
		require.NoError(t, err)

		mockRecipeRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})
}
