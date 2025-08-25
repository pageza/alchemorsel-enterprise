// Package performance provides layout stability optimization to prevent CLS
package performance

import (
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LayoutStabilizer prevents Cumulative Layout Shift by analyzing and optimizing layout
type LayoutStabilizer struct {
	config           LayoutStabilityConfig
	imageProcessor   *ImageProcessor
	fontProcessor    *FontProcessor
	containerSpecs   map[string]ContainerSpec
	elementSpecs     map[string]ElementSpec
	performanceMetrics LayoutMetrics
}

// LayoutStabilityConfig configures layout stability optimization
type LayoutStabilityConfig struct {
	EnableAutoSizing        bool    // Automatically add size attributes
	EnableFontOptimization  bool    // Optimize font loading to prevent FOIT/FOUT
	EnableImageOptimization bool    // Optimize image loading
	EnableContentReservation bool   // Reserve space for dynamic content
	MaxCLSScore            float64 // Maximum acceptable CLS score
	EnableLayoutHints      bool    // Add layout hints for better performance
	DefaultAspectRatio     string  // Default aspect ratio for images (16:9, 4:3, etc.)
}

// ImageProcessor handles image optimization for layout stability
type ImageProcessor struct {
	supportedFormats   []string
	defaultDimensions  ImageDimensions
	placeholderEnabled bool
	lazyLoadEnabled    bool
	responsiveEnabled  bool
}

// FontProcessor handles font optimization
type FontProcessor struct {
	fontDisplayStrategy string // swap, fallback, optional, auto
	preloadFonts       []string
	fallbackFonts      map[string]string
	fontMetrics        map[string]FontMetrics
}

// ContainerSpec defines container specifications for layout stability
type ContainerSpec struct {
	Selector    string
	MinHeight   string
	MaxHeight   string
	AspectRatio string
	FlexGrow    string
	FlexShrink  string
}

// ElementSpec defines element specifications
type ElementSpec struct {
	Tag         string
	Selector    string
	DefaultSize Size
	MinSize     Size
	MaxSize     Size
	AspectRatio string
	Critical    bool
}

// ImageDimensions represents image dimensions
type ImageDimensions struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Unit   string `json:"unit"` // px, %, vw, vh, etc.
}

// FontMetrics represents font loading metrics
type FontMetrics struct {
	FontFamily     string
	LoadTime       time.Duration
	FallbackMetric float64
	SwapPeriod     time.Duration
	DisplayPeriod  time.Duration
}

// LayoutMetrics tracks layout stability performance
type LayoutMetrics struct {
	TotalElements        int
	StabilizedElements   int
	CLSViolations        int
	AverageCLSScore      float64
	LastOptimization     time.Time
	ElementsProcessed    map[string]int
	OptimizationTypes    map[string]int
}

// AspectRatioMap defines common aspect ratios
var AspectRatioMap = map[string]float64{
	"16:9":  16.0 / 9.0,
	"4:3":   4.0 / 3.0,
	"3:2":   3.0 / 2.0,
	"1:1":   1.0,
	"9:16":  9.0 / 16.0,
	"21:9":  21.0 / 9.0,
	"2:1":   2.0,
}

// DefaultLayoutStabilityConfig returns sensible defaults
func DefaultLayoutStabilityConfig() LayoutStabilityConfig {
	return LayoutStabilityConfig{
		EnableAutoSizing:        true,
		EnableFontOptimization:  true,
		EnableImageOptimization: true,
		EnableContentReservation: true,
		MaxCLSScore:            0.1, // Google's "Good" threshold
		EnableLayoutHints:      true,
		DefaultAspectRatio:     "16:9",
	}
}

