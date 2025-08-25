// Package testutils provides conversation-specific mock implementations
package testutils

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/stretchr/testify/mock"
)

// MockConversationRepository provides a mock implementation of ConversationRepository
type MockConversationRepository struct {
	mock.Mock
	conversations map[string]*conversation.Conversation
	mu           sync.RWMutex
}

// NewMockConversationRepository creates a new mock conversation repository
func NewMockConversationRepository() *MockConversationRepository {
	return &MockConversationRepository{
		conversations: make(map[string]*conversation.Conversation),
	}
}

// CreateConversation creates a conversation
func (m *MockConversationRepository) CreateConversation(ctx context.Context, conv *conversation.Conversation) error {
	args := m.Called(ctx, conv)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.conversations[conv.ID] = conv
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// GetConversation retrieves a conversation by ID
func (m *MockConversationRepository) GetConversation(ctx context.Context, id string) (*conversation.Conversation, error) {
	args := m.Called(ctx, id)
	
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if conv, exists := m.conversations[id]; exists {
		return conv, nil
	}
	
	return args.Get(0).(*conversation.Conversation), args.Error(1)
}

// GetUserConversations retrieves conversations for a user
func (m *MockConversationRepository) GetUserConversations(ctx context.Context, userID string, limit, offset int) ([]*conversation.Conversation, error) {
	args := m.Called(ctx, userID, limit, offset)
	return args.Get(0).([]*conversation.Conversation), args.Error(1)
}

// UpdateConversation updates a conversation
func (m *MockConversationRepository) UpdateConversation(ctx context.Context, conv *conversation.Conversation) error {
	args := m.Called(ctx, conv)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.conversations[conv.ID] = conv
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// DeleteConversation deletes a conversation
func (m *MockConversationRepository) DeleteConversation(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		delete(m.conversations, id)
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// SetupStandardMockBehavior sets up standard mock behaviors
func (m *MockConversationRepository) SetupStandardMockBehavior() {
	// CreateConversation succeeds by default
	m.On("CreateConversation", mock.Anything, mock.AnythingOfType("*conversation.Conversation")).
		Return(nil).Maybe()
	
	// GetConversation returns not found by default
	m.On("GetConversation", mock.Anything, mock.AnythingOfType("string")).
		Return((*conversation.Conversation)(nil), fmt.Errorf("conversation not found")).Maybe()
	
	// GetUserConversations returns empty list by default
	m.On("GetUserConversations", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return([]*conversation.Conversation{}, nil).Maybe()
	
	// UpdateConversation succeeds by default
	m.On("UpdateConversation", mock.Anything, mock.AnythingOfType("*conversation.Conversation")).
		Return(nil).Maybe()
	
	// DeleteConversation succeeds by default
	m.On("DeleteConversation", mock.Anything, mock.AnythingOfType("string")).
		Return(nil).Maybe()
}

// MockMessageRepository provides a mock implementation of MessageRepository
type MockMessageRepository struct {
	mock.Mock
	messages map[string]*conversation.Message
	mu       sync.RWMutex
}

// NewMockMessageRepository creates a new mock message repository
func NewMockMessageRepository() *MockMessageRepository {
	return &MockMessageRepository{
		messages: make(map[string]*conversation.Message),
	}
}

// CreateMessage creates a message
func (m *MockMessageRepository) CreateMessage(ctx context.Context, msg *conversation.Message) error {
	args := m.Called(ctx, msg)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.messages[msg.ID] = msg
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// GetMessage retrieves a message by ID
func (m *MockMessageRepository) GetMessage(ctx context.Context, id string) (*conversation.Message, error) {
	args := m.Called(ctx, id)
	
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if msg, exists := m.messages[id]; exists {
		return msg, nil
	}
	
	return args.Get(0).(*conversation.Message), args.Error(1)
}

// GetConversationMessages retrieves messages for a conversation
func (m *MockMessageRepository) GetConversationMessages(ctx context.Context, conversationID string, limit, offset int) ([]*conversation.Message, error) {
	args := m.Called(ctx, conversationID, limit, offset)
	return args.Get(0).([]*conversation.Message), args.Error(1)
}

// UpdateMessage updates a message
func (m *MockMessageRepository) UpdateMessage(ctx context.Context, msg *conversation.Message) error {
	args := m.Called(ctx, msg)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		m.messages[msg.ID] = msg
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// DeleteMessage deletes a message
func (m *MockMessageRepository) DeleteMessage(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		delete(m.messages, id)
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// SetupStandardMockBehavior sets up standard mock behaviors
func (m *MockMessageRepository) SetupStandardMockBehavior() {
	// CreateMessage succeeds by default
	m.On("CreateMessage", mock.Anything, mock.AnythingOfType("*conversation.Message")).
		Return(nil).Maybe()
	
	// GetMessage returns not found by default
	m.On("GetMessage", mock.Anything, mock.AnythingOfType("string")).
		Return((*conversation.Message)(nil), fmt.Errorf("message not found")).Maybe()
	
	// GetConversationMessages returns empty list by default
	m.On("GetConversationMessages", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("int"), mock.AnythingOfType("int")).
		Return([]*conversation.Message{}, nil).Maybe()
	
	// UpdateMessage succeeds by default
	m.On("UpdateMessage", mock.Anything, mock.AnythingOfType("*conversation.Message")).
		Return(nil).Maybe()
	
	// DeleteMessage succeeds by default
	m.On("DeleteMessage", mock.Anything, mock.AnythingOfType("string")).
		Return(nil).Maybe()
}

// MockContextRepository provides a mock implementation of ContextRepository
type MockContextRepository struct {
	mock.Mock
	contexts map[string]map[string]*conversation.ConversationContext // conversationID -> contextType -> context
	mu       sync.RWMutex
}

// NewMockContextRepository creates a new mock context repository
func NewMockContextRepository() *MockContextRepository {
	return &MockContextRepository{
		contexts: make(map[string]map[string]*conversation.ConversationContext),
	}
}

// SetContext sets context data
func (m *MockContextRepository) SetContext(ctx context.Context, convContext *conversation.ConversationContext) error {
	args := m.Called(ctx, convContext)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		if m.contexts[convContext.ConversationID] == nil {
			m.contexts[convContext.ConversationID] = make(map[string]*conversation.ConversationContext)
		}
		m.contexts[convContext.ConversationID][convContext.ContextType] = convContext
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// GetContext retrieves context data
func (m *MockContextRepository) GetContext(ctx context.Context, conversationID, contextType string) (*conversation.ConversationContext, error) {
	args := m.Called(ctx, conversationID, contextType)
	
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if convContexts, exists := m.contexts[conversationID]; exists {
		if context, exists := convContexts[contextType]; exists {
			return context, nil
		}
	}
	
	return args.Get(0).(*conversation.ConversationContext), args.Error(1)
}

// GetAllContext retrieves all context data for a conversation
func (m *MockContextRepository) GetAllContext(ctx context.Context, conversationID string) ([]*conversation.ConversationContext, error) {
	args := m.Called(ctx, conversationID)
	return args.Get(0).([]*conversation.ConversationContext), args.Error(1)
}

// DeleteContext deletes context data
func (m *MockContextRepository) DeleteContext(ctx context.Context, conversationID, contextType string) error {
	args := m.Called(ctx, conversationID, contextType)
	
	if args.Error(0) == nil {
		m.mu.Lock()
		if convContexts, exists := m.contexts[conversationID]; exists {
			delete(convContexts, contextType)
		}
		m.mu.Unlock()
	}
	
	return args.Error(0)
}

// SetupStandardMockBehavior sets up standard mock behaviors
func (m *MockContextRepository) SetupStandardMockBehavior() {
	// SetContext succeeds by default
	m.On("SetContext", mock.Anything, mock.AnythingOfType("*conversation.ConversationContext")).
		Return(nil).Maybe()
	
	// GetContext returns not found by default
	m.On("GetContext", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
		Return((*conversation.ConversationContext)(nil), fmt.Errorf("context not found")).Maybe()
	
	// GetAllContext returns empty list by default
	m.On("GetAllContext", mock.Anything, mock.AnythingOfType("string")).
		Return([]*conversation.ConversationContext{}, nil).Maybe()
	
	// DeleteContext succeeds by default
	m.On("DeleteContext", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).
		Return(nil).Maybe()
}

// MockConversationAIService provides a mock implementation of conversation AI service
type MockConversationAIService struct {
	mock.Mock
}

// NewMockConversationAIService creates a new mock conversation AI service
func NewMockConversationAIService() *MockConversationAIService {
	return &MockConversationAIService{}
}

// GenerateConversationalResponse generates a conversational response
func (m *MockConversationAIService) GenerateConversationalResponse(ctx context.Context, conv *conversation.Conversation, messages []*conversation.Message, userMessage string) (*conversation.ConversationalResponse, error) {
	args := m.Called(ctx, conv, messages, userMessage)
	return args.Get(0).(*conversation.ConversationalResponse), args.Error(1)
}

// ExtractRecipeFromConversation extracts recipe information from conversation
func (m *MockConversationAIService) ExtractRecipeFromConversation(ctx context.Context, messages []*conversation.Message) (*conversation.RecipeCreationContext, error) {
	args := m.Called(ctx, messages)
	return args.Get(0).(*conversation.RecipeCreationContext), args.Error(1)
}

// SetupStandardMockBehavior sets up standard mock behaviors
func (m *MockConversationAIService) SetupStandardMockBehavior() {
	// Standard conversational response
	standardResponse := &conversation.ConversationalResponse{
		Content:    "I'd be happy to help you with that! Can you tell me more details?",
		Intent:     conversation.IntentGeneralQuestion,
		Confidence: 0.8,
		Metadata: map[string]interface{}{
			"provider": "mock",
			"model":    "test-model",
		},
	}
	
	m.On("GenerateConversationalResponse", mock.Anything, mock.AnythingOfType("*conversation.Conversation"), mock.AnythingOfType("[]*conversation.Message"), mock.AnythingOfType("string")).
		Return(standardResponse, nil).Maybe()
	
	// Standard recipe extraction context
	standardRecipeContext := &conversation.RecipeCreationContext{
		CurrentStep: "gathering_info",
		Ingredients: []string{},
		Instructions: []string{},
		DietaryReqs: []string{},
		MissingInfo: []string{"dish_name", "ingredients", "serving_size"},
	}
	
	m.On("ExtractRecipeFromConversation", mock.Anything, mock.AnythingOfType("[]*conversation.Message")).
		Return(standardRecipeContext, nil).Maybe()
}

// MockOllamaClient provides a mock implementation of OllamaClient
type MockOllamaClient struct {
	mock.Mock
}

// NewMockOllamaClient creates a new mock Ollama client
func NewMockOllamaClient() *MockOllamaClient {
	return &MockOllamaClient{}
}

// GenerateChatCompletion generates a chat completion
func (m *MockOllamaClient) GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage) (string, error) {
	args := m.Called(ctx, messages)
	return args.String(0), args.Error(1)
}

// HealthCheck performs a health check
func (m *MockOllamaClient) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// SetupStandardMockBehavior sets up standard mock behaviors
func (m *MockOllamaClient) SetupStandardMockBehavior() {
	// Health check succeeds by default
	m.On("HealthCheck", mock.Anything).
		Return(nil).Maybe()
	
	// Generate chat completion returns helpful response
	m.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
		Return("I'm here to help you with cooking! What would you like to know?", nil).Maybe()
}

// MockOpenAIClient provides a mock implementation of OpenAIClient
type MockOpenAIClient struct {
	mock.Mock
}

// NewMockOpenAIClient creates a new mock OpenAI client
func NewMockOpenAIClient() *MockOpenAIClient {
	return &MockOpenAIClient{}
}

// GenerateChatCompletion generates a chat completion
func (m *MockOpenAIClient) GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage) (string, error) {
	args := m.Called(ctx, messages)
	return args.String(0), args.Error(1)
}

// SetupStandardMockBehavior sets up standard mock behaviors
func (m *MockOpenAIClient) SetupStandardMockBehavior() {
	// Generate chat completion returns helpful response
	m.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
		Return("I'm your AI cooking assistant! How can I help you today?", nil).Maybe()
}

// ConversationMockContainer provides a container with all conversation-related mocks
type ConversationMockContainer struct {
	ConversationRepo *MockConversationRepository
	MessageRepo      *MockMessageRepository
	ContextRepo      *MockContextRepository
	AIService        *MockConversationAIService
	OllamaClient     *MockOllamaClient
	OpenAIClient     *MockOpenAIClient
}

// NewConversationMockContainer creates a new conversation mock container
func NewConversationMockContainer() *ConversationMockContainer {
	container := &ConversationMockContainer{
		ConversationRepo: NewMockConversationRepository(),
		MessageRepo:      NewMockMessageRepository(),
		ContextRepo:      NewMockContextRepository(),
		AIService:        NewMockConversationAIService(),
		OllamaClient:     NewMockOllamaClient(),
		OpenAIClient:     NewMockOpenAIClient(),
	}

	// Setup standard behaviors
	container.ConversationRepo.SetupStandardMockBehavior()
	container.MessageRepo.SetupStandardMockBehavior()
	container.ContextRepo.SetupStandardMockBehavior()
	container.AIService.SetupStandardMockBehavior()
	container.OllamaClient.SetupStandardMockBehavior()
	container.OpenAIClient.SetupStandardMockBehavior()

	return container
}

// AssertExpectations asserts that all mocks met their expectations
func (c *ConversationMockContainer) AssertExpectations(t mock.TestingT) {
	c.ConversationRepo.AssertExpectations(t)
	c.MessageRepo.AssertExpectations(t)
	c.ContextRepo.AssertExpectations(t)
	c.AIService.AssertExpectations(t)
	c.OllamaClient.AssertExpectations(t)
	c.OpenAIClient.AssertExpectations(t)
}

// ResetMocks resets all mocks to their initial state
func (c *ConversationMockContainer) ResetMocks() {
	c.ConversationRepo.Mock = mock.Mock{}
	c.MessageRepo.Mock = mock.Mock{}
	c.ContextRepo.Mock = mock.Mock{}
	c.AIService.Mock = mock.Mock{}
	c.OllamaClient.Mock = mock.Mock{}
	c.OpenAIClient.Mock = mock.Mock{}

	// Re-setup standard behaviors
	c.ConversationRepo.SetupStandardMockBehavior()
	c.MessageRepo.SetupStandardMockBehavior()
	c.ContextRepo.SetupStandardMockBehavior()
	c.AIService.SetupStandardMockBehavior()
	c.OllamaClient.SetupStandardMockBehavior()
	c.OpenAIClient.SetupStandardMockBehavior()
}