// Package recipe provides the application layer for recipe management
// This implements the use cases defined in the inbound ports
package recipe

import (
	"context"
	"fmt"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/ports/inbound"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/alchemorsel/v3/pkg/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RecipeService implements the recipe use cases
type RecipeService struct {
	recipeRepo outbound.RecipeRepository
	userRepo   outbound.UserRepository
	cache      outbound.CacheRepository
	aiService  outbound.AIService
	events     outbound.MessageBus
	logger     *zap.Logger
}

// NewRecipeService creates a new recipe service
func NewRecipeService(
	recipeRepo outbound.RecipeRepository,
	userRepo outbound.UserRepository,
	cache outbound.CacheRepository,
	aiService outbound.AIService,
	events outbound.MessageBus,
	logger *zap.Logger,
) inbound.RecipeService {
	return &RecipeService{
		recipeRepo: recipeRepo,
		userRepo:   userRepo,
		cache:      cache,
		aiService:  aiService,
		events:     events,
		logger:     logger.Named("recipe-service"),
	}
}

// CreateRecipe creates a new recipe
func (s *RecipeService) CreateRecipe(ctx context.Context, cmd inbound.CreateRecipeCommand) (*inbound.RecipeDTO, error) {
	s.logger.Info("Creating new recipe",
		zap.String("title", cmd.Title),
		zap.String("author_id", cmd.AuthorID.String()),
	)
	
	// Validate author exists
	exists, err := s.userRepo.Exists(ctx, cmd.AuthorID)
	if err != nil {
		return nil, errors.NewDatabaseError("check user existence", err)
	}
	if !exists {
		return nil, errors.NewUserNotFoundError(cmd.AuthorID.String())
	}
	
	// Create domain entity
	recipeEntity, err := recipe.NewRecipe(cmd.Title, cmd.Description, cmd.AuthorID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create recipe entity")
	}
	
	// Add ingredients
	for _, ingredientCmd := range cmd.Ingredients {
		ingredient := recipe.Ingredient{
			ID:       uuid.New(),
			Name:     ingredientCmd.Name,
			Amount:   ingredientCmd.Amount,
			Unit:     ingredientCmd.Unit,
			Optional: ingredientCmd.Optional,
			Notes:    ingredientCmd.Notes,
		}
		
		if err := recipeEntity.AddIngredient(ingredient); err != nil {
			return nil, errors.Wrap(err, "failed to add ingredient")
		}
	}
	
	// Add instructions
	for _, instructionCmd := range cmd.Instructions {
		instruction := recipe.Instruction{
			Description: instructionCmd.Description,
		}
		
		if instructionCmd.Temperature != nil {
			instruction.Temperature = &recipe.Temperature{
				Value: instructionCmd.Temperature.Value,
				Unit:  instructionCmd.Temperature.Unit,
			}
		}
		
		if err := recipeEntity.AddInstruction(instruction); err != nil {
			return nil, errors.Wrap(err, "failed to add instruction")
		}
	}
	
	// Save to repository
	if err := s.recipeRepo.Create(ctx, recipeEntity); err != nil {
		return nil, errors.NewDatabaseError("create recipe", err)
	}
	
	// Publish domain events
	for _, event := range recipeEntity.Events() {
		if err := s.publishEvent(ctx, event); err != nil {
			s.logger.Error("Failed to publish event",
				zap.String("event", event.EventName()),
				zap.Error(err),
			)
		}
	}
	
	// Convert to DTO
	dto := s.entityToDTO(recipeEntity)
	
	s.logger.Info("Recipe created successfully",
		zap.String("recipe_id", dto.ID.String()),
		zap.String("title", dto.Title),
	)
	
	return dto, nil
}

// UpdateRecipe updates an existing recipe
func (s *RecipeService) UpdateRecipe(ctx context.Context, cmd inbound.UpdateRecipeCommand) (*inbound.RecipeDTO, error) {
	s.logger.Info("Updating recipe",
		zap.String("recipe_id", cmd.RecipeID.String()),
		zap.String("user_id", cmd.UserID.String()),
	)
	
	// Load existing recipe
	recipeEntity, err := s.recipeRepo.FindByID(ctx, cmd.RecipeID)
	if err != nil {
		return nil, errors.NewDatabaseError("find recipe", err)
	}
	if recipeEntity == nil {
		return nil, errors.NewRecipeNotFoundError(cmd.RecipeID.String())
	}
	
	// Check authorization (simplified - would use proper authorization service)
	if recipeEntity.AuthorID() != cmd.UserID {
		return nil, errors.NewInsufficientPermissionsError("update this recipe")
	}
	
	// Apply updates
	if cmd.Title != nil {
		if err := recipeEntity.UpdateTitle(*cmd.Title); err != nil {
			return nil, errors.Wrap(err, "failed to update title")
		}
	}
	
	// Save changes
	if err := s.recipeRepo.Update(ctx, recipeEntity); err != nil {
		return nil, errors.NewDatabaseError("update recipe", err)
	}
	
	// Publish events
	for _, event := range recipeEntity.Events() {
		if err := s.publishEvent(ctx, event); err != nil {
			s.logger.Error("Failed to publish event",
				zap.String("event", event.EventName()),
				zap.Error(err),
			)
		}
	}
	
	// Invalidate cache
	s.invalidateRecipeCache(cmd.RecipeID)
	
	dto := s.entityToDTO(recipeEntity)
	
	s.logger.Info("Recipe updated successfully",
		zap.String("recipe_id", dto.ID.String()),
	)
	
	return dto, nil
}

// PublishRecipe publishes a recipe
func (s *RecipeService) PublishRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	s.logger.Info("Publishing recipe",
		zap.String("recipe_id", recipeID.String()),
		zap.String("user_id", userID.String()),
	)
	
	// Load recipe
	recipeEntity, err := s.recipeRepo.FindByID(ctx, recipeID)
	if err != nil {
		return errors.NewDatabaseError("find recipe", err)
	}
	if recipeEntity == nil {
		return errors.NewRecipeNotFoundError(recipeID.String())
	}
	
	// Check authorization
	if recipeEntity.AuthorID() != userID {
		return errors.NewInsufficientPermissionsError("publish this recipe")
	}
	
	// Publish recipe
	if err := recipeEntity.Publish(); err != nil {
		return errors.Wrap(err, "failed to publish recipe")
	}
	
	// Save changes
	if err := s.recipeRepo.Update(ctx, recipeEntity); err != nil {
		return errors.NewDatabaseError("update recipe status", err)
	}
	
	// Publish events
	for _, event := range recipeEntity.Events() {
		if err := s.publishEvent(ctx, event); err != nil {
			s.logger.Error("Failed to publish event",
				zap.String("event", event.EventName()),
				zap.Error(err),
			)
		}
	}
	
	s.logger.Info("Recipe published successfully",
		zap.String("recipe_id", recipeID.String()),
	)
	
	return nil
}

