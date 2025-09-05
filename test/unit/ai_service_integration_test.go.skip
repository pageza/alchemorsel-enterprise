package unit

import (
	"context"
	"fmt"
	"strings"
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

// AIServiceIntegrationTestSuite tests AI service integration scenarios
type AIServiceIntegrationTestSuite struct {
	suite.Suite
	aiService  *conversation.AIService
	mockOllama *testutils.MockOllamaClient
	mockOpenAI *testutils.MockOpenAIClient
	ctx        context.Context
}

// SetupSuite initializes the test suite
func (suite *AIServiceIntegrationTestSuite) SetupSuite() {
	suite.mockOllama = testutils.NewMockOllamaClient()
	suite.mockOpenAI = testutils.NewMockOpenAIClient()
	suite.aiService = conversation.NewAIService(suite.mockOllama, suite.mockOpenAI)
	suite.ctx = context.Background()
}

// SetupTest resets mocks before each test
func (suite *AIServiceIntegrationTestSuite) SetupTest() {
	suite.mockOllama.Mock = mock.Mock{}
	suite.mockOpenAI.Mock = mock.Mock{}
	suite.mockOllama.SetupStandardMockBehavior()
	suite.mockOpenAI.SetupStandardMockBehavior()
}

// TestOllamaIntegration tests Ollama service integration
func (suite *AIServiceIntegrationTestSuite) TestOllamaIntegration() {
	testCases := []struct {
		name           string
		intent         conversation.ConversationIntent
		userMessage    string
		expectedInResp string
		setupMocks     func()
	}{
		{
			name:           "Recipe Creation with Ollama",
			intent:         conversation.IntentRecipeCreation,
			userMessage:    "I want to make pasta carbonara",
			expectedInResp: "carbonara",
			setupMocks: func() {
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
				suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(messages []conversation.ChatMessage) bool {
					// Verify system prompt is for recipe creation
					return len(messages) > 0 &&
						strings.Contains(messages[0].Content, "RECIPE CREATION MODE") &&
						messages[len(messages)-1].Content == "I want to make pasta carbonara"
				})).Return("Great! I'll help you make pasta carbonara. Let me gather some information about your preferences.", nil)
			},
		},
		{
			name:           "Cooking Help with Ollama",
			intent:         conversation.IntentCookingHelp,
			userMessage:    "How do I properly cook rice?",
			expectedInResp: "rice",
			setupMocks: func() {
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
				suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(messages []conversation.ChatMessage) bool {
					return len(messages) > 0 &&
						strings.Contains(messages[0].Content, "COOKING HELP MODE") &&
						messages[len(messages)-1].Content == "How do I properly cook rice?"
				})).Return("To cook rice properly, use a 1:2 ratio of rice to water. Bring to boil, then simmer covered for 18 minutes.", nil)
			},
		},
		{
			name:           "Ingredient Substitution with Ollama",
			intent:         conversation.IntentIngredientSubst,
			userMessage:    "I don't have butter, what can I substitute?",
			expectedInResp: "substitute",
			setupMocks: func() {
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
				suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(messages []conversation.ChatMessage) bool {
					return len(messages) > 0 &&
						strings.Contains(messages[0].Content, "INGREDIENT SUBSTITUTION MODE") &&
						messages[len(messages)-1].Content == "I don't have butter, what can I substitute?"
				})).Return("You can substitute butter with olive oil (3/4 the amount), vegetable oil, or margarine in most recipes.", nil)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			// Create test conversation
			conv := &conversation.Conversation{
				ID:     uuid.New().String(),
				UserID: "test-user",
				Intent: tc.intent,
				Title:  "Test Conversation",
			}

			// Create empty message history
			messages := []*conversation.Message{}

			// Generate response
			response, err := suite.aiService.GenerateConversationalResponse(
				suite.ctx,
				conv,
				messages,
				tc.userMessage,
			)

			suite.NoError(err)
			suite.NotNil(response)
			suite.Equal(tc.intent, response.Intent)
			suite.Contains(strings.ToLower(response.Content), strings.ToLower(tc.expectedInResp))
			suite.Equal("ollama", response.Metadata["provider"])
			suite.Greater(response.Confidence, 0.8)

			suite.mockOllama.AssertExpectations(suite.T())
		})
	}
}

