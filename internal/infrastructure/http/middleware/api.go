// Package middleware provides Chi-compatible middleware for the pure API server
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/security"
	"go.uber.org/zap"
)

// Logger creates a Chi-compatible logging middleware
func Logger(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Wrap the response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			
			// Process request
			next.ServeHTTP(wrapped, r)
			
			// Log the request
			logger.Info("API Request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
				zap.Int("status_code", wrapped.statusCode),
				zap.Duration("duration", time.Since(start)),
				zap.String("user_agent", r.UserAgent()),
			)
		})
	}
}

// Security adds security headers for API responses
func Security() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add security headers for API
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			
			// Add Content Security Policy for API endpoints
			csp := strings.Join([]string{
				"default-src 'self'",
				"script-src 'self'",
				"style-src 'self' 'unsafe-inline'",
				"img-src 'self' data: https:",
				"font-src 'self' data:",
				"connect-src 'self'",
				"frame-ancestors 'none'",
				"base-uri 'none'",
				"object-src 'none'",
				"media-src 'self'",
				"form-action 'self'",
			}, "; ")
			w.Header().Set("Content-Security-Policy", csp)
			
			next.ServeHTTP(w, r)
		})
	}
}

// CORS adds CORS headers for API endpoints
func CORS() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "*") // Configure appropriately for production
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
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

// JSONOnly forces all responses to be JSON for pure API
func JSONOnly() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Force JSON content type for all API responses
			w.Header().Set("Content-Type", "application/json")
			
			// Only accept JSON requests for POST/PUT
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				contentType := r.Header.Get("Content-Type")
				if !strings.Contains(contentType, "application/json") {
					w.WriteHeader(http.StatusUnsupportedMediaType)
					fmt.Fprint(w, `{"error":"Content-Type must be application/json"}`)
					return
				}
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// AuthenticateAPI provides JWT authentication for API endpoints
func AuthenticateAPI(authService *security.AuthService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract JWT token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"Authorization header required"}`)
				return
			}
			
			// Check Bearer token format
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, `{"error":"Invalid authorization header format"}`)
				return
			}
			
			token := parts[1]
			
			// Validate JWT token
			claims, err := authService.ValidateToken(token, security.AccessToken)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, `{"error":"Invalid token: %s"}`, err.Error())
				return
			}
			
			// Add user info to request context
			ctx := r.Context()
			ctx = addUserToContext(ctx, claims.UserID, claims.Email)
			r = r.WithContext(ctx)
			
			next.ServeHTTP(w, r)
		})
	}
}

// Performance adds performance headers and optimizations
func Performance() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add performance headers
			w.Header().Set("X-API-Version", "v1")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			
			next.ServeHTTP(w, r)
		})
	}
}

// HTMXOptimization is a no-op for pure API (kept for compatibility)
func HTMXOptimization() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// No HTMX optimizations needed for pure API
			next.ServeHTTP(w, r)
		})
	}
}

// XSSProtection provides XSS protection for form data
func XSSProtection(xssService *security.XSSProtectionService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set XSS protection headers
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			
			// For POST/PUT requests, validate form data
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				if err := r.ParseForm(); err == nil {
					for key, values := range r.Form {
						for _, value := range values {
							if err := xssService.ValidateInput(value); err != nil {
								w.WriteHeader(http.StatusBadRequest)
								fmt.Fprintf(w, `{"error":"Invalid input detected in field %s","field":"%s"}`, key, key)
								return
							}
						}
					}
				}
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// CSRFProtection provides CSRF protection for Chi router
func CSRFProtection(authService *security.AuthService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CSRF check for GET, HEAD, OPTIONS methods
			if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}
			
			// Check for CSRF token in header or form
			csrfToken := r.Header.Get("X-CSRF-Token")
			if csrfToken == "" {
				// Parse form to get CSRF token from form data
				if err := r.ParseForm(); err == nil {
					csrfToken = r.FormValue("csrf_token")
				}
			}
			
			if csrfToken == "" {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error":"CSRF token required"}`)
				return
			}
			
			// Validate CSRF token
			claims, err := authService.ValidateToken(csrfToken, security.CSRFToken)
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintf(w, `{"error":"Invalid CSRF token: %s"}`, err.Error())
				return
			}
			
			// Validate session ID (if available)
			sessionID := r.Header.Get("X-Session-ID")
			if sessionID == "" {
				// Try to get session ID from cookie
				if cookie, err := r.Cookie("session"); err == nil {
					sessionID = cookie.Value
				}
			}
			
			if sessionID != "" && claims.SessionID != sessionID {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"error":"CSRF token session mismatch"}`)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Helper function to add user info to context
func addUserToContext(ctx context.Context, userID, email string) context.Context {
	ctx = context.WithValue(ctx, "user_id", userID)
	ctx = context.WithValue(ctx, "user_email", email)
	return ctx
}

// GetUserIDFromContext extracts user ID from request context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value("user_id").(string)
	return userID, ok
}

// GetUserEmailFromContext extracts user email from request context  
func GetUserEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value("user_email").(string)
	return email, ok
}