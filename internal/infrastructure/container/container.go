// Package container provides dependency injection using Uber FX
// This implements the Dependency Inversion Principle from SOLID
package container

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alchemorsel/v3/internal/application/recipe"
	"github.com/alchemorsel/v3/internal/application/user"
	"github.com/alchemorsel/v3/internal/infrastructure/ai/openai"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/http/apiserver"
	"github.com/alchemorsel/v3/internal/infrastructure/http/server"
	gormRepo "github.com/alchemorsel/v3/internal/infrastructure/persistence/gorm"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/memory"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/postgres"
	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"github.com/alchemorsel/v3/internal/ports/inbound"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/alchemorsel/v3/pkg/healthcheck"
	"github.com/alchemorsel/v3/pkg/logger"
	
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Module provides all dependency injection modules
var Module = fx.Options(
	// Infrastructure modules
	ConfigModule,
	LoggerModule,
	DatabaseModule,
	CacheModule,
	
	// Health check module
	HealthCheckModule,
	
	// Repository modules
	RepositoryModule,
	
	// Service modules
	ServiceModule,
	
	// HTTP modules
	HTTPModule,
	
	// Event modules
	EventModule,
	
	// Lifecycle hooks
	LifecycleModule,
)

// ConfigModule provides configuration
var ConfigModule = fx.Provide(
	func() (*config.Config, error) {
		return config.Load("")
	},
)

// LoggerModule provides logging
var LoggerModule = fx.Provide(
	func(cfg *config.Config) (*zap.Logger, error) {
		return logger.New(logger.Config{
			Level:       cfg.App.LogLevel,
			Format:      cfg.App.LogFormat,
			Development: cfg.App.Debug,
		})
	},
	// Provide sugared logger
	func(log *zap.Logger) *zap.SugaredLogger {
		return log.Sugar()
	},
)

// DatabaseModule provides database connections with PostgreSQL and performance optimization
var DatabaseModule = fx.Provide(
	// PostgreSQL database with performance optimization
	func(cfg *config.Config, log *zap.Logger) (*gorm.DB, error) {
		// Import PostgreSQL connection manager
		pgPkg := "github.com/alchemorsel/v3/internal/infrastructure/persistence/postgres"
		_ = pgPkg // Ensure import
		
		// Create PostgreSQL connection manager with optimized settings
		connectionManager, err := postgres.NewConnectionManager(cfg, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create PostgreSQL connection manager: %w", err)
		}

		db := connectionManager.GetDB()
		
		// Auto-migrate models if enabled
		if cfg.Database.AutoMigrate {
			if err := db.AutoMigrate(
				&gormRepo.UserModel{},
				&gormRepo.RecipeModel{},
				&gormRepo.RatingModel{},
				&gormRepo.AIRequestModel{},
				&gormRepo.RecipeLikeModel{},
				&gormRepo.UserFollowModel{},
				&gormRepo.CollectionModel{},
				&gormRepo.CollectionRecipeModel{},
				&gormRepo.CommentModel{},
				&gormRepo.ActivityModel{},
				&gormRepo.RecipeViewModel{},
			); err != nil {
				log.Warn("Failed to auto-migrate database", zap.Error(err))
			}
		}

		log.Info("Connected to PostgreSQL database with performance optimization",
			zap.String("host", cfg.Database.Host),
			zap.Int("port", cfg.Database.Port),
			zap.String("database", cfg.Database.Database),
			zap.Int("max_open_conns", cfg.Database.MaxOpenConns),
			zap.Int("max_idle_conns", cfg.Database.MaxIdleConns),
		)

		return db, nil
	},
	
	// PostgreSQL Connection Manager
	func(cfg *config.Config, log *zap.Logger) (*postgres.ConnectionManager, error) {
		return postgres.NewConnectionManager(cfg, log)
	},
	
	// Query Cache with Redis integration
	func(cfg *config.Config, log *zap.Logger) (*postgres.QueryCache, error) {
		// Create Redis client for cache
		redisClient := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.Database,
		})
		
		// Test Redis connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Warn("Redis connection failed, query cache disabled", zap.Error(err))
			return nil, err
		}
		
		cacheConfig := postgres.CacheConfig{
			Enabled:    true,
			DefaultTTL: 5 * time.Minute,
			KeyPrefix:  "alchemorsel:query",
		}
		
		return postgres.NewQueryCache(redisClient, log, cacheConfig), nil
	},
	
	// Performance Dashboard
	func(
		cm *postgres.ConnectionManager, 
		log *zap.Logger,
		db *gorm.DB,
		qc *postgres.QueryCache,
	) *postgres.PerformanceDashboard {
		qm := cm.GetQueryMonitor()
		io := cm.GetIndexOptimizer()
		io.SetDB(db)
		
		return postgres.NewPerformanceDashboard(cm, qm, io, qc, log)
	},
)

