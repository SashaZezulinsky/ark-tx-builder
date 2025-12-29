.PHONY: all test build clean lint fmt vet install-tools

# Variables
BINARY_NAME=ark-tx-builder
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"

all: test build

# Install development tools
install-tools:
	@echo "Installing development tools..."
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed successfully"

# Run tests
test:
	@echo "Running tests..."
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Tests completed successfully"

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	$(GO) test $(GOFLAGS) -race -v ./...

# Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build binary
build:
	@echo "Building binary..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)
	@echo "Binary built: bin/$(BINARY_NAME)"

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	@echo "Code formatted successfully"

# Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...
	@echo "Vet completed successfully"

# Run linter
lint: install-tools
	@echo "Running linter..."
	golangci-lint run ./...
	@echo "Linting completed successfully"

# Run all checks
check: fmt vet lint test
	@echo "All checks passed!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	$(GO) clean
	@echo "Clean completed"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	@echo "Dependencies downloaded"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GO) mod tidy
	@echo "Dependencies tidied"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

help:
	@echo "Available targets:"
	@echo "  all            - Run tests and build (default)"
	@echo "  test           - Run all tests"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  test-coverage  - Run tests and generate coverage report"
	@echo "  build          - Build the binary"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format code"
	@echo "  vet            - Run go vet"
	@echo "  check          - Run all checks (fmt, vet, lint, test)"
	@echo "  clean          - Remove build artifacts"
	@echo "  deps           - Download dependencies"
	@echo "  tidy           - Tidy dependencies"
	@echo "  bench          - Run benchmarks"
	@echo "  install-tools  - Install development tools"
	@echo "  help           - Show this help message"
