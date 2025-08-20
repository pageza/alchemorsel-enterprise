// Package hotreload provides comprehensive file watching for templates, assets, and migrations
package hotreload

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher provides advanced file watching capabilities
type FileWatcher struct {
	watcher    *fsnotify.Watcher
	handlers   map[string]FileHandler
	debouncer  map[string]*time.Timer
	mutex      sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	
	// Configuration
	debounceDelay time.Duration
	maxEvents     int
	batchSize     int
}

// FileHandler defines how to handle different types of file changes
type FileHandler interface {
	HandleChange(event FileChangeEvent) error
	ShouldHandle(path string) bool
	GetDescription() string
}

// TemplateWatcher handles template file changes
type TemplateWatcher struct {
	templatePaths []string
	reloadServer  *LiveReloadServer
}

// StaticAssetWatcher handles CSS/JS asset changes
type StaticAssetWatcher struct {
	assetPaths   []string
	reloadServer *LiveReloadServer
	buildCommand string
}

// MigrationWatcher handles database migration changes
type MigrationWatcher struct {
	migrationPath string
	db            *sql.DB
	autoMigrate   bool
}

// ConfigWatcher handles configuration file changes
type ConfigWatcher struct {
	configPaths []string
	restartFn   func() error
}

// NewFileWatcher creates a comprehensive file watcher
func NewFileWatcher() (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	fw := &FileWatcher{
		watcher:       watcher,
		handlers:      make(map[string]FileHandler),
		debouncer:     make(map[string]*time.Timer),
		ctx:           ctx,
		cancel:        cancel,
		debounceDelay: 250 * time.Millisecond,
		maxEvents:     100,
		batchSize:     10,
	}

	return fw, nil
}

// RegisterHandler registers a file handler for a specific pattern
func (fw *FileWatcher) RegisterHandler(pattern string, handler FileHandler) {
	fw.mutex.Lock()
	defer fw.mutex.Unlock()
	
	fw.handlers[pattern] = handler
	log.Printf("Registered file handler: %s - %s", pattern, handler.GetDescription())
}

// AddWatchPath adds a path to be watched
func (fw *FileWatcher) AddWatchPath(path string) error {
	return filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and common excludes
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || 
			info.Name() == "tmp" || 
			info.Name() == "vendor" || 
			info.Name() == "node_modules") {
			return filepath.SkipDir
		}

		if info.IsDir() {
			if err := fw.watcher.Add(walkPath); err != nil {
				log.Printf("Failed to watch directory %s: %v", walkPath, err)
			} else {
				log.Printf("Watching directory: %s", walkPath)
			}
		}

		return nil
	})
}

// Start begins file watching
func (fw *FileWatcher) Start() {
	go fw.watchLoop()
	log.Printf("File watcher started with %d handlers", len(fw.handlers))
}

// Stop gracefully shuts down the file watcher
func (fw *FileWatcher) Stop() error {
	fw.cancel()
	return fw.watcher.Close()
}

