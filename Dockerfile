# Build stage
FROM golang:1.25.4-alpine AS builder

WORKDIR /app

# Install git for version detection and git operations
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with version information
# Accept build args for version info, fall back to git describe if not provided
ARG VERSION
ARG COMMIT
ARG BUILD_DATE

RUN VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")} && \
    COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")} && \
    BUILD_DATE=${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")} && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-s -w \
        -X github.com/d0ugal/git-exporter/internal/version.Version=$VERSION \
        -X github.com/d0ugal/git-exporter/internal/version.Commit=$COMMIT \
        -X github.com/d0ugal/git-exporter/internal/version.BuildDate=$BUILD_DATE" \
    -o git-exporter ./cmd/main.go

# Final stage
FROM alpine:3.23.0

# Install git for git operations in the container
RUN apk --no-cache add ca-certificates git

# Setup an unprivileged user
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

WORKDIR /app
RUN chown appuser:appgroup /app

USER appuser

# Copy the binary from builder stage
COPY --from=builder --chown=appuser:appuser /app/git-exporter .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./git-exporter"]

