# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=javsp
BINARY_UNIX=$(BINARY_NAME)_unix

# Build info
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# Linker flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -w -s"

# Build targets
.PHONY: all build clean test test-unit test-integration test-real test-real-quick test-network test-benchmark test-all test-ci-safe coverage lint format help install deps

all: clean deps test build

# Build the binary
build:
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v ./cmd/javsp

# Build for different platforms
build-all: build-linux build-darwin build-windows

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 ./cmd/javsp
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/javsp

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 ./cmd/javsp
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 ./cmd/javsp

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe ./cmd/javsp

# Install dependencies
deps:
	$(GOMOD) tidy
	$(GOMOD) download
	$(GOMOD) verify

# Run all tests (unit + integration)
test: test-unit test-integration

# Run unit tests only
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -race -timeout=30s -tags=unit ./...

# Run integration tests only  
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -race -timeout=60s -tags=integration ./test/integration/...

# Run real site tests (requires internet connection)
test-real:
	@echo "Running real site integration tests..."
	@echo "Warning: This will make real HTTP requests to external websites"
	$(GOTEST) -v -race -timeout=300s -tags="integration && real_sites" ./test/integration/...

# Run quick real site tests (faster version)
test-real-quick:
	@echo "Running quick real site tests..."
	@echo "Warning: This will make real HTTP requests to external websites"
	$(GOTEST) -v -race -timeout=120s -tags="integration && real_sites" -run="TestQuickRealSites" ./test/integration/...

# Run network resilience tests
test-network:
	@echo "Running network resilience tests..."
	$(GOTEST) -v -race -timeout=180s -tags="integration && real_sites" -run="TestNetwork.*|.*Resilience.*" ./test/integration/...

# Run benchmark tests only
test-benchmark:
	@echo "Running benchmark tests..."
	$(GOTEST) -v -run=^$$ -bench=. -benchmem -timeout=300s -tags=benchmark ./test/benchmark/...

# Run all tests including benchmarks and real sites
test-all: test-unit test-integration test-real test-benchmark

# Run all tests excluding real site tests (for CI)
test-ci-safe: test-unit test-integration test-benchmark

# Run tests with coverage (unit tests only by default)
coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic -tags=unit ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	$(GOCMD) tool cover -func=coverage.out

# Run coverage for all test types
coverage-full:
	@echo "Running full coverage analysis..."
	$(GOTEST) -v -race -coverprofile=coverage-unit.out -covermode=atomic -tags=unit ./...
	$(GOTEST) -v -race -coverprofile=coverage-integration.out -covermode=atomic -tags=integration ./test/integration/...
	echo "mode: atomic" > coverage-full.out
	grep -h -v "mode: atomic" coverage-unit.out coverage-integration.out >> coverage-full.out
	$(GOCMD) tool cover -html=coverage-full.out -o coverage-full.html
	$(GOCMD) tool cover -func=coverage-full.out

# Run benchmarks (alias for test-benchmark)
bench: test-benchmark

# Run benchmarks and save results
bench-save:
	@echo "Running benchmarks and saving results..."
	$(GOTEST) -run=^$$ -bench=. -benchmem -timeout=300s -tags=benchmark ./test/benchmark/... | tee benchmark-results.txt

# Lint the code
lint:
	golangci-lint run

# Format the code
format:
	gofmt -s -w .
	goimports -w .

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f coverage*.out coverage*.html
	rm -f benchmark-results.txt

# Install the binary
install: build
	sudo cp $(BINARY_NAME) /usr/local/bin/

# Development setup
setup:
	@echo "Setting up development environment..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development setup complete!"

# Run in development mode
dev: format lint test build
	./$(BINARY_NAME) --version

# Docker targets
docker-build:
	docker build -t javsp-go:$(VERSION) .

# Quick test targets for CI/CD
test-quick:
	@echo "Running quick tests (unit only, no race detection)..."
	$(GOTEST) -timeout=15s -tags=unit ./...

test-ci: test-unit
	@echo "Running CI test suite..."

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  all          - Clean, install deps, test, and build"
	@echo "  build        - Build the binary"
	@echo "  build-all    - Build for all platforms"
	@echo "  clean        - Clean build artifacts"
	@echo ""
	@echo "Test targets:"
	@echo "  test         - Run unit + integration tests"
	@echo "  test-unit    - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-real    - Run real site integration tests (requires internet)"
	@echo "  test-real-quick - Run quick real site tests (faster version)"
	@echo "  test-network - Run network resilience tests"
	@echo "  test-benchmark   - Run benchmark tests only"
	@echo "  test-all     - Run all tests including real sites and benchmarks"
	@echo "  test-ci-safe - Run all tests except real site tests (for CI)"
	@echo "  test-quick   - Run quick unit tests (no race detection)"
	@echo "  test-ci      - Run CI test suite"
	@echo ""
	@echo "Coverage targets:"
	@echo "  coverage     - Run tests with coverage (unit only)"
	@echo "  coverage-full - Run coverage for all test types"
	@echo ""
	@echo "Benchmark targets:"
	@echo "  bench        - Run benchmarks"
	@echo "  bench-save   - Run benchmarks and save results"
	@echo ""
	@echo "Development targets:"
	@echo "  deps         - Install dependencies"
	@echo "  lint         - Run linter"
	@echo "  format       - Format code"
	@echo "  setup        - Setup development environment"
	@echo "  dev          - Development workflow (format, lint, test, build)"
	@echo ""
	@echo "Other targets:"
	@echo "  install      - Install binary to /usr/local/bin"
	@echo "  help         - Show this help"