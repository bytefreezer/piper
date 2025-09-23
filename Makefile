.PHONY: build clean test help ansible-prep

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
BINARY_NAME=bytefreezer-piper

# Build info
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS = -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "✓ Build completed: $(BINARY_NAME)"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...
	@echo "✓ Tests passed"

ansible-prep: build ## Build binary and prepare for Ansible deployment
	@echo "Preparing for Ansible deployment..."
	@mkdir -p ansible/playbooks/dist
	@cp $(BINARY_NAME) ansible/playbooks/dist/
	@echo "✓ Binary copied to ansible/playbooks/dist/"
	@echo "Ready for Ansible deployment:"
	@echo "  cd ansible/playbooks && ansible-playbook -i inventory install.yml"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -f $(BINARY_NAME)
	@rm -rf ansible/playbooks/dist
	@echo "✓ Cleaned"