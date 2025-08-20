// Package postgres provides database migration optimization
package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MigrationOptimizer optimizes database migrations for performance
type MigrationOptimizer struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewMigrationOptimizer creates a new migration optimizer
func NewMigrationOptimizer(db *gorm.DB, logger *zap.Logger) *MigrationOptimizer {
	return &MigrationOptimizer{
		db:     db,
		logger: logger,
	}
}

// OptimizedMigration represents an optimized migration with performance considerations
type OptimizedMigration struct {
	Name        string              `json:"name"`
	SQL         string              `json:"sql"`
	Performance PerformanceImpact   `json:"performance"`
	Safety      SafetyAssessment    `json:"safety"`
	Timing      ExecutionTiming     `json:"timing"`
	Rollback    string              `json:"rollback"`
}

// PerformanceImpact assesses the performance impact of a migration
type PerformanceImpact struct {
	Level          string        `json:"level"`          // low, medium, high, critical
	LockType       string        `json:"lock_type"`      // none, shared, exclusive
	EstimatedTime  time.Duration `json:"estimated_time"`
	TableSize      int64         `json:"table_size"`
	ConcurrentSafe bool          `json:"concurrent_safe"`
	Recommendations []string     `json:"recommendations"`
}

// SafetyAssessment evaluates migration safety
type SafetyAssessment struct {
	RiskLevel       string   `json:"risk_level"`       // low, medium, high
	BreakingChanges bool     `json:"breaking_changes"`
	DataLoss        bool     `json:"data_loss"`
	Reversible      bool     `json:"reversible"`
	Preconditions   []string `json:"preconditions"`
	Warnings        []string `json:"warnings"`
}

// ExecutionTiming provides timing recommendations
type ExecutionTiming struct {
	MaintenanceWindow bool          `json:"maintenance_window"`
	EstimatedDuration time.Duration `json:"estimated_duration"`
	BestTime          string        `json:"best_time"`
	Phased           bool          `json:"phased"`
	Phases           []MigrationPhase `json:"phases,omitempty"`
}

// MigrationPhase represents a phase in a multi-phase migration
type MigrationPhase struct {
	Name        string        `json:"name"`
	SQL         string        `json:"sql"`
	Duration    time.Duration `json:"duration"`
	LockLevel   string        `json:"lock_level"`
	Description string        `json:"description"`
}

// OptimizeAddColumn optimizes ADD COLUMN migrations
func (mo *MigrationOptimizer) OptimizeAddColumn(tableName, columnName, columnType string, nullable bool) *OptimizedMigration {
	var sql strings.Builder
	var rollback strings.Builder
	
	// Build optimized ADD COLUMN statement
	sql.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnType))
	if nullable {
		sql.WriteString(" DEFAULT NULL")
	}
	sql.WriteString(";")
	
	// Build rollback
	rollback.WriteString(fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;", tableName, columnName))
	
	// Assess performance impact
	tableSize := mo.getTableSize(tableName)
	impact := mo.assessAddColumnImpact(tableSize, nullable)
	
	// Assess safety
	safety := SafetyAssessment{
		RiskLevel:       "low",
		BreakingChanges: false,
		DataLoss:        false,
		Reversible:      true,
		Preconditions:   []string{"Verify column name doesn't conflict"},
		Warnings:        []string{},
	}
	
	if !nullable {
		safety.RiskLevel = "medium"
		safety.Warnings = append(safety.Warnings, "Non-nullable column addition may require default value")
	}
	
	// Timing recommendations
	timing := ExecutionTiming{
		MaintenanceWindow: tableSize > 1000000, // Large tables need maintenance window
		EstimatedDuration: impact.EstimatedTime,
		BestTime:          "off-peak hours",
		Phased:           false,
	}
	
	return &OptimizedMigration{
		Name:        fmt.Sprintf("add_column_%s_%s", tableName, columnName),
		SQL:         sql.String(),
		Performance: impact,
		Safety:      safety,
		Timing:      timing,
		Rollback:    rollback.String(),
	}
}

