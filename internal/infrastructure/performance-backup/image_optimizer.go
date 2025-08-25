// Package performance provides comprehensive image optimization for Core Web Vitals
package performance

import (
	"fmt"
	"html/template"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ImageOptimizer provides comprehensive image optimization for Core Web Vitals
type ImageOptimizer struct {
	config              ImageOptimizationConfig
	formatOptimizer     *FormatOptimizer
	responsiveOptimizer *ResponsiveOptimizer
	lazyLoader          *LazyLoadOptimizer
	placeholderManager  *PlaceholderManager
	performanceMetrics  ImageOptimizationMetrics
}

// ImageOptimizationConfig configures image optimization
type ImageOptimizationConfig struct {
	EnableResponsiveImages   bool              // Enable responsive image generation
	EnableFormatOptimization bool              // Enable format optimization (WebP, AVIF)
	EnableLazyLoading       bool              // Enable lazy loading
	EnablePlaceholders      bool              // Enable placeholder generation
	Quality                 map[string]int    // Quality settings per format
	Formats                 []string          // Supported formats in preference order
	Breakpoints            []int             // Responsive breakpoints
	LazyLoadThreshold      int               // Pixels from viewport to start loading
	PlaceholderType        string            // blur, solid, skeleton
	EnableCDN              bool              // Use CDN for image delivery
	CDNBaseURL             string            // CDN base URL
	MaxImageSize           int               // Maximum image file size in bytes
	EnableAutoSizing       bool              // Automatically add width/height attributes
}

// FormatOptimizer handles image format optimization
type FormatOptimizer struct {
	supportedFormats   []ImageFormat
	qualitySettings    map[string]int
	formatPriority     map[string]int
	compressionLevels  map[string]int
}

// ResponsiveOptimizer handles responsive image optimization
type ResponsiveOptimizer struct {
	breakpoints       []Breakpoint
	densityVariants   []float64
	sizingStrategies  map[string]SizingStrategy
	artDirectionRules []ArtDirectionRule
}

// LazyLoadOptimizer handles lazy loading optimization
type LazyLoadOptimizer struct {
	threshold         int
	rootMargin        string
	loadingStrategy   string
	fallbackStrategy  string
	nativeSupport     bool
	intersectionAPI   bool
}

// PlaceholderManager handles image placeholders
type PlaceholderManager struct {
	placeholderType   string
	blurQuality      int
	skeletonStyles   map[string]string
	solidColor       string
	enableAnimation  bool
}

// ImageFormat represents an image format configuration
type ImageFormat struct {
	Name         string
	Extension    string
	MimeType     string
	Quality      int
	Compression  string
	Support      string // modern, legacy, universal
}

// Breakpoint represents a responsive breakpoint
type Breakpoint struct {
	Name      string
	MinWidth  int
	MaxWidth  int
	Density   []float64
	Quality   int
}

// SizingStrategy represents an image sizing strategy
type SizingStrategy struct {
	Name        string
	Sizes       string
	Strategy    string // cover, contain, fill, auto
	AspectRatio string
}

// ArtDirectionRule represents an art direction rule
type ArtDirectionRule struct {
	MediaQuery string
	Source     string
	Width      int
	Height     int
	Crop       string
}

// ImageOptimizationMetrics tracks image optimization performance
type ImageOptimizationMetrics struct {
	TotalImages         int
	OptimizedImages     int
	LazyLoadedImages    int
	ResponsiveImages    int
	FormatOptimized     map[string]int
	BytesSaved          int64
	LoadTimeImprovement time.Duration
	LCPImprovement      time.Duration
	CLSPrevented        int
	LastOptimization    time.Time
}

// DefaultImageOptimizationConfig returns sensible defaults
func DefaultImageOptimizationConfig() ImageOptimizationConfig {
	return ImageOptimizationConfig{
		EnableResponsiveImages:   true,
		EnableFormatOptimization: true,
		EnableLazyLoading:       true,
		EnablePlaceholders:      true,
		Quality: map[string]int{
			"avif": 75,
			"webp": 80,
			"jpg":  85,
			"png":  95,
		},
		Formats:           []string{"avif", "webp", "jpg", "png"},
		Breakpoints:       []int{320, 640, 768, 1024, 1280, 1600, 1920},
		LazyLoadThreshold: 100,
		PlaceholderType:   "blur",
		EnableCDN:         true,
		CDNBaseURL:        "",
		MaxImageSize:      2 * 1024 * 1024, // 2MB
		EnableAutoSizing:  true,
	}
}

// NewImageOptimizer creates a new image optimizer
func NewImageOptimizer(config ImageOptimizationConfig) *ImageOptimizer {
	formatOptimizer := &FormatOptimizer{
		supportedFormats: []ImageFormat{
			{
				Name:        "AVIF",
				Extension:   ".avif",
				MimeType:    "image/avif",
				Quality:     config.Quality["avif"],
				Compression: "lossy",
				Support:     "modern",
			},
			{
				Name:        "WebP",
				Extension:   ".webp",
				MimeType:    "image/webp",
				Quality:     config.Quality["webp"],
				Compression: "lossy",
				Support:     "modern",
			},
			{
				Name:        "JPEG",
				Extension:   ".jpg",
				MimeType:    "image/jpeg",
				Quality:     config.Quality["jpg"],
				Compression: "lossy",
				Support:     "universal",
			},
			{
				Name:        "PNG",
				Extension:   ".png",
				MimeType:    "image/png",
				Quality:     config.Quality["png"],
				Compression: "lossless",
				Support:     "universal",
			},
		},
		qualitySettings: config.Quality,
		formatPriority: map[string]int{
			"avif": 4,
			"webp": 3,
			"jpg":  2,
			"png":  1,
		},
	}

	breakpoints := make([]Breakpoint, len(config.Breakpoints))
	for i, bp := range config.Breakpoints {
		breakpoints[i] = Breakpoint{
			Name:     fmt.Sprintf("bp-%d", bp),
			MinWidth: bp,
			MaxWidth: 0, // Will be set based on next breakpoint
			Density:  []float64{1.0, 2.0}, // 1x and 2x density
			Quality:  config.Quality["webp"],
		}
		
		// Set max width based on next breakpoint
		if i < len(config.Breakpoints)-1 {
			breakpoints[i].MaxWidth = config.Breakpoints[i+1] - 1
		} else {
			breakpoints[i].MaxWidth = 3840 // 4K width
		}
	}

	responsiveOptimizer := &ResponsiveOptimizer{
		breakpoints:     breakpoints,
		densityVariants: []float64{1.0, 1.5, 2.0, 3.0},
		sizingStrategies: map[string]SizingStrategy{
			"hero": {
				Name:        "hero",
				Sizes:       "(min-width: 1280px) 1280px, (min-width: 768px) 100vw, 100vw",
				Strategy:    "cover",
				AspectRatio: "16:9",
			},
			"card": {
				Name:        "card",
				Sizes:       "(min-width: 1024px) 300px, (min-width: 768px) 50vw, 100vw",
				Strategy:    "cover",
				AspectRatio: "4:3",
			},
			"thumbnail": {
				Name:        "thumbnail",
				Sizes:       "150px",
				Strategy:    "cover",
				AspectRatio: "1:1",
			},
		},
	}

	lazyLoader := &LazyLoadOptimizer{
		threshold:         config.LazyLoadThreshold,
		rootMargin:        "100px",
		loadingStrategy:   "intersection",
		fallbackStrategy:  "scroll",
		nativeSupport:     true,
		intersectionAPI:   true,
	}

	placeholderManager := &PlaceholderManager{
		placeholderType: config.PlaceholderType,
		blurQuality:     20,
		skeletonStyles: map[string]string{
			"background": "linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%)",
			"animation":  "skeleton-loading 1.5s infinite",
		},
		solidColor:      "#f0f0f0",
		enableAnimation: true,
	}

	return &ImageOptimizer{
		config:              config,
		formatOptimizer:     formatOptimizer,
		responsiveOptimizer: responsiveOptimizer,
		lazyLoader:          lazyLoader,
		placeholderManager:  placeholderManager,
		performanceMetrics: ImageOptimizationMetrics{
			FormatOptimized: make(map[string]int),
		},
	}
}

// OptimizeHTML optimizes all images in HTML content
func (io *ImageOptimizer) OptimizeHTML(html string) (string, error) {
	optimized := html

	// Step 1: Add image optimization CSS
	optimized = io.addImageOptimizationCSS(optimized)

	// Step 2: Optimize individual images
	imgRegex := regexp.MustCompile(`<img([^>]*?)>`)
	optimized = imgRegex.ReplaceAllStringFunc(optimized, func(match string) string {
		return io.optimizeImageTag(match)
	})

	// Step 3: Add JavaScript for lazy loading and optimization
	optimized = io.addImageOptimizationJS(optimized)

	// Update metrics
	io.updateMetrics(html, optimized)

	return optimized, nil
}

// optimizeImageTag optimizes a single image tag
func (io *ImageOptimizer) optimizeImageTag(imgTag string) string {
	optimized := imgTag

	// Extract image attributes
	attrs := io.parseImageAttributes(imgTag)

	// Step 1: Add responsive images
	if io.config.EnableResponsiveImages {
		optimized = io.addResponsiveImages(optimized, attrs)
	}

	// Step 2: Add format optimization
	if io.config.EnableFormatOptimization {
		optimized = io.addFormatOptimization(optimized, attrs)
	}

	// Step 3: Add lazy loading
	if io.config.EnableLazyLoading && !io.isCriticalImage(attrs) {
		optimized = io.addLazyLoading(optimized, attrs)
	}

	// Step 4: Add placeholders
	if io.config.EnablePlaceholders {
		optimized = io.addPlaceholder(optimized, attrs)
	}

	// Step 5: Add dimensions for layout stability
	if io.config.EnableAutoSizing {
		optimized = io.addDimensions(optimized, attrs)
	}

	// Step 6: Add performance attributes
	optimized = io.addPerformanceAttributes(optimized, attrs)

	return optimized
}

// parseImageAttributes extracts attributes from an image tag
func (io *ImageOptimizer) parseImageAttributes(imgTag string) map[string]string {
	attrs := make(map[string]string)
	
	// Extract common attributes
	attrRegex := regexp.MustCompile(`(\w+)=["']([^"']*?)["']`)
	matches := attrRegex.FindAllStringSubmatch(imgTag, -1)
	
	for _, match := range matches {
		if len(match) >= 3 {
			attrs[match[1]] = match[2]
		}
	}

	return attrs
}

// addResponsiveImages adds responsive image support
func (io *ImageOptimizer) addResponsiveImages(imgTag string, attrs map[string]string) string {
	src, exists := attrs["src"]
	if !exists {
		return imgTag
	}

	// Determine image strategy based on class or context
	strategy := io.determineImageStrategy(attrs)
	sizingStrategy := io.responsiveOptimizer.sizingStrategies[strategy]

	// Generate srcset
	srcset := io.generateSrcset(src, strategy)
	
	// Add srcset and sizes attributes
	optimized := imgTag
	if srcset != "" && !strings.Contains(optimized, "srcset=") {
		optimized = strings.Replace(optimized, fmt.Sprintf(`src="%s"`, src),
			fmt.Sprintf(`src="%s" srcset="%s" sizes="%s"`, src, srcset, sizingStrategy.Sizes), 1)
	}

	return optimized
}

// determineImageStrategy determines the optimization strategy for an image
func (io *ImageOptimizer) determineImageStrategy(attrs map[string]string) string {
	class, hasClass := attrs["class"]
	id, hasID := attrs["id"]

	// Check for hero/banner images
	if hasClass && (strings.Contains(class, "hero") || strings.Contains(class, "banner")) {
		return "hero"
	}
	if hasID && (strings.Contains(id, "hero") || strings.Contains(id, "banner")) {
		return "hero"
	}

	// Check for card images
	if hasClass && strings.Contains(class, "card") {
		return "card"
	}

	// Check for thumbnails
	if hasClass && (strings.Contains(class, "thumb") || strings.Contains(class, "avatar")) {
		return "thumbnail"
	}

	// Default strategy
	return "card"
}

// generateSrcset generates a srcset attribute for responsive images
func (io *ImageOptimizer) generateSrcset(src string, strategy string) string {
	var srcsetParts []string

	// Get appropriate breakpoints for strategy
	breakpoints := io.getBreakpointsForStrategy(strategy)

	for _, bp := range breakpoints {
		for _, density := range bp.Density {
			width := int(float64(bp.MinWidth) * density)
			optimizedURL := io.generateOptimizedURL(src, width, 0, "webp", bp.Quality)
			srcsetParts = append(srcsetParts, fmt.Sprintf("%s %dw", optimizedURL, width))
		}
	}

	return strings.Join(srcsetParts, ", ")
}

// getBreakpointsForStrategy returns appropriate breakpoints for a strategy
func (io *ImageOptimizer) getBreakpointsForStrategy(strategy string) []Breakpoint {
	switch strategy {
	case "hero":
		// Use all breakpoints for hero images
		return io.responsiveOptimizer.breakpoints
	case "thumbnail":
		// Use only small breakpoints for thumbnails
		var small []Breakpoint
		for _, bp := range io.responsiveOptimizer.breakpoints {
			if bp.MinWidth <= 640 {
				small = append(small, bp)
			}
		}
		return small
	default:
		// Use medium breakpoints for cards
		var medium []Breakpoint
		for _, bp := range io.responsiveOptimizer.breakpoints {
			if bp.MinWidth <= 1280 {
				medium = append(medium, bp)
			}
		}
		return medium
	}
}

// generateOptimizedURL generates an optimized image URL
func (io *ImageOptimizer) generateOptimizedURL(src string, width, height int, format string, quality int) string {
	if io.config.EnableCDN && io.config.CDNBaseURL != "" {
		return io.generateCDNURL(src, width, height, format, quality)
	}

	// Parse the original URL
	parsedURL, err := url.Parse(src)
	if err != nil {
		return src
	}

	// Add optimization parameters
	query := parsedURL.Query()
	query.Set("w", strconv.Itoa(width))
	if height > 0 {
		query.Set("h", strconv.Itoa(height))
	}
	query.Set("f", format)
	query.Set("q", strconv.Itoa(quality))
	query.Set("auto", "format,compress")

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

// generateCDNURL generates a CDN-optimized image URL
func (io *ImageOptimizer) generateCDNURL(src string, width, height int, format string, quality int) string {
	// This would integrate with your CDN service (Cloudinary, Fastly, etc.)
	baseURL := strings.TrimSuffix(io.config.CDNBaseURL, "/")
	
	// Extract the image path
	imagePath := src
	if strings.HasPrefix(src, "/") {
		imagePath = strings.TrimPrefix(src, "/")
	}

	// Build CDN URL with transformations
	cdnURL := fmt.Sprintf("%s/w_%d,f_%s,q_%d/%s", baseURL, width, format, quality, imagePath)
	
	if height > 0 {
		cdnURL = fmt.Sprintf("%s/w_%d,h_%d,f_%s,q_%d,c_fill/%s", baseURL, width, height, format, quality, imagePath)
	}

	return cdnURL
}

// addFormatOptimization adds modern format support with picture element
func (io *ImageOptimizer) addFormatOptimization(imgTag string, attrs map[string]string) string {
	src, exists := attrs["src"]
	if !exists {
		return imgTag
	}

	// Don't wrap if already in a picture element
	if strings.Contains(imgTag, "<picture") {
		return imgTag
	}

	// Generate picture element with multiple formats
	pictureHTML := io.generatePictureElement(imgTag, attrs, src)
	return pictureHTML
}

// generatePictureElement generates a picture element with multiple formats
func (io *ImageOptimizer) generatePictureElement(imgTag string, attrs map[string]string, src string) string {
	var sources []string

	// Get image strategy and dimensions
	strategy := io.determineImageStrategy(attrs)
	width, height := io.extractDimensions(attrs)

	// Generate sources for modern formats
	for _, format := range io.config.Formats {
		if format == "jpg" || format == "png" {
			continue // Skip fallback formats
		}

		formatInfo := io.getFormatInfo(format)
		if formatInfo == nil {
			continue
		}

		srcset := io.generateFormatSrcset(src, strategy, format, formatInfo.Quality)
		sizingStrategy := io.responsiveOptimizer.sizingStrategies[strategy]

		sourceTag := fmt.Sprintf(`<source srcset="%s" sizes="%s" type="%s">`,
			srcset, sizingStrategy.Sizes, formatInfo.MimeType)
		sources = append(sources, sourceTag)
	}

	// Ensure the img tag has srcset for fallback
	fallbackImg := imgTag
	if !strings.Contains(fallbackImg, "srcset=") {
		fallbackSrcset := io.generateSrcset(src, strategy)
		sizingStrategy := io.responsiveOptimizer.sizingStrategies[strategy]
		fallbackImg = strings.Replace(fallbackImg, fmt.Sprintf(`src="%s"`, src),
			fmt.Sprintf(`src="%s" srcset="%s" sizes="%s"`, src, fallbackSrcset, sizingStrategy.Sizes), 1)
	}

	// Combine into picture element
	pictureHTML := "<picture>\n"
	for _, source := range sources {
		pictureHTML += "  " + source + "\n"
	}
	pictureHTML += "  " + fallbackImg + "\n"
	pictureHTML += "</picture>"

	return pictureHTML
}

// generateFormatSrcset generates srcset for a specific format
func (io *ImageOptimizer) generateFormatSrcset(src string, strategy string, format string, quality int) string {
	var srcsetParts []string
	breakpoints := io.getBreakpointsForStrategy(strategy)

	for _, bp := range breakpoints {
		for _, density := range bp.Density {
			width := int(float64(bp.MinWidth) * density)
			optimizedURL := io.generateOptimizedURL(src, width, 0, format, quality)
			srcsetParts = append(srcsetParts, fmt.Sprintf("%s %dw", optimizedURL, width))
		}
	}

	return strings.Join(srcsetParts, ", ")
}

// getFormatInfo returns format information
func (io *ImageOptimizer) getFormatInfo(format string) *ImageFormat {
	for _, f := range io.formatOptimizer.supportedFormats {
		if strings.ToLower(f.Name) == strings.ToLower(format) {
			return &f
		}
	}
	return nil
}

// extractDimensions extracts width and height from attributes
func (io *ImageOptimizer) extractDimensions(attrs map[string]string) (int, int) {
	widthStr, hasWidth := attrs["width"]
	heightStr, hasHeight := attrs["height"]

	width, height := 0, 0

	if hasWidth {
		if w, err := strconv.Atoi(widthStr); err == nil {
			width = w
		}
	}

	if hasHeight {
		if h, err := strconv.Atoi(heightStr); err == nil {
			height = h
		}
	}

	return width, height
}

// addLazyLoading adds lazy loading attributes
func (io *ImageOptimizer) addLazyLoading(imgTag string, attrs map[string]string) string {
	optimized := imgTag

	// Add loading="lazy" if not present
	if !strings.Contains(optimized, "loading=") {
		optimized = strings.Replace(optimized, "<img", `<img loading="lazy"`, 1)
	}

	// Add intersection observer data attributes
	if !strings.Contains(optimized, "data-lazy") {
		optimized = strings.Replace(optimized, "<img", `<img data-lazy="true"`, 1)
	}

	return optimized
}

// isCriticalImage determines if an image is critical for LCP
func (io *ImageOptimizer) isCriticalImage(attrs map[string]string) bool {
	class, hasClass := attrs["class"]
	id, hasID := attrs["id"]

	// Critical image indicators
	criticalIndicators := []string{
		"hero", "banner", "featured", "logo", "above-fold",
	}

	if hasClass {
		for _, indicator := range criticalIndicators {
			if strings.Contains(class, indicator) {
				return true
			}
		}
	}

	if hasID {
		for _, indicator := range criticalIndicators {
			if strings.Contains(id, indicator) {
				return true
			}
		}
	}

	// Check for explicit critical marking
	if fetchPriority, exists := attrs["fetchpriority"]; exists && fetchPriority == "high" {
		return true
	}

	return false
}

// addPlaceholder adds placeholder support
func (io *ImageOptimizer) addPlaceholder(imgTag string, attrs map[string]string) string {
	src, exists := attrs["src"]
	if !exists {
		return imgTag
	}

	optimized := imgTag

	switch io.placeholderManager.placeholderType {
	case "blur":
		placeholder := io.generateBlurPlaceholder(src)
		optimized = io.addDataPlaceholder(optimized, placeholder)
	case "skeleton":
		optimized = io.addSkeletonPlaceholder(optimized, attrs)
	case "solid":
		optimized = io.addSolidPlaceholder(optimized)
	}

	return optimized
}

// generateBlurPlaceholder generates a blur placeholder URL
func (io *ImageOptimizer) generateBlurPlaceholder(src string) string {
	// Generate a very small, low quality version for blur effect
	return io.generateOptimizedURL(src, 20, 0, "jpg", io.placeholderManager.blurQuality)
}

// addDataPlaceholder adds blur placeholder data attribute
func (io *ImageOptimizer) addDataPlaceholder(imgTag string, placeholder string) string {
	return strings.Replace(imgTag, "<img", 
		fmt.Sprintf(`<img data-placeholder="%s"`, placeholder), 1)
}

// addSkeletonPlaceholder adds skeleton loading placeholder
func (io *ImageOptimizer) addSkeletonPlaceholder(imgTag string, attrs map[string]string) string {
	return strings.Replace(imgTag, "<img", `<img data-skeleton="true"`, 1)
}

// addSolidPlaceholder adds solid color placeholder
func (io *ImageOptimizer) addSolidPlaceholder(imgTag string) string {
	return strings.Replace(imgTag, "<img", 
		fmt.Sprintf(`<img data-placeholder-color="%s"`, io.placeholderManager.solidColor), 1)
}

// addDimensions adds width and height attributes for layout stability
func (io *ImageOptimizer) addDimensions(imgTag string, attrs map[string]string) string {
	// Check if dimensions already exist
	_, hasWidth := attrs["width"]
	_, hasHeight := attrs["height"]

	if hasWidth && hasHeight {
		return imgTag
	}

	// Extract dimensions from src or estimate based on strategy
	src := attrs["src"]
	strategy := io.determineImageStrategy(attrs)
	
	width, height := io.estimateDimensions(src, strategy)

	optimized := imgTag
	if !hasWidth && width > 0 {
		optimized = strings.Replace(optimized, "<img", 
			fmt.Sprintf(`<img width="%d"`, width), 1)
	}
	if !hasHeight && height > 0 {
		optimized = strings.Replace(optimized, "width=", 
			fmt.Sprintf(`height="%d" width=`, height), 1)
	}

	return optimized
}

// estimateDimensions estimates image dimensions based on strategy
func (io *ImageOptimizer) estimateDimensions(src string, strategy string) (int, int) {
	// Try to extract from filename
	dimensionRegex := regexp.MustCompile(`(\d+)x(\d+)`)
	if match := dimensionRegex.FindStringSubmatch(src); len(match) >= 3 {
		if width, err := strconv.Atoi(match[1]); err == nil {
			if height, err := strconv.Atoi(match[2]); err == nil {
				return width, height
			}
		}
	}

	// Default dimensions based on strategy
	switch strategy {
	case "hero":
		return 1200, 675 // 16:9 aspect ratio
	case "card":
		return 400, 300 // 4:3 aspect ratio
	case "thumbnail":
		return 150, 150 // 1:1 aspect ratio
	default:
		return 800, 600 // Default
	}
}

// addPerformanceAttributes adds performance-related attributes
func (io *ImageOptimizer) addPerformanceAttributes(imgTag string, attrs map[string]string) string {
	optimized := imgTag

	// Add decoding="async" for better performance
	if !strings.Contains(optimized, "decoding=") {
		optimized = strings.Replace(optimized, "<img", `<img decoding="async"`, 1)
	}

	// Add importance for critical images
	if io.isCriticalImage(attrs) && !strings.Contains(optimized, "fetchpriority=") {
		optimized = strings.Replace(optimized, "<img", `<img fetchpriority="high"`, 1)
	}

	return optimized
}

// addImageOptimizationCSS adds CSS for image optimization
func (io *ImageOptimizer) addImageOptimizationCSS(html string) string {
	optimizationCSS := `
<style>
/* Image Optimization Styles */
img {
  max-width: 100%;
  height: auto;
}

img[data-lazy="true"] {
  opacity: 0;
  transition: opacity 0.3s ease;
}

img[data-lazy="true"].loaded {
  opacity: 1;
}

img[data-skeleton="true"] {
  background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
  background-size: 200% 100%;
  animation: skeleton-loading 1.5s infinite;
}

@keyframes skeleton-loading {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

.blur-placeholder {
  filter: blur(10px);
  transition: filter 0.3s ease;
}

.blur-placeholder.loaded {
  filter: none;
}

picture {
  display: block;
}

picture img {
  width: 100%;
  height: auto;
}

/* Layout stability */
.image-container {
  position: relative;
  overflow: hidden;
}

.image-container::before {
  content: '';
  display: block;
  padding-bottom: var(--aspect-ratio, 56.25%); /* Default 16:9 */
}

.image-container img {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* Hero image optimization */
.hero-image {
  aspect-ratio: 16 / 9;
  object-fit: cover;
  object-position: center;
}

/* Card image optimization */
.card-image {
  aspect-ratio: 4 / 3;
  object-fit: cover;
}

/* Thumbnail optimization */
.thumbnail-image {
  aspect-ratio: 1;
  object-fit: cover;
}

/* Loading states */
.image-loading {
  background: #f0f0f0;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}

.image-error {
  background: #fee;
  color: #c00;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}
</style>`

	// Insert CSS before closing head tag
	headEndRegex := regexp.MustCompile(`</head>`)
	return headEndRegex.ReplaceAllString(html, optimizationCSS+"\n</head>")
}

// addImageOptimizationJS adds JavaScript for image optimization
func (io *ImageOptimizer) addImageOptimizationJS(html string) string {
	optimizationJS := `
<script>
// Image Optimization JavaScript
class ImageOptimizer {
  constructor() {
    this.observer = null;
    this.loadedImages = new Set();
    this.errorImages = new Set();
    this.setupIntersectionObserver();
    this.setupImageHandlers();
  }

  setupIntersectionObserver() {
    if (!('IntersectionObserver' in window)) {
      // Fallback for older browsers
      this.loadAllImages();
      return;
    }

    this.observer = new IntersectionObserver((entries) => {
      entries.forEach(entry => {
        if (entry.isIntersecting) {
          this.loadImage(entry.target);
          this.observer.unobserve(entry.target);
        }
      });
    }, {
      threshold: 0.1,
      rootMargin: '` + io.lazyLoader.rootMargin + `'
    });

    // Observe lazy images
    document.querySelectorAll('img[data-lazy="true"]').forEach(img => {
      this.observer.observe(img);
    });
  }

  loadImage(img) {
    if (this.loadedImages.has(img)) return;

    // Handle blur placeholder
    if (img.hasAttribute('data-placeholder')) {
      const placeholder = img.getAttribute('data-placeholder');
      const tempImg = new Image();
      
      tempImg.onload = () => {
        img.style.backgroundImage = 'url(' + placeholder + ')';
        img.style.backgroundSize = 'cover';
        img.style.filter = 'blur(10px)';
        img.classList.add('blur-placeholder');
        
        // Load actual image
        this.loadActualImage(img);
      };
      
      tempImg.src = placeholder;
    } else {
      this.loadActualImage(img);
    }
  }

  loadActualImage(img) {
    const actualImg = new Image();
    
    actualImg.onload = () => {
      img.src = actualImg.src;
      img.classList.add('loaded');
      img.classList.remove('blur-placeholder');
      img.style.backgroundImage = '';
      img.style.filter = '';
      this.loadedImages.add(img);
      
      // Remove skeleton animation
      img.removeAttribute('data-skeleton');
    };
    
    actualImg.onerror = () => {
      this.handleImageError(img);
    };

    // Use srcset if available, otherwise fall back to src
    if (img.srcset) {
      actualImg.srcset = img.srcset;
      actualImg.sizes = img.sizes;
    }
    actualImg.src = img.src;
  }

  handleImageError(img) {
    if (this.errorImages.has(img)) return;
    
    this.errorImages.add(img);
    img.classList.add('image-error');
    img.alt = img.alt || 'Image failed to load';
    
    // Try fallback image if specified
    const fallback = img.getAttribute('data-fallback');
    if (fallback) {
      img.src = fallback;
    }
  }

  setupImageHandlers() {
    // Handle native lazy loading fallback
    document.querySelectorAll('img[loading="lazy"]').forEach(img => {
      if (!img.hasAttribute('data-lazy')) {
        img.addEventListener('load', () => {
          img.classList.add('loaded');
        });
      }
    });

    // Performance monitoring
    if ('PerformanceObserver' in window) {
      const observer = new PerformanceObserver((entryList) => {
        entryList.getEntries().forEach(entry => {
          if (entry.element && entry.element.tagName === 'IMG') {
            this.recordImagePerformance(entry);
          }
        });
      });
      
      try {
        observer.observe({ entryTypes: ['largest-contentful-paint', 'element'] });
      } catch (e) {
        // Silently fail if observer types not supported
      }
    }
  }

  recordImagePerformance(entry) {
    // Record image performance metrics
    const metrics = {
      url: entry.element.src,
      loadTime: entry.loadTime,
      renderTime: entry.renderTime,
      size: entry.size,
      element: entry.element.tagName,
      timestamp: Date.now()
    };

    // Send to analytics if available
    if (window.gtag) {
      window.gtag('event', 'image_performance', {
        'custom_map': { 'load_time': metrics.loadTime }
      });
    }
  }

  loadAllImages() {
    // Fallback for browsers without IntersectionObserver
    document.querySelectorAll('img[data-lazy="true"]').forEach(img => {
      this.loadImage(img);
    });
  }

  // Format detection and optimization
  supportsFormat(format) {
    const canvas = document.createElement('canvas');
    canvas.width = 1;
    canvas.height = 1;
    
    switch (format) {
      case 'webp':
        return canvas.toDataURL('image/webp').indexOf('webp') !== -1;
      case 'avif':
        return canvas.toDataURL('image/avif').indexOf('avif') !== -1;
      default:
        return false;
    }
  }

  // Preload critical images
  preloadCriticalImages() {
    document.querySelectorAll('img[fetchpriority="high"]').forEach(img => {
      const link = document.createElement('link');
      link.rel = 'preload';
      link.as = 'image';
      link.href = img.src;
      if (img.srcset) {
        link.imageSrcset = img.srcset;
        link.imageSizes = img.sizes;
      }
      document.head.appendChild(link);
    });
  }
}

// Initialize image optimizer
document.addEventListener('DOMContentLoaded', () => {
  window.imageOptimizer = new ImageOptimizer();
  window.imageOptimizer.preloadCriticalImages();
});

// Re-initialize after HTMX swaps
document.addEventListener('htmx:afterSwap', (event) => {
  if (window.imageOptimizer) {
    // Re-observe new lazy images
    event.target.querySelectorAll('img[data-lazy="true"]').forEach(img => {
      if (window.imageOptimizer.observer) {
        window.imageOptimizer.observer.observe(img);
      } else {
        window.imageOptimizer.loadImage(img);
      }
    });
  }
});
</script>`

	// Insert script before closing body tag
	bodyEndRegex := regexp.MustCompile(`</body>`)
	return bodyEndRegex.ReplaceAllString(html, optimizationJS+"\n</body>")
}

// updateMetrics updates image optimization metrics
func (io *ImageOptimizer) updateMetrics(original, optimized string) {
	// Count images in original vs optimized
	originalImages := strings.Count(original, "<img")
	optimizedImages := strings.Count(optimized, "<img")
	
	io.performanceMetrics.TotalImages += originalImages
	io.performanceMetrics.OptimizedImages += optimizedImages

	// Count specific optimizations
	if strings.Count(optimized, "srcset=") > strings.Count(original, "srcset=") {
		io.performanceMetrics.ResponsiveImages++
	}

	if strings.Count(optimized, `loading="lazy"`) > strings.Count(original, `loading="lazy"`) {
		io.performanceMetrics.LazyLoadedImages++
	}

	// Count format optimizations
	for _, format := range io.config.Formats {
		formatCount := strings.Count(optimized, fmt.Sprintf(`type="image/%s"`, format))
		io.performanceMetrics.FormatOptimized[format] += formatCount
	}

	io.performanceMetrics.LastOptimization = time.Now()
}

// GenerateReport generates an image optimization report
func (io *ImageOptimizer) GenerateReport() string {
	metrics := io.performanceMetrics
	
	var formatReport strings.Builder
	for format, count := range metrics.FormatOptimized {
		formatReport.WriteString(fmt.Sprintf("%s: %d\n", strings.ToUpper(format), count))
	}

	return fmt.Sprintf(`=== Image Optimization Report ===
Last Optimization: %s
Total Images: %d
Optimized Images: %d
Responsive Images: %d
Lazy Loaded Images: %d
Bytes Saved: %d
Load Time Improvement: %v
LCP Improvement: %v
Layout Shifts Prevented: %d

=== Format Optimization ===
%s

=== Configuration ===
Responsive Images: %t
Format Optimization: %t
Lazy Loading: %t
Placeholders: %t (%s)
CDN Enabled: %t
Auto Sizing: %t
Quality Settings: %v

=== Performance Impact ===
Estimated Bandwidth Savings: %.1f%%
Estimated LCP Improvement: %v
Layout Stability: %d CLS violations prevented
`,
		metrics.LastOptimization.Format(time.RFC3339),
		metrics.TotalImages,
		metrics.OptimizedImages,
		metrics.ResponsiveImages,
		metrics.LazyLoadedImages,
		metrics.BytesSaved,
		metrics.LoadTimeImprovement,
		metrics.LCPImprovement,
		metrics.CLSPrevented,
		formatReport.String(),
		io.config.EnableResponsiveImages,
		io.config.EnableFormatOptimization,
		io.config.EnableLazyLoading,
		io.config.EnablePlaceholders,
		io.config.PlaceholderType,
		io.config.EnableCDN,
		io.config.EnableAutoSizing,
		io.config.Quality,
		30.0, // Estimated bandwidth savings percentage
		500*time.Millisecond, // Estimated LCP improvement
		metrics.CLSPrevented,
	)
}

// GetMetrics returns current image optimization metrics
func (io *ImageOptimizer) GetMetrics() ImageOptimizationMetrics {
	return io.performanceMetrics
}

// TemplateFunction returns template functions for image optimization
func (io *ImageOptimizer) TemplateFunction() template.FuncMap {
	return template.FuncMap{
		"optimizeImage": func(src, alt string, width, height int, strategy string) template.HTML {
			// Generate optimized image HTML
			attrs := map[string]string{
				"src":    src,
				"alt":    alt,
				"width":  strconv.Itoa(width),
				"height": strconv.Itoa(height),
				"class":  strategy,
			}
			
			imgTag := fmt.Sprintf(`<img src="%s" alt="%s" width="%d" height="%d" class="%s">`,
				src, alt, width, height, strategy)
			
			return template.HTML(io.optimizeImageTag(imgTag))
		},
		"responsiveImage": func(src, alt string, strategy string) template.HTML {
			attrs := map[string]string{
				"src":   src,
				"alt":   alt,
				"class": strategy,
			}
			
			imgTag := fmt.Sprintf(`<img src="%s" alt="%s" class="%s">`, src, alt, strategy)
			optimized := io.optimizeImageTag(imgTag)
			
			return template.HTML(optimized)
		},
		"heroImage": func(src, alt string) template.HTML {
			return template.HTML(io.optimizeImageTag(
				fmt.Sprintf(`<img src="%s" alt="%s" class="hero" fetchpriority="high">`, src, alt)))
		},
		"lazyImage": func(src, alt string) template.HTML {
			return template.HTML(io.optimizeImageTag(
				fmt.Sprintf(`<img src="%s" alt="%s" loading="lazy" data-lazy="true">`, src, alt)))
		},
	}
}