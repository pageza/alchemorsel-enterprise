// Package performance provides resource bundling and optimization for 14KB compliance
package performance

import (
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ResourceBundler manages static resource bundling and optimization
type ResourceBundler struct {
	config          BundleConfig
	bundles         map[string]*Bundle
	assetMap        map[string]AssetInfo
	dependencies    map[string][]string
	cacheBusters    map[string]string
	mutex           sync.RWMutex
	lastOptimized   time.Time
	optimizationLog []OptimizationEntry
}

// BundleConfig configures resource bundling behavior
type BundleConfig struct {
	StaticDir         string   // Directory containing static assets
	OutputDir         string   // Directory for optimized bundles
	EnableMinification bool     // Enable CSS/JS minification
	EnableSourceMaps   bool     // Generate source maps
	CriticalBundleSize int      // Max size for critical bundles
	ChunkSize         int      // Target chunk size for code splitting
	EnableCacheBusting bool     // Add hash to filenames
	PreloadCritical   bool     // Add preload hints for critical resources
	SupportedFormats  []string // Supported asset formats
	CompressionLevel  int      // Compression level for bundled assets
}

// Bundle represents a collection of related assets
type Bundle struct {
	Name         string
	Type         string // css, js, images
	Assets       []AssetInfo
	Critical     bool
	Size         int
	CompressedSize int
	Hash         string
	Dependencies []string
	LoadPriority int
	CreatedAt    time.Time
}

// AssetInfo contains metadata about an asset
type AssetInfo struct {
	Path         string
	Size         int
	Type         string
	Critical     bool
	Dependencies []string
	Hash         string
	LastModified time.Time
}

// OptimizationEntry logs optimization activities
type OptimizationEntry struct {
	Timestamp   time.Time
	Action      string
	Asset       string
	SizeBefore  int
	SizeAfter   int
	Compression float64
}

// DefaultBundleConfig returns sensible defaults
func DefaultBundleConfig() BundleConfig {
	return BundleConfig{
		StaticDir:          "web/static",
		OutputDir:          "web/static/dist",
		EnableMinification: true,
		EnableSourceMaps:   false, // Disabled for production 14KB optimization
		CriticalBundleSize: MaxCriticalCSS, // 8KB for critical CSS
		ChunkSize:          32 * 1024, // 32KB chunks
		EnableCacheBusting: true,
		PreloadCritical:    true,
		CompressionLevel:   6,
		SupportedFormats: []string{
			".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg",
		},
	}
}

// NewResourceBundler creates a new resource bundler
func NewResourceBundler(config BundleConfig) *ResourceBundler {
	return &ResourceBundler{
		config:       config,
		bundles:      make(map[string]*Bundle),
		assetMap:     make(map[string]AssetInfo),
		dependencies: make(map[string][]string),
		cacheBusters: make(map[string]string),
	}
}

// ScanAssets discovers and catalogs all static assets
func (rb *ResourceBundler) ScanAssets() error {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	// Clear existing asset map
	rb.assetMap = make(map[string]AssetInfo)

	// Walk through static directory
	err := filepath.WalkDir(rb.config.StaticDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Check if file type is supported
		ext := filepath.Ext(path)
		if !rb.isSupportedFormat(ext) {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return err
		}

		// Create asset info
		relativePath, _ := filepath.Rel(rb.config.StaticDir, path)
		asset := AssetInfo{
			Path:         relativePath,
			Size:         int(info.Size()),
			Type:         rb.getAssetType(ext),
			LastModified: info.ModTime(),
		}

		// Calculate hash for cache busting
		if rb.config.EnableCacheBusting {
			hash, err := rb.calculateFileHash(path)
			if err == nil {
				asset.Hash = hash[:8] // Use first 8 characters
			}
		}

		// Determine if asset is critical
		asset.Critical = rb.isCriticalAsset(relativePath)

		rb.assetMap[relativePath] = asset
		return nil
	})

	if err != nil {
		return fmt.Errorf("asset scanning failed: %w", err)
	}

	// Analyze dependencies
	if err := rb.analyzeDependencies(); err != nil {
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	return nil
}

// CreateBundles creates optimized bundles from discovered assets
func (rb *ResourceBundler) CreateBundles() error {
	rb.mutex.Lock()
	defer rb.mutex.Unlock()

	// Create bundles by type and criticality
	cssBundles := rb.createCSSBundles()
	jsBundles := rb.createJSBundles()
	imageBundles := rb.createImageBundles()

	// Merge all bundles
	allBundles := make(map[string]*Bundle)
	for name, bundle := range cssBundles {
		allBundles[name] = bundle
	}
	for name, bundle := range jsBundles {
		allBundles[name] = bundle
	}
	for name, bundle := range imageBundles {
		allBundles[name] = bundle
	}

	rb.bundles = allBundles

	// Generate bundled files
	return rb.generateBundleFiles()
}

// createCSSBundles creates CSS bundles optimized for 14KB delivery
func (rb *ResourceBundler) createCSSBundles() map[string]*Bundle {
	bundles := make(map[string]*Bundle)

	var criticalCSS, nonCriticalCSS []AssetInfo
	for _, asset := range rb.assetMap {
		if asset.Type == "css" {
			if asset.Critical {
				criticalCSS = append(criticalCSS, asset)
			} else {
				nonCriticalCSS = append(nonCriticalCSS, asset)
			}
		}
	}

	// Create critical CSS bundle (must fit in first packet)
	if len(criticalCSS) > 0 {
		criticalBundle := &Bundle{
			Name:         "critical",
			Type:         "css",
			Assets:       criticalCSS,
			Critical:     true,
			LoadPriority: 100,
			CreatedAt:    time.Now(),
		}
		
		// Calculate total size
		for _, asset := range criticalCSS {
			criticalBundle.Size += asset.Size
		}
		
		// Ensure critical bundle fits in 14KB limit
		if criticalBundle.Size > rb.config.CriticalBundleSize {
			criticalBundle = rb.splitCriticalBundle(criticalBundle)
		}
		
		bundles["critical.css"] = criticalBundle
	}

	// Create non-critical CSS bundles
	if len(nonCriticalCSS) > 0 {
		nonCriticalBundle := &Bundle{
			Name:         "extended",
			Type:         "css",
			Assets:       nonCriticalCSS,
			Critical:     false,
			LoadPriority: 50,
			CreatedAt:    time.Now(),
		}
		
		for _, asset := range nonCriticalCSS {
			nonCriticalBundle.Size += asset.Size
		}
		
		bundles["extended.css"] = nonCriticalBundle
	}

	return bundles
}

// createJSBundles creates JavaScript bundles with code splitting
func (rb *ResourceBundler) createJSBundles() map[string]*Bundle {
	bundles := make(map[string]*Bundle)

	var criticalJS, nonCriticalJS []AssetInfo
	for _, asset := range rb.assetMap {
		if asset.Type == "js" {
			if asset.Critical {
				criticalJS = append(criticalJS, asset)
			} else {
				nonCriticalJS = append(nonCriticalJS, asset)
			}
		}
	}

	// Create critical JS bundle (minimal, inline-friendly)
	if len(criticalJS) > 0 {
		criticalBundle := &Bundle{
			Name:         "critical",
			Type:         "js",
			Assets:       criticalJS,
			Critical:     true,
			LoadPriority: 100,
			CreatedAt:    time.Now(),
		}
		
		for _, asset := range criticalJS {
			criticalBundle.Size += asset.Size
		}
		
		bundles["critical.js"] = criticalBundle
	}

	// Create application JS bundle
	if len(nonCriticalJS) > 0 {
		appBundle := &Bundle{
			Name:         "app",
			Type:         "js",
			Assets:       nonCriticalJS,
			Critical:     false,
			LoadPriority: 75,
			CreatedAt:    time.Now(),
		}
		
		for _, asset := range nonCriticalJS {
			appBundle.Size += asset.Size
		}
		
		// Split large bundles
		if appBundle.Size > rb.config.ChunkSize {
			chunks := rb.splitJSBundle(appBundle)
			for name, chunk := range chunks {
				bundles[name] = chunk
			}
		} else {
			bundles["app.js"] = appBundle
		}
	}

	return bundles
}

// createImageBundles optimizes image assets
func (rb *ResourceBundler) createImageBundles() map[string]*Bundle {
	bundles := make(map[string]*Bundle)

	var criticalImages, nonCriticalImages []AssetInfo
	for _, asset := range rb.assetMap {
		if rb.isImageType(asset.Type) {
			if asset.Critical {
				criticalImages = append(criticalImages, asset)
			} else {
				nonCriticalImages = append(nonCriticalImages, asset)
			}
		}
	}

	// Create critical images bundle (for preloading)
	if len(criticalImages) > 0 {
		criticalBundle := &Bundle{
			Name:         "critical-images",
			Type:         "images",
			Assets:       criticalImages,
			Critical:     true,
			LoadPriority: 90,
			CreatedAt:    time.Now(),
		}
		
		for _, asset := range criticalImages {
			criticalBundle.Size += asset.Size
		}
		
		bundles["critical-images"] = criticalBundle
	}

	// Create non-critical images bundle (for lazy loading)
	if len(nonCriticalImages) > 0 {
		lazyBundle := &Bundle{
			Name:         "lazy-images",
			Type:         "images",
			Assets:       nonCriticalImages,
			Critical:     false,
			LoadPriority: 25,
			CreatedAt:    time.Now(),
		}
		
		for _, asset := range nonCriticalImages {
			lazyBundle.Size += asset.Size
		}
		
		bundles["lazy-images"] = lazyBundle
	}

	return bundles
}

// splitCriticalBundle splits oversized critical bundles
func (rb *ResourceBundler) splitCriticalBundle(bundle *Bundle) *Bundle {
	// Sort assets by priority
	sort.Slice(bundle.Assets, func(i, j int) bool {
		return rb.getAssetPriority(bundle.Assets[i]) > rb.getAssetPriority(bundle.Assets[j])
	})

	// Keep only assets that fit in critical size limit
	var criticalAssets []AssetInfo
	currentSize := 0
	
	for _, asset := range bundle.Assets {
		if currentSize+asset.Size <= rb.config.CriticalBundleSize {
			criticalAssets = append(criticalAssets, asset)
			currentSize += asset.Size
		} else {
			break
		}
	}

	bundle.Assets = criticalAssets
	bundle.Size = currentSize
	
	return bundle
}

// splitJSBundle splits large JavaScript bundles into chunks
func (rb *ResourceBundler) splitJSBundle(bundle *Bundle) map[string]*Bundle {
	chunks := make(map[string]*Bundle)
	
	chunkIndex := 0
	currentChunk := &Bundle{
		Type:         "js",
		Critical:     false,
		LoadPriority: bundle.LoadPriority - 5,
		CreatedAt:    time.Now(),
	}
	
	for _, asset := range bundle.Assets {
		if currentChunk.Size+asset.Size > rb.config.ChunkSize && len(currentChunk.Assets) > 0 {
			// Finalize current chunk
			currentChunk.Name = fmt.Sprintf("chunk-%d", chunkIndex)
			chunks[fmt.Sprintf("chunk-%d.js", chunkIndex)] = currentChunk
			chunkIndex++
			
			// Start new chunk
			currentChunk = &Bundle{
				Type:         "js",
				Critical:     false,
				LoadPriority: bundle.LoadPriority - 5,
				CreatedAt:    time.Now(),
			}
		}
		
		currentChunk.Assets = append(currentChunk.Assets, asset)
		currentChunk.Size += asset.Size
	}
	
	// Add final chunk if it has assets
	if len(currentChunk.Assets) > 0 {
		currentChunk.Name = fmt.Sprintf("chunk-%d", chunkIndex)
		chunks[fmt.Sprintf("chunk-%d.js", chunkIndex)] = currentChunk
	}
	
	return chunks
}

// generateBundleFiles creates physical bundle files
func (rb *ResourceBundler) generateBundleFiles() error {
	// Ensure output directory exists
	if err := os.MkdirAll(rb.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for filename, bundle := range rb.bundles {
		outputPath := filepath.Join(rb.config.OutputDir, filename)
		
		switch bundle.Type {
		case "css":
			if err := rb.generateCSSBundle(bundle, outputPath); err != nil {
				return fmt.Errorf("CSS bundle generation failed: %w", err)
			}
		case "js":
			if err := rb.generateJSBundle(bundle, outputPath); err != nil {
				return fmt.Errorf("JS bundle generation failed: %w", err)
			}
		}
		
		// Update bundle with compressed size
		if info, err := os.Stat(outputPath); err == nil {
			bundle.CompressedSize = int(info.Size())
		}
	}

	rb.lastOptimized = time.Now()
	return nil
}

// generateCSSBundle creates a CSS bundle file
func (rb *ResourceBundler) generateCSSBundle(bundle *Bundle, outputPath string) error {
	var content strings.Builder
	
	// Add bundle header
	content.WriteString(fmt.Sprintf("/* %s CSS Bundle - Generated %s */\n", 
		bundle.Name, time.Now().Format(time.RFC3339)))
	
	for _, asset := range bundle.Assets {
		assetPath := filepath.Join(rb.config.StaticDir, asset.Path)
		assetContent, err := os.ReadFile(assetPath)
		if err != nil {
			return fmt.Errorf("failed to read asset %s: %w", asset.Path, err)
		}
		
		content.WriteString(fmt.Sprintf("\n/* %s */\n", asset.Path))
		
		if rb.config.EnableMinification {
			minified := rb.minifyCSS(string(assetContent))
			content.WriteString(minified)
		} else {
			content.WriteString(string(assetContent))
		}
		
		content.WriteString("\n")
	}
	
	// Write bundle file
	return os.WriteFile(outputPath, []byte(content.String()), 0644)
}

// generateJSBundle creates a JavaScript bundle file
func (rb *ResourceBundler) generateJSBundle(bundle *Bundle, outputPath string) error {
	var content strings.Builder
	
	// Add bundle header
	content.WriteString(fmt.Sprintf("/* %s JS Bundle - Generated %s */\n", 
		bundle.Name, time.Now().Format(time.RFC3339)))
	
	for _, asset := range bundle.Assets {
		assetPath := filepath.Join(rb.config.StaticDir, asset.Path)
		assetContent, err := os.ReadFile(assetPath)
		if err != nil {
			return fmt.Errorf("failed to read asset %s: %w", asset.Path, err)
		}
		
		content.WriteString(fmt.Sprintf("\n/* %s */\n", asset.Path))
		
		if rb.config.EnableMinification {
			minified := rb.minifyJS(string(assetContent))
			content.WriteString(minified)
		} else {
			content.WriteString(string(assetContent))
		}
		
		content.WriteString(";\n") // Ensure statement termination
	}
	
	// Write bundle file
	return os.WriteFile(outputPath, []byte(content.String()), 0644)
}

// Helper methods

func (rb *ResourceBundler) isSupportedFormat(ext string) bool {
	for _, format := range rb.config.SupportedFormats {
		if ext == format {
			return true
		}
	}
	return false
}

func (rb *ResourceBundler) getAssetType(ext string) string {
	switch ext {
	case ".css":
		return "css"
	case ".js":
		return "js"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		return "image"
	default:
		return "other"
	}
}

func (rb *ResourceBundler) isImageType(assetType string) bool {
	return assetType == "image"
}

func (rb *ResourceBundler) isCriticalAsset(path string) bool {
	// Define critical asset patterns
	criticalPatterns := []string{
		"critical.css", "main.css", "base.css",
		"critical.js", "app.js", "htmx",
		"logo", "hero", "favicon",
	}
	
	pathLower := strings.ToLower(path)
	for _, pattern := range criticalPatterns {
		if strings.Contains(pathLower, pattern) {
			return true
		}
	}
	
	return false
}

func (rb *ResourceBundler) getAssetPriority(asset AssetInfo) int {
	priority := 50 // Default priority
	
	pathLower := strings.ToLower(asset.Path)
	
	// High priority patterns
	if strings.Contains(pathLower, "critical") {
		priority += 50
	}
	if strings.Contains(pathLower, "main") || strings.Contains(pathLower, "base") {
		priority += 30
	}
	if strings.Contains(pathLower, "htmx") {
		priority += 25
	}
	
	// Penalty for large files
	if asset.Size > 10*1024 { // 10KB
		priority -= 20
	}
	
	return priority
}

func (rb *ResourceBundler) calculateFileHash(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	
	hash := md5.Sum(content)
	return fmt.Sprintf("%x", hash), nil
}

func (rb *ResourceBundler) analyzeDependencies() error {
	// Simplified dependency analysis
	// In practice, you'd parse import/require statements
	return nil
}

// Basic CSS minification
func (rb *ResourceBundler) minifyCSS(css string) string {
	// Remove comments
	css = strings.ReplaceAll(css, "/*", "").ReplaceAll(css, "*/", "")
	
	// Remove extra whitespace
	css = strings.ReplaceAll(css, "\n", "")
	css = strings.ReplaceAll(css, "\t", "")
	css = strings.ReplaceAll(css, "  ", " ")
	
	// Remove whitespace around braces and semicolons
	css = strings.ReplaceAll(css, " {", "{")
	css = strings.ReplaceAll(css, "{ ", "{")
	css = strings.ReplaceAll(css, " }", "}")
	css = strings.ReplaceAll(css, "; ", ";")
	css = strings.ReplaceAll(css, " ;", ";")
	
	return strings.TrimSpace(css)
}

// Basic JavaScript minification
func (rb *ResourceBundler) minifyJS(js string) string {
	// Very basic minification - remove comments and extra whitespace
	lines := strings.Split(js, "\n")
	var minified []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "//") {
			minified = append(minified, line)
		}
	}
	
	return strings.Join(minified, "")
}

