// Package performance provides Largest Contentful Paint (LCP) optimization
package performance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/cache"
)

// LCPOptimizer optimizes Largest Contentful Paint performance
type LCPOptimizer struct {
	config              LCPConfig
	resourcePrioritizer *ResourcePrioritizer
	imageOptimizer      *LCPImageOptimizer
	fontOptimizer       *LCPFontOptimizer
	serverOptimizer     *ServerOptimizer
	cacheClient         *cache.RedisClient
	bundleOptimizer     *BundleOptimizer
	performanceMetrics  LCPMetrics
}

// LCPConfig configures LCP optimization
type LCPConfig struct {
	EnableResourcePrioritization bool          // Enable resource prioritization
	EnableImageOptimization     bool          // Enable image optimization
	EnableFontOptimization      bool          // Enable font optimization
	EnableServerOptimization    bool          // Enable server-side optimizations
	TargetLCP                   time.Duration // Target LCP time (2.5s for "Good")
	CriticalResourcesMaxSize    int           // Max size for critical resources
	PreloadCriticalResources    bool          // Preload critical resources
	EnableHeroPrioritization    bool          // Prioritize hero content
	CDNEnabled                  bool          // Use CDN for static assets
	EnableBundleOptimization    bool          // Enable 14KB bundle optimization
	MaxBundleSize              int           // Maximum initial bundle size (14KB)
	EnableRedisCache           bool          // Enable Redis caching for optimizations
	CacheTTL                   time.Duration // Cache TTL for optimization results
}

// ResourcePrioritizer manages resource loading priorities
type ResourcePrioritizer struct {
	criticalResources []CriticalResource
	preloadHints      []PreloadHint
	priorityHints     []PriorityHint
	deferredResources []DeferredResource
}

// LCPImageOptimizer optimizes images for LCP
type LCPImageOptimizer struct {
	heroImageSizes    []ImageSize
	responsiveSizes   []ResponsiveSize
	formatPreferences []string
	lazyLoadThreshold int
	placeholderType   string
}

// LCPFontOptimizer optimizes fonts for LCP
type LCPFontOptimizer struct {
	criticalFonts     []CriticalFont
	preloadFonts      []string
	fontDisplayStyle  string
	fontSwapStrategy  string
	fallbackFonts     map[string]string
}

// ServerOptimizer handles server-side LCP optimizations
type ServerOptimizer struct {
	enableGzip          bool
	enableBrotli        bool
	enableHTTP2Push     bool
	maxResourceSize     int
	compressionLevel    int
	cacheStrategies     map[string]CacheStrategy
}

// BundleOptimizer handles 14KB critical bundle optimization
type BundleOptimizer struct {
	criticalCSS         string
	criticalJS          string
	inlineThreshold     int
	bundleCache         map[string]CachedBundle
	resourceAnalyzer    *ResourceAnalyzer
	treeShaker          *TreeShaker
}

// CachedBundle represents a cached optimization bundle
type CachedBundle struct {
	Content     string            `json:"content"`
	Resources   []string          `json:"resources"`
	Size        int               `json:"size"`
	Hash        string            `json:"hash"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata"`
}

// ResourceAnalyzer analyzes resource dependencies
type ResourceAnalyzer struct {
	dependencyGraph map[string][]string
	usageStats      map[string]int
	criticalPath    []string
}

// TreeShaker removes unused code from bundles
type TreeShaker struct {
	usedSelectors   map[string]bool
	usedFunctions   map[string]bool
	deadCodeRules   []string
}

// CriticalResource represents a critical resource for LCP
type CriticalResource struct {
	URL          string
	Type         string // css, js, image, font
	Priority     int    // 1-10, higher = more critical
	Size         int
	LoadTime     time.Duration
	IsAboveFold  bool
	IsHeroImage  bool
	MediaQuery   string
}

// PreloadHint represents a resource preload hint
type PreloadHint struct {
	URL         string
	Type        string
	CrossOrigin string
	MediaQuery  string
	As          string
}

// PriorityHint represents a resource priority hint
type PriorityHint struct {
	URL      string
	Priority string // high, low, auto
	FetchPriority string
}

// DeferredResource represents a resource that can be deferred
type DeferredResource struct {
	URL         string
	Type        string
	DeferUntil  string // load, interaction, visible
	Importance  string // low, high
}

// ImageSize represents an image size variant
type ImageSize struct {
	Width     int
	Height    int
	Format    string
	Quality   int
	URL       string
	IsHero    bool
}

// ResponsiveSize represents responsive image sizing
type ResponsiveSize struct {
	MediaQuery string
	Size       string
	Density    string
}

// CriticalFont represents a font critical for LCP
type CriticalFont struct {
	Family      string
	Weight      string
	Style       string
	URL         string
	Format      string
	IsHeroFont  bool
	LoadTime    time.Duration
}

// CacheStrategy represents a caching strategy
type CacheStrategy struct {
	Type        string        // browser, cdn, server
	Duration    time.Duration
	Conditions  []string
	Priority    int
}

// LCPMetrics tracks LCP optimization performance
type LCPMetrics struct {
	TotalOptimizations     int
	ResourcesOptimized     int
	ImagesOptimized        int
	FontsOptimized         int
	AverageLCPImprovement  time.Duration
	CriticalResourceCount  int
	PreloadedResourceCount int
	LastOptimization       time.Time
	LCPElementType         string
	LCPElementSelector     string
}

// DefaultLCPConfig returns sensible LCP optimization defaults
func DefaultLCPConfig() LCPConfig {
	return LCPConfig{
		EnableResourcePrioritization: true,
		EnableImageOptimization:     true,
		EnableFontOptimization:      true,
		EnableServerOptimization:    true,
		TargetLCP:                   2500 * time.Millisecond, // 2.5s Google "Good" threshold
		CriticalResourcesMaxSize:    100 * 1024,             // 100KB limit for critical resources
		PreloadCriticalResources:    true,
		EnableHeroPrioritization:    true,
		CDNEnabled:                  true,
		EnableBundleOptimization:    true,
		MaxBundleSize:              14 * 1024,               // 14KB initial bundle
		EnableRedisCache:           true,
		CacheTTL:                   1 * time.Hour,           // Cache optimizations for 1 hour
	}
}

