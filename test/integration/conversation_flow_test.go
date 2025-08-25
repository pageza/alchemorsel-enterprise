package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/gorm"
	"github.com/alchemorsel/v3/internal/infrastructure/websocket"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	gormdb "gorm.io/gorm"
)

// ConversationFlowTestSuite tests complete conversation flows with real database and WebSocket
type ConversationFlowTestSuite struct {
	suite.Suite
	db              *gormdb.DB
	conversationRepo *gorm.ConversationRepository
	messageRepo     *gorm.MessageRepository
	contextRepo     *gorm.ContextRepository
	convService     *conversation.Service
	aiService       *conversation.AIService
	wsManager       *websocket.Manager
	chatHandler     *handlers.ChatHandler
	wsHandler       *handlers.WebSocketHandler
	server          *httptest.Server
	ctx             context.Context
	cancel          context.CancelFunc
	testUsers       map[string]*handlers.User
}

// SetupSuite initializes the test suite with real components
func (suite *ConversationFlowTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	
	// Setup test database
	suite.db = testutils.SetupTestDatabase(suite.T())
	
	// Create repositories
	suite.conversationRepo = gorm.NewConversationRepository(suite.db)
	suite.messageRepo = gorm.NewMessageRepository(suite.db)
	suite.contextRepo = gorm.NewContextRepository(suite.db)
	
	// Create mock AI service for integration testing
	mockOllamaClient := testutils.NewMockOllamaClient()
	mockOpenAIClient := testutils.NewMockOpenAIClient()
	suite.aiService = conversation.NewAIService(mockOllamaClient, mockOpenAIClient)
	
	// Setup standard AI mock behavior
	mockOllamaClient.SetupStandardMockBehavior()
	mockOpenAIClient.SetupStandardMockBehavior()
	
	// Create conversation service
	suite.convService = conversation.NewService(
		suite.conversationRepo,
		suite.messageRepo,
		suite.contextRepo,
		suite.aiService,
	)
	
	// Create WebSocket manager
	suite.wsManager = websocket.NewManager()
	go suite.wsManager.Start(suite.ctx)
	
	// Create handlers
	suite.chatHandler = handlers.NewChatHandler(suite.convService)
	suite.wsHandler = handlers.NewWebSocketHandler(suite.wsManager, suite.convService)
	
	// Create test server
	suite.server = httptest.NewServer(suite.createTestMux())
	
	// Create test users
	suite.testUsers = map[string]*handlers.User{
		"chef": {
			ID:    "550e8400-e29b-41d4-a716-446655440001",
			Email: "chef@example.com",
			Name:  "Chef User",
		},
		"home_cook": {
			ID:    "550e8400-e29b-41d4-a716-446655440002",
			Email: "homecook@example.com",
			Name:  "Home Cook",
		},
	}
}

// TearDownSuite cleans up the test suite
func (suite *ConversationFlowTestSuite) TearDownSuite() {
	suite.cancel()
	suite.server.Close()
	testutils.TeardownTestDatabase(suite.T(), suite.db)
}

// SetupTest cleans up data before each test
func (suite *ConversationFlowTestSuite) SetupTest() {
	testutils.CleanupTestData(suite.T(), suite.db)
}

// createTestMux creates a test HTTP mux with all necessary routes
func (suite *ConversationFlowTestSuite) createTestMux() http.Handler {
	mux := http.NewServeMux()
	
	// Add authentication middleware for API routes
	authMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID := r.Header.Get("X-User-ID")
			if userID == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			
			// Find user by ID
			var user *handlers.User
			for _, u := range suite.testUsers {
				if u.ID == userID {
					user = u
					break
				}
			}
			
			if user == nil {
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}
			
			ctx := context.WithValue(r.Context(), "user", user)
			next(w, r.WithContext(ctx))
		}
	}
	
	// API routes
	mux.HandleFunc("/api/chat", authMiddleware(suite.chatHandler.HandleChatMessage))
	mux.HandleFunc("/api/conversations", authMiddleware(suite.chatHandler.HandleConversationList))
	mux.HandleFunc("/api/conversation/history", authMiddleware(suite.chatHandler.HandleConversationHistory))
	mux.HandleFunc("/api/conversation/delete", authMiddleware(suite.chatHandler.HandleConversationDelete))
	mux.HandleFunc("/api/conversation/stats", authMiddleware(suite.chatHandler.HandleConversationStats))
	mux.HandleFunc("/chat/htmx", authMiddleware(suite.chatHandler.HandleAIChatHTMX))
	
	// WebSocket route
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		err := suite.wsManager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	
	return mux
}

