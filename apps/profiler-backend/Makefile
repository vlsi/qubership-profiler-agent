# Profiler backend (Go) Makefile

.PHONY: help build clean test ui-build docker-binaries docker-build archive run

ARTIFACT_NAME := profiler-backend
BINARY_NAME := profiler-backend
DOCKER_IMAGE := $(ARTIFACT_NAME)

# Build settings - can be overridden for cross-compilation
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
PLATFORMS ?= linux/$(GOARCH)
UI_BASE_PATH ?= /ui/
GO_TOOLCHAIN := $(shell sed -n 's/^toolchain //p' ../../go.mod)
COMMA := ,

help:
	@echo "Available targets:"
	@echo "  build         - Build the Go application"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  docker-binaries - Build UI and Linux binaries for PLATFORMS"
	@echo "  docker-build  - Build Docker image"
	@echo "  archive       - No-op; the Docker image is the deliverable"

clean:
	@echo "==> Removing old artifacts"
	rm -f $(BINARY_NAME)
	rm -rf build

build:
	@echo "==> Building Go application for $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(BINARY_NAME) .

test:
	@echo "==> Running tests..."
	go test ./...

ui-build:
	cd ../ui && npm ci --no-audit --no-fund
	cd ../ui && UI_BASE_PATH=$(UI_BASE_PATH) npm run build

docker-binaries: ui-build
	mkdir -p build/image/data
	chmod 755 build/image/data
	@set -eu; for platform in $(subst $(COMMA), ,$(PLATFORMS)); do \
		case "$$platform" in linux/amd64|linux/arm64) ;; *) echo "Unsupported platform: $$platform" >&2; exit 1 ;; esac; \
		arch=$${platform#linux/}; \
		mkdir -p build/image/linux_$$arch; \
		GOTOOLCHAIN=$(GO_TOOLCHAIN) CGO_ENABLED=0 GOOS=linux GOARCH=$$arch \
			go build -o build/image/linux_$$arch/$(BINARY_NAME) .; \
		go version -m build/image/linux_$$arch/$(BINARY_NAME); \
	done

docker-build: docker-binaries
	@echo "==> Building Docker image $(DOCKER_IMAGE)..."
	cd ../.. && docker build --platform $(PLATFORMS) -f apps/profiler-backend/Dockerfile -t $(DOCKER_IMAGE) .

archive:
	@echo "==> No archive for $(ARTIFACT_NAME); the Docker image is the deliverable"