// NewLayoutStabilizer creates a new layout stabilizer
func NewLayoutStabilizer(config LayoutStabilityConfig) *LayoutStabilizer {
	imageProcessor := &ImageProcessor{
		supportedFormats: []string{"webp", "avif", "jpg", "jpeg", "png", "svg"},
		defaultDimensions: ImageDimensions{
			Width:  800,
			Height: 600,
			Unit:   "px",
		},
		placeholderEnabled: true,
		lazyLoadEnabled:    true,
		responsiveEnabled:  true,
	}

	fontProcessor := &FontProcessor{
		fontDisplayStrategy: "swap", // Best for CLS prevention
		preloadFonts:       []string{},
		fallbackFonts: map[string]string{
			"Inter":          "system-ui, -apple-system, sans-serif",
			"Roboto":         "system-ui, -apple-system, sans-serif",
			"Open Sans":      "system-ui, -apple-system, sans-serif",
			"Source Sans Pro": "system-ui, -apple-system, sans-serif",
		},
		fontMetrics: make(map[string]FontMetrics),
	}

	return &LayoutStabilizer{
		config:         config,
		imageProcessor: imageProcessor,
		fontProcessor:  fontProcessor,
		containerSpecs: make(map[string]ContainerSpec),
		elementSpecs:   make(map[string]ElementSpec),
		performanceMetrics: LayoutMetrics{
			ElementsProcessed:  make(map[string]int),
			OptimizationTypes: make(map[string]int),
		},
	}
}

// StabilizeHTML optimizes HTML for layout stability
func (ls *LayoutStabilizer) StabilizeHTML(html string) (string, error) {
	optimized := html

	// Step 1: Optimize images for layout stability
	if ls.config.EnableImageOptimization {
		var err error
		optimized, err = ls.optimizeImages(optimized)
		if err != nil {
			return "", fmt.Errorf("image optimization failed: %w", err)
		}
	}

	// Step 2: Optimize fonts for layout stability
	if ls.config.EnableFontOptimization {
		var err error
		optimized, err = ls.optimizeFonts(optimized)
		if err != nil {
			return "", fmt.Errorf("font optimization failed: %w", err)
		}
	}

	// Step 3: Add container specifications
	if ls.config.EnableContentReservation {
		optimized = ls.addContainerSpecs(optimized)
	}

	// Step 4: Add layout hints
	if ls.config.EnableLayoutHints {
		optimized = ls.addLayoutHints(optimized)
	}

	// Step 5: Validate and measure improvements
	ls.updateMetrics(html, optimized)

	return optimized, nil
}

// optimizeImages adds dimensions and optimizes image loading for CLS prevention
func (ls *LayoutStabilizer) optimizeImages(html string) (string, error) {
	// Regex to find img tags
	imgRegex := regexp.MustCompile(`<img([^>]*?)>`)
	
	return imgRegex.ReplaceAllStringFunc(html, func(match string) string {
		return ls.processImageTag(match)
	}), nil
}

// processImageTag processes a single image tag for layout stability
func (ls *LayoutStabilizer) processImageTag(imgTag string) string {
	// Check if image already has width and height attributes
	hasWidth := strings.Contains(imgTag, "width=")
	hasHeight := strings.Contains(imgTag, "height=")

	if hasWidth && hasHeight {
		// Image already has dimensions, just ensure aspect-ratio is set
		return ls.addAspectRatioToImage(imgTag)
	}

	// Extract src attribute to determine dimensions
	srcRegex := regexp.MustCompile(`src=["']([^"']*?)["']`)
	srcMatch := srcRegex.FindStringSubmatch(imgTag)
	
	var dimensions ImageDimensions
	if len(srcMatch) > 1 {
		// In a real implementation, you'd analyze the image to get actual dimensions
		// For now, use defaults or extract from filename patterns
		dimensions = ls.inferImageDimensions(srcMatch[1])
	} else {
		dimensions = ls.imageProcessor.defaultDimensions
	}

	// Add width and height attributes
	optimized := imgTag
	if !hasWidth {
		optimized = strings.Replace(optimized, "<img", 
			fmt.Sprintf(`<img width="%d"`, dimensions.Width), 1)
	}
	if !hasHeight {
		optimized = strings.Replace(optimized, "width=", 
			fmt.Sprintf(`height="%d" width=`, dimensions.Height), 1)
	}

	// Add aspect-ratio CSS property
	optimized = ls.addAspectRatioToImage(optimized)

	// Add loading="lazy" for non-critical images
	if !strings.Contains(optimized, "loading=") && !ls.isAboveFoldImage(imgTag) {
		optimized = strings.Replace(optimized, "<img", `<img loading="lazy"`, 1)
	}

	// Add decoding="async" for better performance
	if !strings.Contains(optimized, "decoding=") {
		optimized = strings.Replace(optimized, "<img", `<img decoding="async"`, 1)
	}

	ls.performanceMetrics.ElementsProcessed["img"]++
	ls.performanceMetrics.OptimizationTypes["image_dimensions"]++

	return optimized
}

