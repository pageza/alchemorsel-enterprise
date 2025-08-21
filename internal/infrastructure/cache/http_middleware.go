// Package cache provides HTTP caching middleware for cache-first architecture
package cache

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// HTTPCacheMiddleware provides HTTP response caching for optimal 14KB first packet delivery
type HTTPCacheMiddleware struct {
	cache      *CacheService
	keyBuilder *KeyBuilder
	config     *HTTPCacheConfig
	logger     *zap.Logger
}

// HTTPCacheConfig configures HTTP caching behavior
type HTTPCacheConfig struct {
	// TTL configurations for different response types
	DefaultTTL    time.Duration `json:"default_ttl"`
	APITTL        time.Duration `json:"api_ttl"`
	StaticTTL     time.Duration `json:"static_ttl"`
	HTMXTTL       time.Duration `json:"htmx_ttl"`
	
	// Caching behavior
	CacheGETOnly        bool     `json:"cache_get_only"`
	CacheOnlySuccessful bool     `json:"cache_only_successful"`
	CompressResponses   bool     `json:"compress_responses"`
	SkipPaths          []string `json:"skip_paths"`
	CacheableMethods   []string `json:"cacheable_methods"`
	
	// Response size limits
	MaxCacheableSize   int64 `json:"max_cacheable_size"`
	MinCacheableSize   int64 `json:"min_cacheable_size"`
	
	// Headers
	VaryHeaders        []string `json:"vary_headers"`
	CacheControlHeader string   `json:"cache_control_header"`
	ETags              bool     `json:"etags"`
	
	// 14KB optimization settings
	FirstPacketOptimization bool `json:"first_packet_optimization"`
	FirstPacketTarget      int  `json:"first_packet_target"` // 14KB target
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
	ETag       string              `json:"etag,omitempty"`
	CachedAt   time.Time           `json:"cached_at"`
	TTL        time.Duration       `json:"ttl"`
	Compressed bool                `json:"compressed"`
}

// ResponseWriter wraps http.ResponseWriter to capture response data
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	headers    http.Header
	written    bool
}

// NewHTTPCacheMiddleware creates a new HTTP cache middleware
func NewHTTPCacheMiddleware(cache *CacheService, logger *zap.Logger) *HTTPCacheMiddleware {
	config := DefaultHTTPCacheConfig()
	
	return &HTTPCacheMiddleware{
		cache:      cache,
		keyBuilder: NewKeyBuilder(),
		config:     config,
		logger:     logger,
	}
}

// NewHTTPCacheMiddlewareWithConfig creates middleware with custom configuration
func NewHTTPCacheMiddlewareWithConfig(cache *CacheService, config *HTTPCacheConfig, logger *zap.Logger) *HTTPCacheMiddleware {
	return &HTTPCacheMiddleware{
		cache:      cache,
		keyBuilder: NewKeyBuilder(),
		config:     config,
		logger:     logger,
	}
}

// Middleware returns the HTTP middleware function
func (hcm *HTTPCacheMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip caching for non-cacheable requests
			if !hcm.shouldCache(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Generate cache key
			cacheKey := hcm.generateCacheKey(r)
			
			// Try to serve from cache
			if hcm.tryServeFromCache(w, r, cacheKey) {
				return
			}

			// Wrap response writer to capture response
			rw := NewResponseWriter(w)
			
			// Handle conditional requests (If-None-Match)
			if hcm.handleConditionalRequest(rw, r, cacheKey) {
				return
			}

			// Serve request and capture response
			next.ServeHTTP(rw, r)

			// Cache the response if appropriate
			hcm.cacheResponse(r, rw, cacheKey)
		})
	}
}

