package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ChatWorkflowE2ETestSuite tests end-to-end chat workflows from a user perspective
type ChatWorkflowE2ETestSuite struct {
	suite.Suite
	helper    *testutils.WebSocketTestHelper
	baseURL   string
	testUsers map[string]string // username -> userID
	ctx       context.Context
}

// SetupSuite initializes the E2E test suite
func (suite *ChatWorkflowE2ETestSuite) SetupSuite() {
	suite.helper = testutils.NewWebSocketTestHelper()
	suite.baseURL = suite.helper.Server.URL
	suite.ctx = context.Background()
	
	suite.testUsers = map[string]string{
		"chef_maria":     "chef-maria-" + uuid.New().String(),
		"home_cook_john": "home-cook-john-" + uuid.New().String(),
		"beginner_sarah": "beginner-sarah-" + uuid.New().String(),
	}
}

// TearDownSuite cleans up the E2E test suite
func (suite *ChatWorkflowE2ETestSuite) TearDownSuite() {
	suite.helper.Cleanup()
}

// TestNewUserRecipeCreationJourney tests a complete new user recipe creation journey
func (suite *ChatWorkflowE2ETestSuite) TestNewUserRecipeCreationJourney() {
	userID := suite.testUsers["beginner_sarah"]
	
	// Connect to chat
	conn := suite.helper.ConnectWebSocket(suite.T(), userID)
	defer conn.Close()
	
	// Scenario: New user wants to create their first recipe
	suite.Run("First Time Recipe Creation", func() {
		// Step 1: User asks for help creating a recipe
		suite.sendChatMessage(conn, map[string]interface{}{
			"message": "Hi! I'm new to cooking and want to create a simple recipe. Can you help me?",
		})
		
		// Expect conversation creation notification
		convCreatedMsg, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
		suite.NoError(err)
		
		// Expect AI response with guidance
		aiResponse, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(aiResponse.Content), &chatResp)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp.Content), "help")
		suite.Contains(strings.ToLower(chatResp.Content), "recipe")
		
		conversationID := chatResp.ConversationID
		
		// Step 2: User specifies they want to make pasta
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "I want to make something with pasta, something easy",
			"conversation_id": conversationID,
		})
		
		response2, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp2 handlers.ChatResponse
		err = json.Unmarshal([]byte(response2.Content), &chatResp2)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp2.Content), "pasta")
		
		// Step 3: User provides more details
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "Maybe spaghetti aglio e olio? For 2 people",
			"conversation_id": conversationID,
		})
		
		response3, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp3 handlers.ChatResponse
		err = json.Unmarshal([]byte(response3.Content), &chatResp3)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp3.Content), "aglio")
		
		// Step 4: User confirms they want the complete recipe
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "Yes, please give me the complete recipe with step-by-step instructions",
			"conversation_id": conversationID,
		})
		
		finalResponse, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var finalChatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(finalResponse.Content), &finalChatResp)
		suite.NoError(err)
		suite.Contains(strings.ToLower(finalChatResp.Content), "recipe")
		suite.Contains(strings.ToLower(finalChatResp.Content), "step")
	})
}

// TestExperiencedChefComplexRecipe tests an experienced chef creating a complex recipe
func (suite *ChatWorkflowE2ETestSuite) TestExperiencedChefComplexRecipe() {
	userID := suite.testUsers["chef_maria"]
	
	conn := suite.helper.ConnectWebSocket(suite.T(), userID)
	defer conn.Close()
	
	suite.Run("Complex Recipe Creation", func() {
		// Step 1: Chef requests a complex recipe
		suite.sendChatMessage(conn, map[string]interface{}{
			"message": "I want to create a recipe for beef wellington with mushroom duxelles and pâté de foie gras for 8 people",
		})
		
		// Get conversation ID
		convCreated, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
		suite.NoError(err)
		
		aiResponse, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(aiResponse.Content), &chatResp)
		suite.NoError(err)
		conversationID := chatResp.ConversationID
		
		// Step 2: Specify dietary requirements and preferences
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "I want it to be restaurant quality, medium-rare beef, and I can source high-quality ingredients",
			"conversation_id": conversationID,
		})
		
		response2, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		// Step 3: Request specific techniques
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "Include advanced techniques like proper searing and temperature control",
			"conversation_id": conversationID,
		})
		
		response3, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp3 handlers.ChatResponse
		err = json.Unmarshal([]byte(response3.Content), &chatResp3)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp3.Content), "temperature")
		
		// Step 4: Request timing and coordination advice
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "How should I time everything for a dinner party? What can be prepared ahead?",
			"conversation_id": conversationID,
		})
		
		finalResponse, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var finalResp handlers.ChatResponse
		err = json.Unmarshal([]byte(finalResponse.Content), &finalResp)
		suite.NoError(err)
		suite.Contains(strings.ToLower(finalResp.Content), "timing")
	})
}