// TestCompleteRecipeCreationFlow tests a complete recipe creation conversation
func (suite *ConversationFlowTestSuite) TestCompleteRecipeCreationFlow() {
	userID := suite.testUsers["chef"].ID
	
	// Step 1: Start conversation with recipe request
	resp1 := suite.sendChatMessage(userID, "I want to create a recipe for pasta carbonara", "")
	suite.Equal(http.StatusOK, resp1.StatusCode)
	
	var chatResp1 handlers.HTTPChatResponse
	err := json.NewDecoder(resp1.Body).Decode(&chatResp1)
	suite.NoError(err)
	suite.True(chatResp1.Success)
	suite.NotEmpty(chatResp1.ConversationID)
	suite.Contains(strings.ToLower(chatResp1.Response), "carbonara")
	
	conversationID := chatResp1.ConversationID
	
	// Step 2: Provide serving size
	resp2 := suite.sendChatMessage(userID, "For 4 people please", conversationID)
	suite.Equal(http.StatusOK, resp2.StatusCode)
	
	var chatResp2 handlers.HTTPChatResponse
	err = json.NewDecoder(resp2.Body).Decode(&chatResp2)
	suite.NoError(err)
	suite.True(chatResp2.Success)
	suite.Equal(conversationID, chatResp2.ConversationID)
	
	// Step 3: Specify dietary requirements
	resp3 := suite.sendChatMessage(userID, "No dietary restrictions", conversationID)
	suite.Equal(http.StatusOK, resp3.StatusCode)
	
	// Step 4: Finalize recipe
	resp4 := suite.sendChatMessage(userID, "Yes, please create the complete recipe", conversationID)
	suite.Equal(http.StatusOK, resp4.StatusCode)
	
	// Verify conversation was created and persisted
	conv, err := suite.conversationRepo.GetConversation(suite.ctx, conversationID)
	suite.NoError(err)
	suite.Equal(userID, conv.UserID)
	suite.Equal(conversation.IntentRecipeCreation, conv.Intent)
	suite.Equal(conversation.StatusActive, conv.Status)
	suite.Contains(conv.Title, "Recipe")
	
	// Verify messages were stored
	messages, err := suite.messageRepo.GetConversationMessages(suite.ctx, conversationID, 100, 0)
	suite.NoError(err)
	suite.GreaterOrEqual(len(messages), 8) // At least 4 user + 4 assistant messages
	
	// Verify message order and content
	suite.Equal(conversation.RoleUser, messages[0].Role)
	suite.Contains(messages[0].Content, "carbonara")
	suite.Equal(conversation.RoleAssistant, messages[1].Role)
	
	// Verify conversation appears in user's conversation list
	userConversations, err := suite.conversationRepo.GetUserConversations(suite.ctx, userID, 10, 0)
	suite.NoError(err)
	suite.Len(userConversations, 1)
	suite.Equal(conversationID, userConversations[0].ID)
}

