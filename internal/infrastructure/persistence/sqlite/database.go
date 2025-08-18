// Package sqlite provides SQLite database setup and configuration
package sqlite

import (
	"fmt"

	gormModels "github.com/alchemorsel/v3/internal/infrastructure/persistence/gorm"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupDatabase creates and configures the SQLite database
func SetupDatabase(dbPath string, logLevel logger.LogLevel) (*gorm.DB, error) {
	// Use in-memory database if no path provided
	if dbPath == "" {
		dbPath = ":memory:"
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run auto-migration
	err = db.AutoMigrate(
		&gormModels.UserModel{},
		&gormModels.RecipeModel{},
		&gormModels.RatingModel{},
		&gormModels.AIRequestModel{},
		&gormModels.RecipeLikeModel{},
		&gormModels.UserFollowModel{},
		&gormModels.CollectionModel{},
		&gormModels.CollectionRecipeModel{},
		&gormModels.CommentModel{},
		&gormModels.ActivityModel{},
		&gormModels.RecipeViewModel{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// SeedDatabase populates the database with initial data
func SeedDatabase(db *gorm.DB) error {
	// Check if data already exists
	var userCount int64
	db.Model(&gormModels.UserModel{}).Count(&userCount)
	if userCount > 0 {
		return nil // Already seeded
	}

	// Create demo users
	demoUsers := []gormModels.UserModel{
		{
			Email:        "chef@alchemorsel.com",
			Name:         "Chef Demo",
			PasswordHash: "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi", // password
			IsActive:     true,
			IsVerified:   true,
			Role:         "chef",
			Profile: &gormModels.UserProfileModel{
				FirstName:    "Chef",
				LastName:     "Demo",
				Bio:          "Professional chef with 10+ years of experience in French cuisine",
				CookingLevel: "professional",
			},
			Preferences: &gormModels.UserPreferencesModel{
				DietaryRestrictions: []string{},
				PreferredCuisines:   []string{"french", "italian"},
				MeasurementSystem:   "metric",
				Language:            "en",
				EmailNotifications:  true,
				PushNotifications:   true,
			},
		},
		{
			Email:        "user@alchemorsel.com",
			Name:         "Home Cook",
			PasswordHash: "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi", // password
			IsActive:     true,
			IsVerified:   true,
			Role:         "user",
			Profile: &gormModels.UserProfileModel{
				FirstName:    "Home",
				LastName:     "Cook",
				Bio:          "Passionate home cook who loves trying new recipes",
				CookingLevel: "intermediate",
			},
			Preferences: &gormModels.UserPreferencesModel{
				DietaryRestrictions: []string{"vegetarian"},
				PreferredCuisines:   []string{"italian", "asian"},
				MeasurementSystem:   "imperial",
				Language:            "en",
				EmailNotifications:  true,
				PushNotifications:   false,
			},
		},
	}

	// Create users
	for _, user := range demoUsers {
		if err := db.Create(&user).Error; err != nil {
			return fmt.Errorf("failed to create demo user: %w", err)
		}
	}

	// Create demo recipes
	demoRecipes := []gormModels.RecipeModel{
		{
			Title:       "Classic Spaghetti Carbonara",
			Description: "A traditional Italian pasta dish with eggs, cheese, pancetta, and pepper",
			AuthorID:    demoUsers[0].ID,
			Ingredients: gormModels.JSONField{
				"ingredients": []map[string]interface{}{
					{
						"name":     "Spaghetti",
						"amount":   400,
						"unit":     "g",
						"optional": false,
					},
					{
						"name":     "Pancetta",
						"amount":   150,
						"unit":     "g",
						"optional": false,
					},
					{
						"name":     "Eggs",
						"amount":   4,
						"unit":     "piece",
						"optional": false,
					},
					{
						"name":     "Pecorino Romano",
						"amount":   100,
						"unit":     "g",
						"optional": false,
					},
				},
			},
			Instructions: gormModels.JSONField{
				"instructions": []map[string]interface{}{
					{
						"step_number": 1,
						"description": "Bring a large pot of salted water to boil and cook spaghetti according to package directions",
						"duration":    10,
					},
					{
						"step_number": 2,
						"description": "Meanwhile, cook pancetta in a large skillet until crispy",
						"duration":    5,
					},
					{
						"step_number": 3,
						"description": "In a bowl, whisk together eggs and grated cheese",
						"duration":    2,
					},
					{
						"step_number": 4,
						"description": "Drain pasta and immediately toss with pancetta and egg mixture",
						"duration":    2,
					},
				},
			},
			Cuisine:          "italian",
			Category:         "main_course",
			Difficulty:       "medium",
			Tags:             []string{"pasta", "traditional", "quick"},
			PrepTimeMinutes:  10,
			CookTimeMinutes:  15,
			TotalTimeMinutes: 25,
			Servings:         4,
			Calories:         650,
			Status:           "published",
			Likes:            42,
			Views:            156,
			AverageRating:    4.8,
		},
		{
			Title:       "Vegetarian Buddha Bowl",
			Description: "A nutritious and colorful bowl with quinoa, roasted vegetables, and tahini dressing",
			AuthorID:    demoUsers[1].ID,
			Ingredients: gormModels.JSONField{
				"ingredients": []map[string]interface{}{
					{
						"name":     "Quinoa",
						"amount":   1,
						"unit":     "cup",
						"optional": false,
					},
					{
						"name":     "Sweet potato",
						"amount":   2,
						"unit":     "piece",
						"optional": false,
					},
					{
						"name":     "Chickpeas",
						"amount":   1,
						"unit":     "cup",
						"optional": false,
					},
					{
						"name":     "Avocado",
						"amount":   1,
						"unit":     "piece",
						"optional": false,
					},
				},
			},
			Instructions: gormModels.JSONField{
				"instructions": []map[string]interface{}{
					{
						"step_number": 1,
						"description": "Cook quinoa according to package instructions",
						"duration":    15,
					},
					{
						"step_number": 2,
						"description": "Roast sweet potato cubes at 400°F for 25 minutes",
						"duration":    25,
					},
					{
						"step_number": 3,
						"description": "Prepare tahini dressing by mixing tahini, lemon juice, and water",
						"duration":    5,
					},
					{
						"step_number": 4,
						"description": "Assemble bowl with quinoa, vegetables, and dressing",
						"duration":    5,
					},
				},
			},
			Cuisine:          "american",
			Category:         "main_course",
			Difficulty:       "easy",
			Tags:             []string{"vegetarian", "healthy", "bowl"},
			PrepTimeMinutes:  15,
			CookTimeMinutes:  30,
			TotalTimeMinutes: 45,
			Servings:         2,
			Calories:         480,
			Status:           "published",
			Likes:            28,
			Views:            89,
			AverageRating:    4.6,
		},
		{
			Title:       "AI-Generated Fusion Tacos",
			Description: "Creative fusion tacos combining Korean and Mexican flavors, generated by AI",
			AuthorID:    demoUsers[0].ID,
			AIGenerated: true,
			AIPrompt:    "Create a fusion recipe combining Korean BBQ flavors with Mexican tacos",
			AIModel:     "recipe-generator-v1",
			Ingredients: gormModels.JSONField{
				"ingredients": []map[string]interface{}{
					{
						"name":     "Corn tortillas",
						"amount":   8,
						"unit":     "piece",
						"optional": false,
					},
					{
						"name":     "Bulgogi beef",
						"amount":   300,
						"unit":     "g",
						"optional": false,
					},
					{
						"name":     "Kimchi",
						"amount":   0.5,
						"unit":     "cup",
						"optional": false,
					},
					{
						"name":     "Cilantro",
						"amount":   0.25,
						"unit":     "cup",
						"optional": true,
					},
				},
			},
			Instructions: gormModels.JSONField{
				"instructions": []map[string]interface{}{
					{
						"step_number": 1,
						"description": "Marinate beef in Korean BBQ sauce for 30 minutes",
						"duration":    30,
					},
					{
						"step_number": 2,
						"description": "Grill beef until cooked through, about 3-4 minutes per side",
						"duration":    8,
					},
					{
						"step_number": 3,
						"description": "Warm tortillas on a griddle",
						"duration":    2,
					},
					{
						"step_number": 4,
						"description": "Assemble tacos with beef, kimchi, and cilantro",
						"duration":    5,
					},
				},
			},
			Cuisine:          "fusion",
			Category:         "main_course",
			Difficulty:       "medium",
			Tags:             []string{"fusion", "tacos", "korean", "ai-generated"},
			PrepTimeMinutes:  35,
			CookTimeMinutes:  10,
			TotalTimeMinutes: 45,
			Servings:         4,
			Calories:         320,
			Status:           "published",
			Likes:            15,
			Views:            67,
			AverageRating:    4.3,
		},
	}

	// Create recipes
	for _, recipe := range demoRecipes {
		if err := db.Create(&recipe).Error; err != nil {
			return fmt.Errorf("failed to create demo recipe: %w", err)
		}
	}

	// Create social data - likes, follows, collections, comments
	
	// User follows
	follows := []gormModels.UserFollowModel{
		{
			FollowerID:  demoUsers[1].ID, // Home Cook follows Chef
			FollowingID: demoUsers[0].ID,
		},
	}
	
	for _, follow := range follows {
		if err := db.Create(&follow).Error; err != nil {
			return fmt.Errorf("failed to create follow: %w", err)
		}
	}
	
	// Recipe likes
	likes := []gormModels.RecipeLikeModel{
		{
			RecipeID: demoRecipes[0].ID, // Home Cook likes Carbonara
			UserID:   demoUsers[1].ID,
		},
		{
			RecipeID: demoRecipes[2].ID, // Home Cook likes Fusion Tacos
			UserID:   demoUsers[1].ID,
		},
	}
	
	for _, like := range likes {
		if err := db.Create(&like).Error; err != nil {
			return fmt.Errorf("failed to create like: %w", err)
		}
	}
	
	// Collections
	collections := []gormModels.CollectionModel{
		{
			UserID:      demoUsers[1].ID,
			Name:        "Favorite Pasta",
			Description: "My collection of favorite pasta recipes",
			IsPublic:    true,
		},
		{
			UserID:      demoUsers[0].ID,
			Name:        "Professional Techniques",
			Description: "Advanced cooking techniques for professional chefs",
			IsPublic:    true,
		},
		{
			UserID:      demoUsers[1].ID,
			Name:        "Quick Weeknight Meals",
			Description: "Fast and easy recipes for busy weeknights",
			IsPublic:    false,
		},
	}
	
	for _, collection := range collections {
		if err := db.Create(&collection).Error; err != nil {
			return fmt.Errorf("failed to create collection: %w", err)
		}
	}
	
	// Collection recipes
	collectionRecipes := []gormModels.CollectionRecipeModel{
		{
			CollectionID: collections[0].ID, // Favorite Pasta collection
			RecipeID:     demoRecipes[0].ID, // Carbonara
			OrderIndex:   1,
		},
		{
			CollectionID: collections[2].ID, // Quick Weeknight Meals
			RecipeID:     demoRecipes[1].ID, // Buddha Bowl
			OrderIndex:   1,
		},
	}
	
	for _, collectionRecipe := range collectionRecipes {
		if err := db.Create(&collectionRecipe).Error; err != nil {
			return fmt.Errorf("failed to add recipe to collection: %w", err)
		}
	}
	
	// Recipe ratings
	ratings := []gormModels.RatingModel{
		{
			RecipeID: demoRecipes[0].ID,
			UserID:   demoUsers[1].ID,
			Value:    5,
			Comment:  "Absolutely delicious! Perfect carbonara recipe.",
		},
		{
			RecipeID: demoRecipes[1].ID,
			UserID:   demoUsers[0].ID,
			Value:    4,
			Comment:  "Great healthy option, very satisfying!",
		},
	}
	
	for _, rating := range ratings {
		if err := db.Create(&rating).Error; err != nil {
			return fmt.Errorf("failed to create rating: %w", err)
		}
	}
	
	// Comments
	comments := []gormModels.CommentModel{
		{
			RecipeID: demoRecipes[0].ID,
			UserID:   demoUsers[1].ID,
			Content:  "This is exactly how my Italian grandmother used to make it! Thank you for sharing this authentic recipe.",
		},
		{
			RecipeID: demoRecipes[1].ID,
			UserID:   demoUsers[0].ID,
			Content:  "Love the combination of flavors in this bowl. I added some roasted chickpeas for extra protein!",
		},
		{
			RecipeID: demoRecipes[2].ID,
			UserID:   demoUsers[1].ID,
			Content:  "What an interesting fusion! I never thought to combine Korean and Mexican flavors like this.",
		},
	}
	
	for _, comment := range comments {
		if err := db.Create(&comment).Error; err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}
	}
	
	// Activities (notifications)
	activities := []gormModels.ActivityModel{
		{
			UserID:      demoUsers[0].ID, // Chef receives notification
			ActorID:     demoUsers[1].ID, // Home Cook is the actor
			Type:        "recipe_liked",
			EntityType:  "recipe",
			EntityID:    demoRecipes[0].ID,
			Title:       "Recipe Liked",
			Description: "Home Cook liked your recipe 'Classic Spaghetti Carbonara'",
			Data: gormModels.JSONField{
				"recipe_title": "Classic Spaghetti Carbonara",
				"actor_name":   "Home Cook",
			},
			IsRead: false,
		},
		{
			UserID:      demoUsers[0].ID, // Chef receives notification
			ActorID:     demoUsers[1].ID, // Home Cook is the actor
			Type:        "user_followed",
			EntityType:  "user",
			EntityID:    demoUsers[1].ID,
			Title:       "New Follower",
			Description: "Home Cook started following you",
			Data: gormModels.JSONField{
				"actor_name": "Home Cook",
			},
			IsRead: false,
		},
		{
			UserID:      demoUsers[0].ID, // Chef receives notification
			ActorID:     demoUsers[1].ID, // Home Cook is the actor
			Type:        "recipe_commented",
			EntityType:  "recipe",
			EntityID:    demoRecipes[0].ID,
			Title:       "New Comment",
			Description: "Home Cook commented on your recipe 'Classic Spaghetti Carbonara'",
			Data: gormModels.JSONField{
				"recipe_title": "Classic Spaghetti Carbonara",
				"actor_name":   "Home Cook",
				"comment":      "This is exactly how my Italian grandmother used to make it!",
			},
			IsRead: false,
		},
	}
	
	for _, activity := range activities {
		if err := db.Create(&activity).Error; err != nil {
			return fmt.Errorf("failed to create activity: %w", err)
		}
	}

	return nil
}