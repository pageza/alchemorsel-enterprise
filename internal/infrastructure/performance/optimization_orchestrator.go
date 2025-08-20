// Package performance provides orchestration for 14KB first packet optimization system
package performance

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OptimizationOrchestrator coordinates all optimization components
type OptimizationOrchestrator struct {
	config                  OrchestratorConfig
	firstPacketOptimizer    *FirstPacketOptimizer
	criticalCSSExtractor    *CriticalCSSExtractor
	compressionMiddleware   *CompressionMiddleware
	htmxOptimizer          *HTMXOptimizer
	resourceBundler        *ResourceBundler
	performanceMonitor     *PerformanceMonitor
	buildCache             map[string]BuildCacheEntry
	optimizationPipeline   []OptimizationStage
	mutex                  sync.RWMutex
	lastBuildTime          time.Time
	buildResults           BuildResults
}

// OrchestratorConfig configures the optimization orchestrator
type OrchestratorConfig struct {
	ProjectRoot        string        // Root directory of the project
	StaticDir          string        // Static assets directory
	TemplatesDir       string        // Templates directory
	OutputDir          string        // Build output directory
	EnableBuildCache   bool          // Enable build caching
	CacheDir           string        // Cache directory
	WatchMode          bool          // Enable file watching for development
	BuildTimeout       time.Duration // Timeout for build operations
	ParallelStages     bool          // Enable parallel optimization stages
	ValidateCompliance bool          // Validate 14KB compliance after build
}

// BuildCacheEntry represents a cached build artifact
type BuildCacheEntry struct {
	Hash         string
	Size         int
	ComplianceOK bool
	CreatedAt    time.Time
	FilePath     string
}

// OptimizationStage represents a stage in the optimization pipeline
type OptimizationStage struct {
	Name        string
	Function    func(context.Context) error
	Parallel    bool
	Critical    bool
	Timeout     time.Duration
	Retries     int
}

// BuildResults contains the results of a build operation
type BuildResults struct {
	Success            bool
	Duration           time.Duration
	TotalFiles         int
	OptimizedFiles     int
	ComplianceViolations []string
	Warnings           []string
	Errors             []string
	SizeSavings        int64
	ComplianceRate     float64
	StartTime          time.Time
	EndTime            time.Time
}

// DefaultOrchestratorConfig returns sensible defaults
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		ProjectRoot:        ".",
		StaticDir:          "web/static",
		TemplatesDir:       "internal/infrastructure/http/server/templates",
		OutputDir:          "web/static/dist",
		EnableBuildCache:   true,
		CacheDir:           ".cache/optimization",
		WatchMode:          false,
		BuildTimeout:       5 * time.Minute,
		ParallelStages:     true,
		ValidateCompliance: true,
	}
}

// NewOptimizationOrchestrator creates a new optimization orchestrator
func NewOptimizationOrchestrator(config OrchestratorConfig) (*OptimizationOrchestrator, error) {
	// Initialize components with coordinated configs
	firstPacketOptimizer := NewFirstPacketOptimizer()
	
	cssConfig := CSSExtractConfig{
		FoldHeight:        600,
		MaxCriticalSize:   MaxCriticalCSS,
		OptimizationLevel: 2,
	}
	criticalCSSExtractor := NewCriticalCSSExtractor(cssConfig)
	
	compressionConfig := DefaultCompressionConfig()
	compressionMiddleware := NewCompressionMiddleware(compressionConfig, firstPacketOptimizer)
	
	htmxConfig := DefaultHTMXOptimizationConfig()
	htmxOptimizer := NewHTMXOptimizer(htmxConfig)
	
	bundlerConfig := DefaultBundleConfig()
	bundlerConfig.StaticDir = config.StaticDir
	bundlerConfig.OutputDir = config.OutputDir
	resourceBundler := NewResourceBundler(bundlerConfig)
	
	monitorConfig := DefaultMonitorConfig()
	performanceMonitor := NewPerformanceMonitor(
		monitorConfig, firstPacketOptimizer, compressionMiddleware, 
		resourceBundler, htmxOptimizer, criticalCSSExtractor)

	orchestrator := &OptimizationOrchestrator{
		config:                config,
		firstPacketOptimizer:  firstPacketOptimizer,
		criticalCSSExtractor:  criticalCSSExtractor,
		compressionMiddleware: compressionMiddleware,
		htmxOptimizer:        htmxOptimizer,
		resourceBundler:      resourceBundler,
		performanceMonitor:   performanceMonitor,
		buildCache:           make(map[string]BuildCacheEntry),
	}

	// Initialize build pipeline
	orchestrator.initializePipeline()

	// Create necessary directories
	if err := orchestrator.createDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	return orchestrator, nil
}

