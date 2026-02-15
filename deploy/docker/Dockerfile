# Build stage
FROM golang:1.25.7-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with version info
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
    -o pfm \
    ./cmd/pfm

# Runtime stage
FROM alpine:3.23

# Install runtime dependencies and create non-root user
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 1000 pfm && \
    adduser -D -u 1000 -G pfm pfm

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/pfm .

# Set ownership
RUN chown pfm:pfm /app/pfm

# Switch to non-root user
USER pfm

# Expose port (configurable via env)
EXPOSE 8080

# Run the binary
CMD ["/app/pfm"]
