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
GOFUMPT_VERSION ?= v0.9.2
GOLANGCI_LINT_VERSION ?= v1.64.8

# Directories
SRC_DIR := .
BUILD_DIR := build
DIST_DIR := dist
RELEASE_DIR := release

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
build:
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "$(GREEN)✓ Build complete: $(BINARY_NAME)$(NC)"

# Build release-style binaries for supported platforms
.PHONY: build-all
build-all: clean
	@echo "$(BLUE)Building for multiple platforms...$(NC)"
	@mkdir -p $(RELEASE_DIR)
	@for target in "darwin amd64 macOS" "darwin arm64 macOS" "linux amd64 linux" "linux arm64 linux" "windows amd64 windows"; do \
		set -- $$target; \
		os="$$1"; arch="$$2"; label="$$3"; suffix=""; \
		if [ "$$os" = "windows" ]; then suffix=".exe"; fi; \
		echo "Building $$label/$$arch..."; \
		GOOS="$$os" GOARCH="$$arch" $(GO) build -ldflags "$(LDFLAGS)" -o "$(RELEASE_DIR)/$(BINARY_NAME)_$(VERSION)_$${label}_$${arch}$${suffix}" .; \
	done
	@echo "$(GREEN)✓ Release binaries written to $(RELEASE_DIR)/$(NC)"

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
	$(GO) install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
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

.PHONY: studio-frontend-install
studio-frontend-install:
	@echo "$(BLUE)Installing ASC Studio frontend dependencies...$(NC)"
	cd apps/studio/frontend && npm install

.PHONY: studio-frontend-test
studio-frontend-test:
	@echo "$(BLUE)Running ASC Studio frontend tests...$(NC)"
	cd apps/studio/frontend && npm run test -- --run

.PHONY: studio-frontend-build
studio-frontend-build:
	@echo "$(BLUE)Building ASC Studio frontend assets...$(NC)"
	cd apps/studio/frontend && npm run build

.PHONY: studio-test
studio-test:
	@echo "$(BLUE)Running ASC Studio Go tests...$(NC)"
	$(GO) test ./apps/studio/...

.PHONY: studio-build
studio-build:
	@echo "$(BLUE)Building ASC Studio bootstrap binary...$(NC)"
	$(GO) build ./apps/studio

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

.PHONY: check-repo-docs
check-repo-docs:
	@echo "$(BLUE)Checking repository docs links...$(NC)"
	python3 ./scripts/check_repo_docs.py

.PHONY: check-website-docs
check-website-docs:
	@echo "$(BLUE)Checking Mintlify website docs...$(NC)"
	python3 ./scripts/check_website_docs.py
	python3 ./scripts/check_website_commands.py

.PHONY: check-release-docs
check-release-docs:
	@if [ -z "$(VERSION)" ]; then \
		echo "$(YELLOW)VERSION is required. Example: make check-release-docs VERSION=0.36.4$(NC)"; \
		exit 1; \
	fi
	@echo "$(BLUE)Checking release docs for $(VERSION)...$(NC)"
	python3 ./scripts/check_release_docs.py "$(VERSION)"

.PHONY: check-docs
check-docs: check-command-docs check-repo-docs check-website-docs

.PHONY: check-wall-of-apps
check-wall-of-apps:
	@echo "$(BLUE)Checking Wall of Apps source...$(NC)"
	$(GO) test ./internal/cli/apps -run TestCommunityWallSourceFileIsCanonical -count=1

# Clean build artifacts
.PHONY: clean
clean:
	@echo "$(BLUE)Cleaning...$(NC)"
	rm -f $(BINARY_NAME) $(BINARY_NAME)-debug
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(RELEASE_DIR)
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
	@echo "  build-all      Build release binaries for supported platforms"
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
	@echo "  studio-frontend-install  Install ASC Studio frontend dependencies"
	@echo "  studio-frontend-test  Run ASC Studio frontend tests"
	@echo "  studio-frontend-build  Build ASC Studio frontend assets"
	@echo "  studio-test    Run ASC Studio Go tests"
	@echo "  studio-build   Build ASC Studio bootstrap binary"
	@echo "  update-deps    Update dependencies"
	@echo "  update-openapi Update OpenAPI paths index"
	@echo "  generate-command-docs Generate docs/COMMANDS.md from live CLI help"
	@echo "  check-command-docs Validate docs command lists against live CLI help"
	@echo "  check-repo-docs Validate local links in repository markdown docs"
	@echo "  check-website-docs Validate Mintlify website navigation, links, and CLI examples"
	@echo "  check-release-docs Validate website release docs for VERSION=<x.y.z>"
	@echo "  check-docs     Run all documentation checks"
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
