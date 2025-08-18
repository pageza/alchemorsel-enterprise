// Package performance provides comprehensive performance testing and benchmarks
//go:build performance
// +build performance

package performance

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/postgres"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Performance test configuration
const (
	SmallDataset  = 100
	MediumDataset = 1000
	LargeDataset  = 10000
	
	// Performance targets (adjust based on requirements)
	MaxResponseTime       = 100 * time.Millisecond
	MaxDatabaseQueryTime  = 50 * time.Millisecond
	MaxMemoryIncreaseMB   = 100
	MaxCPUUsagePercent    = 80
)

// PerformanceMetrics holds performance measurement data
type PerformanceMetrics struct {
	StartTime     time.Time
	EndTime       time.Time
	Duration      time.Duration
	MemoryBefore  runtime.MemStats
	MemoryAfter   runtime.MemStats
	AllocsBefore  uint64
	AllocsAfter   uint64
	GCCycles      uint32
}

// NewPerformanceMetrics creates a new performance metrics instance
func NewPerformanceMetrics() *PerformanceMetrics {
	pm := &PerformanceMetrics{
		StartTime: time.Now(),
	}
	runtime.GC() // Force GC before measurement
	runtime.ReadMemStats(&pm.MemoryBefore)
	pm.AllocsBefore = pm.MemoryBefore.Mallocs
	return pm
}

// Stop stops performance measurement and calculates metrics
func (pm *PerformanceMetrics) Stop() {
	pm.EndTime = time.Now()
	pm.Duration = pm.EndTime.Sub(pm.StartTime)
	runtime.ReadMemStats(&pm.MemoryAfter)
	pm.AllocsAfter = pm.MemoryAfter.Mallocs
	pm.GCCycles = pm.MemoryAfter.NumGC - pm.MemoryBefore.NumGC
}

// MemoryUsedMB returns memory usage in MB
func (pm *PerformanceMetrics) MemoryUsedMB() float64 {
	return float64(pm.MemoryAfter.Alloc-pm.MemoryBefore.Alloc) / 1024 / 1024
}

// AllocationsPerOp returns allocations per operation
func (pm *PerformanceMetrics) AllocationsPerOp(operations int) uint64 {
	if operations == 0 {
		return 0
	}
	return (pm.AllocsAfter - pm.AllocsBefore) / uint64(operations)
}

// Domain Layer Performance Tests