// TestOpenAIFallback tests OpenAI fallback when Ollama fails
func (suite *AIServiceIntegrationTestSuite) TestOpenAIFallback() {
	testCases := []struct {
		name           string
		intent         conversation.ConversationIntent
		userMessage    string
		expectedInResp string
		setupMocks     func()
	}{
		{
			name:           "Ollama Down - OpenAI Fallback",
			intent:         conversation.IntentRecipeCreation,
			userMessage:    "Create a chocolate cake recipe",
			expectedInResp: "chocolate",
			setupMocks: func() {
				// Ollama health check fails
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(assert.AnError)

				// OpenAI succeeds
				suite.mockOpenAI.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(messages []conversation.ChatMessage) bool {
					return len(messages) > 0 &&
						strings.Contains(messages[0].Content, "RECIPE CREATION MODE") &&
						messages[len(messages)-1].Content == "Create a chocolate cake recipe"
				})).Return("I'll help you create a delicious chocolate cake recipe! Here's what we'll need...", nil)
			},
		},
		{
			name:           "Ollama Generation Fails - OpenAI Fallback",
			intent:         conversation.IntentCookingHelp,
			userMessage:    "How do I make perfect scrambled eggs?",
			expectedInResp: "eggs",
			setupMocks: func() {
				// Ollama health check passes but generation fails
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
				suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
					Return("", assert.AnError)

				// OpenAI succeeds
				suite.mockOpenAI.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(messages []conversation.ChatMessage) bool {
					return len(messages) > 0 &&
						strings.Contains(messages[0].Content, "COOKING HELP MODE") &&
						messages[len(messages)-1].Content == "How do I make perfect scrambled eggs?"
				})).Return("For perfect scrambled eggs, use low heat and stir constantly. Add butter for creaminess.", nil)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			// Create test conversation
			conv := &conversation.Conversation{
				ID:     uuid.New().String(),
				UserID: "test-user",
				Intent: tc.intent,
				Title:  "Test Conversation",
			}

			messages := []*conversation.Message{}

			// Generate response
			response, err := suite.aiService.GenerateConversationalResponse(
				suite.ctx,
				conv,
				messages,
				tc.userMessage,
			)

			suite.NoError(err)
			suite.NotNil(response)
			suite.Equal(tc.intent, response.Intent)
			suite.Contains(strings.ToLower(response.Content), strings.ToLower(tc.expectedInResp))
			suite.Equal("openai", response.Metadata["provider"])
			suite.Greater(response.Confidence, 0.7)

			suite.mockOllama.AssertExpectations(suite.T())
			suite.mockOpenAI.AssertExpectations(suite.T())
		})
	}
}

