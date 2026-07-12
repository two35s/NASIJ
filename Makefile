BINARY     := nasij
CMD        := ./cmd/nasij
BIN_DIR    := ./bin
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS    := -X github.com/nasij/nasij/pkg/version.Version=$(VERSION) \
              -X github.com/nasij/nasij/pkg/version.GitCommit=$(COMMIT) \
              -X github.com/nasij/nasij/pkg/version.BuildDate=$(BUILD_DATE)

.PHONY: all build test lint fmt clean install help

all: build

## build: Compile the binary into ./bin/nasij
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(CMD)
	@echo "  Built → $(BIN_DIR)/$(BINARY)"

## install: Install nasij to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" $(CMD)

## test: Run all unit + integration tests with race detector
test:
	go test ./... -v -race -count=1 -timeout=120s

## test-cover: Run tests with HTML coverage report
test-cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@echo "  Coverage report → coverage.html"

## lint: Run golangci-lint (install with scripts/install-tools.sh)
lint:
	golangci-lint run ./...

## fmt: Format all Go source files
fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

## vet: Run go vet
vet:
	go vet ./...

## clean: Remove build artifacts
clean:
	rm -rf $(BIN_DIR) coverage.out coverage.html

## help: Show this help message
help:
	@grep -E '^##' Makefile | sed 's/## //' | column -t -s ':'
