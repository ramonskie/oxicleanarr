# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder

WORKDIR /app/web

# Copy frontend package files
COPY web/package*.json ./

# Install frontend dependencies (including dev deps needed for build)
RUN npm ci

# Copy frontend source
COPY web/ ./

# Build frontend (output to /app/web/dist)
RUN npm run build

# Stage 2: Build Backend
FROM golang:1.25-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binary with static linking
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o oxicleanarr \
    ./cmd/oxicleanarr

# Stage 3: Runtime Image
FROM alpine:latest

# Install runtime dependencies (removed shadow since we don't need usermod/groupmod)
RUN apk add --no-cache ca-certificates tzdata su-exec

# Set working directory
WORKDIR /app

# Copy binary from backend builder
COPY --from=backend-builder /app/oxicleanarr /app/oxicleanarr

# Copy frontend dist from frontend builder
COPY --from=frontend-builder /app/web/dist /app/web/dist

# Copy example config (users can override with volume mount)
COPY config/oxicleanarr.yaml.example /app/config/oxicleanarr.yaml.example

# Copy entrypoint script
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Create directories (no ownership needed - handled by PUID/PGID at runtime)
RUN mkdir -p /app/data /app/config /app/logs

# Note: Container starts as root, entrypoint drops to PUID:PGID

# Expose HTTP port
EXPOSE 8080

# Set environment variables
ENV LOG_LEVEL=info \
    LOG_FORMAT=json \
    CONFIG_PATH=/app/config/oxicleanarr.yaml \
    DATA_PATH=/app/data \
    LOG_FILE=/app/logs/oxicleanarr.log \
    FRONTEND_DIST_PATH=/app/web/dist

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run via entrypoint script (handles PUID/PGID)
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/app/oxicleanarr", "--config", "/app/config/oxicleanarr.yaml"]