// shouldCache determines if a request should be cached
func (hcm *HTTPCacheMiddleware) shouldCache(r *http.Request) bool {
	// Check method
	if hcm.config.CacheGETOnly && r.Method != http.MethodGet {
		return false
	}
	
	// Check if method is in cacheable methods list
	if len(hcm.config.CacheableMethods) > 0 {
		methodAllowed := false
		for _, method := range hcm.config.CacheableMethods {
			if r.Method == method {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			return false
		}
	}

	// Check skip paths
	for _, skipPath := range hcm.config.SkipPaths {
		if strings.HasPrefix(r.URL.Path, skipPath) {
			return false
		}
	}

	// Check for no-cache headers
	if r.Header.Get("Cache-Control") == "no-cache" {
		return false
	}

	return true
}

// generateCacheKey creates a unique cache key for the request
func (hcm *HTTPCacheMiddleware) generateCacheKey(r *http.Request) string {
	// Base key components
	keyParts := []string{
		"http",
		r.Method,
		r.URL.Path,
	}

	// Add query parameters
	if r.URL.RawQuery != "" {
		keyParts = append(keyParts, "query", hcm.hashString(r.URL.RawQuery))
	}

	// Add vary headers
	for _, header := range hcm.config.VaryHeaders {
		if value := r.Header.Get(header); value != "" {
			keyParts = append(keyParts, strings.ToLower(header), hcm.hashString(value))
		}
	}

	// Add user context for personalized responses
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		keyParts = append(keyParts, "user", userID)
	}

	// Add Accept-Encoding for compression variants
	if encoding := r.Header.Get("Accept-Encoding"); encoding != "" && hcm.config.CompressResponses {
		keyParts = append(keyParts, "encoding", hcm.hashString(encoding))
	}

	return hcm.keyBuilder.BuildKey(keyParts...)
}

