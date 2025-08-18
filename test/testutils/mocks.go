// Package testutils provides mock implementations for testing
package testutils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockRecipeRepository provides a mock implementation of RecipeRepository
type MockRecipeRepository struct {
	mock.Mock
	recipes map[uuid.UUID]*recipe.Recipe
	mu      sync.RWMutex
}

// NewMockRecipeRepository creates a new mock recipe repository
func NewMockRecipeRepository() *MockRecipeRepository {
	return &MockRecipeRepository{
		recipes: make(map[uuid.UUID]*recipe.Recipe),
	}
}

// Save saves a recipe
func (m *MockRecipeRepository) Save(ctx context.Context, r *recipe.Recipe) error {
	args := m.Called(ctx, r)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.recipes[r.ID()] = r
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// FindByID finds a recipe by ID
func (m *MockRecipeRepository) FindByID(ctx context.Context, id uuid.UUID) (*recipe.Recipe, error) {
	args := m.Called(ctx, id)
	
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if r, exists := m.recipes[id]; exists {
		return r, nil
	}
	
	return args.Get(0).(*recipe.Recipe), args.Error(1)
}

// FindByAuthorID finds recipes by author ID
func (m *MockRecipeRepository) FindByAuthorID(ctx context.Context, authorID uuid.UUID, limit, offset int) ([]*recipe.Recipe, error) {
	args := m.Called(ctx, authorID, limit, offset)
	return args.Get(0).([]*recipe.Recipe), args.Error(1)
}

// FindPublished finds published recipes
func (m *MockRecipeRepository) FindPublished(ctx context.Context, limit, offset int) ([]*recipe.Recipe, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*recipe.Recipe), args.Error(1)
}

// Search searches for recipes
func (m *MockRecipeRepository) Search(ctx context.Context, query string, filters map[string]interface{}, limit, offset int) ([]*recipe.Recipe, error) {
	args := m.Called(ctx, query, filters, limit, offset)
	return args.Get(0).([]*recipe.Recipe), args.Error(1)
}

