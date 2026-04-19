BINARY_NAME := grove
BUILD_DIR := out

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/jmcampanini/grove-cli/cmd.Version=$(VERSION)"

.DEFAULT_GOAL := help
.PHONY: build check clean fmt help lint test tidy

help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## compile the grove binary
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

test: ## run tests with -race
	go test -race ./...

lint: ## run golangci-lint
	golangci-lint run ./...

check: ## fmt check + tidy check + lint + test
	@# Capture stderr so parse errors / missing gofmt fail loudly, not silently.
	@out=$$(gofmt -l . 2>&1 || true); if [ -n "$$out" ]; then echo "gofmt output:"; echo "$$out"; gofmt -d .; exit 1; fi
	go mod tidy -diff
	golangci-lint run ./...
	go test -race ./...

fmt: ## apply gofmt in-place
	gofmt -w .

tidy: ## apply go mod tidy
	go mod tidy

clean: ## remove out/, test cache, *.out, cover.*
	rm -rf $(BUILD_DIR)
	go clean -testcache
	rm -f *.out cover.*
