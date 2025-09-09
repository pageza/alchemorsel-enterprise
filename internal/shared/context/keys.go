// Package context provides typed context keys for the application
package context

// ContextKey is a custom type for context keys to avoid collisions
type ContextKey string

// Predefined context keys
const (
	// UserIDKey is used to store user ID in context
	UserIDKey ContextKey = "user_id"

	// RequestIDKey is used to store request ID in context
	RequestIDKey ContextKey = "request_id"

	// SessionIDKey is used to store session ID in context
	SessionIDKey ContextKey = "session_id"

	// TenantIDKey is used to store tenant ID in context
	TenantIDKey ContextKey = "tenant_id"
)
