// Package handlers provides HTTP handlers for authentication API endpoints
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/alchemorsel/v3/internal/application/user"
	"github.com/alchemorsel/v3/internal/infrastructure/http/middleware"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"go.uber.org/zap"
)

// AuthAPIHandlers handles authentication API requests
type AuthAPIHandlers struct {
	userService *user.UserService
	authService *security.AuthService
	logger      *zap.Logger
}

// NewAuthAPIHandlers creates a new authentication API handlers instance
func NewAuthAPIHandlers(
	userService *user.UserService,
	authService *security.AuthService,
	logger *zap.Logger,
) *AuthAPIHandlers {
	return &AuthAPIHandlers{
		userService: userService,
		authService: authService,
		logger:      logger,
	}
}

// RegisterRequest represents user registration request
type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginRequest represents user login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse represents authentication response with token
type AuthResponse struct {
	Success      bool   `json:"success"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	User         *UserResponse `json:"user,omitempty"`
	Error        string `json:"error,omitempty"`
	Message      string `json:"message,omitempty"`
}

// UserResponse represents user data in API responses
type UserResponse struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	IsActive bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Register handles POST /api/v1/auth/register
func (h *AuthAPIHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// TODO: Validate input
	if req.Name == "" || req.Email == "" || req.Password == "" {
		h.writeErrorJSON(w, http.StatusBadRequest, "Name, email, and password are required")
		return
	}

	// TODO: Call user service to create user
	h.logger.Info("User registration attempt", zap.String("email", req.Email))

	// Mock response for now
	response := AuthResponse{
		Success: true,
		Message: "User registered successfully",
		User: &UserResponse{
			ID:       "mock-user-id",
			Name:     req.Name,
			Email:    req.Email,
			Role:     "user",
			IsActive: true,
			CreatedAt: time.Now(),
		},
	}

	h.writeJSON(w, http.StatusCreated, response)
}

// Login handles POST /api/v1/auth/login
func (h *AuthAPIHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// TODO: Validate credentials with user service
	if req.Email == "" || req.Password == "" {
		h.writeErrorJSON(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	h.logger.Info("User login attempt", zap.String("email", req.Email))

	// TODO: Generate actual JWT tokens
	// Mock response for now
	response := AuthResponse{
		Success:      true,
		AccessToken:  "mock-jwt-access-token",
		RefreshToken: "mock-jwt-refresh-token", 
		ExpiresIn:    3600, // 1 hour
		Message:      "Login successful",
		User: &UserResponse{
			ID:       "mock-user-id",
			Name:     "Mock User",
			Email:    req.Email,
			Role:     "user",
			IsActive: true,
			CreatedAt: time.Now().Add(-24 * time.Hour), // Created yesterday
		},
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthAPIHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	// TODO: Invalidate JWT token (add to blacklist)
	
	response := APIResponse{
		Success: true,
		Message: "Logout successful",
	}

	h.writeJSON(w, http.StatusOK, response)
}

// RefreshToken handles POST /api/v1/auth/refresh
func (h *AuthAPIHandlers) RefreshToken(w http.ResponseWriter, r *http.Request) {
	// TODO: Validate refresh token and generate new access token
	
	response := AuthResponse{
		Success:     true,
		AccessToken: "new-mock-jwt-access-token",
		ExpiresIn:   3600,
		Message:     "Token refreshed successfully",
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetProfile handles GET /api/v1/auth/profile
func (h *AuthAPIHandlers) GetProfile(w http.ResponseWriter, r *http.Request) {
	// Get user info from context (set by auth middleware)
	userID, exists := middleware.GetUserIDFromContext(r.Context())
	if !exists {
		h.writeErrorJSON(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// TODO: Get user profile from user service
	h.logger.Info("Get profile request", zap.String("user_id", userID))

	// Mock response for now
	response := APIResponse{
		Success: true,
		Data: UserResponse{
			ID:       userID,
			Name:     "Mock User",
			Email:    "user@example.com",
			Role:     "user", 
			IsActive: true,
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
		Message: "Profile retrieved successfully",
	}

	h.writeJSON(w, http.StatusOK, response)
}

// UpdateProfile handles PUT /api/v1/auth/profile
func (h *AuthAPIHandlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	// Get user info from context
	userID, exists := middleware.GetUserIDFromContext(r.Context())
	if !exists {
		h.writeErrorJSON(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var updateReq struct {
		Name string `json:"name"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	// TODO: Update user profile via user service
	h.logger.Info("Update profile request", 
		zap.String("user_id", userID),
		zap.String("new_name", updateReq.Name))

	response := APIResponse{
		Success: true,
		Message: "Profile updated successfully",
	}

	h.writeJSON(w, http.StatusOK, response)
}

// Helper methods

func (h *AuthAPIHandlers) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode JSON response", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *AuthAPIHandlers) writeErrorJSON(w http.ResponseWriter, status int, message string) {
	response := AuthResponse{
		Success: false,
		Error:   message,
	}
	h.writeJSON(w, status, response)
}