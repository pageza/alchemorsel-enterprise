// Package webserver provides session management for the web frontend
package webserver

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	sessions map[string]*Session
	mu       sync.RWMutex
	config   *config.Config
	logger   *zap.Logger
}

// NewSessionStore creates a new session store
func NewSessionStore(cfg *config.Config, logger *zap.Logger) *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*Session),
		config:   cfg,
		logger:   logger,
	}

	// Start cleanup goroutine
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
	
	session := &Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hour sessions
		Data:      make(map[string]interface{}),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return session
}

// Save saves the session and sets the cookie
func (session *Session) Save(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "alchemorsel-session",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
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
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, session := range s.sessions {
			if now.After(session.ExpiresAt) {
				delete(s.sessions, id)
				s.logger.Debug("Cleaned up expired session", zap.String("session_id", id))
			}
		}
		s.mu.Unlock()
	}
}

// generateSessionID generates a random session ID
func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
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