BUILD_DIR := build
BINARY := $(BUILD_DIR)/grove
GOFMT_FILES := $(shell git ls-files --cached --others --exclude-standard -- '*.go')

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/jmcampanini/grove-cli/cmd.Version=$(VERSION)"

.DEFAULT_GOAL := help
.PHONY: help build test lint lint-fix fmt fmt-check tidy tidy-check check clean

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

fmt: ## Apply gofmt -w to tracked/non-ignored Go files.
	@if [ -n "$(GOFMT_FILES)" ]; then gofmt -w $(GOFMT_FILES); fi

fmt-check: ## Fail if tracked/non-ignored Go files need gofmt.
	@if [ -z "$(GOFMT_FILES)" ]; then exit 0; fi; \
	diff=$$(gofmt -l $(GOFMT_FILES) 2>&1); rc=$$?; \
	if [ $$rc -ne 0 ]; then echo "gofmt failed (rc=$$rc):"; echo "$$diff"; exit $$rc; fi; \
	if [ -n "$$diff" ]; then echo "gofmt issues:"; echo "$$diff"; exit 1; fi

tidy: ## Apply go mod tidy.
	go mod tidy

tidy-check: ## Check go.mod/go.sum without modifying files.
	go mod tidy -diff

check: fmt-check tidy-check lint test ## Run all non-mutating checks.

clean: ## Remove build artifacts, coverage files, and test cache.
	rm -rf $(BUILD_DIR) dist
	go clean -testcache
	rm -f *.out cover.* coverage.*
