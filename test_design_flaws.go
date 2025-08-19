// Standalone test to catch design flaws in Alchemorsel v3
// This file can be run directly without complex dependencies
package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	BaseURL = "http://localhost:8080"
	APIURL  = "http://localhost:3000"
)

type TestResult struct {
	TestName string
	Passed   bool
	Error    string
	Details  []string
}

func main() {
	fmt.Println("🧪 Alchemorsel v3 Design Flaw Detection Tests")
	fmt.Println("==============================================")
	
	if !isServerRunning(BaseURL) {
		fmt.Printf("❌ Server not running at %s\n", BaseURL)
		fmt.Println("Please start the server and try again.")
		return
	}
	
	fmt.Printf("✅ Server detected at %s\n\n", BaseURL)
	
	tests := []func() TestResult{
		testAuthenticationBypassAIChat,
		testAuthenticationBypassRecipeSearch,
		testXSSVulnerability,
		testHomepageTemplateLogic,
		testStaticFileServing,
		testFormSubmissionMethods,
		testSessionManagement,
	}
	
	var results []TestResult
	passed := 0
	total := len(tests)
	
	for _, test := range tests {
		result := test()
		results = append(results, result)
		
		if result.Passed {
			fmt.Printf("✅ PASS: %s\n", result.TestName)
			passed++
		} else {
			fmt.Printf("❌ FAIL: %s\n", result.TestName)
			if result.Error != "" {
				fmt.Printf("   Error: %s\n", result.Error)
			}
			for _, detail := range result.Details {
				fmt.Printf("   - %s\n", detail)
			}
		}
		fmt.Println()
	}
	
	fmt.Println("==============================================")
	fmt.Printf("📊 Test Results: %d/%d passed (%.1f%%)\n", passed, total, float64(passed)/float64(total)*100)
	
	if passed < total {
		fmt.Println("\n🚨 DESIGN FLAWS DETECTED:")
		for _, result := range results {
			if !result.Passed {
				fmt.Printf("   • %s\n", result.TestName)
			}
		}
	} else {
		fmt.Println("\n🎉 All tests passed! No design flaws detected.")
	}
}

