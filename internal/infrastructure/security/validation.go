// Package security provides comprehensive input validation and sanitization
package security

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

// ValidationService provides input validation and sanitization
type ValidationService struct {
	logger    *zap.Logger
	validator *validator.Validate
}

// NewValidationService creates a new validation service
func NewValidationService(logger *zap.Logger) *ValidationService {
	validate := validator.New()
	
	// Register custom validation rules
	validate.RegisterValidation("recipe_name", validateRecipeName)
	validate.RegisterValidation("ingredient", validateIngredient)
	validate.RegisterValidation("no_sql_injection", validateNoSQLInjection)
	validate.RegisterValidation("no_xss", validateNoXSS)
	validate.RegisterValidation("safe_filename", validateSafeFilename)
	validate.RegisterValidation("safe_html", validateSafeHTML)
	validate.RegisterValidation("strong_password", validateStrongPassword)
	
	return &ValidationService{
		logger:    logger,
		validator: validate,
	}
}

// SanitizationConfig defines sanitization rules
type SanitizationConfig struct {
	StripHTML           bool
	StripJavaScript     bool
	StripSQLKeywords    bool
	NormalizeWhitespace bool
	MaxLength           int
	AllowedTags         []string
	AllowedAttributes   []string
}

// DefaultSanitizationConfig returns safe defaults
func DefaultSanitizationConfig() SanitizationConfig {
	return SanitizationConfig{
		StripHTML:           true,
		StripJavaScript:     true,
		StripSQLKeywords:    true,
		NormalizeWhitespace: true,
		MaxLength:           1000,
		AllowedTags:         []string{},
		AllowedAttributes:   []string{},
	}
}

// RecipeSanitizationConfig returns config for recipe content
func RecipeSanitizationConfig() SanitizationConfig {
	return SanitizationConfig{
		StripHTML:           false,
		StripJavaScript:     true,
		StripSQLKeywords:    true,
		NormalizeWhitespace: true,
		MaxLength:           5000,
		AllowedTags:         []string{"p", "br", "strong", "em", "ul", "ol", "li"},
		AllowedAttributes:   []string{},
	}
}

// SanitizeInput sanitizes input based on configuration
func (v *ValidationService) SanitizeInput(input string, config SanitizationConfig) string {
	// Trim whitespace
	result := strings.TrimSpace(input)
	
	// Enforce max length
	if config.MaxLength > 0 && len(result) > config.MaxLength {
		result = result[:config.MaxLength]
	}
	
	// Strip JavaScript
	if config.StripJavaScript {
		result = v.stripJavaScript(result)
	}
	
	// Strip SQL keywords
	if config.StripSQLKeywords {
		result = v.stripSQLKeywords(result)
	}
	
	// Handle HTML
	if config.StripHTML {
		result = v.stripHTML(result)
	} else {
		result = v.sanitizeHTML(result, config.AllowedTags, config.AllowedAttributes)
	}
	
	// Normalize whitespace
	if config.NormalizeWhitespace {
		result = v.normalizeWhitespace(result)
	}
	
	return result
}

// stripJavaScript removes JavaScript code and event handlers
func (v *ValidationService) stripJavaScript(input string) string {
	// Remove script tags
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	input = scriptRegex.ReplaceAllString(input, "")
	
	// Remove JavaScript event handlers
	eventRegex := regexp.MustCompile(`(?i)on[a-z]+\s*=\s*["'][^"']*["']`)
	input = eventRegex.ReplaceAllString(input, "")
	
	// Remove javascript: URLs
	jsURLRegex := regexp.MustCompile(`(?i)javascript:\s*[^"'\s>]*`)
	input = jsURLRegex.ReplaceAllString(input, "")
	
	// Remove eval, setTimeout, setInterval patterns
	evalRegex := regexp.MustCompile(`(?i)(eval|setTimeout|setInterval)\s*\(`)
	input = evalRegex.ReplaceAllString(input, "")
	
	return input
}

// stripSQLKeywords removes common SQL injection patterns
func (v *ValidationService) stripSQLKeywords(input string) string {
	sqlKeywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER",
		"EXEC", "EXECUTE", "UNION", "DECLARE", "CAST", "CONVERT",
		"SCRIPT", "JAVASCRIPT", "VBSCRIPT", "ONLOAD", "ONERROR",
		"--", "/*", "*/", "xp_", "sp_", "@@",
	}
	
	result := input
	for _, keyword := range sqlKeywords {
		// Case-insensitive replacement
		pattern := fmt.Sprintf(`(?i)\b%s\b`, regexp.QuoteMeta(keyword))
		regex := regexp.MustCompile(pattern)
		result = regex.ReplaceAllString(result, "")
	}
	
	return result
}

