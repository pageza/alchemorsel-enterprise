// Package hotreload provides development proxy server with live reload injection
package hotreload

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DevProxy provides a development proxy with hot reload injection
type DevProxy struct {
	server       *http.Server
	port         int
	apiTarget    *url.URL
	webTarget    *url.URL
	reloadServer *LiveReloadServer
	
	// Proxy instances
	apiProxy *httputil.ReverseProxy
	webProxy *httputil.ReverseProxy
	
	// Configuration
	injectScript  bool
	corsEnabled   bool
	cacheDisabled bool
	
	// Statistics
	stats     *ProxyStats
	statsLock sync.RWMutex
}

// ProxyStats tracks proxy usage statistics
type ProxyStats struct {
	RequestCount    int64
	APIRequests     int64
	WebRequests     int64
	StaticRequests  int64
	ErrorCount      int64
	StartTime       time.Time
	LastRequest     time.Time
	ResponseTimes   []time.Duration
}

// ProxyConfig configures the development proxy
type ProxyConfig struct {
	Port          int
	APITarget     string
	WebTarget     string
	InjectScript  bool
	CORSEnabled   bool
	CacheDisabled bool
	ReloadPort    int
}

// DefaultProxyConfig returns sensible defaults for development
func DefaultProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		Port:          8090,
		APITarget:     "http://localhost:3010",
		WebTarget:     "http://localhost:3011",
		InjectScript:  true,
		CORSEnabled:   true,
		CacheDisabled: true,
		ReloadPort:    35729,
	}
}

// NewDevProxy creates a new development proxy server
func NewDevProxy(config *ProxyConfig, reloadServer *LiveReloadServer) (*DevProxy, error) {
	if config == nil {
		config = DefaultProxyConfig()
	}

	apiTarget, err := url.Parse(config.APITarget)
	if err != nil {
		return nil, fmt.Errorf("invalid API target URL: %w", err)
	}

	webTarget, err := url.Parse(config.WebTarget)
	if err != nil {
		return nil, fmt.Errorf("invalid Web target URL: %w", err)
	}

	proxy := &DevProxy{
		port:          config.Port,
		apiTarget:     apiTarget,
		webTarget:     webTarget,
		reloadServer:  reloadServer,
		injectScript:  config.InjectScript,
		corsEnabled:   config.CORSEnabled,
		cacheDisabled: config.CacheDisabled,
		stats: &ProxyStats{
			StartTime:     time.Now(),
			ResponseTimes: make([]time.Duration, 0, 100),
		},
	}

	// Create reverse proxies
	proxy.apiProxy = httputil.NewSingleHostReverseProxy(apiTarget)
	proxy.webProxy = httputil.NewSingleHostReverseProxy(webTarget)

	// Configure API proxy
	proxy.apiProxy.ModifyResponse = proxy.modifyAPIResponse
	proxy.apiProxy.ErrorHandler = proxy.handleProxyError

	// Configure Web proxy with script injection
	proxy.webProxy.ModifyResponse = proxy.modifyWebResponse
	proxy.webProxy.ErrorHandler = proxy.handleProxyError

	// Setup HTTP server
	mux := http.NewServeMux()
	proxy.setupRoutes(mux, config.ReloadPort)

	proxy.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: proxy.corsMiddleware(proxy.statsMiddleware(mux)),
	}

	return proxy, nil
}

// setupRoutes configures proxy routing
func (p *DevProxy) setupRoutes(mux *http.ServeMux, reloadPort int) {
	// API routes
	mux.HandleFunc("/api/", p.handleAPIProxy)
	mux.HandleFunc("/health", p.handleAPIProxy)
	mux.HandleFunc("/metrics", p.handleAPIProxy)
	mux.HandleFunc("/swagger/", p.handleAPIProxy)

	// Live reload routes
	mux.HandleFunc("/livereload", p.handleLiveReloadProxy(reloadPort))
	mux.HandleFunc("/livereload.js", p.handleLiveReloadScript(reloadPort))

	// Development routes
	mux.HandleFunc("/dev-proxy/status", p.handleProxyStatus)
	mux.HandleFunc("/dev-proxy/stats", p.handleProxyStats)
	mux.HandleFunc("/dev-proxy/health", p.handleProxyHealth)

	// Static assets
	mux.HandleFunc("/static/", p.handleStaticProxy)
	mux.HandleFunc("/assets/", p.handleStaticProxy)
	mux.HandleFunc("/css/", p.handleStaticProxy)
	mux.HandleFunc("/js/", p.handleStaticProxy)

	// Default to web proxy
	mux.HandleFunc("/", p.handleWebProxy)
}