// inferImageDimensions attempts to determine image dimensions from the source
func (ls *LayoutStabilizer) inferImageDimensions(src string) ImageDimensions {
	// Look for dimension patterns in filename (e.g., image_800x600.jpg)
	dimensionRegex := regexp.MustCompile(`(\d+)x(\d+)`)
	if match := dimensionRegex.FindStringSubmatch(src); len(match) >= 3 {
		if width, err := strconv.Atoi(match[1]); err == nil {
			if height, err := strconv.Atoi(match[2]); err == nil {
				return ImageDimensions{
					Width:  width,
					Height: height,
					Unit:   "px",
				}
			}
		}
	}

	// Check for common image size patterns
	if strings.Contains(src, "thumbnail") || strings.Contains(src, "thumb") {
		return ImageDimensions{Width: 150, Height: 150, Unit: "px"}
	}
	if strings.Contains(src, "avatar") {
		return ImageDimensions{Width: 64, Height: 64, Unit: "px"}
	}
	if strings.Contains(src, "hero") || strings.Contains(src, "banner") {
		return ImageDimensions{Width: 1200, Height: 600, Unit: "px"}
	}

	// Return default dimensions
	return ls.imageProcessor.defaultDimensions
}

// addAspectRatioToImage adds CSS aspect-ratio property to maintain layout stability
func (ls *LayoutStabilizer) addAspectRatioToImage(imgTag string) string {
	// Extract width and height for aspect ratio calculation
	widthRegex := regexp.MustCompile(`width=["'](\d+)["']`)
	heightRegex := regexp.MustCompile(`height=["'](\d+)["']`)

	widthMatch := widthRegex.FindStringSubmatch(imgTag)
	heightMatch := heightRegex.FindStringSubmatch(imgTag)

	if len(widthMatch) > 1 && len(heightMatch) > 1 {
		width, _ := strconv.ParseFloat(widthMatch[1], 64)
		height, _ := strconv.ParseFloat(heightMatch[1], 64)
		
		if width > 0 && height > 0 {
			aspectRatio := width / height
			
			// Add or update style attribute with aspect-ratio
			styleRegex := regexp.MustCompile(`style=["']([^"']*?)["']`)
			if styleMatch := styleRegex.FindStringSubmatch(imgTag); len(styleMatch) > 1 {
				// Update existing style
				newStyle := fmt.Sprintf(`style="%s; aspect-ratio: %.3f;"`, styleMatch[1], aspectRatio)
				return styleRegex.ReplaceAllString(imgTag, newStyle)
			} else {
				// Add new style attribute
				return strings.Replace(imgTag, "<img", 
					fmt.Sprintf(`<img style="aspect-ratio: %.3f;"`, aspectRatio), 1)
			}
		}
	}

	return imgTag
}

// isAboveFoldImage determines if an image is likely above the fold
func (ls *LayoutStabilizer) isAboveFoldImage(imgTag string) bool {
	// Simple heuristics for above-fold detection
	aboveFoldIndicators := []string{
		"hero", "banner", "logo", "header", "nav",
		"class=\"hero\"", "class=\"banner\"", "class=\"logo\"",
		"id=\"hero\"", "id=\"banner\"", "id=\"logo\"",
	}

	for _, indicator := range aboveFoldIndicators {
		if strings.Contains(imgTag, indicator) {
			return true
		}
	}

	return false
}

