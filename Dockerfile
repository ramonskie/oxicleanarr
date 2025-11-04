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
    -o prunarr \
    ./cmd/prunarr

# Stage 3: Runtime Image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 prunarr && \
    adduser -D -u 1000 -G prunarr prunarr

# Set working directory
WORKDIR /app

# Copy binary from backend builder
COPY --from=backend-builder /app/prunarr /app/prunarr

# Copy frontend dist from frontend builder
COPY --from=frontend-builder /app/web/dist /app/web/dist

# Copy example config (users can override with volume mount)
COPY config/prunarr.yaml.example /app/config/prunarr.yaml.example

# Create directories with proper ownership
RUN mkdir -p /app/data /app/config /app/logs && \
    chown -R prunarr:prunarr /app

# Switch to non-root user
USER prunarr

# Expose HTTP port
EXPOSE 8080

# Set environment variables
ENV LOG_LEVEL=info \
    LOG_FORMAT=json \
    CONFIG_PATH=/app/config/prunarr.yaml \
    DATA_PATH=/app/data \
    LOG_FILE=/app/logs/prunarr.log \
    FRONTEND_DIST_PATH=/app/web/dist

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
ENTRYPOINT ["/app/prunarr"]
CMD ["--config", "/app/config/prunarr.yaml"]
