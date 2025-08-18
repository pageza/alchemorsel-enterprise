// Package integration provides integration tests using real database instances
//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/postgres"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// RecipeRepositoryIntegrationTestSuite provides integration tests for recipe repository
type RecipeRepositoryIntegrationTestSuite struct {
	suite.Suite
	testDB         *testutils.TestDatabase
	repository     *postgres.RecipeRepository
	recipeFactory  *testutils.RecipeFactory
	dbHelper       *testutils.DatabaseHelper
	assertions     *testutils.ComprehensiveAssertions
	ctx            context.Context
}

// SetupSuite initializes the test suite with real database
func (suite *RecipeRepositoryIntegrationTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	
	// Setup test database with testcontainers
	suite.testDB = testutils.SetupTestDatabase(suite.T())
	
	// Run migrations
	err := suite.testDB.RunMigrations()
	require.NoError(suite.T(), err, "Failed to run database migrations")
	
	// Setup repository
	suite.repository = postgres.NewRecipeRepository(suite.testDB.GormDB)
	
	// Setup test utilities
	suite.recipeFactory = testutils.NewRecipeFactory(time.Now().UnixNano())
	suite.dbHelper = testutils.NewDatabaseHelper(suite.testDB)
	suite.assertions = testutils.NewComprehensiveAssertions(suite.T(), suite.testDB)
}

// SetupTest prepares each test with clean database state
func (suite *RecipeRepositoryIntegrationTestSuite) SetupTest() {
	// Clean database before each test
	err := suite.testDB.TruncateAllTables()
	require.NoError(suite.T(), err, "Failed to clean database")
	
	// Seed basic test data
	err = suite.testDB.SeedTestData()
	require.NoError(suite.T(), err, "Failed to seed test data")
}

// TestSaveRecipe tests recipe saving functionality
func (suite *RecipeRepositoryIntegrationTestSuite) TestSaveRecipe() {
	suite.Run("SaveNewRecipe_ShouldPersistToDatabase", func() {
		// Arrange
		recipe, err := suite.recipeFactory.CreateValidRecipe()
		require.NoError(suite.T(), err)

		// Act
		err = suite.repository.Save(suite.ctx, recipe)

		// Assert
		require.NoError(suite.T(), err)
		
		// Verify recipe was saved to database
		suite.assertions.Database.RecordExists("recipes", "id = $1", recipe.ID())
		
		// Verify recipe data integrity
		var savedTitle string
		err = suite.testDB.DB.QueryRow("SELECT title FROM recipes WHERE id = $1", recipe.ID()).Scan(&savedTitle)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), recipe.Title(), savedTitle)
	})

	suite.Run("SaveExistingRecipe_ShouldUpdateInDatabase", func() {
		// Arrange
		recipe, err := suite.recipeFactory.CreateValidRecipe()
		require.NoError(suite.T(), err)
		
		// Save recipe first time
		err = suite.repository.Save(suite.ctx, recipe)
		require.NoError(suite.T(), err)
		
		// Modify recipe
		originalTitle := recipe.Title()
		newTitle := "Updated Recipe Title"
		err = recipe.UpdateTitle(newTitle)
		require.NoError(suite.T(), err)

		// Act - Save updated recipe
		err = suite.repository.Save(suite.ctx, recipe)

		// Assert
		require.NoError(suite.T(), err)
		
		// Verify recipe was updated
		var savedTitle string
		err = suite.testDB.DB.QueryRow("SELECT title FROM recipes WHERE id = $1", recipe.ID()).Scan(&savedTitle)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), newTitle, savedTitle)
		assert.NotEqual(suite.T(), originalTitle, savedTitle)
		
		// Verify only one record exists (update, not insert)
		count, err := suite.dbHelper.CountRecords("recipes")
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), 3, count) // 2 from seed data + 1 new
	})

	suite.Run("SaveRecipe_WithIngredients_ShouldPersistRelatedData", func() {
		// Arrange
		recipe, err := suite.recipeFactory.CreateValidRecipe()
		require.NoError(suite.T(), err)

		// Act
		err = suite.repository.Save(suite.ctx, recipe)

		// Assert
		require.NoError(suite.T(), err)
		
		// Verify recipe exists
		suite.assertions.Database.RecordExists("recipes", "id = $1", recipe.ID())
		
		// Verify ingredients were saved
		ingredientCount, err := suite.dbHelper.CountRecords("recipe_ingredients WHERE recipe_id = '" + recipe.ID().String() + "'")
		require.NoError(suite.T(), err)
		assert.Greater(suite.T(), ingredientCount, 0, "Recipe should have ingredients")
	})

	suite.Run("SaveRecipe_DatabaseError_ShouldReturnError", func() {
		// Arrange
		recipe, err := suite.recipeFactory.CreateValidRecipe()
		require.NoError(suite.T(), err)
		
		// Close database connection to simulate error
		suite.testDB.DB.Close()

		// Act
		err = suite.repository.Save(suite.ctx, recipe)

		// Assert
		assert.Error(suite.T(), err, "Should return error when database is unavailable")
		
		// Restore database connection for cleanup
		suite.testDB = testutils.SetupTestDatabase(suite.T())
		suite.repository = postgres.NewRecipeRepository(suite.testDB.GormDB)
	})
}

