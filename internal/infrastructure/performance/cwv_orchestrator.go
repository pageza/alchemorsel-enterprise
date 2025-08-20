// Package performance provides Core Web Vitals optimization orchestration
package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/cache"
)

// CoreWebVitalsOrchestrator coordinates all Core Web Vitals optimization components
type CoreWebVitalsOrchestrator struct {
	config                  CWVOrchestratorConfig
	
	// Core Web Vitals optimizers
	lcpOptimizer           *LCPOptimizer
	clsStabilizer          *LayoutStabilizer
	inpEnhancer            *INPEnhancer
	rumSystem              *RUMSystem
	coreWebVitalsMonitor   *CoreWebVitalsMonitor
	
	// Infrastructure
	cacheClient            *cache.RedisClient
	optimizationPipeline   []OptimizationStage
	mutex                  sync.RWMutex
	lastOptimization       time.Time
	optimizationResults    OptimizationResults
	
	// Performance targets
	targets                PerformanceTargets
}

// CWVOrchestratorConfig configures the Core Web Vitals orchestrator
type CWVOrchestratorConfig struct {
	// Core Web Vitals optimization settings
	EnableLCPOptimization    bool          // Enable LCP optimization
	EnableCLSStabilization   bool          // Enable CLS stabilization
	EnableINPEnhancement     bool          // Enable INP enhancement
	EnableRealUserMonitoring bool          // Enable RUM system
	EnableRedisCache         bool          // Enable Redis caching
	
	// Performance targets (Google's "Good" thresholds)
	TargetLCP              time.Duration // Target LCP (default: 2.5s)
	TargetCLS              float64       // Target CLS (default: 0.1)
	TargetINP              time.Duration // Target INP (default: 200ms)
	
	// Optimization settings
	OptimizationLevel        string        // conservative, balanced, aggressive
	EnableBundleOptimization bool          // Enable 14KB bundle optimization
	MaxBundleSize           int           // Maximum bundle size in bytes
	CacheTTL                time.Duration // Cache TTL for optimizations
	
	// Monitoring settings
	SampleRate              float64       // RUM sampling rate (0.0-1.0)
	EnableRealTimeAlerts    bool          // Enable real-time alerting
	AlertThresholds         CWVThresholds // Alert thresholds
}

// OptimizationStage represents a stage in the optimization pipeline
type OptimizationStage struct {
	Name        string
	Function    func(context.Context, string) (string, error)
	Parallel    bool
	Critical    bool
	Timeout     time.Duration
	Retries     int
}

// OptimizationResults contains the results of optimization operations
type OptimizationResults struct {
	Success            bool
	Duration           time.Duration
	OptimizationsApplied []string
	Errors             []string
	Warnings           []string
	StartTime          time.Time
	EndTime            time.Time
	
	// Core Web Vitals results
	CoreWebVitalsScore    CoreWebVitalsScore
	OptimizationSummary   OptimizationSummary
	PerformanceImpact     PerformanceImpact
}

// PerformanceTargets defines Core Web Vitals targets
type PerformanceTargets struct {
	LCP time.Duration // Largest Contentful Paint target
	CLS float64       // Cumulative Layout Shift target
	INP time.Duration // Interaction to Next Paint target
}

// CoreWebVitalsScore represents the overall Core Web Vitals score
type CoreWebVitalsScore struct {
	OverallScore   float64 // 0-100
	LCPScore       float64 // 0-100
	CLSScore       float64 // 0-100
	INPScore       float64 // 0-100
	Passing        bool    // All metrics meet targets
	Grade          string  // A, B, C, D, F
}

// OptimizationSummary summarizes applied optimizations
type OptimizationSummary struct {
	LCPOptimizations    []string
	CLSOptimizations    []string
	INPOptimizations    []string
	BundleOptimizations []string
	CacheOptimizations  []string
	TotalOptimizations  int
}

// PerformanceImpact estimates the performance impact
type PerformanceImpact struct {
	EstimatedLCPImprovement time.Duration
	EstimatedCLSImprovement float64
	EstimatedINPImprovement time.Duration
	BundleSizeReduction     int64
	CacheHitRatio          float64
	OverallImprovement     float64 // percentage
}

