// Package security provides XSS (Cross-Site Scripting) protection
package security

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// XSSProtectionService provides comprehensive XSS protection
type XSSProtectionService struct {
	logger             *zap.Logger
	allowedTags        map[string]bool
	allowedAttributes  map[string]bool
	dangerousPatterns  []*regexp.Regexp
	urlSchemeWhitelist map[string]bool
}

// NewXSSProtectionService creates a new XSS protection service
func NewXSSProtectionService(logger *zap.Logger) *XSSProtectionService {
	service := &XSSProtectionService{
		logger:             logger,
		allowedTags:        make(map[string]bool),
		allowedAttributes:  make(map[string]bool),
		urlSchemeWhitelist: make(map[string]bool),
	}
	
	service.initializeDefaults()
	service.initializeDangerousPatterns()
	
	return service
}

// initializeDefaults sets up default allowed tags and attributes
func (x *XSSProtectionService) initializeDefaults() {
	// Safe HTML tags for rich content
	safeTags := []string{
		"p", "br", "strong", "em", "u", "s", "sub", "sup",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li", "dl", "dt", "dd",
		"blockquote", "cite", "q",
		"table", "thead", "tbody", "tr", "td", "th",
		"a", "img",
	}
	
	for _, tag := range safeTags {
		x.allowedTags[tag] = true
	}
	
	// Safe attributes
	safeAttributes := []string{
		"href", "src", "alt", "title", "class", "id",
		"width", "height", "style",
		"colspan", "rowspan",
	}
	
	for _, attr := range safeAttributes {
		x.allowedAttributes[attr] = true
	}
	
	// Safe URL schemes
	safeSchemes := []string{
		"http", "https", "mailto", "tel", "ftp",
	}
	
	for _, scheme := range safeSchemes {
		x.urlSchemeWhitelist[scheme] = true
	}
}

// initializeDangerousPatterns sets up regex patterns for XSS detection
func (x *XSSProtectionService) initializeDangerousPatterns() {
	patterns := []string{
		// Script tags
		`(?i)<script[^>]*>.*?</script>`,
		`(?i)<script[^>]*/>`,
		`(?i)<script[^>]*>`,
		
		// JavaScript event handlers
		`(?i)on\w+\s*=\s*["'][^"']*["']`,
		`(?i)on\w+\s*=\s*[^"'\s>]+`,
		
		// JavaScript URLs
		`(?i)javascript:\s*[^"'\s>]*`,
		`(?i)vbscript:\s*[^"'\s>]*`,
		`(?i)data:\s*[^"'\s>]*`,
		
		// Style tags and expressions
		`(?i)<style[^>]*>.*?</style>`,
		`(?i)expression\s*\(`,
		`(?i)url\s*\(\s*["']?javascript:`,
		
		// Meta and link tags
		`(?i)<meta[^>]*http-equiv`,
		`(?i)<link[^>]*rel\s*=\s*["']?stylesheet`,
		
		// Object, embed, applet tags
		`(?i)<object[^>]*>`,
		`(?i)<embed[^>]*>`,
		`(?i)<applet[^>]*>`,
		
		// IFrame tags
		`(?i)<iframe[^>]*>`,
		`(?i)<frame[^>]*>`,
		`(?i)<frameset[^>]*>`,
		
		// Form tags
		`(?i)<form[^>]*>`,
		`(?i)<input[^>]*>`,
		`(?i)<textarea[^>]*>`,
		`(?i)<select[^>]*>`,
		`(?i)<button[^>]*>`,
		
		// Base tag
		`(?i)<base[^>]*>`,
		
		// Comment-based XSS
		`<!--.*?-->`,
		
		// CDATA sections
		`<!\[CDATA\[.*?\]\]>`,
		
		// XML processing instructions
		`<\?.*?\?>`,
		
		// Import statements
		`(?i)@import`,
		
		// Function calls in attributes
		`(?i)(alert|confirm|prompt|eval|setTimeout|setInterval)\s*\(`,
		
		// Document methods
		`(?i)document\.(write|writeln|open|close|cookie)`,
		
		// Window methods
		`(?i)window\.(open|location|navigate)`,
		
		// Location object
		`(?i)location\.(href|replace|assign)`,
		
		// Dangerous CSS
		`(?i)behavior\s*:\s*url`,
		`(?i)-moz-binding\s*:`,
	}
	
	x.dangerousPatterns = make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		x.dangerousPatterns[i] = regexp.MustCompile(pattern)
	}
}

