// Package security provides tests to catch authentication bypass vulnerabilities
//go:build security
// +build security

package security

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Test against the running server
	BaseURL = "http://localhost:8080"
	APIURL  = "http://localhost:3000"
)

// TestAuthenticationBypass tests for authentication bypass vulnerabilities
// These tests are designed to catch the specific flaws observed in the logs
func TestAuthenticationBypass(t *testing.T) {
	// Skip if server is not running
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("AIChatShouldRequireAuthentication", func(t *testing.T) {
		// This test should FAIL if there's an authentication bypass
		client := &http.Client{Timeout: 5 * time.Second}
		
		// Attempt to access AI chat without authentication
		payload := strings.NewReader("message=Hello AI")
		req, err := http.NewRequest("POST", BaseURL+"/htmx/ai/chat", payload)
		require.NoError(t, err)
		
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		
		// SECURITY FLAW: This should return 401/403, not 200
		if resp.StatusCode == http.StatusOK {
			t.Errorf("SECURITY VULNERABILITY: AI Chat accessible without authentication!")
			t.Errorf("Response Status: %d (should be 401 or 403)", resp.StatusCode)
			t.Errorf("Response Body: %s", string(body))
		}
		
		// Additional checks for proper authentication handling
		assert.NotEqual(t, http.StatusOK, resp.StatusCode, 
			"AI Chat should require authentication")
		assert.Contains(t, []int{http.StatusUnauthorized, http.StatusForbidden}, 
			resp.StatusCode, "Should return 401 or 403 for unauthenticated access")
	})

	t.Run("RecipeSearchShouldRequireAuthentication", func(t *testing.T) {
		// This test should FAIL if there's an authentication bypass
		client := &http.Client{Timeout: 5 * time.Second}
		
		// Attempt to search recipes without authentication
		payload := strings.NewReader("query=chicken")
		req, err := http.NewRequest("POST", BaseURL+"/htmx/recipes/search", payload)
		require.NoError(t, err)
		
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		
		// SECURITY FLAW: This should return 401/403, not 200
		if resp.StatusCode == http.StatusOK {
			t.Errorf("SECURITY VULNERABILITY: Recipe Search accessible without authentication!")
			t.Errorf("Response Status: %d (should be 401 or 403)", resp.StatusCode)
			t.Errorf("Response Body: %s", string(body))
		}
		
		assert.NotEqual(t, http.StatusOK, resp.StatusCode, 
			"Recipe Search should require authentication")
	})

	t.Run("ProtectedHTMXEndpointsShouldRequireAuth", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		
		protectedEndpoints := []struct {
			method string
			path   string
			body   string
		}{
			{"POST", "/htmx/ai/chat", "message=test"},
			{"POST", "/htmx/recipes/search", "query=test"},
			{"GET", "/recipes", ""},
			{"GET", "/ai/chat", ""},
		}
		
		for _, endpoint := range protectedEndpoints {
			t.Run(fmt.Sprintf("%s_%s", endpoint.method, endpoint.path), func(t *testing.T) {
				var req *http.Request
				var err error
				
				if endpoint.body != "" {
					req, err = http.NewRequest(endpoint.method, BaseURL+endpoint.path, 
						strings.NewReader(endpoint.body))
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				} else {
					req, err = http.NewRequest(endpoint.method, BaseURL+endpoint.path, nil)
				}
				require.NoError(t, err)
				
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				// Check for authentication bypass
				if resp.StatusCode == http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("SECURITY VULNERABILITY: %s %s accessible without auth!", 
						endpoint.method, endpoint.path)
					t.Errorf("Response Status: %d", resp.StatusCode)
					t.Errorf("Response Body: %s", string(body))
				}
			})
		}
	})

	t.Run("XSSProtectionInAIChat", func(t *testing.T) {
		// Test XSS protection as observed in the logs
		client := &http.Client{Timeout: 5 * time.Second}
		
		xssPayloads := []string{
			"<script>alert('XSS')</script>",
			"<img src=x onerror=alert('XSS')>",
			"javascript:alert('XSS')",
			"<svg onload=alert('XSS')>",
		}
		
		for _, payload := range xssPayloads {
			t.Run(fmt.Sprintf("XSS_%s", payload[:10]), func(t *testing.T) {
				reqBody := fmt.Sprintf("message=%s", payload)
				req, err := http.NewRequest("POST", BaseURL+"/htmx/ai/chat", 
					strings.NewReader(reqBody))
				require.NoError(t, err)
				
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("HX-Request", "true")
				
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				
				// Check if XSS payload is reflected without sanitization
				if strings.Contains(string(body), payload) {
					t.Errorf("XSS VULNERABILITY: Payload reflected unsanitized!")
					t.Errorf("Payload: %s", payload)
					t.Errorf("Response: %s", string(body))
				}
				
				// Response should either be blocked or sanitized
				bodyStr := string(body)
				assert.False(t, strings.Contains(bodyStr, "<script>"), 
					"Script tags should be sanitized")
				assert.False(t, strings.Contains(bodyStr, "javascript:"), 
					"JavaScript URLs should be sanitized")
			})
		}
	})
}

