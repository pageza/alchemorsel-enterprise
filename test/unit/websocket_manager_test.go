package unit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/websocket"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// WebSocketManagerTestSuite tests the WebSocket manager
type WebSocketManagerTestSuite struct {
	suite.Suite
	manager    *websocket.Manager
	server     *httptest.Server
	ctx        context.Context
	cancel     context.CancelFunc
	wsUpgrader websocket.Upgrader
}

// SetupSuite initializes the test suite
func (suite *WebSocketManagerTestSuite) SetupSuite() {
	suite.manager = websocket.NewManager()
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	
	// Start the manager
	go suite.manager.Start(suite.ctx)
	
	// Create test server
	suite.server = httptest.NewServer(http.HandlerFunc(suite.handleWebSocketUpgrade))
	
	suite.wsUpgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
}

// TearDownSuite cleans up the test suite
func (suite *WebSocketManagerTestSuite) TearDownSuite() {
	suite.cancel()
	suite.server.Close()
}

// handleWebSocketUpgrade handles WebSocket upgrade for testing
func (suite *WebSocketManagerTestSuite) handleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "test-user-" + uuid.New().String()
	}
	
	err := suite.manager.HandleUpgrade(w, r, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// TestWebSocketConnection tests basic WebSocket connection establishment
func (suite *WebSocketManagerTestSuite) TestWebSocketConnection() {
	userID := "test-user-" + uuid.New().String()
	
	// Connect to WebSocket
	conn := suite.connectWebSocket(userID)
	defer conn.Close()
	
	// Wait for connection established message
	message, err := suite.readMessage(conn, 5*time.Second)
	suite.NoError(err)
	suite.Equal("connection_established", message.Type)
	suite.Equal("Connection established successfully", message.Content)
	
	// Verify connection count
	suite.Equal(1, suite.manager.GetConnectionCount())
}

// TestMultipleConnections tests multiple WebSocket connections
func (suite *WebSocketManagerTestSuite) TestMultipleConnections() {
	userIDs := []string{
		"test-user-1-" + uuid.New().String(),
		"test-user-2-" + uuid.New().String(),
		"test-user-3-" + uuid.New().String(),
	}
	
	var connections []*websocket.Conn
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()
	
	// Connect multiple users
	for _, userID := range userIDs {
		conn := suite.connectWebSocket(userID)
		connections = append(connections, conn)
		
		// Wait for connection established message
		message, err := suite.readMessage(conn, 5*time.Second)
		suite.NoError(err)
		suite.Equal("connection_established", message.Type)
	}
	
	// Verify connection count
	suite.Equal(3, suite.manager.GetConnectionCount())
	
	// Test getting user connections
	userConnections := suite.manager.GetUserConnections(userIDs[0])
	suite.Len(userConnections, 1)
	suite.Equal(userIDs[0], userConnections[0].UserID)
}

// TestSameUserMultipleConnections tests multiple connections from same user
func (suite *WebSocketManagerTestSuite) TestSameUserMultipleConnections() {
	userID := "test-user-" + uuid.New().String()
	
	var connections []*websocket.Conn
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()
	
	// Connect same user multiple times (multiple tabs scenario)
	for i := 0; i < 3; i++ {
		conn := suite.connectWebSocket(userID)
		connections = append(connections, conn)
		
		// Wait for connection established message
		message, err := suite.readMessage(conn, 5*time.Second)
		suite.NoError(err)
		suite.Equal("connection_established", message.Type)
	}
	
	// Verify total connection count
	suite.Equal(3, suite.manager.GetConnectionCount())
	
	// Verify user has 3 connections
	userConnections := suite.manager.GetUserConnections(userID)
	suite.Len(userConnections, 3)
}

// TestMessageSending tests sending messages to specific users and connections
func (suite *WebSocketManagerTestSuite) TestMessageSending() {
	userID := "test-user-" + uuid.New().String()
	
	// Connect user
	conn := suite.connectWebSocket(userID)
	defer conn.Close()
	
	// Wait for connection established message
	_, err := suite.readMessage(conn, 5*time.Second)
	suite.NoError(err)
	
	// Send message to user
	testMessage := websocket.Message{
		ID:        uuid.New().String(),
		Type:      "test_message",
		Content:   "Hello, World!",
		Timestamp: time.Now(),
	}
	
	err = suite.manager.SendToUser(userID, testMessage)
	suite.NoError(err)
	
	// Read the message
	receivedMessage, err := suite.readMessage(conn, 5*time.Second)
	suite.NoError(err)
	suite.Equal(testMessage.Type, receivedMessage.Type)
	suite.Equal(testMessage.Content, receivedMessage.Content)
}

// TestPingPongHeartbeat tests ping/pong heartbeat mechanism
func (suite *WebSocketManagerTestSuite) TestPingPongHeartbeat() {
	userID := "test-user-" + uuid.New().String()
	
	// Connect user
	conn := suite.connectWebSocket(userID)
	defer conn.Close()
	
	// Wait for connection established message
	_, err := suite.readMessage(conn, 5*time.Second)
	suite.NoError(err)
	
	// Send ping message
	pingMessage := websocket.Message{
		ID:        uuid.New().String(),
		Type:      "ping",
		Timestamp: time.Now(),
	}
	
	err = conn.WriteJSON(pingMessage)
	suite.NoError(err)
	
	// Should receive pong response
	pongMessage, err := suite.readMessage(conn, 5*time.Second)
	suite.NoError(err)
	suite.Equal("pong", pongMessage.Type)
	suite.Equal(pingMessage.ID, pongMessage.ID)
}

// TestConnectionCleanup tests automatic cleanup of stale connections
func (suite *WebSocketManagerTestSuite) TestConnectionCleanup() {
	userID := "test-user-" + uuid.New().String()
	
	// Connect user
	conn := suite.connectWebSocket(userID)
	
	// Wait for connection established message
	_, err := suite.readMessage(conn, 5*time.Second)
	suite.NoError(err)
	
	// Verify connection exists
	suite.Equal(1, suite.manager.GetConnectionCount())
	
	// Close connection abruptly
	conn.Close()
	
	// Wait a bit for cleanup to happen
	time.Sleep(100 * time.Millisecond)
	
	// Connection count should eventually become 0 after cleanup
	suite.Eventually(func() bool {
		return suite.manager.GetConnectionCount() == 0
	}, 10*time.Second, 100*time.Millisecond)
}

// TestConcurrentConnections tests concurrent connection handling
func (suite *WebSocketManagerTestSuite) TestConcurrentConnections() {
	const numConnections = 50
	const connectionsPerUser = 5
	
	var wg sync.WaitGroup
	var connections []struct {
		conn   *websocket.Conn
		userID string
	}
	var mu sync.Mutex
	
	// Connect multiple users concurrently
	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			userID := fmt.Sprintf("concurrent-user-%d", index/connectionsPerUser)
			conn := suite.connectWebSocket(userID)
			
			// Wait for connection established
			_, err := suite.readMessage(conn, 10*time.Second)
			suite.NoError(err)
			
			mu.Lock()
			connections = append(connections, struct {
				conn   *websocket.Conn
				userID string
			}{conn, userID})
			mu.Unlock()
		}(i)
	}
	
	wg.Wait()
	
	// Verify all connections were established
	suite.Equal(numConnections, len(connections))
	suite.Equal(numConnections, suite.manager.GetConnectionCount())
	
	// Clean up connections
	for _, connInfo := range connections {
		connInfo.conn.Close()
	}
	
	// Wait for cleanup
	suite.Eventually(func() bool {
		return suite.manager.GetConnectionCount() == 0
	}, 10*time.Second, 100*time.Millisecond)
}