// TestMultiUserConversationIsolation tests that conversations are isolated between users
func (suite *ConversationFlowTestSuite) TestMultiUserConversationIsolation() {
	user1ID := suite.testUsers["chef"].ID
	user2ID := suite.testUsers["home_cook"].ID
	
	// User 1 creates a conversation
	resp1 := suite.sendChatMessage(user1ID, "I want to make pizza", "")
	suite.Equal(http.StatusOK, resp1.StatusCode)
	
	var chatResp1 handlers.HTTPChatResponse
	err := json.NewDecoder(resp1.Body).Decode(&chatResp1)
	suite.NoError(err)
	conv1ID := chatResp1.ConversationID
	
	// User 2 creates a different conversation
	resp2 := suite.sendChatMessage(user2ID, "How do I boil eggs?", "")
	suite.Equal(http.StatusOK, resp2.StatusCode)
	
	var chatResp2 handlers.HTTPChatResponse
	err = json.NewDecoder(resp2.Body).Decode(&chatResp2)
	suite.NoError(err)
	conv2ID := chatResp2.ConversationID
	
	// Verify conversations are different
	suite.NotEqual(conv1ID, conv2ID)
	
	// Verify user 1 can only see their conversation
	user1Conversations, err := suite.conversationRepo.GetUserConversations(suite.ctx, user1ID, 10, 0)
	suite.NoError(err)
	suite.Len(user1Conversations, 1)
	suite.Equal(conv1ID, user1Conversations[0].ID)
	
	// Verify user 2 can only see their conversation
	user2Conversations, err := suite.conversationRepo.GetUserConversations(suite.ctx, user2ID, 10, 0)
	suite.NoError(err)
	suite.Len(user2Conversations, 1)
	suite.Equal(conv2ID, user2Conversations[0].ID)
	
	// Verify user 1 cannot access user 2's conversation history
	resp3 := suite.getConversationHistory(user1ID, conv2ID)
	suite.Equal(http.StatusForbidden, resp3.StatusCode)
	
	// Verify user 2 cannot access user 1's conversation history
	resp4 := suite.getConversationHistory(user2ID, conv1ID)
	suite.Equal(http.StatusForbidden, resp4.StatusCode)
}

// TestWebSocketConversationFlow tests conversation flow through WebSocket
func (suite *ConversationFlowTestSuite) TestWebSocketConversationFlow() {
	userID := suite.testUsers["chef"].ID
	
	// Connect to WebSocket
	wsURL := "ws" + suite.server.URL[4:] + "/ws"
	headers := http.Header{}
	headers.Set("X-User-ID", userID)
	
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	suite.Require().NoError(err)
	defer conn.Close()
	
	// Wait for connection established message
	var welcomeMsg websocket.Message
	err = conn.ReadJSON(&welcomeMsg)
	suite.NoError(err)
	suite.Equal("connection_established", welcomeMsg.Type)
	
	// Send chat message through WebSocket
	chatMessage := map[string]interface{}{
		"message":         "I want to make risotto",
		"conversation_id": "",
	}
	chatMessageJSON, _ := json.Marshal(chatMessage)
	
	wsMessage := websocket.Message{
		ID:        uuid.New().String(),
		Type:      "chat_message",
		Content:   string(chatMessageJSON),
		Timestamp: time.Now(),
	}
	
	err = conn.WriteJSON(wsMessage)
	suite.NoError(err)
	
	// Read conversation created response
	var convCreatedMsg websocket.Message
	err = conn.ReadJSON(&convCreatedMsg)
	suite.NoError(err)
	suite.Equal("conversation_created", convCreatedMsg.Type)
	
	// Read AI response
	var aiResponseMsg websocket.Message
	err = conn.ReadJSON(&aiResponseMsg)
	suite.NoError(err)
	suite.Equal("chat_response", aiResponseMsg.Type)
	
	var chatResponse handlers.ChatResponse
	err = json.Unmarshal([]byte(aiResponseMsg.Content), &chatResponse)
	suite.NoError(err)
	suite.Contains(strings.ToLower(chatResponse.Content), "risotto")
	suite.NotEmpty(chatResponse.ConversationID)
	
	// Verify conversation was persisted
	conv, err := suite.conversationRepo.GetConversation(suite.ctx, chatResponse.ConversationID)
	suite.NoError(err)
	suite.Equal(userID, conv.UserID)
	suite.Equal(conversation.IntentRecipeCreation, conv.Intent)
	
	// Send follow-up message
	followUpMessage := map[string]interface{}{
		"message":         "Mushroom risotto please",
		"conversation_id": chatResponse.ConversationID,
	}
	followUpJSON, _ := json.Marshal(followUpMessage)
	
	followUpWSMessage := websocket.Message{
		ID:        uuid.New().String(),
		Type:      "chat_message",
		Content:   string(followUpJSON),
		Timestamp: time.Now(),
	}
	
	err = conn.WriteJSON(followUpWSMessage)
	suite.NoError(err)
	
	// Read follow-up response
	var followUpResponse websocket.Message
	err = conn.ReadJSON(&followUpResponse)
	suite.NoError(err)
	suite.Equal("chat_response", followUpResponse.Type)
	
	// Verify messages were persisted
	messages, err := suite.messageRepo.GetConversationMessages(suite.ctx, chatResponse.ConversationID, 100, 0)
	suite.NoError(err)
	suite.GreaterOrEqual(len(messages), 4) // At least 2 user + 2 assistant messages
}