// stripHTML removes all HTML tags
func (v *ValidationService) stripHTML(input string) string {
	// Remove HTML tags
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	result := htmlRegex.ReplaceAllString(input, "")
	
	// Decode HTML entities
	result = html.UnescapeString(result)
	
	return result
}

// sanitizeHTML allows only whitelisted HTML tags and attributes
func (v *ValidationService) sanitizeHTML(input string, allowedTags, allowedAttrs []string) string {
	if len(allowedTags) == 0 {
		return v.stripHTML(input)
	}
	
	// This is a simplified implementation
	// In production, use a library like bluemonday
	result := input
	
	// Remove dangerous tags
	dangerousTags := []string{
		"script", "object", "embed", "link", "style", "iframe",
		"frame", "frameset", "meta", "base", "form", "input",
		"textarea", "button", "select", "option",
	}
	
	for _, tag := range dangerousTags {
		pattern := fmt.Sprintf(`(?i)<%s[^>]*>.*?</%s>|<%s[^>]*/>`, tag, tag, tag)
		regex := regexp.MustCompile(pattern)
		result = regex.ReplaceAllString(result, "")
	}
	
	// Escape HTML for safety
	result = html.EscapeString(result)
	
	return result
}

// normalizeWhitespace normalizes whitespace characters
func (v *ValidationService) normalizeWhitespace(input string) string {
	// Replace multiple whitespace with single space
	spaceRegex := regexp.MustCompile(`\s+`)
	result := spaceRegex.ReplaceAllString(input, " ")
	
	// Trim
	result = strings.TrimSpace(result)
	
	return result
}

// ValidationMiddleware provides request validation middleware
func (v *ValidationService) ValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate content type for POST/PUT requests
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			contentType := c.GetHeader("Content-Type")
			if contentType == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Content-Type header required"})
				c.Abort()
				return
			}
			
			// Check for valid content types
			validTypes := []string{
				"application/json",
				"application/x-www-form-urlencoded",
				"multipart/form-data",
			}
			
			valid := false
			for _, validType := range validTypes {
				if strings.Contains(contentType, validType) {
					valid = true
					break
				}
			}
			
			if !valid {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid content type"})
				c.Abort()
				return
			}
		}
		
		// Validate request size
		if c.Request.ContentLength > 10*1024*1024 { // 10MB limit
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Request too large"})
			c.Abort()
			return
		}
		
		// Check for suspicious patterns in URL
		if v.containsSuspiciousPatterns(c.Request.URL.Path) {
			v.logger.Warn("Suspicious URL pattern detected",
				zap.String("path", c.Request.URL.Path),
				zap.String("ip", c.ClientIP()),
				zap.String("user_agent", c.Request.UserAgent()),
			)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// containsSuspiciousPatterns checks for common attack patterns in URLs
func (v *ValidationService) containsSuspiciousPatterns(path string) bool {
	suspiciousPatterns := []string{
		"../", "..\\", "..", "%2e%2e", "%252e%252e",
		"<script", "</script>", "javascript:", "vbscript:",
		"onload=", "onerror=", "onclick=", "onmouseover=",
		"eval(", "alert(", "confirm(", "prompt(",
		"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "DROP ",
		"UNION ", "OR 1=1", "AND 1=1", "' OR '", "' AND '",
		"admin'--", "admin'/*", "1' OR '1'='1",
		"null", "/etc/passwd", "/proc/", "\\windows\\",
		"cmd.exe", "powershell", "/bin/bash", "/bin/sh",
	}
	
	pathLower := strings.ToLower(path)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(pathLower, strings.ToLower(pattern)) {
			return true
		}
	}
	
	return false
}

// Custom validation functions

// validateRecipeName validates recipe names
func validateRecipeName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	
	// Check length
	if len(name) < 3 || len(name) > 100 {
		return false
	}
	
	// Check for valid characters (letters, numbers, spaces, basic punctuation)
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsSpace(r) &&
			r != '-' && r != '\'' && r != '(' && r != ')' && r != ',' && r != '.' {
			return false
		}
	}
	
	return true
}

// validateIngredient validates ingredient names
func validateIngredient(fl validator.FieldLevel) bool {
	ingredient := fl.Field().String()
	
	// Check length
	if len(ingredient) < 1 || len(ingredient) > 200 {
		return false
	}
	
	// Check for dangerous characters
	dangerous := []string{"<", ">", "script", "javascript:", "onload", "onerror"}
	ingredientLower := strings.ToLower(ingredient)
	for _, danger := range dangerous {
		if strings.Contains(ingredientLower, danger) {
			return false
		}
	}
	
	return true
}

