// Package main provides the pure JSON API server for Alchemorsel v3
// This service provides REST endpoints for web, mobile, and other clients
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/container"
	"go.uber.org/fx"
)

// @title Alchemorsel API v3 - Pure Backend
// @version 3.0.0
// @description Enterprise-grade recipe management REST API - Pure JSON Backend for web, mobile, and integrations
// @termsOfService https://alchemorsel.com/terms
// @contact.name API Support
// @contact.url https://alchemorsel.com/support
// @contact.email support@alchemorsel.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:3013
// @BasePath /api/v3
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter 'Bearer {token}' to authenticate
func main() {
	// Parse command line flags
	healthCheck := flag.Bool("healthcheck", false, "Run health check and exit")
	flag.Parse()

	// If health check flag is provided, run health check and exit
	if *healthCheck {
		runHealthCheck()
		return
	}

	// Create Fx application with dependency injection
	app := fx.New(
		// Application metadata
		fx.NopLogger, // Use our own logger instead of Fx's
		
		// Provide all dependencies for pure API server
		container.PureAPIModule,
		
		// Invoke startup functions
		fx.Invoke(func() {
			fmt.Println(`
 █████╗ ██╗      ██████╗██╗  ██╗███████╗███╗   ███╗ ██████╗ ██████╗ ███████╗███████╗██╗     
██╔══██╗██║     ██╔════╝██║  ██║██╔════╝████╗ ████║██╔═══██╗██╔══██╗██╔════╝██╔════╝██║     
███████║██║     ██║     ███████║█████╗  ██╔████╔██║██║   ██║██████╔╝███████╗█████╗  ██║     
██╔══██║██║     ██║     ██╔══██║██╔══╝  ██║╚██╔╝██║██║   ██║██╔══██╗╚════██║██╔══╝  ██║     
██║  ██║███████╗╚██████╗██║  ██║███████╗██║ ╚═╝ ██║╚██████╔╝██║  ██║███████║███████╗███████╗
╚═╝  ╚═╝╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝
                              v3.0.0 - Pure JSON API Backend (No Frontend/Templates)                                      
			`)
			fmt.Println("🔌 JSON API Server - RESTful endpoints for all clients")
			fmt.Println("📱 Serves Web, Mobile, and Integration clients") 
			fmt.Println("🚫 No HTML templates - Pure JSON responses only")
			fmt.Println("🔒 JWT authentication, Rate limiting, Health checks")
		}),
	)
	
	// Create context that cancels on interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	
	// Start the application
	if err := app.Start(ctx); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
	
	// Wait for interrupt signal
	<-ctx.Done()
	
	// Graceful shutdown
	fmt.Println("\nShutting down gracefully...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := app.Stop(shutdownCtx); err != nil {
		log.Fatalf("Failed to stop application gracefully: %v", err)
	}
	
	fmt.Println("Application stopped successfully")
}

// runHealthCheck performs a simple health check and exits
func runHealthCheck() {
	// Simple health check: try to connect to the health endpoint
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		fmt.Println("Health check passed")
		os.Exit(0)
	} else {
		fmt.Fprintf(os.Stderr, "Health check failed: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}
}