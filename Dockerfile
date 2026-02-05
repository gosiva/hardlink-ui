# Multi-stage build for Go application
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o hardlink-ui ./cmd/server

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata shadow su-exec sqlite-libs

# Copy binary and web assets
COPY --from=builder /build/hardlink-ui /usr/local/bin/hardlink-ui
COPY web/ /app/web/

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Create appuser (will be adjusted by entrypoint)
RUN useradd -m appuser || adduser -D appuser

# Create data directory for database
RUN mkdir -p /app/data && chown -R appuser:appuser /app/data

EXPOSE 8000

ENV WEB_PATH=/app/web
ENV DB_PATH=/app/data/hardlink-ui.db

ENTRYPOINT ["/entrypoint.sh"]
CMD ["hardlink-ui"]
