// Package postgres provides database index optimization
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// IndexOptimizer analyzes and optimizes database indexes
type IndexOptimizer struct {
	logger *zap.Logger
	db     *gorm.DB
}

// NewIndexOptimizer creates a new index optimizer
func NewIndexOptimizer(logger *zap.Logger) *IndexOptimizer {
	return &IndexOptimizer{
		logger: logger,
	}
}

// SetDB sets the database connection
func (io *IndexOptimizer) SetDB(db *gorm.DB) {
	io.db = db
}

// IndexUsageStats represents index usage statistics
type IndexUsageStats struct {
	SchemaName     string `json:"schema_name"`
	TableName      string `json:"table_name"`
	IndexName      string `json:"index_name"`
	IndexSize      int64  `json:"index_size"`
	TuplesFetched  int64  `json:"tuples_fetched"`
	TuplesRead     int64  `json:"tuples_read"`
	IndexScans     int64  `json:"index_scans"`
	IndexTupleReads int64 `json:"index_tuple_reads"`
	LastUsed       *time.Time `json:"last_used"`
	Effectiveness  float64 `json:"effectiveness"`
}

// TableStats represents table statistics
type TableStats struct {
	SchemaName    string  `json:"schema_name"`
	TableName     string  `json:"table_name"`
	RowCount      int64   `json:"row_count"`
	TableSize     int64   `json:"table_size"`
	IndexSize     int64   `json:"index_size"`
	SeqScans      int64   `json:"seq_scans"`
	SeqTupleReads int64   `json:"seq_tuple_reads"`
	IndexScans    int64   `json:"index_scans"`
	IndexTupleReads int64 `json:"index_tuple_reads"`
	IndexEfficiency float64 `json:"index_efficiency"`
}

// MissingIndexSuggestion represents a suggested missing index
type MissingIndexSuggestion struct {
	TableName     string   `json:"table_name"`
	Columns       []string `json:"columns"`
	Reason        string   `json:"reason"`
	Priority      string   `json:"priority"`
	Impact        string   `json:"impact"`
	CreateSQL     string   `json:"create_sql"`
	EstimatedGain float64  `json:"estimated_gain"`
}

// UnusedIndex represents an unused or low-usage index
type UnusedIndex struct {
	SchemaName   string    `json:"schema_name"`
	TableName    string    `json:"table_name"`
	IndexName    string    `json:"index_name"`
	IndexSize    int64     `json:"index_size"`
	LastUsed     *time.Time `json:"last_used"`
	UsageCount   int64     `json:"usage_count"`
	Recommendation string  `json:"recommendation"`
	DropSQL      string    `json:"drop_sql"`
}

// IndexAnalysisReport provides comprehensive index analysis
type IndexAnalysisReport struct {
	Timestamp           time.Time                `json:"timestamp"`
	TableStats          []TableStats             `json:"table_stats"`
	IndexUsageStats     []IndexUsageStats        `json:"index_usage_stats"`
	UnusedIndexes       []UnusedIndex            `json:"unused_indexes"`
	MissingIndexes      []MissingIndexSuggestion `json:"missing_indexes"`
	OverallIndexHealth  IndexHealthScore         `json:"overall_index_health"`
	Recommendations     []string                 `json:"recommendations"`
	OptimizationScript  string                   `json:"optimization_script"`
}

// IndexHealthScore represents overall index health
type IndexHealthScore struct {
	Score                float64 `json:"score"`
	IndexUsageRatio      float64 `json:"index_usage_ratio"`
	UnusedIndexCount     int     `json:"unused_index_count"`
	MissingIndexCount    int     `json:"missing_index_count"`
	OverallEfficiency    float64 `json:"overall_efficiency"`
	RecommendationCount  int     `json:"recommendation_count"`
}