// CacheModule provides caching
var CacheModule = fx.Provide(
	func(log *zap.Logger) outbound.CacheRepository {
		log.Info("Using in-memory cache for demo")
		return memory.NewCacheRepository()
	},
	
	// Mock message bus for demo
	func(log *zap.Logger) outbound.MessageBus {
		log.Info("Using mock message bus for demo")
		return &MockMessageBus{}
	},
)

// HealthCheckModule provides health check functionality
var HealthCheckModule = fx.Provide(
	// Health metrics
	func(cfg *config.Config) *healthcheck.HealthMetrics {
		if cfg.Monitoring.HealthCheck.EnableMetrics {
			return healthcheck.NewHealthMetricsWithConfig(healthcheck.MetricsConfig{
				Namespace: cfg.Monitoring.HealthCheck.Metrics.Namespace,
				Subsystem: cfg.Monitoring.HealthCheck.Metrics.Subsystem,
				Enabled:   cfg.Monitoring.HealthCheck.Metrics.Enabled,
			})
		}
		return healthcheck.NewHealthMetrics()
	},
	
	// Enterprise health check
	func(cfg *config.Config, log *zap.Logger, metrics *healthcheck.HealthMetrics) *healthcheck.EnterpriseHealthCheck {
		if cfg.Monitoring.HealthCheck.EnableEnterprise {
			hc := healthcheck.NewEnterpriseHealthCheckWithMetrics(cfg.App.Version, log, metrics)
			// Configure cache TTL
			hc.HealthCheck.SetCacheTTL(cfg.Monitoring.HealthCheck.CacheTTL)
			return hc
		}
		// Always use the metrics version to avoid duplicate registrations
		hc := healthcheck.NewEnterpriseHealthCheckWithMetrics(cfg.App.Version, log, metrics)
		hc.HealthCheck.SetCacheTTL(cfg.Monitoring.HealthCheck.CacheTTL)
		return hc
	},
	
	// System checker (using value group)
	fx.Annotate(
		func(cfg *config.Config) healthcheck.Checker {
			return healthcheck.NewCustomChecker("system", func(ctx context.Context) (healthcheck.Status, string, interface{}) {
				return healthcheck.StatusHealthy, "System operational", map[string]interface{}{
					"service": "alchemorsel-v3",
					"version": cfg.App.Version,
					"environment": cfg.App.Environment,
				}
			})
		},
		fx.ResultTags(`group:"healthcheckers"`),
	),
	
	// Database checker (using value group)
	fx.Annotate(
		func(db *gorm.DB) healthcheck.Checker {
			return healthcheck.NewCustomChecker("database", func(ctx context.Context) (healthcheck.Status, string, interface{}) {
				sqlDB, err := db.DB()
				if err != nil {
					return healthcheck.StatusUnhealthy, err.Error(), nil
				}
				
				if err := sqlDB.PingContext(ctx); err != nil {
					return healthcheck.StatusUnhealthy, err.Error(), nil
				}
				
				stats := sqlDB.Stats()
				return healthcheck.StatusHealthy, "Database operational", map[string]interface{}{
					"open_connections": stats.OpenConnections,
					"in_use": stats.InUse,
					"idle": stats.Idle,
					"max_open_connections": stats.MaxOpenConnections,
				}
			})
		},
		fx.ResultTags(`group:"healthcheckers"`),
	),

	// Health checker group collector
	fx.Annotate(
		func(checkers []healthcheck.Checker) HealthCheckerGroup {
			return HealthCheckerGroup{Checkers: checkers}
		},
		fx.ParamTags(`group:"healthcheckers"`),
	),
)