// TestCookingHelpWorkflow tests getting cooking help and technique advice
func (suite *ChatWorkflowE2ETestSuite) TestCookingHelpWorkflow() {
	userID := suite.testUsers["home_cook_john"]
	
	conn := suite.helper.ConnectWebSocket(suite.T(), userID)
	defer conn.Close()
	
	suite.Run("Cooking Technique Help", func() {
		// Step 1: Ask for cooking technique help
		suite.sendChatMessage(conn, map[string]interface{}{
			"message": "How do I properly sear a steak to get a good crust?",
		})
		
		convCreated, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
		suite.NoError(err)
		
		response1, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(response1.Content), &chatResp)
		suite.NoError(err)
		conversationID := chatResp.ConversationID
		
		suite.Contains(strings.ToLower(chatResp.Content), "sear")
		suite.Contains(strings.ToLower(chatResp.Content), "steak")
		
		// Step 2: Ask follow-up questions
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "What temperature should the pan be? And how long should I sear each side?",
			"conversation_id": conversationID,
		})
		
		response2, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp2 handlers.ChatResponse
		err = json.Unmarshal([]byte(response2.Content), &chatResp2)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp2.Content), "temperature")
		
		// Step 3: Ask about resting the meat
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "Should I let the steak rest after cooking? For how long?",
			"conversation_id": conversationID,
		})
		
		response3, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp3 handlers.ChatResponse
		err = json.Unmarshal([]byte(response3.Content), &chatResp3)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp3.Content), "rest")
	})
}

// TestIngredientSubstitutionWorkflow tests ingredient substitution scenarios
func (suite *ChatWorkflowE2ETestSuite) TestIngredientSubstitutionWorkflow() {
	userID := suite.testUsers["home_cook_john"]
	
	conn := suite.helper.ConnectWebSocket(suite.T(), userID)
	defer conn.Close()
	
	suite.Run("Emergency Substitution", func() {
		// Step 1: Urgent substitution need
		suite.sendChatMessage(conn, map[string]interface{}{
			"message": "I'm in the middle of making a cake and I just realized I'm out of eggs! What can I substitute?",
		})
		
		convCreated, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
		suite.NoError(err)
		
		response1, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(response1.Content), &chatResp)
		suite.NoError(err)
		conversationID := chatResp.ConversationID
		
		suite.Contains(strings.ToLower(chatResp.Content), "egg")
		suite.Contains(strings.ToLower(chatResp.Content), "substitute")
		
		// Step 2: Specify what type of cake
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "It's a chocolate cake, and I need 2 eggs. What are my options?",
			"conversation_id": conversationID,
		})
		
		response2, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp2 handlers.ChatResponse
		err = json.Unmarshal([]byte(response2.Content), &chatResp2)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp2.Content), "chocolate")
		
		// Step 3: Choose a specific substitution
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "I have applesauce and flax seeds. Which would work better?",
			"conversation_id": conversationID,
		})
		
		response3, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp3 handlers.ChatResponse
		err = json.Unmarshal([]byte(response3.Content), &chatResp3)
		suite.NoError(err)
		suite.True(strings.Contains(strings.ToLower(chatResp3.Content), "applesauce") ||
				   strings.Contains(strings.ToLower(chatResp3.Content), "flax"))
	})
}

