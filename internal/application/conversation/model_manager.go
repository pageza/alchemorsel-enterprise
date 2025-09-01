// Package conversation provides model management for AI conversations
package conversation

import (
	"context"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"go.uber.org/zap"
)

// ModelType represents different types of AI models
type ModelType string

const (
	ModelTypeChat   ModelType = "chat"
	ModelTypeRecipe ModelType = "recipe"
	ModelTypeCode   ModelType = "code"
	ModelTypeHelp   ModelType = "help"
)

// QualityLevel represents response quality requirements
type QualityLevel string

const (
	QualityFast     QualityLevel = "fast"     // Quick responses, lower quality
	QualityBalanced QualityLevel = "balanced" // Default balance
	QualityHigh     QualityLevel = "high"     // Best quality, slower
)

// ModelInfo contains metadata about an AI model
type ModelInfo struct {
	Name           string
	Type           ModelType
	MemoryUsage    int64
	InferenceSpeed float64 // tokens per second
	QualityScore   float64 // 0.0 to 1.0
	Loaded         bool
	LastUsed       time.Time
	HealthStatus   string
}

// GenerationOptions contains options for AI text generation
type GenerationOptions struct {
	Intent      ConversationIntent
	Context     ConversationContext
	Streaming   bool
	MaxTokens   int
	Temperature float64
	Quality     QualityLevel
}

// GenerationResult contains the result of AI text generation
type GenerationResult struct {
	Content   string
	Quality   float64
	Duration  time.Duration
	TokensUsed int
	ModelUsed string
	Metadata  map[string]interface{}
}

// ModelLoadBalancer handles distributing requests across models
type ModelLoadBalancer struct {
	models    map[string]*ModelInfo
	mu        sync.RWMutex
	logger    *zap.Logger
	roundRobin int
}

// ModelHealthChecker monitors model health and performance
type ModelHealthChecker struct {
	models     map[string]*ModelInfo
	mu         sync.RWMutex
	logger     *zap.Logger
	checkTicker *time.Ticker
}

// ModelManager manages multiple AI models with intelligent selection
type ModelManager struct {
	models         map[string]*ModelInfo
	defaultModel   string
	fallbackModel  string
	loadBalancer   *ModelLoadBalancer
	healthChecker  *ModelHealthChecker
	config         *config.AIConfig
	logger         *zap.Logger
	mu             sync.RWMutex
}

// NewModelManager creates a new model manager instance
func NewModelManager(cfg *config.Config, logger *zap.Logger) *ModelManager {
	models := make(map[string]*ModelInfo)
	
	// Initialize predefined models based on configuration
	models[cfg.AI.ChatModel] = &ModelInfo{
		Name:           cfg.AI.ChatModel,
		Type:           ModelTypeChat,
		QualityScore:   0.85,
		InferenceSpeed: 15.0, // estimated tokens per second
		MemoryUsage:    4700 * 1024 * 1024, // ~4.7GB for 8B model
		HealthStatus:   "unknown",
	}
	
	models[cfg.AI.RecipeModel] = &ModelInfo{
		Name:           cfg.AI.RecipeModel,
		Type:           ModelTypeRecipe,
		QualityScore:   0.88,
		InferenceSpeed: 15.0,
		MemoryUsage:    4700 * 1024 * 1024,
		HealthStatus:   "unknown",
	}
	
	models[cfg.AI.CodeModel] = &ModelInfo{
		Name:           cfg.AI.CodeModel,
		Type:           ModelTypeCode,
		QualityScore:   0.82,
		InferenceSpeed: 12.0,
		MemoryUsage:    4100 * 1024 * 1024, // ~4.1GB for 7B model
		HealthStatus:   "unknown",
	}
	
	models[cfg.AI.HelpModel] = &ModelInfo{
		Name:           cfg.AI.HelpModel,
		Type:           ModelTypeHelp,
		QualityScore:   0.75,
		InferenceSpeed: 25.0,
		MemoryUsage:    2300 * 1024 * 1024, // ~2.3GB for 3.8B model
		HealthStatus:   "unknown",
	}
	
	loadBalancer := &ModelLoadBalancer{
		models: models,
		logger: logger.Named("load_balancer"),
	}
	
	healthChecker := &ModelHealthChecker{
		models: models,
		logger: logger.Named("health_checker"),
	}
	
	return &ModelManager{
		models:        models,
		defaultModel:  cfg.AI.OllamaModel,
		fallbackModel: cfg.AI.OllamaFallbackModel,
		loadBalancer:  loadBalancer,
		healthChecker: healthChecker,
		config:        &cfg.AI,
		logger:        logger.Named("model_manager"),
	}
}

