package handlers

import (
	"context"
	"log"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/ai"
	"github.com/alchemorsel/v3/internal/infrastructure/websocket"
)

// ChatMessageHandler handles WebSocket chat messages and integrates with AI service
type ChatMessageHandler struct {
	ollamaClient *ai.OllamaClient
	wsManager    *websocket.Manager
}

// NewChatMessageHandler creates a new chat message handler
func NewChatMessageHandler(ollamaClient *ai.OllamaClient, wsManager *websocket.Manager) *ChatMessageHandler {
	return &ChatMessageHandler{
		ollamaClient: ollamaClient,
		wsManager:    wsManager,
	}
}

// HandleMessage processes incoming chat messages
func (h *ChatMessageHandler) HandleMessage(ctx context.Context, userID string, message websocket.Message) error {
	log.Printf("Processing chat message from user %s: %s", userID, message.Content)

	// For now, we'll create a simple conversation flow without database
	// This will be updated when we have proper conversation persistence

	// Create chat history for AI context
	messages := []conversation.ChatMessage{
		{
			Role:    "system",
			Content: "You are an expert AI chef assistant helping users with cooking and recipes. You are knowledgeable, friendly, and practical. Always provide helpful, accurate, and safe cooking advice.",
		},
		{
			Role:    "user",
			Content: message.Content,
		},
	}

	// Generate AI response using Ollama
	log.Printf("About to call Ollama GenerateChatCompletion for user %s", userID)
	log.Printf("Messages being sent to Ollama: %+v", messages)
	aiResult, err := h.ollamaClient.GenerateChatCompletion(ctx, messages, 0.7, 2048)
	var aiContent string
	if err != nil {
		log.Printf("Ollama generation failed: %v", err)
		log.Printf("Error type: %T", err)
		
		// Send fallback response
		aiContent = "I'm having trouble connecting to my AI chef brain right now! 🧠 Could you try asking again in a moment? In the meantime, I'm here to help with any cooking questions you have!"
	} else {
		log.Printf("Ollama generation successful for user %s", userID)
		log.Printf("AI Response: %s", aiResult.Content)
		aiContent = aiResult.Content
	}

	// Send response back to user
	responseMessage := websocket.Message{
		ID:             message.ID,
		Type:           "chat_response",
		ConversationID: "", // Will be set when we have conversation persistence
		Content:        aiContent,
		Role:           "assistant",
		Metadata:       map[string]interface{}{
			"provider": "ollama",
			"model":    "phi3:mini",
		},
	}

	// Send to specific user
	err = h.wsManager.SendToUser(userID, responseMessage)
	if err != nil {
		log.Printf("Failed to send response to user %s: %v", userID, err)
		return err
	}

	log.Printf("Successfully sent AI response to user %s", userID)
	return nil
}