// SanitizeHTML removes dangerous HTML content while preserving safe formatting
func (x *XSSProtectionService) SanitizeHTML(input string) string {
	// First, check for dangerous patterns
	for _, pattern := range x.dangerousPatterns {
		if pattern.MatchString(input) {
			x.logger.Warn("Dangerous XSS pattern detected",
				zap.String("pattern", pattern.String()),
				zap.String("input_sample", x.truncateForLogging(input, 100)),
			)
			// Remove the dangerous content
			input = pattern.ReplaceAllString(input, "")
		}
	}
	
	// Parse and sanitize HTML
	sanitized := x.sanitizeHTMLTags(input)
	
	// Additional cleanup
	sanitized = x.cleanupHTML(sanitized)
	
	return sanitized
}

// sanitizeHTMLTags processes HTML tags and attributes
func (x *XSSProtectionService) sanitizeHTMLTags(input string) string {
	// Regex to find HTML tags
	tagRegex := regexp.MustCompile(`<(/?)(\w+)([^>]*)>`)
	
	result := tagRegex.ReplaceAllStringFunc(input, func(match string) string {
		matches := tagRegex.FindStringSubmatch(match)
		if len(matches) != 4 {
			return ""
		}
		
		isClosing := matches[1] == "/"
		tagName := strings.ToLower(matches[2])
		attributes := matches[3]
		
		// Check if tag is allowed
		if !x.allowedTags[tagName] {
			x.logger.Debug("Removing disallowed HTML tag",
				zap.String("tag", tagName),
			)
			return ""
		}
		
		// For closing tags, just return if allowed
		if isClosing {
			return fmt.Sprintf("</%s>", tagName)
		}
		
		// Sanitize attributes
		sanitizedAttrs := x.sanitizeAttributes(attributes)
		
		if sanitizedAttrs == "" {
			return fmt.Sprintf("<%s>", tagName)
		}
		
		return fmt.Sprintf("<%s%s>", tagName, sanitizedAttrs)
	})
	
	return result
}

// sanitizeAttributes cleans HTML attributes
func (x *XSSProtectionService) sanitizeAttributes(attributes string) string {
	if attributes == "" {
		return ""
	}
	
	// Regex to find attributes
	attrRegex := regexp.MustCompile(`(\w+)\s*=\s*["']([^"']*)["']`)
	
	var sanitizedAttrs []string
	
	matches := attrRegex.FindAllStringSubmatch(attributes, -1)
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		
		attrName := strings.ToLower(match[1])
		attrValue := match[2]
		
		// Check if attribute is allowed
		if !x.allowedAttributes[attrName] {
			x.logger.Debug("Removing disallowed HTML attribute",
				zap.String("attribute", attrName),
			)
			continue
		}
		
		// Sanitize attribute value
		sanitizedValue := x.sanitizeAttributeValue(attrName, attrValue)
		if sanitizedValue == "" {
			continue
		}
		
		sanitizedAttrs = append(sanitizedAttrs, fmt.Sprintf(`%s="%s"`, attrName, sanitizedValue))
	}
	
	if len(sanitizedAttrs) == 0 {
		return ""
	}
	
	return " " + strings.Join(sanitizedAttrs, " ")
}

