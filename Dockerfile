# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build sentientd
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sentientd ./cmd

# Runtime stage
FROM alpine:3.22 AS runtime

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 sentientd && \
    adduser -D -u 1000 -G sentientd sentientd

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/sentientd /app/sentientd

# Create tmp directory for writable filesystem
RUN mkdir -p /tmp && chown -R sentientd:sentientd /tmp

# Switch to non-root user
USER sentientd

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/sentientd"]
