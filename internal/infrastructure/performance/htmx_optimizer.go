// Package performance provides HTMX optimization for progressive enhancement and 14KB compliance
package performance

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// HTMXOptimizer manages HTMX-specific optimizations for first packet delivery
type HTMXOptimizer struct {
	config               HTMXOptimizationConfig
	criticalElements     map[string]HTMXElement
	deferredElements     []HTMXElement
	loadingStrategies    map[string]LoadingStrategy
	performanceMetrics   HTMXPerformanceMetrics
}

// HTMXOptimizationConfig configures HTMX optimization behavior
type HTMXOptimizationConfig struct {
	EnableCriticalPath     bool          // Enable critical path optimization
	DeferNonCritical       bool          // Defer non-critical HTMX elements
	InlineHTMXCore         bool          // Inline HTMX core library
	MaxCriticalElements    int           // Max number of critical HTMX elements
	LazyLoadThreshold      string        // CSS selector for lazy loading trigger
	ProgressiveTimeout     time.Duration // Timeout for progressive loading
	EnableBoostMode        bool          // Enable HTMX boost for SPA-like behavior
	PriorityDirectives     []string      // High priority HTMX directives
}

// HTMXElement represents an HTMX-enabled element
type HTMXElement struct {
	Tag               string
	Attributes        map[string]string
	Critical          bool
	LoadingStrategy   string
	Priority          int
	SizeEstimate      int
	Dependencies      []string
}

// LoadingStrategy defines how HTMX elements should be loaded
type LoadingStrategy struct {
	Name            string
	Trigger         string
	Delay           time.Duration
	Dependencies    []string
	FallbackContent string
}

// HTMXPerformanceMetrics tracks HTMX performance
type HTMXPerformanceMetrics struct {
	CriticalElementsCount   int
	DeferredElementsCount   int
	TotalSizeSaved          int
	AverageLoadTime         time.Duration
	ProgressiveLoadSuccess  int
	ProgressiveLoadFailures int
	LastOptimized           time.Time
}

// DefaultHTMXOptimizationConfig returns sensible defaults
func DefaultHTMXOptimizationConfig() HTMXOptimizationConfig {
	return HTMXOptimizationConfig{
		EnableCriticalPath:  true,
		DeferNonCritical:   true,
		InlineHTMXCore:     false, // Keep external for caching
		MaxCriticalElements: 5,
		LazyLoadThreshold:  ".fold-marker",
		ProgressiveTimeout: 5 * time.Second,
		EnableBoostMode:    true,
		PriorityDirectives: []string{
			"hx-get", "hx-post", "hx-trigger",
			"hx-target", "hx-swap", "hx-boost",
		},
	}
}

// NewHTMXOptimizer creates a new HTMX optimizer
func NewHTMXOptimizer(config HTMXOptimizationConfig) *HTMXOptimizer {
	return &HTMXOptimizer{
		config:           config,
		criticalElements: make(map[string]HTMXElement),
		loadingStrategies: map[string]LoadingStrategy{
			"immediate": {
				Name:    "immediate",
				Trigger: "load",
				Delay:   0,
			},
			"deferred": {
				Name:    "deferred",
				Trigger: "intersect",
				Delay:   100 * time.Millisecond,
			},
			"lazy": {
				Name:    "lazy",
				Trigger: "revealed",
				Delay:   200 * time.Millisecond,
			},
			"on-demand": {
				Name:    "on-demand",
				Trigger: "click",
				Delay:   0,
			},
		},
		performanceMetrics: HTMXPerformanceMetrics{
			LastOptimized: time.Now(),
		},
	}
}

// OptimizeHTML optimizes HTML content for HTMX progressive enhancement
func (ho *HTMXOptimizer) OptimizeHTML(html string) (string, error) {
	// Parse HTMX elements
	elements, err := ho.parseHTMXElements(html)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTMX elements: %w", err)
	}

	// Classify elements by priority
	critical, deferred := ho.classifyElements(elements)

	// Generate optimized HTML
	optimized := ho.generateOptimizedHTML(html, critical, deferred)

	// Add progressive enhancement script
	if ho.config.EnableCriticalPath {
		optimized = ho.addProgressiveEnhancement(optimized)
	}

	// Update metrics
	ho.updateMetrics(critical, deferred)

	return optimized, nil
}

// parseHTMXElements extracts HTMX elements from HTML
func (ho *HTMXOptimizer) parseHTMXElements(html string) ([]HTMXElement, error) {
	var elements []HTMXElement

	// Regex to find HTMX attributes
	htmxRegex := regexp.MustCompile(`<([^>]*?\bhx-[^>]*?)>`)
	matches := htmxRegex.FindAllStringSubmatch(html, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		element := ho.parseElementAttributes(match[1])
		if element.Tag != "" {
			elements = append(elements, element)
		}
	}

	return elements, nil
}