// DefaultCWVOrchestratorConfig returns sensible defaults for Core Web Vitals optimization
func DefaultCWVOrchestratorConfig() CWVOrchestratorConfig {
	return CWVOrchestratorConfig{
		// Core Web Vitals optimization (enabled by default)
		EnableLCPOptimization:    true,
		EnableCLSStabilization:   true,
		EnableINPEnhancement:     true,
		EnableRealUserMonitoring: true,
		EnableRedisCache:         true,
		
		// Performance targets (Google's "Good" thresholds)
		TargetLCP:              2500 * time.Millisecond, // 2.5s
		TargetCLS:              0.1,                     // 0.1
		TargetINP:              200 * time.Millisecond,  // 200ms
		
		// Optimization settings
		OptimizationLevel:        "balanced",
		EnableBundleOptimization: true,
		MaxBundleSize:           14 * 1024, // 14KB
		CacheTTL:                1 * time.Hour,
		
		// Monitoring settings
		SampleRate:           0.05, // 5% sampling rate
		EnableRealTimeAlerts: true,
		AlertThresholds:      DefaultCWVThresholds(),
	}
}

// DefaultPerformanceTargets returns the Google Core Web Vitals "Good" thresholds
func DefaultPerformanceTargets() PerformanceTargets {
	return PerformanceTargets{
		LCP: 2500 * time.Millisecond, // 2.5 seconds
		CLS: 0.1,                     // 0.1
		INP: 200 * time.Millisecond,  // 200 milliseconds
	}
}

// NewCoreWebVitalsOrchestrator creates a new Core Web Vitals optimization orchestrator
func NewCoreWebVitalsOrchestrator(config CWVOrchestratorConfig, cacheClient *cache.RedisClient) (*CoreWebVitalsOrchestrator, error) {
	// Set performance targets
	targets := PerformanceTargets{
		LCP: config.TargetLCP,
		CLS: config.TargetCLS,
		INP: config.TargetINP,
	}
	
	// Initialize Core Web Vitals optimizers
	var lcpOptimizer *LCPOptimizer
	var clsStabilizer *LayoutStabilizer
	var inpEnhancer *INPEnhancer
	var rumSystem *RUMSystem
	var coreWebVitalsMonitor *CoreWebVitalsMonitor
	
	if config.EnableLCPOptimization {
		lcpConfig := DefaultLCPConfig()
		lcpConfig.TargetLCP = config.TargetLCP
		lcpConfig.EnableRedisCache = config.EnableRedisCache
		lcpConfig.EnableBundleOptimization = config.EnableBundleOptimization
		lcpConfig.MaxBundleSize = config.MaxBundleSize
		lcpConfig.CacheTTL = config.CacheTTL
		lcpOptimizer = NewLCPOptimizer(lcpConfig, cacheClient)
	}
	
	if config.EnableCLSStabilization {
		clsConfig := DefaultLayoutStabilityConfig()
		clsConfig.MaxCLSScore = config.TargetCLS
		clsStabilizer = NewLayoutStabilizer(clsConfig)
	}
	
	if config.EnableINPEnhancement {
		inpConfig := DefaultINPConfig()
		inpConfig.TargetINP = config.TargetINP
		inpConfig.EnableHTMXOptimization = true
		inpEnhancer = NewINPEnhancer(inpConfig)
	}
	
	if config.EnableRealUserMonitoring {
		rumConfig := DefaultRUMConfig()
		rumConfig.SampleRate = config.SampleRate
		rumConfig.EnableRealTimeAlerts = config.EnableRealTimeAlerts
		rumConfig.AlertThresholds = config.AlertThresholds
		rumSystem = NewRUMSystem(rumConfig, cacheClient)
		
		// Initialize Core Web Vitals monitor
		cwvConfig := DefaultCWVConfig()
		cwvConfig.AlertThresholds = config.AlertThresholds
		coreWebVitalsMonitor = NewCoreWebVitalsMonitor(cwvConfig)
	}
	
	orchestrator := &CoreWebVitalsOrchestrator{
		config:                  config,
		targets:                 targets,
		
		// Core Web Vitals optimizers
		lcpOptimizer:           lcpOptimizer,
		clsStabilizer:          clsStabilizer,
		inpEnhancer:            inpEnhancer,
		rumSystem:              rumSystem,
		coreWebVitalsMonitor:   coreWebVitalsMonitor,
		
		// Infrastructure
		cacheClient:            cacheClient,
	}
	
	// Setup optimization pipeline
	orchestrator.setupOptimizationPipeline()
	
	return orchestrator, nil
}

