// Package performance provides first packet optimization for 14KB compliance
package performance

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

const (
	// MaxFirstPacketSize defines the 14KB limit for optimal TCP slow start
	MaxFirstPacketSize = 14336 // 14KB in bytes
	
	// Critical thresholds for different content types
	MaxCriticalCSS     = 8192  // 8KB for critical CSS
	MaxCriticalJS      = 2048  // 2KB for critical JavaScript
	MaxInlineHTML      = 4096  // 4KB for base HTML structure
	
	// Compression levels
	BrotliLevel = 6
	GzipLevel   = 6
)

// FirstPacketOptimizer manages template and resource optimization
type FirstPacketOptimizer struct {
	templates      map[string]*template.Template
	criticalCSS    string
	criticalJS     string
	compressionMap map[string]CompressionResult
	metrics        *OptimizationMetrics
}

// CompressionResult stores compression analysis
type CompressionResult struct {
	Original        int
	Gzipped         int
	Brotli          int
	CompressionType string
	Compliant       bool
}

// OptimizationMetrics tracks performance metrics
type OptimizationMetrics struct {
	TemplateCount       int
	TotalCompliant      int
	TotalViolations     int
	AverageSize         float64
	LargestTemplate     string
	LargestTemplateSize int
	LastAnalyzed        time.Time
}

// TemplateData represents data passed to templates
type TemplateData struct {
	Title       string
	Description string
	Keywords    string
	Content     template.HTML
	Messages    []Message
	Debug       bool
	BaseURL     string
	URL         string
}

// Message represents flash messages
type Message struct {
	Type    string
	Content string
}

// NewFirstPacketOptimizer creates a new optimizer instance
func NewFirstPacketOptimizer() *FirstPacketOptimizer {
	return &FirstPacketOptimizer{
		templates:      make(map[string]*template.Template),
		compressionMap: make(map[string]CompressionResult),
		metrics: &OptimizationMetrics{
			LastAnalyzed: time.Now(),
		},
	}
}

// AnalyzeTemplate analyzes a template for 14KB compliance
func (fpo *FirstPacketOptimizer) AnalyzeTemplate(name string, content []byte) (*CompressionResult, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("template content is empty")
	}

	// Compress with Gzip
	gzippedSize, err := fpo.compressGzip(content)
	if err != nil {
		return nil, fmt.Errorf("gzip compression failed: %w", err)
	}

	// Compress with Brotli
	brotliSize, err := fpo.compressBrotli(content)
	if err != nil {
		return nil, fmt.Errorf("brotli compression failed: %w", err)
	}

	// Determine best compression
	var finalSize int
	var compressionType string
	
	if brotliSize < gzippedSize {
		finalSize = brotliSize
		compressionType = "brotli"
	} else {
		finalSize = gzippedSize
		compressionType = "gzip"
	}

	result := &CompressionResult{
		Original:        len(content),
		Gzipped:         gzippedSize,
		Brotli:          brotliSize,
		CompressionType: compressionType,
		Compliant:       finalSize <= MaxFirstPacketSize,
	}

	fpo.compressionMap[name] = *result
	fpo.updateMetrics(name, result)

	return result, nil
}

// OptimizeHTML optimizes HTML content for first packet delivery
func (fpo *FirstPacketOptimizer) OptimizeHTML(html string) (string, error) {
	// Remove unnecessary whitespace while preserving semantics
	optimized := fpo.minifyHTML(html)
	
	// Inline critical CSS if available
	if fpo.criticalCSS != "" {
		optimized = fpo.inlineCriticalCSS(optimized)
	}
	
	// Inline critical JavaScript if small enough
	if fpo.criticalJS != "" && len(fpo.criticalJS) <= MaxCriticalJS {
		optimized = fpo.inlineCriticalJS(optimized)
	}
	
	return optimized, nil
}

// SetCriticalCSS sets the critical CSS to be inlined
func (fpo *FirstPacketOptimizer) SetCriticalCSS(css string) error {
	if len(css) > MaxCriticalCSS {
		return fmt.Errorf("critical CSS exceeds maximum size of %d bytes", MaxCriticalCSS)
	}
	fpo.criticalCSS = css
	return nil
}

