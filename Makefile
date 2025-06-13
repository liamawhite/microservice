.PHONY: fmt lint tidy docker-build docker-push test test-coverage

SHELL := nix develop --command bash

# Version variables
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Docker variables
DOCKER_REGISTRY?=ghcr.io
DOCKER_IMAGE?=$(DOCKER_REGISTRY)/liamawhite/microservice
DOCKER_TAG?=$(VERSION)
DOCKER_PLATFORMS?=linux/amd64,linux/arm64
DOCKER_PUSH?=false

# Format all Go files
fmt:
	gofmt -w .

# Run golangci-lint
lint:
	golangci-lint run

# Tidy and verify Go module dependencies
tidy:
	go mod tidy
	go mod verify

# Build multi-platform Docker image (optionally push)
docker-build:
	docker buildx build \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_IMAGE):latest \
		$(if $(filter true,$(DOCKER_PUSH)),--push,--load) \
		.

# Development workflow: format, lint, and tidy
dev: fmt lint tidy

# Run all tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

security:
	gosec -severity medium -confidence high -exclude-generated ./...

# Help command
help:
	@echo "Available commands:"
	@echo "  make fmt          - Format Go code"
	@echo "  make lint         - Run linter"
	@echo "  make tidy         - Tidy Go modules"
	@echo "  make docker-build - Build multi-platform Docker image (DOCKER_PUSH=true to push)"
	@echo "  make dev          - Run fmt, lint, and tidy"
	@echo "  make test         - Run all tests"
	@echo "  make test-cov     - Run tests with coverage"
	@echo "  make help         - Show this help message" 