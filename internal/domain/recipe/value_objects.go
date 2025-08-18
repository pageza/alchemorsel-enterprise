package recipe

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Value Objects - Immutable objects that describe aspects of the domain

// Ingredient represents an ingredient in a recipe
type Ingredient struct {
	ID       uuid.UUID
	Name     string
	Amount   float64
	Unit     MeasurementUnit
	Optional bool
	Notes    string
}

// Validate validates the ingredient
func (i Ingredient) Validate() error {
	if i.Name == "" {
		return errors.New("ingredient name is required")
	}
	if i.Amount < 0 {
		return errors.New("ingredient amount cannot be negative")
	}
	return nil
}

// Instruction represents a cooking instruction step
type Instruction struct {
	StepNumber  int
	Description string
	Duration    time.Duration
	Temperature *Temperature
	Images      []string
}

// Validate validates the instruction
func (i Instruction) Validate() error {
	if i.Description == "" {
		return errors.New("instruction description is required")
	}
	if len(i.Description) > 1000 {
		return errors.New("instruction description too long")
	}
	return nil
}

// Temperature represents cooking temperature
type Temperature struct {
	Value float64
	Unit  TemperatureUnit
}

// ToCelsius converts temperature to Celsius
func (t Temperature) ToCelsius() float64 {
	switch t.Unit {
	case TemperatureUnitFahrenheit:
		return (t.Value - 32) * 5 / 9
	case TemperatureUnitKelvin:
		return t.Value - 273.15
	default:
		return t.Value
	}
}

// NutritionInfo contains nutritional information
type NutritionInfo struct {
	Calories      int
	Protein       float64 // in grams
	Carbohydrates float64 // in grams
	Fat           float64 // in grams
	Fiber         float64 // in grams
	Sugar         float64 // in grams
	Sodium        float64 // in milligrams
	Cholesterol   float64 // in milligrams
}

// Rating represents a user's rating of a recipe
type Rating struct {
	UserID    uuid.UUID
	Value     int // 1-5 stars
	Comment   string
	CreatedAt time.Time
}

// Validate validates the rating
func (r Rating) Validate() error {
	if r.Value < 1 || r.Value > 5 {
		return errors.New("rating must be between 1 and 5")
	}
	if len(r.Comment) > 500 {
		return errors.New("comment too long")
	}
	return nil
}

// Image represents a recipe image
type Image struct {
	ID          uuid.UUID
	URL         string
	ThumbnailURL string
	Caption     string
	IsPrimary   bool
	UploadedAt  time.Time
}

// Video represents a recipe video
type Video struct {
	ID          uuid.UUID
	URL         string
	ThumbnailURL string
	Duration    time.Duration
	Caption     string
	UploadedAt  time.Time
}

// MeasurementUnit represents units of measurement
type MeasurementUnit string

const (
	// Volume units
	MeasurementUnitTeaspoon   MeasurementUnit = "tsp"
	MeasurementUnitTablespoon MeasurementUnit = "tbsp"
	MeasurementUnitCup        MeasurementUnit = "cup"
	MeasurementUnitOunce      MeasurementUnit = "oz"
	MeasurementUnitMilliliter MeasurementUnit = "ml"
	MeasurementUnitLiter      MeasurementUnit = "l"
	
	// Weight units
	MeasurementUnitGram     MeasurementUnit = "g"
	MeasurementUnitKilogram MeasurementUnit = "kg"
	MeasurementUnitPound    MeasurementUnit = "lb"
	
	// Count units
	MeasurementUnitPiece MeasurementUnit = "piece"
	MeasurementUnitDash  MeasurementUnit = "dash"
	MeasurementUnitPinch MeasurementUnit = "pinch"
)

// TemperatureUnit represents temperature units
type TemperatureUnit string

const (
	TemperatureUnitCelsius    TemperatureUnit = "C"
	TemperatureUnitFahrenheit TemperatureUnit = "F"
	TemperatureUnitKelvin     TemperatureUnit = "K"
)

// CuisineType represents different cuisine types
type CuisineType string

const (
	CuisineTypeItalian     CuisineType = "italian"
	CuisineTypeFrench      CuisineType = "french"
	CuisineTypeChinese     CuisineType = "chinese"
	CuisineTypeJapanese    CuisineType = "japanese"
	CuisineTypeIndian      CuisineType = "indian"
	CuisineTypeMexican     CuisineType = "mexican"
	CuisineTypeAmerican    CuisineType = "american"
	CuisineTypeMediterranean CuisineType = "mediterranean"
	CuisineTypeThai        CuisineType = "thai"
	CuisineTypeOther       CuisineType = "other"
)

// CategoryType represents recipe categories
type CategoryType string

const (
	CategoryTypeAppetizer  CategoryType = "appetizer"
	CategoryTypeMainCourse CategoryType = "main_course"
	CategoryTypeSideDish   CategoryType = "side_dish"
	CategoryTypeDessert    CategoryType = "dessert"
	CategoryTypeBeverage   CategoryType = "beverage"
	CategoryTypeBreakfast  CategoryType = "breakfast"
	CategoryTypeLunch      CategoryType = "lunch"
	CategoryTypeDinner     CategoryType = "dinner"
	CategoryTypeSnack      CategoryType = "snack"
)

// DifficultyLevel represents recipe difficulty
type DifficultyLevel string

const (
	DifficultyLevelEasy   DifficultyLevel = "easy"
	DifficultyLevelMedium DifficultyLevel = "medium"
	DifficultyLevelHard   DifficultyLevel = "hard"
	DifficultyLevelExpert DifficultyLevel = "expert"
)

// RecipeStatus represents the status of a recipe
type RecipeStatus string

const (
	RecipeStatusDraft     RecipeStatus = "draft"
	RecipeStatusPublished RecipeStatus = "published"
	RecipeStatusArchived  RecipeStatus = "archived"
	RecipeStatusDeleted   RecipeStatus = "deleted"
)