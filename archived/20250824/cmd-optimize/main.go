// Package main provides CLI tool for 14KB first packet optimization
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/performance"
)

const (
	version = "1.0.0"
	banner  = `
    ╔═══════════════════════════════════════════════════════════════╗
    ║                14KB First Packet Optimizer                    ║
    ║                    Alchemorsel v3                             ║
    ║                      Version %s                             ║
    ╚═══════════════════════════════════════════════════════════════╝
`
)

// CLI represents the command-line interface
type CLI struct {
	orchestrator *performance.OptimizationOrchestrator
	config       CLIConfig
}

// CLIConfig configures CLI behavior
type CLIConfig struct {
	ProjectRoot    string
	Command        string
	Verbose        bool
	Watch          bool
	OutputFormat   string
	ConfigFile     string
	Force          bool
	DryRun         bool
}

func main() {
	// Parse command line flags
	config := parseFlags()

	// Print banner
	fmt.Printf(banner, version)

	// Create CLI instance
	cli, err := newCLI(config)
	if err != nil {
		log.Fatalf("Failed to initialize CLI: %v", err)
	}

	// Handle signals for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nReceived shutdown signal, cleaning up...")
		cancel()
	}()

	// Execute command
	if err := cli.execute(ctx); err != nil {
		log.Fatalf("Command failed: %v", err)
	}
}

// parseFlags parses command line flags
func parseFlags() CLIConfig {
	var config CLIConfig

	// Define flags
	flag.StringVar(&config.ProjectRoot, "project-root", ".", "Project root directory")
	flag.StringVar(&config.Command, "command", "build", "Command to execute (build, analyze, watch, report)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&config.Watch, "watch", false, "Enable watch mode for development")
	flag.StringVar(&config.OutputFormat, "format", "text", "Output format (text, json, html)")
	flag.StringVar(&config.ConfigFile, "config", "", "Configuration file path")
	flag.BoolVar(&config.Force, "force", false, "Force rebuild even if no changes detected")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Show what would be done without making changes")

	// Custom usage function
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "14KB First Packet Optimization Tool for Alchemorsel v3\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  build    - Build optimized assets (default)\n")
		fmt.Fprintf(os.Stderr, "  analyze  - Analyze current optimization status\n")
		fmt.Fprintf(os.Stderr, "  watch    - Watch for changes and rebuild automatically\n")
		fmt.Fprintf(os.Stderr, "  report   - Generate detailed optimization report\n")
		fmt.Fprintf(os.Stderr, "  validate - Validate 14KB compliance\n")
		fmt.Fprintf(os.Stderr, "  clean    - Clean build artifacts\n")
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --command=build --verbose\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --command=watch --project-root=/path/to/project\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --command=report --format=json\n", os.Args[0])
	}

	flag.Parse()

	// Validate flags
	if config.ProjectRoot == "" {
		config.ProjectRoot = "."
	}

	return config
}

// newCLI creates a new CLI instance
func newCLI(config CLIConfig) (*CLI, error) {
	// Create orchestrator configuration
	orchestratorConfig := performance.DefaultOrchestratorConfig()
	orchestratorConfig.ProjectRoot = config.ProjectRoot
	orchestratorConfig.WatchMode = config.Watch
	
	// Adjust paths relative to project root
	orchestratorConfig.StaticDir = filepath.Join(config.ProjectRoot, "web", "static")
	orchestratorConfig.TemplatesDir = filepath.Join(config.ProjectRoot, "internal", "infrastructure", "http", "server", "templates")
	orchestratorConfig.OutputDir = filepath.Join(config.ProjectRoot, "web", "static", "dist")
	orchestratorConfig.CacheDir = filepath.Join(config.ProjectRoot, ".cache", "optimization")

	// Create orchestrator
	orchestrator, err := performance.NewOptimizationOrchestrator(orchestratorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create orchestrator: %w", err)
	}

	return &CLI{
		orchestrator: orchestrator,
		config:       config,
	}, nil
}

