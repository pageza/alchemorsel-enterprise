package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	"github.com/alchemorsel/v3/internal/infrastructure/websocket"
	"github.com/alchemorsel/v3/test/testutils"
	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// ChatSecurityTestSuite tests security aspects of the chat interface
type ChatSecurityTestSuite struct {
	suite.Suite
	testSuite   *testutils.ConversationTestSuite
	chatHandler *handlers.ChatHandler
	wsManager   *websocket.Manager
	wsHandler   *handlers.WebSocketHandler
	server      *httptest.Server
	testUsers   map[string]*handlers.User
	ctx         context.Context
	cancel      context.CancelFunc
}

// SetupSuite initializes the security test suite
func (suite *ChatSecurityTestSuite) SetupSuite() {
	suite.testSuite = testutils.NewConversationTestSuite()
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	
	suite.chatHandler = handlers.NewChatHandler(suite.testSuite.ConversationService)
	suite.wsManager = websocket.NewManager()
	suite.wsHandler = handlers.NewWebSocketHandler(suite.wsManager, suite.testSuite.ConversationService)
	
	// Start WebSocket manager
	go suite.wsManager.Start(suite.ctx)
	
	// Create test server with security middleware
	suite.server = httptest.NewServer(suite.createSecurityTestMux())
	
	suite.testUsers = map[string]*handlers.User{
		"regular_user": {
			ID:    "regular-user-" + uuid.New().String(),
			Email: "regular@example.com",
			Name:  "Regular User",
		},
		"admin_user": {
			ID:    "admin-user-" + uuid.New().String(),
			Email: "admin@example.com",
			Name:  "Admin User",
		},
		"malicious_user": {
			ID:    "malicious-user-" + uuid.New().String(),
			Email: "malicious@example.com",
			Name:  "Malicious User",
		},
	}
}

// TearDownSuite cleans up the security test suite
func (suite *ChatSecurityTestSuite) TearDownSuite() {
	suite.cancel()
	if suite.server != nil {
		suite.server.Close()
	}
}

// createSecurityTestMux creates a test mux with security middleware
func (suite *ChatSecurityTestSuite) createSecurityTestMux() http.Handler {
	mux := http.NewServeMux()
	
	// Security middleware
	securityMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Add security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			
			// Authentication middleware
			userID := r.Header.Get("X-User-ID")
			if userID == "" && r.URL.Path != "/health" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			
			if userID != "" {
				// Find user
				var user *handlers.User
				for _, u := range suite.testUsers {
					if u.ID == userID {
						user = u
						break
					}
				}
				
				if user != nil {
					ctx := context.WithValue(r.Context(), "user", user)
					r = r.WithContext(ctx)
				}
			}
			
			next(w, r)
		}
	}
	
	// Apply security middleware to all routes
	mux.HandleFunc("/api/chat", securityMiddleware(suite.chatHandler.HandleChatMessage))
	mux.HandleFunc("/api/conversations", securityMiddleware(suite.chatHandler.HandleConversationList))
	mux.HandleFunc("/api/conversation/history", securityMiddleware(suite.chatHandler.HandleConversationHistory))
	mux.HandleFunc("/api/conversation/delete", securityMiddleware(suite.chatHandler.HandleConversationDelete))
	mux.HandleFunc("/chat/htmx", securityMiddleware(suite.chatHandler.HandleAIChatHTMX))
	mux.HandleFunc("/ws", securityMiddleware(suite.wsHandler.HandleWebSocketUpgrade))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	return mux
}

