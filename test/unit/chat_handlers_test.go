package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// ChatHandlerTestSuite tests the chat HTTP handlers
type ChatHandlerTestSuite struct {
	suite.Suite
	chatHandler *handlers.ChatHandler
	convService *conversation.Service
	testSuite   *testutils.ConversationTestSuite
	testUser    *user.User
	ctx         context.Context
}

// SetupSuite initializes the test suite
func (suite *ChatHandlerTestSuite) SetupSuite() {
	suite.testSuite = testutils.NewConversationTestSuite()
	suite.convService = suite.testSuite.ConversationService
	suite.chatHandler = handlers.NewChatHandler(suite.convService)
	suite.ctx = context.Background()

	// Create test user
	testUser, err := user.NewUser("test@example.com", "Test User", "password")
	if err != nil {
		panic(err) // Should not happen in tests
	}
	suite.testUser = testUser
}

// SetupTest runs before each test to ensure clean mock state
func (suite *ChatHandlerTestSuite) SetupTest() {
	// Reset all mocks to ensure clean state
	suite.testSuite.MockConversationRepo.ExpectedCalls = nil
	suite.testSuite.MockMessageRepo.ExpectedCalls = nil
	suite.testSuite.MockContextRepo.ExpectedCalls = nil
	suite.testSuite.MockAIService.ExpectedCalls = nil
	suite.testSuite.MockOllamaClient.ExpectedCalls = nil
	suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

	// Restore standard mock behaviors for tests that don't have specific setupMocks
	suite.testSuite.SetupStandardMocks()
}

