package recipe

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// RecipeTestSuite provides a test suite for Recipe entity
type RecipeTestSuite struct {
	suite.Suite
}

// SetupSuite initializes the test suite
func (suite *RecipeTestSuite) SetupSuite() {
	// No initialization needed
}

// createValidRecipe creates a valid recipe with ingredients and instructions for testing
func (suite *RecipeTestSuite) createValidRecipe() (*Recipe, error) {
	recipe, err := NewRecipe("Test Recipe", "A valid test recipe", uuid.New())
	if err != nil {
		return nil, err
	}

	// Add a valid ingredient
	ingredient := Ingredient{
		ID:     uuid.New(),
		Name:   "Test Ingredient",
		Amount: 1.0,
		Unit:   MeasurementUnitCup,
	}
	err = recipe.AddIngredient(ingredient)
	if err != nil {
		return nil, err
	}

	// Add a valid instruction
	instruction := Instruction{
		Description: "Test instruction",
		Duration:    5 * time.Minute,
	}
	err = recipe.AddInstruction(instruction)
	if err != nil {
		return nil, err
	}

	return recipe, nil
}

// TestRecipeCreation tests recipe creation scenarios
func (suite *RecipeTestSuite) TestRecipeCreation() {
	suite.Run("ValidRecipe_ShouldCreateSuccessfully", func() {
		// Arrange
		title := "Spaghetti Carbonara"
		description := "A classic Italian pasta dish"
		authorID := uuid.New()

		// Act
		recipe, err := NewRecipe(title, description, authorID)

		// Assert
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), recipe)

		assert.Equal(suite.T(), title, recipe.Title())
		assert.NotEqual(suite.T(), uuid.Nil, recipe.ID())
		assert.Equal(suite.T(), RecipeStatusDraft, recipe.status)
		assert.NotZero(suite.T(), recipe.createdAt)
		assert.NotZero(suite.T(), recipe.updatedAt)
		assert.Equal(suite.T(), int64(1), recipe.version)

		// Check domain events
		events := recipe.Events()
		assert.Len(suite.T(), events, 1)

		createdEvent, ok := events[0].(RecipeCreatedEvent)
		assert.True(suite.T(), ok, "Should emit RecipeCreatedEvent")
		assert.Equal(suite.T(), recipe.ID(), createdEvent.RecipeID)
		assert.Equal(suite.T(), authorID, createdEvent.AuthorID)
	})

	suite.Run("EmptyTitle_ShouldReturnError", func() {
		// Arrange
		title := ""
		description := "Valid description"
		authorID := uuid.New()

		// Act
		recipe, err := NewRecipe(title, description, authorID)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), recipe)
		assert.Equal(suite.T(), ErrTitleTooShort, err)
	})

	suite.Run("TitleTooShort_ShouldReturnError", func() {
		// Arrange
		title := "AB" // Less than 3 characters
		description := "Valid description"
		authorID := uuid.New()

		// Act
		recipe, err := NewRecipe(title, description, authorID)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), recipe)
		assert.Equal(suite.T(), ErrTitleTooShort, err)
	})

	suite.Run("TitleTooLong_ShouldReturnError", func() {
		// Arrange
		title := string(make([]byte, 201)) // More than 200 characters
		description := "Valid description"
		authorID := uuid.New()

		// Act
		recipe, err := NewRecipe(title, description, authorID)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), recipe)
		assert.Equal(suite.T(), ErrTitleTooLong, err)
	})

	suite.Run("DescriptionTooLong_ShouldReturnError", func() {
		// Arrange
		title := "Valid Title"
		description := string(make([]byte, 2001)) // More than 2000 characters
		authorID := uuid.New()

		// Act
		recipe, err := NewRecipe(title, description, authorID)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), recipe)
		assert.Equal(suite.T(), ErrDescriptionTooLong, err)
	})
}