// TestMessageBroadcast tests broadcasting messages to all connections
func (suite *WebSocketManagerTestSuite) TestMessageBroadcast() {
	userIDs := []string{
		"broadcast-user-1-" + uuid.New().String(),
		"broadcast-user-2-" + uuid.New().String(),
		"broadcast-user-3-" + uuid.New().String(),
	}
	
	var connections []*websocket.Conn
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()
	
	// Connect multiple users
	for _, userID := range userIDs {
		conn := suite.connectWebSocket(userID)
		connections = append(connections, conn)
		
		// Wait for connection established message
		_, err := suite.readMessage(conn, 5*time.Second)
		suite.NoError(err)
	}
	
	// Broadcast message
	broadcastMessage := websocket.Message{
		ID:        uuid.New().String(),
		Type:      "broadcast",
		Content:   "This is a broadcast message",
		Timestamp: time.Now(),
	}
	
	// Note: Since we don't have a direct broadcast method exposed,
	// we'll test by sending to each user individually
	for _, userID := range userIDs {
		err := suite.manager.SendToUser(userID, broadcastMessage)
		suite.NoError(err)
	}
	
	// Verify all connections received the message
	for i, conn := range connections {
		receivedMessage, err := suite.readMessage(conn, 5*time.Second)
		suite.NoError(err, "Connection %d should receive broadcast message", i)
		suite.Equal(broadcastMessage.Type, receivedMessage.Type)
		suite.Equal(broadcastMessage.Content, receivedMessage.Content)
	}
}

