// Package webserver provides SCS session management for the web frontend
package webserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/redisstore"
	"github.com/alchemorsel/v3/internal/infrastructure/config"
	"github.com/gomodule/redigo/redis"
	"go.uber.org/zap"
)

// SessionManager wraps SCS session manager with Redis store
type SessionManager struct {
	*scs.SessionManager
	config *config.Config
	logger *zap.Logger
}

// NewSessionManager creates a new SCS session manager with Redis store
func NewSessionManager(cfg *config.Config, logger *zap.Logger) (*SessionManager, error) {
	// Create Redis connection pool for SCS (using Redigo)
	// Note: This creates a separate connection pool from the main Redis client
	// because SCS redisstore uses the Redigo client, while our cache uses go-redis
	pool := &redis.Pool{
		MaxIdle:     10,
		MaxActive:   100,
		IdleTimeout: 5 * time.Minute,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", cfg.Redis.Host+":"+strconv.Itoa(cfg.Redis.Port))
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}

	// Test the connection
	conn := pool.Get()
	defer conn.Close()
	if _, err := conn.Do("PING"); err != nil {
		return nil, err
	}

	// Create SCS session manager
	sessionManager := scs.New()
	sessionManager.Store = redisstore.NewWithPrefix(pool, "session:")

	// Configure session settings
	sessionManager.Lifetime = 24 * time.Hour                    // 24 hour absolute timeout
	sessionManager.IdleTimeout = 30 * time.Minute               // 30 minute idle timeout
	sessionManager.Cookie.Name = "alchemorsel-session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Path = "/"
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode        // HTMX compatible
	sessionManager.Cookie.Secure = false                        // Set to true in production with HTTPS

	// Configure environment-specific settings
	if cfg.App.Environment == "production" {
		sessionManager.Cookie.Secure = true
		sessionManager.Cookie.SameSite = http.SameSiteStrictMode
	}

	logger.Info("Initialized SCS session manager",
		zap.String("store", "redis"),
		zap.String("prefix", "session:"),
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