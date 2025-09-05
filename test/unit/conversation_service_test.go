package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ConversationServiceTestSuite tests the conversation service
type ConversationServiceTestSuite struct {
	suite.Suite
	testSuite *testutils.ConversationTestSuite
	ctx       context.Context
}

// SetupSuite initializes the test suite
func (suite *ConversationServiceTestSuite) SetupSuite() {
	suite.testSuite = testutils.NewConversationTestSuite()
	suite.ctx = context.Background()
}

// TearDownTest cleans up after each test
func (suite *ConversationServiceTestSuite) TearDownTest() {
	// Reset mocks after each test - don't call SetupStandardMocks to avoid conflicts
	suite.testSuite.MockConversationRepo.Mock = mock.Mock{}
	suite.testSuite.MockMessageRepo.Mock = mock.Mock{}
	suite.testSuite.MockContextRepo.Mock = mock.Mock{}
	suite.testSuite.MockAIService.Mock = mock.Mock{}
	suite.testSuite.MockOllamaClient.Mock = mock.Mock{}
	suite.testSuite.MockOpenAIClient.Mock = mock.Mock{}
}

// TestCreateConversation tests conversation creation
func (suite *ConversationServiceTestSuite) TestCreateConversation() {
	userID := suite.testSuite.GetTestUserID("testuser")

	testCases := []struct {
		name           string
		firstMessage   string
		expectedIntent conversation.ConversationIntent
		expectedTitle  string
		shouldError    bool
	}{
		{
			name:           "Recipe Creation Request",
			firstMessage:   "I want to create a recipe for pasta",
			expectedIntent: conversation.IntentRecipeCreation,
			expectedTitle:  "Recipe: Pasta",
			shouldError:    false,
		},
		{
			name:           "Cooking Help Request",
			firstMessage:   "How do I cook rice properly?",
			expectedIntent: conversation.IntentCookingHelp,
			expectedTitle:  "Cooking Help",
			shouldError:    false,
		},
		{
			name:           "Ingredient Substitution",
			firstMessage:   "I don't have butter, what can I substitute?",
			expectedIntent: conversation.IntentIngredientSubst,
			expectedTitle:  "Ingredient Substitution",
			shouldError:    false,
		},
		{
			name:           "Meal Planning",
			firstMessage:   "Help me plan meals for this week",
			expectedIntent: conversation.IntentMealPlanning,
			expectedTitle:  "Meal Planning",
			shouldError:    false,
		},
		{
			name:           "General Question",
			firstMessage:   "Hello, what can you do?",
			expectedIntent: conversation.IntentGeneralQuestion,
			expectedTitle:  "Chat with AI Chef",
			shouldError:    false,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture loop variable
		suite.Run(tc.name, func() {
			// Reset mocks for clean state
			suite.testSuite.MockConversationRepo.Mock = mock.Mock{}

			// Setup mock to capture created conversation
			suite.testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.MatchedBy(func(conv *conversation.Conversation) bool {
				suite.Equal(userID, conv.UserID)
				suite.Equal(tc.expectedIntent, conv.Intent)
				suite.Equal(conversation.StatusActive, conv.Status)
				suite.Contains(conv.Title, tc.expectedTitle)
				suite.NotEmpty(conv.ID)
				suite.NotNil(conv.Metadata)
				return true
			})).Return(nil).Once()

			// Create conversation
			conv, err := suite.testSuite.ConversationService.CreateConversation(suite.ctx, userID, tc.firstMessage)

			if tc.shouldError {
				suite.Error(err)
				suite.Nil(conv)
			} else {
				suite.NoError(err)
				suite.NotNil(conv)
				suite.Equal(userID, conv.UserID)
				suite.Equal(tc.expectedIntent, conv.Intent)
				suite.Equal(conversation.StatusActive, conv.Status)
			}

			suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
		})
	}
}

// TestIntentClassification tests intent classification accuracy
func (suite *ConversationServiceTestSuite) TestIntentClassification() {
	testCases := testutils.IntentClassificationTestCases()

	// Create intent classifier
	classifier := conversation.NewIntentClassifier()

	for _, tc := range testCases {
		tc := tc // Capture loop variable
		suite.Run(tc.Message, func() {
			intent := classifier.ClassifyIntent(tc.Message)
			suite.Equal(tc.ExpectedIntent, intent, "Failed to classify: %s", tc.Message)
		})
	}
}

