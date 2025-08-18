package recipe

import (
	"time"

	"github.com/google/uuid"
)

// Domain Events - Events that occur within the recipe domain

// RecipeCreatedEvent is raised when a new recipe is created
type RecipeCreatedEvent struct {
	RecipeID  uuid.UUID
	AuthorID  uuid.UUID
	Title     string
	CreatedAt time.Time
}

func (e RecipeCreatedEvent) EventName() string {
	return "recipe.created"
}

func (e RecipeCreatedEvent) OccurredAt() time.Time {
	return e.CreatedAt
}

// RecipeTitleUpdatedEvent is raised when a recipe title is updated
type RecipeTitleUpdatedEvent struct {
	RecipeID  uuid.UUID
	OldTitle  string
	NewTitle  string
	UpdatedAt time.Time
}

func (e RecipeTitleUpdatedEvent) EventName() string {
	return "recipe.title.updated"
}

func (e RecipeTitleUpdatedEvent) OccurredAt() time.Time {
	return e.UpdatedAt
}

// RecipePublishedEvent is raised when a recipe is published
type RecipePublishedEvent struct {
	RecipeID    uuid.UUID
	PublishedAt time.Time
}

func (e RecipePublishedEvent) EventName() string {
	return "recipe.published"
}

func (e RecipePublishedEvent) OccurredAt() time.Time {
	return e.PublishedAt
}

// RecipeArchivedEvent is raised when a recipe is archived
type RecipeArchivedEvent struct {
	RecipeID   uuid.UUID
	ArchivedAt time.Time
}

func (e RecipeArchivedEvent) EventName() string {
	return "recipe.archived"
}

func (e RecipeArchivedEvent) OccurredAt() time.Time {
	return e.ArchivedAt
}

// RecipeLikedEvent is raised when a recipe is liked
type RecipeLikedEvent struct {
	RecipeID uuid.UUID
	UserID   uuid.UUID
	LikedAt  time.Time
}

func (e RecipeLikedEvent) EventName() string {
	return "recipe.liked"
}

func (e RecipeLikedEvent) OccurredAt() time.Time {
	return e.LikedAt
}

// RecipeRatedEvent is raised when a recipe is rated
type RecipeRatedEvent struct {
	RecipeID uuid.UUID
	UserID   uuid.UUID
	Rating   int
	RatedAt  time.Time
}

func (e RecipeRatedEvent) EventName() string {
	return "recipe.rated"
}

func (e RecipeRatedEvent) OccurredAt() time.Time {
	return e.RatedAt
}

// IngredientAddedEvent is raised when an ingredient is added
type IngredientAddedEvent struct {
	RecipeID     uuid.UUID
	IngredientID uuid.UUID
	AddedAt      time.Time
}

func (e IngredientAddedEvent) EventName() string {
	return "recipe.ingredient.added"
}

func (e IngredientAddedEvent) OccurredAt() time.Time {
	return e.AddedAt
}