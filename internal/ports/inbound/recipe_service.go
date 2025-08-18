// Package inbound defines the interfaces for inbound ports (primary/driving adapters)
// These are the interfaces that the application exposes to the outside world
package inbound

import (
	"context"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/google/uuid"
)

// RecipeService defines the use cases for recipe management
// This is the primary port that HTTP handlers and other driving adapters will use
type RecipeService interface {
	// Commands - operations that modify state
	CreateRecipe(ctx context.Context, cmd CreateRecipeCommand) (*RecipeDTO, error)
	UpdateRecipe(ctx context.Context, cmd UpdateRecipeCommand) (*RecipeDTO, error)
	PublishRecipe(ctx context.Context, recipeID, userID uuid.UUID) error
	ArchiveRecipe(ctx context.Context, recipeID, userID uuid.UUID) error
	DeleteRecipe(ctx context.Context, recipeID, userID uuid.UUID) error
	
	// Recipe interactions
	LikeRecipe(ctx context.Context, recipeID, userID uuid.UUID) error
	UnlikeRecipe(ctx context.Context, recipeID, userID uuid.UUID) error
	RateRecipe(ctx context.Context, cmd RateRecipeCommand) error
	
	// Queries - operations that read state
	GetRecipeByID(ctx context.Context, recipeID uuid.UUID) (*RecipeDTO, error)
	GetRecipesByUser(ctx context.Context, userID uuid.UUID, params PaginationParams) (*RecipeList, error)
	SearchRecipes(ctx context.Context, query SearchQuery) (*RecipeList, error)
	GetTrendingRecipes(ctx context.Context, params PaginationParams) (*RecipeList, error)
	GetRecommendedRecipes(ctx context.Context, userID uuid.UUID, params PaginationParams) (*RecipeList, error)
	
	// AI operations
	GenerateRecipeWithAI(ctx context.Context, cmd GenerateRecipeCommand) (*RecipeDTO, error)
	SuggestIngredientSubstitutes(ctx context.Context, ingredientID uuid.UUID) ([]IngredientDTO, error)
	AnalyzeNutrition(ctx context.Context, recipeID uuid.UUID) (*NutritionAnalysis, error)
}

// Command objects for operations

// CreateRecipeCommand contains data for creating a new recipe
type CreateRecipeCommand struct {
	Title        string
	Description  string
	AuthorID     uuid.UUID
	Ingredients  []CreateIngredientCommand
	Instructions []CreateInstructionCommand
	Cuisine      recipe.CuisineType
	Category     recipe.CategoryType
	Difficulty   recipe.DifficultyLevel
	PrepTime     int // minutes
	CookTime     int // minutes
	Servings     int
	Tags         []string
	Images       []string
}

// UpdateRecipeCommand contains data for updating a recipe
type UpdateRecipeCommand struct {
	RecipeID     uuid.UUID
	UserID       uuid.UUID
	Title        *string
	Description  *string
	Ingredients  *[]CreateIngredientCommand
	Instructions *[]CreateInstructionCommand
	Cuisine      *recipe.CuisineType
	Category     *recipe.CategoryType
	Difficulty   *recipe.DifficultyLevel
	PrepTime     *int
	CookTime     *int
	Servings     *int
	Tags         *[]string
}

// CreateIngredientCommand for adding ingredients
type CreateIngredientCommand struct {
	Name     string
	Amount   float64
	Unit     recipe.MeasurementUnit
	Optional bool
	Notes    string
}

// CreateInstructionCommand for adding instructions
type CreateInstructionCommand struct {
	Description string
	Duration    int // minutes
	Temperature *TemperatureCommand
	Images      []string
}

// TemperatureCommand for temperature settings
type TemperatureCommand struct {
	Value float64
	Unit  recipe.TemperatureUnit
}

// RateRecipeCommand for rating a recipe
type RateRecipeCommand struct {
	RecipeID uuid.UUID
	UserID   uuid.UUID
	Rating   int
	Comment  string
}

// GenerateRecipeCommand for AI recipe generation
type GenerateRecipeCommand struct {
	UserID      uuid.UUID
	Prompt      string
	Ingredients []string
	Cuisine     recipe.CuisineType
	Dietary     []string
	MaxCalories int
}