// TestAddMessage tests adding messages to conversations
func (suite *ConversationServiceTestSuite) TestAddMessage() {
	conversationID := uuid.New().String()

	testCases := []struct {
		name        string
		role        conversation.MessageRole
		content     string
		metadata    map[string]interface{}
		shouldError bool
	}{
		{
			name:        "User Message",
			role:        conversation.RoleUser,
			content:     "I want to make pasta",
			metadata:    nil,
			shouldError: false,
		},
		{
			name:        "Assistant Message",
			role:        conversation.RoleAssistant,
			content:     "I'd be happy to help you make pasta!",
			metadata:    map[string]interface{}{"ai_provider": "ollama"},
			shouldError: false,
		},
		{
			name:        "System Message",
			role:        conversation.RoleSystem,
			content:     "Conversation started",
			metadata:    nil,
			shouldError: false,
		},
		{
			name:        "Empty Content",
			role:        conversation.RoleUser,
			content:     "",
			metadata:    nil,
			shouldError: false, // Service should handle empty content gracefully
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture loop variable
		suite.Run(tc.name, func() {
			// Create a fresh mock for this specific test
			freshMockRepo := testutils.NewMockMessageRepository()

			// Replace the service's message repo with our fresh mock
			originalRepo := suite.testSuite.MockMessageRepo
			suite.testSuite.MockMessageRepo = freshMockRepo
			suite.testSuite.ConversationService = conversation.NewService(
				suite.testSuite.MockConversationRepo,
				freshMockRepo,
				suite.testSuite.MockContextRepo,
				suite.testSuite.AIService,
			)

			// Setup mock to capture created message
			freshMockRepo.On("CreateMessage", mock.Anything, mock.MatchedBy(func(msg *conversation.Message) bool {
				return msg.ConversationID == conversationID &&
					msg.Role == tc.role &&
					msg.Content == tc.content &&
					msg.ID != ""
			})).Return(nil).Once()

			// Add message
			msg, err := suite.testSuite.ConversationService.AddMessage(suite.ctx, conversationID, tc.role, tc.content, tc.metadata)

			if tc.shouldError {
				suite.Error(err)
				suite.Nil(msg)
			} else {
				suite.NoError(err)
				suite.NotNil(msg)
				suite.Equal(conversationID, msg.ConversationID)
				suite.Equal(tc.role, msg.Role)
				suite.Equal(tc.content, msg.Content)
			}

			freshMockRepo.AssertExpectations(suite.T())

			// Restore original mock and service
			suite.testSuite.MockMessageRepo = originalRepo
			suite.testSuite.ConversationService = conversation.NewService(
				suite.testSuite.MockConversationRepo,
				originalRepo,
				suite.testSuite.MockContextRepo,
				suite.testSuite.AIService,
			)
		})
	}
}

// TestGetConversationWithMessages tests retrieving conversations with messages
func (suite *ConversationServiceTestSuite) TestGetConversationWithMessages() {
	userID := suite.testSuite.GetTestUserID("testuser")
	conv, messages := suite.testSuite.CreateConversationWithMessages(
		userID,
		conversation.IntentRecipeCreation,
		[]string{"I want to make pasta", "What type of pasta?", "Spaghetti carbonara"},
	)

	// Test successful retrieval
	suite.Run("Successful Retrieval", func() {
		// Reset mocks for clean state
		suite.testSuite.MockConversationRepo.Mock = mock.Mock{}
		suite.testSuite.MockMessageRepo.Mock = mock.Mock{}

		// Setup mocks
		suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, conv.ID).Return(conv, nil).Once()
		suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, conv.ID, 100, 0).Return(messages, nil).Once()

		// Get conversation with messages
		retrievedConv, retrievedMessages, err := suite.testSuite.ConversationService.GetConversationWithMessages(suite.ctx, conv.ID)

		suite.NoError(err)
		suite.NotNil(retrievedConv)
		suite.NotNil(retrievedMessages)
		suite.Equal(conv.ID, retrievedConv.ID)
		suite.Len(retrievedMessages, len(messages))

		suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
		suite.testSuite.MockMessageRepo.AssertExpectations(suite.T())
	})

	// Test conversation not found
	suite.Run("Conversation Not Found", func() {
		// Reset mocks for clean state
		suite.testSuite.MockConversationRepo.Mock = mock.Mock{}
		suite.testSuite.MockMessageRepo.Mock = mock.Mock{}

		nonExistentID := uuid.New().String()
		suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, nonExistentID).
			Return((*conversation.Conversation)(nil), assert.AnError).Once()

		retrievedConv, retrievedMessages, err := suite.testSuite.ConversationService.GetConversationWithMessages(suite.ctx, nonExistentID)

		suite.Error(err)
		suite.Nil(retrievedConv)
		suite.Nil(retrievedMessages)

		suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
	})
}