// TestFindRecipeByID tests recipe retrieval by ID
func (suite *RecipeRepositoryIntegrationTestSuite) TestFindRecipeByID() {
	suite.Run("FindExistingRecipe_ShouldReturnRecipe", func() {
		// Arrange
		recipe, err := suite.recipeFactory.CreateValidRecipe()
		require.NoError(suite.T(), err)
		
		err = suite.repository.Save(suite.ctx, recipe)
		require.NoError(suite.T(), err)

		// Act
		foundRecipe, err := suite.repository.FindByID(suite.ctx, recipe.ID())

		// Assert
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), foundRecipe)
		
		assert.Equal(suite.T(), recipe.ID(), foundRecipe.ID())
		assert.Equal(suite.T(), recipe.Title(), foundRecipe.Title())
		suite.assertions.Recipe.ValidRecipe(foundRecipe)
	})

	suite.Run("FindNonExistentRecipe_ShouldReturnNotFoundError", func() {
		// Arrange
		nonExistentID := uuid.New()

		// Act
		foundRecipe, err := suite.repository.FindByID(suite.ctx, nonExistentID)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), foundRecipe)
		// assert.Equal(suite.T(), outbound.ErrRecipeNotFound, err) // Uncomment when error is defined
	})

	suite.Run("FindRecipe_WithRelatedData_ShouldLoadCompleteRecipe", func() {
		// Arrange
		recipe, err := suite.recipeFactory.CreateComplexRecipe()
		require.NoError(suite.T(), err)
		
		err = suite.repository.Save(suite.ctx, recipe)
		require.NoError(suite.T(), err)

		// Act
		foundRecipe, err := suite.repository.FindByID(suite.ctx, recipe.ID())

		// Assert
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), foundRecipe)
		
		// Verify related data is loaded
		suite.assertions.Recipe.RecipeHasIngredients(foundRecipe, 1) // At least 1 ingredient
		suite.assertions.Recipe.RecipeHasInstructions(foundRecipe, 1) // At least 1 instruction
	})
}

// TestFindRecipesByAuthor tests finding recipes by author
func (suite *RecipeRepositoryIntegrationTestSuite) TestFindRecipesByAuthor() {
	suite.Run("FindRecipesByExistingAuthor_ShouldReturnRecipes", func() {
		// Arrange
		authorID := uuid.New()
		
		// Create multiple recipes for the same author
		recipe1, _ := testutils.NewRecipeBuilder().WithAuthor(authorID).BuildValid()
		recipe2, _ := testutils.NewRecipeBuilder().WithAuthor(authorID).BuildValid()
		recipe3, _ := testutils.NewRecipeBuilder().WithAuthor(uuid.New()).BuildValid() // Different author
		
		suite.repository.Save(suite.ctx, recipe1)
		suite.repository.Save(suite.ctx, recipe2)
		suite.repository.Save(suite.ctx, recipe3)

		// Act
		recipes, err := suite.repository.FindByAuthorID(suite.ctx, authorID, 10, 0)

		// Assert
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), recipes, 2, "Should return only recipes by the specified author")
		
		for _, recipe := range recipes {
			// Note: We would need a public method to get AuthorID
			// assert.Equal(suite.T(), authorID, recipe.AuthorID())
		}
	})

	suite.Run("FindRecipesByAuthor_WithPagination_ShouldReturnLimitedResults", func() {
		// Arrange
		authorID := uuid.New()
		
		// Create 5 recipes for the same author
		for i := 0; i < 5; i++ {
			recipe, _ := testutils.NewRecipeBuilder().WithAuthor(authorID).BuildValid()
			suite.repository.Save(suite.ctx, recipe)
		}

		// Act - Request only 3 recipes with offset 1
		recipes, err := suite.repository.FindByAuthorID(suite.ctx, authorID, 3, 1)

		// Assert
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), recipes, 3, "Should return exactly 3 recipes")
	})

	suite.Run("FindRecipesByNonExistentAuthor_ShouldReturnEmptySlice", func() {
		// Arrange
		nonExistentAuthorID := uuid.New()

		// Act
		recipes, err := suite.repository.FindByAuthorID(suite.ctx, nonExistentAuthorID, 10, 0)

		// Assert
		require.NoError(suite.T(), err)
		assert.Empty(suite.T(), recipes, "Should return empty slice for non-existent author")
	})
}

