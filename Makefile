# App-Store-Connect-CLI Makefile

# Variables
BINARY_NAME := asc
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Go variables
GO := go
GOMOD := go.mod
GOBIN := $(shell $(GO) env GOPATH)/bin
GOLANGCI_LINT_TIMEOUT ?= 5m
INSTALL_PREFIX ?= /usr/local/bin

# Directories
SRC_DIR := .
BUILD_DIR := build
DIST_DIR := dist

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
BLUE := \033[0;34m
NC := \033[0m # No Color

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build: $(BINARY_NAME)
	@echo "$(GREEN)✓ Build complete: $(BINARY_NAME)$(NC)"

$(BINARY_NAME): $(GOMOD)
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	@echo "$(BLUE)Building for multiple platforms...$(NC)"
	$(GO) run github.com/goreleaser/nfpm/v2@latest --config .nfpm.yaml --packer deb --packer rpm --packer apk --packer tarball

# Build with debug symbols
.PHONY: build-debug
build-debug:
	$(GO) build -gcflags="all=-N -l" -o $(BINARY_NAME)-debug .

# Run tests
.PHONY: test
test:
	@echo "$(BLUE)Running tests...$(NC)"
	ASC_BYPASS_KEYCHAIN=1 $(GO) test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	ASC_BYPASS_KEYCHAIN=1 $(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

# Run integration tests (opt-in)
.PHONY: test-integration
test-integration:
	@echo "$(BLUE)Running integration tests (requires ASC_* env vars)...$(NC)"
	$(GO) test -tags=integration -v ./internal/asc -run Integration

# Lint the code
.PHONY: lint
lint:
	@echo "$(BLUE)Linting code...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=$(GOLANGCI_LINT_TIMEOUT) ./...; \
	else \
		echo "$(YELLOW)golangci-lint not found; falling back to 'go vet ./...'.$(NC)"; \
		echo "$(YELLOW)Install with: make tools (or: $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)$(NC)"; \
		$(GO) vet ./...; \
	fi

# Format code
.PHONY: format
format:
	@echo "$(BLUE)Formatting code...$(NC)"
	@if ! command -v gofumpt >/dev/null 2>&1; then \
		echo "$(YELLOW)gofumpt not found; install with: make tools (or: $(GO) install mvdan.cc/gofumpt@latest)$(NC)"; \
		exit 1; \
	fi
	$(GO) fmt ./...
	gofumpt -w .

.PHONY: format-check
format-check:
	@echo "$(BLUE)Checking formatting (no writes)...$(NC)"
	@if ! command -v gofumpt >/dev/null 2>&1; then \
		echo "$(YELLOW)gofumpt not found; install with: make tools (or: $(GO) install mvdan.cc/gofumpt@latest)$(NC)"; \
		exit 1; \
	fi
	@unformatted_gofmt="$$(gofmt -l .)"; \
	unformatted_gofumpt="$$(gofumpt -l .)"; \
	if [ -n "$$unformatted_gofmt" ] || [ -n "$$unformatted_gofumpt" ]; then \
		echo "Formatting issues detected."; \
		if [ -n "$$unformatted_gofmt" ]; then \
			echo "gofmt:"; \
			echo "$$unformatted_gofmt"; \
		fi; \
		if [ -n "$$unformatted_gofumpt" ]; then \
			echo "gofumpt:"; \
			echo "$$unformatted_gofumpt"; \
		fi; \
		exit 1; \
	fi

# Install dev tools
.PHONY: tools
tools:
	@echo "$(BLUE)Installing dev tools...$(NC)"
	$(GO) install mvdan.cc/gofumpt@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)✓ Tools installed$(NC)"
	@echo "$(YELLOW)Make sure '$(GOBIN)' is on your PATH$(NC)"

# Install local git hooks
.PHONY: install-hooks
install-hooks:
	@echo "$(BLUE)Installing git hooks...$(NC)"
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "$(GREEN)✓ Hooks installed (core.hooksPath=.githooks)$(NC)"

# Install dependencies
.PHONY: deps
deps:
	@echo "$(BLUE)Installing dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod tidy

# Update dependencies
.PHONY: update-deps
update-deps:
	@echo "$(BLUE)Updating dependencies...$(NC)"
	$(GO) get -u ./...
	$(GO) mod tidy

# Update OpenAPI index
.PHONY: update-openapi
update-openapi:
	@echo "$(BLUE)Updating OpenAPI paths index...$(NC)"
	python3 scripts/update-openapi-index.py

.PHONY: update-schema-index
update-schema-index:
	@echo "$(BLUE)Updating schema index...$(NC)"
	python3 scripts/generate-schema-index.py

# Generate docs/COMMANDS.md from live CLI help
.PHONY: generate-command-docs
generate-command-docs:
	@echo "$(BLUE)Generating command docs...$(NC)"
	python3 ./scripts/generate-command-docs.py

# Validate docs command lists against live CLI output
.PHONY: check-command-docs
check-command-docs:
	@echo "$(BLUE)Checking command docs sync...$(NC)"
	python3 ./scripts/generate-command-docs.py --check
	python3 ./scripts/check-commands-docs.py

.PHONY: check-wall-of-apps
check-wall-of-apps:
	@echo "$(BLUE)Checking Wall of Apps source...$(NC)"
	$(GO) test ./internal/cli/apps -run TestCommunityWallSourceFileIsCanonical -count=1

# Clean build artifacts
.PHONY: clean
clean:
	@echo "$(BLUE)Cleaning...$(NC)"
	rm -f $(BINARY_NAME) $(BINARY_NAME)-debug
	rm -rf $(BUILD_DIR) $(DIST_DIR)
	rm -f coverage.out coverage.html

# Install the binary
.PHONY: install
install: build
	@echo "$(BLUE)Installing to $(INSTALL_PREFIX)...$(NC)"
	install -d $(INSTALL_PREFIX)
	install -m 755 $(BINARY_NAME) $(INSTALL_PREFIX)/$(BINARY_NAME)

# Uninstall the binary
.PHONY: uninstall
uninstall:
	@echo "$(BLUE)Uninstalling...$(NC)"
	rm -f $(INSTALL_PREFIX)/$(BINARY_NAME)

# Run the CLI locally
.PHONY: run
run: build
	@echo "$(BLUE)Running locally...$(NC)"
	./$(BINARY_NAME) --help

# Create a release
.PHONY: release
release: clean
	@echo "$(BLUE)Creating release...$(NC)"
	@echo "$(YELLOW)Note: Use GitHub Actions for releases$(NC)"

# Show help
.PHONY: help
help:
	@echo ""
	@echo "$(GREEN)App-Store-Connect-CLI$(NC) - Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  build-all      Build for multiple platforms"
	@echo "  build-debug    Build with debug symbols"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage"
	@echo "  test-integration  Run opt-in integration tests"
	@echo "  lint           Lint the code"
	@echo "  format         Format code"
	@echo "  format-check   Check formatting without writing files"
	@echo "  tools          Install dev tools"
	@echo "  install-hooks  Install local git hooks"
	@echo "  deps           Install dependencies"
	@echo "  update-deps    Update dependencies"
	@echo "  update-openapi Update OpenAPI paths index"
	@echo "  generate-command-docs Generate docs/COMMANDS.md from live CLI help"
	@echo "  check-command-docs Validate docs command lists against live CLI help"
	@echo "  clean          Clean build artifacts"
	@echo "  install        Install binary"
	@echo "  uninstall      Uninstall binary"
	@echo "  run            Run CLI locally"
	@echo "  help           Show this help"
	@echo ""

# Development shortcuts
.PHONY: dev
dev: format lint test build
	@echo "$(GREEN)✓ Ready for development!$(NC)"

# Check for security vulnerabilities
.PHONY: security
security:
	@echo "$(BLUE)Checking for security vulnerabilities...$(NC)"
	@which gosec > /dev/null 2>&1 && \
		gosec ./... || \
		echo "$(YELLOW)Install gosec for security checks$(NC)"