// TestSessionManagement tests session management issues
func TestSessionManagement(t *testing.T) {
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("SessionErrorsInLogs", func(t *testing.T) {
		// This test documents the "Failed to get session" errors
		client := &http.Client{Timeout: 5 * time.Second}
		
		// Make request that triggers session check
		req, err := http.NewRequest("GET", BaseURL+"/", nil)
		require.NoError(t, err)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// The homepage should load even without session
		assert.Equal(t, http.StatusOK, resp.StatusCode, 
			"Homepage should load without session")
		
		// But the server should handle missing sessions gracefully
		// (This test documents the current behavior - the actual fix
		// would be in the session middleware)
	})

	t.Run("CookieHandling", func(t *testing.T) {
		client := &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		}
		
		// Test homepage cookie handling
		req, err := http.NewRequest("GET", BaseURL+"/", nil)
		require.NoError(t, err)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		// Check if any cookies are set
		cookies := resp.Cookies()
		t.Logf("Cookies set: %d", len(cookies))
		for _, cookie := range cookies {
			t.Logf("Cookie: %s=%s (Secure: %v, HttpOnly: %v)", 
				cookie.Name, cookie.Value, cookie.Secure, cookie.HttpOnly)
		}
	})
}

// TestFormSubmissionBehavior tests form submission methods
func TestFormSubmissionBehavior(t *testing.T) {
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("FormMethodValidation", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		
		// Test forms that should use POST but might be submitted as GET
		testCases := []struct {
			endpoint string
			method   string
			shouldFail bool
		}{
			{"/htmx/ai/chat", "GET", true},   // Should fail - POST only
			{"/htmx/ai/chat", "POST", false}, // Should work (if authenticated)
			{"/htmx/recipes/search", "GET", true},   // Should fail - POST only
			{"/htmx/recipes/search", "POST", false}, // Should work (if authenticated)
		}
		
		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s_%s", tc.method, tc.endpoint), func(t *testing.T) {
				var req *http.Request
				var err error
				
				if tc.method == "POST" {
					req, err = http.NewRequest("POST", BaseURL+tc.endpoint, 
						strings.NewReader("test=data"))
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				} else {
					req, err = http.NewRequest("GET", BaseURL+tc.endpoint+"?test=data", nil)
				}
				require.NoError(t, err)
				
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				if tc.shouldFail {
					assert.NotEqual(t, http.StatusOK, resp.StatusCode, 
						"%s %s should not accept %s method", tc.method, tc.endpoint, tc.method)
					assert.Contains(t, []int{
						http.StatusMethodNotAllowed, 
						http.StatusBadRequest,
						http.StatusUnauthorized,
						http.StatusForbidden,
					}, resp.StatusCode, "Should return appropriate error status")
				}
			})
		}
	})
}

// TestStaticFileServing tests static file availability
func TestStaticFileServing(t *testing.T) {
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("StaticFileAvailability", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		
		// Test critical static files mentioned in the issue
		staticFiles := []string{
			"/static/js/htmx.min.js",
			"/static/css/style.css",
			"/static/css/main.css",
			"/static/js/app.js",
		}
		
		for _, file := range staticFiles {
			t.Run(file, func(t *testing.T) {
				req, err := http.NewRequest("GET", BaseURL+file, nil)
				require.NoError(t, err)
				
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				if resp.StatusCode == http.StatusNotFound {
					t.Errorf("STATIC FILE MISSING: %s returns 404", file)
					t.Errorf("This could break frontend functionality")
				}
				
				// Log the status for analysis
				t.Logf("Static file %s: Status %d", file, resp.StatusCode)
			})
		}
	})
}

// TestTemplateLogic tests template rendering issues
func TestTemplateLogic(t *testing.T) {
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("HomepageTemplateLogic", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		
		// Test homepage template rendering
		req, err := http.NewRequest("GET", BaseURL+"/", nil)
		require.NoError(t, err)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		
		bodyStr := string(body)
		
		// Check for the UX design flaw: both hero section AND AI chat showing
		hasHeroSection := strings.Contains(bodyStr, "hero") || 
			strings.Contains(bodyStr, "welcome") ||
			strings.Contains(bodyStr, "get-started")
		
		hasAIChat := strings.Contains(bodyStr, "ai-chat") || 
			strings.Contains(bodyStr, "chat-input") ||
			strings.Contains(bodyStr, "message")
		
		if hasHeroSection && hasAIChat {
			t.Errorf("UX DESIGN FLAW: Homepage shows both hero section AND AI chat!")
			t.Errorf("This creates a confusing user experience")
			t.Logf("Page length: %d characters", len(bodyStr))
			
			// Extract snippets showing both elements
			if strings.Contains(bodyStr, "hero") {
				t.Logf("Hero section found in template")
			}
			if strings.Contains(bodyStr, "chat") {
				t.Logf("Chat section found in template")
			}
		}
		
		// The page should show either hero (for anonymous) OR chat (for authenticated)
		// but not both simultaneously
	})

	t.Run("AuthenticationStateInTemplate", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		
		// Test homepage without authentication
		req, err := http.NewRequest("GET", BaseURL+"/", nil)
		require.NoError(t, err)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		
		bodyStr := string(body)
		
		// For unauthenticated users, should show login/register options
		hasLoginOption := strings.Contains(bodyStr, "login") || 
			strings.Contains(bodyStr, "sign-in")
		
		hasRegisterOption := strings.Contains(bodyStr, "register") || 
			strings.Contains(bodyStr, "sign-up")
		
		// Should not show authenticated features
		hasLogoutOption := strings.Contains(bodyStr, "logout") || 
			strings.Contains(bodyStr, "sign-out")
		
		hasUserProfile := strings.Contains(bodyStr, "profile") || 
			strings.Contains(bodyStr, "dashboard")
		
		t.Logf("Login option: %v", hasLoginOption)
		t.Logf("Register option: %v", hasRegisterOption)
		t.Logf("Logout option: %v", hasLogoutOption)
		t.Logf("User profile: %v", hasUserProfile)
		
		// For anonymous users, should not show authenticated features
		if hasLogoutOption || hasUserProfile {
			t.Errorf("TEMPLATE LOGIC ERROR: Anonymous user sees authenticated features")
		}
	})
}

// Helper function to check if server is running
func isServerRunning(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}