// NewLCPOptimizer creates a new LCP optimizer
func NewLCPOptimizer(config LCPConfig, cacheClient *cache.RedisClient) *LCPOptimizer {
	resourcePrioritizer := &ResourcePrioritizer{
		criticalResources: []CriticalResource{},
		preloadHints:      []PreloadHint{},
		priorityHints:     []PriorityHint{},
		deferredResources: []DeferredResource{},
	}

	imageOptimizer := &LCPImageOptimizer{
		heroImageSizes: []ImageSize{
			{Width: 1920, Height: 1080, Format: "webp", Quality: 85, IsHero: true},
			{Width: 1200, Height: 675, Format: "webp", Quality: 85, IsHero: true},
			{Width: 800, Height: 450, Format: "webp", Quality: 80, IsHero: true},
			{Width: 400, Height: 225, Format: "webp", Quality: 75, IsHero: true},
		},
		responsiveSizes: []ResponsiveSize{
			{MediaQuery: "(min-width: 1200px)", Size: "1200px", Density: "1x"},
			{MediaQuery: "(min-width: 768px)", Size: "800px", Density: "1x"},
			{MediaQuery: "(max-width: 767px)", Size: "400px", Density: "1x"},
		},
		formatPreferences: []string{"avif", "webp", "jpg", "png"},
		lazyLoadThreshold: 600, // pixels below fold
		placeholderType:   "blur",
	}

	fontOptimizer := &LCPFontOptimizer{
		criticalFonts: []CriticalFont{},
		preloadFonts:  []string{},
		fontDisplayStyle: "swap",
		fontSwapStrategy: "immediate",
		fallbackFonts: map[string]string{
			"Inter":          "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
			"Roboto":         "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
			"Open Sans":      "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
			"Playfair Display": "Georgia, serif",
			"Source Sans Pro": "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
		},
	}

	serverOptimizer := &ServerOptimizer{
		enableGzip:       true,
		enableBrotli:     true,
		enableHTTP2Push:  true,
		maxResourceSize:  1024 * 1024, // 1MB
		compressionLevel: 6,
		cacheStrategies: map[string]CacheStrategy{
			"images": {
				Type:     "browser",
				Duration: 7 * 24 * time.Hour, // 1 week
				Conditions: []string{"public", "immutable"},
				Priority: 1,
			},
			"fonts": {
				Type:     "browser",
				Duration: 30 * 24 * time.Hour, // 30 days
				Conditions: []string{"public", "immutable"},
				Priority: 1,
			},
			"css": {
				Type:     "browser",
				Duration: 24 * time.Hour, // 1 day
				Conditions: []string{"public"},
				Priority: 2,
			},
		},
	}

	bundleOptimizer := &BundleOptimizer{
		inlineThreshold:  config.MaxBundleSize,
		bundleCache:      make(map[string]CachedBundle),
		resourceAnalyzer: &ResourceAnalyzer{
			dependencyGraph: make(map[string][]string),
			usageStats:      make(map[string]int),
			criticalPath:    []string{},
		},
		treeShaker: &TreeShaker{
			usedSelectors: make(map[string]bool),
			usedFunctions: make(map[string]bool),
			deadCodeRules: []string{
				".unused", "[data-test]", ".debug",
			},
		},
	}

	return &LCPOptimizer{
		config:              config,
		resourcePrioritizer: resourcePrioritizer,
		imageOptimizer:      imageOptimizer,
		fontOptimizer:       fontOptimizer,
		serverOptimizer:     serverOptimizer,
		cacheClient:         cacheClient,
		bundleOptimizer:     bundleOptimizer,
		performanceMetrics:  LCPMetrics{},
	}
}

// OptimizeHTML optimizes HTML for LCP performance with caching support
func (lcp *LCPOptimizer) OptimizeHTML(html string) (string, error) {
	return lcp.OptimizeHTMLWithContext(context.Background(), html)
}

// OptimizeHTMLWithContext optimizes HTML for LCP performance with context and caching
func (lcp *LCPOptimizer) OptimizeHTMLWithContext(ctx context.Context, html string) (string, error) {
	// Check cache first if enabled
	if lcp.config.EnableRedisCache && lcp.cacheClient != nil {
		if cached, err := lcp.getCachedOptimization(ctx, html); err == nil {
			return cached, nil
		}
	}

	optimized := html

	// Step 1: Identify LCP element
	lcpElement, err := lcp.identifyLCPElement(html)
	if err != nil {
		return "", fmt.Errorf("failed to identify LCP element: %w", err)
	}

	// Step 2: Create 14KB critical bundle
	if lcp.config.EnableBundleOptimization {
		optimized, err = lcp.optimizeCriticalBundle(optimized, lcpElement)
		if err != nil {
			return "", fmt.Errorf("failed to optimize critical bundle: %w", err)
		}
	}

	// Step 3: Prioritize critical resources
	if lcp.config.EnableResourcePrioritization {
		optimized = lcp.prioritizeResources(optimized, lcpElement)
	}

	// Step 4: Optimize images for LCP
	if lcp.config.EnableImageOptimization {
		optimized = lcp.optimizeImages(optimized, lcpElement)
	}

	// Step 5: Optimize fonts for LCP
	if lcp.config.EnableFontOptimization {
		optimized = lcp.optimizeFonts(optimized, lcpElement)
	}

	// Step 6: Add resource hints
	optimized = lcp.addResourceHints(optimized)

	// Cache the result if enabled
	if lcp.config.EnableRedisCache && lcp.cacheClient != nil {
		go lcp.cacheOptimization(context.Background(), html, optimized)
	}

	// Update metrics
	lcp.updateMetrics(lcpElement)

	return optimized, nil
}

// getCachedOptimization retrieves cached optimization result
func (lcp *LCPOptimizer) getCachedOptimization(ctx context.Context, html string) (string, error) {
	cacheKey := lcp.generateCacheKey(html)
	result, err := lcp.cacheClient.Get(ctx, "lcp:opt:"+cacheKey)
	if err != nil {
		return "", err
	}
	return result, nil
}

// cacheOptimization stores optimization result in cache
func (lcp *LCPOptimizer) cacheOptimization(ctx context.Context, original, optimized string) {
	cacheKey := lcp.generateCacheKey(original)
	lcp.cacheClient.Set(ctx, "lcp:opt:"+cacheKey, optimized, lcp.config.CacheTTL)
}

// generateCacheKey generates a cache key for HTML content
func (lcp *LCPOptimizer) generateCacheKey(html string) string {
	hash := sha256.Sum256([]byte(html))
	return hex.EncodeToString(hash[:])
}

// optimizeCriticalBundle creates a 14KB critical resource bundle
func (lcp *LCPOptimizer) optimizeCriticalBundle(html string, lcpElement *LCPElement) (string, error) {
	// Analyze critical resources
	criticalResources := lcp.identifyCriticalResources(html, lcpElement)

	// Extract critical CSS
	criticalCSS, err := lcp.extractCriticalCSS(html, lcpElement)
	if err != nil {
		return html, err
	}

	// Extract critical JavaScript
	criticalJS, err := lcp.extractCriticalJS(html, lcpElement)
	if err != nil {
		return html, err
	}

	// Create inline bundle if under 14KB
	bundleSize := len(criticalCSS) + len(criticalJS)
	if bundleSize <= lcp.config.MaxBundleSize {
		return lcp.inlineCriticalBundle(html, criticalCSS, criticalJS), nil
	}

	// Optimize to fit 14KB limit
	optimizedCSS := lcp.compressCSS(criticalCSS)
	optimizedJS := lcp.compressJS(criticalJS)

	bundleSize = len(optimizedCSS) + len(optimizedJS)
	if bundleSize <= lcp.config.MaxBundleSize {
		return lcp.inlineCriticalBundle(html, optimizedCSS, optimizedJS), nil
	}

	// If still too large, tree shake
	shakeCSS := lcp.treeShakeCSS(optimizedCSS, html)
	shakeJS := lcp.treeShakeJS(optimizedJS, html)

	return lcp.inlineCriticalBundle(html, shakeCSS, shakeJS), nil
}

// extractCriticalCSS extracts CSS critical for above-the-fold content
func (lcp *LCPOptimizer) extractCriticalCSS(html string, lcpElement *LCPElement) (string, error) {
	var criticalCSS strings.Builder

	// Base critical styles for layout stability
	criticalCSS.WriteString(`
/* Critical styles for LCP optimization */
body { margin: 0; font-display: swap; }
img { max-width: 100%; height: auto; }
.hero, .banner { min-height: 400px; }
`)

	// Extract inline styles
	styleRegex := regexp.MustCompile(`<style[^>]*>([\s\S]*?)</style>`)
	styleMatches := styleRegex.FindAllStringSubmatch(html, -1)
	for _, match := range styleMatches {
		if len(match) > 1 {
			criticalCSS.WriteString(match[1])
		}
	}

	// Extract critical external CSS (first 2 stylesheets)
	linkRegex := regexp.MustCompile(`<link[^>]*rel="stylesheet"[^>]*href="([^"]+)"[^>]*>`)
	linkMatches := linkRegex.FindAllStringSubmatch(html, 2) // Limit to first 2
	for _, match := range linkMatches {
		if len(match) > 1 {
			// In production, this would fetch and inline the CSS
			criticalCSS.WriteString(fmt.Sprintf("/* Critical CSS from %s */\n", match[1]))
		}
	}

	return criticalCSS.String(), nil
}