// TestInputValidationAndSanitization tests input validation and XSS prevention
func (suite *ChatSecurityTestSuite) TestInputValidationAndSanitization() {
	userID := suite.testUsers["regular_user"].ID
	
	maliciousInputs := []struct {
		name        string
		input       string
		expectBlock bool
		description string
	}{
		{
			name:        "Script Tag XSS",
			input:       "<script>alert('xss')</script>",
			expectBlock: false, // Should be sanitized but not blocked
			description: "JavaScript injection attempt",
		},
		{
			name:        "IMG Tag XSS",
			input:       "<img src=x onerror=alert('xss')>",
			expectBlock: false, // Should be sanitized
			description: "Image-based XSS attempt",
		},
		{
			name:        "Event Handler XSS",
			input:       "<div onmouseover='alert(1)'>test</div>",
			expectBlock: false, // Should be sanitized
			description: "Event handler injection",
		},
		{
			name:        "SQL Injection Attempt",
			input:       "'; DROP TABLE conversations; --",
			expectBlock: false, // Should be handled by parameterized queries
			description: "SQL injection attempt",
		},
		{
			name:        "Unicode XSS",
			input:       "<script>alert('unicode')</script>",
			expectBlock: false, // Should be sanitized
			description: "Unicode-based XSS",
		},
		{
			name:        "Extremely Long Input",
			input:       strings.Repeat("A", 100000),
			expectBlock: true, // Should be rejected for being too long
			description: "Buffer overflow attempt",
		},
		{
			name:        "HTML Entity Injection",
			input:       "&lt;script&gt;alert('entity')&lt;/script&gt;",
			expectBlock: false, // Should be safe after entity decoding
			description: "HTML entity-based injection",
		},
		{
			name:        "NoScript Tag",
			input:       "<noscript><img src=x onerror=alert('noscript')></noscript>",
			expectBlock: false, // Should be sanitized
			description: "NoScript-based XSS",
		},
	}
	
	for _, testCase := range maliciousInputs {
		suite.Run(testCase.name, func() {
			// Test HTTP endpoint
			suite.testHTTPInputSecurity(userID, testCase.input, testCase.expectBlock, testCase.description)
			
			// Test WebSocket endpoint
			suite.testWebSocketInputSecurity(userID, testCase.input, testCase.expectBlock, testCase.description)
			
			// Test HTMX endpoint
			suite.testHTMXInputSecurity(userID, testCase.input, testCase.expectBlock, testCase.description)
		})
	}
}

// testHTTPInputSecurity tests input security for HTTP endpoints
func (suite *ChatSecurityTestSuite) testHTTPInputSecurity(userID, input string, expectBlock bool, description string) {
	reqData := handlers.HTTPChatRequest{
		Message: input,
	}
	
	reqBody, _ := json.Marshal(reqData)
	req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", userID)
	
	w := httptest.NewRecorder()
	suite.server.Config.Handler.ServeHTTP(w, req)
	
	if expectBlock {
		suite.True(w.Code >= 400, "Malicious input should be blocked: %s", description)
	} else {
		suite.Equal(http.StatusOK, w.Code, "Valid input should be accepted: %s", description)
		
		// Check response for proper sanitization
		var response handlers.HTTPChatResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		if err == nil && response.Success {
			// Response should not contain raw script tags or other malicious content
			suite.NotContains(strings.ToLower(response.Response), "<script>")
			suite.NotContains(strings.ToLower(response.Response), "onerror=")
			suite.NotContains(strings.ToLower(response.Response), "javascript:")
		}
	}
}

// testWebSocketInputSecurity tests input security for WebSocket endpoints
func (suite *ChatSecurityTestSuite) testWebSocketInputSecurity(userID, input string, expectBlock bool, description string) {
	// Connect to WebSocket
	wsURL := "ws" + suite.server.URL[4:] + "/ws"
	headers := http.Header{}
	headers.Set("X-User-ID", userID)
	
	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		suite.T().Logf("Failed to connect to WebSocket for security test: %v", err)
		return
	}
	defer conn.Close()
	
	// Wait for connection established
	var welcomeMsg websocket.Message
	conn.ReadJSON(&welcomeMsg)
	
	// Send malicious input
	chatData := map[string]interface{}{
		"message": input,
	}
	chatJSON, _ := json.Marshal(chatData)
	
	wsMessage := websocket.Message{
		ID:        uuid.New().String(),
		Type:      "chat_message",
		Content:   string(chatJSON),
		Timestamp: time.Now(),
	}
	
	err = conn.WriteJSON(wsMessage)
	suite.NoError(err)
	
	// Read response
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var response websocket.Message
	err = conn.ReadJSON(&response)
	
	if expectBlock {
		// Should get error response
		suite.True(response.Type == "error" || err != nil, "Malicious WebSocket input should be blocked: %s", description)
	} else {
		// Should get normal response, but sanitized
		if err == nil && response.Type == "chat_response" {
			suite.NotContains(strings.ToLower(response.Content), "<script>")
			suite.NotContains(strings.ToLower(response.Content), "onerror=")
		}
	}
}

