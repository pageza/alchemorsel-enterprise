// Package gorm provides mapping between domain entities and GORM models
package gorm

import (
	"github.com/alchemorsel/v3/internal/domain/ai"
	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/domain/user"
)

// UserToModel converts a domain user to a GORM model
func UserToModel(u *user.User) *UserModel {
	model := &UserModel{
		ID:           u.ID(),
		Email:        u.Email(),
		Name:         u.Name(),
		IsActive:     u.IsActive(),
		IsVerified:   u.IsVerified(),
		Role:         string(u.Role()),
		CreatedAt:    u.CreatedAt(),
		UpdatedAt:    u.UpdatedAt(),
		LastLoginAt:  u.LastLoginAt(),
	}

	// Map profile if it exists
	if profile := u.Profile(); profile != nil {
		model.Profile = &UserProfileModel{
			FirstName:    profile.FirstName,
			LastName:     profile.LastName,
			Avatar:       profile.Avatar,
			Bio:          profile.Bio,
			Location:     profile.Location,
			Website:      profile.Website,
			Birthday:     profile.Birthday,
			CookingLevel: string(profile.CookingLevel),
		}
	}

	// Map preferences if they exist
	if prefs := u.Preferences(); prefs != nil {
		dietaryRestrictions := make([]string, len(prefs.DietaryRestrictions))
		for i, dr := range prefs.DietaryRestrictions {
			dietaryRestrictions[i] = string(dr)
		}

		model.Preferences = &UserPreferencesModel{
			DietaryRestrictions: dietaryRestrictions,
			Allergies:           prefs.Allergies,
			PreferredCuisines:   prefs.PreferredCuisines,
			DislikedIngredients: prefs.DislikedIngredients,
			MeasurementSystem:   string(prefs.MeasurementSystem),
			Language:            prefs.Language,
			Timezone:            prefs.Timezone,
			EmailNotifications:  prefs.EmailNotifications,
			PushNotifications:   prefs.PushNotifications,
		}
	}

	return model
}

// ModelToUser converts a GORM model to a domain user
func ModelToUser(model *UserModel) (*user.User, error) {
	// Create new user - we need to access private fields via reflection or constructor
	// For now, we'll use the constructor and then manually set fields
	u, err := user.NewUser(model.Email, model.Name, "dummy") // Password won't be used
	if err != nil {
		return nil, err
	}

	// We would need setters or reflection to properly map back
	// For now, this is a simplified approach
	return u, nil
}

// RecipeToModel converts a domain recipe to a GORM model
func RecipeToModel(r *recipe.Recipe) *RecipeModel {
	ingredientsJSON := convertIngredientsToJSON(r.Ingredients())
	instructionsJSON := convertInstructionsToJSON(r.Instructions())
	nutritionJSON := convertNutritionToJSON(r.NutritionInfo())
	imagesJSON := convertImagesToJSON(r.Images())
	videosJSON := convertVideosToJSON(r.Videos())

	tags := make([]string, len(r.Tags()))
	for i, tag := range r.Tags() {
		tags[i] = tag
	}

	return &RecipeModel{
		ID:               r.ID(),
		Version:          r.Version(),
		Title:            r.Title(),
		Description:      r.Description(),
		AuthorID:         r.AuthorID(),
		Ingredients:      JSONField(map[string]interface{}{"data": ingredientsJSON}),
		Instructions:     JSONField(map[string]interface{}{"data": instructionsJSON}),
		NutritionInfo:    JSONField(nutritionJSON),
		Cuisine:          string(r.Cuisine()),
		Category:         string(r.Category()),
		Difficulty:       string(r.Difficulty()),
		Tags:             tags,
		PrepTimeMinutes:  int(r.PrepTime().Minutes()),
		CookTimeMinutes:  int(r.CookTime().Minutes()),
		TotalTimeMinutes: int(r.TotalTime().Minutes()),
		Servings:         r.Servings(),
		Calories:         r.Calories(),
		AIGenerated:      r.IsAIGenerated(),
		AIPrompt:         r.AIPrompt(),
		AIModel:          r.AIModel(),
		Likes:            r.Likes(),
		Views:            r.Views(),
		AverageRating:    r.AverageRating(),
		Images:           JSONField(map[string]interface{}{"data": imagesJSON}),
		Videos:           JSONField(map[string]interface{}{"data": videosJSON}),
		Status:           string(r.Status()),
		PublishedAt:      r.PublishedAt(),
		CreatedAt:        r.CreatedAt(),
		UpdatedAt:        r.UpdatedAt(),
	}
}

