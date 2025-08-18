// Package ai defines AI-related domain entities and services
package ai

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// AIRequest represents a request to an AI service
type AIRequest struct {
	id           uuid.UUID
	userID       uuid.UUID
	prompt       string
	provider     ProviderType
	model        string
	parameters   map[string]interface{}
	status       RequestStatus
	response     *AIResponse
	tokensUsed   int
	costCents    int
	createdAt    time.Time
	completedAt  *time.Time
	errorMessage string
}

// AIResponse represents the response from an AI service
type AIResponse struct {
	content      string
	metadata     map[string]interface{}
	finishReason FinishReason
	usage        *TokenUsage
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ProviderType represents different AI providers
type ProviderType string

const (
	ProviderTypeOpenAI     ProviderType = "openai"
	ProviderTypeAnthropic  ProviderType = "anthropic"
	ProviderTypeMock       ProviderType = "mock"
)

// RequestStatus represents the status of an AI request
type RequestStatus string

const (
	RequestStatusPending   RequestStatus = "pending"
	RequestStatusProcessing RequestStatus = "processing"
	RequestStatusCompleted RequestStatus = "completed"
	RequestStatusFailed    RequestStatus = "failed"
	RequestStatusCanceled  RequestStatus = "canceled"
)

// FinishReason represents why the AI stopped generating
type FinishReason string

const (
	FinishReasonStop       FinishReason = "stop"
	FinishReasonLength     FinishReason = "length"
	FinishReasonTimeout    FinishReason = "timeout"
	FinishReasonError      FinishReason = "error"
)

// NewAIRequest creates a new AI request
func NewAIRequest(userID uuid.UUID, prompt string, provider ProviderType, model string) (*AIRequest, error) {
	if err := validatePrompt(prompt); err != nil {
		return nil, err
	}

	return &AIRequest{
		id:         uuid.New(),
		userID:     userID,
		prompt:     prompt,
		provider:   provider,
		model:      model,
		status:     RequestStatusPending,
		parameters: make(map[string]interface{}),
		createdAt:  time.Now(),
	}, nil
}

// ID returns the request ID
func (r *AIRequest) ID() uuid.UUID {
	return r.id
}

// UserID returns the user ID
func (r *AIRequest) UserID() uuid.UUID {
	return r.userID
}

// Prompt returns the prompt
func (r *AIRequest) Prompt() string {
	return r.prompt
}

// Provider returns the AI provider
func (r *AIRequest) Provider() ProviderType {
	return r.provider
}

// Model returns the AI model
func (r *AIRequest) Model() string {
	return r.model
}

// Status returns the request status
func (r *AIRequest) Status() RequestStatus {
	return r.status
}

// Response returns the AI response
func (r *AIRequest) Response() *AIResponse {
	return r.response
}

// TokensUsed returns the number of tokens used
func (r *AIRequest) TokensUsed() int {
	return r.tokensUsed
}

// CostCents returns the cost in cents
func (r *AIRequest) CostCents() int {
	return r.costCents
}

// CreatedAt returns when the request was created
func (r *AIRequest) CreatedAt() time.Time {
	return r.createdAt
}

// CompletedAt returns when the request was completed
func (r *AIRequest) CompletedAt() *time.Time {
	return r.completedAt
}

// ErrorMessage returns any error message
func (r *AIRequest) ErrorMessage() string {
	return r.errorMessage
}

// SetParameter sets a parameter for the request
func (r *AIRequest) SetParameter(key string, value interface{}) {
	r.parameters[key] = value
}

// GetParameter gets a parameter value
func (r *AIRequest) GetParameter(key string) (interface{}, bool) {
	value, exists := r.parameters[key]
	return value, exists
}

// Parameters returns all parameters
func (r *AIRequest) Parameters() map[string]interface{} {
	return r.parameters
}

// StartProcessing marks the request as processing
func (r *AIRequest) StartProcessing() error {
	if r.status != RequestStatusPending {
		return errors.New("can only start processing pending requests")
	}
	
	r.status = RequestStatusProcessing
	return nil
}

// Complete marks the request as completed with response
func (r *AIRequest) Complete(response *AIResponse) error {
	if r.status != RequestStatusProcessing {
		return errors.New("can only complete processing requests")
	}
	
	r.status = RequestStatusCompleted
	r.response = response
	
	if response.usage != nil {
		r.tokensUsed = response.usage.TotalTokens
	}
	
	now := time.Now()
	r.completedAt = &now
	
	return nil
}

// Fail marks the request as failed
func (r *AIRequest) Fail(errorMessage string) error {
	if r.status == RequestStatusCompleted {
		return errors.New("cannot fail a completed request")
	}
	
	r.status = RequestStatusFailed
	r.errorMessage = errorMessage
	
	now := time.Now()
	r.completedAt = &now
	
	return nil
}

// Cancel cancels the request
func (r *AIRequest) Cancel() error {
	if r.status == RequestStatusCompleted || r.status == RequestStatusFailed {
		return errors.New("cannot cancel completed or failed request")
	}
	
	r.status = RequestStatusCanceled
	
	now := time.Now()
	r.completedAt = &now
	
	return nil
}

// NewAIResponse creates a new AI response
func NewAIResponse(content string, usage *TokenUsage, finishReason FinishReason) *AIResponse {
	return &AIResponse{
		content:      content,
		usage:        usage,
		finishReason: finishReason,
		metadata:     make(map[string]interface{}),
	}
}

// Content returns the response content
func (r *AIResponse) Content() string {
	return r.content
}

// Usage returns token usage
func (r *AIResponse) Usage() *TokenUsage {
	return r.usage
}

// FinishReason returns why generation stopped
func (r *AIResponse) FinishReason() FinishReason {
	return r.finishReason
}

// SetMetadata sets metadata
func (r *AIResponse) SetMetadata(key string, value interface{}) {
	r.metadata[key] = value
}

// GetMetadata gets metadata
func (r *AIResponse) GetMetadata(key string) (interface{}, bool) {
	value, exists := r.metadata[key]
	return value, exists
}

// RecipeGenerationRequest represents a specific request for recipe generation
type RecipeGenerationRequest struct {
	aiRequest           *AIRequest
	ingredients         []string
	cuisine             string
	dietaryRestrictions []string
	servings            int
	difficulty          string
	cookingTime         time.Duration
	equipment           []string
}

// NewRecipeGenerationRequest creates a new recipe generation request
func NewRecipeGenerationRequest(
	userID uuid.UUID,
	ingredients []string,
	cuisine string,
	dietaryRestrictions []string,
	servings int,
) (*RecipeGenerationRequest, error) {
	if len(ingredients) == 0 {
		return nil, errors.New("at least one ingredient is required")
	}
	
	if servings <= 0 {
		return nil, errors.New("servings must be greater than 0")
	}

	// Build prompt from parameters
	prompt := buildRecipePrompt(ingredients, cuisine, dietaryRestrictions, servings)
	
	aiRequest, err := NewAIRequest(userID, prompt, ProviderTypeMock, "recipe-generator")
	if err != nil {
		return nil, err
	}

	return &RecipeGenerationRequest{
		aiRequest:           aiRequest,
		ingredients:         ingredients,
		cuisine:             cuisine,
		dietaryRestrictions: dietaryRestrictions,
		servings:            servings,
	}, nil
}

// AIRequest returns the underlying AI request
func (r *RecipeGenerationRequest) AIRequest() *AIRequest {
	return r.aiRequest
}

// Ingredients returns the ingredients
func (r *RecipeGenerationRequest) Ingredients() []string {
	return r.ingredients
}

// Cuisine returns the cuisine type
func (r *RecipeGenerationRequest) Cuisine() string {
	return r.cuisine
}

// DietaryRestrictions returns dietary restrictions
func (r *RecipeGenerationRequest) DietaryRestrictions() []string {
	return r.dietaryRestrictions
}

// Servings returns the number of servings
func (r *RecipeGenerationRequest) Servings() int {
	return r.servings
}

// Helper functions
func validatePrompt(prompt string) error {
	if prompt == "" {
		return errors.New("prompt cannot be empty")
	}
	
	if len(prompt) > 10000 {
		return errors.New("prompt too long")
	}
	
	return nil
}

func buildRecipePrompt(ingredients []string, cuisine string, dietaryRestrictions []string, servings int) string {
	prompt := "Generate a recipe using the following ingredients: "
	for i, ingredient := range ingredients {
		if i > 0 {
			prompt += ", "
		}
		prompt += ingredient
	}
	
	prompt += ". "
	
	if cuisine != "" {
		prompt += "The recipe should be " + cuisine + " cuisine. "
	}
	
	if len(dietaryRestrictions) > 0 {
		prompt += "Please accommodate these dietary restrictions: "
		for i, restriction := range dietaryRestrictions {
			if i > 0 {
				prompt += ", "
			}
			prompt += restriction
		}
		prompt += ". "
	}
	
	prompt += "The recipe should serve " + string(rune(servings)) + " people. "
	prompt += "Please provide a complete recipe with ingredients list, instructions, and estimated cooking time."
	
	return prompt
}