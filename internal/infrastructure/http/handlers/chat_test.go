// Package handlers provides HTTP handlers for chat functionality
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test setup helper - creates a basic handler for HTTP layer testing
func setupChatHandler(t *testing.T) *ChatHandler {
	// Create a service with nil dependencies for basic HTTP layer testing
	convService := conversation.NewService(nil, nil, nil, nil)
	handler := NewChatHandler(convService)
	return handler
}

// Helper to create test user
func createTestUser() *user.User {
	testUser, _ := user.NewUser("test@example.com", "Test User", "hashedpassword")
	return testUser
}

// Helper to add user to context
func addUserToContext(r *http.Request, u *user.User) *http.Request {
	ctx := context.WithValue(r.Context(), "user", u)
	return r.WithContext(ctx)
}

// TestNewChatHandler tests the constructor
func TestNewChatHandler(t *testing.T) {
	handler := setupChatHandler(t)
	assert.NotNil(t, handler)
	assert.NotNil(t, handler.convService)
}

// TestHandleChatMessage tests the HandleChatMessage handler
func TestHandleChatMessage(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
		checkResponse  func(t *testing.T, response HTTPChatResponse)
	}{
		{
			name: "unauthenticated request",
			setupRequest: func() *http.Request {
				reqBody := HTTPChatRequest{Message: "Hello"}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/chat", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, response HTTPChatResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "Authentication required", response.Error)
			},
		},
		{
			name: "empty message",
			setupRequest: func() *http.Request {
				reqBody := HTTPChatRequest{Message: ""}
				body, _ := json.Marshal(reqBody)
				req := httptest.NewRequest("POST", "/chat", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				return addUserToContext(req, createTestUser())
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response HTTPChatResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "Message cannot be empty", response.Error)
			},
		},
		{
			name: "invalid JSON",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("POST", "/chat", bytes.NewBuffer([]byte("invalid json")))
				req.Header.Set("Content-Type", "application/json")
				return addUserToContext(req, createTestUser())
			},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, response HTTPChatResponse) {
				assert.False(t, response.Success)
				assert.Equal(t, "Invalid JSON", response.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupChatHandler(t)

			req := tt.setupRequest()
			w := httptest.NewRecorder()

			handler.HandleChatMessage(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response HTTPChatResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			tt.checkResponse(t, response)
		})
	}
}

// TestHandleConversationList tests the HandleConversationList handler
func TestHandleConversationList(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      func() *user.User
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "unauthenticated request",
			setupUser:      func() *user.User { return nil },
			expectedStatus: http.StatusUnauthorized,
			checkResponse: func(t *testing.T, body []byte) {
				var response HTTPChatResponse
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				assert.False(t, response.Success)
				assert.Equal(t, "Authentication required", response.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupChatHandler(t)
			testUser := tt.setupUser()

			req := httptest.NewRequest("GET", "/conversations", nil)
			if testUser != nil {
				req = addUserToContext(req, testUser)
			}
			w := httptest.NewRecorder()

			handler.HandleConversationList(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w.Body.Bytes())
		})
	}
}

