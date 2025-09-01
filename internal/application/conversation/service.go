package conversation

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ConversationStatus represents the status of a conversation
type ConversationStatus string

const (
	StatusActive   ConversationStatus = "active"
	StatusArchived ConversationStatus = "archived"
	StatusDeleted  ConversationStatus = "deleted"
)

// MessageRole represents the role of a message sender
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
)

// ConversationIntent represents the detected intent of a conversation
type ConversationIntent string

const (
	IntentRecipeCreation      ConversationIntent = "recipe_creation"
	IntentCookingHelp         ConversationIntent = "cooking_help"
	IntentIngredientSubst     ConversationIntent = "ingredient_substitution"
	IntentMealPlanning        ConversationIntent = "meal_planning"
	IntentGeneralQuestion     ConversationIntent = "general_question"
	IntentTechnicalSupport    ConversationIntent = "technical_support"
	IntentGeneral             ConversationIntent = "general"
)

// Conversation represents a conversation
type Conversation struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Title     string                 `json:"title"`
	Intent    ConversationIntent     `json:"intent"`
	Status    ConversationStatus     `json:"status"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// Message represents a message in a conversation
type Message struct {
	ID               string                 `json:"id"`
	ConversationID   string                 `json:"conversation_id"`
	Role             MessageRole            `json:"role"`
	Content          string                 `json:"content"`
	Metadata         map[string]interface{} `json:"metadata"`
	TokensUsed       int                    `json:"tokens_used"`
	ProcessingTimeMs int                    `json:"processing_time_ms"`
	ModelUsed        string                 `json:"model_used,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
}