// GetBundles returns all created bundles
func (rb *ResourceBundler) GetBundles() map[string]*Bundle {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()
	return rb.bundles
}

// GetCriticalBundles returns only critical bundles
func (rb *ResourceBundler) GetCriticalBundles() map[string]*Bundle {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()
	
	critical := make(map[string]*Bundle)
	for name, bundle := range rb.bundles {
		if bundle.Critical {
			critical[name] = bundle
		}
	}
	return critical
}

// GeneratePreloadHints generates preload hints for critical resources
func (rb *ResourceBundler) GeneratePreloadHints() []string {
	var hints []string
	
	for filename, bundle := range rb.GetCriticalBundles() {
		if bundle.Critical {
			switch bundle.Type {
			case "css":
				hints = append(hints, fmt.Sprintf(
					`<link rel="preload" href="/static/dist/%s" as="style">`, filename))
			case "js":
				hints = append(hints, fmt.Sprintf(
					`<link rel="preload" href="/static/dist/%s" as="script">`, filename))
			}
		}
	}
	
	return hints
}

// GetOptimizationReport generates a detailed bundling report
func (rb *ResourceBundler) GetOptimizationReport() string {
	rb.mutex.RLock()
	defer rb.mutex.RUnlock()
	
	var report strings.Builder
	report.WriteString("=== Resource Bundling Report ===\n")
	report.WriteString(fmt.Sprintf("Last Optimized: %s\n", rb.lastOptimized.Format(time.RFC3339)))
	report.WriteString(fmt.Sprintf("Total Assets: %d\n", len(rb.assetMap)))
	report.WriteString(fmt.Sprintf("Total Bundles: %d\n", len(rb.bundles)))
	
	// Bundle breakdown
	criticalCount := 0
	totalOriginalSize := 0
	totalCompressedSize := 0
	
	for _, bundle := range rb.bundles {
		if bundle.Critical {
			criticalCount++
		}
		totalOriginalSize += bundle.Size
		totalCompressedSize += bundle.CompressedSize
	}
	
	report.WriteString(fmt.Sprintf("Critical Bundles: %d\n", criticalCount))
	report.WriteString(fmt.Sprintf("Original Size: %d bytes\n", totalOriginalSize))
	report.WriteString(fmt.Sprintf("Compressed Size: %d bytes\n", totalCompressedSize))
	
	if totalOriginalSize > 0 {
		compressionRatio := float64(totalCompressedSize) / float64(totalOriginalSize) * 100
		savings := float64(totalOriginalSize-totalCompressedSize) / float64(totalOriginalSize) * 100
		report.WriteString(fmt.Sprintf("Compression Ratio: %.1f%%\n", compressionRatio))
		report.WriteString(fmt.Sprintf("Size Savings: %.1f%%\n", savings))
	}
	
	// Critical bundle compliance
	report.WriteString("\n=== Critical Bundle Analysis ===\n")
	for filename, bundle := range rb.bundles {
		if bundle.Critical {
			compliant := bundle.CompressedSize <= rb.config.CriticalBundleSize
			status := "COMPLIANT"
			if !compliant {
				status = "VIOLATION"
			}
			report.WriteString(fmt.Sprintf("%s: %d bytes [%s]\n", 
				filename, bundle.CompressedSize, status))
		}
	}
	
	return report.String()
}