// sanitizeAttributeValue sanitizes attribute values
func (x *XSSProtectionService) sanitizeAttributeValue(attrName, value string) string {
	// Check for dangerous patterns in attribute values
	for _, pattern := range x.dangerousPatterns {
		if pattern.MatchString(value) {
			x.logger.Warn("Dangerous pattern in attribute value",
				zap.String("attribute", attrName),
				zap.String("pattern", pattern.String()),
			)
			return ""
		}
	}
	
	// Special handling for URLs
	if attrName == "href" || attrName == "src" {
		return x.sanitizeURL(value)
	}
	
	// Special handling for style attributes
	if attrName == "style" {
		return x.sanitizeCSS(value)
	}
	
	// HTML encode the value
	return html.EscapeString(value)
}

// sanitizeURL validates and sanitizes URLs
func (x *XSSProtectionService) sanitizeURL(url string) string {
	url = strings.TrimSpace(url)
	
	// Check for JavaScript URLs
	if strings.HasPrefix(strings.ToLower(url), "javascript:") ||
		strings.HasPrefix(strings.ToLower(url), "vbscript:") ||
		strings.HasPrefix(strings.ToLower(url), "data:") {
		x.logger.Warn("Dangerous URL scheme detected", zap.String("url", url))
		return ""
	}
	
	// Check scheme whitelist
	if strings.Contains(url, ":") {
		parts := strings.SplitN(url, ":", 2)
		scheme := strings.ToLower(parts[0])
		if !x.urlSchemeWhitelist[scheme] {
			x.logger.Warn("Non-whitelisted URL scheme", zap.String("scheme", scheme))
			return ""
		}
	}
	
	return html.EscapeString(url)
}

// sanitizeCSS sanitizes CSS styles
func (x *XSSProtectionService) sanitizeCSS(css string) string {
	css = strings.TrimSpace(css)
	
	// Remove dangerous CSS patterns
	dangerousCSS := []string{
		"expression", "behavior", "javascript:", "vbscript:",
		"@import", "binding", "-moz-binding",
	}
	
	cssLower := strings.ToLower(css)
	for _, danger := range dangerousCSS {
		if strings.Contains(cssLower, danger) {
			x.logger.Warn("Dangerous CSS pattern detected", zap.String("pattern", danger))
			return ""
		}
	}
	
	return html.EscapeString(css)
}

// cleanupHTML performs final cleanup
func (x *XSSProtectionService) cleanupHTML(input string) string {
	// Remove any remaining dangerous sequences
	input = strings.ReplaceAll(input, "javascript:", "")
	input = strings.ReplaceAll(input, "vbscript:", "")
	input = strings.ReplaceAll(input, "data:", "")
	
	// Normalize whitespace
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")
	input = strings.TrimSpace(input)
	
	return input
}

// StripHTML completely removes all HTML tags
func (x *XSSProtectionService) StripHTML(input string) string {
	// Remove all HTML tags
	tagRegex := regexp.MustCompile(`<[^>]*>`)
	result := tagRegex.ReplaceAllString(input, "")
	
	// Decode HTML entities
	result = html.UnescapeString(result)
	
	// Remove any remaining dangerous content
	for _, pattern := range x.dangerousPatterns {
		result = pattern.ReplaceAllString(result, "")
	}
	
	return strings.TrimSpace(result)
}

// ValidateInput checks input for XSS patterns without modifying it
func (x *XSSProtectionService) ValidateInput(input string) error {
	for _, pattern := range x.dangerousPatterns {
		if pattern.MatchString(input) {
			return fmt.Errorf("input contains potentially dangerous content")
		}
	}
	return nil
}

// XSSProtectionMiddleware provides XSS protection for HTTP requests
func (x *XSSProtectionService) XSSProtectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set XSS protection headers
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("X-Content-Type-Options", "nosniff")
		
		// For POST/PUT requests, validate form data
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			if err := c.Request.ParseForm(); err == nil {
				for key, values := range c.Request.Form {
					for _, value := range values {
						if err := x.ValidateInput(value); err != nil {
							x.logger.Warn("XSS pattern detected in form data",
								zap.String("field", key),
								zap.String("error", err.Error()),
								zap.String("ip", c.ClientIP()),
							)
							
							c.JSON(http.StatusBadRequest, gin.H{
								"error": "Invalid input detected",
								"field": key,
							})
							c.Abort()
							return
						}
					}
				}
			}
		}
		
		c.Next()
	}
}

