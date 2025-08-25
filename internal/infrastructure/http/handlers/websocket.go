package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/websocket"
)

// WebSocketHandler handles WebSocket connections for chat
type WebSocketHandler struct {
	wsManager   *websocket.Manager
	convService *conversation.Service
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(wsManager *websocket.Manager, convService *conversation.Service) *WebSocketHandler {
	return &WebSocketHandler{
		wsManager:   wsManager,
		convService: convService,
	}
}

// HandleWebSocketUpgrade handles WebSocket upgrade requests
func (h *WebSocketHandler) HandleWebSocketUpgrade(w http.ResponseWriter, r *http.Request) {
	// Get user from context (authentication middleware should set this)
	userID := getUserIDFromContext(r.Context())
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade the connection
	err := h.wsManager.HandleUpgrade(w, r, userID)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}
}

// ChatMessage represents a chat message from the client
type ChatMessage struct {
	Type           string                 `json:"type"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	Content        string                 `json:"content"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// ChatResponse represents a response to the client
type ChatResponse struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	Content        string                 `json:"content"`
	Role           string                 `json:"role"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Timestamp      time.Time              `json:"timestamp"`
	Error          string                 `json:"error,omitempty"`
}

// ProcessMessage processes incoming chat messages
func (h *WebSocketHandler) ProcessMessage(ctx context.Context, userID string, message websocket.Message) {
	log.Printf("Processing WebSocket message from user %s: %s", userID, message.Type)

	switch message.Type {
	case "chat_message":
		h.handleChatMessage(ctx, userID, message)
	case "ping":
		h.handlePing(ctx, userID, message)
	default:
		log.Printf("Unknown message type: %s", message.Type)
		h.sendError(userID, message.ID, "Unknown message type")
	}
}

// handleChatMessage processes chat messages
func (h *WebSocketHandler) handleChatMessage(ctx context.Context, userID string, message websocket.Message) {
	var chatMsg ChatMessage
	if err := json.Unmarshal([]byte(message.Content), &chatMsg); err != nil {
		log.Printf("Failed to parse chat message: %v", err)
		h.sendError(userID, message.ID, "Invalid message format")
		return
	}

	// Handle conversation creation or continuation
	conversationID := chatMsg.ConversationID
	if conversationID == "" {
		// Create new conversation
		conv, err := h.convService.CreateConversation(ctx, userID, chatMsg.Content)
		if err != nil {
			log.Printf("Failed to create conversation: %v", err)
			h.sendError(userID, message.ID, "Failed to create conversation")
			return
		}
		conversationID = conv.ID

		// Notify client about new conversation
		h.sendResponse(userID, ChatResponse{
			Type:           "conversation_created",
			ConversationID: conversationID,
			Timestamp:      time.Now(),
		})
	}

	// Process the message
	userMsg, aiResponse, err := h.convService.ProcessMessage(ctx, conversationID, chatMsg.Content, userID)
	if err != nil {
		log.Printf("Failed to process message: %v", err)
		h.sendError(userID, message.ID, "Failed to process message")
		return
	}

	// Send AI response back to user
	response := ChatResponse{
		ID:             userMsg.ID,
		Type:           "chat_response",
		ConversationID: conversationID,
		Content:        aiResponse,
		Role:           "assistant",
		Metadata:       make(map[string]interface{}),
		Timestamp:      time.Now(),
	}

	h.sendResponse(userID, response)
}

// handlePing handles ping messages
func (h *WebSocketHandler) handlePing(ctx context.Context, userID string, message websocket.Message) {
	response := websocket.Message{
		ID:        message.ID,
		Type:      "pong",
		Timestamp: time.Now(),
	}

	if err := h.wsManager.SendToUser(userID, response); err != nil {
		log.Printf("Failed to send pong: %v", err)
	}
}

// sendResponse sends a chat response to a user
func (h *WebSocketHandler) sendResponse(userID string, response ChatResponse) {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	wsMessage := websocket.Message{
		ID:        response.ID,
		Type:      response.Type,
		Content:   string(responseBytes),
		Timestamp: response.Timestamp,
	}

	if err := h.wsManager.SendToUser(userID, wsMessage); err != nil {
		log.Printf("Failed to send response to user %s: %v", userID, err)
	}
}

// sendError sends an error message to a user
func (h *WebSocketHandler) sendError(userID, messageID, errorMsg string) {
	response := ChatResponse{
		ID:        messageID,
		Type:      "error",
		Error:     errorMsg,
		Timestamp: time.Now(),
	}

	responseBytes, _ := json.Marshal(response)
	wsMessage := websocket.Message{
		ID:        messageID,
		Type:      "error",
		Content:   string(responseBytes),
		Timestamp: time.Now(),
	}

	if err := h.wsManager.SendToUser(userID, wsMessage); err != nil {
		log.Printf("Failed to send error to user %s: %v", userID, err)
	}
}

// getUserIDFromContext extracts user ID from request context
func getUserIDFromContext(ctx context.Context) string {
	if user, ok := ctx.Value("user").(*User); ok && user != nil {
		return user.ID
	}
	return ""
}

// User represents a user (this should match your existing User struct)
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}