// Package webserver provides the web frontend HTTP server implementation
package webserver

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/performance"
	"github.com/alchemorsel/v3/pkg/healthcheck"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// WebServer represents the web frontend HTTP server
type WebServer struct {
	config         *config.Config
	logger         *zap.Logger
	server         *http.Server
	router         *chi.Mux
	apiClient      *APIClient
	sessionStore   *SessionStore
	templates      *template.Template
	healthCheck    *healthcheck.EnterpriseHealthCheck
	rateLimitStore *sync.Map // For rate limiting
	csrfSecret     []byte    // For CSRF protection
	// Performance monitoring
	perfMonitor    *performance.PerformanceMonitor
}

// NewWebServer creates a new web frontend server instance
func NewWebServer(
	cfg *config.Config,
	log *zap.Logger,
	apiClient *APIClient,
	sessionStore *SessionStore,
	healthCheck *healthcheck.EnterpriseHealthCheck,
) (*WebServer, error) {
	// Parse templates
	log.Info("Parsing templates...")
	templates, err := parseTemplates()
	if err != nil {
		log.Error("Failed to parse templates", zap.Error(err))
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	log.Info("Templates parsed successfully")

	// Initialize performance monitoring
	log.Info("Initializing performance monitoring...")
	perfMonitor := performance.NewPerformanceMonitor()
	log.Info("Performance monitoring initialized successfully")

	server := &WebServer{
		config:         cfg,
		logger:         log,
		apiClient:      apiClient,
		sessionStore:   sessionStore,
		templates:      templates,
		healthCheck:    healthCheck,
		rateLimitStore: &sync.Map{},
		csrfSecret:     []byte("secure-csrf-secret-key-32-chars"), // TODO: Generate from config
		perfMonitor:    perfMonitor,
	}

	server.router = server.setupRoutes()
	server.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      server.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server, nil
}

// setupRoutes configures the web frontend routes
func (s *WebServer) setupRoutes() *chi.Mux {
	r := chi.NewRouter()

	// SECURITY ENHANCEMENT: Enhanced middleware stack with security headers
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// Performance monitoring middleware
	r.Use(s.perfMonitor.Middleware())
	r.Use(s.securityHeadersMiddleware)
	r.Use(s.sessionMiddleware)
	r.Use(s.rateLimitMiddleware)

	// Static files - serve from embedded static subdirectory
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		s.logger.Error("Failed to create static sub-filesystem", zap.Error(err))
	} else {
		r.Mount("/static", http.StripPrefix("/static", http.FileServer(http.FS(staticSubFS))))
	}
	
	// Health check endpoints
	r.Get("/health", s.handleHealthCheck)
	r.Get("/ready", s.handleReadinessCheck)
	r.Get("/live", s.handleLivenessCheck)
	
	// Debug endpoint to check embedded files
	r.Get("/debug/static", s.handleDebugStatic)
	r.Get("/debug/htmx", s.handleDebugHTMX)

	// Public pages
	r.Get("/", s.handleHome)
	r.Get("/login", s.handleLoginPage)
	r.Post("/login", s.handleLogin)
	r.Get("/register", s.handleRegisterPage)
	r.Post("/register", s.handleRegister)
	r.Post("/logout", s.handleLogout)
	
	// Protected pages (require authentication)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		
		// Dashboard (now properly protected)
		r.Get("/dashboard", s.handleDashboard)
		
		// Recipe pages
		r.Get("/recipes", s.handleRecipeList)
		r.Get("/recipes/new", s.handleNewRecipePage)
		r.Post("/recipes", s.handleCreateRecipe)
		r.Get("/recipes/{id}", s.handleRecipeDetail)
		r.Get("/recipes/{id}/edit", s.handleEditRecipePage)
		r.Put("/recipes/{id}", s.handleUpdateRecipe)
		r.Delete("/recipes/{id}", s.handleDeleteRecipe)
		
		// AI features
		r.Get("/ai/chat", s.handleAIChatPage)
		r.Post("/ai/generate", s.handleAIGenerate)
		r.Post("/ai/suggest", s.handleAISuggest)
		
		// User profile
		r.Get("/profile", s.handleProfile)
		r.Put("/profile", s.handleUpdateProfile)
		r.Get("/favorites", s.handleFavorites)
	})

	// HTMX endpoints (partial templates) - ALL require authentication
	// CRITICAL SECURITY FIX ALV3-2025-001: Authentication required for all HTMX endpoints
	r.Route("/htmx", func(r chi.Router) {
		// CRITICAL: Require authentication for ALL HTMX endpoints
		r.Use(s.requireAuth)
		// CRITICAL SECURITY FIX ALV3-2025-003: Add CSRF protection
		r.Use(s.csrfMiddleware)
		// Input validation middleware for all HTMX endpoints
		r.Use(s.inputValidationMiddleware)
		
		r.Post("/search", s.handleHTMXSearch)
		r.Post("/recipes/{id}/like", s.handleHTMXLike)
		r.Post("/recipes/{id}/rate", s.handleHTMXRate)
		r.Get("/recipes/{id}/comments", s.handleHTMXComments)
		r.Post("/recipes/{id}/comments", s.handleHTMXAddComment)
		r.Get("/notifications", s.handleHTMXNotifications)
		
		// AI Chat endpoints - Now properly secured
		r.Post("/ai/chat", s.handleHTMXAIChat)
		r.Post("/recipes/search", s.handleHTMXRecipeSearch)
	})

	return r
}