// RepositoryModule provides repository implementations
var RepositoryModule = fx.Provide(
	// Recipe repository
	fx.Annotate(
		gormRepo.NewRecipeRepository,
		fx.As(new(outbound.RecipeRepository)),
	),
	
	// User repository
	fx.Annotate(
		gormRepo.NewUserRepository,
		fx.As(new(outbound.UserRepository)),
	),
)

// ServiceModule provides application services
var ServiceModule = fx.Provide(
	// AI service
	func(log *zap.Logger) outbound.AIService {
		// Use OpenAI client for real AI functionality
		return openai.NewClient(log)
	},
	
	// User service
	func(
		userRepo outbound.UserRepository,
		cache outbound.CacheRepository,
		cfg *config.Config,
		log *zap.Logger,
	) *user.UserService {
		jwtSecret := cfg.Auth.JWTSecret
		if jwtSecret == "" {
			jwtSecret = "demo-secret-key" // Default for demo
		}
		return user.NewUserService(userRepo, cache, jwtSecret, log)
	},
	
	// Recipe service
	fx.Annotate(
		recipe.NewRecipeService,
		fx.As(new(inbound.RecipeService)),
	),
	
	// Auth service (without Redis for now)
	func(cfg *config.Config, log *zap.Logger) *security.AuthService {
		return security.NewAuthService(cfg, log, nil)
	},
)

// HTTPModule provides HTTP server and handlers
var HTTPModule = fx.Provide(
	server.NewServer,
)

// EventModule provides event handling
var EventModule = fx.Provide(
	NewEventDispatcher,
	NewEventHandlers,
)

// LifecycleModule provides lifecycle hooks
var LifecycleModule = fx.Invoke(
	RegisterLifecycleHooks,
	InitializeHealthChecks,
)

// RegisterLifecycleHooks registers application lifecycle hooks
func RegisterLifecycleHooks(
	lc fx.Lifecycle,
	cfg *config.Config,
	log *zap.Logger,
	db *gorm.DB,
	server *server.Server,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Starting Alchemorsel application",
				zap.String("version", cfg.App.Version),
				zap.String("environment", cfg.App.Environment),
			)
			
			// Start HTTP server
			go func() {
				if err := server.Start(); err != nil {
					log.Fatal("Failed to start HTTP server", zap.Error(err))
				}
			}()
			
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Shutting down Alchemorsel application")
			
			// Shutdown HTTP server
			if err := server.Shutdown(ctx); err != nil {
				log.Error("Failed to shutdown HTTP server", zap.Error(err))
			}
			
			// Close database connections
			sqlDB, err := db.DB()
			if err == nil {
				if err := sqlDB.Close(); err != nil {
					log.Error("Failed to close database connection", zap.Error(err))
				}
			}
			
			// Flush logs
			_ = log.Sync()
			
			return nil
		},
	})
}

// EventDispatcher implementation
type EventDispatcher struct {
	handlers map[string][]outbound.MessageHandler
	log      *zap.Logger
}

// NewEventDispatcher creates a new event dispatcher
func NewEventDispatcher(log *zap.Logger) *EventDispatcher {
	return &EventDispatcher{
		handlers: make(map[string][]outbound.MessageHandler),
		log:      log,
	}
}

// Dispatch dispatches an event to registered handlers
func (d *EventDispatcher) Dispatch(ctx context.Context, event string, payload []byte) error {
	handlers, exists := d.handlers[event]
	if !exists {
		d.log.Debug("No handlers registered for event", zap.String("event", event))
		return nil
	}
	
	for _, handler := range handlers {
		message := outbound.Message{
			Type:    event,
			Payload: payload,
		}
		
		if err := handler(ctx, message); err != nil {
			d.log.Error("Failed to handle event",
				zap.String("event", event),
				zap.Error(err),
			)
			// Continue processing other handlers
		}
	}
	
	return nil
}

// Register registers an event handler
func (d *EventDispatcher) Register(event string, handler outbound.MessageHandler) {
	d.handlers[event] = append(d.handlers[event], handler)
	d.log.Debug("Registered event handler", zap.String("event", event))
}