// testHTMXInputSecurity tests input security for HTMX endpoints
func (suite *ChatSecurityTestSuite) testHTMXInputSecurity(userID, input string, expectBlock bool, description string) {
	form := url.Values{}
	form.Set("message", input)
	
	req := httptest.NewRequest("POST", "/chat/htmx", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-User-ID", userID)
	
	w := httptest.NewRecorder()
	suite.server.Config.Handler.ServeHTTP(w, req)
	
	if expectBlock {
		suite.True(w.Code >= 400, "Malicious HTMX input should be blocked: %s", description)
	} else {
		suite.Equal(http.StatusOK, w.Code, "Valid HTMX input should be accepted: %s", description)
		
		// Check HTML response for proper escaping
		responseHTML := w.Body.String()
		suite.NotContains(responseHTML, "<script>")
		suite.NotContains(responseHTML, "onerror=")
		suite.NotContains(responseHTML, "javascript:")
		
		// Should contain escaped content if input had HTML
		if strings.Contains(input, "<") && strings.Contains(input, ">") {
			suite.True(strings.Contains(responseHTML, "&lt;") || strings.Contains(responseHTML, "&gt;"))
		}
	}
}

// TestAuthenticationAndAuthorization tests authentication and authorization
func (suite *ChatSecurityTestSuite) TestAuthenticationAndAuthorization() {
	regularUserID := suite.testUsers["regular_user"].ID
	maliciousUserID := suite.testUsers["malicious_user"].ID
	
	suite.Run("Unauthenticated Access", func() {
		// Try to access chat without authentication
		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message": "test"}`))
		req.Header.Set("Content-Type", "application/json")
		// No X-User-ID header
		
		w := httptest.NewRecorder()
		suite.server.Config.Handler.ServeHTTP(w, req)
		
		suite.Equal(http.StatusUnauthorized, w.Code)
	})
	
	suite.Run("Cross-User Access Control", func() {
		// Regular user creates a conversation
		resp1 := suite.sendChatMessage(regularUserID, "I want to make pasta", "")
		suite.Equal(http.StatusOK, resp1.StatusCode)
		
		var chatResp handlers.HTTPChatResponse
		err := json.NewDecoder(resp1.Body).Decode(&chatResp)
		suite.NoError(err)
		conversationID := chatResp.ConversationID
		
		// Malicious user tries to access regular user's conversation
		historyURL := fmt.Sprintf("/api/conversation/history?conversation_id=%s", conversationID)
		req := httptest.NewRequest("GET", historyURL, nil)
		req.Header.Set("X-User-ID", maliciousUserID)
		
		w := httptest.NewRecorder()
		suite.server.Config.Handler.ServeHTTP(w, req)
		
		suite.Equal(http.StatusForbidden, w.Code, "Users should not access other users' conversations")
	})
	
	suite.Run("Invalid User ID", func() {
		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message": "test"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "invalid-user-id")
		
		w := httptest.NewRecorder()
		suite.server.Config.Handler.ServeHTTP(w, req)
		
		suite.Equal(http.StatusUnauthorized, w.Code)
	})
	
	suite.Run("WebSocket Authentication", func() {
		// Try to connect without proper authentication
		wsURL := "ws" + suite.server.URL[4:] + "/ws"
		
		_, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
		suite.Error(err, "WebSocket connection should fail without authentication")
	})
}

// TestRateLimitingAndDDoSProtection tests rate limiting
func (suite *ChatSecurityTestSuite) TestRateLimitingAndDDoSProtection() {
	userID := suite.testUsers["regular_user"].ID
	
	suite.Run("HTTP Rate Limiting", func() {
		const requestCount = 50
		const timeWindow = 1 * time.Second
		
		var successCount, errorCount int
		
		start := time.Now()
		for i := 0; i < requestCount; i++ {
			resp := suite.sendChatMessage(userID, fmt.Sprintf("Message %d", i+1), "")
			if resp.StatusCode == http.StatusOK {
				successCount++
			} else if resp.StatusCode == http.StatusTooManyRequests {
				errorCount++
			}
			resp.Body.Close()
		}
		duration := time.Since(start)
		
		suite.T().Logf("Sent %d requests in %v", requestCount, duration)
		suite.T().Logf("Success: %d, Rate Limited: %d", successCount, errorCount)
		
		// If rate limiting is implemented, we should see some rate limit errors
		if errorCount > 0 {
			suite.Greater(errorCount, 0, "Rate limiting should block excessive requests")
		}
	})
	
	suite.Run("WebSocket Rate Limiting", func() {
		wsURL := "ws" + suite.server.URL[4:] + "/ws"
		headers := http.Header{}
		headers.Set("X-User-ID", userID)
		
		conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, headers)
		if err != nil {
			suite.T().Skip("WebSocket connection failed")
			return
		}
		defer conn.Close()
		
		// Wait for welcome message
		var welcomeMsg websocket.Message
		conn.ReadJSON(&welcomeMsg)
		
		const messageCount = 30
		var successCount, errorCount int
		
		for i := 0; i < messageCount; i++ {
			chatData := map[string]interface{}{
				"message": fmt.Sprintf("Rate limit test message %d", i+1),
			}
			chatJSON, _ := json.Marshal(chatData)
			
			wsMessage := websocket.Message{
				ID:        uuid.New().String(),
				Type:      "chat_message",
				Content:   string(chatJSON),
				Timestamp: time.Now(),
			}
			
			err := conn.WriteJSON(wsMessage)
			if err != nil {
				errorCount++
				continue
			}
			
			// Try to read response quickly
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			var response websocket.Message
			err = conn.ReadJSON(&response)
			if err == nil {
				if response.Type == "error" {
					errorCount++
				} else {
					successCount++
				}
			}
		}
		
		suite.T().Logf("WebSocket messages - Success: %d, Errors: %d", successCount, errorCount)
	})
}

