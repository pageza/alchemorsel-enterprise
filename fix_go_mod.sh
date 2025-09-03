#!/bin/bash
set -e

# Fix go mod tidy issue for CI vulnerability scan
echo "🔧 Fixing Go module dependencies..."

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo "❌ go.mod not found in current directory"
    exit 1
fi

echo "✅ Found go.mod file"
echo "📦 Current Go version and environment:"

# Try to use docker for go mod tidy
if command -v docker &> /dev/null; then
    echo "🐳 Using Docker to run go mod tidy..."
    
    # Use absolute paths and ensure proper mounting
    WORK_DIR="$(pwd)"
    echo "Working directory: $WORK_DIR"
    
    # Run go mod tidy in docker with proper permissions
    sudo docker run --rm \
        -v "$WORK_DIR:/workspace" \
        -w /workspace \
        --user "$(id -u):$(id -g)" \
        golang:1.23-alpine \
        sh -c "
            echo 'Current directory:' && pwd
            echo 'Files present:' && ls -la go.*
            echo 'Running go mod tidy...'
            go mod tidy
            echo 'go mod tidy completed successfully!'
        "
    
    echo "✅ Go mod tidy completed via Docker"
else
    echo "❌ Docker not available, cannot run go mod tidy"
    exit 1
fi

echo "🎉 Go module dependencies fixed!"