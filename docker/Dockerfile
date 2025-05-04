# Development stage
FROM golang:1.24-alpine AS dev

# Set working directory
WORKDIR /app

# Install development tools
RUN apk add --no-cache git

# Copy the source code
COPY api/ ./

# Command for development
CMD ["go", "run", "."]

# Build stage
FROM golang:1.24-alpine AS build

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go.mod and go.sum files first for better layer caching
COPY api/go.mod api/go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY api/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o worldgen-api .

# Final lightweight runtime stage
FROM alpine:latest

# Add necessary runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create a non-root user to run the application
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /app

# Copy the binary from the build stage
COPY --from=build /app/worldgen-api .

# Set ownership of the application files
RUN chown -R appuser:appuser /app

# Use the non-root user
USER appuser

# Expose API port
EXPOSE 8080

# Command to run the application
CMD ["./worldgen-api"]

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1