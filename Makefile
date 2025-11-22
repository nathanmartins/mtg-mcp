# Makefile for MTG Commander MCP Server
# Compatible with MacOS and Linux

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
BINARY_NAME=mtg-mcp
COVERAGE_FILE=coverage.out

# Detect OS for cross-platform compatibility
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
    OS := darwin
else
    OS := linux
endif

# Build flags
LDFLAGS=-ldflags="-w -s"

.PHONY: all build test test-unit test-e2e test-coverage clean fmt lint help install deps tidy

# Default target
all: fmt lint test-unit build

## help: Display this help message
help:
	@echo "MTG Commander MCP Server - Development Commands"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':' | sed 's/^/  /'

## build: Build the binary
build:
	@echo "Building binary for $(OS)..."
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) -v

## test-unit: Run unit tests only (fast)
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -short -v ./...

## test-e2e: Run end-to-end tests (requires API access)
test-e2e:
	@echo "Running E2E tests..."
	$(GOTEST) -v -run E2E -timeout 5m ./...

## test: Run all tests (unit + E2E)
test:
	@echo "Running all tests..."
	$(GOTEST) -v -timeout 5m ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=$(COVERAGE_FILE) -covermode=atomic -v ./...
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

## test-coverage-cli: Run tests with coverage (terminal output only)
test-coverage-cli:
	@echo "Running tests with coverage..."
	$(GOTEST) -short -coverprofile=$(COVERAGE_FILE) -covermode=atomic -v ./...
	@echo "Coverage summary:"
	$(GOCMD) tool cover -func=$(COVERAGE_FILE)

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	$(GOTEST) -short -race -v ./...

## bench: Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

## lint: Run linters
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...
	@echo "Running markdown linter..."
	-markdownlint *.md --fix 2>/dev/null || echo "markdownlint not found, skipping..."
	@echo "Running yaml linter..."
	-yamllint .github/workflows/*.yaml 2>/dev/null || echo "yamllint not found, skipping..."

## clean: Clean build artifacts and test cache
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE)
	rm -f coverage.html
	rm -rf *.log

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) -v ./...

## tidy: Tidy and verify go.mod
tidy:
	@echo "Tidying go.mod..."
	$(GOMOD) tidy
	$(GOMOD) verify

## install: Install the binary
install: build
	@echo "Installing binary..."
	cp $(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

## run: Run the application
run: build
	@echo "Running application..."
	./$(BINARY_NAME)

## check: Run all checks (fmt, lint, test)
check: fmt lint test-unit
	@echo "All checks passed!"

## ci: Run CI pipeline locally
ci: deps tidy fmt lint test-unit build
	@echo "CI pipeline completed successfully!"