// Start starts the web frontend HTTP server
func (s *WebServer) Start() error {
	s.logger.Info("Starting Web Frontend server",
		zap.String("address", s.server.Addr),
		zap.String("mode", "HTMX-templates"),
	)

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the web server
func (s *WebServer) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down Web Frontend server...")
	return s.server.Shutdown(ctx)
}

// parseTemplates parses all HTML templates from the embedded filesystem
func parseTemplates() (*template.Template, error) {
	// Template functions
	funcMap := template.FuncMap{
		"formatDate": func(t time.Time) string {
			return t.Format("Jan 2, 2006")
		},
		"formatTime": func(t time.Time) string {
			return t.Format("3:04 PM")
		},
		"timeAgo": func(t time.Time) string {
			duration := time.Since(t)
			if duration < time.Minute {
				return "just now"
			} else if duration < time.Hour {
				return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
			} else if duration < 24*time.Hour {
				return fmt.Sprintf("%d hours ago", int(duration.Hours()))
			} else if duration < 7*24*time.Hour {
				return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
			} else {
				return t.Format("Jan 2")
			}
		},
		"truncate": func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			return s[:n] + "..."
		},
		"title": func(s string) string {
			return strings.Title(strings.ToLower(s))
		},
		"trimPrefix": func(prefix, s string) string {
			return strings.TrimPrefix(s, prefix)
		},
		"urlQuery": func(s string) string {
			return url.QueryEscape(s)
		},
		"iterate": func(count int) []int {
			var items []int
			for i := 0; i < count; i++ {
				items = append(items, i)
			}
			return items
		},
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultValue
			}
			return value
		},
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"lt": func(a, b interface{}) bool {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					return av < bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					return av < bv
				}
			}
			return false
		},
		"gt": func(a, b interface{}) bool {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					return av > bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					return av > bv
				}
			}
			return false
		},
		"sub": func(a, b interface{}) interface{} {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					return av - bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					return av - bv
				}
			}
			return 0
		},
		"add": func(a, b interface{}) interface{} {
			switch av := a.(type) {
			case int:
				if bv, ok := b.(int); ok {
					return av + bv
				}
			case float64:
				if bv, ok := b.(float64); ok {
					return av + bv
				}
			}
			return 0
		},
		"join": func(sep string, elems []string) string {
			return strings.Join(elems, sep)
		},
		"contains": func(substr, str string) bool {
			return strings.Contains(str, substr)
		},
		"seq": func(start, end int) []int {
			var result []int
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
			return result
		},
	}

	// Parse all templates together from embedded filesystem
	// Walk through the embedded filesystem to find all .html files
	tmpl := template.New("").Funcs(funcMap)
	
	err := fs.WalkDir(templatesFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			content, err := templatesFS.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read template file %s: %w", path, err)
			}
			
			// Get template name from filename (remove .html extension)
			name := strings.TrimSuffix(filepath.Base(path), ".html")
			_, err = tmpl.New(name).Parse(string(content))
			if err != nil {
				return fmt.Errorf("failed to parse template %s: %w", name, err)
			}
		}
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to walk template directory: %w", err)
	}

	// Debug: Log template names that were loaded
	fmt.Printf("Loaded templates: ")
	for _, t := range tmpl.Templates() {
		fmt.Printf("%s ", t.Name())
	}
	fmt.Println()

	return tmpl, nil
}

// Middleware

