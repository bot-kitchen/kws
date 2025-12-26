.PHONY: all build run clean fmt test tidy css css-dev css-watch dev

# CSS build settings
CSS_INPUT := web/static/css/input.css
CSS_OUTPUT := web/static/css/styles.css
BUN := $(shell which bun)

# Default target
all: fmt build

# Build production CSS (minified)
css:
	@echo "Building production CSS..."
	@$(BUN) run build

# Build development CSS
css-dev:
	@$(BUN)x tailwindcss -i $(CSS_INPUT) -o $(CSS_OUTPUT)

# Watch CSS files for changes
css-watch:
	@echo "Watching CSS files for changes..."
	@$(BUN) run watch

# Build the application (includes CSS)
build: css
	go build -o bin/kws ./cmd/kws

# Run the application
run: build
	@mkdir -p log
	./bin/kws serve

# Development mode with CSS watch
dev: css-dev
	@echo "Starting development mode..."
	@trap 'kill 0' EXIT; \
	$(MAKE) css-watch & \
	go run cmd/kws/main.go serve --dev

# Format code
fmt:
	go fmt ./...

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/ log/

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
