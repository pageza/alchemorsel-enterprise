// Package testutils provides conversation-specific testing utilities
package testutils

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/google/uuid"
)

// ConversationTestSuite provides comprehensive testing infrastructure for conversations
type ConversationTestSuite struct {
	MockConversationRepo *MockConversationRepository
	MockMessageRepo      *MockMessageRepository
	MockContextRepo      *MockContextRepository
	MockAIService        *MockConversationAIService
	MockOllamaClient     *MockOllamaClient
	MockOpenAIClient     *MockOpenAIClient
	ConversationService  *conversation.Service
	AIService            *conversation.AIService
	TestUsers            map[string]string // username -> userID
}

// NewConversationTestSuite creates a new conversation test suite
func NewConversationTestSuite() *ConversationTestSuite {
	suite := &ConversationTestSuite{
		MockConversationRepo: NewMockConversationRepository(),
		MockMessageRepo:      NewMockMessageRepository(),
		MockContextRepo:      NewMockContextRepository(),
		MockAIService:        NewMockConversationAIService(),
		MockOllamaClient:     NewMockOllamaClient(),
		MockOpenAIClient:     NewMockOpenAIClient(),
		TestUsers: map[string]string{
			"testuser":  "550e8400-e29b-41d4-a716-446655440001",
			"chefuser":  "550e8400-e29b-41d4-a716-446655440002",
			"adminuser": "550e8400-e29b-41d4-a716-446655440003",
		},
	}

	// Setup standard mock behaviors
	suite.SetupStandardMocks()

	// Create AI service with mock clients
	suite.AIService = conversation.NewAIService(suite.MockOllamaClient, suite.MockOpenAIClient)

	// Create conversation service
	suite.ConversationService = conversation.NewService(
		suite.MockConversationRepo,
		suite.MockMessageRepo,
		suite.MockContextRepo,
		suite.AIService,
	)

	return suite
}

// SetupStandardMocks configures default mock behaviors
func (s *ConversationTestSuite) SetupStandardMocks() {
	s.MockConversationRepo.SetupStandardMockBehavior()
	s.MockMessageRepo.SetupStandardMockBehavior()
	s.MockContextRepo.SetupStandardMockBehavior()
	s.MockAIService.SetupStandardMockBehavior()
	s.MockOllamaClient.SetupStandardMockBehavior()
	s.MockOpenAIClient.SetupStandardMockBehavior()
}

