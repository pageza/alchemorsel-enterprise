// Package performance provides tests for Core Web Vitals optimization
package performance

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/cache"
)

// TestCoreWebVitalsTargets validates that optimization meets Google's Core Web Vitals targets
func TestCoreWebVitalsTargets(t *testing.T) {
	// Initialize orchestrator with target thresholds
	config := DefaultCWVOrchestratorConfig()
	config.TargetLCP = 2500 * time.Millisecond // 2.5s
	config.TargetCLS = 0.1                     // 0.1
	config.TargetINP = 200 * time.Millisecond  // 200ms
	
	// Mock cache client for testing
	cacheClient := &cache.RedisClient{} // Assuming this exists
	
	orchestrator, err := NewCoreWebVitalsOrchestrator(config, cacheClient)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	
	// Test cases with sample HTML content
	testCases := []struct {
		name     string
		html     string
		expected CoreWebVitalsValidation
	}{
		{
			name: "Recipe page with hero image",
			html: `<!DOCTYPE html>
<html>
<head>
	<title>Delicious Chocolate Cake Recipe</title>
	<link rel="stylesheet" href="/static/css/styles.css">
</head>
<body>
	<div class="hero">
		<img src="/static/images/chocolate-cake-hero.jpg" alt="Chocolate Cake">
		<h1>Delicious Chocolate Cake</h1>
	</div>
	<div class="recipe-content">
		<div class="ingredients">
			<h2>Ingredients</h2>
			<ul>
				<li>2 cups flour</li>
				<li>1.5 cups sugar</li>
				<li>3/4 cup cocoa powder</li>
			</ul>
		</div>
		<div class="instructions">
			<h2>Instructions</h2>
			<ol>
				<li>Preheat oven to 350°F</li>
				<li>Mix dry ingredients</li>
				<li>Add wet ingredients</li>
			</ol>
		</div>
	</div>
	<script src="/static/js/app.js"></script>
</body>
</html>`,
			expected: CoreWebVitalsValidation{
				LCPValid: true,
				CLSValid: true,
				INPValid: true,
			},
		},
		{
			name: "Recipe list page with multiple images",
			html: `<!DOCTYPE html>
<html>
<head>
	<title>Recipe Collection</title>
	<link rel="stylesheet" href="/static/css/styles.css">
</head>
<body>
	<header>
		<h1>Recipe Collection</h1>
		<input type="search" placeholder="Search recipes..." data-search>
	</header>
	<main class="recipe-list">
		<div class="recipe-card">
			<img src="/static/images/pasta.jpg" alt="Pasta Recipe">
			<h3>Creamy Pasta</h3>
			<p>Delicious creamy pasta with herbs</p>
		</div>
		<div class="recipe-card">
			<img src="/static/images/salad.jpg" alt="Salad Recipe">
			<h3>Fresh Garden Salad</h3>
			<p>Healthy salad with seasonal vegetables</p>
		</div>
		<div class="recipe-card">
			<img src="/static/images/soup.jpg" alt="Soup Recipe">
			<h3>Tomato Soup</h3>
			<p>Warming tomato soup for cold days</p>
		</div>
	</main>
	<script src="/static/js/search.js"></script>
</body>
</html>`,
			expected: CoreWebVitalsValidation{
				LCPValid: true,
				CLSValid: true,
				INPValid: true,
			},
		},
		{
			name: "HTMX-enabled interactive page",
			html: `<!DOCTYPE html>
<html>
<head>
	<title>Interactive Recipe Builder</title>
	<link rel="stylesheet" href="/static/css/styles.css">
	<script src="https://unpkg.com/htmx.org@1.9.6"></script>
</head>
<body>
	<div class="recipe-builder">
		<h1>Build Your Recipe</h1>
		<form hx-post="/api/recipes/create" hx-target="#recipe-preview">
			<div class="ingredient-selector">
				<input type="text" name="ingredient" 
				       hx-get="/api/ingredients/search" 
				       hx-trigger="input delay:300ms"
				       hx-target="#ingredient-suggestions">
				<div id="ingredient-suggestions"></div>
			</div>
			<button type="submit">Create Recipe</button>
		</form>
		<div id="recipe-preview" class="dynamic-content">
			<!-- Dynamic content will load here -->
		</div>
	</div>
</body>
</html>`,
			expected: CoreWebVitalsValidation{
				LCPValid: true,
				CLSValid: true,
				INPValid: true,
			},
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Apply optimizations
			optimized, err := orchestrator.OptimizeHTML(tc.html)
			if err != nil {
				t.Errorf("Optimization failed: %v", err)
				return
			}
			
			// Validate optimizations
			validation := validateOptimizedHTML(optimized)
			
			// Check LCP optimization
			if !validation.LCPValid && tc.expected.LCPValid {
				t.Errorf("LCP optimization failed - expected valid, got invalid")
			}
			
			// Check CLS optimization
			if !validation.CLSValid && tc.expected.CLSValid {
				t.Errorf("CLS optimization failed - expected valid, got invalid")
			}
			
			// Check INP optimization
			if !validation.INPValid && tc.expected.INPValid {
				t.Errorf("INP optimization failed - expected valid, got invalid")
			}
			
			// Verify specific optimizations were applied
			t.Run("LCP optimizations", func(t *testing.T) {
				validateLCPOptimizations(t, tc.html, optimized)
			})
			
			t.Run("CLS optimizations", func(t *testing.T) {
				validateCLSOptimizations(t, tc.html, optimized)
			})
			
			t.Run("INP optimizations", func(t *testing.T) {
				validateINPOptimizations(t, tc.html, optimized)
			})
		})
	}
}

