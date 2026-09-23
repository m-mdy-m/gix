ifeq ($(OS),Windows_NT)
  EXE       := .exe
  VERSION   ?= $(shell git describe --tags --always --dirty 2>NUL || echo dev)
  MKDIR_P    = if not exist "$(1)" mkdir "$(1)"
  RM_RF      = if exist "$(1)" rmdir /s /q "$(1)"
  FMT_CHECK  = powershell -NoProfile -Command "$$out = gofmt -l cmd internal; if ($$out) { Write-Host 'not gofmt-formatted:'; Write-Host $$out; exit 1 }"
  GOBIN_BIN  = $(shell go env GOPATH)\bin
  SET_EXEC   = rem exec bit not needed on windows - git runs hooks via sh
  GOLANGCI  := $(firstword $(shell golangci-lint version 2>NUL))
  ECHO_HELP  = @echo
else
  EXE       :=
  VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
  MKDIR_P    = mkdir -p $(1)
  RM_RF      = rm -rf $(1)
  FMT_CHECK  = out=$$(gofmt -l cmd internal); if [ -n "$$out" ]; then echo "not gofmt-formatted:"; echo "$$out"; exit 1; fi
  GOBIN_BIN  = $(shell go env GOPATH)/bin
  SET_EXEC   = chmod +x .husky/pre-commit .husky/commit-msg .husky/pre-push
  GOLANGCI  := $(firstword $(shell golangci-lint version 2>/dev/null))
  ECHO_HELP  = @echo
endif

BINARY    := gix$(EXE)
CMD_DIR   := ./cmd/gix
BUILD_DIR := build
IMG       ?= bitsgenix/gix
TAG       ?= $(VERSION)

# Version var lives in internal/commands, not package main
LDFLAGS := -s -w -X github.com/m-mdy-m/gix/internal/commands.Version=$(VERSION)

.PHONY: help setup hooks build install uninstall test cover fmt fmt-check \
        vet quality lint check ci docker clean

help: # Show this help
	$(ECHO_HELP) gix make targets:
	$(ECHO_HELP)   setup      enable git hooks in .husky - once per clone
	$(ECHO_HELP)   build      build ./build/$(BINARY)
	$(ECHO_HELP)   install    go install to GOPATH/bin
	$(ECHO_HELP)   uninstall  remove the installed binary
	$(ECHO_HELP)   test       go test ./...
	$(ECHO_HELP)   cover      go test -cover ./...
	$(ECHO_HELP)   fmt        gofmt -w cmd internal
	$(ECHO_HELP)   fmt-check  fail if Go code is not formatted
	$(ECHO_HELP)   vet        go vet ./...
	$(ECHO_HELP)   quality    fmt-check + vet + test
	$(ECHO_HELP)   lint       golangci-lint - skipped if not installed
	$(ECHO_HELP)   check      quality + lint - what CI runs
	$(ECHO_HELP)   docker     build $(IMG):$(TAG)
	$(ECHO_HELP)   clean      remove build/

setup hooks: # Enable git hooks, once per clone
	git config core.hooksPath .husky
	@$(SET_EXEC)
	@echo hooks enabled - .husky

build: # Build ./build/$(BINARY)
	@$(call MKDIR_P,$(BUILD_DIR))
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)
	@echo built $(BUILD_DIR)/$(BINARY) - $(VERSION)

install: # go install to GOPATH/bin
	go install -ldflags "$(LDFLAGS)" $(CMD_DIR)

uninstall: # Remove installed binary
	@$(call RM_RF,$(GOBIN_BIN)/$(BINARY))

test: # Run tests
	go test ./...

cover: # Run tests with coverage
	go test -cover ./...

fmt: # Format Go code
	gofmt -w cmd internal

fmt-check: # Fail if Go code is not formatted
	@$(FMT_CHECK)

vet: # Static analysis
	go vet ./...

quality: fmt-check vet test # fmt-check + vet + test

lint: # golangci-lint, skipped when not installed
ifeq ($(GOLANGCI),)
	@echo golangci-lint not installed - skipping - https://golangci-lint.run/usage/install/
else
	golangci-lint run ./...
endif

check ci: quality lint # quality + lint - what CI runs
	@echo check passed

docker: # Build Docker image
	docker build --build-arg VERSION=$(VERSION) -t $(IMG):$(TAG) .

clean: # Remove build output
	@$(call RM_RF,$(BUILD_DIR))