// extractCriticalJS extracts JavaScript critical for LCP
func (lcp *LCPOptimizer) extractCriticalJS(html string, lcpElement *LCPElement) (string, error) {
	var criticalJS strings.Builder

	// Essential performance monitoring
	criticalJS.WriteString(`
// Critical JS for LCP optimization
window.lcpOptimization = {
	start: performance.now(),
	lcpElement: null,
	measureLCP: function() {
		new PerformanceObserver((list) => {
			for (const entry of list.getEntries()) {
				if (entry.element) {
					this.lcpElement = entry.element;
				}
			}
		}).observe({entryTypes: ['largest-contentful-paint']});
	}
};
window.lcpOptimization.measureLCP();
`)

	// Extract critical inline scripts
	scriptRegex := regexp.MustCompile(`<script[^>]*>([\s\S]*?)</script>`)
	scriptMatches := scriptRegex.FindAllStringSubmatch(html, 2) // Limit to first 2
	for _, match := range scriptMatches {
		if len(match) > 1 && !strings.Contains(match[1], "async") && !strings.Contains(match[1], "defer") {
			criticalJS.WriteString(match[1])
		}
	}

	return criticalJS.String(), nil
}

// inlineCriticalBundle inlines critical CSS and JS into HTML
func (lcp *LCPOptimizer) inlineCriticalBundle(html, css, js string) string {
	optimized := html

	// Inline critical CSS in head
	criticalStyleTag := fmt.Sprintf(`<style data-critical="true">%s</style>`, css)
	headEndRegex := regexp.MustCompile(`</head>`)
	optimized = headEndRegex.ReplaceAllString(optimized, "    "+criticalStyleTag+"\n</head>")

	// Inline critical JS after body start
	criticalScriptTag := fmt.Sprintf(`<script data-critical="true">%s</script>`, js)
	bodyStartRegex := regexp.MustCompile(`<body[^>]*>`)
	optimized = bodyStartRegex.ReplaceAllStringFunc(optimized, func(match string) string {
		return match + "\n    " + criticalScriptTag
	})

	return optimized
}

// compressCSS compresses CSS by removing whitespace and comments
func (lcp *LCPOptimizer) compressCSS(css string) string {
	// Remove comments
	commentRegex := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	compressed := commentRegex.ReplaceAllString(css, "")

	// Remove excess whitespace
	whitespaceRegex := regexp.MustCompile(`\s+`)
	compressed = whitespaceRegex.ReplaceAllString(compressed, " ")

	// Remove spaces around special characters
	spaceRegex := regexp.MustCompile(`\s*([{}:;,>+~])\s*`)
	compressed = spaceRegex.ReplaceAllString(compressed, "$1")

	return strings.TrimSpace(compressed)
}