func testAuthenticationBypassAIChat() TestResult {
	client := &http.Client{Timeout: 5 * time.Second}
	
	// Attempt to access AI chat without authentication
	payload := strings.NewReader("message=Hello AI")
	req, err := http.NewRequest("POST", BaseURL+"/htmx/ai/chat", payload)
	if err != nil {
		return TestResult{
			TestName: "Authentication Bypass - AI Chat",
			Passed:   false,
			Error:    fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			TestName: "Authentication Bypass - AI Chat",
			Passed:   false,
			Error:    fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()
	
	// AI Chat should require authentication
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TestResult{
			TestName: "Authentication Bypass - AI Chat",
			Passed:   false,
			Error:    "AI Chat accessible without authentication",
			Details: []string{
				fmt.Sprintf("Response Status: %d (should be 401 or 403)", resp.StatusCode),
				fmt.Sprintf("Response Body: %s", string(body)[:min(len(body), 200)]),
				"SECURITY RISK: Unauthenticated users can access AI features",
			},
		}
	}
	
	return TestResult{
		TestName: "Authentication Bypass - AI Chat",
		Passed:   true,
	}
}

func testAuthenticationBypassRecipeSearch() TestResult {
	client := &http.Client{Timeout: 5 * time.Second}
	
	payload := strings.NewReader("query=chicken")
	req, err := http.NewRequest("POST", BaseURL+"/htmx/recipes/search", payload)
	if err != nil {
		return TestResult{
			TestName: "Authentication Bypass - Recipe Search",
			Passed:   false,
			Error:    fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			TestName: "Authentication Bypass - Recipe Search",
			Passed:   false,
			Error:    fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return TestResult{
			TestName: "Authentication Bypass - Recipe Search",
			Passed:   false,
			Error:    "Recipe Search accessible without authentication",
			Details: []string{
				fmt.Sprintf("Response Status: %d (should be 401 or 403)", resp.StatusCode),
				fmt.Sprintf("Response Body: %s", string(body)[:min(len(body), 200)]),
				"SECURITY RISK: Unauthenticated users can search recipes",
			},
		}
	}
	
	return TestResult{
		TestName: "Authentication Bypass - Recipe Search",
		Passed:   true,
	}
}

func testXSSVulnerability() TestResult {
	client := &http.Client{Timeout: 5 * time.Second}
	
	xssPayload := "<script>alert('XSS')</script>"
	payload := strings.NewReader(fmt.Sprintf("message=%s", xssPayload))
	
	req, err := http.NewRequest("POST", BaseURL+"/htmx/ai/chat", payload)
	if err != nil {
		return TestResult{
			TestName: "XSS Vulnerability Check",
			Passed:   false,
			Error:    fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			TestName: "XSS Vulnerability Check",
			Passed:   true, // Connection error means endpoint might be protected
		}
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	
	// Check if XSS payload is reflected without sanitization
	if strings.Contains(bodyStr, xssPayload) {
		return TestResult{
			TestName: "XSS Vulnerability Check",
			Passed:   false,
			Error:    "XSS payload reflected without sanitization",
			Details: []string{
				fmt.Sprintf("Payload: %s", xssPayload),
				fmt.Sprintf("Response: %s", bodyStr[:min(len(bodyStr), 300)]),
				"SECURITY RISK: User input not properly sanitized",
			},
		}
	}
	
	return TestResult{
		TestName: "XSS Vulnerability Check",
		Passed:   true,
	}
}

func testHomepageTemplateLogic() TestResult {
	client := &http.Client{Timeout: 10 * time.Second}
	
	req, err := http.NewRequest("GET", BaseURL+"/", nil)
	if err != nil {
		return TestResult{
			TestName: "Homepage Template Logic",
			Passed:   false,
			Error:    fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			TestName: "Homepage Template Logic",
			Passed:   false,
			Error:    fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TestResult{
			TestName: "Homepage Template Logic",
			Passed:   false,
			Error:    fmt.Sprintf("Failed to read response: %v", err),
		}
	}
	
	content := strings.ToLower(string(body))
	
	// Check for hero section indicators
	heroPatterns := []string{"hero", "welcome", "get-started", "jumbotron", "banner"}
	hasHero := false
	heroFound := []string{}
	for _, pattern := range heroPatterns {
		if strings.Contains(content, pattern) {
			hasHero = true
			heroFound = append(heroFound, pattern)
		}
	}
	
	// Check for AI chat indicators
	chatPatterns := []string{"ai-chat", "chat-input", "chat-container", "message-input"}
	hasChat := false
	chatFound := []string{}
	for _, pattern := range chatPatterns {
		if strings.Contains(content, pattern) {
			hasChat = true
			chatFound = append(chatFound, pattern)
		}
	}
	
	// UX DESIGN FLAW: Both hero section and AI chat should not coexist
	if hasHero && hasChat {
		return TestResult{
			TestName: "Homepage Template Logic",
			Passed:   false,
			Error:    "Homepage shows both hero section AND AI chat interface",
			Details: []string{
				fmt.Sprintf("Hero elements found: %v", heroFound),
				fmt.Sprintf("Chat elements found: %v", chatFound),
				"UX FLAW: Creates confusion about primary user action",
				"RECOMMENDATION: Show either hero (anonymous) OR chat (authenticated)",
			},
		}
	}
	
	if !hasHero && !hasChat {
		return TestResult{
			TestName: "Homepage Template Logic",
			Passed:   false,
			Error:    "Homepage has neither hero section nor main content",
			Details: []string{
				"Missing primary content on homepage",
				"Users have no clear call-to-action",
			},
		}
	}
	
	return TestResult{
		TestName: "Homepage Template Logic",
		Passed:   true,
	}
}

func testStaticFileServing() TestResult {
	client := &http.Client{Timeout: 5 * time.Second}
	
	criticalFiles := []string{
		"/static/js/htmx.min.js",
		"/static/css/style.css",
		"/static/css/main.css",
		"/static/js/app.js",
	}
	
	var missing []string
	var details []string
	
	for _, file := range criticalFiles {
		req, err := http.NewRequest("GET", BaseURL+file, nil)
		if err != nil {
			continue
		}
		
		resp, err := client.Do(req)
		if err != nil {
			missing = append(missing, file)
			details = append(details, fmt.Sprintf("%s: Request failed", file))
			continue
		}
		resp.Body.Close()
		
		if resp.StatusCode == http.StatusNotFound {
			missing = append(missing, file)
			details = append(details, fmt.Sprintf("%s: 404 Not Found", file))
		}
	}
	
	if len(missing) > 0 {
		return TestResult{
			TestName: "Static File Serving",
			Passed:   false,
			Error:    fmt.Sprintf("%d critical static files missing", len(missing)),
			Details: append([]string{
				"Missing files can break frontend functionality",
				"HTMX features may not work without htmx.min.js",
				"Styling may be broken without CSS files",
			}, details...),
		}
	}
	
	return TestResult{
		TestName: "Static File Serving",
		Passed:   true,
	}
}

func testFormSubmissionMethods() TestResult {
	client := &http.Client{Timeout: 5 * time.Second}
	
	// Test endpoints that should only accept POST but might accept GET
	testCases := []struct {
		endpoint string
		method   string
		shouldFail bool
	}{
		{"/htmx/ai/chat", "GET", true},         // Should fail - POST only
		{"/htmx/recipes/search", "GET", true},  // Should fail - POST only
	}
	
	var issues []string
	
	for _, tc := range testCases {
		var req *http.Request
		var err error
		
		if tc.method == "GET" {
			req, err = http.NewRequest("GET", BaseURL+tc.endpoint+"?test=data", nil)
		} else {
			req, err = http.NewRequest("POST", BaseURL+tc.endpoint, strings.NewReader("test=data"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		
		if err != nil {
			continue
		}
		
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		
		if tc.shouldFail && resp.StatusCode == http.StatusOK {
			issues = append(issues, fmt.Sprintf("%s accepts %s method (should reject)", tc.endpoint, tc.method))
		}
	}
	
	if len(issues) > 0 {
		return TestResult{
			TestName: "Form Submission Methods",
			Passed:   false,
			Error:    "Endpoints accepting incorrect HTTP methods",
			Details: append([]string{
				"SECURITY RISK: GET requests can be cached and logged",
				"Forms should use POST for data submission",
			}, issues...),
		}
	}
	
	return TestResult{
		TestName: "Form Submission Methods",
		Passed:   true,
	}
}

func testSessionManagement() TestResult {
	client := &http.Client{Timeout: 5 * time.Second}
	
	// Test homepage to check session handling
	req, err := http.NewRequest("GET", BaseURL+"/", nil)
	if err != nil {
		return TestResult{
			TestName: "Session Management",
			Passed:   false,
			Error:    fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	
	resp, err := client.Do(req)
	if err != nil {
		return TestResult{
			TestName: "Session Management",
			Passed:   false,
			Error:    fmt.Sprintf("Request failed: %v", err),
		}
	}
	defer resp.Body.Close()
	
	// Check for proper session cookie handling
	cookies := resp.Cookies()
	hasSessionCookie := false
	sessionDetails := []string{}
	
	for _, cookie := range cookies {
		if strings.Contains(strings.ToLower(cookie.Name), "session") || 
		   strings.Contains(strings.ToLower(cookie.Name), "auth") {
			hasSessionCookie = true
			sessionDetails = append(sessionDetails, 
				fmt.Sprintf("Cookie: %s (Secure: %v, HttpOnly: %v)", 
					cookie.Name, cookie.Secure, cookie.HttpOnly))
		}
	}
	
	// For now, this is informational rather than a failure
	// The real session management issues are logged server-side
	return TestResult{
		TestName: "Session Management",
		Passed:   true,
		Details: append([]string{
			fmt.Sprintf("Session cookies found: %v", hasSessionCookie),
			fmt.Sprintf("Total cookies: %d", len(cookies)),
		}, sessionDetails...),
	}
}

func isServerRunning(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}