// Package middleware provides HTTP middleware for security and performance
package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Logger middleware logs HTTP requests
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Create a response writer wrapper to capture status code
			ww := &responseWriter{ResponseWriter: w}
			
			// Process request
			next.ServeHTTP(ww, r)
			
			// Log request
			logger.Info("HTTP request",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
				zap.Int("status", ww.status),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", r.Header.Get("X-Request-ID")),
			)
		})
	}
}

// Security middleware adds security headers optimized for HTMX
func Security() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content Security Policy optimized for HTMX
			csp := "default-src 'self'; " +
				"script-src 'self' 'unsafe-inline' https://unpkg.com; " +
				"style-src 'self' 'unsafe-inline'; " +
				"img-src 'self' data: https:; " +
				"font-src 'self' https:; " +
				"connect-src 'self'; " +
				"frame-ancestors 'none'; " +
				"base-uri 'self'"
			
			w.Header().Set("Content-Security-Policy", csp)
			
			// Security headers
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			
			// HSTS for HTTPS
			if r.TLS != nil {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}
			
			// Cache control for static resources
			if isStaticResource(r.URL.Path) {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// CORS middleware handles cross-origin requests
func CORS() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// Allow same-origin requests
			if origin == "" || isSameOrigin(r, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, HX-Request, HX-Target, HX-Current-URL")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
			
			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// Performance middleware adds performance optimization headers
func Performance() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Server push hints for HTTP/2
			if pusher, ok := w.(http.Pusher); ok {
				// Push critical resources
				pusher.Push("/static/css/critical.css", nil)
				pusher.Push("/static/js/htmx.min.js", nil)
			}
			
			// Resource hints
			w.Header().Set("Link", "</static/css/critical.css>; rel=preload; as=style")
			w.Header().Add("Link", "</static/js/htmx.min.js>; rel=preload; as=script")
			
			// Performance timing headers
			w.Header().Set("Server-Timing", "app;dur=0")
			
			next.ServeHTTP(w, r)
		})
	}
}

// HTMXOptimization middleware optimizes responses for HTMX requests
func HTMXOptimization() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this is an HTMX request
			if r.Header.Get("HX-Request") == "true" {
				// Add HTMX-specific headers
				w.Header().Set("Cache-Control", "no-cache")
				
				// Enable server-sent events for streaming responses
				if r.Header.Get("Accept") == "text/event-stream" {
					w.Header().Set("Content-Type", "text/event-stream")
					w.Header().Set("Cache-Control", "no-cache")
					w.Header().Set("Connection", "keep-alive")
				}
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = 200
	}
	return rw.ResponseWriter.Write(b)
}

// isStaticResource checks if the path is a static resource
func isStaticResource(path string) bool {
	staticPaths := []string{"/static/", "/favicon.ico", "/robots.txt", "/sitemap.xml"}
	for _, staticPath := range staticPaths {
		if len(path) >= len(staticPath) && path[:len(staticPath)] == staticPath {
			return true
		}
	}
	return false
}

// isSameOrigin checks if the origin matches the request host
func isSameOrigin(r *http.Request, origin string) bool {
	// Simple same-origin check
	return origin == "http://"+r.Host || origin == "https://"+r.Host
}