// setupOptimizationPipeline configures the optimization pipeline
func (o *CoreWebVitalsOrchestrator) setupOptimizationPipeline() {
	o.optimizationPipeline = []OptimizationStage{
		{
			Name:     "LCP Optimization",
			Function: o.optimizeLCPStage,
			Parallel: false,
			Critical: true,
			Timeout:  2 * time.Minute,
			Retries:  1,
		},
		{
			Name:     "CLS Stabilization",
			Function: o.stabilizeCLSStage,
			Parallel: true,
			Critical: true,
			Timeout:  1 * time.Minute,
			Retries:  1,
		},
		{
			Name:     "INP Enhancement",
			Function: o.enhanceINPStage,
			Parallel: true,
			Critical: true,
			Timeout:  1 * time.Minute,
			Retries:  1,
		},
	}
}

// OptimizeHTML applies all Core Web Vitals optimizations to HTML content
func (o *CoreWebVitalsOrchestrator) OptimizeHTML(html string) (string, error) {
	return o.OptimizeHTMLWithContext(context.Background(), html)
}

// OptimizeHTMLWithContext applies all Core Web Vitals optimizations with context
func (o *CoreWebVitalsOrchestrator) OptimizeHTMLWithContext(ctx context.Context, html string) (string, error) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	
	startTime := time.Now()
	optimized := html
	var appliedOptimizations []string
	var errors []string
	var warnings []string
	
	// Execute optimization pipeline
	for _, stage := range o.optimizationPipeline {
		stageCtx, cancel := context.WithTimeout(ctx, stage.Timeout)
		defer cancel()
		
		var err error
		var stageResult string
		
		// Retry logic
		for attempt := 0; attempt <= stage.Retries; attempt++ {
			stageResult, err = stage.Function(stageCtx, optimized)
			if err == nil {
				break
			}
			
			if attempt == stage.Retries {
				errorMsg := fmt.Sprintf("Stage %s failed after %d retries: %v", stage.Name, stage.Retries, err)
				if stage.Critical {
					return "", fmt.Errorf(errorMsg)
				} else {
					errors = append(errors, errorMsg)
				}
			}
		}
		
		if err == nil && stageResult != "" {
			optimized = stageResult
			appliedOptimizations = append(appliedOptimizations, stage.Name)
		}
	}
	
	// Update results
	o.optimizationResults = OptimizationResults{
		Success:              len(errors) == 0,
		Duration:             time.Since(startTime),
		OptimizationsApplied: appliedOptimizations,
		Errors:               errors,
		Warnings:             warnings,
		StartTime:            startTime,
		EndTime:              time.Now(),
		CoreWebVitalsScore:   o.calculateCoreWebVitalsScore(),
		OptimizationSummary:  o.generateOptimizationSummary(appliedOptimizations),
		PerformanceImpact:    o.estimatePerformanceImpact(),
	}
	
	o.lastOptimization = time.Now()
	
	return optimized, nil
}

// optimizeLCPStage applies LCP optimizations
func (o *CoreWebVitalsOrchestrator) optimizeLCPStage(ctx context.Context, html string) (string, error) {
	if o.lcpOptimizer == nil {
		return html, nil
	}
	
	optimized, err := o.lcpOptimizer.OptimizeHTMLWithContext(ctx, html)
	if err != nil {
		return "", fmt.Errorf("LCP optimization failed: %w", err)
	}
	
	return optimized, nil
}

// stabilizeCLSStage applies CLS stabilization
func (o *CoreWebVitalsOrchestrator) stabilizeCLSStage(ctx context.Context, html string) (string, error) {
	if o.clsStabilizer == nil {
		return html, nil
	}
	
	optimized, err := o.clsStabilizer.StabilizeHTML(html)
	if err != nil {
		return "", fmt.Errorf("CLS stabilization failed: %w", err)
	}
	
	return optimized, nil
}

// enhanceINPStage applies INP enhancements
func (o *CoreWebVitalsOrchestrator) enhanceINPStage(ctx context.Context, html string) (string, error) {
	if o.inpEnhancer == nil {
		return html, nil
	}
	
	optimized, err := o.inpEnhancer.OptimizeHTML(html)
	if err != nil {
		return "", fmt.Errorf("INP enhancement failed: %w", err)
	}
	
	return optimized, nil
}

// RecordMeasurement records a Core Web Vitals measurement
func (o *CoreWebVitalsOrchestrator) RecordMeasurement(measurement CWVMeasurement) error {
	if o.coreWebVitalsMonitor == nil {
		return fmt.Errorf("Core Web Vitals monitor not enabled")
	}
	
	return o.coreWebVitalsMonitor.RecordMeasurement(measurement)
}

