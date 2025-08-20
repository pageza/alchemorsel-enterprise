// Package main provides a standalone health check command for Alchemorsel v3
// This command can be used for Docker health checks, monitoring scripts, and debugging
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alchemorsel/v3/pkg/healthcheck"
	"github.com/alchemorsel/v3/pkg/logger"
	"go.uber.org/zap"
)

const (
	exitCodeSuccess = 0
	exitCodeFailure = 1
	exitCodeError   = 2
)

// Config holds command-line configuration
type Config struct {
	URL              string
	Timeout          time.Duration
	Verbose          bool
	OutputFormat     string
	CheckMode        string
	ExpectedStatus   string
	RetryCount       int
	RetryDelay       time.Duration
	ServiceName      string
	ConfigPath       string
	LocalCheck       bool
	CheckDependencies bool
	CheckCircuitBreakers bool
}

func main() {
	config := parseFlags()
	
	if config.LocalCheck {
		os.Exit(runLocalHealthCheck(config))
	} else {
		os.Exit(runRemoteHealthCheck(config))
	}
}

// parseFlags parses command-line flags
func parseFlags() Config {
	config := Config{}
	
	flag.StringVar(&config.URL, "url", "", "Health check endpoint URL (e.g., http://localhost:8080/health)")
	flag.DurationVar(&config.Timeout, "timeout", 10*time.Second, "Request timeout")
	flag.BoolVar(&config.Verbose, "verbose", false, "Verbose output")
	flag.StringVar(&config.OutputFormat, "format", "text", "Output format: text, json, compact")
	flag.StringVar(&config.CheckMode, "mode", "standard", "Check mode: quick, standard, deep, maintenance")
	flag.StringVar(&config.ExpectedStatus, "expect", "healthy", "Expected status: healthy, degraded, unhealthy")
	flag.IntVar(&config.RetryCount, "retry", 0, "Number of retries on failure")
	flag.DurationVar(&config.RetryDelay, "retry-delay", 1*time.Second, "Delay between retries")
	flag.StringVar(&config.ServiceName, "service", "alchemorsel", "Service name")
	flag.StringVar(&config.ConfigPath, "config", "", "Configuration file path")
	flag.BoolVar(&config.LocalCheck, "local", false, "Perform local health check instead of HTTP request")
	flag.BoolVar(&config.CheckDependencies, "dependencies", false, "Check dependencies only")
	flag.BoolVar(&config.CheckCircuitBreakers, "circuit-breakers", false, "Check circuit breakers only")
	
	flag.Parse()
	
	// Auto-detect URL if not provided
	if config.URL == "" && !config.LocalCheck {
		config.URL = detectHealthCheckURL()
	}
	
	return config
}

// detectHealthCheckURL attempts to detect the health check URL
func detectHealthCheckURL() string {
	// Check environment variables
	if url := os.Getenv("HEALTH_CHECK_URL"); url != "" {
		return url
	}
	
	// Check common ports and paths
	commonURLs := []string{
		"http://localhost:8080/health",
		"http://localhost:3000/health",
		"http://127.0.0.1:8080/health",
		"http://127.0.0.1:3000/health",
	}
	
	for _, url := range commonURLs {
		if checkURLReachable(url) {
			return url
		}
	}
	
	return "http://localhost:8080/health"
}

// checkURLReachable checks if a URL is reachable
func checkURLReachable(url string) bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// runRemoteHealthCheck performs a remote health check via HTTP
func runRemoteHealthCheck(config Config) int {
	client := &http.Client{Timeout: config.Timeout}
	
	var lastError error
	for attempt := 0; attempt <= config.RetryCount; attempt++ {
		if attempt > 0 {
			if config.Verbose {
				fmt.Printf("Retrying in %v... (attempt %d/%d)\n", config.RetryDelay, attempt, config.RetryCount)
			}
			time.Sleep(config.RetryDelay)
		}
		
		resp, err := client.Get(config.URL)
		if err != nil {
			lastError = err
			if config.Verbose {
				fmt.Printf("Request failed: %v\n", err)
			}
			continue
		}
		
		return handleResponse(resp, config)
	}
	
	fmt.Printf("Health check failed after %d attempts: %v\n", config.RetryCount+1, lastError)
	return exitCodeError
}

