.PHONY: build test clean install dev-build run help

# Default target
help:
	@echo "Available targets:"
	@echo "  build      - Build the binary for current platform"
	@echo "  dev-build  - Build with dev version for testing"
	@echo "  test       - Run tests"
	@echo "  clean      - Clean build artifacts"
	@echo "  install    - Install to /usr/local/bin"
	@echo "  run        - Build and run the tool"
	@echo "  release    - Build all platform binaries"

# Build for current platform
build:
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev") && \
	BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
	GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") && \
	LDFLAGS="-X nmcrun/internal/version.Version=$$VERSION -X nmcrun/internal/version.BuildDate=$$BUILD_DATE -X nmcrun/internal/version.GitCommit=$$GIT_COMMIT" && \
	go build -ldflags="$$LDFLAGS" -o nmcrun .

# Development build with dev version
dev-build:
	@BUILD_DATE=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") && \
	LDFLAGS="-X nmcrun/internal/version.Version=dev -X nmcrun/internal/version.BuildDate=$$BUILD_DATE -X nmcrun/internal/version.GitCommit=dev" && \
	go build -ldflags="$$LDFLAGS" -o nmcrun .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f nmcrun
	rm -rf dist/

# Install to system
install: build
	sudo cp nmcrun /usr/local/bin/
	@echo "nmcrun installed to /usr/local/bin/"

# Build and run
run: dev-build
	./nmcrun

# Build all platform binaries
release:
	chmod +x build.sh
	./build.sh

# Quick version check
version: dev-build
	./nmcrun version 