// GetCurrentPerformance returns current performance metrics
func (o *CoreWebVitalsOrchestrator) GetCurrentPerformance() *CoreWebVitalsPerformance {
	performance := &CoreWebVitalsPerformance{
		Targets:  o.targets,
		LastUpdate: o.lastOptimization,
	}
	
	if o.coreWebVitalsMonitor != nil {
		aggregates := o.coreWebVitalsMonitor.GetAggregates()
		performance.Current = CurrentMetrics{
			LCP: aggregates.LCP.P75,
			CLS: aggregates.CLS.P75,
			INP: aggregates.INP.P75,
		}
		
		performance.Score = o.calculateCoreWebVitalsScore()
	}
	
	return performance
}

// CoreWebVitalsPerformance represents current performance state
type CoreWebVitalsPerformance struct {
	Targets    PerformanceTargets
	Current    CurrentMetrics
	Score      CoreWebVitalsScore
	LastUpdate time.Time
}

// CurrentMetrics represents current metric values
type CurrentMetrics struct {
	LCP float64 // milliseconds
	CLS float64 // score
	INP float64 // milliseconds
}

// calculateCoreWebVitalsScore calculates the overall Core Web Vitals score
func (o *CoreWebVitalsOrchestrator) calculateCoreWebVitalsScore() CoreWebVitalsScore {
	if o.coreWebVitalsMonitor == nil {
		return CoreWebVitalsScore{Grade: "N/A"}
	}
	
	aggregates := o.coreWebVitalsMonitor.GetAggregates()
	
	// Calculate individual scores (0-100)
	lcpScore := o.calculateMetricScore(aggregates.LCP.P75, float64(o.targets.LCP.Milliseconds()), "LCP")
	clsScore := o.calculateMetricScore(aggregates.CLS.P75, o.targets.CLS, "CLS")
	inpScore := o.calculateMetricScore(aggregates.INP.P75, float64(o.targets.INP.Milliseconds()), "INP")
	
	// Calculate overall score (weighted average)
	overallScore := (lcpScore*0.4 + clsScore*0.3 + inpScore*0.3)
	
	// Determine if all metrics pass
	passing := aggregates.LCP.P75 <= float64(o.targets.LCP.Milliseconds()) &&
		aggregates.CLS.P75 <= o.targets.CLS &&
		aggregates.INP.P75 <= float64(o.targets.INP.Milliseconds())
	
	// Assign grade
	grade := o.assignGrade(overallScore)
	
	return CoreWebVitalsScore{
		OverallScore: overallScore,
		LCPScore:     lcpScore,
		CLSScore:     clsScore,
		INPScore:     inpScore,
		Passing:      passing,
		Grade:        grade,
	}
}

// calculateMetricScore calculates a 0-100 score for a metric
func (o *CoreWebVitalsOrchestrator) calculateMetricScore(current, target float64, metricType string) float64 {
	if current <= target {
		return 100.0
	}
	
	// Define "poor" thresholds based on Google's guidelines
	var poorThreshold float64
	switch metricType {
	case "LCP":
		poorThreshold = target * 1.6 // 4000ms for 2500ms target
	case "CLS":
		poorThreshold = target * 2.5 // 0.25 for 0.1 target
	case "INP":
		poorThreshold = target * 2.5 // 500ms for 200ms target
	default:
		poorThreshold = target * 2.0
	}
	
	if current >= poorThreshold {
		return 0.0
	}
	
	// Linear scale between target and poor threshold
	ratio := (current - target) / (poorThreshold - target)
	return 100.0 * (1.0 - ratio)
}

// assignGrade assigns a letter grade based on the overall score
func (o *CoreWebVitalsOrchestrator) assignGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// generateOptimizationSummary generates a summary of applied optimizations
func (o *CoreWebVitalsOrchestrator) generateOptimizationSummary(appliedOptimizations []string) OptimizationSummary {
	summary := OptimizationSummary{
		TotalOptimizations: len(appliedOptimizations),
	}
	
	for _, optimization := range appliedOptimizations {
		switch {
		case strings.Contains(optimization, "LCP"):
			summary.LCPOptimizations = append(summary.LCPOptimizations, optimization)
		case strings.Contains(optimization, "CLS"):
			summary.CLSOptimizations = append(summary.CLSOptimizations, optimization)
		case strings.Contains(optimization, "INP"):
			summary.INPOptimizations = append(summary.INPOptimizations, optimization)
		case strings.Contains(optimization, "Bundle"):
			summary.BundleOptimizations = append(summary.BundleOptimizations, optimization)
		case strings.Contains(optimization, "Cache"):
			summary.CacheOptimizations = append(summary.CacheOptimizations, optimization)
		}
	}
	
	return summary
}

