# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based microservice that creates composable mock microservice topologies. The service acts as an HTTP proxy that can chain requests through multiple services to simulate complex microservice architectures for testing purposes.

## Development Environment

This project uses Nix for development environment management. Always run commands within the Nix development shell:

```bash
nix develop --command bash
```

The Makefile automatically uses the Nix shell via `SHELL := nix develop --command bash`.

## Core Architecture

### Proxy Chain System
- **Entry Point**: `cmd/main.go` - HTTP server with configurable ports, timeouts, and logging
- **Proxy Handler**: `internal/proxy/handler.go` - Core proxy logic that parses paths and forwards requests
- **Path Format**: `/proxy/service:port/proxy/next-service:port/...` - Chain multiple services together
- **Final Hop**: When no more `/proxy/` segments exist, the service returns its own response

### Request Flow
1. Request arrives at service with path like `/proxy/service-b:8080/proxy/service-c:80`
2. Handler parses path to extract next hop (`service-b:8080`) and remaining path (`/proxy/service-c:80`)
3. If remaining path exists, forwards request to next service
4. If no remaining path, returns final JSON response with service name

### Response Format
```json
{
  "status": 200,
  "service": "service-name",
  "message": "Request processed successfully"
}
```

## Common Commands

### Development
- `make check` - Format, lint, and tidy (run before committing)
- `make test` - Run all tests
- `make test-coverage` - Run tests with coverage report
- `make fmt` - Format Go code
- `make lint` - Run golangci-lint
- `make tidy` - Tidy Go modules

### Testing
- `go test -v ./...` - Run all tests with verbose output
- `go test -v ./internal/proxy` - Run specific package tests
- `go test -v ./tests/functional` - Run functional tests (requires Docker)

### Security
- `make security` - Run gosec security scanner

### Docker
- `make docker-build` - Build multi-platform Docker image
- `make docker-build DOCKER_PUSH=true` - Build and push to registry

## Testing Architecture

### Unit Tests
- Located in `internal/proxy/handler_test.go`
- Test individual proxy handler functions

### Functional Tests
- Located in `tests/functional/topology_test.go` 
- Use testcontainers to create real Docker networks with multiple services
- Test actual proxy chains with HTTP requests between containers
- Utilities in `tests/functional/utils.go` for container management

### Test Network Setup
Functional tests create isolated Docker networks where containers communicate using:
- Container names as hostnames (e.g., `service-a`, `service-b`)
- Explicit port configurations for each service
- Parallel container startup for performance

## Helm Chart

The service includes a Helm chart for Kubernetes deployment located in the `chart/` directory.

### Installation

#### From Local Chart
```bash
# Install from local chart directory
helm install my-microservice ./chart/

# Package and install
helm package chart/ --destination .
helm install my-microservice ./microservice-0.1.0.tgz
```

## Configuration

### Service Configuration
- **Port**: `-port` flag (default: 8080)
- **Timeout**: `-timeout` flag (default: 30s)  
- **Service Name**: `-service-name` flag (default: "proxy")
- **Log Level**: `-log-level` flag (debug, info, warn, error)
- **Log Format**: `-log-format` flag (json, text)

### Health Check
All services expose `/health` endpoint returning:
```json
{"status":"healthy","service":"service-name"}
```

