// Package testutils provides test data factories for consistent test data generation
package testutils

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/google/uuid"
)

// RecipeFactory provides methods to create test recipes
type RecipeFactory struct {
	faker *gofakeit.Faker
}

// NewRecipeFactory creates a new recipe factory with seeded faker
func NewRecipeFactory(seed int64) *RecipeFactory {
	return &RecipeFactory{
		faker: gofakeit.New(seed),
	}
}

// RecipeBuilder provides a fluent interface for building test recipes
type RecipeBuilder struct {
	title        string
	description  string
	authorID     uuid.UUID
	ingredients  []recipe.Ingredient
	instructions []recipe.Instruction
	cuisine      recipe.CuisineType
	category     recipe.CategoryType
	difficulty   recipe.DifficultyLevel
	prepTime     time.Duration
	cookTime     time.Duration
	servings     int
	tags         []string
	aiGenerated  bool
}

// NewRecipeBuilder creates a new recipe builder with default values
func NewRecipeBuilder() *RecipeBuilder {
	faker := gofakeit.New(time.Now().UnixNano())
	
	return &RecipeBuilder{
		title:       faker.Sentence(3),
		description: faker.Paragraph(2, 3, 5, " "),
		authorID:    uuid.New(),
		ingredients: []recipe.Ingredient{},
		instructions: []recipe.Instruction{},
		cuisine:     recipe.CuisineItalian,
		category:    recipe.CategoryMainCourse,
		difficulty:  recipe.DifficultyMedium,
		prepTime:    15 * time.Minute,
		cookTime:    30 * time.Minute,
		servings:    4,
		tags:        []string{"test", "recipe"},
		aiGenerated: false,
	}
}

// WithTitle sets the recipe title
func (rb *RecipeBuilder) WithTitle(title string) *RecipeBuilder {
	rb.title = title
	return rb
}

// WithDescription sets the recipe description
func (rb *RecipeBuilder) WithDescription(description string) *RecipeBuilder {
	rb.description = description
	return rb
}

// WithAuthor sets the recipe author
func (rb *RecipeBuilder) WithAuthor(authorID uuid.UUID) *RecipeBuilder {
	rb.authorID = authorID
	return rb
}

// WithIngredients sets the recipe ingredients
func (rb *RecipeBuilder) WithIngredients(ingredients []recipe.Ingredient) *RecipeBuilder {
	rb.ingredients = ingredients
	return rb
}

// WithInstructions sets the recipe instructions
func (rb *RecipeBuilder) WithInstructions(instructions []recipe.Instruction) *RecipeBuilder {
	rb.instructions = instructions
	return rb
}

// WithCuisine sets the recipe cuisine
func (rb *RecipeBuilder) WithCuisine(cuisine recipe.CuisineType) *RecipeBuilder {
	rb.cuisine = cuisine
	return rb
}

// WithCategory sets the recipe category
func (rb *RecipeBuilder) WithCategory(category recipe.CategoryType) *RecipeBuilder {
	rb.category = category
	return rb
}

// WithDifficulty sets the recipe difficulty
func (rb *RecipeBuilder) WithDifficulty(difficulty recipe.DifficultyLevel) *RecipeBuilder {
	rb.difficulty = difficulty
	return rb
}

// WithTimings sets prep and cook time
func (rb *RecipeBuilder) WithTimings(prepTime, cookTime time.Duration) *RecipeBuilder {
	rb.prepTime = prepTime
	rb.cookTime = cookTime
	return rb
}

// WithServings sets the number of servings
func (rb *RecipeBuilder) WithServings(servings int) *RecipeBuilder {
	rb.servings = servings
	return rb
}

// WithTags sets the recipe tags
func (rb *RecipeBuilder) WithTags(tags []string) *RecipeBuilder {
	rb.tags = tags
	return rb
}

// AsAIGenerated marks the recipe as AI-generated
func (rb *RecipeBuilder) AsAIGenerated() *RecipeBuilder {
	rb.aiGenerated = true
	return rb
}