// OptimizeAddIndex optimizes index creation
func (mo *MigrationOptimizer) OptimizeAddIndex(tableName string, columns []string, unique bool) *OptimizedMigration {
	indexName := fmt.Sprintf("idx_%s_%s", tableName, strings.Join(columns, "_"))
	
	var sql strings.Builder
	
	// Use CONCURRENT index creation for better performance
	sql.WriteString("CREATE ")
	if unique {
		sql.WriteString("UNIQUE ")
	}
	sql.WriteString("INDEX CONCURRENTLY ")
	sql.WriteString(indexName)
	sql.WriteString(" ON ")
	sql.WriteString(tableName)
	sql.WriteString(" (")
	sql.WriteString(strings.Join(columns, ", "))
	sql.WriteString(");")
	
	// Rollback
	rollback := fmt.Sprintf("DROP INDEX CONCURRENTLY IF EXISTS %s;", indexName)
	
	// Assess performance impact
	tableSize := mo.getTableSize(tableName)
	impact := mo.assessIndexCreationImpact(tableSize, len(columns))
	
	// Safety assessment
	safety := SafetyAssessment{
		RiskLevel:       "low",
		BreakingChanges: false,
		DataLoss:        false,
		Reversible:      true,
		Preconditions:   []string{"Verify index name doesn't conflict", "Ensure sufficient disk space"},
		Warnings:        []string{},
	}
	
	if unique {
		safety.RiskLevel = "medium"
		safety.Preconditions = append(safety.Preconditions, "Verify data uniqueness constraint")
	}
	
	// Timing
	timing := ExecutionTiming{
		MaintenanceWindow: false, // CONCURRENT doesn't require maintenance window
		EstimatedDuration: impact.EstimatedTime,
		BestTime:          "any time (concurrent safe)",
		Phased:           false,
	}
	
	return &OptimizedMigration{
		Name:        fmt.Sprintf("add_index_%s", indexName),
		SQL:         sql.String(),
		Performance: impact,
		Safety:      safety,
		Timing:      timing,
		Rollback:    rollback,
	}
}

// OptimizeDropColumn optimizes column removal with safety checks
func (mo *MigrationOptimizer) OptimizeDropColumn(tableName, columnName string) *OptimizedMigration {
	// Get column info for rollback
	columnInfo := mo.getColumnInfo(tableName, columnName)
	
	// Phase 1: Make column nullable (if not already)
	// Phase 2: Drop default (if exists)
	// Phase 3: Drop column
	
	phases := []MigrationPhase{
		{
			Name:        "prepare_column_drop",
			SQL:         fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", tableName, columnName),
			Duration:    5 * time.Second,
			LockLevel:   "shared",
			Description: "Remove NOT NULL constraint to prepare for column drop",
		},
		{
			Name:        "drop_column_default",
			SQL:         fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;", tableName, columnName),
			Duration:    2 * time.Second,
			LockLevel:   "shared",
			Description: "Remove default value",
		},
		{
			Name:        "drop_column",
			SQL:         fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", tableName, columnName),
			Duration:    30 * time.Second,
			LockLevel:   "exclusive",
			Description: "Drop the column",
		},
	}
	
	// Build full SQL
	var sql strings.Builder
	for i, phase := range phases {
		sql.WriteString(phase.SQL)
		if i < len(phases)-1 {
			sql.WriteString("\n")
		}
	}
	
	// Build rollback (requires column info)
	rollback := fmt.Sprintf("-- WARNING: Data will be lost\n-- ALTER TABLE %s ADD COLUMN %s %s;", 
		tableName, columnName, columnInfo)
	
	// Performance assessment
	tableSize := mo.getTableSize(tableName)
	impact := PerformanceImpact{
		Level:         "medium",
		LockType:      "exclusive",
		EstimatedTime: 30 * time.Second,
		TableSize:     tableSize,
		ConcurrentSafe: false,
		Recommendations: []string{
			"Execute during maintenance window",
			"Verify no application dependencies on column",
			"Consider soft deletion pattern instead",
		},
	}
	
	// Safety assessment
	safety := SafetyAssessment{
		RiskLevel:       "high",
		BreakingChanges: true,
		DataLoss:        true,
		Reversible:      false,
		Preconditions: []string{
			"Verify no foreign key references",
			"Confirm no application code uses column",
			"Backup data if needed",
		},
		Warnings: []string{
			"Data in column will be permanently lost",
			"Breaking change for applications",
		},
	}
	
	// Timing
	timing := ExecutionTiming{
		MaintenanceWindow: true,
		EstimatedDuration: time.Minute,
		BestTime:          "maintenance window only",
		Phased:           true,
		Phases:           phases,
	}
	
	return &OptimizedMigration{
		Name:        fmt.Sprintf("drop_column_%s_%s", tableName, columnName),
		SQL:         sql.String(),
		Performance: impact,
		Safety:      safety,
		Timing:      timing,
		Rollback:    rollback,
	}
}

