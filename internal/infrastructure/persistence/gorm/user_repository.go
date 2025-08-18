// Package gorm provides GORM-based repository implementations
package gorm

import (
	"context"
	"errors"
	"strings"

	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRepository implements the user repository interface using GORM
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) outbound.UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *user.User) error {
	model := UserToModel(user)
	
	result := r.db.WithContext(ctx).Create(model)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "UNIQUE constraint failed") ||
		   strings.Contains(result.Error.Error(), "duplicate key") {
			return errors.New("user with this email already exists")
		}
		return result.Error
	}
	
	return nil
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, user *user.User) error {
	model := UserToModel(user)
	
	result := r.db.WithContext(ctx).Save(model)
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return errors.New("user not found")
	}
	
	return nil
}

// Delete deletes a user by ID
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&UserModel{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return errors.New("user not found")
	}
	
	return nil
}

// FindByID finds a user by ID
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	var model UserModel
	
	result := r.db.WithContext(ctx).First(&model, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, result.Error
	}
	
	return ModelToUser(&model)
}

// FindByEmail finds a user by email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	var model UserModel
	
	result := r.db.WithContext(ctx).First(&model, "email = ?", strings.ToLower(email))
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, result.Error
	}
	
	return ModelToUser(&model)
}

// FindByUsername finds a user by username (using name field)
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	var model UserModel
	
	result := r.db.WithContext(ctx).First(&model, "name = ?", username)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, result.Error
	}
	
	return ModelToUser(&model)
}

// Exists checks if a user exists by ID
func (r *UserRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var count int64
	
	result := r.db.WithContext(ctx).Model(&UserModel{}).Where("id = ?", id).Count(&count)
	if result.Error != nil {
		return false, result.Error
	}
	
	return count > 0, nil
}

// UpdateLastLogin updates the last login timestamp
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Model(&UserModel{}).
		Where("id = ?", id).
		Update("last_login_at", gorm.Expr("CURRENT_TIMESTAMP"))
		
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		return errors.New("user not found")
	}
	
	return nil
}