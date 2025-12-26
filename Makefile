.PHONY: all build run clean fmt test tidy

# Default target
all: fmt build

# Build the application
build:
	go build -o bin/kws ./cmd/kws

# Run the application
run: build
	./bin/kws serve

# Format code
fmt:
	go fmt ./...

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Tidy dependencies
tidy:
	go mod tidy

# Generate CA certificates
ca-init:
	./scripts/ca/ca-setup.sh

# Docker compose up
up:
	docker compose up -d

# Docker compose down
down:
	docker compose down
