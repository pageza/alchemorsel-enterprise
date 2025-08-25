package unit

import (
	"context"
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

// AIServiceTestSuite tests the AI service integration
type AIServiceTestSuite struct {
	suite.Suite
	mockOllama   *testutils.MockOllamaClient
	mockOpenAI   *testutils.MockOpenAIClient
	aiService    *conversation.AIService
	ctx          context.Context
}

// SetupSuite initializes the test suite
func (suite *AIServiceTestSuite) SetupSuite() {
	suite.mockOllama = testutils.NewMockOllamaClient()
	suite.mockOpenAI = testutils.NewMockOpenAIClient()
	suite.aiService = conversation.NewAIService(suite.mockOllama, suite.mockOpenAI)
	suite.ctx = context.Background()
}

// TearDownTest cleans up after each test
func (suite *AIServiceTestSuite) TearDownTest() {
	// Reset mocks after each test
	suite.mockOllama.Mock = mock.Mock{}
	suite.mockOpenAI.Mock = mock.Mock{}
	suite.mockOllama.SetupStandardMockBehavior()
	suite.mockOpenAI.SetupStandardMockBehavior()
}

// TestGenerateConversationalResponse tests conversation response generation
func (suite *AIServiceTestSuite) TestGenerateConversationalResponse() {
	conv := &conversation.Conversation{
		ID:     uuid.New().String(),
		UserID: "test-user-id",
		Intent: conversation.IntentRecipeCreation,
		Title:  "Recipe Creation",
		Status: conversation.StatusActive,
	}

	messages := []*conversation.Message{
		{
			ID:             uuid.New().String(),
			ConversationID: conv.ID,
			Role:           conversation.RoleUser,
			Content:        "I want to make pasta",
			CreatedAt:      time.Now(),
		},
	}

	testCases := []struct {
		name            string
		userMessage     string
		setupMocks      func()
		expectedContent string
		expectedError   bool
	}{
		{
			name:        "Successful Ollama Response",
			userMessage: "How do I make carbonara?",
			setupMocks: func() {
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil).Once()
				suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(msgs []conversation.ChatMessage) bool {
					// Verify system prompt is included
					suite.True(len(msgs) > 0)
					suite.Equal("system", msgs[0].Role)
					suite.Contains(msgs[0].Content, "RECIPE CREATION MODE")
					
					// Verify user message is included
					userMsgFound := false
					for _, msg := range msgs {
						if msg.Role == "user" && msg.Content == "How do I make carbonara?" {
							userMsgFound = true
							break
						}
					}
					suite.True(userMsgFound)
					
					return true
				})).Return("I'll help you make carbonara! You'll need eggs, cheese, pancetta, and pasta.", nil).Once()
			},
			expectedContent: "I'll help you make carbonara! You'll need eggs, cheese, pancetta, and pasta.",
			expectedError:   false,
		},
		{
			name:        "Ollama Failure - OpenAI Fallback",
			userMessage: "What ingredients do I need?",
			setupMocks: func() {
				// Ollama health check fails
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(assert.AnError).Once()
				
				// OpenAI succeeds
				suite.mockOpenAI.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
					Return("For carbonara, you'll need: eggs, parmesan cheese, pancetta, black pepper, and spaghetti.", nil).Once()
			},
			expectedContent: "For carbonara, you'll need: eggs, parmesan cheese, pancetta, black pepper, and spaghetti.",
			expectedError:   false,
		},
		{
			name:        "Both Services Fail - Fallback Response",
			userMessage: "Help me cook",
			setupMocks: func() {
				// Ollama health check fails
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(assert.AnError).Once()
				
				// OpenAI fails
				suite.mockOpenAI.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
					Return("", assert.AnError).Once()
			},
			expectedContent: "I'd love to help you create a recipe!",
			expectedError:   false,
		},
		{
			name:        "Cooking Help Intent",
			userMessage: "How do I properly season meat?",
			setupMocks: func() {
				// Update conversation intent
				conv.Intent = conversation.IntentCookingHelp
				
				suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil).Once()
				suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(msgs []conversation.ChatMessage) bool {
					// Verify system prompt includes cooking help instructions
					suite.True(len(msgs) > 0)
					suite.Equal("system", msgs[0].Role)
					suite.Contains(msgs[0].Content, "COOKING HELP MODE")
					return true
				})).Return("Season meat 30-40 minutes before cooking. Use salt, pepper, and herbs like thyme or rosemary.", nil).Once()
			},
			expectedContent: "Season meat 30-40 minutes before cooking. Use salt, pepper, and herbs like thyme or rosemary.",
			expectedError:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			response, err := suite.aiService.GenerateConversationalResponse(suite.ctx, conv, messages, tc.userMessage)

			if tc.expectedError {
				suite.Error(err)
				suite.Nil(response)
			} else {
				suite.NoError(err)
				suite.NotNil(response)
				suite.Contains(response.Content, tc.expectedContent)
				suite.NotNil(response.Metadata)
			}

			suite.mockOllama.AssertExpectations(suite.T())
			suite.mockOpenAI.AssertExpectations(suite.T())
		})
	}
}