// TestErrorHandling tests error handling scenarios
func (suite *WebSocketManagerTestSuite) TestErrorHandling() {
	// Test sending to non-existent user
	nonExistentUserID := "non-existent-user-" + uuid.New().String()
	testMessage := websocket.Message{
		ID:        uuid.New().String(),
		Type:      "test",
		Content:   "test",
		Timestamp: time.Now(),
	}
	
	err := suite.manager.SendToUser(nonExistentUserID, testMessage)
	suite.Error(err)
	suite.Equal(websocket.ErrUserNotConnected, err)
	
	// Test sending to non-existent connection
	nonExistentConnID := "non-existent-conn-" + uuid.New().String()
	err = suite.manager.SendToConnection(nonExistentConnID, testMessage)
	suite.Error(err)
	suite.Equal(websocket.ErrConnectionNotFound, err)
}

// TestWebSocketProtocolCompliance tests WebSocket protocol compliance
func (suite *WebSocketManagerTestSuite) TestWebSocketProtocolCompliance() {
	userID := "protocol-test-user-" + uuid.New().String()
	
	// Connect user
	conn := suite.connectWebSocket(userID)
	defer conn.Close()
	
	// Wait for connection established message
	_, err := suite.readMessage(conn, 5*time.Second)
	suite.NoError(err)
	
	// Test different message types
	testCases := []struct {
		name    string
		message websocket.Message
	}{
		{
			name: "Text Message",
			message: websocket.Message{
				ID:        uuid.New().String(),
				Type:      "text",
				Content:   "Simple text message",
				Timestamp: time.Now(),
			},
		},
		{
			name: "JSON Message",
			message: websocket.Message{
				ID:   uuid.New().String(),
				Type: "json",
				Content: func() string {
					data := map[string]interface{}{
						"key1": "value1",
						"key2": 123,
						"key3": []string{"a", "b", "c"},
					}
					bytes, _ := json.Marshal(data)
					return string(bytes)
				}(),
				Timestamp: time.Now(),
			},
		},
		{
			name: "Empty Content",
			message: websocket.Message{
				ID:        uuid.New().String(),
				Type:      "empty",
				Content:   "",
				Timestamp: time.Now(),
			},
		},
	}
	
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			err := suite.manager.SendToUser(userID, tc.message)
			suite.NoError(err)
			
			receivedMessage, err := suite.readMessage(conn, 5*time.Second)
			suite.NoError(err)
			suite.Equal(tc.message.Type, receivedMessage.Type)
			suite.Equal(tc.message.Content, receivedMessage.Content)
		})
	}
}

// TestWebSocketSecurity tests security aspects of WebSocket connections
func (suite *WebSocketManagerTestSuite) TestWebSocketSecurity() {
	// Test connection limit per user (if implemented)
	userID := "security-test-user-" + uuid.New().String()
	
	var connections []*websocket.Conn
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()
	
	// Connect multiple times as same user
	maxConnections := 10 // Reasonable limit for testing
	for i := 0; i < maxConnections; i++ {
		conn := suite.connectWebSocket(userID)
		connections = append(connections, conn)
		
		// Wait for connection established message
		_, err := suite.readMessage(conn, 5*time.Second)
		suite.NoError(err)
	}
	
	// Verify all connections are tracked
	userConnections := suite.manager.GetUserConnections(userID)
	suite.Len(userConnections, maxConnections)
}

// Helper methods

// connectWebSocket establishes a WebSocket connection
func (suite *WebSocketManagerTestSuite) connectWebSocket(userID string) *websocket.Conn {
	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + suite.server.URL[4:] + "?user_id=" + url.QueryEscape(userID)
	
	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	suite.Require().NoError(err)
	
	return conn
}

// readMessage reads a message from WebSocket connection with timeout
func (suite *WebSocketManagerTestSuite) readMessage(conn *websocket.Conn, timeout time.Duration) (websocket.Message, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	
	var message websocket.Message
	err := conn.ReadJSON(&message)
	return message, err
}