// TestBothAIServicesDown tests fallback response when both services fail
func (suite *AIServiceIntegrationTestSuite) TestBothAIServicesDown() {
	intents := []conversation.ConversationIntent{
		conversation.IntentRecipeCreation,
		conversation.IntentCookingHelp,
		conversation.IntentIngredientSubst,
		conversation.IntentMealPlanning,
		conversation.IntentGeneralQuestion,
	}

	for _, intent := range intents {
		suite.Run(fmt.Sprintf("Fallback for %s", intent), func() {
			// Both services fail
			suite.mockOllama.On("HealthCheck", mock.Anything).Return(assert.AnError)
			suite.mockOpenAI.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
				Return("", assert.AnError)

			conv := &conversation.Conversation{
				ID:     uuid.New().String(),
				UserID: "test-user",
				Intent: intent,
				Title:  "Test Conversation",
			}

			messages := []*conversation.Message{}

			response, err := suite.aiService.GenerateConversationalResponse(
				suite.ctx,
				conv,
				messages,
				"Test message",
			)

			suite.NoError(err) // Should not error, should return fallback
			suite.NotNil(response)
			suite.Equal(intent, response.Intent)
			suite.Equal("fallback", response.Metadata["provider"])
			suite.Equal(0.5, response.Confidence)
			suite.Contains(response.Metadata, "reason")

			// Verify fallback content is appropriate for intent
			switch intent {
			case conversation.IntentRecipeCreation:
				suite.Contains(response.Content, "recipe")
			case conversation.IntentCookingHelp:
				suite.Contains(response.Content, "cooking")
			case conversation.IntentIngredientSubst:
				suite.Contains(response.Content, "substitution")
			case conversation.IntentMealPlanning:
				suite.Contains(response.Content, "meal")
			default:
				suite.Contains(response.Content, "AI chef")
			}

			suite.mockOllama.AssertExpectations(suite.T())
		})
	}
}

// TestConversationHistoryHandling tests how AI service handles conversation history
func (suite *AIServiceIntegrationTestSuite) TestConversationHistoryHandling() {
	suite.Run("With Conversation History", func() {
		// Setup Ollama to succeed
		suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
		suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(messages []conversation.ChatMessage) bool {
			// Should have system prompt + conversation history + current message
			// System + 4 history messages + current = 6 total
			if len(messages) != 6 {
				return false
			}

			// Check system prompt
			if messages[0].Role != "system" {
				return false
			}

			// Check conversation history is included
			if messages[1].Content != "I want to make pasta" {
				return false
			}

			if messages[2].Content != "What type of pasta?" {
				return false
			}

			// Check current message is last
			if messages[5].Content != "Spaghetti carbonara please" {
				return false
			}

			return true
		})).Return("Perfect! Spaghetti carbonara is a classic. Let me create a traditional recipe for you.", nil)

		conv := &conversation.Conversation{
			ID:     uuid.New().String(),
			UserID: "test-user",
			Intent: conversation.IntentRecipeCreation,
			Title:  "Pasta Recipe",
		}

		// Create conversation history
		messages := []*conversation.Message{
			{
				ID:             uuid.New().String(),
				ConversationID: conv.ID,
				Role:           conversation.RoleUser,
				Content:        "I want to make pasta",
				CreatedAt:      time.Now().Add(-5 * time.Minute),
			},
			{
				ID:             uuid.New().String(),
				ConversationID: conv.ID,
				Role:           conversation.RoleAssistant,
				Content:        "What type of pasta?",
				CreatedAt:      time.Now().Add(-4 * time.Minute),
			},
			{
				ID:             uuid.New().String(),
				ConversationID: conv.ID,
				Role:           conversation.RoleUser,
				Content:        "Something with eggs",
				CreatedAt:      time.Now().Add(-3 * time.Minute),
			},
			{
				ID:             uuid.New().String(),
				ConversationID: conv.ID,
				Role:           conversation.RoleAssistant,
				Content:        "How about carbonara?",
				CreatedAt:      time.Now().Add(-2 * time.Minute),
			},
		}

		response, err := suite.aiService.GenerateConversationalResponse(
			suite.ctx,
			conv,
			messages,
			"Spaghetti carbonara please",
		)

		suite.NoError(err)
		suite.NotNil(response)
		suite.Contains(response.Content, "carbonara")
		suite.Equal("ollama", response.Metadata["provider"])

		suite.mockOllama.AssertExpectations(suite.T())
	})

	suite.Run("With Long History - Truncation", func() {
		// Setup Ollama to succeed
		suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
		suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(messages []conversation.ChatMessage) bool {
			// Should limit to last 10 conversation messages + system + current
			// So max 12 messages (system + 10 history + current)
			return len(messages) <= 12
		})).Return("Based on our conversation, here's the carbonara recipe.", nil)

		conv := &conversation.Conversation{
			ID:     uuid.New().String(),
			UserID: "test-user",
			Intent: conversation.IntentRecipeCreation,
			Title:  "Long Conversation",
		}

		// Create long conversation history (15 messages)
		var messages []*conversation.Message
		for i := 0; i < 15; i++ {
			role := conversation.RoleUser
			content := fmt.Sprintf("User message %d", i+1)
			if i%2 == 1 {
				role = conversation.RoleAssistant
				content = fmt.Sprintf("Assistant response %d", i+1)
			}

			messages = append(messages, &conversation.Message{
				ID:             uuid.New().String(),
				ConversationID: conv.ID,
				Role:           role,
				Content:        content,
				CreatedAt:      time.Now().Add(-time.Duration(15-i) * time.Minute),
			})
		}

		response, err := suite.aiService.GenerateConversationalResponse(
			suite.ctx,
			conv,
			messages,
			"Final message",
		)

		suite.NoError(err)
		suite.NotNil(response)
		suite.Equal("ollama", response.Metadata["provider"])

		suite.mockOllama.AssertExpectations(suite.T())
	})
}

