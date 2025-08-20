# ADR-0018: Hot Reload Development Workflow

## Status
Accepted

## Context
Alchemorsel v3 development requires rapid iteration cycles for both backend Go code and frontend templates. Traditional containerized development can introduce significant delays between code changes and seeing results, reducing developer productivity and breaking the development flow state.

Development workflow requirements:
- Instant feedback on Go code changes
- Automatic template and asset reloading
- Database schema migration on changes  
- Consistent behavior between development and production
- Support for debugging and profiling tools

Current challenges:
- Docker container rebuilds are slow (30-60 seconds)
- Go compilation delays interrupt development flow
- Template changes require manual reloads
- Database migration testing requires container restarts
- Debugging tools difficult to attach in containers

## Decision
We will implement a comprehensive hot reload development workflow using Air for Go hot reloading, integrated with Docker Compose for consistent environment management.

**Hot Reload Architecture:**

**Air Configuration (.air.toml):**
```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/server"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "docs"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html", "css", "js"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = false
  stop_on_root = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

**Docker Compose Development Setup:**
```yaml
version: '3.8'

services:
  web:
    build:
      context: .
      dockerfile: Dockerfile.dev
    volumes:
      - .:/app
      - go-modules:/go/pkg/mod  # Cache Go modules
    ports:
      - "8090:8080"  # External port mapping
      - "2345:2345"  # Delve debugger port
    environment:
      - CGO_ENABLED=0
      - GOOS=linux
      - GO111MODULE=on
      - AIR_ENABLE_COLORS=true
    networks:
      - alchemorsel-network
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
    command: air -c .air.toml

volumes:
  go-modules:
```

**Development Dockerfile:**
```dockerfile
# Dockerfile.dev
FROM golang:1.23-alpine

# Install Air for hot reloading
RUN go install github.com/cosmtrek/air@latest

# Install Delve debugger
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Set working directory
WORKDIR /app

# Install system dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files for dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Create tmp directory for Air
RUN mkdir -p tmp

# Expose ports for app and debugger
EXPOSE 8080 2345

# Default command (can be overridden)
CMD ["air", "-c", ".air.toml"]
```

**Asset Hot Reloading (CSS/JS):**
```yaml
# Additional service for frontend asset compilation
  assets:
    image: node:18-alpine
    working_dir: /app
    volumes:
      - ./assets:/app
      - node-modules:/app/node_modules
    command: npm run watch
    networks:
      - alchemorsel-network
```

**Database Migration Hot Reloading:**
```go
// pkg/migrations/watcher.go
func WatchMigrations(ctx context.Context, db *sql.DB) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        log.Fatal(err)
    }
    defer watcher.Close()

    err = watcher.Add("./migrations")
    if err != nil {
        log.Fatal(err)
    }

    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }
            if event.Op&fsnotify.Write == fsnotify.Write {
                log.Println("Migration file modified:", event.Name)
                // Run migrations automatically in development
                if os.Getenv("APP_ENV") == "development" {
                    runMigrations(db)
                }
            }
        case err, ok := <-watcher.Errors:
            if !ok {
                return
            }
            log.Println("Watcher error:", err)
        }
    }
}
```

**Template Hot Reloading:**
```go
// pkg/templates/loader.go
type TemplateLoader struct {
    templates map[string]*template.Template
    watcher   *fsnotify.Watcher
    debug     bool
}

func (tl *TemplateLoader) LoadTemplate(name string) (*template.Template, error) {
    if tl.debug {
        // Always reload templates in development
        return template.ParseGlob("templates/" + name + "*.html")
    }
    
    // Use cached templates in production
    if tmpl, exists := tl.templates[name]; exists {
        return tmpl, nil
    }
    
    return nil, fmt.Errorf("template %s not found", name)
}
```

**Development Workflow:**
1. `docker-compose -f docker-compose.dev.yml up`
2. Code changes automatically trigger:
   - Go binary recompilation and restart
   - Template reloading on next request
   - Asset recompilation (CSS/JS)
   - Database migration application (if enabled)
3. Browser automatically refreshes (with LiveReload extension)
4. Debugger available on port 2345 for IDE integration

**Performance Optimizations:**
- Go module caching in named volumes
- Incremental builds with proper .dockerignore
- File watching limited to relevant directories
- Parallel asset compilation where possible

## Consequences

### Positive
- Near-instant feedback on code changes (1-2 seconds)
- Maintained development flow state with minimal interruptions
- Consistent environment between development and production
- Integrated debugging and profiling capabilities
- Automated testing and migration workflows

### Negative
- Additional complexity in development environment setup
- Resource usage higher than production due to file watching
- Potential for development/production environment drift
- Requires team training on hot reload workflow

### Neutral
- Industry standard development workflow for Go applications
- Compatible with popular IDEs and editors
- Supports both individual and team development workflows