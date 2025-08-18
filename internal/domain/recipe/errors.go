package recipe

import "errors"

// Domain errors for recipe operations

var (
	// Entity validation errors
	ErrTitleTooShort       = errors.New("recipe title must be at least 3 characters")
	ErrTitleTooLong        = errors.New("recipe title must not exceed 200 characters")
	ErrDescriptionTooLong  = errors.New("recipe description must not exceed 2000 characters")
	ErrInvalidServings     = errors.New("servings must be greater than 0")
	ErrNoIngredients       = errors.New("recipe must have at least one ingredient")
	ErrNoInstructions      = errors.New("recipe must have at least one instruction")
	
	// State transition errors
	ErrInvalidStatusTransition = errors.New("invalid recipe status transition")
	ErrRecipeNotFound         = errors.New("recipe not found")
	ErrRecipeAlreadyPublished = errors.New("recipe is already published")
	ErrRecipeArchived         = errors.New("cannot modify archived recipe")
	
	// Business rule violations
	ErrDuplicateIngredient    = errors.New("ingredient already exists in recipe")
	ErrInvalidRating         = errors.New("rating must be between 1 and 5")
	ErrUserAlreadyRated      = errors.New("user has already rated this recipe")
	ErrCannotRateOwnRecipe   = errors.New("cannot rate your own recipe")
	
	// Permission errors
	ErrUnauthorized          = errors.New("unauthorized to perform this action")
	ErrNotRecipeOwner        = errors.New("only recipe owner can perform this action")
)