// execute runs the specified command
func (cli *CLI) execute(ctx context.Context) error {
	switch cli.config.Command {
	case "build":
		return cli.executeBuild(ctx)
	case "analyze":
		return cli.executeAnalyze(ctx)
	case "watch":
		return cli.executeWatch(ctx)
	case "report":
		return cli.executeReport(ctx)
	case "validate":
		return cli.executeValidate(ctx)
	case "clean":
		return cli.executeClean(ctx)
	default:
		return fmt.Errorf("unknown command: %s", cli.config.Command)
	}
}

// executeBuild runs the build command
func (cli *CLI) executeBuild(ctx context.Context) error {
	fmt.Println("🚀 Starting 14KB optimization build...")

	if cli.config.DryRun {
		fmt.Println("📋 Dry run mode - showing what would be done:")
		fmt.Println("  ✓ Scan static assets")
		fmt.Println("  ✓ Extract critical CSS")
		fmt.Println("  ✓ Optimize HTMX elements")
		fmt.Println("  ✓ Bundle resources")
		fmt.Println("  ✓ Optimize templates")
		fmt.Println("  ✓ Validate 14KB compliance")
		fmt.Println("  ✓ Generate reports")
		return nil
	}

	startTime := time.Now()
	results, err := cli.orchestrator.BuildOptimized(ctx)
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Display results
	if results.Success {
		fmt.Printf("✅ Build completed successfully in %v\n", results.Duration)
		fmt.Printf("📊 Build Summary:\n")
		fmt.Printf("   • Files processed: %d\n", results.TotalFiles)
		fmt.Printf("   • Files optimized: %d\n", results.OptimizedFiles)
		fmt.Printf("   • Compliance rate: %.1f%%\n", results.ComplianceRate*100)
		fmt.Printf("   • Size savings: %d bytes\n", results.SizeSavings)

		if len(results.ComplianceViolations) > 0 {
			fmt.Printf("⚠️  Compliance Violations (%d):\n", len(results.ComplianceViolations))
			for _, violation := range results.ComplianceViolations {
				fmt.Printf("   • %s\n", violation)
			}
		}

		if len(results.Warnings) > 0 {
			fmt.Printf("⚠️  Warnings (%d):\n", len(results.Warnings))
			for _, warning := range results.Warnings {
				fmt.Printf("   • %s\n", warning)
			}
		}
	} else {
		fmt.Printf("❌ Build failed in %v\n", time.Since(startTime))
		fmt.Printf("🔍 Errors (%d):\n", len(results.Errors))
		for _, err := range results.Errors {
			fmt.Printf("   • %s\n", err)
		}
		return fmt.Errorf("build completed with errors")
	}

	return nil
}

// executeAnalyze runs the analyze command
func (cli *CLI) executeAnalyze(ctx context.Context) error {
	fmt.Println("🔍 Analyzing optimization status...")

	// Get current metrics
	monitor := cli.orchestrator.GetPerformanceMonitor()
	monitor.CollectSystemMetrics()
	metrics := monitor.GetMetrics()

	switch cli.config.OutputFormat {
	case "json":
		return cli.printJSON(metrics)
	case "html":
		return cli.generateHTMLReport(metrics)
	default:
		return cli.printTextAnalysis(metrics)
	}
}

// executeWatch runs the watch command
func (cli *CLI) executeWatch(ctx context.Context) error {
	fmt.Println("👀 Starting watch mode...")
	fmt.Println("📁 Watching for changes in:")
	fmt.Printf("   • Templates: %s\n", filepath.Join(cli.config.ProjectRoot, "internal/infrastructure/http/server/templates"))
	fmt.Printf("   • Static assets: %s\n", filepath.Join(cli.config.ProjectRoot, "web/static"))
	fmt.Println("Press Ctrl+C to stop watching")

	return cli.orchestrator.StartDevelopmentWatcher(ctx)
}

// executeReport runs the report command
func (cli *CLI) executeReport(ctx context.Context) error {
	fmt.Println("📋 Generating optimization report...")

	monitor := cli.orchestrator.GetPerformanceMonitor()
	report := monitor.GenerateReport()

	switch cli.config.OutputFormat {
	case "json":
		// Convert report to JSON format
		fmt.Println(`{"report": "` + report + `"}`)
	case "html":
		return cli.generateHTMLReport(monitor.GetMetrics())
	default:
		fmt.Println(report)
	}

	return nil
}