// estimatePerformanceImpact estimates the performance impact of optimizations
func (o *CoreWebVitalsOrchestrator) estimatePerformanceImpact() PerformanceImpact {
	// This is a simplified estimation model
	// In a real implementation, you'd use historical data and ML models
	
	impact := PerformanceImpact{
		OverallImprovement: 0.0,
	}
	
	// Estimate LCP improvement
	if o.lcpOptimizer != nil {
		metrics := o.lcpOptimizer.GetMetrics()
		if metrics.TotalOptimizations > 0 {
			impact.EstimatedLCPImprovement = 500 * time.Millisecond // Conservative estimate
			impact.BundleSizeReduction = int64(o.config.MaxBundleSize) / 2 // Assume 50% reduction
		}
	}
	
	// Estimate CLS improvement
	if o.clsStabilizer != nil {
		impact.EstimatedCLSImprovement = 0.05 // Conservative estimate
	}
	
	// Estimate INP improvement
	if o.inpEnhancer != nil {
		impact.EstimatedINPImprovement = 50 * time.Millisecond // Conservative estimate
	}
	
	// Estimate cache hit ratio
	if o.config.EnableRedisCache {
		impact.CacheHitRatio = 0.8 // Assume 80% cache hit ratio
	}
	
	// Calculate overall improvement
	improvements := 0
	if impact.EstimatedLCPImprovement > 0 {
		improvements++
	}
	if impact.EstimatedCLSImprovement > 0 {
		improvements++
	}
	if impact.EstimatedINPImprovement > 0 {
		improvements++
	}
	
	if improvements > 0 {
		impact.OverallImprovement = float64(improvements) * 15.0 // 15% per metric improvement
	}
	
	return impact
}

// GenerateReport generates a comprehensive optimization report
func (o *CoreWebVitalsOrchestrator) GenerateReport() string {
	performance := o.GetCurrentPerformance()
	results := o.optimizationResults
	
	return fmt.Sprintf(`=== Core Web Vitals Optimization Report ===
Generated: %s
Last Optimization: %s

=== Performance Targets vs Current ===
LCP Target: %v | Current: %.0fms | Score: %.1f
CLS Target: %.3f | Current: %.3f | Score: %.1f
INP Target: %v | Current: %.0fms | Score: %.1f

=== Overall Score ===
Score: %.1f/100 | Grade: %s | Passing: %t

=== Optimization Summary ===
Total Optimizations Applied: %d
LCP Optimizations: %d
CLS Optimizations: %d
INP Optimizations: %d
Bundle Optimizations: %d
Cache Optimizations: %d

=== Performance Impact ===
Estimated LCP Improvement: %v
Estimated CLS Improvement: %.3f
Estimated INP Improvement: %v
Bundle Size Reduction: %d bytes
Overall Improvement: %.1f%%

=== Last Optimization Results ===
Success: %t
Duration: %v
Applied: %s
Errors: %s
Warnings: %s

=== Recommendations ===
%s
`,
		time.Now().Format(time.RFC3339),
		o.lastOptimization.Format(time.RFC3339),
		
		// Performance comparison
		o.targets.LCP,
		performance.Current.LCP,
		performance.Score.LCPScore,
		o.targets.CLS,
		performance.Current.CLS,
		performance.Score.CLSScore,
		o.targets.INP,
		performance.Current.INP,
		performance.Score.INPScore,
		
		// Overall score
		performance.Score.OverallScore,
		performance.Score.Grade,
		performance.Score.Passing,
		
		// Optimization summary
		results.OptimizationSummary.TotalOptimizations,
		len(results.OptimizationSummary.LCPOptimizations),
		len(results.OptimizationSummary.CLSOptimizations),
		len(results.OptimizationSummary.INPOptimizations),
		len(results.OptimizationSummary.BundleOptimizations),
		len(results.OptimizationSummary.CacheOptimizations),
		
		// Performance impact
		results.PerformanceImpact.EstimatedLCPImprovement,
		results.PerformanceImpact.EstimatedCLSImprovement,
		results.PerformanceImpact.EstimatedINPImprovement,
		results.PerformanceImpact.BundleSizeReduction,
		results.PerformanceImpact.OverallImprovement,
		
		// Last optimization
		results.Success,
		results.Duration,
		strings.Join(results.OptimizationsApplied, ", "),
		strings.Join(results.Errors, "; "),
		strings.Join(results.Warnings, "; "),
		
		// Recommendations
		o.generateRecommendations(),
	)
}

