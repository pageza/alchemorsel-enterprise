// Package performance provides Core Web Vitals optimization middleware
package performance

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/cache"
)

// CoreWebVitalsMiddleware provides automatic Core Web Vitals optimization for HTTP responses
type CoreWebVitalsMiddleware struct {
	orchestrator *CoreWebVitalsOrchestrator
	config       MiddlewareConfig
}

// MiddlewareConfig configures the Core Web Vitals middleware
type MiddlewareConfig struct {
	// Optimization settings
	EnableAutomaticOptimization bool          // Enable automatic optimization
	OptimizeHTML               bool          // Optimize HTML responses
	OptimizeCSS                bool          // Optimize CSS responses
	OptimizeJS                 bool          // Optimize JavaScript responses
	
	// Content type filters
	HTMLContentTypes []string // Content types to treat as HTML
	CSSContentTypes  []string // Content types to treat as CSS
	JSContentTypes   []string // Content types to treat as JavaScript
	
	// URL patterns
	IncludePatterns []string // URL patterns to include
	ExcludePatterns []string // URL patterns to exclude
	
	// Performance settings
	OptimizationTimeout time.Duration // Timeout for optimization
	EnableCaching       bool          // Enable response caching
	CacheTTL           time.Duration // Cache TTL for optimized responses
	
	// Monitoring settings
	EnableMetrics      bool    // Enable performance metrics collection
	EnableRUM          bool    // Enable Real User Monitoring
	RUMSampleRate     float64 // RUM sampling rate
	
	// Debug settings
	EnableDebugHeaders bool // Add debug headers to responses
	LogOptimizations   bool // Log optimization activities
}

// ResponseWrapper wraps http.ResponseWriter to capture response data
type ResponseWrapper struct {
	http.ResponseWriter
	statusCode   int
	body         *bytes.Buffer
	contentType  string
	headers      http.Header
}

// DefaultMiddlewareConfig returns sensible defaults for the middleware
func DefaultMiddlewareConfig() MiddlewareConfig {
	return MiddlewareConfig{
		// Optimization settings
		EnableAutomaticOptimization: true,
		OptimizeHTML:               true,
		OptimizeCSS:                false, // Disabled by default for safety
		OptimizeJS:                 false, // Disabled by default for safety
		
		// Content type filters
		HTMLContentTypes: []string{
			"text/html",
			"application/xhtml+xml",
		},
		CSSContentTypes: []string{
			"text/css",
		},
		JSContentTypes: []string{
			"application/javascript",
			"text/javascript",
			"application/x-javascript",
		},
		
		// URL patterns (empty means all URLs)
		IncludePatterns: []string{},
		ExcludePatterns: []string{
			"/api/",
			"/static/",
			"/assets/",
			"/_health",
			"/metrics",
		},
		
		// Performance settings
		OptimizationTimeout: 5 * time.Second,
		EnableCaching:       true,
		CacheTTL:           1 * time.Hour,
		
		// Monitoring settings
		EnableMetrics:     true,
		EnableRUM:         true,
		RUMSampleRate:    0.05, // 5% sampling
		
		// Debug settings
		EnableDebugHeaders: false,
		LogOptimizations:   true,
	}
}

// NewCoreWebVitalsMiddleware creates a new Core Web Vitals middleware
func NewCoreWebVitalsMiddleware(orchestrator *CoreWebVitalsOrchestrator, config MiddlewareConfig) *CoreWebVitalsMiddleware {
	return &CoreWebVitalsMiddleware{
		orchestrator: orchestrator,
		config:       config,
	}
}

// Middleware returns the HTTP middleware function
func (m *CoreWebVitalsMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if optimization should be applied to this request
			if !m.shouldOptimize(r) {
				next.ServeHTTP(w, r)
				return
			}
			
			// Wrap the response writer to capture output
			wrapper := &ResponseWrapper{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				headers:        make(http.Header),
			}
			
			// Record request start time for metrics
			startTime := time.Now()
			
			// Serve the request
			next.ServeHTTP(wrapper, r)
			
			// Process the response
			m.processResponse(wrapper, r, startTime)
		})
	}
}

