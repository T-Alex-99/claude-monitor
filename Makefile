.PHONY: build run clean dev

# Binary name
BINARY=claude-monitor

# Build flags
LDFLAGS=-s -w

# Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Run the application
run: build
	./$(BINARY)

# Run with custom port
run-port: build
	./$(BINARY) -port $(PORT)

# Development mode (build and run)
dev:
	go run .

# Clean build artifacts
clean:
	rm -f $(BINARY)

# Install dependencies
deps:
	go mod tidy

# Build for release (smaller binary)
release:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BINARY) .
	upx --best $(BINARY) 2>/dev/null || true

# Show help
help:
	@echo "Available targets:"
	@echo "  build   - Build the binary"
	@echo "  run     - Build and run"
	@echo "  dev     - Run in development mode"
	@echo "  clean   - Remove build artifacts"
	@echo "  deps    - Install dependencies"
	@echo "  release - Build optimized release binary"