// TestFindPublishedRecipes tests finding published recipes
func (suite *RecipeRepositoryIntegrationTestSuite) TestFindPublishedRecipes() {
	suite.Run("FindPublishedRecipes_ShouldReturnOnlyPublishedRecipes", func() {
		// Arrange
		publishedRecipe1, _ := suite.recipeFactory.CreateValidRecipe()
		publishedRecipe2, _ := suite.recipeFactory.CreateValidRecipe()
		draftRecipe, _ := suite.recipeFactory.CreateValidRecipe()
		
		// Publish some recipes
		publishedRecipe1.Publish()
		publishedRecipe2.Publish()
		// Keep draftRecipe as draft
		
		suite.repository.Save(suite.ctx, publishedRecipe1)
		suite.repository.Save(suite.ctx, publishedRecipe2)
		suite.repository.Save(suite.ctx, draftRecipe)

		// Act
		recipes, err := suite.repository.FindPublished(suite.ctx, 10, 0)

		// Assert
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), recipes, 3, "Should return 2 published recipes + 1 from seed data")
		
		for _, recipe := range recipes {
			// Verify all returned recipes are published
			// suite.assertions.Recipe.RecipeStatus(recipe, recipe.RecipeStatusPublished)
		}
	})

	suite.Run("FindPublishedRecipes_WithPagination_ShouldReturnCorrectPage", func() {
		// Arrange - Create 5 published recipes
		for i := 0; i < 5; i++ {
			recipe, _ := suite.recipeFactory.CreateValidRecipe()
			recipe.Publish()
			suite.repository.Save(suite.ctx, recipe)
		}

		// Act - Get second page with 2 recipes per page
		recipes, err := suite.repository.FindPublished(suite.ctx, 2, 2)

		// Assert
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), recipes, 2, "Should return exactly 2 recipes for page 2")
	})
}

// TestSearchRecipes tests recipe search functionality
func (suite *RecipeRepositoryIntegrationTestSuite) TestSearchRecipes() {
	suite.Run("SearchRecipes_ByTitle_ShouldReturnMatchingRecipes", func() {
		// Arrange
		pastaRecipe, _ := testutils.NewRecipeBuilder().
			WithTitle("Delicious Pasta Carbonara").
			BuildValid()
		pizzaRecipe, _ := testutils.NewRecipeBuilder().
			WithTitle("Margherita Pizza").
			BuildValid()
		saladRecipe, _ := testutils.NewRecipeBuilder().
			WithTitle("Caesar Salad").
			BuildValid()
		
		suite.repository.Save(suite.ctx, pastaRecipe)
		suite.repository.Save(suite.ctx, pizzaRecipe)
		suite.repository.Save(suite.ctx, saladRecipe)

		// Act
		recipes, err := suite.repository.Search(suite.ctx, "pasta", nil, 10, 0)

		// Assert
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), recipes, 1, "Should return only pasta recipe")
		assert.Contains(suite.T(), recipes[0].Title(), "Pasta")
	})

	suite.Run("SearchRecipes_WithFilters_ShouldApplyFilters", func() {
		// Arrange
		italianRecipe, _ := testutils.NewRecipeBuilder().
			WithCuisine(recipe.CuisineItalian).
			WithDifficulty(recipe.DifficultyEasy).
			BuildValid()
		mexicanRecipe, _ := testutils.NewRecipeBuilder().
			WithCuisine(recipe.CuisineMexican).
			WithDifficulty(recipe.DifficultyEasy).
			BuildValid()
		
		suite.repository.Save(suite.ctx, italianRecipe)
		suite.repository.Save(suite.ctx, mexicanRecipe)

		// Act
		filters := map[string]interface{}{
			"cuisine": recipe.CuisineItalian,
		}
		recipes, err := suite.repository.Search(suite.ctx, "", filters, 10, 0)

		// Assert
		require.NoError(suite.T(), err)
		assert.Len(suite.T(), recipes, 1, "Should return only Italian recipe")
	})

	suite.Run("SearchRecipes_NoMatches_ShouldReturnEmptySlice", func() {
		// Arrange
		recipe, _ := suite.recipeFactory.CreateValidRecipe()
		suite.repository.Save(suite.ctx, recipe)

		// Act
		recipes, err := suite.repository.Search(suite.ctx, "nonexistent", nil, 10, 0)

		// Assert
		require.NoError(suite.T(), err)
		assert.Empty(suite.T(), recipes, "Should return empty slice for no matches")
	})
}