// Run the test suite
func TestWebSocketManagerSuite(t *testing.T) {
	suite.Run(t, new(WebSocketManagerTestSuite))
}

// TestWebSocketManagerWithRealScenarios tests real-world scenarios
func TestWebSocketManagerWithRealScenarios(t *testing.T) {
	manager := websocket.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start the manager
	go manager.Start(ctx)
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "default-user"
		}
		
		err := manager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	
	// Test chat-like scenario
	t.Run("Chat Scenario", func(t *testing.T) {
		helper := testutils.NewWebSocketTestHelper()
		defer helper.Cleanup()
		
		// Connect two users
		user1 := "chat-user-1"
		user2 := "chat-user-2"
		
		conn1 := helper.ConnectWebSocket(t, user1)
		conn2 := helper.ConnectWebSocket(t, user2)
		
		defer conn1.Close()
		defer conn2.Close()
		
		// Simulate chat messages
		chatMessages := []struct {
			from    string
			to      string
			content string
		}{
			{user1, user2, "Hello, how are you?"},
			{user2, user1, "I'm doing great! How about you?"},
			{user1, user2, "Excellent! Want to cook something together?"},
		}
		
		for _, msg := range chatMessages {
			chatMsg := websocket.Message{
				ID:        uuid.New().String(),
				Type:      "chat_message",
				Content:   msg.content,
				Timestamp: time.Now(),
			}
			
			// Send message
			err := manager.SendToUser(msg.to, chatMsg)
			require.NoError(t, err)
			
			// Verify receipt
			var targetConn *testutils.TestWebSocketConnection
			if msg.to == user1 {
				targetConn = conn1
			} else {
				targetConn = conn2
			}
			
			receivedMsg, err := targetConn.WaitForMessageType("chat_message", 5*time.Second)
			require.NoError(t, err)
			assert.Equal(t, msg.content, receivedMsg.Content)
		}
	})
	
	// Test recipe creation scenario
	t.Run("Recipe Creation Scenario", func(t *testing.T) {
		helper := testutils.NewWebSocketTestHelper()
		defer helper.Cleanup()
		
		userID := "recipe-creator"
		conn := helper.ConnectWebSocket(t, userID)
		defer conn.Close()
		
		// Simulate recipe creation flow
		recipeSteps := []string{
			"I want to create a pasta recipe",
			"What type of pasta?",
			"Spaghetti carbonara please",
			"How many servings?",
			"4 servings",
			"Here's your carbonara recipe...",
		}
		
		for i, step := range recipeSteps {
			msgType := "recipe_step"
			if i%2 == 0 {
				msgType = "user_message"
			} else {
				msgType = "ai_response"
			}
			
			stepMsg := websocket.Message{
				ID:        uuid.New().String(),
				Type:      msgType,
				Content:   step,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"step":      i + 1,
					"intent":    "recipe_creation",
					"progress":  float64(i+1) / float64(len(recipeSteps)),
				},
			}
			
			err := manager.SendToUser(userID, stepMsg)
			require.NoError(t, err)
			
			receivedMsg, err := conn.WaitForMessage(5 * time.Second)
			require.NoError(t, err)
			assert.Equal(t, step, receivedMsg.Content)
			assert.Equal(t, msgType, receivedMsg.Type)
		}
	})
	
	// Test connection resilience
	t.Run("Connection Resilience", func(t *testing.T) {
		helper := testutils.NewWebSocketTestHelper()
		defer helper.Cleanup()
		
		userID := "resilient-user"
		
		// Connect and disconnect multiple times
		for i := 0; i < 5; i++ {
			conn := helper.ConnectWebSocket(t, userID)
			
			// Send a message
			testMsg := websocket.Message{
				ID:        uuid.New().String(),
				Type:      "test",
				Content:   fmt.Sprintf("Test message %d", i+1),
				Timestamp: time.Now(),
			}
			
			err := manager.SendToUser(userID, testMsg)
			require.NoError(t, err)
			
			receivedMsg, err := conn.WaitForMessage(5 * time.Second)
			require.NoError(t, err)
			assert.Equal(t, testMsg.Content, receivedMsg.Content)
			
			// Close connection
			conn.Close()
			
			// Small delay between connections
			time.Sleep(100 * time.Millisecond)
		}
	})
}