// compressJS compresses JavaScript by removing whitespace and comments
func (lcp *LCPOptimizer) compressJS(js string) string {
	// Remove single-line comments
	singleCommentRegex := regexp.MustCompile(`//.*// Package performance provides Largest Contentful Paint (LCP) optimization
package performance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/cache"
)

// LCPOptimizer optimizes Largest Contentful Paint performance
type LCPOptimizer struct {
	config              LCPConfig
	resourcePrioritizer *ResourcePrioritizer
	imageOptimizer      *LCPImageOptimizer
	fontOptimizer       *LCPFontOptimizer
	serverOptimizer     *ServerOptimizer
	cacheClient         *cache.RedisClient
	bundleOptimizer     *BundleOptimizer
	performanceMetrics  LCPMetrics
}

// LCPConfig configures LCP optimization
type LCPConfig struct {
	EnableResourcePrioritization bool          // Enable resource prioritization
	EnableImageOptimization     bool          // Enable image optimization
	EnableFontOptimization      bool          // Enable font optimization
	EnableServerOptimization    bool          // Enable server-side optimizations
	TargetLCP                   time.Duration // Target LCP time (2.5s for "Good")
	CriticalResourcesMaxSize    int           // Max size for critical resources
	PreloadCriticalResources    bool          // Preload critical resources
	EnableHeroPrioritization    bool          // Prioritize hero content
	CDNEnabled                  bool          // Use CDN for static assets
	EnableBundleOptimization    bool          // Enable 14KB bundle optimization
	MaxBundleSize              int           // Maximum initial bundle size (14KB)
	EnableRedisCache           bool          // Enable Redis caching for optimizations
	CacheTTL                   time.Duration // Cache TTL for optimization results
}

// ResourcePrioritizer manages resource loading priorities
type ResourcePrioritizer struct {
	criticalResources []CriticalResource
	preloadHints      []PreloadHint
	priorityHints     []PriorityHint
	deferredResources []DeferredResource
}

// LCPImageOptimizer optimizes images for LCP
type LCPImageOptimizer struct {
	heroImageSizes    []ImageSize
	responsiveSizes   []ResponsiveSize
	formatPreferences []string
	lazyLoadThreshold int
	placeholderType   string
}

// LCPFontOptimizer optimizes fonts for LCP
type LCPFontOptimizer struct {
	criticalFonts     []CriticalFont
	preloadFonts      []string
	fontDisplayStyle  string
	fontSwapStrategy  string
	fallbackFonts     map[string]string
}

// ServerOptimizer handles server-side LCP optimizations
type ServerOptimizer struct {
	enableGzip          bool
	enableBrotli        bool
	enableHTTP2Push     bool
	maxResourceSize     int
	compressionLevel    int
	cacheStrategies     map[string]CacheStrategy
}

// BundleOptimizer handles 14KB critical bundle optimization
type BundleOptimizer struct {
	criticalCSS         string
	criticalJS          string
	inlineThreshold     int
	bundleCache         map[string]CachedBundle
	resourceAnalyzer    *ResourceAnalyzer
	treeShaker          *TreeShaker
}

// CachedBundle represents a cached optimization bundle
type CachedBundle struct {
	Content     string            `json:"content"`
	Resources   []string          `json:"resources"`
	Size        int               `json:"size"`
	Hash        string            `json:"hash"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata"`
}

// ResourceAnalyzer analyzes resource dependencies
type ResourceAnalyzer struct {
	dependencyGraph map[string][]string
	usageStats      map[string]int
	criticalPath    []string
}

// TreeShaker removes unused code from bundles
type TreeShaker struct {
	usedSelectors   map[string]bool
	usedFunctions   map[string]bool
	deadCodeRules   []string
}

// CriticalResource represents a critical resource for LCP
type CriticalResource struct {
	URL          string
	Type         string // css, js, image, font
	Priority     int    // 1-10, higher = more critical
	Size         int
	LoadTime     time.Duration
	IsAboveFold  bool
	IsHeroImage  bool
	MediaQuery   string
}

// PreloadHint represents a resource preload hint
type PreloadHint struct {
	URL         string
	Type        string
	CrossOrigin string
	MediaQuery  string
	As          string
}

// PriorityHint represents a resource priority hint
type PriorityHint struct {
	URL      string
	Priority string // high, low, auto
	FetchPriority string
}

// DeferredResource represents a resource that can be deferred
type DeferredResource struct {
	URL         string
	Type        string
	DeferUntil  string // load, interaction, visible
	Importance  string // low, high
}

// ImageSize represents an image size variant
type ImageSize struct {
	Width     int
	Height    int
	Format    string
	Quality   int
	URL       string
	IsHero    bool
}

// ResponsiveSize represents responsive image sizing
type ResponsiveSize struct {
	MediaQuery string
	Size       string
	Density    string
}

// CriticalFont represents a font critical for LCP
type CriticalFont struct {
	Family      string
	Weight      string
	Style       string
	URL         string
	Format      string
	IsHeroFont  bool
	LoadTime    time.Duration
}

// CacheStrategy represents a caching strategy
type CacheStrategy struct {
	Type        string        // browser, cdn, server
	Duration    time.Duration
	Conditions  []string
	Priority    int
}

// LCPMetrics tracks LCP optimization performance
type LCPMetrics struct {
	TotalOptimizations     int
	ResourcesOptimized     int
	ImagesOptimized        int
	FontsOptimized         int
	AverageLCPImprovement  time.Duration
	CriticalResourceCount  int
	PreloadedResourceCount int
	LastOptimization       time.Time
	LCPElementType         string
	LCPElementSelector     string
}

// DefaultLCPConfig returns sensible LCP optimization defaults
func DefaultLCPConfig() LCPConfig {
	return LCPConfig{
		EnableResourcePrioritization: true,
		EnableImageOptimization:     true,
		EnableFontOptimization:      true,
		EnableServerOptimization:    true,
		TargetLCP:                   2500 * time.Millisecond, // 2.5s Google "Good" threshold
		CriticalResourcesMaxSize:    100 * 1024,             // 100KB limit for critical resources
		PreloadCriticalResources:    true,
		EnableHeroPrioritization:    true,
		CDNEnabled:                  true,
		EnableBundleOptimization:    true,
		MaxBundleSize:              14 * 1024,               // 14KB initial bundle
		EnableRedisCache:           true,
		CacheTTL:                   1 * time.Hour,           // Cache optimizations for 1 hour
	}
}

// NewLCPOptimizer creates a new LCP optimizer
func NewLCPOptimizer(config LCPConfig, cacheClient *cache.RedisClient) *LCPOptimizer {
	resourcePrioritizer := &ResourcePrioritizer{
		criticalResources: []CriticalResource{},
		preloadHints:      []PreloadHint{},
		priorityHints:     []PriorityHint{},
		deferredResources: []DeferredResource{},
	}

	imageOptimizer := &LCPImageOptimizer{
		heroImageSizes: []ImageSize{
			{Width: 1920, Height: 1080, Format: "webp", Quality: 85, IsHero: true},
			{Width: 1200, Height: 675, Format: "webp", Quality: 85, IsHero: true},
			{Width: 800, Height: 450, Format: "webp", Quality: 80, IsHero: true},
			{Width: 400, Height: 225, Format: "webp", Quality: 75, IsHero: true},
		},
		responsiveSizes: []ResponsiveSize{
			{MediaQuery: "(min-width: 1200px)", Size: "1200px", Density: "1x"},
			{MediaQuery: "(min-width: 768px)", Size: "800px", Density: "1x"},
			{MediaQuery: "(max-width: 767px)", Size: "400px", Density: "1x"},
		},
		formatPreferences: []string{"avif", "webp", "jpg", "png"},
		lazyLoadThreshold: 600, // pixels below fold
		placeholderType:   "blur",
	}

	fontOptimizer := &LCPFontOptimizer{
		criticalFonts: []CriticalFont{},
		preloadFonts:  []string{},
		fontDisplayStyle: "swap",
		fontSwapStrategy: "immediate",
		fallbackFonts: map[string]string{
			"Inter":          "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
			"Roboto":         "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
			"Open Sans":      "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
			"Playfair Display": "Georgia, serif",
			"Source Sans Pro": "system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
		},
	}

	serverOptimizer := &ServerOptimizer{
		enableGzip:       true,
		enableBrotli:     true,
		enableHTTP2Push:  true,
		maxResourceSize:  1024 * 1024, // 1MB
		compressionLevel: 6,
		cacheStrategies: map[string]CacheStrategy{
			"images": {
				Type:     "browser",
				Duration: 7 * 24 * time.Hour, // 1 week
				Conditions: []string{"public", "immutable"},
				Priority: 1,
			},
			"fonts": {
				Type:     "browser",
				Duration: 30 * 24 * time.Hour, // 30 days
				Conditions: []string{"public", "immutable"},
				Priority: 1,
			},
			"css": {
				Type:     "browser",
				Duration: 24 * time.Hour, // 1 day
				Conditions: []string{"public"},
				Priority: 2,
			},
		},
	}

	bundleOptimizer := &BundleOptimizer{
		inlineThreshold:  config.MaxBundleSize,
		bundleCache:      make(map[string]CachedBundle),
		resourceAnalyzer: &ResourceAnalyzer{
			dependencyGraph: make(map[string][]string),
			usageStats:      make(map[string]int),
			criticalPath:    []string{},
		},
		treeShaker: &TreeShaker{
			usedSelectors: make(map[string]bool),
			usedFunctions: make(map[string]bool),
			deadCodeRules: []string{
				".unused", "[data-test]", ".debug",
			},
		},
	}

	return &LCPOptimizer{
		config:              config,
		resourcePrioritizer: resourcePrioritizer,
		imageOptimizer:      imageOptimizer,
		fontOptimizer:       fontOptimizer,
		serverOptimizer:     serverOptimizer,
		cacheClient:         cacheClient,
		bundleOptimizer:     bundleOptimizer,
		performanceMetrics:  LCPMetrics{},
	}
}

// OptimizeHTML optimizes HTML for LCP performance with caching support
func (lcp *LCPOptimizer) OptimizeHTML(html string) (string, error) {
	return lcp.OptimizeHTMLWithContext(context.Background(), html)
}

// OptimizeHTMLWithContext optimizes HTML for LCP performance with context and caching
func (lcp *LCPOptimizer) OptimizeHTMLWithContext(ctx context.Context, html string) (string, error) {
	// Check cache first if enabled
	if lcp.config.EnableRedisCache && lcp.cacheClient != nil {
		if cached, err := lcp.getCachedOptimization(ctx, html); err == nil {
			return cached, nil
		}
	}

	optimized := html

	// Step 1: Identify LCP element
	lcpElement, err := lcp.identifyLCPElement(html)
	if err != nil {
		return "", fmt.Errorf("failed to identify LCP element: %w", err)
	}

	// Step 2: Create 14KB critical bundle
	if lcp.config.EnableBundleOptimization {
		optimized, err = lcp.optimizeCriticalBundle(optimized, lcpElement)
		if err != nil {
			return "", fmt.Errorf("failed to optimize critical bundle: %w", err)
		}
	}

	// Step 3: Prioritize critical resources
	if lcp.config.EnableResourcePrioritization {
		optimized = lcp.prioritizeResources(optimized, lcpElement)
	}

	// Step 4: Optimize images for LCP
	if lcp.config.EnableImageOptimization {
		optimized = lcp.optimizeImages(optimized, lcpElement)
	}

	// Step 5: Optimize fonts for LCP
	if lcp.config.EnableFontOptimization {
		optimized = lcp.optimizeFonts(optimized, lcpElement)
	}

	// Step 6: Add resource hints
	optimized = lcp.addResourceHints(optimized)

	// Cache the result if enabled
	if lcp.config.EnableRedisCache && lcp.cacheClient != nil {
		go lcp.cacheOptimization(context.Background(), html, optimized)
	}

	// Update metrics
	lcp.updateMetrics(lcpElement)

	return optimized, nil
}

// getCachedOptimization retrieves cached optimization result
func (lcp *LCPOptimizer) getCachedOptimization(ctx context.Context, html string) (string, error) {
	cacheKey := lcp.generateCacheKey(html)
	result, err := lcp.cacheClient.Get(ctx, "lcp:opt:"+cacheKey)
	if err != nil {
		return "", err
	}
	return result, nil
}

// cacheOptimization stores optimization result in cache
func (lcp *LCPOptimizer) cacheOptimization(ctx context.Context, original, optimized string) {
	cacheKey := lcp.generateCacheKey(original)
	lcp.cacheClient.Set(ctx, "lcp:opt:"+cacheKey, optimized, lcp.config.CacheTTL)
}

// generateCacheKey generates a cache key for HTML content
func (lcp *LCPOptimizer) generateCacheKey(html string) string {
	hash := sha256.Sum256([]byte(html))
	return hex.EncodeToString(hash[:])
}

// optimizeCriticalBundle creates a 14KB critical resource bundle
func (lcp *LCPOptimizer) optimizeCriticalBundle(html string, lcpElement *LCPElement) (string, error) {
	// Analyze critical resources
	criticalResources := lcp.identifyCriticalResources(html, lcpElement)

	// Extract critical CSS
	criticalCSS, err := lcp.extractCriticalCSS(html, lcpElement)
	if err != nil {
		return html, err
	}

	// Extract critical JavaScript
	criticalJS, err := lcp.extractCriticalJS(html, lcpElement)
	if err != nil {
		return html, err
	}

	// Create inline bundle if under 14KB
	bundleSize := len(criticalCSS) + len(criticalJS)
	if bundleSize <= lcp.config.MaxBundleSize {
		return lcp.inlineCriticalBundle(html, criticalCSS, criticalJS), nil
	}

	// Optimize to fit 14KB limit
	optimizedCSS := lcp.compressCSS(criticalCSS)
	optimizedJS := lcp.compressJS(criticalJS)

	bundleSize = len(optimizedCSS) + len(optimizedJS)
	if bundleSize <= lcp.config.MaxBundleSize {
		return lcp.inlineCriticalBundle(html, optimizedCSS, optimizedJS), nil
	}

	// If still too large, tree shake
	shakeCSS := lcp.treeShakeCSS(optimizedCSS, html)
	shakeJS := lcp.treeShakeJS(optimizedJS, html)

	return lcp.inlineCriticalBundle(html, shakeCSS, shakeJS), nil
}

// extractCriticalCSS extracts CSS critical for above-the-fold content
func (lcp *LCPOptimizer) extractCriticalCSS(html string, lcpElement *LCPElement) (string, error) {
	var criticalCSS strings.Builder

	// Base critical styles for layout stability
	criticalCSS.WriteString(`
/* Critical styles for LCP optimization */
body { margin: 0; font-display: swap; }
img { max-width: 100%; height: auto; }
.hero, .banner { min-height: 400px; }
`)

	// Extract inline styles
	styleRegex := regexp.MustCompile(`<style[^>]*>([\s\S]*?)</style>`)
	styleMatches := styleRegex.FindAllStringSubmatch(html, -1)
	for _, match := range styleMatches {
		if len(match) > 1 {
			criticalCSS.WriteString(match[1])
		}
	}

	// Extract critical external CSS (first 2 stylesheets)
	linkRegex := regexp.MustCompile(`<link[^>]*rel="stylesheet"[^>]*href="([^"]+)"[^>]*>`)
	linkMatches := linkRegex.FindAllStringSubmatch(html, 2) // Limit to first 2
	for _, match := range linkMatches {
		if len(match) > 1 {
			// In production, this would fetch and inline the CSS
			criticalCSS.WriteString(fmt.Sprintf("/* Critical CSS from %s */\n", match[1]))
		}
	}

	return criticalCSS.String(), nil
}

// extractCriticalJS extracts JavaScript critical for LCP
func (lcp *LCPOptimizer) extractCriticalJS(html string, lcpElement *LCPElement) (string, error) {
	var criticalJS strings.Builder

	// Essential performance monitoring
	criticalJS.WriteString(`
// Critical JS for LCP optimization
window.lcpOptimization = {
	start: performance.now(),
	lcpElement: null,
	measureLCP: function() {
		new PerformanceObserver((list) => {
			for (const entry of list.getEntries()) {
				if (entry.element) {
					this.lcpElement = entry.element;
				}
			}
		}).observe({entryTypes: ['largest-contentful-paint']});
	}
};
window.lcpOptimization.measureLCP();
`)

	// Extract critical inline scripts
	scriptRegex := regexp.MustCompile(`<script[^>]*>([\s\S]*?)</script>`)
	scriptMatches := scriptRegex.FindAllStringSubmatch(html, 2) // Limit to first 2
	for _, match := range scriptMatches {
		if len(match) > 1 && !strings.Contains(match[1], "async") && !strings.Contains(match[1], "defer") {
			criticalJS.WriteString(match[1])
		}
	}

	return criticalJS.String(), nil
}

// inlineCriticalBundle inlines critical CSS and JS into HTML
func (lcp *LCPOptimizer) inlineCriticalBundle(html, css, js string) string {
	optimized := html

	// Inline critical CSS in head
	criticalStyleTag := fmt.Sprintf(`<style data-critical="true">%s</style>`, css)
	headEndRegex := regexp.MustCompile(`</head>`)
	optimized = headEndRegex.ReplaceAllString(optimized, "    "+criticalStyleTag+"\n</head>")

	// Inline critical JS after body start
	criticalScriptTag := fmt.Sprintf(`<script data-critical="true">%s</script>`, js)
	bodyStartRegex := regexp.MustCompile(`<body[^>]*>`)
	optimized = bodyStartRegex.ReplaceAllStringFunc(optimized, func(match string) string {
		return match + "\n    " + criticalScriptTag
	})

	return optimized
}

// compressCSS compresses CSS by removing whitespace and comments
func (lcp *LCPOptimizer) compressCSS(css string) string {
	// Remove comments
	commentRegex := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	compressed := commentRegex.ReplaceAllString(css, "")

	// Remove excess whitespace
	whitespaceRegex := regexp.MustCompile(`\s+`)
	compressed = whitespaceRegex.ReplaceAllString(compressed, " ")

	// Remove spaces around special characters
	spaceRegex := regexp.MustCompile(`\s*([{}:;,>+~])\s*`)
	compressed = spaceRegex.ReplaceAllString(compressed, "$1")

	return strings.TrimSpace(compressed)
}

)
	compressed := singleCommentRegex.ReplaceAllString(js, "")

	// Remove multi-line comments
	multiCommentRegex := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	compressed = multiCommentRegex.ReplaceAllString(compressed, "")

	// Remove excess whitespace
	whitespaceRegex := regexp.MustCompile(`\s+`)
	compressed = whitespaceRegex.ReplaceAllString(compressed, " ")

	return strings.TrimSpace(compressed)
}