// EventHandlers holds all event handlers
type EventHandlers struct {
	RecipeCreatedHandler   outbound.MessageHandler
	RecipePublishedHandler outbound.MessageHandler
	UserRegisteredHandler  outbound.MessageHandler
}

// NewEventHandlers creates event handlers
func NewEventHandlers(log *zap.Logger) *EventHandlers {
	return &EventHandlers{
		RecipeCreatedHandler: func(ctx context.Context, msg outbound.Message) error {
			log.Info("Recipe created event received", zap.String("payload", string(msg.Payload)))
			// Implement actual handler logic
			return nil
		},
		RecipePublishedHandler: func(ctx context.Context, msg outbound.Message) error {
			log.Info("Recipe published event received", zap.String("payload", string(msg.Payload)))
			// Implement actual handler logic
			return nil
		},
		UserRegisteredHandler: func(ctx context.Context, msg outbound.Message) error {
			log.Info("User registered event received", zap.String("payload", string(msg.Payload)))
			// Implement actual handler logic
			return nil
		},
	}
}

// MockMessageBus provides a mock implementation for demo purposes
type MockMessageBus struct{}

func (m *MockMessageBus) Publish(ctx context.Context, topic string, message outbound.Message) error {
	return nil // No-op for demo
}

func (m *MockMessageBus) PublishBatch(ctx context.Context, topic string, messages []outbound.Message) error {
	return nil // No-op for demo
}

func (m *MockMessageBus) Subscribe(ctx context.Context, topic string, handler outbound.MessageHandler) error {
	return nil // No-op for demo
}

func (m *MockMessageBus) Unsubscribe(ctx context.Context, topic string) error {
	return nil // No-op for demo
}

// PureAPIModule provides all dependencies for pure JSON API server (no templates/frontend)
var PureAPIModule = fx.Options(
	// Infrastructure modules (same as full app)
	ConfigModule,
	LoggerModule, 
	DatabaseModule,
	CacheModule,
	
	// Health check module
	HealthCheckModule,
	
	// Repository modules
	RepositoryModule,
	
	// Service modules  
	ServiceModule,
	
	// Pure API HTTP module (no templates)
	PureAPIHTTPModule,
	
	// Event modules
	EventModule,
	
	// Lifecycle hooks for API
	PureAPILifecycleModule,
)

// PureAPIHTTPModule provides HTTP server for pure JSON API
var PureAPIHTTPModule = fx.Provide(
	NewPureAPIServer,
)

// PureAPILifecycleModule provides lifecycle hooks for pure API
var PureAPILifecycleModule = fx.Invoke(
	RegisterPureAPILifecycleHooks,
	InitializeHealthChecks,
)

// NewPureAPIServer creates a new pure API server instance (no templates)
func NewPureAPIServer(
	cfg *config.Config,
	log *zap.Logger,
	recipeService inbound.RecipeService,
	userService *user.UserService,
	authService *security.AuthService,
	aiService outbound.AIService,
	healthCheck *healthcheck.EnterpriseHealthCheck,
) *PureAPIServer {
	return &PureAPIServer{
		config:        cfg,
		logger:        log,
		recipeService: recipeService,
		userService:   userService,
		authService:   authService,
		aiService:     aiService,
		healthCheck:   healthCheck,
	}
}

// RegisterPureAPILifecycleHooks registers lifecycle hooks for pure API server
func RegisterPureAPILifecycleHooks(
	lc fx.Lifecycle,
	cfg *config.Config,
	log *zap.Logger,
	db *gorm.DB,
	server *PureAPIServer,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Override port from environment if set
			if port := os.Getenv("PORT"); port != "" {
				cfg.Server.Port = parsePort(port)
			}
			
			log.Info("Starting Pure API server",
				zap.Int("port", cfg.Server.Port),
				zap.String("environment", cfg.App.Environment),
			)
			
			fmt.Printf("🚀 Alchemorsel v3 Pure API starting on http://localhost:%d\n", cfg.Server.Port)
			fmt.Println("🔥 Pure JSON API Backend - No Frontend Templates") 
			fmt.Println("📊 Enterprise Architecture with DI Container")
			fmt.Println("🛡️  Authentication, AI, Recipe Management APIs")
			fmt.Printf("📖 API Documentation: http://localhost:%d/api/v1/docs\n", cfg.Server.Port)
			
			// Start server in background
			go func() {
				if err := server.Start(); err != nil && err != http.ErrServerClosed {
					log.Fatal("Pure API server failed to start", zap.Error(err))
				}
			}()
			
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Shutting down Pure API server...")
			return server.Shutdown(ctx)
		},
	})
}