func (s *WebServer) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Load session from cookie
		session, err := s.sessionStore.Get(r, "alchemorsel-session")
		if err != nil {
			s.logger.Debug("Failed to get session", zap.Error(err))
			// Create new session
			session = s.sessionStore.New("alchemorsel-session")
		}

		// Add session to context
		ctx := context.WithValue(r.Context(), "session", session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *WebServer) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value("session").(*Session)
		if session.UserID == "" || session.AccessToken == "" {
			// Check if this is an HTMX request
			if r.Header.Get("HX-Request") == "true" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`<div class="error">Authentication required. Please <a href="/login">login</a> to continue.</div>`))
				return
			}
			// Regular request - redirect to login
			http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
			return
		}

		// Verify token is still valid with API
		if !s.apiClient.VerifyToken(r.Context(), session.AccessToken) {
			// Token invalid, clear session
			session.Clear()
			session.Save(w)
			
			// Check if this is an HTMX request
			if r.Header.Get("HX-Request") == "true" {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`<div class="error">Session expired. Please <a href="/login">login</a> again.</div>`))
				return
			}
			// Regular request - redirect to login
			http.Redirect(w, r, "/login?error=session_expired", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handler functions

func (s *WebServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Determine check mode from query parameter
	mode := healthcheck.ModeStandard
	if modeParam := r.URL.Query().Get("mode"); modeParam != "" {
		switch modeParam {
		case "quick":
			mode = healthcheck.ModeQuick
		case "deep":
			mode = healthcheck.ModeDeep
		case "maintenance":
			mode = healthcheck.ModeMaintenance
		}
	}
	
	// Perform enterprise health check
	response := s.healthCheck.CheckWithMode(ctx, mode)
	
	// Determine HTTP status code
	statusCode := http.StatusOK
	if response.Status == healthcheck.StatusUnhealthy {
		statusCode = http.StatusServiceUnavailable
	} else if response.Status == healthcheck.StatusDegraded {
		statusCode = http.StatusPartialContent
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	// Use JSON encoding for enterprise response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode health check response", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *WebServer) handleReadinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	response := s.healthCheck.CheckWithMode(ctx, healthcheck.ModeStandard)
	
	// Service is ready only if all checks pass and API is accessible
	if response.Status != healthcheck.StatusHealthy {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not_ready",
			"reason": "Health checks failed",
			"checks": response.Checks,
		})
		return
	}
	
	// Also check if API is reachable
	if !s.apiClient.VerifyConnection(ctx) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "not_ready",
			"reason": "API backend not accessible",
		})
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now(),
	})
}

func (s *WebServer) handleLivenessCheck(w http.ResponseWriter, r *http.Request) {
	// Simple liveness check - if the handler responds, the service is alive
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now(),
	})
}

func (s *WebServer) handleDebugStatic(w http.ResponseWriter, r *http.Request) {
	// Debug endpoint to list embedded static files
	files := make([]string, 0)
	
	err := fs.WalkDir(staticFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		files = append(files, path)
		return nil
	})
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"embedded_files": files,
		"error":          err,
	})
}

func (s *WebServer) handleDebugHTMX(w http.ResponseWriter, r *http.Request) {
	// Serve HTMX file directly to test embedding
	data, err := fs.ReadFile(staticFS, "static/js/htmx.min.js")
	if err != nil {
		http.Error(w, "HTMX file not found: "+err.Error(), http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/javascript")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *WebServer) handleHome(w http.ResponseWriter, r *http.Request) {
	// Get session to check authentication state
	session := r.Context().Value("session").(*Session)
	
	// Determine if user is authenticated and token is valid
	var user interface{}
	isAuthenticated := false
	
	if session.UserID != "" && session.AccessToken != "" {
		// Verify token is still valid with API
		if s.apiClient.VerifyToken(r.Context(), session.AccessToken) {
			isAuthenticated = true
			user = map[string]interface{}{
				"ID":   session.UserID,
				"Name": "User", // Could fetch from API later
			}
		} else {
			// Token invalid, clear session
			session.Clear()
			session.Save(w)
		}
	}
	
	// Render home page with user context
	s.renderTemplate(w, "home", map[string]interface{}{
		"Title":           "Welcome to Alchemorsel",
		"User":            user,
		"IsAuthenticated": isAuthenticated,
	})
}

func (s *WebServer) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "login", map[string]interface{}{
		"Title": "Login - Alchemorsel",
	})
}

func (s *WebServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	email := r.FormValue("email")
	password := r.FormValue("password")

	// Call API to authenticate
	resp, err := s.apiClient.Login(r.Context(), email, password)
	if err != nil {
		s.renderTemplate(w, "login", map[string]interface{}{
			"Title": "Login - Alchemorsel",
			"Error": "Invalid credentials",
		})
		return
	}

	// Create session
	session := r.Context().Value("session").(*Session)
	session.UserID = resp.User.ID
	session.AccessToken = resp.AccessToken
	session.RefreshToken = resp.RefreshToken
	session.Save(w)

	// Redirect to home or requested page
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/recipes"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

func (s *WebServer) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "register", map[string]interface{}{
		"Title": "Register - Alchemorsel",
	})
}