// TestHandleChatMessage tests the HTTP chat message handler
func (suite *ChatHandlerTestSuite) TestHandleChatMessage() {
	testCases := []struct {
		name           string
		method         string
		contentType    string
		body           interface{}
		setupMocks     func()
		expectedStatus int
		expectedResp   map[string]interface{}
		withAuth       bool
	}{
		{
			name:        "Successful JSON Chat Message",
			method:      "POST",
			contentType: "application/json",
			body: handlers.HTTPChatRequest{
				Message: "I want to make pasta carbonara",
			},
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil
				
				// Create a test conversation with a known ID
				testConv := suite.testSuite.CreateTestConversation(suite.testUser.ID().String(), conversation.IntentRecipeCreation)
				
				// Set up only what we need for this test flow
				suite.testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.Anything).Return(nil)
				suite.testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.Anything).Return(nil)
				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, mock.Anything).Return(testConv, nil)
				suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, mock.Anything, 20, 0).Return([]*conversation.Message{}, nil)
				
				// AI service mocks (since this test uses real AIService)
				suite.testSuite.MockOllamaClient.On("HealthCheck", mock.Anything).Return(nil)
				suite.testSuite.MockOllamaClient.On("GenerateChatCompletion", mock.Anything, mock.Anything).Return("Great! I'll help you make carbonara. What ingredients do you have?", nil)
				
				suite.testSuite.MockContextRepo.On("SetContext", mock.Anything, mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusOK,
			expectedResp: map[string]interface{}{
				"success":  true,
				"response": "Great! I'll help you make carbonara. What ingredients do you have?",
			},
			withAuth: true,
		},
		{
			name:        "Successful Form Data Chat Message",
			method:      "POST",
			contentType: "application/x-www-form-urlencoded",
			body:        "message=How do I cook rice?&conversation_id=",
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil
				
				// Create a test conversation
				testConv := suite.testSuite.CreateTestConversation(suite.testUser.ID().String(), conversation.IntentCookingHelp)
				
				// Set up repository mocks
				suite.testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.Anything).Return(nil)
				suite.testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.Anything).Return(nil)
				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, mock.Anything).Return(testConv, nil)
				suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, mock.Anything, 20, 0).Return([]*conversation.Message{}, nil)
				
				// AI service mocks (using real AIService with mocked clients)
				suite.testSuite.MockOllamaClient.On("HealthCheck", mock.Anything).Return(nil)
				suite.testSuite.MockOllamaClient.On("GenerateChatCompletion", mock.Anything, mock.Anything).Return("To cook rice properly, use a 1:2 ratio of rice to water...", nil)
				
				suite.testSuite.MockContextRepo.On("SetContext", mock.Anything, mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusOK,
			expectedResp: map[string]interface{}{
				"success":  true,
				"response": "To cook rice properly, use a 1:2 ratio of rice to water...",
			},
			withAuth: true,
		},
		{
			name:           "Unauthenticated Request",
			method:         "POST",
			contentType:    "application/json",
			body:           handlers.HTTPChatRequest{Message: "Test message"},
			setupMocks:     func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedResp: map[string]interface{}{
				"success": false,
				"error":   "Authentication required",
			},
			withAuth: false,
		},
		{
			name:           "Empty Message",
			method:         "POST",
			contentType:    "application/json",
			body:           handlers.HTTPChatRequest{Message: ""},
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectedResp: map[string]interface{}{
				"success": false,
				"error":   "Message cannot be empty",
			},
			withAuth: true,
		},
		{
			name:           "Invalid JSON",
			method:         "POST",
			contentType:    "application/json",
			body:           `{"invalid": json}`,
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			expectedResp: map[string]interface{}{
				"success": false,
				"error":   "Invalid JSON",
			},
			withAuth: true,
		},
		{
			name:        "Existing Conversation",
			method:      "POST",
			contentType: "application/json",
			body: handlers.HTTPChatRequest{
				Message:        "What ingredients do I need?",
				ConversationID: "existing-conv-id",
			},
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

				// Mock message creation and processing for existing conversation
				suite.testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.Anything).Return(nil)

				testConv := suite.testSuite.CreateTestConversation(suite.testUser.ID().String(), conversation.IntentRecipeCreation)
				testConv.ID = "existing-conv-id"
				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, "existing-conv-id").Return(testConv, nil)
				suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, "existing-conv-id", 20, 0).Return([]*conversation.Message{}, nil)

				// AI service mocks (using real AIService with mocked clients)
				suite.testSuite.MockOllamaClient.On("HealthCheck", mock.Anything).Return(nil)
				suite.testSuite.MockOllamaClient.On("GenerateChatCompletion", mock.Anything, mock.Anything).Return("For carbonara you'll need pasta, eggs, cheese, and pancetta.", nil)

				suite.testSuite.MockContextRepo.On("SetContext", mock.Anything, mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusOK,
			expectedResp: map[string]interface{}{
				"success":         true,
				"response":        "For carbonara you'll need pasta, eggs, cheese, and pancetta.",
				"conversation_id": "existing-conv-id",
			},
			withAuth: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			// Create request
			var req *http.Request
			var err error

			if tc.contentType == "application/json" {
				var bodyBytes []byte
				if str, ok := tc.body.(string); ok {
					bodyBytes = []byte(str)
				} else {
					bodyBytes, _ = json.Marshal(tc.body)
				}
				req = httptest.NewRequest(tc.method, "/api/chat", bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", tc.contentType)
			} else {
				req = httptest.NewRequest(tc.method, "/api/chat", strings.NewReader(tc.body.(string)))
				req.Header.Set("Content-Type", tc.contentType)
			}

			// Add authentication if required
			if tc.withAuth {
				ctx := context.WithValue(req.Context(), "user", suite.testUser)
				req = req.WithContext(ctx)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Handle request
			suite.chatHandler.HandleChatMessage(w, req)

			// Check status code
			suite.Equal(tc.expectedStatus, w.Code)

			// Parse response
			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			suite.NoError(err)

			// Check expected response fields
			for key, expectedValue := range tc.expectedResp {
				if key == "response" && response["success"] == true {
					// For successful responses, just check that response contains expected content
					suite.Contains(fmt.Sprintf("%v", response[key]), fmt.Sprintf("%v", expectedValue))
				} else {
					suite.Equal(expectedValue, response[key], "Field %s should match", key)
				}
			}

			suite.testSuite.AssertExpectations(suite.T())
		})
	}
}