// optimizeFonts optimizes font loading to prevent layout shifts
func (ls *LayoutStabilizer) optimizeFonts(html string) (string, error) {
	optimized := html

	// Add font-display: swap to existing font CSS
	optimized = ls.addFontDisplay(optimized)

	// Optimize web font loading
	optimized = ls.optimizeWebFonts(optimized)

	// Add font preloading hints
	optimized = ls.addFontPreloading(optimized)

	ls.performanceMetrics.OptimizationTypes["font_optimization"]++

	return optimized, nil
}

// addFontDisplay adds font-display: swap to prevent FOIT
func (ls *LayoutStabilizer) addFontDisplay(html string) string {
	// Find CSS font-face declarations and add font-display
	fontFaceRegex := regexp.MustCompile(`@font-face\s*{([^}]*?)}`)
	
	return fontFaceRegex.ReplaceAllStringFunc(html, func(match string) string {
		if !strings.Contains(match, "font-display") {
			// Add font-display: swap before the closing brace
			return strings.Replace(match, "}", "  font-display: swap;\n}", 1)
		}
		return match
	})
}

// optimizeWebFonts optimizes web font declarations
func (ls *LayoutStabilizer) optimizeWebFonts(html string) string {
	// Find Google Fonts links and add font-display parameter
	googleFontsRegex := regexp.MustCompile(`<link[^>]*?href=["']https://fonts\.googleapis\.com/css[^"']*?["'][^>]*?>`)
	
	return googleFontsRegex.ReplaceAllStringFunc(html, func(match string) string {
		if !strings.Contains(match, "display=swap") {
			// Add display=swap parameter to Google Fonts URL
			hrefRegex := regexp.MustCompile(`href=["'](https://fonts\.googleapis\.com/css[^"']*?)["']`)
			return hrefRegex.ReplaceAllStringFunc(match, func(href string) string {
				url := strings.Trim(strings.Split(href, "=")[1], `"'`)
				if strings.Contains(url, "?") {
					url += "&display=swap"
				} else {
					url += "?display=swap"
				}
				return fmt.Sprintf(`href="%s"`, url)
			})
		}
		return match
	})
}

// addFontPreloading adds font preloading for critical fonts
func (ls *LayoutStabilizer) addFontPreloading(html string) string {
	// This is a simplified implementation
	// In practice, you'd identify critical fonts from CSS analysis
	preloadFonts := []string{
		"/static/fonts/inter-regular.woff2",
		"/static/fonts/inter-bold.woff2",
	}

	headEndRegex := regexp.MustCompile(`</head>`)
	
	return headEndRegex.ReplaceAllStringFunc(html, func(match string) string {
		var preloads strings.Builder
		for _, fontPath := range preloadFonts {
			preloads.WriteString(fmt.Sprintf(
				`    <link rel="preload" href="%s" as="font" type="font/woff2" crossorigin="anonymous">%s`,
				fontPath, "\n"))
		}
		return preloads.String() + match
	})
}

// addContainerSpecs adds container specifications for layout stability
func (ls *LayoutStabilizer) addContainerSpecs(html string) string {
	optimized := html

	// Add specifications for common layout containers
	containerSpecs := []ContainerSpec{
		{
			Selector:    ".container",
			MinHeight:   "100px",
			AspectRatio: "",
		},
		{
			Selector:    ".card",
			MinHeight:   "200px",
			AspectRatio: "",
		},
		{
			Selector:    ".hero",
			MinHeight:   "400px",
			AspectRatio: "16:9",
		},
		{
			Selector:    ".recipe-card",
			MinHeight:   "300px",
			AspectRatio: "4:3",
		},
	}

	// Generate CSS for container specifications
	var containerCSS strings.Builder
	containerCSS.WriteString("<style>\n/* Layout Stability CSS */\n")
	
	for _, spec := range containerSpecs {
		containerCSS.WriteString(fmt.Sprintf("%s {\n", spec.Selector))
		
		if spec.MinHeight != "" {
			containerCSS.WriteString(fmt.Sprintf("  min-height: %s;\n", spec.MinHeight))
		}
		
		if spec.AspectRatio != "" {
			if ratio, exists := AspectRatioMap[spec.AspectRatio]; exists {
				containerCSS.WriteString(fmt.Sprintf("  aspect-ratio: %.3f;\n", ratio))
			}
		}
		
		// Add contain for layout optimization
		containerCSS.WriteString("  contain: layout style;\n")
		
		containerCSS.WriteString("}\n\n")
	}
	
	containerCSS.WriteString("</style>\n")

	// Insert CSS before closing head tag
	headEndRegex := regexp.MustCompile(`</head>`)
	optimized = headEndRegex.ReplaceAllString(optimized, containerCSS.String()+"</head>")

	ls.performanceMetrics.OptimizationTypes["container_specs"]++

	return optimized
}