// AnalyzeIndexes performs comprehensive index analysis
func (io *IndexOptimizer) AnalyzeIndexes(ctx context.Context) (*IndexAnalysisReport, error) {
	if io.db == nil {
		return nil, fmt.Errorf("database connection not set")
	}

	report := &IndexAnalysisReport{
		Timestamp: time.Now(),
	}

	// Get table statistics
	tableStats, err := io.getTableStats(ctx)
	if err != nil {
		io.logger.Error("Failed to get table stats", zap.Error(err))
	} else {
		report.TableStats = tableStats
	}

	// Get index usage statistics
	indexStats, err := io.getIndexUsageStats(ctx)
	if err != nil {
		io.logger.Error("Failed to get index usage stats", zap.Error(err))
	} else {
		report.IndexUsageStats = indexStats
	}

	// Find unused indexes
	unusedIndexes, err := io.findUnusedIndexes(ctx)
	if err != nil {
		io.logger.Error("Failed to find unused indexes", zap.Error(err))
	} else {
		report.UnusedIndexes = unusedIndexes
	}

	// Suggest missing indexes
	missingIndexes := io.suggestMissingIndexes(ctx, tableStats)
	report.MissingIndexes = missingIndexes

	// Calculate overall health score
	report.OverallIndexHealth = io.calculateIndexHealth(tableStats, indexStats, unusedIndexes, missingIndexes)

	// Generate recommendations
	report.Recommendations = io.generateRecommendations(report)

	// Generate optimization script
	report.OptimizationScript = io.generateOptimizationScript(report)

	return report, nil
}

