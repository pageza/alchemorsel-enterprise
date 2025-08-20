// Package performance provides advanced compression middleware for optimal first packet delivery
package performance

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
)

// CompressionMiddleware provides intelligent compression for HTTP responses
type CompressionMiddleware struct {
	config              CompressionConfig
	compressionCache    map[string]CachedCompression
	cacheMutex          sync.RWMutex
	stats               CompressionStats
	firstPacketOptimizer *FirstPacketOptimizer
}

// CompressionConfig configures compression behavior
type CompressionConfig struct {
	BrotliLevel         int      // Brotli compression level (1-11)
	GzipLevel          int      // Gzip compression level (1-9)
	MinSizeBytes       int      // Minimum response size to compress
	MaxSizeBytes       int      // Maximum response size to compress
	CacheCompressed    bool     // Cache compressed responses
	CacheTTL           time.Duration
	PreferBrotli       bool     // Prefer Brotli over Gzip when both supported
	CompressibleTypes  []string // MIME types to compress
	FirstPacketMode    bool     // Enable 14KB first packet optimization
}

// CachedCompression stores pre-compressed content
type CachedCompression struct {
	GzipContent   []byte
	BrotliContent []byte
	Encoding      string
	Size          int
	CreatedAt     time.Time
	ETag          string
}

// CompressionStats tracks compression performance
type CompressionStats struct {
	TotalRequests       int64
	CompressedRequests  int64
	BrotliRequests      int64
	GzipRequests        int64
	TotalBytesSaved     int64
	AverageCompression  float64
	CacheHits           int64
	CacheMisses         int64
	FirstPacketHits     int64
	mutex               sync.RWMutex
}

// ResponseWriterWrapper wraps http.ResponseWriter for compression
type ResponseWriterWrapper struct {
	http.ResponseWriter
	writer    io.Writer
	buffer    *bytes.Buffer
	encoding  string
	size      int
	written   bool
	statusCode int
}

// DefaultCompressionConfig returns sensible defaults
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		BrotliLevel:   6,  // Good balance of compression vs speed
		GzipLevel:     6,  // Good balance of compression vs speed
		MinSizeBytes:  1024, // Don't compress tiny responses
		MaxSizeBytes:  10 * 1024 * 1024, // 10MB max
		CacheCompressed: true,
		CacheTTL:      5 * time.Minute,
		PreferBrotli:  true,
		FirstPacketMode: true,
		CompressibleTypes: []string{
			"text/html",
			"text/css",
			"text/javascript",
			"application/javascript",
			"application/json",
			"text/xml",
			"application/xml",
			"text/plain",
			"image/svg+xml",
		},
	}
}

// NewCompressionMiddleware creates a new compression middleware
func NewCompressionMiddleware(config CompressionConfig, optimizer *FirstPacketOptimizer) *CompressionMiddleware {
	return &CompressionMiddleware{
		config:              config,
		compressionCache:    make(map[string]CachedCompression),
		firstPacketOptimizer: optimizer,
		stats:               CompressionStats{},
	}
}

// Handler returns the middleware handler function
func (cm *CompressionMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cm.updateStats(func(s *CompressionStats) { s.TotalRequests++ })

		// Check if compression is supported and beneficial
		if !cm.shouldCompress(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Determine best compression method
		encoding := cm.getBestEncoding(r)
		if encoding == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Create compression wrapper
		wrapper := &ResponseWriterWrapper{
			ResponseWriter: w,
			buffer:        new(bytes.Buffer),
			encoding:      encoding,
		}

		// Execute handler with compression wrapper
		next.ServeHTTP(wrapper, r)

		// Finalize compression
		if err := cm.finalizeCompression(wrapper); err != nil {
			// Fall back to uncompressed response
			w.Write(wrapper.buffer.Bytes())
		}
	})
}

// shouldCompress determines if the request should be compressed
func (cm *CompressionMiddleware) shouldCompress(r *http.Request) bool {
	// Skip if client doesn't support compression
	acceptEncoding := r.Header.Get("Accept-Encoding")
	if acceptEncoding == "" {
		return false
	}

	// Skip if already compressed
	if r.Header.Get("Content-Encoding") != "" {
		return false
	}

	// Skip for certain methods
	if r.Method == "HEAD" || r.Method == "OPTIONS" {
		return false
	}

	return true
}

// getBestEncoding determines the best compression encoding for the request
func (cm *CompressionMiddleware) getBestEncoding(r *http.Request) string {
	acceptEncoding := r.Header.Get("Accept-Encoding")
	if acceptEncoding == "" {
		return ""
	}

	// Parse Accept-Encoding header
	encodings := cm.parseAcceptEncoding(acceptEncoding)

	// Prefer Brotli if supported and configured
	if cm.config.PreferBrotli {
		if quality, exists := encodings["br"]; exists && quality > 0 {
			return "br"
		}
	}

	// Fall back to Gzip
	if quality, exists := encodings["gzip"]; exists && quality > 0 {
		return "gzip"
	}

	// Check for deflate as last resort
	if quality, exists := encodings["deflate"]; exists && quality > 0 {
		return "gzip" // Use gzip for deflate requests
	}

	return ""
}

