// Package main provides the pure JSON API server for Alchemorsel v3
// This is the enterprise backend API without any frontend/template handling
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

// @title Alchemorsel API v3 - Pure Backend
// @version 3.0.0
// @description Enterprise-grade recipe management API with AI capabilities - Pure JSON Backend
// @termsOfService https://alchemorsel.com/terms
// @contact.name API Support
// @contact.url https://alchemorsel.com/support  
// @contact.email support@alchemorsel.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:3000
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter 'Bearer {token}' to authenticate
func main() {
	// Create Fx application with dependency injection for pure API
	app := fx.New(
		// Application metadata
		fx.NopLogger, // Use our own logger instead of Fx's
		
		// Provide all dependencies for pure API
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
                                   v3.0.0 - Pure JSON API Backend                                      
			`)
			fmt.Println("🔥 Pure API Backend - No Frontend Templates")
			fmt.Println("📊 Enterprise Architecture with DI Container")
			fmt.Println("🚀 JSON-only responses for maximum flexibility")
		}),
	)
	
	// Create context that cancels on interrupt
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	
	// Start the application
	if err := app.Start(ctx); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}
	
	// Wait for interrupt signal
	<-ctx.Done()
	
	// Graceful shutdown
	fmt.Println("\nShutting down API server gracefully...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := app.Stop(shutdownCtx); err != nil {
		log.Fatalf("Failed to stop API server gracefully: %v", err)
	}
	
	fmt.Println("API server stopped successfully")
}