// getTableStats retrieves table statistics
func (io *IndexOptimizer) getTableStats(ctx context.Context) ([]TableStats, error) {
	query := `
		SELECT 
			schemaname,
			tablename,
			n_tup_ins + n_tup_upd + n_tup_del as row_count,
			pg_total_relation_size(schemaname||'.'||tablename) as table_size,
			pg_indexes_size(schemaname||'.'||tablename) as index_size,
			seq_scan,
			seq_tup_read,
			idx_scan,
			idx_tup_fetch
		FROM pg_stat_user_tables
		WHERE schemaname = 'public'
		ORDER BY table_size DESC`

	rows, err := io.db.Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []TableStats
	for rows.Next() {
		var stat TableStats
		var seqScan, idxScan sql.NullInt64
		var seqTupRead, idxTupFetch sql.NullInt64

		err := rows.Scan(
			&stat.SchemaName,
			&stat.TableName,
			&stat.RowCount,
			&stat.TableSize,
			&stat.IndexSize,
			&seqScan,
			&seqTupRead,
			&idxScan,
			&idxTupFetch,
		)
		if err != nil {
			continue
		}

		if seqScan.Valid {
			stat.SeqScans = seqScan.Int64
		}
		if seqTupRead.Valid {
			stat.SeqTupleReads = seqTupRead.Int64
		}
		if idxScan.Valid {
			stat.IndexScans = idxScan.Int64
		}
		if idxTupFetch.Valid {
			stat.IndexTupleReads = idxTupFetch.Int64
		}

		// Calculate index efficiency
		totalScans := stat.SeqScans + stat.IndexScans
		if totalScans > 0 {
			stat.IndexEfficiency = float64(stat.IndexScans) / float64(totalScans) * 100
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// getIndexUsageStats retrieves index usage statistics
func (io *IndexOptimizer) getIndexUsageStats(ctx context.Context) ([]IndexUsageStats, error) {
	query := `
		SELECT 
			schemaname,
			tablename,
			indexname,
			pg_relation_size(schemaname||'.'||indexname) as index_size,
			idx_tup_fetch,
			idx_tup_read,
			idx_scan,
			idx_tup_read
		FROM pg_stat_user_indexes
		WHERE schemaname = 'public'
		ORDER BY idx_scan DESC`

	rows, err := io.db.Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []IndexUsageStats
	for rows.Next() {
		var stat IndexUsageStats
		var tuplesFetched, tuplesRead, indexScans, indexTupleReads sql.NullInt64

		err := rows.Scan(
			&stat.SchemaName,
			&stat.TableName,
			&stat.IndexName,
			&stat.IndexSize,
			&tuplesFetched,
			&tuplesRead,
			&indexScans,
			&indexTupleReads,
		)
		if err != nil {
			continue
		}

		if tuplesFetched.Valid {
			stat.TuplesFetched = tuplesFetched.Int64
		}
		if tuplesRead.Valid {
			stat.TuplesRead = tuplesRead.Int64
		}
		if indexScans.Valid {
			stat.IndexScans = indexScans.Int64
		}
		if indexTupleReads.Valid {
			stat.IndexTupleReads = indexTupleReads.Int64
		}

		// Calculate effectiveness
		if stat.IndexScans > 0 && stat.TuplesFetched > 0 {
			stat.Effectiveness = float64(stat.TuplesFetched) / float64(stat.IndexScans)
		}

		stats = append(stats, stat)
	}

	return stats, nil
}

// findUnusedIndexes identifies unused or rarely used indexes
func (io *IndexOptimizer) findUnusedIndexes(ctx context.Context) ([]UnusedIndex, error) {
	query := `
		SELECT 
			schemaname,
			tablename,
			indexname,
			pg_relation_size(schemaname||'.'||indexname) as index_size,
			idx_scan
		FROM pg_stat_user_indexes
		WHERE schemaname = 'public'
		AND idx_scan < 10  -- Rarely used
		AND indexname NOT LIKE '%_pkey'  -- Exclude primary keys
		ORDER BY index_size DESC`

	rows, err := io.db.Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var unused []UnusedIndex
	for rows.Next() {
		var index UnusedIndex
		var usageCount sql.NullInt64

		err := rows.Scan(
			&index.SchemaName,
			&index.TableName,
			&index.IndexName,
			&index.IndexSize,
			&usageCount,
		)
		if err != nil {
			continue
		}

		if usageCount.Valid {
			index.UsageCount = usageCount.Int64
		}

		// Generate recommendation
		if index.UsageCount == 0 {
			index.Recommendation = "Consider dropping - never used"
		} else {
			index.Recommendation = fmt.Sprintf("Consider dropping - only used %d times", index.UsageCount)
		}

		// Generate drop SQL
		index.DropSQL = fmt.Sprintf("DROP INDEX IF EXISTS %s.%s;", index.SchemaName, index.IndexName)

		unused = append(unused, index)
	}

	return unused, nil
}

// suggestMissingIndexes suggests potentially beneficial indexes
func (io *IndexOptimizer) suggestMissingIndexes(ctx context.Context, tableStats []TableStats) []MissingIndexSuggestion {
	var suggestions []MissingIndexSuggestion

	// Common patterns that benefit from indexes
	commonPatterns := []struct {
		table   string
		columns []string
		reason  string
		priority string
	}{
		{"recipes", []string{"status", "published_at"}, "Frequently filtered by status and ordered by publish date", "high"},
		{"recipes", []string{"author_id", "status"}, "User's recipes filtered by status", "high"},
		{"recipes", []string{"cuisine", "difficulty"}, "Recipe search by cuisine and difficulty", "medium"},
		{"recipes", []string{"average_rating", "likes_count"}, "Sorting by popularity metrics", "medium"},
		{"recipe_views", []string{"recipe_id", "viewed_at"}, "Analytics queries on recipe views", "medium"},
		{"recipe_likes", []string{"user_id", "created_at"}, "User activity queries", "low"},
		{"users", []string{"email", "is_active"}, "Login and user lookup queries", "high"},
		{"notifications", []string{"user_id", "is_read", "created_at"}, "User notifications lookup", "high"},
	}

	for _, pattern := range commonPatterns {
		// Check if this table exists in our stats
		var tableExists bool
		for _, stat := range tableStats {
			if stat.TableName == pattern.table {
				tableExists = true
				break
			}
		}

		if !tableExists {
			continue
		}

		suggestion := MissingIndexSuggestion{
			TableName: pattern.table,
			Columns:   pattern.columns,
			Reason:    pattern.reason,
			Priority:  pattern.priority,
			Impact:    io.estimateIndexImpact(pattern.priority),
		}

		// Generate CREATE INDEX SQL
		indexName := fmt.Sprintf("idx_%s_%s", pattern.table, strings.Join(pattern.columns, "_"))
		suggestion.CreateSQL = fmt.Sprintf(
			"CREATE INDEX CONCURRENTLY %s ON %s (%s);",
			indexName,
			pattern.table,
			strings.Join(pattern.columns, ", "),
		)

		// Estimate performance gain
		switch pattern.priority {
		case "high":
			suggestion.EstimatedGain = 70.0
		case "medium":
			suggestion.EstimatedGain = 40.0
		case "low":
			suggestion.EstimatedGain = 20.0
		}

		suggestions = append(suggestions, suggestion)
	}

	return suggestions
}

// estimateIndexImpact estimates the impact of an index
func (io *IndexOptimizer) estimateIndexImpact(priority string) string {
	switch priority {
	case "high":
		return "Significant performance improvement expected"
	case "medium":
		return "Moderate performance improvement expected"
	case "low":
		return "Minor performance improvement expected"
	default:
		return "Unknown impact"
	}
}

// calculateIndexHealth calculates overall index health score
func (io *IndexOptimizer) calculateIndexHealth(
	tableStats []TableStats,
	indexStats []IndexUsageStats,
	unusedIndexes []UnusedIndex,
	missingIndexes []MissingIndexSuggestion,
) IndexHealthScore {
	score := 100.0
	
	// Calculate index usage ratio
	var totalScans, indexScans int64
	for _, stat := range tableStats {
		totalScans += stat.SeqScans + stat.IndexScans
		indexScans += stat.IndexScans
	}
	
	var indexUsageRatio float64
	if totalScans > 0 {
		indexUsageRatio = float64(indexScans) / float64(totalScans) * 100
	}
	
	// Calculate overall efficiency
	var totalEfficiency float64
	var efficiencyCount int
	for _, stat := range tableStats {
		if stat.SeqScans+stat.IndexScans > 0 {
			totalEfficiency += stat.IndexEfficiency
			efficiencyCount++
		}
	}
	
	var overallEfficiency float64
	if efficiencyCount > 0 {
		overallEfficiency = totalEfficiency / float64(efficiencyCount)
	}
	
	// Penalize for unused indexes
	unusedPenalty := float64(len(unusedIndexes)) * 5.0
	score -= unusedPenalty
	
	// Penalize for missing high-priority indexes
	highPriorityMissing := 0
	for _, missing := range missingIndexes {
		if missing.Priority == "high" {
			highPriorityMissing++
		}
	}
	missingPenalty := float64(highPriorityMissing) * 10.0
	score -= missingPenalty
	
	// Penalize for low index usage
	if indexUsageRatio < 90 {
		usagePenalty := (90 - indexUsageRatio) * 0.5
		score -= usagePenalty
	}
	
	if score < 0 {
		score = 0
	}
	
	return IndexHealthScore{
		Score:               score,
		IndexUsageRatio:     indexUsageRatio,
		UnusedIndexCount:    len(unusedIndexes),
		MissingIndexCount:   len(missingIndexes),
		OverallEfficiency:   overallEfficiency,
		RecommendationCount: len(unusedIndexes) + len(missingIndexes),
	}
}

// generateRecommendations generates optimization recommendations
func (io *IndexOptimizer) generateRecommendations(report *IndexAnalysisReport) []string {
	var recommendations []string
	
	// Index usage recommendations
	if report.OverallIndexHealth.IndexUsageRatio < 90 {
		recommendations = append(recommendations, 
			fmt.Sprintf("Index usage ratio is %.1f%% - consider adding missing indexes", 
				report.OverallIndexHealth.IndexUsageRatio))
	}
	
	// Unused index recommendations
	if len(report.UnusedIndexes) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Found %d unused indexes - consider dropping to save space and improve write performance",
				len(report.UnusedIndexes)))
	}
	
	// Missing index recommendations
	highPriorityMissing := 0
	for _, missing := range report.MissingIndexes {
		if missing.Priority == "high" {
			highPriorityMissing++
		}
	}
	
	if highPriorityMissing > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Found %d high-priority missing indexes - immediate attention recommended",
				highPriorityMissing))
	}
	
	// Table-specific recommendations
	for _, stat := range report.TableStats {
		if stat.IndexEfficiency < 80 && stat.SeqScans+stat.IndexScans > 100 {
			recommendations = append(recommendations,
				fmt.Sprintf("Table '%s' has low index efficiency (%.1f%%) - review query patterns",
					stat.TableName, stat.IndexEfficiency))
		}
	}
	
	return recommendations
}