// CoreWebVitalsValidation represents validation results
type CoreWebVitalsValidation struct {
	LCPValid bool
	CLSValid bool
	INPValid bool
}

// validateOptimizedHTML validates that HTML has been properly optimized
func validateOptimizedHTML(html string) CoreWebVitalsValidation {
	return CoreWebVitalsValidation{
		LCPValid: validateLCPOptimization(html),
		CLSValid: validateCLSOptimization(html),
		INPValid: validateINPOptimization(html),
	}
}

// validateLCPOptimization checks for LCP optimizations
func validateLCPOptimization(html string) bool {
	// Check for critical CSS inlining
	hasCriticalCSS := strings.Contains(html, `<style data-critical="true">`)
	
	// Check for resource preloading
	hasPreload := strings.Contains(html, `rel="preload"`)
	
	// Check for fetch priority on images
	hasFetchPriority := strings.Contains(html, `fetchpriority="high"`)
	
	// Check for hero image optimization
	hasOptimizedImages := strings.Contains(html, `loading="eager"`) || 
						 strings.Contains(html, `decoding="async"`)
	
	return hasCriticalCSS || hasPreload || hasFetchPriority || hasOptimizedImages
}

// validateCLSOptimization checks for CLS optimizations
func validateCLSOptimization(html string) bool {
	// Check for image dimensions
	hasImageDimensions := strings.Contains(html, `width="`) && strings.Contains(html, `height="`)
	
	// Check for aspect ratio CSS
	hasAspectRatio := strings.Contains(html, `aspect-ratio:`)
	
	// Check for layout stability CSS
	hasLayoutCSS := strings.Contains(html, `Layout Stability CSS`) ||
					strings.Contains(html, `contain: layout`)
	
	// Check for font-display: swap
	hasFontDisplay := strings.Contains(html, `font-display: swap`)
	
	return hasImageDimensions || hasAspectRatio || hasLayoutCSS || hasFontDisplay
}

// validateINPOptimization checks for INP optimizations
func validateINPOptimization(html string) bool {
	// Check for HTMX debouncing
	hasHTMXDebouncing := strings.Contains(html, `delay:`) && strings.Contains(html, `hx-trigger`)
	
	// Check for task scheduling script
	hasTaskScheduling := strings.Contains(html, `TaskScheduler`) ||
						strings.Contains(html, `window.taskScheduler`)
	
	// Check for touch optimizations
	hasTouchOptimizations := strings.Contains(html, `touch-action:`) ||
							strings.Contains(html, `fast-tap`)
	
	// Check for progressive enhancement
	hasProgressiveEnhancement := strings.Contains(html, `data-progressive-htmx`) ||
								strings.Contains(html, `ProgressiveEnhancer`)
	
	return hasHTMXDebouncing || hasTaskScheduling || hasTouchOptimizations || hasProgressiveEnhancement
}