// TestHandleConversationList tests the conversation list handler
func (suite *ChatHandlerTestSuite) TestHandleConversationList() {
	testCases := []struct {
		name           string
		setupMocks     func()
		expectedStatus int
		withAuth       bool
		expectedCount  int
	}{
		{
			name: "Successful List",
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

				conversations := []*conversation.Conversation{
					suite.testSuite.CreateTestConversation(suite.testUser.ID().String(), conversation.IntentRecipeCreation),
					suite.testSuite.CreateTestConversation(suite.testUser.ID().String(), conversation.IntentCookingHelp),
				}
				suite.testSuite.MockConversationRepo.On("GetUserConversations", mock.Anything, suite.testUser.ID().String(), 50, 0).
					Return(conversations, nil)
			},
			expectedStatus: http.StatusOK,
			withAuth:       true,
			expectedCount:  2,
		},
		{
			name:           "Unauthenticated",
			setupMocks:     func() {},
			expectedStatus: http.StatusUnauthorized,
			withAuth:       false,
			expectedCount:  0,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			req := httptest.NewRequest("GET", "/api/conversations", nil)
			if tc.withAuth {
				ctx := context.WithValue(req.Context(), "user", suite.testUser)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			suite.chatHandler.HandleConversationList(w, req)

			suite.Equal(tc.expectedStatus, w.Code)

			if tc.withAuth && tc.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				suite.NoError(err)
				suite.True(response["success"].(bool))

				conversations := response["conversations"].([]interface{})
				suite.Len(conversations, tc.expectedCount)
			}

			suite.testSuite.AssertExpectations(suite.T())
		})
	}
}

// TestHandleConversationHistory tests the conversation history handler
func (suite *ChatHandlerTestSuite) TestHandleConversationHistory() {
	userConv := suite.testSuite.CreateTestConversation(suite.testUser.ID().String(), conversation.IntentRecipeCreation)
	otherUserConv := suite.testSuite.CreateTestConversation("other-user-id", conversation.IntentCookingHelp)

	testCases := []struct {
		name             string
		conversationID   string
		setupMocks       func()
		expectedStatus   int
		withAuth         bool
		expectConv       bool
		expectedMessages int
	}{
		{
			name:           "Successful History Retrieval",
			conversationID: userConv.ID,
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

				messages := []*conversation.Message{
					suite.testSuite.CreateTestMessage(userConv.ID, conversation.RoleUser, "I want pasta"),
					suite.testSuite.CreateTestMessage(userConv.ID, conversation.RoleAssistant, "What type?"),
				}
				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, userConv.ID).Return(userConv, nil)
				suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, userConv.ID, 100, 0).Return(messages, nil)
			},
			expectedStatus:   http.StatusOK,
			withAuth:         true,
			expectConv:       true,
			expectedMessages: 2,
		},
		{
			name:           "Access Denied - Other User's Conversation",
			conversationID: otherUserConv.ID,
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil
				
				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, otherUserConv.ID).Return(otherUserConv, nil)
				suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, otherUserConv.ID, 100, 0).Return([]*conversation.Message{}, nil)
			},
			expectedStatus: http.StatusForbidden,
			withAuth:       true,
			expectConv:     false,
		},
		{
			name:           "Missing Conversation ID",
			conversationID: "",
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			withAuth:       true,
			expectConv:     false,
		},
		{
			name:           "Unauthenticated",
			conversationID: userConv.ID,
			setupMocks:     func() {},
			expectedStatus: http.StatusUnauthorized,
			withAuth:       false,
			expectConv:     false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			url := "/api/conversation/history"
			if tc.conversationID != "" {
				url += "?conversation_id=" + tc.conversationID
			}

			req := httptest.NewRequest("GET", url, nil)
			if tc.withAuth {
				ctx := context.WithValue(req.Context(), "user", suite.testUser)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			suite.chatHandler.HandleConversationHistory(w, req)

			suite.Equal(tc.expectedStatus, w.Code)

			if tc.expectConv && tc.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				suite.NoError(err)
				suite.True(response["success"].(bool))

				conversation := response["conversation"].(map[string]interface{})
				suite.Equal(userConv.ID, conversation["id"])

				messages := response["messages"].([]interface{})
				suite.Len(messages, tc.expectedMessages)
			}

			suite.testSuite.AssertExpectations(suite.T())
		})
	}
}

