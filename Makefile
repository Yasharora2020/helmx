# Makefile for helmx (Helm Explorer)

BINARY_NAME=helmx
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE?=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

.PHONY: all build run clean test install deps lint fmt fmt-check vet setup check help

# Default target
all: build

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build the binary
build: deps
	go build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/helmx

# Run without building binary
run:
	go run ./cmd/helmx

# Install to GOPATH/bin
install: deps
	go install ${LDFLAGS} ./cmd/helmx

# Run tests
test:
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run ./...

# Fix lint issues
lint-fix:
	golangci-lint run --fix ./...

# Format code
fmt:
	gofmt -w .

# Check formatting
fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Files need formatting:" && gofmt -l . && exit 1)

# Run go vet
vet:
	go vet ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

# Build for multiple platforms (for releases)
build-all:
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-amd64 ./cmd/helmx
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-darwin-arm64 ./cmd/helmx
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-linux-amd64 ./cmd/helmx
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-linux-arm64 ./cmd/helmx
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-windows-amd64.exe ./cmd/helmx

# Quick dev cycle - build and run
dev: build
	./bin/${BINARY_NAME}

# Setup development environment (install pre-commit hooks)
setup:
	./scripts/setup-hooks.sh

# Install golangci-lint
install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run all checks (same as CI)
check: fmt-check vet lint test

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  run           - Run without building"
	@echo "  dev           - Build and run"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  lint          - Run golangci-lint"
	@echo "  lint-fix      - Run golangci-lint with auto-fix"
	@echo "  fmt           - Format code"
	@echo "  fmt-check     - Check formatting"
	@echo "  vet           - Run go vet"
	@echo "  clean         - Remove build artifacts"
	@echo "  setup         - Setup pre-commit hooks"
	@echo "  install-lint  - Install golangci-lint"
	@echo "  check         - Run all checks (CI)"
	@echo "  build-all     - Build for all platforms"
	@echo "  help          - Show this help"
