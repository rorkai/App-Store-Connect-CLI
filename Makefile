# App-Store-Connect-CLI Makefile

# Variables
BINARY_NAME := asc
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Keep local build metadata stable so unchanged builds can reuse Go's link cache.
# Release builds set their actual build timestamp in the release workflow.
DATE := $(shell git show -s --format=%cI HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
# Release builds strip symbol/DWARF tables (-s -w) and file-system paths
# (-trimpath) for smaller, reproducible binaries. Dev builds (`make build`)
# stay unstripped for debuggability.
RELEASE_LDFLAGS := -s -w $(LDFLAGS)
RELEASE_BUILD_FLAGS := -trimpath

# Go variables
GO := go
# CGO's external linker otherwise inherits the installed macOS SDK minimum.
override MACOSX_DEPLOYMENT_TARGET := 13.0
export MACOSX_DEPLOYMENT_TARGET
ifeq ($(shell $(GO) env GOHOSTOS),darwin)
# Include the target in Go's CGO cache key, preserving other compiler flags.
override CGO_CFLAGS := $(filter-out -mmacosx-version-min=%,$(or $(CGO_CFLAGS),-O2 -g)) -mmacosx-version-min=13.0
override CGO_LDFLAGS := $(filter-out -mmacosx-version-min=%,$(or $(CGO_LDFLAGS),-O2 -g)) -mmacosx-version-min=13.0
export CGO_CFLAGS CGO_LDFLAGS
endif
GOMOD := go.mod
GOBIN := $(shell $(GO) env GOPATH)/bin
GO_TOOLCHAIN_VERSION := $(shell $(GO) env GOVERSION)
GOFMT := $(shell $(GO) env GOROOT)/bin/gofmt
# Cold hosted runners can require more than ten minutes for the full-module lint.
GOLANGCI_LINT_TIMEOUT ?= 15m
INSTALL_PREFIX ?= /usr/local/bin
GOFUMPT_VERSION ?= v0.12.0
GOLANGCI_LINT_VERSION ?= v2.14.0
GOVULNCHECK_VERSION ?= v1.6.0

# Test environment
# Tests must not depend on the invoking shell or the developer's stored asc
# state. Unset every inherited ASC_* variable (except test-only switches) and
# DO_NOT_TRACK, bypass the keychain, and point ASC_CONFIG_PATH at a missing
# file in a fresh per-run directory that is removed afterwards: a temporary HOME
# alone is not enough, because the upward .asc/config.json search from a
# checkout inside the home directory reaches ~/.asc/config.json. The directory
# is read-only so a test that writes config without choosing its own path fails
# (root bypasses the mode, but the directory is still private to the run).
# Tests that need any of these inputs set them with t.Setenv. TEST_AFFECTED_BY
# and MAKEFLAGS are unset too, so tests that run make get the whole suite.
TEST_ENV_PASSTHROUGH := ASC_UPDATE_GOLDEN ASC_SIGNING_RUN_LIVE_TEST ASC_SIGNING_KEYCHAIN_INSTALL_LIVE_TEST
TEST_ENV = env $(foreach var,$(sort $(filter-out $(TEST_ENV_PASSTHROUGH),$(filter ASC_%,$(.VARIABLES)))),-u $(var)) -u DO_NOT_TRACK -u TEST_AFFECTED_BY -u MAKEFLAGS ASC_BYPASS_KEYCHAIN=1

# $(call run_isolated_tests,<go test arguments>[,<test command>])
# The test command defaults to `go test`.
# The EXIT trap removes the directory however the run ends, including Ctrl-C.
define run_isolated_tests
	@config_dir="$$(mktemp -d "$${TMPDIR:-/tmp}/asc-test-config.XXXXXX")" || exit 1; \
	trap 'chmod 700 "$$config_dir" 2>/dev/null; rm -rf "$$config_dir"' EXIT; \
	trap 'exit 130' INT; \
	trap 'exit 143' TERM; \
	chmod 500 "$$config_dir"; \
	echo "ASC_CONFIG_PATH=$$config_dir/config.json $(or $(2),$(GO) test) $(1)"; \
	$(TEST_ENV) ASC_CONFIG_PATH="$$config_dir/config.json" $(or $(2),$(GO) test) $(1); \
	status=$$?; \
	if [ -n "$$(ls -A "$$config_dir")" ]; then \
		echo "error: tests wrote to the shared test config directory $$config_dir; set ASC_CONFIG_PATH in the test instead" >&2; \
		[ $$status -ne 0 ] || status=1; \
	fi; \
	exit $$status
endef

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
	@mkdir -p "$(RELEASE_DIR)"
	@for target in "darwin amd64 macOS" "darwin arm64 macOS" "linux amd64 linux" "linux arm64 linux" "windows amd64 windows"; do \
		set -- $$target; \
		os="$$1"; arch="$$2"; label="$$3"; suffix=""; \
		if [ "$$os" = "windows" ]; then suffix=".exe"; fi; \
		if [ "$$os" = "darwin" ]; then cgo="1"; else cgo="0"; fi; \
		echo "Building $$label/$$arch..."; \
		CGO_ENABLED="$$cgo" GOOS="$$os" GOARCH="$$arch" $(GO) build $(RELEASE_BUILD_FLAGS) -ldflags "$(RELEASE_LDFLAGS)" -o "$(RELEASE_DIR)/$(BINARY_NAME)_$(VERSION)_$${label}_$${arch}$${suffix}" . || exit $$?; \
	done
	@echo "$(GREEN)✓ Release binaries written to $(RELEASE_DIR)/$(NC)"

# Build the same self-contained Intel/Apple Silicon binaries as macOS releases.
.PHONY: build-native-macos
build-native-macos: clean
	VERSION="$(VERSION)" RELEASE_DIR="$(RELEASE_DIR)" sh scripts/build_release_macos.sh

# Build with debug symbols
.PHONY: build-debug
build-debug:
	$(GO) build -gcflags="all=-N -l" -o $(BINARY_NAME)-debug .

# Per-run config isolation and high-volume filesystem logs make test-result
# caching costly. Disable result caching while retaining Go's compile cache.
# The slowest packages run as concurrent test shards beside the rest of ./...;
# TEST_JOBS sets the CPU budget (default: GOMAXPROCS, else the CPU count). It is
# a soft budget: it sizes shard and package fan-out, not a hard CPU limit.
# TEST_AFFECTED_BY=<Go files> tests only the packages those files reach.
TEST_JOBS ?=
LOCAL_TEST = GO="$(GO)" python3 scripts/go_test_shard.py local --split ./internal/cli/cmdtest --split ./internal/cli/web $(if $(TEST_JOBS),--jobs $(TEST_JOBS)) $(if $(TEST_AFFECTED_BY),--affected-by $(TEST_AFFECTED_BY)) --

# Run tests
.PHONY: test
test:
	@echo "$(BLUE)Running tests...$(NC)"
	$(call run_isolated_tests,-count=1 -v,$(LOCAL_TEST))

# Run the short test suite (used by the pre-commit hook)
.PHONY: test-short
test-short:
	@echo "$(BLUE)Running short tests...$(NC)"
	$(call run_isolated_tests,-count=1 -short,$(LOCAL_TEST))

# Run tests with parallel package compilation
# Defaults to GOMAXPROCS; set PARALLEL to override (e.g. PARALLEL=4)
.PHONY: test-parallel
test-parallel:
	@echo "$(BLUE)Running tests (parallel=$(or $(PARALLEL),auto))...$(NC)"
	$(call run_isolated_tests,-v -count=1 $(if $(PARALLEL),-p=$(PARALLEL)) ./...)

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	$(call run_isolated_tests,-coverprofile=coverage.out ./...)
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
		case "$$(golangci-lint version 2>&1)" in \
			*"has version $(GOLANGCI_LINT_VERSION:v%=%) built with $(GO_TOOLCHAIN_VERSION) "*) ;; \
			*) echo "$(YELLOW)golangci-lint does not match $(GOLANGCI_LINT_VERSION) built with $(GO_TOOLCHAIN_VERSION): $$(golangci-lint version 2>&1)$(NC)"; \
			   echo "$(YELLOW)Run: make tools$(NC)"; exit 1;; \
		esac; \
		GOLANGCI_LINT_CACHE="$(CURDIR)/.golangci-cache" golangci-lint run --timeout=$(GOLANGCI_LINT_TIMEOUT) ./...; \
	else \
		echo "$(YELLOW)golangci-lint not found; falling back to 'go vet ./...'.$(NC)"; \
		echo "$(YELLOW)Install with: make tools$(NC)"; \
		$(GO) vet ./...; \
	fi

