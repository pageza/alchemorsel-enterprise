// Package recipe contains the core domain logic for recipe management.
// This follows Domain-Driven Design principles with rich domain models.
package recipe

import (
	"time"

	"github.com/alchemorsel/v3/internal/domain/shared"
	"github.com/google/uuid"
)

// Recipe represents the core recipe entity in our domain.
// It encapsulates all business logic related to recipes.
type Recipe struct {
	// Aggregate root identifier
	id          uuid.UUID
	version     int64 // Optimistic locking
	
	// Basic attributes
	title       string
	description string
	authorID    uuid.UUID
	
	// Recipe details
	ingredients    []Ingredient
	instructions   []Instruction
	nutritionInfo  *NutritionInfo
	
	// Categorization
	cuisine     CuisineType
	category    CategoryType
	difficulty  DifficultyLevel
	tags        []string
	
	// Timing
	prepTime    time.Duration
	cookTime    time.Duration
	totalTime   time.Duration
	
	// Metrics
	servings    int
	calories    int
	
	// AI-generated content
	aiGenerated bool
	aiPrompt    string
	aiModel     string
	
	// Social features
	likes       int
	views       int
	ratings     []Rating
	averageRating float64
	
	// Media
	images      []Image
	videos      []Video
	
	// Metadata
	status      RecipeStatus
	publishedAt *time.Time
	createdAt   time.Time
	updatedAt   time.Time
	deletedAt   *time.Time
	
	// Domain events to be dispatched
	events []shared.DomainEvent
}

// NewRecipe creates a new Recipe with validation
func NewRecipe(title, description string, authorID uuid.UUID) (*Recipe, error) {
	if err := validateTitle(title); err != nil {
		return nil, err
	}
	
	if err := validateDescription(description); err != nil {
		return nil, err
	}
	
	now := time.Now()
	recipe := &Recipe{
		id:          uuid.New(),
		version:     1,
		title:       title,
		description: description,
		authorID:    authorID,
		status:      RecipeStatusDraft,
		createdAt:   now,
		updatedAt:   now,
		events:      []shared.DomainEvent{},
	}
	
	// Raise domain event
	recipe.addEvent(RecipeCreatedEvent{
		RecipeID:  recipe.id,
		AuthorID:  authorID,
		Title:     title,
		CreatedAt: now,
	})
	
	return recipe, nil
}

// ID returns the recipe's unique identifier
func (r *Recipe) ID() uuid.UUID {
	return r.id
}

// Title returns the recipe's title
func (r *Recipe) Title() string {
	return r.title
}

// Description returns the recipe's description
func (r *Recipe) Description() string {
	return r.description
}

// AuthorID returns the recipe's author ID
func (r *Recipe) AuthorID() uuid.UUID {
	return r.authorID
}

// Version returns the recipe's version
func (r *Recipe) Version() int64 {
	return r.version
}

// Ingredients returns the recipe's ingredients
func (r *Recipe) Ingredients() []Ingredient {
	return r.ingredients
}

// Instructions returns the recipe's instructions
func (r *Recipe) Instructions() []Instruction {
	return r.instructions
}

// NutritionInfo returns the recipe's nutrition information
func (r *Recipe) NutritionInfo() *NutritionInfo {
	return r.nutritionInfo
}

// Cuisine returns the recipe's cuisine type
func (r *Recipe) Cuisine() CuisineType {
	return r.cuisine
}

// Category returns the recipe's category
func (r *Recipe) Category() CategoryType {
	return r.category
}

// Difficulty returns the recipe's difficulty level
func (r *Recipe) Difficulty() DifficultyLevel {
	return r.difficulty
}

// Tags returns the recipe's tags
func (r *Recipe) Tags() []string {
	return r.tags
}

// PrepTime returns the preparation time
func (r *Recipe) PrepTime() time.Duration {
	return r.prepTime
}

// CookTime returns the cooking time
func (r *Recipe) CookTime() time.Duration {
	return r.cookTime
}

// TotalTime returns the total time
func (r *Recipe) TotalTime() time.Duration {
	return r.totalTime
}

// Servings returns the number of servings
func (r *Recipe) Servings() int {
	return r.servings
}

// Calories returns the calorie count
func (r *Recipe) Calories() int {
	return r.calories
}

// IsAIGenerated returns whether the recipe was AI generated
func (r *Recipe) IsAIGenerated() bool {
	return r.aiGenerated
}

// AIPrompt returns the AI prompt used
func (r *Recipe) AIPrompt() string {
	return r.aiPrompt
}

// AIModel returns the AI model used
func (r *Recipe) AIModel() string {
	return r.aiModel
}

// Likes returns the number of likes
func (r *Recipe) Likes() int {
	return r.likes
}

// Views returns the number of views
func (r *Recipe) Views() int {
	return r.views
}

// Ratings returns the recipe ratings
func (r *Recipe) Ratings() []Rating {
	return r.ratings
}

// AverageRating returns the average rating
func (r *Recipe) AverageRating() float64 {
	return r.averageRating
}

// Images returns the recipe images
func (r *Recipe) Images() []Image {
	return r.images
}