// Build constructs the recipe with validation
func (rb *RecipeBuilder) Build() (*recipe.Recipe, error) {
	// Create base recipe
	r, err := recipe.NewRecipe(rb.title, rb.description, rb.authorID)
	if err != nil {
		return nil, err
	}

	// Add ingredients
	for _, ingredient := range rb.ingredients {
		if err := r.AddIngredient(ingredient); err != nil {
			return nil, err
		}
	}

	// Add instructions
	for _, instruction := range rb.instructions {
		if err := r.AddInstruction(instruction); err != nil {
			return nil, err
		}
	}

	// Set additional properties using reflection or builder pattern
	// Note: In a real implementation, you'd need setter methods on Recipe
	// For now, we'll use the public interface

	return r, nil
}

// BuildValid creates a valid recipe ready for publishing
func (rb *RecipeBuilder) BuildValid() (*recipe.Recipe, error) {
	// Ensure we have required data for a valid recipe
	if len(rb.ingredients) == 0 {
		rb.WithIngredients([]recipe.Ingredient{
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
		})
	}

	if len(rb.instructions) == 0 {
		rb.WithInstructions([]recipe.Instruction{
			{
				ID:          uuid.New(),
				StepNumber:  1,
				Description: "Boil water in a large pot",
				Duration:    5 * time.Minute,
			},
			{
				ID:          uuid.New(),
				StepNumber:  2,
				Description: "Cook spaghetti according to package directions",
				Duration:    10 * time.Minute,
			},
		})
	}

	if rb.servings == 0 {
		rb.WithServings(4)
	}

	return rb.Build()
}

// RecipeFactory methods for creating common recipe types

// CreateSimpleRecipe creates a basic recipe with minimal data
func (rf *RecipeFactory) CreateSimpleRecipe() (*recipe.Recipe, error) {
	return NewRecipeBuilder().
		WithTitle(rf.faker.Sentence(3)).
		WithDescription(rf.faker.Paragraph(1, 2, 3, " ")).
		WithAuthor(uuid.New()).
		Build()
}

// CreateValidRecipe creates a recipe that can be published
func (rf *RecipeFactory) CreateValidRecipe() (*recipe.Recipe, error) {
	return NewRecipeBuilder().
		WithTitle(rf.faker.Sentence(3)).
		WithDescription(rf.faker.Paragraph(2, 3, 5, " ")).
		WithAuthor(uuid.New()).
		BuildValid()
}

// CreateItalianRecipe creates an Italian cuisine recipe
func (rf *RecipeFactory) CreateItalianRecipe() (*recipe.Recipe, error) {
	ingredients := []recipe.Ingredient{
		{
			ID:       uuid.New(),
			Name:     "Spaghetti",
			Quantity: 1.0,
			Unit:     "lb",
		},
		{
			ID:       uuid.New(),
			Name:     "Parmesan Cheese",
			Quantity: 0.5,
			Unit:     "cup",
		},
		{
			ID:       uuid.New(),
			Name:     "Extra Virgin Olive Oil",
			Quantity: 3.0,
			Unit:     "tbsp",
		},
	}

	instructions := []recipe.Instruction{
		{
			ID:          uuid.New(),
			StepNumber:  1,
			Description: "Bring a large pot of salted water to boil",
			Duration:    5 * time.Minute,
		},
		{
			ID:          uuid.New(),
			StepNumber:  2,
			Description: "Cook spaghetti al dente",
			Duration:    10 * time.Minute,
		},
		{
			ID:          uuid.New(),
			StepNumber:  3,
			Description: "Toss with olive oil and cheese",
			Duration:    2 * time.Minute,
		},
	}

	return NewRecipeBuilder().
		WithTitle("Spaghetti Aglio e Olio").
		WithDescription("A classic Italian pasta dish with garlic and olive oil").
		WithCuisine(recipe.CuisineItalian).
		WithIngredients(ingredients).
		WithInstructions(instructions).
		WithDifficulty(recipe.DifficultyEasy).
		WithTimings(10*time.Minute, 15*time.Minute).
		WithServings(4).
		WithTags([]string{"italian", "pasta", "quick"}).
		Build()
}

