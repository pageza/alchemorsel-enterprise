// Package main provides the main entry point for the Alchemorsel API server
// This demonstrates clean architecture with proper dependency injection
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/container"
	"go.uber.org/fx"
)

// @title Alchemorsel API v3
// @version 3.0.0
// @description Enterprise-grade recipe management platform with AI capabilities
// @termsOfService https://alchemorsel.com/terms
// @contact.name API Support
// @contact.url https://alchemorsel.com/support
// @contact.email support@alchemorsel.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /api/v3
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter 'Bearer {token}' to authenticate
func main() {
	// Create Fx application with dependency injection
	app := fx.New(
		// Application metadata
		fx.NopLogger, // Use our own logger instead of Fx's
		
		// Provide all dependencies
		container.Module,
		
		// Invoke startup functions
		fx.Invoke(func() {
			fmt.Println(`
 █████╗ ██╗      ██████╗██╗  ██╗███████╗███╗   ███╗ ██████╗ ██████╗ ███████╗███████╗██╗     
██╔══██╗██║     ██╔════╝██║  ██║██╔════╝████╗ ████║██╔═══██╗██╔══██╗██╔════╝██╔════╝██║     
███████║██║     ██║     ███████║█████╗  ██╔████╔██║██║   ██║██████╔╝███████╗█████╗  ██║     
██╔══██║██║     ██║     ██╔══██║██╔══╝  ██║╚██╔╝██║██║   ██║██╔══██╗╚════██║██╔══╝  ██║     
██║  ██║███████╗╚██████╗██║  ██║███████╗██║ ╚═╝ ██║╚██████╔╝██║  ██║███████║███████╗███████╗
╚═╝  ╚═╝╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝
                                      v3.0.0 - Enterprise Recipe Platform                                      
			`)
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