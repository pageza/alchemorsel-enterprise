// Package ai provides enhanced AI client functionality with caching and metrics
package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"go.uber.org/zap"
)

// EnhancedOllamaClient wraps the basic OllamaClient with additional features
type EnhancedOllamaClient struct {
	baseClient       conversation.OllamaClient
	modelManager     *conversation.ModelManager
	responseCache    *ResponseCache
	streamHandler    *StreamHandler
	metricsCollector *MetricsCollector
	logger           *zap.Logger
}

// StreamHandler manages streaming response handling
type StreamHandler struct {
	logger *zap.Logger
}

// NewEnhancedOllamaClient creates a new enhanced Ollama client
func NewEnhancedOllamaClient(
	baseClient conversation.OllamaClient,
	modelManager *conversation.ModelManager,
	responseCache *ResponseCache,
	streamHandler *StreamHandler,
	metricsCollector *MetricsCollector,
) *EnhancedOllamaClient {
	return &EnhancedOllamaClient{
		baseClient:       baseClient,
		modelManager:     modelManager,
		responseCache:    responseCache,
		streamHandler:    streamHandler,
		metricsCollector: metricsCollector,
		logger:           zap.L().Named("enhanced_ollama_client"),
	}
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(logger *zap.Logger) *StreamHandler {
	return &StreamHandler{
		logger: logger.Named("stream_handler"),
	}
}

// GenerateCompletion generates a text completion using optimal model selection
func (c *EnhancedOllamaClient) GenerateCompletion(ctx context.Context, prompt string) (string, error) {
	// Use default context for simple completion
	defaultContext := conversation.ConversationContext{
		Complexity: "medium",
	}
	
	options := conversation.GenerationOptions{
		Intent:      conversation.IntentGeneral,
		Context:     defaultContext,
		Streaming:   false,
		MaxTokens:   1500,
		Temperature: 0.7,
		Quality:     conversation.QualityBalanced,
	}
	
	// Convert prompt to messages format
	messages := []conversation.ChatMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}
	
	result, err := c.GenerateChatCompletionWithOptions(ctx, messages, options)
	if err != nil {
		return "", err
	}
	
	return result.Content, nil
}

// GenerateChatCompletionWithOptions generates a chat completion with enhanced options
func (c *EnhancedOllamaClient) GenerateChatCompletionWithOptions(
	ctx context.Context,
	messages []conversation.ChatMessage,
	options conversation.GenerationOptions,
) (*conversation.GenerationResult, error) {
	
	// Select optimal model
	selectedModel, err := c.modelManager.SelectOptimalModel(options.Intent, options.Context)
	if err != nil {
		c.logger.Warn("Failed to select optimal model, using fallback",
			zap.Error(err),
			zap.String("fallback", c.modelManager.GetFallbackModel()))
		selectedModel = c.modelManager.GetFallbackModel()
	}
	
	// Check cache first
	if c.responseCache != nil {
		if cached := c.responseCache.Get(ctx, messages, selectedModel); cached != nil {
			c.logger.Debug("Cache hit for completion request",
				zap.String("model", selectedModel),
				zap.Int("message_count", len(messages)))
			
			// Record cache hit metric
			if c.metricsCollector != nil {
				c.metricsCollector.RecordCacheHit(selectedModel)
			}
			
			return cached, nil
		}
	}
	
	// Generate with telemetry
	startTime := time.Now()
	result, err := c.generateWithModel(ctx, messages, selectedModel, options)
	duration := time.Since(startTime)
	
	// Update model metrics
	if result != nil {
		c.modelManager.UpdateModelMetrics(selectedModel, duration, result.TokensUsed, result.Quality)
	}
	
	// Collect performance metrics
	if c.metricsCollector != nil {
		c.metricsCollector.RecordGeneration(selectedModel, duration, result.TokensUsed, err)
	}
	
	// Cache successful high-quality results
	if err == nil && result.Quality > 0.8 && c.responseCache != nil {
		cacheTTL := 5 * time.Minute
		if options.Quality == conversation.QualityHigh {
			cacheTTL = 15 * time.Minute // Cache high-quality responses longer
		}
		
		if cacheErr := c.responseCache.Set(ctx, messages, selectedModel, result, cacheTTL); cacheErr != nil {
			c.logger.Warn("Failed to cache response", zap.Error(cacheErr))
		}
	}
	
	return result, err
}

