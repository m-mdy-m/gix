SHELL := /bin/sh

GO ?= go
GOLANGCI_LINT ?= golangci-lint
BINARY := gix
CMD := ./cmd/gix
BUILD_DIR := build
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.Version=$(VERSION)
IMAGE ?= bitsgenix/gix

.PHONY: all setup fmt fmt-check build test vet lint quality ci run install uninstall docker docker-build release clean

all: quality build

setup: hooks
	@echo "gix development environment ready"

hooks:
	@git config core.hooksPath .husky
	@chmod +x .husky/pre-commit .husky/commit-msg .husky/pre-push
	@echo "Git hooks enabled"

fmt:
	@$(GO) fmt ./...

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -type f -not -path './vendor/*'))" || \
		{ echo "Go files need formatting. Run: make fmt"; exit 1; }

build:
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 $(GO) build -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD)

run:
	@$(GO) run $(CMD)

test:
	@$(GO) test ./...

vet:
	@$(GO) vet ./...

lint:
	@$(GOLANGCI_LINT) run ./...

quality: fmt-check test vet

ci: quality lint

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

docker: docker-build

install: build
	@$(GO) install -trimpath -ldflags "$(LDFLAGS)" $(CMD)
	@echo "installed $(BINARY)"

uninstall:
	@rm -f "$${GOBIN:-$$(go env GOPATH)/bin}/$(BINARY)"

release:
	@test -n "$(VERSION)" || { echo "VERSION is required"; exit 1; }
	@test "$$(git branch --show-current)" = "main" || { echo "release must run from main"; exit 1; }
	@test -z "$$(git status --porcelain)" || { echo "working tree must be clean"; exit 1; }
	@printf '%s\n' "$(VERSION)" | grep -Eq '^v?[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "VERSION must look like 1.2.3 or v1.2.3"; exit 1; }
	@TAG="$(VERSION)"; case "$$TAG" in v*) ;; *) TAG="v$$TAG";; esac; \
		git tag -a "$$TAG" -m "Release $$TAG"; git push origin "$$TAG"

clean:
	@rm -rf $(BUILD_DIR)