// ArchiveRecipe archives a recipe
func (s *RecipeService) ArchiveRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	s.logger.Info("Archiving recipe",
		zap.String("recipe_id", recipeID.String()),
		zap.String("user_id", userID.String()),
	)
	
	// Load recipe
	recipeEntity, err := s.recipeRepo.FindByID(ctx, recipeID)
	if err != nil {
		return errors.NewDatabaseError("find recipe", err)
	}
	if recipeEntity == nil {
		return errors.NewRecipeNotFoundError(recipeID.String())
	}
	
	// Check authorization
	if recipeEntity.AuthorID() != userID {
		return errors.NewInsufficientPermissionsError("archive this recipe")
	}
	
	// Archive recipe
	if err := recipeEntity.Archive(); err != nil {
		return errors.Wrap(err, "failed to archive recipe")
	}
	
	// Save changes
	if err := s.recipeRepo.Update(ctx, recipeEntity); err != nil {
		return errors.NewDatabaseError("update recipe status", err)
	}
	
	// Publish events
	for _, event := range recipeEntity.Events() {
		if err := s.publishEvent(ctx, event); err != nil {
			s.logger.Error("Failed to publish event",
				zap.String("event", event.EventName()),
				zap.Error(err),
			)
		}
	}
	
	s.logger.Info("Recipe archived successfully",
		zap.String("recipe_id", recipeID.String()),
	)
	
	return nil
}

