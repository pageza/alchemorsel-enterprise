// Package gorm provides GORM-based repository implementations
package gorm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RecipeRepository implements the recipe repository interface using GORM
type RecipeRepository struct {
	db *gorm.DB
}

// NewRecipeRepository creates a new recipe repository
func NewRecipeRepository(db *gorm.DB) outbound.RecipeRepository {
	return &RecipeRepository{db: db}
}

// Create creates a new recipe
func (r *RecipeRepository) Create(ctx context.Context, recipe *recipe.Recipe) error {
	model := RecipeToModel(recipe)
	
	result := r.db.WithContext(ctx).Create(model)
	if result.Error != nil {
		return result.Error
	}
	
	return nil
}

// Update updates an existing recipe
func (r *RecipeRepository) Update(ctx context.Context, recipe *recipe.Recipe) error {
	model := RecipeToModel(recipe)
	
	result := r.db.WithContext(ctx).Save(model)
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return errors.New("recipe not found")
	}
	
	return nil
}

// Delete deletes a recipe by ID (soft delete)
func (r *RecipeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&RecipeModel{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return errors.New("recipe not found")
	}
	
	return nil
}

// FindByID finds a recipe by ID
func (r *RecipeRepository) FindByID(ctx context.Context, id uuid.UUID) (*recipe.Recipe, error) {
	var model RecipeModel
	
	result := r.db.WithContext(ctx).
		Preload("Author").
		Preload("Ratings").
		First(&model, "id = ?", id)
		
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("recipe not found")
		}
		return nil, result.Error
	}
	
	return ModelToRecipe(&model)
}