// TestMealPlanningWorkflow tests meal planning for different scenarios
func (suite *ChatWorkflowE2ETestSuite) TestMealPlanningWorkflow() {
	userID := suite.testUsers["beginner_sarah"]
	
	conn := suite.helper.ConnectWebSocket(suite.T(), userID)
	defer conn.Close()
	
	suite.Run("Weekly Meal Planning", func() {
		// Step 1: Request weekly meal planning
		suite.sendChatMessage(conn, map[string]interface{}{
			"message": "I need help planning meals for next week. I want healthy options for 2 people on a budget",
		})
		
		convCreated, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
		suite.NoError(err)
		
		response1, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(response1.Content), &chatResp)
		suite.NoError(err)
		conversationID := chatResp.ConversationID
		
		suite.Contains(strings.ToLower(chatResp.Content), "meal")
		suite.Contains(strings.ToLower(chatResp.Content), "week")
		
		// Step 2: Specify dietary preferences and restrictions
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "We eat everything but prefer less red meat. We like pasta, chicken, and lots of vegetables",
			"conversation_id": conversationID,
		})
		
		response2, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		// Step 3: Specify time constraints
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "We're both busy with work, so quick weeknight meals would be great, maybe 30 minutes or less",
			"conversation_id": conversationID,
		})
		
		response3, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp3 handlers.ChatResponse
		err = json.Unmarshal([]byte(response3.Content), &chatResp3)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp3.Content), "30")
		
		// Step 4: Request shopping list
		suite.sendChatMessage(conn, map[string]interface{}{
			"message":         "Can you also give me a shopping list for these meals?",
			"conversation_id": conversationID,
		})
		
		response4, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp4 handlers.ChatResponse
		err = json.Unmarshal([]byte(response4.Content), &chatResp4)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp4.Content), "shopping")
	})
}

// TestMultiDeviceConversationContinuity tests conversation continuity across devices
func (suite *ChatWorkflowE2ETestSuite) TestMultiDeviceConversationContinuity() {
	userID := suite.testUsers["chef_maria"]
	
	suite.Run("Cross-Device Conversation", func() {
		// Device 1: Start conversation on mobile (WebSocket)
		mobileConn := suite.helper.ConnectWebSocket(suite.T(), userID)
		defer mobileConn.Close()
		
		suite.sendChatMessage(mobileConn, map[string]interface{}{
			"message": "I want to start planning a dinner party menu",
		})
		
		convCreated, err := mobileConn.WaitForMessageType("conversation_created", 5*time.Second)
		suite.NoError(err)
		
		response1, err := mobileConn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(response1.Content), &chatResp)
		suite.NoError(err)
		conversationID := chatResp.ConversationID
		
		// Device 2: Continue conversation on desktop (HTTP API)
		httpResp := suite.sendHTTPChatMessage(userID, "I need appetizers, main course, and dessert for 8 people", conversationID)
		suite.Equal(http.StatusOK, httpResp.StatusCode)
		
		var httpChatResp handlers.HTTPChatResponse
		err = json.NewDecoder(httpResp.Body).Decode(&httpChatResp)
		suite.NoError(err)
		suite.True(httpChatResp.Success)
		suite.Equal(conversationID, httpChatResp.ConversationID)
		
		// Device 1: Continue on mobile and see the context is maintained
		suite.sendChatMessage(mobileConn, map[string]interface{}{
			"message":         "What appetizers would you recommend?",
			"conversation_id": conversationID,
		})
		
		response3, err := mobileConn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp3 handlers.ChatResponse
		err = json.Unmarshal([]byte(response3.Content), &chatResp3)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp3.Content), "appetizer")
	})
}