// executeValidate runs the validate command
func (cli *CLI) executeValidate(ctx context.Context) error {
	fmt.Println("✅ Validating 14KB compliance...")

	// Trigger a quick validation build
	results, err := cli.orchestrator.BuildOptimized(ctx)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if len(results.ComplianceViolations) == 0 {
		fmt.Printf("✅ All templates comply with 14KB limit\n")
		fmt.Printf("📊 Compliance rate: %.1f%%\n", results.ComplianceRate*100)
		return nil
	}

	fmt.Printf("❌ Found %d compliance violations:\n", len(results.ComplianceViolations))
	for _, violation := range results.ComplianceViolations {
		fmt.Printf("   • %s\n", violation)
	}

	return fmt.Errorf("compliance validation failed")
}

// executeClean runs the clean command
func (cli *CLI) executeClean(ctx context.Context) error {
	fmt.Println("🧹 Cleaning build artifacts...")

	// Clean output directory
	outputDir := filepath.Join(cli.config.ProjectRoot, "web", "static", "dist")
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output directory: %w", err)
	}

	// Clean cache directory
	cacheDir := filepath.Join(cli.config.ProjectRoot, ".cache", "optimization")
	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("failed to clean cache directory: %w", err)
	}

	fmt.Println("✅ Clean completed successfully")
	return nil
}

// printTextAnalysis prints analysis in text format
func (cli *CLI) printTextAnalysis(metrics *performance.PerformanceMetrics) error {
	fmt.Printf("📊 Optimization Analysis\n")
	fmt.Printf("========================\n\n")

	fmt.Printf("🎯 Overall Health Score: %.1f/100\n", metrics.OverallHealth.OverallScore)
	
	if metrics.OverallHealth.OverallScore >= 90 {
		fmt.Printf("   Status: 🟢 Excellent\n")
	} else if metrics.OverallHealth.OverallScore >= 70 {
		fmt.Printf("   Status: 🟡 Good\n")
	} else {
		fmt.Printf("   Status: 🔴 Needs Improvement\n")
	}

	fmt.Printf("\n📦 First Packet Compliance\n")
	fmt.Printf("   Compliance Rate: %.1f%%\n", metrics.FirstPacketCompliance.ComplianceRate*100)
	fmt.Printf("   Total Requests: %d\n", metrics.FirstPacketCompliance.TotalRequests)
	fmt.Printf("   Violations: %d\n", metrics.FirstPacketCompliance.ViolationRequests)
	fmt.Printf("   Average Size: %d bytes\n", metrics.FirstPacketCompliance.AverageFirstPacketSize)

	fmt.Printf("\n🗜️  Compression Performance\n")
	fmt.Printf("   Compression Rate: %.1f%%\n", metrics.CompressionEfficiency.CompressionRate*100)
	fmt.Printf("   Total Requests: %d\n", metrics.CompressionEfficiency.TotalRequests)
	fmt.Printf("   Bytes Saved: %d\n", metrics.CompressionEfficiency.TotalBytesSaved)

	fmt.Printf("\n📁 Resource Optimization\n")
	fmt.Printf("   Total Bundles: %d\n", metrics.ResourceOptimization.BundleCount)
	fmt.Printf("   Critical Assets: %d\n", metrics.ResourceOptimization.CriticalAssets)
	fmt.Printf("   Optimization Ratio: %.1f%%\n", metrics.ResourceOptimization.OptimizationRatio*100)

	fmt.Printf("\n⚡ HTMX Performance\n")
	fmt.Printf("   Total Elements: %d\n", metrics.HTMXPerformance.TotalElements)
	fmt.Printf("   Critical Elements: %d\n", metrics.HTMXPerformance.CriticalElements)
	fmt.Printf("   Deferred Elements: %d\n", metrics.HTMXPerformance.DeferredElements)

	fmt.Printf("\n🎨 CSS Optimization\n")
	fmt.Printf("   Critical CSS Size: %d bytes\n", metrics.CSSOptimization.CriticalCSSSize)
	fmt.Printf("   Total Selectors: %d\n", metrics.CSSOptimization.SelectorCount)
	fmt.Printf("   Critical Selectors: %d\n", metrics.CSSOptimization.CriticalSelectors)

	// Recommendations
	fmt.Printf("\n💡 Recommendations\n")
	if metrics.FirstPacketCompliance.ComplianceRate < 0.9 {
		fmt.Printf("   • Improve first packet compliance (currently %.1f%%)\n", 
			metrics.FirstPacketCompliance.ComplianceRate*100)
	}
	if metrics.CompressionEfficiency.CompressionRate < 0.8 {
		fmt.Printf("   • Increase compression rate (currently %.1f%%)\n", 
			metrics.CompressionEfficiency.CompressionRate*100)
	}
	if metrics.CSSOptimization.CriticalCSSSize > 8192 {
		fmt.Printf("   • Reduce critical CSS size (currently %d bytes)\n", 
			metrics.CSSOptimization.CriticalCSSSize)
	}

	return nil
}

