.DEFAULT_GOAL := help

.PHONY: *

BINARY := whatsapp
BUILD_DIR := ./dist
CMD_DIR := ./cmd/whatsapp

help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z\/_%-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

build: ## Build binary with CGO for SQLite FTS5
	CGO_ENABLED=1 go build -tags sqlite_fts5 -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)

install: build ## Install binary to GOPATH/bin
	cp $(BUILD_DIR)/$(BINARY) $(GOPATH)/bin/$(BINARY)

run: ## Run a command (CMD="chats --limit 5")
	CGO_ENABLED=1 go run -tags sqlite_fts5 $(CMD_DIR) $(CMD)

dev: build ## Build and run a command (CMD="chats --limit 5")
	$(BUILD_DIR)/$(BINARY) $(CMD)

deps: ## Sync dependencies
	go mod tidy
	go mod download

##@ Testing/Linting

can-release: lint test ## Run all CI checks (lint + test)

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format code
	go fmt ./...
	goimports -w .

test: ## Run all tests
	CGO_ENABLED=1 go test -tags sqlite_fts5 -v ./...

##@ Utilities

set-version: ## Set version (VERSION=x.x.x)
	@if [ -z "$(VERSION)" ]; then echo "Usage: make set-version VERSION=x.x.x"; exit 1; fi
	sed -i.bak 's/version = "[^"]*"/version = "$(VERSION)"/' internal/cli/root.go && rm internal/cli/root.go.bak

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	go clean