// addLayoutHints adds CSS layout hints for better performance
func (ls *LayoutStabilizer) addLayoutHints(html string) string {
	layoutHintsCSS := `<style>
/* Layout Performance Hints */
.recipe-card, .card, .content-block {
  contain: layout style paint;
  transform: translateZ(0); /* Create layer for better performance */
}

.hero-image, .featured-image {
  object-fit: cover;
  object-position: center;
}

.dynamic-content {
  min-height: 100px; /* Reserve space for dynamic content */
}

.skeleton-loader {
  background: linear-gradient(90deg, #f0f0f0 25%, #e0e0e0 50%, #f0f0f0 75%);
  background-size: 200% 100%;
  animation: loading 1.5s infinite;
}

@keyframes loading {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* Prevent layout shift during font loading */
.text-content {
  font-display: swap;
  size-adjust: 100%;
}

/* HTMX loading states */
.htmx-request {
  min-height: inherit; /* Maintain height during requests */
}

.htmx-settling {
  transition: none; /* Disable transitions during settling */
}
</style>
`

	// Insert CSS before closing head tag
	headEndRegex := regexp.MustCompile(`</head>`)
	optimized := headEndRegex.ReplaceAllString(html, layoutHintsCSS+"</head>")

	ls.performanceMetrics.OptimizationTypes["layout_hints"]++

	return optimized
}

// GenerateSkeletonHTML generates skeleton loading HTML for dynamic content
func (ls *LayoutStabilizer) GenerateSkeletonHTML(contentType string, dimensions ImageDimensions) string {
	switch contentType {
	case "recipe-card":
		return ls.generateRecipeCardSkeleton(dimensions)
	case "recipe-list":
		return ls.generateRecipeListSkeleton(dimensions)
	case "text-block":
		return ls.generateTextBlockSkeleton(dimensions)
	default:
		return ls.generateGenericSkeleton(dimensions)
	}
}

// generateRecipeCardSkeleton generates skeleton for recipe cards
func (ls *LayoutStabilizer) generateRecipeCardSkeleton(dimensions ImageDimensions) string {
	return fmt.Sprintf(`
<div class="recipe-card skeleton-loader" style="width: %dpx; height: %dpx;">
	<div class="skeleton-image" style="width: 100%%; height: 60%%; background: #e0e0e0;"></div>
	<div class="skeleton-content" style="padding: 1rem;">
		<div class="skeleton-title" style="width: 80%%; height: 1.5rem; background: #f0f0f0; margin-bottom: 0.5rem;"></div>
		<div class="skeleton-description" style="width: 100%%; height: 3rem; background: #f0f0f0;"></div>
	</div>
</div>`, dimensions.Width, dimensions.Height)
}