// TestProcessMessage tests message processing workflow - SKIPPED due to mock isolation issues in CI
func (suite *ConversationServiceTestSuite) TestProcessMessage() {
	userID := suite.testSuite.GetTestUserID("testuser")

	testCases := []struct {
		name         string
		userMessage  string
		setupMocks   func() *conversation.Conversation
		expectedResp string
		shouldError  bool
	}{
		{
			name:        "Recipe Creation Flow",
			userMessage: "I want to make carbonara",
			setupMocks: func() *conversation.Conversation {
				// Create conversation directly without relying on helper mock setup
				conv := &conversation.Conversation{
					ID:        uuid.New().String(),
					UserID:    userID,
					Title:     "Test Recipe Creation Conversation",
					Intent:    conversation.IntentRecipeCreation,
					Status:    conversation.StatusActive,
					Metadata:  make(map[string]interface{}),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				// Mock getting conversation (called by ProcessMessage after AddMessage)
				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, conv.ID).Return(conv, nil).Times(1)

				// Mock adding user message with more flexible matching
				suite.testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*conversation.Message")).Return(nil).Times(1)

				// Mock getting conversation messages for AI context
				suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, conv.ID, 20, 0).Return([]*conversation.Message{}, nil).Once()

				// Mock AI client health checks and responses (since the real AIService is being used)
				suite.testSuite.MockOllamaClient.On("HealthCheck", mock.Anything).Return(nil).Once()
				suite.testSuite.MockOllamaClient.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
					Return("Great! I'll help you make carbonara. What ingredients do you have?", nil).Once()

				// Mock setting AI metadata context
				suite.testSuite.MockContextRepo.On("SetContext", mock.Anything, mock.MatchedBy(func(ctx *conversation.ConversationContext) bool {
					return ctx.ConversationID == conv.ID && ctx.ContextType == "ai_metadata"
				})).Return(nil).Once()

				return conv
			},
			expectedResp: "Great! I'll help you make carbonara. What ingredients do you have?",
			shouldError:  false,
		},
		{
			name:        "AI Service Failure - Fallback",
			userMessage: "How do I cook rice?",
			setupMocks: func() *conversation.Conversation {
				// Create conversation with cooking help intent for this test directly
				conv := &conversation.Conversation{
					ID:        uuid.New().String(),
					UserID:    userID,
					Title:     "Test Cooking Help Conversation",
					Intent:    conversation.IntentCookingHelp,
					Status:    conversation.StatusActive,
					Metadata:  make(map[string]interface{}),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				// Mock getting conversation (called by ProcessMessage after AddMessage)
				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, conv.ID).Return(conv, nil).Times(1)

				// Mock adding user message with more flexible matching
				suite.testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*conversation.Message")).Return(nil).Times(1)

				// Mock getting conversation messages
				suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, conv.ID, 20, 0).Return([]*conversation.Message{}, nil).Once()

				// Mock AI service failure - both Ollama and OpenAI fail so it falls back to simple response
				suite.testSuite.MockOllamaClient.On("HealthCheck", mock.Anything).Return(assert.AnError).Once()
				suite.testSuite.MockOpenAIClient.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
					Return("", assert.AnError).Once()

				// Mock setting fallback metadata context
				suite.testSuite.MockContextRepo.On("SetContext", mock.Anything, mock.MatchedBy(func(ctx *conversation.ConversationContext) bool {
					return ctx.ConversationID == conv.ID && ctx.ContextType == "ai_metadata"
				})).Return(nil).Once()

				return conv
			},
			expectedResp: "I'm here to help with your cooking question!",
			shouldError:  false,
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture loop variable
		suite.Run(tc.name, func() {
			// Reset mocks for clean state
			suite.testSuite.MockConversationRepo.Mock = mock.Mock{}
			suite.testSuite.MockMessageRepo.Mock = mock.Mock{}
			suite.testSuite.MockContextRepo.Mock = mock.Mock{}
			suite.testSuite.MockOllamaClient.Mock = mock.Mock{}
			suite.testSuite.MockOpenAIClient.Mock = mock.Mock{}

			conv := tc.setupMocks()

			userMsg, response, err := suite.testSuite.ConversationService.ProcessMessage(suite.ctx, conv.ID, tc.userMessage, userID)

			if tc.shouldError {
				suite.Error(err)
			} else {
				suite.NoError(err)
				suite.NotNil(userMsg)
				suite.Equal(tc.userMessage, userMsg.Content)
				suite.Contains(response, tc.expectedResp)
			}

			suite.testSuite.AssertExpectations(suite.T())
		})
	}
}