// parsePort parses a string to int for port, defaults to 3000 if invalid
func parsePort(portStr string) int {
	if portStr == "" {
		return 3000
	}
	port := 3000
	fmt.Sscanf(portStr, "%d", &port)
	return port
}

// PureAPIServer represents a pure JSON API HTTP server (no templates)
type PureAPIServer struct {
	config        *config.Config
	logger        *zap.Logger
	server        *http.Server
	recipeService inbound.RecipeService
	userService   *user.UserService
	authService   *security.AuthService
	aiService     outbound.AIService
	healthCheck   *healthcheck.EnterpriseHealthCheck
}

// Start starts the pure API HTTP server
func (s *PureAPIServer) Start() error {
	// Use the existing API server constructor
	apiServer := apiserver.NewPureAPIServer(
		s.config,
		s.logger,
		s.recipeService,
		s.userService,
		s.authService,
		s.aiService,
		s.healthCheck,
	)
	
	// Store the server instance for shutdown
	s.server = apiServer.Server()
	
	return apiServer.Start()
}

// Shutdown gracefully shuts down the pure API server
func (s *PureAPIServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// HealthCheckerGroup represents the collected health checkers from the value group
type HealthCheckerGroup struct {
	Checkers []healthcheck.Checker `group:"healthcheckers"`
}

// InitializeHealthChecks registers all health checks with the enterprise health check instance
func InitializeHealthChecks(
	cfg *config.Config,
	log *zap.Logger,
	hc *healthcheck.EnterpriseHealthCheck,
	group HealthCheckerGroup,
) {
	log.Info("Initializing enterprise health checks")
	
	// Create a map to store checkers by name for dependency registration
	checkerMap := make(map[string]healthcheck.Checker)
	
	// Register all checkers from the value group
	for _, checker := range group.Checkers {
		// Get the checker name by performing a test check to extract the name
		testCtx := context.Background()
		testCheck := checker.Check(testCtx)
		checkerName := testCheck.Name
		
		// Store in map for later dependency registration
		checkerMap[checkerName] = checker
		
		// Register with or without circuit breaker
		if cfg.Monitoring.HealthCheck.EnableCircuitBreaker {
			circuitConfig := healthcheck.CircuitBreakerConfig{
				FailureThreshold: cfg.Monitoring.HealthCheck.CircuitBreaker.FailureThreshold,
				SuccessThreshold: cfg.Monitoring.HealthCheck.CircuitBreaker.SuccessThreshold,
				Timeout:         cfg.Monitoring.HealthCheck.CircuitBreaker.Timeout,
				MaxRequests:     cfg.Monitoring.HealthCheck.CircuitBreaker.MaxRequests,
			}
			hc.RegisterWithCircuitBreaker(checkerName, checker, circuitConfig)
		} else {
			hc.Register(checkerName, checker)
		}
		
		log.Info("Registered health checker", zap.String("name", checkerName))
	}
	
	// Register dependencies if enabled
	if cfg.Monitoring.HealthCheck.EnableDependencies {
		// Register database dependency if database checker exists
		if dbChecker, exists := checkerMap["database"]; exists {
			dbDep := healthcheck.DatabaseDependency("database", true, dbChecker)
			hc.RegisterDependency(dbDep)
		}
		
		log.Info("Registered health check dependencies")
	}
	
	log.Info("Enterprise health checks initialized successfully",
		zap.Int("checkers_count", len(group.Checkers)),
		zap.Bool("circuit_breaker", cfg.Monitoring.HealthCheck.EnableCircuitBreaker),
		zap.Bool("dependencies", cfg.Monitoring.HealthCheck.EnableDependencies),
		zap.Bool("metrics", cfg.Monitoring.HealthCheck.EnableMetrics),
	)
}