// Package ai provides DeepSeek API client for AI completions
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"go.uber.org/zap"
)

// DeepSeekClient provides access to DeepSeek API
type DeepSeekClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *zap.Logger
}

// DeepSeekRequest represents a DeepSeek API request
type DeepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []DeepSeekMessage `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream"`
	TopP        float64           `json:"top_p,omitempty"`
	Stop        []string          `json:"stop,omitempty"`
}

// DeepSeekMessage represents a message in the conversation
type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DeepSeekResponse represents a DeepSeek API response
type DeepSeekResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []DeepSeekChoice `json:"choices"`
	Usage   DeepSeekUsage    `json:"usage"`
}

// DeepSeekChoice represents a choice in the response
type DeepSeekChoice struct {
	Index        int             `json:"index"`
	Message      DeepSeekMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

// DeepSeekUsage represents token usage information
type DeepSeekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// DeepSeekError represents an API error response
type DeepSeekError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewDeepSeekClient creates a new DeepSeek API client
func NewDeepSeekClient(apiKey, baseURL, model string, timeout time.Duration, logger *zap.Logger) *DeepSeekClient {
	return &DeepSeekClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		logger: logger.Named("deepseek_client"),
	}
}

// GenerateChatCompletion generates a chat completion using DeepSeek API
func (c *DeepSeekClient) GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage, temperature float64, maxTokens int) (*conversation.GenerationResult, error) {
	startTime := time.Now()

	// Convert messages to DeepSeek format
	deepSeekMessages := make([]DeepSeekMessage, len(messages))
	for i, msg := range messages {
		deepSeekMessages[i] = DeepSeekMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Prepare request
	request := DeepSeekRequest{
		Model:       c.model,
		Messages:    deepSeekMessages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
		Stream:      false,
	}

	// Marshal request to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	c.logger.Debug("Sending DeepSeek API request",
		zap.String("model", c.model),
		zap.Int("messages", len(messages)),
		zap.Float64("temperature", temperature),
		zap.Int("max_tokens", maxTokens))

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("User-Agent", "Alchemorsel/3.0.0")

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var deepSeekErr DeepSeekError
		if err := json.Unmarshal(body, &deepSeekErr); err == nil {
			return nil, fmt.Errorf("DeepSeek API error (%d): %s", resp.StatusCode, deepSeekErr.Error.Message)
		}
		return nil, fmt.Errorf("DeepSeek API error (%d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response DeepSeekResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Validate response
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := response.Choices[0]
	duration := time.Since(startTime)

	// Calculate quality score based on response characteristics
	quality := c.calculateQualityScore(choice.Message.Content, choice.FinishReason, duration)

	result := &conversation.GenerationResult{
		Content:    choice.Message.Content,
		Quality:    quality,
		Duration:   duration,
		TokensUsed: response.Usage.TotalTokens,
		ModelUsed:  response.Model,
		Metadata: map[string]interface{}{
			"finish_reason":     choice.FinishReason,
			"prompt_tokens":     response.Usage.PromptTokens,
			"completion_tokens": response.Usage.CompletionTokens,
			"api_response_id":   response.ID,
			"created":           response.Created,
		},
	}

	c.logger.Debug("DeepSeek generation completed",
		zap.String("model", response.Model),
		zap.Duration("duration", duration),
		zap.Int("tokens_used", response.Usage.TotalTokens),
		zap.Float64("quality", quality),
		zap.String("finish_reason", choice.FinishReason))

	return result, nil
}

// calculateQualityScore calculates a quality score based on response characteristics
func (c *DeepSeekClient) calculateQualityScore(content, finishReason string, duration time.Duration) float64 {
	score := 0.8 // Base score for DeepSeek

	// Adjust based on finish reason
	switch finishReason {
	case "stop":
		score += 0.1
	case "length":
		score -= 0.1
	case "content_filter":
		score -= 0.3
	}

	// Adjust based on content length (reasonable responses get higher scores)
	contentLength := len(content)
	if contentLength > 50 && contentLength < 2000 {
		score += 0.1
	} else if contentLength < 20 {
		score -= 0.2
	}

	// Adjust based on response time (faster responses get slightly higher scores)
	if duration < 3*time.Second {
		score += 0.05
	} else if duration > 10*time.Second {
		score -= 0.1
	}

	// Ensure score is within bounds
	if score > 1.0 {
		score = 1.0
	} else if score < 0.0 {
		score = 0.0
	}

	return score
}

// TestConnection tests the connection to DeepSeek API
func (c *DeepSeekClient) TestConnection(ctx context.Context) error {
	testMessages := []conversation.ChatMessage{
		{
			Role:    "user",
			Content: "Hello! Please respond with a simple greeting.",
		},
	}

	result, err := c.GenerateChatCompletion(ctx, testMessages, 0.1, 50)
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	c.logger.Info("DeepSeek connection test successful",
		zap.String("response", result.Content),
		zap.Duration("duration", result.Duration))

	return nil
}

// GetModelInfo returns information about the configured model
func (c *DeepSeekClient) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider": "deepseek",
		"model":    c.model,
		"base_url": c.baseURL,
		"timeout":  c.httpClient.Timeout,
		"features": []string{"chat", "streaming", "function_calling"},
	}
}