func (s *WebServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")

	// Call API to register
	resp, err := s.apiClient.Register(r.Context(), name, email, password)
	if err != nil {
		s.renderTemplate(w, "register", map[string]interface{}{
			"Title": "Register - Alchemorsel",
			"Error": "Registration failed",
		})
		return
	}

	// Auto-login after registration
	loginResp, err := s.apiClient.Login(r.Context(), email, password)
	if err == nil {
		session := r.Context().Value("session").(*Session)
		session.UserID = resp.User.ID
		session.AccessToken = loginResp.AccessToken
		session.RefreshToken = loginResp.RefreshToken
		session.Save(w)
	}

	// Redirect to recipes
	http.Redirect(w, r, "/recipes", http.StatusSeeOther)
}

func (s *WebServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear session if it exists
	if sessionValue := r.Context().Value("session"); sessionValue != nil {
		if session, ok := sessionValue.(*Session); ok {
			// Delete from server store
			s.sessionStore.Delete(session.ID)
		}
	}

	// Always send deletion cookie to clear browser session
	deletionCookie := &http.Cookie{
		Name:     "alchemorsel-session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Allow HTTP for development
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1, // Delete immediately
	}
	http.SetCookie(w, deletionCookie)

	// Always use standard redirect for logout
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *WebServer) handleRecipeList(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*Session)
	
	// Get user context for navigation
	user, isAuthenticated := s.getUserContext(r)
	
	// Get recipes from API
	recipes, err := s.apiClient.GetRecipes(r.Context(), session.AccessToken)
	if err != nil {
		s.renderError(w, "Failed to load recipes", err)
		return
	}

	s.renderTemplate(w, "recipes", map[string]interface{}{
		"Title":           "Recipes - Alchemorsel",
		"Recipes":         recipes,
		"User":            user,
		"IsAuthenticated": isAuthenticated,
	})
}

func (s *WebServer) handleNewRecipePage(w http.ResponseWriter, r *http.Request) {
	// Get user context for navigation
	user, isAuthenticated := s.getUserContext(r)
	
	s.renderTemplate(w, "recipe-new", map[string]interface{}{
		"Title":           "New Recipe - Alchemorsel",
		"User":            user,
		"IsAuthenticated": isAuthenticated,
	})
}

func (s *WebServer) handleCreateRecipe(w http.ResponseWriter, r *http.Request) {
	// TODO: Parse form and create recipe via API
	http.Redirect(w, r, "/recipes", http.StatusSeeOther)
}

func (s *WebServer) handleRecipeDetail(w http.ResponseWriter, r *http.Request) {
	// TODO: Get recipe ID and fetch from API
	s.renderTemplate(w, "recipe-detail", map[string]interface{}{
		"Title": "Recipe - Alchemorsel",
	})
}

func (s *WebServer) handleEditRecipePage(w http.ResponseWriter, r *http.Request) {
	// TODO: Get recipe and render edit form
	s.renderTemplate(w, "recipe-edit", map[string]interface{}{
		"Title": "Edit Recipe - Alchemorsel",
	})
}

func (s *WebServer) handleUpdateRecipe(w http.ResponseWriter, r *http.Request) {
	// TODO: Update recipe via API
	http.Redirect(w, r, "/recipes", http.StatusSeeOther)
}

func (s *WebServer) handleDeleteRecipe(w http.ResponseWriter, r *http.Request) {
	// TODO: Delete recipe via API
	http.Redirect(w, r, "/recipes", http.StatusSeeOther)
}

func (s *WebServer) handleAIChatPage(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "ai-chat", map[string]interface{}{
		"Title": "AI Chef - Alchemorsel",
	})
}

func (s *WebServer) handleAIGenerate(w http.ResponseWriter, r *http.Request) {
	// TODO: Generate recipe via AI API
	w.Write([]byte("<div>AI generated recipe</div>"))
}

func (s *WebServer) handleAISuggest(w http.ResponseWriter, r *http.Request) {
	// TODO: Get suggestions from AI API
	w.Write([]byte("<div>AI suggestions</div>"))
}