// runLocalHealthCheck performs a local health check
func runLocalHealthCheck(config Config) int {
	// Load configuration
	cfg, err := config.LoadConfig(config.ConfigPath)
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		return exitCodeError
	}
	
	// Create logger
	log, err := logger.New(logger.Config{
		Level:       "info",
		Format:      "json",
		Development: cfg.App.Debug,
	})
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		return exitCodeError
	}
	
	// Create health check instance
	var hc interface{}
	if cfg.Monitoring.HealthCheck.EnableEnterprise {
		hc = createEnterpriseHealthCheck(cfg, log)
	} else {
		hc = healthcheck.New(cfg.App.Version, log)
	}
	
	// Register health checks based on configuration
	registerHealthChecks(hc, cfg, log)
	
	// Perform health check
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	
	var result interface{}
	
	if ehc, ok := hc.(*healthcheck.EnterpriseHealthCheck); ok {
		if config.CheckDependencies {
			deps := ehc.CheckDependencies(ctx)
			result = deps
		} else if config.CheckCircuitBreakers {
			cb := ehc.GetCircuitBreakerStatus()
			result = cb
		} else {
			mode := parseCheckMode(config.CheckMode)
			result = ehc.CheckWithMode(ctx, mode)
		}
	} else if basic, ok := hc.(*healthcheck.HealthCheck); ok {
		result = basic.Check(ctx)
	}
	
	return outputResult(result, config)
}

// LoadConfig loads configuration from the given path
func (c Config) LoadConfig(configPath string) (*config.Config, error) {
	return config.Load(configPath)
}

// createEnterpriseHealthCheck creates an enterprise health check instance
func createEnterpriseHealthCheck(cfg *config.Config, log *zap.Logger) *healthcheck.EnterpriseHealthCheck {
	return healthcheck.NewEnterpriseHealthCheck(cfg.App.Version, log)
}

// registerHealthChecks registers health checks based on configuration
func registerHealthChecks(hc interface{}, cfg *config.Config, log *zap.Logger) {
	// This would register actual health checks
	// For now, we'll register some basic checks
	
	if ehc, ok := hc.(*healthcheck.EnterpriseHealthCheck); ok {
		// Register basic system check
		ehc.Register("system", healthcheck.NewCustomChecker("system", func(ctx context.Context) (healthcheck.Status, string, interface{}) {
			return healthcheck.StatusHealthy, "System operational", map[string]interface{}{
				"service": "alchemorsel-v3",
				"version": cfg.App.Version,
			}
		}))
	} else if basic, ok := hc.(*healthcheck.HealthCheck); ok {
		// Register basic system check
		basic.Register("system", healthcheck.NewCustomChecker("system", func(ctx context.Context) (healthcheck.Status, string, interface{}) {
			return healthcheck.StatusHealthy, "System operational", map[string]interface{}{
				"service": "alchemorsel-v3",
				"version": cfg.App.Version,
			}
		}))
	}
}

// parseCheckMode parses the check mode string
func parseCheckMode(mode string) healthcheck.HealthCheckMode {
	switch strings.ToLower(mode) {
	case "quick":
		return healthcheck.ModeQuick
	case "deep":
		return healthcheck.ModeDeep
	case "maintenance":
		return healthcheck.ModeMaintenance
	default:
		return healthcheck.ModeStandard
	}
}

// handleResponse handles the HTTP response
func handleResponse(resp *http.Response, config Config) int {
	defer resp.Body.Close()
	
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		fmt.Printf("Failed to decode response: %v\n", err)
		return exitCodeError
	}
	
	return outputResult(response, config)
}

// outputResult outputs the result based on the configured format
func outputResult(result interface{}, config Config) int {
	status := extractStatus(result)
	
	switch config.OutputFormat {
	case "json":
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	case "compact":
		data, _ := json.Marshal(result)
		fmt.Println(string(data))
	default: // text
		outputText(result, config.Verbose)
	}
	
	// Determine exit code based on status
	expectedStatus := healthcheck.Status(config.ExpectedStatus)
	if status == expectedStatus {
		return exitCodeSuccess
	}
	
	if status == healthcheck.StatusUnhealthy {
		return exitCodeFailure
	}
	
	// For degraded status when expecting healthy
	if status == healthcheck.StatusDegraded && expectedStatus == healthcheck.StatusHealthy {
		return exitCodeFailure
	}
	
	return exitCodeSuccess
}