// DeleteRecipe deletes a recipe
func (s *RecipeService) DeleteRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	s.logger.Info("Deleting recipe",
		zap.String("recipe_id", recipeID.String()),
		zap.String("user_id", userID.String()),
	)
	
	// Load recipe
	recipeEntity, err := s.recipeRepo.FindByID(ctx, recipeID)
	if err != nil {
		return errors.NewDatabaseError("find recipe", err)
	}
	if recipeEntity == nil {
		return errors.NewRecipeNotFoundError(recipeID.String())
	}
	
	// Check authorization
	if recipeEntity.AuthorID() != userID {
		return errors.NewInsufficientPermissionsError("delete this recipe")
	}
	
	// Soft delete
	if err := s.recipeRepo.Delete(ctx, recipeID); err != nil {
		return errors.NewDatabaseError("delete recipe", err)
	}
	
	// Invalidate cache
	s.invalidateRecipeCache(recipeID)
	
	s.logger.Info("Recipe deleted successfully",
		zap.String("recipe_id", recipeID.String()),
	)
	
	return nil
}

// LikeRecipe likes a recipe
func (s *RecipeService) LikeRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	s.logger.Info("Liking recipe",
		zap.String("recipe_id", recipeID.String()),
		zap.String("user_id", userID.String()),
	)
	
	// Load recipe
	recipeEntity, err := s.recipeRepo.FindByID(ctx, recipeID)
	if err != nil {
		return errors.NewDatabaseError("find recipe", err)
	}
	if recipeEntity == nil {
		return errors.NewRecipeNotFoundError(recipeID.String())
	}
	
	// Like recipe
	recipeEntity.Like(userID)
	
	// Save changes
	if err := s.recipeRepo.Update(ctx, recipeEntity); err != nil {
		return errors.NewDatabaseError("update recipe likes", err)
	}
	
	// Publish events
	for _, event := range recipeEntity.Events() {
		if err := s.publishEvent(ctx, event); err != nil {
			s.logger.Error("Failed to publish event",
				zap.String("event", event.EventName()),
				zap.Error(err),
			)
		}
	}
	
	return nil
}

// UnlikeRecipe unlikes a recipe
func (s *RecipeService) UnlikeRecipe(ctx context.Context, recipeID, userID uuid.UUID) error {
	// Implementation would be similar to LikeRecipe
	return nil
}

// RateRecipe rates a recipe
func (s *RecipeService) RateRecipe(ctx context.Context, cmd inbound.RateRecipeCommand) error {
	s.logger.Info("Rating recipe",
		zap.String("recipe_id", cmd.RecipeID.String()),
		zap.String("user_id", cmd.UserID.String()),
		zap.Int("rating", cmd.Rating),
	)
	
	// Load recipe
	recipeEntity, err := s.recipeRepo.FindByID(ctx, cmd.RecipeID)
	if err != nil {
		return errors.NewDatabaseError("find recipe", err)
	}
	if recipeEntity == nil {
		return errors.NewRecipeNotFoundError(cmd.RecipeID.String())
	}
	
	// Create rating
	rating := recipe.Rating{
		UserID:  cmd.UserID,
		Value:   cmd.Rating,
		Comment: cmd.Comment,
	}
	
	// Add rating
	if err := recipeEntity.AddRating(rating); err != nil {
		return errors.Wrap(err, "failed to add rating")
	}
	
	// Save changes
	if err := s.recipeRepo.Update(ctx, recipeEntity); err != nil {
		return errors.NewDatabaseError("update recipe rating", err)
	}
	
	return nil
}

