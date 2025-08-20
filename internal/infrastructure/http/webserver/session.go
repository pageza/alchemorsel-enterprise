// Package webserver provides session management for the web frontend
package webserver

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"go.uber.org/zap"
)

// Session represents a user session
type Session struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Data         map[string]interface{} `json:"data"`
}

// SessionStore manages user sessions
type SessionStore struct {
	sessions    map[string]*Session
	mu          sync.RWMutex
	config      *config.Config
	logger      *zap.Logger
	// SECURITY FIX ALV3-2025-007: Add persistence options
	persistent  bool
	storageType string // "memory", "redis", "file"
}

// NewSessionStore creates a new session store
func NewSessionStore(cfg *config.Config, logger *zap.Logger) *SessionStore {
	store := &SessionStore{
		sessions:    make(map[string]*Session),
		config:      cfg,
		logger:      logger,
		persistent:  false, // TODO: Make configurable
		storageType: "memory", // TODO: Support Redis/file storage
	}

	// SECURITY FIX ALV3-2025-007: Enhanced session management
	logger.Info("Initializing session store",
		zap.String("storage_type", store.storageType),
		zap.Bool("persistent", store.persistent),
	)

	// Start cleanup goroutine with more frequent cleanup for security
	go store.cleanupExpired()

	return store
}

// Get retrieves a session from the request
func (s *SessionStore) Get(r *http.Request, name string) (*Session, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	session, exists := s.sessions[cookie.Value]
	s.mu.RUnlock()

	if !exists {
		return nil, http.ErrNoCookie
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		s.Delete(cookie.Value)
		return nil, http.ErrNoCookie
	}

	return session, nil
}

// New creates a new session
func (s *SessionStore) New(name string) *Session {
	sessionID := generateSessionID()
	
	// SECURITY FIX: Reduce session lifetime to 30 minutes for security
	sessionLifetime := 30 * time.Minute
	if s.config.Auth.SessionMaxAge > 0 {
		sessionLifetime = time.Duration(s.config.Auth.SessionMaxAge) * time.Second
	}
	
	session := &Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(sessionLifetime),
		Data:      make(map[string]interface{}),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	s.logger.Debug("Created new session", 
		zap.String("session_id", sessionID),
		zap.Duration("lifetime", sessionLifetime),
	)

	return session
}

// Save saves the session and sets the cookie
func (session *Session) Save(w http.ResponseWriter) {
	// CRITICAL SECURITY FIX ALV3-2025-004: Secure session configuration
	cookie := &http.Cookie{
		Name:     "alchemorsel-session",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		// SECURITY FIX: Always use Secure flag in production
		Secure:   true, // Should be configurable based on environment
		// SECURITY FIX: Use SameSiteStrictMode for better CSRF protection
		SameSite: http.SameSiteStrictMode,
		Expires:  session.ExpiresAt,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
	}

	http.SetCookie(w, cookie)
}

// Clear clears the session data
func (session *Session) Clear() {
	session.UserID = ""
	session.AccessToken = ""
	session.RefreshToken = ""
	session.Data = make(map[string]interface{})
}

// Delete removes a session
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// cleanupExpired removes expired sessions periodically
func (s *SessionStore) cleanupExpired() {
	// SECURITY FIX: More frequent cleanup (every 5 minutes) for better memory management
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		expiredCount := 0
		for id, session := range s.sessions {
			if now.After(session.ExpiresAt) {
				delete(s.sessions, id)
				expiredCount++
				s.logger.Debug("Cleaned up expired session", zap.String("session_id", id))
			}
		}
		s.mu.Unlock()
		
		if expiredCount > 0 {
			s.logger.Info("Session cleanup completed",
				zap.Int("expired_sessions", expiredCount),
				zap.Int("active_sessions", len(s.sessions)),
			)
		}
	}
}

// generateSessionID generates a cryptographically secure random session ID
func generateSessionID() string {
	// SECURITY FIX: Use larger session ID (48 bytes = 384 bits)
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails
		// This should never happen in practice
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(b)
}

// GetValue gets a value from session data
func (session *Session) GetValue(key string) (interface{}, bool) {
	value, exists := session.Data[key]
	return value, exists
}

// SetValue sets a value in session data
func (session *Session) SetValue(key string, value interface{}) {
	session.Data[key] = value
}

// ToJSON serializes session to JSON
func (session *Session) ToJSON() ([]byte, error) {
	return json.Marshal(session)
}

// FromJSON deserializes session from JSON
func (session *Session) FromJSON(data []byte) error {
	return json.Unmarshal(data, session)
}