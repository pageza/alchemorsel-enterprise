// Package main provides the real Alchemorsel v3 application
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alchemorsel/v3/internal/application/recipe"
	"github.com/alchemorsel/v3/internal/application/user"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/sqlite"
	"github.com/alchemorsel/v3/internal/infrastructure/http/server"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"go.uber.org/zap"
)


func main() {
	fmt.Println(`
 █████╗ ██╗      ██████╗██╗  ██╗███████╗███╗   ███╗ ██████╗ ██████╗ ███████╗███████╗██╗     
██╔══██╗██║     ██╔════╝██║  ██║██╔════╝████╗ ████║██╔═══██╗██╔══██╗██╔════╝██╔════╝██║     
███████║██║     ██║     ███████║█████╗  ██╔████╔██║██║   ██║██████╔╝███████╗█████╗  ██║     
██╔══██║██║     ██║     ██╔══██║██╔══╝  ██║╚██╔╝██║██║   ██║██╔══██╗╚════██║██╔══╝  ██║     
██║  ██║███████╗╚██████╗██║  ██║███████╗██║ ╚═╝ ██║╚██████╔╝██║  ██║███████║███████╗███████╗
╚═╝  ╚═╝╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝
                                      v3.0.0 - Enterprise Recipe Platform                                      
	`)

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Override port from environment if set
	if port := os.Getenv("PORT"); port != "" {
		cfg.Server.Port = parsePort(port)
	}

	// Initialize database - use SQLite for demo
	logger.Info("Initializing SQLite database for demo")
	db, err := sqlite.SetupDatabase(":memory:", 0)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	
	// Seed demo data
	err = sqlite.SeedDatabase(db)
	if err != nil {
		logger.Fatal("Failed to seed database", zap.Error(err))
	}

	// Initialize services
	recipeService := recipe.NewService(db, logger)
	userService := user.NewUserService(db, logger)
	authService := security.NewAuthService(cfg.Security.JWT.Secret, logger)


	// Create HTTP server
	httpServer := server.NewServer(cfg, logger, recipeService, userService, authService)

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("Starting Alchemorsel v3 server",
			zap.Int("port", cfg.Server.Port),
			zap.String("environment", cfg.App.Environment),
		)
		
		fmt.Printf("🚀 Alchemorsel v3 server starting on http://localhost:%d\n", cfg.Server.Port)
		fmt.Println("✅ Features: Complete HTMX Frontend, User Auth, AI Chat, Recipe Management")
		fmt.Println("👤 Demo accounts: chef@alchemorsel.com / user@alchemorsel.com (password: password)")
		fmt.Println("🤖 Try the AI chat interface and recipe generation!")

		if err := httpServer.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	<-quit
	logger.Info("Shutting down server...")

	// Create shutdown context
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

func parsePort(portStr string) int {
	// Simple port parsing - default to 8080 if invalid
	if portStr == "" {
		return 8080
	}
	// Convert string to int, default to 8080 if invalid
	port := 8080
	fmt.Sscanf(portStr, "%d", &port)
	return port
}