// TestBuildChatHistory tests chat history construction
func (suite *AIServiceTestSuite) TestBuildChatHistory() {
	conv := &conversation.Conversation{
		ID:     uuid.New().String(),
		Intent: conversation.IntentRecipeCreation,
	}

	// Create a long conversation history to test truncation
	messages := make([]*conversation.Message, 15)
	for i := 0; i < 15; i++ {
		role := conversation.RoleUser
		if i%2 == 1 {
			role = conversation.RoleAssistant
		}
		messages[i] = &conversation.Message{
			ID:             uuid.New().String(),
			ConversationID: conv.ID,
			Role:           role,
			Content:        fmt.Sprintf("Message %d", i),
			CreatedAt:      time.Now().Add(time.Duration(i) * time.Minute),
		}
	}

	suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil).Once()
	suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(msgs []conversation.ChatMessage) bool {
		// Should have system prompt + last 10 messages + current user message
		expectedLength := 1 + 10 + 1 // system + history + current
		suite.Equal(expectedLength, len(msgs))
		
		// First message should be system prompt
		suite.Equal("system", msgs[0].Role)
		suite.Contains(msgs[0].Content, "RECIPE CREATION MODE")
		
		// Last message should be the new user message
		suite.Equal("user", msgs[len(msgs)-1].Role)
		suite.Equal("New user message", msgs[len(msgs)-1].Content)
		
		return true
	})).Return("AI response", nil).Once()

	response, err := suite.aiService.GenerateConversationalResponse(suite.ctx, conv, messages, "New user message")

	suite.NoError(err)
	suite.NotNil(response)
	suite.mockOllama.AssertExpectations(suite.T())
}

// TestSystemPromptGeneration tests system prompt generation for different intents
func (suite *AIServiceTestSuite) TestSystemPromptGeneration() {
	testCases := []struct {
		name            string
		intent          conversation.ConversationIntent
		expectedContent []string
	}{
		{
			name:   "Recipe Creation Intent",
			intent: conversation.IntentRecipeCreation,
			expectedContent: []string{
				"RECIPE CREATION MODE",
				"GATHER INFORMATION",
				"CREATE RECIPE",
				"ingredient list",
				"step-by-step instructions",
			},
		},
		{
			name:   "Cooking Help Intent",
			intent: conversation.IntentCookingHelp,
			expectedContent: []string{
				"COOKING HELP MODE",
				"Clear, practical explanations",
				"Safety tips",
				"troubleshooting",
			},
		},
		{
			name:   "Ingredient Substitution Intent",
			intent: conversation.IntentIngredientSubst,
			expectedContent: []string{
				"INGREDIENT SUBSTITUTION MODE",
				"Similar flavor profiles",
				"Texture compatibility",
				"measurement conversions",
			},
		},
		{
			name:   "Meal Planning Intent",
			intent: conversation.IntentMealPlanning,
			expectedContent: []string{
				"MEAL PLANNING MODE",
				"Nutritional balance",
				"Time constraints",
				"Budget considerations",
			},
		},
		{
			name:   "General Question Intent",
			intent: conversation.IntentGeneralQuestion,
			expectedContent: []string{
				"GENERAL COOKING ASSISTANT MODE",
				"recipe requests",
				"meal planning",
				"cooking techniques",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			conv := &conversation.Conversation{
				ID:     uuid.New().String(),
				Intent: tc.intent,
			}

			suite.mockOllama.On("HealthCheck", mock.Anything).Return(nil).Once()
			suite.mockOllama.On("GenerateChatCompletion", mock.Anything, mock.MatchedBy(func(msgs []conversation.ChatMessage) bool {
				suite.True(len(msgs) > 0)
				suite.Equal("system", msgs[0].Role)
				
				systemPrompt := msgs[0].Content
				for _, expectedContent := range tc.expectedContent {
					suite.Contains(systemPrompt, expectedContent, "System prompt should contain: %s", expectedContent)
				}
				
				return true
			})).Return("AI response for " + string(tc.intent), nil).Once()

			response, err := suite.aiService.GenerateConversationalResponse(suite.ctx, conv, []*conversation.Message{}, "Test message")

			suite.NoError(err)
			suite.NotNil(response)
			suite.mockOllama.AssertExpectations(suite.T())
		})
	}
}

