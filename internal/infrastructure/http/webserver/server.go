// Package webserver provides the web frontend HTTP server implementation
package webserver

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
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
	config      *config.Config
	logger      *zap.Logger
	server      *http.Server
	router      *chi.Mux
	apiClient   *APIClient
	sessionStore *SessionStore
	templates   *template.Template
}

// NewWebServer creates a new web frontend server instance
func NewWebServer(
	cfg *config.Config,
	log *zap.Logger,
	apiClient *APIClient,
	sessionStore *SessionStore,
) (*WebServer, error) {
	// Parse templates
	log.Info("Parsing templates...")
	templates, err := parseTemplates()
	if err != nil {
		log.Error("Failed to parse templates", zap.Error(err))
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	log.Info("Templates parsed successfully")

	server := &WebServer{
		config:       cfg,
		logger:       log,
		apiClient:    apiClient,
		sessionStore: sessionStore,
		templates:    templates,
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

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))
	r.Use(s.sessionMiddleware)

	// Static files - serve directly from file system for development
	localStaticDir := "/home/hermes/alchemorsel-v3/internal/infrastructure/http/webserver/static/"
	fileServer := http.FileServer(http.Dir(localStaticDir))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))
	
	// Health check
	r.Get("/health", s.handleHealthCheck)

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

	// HTMX endpoints (partial templates) - Protected by authentication
	r.Route("/htmx", func(r chi.Router) {
		// Add authentication middleware to all HTMX routes
		r.Use(s.requireAuth)
		
		r.Post("/search", s.handleHTMXSearch)
		r.Post("/recipes/{id}/like", s.handleHTMXLike)
		r.Post("/recipes/{id}/rate", s.handleHTMXRate)
		r.Get("/recipes/{id}/comments", s.handleHTMXComments)
		r.Post("/recipes/{id}/comments", s.handleHTMXAddComment)
		r.Get("/notifications", s.handleHTMXNotifications)
		
		// AI Chat endpoints - Require authentication
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

// parseTemplates parses all HTML templates from the server templates directory
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

	// Template directory path (absolute from project root)
	templateDir := "/home/hermes/alchemorsel-v3/internal/infrastructure/http/server/templates"
	
	// Collect all template files first
	var allFiles []string
	patterns := []string{
		filepath.Join(templateDir, "layout/*.html"),
		filepath.Join(templateDir, "components/*.html"),
		filepath.Join(templateDir, "pages/*.html"),
		filepath.Join(templateDir, "partials/*.html"),
	}
	
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
		}
		allFiles = append(allFiles, matches...)
	}
	
	if len(allFiles) == 0 {
		return nil, fmt.Errorf("no template files found in %s", templateDir)
	}

	// Parse all template files at once to handle template dependencies correctly
	tmpl, err := template.New("base").Funcs(funcMap).ParseFiles(allFiles...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
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
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "alchemorsel-web",
		"version":   "3.0.0",
		"timestamp": time.Now().Unix(),
		"mode":      "web-frontend",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	fmt.Fprintf(w, `{"status":"%s","service":"%s","version":"%s","timestamp":%d,"mode":"%s"}`,
		response["status"], response["service"], response["version"], 
		response["timestamp"], response["mode"])
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
	// Clear session
	session := r.Context().Value("session").(*Session)
	session.Clear()
	session.Save(w)

	// Redirect to home
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *WebServer) handleRecipeList(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*Session)
	
	// Get recipes from API
	recipes, err := s.apiClient.GetRecipes(r.Context(), session.AccessToken)
	if err != nil {
		s.renderError(w, "Failed to load recipes", err)
		return
	}

	s.renderTemplate(w, "recipes", map[string]interface{}{
		"Title":   "Recipes - Alchemorsel",
		"Recipes": recipes,
	})
}

func (s *WebServer) handleNewRecipePage(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "recipe-new", map[string]interface{}{
		"Title": "New Recipe - Alchemorsel",
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

func (s *WebServer) handleProfile(w http.ResponseWriter, r *http.Request) {
	s.renderTemplate(w, "profile", map[string]interface{}{
		"Title": "Profile - Alchemorsel",
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
	// Get the message from form
	message := r.FormValue("message")
	if message == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<div class=\"error\">Message is required</div>"))
		return
	}

	s.logger.Debug("AI Chat request", zap.String("message", message))

	// TODO: Call AI service to get response
	// For now, return a mock response
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
	query := r.FormValue("q")
	if query == "" {
		w.Write([]byte("<div>Please enter a search term</div>"))
		return
	}

	s.logger.Debug("Recipe search", zap.String("query", query))

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

func (s *WebServer) renderError(w http.ResponseWriter, message string, err error) {
	s.logger.Error(message, zap.Error(err))
	w.WriteHeader(http.StatusInternalServerError)
	s.renderTemplate(w, "error", map[string]interface{}{
		"Title":   "Error - Alchemorsel",
		"Message": message,
	})
}