// parseAcceptEncoding parses the Accept-Encoding header
func (cm *CompressionMiddleware) parseAcceptEncoding(header string) map[string]float64 {
	encodings := make(map[string]float64)
	
	parts := strings.Split(header, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		
		// Handle quality values
		if strings.Contains(part, ";q=") {
			subparts := strings.Split(part, ";q=")
			if len(subparts) == 2 {
				if quality, err := strconv.ParseFloat(subparts[1], 64); err == nil {
					encodings[strings.TrimSpace(subparts[0])] = quality
				}
			}
		} else {
			encodings[part] = 1.0
		}
	}
	
	return encodings
}

// finalizeCompression compresses and writes the final response
func (cm *CompressionMiddleware) finalizeCompression(wrapper *ResponseWriterWrapper) error {
	content := wrapper.buffer.Bytes()
	
	// Check size constraints
	if len(content) < cm.config.MinSizeBytes || len(content) > cm.config.MaxSizeBytes {
		wrapper.ResponseWriter.Write(content)
		return nil
	}

	// Check if content type is compressible
	contentType := wrapper.Header().Get("Content-Type")
	if !cm.isCompressibleType(contentType) {
		wrapper.ResponseWriter.Write(content)
		return nil
	}

	// Generate cache key
	cacheKey := cm.generateCacheKey(content, wrapper.encoding)

	// Check cache first
	if cm.config.CacheCompressed {
		if cached, exists := cm.getCachedCompression(cacheKey); exists {
			cm.updateStats(func(s *CompressionStats) { s.CacheHits++ })
			return cm.writeCompressedResponse(wrapper, cached.GzipContent, cached.BrotliContent, cached.Encoding)
		}
		cm.updateStats(func(s *CompressionStats) { s.CacheMisses++ })
	}

	// Compress content
	var gzipContent, brotliContent []byte
	var err error

	if wrapper.encoding == "gzip" || wrapper.encoding == "br" {
		gzipContent, err = cm.compressGzip(content)
		if err != nil {
			return err
		}
	}

	if wrapper.encoding == "br" {
		brotliContent, err = cm.compressBrotli(content)
		if err != nil {
			return err
		}
	}

	// Check 14KB first packet compliance
	if cm.config.FirstPacketMode && cm.firstPacketOptimizer != nil {
		var compressedSize int
		if wrapper.encoding == "br" && len(brotliContent) > 0 {
			compressedSize = len(brotliContent)
		} else if len(gzipContent) > 0 {
			compressedSize = len(gzipContent)
		}

		if compressedSize <= MaxFirstPacketSize {
			cm.updateStats(func(s *CompressionStats) { s.FirstPacketHits++ })
		}
	}

	// Cache compressed content
	if cm.config.CacheCompressed {
		cached := CachedCompression{
			GzipContent:   gzipContent,
			BrotliContent: brotliContent,
			Encoding:      wrapper.encoding,
			Size:          len(content),
			CreatedAt:     time.Now(),
			ETag:          fmt.Sprintf(`"%x"`, cacheKey),
		}
		cm.setCachedCompression(cacheKey, cached)
	}

	// Update compression stats
	originalSize := len(content)
	var compressedSize int
	if wrapper.encoding == "br" && len(brotliContent) > 0 {
		compressedSize = len(brotliContent)
		cm.updateStats(func(s *CompressionStats) { s.BrotliRequests++ })
	} else if len(gzipContent) > 0 {
		compressedSize = len(gzipContent)
		cm.updateStats(func(s *CompressionStats) { s.GzipRequests++ })
	}

	if compressedSize > 0 {
		bytesSaved := originalSize - compressedSize
		cm.updateStats(func(s *CompressionStats) {
			s.CompressedRequests++
			s.TotalBytesSaved += int64(bytesSaved)
			s.AverageCompression = float64(s.TotalBytesSaved) / float64(s.CompressedRequests)
		})
	}

	return cm.writeCompressedResponse(wrapper, gzipContent, brotliContent, wrapper.encoding)
}