// TestExtractRecipeFromConversation tests recipe extraction from conversation
func (suite *AIServiceTestSuite) TestExtractRecipeFromConversation() {
	testCases := []struct {
		name             string
		messages         []*conversation.Message
		expectedTitle    string
		expectedStep     string
		expectedInfo     []string
		expectedServing  int
	}{
		{
			name: "Complete Recipe Information",
			messages: []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: "I want to make spaghetti carbonara for 4 people",
				},
				{
					Role:    conversation.RoleUser,
					Content: "I have eggs, cheese, pasta, and bacon",
				},
			},
			expectedTitle:   "Spaghetti Carbonara",
			expectedStep:    "gathering_details",
			expectedServing: 4,
		},
		{
			name: "Partial Recipe Information",
			messages: []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: "I want to make chocolate cake",
				},
			},
			expectedTitle: "Chocolate Cake",
			expectedStep:  "gathering_details",
			expectedInfo:  []string{"ingredients", "serving_size"},
		},
		{
			name: "No Specific Dish",
			messages: []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: "I want to cook something",
				},
			},
			expectedStep: "gathering_info",
			expectedInfo: []string{"dish_name", "ingredients", "serving_size"},
		},
		{
			name: "Dietary Requirements",
			messages: []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: "I want to make a vegan gluten-free pasta dish",
				},
			},
			expectedTitle: "Vegan Gluten-free Pasta Dish",
			expectedStep:  "gathering_details",
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

			suite.Equal(tc.expectedStep, context.CurrentStep)

			if tc.expectedServing > 0 {
				suite.Equal(tc.expectedServing, context.ServingSize)
			}

			if len(tc.expectedInfo) > 0 {
				for _, info := range tc.expectedInfo {
					suite.Contains(context.MissingInfo, info)
				}
			}
		})
	}
}

// TestIngredientExtraction tests ingredient extraction from messages
func (suite *AIServiceTestSuite) TestIngredientExtraction() {
	testCases := []struct {
		name                string
		content             string
		expectedIngredients []string
	}{
		{
			name:                "Common Ingredients",
			content:             "I have chicken, rice, onions, and garlic",
			expectedIngredients: []string{"chicken", "rice", "onions", "garlic"},
		},
		{
			name:                "Pasta Ingredients",
			content:             "Let's use pasta, tomatoes, basil, and cheese",
			expectedIngredients: []string{"pasta", "tomatoes", "basil", "cheese"},
		},
		{
			name:                "No Recognizable Ingredients",
			content:             "I want to cook something delicious",
			expectedIngredients: []string{},
		},
		{
			name:                "Mixed Content",
			content:             "I think beef would be good with mushrooms for dinner",
			expectedIngredients: []string{"beef", "mushrooms"},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			messages := []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: tc.content,
				},
			}

			context, err := suite.aiService.ExtractRecipeFromConversation(suite.ctx, messages)

			suite.NoError(err)
			suite.NotNil(context)

			for _, expectedIngredient := range tc.expectedIngredients {
				suite.Contains(context.Ingredients, expectedIngredient)
			}
		})
	}
}