// validateLCPOptimizations validates specific LCP optimizations
func validateLCPOptimizations(t *testing.T, original, optimized string) {
	// Test 1: Critical CSS should be inlined
	if !strings.Contains(optimized, `<style data-critical="true">`) {
		t.Error("Critical CSS was not inlined")
	}
	
	// Test 2: Hero images should have eager loading
	if strings.Contains(original, `class="hero"`) && strings.Contains(original, `<img`) {
		if !strings.Contains(optimized, `loading="eager"`) {
			t.Error("Hero image should have eager loading")
		}
	}
	
	// Test 3: Critical resources should have preload hints
	expectedPreloads := []string{
		`rel="preload"`,
		`as="font"`,
		`crossorigin="anonymous"`,
	}
	
	for _, preload := range expectedPreloads {
		if !strings.Contains(optimized, preload) {
			t.Errorf("Missing preload optimization: %s", preload)
		}
	}
	
	// Test 4: Resource hints should be added
	expectedHints := []string{
		`rel="dns-prefetch"`,
		`rel="preconnect"`,
	}
	
	hasAnyHint := false
	for _, hint := range expectedHints {
		if strings.Contains(optimized, hint) {
			hasAnyHint = true
			break
		}
	}
	
	if !hasAnyHint {
		t.Error("No resource hints were added")
	}
}

// validateCLSOptimizations validates specific CLS optimizations
func validateCLSOptimizations(t *testing.T, original, optimized string) {
	// Test 1: Images should have dimensions
	if strings.Contains(original, `<img`) {
		hasWidth := strings.Contains(optimized, `width="`)
		hasHeight := strings.Contains(optimized, `height="`)
		
		if !hasWidth || !hasHeight {
			t.Error("Images should have explicit width and height attributes")
		}
	}
	
	// Test 2: Layout stability CSS should be added
	expectedCSS := []string{
		`Layout Stability CSS`,
		`contain: layout`,
		`aspect-ratio:`,
	}
	
	hasAnyCSS := false
	for _, css := range expectedCSS {
		if strings.Contains(optimized, css) {
			hasAnyCSS = true
			break
		}
	}
	
	if !hasAnyCSS {
		t.Error("Layout stability CSS was not added")
	}
	
	// Test 3: Font optimizations should be applied
	if strings.Contains(original, `@font-face`) || strings.Contains(original, `fonts.googleapis.com`) {
		if !strings.Contains(optimized, `font-display: swap`) {
			t.Error("Font-display: swap should be added for CLS prevention")
		}
	}
	
	// Test 4: Container specifications should be added
	if strings.Contains(original, `class="hero"`) || strings.Contains(original, `class="recipe-card"`) {
		if !strings.Contains(optimized, `min-height:`) {
			t.Error("Container min-height should be specified for layout stability")
		}
	}
}

// validateINPOptimizations validates specific INP optimizations
func validateINPOptimizations(t *testing.T, original, optimized string) {
	// Test 1: HTMX triggers should have debouncing
	if strings.Contains(original, `hx-trigger="input"`) {
		if !strings.Contains(optimized, `delay:`) {
			t.Error("HTMX input triggers should have debouncing")
		}
	}
	
	// Test 2: Task scheduling script should be added
	expectedScripts := []string{
		`TaskScheduler`,
		`window.taskScheduler`,
		`processTasks`,
	}
	
	hasTaskScheduling := false
	for _, script := range expectedScripts {
		if strings.Contains(optimized, script) {
			hasTaskScheduling = true
			break
		}
	}
	
	if !hasTaskScheduling {
		t.Error("Task scheduling script should be added for INP optimization")
	}
	
	// Test 3: Touch optimizations should be applied
	expectedTouchOpts := []string{
		`touch-action: manipulation`,
		`fast-tap`,
		`no-touch-delay`,
	}
	
	hasTouchOpts := false
	for _, opt := range expectedTouchOpts {
		if strings.Contains(optimized, opt) {
			hasTouchOpts = true
			break
		}
	}
	
	if !hasTouchOpts {
		t.Error("Touch optimizations should be applied")
	}
	
	// Test 4: Progressive enhancement should be available
	if strings.Contains(original, `hx-`) {
		expectedProgressive := []string{
			`ProgressiveEnhancer`,
			`progressive-htmx`,
			`lazy-htmx`,
		}
		
		hasProgressive := false
		for _, prog := range expectedProgressive {
			if strings.Contains(optimized, prog) {
				hasProgressive = true
				break
			}
		}
		
		if !hasProgressive {
			t.Error("Progressive enhancement should be available for HTMX content")
		}
	}
}

