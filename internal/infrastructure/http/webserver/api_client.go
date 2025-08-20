// Package webserver provides API client for backend communication
package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"go.uber.org/zap"
)

// APIClient handles communication with the backend API
type APIClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewAPIClient creates a new API client instance
func NewAPIClient(cfg *config.Config, logger *zap.Logger) *APIClient {
	// Get API URL from environment or config
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = fmt.Sprintf("http://localhost:3000")
	}

	return &APIClient{
		baseURL: apiURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// Authentication

// LoginRequest represents login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Success      bool         `json:"success"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
	User         UserResponse `json:"user"`
	Error        string       `json:"error,omitempty"`
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterResponse represents registration response
type RegisterResponse struct {
	Success bool         `json:"success"`
	User    UserResponse `json:"user"`
	Message string       `json:"message"`
	Error   string       `json:"error,omitempty"`
}

// UserResponse represents user data in API responses
type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// RecipeResponse represents recipe data
type RecipeResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	AuthorID    string    `json:"author_id"`
	AuthorName  string    `json:"author_name"`
	CookTime    int       `json:"cook_time"`
	PrepTime    int       `json:"prep_time"`
	Servings    int       `json:"servings"`
	Difficulty  string    `json:"difficulty"`
	Cuisine     string    `json:"cuisine"`
	Category    string    `json:"category"`
	ImageURL    string    `json:"image_url"`
	Likes       int       `json:"likes"`
	Rating      float64   `json:"rating"`
	CreatedAt   time.Time `json:"created_at"`
}

// Login authenticates a user with the API
func (c *APIClient) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	req := LoginRequest{
		Email:    email,
		Password: password,
	}

	var resp LoginResponse
	err := c.post(ctx, "/api/v1/auth/login", req, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("login failed: %s", resp.Error)
	}

	return &resp, nil
}

// Register creates a new user account
func (c *APIClient) Register(ctx context.Context, name, email, password string) (*RegisterResponse, error) {
	req := RegisterRequest{
		Name:     name,
		Email:    email,
		Password: password,
	}

	var resp RegisterResponse
	err := c.post(ctx, "/api/v1/auth/register", req, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("registration failed: %s", resp.Error)
	}

	return &resp, nil
}

// VerifyToken checks if an access token is still valid
func (c *APIClient) VerifyToken(ctx context.Context, token string) bool {
	if token == "" {
		return false
	}

	// Call profile endpoint to verify token
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v1/auth/profile", nil)
	if err != nil {
		return false
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Debug("Token verification failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// VerifyConnection checks if the API backend is reachable
func (c *APIClient) VerifyConnection(ctx context.Context) bool {
	// Call health endpoint to verify connection
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		c.logger.Debug("Connection verification request creation failed", zap.Error(err))
		return false
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Debug("Connection verification failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}

// GetProfile gets the current user's profile
func (c *APIClient) GetProfile(ctx context.Context, token string) (*UserResponse, error) {
	var resp struct {
		Success bool         `json:"success"`
		Data    UserResponse `json:"data"`
		Error   string       `json:"error,omitempty"`
	}

	err := c.getWithAuth(ctx, "/api/v1/auth/profile", token, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to get profile: %s", resp.Error)
	}

	return &resp.Data, nil
}

// Recipes

// GetRecipes fetches the list of recipes
func (c *APIClient) GetRecipes(ctx context.Context, token string) ([]RecipeResponse, error) {
	var resp struct {
		Success bool             `json:"success"`
		Data    []RecipeResponse `json:"data"`
		Error   string           `json:"error,omitempty"`
	}

	err := c.getWithAuth(ctx, "/api/v1/recipes", token, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to get recipes: %s", resp.Error)
	}

	return resp.Data, nil
}

// GetRecipe fetches a single recipe by ID
func (c *APIClient) GetRecipe(ctx context.Context, token, recipeID string) (*RecipeResponse, error) {
	var resp struct {
		Success bool           `json:"success"`
		Data    RecipeResponse `json:"data"`
		Error   string         `json:"error,omitempty"`
	}

	err := c.getWithAuth(ctx, "/api/v1/recipes/"+recipeID, token, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to get recipe: %s", resp.Error)
	}

	return &resp.Data, nil
}

// CreateRecipe creates a new recipe
func (c *APIClient) CreateRecipe(ctx context.Context, token string, recipe RecipeResponse) (*RecipeResponse, error) {
	var resp struct {
		Success bool           `json:"success"`
		Data    RecipeResponse `json:"data"`
		Error   string         `json:"error,omitempty"`
	}

	err := c.postWithAuth(ctx, "/api/v1/recipes", token, recipe, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to create recipe: %s", resp.Error)
	}

	return &resp.Data, nil
}

// AI Features

// GenerateRecipe generates a recipe using AI
func (c *APIClient) GenerateRecipe(ctx context.Context, token, prompt string) (*RecipeResponse, error) {
	req := map[string]interface{}{
		"prompt": prompt,
	}

	var resp struct {
		Success bool           `json:"success"`
		Data    RecipeResponse `json:"data"`
		Error   string         `json:"error,omitempty"`
	}

	err := c.postWithAuth(ctx, "/api/v1/ai/generate-recipe", token, req, &resp)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("failed to generate recipe: %s", resp.Error)
	}

	return &resp.Data, nil
}

// Helper methods

func (c *APIClient) post(ctx context.Context, path string, body interface{}, response interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return c.doRequest(req, response)
}

func (c *APIClient) postWithAuth(ctx context.Context, path, token string, body interface{}, response interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	return c.doRequest(req, response)
}

func (c *APIClient) getWithAuth(ctx context.Context, path, token string, response interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	return c.doRequest(req, response)
}

func (c *APIClient) doRequest(req *http.Request, response interface{}) error {
	c.logger.Debug("API request",
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		c.logger.Error("API error response",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		return fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}