// ConversationContext represents the context and state of a conversation
type ConversationContext struct {
	ConversationID string                 `json:"conversation_id"`
	ContextType    string                 `json:"context_type"`
	ContextData    map[string]interface{} `json:"context_data"`
	Complexity     string                 `json:"complexity"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// Repository interfaces
type ConversationRepository interface {
	CreateConversation(ctx context.Context, conversation *Conversation) error
	GetConversation(ctx context.Context, id string) (*Conversation, error)
	GetUserConversations(ctx context.Context, userID string, limit, offset int) ([]*Conversation, error)
	UpdateConversation(ctx context.Context, conversation *Conversation) error
	DeleteConversation(ctx context.Context, id string) error
}

type MessageRepository interface {
	CreateMessage(ctx context.Context, message *Message) error
	GetMessage(ctx context.Context, id string) (*Message, error)
	GetConversationMessages(ctx context.Context, conversationID string, limit, offset int) ([]*Message, error)
	UpdateMessage(ctx context.Context, message *Message) error
	DeleteMessage(ctx context.Context, id string) error
}

type ContextRepository interface {
	SetContext(ctx context.Context, context *ConversationContext) error
	GetContext(ctx context.Context, conversationID, contextType string) (*ConversationContext, error)
	GetAllContext(ctx context.Context, conversationID string) ([]*ConversationContext, error)
	DeleteContext(ctx context.Context, conversationID, contextType string) error
}

// IntentClassifier classifies user messages to determine intent
type IntentClassifier struct {
	recipePatterns      []*regexp.Regexp
	helpPatterns        []*regexp.Regexp
	substitutionPatterns []*regexp.Regexp
	planningPatterns    []*regexp.Regexp
}

// NewIntentClassifier creates a new intent classifier
func NewIntentClassifier() *IntentClassifier {
	return &IntentClassifier{
		recipePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\b(create|make|generate|cook|recipe for|how to make)\b.*\b(recipe|dish|food)\b`),
			regexp.MustCompile(`(?i)\bi want to (make|cook|create|prepare)\b`),
			regexp.MustCompile(`(?i)\brecipe (for|with|using)\b`),
			regexp.MustCompile(`(?i)\b(show me|give me|suggest) (a|some)? ?recipe\b`),
			regexp.MustCompile(`(?i)\b(can you|please) (make|create|generate)\b.*\brecipe\b`),
		},
		helpPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\bhow (to|do i)\b`),
			regexp.MustCompile(`(?i)\b(what is|what's|explain|help me)\b`),
			regexp.MustCompile(`(?i)\b(cooking|baking|technique|method)\b`),
			regexp.MustCompile(`(?i)\b(temperature|time|duration)\b`),
			regexp.MustCompile(`(?i)\b(tips|advice|suggestions)\b`),
		},
		substitutionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\b(substitute|replace|alternative|instead of)\b`),
			regexp.MustCompile(`(?i)\bdon't have\b`),
			regexp.MustCompile(`(?i)\bwhat can i use (for|instead)\b`),
			regexp.MustCompile(`(?i)\b(out of|ran out)\b`),
		},
		planningPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\bmeal plan(ning)?\b`),
			regexp.MustCompile(`(?i)\bweek(ly)? menu\b`),
			regexp.MustCompile(`(?i)\bwhat should i (cook|make) (today|tomorrow|this week)\b`),
			regexp.MustCompile(`(?i)\bmeal prep\b`),
		},
	}
}

// ClassifyIntent analyzes a message and determines the conversation intent
func (ic *IntentClassifier) ClassifyIntent(message string) ConversationIntent {
	message = strings.ToLower(strings.TrimSpace(message))
	
	// Check recipe creation patterns first (highest priority)
	for _, pattern := range ic.recipePatterns {
		if pattern.MatchString(message) {
			return IntentRecipeCreation
		}
	}
	
	// Check substitution patterns
	for _, pattern := range ic.substitutionPatterns {
		if pattern.MatchString(message) {
			return IntentIngredientSubst
		}
	}
	
	// Check meal planning patterns
	for _, pattern := range ic.planningPatterns {
		if pattern.MatchString(message) {
			return IntentMealPlanning
		}
	}
	
	// Check help patterns
	for _, pattern := range ic.helpPatterns {
		if pattern.MatchString(message) {
			return IntentCookingHelp
		}
	}
	
	// Default to general question
	return IntentGeneralQuestion
}

// Service handles conversation operations
type Service struct {
	conversationRepo ConversationRepository
	messageRepo      MessageRepository
	contextRepo      ContextRepository
	intentClassifier *IntentClassifier
	aiService        *AIService
}

// NewService creates a new conversation service
func NewService(
	conversationRepo ConversationRepository,
	messageRepo MessageRepository,
	contextRepo ContextRepository,
	aiService *AIService,
) *Service {
	return &Service{
		conversationRepo: conversationRepo,
		messageRepo:      messageRepo,
		contextRepo:      contextRepo,
		intentClassifier: NewIntentClassifier(),
		aiService:        aiService,
	}
}

// CreateConversation creates a new conversation
func (s *Service) CreateConversation(ctx context.Context, userID string, firstMessage string) (*Conversation, error) {
	// Classify intent from first message
	intent := s.intentClassifier.ClassifyIntent(firstMessage)
	
	conversation := &Conversation{
		ID:        uuid.New().String(),
		UserID:    userID,
		Intent:    intent,
		Status:    StatusActive,
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Generate title based on intent
	conversation.Title = s.generateConversationTitle(intent, firstMessage)
	
	err := s.conversationRepo.CreateConversation(ctx, conversation)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	
	return conversation, nil
}

// AddMessage adds a message to a conversation
func (s *Service) AddMessage(ctx context.Context, conversationID string, role MessageRole, content string, metadata map[string]interface{}) (*Message, error) {
	message := &Message{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		Metadata:       metadata,
		CreatedAt:      time.Now(),
	}
	
	err := s.messageRepo.CreateMessage(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}
	
	return message, nil
}

// GetConversation retrieves a conversation by ID
func (s *Service) GetConversation(ctx context.Context, id string) (*Conversation, error) {
	return s.conversationRepo.GetConversation(ctx, id)
}

// GetConversationWithMessages retrieves a conversation with its messages
func (s *Service) GetConversationWithMessages(ctx context.Context, conversationID string) (*Conversation, []*Message, error) {
	conversation, err := s.conversationRepo.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get conversation: %w", err)
	}
	
	messages, err := s.messageRepo.GetConversationMessages(ctx, conversationID, 100, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get messages: %w", err)
	}
	
	return conversation, messages, nil
}

// GetUserConversations retrieves all conversations for a user
func (s *Service) GetUserConversations(ctx context.Context, userID string, limit, offset int) ([]*Conversation, error) {
	return s.conversationRepo.GetUserConversations(ctx, userID, limit, offset)
}

// SetContext sets context data for a conversation
func (s *Service) SetContext(ctx context.Context, conversationID, contextType string, data map[string]interface{}) error {
	context := &ConversationContext{
		ConversationID: conversationID,
		ContextType:    contextType,
		ContextData:    data,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	
	return s.contextRepo.SetContext(ctx, context)
}

// GetContext retrieves context data for a conversation
func (s *Service) GetContext(ctx context.Context, conversationID, contextType string) (map[string]interface{}, error) {
	context, err := s.contextRepo.GetContext(ctx, conversationID, contextType)
	if err != nil {
		return nil, err
	}
	
	return context.ContextData, nil
}

// ProcessMessage processes an incoming user message and returns appropriate response
func (s *Service) ProcessMessage(ctx context.Context, conversationID, userMessage string, userID string) (*Message, string, error) {
	// Add user message to conversation
	userMsg, err := s.AddMessage(ctx, conversationID, RoleUser, userMessage, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to add user message: %w", err)
	}
	
	// Get conversation to understand context and intent
	conversation, err := s.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get conversation: %w", err)
	}
	
	// Generate AI response based on intent and context
	response, err := s.generateResponse(ctx, conversation, userMessage)
	if err != nil {
		return userMsg, "", fmt.Errorf("failed to generate response: %w", err)
	}
	
	return userMsg, response, nil
}

// generateResponse generates an appropriate AI response
func (s *Service) generateResponse(ctx context.Context, conversation *Conversation, userMessage string) (string, error) {
	if s.aiService == nil {
		// Fall back to simple responses
		return s.generateSimpleResponse(ctx, conversation, userMessage)
	}

	// Get conversation messages for context
	messages, err := s.messageRepo.GetConversationMessages(ctx, conversation.ID, 20, 0)
	if err != nil {
		log.Printf("Failed to get conversation messages for AI context: %v", err)
		messages = []*Message{} // Continue with empty context
	}

	// Generate AI response
	aiResponse, err := s.aiService.GenerateConversationalResponse(ctx, conversation, messages, userMessage)
	if err != nil {
		log.Printf("AI service failed, falling back to simple response: %v", err)
		return s.generateSimpleResponse(ctx, conversation, userMessage)
	}

	// Store AI metadata if needed
	if aiResponse.Metadata != nil {
		err = s.SetContext(ctx, conversation.ID, "ai_metadata", aiResponse.Metadata)
		if err != nil {
			log.Printf("Failed to store AI metadata: %v", err)
		}
	}

	return aiResponse.Content, nil
}

// generateSimpleResponse generates simple fallback responses
func (s *Service) generateSimpleResponse(ctx context.Context, conversation *Conversation, userMessage string) (string, error) {
	switch conversation.Intent {
	case IntentRecipeCreation:
		return s.generateRecipeCreationResponse(ctx, conversation, userMessage)
	case IntentCookingHelp:
		return s.generateCookingHelpResponse(ctx, conversation, userMessage)
	case IntentIngredientSubst:
		return s.generateSubstitutionResponse(ctx, conversation, userMessage)
	case IntentMealPlanning:
		return s.generateMealPlanningResponse(ctx, conversation, userMessage)
	default:
		return s.generateGeneralResponse(ctx, conversation, userMessage)
	}
}

// generateConversationTitle generates a title based on intent and first message
func (s *Service) generateConversationTitle(intent ConversationIntent, message string) string {
	switch intent {
	case IntentRecipeCreation:
		// Try to extract dish name from message
		if dishName := s.extractDishName(message); dishName != "" {
			return fmt.Sprintf("Recipe: %s", dishName)
		}
		return "New Recipe Creation"
	case IntentCookingHelp:
		return "Cooking Help"
	case IntentIngredientSubst:
		return "Ingredient Substitution"
	case IntentMealPlanning:
		return "Meal Planning"
	default:
		return "Chat with AI Chef"
	}
}

// extractDishName attempts to extract a dish name from a message
func (s *Service) extractDishName(message string) string {
	// Simple patterns to extract dish names
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)recipe for (.+?)(?:\.|$|\s+please|\s+with)`),
		regexp.MustCompile(`(?i)make (.+?)(?:\.|$|\s+please|\s+with)`),
		regexp.MustCompile(`(?i)cook (.+?)(?:\.|$|\s+please|\s+with)`),
		regexp.MustCompile(`(?i)create (.+?)(?:\.|$|\s+please|\s+with)`),
	}
	
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(message)
		if len(matches) > 1 {
			dishName := strings.TrimSpace(matches[1])
			if len(dishName) > 0 && len(dishName) < 50 {
				return strings.Title(dishName)
			}
		}
	}
	
	return ""
}