// Delete deletes a recipe
func (m *MockRecipeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		delete(m.recipes, id)
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// Count returns the total count of recipes
func (m *MockRecipeRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// SetupStandardMockBehavior sets up common mock behaviors
func (m *MockRecipeRepository) SetupStandardMockBehavior() {
	// Save always succeeds
	m.On("Save", mock.Anything, mock.AnythingOfType("*recipe.Recipe")).
		Return(nil)
	
	// FindByID returns recipe not found by default
	m.On("FindByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).
		Return((*recipe.Recipe)(nil), outbound.ErrRecipeNotFound)
	
	// Other methods return empty results
	m.On("FindByAuthorID", mock.Anything, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return([]*recipe.Recipe{}, nil)
	
	m.On("FindPublished", mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return([]*recipe.Recipe{}, nil)
	
	m.On("Search", mock.Anything, mock.AnythingOfType("string"), mock.Anything, mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return([]*recipe.Recipe{}, nil)
	
	m.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).
		Return(nil)
	
	m.On("Count", mock.Anything).
		Return(int64(0), nil)
}

// MockUserRepository provides a mock implementation of UserRepository
type MockUserRepository struct {
	mock.Mock
	users map[uuid.UUID]*user.User
	mu    sync.RWMutex
}

// NewMockUserRepository creates a new mock user repository
func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		users: make(map[uuid.UUID]*user.User),
	}
}

// Save saves a user
func (m *MockUserRepository) Save(ctx context.Context, u *user.User) error {
	args := m.Called(ctx, u)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.users[u.ID()] = u
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// FindByID finds a user by ID
func (m *MockUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	args := m.Called(ctx, id)
	
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if u, exists := m.users[id]; exists {
		return u, nil
	}
	
	return args.Get(0).(*user.User), args.Error(1)
}

// FindByEmail finds a user by email
func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(*user.User), args.Error(1)
}

// FindByUsername finds a user by username
func (m *MockUserRepository) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	args := m.Called(ctx, username)
	return args.Get(0).(*user.User), args.Error(1)
}

// Delete deletes a user
func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		delete(m.users, id)
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// SetupStandardMockBehavior sets up common mock behaviors
func (m *MockUserRepository) SetupStandardMockBehavior() {
	// Save always succeeds
	m.On("Save", mock.Anything, mock.AnythingOfType("*user.User")).
		Return(nil)
	
	// FindByID returns user not found by default
	m.On("FindByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).
		Return((*user.User)(nil), outbound.ErrUserNotFound)
	
	// FindByEmail returns user not found by default
	m.On("FindByEmail", mock.Anything, mock.AnythingOfType("string")).
		Return((*user.User)(nil), outbound.ErrUserNotFound)
	
	// FindByUsername returns user not found by default
	m.On("FindByUsername", mock.Anything, mock.AnythingOfType("string")).
		Return((*user.User)(nil), outbound.ErrUserNotFound)
	
	// Delete always succeeds
	m.On("Delete", mock.Anything, mock.AnythingOfType("uuid.UUID")).
		Return(nil)
}

// MockAIService provides a mock implementation of AI service
type MockAIService struct {
	mock.Mock
}

// NewMockAIService creates a new mock AI service
func NewMockAIService() *MockAIService {
	return &MockAIService{}
}

// AIRecipe represents an AI-generated recipe response
type AIRecipe struct {
	Title        string                `json:"title"`
	Description  string                `json:"description"`
	Ingredients  []recipe.Ingredient   `json:"ingredients"`
	Instructions []recipe.Instruction  `json:"instructions"`
	PrepTime     time.Duration         `json:"prep_time"`
	CookTime     time.Duration         `json:"cook_time"`
	Servings     int                   `json:"servings"`
	Cuisine      recipe.CuisineType    `json:"cuisine"`
	Difficulty   recipe.DifficultyLevel `json:"difficulty"`
}

// GenerateRecipe generates a recipe using AI
func (m *MockAIService) GenerateRecipe(ctx context.Context, prompt string) (*AIRecipe, error) {
	args := m.Called(ctx, prompt)
	return args.Get(0).(*AIRecipe), args.Error(1)
}

// AnalyzeNutrition analyzes nutritional content
func (m *MockAIService) AnalyzeNutrition(ctx context.Context, ingredients []recipe.Ingredient) (*recipe.NutritionInfo, error) {
	args := m.Called(ctx, ingredients)
	return args.Get(0).(*recipe.NutritionInfo), args.Error(1)
}

// SuggestIngredients suggests ingredients based on cuisine
func (m *MockAIService) SuggestIngredients(ctx context.Context, cuisine recipe.CuisineType, existingIngredients []recipe.Ingredient) ([]recipe.Ingredient, error) {
	args := m.Called(ctx, cuisine, existingIngredients)
	return args.Get(0).([]recipe.Ingredient), args.Error(1)
}

// SetupStandardMockBehavior sets up common mock behaviors
func (m *MockAIService) SetupStandardMockBehavior() {
	// Create standard AI-generated recipe
	standardRecipe := &AIRecipe{
		Title:       "AI-Generated Pasta Dish",
		Description: "A delicious pasta dish created by AI",
		Ingredients: []recipe.Ingredient{
			{
				ID:       uuid.New(),
				Name:     "Spaghetti",
				Quantity: 1.0,
				Unit:     "lb",
			},
			{
				ID:       uuid.New(),
				Name:     "Tomato Sauce",
				Quantity: 2.0,
				Unit:     "cups",
			},
		},
		Instructions: []recipe.Instruction{
			{
				ID:          uuid.New(),
				StepNumber:  1,
				Description: "Boil water",
				Duration:    5 * time.Minute,
			},
			{
				ID:          uuid.New(),
				StepNumber:  2,
				Description: "Cook pasta",
				Duration:    10 * time.Minute,
			},
		},
		PrepTime:   10 * time.Minute,
		CookTime:   15 * time.Minute,
		Servings:   4,
		Cuisine:    recipe.CuisineItalian,
		Difficulty: recipe.DifficultyEasy,
	}

	// Standard nutrition info
	standardNutrition := &recipe.NutritionInfo{
		Calories:        400,
		Protein:         15.0,
		Carbohydrates:   60.0,
		Fat:             8.0,
		Fiber:           5.0,
		Sugar:           10.0,
		Sodium:          800.0,
		Cholesterol:     25.0,
		ServingSize:     "1 portion",
		VitaminA:        20.0,
		VitaminC:        15.0,
		Calcium:         10.0,
		Iron:            8.0,
	}

	// Setup mock responses
	m.On("GenerateRecipe", mock.Anything, mock.AnythingOfType("string")).
		Return(standardRecipe, nil)
	
	m.On("AnalyzeNutrition", mock.Anything, mock.AnythingOfType("[]recipe.Ingredient")).
		Return(standardNutrition, nil)
	
	m.On("SuggestIngredients", mock.Anything, mock.AnythingOfType("recipe.CuisineType"), mock.AnythingOfType("[]recipe.Ingredient")).
		Return([]recipe.Ingredient{
			{
				ID:       uuid.New(),
				Name:     "Suggested Ingredient",
				Quantity: 1.0,
				Unit:     "cup",
			},
		}, nil)
}

// MockEmailService provides a mock implementation of email service
type MockEmailService struct {
	mock.Mock
	sentEmails []EmailMessage
	mu         sync.RWMutex
}

// EmailMessage represents an email message
type EmailMessage struct {
	To      string
	Subject string
	Body    string
	IsHTML  bool
}

// NewMockEmailService creates a new mock email service
func NewMockEmailService() *MockEmailService {
	return &MockEmailService{
		sentEmails: make([]EmailMessage, 0),
	}
}

// SendEmail sends an email
func (m *MockEmailService) SendEmail(ctx context.Context, to, subject, body string, isHTML bool) error {
	args := m.Called(ctx, to, subject, body, isHTML)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.sentEmails = append(m.sentEmails, EmailMessage{
			To:      to,
			Subject: subject,
			Body:    body,
			IsHTML:  isHTML,
		})
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// SendTemplateEmail sends a templated email
func (m *MockEmailService) SendTemplateEmail(ctx context.Context, to, template string, data map[string]interface{}) error {
	args := m.Called(ctx, to, template, data)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.sentEmails = append(m.sentEmails, EmailMessage{
			To:      to,
			Subject: fmt.Sprintf("Template: %s", template),
			Body:    fmt.Sprintf("Template data: %+v", data),
			IsHTML:  true,
		})
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// GetSentEmails returns all sent emails (for testing)
func (m *MockEmailService) GetSentEmails() []EmailMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	emails := make([]EmailMessage, len(m.sentEmails))
	copy(emails, m.sentEmails)
	return emails
}

// ClearSentEmails clears the sent emails list
func (m *MockEmailService) ClearSentEmails() {
	m.mu.Lock()
	m.sentEmails = m.sentEmails[:0]
	m.mu.Unlock()
}

// SetupStandardMockBehavior sets up common mock behaviors
func (m *MockEmailService) SetupStandardMockBehavior() {
	// All email operations succeed by default
	m.On("SendEmail", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("bool")).
		Return(nil)
	
	m.On("SendTemplateEmail", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.Anything).
		Return(nil)
}

// MockMessageBus provides a mock implementation of message bus
type MockMessageBus struct {
	mock.Mock
	publishedEvents []interface{}
	mu              sync.RWMutex
}

// NewMockMessageBus creates a new mock message bus
func NewMockMessageBus() *MockMessageBus {
	return &MockMessageBus{
		publishedEvents: make([]interface{}, 0),
	}
}

// Publish publishes an event
func (m *MockMessageBus) Publish(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.publishedEvents = append(m.publishedEvents, event)
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// Subscribe subscribes to events
func (m *MockMessageBus) Subscribe(ctx context.Context, eventType string, handler func(interface{}) error) error {
	args := m.Called(ctx, eventType, handler)
	return args.Error(0)
}

// GetPublishedEvents returns all published events (for testing)
func (m *MockMessageBus) GetPublishedEvents() []interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	events := make([]interface{}, len(m.publishedEvents))
	copy(events, m.publishedEvents)
	return events
}

// ClearPublishedEvents clears the published events list
func (m *MockMessageBus) ClearPublishedEvents() {
	m.mu.Lock()
	m.publishedEvents = m.publishedEvents[:0]
	m.mu.Unlock()
}

// SetupStandardMockBehavior sets up common mock behaviors
func (m *MockMessageBus) SetupStandardMockBehavior() {
	// All operations succeed by default
	m.On("Publish", mock.Anything, mock.Anything).
		Return(nil)
	
	m.On("Subscribe", mock.Anything, mock.AnythingOfType("string"), mock.Anything).
		Return(nil)
}

// MockServiceContainer provides a container with all mock services
type MockServiceContainer struct {
	RecipeRepo  *MockRecipeRepository
	UserRepo    *MockUserRepository
	AIService   *MockAIService
	EmailService *MockEmailService
	MessageBus  *MockMessageBus
}

// NewMockServiceContainer creates a new mock service container
func NewMockServiceContainer() *MockServiceContainer {
	container := &MockServiceContainer{
		RecipeRepo:   NewMockRecipeRepository(),
		UserRepo:     NewMockUserRepository(),
		AIService:    NewMockAIService(),
		EmailService: NewMockEmailService(),
		MessageBus:   NewMockMessageBus(),
	}

	// Setup standard behaviors
	container.RecipeRepo.SetupStandardMockBehavior()
	container.UserRepo.SetupStandardMockBehavior()
	container.AIService.SetupStandardMockBehavior()
	container.EmailService.SetupStandardMockBehavior()
	container.MessageBus.SetupStandardMockBehavior()

	return container
}

// AssertExpectations asserts that all mocks met their expectations
func (c *MockServiceContainer) AssertExpectations(t mock.TestingT) {
	c.RecipeRepo.AssertExpectations(t)
	c.UserRepo.AssertExpectations(t)
	c.AIService.AssertExpectations(t)
	c.EmailService.AssertExpectations(t)
	c.MessageBus.AssertExpectations(t)
}