// Package postgres provides PostgreSQL database connection and management
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

// ConnectionManager manages PostgreSQL database connections with optimized pooling
type ConnectionManager struct {
	config         *config.Config
	logger         *zap.Logger
	db             *gorm.DB
	writeDB        *sql.DB
	readDBs        []*sql.DB
	metrics        *ConnectionMetrics
	queryMonitor   *QueryMonitor
	indexOptimizer *IndexOptimizer
}

// ConnectionConfig holds advanced connection configuration
type ConnectionConfig struct {
	// Connection Pool Settings
	MaxOpenConns        int           `json:"max_open_conns"`
	MaxIdleConns        int           `json:"max_idle_conns"`
	ConnMaxLifetime     time.Duration `json:"conn_max_lifetime"`
	ConnMaxIdleTime     time.Duration `json:"conn_max_idle_time"`
	
	// Performance Settings
	SlowQueryThreshold  time.Duration `json:"slow_query_threshold"`
	QueryTimeout        time.Duration `json:"query_timeout"`
	LogLevel            string        `json:"log_level"`
	
	// Read Replica Settings
	ReadReplicas        []string      `json:"read_replicas"`
	ReadWritePolicy     string        `json:"read_write_policy"`
	LoadBalancePolicy   string        `json:"load_balance_policy"`
	
	// Cache Settings
	EnableQueryCache    bool          `json:"enable_query_cache"`
	CacheTTL           time.Duration `json:"cache_ttl"`
	
	// Monitoring Settings
	EnableMetrics       bool          `json:"enable_metrics"`
	MetricsInterval     time.Duration `json:"metrics_interval"`
}

// DefaultConnectionConfig returns optimized default configuration
func DefaultConnectionConfig() *ConnectionConfig {
	return &ConnectionConfig{
		// Optimized for 1000+ concurrent users
		MaxOpenConns:        100,  // Increased from 25
		MaxIdleConns:        25,   // Increased from 5
		ConnMaxLifetime:     30 * time.Minute, // Reduced from 1h
		ConnMaxIdleTime:     5 * time.Minute,  // Reduced from 10m
		
		// Performance tuning
		SlowQueryThreshold:  50 * time.Millisecond, // Aggressive threshold
		QueryTimeout:        30 * time.Second,
		LogLevel:           "warn",
		
		// Read replica configuration
		ReadWritePolicy:    "auto",
		LoadBalancePolicy:  "round_robin",
		
		// Cache configuration
		EnableQueryCache:   true,
		CacheTTL:          5 * time.Minute,
		
		// Monitoring
		EnableMetrics:      true,
		MetricsInterval:    10 * time.Second,
	}
}

// NewConnectionManager creates a new connection manager with optimized settings
func NewConnectionManager(cfg *config.Config, log *zap.Logger) (*ConnectionManager, error) {
	connConfig := DefaultConnectionConfig()
	
	// Override defaults with config values
	if cfg.Database.MaxOpenConns > 0 {
		connConfig.MaxOpenConns = cfg.Database.MaxOpenConns
	}
	if cfg.Database.MaxIdleConns > 0 {
		connConfig.MaxIdleConns = cfg.Database.MaxIdleConns
	}
	if cfg.Database.ConnMaxLifetime > 0 {
		connConfig.ConnMaxLifetime = cfg.Database.ConnMaxLifetime
	}
	if cfg.Database.ConnMaxIdleTime > 0 {
		connConfig.ConnMaxIdleTime = cfg.Database.ConnMaxIdleTime
	}
	if cfg.Database.SlowQueryThreshold > 0 {
		connConfig.SlowQueryThreshold = cfg.Database.SlowQueryThreshold
	}

	cm := &ConnectionManager{
		config:         cfg,
		logger:         log,
		metrics:        NewConnectionMetrics(),
		queryMonitor:   NewQueryMonitor(log),
		indexOptimizer: NewIndexOptimizer(log),
	}

	// Initialize primary database connection
	if err := cm.initializePrimaryConnection(connConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize primary connection: %w", err)
	}

	// Initialize read replicas if configured
	if err := cm.initializeReadReplicas(connConfig); err != nil {
		log.Warn("Failed to initialize read replicas", zap.Error(err))
	}

	// Start monitoring
	if connConfig.EnableMetrics {
		go cm.startMetricsCollection(connConfig.MetricsInterval)
	}

	log.Info("Database connection manager initialized",
		zap.Int("max_open_conns", connConfig.MaxOpenConns),
		zap.Int("max_idle_conns", connConfig.MaxIdleConns),
		zap.Duration("conn_max_lifetime", connConfig.ConnMaxLifetime),
		zap.Duration("slow_query_threshold", connConfig.SlowQueryThreshold),
	)

	return cm, nil
}

// initializePrimaryConnection sets up the primary database connection
func (cm *ConnectionManager) initializePrimaryConnection(config *ConnectionConfig) error {
	dsn := cm.config.GetDSN()
	
	// Create GORM logger with performance optimization
	gormLogger := cm.createGORMLogger(config)
	
	// Open database connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true, // Improve performance
		PrepareStmt:           true,  // Enable prepared statements
		DisableForeignKeyConstraintWhenMigrating: false,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Configure connection pool for high performance
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	cm.db = db
	cm.writeDB = sqlDB

	// Install query monitoring plugin
	if err := cm.installQueryMonitoring(); err != nil {
		cm.logger.Warn("Failed to install query monitoring", zap.Error(err))
	}

	return nil
}