// TestBundleOptimization tests that 14KB bundle optimization works
func TestBundleOptimization(t *testing.T) {
	config := DefaultCWVOrchestratorConfig()
	config.EnableBundleOptimization = true
	config.MaxBundleSize = 14 * 1024 // 14KB
	
	cacheClient := &cache.RedisClient{}
	orchestrator, err := NewCoreWebVitalsOrchestrator(config, cacheClient)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	
	// Test HTML with large CSS and JS that should be optimized
	html := `<!DOCTYPE html>
<html>
<head>
	<title>Bundle Optimization Test</title>
	<style>
	/* Large CSS block for testing */
	.container { width: 100%; max-width: 1200px; margin: 0 auto; padding: 20px; }
	.hero { height: 400px; background: linear-gradient(to right, #ff7e5f, #feb47b); }
	.recipe-card { border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin: 16px 0; }
	.ingredient-list { list-style: none; padding: 0; }
	.instruction-step { margin: 12px 0; line-height: 1.6; }
	/* More CSS content to test compression... */
	.btn { background: #007bff; color: white; border: none; padding: 12px 24px; border-radius: 4px; }
	.btn:hover { background: #0056b3; }
	</style>
</head>
<body>
	<div class="container">
		<div class="hero">
			<h1>Test Recipe</h1>
		</div>
	</div>
	<script>
	// Large JavaScript block for testing
	function initializeApp() {
		console.log('App initialized');
		setupEventListeners();
		loadInitialData();
	}
	function setupEventListeners() {
		document.addEventListener('click', handleClick);
		document.addEventListener('input', handleInput);
	}
	function loadInitialData() {
		fetch('/api/recipes').then(response => response.json()).then(data => {
			renderRecipes(data);
		});
	}
	// More JS content to test compression...
	initializeApp();
	</script>
</body>
</html>`
	
	optimized, err := orchestrator.OptimizeHTML(html)
	if err != nil {
		t.Fatalf("Bundle optimization failed: %v", err)
	}
	
	// Verify bundle optimization was applied
	if !strings.Contains(optimized, `<style data-critical="true">`) {
		t.Error("Critical CSS bundle was not created")
	}
	
	if !strings.Contains(optimized, `<script data-critical="true">`) {
		t.Error("Critical JS bundle was not created")
	}
	
	// Verify the optimized content is smaller
	if len(optimized) >= len(html) {
		t.Error("Bundle optimization should reduce content size")
	}
	
	// Verify 14KB compliance (this would need more sophisticated measurement in practice)
	inlineCSS := extractInlineContent(optimized, `<style data-critical="true">`, `</style>`)
	inlineJS := extractInlineContent(optimized, `<script data-critical="true">`, `</script>`)
	
	bundleSize := len(inlineCSS) + len(inlineJS)
	if bundleSize > config.MaxBundleSize {
		t.Errorf("Bundle size %d exceeds limit %d", bundleSize, config.MaxBundleSize)
	}
}

