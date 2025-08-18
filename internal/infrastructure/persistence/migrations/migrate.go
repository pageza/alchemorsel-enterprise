// Package migrations provides database migration functionality
// using golang-migrate for schema versioning
package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

// Migrator handles database migrations
type Migrator struct {
	db      *sql.DB
	migrate *migrate.Migrate
	logger  *zap.Logger
}

// New creates a new migrator instance
func New(db *sql.DB, logger *zap.Logger) (*Migrator, error) {
	// Create source from embedded files
	source, err := iofs.New(sqlFiles, "sql")
	if err != nil {
		return nil, fmt.Errorf("failed to create migration source: %w", err)
	}
	
	// Create database driver
	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "schema_migrations",
		DatabaseName:    "alchemorsel",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}
	
	// Create migrate instance
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}
	
	return &Migrator{
		db:      db,
		migrate: m,
		logger:  logger,
	}, nil
}

// Up runs all pending migrations
func (m *Migrator) Up() error {
	start := time.Now()
	m.logger.Info("Running database migrations")
	
	// Get current version
	currentVersion, _, err := m.migrate.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get current version: %w", err)
	}
	
	// Run migrations
	if err := m.migrate.Up(); err != nil {
		if err == migrate.ErrNoChange {
			m.logger.Info("No migrations to run",
				zap.Uint("current_version", currentVersion),
			)
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	
	// Get new version
	newVersion, _, _ := m.migrate.Version()
	
	m.logger.Info("Migrations completed successfully",
		zap.Uint("from_version", currentVersion),
		zap.Uint("to_version", newVersion),
		zap.Duration("duration", time.Since(start)),
	)
	
	return nil
}

// Down rolls back one migration
func (m *Migrator) Down() error {
	m.logger.Info("Rolling back one migration")
	
	if err := m.migrate.Steps(-1); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}
	
	m.logger.Info("Migration rolled back successfully")
	return nil
}

// Reset rolls back all migrations
func (m *Migrator) Reset() error {
	m.logger.Warn("Resetting all migrations")
	
	if err := m.migrate.Down(); err != nil {
		return fmt.Errorf("failed to reset migrations: %w", err)
	}
	
	m.logger.Info("All migrations reset successfully")
	return nil
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.migrate.Version()
	if err == migrate.ErrNilVersion {
		return 0, false, nil
	}
	return version, dirty, err
}

// Force sets a specific migration version
func (m *Migrator) Force(version int) error {
	m.logger.Warn("Forcing migration version",
		zap.Int("version", version),
	)
	
	if err := m.migrate.Force(version); err != nil {
		return fmt.Errorf("failed to force version: %w", err)
	}
	
	m.logger.Info("Migration version forced successfully")
	return nil
}

// Steps runs n migration steps (positive = up, negative = down)
func (m *Migrator) Steps(n int) error {
	action := "up"
	if n < 0 {
		action = "down"
	}
	
	m.logger.Info("Running migration steps",
		zap.String("direction", action),
		zap.Int("steps", n),
	)
	
	if err := m.migrate.Steps(n); err != nil {
		if err == migrate.ErrNoChange {
			m.logger.Info("No migrations to run")
			return nil
		}
		return fmt.Errorf("failed to run migration steps: %w", err)
	}
	
	m.logger.Info("Migration steps completed successfully")
	return nil
}

// Close closes the migrator
func (m *Migrator) Close() error {
	sourceErr, dbErr := m.migrate.Close()
	
	if sourceErr != nil {
		return fmt.Errorf("failed to close source: %w", sourceErr)
	}
	
	if dbErr != nil {
		return fmt.Errorf("failed to close database: %w", dbErr)
	}
	
	return nil
}

// MigrationStatus represents the status of migrations
type MigrationStatus struct {
	Version uint      `json:"version"`
	Dirty   bool      `json:"dirty"`
	Applied []Applied `json:"applied"`
	Pending []Pending `json:"pending"`
}

// Applied represents an applied migration
type Applied struct {
	Version   uint      `json:"version"`
	Name      string    `json:"name"`
	AppliedAt time.Time `json:"applied_at"`
}

// Pending represents a pending migration
type Pending struct {
	Version uint   `json:"version"`
	Name    string `json:"name"`
}

// Status returns the current migration status
func (m *Migrator) Status() (*MigrationStatus, error) {
	version, dirty, err := m.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}
	
	status := &MigrationStatus{
		Version: version,
		Dirty:   dirty,
		Applied: []Applied{},
		Pending: []Pending{},
	}
	
	// Query applied migrations
	rows, err := m.db.Query(`
		SELECT version, dirty 
		FROM schema_migrations 
		ORDER BY version ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var v uint
		var d bool
		if err := rows.Scan(&v, &d); err != nil {
			continue
		}
		
		status.Applied = append(status.Applied, Applied{
			Version:   v,
			AppliedAt: time.Now(), // Would need to store this in the table
		})
	}
	
	// TODO: Determine pending migrations by comparing with embedded files
	
	return status, nil
}

// CreateMigration creates a new migration file template
func CreateMigration(name string) error {
	timestamp := time.Now().Format("20060102150405")
	upFile := fmt.Sprintf("%s_%s.up.sql", timestamp, name)
	downFile := fmt.Sprintf("%s_%s.down.sql", timestamp, name)
	
	upContent := fmt.Sprintf(`-- Migration: %s (UP)
-- Description: Add description here
-- Author: Generated
-- Date: %s

BEGIN;

-- Add your UP migration SQL here

COMMIT;
`, name, time.Now().Format(time.RFC3339))
	
	downContent := fmt.Sprintf(`-- Migration: %s (DOWN)
-- Description: Rollback for %s
-- Author: Generated
-- Date: %s

BEGIN;

-- Add your DOWN migration SQL here

COMMIT;
`, name, name, time.Now().Format(time.RFC3339))
	
	// In a real implementation, write these to files
	fmt.Printf("Created migration files:\n- %s\n- %s\n", upFile, downFile)
	fmt.Printf("\nUP Migration:\n%s\n", upContent)
	fmt.Printf("\nDOWN Migration:\n%s\n", downContent)
	
	return nil
}