// TestConversationContextManagement tests context management
func (suite *ConversationServiceTestSuite) TestConversationContextManagement() {
	conversationID := uuid.New().String()

	// Test setting context
	suite.Run("Set Context", func() {
		contextData := map[string]interface{}{
			"recipe_progress": "gathering_ingredients",
			"ingredients":     []string{"pasta", "eggs", "cheese"},
		}

		suite.testSuite.MockContextRepo.On("SetContext", mock.Anything, mock.MatchedBy(func(ctx *conversation.ConversationContext) bool {
			return ctx.ConversationID == conversationID && ctx.ContextType == "recipe_creation"
		})).Return(nil).Once()

		err := suite.testSuite.ConversationService.SetContext(suite.ctx, conversationID, "recipe_creation", contextData)

		suite.NoError(err)
		suite.testSuite.MockContextRepo.AssertExpectations(suite.T())
	})

	// Test getting context
	suite.Run("Get Context", func() {
		expectedData := map[string]interface{}{
			"recipe_progress": "gathering_ingredients",
			"ingredients":     []string{"pasta", "eggs", "cheese"},
		}

		contextObj := &conversation.ConversationContext{
			ConversationID: conversationID,
			ContextType:    "recipe_creation",
			ContextData:    expectedData,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		suite.testSuite.MockContextRepo.On("GetContext", mock.Anything, conversationID, "recipe_creation").
			Return(contextObj, nil).Once()

		data, err := suite.testSuite.ConversationService.GetContext(suite.ctx, conversationID, "recipe_creation")

		suite.NoError(err)
		suite.Equal(expectedData, data)
		suite.testSuite.MockContextRepo.AssertExpectations(suite.T())
	})
}

// TestConversationLifecycle tests conversation lifecycle operations
func (suite *ConversationServiceTestSuite) TestConversationLifecycle() {
	userID := suite.testSuite.GetTestUserID("testuser")
	conv := suite.testSuite.CreateTestConversation(userID, conversation.IntentRecipeCreation)

	// Test archiving conversation
	suite.Run("Archive Conversation", func() {
		// Reset mocks for clean state
		suite.testSuite.MockConversationRepo.Mock = mock.Mock{}

		suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, conv.ID).Return(conv, nil).Once()
		suite.testSuite.MockConversationRepo.On("UpdateConversation", mock.Anything, mock.MatchedBy(func(c *conversation.Conversation) bool {
			return c.ID == conv.ID && c.Status == conversation.StatusArchived
		})).Return(nil).Once()

		err := suite.testSuite.ConversationService.ArchiveConversation(suite.ctx, conv.ID)

		suite.NoError(err)
		suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
	})

	// Test deleting conversation
	suite.Run("Delete Conversation", func() {
		// Reset mocks for clean state
		suite.testSuite.MockConversationRepo.Mock = mock.Mock{}

		suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, conv.ID).Return(conv, nil).Once()
		suite.testSuite.MockConversationRepo.On("UpdateConversation", mock.Anything, mock.MatchedBy(func(c *conversation.Conversation) bool {
			return c.ID == conv.ID && c.Status == conversation.StatusDeleted
		})).Return(nil).Once()

		err := suite.testSuite.ConversationService.DeleteConversation(suite.ctx, conv.ID)

		suite.NoError(err)
		suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
	})
}