// SafeHTML wraps template.HTML with XSS protection
func (x *XSSProtectionService) SafeHTML(input string) template.HTML {
	sanitized := x.SanitizeHTML(input)
	return template.HTML(sanitized)
}

// SafeJS sanitizes JavaScript content
func (x *XSSProtectionService) SafeJS(input string) template.JS {
	// Remove dangerous JavaScript patterns
	jsPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)eval\s*\(`),
		regexp.MustCompile(`(?i)Function\s*\(`),
		regexp.MustCompile(`(?i)setTimeout\s*\(\s*["'][^"']*["']`),
		regexp.MustCompile(`(?i)setInterval\s*\(\s*["'][^"']*["']`),
		regexp.MustCompile(`(?i)document\.write`),
		regexp.MustCompile(`(?i)document\.writeln`),
		regexp.MustCompile(`(?i)location\.href`),
		regexp.MustCompile(`(?i)window\.location`),
	}
	
	result := input
	for _, pattern := range jsPatterns {
		if pattern.MatchString(result) {
			x.logger.Warn("Dangerous JavaScript pattern detected")
			result = pattern.ReplaceAllString(result, "")
		}
	}
	
	return template.JS(result)
}

// SafeCSS sanitizes CSS content
func (x *XSSProtectionService) SafeCSS(input string) template.CSS {
	sanitized := x.sanitizeCSS(input)
	return template.CSS(sanitized)
}

// SafeURL sanitizes URLs for templates
func (x *XSSProtectionService) SafeURL(input string) template.URL {
	sanitized := x.sanitizeURL(input)
	return template.URL(sanitized)
}

// truncateForLogging truncates strings for safe logging
func (x *XSSProtectionService) truncateForLogging(input string, maxLen int) string {
	if len(input) <= maxLen {
		return input
	}
	return input[:maxLen] + "..."
}

// SetAllowedTags updates the list of allowed HTML tags
func (x *XSSProtectionService) SetAllowedTags(tags []string) {
	x.allowedTags = make(map[string]bool)
	for _, tag := range tags {
		x.allowedTags[strings.ToLower(tag)] = true
	}
}

// SetAllowedAttributes updates the list of allowed HTML attributes
func (x *XSSProtectionService) SetAllowedAttributes(attributes []string) {
	x.allowedAttributes = make(map[string]bool)
	for _, attr := range attributes {
		x.allowedAttributes[strings.ToLower(attr)] = true
	}
}

// AddAllowedTag adds a tag to the allowed list
func (x *XSSProtectionService) AddAllowedTag(tag string) {
	x.allowedTags[strings.ToLower(tag)] = true
}

// RemoveAllowedTag removes a tag from the allowed list
func (x *XSSProtectionService) RemoveAllowedTag(tag string) {
	delete(x.allowedTags, strings.ToLower(tag))
}

// GetContentSecurityPolicy returns a strict CSP header value
func (x *XSSProtectionService) GetContentSecurityPolicy(nonce string) string {
	csp := []string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: https:",
		"font-src 'self' data:",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'none'",
		"object-src 'none'",
		"media-src 'self'",
	}
	
	if nonce != "" {
		// Add nonce for inline scripts and styles
		for i, directive := range csp {
			if strings.HasPrefix(directive, "script-src") {
				csp[i] = strings.Replace(directive, "'unsafe-inline'", fmt.Sprintf("'nonce-%s'", nonce), 1)
			}
			if strings.HasPrefix(directive, "style-src") {
				csp[i] = strings.Replace(directive, "'unsafe-inline'", fmt.Sprintf("'nonce-%s'", nonce), 1)
			}
		}
	}
	
	return strings.Join(csp, "; ")
}