// TestDeleteRecipe tests recipe deletion
func (suite *RecipeRepositoryIntegrationTestSuite) TestDeleteRecipe() {
	suite.Run("DeleteExistingRecipe_ShouldRemoveFromDatabase", func() {
		// Arrange
		recipe, err := suite.recipeFactory.CreateValidRecipe()
		require.NoError(suite.T(), err)
		
		err = suite.repository.Save(suite.ctx, recipe)
		require.NoError(suite.T(), err)
		
		// Verify recipe exists
		suite.assertions.Database.RecordExists("recipes", "id = $1", recipe.ID())

		// Act
		err = suite.repository.Delete(suite.ctx, recipe.ID())

		// Assert
		require.NoError(suite.T(), err)
		
		// Verify recipe is deleted (soft delete or hard delete depending on implementation)
		suite.assertions.Database.RecordNotExists("recipes", "id = $1 AND deleted_at IS NULL", recipe.ID())
	})

	suite.Run("DeleteNonExistentRecipe_ShouldReturnError", func() {
		// Arrange
		nonExistentID := uuid.New()

		// Act
		err := suite.repository.Delete(suite.ctx, nonExistentID)

		// Assert
		assert.Error(suite.T(), err, "Should return error when trying to delete non-existent recipe")
	})

	suite.Run("DeleteRecipe_ShouldCascadeToRelatedData", func() {
		// Arrange
		recipe, err := suite.recipeFactory.CreateComplexRecipe()
		require.NoError(suite.T(), err)
		
		err = suite.repository.Save(suite.ctx, recipe)
		require.NoError(suite.T(), err)
		
		// Verify related data exists
		ingredientCount, _ := suite.dbHelper.CountRecords("recipe_ingredients WHERE recipe_id = '" + recipe.ID().String() + "'")
		assert.Greater(suite.T(), ingredientCount, 0, "Recipe should have ingredients")

		// Act
		err = suite.repository.Delete(suite.ctx, recipe.ID())

		// Assert
		require.NoError(suite.T(), err)
		
		// Verify related data is also deleted
		ingredientCount, _ = suite.dbHelper.CountRecords("recipe_ingredients WHERE recipe_id = '" + recipe.ID().String() + "'")
		assert.Equal(suite.T(), 0, ingredientCount, "Related ingredients should be deleted")
	})
}

// TestRecipeRepositoryCount tests recipe counting
func (suite *RecipeRepositoryIntegrationTestSuite) TestRecipeRepositoryCount() {
	suite.Run("Count_WithRecipes_ShouldReturnCorrectCount", func() {
		// Arrange
		initialCount, err := suite.repository.Count(suite.ctx)
		require.NoError(suite.T(), err)
		
		// Add 3 new recipes
		for i := 0; i < 3; i++ {
			recipe, _ := suite.recipeFactory.CreateValidRecipe()
			suite.repository.Save(suite.ctx, recipe)
		}

		// Act
		finalCount, err := suite.repository.Count(suite.ctx)

		// Assert
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), initialCount+3, finalCount, "Count should increase by 3")
	})

	suite.Run("Count_EmptyDatabase_ShouldReturnZero", func() {
		// Arrange - Clean all recipes
		suite.testDB.TruncateAllTables()

		// Act
		count, err := suite.repository.Count(suite.ctx)

		// Assert
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), int64(0), count, "Empty database should return zero count")
	})
}