// TestGetUserConversations tests retrieving user conversations
func (suite *ConversationServiceTestSuite) TestGetUserConversations() {
	userID := suite.testSuite.GetTestUserID("testuser")

	conversations := []*conversation.Conversation{
		suite.testSuite.CreateTestConversation(userID, conversation.IntentRecipeCreation),
		suite.testSuite.CreateTestConversation(userID, conversation.IntentCookingHelp),
		suite.testSuite.CreateTestConversation(userID, conversation.IntentIngredientSubst),
	}

	suite.testSuite.MockConversationRepo.On("GetUserConversations", mock.Anything, userID, 10, 0).
		Return(conversations, nil).Once()

	retrievedConversations, err := suite.testSuite.ConversationService.GetUserConversations(suite.ctx, userID, 10, 0)

	suite.NoError(err)
	suite.Len(retrievedConversations, 3)
	suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
}

// TestGetConversationStats tests conversation statistics
func (suite *ConversationServiceTestSuite) TestGetConversationStats() {
	userID := suite.testSuite.GetTestUserID("testuser")

	conversations := []*conversation.Conversation{
		{
			ID:     uuid.New().String(),
			UserID: userID,
			Intent: conversation.IntentRecipeCreation,
			Status: conversation.StatusActive,
		},
		{
			ID:     uuid.New().String(),
			UserID: userID,
			Intent: conversation.IntentRecipeCreation,
			Status: conversation.StatusArchived,
		},
		{
			ID:     uuid.New().String(),
			UserID: userID,
			Intent: conversation.IntentCookingHelp,
			Status: conversation.StatusActive,
		},
	}

	suite.testSuite.MockConversationRepo.On("GetUserConversations", mock.Anything, userID, 1000, 0).
		Return(conversations, nil).Once()

	stats, err := suite.testSuite.ConversationService.GetConversationStats(suite.ctx, userID)

	suite.NoError(err)
	suite.NotNil(stats)

	// Verify stats structure
	suite.Equal(3, stats["total_conversations"])
	suite.Equal(2, stats["active_conversations"])
	suite.Equal(1, stats["archived_conversations"])

	intents := stats["intents"].(map[conversation.ConversationIntent]int)
	suite.Equal(2, intents[conversation.IntentRecipeCreation])
	suite.Equal(1, intents[conversation.IntentCookingHelp])

	suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
}

// TestConversationTitleGeneration tests automatic title generation
func (suite *ConversationServiceTestSuite) TestConversationTitleGeneration() {
	testCases := []struct {
		name          string
		intent        conversation.ConversationIntent
		message       string
		expectedTitle string
	}{
		{
			name:          "Recipe with dish name",
			intent:        conversation.IntentRecipeCreation,
			message:       "I want to make a recipe for chocolate cake",
			expectedTitle: "Recipe: Chocolate Cake",
		},
		{
			name:          "Recipe without specific dish",
			intent:        conversation.IntentRecipeCreation,
			message:       "I want to make something delicious", // This will be recipe creation but no specific dish
			expectedTitle: "Recipe: Something Delicious",
		},
		{
			name:          "Cooking help",
			intent:        conversation.IntentCookingHelp,
			message:       "How do I grill chicken?",
			expectedTitle: "Cooking Help",
		},
		{
			name:          "Ingredient substitution",
			intent:        conversation.IntentIngredientSubst,
			message:       "What can I use instead of butter?",
			expectedTitle: "Ingredient Substitution",
		},
		{
			name:          "Meal planning",
			intent:        conversation.IntentMealPlanning,
			message:       "Help me plan meals for the week",
			expectedTitle: "Meal Planning",
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture loop variable
		suite.Run(tc.name, func() {
			// Reset mocks for clean state
			suite.testSuite.MockConversationRepo.Mock = mock.Mock{}

			userID := suite.testSuite.GetTestUserID("testuser")

			suite.testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.MatchedBy(func(conv *conversation.Conversation) bool {
				suite.Contains(conv.Title, tc.expectedTitle)
				return conv.Intent == tc.intent
			})).Return(nil).Once()

			conv, err := suite.testSuite.ConversationService.CreateConversation(suite.ctx, userID, tc.message)

			suite.NoError(err)
			suite.NotNil(conv)
			suite.Contains(conv.Title, tc.expectedTitle)

			suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
		})
	}
}