// TestConversationContextPersistence tests that conversation context is maintained
func (suite *ConversationFlowTestSuite) TestConversationContextPersistence() {
	userID := suite.testUsers["chef"].ID
	
	// Start conversation
	resp1 := suite.sendChatMessage(userID, "I want to make bread", "")
	suite.Equal(http.StatusOK, resp1.StatusCode)
	
	var chatResp1 handlers.HTTPChatResponse
	err := json.NewDecoder(resp1.Body).Decode(&chatResp1)
	suite.NoError(err)
	conversationID := chatResp1.ConversationID
	
	// Set some context manually (simulating AI service setting context)
	contextData := map[string]interface{}{
		"recipe_type":    "bread",
		"skill_level":    "beginner",
		"preferences":    []string{"whole wheat", "no nuts"},
		"progress_step":  "gathering_requirements",
	}
	
	err = suite.convService.SetContext(suite.ctx, conversationID, "recipe_creation", contextData)
	suite.NoError(err)
	
	// Continue conversation
	resp2 := suite.sendChatMessage(userID, "I want whole wheat bread", conversationID)
	suite.Equal(http.StatusOK, resp2.StatusCode)
	
	// Verify context is preserved
	retrievedContext, err := suite.convService.GetContext(suite.ctx, conversationID, "recipe_creation")
	suite.NoError(err)
	suite.Equal("bread", retrievedContext["recipe_type"])
	suite.Equal("beginner", retrievedContext["skill_level"])
	
	// Add more context
	additionalContext := map[string]interface{}{
		"recipe_type":   "bread",
		"bread_type":    "whole_wheat",
		"progress_step": "creating_recipe",
	}
	
	err = suite.convService.SetContext(suite.ctx, conversationID, "recipe_creation", additionalContext)
	suite.NoError(err)
	
	// Verify updated context
	updatedContext, err := suite.convService.GetContext(suite.ctx, conversationID, "recipe_creation")
	suite.NoError(err)
	suite.Equal("whole_wheat", updatedContext["bread_type"])
	suite.Equal("creating_recipe", updatedContext["progress_step"])
}

