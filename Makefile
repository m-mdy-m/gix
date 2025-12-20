VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build info
BINARY_NAME := gix
BUILD_DIR := build
CMD_DIR := ./cmd/$(BINARY_NAME)

# LDFLAGS
LDFLAGS = -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE)"

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m


all: clean build

build:
	@echo "$(YELLOW)Building $(BINARY_NAME) $(VERSION)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"
	@echo "$(GREEN)✓ Version: $(VERSION)$(NC)"

dev:
	@echo "$(YELLOW)Building development version...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go build -race -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "$(GREEN)✓ Dev build complete$(NC)"

clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)✓ Clean complete$(NC)"