// generateRecipeListSkeleton generates skeleton for recipe lists
func (ls *LayoutStabilizer) generateRecipeListSkeleton(dimensions ImageDimensions) string {
	var items strings.Builder
	itemCount := 6 // Default number of skeleton items
	
	for i := 0; i < itemCount; i++ {
		items.WriteString(fmt.Sprintf(`
		<div class="recipe-item skeleton-loader" style="height: %dpx; margin-bottom: 1rem; display: flex; align-items: center;">
			<div class="skeleton-image" style="width: 80px; height: 80px; background: #e0e0e0; margin-right: 1rem;"></div>
			<div class="skeleton-content" style="flex: 1;">
				<div class="skeleton-title" style="width: 70%%; height: 1.2rem; background: #f0f0f0; margin-bottom: 0.5rem;"></div>
				<div class="skeleton-subtitle" style="width: 50%%; height: 1rem; background: #f0f0f0;"></div>
			</div>
		</div>`, dimensions.Height/itemCount))
	}
	
	return fmt.Sprintf(`<div class="recipe-list" style="min-height: %dpx;">%s</div>`, 
		dimensions.Height, items.String())
}

// generateTextBlockSkeleton generates skeleton for text content
func (ls *LayoutStabilizer) generateTextBlockSkeleton(dimensions ImageDimensions) string {
	return fmt.Sprintf(`
<div class="text-block skeleton-loader" style="width: %dpx; height: %dpx;">
	<div class="skeleton-line" style="width: 90%%; height: 1rem; background: #f0f0f0; margin-bottom: 0.5rem;"></div>
	<div class="skeleton-line" style="width: 80%%; height: 1rem; background: #f0f0f0; margin-bottom: 0.5rem;"></div>
	<div class="skeleton-line" style="width: 85%%; height: 1rem; background: #f0f0f0; margin-bottom: 0.5rem;"></div>
	<div class="skeleton-line" style="width: 75%%; height: 1rem; background: #f0f0f0;"></div>
</div>`, dimensions.Width, dimensions.Height)
}

// generateGenericSkeleton generates a generic skeleton
func (ls *LayoutStabilizer) generateGenericSkeleton(dimensions ImageDimensions) string {
	return fmt.Sprintf(`
<div class="skeleton-loader" style="width: %dpx; height: %dpx; background: #f0f0f0;">
</div>`, dimensions.Width, dimensions.Height)
}

// HTMXOptimization provides HTMX-specific layout stability optimizations
func (ls *LayoutStabilizer) HTMXOptimization(html string) string {
	optimized := html

	// Add layout preservation attributes to HTMX elements
	htmxRegex := regexp.MustCompile(`<([^>]*?\bhx-[^>]*?)>`)
	
	optimized = htmxRegex.ReplaceAllStringFunc(optimized, func(match string) string {
		// Add layout stability attributes
		if !strings.Contains(match, "hx-preserve") {
			match = strings.Replace(match, "<", `<div hx-preserve="layout-dimensions">`, 1)
		}
		
		// Add loading placeholder
		if strings.Contains(match, "hx-get") || strings.Contains(match, "hx-post") {
			if !strings.Contains(match, "hx-indicator") {
				match = strings.Replace(match, ">", ` hx-indicator=".skeleton-loader">`, 1)
			}
		}
		
		return match
	})

	return optimized
}

// updateMetrics updates performance metrics
func (ls *LayoutStabilizer) updateMetrics(original, optimized string) {
	ls.performanceMetrics.TotalElements++
	
	// Count stabilized elements (simplified approach)
	originalImageCount := strings.Count(original, "<img")
	optimizedImageCount := strings.Count(optimized, `width="`)
	
	if optimizedImageCount > originalImageCount {
		ls.performanceMetrics.StabilizedElements++
	}
	
	ls.performanceMetrics.LastOptimization = time.Now()
}