// tryServeFromCache attempts to serve the response from cache
func (hcm *HTTPCacheMiddleware) tryServeFromCache(w http.ResponseWriter, r *http.Request, cacheKey string) bool {
	ctx, cancel := context.WithTimeout(r.Context(), time.Millisecond*50) // Fast cache lookup
	defer cancel()

	data, err := hcm.cache.Get(ctx, cacheKey)
	if err != nil {
		if err != ErrKeyNotFound {
			hcm.logger.Debug("Cache lookup error", zap.String("key", cacheKey), zap.Error(err))
		}
		return false
	}

	// Deserialize cached response
	var cachedResp CachedResponse
	if err := hcm.cache.serializer.Deserialize(data, &cachedResp); err != nil {
		hcm.logger.Error("Failed to deserialize cached response", zap.String("key", cacheKey), zap.Error(err))
		return false
	}

	// Check if cache entry is still valid
	if time.Since(cachedResp.CachedAt) > cachedResp.TTL {
		// Expired, remove from cache
		go func() {
			hcm.cache.Delete(context.Background(), cacheKey)
		}()
		return false
	}

	// Set cached headers
	for name, values := range cachedResp.Headers {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set cache headers
	w.Header().Set("X-Cache", "HIT")
	w.Header().Set("X-Cache-Key", hcm.hashString(cacheKey)[:8])
	
	if cachedResp.ETag != "" {
		w.Header().Set("ETag", cachedResp.ETag)
	}

	// Set cache control header
	if hcm.config.CacheControlHeader != "" {
		w.Header().Set("Cache-Control", hcm.config.CacheControlHeader)
	}

	// Write status and body
	w.WriteHeader(cachedResp.StatusCode)
	w.Write(cachedResp.Body)

	hcm.logger.Debug("Served from cache", 
		zap.String("key", cacheKey),
		zap.Int("status", cachedResp.StatusCode),
		zap.Int("size", len(cachedResp.Body)),
		zap.Bool("compressed", cachedResp.Compressed))

	return true
}

// handleConditionalRequest handles If-None-Match headers for ETags
func (hcm *HTTPCacheMiddleware) handleConditionalRequest(w http.ResponseWriter, r *http.Request, cacheKey string) bool {
	if !hcm.config.ETags {
		return false
	}

	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == "" {
		return false
	}

	// Check if we have a cached ETag
	ctx, cancel := context.WithTimeout(r.Context(), time.Millisecond*50)
	defer cancel()

	data, err := hcm.cache.Get(ctx, cacheKey)
	if err != nil {
		return false
	}

	var cachedResp CachedResponse
	if err := hcm.cache.serializer.Deserialize(data, &cachedResp); err != nil {
		return false
	}

	if cachedResp.ETag != "" && cachedResp.ETag == ifNoneMatch {
		w.Header().Set("ETag", cachedResp.ETag)
		w.Header().Set("X-Cache", "NOT-MODIFIED")
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	return false
}

// cacheResponse stores the response in cache if appropriate
func (hcm *HTTPCacheMiddleware) cacheResponse(r *http.Request, rw *ResponseWriter, cacheKey string) {
	// Check if response should be cached
	if !hcm.shouldCacheResponse(rw) {
		return
	}

	// Determine TTL based on content type and path
	ttl := hcm.determineTTL(r, rw)
	if ttl <= 0 {
		return
	}

	// Create cached response
	cachedResp := CachedResponse{
		StatusCode: rw.statusCode,
		Headers:    make(map[string][]string),
		Body:       rw.body.Bytes(),
		CachedAt:   time.Now(),
		TTL:        ttl,
		Compressed: false,
	}

	// Copy headers (exclude hop-by-hop headers)
	for name, values := range rw.Header() {
		if !isHopByHopHeader(name) {
			cachedResp.Headers[name] = values
		}
	}

	// Generate ETag if enabled
	if hcm.config.ETags {
		cachedResp.ETag = hcm.generateETag(cachedResp.Body)
		cachedResp.Headers["ETag"] = []string{cachedResp.ETag}
	}

	// Optimize for 14KB first packet if enabled
	if hcm.config.FirstPacketOptimization && len(cachedResp.Body) > hcm.config.FirstPacketTarget {
		hcm.optimizeFirstPacket(&cachedResp, r)
	}

	// Serialize and store
	data, err := hcm.cache.serializer.Serialize(cachedResp)
	if err != nil {
		hcm.logger.Error("Failed to serialize response for caching", zap.String("key", cacheKey), zap.Error(err))
		return
	}

	// Store in cache asynchronously to avoid blocking response
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		if err := hcm.cache.Set(ctx, cacheKey, data, ttl); err != nil {
			hcm.logger.Error("Failed to cache response", zap.String("key", cacheKey), zap.Error(err))
		} else {
			hcm.logger.Debug("Response cached", 
				zap.String("key", cacheKey),
				zap.Int("status", cachedResp.StatusCode),
				zap.Int("size", len(cachedResp.Body)),
				zap.Duration("ttl", ttl))
		}
	}()

	// Set cache headers on response
	rw.Header().Set("X-Cache", "MISS")
	rw.Header().Set("X-Cache-Key", hcm.hashString(cacheKey)[:8])
	
	if hcm.config.CacheControlHeader != "" {
		rw.Header().Set("Cache-Control", hcm.config.CacheControlHeader)
	}
}

// shouldCacheResponse determines if a response should be cached
func (hcm *HTTPCacheMiddleware) shouldCacheResponse(rw *ResponseWriter) bool {
	// Check if we only cache successful responses
	if hcm.config.CacheOnlySuccessful && (rw.statusCode < 200 || rw.statusCode >= 300) {
		return false
	}

	// Check response size limits
	bodySize := int64(rw.body.Len())
	if bodySize < hcm.config.MinCacheableSize || bodySize > hcm.config.MaxCacheableSize {
		return false
	}

	// Check for no-cache headers in response
	cacheControl := rw.Header().Get("Cache-Control")
	if strings.Contains(cacheControl, "no-cache") || strings.Contains(cacheControl, "no-store") {
		return false
	}

	return true
}

// determineTTL determines the appropriate TTL for a response
func (hcm *HTTPCacheMiddleware) determineTTL(r *http.Request, rw *ResponseWriter) time.Duration {
	// Check for explicit Cache-Control max-age
	cacheControl := rw.Header().Get("Cache-Control")
	if maxAge := extractMaxAge(cacheControl); maxAge > 0 {
		return time.Duration(maxAge) * time.Second
	}

	// Determine TTL based on path and content type
	path := r.URL.Path
	contentType := rw.Header().Get("Content-Type")

	// API endpoints
	if strings.HasPrefix(path, "/api/") {
		return hcm.config.APITTL
	}

	// Static resources
	if isStaticResource(path, contentType) {
		return hcm.config.StaticTTL
	}

	// HTMX responses
	if isHTMXRequest(r) {
		return hcm.config.HTMXTTL
	}

	// Default TTL
	return hcm.config.DefaultTTL
}

// optimizeFirstPacket optimizes response for 14KB first packet delivery
func (hcm *HTTPCacheMiddleware) optimizeFirstPacket(cachedResp *CachedResponse, r *http.Request) {
	bodySize := len(cachedResp.Body)
	target := hcm.config.FirstPacketTarget
	
	if bodySize <= target {
		return // Already within target
	}

	// For HTML responses, try to extract critical above-the-fold content
	contentType := cachedResp.Headers["Content-Type"]
	if len(contentType) > 0 && strings.Contains(contentType[0], "text/html") {
		hcm.optimizeHTMLFirstPacket(cachedResp, target)
		return
	}

	// For JSON responses, try to prioritize important fields
	if len(contentType) > 0 && strings.Contains(contentType[0], "application/json") {
		hcm.optimizeJSONFirstPacket(cachedResp, target)
		return
	}

	// For other responses, log the opportunity
	hcm.logger.Debug("Large response detected - consider optimization",
		zap.String("path", r.URL.Path),
		zap.Int("size", bodySize),
		zap.Int("target", target),
		zap.String("content_type", contentType[0]))
}

// optimizeHTMLFirstPacket optimizes HTML for first packet delivery
func (hcm *HTTPCacheMiddleware) optimizeHTMLFirstPacket(cachedResp *CachedResponse, target int) {
	// This is a simplified optimization
	// In production, you'd want more sophisticated HTML parsing and optimization
	
	html := string(cachedResp.Body)
	
	// Add performance hints in the head section
	optimizationHints := `
<!-- 14KB First Packet Optimization -->
<link rel="preload" href="/static/css/critical.css" as="style">
<link rel="preload" href="/static/js/critical.js" as="script">
<style>
/* Critical CSS inlined for first packet */
body { margin: 0; font-family: -apple-system, BlinkMacSystemFont, sans-serif; }
.critical-content { display: block; }
.non-critical { display: none; }
</style>
<script>
/* Critical JS for immediate interactivity */
document.addEventListener('DOMContentLoaded', function() {
    document.querySelectorAll('.non-critical').forEach(el => el.style.display = 'block');
});
</script>
`
	
	// Insert optimization hints after <head> tag
	if headIndex := strings.Index(html, "<head>"); headIndex != -1 {
		insertPoint := headIndex + 6
		optimizedHTML := html[:insertPoint] + optimizationHints + html[insertPoint:]
		
		// If still too large, we could truncate non-critical content
		if len(optimizedHTML) > target {
			// Mark content as critical vs non-critical
			// This is a simplified approach - production would need more sophisticated parsing
			cachedResp.Body = []byte(optimizedHTML[:target] + "<!-- Content truncated for first packet optimization -->")
		} else {
			cachedResp.Body = []byte(optimizedHTML)
		}
		
		// Add header to indicate optimization
		cachedResp.Headers["X-First-Packet-Optimized"] = []string{"true"}
	}
}

// optimizeJSONFirstPacket optimizes JSON for first packet delivery
func (hcm *HTTPCacheMiddleware) optimizeJSONFirstPacket(cachedResp *CachedResponse, target int) {
	// For JSON, we could implement field prioritization
	// This is a placeholder for more sophisticated JSON optimization
	
	if len(cachedResp.Body) > target {
		// Add header to indicate large JSON response
		cachedResp.Headers["X-Large-Response"] = []string{"true"}
		cachedResp.Headers["X-Response-Size"] = []string{strconv.Itoa(len(cachedResp.Body))}
		
		hcm.logger.Info("Large JSON response - consider pagination or field selection",
			zap.Int("size", len(cachedResp.Body)),
			zap.Int("target", target))
	}
}

// generateETag creates an ETag for the response body
func (hcm *HTTPCacheMiddleware) generateETag(body []byte) string {
	hash := md5.Sum(body)
	return fmt.Sprintf(`"%x"`, hash)
}

// hashString creates a short hash of a string
func (hcm *HTTPCacheMiddleware) hashString(s string) string {
	hash := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", hash)[:8]
}

// Helper functions

func extractMaxAge(cacheControl string) int {
	parts := strings.Split(cacheControl, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			if maxAge, err := strconv.Atoi(part[8:]); err == nil {
				return maxAge
			}
		}
	}
	return 0
}

