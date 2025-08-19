// Package apiserver provides a pure JSON API HTTP server implementation
package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/alchemorsel/v3/internal/application/user"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/http/handlers"
	"github.com/alchemorsel/v3/internal/infrastructure/http/middleware"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"github.com/alchemorsel/v3/internal/ports/inbound"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// PureAPIServer represents a pure JSON API HTTP server (no frontend templates)
type PureAPIServer struct {
	config        *config.Config
	logger        *zap.Logger
	server        *http.Server
	router        *chi.Mux
	recipeService inbound.RecipeService
	userService   *user.UserService
	authService   *security.AuthService
	aiService     outbound.AIService
	openAPIHandler *OpenAPIHandler
}

// NewPureAPIServer creates a new pure API server instance
func NewPureAPIServer(
	cfg *config.Config,
	log *zap.Logger,
	recipeService inbound.RecipeService,
	userService *user.UserService,
	authService *security.AuthService,
	aiService outbound.AIService,
) *PureAPIServer {
	server := &PureAPIServer{
		config:        cfg,
		logger:        log,
		recipeService: recipeService,
		userService:   userService,
		authService:   authService,
		aiService:     aiService,
		openAPIHandler: NewOpenAPIHandler(log),
	}

	server.router = server.setupRoutes()
	server.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      server.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server
}

// setupRoutes configures pure JSON API routes
func (s *PureAPIServer) setupRoutes() *chi.Mux {
	r := chi.NewRouter()

	// Global middleware for API
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger(s.logger))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Security())
	r.Use(middleware.CORS())
	
	// API-specific middleware
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(chimiddleware.Compress(5))
	r.Use(middleware.JSONOnly()) // Force JSON responses only

	// Health check endpoint
	r.Get("/health", s.handleHealthCheck)

	// OpenAPI Documentation endpoints
	r.Get("/api/v1/openapi.yaml", s.openAPIHandler.ServeOpenAPISpec)
	r.Get("/api/v1/openapi.json", s.openAPIHandler.ServeOpenAPIJSON)
	r.Get("/api/v1/docs", s.openAPIHandler.ServeSwaggerUI)
	r.Get("/api/v1/docs/swagger", s.openAPIHandler.ServeSwaggerUI)
	r.Get("/api/v1/docs/redoc", s.openAPIHandler.ServeRedocUI)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		s.setupAPIV1Routes(r)
	})

	return r
}

// setupAPIV1Routes configures API v1 endpoints
func (s *PureAPIServer) setupAPIV1Routes(r chi.Router) {
	h := handlers.NewAPIHandlers(s.recipeService, s.logger)
	authH := handlers.NewAuthAPIHandlers(s.userService, s.authService, s.logger)
	aiH := handlers.NewAIAPIHandlers(s.aiService, s.logger)

	// Authentication routes
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authH.Register)
		r.Post("/login", authH.Login) 
		r.Post("/logout", authH.Logout)
		r.Post("/refresh", authH.RefreshToken)
		
		// Protected auth routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthenticateAPI(s.authService))
			r.Get("/profile", authH.GetProfile)
			r.Put("/profile", authH.UpdateProfile)
		})
	})

	// Recipe routes
	r.Route("/recipes", func(r chi.Router) {
		// Public routes
		r.Get("/", h.ListRecipes)
		r.Get("/{id}", h.GetRecipe)
		
		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthenticateAPI(s.authService))
			r.Post("/", h.CreateRecipe)
			r.Put("/{id}", h.UpdateRecipe)
			r.Delete("/{id}", h.DeleteRecipe)
			r.Post("/{id}/like", h.LikeRecipe)
			r.Post("/{id}/rating", h.RateRecipe)
		})
	})

	// AI routes
	r.Route("/ai", func(r chi.Router) {
		r.Use(middleware.AuthenticateAPI(s.authService))
		r.Post("/generate-recipe", aiH.GenerateRecipe)
		r.Post("/suggest-ingredients", aiH.SuggestIngredients)
		r.Post("/analyze-nutrition", aiH.AnalyzeNutrition)
	})

	// User routes  
	r.Route("/users", func(r chi.Router) {
		r.Use(middleware.AuthenticateAPI(s.authService))
		r.Get("/{id}/recipes", h.GetUserRecipes)
		r.Get("/{id}/favorites", h.GetUserFavorites)
	})

	// Health check
	r.Get("/health", h.HealthCheck)
}

// Start starts the pure API HTTP server
func (s *PureAPIServer) Start() error {
	s.logger.Info("Starting Pure JSON API server",
		zap.String("address", s.server.Addr),
		zap.String("mode", "API-only"),
	)

	return s.server.ListenAndServe()
}

// Server returns the underlying HTTP server instance
func (s *PureAPIServer) Server() *http.Server {
	return s.server
}

// Shutdown gracefully shuts down the pure API server
func (s *PureAPIServer) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down Pure API server...")
	return s.server.Shutdown(ctx)
}

// handleHealthCheck provides health check endpoint
func (s *PureAPIServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "alchemorsel-api",
		"version":   "3.0.0",
		"timestamp": time.Now().Unix(),
		"mode":      "pure-api",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// Simple JSON encoding for health check
	fmt.Fprintf(w, `{"status":"%s","service":"%s","version":"%s","timestamp":%d,"mode":"%s"}`,
		response["status"], response["service"], response["version"], 
		response["timestamp"], response["mode"])
}

// Deprecated: Use OpenAPI documentation endpoints instead