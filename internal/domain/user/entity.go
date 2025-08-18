// Package user defines the user domain entity
package user

import (
	"errors"
	"strings"
	"time"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	id           uuid.UUID
	email        string
	name         string
	passwordHash string
	isActive     bool
	isVerified   bool
	role         UserRole
	profile      *UserProfile
	preferences  *UserPreferences
	createdAt    time.Time
	updatedAt    time.Time
	lastLoginAt  *time.Time
}

// UserProfile contains additional user profile information
type UserProfile struct {
	FirstName    string
	LastName     string
	Avatar       string
	Bio          string
	Location     string
	Website      string
	Birthday     *time.Time
	CookingLevel CookingLevel
}

// UserPreferences contains user preferences
type UserPreferences struct {
	DietaryRestrictions []DietaryRestriction
	Allergies          []string
	PreferredCuisines  []string
	DislikedIngredients []string
	MeasurementSystem  MeasurementSystem
	Language           string
	Timezone           string
	EmailNotifications bool
	PushNotifications  bool
}

// UserRole represents the role of a user
type UserRole string

const (
	UserRoleUser  UserRole = "user"
	UserRoleAdmin UserRole = "admin"
	UserRoleChef  UserRole = "chef"
)

// CookingLevel represents a user's cooking skill level
type CookingLevel string

const (
	CookingLevelBeginner     CookingLevel = "beginner"
	CookingLevelIntermediate CookingLevel = "intermediate"
	CookingLevelAdvanced     CookingLevel = "advanced"
	CookingLevelProfessional CookingLevel = "professional"
)

// DietaryRestriction represents dietary restrictions
type DietaryRestriction string

const (
	DietaryRestrictionVegetarian DietaryRestriction = "vegetarian"
	DietaryRestrictionVegan      DietaryRestriction = "vegan"
	DietaryRestrictionGlutenFree DietaryRestriction = "gluten_free"
	DietaryRestrictionDairyFree  DietaryRestriction = "dairy_free"
	DietaryRestrictionKeto       DietaryRestriction = "keto"
	DietaryRestrictionPaleo      DietaryRestriction = "paleo"
	DietaryRestrictionHalal      DietaryRestriction = "halal"
	DietaryRestrictionKosher     DietaryRestriction = "kosher"
)

// MeasurementSystem represents measurement preferences
type MeasurementSystem string

const (
	MeasurementSystemMetric   MeasurementSystem = "metric"
	MeasurementSystemImperial MeasurementSystem = "imperial"
)

// NewUser creates a new user with validation
func NewUser(email, name, password string) (*User, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	
	if err := validateName(name); err != nil {
		return nil, err
	}
	
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}
	
	now := time.Now()
	return &User{
		id:           uuid.New(),
		email:        strings.ToLower(email),
		name:         name,
		passwordHash: string(hashedPassword),
		isActive:     true,
		isVerified:   false,
		role:         UserRoleUser,
		preferences: &UserPreferences{
			MeasurementSystem:  MeasurementSystemMetric,
			Language:           "en",
			EmailNotifications: true,
			PushNotifications:  true,
		},
		createdAt: now,
		updatedAt: now,
	}, nil
}

// ID returns the user's ID
func (u *User) ID() uuid.UUID {
	return u.id
}

// Email returns the user's email
func (u *User) Email() string {
	return u.email
}

// Name returns the user's name
func (u *User) Name() string {
	return u.name
}

// IsActive returns whether the user is active
func (u *User) IsActive() bool {
	return u.isActive
}

// IsVerified returns whether the user is verified
func (u *User) IsVerified() bool {
	return u.isVerified
}

// Role returns the user's role
func (u *User) Role() UserRole {
	return u.role
}

// Profile returns the user's profile
func (u *User) Profile() *UserProfile {
	return u.profile
}

// Preferences returns the user's preferences
func (u *User) Preferences() *UserPreferences {
	return u.preferences
}

// CreatedAt returns when the user was created
func (u *User) CreatedAt() time.Time {
	return u.createdAt
}

// UpdatedAt returns when the user was last updated
func (u *User) UpdatedAt() time.Time {
	return u.updatedAt
}

// LastLoginAt returns when the user last logged in
func (u *User) LastLoginAt() *time.Time {
	return u.lastLoginAt
}

// CheckPassword verifies if the provided password matches
func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.passwordHash), []byte(password))
}

// UpdatePassword updates the user's password
func (u *User) UpdatePassword(newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("failed to hash password")
	}
	
	u.passwordHash = string(hashedPassword)
	u.updatedAt = time.Now()
	return nil
}

// UpdateProfile updates the user's profile
func (u *User) UpdateProfile(profile *UserProfile) {
	u.profile = profile
	u.updatedAt = time.Now()
}

// UpdatePreferences updates the user's preferences
func (u *User) UpdatePreferences(preferences *UserPreferences) {
	u.preferences = preferences
	u.updatedAt = time.Now()
}

// Verify marks the user as verified
func (u *User) Verify() {
	u.isVerified = true
	u.updatedAt = time.Now()
}

// Deactivate deactivates the user
func (u *User) Deactivate() {
	u.isActive = false
	u.updatedAt = time.Now()
}

// Activate activates the user
func (u *User) Activate() {
	u.isActive = true
	u.updatedAt = time.Now()
}

// RecordLogin records a login timestamp
func (u *User) RecordLogin() {
	now := time.Now()
	u.lastLoginAt = &now
	u.updatedAt = now
}

// HasDietaryRestriction checks if user has a specific dietary restriction
func (u *User) HasDietaryRestriction(restriction DietaryRestriction) bool {
	if u.preferences == nil {
		return false
	}
	
	for _, r := range u.preferences.DietaryRestrictions {
		if r == restriction {
			return true
		}
	}
	return false
}

// Validation functions
func validateEmail(email string) error {
	if email == "" {
		return errors.New("email is required")
	}
	
	if !strings.Contains(email, "@") {
		return errors.New("invalid email format")
	}
	
	if len(email) > 255 {
		return errors.New("email too long")
	}
	
	return nil
}

func validateName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	
	if len(name) < 2 {
		return errors.New("name must be at least 2 characters")
	}
	
	if len(name) > 100 {
		return errors.New("name too long")
	}
	
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	
	if len(password) > 128 {
		return errors.New("password too long")
	}
	
	return nil
}