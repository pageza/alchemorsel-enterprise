package performance

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

// HTTP2Config holds HTTP/2 server configuration
type HTTP2Config struct {
	MaxConcurrentStreams         uint32        // Maximum number of concurrent streams
	MaxReadFrameSize            uint32        // Maximum size of frames
	PermitProhibitedCipherSuites bool          // Allow weaker cipher suites
	IdleTimeout                 time.Duration // Connection idle timeout
	MaxUploadBufferPerConnection int32         // Upload buffer per connection
	MaxUploadBufferPerStream     int32         // Upload buffer per stream
	EnablePush                   bool          // Enable HTTP/2 server push
	PushPromises                 []PushPromise // Configured push promises
}

// PushPromise defines resources to push with HTTP/2 server push
type PushPromise struct {
	Path        string            // Request path that triggers push
	Resources   []PushResource    // Resources to push
	Conditions  []PushCondition   // Conditions for pushing
}

type PushResource struct {
	URL     string            // Resource URL to push
	Headers map[string]string // Headers for the pushed resource
	As      string            // Resource type (script, style, image, etc.)
}

type PushCondition struct {
	Type  string // "user-agent", "accept", "cookie"
	Value string // Condition value
}

// HTTP2Server wraps the standard HTTP server with HTTP/2 optimizations
type HTTP2Server struct {
	config     HTTP2Config
	server     *http.Server
	logger     *zap.Logger
	pushCache  *PushCache
	metrics    *HTTP2Metrics
}

// HTTP2Metrics tracks HTTP/2 performance metrics
type HTTP2Metrics struct {
	ConnectionsActive    int64
	StreamsActive        int64
	FramesReceived       int64
	FramesSent           int64
	PushPromisesSent     int64
	PushPromisesAccepted int64
	DataFrameBytes       int64
	HeaderCompressionRatio float64
}

// NewHTTP2Server creates a new HTTP/2 optimized server
func NewHTTP2Server(config HTTP2Config, logger *zap.Logger) *HTTP2Server {
	return &HTTP2Server{
		config:    config,
		logger:    logger,
		pushCache: NewPushCache(),
		metrics:   &HTTP2Metrics{},
	}
}

// CreateTLSConfig creates optimized TLS configuration for HTTP/2
func (s *HTTP2Server) CreateTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h2", "http/1.1"},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
		PreferServerCipherSuites: true,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.CurveP384,
		},
	}

	return tlsConfig, nil
}

// ConfigureHTTP2Server configures the HTTP server for HTTP/2
func (s *HTTP2Server) ConfigureHTTP2Server(handler http.Handler) *http.Server {
	server := &http.Server{
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  s.config.IdleTimeout,
	}

	// Configure HTTP/2
	http2Server := &http2.Server{
		MaxConcurrentStreams:         s.config.MaxConcurrentStreams,
		MaxReadFrameSize:            s.config.MaxReadFrameSize,
		PermitProhibitedCipherSuites: s.config.PermitProhibitedCipherSuites,
		IdleTimeout:                 s.config.IdleTimeout,
		MaxUploadBufferPerConnection: s.config.MaxUploadBufferPerConnection,
		MaxUploadBufferPerStream:     s.config.MaxUploadBufferPerStream,
	}

	err := http2.ConfigureServer(server, http2Server)
	if err != nil {
		s.logger.Error("Failed to configure HTTP/2 server", zap.Error(err))
	}

	s.server = server
	return server
}

// ServerPushMiddleware implements HTTP/2 server push
func (s *HTTP2Server) ServerPushMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.config.EnablePush {
			c.Next()
			return
		}

		// Check if the connection supports HTTP/2 push
		if pusher, ok := c.Writer.(http.Pusher); ok {
			s.handleServerPush(c, pusher)
		}

		c.Next()
	}
}