// SelectOptimalModel selects the best model for given intent and context
func (mm *ModelManager) SelectOptimalModel(intent ConversationIntent, context ConversationContext) (string, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	
	// Determine model type based on intent
	var modelType ModelType
	switch intent {
	case IntentRecipeCreation:
		modelType = ModelTypeRecipe
	case IntentCookingHelp:
		modelType = ModelTypeHelp
	case IntentTechnicalSupport:
		modelType = ModelTypeCode
	default:
		modelType = ModelTypeChat
	}
	
	// Find best model for the type
	return mm.selectModelByType(modelType, context.Complexity)
}

// selectModelByType selects the best model for a specific type and complexity
func (mm *ModelManager) selectModelByType(modelType ModelType, complexity string) (string, error) {
	var candidates []*ModelInfo
	
	// Collect candidates of the right type
	for _, model := range mm.models {
		if model.Type == modelType && model.HealthStatus != "failed" {
			candidates = append(candidates, model)
		}
	}
	
	// If no candidates found, fall back to default
	if len(candidates) == 0 {
		mm.logger.Warn("No healthy models found for type, using default",
			zap.String("model_type", string(modelType)),
			zap.String("default_model", mm.defaultModel))
		return mm.defaultModel, nil
	}
	
	// Select based on complexity and performance
	var selectedModel *ModelInfo
	switch complexity {
	case "high":
		// Use highest quality model
		selectedModel = mm.selectByQuality(candidates)
	case "low":
		// Use fastest model
		selectedModel = mm.selectBySpeed(candidates)
	default:
		// Balanced selection
		selectedModel = mm.selectBalanced(candidates)
	}
	
	// Update last used time
	selectedModel.LastUsed = time.Now()
	
	mm.logger.Debug("Selected model",
		zap.String("model", selectedModel.Name),
		zap.String("type", string(modelType)),
		zap.String("complexity", complexity),
		zap.Float64("quality_score", selectedModel.QualityScore))
	
	return selectedModel.Name, nil
}

// selectByQuality selects model with highest quality score
func (mm *ModelManager) selectByQuality(candidates []*ModelInfo) *ModelInfo {
	if len(candidates) == 0 {
		return nil
	}
	
	best := candidates[0]
	for _, model := range candidates[1:] {
		if model.QualityScore > best.QualityScore {
			best = model
		}
	}
	return best
}

// selectBySpeed selects model with highest inference speed
func (mm *ModelManager) selectBySpeed(candidates []*ModelInfo) *ModelInfo {
	if len(candidates) == 0 {
		return nil
	}
	
	fastest := candidates[0]
	for _, model := range candidates[1:] {
		if model.InferenceSpeed > fastest.InferenceSpeed {
			fastest = model
		}
	}
	return fastest
}

// selectBalanced selects model with best quality/speed balance
func (mm *ModelManager) selectBalanced(candidates []*ModelInfo) *ModelInfo {
	if len(candidates) == 0 {
		return nil
	}
	
	best := candidates[0]
	bestScore := mm.calculateBalanceScore(best)
	
	for _, model := range candidates[1:] {
		score := mm.calculateBalanceScore(model)
		if score > bestScore {
			best = model
			bestScore = score
		}
	}
	return best
}