// TestConversationErrorHandlingAndRecovery tests how the system handles errors gracefully
func (suite *ChatWorkflowE2ETestSuite) TestConversationErrorHandlingAndRecovery() {
	userID := suite.testUsers["home_cook_john"]
	
	conn := suite.helper.ConnectWebSocket(suite.T(), userID)
	defer conn.Close()
	
	suite.Run("Error Recovery", func() {
		// Test 1: Send malformed JSON
		malformedMsg := websocket.Message{
			ID:        uuid.New().String(),
			Type:      "chat_message",
			Content:   `{"malformed": json}`,
			Timestamp: time.Now(),
		}
		
		err := conn.SendMessage(malformedMsg)
		suite.NoError(err)
		
		// Should receive error response
		errorMsg, err := conn.WaitForMessageType("error", 5*time.Second)
		suite.NoError(err)
		suite.Contains(errorMsg.Content, "Invalid")
		
		// Test 2: Send empty message (should still work and create conversation)
		suite.sendChatMessage(conn, map[string]interface{}{
			"message": "",
		})
		
		// May get error or create conversation - either is acceptable
		response, err := conn.WaitForMessage(5 * time.Second)
		suite.NoError(err)
		suite.True(response.Type == "error" || response.Type == "conversation_created")
		
		// Test 3: Continue with valid message after errors
		suite.sendChatMessage(conn, map[string]interface{}{
			"message": "How do I make scrambled eggs?",
		})
		
		// Should work normally
		if response.Type != "conversation_created" {
			convCreated, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
			suite.NoError(err)
			suite.NotEmpty(convCreated)
		}
		
		aiResponse, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(aiResponse.Content), &chatResp)
		suite.NoError(err)
		suite.Contains(strings.ToLower(chatResp.Content), "egg")
	})
}

// TestHighVolumeConversations tests system behavior under high conversation volume
func (suite *ChatWorkflowE2ETestSuite) TestHighVolumeConversations() {
	suite.Run("Multiple Concurrent Users", func() {
		const numUsers = 5
		const messagesPerUser = 3
		
		var connections []*testutils.TestWebSocketConnection
		defer func() {
			for _, conn := range connections {
				conn.Close()
			}
		}()
		
		// Connect multiple users
		for i := 0; i < numUsers; i++ {
			userID := fmt.Sprintf("load-test-user-%d-%s", i, uuid.New().String())
			conn := suite.helper.ConnectWebSocket(suite.T(), userID)
			connections = append(connections, conn)
		}
		
		// Each user starts a conversation
		conversationIDs := make([]string, numUsers)
		for i, conn := range connections {
			suite.sendChatMessage(conn, map[string]interface{}{
				"message": fmt.Sprintf("I want to make recipe number %d", i+1),
			})
			
			convCreated, err := conn.WaitForMessageType("conversation_created", 10*time.Second)
			suite.NoError(err, "User %d should create conversation", i)
			
			aiResponse, err := conn.WaitForMessageType("chat_response", 10*time.Second)
			suite.NoError(err, "User %d should get AI response", i)
			
			var chatResp handlers.ChatResponse
			err = json.Unmarshal([]byte(aiResponse.Content), &chatResp)
			suite.NoError(err)
			conversationIDs[i] = chatResp.ConversationID
		}
		
		// Each user sends multiple follow-up messages
		for msgNum := 0; msgNum < messagesPerUser; msgNum++ {
			for i, conn := range connections {
				suite.sendChatMessage(conn, map[string]interface{}{
					"message":         fmt.Sprintf("Follow-up message %d for recipe %d", msgNum+1, i+1),
					"conversation_id": conversationIDs[i],
				})
				
				_, err := conn.WaitForMessageType("chat_response", 10*time.Second)
				suite.NoError(err, "User %d message %d should get response", i, msgNum+1)
			}
		}
	})
}

