// Package main provides the entry point for Alchemorsel v3 Web Frontend Service
// This service handles HTMX templates and communicates with the Pure API backend
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/http/webserver"
	"github.com/alchemorsel/v3/pkg/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// @title Alchemorsel Web Frontend v3
// @version 3.0.0
// @description Enterprise-grade recipe management web frontend with HTMX
// @termsOfService https://alchemorsel.com/terms
// @contact.name API Support
// @contact.url https://alchemorsel.com/support
// @contact.email support@alchemorsel.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @BasePath /

func main() {
	// Print startup banner
	fmt.Println(`
 █████╗ ██╗      ██████╗██╗  ██╗███████╗███╗   ███╗ ██████╗ ██████╗ ███████╗███████╗██╗      
██╔══██╗██║     ██╔════╝██║  ██║██╔════╝████╗ ████║██╔═══██╗██╔══██╗██╔════╝██╔════╝██║     
███████║██║     ██║     ███████║█████╗  ██╔████╔██║██║   ██║██████╔╝███████╗█████╗  ██║     
██╔══██║██║     ██║     ██╔══██║██╔══╝  ██║╚██╔╝██║██║   ██║██╔══██╗╚════██║██╔══╝  ██║     
██║  ██║███████╗╚██████╗██║  ██║███████╗██║ ╚═╝ ██║╚██████╔╝██║  ██║███████║███████╗███████╗
╚═╝  ╚═╝╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝
                                   v3.0.0 - Web Frontend Service                                      
	`)

	fmt.Println("🌐 Web Frontend Service - HTMX Templates")
	fmt.Println("📡 Communicates with Pure API Backend")
	fmt.Println("🚀 Enterprise Architecture with Service Separation")
	fmt.Println()

	// Create FX application for web frontend
	app := fx.New(
		fx.NopLogger,
		
		// Configuration
		fx.Provide(func() (*config.Config, error) {
			return config.Load("")
		}),
		
		// Logger
		fx.Provide(func(cfg *config.Config) (*zap.Logger, error) {
			return logger.New(logger.Config{
				Level:       cfg.App.LogLevel,
				Format:      cfg.App.LogFormat,
				Development: cfg.App.Debug,
			})
		}),
		
		// API Client for backend communication
		fx.Provide(webserver.NewAPIClient),
		
		// Session Store
		fx.Provide(webserver.NewSessionStore),
		
		// Web Server
		fx.Provide(webserver.NewWebServer),
		
		// Lifecycle
		fx.Invoke(registerLifecycleHooks),
	)

	// Run the application
	app.Run()
}

func registerLifecycleHooks(
	lc fx.Lifecycle,
	cfg *config.Config,
	log *zap.Logger,
	server *webserver.WebServer,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Override port from environment if set
			if port := os.Getenv("PORT"); port != "" {
				cfg.Server.Port = parsePort(port)
			}
			
			// Use different port for web frontend (8080 default)
			if cfg.Server.Port == 3000 {
				cfg.Server.Port = 8080
			}
			
			log.Info("Starting Web Frontend server",
				zap.Int("port", cfg.Server.Port),
				zap.String("environment", cfg.App.Environment),
				zap.String("api_url", getAPIURL(cfg)),
			)
			
			fmt.Printf("🚀 Alchemorsel v3 Web Frontend starting on http://localhost:%d\n", cfg.Server.Port)
			fmt.Printf("🔗 Connected to API Backend at %s\n", getAPIURL(cfg))
			fmt.Println("🎨 HTMX-powered interactive UI")
			fmt.Println("🍳 Recipe management with AI capabilities")
			
			// Start server in background
			go func() {
				if err := server.Start(); err != nil {
					log.Fatal("Web server failed to start", zap.Error(err))
				}
			}()
			
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Shutting down Web Frontend server...")
			return server.Shutdown(ctx)
		},
	})
}

func parsePort(portStr string) int {
	if portStr == "" {
		return 8080
	}
	port := 8080
	fmt.Sscanf(portStr, "%d", &port)
	return port
}

func getAPIURL(cfg *config.Config) string {
	// Check environment variable first
	if apiURL := os.Getenv("API_URL"); apiURL != "" {
		return apiURL
	}
	
	// Default to localhost with API port
	return fmt.Sprintf("http://localhost:3000")
}

func setupGracefulShutdown(log *zap.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Info("Received shutdown signal, gracefully stopping...")
		
		// Give the application time to cleanup
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// The FX framework will handle the actual shutdown
		_ = ctx
	}()
}