// generateRecommendations generates optimization recommendations
func (o *CoreWebVitalsOrchestrator) generateRecommendations() string {
	var recommendations []string
	performance := o.GetCurrentPerformance()
	
	// LCP recommendations
	if performance.Current.LCP > float64(o.targets.LCP.Milliseconds()) {
		recommendations = append(recommendations, 
			"• Optimize LCP: Focus on image optimization and critical resource prioritization")
		if performance.Current.LCP > float64(o.targets.LCP.Milliseconds())*1.6 {
			recommendations = append(recommendations, 
				"• Critical: LCP is very poor - implement aggressive optimization strategies")
		}
	}
	
	// CLS recommendations
	if performance.Current.CLS > o.targets.CLS {
		recommendations = append(recommendations, 
			"• Reduce CLS: Add explicit dimensions to images and reserve space for dynamic content")
		if performance.Current.CLS > o.targets.CLS*2.5 {
			recommendations = append(recommendations, 
				"• Critical: High layout shift - implement comprehensive layout stability measures")
		}
	}
	
	// INP recommendations
	if performance.Current.INP > float64(o.targets.INP.Milliseconds()) {
		recommendations = append(recommendations, 
			"• Improve INP: Optimize JavaScript execution and implement task scheduling")
		if performance.Current.INP > float64(o.targets.INP.Milliseconds())*2.5 {
			recommendations = append(recommendations, 
				"• Critical: Very slow interactions - enable aggressive HTMX optimizations")
		}
	}
	
	// Configuration recommendations
	if !o.config.EnableRedisCache {
		recommendations = append(recommendations, 
			"• Enable Redis caching for better performance")
	}
	
	if !o.config.EnableBundleOptimization {
		recommendations = append(recommendations, 
			"• Enable 14KB bundle optimization for faster initial loads")
	}
	
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "• All Core Web Vitals are performing well!")
	}
	
	return strings.Join(recommendations, "\n")
}

// HTTPHandler returns HTTP handlers for Core Web Vitals APIs
func (o *CoreWebVitalsOrchestrator) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cwv/optimize":
			o.handleOptimize(w, r)
		case "/cwv/performance":
			o.handlePerformance(w, r)
		case "/cwv/report":
			o.handleReport(w, r)
		case "/cwv/record":
			o.handleRecord(w, r)
		default:
			http.NotFound(w, r)
		}
	}
}

func (o *CoreWebVitalsOrchestrator) handleOptimize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var request struct {
		HTML string `json:"html"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	optimized, err := o.OptimizeHTML(request.HTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	response := struct {
		OptimizedHTML string                `json:"optimized_html"`
		Results       OptimizationResults   `json:"results"`
	}{
		OptimizedHTML: optimized,
		Results:       o.optimizationResults,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (o *CoreWebVitalsOrchestrator) handlePerformance(w http.ResponseWriter, r *http.Request) {
	performance := o.GetCurrentPerformance()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(performance)
}

func (o *CoreWebVitalsOrchestrator) handleReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(o.GenerateReport()))
}

func (o *CoreWebVitalsOrchestrator) handleRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var measurement CWVMeasurement
	if err := json.NewDecoder(r.Body).Decode(&measurement); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	if err := o.RecordMeasurement(measurement); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "recorded"})
}

// TemplateFunction returns template functions for Core Web Vitals optimization
func (o *CoreWebVitalsOrchestrator) TemplateFunction() template.FuncMap {
	return template.FuncMap{
		"optimizeCWV": func(content string) template.HTML {
			optimized, err := o.OptimizeHTML(content)
			if err != nil {
				return template.HTML(content)
			}
			return template.HTML(optimized)
		},
		"cwvScore": func() CoreWebVitalsScore {
			return o.calculateCoreWebVitalsScore()
		},
		"cwvPerformance": func() *CoreWebVitalsPerformance {
			return o.GetCurrentPerformance()
		},
		"rumScript": func() template.HTML {
			return template.HTML(fmt.Sprintf(`
<script>
window.rumConfig = {
	endpoint: '/api/rum/collect',
	sampleRate: %f,
	enableDetailedMetrics: true,
	enableBusinessMetrics: true
};
</script>
<script src="/static/js/rum-client.js"></script>
`, o.config.SampleRate))
		},
	}
}