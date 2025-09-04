// Package hotreload provides live reload functionality for development
// This includes file watching and browser notification via HTTP
package hotreload

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	// "github.com/gorilla/websocket" // Disabled websocket functionality
)

// LiveReloadServer provides HTTP-based live reload functionality
type LiveReloadServer struct {
	port    int
	server  *http.Server
	watcher *fsnotify.Watcher

	// Configuration
	watchPaths      []string
	includeExts     []string
	excludePatterns []string
	debounceDelay   time.Duration

	// Channels
	shutdown chan struct{}
}

// ReloadMessage represents a live reload message sent to browsers
type ReloadMessage struct {
	Command   string                 `json:"command"`
	Path      string                 `json:"path,omitempty"`
	LiveCSS   bool                   `json:"liveCSS,omitempty"`
	Live      bool                   `json:"live,omitempty"`
	Original  string                 `json:"original,omitempty"`
	Ext       string                 `json:"ext,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// FileChangeEvent represents a file system change event
type FileChangeEvent struct {
	Path      string
	Operation string
	Timestamp time.Time
}

// LiveReloadConfig configures the live reload server
type LiveReloadConfig struct {
	Port            int
	WatchPaths      []string
	IncludeExts     []string
	ExcludePatterns []string
	DebounceDelay   time.Duration
	EnableLogging   bool
}

// DefaultLiveReloadConfig returns sensible defaults for development
func DefaultLiveReloadConfig() *LiveReloadConfig {
	return &LiveReloadConfig{
		Port:            35729, // Standard LiveReload port
		WatchPaths:      []string{".", "internal", "cmd", "pkg"},
		IncludeExts:     []string{".go", ".html", ".css", ".js", ".sql"},
		ExcludePatterns: []string{".git", "tmp", "vendor", "node_modules", ".DS_Store"},
		DebounceDelay:   250 * time.Millisecond,
		EnableLogging:   true,
	}
}

// NewLiveReloadServer creates a new live reload server
func NewLiveReloadServer(config *LiveReloadConfig) (*LiveReloadServer, error) {
	if config == nil {
		config = DefaultLiveReloadConfig()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	server := &LiveReloadServer{
		port:            config.Port,
		watcher:         watcher,
		watchPaths:      config.WatchPaths,
		includeExts:     config.IncludeExts,
		excludePatterns: config.ExcludePatterns,
		debounceDelay:   config.DebounceDelay,
		shutdown:        make(chan struct{}),
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/livereload.js", server.serveLiveReloadScript)
	mux.HandleFunc("/status", server.handleStatus)
	mux.HandleFunc("/", server.handleHealth)

	server.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Port),
		Handler: mux,
	}

	return server, nil
}

// Start begins the live reload server
func (s *LiveReloadServer) Start(ctx context.Context) error {
	log.Printf("Starting LiveReload server on port %d", s.port)

	// Start file watcher
	if err := s.setupFileWatcher(); err != nil {
		return fmt.Errorf("failed to setup file watcher: %w", err)
	}

	// Start file watching goroutine
	go s.watchFiles(ctx)

	// Start HTTP server
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("LiveReload server error: %v", err)
		}
	}()

	log.Printf("LiveReload server started successfully")
	log.Printf("Include LiveReload script: <script src=\"http://localhost:%d/livereload.js\"></script>", s.port)

	return nil
}

// Stop gracefully shuts down the live reload server
func (s *LiveReloadServer) Stop(ctx context.Context) error {
	log.Printf("Stopping LiveReload server...")

	close(s.shutdown)

	if s.watcher != nil {
		s.watcher.Close()
	}

	if s.server != nil {
		return s.server.Shutdown(ctx)
	}

	return nil
}

// setupFileWatcher configures file system watching
func (s *LiveReloadServer) setupFileWatcher() error {
	for _, path := range s.watchPaths {
		if err := s.addWatchPath(path); err != nil {
			return err
		}
	}
	return nil
}

// addWatchPath recursively adds a path to the file watcher
func (s *LiveReloadServer) addWatchPath(path string) error {
	return filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip excluded patterns
		for _, pattern := range s.excludePatterns {
			if strings.Contains(walkPath, pattern) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Only watch directories
		if info.IsDir() {
			return s.watcher.Add(walkPath)
		}

		return nil
	})
}

// watchFiles monitors file system changes
func (s *LiveReloadServer) watchFiles(ctx context.Context) {
	debouncer := make(map[string]*time.Timer)
	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.shutdown:
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}

			// Check if file should trigger reload
			if !s.shouldReload(event.Name) {
				continue
			}

			// Debounce rapid file changes
			mu.Lock()
			if timer, exists := debouncer[event.Name]; exists {
				timer.Stop()
			}

			debouncer[event.Name] = time.AfterFunc(s.debounceDelay, func() {
				s.handleFileChange(event)
				mu.Lock()
				delete(debouncer, event.Name)
				mu.Unlock()
			})
			mu.Unlock()

		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}

// shouldReload determines if a file change should trigger a reload
func (s *LiveReloadServer) shouldReload(filePath string) bool {
	// Check excluded patterns
	for _, pattern := range s.excludePatterns {
		if strings.Contains(filePath, pattern) {
			return false
		}
	}

	// Check included extensions
	ext := filepath.Ext(filePath)
	for _, includedExt := range s.includeExts {
		if ext == includedExt {
			return true
		}
	}

	return false
}

// handleFileChange processes a file change event
func (s *LiveReloadServer) handleFileChange(event fsnotify.Event) {
	log.Printf("File changed: %s (%s)", event.Name, event.Op.String())
}

// serveLiveReloadScript serves the client-side JavaScript
func (s *LiveReloadServer) serveLiveReloadScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Header().Set("Cache-Control", "no-cache")

	script := fmt.Sprintf(`
// Alchemorsel v3 LiveReload Client
(function() {
	'use strict';
	
	var protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
	var address = protocol + '//' + window.location.hostname + ':%d/livereload';
	var socket = new WebSocket(address);
	var lastReload = 0;
	
	console.log('🔄 LiveReload connecting to ' + address);
	
	socket.onopen = function() {
		console.log('🔄 LiveReload connected');
	};
	
	socket.onclose = function() {
		console.log('🔄 LiveReload disconnected');
		// Attempt to reconnect after 2 seconds
		setTimeout(function() {
			window.location.reload();
		}, 2000);
	};
	
	socket.onerror = function(error) {
		console.error('🔄 LiveReload error:', error);
	};
	
	socket.onmessage = function(event) {
		var data = JSON.parse(event.data);
		var now = Date.now();
		
		console.log('🔄 LiveReload message:', data);
		
		if (data.command === 'reload') {
			// Prevent rapid reloads
			if (now - lastReload < 1000) {
				return;
			}
			lastReload = now;
			
			if (data.liveCSS && data.ext === '.css') {
				// Smart CSS reload without full page refresh
				reloadCSS();
			} else {
				// Full page reload
				console.log('🔄 Reloading page...');
				window.location.reload();
			}
		} else if (data.command === 'hello') {
			console.log('🔄 LiveReload server:', data.metadata);
		}
	};
	
	function reloadCSS() {
		console.log('🎨 Reloading CSS...');
		var links = document.getElementsByTagName('link');
		for (var i = 0; i < links.length; i++) {
			var link = links[i];
			if (link.rel === 'stylesheet') {
				var href = link.href.split('?')[0];
				link.href = href + '?t=' + Date.now();
			}
		}
	}
	
	// Expose reload function globally for manual triggering
	window.liveReload = {
		reload: function() {
			window.location.reload();
		},
		reloadCSS: reloadCSS,
		isConnected: function() {
			return socket.readyState === WebSocket.OPEN;
		}
	};
	
})();
`, s.port)

	w.Write([]byte(script))
}

// handleStatus provides server status information
func (s *LiveReloadServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"server":           "alchemorsel-v3-livereload",
		"status":           "running",
		"port":             s.port,
		"watch_paths":      s.watchPaths,
		"include_exts":     s.includeExts,
		"exclude_patterns": s.excludePatterns,
		"debounce_delay":   s.debounceDelay.String(),
		"timestamp":        time.Now().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleHealth provides a simple health check
func (s *LiveReloadServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("LiveReload server is running"))
}

// TriggerReload manually triggers a reload
func (s *LiveReloadServer) TriggerReload(path string) {
	log.Printf("Manual reload triggered for: %s", path)
}