// SetCriticalJS sets the critical JavaScript to be inlined
func (fpo *FirstPacketOptimizer) SetCriticalJS(js string) error {
	if len(js) > MaxCriticalJS {
		return fmt.Errorf("critical JavaScript exceeds maximum size of %d bytes", MaxCriticalJS)
	}
	fpo.criticalJS = js
	return nil
}

// ValidateCompliance validates that a template meets 14KB requirements
func (fpo *FirstPacketOptimizer) ValidateCompliance(name string, data interface{}) (*ComplianceReport, error) {
	tmpl, exists := fpo.templates[name]
	if !exists {
		return nil, fmt.Errorf("template %s not found", name)
	}

	// Render template to get actual size
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	// Optimize the rendered HTML
	optimized, err := fpo.OptimizeHTML(buf.String())
	if err != nil {
		return nil, fmt.Errorf("HTML optimization failed: %w", err)
	}

	// Analyze optimized content
	result, err := fpo.AnalyzeTemplate(name+"_rendered", []byte(optimized))
	if err != nil {
		return nil, fmt.Errorf("template analysis failed: %w", err)
	}

	report := &ComplianceReport{
		TemplateName:     name,
		OriginalSize:     buf.Len(),
		OptimizedSize:    len(optimized),
		CompressedSize:   result.Brotli,
		CompressionType:  result.CompressionType,
		Compliant:        result.Compliant,
		CompressionRatio: float64(result.Brotli) / float64(len(optimized)),
		Recommendations:  fpo.generateRecommendations(result),
		Timestamp:        time.Now(),
	}

	return report, nil
}

// ComplianceReport provides detailed analysis of template compliance
type ComplianceReport struct {
	TemplateName     string
	OriginalSize     int
	OptimizedSize    int
	CompressedSize   int
	CompressionType  string
	Compliant        bool
	CompressionRatio float64
	Recommendations  []string
	Timestamp        time.Time
}

// generateRecommendations provides optimization suggestions
func (fpo *FirstPacketOptimizer) generateRecommendations(result *CompressionResult) []string {
	var recommendations []string

	if !result.Compliant {
		excess := result.Brotli - MaxFirstPacketSize
		recommendations = append(recommendations, 
			fmt.Sprintf("Template exceeds 14KB limit by %d bytes", excess))
	}

	if result.Original > 50000 {
		recommendations = append(recommendations, 
			"Consider splitting large templates into smaller components")
	}

	compressionRatio := float64(result.Brotli) / float64(result.Original)
	if compressionRatio > 0.7 {
		recommendations = append(recommendations, 
			"Poor compression ratio - consider removing redundant content")
	}

	if result.Brotli > MaxFirstPacketSize*0.9 {
		recommendations = append(recommendations, 
			"Template is close to 14KB limit - monitor for future additions")
	}

	return recommendations
}

// compressGzip compresses content using Gzip
func (fpo *FirstPacketOptimizer) compressGzip(content []byte) (int, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, GzipLevel)
	if err != nil {
		return 0, err
	}
	
	if _, err := writer.Write(content); err != nil {
		writer.Close()
		return 0, err
	}
	
	if err := writer.Close(); err != nil {
		return 0, err
	}
	
	return buf.Len(), nil
}

// compressBrotli compresses content using Brotli
func (fpo *FirstPacketOptimizer) compressBrotli(content []byte) (int, error) {
	var buf bytes.Buffer
	writer := brotli.NewWriterLevel(&buf, BrotliLevel)
	
	if _, err := writer.Write(content); err != nil {
		writer.Close()
		return 0, err
	}
	
	if err := writer.Close(); err != nil {
		return 0, err
	}
	
	return buf.Len(), nil
}

// minifyHTML performs basic HTML minification
func (fpo *FirstPacketOptimizer) minifyHTML(html string) string {
	// Remove unnecessary whitespace between tags
	minified := strings.ReplaceAll(html, ">\n<", "><")
	minified = strings.ReplaceAll(minified, ">\t<", "><")
	minified = strings.ReplaceAll(minified, "> <", "><")
	
	// Remove extra whitespace in attributes
	minified = strings.ReplaceAll(minified, "  ", " ")
	
	// Remove HTML comments (except IE conditionals)
	lines := strings.Split(minified, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "<!--") {
			cleaned = append(cleaned, trimmed)
		}
	}
	
	return strings.Join(cleaned, "")
}

