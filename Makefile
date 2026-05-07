.PHONY: all build test test-quick fmt vet install clean release build-linux build-macos-arm run-sample setup deps tidy help

# Build variables
BINARY_NAME := logpulse
VERSION := dev
BUILD_TIME := 2026-05-07
LDFLAGS := -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}"

# Go commands
GO := /usr/local/bin/go
GOBUILD := $(GO) build
GOTEST := $(GO) test
GOFMT := $(GO) fmt
GOVET := $(GO) vet

# Directories
BUILD_DIR := ./bin
CONFIG_DIR := ./config
LOG_DIR := ./log

# Default target
all: build

# Build the binary for the current platform.
build:
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/logpulse
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME) (version $(VERSION))"

# Run tests with coverage.
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run tests without coverage (faster).
test-quick:
	$(GOTEST) -v ./...

# Format code.
fmt:
	$(GOFMT) ./...

# Run linter.
vet:
	$(GOVET) ./...

# Install the binary to GOPATH/bin.
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(shell go env GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to: $(shell go env GOPATH)/bin/$(BINARY_NAME)"

# Cross-compile for multiple platforms.
release:
	@mkdir -p $(BUILD_DIR)/releases
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			ext=""; \
			if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch $(GOBUILD) $(LDFLAGS) \
				-o $(BUILD_DIR)/releases/$(BINARY_NAME)-$$os-$$arch$$ext \
				./cmd/logpulse; \
			echo "Built: $(BINARY_NAME)-$$os-$$arch$$ext"; \
		done; \
	done

# Build for Linux (common for server deployment).
build-linux:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 \
		./cmd/logpulse

# Build for macOS (Apple Silicon).
build-macos-arm:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 \
		./cmd/logpulse

# Clean build artifacts.
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

# Run the tool locally against a sample log.
run-sample:
	@mkdir -p $(LOG_DIR)
	$(GO) run ./cmd/logpulse -config $(CONFIG_DIR)/config.yaml

# Create required directories.
setup:
	mkdir -p $(BUILD_DIR) $(CONFIG_DIR) $(LOG_DIR)

# Verify dependencies.
deps:
	$(GO) mod download
	$(GO) mod verify

# Tidy dependencies.
tidy:
	$(GO) mod tidy

# Show help.
help:
	@echo "logpulse Makefile targets:"
	@echo ""
	@echo "  make build        - Build binary for current platform"
	@echo "  make test         - Run tests with coverage"
	@echo "  make test-quick   - Run tests without coverage"
	@echo "  make fmt          - Format code"
	@echo "  make vet          - Run linter"
	@echo "  make install      - Install binary to GOPATH/bin"
	@echo "  make release      - Cross-compile for all platforms"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make deps         - Download and verify dependencies"
	@echo "  make tidy         - Tidy go.mod"
	@echo "  make setup        - Create required directories"
	@echo ""
