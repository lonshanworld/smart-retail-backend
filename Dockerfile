# ============================================
# Stage 1: Build Go Application
# ============================================
FROM golang:1.21-alpine AS go-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download
RUN go mod verify

# Copy the entire source code
COPY . .

# Build the application
# CGO_ENABLED=0 creates a static binary
# -ldflags="-w -s" reduces binary size by removing debug info
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o smart-retail-backend \
    main.go

# ============================================
# Stage 2: Run Application
# ============================================
FROM alpine:3.19

# Install ca-certificates for HTTPS requests and timezone data
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user for security
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=go-builder /build/smart-retail-backend .

# Copy database schema (if needed for migrations)
COPY --from=go-builder /build/schema.sql .

# Change ownership to non-root user
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Expose the application port
EXPOSE 5000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:5000/api/v1/health || exit 1

# Run the application
CMD ["./smart-retail-backend"]