// TestHandleConversationDelete tests the conversation deletion handler
func (suite *ChatHandlerTestSuite) TestHandleConversationDelete() {
	userConv := suite.testSuite.CreateTestConversation(suite.testUser.ID().String(), conversation.IntentRecipeCreation)
	otherUserConv := suite.testSuite.CreateTestConversation("other-user-id", conversation.IntentCookingHelp)

	testCases := []struct {
		name           string
		conversationID string
		setupMocks     func()
		expectedStatus int
		withAuth       bool
	}{
		{
			name:           "Successful Deletion",
			conversationID: userConv.ID,
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, userConv.ID).Return(userConv, nil)
				suite.testSuite.MockConversationRepo.On("UpdateConversation", mock.Anything, mock.Anything).Return(nil)
			},
			expectedStatus: http.StatusOK,
			withAuth:       true,
		},
		{
			name:           "Access Denied - Other User's Conversation",
			conversationID: otherUserConv.ID,
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, otherUserConv.ID).Return(otherUserConv, nil)
			},
			expectedStatus: http.StatusForbidden,
			withAuth:       true,
		},
		{
			name:           "Missing Conversation ID",
			conversationID: "",
			setupMocks:     func() {},
			expectedStatus: http.StatusBadRequest,
			withAuth:       true,
		},
		{
			name:           "Unauthenticated",
			conversationID: userConv.ID,
			setupMocks:     func() {},
			expectedStatus: http.StatusUnauthorized,
			withAuth:       false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			form := url.Values{}
			if tc.conversationID != "" {
				form.Set("conversation_id", tc.conversationID)
			}

			req := httptest.NewRequest("POST", "/api/conversation/delete", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			if tc.withAuth {
				ctx := context.WithValue(req.Context(), "user", suite.testUser)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			suite.chatHandler.HandleConversationDelete(w, req)

			suite.Equal(tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				suite.NoError(err)
				suite.True(response["success"].(bool))
			}

			suite.testSuite.AssertExpectations(suite.T())
		})
	}
}

// TestHandleConversationStats tests the conversation statistics handler
func (suite *ChatHandlerTestSuite) TestHandleConversationStats() {
	testCases := []struct {
		name           string
		setupMocks     func()
		expectedStatus int
		withAuth       bool
	}{
		{
			name: "Successful Stats Retrieval",
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

				conversations := []*conversation.Conversation{
					{
						ID:     uuid.New().String(),
						UserID: suite.testUser.ID().String(),
						Intent: conversation.IntentRecipeCreation,
						Status: conversation.StatusActive,
					},
					{
						ID:     uuid.New().String(),
						UserID: suite.testUser.ID().String(),
						Intent: conversation.IntentCookingHelp,
						Status: conversation.StatusArchived,
					},
				}
				suite.testSuite.MockConversationRepo.On("GetUserConversations", mock.Anything, suite.testUser.ID().String(), 1000, 0).
					Return(conversations, nil)
			},
			expectedStatus: http.StatusOK,
			withAuth:       true,
		},
		{
			name:           "Unauthenticated",
			setupMocks:     func() {},
			expectedStatus: http.StatusUnauthorized,
			withAuth:       false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			req := httptest.NewRequest("GET", "/api/conversation/stats", nil)
			if tc.withAuth {
				ctx := context.WithValue(req.Context(), "user", suite.testUser)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			suite.chatHandler.HandleConversationStats(w, req)

			suite.Equal(tc.expectedStatus, w.Code)

			if tc.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				suite.NoError(err)
				suite.True(response["success"].(bool))

				stats := response["stats"].(map[string]interface{})
				suite.Contains(stats, "total_conversations")
				suite.Contains(stats, "active_conversations")
				suite.Contains(stats, "archived_conversations")
				suite.Contains(stats, "intents")
			}

			suite.testSuite.AssertExpectations(suite.T())
		})
	}
}

