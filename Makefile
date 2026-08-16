BUILD_DIR := build
BINARY := $(BUILD_DIR)/grove
GOFMT_FILES := $(shell git ls-files '*.go')

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/jmcampanini/grove/cmd.Version=$(VERSION)"

.DEFAULT_GOAL := help
.PHONY: help build test lint lint-fix fmt fmt-check tidy tidy-check vuln check clean

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*##/ { printf "  %-16s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Compile the grove binary.
	mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BINARY) .

test: ## Run tests with the race detector.
	go test -race ./...

lint: ## Run golangci-lint.
	golangci-lint run ./...

lint-fix: ## Run golangci-lint with autofixes.
	golangci-lint run --fix ./...

fmt: ## Format tracked Go files.
	@if [ -n "$(GOFMT_FILES)" ]; then gofmt -w $(GOFMT_FILES); fi

fmt-check: ## Fail if tracked Go files need gofmt.
	@files="$$(gofmt -l $(GOFMT_FILES))"; \
	if [ -n "$$files" ]; then \
		echo "gofmt needed:"; \
		echo "$$files"; \
		echo "Run: make fmt"; \
		exit 1; \
	fi

tidy: ## Apply go mod tidy.
	go mod tidy

tidy-check: ## Check go.mod/go.sum without modifying files.
	go mod tidy -diff

vuln: ## Check dependencies and reachable code for known vulnerabilities.
	go tool govulncheck ./...

check: fmt-check tidy-check vuln lint test ## Run all non-mutating checks.

clean: ## Remove build artifacts, coverage files, and test cache.
	rm -rf $(BUILD_DIR) dist
	go clean -testcache
	rm -f *.out cover.* coverage.*
