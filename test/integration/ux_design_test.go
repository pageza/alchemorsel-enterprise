// Package integration provides tests to catch UX design flaws
//go:build integration
// +build integration

package integration

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	BaseURL = "http://localhost:8080"
)

// TestHomepageUXDesign tests for UX design flaws on the homepage
func TestHomepageUXDesign(t *testing.T) {
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("HomepageTemplateConsistency", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		
		// Get homepage content
		req, err := http.NewRequest("GET", BaseURL+"/", nil)
		require.NoError(t, err)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		
		content := string(body)
		
		// Analyze the page structure
		analysis := analyzePageStructure(content)
		
		t.Logf("Page Analysis:")
		t.Logf("- Has Hero Section: %v", analysis.HasHeroSection)
		t.Logf("- Has AI Chat: %v", analysis.HasAIChat)
		t.Logf("- Has Login Form: %v", analysis.HasLoginForm)
		t.Logf("- Has Navigation: %v", analysis.HasNavigation)
		t.Logf("- Content Length: %d characters", len(content))
		t.Logf("- Number of Forms: %d", analysis.FormCount)
		t.Logf("- Number of Scripts: %d", analysis.ScriptCount)
		
		// UX DESIGN FLAW CHECK: Hero section and AI chat should not coexist
		if analysis.HasHeroSection && analysis.HasAIChat {
			t.Errorf("UX DESIGN FLAW: Homepage shows both hero section AND AI chat interface!")
			t.Errorf("This creates confusion about the primary action users should take")
			t.Errorf("Expected: Either hero section (for new users) OR chat interface (for authenticated users)")
			
			// Provide specific evidence
			if analysis.HeroElements != nil {
				t.Errorf("Hero elements found: %v", analysis.HeroElements)
			}
			if analysis.ChatElements != nil {
				t.Errorf("Chat elements found: %v", analysis.ChatElements)
			}
		}
		
		// Additional UX checks
		if analysis.HasAIChat && !analysis.IsAuthenticated {
			t.Errorf("UX LOGIC ERROR: AI Chat visible to unauthenticated users")
		}
		
		if !analysis.HasHeroSection && !analysis.HasAIChat {
			t.Errorf("UX MISSING CONTENT: Homepage has neither hero section nor main content")
		}
	})

	t.Run("ResponsiveDesignElements", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		
		req, err := http.NewRequest("GET", BaseURL+"/", nil)
		require.NoError(t, err)
		
		// Simulate mobile user agent
		req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15")
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		
		content := string(body)
		
		// Check for responsive design indicators
		hasViewport := strings.Contains(content, "viewport")
		hasResponsiveCSS := strings.Contains(content, "media") || 
			strings.Contains(content, "@media") ||
			strings.Contains(content, "responsive")
		
		if !hasViewport {
			t.Errorf("RESPONSIVE DESIGN ISSUE: Missing viewport meta tag")
		}
		
		t.Logf("Has viewport meta: %v", hasViewport)
		t.Logf("Has responsive CSS: %v", hasResponsiveCSS)
	})

	t.Run("AccessibilityChecks", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		
		req, err := http.NewRequest("GET", BaseURL+"/", nil)
		require.NoError(t, err)
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		
		content := string(body)
		
		accessibilityIssues := checkAccessibility(content)
		
		for _, issue := range accessibilityIssues {
			t.Errorf("ACCESSIBILITY ISSUE: %s", issue)
		}
		
		if len(accessibilityIssues) == 0 {
			t.Logf("No obvious accessibility issues found")
		}
	})
}