// FindByUserID finds recipes by user ID with pagination
func (r *RecipeRepository) FindByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*recipe.Recipe, int, error) {
	var models []RecipeModel
	var total int64
	
	// Count total
	countResult := r.db.WithContext(ctx).Model(&RecipeModel{}).
		Where("author_id = ?", userID).
		Count(&total)
	if countResult.Error != nil {
		return nil, 0, countResult.Error
	}
	
	// Get recipes
	result := r.db.WithContext(ctx).
		Preload("Author").
		Where("author_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&models)
		
	if result.Error != nil {
		return nil, 0, result.Error
	}
	
	recipes := make([]*recipe.Recipe, len(models))
	for i, model := range models {
		r, err := ModelToRecipe(&model)
		if err != nil {
			return nil, 0, err
		}
		recipes[i] = r
	}
	
	return recipes, int(total), nil
}

// FindPublished finds published recipes with pagination
func (r *RecipeRepository) FindPublished(ctx context.Context, offset, limit int) ([]*recipe.Recipe, int, error) {
	var models []RecipeModel
	var total int64
	
	// Count total
	countResult := r.db.WithContext(ctx).Model(&RecipeModel{}).
		Where("status = ?", "published").
		Count(&total)
	if countResult.Error != nil {
		return nil, 0, countResult.Error
	}
	
	// Get recipes
	result := r.db.WithContext(ctx).
		Preload("Author").
		Where("status = ?", "published").
		Order("published_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&models)
		
	if result.Error != nil {
		return nil, 0, result.Error
	}
	
	recipes := make([]*recipe.Recipe, len(models))
	for i, model := range models {
		r, err := ModelToRecipe(&model)
		if err != nil {
			return nil, 0, err
		}
		recipes[i] = r
	}
	
	return recipes, int(total), nil
}

// FindByStatus finds recipes by status with pagination
func (r *RecipeRepository) FindByStatus(ctx context.Context, status recipe.RecipeStatus, offset, limit int) ([]*recipe.Recipe, int, error) {
	var models []RecipeModel
	var total int64
	
	statusStr := string(status)
	
	// Count total
	countResult := r.db.WithContext(ctx).Model(&RecipeModel{}).
		Where("status = ?", statusStr).
		Count(&total)
	if countResult.Error != nil {
		return nil, 0, countResult.Error
	}
	
	// Get recipes
	result := r.db.WithContext(ctx).
		Preload("Author").
		Where("status = ?", statusStr).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&models)
		
	if result.Error != nil {
		return nil, 0, result.Error
	}
	
	recipes := make([]*recipe.Recipe, len(models))
	for i, model := range models {
		r, err := ModelToRecipe(&model)
		if err != nil {
			return nil, 0, err
		}
		recipes[i] = r
	}
	
	return recipes, int(total), nil
}

// Search searches for recipes based on criteria
func (r *RecipeRepository) Search(ctx context.Context, criteria outbound.SearchCriteria) ([]*recipe.Recipe, int, error) {
	query := r.db.WithContext(ctx).Model(&RecipeModel{}).Preload("Author")
	
	// Apply filters
	if criteria.Query != "" {
		searchTerm := "%" + strings.ToLower(criteria.Query) + "%"
		query = query.Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ?", searchTerm, searchTerm)
	}
	
	if criteria.AuthorID != nil {
		query = query.Where("author_id = ?", *criteria.AuthorID)
	}
	
	if len(criteria.Cuisines) > 0 {
		cuisines := make([]string, len(criteria.Cuisines))
		for i, c := range criteria.Cuisines {
			cuisines[i] = string(c)
		}
		query = query.Where("cuisine IN ?", cuisines)
	}
	
	if len(criteria.Categories) > 0 {
		categories := make([]string, len(criteria.Categories))
		for i, c := range criteria.Categories {
			categories[i] = string(c)
		}
		query = query.Where("category IN ?", categories)
	}
	
	if len(criteria.Difficulty) > 0 {
		difficulties := make([]string, len(criteria.Difficulty))
		for i, d := range criteria.Difficulty {
			difficulties[i] = string(d)
		}
		query = query.Where("difficulty IN ?", difficulties)
	}
	
	if criteria.MinRating != nil {
		query = query.Where("average_rating >= ?", *criteria.MinRating)
	}
	
	if criteria.MaxTime != nil {
		query = query.Where("total_time_minutes <= ?", *criteria.MaxTime)
	}
	
	// Only show published recipes for search
	query = query.Where("status = ?", "published")
	
	// Count total
	var total int64
	countQuery := query
	countResult := countQuery.Count(&total)
	if countResult.Error != nil {
		return nil, 0, countResult.Error
	}
	
	// Apply ordering
	orderBy := "created_at DESC"
	if criteria.OrderBy != "" {
		direction := "ASC"
		if criteria.OrderDir == "desc" {
			direction = "DESC"
		}
		
		switch criteria.OrderBy {
		case "title":
			orderBy = fmt.Sprintf("title %s", direction)
		case "rating":
			orderBy = fmt.Sprintf("average_rating %s", direction)
		case "likes":
			orderBy = fmt.Sprintf("likes_count %s", direction)
		case "created_at":
			orderBy = fmt.Sprintf("created_at %s", direction)
		}
	}
	
	// Get recipes
	var models []RecipeModel
	result := query.
		Order(orderBy).
		Offset(criteria.Offset).
		Limit(criteria.Limit).
		Find(&models)
		
	if result.Error != nil {
		return nil, 0, result.Error
	}
	
	recipes := make([]*recipe.Recipe, len(models))
	for i, model := range models {
		r, err := ModelToRecipe(&model)
		if err != nil {
			return nil, 0, err
		}
		recipes[i] = r
	}
	
	return recipes, int(total), nil
}

// FindTrending finds trending recipes since a given time
func (r *RecipeRepository) FindTrending(ctx context.Context, since time.Time, limit int) ([]*recipe.Recipe, error) {
	var models []RecipeModel
	
	result := r.db.WithContext(ctx).
		Preload("Author").
		Where("status = ? AND created_at >= ?", "published", since).
		Order("likes_count DESC, views_count DESC, average_rating DESC").
		Limit(limit).
		Find(&models)
		
	if result.Error != nil {
		return nil, result.Error
	}
	
	recipes := make([]*recipe.Recipe, len(models))
	for i, model := range models {
		r, err := ModelToRecipe(&model)
		if err != nil {
			return nil, err
		}
		recipes[i] = r
	}
	
	return recipes, nil
}

// FindRecommended finds recommended recipes for a user
func (r *RecipeRepository) FindRecommended(ctx context.Context, userID uuid.UUID, limit int) ([]*recipe.Recipe, error) {
	// Simple recommendation: highest rated recipes the user hasn't created
	var models []RecipeModel
	
	result := r.db.WithContext(ctx).
		Preload("Author").
		Where("status = ? AND author_id != ?", "published", userID).
		Order("average_rating DESC, likes_count DESC").
		Limit(limit).
		Find(&models)
		
	if result.Error != nil {
		return nil, result.Error
	}
	
	recipes := make([]*recipe.Recipe, len(models))
	for i, model := range models {
		r, err := ModelToRecipe(&model)
		if err != nil {
			return nil, err
		}
		recipes[i] = r
	}
	
	return recipes, nil
}

// FindByIDs finds recipes by multiple IDs
func (r *RecipeRepository) FindByIDs(ctx context.Context, ids []uuid.UUID) ([]*recipe.Recipe, error) {
	var models []RecipeModel
	
	result := r.db.WithContext(ctx).
		Preload("Author").
		Where("id IN ?", ids).
		Find(&models)
		
	if result.Error != nil {
		return nil, result.Error
	}
	
	recipes := make([]*recipe.Recipe, len(models))
	for i, model := range models {
		r, err := ModelToRecipe(&model)
		if err != nil {
			return nil, err
		}
		recipes[i] = r
	}
	
	return recipes, nil
}

// BulkCreate creates multiple recipes
func (r *RecipeRepository) BulkCreate(ctx context.Context, recipes []*recipe.Recipe) error {
	models := make([]*RecipeModel, len(recipes))
	for i, recipe := range recipes {
		models[i] = RecipeToModel(recipe)
	}
	
	result := r.db.WithContext(ctx).Create(&models)
	if result.Error != nil {
		return result.Error
	}
	
	return nil
}

// UpdateWithVersion updates a recipe with optimistic locking
func (r *RecipeRepository) UpdateWithVersion(ctx context.Context, recipe *recipe.Recipe, expectedVersion int64) error {
	model := RecipeToModel(recipe)
	
	result := r.db.WithContext(ctx).
		Model(model).
		Where("id = ? AND version = ?", model.ID, expectedVersion).
		Updates(model)
		
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return errors.New("recipe not found or version mismatch")
	}
	
	// Increment version
	r.db.WithContext(ctx).Model(model).Update("version", gorm.Expr("version + 1"))
	
	return nil
}