// Package container provides dependency injection using Uber FX
// This implements the Dependency Inversion Principle from SOLID
package container

import (
	"context"
	"fmt"

	"github.com/alchemorsel/v3/internal/application/ai"
	"github.com/alchemorsel/v3/internal/application/recipe"
	"github.com/alchemorsel/v3/internal/application/user"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/http/server"
	gormRepo "github.com/alchemorsel/v3/internal/infrastructure/persistence/gorm"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/memory"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/sqlite"
	"github.com/alchemorsel/v3/internal/ports/inbound"
	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/alchemorsel/v3/pkg/logger"
	
	"go.uber.org/fx"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// Module provides all dependency injection modules
var Module = fx.Options(
	// Infrastructure modules
	ConfigModule,
	LoggerModule,
	DatabaseModule,
	CacheModule,
	
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

// DatabaseModule provides database connections
var DatabaseModule = fx.Provide(
	// SQLite database with GORM
	func(cfg *config.Config, log *zap.Logger) (*gorm.DB, error) {
		// Use in-memory SQLite for demo
		dbPath := ":memory:"
		if cfg.Database.Database != "" {
			dbPath = cfg.Database.Database + ".db"
		}

		// Set log level based on config
		logLevel := gormLogger.Silent
		if cfg.App.Debug {
			logLevel = gormLogger.Info
		}

		db, err := sqlite.SetupDatabase(dbPath, logLevel)
		if err != nil {
			return nil, fmt.Errorf("failed to setup SQLite database: %w", err)
		}

		// Seed database with demo data
		if err := sqlite.SeedDatabase(db); err != nil {
			log.Warn("Failed to seed database", zap.Error(err))
		}

		log.Info("Connected to SQLite database",
			zap.String("path", dbPath),
			zap.Bool("in_memory", dbPath == ":memory:"),
		)

		return db, nil
	},
)

// CacheModule provides caching
var CacheModule = fx.Provide(
	func(log *zap.Logger) outbound.CacheRepository {
		log.Info("Using in-memory cache for demo")
		return memory.NewCacheRepository()
	},
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
	func(cfg *config.Config, log *zap.Logger) outbound.AIService {
		provider := cfg.AI.Provider
		if provider == "" {
			provider = "mock" // Default to mock for demo
		}
		return ai.NewAIService(provider, log)
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