// TestHandleAIChatHTMX tests the HTMX chat handler
func (suite *ChatHandlerTestSuite) TestHandleAIChatHTMX() {
	testCases := []struct {
		name             string
		message          string
		conversationID   string
		setupMocks       func()
		withAuth         bool
		expectedContains []string
	}{
		{
			name:    "Successful HTMX Chat",
			message: "I want to make pasta",
			setupMocks: func() {
				// Clear all mocks before setting up this test
				suite.testSuite.MockConversationRepo.ExpectedCalls = nil
				suite.testSuite.MockMessageRepo.ExpectedCalls = nil
				suite.testSuite.MockContextRepo.ExpectedCalls = nil
				suite.testSuite.MockOllamaClient.ExpectedCalls = nil
				suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

				// Mock conversation creation and processing
				suite.testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.Anything).Return(nil)
				suite.testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.Anything).Return(nil)

				testConv := suite.testSuite.CreateTestConversation(suite.testUser.ID().String(), conversation.IntentRecipeCreation)
				suite.testSuite.MockConversationRepo.On("GetConversation", mock.Anything, mock.Anything).Return(testConv, nil)
				suite.testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, mock.Anything, 20, 0).Return([]*conversation.Message{}, nil)

				// AI service mocks (using real AIService with mocked clients)
				suite.testSuite.MockOllamaClient.On("HealthCheck", mock.Anything).Return(nil)
				suite.testSuite.MockOllamaClient.On("GenerateChatCompletion", mock.Anything, mock.Anything).Return("I'll help you make pasta! What type would you like?", nil)

				suite.testSuite.MockContextRepo.On("SetContext", mock.Anything, mock.Anything).Return(nil)
			},
			withAuth: true,
			expectedContains: []string{
				"I want to make pasta",
				"I'll help you make pasta! What type would you like?",
				"message user",
				"message assistant",
				"Test User",
				"AI Chef",
			},
		},
		{
			name:       "Unauthenticated HTMX Chat",
			message:    "Hello",
			setupMocks: func() {},
			withAuth:   false,
			expectedContains: []string{
				"Hello",
				"need to be logged in",
				"Login",
				"Register",
			},
		},
		{
			name:       "Empty Message",
			message:    "",
			setupMocks: func() {},
			withAuth:   true,
			expectedContains: []string{
				"Message cannot be empty",
				"❌",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.setupMocks()

			form := url.Values{}
			form.Set("message", tc.message)
			if tc.conversationID != "" {
				form.Set("conversation_id", tc.conversationID)
			}

			req := httptest.NewRequest("POST", "/chat/htmx", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			if tc.withAuth {
				ctx := context.WithValue(req.Context(), "user", suite.testUser)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			suite.chatHandler.HandleAIChatHTMX(w, req)

			suite.Equal(http.StatusOK, w.Code)
			suite.Equal("text/html", w.Header().Get("Content-Type"))

			responseBody := w.Body.String()
			for _, expectedContent := range tc.expectedContains {
				suite.Contains(responseBody, expectedContent)
			}

			suite.testSuite.AssertExpectations(suite.T())
		})
	}
}

// TestChatHandlerErrorScenarios tests various error scenarios
func (suite *ChatHandlerTestSuite) TestChatHandlerErrorScenarios() {
	suite.Run("Service Errors", func() {
		// Clear all mocks before setting up this test
		suite.testSuite.MockConversationRepo.ExpectedCalls = nil
		suite.testSuite.MockMessageRepo.ExpectedCalls = nil
		suite.testSuite.MockContextRepo.ExpectedCalls = nil
		suite.testSuite.MockOllamaClient.ExpectedCalls = nil
		suite.testSuite.MockOpenAIClient.ExpectedCalls = nil

		// Mock service to return error
		suite.testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.Anything).
			Return(assert.AnError).Once()

		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message": "test"}`))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user", suite.testUser)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		suite.chatHandler.HandleChatMessage(w, req)

		suite.Equal(http.StatusInternalServerError, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		suite.NoError(err)
		suite.False(response["success"].(bool))
		suite.Contains(response["error"], "Failed to create conversation")

		suite.testSuite.AssertExpectations(suite.T())
	})
}