// TestSystemPromptGeneration tests system prompt generation for different intents
func (suite *AIServiceIntegrationTestSuite) TestSystemPromptGeneration() {
	testCases := []struct {
		intent           conversation.ConversationIntent
		expectedKeywords []string
	}{
		{
			intent:           conversation.IntentRecipeCreation,
			expectedKeywords: []string{"RECIPE CREATION MODE", "workflow", "ingredients", "instructions"},
		},
		{
			intent:           conversation.IntentCookingHelp,
			expectedKeywords: []string{"COOKING HELP MODE", "techniques", "safety", "explanations"},
		},
		{
			intent:           conversation.IntentIngredientSubst,
			expectedKeywords: []string{"INGREDIENT SUBSTITUTION MODE", "alternatives", "flavor profiles", "measurements"},
		},
		{
			intent:           conversation.IntentMealPlanning,
			expectedKeywords: []string{"MEAL PLANNING MODE", "nutritional", "budget", "time constraints"},
		},
		{
			intent:           conversation.IntentGeneralQuestion,
			expectedKeywords: []string{"GENERAL COOKING ASSISTANT MODE", "cooking-related", "clarifying questions"},
		},
	}

	for _, tc := range testCases {
		suite.Run(string(tc.intent), func() {
			suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
			suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(messages []conversation.ChatMessage) bool {
				if len(messages) == 0 {
					return false
				}

				systemPrompt := messages[0].Content
				if messages[0].Role != "system" {
					return false
				}

				// Check that all expected keywords are in the system prompt
				for _, keyword := range tc.expectedKeywords {
					if !strings.Contains(systemPrompt, keyword) {
						suite.T().Logf("Missing keyword '%s' in system prompt for intent %s", keyword, tc.intent)
						return false
					}
				}

				return true
			})).Return("Test response for "+string(tc.intent), nil)

			conv := &conversation.Conversation{
				ID:     uuid.New().String(),
				UserID: "test-user",
				Intent: tc.intent,
				Title:  "Test Conversation",
			}

			response, err := suite.aiService.GenerateConversationalResponse(
				suite.ctx,
				conv,
				[]*conversation.Message{},
				"Test message",
			)

			suite.NoError(err)
			suite.NotNil(response)
			suite.Equal(tc.intent, response.Intent)

			suite.mockOllama.AssertExpectations(suite.T())
		})
	}
}