// TestHandleConversationHistory tests the HandleConversationHistory handler
func TestHandleConversationHistory(t *testing.T) {
	tests := []struct {
		name           string
		conversationID string
		expectedStatus int
		checkResponse  func(t *testing.T, body []byte)
	}{
		{
			name:           "unauthenticated request",
			conversationID: "conv-123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing conversation ID",
			conversationID: "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupChatHandler(t)

			url := "/conversation/history"
			if tt.conversationID != "" {
				url += "?conversation_id=" + tt.conversationID
			}
			req := httptest.NewRequest("GET", url, nil)

			// Only add user for non-auth tests
			if tt.name != "unauthenticated request" {
				req = addUserToContext(req, createTestUser())
			}

			w := httptest.NewRecorder()

			handler.HandleConversationHistory(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestHandleConversationDelete tests the HandleConversationDelete handler
func TestHandleConversationDelete(t *testing.T) {
	tests := []struct {
		name           string
		conversationID string
		expectedStatus int
	}{
		{
			name:           "unauthenticated request",
			conversationID: "conv-123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "missing conversation ID",
			conversationID: "",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupChatHandler(t)

			form := url.Values{}
			if tt.conversationID != "" {
				form.Add("conversation_id", tt.conversationID)
			}
			req := httptest.NewRequest("POST", "/conversation/delete", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Only add user for non-auth tests
			if tt.name != "unauthenticated request" {
				req = addUserToContext(req, createTestUser())
			}

			w := httptest.NewRecorder()

			handler.HandleConversationDelete(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestHandleConversationRename tests the HandleConversationRename handler
func TestHandleConversationRename(t *testing.T) {
	tests := []struct {
		name           string
		conversationID string
		newTitle       string
		expectedStatus int
	}{
		{
			name:           "unauthenticated request",
			conversationID: "conv-123",
			newTitle:       "New Title",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "empty title",
			conversationID: "conv-123",
			newTitle:       "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing conversation ID",
			conversationID: "",
			newTitle:       "New Title",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupChatHandler(t)

			form := url.Values{}
			form.Add("conversation_id", tt.conversationID)
			form.Add("title", tt.newTitle)
			req := httptest.NewRequest("POST", "/conversation/rename", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			// Only add user for non-auth tests
			if tt.name != "unauthenticated request" {
				req = addUserToContext(req, createTestUser())
			}

			w := httptest.NewRecorder()

			handler.HandleConversationRename(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

// TestHandleAIChatHTMX tests the HTMX chat handler
func TestHandleAIChatHTMX(t *testing.T) {
	tests := []struct {
		name         string
		setupRequest func() *http.Request
		checkHTML    func(t *testing.T, html string)
	}{
		{
			name: "unauthenticated user",
			setupRequest: func() *http.Request {
				form := url.Values{}
				form.Add("message", "Hello")
				req := httptest.NewRequest("POST", "/chat/htmx", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return req
			},
			checkHTML: func(t *testing.T, html string) {
				assert.Contains(t, html, "need to be logged in")
				assert.Contains(t, html, "Login")
				assert.Contains(t, html, "Register")
			},
		},
		{
			name: "empty message",
			setupRequest: func() *http.Request {
				form := url.Values{}
				form.Add("message", "")
				req := httptest.NewRequest("POST", "/chat/htmx", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				return addUserToContext(req, createTestUser())
			},
			checkHTML: func(t *testing.T, html string) {
				assert.Contains(t, html, "Message cannot be empty")
				assert.Contains(t, html, "❌")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := setupChatHandler(t)

			req := tt.setupRequest()
			w := httptest.NewRecorder()

			handler.HandleAIChatHTMX(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "text/html", w.Header().Get("Content-Type"))

			html := w.Body.String()
			tt.checkHTML(t, html)
		})
	}
}

// TestHandleConversationListHTMX tests the HTMX conversation list handler
func TestHandleConversationListHTMX(t *testing.T) {
	handler := setupChatHandler(t)

	t.Run("unauthenticated user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/conversations/htmx", nil)
		w := httptest.NewRecorder()

		handler.HandleConversationListHTMX(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		html := w.Body.String()
		assert.Contains(t, html, "Please log in to view conversations")
	})
}

// TestHandleConversationStats tests the HandleConversationStats handler
func TestHandleConversationStats(t *testing.T) {
	handler := setupChatHandler(t)

	t.Run("unauthenticated request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/conversation/stats", nil)
		w := httptest.NewRecorder()

		handler.HandleConversationStats(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestUtilityFunctions tests utility functions
func TestUtilityFunctions(t *testing.T) {
	handler := setupChatHandler(t)

	t.Run("formatTimeAgo", func(t *testing.T) {
		now := time.Now()

		tests := []struct {
			time     time.Time
			expected string
		}{
			{now.Add(-30 * time.Second), "Just now"},
			{now.Add(-2 * time.Minute), "2 minutes ago"},
			{now.Add(-1 * time.Minute), "1 minute ago"},
			{now.Add(-2 * time.Hour), "2 hours ago"},
			{now.Add(-1 * time.Hour), "1 hour ago"},
			{now.Add(-2 * 24 * time.Hour), "2 days ago"},
			{now.Add(-1 * 24 * time.Hour), "1 day ago"},
		}

		for _, tt := range tests {
			result := handler.formatTimeAgo(tt.time)
			assert.Equal(t, tt.expected, result)
		}
	})

	t.Run("escapeHTML", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
			{"Hello & goodbye", "Hello &amp; goodbye"},
			{`"quoted" text`, "&quot;quoted&quot; text"},
		}

		for _, tt := range tests {
			result := handler.escapeHTML(tt.input)
			assert.Equal(t, tt.expected, result)
		}
	})
}

// TestWriteErrorResponse tests the error response helper
func TestWriteErrorResponse(t *testing.T) {
	handler := setupChatHandler(t)

	w := httptest.NewRecorder()
	handler.writeErrorResponse(w, "Test error", http.StatusBadRequest)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response HTTPChatResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.False(t, response.Success)
	assert.Equal(t, "Test error", response.Error)
}

// TestWriteHTMXError tests the HTMX error helper
func TestWriteHTMXError(t *testing.T) {
	handler := setupChatHandler(t)

	w := httptest.NewRecorder()
	handler.writeHTMXError(w, "Test HTMX error")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/html", w.Header().Get("Content-Type"))

	html := w.Body.String()
	assert.Contains(t, html, "Test HTMX error")
	assert.Contains(t, html, "❌")
	assert.Contains(t, html, "message assistant")
}