// Response generation methods (these would integrate with your AI service)
func (s *Service) generateRecipeCreationResponse(ctx context.Context, conversation *Conversation, message string) (string, error) {
	// This would integrate with your existing AI recipe generation logic
	return "I'd be happy to help you create a recipe! Let me understand what you'd like to make. Can you tell me more about the dish, preferred ingredients, or any dietary requirements?", nil
}

func (s *Service) generateCookingHelpResponse(ctx context.Context, conversation *Conversation, message string) (string, error) {
	return "I'm here to help with your cooking question! What specific technique or cooking challenge can I assist you with?", nil
}

func (s *Service) generateSubstitutionResponse(ctx context.Context, conversation *Conversation, message string) (string, error) {
	return "I can help you find the perfect ingredient substitution! What ingredient do you need to replace, and what are you cooking?", nil
}

func (s *Service) generateMealPlanningResponse(ctx context.Context, conversation *Conversation, message string) (string, error) {
	return "Let's plan some delicious meals! What's your timeframe, dietary preferences, and how many people are you cooking for?", nil
}

func (s *Service) generateGeneralResponse(ctx context.Context, conversation *Conversation, message string) (string, error) {
	return "I'm your AI chef assistant! I can help you create recipes, answer cooking questions, suggest ingredient substitutions, or plan meals. What would you like to explore?", nil
}

