// Package performance provides critical CSS extraction and optimization
package performance

import (
	"bufio"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// CriticalCSSExtractor extracts and manages critical CSS for above-the-fold content
type CriticalCSSExtractor struct {
	selectors          map[string]*CSSRule
	criticalSelectors  []string
	foldHeight         int
	extractedCSS       string
	lastExtraction     time.Time
	optimizationLevel  int
}

// CSSRule represents a CSS rule with metadata
type CSSRule struct {
	Selector    string
	Properties  map[string]string
	Priority    int
	Critical    bool
	Size        int
	UsageCount  int
}

// CSSExtractConfig configures CSS extraction behavior
type CSSExtractConfig struct {
	FoldHeight         int      // Pixels above the fold to consider
	MaxCriticalSize    int      // Maximum critical CSS size in bytes
	PrioritySelectors  []string // High priority CSS selectors
	ExcludeSelectors   []string // Selectors to exclude from critical CSS
	OptimizationLevel  int      // 1-3, higher = more aggressive optimization
}

// NewCriticalCSSExtractor creates a new CSS extractor
func NewCriticalCSSExtractor(config CSSExtractConfig) *CriticalCSSExtractor {
	if config.FoldHeight == 0 {
		config.FoldHeight = 600 // Default fold height for mobile
	}
	if config.MaxCriticalSize == 0 {
		config.MaxCriticalSize = MaxCriticalCSS
	}
	if config.OptimizationLevel == 0 {
		config.OptimizationLevel = 2
	}

	return &CriticalCSSExtractor{
		selectors:         make(map[string]*CSSRule),
		foldHeight:        config.FoldHeight,
		optimizationLevel: config.OptimizationLevel,
	}
}

// ExtractCriticalCSS extracts critical CSS from full CSS content
func (cce *CriticalCSSExtractor) ExtractCriticalCSS(cssContent string, htmlContent string) (string, error) {
	// Parse CSS rules
	rules, err := cce.parseCSS(cssContent)
	if err != nil {
		return "", fmt.Errorf("CSS parsing failed: %w", err)
	}

	// Analyze HTML structure to identify critical selectors
	criticalSelectors := cce.identifyCriticalSelectors(htmlContent, rules)

	// Build critical CSS with size constraints
	criticalCSS := cce.buildCriticalCSS(criticalSelectors, rules)

	// Optimize critical CSS
	optimized := cce.optimizeCSS(criticalCSS)

	cce.extractedCSS = optimized
	cce.lastExtraction = time.Now()

	return optimized, nil
}

// parseCSS parses CSS content into structured rules
func (cce *CriticalCSSExtractor) parseCSS(cssContent string) ([]*CSSRule, error) {
	var rules []*CSSRule
	
	// Remove comments
	cssContent = cce.removeComments(cssContent)
	
	// Split into rules using regex
	ruleRegex := regexp.MustCompile(`([^{]+)\{([^}]+)\}`)
	matches := ruleRegex.FindAllStringSubmatch(cssContent, -1)
	
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		
		selector := strings.TrimSpace(match[1])
		declarations := strings.TrimSpace(match[2])
		
		rule := &CSSRule{
			Selector:   selector,
			Properties: cce.parseDeclarations(declarations),
			Size:       len(match[0]),
		}
		
		// Assign priority based on selector type
		rule.Priority = cce.calculateSelectorPriority(selector)
		
		rules = append(rules, rule)
		cce.selectors[selector] = rule
	}
	
	return rules, nil
}