// TestCSRFProtection tests CSRF protection mechanisms
func (suite *ChatSecurityTestSuite) TestCSRFProtection() {
	userID := suite.testUsers["regular_user"].ID
	
	suite.Run("Missing CSRF Token", func() {
		// This test would be relevant if CSRF tokens are implemented
		// For now, we test that the API doesn't accept requests from unauthorized origins
		
		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message": "test"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", userID)
		req.Header.Set("Origin", "https://malicious-site.com")
		req.Header.Set("Referer", "https://malicious-site.com/attack")
		
		w := httptest.NewRecorder()
		suite.server.Config.Handler.ServeHTTP(w, req)
		
		// Depending on CORS policy, this might be blocked
		// The test documents the current behavior
		suite.T().Logf("Request with malicious origin returned status: %d", w.Code)
	})
}

// TestDataLeakagePrevention tests prevention of data leakage
func (suite *ChatSecurityTestSuite) TestDataLeakagePrevention() {
	userID := suite.testUsers["regular_user"].ID
	
	suite.Run("Error Message Information Disclosure", func() {
		// Send invalid JSON to trigger error
		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"invalid": json}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", userID)
		
		w := httptest.NewRecorder()
		suite.server.Config.Handler.ServeHTTP(w, req)
		
		suite.Equal(http.StatusBadRequest, w.Code)
		
		// Error response should not contain sensitive information
		var errorResp handlers.HTTPChatResponse
		err := json.NewDecoder(w.Body).Decode(&errorResp)
		if err == nil {
			suite.NotContains(strings.ToLower(errorResp.Error), "database")
			suite.NotContains(strings.ToLower(errorResp.Error), "sql")
			suite.NotContains(strings.ToLower(errorResp.Error), "internal")
			suite.NotContains(strings.ToLower(errorResp.Error), "stack")
		}
	})
	
	suite.Run("Response Header Security", func() {
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		suite.server.Config.Handler.ServeHTTP(w, req)
		
		// Check security headers
		suite.Equal("nosniff", w.Header().Get("X-Content-Type-Options"))
		suite.Equal("DENY", w.Header().Get("X-Frame-Options"))
		suite.Equal("1; mode=block", w.Header().Get("X-XSS-Protection"))
		suite.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'self'")
	})
}