// treeShakeCSS removes unused CSS selectors
func (lcp *LCPOptimizer) treeShakeCSS(css, html string) string {
	// Extract all CSS selectors
	selectorRegex := regexp.MustCompile(`([.#]?[a-zA-Z][a-zA-Z0-9_-]*|\[[^\]]+\])\s*{`)
	selectorMatches := selectorRegex.FindAllStringSubmatch(css, -1)

	usedCSS := strings.Builder{}
	cssRules := strings.Split(css, "}")

	for _, rule := range cssRules {
		if !strings.Contains(rule, "{") {
			continue
		}

		parts := strings.Split(rule, "{")
		if len(parts) != 2 {
			continue
		}

		selector := strings.TrimSpace(parts[0])
		
		// Keep rule if selector is found in HTML or is a critical selector
		if lcp.isCriticalSelector(selector) || strings.Contains(html, selector) {
			usedCSS.WriteString(rule + "}")
		}
	}

	return usedCSS.String()
}

// treeShakeJS removes unused JavaScript functions
func (lcp *LCPOptimizer) treeShakeJS(js, html string) string {
	// For now, just keep essential functions
	// In production, this would do proper AST analysis
	essentialFunctions := []string{
		"performance", "PerformanceObserver", "window", "document",
		"addEventListener", "querySelector", "measureLCP",
	}

	lines := strings.Split(js, "\n")
	usedJS := strings.Builder{}

	for _, line := range lines {
		isEssential := false
		for _, fn := range essentialFunctions {
			if strings.Contains(line, fn) {
				isEssential = true
				break
			}
		}

		if isEssential || len(strings.TrimSpace(line)) < 5 {
			usedJS.WriteString(line + "\n")
		}
	}

	return usedJS.String()
}

