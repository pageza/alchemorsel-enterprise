// Package hotreload provides state preservation across hot reloads
package hotreload

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// StateManager handles application state preservation during hot reloads
type StateManager struct {
	redisClient   *redis.Client
	stateFile     string
	stateRegistry map[string]StateProvider
	mutex         sync.RWMutex
	
	// Configuration
	enableRedis    bool
	enableFile     bool
	saveInterval   time.Duration
	retentionTime  time.Duration
}

// StateProvider interface for components that can save/restore state
type StateProvider interface {
	SaveState() (interface{}, error)
	RestoreState(data interface{}) error
	GetStateKey() string
}

// ApplicationState represents the complete application state
type ApplicationState struct {
	Timestamp    time.Time              `json:"timestamp"`
	Version      string                 `json:"version"`
	Environment  string                 `json:"environment"`
	ComponentStates map[string]interface{} `json:"component_states"`
}

// SessionState represents user session state
type SessionState struct {
	UserID        string            `json:"user_id,omitempty"`
	SessionData   map[string]interface{} `json:"session_data"`
	Authenticated bool              `json:"authenticated"`
	LastActivity  time.Time         `json:"last_activity"`
	ExpiresAt     time.Time         `json:"expires_at"`
}

// CacheState represents cache state
type CacheState struct {
	Keys     []string          `json:"keys"`
	Values   map[string]string `json:"values"`
	TTLs     map[string]int64  `json:"ttls"`
	SavedAt  time.Time         `json:"saved_at"`
}

// DatabaseState represents database connection state
type DatabaseState struct {
	ActiveConnections int       `json:"active_connections"`
	PoolSize          int       `json:"pool_size"`
	LastMigration     string    `json:"last_migration"`
	SavedAt           time.Time `json:"saved_at"`
}

// StateManagerConfig configures the state manager
type StateManagerConfig struct {
	RedisURL        string
	StateFile       string
	EnableRedis     bool
	EnableFile      bool
	SaveInterval    time.Duration
	RetentionTime   time.Duration
}

// DefaultStateManagerConfig returns sensible defaults
func DefaultStateManagerConfig() *StateManagerConfig {
	return &StateManagerConfig{
		RedisURL:      "redis://localhost:6379",
		StateFile:     "tmp/app-state.json",
		EnableRedis:   true,
		EnableFile:    true,
		SaveInterval:  10 * time.Second,
		RetentionTime: 1 * time.Hour,
	}
}

// NewStateManager creates a new state manager
func NewStateManager(config *StateManagerConfig) (*StateManager, error) {
	if config == nil {
		config = DefaultStateManagerConfig()
	}

	sm := &StateManager{
		stateFile:     config.StateFile,
		stateRegistry: make(map[string]StateProvider),
		enableRedis:   config.EnableRedis,
		enableFile:    config.EnableFile,
		saveInterval:  config.SaveInterval,
		retentionTime: config.RetentionTime,
	}

	// Initialize Redis client if enabled
	if config.EnableRedis && config.RedisURL != "" {
		opt, err := redis.ParseURL(config.RedisURL)
		if err != nil {
			log.Printf("Failed to parse Redis URL: %v", err)
			sm.enableRedis = false
		} else {
			sm.redisClient = redis.NewClient(opt)
			
			// Test Redis connection
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := sm.redisClient.Ping(ctx).Err(); err != nil {
				log.Printf("Redis connection failed: %v", err)
				sm.enableRedis = false
			} else {
				log.Printf("Redis connection established for state persistence")
			}
		}
	}

	// Ensure state file directory exists
	if config.EnableFile {
		if err := os.MkdirAll(filepath.Dir(config.StateFile), 0755); err != nil {
			log.Printf("Failed to create state directory: %v", err)
			sm.enableFile = false
		}
	}

	return sm, nil
}

// RegisterStateProvider registers a component that can save/restore state
func (sm *StateManager) RegisterStateProvider(provider StateProvider) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	
	key := provider.GetStateKey()
	sm.stateRegistry[key] = provider
	
	log.Printf("Registered state provider: %s", key)
}