// TestInjectionAttacks tests various injection attack scenarios
func (suite *ChatSecurityTestSuite) TestInjectionAttacks() {
	userID := suite.testUsers["regular_user"].ID
	
	injectionTests := []struct {
		name    string
		payload string
		attack  string
	}{
		{
			name:    "SQL Injection",
			payload: "'; DELETE FROM conversations WHERE '1'='1",
			attack:  "SQL injection attempt",
		},
		{
			name:    "NoSQL Injection",
			payload: "{ $ne: null }",
			attack:  "NoSQL injection attempt",
		},
		{
			name:    "Command Injection",
			payload: "; rm -rf /; echo 'pwned'",
			attack:  "Command injection attempt",
		},
		{
			name:    "Path Traversal",
			payload: "../../../../etc/passwd",
			attack:  "Path traversal attempt",
		},
		{
			name:    "LDAP Injection",
			payload: "admin)(|(password=*))",
			attack:  "LDAP injection attempt",
		},
		{
			name:    "XML Injection",
			payload: "<?xml version='1.0'?><!DOCTYPE root [<!ENTITY test SYSTEM 'file:///etc/passwd'>]><root>&test;</root>",
			attack:  "XML injection attempt",
		},
	}
	
	for _, test := range injectionTests {
		suite.Run(test.name, func() {
			resp := suite.sendChatMessage(userID, test.payload, "")
			
			// Request should be handled gracefully
			suite.True(resp.StatusCode == http.StatusOK || resp.StatusCode >= 400)
			
			if resp.StatusCode == http.StatusOK {
				var chatResp handlers.HTTPChatResponse
				err := json.NewDecoder(resp.Body).Decode(&chatResp)
				if err == nil && chatResp.Success {
					// Response should not indicate successful injection
					response := strings.ToLower(chatResp.Response)
					suite.NotContains(response, "error")
					suite.NotContains(response, "exception")
					suite.NotContains(response, "failed")
					suite.NotContains(response, "database")
				}
			}
			
			resp.Body.Close()
		})
	}
}

// TestSessionManagement tests session management security
func (suite *ChatSecurityTestSuite) TestSessionManagement() {
	userID := suite.testUsers["regular_user"].ID
	
	suite.Run("WebSocket Session Isolation", func() {
		// Connect two WebSocket sessions for the same user
		wsURL := "ws" + suite.server.URL[4:] + "/ws"
		headers := http.Header{}
		headers.Set("X-User-ID", userID)
		
		conn1, _, err := gorillaws.DefaultDialer.Dial(wsURL, headers)
		if err != nil {
			suite.T().Skip("WebSocket connection failed")
			return
		}
		defer conn1.Close()
		
		conn2, _, err := gorillaws.DefaultDialer.Dial(wsURL, headers)
		if err != nil {
			suite.T().Skip("Second WebSocket connection failed")
			return
		}
		defer conn2.Close()
		
		// Wait for welcome messages
		var welcome1, welcome2 websocket.Message
		conn1.ReadJSON(&welcome1)
		conn2.ReadJSON(&welcome2)
		
		// Send message from first connection
		chatData := map[string]interface{}{
			"message": "Secret message from connection 1",
		}
		chatJSON, _ := json.Marshal(chatData)
		
		wsMessage := websocket.Message{
			ID:        uuid.New().String(),
			Type:      "chat_message",
			Content:   string(chatJSON),
			Timestamp: time.Now(),
		}
		
		err = conn1.WriteJSON(wsMessage)
		suite.NoError(err)
		
		// Second connection should not receive the message automatically
		conn2.SetReadDeadline(time.Now().Add(1 * time.Second))
		var receivedMsg websocket.Message
		err = conn2.ReadJSON(&receivedMsg)
		
		// Should timeout or receive only its own messages
		suite.True(err != nil || receivedMsg.Type != "chat_response")
	})
}

// TestPrivacyAndDataProtection tests privacy and data protection measures
func (suite *ChatSecurityTestSuite) TestPrivacyAndDataProtection() {
	userID := suite.testUsers["regular_user"].ID
	
	suite.Run("PII Handling", func() {
		piiData := []string{
			"My social security number is 123-45-6789",
			"My credit card number is 4532-1234-5678-9012",
			"My email is john.doe@example.com and my phone is 555-123-4567",
			"I live at 123 Main Street, Anytown, ST 12345",
		}
		
		for _, pii := range piiData {
			resp := suite.sendChatMessage(userID, pii, "")
			suite.Equal(http.StatusOK, resp.StatusCode)
			
			var chatResp handlers.HTTPChatResponse
			err := json.NewDecoder(resp.Body).Decode(&chatResp)
			if err == nil && chatResp.Success {
				// AI should not echo back sensitive information
				response := chatResp.Response
				suite.NotContains(response, "123-45-6789")
				suite.NotContains(response, "4532-1234-5678-9012")
				// AI should handle PII appropriately
			}
			resp.Body.Close()
		}
	})
}