# Format code
.PHONY: format
format: check-gofumpt
	@echo "$(BLUE)Formatting code...$(NC)"
	gofumpt -w .

.PHONY: check-gofumpt
check-gofumpt:
	@if ! command -v gofumpt >/dev/null 2>&1; then \
		echo "$(YELLOW)gofumpt not found; install with: make tools$(NC)"; \
		exit 1; \
	fi
	@if [ "$$(gofumpt --version)" != "$(GOFUMPT_VERSION) ($(GO_TOOLCHAIN_VERSION))" ]; then \
		echo "$(YELLOW)gofumpt $$(gofumpt --version) does not match $(GOFUMPT_VERSION) ($(GO_TOOLCHAIN_VERSION)); run: make tools$(NC)"; \
		exit 1; \
	fi

.PHONY: format-check
format-check: check-gofumpt
	@echo "$(BLUE)Checking formatting (no writes)...$(NC)"
	@unformatted_gofmt="$$($(GOFMT) -l .)"; \
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
	GOTOOLCHAIN=$(GO_TOOLCHAIN_VERSION) $(GO) install mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
	GOTOOLCHAIN=$(GO_TOOLCHAIN_VERSION) $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "$(GREEN)✓ Tools installed$(NC)"
	@echo "$(YELLOW)Make sure '$(GOBIN)' is on your PATH$(NC)"

.PHONY: install-govulncheck
install-govulncheck:
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

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

