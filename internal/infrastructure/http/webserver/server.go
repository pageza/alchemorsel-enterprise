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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/ai"
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
	sessionManager *SessionManager
	templates      *template.Template
	healthCheck    *healthcheck.EnterpriseHealthCheck
	rateLimitStore *sync.Map // For rate limiting
	csrfSecret     []byte    // For CSRF protection
	// Performance monitoring
	perfMonitor *performance.PerformanceMonitor
	// AI chat
	convService  *conversation.Service
	ollamaClient *ai.OllamaClient
}

// NewWebServer creates a new web frontend server instance
func NewWebServer(
	cfg *config.Config,
	log *zap.Logger,
	apiClient *APIClient,
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

	// Initialize AI components
	log.Info("Initializing Ollama client...")
	// Try the AI-specific host first, then fall back to general host
	ollamaHost := cfg.GetString("ALCHEMORSEL_AI_OLLAMA_HOST")
	log.Info("Ollama host from ALCHEMORSEL_AI_OLLAMA_HOST", zap.String("host", ollamaHost))
	if ollamaHost == "" {
		ollamaHost = cfg.GetString("ALCHEMORSEL_OLLAMA_HOST")
		log.Info("Ollama host from ALCHEMORSEL_OLLAMA_HOST", zap.String("host", ollamaHost))
	}
	if ollamaHost == "" {
		ollamaHost = "http://172.17.0.1:11434" // Default to Docker gateway IP
		log.Info("Using default Ollama host", zap.String("host", ollamaHost))
	}
	ollamaModel := cfg.GetString("ALCHEMORSEL_AI_CHAT_MODEL")
	log.Info("Ollama model from ALCHEMORSEL_AI_CHAT_MODEL", zap.String("model", ollamaModel))
	if ollamaModel == "" {
		ollamaModel = cfg.GetString("ALCHEMORSEL_OLLAMA_CHAT_MODEL")
		log.Info("Ollama model from ALCHEMORSEL_OLLAMA_CHAT_MODEL", zap.String("model", ollamaModel))
	}
	if ollamaModel == "" {
		ollamaModel = "phi3:mini" // Default model matching .env
		log.Info("Using default Ollama model", zap.String("model", ollamaModel))
	}
	log.Info("Creating Ollama client with configuration",
		zap.String("host", ollamaHost),
		zap.String("model", ollamaModel))
	ollamaClient := ai.NewOllamaClient(ollamaHost, ollamaModel)

	// Initialize SCS session manager
	log.Info("Initializing session manager...")
	sessionManager, err := NewSessionManager(cfg, log)
	if err != nil {
		log.Error("Failed to initialize session manager", zap.Error(err))
		return nil, fmt.Errorf("failed to initialize session manager: %w", err)
	}
	log.Info("Session manager initialized successfully")

	// TODO: Initialize conversation service with proper repositories
	// For now, we'll initialize it with nil and update later when we have database setup
	var convService *conversation.Service

	server := &WebServer{
		config:         cfg,
		logger:         log,
		apiClient:      apiClient,
		sessionManager: sessionManager,
		templates:      templates,
		healthCheck:    healthCheck,
		rateLimitStore: &sync.Map{},
		convService:    convService,
		ollamaClient:   ollamaClient,
		csrfSecret:     []byte("secure-csrf-secret-key-32-chars"), // TODO: Generate from config
		perfMonitor:    perfMonitor,
	}

	server.router = server.setupRoutes()

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Info("Creating HTTP server", zap.String("addr", addr))

	server.server = &http.Server{
		Addr:         addr,
		Handler:      server.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Info("HTTP server created", zap.String("server_addr", server.server.Addr))

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

	// Performance monitoring middleware with recovery
	r.Use(s.resilientMiddleware("performance", s.perfMonitor.Middleware()))
	r.Use(s.securityHeadersMiddleware)

	// Session middleware with resilient wrapper to handle Redis failures gracefully
	r.Use(s.resilientMiddleware("session", s.sessionManager.LoadAndSave))
	r.Use(s.rateLimitMiddleware)

	// Static files - serve from embedded static subdirectory
	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		s.logger.Error("Failed to create static sub-filesystem", zap.Error(err))
	} else {
		r.Mount("/static", http.StripPrefix("/static", http.FileServer(http.FS(staticSubFS))))
	}

	// Service Worker - serve from root for full scope control
	r.Get("/sw.js", s.handleServiceWorker)

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

		// Dashboard HTMX endpoints
		r.Get("/dashboard/recipes", s.handleHTMXDashboardRecipes)
		r.Get("/dashboard/activity", s.handleHTMXDashboardActivity)
		r.Get("/dashboard/trending", s.handleHTMXDashboardTrending)
		r.Get("/dashboard/collections", s.handleHTMXDashboardCollections)

		// AI Chat endpoints - Now properly secured
		r.Post("/ai/chat", s.handleHTMXAIChat)
		r.Post("/ai/chat/reset", s.handleHTMXAIChatReset)

		// Multi-Chat API endpoints
		r.Get("/api/chat/conversations", s.handleAPIConversationList)
		r.Get("/api/chat/history", s.handleAPIConversationHistory)
		r.Post("/api/chat/message", s.handleAPIChatMessage)
		r.Post("/api/chat/rename", s.handleAPIConversationRename)
		r.Post("/api/chat/delete", s.handleAPIConversationDelete)

		// Multi-Chat HTMX endpoints
		r.Get("/chat/conversations-list", s.handleHTMXConversationList)
		r.Get("/chat/history", s.handleHTMXConversationHistory)

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

	s.logger.Info("About to call ListenAndServe", zap.String("addr", s.server.Addr))
	err := s.server.ListenAndServe()
	s.logger.Info("ListenAndServe returned", zap.Error(err))
	return err
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

func (s *WebServer) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get authentication data from SCS session
		userID := s.sessionManager.GetString(r.Context(), "user_id")
		accessToken := s.sessionManager.GetString(r.Context(), "access_token")

		if userID == "" || accessToken == "" {
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

		// DEBUG: Log session data for debugging
		s.logger.Debug("requireAuth middleware",
			zap.String("user_id", userID),
			zap.String("access_token_prefix", func() string {
				if len(accessToken) > 10 {
					return accessToken[:10] + "..."
				}
				return accessToken
			}()),
			zap.String("session_token", s.sessionManager.Token(r.Context())),
			zap.String("path", r.URL.Path),
		)

		// Verify token is still valid with API
		if !s.apiClient.VerifyToken(r.Context(), accessToken) {
			s.logger.Warn("Token verification failed",
				zap.String("user_id", userID),
				zap.String("access_token_prefix", func() string {
					if len(accessToken) > 10 {
						return accessToken[:10] + "..."
					}
					return accessToken
				}()),
			)

			// Token invalid, clear session
			s.sessionManager.Clear(r.Context())
			s.sessionManager.RenewToken(r.Context())

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

func (s *WebServer) handleServiceWorker(w http.ResponseWriter, r *http.Request) {
	// Serve service worker from root for full scope control
	data, err := staticFS.ReadFile("static/sw.js")
	if err != nil {
		s.logger.Error("Service worker not found", zap.Error(err))
		http.Error(w, "Service worker not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache") // Service workers should not be cached
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *WebServer) handleHome(w http.ResponseWriter, r *http.Request) {
	// Use the same authentication method as other pages
	user, isAuthenticated := s.getUserContext(r)

	s.logger.Info("Home page authentication check",
		zap.Bool("is_authenticated", isAuthenticated),
		zap.Any("user", user),
	)

	// Authenticated users get redirected to AI Chat (their "home")
	if isAuthenticated {
		http.Redirect(w, r, "/ai/chat", http.StatusSeeOther)
		return
	}

	// For unauthenticated users, show marketing page
	data := map[string]interface{}{
		"Title":           "Welcome to Alchemorsel",
		"User":            nil,
		"IsAuthenticated": false,
		"CurrentPage":     "home",
	}

	// Fetch featured recipes for unauthenticated users
	featuredRecipes, err := s.getFeaturedRecipes(r.Context())
	if err != nil {
		s.logger.Error("Failed to fetch featured recipes", zap.Error(err))
	} else {
		data["FeaturedRecipes"] = featuredRecipes
	}

	// Render home page for unauthenticated users only
	s.renderTemplate(w, "home", data)
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

	// Store authentication data in SCS session
	s.sessionManager.Put(r.Context(), "user_id", resp.User.ID)
	s.sessionManager.Put(r.Context(), "access_token", resp.AccessToken)
	s.sessionManager.Put(r.Context(), "refresh_token", resp.RefreshToken)
	s.sessionManager.Put(r.Context(), "user_name", resp.User.Name)
	s.sessionManager.Put(r.Context(), "user_email", resp.User.Email)

	s.logger.Info("Session data stored after login",
		zap.String("user_id", resp.User.ID),
		zap.String("user_name", resp.User.Name),
		zap.String("user_email", resp.User.Email),
		zap.String("access_token_prefix", func() string {
			if len(resp.AccessToken) > 10 {
				return resp.AccessToken[:10] + "..."
			}
			return resp.AccessToken
		}()),
		zap.String("session_token", s.sessionManager.Token(r.Context())),
	)

	// DEBUG: Verify session data was stored correctly
	storedUserID := s.sessionManager.GetString(r.Context(), "user_id")
	storedAccessToken := s.sessionManager.GetString(r.Context(), "access_token")
	s.logger.Debug("Verification after session storage",
		zap.String("stored_user_id", storedUserID),
		zap.String("stored_access_token_prefix", func() string {
			if len(storedAccessToken) > 10 {
				return storedAccessToken[:10] + "..."
			}
			return storedAccessToken
		}()),
	)

	// Redirect to home or requested page
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" {
		redirect = "/" // Let handleHome redirect authenticated users to AI chat
	}

	// Check if this is an HTMX request
	if r.Header.Get("HX-Request") == "true" {
		// For HTMX requests, use HX-Redirect header
		w.Header().Set("HX-Redirect", redirect)
		w.WriteHeader(http.StatusOK)
		return
	}

	// For regular requests, use standard redirect
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
		// Handle HTMX requests differently - return just the form with error
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`<div class="alert alert-error">Registration failed: ` + err.Error() + `</div>`))
			return
		}

		// For regular requests, render full template
		s.renderTemplate(w, "register", map[string]interface{}{
			"Title": "Register - Alchemorsel",
			"Error": "Registration failed: " + err.Error(),
		})
		return
	}

	// Auto-login after registration
	loginResp, err := s.apiClient.Login(r.Context(), email, password)
	if err == nil {
		// Store authentication data in SCS session
		s.sessionManager.Put(r.Context(), "user_id", resp.User.ID)
		s.sessionManager.Put(r.Context(), "access_token", loginResp.AccessToken)
		s.sessionManager.Put(r.Context(), "refresh_token", loginResp.RefreshToken)
		s.sessionManager.Put(r.Context(), "user_name", resp.User.Name)
		s.sessionManager.Put(r.Context(), "user_email", resp.User.Email)

		// DEBUG: Verify session data was stored correctly after registration
		storedUserID := s.sessionManager.GetString(r.Context(), "user_id")
		storedAccessToken := s.sessionManager.GetString(r.Context(), "access_token")
		s.logger.Debug("Registration session verification",
			zap.String("stored_user_id", storedUserID),
			zap.String("stored_access_token_prefix", func() string {
				if len(storedAccessToken) > 10 {
					return storedAccessToken[:10] + "..."
				}
				return storedAccessToken
			}()),
			zap.String("session_token", s.sessionManager.Token(r.Context())),
		)
	}

	// HTMX-aware redirect to dashboard
	if r.Header.Get("HX-Request") == "true" {
		// For HTMX requests, use HX-Redirect header
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}

	// For regular requests, use standard redirect
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (s *WebServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear SCS session data
	err := s.sessionManager.Destroy(r.Context())
	if err != nil {
		s.logger.Error("Failed to destroy session", zap.Error(err))
	}

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *WebServer) handleRecipeList(w http.ResponseWriter, r *http.Request) {
	// Get user context for navigation
	user, isAuthenticated := s.getUserContext(r)

	// Get access token from SCS session
	accessToken := s.sessionManager.GetString(r.Context(), "access_token")

	// Get recipes from API
	recipes, err := s.apiClient.GetRecipes(r.Context(), accessToken)
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
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	var user interface{} = nil

	// Get user information if authenticated
	if userID != "" {
		// In a real application, you'd fetch user details from the database
		// For now, we'll create a simple user object
		user = map[string]interface{}{
			"ID":   userID,
			"Name": s.sessionManager.GetString(r.Context(), "user_name"),
		}
	}

	// Generate CSRF token for authenticated users
	var csrfToken string
	if userID != "" {
		csrfToken = s.generateCSRFToken(userID)
	}

	// Load existing conversation history
	var conversationHistory []ConversationMessage
	if userID != "" {
		conversationID := r.URL.Query().Get("conversation_id")
		if conversationID == "" {
			// Get the most recent conversation for this user
			conversationIndexKey := fmt.Sprintf("conversation_index_%s", userID)
			existingIndex := s.sessionManager.Get(r.Context(), conversationIndexKey)

			if existingIndex != nil {
				if index, ok := existingIndex.([]map[string]interface{}); ok && len(index) > 0 {
					// Get the most recent conversation (first in index)
					mostRecent := index[0]
					if convID, ok := mostRecent["id"].(string); ok {
						conversationID = convID
					}
				}
			}
		}

		// Load conversation history if we have a conversation ID
		if conversationID != "" {
			conversationKey := fmt.Sprintf("ai_conversation_%s_%s", userID, conversationID)
			savedHistory := s.sessionManager.Get(r.Context(), conversationKey)

			if savedHistory != nil {
				if history, ok := savedHistory.([]ConversationMessage); ok {
					conversationHistory = history
					s.logger.Debug("Loaded conversation history",
						zap.String("user_id", userID),
						zap.String("conversation_id", conversationID),
						zap.Int("message_count", len(history)),
					)
				}
			}
		}
	}

	// DEBUG: Log conversation history before template render
	s.logger.Debug("Template render debug",
		zap.String("user_id", userID),
		zap.Int("conversation_history_length", len(conversationHistory)),
		zap.Bool("conversation_history_nil", conversationHistory == nil),
		zap.String("conversation_history_type", fmt.Sprintf("%T", conversationHistory)),
	)

	// Log first message if exists
	if len(conversationHistory) > 0 {
		firstMsg := conversationHistory[0]
		s.logger.Debug("First conversation message",
			zap.String("role", firstMsg.Role),
			zap.String("content_preview", firstMsg.Content[:min(50, len(firstMsg.Content))]),
			zap.Time("timestamp", firstMsg.Timestamp),
		)
	}

	s.renderTemplate(w, "chat", map[string]interface{}{
		"Title":               "Chat with AI Chef - Alchemorsel",
		"Description":         "Chat with our AI Chef to create amazing recipes through conversation",
		"User":                user,
		"IsAuthenticated":     user != nil,
		"CurrentPage":         "ai-chat",
		"CSRFToken":           csrfToken,
		"ConversationHistory": conversationHistory,
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

	// Debug: Log authentication status
	s.logger.Debug("Dashboard access attempt",
		zap.Bool("isAuthenticated", isAuthenticated),
		zap.Any("user", user),
	)

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
		"UserRecipes":     []interface{}{}, // Empty for now - will be loaded via HTMX
		"RecentActivity":  []interface{}{}, // Empty for now - will be loaded via HTMX
		"TrendingRecipes": []interface{}{}, // Empty for now - will be loaded via HTMX
		"UserCollections": []interface{}{}, // Empty for now - will be loaded via HTMX
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
	startTime := time.Now()

	// Add panic recovery with detailed logging
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("PANIC in handleHTMXAIChat",
				zap.Any("panic", r),
				zap.String("stack", fmt.Sprintf("%+v", r)),
				zap.Duration("duration", time.Since(startTime)),
			)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("<div class=\"error\">Internal server error occurred. Please try again.</div>"))
		}
	}()

	s.logger.Info("AI Chat request started",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("timestamp", startTime.Format("15:04:05.000")),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("user_agent", r.UserAgent()),
		zap.String("content_type", r.Header.Get("Content-Type")),
	)

	// Log form data for debugging
	if err := r.ParseForm(); err != nil {
		s.logger.Error("Failed to parse form data", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<div class=\"error\">Invalid form data</div>"))
		return
	}

	s.logger.Debug("Form data received",
		zap.Any("form_values", r.Form),
		zap.String("message_raw", r.FormValue("message")),
		zap.String("csrf_token", r.FormValue("csrf_token")),
	)

	// CRITICAL SECURITY FIX ALV3-2025-001: Validate authentication (enforced by middleware)
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		s.logger.Warn("Unauthorized AI chat request", zap.Duration("duration", time.Since(startTime)))
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("<div class=\"error\">Authentication required. Please <a href=\"/login\">login</a> to use AI features.</div>"))
		return
	}

	// CRITICAL SECURITY FIX ALV3-2025-002: XSS Protection - Sanitize input
	message := strings.TrimSpace(r.FormValue("message"))
	if message == "" {
		s.logger.Warn("Empty message in AI chat", zap.String("user_id", userID), zap.Duration("duration", time.Since(startTime)))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<div class=\"error\">Message is required</div>"))
		return
	}

	// SECURITY: Validate message length and content
	if len(message) > 1000 {
		s.logger.Warn("Message too long in AI chat", zap.String("user_id", userID), zap.Int("length", len(message)), zap.Duration("duration", time.Since(startTime)))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<div class=\"error\">Message too long (max 1000 characters)</div>"))
		return
	}

	// SECURITY: Sanitize user input to prevent XSS
	message = html.EscapeString(message)

	validationTime := time.Since(startTime)
	s.logger.Info("AI Chat validation completed",
		zap.String("user_id", userID),
		zap.String("message_preview", message[:min(50, len(message))]),
		zap.Int("message_length", len(message)),
		zap.Duration("validation_duration", validationTime),
	)

	// Get conversation ID from request
	conversationID := strings.TrimSpace(r.FormValue("conversation_id"))
	if conversationID == "" {
		// Generate new conversation ID if not provided
		conversationID = fmt.Sprintf("conv_%d_%s", time.Now().UnixNano(), userID)
	}

	// Get conversation history from session using conversation-specific key
	conversationKey := fmt.Sprintf("ai_conversation_%s_%s", userID, conversationID)
	s.logger.Debug("Attempting to retrieve conversation history",
		zap.String("user_id", userID),
		zap.String("conversation_id", conversationID),
		zap.String("conversation_key", conversationKey),
	)

	existingHistory := s.sessionManager.Get(r.Context(), conversationKey)
	s.logger.Debug("Session history retrieval",
		zap.String("user_id", userID),
		zap.Bool("history_exists", existingHistory != nil),
		zap.String("history_type", fmt.Sprintf("%T", existingHistory)),
	)

	var persistentHistory []ConversationMessage
	if existingHistory != nil {
		if history, ok := existingHistory.([]ConversationMessage); ok {
			persistentHistory = history
			s.logger.Info("Retrieved conversation history",
				zap.String("user_id", userID),
				zap.Int("history_length", len(history)),
			)
		} else {
			s.logger.Warn("Conversation history type assertion failed",
				zap.String("user_id", userID),
				zap.String("actual_type", fmt.Sprintf("%T", existingHistory)),
			)
		}
	} else {
		s.logger.Debug("No existing conversation history found", zap.String("user_id", userID))
	}

	// Create chat messages starting with system prompt
	messages := []conversation.ChatMessage{
		{
			Role:    "system",
			Content: "You are an expert AI chef assistant helping users with cooking and recipes. You are knowledgeable, friendly, and practical. Always provide helpful, accurate, and safe cooking advice. Maintain context from previous messages in this conversation to provide coherent, connected responses.",
		},
	}

	// Convert persistent history to API format and apply limit
	historyLimit := 10
	apiHistory := make([]conversation.ChatMessage, 0, len(persistentHistory))

	startIdx := 0
	if len(persistentHistory) > historyLimit {
		startIdx = len(persistentHistory) - historyLimit
	}

	for i := startIdx; i < len(persistentHistory); i++ {
		msg := persistentHistory[i]
		apiHistory = append(apiHistory, conversation.ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	messages = append(messages, apiHistory...)

	// Add current user message
	currentUserMessage := conversation.ChatMessage{
		Role:    "user",
		Content: message, // Already sanitized above
	}
	messages = append(messages, currentUserMessage)

	// Generate AI response using Ollama
	ollamaStartTime := time.Now()
	s.logger.Info("Starting Ollama AI generation",
		zap.String("user_id", userID),
		zap.String("timestamp", ollamaStartTime.Format("15:04:05.000")),
		zap.Int("total_messages", len(messages)),
		zap.Bool("ollama_client_exists", s.ollamaClient != nil),
	)

	// Log the messages being sent (first few for debugging)
	for i, msg := range messages {
		if i < 3 || i == len(messages)-1 { // Log first 3 and last message
			s.logger.Debug("Message to Ollama",
				zap.Int("index", i),
				zap.String("role", msg.Role),
				zap.String("content_preview", msg.Content[:min(100, len(msg.Content))]),
			)
		}
	}

	if s.ollamaClient == nil {
		s.logger.Error("Ollama client is nil")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("<div class=\"error\">AI service not available</div>"))
		return
	}

	// Use a very short timeout (10 seconds) to avoid connection issues
	// If Ollama is slow, we'll use the fallback response
	ollamaCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	aiResult, err := s.ollamaClient.GenerateChatCompletion(ollamaCtx, messages, 0.7, 2048)

	ollmaDuration := time.Since(ollamaStartTime)
	var aiResponseContent string
	if err != nil {
		s.logger.Error("Ollama generation failed",
			zap.Error(err),
			zap.String("user_id", userID),
			zap.Duration("ollama_duration", ollmaDuration),
			zap.Duration("total_duration", time.Since(startTime)),
		)

		// Send fallback response
		aiResponseContent = "I'm having trouble connecting to my AI chef brain right now! 🧠 Could you try asking again in a moment? In the meantime, I'm here to help with any cooking questions you have!"
	} else {
		s.logger.Info("Ollama generation completed successfully",
			zap.String("user_id", userID),
			zap.Duration("ollama_duration", ollmaDuration),
			zap.Duration("total_duration_so_far", time.Since(startTime)),
			zap.Int("response_length", len(aiResult.Content)),
		)
		aiResponseContent = aiResult.Content
	}

	// Format AI response for better readability
	s.logger.Debug("Formatting AI response",
		zap.String("user_id", userID),
		zap.Int("response_length", len(aiResponseContent)),
	)

	formattedAIResponse := s.formatAIResponse(aiResponseContent)

	s.logger.Debug("AI response formatted",
		zap.String("user_id", userID),
		zap.Int("formatted_length", len(formattedAIResponse)),
	)

	// Create ConversationMessage objects for session storage (with timestamps)
	now := time.Now()

	sessionUserMessage := ConversationMessage{
		Role:      "user",
		Content:   message,                   // Already sanitized above
		Timestamp: now.Add(-1 * time.Minute), // User message slightly before AI response
	}

	sessionAIMessage := ConversationMessage{
		Role:      "assistant",
		Content:   aiResponseContent, // Store unformatted content for AI context
		Timestamp: now,
	}

	// Update conversation history
	updatedHistory := append(persistentHistory, sessionUserMessage, sessionAIMessage)

	s.logger.Debug("Attempting to save conversation history to session",
		zap.String("user_id", userID),
		zap.Int("history_length", len(updatedHistory)),
		zap.String("conversation_key", conversationKey),
	)

	s.sessionManager.Put(r.Context(), conversationKey, updatedHistory)

	// Also maintain a conversation index for the user
	conversationIndexKey := fmt.Sprintf("conversation_index_%s", userID)
	existingIndex := s.sessionManager.Get(r.Context(), conversationIndexKey)

	var conversationIndex []map[string]interface{}
	if existingIndex != nil {
		if index, ok := existingIndex.([]map[string]interface{}); ok {
			conversationIndex = index
		}
	}

	// Check if this conversation already exists in the index
	found := false
	for i := range conversationIndex {
		if conversationIndex[i]["id"] == conversationID {
			// Update existing conversation
			conversationIndex[i]["updated_at"] = time.Now()
			conversationIndex[i]["message_count"] = len(updatedHistory)
			if len(updatedHistory) > 0 {
				// Use first user message as title
				for _, msg := range updatedHistory {
					if msg.Role == "user" {
						title := msg.Content
						if len(title) > 50 {
							title = title[:47] + "..."
						}
						conversationIndex[i]["title"] = title
						break
					}
				}
			}
			found = true
			break
		}
	}

	// Add new conversation to index if not found
	if !found {
		title := "New Conversation"
		if len(updatedHistory) > 0 {
			// Use first user message as title
			for _, msg := range updatedHistory {
				if msg.Role == "user" {
					title = msg.Content
					if len(title) > 50 {
						title = title[:47] + "..."
					}
					break
				}
			}
		}

		conversationIndex = append(conversationIndex, map[string]interface{}{
			"id":            conversationID,
			"title":         title,
			"created_at":    time.Now(),
			"updated_at":    time.Now(),
			"message_count": len(updatedHistory),
		})
	}

	s.sessionManager.Put(r.Context(), conversationIndexKey, conversationIndex)

	s.logger.Info("Conversation history updated",
		zap.String("user_id", userID),
		zap.Int("total_messages", len(updatedHistory)),
		zap.String("conversation_key", conversationKey),
		zap.String("conversation_id", conversationID),
	)

	// Create formatted HTML response with only AI response (user message is already added by frontend)
	aiResponse := `<div class="chat-message ai-message" style="margin-bottom: 1rem;">
		<div style="display: flex; align-items: flex-start; gap: 0.75rem;">
			<div class="avatar ai-avatar" style="width: 2.5rem; height: 2.5rem; border-radius: 50%; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-weight: bold; flex-shrink: 0;">
				👨‍🍳
			</div>
			<div class="message-content" style="flex: 1; background: #ffffff; padding: 1rem; border-radius: 1rem; border: 1px solid #e2e8f0; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
				<div style="font-weight: 600; color: #4f46e5; margin-bottom: 0.5rem;">AI Chef</div>
				<div style="line-height: 1.6;">` + formattedAIResponse + `</div>
				<div style="font-size: 0.75rem; color: #9ca3af; margin-top: 0.5rem;">Just now</div>
			</div>
		</div>
	</div>`

	totalDuration := time.Since(startTime)
	s.logger.Info("AI Chat response completed",
		zap.String("user_id", userID),
		zap.Duration("total_duration", totalDuration),
		zap.Duration("ollama_duration", ollmaDuration),
		zap.Duration("validation_duration", validationTime),
		zap.Int("response_html_length", len(aiResponse)),
		zap.String("performance_summary", fmt.Sprintf("validation: %dms, ollama: %dms, total: %dms",
			validationTime.Milliseconds(),
			ollmaDuration.Milliseconds(),
			totalDuration.Milliseconds())),
	)

	s.logger.Info("AI Chat request completed successfully",
		zap.String("user_id", userID),
		zap.Duration("total_duration", time.Since(startTime)),
		zap.Int("response_size", len(aiResponse)),
	)

	// Set response headers and write the response
	w.Header().Set("Content-Type", "text/html")
	written, err := w.Write([]byte(aiResponse))
	if err != nil {
		s.logger.Error("Failed to write response",
			zap.Error(err),
			zap.String("user_id", userID),
		)
	} else {
		s.logger.Debug("Response written successfully",
			zap.String("user_id", userID),
			zap.Int("bytes_written", written),
		)
	}
}

