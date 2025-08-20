// Package cache provides template caching for optimized HTMX response delivery
package cache

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// TemplateCacheService provides comprehensive template caching for HTMX optimization
type TemplateCacheService struct {
	cache      *CacheService
	keyBuilder *KeyBuilder
	config     *TemplateCacheConfig
	logger     *zap.Logger
	templates  map[string]*template.Template // In-memory template store
}

// TemplateCacheConfig configures template caching behavior
type TemplateCacheConfig struct {
	// TTL configurations for different template types
	ComponentTTL     time.Duration `json:"component_ttl"`
	PageTTL          time.Duration `json:"page_ttl"`
	PartialTTL       time.Duration `json:"partial_ttl"`
	FragmentTTL      time.Duration `json:"fragment_ttl"`
	
	// HTMX-specific caching
	HTMXResponseTTL  time.Duration `json:"htmx_response_ttl"`
	HTMXPartialTTL   time.Duration `json:"htmx_partial_ttl"`
	HTMXSwapTTL      time.Duration `json:"htmx_swap_ttl"`
	
	// Performance optimizations
	CompressionEnabled    bool `json:"compression_enabled"`
	MinCompressionSize    int  `json:"min_compression_size"`
	MaxTemplateSize       int64 `json:"max_template_size"`
	InMemoryTemplates     bool `json:"in_memory_templates"`
	
	// Cache behavior
	CacheByUserType       bool     `json:"cache_by_user_type"`
	CacheByPermissions    bool     `json:"cache_by_permissions"`
	VaryByHeaders         []string `json:"vary_by_headers"`
	SkipTemplates         []string `json:"skip_templates"`
	
	// 14KB optimization
	FirstPacketOptimization bool `json:"first_packet_optimization"`
	FirstPacketTarget       int  `json:"first_packet_target"`
	CriticalCSS             bool `json:"critical_css"`
	LazyLoadNonCritical     bool `json:"lazy_load_non_critical"`
}

// CachedTemplate represents a cached template response
type CachedTemplate struct {
	Name         string                 `json:"name"`
	Data         map[string]interface{} `json:"data"`
	DataHash     string                 `json:"data_hash"`
	RenderedHTML string                 `json:"rendered_html"`
	ContentType  string                 `json:"content_type"`
	Headers      map[string]string      `json:"headers"`
	CachedAt     time.Time              `json:"cached_at"`
	AccessCount  int64                  `json:"access_count"`
	LastAccess   time.Time              `json:"last_access"`
	HTMXContext  *HTMXContext           `json:"htmx_context,omitempty"`
	UserContext  *UserContext           `json:"user_context,omitempty"`
	Size         int                    `json:"size"`
	Compressed   bool                   `json:"compressed"`
}

// HTMXContext contains HTMX-specific template context
type HTMXContext struct {
	IsHTMXRequest    bool   `json:"is_htmx_request"`
	Target           string `json:"target,omitempty"`
	Trigger          string `json:"trigger,omitempty"`
	TriggerName      string `json:"trigger_name,omitempty"`
	CurrentURL       string `json:"current_url,omitempty"`
	SwapType         string `json:"swap_type,omitempty"`
	IsBoost          bool   `json:"is_boost"`
	HistoryRestore   bool   `json:"history_restore"`
}