// GetRecipeByID retrieves a recipe by ID
func (s *RecipeService) GetRecipeByID(ctx context.Context, recipeID uuid.UUID) (*inbound.RecipeDTO, error) {
	// Try cache first
	if cached, err := s.getCachedRecipe(ctx, recipeID); err == nil && cached != nil {
		return cached, nil
	}
	
	// Load from repository
	recipeEntity, err := s.recipeRepo.FindByID(ctx, recipeID)
	if err != nil {
		return nil, errors.NewDatabaseError("find recipe", err)
	}
	if recipeEntity == nil {
		return nil, errors.NewRecipeNotFoundError(recipeID.String())
	}
	
	dto := s.entityToDTO(recipeEntity)
	
	// Cache the result
	s.cacheRecipe(ctx, dto)
	
	return dto, nil
}

// GetRecipesByUser retrieves recipes by user
func (s *RecipeService) GetRecipesByUser(ctx context.Context, userID uuid.UUID, params inbound.PaginationParams) (*inbound.RecipeList, error) {
	// Validate user exists
	exists, err := s.userRepo.Exists(ctx, userID)
	if err != nil {
		return nil, errors.NewDatabaseError("check user existence", err)
	}
	if !exists {
		return nil, errors.NewUserNotFoundError(userID.String())
	}
	
	// Get recipes
	recipes, total, err := s.recipeRepo.FindByUserID(ctx, userID, params.Page*params.PageSize, params.PageSize)
	if err != nil {
		return nil, errors.NewDatabaseError("find user recipes", err)
	}
	
	// Convert to DTOs
	recipeDTOs := make([]inbound.RecipeDTO, len(recipes))
	for i, r := range recipes {
		recipeDTOs[i] = *s.entityToDTO(r)
	}
	
	return &inbound.RecipeList{
		Recipes:    recipeDTOs,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: (total + params.PageSize - 1) / params.PageSize,
	}, nil
}

// SearchRecipes searches recipes
func (s *RecipeService) SearchRecipes(ctx context.Context, query inbound.SearchQuery) (*inbound.RecipeList, error) {
	// Convert to repository search criteria
	criteria := outbound.SearchCriteria{
		Query:      query.Text,
		Cuisines:   query.Cuisine,
		Categories: query.Category,
		Difficulty: query.Difficulty,
		MaxTime:    &query.MaxTime,
		Tags:       query.Tags,
		Offset:     query.Pagination.Page * query.Pagination.PageSize,
		Limit:      query.Pagination.PageSize,
		OrderBy:    query.Pagination.OrderBy,
		OrderDir:   query.Pagination.Order,
	}
	
	// Search recipes
	recipes, total, err := s.recipeRepo.Search(ctx, criteria)
	if err != nil {
		return nil, errors.NewDatabaseError("search recipes", err)
	}
	
	// Convert to DTOs
	recipeDTOs := make([]inbound.RecipeDTO, len(recipes))
	for i, r := range recipes {
		recipeDTOs[i] = *s.entityToDTO(r)
	}
	
	return &inbound.RecipeList{
		Recipes:    recipeDTOs,
		Total:      total,
		Page:       query.Pagination.Page,
		PageSize:   query.Pagination.PageSize,
		TotalPages: (total + query.Pagination.PageSize - 1) / query.Pagination.PageSize,
	}, nil
}

// GetTrendingRecipes retrieves trending recipes
func (s *RecipeService) GetTrendingRecipes(ctx context.Context, params inbound.PaginationParams) (*inbound.RecipeList, error) {
	// This would implement trending logic
	// For now, just return published recipes
	recipes, total, err := s.recipeRepo.FindPublished(ctx, params.Page*params.PageSize, params.PageSize)
	if err != nil {
		return nil, errors.NewDatabaseError("find trending recipes", err)
	}
	
	// Convert to DTOs
	recipeDTOs := make([]inbound.RecipeDTO, len(recipes))
	for i, r := range recipes {
		recipeDTOs[i] = *s.entityToDTO(r)
	}
	
	return &inbound.RecipeList{
		Recipes:    recipeDTOs,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: (total + params.PageSize - 1) / params.PageSize,
	}, nil
}

// GetRecommendedRecipes retrieves recommended recipes for a user
func (s *RecipeService) GetRecommendedRecipes(ctx context.Context, userID uuid.UUID, params inbound.PaginationParams) (*inbound.RecipeList, error) {
	// This would implement recommendation logic
	// For now, just return trending recipes
	return s.GetTrendingRecipes(ctx, params)
}