// TestConcurrentAccess tests concurrent repository access
func (suite *RecipeRepositoryIntegrationTestSuite) TestConcurrentAccess() {
	suite.Run("ConcurrentSave_ShouldHandleMultipleWrites", func() {
		// This test would verify that concurrent writes are handled properly
		// and that database locks/transactions work correctly
		
		const numConcurrentWrites = 10
		errors := make(chan error, numConcurrentWrites)
		
		// Launch concurrent saves
		for i := 0; i < numConcurrentWrites; i++ {
			go func(index int) {
				recipe, err := testutils.NewRecipeBuilder().
					WithTitle(fmt.Sprintf("Concurrent Recipe %d", index)).
					BuildValid()
				if err != nil {
					errors <- err
					return
				}
				
				err = suite.repository.Save(suite.ctx, recipe)
				errors <- err
			}(i)
		}
		
		// Collect all errors
		for i := 0; i < numConcurrentWrites; i++ {
			err := <-errors
			assert.NoError(suite.T(), err, "Concurrent save should not fail")
		}
		
		// Verify all recipes were saved
		count, err := suite.repository.Count(suite.ctx)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), int64(numConcurrentWrites+2), count, "All concurrent saves should succeed") // +2 from seed data
	})
}

// TestPerformance tests repository performance characteristics
func (suite *RecipeRepositoryIntegrationTestSuite) TestPerformance() {
	suite.Run("LargeDataset_ShouldMaintainPerformance", func() {
		// Skip if not running performance tests
		if testing.Short() {
			suite.T().Skip("Skipping performance test in short mode")
		}
		
		// Arrange - Create 1000 recipes
		const numRecipes = 1000
		for i := 0; i < numRecipes; i++ {
			recipe, _ := suite.recipeFactory.CreateValidRecipe()
			suite.repository.Save(suite.ctx, recipe)
		}
		
		// Act - Time a search operation
		start := time.Now()
		recipes, err := suite.repository.FindPublished(suite.ctx, 50, 0)
		duration := time.Since(start)
		
		// Assert
		require.NoError(suite.T(), err)
		assert.NotEmpty(suite.T(), recipes)
		
		// Performance assertion - should complete within reasonable time
		suite.assertions.Performance.ResponseTime(duration, 100*time.Millisecond, 
			"Search should complete within 100ms even with large dataset")
	})
}

// BenchmarkRecipeRepositoryOperations benchmarks repository operations
func BenchmarkRecipeRepositoryOperations(b *testing.B) {
	// Setup
	testDB := testutils.SetupTestDatabase(&testing.T{})
	defer testDB.Cleanup()
	
	testDB.RunMigrations()
	repository := postgres.NewRecipeRepository(testDB.GormDB)
	factory := testutils.NewRecipeFactory(time.Now().UnixNano())
	ctx := context.Background()
	
	b.Run("Save", func(b *testing.B) {
		recipes := make([]*recipe.Recipe, b.N)
		for i := 0; i < b.N; i++ {
			recipe, _ := factory.CreateValidRecipe()
			recipes[i] = recipe
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			repository.Save(ctx, recipes[i])
		}
	})
	
	b.Run("FindByID", func(b *testing.B) {
		// Setup test data
		recipe, _ := factory.CreateValidRecipe()
		repository.Save(ctx, recipe)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			repository.FindByID(ctx, recipe.ID())
		}
	})
	
	b.Run("Search", func(b *testing.B) {
		// Setup test data
		for i := 0; i < 100; i++ {
			recipe, _ := factory.CreateValidRecipe()
			repository.Save(ctx, recipe)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			repository.Search(ctx, "recipe", nil, 10, 0)
		}
	})
}

// TestRecipeRepositoryIntegrationTestSuite runs the integration test suite
func TestRecipeRepositoryIntegrationTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}
	
	suite.Run(t, new(RecipeRepositoryIntegrationTestSuite))
}