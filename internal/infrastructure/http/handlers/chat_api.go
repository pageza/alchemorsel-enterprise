package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// ChatAPIHandlers handles API chat endpoints
type ChatAPIHandlers struct {
	conversationService *conversation.Service
	logger             *zap.Logger
}

// NewChatAPIHandlers creates new chat API handlers
func NewChatAPIHandlers(conversationService *conversation.Service, logger *zap.Logger) *ChatAPIHandlers {
	return &ChatAPIHandlers{
		conversationService: conversationService,
		logger:             logger,
	}
}

// ListConversations handles GET /api/v3/chat/conversations
func (h *ChatAPIHandlers) ListConversations(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	
	limit := 50 // default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}
	
	offset := 0 // default
	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	conversations, err := h.conversationService.GetUserConversations(r.Context(), user.ID, limit, offset)
	if err != nil {
		h.logger.Error("Failed to get conversations", zap.Error(err), zap.String("user_id", user.ID))
		h.writeErrorResponse(w, "Failed to retrieve conversations", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"success":       true,
		"conversations": conversations,
		"limit":         limit,
		"offset":        offset,
		"count":         len(conversations),
	})
}

// CreateConversation handles POST /api/v3/chat/conversations
func (h *ChatAPIHandlers) CreateConversation(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req struct {
		Title         string `json:"title"`
		InitialMessage string `json:"initial_message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.InitialMessage == "" {
		h.writeErrorResponse(w, "Initial message is required", http.StatusBadRequest)
		return
	}

	conversation, err := h.conversationService.CreateConversation(r.Context(), user.ID, req.InitialMessage)
	if err != nil {
		h.logger.Error("Failed to create conversation", zap.Error(err), zap.String("user_id", user.ID))
		h.writeErrorResponse(w, "Failed to create conversation", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"success":      true,
		"conversation": conversation,
	})
}

// GetConversation handles GET /api/v3/chat/conversations/{id}
func (h *ChatAPIHandlers) GetConversation(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	conversationID := chi.URLParam(r, "id")
	if conversationID == "" {
		h.writeErrorResponse(w, "Conversation ID is required", http.StatusBadRequest)
		return
	}

	conversation, err := h.conversationService.GetConversation(r.Context(), conversationID)
	if err != nil {
		h.logger.Error("Failed to get conversation", zap.Error(err), zap.String("conversation_id", conversationID))
		h.writeErrorResponse(w, "Conversation not found", http.StatusNotFound)
		return
	}

	// Check if user owns the conversation
	if conversation.UserID != user.ID {
		h.writeErrorResponse(w, "Access denied", http.StatusForbidden)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"success":      true,
		"conversation": conversation,
	})
}

// UpdateConversation handles PUT /api/v3/chat/conversations/{id}
func (h *ChatAPIHandlers) UpdateConversation(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	conversationID := chi.URLParam(r, "id")
	if conversationID == "" {
		h.writeErrorResponse(w, "Conversation ID is required", http.StatusBadRequest)
		return
	}

	var req struct {
		Title string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		h.writeErrorResponse(w, "Title is required", http.StatusBadRequest)
		return
	}

	// Check if user owns the conversation
	conversation, err := h.conversationService.GetConversation(r.Context(), conversationID)
	if err != nil {
		h.logger.Error("Failed to get conversation", zap.Error(err), zap.String("conversation_id", conversationID))
		h.writeErrorResponse(w, "Conversation not found", http.StatusNotFound)
		return
	}

	if conversation.UserID != user.ID {
		h.writeErrorResponse(w, "Access denied", http.StatusForbidden)
		return
	}

	// Rename the conversation
	if err := h.conversationService.RenameConversation(r.Context(), conversationID, req.Title); err != nil {
		h.logger.Error("Failed to rename conversation", zap.Error(err), zap.String("conversation_id", conversationID))
		h.writeErrorResponse(w, "Failed to rename conversation", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Conversation renamed successfully",
		"title":   req.Title,
	})
}

// DeleteConversation handles DELETE /api/v3/chat/conversations/{id}
func (h *ChatAPIHandlers) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	conversationID := chi.URLParam(r, "id")
	if conversationID == "" {
		h.writeErrorResponse(w, "Conversation ID is required", http.StatusBadRequest)
		return
	}

	// Check if user owns the conversation
	conversation, err := h.conversationService.GetConversation(r.Context(), conversationID)
	if err != nil {
		h.logger.Error("Failed to get conversation", zap.Error(err), zap.String("conversation_id", conversationID))
		h.writeErrorResponse(w, "Conversation not found", http.StatusNotFound)
		return
	}

	if conversation.UserID != user.ID {
		h.writeErrorResponse(w, "Access denied", http.StatusForbidden)
		return
	}

	// Delete the conversation
	if err := h.conversationService.DeleteConversation(r.Context(), conversationID); err != nil {
		h.logger.Error("Failed to delete conversation", zap.Error(err), zap.String("conversation_id", conversationID))
		h.writeErrorResponse(w, "Failed to delete conversation", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Conversation deleted successfully",
	})
}

// GetConversationMessages handles GET /api/v3/chat/conversations/{id}/messages
func (h *ChatAPIHandlers) GetConversationMessages(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	conversationID := chi.URLParam(r, "id")
	if conversationID == "" {
		h.writeErrorResponse(w, "Conversation ID is required", http.StatusBadRequest)
		return
	}

	conversation, messages, err := h.conversationService.GetConversationWithMessages(r.Context(), conversationID)
	if err != nil {
		h.logger.Error("Failed to get conversation messages", zap.Error(err), zap.String("conversation_id", conversationID))
		h.writeErrorResponse(w, "Failed to retrieve conversation messages", http.StatusInternalServerError)
		return
	}

	// Check if user owns the conversation
	if conversation.UserID != user.ID {
		h.writeErrorResponse(w, "Access denied", http.StatusForbidden)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"success":      true,
		"conversation": conversation,
		"messages":     messages,
		"count":        len(messages),
	})
}

// SendMessage handles POST /api/v3/chat/conversations/{id}/messages
func (h *ChatAPIHandlers) SendMessage(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	conversationID := chi.URLParam(r, "id")
	if conversationID == "" {
		h.writeErrorResponse(w, "Conversation ID is required", http.StatusBadRequest)
		return
	}

	var req struct {
		Message string `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		h.writeErrorResponse(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Check if user owns the conversation
	conversation, err := h.conversationService.GetConversation(r.Context(), conversationID)
	if err != nil {
		h.logger.Error("Failed to get conversation", zap.Error(err), zap.String("conversation_id", conversationID))
		h.writeErrorResponse(w, "Conversation not found", http.StatusNotFound)
		return
	}

	if conversation.UserID != user.ID {
		h.writeErrorResponse(w, "Access denied", http.StatusForbidden)
		return
	}

	// Process the message
	userMessage, aiResponse, err := h.conversationService.ProcessMessage(r.Context(), conversationID, req.Message, user.ID)
	if err != nil {
		h.logger.Error("Failed to process message", zap.Error(err), zap.String("conversation_id", conversationID))
		h.writeErrorResponse(w, "Failed to process message", http.StatusInternalServerError)
		return
	}

	h.writeJSONResponse(w, map[string]interface{}{
		"success":         true,
		"user_message":    userMessage,
		"ai_response":     aiResponse,
		"conversation_id": conversationID,
	})
}

// writeJSONResponse writes a JSON response
func (h *ChatAPIHandlers) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// writeErrorResponse writes an error response
func (h *ChatAPIHandlers) writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

// getUserFromContext extracts user from context
func getUserFromContext(ctx context.Context) *User {
	if user, ok := ctx.Value("user").(*User); ok {
		return user
	}
	return nil
}