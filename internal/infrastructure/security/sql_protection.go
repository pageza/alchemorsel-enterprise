// Package security provides SQL injection and database security protection
package security

import (
	"context"
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQLSecurityService provides SQL injection protection and database security
type SQLSecurityService struct {
	logger            *zap.Logger
	queryWhitelist    map[string]bool
	dangerousPatterns []*regexp.Regexp
}

// NewSQLSecurityService creates a new SQL security service
func NewSQLSecurityService(logger *zap.Logger) *SQLSecurityService {
	service := &SQLSecurityService{
		logger:         logger,
		queryWhitelist: make(map[string]bool),
	}
	
	// Initialize dangerous SQL patterns
	service.initializeDangerousPatterns()
	
	return service
}

// initializeDangerousPatterns sets up regex patterns for detecting SQL injection
func (s *SQLSecurityService) initializeDangerousPatterns() {
	patterns := []string{
		// Union-based injection
		`(?i)\bunion\b.*\bselect\b`,
		`(?i)\bunion\b.*\ball\b.*\bselect\b`,
		
		// Boolean-based injection
		`(?i)\bor\b\s+\d+\s*=\s*\d+`,
		`(?i)\band\b\s+\d+\s*=\s*\d+`,
		`(?i)'\s*or\s*'.*'`,
		`(?i)'\s*and\s*'.*'`,
		
		// Time-based injection
		`(?i)\bwaitfor\b.*\bdelay\b`,
		`(?i)\bsleep\b\s*\(`,
		`(?i)\bbenchmark\b\s*\(`,
		
		// Stacked queries
		`(?i);\s*drop\b`,
		`(?i);\s*delete\b`,
		`(?i);\s*update\b`,
		`(?i);\s*insert\b`,
		`(?i);\s*create\b`,
		`(?i);\s*alter\b`,
		
		// Information disclosure
		`(?i)\binformation_schema\b`,
		`(?i)\bsys\.\b`,
		`(?i)\bmaster\.\b`,
		`(?i)\bmysql\.\b`,
		`(?i)\bpg_\w+`,
		
		// Function calls
		`(?i)\bexec\b\s*\(`,
		`(?i)\bexecute\b\s*\(`,
		`(?i)\beval\b\s*\(`,
		`(?i)\bcast\b\s*\(`,
		`(?i)\bconvert\b\s*\(`,
		
		// Comment patterns
		`--[^\r\n]*`,
		`/\*.*?\*/`,
		`#[^\r\n]*`,
		
		// Hex encoding
		`0x[0-9a-fA-F]+`,
		
		// Substring/char functions
		`(?i)\bsubstring\b\s*\(`,
		`(?i)\bchar\b\s*\(`,
		`(?i)\bash\b\s*\(`,
		`(?i)\bord\b\s*\(`,
		
		// Conditional statements
		`(?i)\bif\b\s*\(.*,.*,.*\)`,
		`(?i)\bcase\b.*\bwhen\b`,
	}
	
	s.dangerousPatterns = make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		s.dangerousPatterns[i] = regexp.MustCompile(pattern)
	}
}

// ValidateSQL checks SQL query for injection patterns
func (s *SQLSecurityService) ValidateSQL(query string, args ...interface{}) error {
	// Check for dangerous patterns
	for _, pattern := range s.dangerousPatterns {
		if pattern.MatchString(query) {
			s.logger.Warn("Dangerous SQL pattern detected",
				zap.String("pattern", pattern.String()),
				zap.String("query", s.sanitizeQueryForLogging(query)),
			)
			return fmt.Errorf("potentially dangerous SQL pattern detected")
		}
	}
	
	// Check arguments for injection patterns
	for i, arg := range args {
		if argStr, ok := arg.(string); ok {
			for _, pattern := range s.dangerousPatterns {
				if pattern.MatchString(argStr) {
					s.logger.Warn("Dangerous SQL pattern in argument",
						zap.Int("arg_index", i),
						zap.String("pattern", pattern.String()),
					)
					return fmt.Errorf("potentially dangerous SQL pattern in argument %d", i)
				}
			}
		}
	}
	
	return nil
}

// sanitizeQueryForLogging removes sensitive data from query for logging
func (s *SQLSecurityService) sanitizeQueryForLogging(query string) string {
	// Replace potential sensitive values with placeholders
	sensitivePatterns := []*regexp.Regexp{
		regexp.MustCompile(`'[^']*'`),          // String literals
		regexp.MustCompile(`"[^"]*"`),          // Quoted strings
		regexp.MustCompile(`\b\d{13,19}\b`),   // Potential credit card numbers
		regexp.MustCompile(`\b\d{9,11}\b`),    // Potential SSNs
	}
	
	result := query
	for _, pattern := range sensitivePatterns {
		result = pattern.ReplaceAllString(result, "'***'")
	}
	
	return result
}