// TestRecipeModification tests recipe modification scenarios
func (suite *RecipeTestSuite) TestRecipeModification() {
	suite.Run("UpdateTitle_ValidTitle_ShouldUpdate", func() {
		// Arrange
		recipe, _ := NewRecipe("Original Title", "Description", uuid.New())
		newTitle := "Updated Title"
		originalUpdatedAt := recipe.updatedAt

		// Act
		time.Sleep(1 * time.Millisecond) // Ensure time difference
		err := recipe.UpdateTitle(newTitle)

		// Assert
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), newTitle, recipe.Title())
		assert.True(suite.T(), recipe.updatedAt.After(originalUpdatedAt))

		// Check domain events
		events := recipe.Events()
		assert.Len(suite.T(), events, 1) // Only the update event (creation event was consumed)

		titleEvent, ok := events[0].(RecipeTitleUpdatedEvent)
		assert.True(suite.T(), ok, "Should emit RecipeTitleUpdatedEvent")
		assert.Equal(suite.T(), "Original Title", titleEvent.OldTitle)
		assert.Equal(suite.T(), newTitle, titleEvent.NewTitle)
	})

	suite.Run("UpdateTitle_InvalidTitle_ShouldReturnError", func() {
		// Arrange
		recipe, _ := NewRecipe("Original Title", "Description", uuid.New())
		newTitle := "" // Invalid title

		// Act
		err := recipe.UpdateTitle(newTitle)

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), ErrTitleTooShort, err)
		assert.Equal(suite.T(), "Original Title", recipe.Title()) // Should not change
	})
}

// TestRecipeIngredients tests ingredient management
func (suite *RecipeTestSuite) TestRecipeIngredients() {
	suite.Run("AddValidIngredient_ShouldAdd", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		ingredient := Ingredient{
			ID:     uuid.New(),
			Name:   "Spaghetti",
			Amount: 1.0,
			Unit:   MeasurementUnitPound,
		}

		// Act
		err := recipe.AddIngredient(ingredient)

		// Assert
		require.NoError(suite.T(), err)

		// Check domain events
		events := recipe.Events()
		assert.Len(suite.T(), events, 1) // Creation event was consumed earlier

		ingredientEvent, ok := events[0].(IngredientAddedEvent)
		assert.True(suite.T(), ok, "Should emit IngredientAddedEvent")
		assert.Equal(suite.T(), recipe.ID(), ingredientEvent.RecipeID)
		assert.Equal(suite.T(), ingredient.ID, ingredientEvent.IngredientID)
	})

	suite.Run("AddInvalidIngredient_ShouldReturnError", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		ingredient := Ingredient{
			ID:     uuid.New(),
			Name:   "", // Invalid - empty name
			Amount: 1.0,
			Unit:   MeasurementUnitPound,
		}

		// Act
		err := recipe.AddIngredient(ingredient)

		// Assert
		assert.Error(suite.T(), err)
	})
}

// TestRecipeInstructions tests instruction management
func (suite *RecipeTestSuite) TestRecipeInstructions() {
	suite.Run("AddValidInstruction_ShouldAdd", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		instruction := Instruction{
			Description: "Boil water in a large pot",
			Duration:    5 * time.Minute,
		}

		// Act
		err := recipe.AddInstruction(instruction)

		// Assert
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 1, instruction.StepNumber) // Should be set automatically
	})

	suite.Run("AddMultipleInstructions_ShouldNumberSequentially", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		instruction1 := Instruction{
			Description: "First step",
			Duration:    5 * time.Minute,
		}
		instruction2 := Instruction{
			Description: "Second step",
			Duration:    10 * time.Minute,
		}

		// Act
		err1 := recipe.AddInstruction(instruction1)
		err2 := recipe.AddInstruction(instruction2)

		// Assert
		require.NoError(suite.T(), err1)
		require.NoError(suite.T(), err2)
		assert.Equal(suite.T(), 1, instruction1.StepNumber)
		assert.Equal(suite.T(), 2, instruction2.StepNumber)
	})
}

// TestRecipePublishing tests recipe publishing logic
func (suite *RecipeTestSuite) TestRecipePublishing() {
	suite.Run("PublishValidRecipe_ShouldPublish", func() {
		// Arrange
		recipe, _ := suite.createValidRecipe()
		recipe.Events() // Clear creation events

		// Act
		err := recipe.Publish()

		// Assert
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), RecipeStatusPublished, recipe.status)
		assert.NotNil(suite.T(), recipe.publishedAt)

		// Check domain events
		events := recipe.Events()
		assert.Len(suite.T(), events, 1)

		publishedEvent, ok := events[0].(RecipePublishedEvent)
		assert.True(suite.T(), ok, "Should emit RecipePublishedEvent")
		assert.Equal(suite.T(), recipe.ID(), publishedEvent.RecipeID)
	})

	suite.Run("PublishRecipeWithoutIngredients_ShouldReturnError", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		// Don't add ingredients

		// Act
		err := recipe.Publish()

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), ErrNoIngredients, err)
		assert.Equal(suite.T(), RecipeStatusDraft, recipe.status)
	})

	suite.Run("PublishRecipeWithoutInstructions_ShouldReturnError", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		recipe.AddIngredient(Ingredient{
			ID:     uuid.New(),
			Name:   "Test Ingredient",
			Amount: 1.0,
			Unit:   MeasurementUnitCup,
		})
		// Don't add instructions

		// Act
		err := recipe.Publish()

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), ErrNoInstructions, err)
		assert.Equal(suite.T(), RecipeStatusDraft, recipe.status)
	})

	suite.Run("PublishAlreadyPublishedRecipe_ShouldReturnError", func() {
		// Arrange
		recipe, _ := suite.createValidRecipe()
		recipe.Publish() // Publish first time

		// Act
		err := recipe.Publish() // Try to publish again

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), ErrInvalidStatusTransition, err)
	})
}