// TestConversationPersistenceAndRetrieval tests conversation history persistence
func (suite *ChatWorkflowE2ETestSuite) TestConversationPersistenceAndRetrieval() {
	userID := suite.testUsers["chef_maria"]
	
	suite.Run("Conversation History", func() {
		// Create a conversation with multiple messages
		conn := suite.helper.ConnectWebSocket(suite.T(), userID)
		defer conn.Close()
		
		// Start conversation
		suite.sendChatMessage(conn, map[string]interface{}{
			"message": "I want to create a lasagna recipe",
		})
		
		convCreated, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
		suite.NoError(err)
		
		response1, err := conn.WaitForMessageType("chat_response", 5*time.Second)
		suite.NoError(err)
		
		var chatResp handlers.ChatResponse
		err = json.Unmarshal([]byte(response1.Content), &chatResp)
		suite.NoError(err)
		conversationID := chatResp.ConversationID
		
		// Add several more messages
		messages := []string{
			"Meat lasagna with bechamel sauce",
			"For 6 people",
			"I want it to be authentic Italian style",
			"Yes, create the complete recipe",
		}
		
		for _, msg := range messages {
			suite.sendChatMessage(conn, map[string]interface{}{
				"message":         msg,
				"conversation_id": conversationID,
			})
			
			_, err := conn.WaitForMessageType("chat_response", 5*time.Second)
			suite.NoError(err)
		}
		
		// Test conversation list retrieval via HTTP
		listResp := suite.getConversationList(userID)
		suite.Equal(http.StatusOK, listResp.StatusCode)
		
		var listResult map[string]interface{}
		err = json.NewDecoder(listResp.Body).Decode(&listResult)
		suite.NoError(err)
		suite.True(listResult["success"].(bool))
		
		conversations := listResult["conversations"].([]interface{})
		suite.Len(conversations, 1)
		
		// Test conversation history retrieval via HTTP
		historyResp := suite.getConversationHistory(userID, conversationID)
		suite.Equal(http.StatusOK, historyResp.StatusCode)
		
		var historyResult map[string]interface{}
		err = json.NewDecoder(historyResp.Body).Decode(&historyResult)
		suite.NoError(err)
		suite.True(historyResult["success"].(bool))
		
		historyMessages := historyResult["messages"].([]interface{})
		suite.GreaterOrEqual(len(historyMessages), 10) // At least 5 user + 5 AI messages
		
		// Verify message order (should be chronological)
		firstMsg := historyMessages[0].(map[string]interface{})
		suite.Contains(strings.ToLower(firstMsg["content"].(string)), "lasagna")
	})
}

// Helper methods

// sendChatMessage sends a chat message through WebSocket
func (suite *ChatWorkflowE2ETestSuite) sendChatMessage(conn *testutils.TestWebSocketConnection, chatData map[string]interface{}) {
	chatJSON, _ := json.Marshal(chatData)
	
	wsMessage := websocket.Message{
		ID:        uuid.New().String(),
		Type:      "chat_message",
		Content:   string(chatJSON),
		Timestamp: time.Now(),
	}
	
	err := conn.SendMessage(wsMessage)
	suite.NoError(err)
}

// sendHTTPChatMessage sends a chat message through HTTP API
func (suite *ChatWorkflowE2ETestSuite) sendHTTPChatMessage(userID, message, conversationID string) *http.Response {
	reqData := map[string]string{
		"message":         message,
		"conversation_id": conversationID,
	}
	
	reqBody, _ := json.Marshal(reqData)
	
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", suite.baseURL+"/api/chat", strings.NewReader(string(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", userID)
	
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	
	return resp
}

// getConversationList gets user's conversation list via HTTP API
func (suite *ChatWorkflowE2ETestSuite) getConversationList(userID string) *http.Response {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", suite.baseURL+"/api/conversations", nil)
	req.Header.Set("X-User-ID", userID)
	
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	
	return resp
}

// getConversationHistory gets conversation history via HTTP API
func (suite *ChatWorkflowE2ETestSuite) getConversationHistory(userID, conversationID string) *http.Response {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/api/conversation/history?conversation_id=%s", suite.baseURL, conversationID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-User-ID", userID)
	
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	
	return resp
}

// getConversationStats gets conversation statistics via HTTP API
func (suite *ChatWorkflowE2ETestSuite) getConversationStats(userID string) *http.Response {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", suite.baseURL+"/api/conversation/stats", nil)
	req.Header.Set("X-User-ID", userID)
	
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	
	return resp
}

// deleteConversation deletes a conversation via HTTP API
func (suite *ChatWorkflowE2ETestSuite) deleteConversation(userID, conversationID string) *http.Response {
	client := &http.Client{Timeout: 10 * time.Second}
	
	form := url.Values{}
	form.Set("conversation_id", conversationID)
	
	req, _ := http.NewRequest("POST", suite.baseURL+"/api/conversation/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-User-ID", userID)
	
	resp, err := client.Do(req)
	suite.Require().NoError(err)
	
	return resp
}

