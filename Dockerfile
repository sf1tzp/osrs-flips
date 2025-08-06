# Build stage
FROM golang:1.24-alpine AS deps

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod go mod download

FROM deps AS builder

# Copy source code
COPY . .

# Build the application
RUN --mount=type=cache,target=/go/pkg/mod CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o osrs-flips-bot ./cmd/bot

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S osrsbot && \
    adduser -u 1001 -S osrsbot -G osrsbot

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/osrs-flips-bot .

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Create directory for config files (will be mounted as volumes)
RUN mkdir -p /app/config

# Change ownership to non-root user
RUN chown -R osrsbot:osrsbot /app

# Switch to non-root user
USER osrsbot

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD pgrep osrs-flips-bot || exit 1

# Expose no ports (Discord bot doesn't need incoming connections)

# Run the bot
CMD ["./osrs-flips-bot"]