// generateOptimizationScript generates SQL script for optimization
func (io *IndexOptimizer) generateOptimizationScript(report *IndexAnalysisReport) string {
	var script strings.Builder
	
	script.WriteString("-- Database Index Optimization Script\n")
	script.WriteString(fmt.Sprintf("-- Generated: %s\n", report.Timestamp.Format(time.RFC3339)))
	script.WriteString("-- WARNING: Review each statement before execution\n\n")
	
	// Add missing indexes (high priority first)
	highPriorityMissing := make([]MissingIndexSuggestion, 0)
	otherMissing := make([]MissingIndexSuggestion, 0)
	
	for _, missing := range report.MissingIndexes {
		if missing.Priority == "high" {
			highPriorityMissing = append(highPriorityMissing, missing)
		} else {
			otherMissing = append(otherMissing, missing)
		}
	}
	
	if len(highPriorityMissing) > 0 {
		script.WriteString("-- HIGH PRIORITY: Add missing indexes\n")
		for _, missing := range highPriorityMissing {
			script.WriteString(fmt.Sprintf("-- %s\n", missing.Reason))
			script.WriteString(fmt.Sprintf("%s\n\n", missing.CreateSQL))
		}
	}
	
	if len(otherMissing) > 0 {
		script.WriteString("-- MEDIUM/LOW PRIORITY: Additional indexes\n")
		for _, missing := range otherMissing {
			script.WriteString(fmt.Sprintf("-- %s\n", missing.Reason))
			script.WriteString(fmt.Sprintf("%s\n\n", missing.CreateSQL))
		}
	}
	
	// Drop unused indexes
	if len(report.UnusedIndexes) > 0 {
		script.WriteString("-- Remove unused indexes (review carefully)\n")
		for _, unused := range report.UnusedIndexes {
			script.WriteString(fmt.Sprintf("-- %s\n", unused.Recommendation))
			script.WriteString(fmt.Sprintf("-- %s\n\n", unused.DropSQL))
		}
	}
	
	script.WriteString("-- End of optimization script\n")
	
	return script.String()
}

// OptimizeIndexesConcurrently runs index optimization concurrently
func (io *IndexOptimizer) OptimizeIndexesConcurrently(ctx context.Context, suggestions []MissingIndexSuggestion) error {
	for _, suggestion := range suggestions {
		if suggestion.Priority == "high" {
			io.logger.Info("Creating high-priority index",
				zap.String("table", suggestion.TableName),
				zap.Strings("columns", suggestion.Columns),
			)
			
			if err := io.db.WithContext(ctx).Exec(suggestion.CreateSQL).Error; err != nil {
				io.logger.Error("Failed to create index",
					zap.String("sql", suggestion.CreateSQL),
					zap.Error(err),
				)
				continue
			}
			
			io.logger.Info("Successfully created index",
				zap.String("table", suggestion.TableName),
				zap.Strings("columns", suggestion.Columns),
			)
		}
	}
	
	return nil
}