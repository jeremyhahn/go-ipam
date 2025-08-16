# IPAM-Go Makefile

.PHONY: build clean test test-race test-cli test-integration bench install lint fmt vet deps help

# Build variables
BINARY_NAME=ipam
BUILD_DIR=./
GO_FILES=$(shell find . -type f -name '*.go' -not -path './vendor/*')

# Default target
all: build

## Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) $(BUILD_DIR)

## Clean build artifacts and test data
clean:
	@echo "Cleaning..."
	@echo "Stopping any running IPAM processes..."
	@-pkill -9 -f "ipam" 2>/dev/null || true
	@sleep 1
	@rm -f $(BINARY_NAME)
	@rm -rf ipam-data/ *-data/ *.db *.log
	@rm -rf *-cluster/ cluster-config-*/ stop-cluster.sh test-db
	@go clean -testcache

## Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

## Run unit tests
test:
	@echo "Running unit tests..."
	@go test -v ./api ./pkg/ipam ./pkg/store

## Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	@go test -race -v ./api ./pkg/ipam ./pkg/store

## Run CLI tests (sequential to avoid state issues)
test-cli:
	@echo "Running CLI tests..."
	@go test -v -p 1 -tags=cli ./cmd

## Run integration tests
test-integration: build
	@echo "Running integration tests..."
	@go test -v -tags=integration . -timeout 10m

## Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem -benchtime=10s ./pkg/ipam

## Generate test coverage
coverage:
	@echo "Generating test coverage..."
	@go test -coverprofile=coverage.out ./api ./pkg/ipam ./pkg/store
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

## Vet code
vet:
	@echo "Vetting code..."
	@go vet ./...

## Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	@sudo mv $(BINARY_NAME) /usr/local/bin/

## Initialize a standalone IPAM instance
init-standalone:
	@echo "Initializing standalone IPAM..."
	@./$(BINARY_NAME) network add "192.168.1.0/24" -d "Default test network"

## Initialize a single-node cluster (for local testing)
init-cluster:
	@echo "Initializing single-node cluster..."
	@./$(BINARY_NAME) cluster init --node-id 1 --cluster-id 100 --raft-addr localhost:5001 --single-node

## Start a 3-node cluster
cluster-3-node: build
	@echo "Starting 3-node cluster..."
	@./scripts/3-node-cluster.sh


## Start API server (standalone)
server:
	@echo "Starting IPAM API server..."
	@./$(BINARY_NAME) server

## Start API server (cluster)
server-cluster:
	@echo "Starting IPAM cluster server..."
	@./$(BINARY_NAME) server --cluster --config ipam-cluster-data/cluster.json

## Show help
help:
	@echo "Available targets:"
	@echo "  build              Build the binary"
	@echo "  clean              Clean build artifacts and test data"
	@echo "  deps               Install dependencies"
	@echo "  test               Run unit tests"
	@echo "  test-race          Run tests with race detection"
	@echo "  test-cli           Run CLI tests"
	@echo "  test-integration   Run integration tests"
	@echo ""
	@echo "  bench              Run benchmarks"
	@echo "  coverage           Generate test coverage report"
	@echo "  fmt                Format code"
	@echo "  vet                Vet code"
	@echo "  lint               Lint code (requires golangci-lint)"
	@echo "  install            Install binary to /usr/local/bin"
	@echo "  init-standalone    Initialize standalone IPAM"
	@echo "  init-cluster       Initialize single-node cluster (for testing)"
	@echo "  cluster-3-node     Start 3-node cluster using script"
	@echo "  server             Start API server (standalone)"
	@echo "  server-cluster     Start API server (single-node cluster)"
	@echo "  help               Show this help message"
