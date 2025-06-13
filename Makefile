.PHONY: fmt lint tidy docker-build docker-push test test-coverage

SHELL := nix develop --command bash

# Version variables
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Docker variables
DOCKER_REGISTRY?=docker.io
DOCKER_IMAGE?=$(DOCKER_REGISTRY)/liamawhite/microservice
DOCKER_TAG?=$(VERSION)

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

# Build Docker image
docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		.

# Push Docker image
docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

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
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-push  - Push Docker image to registry"
	@echo "  make dev          - Run fmt, lint, and tidy"
	@echo "  make test         - Run all tests"
	@echo "  make test-cov     - Run tests with coverage"
	@echo "  make help         - Show this help message" 