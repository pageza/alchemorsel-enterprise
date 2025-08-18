// Package user provides the application layer for user management
package user

import (
	"context"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// UserService implements user management use cases
type UserService struct {
	userRepo    outbound.UserRepository
	cache       outbound.CacheRepository
	jwtSecret   string
	logger      *zap.Logger
}

// NewUserService creates a new user service
func NewUserService(
	userRepo outbound.UserRepository,
	cache outbound.CacheRepository,
	jwtSecret string,
	logger *zap.Logger,
) *UserService {
	return &UserService{
		userRepo:  userRepo,
		cache:     cache,
		jwtSecret: jwtSecret,
		logger:    logger.Named("user-service"),
	}
}

// RegisterCommand contains user registration data
type RegisterCommand struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginCommand contains user login data
type LoginCommand struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// UserDTO represents user data transfer object
type UserDTO struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	IsVerified bool      `json:"is_verified"`
	Role       string    `json:"role"`
	CreatedAt  time.Time `json:"created_at"`
}

// AuthResponse contains authentication response data
type AuthResponse struct {
	User         UserDTO `json:"user"`
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresIn    int     `json:"expires_in"`
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	jwt.StandardClaims
}

// Register creates a new user account
func (s *UserService) Register(ctx context.Context, cmd RegisterCommand) (*AuthResponse, error) {
	s.logger.Info("Registering new user", zap.String("email", cmd.Email))

	// Check if user already exists
	existingUser, err := s.userRepo.FindByEmail(ctx, cmd.Email)
	if err == nil && existingUser != nil {
		return nil, fmt.Errorf("user with email %s already exists", cmd.Email)
	}

	// Create new user
	newUser, err := user.NewUser(cmd.Email, cmd.Name, cmd.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Save user
	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(newUser)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	s.logger.Info("User registered successfully", 
		zap.String("user_id", newUser.ID().String()),
		zap.String("email", newUser.Email()),
	)

	return &AuthResponse{
		User:         s.entityToDTO(newUser),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600, // 1 hour
	}, nil
}

// Login authenticates a user
func (s *UserService) Login(ctx context.Context, cmd LoginCommand) (*AuthResponse, error) {
	s.logger.Info("User login attempt", zap.String("email", cmd.Email))

	// Find user by email
	userEntity, err := s.userRepo.FindByEmail(ctx, cmd.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check password
	if err := userEntity.CheckPassword(cmd.Password); err != nil {
		s.logger.Warn("Invalid password attempt", zap.String("email", cmd.Email))
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user is active
	if !userEntity.IsActive() {
		return nil, fmt.Errorf("account is deactivated")
	}

	// Update last login
	userEntity.RecordLogin()
	if err := s.userRepo.Update(ctx, userEntity); err != nil {
		s.logger.Error("Failed to update last login", zap.Error(err))
	}

	// Generate tokens
	accessToken, refreshToken, err := s.generateTokens(userEntity)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	s.logger.Info("User logged in successfully", 
		zap.String("user_id", userEntity.ID().String()),
		zap.String("email", userEntity.Email()),
	)

	return &AuthResponse{
		User:         s.entityToDTO(userEntity),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600, // 1 hour
	}, nil
}

// GetUserByID retrieves a user by ID
func (s *UserService) GetUserByID(ctx context.Context, userID uuid.UUID) (*UserDTO, error) {
	userEntity, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	dto := s.entityToDTO(userEntity)
	return &dto, nil
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*UserDTO, error) {
	userEntity, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	dto := s.entityToDTO(userEntity)
	return &dto, nil
}

// UpdateProfile updates user profile information
func (s *UserService) UpdateProfile(ctx context.Context, userID uuid.UUID, profile *user.UserProfile) error {
	userEntity, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	userEntity.UpdateProfile(profile)

	if err := s.userRepo.Update(ctx, userEntity); err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	s.logger.Info("User profile updated", zap.String("user_id", userID.String()))
	return nil
}

// UpdatePreferences updates user preferences
func (s *UserService) UpdatePreferences(ctx context.Context, userID uuid.UUID, preferences *user.UserPreferences) error {
	userEntity, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	userEntity.UpdatePreferences(preferences)

	if err := s.userRepo.Update(ctx, userEntity); err != nil {
		return fmt.Errorf("failed to update preferences: %w", err)
	}

	s.logger.Info("User preferences updated", zap.String("user_id", userID.String()))
	return nil
}

// ChangePassword changes user password
func (s *UserService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	userEntity, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Verify current password
	if err := userEntity.CheckPassword(currentPassword); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	// Update password
	if err := userEntity.UpdatePassword(newPassword); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	if err := s.userRepo.Update(ctx, userEntity); err != nil {
		return fmt.Errorf("failed to save password: %w", err)
	}

	s.logger.Info("User password changed", zap.String("user_id", userID.String()))
	return nil
}

// ValidateToken validates a JWT token and returns user claims
func (s *UserService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// Helper methods

// generateTokens generates access and refresh tokens
func (s *UserService) generateTokens(userEntity *user.User) (string, string, error) {
	now := time.Now()
	
	// Access token (1 hour)
	accessClaims := &JWTClaims{
		UserID: userEntity.ID(),
		Email:  userEntity.Email(),
		Role:   string(userEntity.Role()),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: now.Add(time.Hour).Unix(),
			IssuedAt:  now.Unix(),
			NotBefore: now.Unix(),
			Subject:   userEntity.ID().String(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", err
	}

	// Refresh token (7 days)
	refreshClaims := &JWTClaims{
		UserID: userEntity.ID(),
		Email:  userEntity.Email(),
		Role:   string(userEntity.Role()),
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: now.Add(7 * 24 * time.Hour).Unix(),
			IssuedAt:  now.Unix(),
			NotBefore: now.Unix(),
			Subject:   userEntity.ID().String(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
}

// entityToDTO converts user entity to DTO
func (s *UserService) entityToDTO(userEntity *user.User) UserDTO {
	return UserDTO{
		ID:         userEntity.ID(),
		Email:      userEntity.Email(),
		Name:       userEntity.Name(),
		IsVerified: userEntity.IsVerified(),
		Role:       string(userEntity.Role()),
		CreatedAt:  userEntity.CreatedAt(),
	}
}