// AI Operations

// GenerateRecipeWithAI generates a recipe using AI
func (s *RecipeService) GenerateRecipeWithAI(ctx context.Context, cmd inbound.GenerateRecipeCommand) (*inbound.RecipeDTO, error) {
	s.logger.Info("Generating recipe with AI",
		zap.String("user_id", cmd.UserID.String()),
		zap.String("prompt", cmd.Prompt),
	)
	
	// Validate user exists
	exists, err := s.userRepo.Exists(ctx, cmd.UserID)
	if err != nil {
		return nil, errors.NewDatabaseError("check user existence", err)
	}
	if !exists {
		return nil, errors.NewUserNotFoundError(cmd.UserID.String())
	}
	
	// Generate recipe with AI
	constraints := outbound.AIConstraints{
		MaxCalories: cmd.MaxCalories,
		Dietary:     cmd.Dietary,
		Cuisine:     string(cmd.Cuisine),
	}
	
	aiResponse, err := s.aiService.GenerateRecipe(ctx, cmd.Prompt, constraints)
	if err != nil {
		return nil, errors.NewExternalServiceError("AI service", err)
	}
	
	// Create recipe entity from AI response
	recipeEntity, err := recipe.NewRecipe(aiResponse.Title, aiResponse.Description, cmd.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create AI recipe entity")
	}
	
	// Add AI-generated content
	// This would set AI-specific fields
	
	// Save to repository
	if err := s.recipeRepo.Create(ctx, recipeEntity); err != nil {
		return nil, errors.NewDatabaseError("create AI recipe", err)
	}
	
	dto := s.entityToDTO(recipeEntity)
	
	s.logger.Info("AI recipe generated successfully",
		zap.String("recipe_id", dto.ID.String()),
		zap.String("title", dto.Title),
	)
	
	return dto, nil
}

// SuggestIngredientSubstitutes suggests ingredient substitutes
func (s *RecipeService) SuggestIngredientSubstitutes(ctx context.Context, ingredientID uuid.UUID) ([]inbound.IngredientDTO, error) {
	// This would implement ingredient substitution logic
	return []inbound.IngredientDTO{}, nil
}

// AnalyzeNutrition analyzes recipe nutrition
func (s *RecipeService) AnalyzeNutrition(ctx context.Context, recipeID uuid.UUID) (*inbound.NutritionAnalysis, error) {
	// This would implement nutrition analysis
	return &inbound.NutritionAnalysis{}, nil
}

// Helper methods

// entityToDTO converts domain entity to DTO
func (s *RecipeService) entityToDTO(entity *recipe.Recipe) *inbound.RecipeDTO {
	return &inbound.RecipeDTO{
		ID:          entity.ID(),
		Title:       entity.Title(),
		// Map other fields...
		// This would be a comprehensive mapping
	}
}

// publishEvent publishes a domain event
func (s *RecipeService) publishEvent(ctx context.Context, event interface{}) error {
	// Convert event to message and publish
	// This would serialize the event and publish to message bus
	return nil
}

// Cache operations

// getCachedRecipe retrieves a recipe from cache
func (s *RecipeService) getCachedRecipe(ctx context.Context, recipeID uuid.UUID) (*inbound.RecipeDTO, error) {
	key := fmt.Sprintf("recipe:%s", recipeID.String())
	_, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	// Deserialize and return
	return nil, nil
}

// cacheRecipe caches a recipe
func (s *RecipeService) cacheRecipe(ctx context.Context, recipe *inbound.RecipeDTO) {
	key := fmt.Sprintf("recipe:%s", recipe.ID.String())
	// Serialize and cache
	s.cache.Set(ctx, key, []byte{}, 3600) // 1 hour
}

// invalidateRecipeCache invalidates recipe cache
func (s *RecipeService) invalidateRecipeCache(recipeID uuid.UUID) {
	key := fmt.Sprintf("recipe:%s", recipeID.String())
	s.cache.Delete(context.Background(), key)
}