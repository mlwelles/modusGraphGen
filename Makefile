SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

AUTO_INSTALL ?= false

# The generator's golden tests parse structs from the sibling movies project.
MOVIES_PROJECT ?= ../modusGraphMoviesProject

.PHONY: help build test check deps deps-go deps-test-data update-golden clean

.DEFAULT_GOAL := help

# =============================================================================
# User-facing targets
# =============================================================================

help: ## Show this help message
	@echo ""
	@echo "Environment Variables:"
	@echo "  MOVIES_PROJECT=<path>  Path to modusGraphMoviesProject (default: ../modusGraphMoviesProject)"
	@echo "  AUTO_INSTALL=true      Auto-install missing deps instead of printing instructions"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""

build: deps-go ## Build the modusGraphGen binary
	@echo "Building modusGraphGen …"
	go build -o bin/modusGraphGen .
	@echo "  bin/modusGraphGen built."

test: deps-go deps-test-data ## Run all tests
	@echo "Running tests …"
	go test ./...

check: deps-go ## Run go vet on all packages
	@echo "Running go vet …"
	go vet ./...

update-golden: deps-go deps-test-data ## Regenerate golden test files from movies project
	@echo "Updating golden files from $(MOVIES_PROJECT)/movies …"
	go test ./generator -run TestGenerate -update
	@echo "Golden files updated."

deps: deps-go deps-test-data ## Check all dependencies

clean: ## Remove build artifacts
	rm -rf bin/

# =============================================================================
# Internal targets
# =============================================================================

deps-go:
	@echo "Checking Go …"
	@if command -v go >/dev/null 2>&1; then \
		echo "  go: $$(go version)"; \
	else \
		if [ "$(AUTO_INSTALL)" = "true" ]; then \
			if [ "$$(uname)" = "Darwin" ]; then \
				echo "  go: not found — installing via Homebrew …"; \
				brew install go; \
				echo "  go: $$(go version)"; \
			else \
				echo "Error: go is not installed and AUTO_INSTALL is not supported for $$(uname)."; \
				echo "  Install Go from: https://go.dev/dl/"; \
				exit 1; \
			fi; \
		else \
			echo "Error: go is not installed."; \
			echo ""; \
			echo "  Install Go from: https://go.dev/dl/"; \
			if [ "$$(uname)" = "Darwin" ]; then \
				echo "  macOS: brew install go"; \
			fi; \
			echo ""; \
			echo "  Or re-run with AUTO_INSTALL=true to install automatically (macOS)."; \
			exit 1; \
		fi; \
	fi

deps-test-data: ## Check that modusGraphMoviesProject exists (needed for golden tests)
	@echo "Checking test data source …"
	@if [ -d "$(MOVIES_PROJECT)/movies" ]; then \
		echo "  movies project: $(MOVIES_PROJECT)/movies"; \
	else \
		echo "Error: $(MOVIES_PROJECT)/movies not found."; \
		echo "  The generator tests parse struct definitions from the movies project."; \
		echo ""; \
		echo "  Clone it: git clone https://github.com/mlwelles/modusGraphMoviesProject.git $(MOVIES_PROJECT)"; \
		exit 1; \
	fi
