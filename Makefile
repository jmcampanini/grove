BUILD_DIR := build
BINARY := $(BUILD_DIR)/grove

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || printf 'unknown')
LDFLAGS := -ldflags "-X github.com/jmcampanini/grove/cmd.Version=$(VERSION)"

.DEFAULT_GOAL := help
.PHONY: help build test lint lint-fix fmt fmt-check tidy tidy-check version-check vuln check clean

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Compile the grove binary.
	mkdir -p $(BUILD_DIR)
	go build -trimpath -buildvcs=false $(LDFLAGS) -o $(BINARY) .

test: ## Run all tests uncached with the race detector.
	go test -count=1 -race ./...

lint: ## Run golangci-lint.
	go tool golangci-lint run ./...

lint-fix: ## Run golangci-lint with autofixes.
	go tool golangci-lint run --fix ./...

fmt: ## Format Go source files.
	go tool golangci-lint fmt

fmt-check: ## Verify formatting without changing files.
	go tool golangci-lint fmt --diff

tidy: ## Apply go mod tidy.
	go mod tidy

tidy-check: ## Check go.mod/go.sum without modifying files.
	go mod tidy -diff

version-check: build ## Verify the built binary reports the injected version.
	@case "$(VERSION)" in unknown|n/a|"") echo "degenerate version identity: '$(VERSION)'"; exit 1;; esac
	@out="$$($(BINARY) --version)"; \
	if [ "$$out" != "grove version $(VERSION)" ]; then \
		echo "version mismatch: got '$$out', want 'grove version $(VERSION)'"; \
		exit 1; \
	fi

vuln: ## Check dependencies and reachable code for known vulnerabilities.
	go tool govulncheck ./...

check: fmt-check tidy-check lint test build version-check vuln ## Run the complete local verification contract.

clean: ## Remove build artifacts, coverage files, and test cache.
	rm -rf $(BUILD_DIR) dist
	go clean -testcache
	rm -f *.out cover.* coverage.*