func BenchmarkRecipeCreation(b *testing.B) {
	factory := testutils.NewRecipeFactory(time.Now().UnixNano())
	
	b.Run("Simple", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			recipe, err := recipe.NewRecipe(
				"Test Recipe",
				"A test recipe",
				uuid.New(),
			)
			if err != nil {
				b.Fatal(err)
			}
			_ = recipe
		}
	})
	
	b.Run("WithIngredients", func(b *testing.B) {
		ingredients := []recipe.Ingredient{
			{ID: uuid.New(), Name: "Ingredient 1", Quantity: 1.0, Unit: "cup"},
			{ID: uuid.New(), Name: "Ingredient 2", Quantity: 2.0, Unit: "tbsp"},
			{ID: uuid.New(), Name: "Ingredient 3", Quantity: 0.5, Unit: "lb"},
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			recipe, err := factory.CreateSimpleRecipe()
			if err != nil {
				b.Fatal(err)
			}
			
			for _, ingredient := range ingredients {
				err := recipe.AddIngredient(ingredient)
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	
	b.Run("ComplexRecipe", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			recipe, err := factory.CreateComplexRecipe()
			if err != nil {
				b.Fatal(err)
			}
			_ = recipe
		}
	})
}

func BenchmarkRecipeOperations(b *testing.B) {
	factory := testutils.NewRecipeFactory(time.Now().UnixNano())
	
	// Pre-create recipes for operations
	recipes := make([]*recipe.Recipe, b.N)
	for i := 0; i < b.N; i++ {
		recipe, _ := factory.CreateValidRecipe()
		recipes[i] = recipe
	}
	
	b.Run("Publishing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := recipes[i].Publish()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("AddRating", func(b *testing.B) {
		rating := recipe.Rating{
			ID:     uuid.New(),
			UserID: uuid.New(),
			Value:  5,
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := recipes[i%len(recipes)].AddRating(rating)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("Like", func(b *testing.B) {
		userID := uuid.New()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			recipes[i%len(recipes)].Like(userID)
		}
	})
}

// Repository Performance Tests

func BenchmarkRecipeRepository(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping repository benchmarks in short mode")
	}
	
	// Setup test database
	testDB := testutils.SetupTestDatabase(&testing.T{})
	defer testDB.Cleanup()
	
	err := testDB.RunMigrations()
	require.NoError(b, err)
	
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
			err := repository.Save(ctx, recipes[i])
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("FindByID", func(b *testing.B) {
		// Pre-populate database
		recipes := make([]*recipe.Recipe, 1000)
		for i := 0; i < 1000; i++ {
			recipe, _ := factory.CreateValidRecipe()
			repository.Save(ctx, recipe)
			recipes[i] = recipe
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			recipe := recipes[i%len(recipes)]
			_, err := repository.FindByID(ctx, recipe.ID())
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("Search", func(b *testing.B) {
		// Pre-populate database
		for i := 0; i < 1000; i++ {
			recipe, _ := factory.CreateValidRecipe()
			repository.Save(ctx, recipe)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := repository.Search(ctx, "recipe", nil, 10, 0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("FindPublished", func(b *testing.B) {
		// Pre-populate with published recipes
		for i := 0; i < 1000; i++ {
			recipe, _ := factory.CreateValidRecipe()
			recipe.Publish()
			repository.Save(ctx, recipe)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := repository.FindPublished(ctx, 10, 0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Security Performance Tests

func BenchmarkSecurityOperations(b *testing.B) {
	config := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:  "test-secret-key-for-performance-testing-32-bytes",
			BCryptCost: 10, // Realistic cost
		},
	}
	
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   6,
	})
	
	authService := security.NewAuthService(config, zap.NewNop(), redisClient)
	
	b.Run("PasswordHashing", func(b *testing.B) {
		passwords := make([]string, b.N)
		for i := 0; i < b.N; i++ {
			passwords[i] = fmt.Sprintf("Password%d!", i)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := authService.HashPassword(passwords[i])
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("PasswordVerification", func(b *testing.B) {
		password := "TestPassword123!"
		hash, _ := authService.HashPassword(password)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := authService.VerifyPassword(hash, password)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("TokenGeneration", func(b *testing.B) {
		userID := uuid.New().String()
		email := "test@example.com"
		roles := []string{"user"}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sessionID := uuid.New().String()
			_, err := authService.GenerateAccessToken(
				userID, email, roles, sessionID, "192.168.1.1", "Test Browser",
			)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("TokenValidation", func(b *testing.B) {
		// Pre-generate tokens
		tokens := make([]string, b.N)
		userID := uuid.New().String()
		
		for i := 0; i < b.N; i++ {
			sessionID := uuid.New().String()
			token, _ := authService.GenerateAccessToken(
				userID, "test@example.com", []string{"user"}, 
				sessionID, "192.168.1.1", "Test Browser",
			)
			tokens[i] = token
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := authService.ValidateToken(tokens[i], security.AccessToken)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("SessionCreation", func(b *testing.B) {
		userID := uuid.New().String()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := authService.CreateSession(userID, "192.168.1.1", "Test Browser")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("SessionValidation", func(b *testing.B) {
		// Pre-create sessions
		sessions := make([]string, b.N)
		userID := uuid.New().String()
		
		for i := 0; i < b.N; i++ {
			session, _ := authService.CreateSession(userID, "192.168.1.1", "Test Browser")
			sessions[i] = session.SessionID
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := authService.ValidateSession(sessions[i], userID, "192.168.1.1")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Concurrent Performance Tests

func BenchmarkConcurrentOperations(b *testing.B) {
	factory := testutils.NewRecipeFactory(time.Now().UnixNano())
	
	b.Run("RecipeCreation", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				recipe, err := factory.CreateValidRecipe()
				if err != nil {
					b.Fatal(err)
				}
				_ = recipe
			}
		})
	})
	
	b.Run("RecipeOperations", func(b *testing.B) {
		// Pre-create recipes
		recipes := make([]*recipe.Recipe, 1000)
		for i := 0; i < 1000; i++ {
			recipe, _ := factory.CreateValidRecipe()
			recipes[i] = recipe
		}
		
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				recipe := recipes[i%len(recipes)]
				recipe.Like(uuid.New())
				i++
			}
		})
	})
}

// Memory Performance Tests

func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("RecipeCreation_MemoryUsage", func(b *testing.B) {
		factory := testutils.NewRecipeFactory(time.Now().UnixNano())
		
		// Measure memory usage
		metrics := NewPerformanceMetrics()
		
		recipes := make([]*recipe.Recipe, b.N)
		for i := 0; i < b.N; i++ {
			recipe, err := factory.CreateValidRecipe()
			if err != nil {
				b.Fatal(err)
			}
			recipes[i] = recipe
		}
		
		metrics.Stop()
		
		b.ReportMetric(metrics.MemoryUsedMB(), "MB/op")
		b.ReportMetric(float64(metrics.AllocationsPerOp(b.N)), "allocs/op")
		
		// Keep recipes in memory to prevent GC
		_ = recipes
	})
	
	b.Run("LargeDataset_MemoryGrowth", func(b *testing.B) {
		if testing.Short() {
			b.Skip("Skipping memory test in short mode")
		}
		
		factory := testutils.NewRecipeFactory(time.Now().UnixNano())
		recipes := make([]*recipe.Recipe, 0, LargeDataset)
		
		metrics := NewPerformanceMetrics()
		
		for i := 0; i < LargeDataset; i++ {
			recipe, _ := factory.CreateComplexRecipe()
			recipes = append(recipes, recipe)
		}
		
		metrics.Stop()
		
		b.ReportMetric(metrics.MemoryUsedMB(), "MB")
		b.ReportMetric(float64(len(recipes)), "recipes")
		
		memoryPerRecipe := metrics.MemoryUsedMB() / float64(len(recipes))
		b.ReportMetric(memoryPerRecipe*1024, "KB/recipe")
		
		// Ensure we don't exceed memory limits
		if metrics.MemoryUsedMB() > MaxMemoryIncreaseMB {
			b.Errorf("Memory usage exceeded limit: %.2f MB > %d MB", 
				metrics.MemoryUsedMB(), MaxMemoryIncreaseMB)
		}
	})
}

// Performance Regression Tests

func TestPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression tests in short mode")
	}
	
	// These tests would typically compare against baseline measurements
	// stored in a file or database
	
	t.Run("RecipeCreation_Performance", func(t *testing.T) {
		factory := testutils.NewRecipeFactory(time.Now().UnixNano())
		
		start := time.Now()
		
		for i := 0; i < 1000; i++ {
			recipe, err := factory.CreateValidRecipe()
			require.NoError(t, err)
			_ = recipe
		}
		
		duration := time.Since(start)
		avgTime := duration / 1000
		
		if avgTime > MaxResponseTime/10 { // Allow 10ms per recipe creation
			t.Errorf("Recipe creation too slow: %v > %v", avgTime, MaxResponseTime/10)
		}
	})
	
	t.Run("DatabaseOperations_Performance", func(t *testing.T) {
		testDB := testutils.SetupTestDatabase(t)
		defer testDB.Cleanup()
		
		err := testDB.RunMigrations()
		require.NoError(t, err)
		
		repository := postgres.NewRecipeRepository(testDB.GormDB)
		factory := testutils.NewRecipeFactory(time.Now().UnixNano())
		ctx := context.Background()
		
		// Test save performance
		recipe, _ := factory.CreateValidRecipe()
		
		start := time.Now()
		err = repository.Save(ctx, recipe)
		saveTime := time.Since(start)
		
		require.NoError(t, err)
		if saveTime > MaxDatabaseQueryTime {
			t.Errorf("Database save too slow: %v > %v", saveTime, MaxDatabaseQueryTime)
		}
		
		// Test find performance
		start = time.Now()
		_, err = repository.FindByID(ctx, recipe.ID())
		findTime := time.Since(start)
		
		require.NoError(t, err)
		if findTime > MaxDatabaseQueryTime {
			t.Errorf("Database find too slow: %v > %v", findTime, MaxDatabaseQueryTime)
		}
	})
	
	t.Run("SecurityOperations_Performance", func(t *testing.T) {
		config := &config.Config{
			Auth: config.AuthConfig{
				JWTSecret:  "test-secret-key-for-performance-testing-32-bytes",
				BCryptCost: 10,
			},
		}
		
		redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 7})
		authService := security.NewAuthService(config, zap.NewNop(), redisClient)
		
		// Test password hashing performance
		start := time.Now()
		_, err := authService.HashPassword("TestPassword123!")
		hashTime := time.Since(start)
		
		require.NoError(t, err)
		if hashTime > 500*time.Millisecond { // Bcrypt is inherently slow
			t.Errorf("Password hashing too slow: %v > 500ms", hashTime)
		}
		
		// Test token generation performance
		start = time.Now()
		userID := uuid.New().String()
		_, err = authService.GenerateAccessToken(
			userID, "test@example.com", []string{"user"}, 
			uuid.New().String(), "192.168.1.1", "Test Browser",
		)
		tokenTime := time.Since(start)
		
		require.NoError(t, err)
		if tokenTime > 10*time.Millisecond {
			t.Errorf("Token generation too slow: %v > 10ms", tokenTime)
		}
	})
}

// Stress Tests

func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress tests in short mode")
	}
	
	t.Run("HighVolumeRecipeCreation", func(t *testing.T) {
		factory := testutils.NewRecipeFactory(time.Now().UnixNano())
		
		const numRecipes = 10000
		const numWorkers = 10
		
		recipes := make(chan *recipe.Recipe, numRecipes)
		errors := make(chan error, numRecipes)
		
		// Start workers
		for i := 0; i < numWorkers; i++ {
			go func() {
				for j := 0; j < numRecipes/numWorkers; j++ {
					recipe, err := factory.CreateValidRecipe()
					if err != nil {
						errors <- err
						return
					}
					recipes <- recipe
				}
			}()
		}
		
		// Collect results
		createdRecipes := make([]*recipe.Recipe, 0, numRecipes)
		for i := 0; i < numRecipes; i++ {
			select {
			case recipe := <-recipes:
				createdRecipes = append(createdRecipes, recipe)
			case err := <-errors:
				t.Fatal(err)
			case <-time.After(30 * time.Second):
				t.Fatal("Stress test timed out")
			}
		}
		
		require.Len(t, createdRecipes, numRecipes)
	})
	
	t.Run("MemoryStress", func(t *testing.T) {
		factory := testutils.NewRecipeFactory(time.Now().UnixNano())
		
		var beforeMem runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&beforeMem)
		
		// Create many recipes to stress memory
		recipes := make([]*recipe.Recipe, LargeDataset)
		for i := 0; i < LargeDataset; i++ {
			recipe, err := factory.CreateComplexRecipe()
			require.NoError(t, err)
			recipes[i] = recipe
		}
		
		var afterMem runtime.MemStats
		runtime.ReadMemStats(&afterMem)
		
		memoryUsedMB := float64(afterMem.Alloc-beforeMem.Alloc) / 1024 / 1024
		
		t.Logf("Created %d recipes using %.2f MB of memory (%.2f KB per recipe)",
			len(recipes), memoryUsedMB, memoryUsedMB*1024/float64(len(recipes)))
		
		// Verify memory usage is reasonable
		if memoryUsedMB > MaxMemoryIncreaseMB {
			t.Errorf("Memory usage too high: %.2f MB > %d MB", memoryUsedMB, MaxMemoryIncreaseMB)
		}
	})
}

// Helper function to run performance tests with timeout
func runWithTimeout(t *testing.T, timeout time.Duration, fn func()) {
	done := make(chan bool, 1)
	
	go func() {
		fn()
		done <- true
	}()
	
	select {
	case <-done:
		// Test completed successfully
	case <-time.After(timeout):
		t.Fatal("Test timed out")
	}
}

// Performance comparison test
func TestPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance comparison tests in short mode")
	}
	
	factory := testutils.NewRecipeFactory(time.Now().UnixNano())
	
	t.Run("RecipeCreation_Simple_vs_Complex", func(t *testing.T) {
		const iterations = 1000
		
		// Measure simple recipe creation
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_, err := factory.CreateSimpleRecipe()
			require.NoError(t, err)
		}
		simpleTime := time.Since(start)
		
		// Measure complex recipe creation
		start = time.Now()
		for i := 0; i < iterations; i++ {
			_, err := factory.CreateComplexRecipe()
			require.NoError(t, err)
		}
		complexTime := time.Since(start)
		
		ratio := float64(complexTime) / float64(simpleTime)
		
		t.Logf("Simple recipe creation: %v", simpleTime/iterations)
		t.Logf("Complex recipe creation: %v", complexTime/iterations)
		t.Logf("Complex/Simple ratio: %.2fx", ratio)
		
		// Complex recipes shouldn't be more than 10x slower
		if ratio > 10 {
			t.Errorf("Complex recipes too much slower than simple: %.2fx > 10x", ratio)
		}
	})
}