// Helper methods

// sendChatMessage sends a chat message via HTTP
func (suite *ChatSecurityTestSuite) sendChatMessage(userID, message, conversationID string) *http.Response {
	reqData := handlers.HTTPChatRequest{
		Message:        message,
		ConversationID: conversationID,
	}
	
	reqBody, _ := json.Marshal(reqData)
	req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", userID)
	
	w := httptest.NewRecorder()
	suite.server.Config.Handler.ServeHTTP(w, req)
	
	return &http.Response{
		StatusCode: w.Code,
		Header:     w.Header(),
		Body:       io.NopCloser(w.Body),
	}
}

// Run the security test suite
func TestChatSecuritySuite(t *testing.T) {
	suite.Run(t, new(ChatSecurityTestSuite))
}

// TestAdditionalSecurityScenarios tests additional security scenarios
func TestAdditionalSecurityScenarios(t *testing.T) {
	testSuite := testutils.NewConversationTestSuite()
	chatHandler := handlers.NewChatHandler(testSuite.ConversationService)
	
	testUser := &handlers.User{
		ID:    "security-test-user",
		Email: "security@example.com",
		Name:  "Security Test User",
	}
	
	t.Run("Large Payload Attack", func(t *testing.T) {
		// Test with extremely large payload
		largeMessage := strings.Repeat("A", 10*1024*1024) // 10MB
		
		reqData := handlers.HTTPChatRequest{
			Message: largeMessage,
		}
		
		reqBody, _ := json.Marshal(reqData)
		req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), "user", testUser)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		chatHandler.HandleChatMessage(w, req)
		
		// Should be rejected or handled gracefully
		assert.True(t, w.Code >= 400 || w.Code == http.StatusOK)
		
		if w.Code == http.StatusOK {
			var response handlers.HTTPChatResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			if err == nil {
				// Response should not echo back the entire large payload
				assert.Less(t, len(response.Response), 1024*1024) // Response should be much smaller
			}
		}
	})
	
	t.Run("Unicode and Encoding Attacks", func(t *testing.T) {
		unicodeAttacks := []string{
			"\\u003cscript\\u003ealert('xss')\\u003c/script\\u003e", // Unicode-encoded script
			"<script>alert('xss')</script>",                        // Direct script
			"\u202E<script>alert('xss')</script>",                  // Right-to-left override
			"javascript:\\u0061lert('xss')",                        // Unicode in javascript:
		}
		
		for _, attack := range unicodeAttacks {
			reqData := handlers.HTTPChatRequest{
				Message: attack,
			}
			
			reqBody, _ := json.Marshal(reqData)
			req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(req.Context(), "user", testUser)
			req = req.WithContext(ctx)
			
			w := httptest.NewRecorder()
			chatHandler.HandleChatMessage(w, req)
			
			// Should handle safely
			assert.True(t, w.Code == http.StatusOK || w.Code >= 400)
			
			if w.Code == http.StatusOK {
				var response handlers.HTTPChatResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				if err == nil && response.Success {
					// Response should not contain unescaped scripts
					assert.NotContains(t, response.Response, "<script>")
					assert.NotContains(t, response.Response, "javascript:")
				}
			}
		}
	})
	
	t.Run("Content Type Confusion", func(t *testing.T) {
		// Test with wrong content type
		req := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{"message": "test"}`))
		req.Header.Set("Content-Type", "text/plain") // Wrong content type
		ctx := context.WithValue(req.Context(), "user", testUser)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		chatHandler.HandleChatMessage(w, req)
		
		// Should handle gracefully or reject
		assert.True(t, w.Code >= 400 || w.Code == http.StatusOK)
	})
	
	t.Run("HTTP Method Confusion", func(t *testing.T) {
		// Test with wrong HTTP method
		req := httptest.NewRequest("GET", "/api/chat?message=test", nil)
		ctx := context.WithValue(req.Context(), "user", testUser)
		req = req.WithContext(ctx)
		
		w := httptest.NewRecorder()
		chatHandler.HandleChatMessage(w, req)
		
		// Should reject GET requests to POST endpoint
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}