# Regenerate the client code in internal/asc from the OpenAPI snapshot and in
# internal/web from the web interface schema
.PHONY: generate-client
generate-client:
	@echo "$(BLUE)Generating API client code...$(NC)"
	$(GO) generate ./internal/asc ./internal/web

# Generate docs/COMMANDS.md from live CLI help
.PHONY: generate-command-docs
generate-command-docs:
	@echo "$(BLUE)Generating command docs...$(NC)"
	python3 ./scripts/generate-command-docs.py

# Validate docs command lists against live CLI output
.PHONY: check-command-docs
check-command-docs:
	@echo "$(BLUE)Checking command docs sync...$(NC)"
	python3 ./scripts/test_generate_command_docs.py
	python3 ./scripts/check-commands-docs.py --check-generated

.PHONY: check-repo-docs
check-repo-docs:
	@echo "$(BLUE)Checking repository docs links...$(NC)"
	python3 ./scripts/check_repo_docs.py

.PHONY: check-website-docs
check-website-docs:
	@echo "$(BLUE)Checking website docs...$(NC)"
	python3 ./scripts/check_website_docs.py
	python3 ./scripts/check_website_commands.py

.PHONY: check-agent-skills
check-agent-skills:
	@echo "$(BLUE)Checking repository agent skills...$(NC)"
	python3 ./scripts/test_check_agent_skills.py
	python3 ./scripts/check_agent_skills.py

.PHONY: check-openapi
check-openapi:
	@echo "$(BLUE)Checking OpenAPI generated artifacts...$(NC)"
	python3 ./scripts/test_update_openapi_index.py
	python3 ./scripts/test_generate_schema_index.py
	python3 ./scripts/update-openapi-index.py --check
	python3 ./scripts/generate-schema-index.py --check
	cd internal/asc && $(GO) run ../openapigen -check
	cd internal/web && $(GO) run ../openapigen -target web -check

.PHONY: check-docs
check-docs: check-repo-docs check-agent-skills check-openapi
	python3 ./scripts/test_generate_command_docs.py
	python3 ./scripts/test_check_docs_commands.py
	python3 ./scripts/check_website_docs.py
	python3 ./scripts/check_docs_commands.py

.PHONY: check-wall-of-apps
check-wall-of-apps:
	@echo "$(BLUE)Checking Wall of Apps source...$(NC)"
	$(GO) test ./internal/cli/apps -run TestCommunityWallSourceFileIsCanonical -count=1

# Clean build artifacts
.PHONY: clean
clean:
	@echo "$(BLUE)Cleaning...$(NC)"
	rm -f $(BINARY_NAME) $(BINARY_NAME)-debug
	rm -rf $(BUILD_DIR) $(DIST_DIR) "$(RELEASE_DIR)"
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

# Run the non-publishing checks used by the release rehearsal workflow
.PHONY: release-guardrails
release-guardrails:
	python3 scripts/test_build_macos_release.py
	python3 scripts/test_release_rehearsal.py
	python3 scripts/test_check_docs.py
	$(MAKE) format-check
	$(MAKE) check-docs
	$(MAKE) lint
	ASC_BYPASS_KEYCHAIN=1 $(MAKE) test

# Show help
.PHONY: help
help:
	@echo ""
	@echo "$(GREEN)App-Store-Connect-CLI$(NC) - Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  build-all      Build cross-platform rehearsal binaries"
	@echo "  build-native-macos Build self-contained macOS releases"
	@echo "  build-debug    Build with debug symbols"
	@echo "  test           Run tests (TEST_JOBS=<n> sets a soft CPU budget)"
	@echo "  test-short     Run the short test suite (TEST_JOBS=<n> sets a soft CPU budget)"
	@echo "  test-parallel  Run tests with optional package parallelism (PARALLEL=<n>)"
	@echo "  test-coverage  Run tests with coverage"
	@echo "  test-integration  Run opt-in integration tests"
	@echo "  lint           Lint the code"
	@echo "  format         Format code"
	@echo "  format-check   Check formatting without writing files"
	@echo "  tools          Install dev tools"
	@echo "  install-govulncheck Install the pinned vulnerability scanner"
	@echo "  install-hooks  Install local git hooks"
	@echo "  deps           Install dependencies"
	@echo "  update-deps    Update dependencies"
	@echo "  update-openapi Update OpenAPI paths index"
	@echo "  update-schema-index Update runtime schema index"
	@echo "  generate-client Regenerate API client code from the OpenAPI snapshot and the web interface schema"
	@echo "  generate-command-docs Generate docs/COMMANDS.md from live CLI help"
	@echo "  check-command-docs Validate docs command lists against live CLI help"
	@echo "  check-repo-docs Validate local links in repository markdown docs"
	@echo "  check-website-docs Validate website navigation, links, and CLI examples"
	@echo "  check-agent-skills Validate repository-scoped Codex skills"
	@echo "  check-openapi  Validate generated OpenAPI indexes and client code"
	@echo "  check-docs     Run all documentation checks"
	@echo "  release-guardrails Run non-publishing release checks"
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
	@if command -v gosec > /dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "$(YELLOW)Install gosec for security checks$(NC)"; \
	fi
