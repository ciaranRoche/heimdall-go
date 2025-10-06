# Include bingo-managed tool binaries
include .bingo/Variables.mk

.PHONY: help test test-coverage build clean fmt lint vet examples

# Default target
help:
	@echo "Heimdall Go - Makefile Commands"
	@echo ""
	@echo "  make test             - Run all tests"
	@echo "  make test-coverage    - Run tests with coverage report"
	@echo "  make build            - Build example applications"
	@echo "  make clean            - Remove build artifacts"
	@echo "  make fmt              - Format code"
	@echo "  make lint             - Run linter (using bingo-managed golangci-lint)"
	@echo "  make vet              - Run go vet"
	@echo "  make examples         - Build all examples"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build example applications
build: examples

examples:
	@echo "Building examples..."
	@mkdir -p bin
	go build -o bin/basic-example ./examples/basic
	@echo "Examples built in bin/"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache -testcache

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter (bingo auto-installs correct version if needed)
lint: $(GOLANGCI_LINT)
	@echo "Running linter..."
	@$(GOLANGCI_LINT) run ./...

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run all checks before commit
precommit: fmt vet lint test
	@echo "Pre-commit checks passed!"

# Update dependencies
deps:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# Verify dependencies
verify:
	@echo "Verifying dependencies..."
	go mod verify
	go mod tidy
	git diff --exit-code go.mod go.sum
