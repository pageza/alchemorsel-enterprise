// Package outbound defines the interfaces for outbound ports (secondary/driven adapters)
// These are the interfaces that the application uses to interact with external systems
package outbound

import (
	"context"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/google/uuid"
)

// RecipeRepository defines the interface for recipe persistence
// This follows the Repository pattern for data access abstraction
type RecipeRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, recipe *recipe.Recipe) error
	Update(ctx context.Context, recipe *recipe.Recipe) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (*recipe.Recipe, error)
	
	// Query operations
	FindByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*recipe.Recipe, int, error)
	FindPublished(ctx context.Context, offset, limit int) ([]*recipe.Recipe, int, error)
	FindByStatus(ctx context.Context, status recipe.RecipeStatus, offset, limit int) ([]*recipe.Recipe, int, error)
	
	// Search operations
	Search(ctx context.Context, criteria SearchCriteria) ([]*recipe.Recipe, int, error)
	FindTrending(ctx context.Context, since time.Time, limit int) ([]*recipe.Recipe, error)
	FindRecommended(ctx context.Context, userID uuid.UUID, limit int) ([]*recipe.Recipe, error)
	
	// Batch operations
	FindByIDs(ctx context.Context, ids []uuid.UUID) ([]*recipe.Recipe, error)
	BulkCreate(ctx context.Context, recipes []*recipe.Recipe) error
	
	// Optimistic locking
	UpdateWithVersion(ctx context.Context, recipe *recipe.Recipe, expectedVersion int64) error
}

// UserRepository defines the interface for user persistence
type UserRepository interface {
	Create(ctx context.Context, user *user.User) error
	Update(ctx context.Context, user *user.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindByID(ctx context.Context, id uuid.UUID) (*user.User, error)
	FindByEmail(ctx context.Context, email string) (*user.User, error)
	FindByUsername(ctx context.Context, username string) (*user.User, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
}

// SearchCriteria defines search parameters for recipes
type SearchCriteria struct {
	Query       string
	AuthorID    *uuid.UUID
	Cuisines    []recipe.CuisineType
	Categories  []recipe.CategoryType
	Difficulty  []recipe.DifficultyLevel
	Tags        []string
	MinRating   *float64
	MaxTime     *int
	Ingredients []string
	ExcludeIngredients []string
	Offset      int
	Limit       int
	OrderBy     string
	OrderDir    string
}

// CacheRepository defines the interface for caching operations
type CacheRepository interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	
	// Batch operations
	MGet(ctx context.Context, keys []string) (map[string][]byte, error)
	MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	
	// Counter operations
	Increment(ctx context.Context, key string) (int64, error)
	Decrement(ctx context.Context, key string) (int64, error)
	
	// Set operations
	SAdd(ctx context.Context, key string, members ...string) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SRem(ctx context.Context, key string, members ...string) error
}

// EventStore defines the interface for event sourcing
type EventStore interface {
	Append(ctx context.Context, aggregateID uuid.UUID, events []Event) error
	Load(ctx context.Context, aggregateID uuid.UUID, fromVersion int64) ([]Event, error)
	LoadSnapshot(ctx context.Context, aggregateID uuid.UUID) (*Snapshot, error)
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error
}

// Event represents a stored domain event
type Event struct {
	ID           uuid.UUID
	AggregateID  uuid.UUID
	EventType    string
	EventData    []byte
	EventVersion int64
	OccurredAt   time.Time
}

// Snapshot represents an aggregate snapshot
type Snapshot struct {
	AggregateID uuid.UUID
	Version     int64
	Data        []byte
	CreatedAt   time.Time
}

// MessageBus defines the interface for publishing messages
type MessageBus interface {
	Publish(ctx context.Context, topic string, message Message) error
	PublishBatch(ctx context.Context, topic string, messages []Message) error
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error
	Unsubscribe(ctx context.Context, topic string) error
}

// Message represents a message to be published
type Message struct {
	ID        string
	Type      string
	Payload   []byte
	Metadata  map[string]string
	Timestamp time.Time
}

// MessageHandler handles incoming messages
type MessageHandler func(ctx context.Context, message Message) error

// StorageService defines the interface for file storage
type StorageService interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)
	Download(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	GeneratePresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
	ListObjects(ctx context.Context, prefix string) ([]string, error)
}

// AIService defines the interface for AI operations
type AIService interface {
	GenerateRecipe(ctx context.Context, prompt string, constraints AIConstraints) (*AIRecipeResponse, error)
	SuggestIngredients(ctx context.Context, partial []string) ([]string, error)
	AnalyzeNutrition(ctx context.Context, ingredients []string) (*NutritionInfo, error)
	GenerateDescription(ctx context.Context, recipe *recipe.Recipe) (string, error)
	ClassifyRecipe(ctx context.Context, recipe *recipe.Recipe) (*RecipeClassification, error)
}

// AIConstraints for AI recipe generation
type AIConstraints struct {
	MaxCalories   int
	Dietary       []string
	Cuisine       string
	ServingSize   int
	CookingTime   int
	SkillLevel    string
	Equipment     []string
	AvoidIngredients []string
}

// AIRecipeResponse from AI service
type AIRecipeResponse struct {
	Title        string
	Description  string
	Ingredients  []AIIngredient
	Instructions []string
	Nutrition    *NutritionInfo
	Tags         []string
	Confidence   float64
}

// AIIngredient from AI service
type AIIngredient struct {
	Name   string
	Amount float64
	Unit   string
}

// NutritionInfo from AI analysis
type NutritionInfo struct {
	Calories      int
	Protein       float64
	Carbs         float64
	Fat           float64
	Fiber         float64
	Sugar         float64
	Sodium        float64
}

// RecipeClassification from AI
type RecipeClassification struct {
	Cuisine    string
	Category   string
	Difficulty string
	Dietary    []string
	Confidence float64
}

// EmailService defines the interface for sending emails
type EmailService interface {
	SendWelcome(ctx context.Context, to string, name string) error
	SendPasswordReset(ctx context.Context, to string, token string) error
	SendRecipePublished(ctx context.Context, to string, recipeTitle string) error
	SendNewFollower(ctx context.Context, to string, followerName string) error
	SendBulk(ctx context.Context, recipients []string, subject string, body string) error
}

// NotificationService defines the interface for push notifications
type NotificationService interface {
	SendPush(ctx context.Context, userID uuid.UUID, title, body string, data map[string]string) error
	SendToTopic(ctx context.Context, topic string, title, body string, data map[string]string) error
	SubscribeToTopic(ctx context.Context, userID uuid.UUID, topic string) error
	UnsubscribeFromTopic(ctx context.Context, userID uuid.UUID, topic string) error
}