// TestRecipeArchiving tests recipe archiving logic
func (suite *RecipeTestSuite) TestRecipeArchiving() {
	suite.Run("ArchivePublishedRecipe_ShouldArchive", func() {
		// Arrange
		recipe, _ := suite.createValidRecipe()
		recipe.Publish()
		recipe.Events() // Clear events

		// Act
		err := recipe.Archive()

		// Assert
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), RecipeStatusArchived, recipe.status)

		// Check domain events
		events := recipe.Events()
		assert.Len(suite.T(), events, 1)

		archivedEvent, ok := events[0].(RecipeArchivedEvent)
		assert.True(suite.T(), ok, "Should emit RecipeArchivedEvent")
		assert.Equal(suite.T(), recipe.ID(), archivedEvent.RecipeID)
	})

	suite.Run("ArchiveDraftRecipe_ShouldReturnError", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())

		// Act
		err := recipe.Archive()

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), ErrInvalidStatusTransition, err)
		assert.Equal(suite.T(), RecipeStatusDraft, recipe.status)
	})
}

// TestRecipeSocialFeatures tests social features like likes and ratings
func (suite *RecipeTestSuite) TestRecipeSocialFeatures() {
	suite.Run("LikeRecipe_ShouldIncrementLikes", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		userID := uuid.New()
		originalLikes := recipe.likes
		recipe.Events() // Clear creation events

		// Act
		recipe.Like(userID)

		// Assert
		assert.Equal(suite.T(), originalLikes+1, recipe.likes)

		// Check domain events
		events := recipe.Events()
		assert.Len(suite.T(), events, 1)

		likedEvent, ok := events[0].(RecipeLikedEvent)
		assert.True(suite.T(), ok, "Should emit RecipeLikedEvent")
		assert.Equal(suite.T(), recipe.ID(), likedEvent.RecipeID)
		assert.Equal(suite.T(), userID, likedEvent.UserID)
	})

	suite.Run("AddValidRating_ShouldAddAndUpdateAverage", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		rating := Rating{
			UserID: uuid.New(),
			Value:  5,
		}
		recipe.Events() // Clear creation events

		// Act
		err := recipe.AddRating(rating)

		// Assert
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 5.0, recipe.averageRating)

		// Check domain events
		events := recipe.Events()
		assert.Len(suite.T(), events, 1)

		ratedEvent, ok := events[0].(RecipeRatedEvent)
		assert.True(suite.T(), ok, "Should emit RecipeRatedEvent")
		assert.Equal(suite.T(), recipe.ID(), ratedEvent.RecipeID)
		assert.Equal(suite.T(), rating.UserID, ratedEvent.UserID)
		assert.Equal(suite.T(), rating.Value, ratedEvent.Rating)
	})

	suite.Run("AddMultipleRatings_ShouldCalculateCorrectAverage", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())

		// Act - Add ratings of 3, 4, 5
		recipe.AddRating(Rating{UserID: uuid.New(), Value: 3})
		recipe.AddRating(Rating{UserID: uuid.New(), Value: 4})
		recipe.AddRating(Rating{UserID: uuid.New(), Value: 5})

		// Assert
		expectedAverage := (3.0 + 4.0 + 5.0) / 3.0
		assert.Equal(suite.T(), expectedAverage, recipe.averageRating)
	})

	suite.Run("AddInvalidRating_ShouldReturnError", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		rating := Rating{
			UserID: uuid.New(),
			Value:  0, // Invalid - should be 1-5
		}

		// Act
		err := recipe.AddRating(rating)

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), 0.0, recipe.averageRating) // Should remain unchanged
	})
}