// OptimizeChangeColumnType optimizes column type changes
func (mo *MigrationOptimizer) OptimizeChangeColumnType(tableName, columnName, newType string) *OptimizedMigration {
	oldType := mo.getColumnType(tableName, columnName)
	
	// Determine if change is safe/compatible
	compatible := mo.isTypeChangeCompatible(oldType, newType)
	
	var sql string
	var phases []MigrationPhase
	var impact PerformanceImpact
	var safety SafetyAssessment
	
	if compatible {
		// Simple type change
		sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", tableName, columnName, newType)
		
		impact = PerformanceImpact{
			Level:         "low",
			LockType:      "shared",
			EstimatedTime: 10 * time.Second,
			ConcurrentSafe: true,
			Recommendations: []string{"Safe type conversion"},
		}
		
		safety = SafetyAssessment{
			RiskLevel:       "low",
			BreakingChanges: false,
			DataLoss:        false,
			Reversible:      true,
		}
	} else {
		// Complex type change requiring data conversion
		tempColumn := columnName + "_new"
		
		phases = []MigrationPhase{
			{
				Name:        "add_temp_column",
				SQL:         fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", tableName, tempColumn, newType),
				Duration:    5 * time.Second,
				LockLevel:   "shared",
				Description: "Add temporary column with new type",
			},
			{
				Name:        "convert_data",
				SQL:         fmt.Sprintf("UPDATE %s SET %s = %s::%s;", tableName, tempColumn, columnName, newType),
				Duration:    2 * time.Minute,
				LockLevel:   "exclusive",
				Description: "Convert existing data to new type",
			},
			{
				Name:        "drop_old_column",
				SQL:         fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", tableName, columnName),
				Duration:    10 * time.Second,
				LockLevel:   "exclusive",
				Description: "Drop old column",
			},
			{
				Name:        "rename_column",
				SQL:         fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", tableName, tempColumn, columnName),
				Duration:    2 * time.Second,
				LockLevel:   "shared",
				Description: "Rename new column to original name",
			},
		}
		
		// Build full SQL
		var sqlBuilder strings.Builder
		for i, phase := range phases {
			sqlBuilder.WriteString(phase.SQL)
			if i < len(phases)-1 {
				sqlBuilder.WriteString("\n")
			}
		}
		sql = sqlBuilder.String()
		
		impact = PerformanceImpact{
			Level:         "high",
			LockType:      "exclusive",
			EstimatedTime: 3 * time.Minute,
			ConcurrentSafe: false,
			Recommendations: []string{
				"Execute during maintenance window",
				"Test data conversion thoroughly",
				"Consider application compatibility",
			},
		}
		
		safety = SafetyAssessment{
			RiskLevel:       "high",
			BreakingChanges: true,
			DataLoss:        false,
			Reversible:      true,
			Preconditions: []string{
				"Verify data conversion is lossless",
				"Test with sample data",
				"Plan rollback strategy",
			},
			Warnings: []string{
				"Data conversion may fail for invalid values",
				"Application compatibility required",
			},
		}
	}
	
	// Rollback
	rollback := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s;", tableName, columnName, oldType)
	
	// Timing
	timing := ExecutionTiming{
		MaintenanceWindow: !compatible,
		EstimatedDuration: impact.EstimatedTime,
		BestTime:          "maintenance window if complex",
		Phased:           len(phases) > 0,
		Phases:           phases,
	}
	
	return &OptimizedMigration{
		Name:        fmt.Sprintf("change_column_type_%s_%s", tableName, columnName),
		SQL:         sql,
		Performance: impact,
		Safety:      safety,
		Timing:      timing,
		Rollback:    rollback,
	}
}