// ArchiveConversation archives a conversation
func (s *Service) ArchiveConversation(ctx context.Context, conversationID string) error {
	conversation, err := s.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	
	conversation.Status = StatusArchived
	conversation.UpdatedAt = time.Now()
	
	return s.conversationRepo.UpdateConversation(ctx, conversation)
}

// DeleteConversation soft deletes a conversation
func (s *Service) DeleteConversation(ctx context.Context, conversationID string) error {
	conversation, err := s.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	
	conversation.Status = StatusDeleted
	conversation.UpdatedAt = time.Now()
	
	return s.conversationRepo.UpdateConversation(ctx, conversation)
}

// GetConversationStats returns statistics about conversations
func (s *Service) GetConversationStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	conversations, err := s.GetUserConversations(ctx, userID, 1000, 0)
	if err != nil {
		return nil, err
	}
	
	stats := map[string]interface{}{
		"total_conversations": len(conversations),
		"active_conversations": 0,
		"archived_conversations": 0,
		"intents": make(map[ConversationIntent]int),
	}
	
	intentCounts := make(map[ConversationIntent]int)
	activeCount := 0
	archivedCount := 0
	
	for _, conv := range conversations {
		switch conv.Status {
		case StatusActive:
			activeCount++
		case StatusArchived:
			archivedCount++
		}
		
		intentCounts[conv.Intent]++
	}
	
	stats["active_conversations"] = activeCount
	stats["archived_conversations"] = archivedCount
	stats["intents"] = intentCounts
	
	return stats, nil
}