// TestDietaryRequirementsExtraction tests dietary requirements extraction
func (suite *AIServiceTestSuite) TestDietaryRequirementsExtraction() {
	testCases := []struct {
		name                    string
		content                 string
		expectedDietaryReqs     []string
	}{
		{
			name:                "Vegan and Gluten-free",
			content:             "I need a vegan and gluten-free recipe",
			expectedDietaryReqs: []string{"vegan", "gluten-free"},
		},
		{
			name:                "Dairy-free",
			content:             "I can't have dairy products",
			expectedDietaryReqs: []string{"dairy-free"},
		},
		{
			name:                "Keto Diet",
			content:             "I'm on a keto diet",
			expectedDietaryReqs: []string{"keto"},
		},
		{
			name:                "Healthy Options",
			content:             "I want something healthy and low carb",
			expectedDietaryReqs: []string{"healthy", "low-carb"},
		},
		{
			name:                "No Dietary Requirements",
			content:             "I just want something tasty",
			expectedDietaryReqs: []string{},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			messages := []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: tc.content,
				},
			}

			context, err := suite.aiService.ExtractRecipeFromConversation(suite.ctx, messages)

			suite.NoError(err)
			suite.NotNil(context)

			for _, expectedReq := range tc.expectedDietaryReqs {
				suite.Contains(context.DietaryReqs, expectedReq)
			}
		})
	}
}

// TestServingSizeExtraction tests serving size extraction
func (suite *AIServiceTestSuite) TestServingSizeExtraction() {
	testCases := []struct {
		name            string
		content         string
		expectedServing int
	}{
		{
			name:            "4 People",
			content:         "I need to cook for 4 people",
			expectedServing: 4,
		},
		{
			name:            "2 Servings",
			content:         "Make 2 servings please",
			expectedServing: 2,
		},
		{
			name:            "1 Person",
			content:         "Just for 1 person tonight",
			expectedServing: 1,
		},
		{
			name:            "No Serving Size",
			content:         "I want to make pasta",
			expectedServing: 0,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			messages := []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: tc.content,
				},
			}

			context, err := suite.aiService.ExtractRecipeFromConversation(suite.ctx, messages)

			suite.NoError(err)
			suite.NotNil(context)
			suite.Equal(tc.expectedServing, context.ServingSize)
		})
	}
}

// TestCookingTimeExtraction tests cooking time extraction
func (suite *AIServiceTestSuite) TestCookingTimeExtraction() {
	testCases := []struct {
		name                string
		content             string
		expectedCookingTime int
	}{
		{
			name:                "30 Minutes",
			content:             "I have 30 minutes to cook",
			expectedCookingTime: 30,
		},
		{
			name:                "1 Hour",
			content:             "I can spend 1 hour cooking",
			expectedCookingTime: 60,
		},
		{
			name:                "Quick Recipe",
			content:             "I need something quick",
			expectedCookingTime: 15,
		},
		{
			name:                "No Time Constraint",
			content:             "I want to make something delicious",
			expectedCookingTime: 0,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			messages := []*conversation.Message{
				{
					Role:    conversation.RoleUser,
					Content: tc.content,
				},
			}

			context, err := suite.aiService.ExtractRecipeFromConversation(suite.ctx, messages)

			suite.NoError(err)
			suite.NotNil(context)
			suite.Equal(tc.expectedCookingTime, context.CookingTime)
		})
	}
}