// TestRecipeEvents tests domain event handling
func (suite *RecipeTestSuite) TestRecipeEvents() {
	suite.Run("Events_ShouldBeClearedAfterRetrieval", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())

		// Act
		events1 := recipe.Events()
		events2 := recipe.Events()

		// Assert
		assert.Len(suite.T(), events1, 1) // Should have creation event
		assert.Len(suite.T(), events2, 0) // Should be empty after first retrieval
	})

	suite.Run("MultipleOperations_ShouldAccumulateEvents", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		recipe.Events() // Clear creation event
		userID := uuid.New()

		// Act
		recipe.Like(userID)
		recipe.UpdateTitle("New Title")

		events := recipe.Events()

		// Assert
		assert.Len(suite.T(), events, 2)

		// Verify event types
		likedEvent, ok1 := events[0].(RecipeLikedEvent)
		titleEvent, ok2 := events[1].(RecipeTitleUpdatedEvent)

		assert.True(suite.T(), ok1, "First event should be RecipeLikedEvent")
		assert.True(suite.T(), ok2, "Second event should be RecipeTitleUpdatedEvent")
		assert.Equal(suite.T(), userID, likedEvent.UserID)
		assert.Equal(suite.T(), "New Title", titleEvent.NewTitle)
	})
}

// TestRecipeValidation tests comprehensive recipe validation
func (suite *RecipeTestSuite) TestRecipeValidation() {
	suite.Run("ValidateForPublishing_ValidRecipe_ShouldPass", func() {
		// Arrange
		recipe, _ := suite.createValidRecipe()

		// Act
		err := recipe.validateForPublishing()

		// Assert
		assert.NoError(suite.T(), err)
	})

	suite.Run("ValidateForPublishing_NoIngredients_ShouldFail", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		// Add instructions but no ingredients
		recipe.AddInstruction(Instruction{
			Description: "Test instruction",
			Duration:    5 * time.Minute,
		})

		// Act
		err := recipe.validateForPublishing()

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), ErrNoIngredients, err)
	})

	suite.Run("ValidateForPublishing_NoInstructions_ShouldFail", func() {
		// Arrange
		recipe, _ := NewRecipe("Test Recipe", "Description", uuid.New())
		// Add ingredients but no instructions
		recipe.AddIngredient(Ingredient{
			ID:     uuid.New(),
			Name:   "Test Ingredient",
			Amount: 1.0,
			Unit:   MeasurementUnitCup,
		})

		// Act
		err := recipe.validateForPublishing()

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), ErrNoInstructions, err)
	})
}

// BenchmarkRecipeCreation benchmarks recipe creation performance
func BenchmarkRecipeCreation(b *testing.B) {
	title := "Benchmark Recipe"
	description := "A recipe for benchmarking"
	authorID := uuid.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recipe, err := NewRecipe(title, description, authorID)
		if err != nil {
			b.Fatal(err)
		}
		_ = recipe
	}
}

// BenchmarkRecipeAddIngredient benchmarks adding ingredients
func BenchmarkRecipeAddIngredient(b *testing.B) {
	ingredient := Ingredient{
		ID:     uuid.New(),
		Name:   "Test Ingredient",
		Amount: 1.0,
		Unit:   "cup",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a copy to avoid modifying the same recipe
		testRecipe, _ := NewRecipe("Benchmark Recipe", "Description", uuid.New())
		err := testRecipe.AddIngredient(ingredient)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRecipePublish benchmarks recipe publishing
func BenchmarkRecipePublish(b *testing.B) {
	createValidRecipe := func() (*Recipe, error) {
		recipe, err := NewRecipe("Benchmark Recipe", "A recipe for benchmarking", uuid.New())
		if err != nil {
			return nil, err
		}

		ingredient := Ingredient{
			ID:     uuid.New(),
			Name:   "Test Ingredient",
			Amount: 1.0,
			Unit:   MeasurementUnitCup,
		}
		err = recipe.AddIngredient(ingredient)
		if err != nil {
			return nil, err
		}

		instruction := Instruction{
			Description: "Test instruction",
			Duration:    5 * time.Minute,
		}
		err = recipe.AddInstruction(instruction)
		if err != nil {
			return nil, err
		}

		return recipe, nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recipe, _ := createValidRecipe()
		err := recipe.Publish()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestRecipeTestSuite runs the recipe test suite
func TestRecipeTestSuite(t *testing.T) {
	suite.Run(t, new(RecipeTestSuite))
}