func (s *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Get user context for navigation
	user, isAuthenticated := s.getUserContext(r)
	
	// Mock dashboard data for template
	createdAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	dashboardData := map[string]interface{}{
		"User": map[string]interface{}{
			"ID":        "user123",
			"Name":      "John Doe",
			"Username":  "johndoe",
			"Email":     "john@example.com",
			"Avatar":    "/static/img/default-avatar.jpg",
			"CreatedAt": createdAt,
		},
		"Stats": map[string]interface{}{
			"RecipeCount":   12,
			"LikesReceived": 234,
			"Followers":     45,
			"Collections":   3,
		},
		"UserRecipes":       []interface{}{}, // Empty for now - will be loaded via HTMX
		"RecentActivity":    []interface{}{}, // Empty for now - will be loaded via HTMX
		"TrendingRecipes":   []interface{}{}, // Empty for now - will be loaded via HTMX
		"UserCollections":   []interface{}{}, // Empty for now - will be loaded via HTMX
	}
	
	s.renderTemplate(w, "dashboard", map[string]interface{}{
		"Title":           "Dashboard - Alchemorsel",
		"User":            user,
		"IsAuthenticated": isAuthenticated,
		"Data":            dashboardData,
		"CurrentPage":     "dashboard",
	})
}

func (s *WebServer) handleProfile(w http.ResponseWriter, r *http.Request) {
	// Get user context for navigation
	user, isAuthenticated := s.getUserContext(r)
	
	// Mock profile data for template
	createdAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	profileData := map[string]interface{}{
		"User": map[string]interface{}{
			"ID":        "user123",
			"Name":      "John Doe",
			"Username":  "johndoe",
			"Email":     "john@example.com",
			"Avatar":    "/static/img/default-avatar.jpg",
			"CreatedAt": createdAt,
			"Profile": map[string]interface{}{
				"Bio":          "Passionate home cook and recipe creator",
				"Location":     "San Francisco, CA",
				"Website":      "https://johndoe.com",
				"CookingLevel": "intermediate",
			},
		},
		"Stats": map[string]interface{}{
			"RecipeCount":      12,
			"FollowersCount":   45,
			"FollowingCount":   67,
			"LikesReceived":    234,
			"CollectionsCount": 3,
			"LikedCount":       89,
		},
		"IsOwnProfile": true,
		"IsFollowing":  false,
	}
	
	s.renderTemplate(w, "profile", map[string]interface{}{
		"Title":           "Profile - Alchemorsel",
		"User":            user,
		"IsAuthenticated": isAuthenticated,
		"Data":            profileData,
	})
}

func (s *WebServer) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	// TODO: Update profile via API
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func (s *WebServer) handleFavorites(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "favorites", map[string]interface{}{
		"Title": "Favorites - Alchemorsel",
	})
}

// HTMX handlers (return partial HTML)

func (s *WebServer) handleHTMXSearch(w http.ResponseWriter, r *http.Request) {
	// TODO: Search recipes and return results
	w.Write([]byte("<div>Search results</div>"))
}

func (s *WebServer) handleHTMXLike(w http.ResponseWriter, r *http.Request) {
	// TODO: Like recipe and return updated button
	w.Write([]byte("<button>Liked</button>"))
}

func (s *WebServer) handleHTMXRate(w http.ResponseWriter, r *http.Request) {
	// TODO: Rate recipe and return updated rating
	w.Write([]byte("<div>Rating updated</div>"))
}

func (s *WebServer) handleHTMXComments(w http.ResponseWriter, r *http.Request) {
	// TODO: Get comments and return HTML
	w.Write([]byte("<div>Comments</div>"))
}

func (s *WebServer) handleHTMXAddComment(w http.ResponseWriter, r *http.Request) {
	// TODO: Add comment and return updated comments
	w.Write([]byte("<div>Comment added</div>"))
}

func (s *WebServer) handleHTMXNotifications(w http.ResponseWriter, r *http.Request) {
	// TODO: Get notifications and return HTML
	w.Write([]byte("<div>Notifications</div>"))
}