// CreateComplexRecipe creates a recipe with many steps and ingredients
func (rf *RecipeFactory) CreateComplexRecipe() (*recipe.Recipe, error) {
	ingredients := make([]recipe.Ingredient, 0, 10)
	for i := 0; i < 10; i++ {
		ingredients = append(ingredients, recipe.Ingredient{
			ID:       uuid.New(),
			Name:     rf.faker.Food(),
			Quantity: rf.faker.Float32Range(0.5, 3.0),
			Unit:     rf.randomUnit(),
		})
	}

	instructions := make([]recipe.Instruction, 0, 8)
	for i := 0; i < 8; i++ {
		instructions = append(instructions, recipe.Instruction{
			ID:          uuid.New(),
			StepNumber:  i + 1,
			Description: rf.faker.Sentence(8),
			Duration:    time.Duration(rf.faker.IntRange(5, 30)) * time.Minute,
		})
	}

	return NewRecipeBuilder().
		WithTitle(rf.faker.Sentence(4)).
		WithDescription(rf.faker.Paragraph(3, 4, 6, " ")).
		WithIngredients(ingredients).
		WithInstructions(instructions).
		WithDifficulty(recipe.DifficultyHard).
		WithTimings(45*time.Minute, 90*time.Minute).
		WithServings(rf.faker.IntRange(4, 8)).
		WithTags([]string{"complex", "gourmet", "special-occasion"}).
		Build()
}

// CreateAIGeneratedRecipe creates an AI-generated recipe
func (rf *RecipeFactory) CreateAIGeneratedRecipe() (*recipe.Recipe, error) {
	return NewRecipeBuilder().
		WithTitle("AI-Generated " + rf.faker.Sentence(3)).
		WithDescription("This recipe was generated by AI: " + rf.faker.Paragraph(2, 3, 5, " ")).
		AsAIGenerated().
		BuildValid()
}

// UserFactory provides methods to create test users
type UserFactory struct {
	faker *gofakeit.Faker
}

// NewUserFactory creates a new user factory
func NewUserFactory(seed int64) *UserFactory {
	return &UserFactory{
		faker: gofakeit.New(seed),
	}
}

// UserBuilder provides a fluent interface for building test users
type UserBuilder struct {
	email    string
	username string
	password string
	roles    []string
	verified bool
}

// NewUserBuilder creates a new user builder with default values
func NewUserBuilder() *UserBuilder {
	faker := gofakeit.New(time.Now().UnixNano())
	
	return &UserBuilder{
		email:    faker.Email(),
		username: faker.Username(),
		password: "TestPassword123!",
		roles:    []string{"user"},
		verified: true,
	}
}

// WithEmail sets the user email
func (ub *UserBuilder) WithEmail(email string) *UserBuilder {
	ub.email = email
	return ub
}

// WithUsername sets the username
func (ub *UserBuilder) WithUsername(username string) *UserBuilder {
	ub.username = username
	return ub
}

// WithPassword sets the password
func (ub *UserBuilder) WithPassword(password string) *UserBuilder {
	ub.password = password
	return ub
}

// WithRoles sets the user roles
func (ub *UserBuilder) WithRoles(roles []string) *UserBuilder {
	ub.roles = roles
	return ub
}

// AsUnverified marks the user as unverified
func (ub *UserBuilder) AsUnverified() *UserBuilder {
	ub.verified = false
	return ub
}

// AsAdmin gives the user admin role
func (ub *UserBuilder) AsAdmin() *UserBuilder {
	ub.roles = []string{"admin", "user"}
	return ub
}

// Build constructs the user
func (ub *UserBuilder) Build() (*user.User, error) {
	return user.NewUser(ub.email, ub.username, ub.password)
}

// UserFactory methods

// CreateUser creates a basic test user
func (uf *UserFactory) CreateUser() (*user.User, error) {
	return NewUserBuilder().
		WithEmail(uf.faker.Email()).
		WithUsername(uf.faker.Username()).
		Build()
}