// initializeReadReplicas sets up read replica connections
func (cm *ConnectionManager) initializeReadReplicas(config *ConnectionConfig) error {
	if len(config.ReadReplicas) == 0 {
		return nil
	}

	// Configure read replicas using GORM DB Resolver
	replicas := make([]gorm.Dialector, len(config.ReadReplicas))
	for i, replica := range config.ReadReplicas {
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			replica,
			cm.config.Database.Port,
			cm.config.Database.Username,
			cm.config.Database.Password,
			cm.config.Database.Database,
			cm.config.Database.SSLMode,
		)
		replicas[i] = postgres.Open(dsn)
	}

	// Register read replicas
	err := cm.db.Use(dbresolver.Register(dbresolver.Config{
		Replicas: replicas,
		Policy:   getLoadBalancePolicy(config.LoadBalancePolicy),
	}))
	if err != nil {
		return fmt.Errorf("failed to register read replicas: %w", err)
	}

	cm.logger.Info("Read replicas configured",
		zap.Int("replica_count", len(config.ReadReplicas)),
		zap.String("load_balance_policy", config.LoadBalancePolicy),
	)

	return nil
}

// createGORMLogger creates an optimized GORM logger
func (cm *ConnectionManager) createGORMLogger(config *ConnectionConfig) logger.Interface {
	logLevel := logger.Silent
	switch config.LogLevel {
	case "debug":
		logLevel = logger.Info
	case "info":
		logLevel = logger.Warn
	case "warn":
		logLevel = logger.Error
	case "error":
		logLevel = logger.Error
	}

	return logger.New(
		&GORMLogWriter{logger: cm.logger, queryMonitor: cm.queryMonitor},
		logger.Config{
			SlowThreshold:             config.SlowQueryThreshold,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}

// installQueryMonitoring installs query monitoring callbacks
func (cm *ConnectionManager) installQueryMonitoring() error {
	// Install before/after callbacks for query monitoring
	err := cm.db.Callback().Query().Before("gorm:query").Register("monitor:before", cm.queryMonitor.BeforeQuery)
	if err != nil {
		return err
	}

	err = cm.db.Callback().Query().After("gorm:query").Register("monitor:after", cm.queryMonitor.AfterQuery)
	if err != nil {
		return err
	}

	return nil
}

// GetDB returns the main database connection
func (cm *ConnectionManager) GetDB() *gorm.DB {
	return cm.db
}

// GetMetrics returns connection metrics
func (cm *ConnectionManager) GetMetrics() *ConnectionMetrics {
	return cm.metrics
}

// GetQueryMonitor returns the query monitor
func (cm *ConnectionManager) GetQueryMonitor() *QueryMonitor {
	return cm.queryMonitor
}

// GetIndexOptimizer returns the index optimizer
func (cm *ConnectionManager) GetIndexOptimizer() *IndexOptimizer {
	return cm.indexOptimizer
}

// HealthCheck performs a health check on the database connection
func (cm *ConnectionManager) HealthCheck(ctx context.Context) error {
	if err := cm.writeDB.PingContext(ctx); err != nil {
		return fmt.Errorf("primary database ping failed: %w", err)
	}

	// Check read replicas if available
	for i, readDB := range cm.readDBs {
		if err := readDB.PingContext(ctx); err != nil {
			cm.logger.Warn("Read replica ping failed",
				zap.Int("replica_index", i),
				zap.Error(err),
			)
		}
	}

	return nil
}

// Close closes all database connections
func (cm *ConnectionManager) Close() error {
	if cm.writeDB != nil {
		if err := cm.writeDB.Close(); err != nil {
			cm.logger.Error("Failed to close primary database", zap.Error(err))
		}
	}

	for i, readDB := range cm.readDBs {
		if err := readDB.Close(); err != nil {
			cm.logger.Error("Failed to close read replica",
				zap.Int("replica_index", i),
				zap.Error(err),
			)
		}
	}

	return nil
}

// startMetricsCollection starts periodic metrics collection
func (cm *ConnectionManager) startMetricsCollection(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		cm.collectMetrics()
	}
}

// collectMetrics collects current database metrics
func (cm *ConnectionManager) collectMetrics() {
	if cm.writeDB == nil {
		return
	}

	stats := cm.writeDB.Stats()
	cm.metrics.UpdateConnectionStats(stats)
	
	// Collect query metrics
	queryStats := cm.queryMonitor.GetStats()
	cm.metrics.UpdateQueryStats(queryStats)
}

// getLoadBalancePolicy converts string to dbresolver policy
func getLoadBalancePolicy(policy string) dbresolver.Policy {
	switch policy {
	case "random":
		return dbresolver.RandomPolicy{}
	case "round_robin":
		return dbresolver.RoundRobinPolicy{}
	default:
		return dbresolver.RandomPolicy{}
	}
}