# zepctl Makefile

# Build variables
BINARY_NAME := zepctl
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X github.com/getzep/zepctl/internal/cli.version=$(VERSION) -X github.com/getzep/zepctl/internal/cli.commit=$(COMMIT) -X github.com/getzep/zepctl/internal/cli.date=$(DATE)"

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := gofumpt
GOLINT := golangci-lint

# Paths
CMD_PATH := ./cmd/zepctl
BUILD_DIR := build
DIST_DIR := dist

.PHONY: all build clean test lint fmt tidy help

## Default target
all: lint test build

## Build the binary for the current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

## Build for all target platforms
build-all: build-linux-amd64 build-linux-arm64 build-darwin-arm64

## Build for Linux amd64
build-linux-amd64:
	@echo "Building for linux/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)

## Build for Linux arm64
build-linux-arm64:
	@echo "Building for linux/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)

## Build for macOS arm64
build-darwin-arm64:
	@echo "Building for darwin/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)

## Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

## Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## Run linter
lint:
	@echo "Running linter..."
	$(GOLINT) run ./...

## Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -w .

## Check formatting
fmt-check:
	@echo "Checking code formatting..."
	@test -z "$$($(GOFMT) -l .)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)

## Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

## Verify dependencies
verify:
	@echo "Verifying dependencies..."
	$(GOMOD) verify

## Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.out coverage.html

## Install the binary locally
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

## Run GoReleaser in snapshot mode (for testing)
release-snapshot:
	@echo "Creating snapshot release..."
	goreleaser release --snapshot --clean

## Run GoReleaser (requires GITHUB_TOKEN)
release:
	@echo "Creating release..."
	goreleaser release --clean

## Display help
help:
	@echo "zepctl Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  all              Run lint, test, and build (default)"
	@echo "  build            Build for current platform"
	@echo "  build-all        Build for all target platforms"
	@echo "  test             Run tests"
	@echo "  test-coverage    Run tests with coverage report"
	@echo "  lint             Run linter"
	@echo "  fmt              Format code"
	@echo "  fmt-check        Check code formatting"
	@echo "  tidy             Tidy dependencies"
	@echo "  verify           Verify dependencies"
	@echo "  clean            Clean build artifacts"
	@echo "  install          Install binary locally"
	@echo "  release-snapshot Test release with goreleaser"
	@echo "  release          Create release with goreleaser"
	@echo "  help             Display this help"