// watchLoop is the main event loop for file watching
func (fw *FileWatcher) watchLoop() {
	for {
		select {
		case <-fw.ctx.Done():
			return
			
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)
			
		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}

// handleEvent processes a file system event
func (fw *FileWatcher) handleEvent(event fsnotify.Event) {
	// Skip temporary and backup files
	if strings.HasSuffix(event.Name, "~") || 
		strings.HasSuffix(event.Name, ".tmp") ||
		strings.Contains(event.Name, ".git") {
		return
	}

	fw.mutex.Lock()
	defer fw.mutex.Unlock()

	// Debounce rapid events
	if timer, exists := fw.debouncer[event.Name]; exists {
		timer.Stop()
	}

	fw.debouncer[event.Name] = time.AfterFunc(fw.debounceDelay, func() {
		fw.processEvent(event)
		fw.mutex.Lock()
		delete(fw.debouncer, event.Name)
		fw.mutex.Unlock()
	})
}

// processEvent handles the actual file change processing
func (fw *FileWatcher) processEvent(event fsnotify.Event) {
	changeEvent := FileChangeEvent{
		Path:      event.Name,
		Operation: event.Op.String(),
		Timestamp: time.Now(),
	}

	log.Printf("Processing file change: %s (%s)", event.Name, event.Op.String())

	// Find matching handlers
	for pattern, handler := range fw.handlers {
		if fw.matchesPattern(event.Name, pattern) && handler.ShouldHandle(event.Name) {
			go func(h FileHandler) {
				if err := h.HandleChange(changeEvent); err != nil {
					log.Printf("Handler error for %s: %v", event.Name, err)
				}
			}(handler)
		}
	}
}

// matchesPattern checks if a file path matches a pattern
func (fw *FileWatcher) matchesPattern(path, pattern string) bool {
	matched, err := filepath.Match(pattern, filepath.Base(path))
	if err != nil {
		return false
	}
	return matched || strings.Contains(path, pattern)
}

// Template Watcher Implementation

// NewTemplateWatcher creates a new template watcher
func NewTemplateWatcher(templatePaths []string, reloadServer *LiveReloadServer) *TemplateWatcher {
	return &TemplateWatcher{
		templatePaths: templatePaths,
		reloadServer:  reloadServer,
	}
}

func (tw *TemplateWatcher) HandleChange(event FileChangeEvent) error {
	log.Printf("Template changed: %s", event.Path)
	
	// Trigger browser reload
	if tw.reloadServer != nil {
		tw.reloadServer.TriggerReload(event.Path)
	}
	
	return nil
}

func (tw *TemplateWatcher) ShouldHandle(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".html" || ext == ".tmpl" || ext == ".tpl"
}

func (tw *TemplateWatcher) GetDescription() string {
	return "Template file watcher (HTML, TMPL, TPL)"
}

// Static Asset Watcher Implementation

// NewStaticAssetWatcher creates a new static asset watcher
func NewStaticAssetWatcher(assetPaths []string, reloadServer *LiveReloadServer, buildCommand string) *StaticAssetWatcher {
	return &StaticAssetWatcher{
		assetPaths:   assetPaths,
		reloadServer: reloadServer,
		buildCommand: buildCommand,
	}
}

func (saw *StaticAssetWatcher) HandleChange(event FileChangeEvent) error {
	log.Printf("Static asset changed: %s", event.Path)
	
	ext := filepath.Ext(event.Path)
	
	// Run build command if specified
	if saw.buildCommand != "" && (ext == ".scss" || ext == ".sass" || ext == ".less") {
		if err := saw.runBuildCommand(); err != nil {
			log.Printf("Build command failed: %v", err)
		}
	}
	
	// Trigger browser reload
	if saw.reloadServer != nil {
		saw.reloadServer.TriggerReload(event.Path)
	}
	
	return nil
}

func (saw *StaticAssetWatcher) ShouldHandle(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".css" || ext == ".js" || ext == ".scss" || ext == ".sass" || ext == ".less" || ext == ".ts"
}

func (saw *StaticAssetWatcher) GetDescription() string {
	return "Static asset watcher (CSS, JS, SCSS, TypeScript)"
}

func (saw *StaticAssetWatcher) runBuildCommand() error {
	// This would run the build command (npm run build, etc.)
	// Implementation depends on specific build system
	log.Printf("Running build command: %s", saw.buildCommand)
	return nil
}

// Migration Watcher Implementation

// NewMigrationWatcher creates a new migration watcher
func NewMigrationWatcher(migrationPath string, db *sql.DB, autoMigrate bool) *MigrationWatcher {
	return &MigrationWatcher{
		migrationPath: migrationPath,
		db:            db,
		autoMigrate:   autoMigrate,
	}
}

func (mw *MigrationWatcher) HandleChange(event FileChangeEvent) error {
	log.Printf("Migration file changed: %s", event.Path)
	
	if mw.autoMigrate && mw.db != nil {
		log.Printf("Auto-migrating database...")
		return mw.runMigrations()
	}
	
	log.Printf("Auto-migration disabled. Please run migrations manually.")
	return nil
}

func (mw *MigrationWatcher) ShouldHandle(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".sql" && strings.Contains(path, mw.migrationPath)
}

func (mw *MigrationWatcher) GetDescription() string {
	return "Database migration watcher (SQL files)"
}

func (mw *MigrationWatcher) runMigrations() error {
	// This would run database migrations
	// Implementation depends on migration system (migrate, goose, etc.)
	log.Printf("Running database migrations from %s", mw.migrationPath)
	
	// Example migration logic would go here
	// For now, just log the action
	
	return nil
}

// Config Watcher Implementation

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configPaths []string, restartFn func() error) *ConfigWatcher {
	return &ConfigWatcher{
		configPaths: configPaths,
		restartFn:   restartFn,
	}
}

func (cw *ConfigWatcher) HandleChange(event FileChangeEvent) error {
	log.Printf("Configuration changed: %s", event.Path)
	
	if cw.restartFn != nil {
		log.Printf("Triggering application restart due to config change...")
		return cw.restartFn()
	}
	
	return nil
}

func (cw *ConfigWatcher) ShouldHandle(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".toml" || ext == ".env"
}

func (cw *ConfigWatcher) GetDescription() string {
	return "Configuration file watcher (YAML, JSON, TOML, ENV)"
}

// SetupDevelopmentWatchers configures all watchers for development
func SetupDevelopmentWatchers(reloadServer *LiveReloadServer, db *sql.DB) (*FileWatcher, error) {
	fileWatcher, err := NewFileWatcher()
	if err != nil {
		return nil, err
	}

	// Template watcher
	templatePaths := []string{
		"internal/infrastructure/http/server/templates",
		"internal/infrastructure/http/webserver/templates",
	}
	templateWatcher := NewTemplateWatcher(templatePaths, reloadServer)
	fileWatcher.RegisterHandler("*.html", templateWatcher)

	// Static asset watcher
	assetPaths := []string{
		"internal/infrastructure/http/server/static",
		"internal/infrastructure/http/webserver/static",
	}
	assetWatcher := NewStaticAssetWatcher(assetPaths, reloadServer, "npm run build")
	fileWatcher.RegisterHandler("*.css", assetWatcher)
	fileWatcher.RegisterHandler("*.js", assetWatcher)

	// Migration watcher
	migrationWatcher := NewMigrationWatcher("internal/infrastructure/persistence/migrations/sql", db, true)
	fileWatcher.RegisterHandler("*.sql", migrationWatcher)

	// Config watcher
	configWatcher := NewConfigWatcher([]string{"config"}, nil)
	fileWatcher.RegisterHandler("*.yaml", configWatcher)
	fileWatcher.RegisterHandler("*.yml", configWatcher)

	// Add watch paths
	watchPaths := []string{
		"internal/infrastructure/http/server/templates",
		"internal/infrastructure/http/webserver/templates",
		"internal/infrastructure/http/server/static",
		"internal/infrastructure/http/webserver/static",
		"internal/infrastructure/persistence/migrations/sql",
		"config",
	}

	for _, path := range watchPaths {
		if _, err := os.Stat(path); err == nil {
			if err := fileWatcher.AddWatchPath(path); err != nil {
				log.Printf("Failed to watch path %s: %v", path, err)
			}
		}
	}

	return fileWatcher, nil
}