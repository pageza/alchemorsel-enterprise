# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies and security tools
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    upx \
    gcc \
    musl-dev

# Set security labels
LABEL maintainer="Alchemorsel Team <devops@alchemorsel.com>"
LABEL org.opencontainers.image.source="https://github.com/alchemorsel/v3"
LABEL org.opencontainers.image.description="Alchemorsel v3 API Server"
LABEL org.opencontainers.image.licenses="MIT"

# Set working directory
WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./

# Download dependencies with verification
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Update module dependencies for Go 1.23
RUN go mod tidy

# Run security and quality checks (temporarily disabled for Docker build)
# RUN go vet ./...
# RUN go test -race -short ./...

# Build optimized binary with security flags
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a \
    -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -tags netgo \
    -o main cmd/api-pure/main.go

# Compress binary
RUN upx --best --lzma main

# Distroless stage for maximum security
FROM gcr.io/distroless/static:nonroot

# Import timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy binary from builder stage
COPY --from=builder /app/main /app/main

# Copy configuration files
COPY --from=builder /app/config /app/config

# Copy static files and templates
COPY --from=builder /app/internal/infrastructure/http/server/static /app/static
COPY --from=builder /app/internal/infrastructure/http/server/templates /app/templates

# Copy migration files
COPY --from=builder /app/internal/infrastructure/persistence/migrations /app/migrations

# Set working directory
WORKDIR /app

# Use nonroot user from distroless
USER nonroot:nonroot

# Expose port
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/app/main", "--health-check"]

# Run the application
ENTRYPOINT ["/app/main"]