// Videos returns the recipe videos
func (r *Recipe) Videos() []Video {
	return r.videos
}

// Status returns the recipe status
func (r *Recipe) Status() RecipeStatus {
	return r.status
}

// PublishedAt returns when the recipe was published
func (r *Recipe) PublishedAt() *time.Time {
	return r.publishedAt
}

// CreatedAt returns when the recipe was created
func (r *Recipe) CreatedAt() time.Time {
	return r.createdAt
}

// UpdatedAt returns when the recipe was last updated
func (r *Recipe) UpdatedAt() time.Time {
	return r.updatedAt
}

// DeletedAt returns when the recipe was deleted
func (r *Recipe) DeletedAt() *time.Time {
	return r.deletedAt
}

// UpdateTitle updates the recipe title with validation
func (r *Recipe) UpdateTitle(title string) error {
	if err := validateTitle(title); err != nil {
		return err
	}
	
	oldTitle := r.title
	r.title = title
	r.updatedAt = time.Now()
	
	r.addEvent(RecipeTitleUpdatedEvent{
		RecipeID: r.id,
		OldTitle: oldTitle,
		NewTitle: title,
		UpdatedAt: r.updatedAt,
	})
	
	return nil
}

// AddIngredient adds a new ingredient to the recipe
func (r *Recipe) AddIngredient(ingredient Ingredient) error {
	if err := ingredient.Validate(); err != nil {
		return err
	}
	
	r.ingredients = append(r.ingredients, ingredient)
	r.updatedAt = time.Now()
	
	r.addEvent(IngredientAddedEvent{
		RecipeID:     r.id,
		IngredientID: ingredient.ID,
		AddedAt:      r.updatedAt,
	})
	
	return nil
}

// AddInstruction adds a new instruction step
func (r *Recipe) AddInstruction(instruction Instruction) error {
	if err := instruction.Validate(); err != nil {
		return err
	}
	
	instruction.StepNumber = len(r.instructions) + 1
	r.instructions = append(r.instructions, instruction)
	r.updatedAt = time.Now()
	
	return nil
}

// Publish publishes the recipe making it publicly visible
func (r *Recipe) Publish() error {
	if r.status != RecipeStatusDraft {
		return ErrInvalidStatusTransition
	}
	
	if err := r.validateForPublishing(); err != nil {
		return err
	}
	
	now := time.Now()
	r.status = RecipeStatusPublished
	r.publishedAt = &now
	r.updatedAt = now
	
	r.addEvent(RecipePublishedEvent{
		RecipeID:    r.id,
		PublishedAt: now,
	})
	
	return nil
}

// Archive archives the recipe
func (r *Recipe) Archive() error {
	if r.status != RecipeStatusPublished {
		return ErrInvalidStatusTransition
	}
	
	r.status = RecipeStatusArchived
	r.updatedAt = time.Now()
	
	r.addEvent(RecipeArchivedEvent{
		RecipeID:   r.id,
		ArchivedAt: r.updatedAt,
	})
	
	return nil
}

// Like increments the like count
func (r *Recipe) Like(userID uuid.UUID) {
	r.likes++
	r.addEvent(RecipeLikedEvent{
		RecipeID: r.id,
		UserID:   userID,
		LikedAt:  time.Now(),
	})
}

// AddRating adds a user rating and recalculates average
func (r *Recipe) AddRating(rating Rating) error {
	if err := rating.Validate(); err != nil {
		return err
	}
	
	r.ratings = append(r.ratings, rating)
	r.calculateAverageRating()
	
	r.addEvent(RecipeRatedEvent{
		RecipeID: r.id,
		UserID:   rating.UserID,
		Rating:   rating.Value,
		RatedAt:  time.Now(),
	})
	
	return nil
}

// calculateAverageRating recalculates the average rating
func (r *Recipe) calculateAverageRating() {
	if len(r.ratings) == 0 {
		r.averageRating = 0
		return
	}
	
	var sum float64
	for _, rating := range r.ratings {
		sum += float64(rating.Value)
	}
	r.averageRating = sum / float64(len(r.ratings))
}

// validateForPublishing ensures recipe meets publishing requirements
func (r *Recipe) validateForPublishing() error {
	if len(r.ingredients) == 0 {
		return ErrNoIngredients
	}
	
	if len(r.instructions) == 0 {
		return ErrNoInstructions
	}
	
	if r.servings <= 0 {
		return ErrInvalidServings
	}
	
	return nil
}

// addEvent adds a domain event to be dispatched
func (r *Recipe) addEvent(event shared.DomainEvent) {
	r.events = append(r.events, event)
}

// Events returns and clears pending domain events
func (r *Recipe) Events() []shared.DomainEvent {
	events := r.events
	r.events = []shared.DomainEvent{}
	return events
}

// validateTitle validates recipe title
func validateTitle(title string) error {
	if len(title) < 3 {
		return ErrTitleTooShort
	}
	if len(title) > 200 {
		return ErrTitleTooLong
	}
	return nil
}

// validateDescription validates recipe description
func validateDescription(description string) error {
	if len(description) > 2000 {
		return ErrDescriptionTooLong
	}
	return nil
}