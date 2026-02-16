# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the DAG runner
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dag-runner ./cmd/dag-runner

# Runtime stage
FROM alpine:3.22

# Install kubectl for kubectl tool support
RUN apk add --no-cache ca-certificates kubectl

# Create non-root user
RUN addgroup -g 1000 dagrunner && \
    adduser -D -u 1000 -G dagrunner dagrunner

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/dag-runner /app/dag-runner

# Set ownership to non-root user
RUN chown dagrunner:dagrunner /app/dag-runner && \
    chmod 555 /app/dag-runner

# Create writable /tmp for runtime (required for some operations)
RUN mkdir -p /tmp && chown -R dagrunner:dagrunner /tmp

# Switch to non-root user
USER dagrunner

# Security labels
LABEL org.opencontainers.image.title="torvyn-dag-runner" \
      org.opencontainers.image.description="DAG task executor for sentientd" \
      org.opencontainers.image.authors="torvyn" \
      security.non-root=true \
      security.read-only-root-filesystem=true

ENTRYPOINT ["/app/dag-runner"]