// TestRecipeExtractionFromConversation tests recipe extraction functionality
func (suite *AIServiceIntegrationTestSuite) TestRecipeExtractionFromConversation() {
	testCases := []struct {
		name                string
		messages            []*conversation.Message
		expectedTitle       string
		expectedIngredients []string
		expectedStep        string
	}{
		{
			name: "Complete Recipe Discussion",
			messages: []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: "I want to make carbonara for 4 people",
				},
				{
					Role:    conversation.RoleAssistant,
					Content: "Great! Carbonara is delicious. You'll need pasta, eggs, cheese, and pancetta.",
				},
				{
					Role:    conversation.RoleUser,
					Content: "I have spaghetti, eggs, parmesan cheese, and bacon",
				},
				{
					Role:    conversation.RoleAssistant,
					Content: "Perfect! That will work well. Bacon is a great substitute for pancetta.",
				},
			},
			expectedTitle:       "Carbonara",
			expectedIngredients: []string{"pasta", "eggs", "cheese"},
			expectedStep:        "ready_to_create",
		},
		{
			name: "Partial Recipe Discussion",
			messages: []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: "I want to make something with chicken",
				},
				{
					Role:    conversation.RoleAssistant,
					Content: "What type of chicken dish would you like?",
				},
				{
					Role:    conversation.RoleUser,
					Content: "Maybe something healthy",
				},
			},
			expectedTitle:       "",
			expectedIngredients: []string{"chicken"},
			expectedStep:        "gathering_info",
		},
		{
			name: "Dietary Requirements Discussion",
			messages: []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: "I need a vegetarian pasta recipe that's gluten-free",
				},
				{
					Role:    conversation.RoleAssistant,
					Content: "I can help with that! How many servings do you need?",
				},
				{
					Role:    conversation.RoleUser,
					Content: "2 servings please, and I want it to be healthy",
				},
			},
			expectedTitle:       "Pasta",
			expectedIngredients: []string{"pasta"},
			expectedStep:        "gathering_details",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			context, err := suite.aiService.ExtractRecipeFromConversation(suite.ctx, tc.messages)

			suite.NoError(err)
			suite.NotNil(context)

			if tc.expectedTitle != "" {
				suite.Contains(context.RecipeTitle, tc.expectedTitle)
			}

			for _, expectedIngredient := range tc.expectedIngredients {
				found := false
				for _, ingredient := range context.Ingredients {
					if strings.Contains(strings.ToLower(ingredient), strings.ToLower(expectedIngredient)) {
						found = true
						break
					}
				}
				suite.True(found, "Expected ingredient '%s' not found in extracted ingredients", expectedIngredient)
			}

			suite.Equal(tc.expectedStep, context.CurrentStep)
		})
	}
}

// TestAIServicePerformance tests AI service performance characteristics
func (suite *AIServiceIntegrationTestSuite) TestAIServicePerformance() {
	suite.Run("Response Time Measurement", func() {
		suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
		suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
			Return("Quick response", nil)

		conv := &conversation.Conversation{
			ID:     uuid.New().String(),
			UserID: "test-user",
			Intent: conversation.IntentRecipeCreation,
			Title:  "Performance Test",
		}

		start := time.Now()
		response, err := suite.aiService.GenerateConversationalResponse(
			suite.ctx,
			conv,
			[]*conversation.Message{},
			"Quick test message",
		)
		duration := time.Since(start)

		suite.NoError(err)
		suite.NotNil(response)
		suite.Less(duration, 5*time.Second, "AI service should respond quickly in test environment")

		suite.mockOllama.AssertExpectations(suite.T())
	})

	suite.Run("Multiple Concurrent Requests", func() {
		const numRequests = 10

		// Setup mocks for concurrent requests
		suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil).Times(numRequests)
		suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
			Return("Concurrent response", nil).Times(numRequests)

		// Channel to collect results
		results := make(chan error, numRequests)

		// Launch concurrent requests
		for i := 0; i < numRequests; i++ {
			go func(index int) {
				conv := &conversation.Conversation{
					ID:     uuid.New().String(),
					UserID: "test-user",
					Intent: conversation.IntentRecipeCreation,
					Title:  fmt.Sprintf("Concurrent Test %d", index),
				}

				_, err := suite.aiService.GenerateConversationalResponse(
					suite.ctx,
					conv,
					[]*conversation.Message{},
					fmt.Sprintf("Concurrent message %d", index),
				)
				results <- err
			}(i)
		}

		// Collect all results
		for i := 0; i < numRequests; i++ {
			err := <-results
			suite.NoError(err, "Concurrent request %d should succeed", i)
		}

		suite.mockOllama.AssertExpectations(suite.T())
	})
}

