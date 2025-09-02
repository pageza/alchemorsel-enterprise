// Package main provides the entry point for Alchemorsel v3 Web Frontend Service
// This service handles HTMX templates and communicates with the Pure API backend
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/http/webserver"
	"github.com/alchemorsel/v3/pkg/healthcheck"
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
// @host localhost:3014
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

	fmt.Println("🌐 Web Frontend Service - HTMX Templates & Interactive UI")
	fmt.Println("📡 Communicates with API Backend at http://api:8080")
	fmt.Println("🚀 Enterprise Architecture: Separated Frontend Service")
	fmt.Println("🎨 Serves HTML, CSS, JS - No business logic (API calls only)")
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
		
		// Health Check
		fx.Provide(func(cfg *config.Config, log *zap.Logger) *healthcheck.EnterpriseHealthCheck {
			if cfg.Monitoring.HealthCheck.EnableEnterprise {
				hc := healthcheck.NewEnterpriseHealthCheck(cfg.App.Version, log)
				hc.HealthCheck.SetCacheTTL(cfg.Monitoring.HealthCheck.CacheTTL)
				return hc
			}
			// Return basic health check wrapped in enterprise interface
			basic := healthcheck.New(cfg.App.Version, log)
			basic.SetCacheTTL(cfg.Monitoring.HealthCheck.CacheTTL)
			return &healthcheck.EnterpriseHealthCheck{HealthCheck: basic}
		}),
		
		// Web Server
		fx.Provide(webserver.NewWebServer),
		
		// Lifecycle
		fx.Invoke(registerLifecycleHooks),
		fx.Invoke(initializeWebHealthChecks),
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
			
			// Start server in a goroutine
			go func() {
				log.Info("Starting web server listener")
				if err := server.Start(); err != nil && err != http.ErrServerClosed {
					log.Error("Web server error", zap.Error(err))
				}
			}()
			
			// Server started successfully (will block in the goroutine)
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
	// Check environment variable first (set in docker-compose.yml)
	if apiURL := os.Getenv("API_URL"); apiURL != "" {
		return apiURL
	}
	
	// Default for Docker Compose (api service name)
	return "http://api:8080"
}

// initializeWebHealthChecks registers health checks for the web service
func initializeWebHealthChecks(
	cfg *config.Config,
	log *zap.Logger,
	hc *healthcheck.EnterpriseHealthCheck,
	apiClient *webserver.APIClient,
) {
	log.Info("Initializing web service health checks")
	
	// Register system checker
	systemChecker := healthcheck.NewCustomChecker("system", func(ctx context.Context) (healthcheck.Status, string, interface{}) {
		return healthcheck.StatusHealthy, "System operational", map[string]interface{}{
			"service": "alchemorsel-web",
			"version": cfg.App.Version,
			"environment": cfg.App.Environment,
		}
	})
	
	// Register API dependency checker
	apiChecker := healthcheck.NewCustomChecker("api_backend", func(ctx context.Context) (healthcheck.Status, string, interface{}) {
		if apiClient.VerifyConnection(ctx) {
			return healthcheck.StatusHealthy, "API backend accessible", map[string]interface{}{
				"api_url": getAPIURL(cfg),
			}
		}
		return healthcheck.StatusUnhealthy, "API backend not accessible", map[string]interface{}{
			"api_url": getAPIURL(cfg),
		}
	})
	
	if cfg.Monitoring.HealthCheck.EnableCircuitBreaker {
		circuitConfig := healthcheck.CircuitBreakerConfig{
			FailureThreshold: cfg.Monitoring.HealthCheck.CircuitBreaker.FailureThreshold,
			SuccessThreshold: cfg.Monitoring.HealthCheck.CircuitBreaker.SuccessThreshold,
			Timeout:         cfg.Monitoring.HealthCheck.CircuitBreaker.Timeout,
			MaxRequests:     cfg.Monitoring.HealthCheck.CircuitBreaker.MaxRequests,
		}
		hc.RegisterWithCircuitBreaker("system", systemChecker, circuitConfig)
		hc.RegisterWithCircuitBreaker("api_backend", apiChecker, circuitConfig)
	} else {
		hc.Register("system", systemChecker)
		hc.Register("api_backend", apiChecker)
	}
	
	// Register dependencies if enabled
	if cfg.Monitoring.HealthCheck.EnableDependencies {
		// Register API backend as a dependency
		apiDep := healthcheck.ServiceDependency("api_backend", true, []string{}, apiChecker)
		hc.RegisterDependency(apiDep)
		
		log.Info("Registered web service dependencies")
	}
	
	log.Info("Web service health checks initialized successfully",
		zap.Bool("circuit_breaker", cfg.Monitoring.HealthCheck.EnableCircuitBreaker),
		zap.Bool("dependencies", cfg.Monitoring.HealthCheck.EnableDependencies),
		zap.Bool("metrics", cfg.Monitoring.HealthCheck.EnableMetrics),
	)
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