// identifyCriticalSelectors identifies which selectors are critical for above-the-fold content
func (cce *CriticalCSSExtractor) identifyCriticalSelectors(htmlContent string, rules []*CSSRule) []string {
	var criticalSelectors []string
	
	// Define critical selector patterns
	criticalPatterns := []string{
		`body`, `html`, `*`,                    // Base elements
		`\.container`, `\.header`, `\.nav`,     // Layout elements
		`\.hero`, `\.main`, `\.content`,        // Content areas
		`h1`, `h2`, `h3`, `p`,                  // Typography
		`\.btn`, `\.btn-primary`,               // Interactive elements
		`\.logo`, `\.menu`,                     // Navigation
		`\.form`, `\.form-input`,               // Forms
		`\.alert`, `\.message`,                 // Messages
		`\.grid`, `\.flex`, `\.card`,           // Layout utilities
	}
	
	// Add selectors that appear in HTML
	for _, rule := range rules {
		selector := rule.Selector
		
		// Check if selector matches critical patterns
		for _, pattern := range criticalPatterns {
			if matched, _ := regexp.MatchString(pattern, selector); matched {
				criticalSelectors = append(criticalSelectors, selector)
				rule.Critical = true
				break
			}
		}
		
		// Check if selector elements exist in HTML
		if cce.selectorExistsInHTML(selector, htmlContent) {
			// Check if it's likely above the fold
			if cce.isAboveFold(selector) {
				criticalSelectors = append(criticalSelectors, selector)
				rule.Critical = true
			}
		}
	}
	
	return criticalSelectors
}

// selectorExistsInHTML checks if a CSS selector targets elements in the HTML
func (cce *CriticalCSSExtractor) selectorExistsInHTML(selector, htmlContent string) bool {
	// Simplified check for basic selectors
	selector = strings.TrimSpace(selector)
	
	// Handle class selectors
	if strings.HasPrefix(selector, ".") {
		className := strings.TrimPrefix(selector, ".")
		return strings.Contains(htmlContent, fmt.Sprintf(`class="%s"`, className)) ||
			   strings.Contains(htmlContent, fmt.Sprintf(`class='%s'`, className))
	}
	
	// Handle ID selectors
	if strings.HasPrefix(selector, "#") {
		idName := strings.TrimPrefix(selector, "#")
		return strings.Contains(htmlContent, fmt.Sprintf(`id="%s"`, idName)) ||
			   strings.Contains(htmlContent, fmt.Sprintf(`id='%s'`, idName))
	}
	
	// Handle element selectors
	if !strings.Contains(selector, " ") && !strings.Contains(selector, ".") && 
		!strings.Contains(selector, "#") && !strings.Contains(selector, ":") {
		return strings.Contains(htmlContent, fmt.Sprintf("<%s", selector))
	}
	
	return false
}

// isAboveFold determines if a selector is likely above the fold
func (cce *CriticalCSSExtractor) isAboveFold(selector string) bool {
	// High priority selectors that are almost always above the fold
	aboveFoldSelectors := []string{
		"body", "html", "header", ".header", ".nav", ".navbar",
		"h1", ".hero", ".banner", ".logo", ".main-nav",
		".container", ".wrapper", ".content-wrapper",
	}
	
	for _, afs := range aboveFoldSelectors {
		if strings.Contains(selector, afs) {
			return true
		}
	}
	
	// Check for footer, sidebar, or other below-fold indicators
	belowFoldIndicators := []string{
		"footer", ".footer", ".sidebar", ".aside",
		".pagination", ".load-more", ".contact-form",
	}
	
	for _, bfi := range belowFoldIndicators {
		if strings.Contains(selector, bfi) {
			return false
		}
	}
	
	return true // Default to critical if uncertain
}

