// Package webserver provides SCS session management for the web frontend
package webserver

import (
	"context"
	"encoding/gob"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/gomodule/redigo/redis"
	"go.uber.org/zap"
)

// ConversationMessage represents a message in the session-based chat system
type ConversationMessage struct {
	Role      string    `json:"role"`      // "user" or "assistant"
	Content   string    `json:"content"`   // Message content
	Timestamp time.Time `json:"timestamp"` // When the message was created
}

// FormattedTime returns a human-readable time string
func (m *ConversationMessage) FormattedTime() string {
	now := time.Now()
	diff := now.Sub(m.Timestamp)

	if diff < time.Minute {
		return "Just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		} else if days < 7 {
			return fmt.Sprintf("%d days ago", days)
		}
		return m.Timestamp.Format("Jan 2, 2006")
	}
}

// SessionManager wraps SCS session manager with Redis store
type SessionManager struct {
	*scs.SessionManager
	config *config.Config
	logger *zap.Logger
}

// NewSessionManager creates a new SCS session manager with Redis store (with fallback)
func NewSessionManager(cfg *config.Config, logger *zap.Logger) (*SessionManager, error) {
	// Register types with gob for session serialization
	gob.Register(conversation.ChatMessage{})
	gob.Register([]conversation.ChatMessage{})
	gob.Register(ConversationMessage{})
	gob.Register([]ConversationMessage{})
	gob.Register(map[string]interface{}{})
	gob.Register([]map[string]interface{}{})
	gob.Register(time.Time{})

	// Create SCS session manager first
	sessionManager := scs.New()

	// Try to establish Redis connection with timeout
	redisAvailable := false
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create Redis connection pool for SCS (using Redigo)
	// Note: This creates a separate connection pool from the main Redis client
	// because SCS redisstore uses the Redigo client, while our cache uses go-redis
	pool := &redis.Pool{
		MaxIdle:     10,
		MaxActive:   100,
		IdleTimeout: 5 * time.Minute,
		Dial: func() (redis.Conn, error) {
			return redis.DialContext(ctx, "tcp", cfg.Redis.Host+":"+strconv.Itoa(cfg.Redis.Port))
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}

	// Test the connection with timeout
	done := make(chan bool)
	go func() {
		conn := pool.Get()
		defer conn.Close()
		if _, err := conn.Do("PING"); err == nil {
			redisAvailable = true
		}
		done <- true
	}()

	// Wait for connection test or timeout
	select {
	case <-done:
		// Connection test completed
	case <-ctx.Done():
		// Timeout reached
		logger.Warn("Redis connection timeout, falling back to in-memory sessions",
			zap.String("redis_host", cfg.Redis.Host),
			zap.Int("redis_port", cfg.Redis.Port))
	}

	// Configure session store based on Redis availability
	if redisAvailable {
		sessionManager.Store = redisstore.NewWithPrefix(pool, "session:")
		logger.Info("Using Redis session store",
			zap.String("prefix", "session:"))
	} else {
		// Fallback to in-memory store
		sessionManager.Store = memstore.New()
		logger.Warn("Using in-memory session store (Redis unavailable)")

	}

	// Configure session settings
	sessionManager.Lifetime = 24 * time.Hour      // 24 hour absolute timeout
	sessionManager.IdleTimeout = 30 * time.Minute // 30 minute idle timeout
	sessionManager.Cookie.Name = "alchemorsel-session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Path = "/"
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode // HTMX compatible
	sessionManager.Cookie.Secure = false                  // Set to true in production with HTTPS

	// Configure environment-specific settings
	if cfg.App.Environment == "production" {
		sessionManager.Cookie.Secure = true
		sessionManager.Cookie.SameSite = http.SameSiteStrictMode
	}

	storeType := "redis"
	if !redisAvailable {
		storeType = "memory"
	}

	logger.Info("Initialized SCS session manager",
		zap.String("store", storeType),
		zap.Duration("lifetime", sessionManager.Lifetime),
		zap.Duration("idle_timeout", sessionManager.IdleTimeout),
		zap.String("cookie_name", sessionManager.Cookie.Name),
		zap.Bool("secure", sessionManager.Cookie.Secure),
	)

	return &SessionManager{
		SessionManager: sessionManager,
		config:         cfg,
		logger:         logger,
	}, nil
}