// TestPerformanceTargetsMet tests that performance targets are met
func TestPerformanceTargetsMet(t *testing.T) {
	config := DefaultCWVOrchestratorConfig()
	cacheClient := &cache.RedisClient{}
	
	orchestrator, err := NewCoreWebVitalsOrchestrator(config, cacheClient)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	
	// Simulate measurements that should meet targets
	measurements := []CWVMeasurement{
		{
			Timestamp: time.Now(),
			URL:       "/recipes/chocolate-cake",
			Metrics: map[string]float64{
				"LCP": 2200.0, // 2.2s - under 2.5s target
				"CLS": 0.08,   // 0.08 - under 0.1 target
				"INP": 180.0,  // 180ms - under 200ms target
			},
		},
		{
			Timestamp: time.Now(),
			URL:       "/recipes/pasta",
			Metrics: map[string]float64{
				"LCP": 2400.0, // 2.4s - under 2.5s target
				"CLS": 0.09,   // 0.09 - under 0.1 target
				"INP": 190.0,  // 190ms - under 200ms target
			},
		},
		{
			Timestamp: time.Now(),
			URL:       "/recipes",
			Metrics: map[string]float64{
				"LCP": 2100.0, // 2.1s - under 2.5s target
				"CLS": 0.06,   // 0.06 - under 0.1 target
				"INP": 150.0,  // 150ms - under 200ms target
			},
		},
	}
	
	// Record measurements
	for _, measurement := range measurements {
		err := orchestrator.RecordMeasurement(measurement)
		if err != nil {
			t.Errorf("Failed to record measurement: %v", err)
		}
	}
	
	// Get performance score
	performance := orchestrator.GetCurrentPerformance()
	
	// Verify targets are met
	if performance.Current.LCP > float64(config.TargetLCP.Milliseconds()) {
		t.Errorf("LCP target not met: %.0fms > %.0fms", 
			performance.Current.LCP, float64(config.TargetLCP.Milliseconds()))
	}
	
	if performance.Current.CLS > config.TargetCLS {
		t.Errorf("CLS target not met: %.3f > %.3f", 
			performance.Current.CLS, config.TargetCLS)
	}
	
	if performance.Current.INP > float64(config.TargetINP.Milliseconds()) {
		t.Errorf("INP target not met: %.0fms > %.0fms", 
			performance.Current.INP, float64(config.TargetINP.Milliseconds()))
	}
	
	// Verify overall score
	if !performance.Score.Passing {
		t.Error("Overall Core Web Vitals score should be passing")
	}
	
	if performance.Score.OverallScore < 80.0 {
		t.Errorf("Overall score too low: %.1f < 80.0", performance.Score.OverallScore)
	}
	
	if performance.Score.Grade == "F" || performance.Score.Grade == "D" {
		t.Errorf("Performance grade too low: %s", performance.Score.Grade)
	}
}

// TestRealWorldScenarios tests real-world scenarios
func TestRealWorldScenarios(t *testing.T) {
	scenarios := []struct {
		name        string
		description string
		html        string
		deviceType  string
		connection  string
	}{
		{
			name:        "Mobile slow 3G",
			description: "Recipe page on mobile with slow 3G connection",
			deviceType:  "mobile",
			connection:  "3g",
			html: generateMobileRecipeHTML(),
		},
		{
			name:        "Desktop fast WiFi",
			description: "Recipe collection on desktop with fast WiFi",
			deviceType:  "desktop", 
			connection:  "4g",
			html: generateDesktopCollectionHTML(),
		},
		{
			name:        "Tablet medium connection",
			description: "Interactive recipe builder on tablet",
			deviceType:  "tablet",
			connection:  "3g",
			html: generateTabletInteractiveHTML(),
		},
	}
	
	config := DefaultCWVOrchestratorConfig()
	cacheClient := &cache.RedisClient{}
	
	orchestrator, err := NewCoreWebVitalsOrchestrator(config, cacheClient)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Apply optimizations
			optimized, err := orchestrator.OptimizeHTML(scenario.html)
			if err != nil {
				t.Errorf("Optimization failed for %s: %v", scenario.name, err)
				return
			}
			
			// Verify optimizations are appropriate for device/connection
			validation := validateScenarioOptimizations(optimized, scenario.deviceType, scenario.connection)
			
			if !validation.Appropriate {
				t.Errorf("Optimizations not appropriate for %s (%s, %s): %s", 
					scenario.name, scenario.deviceType, scenario.connection, validation.Issues)
			}
		})
	}
}