// TestFallbackResponseGeneration tests fallback response generation
func (suite *AIServiceTestSuite) TestFallbackResponseGeneration() {
	testCases := []struct {
		name            string
		intent          conversation.ConversationIntent
		expectedContent []string
	}{
		{
			name:   "Recipe Creation Fallback",
			intent: conversation.IntentRecipeCreation,
			expectedContent: []string{
				"I'd love to help you create a recipe",
				"describe the dish",
				"ingredients you have",
				"dietary restrictions",
			},
		},
		{
			name:   "Cooking Help Fallback",
			intent: conversation.IntentCookingHelp,
			expectedContent: []string{
				"help with your cooking question",
				"cooking websites",
				"YouTube channels",
				"cooking forums",
			},
		},
		{
			name:   "Ingredient Substitution Fallback",
			intent: conversation.IntentIngredientSubst,
			expectedContent: []string{
				"ingredient substitutions",
				"common substitutions",
				"Butter → Oil",
				"substitution calculators",
			},
		},
		{
			name:   "Meal Planning Fallback",
			intent: conversation.IntentMealPlanning,
			expectedContent: []string{
				"plan meals",
				"what's in season",
				"meal prep",
				"weekly schedule",
			},
		},
		{
			name:   "General Fallback",
			intent: conversation.IntentGeneralQuestion,
			expectedContent: []string{
				"AI chef assistant",
				"technical difficulties",
				"recipe collection",
				"culinary questions",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Simulate both AI services failing
			suite.mockOllama.On("HealthCheck", mock.Anything).Return(assert.AnError).Once()
			suite.mockOpenAI.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
				Return("", assert.AnError).Once()

			conv := &conversation.Conversation{
				ID:     uuid.New().String(),
				Intent: tc.intent,
			}

			response, err := suite.aiService.GenerateConversationalResponse(suite.ctx, conv, []*conversation.Message{}, "Test message")

			suite.NoError(err)
			suite.NotNil(response)
			suite.Equal(tc.intent, response.Intent)
			suite.Equal(0.5, response.Confidence) // Fallback has lower confidence
			suite.Equal("fallback", response.Metadata["provider"])

			for _, expectedContent := range tc.expectedContent {
				suite.Contains(response.Content, expectedContent)
			}

			suite.mockOllama.AssertExpectations(suite.T())
			suite.mockOpenAI.AssertExpectations(suite.T())
		})
	}
}

// TestErrorHandling tests error handling in AI service
func (suite *AIServiceTestSuite) TestErrorHandling() {
	conv := &conversation.Conversation{
		ID:     uuid.New().String(),
		Intent: conversation.IntentRecipeCreation,
	}

	// Test nil Ollama client
	suite.Run("Nil Ollama Client", func() {
		aiService := conversation.NewAIService(nil, suite.mockOpenAI)
		
		suite.mockOpenAI.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
			Return("OpenAI response", nil).Once()

		response, err := aiService.GenerateConversationalResponse(suite.ctx, conv, []*conversation.Message{}, "Test message")

		suite.NoError(err)
		suite.NotNil(response)
		suite.Contains(response.Content, "OpenAI response")
		suite.mockOpenAI.AssertExpectations(suite.T())
	})

	// Test nil OpenAI client
	suite.Run("Nil OpenAI Client", func() {
		aiService := conversation.NewAIService(suite.mockOllama, nil)
		
		suite.mockOllama.On("HealthCheck", mock.Anything).Return(assert.AnError).Once()

		response, err := aiService.GenerateConversationalResponse(suite.ctx, conv, []*conversation.Message{}, "Test message")

		suite.NoError(err)
		suite.NotNil(response)
		suite.Equal("fallback", response.Metadata["provider"])
		suite.mockOllama.AssertExpectations(suite.T())
	})
}

// Run the test suite
func TestAIServiceSuite(t *testing.T) {
	suite.Run(t, new(AIServiceTestSuite))
}

// TestAIServicePerformance tests AI service performance characteristics
func TestAIServicePerformance(t *testing.T) {
	mockOllama := testutils.NewMockOllamaClient()
	mockOpenAI := testutils.NewMockOpenAIClient()
	aiService := conversation.NewAIService(mockOllama, mockOpenAI)
	ctx := context.Background()

	conv := &conversation.Conversation{
		ID:     uuid.New().String(),
		Intent: conversation.IntentRecipeCreation,
	}

	// Simulate slow AI response
	mockOllama.On("HealthCheck", mock.Anything).Return(nil).Maybe()
	mockOllama.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
		Return("AI response", nil).Run(func(args mock.Arguments) {
			time.Sleep(100 * time.Millisecond) // Simulate processing time
		}).Maybe()

	const numRequests = 10
	var totalDuration time.Duration

	for i := 0; i < numRequests; i++ {
		start := time.Now()
		
		response, err := aiService.GenerateConversationalResponse(ctx, conv, []*conversation.Message{}, "Test message")
		
		duration := time.Since(start)
		totalDuration += duration

		require.NoError(t, err)
		require.NotNil(t, response)
	}

	averageDuration := totalDuration / numRequests
	t.Logf("Average AI response time: %v", averageDuration)

	// Assert reasonable performance (adjust threshold based on requirements)
	assert.Less(t, averageDuration, 500*time.Millisecond, "AI responses should be reasonably fast")
}