func (s *WebServer) handleHTMXAIChatReset(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	s.logger.Info("AI Chat reset request started",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("timestamp", startTime.Format("15:04:05.000")),
	)

	// CRITICAL SECURITY FIX: Validate authentication (enforced by middleware)
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		s.logger.Warn("Unauthorized AI chat reset request", zap.Duration("duration", time.Since(startTime)))
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("<div class=\"error\">Authentication required to reset chat.</div>"))
		return
	}

	// Clear conversation history from session
	conversationKey := fmt.Sprintf("ai_conversation_%s", userID)
	s.sessionManager.Remove(r.Context(), conversationKey)

	s.logger.Info("AI Chat conversation reset",
		zap.String("user_id", userID),
		zap.String("conversation_key", conversationKey),
		zap.Duration("duration", time.Since(startTime)),
	)

	// Return success response
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<div id="chat-messages" style="min-height: 400px; max-height: 600px; overflow-y: auto; padding: 1rem; border: 1px solid #e2e8f0; border-radius: 0.5rem; margin-bottom: 1rem; background: #f8f9fa;">
		<div style="text-align: center; color: #718096; padding: 2rem;">
			<div style="font-size: 3rem; margin-bottom: 1rem;">👨‍🍳</div>
			<h3>Chat Reset Successfully!</h3>
			<p>Hello! I'm your AI Chef assistant. What can I help you cook today?</p>
		</div>
	</div>`))
}

// Multi-Chat API Handlers

func (s *WebServer) handleAPIConversationList(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		s.writeJSONError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Forward request to API server instead of calling convService directly
	err := s.apiClient.ForwardRequest(r.Context(), w, r, "/api/v3/chat/conversations")
	if err != nil {
		s.logger.Error("Failed to forward conversation list request",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (s *WebServer) handleAPIConversationHistory(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		s.writeJSONError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get conversation ID from query parameter and forward to proper endpoint
	conversationID := r.URL.Query().Get("conversation_id")
	if conversationID == "" {
		s.writeJSONError(w, "Conversation ID required", http.StatusBadRequest)
		return
	}

	// Forward request to API server messages endpoint
	err := s.apiClient.ForwardRequest(r.Context(), w, r, "/api/v3/chat/conversations/"+conversationID+"/messages")
	if err != nil {
		s.logger.Error("Failed to forward conversation history request",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (s *WebServer) handleAPIChatMessage(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		s.writeJSONError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get conversation ID from form data
	conversationID := r.FormValue("conversation_id")
	if conversationID == "" {
		s.writeJSONError(w, "Conversation ID required", http.StatusBadRequest)
		return
	}

	// Forward request to API server messages endpoint
	err := s.apiClient.ForwardRequest(r.Context(), w, r, "/api/v3/chat/conversations/"+conversationID+"/messages")
	if err != nil {
		s.logger.Error("Failed to forward chat message request",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (s *WebServer) handleAPIConversationRename(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		s.writeJSONError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get conversation ID from form data
	conversationID := r.FormValue("conversation_id")
	if conversationID == "" {
		s.writeJSONError(w, "Conversation ID required", http.StatusBadRequest)
		return
	}

	// Forward request to API server conversation endpoint
	err := s.apiClient.ForwardRequest(r.Context(), w, r, "/api/v3/chat/conversations/"+conversationID)
	if err != nil {
		s.logger.Error("Failed to forward conversation rename request",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (s *WebServer) handleAPIConversationDelete(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		s.writeJSONError(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Get conversation ID from form data
	conversationID := r.FormValue("conversation_id")
	if conversationID == "" {
		s.writeJSONError(w, "Conversation ID required", http.StatusBadRequest)
		return
	}

	// Forward request to API server conversation endpoint
	err := s.apiClient.ForwardRequest(r.Context(), w, r, "/api/v3/chat/conversations/"+conversationID)
	if err != nil {
		s.logger.Error("Failed to forward conversation delete request",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (s *WebServer) handleHTMXConversationList(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<div class="error">Please log in to view conversations</div>`))
		return
	}

	// Get conversation index from session storage
	conversationIndexKey := fmt.Sprintf("conversation_index_%s", userID)
	existingIndex := s.sessionManager.Get(r.Context(), conversationIndexKey)

	var conversationIndex []map[string]interface{}
	if existingIndex != nil {
		if index, ok := existingIndex.([]map[string]interface{}); ok {
			conversationIndex = index
		}
	}

	// If no conversations, show empty state
	if len(conversationIndex) == 0 {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<div style="padding: 2rem 1rem; text-align: center; color: #718096;">
				<div style="margin-bottom: 1rem;">💬</div>
				<p style="font-size: 0.875rem;">Start a new conversation with our AI Chef!</p>
			</div>
		`))
		return
	}

	// Generate HTML for conversation list (most recent first)
	html := ""
	for i := len(conversationIndex) - 1; i >= 0; i-- {
		conv := conversationIndex[i]
		title := conv["title"].(string)
		conversationID := conv["id"].(string)
		messageCount := conv["message_count"].(int)

		// Format time
		var timeStr string
		if updatedAt, ok := conv["updated_at"].(time.Time); ok {
			timeStr = s.formatTimeAgo(updatedAt)
		} else {
			timeStr = "Recently"
		}

		html += fmt.Sprintf(`
			<div class="conversation-item" data-conversation-id="%s" style="padding: 1rem; border-bottom: 1px solid #e2e8f0; transition: background-color 0.2s;">
				<div style="display: flex; justify-content: space-between; align-items: start;">
					<div style="flex: 1; min-width: 0;">
						<div style="font-weight: 500; font-size: 0.875rem; margin-bottom: 0.25rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">%s</div>
						<div style="color: #718096; font-size: 0.75rem;">%s • %d messages</div>
					</div>
					<div style="display: flex; gap: 0.5rem; opacity: 0;" class="conversation-actions">
						<button onclick="event.stopPropagation(); deleteConversation('%s')" title="Delete" style="background: none; border: none; cursor: pointer; font-size: 0.75rem;">
							🗑️
						</button>
					</div>
				</div>
			</div>`, conversationID, s.escapeHTML(title), timeStr, messageCount, conversationID)
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *WebServer) handleHTMXConversationHistory(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<div class="error">Please log in to view conversation history</div>`))
		return
	}

	// Get conversation ID from request (for loading specific conversation)
	conversationID := strings.TrimSpace(r.URL.Query().Get("conversation_id"))
	var conversationKey string

	if conversationID != "" {
		// Load specific conversation
		conversationKey = fmt.Sprintf("ai_conversation_%s_%s", userID, conversationID)
	} else {
		// For now, just show welcome message since we don't have active conversation tracking yet
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div id="chat-messages" style="min-height: 400px; max-height: 600px; overflow-y: auto; padding: 1rem; border: 1px solid #e2e8f0; border-radius: 0.5rem; margin-bottom: 1rem; background: #f8f9fa;">
			<div style="text-align: center; color: #718096; padding: 2rem;">
				<div style="font-size: 3rem; margin-bottom: 1rem;">👨‍🍳</div>
				<h3>Welcome to AI Chef Chat!</h3>
				<p>I'm here to help you with recipes, cooking tips, and culinary questions. What would you like to cook today?</p>
			</div>
		</div>`))
		return
	}

	// Get conversation history from session storage
	existingHistory := s.sessionManager.Get(r.Context(), conversationKey)

	var conversationHistory []conversation.ChatMessage
	if existingHistory != nil {
		if history, ok := existingHistory.([]conversation.ChatMessage); ok {
			conversationHistory = history
		}
	}

	// If no conversation history, show welcome message
	if len(conversationHistory) == 0 {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div id="chat-messages" style="min-height: 400px; max-height: 600px; overflow-y: auto; padding: 1rem; border: 1px solid #e2e8f0; border-radius: 0.5rem; margin-bottom: 1rem; background: #f8f9fa;">
			<div style="text-align: center; color: #718096; padding: 2rem;">
				<div style="font-size: 3rem; margin-bottom: 1rem;">👨‍🍳</div>
				<h3>Welcome to AI Chef Chat!</h3>
				<p>I'm here to help you with recipes, cooking tips, and culinary questions. What would you like to cook today?</p>
			</div>
		</div>`))
		return
	}

	// Generate HTML for conversation history
	html := `<div id="chat-messages" style="min-height: 400px; max-height: 600px; overflow-y: auto; padding: 1rem; border: 1px solid #e2e8f0; border-radius: 0.5rem; margin-bottom: 1rem; background: #f8f9fa;">`

	for _, msg := range conversationHistory {
		if msg.Role == "user" {
			escapedContent := s.escapeHTML(msg.Content)
			html += `<div class="chat-message user-message" style="margin-bottom: 1rem;">
				<div style="display: flex; justify-content: flex-end; gap: 0.75rem;">
					<div class="message-content" style="flex: 1; background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%); color: white; padding: 1rem; border-radius: 1rem; max-width: 80%;">
						<div style="font-weight: 600; margin-bottom: 0.5rem;">You</div>
						<div style="line-height: 1.6;">` + escapedContent + `</div>
						<div style="font-size: 0.75rem; opacity: 0.8; margin-top: 0.5rem;">Earlier</div>
					</div>
					<div class="avatar user-avatar" style="width: 2.5rem; height: 2.5rem; border-radius: 50%; background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%); display: flex; align-items: center; justify-content: center; color: white; font-weight: bold; flex-shrink: 0;">
						👤
					</div>
				</div>
			</div>`
		} else if msg.Role == "assistant" {
			formattedResponse := s.formatAIResponse(msg.Content)
			html += `<div class="chat-message ai-message" style="margin-bottom: 1rem;">
				<div style="display: flex; align-items: flex-start; gap: 0.75rem;">
					<div class="avatar ai-avatar" style="width: 2.5rem; height: 2.5rem; border-radius: 50%; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); display: flex; align-items: center; justify-content: center; color: white; font-weight: bold; flex-shrink: 0;">
						👨‍🍳
					</div>
					<div class="message-content" style="flex: 1; background: #ffffff; padding: 1rem; border-radius: 1rem; border: 1px solid #e2e8f0; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
						<div style="font-weight: 600; color: #4f46e5; margin-bottom: 0.5rem;">AI Chef</div>
						<div style="line-height: 1.6;">` + formattedResponse + `</div>
						<div style="font-size: 0.75rem; color: #9ca3af; margin-top: 0.5rem;">Earlier</div>
					</div>
				</div>
			</div>`
		}
	}

	html += `</div>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// Helper methods

func (s *WebServer) writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}

func (s *WebServer) formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "Just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	} else {
		return t.Format("Jan 2")
	}
}

func (s *WebServer) escapeHTML(input string) string {
	return html.EscapeString(input)
}

func (s *WebServer) handleHTMXRecipeSearch(w http.ResponseWriter, r *http.Request) {
	// SECURITY FIX ALV3-2025-005: Recipe Search now requires authentication (enforced by middleware)
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
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

	s.logger.Debug("Recipe search", zap.String("query", query), zap.String("user_id", userID))

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

// getUserContext extracts user information from SCS session for template rendering
func (s *WebServer) getUserContext(r *http.Request) (user interface{}, isAuthenticated bool) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	accessToken := s.sessionManager.GetString(r.Context(), "access_token")
	userName := s.sessionManager.GetString(r.Context(), "user_name")
	sessionToken := s.sessionManager.Token(r.Context())

	s.logger.Debug("getUserContext session data",
		zap.String("user_id", userID),
		zap.String("user_name", userName),
		zap.String("access_token_prefix", func() string {
			if len(accessToken) > 10 {
				return accessToken[:10] + "..."
			}
			return accessToken
		}()),
		zap.String("session_token", sessionToken),
	)

	if userID != "" && accessToken != "" {
		// Verify token is still valid with API
		if s.apiClient.VerifyToken(r.Context(), accessToken) {
			isAuthenticated = true
			user = map[string]interface{}{
				"ID":    userID,
				"Name":  userName,
				"Email": s.sessionManager.GetString(r.Context(), "user_email"),
			}
			s.logger.Debug("Token verification successful", zap.Bool("authenticated", true))
		} else {
			s.logger.Debug("Token verification failed", zap.Bool("authenticated", false))
		}
	} else {
		s.logger.Debug("Missing session data",
			zap.Bool("has_user_id", userID != ""),
			zap.Bool("has_access_token", accessToken != ""))
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

// resilientMiddleware wraps middleware to handle failures gracefully
func (s *WebServer) resilientMiddleware(name string, middleware func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add panic recovery for the middleware
			defer func() {
				if rec := recover(); rec != nil {
					s.logger.Error("Middleware panic recovered",
						zap.String("middleware", name),
						zap.Any("panic", rec),
						zap.String("path", r.URL.Path),
					)
					// Continue to next handler even if middleware fails
					next.ServeHTTP(w, r)
				}
			}()

			// Try to execute the middleware
			handler := middleware(next)
			handler.ServeHTTP(w, r)
		})
	}
}

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

		// Check if user has a valid session for CSRF protection
		userID := s.sessionManager.GetString(r.Context(), "user_id")
		if userID == "" {
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

		// Validate CSRF token using user ID (more secure than session ID)
		expectedToken := s.generateCSRFToken(userID)
		if !s.validateCSRFToken(token, expectedToken) {
			s.logger.Warn("Invalid CSRF token",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("user_id", userID),
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

// validateCSRFToken validates a CSRF token with time window
func (s *WebServer) validateCSRFToken(providedToken, expectedToken string) bool {
	// Parse the provided token to extract the timestamp
	parts := strings.Split(providedToken, ":")
	if len(parts) != 2 {
		return false
	}

	providedID := parts[0]
	providedTimestampStr := parts[1]

	// Parse the expected token to get the user ID
	expectedParts := strings.Split(expectedToken, ":")
	if len(expectedParts) != 2 {
		return false
	}
	expectedID := expectedParts[0]

	// Check if user IDs match
	if subtle.ConstantTimeCompare([]byte(providedID), []byte(expectedID)) != 1 {
		return false
	}

	// Parse and validate timestamp (allow 1 hour window)
	providedTimestamp, err := strconv.ParseInt(providedTimestampStr, 10, 64)
	if err != nil {
		return false
	}

	now := time.Now().Unix()
	timeDiff := now - providedTimestamp

	// Allow tokens within 1 hour (3600 seconds)
	return timeDiff >= 0 && timeDiff <= 3600
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

// formatAIResponse formats AI response text to preserve formatting and improve readability
func (s *WebServer) formatAIResponse(text string) string {
	// Escape HTML first to prevent XSS
	text = html.EscapeString(text)

	// Convert double line breaks to paragraphs
	text = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(text, "</p><p>")

	// Convert single line breaks to <br> tags
	text = strings.ReplaceAll(text, "\n", "<br>")

	// Format bullet points
	bulletPattern := regexp.MustCompile(`(?m)^[\s]*[-•*]\s+(.+)$`)
	text = bulletPattern.ReplaceAllString(text, `<div style="margin-left: 1rem; margin-bottom: 0.5rem;">• $1</div>`)

	// Format numbered lists
	numberedPattern := regexp.MustCompile(`(?m)^[\s]*(\d+)\.\s+(.+)$`)
	text = numberedPattern.ReplaceAllString(text, `<div style="margin-left: 1rem; margin-bottom: 0.5rem;">$1. $2</div>`)

	// Format bold text (basic markdown-style)
	boldPattern := regexp.MustCompile(`\*\*([^*]+)\*\*`)
	text = boldPattern.ReplaceAllString(text, `<strong>$1</strong>`)

	// Format italic text (basic markdown-style)
	italicPattern := regexp.MustCompile(`\*([^*]+)\*`)
	text = italicPattern.ReplaceAllString(text, `<em>$1</em>`)

	// Wrap in paragraphs if not already wrapped
	if !strings.Contains(text, "<p>") && !strings.Contains(text, "<div") {
		text = "<p>" + text + "</p>"
	} else if strings.HasPrefix(text, "</p>") {
		// Fix leading paragraph close tag
		text = "<p>" + strings.TrimPrefix(text, "</p>")
	}

	return text
}

// Dashboard HTMX handlers

func (s *WebServer) handleHTMXDashboardRecipes(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<div class="error">Authentication required</div>`))
		return
	}

	// Get user's token from session for API call
	token := s.sessionManager.GetString(r.Context(), "auth_token")
	if token == "" {
		w.Write([]byte(`<div style="text-align: center; padding: 2rem; color: #718096;">
			<p>No recipes found. <a href="/recipes">Browse recipes</a> to get started!</p>
		</div>`))
		return
	}

	// Fetch user's recipes from API
	recipes, err := s.apiClient.GetRecipes(r.Context(), token)
	if err != nil {
		s.logger.Error("Failed to fetch user recipes", zap.Error(err))
		w.Write([]byte(`<div style="text-align: center; padding: 2rem; color: #718096;">
			<p>Unable to load recipes right now. Please try again later.</p>
		</div>`))
		return
	}

	if len(recipes) == 0 {
		w.Write([]byte(`<div style="text-align: center; padding: 2rem; color: #718096;">
			<svg width="48" height="48" viewBox="0 0 24 24" fill="currentColor" style="margin-bottom: 1rem;">
				<path d="M12 2l3.09 6.26L22 9l-6.91 1.01L12 16l-3.09-5.99L2 9l6.91-1.01L12 2z"/>
			</svg>
			<h3>No recipes yet</h3>
			<p>Start by browsing our recipe collection!</p>
			<a href="/recipes" class="btn btn-primary" style="margin-top: 1rem;">Browse Recipes</a>
		</div>`))
		return
	}

	// Render recipe cards
	html := `<div class="recipe-grid" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1.5rem;">`
	for i, recipe := range recipes {
		if i >= 6 { // Limit to 6 recent recipes
			break
		}
		html += fmt.Sprintf(`
		<div class="recipe-card" style="border: 1px solid #e2e8f0; border-radius: 0.5rem; overflow: hidden; background: white; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
			<img src="%s" alt="%s" style="width: 100%%; height: 150px; object-fit: cover;">
			<div style="padding: 1rem;">
				<h4 style="margin: 0 0 0.5rem 0; font-size: 1rem;">%s</h4>
				<p style="color: #718096; font-size: 0.875rem; margin: 0 0 1rem 0;">%s</p>
				<div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.75rem; color: #718096;">
					<span>⭐ %.1f</span>
					<span>❤️ %d likes</span>
				</div>
			</div>
		</div>`,
			recipe.ImageURL,
			recipe.Title,
			recipe.Title,
			recipe.Description,
			recipe.Rating,
			recipe.Likes,
		)
	}
	html += `</div>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *WebServer) handleHTMXDashboardActivity(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<div class="error">Authentication required</div>`))
		return
	}

	// Mock activity data for now
	activities := []map[string]interface{}{
		{
			"type":    "like",
			"message": "Someone liked your recipe 'Chocolate Chip Cookies'",
			"time":    "2 hours ago",
		},
		{
			"type":    "comment",
			"message": "New comment on 'Italian Pasta'",
			"time":    "4 hours ago",
		},
		{
			"type":    "follow",
			"message": "ChefMaster started following you",
			"time":    "1 day ago",
		},
	}

	if len(activities) == 0 {
		w.Write([]byte(`<p style="color: #718096; text-align: center; padding: 1rem;">No recent activity</p>`))
		return
	}

	html := `<div style="display: flex; flex-direction: column; gap: 1rem;">`
	for _, activity := range activities {
		icon := "📝"
		if activity["type"] == "like" {
			icon = "❤️"
		} else if activity["type"] == "comment" {
			icon = "💬"
		} else if activity["type"] == "follow" {
			icon = "👤"
		}

		html += fmt.Sprintf(`
		<div style="display: flex; align-items: center; gap: 1rem; padding: 1rem; background: #f8f9fa; border-radius: 0.5rem;">
			<div style="width: 2rem; height: 2rem; border-radius: 50%%; background: #4f46e5; display: flex; align-items: center; justify-content: center; color: white; font-size: 0.875rem;">
				%s
			</div>
			<div style="flex: 1;">
				<div style="font-weight: 500;">%s</div>
				<div style="color: #718096; font-size: 0.875rem;">%s</div>
			</div>
		</div>`, icon, activity["message"], activity["time"])
	}
	html += `</div>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *WebServer) handleHTMXDashboardTrending(w http.ResponseWriter, r *http.Request) {
	// Mock trending recipes data
	trendingRecipes := []map[string]interface{}{
		{
			"id":         "1",
			"title":      "Classic Margherita Pizza",
			"imageURL":   "/static/img/placeholder-recipe.jpg",
			"likesCount": 234,
		},
		{
			"id":         "2",
			"title":      "Homemade Pasta Carbonara",
			"imageURL":   "/static/img/placeholder-recipe.jpg",
			"likesCount": 189,
		},
		{
			"id":         "3",
			"title":      "Chocolate Lava Cake",
			"imageURL":   "/static/img/placeholder-recipe.jpg",
			"likesCount": 156,
		},
	}

	if len(trendingRecipes) == 0 {
		w.Write([]byte(`<p style="color: #718096; font-size: 0.875rem;">Check back soon for trending recipes!</p>`))
		return
	}

	html := `<div style="display: flex; flex-direction: column; gap: 1rem;">`
	for _, recipe := range trendingRecipes {
		html += fmt.Sprintf(`
		<div style="display: flex; gap: 1rem;">
			<img 
				src="%s" 
				alt="%s"
				style="width: 3rem; height: 3rem; border-radius: 0.5rem; object-fit: cover;"
			>
			<div style="flex: 1;">
				<a href="/recipes/%s" style="text-decoration: none; color: #1a202c;">
					<div style="font-weight: 500; font-size: 0.875rem;">%s</div>
				</a>
				<div style="color: #718096; font-size: 0.75rem;">%d likes</div>
			</div>
		</div>`,
			recipe["imageURL"],
			recipe["title"],
			recipe["id"],
			recipe["title"],
			recipe["likesCount"],
		)
	}
	html += `</div>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *WebServer) handleHTMXDashboardCollections(w http.ResponseWriter, r *http.Request) {
	userID := s.sessionManager.GetString(r.Context(), "user_id")
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`<div class="error">Authentication required</div>`))
		return
	}

	// Mock collections data for now
	collections := []map[string]interface{}{
		{
			"id":          "1",
			"name":        "Favorite Desserts",
			"recipeCount": 12,
		},
		{
			"id":          "2",
			"name":        "Quick Weeknight Meals",
			"recipeCount": 8,
		},
		{
			"id":          "3",
			"name":        "Holiday Specials",
			"recipeCount": 5,
		},
	}

	if len(collections) == 0 {
		w.Write([]byte(`<p style="color: #718096; font-size: 0.875rem;">No collections yet. Create one to organize your favorite recipes!</p>`))
		return
	}

	html := `<div style="display: flex; flex-direction: column; gap: 0.5rem;">`
	for _, collection := range collections {
		html += fmt.Sprintf(`
		<a href="/collections/%s" style="text-decoration: none; color: #1a202c;">
			<div style="padding: 0.75rem; background: #f8f9fa; border-radius: 0.5rem; border: 1px solid #e2e8f0;">
				<div style="font-weight: 500; font-size: 0.875rem;">%s</div>
				<div style="color: #718096; font-size: 0.75rem;">%d recipes</div>
			</div>
		</a>`,
			collection["id"],
			collection["name"],
			collection["recipeCount"],
		)
	}
	html += `</div>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