// Run the test suite
func TestChatWorkflowE2ESuite(t *testing.T) {
	suite.Run(t, new(ChatWorkflowE2ETestSuite))
}

// TestRealWorldScenarios tests realistic user scenarios
func TestRealWorldScenarios(t *testing.T) {
	helper := testutils.NewWebSocketTestHelper()
	defer helper.Cleanup()
	
	t.Run("Family Dinner Planning", func(t *testing.T) {
		userID := "family-planner-" + uuid.New().String()
		conn := helper.ConnectWebSocket(t, userID)
		defer conn.Close()
		
		// Realistic family dinner planning conversation
		conversation := []map[string]interface{}{
			{"message": "Hi! I need help planning dinner for my family of 4 tonight. Two adults and two kids (ages 6 and 9)"},
			{"message": "The kids are picky eaters - they like chicken and pasta but not too many vegetables mixed in"},
			{"message": "I have about 45 minutes to cook. What would you recommend?"},
			{"message": "That sounds perfect! Can you give me the recipe and cooking timeline?"},
		}
		
		var conversationID string
		for i, msgData := range conversation {
			chatJSON, _ := json.Marshal(msgData)
			wsMessage := websocket.Message{
				ID:        uuid.New().String(),
				Type:      "chat_message",
				Content:   string(chatJSON),
				Timestamp: time.Now(),
			}
			
			if conversationID != "" {
				var msgWithConv map[string]interface{}
				json.Unmarshal([]byte(wsMessage.Content), &msgWithConv)
				msgWithConv["conversation_id"] = conversationID
				newJSON, _ := json.Marshal(msgWithConv)
				wsMessage.Content = string(newJSON)
			}
			
			err := conn.SendMessage(wsMessage)
			require.NoError(t, err)
			
			if i == 0 {
				convCreated, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
				require.NoError(t, err)
				
				aiResponse, err := conn.WaitForMessageType("chat_response", 5*time.Second)
				require.NoError(t, err)
				
				var chatResp handlers.ChatResponse
				err = json.Unmarshal([]byte(aiResponse.Content), &chatResp)
				require.NoError(t, err)
				conversationID = chatResp.ConversationID
			} else {
				_, err := conn.WaitForMessageType("chat_response", 5*time.Second)
				require.NoError(t, err)
			}
		}
	})
	
	t.Run("Dietary Restriction Recipe Request", func(t *testing.T) {
		userID := "dietary-user-" + uuid.New().String()
		conn := helper.ConnectWebSocket(t, userID)
		defer conn.Close()
		
		// User with multiple dietary restrictions
		conversation := []map[string]interface{}{
			{"message": "I need a recipe that's gluten-free, dairy-free, and vegan for a birthday party"},
			{"message": "It's for 12 people and I want something that feels special, like a dessert"},
			{"message": "I have access to alternative flours and plant-based ingredients"},
			{"message": "A chocolate cake sounds amazing! Can you create a recipe?"},
		}
		
		var conversationID string
		for i, msgData := range conversation {
			if conversationID != "" {
				msgData["conversation_id"] = conversationID
			}
			
			chatJSON, _ := json.Marshal(msgData)
			wsMessage := websocket.Message{
				ID:        uuid.New().String(),
				Type:      "chat_message",
				Content:   string(chatJSON),
				Timestamp: time.Now(),
			}
			
			err := conn.SendMessage(wsMessage)
			require.NoError(t, err)
			
			if i == 0 {
				_, err := conn.WaitForMessageType("conversation_created", 5*time.Second)
				require.NoError(t, err)
				
				aiResponse, err := conn.WaitForMessageType("chat_response", 5*time.Second)
				require.NoError(t, err)
				
				var chatResp handlers.ChatResponse
				err = json.Unmarshal([]byte(aiResponse.Content), &chatResp)
				require.NoError(t, err)
				conversationID = chatResp.ConversationID
				
				// Verify AI understood the dietary restrictions
				assert.Contains(t, strings.ToLower(chatResp.Content), "gluten")
			} else {
				_, err := conn.WaitForMessageType("chat_response", 5*time.Second)
				require.NoError(t, err)
			}
		}
	})
}