func isStaticResource(path, contentType string) bool {
	staticExtensions := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2"}
	staticContentTypes := []string{"text/css", "application/javascript", "image/", "font/"}
	
	for _, ext := range staticExtensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	
	for _, ct := range staticContentTypes {
		if strings.HasPrefix(contentType, ct) {
			return true
		}
	}
	
	return false
}

func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func isHopByHopHeader(name string) bool {
	hopByHopHeaders := []string{
		"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"TE", "Trailers", "Transfer-Encoding", "Upgrade",
	}
	
	nameLower := strings.ToLower(name)
	for _, header := range hopByHopHeaders {
		if nameLower == strings.ToLower(header) {
			return true
		}
	}
	return false
}

// ResponseWriter implementation

func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
		headers:        make(http.Header),
	}
}

func (rw *ResponseWriter) WriteHeader(statusCode int) {
	if rw.written {
		return
	}
	rw.statusCode = statusCode
	rw.written = true
	
	// Copy headers
	for name, values := range rw.headers {
		for _, value := range values {
			rw.ResponseWriter.Header().Add(name, value)
		}
	}
	
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *ResponseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	
	// Write to both buffer and original writer
	rw.body.Write(data)
	return rw.ResponseWriter.Write(data)
}

func (rw *ResponseWriter) Header() http.Header {
	// Return our header copy so we can capture changes
	if rw.headers == nil {
		rw.headers = make(http.Header)
	}
	
	// Copy from original header
	for name, values := range rw.ResponseWriter.Header() {
		rw.headers[name] = values
	}
	
	return rw.headers
}

// DefaultHTTPCacheConfig returns default HTTP cache configuration
func DefaultHTTPCacheConfig() *HTTPCacheConfig {
	return &HTTPCacheConfig{
		DefaultTTL:             time.Minute * 5,
		APITTL:                 time.Minute * 2,
		StaticTTL:              time.Hour * 24,
		HTMXTTL:                time.Minute * 10,
		CacheGETOnly:           true,
		CacheOnlySuccessful:    true,
		CompressResponses:      true,
		SkipPaths:              []string{"/health", "/metrics", "/debug"},
		CacheableMethods:       []string{"GET", "HEAD"},
		MaxCacheableSize:       10 * 1024 * 1024, // 10MB
		MinCacheableSize:       100,               // 100 bytes
		VaryHeaders:            []string{"Accept-Encoding", "Authorization"},
		CacheControlHeader:     "public, max-age=300",
		ETags:                  true,
		FirstPacketOptimization: true,
		FirstPacketTarget:      14 * 1024, // 14KB
	}
}