// TestErrorHandling tests error handling scenarios
func (suite *ConversationServiceTestSuite) TestErrorHandling() {
	userID := suite.testSuite.GetTestUserID("testuser")

	// Test repository error during conversation creation
	suite.Run("Repository Error During Creation", func() {
		// Reset mocks for clean state
		suite.testSuite.MockConversationRepo.Mock = mock.Mock{}

		suite.testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.AnythingOfType("*conversation.Conversation")).
			Return(assert.AnError).Once()

		conv, err := suite.testSuite.ConversationService.CreateConversation(suite.ctx, userID, "test message")

		suite.Error(err)
		suite.Nil(conv)
		suite.testSuite.MockConversationRepo.AssertExpectations(suite.T())
	})

	// Test repository error during message creation
	suite.Run("Repository Error During Message Creation", func() {
		// Reset mocks for clean state
		suite.testSuite.MockMessageRepo.Mock = mock.Mock{}

		conversationID := uuid.New().String()

		suite.testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*conversation.Message")).
			Return(assert.AnError).Once()

		msg, err := suite.testSuite.ConversationService.AddMessage(suite.ctx, conversationID, conversation.RoleUser, "test", nil)

		suite.Error(err)
		suite.Nil(msg)
		suite.testSuite.MockMessageRepo.AssertExpectations(suite.T())
	})
}

// Run the test suite
func TestConversationServiceSuite(t *testing.T) {
	suite.Run(t, new(ConversationServiceTestSuite))
}

// TestIntentClassifierEdgeCases tests edge cases for intent classification
func TestIntentClassifierEdgeCases(t *testing.T) {
	classifier := conversation.NewIntentClassifier()

	edgeCases := []struct {
		name           string
		message        string
		expectedIntent conversation.ConversationIntent
	}{
		{
			name:           "Empty message",
			message:        "",
			expectedIntent: conversation.IntentGeneralQuestion,
		},
		{
			name:           "Whitespace only",
			message:        "   \t\n  ",
			expectedIntent: conversation.IntentGeneralQuestion,
		},
		{
			name:           "Mixed case recipe request",
			message:        "CrEaTe A ReciPe FoR ChIcKeN",
			expectedIntent: conversation.IntentRecipeCreation,
		},
		{
			name:           "Very long message with recipe intent",
			message:        "I would really like to create a delicious and nutritious recipe for my family dinner tonight, something with chicken would be great",
			expectedIntent: conversation.IntentRecipeCreation,
		},
		{
			name:           "Multiple intents in one message",
			message:        "I want to make a recipe and also need help with cooking techniques",
			expectedIntent: conversation.IntentRecipeCreation, // Recipe creation takes priority
		},
		{
			name:           "Non-English characters",
			message:        "Hola, ¿puedes crear una receta?",
			expectedIntent: conversation.IntentGeneralQuestion, // Falls back to general
		},
	}

	for _, tc := range edgeCases {
		tc := tc // Capture loop variable
		t.Run(tc.name, func(t *testing.T) {
			intent := classifier.ClassifyIntent(tc.message)
			assert.Equal(t, tc.expectedIntent, intent)
		})
	}
}

// TestConcurrentConversationOperations tests concurrent access to conversation service
func TestConcurrentConversationOperations(t *testing.T) {
	testSuite := testutils.NewConversationTestSuite()
	ctx := context.Background()
	userID := testSuite.GetTestUserID("testuser")

	// Setup mocks for concurrent operations
	testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.AnythingOfType("*conversation.Conversation")).
		Return(nil).Maybe()
	testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.AnythingOfType("*conversation.Message")).
		Return(nil).Maybe()

	const numGoroutines = 10
	const operationsPerGoroutine = 5

	results := make(chan error, numGoroutines*operationsPerGoroutine)

	// Launch concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			for j := 0; j < operationsPerGoroutine; j++ {
				// Create conversation
				conv, err := testSuite.ConversationService.CreateConversation(
					ctx,
					userID,
					fmt.Sprintf("Test message %d-%d", routineID, j),
				)
				if err != nil {
					results <- err
					continue
				}

				// Add message to conversation
				_, err = testSuite.ConversationService.AddMessage(
					ctx,
					conv.ID,
					conversation.RoleUser,
					fmt.Sprintf("Additional message %d-%d", routineID, j),
					nil,
				)
				results <- err
			}
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines*operationsPerGoroutine; i++ {
		err := <-results
		require.NoError(t, err, "Concurrent operation failed")
	}
}
