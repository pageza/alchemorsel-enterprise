// Dashboard helper functions for webserver
package webserver

import (
	"context"
	"go.uber.org/zap"
)

// Dashboard data structures for webserver
type UserDashboardData struct {
	RecipeStats       *UserRecipeStatsData       `json:"recipe_stats"`
	ConversationStats *UserConversationStatsData `json:"conversation_stats"`
	RecentRecipes     []RecipeResponse           `json:"recent_recipes"`
	FeaturedRecipes   []RecipeResponse           `json:"featured_recipes"`
	QuickActions      []QuickActionData          `json:"quick_actions"`
}

type UserRecipeStatsData struct {
	TotalRecipes     int     `json:"total_recipes"`
	PublishedRecipes int     `json:"published_recipes"`
	TotalLikes       int     `json:"total_likes"`
	TotalViews       int     `json:"total_views"`
	AvgRating        float64 `json:"avg_rating"`
}

type UserConversationStatsData struct {
	TotalConversations  int `json:"total_conversations"`
	ActiveConversations int `json:"active_conversations"`
	RecipesGenerated    int `json:"recipes_generated"`
}

type QuickActionData struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	URL         string `json:"url"`
}

// buildUserDashboard constructs dashboard data for authenticated users
func (s *WebServer) buildUserDashboard(ctx context.Context, user UserResponse, accessToken string) (*UserDashboardData, error) {
	dashboard := &UserDashboardData{}

	// Fetch user recipes
	recipes, err := s.apiClient.GetRecipes(ctx, accessToken)
	if err != nil {
		s.logger.Warn("Failed to fetch user recipes for dashboard",
			zap.Error(err),
			zap.String("user_id", user.ID),
		)
		// Continue with partial data
		recipes = []RecipeResponse{}
	} else {
		// Calculate recipe statistics
		dashboard.RecipeStats = s.calculateRecipeStatsFromAPI(recipes)
		dashboard.RecentRecipes = s.limitRecipes(recipes, 6)
	}

	// Get conversation statistics from the conversation service (if available)
	if s.convService != nil {
		convStats, err := s.convService.GetConversationStats(ctx, user.ID)
		if err != nil {
			s.logger.Warn("Failed to fetch conversation stats for dashboard",
				zap.Error(err),
				zap.String("user_id", user.ID),
			)
		} else {
			dashboard.ConversationStats = s.buildConversationStatsFromMap(convStats)
		}
	}

	// Get featured recipes (public recipes for inspiration)
	featuredRecipes, err := s.getFeaturedRecipes(ctx)
	if err != nil {
		s.logger.Warn("Failed to fetch featured recipes for dashboard", zap.Error(err))
	} else {
		dashboard.FeaturedRecipes = s.limitRecipes(featuredRecipes, 6)
	}

	// Build quick actions
	dashboard.QuickActions = []QuickActionData{
		{
			Title:       "Create Recipe",
			Description: "Add a new recipe to your collection",
			Icon:        "➕",
			URL:         "/recipes/new",
		},
		{
			Title:       "AI Chef Chat",
			Description: "Get cooking help from AI",
			Icon:        "🤖",
			URL:         "/ai/chat",
		},
		{
			Title:       "My Recipes",
			Description: "Browse your recipe collection",
			Icon:        "📚",
			URL:         "/recipes?author=" + user.ID,
		},
		{
			Title:       "Favorites",
			Description: "View your saved recipes",
			Icon:        "❤️",
			URL:         "/favorites",
		},
	}

	return dashboard, nil
}

// calculateRecipeStatsFromAPI computes statistics from API recipe responses
func (s *WebServer) calculateRecipeStatsFromAPI(recipes []RecipeResponse) *UserRecipeStatsData {
	stats := &UserRecipeStatsData{}

	totalLikes := 0
	totalViews := 0 // Note: views not in current RecipeResponse but we'll calculate from likes for now
	totalRating := 0.0
	publishedCount := len(recipes) // Assume all fetched recipes are published

	for _, recipe := range recipes {
		totalLikes += recipe.Likes
		totalViews += recipe.Likes * 3 // Estimate views as 3x likes for now
		totalRating += recipe.Rating
	}

	stats.TotalRecipes = len(recipes)
	stats.PublishedRecipes = publishedCount
	stats.TotalLikes = totalLikes
	stats.TotalViews = totalViews

	if len(recipes) > 0 {
		stats.AvgRating = totalRating / float64(len(recipes))
	}

	return stats
}

// buildConversationStatsFromMap converts conversation service stats to dashboard format
func (s *WebServer) buildConversationStatsFromMap(stats map[string]interface{}) *UserConversationStatsData {
	convStats := &UserConversationStatsData{}

	if totalConv, ok := stats["total_conversations"].(int); ok {
		convStats.TotalConversations = totalConv
	}

	if activeConv, ok := stats["active_conversations"].(int); ok {
		convStats.ActiveConversations = activeConv
	}

	// Count recipe-related conversations as recipes generated
	if intents, ok := stats["intents"].(map[string]interface{}); ok {
		if recipeCount, ok := intents["recipe_generation"].(int); ok {
			convStats.RecipesGenerated = recipeCount
		}
	}

	return convStats
}

// getFeaturedRecipes fetches featured recipes for display
func (s *WebServer) getFeaturedRecipes(ctx context.Context) ([]RecipeResponse, error) {
	// For now, we'll use the existing GetRecipes method without auth to get public recipes
	// In a real implementation, this would call a separate endpoint for trending/featured recipes
	recipes, err := s.apiClient.GetRecipes(ctx, "")
	if err != nil {
		// Return empty slice instead of error to not break the page
		return []RecipeResponse{}, nil
	}

	return s.limitRecipes(recipes, 6), nil
}

// limitRecipes limits the number of recipes returned
func (s *WebServer) limitRecipes(recipes []RecipeResponse, limit int) []RecipeResponse {
	if len(recipes) <= limit {
		return recipes
	}
	return recipes[:limit]
}

// mapToUserResponse converts a map[string]interface{} to UserResponse
func (s *WebServer) mapToUserResponse(userMap map[string]interface{}) UserResponse {
	user := UserResponse{}

	if id, ok := userMap["ID"].(string); ok {
		user.ID = id
	}

	if name, ok := userMap["Name"].(string); ok {
		user.Name = name
	}

	if email, ok := userMap["Email"].(string); ok {
		user.Email = email
	}

	if role, ok := userMap["Role"].(string); ok {
		user.Role = role
	}

	if isActive, ok := userMap["IsActive"].(bool); ok {
		user.IsActive = isActive
	}

	return user
}