// TestAIServiceErrorHandling tests comprehensive error handling
func (suite *AIServiceIntegrationTestSuite) TestAIServiceErrorHandling() {
	errorScenarios := []struct {
		name       string
		setupMocks func()
		expectErr  bool
	}{
		{
			name: "Context Timeout",
			setupMocks: func() {
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
				suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
					Return("", context.DeadlineExceeded)
			},
			expectErr: false, // Should fallback to OpenAI, then fallback response
		},
		{
			name: "Invalid Response Format",
			setupMocks: func() {
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil)
				suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
					Return("", fmt.Errorf("invalid response format"))
			},
			expectErr: false, // Should fallback
		},
		{
			name: "Network Error",
			setupMocks: func() {
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(fmt.Errorf("network unreachable"))
			},
			expectErr: false, // Should fallback to OpenAI
		},
	}

	for _, scenario := range errorScenarios {
		suite.Run(scenario.name, func() {
			scenario.setupMocks()

			// Setup OpenAI fallback to also fail
			suite.mockOpenAI.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
				Return("", assert.AnError).Maybe()

			conv := &conversation.Conversation{
				ID:     uuid.New().String(),
				UserID: "test-user",
				Intent: conversation.IntentRecipeCreation,
				Title:  "Error Test",
			}

			response, err := suite.aiService.GenerateConversationalResponse(
				suite.ctx,
				conv,
				[]*conversation.Message{},
				"Test message",
			)

			if scenario.expectErr {
				suite.Error(err)
				suite.Nil(response)
			} else {
				suite.NoError(err)
				suite.NotNil(response)
				// Should be fallback response
				if response.Metadata["provider"] == "fallback" {
					suite.Equal(0.5, response.Confidence)
				}
			}

			suite.mockOllama.AssertExpectations(suite.T())
		})
	}
}

// Run the test suite
func TestAIServiceIntegrationSuite(t *testing.T) {
	suite.Run(t, new(AIServiceIntegrationTestSuite))
}

// TestAIServiceWithDifferentModels tests AI service with different model configurations
func TestAIServiceWithDifferentModels(t *testing.T) {
	t.Run("Ollama Different Models", func(t *testing.T) {
		mockOllama := testutils.NewMockOllamaClient()
		mockOpenAI := testutils.NewMockOpenAIClient()
		aiService := conversation.NewAIService(mockOllama, mockOpenAI)

		models := []string{"llama2", "codellama", "mistral", "gemma"}

		for _, model := range models {
			t.Run(fmt.Sprintf("Model_%s", model), func(t *testing.T) {
				mockOllama.On("HealthCheck", mock.Anything).Return(nil)
				mockOllama.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
					Return(fmt.Sprintf("Response from %s model", model), nil)

				conv := &conversation.Conversation{
					ID:     uuid.New().String(),
					UserID: "test-user",
					Intent: conversation.IntentRecipeCreation,
					Title:  "Model Test",
				}

				response, err := aiService.GenerateConversationalResponse(
					context.Background(),
					conv,
					[]*conversation.Message{},
					"Test message",
				)

				require.NoError(t, err)
				require.NotNil(t, response)
				assert.Contains(t, response.Content, model)
				assert.Equal(t, "ollama", response.Metadata["provider"])

				mockOllama.AssertExpectations(t)
				mockOllama.Mock = mock.Mock{} // Reset for next iteration
				mockOllama.SetupStandardMockBehavior()
			})
		}
	})
}
