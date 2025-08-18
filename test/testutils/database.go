// Package testutils provides common testing utilities and infrastructure setup
package testutils

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/internal/infrastructure/persistence/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestDatabase provides a test database instance with cleanup
type TestDatabase struct {
	Container testcontainers.Container
	DB        *sql.DB
	GormDB    *gorm.DB
	PgxPool   *pgxpool.Pool
	DSN       string
	t         *testing.T
}

// DatabaseConfig holds test database configuration
type DatabaseConfig struct {
	Image    string
	Database string
	Username string
	Password string
	Port     string
}

// DefaultDatabaseConfig returns the default test database configuration
func DefaultDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Image:    "postgres:15-alpine",
		Database: "alchemorsel_test",
		Username: "test_user",
		Password: "test_password",
		Port:     "5432",
	}
}

// SetupTestDatabase creates a new test database using testcontainers
func SetupTestDatabase(t *testing.T) *TestDatabase {
	return SetupTestDatabaseWithConfig(t, DefaultDatabaseConfig())
}

// SetupTestDatabaseWithConfig creates a test database with custom configuration
func SetupTestDatabaseWithConfig(t *testing.T, cfg DatabaseConfig) *TestDatabase {
	ctx := context.Background()

	// Create postgres container
	postgres, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Image:        cfg.Image,
				ExposedPorts: []string{cfg.Port + "/tcp"},
				Env: map[string]string{
					"POSTGRES_DB":       cfg.Database,
					"POSTGRES_USER":     cfg.Username,
					"POSTGRES_PASSWORD": cfg.Password,
					"POSTGRES_HOST_AUTH_METHOD": "trust",
				},
				WaitingFor: wait.ForAll(
					wait.ForLog("database system is ready to accept connections").
						WithOccurrence(2).
						WithStartupTimeout(60*time.Second),
					wait.ForSQL(cfg.Port+"/tcp", "postgres", func(host string, port nat.Port) string {
						return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
							cfg.Username, cfg.Password, host, port.Port(), cfg.Database)
					}),
				),
				Tmpfs: map[string]string{
					"/var/lib/postgresql/data": "rw,noexec,nosuid,size=1024m",
				},
			},
			Started: true,
		})
	require.NoError(t, err, "Failed to start postgres container")

	// Get connection details
	host, err := postgres.Host(ctx)
	require.NoError(t, err)

	port, err := postgres.MappedPort(ctx, cfg.Port)
	require.NoError(t, err)

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.Username, cfg.Password, host, port.Port(), cfg.Database)

	// Create standard database connection
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err, "Failed to connect to test database")

	// Verify connection
	err = db.Ping()
	require.NoError(t, err, "Failed to ping test database")

	// Create GORM connection
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Suppress logs in tests
	})
	require.NoError(t, err, "Failed to create GORM connection")

	// Create pgx connection pool
	pgxConfig, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err, "Failed to parse pgx config")

	pgxConfig.MaxConns = 10 // Limit connections for tests
	pgxConfig.MinConns = 1
	pgxConfig.MaxConnLifetime = time.Hour
	pgxConfig.MaxConnIdleTime = time.Minute * 30

	pgxPool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	require.NoError(t, err, "Failed to create pgx pool")

	testDB := &TestDatabase{
		Container: postgres,
		DB:        db,
		GormDB:    gormDB,
		PgxPool:   pgxPool,
		DSN:       dsn,
		t:         t,
	}

	// Setup cleanup
	t.Cleanup(func() {
		testDB.Cleanup()
	})

	return testDB
}