// TestNavigationConsistency tests navigation and routing
func TestNavigationConsistency(t *testing.T) {
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("NavigationLinksWork", func(t *testing.T) {
		client := &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
		}
		
		// Test common navigation links
		links := []string{
			"/",
			"/recipes",
			"/login",
			"/register",
			"/ai/chat",
		}
		
		for _, link := range links {
			t.Run(link, func(t *testing.T) {
				req, err := http.NewRequest("GET", BaseURL+link, nil)
				require.NoError(t, err)
				
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				// Check for navigation consistency
				if resp.StatusCode >= 500 {
					t.Errorf("NAVIGATION ERROR: %s returns server error (%d)", link, resp.StatusCode)
				}
				
				// Protected routes should redirect or return 401/403
				if strings.Contains(link, "/ai/") || strings.Contains(link, "/recipes") {
					if resp.StatusCode == http.StatusOK {
						t.Errorf("AUTHENTICATION BYPASS: Protected route %s accessible without auth", link)
					}
				}
				
				t.Logf("Link %s: Status %d", link, resp.StatusCode)
			})
		}
	})

	t.Run("ConsistentNavigationAcrossPages", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		
		pages := []string{"/", "/login", "/register"}
		
		for _, page := range pages {
			t.Run(page, func(t *testing.T) {
				req, err := http.NewRequest("GET", BaseURL+page, nil)
				require.NoError(t, err)
				
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					t.Skip("Page not accessible:", resp.StatusCode)
				}
				
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				
				content := string(body)
				
				// Check for consistent navigation elements
				hasNavigation := strings.Contains(content, "<nav") || 
					strings.Contains(content, "navigation") ||
					strings.Contains(content, "navbar")
				
				hasLogo := strings.Contains(content, "logo") || 
					strings.Contains(content, "brand")
				
				t.Logf("Page %s - Navigation: %v, Logo: %v", page, hasNavigation, hasLogo)
			})
		}
	})
}

// TestFormUXBehavior tests form user experience
func TestFormUXBehavior(t *testing.T) {
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("FormValidationAndFeedback", func(t *testing.T) {
		client := &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		
		// Test form pages
		formPages := []string{"/login", "/register"}
		
		for _, page := range formPages {
			t.Run(page, func(t *testing.T) {
				req, err := http.NewRequest("GET", BaseURL+page, nil)
				require.NoError(t, err)
				
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				if resp.StatusCode != http.StatusOK {
					t.Skip("Form page not accessible:", resp.StatusCode)
				}
				
				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				
				content := string(body)
				
				formIssues := analyzeFormUX(content)
				
				for _, issue := range formIssues {
					t.Errorf("FORM UX ISSUE on %s: %s", page, issue)
				}
			})
		}
	})

	t.Run("HTMXFormSubmissionBehavior", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		
		// Test HTMX form submission - this should fail due to auth bypass
		testData := []struct {
			endpoint string
			payload  string
		}{
			{"/htmx/ai/chat", "message=test"},
			{"/htmx/recipes/search", "query=test"},
		}
		
		for _, test := range testData {
			t.Run(test.endpoint, func(t *testing.T) {
				req, err := http.NewRequest("POST", BaseURL+test.endpoint, 
					strings.NewReader(test.payload))
				require.NoError(t, err)
				
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.Header.Set("HX-Request", "true")
				req.Header.Set("HX-Target", "#content")
				
				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				
				// These should fail with proper authentication
				if resp.StatusCode == http.StatusOK {
					body, _ := io.ReadAll(resp.Body)
					t.Errorf("HTMX ENDPOINT ACCESSIBLE WITHOUT AUTH: %s", test.endpoint)
					t.Errorf("Response: %s", string(body))
				}
			})
		}
	})
}

// TestMobileUXExperience tests mobile-specific UX
func TestMobileUXExperience(t *testing.T) {
	if !isServerRunning(BaseURL) {
		t.Skip("Server not running at", BaseURL)
	}

	t.Run("MobileViewportAndScaling", func(t *testing.T) {
		client := &http.Client{Timeout: 10 * time.Second}
		
		req, err := http.NewRequest("GET", BaseURL+"/", nil)
		require.NoError(t, err)
		
		// Mobile user agent
		req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X)")
		
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		
		content := string(body)
		
		// Check viewport configuration
		viewportRegex := regexp.MustCompile(`<meta\s+name=["']viewport["'][^>]*>`)
		viewportTags := viewportRegex.FindAllString(content, -1)
		
		if len(viewportTags) == 0 {
			t.Errorf("MOBILE UX ISSUE: No viewport meta tag found")
		} else {
			for _, tag := range viewportTags {
				t.Logf("Viewport tag: %s", tag)
				
				// Check for proper mobile settings
				if !strings.Contains(tag, "width=device-width") {
					t.Errorf("MOBILE UX ISSUE: Viewport should include width=device-width")
				}
			}
		}
	})
}