// extractStatus extracts the status from the result
func extractStatus(result interface{}) healthcheck.Status {
	if result == nil {
		return healthcheck.StatusUnhealthy
	}
	
	switch r := result.(type) {
	case healthcheck.Response:
		return r.Status
	case healthcheck.EnterpriseResponse:
		return r.Status
	case map[string]interface{}:
		if status, ok := r["status"].(string); ok {
			return healthcheck.Status(status)
		}
	}
	
	return healthcheck.StatusUnhealthy
}

// outputText outputs the result in text format
func outputText(result interface{}, verbose bool) {
	switch r := result.(type) {
	case healthcheck.Response:
		fmt.Printf("Status: %s\n", r.Status)
		fmt.Printf("Version: %s\n", r.Version)
		fmt.Printf("Timestamp: %s\n", r.Timestamp.Format(time.RFC3339))
		fmt.Printf("Duration: %dms\n", r.TotalDuration.Milliseconds())
		
		if verbose && len(r.Checks) > 0 {
			fmt.Println("\nChecks:")
			for _, check := range r.Checks {
				fmt.Printf("  %s: %s", check.Name, check.Status)
				if check.Message != "" {
					fmt.Printf(" (%s)", check.Message)
				}
				fmt.Printf(" [%dms]\n", check.Duration.Milliseconds())
			}
		}
		
	case healthcheck.EnterpriseResponse:
		fmt.Printf("Status: %s\n", r.Status)
		fmt.Printf("Version: %s\n", r.Version)
		fmt.Printf("Timestamp: %s\n", r.Timestamp.Format(time.RFC3339))
		fmt.Printf("Duration: %dms\n", r.TotalDuration.Milliseconds())
		
		if r.Maintenance != nil && r.Maintenance.Enabled {
			fmt.Printf("Maintenance: %s\n", r.Maintenance.Message)
		}
		
		if verbose {
			if len(r.Checks) > 0 {
				fmt.Println("\nChecks:")
				for _, check := range r.Checks {
					fmt.Printf("  %s: %s", check.Name, check.Status)
					if check.Message != "" {
						fmt.Printf(" (%s)", check.Message)
					}
					fmt.Printf(" [%dms]\n", check.Duration.Milliseconds())
				}
			}
			
			if len(r.Dependencies) > 0 {
				fmt.Println("\nDependencies:")
				for _, dep := range r.Dependencies {
					fmt.Printf("  %s (%s): %s", dep.Name, dep.Type, dep.Status)
					if dep.Critical {
						fmt.Print(" [CRITICAL]")
					}
					if dep.Message != "" {
						fmt.Printf(" (%s)", dep.Message)
					}
					fmt.Printf(" [%dms]\n", dep.Duration.Milliseconds())
				}
			}
			
			if len(r.CircuitBreakers) > 0 {
				fmt.Println("\nCircuit Breakers:")
				for name, cb := range r.CircuitBreakers {
					fmt.Printf("  %s: %s", name, cb.State)
					if cb.FailureCount > 0 {
						fmt.Printf(" (failures: %d)", cb.FailureCount)
					}
					fmt.Println()
				}
			}
		}
		
	case []healthcheck.DependencyStatus:
		fmt.Println("Dependencies:")
		for _, dep := range r {
			fmt.Printf("  %s (%s): %s", dep.Name, dep.Type, dep.Status)
			if dep.Critical {
				fmt.Print(" [CRITICAL]")
			}
			if dep.Message != "" {
				fmt.Printf(" (%s)", dep.Message)
			}
			fmt.Printf(" [%dms]\n", dep.Duration.Milliseconds())
		}
		
	case map[string]healthcheck.CircuitBreakerStatus:
		fmt.Println("Circuit Breakers:")
		for name, cb := range r {
			fmt.Printf("  %s: %s", name, cb.State)
			if cb.FailureCount > 0 {
				fmt.Printf(" (failures: %d)", cb.FailureCount)
			}
			fmt.Println()
		}
		
	case map[string]interface{}:
		if status, ok := r["status"].(string); ok {
			fmt.Printf("Status: %s\n", status)
		}
		if verbose {
			data, _ := json.MarshalIndent(r, "", "  ")
			fmt.Println(string(data))
		}
		
	default:
		fmt.Printf("Unknown result type: %T\n", result)
	}
}