// CreateTestConversation creates a test conversation with default values
func (s *ConversationTestSuite) CreateTestConversation(userID string, intent conversation.ConversationIntent) *conversation.Conversation {
	conv := &conversation.Conversation{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     fmt.Sprintf("Test %s Conversation", intent),
		Intent:    intent,
		Status:    conversation.StatusActive,
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Don't set up mock expectations here - let individual tests control their mocks
	// This prevents conflicts with test-specific expectations

	return conv
}

// CreateTestMessage creates a test message
func (s *ConversationTestSuite) CreateTestMessage(conversationID string, role conversation.MessageRole, content string) *conversation.Message {
	msg := &conversation.Message{
		ID:             uuid.New().String(),
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		Metadata:       make(map[string]interface{}),
		CreatedAt:      time.Now(),
	}

	// Don't set up mock expectations here - let individual tests control their mocks
	// This prevents conflicts with test-specific expectations

	return msg
}

// CreateConversationWithMessages creates a conversation with a series of messages
func (s *ConversationTestSuite) CreateConversationWithMessages(userID string, intent conversation.ConversationIntent, messageContents []string) (*conversation.Conversation, []*conversation.Message) {
	conv := s.CreateTestConversation(userID, intent)
	var messages []*conversation.Message

	for i, content := range messageContents {
		role := conversation.RoleUser
		if i%2 == 1 { // Alternate between user and assistant
			role = conversation.RoleAssistant
		}
		msg := s.CreateTestMessage(conv.ID, role, content)
		messages = append(messages, msg)
	}

	// Don't set up mock expectations here - let individual tests control their mocks
	// This prevents conflicts with test-specific expectations

	return conv, messages
}

// GetTestUserID returns a test user ID by username
func (s *ConversationTestSuite) GetTestUserID(username string) string {
	if userID, exists := s.TestUsers[username]; exists {
		return userID
	}
	return s.TestUsers["testuser"] // Default fallback
}

// AssertExpectations verifies all mock expectations
func (s *ConversationTestSuite) AssertExpectations(t *testing.T) {
	s.MockConversationRepo.AssertExpectations(t)
	s.MockMessageRepo.AssertExpectations(t)
	s.MockContextRepo.AssertExpectations(t)
	s.MockAIService.AssertExpectations(t)
	s.MockOllamaClient.AssertExpectations(t)
	s.MockOpenAIClient.AssertExpectations(t)
}

// ConversationTestScenario represents a test scenario for conversations
type ConversationTestScenario struct {
	Name         string
	UserID       string
	Intent       conversation.ConversationIntent
	Messages     []ScenarioMessage
	ExpectedFlow []string
	Context      map[string]interface{}
}

// ScenarioMessage represents a message in a test scenario
type ScenarioMessage struct {
	Role    conversation.MessageRole
	Content string
	Delay   time.Duration
}

// ConversationScenarios provides predefined test scenarios
func ConversationScenarios() []ConversationTestScenario {
	return []ConversationTestScenario{
		{
			Name:   "Recipe Creation - Pasta",
			UserID: "550e8400-e29b-41d4-a716-446655440001",
			Intent: conversation.IntentRecipeCreation,
			Messages: []ScenarioMessage{
				{Role: conversation.RoleUser, Content: "I want to make pasta"},
				{Role: conversation.RoleAssistant, Content: "What type of pasta would you like to make?"},
				{Role: conversation.RoleUser, Content: "Spaghetti carbonara"},
				{Role: conversation.RoleAssistant, Content: "Great choice! How many servings?"},
				{Role: conversation.RoleUser, Content: "4 servings"},
			},
			ExpectedFlow: []string{"gather_info", "gather_details", "ready_to_create"},
		},
		{
			Name:   "Cooking Help - Temperature",
			UserID: "550e8400-e29b-41d4-a716-446655440001",
			Intent: conversation.IntentCookingHelp,
			Messages: []ScenarioMessage{
				{Role: conversation.RoleUser, Content: "What temperature should I cook chicken at?"},
				{Role: conversation.RoleAssistant, Content: "For safety, chicken should reach 165°F internal temperature..."},
			},
			ExpectedFlow: []string{"providing_help"},
		},
		{
			Name:   "Ingredient Substitution",
			UserID: "550e8400-e29b-41d4-a716-446655440001",
			Intent: conversation.IntentIngredientSubst,
			Messages: []ScenarioMessage{
				{Role: conversation.RoleUser, Content: "I don't have butter, what can I use instead?"},
				{Role: conversation.RoleAssistant, Content: "You can substitute butter with oil (3/4 the amount)..."},
			},
			ExpectedFlow: []string{"providing_substitution"},
		},
	}
}

// IntentClassificationTestCases provides test cases for intent classification
func IntentClassificationTestCases() []struct {
	Message        string
	ExpectedIntent conversation.ConversationIntent
} {
	return []struct {
		Message        string
		ExpectedIntent conversation.ConversationIntent
	}{
		// Recipe Creation
		{"I want to make a recipe for pasta", conversation.IntentRecipeCreation},
		{"Can you help me create a recipe?", conversation.IntentRecipeCreation},
		{"How to make chocolate cake", conversation.IntentRecipeCreation},
		{"Give me a recipe for chicken", conversation.IntentRecipeCreation},
		{"Generate a recipe using tomatoes", conversation.IntentRecipeCreation},

		// Cooking Help
		{"How do I cook rice?", conversation.IntentCookingHelp},
		{"What temperature for baking bread?", conversation.IntentCookingHelp},
		{"Explain braising technique", conversation.IntentCookingHelp},
		{"Tips for making pasta", conversation.IntentCookingHelp},
		{"Help me with cooking time", conversation.IntentCookingHelp},

		// Ingredient Substitution
		{"I don't have eggs, what can I substitute?", conversation.IntentIngredientSubst},
		{"What can I use instead of milk?", conversation.IntentIngredientSubst},
		{"Alternative to butter", conversation.IntentIngredientSubst},
		{"I ran out of flour", conversation.IntentIngredientSubst},
		{"Replace sugar with what?", conversation.IntentIngredientSubst},

		// Meal Planning
		{"Help me plan meals for the week", conversation.IntentMealPlanning},
		{"What should I cook this week?", conversation.IntentMealPlanning},
		{"Meal prep ideas", conversation.IntentMealPlanning},
		{"Weekly menu suggestions", conversation.IntentMealPlanning},

		// General Questions
		{"Hello", conversation.IntentGeneralQuestion},
		{"What can you do?", conversation.IntentGeneralQuestion},
		{"Thanks for the help", conversation.IntentGeneralQuestion},
	}
}

// ConversationMetrics tracks conversation test metrics
type ConversationMetrics struct {
	MessagesSent     int
	MessagesReceived int
	ResponseTimes    []time.Duration
	ErrorCount       int
	ConnectionCount  int
	SuccessfulFlows  int
	FailedFlows      int
	IntentAccuracy   float64
	mu               sync.RWMutex
}

// NewConversationMetrics creates new conversation metrics
func NewConversationMetrics() *ConversationMetrics {
	return &ConversationMetrics{
		ResponseTimes: make([]time.Duration, 0),
	}
}

// RecordMessageSent records a sent message
func (m *ConversationMetrics) RecordMessageSent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesSent++
}

// RecordMessageReceived records a received message with response time
func (m *ConversationMetrics) RecordMessageReceived(responseTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesReceived++
	m.ResponseTimes = append(m.ResponseTimes, responseTime)
}

// RecordError records an error
func (m *ConversationMetrics) RecordError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ErrorCount++
}

// RecordConnection records a connection
func (m *ConversationMetrics) RecordConnection() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConnectionCount++
}

// RecordSuccessfulFlow records a successful conversation flow
func (m *ConversationMetrics) RecordSuccessfulFlow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SuccessfulFlows++
}

// RecordFailedFlow records a failed conversation flow
func (m *ConversationMetrics) RecordFailedFlow() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.FailedFlows++
}

// GetAverageResponseTime calculates average response time
func (m *ConversationMetrics) GetAverageResponseTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.ResponseTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, rt := range m.ResponseTimes {
		total += rt
	}

	return total / time.Duration(len(m.ResponseTimes))
}

// GetSuccessRate calculates conversation success rate
func (m *ConversationMetrics) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := m.SuccessfulFlows + m.FailedFlows
	if total == 0 {
		return 0
	}

	return float64(m.SuccessfulFlows) / float64(total)
}

// GetMetricsReport generates a metrics report
func (m *ConversationMetrics) GetMetricsReport() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"messages_sent":         m.MessagesSent,
		"messages_received":     m.MessagesReceived,
		"error_count":           m.ErrorCount,
		"connection_count":      m.ConnectionCount,
		"successful_flows":      m.SuccessfulFlows,
		"failed_flows":          m.FailedFlows,
		"average_response_time": m.GetAverageResponseTime(),
		"success_rate":          m.GetSuccessRate(),
		"intent_accuracy":       m.IntentAccuracy,
	}
}
