# Makefile for agent-manager-pro

BINARY     := agent-manager
CMD_PATH   := ./cmd/agent-manager
VERSION    ?= dev
LDFLAGS    := -s -w -X main.version=$(VERSION)

.PHONY: all build test test-race vet fmt fmt-check lint smoke install clean help

# Default target
all: build

## build: Compile the binary to ./agent-manager
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD_PATH)

## test: Run all unit tests
test:
	go test ./...

## test-race: Run all unit tests with race detector
test-race:
	go test -race ./...

## vet: Run go vet static analysis
vet:
	go vet ./...

## fmt: Format all Go source files in-place
fmt:
	gofmt -w .

## fmt-check: Fail if any Go source files need formatting
fmt-check:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files need formatting (run 'make fmt'):"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

## lint: Run golangci-lint if available, otherwise fall back to go vet
lint:
	@if command -v golangci-lint > /dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, falling back to go vet"; \
		go vet ./...; \
	fi

## smoke: Build the binary then run the smoke-test script
smoke: build
	./scripts/smoke-test.sh

## install: Install the binary to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" $(CMD_PATH)

## clean: Remove the built binary, dist/ directory, and test caches
clean:
	rm -f $(BINARY)
	rm -rf dist/
	go clean -testcache

## help: List available targets with descriptions
help:
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