// TestConversationWorkflowStates tests different conversation workflow states
func (suite *ConversationFlowTestSuite) TestConversationWorkflowStates() {
	userID := suite.testUsers["chef"].ID
	
	testCases := []struct {
		name           string
		userMessage    string
		expectedIntent conversation.ConversationIntent
		contextCheck   func(conversationID string)
	}{
		{
			name:           "Recipe Creation Intent",
			userMessage:    "Create a recipe for chicken curry",
			expectedIntent: conversation.IntentRecipeCreation,
			contextCheck: func(conversationID string) {
				// Verify recipe creation workflow context
				conv, err := suite.conversationRepo.GetConversation(suite.ctx, conversationID)
				suite.NoError(err)
				suite.Equal(conversation.IntentRecipeCreation, conv.Intent)
				suite.Contains(conv.Title, "Recipe")
			},
		},
		{
			name:           "Cooking Help Intent",
			userMessage:    "How do I properly sear a steak?",
			expectedIntent: conversation.IntentCookingHelp,
			contextCheck: func(conversationID string) {
				conv, err := suite.conversationRepo.GetConversation(suite.ctx, conversationID)
				suite.NoError(err)
				suite.Equal(conversation.IntentCookingHelp, conv.Intent)
				suite.Contains(conv.Title, "Cooking Help")
			},
		},
		{
			name:           "Ingredient Substitution Intent",
			userMessage:    "I don't have heavy cream, what can I substitute?",
			expectedIntent: conversation.IntentIngredientSubst,
			contextCheck: func(conversationID string) {
				conv, err := suite.conversationRepo.GetConversation(suite.ctx, conversationID)
				suite.NoError(err)
				suite.Equal(conversation.IntentIngredientSubst, conv.Intent)
				suite.Contains(conv.Title, "Ingredient Substitution")
			},
		},
		{
			name:           "Meal Planning Intent",
			userMessage:    "Help me plan meals for this week",
			expectedIntent: conversation.IntentMealPlanning,
			contextCheck: func(conversationID string) {
				conv, err := suite.conversationRepo.GetConversation(suite.ctx, conversationID)
				suite.NoError(err)
				suite.Equal(conversation.IntentMealPlanning, conv.Intent)
				suite.Contains(conv.Title, "Meal Planning")
			},
		},
	}
	
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			resp := suite.sendChatMessage(userID, tc.userMessage, "")
			suite.Equal(http.StatusOK, resp.StatusCode)
			
			var chatResp handlers.HTTPChatResponse
			err := json.NewDecoder(resp.Body).Decode(&chatResp)
			suite.NoError(err)
			suite.True(chatResp.Success)
			suite.NotEmpty(chatResp.ConversationID)
			
			tc.contextCheck(chatResp.ConversationID)
		})
	}
}

// TestConversationLifecycleManagement tests conversation lifecycle operations
func (suite *ConversationFlowTestSuite) TestConversationLifecycleManagement() {
	userID := suite.testUsers["chef"].ID
	
	// Create conversation
	resp1 := suite.sendChatMessage(userID, "I want to make soup", "")
	suite.Equal(http.StatusOK, resp1.StatusCode)
	
	var chatResp handlers.HTTPChatResponse
	err := json.NewDecoder(resp1.Body).Decode(&chatResp)
	suite.NoError(err)
	conversationID := chatResp.ConversationID
	
	// Add more messages
	suite.sendChatMessage(userID, "Tomato soup please", conversationID)
	suite.sendChatMessage(userID, "For 6 servings", conversationID)
	
	// Get conversation history
	historyResp := suite.getConversationHistory(userID, conversationID)
	suite.Equal(http.StatusOK, historyResp.StatusCode)
	
	var historyData map[string]interface{}
	err = json.NewDecoder(historyResp.Body).Decode(&historyData)
	suite.NoError(err)
	suite.True(historyData["success"].(bool))
	
	messages := historyData["messages"].([]interface{})
	suite.GreaterOrEqual(len(messages), 6) // At least 3 user + 3 assistant messages
	
	// Archive conversation
	err = suite.convService.ArchiveConversation(suite.ctx, conversationID)
	suite.NoError(err)
	
	// Verify conversation is archived
	conv, err := suite.conversationRepo.GetConversation(suite.ctx, conversationID)
	suite.NoError(err)
	suite.Equal(conversation.StatusArchived, conv.Status)
	
	// Delete conversation
	deleteResp := suite.deleteConversation(userID, conversationID)
	suite.Equal(http.StatusOK, deleteResp.StatusCode)
	
	// Verify conversation is soft deleted
	conv, err = suite.conversationRepo.GetConversation(suite.ctx, conversationID)
	if err == nil {
		suite.Equal(conversation.StatusDeleted, conv.Status)
	} else {
		// Depending on implementation, might be hard deleted or not found
		suite.Contains(err.Error(), "not found")
	}
}

