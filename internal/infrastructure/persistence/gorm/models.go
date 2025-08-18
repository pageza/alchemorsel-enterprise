// Package gorm provides GORM model definitions for the application
package gorm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserModel represents the GORM model for users
type UserModel struct {
	ID           uuid.UUID `gorm:"type:char(36);primaryKey"`
	Email        string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	Name         string    `gorm:"type:varchar(255);not null"`
	PasswordHash string    `gorm:"type:varchar(255);not null"`
	IsActive     bool      `gorm:"default:true"`
	IsVerified   bool      `gorm:"default:false"`
	Role         string    `gorm:"type:varchar(50);default:'user'"`
	Profile      *UserProfileModel `gorm:"embedded;embeddedPrefix:profile_"`
	Preferences  *UserPreferencesModel `gorm:"embedded;embeddedPrefix:pref_"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
	
	// Relationships
	Recipes []RecipeModel `gorm:"foreignKey:AuthorID"`
}

// UserProfileModel represents embedded user profile
type UserProfileModel struct {
	FirstName    string     `gorm:"type:varchar(100)"`
	LastName     string     `gorm:"type:varchar(100)"`
	Avatar       string     `gorm:"type:text"`
	Bio          string     `gorm:"type:text"`
	Location     string     `gorm:"type:varchar(255)"`
	Website      string     `gorm:"type:varchar(255)"`
	Birthday     *time.Time
	CookingLevel string     `gorm:"type:varchar(50)"`
}

// UserPreferencesModel represents embedded user preferences
type UserPreferencesModel struct {
	DietaryRestrictions StringSlice `gorm:"type:json"`
	Allergies          StringSlice `gorm:"type:json"`
	PreferredCuisines  StringSlice `gorm:"type:json"`
	DislikedIngredients StringSlice `gorm:"type:json"`
	MeasurementSystem  string      `gorm:"type:varchar(20);default:'metric'"`
	Language           string      `gorm:"type:varchar(10);default:'en'"`
	Timezone           string      `gorm:"type:varchar(50)"`
	EmailNotifications bool        `gorm:"default:true"`
	PushNotifications  bool        `gorm:"default:true"`
}

// RecipeModel represents the GORM model for recipes
type RecipeModel struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey"`
	Version     int64     `gorm:"default:1"`
	Title       string    `gorm:"type:varchar(255);not null;index"`
	Description string    `gorm:"type:text"`
	AuthorID    uuid.UUID `gorm:"type:char(36);not null;index"`
	
	// Recipe details
	Ingredients   JSONField `gorm:"type:json"`
	Instructions  JSONField `gorm:"type:json"`
	NutritionInfo JSONField `gorm:"type:json"`
	
	// Categorization
	Cuisine    string      `gorm:"type:varchar(50);index"`
	Category   string      `gorm:"type:varchar(50);index"`
	Difficulty string      `gorm:"type:varchar(20);index"`
	Tags       StringSlice `gorm:"type:json"`
	
	// Timing (stored in minutes)
	PrepTimeMinutes  int `gorm:"column:prep_time_minutes;default:0"`
	CookTimeMinutes  int `gorm:"column:cook_time_minutes;default:0"`
	TotalTimeMinutes int `gorm:"column:total_time_minutes;default:0"`
	
	// Metrics
	Servings int `gorm:"default:1"`
	Calories int `gorm:"default:0"`
	
	// AI-generated content
	AIGenerated bool   `gorm:"default:false"`
	AIPrompt    string `gorm:"type:text"`
	AIModel     string `gorm:"type:varchar(100)"`
	
	// Social features
	Likes         int     `gorm:"column:likes_count;default:0;index"`
	Views         int     `gorm:"column:views_count;default:0"`
	AverageRating float64 `gorm:"column:average_rating;default:0;index"`
	
	// Media
	Images JSONField `gorm:"type:json"`
	Videos JSONField `gorm:"type:json"`
	
	// Metadata
	Status      string     `gorm:"type:varchar(20);default:'draft';index"`
	PublishedAt *time.Time `gorm:"index"`
	CreatedAt   time.Time  `gorm:"index"`
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
	
	// Relationships
	Author  UserModel     `gorm:"foreignKey:AuthorID"`
	Ratings []RatingModel `gorm:"foreignKey:RecipeID"`
}

