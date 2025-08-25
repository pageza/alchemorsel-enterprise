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

	"github.com/gin-gonic/gin"
)

// CompressionConfig holds compression configuration
type CompressionConfig struct {
	Level            int      // Compression level (1-9)
	MinSize          int      // Minimum size to compress (bytes)
	ExcludedMimeTypes []string // MIME types to exclude from compression
	IncludedMimeTypes []string // MIME types to include for compression
}

// DefaultCompressionConfig returns default compression settings
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Level:   6, // Good balance between compression ratio and speed
		MinSize: 1024, // Don't compress files smaller than 1KB
		ExcludedMimeTypes: []string{
			"image/jpeg",
			"image/png",
			"image/gif",
			"image/webp",
			"video/mp4",
			"video/avi",
			"audio/mpeg",
			"application/zip",
			"application/gzip",
			"application/pdf",
		},
		IncludedMimeTypes: []string{
			"text/html",
			"text/css",
			"text/javascript",
			"application/javascript",
			"application/json",
			"text/xml",
			"application/xml",
			"text/plain",
			"application/rss+xml",
			"image/svg+xml",
		},
	}
}

// GzipMiddleware creates a Gin middleware for gzip compression
func GzipMiddleware(config CompressionConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if client accepts gzip
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// Skip compression for small responses or excluded types
		writer := &gzipResponseWriter{
			ResponseWriter: c.Writer,
			config:         config,
			c:              c,
		}

		c.Writer = writer
		c.Next()
		writer.Close()
	}
}

// gzipResponseWriter wraps gin.ResponseWriter to provide gzip compression
type gzipResponseWriter struct {
	gin.ResponseWriter
	config     CompressionConfig
	c          *gin.Context
	gzipWriter *gzip.Writer
	buffer     *bytes.Buffer
	written    bool
}

// Write implements the io.Writer interface
func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.writeHeader()
	}

	if w.gzipWriter != nil {
		return w.gzipWriter.Write(data)
	}

	return w.ResponseWriter.Write(data)
}

// WriteHeader implements gin.ResponseWriter interface
func (w *gzipResponseWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
}

// writeHeader determines whether to use compression based on response headers
func (w *gzipResponseWriter) writeHeader() {
	w.written = true

	contentType := w.ResponseWriter.Header().Get("Content-Type")
	contentLength := w.getContentLength()

	// Don't compress if content is too small
	if contentLength > 0 && contentLength < w.config.MinSize {
		return
	}

	// Check if content type should be compressed
	if !w.shouldCompress(contentType) {
		return
	}

	// Initialize gzip writer
	w.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	w.ResponseWriter.Header().Set("Vary", "Accept-Encoding")
	w.ResponseWriter.Header().Del("Content-Length") // Will be set by gzip writer

	w.gzipWriter, _ = gzip.NewWriterLevel(w.ResponseWriter, w.config.Level)
}

// getContentLength extracts content length from headers
func (w *gzipResponseWriter) getContentLength() int {
	contentLengthStr := w.ResponseWriter.Header().Get("Content-Length")
	if contentLengthStr == "" {
		return 0
	}

	contentLength, err := strconv.Atoi(contentLengthStr)
	if err != nil {
		return 0
	}

	return contentLength
}

// shouldCompress determines if content should be compressed based on MIME type
func (w *gzipResponseWriter) shouldCompress(contentType string) bool {
	// Extract main content type (ignore charset, etc.)
	mainType := strings.Split(contentType, ";")[0]
	mainType = strings.TrimSpace(mainType)

	// Check excluded types first
	for _, excluded := range w.config.ExcludedMimeTypes {
		if strings.EqualFold(mainType, excluded) {
			return false
		}
	}

	// If included types are specified, only compress those
	if len(w.config.IncludedMimeTypes) > 0 {
		for _, included := range w.config.IncludedMimeTypes {
			if strings.EqualFold(mainType, included) {
				return true
			}
		}
		return false
	}

	// Default: compress text-based content
	return strings.HasPrefix(mainType, "text/") ||
		strings.HasPrefix(mainType, "application/json") ||
		strings.HasPrefix(mainType, "application/javascript") ||
		strings.HasPrefix(mainType, "application/xml")
}

// Close closes the gzip writer
func (w *gzipResponseWriter) Close() {
	if w.gzipWriter != nil {
		w.gzipWriter.Close()
	}
}

// BrotliMiddleware creates a Gin middleware for Brotli compression
func BrotliMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if client accepts brotli
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "br") {
			c.Next()
			return
		}

		// Brotli implementation would go here
		// For now, fall back to gzip
		c.Next()
	}
}

// StaticFileCompressor handles compression of static files
type StaticFileCompressor struct {
	config CompressionConfig
}

// NewStaticFileCompressor creates a new static file compressor
func NewStaticFileCompressor(config CompressionConfig) *StaticFileCompressor {
	return &StaticFileCompressor{
		config: config,
	}
}