// TestConversationErrorRecovery tests error handling and recovery
func (suite *ConversationFlowTestSuite) TestConversationErrorRecovery() {
	userID := suite.testUsers["chef"].ID
	
	// Test with very long message (edge case)
	longMessage := strings.Repeat("This is a very long message about cooking. ", 100)
	resp1 := suite.sendChatMessage(userID, longMessage, "")
	
	// Should still work (assuming system handles long messages gracefully)
	suite.Equal(http.StatusOK, resp1.StatusCode)
	
	// Test with empty message
	resp2 := suite.sendChatMessage(userID, "", "")
	suite.Equal(http.StatusBadRequest, resp2.StatusCode)
	
	// Test with invalid conversation ID
	resp3 := suite.sendChatMessage(userID, "Test message", "invalid-conversation-id")
	
	// Should either create new conversation or return appropriate error
	suite.True(resp3.StatusCode == http.StatusOK || resp3.StatusCode >= 400)
	
	// Test with special characters and unicode
	unicodeMessage := "I want to make 🍝 pasta with émmental cheese and jalapeños!"
	resp4 := suite.sendChatMessage(userID, unicodeMessage, "")
	suite.Equal(http.StatusOK, resp4.StatusCode)
	
	var chatResp handlers.HTTPChatResponse
	err := json.NewDecoder(resp4.Body).Decode(&chatResp)
	suite.NoError(err)
	suite.True(chatResp.Success)
	
	// Verify unicode content was stored correctly
	messages, err := suite.messageRepo.GetConversationMessages(suite.ctx, chatResp.ConversationID, 100, 0)
	suite.NoError(err)
	suite.Contains(messages[0].Content, "🍝")
	suite.Contains(messages[0].Content, "émmental")
	suite.Contains(messages[0].Content, "jalapeños")
}

// TestConcurrentConversations tests handling of concurrent conversations
func (suite *ConversationFlowTestSuite) TestConcurrentConversations() {
	userID := suite.testUsers["chef"].ID
	const numConcurrentConversations = 5
	
	// Channel to collect conversation IDs
	conversationIDs := make(chan string, numConcurrentConversations)
	errors := make(chan error, numConcurrentConversations)
	
	// Start multiple conversations concurrently
	for i := 0; i < numConcurrentConversations; i++ {
		go func(index int) {
			message := fmt.Sprintf("I want to make recipe number %d", index+1)
			resp := suite.sendChatMessage(userID, message, "")
			
			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("request %d failed with status %d", index, resp.StatusCode)
				return
			}
			
			var chatResp handlers.HTTPChatResponse
			err := json.NewDecoder(resp.Body).Decode(&chatResp)
			if err != nil {
				errors <- fmt.Errorf("request %d decode error: %v", index, err)
				return
			}
			
			if !chatResp.Success {
				errors <- fmt.Errorf("request %d not successful: %s", index, chatResp.Error)
				return
			}
			
			conversationIDs <- chatResp.ConversationID
			errors <- nil
		}(i)
	}
	
	// Collect results
	var collectedIDs []string
	for i := 0; i < numConcurrentConversations; i++ {
		err := <-errors
		suite.NoError(err, "Concurrent conversation %d should succeed", i)
		
		if err == nil {
			convID := <-conversationIDs
			collectedIDs = append(collectedIDs, convID)
		}
	}
	
	// Verify all conversations are unique
	suite.Len(collectedIDs, numConcurrentConversations)
	uniqueIDs := make(map[string]bool)
	for _, id := range collectedIDs {
		suite.False(uniqueIDs[id], "Conversation ID should be unique")
		uniqueIDs[id] = true
	}
	
	// Verify all conversations were persisted
	userConversations, err := suite.conversationRepo.GetUserConversations(suite.ctx, userID, 100, 0)
	suite.NoError(err)
	suite.Len(userConversations, numConcurrentConversations)
}

// Helper methods