// initializePipeline sets up the optimization pipeline
func (oo *OptimizationOrchestrator) initializePipeline() {
	oo.optimizationPipeline = []OptimizationStage{
		{
			Name:     "scan_assets",
			Function: oo.scanAssets,
			Parallel: false,
			Critical: true,
			Timeout:  30 * time.Second,
			Retries:  2,
		},
		{
			Name:     "extract_critical_css",
			Function: oo.extractCriticalCSS,
			Parallel: true,
			Critical: true,
			Timeout:  60 * time.Second,
			Retries:  2,
		},
		{
			Name:     "optimize_htmx",
			Function: oo.optimizeHTMX,
			Parallel: true,
			Critical: false,
			Timeout:  45 * time.Second,
			Retries:  1,
		},
		{
			Name:     "bundle_resources",
			Function: oo.bundleResources,
			Parallel: false,
			Critical: true,
			Timeout:  120 * time.Second,
			Retries:  2,
		},
		{
			Name:     "optimize_templates",
			Function: oo.optimizeTemplates,
			Parallel: false,
			Critical: true,
			Timeout:  90 * time.Second,
			Retries:  2,
		},
		{
			Name:     "validate_compliance",
			Function: oo.validateCompliance,
			Parallel: false,
			Critical: true,
			Timeout:  30 * time.Second,
			Retries:  1,
		},
		{
			Name:     "generate_reports",
			Function: oo.generateReports,
			Parallel: false,
			Critical: false,
			Timeout:  15 * time.Second,
			Retries:  1,
		},
	}
}

// BuildOptimized performs a complete optimization build
func (oo *OptimizationOrchestrator) BuildOptimized(ctx context.Context) (*BuildResults, error) {
	startTime := time.Now()
	
	oo.buildResults = BuildResults{
		StartTime: startTime,
		Success:   false,
	}

	log.Printf("Starting 14KB optimization build...")

	// Create build context with timeout
	buildCtx, cancel := context.WithTimeout(ctx, oo.config.BuildTimeout)
	defer cancel()

	// Execute optimization pipeline
	if err := oo.executePipeline(buildCtx); err != nil {
		oo.buildResults.Errors = append(oo.buildResults.Errors, err.Error())
		oo.buildResults.EndTime = time.Now()
		oo.buildResults.Duration = oo.buildResults.EndTime.Sub(startTime)
		return &oo.buildResults, fmt.Errorf("build pipeline failed: %w", err)
	}

	// Finalize build results
	oo.buildResults.Success = true
	oo.buildResults.EndTime = time.Now()
	oo.buildResults.Duration = oo.buildResults.EndTime.Sub(startTime)
	oo.lastBuildTime = oo.buildResults.EndTime

	log.Printf("14KB optimization build completed successfully in %v", oo.buildResults.Duration)
	return &oo.buildResults, nil
}

// executePipeline executes the optimization pipeline
func (oo *OptimizationOrchestrator) executePipeline(ctx context.Context) error {
	if oo.config.ParallelStages {
		return oo.executeParallelPipeline(ctx)
	}
	return oo.executeSequentialPipeline(ctx)
}

// executeSequentialPipeline executes stages sequentially
func (oo *OptimizationOrchestrator) executeSequentialPipeline(ctx context.Context) error {
	for _, stage := range oo.optimizationPipeline {
		if err := oo.executeStage(ctx, stage); err != nil {
			if stage.Critical {
				return fmt.Errorf("critical stage %s failed: %w", stage.Name, err)
			}
			oo.buildResults.Warnings = append(oo.buildResults.Warnings, 
				fmt.Sprintf("Non-critical stage %s failed: %v", stage.Name, err))
			log.Printf("Warning: Stage %s failed but build continues: %v", stage.Name, err)
		}
	}
	return nil
}

