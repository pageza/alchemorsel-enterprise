// Package postgres provides PostgreSQL repository implementations
package postgres

import (
	"context"

	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// UserRepository implements the user repository interface
type UserRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *pgxpool.Pool, logger *zap.Logger) outbound.UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *user.User) error {
	// Implementation would go here
	return nil
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, user *user.User) error {
	// Implementation would go here
	return nil
}

// FindByID retrieves a user by ID
func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*user.User, error) {
	// Implementation would go here
	return nil, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	// Implementation would go here
	return nil, nil
}

// Delete deletes a user by ID
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to delete user",
			zap.String("user_id", id.String()),
			zap.Error(err),
		)
		return err
	}

	r.logger.Info("User deleted successfully",
		zap.String("user_id", id.String()),
	)

	return nil
}

// FindByUsername retrieves a user by username
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*user.User, error) {
	// Implementation would go here
	return nil, nil
}

// Exists checks if a user exists by ID
func (r *UserRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	query := `SELECT COUNT(1) FROM users WHERE id = $1`

	var count int
	err := r.db.QueryRow(ctx, query, id).Scan(&count)
	if err != nil {
		r.logger.Error("Failed to check user existence",
			zap.String("user_id", id.String()),
			zap.Error(err),
		)
		return false, err
	}

	return count > 0, nil
}

// FindByEmail finds a user by email address
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	query := `SELECT id, username, email, created_at, updated_at FROM users WHERE email = $1`

	var u user.User
	var createdAt, updatedAt time.Time
	
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Username, 
		&u.Email,
		&createdAt,
		&updatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found with email: %s", email)
		}
		r.logger.Error("Failed to find user by email",
			zap.String("email", email),
			zap.Error(err),
		)
		return nil, err
	}

	// Note: This is a simplified implementation - in production you'd properly 
	// reconstruct the domain object with all its methods and validation
	return &u, nil
}

// UpdateLastLogin updates the last login timestamp for a user
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET last_login = NOW() WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		r.logger.Error("Failed to update last login",
			zap.String("user_id", id.String()),
			zap.Error(err),
		)
		return err
	}

	return nil
}