// compressGzip compresses content using Gzip
func (cm *CompressionMiddleware) compressGzip(content []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, cm.config.GzipLevel)
	if err != nil {
		return nil, err
	}

	if _, err := writer.Write(content); err != nil {
		writer.Close()
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// compressBrotli compresses content using Brotli
func (cm *CompressionMiddleware) compressBrotli(content []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := brotli.NewWriterLevel(&buf, cm.config.BrotliLevel)

	if _, err := writer.Write(content); err != nil {
		writer.Close()
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// writeCompressedResponse writes the compressed response
func (cm *CompressionMiddleware) writeCompressedResponse(wrapper *ResponseWriterWrapper, gzipContent, brotliContent []byte, encoding string) error {
	var content []byte
	var encodingHeader string

	if encoding == "br" && len(brotliContent) > 0 {
		content = brotliContent
		encodingHeader = "br"
	} else if len(gzipContent) > 0 {
		content = gzipContent
		encodingHeader = "gzip"
	} else {
		return fmt.Errorf("no compressed content available")
	}

	// Set compression headers
	wrapper.Header().Set("Content-Encoding", encodingHeader)
	wrapper.Header().Set("Content-Length", strconv.Itoa(len(content)))
	wrapper.Header().Set("Vary", "Accept-Encoding")

	// Write compressed content
	_, err := wrapper.ResponseWriter.Write(content)
	return err
}

// isCompressibleType checks if the content type should be compressed
func (cm *CompressionMiddleware) isCompressibleType(contentType string) bool {
	if contentType == "" {
		return false
	}

	// Extract main type without parameters
	mainType := strings.Split(contentType, ";")[0]
	mainType = strings.TrimSpace(strings.ToLower(mainType))

	for _, compressibleType := range cm.config.CompressibleTypes {
		if mainType == compressibleType || strings.HasPrefix(mainType, compressibleType) {
			return true
		}
	}

	return false
}

// Cache management methods

func (cm *CompressionMiddleware) generateCacheKey(content []byte, encoding string) string {
	return fmt.Sprintf("%x-%s", content[:min(len(content), 32)], encoding)
}

func (cm *CompressionMiddleware) getCachedCompression(key string) (CachedCompression, bool) {
	cm.cacheMutex.RLock()
	defer cm.cacheMutex.RUnlock()

	cached, exists := cm.compressionCache[key]
	if !exists {
		return CachedCompression{}, false
	}

	// Check TTL
	if time.Since(cached.CreatedAt) > cm.config.CacheTTL {
		return CachedCompression{}, false
	}

	return cached, true
}

func (cm *CompressionMiddleware) setCachedCompression(key string, cached CachedCompression) {
	cm.cacheMutex.Lock()
	defer cm.cacheMutex.Unlock()

	cm.compressionCache[key] = cached

	// Simple cache cleanup (remove oldest entries if cache gets too large)
	if len(cm.compressionCache) > 1000 {
		cm.cleanupCache()
	}
}

func (cm *CompressionMiddleware) cleanupCache() {
	// Remove entries older than TTL
	cutoff := time.Now().Add(-cm.config.CacheTTL)
	for key, cached := range cm.compressionCache {
		if cached.CreatedAt.Before(cutoff) {
			delete(cm.compressionCache, key)
		}
	}
}

// Stats management

func (cm *CompressionMiddleware) updateStats(fn func(*CompressionStats)) {
	cm.stats.mutex.Lock()
	defer cm.stats.mutex.Unlock()
	fn(&cm.stats)
}

func (cm *CompressionMiddleware) GetStats() CompressionStats {
	cm.stats.mutex.RLock()
	defer cm.stats.mutex.RUnlock()
	return cm.stats
}

// ResponseWriterWrapper methods

func (rw *ResponseWriterWrapper) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
		if rw.statusCode == 0 {
			rw.statusCode = http.StatusOK
		}
	}
	rw.size += len(b)
	return rw.buffer.Write(b)
}

func (rw *ResponseWriterWrapper) WriteHeader(statusCode int) {
	if !rw.written {
		rw.statusCode = statusCode
		rw.ResponseWriter.WriteHeader(statusCode)
		rw.written = true
	}
}

func (rw *ResponseWriterWrapper) Header() http.Header {
	return rw.ResponseWriter.Header()
}

func (rw *ResponseWriterWrapper) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Utility functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetCompressionReport generates a detailed compression report
func (cm *CompressionMiddleware) GetCompressionReport() string {
	stats := cm.GetStats()
	
	var report strings.Builder
	report.WriteString("=== Compression Middleware Report ===\n")
	report.WriteString(fmt.Sprintf("Total Requests: %d\n", stats.TotalRequests))
	report.WriteString(fmt.Sprintf("Compressed Requests: %d\n", stats.CompressedRequests))
	
	if stats.TotalRequests > 0 {
		compressionRate := float64(stats.CompressedRequests) / float64(stats.TotalRequests) * 100
		report.WriteString(fmt.Sprintf("Compression Rate: %.1f%%\n", compressionRate))
	}
	
	report.WriteString(fmt.Sprintf("Brotli Requests: %d\n", stats.BrotliRequests))
	report.WriteString(fmt.Sprintf("Gzip Requests: %d\n", stats.GzipRequests))
	report.WriteString(fmt.Sprintf("Total Bytes Saved: %d\n", stats.TotalBytesSaved))
	report.WriteString(fmt.Sprintf("Average Compression: %.1f bytes/request\n", stats.AverageCompression))
	report.WriteString(fmt.Sprintf("Cache Hits: %d\n", stats.CacheHits))
	report.WriteString(fmt.Sprintf("Cache Misses: %d\n", stats.CacheMisses))
	report.WriteString(fmt.Sprintf("14KB First Packet Hits: %d\n", stats.FirstPacketHits))
	
	if stats.CacheHits+stats.CacheMisses > 0 {
		cacheHitRate := float64(stats.CacheHits) / float64(stats.CacheHits+stats.CacheMisses) * 100
		report.WriteString(fmt.Sprintf("Cache Hit Rate: %.1f%%\n", cacheHitRate))
	}
	
	return report.String()
}