// validateNoSQLInjection checks for SQL injection patterns
func validateNoSQLInjection(fl validator.FieldLevel) bool {
	value := strings.ToLower(fl.Field().String())
	
	sqlPatterns := []string{
		"'", "\"", ";", "--", "/*", "*/",
		"union", "select", "insert", "update", "delete", "drop",
		"exec", "execute", "xp_", "sp_", "@@",
		"or 1=1", "and 1=1", "' or '", "' and '",
		"1' or '1'='1", "admin'--",
	}
	
	for _, pattern := range sqlPatterns {
		if strings.Contains(value, pattern) {
			return false
		}
	}
	
	return true
}

// validateNoXSS checks for XSS patterns
func validateNoXSS(fl validator.FieldLevel) bool {
	value := strings.ToLower(fl.Field().String())
	
	xssPatterns := []string{
		"<script", "</script>", "javascript:", "vbscript:",
		"onload", "onerror", "onclick", "onmouseover", "onfocus",
		"onblur", "onchange", "onsubmit", "onreset", "onselect",
		"onkeydown", "onkeypress", "onkeyup",
		"eval(", "alert(", "confirm(", "prompt(",
		"document.cookie", "document.write", "window.location",
	}
	
	for _, pattern := range xssPatterns {
		if strings.Contains(value, pattern) {
			return false
		}
	}
	
	return true
}

// validateSafeFilename validates filenames for safety
func validateSafeFilename(fl validator.FieldLevel) bool {
	filename := fl.Field().String()
	
	// Check length
	if len(filename) < 1 || len(filename) > 255 {
		return false
	}
	
	// Check for dangerous patterns
	dangerous := []string{
		"../", "..\\", "\\", "/", ":", "*", "?", "\"", "<", ">", "|",
		"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4",
		"COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2",
		"LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	}
	
	filenameUpper := strings.ToUpper(filename)
	for _, danger := range dangerous {
		if strings.Contains(filenameUpper, danger) {
			return false
		}
	}
	
	return true
}

// validateSafeHTML validates HTML content for safety
func validateSafeHTML(fl validator.FieldLevel) bool {
	html := strings.ToLower(fl.Field().String())
	
	// Check for dangerous HTML elements
	dangerous := []string{
		"<script", "<object", "<embed", "<link", "<style", "<iframe",
		"<frame", "<frameset", "<meta", "<base", "<form", "<input",
		"<textarea", "<button", "<select", "<option", "<applet",
		"javascript:", "vbscript:", "data:", "onload", "onerror",
	}
	
	for _, danger := range dangerous {
		if strings.Contains(html, danger) {
			return false
		}
	}
	
	return true
}

// validateStrongPassword validates password strength
func validateStrongPassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	
	// Minimum length
	if len(password) < 8 {
		return false
	}
	
	// Check for required character types
	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false
	
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}
	
	// Require at least 3 of 4 character types
	typeCount := 0
	if hasUpper {
		typeCount++
	}
	if hasLower {
		typeCount++
	}
	if hasNumber {
		typeCount++
	}
	if hasSpecial {
		typeCount++
	}
	
	return typeCount >= 3
}

// ValidateStruct validates a struct using the validation rules
func (v *ValidationService) ValidateStruct(s interface{}) error {
	return v.validator.Struct(s)
}

// GetValidationError formats validation errors for API responses
func (v *ValidationService) GetValidationError(err error) map[string]string {
	errors := make(map[string]string)
	
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			field := e.Field()
			tag := e.Tag()
			
			switch tag {
			case "required":
				errors[field] = fmt.Sprintf("%s is required", field)
			case "email":
				errors[field] = fmt.Sprintf("%s must be a valid email", field)
			case "min":
				errors[field] = fmt.Sprintf("%s must be at least %s characters", field, e.Param())
			case "max":
				errors[field] = fmt.Sprintf("%s must be at most %s characters", field, e.Param())
			case "recipe_name":
				errors[field] = "Recipe name must be 3-100 characters with valid characters only"
			case "ingredient":
				errors[field] = "Invalid ingredient name"
			case "no_sql_injection":
				errors[field] = "Input contains potential SQL injection"
			case "no_xss":
				errors[field] = "Input contains potential XSS"
			case "safe_filename":
				errors[field] = "Invalid filename"
			case "safe_html":
				errors[field] = "HTML content contains unsafe elements"
			case "strong_password":
				errors[field] = "Password must be at least 8 characters with 3 different character types"
			default:
				errors[field] = fmt.Sprintf("%s is invalid", field)
			}
		}
	}
	
	return errors
}