// executeParallelPipeline executes stages with parallelization where possible
func (oo *OptimizationOrchestrator) executeParallelPipeline(ctx context.Context) error {
	var parallelStages []OptimizationStage
	
	for _, stage := range oo.optimizationPipeline {
		if stage.Parallel && len(parallelStages) == 0 {
			// Start collecting parallel stages
			parallelStages = append(parallelStages, stage)
		} else if stage.Parallel && len(parallelStages) > 0 {
			// Add to parallel group
			parallelStages = append(parallelStages, stage)
		} else {
			// Execute any pending parallel stages
			if len(parallelStages) > 0 {
				if err := oo.executeParallelStages(ctx, parallelStages); err != nil {
					return err
				}
				parallelStages = nil
			}
			
			// Execute this sequential stage
			if err := oo.executeStage(ctx, stage); err != nil {
				if stage.Critical {
					return fmt.Errorf("critical stage %s failed: %w", stage.Name, err)
				}
				oo.buildResults.Warnings = append(oo.buildResults.Warnings, 
					fmt.Sprintf("Stage %s failed: %v", stage.Name, err))
			}
		}
	}
	
	// Execute any remaining parallel stages
	if len(parallelStages) > 0 {
		return oo.executeParallelStages(ctx, parallelStages)
	}
	
	return nil
}

// executeParallelStages executes multiple stages in parallel
func (oo *OptimizationOrchestrator) executeParallelStages(ctx context.Context, stages []OptimizationStage) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(stages))
	
	for _, stage := range stages {
		wg.Add(1)
		go func(s OptimizationStage) {
			defer wg.Done()
			if err := oo.executeStage(ctx, s); err != nil {
				if s.Critical {
					errors <- fmt.Errorf("critical stage %s failed: %w", s.Name, err)
				} else {
					oo.buildResults.Warnings = append(oo.buildResults.Warnings, 
						fmt.Sprintf("Stage %s failed: %v", s.Name, err))
				}
			}
		}(stage)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for critical errors
	for err := range errors {
		if err != nil {
			return err
		}
	}
	
	return nil
}

// executeStage executes a single optimization stage with retries
func (oo *OptimizationOrchestrator) executeStage(ctx context.Context, stage OptimizationStage) error {
	stageCtx, cancel := context.WithTimeout(ctx, stage.Timeout)
	defer cancel()
	
	var lastErr error
	for attempt := 0; attempt <= stage.Retries; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying stage %s (attempt %d/%d)", stage.Name, attempt+1, stage.Retries+1)
		}
		
		log.Printf("Executing stage: %s", stage.Name)
		if err := stage.Function(stageCtx); err != nil {
			lastErr = err
			if attempt < stage.Retries {
				// Brief backoff before retry
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
		} else {
			log.Printf("Stage %s completed successfully", stage.Name)
			return nil
		}
	}
	
	return fmt.Errorf("stage %s failed after %d retries: %w", stage.Name, stage.Retries, lastErr)
}

// Pipeline stage implementations

func (oo *OptimizationOrchestrator) scanAssets(ctx context.Context) error {
	log.Printf("Scanning assets in %s", oo.config.StaticDir)
	return oo.resourceBundler.ScanAssets()
}

func (oo *OptimizationOrchestrator) extractCriticalCSS(ctx context.Context) error {
	log.Printf("Extracting critical CSS")
	
	// Read base template to understand structure
	basePath := filepath.Join(oo.config.TemplatesDir, "layout", "base.html")
	baseHTML, err := os.ReadFile(basePath)
	if err != nil {
		return fmt.Errorf("failed to read base template: %w", err)
	}
	
	// Read existing CSS files
	cssPath := filepath.Join(oo.config.StaticDir, "css", "main.css")
	if _, err := os.Stat(cssPath); os.IsNotExist(err) {
		// Try alternative CSS locations
		cssPath = filepath.Join(oo.config.StaticDir, "css", "style.css")
	}
	
	cssContent, err := os.ReadFile(cssPath)
	if err != nil {
		log.Printf("Warning: Could not read CSS file %s: %v", cssPath, err)
		// Continue with embedded critical CSS
		return nil
	}
	
	// Extract critical CSS
	critical, err := oo.criticalCSSExtractor.ExtractCriticalCSS(string(cssContent), string(baseHTML))
	if err != nil {
		return fmt.Errorf("critical CSS extraction failed: %w", err)
	}
	
	// Set critical CSS in optimizer
	return oo.firstPacketOptimizer.SetCriticalCSS(critical)
}

