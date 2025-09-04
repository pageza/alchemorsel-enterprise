package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"go.uber.org/zap"
)

// ConversationAIAdapter adapts existing AI services for conversation use
type ConversationAIAdapter struct {
	baseURL string
	model   string
	client  *http.Client
	logger  *zap.Logger
	timeout time.Duration
}

// NewConversationAIAdapter creates a new adapter
func NewConversationAIAdapter(logger *zap.Logger) *ConversationAIAdapter {
	// Get configuration from environment variables
	baseURL := os.Getenv("ALCHEMORSEL_OLLAMA_HOST")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	model := os.Getenv("ALCHEMORSEL_OLLAMA_MODEL")
	if model == "" {
		model = "llama3.2:3b"
	}

	timeout := 60 * time.Second // Longer timeout for conversation
	if timeoutStr := os.Getenv("ALCHEMORSEL_OLLAMA_TIMEOUT"); timeoutStr != "" {
		if parsedTimeout, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = parsedTimeout
		}
	}

	logger.Info("Conversation AI adapter initialized",
		zap.String("base_url", baseURL),
		zap.String("model", model),
		zap.Duration("timeout", timeout))

	return &ConversationAIAdapter{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: timeout,
		},
		logger:  logger.Named("conversation-ai"),
		timeout: timeout,
	}
}

// GenerateChatCompletion generates a chat completion using Ollama
func (a *ConversationAIAdapter) GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage) (string, error) {
	endpoint := a.baseURL + "/api/chat"

	// Convert conversation messages to Ollama format
	ollamaMessages := make([]OllamaChatMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = OllamaChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := OllamaChatRequest{
		Model:    a.model,
		Messages: ollamaMessages,
		Stream:   false,
		Options: map[string]interface{}{
			"temperature":    0.7,
			"num_predict":    1500, // Allow longer responses for conversation
			"stop":           []string{},
			"num_ctx":        8192, // Larger context window for conversation history
			"repeat_penalty": 1.1,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	a.logger.Debug("Sending chat completion request",
		zap.String("endpoint", endpoint),
		zap.Int("message_count", len(messages)))

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		a.logger.Error("Ollama API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)))
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var chatResp OllamaChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if !chatResp.Done {
		return "", fmt.Errorf("incomplete response from Ollama")
	}

	a.logger.Debug("Chat completion successful",
		zap.String("model", chatResp.Model),
		zap.Int64("eval_duration", chatResp.EvalDuration),
		zap.Int("eval_count", chatResp.EvalCount),
		zap.Int("response_length", len(chatResp.Message.Content)))

	return chatResp.Message.Content, nil
}

// HealthCheck checks if Ollama service is available
func (a *ConversationAIAdapter) HealthCheck(ctx context.Context) error {
	endpoint := a.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check failed with status %d", resp.StatusCode)
	}

	a.logger.Debug("Ollama health check passed")
	return nil
}

// GenerateStreamingResponse generates a streaming response (future enhancement)
func (a *ConversationAIAdapter) GenerateStreamingResponse(ctx context.Context, messages []conversation.ChatMessage, callback func(chunk string) error) error {
	endpoint := a.baseURL + "/api/chat"

	// Convert conversation messages to Ollama format
	ollamaMessages := make([]OllamaChatMessage, len(messages))
	for i, msg := range messages {
		ollamaMessages[i] = OllamaChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := OllamaChatRequest{
		Model:    a.model,
		Messages: ollamaMessages,
		Stream:   true, // Enable streaming
		Options: map[string]interface{}{
			"temperature":    0.7,
			"num_predict":    1500,
			"num_ctx":        8192,
			"repeat_penalty": 1.1,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error %d", resp.StatusCode)
	}

	// Read streaming response line by line
	decoder := json.NewDecoder(resp.Body)

	for decoder.More() {
		var streamResp OllamaChatResponse
		if err := decoder.Decode(&streamResp); err != nil {
			return fmt.Errorf("failed to decode streaming response: %w", err)
		}

		// Send chunk to callback
		if streamResp.Message.Content != "" {
			if err := callback(streamResp.Message.Content); err != nil {
				return fmt.Errorf("streaming callback error: %w", err)
			}
		}

		// Check if done
		if streamResp.Done {
			break
		}
	}

	return nil
}

// Ollama API structures for conversation
type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaChatRequest struct {
	Model    string                 `json:"model"`
	Messages []OllamaChatMessage    `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type OllamaChatResponse struct {
	Model              string            `json:"model"`
	Message            OllamaChatMessage `json:"message"`
	Done               bool              `json:"done"`
	TotalDuration      int64             `json:"total_duration,omitempty"`
	LoadDuration       int64             `json:"load_duration,omitempty"`
	PromptEvalCount    int               `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64             `json:"prompt_eval_duration,omitempty"`
	EvalCount          int               `json:"eval_count,omitempty"`
	EvalDuration       int64             `json:"eval_duration,omitempty"`
}

// OpenAIAdapter provides OpenAI fallback functionality
type OpenAIAdapter struct {
	apiKey string
	client *http.Client
	logger *zap.Logger
}

// NewOpenAIAdapter creates a new OpenAI adapter
func NewOpenAIAdapter(logger *zap.Logger) *OpenAIAdapter {
	apiKey := os.Getenv("OPENAI_API_KEY")

	return &OpenAIAdapter{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger.Named("openai-adapter"),
	}
}

// GenerateChatCompletion generates a chat completion using OpenAI API
func (a *OpenAIAdapter) GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	endpoint := "https://api.openai.com/v1/chat/completions"

	// Convert to OpenAI format
	openAIMessages := make([]OpenAIChatMessage, len(messages))
	for i, msg := range messages {
		openAIMessages[i] = OpenAIChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := OpenAIChatRequest{
		Model:       "gpt-3.5-turbo",
		Messages:    openAIMessages,
		MaxTokens:   1500,
		Temperature: 0.7,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var chatResp OpenAIChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// OpenAI API structures
type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenAIChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float64             `json:"temperature"`
}

type OpenAIChatResponse struct {
	Choices []struct {
		Message OpenAIChatMessage `json:"message"`
	} `json:"choices"`
}