// RatingModel represents the GORM model for recipe ratings
type RatingModel struct {
	ID       uuid.UUID `gorm:"type:char(36);primaryKey"`
	RecipeID uuid.UUID `gorm:"type:char(36);not null;index"`
	UserID   uuid.UUID `gorm:"type:char(36);not null;index"`
	Value    int       `gorm:"not null;check:value >= 1 AND value <= 5"`
	Comment  string    `gorm:"type:text"`
	CreatedAt time.Time
	
	// Relationships
	Recipe RecipeModel `gorm:"foreignKey:RecipeID"`
	User   UserModel   `gorm:"foreignKey:UserID"`
}

// AIRequestModel represents the GORM model for AI requests
type AIRequestModel struct {
	ID           uuid.UUID `gorm:"type:char(36);primaryKey"`
	UserID       uuid.UUID `gorm:"type:char(36);not null;index"`
	Prompt       string    `gorm:"type:text;not null"`
	Provider     string    `gorm:"type:varchar(50);not null"`
	Model        string    `gorm:"type:varchar(100);not null"`
	Parameters   JSONField `gorm:"type:json"`
	Status       string    `gorm:"type:varchar(20);default:'pending';index"`
	Response     JSONField `gorm:"type:json"`
	TokensUsed   int       `gorm:"default:0"`
	CostCents    int       `gorm:"default:0"`
	CreatedAt    time.Time `gorm:"index"`
	CompletedAt  *time.Time
	ErrorMessage string    `gorm:"type:text"`
	
	// Relationships
	User UserModel `gorm:"foreignKey:UserID"`
}

// RecipeLikeModel represents the GORM model for recipe likes
type RecipeLikeModel struct {
	RecipeID  uuid.UUID `gorm:"type:char(36);primaryKey"`
	UserID    uuid.UUID `gorm:"type:char(36);primaryKey"`
	CreatedAt time.Time `gorm:"index"`
	
	// Relationships
	Recipe RecipeModel `gorm:"foreignKey:RecipeID"`
	User   UserModel   `gorm:"foreignKey:UserID"`
}

// UserFollowModel represents the GORM model for user follows
type UserFollowModel struct {
	FollowerID  uuid.UUID `gorm:"type:char(36);primaryKey"`
	FollowingID uuid.UUID `gorm:"type:char(36);primaryKey"`
	CreatedAt   time.Time `gorm:"index"`
	
	// Relationships
	Follower  UserModel `gorm:"foreignKey:FollowerID"`
	Following UserModel `gorm:"foreignKey:FollowingID"`
}

// CollectionModel represents the GORM model for recipe collections
type CollectionModel struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey"`
	UserID      uuid.UUID `gorm:"type:char(36);not null;index"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	IsPublic    bool      `gorm:"default:true"`
	CreatedAt   time.Time `gorm:"index"`
	UpdatedAt   time.Time
	
	// Relationships
	User    UserModel             `gorm:"foreignKey:UserID"`
	Recipes []CollectionRecipeModel `gorm:"foreignKey:CollectionID"`
}

// CollectionRecipeModel represents the GORM model for recipes in collections
type CollectionRecipeModel struct {
	CollectionID uuid.UUID `gorm:"type:char(36);primaryKey"`
	RecipeID     uuid.UUID `gorm:"type:char(36);primaryKey"`
	OrderIndex   int       `gorm:"default:0"`
	AddedAt      time.Time `gorm:"index"`
	
	// Relationships
	Collection CollectionModel `gorm:"foreignKey:CollectionID"`
	Recipe     RecipeModel     `gorm:"foreignKey:RecipeID"`
}

// CommentModel represents the GORM model for recipe comments
type CommentModel struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey"`
	RecipeID  uuid.UUID `gorm:"type:char(36);not null;index"`
	UserID    uuid.UUID `gorm:"type:char(36);not null;index"`
	ParentID  *uuid.UUID `gorm:"type:char(36);index"` // For nested comments
	Content   string     `gorm:"type:text;not null"`
	CreatedAt time.Time  `gorm:"index"`
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	
	// Relationships
	Recipe   RecipeModel    `gorm:"foreignKey:RecipeID"`
	User     UserModel      `gorm:"foreignKey:UserID"`
	Parent   *CommentModel  `gorm:"foreignKey:ParentID"`
	Replies  []CommentModel `gorm:"foreignKey:ParentID"`
}