func (oo *OptimizationOrchestrator) optimizeHTMX(ctx context.Context) error {
	log.Printf("Optimizing HTMX elements")
	
	// Walk through templates and optimize HTMX usage
	templatesPath := oo.config.TemplatesDir
	return filepath.Walk(templatesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if filepath.Ext(path) != ".html" {
			return nil
		}
		
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		
		optimized, err := oo.htmxOptimizer.OptimizeHTML(string(content))
		if err != nil {
			log.Printf("Warning: HTMX optimization failed for %s: %v", path, err)
			return nil // Non-critical error
		}
		
		// Write back optimized content
		return os.WriteFile(path, []byte(optimized), info.Mode())
	})
}

func (oo *OptimizationOrchestrator) bundleResources(ctx context.Context) error {
	log.Printf("Creating optimized resource bundles")
	
	if err := oo.resourceBundler.CreateBundles(); err != nil {
		return fmt.Errorf("resource bundling failed: %w", err)
	}
	
	// Update build stats
	bundles := oo.resourceBundler.GetBundles()
	oo.buildResults.TotalFiles = len(bundles)
	oo.buildResults.OptimizedFiles = len(bundles)
	
	return nil
}

func (oo *OptimizationOrchestrator) optimizeTemplates(ctx context.Context) error {
	log.Printf("Optimizing templates for 14KB compliance")
	
	templatesPath := oo.config.TemplatesDir
	violationCount := 0
	
	err := filepath.Walk(templatesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if filepath.Ext(path) != ".html" {
			return nil
		}
		
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		
		// Analyze template compliance
		result, err := oo.firstPacketOptimizer.AnalyzeTemplate(path, content)
		if err != nil {
			log.Printf("Warning: Template analysis failed for %s: %v", path, err)
			return nil
		}
		
		if !result.Compliant {
			violationCount++
			violation := fmt.Sprintf("%s: %d bytes (exceeds 14KB by %d bytes)", 
				path, result.Brotli, result.Brotli-MaxFirstPacketSize)
			oo.buildResults.ComplianceViolations = append(oo.buildResults.ComplianceViolations, violation)
		}
		
		return nil
	})
	
	if err != nil {
		return err
	}
	
	// Calculate compliance rate
	if oo.buildResults.TotalFiles > 0 {
		oo.buildResults.ComplianceRate = float64(oo.buildResults.TotalFiles-violationCount) / 
			float64(oo.buildResults.TotalFiles)
	}
	
	return nil
}

func (oo *OptimizationOrchestrator) validateCompliance(ctx context.Context) error {
	if !oo.config.ValidateCompliance {
		return nil
	}
	
	log.Printf("Validating 14KB compliance")
	
	if len(oo.buildResults.ComplianceViolations) > 0 {
		log.Printf("Found %d compliance violations", len(oo.buildResults.ComplianceViolations))
		for _, violation := range oo.buildResults.ComplianceViolations {
			log.Printf("  - %s", violation)
		}
		
		// Don't fail build, but warn about violations
		oo.buildResults.Warnings = append(oo.buildResults.Warnings, 
			fmt.Sprintf("%d templates exceed 14KB limit", len(oo.buildResults.ComplianceViolations)))
	}
	
	return nil
}

func (oo *OptimizationOrchestrator) generateReports(ctx context.Context) error {
	log.Printf("Generating optimization reports")
	
	// Collect all component reports
	reports := []string{
		oo.firstPacketOptimizer.GenerateReport(),
		oo.compressionMiddleware.GetCompressionReport(),
		oo.resourceBundler.GetOptimizationReport(),
		oo.htmxOptimizer.GetOptimizationReport(),
		oo.criticalCSSExtractor.GetOptimizationReport(),
	}
	
	// Write reports to output directory
	reportsDir := filepath.Join(oo.config.OutputDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}
	
	reportNames := []string{
		"first-packet-optimization.txt",
		"compression-performance.txt",
		"resource-bundling.txt",
		"htmx-optimization.txt",
		"critical-css-extraction.txt",
	}
	
	for i, report := range reports {
		reportPath := filepath.Join(reportsDir, reportNames[i])
		if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
			log.Printf("Warning: Failed to write report %s: %v", reportPath, err)
		}
	}
	
	return nil
}

