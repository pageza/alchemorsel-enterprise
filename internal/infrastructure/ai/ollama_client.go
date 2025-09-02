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
)

// OllamaClient implements the OllamaClient interface for the conversation service
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	model      string
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL, model string) *OllamaClient {
	return &OllamaClient{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// OllamaRequest represents a request to Ollama
type OllamaRequest struct {
	Model    string                         `json:"model"`
	Messages []conversation.ChatMessage     `json:"messages"`
	Stream   bool                          `json:"stream"`
	Options  map[string]interface{}        `json:"options,omitempty"`
}

// OllamaResponse represents a response from Ollama
type OllamaResponse struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// GenerateChatCompletion generates a chat completion using Ollama
func (c *OllamaClient) GenerateChatCompletion(ctx context.Context, messages []conversation.ChatMessage, temperature float64, maxTokens int) (*conversation.GenerationResult, error) {
	startTime := time.Now()

	request := OllamaRequest{
		Model:    c.model,
		Messages: messages,
		Stream:   false,
		Options: map[string]interface{}{
			"temperature":     temperature,
			"max_tokens":      maxTokens,
			"top_p":          0.9,
			"frequency_penalty": 0.0,
			"presence_penalty":  0.0,
		},
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	fmt.Printf("DEBUG: OllamaClient making request to URL: %s with model: %s\n", url, c.model)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("DEBUG: About to make HTTP request to %s\n", url)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("DEBUG: HTTP request failed: %v\n", err)
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	duration := time.Since(startTime)

	return &conversation.GenerationResult{
		Content:    ollamaResp.Message.Content,
		Quality:    0.8, // Default quality score for Ollama
		Duration:   duration,
		TokensUsed: len(ollamaResp.Message.Content) / 4, // Rough token estimation
		ModelUsed:  c.model,
		Metadata: map[string]interface{}{
			"provider": "ollama",
			"temperature": temperature,
			"max_tokens": maxTokens,
		},
	}, nil
}

// HealthCheck checks if Ollama is healthy and responsive
func (c *OllamaClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/tags", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check returned status %d", resp.StatusCode)
	}

	return nil
}

// GetModels retrieves available models from Ollama
func (c *OllamaClient) GetModels(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/api/tags", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create models request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read models response: %w", err)
	}

	var modelsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal models response: %w", err)
	}

	models := make([]string, len(modelsResp.Models))
	for i, model := range modelsResp.Models {
		models[i] = model.Name
	}

	return models, nil
}

// TestConnection tests if the Ollama service is accessible
func (c *OllamaClient) TestConnection(ctx context.Context) error {
	return c.HealthCheck(ctx)
}

// GetModelInfo returns information about the configured Ollama model
func (c *OllamaClient) GetModelInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider": "ollama",
		"model":    c.model,
		"base_url": c.baseURL,
		"timeout":  c.httpClient.Timeout,
	}
}