// isCriticalSelector determines if a CSS selector is critical
func (lcp *LCPOptimizer) isCriticalSelector(selector string) bool {
	criticalSelectors := []string{
		"body", "html", "*", "h1", "h2", "h3", ".hero", ".banner",
		".container", ".wrapper", "img", "picture", "video",
		"@media", "@font-face", ":root",
	}

	selectorLower := strings.ToLower(selector)
	for _, critical := range criticalSelectors {
		if strings.Contains(selectorLower, critical) {
			return true
		}
	}

	return false
}

// identifyLCPElement identifies the likely LCP element in HTML
func (lcp *LCPOptimizer) identifyLCPElement(html string) (*LCPElement, error) {
	// Look for potential LCP elements in order of likelihood
	candidates := []LCPCandidate{}

	// Hero images
	heroImageRegex := regexp.MustCompile(`<img[^>]*?(?:class="[^"]*(?:hero|banner|featured)[^"]*"|id="[^"]*(?:hero|banner|featured)[^"]*")[^>]*?>`)
	heroImages := heroImageRegex.FindAllString(html, -1)
	for i, img := range heroImages {
		candidates = append(candidates, LCPCandidate{
			Element:  img,
			Type:     "image",
			Priority: 100 - i, // First hero image gets highest priority
			Selector: lcp.extractSelector(img),
			Size:     lcp.estimateElementSize(img),
		})
	}

	// Large images above the fold
	imgRegex := regexp.MustCompile(`<img[^>]*?>`)
	images := imgRegex.FindAllString(html, -1)
	for i, img := range images {
		if lcp.isAboveFold(img) {
			size := lcp.estimateElementSize(img)
			if size.Width >= 300 && size.Height >= 200 { // Likely LCP candidates
				candidates = append(candidates, LCPCandidate{
					Element:  img,
					Type:     "image",
					Priority: 80 - i,
					Selector: lcp.extractSelector(img),
					Size:     size,
				})
			}
		}
	}

	// Text blocks (headings, large text)
	textRegex := regexp.MustCompile(`<(?:h1|h2|h3|div class="[^"]*(?:hero|title|heading)[^"]*")[^>]*>([^<]+)</`)
	textElements := textRegex.FindAllString(html, -1)
	for i, text := range textElements {
		if lcp.isAboveFold(text) {
			candidates = append(candidates, LCPCandidate{
				Element:  text,
				Type:     "text",
				Priority: 60 - i,
				Selector: lcp.extractSelector(text),
				Size:     ElementSize{Width: 800, Height: 100}, // Estimated text size
			})
		}
	}

	// Sort candidates by priority
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no LCP candidates found")
	}

	// Return the highest priority candidate
	best := candidates[0]
	return &LCPElement{
		Type:     best.Type,
		Element:  best.Element,
		Selector: best.Selector,
		Size:     best.Size,
		Priority: best.Priority,
		IsHero:   strings.Contains(best.Element, "hero") || strings.Contains(best.Element, "banner"),
	}, nil
}

// LCPElement represents the identified LCP element
type LCPElement struct {
	Type     string      // image, text, video
	Element  string      // HTML element
	Selector string      // CSS selector
	Size     ElementSize // Element dimensions
	Priority int         // Priority score
	IsHero   bool        // Is hero content
}

// LCPCandidate represents a potential LCP element
type LCPCandidate struct {
	Element  string
	Type     string
	Priority int
	Selector string
	Size     ElementSize
}

// ElementSize represents element dimensions
type ElementSize struct {
	Width  int
	Height int
}

// extractSelector extracts a CSS selector from an HTML element
func (lcp *LCPOptimizer) extractSelector(element string) string {
	// Extract ID
	if idMatch := regexp.MustCompile(`id="([^"]+)"`).FindStringSubmatch(element); len(idMatch) > 1 {
		return "#" + idMatch[1]
	}

	// Extract first class
	if classMatch := regexp.MustCompile(`class="([^"]+)"`).FindStringSubmatch(element); len(classMatch) > 1 {
		classes := strings.Fields(classMatch[1])
		if len(classes) > 0 {
			return "." + classes[0]
		}
	}

	// Extract tag name
	if tagMatch := regexp.MustCompile(`<(\w+)`).FindStringSubmatch(element); len(tagMatch) > 1 {
		return tagMatch[1]
	}

	return ""
}

// estimateElementSize estimates the size of an HTML element
func (lcp *LCPOptimizer) estimateElementSize(element string) ElementSize {
	// Try to extract width/height attributes
	if widthMatch := regexp.MustCompile(`width="(\d+)"`).FindStringSubmatch(element); len(widthMatch) > 1 {
		if heightMatch := regexp.MustCompile(`height="(\d+)"`).FindStringSubmatch(element); len(heightMatch) > 1 {
			width, _ := strconv.Atoi(widthMatch[1])
			height, _ := strconv.Atoi(heightMatch[1])
			return ElementSize{Width: width, Height: height}
		}
	}

	// Default sizes based on element type
	if strings.Contains(element, "hero") || strings.Contains(element, "banner") {
		return ElementSize{Width: 1200, Height: 600}
	}
	if strings.Contains(element, "<img") {
		return ElementSize{Width: 800, Height: 400}
	}
	if strings.Contains(element, "<h1") {
		return ElementSize{Width: 800, Height: 80}
	}

	return ElementSize{Width: 400, Height: 200}
}

// isAboveFold determines if an element is likely above the fold
func (lcp *LCPOptimizer) isAboveFold(element string) bool {
	aboveFoldIndicators := []string{
		"hero", "banner", "header", "nav", "main", "featured",
		"h1", "h2", "logo", "title", "headline",
	}

	elementLower := strings.ToLower(element)
	for _, indicator := range aboveFoldIndicators {
		if strings.Contains(elementLower, indicator) {
			return true
		}
	}

	return true // Default to above fold if uncertain
}

// prioritizeResources optimizes resource loading for LCP
func (lcp *LCPOptimizer) prioritizeResources(html string, lcpElement *LCPElement) string {
	optimized := html

	// Identify critical resources for LCP element
	criticalResources := lcp.identifyCriticalResources(html, lcpElement)

	// Add preload hints for critical resources
	for _, resource := range criticalResources {
		if lcp.config.PreloadCriticalResources && resource.Size <= lcp.config.CriticalResourcesMaxSize {
			preloadHint := lcp.generatePreloadHint(resource)
			optimized = lcp.insertPreloadHint(optimized, preloadHint)
		}
	}

	// Add priority hints
	optimized = lcp.addPriorityHints(optimized, lcpElement)

	// Defer non-critical resources
	optimized = lcp.deferNonCriticalResources(optimized, criticalResources)

	return optimized
}

// identifyCriticalResources identifies resources critical for LCP
func (lcp *LCPOptimizer) identifyCriticalResources(html string, lcpElement *LCPElement) []CriticalResource {
	var resources []CriticalResource

	// For image LCP elements, the image is critical
	if lcpElement.Type == "image" {
		if srcMatch := regexp.MustCompile(`src="([^"]+)"`).FindStringSubmatch(lcpElement.Element); len(srcMatch) > 1 {
			resources = append(resources, CriticalResource{
				URL:         srcMatch[1],
				Type:        "image",
				Priority:    10,
				IsAboveFold: true,
				IsHeroImage: lcpElement.IsHero,
			})
		}
	}

	// Critical CSS (above-the-fold styles)
	cssRegex := regexp.MustCompile(`<link[^>]*?rel="stylesheet"[^>]*?href="([^"]+)"[^>]*?>`)
	cssMatches := cssRegex.FindAllStringSubmatch(html, -1)
	for i, match := range cssMatches {
		if len(match) > 1 {
			resources = append(resources, CriticalResource{
				URL:         match[1],
				Type:        "css",
				Priority:    8 - i, // First stylesheets are more critical
				IsAboveFold: true,
			})
		}
	}

	// Critical fonts
	fontRegex := regexp.MustCompile(`@font-face[^}]*?src:\s*url\(["']?([^"')]+)["']?\)`)
	fontMatches := fontRegex.FindAllStringSubmatch(html, -1)
	for _, match := range fontMatches {
		if len(match) > 1 {
			resources = append(resources, CriticalResource{
				URL:         match[1],
				Type:        "font",
				Priority:    7,
				IsAboveFold: true,
			})
		}
	}

	return resources
}