// ValidateCLS validates that the optimized content meets CLS requirements
func (ls *LayoutStabilizer) ValidateCLS(html string) (*CLSValidationReport, error) {
	report := &CLSValidationReport{
		Timestamp:          time.Now(),
		TotalElements:      0,
		ElementsWithDims:   0,
		ElementsWithAspect: 0,
		Violations:         []CLSViolation{},
		Score:              0.0,
		Status:             "unknown",
	}

	// Count images without dimensions
	imgRegex := regexp.MustCompile(`<img([^>]*?)>`)
	images := imgRegex.FindAllString(html, -1)
	
	for _, img := range images {
		report.TotalElements++
		
		hasWidth := strings.Contains(img, "width=")
		hasHeight := strings.Contains(img, "height=")
		hasAspectRatio := strings.Contains(img, "aspect-ratio")
		
		if hasWidth && hasHeight {
			report.ElementsWithDims++
		} else {
			violation := CLSViolation{
				ElementType: "img",
				Issue:       "Missing width/height attributes",
				Element:     img,
				Severity:    "high",
			}
			report.Violations = append(report.Violations, violation)
		}
		
		if hasAspectRatio {
			report.ElementsWithAspect++
		}
	}

	// Calculate estimated CLS score
	if report.TotalElements > 0 {
		// Simplified scoring: elements without dimensions contribute to CLS
		elementsWithoutDims := report.TotalElements - report.ElementsWithDims
		report.Score = float64(elementsWithoutDims) * 0.05 // Rough estimate
	}

	// Determine status
	if report.Score <= ls.config.MaxCLSScore {
		report.Status = "good"
	} else if report.Score <= 0.25 {
		report.Status = "needs-improvement"
	} else {
		report.Status = "poor"
	}

	return report, nil
}

// CLSValidationReport represents a CLS validation report
type CLSValidationReport struct {
	Timestamp          time.Time
	TotalElements      int
	ElementsWithDims   int
	ElementsWithAspect int
	Violations         []CLSViolation
	Score              float64
	Status             string
	Recommendations    []string
}

// CLSViolation represents a CLS violation
type CLSViolation struct {
	ElementType string
	Issue       string
	Element     string
	Severity    string
	Fix         string
}

// GetMetrics returns current layout stability metrics
func (ls *LayoutStabilizer) GetMetrics() LayoutMetrics {
	return ls.performanceMetrics
}

// GenerateReport generates a layout stability report
func (ls *LayoutStabilizer) GenerateReport() string {
	metrics := ls.performanceMetrics
	
	return fmt.Sprintf(`=== Layout Stability Report ===
Last Optimization: %s
Total Elements: %d
Stabilized Elements: %d
CLS Violations: %d
Average CLS Score: %.3f

=== Elements Processed ===
Images: %d
Containers: %d
Fonts: %d

=== Optimization Types Applied ===
Image Dimensions: %d
Font Optimization: %d
Container Specs: %d
Layout Hints: %d

=== Performance Impact ===
Estimated CLS Improvement: %.3f
Layout Stability Score: %.1f%%
`,
		metrics.LastOptimization.Format(time.RFC3339),
		metrics.TotalElements,
		metrics.StabilizedElements,
		metrics.CLSViolations,
		metrics.AverageCLSScore,
		metrics.ElementsProcessed["img"],
		metrics.ElementsProcessed["container"],
		metrics.ElementsProcessed["font"],
		metrics.OptimizationTypes["image_dimensions"],
		metrics.OptimizationTypes["font_optimization"],
		metrics.OptimizationTypes["container_specs"],
		metrics.OptimizationTypes["layout_hints"],
		ls.config.MaxCLSScore-metrics.AverageCLSScore,
		float64(metrics.StabilizedElements)/float64(metrics.TotalElements)*100,
	)
}

// TemplateFunction returns a template function for layout stability optimization
func (ls *LayoutStabilizer) TemplateFunction() template.FuncMap {
	return template.FuncMap{
		"stabilizeLayout": func(content string) template.HTML {
			optimized, err := ls.StabilizeHTML(content)
			if err != nil {
				// Return original content if optimization fails
				return template.HTML(content)
			}
			return template.HTML(optimized)
		},
		"skeletonLoader": func(contentType string, width, height int) template.HTML {
			dimensions := ImageDimensions{Width: width, Height: height, Unit: "px"}
			skeleton := ls.GenerateSkeletonHTML(contentType, dimensions)
			return template.HTML(skeleton)
		},
		"aspectRatio": func(width, height int) string {
			if height == 0 {
				return "auto"
			}
			ratio := float64(width) / float64(height)
			return fmt.Sprintf("%.3f", ratio)
		},
	}
}