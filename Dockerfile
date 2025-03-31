# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Copy go mod files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN go build -ldflags="-w -s" -o promcached ./cmd/promcached

# Runtime stage
FROM alpine:latest

# Install CA certificates for HTTPS connections
RUN apk --no-cache add ca-certificates && \
    mkdir -p /app

WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/promcached /app/promcached

# Create non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose the default port
EXPOSE 9091

# Add health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:9091/health || exit 1

# Set environment variables with defaults
ENV PROMCACHE_LISTEN_ADDR=:9091 \
    PROMCACHE_UPSTREAM_URL=http://prometheus:9090 \
    PROMCACHE_TTL=5m \
    PROMCACHE_LOG_LEVEL=info

ENTRYPOINT ["/app/promcached"]