// Start begins the development proxy server
func (p *DevProxy) Start(ctx context.Context) error {
	log.Printf("Starting development proxy on port %d", p.port)
	log.Printf("API target: %s", p.apiTarget.String())
	log.Printf("Web target: %s", p.webTarget.String())
	log.Printf("Live reload injection: %v", p.injectScript)

	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Proxy server error: %v", err)
		}
	}()

	log.Printf("Development proxy started: http://localhost:%d", p.port)
	return nil
}

// Stop gracefully shuts down the proxy server
func (p *DevProxy) Stop(ctx context.Context) error {
	log.Printf("Stopping development proxy...")
	return p.server.Shutdown(ctx)
}

// handleAPIProxy routes requests to the API service
func (p *DevProxy) handleAPIProxy(w http.ResponseWriter, r *http.Request) {
	p.updateStats("api")
	p.apiProxy.ServeHTTP(w, r)
}

// handleWebProxy routes requests to the web service
func (p *DevProxy) handleWebProxy(w http.ResponseWriter, r *http.Request) {
	p.updateStats("web")
	p.webProxy.ServeHTTP(w, r)
}

// handleStaticProxy routes static asset requests
func (p *DevProxy) handleStaticProxy(w http.ResponseWriter, r *http.Request) {
	p.updateStats("static")
	
	// Disable caching for static assets in development
	if p.cacheDisabled {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
	}
	
	// Route to web service for static assets
	p.webProxy.ServeHTTP(w, r)
}

// handleLiveReloadProxy proxies live reload WebSocket connections
func (p *DevProxy) handleLiveReloadProxy(reloadPort int) http.HandlerFunc {
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", reloadPort))
	proxy := httputil.NewSingleHostReverseProxy(target)
	
	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}
}

// handleLiveReloadScript serves the live reload script
func (p *DevProxy) handleLiveReloadScript(reloadPort int) http.HandlerFunc {
	target, _ := url.Parse(fmt.Sprintf("http://localhost:%d", reloadPort))
	proxy := httputil.NewSingleHostReverseProxy(target)
	
	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}
}

// modifyWebResponse injects live reload script into HTML responses
func (p *DevProxy) modifyWebResponse(resp *http.Response) error {
	if !p.injectScript {
		return nil
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		return nil
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	resp.Body.Close()

	// Inject live reload script
	body := string(bodyBytes)
	liveReloadScript := fmt.Sprintf(`<script src="/livereload.js"></script>`)
	
	// Try to inject before </head>
	headRegex := regexp.MustCompile(`(?i)</head>`)
	if headRegex.MatchString(body) {
		body = headRegex.ReplaceAllString(body, liveReloadScript+"\n</head>")
	} else {
		// Fallback: inject before </body>
		bodyRegex := regexp.MustCompile(`(?i)</body>`)
		if bodyRegex.MatchString(body) {
			body = bodyRegex.ReplaceAllString(body, liveReloadScript+"\n</body>")
		} else {
			// Fallback: append to end
			body += "\n" + liveReloadScript
		}
	}

	// Update response
	resp.Body = io.NopCloser(strings.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))

	return nil
}

// modifyAPIResponse handles API response modifications
func (p *DevProxy) modifyAPIResponse(resp *http.Response) error {
	// Add CORS headers if enabled
	if p.corsEnabled {
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		resp.Header.Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}

	return nil
}

