package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
)

// ChatHandler handles HTTP-based chat interactions
type ChatHandler struct {
	convService *conversation.Service
}

// NewChatHandler creates a new chat handler
func NewChatHandler(convService *conversation.Service) *ChatHandler {
	return &ChatHandler{
		convService: convService,
	}
}

// HTTPChatRequest represents an HTTP chat request
type HTTPChatRequest struct {
	Message        string `json:"message"`
	ConversationID string `json:"conversation_id"`
}

// HTTPChatResponse represents an HTTP chat response
type HTTPChatResponse struct {
	Success        bool                   `json:"success"`
	Response       string                 `json:"response"`
	ConversationID string                 `json:"conversation_id"`
	MessageID      string                 `json:"message_id"`
	Intent         string                 `json:"intent,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

// HandleChatMessage processes HTTP chat messages
func (h *ChatHandler) HandleChatMessage(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user := getUserFromHTTPContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Parse request
	var req HTTPChatRequest
	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeErrorResponse(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
	} else {
		// Handle form data
		req.Message = r.FormValue("message")
		req.ConversationID = r.FormValue("conversation_id")
	}

	if req.Message == "" {
		h.writeErrorResponse(w, "Message cannot be empty", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Handle conversation creation or continuation
	conversationID := req.ConversationID
	if conversationID == "" {
		// Create new conversation
		conv, err := h.convService.CreateConversation(ctx, user.ID, req.Message)
		if err != nil {
			log.Printf("Failed to create conversation: %v", err)
			h.writeErrorResponse(w, "Failed to create conversation", http.StatusInternalServerError)
			return
		}
		conversationID = conv.ID
	}

	// Process the message
	userMsg, aiResponse, err := h.convService.ProcessMessage(ctx, conversationID, req.Message, user.ID)
	if err != nil {
		log.Printf("Failed to process message: %v", err)
		h.writeErrorResponse(w, "Failed to process message", http.StatusInternalServerError)
		return
	}

	// Create response
	response := HTTPChatResponse{
		Success:        true,
		Response:       aiResponse,
		ConversationID: conversationID,
		MessageID:      userMsg.ID,
		Metadata:       make(map[string]interface{}),
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleChatPage renders the chat page
func (h *ChatHandler) HandleChatPage(w http.ResponseWriter, r *http.Request) {
	user := getUserFromHTTPContext(r.Context())
	
	data := map[string]interface{}{
		"Title":          "Chat with AI Chef - Alchemorsel",
		"Description":    "Chat with our AI Chef to create amazing recipes through conversation",
		"User":           user,
		"IsAuthenticated": user != nil,
		"CurrentPage":    "chat",
	}

	if err := renderHTMLTemplate(w, "chat", data); err != nil {
		log.Printf("Failed to render chat template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// HandleConversationList returns user's conversations
func (h *ChatHandler) HandleConversationList(w http.ResponseWriter, r *http.Request) {
	user := getUserFromHTTPContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	conversations, err := h.convService.GetUserConversations(ctx, user.ID, 50, 0)
	if err != nil {
		log.Printf("Failed to get conversations: %v", err)
		h.writeErrorResponse(w, "Failed to retrieve conversations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"conversations": conversations,
	})
}

// HandleConversationHistory returns messages for a specific conversation
func (h *ChatHandler) HandleConversationHistory(w http.ResponseWriter, r *http.Request) {
	user := getUserFromHTTPContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	conversationID := r.URL.Query().Get("conversation_id")
	if conversationID == "" {
		h.writeErrorResponse(w, "Conversation ID required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	conv, messages, err := h.convService.GetConversationWithMessages(ctx, conversationID)
	if err != nil {
		log.Printf("Failed to get conversation history: %v", err)
		h.writeErrorResponse(w, "Failed to retrieve conversation history", http.StatusInternalServerError)
		return
	}

	// Check if user owns the conversation
	if conv.UserID != user.ID {
		h.writeErrorResponse(w, "Access denied", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"conversation": conv,
		"messages":     messages,
	})
}

// HandleConversationDelete deletes a conversation
func (h *ChatHandler) HandleConversationDelete(w http.ResponseWriter, r *http.Request) {
	user := getUserFromHTTPContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	conversationID := r.FormValue("conversation_id")
	if conversationID == "" {
		h.writeErrorResponse(w, "Conversation ID required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if user owns the conversation
	conv, err := h.convService.GetConversation(ctx, conversationID)
	if err != nil {
		log.Printf("Failed to get conversation: %v", err)
		h.writeErrorResponse(w, "Conversation not found", http.StatusNotFound)
		return
	}

	if conv.UserID != user.ID {
		h.writeErrorResponse(w, "Access denied", http.StatusForbidden)
		return
	}

	// Delete the conversation
	if err := h.convService.DeleteConversation(ctx, conversationID); err != nil {
		log.Printf("Failed to delete conversation: %v", err)
		h.writeErrorResponse(w, "Failed to delete conversation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Conversation deleted successfully",
	})
}

// HandleConversationRename renames a conversation
func (h *ChatHandler) HandleConversationRename(w http.ResponseWriter, r *http.Request) {
	user := getUserFromHTTPContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	conversationID := r.FormValue("conversation_id")
	newTitle := strings.TrimSpace(r.FormValue("title"))
	
	if conversationID == "" {
		h.writeErrorResponse(w, "Conversation ID required", http.StatusBadRequest)
		return
	}
	
	if newTitle == "" {
		h.writeErrorResponse(w, "Title cannot be empty", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if user owns the conversation
	conv, err := h.convService.GetConversation(ctx, conversationID)
	if err != nil {
		log.Printf("Failed to get conversation: %v", err)
		h.writeErrorResponse(w, "Conversation not found", http.StatusNotFound)
		return
	}

	if conv.UserID != user.ID {
		h.writeErrorResponse(w, "Access denied", http.StatusForbidden)
		return
	}

	// Rename the conversation
	if err := h.convService.RenameConversation(ctx, conversationID, newTitle); err != nil {
		log.Printf("Failed to rename conversation: %v", err)
		h.writeErrorResponse(w, "Failed to rename conversation", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Conversation renamed successfully",
		"title":   newTitle,
	})
}

// HandleConversationListHTMX returns user's conversations as HTMX partial
func (h *ChatHandler) HandleConversationListHTMX(w http.ResponseWriter, r *http.Request) {
	user := getUserFromHTTPContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<div class="error">Please log in to view conversations</div>`))
		return
	}

	ctx := r.Context()
	conversations, err := h.convService.GetUserConversations(ctx, user.ID, 50, 0)
	if err != nil {
		log.Printf("Failed to get conversations: %v", err)
		w.Write([]byte(`<div class="error">Failed to load conversations</div>`))
		return
	}

	html := ""
	for _, conv := range conversations {
		// Get first message for preview
		_, messages, err := h.convService.GetConversationWithMessages(ctx, conv.ID)
		preview := "No messages yet"
		if err == nil && len(messages) > 0 {
			content := messages[0].Content
			if len(content) > 60 {
				content = content[:60] + "..."
			}
			preview = content
		}

		// Format time
		timeFormatted := h.formatTimeAgo(conv.UpdatedAt)
		
		html += fmt.Sprintf(`
			<div class="conversation-item" onclick="loadConversation('%s')" data-conversation-id="%s">
				<div class="conversation-title">%s</div>
				<div class="conversation-preview">%s</div>
				<div class="conversation-time">%s</div>
				<div class="conversation-actions" style="display: none;">
					<button onclick="event.stopPropagation(); renameConversation('%s', '%s')" title="Rename">
						✏️
					</button>
					<button onclick="event.stopPropagation(); deleteConversation('%s')" title="Delete">
						🗑️
					</button>
				</div>
			</div>
		`, conv.ID, conv.ID, h.escapeHTML(conv.Title), h.escapeHTML(preview), timeFormatted, conv.ID, h.escapeHTML(conv.Title), conv.ID)
	}

	if html == "" {
		html = `
			<div style="padding: 2rem 1rem; text-align: center; color: #718096;">
				<div style="margin-bottom: 1rem;">💬</div>
				<p style="font-size: 0.875rem;">Start a new conversation with our AI Chef!</p>
			</div>
		`
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// HandleConversationStats returns conversation statistics for a user
func (h *ChatHandler) HandleConversationStats(w http.ResponseWriter, r *http.Request) {
	user := getUserFromHTTPContext(r.Context())
	if user == nil {
		h.writeErrorResponse(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	stats, err := h.convService.GetConversationStats(ctx, user.ID)
	if err != nil {
		log.Printf("Failed to get conversation stats: %v", err)
		h.writeErrorResponse(w, "Failed to retrieve statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}

// writeErrorResponse writes an error response
func (h *ChatHandler) writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(HTTPChatResponse{
		Success: false,
		Error:   message,
	})
}

// getUserFromHTTPContext extracts user from HTTP context
func getUserFromHTTPContext(ctx context.Context) *User {
	if user, ok := ctx.Value("user").(*User); ok {
		return user
	}
	return nil
}

// renderHTMLTemplate renders an HTML template (placeholder - you'll need to implement this)
func renderHTMLTemplate(w http.ResponseWriter, templateName string, data interface{}) error {
	// This should use your existing template rendering system
	// For now, return a simple implementation
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// You'll need to integrate this with your existing template system
	// from the main.go file
	return fmt.Errorf("template rendering not implemented - integrate with existing system")
}

// Enhanced AI Chat with Recipe Intent Detection
func (h *ChatHandler) HandleAIChatHTMX(w http.ResponseWriter, r *http.Request) {
	message := r.FormValue("message")
	conversationID := r.FormValue("conversation_id")
	user := getUserFromHTTPContext(r.Context())

	if message == "" {
		h.writeHTMXError(w, "Message cannot be empty")
		return
	}

	ctx := r.Context()

	// Handle conversation creation or continuation
	if conversationID == "" && user != nil {
		// Create new conversation
		conv, err := h.convService.CreateConversation(ctx, user.ID, message)
		if err != nil {
			log.Printf("Failed to create conversation: %v", err)
			h.writeHTMXError(w, "Failed to create conversation")
			return
		}
		conversationID = conv.ID
	}

	var response string

	if user != nil && conversationID != "" {
		// Process message with conversation service
		_, aiResponse, err := h.convService.ProcessMessage(ctx, conversationID, message, user.ID)
		if err != nil {
			log.Printf("Failed to process message: %v", err)
			response = "I apologize, but I encountered an error processing your message. Please try again."
		} else {
			response = aiResponse
		}
	} else if user == nil {
		// Not authenticated
		response = `I'd love to help you create recipes! However, you need to be logged in to have conversations with me. 
		<div style="margin-top: 1rem;">
			<a href="/login" class="btn btn-primary">Login</a>
			<a href="/register" class="btn btn-secondary">Register</a>
		</div>`
	} else {
		// Generic response
		response = "I'm your AI chef assistant! Ask me to create recipes, answer cooking questions, or help with meal planning."
	}

	// Create user message HTML
	userName := "Anonymous"
	if user != nil {
		userName = user.Name
	}

	userMessageHTML := fmt.Sprintf(`
		<div class="message user">
			<div class="message-avatar user">👤</div>
			<div class="message-content">
				<div>%s</div>
				<div class="message-meta">
					<span>%s</span>
					<span>Just now</span>
				</div>
			</div>
		</div>`, h.escapeHTML(message), h.escapeHTML(userName))

	// Create AI response HTML
	aiMessageHTML := fmt.Sprintf(`
		<div class="message assistant">
			<div class="message-avatar assistant">👨‍🍳</div>
			<div class="message-content">
				<div>%s</div>
				<div class="message-meta">
					<span>AI Chef</span>
					<span>Just now</span>
				</div>
			</div>
		</div>`, response)

	// Combine messages
	fullHTML := userMessageHTML + aiMessageHTML

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(fullHTML))
}

// writeHTMXError writes an HTMX-compatible error response
func (h *ChatHandler) writeHTMXError(w http.ResponseWriter, message string) {
	html := fmt.Sprintf(`
		<div class="message assistant">
			<div class="message-avatar assistant">👨‍🍳</div>
			<div class="message-content">
				<div style="color: #e53e3e;">❌ %s</div>
				<div class="message-meta">
					<span>AI Chef</span>
					<span>Just now</span>
				</div>
			</div>
		</div>`, h.escapeHTML(message))
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// formatTimeAgo formats a timestamp to a human-readable relative time
func (h *ChatHandler) formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	
	if diff < time.Minute {
		return "Just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else {
		return t.Format("Jan 2, 2006")
	}
}


// escapeHTML escapes HTML characters to prevent XSS
func (h *ChatHandler) escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}