// RunMigrations runs database migrations on the test database
func (td *TestDatabase) RunMigrations() error {
	driver, err := postgres.WithInstance(td.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	// Get migration path
	migrationPath := "file://" + filepath.Join("../../internal/infrastructure/persistence/migrations/sql")
	
	m, err := migrate.NewWithDatabaseInstance(migrationPath, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// SeedTestData inserts test data into the database
func (td *TestDatabase) SeedTestData() error {
	// Insert test users
	_, err := td.DB.Exec(`
		INSERT INTO users (id, email, password_hash, username, created_at, updated_at)
		VALUES 
			('550e8400-e29b-41d4-a716-446655440001', 'test@example.com', '$2a$10$hash', 'testuser', NOW(), NOW()),
			('550e8400-e29b-41d4-a716-446655440002', 'chef@example.com', '$2a$10$hash', 'chefuser', NOW(), NOW())
	`)
	if err != nil {
		return fmt.Errorf("failed to seed users: %w", err)
	}

	// Insert test recipes
	_, err = td.DB.Exec(`
		INSERT INTO recipes (id, title, description, author_id, status, created_at, updated_at)
		VALUES 
			('550e8400-e29b-41d4-a716-446655440003', 'Test Recipe', 'A test recipe', '550e8400-e29b-41d4-a716-446655440001', 'published', NOW(), NOW()),
			('550e8400-e29b-41d4-a716-446655440004', 'Draft Recipe', 'A draft recipe', '550e8400-e29b-41d4-a716-446655440002', 'draft', NOW(), NOW())
	`)
	if err != nil {
		return fmt.Errorf("failed to seed recipes: %w", err)
	}

	return nil
}

// TruncateAllTables removes all data from tables while preserving structure
func (td *TestDatabase) TruncateAllTables() error {
	tables := []string{
		"recipe_ratings",
		"recipe_ingredients", 
		"recipe_instructions",
		"recipes",
		"user_sessions",
		"users",
	}

	for _, table := range tables {
		_, err := td.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			return fmt.Errorf("failed to truncate table %s: %w", table, err)
		}
	}

	return nil
}

// BeginTransaction starts a new transaction for test isolation
func (td *TestDatabase) BeginTransaction() (*sql.Tx, error) {
	return td.DB.Begin()
}

// Cleanup closes all connections and stops the container
func (td *TestDatabase) Cleanup() {
	ctx := context.Background()

	if td.PgxPool != nil {
		td.PgxPool.Close()
	}

	if td.DB != nil {
		td.DB.Close()
	}

	if td.Container != nil {
		if err := td.Container.Terminate(ctx); err != nil {
			td.t.Logf("Failed to terminate postgres container: %v", err)
		}
	}
}

// TestDBSuite provides a test suite with database setup
type TestDBSuite struct {
	DB     *TestDatabase
	Logger *zap.Logger
	Config *config.Config
}

// SetupSuite initializes the test suite
func (suite *TestDBSuite) SetupSuite(t *testing.T) {
	// Setup logger
	suite.Logger = zap.NewNop() // Silent logger for tests

	// Setup test config
	suite.Config = &config.Config{
		Database: config.DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			Name:            "alchemorsel_test",
			User:            "test_user",
			Password:        "test_password",
			SSLMode:         "disable",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
		},
		Auth: config.AuthConfig{
			JWTSecret:         "test-secret-key-for-testing-only",
			JWTExpiration:     time.Hour,
			RefreshExpiration: 24 * time.Hour,
			BCryptCost:        4, // Lower cost for faster tests
		},
	}

	// Setup database
	suite.DB = SetupTestDatabase(t)
	err := suite.DB.RunMigrations()
	require.NoError(t, err, "Failed to run migrations")
}

// TearDownTest cleans up after each test
func (suite *TestDBSuite) TearDownTest() {
	if suite.DB != nil {
		suite.DB.TruncateAllTables()
	}
}

// DatabaseHelper provides helper methods for database testing
type DatabaseHelper struct {
	db *TestDatabase
}

// NewDatabaseHelper creates a new database helper
func NewDatabaseHelper(db *TestDatabase) *DatabaseHelper {
	return &DatabaseHelper{db: db}
}

// CreateTestUser creates a test user and returns the ID
func (h *DatabaseHelper) CreateTestUser(email, username string) (string, error) {
	userID := "550e8400-e29b-41d4-a716-" + fmt.Sprintf("%012d", time.Now().UnixNano()%1000000000000)
	
	_, err := h.db.DB.Exec(`
		INSERT INTO users (id, email, password_hash, username, created_at, updated_at)
		VALUES ($1, $2, '$2a$04$hash', $3, NOW(), NOW())
	`, userID, email, username)
	
	return userID, err
}

// CreateTestRecipe creates a test recipe and returns the ID
func (h *DatabaseHelper) CreateTestRecipe(title, description, authorID string) (string, error) {
	recipeID := "550e8400-e29b-41d4-a716-" + fmt.Sprintf("%012d", time.Now().UnixNano()%1000000000000)
	
	_, err := h.db.DB.Exec(`
		INSERT INTO recipes (id, title, description, author_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'draft', NOW(), NOW())
	`, recipeID, title, description, authorID)
	
	return recipeID, err
}

// CountRecords counts records in a table
func (h *DatabaseHelper) CountRecords(table string) (int, error) {
	var count int
	err := h.db.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	return count, err
}

// RecordExists checks if a record exists with given conditions
func (h *DatabaseHelper) RecordExists(table, whereClause string, args ...interface{}) (bool, error) {
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s)", table, whereClause)
	err := h.db.DB.QueryRow(query, args...).Scan(&exists)
	return exists, err
}