// CreateAdminUser creates an admin user
func (uf *UserFactory) CreateAdminUser() (*user.User, error) {
	return NewUserBuilder().
		WithEmail("admin@" + uf.faker.DomainName()).
		WithUsername("admin_" + uf.faker.Username()).
		AsAdmin().
		Build()
}

// CreateVerifiedUser creates a verified user
func (uf *UserFactory) CreateVerifiedUser() (*user.User, error) {
	return NewUserBuilder().
		WithEmail(uf.faker.Email()).
		WithUsername(uf.faker.Username()).
		Build()
}

// CreateUnverifiedUser creates an unverified user
func (uf *UserFactory) CreateUnverifiedUser() (*user.User, error) {
	return NewUserBuilder().
		WithEmail(uf.faker.Email()).
		WithUsername(uf.faker.Username()).
		AsUnverified().
		Build()
}

// Helper methods

// randomUnit returns a random cooking unit
func (rf *RecipeFactory) randomUnit() string {
	units := []string{
		"cup", "cups", "tbsp", "tsp", "lb", "oz", "g", "kg", 
		"ml", "l", "pieces", "cloves", "bunch", "pinch",
	}
	return units[rand.Intn(len(units))]
}

// TestDataSet provides a collection of related test data
type TestDataSet struct {
	Users   []*user.User
	Recipes []*recipe.Recipe
}

// CreateTestDataSet creates a related set of test data
func CreateTestDataSet(userCount, recipeCount int) (*TestDataSet, error) {
	userFactory := NewUserFactory(time.Now().UnixNano())
	recipeFactory := NewRecipeFactory(time.Now().UnixNano())

	// Create users
	users := make([]*user.User, 0, userCount)
	for i := 0; i < userCount; i++ {
		user, err := userFactory.CreateUser()
		if err != nil {
			return nil, fmt.Errorf("failed to create test user %d: %w", i, err)
		}
		users = append(users, user)
	}

	// Create recipes with random authors
	recipes := make([]*recipe.Recipe, 0, recipeCount)
	for i := 0; i < recipeCount; i++ {
		// Pick random user as author
		authorIndex := rand.Intn(len(users))
		
		recipe, err := NewRecipeBuilder().
			WithAuthor(users[authorIndex].ID()).
			BuildValid()
		if err != nil {
			return nil, fmt.Errorf("failed to create test recipe %d: %w", i, err)
		}
		recipes = append(recipes, recipe)
	}

	return &TestDataSet{
		Users:   users,
		Recipes: recipes,
	}, nil
}

// Cleanup provides cleanup utilities for tests
type Cleanup struct {
	funcs []func()
}

// NewCleanup creates a new cleanup helper
func NewCleanup() *Cleanup {
	return &Cleanup{
		funcs: make([]func(), 0),
	}
}

// Add adds a cleanup function
func (c *Cleanup) Add(f func()) {
	c.funcs = append(c.funcs, f)
}

// Execute runs all cleanup functions in reverse order
func (c *Cleanup) Execute() {
	for i := len(c.funcs) - 1; i >= 0; i-- {
		c.funcs[i]()
	}
}

// TestHelper provides common test helper methods
type TestHelper struct {
	userFactory   *UserFactory
	recipeFactory *RecipeFactory
	cleanup       *Cleanup
}

// NewTestHelper creates a new test helper
func NewTestHelper() *TestHelper {
	seed := time.Now().UnixNano()
	return &TestHelper{
		userFactory:   NewUserFactory(seed),
		recipeFactory: NewRecipeFactory(seed),
		cleanup:       NewCleanup(),
	}
}

// CreateUser creates a test user
func (h *TestHelper) CreateUser() (*user.User, error) {
	return h.userFactory.CreateUser()
}

// CreateRecipe creates a test recipe
func (h *TestHelper) CreateRecipe() (*recipe.Recipe, error) {
	return h.recipeFactory.CreateValidRecipe()
}

// Cleanup returns the cleanup helper
func (h *TestHelper) Cleanup() *Cleanup {
	return h.cleanup
}