func (s *WebServer) handleHTMXAIChat(w http.ResponseWriter, r *http.Request) {
	// CRITICAL SECURITY FIX ALV3-2025-001: Validate authentication (enforced by middleware)
	session := r.Context().Value("session").(*Session)
	if session.UserID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("<div class=\"error\">Authentication required. Please <a href=\"/login\">login</a> to use AI features.</div>"))
		return
	}

	// CRITICAL SECURITY FIX ALV3-2025-002: XSS Protection - Sanitize input
	message := strings.TrimSpace(r.FormValue("message"))
	if message == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<div class=\"error\">Message is required</div>"))
		return
	}

	// SECURITY: Validate message length and content
	if len(message) > 1000 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<div class=\"error\">Message too long (max 1000 characters)</div>"))
		return
	}

	// SECURITY: Sanitize user input to prevent XSS
	message = html.EscapeString(message)

	s.logger.Debug("AI Chat request", zap.String("message", message), zap.String("user_id", session.UserID))

	// TODO: Call AI service to get response
	// SECURITY NOTE: User message is now properly sanitized above
	// For now, return a mock response with sanitized input
	aiResponse := `<div class="chat-message user-message" style="margin-bottom: 1rem;">
		<div style="display: flex; justify-content: flex-end; gap: 0.75rem;">
			<div class="message-content" style="flex: 1; background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%); color: white; padding: 1rem; border-radius: 1rem; max-width: 80%;">
				<div style="font-weight: 600; margin-bottom: 0.5rem;">You</div>
				<div style="line-height: 1.6;">` + message + `</div>
				<div style="font-size: 0.75rem; opacity: 0.8; margin-top: 0.5rem;">Just now</div>
			</div>
			<div class="avatar user-avatar" style="width: 2.5rem; height: 2.5rem; border-radius: 50%; background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%); display: flex; align-items: center; justify-content: center; color: white; font-weight: bold; flex-shrink: 0;">
				👤
			</div>
		</div>
	</div>
	<div class="chat-message ai-message" style="margin-bottom: 1rem;">
		<div style="display: flex; align-items: flex-start; gap: 0.75rem;">
			<div class="avatar ai-avatar" style="width: 2.5rem; height: 2.5rem; border-radius: 50%; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-weight: bold; flex-shrink: 0;">
				👨‍🍳
			</div>
			<div class="message-content" style="flex: 1; background: #ffffff; padding: 1rem; border-radius: 1rem; border: 1px solid #e2e8f0; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
				<div style="font-weight: 600; color: #4f46e5; margin-bottom: 0.5rem;">AI Chef</div>
				<div style="line-height: 1.6;">
					Great question! For chicken and vegetables, I recommend trying a delicious **Chicken Stir-Fry**. Here's what you can make:
					<br><br>
					🍗 **Quick Chicken & Veggie Stir-Fry** (20 mins)<br>
					• 1 lb chicken breast, sliced<br>
					• 2 cups mixed vegetables (bell peppers, broccoli, carrots)<br>
					• 2 tbsp soy sauce, 1 tbsp garlic, ginger<br>
					• Serve over rice or noodles
					<br><br>
					Would you like the full recipe with step-by-step instructions?
				</div>
				<div style="font-size: 0.75rem; color: #9ca3af; margin-top: 0.5rem;">Just now</div>
			</div>
		</div>
	</div>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(aiResponse))
}

func (s *WebServer) handleHTMXRecipeSearch(w http.ResponseWriter, r *http.Request) {
	// SECURITY FIX ALV3-2025-005: Recipe Search now requires authentication (enforced by middleware)
	session := r.Context().Value("session").(*Session)
	if session.UserID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("<div class=\"error\">Authentication required to search recipes.</div>"))
		return
	}

	// SECURITY FIX ALV3-2025-006: Input validation and sanitization
	query := strings.TrimSpace(r.FormValue("q"))
	if query == "" {
		w.Write([]byte("<div>Please enter a search term</div>"))
		return
	}

	// SECURITY: Validate query length
	if len(query) > 100 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<div class=\"error\">Search term too long (max 100 characters)</div>"))
		return
	}

	// SECURITY: Sanitize search query to prevent XSS and injection attacks
	query = html.EscapeString(query)

	s.logger.Debug("Recipe search", zap.String("query", query), zap.String("user_id", session.UserID))

	// TODO: Call API to search recipes
	// For now, return mock search results
	searchResults := `<div class="search-results">
		<h3 style="margin-bottom: 1rem;">Search Results for "` + query + `"</h3>
		<div class="recipe-grid" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(250px, 1fr)); gap: 1rem;">
			<div class="recipe-card" style="background: white; border-radius: 0.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); overflow: hidden;">
				<div style="height: 120px; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 2rem;">🍗</div>
				<div style="padding: 1rem;">
					<h4 style="margin-bottom: 0.5rem;">Chicken Stir-Fry</h4>
					<p style="color: #718096; font-size: 0.875rem; margin-bottom: 1rem;">Quick and healthy chicken with vegetables</p>
					<div style="display: flex; justify-content: space-between; align-items: center;">
						<span style="color: #f39c12;">★★★★★</span>
						<span style="color: #718096; font-size: 0.875rem;">20 min</span>
					</div>
				</div>
			</div>
			<div class="recipe-card" style="background: white; border-radius: 0.5rem; box-shadow: 0 1px 3px rgba(0,0,0,0.1); overflow: hidden;">
				<div style="height: 120px; background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); display: flex; align-items: center; justify-content: center; color: white; font-size: 2rem;">🥗</div>
				<div style="padding: 1rem;">
					<h4 style="margin-bottom: 0.5rem;">Garden Salad</h4>
					<p style="color: #718096; font-size: 0.875rem; margin-bottom: 1rem;">Fresh vegetables with herb dressing</p>
					<div style="display: flex; justify-content: space-between; align-items: center;">
						<span style="color: #f39c12;">★★★★☆</span>
						<span style="color: #718096; font-size: 0.875rem;">10 min</span>
					</div>
				</div>
			</div>
		</div>
	</div>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(searchResults))
}