// sendChatMessage sends a chat message via HTTP API
func (suite *ConversationFlowTestSuite) sendChatMessage(userID, message, conversationID string) *http.Response {
	reqData := handlers.HTTPChatRequest{
		Message:        message,
		ConversationID: conversationID,
	}
	
	reqBody, _ := json.Marshal(reqData)
	req, _ := http.NewRequest("POST", suite.server.URL+"/api/chat", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", userID)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	
	return resp
}

// getConversationHistory gets conversation history via HTTP API
func (suite *ConversationFlowTestSuite) getConversationHistory(userID, conversationID string) *http.Response {
	url := fmt.Sprintf("%s/api/conversation/history?conversation_id=%s", suite.server.URL, conversationID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-User-ID", userID)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	
	return resp
}

// deleteConversation deletes a conversation via HTTP API
func (suite *ConversationFlowTestSuite) deleteConversation(userID, conversationID string) *http.Response {
	form := fmt.Sprintf("conversation_id=%s", conversationID)
	req, _ := http.NewRequest("POST", suite.server.URL+"/api/conversation/delete", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-User-ID", userID)
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	
	return resp
}

// Run the test suite
func TestConversationFlowSuite(t *testing.T) {
	suite.Run(t, new(ConversationFlowTestSuite))
}

// TestFullStackConversationFlow tests the complete stack from HTTP to database
func TestFullStackConversationFlow(t *testing.T) {
	// This test verifies that all components work together correctly
	// when processing a conversation from start to finish
	
	db := testutils.SetupTestDatabase(t)
	defer testutils.TeardownTestDatabase(t, db)
	
	ctx := context.Background()
	conversationRepo := gorm.NewConversationRepository(db)
	messageRepo := gorm.NewMessageRepository(db)
	contextRepo := gorm.NewContextRepository(db)
	
	// Create mock AI service
	mockOllamaClient := testutils.NewMockOllamaClient()
	mockOpenAIClient := testutils.NewMockOpenAIClient()
	aiService := conversation.NewAIService(mockOllamaClient, mockOpenAIClient)
	
	// Setup mock behaviors
	mockOllamaClient.SetupStandardMockBehavior()
	mockOpenAIClient.SetupStandardMockBehavior()
	
	convService := conversation.NewService(conversationRepo, messageRepo, contextRepo, aiService)
	
	userID := "test-user-id"
	
	t.Run("End-to-End Recipe Creation", func(t *testing.T) {
		// Step 1: Create conversation
		conv, err := convService.CreateConversation(ctx, userID, "I want to make chocolate chip cookies")
		require.NoError(t, err)
		assert.Equal(t, userID, conv.UserID)
		assert.Equal(t, conversation.IntentRecipeCreation, conv.Intent)
		assert.Equal(t, conversation.StatusActive, conv.Status)
		
		// Step 2: Process messages
		messages := []string{
			"I want to make chocolate chip cookies",
			"For 12 cookies please",
			"I prefer crispy cookies",
			"Yes, create the recipe",
		}
		
		for i, msgContent := range messages {
			userMsg, aiResponse, err := convService.ProcessMessage(ctx, conv.ID, msgContent, userID)
			require.NoError(t, err, "Message %d should be processed successfully", i+1)
			assert.Equal(t, msgContent, userMsg.Content)
			assert.NotEmpty(t, aiResponse)
		}
		
		// Step 3: Verify data persistence
		retrievedConv, retrievedMessages, err := convService.GetConversationWithMessages(ctx, conv.ID)
		require.NoError(t, err)
		
		assert.Equal(t, conv.ID, retrievedConv.ID)
		assert.Equal(t, userID, retrievedConv.UserID)
		assert.GreaterOrEqual(t, len(retrievedMessages), 8) // 4 user + 4 AI messages
		
		// Step 4: Verify conversation statistics
		stats, err := convService.GetConversationStats(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, 1, stats["total_conversations"])
		assert.Equal(t, 1, stats["active_conversations"])
		
		// Step 5: Test conversation lifecycle
		err = convService.ArchiveConversation(ctx, conv.ID)
		require.NoError(t, err)
		
		archivedConv, err := conversationRepo.GetConversation(ctx, conv.ID)
		require.NoError(t, err)
		assert.Equal(t, conversation.StatusArchived, archivedConv.Status)
	})
}