// buildCriticalCSS builds the final critical CSS from selected rules
func (cce *CriticalCSSExtractor) buildCriticalCSS(selectors []string, rules []*CSSRule) string {
	var criticalRules []*CSSRule
	
	// Sort selectors by priority
	sort.Slice(selectors, func(i, j int) bool {
		rule1 := cce.selectors[selectors[i]]
		rule2 := cce.selectors[selectors[j]]
		return rule1.Priority > rule2.Priority
	})
	
	currentSize := 0
	maxSize := MaxCriticalCSS
	
	// Add rules while staying within size limit
	for _, selector := range selectors {
		rule := cce.selectors[selector]
		if rule == nil {
			continue
		}
		
		ruleCSS := cce.formatCSSRule(rule)
		if currentSize+len(ruleCSS) > maxSize {
			break
		}
		
		criticalRules = append(criticalRules, rule)
		currentSize += len(ruleCSS)
	}
	
	// Build final CSS
	var cssBuilder strings.Builder
	cssBuilder.WriteString("/* Critical CSS - Inlined for 14KB optimization */\n")
	
	for _, rule := range criticalRules {
		cssBuilder.WriteString(cce.formatCSSRule(rule))
		cssBuilder.WriteString("\n")
	}
	
	return cssBuilder.String()
}

// optimizeCSS applies optimization techniques to reduce CSS size
func (cce *CriticalCSSExtractor) optimizeCSS(css string) string {
	optimized := css
	
	// Level 1 optimizations (safe)
	optimized = cce.removeComments(optimized)
	optimized = cce.removeUnnecessaryWhitespace(optimized)
	optimized = cce.removeEmptyRules(optimized)
	
	if cce.optimizationLevel >= 2 {
		// Level 2 optimizations (moderate)
		optimized = cce.shortenHexColors(optimized)
		optimized = cce.removeUnnecessaryQuotes(optimized)
		optimized = cce.optimizeMarginPadding(optimized)
	}
	
	if cce.optimizationLevel >= 3 {
		// Level 3 optimizations (aggressive)
		optimized = cce.mergeIdenticalRules(optimized)
		optimized = cce.removeRedundantDeclarations(optimized)
	}
	
	return optimized
}

// parseDeclarations parses CSS declarations into a map
func (cce *CriticalCSSExtractor) parseDeclarations(declarations string) map[string]string {
	props := make(map[string]string)
	
	scanner := bufio.NewScanner(strings.NewReader(declarations))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			prop := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(strings.TrimSuffix(parts[1], ";"))
			props[prop] = value
		}
	}
	
	return props
}

// calculateSelectorPriority assigns priority scores to selectors
func (cce *CriticalCSSExtractor) calculateSelectorPriority(selector string) int {
	priority := 0
	
	// High priority patterns
	highPriority := []string{"body", "html", "*", "container", "header", "nav", "main"}
	for _, hp := range highPriority {
		if strings.Contains(selector, hp) {
			priority += 100
		}
	}
	
	// Medium priority patterns
	mediumPriority := []string{"h1", "h2", "h3", "btn", "form", "input", "card"}
	for _, mp := range mediumPriority {
		if strings.Contains(selector, mp) {
			priority += 50
		}
	}
	
	// Penalty for complex selectors
	complexity := strings.Count(selector, " ") + strings.Count(selector, ">") + 
		strings.Count(selector, "+") + strings.Count(selector, "~")
	priority -= complexity * 5
	
	// Penalty for pseudo-selectors (often not critical)
	if strings.Contains(selector, ":") {
		priority -= 20
	}
	
	return priority
}

// formatCSSRule formats a CSS rule back to string
func (cce *CriticalCSSExtractor) formatCSSRule(rule *CSSRule) string {
	if len(rule.Properties) == 0 {
		return ""
	}
	
	var props []string
	for prop, value := range rule.Properties {
		props = append(props, fmt.Sprintf("%s:%s", prop, value))
	}
	
	return fmt.Sprintf("%s{%s}", rule.Selector, strings.Join(props, ";"))
}

// Optimization helper functions

func (cce *CriticalCSSExtractor) removeComments(css string) string {
	commentRegex := regexp.MustCompile(`/\*[^*]*\*+(?:[^/*][^*]*\*+)*/`)
	return commentRegex.ReplaceAllString(css, "")
}

