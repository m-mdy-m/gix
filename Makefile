VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := gix
CMD_DIR := ./cmd/gix
BUILD_DIR := build

LDFLAGS := -s -w -X main.Version=$(VERSION)

.PHONY: build install test clean

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)
	@echo "built $(BUILD_DIR)/$(BINARY) ($(VERSION))"

install:
	go install -ldflags "$(LDFLAGS)" $(CMD_DIR)

test:
	go test ./...

clean:
	rm -rf $(BUILD_DIR)