// handleServerPush processes server push logic
func (s *HTTP2Server) handleServerPush(c *gin.Context, pusher http.Pusher) {
	path := c.Request.URL.Path

	// Find matching push promises
	for _, promise := range s.config.PushPromises {
		if s.matchesPath(path, promise.Path) && s.evaluateConditions(c, promise.Conditions) {
			s.executePushPromise(c, pusher, promise)
		}
	}
}

// executePushPromise executes a server push promise
func (s *HTTP2Server) executePushPromise(c *gin.Context, pusher http.Pusher, promise PushPromise) {
	for _, resource := range promise.Resources {
		// Check if resource was already pushed in this connection
		if s.pushCache.WasPushed(c.ClientIP(), resource.URL) {
			continue
		}

		// Create push options
		options := &http.PushOptions{
			Method: "GET",
			Header: make(http.Header),
		}

		// Set headers for the pushed resource
		for key, value := range resource.Headers {
			options.Header.Set(key, value)
		}

		// Set resource type hint
		if resource.As != "" {
			options.Header.Set("X-Resource-Type", resource.As)
		}

		// Execute the push
		err := pusher.Push(resource.URL, options)
		if err != nil {
			s.logger.Debug("Server push failed", 
				zap.String("resource", resource.URL), 
				zap.Error(err))
			continue
		}

		// Track the push
		s.pushCache.MarkPushed(c.ClientIP(), resource.URL)
		s.metrics.PushPromisesSent++

		s.logger.Debug("Server push executed", 
			zap.String("resource", resource.URL),
			zap.String("client", c.ClientIP()))
	}
}

// matchesPath checks if a request path matches a push promise pattern
func (s *HTTP2Server) matchesPath(requestPath, pattern string) bool {
	// Simple pattern matching - could be enhanced with regex
	if pattern == "*" {
		return true
	}
	return requestPath == pattern
}

// evaluateConditions checks if push conditions are met
func (s *HTTP2Server) evaluateConditions(c *gin.Context, conditions []PushCondition) bool {
	for _, condition := range conditions {
		switch condition.Type {
		case "user-agent":
			if c.GetHeader("User-Agent") != condition.Value {
				return false
			}
		case "accept":
			if c.GetHeader("Accept") != condition.Value {
				return false
			}
		case "cookie":
			if c.GetHeader("Cookie") != condition.Value {
				return false
			}
		}
	}
	return true
}

// PushCache tracks pushed resources to avoid duplicate pushes
type PushCache struct {
	cache map[string]map[string]time.Time
	ttl   time.Duration
}

// NewPushCache creates a new push cache
func NewPushCache() *PushCache {
	cache := &PushCache{
		cache: make(map[string]map[string]time.Time),
		ttl:   1 * time.Hour,
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

// WasPushed checks if a resource was recently pushed to a client
func (p *PushCache) WasPushed(clientIP, resource string) bool {
	clientCache, exists := p.cache[clientIP]
	if !exists {
		return false
	}

	pushTime, exists := clientCache[resource]
	if !exists {
		return false
	}

	// Check if push is still within TTL
	return time.Since(pushTime) < p.ttl
}

// MarkPushed marks a resource as pushed to a client
func (p *PushCache) MarkPushed(clientIP, resource string) {
	if p.cache[clientIP] == nil {
		p.cache[clientIP] = make(map[string]time.Time)
	}
	p.cache[clientIP][resource] = time.Now()
}

// cleanup removes expired push cache entries
func (p *PushCache) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		for clientIP, clientCache := range p.cache {
			for resource, pushTime := range clientCache {
				if now.Sub(pushTime) > p.ttl {
					delete(clientCache, resource)
				}
			}
			
			// Remove empty client caches
			if len(clientCache) == 0 {
				delete(p.cache, clientIP)
			}
		}
	}
}

// HeaderCompressionMiddleware optimizes header compression for HTTP/2
func HeaderCompressionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set headers that benefit from HPACK compression
		c.Header("Server", "Alchemorsel/3.0")
		c.Header("X-Content-Type-Options", "nosniff")
		
		c.Next()
	}
}