// shouldOptimize determines if a request should be optimized
func (m *CoreWebVitalsMiddleware) shouldOptimize(r *http.Request) bool {
	if !m.config.EnableAutomaticOptimization {
		return false
	}
	
	// Check HTTP method (only optimize GET requests)
	if r.Method != http.MethodGet {
		return false
	}
	
	// Check exclude patterns
	for _, pattern := range m.config.ExcludePatterns {
		if matched, _ := regexp.MatchString(pattern, r.URL.Path); matched {
			return false
		}
	}
	
	// Check include patterns (if specified)
	if len(m.config.IncludePatterns) > 0 {
		for _, pattern := range m.config.IncludePatterns {
			if matched, _ := regexp.MatchString(pattern, r.URL.Path); matched {
				return true
			}
		}
		return false
	}
	
	return true
}

// processResponse processes and optimizes the response
func (m *CoreWebVitalsMiddleware) processResponse(wrapper *ResponseWrapper, r *http.Request, startTime time.Time) {
	// Get the original response body
	originalBody := wrapper.body.Bytes()
	contentType := wrapper.contentType
	
	// Skip empty responses
	if len(originalBody) == 0 {
		m.writeResponse(wrapper, originalBody)
		return
	}
	
	// Determine if optimization should be applied based on content type
	shouldOptimizeContent := m.shouldOptimizeContentType(contentType)
	
	if !shouldOptimizeContent {
		// Add RUM script to HTML responses even if not optimizing
		if m.config.EnableRUM && m.isHTMLContentType(contentType) {
			optimizedBody := m.injectRUMScript(originalBody, r)
			m.writeResponse(wrapper, optimizedBody)
		} else {
			m.writeResponse(wrapper, originalBody)
		}
		return
	}
	
	// Apply optimization with timeout
	ctx, cancel := context.WithTimeout(r.Context(), m.config.OptimizationTimeout)
	defer cancel()
	
	optimizedBody, err := m.optimizeContent(ctx, originalBody, contentType, r)
	if err != nil {
		// Log error and serve original content
		if m.config.LogOptimizations {
			fmt.Printf("Core Web Vitals optimization failed for %s: %v\n", r.URL.Path, err)
		}
		m.writeResponse(wrapper, originalBody)
		return
	}
	
	// Add debug headers if enabled
	if m.config.EnableDebugHeaders {
		m.addDebugHeaders(wrapper, len(originalBody), len(optimizedBody), time.Since(startTime))
	}
	
	// Record metrics if enabled
	if m.config.EnableMetrics {
		m.recordMetrics(r, len(originalBody), len(optimizedBody), time.Since(startTime))
	}
	
	// Write optimized response
	m.writeResponse(wrapper, optimizedBody)
}

// shouldOptimizeContentType determines if content type should be optimized
func (m *CoreWebVitalsMiddleware) shouldOptimizeContentType(contentType string) bool {
	// Normalize content type (remove charset, etc.)
	ct := strings.ToLower(strings.Split(contentType, ";")[0])
	
	// Check HTML
	if m.config.OptimizeHTML && m.isContentTypeIn(ct, m.config.HTMLContentTypes) {
		return true
	}
	
	// Check CSS
	if m.config.OptimizeCSS && m.isContentTypeIn(ct, m.config.CSSContentTypes) {
		return true
	}
	
	// Check JavaScript
	if m.config.OptimizeJS && m.isContentTypeIn(ct, m.config.JSContentTypes) {
		return true
	}
	
	return false
}

// isHTMLContentType checks if content type is HTML
func (m *CoreWebVitalsMiddleware) isHTMLContentType(contentType string) bool {
	ct := strings.ToLower(strings.Split(contentType, ";")[0])
	return m.isContentTypeIn(ct, m.config.HTMLContentTypes)
}

// isContentTypeIn checks if content type is in the list
func (m *CoreWebVitalsMiddleware) isContentTypeIn(contentType string, types []string) bool {
	for _, t := range types {
		if contentType == strings.ToLower(t) {
			return true
		}
	}
	return false
}

// optimizeContent applies Core Web Vitals optimization to content
func (m *CoreWebVitalsMiddleware) optimizeContent(ctx context.Context, content []byte, contentType string, r *http.Request) ([]byte, error) {
	contentStr := string(content)
	
	// Apply optimization based on content type
	if m.isHTMLContentType(contentType) {
		return m.optimizeHTML(ctx, contentStr, r)
	}
	
	// For non-HTML content, return as-is for now
	// TODO: Implement CSS and JS optimization
	return content, nil
}