// createDirectories creates necessary directories for the build
func (oo *OptimizationOrchestrator) createDirectories() error {
	dirs := []string{
		oo.config.OutputDir,
		oo.config.CacheDir,
		filepath.Join(oo.config.OutputDir, "reports"),
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	return nil
}

// GetLastBuildResults returns the results of the last build
func (oo *OptimizationOrchestrator) GetLastBuildResults() BuildResults {
	oo.mutex.RLock()
	defer oo.mutex.RUnlock()
	return oo.buildResults
}

// GetPerformanceMonitor returns the performance monitor instance
func (oo *OptimizationOrchestrator) GetPerformanceMonitor() *PerformanceMonitor {
	return oo.performanceMonitor
}

// GetCompressionMiddleware returns the compression middleware for HTTP integration
func (oo *OptimizationOrchestrator) GetCompressionMiddleware() *CompressionMiddleware {
	return oo.compressionMiddleware
}

// StartDevelopmentWatcher starts file watching for development mode
func (oo *OptimizationOrchestrator) StartDevelopmentWatcher(ctx context.Context) error {
	if !oo.config.WatchMode {
		return nil
	}
	
	log.Printf("Starting development file watcher...")
	
	// This is a simplified watcher - in production you'd use fsnotify
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			// Check if any source files have changed
			if oo.shouldRebuild() {
				log.Printf("File changes detected, rebuilding...")
				if _, err := oo.BuildOptimized(ctx); err != nil {
					log.Printf("Development rebuild failed: %v", err)
				}
			}
		}
	}
}

// shouldRebuild checks if a rebuild is needed
func (oo *OptimizationOrchestrator) shouldRebuild() bool {
	// Simplified check - in production you'd track file modification times
	return time.Since(oo.lastBuildTime) > 10*time.Second
}

// GenerateBuildSummary generates a summary of the build process
func (oo *OptimizationOrchestrator) GenerateBuildSummary() string {
	results := oo.GetLastBuildResults()
	
	summary := fmt.Sprintf(`
=== 14KB First Packet Optimization Build Summary ===
Build Time: %s
Duration: %v
Status: %s

Files Processed: %d
Files Optimized: %d
Compliance Rate: %.1f%%
Size Savings: %d bytes

Compliance Violations: %d
Warnings: %d
Errors: %d

Performance Metrics:
- First Packet Compliance: %.1f%%
- Resource Optimization: %d bundles created
- Critical CSS: %d bytes
- HTMX Elements: %d optimized

`, 
		results.StartTime.Format(time.RFC3339),
		results.Duration,
		map[bool]string{true: "SUCCESS", false: "FAILED"}[results.Success],
		results.TotalFiles,
		results.OptimizedFiles,
		results.ComplianceRate*100,
		results.SizeSavings,
		len(results.ComplianceViolations),
		len(results.Warnings),
		len(results.Errors),
		oo.performanceMonitor.GetMetrics().FirstPacketCompliance.ComplianceRate*100,
		oo.performanceMonitor.GetMetrics().ResourceOptimization.BundleCount,
		oo.performanceMonitor.GetMetrics().CSSOptimization.CriticalCSSSize,
		oo.performanceMonitor.GetMetrics().HTMXPerformance.TotalElements,
	)
	
	if len(results.ComplianceViolations) > 0 {
		summary += "\nCompliance Violations:\n"
		for _, violation := range results.ComplianceViolations {
			summary += fmt.Sprintf("  - %s\n", violation)
		}
	}
	
	if len(results.Warnings) > 0 {
		summary += "\nWarnings:\n"
		for _, warning := range results.Warnings {
			summary += fmt.Sprintf("  - %s\n", warning)
		}
	}
	
	return summary
}