// handleProxyError handles proxy errors
func (p *DevProxy) handleProxyError(w http.ResponseWriter, r *http.Request, err error) {
	p.statsLock.Lock()
	p.stats.ErrorCount++
	p.statsLock.Unlock()

	log.Printf("Proxy error for %s: %v", r.URL.Path, err)

	// Return development-friendly error page
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusBadGateway)
	
	errorHTML := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>Proxy Error - Alchemorsel v3 Dev</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
		.container { max-width: 600px; margin: 0 auto; background: white; padding: 40px; border-radius: 8px; }
		.error { background: #ffe6e6; padding: 20px; border-radius: 4px; border-left: 4px solid #e74c3c; }
		h1 { color: #e74c3c; }
		.retry { margin-top: 20px; }
		.retry button { padding: 10px 20px; background: #3498db; color: white; border: none; border-radius: 4px; cursor: pointer; }
	</style>
</head>
<body>
	<div class="container">
		<h1>🚨 Proxy Error</h1>
		<div class="error">
			<strong>Error:</strong> %s<br>
			<strong>Target:</strong> %s<br>
			<strong>Path:</strong> %s
		</div>
		<p>The development proxy could not connect to the target service.</p>
		<div class="retry">
			<button onclick="window.location.reload()">🔄 Retry</button>
		</div>
	</div>
	<script src="/livereload.js"></script>
</body>
</html>
`, err.Error(), p.getTargetForPath(r.URL.Path), r.URL.Path)

	w.Write([]byte(errorHTML))
}

// getTargetForPath returns the target URL for a given path
func (p *DevProxy) getTargetForPath(path string) string {
	if strings.HasPrefix(path, "/api/") || path == "/health" || path == "/metrics" {
		return p.apiTarget.String()
	}
	return p.webTarget.String()
}

// corsMiddleware adds CORS headers if enabled
func (p *DevProxy) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p.corsEnabled {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		if p.cacheDisabled {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}

		next.ServeHTTP(w, r)
	})
}

// statsMiddleware tracks request statistics
func (p *DevProxy) statsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		next.ServeHTTP(w, r)
		
		duration := time.Since(start)
		p.updateResponseTime(duration)
	})
}

// updateStats updates proxy statistics
func (p *DevProxy) updateStats(requestType string) {
	p.statsLock.Lock()
	defer p.statsLock.Unlock()
	
	p.stats.RequestCount++
	p.stats.LastRequest = time.Now()
	
	switch requestType {
	case "api":
		p.stats.APIRequests++
	case "web":
		p.stats.WebRequests++
	case "static":
		p.stats.StaticRequests++
	}
}

// updateResponseTime records response time
func (p *DevProxy) updateResponseTime(duration time.Duration) {
	p.statsLock.Lock()
	defer p.statsLock.Unlock()
	
	if len(p.stats.ResponseTimes) >= 100 {
		// Keep only the last 100 response times
		p.stats.ResponseTimes = p.stats.ResponseTimes[1:]
	}
	
	p.stats.ResponseTimes = append(p.stats.ResponseTimes, duration)
}

// Status endpoints

// handleProxyStatus returns proxy status
func (p *DevProxy) handleProxyStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	status := map[string]interface{}{
		"service":           "alchemorsel-v3-dev-proxy",
		"status":            "running",
		"port":              p.port,
		"api_target":        p.apiTarget.String(),
		"web_target":        p.webTarget.String(),
		"features": map[string]bool{
			"live_reload_injection": p.injectScript,
			"cors_enabled":          p.corsEnabled,
			"cache_disabled":        p.cacheDisabled,
		},
		"uptime": time.Since(p.stats.StartTime).String(),
	}
	
	json.NewEncoder(w).Encode(status)
}

// handleProxyStats returns detailed statistics
func (p *DevProxy) handleProxyStats(w http.ResponseWriter, r *http.Request) {
	p.statsLock.RLock()
	defer p.statsLock.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	
	// Calculate average response time
	var avgResponseTime time.Duration
	if len(p.stats.ResponseTimes) > 0 {
		var total time.Duration
		for _, rt := range p.stats.ResponseTimes {
			total += rt
		}
		avgResponseTime = total / time.Duration(len(p.stats.ResponseTimes))
	}
	
	stats := map[string]interface{}{
		"requests": map[string]int64{
			"total":  p.stats.RequestCount,
			"api":    p.stats.APIRequests,
			"web":    p.stats.WebRequests,
			"static": p.stats.StaticRequests,
			"errors": p.stats.ErrorCount,
		},
		"timing": map[string]interface{}{
			"uptime":               time.Since(p.stats.StartTime).String(),
			"last_request":         p.stats.LastRequest.Format(time.RFC3339),
			"avg_response_time_ms": avgResponseTime.Milliseconds(),
		},
		"targets": map[string]string{
			"api": p.apiTarget.String(),
			"web": p.webTarget.String(),
		},
	}
	
	json.NewEncoder(w).Encode(stats)
}

// handleProxyHealth provides health check
func (p *DevProxy) handleProxyHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"proxy":     "running",
	}
	
	json.NewEncoder(w).Encode(health)
}