// SaveState saves the current application state
func (sm *StateManager) SaveState(ctx context.Context) error {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	state := ApplicationState{
		Timestamp:       time.Now(),
		Version:         "v3.0.0-dev",
		Environment:     "development",
		ComponentStates: make(map[string]interface{}),
	}

	// Collect state from all registered providers
	for key, provider := range sm.stateRegistry {
		providerState, err := provider.SaveState()
		if err != nil {
			log.Printf("Failed to save state for %s: %v", key, err)
			continue
		}
		state.ComponentStates[key] = providerState
	}

	// Save to Redis if enabled
	if sm.enableRedis && sm.redisClient != nil {
		if err := sm.saveToRedis(ctx, state); err != nil {
			log.Printf("Failed to save state to Redis: %v", err)
		}
	}

	// Save to file if enabled
	if sm.enableFile {
		if err := sm.saveToFile(state); err != nil {
			log.Printf("Failed to save state to file: %v", err)
		}
	}

	log.Printf("State saved successfully with %d components", len(state.ComponentStates))
	return nil
}

// RestoreState restores the application state
func (sm *StateManager) RestoreState(ctx context.Context) error {
	var state ApplicationState
	var err error

	// Try to restore from Redis first
	if sm.enableRedis && sm.redisClient != nil {
		state, err = sm.restoreFromRedis(ctx)
		if err == nil {
			log.Printf("State restored from Redis")
		} else {
			log.Printf("Failed to restore from Redis: %v", err)
		}
	}

	// Fallback to file if Redis failed or not available
	if err != nil && sm.enableFile {
		state, err = sm.restoreFromFile()
		if err == nil {
			log.Printf("State restored from file")
		} else {
			log.Printf("Failed to restore from file: %v", err)
			return err
		}
	}

	if err != nil {
		return fmt.Errorf("no state source available: %w", err)
	}

	// Check if state is too old
	if time.Since(state.Timestamp) > sm.retentionTime {
		log.Printf("Stored state is too old (%s), skipping restoration", time.Since(state.Timestamp))
		return fmt.Errorf("state is too old")
	}

	// Restore state to registered providers
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	for key, provider := range sm.stateRegistry {
		if componentState, exists := state.ComponentStates[key]; exists {
			if err := provider.RestoreState(componentState); err != nil {
				log.Printf("Failed to restore state for %s: %v", key, err)
			} else {
				log.Printf("State restored for component: %s", key)
			}
		}
	}

	log.Printf("State restoration completed for %d components", len(sm.stateRegistry))
	return nil
}

// saveToRedis saves state to Redis
func (sm *StateManager) saveToRedis(ctx context.Context, state ApplicationState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	key := "alchemorsel:dev:state"
	if err := sm.redisClient.Set(ctx, key, data, sm.retentionTime).Err(); err != nil {
		return fmt.Errorf("failed to set Redis key: %w", err)
	}

	return nil
}

// restoreFromRedis restores state from Redis
func (sm *StateManager) restoreFromRedis(ctx context.Context) (ApplicationState, error) {
	key := "alchemorsel:dev:state"
	data, err := sm.redisClient.Get(ctx, key).Result()
	if err != nil {
		return ApplicationState{}, fmt.Errorf("failed to get Redis key: %w", err)
	}

	var state ApplicationState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return ApplicationState{}, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return state, nil
}