// inlineCriticalCSS inlines critical CSS into the HTML
func (fpo *FirstPacketOptimizer) inlineCriticalCSS(html string) string {
	placeholder := "{{template \"critical-css\" .}}"
	return strings.ReplaceAll(html, placeholder, fpo.criticalCSS)
}

// inlineCriticalJS inlines critical JavaScript into the HTML
func (fpo *FirstPacketOptimizer) inlineCriticalJS(html string) string {
	placeholder := "/* CRITICAL_JS_PLACEHOLDER */"
	return strings.ReplaceAll(html, placeholder, fpo.criticalJS)
}

// updateMetrics updates internal metrics
func (fpo *FirstPacketOptimizer) updateMetrics(name string, result *CompressionResult) {
	fpo.metrics.TemplateCount++
	
	if result.Compliant {
		fpo.metrics.TotalCompliant++
	} else {
		fpo.metrics.TotalViolations++
	}
	
	// Update average size
	if fpo.metrics.TemplateCount == 1 {
		fpo.metrics.AverageSize = float64(result.Brotli)
	} else {
		fpo.metrics.AverageSize = (fpo.metrics.AverageSize*float64(fpo.metrics.TemplateCount-1) + 
			float64(result.Brotli)) / float64(fpo.metrics.TemplateCount)
	}
	
	// Track largest template
	if result.Brotli > fpo.metrics.LargestTemplateSize {
		fpo.metrics.LargestTemplate = name
		fpo.metrics.LargestTemplateSize = result.Brotli
	}
	
	fpo.metrics.LastAnalyzed = time.Now()
}

// GetMetrics returns current optimization metrics
func (fpo *FirstPacketOptimizer) GetMetrics() *OptimizationMetrics {
	return fpo.metrics
}

// GenerateReport generates a comprehensive optimization report
func (fpo *FirstPacketOptimizer) GenerateReport() string {
	var report strings.Builder
	
	report.WriteString("=== First Packet Optimization Report ===\n")
	report.WriteString(fmt.Sprintf("Analysis Date: %s\n", fpo.metrics.LastAnalyzed.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Templates Analyzed: %d\n", fpo.metrics.TemplateCount))
	report.WriteString(fmt.Sprintf("Compliant Templates: %d\n", fpo.metrics.TotalCompliant))
	report.WriteString(fmt.Sprintf("Non-compliant Templates: %d\n", fpo.metrics.TotalViolations))
	report.WriteString(fmt.Sprintf("Average Compressed Size: %.2f bytes\n", fpo.metrics.AverageSize))
	report.WriteString(fmt.Sprintf("Largest Template: %s (%d bytes)\n", 
		fpo.metrics.LargestTemplate, fpo.metrics.LargestTemplateSize))
	
	if fpo.metrics.TotalViolations > 0 {
		report.WriteString("\n=== Compliance Violations ===\n")
		for name, result := range fpo.compressionMap {
			if !result.Compliant {
				excess := result.Brotli - MaxFirstPacketSize
				report.WriteString(fmt.Sprintf("- %s: %d bytes (exceeds by %d bytes)\n", 
					name, result.Brotli, excess))
			}
		}
	}
	
	complianceRate := float64(fpo.metrics.TotalCompliant) / float64(fpo.metrics.TemplateCount) * 100
	report.WriteString(fmt.Sprintf("\nOverall Compliance Rate: %.1f%%\n", complianceRate))
	
	return report.String()
}

// MiddlewareFunc returns a middleware function for real-time optimization
func (fpo *FirstPacketOptimizer) MiddlewareFunc() func(next func([]byte) []byte) func([]byte) []byte {
	return func(next func([]byte) []byte) func([]byte) []byte {
		return func(content []byte) []byte {
			// Analyze content
			result, err := fpo.AnalyzeTemplate("middleware_analysis", content)
			if err != nil {
				log.Printf("First packet analysis failed: %v", err)
				return content
			}
			
			// Log compliance violations
			if !result.Compliant {
				log.Printf("14KB compliance violation detected: %d bytes (compressed: %d)", 
					len(content), result.Brotli)
			}
			
			// Apply optimizations
			optimized, err := fpo.OptimizeHTML(string(content))
			if err != nil {
				log.Printf("HTML optimization failed: %v", err)
				return content
			}
			
			return []byte(optimized)
		}
	}
}