// printJSON prints metrics in JSON format
func (cli *CLI) printJSON(metrics *performance.PerformanceMetrics) error {
	// Simplified JSON output - in production use encoding/json
	fmt.Printf(`{
  "overall_health": %.1f,
  "first_packet_compliance": %.1f,
  "compression_rate": %.1f,
  "resource_bundles": %d,
  "critical_css_size": %d,
  "htmx_elements": %d
}`,
		metrics.OverallHealth.OverallScore,
		metrics.FirstPacketCompliance.ComplianceRate*100,
		metrics.CompressionEfficiency.CompressionRate*100,
		metrics.ResourceOptimization.BundleCount,
		metrics.CSSOptimization.CriticalCSSSize,
		metrics.HTMXPerformance.TotalElements,
	)
	return nil
}

// generateHTMLReport generates an HTML report
func (cli *CLI) generateHTMLReport(metrics *performance.PerformanceMetrics) error {
	reportPath := filepath.Join(cli.config.ProjectRoot, "optimization-report.html")
	
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>14KB Optimization Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .metric { margin: 10px 0; padding: 15px; border-radius: 5px; }
        .excellent { background-color: #d4edda; border: 1px solid #c3e6cb; }
        .good { background-color: #fff3cd; border: 1px solid #ffeaa7; }
        .poor { background-color: #f8d7da; border: 1px solid #f5c6cb; }
        .score { font-size: 2em; font-weight: bold; }
    </style>
</head>
<body>
    <h1>14KB First Packet Optimization Report</h1>
    <p>Generated: %s</p>
    
    <div class="metric excellent">
        <h2>Overall Health Score</h2>
        <div class="score">%.1f/100</div>
    </div>
    
    <div class="metric">
        <h3>First Packet Compliance</h3>
        <p>Rate: %.1f%% | Average Size: %d bytes</p>
    </div>
    
    <div class="metric">
        <h3>Compression Performance</h3>
        <p>Rate: %.1f%% | Bytes Saved: %d</p>
    </div>
    
    <div class="metric">
        <h3>Resource Optimization</h3>
        <p>Bundles: %d | Critical Assets: %d</p>
    </div>
</body>
</html>`,
		time.Now().Format(time.RFC3339),
		metrics.OverallHealth.OverallScore,
		metrics.FirstPacketCompliance.ComplianceRate*100,
		metrics.FirstPacketCompliance.AverageFirstPacketSize,
		metrics.CompressionEfficiency.CompressionRate*100,
		metrics.CompressionEfficiency.TotalBytesSaved,
		metrics.ResourceOptimization.BundleCount,
		metrics.ResourceOptimization.CriticalAssets,
	)

	if err := os.WriteFile(reportPath, []byte(html), 0644); err != nil {
		return fmt.Errorf("failed to write HTML report: %w", err)
	}

	fmt.Printf("📄 HTML report generated: %s\n", reportPath)
	return nil
}