// parseElementAttributes parses HTML element attributes
func (ho *HTMXOptimizer) parseElementAttributes(elementHTML string) HTMXElement {
	element := HTMXElement{
		Attributes: make(map[string]string),
	}

	// Extract tag name
	tagRegex := regexp.MustCompile(`^(\w+)`)
	if tagMatch := tagRegex.FindStringSubmatch(elementHTML); len(tagMatch) > 1 {
		element.Tag = tagMatch[1]
	}

	// Extract HTMX attributes
	attrRegex := regexp.MustCompile(`(hx-[\w-]+)=["']([^"']*?)["']`)
	attrMatches := attrRegex.FindAllStringSubmatch(elementHTML, -1)

	for _, attrMatch := range attrMatches {
		if len(attrMatch) >= 3 {
			element.Attributes[attrMatch[1]] = attrMatch[2]
		}
	}

	// Calculate priority and size estimate
	element.Priority = ho.calculateElementPriority(element)
	element.SizeEstimate = ho.estimateElementSize(element)

	return element
}

// classifyElements separates critical from deferred elements
func (ho *HTMXOptimizer) classifyElements(elements []HTMXElement) ([]HTMXElement, []HTMXElement) {
	var critical, deferred []HTMXElement

	// Sort elements by priority
	sortedElements := make([]HTMXElement, len(elements))
	copy(sortedElements, elements)

	// Simple bubble sort by priority (sufficient for small arrays)
	for i := 0; i < len(sortedElements)-1; i++ {
		for j := 0; j < len(sortedElements)-i-1; j++ {
			if sortedElements[j].Priority < sortedElements[j+1].Priority {
				sortedElements[j], sortedElements[j+1] = sortedElements[j+1], sortedElements[j]
			}
		}
	}

	// Classify based on priority and limits
	criticalCount := 0
	for _, element := range sortedElements {
		if ho.isCriticalElement(element) && criticalCount < ho.config.MaxCriticalElements {
			element.Critical = true
			element.LoadingStrategy = "immediate"
			critical = append(critical, element)
			criticalCount++
		} else {
			element.Critical = false
			element.LoadingStrategy = ho.selectLoadingStrategy(element)
			deferred = append(deferred, element)
		}
	}

	return critical, deferred
}

// calculateElementPriority calculates priority score for an HTMX element
func (ho *HTMXOptimizer) calculateElementPriority(element HTMXElement) int {
	priority := 0

	// High priority attributes
	highPriorityAttrs := map[string]int{
		"hx-boost":   50,
		"hx-get":     40,
		"hx-post":    40,
		"hx-trigger": 30,
		"hx-target":  25,
		"hx-swap":    20,
	}

	for attr := range element.Attributes {
		if score, exists := highPriorityAttrs[attr]; exists {
			priority += score
		}
	}

	// Boost priority for certain triggers
	if trigger, exists := element.Attributes["hx-trigger"]; exists {
		immediateTriggers := []string{"load", "revealed", "click"}
		for _, immediateTrigger := range immediateTriggers {
			if strings.Contains(trigger, immediateTrigger) {
				priority += 20
				break
			}
		}
	}

	// Boost priority for form elements
	if element.Tag == "form" || element.Tag == "button" || element.Tag == "input" {
		priority += 15
	}

	// Penalty for complex selectors
	if target, exists := element.Attributes["hx-target"]; exists {
		if strings.Contains(target, " ") || strings.Contains(target, ":") {
			priority -= 10
		}
	}

	return priority
}

// estimateElementSize estimates the size impact of an HTMX element
func (ho *HTMXOptimizer) estimateElementSize(element HTMXElement) int {
	size := len(element.Tag) + 10 // Base tag size

	for attr, value := range element.Attributes {
		size += len(attr) + len(value) + 4 // attr="value"
	}

	return size
}

// isCriticalElement determines if an element is critical for first render
func (ho *HTMXOptimizer) isCriticalElement(element HTMXElement) bool {
	// Always critical elements
	criticalTriggers := []string{"load", "revealed"}
	if trigger, exists := element.Attributes["hx-trigger"]; exists {
		for _, critTrigger := range criticalTriggers {
			if strings.Contains(trigger, critTrigger) {
				return true
			}
		}
	}

	// Boost mode is always critical
	if _, exists := element.Attributes["hx-boost"]; exists {
		return true
	}

	// Form submissions are critical
	if element.Tag == "form" || (element.Tag == "button" && element.Attributes["hx-post"] != "") {
		return true
	}

	// High priority score makes it critical
	return element.Priority >= 60
}