// ScenarioValidation represents scenario-specific validation
type ScenarioValidation struct {
	Appropriate bool
	Issues      string
}

// Helper functions for testing

func extractInlineContent(html, startTag, endTag string) string {
	startIdx := strings.Index(html, startTag)
	if startIdx == -1 {
		return ""
	}
	
	startIdx += len(startTag)
	endIdx := strings.Index(html[startIdx:], endTag)
	if endIdx == -1 {
		return ""
	}
	
	return html[startIdx : startIdx+endIdx]
}

func generateMobileRecipeHTML() string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Quick Pasta Recipe</title>
	<link rel="stylesheet" href="/static/css/mobile.css">
</head>
<body>
	<header>
		<h1>Quick Pasta Recipe</h1>
	</header>
	<main>
		<img src="/static/images/pasta-mobile.jpg" alt="Pasta dish" class="hero-image">
		<section class="ingredients">
			<h2>Ingredients</h2>
			<ul>
				<li>400g pasta</li>
				<li>2 tbsp olive oil</li>
				<li>3 cloves garlic</li>
			</ul>
		</section>
	</main>
</body>
</html>`
}

func generateDesktopCollectionHTML() string {
	return `<!DOCTYPE html>
<html>
<head>
	<title>Recipe Collection</title>
	<link rel="stylesheet" href="/static/css/desktop.css">
</head>
<body>
	<header>
		<h1>Recipe Collection</h1>
		<nav>
			<a href="/recipes/appetizers">Appetizers</a>
			<a href="/recipes/mains">Main Courses</a>
			<a href="/recipes/desserts">Desserts</a>
		</nav>
	</header>
	<main class="recipe-grid">
		<div class="recipe-card">
			<img src="/static/images/recipe1.jpg" alt="Recipe 1">
			<h3>Delicious Recipe 1</h3>
		</div>
		<div class="recipe-card">
			<img src="/static/images/recipe2.jpg" alt="Recipe 2">
			<h3>Amazing Recipe 2</h3>
		</div>
		<div class="recipe-card">
			<img src="/static/images/recipe3.jpg" alt="Recipe 3">
			<h3>Fantastic Recipe 3</h3>
		</div>
	</main>
</body>
</html>`
}

func generateTabletInteractiveHTML() string {
	return `<!DOCTYPE html>
<html>
<head>
	<title>Interactive Recipe Builder</title>
	<script src="https://unpkg.com/htmx.org@1.9.6"></script>
</head>
<body>
	<div class="recipe-builder">
		<h1>Build Your Recipe</h1>
		<form hx-post="/api/recipes/create">
			<input type="text" hx-get="/api/ingredients/search" hx-trigger="input delay:300ms">
			<button type="submit">Create</button>
		</form>
		<div id="preview" hx-get="/api/recipes/preview"></div>
	</div>
</body>
</html>`
}

func validateScenarioOptimizations(html, deviceType, connection string) ScenarioValidation {
	issues := []string{}
	
	// Mobile-specific validations
	if deviceType == "mobile" {
		if !strings.Contains(html, `viewport`) {
			issues = append(issues, "Missing viewport meta tag for mobile")
		}
		
		if !strings.Contains(html, `loading="lazy"`) && strings.Count(html, `<img`) > 1 {
			issues = append(issues, "Should use lazy loading for non-critical images on mobile")
		}
	}
	
	// Connection-specific validations
	if connection == "3g" || connection == "slow-2g" {
		if !strings.Contains(html, `<style data-critical="true">`) {
			issues = append(issues, "Should inline critical CSS for slow connections")
		}
		
		if !strings.Contains(html, `delay:`) && strings.Contains(html, `hx-trigger`) {
			issues = append(issues, "Should debounce HTMX requests on slow connections")
		}
	}
	
	return ScenarioValidation{
		Appropriate: len(issues) == 0,
		Issues:      strings.Join(issues, "; "),
	}
}