// Helper methods

func (s *WebServer) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Ensure data is a map for template execution
	templateData := make(map[string]interface{})
	if data != nil {
		if dataMap, ok := data.(map[string]interface{}); ok {
			templateData = dataMap
		}
	}
	
	// Set default values if not provided
	if templateData["Title"] == nil {
		templateData["Title"] = "Alchemorsel"
	}
	if templateData["BaseURL"] == nil {
		templateData["BaseURL"] = "http://localhost:8080"
	}
	// Set template name for base template conditional logic
	templateData["TemplateName"] = name
	
	// Add user context if not already provided and if we can access the request context
	if templateData["User"] == nil || templateData["IsAuthenticated"] == nil {
		// Try to get session from request context if available
		// Note: This requires the request context to be available
		// For now, we'll rely on handlers to explicitly pass User data
	}
	
	// Debug: Log template execution
	s.logger.Debug("Executing template", 
		zap.String("template", name),
		zap.Any("data", templateData))
	
	// Try to execute the named template
	err := s.templates.ExecuteTemplate(w, name, templateData)
	if err != nil {
		s.logger.Error("Failed to execute template", 
			zap.String("template", name), 
			zap.Error(err))
		
		// Fallback to simple HTML if template execution fails
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>%s | Alchemorsel</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <link href="https://cdn.jsdelivr.net/npm/tailwindcss@2.2.19/dist/tailwind.min.css" rel="stylesheet">
</head>
<body class="bg-gray-50">
    <div class="container mx-auto p-4">
        <h1 class="text-3xl font-bold mb-4">Alchemorsel v3</h1>
        <p>Error loading template: %s</p>
        <p>Error: %s</p>
    </div>
</body>
</html>`, templateData["Title"], name, err.Error())
	}
}

// getUserContext extracts user information from session for template rendering
func (s *WebServer) getUserContext(r *http.Request) (user interface{}, isAuthenticated bool) {
	session := r.Context().Value("session").(*Session)
	
	if session.UserID != "" && session.AccessToken != "" {
		// Verify token is still valid with API
		if s.apiClient.VerifyToken(r.Context(), session.AccessToken) {
			isAuthenticated = true
			user = map[string]interface{}{
				"ID":   session.UserID,
				"Name": "User", // Could fetch from API later
			}
		}
	}
	
	return user, isAuthenticated
}

func (s *WebServer) renderError(w http.ResponseWriter, message string, err error) {
	s.logger.Error(message, zap.Error(err))
	w.WriteHeader(http.StatusInternalServerError)
	s.renderTemplate(w, "error", map[string]interface{}{
		"Title":   "Error - Alchemorsel",
		"Message": message,
	})
}

// Security Middleware Functions

// securityHeadersMiddleware adds security headers to all responses
func (s *WebServer) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CRITICAL SECURITY FIX: Add comprehensive security headers
		
		// XSS Protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		
		// Content Type Options
		w.Header().Set("X-Content-Type-Options", "nosniff")
		
		// Frame Options
		w.Header().Set("X-Frame-Options", "DENY")
		
		// Content Security Policy
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' https://unpkg.com https://cdn.jsdelivr.net; " +
			"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
			"img-src 'self' data: https:; " +
			"font-src 'self' data:; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'none'; " +
			"object-src 'none';"
		w.Header().Set("Content-Security-Policy", csp)
		
		// HSTS (HTTP Strict Transport Security) - only in production with HTTPS
		if s.config.IsProduction() {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		
		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Permissions Policy
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware implements basic rate limiting
func (s *WebServer) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			clientIP = strings.Split(xff, ",")[0]
		}
		
		now := time.Now()
		key := fmt.Sprintf("rate_limit:%s", clientIP)
		
		// Check current request count
		if val, exists := s.rateLimitStore.Load(key); exists {
			if requests, ok := val.(*rateLimitEntry); ok {
				// Clean old entries
				var validRequests []time.Time
				for _, reqTime := range requests.requests {
					if now.Sub(reqTime) < time.Minute {
						validRequests = append(validRequests, reqTime)
					}
				}
				
				// Check if limit exceeded (60 requests per minute)
				if len(validRequests) >= 60 {
					s.logger.Warn("Rate limit exceeded",
						zap.String("ip", clientIP),
						zap.String("path", r.URL.Path),
						zap.Int("requests", len(validRequests)),
					)
					w.Header().Set("Retry-After", "60")
					http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
					return
				}
				
				// Update with new request
				requests.requests = append(validRequests, now)
			}
		} else {
			// First request from this IP
			s.rateLimitStore.Store(key, &rateLimitEntry{
				requests: []time.Time{now},
			})
		}
		
		next.ServeHTTP(w, r)
	})
}

// csrfMiddleware provides CSRF protection for state-changing requests
func (s *WebServer) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CRITICAL SECURITY FIX ALV3-2025-003: CSRF Protection
		
		// Skip CSRF check for safe methods
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}
		
		session := r.Context().Value("session").(*Session)
		if session == nil {
			http.Error(w, "Session required", http.StatusForbidden)
			return
		}
		
		// Get CSRF token from header or form
		token := r.Header.Get("X-CSRF-Token")
		if token == "" {
			token = r.FormValue("csrf_token")
		}
		
		if token == "" {
			s.logger.Warn("Missing CSRF token",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("ip", r.RemoteAddr),
			)
			if r.Header.Get("HX-Request") == "true" {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("<div class=\"error\">CSRF token required</div>"))
				return
			}
			http.Error(w, "CSRF token required", http.StatusForbidden)
			return
		}
		
		// Validate CSRF token
		expectedToken := s.generateCSRFToken(session.ID)
		if !s.validateCSRFToken(token, expectedToken) {
			s.logger.Warn("Invalid CSRF token",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("session_id", session.ID),
				zap.String("ip", r.RemoteAddr),
			)
			if r.Header.Get("HX-Request") == "true" {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("<div class=\"error\">Invalid CSRF token</div>"))
				return
			}
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// inputValidationMiddleware validates input data
func (s *WebServer) inputValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SECURITY FIX ALV3-2025-006: Input validation
		
		// Check for suspicious patterns in URL path
		if s.containsSuspiciousPatterns(r.URL.Path) {
			s.logger.Warn("Suspicious URL pattern detected",
				zap.String("path", r.URL.Path),
				zap.String("ip", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			)
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		
		// Validate request size
		if r.ContentLength > 10*1024*1024 { // 10MB limit
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
			return
		}
		
		// For POST requests, parse and validate form data
		if r.Method == "POST" && strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			if err := r.ParseForm(); err == nil {
				for field, values := range r.Form {
					for _, value := range values {
						if s.containsDangerousContent(value) {
							s.logger.Warn("Dangerous content detected in form field",
								zap.String("field", field),
								zap.String("ip", r.RemoteAddr),
							)
							http.Error(w, "Invalid input detected", http.StatusBadRequest)
							return
						}
					}
				}
			}
		}
		
		next.ServeHTTP(w, r)
	})
}

// Helper types and functions for rate limiting
type rateLimitEntry struct {
	requests []time.Time
}

// generateCSRFToken generates a CSRF token for the given session
func (s *WebServer) generateCSRFToken(sessionID string) string {
	// Simple CSRF token generation (should use HMAC in production)
	return fmt.Sprintf("%s:%d", sessionID, time.Now().Unix())
}

// validateCSRFToken validates a CSRF token
func (s *WebServer) validateCSRFToken(providedToken, expectedToken string) bool {
	// Use constant time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(providedToken), []byte(expectedToken)) == 1
}

// containsSuspiciousPatterns checks for common attack patterns
func (s *WebServer) containsSuspiciousPatterns(input string) bool {
	suspiciousPatterns := []string{
		"../", "..\\\\", "..", "%2e%2e", "%252e%252e",
		"<script", "</script>", "javascript:", "vbscript:",
		"onload=", "onerror=", "onclick=", "onmouseover=",
		"eval(", "alert(", "confirm(", "prompt(",
		"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "DROP ",
		"UNION ", "OR 1=1", "AND 1=1", "' OR '", "' AND '",
		"admin'--", "admin'/*", "1' OR '1'='1",
		"null", "/etc/passwd", "/proc/", "\\\\windows\\\\",
		"cmd.exe", "powershell", "/bin/bash", "/bin/sh",
	}
	
	inputLower := strings.ToLower(input)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return true
		}
	}
	
	return false
}

// containsDangerousContent checks for dangerous content in form fields
func (s *WebServer) containsDangerousContent(input string) bool {
	// Regex patterns for XSS and injection attacks
	dangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)<script[^>]*>`),
		regexp.MustCompile(`(?i)javascript:`),
		regexp.MustCompile(`(?i)vbscript:`),
		regexp.MustCompile(`(?i)on\w+\s*=`),
		regexp.MustCompile(`(?i)(union|select|insert|update|delete|drop)\s`),
		regexp.MustCompile(`(?i)(eval|alert|confirm|prompt)\s*\(`),
	}
	
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	
	return false
}