// Query objects

// SearchQuery defines search parameters
type SearchQuery struct {
	Text       string
	Cuisine    []recipe.CuisineType
	Category   []recipe.CategoryType
	Difficulty []recipe.DifficultyLevel
	MaxTime    int // total time in minutes
	Dietary    []string
	Tags       []string
	Pagination PaginationParams
}

// PaginationParams for paginated queries
type PaginationParams struct {
	Page     int
	PageSize int
	OrderBy  string
	Order    string // asc or desc
}

// Response DTOs

// RecipeDTO is the data transfer object for recipes
type RecipeDTO struct {
	ID           uuid.UUID                `json:"id"`
	Title        string                   `json:"title"`
	Description  string                   `json:"description"`
	AuthorID     uuid.UUID                `json:"author_id"`
	AuthorName   string                   `json:"author_name"`
	Ingredients  []IngredientDTO          `json:"ingredients"`
	Instructions []InstructionDTO         `json:"instructions"`
	Nutrition    *NutritionDTO            `json:"nutrition,omitempty"`
	Cuisine      recipe.CuisineType       `json:"cuisine"`
	Category     recipe.CategoryType      `json:"category"`
	Difficulty   recipe.DifficultyLevel   `json:"difficulty"`
	PrepTime     int                      `json:"prep_time"`
	CookTime     int                      `json:"cook_time"`
	TotalTime    int                      `json:"total_time"`
	Servings     int                      `json:"servings"`
	Calories     int                      `json:"calories"`
	Tags         []string                 `json:"tags"`
	Images       []ImageDTO               `json:"images"`
	Likes        int                      `json:"likes"`
	Views        int                      `json:"views"`
	Rating       float64                  `json:"rating"`
	RatingCount  int                      `json:"rating_count"`
	Status       recipe.RecipeStatus      `json:"status"`
	AIGenerated  bool                     `json:"ai_generated"`
	CreatedAt    string                   `json:"created_at"`
	UpdatedAt    string                   `json:"updated_at"`
	PublishedAt  *string                  `json:"published_at,omitempty"`
}

// IngredientDTO for ingredient data
type IngredientDTO struct {
	ID       uuid.UUID              `json:"id"`
	Name     string                 `json:"name"`
	Amount   float64                `json:"amount"`
	Unit     recipe.MeasurementUnit `json:"unit"`
	Optional bool                   `json:"optional"`
	Notes    string                 `json:"notes,omitempty"`
}

// InstructionDTO for instruction data
type InstructionDTO struct {
	StepNumber  int              `json:"step_number"`
	Description string           `json:"description"`
	Duration    int              `json:"duration,omitempty"`
	Temperature *TemperatureDTO  `json:"temperature,omitempty"`
	Images      []string         `json:"images,omitempty"`
}

// TemperatureDTO for temperature data
type TemperatureDTO struct {
	Value float64                `json:"value"`
	Unit  recipe.TemperatureUnit `json:"unit"`
}

// NutritionDTO for nutrition information
type NutritionDTO struct {
	Calories      int     `json:"calories"`
	Protein       float64 `json:"protein"`
	Carbohydrates float64 `json:"carbohydrates"`
	Fat           float64 `json:"fat"`
	Fiber         float64 `json:"fiber"`
	Sugar         float64 `json:"sugar"`
	Sodium        float64 `json:"sodium"`
	Cholesterol   float64 `json:"cholesterol"`
}

// ImageDTO for image data
type ImageDTO struct {
	ID           uuid.UUID `json:"id"`
	URL          string    `json:"url"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Caption      string    `json:"caption,omitempty"`
	IsPrimary    bool      `json:"is_primary"`
}

// RecipeList for paginated results
type RecipeList struct {
	Recipes    []RecipeDTO `json:"recipes"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// NutritionAnalysis for AI nutrition analysis
type NutritionAnalysis struct {
	RecipeID      uuid.UUID     `json:"recipe_id"`
	Nutrition     NutritionDTO  `json:"nutrition"`
	HealthScore   float64       `json:"health_score"`
	Warnings      []string      `json:"warnings,omitempty"`
	Suggestions   []string      `json:"suggestions,omitempty"`
	AnalyzedAt    string        `json:"analyzed_at"`
}