// SecureGormLogger provides secure logging for GORM
type SecureGormLogger struct {
	logger                    *zap.Logger
	sqlSecurity              *SQLSecurityService
	slowThreshold            time.Duration
	logLevel                 logger.LogLevel
	ignoreRecordNotFoundError bool
}

// NewSecureGormLogger creates a new secure GORM logger
func NewSecureGormLogger(logger *zap.Logger, sqlSecurity *SQLSecurityService) *SecureGormLogger {
	return &SecureGormLogger{
		logger:                    logger,
		sqlSecurity:              sqlSecurity,
		slowThreshold:            200 * time.Millisecond,
		logLevel:                 logger.Warn,
		ignoreRecordNotFoundError: true,
	}
}

// LogMode sets log mode
func (l *SecureGormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

// Info logs info level
func (l *SecureGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Info {
		l.logger.Info(fmt.Sprintf(msg, data...))
	}
}

// Warn logs warn level
func (l *SecureGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Warn {
		l.logger.Warn(fmt.Sprintf(msg, data...))
	}
}

// Error logs error level
func (l *SecureGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Error {
		l.logger.Error(fmt.Sprintf(msg, data...))
	}
}

// Trace logs SQL queries with security checks
func (l *SecureGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.logLevel <= logger.Silent {
		return
	}
	
	elapsed := time.Since(begin)
	sql, rows := fc()
	
	// Security check on SQL query
	if secErr := l.sqlSecurity.ValidateSQL(sql); secErr != nil {
		l.logger.Error("SQL security violation",
			zap.String("sql", l.sqlSecurity.sanitizeQueryForLogging(sql)),
			zap.Error(secErr),
			zap.Duration("elapsed", elapsed),
		)
		return
	}
	
	// Log based on conditions
	switch {
	case err != nil && l.logLevel >= logger.Error && (!l.ignoreRecordNotFoundError || err != gorm.ErrRecordNotFound):
		l.logger.Error("SQL query error",
			zap.String("sql", l.sqlSecurity.sanitizeQueryForLogging(sql)),
			zap.Error(err),
			zap.Duration("elapsed", elapsed),
			zap.Int64("rows", rows),
		)
	case elapsed > l.slowThreshold && l.logLevel >= logger.Warn:
		l.logger.Warn("Slow SQL query",
			zap.String("sql", l.sqlSecurity.sanitizeQueryForLogging(sql)),
			zap.Duration("elapsed", elapsed),
			zap.Duration("threshold", l.slowThreshold),
			zap.Int64("rows", rows),
		)
	case l.logLevel >= logger.Info:
		l.logger.Info("SQL query executed",
			zap.String("sql", l.sqlSecurity.sanitizeQueryForLogging(sql)),
			zap.Duration("elapsed", elapsed),
			zap.Int64("rows", rows),
		)
	}
}

// SecureDB wraps gorm.DB with additional security
type SecureDB struct {
	*gorm.DB
	sqlSecurity *SQLSecurityService
	logger      *zap.Logger
}

// NewSecureDB creates a secure database wrapper
func NewSecureDB(db *gorm.DB, sqlSecurity *SQLSecurityService, logger *zap.Logger) *SecureDB {
	return &SecureDB{
		DB:          db,
		sqlSecurity: sqlSecurity,
		logger:      logger,
	}
}

// Raw executes raw SQL with security validation
func (s *SecureDB) Raw(sql string, values ...interface{}) *gorm.DB {
	if err := s.sqlSecurity.ValidateSQL(sql, values...); err != nil {
		s.logger.Error("Raw SQL security violation",
			zap.String("sql", s.sqlSecurity.sanitizeQueryForLogging(sql)),
			zap.Error(err),
		)
		return s.DB.AddError(fmt.Errorf("SQL security violation: %w", err))
	}
	
	return s.DB.Raw(sql, values...)
}

// Exec executes SQL with security validation
func (s *SecureDB) Exec(sql string, values ...interface{}) *gorm.DB {
	if err := s.sqlSecurity.ValidateSQL(sql, values...); err != nil {
		s.logger.Error("Exec SQL security violation",
			zap.String("sql", s.sqlSecurity.sanitizeQueryForLogging(sql)),
			zap.Error(err),
		)
		return s.DB.AddError(fmt.Errorf("SQL security violation: %w", err))
	}
	
	return s.DB.Exec(sql, values...)
}

// DatabaseSecurityConfig holds database security configuration
type DatabaseSecurityConfig struct {
	EnableQueryLogging     bool
	EnableSlowQueryLogging bool
	SlowQueryThreshold     time.Duration
	MaxQueryComplexity     int
	EnablePreparedStmts    bool
	MaxConnections         int
	ConnectionTimeout      time.Duration
	QueryTimeout           time.Duration
}

// ValidateQueryComplexity analyzes query complexity
func (s *SQLSecurityService) ValidateQueryComplexity(query string, maxComplexity int) error {
	complexity := s.calculateQueryComplexity(query)
	
	if complexity > maxComplexity {
		s.logger.Warn("Query complexity exceeded",
			zap.String("query", s.sanitizeQueryForLogging(query)),
			zap.Int("complexity", complexity),
			zap.Int("max_complexity", maxComplexity),
		)
		return fmt.Errorf("query complexity %d exceeds maximum %d", complexity, maxComplexity)
	}
	
	return nil
}

// calculateQueryComplexity calculates a simple complexity score
func (s *SQLSecurityService) calculateQueryComplexity(query string) int {
	complexity := 0
	queryUpper := strings.ToUpper(query)
	
	// Count complex operations
	complexPatterns := map[string]int{
		"JOIN":        2,
		"INNER JOIN":  2,
		"LEFT JOIN":   2,
		"RIGHT JOIN":  2,
		"OUTER JOIN":  3,
		"UNION":       3,
		"SUBQUERY":    4,
		"GROUP BY":    2,
		"ORDER BY":    1,
		"HAVING":      2,
		"DISTINCT":    2,
		"COUNT":       1,
		"SUM":         1,
		"AVG":         1,
		"MAX":         1,
		"MIN":         1,
	}
	
	for pattern, score := range complexPatterns {
		count := strings.Count(queryUpper, pattern)
		complexity += count * score
	}
	
	// Count nested parentheses (indicating subqueries)
	depth := 0
	maxDepth := 0
	for _, char := range query {
		if char == '(' {
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		} else if char == ')' {
			depth--
		}
	}
	complexity += maxDepth * 2
	
	return complexity
}

// ParameterSanitizer provides parameter sanitization
type ParameterSanitizer struct {
	logger *zap.Logger
}

// NewParameterSanitizer creates a new parameter sanitizer
func NewParameterSanitizer(logger *zap.Logger) *ParameterSanitizer {
	return &ParameterSanitizer{logger: logger}
}

// SanitizeParameters sanitizes database parameters
func (p *ParameterSanitizer) SanitizeParameters(params []driver.Value) []driver.Value {
	sanitized := make([]driver.Value, len(params))
	
	for i, param := range params {
		if str, ok := param.(string); ok {
			sanitized[i] = p.sanitizeStringParameter(str)
		} else {
			sanitized[i] = param
		}
	}
	
	return sanitized
}

// sanitizeStringParameter sanitizes string parameters
func (p *ParameterSanitizer) sanitizeStringParameter(value string) string {
	// Remove null bytes
	value = strings.ReplaceAll(value, "\x00", "")
	
	// Limit length
	if len(value) > 10000 {
		p.logger.Warn("Parameter length exceeded limit",
			zap.Int("length", len(value)),
			zap.Int("limit", 10000),
		)
		value = value[:10000]
	}
	
	// Remove dangerous Unicode characters
	var result strings.Builder
	for _, r := range value {
		// Allow printable ASCII and common Unicode
		if (r >= 32 && r <= 126) || (r >= 160 && r <= 65535) {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// WhitelistQuery adds a query to the whitelist
func (s *SQLSecurityService) WhitelistQuery(queryPattern string) {
	s.queryWhitelist[queryPattern] = true
}

// IsWhitelistedQuery checks if a query is whitelisted
func (s *SQLSecurityService) IsWhitelistedQuery(query string) bool {
	// Normalize query for comparison
	normalized := strings.ToUpper(strings.TrimSpace(query))
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	
	return s.queryWhitelist[normalized]
}

// DatabaseAuditLogger logs all database operations for security auditing
type DatabaseAuditLogger struct {
	logger *zap.Logger
}

// NewDatabaseAuditLogger creates a new database audit logger
func NewDatabaseAuditLogger(logger *zap.Logger) *DatabaseAuditLogger {
	return &DatabaseAuditLogger{logger: logger}
}

// LogDatabaseOperation logs database operations for auditing
func (d *DatabaseAuditLogger) LogDatabaseOperation(operation, table, userID, sessionID string, affected int64) {
	d.logger.Info("Database operation",
		zap.String("operation", operation),
		zap.String("table", table),
		zap.String("user_id", userID),
		zap.String("session_id", sessionID),
		zap.Int64("affected_rows", affected),
		zap.Time("timestamp", time.Now()),
	)
}

// LogSensitiveDataAccess logs access to sensitive data
func (d *DatabaseAuditLogger) LogSensitiveDataAccess(table, column, userID, purpose string) {
	d.logger.Warn("Sensitive data access",
		zap.String("table", table),
		zap.String("column", column),
		zap.String("user_id", userID),
		zap.String("purpose", purpose),
		zap.Time("timestamp", time.Now()),
	)
}