func (cce *CriticalCSSExtractor) removeUnnecessaryWhitespace(css string) string {
	// Remove whitespace around braces and semicolons
	css = regexp.MustCompile(`\s*{\s*`).ReplaceAllString(css, "{")
	css = regexp.MustCompile(`\s*}\s*`).ReplaceAllString(css, "}")
	css = regexp.MustCompile(`\s*;\s*`).ReplaceAllString(css, ";")
	css = regexp.MustCompile(`\s*:\s*`).ReplaceAllString(css, ":")
	css = regexp.MustCompile(`\s+`).ReplaceAllString(css, " ")
	return strings.TrimSpace(css)
}

func (cce *CriticalCSSExtractor) removeEmptyRules(css string) string {
	emptyRuleRegex := regexp.MustCompile(`[^}]+{\s*}`)
	return emptyRuleRegex.ReplaceAllString(css, "")
}

func (cce *CriticalCSSExtractor) shortenHexColors(css string) string {
	// Convert 6-digit hex to 3-digit where possible
	hexRegex := regexp.MustCompile(`#([0-9a-fA-F])\1([0-9a-fA-F])\2([0-9a-fA-F])\3`)
	return hexRegex.ReplaceAllString(css, "#$1$2$3")
}

func (cce *CriticalCSSExtractor) removeUnnecessaryQuotes(css string) string {
	// Remove quotes from font families that don't need them
	fontRegex := regexp.MustCompile(`font-family:\s*['"]([a-zA-Z-]+)['"]`)
	return fontRegex.ReplaceAllString(css, "font-family:$1")
}

func (cce *CriticalCSSExtractor) optimizeMarginPadding(css string) string {
	// Convert margin:0 0 0 0 to margin:0
	marginRegex := regexp.MustCompile(`(margin|padding):\s*0\s+0\s+0\s+0`)
	return marginRegex.ReplaceAllString(css, "$1:0")
}

func (cce *CriticalCSSExtractor) mergeIdenticalRules(css string) string {
	// This is a simplified implementation
	// In practice, you'd want more sophisticated rule merging
	return css
}

func (cce *CriticalCSSExtractor) removeRedundantDeclarations(css string) string {
	// Remove redundant declarations (simplified)
	return css
}

// GetExtractedCSS returns the last extracted critical CSS
func (cce *CriticalCSSExtractor) GetExtractedCSS() string {
	return cce.extractedCSS
}

// GetExtractionTime returns when CSS was last extracted
func (cce *CriticalCSSExtractor) GetExtractionTime() time.Time {
	return cce.lastExtraction
}

// ValidateSize ensures critical CSS is within size limits
func (cce *CriticalCSSExtractor) ValidateSize(css string) error {
	if len(css) > MaxCriticalCSS {
		return fmt.Errorf("critical CSS size %d exceeds maximum of %d bytes", 
			len(css), MaxCriticalCSS)
	}
	return nil
}

// GetOptimizationReport generates a report on CSS optimization
func (cce *CriticalCSSExtractor) GetOptimizationReport() string {
	var report strings.Builder
	
	report.WriteString("=== Critical CSS Extraction Report ===\n")
	report.WriteString(fmt.Sprintf("Last Extraction: %s\n", cce.lastExtraction.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Total Selectors: %d\n", len(cce.selectors)))
	
	criticalCount := 0
	totalSize := 0
	for _, rule := range cce.selectors {
		if rule.Critical {
			criticalCount++
		}
		totalSize += rule.Size
	}
	
	report.WriteString(fmt.Sprintf("Critical Selectors: %d\n", criticalCount))
	report.WriteString(fmt.Sprintf("Critical CSS Size: %d bytes\n", len(cce.extractedCSS)))
	report.WriteString(fmt.Sprintf("Optimization Level: %d\n", cce.optimizationLevel))
	
	if len(cce.extractedCSS) > 0 {
		compressionRatio := float64(len(cce.extractedCSS)) / float64(totalSize) * 100
		report.WriteString(fmt.Sprintf("Size Reduction: %.1f%%\n", 100-compressionRatio))
	}
	
	return report.String()
}