// calculateBalanceScore calculates a balanced quality/speed score
func (mm *ModelManager) calculateBalanceScore(model *ModelInfo) float64 {
	// Normalize speed (assuming max speed of 30 tokens/sec)
	normalizedSpeed := model.InferenceSpeed / 30.0
	if normalizedSpeed > 1.0 {
		normalizedSpeed = 1.0
	}
	
	// Balance quality (70%) and speed (30%)
	return (model.QualityScore * 0.7) + (normalizedSpeed * 0.3)
}

// GetModelInfo returns information about a specific model
func (mm *ModelManager) GetModelInfo(modelName string) (*ModelInfo, bool) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	
	model, exists := mm.models[modelName]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent race conditions
	info := *model
	return &info, true
}

// ListModels returns a list of all available models
func (mm *ModelManager) ListModels() []*ModelInfo {
	mm.mu.RLock()
	defer mm.mu.RUnlock()
	
	models := make([]*ModelInfo, 0, len(mm.models))
	for _, model := range mm.models {
		// Return copies to prevent race conditions
		info := *model
		models = append(models, &info)
	}
	
	return models
}

// UpdateModelHealth updates the health status of a model
func (mm *ModelManager) UpdateModelHealth(modelName, status string) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	if model, exists := mm.models[modelName]; exists {
		model.HealthStatus = status
		mm.logger.Debug("Updated model health",
			zap.String("model", modelName),
			zap.String("status", status))
	}
}

// UpdateModelMetrics updates performance metrics for a model
func (mm *ModelManager) UpdateModelMetrics(modelName string, duration time.Duration, tokensUsed int, quality float64) {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	model, exists := mm.models[modelName]
	if !exists {
		return
	}
	
	// Update inference speed (tokens per second)
	if duration > 0 {
		tokensPerSecond := float64(tokensUsed) / duration.Seconds()
		// Use exponential moving average for smoothing
		model.InferenceSpeed = (model.InferenceSpeed * 0.8) + (tokensPerSecond * 0.2)
	}
	
	// Update quality score
	if quality > 0 {
		model.QualityScore = (model.QualityScore * 0.9) + (quality * 0.1)
	}
	
	model.LastUsed = time.Now()
	
	mm.logger.Debug("Updated model metrics",
		zap.String("model", modelName),
		zap.Duration("duration", duration),
		zap.Int("tokens", tokensUsed),
		zap.Float64("quality", quality),
		zap.Float64("inference_speed", model.InferenceSpeed))
}

// GetFallbackModel returns the configured fallback model
func (mm *ModelManager) GetFallbackModel() string {
	return mm.fallbackModel
}

// StartHealthChecking starts background health checking for models
func (mm *ModelManager) StartHealthChecking(ctx context.Context) {
	mm.healthChecker.Start(ctx, mm)
}

// Start begins health checking routine
func (hc *ModelHealthChecker) Start(ctx context.Context, manager *ModelManager) {
	hc.checkTicker = time.NewTicker(30 * time.Second)
	
	go func() {
		defer hc.checkTicker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				hc.logger.Info("Health checker stopped")
				return
			case <-hc.checkTicker.C:
				hc.performHealthChecks(manager)
			}
		}
	}()
	
	hc.logger.Info("Model health checker started")
}

// performHealthChecks checks the health of all models
func (hc *ModelHealthChecker) performHealthChecks(manager *ModelManager) {
	hc.mu.RLock()
	models := make(map[string]*ModelInfo)
	for k, v := range hc.models {
		models[k] = v
	}
	hc.mu.RUnlock()
	
	for modelName, model := range models {
		// Simple health check based on last usage and known issues
		status := "healthy"
		
		// Mark as stale if not used recently
		if time.Since(model.LastUsed) > 30*time.Minute {
			status = "stale"
		}
		
		// Update health status in manager
		manager.UpdateModelHealth(modelName, status)
	}
}

// Shutdown gracefully shuts down the model manager
func (mm *ModelManager) Shutdown(ctx context.Context) error {
	mm.logger.Info("Shutting down model manager")
	
	if mm.healthChecker.checkTicker != nil {
		mm.healthChecker.checkTicker.Stop()
	}
	
	return nil
}