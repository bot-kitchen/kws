# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o kws ./cmd/kws

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install CA certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /build/kws /app/kws

# Copy production config file
COPY config/config.prod.yaml /app/config/config.yaml

# Create non-root user
RUN adduser -D -u 1000 kws && \
    chown -R kws:kws /app

USER kws

EXPOSE 8080

ENTRYPOINT ["/app/kws"]
CMD ["serve"]