// assessAddColumnImpact assesses the performance impact of adding a column
func (mo *MigrationOptimizer) assessAddColumnImpact(tableSize int64, nullable bool) PerformanceImpact {
	var level string
	var estimatedTime time.Duration
	var recommendations []string
	
	if tableSize < 100000 {
		level = "low"
		estimatedTime = 5 * time.Second
	} else if tableSize < 1000000 {
		level = "medium"
		estimatedTime = 30 * time.Second
	} else {
		level = "high"
		estimatedTime = 2 * time.Minute
		recommendations = append(recommendations, "Consider maintenance window for large table")
	}
	
	if !nullable {
		level = "medium"
		estimatedTime += 10 * time.Second
		recommendations = append(recommendations, "Non-nullable column requires table scan")
	}
	
	return PerformanceImpact{
		Level:          level,
		LockType:       "shared",
		EstimatedTime:  estimatedTime,
		TableSize:      tableSize,
		ConcurrentSafe: nullable,
		Recommendations: recommendations,
	}
}

// assessIndexCreationImpact assesses index creation performance impact
func (mo *MigrationOptimizer) assessIndexCreationImpact(tableSize int64, columnCount int) PerformanceImpact {
	var level string
	var estimatedTime time.Duration
	
	baseTime := time.Duration(tableSize/10000) * time.Second // Rough estimate
	multiplier := time.Duration(columnCount)
	estimatedTime = baseTime * multiplier
	
	if tableSize < 100000 {
		level = "low"
		estimatedTime = 10 * time.Second
	} else if tableSize < 1000000 {
		level = "medium"
		estimatedTime = 1 * time.Minute
	} else {
		level = "high"
		estimatedTime = 5 * time.Minute
	}
	
	return PerformanceImpact{
		Level:          level,
		LockType:       "none", // CONCURRENT
		EstimatedTime:  estimatedTime,
		TableSize:      tableSize,
		ConcurrentSafe: true,
		Recommendations: []string{
			"Using CONCURRENT creation for minimal locking",
			"Monitor disk space during creation",
		},
	}
}

// getTableSize gets the approximate size of a table
func (mo *MigrationOptimizer) getTableSize(tableName string) int64 {
	var size int64
	query := `
		SELECT COALESCE(
			(SELECT reltuples::bigint FROM pg_class WHERE relname = ?), 
			0
		) as estimated_rows`
	
	err := mo.db.Raw(query, tableName).Scan(&size).Error
	if err != nil {
		mo.logger.Warn("Failed to get table size", zap.String("table", tableName), zap.Error(err))
		return 0
	}
	
	return size
}

// getColumnInfo gets column information for rollback purposes
func (mo *MigrationOptimizer) getColumnInfo(tableName, columnName string) string {
	var dataType string
	query := `
		SELECT data_type 
		FROM information_schema.columns 
		WHERE table_name = ? AND column_name = ?`
	
	err := mo.db.Raw(query, tableName, columnName).Scan(&dataType).Error
	if err != nil {
		mo.logger.Warn("Failed to get column info", 
			zap.String("table", tableName), 
			zap.String("column", columnName), 
			zap.Error(err))
		return "TEXT" // Default fallback
	}
	
	return dataType
}

// getColumnType gets the current type of a column
func (mo *MigrationOptimizer) getColumnType(tableName, columnName string) string {
	return mo.getColumnInfo(tableName, columnName)
}