// PageStructureAnalysis holds page structure analysis results
type PageStructureAnalysis struct {
	HasHeroSection  bool
	HasAIChat       bool
	HasLoginForm    bool
	HasNavigation   bool
	IsAuthenticated bool
	FormCount       int
	ScriptCount     int
	HeroElements    []string
	ChatElements    []string
}

// analyzePageStructure analyzes the structure of a page
func analyzePageStructure(content string) PageStructureAnalysis {
	analysis := PageStructureAnalysis{}
	
	// Check for hero section indicators
	heroPatterns := []string{
		"hero",
		"welcome",
		"get-started",
		"jumbotron",
		"banner",
		"landing",
	}
	
	for _, pattern := range heroPatterns {
		if strings.Contains(strings.ToLower(content), pattern) {
			analysis.HasHeroSection = true
			analysis.HeroElements = append(analysis.HeroElements, pattern)
		}
	}
	
	// Check for AI chat indicators
	chatPatterns := []string{
		"ai-chat",
		"chat-input",
		"chat-container",
		"message-input",
		"chat-box",
		"ai/chat",
	}
	
	for _, pattern := range chatPatterns {
		if strings.Contains(strings.ToLower(content), pattern) {
			analysis.HasAIChat = true
			analysis.ChatElements = append(analysis.ChatElements, pattern)
		}
	}
	
	// Check for login form
	analysis.HasLoginForm = strings.Contains(content, "login") || 
		strings.Contains(content, "sign-in")
	
	// Check for navigation
	analysis.HasNavigation = strings.Contains(content, "<nav") || 
		strings.Contains(content, "navigation")
	
	// Check authentication state
	analysis.IsAuthenticated = strings.Contains(content, "logout") || 
		strings.Contains(content, "dashboard") ||
		strings.Contains(content, "profile")
	
	// Count forms and scripts
	analysis.FormCount = strings.Count(content, "<form")
	analysis.ScriptCount = strings.Count(content, "<script")
	
	return analysis
}

// checkAccessibility performs basic accessibility checks
func checkAccessibility(content string) []string {
	var issues []string
	
	// Check for missing alt attributes on images
	imgRegex := regexp.MustCompile(`<img[^>]*>`)
	images := imgRegex.FindAllString(content, -1)
	for _, img := range images {
		if !strings.Contains(img, "alt=") {
			issues = append(issues, "Image missing alt attribute: "+img[:50]+"...")
		}
	}
	
	// Check for form labels
	inputRegex := regexp.MustCompile(`<input[^>]*>`)
	inputs := inputRegex.FindAllString(content, -1)
	for _, input := range inputs {
		if strings.Contains(input, `type="text"`) || 
		   strings.Contains(input, `type="email"`) || 
		   strings.Contains(input, `type="password"`) {
			// Should have associated label
			idMatch := regexp.MustCompile(`id=["']([^"']+)["']`).FindStringSubmatch(input)
			if len(idMatch) > 1 {
				labelPattern := fmt.Sprintf(`for=["']%s["']`, idMatch[1])
				if !strings.Contains(content, labelPattern) {
					issues = append(issues, "Input missing associated label: "+input[:50]+"...")
				}
			}
		}
	}
	
	// Check for heading structure
	if !strings.Contains(content, "<h1") {
		issues = append(issues, "Page missing main heading (h1)")
	}
	
	// Check for skip links
	if !strings.Contains(content, "skip") || !strings.Contains(content, "main") {
		issues = append(issues, "Page missing skip navigation link")
	}
	
	return issues
}

// analyzeFormUX analyzes form user experience
func analyzeFormUX(content string) []string {
	var issues []string
	
	// Check for form validation
	if strings.Contains(content, "<form") && !strings.Contains(content, "required") {
		issues = append(issues, "Forms missing required field validation")
	}
	
	// Check for password visibility toggle
	if strings.Contains(content, `type="password"`) && !strings.Contains(content, "show") {
		issues = append(issues, "Password field missing visibility toggle")
	}
	
	// Check for form error handling
	if !strings.Contains(content, "error") && !strings.Contains(content, "invalid") {
		issues = append(issues, "No error handling elements found")
	}
	
	// Check for loading states
	if strings.Contains(content, "submit") && !strings.Contains(content, "loading") {
		issues = append(issues, "No loading state indicators for form submission")
	}
	
	return issues
}

// isServerRunning checks if the server is running
func isServerRunning(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}