// generatePreloadHint creates a preload hint for a resource
func (lcp *LCPOptimizer) generatePreloadHint(resource CriticalResource) PreloadHint {
	hint := PreloadHint{
		URL:  resource.URL,
		Type: resource.Type,
	}

	switch resource.Type {
	case "image":
		hint.As = "image"
	case "css":
		hint.As = "style"
	case "font":
		hint.As = "font"
		hint.CrossOrigin = "anonymous"
	case "js":
		hint.As = "script"
	}

	return hint
}

// insertPreloadHint inserts a preload hint into the HTML head
func (lcp *LCPOptimizer) insertPreloadHint(html string, hint PreloadHint) string {
	preloadTag := fmt.Sprintf(`<link rel="preload" href="%s" as="%s"`, hint.URL, hint.As)
	
	if hint.CrossOrigin != "" {
		preloadTag += fmt.Sprintf(` crossorigin="%s"`, hint.CrossOrigin)
	}
	
	if hint.MediaQuery != "" {
		preloadTag += fmt.Sprintf(` media="%s"`, hint.MediaQuery)
	}
	
	preloadTag += ">"

	// Insert before closing head tag
	headEndRegex := regexp.MustCompile(`</head>`)
	return headEndRegex.ReplaceAllString(html, "    "+preloadTag+"\n</head>")
}

// addPriorityHints adds fetch priority hints to critical elements
func (lcp *LCPOptimizer) addPriorityHints(html string, lcpElement *LCPElement) string {
	optimized := html

	// Add high priority to LCP element
	if lcpElement.Type == "image" {
		imgRegex := regexp.MustCompile(regexp.QuoteMeta(lcpElement.Element))
		optimized = imgRegex.ReplaceAllStringFunc(optimized, func(match string) string {
			if !strings.Contains(match, "fetchpriority") {
				return strings.Replace(match, "<img", `<img fetchpriority="high"`, 1)
			}
			return match
		})
	}

	// Add high priority to critical CSS
	cssRegex := regexp.MustCompile(`<link([^>]*?)rel="stylesheet"([^>]*?)>`)
	count := 0
	optimized = cssRegex.ReplaceAllStringFunc(optimized, func(match string) string {
		count++
		if count <= 2 && !strings.Contains(match, "fetchpriority") { // First 2 stylesheets
			return strings.Replace(match, "<link", `<link fetchpriority="high"`, 1)
		}
		return match
	})

	return optimized
}

// deferNonCriticalResources defers loading of non-critical resources
func (lcp *LCPOptimizer) deferNonCriticalResources(html string, criticalResources []CriticalResource) string {
	optimized := html

	// Create map of critical resource URLs
	criticalURLs := make(map[string]bool)
	for _, resource := range criticalResources {
		criticalURLs[resource.URL] = true
	}

	// Defer non-critical JavaScript
	jsRegex := regexp.MustCompile(`<script([^>]*?)src="([^"]+)"([^>]*?)>`)
	optimized = jsRegex.ReplaceAllStringFunc(optimized, func(match string) string {
		srcMatch := regexp.MustCompile(`src="([^"]+)"`).FindStringSubmatch(match)
		if len(srcMatch) > 1 {
			if !criticalURLs[srcMatch[1]] && !strings.Contains(match, "defer") && !strings.Contains(match, "async") {
				return strings.Replace(match, "<script", `<script defer`, 1)
			}
		}
		return match
	})

	// Defer non-critical CSS (load asynchronously)
	cssRegex := regexp.MustCompile(`<link([^>]*?)rel="stylesheet"([^>]*?)href="([^"]+)"([^>]*?)>`)
	cssCount := 0
	optimized = cssRegex.ReplaceAllStringFunc(optimized, func(match string) string {
		cssCount++
		hrefMatch := regexp.MustCompile(`href="([^"]+)"`).FindStringSubmatch(match)
		if len(hrefMatch) > 1 {
			if !criticalURLs[hrefMatch[1]] && cssCount > 2 { // Defer CSS after first 2 critical ones
				// Convert to async loading
				return fmt.Sprintf(`<link rel="preload" href="%s" as="style" onload="this.onload=null;this.rel='stylesheet'">
<noscript><link rel="stylesheet" href="%s"></noscript>`, hrefMatch[1], hrefMatch[1])
			}
		}
		return match
	})

	return optimized
}

// optimizeImages optimizes images for LCP performance
func (lcp *LCPOptimizer) optimizeImages(html string, lcpElement *LCPElement) string {
	optimized := html

	// Optimize LCP image specifically
	if lcpElement.Type == "image" {
		optimized = lcp.optimizeLCPImage(optimized, lcpElement)
	}

	// Optimize other images
	optimized = lcp.optimizeRegularImages(optimized)

	return optimized
}

// optimizeLCPImage optimizes the specific LCP image
func (lcp *LCPOptimizer) optimizeLCPImage(html string, lcpElement *LCPElement) string {
	imgRegex := regexp.MustCompile(regexp.QuoteMeta(lcpElement.Element))
	
	return imgRegex.ReplaceAllStringFunc(html, func(match string) string {
		optimized := match

		// Add responsive images
		optimized = lcp.addResponsiveImages(optimized, true) // isLCP = true

		// Ensure no lazy loading for LCP image
		if strings.Contains(optimized, `loading="lazy"`) {
			optimized = strings.Replace(optimized, `loading="lazy"`, `loading="eager"`, 1)
		} else if !strings.Contains(optimized, "loading=") {
			optimized = strings.Replace(optimized, "<img", `<img loading="eager"`, 1)
		}

		// Add decode="async" for better performance
		if !strings.Contains(optimized, "decoding=") {
			optimized = strings.Replace(optimized, "<img", `<img decoding="async"`, 1)
		}

		return optimized
	})
}

// addResponsiveImages adds responsive image support
func (lcp *LCPOptimizer) addResponsiveImages(imgTag string, isLCP bool) string {
	// Extract src attribute
	srcRegex := regexp.MustCompile(`src="([^"]+)"`)
	srcMatch := srcRegex.FindStringSubmatch(imgTag)
	if len(srcMatch) < 2 {
		return imgTag
	}

	originalSrc := srcMatch[1]
	
	// Generate srcset for responsive images
	var srcsetParts []string
	var sizesParts []string
	
	for _, size := range lcp.imageOptimizer.heroImageSizes {
		// Generate optimized URL (in practice, this would call an image service)
		optimizedURL := lcp.generateOptimizedImageURL(originalSrc, size)
		srcsetParts = append(srcsetParts, fmt.Sprintf("%s %dw", optimizedURL, size.Width))
	}
	
	for _, respSize := range lcp.imageOptimizer.responsiveSizes {
		sizesParts = append(sizesParts, fmt.Sprintf("%s %s", respSize.MediaQuery, respSize.Size))
	}

	// Add srcset attribute
	srcset := strings.Join(srcsetParts, ", ")
	sizes := strings.Join(sizesParts, ", ")

	optimized := imgTag
	if !strings.Contains(optimized, "srcset=") {
		optimized = strings.Replace(optimized, fmt.Sprintf(`src="%s"`, originalSrc),
			fmt.Sprintf(`src="%s" srcset="%s" sizes="%s"`, originalSrc, srcset, sizes), 1)
	}

	return optimized
}

