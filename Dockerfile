# Build stage
FROM golang:1.25.5-alpine AS builder

WORKDIR /app

# The platform depends on veex-build via local replace
# Context must be the root containing both
COPY veex-build ./veex-build
COPY veex-platform ./veex-platform

WORKDIR /app/veex-platform
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o veex-platform ./cmd/registry/main.go

# Final stage
FROM alpine:latest

# Install CA certificates for secure communication
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binary and assets
COPY --from=builder /app/veex-platform/veex-platform /usr/local/bin/veex-platform
COPY --from=builder /app/veex-platform/data ./data
# Copy needed schemas for the compiler library
COPY --from=builder /app/veex-build/internal/schema ./internal/schema
# Copy templates for the studio
COPY --from=builder /app/veex-platform/veex-templates ./veex-templates

# Expose the API port
ENV PORT=8080
ENV STORAGE_DIR=/app/data/artifacts
ENV TEMPLATES_DIR=/app/veex-templates

# Define volume for persistent storage
VOLUME ["/app/data"]

EXPOSE 8080

# Run the platform
ENTRYPOINT ["veex-platform"]