// ActivityModel represents the GORM model for user activities
type ActivityModel struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey"`
	UserID      uuid.UUID `gorm:"type:char(36);not null;index"`
	ActorID     uuid.UUID `gorm:"type:char(36);not null;index"` // Who performed the action
	Type        string    `gorm:"type:varchar(50);not null;index"`
	EntityType  string    `gorm:"type:varchar(50);not null"`
	EntityID    uuid.UUID `gorm:"type:char(36);not null"`
	Title       string    `gorm:"type:varchar(255);not null"`
	Description string    `gorm:"type:text"`
	Data        JSONField `gorm:"type:json"`
	IsRead      bool      `gorm:"default:false;index"`
	CreatedAt   time.Time `gorm:"index"`
	
	// Relationships
	User  UserModel `gorm:"foreignKey:UserID"`
	Actor UserModel `gorm:"foreignKey:ActorID"`
}

// RecipeViewModel represents the GORM model for recipe views
type RecipeViewModel struct {
	ID        uuid.UUID `gorm:"type:char(36);primaryKey"`
	RecipeID  uuid.UUID `gorm:"type:char(36);not null;index"`
	UserID    *uuid.UUID `gorm:"type:char(36);index"` // Nullable for anonymous views
	IPAddress string     `gorm:"type:varchar(45)"`
	UserAgent string     `gorm:"type:text"`
	Referrer  string     `gorm:"type:text"`
	CreatedAt time.Time  `gorm:"index"`
	
	// Relationships
	Recipe RecipeModel `gorm:"foreignKey:RecipeID"`
	User   *UserModel  `gorm:"foreignKey:UserID"`
}

// StringSlice custom type for handling string slices in JSON
type StringSlice []string

// Scan implements the sql.Scanner interface
func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	default:
		return fmt.Errorf("cannot scan %T into StringSlice", value)
	}
}

// Value implements the driver.Valuer interface
func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

// JSONField custom type for handling JSON fields
type JSONField map[string]interface{}

// Scan implements the sql.Scanner interface
func (j *JSONField) Scan(value interface{}) error {
	if value == nil {
		*j = JSONField{}
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	default:
		return fmt.Errorf("cannot scan %T into JSONField", value)
	}
}

// Value implements the driver.Valuer interface
func (j JSONField) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return json.Marshal(j)
}

// BeforeCreate hook for UserModel
func (u *UserModel) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook for RecipeModel
func (r *RecipeModel) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook for RatingModel
func (r *RatingModel) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook for AIRequestModel
func (a *AIRequestModel) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook for CollectionModel
func (c *CollectionModel) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook for CommentModel
func (c *CommentModel) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook for ActivityModel
func (a *ActivityModel) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// BeforeCreate hook for RecipeViewModel
func (r *RecipeViewModel) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// TableName methods for custom table names
func (UserModel) TableName() string {
	return "users"
}

func (RecipeModel) TableName() string {
	return "recipes"
}

func (RatingModel) TableName() string {
	return "ratings"
}

func (AIRequestModel) TableName() string {
	return "ai_requests"
}

func (RecipeLikeModel) TableName() string {
	return "recipe_likes"
}

func (UserFollowModel) TableName() string {
	return "user_follows"
}

func (CollectionModel) TableName() string {
	return "collections"
}

func (CollectionRecipeModel) TableName() string {
	return "collection_recipes"
}

func (CommentModel) TableName() string {
	return "comments"
}

func (ActivityModel) TableName() string {
	return "activities"
}

func (RecipeViewModel) TableName() string {
	return "recipe_views"
}