// UserContext contains user-specific template context
type UserContext struct {
	UserID       string   `json:"user_id,omitempty"`
	UserType     string   `json:"user_type,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	IsAnonymous  bool     `json:"is_anonymous"`
	Preferences  map[string]interface{} `json:"preferences,omitempty"`
}

// TemplateMetrics tracks template rendering performance
type TemplateMetrics struct {
	RenderTime    time.Duration `json:"render_time"`
	CacheHit      bool          `json:"cache_hit"`
	TemplateSize  int           `json:"template_size"`
	CompressionRatio float64    `json:"compression_ratio,omitempty"`
}

// NewTemplateCacheService creates a new template cache service
func NewTemplateCacheService(cache *CacheService, logger *zap.Logger) *TemplateCacheService {
	config := DefaultTemplateCacheConfig()
	
	return &TemplateCacheService{
		cache:      cache,
		keyBuilder: NewKeyBuilder(),
		config:     config,
		logger:     logger,
		templates:  make(map[string]*template.Template),
	}
}

// CacheTemplate caches a rendered template with context
func (tcs *TemplateCacheService) CacheTemplate(ctx context.Context, templateName string, data map[string]interface{}, renderedHTML string, r *http.Request) error {
	if tcs.shouldSkipTemplate(templateName) {
		return nil
	}
	
	// Extract contexts
	htmxContext := tcs.extractHTMXContext(r)
	userContext := tcs.extractUserContext(r, data)
	
	// Generate cache key
	cacheKey := tcs.buildTemplateKey(templateName, data, htmxContext, userContext)
	
	// Create cached template
	cached := CachedTemplate{
		Name:         templateName,
		Data:         data,
		DataHash:     tcs.hashData(data),
		RenderedHTML: renderedHTML,
		ContentType:  "text/html; charset=utf-8",
		Headers:      tcs.extractHeaders(r),
		CachedAt:     time.Now(),
		AccessCount:  0,
		LastAccess:   time.Now(),
		HTMXContext:  htmxContext,
		UserContext:  userContext,
		Size:         len(renderedHTML),
		Compressed:   false,
	}
	
	// Apply 14KB optimization if enabled
	if tcs.config.FirstPacketOptimization && len(renderedHTML) > tcs.config.FirstPacketTarget {
		optimizedHTML := tcs.optimizeForFirstPacket(renderedHTML, templateName)
		cached.RenderedHTML = optimizedHTML
		cached.Size = len(optimizedHTML)
	}
	
	// Determine TTL based on template type
	ttl := tcs.determineTTL(templateName, htmxContext)
	
	return tcs.cacheTemplateResponse(ctx, cacheKey, &cached, ttl)
}

// GetTemplate retrieves a cached template or renders using fallback
func (tcs *TemplateCacheService) GetTemplate(ctx context.Context, templateName string, data map[string]interface{}, r *http.Request, fallback func(string, map[string]interface{}) (string, error)) (string, *TemplateMetrics, error) {
	start := time.Now()
	metrics := &TemplateMetrics{
		CacheHit: false,
	}
	
	if tcs.shouldSkipTemplate(templateName) {
		// Skip cache, use fallback directly
		html, err := fallback(templateName, data)
		metrics.RenderTime = time.Since(start)
		metrics.TemplateSize = len(html)
		return html, metrics, err
	}
	
	// Extract contexts
	htmxContext := tcs.extractHTMXContext(r)
	userContext := tcs.extractUserContext(r, data)
	
	// Generate cache key
	cacheKey := tcs.buildTemplateKey(templateName, data, htmxContext, userContext)
	
	// Try cache first
	cached, err := tcs.getCachedTemplate(ctx, cacheKey)
	if err == nil {
		// Update access stats asynchronously
		go tcs.updateTemplateAccessStats(ctx, cacheKey, cached)
		
		metrics.RenderTime = time.Since(start)
		metrics.CacheHit = true
		metrics.TemplateSize = cached.Size
		
		tcs.logger.Debug("Template cache hit",
			zap.String("template", templateName),
			zap.String("key", cacheKey),
			zap.Int64("access_count", cached.AccessCount))
		
		return cached.RenderedHTML, metrics, nil
	}
	
	// Cache miss - use fallback
	if fallback == nil {
		return "", metrics, fmt.Errorf("template not found in cache and no fallback provided")
	}
	
	renderStart := time.Now()
	html, err := fallback(templateName, data)
	renderTime := time.Since(renderStart)
	
	if err != nil {
		return "", metrics, err
	}
	
	metrics.RenderTime = time.Since(start)
	metrics.TemplateSize = len(html)
	
	// Cache the result asynchronously to avoid blocking response
	go func() {
		if err := tcs.CacheTemplate(context.Background(), templateName, data, html, r); err != nil {
			tcs.logger.Error("Failed to cache template after render",
				zap.String("template", templateName),
				zap.Error(err))
		}
	}()
	
	tcs.logger.Debug("Template rendered and cached",
		zap.String("template", templateName),
		zap.Duration("render_time", renderTime),
		zap.Int("size", len(html)))
	
	return html, metrics, nil
}

// InvalidateTemplate removes cached templates matching criteria
func (tcs *TemplateCacheService) InvalidateTemplate(ctx context.Context, templateName string, userID ...string) error {
	patterns := []string{
		tcs.keyBuilder.BuildKey("template", templateName, "*"),
	}
	
	// Add user-specific patterns if provided
	for _, uid := range userID {
		patterns = append(patterns, 
			tcs.keyBuilder.BuildKey("template", templateName, fmt.Sprintf("*user:%s*", uid)))
	}
	
	for _, pattern := range patterns {
		if err := tcs.cache.InvalidateByPattern(ctx, pattern); err != nil {
			tcs.logger.Error("Failed to invalidate template cache",
				zap.String("template", templateName),
				zap.String("pattern", pattern),
				zap.Error(err))
		}
	}
	
	tcs.logger.Info("Template cache invalidated",
		zap.String("template", templateName),
		zap.Strings("user_ids", userID))
	
	return nil
}

// InvalidateUserTemplates removes all cached templates for a user
func (tcs *TemplateCacheService) InvalidateUserTemplates(ctx context.Context, userID string) error {
	pattern := tcs.keyBuilder.BuildKey("template", "*", fmt.Sprintf("*user:%s*", userID))
	
	if err := tcs.cache.InvalidateByPattern(ctx, pattern); err != nil {
		tcs.logger.Error("Failed to invalidate user templates",
			zap.String("user_id", userID),
			zap.Error(err))
		return err
	}
	
	tcs.logger.Info("User templates invalidated", zap.String("user_id", userID))
	return nil
}

// PrewarmTemplates pre-renders and caches common templates
func (tcs *TemplateCacheService) PrewarmTemplates(ctx context.Context, templates []PrewarmTemplate) error {
	tcs.logger.Info("Starting template prewarming", zap.Int("count", len(templates)))
	
	start := time.Now()
	warmed := 0
	
	for _, tmpl := range templates {
		// Create mock request for context
		req := &http.Request{
			Header: make(http.Header),
		}
		
		if tmpl.HTMXRequest {
			req.Header.Set("HX-Request", "true")
			if tmpl.HTMXTarget != "" {
				req.Header.Set("HX-Target", tmpl.HTMXTarget)
			}
		}
		
		// Render and cache template
		if err := tcs.CacheTemplate(ctx, tmpl.Name, tmpl.Data, tmpl.PrerenderedHTML, req); err != nil {
			tcs.logger.Error("Failed to prewarm template",
				zap.String("template", tmpl.Name),
				zap.Error(err))
		} else {
			warmed++
		}
	}
	
	duration := time.Since(start)
	tcs.logger.Info("Template prewarming completed",
		zap.Int("total", len(templates)),
		zap.Int("warmed", warmed),
		zap.Duration("duration", duration))
	
	return nil
}

// Helper methods

func (tcs *TemplateCacheService) extractHTMXContext(r *http.Request) *HTMXContext {
	if r == nil {
		return &HTMXContext{}
	}
	
	return &HTMXContext{
		IsHTMXRequest:  r.Header.Get("HX-Request") == "true",
		Target:         r.Header.Get("HX-Target"),
		Trigger:        r.Header.Get("HX-Trigger"),
		TriggerName:    r.Header.Get("HX-Trigger-Name"),
		CurrentURL:     r.Header.Get("HX-Current-URL"),
		SwapType:       r.Header.Get("HX-Swap"),
		IsBoost:        r.Header.Get("HX-Boosted") == "true",
		HistoryRestore: r.Header.Get("HX-History-Restore-Request") == "true",
	}
}

func (tcs *TemplateCacheService) extractUserContext(r *http.Request, data map[string]interface{}) *UserContext {
	userCtx := &UserContext{
		IsAnonymous: true,
	}
	
	if r != nil {
		// Extract user ID from header or context
		if userID := r.Header.Get("X-User-ID"); userID != "" {
			userCtx.UserID = userID
			userCtx.IsAnonymous = false
		}
		
		// Extract user type
		if userType := r.Header.Get("X-User-Type"); userType != "" {
			userCtx.UserType = userType
		}
	}
	
	// Extract from template data
	if data != nil {
		if userID, ok := data["user_id"].(string); ok && userID != "" {
			userCtx.UserID = userID
			userCtx.IsAnonymous = false
		}
		
		if userType, ok := data["user_type"].(string); ok {
			userCtx.UserType = userType
		}
		
		if permissions, ok := data["permissions"].([]string); ok {
			userCtx.Permissions = permissions
		}
		
		if prefs, ok := data["user_preferences"].(map[string]interface{}); ok {
			userCtx.Preferences = prefs
		}
	}
	
	return userCtx
}

func (tcs *TemplateCacheService) extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	
	if r == nil {
		return headers
	}
	
	// Extract vary headers
	for _, header := range tcs.config.VaryByHeaders {
		if value := r.Header.Get(header); value != "" {
			headers[header] = value
		}
	}
	
	return headers
}

func (tcs *TemplateCacheService) buildTemplateKey(templateName string, data map[string]interface{}, htmxCtx *HTMXContext, userCtx *UserContext) string {
	keyParts := []string{"template", templateName}
	
	// Add data hash
	if len(data) > 0 {
		keyParts = append(keyParts, "data", tcs.hashData(data))
	}
	
	// Add user context if caching by user type
	if tcs.config.CacheByUserType && userCtx != nil {
		if !userCtx.IsAnonymous {
			keyParts = append(keyParts, "user", userCtx.UserID)
		} else {
			keyParts = append(keyParts, "anon")
		}
		
		if userCtx.UserType != "" {
			keyParts = append(keyParts, "type", userCtx.UserType)
		}
	}
	
	// Add permissions if caching by permissions
	if tcs.config.CacheByPermissions && userCtx != nil && len(userCtx.Permissions) > 0 {
		permHash := tcs.hashStringSlice(userCtx.Permissions)
		keyParts = append(keyParts, "perm", permHash)
	}
	
	// Add HTMX context
	if htmxCtx != nil && htmxCtx.IsHTMXRequest {
		keyParts = append(keyParts, "htmx")
		
		if htmxCtx.Target != "" {
			keyParts = append(keyParts, "target", htmxCtx.Target)
		}
		
		if htmxCtx.SwapType != "" {
			keyParts = append(keyParts, "swap", htmxCtx.SwapType)
		}
		
		if htmxCtx.IsBoost {
			keyParts = append(keyParts, "boost")
		}
	}
	
	return tcs.keyBuilder.BuildKey(keyParts...)
}

func (tcs *TemplateCacheService) hashData(data map[string]interface{}) string {
	// Remove volatile data before hashing
	filteredData := make(map[string]interface{})
	volatileKeys := []string{"csrf_token", "timestamp", "nonce", "request_id"}
	
	for k, v := range data {
		isVolatile := false
		for _, vk := range volatileKeys {
			if strings.Contains(strings.ToLower(k), vk) {
				isVolatile = true
				break
			}
		}
		
		if !isVolatile {
			filteredData[k] = v
		}
	}
	
	// Serialize and hash
	jsonData, _ := json.Marshal(filteredData)
	hash := md5.Sum(jsonData)
	return fmt.Sprintf("%x", hash)[:8]
}

func (tcs *TemplateCacheService) hashStringSlice(slice []string) string {
	combined := strings.Join(slice, "|")
	hash := md5.Sum([]byte(combined))
	return fmt.Sprintf("%x", hash)[:8]
}

func (tcs *TemplateCacheService) shouldSkipTemplate(templateName string) bool {
	for _, skip := range tcs.config.SkipTemplates {
		if strings.Contains(templateName, skip) {
			return true
		}
	}
	return false
}

func (tcs *TemplateCacheService) determineTTL(templateName string, htmxCtx *HTMXContext) time.Duration {
	// HTMX-specific TTLs
	if htmxCtx != nil && htmxCtx.IsHTMXRequest {
		if strings.Contains(templateName, "partial") {
			return tcs.config.HTMXPartialTTL
		}
		if htmxCtx.SwapType != "" {
			return tcs.config.HTMXSwapTTL
		}
		return tcs.config.HTMXResponseTTL
	}
	
	// Template type-based TTLs
	if strings.Contains(templateName, "component") {
		return tcs.config.ComponentTTL
	}
	if strings.Contains(templateName, "partial") {
		return tcs.config.PartialTTL
	}
	if strings.Contains(templateName, "fragment") {
		return tcs.config.FragmentTTL
	}
	
	// Default page TTL
	return tcs.config.PageTTL
}

func (tcs *TemplateCacheService) optimizeForFirstPacket(html, templateName string) string {
	if !tcs.config.FirstPacketOptimization {
		return html
	}
	
	target := tcs.config.FirstPacketTarget
	
	if len(html) <= target {
		return html
	}
	
	// For HTML templates, try to include critical above-the-fold content
	optimized := html
	
	// Add critical CSS inlining if enabled
	if tcs.config.CriticalCSS {
		criticalCSS := `<style>
/* Critical CSS for first packet */
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, sans-serif; }
.critical { display: block; }
.non-critical { display: none; }
@media (min-width: 768px) { .critical { font-size: 1.1em; } }
</style>`
		
		// Insert after <head> tag
		if headIndex := strings.Index(optimized, "<head>"); headIndex != -1 {
			insertPoint := headIndex + 6
			optimized = optimized[:insertPoint] + criticalCSS + optimized[insertPoint:]
		}
	}
	
	// Add lazy loading script if enabled
	if tcs.config.LazyLoadNonCritical {
		lazyScript := `<script>