// saveToFile saves state to file
func (sm *StateManager) saveToFile(state ApplicationState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := ioutil.WriteFile(sm.stateFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// restoreFromFile restores state from file
func (sm *StateManager) restoreFromFile() (ApplicationState, error) {
	data, err := ioutil.ReadFile(sm.stateFile)
	if err != nil {
		return ApplicationState{}, fmt.Errorf("failed to read state file: %w", err)
	}

	var state ApplicationState
	if err := json.Unmarshal(data, &state); err != nil {
		return ApplicationState{}, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return state, nil
}

// StartPeriodicSave starts automatic periodic state saving
func (sm *StateManager) StartPeriodicSave(ctx context.Context) {
	ticker := time.NewTicker(sm.saveInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := sm.SaveState(ctx); err != nil {
					log.Printf("Periodic state save failed: %v", err)
				}
			}
		}
	}()
	
	log.Printf("Started periodic state saving every %s", sm.saveInterval)
}

// Cleanup removes old state data
func (sm *StateManager) Cleanup(ctx context.Context) error {
	// Remove old state file if it exists and is too old
	if sm.enableFile {
		if info, err := os.Stat(sm.stateFile); err == nil {
			if time.Since(info.ModTime()) > sm.retentionTime {
				if err := os.Remove(sm.stateFile); err != nil {
					log.Printf("Failed to remove old state file: %v", err)
				} else {
					log.Printf("Removed old state file")
				}
			}
		}
	}

	return nil
}

// Stop gracefully shuts down the state manager
func (sm *StateManager) Stop(ctx context.Context) error {
	// Save final state
	if err := sm.SaveState(ctx); err != nil {
		log.Printf("Failed to save final state: %v", err)
	}

	// Close Redis connection
	if sm.redisClient != nil {
		return sm.redisClient.Close()
	}

	return nil
}

// Example implementations of StateProvider

// SessionStateProvider manages session state
type SessionStateProvider struct {
	sessions map[string]*SessionState
	mutex    sync.RWMutex
}

func NewSessionStateProvider() *SessionStateProvider {
	return &SessionStateProvider{
		sessions: make(map[string]*SessionState),
	}
}

func (ssp *SessionStateProvider) SaveState() (interface{}, error) {
	ssp.mutex.RLock()
	defer ssp.mutex.RUnlock()
	
	// Copy sessions to avoid race conditions
	sessionsCopy := make(map[string]*SessionState)
	for k, v := range ssp.sessions {
		sessionsCopy[k] = v
	}
	
	return sessionsCopy, nil
}

func (ssp *SessionStateProvider) RestoreState(data interface{}) error {
	sessions, ok := data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid session state data")
	}
	
	ssp.mutex.Lock()
	defer ssp.mutex.Unlock()
	
	ssp.sessions = make(map[string]*SessionState)
	
	for k, v := range sessions {
		sessionData, err := json.Marshal(v)
		if err != nil {
			continue
		}
		
		var session SessionState
		if err := json.Unmarshal(sessionData, &session); err != nil {
			continue
		}
		
		// Only restore non-expired sessions
		if time.Now().Before(session.ExpiresAt) {
			ssp.sessions[k] = &session
		}
	}
	
	log.Printf("Restored %d active sessions", len(ssp.sessions))
	return nil
}

func (ssp *SessionStateProvider) GetStateKey() string {
	return "sessions"
}

// CacheStateProvider manages cache state
type CacheStateProvider struct {
	cache map[string]string
	ttls  map[string]time.Time
	mutex sync.RWMutex
}

func NewCacheStateProvider() *CacheStateProvider {
	return &CacheStateProvider{
		cache: make(map[string]string),
		ttls:  make(map[string]time.Time),
	}
}

func (csp *CacheStateProvider) SaveState() (interface{}, error) {
	csp.mutex.RLock()
	defer csp.mutex.RUnlock()
	
	// Convert TTLs to Unix timestamps for serialization
	ttlsUnix := make(map[string]int64)
	for k, v := range csp.ttls {
		ttlsUnix[k] = v.Unix()
	}
	
	state := CacheState{
		Values:  csp.cache,
		TTLs:    ttlsUnix,
		SavedAt: time.Now(),
	}
	
	// Only include non-expired entries
	for k := range csp.cache {
		if expiry, exists := csp.ttls[k]; exists && time.Now().After(expiry) {
			delete(state.Values, k)
			delete(state.TTLs, k)
		} else {
			state.Keys = append(state.Keys, k)
		}
	}
	
	return state, nil
}

func (csp *CacheStateProvider) RestoreState(data interface{}) error {
	stateData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	var state CacheState
	if err := json.Unmarshal(stateData, &state); err != nil {
		return err
	}
	
	csp.mutex.Lock()
	defer csp.mutex.Unlock()
	
	csp.cache = make(map[string]string)
	csp.ttls = make(map[string]time.Time)
	
	// Restore non-expired entries
	for k, v := range state.Values {
		if ttlUnix, exists := state.TTLs[k]; exists {
			expiry := time.Unix(ttlUnix, 0)
			if time.Now().Before(expiry) {
				csp.cache[k] = v
				csp.ttls[k] = expiry
			}
		}
	}
	
	log.Printf("Restored %d cache entries", len(csp.cache))
	return nil
}

func (csp *CacheStateProvider) GetStateKey() string {
	return "cache"
}