// ModelToRecipe converts a GORM model to a domain recipe
func ModelToRecipe(model *RecipeModel) (*recipe.Recipe, error) {
	// Create a basic recipe first
	r, err := recipe.NewRecipe(model.Title, model.Description, model.AuthorID)
	if err != nil {
		return nil, err
	}

	// For a proper implementation, we would need to either:
	// 1. Add setters to the domain entity, or
	// 2. Use reflection to set private fields, or  
	// 3. Create a constructor that accepts all fields
	//
	// For now, this simplified approach creates a basic recipe
	// The main issue would be in updating existing recipes where
	// we lose the social metrics (likes, views, ratings, etc.)
	//
	// TODO: Implement proper field mapping when domain entity allows it
	
	return r, nil
}

// AIRequestToModel converts a domain AI request to a GORM model
func AIRequestToModel(req *ai.AIRequest) *AIRequestModel {
	parametersJSON := req.Parameters()
	responseJSON := req.Response()

	return &AIRequestModel{
		ID:           req.ID(),
		UserID:       req.UserID(),
		Prompt:       req.Prompt(),
		Provider:     string(req.Provider()),
		Model:        req.Model(),
		Parameters:   JSONField(parametersJSON),
		Status:       string(req.Status()),
		Response:     JSONField(map[string]interface{}{"data": responseJSON}),
		TokensUsed:   req.TokensUsed(),
		CostCents:    req.CostCents(),
		CreatedAt:    req.CreatedAt(),
		CompletedAt:  req.CompletedAt(),
		ErrorMessage: req.ErrorMessage(),
	}
}

// Helper functions for JSON conversion
func convertIngredientsToJSON(ingredients []recipe.Ingredient) []map[string]interface{} {
	result := make([]map[string]interface{}, len(ingredients))
	for i, ing := range ingredients {
		result[i] = map[string]interface{}{
			"id":       ing.ID,
			"name":     ing.Name,
			"amount":   ing.Amount,
			"unit":     string(ing.Unit),
			"optional": ing.Optional,
			"notes":    ing.Notes,
		}
	}
	return result
}

func convertInstructionsToJSON(instructions []recipe.Instruction) []map[string]interface{} {
	result := make([]map[string]interface{}, len(instructions))
	for i, inst := range instructions {
		var temp *map[string]interface{}
		if inst.Temperature != nil {
			temp = &map[string]interface{}{
				"value": inst.Temperature.Value,
				"unit":  string(inst.Temperature.Unit),
			}
		}

		result[i] = map[string]interface{}{
			"step_number":  inst.StepNumber,
			"description":  inst.Description,
			"duration":     inst.Duration.Minutes(),
			"temperature":  temp,
			"images":       inst.Images,
		}
	}
	return result
}

func convertNutritionToJSON(nutrition *recipe.NutritionInfo) map[string]interface{} {
	if nutrition == nil {
		return nil
	}
	
	return map[string]interface{}{
		"calories":      nutrition.Calories,
		"protein":       nutrition.Protein,
		"carbohydrates": nutrition.Carbohydrates,
		"fat":           nutrition.Fat,
		"fiber":         nutrition.Fiber,
		"sugar":         nutrition.Sugar,
		"sodium":        nutrition.Sodium,
		"cholesterol":   nutrition.Cholesterol,
	}
}

func convertImagesToJSON(images []recipe.Image) []map[string]interface{} {
	result := make([]map[string]interface{}, len(images))
	for i, img := range images {
		result[i] = map[string]interface{}{
			"id":            img.ID,
			"url":           img.URL,
			"thumbnail_url": img.ThumbnailURL,
			"caption":       img.Caption,
			"is_primary":    img.IsPrimary,
			"uploaded_at":   img.UploadedAt,
		}
	}
	return result
}

func convertVideosToJSON(videos []recipe.Video) []map[string]interface{} {
	result := make([]map[string]interface{}, len(videos))
	for i, video := range videos {
		result[i] = map[string]interface{}{
			"id":            video.ID,
			"url":           video.URL,
			"thumbnail_url": video.ThumbnailURL,
			"duration":      video.Duration.Seconds(),
			"caption":       video.Caption,
			"uploaded_at":   video.UploadedAt,
		}
	}
	return result
}