// generateWithModel performs the actual generation with a specific model
func (c *EnhancedOllamaClient) generateWithModel(
	ctx context.Context,
	messages []conversation.ChatMessage,
	model string,
	options conversation.GenerationOptions,
) (*conversation.GenerationResult, error) {
	
	c.logger.Debug("Generating completion with model",
		zap.String("model", model),
		zap.String("intent", string(options.Intent)),
		zap.Bool("streaming", options.Streaming),
		zap.String("quality", string(options.Quality)))
	
	// Convert messages to the format expected by base client
	prompt := c.formatMessagesAsPrompt(messages)
	
	startTime := time.Now()
	
	// Call the base client
	response, err := c.baseClient.GenerateCompletion(ctx, prompt)
	if err != nil {
		c.logger.Error("Generation failed",
			zap.String("model", model),
			zap.Error(err))
		
		// Try fallback model if available
		fallbackModel := c.modelManager.GetFallbackModel()
		if fallbackModel != model {
			c.logger.Info("Retrying with fallback model",
				zap.String("fallback_model", fallbackModel))
			
			// Update health status for failed model
			c.modelManager.UpdateModelHealth(model, "failed")
			
			return c.generateWithModel(ctx, messages, fallbackModel, options)
		}
		
		return nil, fmt.Errorf("generation failed with model %s: %w", model, err)
	}
	
	duration := time.Since(startTime)
	
	// Calculate quality score (simplified heuristic)
	quality := c.calculateQualityScore(response, options)
	
	// Estimate token usage (rough approximation)
	tokensUsed := len(response) / 4 // Rough approximation: 4 chars per token
	
	result := &conversation.GenerationResult{
		Content:    response,
		Quality:    quality,
		Duration:   duration,
		TokensUsed: tokensUsed,
		ModelUsed:  model,
		Metadata: map[string]interface{}{
			"temperature":    options.Temperature,
			"max_tokens":     options.MaxTokens,
			"streaming":      options.Streaming,
			"quality_level":  string(options.Quality),
			"intent":         string(options.Intent),
		},
	}
	
	c.logger.Debug("Generation completed",
		zap.String("model", model),
		zap.Duration("duration", duration),
		zap.Int("tokens_used", tokensUsed),
		zap.Float64("quality", quality))
	
	return result, nil
}

// formatMessagesAsPrompt converts chat messages to a single prompt string
func (c *EnhancedOllamaClient) formatMessagesAsPrompt(messages []conversation.ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	
	// Simple formatting - in production, this would be more sophisticated
	var prompt string
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			prompt += fmt.Sprintf("System: %s\n\n", msg.Content)
		case "user":
			prompt += fmt.Sprintf("Human: %s\n\n", msg.Content)
		case "assistant":
			prompt += fmt.Sprintf("Assistant: %s\n\n", msg.Content)
		}
	}
	
	// Add final assistant prompt
	if len(messages) > 0 && messages[len(messages)-1].Role != "assistant" {
		prompt += "Assistant: "
	}
	
	return prompt
}

// calculateQualityScore calculates a quality score for the response
func (c *EnhancedOllamaClient) calculateQualityScore(response string, options conversation.GenerationOptions) float64 {
	if response == "" {
		return 0.0
	}
	
	score := 0.5 // Base score
	
	// Length-based scoring
	responseLength := len(response)
	if responseLength > 50 {
		score += 0.1
	}
	if responseLength > 200 {
		score += 0.1
	}
	if responseLength > 500 {
		score += 0.1
	}
	
	// Coherence indicators (simple heuristics)
	if responseLength > 20 {
		// Check for sentence structure
		if response[len(response)-1] == '.' || response[len(response)-1] == '!' || response[len(response)-1] == '?' {
			score += 0.1
		}
	}
	
	// Intent-specific scoring
	switch options.Intent {
	case conversation.IntentRecipeCreation:
		// Look for recipe-like structure
		if containsRecipeKeywords(response) {
			score += 0.1
		}
	case conversation.IntentCookingHelp:
		// Look for helpful cooking advice
		if containsHelpfulKeywords(response) {
			score += 0.1
		}
	}
	
	// Ensure score is within bounds
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	
	return score
}

// containsRecipeKeywords checks if response contains recipe-related keywords
func containsRecipeKeywords(response string) bool {
	keywords := []string{"ingredients", "recipe", "cook", "bake", "serve", "instructions", "step"}
	responseLower := response
	for _, keyword := range keywords {
		if len(responseLower) > len(keyword) && containsKeyword(responseLower, keyword) {
			return true
		}
	}
	return false
}

// containsHelpfulKeywords checks if response contains helpful keywords
func containsHelpfulKeywords(response string) bool {
	keywords := []string{"help", "try", "suggest", "recommend", "tip", "advice"}
	responseLower := response
	for _, keyword := range keywords {
		if containsKeyword(responseLower, keyword) {
			return true
		}
	}
	return false
}

// containsKeyword checks if a string contains a keyword (case-insensitive)
func containsKeyword(text, keyword string) bool {
	// Simple substring search - in production, use proper text analysis
	textLen := len(text)
	keywordLen := len(keyword)
	
	if keywordLen > textLen {
		return false
	}
	
	for i := 0; i <= textLen-keywordLen; i++ {
		match := true
		for j := 0; j < keywordLen; j++ {
			if text[i+j] != keyword[j] && text[i+j] != keyword[j]+32 && text[i+j] != keyword[j]-32 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	
	return false
}

// GetModelInfo returns information about available models
func (c *EnhancedOllamaClient) GetModelInfo() []*conversation.ModelInfo {
	return c.modelManager.ListModels()
}

// Shutdown gracefully shuts down the enhanced client
func (c *EnhancedOllamaClient) Shutdown(ctx context.Context) error {
	c.logger.Info("Shutting down enhanced Ollama client")
	
	if c.modelManager != nil {
		if err := c.modelManager.Shutdown(ctx); err != nil {
			c.logger.Error("Failed to shutdown model manager", zap.Error(err))
			return err
		}
	}
	
	return nil
}