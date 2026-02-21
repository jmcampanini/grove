BINARY_NAME := grove
BUILD_DIR := build
INSTALL_DIR := $(HOME)/.local/bin

VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X github.com/jmcampanini/grove-cli/cmd.Version=$(VERSION)"

# Shell completion directories
BREW_PREFIX := $(shell brew --prefix 2>/dev/null || echo "/opt/homebrew")
FISH_COMPLETIONS_DIR := $(HOME)/.config/fish/completions
BASH_COMPLETIONS_DIR := $(BREW_PREFIX)/etc/bash_completion.d
ZSH_COMPLETIONS_DIR := $(BREW_PREFIX)/share/zsh/site-functions

.PHONY: all build check lint test clean help install install-completions uninstall

all: build ## Default target

build: check ## Build the binary
	mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

check: lint test ## Run all quality checks

lint: ## Run golangci-lint
	golangci-lint run ./...

test: ## Run tests
	go test ./...

clean: ## Remove build artifacts
	go clean
	rm -rf $(BUILD_DIR)

install-completions: build ## Install shell completions
	@echo "Installing shell completions..."
	@mkdir -p $(FISH_COMPLETIONS_DIR)
	@$(BUILD_DIR)/$(BINARY_NAME) completion fish > $(FISH_COMPLETIONS_DIR)/$(BINARY_NAME).fish
	@echo "  fish: $(FISH_COMPLETIONS_DIR)/$(BINARY_NAME).fish"
	@mkdir -p $(BASH_COMPLETIONS_DIR)
	@$(BUILD_DIR)/$(BINARY_NAME) completion bash > $(BASH_COMPLETIONS_DIR)/$(BINARY_NAME)
	@echo "  bash: $(BASH_COMPLETIONS_DIR)/$(BINARY_NAME)"
	@mkdir -p $(ZSH_COMPLETIONS_DIR)
	@$(BUILD_DIR)/$(BINARY_NAME) completion zsh > $(ZSH_COMPLETIONS_DIR)/_$(BINARY_NAME)
	@echo "  zsh:  $(ZSH_COMPLETIONS_DIR)/_$(BINARY_NAME)"

install: build install-completions ## Install binary and completions
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(INSTALL_DIR)/$(BINARY_NAME)"

uninstall: ## Remove binary and completions
	@echo "Uninstalling $(BINARY_NAME)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@rm -f $(FISH_COMPLETIONS_DIR)/$(BINARY_NAME).fish
	@rm -f $(BASH_COMPLETIONS_DIR)/$(BINARY_NAME)
	@rm -f $(ZSH_COMPLETIONS_DIR)/_$(BINARY_NAME)
	@echo "Uninstalled"

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
