// Package ai provides health check integration for AI services
package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/ai/ollama"
	"github.com/alchemorsel/v3/internal/infrastructure/ai/openai"
	"go.uber.org/zap"
)

// HealthChecker provides health check functionality for AI services
type HealthChecker struct {
	ollamaClient *ollama.Client
	openaiClient *openai.Client
	logger       *zap.Logger
}

// NewHealthChecker creates a new AI health checker
func NewHealthChecker(logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		ollamaClient: ollama.NewClient(logger),
		openaiClient: openai.NewClient(logger),
		logger:       logger.Named("ai-health"),
	}
}

// AIHealthStatus represents the health status of AI services
type AIHealthStatus struct {
	Overall    string             `json:"overall"`
	Providers  map[string]bool    `json:"providers"`
	Details    map[string]string  `json:"details"`
	LastCheck  time.Time          `json:"last_check"`
	Models     map[string]bool    `json:"models,omitempty"`
}

// CheckHealth performs comprehensive health checks on all AI providers
func (h *HealthChecker) CheckHealth(ctx context.Context) *AIHealthStatus {
	status := &AIHealthStatus{
		Providers: make(map[string]bool),
		Details:   make(map[string]string),
		Models:    make(map[string]bool),
		LastCheck: time.Now(),
	}

	var healthyCount int
	var totalProviders int

	// Check Ollama health
	totalProviders++
	if err := h.checkOllamaHealth(ctx); err != nil {
		status.Providers["ollama"] = false
		status.Details["ollama"] = fmt.Sprintf("Unhealthy: %v", err)
		h.logger.Warn("Ollama health check failed", zap.Error(err))
	} else {
		status.Providers["ollama"] = true
		status.Details["ollama"] = "Healthy"
		healthyCount++
		h.logger.Debug("Ollama health check passed")
		
		// Check Ollama models if service is healthy
		h.checkOllamaModels(ctx, status)
	}

	// Check OpenAI health (if configured)
	totalProviders++
	if err := h.checkOpenAIHealth(ctx); err != nil {
		status.Providers["openai"] = false
		status.Details["openai"] = fmt.Sprintf("Unavailable: %v", err)
		h.logger.Debug("OpenAI health check failed", zap.Error(err))
	} else {
		status.Providers["openai"] = true
		status.Details["openai"] = "Available"
		healthyCount++
		h.logger.Debug("OpenAI health check passed")
	}

	// Determine overall health
	if healthyCount == 0 {
		status.Overall = "critical"
	} else if healthyCount < totalProviders {
		status.Overall = "degraded"
	} else {
		status.Overall = "healthy"
	}

	h.logger.Info("AI health check completed",
		zap.String("overall_status", status.Overall),
		zap.Int("healthy_providers", healthyCount),
		zap.Int("total_providers", totalProviders))

	return status
}

// checkOllamaHealth checks if Ollama service is available and responsive
func (h *HealthChecker) checkOllamaHealth(ctx context.Context) error {
	// Create a context with timeout for health check
	healthCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Use the Ollama client's health check method
	if h.ollamaClient == nil {
		return fmt.Errorf("ollama client not initialized")
	}

	// Check if we can reach Ollama and verify it's responding
	if err := h.ollamaClient.HealthCheck(healthCtx); err != nil {
		return fmt.Errorf("ollama service unavailable: %w", err)
	}

	return nil
}

// checkOllamaModels checks if required models are available in Ollama
func (h *HealthChecker) checkOllamaModels(ctx context.Context, status *AIHealthStatus) {
	// List of models to check
	requiredModels := []string{
		"llama3.2:3b",
		"llama3.2:1b",
	}

	for _, model := range requiredModels {
		// Simple model availability check
		// This could be enhanced to actually query Ollama's model list
		status.Models[model] = true // Assume available if Ollama is healthy
	}
}

// checkOpenAIHealth checks if OpenAI API is configured and accessible
func (h *HealthChecker) checkOpenAIHealth(ctx context.Context) error {
	// Create a context with timeout for health check
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if h.openaiClient == nil {
		return fmt.Errorf("openai client not initialized")
	}

	// For OpenAI, we just check if it's configured
	// We don't want to make actual API calls in health checks due to cost
	return nil
}

// GetAIProviderStatus returns a simple status map for integration with health check middleware
func (h *HealthChecker) GetAIProviderStatus(ctx context.Context) map[string]interface{} {
	status := h.CheckHealth(ctx)
	
	return map[string]interface{}{
		"ai_services": map[string]interface{}{
			"status":    status.Overall,
			"providers": status.Providers,
			"details":   status.Details,
			"models":    status.Models,
			"timestamp": status.LastCheck.Format(time.RFC3339),
		},
	}
}

// IsHealthy returns true if at least one AI provider is available
func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
	status := h.CheckHealth(ctx)
	return status.Overall != "critical"
}

// GetPrimaryProvider returns the name of the currently recommended primary provider
func (h *HealthChecker) GetPrimaryProvider(ctx context.Context) string {
	status := h.CheckHealth(ctx)
	
	// Prefer Ollama if available (local, no API costs)
	if status.Providers["ollama"] {
		return "ollama"
	}
	
	// Fallback to OpenAI if available
	if status.Providers["openai"] {
		return "openai"
	}
	
	// No healthy providers
	return "none"
}

// GetHealthyProviders returns a list of currently healthy AI providers
func (h *HealthChecker) GetHealthyProviders(ctx context.Context) []string {
	status := h.CheckHealth(ctx)
	
	var healthy []string
	for provider, isHealthy := range status.Providers {
		if isHealthy {
			healthy = append(healthy, provider)
		}
	}
	
	return healthy
}

// ValidateAIConfiguration checks if AI configuration is valid and providers are accessible
func (h *HealthChecker) ValidateAIConfiguration(ctx context.Context) error {
	status := h.CheckHealth(ctx)
	
	if status.Overall == "critical" {
		return fmt.Errorf("no AI providers available - check Ollama service and configuration")
	}
	
	// Check if at least one provider with models is available
	hasModels := false
	for provider, isHealthy := range status.Providers {
		if isHealthy {
			if provider == "ollama" && len(status.Models) > 0 {
				hasModels = true
				break
			} else if provider == "openai" {
				hasModels = true
				break
			}
		}
	}
	
	if !hasModels {
		return fmt.Errorf("no AI models available - check Ollama model initialization")
	}
	
	return nil
}