// optimizeHTML applies Core Web Vitals optimization to HTML content
func (m *CoreWebVitalsMiddleware) optimizeHTML(ctx context.Context, html string, r *http.Request) ([]byte, error) {
	// Apply Core Web Vitals optimization
	optimized, err := m.orchestrator.OptimizeHTMLWithContext(ctx, html)
	if err != nil {
		return nil, fmt.Errorf("failed to optimize HTML: %w", err)
	}
	
	// Inject RUM script if enabled
	if m.config.EnableRUM {
		optimized = m.injectRUMScript([]byte(optimized), r)
	} else {
		optimized = []byte(optimized)
	}
	
	return optimized, nil
}

// injectRUMScript injects the RUM (Real User Monitoring) script into HTML
func (m *CoreWebVitalsMiddleware) injectRUMScript(content []byte, r *http.Request) []byte {
	html := string(content)
	
	// Generate RUM configuration
	rumConfig := fmt.Sprintf(`
<script>
window.rumConfig = {
	endpoint: '/api/rum/collect',
	sampleRate: %f,
	enableDetailedMetrics: true,
	enableBusinessMetrics: true,
	enableHeatmaps: %t,
	enableUserJourneys: true,
	url: '%s',
	userAgent: '%s',
	timestamp: %d
};
</script>
<script src="/static/js/rum-client.js"></script>`,
		m.config.RUMSampleRate,
		true, // Enable heatmaps
		r.URL.Path,
		r.UserAgent(),
		time.Now().UnixMilli(),
	)
	
	// Insert before closing head tag
	headEndRegex := regexp.MustCompile(`</head>`)
	if headEndRegex.MatchString(html) {
		html = headEndRegex.ReplaceAllString(html, rumConfig+"\n</head>")
	} else {
		// If no head tag, insert at the beginning of body
		bodyStartRegex := regexp.MustCompile(`<body[^>]*>`)
		if bodyStartRegex.MatchString(html) {
			html = bodyStartRegex.ReplaceAllStringFunc(html, func(match string) string {
				return match + "\n" + rumConfig
			})
		}
	}
	
	return []byte(html)
}

// addDebugHeaders adds debug headers to the response
func (m *CoreWebVitalsMiddleware) addDebugHeaders(wrapper *ResponseWrapper, originalSize, optimizedSize int, duration time.Duration) {
	wrapper.Header().Set("X-CWV-Optimized", "true")
	wrapper.Header().Set("X-CWV-Original-Size", fmt.Sprintf("%d", originalSize))
	wrapper.Header().Set("X-CWV-Optimized-Size", fmt.Sprintf("%d", optimizedSize))
	wrapper.Header().Set("X-CWV-Optimization-Time", duration.String())
	wrapper.Header().Set("X-CWV-Size-Reduction", fmt.Sprintf("%.2f%%", 
		float64(originalSize-optimizedSize)/float64(originalSize)*100))
}

// recordMetrics records optimization metrics
func (m *CoreWebVitalsMiddleware) recordMetrics(r *http.Request, originalSize, optimizedSize int, duration time.Duration) {
	// Create a measurement for the optimization
	measurement := CWVMeasurement{
		ID:        fmt.Sprintf("opt_%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		URL:       r.URL.String(),
		UserAgent: r.UserAgent(),
		CustomData: map[string]interface{}{
			"optimization_type":     "middleware",
			"original_size":         originalSize,
			"optimized_size":        optimizedSize,
			"optimization_duration": duration.Milliseconds(),
			"size_reduction":        originalSize - optimizedSize,
			"size_reduction_pct":    float64(originalSize-optimizedSize) / float64(originalSize) * 100,
		},
	}
	
	// Record the measurement
	if err := m.orchestrator.RecordMeasurement(measurement); err != nil && m.config.LogOptimizations {
		fmt.Printf("Failed to record optimization metrics: %v\n", err)
	}
}

// writeResponse writes the response to the client
func (m *CoreWebVitalsMiddleware) writeResponse(wrapper *ResponseWrapper, body []byte) {
	// Copy headers from wrapper to actual response writer
	for key, values := range wrapper.headers {
		for _, value := range values {
			wrapper.ResponseWriter.Header().Add(key, value)
		}
	}
	
	// Set content length
	wrapper.ResponseWriter.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
	
	// Write status code if it was set
	if wrapper.statusCode != 0 {
		wrapper.ResponseWriter.WriteHeader(wrapper.statusCode)
	}
	
	// Write body
	wrapper.ResponseWriter.Write(body)
}

// ResponseWrapper methods
func (rw *ResponseWrapper) Write(data []byte) (int, error) {
	return rw.body.Write(data)
}

func (rw *ResponseWrapper) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	
	// Capture content type
	if ct := rw.Header().Get("Content-Type"); ct != "" {
		rw.contentType = ct
	}
}