// selectLoadingStrategy selects appropriate loading strategy for deferred elements
func (ho *HTMXOptimizer) selectLoadingStrategy(element HTMXElement) string {
	// Check for existing trigger
	if trigger, exists := element.Attributes["hx-trigger"]; exists {
		if strings.Contains(trigger, "click") || strings.Contains(trigger, "submit") {
			return "on-demand"
		}
		if strings.Contains(trigger, "intersect") || strings.Contains(trigger, "revealed") {
			return "lazy"
		}
	}

	// Default strategies by element type
	if element.Tag == "form" || element.Tag == "button" {
		return "deferred"
	}

	return "lazy"
}

// generateOptimizedHTML creates optimized HTML with progressive enhancement
func (ho *HTMXOptimizer) generateOptimizedHTML(original string, critical, deferred []HTMXElement) string {
	optimized := original

	// Process deferred elements
	for _, element := range deferred {
		optimized = ho.transformDeferredElement(optimized, element)
	}

	return optimized
}

// transformDeferredElement transforms an element for deferred loading
func (ho *HTMXOptimizer) transformDeferredElement(html string, element HTMXElement) string {
	strategy := ho.loadingStrategies[element.LoadingStrategy]

	// Find the element in HTML and modify it
	for attr, value := range element.Attributes {
		// Convert immediate triggers to deferred ones
		if attr == "hx-trigger" && strategy.Name == "lazy" {
			newValue := ho.convertToLazyTrigger(value)
			html = strings.ReplaceAll(html, fmt.Sprintf(`%s="%s"`, attr, value), 
				fmt.Sprintf(`%s="%s"`, attr, newValue))
		}

		// Add loading indicators for deferred content
		if attr == "hx-get" || attr == "hx-post" {
			indicatorAttr := fmt.Sprintf(`hx-indicator="#loading-%s"`, ho.generateElementID(element))
			html = ho.addAttribute(html, element, indicatorAttr)
		}
	}

	return html
}

// convertToLazyTrigger converts immediate triggers to lazy loading triggers
func (ho *HTMXOptimizer) convertToLazyTrigger(trigger string) string {
	// Convert load to intersect for lazy loading
	if trigger == "load" {
		return "intersect once"
	}

	// Add intersection observer to existing triggers
	if !strings.Contains(trigger, "intersect") {
		return fmt.Sprintf("intersect once, %s", trigger)
	}

	return trigger
}

// addAttribute adds an attribute to an element in HTML
func (ho *HTMXOptimizer) addAttribute(html string, element HTMXElement, attribute string) string {
	// This is a simplified implementation
	// In practice, you'd use a proper HTML parser
	return html
}

// generateElementID generates a unique ID for an element
func (ho *HTMXOptimizer) generateElementID(element HTMXElement) string {
	return fmt.Sprintf("htmx-%s-%d", element.Tag, time.Now().UnixNano()%10000)
}

// addProgressiveEnhancement adds progressive enhancement JavaScript
func (ho *HTMXOptimizer) addProgressiveEnhancement(html string) string {
	progressiveScript := ho.generateProgressiveScript()
	
	// Insert before closing body tag
	if strings.Contains(html, "</body>") {
		html = strings.Replace(html, "</body>", progressiveScript+"</body>", 1)
	} else {
		html += progressiveScript
	}

	return html
}