// TestChatHandlerMiddleware tests middleware integration
func (suite *ChatHandlerTestSuite) TestChatHandlerMiddleware() {
	suite.Run("CORS Headers", func() {
		req := httptest.NewRequest("OPTIONS", "/api/chat", nil)
		req.Header.Set("Origin", "https://example.com")

		w := httptest.NewRecorder()
		suite.chatHandler.HandleChatMessage(w, req)

		// In a real implementation, you'd have CORS middleware that sets these headers
		// This test is more about verifying the handler works with different request types
	})
}

// Run the test suite
func TestChatHandlerSuite(t *testing.T) {
	suite.Run(t, new(ChatHandlerTestSuite))
}

// TestChatHandlerPerformance tests performance characteristics
func TestChatHandlerPerformance(t *testing.T) {
	testSuite := testutils.NewConversationTestSuite()
	chatHandler := handlers.NewChatHandler(testSuite.ConversationService)

	testUser, err := user.NewUser("test@example.com", "Test User", "password")
	if err != nil {
		panic(err) // Should not happen in tests
	}

	t.Run("Concurrent Requests", func(t *testing.T) {
		const numRequests = 3

		// Reset all mocks to ensure clean state
		testSuite.MockConversationRepo.ExpectedCalls = nil
		testSuite.MockMessageRepo.ExpectedCalls = nil
		testSuite.MockContextRepo.ExpectedCalls = nil
		testSuite.MockOllamaClient.ExpectedCalls = nil
		testSuite.MockOpenAIClient.ExpectedCalls = nil

		// Setup mocks for concurrent requests
		testSuite.MockConversationRepo.On("CreateConversation", mock.Anything, mock.Anything).Return(nil).Times(numRequests)
		testSuite.MockMessageRepo.On("CreateMessage", mock.Anything, mock.Anything).Return(nil).Times(numRequests)
		testSuite.MockConversationRepo.On("GetConversation", mock.Anything, mock.AnythingOfType("string")).
			Return(testSuite.CreateTestConversation(testUser.ID().String(), conversation.IntentRecipeCreation), nil).Times(numRequests)
		testSuite.MockMessageRepo.On("GetConversationMessages", mock.Anything, mock.AnythingOfType("string"), 20, 0).
			Return([]*conversation.Message{}, nil).Times(numRequests)

		// Restore critical AI client expectations after manual reset (performance test uses real AIService)
		testSuite.MockOllamaClient.On("HealthCheck", mock.Anything).Return(nil).Times(numRequests)
		testSuite.MockOllamaClient.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
			Return("Test response from AI", nil).Times(numRequests)
		testSuite.MockOpenAIClient.On("GenerateChatCompletion", mock.Anything, mock.AnythingOfType("[]conversation.ChatMessage")).
			Return("I'm your AI cooking assistant! How can I help you today?", nil).Maybe()

		testSuite.MockContextRepo.On("SetContext", mock.Anything, mock.Anything).Return(nil).Times(numRequests)

		// Channel to collect results
		results := make(chan int, numRequests)

		// Launch concurrent requests
		for i := 0; i < numRequests; i++ {
			go func(index int) {
				reqBody := fmt.Sprintf(`{"message": "Test message %d"}`, index)
				req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(reqBody))
				req.Header.Set("Content-Type", "application/json")
				ctx := context.WithValue(req.Context(), "user", testUser)
				req = req.WithContext(ctx)

				w := httptest.NewRecorder()
				chatHandler.HandleChatMessage(w, req)

				results <- w.Code
			}(i)
		}

		// Collect results
		for i := 0; i < numRequests; i++ {
			statusCode := <-results
			assert.Equal(t, http.StatusOK, statusCode, "Request %d should succeed", i)
		}

		testSuite.AssertExpectations(t)
	})
}