// generateOptimizedImageURL generates an optimized image URL
func (lcp *LCPOptimizer) generateOptimizedImageURL(originalURL string, size ImageSize) string {
	// Parse the original URL
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}

	// Add optimization parameters
	query := parsedURL.Query()
	query.Set("w", strconv.Itoa(size.Width))
	query.Set("h", strconv.Itoa(size.Height))
	query.Set("f", size.Format)
	query.Set("q", strconv.Itoa(size.Quality))
	
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

// optimizeRegularImages optimizes non-LCP images
func (lcp *LCPOptimizer) optimizeRegularImages(html string) string {
	imgRegex := regexp.MustCompile(`<img([^>]*?)>`)
	
	return imgRegex.ReplaceAllStringFunc(html, func(match string) string {
		optimized := match

		// Add lazy loading for non-LCP images
		if !strings.Contains(optimized, "loading=") && !strings.Contains(optimized, "fetchpriority") {
			optimized = strings.Replace(optimized, "<img", `<img loading="lazy"`, 1)
		}

		// Add decoding="async"
		if !strings.Contains(optimized, "decoding=") {
			optimized = strings.Replace(optimized, "<img", `<img decoding="async"`, 1)
		}

		return optimized
	})
}

// optimizeFonts optimizes fonts for LCP performance
func (lcp *LCPOptimizer) optimizeFonts(html string, lcpElement *LCPElement) string {
	optimized := html

	// Preload critical fonts
	optimized = lcp.preloadCriticalFonts(optimized)

	// Optimize font display
	optimized = lcp.optimizeFontDisplay(optimized)

	// Add font fallbacks
	optimized = lcp.addFontFallbacks(optimized)

	return optimized
}

// preloadCriticalFonts adds preload hints for critical fonts
func (lcp *LCPOptimizer) preloadCriticalFonts(html string) string {
	criticalFonts := []string{
		"/static/fonts/inter-regular.woff2",
		"/static/fonts/inter-bold.woff2",
	}

	var preloads strings.Builder
	for _, fontURL := range criticalFonts {
		preloads.WriteString(fmt.Sprintf(
			`    <link rel="preload" href="%s" as="font" type="font/woff2" crossorigin="anonymous">%s`,
			fontURL, "\n"))
	}

	// Insert before closing head tag
	headEndRegex := regexp.MustCompile(`</head>`)
	return headEndRegex.ReplaceAllString(html, preloads.String()+"</head>")
}

// optimizeFontDisplay optimizes font-display for better LCP
func (lcp *LCPOptimizer) optimizeFontDisplay(html string) string {
	// Add font-display: swap to CSS
	fontFaceRegex := regexp.MustCompile(`(@font-face\s*{[^}]*?})`)
	
	return fontFaceRegex.ReplaceAllStringFunc(html, func(match string) string {
		if !strings.Contains(match, "font-display") {
			return strings.Replace(match, "}", "  font-display: swap;\n}", 1)
		}
		return match
	})
}

// addFontFallbacks adds system font fallbacks
func (lcp *LCPOptimizer) addFontFallbacks(html string) string {
	// This would typically be done in CSS, but can also be enforced in HTML
	// For now, return as-is since this is usually handled in CSS
	return html
}

// addResourceHints adds various resource hints for performance
func (lcp *LCPOptimizer) addResourceHints(html string) string {
	var hints strings.Builder

	// DNS prefetch for external domains
	hints.WriteString(`    <link rel="dns-prefetch" href="//fonts.googleapis.com">` + "\n")
	hints.WriteString(`    <link rel="dns-prefetch" href="//fonts.gstatic.com">` + "\n")
	
	// Preconnect for critical external resources
	hints.WriteString(`    <link rel="preconnect" href="https://fonts.googleapis.com">` + "\n")
	hints.WriteString(`    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>` + "\n")

	// Insert before closing head tag
	headEndRegex := regexp.MustCompile(`</head>`)
	return headEndRegex.ReplaceAllString(html, hints.String()+"</head>")
}

// updateMetrics updates LCP optimization metrics
func (lcp *LCPOptimizer) updateMetrics(lcpElement *LCPElement) {
	lcp.performanceMetrics.TotalOptimizations++
	lcp.performanceMetrics.LastOptimization = time.Now()
	
	if lcpElement != nil {
		lcp.performanceMetrics.LCPElementType = lcpElement.Type
		lcp.performanceMetrics.LCPElementSelector = lcpElement.Selector
	}

	switch lcpElement.Type {
	case "image":
		lcp.performanceMetrics.ImagesOptimized++
	case "text":
		lcp.performanceMetrics.FontsOptimized++
	}
}

// GenerateReport generates an LCP optimization report
func (lcp *LCPOptimizer) GenerateReport() string {
	metrics := lcp.performanceMetrics
	
	return fmt.Sprintf(`=== LCP Optimization Report ===
Last Optimization: %s
Total Optimizations: %d
Resources Optimized: %d
Images Optimized: %d
Fonts Optimized: %d
Critical Resources: %d
Preloaded Resources: %d

=== LCP Element Analysis ===
Type: %s
Selector: %s

=== Configuration ===
Target LCP: %v
Resource Prioritization: %t
Image Optimization: %t
Font Optimization: %t
CDN Enabled: %t

=== Estimated Impact ===
Expected LCP Improvement: %v
Critical Resource Optimization: %t
Hero Content Prioritization: %t
`,
		metrics.LastOptimization.Format(time.RFC3339),
		metrics.TotalOptimizations,
		metrics.ResourcesOptimized,
		metrics.ImagesOptimized,
		metrics.FontsOptimized,
		metrics.CriticalResourceCount,
		metrics.PreloadedResourceCount,
		metrics.LCPElementType,
		metrics.LCPElementSelector,
		lcp.config.TargetLCP,
		lcp.config.EnableResourcePrioritization,
		lcp.config.EnableImageOptimization,
		lcp.config.EnableFontOptimization,
		lcp.config.CDNEnabled,
		lcp.config.TargetLCP/2, // Estimated improvement
		lcp.config.PreloadCriticalResources,
		lcp.config.EnableHeroPrioritization,
	)
}

// GetMetrics returns current LCP optimization metrics
func (lcp *LCPOptimizer) GetMetrics() LCPMetrics {
	return lcp.performanceMetrics
}

// TemplateFunction returns template functions for LCP optimization
func (lcp *LCPOptimizer) TemplateFunction() template.FuncMap {
	return template.FuncMap{
		"optimizeLCP": func(content string) template.HTML {
			optimized, err := lcp.OptimizeHTML(content)
			if err != nil {
				return template.HTML(content)
			}
			return template.HTML(optimized)
		},
		"heroImage": func(src string, alt string, width, height int) template.HTML {
			// Generate optimized hero image HTML
			sizes := lcp.imageOptimizer.heroImageSizes
			var srcsetParts []string
			
			for _, size := range sizes {
				optimizedURL := lcp.generateOptimizedImageURL(src, size)
				srcsetParts = append(srcsetParts, fmt.Sprintf("%s %dw", optimizedURL, size.Width))
			}
			
			srcset := strings.Join(srcsetParts, ", ")
			sizesAttr := "(min-width: 1200px) 1200px, (min-width: 768px) 800px, 400px"
			
			return template.HTML(fmt.Sprintf(
				`<img src="%s" alt="%s" width="%d" height="%d" srcset="%s" sizes="%s" loading="eager" fetchpriority="high" decoding="async">`,
				src, alt, width, height, srcset, sizesAttr))
		},
		"criticalFont": func(family, weight string) template.HTML {
			// Generate critical font preload
			fontURL := fmt.Sprintf("/static/fonts/%s-%s.woff2", 
				strings.ToLower(strings.ReplaceAll(family, " ", "-")), weight)
			return template.HTML(fmt.Sprintf(
				`<link rel="preload" href="%s" as="font" type="font/woff2" crossorigin="anonymous">`,
				fontURL))
		},
	}
}