// isTypeChangeCompatible determines if a type change is compatible/safe
func (mo *MigrationOptimizer) isTypeChangeCompatible(oldType, newType string) bool {
	// Define compatible type changes
	compatibleChanges := map[string][]string{
		"varchar":   {"text", "varchar"},
		"text":      {"varchar"},
		"integer":   {"bigint"},
		"smallint":  {"integer", "bigint"},
		"decimal":   {"numeric"},
		"timestamp": {"timestamptz"},
	}
	
	oldType = strings.ToLower(oldType)
	newType = strings.ToLower(newType)
	
	// Exact match
	if oldType == newType {
		return true
	}
	
	// Check compatible changes
	if compatible, exists := compatibleChanges[oldType]; exists {
		for _, compat := range compatible {
			if strings.Contains(newType, compat) {
				return true
			}
		}
	}
	
	return false
}

// ValidateMigration validates a migration before execution
func (mo *MigrationOptimizer) ValidateMigration(ctx context.Context, migration *OptimizedMigration) error {
	// Check preconditions
	for _, precondition := range migration.Safety.Preconditions {
		mo.logger.Info("Checking precondition", zap.String("condition", precondition))
		// In a real implementation, these would be actual checks
	}
	
	// Validate SQL syntax
	if err := mo.validateSQL(migration.SQL); err != nil {
		return fmt.Errorf("invalid SQL syntax: %w", err)
	}
	
	// Check for breaking changes
	if migration.Safety.BreakingChanges {
		mo.logger.Warn("Migration contains breaking changes", 
			zap.String("migration", migration.Name))
	}
	
	// Check if maintenance window is required
	if migration.Timing.MaintenanceWindow {
		mo.logger.Warn("Migration requires maintenance window",
			zap.String("migration", migration.Name),
			zap.Duration("estimated_duration", migration.Timing.EstimatedDuration))
	}
	
	return nil
}

// validateSQL performs basic SQL validation
func (mo *MigrationOptimizer) validateSQL(sql string) error {
	// Basic validation - check for dangerous patterns
	dangerous := []string{
		"DROP DATABASE",
		"DROP SCHEMA",
		"TRUNCATE",
		"DELETE.*WHERE.*1=1",
		"UPDATE.*WHERE.*1=1",
	}
	
	upperSQL := strings.ToUpper(sql)
	for _, pattern := range dangerous {
		matched, err := regexp.MatchString(pattern, upperSQL)
		if err != nil {
			continue
		}
		if matched {
			return fmt.Errorf("potentially dangerous SQL pattern detected: %s", pattern)
		}
	}
	
	return nil
}

// ExecuteMigration executes an optimized migration
func (mo *MigrationOptimizer) ExecuteMigration(ctx context.Context, migration *OptimizedMigration) error {
	// Validate first
	if err := mo.ValidateMigration(ctx, migration); err != nil {
		return fmt.Errorf("migration validation failed: %w", err)
	}
	
	start := time.Now()
	mo.logger.Info("Executing migration",
		zap.String("name", migration.Name),
		zap.String("performance_level", migration.Performance.Level),
		zap.Bool("phased", migration.Timing.Phased))
	
	if migration.Timing.Phased && len(migration.Timing.Phases) > 0 {
		// Execute phased migration
		for i, phase := range migration.Timing.Phases {
			mo.logger.Info("Executing migration phase",
				zap.Int("phase", i+1),
				zap.String("name", phase.Name),
				zap.String("description", phase.Description))
			
			if err := mo.db.WithContext(ctx).Exec(phase.SQL).Error; err != nil {
				return fmt.Errorf("failed to execute phase %s: %w", phase.Name, err)
			}
		}
	} else {
		// Execute single migration
		if err := mo.db.WithContext(ctx).Exec(migration.SQL).Error; err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}
	
	duration := time.Since(start)
	mo.logger.Info("Migration completed successfully",
		zap.String("name", migration.Name),
		zap.Duration("actual_duration", duration),
		zap.Duration("estimated_duration", migration.Performance.EstimatedTime))
	
	return nil
}