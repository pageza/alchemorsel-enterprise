package ai

import (
	"context"
	"fmt"
	"os"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"go.uber.org/zap"
)

// ConversationOllamaAdapter adapts the existing OllamaClient to the conversation service interface
type ConversationOllamaAdapter struct {
	client *OllamaClient
}

// NewConversationOllamaAdapter creates a new adapter for conversation service
func NewConversationOllamaAdapter(baseURL, model string) *ConversationOllamaAdapter {
	return &ConversationOllamaAdapter{
		client: NewOllamaClient(baseURL, model),
	}
}

// GenerateChatCompletion implements conversation.OllamaClient interface
func (a *ConversationOllamaAdapter) GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage) (string, error) {
	// Use default parameters for conversation use case
	result, err := a.client.GenerateChatCompletion(ctx, messages, 0.7, 2000)
	if err != nil {
		return "", err
	}
	
	return result.Content, nil
}

// HealthCheck implements conversation.OllamaClient interface
func (a *ConversationOllamaAdapter) HealthCheck(ctx context.Context) error {
	return a.client.HealthCheck(ctx)
}

// ConversationOpenAIAdapter adapts OpenAI client to conversation service interface
type ConversationOpenAIAdapter struct {
	logger *zap.Logger
	hasKey bool
}

// NewConversationOpenAIAdapter creates a new adapter for conversation service
func NewConversationOpenAIAdapter(logger *zap.Logger) *ConversationOpenAIAdapter {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ALCHEMORSEL_AI_OPENAI_KEY")
	}
	
	return &ConversationOpenAIAdapter{
		logger: logger,
		hasKey: apiKey != "",
	}
}

// GenerateChatCompletion implements conversation.OpenAIClient interface
func (a *ConversationOpenAIAdapter) GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage) (string, error) {
	if !a.hasKey {
		return "", fmt.Errorf("OpenAI API key not configured")
	}
	
	// For now, return a fallback message until we implement full OpenAI integration
	return "I'm sorry, but OpenAI integration is not fully implemented yet. Please use the local AI model.", nil
}