// CompressFile compresses a file and returns the compressed data
func (c *StaticFileCompressor) CompressFile(data []byte, mimeType string) ([]byte, bool, error) {
	// Check if file should be compressed
	if len(data) < c.config.MinSize || !c.shouldCompress(mimeType) {
		return data, false, nil
	}

	// Compress the data
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, c.config.Level)
	if err != nil {
		return nil, false, err
	}

	_, err = writer.Write(data)
	if err != nil {
		return nil, false, err
	}

	err = writer.Close()
	if err != nil {
		return nil, false, err
	}

	compressed := buf.Bytes()

	// Only return compressed version if it's actually smaller
	if len(compressed) >= len(data) {
		return data, false, nil
	}

	return compressed, true, nil
}

// shouldCompress determines if content should be compressed based on MIME type
func (c *StaticFileCompressor) shouldCompress(contentType string) bool {
	// Extract main content type
	mainType := strings.Split(contentType, ";")[0]
	mainType = strings.TrimSpace(mainType)

	// Check excluded types
	for _, excluded := range c.config.ExcludedMimeTypes {
		if strings.EqualFold(mainType, excluded) {
			return false
		}
	}

	// Check included types
	if len(c.config.IncludedMimeTypes) > 0 {
		for _, included := range c.config.IncludedMimeTypes {
			if strings.EqualFold(mainType, included) {
				return true
			}
		}
		return false
	}

	// Default compression logic
	return strings.HasPrefix(mainType, "text/") ||
		strings.HasPrefix(mainType, "application/json") ||
		strings.HasPrefix(mainType, "application/javascript") ||
		strings.HasPrefix(mainType, "application/xml")
}

// DecompressionHandler handles incoming compressed requests
func DecompressionHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		encoding := c.GetHeader("Content-Encoding")

		switch encoding {
		case "gzip":
			reader, err := gzip.NewReader(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid gzip data"})
				return
			}
			defer reader.Close()
			c.Request.Body = io.NopCloser(reader)

		case "deflate":
			// Handle deflate if needed
			// reader := flate.NewReader(c.Request.Body)
			// c.Request.Body = io.NopCloser(reader)

		default:
			// No compression or unsupported compression
		}

		c.Next()
	}
}

// CompressionStatsMiddleware tracks compression statistics
func CompressionStatsMiddleware() gin.HandlerFunc {
	stats := &CompressionStats{}

	return func(c *gin.Context) {
		// Wrap the response writer to track statistics
		writer := &statsResponseWriter{
			ResponseWriter: c.Writer,
			stats:         stats,
		}

		c.Writer = writer
		c.Next()
	}
}

// CompressionStats tracks compression performance metrics
type CompressionStats struct {
	TotalRequests     int64
	CompressedBytes   int64
	UncompressedBytes int64
	CompressionRatio  float64
}

// statsResponseWriter wraps ResponseWriter to collect compression statistics
type statsResponseWriter struct {
	gin.ResponseWriter
	stats           *CompressionStats
	originalSize    int
	compressedSize  int
}

// Write tracks bytes written
func (w *statsResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.compressedSize += n
	return n, err
}

// GetCompressionStats returns current compression statistics
func (s *CompressionStats) GetStats() map[string]interface{} {
	ratio := float64(0)
	if s.UncompressedBytes > 0 {
		ratio = float64(s.CompressedBytes) / float64(s.UncompressedBytes)
	}

	return map[string]interface{}{
		"total_requests":     s.TotalRequests,
		"compressed_bytes":   s.CompressedBytes,
		"uncompressed_bytes": s.UncompressedBytes,
		"compression_ratio":  ratio,
		"bytes_saved":        s.UncompressedBytes - s.CompressedBytes,
	}
}

// PrecompressionMiddleware serves precompressed files if available
func PrecompressionMiddleware(staticDir string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if client accepts gzip
		if strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			gzipPath := staticDir + c.Request.URL.Path + ".gz"
			
			// Check if precompressed file exists
			if fileExists(gzipPath) {
				c.Header("Content-Encoding", "gzip")
				c.Header("Vary", "Accept-Encoding")
				c.File(gzipPath)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// fileExists checks if a file exists
func fileExists(filename string) bool {
	// Implementation would check file existence
	return false // Placeholder
}

// ContentTypeDetector detects content type for better compression decisions
type ContentTypeDetector struct{}

// DetectContentType detects the MIME type of data
func (d *ContentTypeDetector) DetectContentType(data []byte, filename string) string {
	// Use http.DetectContentType for basic detection
	contentType := http.DetectContentType(data)

	// Enhance detection based on file extension
	if strings.HasSuffix(filename, ".js") {
		return "application/javascript"
	}
	if strings.HasSuffix(filename, ".css") {
		return "text/css"
	}
	if strings.HasSuffix(filename, ".json") {
		return "application/json"
	}
	if strings.HasSuffix(filename, ".xml") {
		return "application/xml"
	}
	if strings.HasSuffix(filename, ".svg") {
		return "image/svg+xml"
	}

	return contentType
}