func (rw *ResponseWrapper) Header() http.Header {
	return rw.headers
}

// PerformanceMiddleware provides additional performance optimizations
type PerformanceMiddleware struct {
	config PerformanceMiddlewareConfig
}

// PerformanceMiddlewareConfig configures performance middleware
type PerformanceMiddlewareConfig struct {
	EnableCompression    bool          // Enable gzip/brotli compression
	EnableCaching        bool          // Enable browser caching headers
	EnableSecurityHeaders bool         // Enable security headers
	EnableEarlyHints     bool          // Enable HTTP/2 Early Hints
	MaxAge              time.Duration // Cache max age
}

// NewPerformanceMiddleware creates performance middleware
func NewPerformanceMiddleware(config PerformanceMiddlewareConfig) *PerformanceMiddleware {
	return &PerformanceMiddleware{
		config: config,
	}
}

// Middleware returns the performance middleware function
func (pm *PerformanceMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add performance headers
			pm.addPerformanceHeaders(w, r)
			
			// Add security headers if enabled
			if pm.config.EnableSecurityHeaders {
				pm.addSecurityHeaders(w)
			}
			
			// Add early hints if enabled
			if pm.config.EnableEarlyHints {
				pm.addEarlyHints(w, r)
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// addPerformanceHeaders adds performance-related headers
func (pm *PerformanceMiddleware) addPerformanceHeaders(w http.ResponseWriter, r *http.Request) {
	// Cache control headers
	if pm.config.EnableCaching {
		maxAge := int(pm.config.MaxAge.Seconds())
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
		w.Header().Set("Expires", time.Now().Add(pm.config.MaxAge).Format(http.TimeFormat))
	}
	
	// Compression headers
	if pm.config.EnableCompression {
		w.Header().Set("Vary", "Accept-Encoding")
	}
}

// addSecurityHeaders adds security headers that can impact performance
func (pm *PerformanceMiddleware) addSecurityHeaders(w http.ResponseWriter) {
	// Security headers that can improve performance
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	
	// Content Security Policy with performance optimizations
	csp := "default-src 'self'; " +
		"img-src 'self' data: https:; " +
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"font-src 'self' https://fonts.gstatic.com; " +
		"connect-src 'self'"
	
	w.Header().Set("Content-Security-Policy", csp)
}

// addEarlyHints adds HTTP/2 Early Hints for performance
func (pm *PerformanceMiddleware) addEarlyHints(w http.ResponseWriter, r *http.Request) {
	// Add Link headers for critical resources
	w.Header().Add("Link", "</static/css/critical.css>; rel=preload; as=style")
	w.Header().Add("Link", "</static/js/critical.js>; rel=preload; as=script")
	w.Header().Add("Link", "</static/fonts/inter-regular.woff2>; rel=preload; as=font; type=font/woff2; crossorigin")
	
	// Add DNS prefetch hints
	w.Header().Add("Link", "//fonts.googleapis.com; rel=dns-prefetch")
	w.Header().Add("Link", "//fonts.gstatic.com; rel=dns-prefetch")
}

// CombinedMiddleware combines Core Web Vitals and Performance middleware
func CombinedMiddleware(
	orchestrator *CoreWebVitalsOrchestrator,
	cwvConfig MiddlewareConfig,
	perfConfig PerformanceMiddlewareConfig,
) func(http.Handler) http.Handler {
	
	cwvMiddleware := NewCoreWebVitalsMiddleware(orchestrator, cwvConfig)
	perfMiddleware := NewPerformanceMiddleware(perfConfig)
	
	return func(next http.Handler) http.Handler {
		// Chain middlewares: Performance -> Core Web Vitals -> Next
		return perfMiddleware.Middleware()(
			cwvMiddleware.Middleware()(next),
		)
	}
}