document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll('.non-critical').forEach(el => {
        el.style.display = 'block';
        el.classList.remove('non-critical');
    });
});
</script>`
		
		// Add before closing </body> tag
		if bodyIndex := strings.LastIndex(optimized, "</body>"); bodyIndex != -1 {
			optimized = optimized[:bodyIndex] + lazyScript + optimized[bodyIndex:]
		}
	}
	
	// If still too large, truncate with continuation marker
	if len(optimized) > target {
		// Find a good truncation point (end of tag)
		truncateAt := target - 200 // Leave room for continuation
		for i := truncateAt; i > truncateAt-100 && i > 0; i-- {
			if optimized[i] == '>' {
				truncateAt = i + 1
				break
			}
		}
		
		continuation := `
<!-- Content truncated for 14KB first packet optimization -->
<script>
// Load remaining content asynchronously
fetch(window.location.href + '?full=1')
    .then(response => response.text())
    .then(html => {
        document.body.innerHTML = html;
    });
</script>`
		
		optimized = optimized[:truncateAt] + continuation + "</body></html>"
	}
	
	return optimized
}

func (tcs *TemplateCacheService) cacheTemplateResponse(ctx context.Context, cacheKey string, cached *CachedTemplate, ttl time.Duration) error {
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("failed to marshal template cache: %w", err)
	}
	
	// Check size limits
	if int64(len(data)) > tcs.config.MaxTemplateSize {
		return fmt.Errorf("template too large to cache: %d bytes", len(data))
	}
	
	tags := []string{"template", "html"}
	if cached.HTMXContext != nil && cached.HTMXContext.IsHTMXRequest {
		tags = append(tags, "htmx")
	}
	
	if err := tcs.cache.SetWithTags(ctx, cacheKey, data, ttl, tags); err != nil {
		return fmt.Errorf("failed to cache template: %w", err)
	}
	
	tcs.logger.Debug("Template cached",
		zap.String("template", cached.Name),
		zap.String("key", cacheKey),
		zap.Duration("ttl", ttl),
		zap.Int("size", cached.Size))
	
	return nil
}

func (tcs *TemplateCacheService) getCachedTemplate(ctx context.Context, cacheKey string) (*CachedTemplate, error) {
	data, err := tcs.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}
	
	var cached CachedTemplate
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached template: %w", err)
	}
	
	return &cached, nil
}

func (tcs *TemplateCacheService) updateTemplateAccessStats(ctx context.Context, cacheKey string, cached *CachedTemplate) {
	cached.AccessCount++
	cached.LastAccess = time.Now()
	
	// Update cache asynchronously
	go func() {
		data, err := json.Marshal(cached)
		if err != nil {
			return
		}
		
		// Estimate remaining TTL
		ttl := tcs.determineTTL(cached.Name, cached.HTMXContext)
		elapsed := time.Since(cached.CachedAt)
		remaining := ttl - elapsed
		
		if remaining > 0 {
			tcs.cache.Set(context.Background(), cacheKey, data, remaining)
		}
	}()
}

// PrewarmTemplate represents a template to be prewarmed
type PrewarmTemplate struct {
	Name            string                 `json:"name"`
	Data            map[string]interface{} `json:"data"`
	PrerenderedHTML string                 `json:"prerendered_html"`
	HTMXRequest     bool                   `json:"htmx_request"`
	HTMXTarget      string                 `json:"htmx_target,omitempty"`
}

// DefaultTemplateCacheConfig returns default template cache configuration
func DefaultTemplateCacheConfig() *TemplateCacheConfig {
	return &TemplateCacheConfig{
		ComponentTTL:            time.Hour * 2,
		PageTTL:                 time.Minute * 30,
		PartialTTL:              time.Minute * 15,
		FragmentTTL:             time.Minute * 10,
		HTMXResponseTTL:         time.Minute * 5,
		HTMXPartialTTL:          time.Minute * 10,
		HTMXSwapTTL:             time.Minute * 3,
		CompressionEnabled:      true,
		MinCompressionSize:      1024,
		MaxTemplateSize:         5 * 1024 * 1024, // 5MB
		InMemoryTemplates:       true,
		CacheByUserType:         true,
		CacheByPermissions:      false,
		VaryByHeaders:           []string{"Accept-Language", "X-Requested-With"},
		SkipTemplates:           []string{"error", "debug", "admin"},
		FirstPacketOptimization: true,
		FirstPacketTarget:       14 * 1024, // 14KB
		CriticalCSS:             true,
		LazyLoadNonCritical:     true,
	}
}