// StreamPriorityMiddleware handles HTTP/2 stream priorities
func (s *HTTP2Server) StreamPriorityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set priority hints based on content type
		contentType := c.GetHeader("Content-Type")
		
		switch {
		case contentType == "text/css":
			c.Header("X-Priority", "high")
		case contentType == "application/javascript":
			c.Header("X-Priority", "medium")
		case contentType == "image/*":
			c.Header("X-Priority", "low")
		default:
			c.Header("X-Priority", "medium")
		}
		
		c.Next()
	}
}

// ResourceHintsMiddleware adds resource hints for optimization
func ResourceHintsMiddleware(hints []ResourceHint) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only add hints for HTML responses
		if c.GetHeader("Content-Type") == "text/html" {
			var linkHeaders []string
			
			for _, hint := range hints {
				linkHeader := fmt.Sprintf("<%s>; rel=%s", hint.URL, hint.Type)
				if hint.As != "" {
					linkHeader += fmt.Sprintf("; as=%s", hint.As)
				}
				linkHeaders = append(linkHeaders, linkHeader)
			}
			
			if len(linkHeaders) > 0 {
				c.Header("Link", fmt.Sprintf("%v", linkHeaders))
			}
		}
		
		c.Next()
	}
}

// GetMetrics returns HTTP/2 performance metrics
func (s *HTTP2Server) GetMetrics() HTTP2Metrics {
	return *s.metrics
}

// ConnectionMonitor monitors HTTP/2 connections
type ConnectionMonitor struct {
	connections map[string]*ConnectionInfo
	logger      *zap.Logger
}

type ConnectionInfo struct {
	RemoteAddr    string
	ConnectedAt   time.Time
	StreamCount   int
	BytesReceived int64
	BytesSent     int64
	LastActivity  time.Time
}

// NewConnectionMonitor creates a new connection monitor
func NewConnectionMonitor(logger *zap.Logger) *ConnectionMonitor {
	return &ConnectionMonitor{
		connections: make(map[string]*ConnectionInfo),
		logger:      logger,
	}
}

// TrackConnection starts tracking a new connection
func (m *ConnectionMonitor) TrackConnection(remoteAddr string) {
	m.connections[remoteAddr] = &ConnectionInfo{
		RemoteAddr:   remoteAddr,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
	}
}

// UpdateActivity updates connection activity
func (m *ConnectionMonitor) UpdateActivity(remoteAddr string, bytesReceived, bytesSent int64) {
	if conn, exists := m.connections[remoteAddr]; exists {
		conn.BytesReceived += bytesReceived
		conn.BytesSent += bytesSent
		conn.LastActivity = time.Now()
	}
}

// GetConnectionStats returns connection statistics
func (m *ConnectionMonitor) GetConnectionStats() map[string]interface{} {
	activeConnections := 0
	totalStreams := 0
	totalBytesReceived := int64(0)
	totalBytesSent := int64(0)

	now := time.Now()
	for _, conn := range m.connections {
		// Consider connection active if there was activity in the last 5 minutes
		if now.Sub(conn.LastActivity) < 5*time.Minute {
			activeConnections++
		}
		totalStreams += conn.StreamCount
		totalBytesReceived += conn.BytesReceived
		totalBytesSent += conn.BytesSent
	}

	return map[string]interface{}{
		"active_connections":   activeConnections,
		"total_streams":        totalStreams,
		"total_bytes_received": totalBytesReceived,
		"total_bytes_sent":     totalBytesSent,
	}
}

// HTTP2OptimizationMiddleware applies various HTTP/2 optimizations
func HTTP2OptimizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set HTTP/2 specific headers
		c.Header("HTTP2-Push", "enabled")
		
		// Enable connection reuse
		c.Header("Connection", "keep-alive")
		
		// Set appropriate cache headers for static resources
		if isStaticResource(c.Request.URL.Path) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		
		c.Next()
	}
}

// isStaticResource checks if the path is for a static resource
func isStaticResource(path string) bool {
	staticExtensions := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".woff", ".woff2"}
	for _, ext := range staticExtensions {
		if len(path) > len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}