// generateProgressiveScript generates JavaScript for progressive enhancement
func (ho *HTMXOptimizer) generateProgressiveScript() string {
	return `
<script>
(function() {
    'use strict';
    
    // Progressive HTMX Enhancement
    const HTMXProgressiveLoader = {
        init: function() {
            this.setupIntersectionObserver();
            this.setupDeferredLoading();
            this.setupErrorHandling();
            this.monitorPerformance();
        },
        
        setupIntersectionObserver: function() {
            if (!('IntersectionObserver' in window)) {
                // Fallback for older browsers
                this.loadAllDeferred();
                return;
            }
            
            const observer = new IntersectionObserver((entries) => {
                entries.forEach(entry => {
                    if (entry.isIntersecting) {
                        const element = entry.target;
                        this.activateElement(element);
                        observer.unobserve(element);
                    }
                });
            }, {
                threshold: 0.1,
                rootMargin: '50px'
            });
            
            // Observe lazy-loaded elements
            document.querySelectorAll('[hx-trigger*="intersect"]').forEach(el => {
                observer.observe(el);
            });
        },
        
        setupDeferredLoading: function() {
            // Load deferred elements after initial page load
            setTimeout(() => {
                document.querySelectorAll('[data-htmx-deferred]').forEach(el => {
                    this.activateElement(el);
                });
            }, 100);
        },
        
        activateElement: function(element) {
            // Remove deferred marker
            element.removeAttribute('data-htmx-deferred');
            
            // Trigger HTMX processing if needed
            if (window.htmx) {
                htmx.process(element);
            }
        },
        
        setupErrorHandling: function() {
            document.addEventListener('htmx:responseError', function(event) {
                console.warn('HTMX Error:', event.detail);
                // Implement fallback behavior
                this.handleError(event.target, event.detail);
            }.bind(this));
        },
        
        handleError: function(element, error) {
            // Show fallback content or retry mechanism
            const fallback = element.getAttribute('data-fallback');
            if (fallback) {
                element.innerHTML = fallback;
            }
        },
        
        loadAllDeferred: function() {
            // Fallback for browsers without IntersectionObserver
            document.querySelectorAll('[data-htmx-deferred]').forEach(el => {
                this.activateElement(el);
            });
        },
        
        monitorPerformance: function() {
            if (!('performance' in window)) return;
            
            // Track HTMX request performance
            document.addEventListener('htmx:beforeRequest', function(event) {
                event.detail.elt.setAttribute('data-start-time', performance.now());
            });
            
            document.addEventListener('htmx:afterRequest', function(event) {
                const startTime = parseFloat(event.detail.elt.getAttribute('data-start-time'));
                if (startTime) {
                    const duration = performance.now() - startTime;
                    console.log('HTMX Request Duration:', duration + 'ms');
                    event.detail.elt.removeAttribute('data-start-time');
                }
            });
        }
    };
    
    // Initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => HTMXProgressiveLoader.init());
    } else {
        HTMXProgressiveLoader.init();
    }
    
    // Global error boundary
    window.addEventListener('error', function(event) {
        console.warn('Progressive Enhancement Error:', event.error);
    });
})();
</script>`
}

// updateMetrics updates performance metrics
func (ho *HTMXOptimizer) updateMetrics(critical, deferred []HTMXElement) {
	ho.performanceMetrics.CriticalElementsCount = len(critical)
	ho.performanceMetrics.DeferredElementsCount = len(deferred)
	ho.performanceMetrics.LastOptimized = time.Now()

	// Calculate size savings
	sizeSaved := 0
	for _, element := range deferred {
		sizeSaved += element.SizeEstimate / 2 // Estimated 50% reduction from deferral
	}
	ho.performanceMetrics.TotalSizeSaved = sizeSaved
}

// GetOptimizationReport generates a detailed HTMX optimization report
func (ho *HTMXOptimizer) GetOptimizationReport() string {
	metrics := ho.performanceMetrics
	
	var report strings.Builder
	report.WriteString("=== HTMX Optimization Report ===\n")
	report.WriteString(fmt.Sprintf("Last Optimized: %s\n", metrics.LastOptimized.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Critical Elements: %d\n", metrics.CriticalElementsCount))
	report.WriteString(fmt.Sprintf("Deferred Elements: %d\n", metrics.DeferredElementsCount))
	report.WriteString(fmt.Sprintf("Total Size Saved: %d bytes\n", metrics.TotalSizeSaved))
	
	if metrics.CriticalElementsCount+metrics.DeferredElementsCount > 0 {
		deferralRate := float64(metrics.DeferredElementsCount) / 
			float64(metrics.CriticalElementsCount+metrics.DeferredElementsCount) * 100
		report.WriteString(fmt.Sprintf("Deferral Rate: %.1f%%\n", deferralRate))
	}
	
	report.WriteString(fmt.Sprintf("Progressive Load Success: %d\n", metrics.ProgressiveLoadSuccess))
	report.WriteString(fmt.Sprintf("Progressive Load Failures: %d\n", metrics.ProgressiveLoadFailures))
	
	// Configuration summary
	report.WriteString("\n=== Configuration ===\n")
	report.WriteString(fmt.Sprintf("Critical Path Enabled: %t\n", ho.config.EnableCriticalPath))
	report.WriteString(fmt.Sprintf("Defer Non-Critical: %t\n", ho.config.DeferNonCritical))
	report.WriteString(fmt.Sprintf("Max Critical Elements: %d\n", ho.config.MaxCriticalElements))
	report.WriteString(fmt.Sprintf("Boost Mode: %t\n", ho.config.EnableBoostMode))
	
	return report.String()
}

// GetMetrics returns current HTMX performance metrics
func (ho *HTMXOptimizer) GetMetrics() HTMXPerformanceMetrics {
	return ho.performanceMetrics
}